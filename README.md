# 💾 wayback-dl

A fast, self-contained command-line tool for downloading archived websites from
the [Wayback Machine](https://web.archive.org/). 

Go adaptation of [wayback-machine-downloader](https://github.com/birbwatcher/wayback-machine-downloader).

---

## Install

Download release or Go native install:

```sh
go install github.com/sigman78/wayback-dl/cmd/wayback-dl@latest
```

Requires Go 1.24+.

---

## Usage

```
wayback-dl [url] [options]

Arguments:
  url                     Domain or URL to archive (same as -url)

Options:
  -url string             Domain or URL to archive
  -from string            Start timestamp YYYYMMDDhhmmss (default: none)
  -to string              End timestamp YYYYMMDDhhmmss (default: none)
  -threads int            Concurrent download threads (default: 3)
  -directory string       Output directory (default: websites/<host>/)
  -rewrite-links          Rewrite page links to relative paths
  -pretty-path            Map extension-less URLs to dir/index.html (default: preserve original path)
  -canonical string       Canonical tag handling: keep|remove (default: keep)
  -exact-url              Download only the exact URL, no wildcard /*
  -external-assets        Also download off-site (external) assets
  -stop-on-error          Stop immediately on first download error (default: continue)
  -cdx-rate int           CDX API requests per minute (default: 60)
  -cdx-retries int        Max retries on CDX throttle or 5xx (default: 5)
  -debug                  Enable verbose debug logging
  -version                Print version and exit
  -h / -help              Show this help and exit
```

### Examples

```sh
# Download all snapshots of a site
wayback-dl example.com

# Limit to a date range with 8 threads
wayback-dl example.com -from 20200101000000 -to 20201231235959 -threads 8

# Rewrite links for offline browsing, remove canonical tags
wayback-dl example.com -rewrite-links -canonical remove -directory ./out

# Exact URL only (no wildcard crawl)
wayback-dl https://example.com/blog/ -exact-url

# Debug output
wayback-dl example.com -debug
```

---

## How it works

1. Queries the [CDX API](https://github.com/internetarchive/wayback/tree/master/wayback-cdx-server)
   for all snapshots of the target URL (wildcarded by default).
2. Deduplicates snapshots by URL path, keeping the most recent timestamp for each.
3. Downloads each snapshot concurrently using Wayback's raw-content (`id_`) endpoint.
4. Optionally rewrites HTML/CSS links to relative paths for offline browsing.

---

## Output structure

Files are saved under `websites/<host>/` mirroring the original URL path:

```
websites/
└── example.com/
    ├── index.html
    ├── about/
    │   └── index.html
    └── assets/
        └── style.css
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `golang.org/x/net/html` | HTML parsing for link rewriting |

Everything else uses the Go standard library.

---

## Testing

```sh
# Build + smoke test
make build
./wayback-dl example.com -from 20200101 -to 20200201 -threads 2
```

---

## Development 

```sh
# Install tooling (one-time)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/goreleaser/goreleaser/v2@latest

# Build with version info
make build

# Run tests
make test

# Run linter
make lint

# Activate pre-commit hook (per clone)
git config core.hooksPath .githooks
```

---

## Release

Releases are automated via [goreleaser](https://goreleaser.com/) and GitHub Actions.
Push a semver tag to trigger a release:

```sh
git tag v0.2.0
git push origin v0.2.0
```

The CI workflow (`ci.yml`) runs on every push to `main`/`master` and on pull
requests. The release workflow (`release.yml`) triggers on `v*` tags and publishes
cross-compiled binaries for Linux, macOS, and Windows.
