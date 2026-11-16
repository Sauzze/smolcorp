//go:build !windows

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// RT126SAX6 B2: the checkpoint store's add reads a copy the core's one reader made (cpMirror), never the desk's folder,
// where git looks at a file and then opens it by its path, following a link swapped in between. Stand-ins only: a
// stand-in secret outside the desk's folder, the size of the decoys.

// cpSecretRig is a repo with a stand-in secret beside it (outside it), and the secret's blob id as the repo names it.
func cpSecretRig(t *testing.T) (dir, secret, blob string) {
	t.Helper()
	cpScratch(t)
	dir = aGitRepo(t)
	secret = filepath.Join(physical(t.TempDir()), "secret")
	if err := os.WriteFile(secret, []byte("FAKEKEY-standin-secret-"+strings.Repeat("S", 377)), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, secret, gitRepoCmd(t, dir, "hash-object", secret)
}

// cpHolds is whether the repo has the object.
func cpHolds(dir, blob string) bool {
	return plainGitErr(dir, "cat-file", "-e", blob) == nil
}

func plainGitErr(dir string, args ...string) error {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Run()
}

// A hard link in the desk's folder is a name of a file elsewhere: the snapshot is refused with words, and the file's
// bytes are in no object. As add read it, they were.
func TestASnapshotTakesNoFileThatHasAnotherName(t *testing.T) {
	dir, secret, blob := cpSecretRig(t)
	if err := os.Link(secret, filepath.Join(dir, "decoy.txt")); err != nil {
		t.Skipf("no hard link here: %v", err)
	}
	cp, note := newCheckpoints().snapshot(cpDesk(dir), "x")
	if cp.ID != "" || note != string(errCpHardLink) {
		t.Errorf("the snapshot: %+v %q, want refused for the hard link", cp, note)
	}
	if cpHolds(dir, blob) {
		t.Error("the stand-in secret's bytes are in the repo")
	}
}

// A desk keeps flipping its decoys between a plain file and a link at the stand-in secret, by atomic renames, while
// snapshots are taken (up to 15 s, or until the secret is in the repo). No snapshot holds the secret's bytes; a link is kept as a link, and the exec bit as it is.
func TestASnapshotTakesNoFileALinkSwappedInLeadsTo(t *testing.T) {
	dir, secret, blob := cpSecretRig(t)
	writeRepoFile(t, dir, "run.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(dir, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	const decoys = 8
	staging := filepath.Join(filepath.Dir(dir), "flip")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	decoy := strings.Repeat("d", 400)
	for i := range decoys {
		writeRepoFile(t, dir, "decoys/"+strconv.Itoa(i), decoy)
	}
	// One flipper a decoy, as fast as it goes: the decoy turns from a link at the secret to a plain file and back, each
	// by one rename of one made beforehand, so it changes every few microseconds.
	stop := make(chan struct{})
	var flippers sync.WaitGroup
	for i := range decoys {
		flippers.Add(1)
		go func() {
			defer flippers.Done()
			at := filepath.Join(dir, "decoys", strconv.Itoa(i))
			link, plain := filepath.Join(staging, "l"+strconv.Itoa(i)), filepath.Join(staging, "p"+strconv.Itoa(i))
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = os.Symlink(secret, link)
				_ = os.Rename(link, at)
				_ = os.WriteFile(plain, []byte(decoy), 0o644)
				_ = os.Rename(plain, at)
			}
		}()
	}
	stopped := false
	halt := func() {
		if !stopped {
			stopped = true
			close(stop)
			flippers.Wait()
		}
	}
	defer halt()
	store := newCheckpoints()
	taken := 0
	for end := time.Now().Add(15 * time.Second); time.Now().Before(end) && !cpHolds(dir, blob); {
		if cp, _ := store.snapshot(cpDesk(dir), "x"); cp.ID != "" {
			taken++
		}
	}
	halt()
	if cpHolds(dir, blob) {
		t.Fatalf("a snapshot holds the stand-in secret's bytes (after %d snapshots)", taken)
	}
	if taken == 0 {
		t.Fatal("no snapshot was taken while the decoys flipped, so the race wasn't run")
	}
	cp := newSnap(t, store, cpDesk(dir), "after")
	tree := gitRepoCmd(t, dir, "ls-tree", cp.ID, "link", "run.sh")
	if !strings.Contains(tree, "120000 blob") || !strings.Contains(tree, "100755 blob") {
		t.Errorf("the link and the exec bit: %q", tree)
	}
	t.Logf("%d snapshots while the decoys flipped", taken)
}
