//go:build !windows

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

// FXSWEEP3P3R3: round 3 on FXSWEEP3P3, the cold review's B3, N3, N4 and O2. Throwaway repos and stand-ins only: a
// wrapper around the test's own gitRunner, or a stand-in git on the test's own PATH that renames a folder of the test's
// temp repo once and then runs the real git.

// r3Landed says what a hire wrote into the other repo ("Q"), whose git folder is gitDir: a hire's branch, a
// registration not in before, a hire's key in its config.
func r3Landed(t *testing.T, gitDir string, before []string) {
	t.Helper()
	if out, _ := exec.Command("git", "--git-dir="+gitDir, "for-each-ref", "--format=%(refname)", "refs/heads/hire/").Output(); strings.TrimSpace(string(out)) != "" {
		t.Errorf("a branch was written into the other repo: %q", out)
	}
	if got := r2Registrations(gitDir); !slices.Equal(got, before) {
		t.Errorf("a registration was written into the other repo: %v, was %v", got, before)
	}
	if config, _ := os.ReadFile(filepath.Join(gitDir, "config")); strings.Contains(string(config), "hire/") {
		t.Errorf("the other repo's config has a hire's branch: %s", config)
	}
}

// r3SeparateClone is x, a clone of the rig's repo (Q, given a remote) whose git folder is kept beside it, x-git, as a
// GM could make one in its own folder.
func r3SeparateClone(t *testing.T) (r *ownerRig, x, xGit, qGit string) {
	t.Helper()
	r = newOwnerRig(t)
	r.withRemote(t, "origin")
	gitRepoCmd(t, r.repo, "push", "-q", "origin", "main")
	x, xGit, qGit = filepath.Join(r.base, "x"), filepath.Join(r.base, "x-git"), filepath.Join(r.repo, ".git")
	gitRepoCmd(t, r.base, "clone", "-q", "--separate-git-dir", xGit, r.repo, x)
	stopSigning(t, x)
	return r, x, xGit, qGit
}

