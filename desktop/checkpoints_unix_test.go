//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// B9, the verdict's P2b: two hires, each in its own linked worktree, each flipping its own
// worktrees/<id>/info/sparse-checkout between a file and a fifo (a hard link renamed in, so the swap is atomic) while
// their snapshots run. A fifo opened to be read waits for a writer; the store opens what is there without waiting and
// judges what it opened, so every snapshot returns within its cap (taken, or refused in words), and the owner's own desk
// still gets its slot.
func TestAFifoSwappedInNeverBlocksASnapshotOrTheOtherDesks(t *testing.T) {
	cpScratch(t)
	defer func(d time.Duration) { checkpointTimeout = d }(checkpointTimeout)
	checkpointTimeout = 5 * time.Second
	owner := aGitRepo(t)
	store := newCheckpoints()
	stop := make(chan struct{})
	var flipping sync.WaitGroup
	var fifos []string
	defer func() {
		close(stop)
		flipping.Wait()
		for _, fifo := range fifos { // a read left waiting on one (a failed run) is let go: a writer comes and goes
			if file, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				_ = file.Close()
			}
		}
	}()
	var hires []checkpointDesk
	for _, id := range []string{"h0", "h1"} {
		tree := filepath.Join(physical(t.TempDir()), id)
		gitRepoCmd(t, owner, "worktree", "add", "-q", "-b", id, tree)
		writeRepoFile(t, tree, "work.txt", id+"\n")
		hires = append(hires, checkpointDesk{Session: id, Folder: tree})
		target := filepath.Join(gitRepoCmd(t, tree, "rev-parse", "--absolute-git-dir"), "info", "sparse-checkout")
		side := physical(t.TempDir())
		file, fifo, next := filepath.Join(side, "file"), filepath.Join(side, "fifo"), filepath.Join(side, "next")
		writeRepoFile(t, side, "file", "/*\n")
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Fatal(err)
		}
		fifos = append(fifos, fifo)
		writeRepoFile(t, filepath.Dir(target), "sparse-checkout", "/*\n")
		flipping.Add(1)
		go func() {
			defer flipping.Done()
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				from := file
				if n%2 == 0 {
					from = fifo
				}
				if err := os.Link(from, next); err != nil {
					t.Error(err)
					return
				}
				if err := os.Rename(next, target); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wait := checkpointTimeout + 5*time.Second
	var running sync.WaitGroup
	var mu sync.Mutex
	taken, refused := 0, 0
	for _, desk := range hires {
		running.Add(1)
		go func() {
			defer running.Done()
			for i := 0; i < 200; i++ {
				answer := make(chan string, 1)
				go func() { _, note := store.snapshot(desk, "flip"); answer <- note }()
				select {
				case note := <-answer:
					mu.Lock()
					switch {
					case note == "":
						taken++
					case strings.Contains(note, "sparse-checkout isn't a file that can be read"):
						refused++
					default:
						t.Errorf("%s, snapshot %d: %q", desk.Session, i+1, note)
					}
					mu.Unlock()
				case <-time.After(wait):
					t.Errorf("%s: snapshot %d hadn't returned after %s", desk.Session, i+1, wait)
					return
				}
			}
		}()
	}
	running.Wait()
	if t.Failed() {
		t.FailNow()
	}
	if refused == 0 || taken == 0 {
		t.Errorf("the flip was never seen (taken %d, refused %d), so this test proves less than it says", taken, refused)
	}
	// The owner's own desk, the hires still flipping: a slot is free and its snapshot is taken.
	started := time.Now()
	if cp, note := store.snapshot(checkpointDesk{Session: "owner", Folder: owner}, "owner"); cp.ID == "" || note != "" || time.Since(started) > checkpointTimeout {
		t.Errorf("the owner's desk: %+v %q after %s", cp, note, time.Since(started))
	}
}
