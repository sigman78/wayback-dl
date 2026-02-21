package wayback

import (
	"strings"
	"testing"
)

// processHTMLInTemp writes htmlContent into a LocalStorage backed by a temp
// directory, runs ProcessHTML, and returns the rewritten file contents.
func processHTMLInTemp(t *testing.T, htmlContent, pageURL string, cfg *Config) string {
	t.Helper()
	store := NewLocalStorage(t.TempDir())
	if err := store.PutBytes("test.html", []byte(htmlContent)); err != nil {
		t.Fatalf("write test HTML: %v", err)
	}

	idx := NewSnapshotIndex()
	if err := (HTMLRewriter{}).Rewrite(store, "test.html", pageURL, cfg, idx); err != nil {
		t.Fatalf("HTMLRewriter.Rewrite: %v", err)
	}

	got, err := store.Get("test.html")
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	return string(got)
}

func testHTMLCfg() *Config {
	return &Config{
		BareHost:        "example.com",
		CanonicalAction: "keep",
	}
}

// <a href> pointing at the same host must be rewritten to a relative path.
func TestProcessHTMLAnchorHref(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><body><a href="http://example.com/about/">About</a></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "http://example.com") {
		t.Errorf("absolute URL should have been rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, `href="about/index.html"`) {
		t.Errorf("expected relative href\n  got: %s", out)
	}
}

// <img src> must be rewritten to a relative path.
func TestProcessHTMLImgSrc(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><body><img src="http://example.com/images/logo.png"/></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if !strings.Contains(out, `src="images/logo.png"`) {
		t.Errorf("img src not rewritten\n  got: %s", out)
	}
}

// <script src> must be rewritten.
func TestProcessHTMLScriptSrc(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><head><script src="http://example.com/js/app.js"></script></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if !strings.Contains(out, `src="js/app.js"`) {
		t.Errorf("script src not rewritten\n  got: %s", out)
	}
}

// Non-canonical <link href> (e.g. stylesheet) must be rewritten.
func TestProcessHTMLLinkStylesheet(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><head><link rel="stylesheet" href="http://example.com/style.css"/></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if !strings.Contains(out, `href="style.css"`) {
		t.Errorf("link stylesheet href not rewritten\n  got: %s", out)
	}
}

// <form action> must be rewritten (preserve mode: extension-less → plain file).
func TestProcessHTMLFormAction(t *testing.T) {
	cfg := testHTMLCfg() // PrettyPath defaults to false
	in := `<html><body><form action="http://example.com/submit"></form></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "http://example.com") {
		t.Errorf("form action not rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, `action="submit"`) {
		t.Errorf("expected relative form action\n  got: %s", out)
	}
}

// <form action> in pretty mode: extension-less → dir/index.html.
func TestProcessHTMLFormActionPretty(t *testing.T) {
	cfg := testHTMLCfg()
	cfg.PrettyPath = true
	in := `<html><body><form action="http://example.com/submit"></form></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "http://example.com") {
		t.Errorf("form action not rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, `action="submit/index.html"`) {
		t.Errorf("expected pretty relative form action\n  got: %s", out)
	}
}

// <link rel="canonical"> must be removed when CanonicalAction == "remove".
func TestProcessHTMLCanonicalRemoved(t *testing.T) {
	cfg := testHTMLCfg()
	cfg.CanonicalAction = "remove"
	in := `<html><head><link rel="canonical" href="http://example.com/"/></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "canonical") {
		t.Errorf("canonical link should have been removed\n  got: %s", out)
	}
}

// <link rel="canonical"> must be left in place when CanonicalAction == "keep".
func TestProcessHTMLCanonicalKept(t *testing.T) {
	cfg := testHTMLCfg()
	cfg.CanonicalAction = "keep"
	in := `<html><head><link rel="canonical" href="http://example.com/"/></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if !strings.Contains(out, "canonical") {
		t.Errorf("canonical link should have been kept\n  got: %s", out)
	}
}

