//go:build darwin

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXWINWORKTREER2: the R1 review's B1 probes, kept as rows, and its N2. An untracked file whose folder is deeper
// than the Mac's PATH_MAX (1024) from the tree's top: git's status can't open that folder, so it warns on standard
// error, exits 0 and says nothing on standard output, while os.Root's RemoveAll (openat) would still reach it. The
// fire never deletes that work: the tree is kept where it was, whole, with the hire's real profile too.

var deepFolder = strings.Repeat("d", 100)

const deepFolders = 12

// deepUntracked writes work.txt 12 folders of 100 letters down in tree, step by step through os.Root.
func deepUntracked(t *testing.T, tree string) {
	t.Helper()
	r, err := os.OpenRoot(tree)
	if err != nil {
		t.Fatal(err)
	}
	for range deepFolders {
		if err := r.Mkdir(deepFolder, 0o755); err != nil {
			t.Fatal(err)
		}
		next, err := r.OpenRoot(deepFolder)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		r = next
	}
	defer r.Close()
	if err := r.WriteFile("work.txt", []byte("unsaved work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// deepWorkLeftIn is the folder beside tree that still holds work.txt: the tree itself, or a moved-aside copy of it
// (.<name>.fired-…); "" for none.
func deepWorkLeftIn(t *testing.T, tree string) string {
	t.Helper()
	parent, name := filepath.Dir(tree), filepath.Base(tree)
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		if e.Name() != name && !strings.HasPrefix(e.Name(), "."+name+".fired-") {
			continue
		}
		r, err := os.OpenRoot(filepath.Join(parent, e.Name()))
		for i := 0; i < deepFolders && err == nil; i++ {
			var next *os.Root
			next, err = r.OpenRoot(deepFolder)
			r.Close()
			r = next
		}
		if err != nil {
			continue
		}
		_, err = r.Stat("work.txt")
		r.Close()
		if err == nil {
			return e.Name()
		}
	}
	return ""
}

func TestUntrackedWorkPastThePathLimitKeepsItsTree(t *testing.T) {
	t.Run("a worktree with no row", func(t *testing.T) {
		isolate(t)
		repo := placeRepo(t)
		path := filepath.Join(t.TempDir(), "w-deep")
		gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-deep", path)
		deepUntracked(t, path)
		logged := logOf(t)
		if removeWorkspace(repo, path, "hire/w-deep", runGit) {
			t.Error("a worktree git couldn't wholly look at was removed")
		}
		if where := deepWorkLeftIn(t, path); where != "w-deep" {
			t.Errorf("the untracked work.txt git couldn't see isn't in the tree where it was (left in %q)", where)
		}
		keptWhole(t, repo, path, "hire/w-deep", filepath.Join("sub", "a.txt"))
		if text := logged.String(); !strings.Contains(text, "w-deep is kept") || !strings.Contains(text, "could not open directory") {
			t.Errorf("the log doesn't name the kept tree with git's words:\n%s", text)
		}
	})
	t.Run("a sandboxed hire's tree", func(t *testing.T) {
		r := newDeskRunRig(t)
		sandboxedHire(t, r)
		deepUntracked(t, r.tree)
		logged := logOf(t)
		// removeOwnWorktree alone, as a fire with force, a gone hire's or a failed start's cleanup reach it: without
		// uncommittedRefusal's look first.
		if removeOwnWorktree(runGit, "", r.hire) {
			t.Error("a hire's tree git couldn't wholly look at was removed")
		}
		if where := deepWorkLeftIn(t, r.tree); where != "ada-abcdef" {
			t.Errorf("the untracked work.txt git couldn't see isn't in the tree where it was (left in %q)", where)
		}
		keptWhole(t, r.repo, r.tree, r.hire.Branch, "a.txt")
		if text := logged.String(); !strings.Contains(text, "ada-abcdef is kept") || !strings.Contains(text, "could not open directory") {
			t.Errorf("the log doesn't name the kept tree with git's words:\n%s", text)
		}
	})
}

// N2: deskGitAs' split with a desk's sandbox, at both looks: a warning on the status's standard error keeps the
// hire's tree. The stand-in git runs in the hire's sandbox, as the real one does.
func TestOnAMacAWarningBesideASandboxedHiresStatusKeepsIt(t *testing.T) {
	for _, look := range theTwoLooks {
		t.Run(look.name, func(t *testing.T) {
			r := newDeskRunRig(t)
			sandboxedHire(t, r)
			gitStandInBeside(t, gitWarnsOnTheStatus(look.when))
			logged := logOf(t)
			if removeOwnWorktree(runGit, "", r.hire) {
				t.Error("a hire's tree git warned it couldn't wholly look at was removed")
			}
			keptWhole(t, r.repo, r.tree, r.hire.Branch, "a.txt")
			text := logged.String()
			if !strings.Contains(text, "ada-abcdef is kept") || !strings.Contains(text, "a stand-in git's warning") {
				t.Errorf("the log doesn't name the kept tree with git's words:\n%s", text)
			}
			if !strings.Contains(text, look.logged) {
				t.Errorf("the log doesn't say %q, so another look kept the tree:\n%s", look.logged, text)
			}
			if strings.Contains(text, " was refused (") { // deskRefused's words (FXWINWORKTREER4's O1)
				t.Errorf("the stand-in git wasn't run in the sandbox, so its warning proves nothing:\n%s", text)
			}
		})
	}
}
