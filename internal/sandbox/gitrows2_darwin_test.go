package sandbox

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXGITROWS2 (RT126SWEEP2 part 1: B2, B3, F1, F2, F4) under the real profile (PrepareProfile, sandbox-exec -f), with
// stand-ins only: a repo git made itself in the test's temp folder, whose default branch is develop (origin/HEAD), the
// hire's linked worktree on hire/h1/work and another desk's on hire/h2/work, and the owner's checkout on feature. The
// owner's git runs outside any sandbox (outside, startRefusal first); the desk's inside, from its worktree.

// newRowsDesk is that repo around an escape desk "h1", its profile written for its Places.
func newRowsDesk(t *testing.T, git string) (*escapeDesk, func(dir string, args ...string) string) {
	t.Helper()
	d := newEscapeDesk(t)
	for _, gone := range []string{d.checkout, d.root, filepath.Join(d.home, "app-hires", "h2")} {
		if err := os.RemoveAll(gone); err != nil {
			t.Fatal(err)
		}
	}
	owner := func(dir string, args ...string) string {
		t.Helper()
		return outside(t, dir, rowsEnv(d, git), git, args...)
	}
	if err := os.MkdirAll(d.checkout, 0o700); err != nil {
		t.Fatal(err)
	}
	owner(d.checkout, "init", "-q", "-b", "main")
	owner(d.checkout, "config", "commit.gpgsign", "false")
	mustWrite(t, filepath.Join(d.checkout, "README"), "the owner's checkout")
	owner(d.checkout, "add", "README")
	owner(d.checkout, "commit", "-q", "-m", "first")
	owner(d.checkout, "branch", "develop")
	owner(d.checkout, "update-ref", "refs/remotes/origin/develop", "develop")
	owner(d.checkout, "update-ref", "refs/remotes/origin/main", "main")
	owner(d.checkout, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
	owner(d.checkout, "worktree", "add", "-q", "-b", "hire/h1/work", d.root)
	owner(d.checkout, "worktree", "add", "-q", "-b", "hire/h2/work", filepath.Join(d.home, "app-hires", "h2"))
	owner(d.checkout, "switch", "-q", "-c", "feature")
	mustWrite(t, filepath.Join(d.root, ".claude", "settings.json"), "{}")
	d.pl.DefaultBranch = DefaultBranch(d.git)
	if d.pl.DefaultBranch != "develop" {
		t.Fatalf("the stand-in's default branch: %q", d.pl.DefaultBranch)
	}
	rowsPlaces(&d.pl, "feature")
	d.prepare(t)
	return d, owner
}

func rowsEnv(d *escapeDesk, git string) []string {
	return append([]string{"HOME=" + d.home}, gitTestEnv(git)...)
}

// ownerTries is the owner's git outside any sandbox, as outside, but a git that fails is an answer.
func ownerTries(t *testing.T, d *escapeDesk, git, dir string, args ...string) (string, int) {
	t.Helper()
	if err := startRefusal(git, args, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, git, args...))
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, rowsEnv(d, git), 5*time.Second
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("the owner's git %q: %v", args, err)
	}
	return string(out), 0
}

// probe runs sbProbe's steps as the desk, and fails the test for each answer that isn't the one it wants.
func (d *escapeDesk) probe(t *testing.T, python string, steps [][]string, want []string) {
	t.Helper()
	encoded, _ := json.Marshal(steps)
	r := d.inside(t, []string{"PYTHONDONTWRITEBYTECODE=1"}, python, "-I", "-c", sbProbe, string(encoded))
	var said []string
	if err := json.Unmarshal([]byte(r.stdout), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v) %s", r.stdout, err, r.stderr)
	}
	for i, s := range steps {
		if said[i] != want[i] {
			t.Errorf("the desk's %s %s: %s, want %s", s[0], strings.Join(s[1:], " → "), said[i], want[i])
		}
	}
}

// deskGit is the desk's git in its worktree, with the settings a desk's start gives it (DeskGitConfig): its exit code
// and words.
func (d *escapeDesk) deskGit(t *testing.T, git string, args ...string) ran {
	t.Helper()
	return d.inside(t, DeskGitConfig(gitTestEnv(git)), git, args...)
}