// Links pointing at external hosts must not be rewritten.
func TestProcessHTMLExternalLinkUntouched(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><body><a href="https://other.com/page">External</a></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if !strings.Contains(out, "https://other.com/page") {
		t.Errorf("external link should be unchanged\n  got: %s", out)
	}
}

// javascript:, mailto:, and fragment (#) hrefs must be left as-is.
func TestProcessHTMLSpecialSchemesUntouched(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><body>` +
		`<a href="javascript:void(0)">JS</a>` +
		`<a href="mailto:user@example.com">Mail</a>` +
		`<a href="#section">Anchor</a>` +
		`</body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if !strings.Contains(out, "javascript:void(0)") {
		t.Errorf("javascript: link should be untouched\n  got: %s", out)
	}
	if !strings.Contains(out, "mailto:user@example.com") {
		t.Errorf("mailto: link should be untouched\n  got: %s", out)
	}
	if !strings.Contains(out, "#section") {
		t.Errorf("fragment link should be untouched\n  got: %s", out)
	}
}

// <link href> with cache-busting query — non-pretty mode.
// Local file is style.css%3FHASH; href must use %253F so browsers
// decode once to %3F and find the file.
func TestProcessHTMLLinkQueryRaw(t *testing.T) {
	cfg := testHTMLCfg() // PrettyPath = false
	in := `<html><head><link rel="stylesheet" href="style.css?fbc4e9ea0c35466f02ad5a4e811ec7ae"/></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "style.css?") {
		t.Errorf("raw query URL should have been rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, `href="style.css%253Ffbc4e9ea0c35466f02ad5a4e811ec7ae"`) {
		t.Errorf("expected %%253F-encoded local path\n  got: %s", out)
	}
}

// <script src> with cache-busting query — non-pretty mode.
func TestProcessHTMLScriptQueryRaw(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><head><script src="main.js?f6d367ed67e79dca2aae2cc22f551a87"></script></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "main.js?") {
		t.Errorf("raw query URL should have been rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, `src="main.js%253Ff6d367ed67e79dca2aae2cc22f551a87"`) {
		t.Errorf("expected %%253F-encoded local path\n  got: %s", out)
	}
}

// <link href> with cache-busting query — pretty mode.
// URLToLocalPath embeds query cleanly: style_HASH.css (no %).
func TestProcessHTMLLinkQueryPretty(t *testing.T) {
	cfg := testHTMLCfg()
	cfg.PrettyPath = true
	in := `<html><head><link rel="stylesheet" href="style.css?fbc4e9ea0c35466f02ad5a4e811ec7ae"/></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "style.css?") {
		t.Errorf("raw query URL should have been rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, `href="style_fbc4e9ea0c35466f02ad5a4e811ec7ae.css"`) {
		t.Errorf("expected pretty local path\n  got: %s", out)
	}
}

// <script src> with cache-busting query — pretty mode.
func TestProcessHTMLScriptQueryPretty(t *testing.T) {
	cfg := testHTMLCfg()
	cfg.PrettyPath = true
	in := `<html><head><script src="main.js?f6d367ed67e79dca2aae2cc22f551a87"></script></head><body></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "main.js?") {
		t.Errorf("raw query URL should have been rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, `src="main_f6d367ed67e79dca2aae2cc22f551a87.js"`) {
		t.Errorf("expected pretty local path\n  got: %s", out)
	}
}

// Inline style attributes must have their url() references rewritten.
func TestProcessHTMLInlineStyleRewritten(t *testing.T) {
	cfg := testHTMLCfg()
	in := `<html><body><div style="background: url('http://example.com/bg.png')"></div></body></html>`
	out := processHTMLInTemp(t, in, "http://example.com/", cfg)

	if strings.Contains(out, "http://example.com") {
		t.Errorf("inline style URL not rewritten\n  got: %s", out)
	}
	if !strings.Contains(out, "bg.png") {
		t.Errorf("rewritten filename not found in inline style\n  got: %s", out)
	}
}
