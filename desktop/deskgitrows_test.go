//go:build !windows

package desktop

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGITROWS2's B2, the core's side: a repo's default branch is pinned the first time the core reads it. A desk that
// then points origin/HEAD at main (as a profile from before the fix let it) changes neither the next start's Places
// nor the fire's base branches, which still name develop. The refresh is the owner's: with the repo's line taken out
// of the support folder's pins, the next start reads origin/HEAD again.
func TestTheCoreKeepsTheDefaultBranchItFirstRead(t *testing.T) {
	isolate(t)
	repo := t.TempDir()
	gitRepoCmd(t, repo, "init", "-q", "-b", "main")
	writeRepoFile(t, repo, "f", "first\n")
	gitRepoCmd(t, repo, "add", "f")
	gitRepoCmd(t, repo, "commit", "-q", "-m", "first")
	gitRepoCmd(t, repo, "branch", "develop")
	gitRepoCmd(t, repo, "update-ref", "refs/remotes/origin/develop", "develop")
	gitRepoCmd(t, repo, "update-ref", "refs/remotes/origin/main", "main")
	gitRepoCmd(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
	tree := filepath.Join(t.TempDir(), "w")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/x/w", tree)
	places := func() sandbox.Places {
		t.Helper()
		pl := sandbox.Places{Root: physical(tree)}
		if err := deskGitPlaces(deskAt{ID: "x", Root: tree, Checkout: repo, Git: runGit}, &pl); err != nil {
			t.Fatal(err)
		}
		return pl
	}
	bases := func() []string {
		t.Helper()
		refs, err := storeBaseRefs(storeReader(context.Background(), repo, nil))
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, r := range refs {
			names = append(names, r[1])
		}
		return names
	}
	if got := places().DefaultBranch; got != "develop" {
		t.Fatalf("the first start's default branch: %q, want develop", got)
	}
	gitRepoCmd(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	if got := places().DefaultBranch; got != "develop" {
		t.Errorf("the next start's default branch, once a desk pointed origin/HEAD at main: %q, want develop", got)
	}
	if got := bases(); !slices.Contains(got, "refs/heads/develop") {
		t.Errorf("the fire's base branches: %q, want develop among them", got)
	}
	if err := os.Remove(filepath.Join(supportDir, defaultBranchesName)); err != nil {
		t.Fatal(err)
	}
	if got := places().DefaultBranch; got != "main" {
		t.Errorf("after the owner took the pin out: %q, want main, as origin/HEAD names it now", got)
	}
}

// F1's Places, from the start: the branch the owner's checkout is on, and the desk's own besides hire/<its id> (the
// folder hireBranch names its branches under, and a branch from before that, hire/<its worktree's name>). A desk at
// the project has neither the checkout's branch nor a worktree's name.
func TestADesksPlacesNameTheOwnersBranchAndItsOwn(t *testing.T) {
	isolate(t)
	repo := t.TempDir()
	gitRepoCmd(t, repo, "init", "-q", "-b", "main")
	gitRepoCmd(t, repo, "commit", "-q", "--allow-empty", "-m", "first")
	tree := filepath.Join(t.TempDir(), "ada-abcdef")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/abcdef01/ada-abcdef", tree)
	gitRepoCmd(t, repo, "switch", "-q", "-c", "feature")
	pl := sandbox.Places{Root: physical(tree)}
	if err := deskGitPlaces(deskAt{ID: "ABCDEF01", Root: tree, Checkout: repo, Git: runGit}, &pl); err != nil {
		t.Fatal(err)
	}
	if want := []string{"hire/abcdef01", "hire/ada-abcdef"}; pl.CheckoutBranch != "feature" || !slices.Equal(pl.OwnBranches, want) {
		t.Errorf("in its worktree: %q %q, want feature %q", pl.CheckoutBranch, pl.OwnBranches, want)
	}
	pl = sandbox.Places{Root: physical(repo)}
	if err := deskGitPlaces(deskAt{ID: "ABCDEF01", Root: repo, Git: runGit}, &pl); err != nil {
		t.Fatal(err)
	}
	if pl.CheckoutBranch != "" || !slices.Equal(pl.OwnBranches, []string{"hire/abcdef01"}) {
		t.Errorf("at the project: %q %q", pl.CheckoutBranch, pl.OwnBranches)
	}
	// FXGITROWS2R4: the owner's HEAD a link, as a clone made with core.preferSymlinkRefs has it, names the branch too.
	head := filepath.Join(repo, ".git", "HEAD")
	if err := os.Remove(head); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("refs/heads/feature", head); err != nil {
		t.Fatal(err)
	}
	pl = sandbox.Places{Root: physical(tree)}
	if err := deskGitPlaces(deskAt{ID: "ABCDEF01", Root: tree, Checkout: repo, Git: runGit}, &pl); err != nil {
		t.Fatal(err)
	}
	if pl.CheckoutBranch != "feature" {
		t.Errorf("with the owner's HEAD a link: %q, want feature", pl.CheckoutBranch)
	}
}

// FXGITROWS2R1, N1: the owner's own change of the default branch is followed. Once the owner runs `git remote
// set-head origin trunk`, the next start's Policy holds trunk, and still develop, the pinned one; the fire's look takes
// both as base branches. On a Mac no desk can point origin/HEAD anywhere (FXGITROWS2's B2 row).
func TestTheCoreHoldsThePinnedDefaultBranchAndTheOneTheOwnerSetsNow(t *testing.T) {
	isolate(t)
	repo := t.TempDir()
	gitRepoCmd(t, repo, "init", "-q", "-b", "main")
	writeRepoFile(t, repo, "f", "first\n")
	gitRepoCmd(t, repo, "add", "f")
	gitRepoCmd(t, repo, "commit", "-q", "-m", "first")
	gitRepoCmd(t, repo, "remote", "add", "origin", repo) // a stand-in remote: its address is never reached here
	for _, branch := range []string{"develop", "trunk"} {
		gitRepoCmd(t, repo, "branch", branch)
		gitRepoCmd(t, repo, "update-ref", "refs/remotes/origin/"+branch, branch)
	}
	gitRepoCmd(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
	tree := filepath.Join(t.TempDir(), "w")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/x/w", tree)
	start := func() (*sandbox.Policy, string) {
		t.Helper()
		pl := sandbox.Places{GOOS: "darwin", Home: physical(homeDir), Support: physical(supportDir), OwnID: "x", Root: physical(tree)}
		if err := deskGitPlaces(deskAt{ID: "x", Root: tree, Checkout: repo, Git: runGit}, &pl); err != nil {
			t.Fatal(err)
		}
		p, err := sandbox.New(pl)
		if err != nil {
			t.Fatal(err)
		}
		return p, pl.GitDir
	}
	if p, git := start(); p == nil || !deniedWrite(p, filepath.Join(git, "refs", "heads", "develop")) {
		t.Fatal("the first start doesn't hold develop")
	}
	gitRepoCmd(t, repo, "remote", "set-head", "origin", "trunk")
	p, git := start()
	for _, branch := range []string{"trunk", "develop"} {
		if !deniedWrite(p, filepath.Join(git, "refs", "heads", branch)) {
			t.Errorf("the next start, once the owner set origin/HEAD to trunk, lets the desk move %s", branch)
		}
	}
	refs, err := storeBaseRefs(storeReader(context.Background(), repo, nil))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range refs {
		names = append(names, r[1])
	}
	for _, want := range []string{"refs/heads/trunk", "refs/heads/develop"} {
		if !slices.Contains(names, want) {
			t.Errorf("the fire's base branches: %q, want %s among them", names, want)
		}
	}
}

func deniedWrite(p *sandbox.Policy, at string) bool {
	v, _ := p.Decide(at, sandbox.Write)
	return v == sandbox.Deny
}

// FXGITROWS2R3 (RT126FIXREVIEW-GITROWS2R2's O3): refPlaceRefusal's words say where the link is, and ask the owner to
// check what it leads to before putting it in the link's place; for a hard link, how to give it a copy of its own.
// FXGITROWS2R4: for a symbolic ref, a link or a file, where it points, and the fix: a ref of its own at a commit the
// owner checked, or none; origin/HEAD asked of origin again. Never `git symbolic-ref` with the same text.
func TestARefusedRefPlaceSaysWhereItIsAndWhatToDo(t *testing.T) {
	for _, c := range []struct {
		name     string
		e        sandbox.RefPlaceError
		say, not []string
	}{
		{"a link in refs/", sandbox.RefPlaceError{At: "/r/.git/refs/heads/topic", Ref: "refs/heads/topic", Link: true},
			[]string{"/r/.git/refs/heads/topic is a link, where git keeps this repository's branches", "ls -l '/r/.git/refs/heads/topic'", "check what's in it", "put it in the link's place", "the desk didn't start"},
			[]string{"preferSymlinkRefs"}},
		{"a link beside the .git", sandbox.RefPlaceError{At: "/r/.git/team", Ref: "team", Link: true},
			[]string{"/r/.git/team is a link, where git looks for a branch's name before its own branches", "check what's in it"},
			[]string{"keeps this repository's branches"}},
		// FXGITROWS2R4 (RT126FIXREVIEW-GITROWS2R3's B1): never the same text written back, which a desk may have chosen.
		{"origin/HEAD, a symbolic ref git wrote as a link", sandbox.RefPlaceError{At: "/r/.git/refs/remotes/origin/HEAD", Ref: "refs/remotes/origin/HEAD", Link: true, Symref: "refs/remotes/origin/main"},
			[]string{"which git reads as a symbolic ref to refs/remotes/origin/main", "core.preferSymlinkRefs", "git remote set-head origin --auto", "the desk didn't start"},
			[]string{"symbolic-ref", "update-ref"}},
		{"a branch, a symbolic ref git wrote as a link", sandbox.RefPlaceError{At: "/r/.git/refs/heads/topic/y", Ref: "refs/heads/topic/y", Link: true, Symref: "refs/heads/hire/h1/work"},
			[]string{"which git reads as a symbolic ref to refs/heads/hire/h1/work", "core.preferSymlinkRefs", "git log refs/heads/hire/h1/work",
				"git update-ref --no-deref refs/heads/topic/y <commit>", "git update-ref -d --no-deref refs/heads/topic/y", "the desk didn't start"},
			[]string{"symbolic-ref", "set-head"}},
		{"a branch, a symbolic ref", sandbox.RefPlaceError{At: "/r/.git/refs/heads/topic/y", Ref: "refs/heads/topic/y", Symref: "refs/heads/hire/h1/work", Moves: "refs/heads/hire/h1/work"},
			[]string{"topic/y isn't a ref of its own: it points at refs/heads/hire/h1/work, which a desk can move", "would move refs/heads/topic/y too",
				"git log refs/heads/hire/h1/work", "git update-ref --no-deref refs/heads/topic/y <commit>", "the desk didn't start"},
			[]string{"symbolic-ref", "is a link", "preferSymlinkRefs"}},
		{"a branch, on through another", sandbox.RefPlaceError{At: "/r/.git/refs/heads/topic/y", Ref: "refs/heads/topic/y", Symref: "refs/heads/alias", Moves: "refs/remotes/origin/x"},
			[]string{"it points at refs/heads/alias, and on to refs/remotes/origin/x, which a desk can move"},
			[]string{"symbolic-ref"}},
		{"a branch, a way that doesn't end", sandbox.RefPlaceError{At: "/r/.git/refs/heads/a", Ref: "refs/heads/a", Symref: "refs/heads/b"},
			[]string{"a isn't a ref of its own: it points at refs/heads/b, which Smolcorp can't follow to a ref no desk can move", "could move refs/heads/a too"},
			[]string{"symbolic-ref", "would move"}},
		{"the owner checkout's HEAD", sandbox.RefPlaceError{At: "/r/.git/HEAD", Ref: "HEAD", Symref: "refs/remotes/origin/x", Moves: "refs/remotes/origin/x"},
			[]string{"Your checkout's HEAD (/r/.git/HEAD) points at refs/remotes/origin/x, which a desk can move", "switch your checkout to a branch of your own", "the desk didn't start"},
			[]string{"symbolic-ref", "update-ref"}},
		// FXGITROWS2R5 (RT126FIXREVIEW-GITROWS2R4's O3 and N2): a tag isn't a branch; another worktree's HEAD; a
		// registration that is a link.
		{"a tag, a symbolic ref", sandbox.RefPlaceError{At: "/r/.git/refs/tags/v1", Ref: "refs/tags/v1", Symref: "refs/heads/hire/h1/work", Moves: "refs/heads/hire/h1/work"},
			[]string{"refs/tags/v1 isn't a ref of its own: it points at refs/heads/hire/h1/work, which a desk can move", "git update-ref --no-deref refs/tags/v1 <commit>"},
			[]string{"a branch of its own"}},
		{"another worktree's HEAD", sandbox.RefPlaceError{At: "/r/.git/worktrees/review/HEAD", Ref: "worktrees/review/HEAD", Symref: "refs/heads/hire/h1/work", Moves: "refs/heads/hire/h1/work"},
			[]string{"The HEAD of the worktree git knows as review (/r/.git/worktrees/review/HEAD) points at refs/heads/hire/h1/work, which a desk can move",
				"would move that worktree too", "git worktree list", "git log refs/heads/hire/h1/work", "switch that worktree to a branch of its own", "the desk didn't start"},
			[]string{"symbolic-ref", "update-ref", "Your checkout"}},
		{"another worktree's HEAD, a way that doesn't end", sandbox.RefPlaceError{At: "/r/.git/worktrees/review/HEAD", Ref: "worktrees/review/HEAD", Symref: "refs/heads/b"},
			[]string{"which Smolcorp can't follow to a ref no desk can move, so a desk's work there could move that worktree too"},
			[]string{"would move"}},
		{"a worktree's registration, a link", sandbox.RefPlaceError{At: "/r/.git/worktrees/review", Ref: "worktrees/review", Link: true},
			[]string{"/r/.git/worktrees/review is a link, where git keeps what it knows of a worktree", "ls -l '/r/.git/worktrees/review'", "git worktree prune", "the desk didn't start"},
			[]string{"branches and their history", "before its own branches"}},
		{"a hard link", sandbox.RefPlaceError{At: "/r/.git/refs/heads/rev/x", Ref: "refs/heads/rev/x", Hard: true},
			[]string{"another name elsewhere (a hard link)", "a copy of its own", "the desk didn't start"},
			[]string{"is a link"}},
		{"a pipe", sandbox.RefPlaceError{At: "/r/.git/refs/tags/v1", Ref: "refs/tags/v1"},
			[]string{"isn't a folder or a file", "Remove it"},
			nil},
	} {
		words := refPlaceRefusal(&c.e)
		for _, s := range c.say {
			if !strings.Contains(words, s) {
				t.Errorf("%s: %q doesn't say %q", c.name, words, s)
			}
		}
		for _, s := range c.not {
			if strings.Contains(words, s) {
				t.Errorf("%s: %q says %q", c.name, words, s)
			}
		}
	}
}