func rowsTools(t *testing.T) (git, python string) {
	t.Helper()
	sandboxExecOrSkip(t)
	git = gitOrSkip(t)
	if python = python3(t); python == "" {
		t.Skip("no python3 here")
	}
	return git, python
}

// times is n of s.
func times(n int, s string) []string { return strings.Split(strings.Repeat(s+"\n", n), "\n")[:n] }

// B2: no desk points origin/HEAD at another branch, by git or by a write, a lock, a rename or a removal, nor moves
// its folder aside, so the next start's DefaultBranch still reads develop. Its fetch into a remote-tracking branch is
// still its work.
func TestOnAMacADeskCantPointOriginHEADAtAnotherBranch(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	if r := d.deskGit(t, git, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"); r.status == 0 {
		t.Errorf("the desk's symbolic-ref of origin/HEAD worked: %+v", r)
	}
	mustWrite(t, filepath.Join(d.root, "head"), "ref: refs/remotes/origin/main\n")
	origin := filepath.Join(d.git, "refs", "remotes", "origin")
	d.probe(t, python, [][]string{{"write", origin + "/HEAD"}, {"write", origin + "/HEAD.lock"}, {"write", origin + "/head.LOCK"},
		{"rename", filepath.Join(d.root, "head"), origin + "/HEAD"}, {"unlink", origin + "/HEAD"},
		{"rename", origin, origin + "-aside"}}, times(6, "PermissionError"))
	if got := DefaultBranch(d.git); got != "develop" {
		t.Errorf("the next start's default branch: %q, want develop", got)
	}
	if r := d.deskGit(t, git, "fetch", "-q", ".", "main:refs/remotes/origin/main"); r.status != 0 {
		t.Errorf("the desk's fetch into origin/main: %+v", r)
	}
	if r := d.deskGit(t, git, "update-ref", "refs/heads/develop", "HEAD"); r.status == 0 {
		t.Errorf("the desk moved develop: %+v", r)
	}
	if got := owner(d.checkout, "rev-parse", "develop"); got != owner(d.checkout, "rev-parse", "main") {
		t.Errorf("develop moved: %s", got)
	}
}

