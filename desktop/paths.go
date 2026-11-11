// Package desktop is Smolcorp for Windows and Linux: the counterpart of
// the Mac app in Sources/AgentDeck, file for file where the two can match.
//
// The phone sees the same deck, pairs the same way and follows the same
// agents. What differs is how a session's keys are pressed: the Mac drives
// Terminal.app, which nothing on Windows or Linux matches, so here agents are
// started through the app's launcher (`smolcorp run claude`), or found in a
// tmux, WezTerm or kitty pane, whose own commands reach them.
package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/Sauzze/smolcorp/internal/appdirs"
)

// Port is where the deck listens on every platform: 7710, unless SMOLCORP_PORT
// says otherwise (1024 to 65535), to try a build beside the one you use.
var Port = portFromEnvironment(os.Getenv("SMOLCORP_PORT"))

func portFromEnvironment(text string) int {
	if n, err := strconv.Atoi(text); err == nil && n >= 1024 && n <= 65535 {
		return n
	}
	return 7710
}

// Where the app keeps its state, outside the program itself, so it can be
// moved or updated without losing the paired phones: ~/.config/sauzzedeck-pro
// on Linux, %APPDATA%\SauzzeDeck Pro on Windows, Application Support/AgentDeck
// on a Mac. They keep the name the app had before it was Smolcorp, as the Mac
// app keeps AgentDeck's.
var (
	supportDir  = makeSupportDir()
	phonesFile  = filepath.Join(supportDir, "phones.json")
	configFile  = filepath.Join(supportDir, "config.json")
	hookKeyFile = filepath.Join(supportDir, "hook-key")
	// The sessions running, and their keys, kept across a restart.
	sessionsFile = filepath.Join(supportDir, "sessions.json")
	// The setup page's key, new at every start, for tools of your own on this computer.
	setupKeyFile = filepath.Join(supportDir, "setup-key")
	// The key of the office window's own paired device, private like the hook's.
	windowKeyFile = filepath.Join(supportDir, "window-key")
	homeDir, _    = os.UserHomeDir()
)

func makeSupportDir() string {
	// A binary `go test` built (testing.Testing) must never create or reach the app's real
	// folder, even at import: it gets a throwaway one, which TestMain removes.
	if testing.Testing() {
		if dir, err := os.MkdirTemp("", "smolcorp-desktop-test-"); err == nil {
			return dir
		}
		return filepath.Join(os.TempDir(), "smolcorp-desktop-test-fallback")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, supportDirName(runtime.GOOS))
	_ = os.MkdirAll(dir, 0o700)
	return dir
}

// useSupportDir points everything kept in the support folder at dir, before
// anything is read from it: the Mac app's engine (`smolcorp core`) is given
// the folder the app chose, a test copy's own among them.
func useSupportDir(dir string) {
	supportDir = dir
	phonesFile = filepath.Join(dir, "phones.json")
	configFile = filepath.Join(dir, "config.json")
	hookKeyFile = filepath.Join(dir, "hook-key")
	sessionsFile = filepath.Join(dir, "sessions.json")
	setupKeyFile = filepath.Join(dir, "setup-key")
	windowKeyFile = filepath.Join(dir, "window-key")
}

// supportDirName is the folder's name on a system: the one the Mac app writes
// on a Mac (AgentDeck), so the setup key is found there.
func supportDirName(goos string) string { return appdirs.Name(goos) }

// writePrivate writes a file only its owner can read, as the keys need.
func writePrivate(path string, data []byte) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
