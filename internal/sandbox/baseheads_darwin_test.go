package sandbox

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXBASEHEADS on this Mac, with stand-ins only: under the real profile (PrepareProfile, sandbox-exec -f), a stand-in
// desk in its own worktree (python, sbProbe) writes no base branch's name where the owner's git looks before
// refs/heads (`$G/main`, `refs/main`, `refs/tags/main` and the others, `release/…`, their locks, a file renamed onto
// one), can't move refs/tags aside, and writes none of the owner checkout's merge, pick, revert or rebase in progress
// (made, rewritten, renamed onto, removed, their locks). Its own tags and its own worktree's state are still its work.
// A desk at the project still writes the state, its own, and still no base branch's name. Each check is only "ok" or
// the error's name, and the files are plain text stand-ins nobody's git reads.
func TestOnAMacTheProfileKeepsABaseBranchsNameAndTheOwnersStateFromADesk(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, "L", "AD")
	hires := filepath.Join(home, "P", "app-hires")
	root := filepath.Join(hires, "h1")
	repo := filepath.Join(home, "P", "app")
	git := filepath.Join(repo, ".git")
	const old = "1111111111111111111111111111111111111111\n"
	for at, text := range map[string]string{
		git + "/config": "[core]\n", git + "/HEAD": "ref: refs/heads/main\n", git + "/refs/heads/main": old,
		git + "/refs/tags/v1": old, git + "/ORIG_HEAD": old, git + "/worktrees/h1/HEAD": old, root + "/main.go": "package main",
		repo + "/main.go": "package main", base + "/elsewhere/main": old, base + "/elsewhere/MERGE_HEAD": old,
		support + "/hires/h1/BRIEF.md": "brief", support + "/hires/p0/BRIEF.md": "brief",
	} {
		mustWrite(t, at, text)
	}
	const ok, no = "ok", "PermissionError"
	type step struct{ what, at, to, want string }
	desks := []struct {
		name   string
		places Places
		steps  []step
	}{
		{"a desk in its own worktree", Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", Root: root, Checkout: repo,
			GitDir: git, OwnWorktree: "h1", OtherWorktrees: []string{"h1"}, DefaultBranch: "develop", HiresFolder: hires}, []step{
			{"write", git + "/refs/tags/main", "", no}, {"write", git + "/refs/tags/main.lock", "", no},
			{"write", git + "/refs/tags/master", "", no}, {"write", git + "/refs/tags/develop.lock", "", no},
			{"write", git + "/refs/main", "", no}, {"write", git + "/refs/develop", "", no}, {"write", git + "/main", "", no},
			{"write", git + "/main.lock", "", no}, {"write", git + "/master", "", no},
			{"mkdir", git + "/refs/tags/release", "", no}, {"mkdir", git + "/refs/release", "", no}, {"mkdir", git + "/release", "", no},
			{"rename", base + "/elsewhere/main", git + "/refs/tags/main", no},
			{"rename", git + "/refs/tags", git + "/refs/tags-aside", no},
			{"write", git + "/MERGE_HEAD", "", no}, {"write", git + "/MERGE_HEAD.lock", "", no}, {"write", git + "/ORIG_HEAD", "", no},
			{"write", git + "/ORIG_HEAD.lock", "", no}, {"unlink", git + "/ORIG_HEAD", "", no},
			{"write", git + "/CHERRY_PICK_HEAD", "", no}, {"write", git + "/REVERT_HEAD", "", no},
			{"write", git + "/REBASE_HEAD.lock", "", no}, {"write", git + "/AUTO_MERGE", "", no},
			{"rename", base + "/elsewhere/MERGE_HEAD", git + "/MERGE_HEAD", no},
			{"mkdir", git + "/rebase-merge", "", no}, {"mkdir", git + "/rebase-apply", "", no},
			// …and its own work.
			{"write", git + "/refs/tags/v2.lock", "", ok}, {"write", git + "/refs/tags/v2", "", ok},
			{"write", git + "/refs/tags/mainline", "", ok}, {"write", git + "/worktrees/h1/MERGE_HEAD", "", ok},
			{"write", git + "/worktrees/h1/ORIG_HEAD.lock", "", ok}, {"write", git + "/worktrees/h1/CHERRY_PICK_HEAD", "", ok},
			{"mkdir", git + "/worktrees/h1/rebase-merge", "", ok}, {"write", root + "/main.go", "", ok},
		}},
		{"a desk at the project", Places{GOOS: "darwin", Home: home, Support: support, OwnID: "p0", Root: repo, GitDir: git,
			DefaultBranch: "develop", HiresFolder: hires}, []step{
			{"write", git + "/MERGE_HEAD", "", ok}, {"write", git + "/ORIG_HEAD.lock", "", ok}, {"write", git + "/ORIG_HEAD", "", ok},
			{"write", git + "/CHERRY_PICK_HEAD", "", ok}, {"write", git + "/REVERT_HEAD", "", ok}, {"write", git + "/REBASE_HEAD", "", ok},
			{"write", git + "/AUTO_MERGE", "", ok}, {"write", repo + "/main.go", "", ok},
			{"write", git + "/refs/tags/main", "", no}, {"write", git + "/main", "", no}, {"mkdir", git + "/rebase-merge", "", no},
		}},
	}
	for i, desk := range desks {
		p := mustNew(t, desk.places)
		file := filepath.Join(support, "hires", desk.places.OwnID, "sandbox.sb")
		if err := PrepareProfile(p, file); err != nil {
			t.Fatal(err)
		}
		var args [][]string
		for _, s := range desk.steps {
			arg := []string{s.what, s.at}
			if s.to != "" {
				arg = append(arg, s.to)
			}
			args = append(args, arg)
		}
		encoded, _ := json.Marshal(args)
		cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", file, python, "-I", "-c", sbProbe, string(encoded)))
		cmd.Dir, cmd.Env = base, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s: the stand-in didn't run under the profile: %v %s", desk.name, err, stderr.Bytes())
		}
		var said []string
		if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(desk.steps) {
			t.Fatalf("%s: the stand-in said %q (%v)", desk.name, out.Bytes(), err)
		}
		for j, s := range desk.steps {
			if said[j] != s.want {
				t.Errorf("%s: %s %s %s: %s, want %s", desk.name, s.what, strings.TrimPrefix(s.at, base), strings.TrimPrefix(s.to, base), said[j], s.want)
			}
		}
		if i == 0 {
			if data, _ := os.ReadFile(git + "/ORIG_HEAD"); string(data) != old {
				t.Errorf("%s: the owner's ORIG_HEAD changed: %q", desk.name, data)
			}
			for _, at := range []string{"/refs/tags/main", "/refs/main", "/main", "/MERGE_HEAD", "/CHERRY_PICK_HEAD", "/AUTO_MERGE", "/rebase-merge"} {
				if _, err := os.Lstat(git + at); err == nil {
					t.Errorf("%s: $G%s was made", desk.name, at)
				}
			}
		}
	}
}