// B3: no desk records a resolution the owner's merge then takes. The verdict's replay: the desk commits a change that
// conflicts with the owner's, makes rr-cache, merges with rerere on, writes its resolution with a line in neither
// side, records it and aborts; the owner's own merge of its branch, with no rerere in its config, must then stop at
// the conflict, with no line of the desk's. Neither rr-cache nor a MERGE_RR, the shared .git's or its own worktree's,
// is the desk's to write, nor their locks; and with the owner's rerere on (rr-cache there), nothing in it.
func TestOnAMacADeskRecordsNoResolutionTheOwnersMergeTakes(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	owner(d.checkout, "merge", "-q", "--no-edit", "hire/h1/work")
	mustWrite(t, filepath.Join(d.checkout, "c.txt"), "base\n")
	owner(d.checkout, "add", "c.txt")
	owner(d.checkout, "commit", "-q", "-m", "base")
	if r := d.deskGit(t, git, "merge", "-q", "--no-edit", "feature"); r.status != 0 {
		t.Fatalf("the desk's merge of feature: %+v", r)
	}
	mustWrite(t, filepath.Join(d.root, "c.txt"), "desk-change\n")
	if r := d.deskGit(t, git, "commit", "-q", "-am", "the desk's"); r.status != 0 {
		t.Fatalf("the desk's commit: %+v", r)
	}
	mustWrite(t, filepath.Join(d.checkout, "c.txt"), "owner-change\n")
	owner(d.checkout, "commit", "-q", "-am", "the owner's")

	rr, wt := filepath.Join(d.git, "rr-cache"), filepath.Join(d.git, "worktrees", "h1")
	mustWrite(t, filepath.Join(d.root, "MERGE_RR"), "")
	if err := os.MkdirAll(filepath.Join(d.root, "rr"), 0o700); err != nil {
		t.Fatal(err)
	}
	d.probe(t, python, [][]string{{"mkdir", rr}, {"mkdir", filepath.Join(d.git, "RR-CACHE")}, {"rename", filepath.Join(d.root, "rr"), rr},
		{"symlink", rr, filepath.Join(d.root, "rr")}, {"write", filepath.Join(d.git, "MERGE_RR")}, {"write", filepath.Join(d.git, "MERGE_RR.lock")},
		{"write", filepath.Join(wt, "MERGE_RR")}, {"write", filepath.Join(wt, "MERGE_RR.lock")},
		{"rename", filepath.Join(d.root, "MERGE_RR"), filepath.Join(wt, "MERGE_RR")}}, times(9, "PermissionError"))
	d.deskGit(t, git, "-c", "rerere.enabled=true", "merge", "--no-edit", "feature")
	mustWrite(t, filepath.Join(d.root, "c.txt"), "desk-change\nSNEAKED-LINE\n")
	d.deskGit(t, git, "-c", "rerere.enabled=true", "rerere")
	d.deskGit(t, git, "merge", "--abort")
	if _, err := os.Lstat(rr); err == nil {
		t.Fatal("rr-cache was made")
	}
	out, status := ownerTries(t, d, git, d.checkout, "merge", "--no-edit", "hire/h1/work")
	data, _ := os.ReadFile(filepath.Join(d.checkout, "c.txt"))
	if status == 0 || strings.Contains(string(data), "SNEAKED-LINE") || !strings.Contains(string(data), "<<<<<<<") || strings.Contains(out, "previous resolution") {
		t.Errorf("the owner's merge (%d, %q) left c.txt %q, want the conflict and no line of the desk's", status, out, data)
	}
	owner(d.checkout, "merge", "--abort")

	// The owner's rerere on: rr-cache is there, the owner's own, and nothing in it is the desk's.
	if err := os.MkdirAll(filepath.Join(rr, "0123"), 0o755); err != nil {
		t.Fatal(err)
	}
	d.probe(t, python, [][]string{{"mkdir", filepath.Join(rr, "abcd")}, {"write", filepath.Join(rr, "0123", "postimage")},
		{"write", filepath.Join(rr, "0123", "preimage")}, {"rename", rr, rr + "-aside"}, {"rmdir", filepath.Join(rr, "0123")}},
		times(5, "PermissionError"))
}

