//go:build !windows

package desktop

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// FXWINWORKTREER1 (the owner's ruling on FXWINWORKTREE §2 and §7) and FXWINWORKTREER2 (the R1 review's B1): the fire's
// exact looks at a clean tree read git's standard output apart from its standard error. The branch it has checked out
// and its submodules' list are standard output alone, so a warning beside them changes nothing. Its status isn't: git
// says it couldn't open a folder (a path too long, a folder the owner can't read) only on standard error, with exit 0,
// so any line there that harmlessGitWarnings doesn't name keeps the tree, at the fire's first look and at the look
// after the tree is moved aside. A git that ends with an error keeps it too, and every kept tree says why in the log,
// with git's own words. Stand-ins are a script first on PATH that says its words and then runs the real git (a plain
// shell script, so these rows are off Windows; hireremove_windows_test.go has the real git there); the folder git
// can't read is a real one.

// gitStandInBeside puts a stand-in git first on PATH: script runs first, with git's arguments as "$@", then the real
// git with them, unless script exits.
func gitStandInBeside(t *testing.T, script string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	bin := t.TempDir()
	wrapper := "#!/bin/sh\n" + script + "\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A warning on standard error, and git's own answer after it, for the reads whose answer is standard output alone:
// the branch a tree has checked out, and the index's records.
const gitWarnsBesideANameOrRecords = `case " $* " in *" ls-files "*|*" symbolic-ref "*)
  echo "warning: a stand-in git's warning, on its standard error" >&2;;
esac`

// gitWarnsOnTheStatus is a warning on standard error, and git's own answer after it, for the status whose folder and
// args match when (a case pattern): the folder is in the args (-C) outside a sandbox, and the one git runs in inside.
func gitWarnsOnTheStatus(when string) string {
	return `case " $PWD $* " in ` + when + `)
  echo "warning: could not open directory 'stand-in/': a stand-in git's warning" >&2;;
esac`
}

// logOf has the package's log written to it until the test ends.
func logOf(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &logged
}

// goneWithItsBranch is an error for each thing of the worktree at path, on branch of repo, that is still there.
func goneWithItsBranch(t *testing.T, repo, path, branch string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("the clean worktree is still at %s", path)
	}
	if left, _ := os.ReadDir(filepath.Join(repo, ".git", "worktrees")); len(left) != 0 {
		t.Errorf("a registration is left: %v", left)
	}
	if out, _ := exec.Command("git", "-C", repo, "branch", "--list", branch).Output(); strings.TrimSpace(string(out)) != "" {
		t.Errorf("the branch is left: %q", out)
	}
}

// keptWhole is an error for each thing of the worktree at tree, on branch of repo, that isn't where it was: its file,
// its registration, its branch; and for a copy of it left moved aside.
func keptWhole(t *testing.T, repo, tree, branch, file string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(tree, file)); err != nil {
		t.Errorf("the worktree isn't where it was, whole: %v", err)
	}
	if aside, _ := filepath.Glob(filepath.Join(filepath.Dir(tree), "."+filepath.Base(tree)+".fired-*")); len(aside) != 0 {
		t.Errorf("a copy is left moved aside: %v", aside)
	}
	if left, _ := os.ReadDir(filepath.Join(repo, ".git", "worktrees")); len(left) == 0 {
		t.Error("its registration was removed")
	}
	if out, _ := exec.Command("git", "-C", repo, "branch", "--list", branch).Output(); strings.TrimSpace(string(out)) == "" {
		t.Error("the branch was deleted")
	}
}

// cleanTree is a clean worktree the fire removes: tree, on branch of repo, with file committed, and the fire's
// removal of it.
type cleanTree struct {
	repo, tree, branch, file string
	remove                   func() bool
}

// cleanTrees are the fire's two ways to a tree: one with no hire's row (removeWorkspace, and git's removal handed to
// removeDeskTree), and a hire's in the registry (runGit by the registry, removeDeskTree for the hire).
var cleanTrees = []struct {
	name string
	make func(t *testing.T) cleanTree
}{
	{"a worktree with no row", func(t *testing.T) cleanTree {
		isolate(t)
		repo := placeRepo(t)
		path := filepath.Join(t.TempDir(), "w-warn")
		gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-warn", path)
		return cleanTree{repo, path, "hire/w-warn", filepath.Join("sub", "a.txt"), func() bool {
			return removeWorkspace(repo, path, "hire/w-warn", runGit)
		}}
	}},
	{"a hire's worktree in the registry", func(t *testing.T) cleanTree {
		isolate(t)
		r := newDeskRunRig(t)
		return cleanTree{r.repo, r.tree, r.hire.Branch, "a.txt", func() bool { return removeOwnWorktree(nil, "", r.hire) }}
	}},
}

