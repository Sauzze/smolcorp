//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A hire writes its own folder: what the recap reads there is only a plain
// file, never through a link, never a pipe, never a second name of another file.
func TestTheRecapReadsOnlyPlainFilesInsideTheHiresFolder(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	os.WriteFile(secret, []byte("SECRET-LINE\n"), 0o600)
	os.WriteFile(filepath.Join(outside, "STATUS.md"), []byte("SECRET-LINE\n"), 0o600)
	os.WriteFile(filepath.Join(outside, "BRIEF.md"), []byte("# SECRET-LINE\n"), 0o600)
	for name, mk := range map[string]func(dir string) error{
		"a link":                func(dir string) error { return os.Symlink(secret, filepath.Join(dir, "STATUS.md")) },
		"a linked .team folder": func(dir string) error { return os.Symlink(outside, filepath.Join(dir, ".team")) },
		"a hard link":           func(dir string) error { return os.Link(secret, filepath.Join(dir, "STATUS.md")) },
		"a linked brief": func(dir string) error {
			return os.Symlink(filepath.Join(outside, "BRIEF.md"), filepath.Join(dir, "BRIEF.md"))
		},
		"a brief in a hard link": func(dir string) error {
			return os.Link(filepath.Join(outside, "BRIEF.md"), filepath.Join(dir, "BRIEF.md"))
		},
		"a pipe for the status": func(dir string) error { return syscall.Mkfifo(filepath.Join(dir, "STATUS.md"), 0o600) },
		"a pipe in .team": func(dir string) error {
			os.Mkdir(filepath.Join(dir, ".team"), 0o700)
			return syscall.Mkfifo(filepath.Join(dir, ".team", "STATUS.md"), 0o600)
		},
		"a pipe for the brief": func(dir string) error { return syscall.Mkfifo(filepath.Join(dir, "BRIEF.md"), 0o600) },
	} {
		dir := t.TempDir()
		if err := mk(dir); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		done := make(chan string, 1)
		go func() {
			done <- strings.Join(lastWords(5, dir, "STATUS.md", filepath.Join(".team", "STATUS.md")), "|") + recapTask(dir)
		}()
		select {
		case got := <-done:
			if got != "" {
				t.Errorf("%s: read %q", name, got)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s: the recap's read blocked", name)
		}
	}
	// A plain status and brief are still read, also in a real .team folder.
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".team"), 0o700)
	os.WriteFile(filepath.Join(dir, ".team", "STATUS.md"), []byte("one\ntwo\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "BRIEF-x.md"), []byte("\n# Paint the door\n"), 0o644)
	if got := lastWords(5, dir, "STATUS.md", filepath.Join(".team", "STATUS.md")); strings.Join(got, "|") != "one|two" {
		t.Errorf("plain status: %q", got)
	}
	if got := recapTask(dir); got != "Paint the door" {
		t.Errorf("plain brief: %q", got)
	}
}

// A pipe left as STATUS.md in a kept worktree never holds a fire: it answers,
// the hire is fired and the recap is written.
func TestAPipeInAKeptWorktreeNeverHoldsTheFire(t *testing.T) {
	_, r, hire := hiredForFire(t, "")
	os.WriteFile(filepath.Join(hire.Worktree, "new.txt"), []byte("x"), 0o644)
	if err := syscall.Mkfifo(filepath.Join(hire.Worktree, "STATUS.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	windowShows(r, pane(t, "idle-empty"), true)
	done := make(chan error, 1)
	go func() { _, err := fireEngineHire(hire, 0, r.o, true); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the fire blocked on a pipe in the worktree")
	}
	data, _ := os.ReadFile(filepath.Join(supportDir, "recaps", hire.ID+".md"))
	if recap := string(data); !strings.Contains(recap, "(kept)") || !strings.Contains(recap, "Nothing was written to STATUS.md.") {
		t.Errorf("recap:\n%s", recap)
	}
	if got := registered(t); len(got) != 1 || got[0].State != "fired" {
		t.Errorf("registry: %+v", got)
	}
}
