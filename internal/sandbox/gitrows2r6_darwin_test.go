package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R6 (RT126FIXREVIEW-GITROWS2R5's B1 and N2) under the real profile, with newRowsDesk's stand-ins: the hire h1
// on hire/h1/work, another desk's worktree h2 on hire/h2/work, the owner's checkout on feature. These rows name no API
// of this round's, so they run on the tree before it too.

// B1, PERWT and BISECT: a later worktree's HEAD planted outside the sandbox (by a desk under an older profile, say)
// that leads to the desk's branch by way of a ref git reads in that worktree's own registration refuses the desk's next
// start, naming it. Once the owner switches that worktree back, the desk starts.
func TestOnAMacALaterWorktreesHEADByWayOfAPerWorktreeRefStopsTheNextStart(t *testing.T) {
	git, _ := rowsTools(t)
	for _, kind := range []string{"worktree/w", "bisect/bad", "rewritten/x"} {
		t.Run(kind, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			laterWorktrees(d, owner)
			reg, review := filepath.Join(d.git, "worktrees", "app-review"), filepath.Join(d.home, "app-review")
			mustWrite(t, filepath.Join(reg, "refs", filepath.FromSlash(kind)), "ref: refs/heads/hire/h1/work\n")
			head := filepath.Join(reg, "HEAD")
			mustWrite(t, head, "ref: refs/"+kind+"\n")
			if got := owner(review, "rev-parse", "--symbolic-full-name", "HEAD"); got != "refs/heads/hire/h1/work\n" {
				t.Fatalf("the stand-in: git reads app-review's HEAD as %q", got)
			}
			if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), head) {
				t.Errorf("the next start, app-review's HEAD by way of refs/%s: %v, want it refused, naming %s", kind, err, head)
			}
			owner(review, "switch", "-q", "-f", "review")
			if err := d.prepared(t); err != nil {
				t.Errorf("the start once the owner switched app-review back: %v", err)
			}
		})
	}
}

// B1, HARDWT and OWNERHARD: a HEAD the desk can't write, a later worktree's or the owner checkout's, that has a second
// name in the desk's worktree refuses the desk's next start, naming it: through that name the desk would rewrite the
// HEAD in place, whatever text it has now. Once the owner gives it a copy of its own, the desk starts.
func TestOnAMacAHEADWithASecondNameStopsTheNextStart(t *testing.T) {
	git, _ := rowsTools(t)
	for _, c := range []struct{ name, head, text string }{
		{"a later worktree's", "worktrees/app-review/HEAD", "ref: refs/heads/review\n"},
		{"the owner checkout's", "HEAD", "ref: refs/heads/feature\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, owner := newRowsDesk(t, git)
			laterWorktrees(d, owner)
			head := filepath.Join(d.git, filepath.FromSlash(c.head))
			second := filepath.Join(d.root, "notes.txt")
			mustWrite(t, second, c.text)
			if err := os.Remove(head); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(second, head); err != nil {
				t.Fatal(err)
			}
			if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), head) {
				t.Errorf("the next start, %s HEAD a hard link: %v, want it refused, naming %s", c.name, err, head)
			}
			copied := head + ".copy"
			mustWrite(t, copied, c.text)
			if err := os.Rename(copied, head); err != nil {
				t.Fatal(err)
			}
			if err := d.prepared(t); err != nil {
				t.Errorf("the start once the owner gave it a copy of its own: %v", err)
			}
		})
	}
}

