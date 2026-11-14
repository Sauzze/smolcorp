//go:build !windows

package desktop

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// The .git swap race, under load (T126CK1): many snapshots at once, each with its prune, while the desk's .git keeps
// flipping between its own folder and another repo (a gitdir: file, or a link). Passing alone was never the bar: this
// runs the swap against every step at once. The other repo gets nothing: no ref, no folder or lock file under
// refs/smolcorp, no object, and its filter never runs; every ref the desk gets is of the desk's own files.
func TestManySnapshotsWhileTheDotGitFlipsBetweenReposWriteNothingInTheOther(t *testing.T) {
	cpScratch(t)
	defer func(n int) { checkpointsKept = n }(checkpointsKept)
	checkpointsKept = 2 // so each snapshot's prune deletes refs while the .git flips too
	h := &hostileRepo{markers: physical(t.TempDir())}
	desk := aGitRepo(t)
	evil := anEvilRepo(t, h)
	evilObjects := gitRepoCmd(t, evil, "count-objects")
	dotGit, aside, evilGit := filepath.Join(desk, ".git"), filepath.Join(desk, ".git.aside"), filepath.Join(evil, ".git")
	stop, done := make(chan struct{}), make(chan struct{})
	flips := 0
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if os.Rename(dotGit, aside) != nil {
				continue
			}
			if i%2 == 0 {
				_ = os.WriteFile(dotGit, []byte("gitdir: "+evilGit+"\n"), 0o644)
			} else {
				_ = os.Symlink(evilGit, dotGit)
			}
			flips++
			time.Sleep(time.Duration(i%3) * time.Millisecond)
			_ = os.RemoveAll(dotGit) // a folder a git of the store's made there, if any, goes with it
			_ = os.Rename(aside, dotGit)
			time.Sleep(time.Duration(4+i%20) * time.Millisecond) // the desk as it is, long enough for most steps
		}
	}()
	var running sync.WaitGroup
	var mu sync.Mutex
	kept, notes, logged := 0, map[string]int{}, 0
	defer func(say func(string)) { cpSay = say }(cpSay)
	cpSay = func(text string) {
		mu.Lock()
		defer mu.Unlock()
		if strings.Contains(text, "nothing was written") {
			logged++
		}
	}
	for w := 0; w < 6; w++ {
		running.Add(1)
		go func() {
			defer running.Done()
			store, desk := newCheckpoints(), checkpointDesk{Session: "s" + strconv.Itoa(w), Folder: desk}
			for i := 0; i < 20; i++ {
				cp, note := store.snapshot(desk, "storm")
				mu.Lock()
				if cp.ID != "" {
					kept++
				} else {
					notes[note]++
				}
				mu.Unlock()
			}
		}()
	}
	running.Wait()
	close(stop)
	<-done
	if info, err := os.Lstat(dotGit); err != nil || !info.IsDir() {
		t.Fatalf("the flip didn't put the desk's .git back: %v", err)
	}
	gitRepoCmd(t, desk, "fsck", "--strict", "--no-dangling") // the desk's repo is whole, refs and objects alike
	if ran := h.ran(); len(ran) != 0 {
		t.Errorf("the store ran: %v", ran)
	}
	if refs := cpRefs(t, evil); refs != "" {
		t.Errorf("the other repo got refs: %q", refs)
	}
	if _, err := os.Lstat(filepath.Join(evilGit, "refs", "smolcorp")); err == nil {
		t.Errorf("the other repo got a folder of checkpoint refs (or their locks)")
	}
	if got := gitRepoCmd(t, evil, "count-objects"); got != evilObjects {
		t.Errorf("the other repo got objects: %s, was %s", got, evilObjects)
	}
	for _, ref := range strings.Fields(cpRefs(t, desk)) {
		if text := gitRepoCmd(t, desk, "show", ref+":a.txt"); text != "one\ntwo\nthree" {
			t.Errorf("%s isn't of the desk's files: %q", ref, text)
		}
	}
	if notes[errDotGitMoved.Error()] > 0 && logged == 0 {
		t.Error("a swap was refused in words but not in the log")
	}
	if kept == 0 || flips == 0 {
		t.Errorf("kept %d, flips %d: the race wasn't run", kept, flips)
	}
	t.Logf("%d of 120 kept over %d flips, %d refusals logged; the others: %v", kept, flips, logged, notes)
}

// cpRefModes is the mode of everything under a git folder's refs/smolcorp, and of its packed-refs, by path.
func cpRefModes(t *testing.T, gitDir string) map[string]string {
	t.Helper()
	modes := map[string]string{}
	_ = filepath.WalkDir(filepath.Join(gitDir, "refs", "smolcorp"), func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := os.Lstat(path); err == nil {
			rel, _ := filepath.Rel(gitDir, path)
			modes[filepath.ToSlash(rel)] = info.Mode().String()
		}
		return nil
	})
	if info, err := os.Lstat(filepath.Join(gitDir, "packed-refs")); err == nil {
		modes["packed-refs"] = info.Mode().String()
	}
	return modes
}

