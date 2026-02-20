package wayback

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Pretty mode (URLToLocalPath with pretty=true)
// ---------------------------------------------------------------------------

func TestURLToLocalPathPretty(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		// Root → index.html
		{"https://example.com/", "index.html"},
		// Trailing slash → directory + index.html
		{"https://example.com/page/", "page/index.html"},
		// Extension-less last segment → implicit directory
		{"https://example.com/dir/about", "dir/about/index.html"},
		// File with extension preserved
		{"https://example.com/style.css", "style.css"},
		{"https://example.com/img/photo.jpg", "img/photo.jpg"},
		// Query with extension: suffix inserted before extension
		{"https://example.com/img/photo.jpg?v=2", "img/photo_v_2.jpg"},
		// Multiple query params with extension
		{"https://example.com/file.css?v=3&t=min", "file_v_3_t_min.css"},
		// Fragment (#) is stripped; only query matters
		{"https://example.com/page.html#section", "page.html"},
		{"https://example.com/search?q=go#hash", "search/index_q_go.html"},
		// URL-encoded path: url.Parse decodes %20 → space; PathName strips space
		{"https://example.com/path%20spaces/file.html", "pathspaces/file.html"},
		// URL-encoded query value: decoded then PathName-normalised
		{"https://example.com/page.html?q=hello%20world", "page_q_helloworld.html"},
		// Query with = and & become underscores
		{"https://example.com/page.html?lang=en&page=2", "page_lang_en_page_2.html"},
		// Deep path
		{"https://example.com/a/b/c/page.html", "a/b/c/page.html"},
		// Extension-less in subdirectory → each becomes a directory component
		{"https://example.com/a/b/about", "a/b/about/index.html"},
		// Extension-less root path
		{"https://example.com/about", "about/index.html"},
		// Query on directory path
		{"https://example.com/dir/?q=search", "dir/index_q_search.html"},
		// Root with query
		{"https://example.com/?q=search", "index_q_search.html"},
	}

	for _, tc := range cases {
		got := URLToLocalPath(tc.url, true)
		if got != tc.want {
			t.Errorf("URLToLocalPath(%q, pretty)\n  got  %q\n  want %q", tc.url, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Preserve mode (URLToLocalPath with pretty=false, the default)
// ---------------------------------------------------------------------------

func TestURLToLocalPathPreserve(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		// Root → index.html
		{"https://example.com/", "index.html"},
		// Trailing slash → dir/index.html
		{"https://example.com/page/", "page/index.html"},
		// Extension-less segment stays as a plain file
		{"https://example.com/dir/about", "dir/about"},
		// Extension-less root segment
		{"https://example.com/about", "about"},
		// File with extension unchanged
		{"https://example.com/style.css", "style.css"},
		{"https://example.com/img/photo.jpg", "img/photo.jpg"},
		// Query appended after full filename with %3F
		{"https://example.com/img/photo.jpg?v=2", "img/photo.jpg%3Fv=2"},
		// Multiple query params: & preserved, = preserved
		{"https://example.com/page.html?lang=en&page=1", "page.html%3Flang=en&page=1"},
		// Extension-less with query
		{"https://example.com/search?q=go", "search%3Fq=go"},
		// Fragment stripped, no query
		{"https://example.com/page.html#section", "page.html"},
		// Fragment stripped, query kept
		{"https://example.com/page.html?q=go#hash", "page.html%3Fq=go"},
		// Fragment only on extension-less path
		{"https://example.com/dir/about#section", "dir/about"},
		// URL-encoded path chars preserved (%20 stays as %20)
		{"https://example.com/path%20with%20spaces/file.html", "path%20with%20spaces/file.html"},
		// URL-encoded query value preserved
		{"https://example.com/page.html?q=hello%20world", "page.html%3Fq=hello%20world"},
		// Windows-unsafe char in query: * is encoded
		{"https://example.com/file?q=a*b", "file%3Fq=a%2Ab"},
		// Colon in query value encoded
		{"https://example.com/page.html?time=12:30", "page.html%3Ftime=12%3A30"},
		// Root with query
		{"https://example.com/?q=search", "index.html%3Fq=search"},
		// Directory with query
		{"https://example.com/dir/?q=search", "dir/index.html%3Fq=search"},
		// Deep path with query
		{"https://example.com/a/b/c/page.html?v=1", "a/b/c/page.html%3Fv=1"},
		// URL-encoded segment (e.g. non-ASCII percent-encoded)
		{"https://example.com/caf%C3%A9/menu.html", "caf%C3%A9/menu.html"},
	}

	for _, tc := range cases {
		got := URLToLocalPath(tc.url, false)
		if got != tc.want {
			t.Errorf("URLToLocalPath(%q, preserve)\n  got  %q\n  want %q", tc.url, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// encodeForFS: filesystem-safe percent-encoding
// ---------------------------------------------------------------------------

func TestEncodeForFS(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Plain paths and filenames are unchanged
		{"normal/path/file.html", "normal/path/file.html"},
		{"simple.txt", "simple.txt"},
		{"normal-chars_ok", "normal-chars_ok"},
		// Existing %xx sequences are not re-encoded (% is not forbidden)
		{"%already-encoded", "%already-encoded"},
		{"path%20with%20spaces", "path%20with%20spaces"},
		// Windows-forbidden characters
		{"page?q=go", "page%3Fq=go"},
		{"file:name", "file%3Aname"},
		{"a*b", "a%2Ab"},
		{"a<b>c", "a%3Cb%3Ec"},
		{"a|b", "a%7Cb"},
		{`back\slash`, "back%5Cslash"},
		{`say"hello"`, "say%22hello%22"},
		// Control characters
		{"ctrl\x01char", "ctrl%01char"},
		{"tab\x09here", "tab%09here"},
		// Spaces (0x20) are allowed on most file systems
		{"spaces ok", "spaces ok"},
		// Dots and hyphens pass through
		{"file.html", "file.html"},
		{"my-file.tar.gz", "my-file.tar.gz"},
	}

	for _, tc := range cases {
		got := encodeForFS(tc.in)
		if got != tc.want {
			t.Errorf("encodeForFS(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}
