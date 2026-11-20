//go:build !windows

package desktop

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FXSWEEP3P3R4: round 4 on FXSWEEP3P3, the cold review's B4, B5, N1 and N3. Throwaway repos and stand-ins only: a
// wrapper around the test's own gitRunner, a stand-in git on the test's own PATH that runs the real git and renames a
// folder of the test's temp repo once, and a FIFO the test makes in its own temp repo, fed by a goroutine of the test.
// Q is the rig's repo, nobody hired into; x is a repo of the GM's own beside it.

// r4Fifo puts a FIFO where file was (or where none was), and feeds file's old contents to every reader until stop is
// closed. As the first
// reader opens it, before it is fed, swap runs. Only a FIFO is ever fed: once file's path leads to anything else
// (after a swap, it may lead into Q), the feeding stops, and nothing is written there.
func r4Fifo(t *testing.T, file string, swap func() bool, stop chan struct{}) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err == nil {
		err = os.Remove(file)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(file, 0o644); err != nil {
		t.Fatal(err)
	}
	go func() {
		first := true
		for {
			select {
			case <-stop:
				return
			default:
			}
			if info, err := os.Lstat(file); err != nil || info.Mode()&fs.ModeNamedPipe == 0 {
				return
			}
			f, err := os.OpenFile(file, os.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err != nil { // no reader yet
				time.Sleep(2 * time.Millisecond)
				continue
			}
			if info, err := f.Stat(); err != nil || info.Mode()&fs.ModeNamedPipe == 0 {
				f.Close()
				return
			}
			if first {
				first = false
				swap()
			}
			_ = syscall.SetNonblock(int(f.Fd()), false)
			_, _ = f.Write(data)
			f.Close()
			time.Sleep(5 * time.Millisecond)
		}
	}()
}

// r4SwapFor puts a link to to where at was, keeping at as at.real; false (and said) when it couldn't.
func r4SwapFor(t *testing.T, at, to string) bool {
	if err := os.Rename(at, at+".real"); err != nil {
		t.Errorf("swap: %v", err)
		return false
	}
	if err := os.Symlink(to, at); err != nil {
		t.Errorf("swap: %v", err)
	}
	return true
}

// r4StandInGit puts a stand-in git first on the test's PATH: for a call whose arguments hold match, the first time
// only, it runs before (a shell line), then the real git, then after, and exits with exit ("" for git's own code);
// every other call is the real git's.
func r4StandInGit(t *testing.T, base, match, before, after, exit string) (once string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	bin, once := filepath.Join(base, "bin"), filepath.Join(base, "stand-in-once")
	writeRepoFile(t, base, "stand-in-once", "")
	if exit == "" {
		exit = "$code"
	}
	script := "#!/bin/sh\ncase \" $* \" in *\"" + match + "\"*) if [ -f '" + once + "' ]; then rm -f '" + once + "'\n" +
		before + "\n'" + real + "' \"$@\"\ncode=$?\n" + after + "\nexit " + exit + "\nfi;; esac\nexec '" + real + "' \"$@\"\n"
	writeRepoFile(t, bin, "git", script)
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return once
}

// r4WithSubmodule gives the rig's repo (Q, with a remote) a submodule mod, and makes x, a clone of it whose git folder
// x-git is kept beside it, with its own mod initialised.
func r4WithSubmodule(t *testing.T) (r *ownerRig, x, xGit, qGit string) {
	t.Helper()
	r = newOwnerRig(t)
	r.withRemote(t, "origin")
	gitRepoCmd(t, r.repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", r2Source(t, r.base, "subsource"), "mod")
	gitRepoCmd(t, r.repo, "commit", "-q", "-m", "mod")
	gitRepoCmd(t, r.repo, "push", "-q", "origin", "main")
	qGit = filepath.Join(r.repo, ".git")
	x, xGit = filepath.Join(r.base, "x"), filepath.Join(r.base, "x-git")
	gitRepoCmd(t, r.base, "clone", "-q", "--separate-git-dir", xGit, r.repo, x)
	stopSigning(t, x)
	gitRepoCmd(t, x, "-c", "protocol.file.allow=always", "submodule", "update", "-q", "--init")
	if _, err := os.Stat(filepath.Join(xGit, "modules", "mod", "HEAD")); err != nil {
		t.Fatalf("x's submodule isn't initialised: %v", err)
	}
	return r, x, xGit, qGit
}

// r4CheckedOut says whether the hire's submodule mod is checked out in its worktree.
func r4CheckedOut(place hirePlace) bool {
	_, err := os.Stat(filepath.Join(place.Worktree, "mod", "s.txt"))
	return place.Worktree != "" && err == nil
}

// B4: presetUpstream's key is written through the root the carry kept, and x-git's config read without waiting, so a
// FIFO there can't hold a write while x-git is swapped for a link to Q's .git: nothing lands in Q's config.
func TestAFifoInTheCarriedGitFolderNeverPutsTheUpstreamIntoAnotherRepo(t *testing.T) {
	for _, swap := range []bool{false, true} {
		name := map[bool]string{false: "control: a FIFO at x-git/config, no swap", true: "a FIFO at x-git/config holds git, and x-git is swapped for a link to Q's .git"}[swap]
		t.Run(name, func(t *testing.T) {
			r, x, xGit, qGit := r3SeparateClone(t)
			before := r2Registrations(qGit)
			stop := make(chan struct{})
			defer close(stop)
			armed := false
			git := func(dir string, args ...string) (int, string, bool) {
				if !armed && len(args) > 1 && args[0] == "config" && args[1] == "--local" {
					armed = true
					r4Fifo(t, filepath.Join(xGit, "config"), func() bool { return swap && r4SwapFor(t, xGit, qGit) }, stop)
				}
				return runGit(dir, args...)
			}
			_, _ = r2HireInto(t, x, filepath.Join(r.base, "hires"), git)
			if !armed {
				t.Fatal("presetUpstream never wrote, so this row proves nothing")
			}
			r3Landed(t, qGit, before)
		})
	}
}

// B5 (a Mac): the submodule's git folder is made in the core's own folder and carried into the registration through
// the root the carry kept; the owner's submodule's commondir is read without waiting. So a FIFO there puts nothing
// into Q's own linked worktree's registration, and a stand-in git that swaps the registration for a link to Q's .git
// as git init runs leaves Q's own submodule git folder as it was.
func TestAHiresSubmoduleIsNeverMadeInAnotherReposRegistration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the core initialises a hire's submodules on a Mac only")
	}
	for _, c := range []string{"control: a FIFO at x-git/modules/mod/commondir, no swap", "a FIFO at x-git/modules/mod/commondir holds the core while the registration is swapped", "the registration swapped for a link to Q's .git as git init runs"} {
		t.Run(c, func(t *testing.T) {
			r, x, xGit, qGit := r4WithSubmodule(t)
			gitRepoCmd(t, r.repo, "worktree", "add", "-q", "-b", "w", filepath.Join(r.base, "proj-w"))
			qReg, qMod := filepath.Join(qGit, "worktrees", "proj-w"), filepath.Join(qGit, "modules", "mod")
			qHead, err := os.ReadFile(filepath.Join(qMod, "HEAD"))
			if err != nil {
				t.Fatal(err)
			}
			swapRegistrations := func() bool {
				regs, _ := filepath.Glob(filepath.Join(xGit, "worktrees", "*"))
				for _, reg := range regs {
					r4SwapFor(t, reg, qReg)
				}
				return len(regs) > 0
			}
			stop := make(chan struct{})
			defer close(stop)
			armed, once := false, ""
			if strings.HasPrefix(c, "the registration") {
				swap := "for d in '" + xGit + "'/worktrees/*; do mv \"$d\" \"$d.real\" && ln -s '" + qGit + "' \"$d\"; done"
				once = r4StandInGit(t, r.base, " init -q --template= --separate-git-dir ", swap, "", "")
			}
			git := func(dir string, args ...string) (int, string, bool) {
				if !armed && once == "" && len(args) == 4 && args[0] == "rev-parse" && args[2] == "--verify" && strings.HasPrefix(args[3], "HEAD:") {
					armed = true
					r4Fifo(t, filepath.Join(xGit, "modules", "mod", "commondir"), func() bool { return strings.Contains(c, "swapped") && swapRegistrations() }, stop)
				}
				return runGit(dir, args...)
			}
			place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), git)
			if once != "" {
				if _, err := os.Stat(once); err == nil {
					t.Fatal("the stand-in never swapped, so this row proves nothing")
				}
			} else if !armed {
				t.Fatal("initHireSubmodules never looked, so this row proves nothing")
			}
			if _, err := os.Lstat(filepath.Join(qReg, "modules")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a git folder was made in Q's own registration: %v", err)
			}
			if head, _ := os.ReadFile(filepath.Join(qMod, "HEAD")); string(head) != string(qHead) {
				t.Errorf("Q's own submodule's HEAD was moved: %q, was %q", head, qHead)
			}
			if _, err := os.Lstat(filepath.Join(qMod, "objects", "info", "alternates")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("an alternates file was written into Q's own submodule git folder: %v", err)
			}
			if strings.HasPrefix(c, "control") && (err != nil || !r4CheckedOut(place)) {
				t.Errorf("the control's submodule isn't checked out: %+v %v", place, err)
			}
		})
	}
}

