//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// FXWINWORKTREER4 (the R3 review's N1): the fire's look for a clean tree's submodules, and the core's own git folder
// for a read on a computer (and for a write everywhere), read listings that grow with the repo: its index
// (`ls-files -z --stage`, some 150 bytes a file here) and its branches (`for-each-ref`). Read whole through gitOutput's
// cap, a clean tree of some 40,000 files, or a repo of some 45,000 branches, was kept with its branch, and the log said
// git couldn't list it. They are read as they come now (gitRecords): such a clean tree goes with its branch, one with
// a change is still kept, and a submodule checked out where the listing is past the cap is still found. Off Windows,
// as the rows beside them (hireremovewarn_test.go): the fire's way is the same code there.

// bigTree is a clean worktree of a grown repo: tree, on branch of repo, and the fire's removal of it.
type bigTree struct {
	repo, tree, branch string
	remove             func() bool
}

// bigTreeKind is one of the fire's ways to a tree, its repo grown by grow before the tree is brought up to it.
type bigTreeKind struct {
	name string
	make func(t *testing.T, grow func(t *testing.T, repo string)) bigTree
}

// bigTreeKinds are the fire's ways to a tree: one with no hire's row (removeWorkspace, git's removal handed to
// removeDeskTree), a hire's in the registry, and on a Mac a hire's with its real profile (hireremovebig_darwin_test.go).
var bigTreeKinds = []bigTreeKind{
	{"a worktree with no row", func(t *testing.T, grow func(*testing.T, string)) bigTree {
		isolate(t)
		repo := placeRepo(t)
		grow(t, repo)
		path := filepath.Join(t.TempDir(), "w-big")
		gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-big", path)
		return bigTree{repo, path, "hire/w-big", func() bool { return removeWorkspace(repo, path, "hire/w-big", runGit) }}
	}},
	{"a hire's worktree in the registry", func(t *testing.T, grow func(*testing.T, string)) bigTree {
		isolate(t)
		return grownRig(t, newDeskRunRig(t), grow)
	}},
}

// grownRig is r's hire's tree, its repo grown by grow and the tree brought up to its main.
func grownRig(t *testing.T, r *deskRunRig, grow func(*testing.T, string)) bigTree {
	t.Helper()
	grow(t, r.repo)
	gitRepoCmd(t, r.tree, "merge", "-q", "--ff-only", "main")
	return bigTree{r.repo, r.tree, r.hire.Branch, func() bool { return removeOwnWorktree(runGit, "", r.hire) }}
}

// bigFiles is the files of a big index: with bigIndexName's paths, its listing is some 4.6 MB, past gitOutputCap.
const bigFiles = 30001

// bigIndexName is the path of a big index's file i, some 100 letters long.
func bigIndexName(i int) string {
	return fmt.Sprintf("big/%03d/%s%05d.txt", i%100, strings.Repeat("a-long-file-name-", 5), i)
}

