package sandbox

import (
	"errors"
	"strings"
	"testing"
)

// FXHIREPROJECTSR6 (RT126FIXREVIEW-HIREPROJECTSR5 B1 and N2): on a Mac, no desk writes, links or renames
// anything in a .git strictly below its place, nor makes or replaces one there; its own .git, at its place, keeps its
// rows. A place that isn't its repo's top makes no .git and no HEAD at its place either. The git folders found below
// it at its start (GitBelow) have the folders on the way held by name. As data, on the made-up Mac of policy_test.go;
// desktop's hireprojectsr6_darwin_test.go runs it under the real profile.

// gitBelowCases are what no desk at root writes below it: in a nested clone's .git (a folder, in any case), a
// submodule's or worktree's .git file, a .git made or renamed in, one at any depth; and what stays its work: .github,
// .gitignore, .gitmodules, a name that only ends in .git, a nested clone's own files.
func gitBelowCases(root string) []projectsCase {
	return []projectsCase{
		{root + "/lib/vendor/x/.git/hooks/post-checkout", Write, Deny, rowGitBelow},
		{root + "/lib/vendor/x/.git/config", Write, Deny, rowGitBelow},
		{root + "/lib/vendor/x/.git", Write, Deny, rowGitBelow},
		{root + "/lib/vendor/x/.git", Move, Deny, rowGitBelow},
		{root + "/lib/vendor/x/.git", Link, Deny, rowGitBelow},
		{root + "/lib/vendor/x/.git/config", Link, Deny, rowGitBelow},
		{root + "/lib/vendor/x/.git/config", Read, Allow, ""},
		{root + "/clients/acme/.GIT/hooks/pre-commit", Write, Deny, rowGitBelow},
		{root + "/a/b/c/d/.git", Write, Deny, rowGitBelow},
		{root + "/sub/.git", Write, Deny, rowGitBelow},
		{root + "/lib/vendor/x/README", Write, Allow, ""},
		{root + "/lib/vendor/x", Move, Allow, ""},
		{root + "/.github/workflows/ci.yml", Write, Allow, ""},
		{root + "/.gitignore", Write, Allow, ""},
		{root + "/sub/.gitignore", Write, Allow, ""},
		{root + "/.gitmodules", Write, Allow, ""},
		{root + "/mirror.git/config", Write, Allow, ""},
		{root + "/sub/.git.bak/config", Write, Allow, ""},
		{root + "/sub/HEAD", Write, Allow, ""},
		{root + "/build", Move, Allow, ""},
		{root + "/src/a.go", Move, Allow, ""},
	}
}

// A hire in its own worktree: every .git below it is another's; its own .git file keeps its row.
func TestADeskWritesNoGitFolderBelowItsWorktree(t *testing.T) {
	pl := hirePlaces(realSupport)
	cases := append(gitBelowCases(pl.Root),
		projectsCase{pl.Root + "/.git", Write, Deny, "the shared git places: the .git at its place"},
		projectsCase{gitDir + "/objects/ab/cd", Write, Allow, ""}, projectsCase{gitDir + "/refs/heads/hire/h1", Write, Allow, ""})
	decideEach(t, "a hire in its worktree", mustNew(t, pl), cases)
}

// A desk at the project, its shared .git at its place: that .git is its own, as before; one below it isn't.
func TestADeskAtItsCheckoutKeepsItsOwnGitButNoneBelow(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.Root, pl.Checkout, pl.OwnWorktree = repo, "", ""
	cases := append(gitBelowCases(pl.Root),
		projectsCase{gitDir + "/objects/ab/cd", Write, Allow, ""}, projectsCase{gitDir + "/config", Write, Deny, "the shared git places"}, projectsCase{repo + "/HEAD", Write, Allow, ""})
	decideEach(t, "a desk at its checkout", mustNew(t, pl), cases)
}

