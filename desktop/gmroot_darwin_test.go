package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXGMROOT (RT126SWEEP2 P1 B1) under the real profile, with stand-ins only: the default layout, which no row laid out
// before. The GM is hired into the folder for hires (~/Projects), which isn't a repo and holds two repos, a hire's
// worktree of one and the owner's checkouts. Each desk's sandbox is made from its registry row as its start makes it
// (deskAtOf, makeDeskSandbox), written by PrepareProfile, and a stand-in desk (python, git) is run by
// /usr/bin/sandbox-exec. The GM writes none of the projects and only its workspace; a hire under it writes its own
// worktree and not the GM's workspace; a hook the GM tries to plant isn't there for the owner's `git checkout` to run.
// Each step says only "ok" or the error's name.

// gmRootProbe does each step of its JSON argument, [what, path], and prints a JSON list of "ok" or the error's name.
// A hook is a script written and made executable; "link <file>" makes path a second name of file (a hard link); a
// tree is a folder of 1,500 small files.
const gmRootProbe = `import json, os, sys
said = []
for what, at in json.loads(sys.argv[1]):
    try:
        if what == "read":
            with open(at) as f:
                f.read()
        elif what == "write":
            with open(at, "w") as f:
                f.write("standin")
        elif what == "hook":
            with open(at, "w") as f:
                f.write("#!/bin/sh\necho planted > " + json.dumps(sys.argv[2]) + "\n")
            os.chmod(at, 0o755)
        elif what == "mkdir":
            os.mkdir(at)
        elif what == "chmod":
            os.chmod(at, 0o777)
        elif what == "rename":
            os.rename(at, at + ".moved")
        elif what.startswith("link "):
            os.link(what[5:], at)
        elif what.startswith("symlink "):
            os.symlink(what[8:], at)
        elif what == "tree":
            os.mkdir(at)
            for i in range(1500):
                with open(os.path.join(at, str(i)), "w") as f:
                    f.write("standin")
        said.append("ok")
    except OSError as e:
        said.append(type(e).__name__)
print(json.dumps(said))`

// gmRootLayout is the default layout, in a stand-in home and support folder.
type gmRootLayout struct {
	base, projects, app, lib, hireTree, workspace, planted string
	gm, hire                                               Hire
}

