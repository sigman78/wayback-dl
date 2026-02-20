package wayback

import (
	"testing"
)

func TestURLToLocalPathPretty(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		// Root → index.html
		{"https://example.com/", "index.html"},
		// Trailing slash directory
		{"https://example.com/page/", "page/index.html"},
		// Extension-less path treated as directory (pretty mode)
		{"https://example.com/dir/about", "dir/about/index.html"},
		// File with extension kept as-is
		{"https://example.com/style.css", "style.css"},
		{"https://example.com/img/photo.jpg", "img/photo.jpg"},
		// Query with extension: suffix inserted before extension
		{"https://example.com/img/photo.jpg?v=2", "img/photo_v_2.jpg"},
		// Multiple query params with extension
		{"https://example.com/file.css?v=3&t=min", "file_v_3_t_min.css"},
	}

	for _, tc := range cases {
		got := URLToLocalPath(tc.url, true)
		if got != tc.want {
			t.Errorf("URLToLocalPath(%q, pretty)\n  got  %q\n  want %q", tc.url, got, tc.want)
		}
	}
}

func TestURLToLocalPathPreserve(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		// Root still → index.html
		{"https://example.com/", "index.html"},
		// Trailing slash still → index.html inside directory
		{"https://example.com/page/", "page/index.html"},
		// Extension-less segment kept as plain file (not a directory)
		{"https://example.com/dir/about", "dir/about"},
		// Extension-less with query → file with query suffix
		{"https://example.com/search?q=go", "search_q_go"},
		// File with extension unchanged
		{"https://example.com/style.css", "style.css"},
		{"https://example.com/img/photo.jpg", "img/photo.jpg"},
		// Query with extension: suffix before extension
		{"https://example.com/img/photo.jpg?v=2", "img/photo_v_2.jpg"},
	}

	for _, tc := range cases {
		got := URLToLocalPath(tc.url, false)
		if got != tc.want {
			t.Errorf("URLToLocalPath(%q, preserve)\n  got  %q\n  want %q", tc.url, got, tc.want)
		}
	}
}

func TestURLToLocalPathQuerySanitize(t *testing.T) {
	// Extension-less path with query → index_<query>.html in pretty mode.
	got := URLToLocalPath("https://example.com/search?q=go", true)
	want := "search/index_q_go.html"
	if got != want {
		t.Errorf("URLToLocalPath with query (pretty)\n  got  %q\n  want %q", got, want)
	}
}
