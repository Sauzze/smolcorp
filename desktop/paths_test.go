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

// N3 (RGAPC): a temp file left by a crash with a wider mode never gives the key its mode.
func TestWritePrivateKeepsTheFileTo0600OverALeftoverTemp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix modes")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gm-token")
	if err := os.WriteFile(path+".tmp", []byte("left by a crash"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path+".tmp", 0o644); err != nil { // whatever the umask made it
		t.Fatal(err)
	}
	if err := writePrivate(path, []byte("secret\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the file is %v (%v), want 0600", info, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "secret\n" {
		t.Errorf("the file holds %q", got)
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("the temp file is still there")
	}
	// Again, over the old file: still 0600.
	if err := writePrivate(path, []byte("second\n")); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the second write left %v (%v)", info, err)
	}
	// Into a folder that isn't there: an error, nothing made.
	if err := writePrivate(filepath.Join(dir, "none", "file"), []byte("x")); err == nil {
		t.Error("a missing folder wrote")
	}
}
