package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sigman78/wayback-dl/internal/wayback"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: wayback-dl [url] [options]

Arguments:
  url                     Domain or URL to archive (same as -url)

Options:
  -url string             Domain or URL to archive
  -from string            Start timestamp YYYYMMDDhhmmss (default: none)
  -to string              End timestamp YYYYMMDDhhmmss (default: none)
  -threads int            Concurrent download threads (default: 3)
  -directory string       Output directory (default: websites/<host>/)
  -rewrite-links          Rewrite page links to relative paths
  -prettyPath             Map extension-less URLs to dir/index.html (default: preserve original path)
  -canonical string       Canonical tag handling: keep|remove (default: keep)
  -exact-url              Download only the exact URL, no wildcard /*
  -external-assets        Also download off-site (external) assets
  -stop-on-error          Stop immediately on first download error (default: continue)
  -cdx-rate int           CDX API requests per minute (default: 60)
  -cdx-retries int        Max retries on CDX throttle or 5xx (default: 5)
  -debug                  Enable verbose debug logging
  -version                Print version and exit
  -h / -help              Show this help and exit
`)
}

func main() {
	// Use ContinueOnError so we can intercept ErrHelp and unknown-flag errors
	// and control the exit code ourselves.
	fs := flag.NewFlagSet("wayback-dl", flag.ContinueOnError)
	fs.Usage = usage

	var (
		urlFlag      string
		fromFlag     string
		toFlag       string
		threadsFlag  int
		dirFlag      string
		rewriteLinks bool
		prettyPath   bool
		canonical    string
		exactURL     bool
		extAssets    bool
		stopOnError  bool
		cdxRate      int
		cdxRetries   int
		debug        bool
	)

	fs.StringVar(&urlFlag, "url", "", "Domain or URL to archive")
	fs.StringVar(&fromFlag, "from", "", "Start timestamp YYYYMMDDhhmmss")
	fs.StringVar(&toFlag, "to", "", "End timestamp YYYYMMDDhhmmss")
	fs.IntVar(&threadsFlag, "threads", 3, "Concurrent download threads")
	fs.StringVar(&dirFlag, "directory", "", "Output directory")
	fs.BoolVar(&rewriteLinks, "rewrite-links", false, "Rewrite page links to relative paths")
	fs.BoolVar(&prettyPath, "prettyPath", false, "Prettify paths: map extension-less URLs to dir/index.html")
	fs.StringVar(&canonical, "canonical", "keep", "Canonical tag handling: keep|remove")
	fs.BoolVar(&exactURL, "exact-url", false, "Download only the exact URL, no wildcard /*")
	fs.BoolVar(&extAssets, "external-assets", false, "Also download off-site (external) assets")
	fs.BoolVar(&stopOnError, "stop-on-error", false, "Stop immediately on first download error")
	fs.IntVar(&cdxRate, "cdx-rate", 60, "CDX API requests per minute")
	fs.IntVar(&cdxRetries, "cdx-retries", 5, "Max retries on CDX throttle or 5xx")
	fs.BoolVar(&debug, "debug", false, "Enable verbose debug logging")

	// Handle -version / -h / -help before the flag parser so we control the exit code.
	for _, a := range os.Args[1:] {
		if a == "-version" || a == "--version" {
			fmt.Printf("wayback-dl %s (commit %s, built %s)\n", version, commit, date)
			os.Exit(0)
		}
		if a == "-h" || a == "-help" || a == "--help" {
			usage()
			os.Exit(0)
		}
	}

	// Extract a leading positional URL argument before flag parsing so that
	// "wayback-dl example.com -canonical remove" works (flags after the URL
	// are still parsed correctly; the stdlib flag package stops at the first
	// non-flag argument).
	args := os.Args[1:]
	var positionalURL string
	if len(args) > 0 && args[0] != "" && !strings.HasPrefix(args[0], "-") {
		positionalURL = args[0]
		args = args[1:]
	}

	if err := fs.Parse(args); err != nil {
		// Unknown/malformed flag: fs already printed the error message
		os.Exit(2)
	}

	// Merge positional URL with -url flag (explicit -url wins)
	if urlFlag == "" {
		urlFlag = positionalURL
	}

	// Validation — check flags before checking URL so flag errors surface clearly
	if threadsFlag <= 0 {
		fmt.Fprintln(os.Stderr, "error: -threads must be greater than 0")
		os.Exit(1)
	}
	canonical = strings.ToLower(canonical)
	if canonical != "keep" && canonical != "remove" {
		fmt.Fprintln(os.Stderr, "error: -canonical must be 'keep' or 'remove'")
		os.Exit(1)
	}
	if urlFlag == "" {
		fmt.Fprintln(os.Stderr, "error: URL is required")
		usage()
		os.Exit(1)
	}

	base, err := wayback.NormalizeBaseURL(urlFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid URL: %v\n", err)
		os.Exit(1)
	}

	outDir := dirFlag
	if outDir == "" {
		outDir = "websites/" + base.BareHost
	}

	cfg := &wayback.Config{
		BaseURL:                base.CanonicalURL,
		Variants:               base.Variants,
		BareHost:               base.BareHost,
		UnicodeHost:            base.UnicodeHost,
		ExactURL:               exactURL,
		Directory:              outDir,
		FromTimestamp:          fromFlag,
		ToTimestamp:            toFlag,
		Threads:                threadsFlag,
		RewriteLinks:           rewriteLinks,
		PrettyPath:             prettyPath,
		CanonicalAction:        canonical,
		DownloadExternalAssets: extAssets,
		StopOnError:            stopOnError,
		CDXRatePerMin:          cdxRate,
		CDXMaxRetries:          cdxRetries,
		Debug:                  debug,
	}

	fmt.Printf("Fetching snapshot index for %s ...\n", base.CanonicalURL)
	if err := wayback.DownloadAll(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
