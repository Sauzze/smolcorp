package sandbox

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXGITROWS2R1 (RT126FIXREVIEW-GITROWS2's B1 and O1) under the real profile, with newRowsDesk's stand-ins: the hire h1
// on hire/h1/work, another desk h2 on hire/h2/work, and here the owner's checkout on topic/x, a branch with a slash,
// beside topic/y.

// newSlashDesk is newRowsDesk with the owner's checkout on topic/x, its profile written again for it.
func newSlashDesk(t *testing.T, git string) (*escapeDesk, func(dir string, args ...string) string) {
	t.Helper()
	d, owner := newRowsDesk(t, git)
	owner(d.checkout, "switch", "-q", "-c", "topic/x")
	owner(d.checkout, "branch", "topic/y")
	rowsPlaces(&d.pl, "topic/x")
	d.prepare(t)
	return d, owner
}

// packAndDelete is what the owner's git and the core's fire do outside any sandbox: a hire's branch made, every ref
// packed (`git gc`, `pack-refs`, an auto gc), the hire removed and its branch deleted (hireplace.go's `branch -D`),
// and the owner's own topic/y deleted. Git then removes every folder below refs/heads it finds empty.
func packAndDelete(d *escapeDesk, owner func(dir string, args ...string) string) {
	h3 := filepath.Join(d.home, "app-hires", "h3")
	owner(d.checkout, "worktree", "add", "-q", "-b", "hire/h3/work", h3)
	owner(d.checkout, "pack-refs", "--all")
	owner(d.checkout, "worktree", "remove", "--force", h3)
	owner(d.checkout, "branch", "-q", "-D", "hire/h3/work", "topic/y")
}

// B1: once every ref is packed and a branch below it deleted, refs/heads/hire is still there (its keeper), so no desk
// renames a folder it prepared in its place, with another desk's branch inside, which would shadow the packed one. Nor
// does it remove, fill or move a keeper. refs/heads/topic, the owner checkout's folder, needs no keeper since
// FXGITROWS2R4: no desk makes anything there, gone or not. Its own commit still works; the owner's fsck and
// for-each-ref don't see the keepers.
func TestOnAMacARefFolderGitWouldRemoveStaysAndNoDeskRenamesOneIn(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newSlashDesk(t, git)
	h2tree := filepath.Join(d.home, "app-hires", "h2")
	h2, x, h2head := owner(d.checkout, "rev-parse", "hire/h2/work"), owner(d.checkout, "rev-parse", "topic/x"), owner(h2tree, "rev-parse", "HEAD")
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
		t.Fatalf("the desk's commit: %+v", r)
	}
	packAndDelete(d, owner)
	heads, logs := filepath.Join(d.git, "refs", "heads"), filepath.Join(d.git, "logs", "refs", "heads")
	for _, folder := range []string{heads + "/hire", logs + "/hire", filepath.Join(d.git, "refs", "remotes", "origin")} {
		if info, err := os.Stat(filepath.Join(folder, ".smolcorp-keep")); err != nil || !info.IsDir() {
			t.Errorf("%s: no keeper (%v)", folder, err)
		}
	}
	// What the desk prepared in its worktree: its own commit, as another desk's branch, the owner's, and a reflog.
	desks := owner(d.root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(d.root, "x", "h2", "work"), desks+"\n")
	mustWrite(t, filepath.Join(d.root, "y", "x"), desks+"\n")
	mustWrite(t, filepath.Join(d.root, "l", "h2", "work"), "the desk's\n")
	keeper := heads + "/hire/.smolcorp-keep"
	d.probe(t, python, [][]string{{"rename", filepath.Join(d.root, "x"), heads + "/hire"}, {"rename", filepath.Join(d.root, "y"), heads + "/topic"},
		{"rename", filepath.Join(d.root, "l"), logs + "/hire"}, {"rmdir", keeper}, {"mkdir", keeper + "/x"},
		{"rename", keeper, filepath.Join(d.root, "k")}, {"rmdir", logs + "/hire/.smolcorp-keep"}, {"mkdir", heads + "/topic"}},
		times(8, "PermissionError"))
	if got := owner(d.checkout, "rev-parse", "hire/h2/work"); got != h2 {
		t.Errorf("hire/h2/work moved: %s, was %s", got, h2)
	}
	if got := owner(h2tree, "rev-parse", "HEAD"); got != h2head {
		t.Errorf("the h2 worktree's HEAD moved: %s, was %s", got, h2head)
	}
	if got := owner(d.checkout, "rev-parse", "topic/x"); got != x {
		t.Errorf("topic/x, the owner's checked-out branch, moved: %s, was %s", got, x)
	}
	// Its own work.
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "after the fire"); r.status != 0 {
		t.Errorf("the desk's commit after the pack and the deletions: %+v", r)
	}
	if got, want := owner(d.checkout, "rev-parse", "hire/h1/work"), owner(d.root, "rev-parse", "HEAD"); got != want {
		t.Errorf("hire/h1/work: %s, want the desk's commit %s", got, want)
	}
	// The owner's git sees no keeper: no ref, no bad ref.
	if out, code := ownerTries(t, d, git, d.checkout, "fsck", "--no-progress"); code != 0 || strings.Contains(out, "error") {
		t.Errorf("the owner's fsck: %d %s", code, out)
	}
	if out := owner(d.checkout, "for-each-ref", "--format=%(refname)"); strings.Contains(out, "smolcorp") {
		t.Errorf("for-each-ref lists a keeper: %s", out)
	}
}