// The two looks at a tree's status: the fire's first, and the one again once the tree is moved aside
// (removeDeskTree), in its new name. logged is what the log says when that look keeps the tree, so a row shows which
// look kept it. The first look's stand-in is quiet once the tree is moved aside, and its words are removeWorkspace's
// own, so only the first look can keep the tree there (FXWINWORKTREER3, the R2 review's N2).
var theTwoLooks = []struct{ name, when, logged string }{
	{"at the fire's first look", `*".fired-"*) ;; *" status "*`, "lines in git's status"},
	{"at the look after the tree is moved aside", `*".fired-"*" status "*`, "git warned while it looked"},
}

func TestAWarningBesideACleanTreesStatusKeepsIt(t *testing.T) {
	for _, tree := range cleanTrees {
		for _, look := range theTwoLooks {
			t.Run(tree.name+", "+look.name, func(t *testing.T) {
				c := tree.make(t)
				gitStandInBeside(t, gitWarnsOnTheStatus(look.when))
				logged := logOf(t)
				if c.remove() {
					t.Error("a worktree git warned it couldn't wholly look at was removed")
				}
				keptWhole(t, c.repo, c.tree, c.branch, c.file)
				text := logged.String()
				if !strings.Contains(text, filepath.Base(c.tree)+" is kept") || !strings.Contains(text, "a stand-in git's warning") {
					t.Errorf("the log doesn't name the kept tree with git's words:\n%s", text)
				}
				if !strings.Contains(text, look.logged) {
					t.Errorf("the log doesn't say %q, so another look kept the tree:\n%s", look.logged, text)
				}
				if strings.Contains(text, "no row's worktree") {
					t.Errorf("the log names a tree with no row oddly:\n%s", text)
				}
			})
		}
	}
}

// The controls: a warning beside a branch's name or the index's records, git's standard output alone, keeps nothing.
func TestAWarningBesideABranchsNameOrTheIndexsRecordsKeepsNothing(t *testing.T) {
	t.Run("a worktree with no row", func(t *testing.T) {
		isolate(t)
		repo := placeRepo(t)
		path := filepath.Join(t.TempDir(), "w-warn")
		gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-warn", path)
		gitStandInBeside(t, gitWarnsBesideANameOrRecords)
		logged := logOf(t)
		if !removeWorkspace(repo, path, "hire/w-warn", runGit) {
			t.Errorf("the clean worktree was kept; the log:\n%s", logged)
		}
		goneWithItsBranch(t, repo, path, "hire/w-warn")
	})
	t.Run("a hire's worktree with no row", func(t *testing.T) {
		isolate(t)
		repo := placeRepo(t)
		o := placeOptions(t)
		place, err := prepareHire(hireRequest{HireID: "0123456789ab", Name: "Bo", Folder: repo, Worktree: true}, o)
		if err != nil {
			t.Fatal(err)
		}
		gitStandInBeside(t, gitWarnsBesideANameOrRecords)
		logged := logOf(t)
		h := Hire{ID: "0123456789ab", Repo: place.Repo, Worktree: place.Worktree, Branch: place.Branch}
		if !removeOwnWorktree(nil, o.Worktrees, h) {
			t.Errorf("the hire's clean worktree was kept; the log:\n%s", logged)
		}
		goneWithItsBranch(t, repo, place.Worktree, place.Branch)
		if !strings.Contains(logged.String(), "a stand-in git's warning") {
			t.Errorf("git's warning isn't in the log:\n%s", logged)
		}
	})
	t.Run("a hire's worktree in the registry", func(t *testing.T) {
		isolate(t)
		r := newDeskRunRig(t)
		gitStandInBeside(t, gitWarnsBesideANameOrRecords)
		logged := logOf(t)
		if !removeOwnWorktree(nil, "", r.hire) {
			t.Errorf("the hire's clean worktree was kept; the log:\n%s", logged)
		}
		goneWithItsBranch(t, r.repo, r.tree, r.hire.Branch)
	})
}

