//go:build !windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// SA-X6 (deskrun.go) with stand-ins only: real throwaway repos (signing off in their local config only), a hire's
// worktree in the test's own registry, and stand-in "programs" a repo's config names, shell scripts that only leave a
// marker file. The rows: RSBXPLAN P3 (the worktree's .git rewritten to a git folder the hire made) and RSBXPLANb R4
// (the shared config's fsmonitor, hooksPath and a filter), through the core's own fire calls; each with its control,
// the same call as the core made it before SA-X6, which runs the stand-in. The sandbox itself is
// deskrun_darwin_test.go's.

// deskRunRig is an owner's repo with one hire's worktree in the registry.
type deskRunRig struct {
	base, repo, tree, marker string
	hire                     Hire
}

func newDeskRunRig(t *testing.T) *deskRunRig { return newDeskRunRigAt(t, "repo-hires") }

// newDeskRunRigAt is the rig with the hire's worktree in hires, a folder of the test's (a path inside "repo" is in
// the owner's checkout).
func newDeskRunRigAt(t *testing.T, hires string) *deskRunRig {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	base := realPath(t.TempDir())
	r := &deskRunRig{base: base, repo: filepath.Join(base, "repo"), marker: filepath.Join(base, "ran")}
	r.tree = filepath.Join(base, hires, "ada-abcdef")
	if err := os.MkdirAll(r.repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, r.repo, "init", "-q", "-b", "main")
	gitRepoCmd(t, r.repo, "config", "commit.gpgsign", "false")
	writeRepoFile(t, r.repo, "a.txt", "one\n")
	gitRepoCmd(t, r.repo, "add", "a.txt")
	gitRepoCmd(t, r.repo, "commit", "-q", "-m", "first")
	gitRepoCmd(t, r.repo, "worktree", "add", "-q", "-b", "hire/ada-abcdef", r.tree)
	r.hire = Hire{ID: "abcdef123456", Name: "ada", Role: "builder", Folder: keptPath(r.repo), Repo: r.repo, Worktree: keptPath(r.tree),
		Branch: "hire/ada-abcdef", State: "working"}
	keepRegistry(t)
	writeRegistry(t, r.hire)
	return r
}

// keepRegistry puts the registry back as it was when the test ends.
func keepRegistry(t *testing.T) {
	t.Helper()
	was, err := os.ReadFile(registryPath())
	t.Cleanup(func() {
		if err != nil {
			os.Remove(registryPath())
		} else {
			_ = writeRegistryFile(registryPath(), was, 0o600)
		}
	})
}

