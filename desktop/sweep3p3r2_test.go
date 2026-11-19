//go:build !windows

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXSWEEP3P3R2: round 2 on FXSWEEP3P3, the cold review's B1, N1 and N2, and the ruled .git link. Throwaway repos and
// stand-in folders only.

// r2HireInto asks prepareHire for a hire's worktree in folder, with git as given (runGit when nil), its worktrees
// beside the repo (`<repo>-hires`) when worktrees is "".
func r2HireInto(t *testing.T, folder, worktrees string, git gitRunner) (hirePlace, error) {
	t.Helper()
	keepRegistry(t)
	o := hirePlaceOptions{Worktrees: worktrees, PortFree: func(int) bool { return true }, Git: git}
	return prepareHire(hireRequest{HireID: "abcdef123456", Name: "Ada", Folder: folder, Worktree: true}, o)
}

// r2Registrations is the names in a git folder's worktrees folder.
func r2Registrations(gitDir string) []string {
	var names []string
	entries, _ := os.ReadDir(filepath.Join(gitDir, "worktrees"))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// r2Source is a repo with one commit, to add as a submodule.
func r2Source(t *testing.T, base, name string) string {
	t.Helper()
	source := filepath.Join(base, name)
	writeRepoFile(t, source, "s.txt", "s\n")
	gitRepoCmd(t, source, "init", "-q", "-b", "main")
	stopSigning(t, source)
	gitRepoCmd(t, source, "add", "s.txt")
	gitRepoCmd(t, source, "commit", "-q", "-m", "s")
	return source
}

// r2Untouched says what a refused hire wrote into the other repo, whose top is top and git folder gitDir: a hire's
// branch, a registration not in before, a hire's branch in its config, or a folder for hires beside it.
func r2Untouched(t *testing.T, top, gitDir string, before []string) {
	t.Helper()
	if got := branches(t, top); got != "" {
		t.Errorf("a branch was written into the other repo: %q", got)
	}
	if got := r2Registrations(gitDir); !slices.Equal(got, before) {
		t.Errorf("a registration was written into the other repo: %v, was %v", got, before)
	}
	if config, _ := os.ReadFile(filepath.Join(gitDir, "config")); strings.Contains(string(config), "hire/") {
		t.Errorf("the other repo's config has a hire's branch: %s", config)
	}
	if _, err := os.Lstat(hiresFolder(top, "")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a folder for hires was made beside the other repo: %s (%v)", hiresFolder(top, ""), err)
	}
}

// B1: a .git in the folder named for the hire that has git take a top the folder isn't in (another repo's submodule
// git folder, or any git folder whose core.worktree names its own top) is refused before anything is made, in words
// naming the folder, and nothing is written into the other repo: no branch, no registration, no `<top>-hires`.
// The core's own worktree add from that folder is refused too, as a .git swapped in between the two looks would be.
func TestAFolderWhoseGitTopIsAnotherReposIsNeverHiredInto(t *testing.T) {
	for _, c := range []struct{ name, plant string }{
		{"a .git file naming another repo's submodule git folder", "submodule"},
		{"a .git file naming another repo's .git, whose core.worktree is its own top", "core.worktree"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newOwnerRig(t) // r.repo stands for a repo nobody hired into
			top, gitDir := r.repo, filepath.Join(r.repo, ".git")
			if c.plant == "submodule" {
				gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", r2Source(t, r.base, "subsource"), "mod")
				top, gitDir = filepath.Join(r.repo, "mod"), filepath.Join(r.repo, ".git", "modules", "mod")
			} else {
				gitRepoCmd(t, r.repo, "config", "core.worktree", r.repo)
			}
			x := filepath.Join(r.base, "x")
			writeRepoFile(t, x, ".git", "gitdir: "+gitDir+"\n")
			// The control: from x, git takes the other repo's top as the top.
			if got := gitRepoCmd(t, x, "rev-parse", "--show-toplevel"); physical(got) != physical(top) {
				t.Fatalf("the control: git takes %q as x's top, so this row proves nothing", got)
			}
			before := r2Registrations(gitDir)
			for _, worktrees := range []string{filepath.Join(r.base, "hires"), ""} {
				place, err := r2HireInto(t, x, worktrees, nil)
				var refused *hireRefusal
				if !errors.As(err, &refused) || refused.Status != 409 || !strings.Contains(refused.Message, "Smolcorp didn't hire into "+x+":") {
					t.Errorf("hired into x (worktrees %q): %+v %v", worktrees, place, err)
				}
			}
			if _, err := os.Stat(filepath.Join(r.base, "hires")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the folder for hires' worktrees was made: %v", err)
			}
			status, out, err := ownerGitRun(t.Context(), x, []string{"worktree", "add", "-q", "-b", "hire/ada-abcdef", filepath.Join(r.base, "hires", "x", "ada-abcdef")})
			var no ownerGitNo
			if !errors.As(err, &no) || !strings.Contains(err.Error(), "the top of "+x+", a folder that isn't in it") {
				t.Errorf("the core's worktree add from x: %d %q %v", status, out, err)
			}
			r2Untouched(t, top, gitDir, before)
		})
	}
}

