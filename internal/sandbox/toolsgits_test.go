package sandbox

import (
	"strings"
	"testing"
)

// FXHIREPROJECTSR7 (RT126FIXREVIEW-HIREPROJECTSR6 N1): the static .git rule below a desk's place leaves out the folders
// package managers keep their own git clones and mirrors in (ToolsGits), so a hire's own `swift build`, `bundle
// install` or `pip install -e git+…` can make them. The rule stays final: each is a `require-not` in its own filter.
// As data, on the made-up Mac of policy_test.go; desktop's hireprojectsr7_darwin_test.go runs it under the real profile.

// toolsGitsCases are what a desk at root writes in a package manager's own folders, at any depth and in any case, and
// what the rule still holds beside them.
func toolsGitsCases(root string) []projectsCase {
	return []projectsCase{
		{root + "/.build/checkouts/dep/.git/config", Write, Allow, ""},
		{root + "/.build/checkouts/dep/.git", Write, Allow, ""},
		{root + "/.build/checkouts/dep/.git", Move, Allow, ""},
		{root + "/.build/repositories/dep-1a2b3c4d/HEAD", Write, Allow, ""},
		{root + "/Packages/Kit/.build/checkouts/dep/.git/hooks/x", Write, Allow, ""},
		{root + "/.BUILD/Checkouts/dep/.GIT/config", Write, Allow, ""},
		{root + "/vendor/bundle/ruby/3.3.0/cache/bundler/git/gem-0a1b/config", Write, Allow, ""},
		{root + "/vendor/bundle/ruby/3.3.0/cache/bundler/git/gem-0a1b/.git/config", Write, Allow, ""},
		{root + "/.venv/src/pkg/.git/config", Write, Allow, ""},
		{root + "/venv/src/pkg/.git/config", Write, Allow, ""},
		// Held still: the same names elsewhere, or only one of them.
		{root + "/.build/other/dep/.git/config", Write, Deny, rowGitBelow},
		{root + "/build/checkouts/dep/.git/config", Write, Deny, rowGitBelow},
		{root + "/checkouts/dep/.git/config", Write, Deny, rowGitBelow},
		{root + "/src/pkg/.git/config", Write, Deny, rowGitBelow},
		{root + "/env/src/pkg/.git/config", Write, Deny, rowGitBelow},
		{root + "/vendor/bundle/cache/gem/.git/config", Write, Deny, rowGitBelow},
		{root + "/lib/vendor/x/.git/hooks/post-checkout", Write, Deny, rowGitBelow},
	}
}

func TestAPackageManagersOwnClonesAreLeftOutOfTheGitRule(t *testing.T) {
	worktree := hirePlaces(realSupport)
	checkout := hirePlaces(realSupport)
	checkout.Root, checkout.Checkout, checkout.OwnWorktree = repo, "", ""
	for name, pl := range map[string]Places{"a hire in its worktree": worktree, "a desk at its checkout": checkout} {
		p := mustNew(t, pl)
		decideEach(t, name, p, toolsGitsCases(pl.Root))
		for _, r := range p.Rules() {
			if strings.HasPrefix(r.Row, rowGitBelow) && r.Wide {
				t.Errorf("%s: an exception could open %+v", name, r)
			}
		}
	}
	// The folders the walk skips are those the rule leaves out (desktop's gitBelow asks InToolsGits).
	root := worktree.Root
	for at, want := range map[string]bool{
		root + "/.build/checkouts": true, root + "/.build/checkouts/dep": true, root + "/a/.build/repositories/x": true,
		root + "/vendor/bundle/ruby/3.3.0/cache/bundler/git": true, root + "/.venv/src/pkg": true,
		root + "/.build": false, root + "/src": false, root + "/vendor/bundle": false, root: false, home + "/.build/checkouts": false,
	} {
		if got := InToolsGits("darwin", root, at); got != want {
			t.Errorf("InToolsGits %s: %v, want %v", at, got, want)
		}
	}
}

// On the Mac's profile the rule is one final deny whose filter leaves out each folder by a `require-not` regex.
func TestTheGitRulesProfileLineLeavesOutThePackageManagersFolders(t *testing.T) {
	text, err := mustNew(t, hirePlaces(realSupport)).Profile()
	if err != nil {
		t.Fatal(err)
	}
	var line string
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, `\\.git(/.*)?$"`) && strings.HasSuffix(l, sbComment(rowGitBelow)) {
			line = l
		}
	}
	if !strings.HasPrefix(line, "(deny file-write* file-link (require-all (regex ") || strings.Count(line, "(require-not (regex ") != len(ToolsGits) ||
		!strings.Contains(line, `\\.build/checkouts(/.*)?$"`) || !strings.Contains(line, `/vendor/bundle(/[^/]+)*/cache/bundler/git(/.*)?$"`) {
		t.Errorf("the rule's line: %s", line)
	}
}