// N2 (TOPSHADOW): no desk writes a name at the top of the shared .git that its own work doesn't need: `$G/feature` and
// `$G/hire/h2/work`, which the owner's `git log feature` and `git merge hire/h2/work` take before the branches, a
// pseudo-ref, gc.pid, a new folder, and anything in logs/ but logs/refs. The owner's git then reads each name as it
// was, with no "ambiguous" warning. The controls: packed-refs.lock, git-lfs's store, its own registration, and its
// work with a remote (commit, push, fetch, pull --rebase, merge, gc --auto), then the next start.
func TestOnAMacADeskWritesNoNameAtTheTopOfTheSharedGitButWhatItsWorkNeeds(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	feature, h2 := owner(d.checkout, "rev-parse", "feature"), owner(d.checkout, "rev-parse", "hire/h2/work")
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
		t.Fatalf("the desk's commit on its own branch: %+v", r)
	}
	g := d.git
	mustWrite(t, filepath.Join(d.root, "side", "work"), "the desk's commit id")
	denied := [][]string{{"write", g + "/feature"}, {"write", g + "/feature.lock"}, {"mkdir", g + "/hire"},
		{"rename", d.root + "/side", g + "/hire"}, {"write", g + "/FEATURE_HEAD"}, {"write", g + "/FETCH_HEAD"},
		{"write", g + "/gc.pid"}, {"write", g + "/description"}, {"mkdir", g + "/new"}, {"mkdir", g + "/logs/new"},
		{"write", g + "/logs/feature"}}
	// $G/lfs itself is made by the core before the start since FXGITROWS2R9 (O2): the desk makes what is in it.
	allowed := [][]string{{"write", g + "/packed-refs.lock"}, {"unlink", g + "/packed-refs.lock"},
		{"mkdir", g + "/lfs/objects"}, {"write", g + "/lfs/objects/standin"}, {"write", g + "/worktrees/h1/FETCH_HEAD"}}
	d.probe(t, python, append(denied, allowed...), append(times(len(denied), "PermissionError"), times(len(allowed), "ok")...))
	for _, name := range []string{"feature", "feature.lock", "hire", "FEATURE_HEAD", "FETCH_HEAD", "gc.pid", "new", "logs/new", "logs/feature"} {
		if _, err := os.Lstat(filepath.Join(g, filepath.FromSlash(name))); err == nil {
			t.Errorf("$G/%s was made", name)
		}
	}
	for name, was := range map[string]string{"feature": feature, "hire/h2/work": h2} {
		if got, status := ownerTries(t, d, git, d.checkout, "rev-parse", name); status != 0 || got != was {
			t.Errorf("the owner's rev-parse %s: %d %q, was %q", name, status, got, was)
		}
	}

	// The controls: its work with a remote.
	up := filepath.Join(d.home, "up.git")
	owner(d.home, "clone", "-q", "--bare", d.checkout, up)
	owner(d.checkout, "remote", "add", "origin", up)
	owner(d.checkout, "config", "branch.hire/h1/work.remote", "origin")
	owner(d.checkout, "config", "branch.hire/h1/work.merge", "refs/heads/hire/h1/work")
	hireGit := func(args ...string) {
		t.Helper()
		if r := d.deskGit(t, git, args...); r.status != 0 {
			t.Errorf("the desk's git %q: %+v", args, r)
		}
	}
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work")
	hireGit("add", "work.go")
	hireGit("commit", "-q", "-m", "the desk's work")
	hireGit("push", "-q", "origin", "hire/h1/work")
	hireGit("fetch", "-q", "origin")
	hireGit("pull", "-q", "--rebase")
	hireGit("merge", "-q", "--no-edit", "origin/main")
	hireGit("gc", "--auto")
	if got, want := owner(d.home, "--git-dir", up, "rev-parse", "hire/h1/work"), owner(d.checkout, "rev-parse", "hire/h1/work"); got != want {
		t.Errorf("the remote's hire/h1/work is %s, the desk's %s", got, want)
	}
	if err := d.prepared(t); err != nil {
		t.Errorf("the next start: %v", err)
	}
}

// N2, a name made before rowGitTop (by a desk under an older profile, say): `$G/feature` or `$G/hire/h2/work` that is a
// symbolic ref to the desk's branch, which the owner's git takes for that branch and follows as the desk commits,
// refuses the desk's next start, naming it; removed, the desk starts. An owner's commit message that starts "ref:"
// is no ref to git, and passes.
func TestOnAMacASymbolicRefAtTheTopOfTheSharedGitStopsTheNextStart(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
		t.Fatalf("the desk's commit on its own branch: %+v", r)
	}
	mine := strings.TrimSpace(owner(d.root, "rev-parse", "HEAD"))
	for _, name := range []string{"feature", "hire/h2/work"} {
		at := filepath.Join(d.git, filepath.FromSlash(name))
		mustWrite(t, at, "ref: refs/heads/hire/h1/work\n")
		if got, _ := ownerTries(t, d, git, d.checkout, "rev-parse", name); !strings.Contains(got, mine) {
			t.Fatalf("the stand-in: the owner's rev-parse %s is %q, not the desk's %s", name, got, mine)
		}
		if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), at) {
			t.Errorf("the next start, $G/%s on the desk's branch: %v, want it refused, naming %s", name, err, at)
		}
		if err := os.RemoveAll(filepath.Join(d.git, strings.Split(name, "/")[0])); err != nil {
			t.Fatal(err)
		}
		if err := d.prepared(t); err != nil {
			t.Errorf("the start once $G/%s was removed: %v", name, err)
		}
	}
	mustWrite(t, filepath.Join(d.git, "COMMIT_EDITMSG"), "ref: #12 the owner's fix\n")
	if err := d.prepared(t); err != nil {
		t.Errorf("the start with an owner's commit message that starts \"ref:\": %v", err)
	}
}
