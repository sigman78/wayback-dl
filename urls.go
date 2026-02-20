package main

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"

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

// EnsureLocalTarget converts a bare directory path to an index file path.
// e.g. "dir/" → "dir/index.html", "dir" → "dir/index.html", "" → "index.html"
func EnsureLocalTarget(pathname string) string {
	if pathname == "" || strings.HasSuffix(pathname, "/") {
		return pathname + "index.html"
	}
	// If no extension treat it as a directory
	if path.Ext(pathname) == "" {
		return pathname + "/index.html"
	}
	return pathname
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

// WindowsSanitize escapes characters that are illegal in Windows filenames.
// The path argument is always a relative URL-derived fragment (never an absolute
// OS path with a drive letter), so all colons are safe to escape.
// On non-Windows platforms it is a no-op.
func WindowsSanitize(p string) string {
	if runtime.GOOS != "windows" {
		return p
	}
	replacer := strings.NewReplacer(
		":", "%3A",
		"*", "%2A",
		"?", "%3F",
		"<", "%3C",
		">", "%3E",
		"|", "%7C",
	)
	return replacer.Replace(p)
}

// URLToLocalPath converts an absolute URL to a relative filesystem path fragment
// (no leading slash) suitable for joining with the output directory.
func URLToLocalPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}

	// Use Hostname() (no port) and strip default ports to keep directory names clean.
	host := u.Hostname()
	port := u.Port()
	switch {
	case port == "80" && u.Scheme == "http":
		// default HTTP port — omit
	case port == "443" && u.Scheme == "https":
		// default HTTPS port — omit
	case port != "":
		// non-default port: append with encoded colon so Windows stays happy
		host = host + "%3A" + port
	}

	p := host + u.Path
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	p = strings.ReplaceAll(p, "//", "/")
	p = EnsureLocalTarget(p)
	p = WindowsSanitize(p)
	return p
}