// RT126CK1 B1: in a repo shared by core.sharedRepository (set on an existing repo, the documented way), each folder
// and ref the store makes has the mode git's own update-ref gives it in a twin of the repo: the folders past the
// umask, with setgid when the group gets anything, the refs at git's mode for them. So does packed-refs after a drop.
func TestASharedRepoGetsGitsModesForItsCheckpointRefs(t *testing.T) {
	for _, shared := range []struct {
		value string
		ref   func(fs.FileMode) bool // what the ref's mode has to be, whatever the umask
	}{
		{"group", func(m fs.FileMode) bool { return m&0o660 == 0o660 }},
		{"0600", func(m fs.FileMode) bool { return m == 0o600 }},
		{"0640", func(m fs.FileMode) bool { return m == 0o640 }},
	} {
		t.Run(shared.value, func(t *testing.T) {
			cpScratch(t)
			desk := aGitRepo(t)
			gitRepoCmd(t, desk, "config", "core.sharedRepository", shared.value)
			twin := filepath.Join(physical(t.TempDir()), "twin")
			if out, err := exec.Command("cp", "-a", desk, twin).CombinedOutput(); err != nil {
				t.Fatalf("%v %s", err, out)
			}
			mine, git := filepath.Join(desk, ".git"), filepath.Join(twin, ".git")
			same := func(step string, want ...string) {
				t.Helper()
				store, twin := cpRefModes(t, mine), cpRefModes(t, git)
				for _, path := range want {
					if store[path] == "" || twin[path] == "" {
						t.Fatalf("%s: %s isn't there: the store's %v, git's %v", step, path, store, twin)
					}
				}
				for path, mode := range twin {
					if got, there := store[path]; there && got != mode {
						t.Errorf("%s: %s is %s, git makes it %s", step, path, got, mode)
					}
				}
			}
			store := newCheckpoints()
			for n := 1; n <= 2; n++ {
				newSnap(t, store, cpDesk(desk), "shared")
				gitRepoCmd(t, twin, "update-ref", "refs/smolcorp/checkpoints/s1/"+strconv.Itoa(n), "HEAD")
				ref := "refs/smolcorp/checkpoints/s1/" + strconv.Itoa(n)
				same("snapshot "+strconv.Itoa(n), "refs/smolcorp", "refs/smolcorp/checkpoints", "refs/smolcorp/checkpoints/s1", ref)
				if info, err := os.Lstat(filepath.Join(mine, filepath.FromSlash(ref))); err != nil || !shared.ref(info.Mode()) {
					t.Errorf("snapshot %d: the ref is %v %v", n, info.Mode(), err)
				}
			}
			// Packed, then one dropped: by the store in the repo, by git's own delete in the twin.
			gitRepoCmd(t, desk, "pack-refs", "--all")
			gitRepoCmd(t, twin, "pack-refs", "--all")
			if note := store.keepOnly(cpDesk(desk), 1); note != "" {
				t.Fatalf("the drop: %q", note)
			}
			gitRepoCmd(t, twin, "update-ref", "-d", "refs/smolcorp/checkpoints/s1/1")
			same("drop", "packed-refs")
			if refs := cpRefs(t, desk); refs != "refs/smolcorp/checkpoints/s1/2" {
				t.Errorf("after the drop: %q", refs)
			}
			gitRepoCmd(t, desk, "fsck", "--strict", "--no-dangling")
		})
	}
}

// RT126CK1 N4: a link left in the desk's own .git where its folder of refs goes (here to refs/heads) is refused in
// words, and no branch is written through it.
func TestALinkWhereTheCheckpointRefsGoIsRefused(t *testing.T) {
	cpScratch(t)
	desk := aGitRepo(t)
	if err := os.MkdirAll(filepath.Join(desk, ".git", "refs", "smolcorp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "heads"), filepath.Join(desk, ".git", "refs", "smolcorp", "checkpoints")); err != nil {
		t.Fatal(err)
	}
	cp, note := newCheckpoints().snapshot(cpDesk(desk), "link")
	if cp.ID != "" || note != errRefsNotFolder.Error() {
		t.Errorf("the snapshot: %+v %q", cp, note)
	}
	if heads := gitRepoCmd(t, desk, "for-each-ref", "--format=%(refname)", "refs/heads"); heads != "refs/heads/main" {
		t.Errorf("a branch was written through the link: %q", heads)
	}
}
