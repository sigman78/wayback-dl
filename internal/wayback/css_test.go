package wayback

import (
	"strings"
	"testing"
)

// testCSSCfg returns a minimal Config sufficient for CSS rewriting tests.
func testCSSCfg() *Config {
	return &Config{
		BareHost:  "example.com",
		Directory: "websites/example.com",
	}
}

func TestRewriteCSSDoubleQuotedURL(t *testing.T) {
	cfg := testCSSCfg()
	idx := NewSnapshotIndex()

	css := `body { background: url("http://example.com/images/bg.png"); }`
	got := RewriteCSSContent(css, "http://example.com/style.css", cfg, idx)

	if !strings.Contains(got, `url("images/bg.png")`) {
		t.Errorf("double-quoted url() not rewritten to relative path\n  got: %s", got)
	}
	if strings.Contains(got, "http://example.com") {
		t.Errorf("absolute URL should have been removed\n  got: %s", got)
	}
}

func TestRewriteCSSSingleQuotedImport(t *testing.T) {
	cfg := testCSSCfg()
	idx := NewSnapshotIndex()

	css := `@import 'http://example.com/fonts/main.css';`
	got := RewriteCSSContent(css, "http://example.com/style.css", cfg, idx)

	if !strings.Contains(got, `@import 'fonts/main.css'`) {
		t.Errorf("single-quoted @import not rewritten\n  got: %s", got)
	}
}

func TestRewriteCSSBareURL(t *testing.T) {
	cfg := testCSSCfg()
	idx := NewSnapshotIndex()

	css := `.icon { background: url(http://example.com/img/logo.png); }`
	got := RewriteCSSContent(css, "http://example.com/style.css", cfg, idx)

	if !strings.Contains(got, "url(img/logo.png)") {
		t.Errorf("bare url() not rewritten\n  got: %s", got)
	}
}

func TestRewriteCSSDoubleQuotedImport(t *testing.T) {
	cfg := testCSSCfg()
	idx := NewSnapshotIndex()

	css := `@import "http://example.com/theme/base.css";`
	got := RewriteCSSContent(css, "http://example.com/style.css", cfg, idx)

	if !strings.Contains(got, `@import "theme/base.css"`) {
		t.Errorf("double-quoted @import not rewritten\n  got: %s", got)
	}
}

func TestRewriteCSSExternalURLUntouched(t *testing.T) {
	cfg := testCSSCfg() // DownloadExternalAssets defaults to false
	idx := NewSnapshotIndex()

	css := `body { background: url("https://cdn.other.com/bg.png"); }`
	got := RewriteCSSContent(css, "http://example.com/style.css", cfg, idx)

	if !strings.Contains(got, "cdn.other.com") {
		t.Errorf("external URL should be left unchanged\n  got: %s", got)
	}
}

func TestRewriteCSSDataURIUntouched(t *testing.T) {
	cfg := testCSSCfg()
	idx := NewSnapshotIndex()

	css := `body { background: url("data:image/png;base64,abc123"); }`
	got := RewriteCSSContent(css, "http://example.com/style.css", cfg, idx)

	if !strings.Contains(got, "data:image/png") {
		t.Errorf("data: URI should be left unchanged\n  got: %s", got)
	}
}
