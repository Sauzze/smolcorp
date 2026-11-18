package desktop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXHIREPROJECTSR1 (RT126FIXREVIEW-GMROOTR1 N1) under the real profile, with stand-ins only: a hire writes nothing the
// GM's Claude Code loads from the folder it starts in (its playbook and briefs, a .claude's contents, a .git there),
// whether the GM is in a folder of its own or in the projects folder; and no desk makes a git folder at home, which the
// owner's git takes for every folder below it in no repo.

// ownerMade makes each file (a name ending in / is a folder) in the stand-in home, as the owner or the GM had them.
func ownerMade(t *testing.T, base string, names ...string) {
	t.Helper()
	for _, name := range names {
		at := filepath.Join(base, name)
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(at, 0o700); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte("the owner's\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// startFolderSteps are what a hire is refused at the top of the folder the GM starts in: what Claude Code loads into
// the GM's session from there, and a .git or a bare repo's HEAD there, which the GM's git status would take.
func startFolderSteps(start string) []probeStep {
	const no = "PermissionError"
	return []probeStep{
		{"write", start + "/PLAYBOOK.md", no}, {"write", start + "/brief-h1.md", no}, {"write", start + "/brief-h2.md", no},
		{"write", start + "/.claude/agents/x.md", no}, {"write", start + "/.claude/commands/x.md", no},
		{"write", start + "/.claude/CLAUDE.md", no}, {"mkdir", start + "/.claude/skills", no},
		{"write", start + "/CLAUDE.md", no}, {"write", start + "/notes.md", no},
		{"mkdir", start + "/.git", no}, {"write", start + "/HEAD", no},
		{"read", start + "/PLAYBOOK.md", "ok"}, {"read", start + "/.claude/agents/a.md", "ok"},
	}
}

// N1 (a): the GM in a folder of its own, outside the projects folder, the layout every row before FXGMROOT used. A hire
// under it is kept out of all of it, as on the base, and writes its own worktree.
func TestAHireIsKeptOutOfTheGMsOwnFolderWhole(t *testing.T) {
	l := newGMRootLayout(t)
	gmHome := filepath.Join(homeDir, "gm-home")
	ownerMade(t, gmHome, "PLAYBOOK.md", "brief-h1.md", "CLAUDE.md", ".claude/agents/a.md", ".claude/commands/")
	l.gm.Folder = gmHome
	writeRegistryHires(t, l.gm, l.hire)
	setHireFolder(t, l.projects)
	if err := makeDeskPlans(gmHome, "gm1"); err != nil {
		t.Fatal(err)
	}
	l.profile(t, l.gm)
	profile, s := l.profile(t, l.hire)
	if s.places.GMStart != gmHome {
		t.Fatalf("the hire's Places give the GM's start folder as %q", s.places.GMStart)
	}
	const no = "PermissionError"
	steps := append(startFolderSteps(gmHome),
		probeStep{"write", gmHome + "/.smolcorp/plans/gm1/p.md", no}, probeStep{"rename", gmHome, no},
		probeStep{"write", l.hireTree + "/own.txt", "ok"}, probeStep{"mkdir", l.hireTree + "/pkg", "ok"})
	l.run(t, profile, l.hireTree, steps)
	if data, _ := os.ReadFile(gmHome + "/PLAYBOOK.md"); string(data) != "the owner's\n" {
		t.Errorf("the GM's playbook is now %q", data)
	}
}

// N1 (b): the GM in the projects folder, the default. The owner's ~/Projects/.claude is there (its commands, as the
// owner's own Claude Code leaves them), and the GM's playbook. A hire held to its own worktree there is refused all of
// the above, and every entry of a .git there, should an empty one be there already.
func TestAHireUnderTheGMInTheProjectsFolderMakesNothingThereTheGMLoads(t *testing.T) {
	l := newGMRootLayout(t)
	ownerMade(t, l.projects, "PLAYBOOK.md", "brief-h1.md", "CLAUDE.md", ".claude/agents/a.md", ".claude/commands/")
	setHireFolder(t, l.projects)
	l.profile(t, l.gm)
	profile, _ := l.profile(t, l.hire)
	const no = "PermissionError"
	l.run(t, profile, l.hireTree, append(startFolderSteps(l.projects), probeStep{"write", l.hireTree + "/own.txt", "ok"}))
	ownerMade(t, l.projects, ".git/")
	l.run(t, profile, l.hireTree, []probeStep{
		{"write", l.projects + "/.git/HEAD", no}, {"mkdir", l.projects + "/.git/objects", no},
		{"mkdir", l.projects + "/.git/refs", no}, {"write", l.projects + "/.git/config", no},
	})
	if data, _ := os.ReadFile(l.projects + "/PLAYBOOK.md"); string(data) != "the owner's\n" {
		t.Errorf("the GM's playbook is now %q", data)
	}
}

// homeGitTries is a hire's try at a git folder at home, run by /bin/sh in its sandbox: $1 the home folder, $2 a script
// of its own for git to run, $3 the marker the script writes. Each step says "ok" or "refused".
const homeGitTries = `t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; }
printf '#!/bin/sh\necho planted > "%s"\ncat\n' "$3" > "$2" && chmod 755 "$2" || exit 3
t mkdir "$1/.git"
t sh -c 'printf "ref: refs/heads/main\n" > "$0/.git/HEAD"' "$1"
t mkdir "$1/.git/objects"
t mkdir "$1/.git/refs"
t sh -c 'printf "[core]\n\tfsmonitor = %s\n\tpager = %s\n" "$1" "$1" > "$0/.git/config"' "$1" "$2"
t ln -s "$2" "$1/.git.link"
t ln -s /nowhere "$1/.git"
t sh -c 'printf "ref: refs/heads/main\n" > "$0/HEAD"' "$1"
t mkdir "$1/objects"
t mkdir "$1/refs"
t sh -c 'printf "[core]\n\tbare = false\n\tworktree = %s\n\tfsmonitor = %s\n\tpager = %s\n" "$0" "$1" "$1" > "$0/config"' "$1" "$2"`

// "Seen outside" in RT126FIXREVIEW-GMROOTR1: a hire makes no ~/.git (a folder, a link) and no ~/HEAD, so the owner's
// git in ~/Projects, which is in no repo, finds no repo above it and runs nothing of the hire's. The rest of ~ stays
// normal work (objects, refs and config there make no repo without a HEAD).
func TestNoDeskMakesAGitFolderAtHomeForTheOwnersGitToRun(t *testing.T) {
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire)
	script := filepath.Join(l.hireTree, "fsm.sh")
	cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c", homeGitTries, "sh", homeDir, script, l.planted))
	cmd.Dir, cmd.Env = l.hireTree, gmRootEnv()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("the hire's tries: %v\n%s", err, out.Bytes())
	}
	want := []string{"refused", "refused", "refused", "refused", "refused", "ok", "refused", "refused", "ok", "ok", "ok"}
	if got := strings.Fields(out.String()); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("mkdir ~/.git, its HEAD, objects, refs, config; a link beside it, a link at it; ~/HEAD, objects, refs, config:\n got %q\nwant %q", got, want)
	}
	// The owner's git in ~/Projects, outside any sandbox, its search kept to the stand-in home.
	for _, args := range [][]string{{"status", "--porcelain"}, {"rev-parse", "--git-dir"}} {
		git := disclaim.Cmd(exec.Command("git", args...))
		git.Dir, git.Env = l.projects, append(gmRootEnv(), "GIT_CEILING_DIRECTORIES="+l.base)
		said, err := git.CombinedOutput()
		if err == nil || !strings.Contains(string(said), "not a git repository") {
			t.Errorf("the owner's git %s in ~/Projects took a repo above it: %v\n%s", args[0], err, said)
		}
	}
	if _, err := os.Lstat(l.planted); err == nil {
		t.Error("the owner's git in ~/Projects ran the hire's program, outside the sandbox")
	}
}
