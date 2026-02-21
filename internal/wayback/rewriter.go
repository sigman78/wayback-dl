package wayback

// Rewriter detects and rewrites a stored resource in-place.
type Rewriter interface {
	// Match reports whether this rewriter handles the given resource.
	Match(logicalPath, contentType string, firstBytes []byte) bool
	// Rewrite rewrites the resource in storage.
	Rewrite(store Storage, logicalPath, pageURL string, cfg *Config, idx *SnapshotIndex) error
}

// rewriters is the ordered list of all known rewriter types.
// DetectRewriter tries them in order and returns the first match.
var rewriters = []Rewriter{HTMLRewriter{}, CSSRewriter{}}

// DetectRewriter returns the Rewriter appropriate for the given resource,
// or nil when no rewriting is needed.
func DetectRewriter(logicalPath, contentType string, firstBytes []byte) Rewriter {
	for _, rw := range rewriters {
		if rw.Match(logicalPath, contentType, firstBytes) {
			return rw
		}
	}
	return nil
}