// F1: a desk moves only its own branches, hire/h1/…: not another desk's (hire/h2/work, one hired after it starts,
// any spelling), nor the branch the owner's checkout is on (feature), nor their reflogs, by git or by a write, a lock or
// a rename, with every ref packed or loose, nor a branch of any other name (FXGITROWS2R4). Its own branch, a new one of
// its own and its deletion are still its work.
func TestOnAMacADeskMovesOnlyItsOwnBranches(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work")
	if r := d.deskGit(t, git, "add", "work.go"); r.status != 0 {
		t.Fatal(r)
	}
	if r := d.deskGit(t, git, "commit", "-q", "-m", "the desk's"); r.status != 0 {
		t.Fatalf("the desk's commit on its own branch: %+v", r)
	}
	h2, feature := owner(d.checkout, "rev-parse", "hire/h2/work"), owner(d.checkout, "rev-parse", "feature")
	heads, logs := filepath.Join(d.git, "refs", "heads"), filepath.Join(d.git, "logs", "refs", "heads")
	mustWrite(t, filepath.Join(d.root, "ref"), h2)
	d.probe(t, python, [][]string{{"write", heads + "/hire/h2/work"}, {"write", heads + "/hire/h2/work.lock"}, {"mkdir", heads + "/hire/h3"},
		{"write", heads + "/feature"}, {"write", heads + "/feature.lock"}, {"write", logs + "/hire/h2/work"},
		{"write", logs + "/feature"}, {"write", logs + "/feature.lock"}, {"rename", filepath.Join(d.root, "ref"), heads + "/hire/h2/work"},
		{"rename", heads + "/hire", heads + "/hire-aside"}, {"write", heads + "/Hire/H2/x"}, {"symlink", heads + "/hire/h2/work2", d.root + "/ref"}},
		times(12, "PermissionError"))
	// …and by git, every ref packed.
	owner(d.checkout, "pack-refs", "--all")
	for _, args := range [][]string{{"update-ref", "refs/heads/hire/h2/work", "HEAD"}, {"branch", "-f", "hire/h2/work", "HEAD"},
		{"update-ref", "refs/heads/feature", "HEAD"}, {"branch", "-f", "feature", "HEAD"}, {"branch", "hire/h3/work"},
		{"update-ref", "refs/heads/HIRE/H2/work", "HEAD"}, {"update-ref", "-d", "refs/heads/hire/h2/work"},
		{"reflog", "expire", "--expire=now", "refs/heads/feature"},
		{"branch", "mine"}} { // a branch of any other name, since FXGITROWS2R4 (it was the desk's own work before)
		if r := d.deskGit(t, git, args...); r.status == 0 {
			t.Errorf("the desk's git %q worked: %+v", args, r)
		}
	}
	if got := owner(d.checkout, "rev-parse", "hire/h2/work"); got != h2 {
		t.Errorf("hire/h2/work moved: %s, was %s", got, h2)
	}
	if got := owner(d.checkout, "rev-parse", "feature"); got != feature {
		t.Errorf("feature moved: %s, was %s", got, feature)
	}
	if got := owner(d.checkout, "for-each-ref", "--format=%(refname)", "refs/heads/hire/h3", "refs/heads/HIRE", "refs/heads/mine"); got != "" {
		t.Errorf("the desk made %q", got)
	}
	// Its own work.
	for _, args := range [][]string{{"commit", "-q", "--allow-empty", "-m", "more"}, {"branch", "hire/h1/side"},
		{"update-ref", "refs/heads/hire/h1/side", "HEAD~1"}, {"branch", "-D", "hire/h1/side"}} {
		if r := d.deskGit(t, git, args...); r.status != 0 {
			t.Errorf("the desk's git %q: %+v", args, r)
		}
	}
	d.probe(t, python, [][]string{{"write", logs + "/hire/h1/work.lock"}, {"unlink", logs + "/hire/h1/work.lock"}}, times(2, "ok"))
}

// F2: no desk writes the owner checkout's reflog, logs/HEAD (the owner's `reset --hard HEAD@{1}`, `checkout -`), its
// lock, a file renamed onto it or its removal. Its own worktree's reflog is its own.
func TestOnAMacADeskCantWriteTheOwnerCheckoutsReflog(t *testing.T) {
	git, python := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	before := owner(d.checkout, "rev-parse", "HEAD@{1}")
	logs := filepath.Join(d.git, "logs")
	mustWrite(t, filepath.Join(d.root, "log"), "the desk's\n")
	d.probe(t, python, [][]string{{"write", logs + "/HEAD"}, {"write", logs + "/HEAD.lock"}, {"rename", filepath.Join(d.root, "log"), logs + "/HEAD"},
		{"unlink", logs + "/HEAD"}, {"rename", logs, logs + "-aside"}, {"write", filepath.Join(d.git, "worktrees", "h1", "logs", "HEAD")}},
		append(times(5, "PermissionError"), "ok"))
	if got := owner(d.checkout, "rev-parse", "HEAD@{1}"); got != before {
		t.Errorf("the owner's HEAD@{1}: %s, was %s", got, before)
	}
}

// modeProbe sets each place's mode, flags and group to what they are, or adds an ACL entry that lets its owner, the
// user named in argv[2], list it, as its mode already does, and says "ok" or the error's name.
const modeProbe = `import json, os, subprocess, sys
said = []
for what, at in json.loads(sys.argv[1]):
    try:
        st = os.lstat(at)
        if what == "chmod":
            os.chmod(at, st.st_mode & 0o7777)
        elif what == "chflags":
            os.chflags(at, st.st_flags)
        elif what == "chgrp":
            os.chown(at, -1, st.st_gid)
        elif what == "acl":
            r = subprocess.run(["/bin/chmod", "+a", "user:" + sys.argv[2] + " allow list", at], capture_output=True)
            if r.returncode != 0:
                raise PermissionError(r.stderr.decode())
        said.append("ok")
    except OSError as e:
        said.append(type(e).__name__)
print(json.dumps(said))`

