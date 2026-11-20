//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

// RT126SWEEP4 part 3's F2: the fire removes a worktree's registration through the repo's worktrees folder as its scan
// found it, held open from the scan, never by its path again. Whoever can write the repo's .git turns that folder into
// a link to another folder (victim) holding a folder of the registration's name, between the tree's removal and its
// registration's (deskTreeGone, the test's seam for that moment): the victim's folder is untouched, and the
// registration itself, in the folder as the scan found it, is gone. Stand-ins only, all in the test's temp folders.
func TestTheFiresRegistrationRemovalNeverFollowsALinkPutOnTheWorktreesFolder(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	tree := filepath.Join(physical(t.TempDir()), "w-reg")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-reg", tree)
	worktrees := filepath.Join(repo, ".git", "worktrees")
	victim := filepath.Join(physical(t.TempDir()), "victim")
	writeFile(t, filepath.Join(victim, "w-reg", "keep.txt"), "the victim's own\n")
	saved := deskTreeGone
	t.Cleanup(func() { deskTreeGone = saved })
	swapped := false
	deskTreeGone = func() {
		if err := os.Rename(worktrees, worktrees+"-aside"); err != nil {
			t.Errorf("moving the worktrees folder aside: %v", err)
			return
		}
		if err := os.Symlink(victim, worktrees); err != nil {
			t.Errorf("the link: %v", err)
			return
		}
		swapped = true
	}
	status, words, ok := removeDeskTree(Hire{Name: "the row's worktree", Repo: repo, Worktree: keptPath(tree)})
	t.Logf("removeDeskTree: %d %q %v (swapped %v)", status, words, ok, swapped)
	if !swapped {
		t.Fatal("the seam never ran: the tree wasn't removed")
	}
	if _, err := os.Stat(filepath.Join(victim, "w-reg", "keep.txt")); err != nil {
		t.Errorf("the victim's folder of the registration's name: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(worktrees+"-aside", "w-reg")); !os.IsNotExist(err) {
		t.Errorf("the registration, in the folder as the scan found it, is still there: %v", err)
	}
	if _, err := os.Lstat(tree); !os.IsNotExist(err) {
		t.Errorf("the tree is still there: %v", err)
	}
}
