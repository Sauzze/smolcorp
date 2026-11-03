package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		return
	}
	// An entry left by a portable copy points at that copy; once Smolcorp is
	// installed, the login should start the installed one.
	installed := installedProgram()
	entry, err := os.ReadFile(autostartFile())
	if installed == "" || err != nil {
		return
	}
	lines := strings.Split(string(entry), "\n")
	changed := false
	for i, line := range lines {
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "hidden=true", "x-gnome-autostart-enabled=false":
			return // the user turned it off: leave it so
		}
		if exec, ok := strings.CutPrefix(line, "Exec="); ok && isStaleExec(exec, installed) {
			lines[i] = "Exec=" + execQuote(installed)
			changed = true
		}
	}
	if changed {
		_ = os.WriteFile(autostartFile(), []byte(strings.Join(lines, "\n")), 0o644)
	}
}

// isStaleExec is whether an Exec value is one of ours that doesn't start the
// installed copy: a lone program path of the deck other than installed, or
// installed in the %q quoting an earlier version wrote. Anything else (a
// wrapper, extra arguments) isn't ours to change.
func isStaleExec(exec, installed string) bool {
	if exec == execQuote(installed) {
		return false
	}
	if exec == fmt.Sprintf("%q", installed) {
		return true
	}
	program := exec
	if strings.HasPrefix(exec, `"`) {
		inner, ok := strings.CutSuffix(strings.TrimPrefix(exec, `"`), `"`)
		if !ok {
			return false
		}
		// Between the quotes every " ` $ \ is escaped, so a bare one is more than a path.
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			switch c := inner[i]; {
			case c == '\\' && strings.HasPrefix(inner[i:], `\\\\`):
				b.WriteByte('\\')
				i += 3
			case c == '\\' && i+1 < len(inner) && strings.IndexByte("\"`$", inner[i+1]) >= 0:
				b.WriteByte(inner[i+1])
				i++
			case c == '%' && strings.HasPrefix(inner[i:], "%%"):
				b.WriteByte('%')
				i++
			case strings.IndexByte("\"`$\\%", c) >= 0:
				return false
			default:
				b.WriteByte(c)
			}
		}
		program = b.String()
	} else if strings.ContainsAny(exec, " \t\"`$\\%") {
		return false
	}
	return strings.HasPrefix(program, "/") && program != installed && isDecksProgram(program)
}

// installedProgram is the copy packaging/linux/install.sh puts in
// ~/.local/bin, or "" when there is none.
func installedProgram() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".local", "bin", "smolcorp")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// installedIcon is the icon install.sh places next to the app's data, or the
// icon's name when it isn't there.
func installedIcon() string {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "smolcorp"
		}
		data = filepath.Join(home, ".local", "share")
	}
	path := filepath.Join(data, "smolcorp", "icon.png")
	if _, err := os.Stat(path); err != nil {
		return "smolcorp"
	}
	return path
}

// execQuote writes a program for an Exec line the way the Desktop Entry
// spec asks: in double quotes, with " ` $ and \ escaped (a literal \ takes
// four backslashes in the file) and a literal % doubled.
func execQuote(program string) string {
	r := strings.NewReplacer(`\`, `\\\\`, `"`, `\"`, "`", "\\`", `$`, `\$`, `%`, `%%`)
	return `"` + r.Replace(program) + `"`
}

func setOpenAtLogin(enabled bool) {
	_ = os.Remove(oldAutostartFile())
	if !enabled {
		_ = os.Remove(autostartFile())
		return
	}
	program, _ := os.Executable()
	writeAutostart(program)
}

func writeAutostart(program string) {
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Smolcorp\nExec=%s\nIcon=%s\nX-GNOME-Autostart-enabled=true\n", execQuote(program), installedIcon())
	_ = os.MkdirAll(filepath.Dir(autostartFile()), 0o755)
	_ = os.WriteFile(autostartFile(), []byte(entry), 0o644)
}