// standIn is a script that leaves the marker (with its name) and fails: what a hire would have a program run.
func (r *deskRunRig) standIn(t *testing.T, name string) string {
	t.Helper()
	script := filepath.Join(r.base, name+".sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho "+name+" >> '"+r.marker+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

func (r *deskRunRig) ran(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(r.marker)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(r.marker)
	return strings.TrimSpace(string(data))
}

// plainGit is git as the core ran it on a hire's tree before SA-X6 (runGit's own run): the control.
func plainGit(dir string, args ...string) string {
	out, _ := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return string(out)
}

// P3: the hire rewrites its worktree's .git file to point at a git folder it made, whose config names an fsmonitor.
// The core's fire runs nothing of it: its status reads the worktree's real registration, and git refuses to remove a
// worktree whose .git doesn't point back, so it is kept.
func TestAWorktreesRewrittenDotGitRunsNothingForTheCoresFire(t *testing.T) {
	r := newDeskRunRig(t)
	fake := filepath.Join(r.base, "made-by-the-hire")
	if out, err := exec.Command("cp", "-R", filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef"), fake).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(fake, "config.worktree"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, fake, "commondir", filepath.Join(r.repo, ".git")+"\n")
	// The hire's own git folder borrows the repo's objects and refs, with its own config on top.
	gitRepoCmd(t, r.repo, "config", "extensions.worktreeConfig", "true")
	if err := os.WriteFile(filepath.Join(fake, "config.worktree"), []byte("[core]\n\tfsmonitor = "+r.standIn(t, "fsmonitor")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, fake, "HEAD", "ref: refs/heads/main\n") // so a git that read it would say another branch
	writeRepoFile(t, r.tree, ".git", "gitdir: "+fake+"\n")

	if said := plainGit(r.tree, "status", "--porcelain"); !strings.Contains(r.ran(t), "fsmonitor") {
		t.Fatalf("the control: the stand-in fsmonitor didn't run for a plain status, so this row proves nothing: %s", said)
	}
	if message := uncommittedRefusal(r.hire, runGit); message != "" {
		t.Errorf("the fire's check: %q, want a clean worktree", message)
	}
	if removeOwnWorktree(runGit, "", r.hire) {
		t.Error("a worktree whose .git doesn't point back was removed")
	}
	if got := r.ran(t); got != "" {
		t.Errorf("the core's git ran the hire's %s", got)
	}
	if _, err := os.Stat(r.tree); err != nil {
		t.Errorf("the worktree went: %v", err)
	}
	// What the core's git read is the registration's: its branch, not whatever the rewritten .git names.
	if status, out, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); !ok || status != 0 || out != r.hire.Branch {
		t.Errorf("symbolic-ref: %d %q %v", status, out, ok)
	}
}

// R4: the hire writes the shared config from its worktree (fsmonitor, hooksPath with a reference-transaction hook,
// and a filter its .gitattributes names on a file whose stat changed). The core's fire runs none of them, finds the
// tree clean, removes it and its branch. The control, the removal as it was (`worktree remove` without --force in the
// checkout), runs the fsmonitor.
func TestTheSharedConfigAHireWritesRunsNothingForTheCoresFire(t *testing.T) {
	r := newDeskRunRig(t)
	hooks := filepath.Join(r.base, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(r.standIn(t, "hook"), filepath.Join(hooks, "reference-transaction")); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, r.tree, ".gitattributes", "*.txt filter=sneak\n")
	gitRepoCmd(t, r.tree, "add", ".gitattributes")
	gitRepoCmd(t, r.tree, "commit", "-q", "-m", "attributes")
	for _, kv := range [][2]string{{"core.fsmonitor", r.standIn(t, "fsmonitor")}, {"core.hooksPath", hooks},
		{"filter.sneak.clean", r.standIn(t, "filter")}} {
		gitRepoCmd(t, r.tree, "config", kv[0], kv[1])
	}
	at := time.Now()
	touch := func() { // same bytes, a new time: status must read the file again, through its filter
		at = at.Add(3 * time.Second)
		if err := os.Chtimes(filepath.Join(r.tree, "a.txt"), at, at); err != nil {
			t.Fatal(err)
		}
	}
	touch()
	plainGit(r.tree, "status", "--porcelain")
	if got := r.ran(t); !strings.Contains(got, "fsmonitor") || !strings.Contains(got, "filter") {
		t.Fatalf("the control: a plain status ran %q, want the fsmonitor and the filter", got)
	}
	touch()
	if message := uncommittedRefusal(r.hire, runGit); message != "" {
		t.Errorf("the fire's check: %q, want a clean worktree", message)
	}
	touch()
	if !removeOwnWorktree(runGit, "", r.hire) {
		t.Error("the clean worktree was kept")
	}
	if got := r.ran(t); got != "" {
		t.Errorf("the core's fire ran the hire's %s", got)
	}
	if _, err := os.Stat(r.tree); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the worktree is still there: %v", err)
	}
	// Its branch has a commit main hasn't, so `branch -d` keeps it, as it always has; it ran no hook (above).
	// The control for the removal: as it was, `worktree remove` runs its own status, and with it the fsmonitor.
	other := filepath.Join(r.base, "repo-hires", "bob-123456")
	gitRepoCmd(t, r.repo, "-c", "core.hooksPath="+os.DevNull, "worktree", "add", "-q", "-b", "hire/bob-123456", other)
	r.ran(t)
	plainGit(r.repo, "worktree", "remove", other)
	if got := r.ran(t); !strings.Contains(got, "fsmonitor") {
		t.Errorf("the control: the old removal ran %q, want the fsmonitor", got)
	}
}

// Only a hire's own tree is the registry's to name: a folder no hire works in runs as before, a hire's tree its repo
// has no registration for is refused (its fire keeps it), and a worktree with submodules is kept, as git keeps it.
func TestTheRegistryDecidesWhichTreesAreADesksAndAnUnknownOneIsRefused(t *testing.T) {
	r := newDeskRunRig(t)
	if _, mine := deskGitFor(r.repo, []string{"status"}); mine {
		t.Error("the owner's checkout was taken for a desk's tree")
	}
	if _, mine := deskGitFor(r.repo, []string{"worktree", "remove", "--force", r.tree}); !mine {
		t.Error("the fire's removal of the hire's worktree isn't run as the desk's")
	}
	if _, mine := deskGitFor(r.repo, []string{"worktree", "remove", "--force", r.repo}); mine {
		t.Error("another removal was taken for the fire's")
	}
	if _, mine := deskGitFor(filepath.Join(r.tree, "sub"), []string{"status"}); !mine {
		t.Error("a folder inside the hire's worktree isn't the desk's")
	}
	if status, out, ok := runGit(r.repo, "rev-parse", "--abbrev-ref", "HEAD"); !ok || status != 0 || out != "main" {
		t.Errorf("the owner's checkout: %d %q %v", status, out, ok)
	}

	modules := filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef", "modules")
	if err := os.MkdirAll(modules, 0o755); err != nil {
		t.Fatal(err)
	}
	if removeOwnWorktree(runGit, "", r.hire) {
		t.Error("a worktree with submodules was removed")
	}
	os.RemoveAll(modules)

	gitdir := filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef", "gitdir")
	writeRepoFile(t, filepath.Dir(gitdir), "gitdir", filepath.Join(r.base, "elsewhere", ".git")+"\n")
	if _, _, ok := runGit(r.tree, "status", "--porcelain"); ok {
		t.Error("a hire's tree with no registration ran")
	}
	if message := uncommittedRefusal(r.hire, runGit); !strings.Contains(message, "Couldn't tell") {
		t.Errorf("the fire's check: %q", message)
	}
	if removeOwnWorktree(runGit, "", r.hire) {
		t.Error("a hire's tree with no registration was removed")
	}
}

// A repo whose config has a key a read doesn't know isn't run on: fail closed, as gitstate's reads.
func TestADesksTreeWhoseConfigHasAnUnknownKeyIsRefused(t *testing.T) {
	r := newDeskRunRig(t)
	gitRepoCmd(t, r.repo, "config", "uploadpack.packObjectsHook", r.standIn(t, "hook"))
	if _, _, ok := runGit(r.tree, "status", "--porcelain"); ok {
		t.Error("a tree whose config names an unknown program key ran")
	}
	if status, _, ok := runGit(r.tree, "rev-parse", "--show-toplevel"); !ok || status != 0 {
		t.Error("rev-parse, which reads no file, was refused too")
	}
	if got := r.ran(t); got != "" {
		t.Errorf("ran %s", got)
	}
}

// The GM's scope and the demo fence: a hire's worktree is held through deskRun, and a worktree a desk made with git
// itself (not in the registry) through gitstate's hardened read, which runs nothing its config names.
func TestTheScopesGitHoldsAHiresWorktreeAndRunsNothingAnywhere(t *testing.T) {
	r := newDeskRunRig(t)
	made := filepath.Join(r.base, "made")
	gitRepoCmd(t, r.repo, "worktree", "add", "-q", "--detach", made)
	gitRepoCmd(t, r.repo, "config", "core.fsmonitor", r.standIn(t, "fsmonitor"))
	scope := gmScope{root: r.repo, git: scopeGit}
	if !scope.holds(r.tree) {
		t.Error("the hire's worktree isn't held")
	}
	if !scope.holds(made) {
		t.Error("a worktree of the project's made by hand isn't held")
	}
	if !gmScopeOf(Hire{Scope: r.repo}).holds(r.tree) {
		t.Error("gmScopeOf's git doesn't hold it")
	}
	if got := r.ran(t); got != "" {
		t.Errorf("ran %s", got)
	}
}

// gh's run for a hire's branch is that hire's (ghDesk), and none for a folder no hire works in.
func TestTheBadgesGhRunIsForTheHireWhoseWorktreeItIs(t *testing.T) {
	r := newDeskRunRig(t)
	if got, _ := withGHDesk(context.Background(), r.tree).Value(ghDesk{}).(string); got != r.hire.ID {
		t.Errorf("the hire's worktree: %q", got)
	}
	if got := withGHDesk(context.Background(), r.repo).Value(ghDesk{}); got != nil {
		t.Errorf("the owner's checkout: %v", got)
	}
}

// The checkpoint store runs only reads in a desk's repo; anything else is refused before git starts (CK1's pin keeps
// every write the store's own).
func TestTheCheckpointStoreRunsOnlyReadsInADesksRepo(t *testing.T) {
	for args, want := range map[string]bool{"rev-parse --git-dir": true, "config --list -z": true, "for-each-ref refs/x": true,
		"diff-tree -r a b": true, "config core.fsmonitor x": false, "update-ref refs/x y": false, "add -A": false,
		"worktree remove x": false, "": false} {
		if got := cpRepoReads(strings.Fields(args)); got != want {
			t.Errorf("%q: %v, want %v", args, got, want)
		}
	}
	r := &cpRepo{dir: t.TempDir()}
	if _, _, err := r.git(context.Background(), nil, "update-ref", "refs/smolcorp/x", "HEAD"); !errors.Is(err, errCpRepoWrite) {
		t.Errorf("a write in the repo: %v", err)
	}
}

// RT126SAX6 B1: a desk process that outlived its session (it left tmux, so the fire didn't end it) swaps a link in
// for a folder of its tree while the fire deletes the tree: two renames, the folder out of the way and a link at a
// stand-in fence (a folder outside the tree with the same names) in its place. The fire deletes nothing outside the
// tree. The control, git's own removal as SA-X6 first ran it (`worktree remove --force`), deletes through the link:
// git walks the tree by path.
func TestTheFiresRemovalDeletesNothingALinkSwappedInLeadsTo(t *testing.T) {
	r := newDeskRunRig(t)
	const files = 20000
	fence := filepath.Join(r.base, "fence")
	fill := func(folder string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(folder, "big"), 0o755); err != nil {
			t.Fatal(err)
		}
		for i := range files {
			if err := os.WriteFile(filepath.Join(folder, "big", fmt.Sprintf("f%05d", i)), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	left := func() int {
		entries, _ := os.ReadDir(filepath.Join(fence, "big"))
		return len(entries)
	}
	// A tree whose d/ is ignored (so the fire finds it clean), full of files.
	plant := func(tree string) {
		t.Helper()
		writeRepoFile(t, tree, ".gitignore", "d/\n")
		gitRepoCmd(t, tree, "add", ".gitignore")
		gitRepoCmd(t, tree, "commit", "-q", "-m", "ignore d")
		fill(filepath.Join(tree, "d"))
	}
	// race swaps d for the link in whichever folder of parent holds d/big once its deletion has begun (big's time
	// changes): the tree where it is, or wherever the core moved it.
	race := func(parent string) (swapped func() bool) {
		t.Helper()
		was := map[uint64]time.Time{}
		look := func(record bool) bool {
			entries, _ := os.ReadDir(parent)
			for _, e := range entries {
				d := filepath.Join(parent, e.Name(), "d")
				info, err := os.Lstat(filepath.Join(d, "big"))
				if err != nil || !info.IsDir() {
					continue
				}
				ino := info.Sys().(*syscall.Stat_t).Ino
				if record {
					was[ino] = info.ModTime()
					continue
				}
				if at, known := was[ino]; known && !info.ModTime().Equal(at) {
					_ = os.Rename(d, d+".moved")
					_ = os.Symlink(fence, d)
					return true
				}
			}
			return false
		}
		look(true)
		done := make(chan bool, 1)
		stop := make(chan struct{})
		go func() {
			for {
				select {
				case <-stop:
					done <- false
					return
				default:
				}
				if look(false) {
					done <- true
					return
				}
			}
		}()
		return func() bool { close(stop); return <-done }
	}

	// The control, up to three tries (it is a race): git's removal deletes fence files through the link.
	fill(fence)
	other := filepath.Join(r.base, "control-hires", "bob-123456")
	gitRepoCmd(t, r.repo, "-c", "core.hooksPath="+os.DevNull, "worktree", "add", "-q", "-b", "hire/bob-123456", other)
	plant(other)
	for try := 0; left() == files; try++ {
		if try == 3 {
			t.Fatal("the control: git's removal never deleted through the link, so this row proves nothing")
		}
		if try > 0 {
			os.RemoveAll(other)
			gitRepoCmd(t, r.repo, "worktree", "prune")
			gitRepoCmd(t, r.repo, "-c", "core.hooksPath="+os.DevNull, "worktree", "add", "-q", other, "hire/bob-123456")
			fill(filepath.Join(other, "d"))
		}
		swapped := race(filepath.Dir(other))
		plainGit(r.repo, "worktree", "remove", "--force", other)
		swapped()
	}
	t.Logf("the control: git's removal left %d of %d fence files", left(), files)
	os.RemoveAll(fence)
	fill(fence)

	// The fire.
	plant(r.tree)
	swapped := race(filepath.Dir(r.tree))
	removeOwnWorktree(runGit, "", r.hire)
	if !swapped() {
		t.Fatal("no link was swapped in while the tree was deleted, so the race wasn't run")
	}
	if n := left(); n != files {
		t.Errorf("the fire deleted %d fence files outside the hire's tree", files-n)
	}
}

// RT126SAX6 N7: work the hire adds after the fire's first look (a process that outlived its session, writing just
// before the removal) is kept: the removal looks again once the tree is moved aside, where nothing a desk runs can
// write, and puts it back. As SA-X6 first had it, the forced removal deleted it.
func TestWorkAHireAddsAfterTheFiresFirstLookIsKept(t *testing.T) {
	r := newDeskRunRig(t)
	late := func(dir string, args ...string) (int, string, bool) {
		if len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
			writeRepoFile(t, r.tree, "late.txt", "the hire's last words\n")
		}
		return runGit(dir, args...)
	}
	if removeOwnWorktree(late, "", r.hire) {
		t.Error("the worktree was removed with the hire's late work in it")
	}
	if data, err := os.ReadFile(filepath.Join(r.tree, "late.txt")); err != nil || string(data) != "the hire's last words\n" {
		t.Errorf("the late work: %q %v", data, err)
	}
	if status, out, ok := runGit(r.tree, "rev-parse", "--abbrev-ref", "HEAD"); !ok || status != 0 || out != r.hire.Branch {
		t.Errorf("the tree put back isn't the hire's worktree: %d %q %v", status, out, ok)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(r.tree), ".*")); len(left) != 0 {
		t.Errorf("left beside it: %v", left)
	}
	// Once the work is gone, the fire removes the tree and its registration.
	os.Remove(filepath.Join(r.tree, "late.txt"))
	if !removeOwnWorktree(runGit, "", r.hire) {
		t.Error("the clean worktree was kept")
	}
	for _, gone := range []string{r.tree, filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef")} {
		if _, err := os.Lstat(gone); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is still there: %v", gone, err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(r.tree), ".*")); len(left) != 0 {
		t.Errorf("left beside it: %v", left)
	}
	if out := gitRepoCmd(t, r.repo, "worktree", "list", "--porcelain"); strings.Contains(out, "ada-abcdef") {
		t.Errorf("git still lists it: %s", out)
	}
}

// RT126SAX6 B3: a hire's worktree inside the owner's checkout (a configured hires folder there) is a desk's tree as
// any other: the fire's check, the GM's scope and gh's badge run as the desk's, so the shared config's fsmonitor and
// a filter run nothing. A call in the checkout itself is still not the desk's, and a registry row whose tree holds its
// checkout has its tree's git refused, never run raw.
func TestAHiresTreeInsideTheOwnersCheckoutIsRunAsTheDesks(t *testing.T) {
	r := newDeskRunRigAt(t, filepath.Join("repo", "hires"))
	writeRepoFile(t, r.tree, ".gitattributes", "*.txt filter=sneak\n")
	gitRepoCmd(t, r.tree, "add", ".gitattributes")
	gitRepoCmd(t, r.tree, "commit", "-q", "-m", "attributes")
	gitRepoCmd(t, r.tree, "config", "core.fsmonitor", r.standIn(t, "fsmonitor"))
	gitRepoCmd(t, r.tree, "config", "filter.sneak.clean", r.standIn(t, "filter"))
	at := time.Now()
	touch := func() {
		at = at.Add(3 * time.Second)
		if err := os.Chtimes(filepath.Join(r.tree, "a.txt"), at, at); err != nil {
			t.Fatal(err)
		}
	}
	touch()
	plainGit(r.tree, "status", "--porcelain")
	if got := r.ran(t); !strings.Contains(got, "fsmonitor") || !strings.Contains(got, "filter") {
		t.Fatalf("the control: a plain status ran %q", got)
	}
	if _, mine := deskGitFor(r.tree, []string{"status"}); !mine {
		t.Error("the hire's tree inside the checkout isn't run as the desk's")
	}
	touch()
	if message := uncommittedRefusal(r.hire, runGit); message != "" {
		t.Errorf("the fire's check: %q", message)
	}
	touch()
	if !(gmScope{root: r.repo, git: scopeGit}).holds(r.tree) {
		t.Error("the GM's scope doesn't hold it")
	}
	if got, _ := withGHDesk(context.Background(), r.tree).Value(ghDesk{}).(string); got != r.hire.ID {
		t.Errorf("gh's desk: %q", got)
	}
	if got := r.ran(t); got != "" {
		t.Errorf("the core's git ran the hire's %s", got)
	}
	if _, mine := deskGitFor(r.repo, []string{"status"}); mine {
		t.Error("the owner's checkout was taken for the desk's tree")
	}

	holder := Hire{ID: "fedcba654321", Name: "cy", Role: "builder", Folder: keptPath(r.repo), Repo: r.repo, Worktree: keptPath(r.base), Branch: "hire/cy", State: "working"}
	writeRegistry(t, r.hire, holder)
	beside := filepath.Join(r.base, "beside")
	if err := os.MkdirAll(beside, 0o755); err != nil {
		t.Fatal(err)
	}
	if run, mine := deskGitFor(beside, []string{"status"}); !mine {
		t.Error("a folder of a tree that holds its checkout ran raw")
	} else if _, _, ok := run(); ok {
		t.Error("a folder of a tree that holds its checkout ran")
	}
	if _, mine := deskGitFor(r.repo, []string{"status"}); mine {
		t.Error("the owner's checkout was taken for the holder's tree")
	}
}

// RT126SAX6 N1: the core's status on a desk's tree never goes into a submodule, whose config (in the desk's own
// registration, worktrees/<id>/modules) the desk writes: a filter planted there runs nothing at the fire, and the
// tree, which has a submodule, is kept. The control, a plain status, runs it.
func TestTheDesksStatusNeverGoesIntoASubmodule(t *testing.T) {
	r := newDeskRunRig(t)
	upstream := filepath.Join(r.base, "upstream")
	if err := os.MkdirAll(upstream, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, upstream, "init", "-q", "-b", "main")
	gitRepoCmd(t, upstream, "config", "commit.gpgsign", "false")
	writeRepoFile(t, upstream, "x.txt", "x\n")
	gitRepoCmd(t, upstream, "add", "x.txt")
	gitRepoCmd(t, upstream, "commit", "-q", "-m", "x")
	gitRepoCmd(t, r.tree, "-c", "protocol.file.allow=always", "submodule", "add", "-q", upstream, "sub")
	gitRepoCmd(t, r.tree, "commit", "-q", "-m", "sub")
	modules := filepath.Join(r.repo, ".git", "worktrees", "ada-abcdef", "modules", "sub")
	if _, err := os.Stat(filepath.Join(modules, "config")); err != nil {
		t.Fatalf("the submodule's git folder isn't in the worktree's registration: %v", err)
	}
	gitRepoCmd(t, filepath.Join(r.tree, "sub"), "config", "filter.subf.clean", r.standIn(t, "subfilter"))
	if err := os.MkdirAll(filepath.Join(modules, "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modules, "info", "attributes"), []byte("*.txt filter=subf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	touch := func() {
		at = at.Add(3 * time.Second)
		if err := os.Chtimes(filepath.Join(r.tree, "sub", "x.txt"), at, at); err != nil {
			t.Fatal(err)
		}
	}
	touch()
	plainGit(r.tree, "status", "--porcelain")
	if got := r.ran(t); !strings.Contains(got, "subfilter") {
		t.Fatalf("the control: a plain status ran %q, want the submodule's filter", got)
	}
	touch()
	if message := uncommittedRefusal(r.hire, runGit); message != "" {
		t.Errorf("the fire's check: %q", message)
	}
	touch()
	if removeOwnWorktree(runGit, "", r.hire) {
		t.Error("a worktree with a submodule was removed")
	}
	touch()
	if status, out, ok := runGit(r.tree, "status", "--porcelain", "--ignore-submodules=none"); !ok || status != 0 || out != "" {
		t.Errorf("a status asking for the submodules: %d %q %v", status, out, ok)
	}
	if got := r.ran(t); got != "" {
		t.Errorf("the core's git ran the desk's %s", got)
	}
	if _, err := os.Stat(r.tree); err != nil {
		t.Errorf("the worktree went: %v", err)
	}
}

// RT126SAX6 N2: what the core runs for a desk is given what a desk's start is (hireEnvironment), never the core's
// whole environment: no GM token, no session. A run for no desk is given env as it is.
func TestADesksRunIsGivenTheHiresEnvironment(t *testing.T) {
	r := newDeskRunRig(t)
	t.Setenv("SMOLCORP_GM_TOKEN", "fake-gm-token")
	env := func(id string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var out strings.Builder
		if err := deskRun(ctx, id, r.base, os.Environ(), &out, &out, "/usr/bin/env"); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if !strings.Contains(env(""), "SMOLCORP_GM_TOKEN=fake-gm-token") {
		t.Fatal("the control: a run for no desk wasn't given the token, so this row proves nothing")
	}
	if got := env(r.hire.ID); strings.Contains(got, "SMOLCORP_GM_TOKEN") {
		t.Errorf("a desk's run was given the GM's token")
	}
}
