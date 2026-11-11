//go:build darwin

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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

// windowScriptLines are a window's few lines: remove the script, point the
// `smolcorp` command at a test copy (its port and folder; the terminal's
// environment is its own, not this program's), then run command, each word
// quoted so it's only ever itself.
func windowScriptLines(command []string, testPort int, testSupport string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("nothing to run")
	}
	quoted := make([]string, len(command))
	for i, word := range command {
		quoted[i] = shellQuoted(word)
	}
	lines := "#!/bin/sh\nrm -f -- \"$0\"\n"
	if testPort != 0 {
		lines += "export SMOLCORP_PORT=" + shellQuoted(strconv.Itoa(testPort)) + "\n" +
			"export SMOLCORP_TEST_SUPPORT_DIR=" + shellQuoted(testSupport) + "\n"
	}
	return lines + "exec " + strings.Join(quoted, " ") + "\n", nil
}

// writeWindowScript writes a window's script into folder (made 0700 if need
// be), only yours to read or run, as the bridge's open requires.
func writeWindowScript(folder string, command []string, testPort int, testSupport string) (string, error) {
	lines, err := windowScriptLines(command, testPort, testSupport)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(folder, "window-*.sh")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := file.Chmod(0o700); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	if _, err := file.WriteString(lines); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
