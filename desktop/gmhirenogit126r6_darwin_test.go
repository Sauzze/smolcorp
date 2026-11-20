//go:build darwin

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGMHIRENOGIT126 round 6: the fire's own tree check (hiresOwnWorktree) resolved the row's tree again and ran git
// where it led. By the lead's ruling, on a Mac a tree that resolves to anything other than the path its hire stored gets
// no git and isn't removed, at the fire and at every other end: the hire ends, its tree is kept, and the words say why
// and what to do. Links, renames and the GM-made folder are the test's own, in its temp folders.

// r6Turn turns the folder for hires that holds tree into a link to a GM-made folder: a folder named as the tree, whose
// .git file names other's git folder. The real folder for hires is kept beside it; r6Turn gives the GM-made folder
// and a func that puts the real folder back.
func r6Turn(t *testing.T, tree, other string) (gm string, putBack func()) {
	t.Helper()
	hires := filepath.Dir(tree)
	gm = filepath.Join(physical(t.TempDir()), "gm-made")
	if err := os.MkdirAll(filepath.Join(gm, filepath.Base(tree)), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(gm, filepath.Base(tree), ".git"), "gitdir: "+filepath.Join(other, ".git")+"\n")
	if err := os.Rename(hires, hires+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(gm, hires); err != nil {
		t.Fatal(err)
	}
	return gm, func() {
		t.Helper()
		if err := os.Remove(hires); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(hires+".real", hires); err != nil {
			t.Fatal(err)
		}
	}
}

// r6Other is the other repo a GM-made folder's .git file names: r5Victim's, on the hire's own branch, so the tree
// check's branch look would pass there. It gives the repo and what its git folder holds now (r6Same compares).
func r6Other(t *testing.T, branch string) (string, map[string]string) {
	t.Helper()
	other := r5Victim(t)
	standInOwnersGit(t, other, "checkout", "-q", "-b", branch)
	return other, r6GitFiles(other)
}

// r6GitFiles is what other's index, HEAD and config hold.
func r6GitFiles(other string) map[string]string {
	files := map[string]string{}
	for _, name := range []string{"index", "HEAD", "config"} {
		files[name] = readOr(filepath.Join(other, ".git", name))
	}
	return files
}

// r6Untouched checks nothing ran or was written in the GM-made folder or the other repo: no git there, the GM-made
// folder holds only its .git file, the other repo is as r5Victim made it (r5Untouched) and its git files unchanged.
func r6Untouched(t *testing.T, what, gm, other string, before map[string]string, calls func(...string) []string) {
	t.Helper()
	if got := calls(gm, other); len(got) > 0 {
		t.Errorf("%s: git ran in the GM-made folder or the other repo: %q", what, got)
	}
	r5Untouched(t, what, other, calls)
	for name, was := range before {
		if now := readOr(filepath.Join(other, ".git", name)); now != was {
			t.Errorf("%s: the other repo's .git/%s changed", what, name)
		}
	}
	entries, err := os.ReadDir(gm)
	if err != nil || len(entries) != 1 {
		t.Errorf("%s: the GM-made folder holds %d entries (%v)", what, len(entries), err)
	}
	for _, e := range entries {
		inside, _ := os.ReadDir(filepath.Join(gm, e.Name()))
		if len(inside) != 1 || inside[0].Name() != ".git" {
			t.Errorf("%s: %s in the GM-made folder holds %d entries", what, e.Name(), len(inside))
		}
	}
}

// The fire: the folder for hires turned into a link to a GM-made folder (its tree's name there holding a .git file
// that names another repo, on the hire's branch) before the fire. The fire goes ahead and the hire ends; no git runs
// in the GM-made folder or the other repo, nothing is removed or written there, the real tree and its branch are
// kept, and the recap's line and the fire's words say it was moved or replaced and how to remove it. Control: an
// unchanged tree is taken as before.
func TestAFireRunsNoGitInATreeWhoseHiresFolderWasTurnedIntoALink(t *testing.T) {
	t.Run("turned", func(t *testing.T) {
		_, r, hire := hiredForFire(t, "")
		windowShows(r, pane(t, "idle-empty"), true)
		other, before := r6Other(t, hire.Branch)
		calls := gitCalls(t)
		gm, _ := r6Turn(t, hire.Worktree.storedOnly(), other)
		kept, err := fireEngineHire(hire, 0, r.o, false)
		words := "It was moved or replaced, so Smolcorp left it alone. To remove it yourself, delete what's at " + hire.Worktree.storedOnly() + " (only the link, if it's a link), then run git worktree prune in your own checkout."
		if err != nil || kept != words {
			t.Errorf("the fire: kept %q, err %v; want %q", kept, err, words)
		}
		r6Untouched(t, "the fire", gm, other, before, calls)
		real := filepath.Join(filepath.Dir(hire.Worktree.storedOnly())+".real", filepath.Base(hire.Worktree.storedOnly()))
		if _, err := os.Stat(filepath.Join(real, "sub", "a.txt")); err != nil {
			t.Errorf("the real tree was touched: %v", err)
		}
		if info, err := os.Lstat(filepath.Dir(hire.Worktree.storedOnly())); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("the link was taken away: %v", err)
		}
		if branches(t, r.repo) == "" {
			t.Error("the hire's branch was deleted")
		}
		got := registered(t)[0]
		if got.State != "fired" || !strings.Contains(got.Notes, "Its worktree "+hire.Worktree.storedOnly()+" was kept: it was moved or replaced, so Smolcorp left it alone.") {
			t.Errorf("its row: %+v", got)
		}
		data, err := os.ReadFile(filepath.Join(supportDir, "recaps", hire.ID+".md"))
		line := "- Worktree: " + hire.Worktree.storedOnly() + " (kept). It was moved or replaced, so Smolcorp left it alone. Don't run git in it. To remove it yourself, delete what's at " + hire.Worktree.storedOnly() + " (only the link, if it's a link; a folder, once you've copied out what you need, without git), then run `git worktree prune` in your own checkout.\n"
		if err != nil || !strings.Contains(string(data), line) {
			t.Errorf("the recap (%v):\n%s\nwant the line %q", err, data, line)
		}
	})
	t.Run("unchanged", func(t *testing.T) {
		_, r, hire := hiredForFire(t, "")
		windowShows(r, pane(t, "idle-empty"), true)
		if kept, err := fireEngineHire(hire, 0, r.o, false); err != nil || kept != "" {
			t.Fatalf("the fire: kept %q, err %v", kept, err)
		}
		if _, err := os.Stat(hire.Worktree.storedOnly()); err == nil || branches(t, r.repo) != "" {
			t.Error("the unchanged tree or its branch was left")
		}
		data, err := os.ReadFile(filepath.Join(supportDir, "recaps", hire.ID+".md"))
		if err != nil || !strings.Contains(string(data), "- Worktree: "+hire.Worktree.storedOnly()+" (removed)\n") {
			t.Errorf("the recap (%v):\n%s", err, data)
		}
	})
}

