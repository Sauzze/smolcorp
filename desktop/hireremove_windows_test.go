//go:build windows

package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// On Windows (CI 44214629117 on windows-latest, and a tester's computer), the fire kept a clean hire's worktree and
// its branch, and logged nothing (FXWINWORKTREE): TestRemoveWorkspaceKeepsAWorktreeThatHoldsChanges,
// TestIfPreTrustRefusesTheWorktreeAndBranchAreRemoved and TestAHiresBranchIsInAFolderOfItsOwn. Every refusal on the
// way logs, so the removal stopped at one of its quiet answers: removeWorkspace's status said something (git's
// standard error is in that answer too), or removeDeskTree kept the tree with words of its own (status 1). These rows
// make a worktree with the real git, once as the owner's own git makes one and once as a hire's (prepareHire), then
// remove it as the fire does, and on a failure say what the core was told at each step.
func TestOnWindowsACleanWorktreeGitMadeIsRemovedWithItsBranch(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	path := filepath.Join(t.TempDir(), "w-win")
	branch := "hire/w-win"
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", "-b", branch, path).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	removedStepByStep(t, repo, path, branch, func() bool { return removeWorkspace(repo, path, branch, runGit) })
}

func TestOnWindowsAHiresCleanWorktreeIsRemovedWithItsBranch(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	o := placeOptions(t)
	place, err := prepareHire(hireRequest{HireID: "0123456789ab", Name: "Bo", Folder: repo, Worktree: true}, o)
	if err != nil {
		t.Fatal(err)
	}
	h := Hire{ID: "0123456789ab", Repo: place.Repo, Worktree: keptPath(place.Worktree), Branch: place.Branch}
	removedStepByStep(t, repo, place.Worktree, place.Branch, func() bool { return removeOwnWorktree(nil, o.Worktrees, h) })
}

// removedStepByStep is what the core is told about the clean worktree at path (on branch, of repo), before remove
// runs: logged, and an error for each answer that isn't a clean tree's. Then remove must take the tree, its
// registration and the branch; if it doesn't, the fire's removal of the tree is asked on its own, for its words.
func removedStepByStep(t *testing.T, repo, path, branch string, remove func() bool) {
	t.Helper()
	t.Logf("git: %q; the owner's line ends: %q", gitHere().version, ownerLineEndsNow())
	dotGit, err := os.ReadFile(filepath.Join(path, ".git"))
	t.Logf("the tree's .git file: %q (%v)", dotGit, err)
	registrations, _ := os.ReadDir(filepath.Join(repo, ".git", "worktrees"))
	for _, entry := range registrations {
		gitdir, err := os.ReadFile(filepath.Join(repo, ".git", "worktrees", entry.Name(), "gitdir"))
		t.Logf("registration %s: gitdir %q (%v)", entry.Name(), gitdir, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	tree, err := deskTreeOf(ctx, Hire{Name: "the row's worktree", Repo: physical(repo), Worktree: keptPath(path)}, true)
	t.Logf("deskTreeOf: %+v (%v); physical(path) %q", tree, err, physical(path))
	tree.close() // the worktrees folder it holds, let go before the removal below
	if err != nil {
		t.Errorf("the worktree's registration wasn't found: %v", err)
	}
	if status, out, ok := runGit(path, "symbolic-ref", "--short", "-q", "HEAD"); status != 0 || out != branch || !ok {
		t.Errorf("symbolic-ref: %d %q %v, want %q", status, out, ok, branch)
	}
	if status, out, ok := runGit(path, "status", "--porcelain", "--ignore-submodules=all"); status != 0 || out != "" || !ok {
		t.Errorf("the clean tree's status: %d %q %v, want 0 and nothing said", status, out, ok)
	}
	if !remove() {
		t.Errorf("the clean worktree wasn't removed")
		if _, err := os.Lstat(path); err == nil {
			status, out, ok := runGit(repo, "worktree", "remove", "--force", path)
			t.Errorf("the fire's removal of the tree on its own: %d %q %v", status, out, ok)
		}
	}
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("the worktree is still at %s", path)
	}
	if left, _ := os.ReadDir(filepath.Join(repo, ".git", "worktrees")); len(left) != 0 {
		t.Errorf("a registration is left: %v", left)
	}
	if got := branches(t, repo); strings.Contains(got, branch) {
		t.Errorf("the branch is left: %q", got)
	}
}

// ownerLineEndsNow is the owner's line ends as the core's git is handed them (gitOwnerLineEnds).
func ownerLineEndsNow() [][2]string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	owner, command := gitOwnerLineEnds(ctx)
	return append(owner, command...)
}
