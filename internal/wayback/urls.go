package wayback

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	sanitize "github.com/mrz1836/go-sanitize"
	"golang.org/x/net/idna"
)

// NormalizedBase holds the canonical form and all URL variants for a base URL.
type NormalizedBase struct {
	CanonicalURL string
	Variants     []string // all http/https + www combinations
	BareHost     string   // hostname without www.
	UnicodeHost  string   // IDN-decoded hostname
}

// NormalizeBaseURL parses and normalises the user-supplied URL/domain input.
func NormalizeBaseURL(input string) (*NormalizedBase, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty URL")
	}
	// Auto-prepend scheme if missing
	if !strings.Contains(input, "://") {
		input = "https://" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	// Strip www. for bare host
	bareHost := host
	if strings.HasPrefix(strings.ToLower(bareHost), "www.") {
		bareHost = bareHost[4:]
	}

	// IDN decode for unicode host
	unicodeHost := bareHost
	if decoded, err := idna.ToUnicode(bareHost); err == nil {
		unicodeHost = decoded
	}

	urlPath := u.Path
	if urlPath == "" {
		urlPath = "/"
	}

	// Build all http/https × bare/www variants
	schemes := []string{"https", "http"}
	hostVariants := []string{bareHost, "www." + bareHost}
	var variants []string
	for _, s := range schemes {
		for _, h := range hostVariants {
			v := s + "://" + h + urlPath
			if u.RawQuery != "" {
				v += "?" + u.RawQuery
			}
			variants = append(variants, v)
		}
	}

	canonical := "https://" + host + urlPath
	if u.RawQuery != "" {
		canonical += "?" + u.RawQuery
	}

	return &NormalizedBase{
		CanonicalURL: canonical,
		Variants:     variants,
		BareHost:     bareHost,
		UnicodeHost:  unicodeHost,
	}, nil
}

// IsHTMLFile returns true when the path/content-type/magic bytes indicate HTML.
func IsHTMLFile(filePath, contentType string, firstBytes []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") {
		return true
	}
	ext := strings.ToLower(path.Ext(filePath))
	if ext == ".html" || ext == ".htm" {
		return true
	}
	// magic: look for a leading BOM or <
	if len(firstBytes) > 0 {
		b := firstBytes
		// skip BOM
		if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
			b = b[3:]
		}
		trimmed := strings.TrimSpace(string(b))
		if strings.HasPrefix(trimmed, "<") {
			return true
		}
	}
	return false
}

// IsCSSResource returns true when the path/content-type indicates CSS.
func IsCSSResource(filePath, contentType string) bool {
	if strings.Contains(strings.ToLower(contentType), "text/css") {
		return true
	}
	return strings.ToLower(path.Ext(filePath)) == ".css"
}

// RelativeLink returns the relative path from fromDir to toFile.
func RelativeLink(fromDir, toFile string) string {
	rel, err := filepath.Rel(filepath.FromSlash(fromDir), filepath.FromSlash(toFile))
	if err != nil {
		return toFile
	}
	return ToPosix(rel)
}

// ToPosix converts backslashes to forward slashes.
func ToPosix(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// URLToLocalPath converts an absolute URL to a relative filesystem path fragment
// (no leading slash) suitable for joining with the output directory.
//
// When pretty is true, extension-less last segments are treated as directories
// and resolved to index.html (e.g. /about → about/index.html).
// When pretty is false, the original URL path structure is preserved: an
// extension-less segment is kept as a plain file (e.g. /about → about).
//
// In both modes:
//   - Each path segment is sanitized via sanitize.PathName (safe on Windows and
//     any other OS); the file extension is separated first so the dot is never
//     stripped.
//   - Query parameters are decoded, key/value separators replaced with "_", and
//     the result is appended before the file extension to keep names unique.
//   - Explicit directory paths (trailing slash or empty) always resolve to
//     index[_query].html inside the directory.
func URLToLocalPath(rawURL string, pretty bool) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}

	isDir := u.Path == "" || strings.HasSuffix(u.Path, "/")

	// Sanitize each path segment independently so we can handle the
	// extension dot explicitly (sanitize.PathName strips dots).
	var segments []string
	for _, seg := range strings.Split(strings.Trim(u.Path, "/"), "/") {
		if seg == "" {
			continue
		}
		if s := sanitizeSegment(seg); s != "" {
			segments = append(segments, s)
		}
	}

	var dirSegs []string
	var filename string

	switch {
	case isDir || len(segments) == 0:
		// Explicit directory or empty path: every segment is a directory.
		dirSegs = segments
		filename = buildIndexName(u.RawQuery)
	default:
		last := segments[len(segments)-1]
		ext := path.Ext(last)
		if ext == "" {
			if pretty {
				// Pretty: extension-less last segment → implicit directory.
				dirSegs = segments
				filename = buildIndexName(u.RawQuery)
			} else {
				// Preserve: extension-less last segment stays as a plain file.
				dirSegs = segments[:len(segments)-1]
				filename = buildFileName(last, "", u.RawQuery)
			}
		} else {
			dirSegs = segments[:len(segments)-1]
			filename = buildFileName(last, ext, u.RawQuery)
		}
	}

	if len(dirSegs) > 0 {
		return strings.Join(dirSegs, "/") + "/" + filename
	}
	return filename
}

// sanitizeSegment sanitizes a single URL path segment.
// The extension is split off first and sanitized separately so it is
// never discarded by PathName (which strips dots).
func sanitizeSegment(seg string) string {
	ext := path.Ext(seg)
	if ext == "" {
		return sanitize.PathName(seg)
	}
	base := sanitize.PathName(seg[:len(seg)-len(ext)])
	extPart := sanitize.PathName(ext[1:]) // strip leading dot before sanitizing
	if base == "" {
		base = "file"
	}
	if extPart == "" {
		return base
	}
	return base + "." + extPart
}

// buildIndexName returns "index[_querySuffix].html".
func buildIndexName(rawQuery string) string {
	return "index" + urlQuerySuffix(rawQuery) + ".html"
}

// buildFileName inserts the query suffix before the file extension so the
// extension is always the final component of the filename.
func buildFileName(sanitizedSeg, ext, rawQuery string) string {
	base := sanitizedSeg[:len(sanitizedSeg)-len(ext)]
	return base + urlQuerySuffix(rawQuery) + ext
}

// urlQuerySuffix converts a raw URL query string into a filesystem-safe
// "_key_value" suffix, or "" when the query is empty.
// Key/value separators (= &) are replaced with underscores before PathName
// strips any remaining unsafe characters.
func urlQuerySuffix(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(rawQuery)
	if err != nil {
		decoded = rawQuery
	}
	q := strings.NewReplacer("=", "_", "&", "_").Replace(decoded)
	s := sanitize.PathName(q)
	if s == "" {
		return ""
	}
	return "_" + s
}