// What the owner's git takes for main, after a desk in its own worktree tried to give a base branch's name to its own
// commit, by git (`tag main`, `tag release/1.26`, `update-ref refs/main`, `update-ref main`, a tag of master and of
// release/1.27) and by a stand-in writing the commit's id there (`$G/main`, `refs/main`, `refs/tags/main`, refs/tags
// moved aside): each is refused, and the owner's `git rev-parse main`, `git show main:README` and `git worktree add … main` still take the
// owner's main, with no "ambiguous" warning; `release/1.26` too. The desk still tags its own names.
func TestOnAMacADesksTagOfABaseBranchsNameNoLongerTakesTheOwnersMain(t *testing.T) {
	sandboxExecOrSkip(t)
	git := gitOrSkip(t)
	python := python3(t)
	if python == "" {
		t.Skip("no python3 here")
	}
	d, outside := newGitDesk(t, git)
	hireGit := func(want bool, args ...string) {
		t.Helper()
		r := d.inside(t, gitTestEnv(git), git, args...)
		if (r.status == 0) != want {
			t.Errorf("the desk's git %q: %+v, want it to work: %v", args, r, want)
		}
	}
	outside(d.checkout, "branch", "release/1.26")
	mainBefore, releaseBefore := outside(d.checkout, "rev-parse", "main"), outside(d.checkout, "rev-parse", "release/1.26")
	mustWrite(t, filepath.Join(d.root, "README"), "the desk's")
	hireGit(true, "commit", "-q", "-a", "-m", "the desk's")
	desk := strings.TrimSpace(outside(d.root, "rev-parse", "HEAD"))
	for _, args := range [][]string{{"tag", "main"}, {"tag", "release/1.26"}, {"tag", "master"}, {"update-ref", "refs/main", "HEAD"},
		{"update-ref", "main", "HEAD"}, {"update-ref", "refs/tags/release/1.27", "HEAD"}} {
		hireGit(false, args...)
	}
	for _, args := range [][]string{{"tag", "h1-v1"}, {"tag", "mainline"}, {"tag", "v/main"}} {
		hireGit(true, args...)
	}
	// The stand-in writes the desk's commit id itself, and tries refs/tags moved aside.
	steps := [][]string{{"write", filepath.Join(d.git, "main")}, {"write", filepath.Join(d.git, "refs", "main")},
		{"write", filepath.Join(d.git, "refs", "tags", "main")}, {"mkdir", filepath.Join(d.git, "refs", "tags", "release")},
		{"rename", filepath.Join(d.git, "refs", "tags"), filepath.Join(d.git, "refs", "tags-aside")}}
	encoded, _ := json.Marshal(steps)
	r := d.inside(t, nil, python, "-I", "-c", plantID, string(encoded), desk)
	var said []string
	if err := json.Unmarshal([]byte(r.stdout), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %+v (%v)", r, err)
	}
	for i, s := range steps {
		if said[i] != "PermissionError" {
			t.Errorf("the stand-in's %s %s: %s, want PermissionError", s[0], strings.TrimPrefix(s[1], d.git), said[i])
		}
	}
	if got := outside(d.checkout, "rev-parse", "main"); got != mainBefore {
		t.Errorf("the owner's rev-parse main: %q, want %q", got, mainBefore)
	}
	if got := outside(d.checkout, "rev-parse", "release/1.26"); got != releaseBefore {
		t.Errorf("the owner's rev-parse release/1.26: %q, want %q", got, releaseBefore)
	}
	if got := outside(d.checkout, "show", "main:README"); got != "the owner's checkout" {
		t.Errorf("the owner's show main:README: %q", got)
	}
	added := filepath.Join(d.home, "added")
	outside(d.checkout, "worktree", "add", "-q", "--detach", added, "main")
	if got := outside(added, "rev-parse", "HEAD"); got != mainBefore {
		t.Errorf("the owner's worktree add of main: %q, want %q", got, mainBefore)
	}
	if data, _ := os.ReadFile(filepath.Join(added, "README")); string(data) != "the owner's checkout" {
		t.Errorf("the owner's worktree add of main checked out %q", data)
	}
	if got := outside(d.checkout, "tag", "--list"); got != "h1-v1\nmainline\nv/main\n" {
		t.Errorf("the tags: %q, want the desk's own three", got)
	}
}

