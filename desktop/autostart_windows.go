package desktop

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// Opening at login, through the per-user Run key: what Windows starts for
// you when you sign in, and what Task Manager's Startup apps lists.
const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "Smolcorp"
	// oldRunValue is the entry under the name the app had before Smolcorp.
	oldRunValue = "SauzzeDeck Pro"
)

func isOpenAtLogin() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue(runValue)
	_, _, old := key.GetStringValue(oldRunValue)
	return err == nil || old == nil
}

// moveOpenAtLogin turns an entry under the old name into one under the new,
// so the app isn't started twice at sign-in.
func moveOpenAtLogin() {
	// Opening at login is the Smolcorp in use's: a test copy's entry would start the copy in its place (RWTC N3).
	if testCopyPort() != 0 {
		return
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	_, _, old := key.GetStringValue(oldRunValue)
	key.Close()
	if old == nil {
		setOpenAtLogin(true)
	}
}

func setOpenAtLogin(enabled bool) {
	// Opening at login is the Smolcorp in use's: a test copy's entry would start the copy in its place (RWTC N3).
	if testCopyPort() != 0 {
		return
	}
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if err != nil {
			return
		}
		_ = key.DeleteValue(oldRunValue)
		_ = key.DeleteValue(runValue)
		key.Close()
	} else if program, _ := os.Executable(); writeRunValue(program) != nil {
		return
	}
	// The Run entry is now the one way Smolcorp opens at sign-in: a shortcut
	// to its app in the Startup folder would open it a second time, or when
	// you turned it off. One to smolcorp-cli.exe is yours, and stays.
	removeShortcuts(smolcorpShortcuts(startupFolder()))
}

// writeRunValue has Windows open program at sign-in, under the name Smolcorp
// has now: a variable, so the tests leave your Run key alone.
var writeRunValue = func(program string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	_ = key.DeleteValue(oldRunValue)
	return key.SetStringValue(runValue, `"`+program+`"`)
}

// startupFolder is your Startup folder, where a shortcut that opens Smolcorp
// may have been put (startupshortcut.go). It's found under %APPDATA%, where
// Windows keeps it, so a test with an APPDATA of its own reads its own.
func startupFolder() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
}

// moveStartupShortcuts, at install: a shortcut to Smolcorp's app in the
// Startup folder becomes the Run entry for program, the installed copy, so
// Smolcorp opens at sign-in once, and from where it stays. If the entry can't
// be written, the shortcut stays, rather than nothing opening at sign-in.
func moveStartupShortcuts(program string) {
	shortcuts := smolcorpShortcuts(startupFolder())
	if len(shortcuts) == 0 || testCopyPort() != 0 || writeRunValue(program) != nil {
		return
	}
	removeShortcuts(shortcuts)
}