// B3: the repo the checks judged is the one written, from the look to the last write. Q is a plain repo; x a clone of
// it whose git folder is x-git. One rename or rewrite in x's own folders after the checks (during the core's own
// worktree add, before presetUpstream's keys, before initHireSubmodules' look) puts nothing into Q.
func TestTheRepoTheChecksJudgedIsTheOneTheHireWrites(t *testing.T) {
	t.Run("control: no swap, hired into x's own git folder", func(t *testing.T) {
		r, x, xGit, qGit := r3SeparateClone(t)
		before := r2Registrations(qGit)
		place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
		if err != nil || place.Worktree == "" {
			t.Fatalf("not hired into x: %+v %v", place, err)
		}
		if got := r2Registrations(xGit); len(got) != 1 {
			t.Errorf("x's own git folder has registrations %v", got)
		}
		if config, _ := os.ReadFile(filepath.Join(xGit, "config")); !strings.Contains(string(config), "[branch \""+place.Branch+"\"]") {
			t.Errorf("x's config has no upstream for the hire's branch: %s", config)
		}
		r3Landed(t, qGit, before)
	})

	t.Run("x's git folder swapped for a link to Q's .git during the core's worktree add", func(t *testing.T) {
		r, x, xGit, qGit := r3SeparateClone(t)
		before := r2Registrations(qGit)
		real, err := exec.LookPath("git")
		if err != nil {
			t.Skip("no git")
		}
		bin, once := filepath.Join(r.base, "bin"), filepath.Join(r.base, "swap-once")
		writeRepoFile(t, r.base, "swap-once", "")
		writeRepoFile(t, bin, "git", "#!/bin/sh\ncase \" $* \" in *\" worktree add \"*) if [ -f '"+once+"' ]; then rm -f '"+once+
			"'; mv '"+xGit+"' '"+xGit+".real' && ln -s '"+qGit+"' '"+xGit+"'; fi;; esac\nexec '"+real+"' \"$@\"\n")
		if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
		if _, statErr := os.Stat(once); statErr == nil {
			t.Fatal("the stand-in never swapped, so this row proves nothing")
		}
		if err == nil {
			t.Errorf("hired, though x's git folder was swapped as the worktree was made: %+v", place)
		}
		r3Landed(t, qGit, before)
		// Nor half-carried into the folder the checks judged (now x-git.real): the carry refused as a whole.
		if got := r2Registrations(xGit + ".real"); len(got) != 0 {
			t.Errorf("a registration was carried into x's git folder: %v", got)
		}
	})

	t.Run("x's .git rewritten to name Q's .git before presetUpstream", func(t *testing.T) {
		r, x, xGit, qGit := r3SeparateClone(t)
		before := r2Registrations(qGit)
		swapped := false
		git := func(dir string, args ...string) (int, string, bool) {
			if !swapped && len(args) > 1 && args[0] == "config" && args[1] == "--local" {
				swapped = true
				writeRepoFile(t, x, ".git", "gitdir: "+qGit+"\n")
			}
			return runGit(dir, args...)
		}
		place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), git)
		if !swapped {
			t.Fatal("presetUpstream wrote no key, so this row proves nothing")
		}
		if err != nil {
			t.Fatalf("not hired into x: %+v %v", place, err)
		}
		r3Landed(t, qGit, before)
		if config, _ := os.ReadFile(filepath.Join(xGit, "config")); !strings.Contains(string(config), "[branch \""+place.Branch+"\"]") {
			t.Errorf("the upstream wasn't written where the worktree was carried, x's git folder: %s", config)
		}
	})

	t.Run("the hire's tree's .git rewritten to name Q's own registration before initHireSubmodules' look", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("the core initialises a hire's submodules on a Mac only")
		}
		r := newOwnerRig(t)
		gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", r2Source(t, r.base, "subsource"), "mod")
		gitRepoCmd(t, r.repo, "commit", "-q", "-m", "mod")
		gitRepoCmd(t, r.repo, "worktree", "add", "-q", "-b", "w", filepath.Join(r.base, "proj-w"))
		qGit := filepath.Join(r.repo, ".git")
		reg := filepath.Join(qGit, "worktrees", "proj-w")
		x := filepath.Join(r.base, "x")
		gitRepoCmd(t, r.base, "clone", "-q", r.repo, x)
		stopSigning(t, x)
		writeRepoFile(t, x, "mod/.git", "gitdir: "+filepath.Join(qGit, "modules", "mod")+"\n")
		swapped := false
		git := func(dir string, args ...string) (int, string, bool) {
			if !swapped && len(args) == 4 && args[0] == "rev-parse" && args[2] == "--git-dir" {
				swapped = true
				writeRepoFile(t, dir, ".git", "gitdir: "+reg+"\n")
			}
			return runGit(dir, args...)
		}
		place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), git)
		if !swapped {
			t.Fatal("initHireSubmodules never looked, so this row proves nothing")
		}
		if err != nil {
			t.Fatalf("not hired into x: %+v %v", place, err)
		}
		if _, err := os.Lstat(filepath.Join(reg, "modules")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a git folder was made in Q's own registration: %v", err)
		}
	})
}

// N3: x's .git a link to another work tree's .git, which is itself a link to that repo's git folder kept elsewhere
// (proj-git): judged by the way there, before it is resolved, and refused, with nothing written into proj-git.
func TestALinkToAnotherWorkTreesDotGitThatIsItselfALinkIsRefused(t *testing.T) {
	r := newOwnerRig(t)
	qGit := filepath.Join(r.base, "proj-git")
	if err := os.Rename(filepath.Join(r.repo, ".git"), qGit); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(qGit, filepath.Join(r.repo, ".git")); err != nil {
		t.Fatal(err)
	}
	x := filepath.Join(r.base, "x")
	if err := os.MkdirAll(x, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(r.repo, ".git"), filepath.Join(x, ".git")); err != nil {
		t.Fatal(err)
	}
	if top, common := gitRepoCmd(t, x, "rev-parse", "--show-toplevel"), gitRepoCmd(t, x, "rev-parse", "--path-format=absolute", "--git-common-dir"); physical(top) != physical(x) || !oneFolderOnDisk(common, qGit) {
		t.Fatalf("the control: git takes %q as the top and %q as the git folder, so this row proves nothing", top, common)
	}
	before := r2Registrations(qGit)
	place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
	var refused *hireRefusal
	if !errors.As(err, &refused) || refused.Status != 409 || !strings.Contains(refused.Message, x+"'s own: it is the .git of another work tree, "+r.repo) {
		t.Errorf("hired into x: %+v %v", place, err)
	}
	r3Landed(t, qGit, before)
}