// What the owner's next commit on main is, after a desk in its own worktree tried to leave the owner checkout a merge,
// pick, revert or rebase in progress of its commit (MERGE_HEAD written, or renamed onto from its own folder; ORIG_HEAD,
// CHERRY_PICK_HEAD, REVERT_HEAD, REBASE_HEAD, AUTO_MERGE, rebase-merge): each is refused, and the owner's `git commit`
// on main makes a commit with main as its only parent, not a merge of the desk's.
func TestOnAMacADesksMergeHeadNoLongerMakesTheOwnersNextCommitAMerge(t *testing.T) {
	sandboxExecOrSkip(t)
	git := gitOrSkip(t)
	python := python3(t)
	if python == "" {
		t.Skip("no python3 here")
	}
	d, outside := newGitDesk(t, git)
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work")
	for _, args := range [][]string{{"add", "work.go"}, {"commit", "-q", "-m", "the desk's"}} {
		if r := d.inside(t, gitTestEnv(git), git, args...); r.status != 0 {
			t.Fatalf("the desk's git %q: %+v", args, r)
		}
	}
	desk := strings.TrimSpace(outside(d.root, "rev-parse", "HEAD"))
	mainBefore := strings.TrimSpace(outside(d.checkout, "rev-parse", "main"))
	mine := filepath.Join(d.root, "MERGE_HEAD")
	mustWrite(t, mine, desk+"\n")
	var steps [][]string
	for _, name := range []string{"MERGE_HEAD", "ORIG_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "REBASE_HEAD", "AUTO_MERGE"} {
		steps = append(steps, []string{"write", filepath.Join(d.git, name)})
	}
	steps = append(steps, []string{"rename", mine, filepath.Join(d.git, "MERGE_HEAD")}, []string{"mkdir", filepath.Join(d.git, "rebase-merge")})
	encoded, _ := json.Marshal(steps)
	r := d.inside(t, nil, python, "-I", "-c", plantID, string(encoded), desk)
	var said []string
	if err := json.Unmarshal([]byte(r.stdout), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %+v (%v)", r, err)
	}
	for i, s := range steps {
		if said[i] != "PermissionError" {
			t.Errorf("the stand-in's %s %s: %s, want PermissionError", s[0], strings.TrimPrefix(s[1], d.git), said[i])
		}
	}
	outside(d.checkout, "commit", "-q", "--allow-empty", "-m", "the owner's next")
	if got := strings.TrimSpace(outside(d.checkout, "log", "-1", "--format=%P", "main")); got != mainBefore {
		t.Errorf("the owner's next commit's parents: %q, want main's %s alone (the desk's is %s)", got, mainBefore, desk)
	}
}