// B1, should the owner remove a keeper and its folder too: no desk makes the folder again, by mkdir, a rename or a
// link, so nothing shadows a packed branch; the desk's own commit stops (a stop, never a move) until its next start
// makes them again.
func TestOnAMacARefFolderTheOwnerRemovedIsNoDesksToMake(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newSlashDesk(t, git)
	h2 := owner(d.checkout, "rev-parse", "hire/h2/work")
	packAndDelete(d, owner)
	heads := filepath.Join(d.git, "refs", "heads")
	for _, folder := range []string{heads + "/hire", heads + "/topic"} {
		removeEmpty(t, folder)
	}
	desks := owner(d.root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(d.root, "x", "h2", "work"), desks+"\n")
	mustWrite(t, filepath.Join(d.root, "y", "x"), desks+"\n")
	d.probe(t, python, [][]string{{"mkdir", heads + "/hire"}, {"rename", filepath.Join(d.root, "x"), heads + "/hire"},
		{"symlink", heads + "/hire", filepath.Join(d.root, "x")}, {"mkdir", heads + "/topic"},
		{"rename", filepath.Join(d.root, "y"), heads + "/topic"}}, times(5, "PermissionError"))
	if got := owner(d.checkout, "rev-parse", "hire/h2/work"); got != h2 {
		t.Errorf("hire/h2/work moved: %s, was %s", got, h2)
	}
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "stopped"); r.status == 0 {
		t.Errorf("the desk's commit made refs/heads/hire again: %+v", r)
	}
	d.prepare(t) // the desk's next start
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the next start"); r.status != 0 {
		t.Errorf("the desk's commit after its next start: %+v", r)
	}
}

// removeEmpty removes folder as the owner might by hand once git left it empty but for its keeper: the keeper and any
// empty folder below it, then the folder. A file anywhere below fails the test (a loose ref left).
func removeEmpty(t *testing.T, folder string) {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(folder, func(at string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() {
			return errors.New("a file is left: " + at)
		}
		dirs = append(dirs, at)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return
	} else if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(dirs)
	for _, dir := range dirs {
		if err := os.Remove(dir); err != nil {
			t.Fatal(err)
		}
	}
}

// O1: in a repo with no origin/HEAD, which no desk may make, a desk's fetch from origin says nothing of it (git 2.48
// and later make a missing one by default), and makes none.
func TestOnAMacADesksFetchSaysNothingOfAMissingOriginHEAD(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	owner(d.checkout, "symbolic-ref", "-d", "refs/remotes/origin/HEAD")
	owner(d.checkout, "remote", "add", "origin", d.checkout) // a stand-in remote: the repo itself, on this disk
	r := d.deskGit(t, git, "fetch", "-q", "origin")
	if r.status != 0 || strings.Contains(r.stderr, "error") || strings.Contains(r.stderr, "origin/HEAD") {
		t.Errorf("the desk's fetch with no origin/HEAD: %+v", r)
	}
	if got := DefaultBranch(d.git); got != "" {
		t.Errorf("origin/HEAD was made: %q", got)
	}
}
