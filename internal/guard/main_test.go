package guard

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points the home and config folders at a fresh temporary folder, so
// no test can reach the real app's support folder (on a Mac, Application
// Support/AgentDeck) by asking the system where it is.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "smolcorp-guard-test-")
	if err != nil {
		println("can't make a temp folder:", err.Error())
		os.Exit(1)
	}
	os.Setenv("HOME", root)
	os.Setenv("USERPROFILE", root)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	os.Setenv("APPDATA", filepath.Join(root, "AppData"))
	// Nor a test copy's folder from whatever ran the tests: a test that wants one sets its own.
	os.Unsetenv(testSupportEnv)
	// Every test reads a line the hook is handed as a Mac's or Linux's, on whatever runs it (the CI's windows-latest
	// too); one that wants Windows' reading says so (asWindows).
	hostWindows, onWindows = onWindows, false
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
