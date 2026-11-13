package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// modeIs says a file has the Unix mode want. Windows has no Unix modes: Go reports any writable
// file as 0666 there, and what keeps a file yours is your profile folder's own permissions,
// so there any mode is taken as right (as saved_test.go does).
func modeIs(info os.FileInfo, want os.FileMode) bool {
	return runtime.GOOS == "windows" || info.Mode().Perm() == want
}

// isolate points every file the package reads or writes, and the home folder
// agents keep their settings in, at fresh folders, so a test never touches
// the real paired phones or an agent's real settings. It returns the home.
func isolate(t *testing.T) string {
	t.Helper()
	saved := []string{supportDir, phonesFile, configFile, hookKeyFile, setupKeyFile, homeDir, windowKeyFile, sessionsFile}
	t.Cleanup(func() {
		supportDir, phonesFile, configFile, hookKeyFile, setupKeyFile, homeDir, windowKeyFile = saved[0], saved[1], saved[2], saved[3], saved[4], saved[5], saved[6]
		sessionsFile = saved[7]
	})
	supportDir = t.TempDir()
	phonesFile = filepath.Join(supportDir, "phones.json")
	configFile = filepath.Join(supportDir, "config.json")
	hookKeyFile = filepath.Join(supportDir, "hook-key")
	setupKeyFile = filepath.Join(supportDir, "setup-key")
	windowKeyFile = filepath.Join(supportDir, "window-key")
	sessionsFile = filepath.Join(supportDir, "sessions.json")
	homeDir = t.TempDir()
	return homeDir
}

// scriptSettings are the settings a start script gives its Claude Code in its command line (--settings=…, one
// shell-quoted word), "" if it gives none.
func scriptSettings(script string) string {
	i := strings.Index(script, "'--settings=")
	if i < 0 {
		return ""
	}
	var word strings.Builder
	rest := script[i+len("'--settings="):]
	for {
		end := strings.IndexByte(rest, '\'')
		if end < 0 {
			return ""
		}
		word.WriteString(rest[:end])
		if !strings.HasPrefix(rest[end:], `'\''`) {
			return word.String()
		}
		word.WriteByte('\'')
		rest = rest[end+len(`'\''`):]
	}
}

// startedSettings are the settings hire hireID's start script gives its Claude Code, "" if none.
func startedSettings(t *testing.T, hireID string) string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(hireFolder(supportDir, hireID), "hire-*.sh"))
	for _, file := range files {
		data, _ := os.ReadFile(file)
		if settings := scriptSettings(string(data)); settings != "" {
			return settings
		}
	}
	return ""
}

// launcherSettings are the settings a launcher hire hireID's start environment holds for its Claude Code, as its
// launcher would take them once, "" if none: they never are in a file or on its command line.
func launcherSettings(d *Deck, hireID string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, pending := range d.hires {
		if pending.hireID == hireID && pending.env != nil {
			return pending.env.Settings
		}
	}
	return ""
}
