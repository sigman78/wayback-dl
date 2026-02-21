package wayback

// Rewriter rewrites the content of a stored resource in-place.
type Rewriter interface {
	Rewrite(store Storage, logicalPath, pageURL string, cfg *Config, idx *SnapshotIndex) error
}

// DetectRewriter returns the Rewriter appropriate for the given resource,
// or nil when no rewriting is needed.
// Detection order mirrors the existing inline checks:
//
//	Content-Type -> file extension -> magic bytes (HTML only).
func DetectRewriter(logicalPath, contentType string, firstBytes []byte) Rewriter {
	if IsHTMLFile(logicalPath, contentType, firstBytes) {
		return HTMLRewriter{}
	}
	if IsCSSResource(logicalPath, contentType) {
		return CSSRewriter{}
	}
	return nil
}