// A desk's own merge, pick, revert and rebase in its own worktree still work beside the owner checkout's denied state:
// each conflict leaves its state in the desk's registration (worktrees/h1: REBASE_HEAD and rebase-merge,
// CHERRY_PICK_HEAD, MERGE_HEAD and AUTO_MERGE) and each --abort clears it; a rebase through a conflict, --continue; a
// revert, a pick and a reset (ORIG_HEAD) work; nothing is refused on the way, nothing of it lands in the shared .git,
// and main stays where the owner put it.
func TestOnAMacADesksOwnRebaseAndPickStillWorkInItsWorktree(t *testing.T) {
	sandboxExecOrSkip(t)
	git := gitOrSkip(t)
	d, outside := newGitDesk(t, git)
	own := filepath.Join(d.git, "worktrees", "h1")
	hireGit := func(want bool, args ...string) {
		t.Helper()
		r := d.inside(t, append(gitTestEnv(git), "GIT_EDITOR=true"), git, args...)
		if (r.status == 0) != want {
			t.Errorf("the desk's git %q: %+v, want it to work: %v", args, r, want)
		}
		for _, line := range strings.Split(r.stderr, "\n") {
			for _, refused := range []string{"not permitted", "Unable to create", "cannot lock", "could not lock", "Permission denied"} {
				if strings.Contains(line, refused) {
					t.Errorf("the desk's git %q: %s", args, line)
				}
			}
		}
	}
	there := func(names ...string) {
		t.Helper()
		for _, name := range names {
			if _, err := os.Lstat(filepath.Join(own, name)); err != nil {
				t.Errorf("the desk's %s: %v", name, err)
			}
		}
	}
	mustWrite(t, filepath.Join(d.root, "README"), "the desk's")
	hireGit(true, "commit", "-q", "-a", "-m", "the desk's")
	mustWrite(t, filepath.Join(d.checkout, "README"), "the owner's next")
	outside(d.checkout, "commit", "-q", "-a", "-m", "the owner's next")
	mainBefore := outside(d.checkout, "rev-parse", "main")

	hireGit(false, "rebase", "-q", "main")
	there("REBASE_HEAD", "rebase-merge")
	hireGit(true, "rebase", "--abort")
	hireGit(false, "cherry-pick", "main")
	there("CHERRY_PICK_HEAD")
	hireGit(true, "cherry-pick", "--abort")
	hireGit(false, "merge", "-q", "main")
	there("MERGE_HEAD", "AUTO_MERGE")
	hireGit(true, "merge", "--abort")
	hireGit(false, "rebase", "-q", "main")
	mustWrite(t, filepath.Join(d.root, "README"), "both")
	hireGit(true, "add", "README")
	hireGit(true, "rebase", "--continue")
	hireGit(true, "revert", "--no-edit", "HEAD")
	hireGit(true, "cherry-pick", "HEAD~1")
	hireGit(true, "reset", "-q", "--hard", "HEAD~1")
	there("ORIG_HEAD")

	if data, _ := os.ReadFile(filepath.Join(d.root, "README")); string(data) != "the owner's next" {
		t.Errorf("the desk's README after its round: %q", data)
	}
	if got := outside(d.checkout, "log", "-1", "--format=%s", "h1~1"); got != "the desk's\n" {
		t.Errorf("the desk's rebase: h1~1 is %q", got)
	}
	if got := outside(d.checkout, "rev-parse", "main"); got != mainBefore {
		t.Errorf("main moved: %s, was %s", got, mainBefore)
	}
	for _, name := range []string{"MERGE_HEAD", "ORIG_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "REBASE_HEAD", "AUTO_MERGE", "rebase-merge", "rebase-apply"} {
		if _, err := os.Lstat(filepath.Join(d.git, name)); err == nil {
			t.Errorf("$G/%s was made", name)
		}
	}
}

// plantID does each step of its first argument, a JSON list of [what, path, (path)]: "write" puts its second argument,
// a commit's id, in the file, "mkdir" makes a folder and "rename" renames; it prints a JSON list of "ok" or the error's
// name, one per step.
const plantID = `import json, os, sys
said = []
for step in json.loads(sys.argv[1]):
    try:
        if step[0] == "rename":
            os.rename(step[1], step[2])
        elif step[0] == "mkdir":
            os.mkdir(step[1])
        else:
            with open(step[1], "w") as f:
                f.write(sys.argv[2] + "\n")
        said.append("ok")
    except OSError as e:
        said.append(type(e).__name__)
print(json.dumps(said))`
