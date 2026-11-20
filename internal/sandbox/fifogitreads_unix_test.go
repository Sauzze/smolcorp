//go:build unix

package sandbox

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// RT126SWEEP4 part 3's B1: a FIFO whoever can write a repo's .git puts where a desk's start reads the shared .git (its
// origin/HEAD, its worktrees folder, a ref's file) is never waited on: each read answers at once and takes nothing from
// the FIFO. Stand-ins only: the test's own temp folder; a read still waiting after the bound is let go by the test.
func TestAFIFOInTheSharedGitIsNeverWaitedOn(t *testing.T) {
	git := t.TempDir()
	fifo := func(rel string) string {
		at := filepath.Join(git, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(at, 0o600); err != nil {
			t.Fatal(err)
		}
		return at
	}
	originHEAD, worktrees, ref := fifo("refs/remotes/origin/HEAD"), fifo("worktrees"), fifo("refs/heads/main")
	answersAtOnce(t, "DefaultBranch", originHEAD, func() {
		if got := DefaultBranch(git); got != "" {
			t.Errorf("a FIFO at origin/HEAD named %q", got)
		}
	})
	answersAtOnce(t, "the worktrees' HEADs", worktrees, func() {
		p := &Policy{goos: "darwin", git: filepath.ToSlash(git)}
		if err := p.worktreeHEADs(); err != nil {
			t.Errorf("a FIFO at worktrees, which git never reads as registrations: %v", err)
		}
	})
	answersAtOnce(t, "a ref's first bytes", ref, func() {
		if text, err := fileStart(filepath.ToSlash(ref)); err == nil {
			t.Errorf("a FIFO read as a ref's file: %q", text)
		}
	})
}

// answersAtOnce runs read, which must be done within 3s; past that the row fails, and the FIFO at fifo is fed an end
// of file until read comes back.
func answersAtOnce(t *testing.T, what, fifo string, read func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); read() }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Errorf("%s waited on the FIFO at %s", what, fifo)
		for {
			if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				w.Close()
			}
			select {
			case <-done:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}