// N4: an own repo whose git folder is kept elsewhere under the name .git, in a folder that holds nothing else
// (`clone --separate-git-dir <base>/store/x/.git`, or x/.git a link to it), is no other work tree's .git: it hires,
// its branch and registration in that git folder. Round 2's link to another work tree's .git stays refused
// (TestADotGitLinkToAnotherWorkTreesDotGitIsRefused).
func TestAnOwnGitFolderKeptElsewhereUnderTheNameDotGitHires(t *testing.T) {
	for _, c := range []struct {
		name string
		make func(t *testing.T, r *ownerRig, x, store string)
	}{
		{"clone --separate-git-dir <store>/x/.git", func(t *testing.T, r *ownerRig, x, store string) {
			gitRepoCmd(t, r.base, "clone", "-q", "--separate-git-dir", store, r.repo, x)
		}},
		{"x/.git a link to <store>/x/.git", func(t *testing.T, r *ownerRig, x, store string) {
			gitRepoCmd(t, r.base, "clone", "-q", r.repo, x)
			if err := os.Rename(filepath.Join(x, ".git"), store); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(store, filepath.Join(x, ".git")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newOwnerRig(t)
			x, store := filepath.Join(r.base, "x"), filepath.Join(r.base, "store", "x", ".git")
			if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
				t.Fatal(err)
			}
			c.make(t, r, x, store)
			stopSigning(t, x)
			if top, common := gitRepoCmd(t, x, "rev-parse", "--show-toplevel"), gitRepoCmd(t, x, "rev-parse", "--path-format=absolute", "--git-common-dir"); physical(top) != physical(x) || !oneFolderOnDisk(common, store) {
				t.Fatalf("the control: git takes %q as the top and %q as the git folder", top, common)
			}
			if entries, _ := os.ReadDir(filepath.Dir(store)); len(entries) != 1 {
				t.Fatalf("the control: the store's folder holds more than its .git: %v", entries)
			}
			place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
			if err != nil || place.Repo != physical(x) || place.Worktree == "" {
				t.Fatalf("not hired into x: %+v %v", place, err)
			}
			if got := branches(t, x); !strings.Contains(got, place.Branch) {
				t.Errorf("the hire's branch isn't in x: %q", got)
			}
			if got := r2Registrations(store); len(got) != 1 {
				t.Errorf("the hire's registration isn't in x's git folder: %v", got)
			}
			if common := gitRepoCmd(t, place.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir"); !oneFolderOnDisk(common, store) {
				t.Errorf("the hire's worktree shares %q, not x's git folder", common)
			}
		})
	}
}

// O2: x its own repo, whose core.worktree names one of its parents, where git finds no repo: refused in plain words,
// before anything is made, and none of the clean-up of a git that didn't answer runs.
func TestACoreWorktreeNamingAParentWithNoRepoIsRefusedInPlainWords(t *testing.T) {
	r := newOwnerRig(t)
	x := filepath.Join(r.base, "x")
	gitRepoCmd(t, r.base, "init", "-q", "-b", "main", x)
	gitRepoCmd(t, x, "config", "core.worktree", r.base)
	if top := gitRepoCmd(t, x, "rev-parse", "--show-toplevel"); physical(top) != r.base {
		t.Fatalf("the control: git takes %q as x's top", top)
	}
	if out, err := exec.Command("git", "-C", r.base, "rev-parse", "--show-toplevel").CombinedOutput(); err == nil {
		t.Fatalf("the control: git finds a repo at %s: %s", r.base, out)
	}
	var mu sync.Mutex
	var calls []string
	git := func(dir string, args ...string) (int, string, bool) {
		mu.Lock()
		calls = append(calls, strings.Join(args, " "))
		mu.Unlock()
		return runGit(dir, args...)
	}
	hires := filepath.Join(r.base, "hires")
	place, err := r2HireInto(t, x, hires, git)
	var refused *hireRefusal
	if !errors.As(err, &refused) || refused.Status != 409 || !strings.Contains(refused.Message, "git takes "+r.base+" as the top, and finds no repo there") ||
		strings.Contains(refused.Message, "didn't answer") {
		t.Errorf("hired, or refused in other words: %+v %v", place, err)
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "worktree") || strings.HasPrefix(call, "branch -D") {
			t.Errorf("git was asked to make or clear a worktree: %q", call)
		}
	}
	if _, err := os.Lstat(hires); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder for hires' worktrees was made: %v", err)
	}
}
