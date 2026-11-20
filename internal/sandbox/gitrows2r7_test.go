package sandbox

import "testing"

// FXGITROWS2R7 (RT126FIXREVIEW-GITROWS2R6's N2) in the Policy: rowGitTop leaves the checkout's FETCH_HEAD and merge,
// pick and rebase state to a desk only when its place is the project's main checkout (no registration of its own in
// worktrees/). A Mac plain hire has no Checkout; in a linked worktree it is held from them as a hire there is. A desk
// the core put in a worktree is held from them even if its registration wasn't found. Its real-profile row is in
// gitrows2r7_darwin_test.go.
func TestOnAMacOnlyADeskAtTheMainCheckoutKeepsItsMergeState(t *testing.T) {
	plainInWorktree := hirePlaces(realSupport)
	plainInWorktree.Checkout = ""
	atProject := hirePlaces(realSupport)
	atProject.Root, atProject.Checkout, atProject.OwnWorktree = repo, "", ""
	plainAtProject := atProject
	plainAtProject.Root = repo + "/sub"
	noRegistration := hirePlaces(realSupport)
	noRegistration.OwnWorktree = ""
	for _, c := range []struct {
		name string
		pl   Places
		v    Verdict
	}{
		{"a plain hire in a linked worktree", plainInWorktree, Deny},
		{"a desk in its worktree, its registration not found", noRegistration, Deny},
		{"a desk at the project", atProject, Allow},
		{"a plain hire in a folder of the main checkout", plainAtProject, Allow},
	} {
		p := mustNew(t, c.pl)
		for _, name := range []string{"MERGE_HEAD", "MERGE_HEAD.lock", "ORIG_HEAD", "FETCH_HEAD", "logs/HEAD", "CHERRY_PICK_HEAD", "AUTO_MERGE"} {
			v, by := p.Decide(gitDir+"/"+name, Write)
			if v != c.v {
				t.Errorf("%s: %s: Write %v by %q, want %v", c.name, name, v, by.Row, c.v)
			}
		}
	}
}
