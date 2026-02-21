package wayback

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
	"golang.org/x/sync/errgroup"
)

// Config holds all runtime configuration for the downloader.
type Config struct {
	BaseURL                string
	Variants               []string
	BareHost               string
	UnicodeHost            string
	ExactURL               bool
	Directory              string
	FromTimestamp          string
	ToTimestamp            string
	Threads                int
	RewriteLinks           bool
	PrettyPath             bool
	CanonicalAction        string
	DownloadExternalAssets bool
	Debug                  bool
	StopOnError            bool
	CDXRatePerMin          int     // CDX API requests per minute (default 60)
	CDXMaxRetries          int     // max retry attempts on throttle/5xx (default 5)
	Storage                Storage // if nil, NewLocalStorage(Directory) is used
}

var downloadHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
}

// DownloadAll fetches the CDX index and downloads every snapshot concurrently.
func DownloadAll(cfg *Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cdxProg := NewCDXProgress()
	entries, err := fetchAllSnapshots(ctx, cfg.Variants, cfg.ExactURL, cfg.FromTimestamp, cfg.ToTimestamp, cdxProg, cfg.CDXRatePerMin, cfg.CDXMaxRetries)
	cdxProg.Finish()
	if err != nil {
		return fmt.Errorf("CDX fetch: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}

	// Build deduplication index
	idx := NewSnapshotIndex()
	for _, e := range entries {
		idx.Register(e.OriginalURL, e.Timestamp)
	}

	manifest := idx.GetManifest()
	total := len(manifest)
	if cfg.Debug {
		fmt.Printf("Found %d unique snapshots to download.\n", total)
	}

	store := cfg.Storage
	if store == nil {
		store = NewLocalStorage(cfg.Directory)
	}

	pool, err := ants.NewPool(cfg.Threads)
	if err != nil {
		return fmt.Errorf("create worker pool: %w", err)
	}
	defer pool.Release()

	g, ctx := errgroup.WithContext(ctx)
	dlProg := NewDownloadProgress(total)
	var failed atomic.Int32

	for _, snap := range manifest {
		s := snap
		g.Go(func() error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			errCh := make(chan error, 1)
			if err := pool.Submit(func() {
				errCh <- downloadOne(ctx, s, cfg, store, idx, dlProg)
			}); err != nil {
				return fmt.Errorf("submit task: %w", err)
			}
			if err := <-errCh; err != nil {
				if cfg.StopOnError {
					return err
				}
				failed.Add(1)
				if cfg.Debug {
					log.Printf("download error %s: %v", s.FileURL, err)
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	dlProg.Finish()
	if n := failed.Load(); n > 0 {
		fmt.Printf("%d resource(s) failed to download.\n", n)
	}
	return nil
}

// downloadOne downloads a single snapshot and optionally rewrites its links.
func downloadOne(ctx context.Context, snap Snapshot, cfg *Config, store Storage, idx *SnapshotIndex, dlProg *Progress) error {

	if ctx.Err() != nil {
		return ctx.Err()
	}

	logicalPath := URLToLocalPath(snap.FileURL, cfg.PrettyPath)

	// Skip existing files
	if store.Exists(logicalPath) {
		dlProg.Inc()
		return nil
	}

	// Build Wayback Machine URL using the id_ flag to get raw content
	waybackURL := fmt.Sprintf("https://web.archive.org/web/%sid_/%s", snap.Timestamp, snap.FileURL)

	if cfg.Debug {
		log.Printf("GET %s", waybackURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, waybackURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := downloadHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// Skip 404s gracefully
		dlProg.Inc()
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, waybackURL)
	}

	// Read first 512 bytes for content sniffing, then stream remainder via storage
	first := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, first)
	first = first[:n]

	if err := store.Put(logicalPath, io.MultiReader(bytes.NewReader(first), resp.Body)); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	// Post-process HTML / CSS
	if cfg.RewriteLinks {
		if rw := DetectRewriter(logicalPath, resp.Header.Get("Content-Type"), first); rw != nil {
			if err := rw.Rewrite(store, logicalPath, snap.FileURL, cfg, idx); err != nil && cfg.Debug {
				log.Printf("rewrite %s: %v", logicalPath, err)
			}
		}
	}

	dlProg.Inc()
	return nil
}

// WaybackAssetURL builds a Wayback raw-content URL for an asset, resolving the
// best available timestamp via the snapshot index.
func WaybackAssetURL(assetURL, fallbackTS string, idx *SnapshotIndex) string {
	ts := idx.Resolve(assetURL, fallbackTS)
	return fmt.Sprintf("https://web.archive.org/web/%sid_/%s", ts, assetURL)
}

// isInternalHost returns true when host (stripped of www.) matches bareHost.
func isInternalHost(host, bareHost string) bool {
	h := strings.TrimPrefix(strings.ToLower(host), "www.")
	return h == strings.ToLower(bareHost)
}