// B5 (a Mac): a failed submodule is undone only through the registration's root, and only what the core carried. A
// stand-in git runs the real checkout, then swaps the registration (or x-git itself) for a link that leads to Q's
// .git, and fails, as git does when its index is moved away: Q's own submodule git folder is still there, with its
// objects.
func TestAFailedHireSubmoduleNeverRemovesAnotherReposGitFolder(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the core initialises a hire's submodules on a Mac only")
	}
	for _, c := range []string{"control: no swap", "the registration swapped for a link to Q's .git during the checkout", "x-git swapped for a link to a folder whose worktrees/<name> links to Q's .git"} {
		t.Run(c, func(t *testing.T) {
			r, x, xGit, qGit := r4WithSubmodule(t)
			qMod := filepath.Join(qGit, "modules", "mod")
			swap := "for d in '" + xGit + "'/worktrees/*; do mv \"$d\" \"$d.real\" && ln -s '" + qGit + "' \"$d\"; done"
			if strings.HasPrefix(c, "x-git") {
				fake := filepath.Join(r.base, "fake")
				swap = "mkdir -p '" + fake + "/worktrees' && for d in '" + xGit + "'/worktrees/*; do ln -s '" + qGit + "' '" + fake + "/worktrees/'\"$(basename \"$d\")\"; done && mv '" + xGit + "' '" + xGit + ".real' && ln -s '" + fake + "' '" + xGit + "'"
			}
			once := ""
			if !strings.HasPrefix(c, "control") {
				once = r4StandInGit(t, r.base, " checkout -q --detach ", "", swap, "1")
			}
			place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
			if once != "" {
				if _, err := os.Stat(once); err == nil {
					t.Fatal("the stand-in never swapped, so this row proves nothing")
				}
			}
			for _, kept := range []string{"HEAD", "objects"} {
				if _, err := os.Stat(filepath.Join(qMod, kept)); err != nil {
					t.Errorf("Q's own submodule git folder lost its %s: %v", kept, err)
				}
			}
			if strings.HasPrefix(c, "control") && (err != nil || !r4CheckedOut(place)) {
				t.Errorf("the control's submodule isn't checked out: %+v %v", place, err)
			}
		})
	}
}