// F4: the folders above a denied place keep their mode, flags, owner and ACL as well as their place: `chmod 000
// .git/refs/heads` stopped the owner's git ("Needed a single revision"). ~ and ~/.ssh too. A folder and a file in the
// desk's own worktree are still its own.
func TestOnAMacTheFoldersAboveADeniedPlaceKeepTheirModeFlagsOwnerAndACL(t *testing.T) {
	git, python := rowsTools(t)
	d, _ := newRowsDesk(t, git)
	held := []string{filepath.Join(d.git, "refs", "heads"), filepath.Join(d.git, "refs", "tags"), filepath.Join(d.git, "logs"),
		filepath.Join(d.git, "refs", "heads", "hire"), filepath.Join(d.git, "worktrees"), d.home, filepath.Join(d.home, ".ssh")}
	own := []string{filepath.Join(d.root, "sub"), filepath.Join(d.root, "main.go")}
	mustWrite(t, filepath.Join(d.root, "sub", "x.txt"), "the desk's")
	mustWrite(t, filepath.Join(d.root, "main.go"), "package main")
	var steps [][]string
	var want []string
	for _, what := range []string{"chmod", "chflags", "chgrp", "acl"} {
		for _, at := range held {
			steps, want = append(steps, []string{what, at}), append(want, "PermissionError")
		}
		for _, at := range own {
			steps, want = append(steps, []string{what, at}), append(want, "ok")
		}
	}
	encoded, _ := json.Marshal(steps)
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	r := d.inside(t, []string{"PYTHONDONTWRITEBYTECODE=1"}, python, "-I", "-c", modeProbe, string(encoded), me.Username)
	var said []string
	if err := json.Unmarshal([]byte(r.stdout), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v) %s", r.stdout, err, r.stderr)
	}
	for i, s := range steps {
		if said[i] != want[i] {
			t.Errorf("the desk's %s %s: %s, want %s", s[0], strings.TrimPrefix(s[1], d.base), said[i], want[i])
		}
	}
	if r := d.inside(t, nil, "/bin/chmod", "000", filepath.Join(d.git, "refs", "heads")); r.status == 0 {
		t.Error("the desk's chmod 000 .git/refs/heads worked")
	}
}

