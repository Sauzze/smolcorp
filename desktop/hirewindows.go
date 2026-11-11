package desktop

import (
	"slices"
	"sync"
)

// The windows this app opened for each hire, by the record its terminal gave
// (T796), as the Mac app's Deck.hireWindows: only a window the Mac app opens
// under its engine has one (hire_darwin.go). Terminal's carries its window's
// id beside its tab's tty (tty:/dev/ttys004#w1234), iTerm2's is its session's
// unique ID, Ghostty's its terminal's id. Switching hires to the background
// closes these and no other; a Linux terminal running `tmux attach` ends by
// itself as its viewer is detached, so it needs no record. Kept in memory, as
// the Mac's: a window from before a restart is left as it is.

var hireWindows = struct {
	sync.Mutex
	byHire map[string][]string
}{byHire: map[string][]string{}}

// recordHireWindow keeps window, opened for hire id; "" is no window.
func recordHireWindow(id, window string) {
	if id == "" || window == "" {
		return
	}
	hireWindows.Lock()
	defer hireWindows.Unlock()
	if !slices.Contains(hireWindows.byHire[id], window) {
		hireWindows.byHire[id] = append(hireWindows.byHire[id], window)
	}
}

// windowsOfHire are the windows recorded for hire id, oldest first.
func windowsOfHire(id string) []string {
	hireWindows.Lock()
	defer hireWindows.Unlock()
	return slices.Clone(hireWindows.byHire[id])
}

// forgetHireWindows drops the record of windows closed or gone, for hire id.
func forgetHireWindows(id string, windows []string) {
	hireWindows.Lock()
	defer hireWindows.Unlock()
	left := slices.DeleteFunc(hireWindows.byHire[id], func(w string) bool { return slices.Contains(windows, w) })
	if len(left) == 0 {
		delete(hireWindows.byHire, id)
	} else {
		hireWindows.byHire[id] = left
	}
}
