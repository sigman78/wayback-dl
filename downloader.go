package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
	CanonicalAction        string
	DownloadExternalAssets bool
	Debug                  bool
}

var downloadHTTPClient = &http.Client{
	Timeout: 120 * time.Second,
}

// DownloadAll fetches the CDX index and downloads every snapshot concurrently.
func DownloadAll(cfg *Config) error {
	entries, err := fetchAllSnapshots(cfg.Variants, cfg.ExactURL, cfg.FromTimestamp, cfg.ToTimestamp)
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
	fmt.Printf("Found %d unique snapshots to download.\n", total)

	if err := os.MkdirAll(cfg.Directory, 0750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	sem := make(chan struct{}, cfg.Threads)
	var wg sync.WaitGroup
	var processed int32
	var mu sync.Mutex

	for _, snap := range manifest {
		wg.Add(1)
		go func(s Snapshot) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := downloadOne(s, cfg, idx, &mu, &processed, total); err != nil {
				if cfg.Debug {
					log.Printf("download error %s: %v", s.FileURL, err)
				}
			}
		}(snap)
	}
	wg.Wait()
	return nil
}

// downloadOne downloads a single snapshot and optionally rewrites its links.
func downloadOne(snap Snapshot, cfg *Config, idx *SnapshotIndex,
	mu *sync.Mutex, processed *int32, total int) error {

	localPath := URLToLocalPath(snap.FileURL)
	localPath = filepath.Join(cfg.Directory, filepath.FromSlash(localPath))

	// Skip existing files
	if _, err := os.Stat(localPath); err == nil {
		n := atomic.AddInt32(processed, 1)
		RenderProgress(int(n), total)
		return nil
	}

	// Build Wayback Machine URL using the id_ flag to get raw content
	waybackURL := fmt.Sprintf("https://web.archive.org/web/%sid_/%s", snap.Timestamp, snap.FileURL)

	if cfg.Debug {
		log.Printf("GET %s", waybackURL)
	}

	resp, err := downloadHTTPClient.Get(waybackURL)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// Skip 404s gracefully
		n := atomic.AddInt32(processed, 1)
		RenderProgress(int(n), total)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, waybackURL)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0750); err != nil {
		return fmt.Errorf("mkdirall: %w", err)
	}

	// Stream to temp file, then rename atomically
	tmpFile, err := os.CreateTemp(filepath.Dir(localPath), ".wbdl-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName) // no-op if renamed
	}()

	// Read first 512 bytes for content sniffing
	first := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, first)
	first = first[:n]

	if _, err := tmpFile.Write(first); err != nil {
		return fmt.Errorf("write first bytes: %w", err)
	}
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpName, localPath); err != nil { //nolint:gosec // G703: localPath is sanitized by URLToLocalPath
		return fmt.Errorf("rename: %w", err)
	}

	// Post-process HTML / CSS
	if cfg.RewriteLinks {
		ct := resp.Header.Get("Content-Type")
		fileURL := snap.FileURL

		if IsHTMLFile(localPath, ct, first) {
			if err := ProcessHTML(localPath, fileURL, cfg, idx); err != nil && cfg.Debug {
				log.Printf("html rewrite %s: %v", localPath, err)
			}
		} else if IsCSSResource(localPath, ct) {
			if err := RewriteCSSFile(localPath, fileURL, cfg, idx); err != nil && cfg.Debug {
				log.Printf("css rewrite %s: %v", localPath, err)
			}
		}
	}

	cnt := atomic.AddInt32(processed, 1)
	_ = mu // mu kept for potential future terminal writes outside RenderProgress
	RenderProgress(int(cnt), total)
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