// N1: a link on the way to x's git folder is judged where the disk takes it: "lnk/../.git", lnk a link to proj/sub,
// is proj's .git (itself a link to proj-git, N3's layout), never x's own. Refused, with nothing in proj-git, in each
// spelling.
func TestADotDotAfterALinkOnTheWayIsJudgedWhereItLeads(t *testing.T) {
	for _, c := range []string{"x/.git a link to lnk/../.git", "x/.git's gitdir line <x>/lnk/../.git", "x/.git a link to <proj>/.git"} {
		t.Run(c, func(t *testing.T) {
			r := newOwnerRig(t)
			writeRepoFile(t, r.repo, "sub/s.txt", "s\n")
			gitRepoCmd(t, r.repo, "add", "sub/s.txt")
			gitRepoCmd(t, r.repo, "commit", "-q", "-m", "sub")
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
			if err := os.Symlink(filepath.Join(r.repo, "sub"), filepath.Join(x, "lnk")); err != nil {
				t.Fatal(err)
			}
			var err error
			switch {
			case strings.Contains(c, "gitdir line"):
				writeRepoFile(t, x, ".git", "gitdir: "+filepath.Join(x, "lnk")+"/../.git\n")
			case strings.Contains(c, "lnk/.."):
				err = os.Symlink("lnk/../.git", filepath.Join(x, ".git"))
			default:
				err = os.Symlink(filepath.Join(r.repo, ".git"), filepath.Join(x, ".git"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if common := gitRepoCmd(t, x, "rev-parse", "--path-format=absolute", "--git-common-dir"); !oneFolderOnDisk(common, qGit) {
				t.Fatalf("git doesn't take proj-git as x's git folder (%s), so this row proves nothing", common)
			}
			before := r2Registrations(qGit)
			place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
			if err == nil {
				t.Errorf("hired into x: %+v", place)
			}
			r3Landed(t, qGit, before)
		})
	}
}

// N3: the fire's removal of a deleted hire branch's upstream goes through the root its deletion was carried through,
// never the repo git would find from the folder again. A stand-in git rewrites x's .git to name Q's .git as the core's
// own branch -D runs; Q holds a key of the same name (planted), which stays, and x-git's keys go.
func TestTheFiresUpstreamRemovalNeverRunsInTheRepoTheGitFileNowNames(t *testing.T) {
	r, x, xGit, qGit := r3SeparateClone(t)
	place, err := r2HireInto(t, x, filepath.Join(r.base, "hires"), nil)
	if err != nil || place.Worktree == "" {
		t.Fatalf("not hired into x: %+v %v", place, err)
	}
	branch := place.Branch
	key := "branch." + branch + ".remote"
	if got := gitRepoCmd(t, x, "config", "--get", key); got != "origin" {
		t.Fatalf("presetUpstream didn't set %s in x: %q", key, got)
	}
	gitRepoCmd(t, x, "worktree", "remove", "--force", place.Worktree)
	gitRepoCmd(t, r.repo, "config", key, "origin")
	once := r4StandInGit(t, r.base, " branch -D ", "printf 'gitdir: %s\\n' '"+qGit+"' > '"+filepath.Join(x, ".git")+"'", "", "")
	status, out, err := ownerGitRun(context.Background(), x, []string{"branch", "-D", branch})
	if _, statErr := os.Stat(once); statErr == nil {
		t.Fatal("the stand-in never rewrote x's .git, so this row proves nothing")
	}
	if err != nil || status != 0 {
		t.Fatalf("the branch wasn't deleted: %d %q %v", status, out, err)
	}
	if config, _ := os.ReadFile(filepath.Join(qGit, "config")); !strings.Contains(string(config), branch) {
		t.Errorf("the upstream was removed from Q's config, a repo nobody hired into: %s", config)
	}
	if config, _ := os.ReadFile(filepath.Join(xGit, "config")); strings.Contains(string(config), branch) {
		t.Errorf("the deleted branch's upstream was left in x-git's config: %s", config)
	}
}
