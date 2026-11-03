package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestThePortIsSevenSevenOneZeroUnlessAskedOtherwise(t *testing.T) {
	for text, want := range map[string]int{"": 7710, "abc": 7710, "80": 7710, "70000": 7710, "4711": 4711} {
		if got := portFromEnvironment(text); got != want {
			t.Errorf("%q: %d, want %d", text, got, want)
		}
	}
}

// A test that never calls isolate still sees temp paths, never the real app's.
func TestATestWithoutIsolateSeesATempSupportDir(t *testing.T) {
	temp := filepath.Clean(os.TempDir())
	real, err := os.UserConfigDir()
	for name, path := range map[string]string{
		"supportDir": supportDir, "phonesFile": phonesFile, "configFile": configFile,
		"hookKeyFile": hookKeyFile, "setupKeyFile": setupKeyFile, "windowKeyFile": windowKeyFile,
		"sessionsFile": sessionsFile, "homeDir": homeDir,
	} {
		if !strings.HasPrefix(filepath.Clean(path), temp) {
			t.Errorf("%s = %q, not under the temp folder %q", name, path, temp)
		}
		if err == nil && strings.Contains(path, filepath.Join(real, supportDirName(runtime.GOOS))) {
			t.Errorf("%s = %q reaches the real support folder", name, path)
		}
	}
	if !testing.Testing() {
		t.Error("testing.Testing() is false inside a test")
	}
}
