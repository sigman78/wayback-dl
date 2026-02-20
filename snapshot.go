package main

import (
	"net/url"
	"sort"
)

// Snapshot represents a single archived file to download.
type Snapshot struct {
	FileURL   string // original URL
	Timestamp string // CDX timestamp string
	FileID    string // decoded URL path (deduplication key)
}

// SnapshotIndex deduplicates CDX entries and builds lookup maps.
type SnapshotIndex struct {
	byPath         map[string]Snapshot // path → latest snapshot
	byPathAndQuery map[string]Snapshot // path+query → latest snapshot
	manifest       []Snapshot          // sorted newest-first (lazy)
	lookupPath     map[string]string   // path → timestamp (lazy)
	lookupQuery    map[string]string   // path+query → timestamp (lazy)
	built          bool
}

// NewSnapshotIndex creates an empty index.
func NewSnapshotIndex() *SnapshotIndex {
	return &SnapshotIndex{
		byPath:         make(map[string]Snapshot),
		byPathAndQuery: make(map[string]Snapshot),
	}
}

// Register adds a CDX entry to the index, keeping the lexicographically greatest timestamp.
func (idx *SnapshotIndex) Register(rawURL, timestamp string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}

	pathKey := u.Path
	queryKey := pathKey
	if u.RawQuery != "" {
		queryKey += "?" + u.RawQuery
	}

	snap := Snapshot{
		FileURL:   rawURL,
		Timestamp: timestamp,
		FileID:    queryKey,
	}

	// Keep only the snapshot with the greatest (latest) timestamp string.
	if existing, ok := idx.byPathAndQuery[queryKey]; !ok || timestamp > existing.Timestamp {
		idx.byPathAndQuery[queryKey] = snap
	}
	if existing, ok := idx.byPath[pathKey]; !ok || timestamp > existing.Timestamp {
		idx.byPath[pathKey] = snap
	}
}

// GetManifest builds and returns the full sorted snapshot list (newest first).
// Also initialises the lookup maps for Resolve.
func (idx *SnapshotIndex) GetManifest() []Snapshot {
	if idx.built {
		return idx.manifest
	}

	// Collect unique snapshots from byPathAndQuery (authoritative)
	seen := make(map[string]bool)
	for _, s := range idx.byPathAndQuery {
		if !seen[s.FileID] {
			seen[s.FileID] = true
			idx.manifest = append(idx.manifest, s)
		}
	}

	// Sort newest-first
	sort.Slice(idx.manifest, func(i, j int) bool {
		return idx.manifest[i].Timestamp > idx.manifest[j].Timestamp
	})

	// Build lookup maps
	idx.lookupPath = make(map[string]string, len(idx.byPath))
	for k, s := range idx.byPath {
		idx.lookupPath[k] = s.Timestamp
	}
	idx.lookupQuery = make(map[string]string, len(idx.byPathAndQuery))
	for k, s := range idx.byPathAndQuery {
		idx.lookupQuery[k] = s.Timestamp
	}

	idx.built = true
	return idx.manifest
}

// Resolve finds the best timestamp for an asset URL.
// It checks path+query first, then path only, falling back to the provided default.
func (idx *SnapshotIndex) Resolve(assetURL, fallback string) string {
	if !idx.built {
		idx.GetManifest()
	}

	u, err := url.Parse(assetURL)
	if err != nil {
		return fallback
	}

	pathKey := u.Path
	queryKey := pathKey
	if u.RawQuery != "" {
		queryKey += "?" + u.RawQuery
	}

	if ts, ok := idx.lookupQuery[queryKey]; ok {
		return ts
	}
	if ts, ok := idx.lookupPath[pathKey]; ok {
		return ts
	}
	return fallback
}
