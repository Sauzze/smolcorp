package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points every path the package keeps its state in at a fresh
// temporary folder before any test runs, so a test that forgets isolate(t)
// still can't touch the real app's files (on a Mac that folder is the Mac
// app's own, Application Support/AgentDeck). It removes the folders after.
func TestMain(m *testing.M) {
	os.Exit(runWithTempSupport(m))
}

func runWithTempSupport(m *testing.M) int {
	realSupport := supportDir // already a temp folder: makeSupportDir sees a test binary
	root, err := os.MkdirTemp("", "smolcorp-desktop-main-")
	if err != nil {
		println("can't make a temp folder:", err.Error())
		return 1
	}
	defer os.RemoveAll(root)
	defer os.RemoveAll(realSupport)

	supportDir = filepath.Join(root, "support")
	homeDir = filepath.Join(root, "home")
	for _, dir := range []string{supportDir, homeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			println("can't make a temp folder:", err.Error())
			return 1
		}
	}
	phonesFile = filepath.Join(supportDir, "phones.json")
	configFile = filepath.Join(supportDir, "config.json")
	hookKeyFile = filepath.Join(supportDir, "hook-key")
	sessionsFile = filepath.Join(supportDir, "sessions.json")
	setupKeyFile = filepath.Join(supportDir, "setup-key")
	windowKeyFile = filepath.Join(supportDir, "window-key")

	// Anything that asks the system for the home or config folder lands here too.
	os.Setenv("HOME", homeDir)
	os.Setenv("USERPROFILE", homeDir)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	os.Setenv("APPDATA", filepath.Join(homeDir, "AppData"))
	return m.Run()
}
