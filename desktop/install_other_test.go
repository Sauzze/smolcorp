//go:build !windows

package desktop

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestTheInstallRowAsksForTheInstallUntilItIsDone(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the install row is only on Linux")
	}
	home := isolate(t)
	saved := executable
	t.Cleanup(func() { executable = saved })

	executable = func() (string, error) { return filepath.Join(t.TempDir(), "download", "smolcorp"), nil }
	if c := installCheck(); c == nil || c.ID != "install" || c.Status != checkFix || c.Title != "Install Smolcorp" {
		t.Errorf("from the download: %+v, want a fix titled Install Smolcorp", c)
	}

	executable = func() (string, error) { return filepath.Join(home, ".local", "bin", "smolcorp"), nil }
	if c := installCheck(); c == nil || c.ID != "install" || c.Status != checkOK || c.Title != "Smolcorp is installed" {
		t.Errorf("when installed: %+v, want OK titled Smolcorp is installed", c)
	}
}
