package desktop

import (
	"path/filepath"
	"testing"
)

// isolate points every file the package reads or writes, and the home folder
// agents keep their settings in, at fresh folders, so a test never touches
// the real paired phones or an agent's real settings. It returns the home.
func isolate(t *testing.T) string {
	t.Helper()
	saved := []string{supportDir, phonesFile, configFile, hookKeyFile, setupKeyFile, homeDir, sessionsFile}
	t.Cleanup(func() {
		supportDir, phonesFile, configFile, hookKeyFile, setupKeyFile, homeDir = saved[0], saved[1], saved[2], saved[3], saved[4], saved[5]
		sessionsFile = saved[6]
	})
	supportDir = t.TempDir()
	phonesFile = filepath.Join(supportDir, "phones.json")
	configFile = filepath.Join(supportDir, "config.json")
	hookKeyFile = filepath.Join(supportDir, "hook-key")
	setupKeyFile = filepath.Join(supportDir, "setup-key")
	sessionsFile = filepath.Join(supportDir, "sessions.json")
	homeDir = t.TempDir()
	return homeDir
}
