package wayback

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// ProcessHTML rewrites links and canonical tags in an HTML file.
func ProcessHTML(filePath, pageURL string, cfg *Config, idx *SnapshotIndex) error {
	data, err := os.ReadFile(filePath) //nolint:gosec // G304: path is written by this program
	if err != nil {
		return err
	}

	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return err
	}

	pageU, err := url.Parse(pageURL)
	if err != nil {
		return err
	}

	// Relative directory of the output file (used for RelativeLink)
	localDir := ToPosix(filepath.ToSlash(filepath.Dir(filePath)))

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a", "form":
				rewriteAttr(n, attrName(n.Data), pageU, localDir, cfg, idx, false)

			case "img", "script", "iframe", "source", "video", "audio":
				rewriteAttr(n, "src", pageU, localDir, cfg, idx, true)

			case "link":
				if isCanonical(n) {
					if cfg.CanonicalAction == "remove" {
						removeNode(n)
						return
					}
				} else {
					rewriteAttr(n, "href", pageU, localDir, cfg, idx, true)
				}

			case "style":
				rewriteStyleNode(n, pageURL, cfg, idx)

			case "base":
				// Do not touch <base>
			}

			// Inline style attribute
			for i, a := range n.Attr {
				if a.Key == "style" {
					n.Attr[i].Val = RewriteCSSContent(a.Val, pageURL, cfg, idx)
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return err
	}
	return os.WriteFile(filePath, buf.Bytes(), 0600)
}

// attrName returns the relevant URL attribute for a given tag name.
func attrName(tag string) string {
	if tag == "form" {
		return "action"
	}
	return "href"
}

// isCanonical returns true for <link rel="canonical">.
func isCanonical(n *html.Node) bool {
	for _, a := range n.Attr {
		if a.Key == "rel" && strings.ToLower(strings.TrimSpace(a.Val)) == "canonical" {
			return true
		}
	}
	return false
}

// removeNode detaches a node from the tree.
func removeNode(n *html.Node) {
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}

// rewriteAttr resolves and rewrites the specified attribute value.
// isAsset controls whether the link is treated as a navigable page (anchor)
// or an embedded asset (img, script, etc.).
func rewriteAttr(n *html.Node, attr string, pageU *url.URL, localDir string,
	cfg *Config, idx *SnapshotIndex, isAsset bool) {

	for i, a := range n.Attr {
		if a.Key != attr {
			continue
		}
		val := strings.TrimSpace(a.Val)
		if val == "" || strings.HasPrefix(val, "#") ||
			strings.HasPrefix(val, "javascript:") || strings.HasPrefix(val, "data:") ||
			strings.HasPrefix(val, "mailto:") {
			return
		}

		resolved, err := pageU.Parse(val)
		if err != nil {
			return
		}
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return
		}

		internal := isInternalHost(resolved.Host, cfg.BareHost)
		if !internal {
			// External asset: optionally queue download; leave link as-is for now
			return
		}

		// Build local file path for the resolved URL
		localTarget := URLToLocalPath(resolved.String())
		localTarget = filepath.Join(cfg.Directory, filepath.FromSlash(localTarget))
		localTarget = ToPosix(localTarget)

		rel := RelativeLink(localDir, localTarget)
		n.Attr[i].Val = rel
		return
	}
}

// rewriteStyleNode rewrites URLs inside an inline <style> block.
func rewriteStyleNode(n *html.Node, pageURL string, cfg *Config, idx *SnapshotIndex) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			c.Data = RewriteCSSContent(c.Data, pageURL, cfg, idx)
		}
	}
}
