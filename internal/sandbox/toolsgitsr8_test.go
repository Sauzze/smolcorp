package sandbox

import (
	"errors"
	"testing"
)

// FXHIREPROJECTSR8 (RT126FIXREVIEW-HIREPROJECTSR7 N1): the clones and mirrors the core found in a package manager's
// own folders at a desk's start (ToolsGitBelow) are held, each whole, and the folders on the way to each by name; one
// made during the session isn't, nor is the rest of those folders. As data, on the made-up Mac of policy_test.go;
// desktop's hireprojectsr8_darwin_test.go runs it under the real profile.
func TestAPackageManagersClonesThereAtTheStartAreHeld(t *testing.T) {
	worktree := hirePlaces(realSupport)
	checkout := hirePlaces(realSupport)
	checkout.Root, checkout.Checkout, checkout.OwnWorktree = repo, "", ""
	for name, pl := range map[string]Places{"a hire in its worktree": worktree, "a desk at its checkout": checkout} {
		root := pl.Root
		pl.ToolsGitBelow = []string{root + "/.build/checkouts/dep/.git", root + "/.build/repositories/dep-1a2b3c4d"}
		const held = rowGitBelow + ": a package manager's own, there at the start"
		const onTheWay = rowGitBelow + ": the folders on the way to one"
		decideEach(t, name, mustNew(t, pl), []projectsCase{
			{root + "/.build/checkouts/dep/.git/hooks/post-checkout", Write, Deny, held},
			{root + "/.build/checkouts/dep/.git/config", Write, Deny, held},
			{root + "/.build/checkouts/dep/.git", Move, Deny, rowAbove},
			{root + "/.BUILD/checkouts/DEP/.git/config", Write, Deny, held},
			{root + "/.build/repositories/dep-1a2b3c4d/hooks/reference-transaction", Write, Deny, held},
			{root + "/.build/repositories/dep-1a2b3c4d/config", Write, Deny, held},
			{root + "/.build/checkouts/dep", Move, Deny, onTheWay},
			{root + "/.build/checkouts", Move, Deny, onTheWay},
			{root + "/.build", Move, Deny, onTheWay},
			{root + "/.build/repositories", Link, Deny, onTheWay},
			// The desk's own: a dependency it fetches now, and the files of the owner's checkout.
			{root + "/.build/checkouts/new/.git/config", Write, Allow, ""},
			{root + "/.build/repositories/new-5e6f7a8b/config", Write, Allow, ""},
			{root + "/.build/checkouts/dep/README", Write, Allow, ""},
		})
	}
	// Only a git folder strictly below the place, in a package manager's folder, is taken.
	for _, git := range []string{worktree.Root + "/src/x/.git", worktree.Root, home + "/.build/checkouts/dep/.git"} {
		pl := worktree
		pl.ToolsGitBelow = []string{git}
		if _, err := New(pl); !errors.Is(err, ErrGitBelow) {
			t.Errorf("ToolsGitBelow %s: %v, want ErrGitBelow", git, err)
		}
	}
}