// growIndex commits bigFiles files (one blob) on the repo's main, and a gitlink at zz-sub, which git lists last, when
// gitlink: with git's plumbing, through an index of the test's, so the repo's own checkout isn't written.
func growIndex(gitlink bool) func(t *testing.T, repo string) {
	return func(t *testing.T, repo string) {
		t.Helper()
		one := filepath.Join(t.TempDir(), "one")
		if err := os.WriteFile(one, []byte("one file of many\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		blob := gitRepoCmd(t, repo, "hash-object", "-w", one)
		var list strings.Builder
		for i := range bigFiles {
			fmt.Fprintf(&list, "100644 %s\t%s\n", blob, bigIndexName(i))
		}
		if gitlink {
			fmt.Fprintf(&list, "160000 %s\tzz-sub\n", gitRepoCmd(t, repo, "rev-parse", "HEAD"))
		}
		index := filepath.Join(t.TempDir(), "index")
		withIndex := func(in string, args ...string) string {
			cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
			cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
			cmd.Stdin = strings.NewReader(in)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
			return strings.TrimSpace(string(out))
		}
		withIndex("", "read-tree", "HEAD")
		withIndex(list.String(), "update-index", "--index-info")
		commit := gitRepoCmd(t, repo, "commit-tree", withIndex("", "write-tree"), "-p", "HEAD", "-m", "many files")
		gitRepoCmd(t, repo, "update-ref", "refs/heads/main", commit)
	}
}

// indexPastTheCap fails the test unless git's listing of tree's index is past gitOutputCap: the row's own shape.
func indexPastTheCap(t *testing.T, tree string) {
	t.Helper()
	out, err := exec.Command("git", "-C", tree, "ls-files", "-z", "--stage").Output()
	if err != nil || len(out) <= gitOutputCap {
		t.Fatalf("the index's listing is %d bytes, not past the cap (%d): %v", len(out), gitOutputCap, err)
	}
}

func TestACleanTreeWithABigIndexGoesWithItsBranch(t *testing.T) {
	for _, kind := range bigTreeKinds {
		t.Run(kind.name, func(t *testing.T) {
			c := kind.make(t, growIndex(false))
			indexPastTheCap(t, c.tree)
			logged := logOf(t)
			if !c.remove() {
				t.Errorf("a clean tree with a big index was kept:\n%s", logged)
			}
			goneWithItsBranch(t, c.repo, c.tree, c.branch)
		})
	}
}

func TestABigIndexsTreeWithAChangedFileIsKept(t *testing.T) {
	last := filepath.FromSlash(bigIndexName(bigFiles - 1))
	for _, kind := range bigTreeKinds {
		t.Run(kind.name, func(t *testing.T) {
			c := kind.make(t, growIndex(false))
			indexPastTheCap(t, c.tree)
			if err := os.WriteFile(filepath.Join(c.tree, last), []byte("unsaved work\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			logged := logOf(t)
			if c.remove() {
				t.Error("a big index's tree with a changed file was removed")
			}
			keptWhole(t, c.repo, c.tree, c.branch, last)
			if data, _ := os.ReadFile(filepath.Join(c.tree, last)); string(data) != "unsaved work\n" {
				t.Errorf("the changed file isn't as it was: %q", data)
			}
			if !strings.Contains(logged.String(), filepath.Base(c.tree)+" is kept") {
				t.Errorf("the log doesn't name the kept tree:\n%s", logged)
			}
		})
	}
}

// A submodule's folder, past the cap in the listing (git lists zz-sub last), with a .git in it: the look after the
// move aside finds it there, as before the cap held.
func TestASubmoduleCheckedOutInABigIndexKeepsTheTree(t *testing.T) {
	sub := filepath.Join("zz-sub", ".git")
	for _, kind := range bigTreeKinds {
		t.Run(kind.name, func(t *testing.T) {
			c := kind.make(t, growIndex(true))
			indexPastTheCap(t, c.tree)
			if err := os.Mkdir(filepath.Join(c.tree, sub), 0o755); err != nil {
				t.Fatal(err)
			}
			logged := logOf(t)
			if c.remove() {
				t.Error("a tree with a submodule checked out was removed")
			}
			keptWhole(t, c.repo, c.tree, c.branch, sub)
			if !strings.Contains(logged.String(), "a submodule is checked out in it") {
				t.Errorf("the log doesn't say the submodule kept it:\n%s", logged)
			}
		})
	}
}

// manyBranches packs 50,000 branches into the repo at its HEAD, with names of some 75 letters: git's listing of them
// is some 6 MB, past gitOutputCap.
func manyBranches(t *testing.T, repo string) {
	t.Helper()
	head := gitRepoCmd(t, repo, "rev-parse", "HEAD")
	var refs strings.Builder
	for i := range 50000 {
		fmt.Fprintf(&refs, "%s refs/heads/many/%s%05d\n", head, strings.Repeat("a-long-branch-name-", 3), i)
	}
	file, err := os.OpenFile(filepath.Join(repo, ".git", "packed-refs"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err == nil {
		_, err = file.WriteString(refs.String())
		err = errors.Join(err, file.Close())
	}
	if err != nil {
		t.Fatal(err)
	}
	if listed := gitRepoCmd(t, repo, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/"); len(listed) <= gitOutputCap {
		t.Fatalf("the branches' listing is %d bytes, not past the cap", len(listed))
	}
}

// The core's folder lists every branch (makeGitReadFolder), and a write through it lists them again before and after
// (gitBranchesIn): a fire's status on a computer, its removal, and its branch's deletion everywhere.
func TestAFireInARepoOfManyBranchesRemovesTheCleanTree(t *testing.T) {
	first := "refs/heads/many/" + strings.Repeat("a-long-branch-name-", 3) + "00000"
	for _, kind := range bigTreeKinds {
		t.Run(kind.name, func(t *testing.T) {
			c := kind.make(t, manyBranches)
			logged := logOf(t)
			if !c.remove() {
				t.Errorf("a clean tree in a repo of many branches was kept:\n%s", logged)
			}
			goneWithItsBranch(t, c.repo, c.tree, c.branch)
			if got := gitRepoCmd(t, c.repo, "for-each-ref", "--format=%(refname)", first); got != first {
				t.Errorf("the repo's other branches went too: %q", got)
			}
		})
	}
}