// A place in no repo (notes), and one inside a repo that isn't its top (a folder of a checkout): no .git below, and
// none made at the place itself, nor a HEAD there, which git would take for a bare repo's.
func TestADeskWhosePlaceIsntItsReposTopMakesNoGitThere(t *testing.T) {
	notes := home + "/Projects/notes"
	plain := hirePlaces(realSupport)
	plain.Root, plain.Checkout, plain.GitDir, plain.OwnWorktree, plain.OtherWorktrees, plain.DefaultBranch = notes, "", "", "", nil, ""
	inside := hirePlaces(realSupport)
	inside.Root, inside.Checkout, inside.OwnWorktree = repo+"/docs", "", ""
	for name, pl := range map[string]Places{"a place in no repo": plain, "a folder inside a checkout": inside} {
		cases := append(gitBelowCases(pl.Root),
			projectsCase{pl.Root + "/.git", Write, Deny, ""}, projectsCase{pl.Root + "/.git/config", Write, Deny, rowGitBelow},
			projectsCase{pl.Root + "/.Git/hooks/x", Write, Deny, rowGitBelow}, projectsCase{pl.Root + "/HEAD", Write, Deny, rowGitBelow},
			projectsCase{pl.Root + "/head", Write, Deny, rowGitBelow}, projectsCase{pl.Root + "/todo.md", Write, Allow, "its own place"})
		p := mustNew(t, pl)
		decideEach(t, name, p, cases)
		for _, folder := range p.Hold() {
			if strings.HasSuffix(folder, "/.git") || strings.HasSuffix(folder, "/HEAD") {
				t.Errorf("%s: Hold asks for %s", name, folder)
			}
		}
	}
}

// The git folders found below the place at its start: each folder on the way to one is held by name, so none is moved
// out of the place, written, and moved back; their other folders and files stay its work.
func TestTheFoldersOnTheWayToAGitFolderBelowArentMoved(t *testing.T) {
	pl := hirePlaces(realSupport)
	nested := pl.Root + "/lib/vendor/x/.git"
	pl.GitBelow = []string{nested, pl.Root + "/clients/acme/.git"}
	held := rowGitBelow + ": the folders on the way to one"
	cases := append(gitBelowCases(pl.Root)[:10],
		projectsCase{pl.Root + "/lib", Move, Deny, ""}, projectsCase{pl.Root + "/lib/vendor", Move, Deny, held},
		projectsCase{pl.Root + "/lib/vendor/x", Move, Deny, held}, projectsCase{pl.Root + "/lib/vendor/x", Link, Deny, held},
		projectsCase{pl.Root + "/clients", Move, Deny, held}, projectsCase{pl.Root + "/clients/acme", Move, Deny, held},
		projectsCase{pl.Root + "/lib/vendor/x/README", Write, Allow, ""}, projectsCase{pl.Root + "/lib/vendor/x/src", Move, Allow, ""},
		projectsCase{pl.Root + "/lib/other", Move, Allow, ""}, projectsCase{pl.Root + "/clients/beta", Move, Allow, ""},
		projectsCase{pl.Root + "/lib/vendor/y.txt", Write, Allow, ""})
	decideEach(t, "a hire with git folders below it", mustNew(t, pl), cases)
	// Only a .git strictly below the place is taken.
	for _, git := range []string{pl.Root + "/.git", pl.Root + "/lib/vendor/x", home + "/Projects/lib/.git", pl.Root + "/x/.github", "lib/.git"} {
		bad := pl
		bad.GitBelow = []string{git}
		if _, err := New(bad); !errors.Is(err, ErrGitBelow) {
			t.Errorf("GitBelow %s: %v", git, err)
		}
	}
}

// The rule's pattern reaches any depth, so it holds no folder by pattern (denyAbove): every Move-only row there is
// for a folder by name. And it is a Mac's: Linux's view isn't changed here.
func TestTheGitBelowRowHoldsNoFolderByPatternAndIsAMacs(t *testing.T) {
	pl := hirePlaces(realSupport)
	for _, r := range mustNew(t, pl).Rules() {
		if r.Row == rowAbove && strings.HasPrefix(r.Path, pl.Root) && r.Tail != "" {
			t.Errorf("a folder held by pattern below the place: %+v", r)
		}
		if strings.HasPrefix(r.Row, rowGitBelow) && r.Wide {
			t.Errorf("an exception could open %+v", r)
		}
	}
	linux := hirePlaces(realSupport)
	linux.GOOS = "linux"
	p, err := New(linux)
	if err != nil {
		t.Fatalf("linux: %v", err)
	}
	for _, r := range p.Rules() {
		if strings.HasPrefix(r.Row, rowGitBelow) {
			t.Errorf("linux: %+v", r)
		}
	}
}
