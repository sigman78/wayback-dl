package wayback

import (
	"testing"
)

func TestSnapshotIndexEmptyManifest(t *testing.T) {
	idx := NewSnapshotIndex()
	if m := idx.GetManifest(); len(m) != 0 {
		t.Errorf("expected empty manifest, got %d entries", len(m))
	}
}

func TestSnapshotIndexRegisterAddsEntries(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("https://example.com/page.html", "20230101000000")
	idx.Register("https://example.com/style.css", "20230101000001")

	if m := idx.GetManifest(); len(m) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m))
	}
}

// Register the same URL twice: only the lexicographically greatest timestamp
// should survive.
func TestSnapshotIndexDeduplicateKeepsLatest(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("https://example.com/page.html", "20220101000000") // older
	idx.Register("https://example.com/page.html", "20230601000000") // newer

	m := idx.GetManifest()
	if len(m) != 1 {
		t.Fatalf("expected 1 entry after dedup, got %d", len(m))
	}
	if m[0].Timestamp != "20230601000000" {
		t.Errorf("expected newest timestamp, got %q", m[0].Timestamp)
	}
}

// GetManifest must sort snapshots newest-first.
func TestSnapshotIndexManifestSortedNewestFirst(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("https://example.com/a.html", "20210101000000") // oldest
	idx.Register("https://example.com/b.html", "20230101000000") // newest
	idx.Register("https://example.com/c.html", "20220101000000") // middle

	m := idx.GetManifest()
	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m))
	}
	for i := 1; i < len(m); i++ {
		if m[i].Timestamp > m[i-1].Timestamp {
			t.Errorf("manifest not sorted newest-first at index %d: %q > %q",
				i, m[i].Timestamp, m[i-1].Timestamp)
		}
	}
}

// Calling GetManifest twice must return the same length (idempotent build).
func TestSnapshotIndexManifestIdempotent(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("https://example.com/page.html", "20230101000000")

	m1 := idx.GetManifest()
	m2 := idx.GetManifest()
	if len(m1) != len(m2) {
		t.Errorf("GetManifest not idempotent: first=%d second=%d", len(m1), len(m2))
	}
}

// Resolve must return the registered timestamp for an exact path+query match.
func TestSnapshotIndexResolveExactQueryMatch(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("https://example.com/search?q=go", "20230601000000")

	ts := idx.Resolve("https://example.com/search?q=go", "fallback")
	if ts != "20230601000000" {
		t.Errorf("expected exact query timestamp, got %q", ts)
	}
}

// Resolve must fall back to the path-only timestamp when the query differs.
func TestSnapshotIndexResolveFallsBackToPath(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("https://example.com/page.html", "20230101000000")

	// Different query → no query match → fall back to /page.html path entry.
	ts := idx.Resolve("https://example.com/page.html?v=2", "fallback")
	if ts != "20230101000000" {
		t.Errorf("expected path-fallback timestamp, got %q", ts)
	}
}

// Resolve must return the caller-supplied default for an unknown URL.
func TestSnapshotIndexResolveFallbackDefault(t *testing.T) {
	idx := NewSnapshotIndex()

	ts := idx.Resolve("https://example.com/unknown.html", "mydefault")
	if ts != "mydefault" {
		t.Errorf("expected fallback default, got %q", ts)
	}
}

// Resolve without a prior GetManifest call must still work (builds lazily).
func TestSnapshotIndexResolveWithoutGetManifest(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("https://example.com/page.html", "20230101000000")

	ts := idx.Resolve("https://example.com/page.html", "fallback")
	if ts != "20230101000000" {
		t.Errorf("expected registered timestamp via lazy build, got %q", ts)
	}
}

// Register with a malformed URL must be silently ignored (no panic).
func TestSnapshotIndexRegisterInvalidURL(t *testing.T) {
	idx := NewSnapshotIndex()
	idx.Register("://bad url", "20230101000000")

	if m := idx.GetManifest(); len(m) != 0 {
		t.Errorf("invalid URL should not be registered, got %d entries", len(m))
	}
}
