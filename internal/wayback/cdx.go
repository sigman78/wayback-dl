package wayback

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// CDXEntry holds one CDX result row.
type CDXEntry struct {
	Timestamp   string
	OriginalURL string
}

var cdxHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

// retryDelay returns how long to wait before the next attempt.
// It honours the Retry-After header when present, otherwise uses
// exponential backoff capped at 60 s: 5 s, 10 s, 20 s, 40 s, 60 s, …
func retryDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if v := resp.Header.Get("Retry-After"); v != "" {
			if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
				d := time.Duration(secs) * time.Second
				if d > 120*time.Second {
					d = 120 * time.Second
				}
				return d
			}
		}
	}
	d := 5 * time.Second << uint(attempt)
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

// fetchCDXPage fetches a single page of CDX results.
// pageIndex == -1 means no pagination parameter (fetch all at once for exact URL).
// It retries on 429 / 5xx up to maxRetries times with exponential backoff.
func fetchCDXPage(ctx context.Context, lim *rate.Limiter, baseURL string, pageIndex int, fromTS, toTS string, maxRetries int) ([]CDXEntry, error) {
	params := url.Values{}
	params.Set("output", "json")
	params.Set("fl", "timestamp,original")
	params.Set("collapse", "digest")
	params.Set("gzip", "false")
	params.Set("filter", "statuscode:200")
	if fromTS != "" {
		params.Set("from", fromTS)
	}
	if toTS != "" {
		params.Set("to", toTS)
	}
	params.Set("url", baseURL)
	if pageIndex >= 0 {
		params.Set("page", strconv.Itoa(pageIndex))
	}

	apiURL := "https://web.archive.org/cdx/search/xd?" + params.Encode()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := lim.Wait(ctx); err != nil {
			return nil, fmt.Errorf("cdx rate limiter: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("cdx create request: %w", err)
		}
		resp, err := cdxHTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("cdx GET: %w", err)
		}

		status := resp.StatusCode
		if status == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("cdx read body: %w", err)
			}

			// The CDX API returns a JSON array of arrays, first row is the header.
			var rows [][]string
			if err := json.Unmarshal(body, &rows); err != nil {
				if strings.TrimSpace(string(body)) == "" {
					return nil, nil
				}
				return nil, fmt.Errorf("cdx json decode: %w", err)
			}

			var entries []CDXEntry
			for i, row := range rows {
				if i == 0 {
					// Skip header row (["timestamp","original"])
					continue
				}
				if len(row) < 2 {
					continue
				}
				entries = append(entries, CDXEntry{
					Timestamp:   row[0],
					OriginalURL: row[1],
				})
			}
			return entries, nil
		}

		// Retriable: 429, 503, or any other 5xx
		retriable := status == http.StatusTooManyRequests ||
			status == http.StatusServiceUnavailable ||
			(status >= 500 && status < 600)

		if !retriable {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("cdx HTTP %d for %s", status, apiURL)
		}

		if attempt == maxRetries {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("cdx HTTP %d after %d retries for %s", status, maxRetries, apiURL)
		}

		delay := retryDelay(attempt, resp)
		_ = resp.Body.Close()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	// Unreachable, but satisfies the compiler.
	return nil, fmt.Errorf("cdx: exhausted retries for %s", apiURL)
}

// fetchAllSnapshots collects every CDX entry for all URL variants.
// When exactURL is false it appends /* for wildcard and paginates.
// prog is advanced by one step for each CDX page successfully fetched.
func fetchAllSnapshots(ctx context.Context, variants []string, exactURL bool, fromTS, toTS string, prog *Progress, ratePerMin, maxRetries int) ([]CDXEntry, error) {
	lim := rate.NewLimiter(rate.Every(time.Minute/time.Duration(ratePerMin)), 5)

	seen := make(map[string]bool)
	var all []CDXEntry

	prog.SetMax(len(variants))

	for _, variant := range variants {
		if exactURL {
			entries, err := fetchCDXPage(ctx, lim, variant, -1, fromTS, toTS, maxRetries)
			if err != nil {
				return nil, err
			}
			prog.Inc()
			for _, e := range entries {
				key := e.Timestamp + "|" + e.OriginalURL
				if !seen[key] {
					seen[key] = true
					all = append(all, e)
				}
			}
		} else {
			// Wildcard: append /* and paginate
			wildcardURL := strings.TrimRight(variant, "/") + "/*"
			for page := 0; page < 100; page++ {
				entries, err := fetchCDXPage(ctx, lim, wildcardURL, page, fromTS, toTS, maxRetries)
				if err != nil {
					// On error stop paginating this variant
					break
				}
				prog.Inc()
				if len(entries) == 0 {
					break
				}
				for _, e := range entries {
					key := e.Timestamp + "|" + e.OriginalURL
					if !seen[key] {
						seen[key] = true
						all = append(all, e)
					}
				}
			}
		}
	}
	return all, nil
}
