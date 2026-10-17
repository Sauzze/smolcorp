package desktop

import (
	"os"

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
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	_ = key.DeleteValue(oldRunValue)
	if !enabled {
		_ = key.DeleteValue(runValue)
		return
	}
	program, _ := os.Executable()
	_ = key.SetStringValue(runValue, `"`+program+`"`)
}
