package desktop

import (
	"errors"
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
	if left, _ := filepath.Glob(path + ".*.tmp"); len(left) != 0 {
		t.Errorf("the temp file is still there: %v", left)
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

// refusePrivateWrites makes every private write to path fail before anything is made, as a full disk would, until
// the func it gives is called or the test ends.
func refusePrivateWrites(t *testing.T, path string) (allow func()) {
	refuse := func(at string) error {
		if at == path {
			return &os.PathError{Op: "open", Path: at, Err: errors.New("no space left on device (a test's)")}
		}
		return nil
	}
	privateWriteRefused.Store(&refuse)
	allow = func() { privateWriteRefused.Store(nil) }
	t.Cleanup(allow)
	return allow
}

// tempOfFailedWrite is the file a private write that failed at its rename went through.
func tempOfFailedWrite(t *testing.T, err error) string {
	t.Helper()
	var renamed *os.LinkError
	if !errors.As(err, &renamed) || renamed.Op != "rename" {
		t.Fatalf("not a rename's failure: %v", err)
	}
	return renamed.Old
}

// FXWRITEPRIVATE: a private write goes through a name of its own each time, beside the file, and one that fails
// leaves nothing behind.
func TestAPrivateWriteGoesThroughANewNameEachTimeAndLeavesNoneBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// A folder where the file goes: the write fails at its rename, on Linux and Windows alike.
	if err := os.MkdirAll(filepath.Join(path, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for range 3 {
		err := writePrivate(path, []byte("{}\n"))
		if err == nil {
			t.Fatal("written over a folder")
		}
		temp := tempOfFailedWrite(t, err)
		if filepath.Dir(temp) != dir || !strings.HasPrefix(filepath.Base(temp), "config.json.") || !strings.HasSuffix(temp, ".tmp") || filepath.Base(temp) == "config.json.tmp" {
			t.Errorf("written through %s", temp)
		}
		if seen[temp] {
			t.Errorf("%s twice", temp)
		}
		seen[temp] = true
		if _, err := os.Lstat(temp); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is left: %v", temp, err)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 || entries[0].Name() != "config.json" || !entries[0].IsDir() {
		t.Errorf("the folder holds %v", entries)
	}
	// Refused before anything is made: nothing either.
	refusePrivateWrites(t, path+"2")
	if err := writePrivate(path+"2", []byte("{}\n")); err == nil {
		t.Error("a refused write wrote")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "config.json2*")); len(left) != 0 {
		t.Errorf("left: %v", left)
	}
}

// FXWRITEPRIVATE: nothing put at a name that could be guessed beforehand is written through or stops the write: not
// the fixed name an older Smolcorp wrote through, a link or a folder there, nor a link at the file's own name, which
// is replaced and never followed.
func TestAPrivateWriteNeverGoesThroughWhatWasPutBesideIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.local.json")
	elsewhere := filepath.Join(dir, "elsewhere")
	if err := os.WriteFile(elsewhere, []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	untouched := func(when string) {
		t.Helper()
		if data, err := os.ReadFile(elsewhere); err != nil || string(data) != "untouched\n" {
			t.Errorf("%s: written through to elsewhere: %q %v", when, data, err)
		}
	}
	holds := func(when, want string) {
		t.Helper()
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("%s: the file is %v %v", when, info, err)
		}
		if data, _ := os.ReadFile(path); string(data) != want || yoursAlone(path) != nil {
			t.Errorf("%s: %q, %v", when, data, yoursAlone(path))
		}
		if left, _ := filepath.Glob(path + ".*.tmp"); len(left) != 0 {
			t.Errorf("%s: left %v", when, left)
		}
	}
	// A folder, with something in it, at the old fixed name: it stops nothing, and is left as it was.
	if err := os.MkdirAll(filepath.Join(path+".tmp", "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writePrivate(path, []byte("one\n")); err != nil {
		t.Fatalf("a folder at the old name stopped the write: %v", err)
	}
	holds("past a folder at the old name", "one\n")
	if err := os.RemoveAll(path + ".tmp"); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return // a link needs a privilege to make
	}
	// A link at the old fixed name: taken away, never written through.
	if err := os.Symlink(elsewhere, path+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := writePrivate(path, []byte("two\n")); err != nil {
		t.Fatal(err)
	}
	untouched("a link at the old name")
	holds("past a link at the old name", "two\n")
	if _, err := os.Lstat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the link at the old name is left: %v", err)
	}
	// A link at the file's own name: replaced by the file, never followed.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, path); err != nil {
		t.Fatal(err)
	}
	if err := writePrivate(path, []byte("three\n")); err != nil {
		t.Fatal(err)
	}
	untouched("a link at the file")
	holds("over a link at the file", "three\n")
}