func newGMRootLayout(t *testing.T) gmRootLayout {
	t.Helper()
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil || deskPython(t) == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 missing", err)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	shortSupport(t)
	supportDir = physical(supportDir)
	deskSockets(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	homeDir = filepath.Join(base, "home")
	l := gmRootLayout{base: base, projects: filepath.Join(homeDir, "Projects"), planted: filepath.Join(base, "planted")}
	l.app, l.lib = filepath.Join(l.projects, "app"), filepath.Join(l.projects, "lib")
	l.hireTree = filepath.Join(hiresFolder(l.app, ""), "h1")
	l.workspace = filepath.Join(hireFolder(supportDir, "gm1"), "work") // sandbox.Workspace(supportDir, "gm1")
	for _, repo := range []string{l.app, l.lib} {
		if err := os.MkdirAll(repo, 0o700); err != nil {
			t.Fatal(err)
		}
		standInOwnersGit(t, repo, "init", "-q", "-b", "main")
		if err := os.WriteFile(filepath.Join(repo, "README"), []byte("the owner's\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		standInOwnersGit(t, repo, "add", "README")
		standInOwnersGit(t, repo, "commit", "-q", "-m", "first")
	}
	standInOwnersGit(t, l.app, "branch", "develop")
	standInOwnersGit(t, l.app, "worktree", "add", "-q", "-b", "hire/h1", l.hireTree)
	l.gm = Hire{ID: "gm1", Name: "General Manager", Role: gmRole, Folder: l.projects, State: "working"}
	l.hire = Hire{ID: "h1", Name: "Janitor", Role: "janitor", Folder: l.app, Repo: l.app, Worktree: l.hireTree, Branch: "hire/h1", State: "working"}
	writeRegistryHires(t, l.gm, l.hire)
	// What the GM's hire makes besides its sandbox: its own folder, its plans folder where it starts.
	for _, id := range []string{"gm1", "h1"} {
		if err := os.MkdirAll(hireFolder(supportDir, id), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := makeDeskPlans(l.projects, "gm1"); err != nil {
		t.Fatal(err)
	}
	return l
}

// standInOwnersGit runs git as the owner does, outside any sandbox, in the stand-in home: no config of the owner's.
func standInOwnersGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := disclaim.Cmd(exec.Command("git", append([]string{"-c", "user.name=Owner", "-c", "user.email=owner@example.invalid",
		"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...))
	cmd.Dir, cmd.Env = dir, gmRootEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %q in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// gmRootEnv is a stand-in desk's and the owner's git's environment: the stand-in home, no system git config.
func gmRootEnv() []string {
	return []string{"HOME=" + homeDir, "PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "PYTHONDONTWRITEBYTECODE=1", "LANG=C"}
}

// profile is desk h's sandbox made from its row as its start makes it, and its profile written: the profile's path.
func (l gmRootLayout) profile(t *testing.T, h Hire) (string, *deskSandbox) {
	t.Helper()
	s, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
	if err != nil {
		t.Fatalf("%s's sandbox: %v", h.ID, err)
	}
	file := filepath.Join(hireFolder(supportDir, h.ID), "sandbox.sb")
	if err := sandbox.PrepareProfile(s.policy, file, deskServices(s.places, deskSocketPath(h.ID, 0))); err != nil {
		t.Fatal(err)
	}
	return file, s
}

// probeStep is one step of a stand-in desk, and what it must say: "ok", an error's name, or "refused" for any error.
type probeStep struct{ what, at, want string }

// run runs steps in a stand-in desk under profile, in dir, as the desk would: the process must end by itself, 0.
func (l gmRootLayout) run(t *testing.T, profile, dir string, steps []probeStep) {
	t.Helper()
	var args [][]string
	for _, s := range steps {
		args = append(args, []string{s.what, s.at})
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", "-f", profile, deskPython(t), "-I", "-c", gmRootProbe, string(encoded), l.planted))
	cmd.Dir, cmd.Env = dir, gmRootEnv()
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var said []string
	if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v)", out.Bytes(), err)
	}
	for i, s := range steps {
		if said[i] != s.want && !(s.want == "refused" && said[i] != "ok") {
			t.Errorf("%s %s: %s, want %s", s.what, strings.TrimPrefix(s.at, l.base), said[i], s.want)
		}
	}
}

// (i): the GM is refused every repo's shared git places and base branches, the hire's worktree, the owner's checkouts
// and a new folder in the projects folder, and the folder itself stays as it is. Reads stay its own.
func TestTheGMInTheProjectsFolderChangesNoRepoItWorksOver(t *testing.T) {
	l := newGMRootLayout(t)
	profile, _ := l.profile(t, l.gm)
	const no = "PermissionError"
	var steps []probeStep
	for _, repo := range []string{l.app, l.lib} {
		steps = append(steps, probeStep{"hook", repo + "/.git/hooks/pre-commit", no}, probeStep{"write", repo + "/.git/config", no},
			probeStep{"write", repo + "/.git/refs/heads/main", no}, probeStep{"write", repo + "/README", no},
			probeStep{"write", repo + "/new.go", no}, probeStep{"read", repo + "/README", "ok"}, probeStep{"read", repo + "/.git/config", "ok"})
	}
	steps = append(steps,
		probeStep{"write", l.app + "/.git/refs/heads/develop", no}, probeStep{"write", l.app + "/.git/refs/heads/hire/h1", no},
		probeStep{"write", l.app + "/.git/worktrees/h1/HEAD", no},
		probeStep{"write", l.hireTree + "/README", no}, probeStep{"write", l.hireTree + "/new.go", no},
		probeStep{"mkdir", l.projects + "/new-project", no}, probeStep{"write", l.projects + "/notes.md", no},
		probeStep{"rename", l.lib, no}, probeStep{"chmod", l.projects, no}, probeStep{"rename", l.projects, no})
	l.run(t, profile, l.projects, steps)
	for _, repo := range []string{l.app, l.lib} {
		if data, _ := os.ReadFile(repo + "/README"); string(data) != "the owner's\n" {
			t.Errorf("the owner's checkout file in %s is now %q", strings.TrimPrefix(repo, l.base), data)
		}
	}
}

// (iv), P1's repro as a row: the GM plants a post-checkout hook in a repo's shared .git, and the owner then checks out
// a branch there, outside any sandbox. The hook isn't there, and nothing planted runs.
func TestTheOwnersCheckoutRunsNoHookTheGMPlanted(t *testing.T) {
	l := newGMRootLayout(t)
	profile, _ := l.profile(t, l.gm)
	hook := l.app + "/.git/hooks/post-checkout"
	l.run(t, profile, l.projects, []probeStep{{"hook", hook, "PermissionError"}})
	standInOwnersGit(t, l.app, "checkout", "-q", "develop")
	if _, err := os.Lstat(l.planted); !errors.Is(err, os.ErrNotExist) {
		t.Error("the owner's git checkout ran a hook the GM planted, outside the sandbox")
	}
	if _, err := os.Lstat(hook); !errors.Is(err, os.ErrNotExist) {
		t.Error("the GM's hook is in the repo's hooks folder")
	}
}

// (ii) and (vi): the GM writes its workspace, and its plans where Claude Code keeps them; what Claude Code would write
// in the folder it starts in is refused, and the session goes on (the stand-in's later steps run, and it ends by
// itself). It reads the repos with git as it reviews.
func TestTheGMInTheProjectsFolderWritesItsWorkspaceAndGoesOnPastARefusal(t *testing.T) {
	l := newGMRootLayout(t)
	profile, _ := l.profile(t, l.gm)
	l.run(t, profile, l.projects, []probeStep{
		// Claude Code's own writes in the folder it starts in (FXGMROOT's table): refused.
		{"mkdir", l.projects + "/.claude", "PermissionError"}, {"write", l.projects + "/.claude/settings.local.json", "refused"},
		{"write", l.projects + "/CLAUDE.md", "PermissionError"}, {"write", l.projects + "/.mcp.json", "PermissionError"},
		{"write", l.app + "/CLAUDE.md", "PermissionError"},
		// …and the GM goes on: its plans, where Claude Code takes them, and its workspace.
		{"write", l.projects + "/" + deskPlansFor("gm1") + "/calm-river.md", "ok"},
		{"read", l.projects + "/" + deskPlansFor("gm1") + "/calm-river.md", "ok"},
		{"write", l.projects + "/.smolcorp/plans/h1.md", "PermissionError"}, {"write", l.projects + "/.smolcorp/.gitignore", "PermissionError"},
		{"write", l.workspace + "/notes.md", "ok"}, {"mkdir", l.workspace + "/team", "ok"}, {"write", l.workspace + "/team/h1.md", "ok"},
		{"read", l.workspace + "/notes.md", "ok"}, {"rename", l.workspace, "PermissionError"},
	})
	// Its reviews' git reads: status, log and diff in a repo it can't write.
	for _, args := range [][]string{{"status", "--porcelain"}, {"log", "--oneline", "-1"}, {"diff", "main", "hire/h1"}} {
		cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", append([]string{"-f", profile, "/usr/bin/git", "-C", l.app}, args...)...))
		cmd.Dir, cmd.Env = l.projects, gmRootEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("the GM's git %q: %v\n%s", args, err, out)
		}
	}
}

// (iii): a hire under that GM writes its own worktree (B1's other half: refused while the GM's folder was the projects
// folder), and is refused the GM's workspace.
func TestAHireUnderTheGMInTheProjectsFolderWritesItsOwnWorktree(t *testing.T) {
	l := newGMRootLayout(t)
	l.profile(t, l.gm) // the GM's start, as it made its workspace
	if err := os.MkdirAll(l.workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(l.workspace+"/notes.md", []byte("the GM's notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, _ := l.profile(t, l.hire)
	l.run(t, profile, l.hireTree, []probeStep{
		{"write", l.hireTree + "/own.txt", "ok"}, {"mkdir", l.hireTree + "/pkg", "ok"}, {"write", l.hireTree + "/README", "ok"},
		{"write", l.workspace + "/notes.md", "PermissionError"}, {"read", l.workspace + "/notes.md", "PermissionError"},
		{"write", l.workspace + "/planted.md", "PermissionError"},
		{"write", l.app + "/README", "PermissionError"}, {"hook", l.app + "/.git/hooks/post-checkout", "PermissionError"},
	})
}

// FXGMROOTR1 N1: the GM's ordinary work in its workspace (a tool's hard links, pnpm's store; a big tree, a project's
// dependencies) stops no desk's start. Each start walks the support folder whole, and the workspace is in it, so this
// refused every hire's start (RT126FIXREVIEW-GMROOT N1). The walk passes over the workspace; what it still meets is
// TestALinkMadeOutsideAnySandboxIntoTheGMsWorkspaceStillStopsTheStart's.
func TestTheGMsLinksAndBigTreesInItsWorkspaceStopNoDesksStart(t *testing.T) {
	l := newGMRootLayout(t)
	gmProfile, _ := l.profile(t, l.gm) // the GM's start, as it made its workspace
	most := linkCheckMost
	t.Cleanup(func() { linkCheckMost = most })
	// A stand-in for 100,000 entries: 1,000, more than the rest of the layout holds, and less than the GM's tree.
	linkCheckMost = 1000
	if _, err := makeDeskSandbox(deskAtOf(l.hire, deskSocketPath(l.hire.ID, 0), nil, engineOptions{Git: runGit})); err != nil {
		t.Fatalf("with the workspace empty, the hire's start: %v", err)
	}
	for _, c := range []struct {
		what  string
		steps []probeStep
	}{
		{"a hard link", []probeStep{{"write", l.workspace + "/notes.md", "ok"}, {"link " + l.workspace + "/notes.md", l.workspace + "/notes-copy.md", "ok"}}},
		{"a big tree", []probeStep{{"tree", l.workspace + "/node_modules", "ok"}}},
	} {
		l.run(t, gmProfile, l.projects, c.steps)
		for _, h := range []Hire{l.hire, l.gm} {
			if _, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit})); err != nil {
				t.Errorf("after %s in the GM's workspace, %s's start: %v", c.what, h.ID, err)
			}
		}
	}
}

// …and what the walk still meets: the GM can't give a file it can't read or write a second name in its workspace (the
// profile refuses the link), so one there was made outside any sandbox, and the file's own name, which the walk meets,
// still stops the start (the setup-key's here). A file the start replaces (a brief) with a name there that the walk
// didn't meet stops it too (X12LINK1).
func TestALinkMadeOutsideAnySandboxIntoTheGMsWorkspaceStillStopsTheStart(t *testing.T) {
	l := newGMRootLayout(t)
	gmProfile, _ := l.profile(t, l.gm)
	key, otherBrief, ownBrief := supportDir+"/setup-key", hireFolder(supportDir, "h1")+"/BRIEF.md", hireFolder(supportDir, "gm1")+"/BRIEF.md"
	for _, f := range []string{key, otherBrief, ownBrief} {
		if err := os.WriteFile(f, []byte("standin"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Under the GM's profile: no second name in its workspace for a file it can't read, or can read and not write.
	l.run(t, gmProfile, l.projects, []probeStep{
		{"link " + key, l.workspace + "/key", "PermissionError"}, {"link " + otherBrief, l.workspace + "/brief", "PermissionError"},
		{"link " + ownBrief, l.workspace + "/own-brief", "PermissionError"}, {"link " + l.app + "/README", l.workspace + "/readme", "PermissionError"},
		{"link " + l.app + "/.git/config", l.workspace + "/config", "PermissionError"},
	})
	for _, name := range []string{"key", "brief", "own-brief", "readme", "config"} {
		if _, err := os.Lstat(l.workspace + "/" + name); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is in the GM's workspace", name)
		}
	}
	// Outside any sandbox: the setup-key's second name there stops every start, the GM's own too.
	if err := os.Link(key, l.workspace+"/key"); err != nil {
		t.Fatal(err)
	}
	for _, h := range []Hire{l.hire, l.gm} {
		_, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("with the setup-key's second name in the GM's workspace, %s's start: %v", h.ID, err)
		}
	}
	// The GM's brief, which the core replaces with a fresh copy at its start: a second name there that the walk passes
	// over stops the GM's start.
	if err := os.Remove(l.workspace + "/key"); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(ownBrief, l.workspace+"/own-brief"); err != nil {
		t.Fatal(err)
	}
	if _, err := makeDeskSandbox(deskAtOf(l.gm, deskSocketPath(l.gm.ID, 0), nil, engineOptions{Git: runGit})); err == nil ||
		!strings.Contains(err.Error(), ownBrief) {
		t.Errorf("with its brief's second name in its workspace, the GM's start: %v", err)
	}
}

// FXGMROOTR1 N2: a hire under the GM writes none of what the GM reads from the folder it starts in, which its
// workspace doesn't hold: the GM's plans (its plan mode shows the owner one and then follows it), and the agent files
// Claude Code loads into the GM's session from there, nor their writers' names. Its own worktree stays its to write.
func TestAHireUnderTheGMInTheProjectsFolderChangesNothingTheGMLoads(t *testing.T) {
	l := newGMRootLayout(t)
	l.profile(t, l.gm)
	plans := l.projects + "/.smolcorp/plans/gm1"
	if err := os.WriteFile(plans+"/calm-river.md", []byte("the GM's plan"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CLAUDE.md", ".mcp.json"} { // the owner's, already there
		if err := os.WriteFile(l.projects+"/"+name, []byte("the owner's"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	profile, _ := l.profile(t, l.hire)
	const no = "PermissionError"
	steps := []probeStep{
		{"write", plans + "/calm-river.md", no}, {"write", plans + "/new.md", no}, {"mkdir", plans + "/sub", no}, {"rename", plans, no},
		{"read", plans + "/calm-river.md", "ok"},
	}
	for _, name := range []string{"CLAUDE.md", "CLAUDE.local.md", ".mcp.json", "AGENTS.md"} {
		steps = append(steps, probeStep{"write", l.projects + "/" + name, no},
			// Their writers' names, renamed onto them: git's lock, Claude Code's and Node's copy, sed -i's, install's.
			probeStep{"write", l.projects + "/" + name + ".lock", no}, probeStep{"write", l.projects + "/" + name + ".tmp.501.1", no},
			probeStep{"write", l.projects + "/.!501!" + name, no})
	}
	steps = append(steps, probeStep{"write", l.projects + "/INS@abc123", no},
		probeStep{"write", l.hireTree + "/own.txt", "ok"}, probeStep{"write", l.hireTree + "/CLAUDE.md", "ok"})
	l.run(t, profile, l.hireTree, steps)
	if data, _ := os.ReadFile(plans + "/calm-river.md"); string(data) != "the GM's plan" {
		t.Errorf("the GM's plan is now %q", data)
	}
}
