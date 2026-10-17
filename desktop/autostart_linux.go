package desktop

import (
	"fmt"
	"os"
	"path/filepath"
)

// Opening at login, through an XDG autostart entry: GNOME, KDE and most
// other desktops start what ~/.config/autostart lists.
func autostartFile() string {
	base, _ := os.UserConfigDir()
	return filepath.Join(base, "autostart", "smolcorp.desktop")
}

// oldAutostartFile is the entry under the name the app had before Smolcorp.
func oldAutostartFile() string {
	base, _ := os.UserConfigDir()
	return filepath.Join(base, "autostart", "sauzzedeck-pro.desktop")
}

func isOpenAtLogin() bool {
	_, err := os.Stat(autostartFile())
	_, old := os.Stat(oldAutostartFile())
	return err == nil || old == nil
}

// moveOpenAtLogin turns an entry under the old name into one under the new,
// so the app isn't started twice at login.
func moveOpenAtLogin() {
	if _, err := os.Stat(oldAutostartFile()); err == nil {
		setOpenAtLogin(true)
	}
}

func setOpenAtLogin(enabled bool) {
	_ = os.Remove(oldAutostartFile())
	if !enabled {
		_ = os.Remove(autostartFile())
		return
	}
	program, _ := os.Executable()
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Smolcorp\nExec=%q\nIcon=smolcorp\nX-GNOME-Autostart-enabled=true\n", program)
	_ = os.MkdirAll(filepath.Dir(autostartFile()), 0o755)
	_ = os.WriteFile(autostartFile(), []byte(entry), 0o644)
}
