package utils

import (
	"io"
	"os"
	"sync"
)

// Package-level quiet switch for progress bars that live outside the
// Downloader (posts, interactions). The TUI enables it so those bars don't
// corrupt the alt screen; CLI mode keeps them on stderr.
var (
	terminalMu    sync.Mutex
	terminalQuiet bool
)

// SetTerminalQuiet suppresses terminal output from package-level progress bars.
func SetTerminalQuiet(quiet bool) {
	terminalMu.Lock()
	defer terminalMu.Unlock()
	terminalQuiet = quiet
}

// ProgressWriter returns where standalone progress bars should render.
func ProgressWriter() io.Writer {
	terminalMu.Lock()
	defer terminalMu.Unlock()
	if terminalQuiet {
		return io.Discard
	}
	return os.Stderr
}

// fetchHook, when set, receives the running count from the posts package's
// fetch progress bars so the TUI can display it.
var fetchHook func(current int)

// SetFetchHook registers a callback for fetch progress counts.
func SetFetchHook(h func(current int)) {
	terminalMu.Lock()
	defer terminalMu.Unlock()
	fetchHook = h
}

// ReportFetch reports the current fetch count to the registered hook.
func ReportFetch(current int) {
	terminalMu.Lock()
	h := fetchHook
	terminalMu.Unlock()
	if h != nil {
		h(current)
	}
}
