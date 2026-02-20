package wayback

import (
	"runtime"
	"testing"
)

func TestURLToLocalPath(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		// Root → index.html
		{"https://example.com/", "index.html"},
		// Trailing slash directory
		{"https://example.com/page/", "page/index.html"},
		// Extension-less path treated as directory
		{"https://example.com/dir/about", "dir/about/index.html"},
		// File with extension kept as-is
		{"https://example.com/style.css", "style.css"},
		{"https://example.com/img/photo.jpg", "img/photo.jpg"},
	}

	for _, tc := range cases {
		got := URLToLocalPath(tc.url)
		if got != tc.want {
			t.Errorf("URLToLocalPath(%q)\n  got  %q\n  want %q", tc.url, got, tc.want)
		}
	}
}

func TestURLToLocalPathQuerySanitize(t *testing.T) {
	// The query string is appended after EnsureLocalTarget; on Windows the
	// literal '?' is escaped to %3F by WindowsSanitize.
	got := URLToLocalPath("https://example.com/search?q=go")
	var want string
	if runtime.GOOS == "windows" {
		want = "search/index.html%3Fq=go"
	} else {
		want = "search/index.html?q=go"
	}
	if got != want {
		t.Errorf("URLToLocalPath with query\n  got  %q\n  want %q", got, want)
	}
}

func TestEnsureLocalTarget(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "index.html"},          // empty → root index
		{"dir/", "dir/index.html"},  // trailing slash
		{"page", "page/index.html"}, // no extension → treated as directory
		{"style.css", "style.css"},  // has extension → unchanged
		{"img/photo.jpg", "img/photo.jpg"},
	}

	for _, tc := range cases {
		got := EnsureLocalTarget(tc.in)
		if got != tc.want {
			t.Errorf("EnsureLocalTarget(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}

func TestWindowsSanitize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WindowsSanitize is a no-op on non-Windows platforms")
	}

	cases := []struct {
		in   string
		want string
	}{
		{"normal/path/file.html", "normal/path/file.html"},
		{"page?q=go", "page%3Fq=go"},
		{"file:name", "file%3Aname"},
		{"a*b", "a%2Ab"},
		{"a<b>c", "a%3Cb%3Ec"},
		{"a|b", "a%7Cb"},
	}

	for _, tc := range cases {
		got := WindowsSanitize(tc.in)
		if got != tc.want {
			t.Errorf("WindowsSanitize(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}