// The data loss B1 found, with a folder the owner can't read: git's status says only "warning: could not open
// directory", on standard error, with exit 0. The tree is kept where it was, whole, with the work in that folder.
func TestAFolderGitCantReadKeepsItsTreeWhole(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a folder with no permissions is still read by root")
	}
	for _, tree := range cleanTrees {
		t.Run(tree.name, func(t *testing.T) {
			c := tree.make(t)
			private := filepath.Join(c.tree, "private")
			if err := os.Mkdir(private, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(private, "work.txt"), []byte("unsaved work\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(private, 0); err != nil {
				t.Fatal(err)
			}
			// Readable again before the test's folders go, wherever the tree is by then.
			t.Cleanup(func() {
				left, _ := filepath.Glob(filepath.Join(filepath.Dir(c.tree), "*", "private"))
				for _, folder := range left {
					os.Chmod(folder, 0o755)
				}
			})
			if _, err := os.ReadDir(private); err == nil {
				t.Skip("this computer reads a folder with no permissions")
			}
			logged := logOf(t)
			if c.remove() {
				t.Error("a worktree with a folder git couldn't read was removed")
			}
			os.Chmod(private, 0o755)
			if _, err := os.Stat(filepath.Join(private, "work.txt")); err != nil {
				t.Errorf("the work in the folder git couldn't read isn't where it was: %v", err)
			}
			keptWhole(t, c.repo, c.tree, c.branch, c.file)
			if text := logged.String(); !strings.Contains(text, filepath.Base(c.tree)+" is kept") || !strings.Contains(text, "could not open directory") {
				t.Errorf("the log doesn't name the kept tree with git's words:\n%s", text)
			}
		})
	}
}

func TestAGitThatFailsOnACleanTreeKeepsItAndTheLogSaysWhy(t *testing.T) {
	// logged is the words of the look that keeps the tree. The first look's stand-in is quiet once the tree is moved
	// aside, so only the first look can keep it there, as theTwoLooks' (FXWINWORKTREER4, the R3 review's O2).
	for _, c := range []struct{ name, when, logged string }{
		{"at the fire's first look", `*".fired-"*) ;; *" status "*`, "git's status there ended with 128"},
		// The look again once the tree is moved aside (removeDeskTree), in its new name.
		{"at the look after the tree is moved aside", `*".fired-"*" status "*`, "it holds changes, or git couldn't say (git ended with 128"},
	} {
		t.Run(c.name, func(t *testing.T) {
			isolate(t)
			repo := placeRepo(t)
			path := filepath.Join(t.TempDir(), "w-fails")
			gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-fails", path)
			gitStandInBeside(t, `case " $* " in `+c.when+`)
  echo "fatal: a stand-in git's refusal" >&2; exit 128;;
esac`)
			logged := logOf(t)
			if removeWorkspace(repo, path, "hire/w-fails", runGit) {
				t.Error("a worktree git couldn't look at was removed")
			}
			if _, err := os.Stat(filepath.Join(path, "sub", "a.txt")); err != nil {
				t.Errorf("the worktree isn't where it was: %v", err)
			}
			if out, _ := exec.Command("git", "-C", repo, "branch", "--list", "hire/w-fails").Output(); strings.TrimSpace(string(out)) == "" {
				t.Error("the branch was deleted")
			}
			text := logged.String()
			if !strings.Contains(text, "w-fails is kept") || !strings.Contains(text, "fatal: a stand-in git's refusal") {
				t.Errorf("the log doesn't name the kept tree with git's words:\n%s", text)
			}
			if !strings.Contains(text, c.logged) {
				t.Errorf("the log doesn't say %q, so another look kept the tree:\n%s", c.logged, text)
			}
		})
	}
}

