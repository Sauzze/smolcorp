//go:build !windows

package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FXRC3FIX (RT126RC3's F2): the fire's look at the object store reads info/alternates as it reads the stored copies,
// never waiting on it and only as a regular file. A GM that swapped it for a named pipe once git count-objects had read
// it (in a repo of the owner's it can write) held that hire's fire without end. Here the pipe is put there from the
// start, and the look's store listing (the step after git's) is what the fire runs: it answers at once, keeps the
// worktree and says it couldn't check. Stand-ins only, in the test's temp folder; the pipe is never written.
func TestAFIFOAtTheStoresAlternatesNeverHoldsTheFire(t *testing.T) {
	objects := filepath.Join(physical(t.TempDir()), "objects")
	if err := os.MkdirAll(filepath.Join(objects, "info"), 0o700); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(objects, "info", "alternates")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Should the look still be waiting, a writer lets it go before the folder is removed.
		if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
	})
	look := func(ctx context.Context, repo string, tips []string) (int, string, error) {
		_, err := storeSecondCopies(ctx, []string{objects}, 20)
		return 0, "", err
	}
	said := make(chan string, 1)
	go func() {
		said <- fireStoreKept(Hire{Name: "Rex", Repo: filepath.Dir(objects), Worktree: keptPath(filepath.Join(filepath.Dir(objects), "w")), Branch: "hire/x"}, look)
	}()
	select {
	case words := <-said:
		want := ": it couldn't check the repo's stored objects (" + fifo + " isn't a plain file, which git never makes), so it didn't look at whether the branch is merged"
		if !strings.HasPrefix(words, "Smolcorp kept ") || !strings.Contains(words, want) {
			t.Errorf("the owner is told %q; want it to say %q", words, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the fire's look is still waiting on the named pipe after 10 s")
	}
	// A plain file there is still read, as git reads it: a store named through a link fails the look closed.
	if err := os.Remove(fifo); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(physical(t.TempDir()), "other")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(objects), "linked")
	if err := os.Symlink(other, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fifo, []byte("# a comment\n"+link+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := alternatesNoLinks(objects); err == nil || !strings.Contains(err.Error(), "the other store "+link) {
		t.Errorf("a store named through a link: %v", err)
	}
}
