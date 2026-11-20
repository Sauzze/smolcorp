package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R5 (RT126FIXREVIEW-GITROWS2R4's N1, N2 and "Seen outside") under the real profile, with newRowsDesk's
// stand-ins: the hire h1 on hire/h1/work, another desk's worktree h2 on hire/h2/work, the owner's checkout on feature.
// These rows name no API of this round's, so they run on the tree before it too.

// N1 (SHADOW) and refs/stash: no desk writes a ref outside its own branches and the remote-tracking ones, by git or by
// a stand-in. refs/feature and refs/hire/h2/work, which the owner's `git log feature` and `git merge hire/h2/work` take
// before the branch itself; refs/replace, which every commit reads as for the owner; refs/notes; refs/stash, which every
// worktree shares, so the owner's `git stash pop` would apply the desk's stash; and any new folder. The owner's git
// then reads each name as it was, with no "ambiguous" warning.
func TestOnAMacADeskWritesNoRefButItsOwnBranchesAndTheRemoteTrackingOnes(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	mainID := strings.TrimSpace(owner(d.checkout, "rev-parse", "main"))
	feature, h2 := owner(d.checkout, "rev-parse", "feature"), owner(d.checkout, "rev-parse", "hire/h2/work")
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "the desk's"); r.status != 0 {
		t.Fatalf("the desk's commit on its own branch: %+v", r)
	}
	for _, ref := range []string{"refs/feature", "refs/hire/h2/work", "refs/replace/" + mainID, "refs/notes/commits", "refs/stash"} {
		if r := d.deskGit(t, git, "update-ref", ref, "HEAD"); r.status == 0 {
			t.Errorf("the desk's update-ref %s worked", ref)
		}
	}
	mustWrite(t, filepath.Join(d.root, "README"), "the desk's work in progress") // a tracked file: git stash has something to save
	for _, args := range [][]string{{"stash", "-q"}, {"notes", "add", "-m", "the desk's note", "main"}, {"replace", "-f", mainID, "HEAD"}} {
		if r := d.deskGit(t, git, args...); r.status == 0 {
			t.Errorf("the desk's git %q worked", args)
		}
	}
	refs, logs := filepath.Join(d.git, "refs"), filepath.Join(d.git, "logs", "refs")
	d.probe(t, python, [][]string{{"write", refs + "/feature"}, {"mkdir", refs + "/hire"}, {"write", refs + "/stash"},
		{"write", refs + "/stash.lock"}, {"mkdir", refs + "/notes"}, {"mkdir", refs + "/new"}, {"write", logs + "/stash"},
		{"write", logs + "/.tmp-renamed-log"}}, times(8, "PermissionError"))
	if got := owner(d.checkout, "for-each-ref", "--format=%(refname)", "refs/feature", "refs/hire", "refs/replace", "refs/notes", "refs/stash", "refs/new"); got != "" {
		t.Errorf("refs the desk made: %q", got)
	}
	if got, status := ownerTries(t, d, git, d.checkout, "stash", "list"); status != 0 || got != "" {
		t.Errorf("the owner's stash list: %d %q", status, got)
	}
	for name, was := range map[string]string{"feature": feature, "hire/h2/work": h2} {
		if got, status := ownerTries(t, d, git, d.checkout, "rev-parse", name); status != 0 || got != was {
			t.Errorf("the owner's rev-parse %s: %d %q, was %q", name, status, got, was)
		}
	}
	if got, _ := ownerTries(t, d, git, d.checkout, "log", "-1", "--format=%s", "main"); got != "first\n" {
		t.Errorf("the owner's log of main reads %q", got)
	}
	if data, err := os.ReadFile(filepath.Join(d.root, "README")); err != nil || string(data) != "the desk's work in progress" {
		t.Errorf("the desk's work in progress after its refused stash: %q %v", data, err)
	}
}

// laterWorktrees are N2's LATE case: after h1's start, the owner's `git worktree add ../app-review -b review` and the
// core's worktree for a later hire, h3 on hire/h3/work. Their registrations, by name.
func laterWorktrees(d *escapeDesk, owner func(string, ...string) string) []string {
	owner(d.checkout, "worktree", "add", "-q", "-b", "review", filepath.Join(d.home, "app-review"))
	owner(d.checkout, "worktree", "add", "-q", "-b", "hire/h3/work", filepath.Join(d.home, "app-hires", "h3"))
	return []string{"app-review", "h3"}
}