// The control: a desk's normal git work, beside FXGITROWS2's rows, with every ref packed: its commits on its own
// branch, a fetch from origin (its remote-tracking branches; origin/HEAD is left as it is), a merge of main into its
// branch, a rebase onto main, and, with the owner's rerere on (in the repo's config, and rr-cache there), a conflict
// merged, resolved and committed, and a rebase: its start turns rerere off for its own git (DeskGitConfig), so none of them
// touches MERGE_RR or rr-cache, and each ends as git's always do. Without that, each would end "Unable to create
// MERGE_RR.lock" (exit 128): the second half checks that too, so the row says why DeskGitConfig turns rerere off.
func TestOnAMacADesksGitWorkStillWorksBesideTheGitRows(t *testing.T) {
	git, _ := rowsTools(t)
	d, owner := newRowsDesk(t, git)
	owner(d.checkout, "remote", "add", "origin", d.checkout)
	owner(d.checkout, "pack-refs", "--all")
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
	mustWrite(t, filepath.Join(d.checkout, "owner.txt"), "the owner's")
	owner(d.checkout, "add", "owner.txt")
	owner(d.checkout, "commit", "-q", "-m", "the owner's next")
	owner(d.checkout, "branch", "-f", "main", "feature")
	hireGit("fetch", "-q", "origin")
	if got, want := owner(d.checkout, "rev-parse", "refs/remotes/origin/main"), owner(d.checkout, "rev-parse", "main"); got != want {
		t.Errorf("the desk's fetch: origin/main %s, want %s", got, want)
	}
	hireGit("merge", "-q", "--no-edit", "main")
	hireGit("rebase", "-q", "main")
	hireGit("status", "--short")
	if got := owner(d.checkout, "symbolic-ref", "refs/remotes/origin/HEAD"); got != "refs/remotes/origin/develop\n" {
		t.Errorf("origin/HEAD: %q", got)
	}

	// A conflict, with the owner's rerere on.
	owner(d.checkout, "config", "rerere.enabled", "true")
	if err := os.MkdirAll(filepath.Join(d.git, "rr-cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	d.prepare(t)
	mustWrite(t, filepath.Join(d.root, "owner.txt"), "the desk's\n")
	hireGit("commit", "-q", "-am", "the desk's change")
	mustWrite(t, filepath.Join(d.checkout, "owner.txt"), "the owner's change\n")
	owner(d.checkout, "commit", "-q", "-am", "the owner's change")
	owner(d.checkout, "branch", "-f", "main", "feature")
	if r := d.deskGit(t, git, "merge", "--no-edit", "main"); r.status != 1 || !strings.Contains(r.stdout, "CONFLICT") || r.stderr != "" {
		t.Errorf("the desk's conflicted merge with the owner's rerere on: %+v, want git's conflict and nothing else", r)
	}
	mustWrite(t, filepath.Join(d.root, "owner.txt"), "both\n")
	hireGit("add", "owner.txt")
	if r := hireGit("commit", "-q", "--no-edit"); r.stderr != "" {
		t.Errorf("its commit said %q", r.stderr)
	}
	if got := owner(d.root, "log", "-1", "--format=%P"); len(strings.Fields(got)) != 2 {
		t.Errorf("the desk's merge commit has parents %q", got)
	}
	hireGit("commit", "-q", "--allow-empty", "-m", "after the merge")
	hireGit("rebase", "-q", "--rebase-merges", "main")
	if entries, _ := os.ReadDir(filepath.Join(d.git, "rr-cache")); len(entries) != 0 {
		t.Errorf("rr-cache holds %d entries", len(entries))
	}
	// Without DeskGitConfig: the desk's own commit stops at MERGE_RR's lock.
	r := d.inside(t, gitTestEnv(git), git, "commit", "-q", "--allow-empty", "-m", "rerere on")
	if r.status != 128 || !strings.Contains(r.stderr, "MERGE_RR.lock") {
		t.Errorf("a desk's commit with the owner's rerere on and no DeskGitConfig: %+v, want git to stop at MERGE_RR.lock", r)
	}
}

// DeskGitConfig adds rerere.enabled=false, remote.origin.followRemoteHEAD=never and remote.origin.tagOpt=--no-tags
// after the command-line config an environment already has, and leaves one git can't read as it is.
func TestDeskGitConfigTurnsRerereAndOriginHEADsFollowingOffAfterTheEnvironmentsOwnConfig(t *testing.T) {
	ours := func(n int) []string {
		return []string{"GIT_CONFIG_KEY_" + strconv.Itoa(n) + "=rerere.enabled", "GIT_CONFIG_VALUE_" + strconv.Itoa(n) + "=false",
			"GIT_CONFIG_KEY_" + strconv.Itoa(n+1) + "=remote.origin.followRemoteHEAD", "GIT_CONFIG_VALUE_" + strconv.Itoa(n+1) + "=never",
			"GIT_CONFIG_KEY_" + strconv.Itoa(n+2) + "=remote.origin.tagOpt", "GIT_CONFIG_VALUE_" + strconv.Itoa(n+2) + "=--no-tags"}
	}
	for _, c := range []struct{ env, want []string }{
		{[]string{"A=1"}, append([]string{"A=1", "GIT_CONFIG_COUNT=3"}, ours(0)...)},
		{[]string{"GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=1", "GIT_CONFIG_KEY_1=rerere.enabled", "GIT_CONFIG_VALUE_1=true"},
			append([]string{"GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=1", "GIT_CONFIG_KEY_1=rerere.enabled", "GIT_CONFIG_VALUE_1=true",
				"GIT_CONFIG_COUNT=5"}, ours(2)...)},
		{[]string{"GIT_CONFIG_COUNT=x"}, []string{"GIT_CONFIG_COUNT=x"}},
	} {
		if got := DeskGitConfig(c.env); strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("DeskGitConfig(%q) = %q, want %q", c.env, got, c.want)
		}
	}
}
