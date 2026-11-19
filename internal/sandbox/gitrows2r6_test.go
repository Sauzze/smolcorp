package sandbox

import "testing"

// FXGITROWS2R6 (RT126FIXREVIEW-GITROWS2R5's N2) in the Policy: the rest of the shared .git is an allowlist on a Mac
// (rowGitTop). A desk in its own worktree writes there only the folders other rows hold (objects, refs and logs/refs,
// worktrees), packed-refs.lock and git-lfs's store; a desk at the project its fetch's FETCH_HEAD too. Every other name
// is held where no narrower row names it: one the owner's git reads before a branch, a pseudo-ref, gc.pid, a new folder.
// Linux's view has no such row. Its real-profile rows are in gitrows2r6_darwin_test.go.
func TestOnAMacThePolicyHoldsTheRestOfTheSharedGitButWhatADesksWorkNeeds(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.CheckoutBranch = "topic/x"
	p := mustNew(t, pl)
	for _, c := range []struct {
		at  string
		v   Verdict
		row string
	}{
		{"feature", Deny, rowGitTop}, {"feature.lock", Deny, rowGitTop}, {"hire/h2/work", Deny, rowGitTop}, {"hire", Deny, rowGitTop},
		{"FEATURE_HEAD", Deny, rowGitTop}, {"FETCH_HEAD", Deny, rowGitTop}, {"gc.pid", Deny, rowGitTop}, {"gc.pid.lock", Deny, rowGitTop},
		{"gc.log", Deny, rowGitTop}, {"description", Deny, rowGitTop}, {"COMMIT_EDITMSG", Deny, rowGitTop}, {"new/x", Deny, rowGitTop},
		{"logs/feature", Deny, rowGitTop}, {"logs/new/x", Deny, rowGitTop}, {"Feature", Deny, rowGitTop}, {"LFS-moved/x", Deny, rowGitTop},
		{"packed-refs.lock.x", Deny, rowGitTop},
		// …where a narrower row names it, that row.
		{"config", Deny, "the shared git places"}, {"ORIG_HEAD", Deny, rowOwnersState}, {"logs/HEAD", Deny, rowOwnersState},
		{"packed-refs", Deny, "the base branches"}, {"packed-refs.new", Deny, "the base branches"}, {"main", Deny, rowBaseNames},
		{"objects/info/alternates", Deny, "the shared git places"}, {"refs/feature", Deny, rowNotOwnRefs},
		{"worktrees/later/HEAD", Deny, rowOtherWorktrees},
		// The desk's own.
		{"packed-refs.lock", Allow, ""}, {"lfs", Allow, ""}, {"lfs/objects/ab/cd/abcd", Allow, ""}, {"lfs/tmp/1", Allow, ""},
		{"objects/ab/cdef", Allow, ""}, {"objects/pack/pack-a.pack", Allow, ""}, {"refs/heads/hire/h1/work", Allow, ""},
		{"logs/refs/heads/hire/h1/work", Allow, ""}, {"refs/remotes/origin/main", Allow, ""}, {"worktrees/h1/FETCH_HEAD", Allow, ""},
		{"worktrees/h1/ORIG_HEAD", Allow, ""}, {"worktrees/h1/gc.log", Allow, ""},
	} {
		if v, by := p.Decide(gitDir+"/"+c.at, Write); v != c.v || c.row != "" && by.Row != c.row {
			t.Errorf("%s: Write %v by %q, want %v by %q", c.at, v, by.Row, c.v, c.row)
		}
	}
	// A desk at the project writes its fetch's FETCH_HEAD in the shared .git, and keeps the checkout's merge, pick and
	// rebase state, its own (FXBASEHEADS' ruling #3); nothing else there.
	pl.Root, pl.Checkout, pl.OwnWorktree, pl.CheckoutBranch = repo, "", "", ""
	at := mustNew(t, pl)
	for _, c := range []struct {
		at  string
		v   Verdict
		row string
	}{
		{"FETCH_HEAD", Allow, ""}, {"FETCH_HEAD.lock", Allow, ""}, {"ORIG_HEAD", Allow, ""}, {"MERGE_HEAD.lock", Allow, ""},
		{"logs/HEAD", Allow, ""}, {"AUTO_MERGE", Allow, ""}, {"packed-refs.lock", Allow, ""}, {"lfs/objects/x", Allow, ""},
		{"feature", Deny, rowGitTop}, {"gc.pid", Deny, rowGitTop}, {"FEATURE_HEAD", Deny, rowGitTop}, {"MERGE_MSG", Deny, rowGitTop},
		{"logs/feature", Deny, rowGitTop}, {"HEAD", Deny, "the shared git places"},
	} {
		if v, by := at.Decide(gitDir+"/"+c.at, Write); v != c.v || c.row != "" && by.Row != c.row {
			t.Errorf("a desk at the project: %s: Write %v by %q, want %v by %q", c.at, v, by.Row, c.v, c.row)
		}
	}
	// The fold beside a base branch's name there (the row TestEveryFullyFoldedSpellingOfAPathIsTheSameRule had as an
	// allow): ﬆable is stable's own row, ıstable and stablé aren't that name, and are held by rowGitTop alone.
	fold := mustNew(t, Places{GOOS: "darwin", Home: home, Support: realSupport, OwnID: "h1", Root: hires + "/h1", Checkout: repo,
		GitDir: gitDir, OwnWorktree: "h1", DefaultBranch: "stable"})
	for name, row := range map[string]string{"\ufb06able": rowBaseNames, "\u0131stable": rowGitTop, "stabl\u00e9": rowGitTop} {
		if v, by := fold.Decide(gitDir+"/"+name, Write); v != Deny || by.Row != row {
			t.Errorf("%+q: Write %v by %q, want it held by %q", name, v, by.Row, row)
		}
	}
	pl.GOOS = "linux"
	for _, r := range mustBuild(t, pl).Rules() {
		if r.Row == rowGitTop {
			t.Errorf("Linux's view has %q", r.Row)
		}
	}
}
