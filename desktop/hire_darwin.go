//go:build darwin

package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Hires' windows on a Mac, under the Mac app (`smolcorp core`): Terminal,
// iTerm2 or Ghostty, as the Swift engine's Openers offer them. This program
// never tells a terminal anything itself (no AppleScript here): it writes the
// few lines the window runs into a script in the hires folder, which only you
// can read or run and which removes itself as it runs, and asks the Mac app to
// open a window of the chosen terminal on it (the bridge's open, HostBridge.swift).
// The app answers with the window's record, which is kept for the hire
// (hirewindows.go). Without the app, as `smolcorp serve` would be, there are none.

// macTerminals are the terminals the Mac app opens, in setup's order (Openers.choices).
var macTerminals = []struct{ id, name string }{{"terminal", "Terminal"}, {"iterm", "iTerm2"}, {"ghostty", "Ghostty"}}

// How long the app's list of installed terminals is kept: setup asks for its
// state often, and the list only changes when you install a terminal.
const macTerminalsFresh = 10 * time.Second

// useMacOpeners makes the terminals the Mac app opens the ones hires open in.
func useMacOpeners(h hostBridge) {
	var (
		mu    sync.Mutex
		at    time.Time
		found []string
	)
	installedOpeners = func() []hireOpener {
		mu.Lock()
		if at.IsZero() || time.Since(at) > macTerminalsFresh {
			// The app couldn't say: none for now, so a hire goes to tmux alone, or says there's no terminal.
			found, _ = hostTerminals(h)
			at = time.Now()
		}
		ids := slices.Clone(found)
		mu.Unlock()
		var openers []hireOpener
		for _, terminal := range macTerminals {
			if slices.Contains(ids, terminal.id) {
				openers = append(openers, macOpener(h, terminal.id, terminal.name))
			}
		}
		return openers
	}
}

// macOpener is terminal as a hire's window opens in it: through the app.
func macOpener(h hostBridge, id, name string) hireOpener {
	return hireOpener{id: id, name: name, open: func(folder string, command []string) (string, error) {
		script, err := writeWindowScript(filepath.Join(supportDir, "hires"), command, testCopyPort(), supportDir)
		if err != nil {
			return "", err
		}
		_, window, err := hostOpenTerminal(h, id, folder, script)
		if err != nil {
			// The window didn't open, so nothing ran it: it goes now.
			_ = os.Remove(script)
			return "", err
		}
		return window, nil
	}}
}
