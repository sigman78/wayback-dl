package main

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// Three patterns for url(): double-quoted, single-quoted, unquoted
	reURLDouble  = regexp.MustCompile(`(?i)url\(\s*"([^"]+)"\s*\)`)
	reURLSingle  = regexp.MustCompile(`(?i)url\(\s*'([^']+)'\s*\)`)
	reURLBare    = regexp.MustCompile(`(?i)url\(\s*([^)'"]+?)\s*\)`)
	reImportDbl  = regexp.MustCompile(`(?i)@import\s+"([^"]+)"`)
	reImportSgl  = regexp.MustCompile(`(?i)@import\s+'([^']+)'`)
)

// RewriteCSSContent rewrites url() and @import references in CSS text.
func RewriteCSSContent(css, pageURL string, cfg *Config, idx *SnapshotIndex) string {
	pageU, err := url.Parse(pageURL)
	if err != nil {
		return css
	}

	// Compute local directory of the page file for RelativeLink
	localPath := URLToLocalPath(pageURL)
	localPath = filepath.Join(cfg.Directory, filepath.FromSlash(localPath))
	localDir := ToPosix(filepath.ToSlash(filepath.Dir(localPath)))

	replace := func(src, ref string) string {
		ref = strings.TrimSpace(ref)
		if ref == "" ||
			strings.HasPrefix(ref, "data:") ||
			strings.HasPrefix(ref, "javascript:") ||
			strings.HasPrefix(ref, "#") {
			return src
		}

		resolved, err := pageU.Parse(ref)
		if err != nil {
			return src
		}
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return src
		}

		if !isInternalHost(resolved.Host, cfg.BareHost) {
			if !cfg.DownloadExternalAssets {
				return src
			}
			// External asset rewriting not implemented; leave as-is
			return src
		}

		localTarget := URLToLocalPath(resolved.String())
		localTarget = filepath.Join(cfg.Directory, filepath.FromSlash(localTarget))
		localTarget = ToPosix(localTarget)

		rel := RelativeLink(localDir, localTarget)
		return strings.Replace(src, ref, rel, 1)
	}

	// Rewrite url(...) — double-quoted, single-quoted, then bare
	rewriteURLRegex := func(re *regexp.Regexp) {
		css = re.ReplaceAllStringFunc(css, func(match string) string {
			sub := re.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			return replace(match, sub[1])
		})
	}
	rewriteURLRegex(reURLDouble)
	rewriteURLRegex(reURLSingle)
	rewriteURLRegex(reURLBare)

	// Rewrite @import "..." / @import '...'
	rewriteImportRegex := func(re *regexp.Regexp) {
		css = re.ReplaceAllStringFunc(css, func(match string) string {
			sub := re.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			return replace(match, sub[1])
		})
	}
	rewriteImportRegex(reImportDbl)
	rewriteImportRegex(reImportSgl)

	return css
}

// RewriteCSSFile reads a CSS file, rewrites its URLs, and writes it back.
func RewriteCSSFile(filePath, pageURL string, cfg *Config, idx *SnapshotIndex) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	rewritten := RewriteCSSContent(string(data), pageURL, cfg, idx)
	return os.WriteFile(filePath, []byte(rewritten), 0644)
}
