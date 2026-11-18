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
	"sync/atomic"
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
	// Where earlier versions kept the setup key and the Windows window's key. Both now live in memory only
	// (SK2): the files are only ever removed, at each start.
	setupKeyFile  = filepath.Join(supportDir, "setup-key")
	windowKeyFile = filepath.Join(supportDir, "window-key")
	// The check-only key, new at every start: it opens GET /setup/selftest and nothing else, for
	// `smolcorp setup` and the Mac's --setup (selftestkey.go).
	selftestKeyFile = filepath.Join(supportDir, "selftest-key")
	homeDir, _      = os.UserHomeDir()
	// The hook key as the Mac app's report.sh sends it (curl -H @hook-header), set only under the Mac
	// app (useSupportDir): "" elsewhere, where nothing reads it.
	hookHeaderFile = ""
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
	selftestKeyFile = filepath.Join(dir, "selftest-key")
	hookHeaderFile = filepath.Join(dir, "hook-header")
}

// supportDirName is the folder's name on a system: the one the Mac app writes
// on a Mac (AgentDeck), so the setup key is found there.
func supportDirName(goos string) string { return appdirs.Name(goos) }

// writePrivate writes a file only its owner can read, as the keys need: to a new file beside it, then put in its
// place. The new file is named after the file with a random part (`<name>.<random>.tmp`, os.CreateTemp's), so no
// one can make it first: it's made by this call alone (O_EXCL), and a file, folder or link at any name that could be
// guessed beforehand is never written through and stops nothing. It's 0600 from the start, set so again once open,
// before any of the data goes in, synced to the disk, then renamed over the file: a link at the file's own name is
// replaced, never followed. On any failure it's taken away. One left by an older Smolcorp at the fixed name it wrote
// through (`<name>.tmp`) is taken away too, never opened.
func writePrivate(path string, data []byte) error {
	_ = os.Remove(path + ".tmp")
	if refuse := privateWriteRefused.Load(); refuse != nil {
		if err := (*refuse)(path); err != nil {
			return err
		}
	}
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	temp := file.Name()
	err = file.Chmod(0o600)
	if err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	if closed := file.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temp, path)
	}
	if err != nil {
		_ = os.Remove(temp)
	}
	return err
}

// privateWriteRefused, while a test sets it, says which private writes fail before anything is made, as on a full
// disk: the error for a path, or nil to let it through. Nil, as outside a test, lets every write through.
var privateWriteRefused atomic.Pointer[func(path string) error]