// The ruling on the .git link: a .git that is a link to a folder named .git (in any case) whose parent is another
// work tree is that repo's, and refused; nothing is written into it. The control: git takes x as the top, and the
// other repo's .git as its git folder.
func TestADotGitLinkToAnotherWorkTreesDotGitIsRefused(t *testing.T) {
	for _, spelt := range []string{".git", ".GIT"} {
		t.Run(spelt, func(t *testing.T) {
			r := newOwnerRig(t)
			gitDir := filepath.Join(r.repo, ".git")
			if _, err := os.Stat(filepath.Join(r.repo, spelt)); err != nil {
				t.Skipf("this disk tells %s from .git: %v", spelt, err)
			}
			x := filepath.Join(r.base, "x")
			if err := os.MkdirAll(x, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(r.repo, spelt), filepath.Join(x, ".git")); err != nil {
				t.Fatal(err)
			}
			if top, common := gitRepoCmd(t, x, "rev-parse", "--show-toplevel"), gitRepoCmd(t, x, "rev-parse", "--path-format=absolute", "--git-common-dir"); physical(top) != physical(x) || !oneFolderOnDisk(common, gitDir) {
				t.Fatalf("the control: git takes %q as the top and %q as the git folder, so this row proves nothing", top, common)
			}
			before := r2Registrations(gitDir)
			place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
			var refused *hireRefusal
			if !errors.As(err, &refused) || refused.Status != 409 || !strings.Contains(refused.Message, x+"'s own: it is the .git of another work tree, "+r.repo) {
				t.Errorf("hired into x: %+v %v", place, err)
			}
			r2Untouched(t, r.repo, gitDir, before)
		})
	}
}

// N1: a refusal of the core's reaches whoever hired in its own words, naming the folder, never as "git didn't
// answer"; and the clean-up of a git stopped for taking too long (discardHalfMade) doesn't run, as nothing was made.
// Both the refusal of a .git file naming another repo's .git, and of one swapped in between prepareHire's look and
// the worktree add, which the core's own look catches (another repo's submodule git folder).
func TestTheCoresRefusalOfAWorktreeIsSaidInItsOwnWords(t *testing.T) {
	for _, c := range []struct{ name, plant, words string }{
		{"a .git file naming another repo's .git", "dotgit", "isn't %s's own: it is the .git of another work tree"},
		{"a .git swapped in for another repo's submodule git folder", "swap", "the top of %s, a folder that isn't in it"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newOwnerRig(t)
			x := filepath.Join(r.base, "x")
			top, gitDir := r.repo, filepath.Join(r.repo, ".git")
			swap := func() {}
			if c.plant == "dotgit" {
				writeRepoFile(t, x, ".git", "gitdir: "+gitDir+"\n")
			} else {
				gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", r2Source(t, r.base, "subsource"), "mod")
				top, gitDir = filepath.Join(r.repo, "mod"), filepath.Join(r.repo, ".git", "modules", "mod")
				// x is a plain repo of its own when prepareHire looks, and leads into mod when the core runs git.
				writeRepoFile(t, x, "a.txt", "a\n")
				gitRepoCmd(t, x, "init", "-q", "-b", "main")
				stopSigning(t, x)
				gitRepoCmd(t, x, "add", "a.txt")
				gitRepoCmd(t, x, "commit", "-q", "-m", "a")
				swap = func() {
					if err := os.RemoveAll(filepath.Join(x, ".git")); err != nil {
						t.Fatal(err)
					}
					writeRepoFile(t, x, ".git", "gitdir: "+gitDir+"\n")
				}
			}
			var calls []string
			git := func(dir string, args ...string) (int, string, bool) {
				if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
					swap()
				}
				calls = append(calls, strings.Join(args, " "))
				return runGit(dir, args...)
			}
			before := r2Registrations(gitDir)
			place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), git)
			var refused *hireRefusal
			if !errors.As(err, &refused) || refused.Status != 409 || !strings.Contains(refused.Message, strings.ReplaceAll(c.words, "%s", x)) ||
				strings.Contains(refused.Message, "didn't answer") {
				t.Errorf("hired into x: %+v %v", place, err)
			}
			for _, call := range calls {
				if strings.HasPrefix(call, "worktree remove") || strings.HasPrefix(call, "worktree prune") || strings.HasPrefix(call, "branch -D") {
					t.Errorf("the clean-up of a git that didn't answer ran after a refusal: %q (all: %q)", call, calls)
				}
			}
			r2Untouched(t, top, gitDir, before)
		})
	}
}

