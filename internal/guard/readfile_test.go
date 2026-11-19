package guard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// soon fails the test if read hasn't returned within a few seconds: a guard
// file that can stall its reader must be refused at once, not waited on.
func soon(t *testing.T, what string, read func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		read()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: the read hung", what)
	}
}

// refusedEverywhere checks that the file at path is a broken file to Load and
// to --print, and that the hook then denies every command, at once (SG0).
func refusedEverywhere(t *testing.T, what, path, want string) {
	t.Helper()
	soon(t, what, func() {
		if _, err := ReadFile(path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: ReadFile said %v, want %q", what, err, want)
		}
		groups, problem := Load(path)
		if problem == "" {
			t.Errorf("%s: loaded with no problem", what)
		}
		if Check("git push", groups) == nil {
			t.Errorf("%s: not the default groups", what)
		}
		var out bytes.Buffer
		Run([]string{"--rules", path}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &bytes.Buffer{})
		if ok, why := denied(t, out.String()); !ok || !strings.Contains(why, "broken") {
			t.Errorf("%s: the hook allowed ls: %q", what, out.String())
		}
		out.Reset()
		Run([]string{"--profile", "gm", "--rules", path}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &bytes.Buffer{})
		if ok, _ := denied(t, out.String()); !ok {
			t.Errorf("%s: the GM's hook allowed ls: %q", what, out.String())
		}
	})
}

// A plain guard file with one name loads as before, up to 64 KiB exactly.
func TestAPlainGuardFileStillLoads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	text := `{"groups":[{"name":"push","on":false,"commands":["git push"]},{"name":"mine","on":true,"commands":["make ship"]}]}`
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	soon(t, "plain", func() {
		groups, problem := Load(path)
		if problem != "" || len(groups) != 2 || Check("git push", groups) != nil || Check("make ship", groups) == nil {
			t.Errorf("%v, %q", groups, problem)
		}
		var out bytes.Buffer
		Run([]string{"--rules", path}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &bytes.Buffer{})
		if out.Len() != 0 {
			t.Errorf("ls refused: %q", out.String())
		}
	})
	// Exactly the cap, padded with spaces after the JSON: still a guard file.
	full := filepath.Join(dir, "full.json")
	if err := os.WriteFile(full, []byte(text+strings.Repeat(" ", fileMost-len(text))), 0o600); err != nil {
		t.Fatal(err)
	}
	if groups, problem := Load(full); problem != "" || len(groups) != 2 {
		t.Errorf("a file of exactly 64 KiB: %v, %q", groups, problem)
	}
	// No file at all is still the default, and nothing wrong.
	if groups, problem := Load(filepath.Join(dir, "none.json")); problem != "" || Check("git push", groups) == nil {
		t.Errorf("missing: %q", problem)
	}
}

// A file over 64 KiB is refused without being read whole.
func TestALargeGuardFileIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	text := `{"groups":[{"name":"push","on":false,"commands":["git push"]}]}`
	if err := os.WriteFile(path, []byte(text+strings.Repeat(" ", fileMost)), 0o600); err != nil {
		t.Fatal(err)
	}
	refusedEverywhere(t, "large", path, "larger than 64 KiB")
}

// A second name for the guard file (a hard link a hire made, or one it can
// change through) is refused, both names.
func TestAHardLinkedGuardFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, DefaultFile(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(dir, "other")); err != nil {
		t.Skipf("no hard links here: %v", err)
	}
	refusedEverywhere(t, "hard link", path, "more than one name")
	refusedEverywhere(t, "hard link, other name", filepath.Join(dir, "other"), "more than one name")
}

// A link in the guard file's place is refused, even to a good guard file.
func TestASymlinkedGuardFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere.json")
	if err := os.WriteFile(target, DefaultFile(), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fileName)
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	refusedEverywhere(t, "symlink", path, "link")
	// A link to nowhere is a link too, not a missing file (which would be the default and no problem).
	dangling := filepath.Join(dir, "dangling.json")
	if err := os.Symlink(filepath.Join(dir, "nothing"), dangling); err != nil {
		t.Fatal(err)
	}
	refusedEverywhere(t, "dangling symlink", dangling, "link")
}

// A folder in the guard file's place is not a plain file.
func TestAFolderGuardFileIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	refusedEverywhere(t, "folder", path, "isn't a plain file")
}
