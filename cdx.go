package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CDXEntry holds one CDX result row.
type CDXEntry struct {
	Timestamp   string
	OriginalURL string
}

var cdxHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

// fetchCDXPage fetches a single page of CDX results.
// pageIndex == -1 means no pagination parameter (fetch all at once for exact URL).
func fetchCDXPage(baseURL string, pageIndex int, fromTS, toTS string) ([]CDXEntry, error) {
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

	resp, err := cdxHTTPClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cdx GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdx HTTP %d for %s", resp.StatusCode, apiURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cdx read body: %w", err)
	}

	// The CDX API returns a JSON array of arrays, first row is the header.
	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		// Empty body or non-JSON means no results.
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

// fetchAllSnapshots collects every CDX entry for all URL variants.
// When exactURL is false it appends /* for wildcard and paginates.
func fetchAllSnapshots(variants []string, exactURL bool, fromTS, toTS string) ([]CDXEntry, error) {
	seen := make(map[string]bool)
	var all []CDXEntry

	for _, variant := range variants {
		if exactURL {
			entries, err := fetchCDXPage(variant, -1, fromTS, toTS)
			if err != nil {
				return nil, err
			}
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
				entries, err := fetchCDXPage(wildcardURL, page, fromTS, toTS)
				if err != nil {
					// On error stop paginating this variant
					break
				}
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
				// If fewer results came back than a full page we are done
				if len(entries) < 100 {
					break
				}
			}
		}
	}
	return all, nil
}