// N2 and the controls kept: a repo made with --separate-git-dir (by init or by clone), whose .git file names its own
// git folder kept elsewhere with no core.worktree, gets hires again, its hire's worktree and branch made in it. So do
// a plain repo, a linked worktree, a submodule, a nested submodule and a repo whose .git is a link to its own git
// folder kept elsewhere (not named .git).
func TestNormalReposStillGetHires(t *testing.T) {
	hires := func(t *testing.T, r *ownerRig, folder, gitDir string) {
		t.Helper()
		place, err := r2HireInto(t, folder, filepath.Join(r.base, "hires"), nil)
		if err != nil || place.Worktree == "" {
			t.Fatalf("hired into %s: %+v %v", folder, place, err)
		}
		if got := branches(t, folder); !strings.Contains(got, "ada-abcdef") {
			t.Errorf("its branch isn't in its repo: %q", got)
		}
		if got := r2Registrations(gitDir); !slices.Contains(got, filepath.Base(place.Worktree)) {
			t.Errorf("its worktree isn't registered in its git folder %s: %v", gitDir, got)
		}
		if common := gitRepoCmd(t, place.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir"); !oneFolderOnDisk(common, gitDir) {
			t.Errorf("its worktree's git folder is %q, want %q", common, gitDir)
		}
	}
	separate := func(t *testing.T, r *ownerRig, made func(sep, sepGit string)) {
		sep, sepGit := filepath.Join(r.base, "sep"), filepath.Join(r.base, "sep-git")
		made(sep, sepGit)
		if worktree, _ := exec.Command("git", "config", "--file", filepath.Join(sepGit, "config"), "--get", "core.worktree").Output(); len(worktree) != 0 {
			t.Fatalf("the control: the separate git folder has core.worktree %q", worktree)
		}
		if dotGit, _ := os.ReadFile(filepath.Join(sep, ".git")); !strings.HasPrefix(string(dotGit), "gitdir: ") {
			t.Fatalf("the control: its .git is %q", dotGit)
		}
		hires(t, r, sep, sepGit)
	}
	t.Run("a repo made by git init --separate-git-dir", func(t *testing.T) {
		r := newOwnerRig(t)
		separate(t, r, func(sep, sepGit string) {
			gitRepoCmd(t, r.base, "init", "-q", "-b", "main", "--separate-git-dir", sepGit, sep)
			stopSigning(t, sep)
			writeRepoFile(t, sep, "a.txt", "a\n")
			gitRepoCmd(t, sep, "add", "a.txt")
			gitRepoCmd(t, sep, "commit", "-q", "-m", "a")
		})
	})
	t.Run("a repo made by git clone --separate-git-dir", func(t *testing.T) {
		r := newOwnerRig(t)
		separate(t, r, func(sep, sepGit string) {
			gitRepoCmd(t, r.base, "clone", "-q", "--separate-git-dir", sepGit, r.repo, sep)
		})
	})
	t.Run("a plain repo", func(t *testing.T) {
		r := newOwnerRig(t)
		hires(t, r, r.repo, filepath.Join(r.repo, ".git"))
	})
	t.Run("a linked worktree", func(t *testing.T) {
		r := newOwnerRig(t)
		linked := filepath.Join(r.base, "linked")
		gitRepoCmd(t, r.repo, "worktree", "add", "-q", "-b", "linked", linked)
		hires(t, r, linked, filepath.Join(r.repo, ".git"))
	})
	t.Run("a submodule", func(t *testing.T) {
		r := newOwnerRig(t)
		gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", r2Source(t, r.base, "subsource"), "mod")
		hires(t, r, filepath.Join(r.repo, "mod"), filepath.Join(r.repo, ".git", "modules", "mod"))
	})
	t.Run("a nested submodule", func(t *testing.T) {
		r := newOwnerRig(t)
		outer := r2Source(t, r.base, "outer")
		gitRepoCmd(t, outer, "-c", "protocol.file.allow=always", "submodule", "add", "-q", r2Source(t, r.base, "inner"), "inner")
		gitRepoCmd(t, outer, "commit", "-q", "-m", "inner")
		gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", outer, "mod")
		gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init", "--recursive")
		hires(t, r, filepath.Join(r.repo, "mod", "inner"), filepath.Join(r.repo, ".git", "modules", "mod", "modules", "inner"))
	})
	t.Run("a repo whose .git is a link to its own git folder kept elsewhere", func(t *testing.T) {
		r := newOwnerRig(t)
		real := filepath.Join(r.base, "proj-git")
		if err := os.Rename(filepath.Join(r.repo, ".git"), real); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, filepath.Join(r.repo, ".git")); err != nil {
			t.Fatal(err)
		}
		if top := gitRepoCmd(t, r.repo, "rev-parse", "--show-toplevel"); physical(top) != physical(r.repo) {
			t.Skipf("this git doesn't take a .git link: its top is %q", top)
		}
		hires(t, r, r.repo, real)
	})
}