// The other ends that take a hire's tree away through the same check: the watch's clean-up (cleanUpHire), the
// launcher's end (removeLauncherHireWorktree) and the plain hire's end (removeOwnWorktree), the folder for hires
// turned the same way: none runs git in the GM-made folder or the other repo, none removes anything, and the
// clean-up's row says why. Control: the real folder put back, the tree is taken.
func TestEveryOtherEndRunsNoGitInATurnedTree(t *testing.T) {
	hire, _, trees := cleanupHireIn(t, "ab12cd34ef56", 0)
	other, before := r6Other(t, hire.Branch)
	calls := gitCalls(t)
	gm, putBack := r6Turn(t, hire.Worktree.storedOnly(), other)
	failed := ""
	left := cleanUpHire(hire, "a test", nil, removeHireWorktree(nil, 0, trees), func(_, why string) { failed = why })
	want := " Its worktree " + hire.Worktree.storedOnly() + " was kept: it was moved or replaced, so Smolcorp left it alone. To remove it yourself, delete what's at " + hire.Worktree.storedOnly() + " (only the link, if it's a link), then run git worktree prune in your own checkout."
	if left != want || failed != "Didn't start: a test."+want {
		t.Errorf("the clean-up: %q, its row %q; want %q", left, failed, want)
	}
	launched := hire
	launched.ViaLauncher = true
	if removeLauncherHireWorktree(nil, trees, func(Hire) bool { return true })(launched) {
		t.Error("the launcher's end took the turned tree")
	}
	if removeOwnWorktree(nil, trees, hire) {
		t.Error("the plain end took the turned tree")
	}
	if uncommittedRefusal(hire, runGit) != "" {
		t.Error("the fire's look for unsaved work refused a turned tree")
	}
	r6Untouched(t, "the other ends", gm, other, before, calls)

	// Control: put back, the tree is the hire's own again and is taken.
	putBack()
	if hireTreeMoved(hire) || !removeOwnWorktree(nil, trees, hire) {
		t.Fatal("the tree put back wasn't taken")
	}
	if _, err := os.Stat(hire.Worktree.storedOnly()); err == nil {
		t.Error("the tree put back is still there")
	}
}

// The owner's own repo, ~/code/app (the test's stand-in home), through a fire: its tree, where its hire left it, is
// taken as before, with its branch, the recap says it was removed, and the owner's checkout is as it was.
func TestTheOwnersAppHiresTreeIsTakenAtItsFireAsBefore(t *testing.T) {
	_, r, _ := engineDeck(t)
	ownersApp(t, filepath.Join(homeDir, "code", "app"))
	app := physical(filepath.Join(homeDir, "code", "app"))
	request := r.request("janitor")
	request.Folder = app
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	if hireTreeMoved(hire) || filepath.Dir(hire.Worktree.storedOnly()) != hiresFolder(app, "") { // the core's own folder (FXCOREWORKTREES126)
		t.Fatalf("its tree: %q", hire.Worktree)
	}
	windowShows(r, pane(t, "idle-empty"), true)
	if kept, err := fireEngineHire(hire, 0, r.o, false); err != nil || kept != "" {
		t.Fatalf("the fire: kept %q, err %v", kept, err)
	}
	if _, err := os.Stat(hire.Worktree.storedOnly()); err == nil {
		t.Error("its tree was left")
	}
	if left := standInOwnersGit(t, app, "branch", "--list", "hire/*"); strings.TrimSpace(left) != "" {
		t.Errorf("its branch was left: %q", left)
	}
	if list := standInOwnersGit(t, app, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Errorf("a registration is left: %q", list)
	}
	if status := standInOwnersGit(t, app, "status", "--porcelain"); strings.TrimSpace(status) != "" || readOr(filepath.Join(app, "marker.txt")) != "harmless\n" {
		t.Errorf("the owner's checkout changed: %q", status)
	}
	data, err := os.ReadFile(filepath.Join(supportDir, "recaps", hire.ID+".md"))
	if err != nil || !strings.Contains(string(data), "- Worktree: "+hire.Worktree.storedOnly()+" (removed)\n") {
		t.Errorf("the recap (%v):\n%s", err, data)
	}
}
