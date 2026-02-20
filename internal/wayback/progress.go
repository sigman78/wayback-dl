package wayback

import (
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
)

// Progress is a nil-safe wrapper around progressbar.ProgressBar.
// A nil *Progress is valid; all methods are no-ops, making it trivial
// to disable output in tests or non-interactive pipelines.
type Progress struct {
	bar *progressbar.ProgressBar
}

// NewCDXProgress creates an indeterminate spinner for the CDX index-fetch phase.
// Each call to Inc() advances the spinner and adds one to the page counter.
func NewCDXProgress() *Progress {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription("[green][1/2][reset] Fetching CDX data"),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(20),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionClearOnFinish(),
	)
	return &Progress{bar: bar}
}

// NewDownloadProgress creates a determinate bar for the file-download phase.
func NewDownloadProgress(total int) *Progress {
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetDescription("[green][2/2][reset] Downloading pages"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionOnCompletion(func() {
			_, _ = os.Stderr.WriteString("\n")
		}),
	)
	return &Progress{bar: bar}
}

// Inc increments the progress bar by one step.
func (p *Progress) Inc() {
	if p == nil {
		return
	}
	_ = p.bar.Add(1)
}

// Finish marks the bar as complete and moves to a new line.
func (p *Progress) Finish() {
	if p == nil {
		return
	}
	_ = p.bar.Finish()
}
