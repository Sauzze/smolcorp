// Package appdirs holds the one place that names the folder the app keeps its
// state in, and finds it, so desktop, internal/gm, internal/guard, the sandbox
// and the upgrade check can't drift apart. (It imports nothing of the app's, so
// the command-line tools can use it, and it never makes the folder.)
package appdirs

import (
	"os"
	"path/filepath"
	"runtime"
)

// The folder's name on each system, as it is today. They keep the names the app
// had before it was Smolcorp (the Mac app is still AgentDeck's), so paired phones
// stay paired.
const (
	Mac     = "AgentDeck"
	Linux   = "sauzzedeck-pro"
	Windows = "SauzzeDeck Pro"
)

// Smolcorp's own names for the folder, beside the old ones. Nothing moves or
// makes it yet: Resolve uses one only once it holds Smolcorp's state.
const (
	NewMac     = "Smolcorp"
	NewLinux   = "smolcorp"
	NewWindows = "Smolcorp"
)

// stateFile is the file that says a folder holds Smolcorp's state: the team
// (desktop's registry.json). On Linux ~/.config/smolcorp can hold a
// developer's own files (sign-identity) and nothing of the app's, so the folder
// alone says nothing.
const stateFile = "registry.json"

// Name is the support folder's old name for a runtime.GOOS value, inside the
// user's config folder (Application Support on a Mac).
func Name(goos string) string {
	switch goos {
	case "darwin":
		return Mac
	case "windows":
		return Windows
	}
	return Linux
}

// NewName is the support folder's Smolcorp name for a runtime.GOOS value.
func NewName(goos string) string {
	switch goos {
	case "darwin":
		return NewMac
	case "windows":
		return NewWindows
	}
	return NewLinux
}

// Resolve is the support folder every reader uses, in the user's config folder
// (os.UserConfigDir: Application Support on a Mac, $XDG_CONFIG_HOME or
// ~/.config on Linux, %APPDATA% on Windows): see ResolveIn. It makes nothing.
func Resolve() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return ResolveIn(base, runtime.GOOS), nil
}

// ResolveIn is the support folder in base on goos: the Smolcorp name when it is
// a real folder (not a link) holding Smolcorp's state, a registry.json that is a
// plain file; otherwise the old name, there or not. It never moves or makes
// either folder.
func ResolveIn(base, goos string) string {
	if HoldsState(filepath.Join(base, NewName(goos))) {
		return filepath.Join(base, NewName(goos))
	}
	return filepath.Join(base, Name(goos))
}

// HoldsState says whether dir is a real folder, not a link, holding Smolcorp's
// state: a registry.json in it that is a plain file.
func HoldsState(dir string) bool {
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		return false
	}
	st, err := os.Lstat(filepath.Join(dir, stateFile))
	return err == nil && st.Mode().IsRegular()
}
