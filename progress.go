package main

import (
	"fmt"
	"sync"
)

var progressMu sync.Mutex

// RenderProgress prints an in-place progress bar to stdout.
func RenderProgress(current, total int) {
	progressMu.Lock()
	defer progressMu.Unlock()

	if total == 0 {
		return
	}

	const barWidth = 40
	pct := float64(current) / float64(total)
	filled := int(pct * barWidth)
	if filled > barWidth {
		filled = barWidth
	}

	bar := make([]rune, barWidth)
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar[i] = '\u2588' // full block
		} else {
			bar[i] = '\u2591' // light shade
		}
	}

	end := "\r"
	if current >= total {
		end = "\n"
	}
	fmt.Printf("\r[%s] %3.0f%%  %d/%d%s", string(bar), pct*100, current, total, end)
}