// N2 (LATE): a worktree registered after the desk's start is held as one there at the start is. The desk can't point
// its HEAD at the desk's own branch, by a write or a `ref:` file renamed onto it, nor write its index or logs; so its
// own commits move only its own branch, and the owner's review checkout stays on review.
func TestOnAMacADeskCantPointALaterWorktreesHEADAtItsOwnBranch(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	later := laterWorktrees(d, owner)
	var steps [][]string
	for i, name := range later {
		reg := filepath.Join(d.git, "worktrees", name)
		ref := filepath.Join(d.root, "ref"+string(rune('0'+i)))
		mustWrite(t, ref, "ref: refs/heads/hire/h1/work\n")
		steps = append(steps, []string{"rename", ref, reg + "/HEAD"}, []string{"write", reg + "/HEAD.lock"},
			[]string{"write", reg + "/index"}, []string{"write", reg + "/logs/HEAD"})
	}
	d.probe(t, python, steps, times(len(steps), "PermissionError"))
	if r := d.deskGit(t, git, "commit", "-q", "--allow-empty", "-m", "desk-made commit"); r.status != 0 {
		t.Fatalf("the desk's commit on its own branch: %+v", r)
	}
	review := filepath.Join(d.home, "app-review")
	if got := owner(review, "rev-parse", "--symbolic-full-name", "HEAD"); got != "refs/heads/review\n" {
		t.Errorf("the owner's review checkout is on %q", got)
	}
	if got, mine := owner(review, "rev-parse", "HEAD"), owner(d.root, "rev-parse", "HEAD"); got == mine {
		t.Errorf("the owner's review checkout's HEAD is at the desk's commit %s", mine)
	}
	if got := owner(filepath.Join(d.home, "app-hires", "h3"), "rev-parse", "--symbolic-full-name", "HEAD"); got != "refs/heads/hire/h3/work\n" {
		t.Errorf("h3's worktree is on %q", got)
	}
	if err := d.prepared(t); err != nil {
		t.Errorf("the next start: %v", err)
	}
}

// N2, a HEAD made outside the sandbox (by a desk under an older profile, say): a later worktree's HEAD pointing at the
// desk's branch refuses the desk's next start, naming it. Once the owner switches that worktree back, the desk starts.
func TestOnAMacALaterWorktreesHEADOnTheDesksBranchStopsTheNextStart(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	laterWorktrees(d, owner)
	head := filepath.Join(d.git, "worktrees", "app-review", "HEAD")
	mustWrite(t, head, "ref: refs/heads/hire/h1/work\n")
	if err := d.prepared(t); err == nil || !strings.Contains(err.Error(), head) {
		t.Errorf("the next start, app-review's HEAD on the desk's branch: %v, want it refused, naming %s", err, head)
	}
	owner(filepath.Join(d.home, "app-review"), "switch", "-q", "-f", "review")
	if err := d.prepared(t); err != nil {
		t.Errorf("the start once the owner switched app-review back: %v", err)
	}
}

// The controls beside the wider allowlist: a desk's commit, amend, a rebase of its own branch (with -r, whose labels git
// keeps in the desk's own registration), a fetch, a merge of main into its branch, a bisect (its refs are in its
// registration too), `gc --auto`, and a branch of its own made and deleted. Its work in progress is saved as a commit,
// since `git stash` is refused. Then the next start goes on.
func TestOnAMacADesksOwnWorkRunsBesideTheWholeRefsAllowlist(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	up := filepath.Join(d.home, "up.git")
	owner(d.home, "clone", "-q", "--bare", d.checkout, up)
	owner(d.checkout, "remote", "add", "origin", up)
	owner(d.checkout, "commit", "-q", "--allow-empty", "-m", "the owner's next")
	owner(d.checkout, "branch", "-f", "main", "feature")
	owner(d.checkout, "push", "-q", "origin", "main")
	laterWorktrees(d, owner)
	hireGit := func(args ...string) ran {
		t.Helper()
		r := d.deskGit(t, git, args...)
		if r.status != 0 {
			t.Errorf("the desk's git %q: %+v", args, r)
		}
		return r
	}
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work")
	hireGit("add", "work.go")
	hireGit("commit", "-q", "-m", "the desk's work")
	hireGit("commit", "-q", "--amend", "-m", "the desk's work, amended")
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work // in progress")
	if r := d.deskGit(t, git, "stash", "-q"); r.status == 0 {
		t.Errorf("the desk's git stash worked")
	}
	hireGit("commit", "-q", "-am", "work in progress") // what its start words say to do instead
	hireGit("fetch", "-q", "origin")
	hireGit("merge", "-q", "--no-edit", "origin/main")
	hireGit("merge", "-q", "--no-edit", "main")
	hireGit("rebase", "-q", "main")
	hireGit("rebase", "-q", "-r", "main")
	hireGit("bisect", "start", "HEAD", "main")
	hireGit("bisect", "good")
	hireGit("bisect", "reset")
	hireGit("branch", "hire/h1/side")
	hireGit("branch", "-D", "hire/h1/side")
	hireGit("gc", "--auto")
	hireGit("status", "--short")
	if got := owner(d.checkout, "for-each-ref", "--format=%(refname)", "refs/bisect", "refs/rewritten", "refs/stash"); got != "" {
		t.Errorf("the shared .git holds the desk's %q", got)
	}
	if got, want := owner(d.checkout, "rev-parse", "main"), owner(d.checkout, "rev-parse", "feature"); got != want {
		t.Errorf("main moved: %s", got)
	}
	if err := d.prepared(t); err != nil {
		t.Errorf("the next start: %v", err)
	}
}