// FXWINWORKTREER3: harmlessGitWarnings' one form, the words Git for Windows printed beside a clean tree's status on
// windows-latest, with exit 0: that line alone lets the tree go, at both looks. The same words naming a path or with
// another reason, a folder git couldn't open, or that line with more beside it, still keep the tree whole (the R2
// review's O2).
func TestOnlyTheHarmlessWarningLetsACleanTreeGo(t *testing.T) {
	const harmless = "warning: unable to access '': Invalid argument"
	for _, said := range []struct {
		name, lines string
		removed     bool
	}{
		{"Git for Windows' words for an empty name", harmless, true},
		{"the same words with a Windows line end", harmless + "\r", true},
		{"the same words naming a path", "warning: unable to access 'C:/x': Invalid argument", false},
		{"an empty name with another reason", "warning: unable to access '': Permission denied", false},
		{"a folder git couldn't open", "warning: could not open directory 'sub/': Permission denied", false},
		{"a path past the limit", "warning: could not open directory 'sub/deep/': Filename too long", false},
		{"more after the same words", harmless + " (and 'sub/')", false},
		{"the same words, then a folder git couldn't open", harmless + "\nwarning: could not open directory 'sub/': Permission denied", false},
	} {
		for _, tree := range cleanTrees {
			t.Run(said.name+", "+tree.name, func(t *testing.T) {
				c := tree.make(t)
				words := filepath.Join(t.TempDir(), "words")
				if err := os.WriteFile(words, []byte(said.lines+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				gitStandInBeside(t, `case " $* " in *" status "*) cat '`+words+`' >&2;; esac`)
				logged := logOf(t)
				removed := c.remove()
				text := logged.String()
				if !strings.Contains(text, "said on its standard error (exit 0)") {
					t.Errorf("the stand-in's words weren't on the status's standard error:\n%s", text)
				}
				if !said.removed {
					if removed {
						t.Error("a worktree git warned it couldn't wholly look at was removed")
					}
					keptWhole(t, c.repo, c.tree, c.branch, c.file)
					if !strings.Contains(text, filepath.Base(c.tree)+" is kept") {
						t.Errorf("the log doesn't name the kept tree:\n%s", text)
					}
					return
				}
				if !removed {
					t.Errorf("the clean worktree was kept for git's harmless line; the log:\n%s", text)
				}
				goneWithItsBranch(t, c.repo, c.tree, c.branch)
			})
		}
	}
}

// FXWINWORKTREER3 (the R2 review's O1): an answer from git over gitOutputCap is refused as too big, on either stream
// (exec's copy of a stream goes through gitOutput's Write, where the cap is), and a clean tree whose status says that
// much on its standard error is kept.
func TestAGitAnswerOverTheCapIsRefusedAsTooBig(t *testing.T) {
	isolate(t)
	repo := placeRepo(t)
	path := filepath.Join(t.TempDir(), "w-big")
	gitRepoCmd(t, repo, "worktree", "add", "-q", "-b", "hire/w-big", path)
	gitStandInBeside(t, `case " $* " in
  *" status "*) head -c 5000000 /dev/zero | tr '\0' x >&2; exit 0;;
  *" log "*) head -c 5000000 /dev/zero | tr '\0' x; exit 0;;
esac`)
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	status := []string{"status", "--porcelain", "--ignore-submodules=all"}
	if _, _, _, err := gitRunSplit(ctx, path, gitCall{global: gitReadArgs}, status); !errors.Is(err, errGitTooBig) {
		t.Errorf("5 MB on the status's standard error, its streams apart (gitRunSplit): %v, want too big", err)
	}
	if _, _, err := gitRun(ctx, path, gitCall{global: gitReadArgs}, status); !errors.Is(err, errGitTooBig) {
		t.Errorf("5 MB on the status's standard error, its streams together (gitRun): %v, want too big", err)
	}
	if _, _, err := execGit(ctx, gitReadEnv(os.Environ(), nil), "-C", path, "log"); !errors.Is(err, errGitTooBig) {
		t.Errorf("5 MB on the log's standard output (execGit): %v, want too big", err)
	}
	logged := logOf(t)
	if removeWorkspace(repo, path, "hire/w-big", runGit) {
		t.Error("a worktree whose status said 5 MB on its standard error was removed")
	}
	keptWhole(t, repo, path, "hire/w-big", filepath.Join("sub", "a.txt"))
	if text := logged.String(); !strings.Contains(text, errGitTooBig.Error()) || !strings.Contains(text, "w-big is kept") {
		t.Errorf("the log doesn't say the status was too big, and the tree kept:\n%s", text)
	}
}

// FXPOLISHA (RT126FIXREVIEW-WINWORKTREER4's O1): git's listing of a clean tree's index, at exit 0, whose last record
// it didn't end (cut short) keeps the tree, saying so: the fire's look reads the records as they come, and only the
// listing's end (gitRecords.end) can tell that a last record never came whole. Real git always ends its records; a
// stand-in git answers the listing with one record and no NUL after it.
func TestAListingOfACleanTreesIndexCutShortKeepsIt(t *testing.T) {
	id := strings.Repeat("a", 40)
	for _, tree := range cleanTrees {
		t.Run(tree.name, func(t *testing.T) {
			c := tree.make(t)
			gitStandInBeside(t, `case " $* " in *" ls-files "*) printf '100644 `+id+` 0\ta.txt'; exit 0;; esac`)
			logged := logOf(t)
			if c.remove() {
				t.Error("a worktree whose index git listed cut short was removed")
			}
			keptWhole(t, c.repo, c.tree, c.branch, c.file)
			if text := logged.String(); !strings.Contains(text, filepath.Base(c.tree)+" is kept") ||
				!strings.Contains(text, "git's listing ends in the middle of a record") {
				t.Errorf("the log doesn't say the listing was cut short:\n%s", text)
			}
		})
	}
}
