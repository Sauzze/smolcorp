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

// RT126SWEEP4 part 3's F3: T3's fence line goes into the repo's info/exclude only as a plain file of the shared .git,
// read without waiting and written through that folder alone. A FIFO there (whoever can write the repo's .git can put
// one) is never waited on; a link there to a file outside the .git leaves that file as it was. Either way t3Exclude
// says no, so the T3 step is refused rather than left unfenced. Stand-ins only: the test's own repo and files.
func TestT3sExcludeIsAPlainFileOfTheSharedGitOnly(t *testing.T) {
	setup := func(t *testing.T) (repo, wt, exclude string) {
		isolate(t)
		repo = placeRepo(t)
		wt = filepath.Join(physical(t.TempDir()), "w-t3")
		gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-t3", wt)
		exclude = filepath.Join(repo, ".git", "info", "exclude")
		_ = os.Remove(exclude)
		return repo, wt, exclude
	}
	t.Run("a FIFO", func(t *testing.T) {
		_, wt, exclude := setup(t)
		if err := syscall.Mkfifo(exclude, 0o644); err != nil {
			t.Fatal(err)
		}
		var ok bool
		done := make(chan struct{})
		go func() { defer close(done); ok = t3Exclude(wt, runGit) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("t3Exclude waited on a FIFO at info/exclude")
			// Let it go, whichever end it waits for: a reader's open or read, or a writer's open.
			for waiting := true; waiting; {
				for _, flag := range []int{os.O_WRONLY, os.O_RDONLY} {
					if f, err := os.OpenFile(exclude, flag|syscall.O_NONBLOCK, 0); err == nil {
						f.Close()
					}
				}
				select {
				case <-done:
					waiting = false
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
		if info, err := os.Lstat(exclude); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
			t.Errorf("the FIFO was replaced: %v %v", info, err)
		}
		if ok {
			t.Error("a FIFO at info/exclude was taken for the fence's line")
		}
	})
	t.Run("a link out of the .git", func(t *testing.T) {
		_, wt, exclude := setup(t)
		outside := filepath.Join(physical(t.TempDir()), "outside")
		writeFile(t, outside, "the owner's own\n")
		if err := os.Symlink(outside, exclude); err != nil {
			t.Fatal(err)
		}
		ok := t3Exclude(wt, runGit)
		if data, _ := os.ReadFile(outside); string(data) != "the owner's own\n" {
			t.Errorf("the file outside the .git was written: %q", data)
		}
		if ok {
			t.Error("a link out of the .git was taken for the fence's line")
		}
	})
	t.Run("the control: a plain file", func(t *testing.T) {
		_, wt, exclude := setup(t)
		writeFile(t, exclude, "# git's own\n*.tmp")
		if !t3Exclude(wt, runGit) {
			t.Fatal("not fenced")
		}
		if data, _ := os.ReadFile(exclude); string(data) != "# git's own\n*.tmp\n"+t3FenceName+"\n" {
			t.Errorf("exclude: %q", data)
		}
		if !t3Exclude(wt, runGit) {
			t.Error("asked again, it isn't ignored")
		}
		if data, _ := os.ReadFile(exclude); strings.Count(string(data), t3FenceName) != 1 {
			t.Errorf("written twice: %q", data)
		}
	})
}
