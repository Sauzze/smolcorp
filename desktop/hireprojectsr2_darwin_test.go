package desktop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXHIREPROJECTSR2 (FXHIREPROJECTSR1's Open 1, 3 and 4) under the real profile, with stand-ins only: no desk makes a
// .git (a folder, a gitfile, a link, one renamed in) or a bare repo's HEAD in a folder above its place, the folder for
// hires or the GM's start folder, so the owner's git (or the GM's) in a folder below it in no repo takes nothing of the
// desk's; a GM's start folder that holds a hire's worktree is held as the folder for hires is; and a desk's own git
// work, and the owner's repos and worktrees, run as before.

// gitTries is a desk's other ways to a .git or a HEAD in a folder, run by /bin/sh in its sandbox: $1 the folder, $2 a
// folder of its own to make a repo's makings in. Each try says "ok" or "refused", and what one made is taken away
// before the next, so each shows for itself.
const gitTries = `F=$1 O=$2
t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; rm -rf "${F:?}/.git" "${F:?}/HEAD" 2>/dev/null; true; }
plant() { rm -rf "${O:?}/plant" "${O:?}/plant-HEAD"; mkdir -p "$O/plant/objects" "$O/plant/refs" &&
  printf 'ref: refs/heads/main\n' > "$O/plant/HEAD" && printf 'ref: refs/heads/main\n' > "$O/plant-HEAD" || exit 3; }
plant; t sh -c 'printf "gitdir: %s\n" "$1" > "$0/.git"' "$F" "$O/plant"
plant; t mv "$O/plant" "$F/.git"
plant; t ln -s "$O/plant" "$F/.git"
plant; t mv "$O/plant-HEAD" "$F/HEAD"
plant; t ln -s "$O/plant-HEAD" "$F/HEAD"
rm -rf "${O:?}/plant" "${O:?}/plant-HEAD"`

// sandboxedSh runs script by /bin/sh under profile in dir, with args, and gives what it said, one word a try.
func sandboxedSh(t *testing.T, profile, dir, script string, args ...string) []string {
	t.Helper()
	cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", append([]string{"-f", profile, "/bin/sh", "-c", script, "sh"}, args...)...))
	cmd.Dir, cmd.Env = dir, gmRootEnv()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("the desk's tries: %v\n%s", err, out.Bytes())
	}
	return strings.Fields(out.String())
}

// refusedAGitFolderAt runs the desk's tries at folder and wants each refused but what rest says ("ok" or "refused") of
// homeGitTries' link beside .git and a bare repo's objects, refs and config. Between homeGitTries and gitTries, the
// owner's git in below, a folder in no repo under folder, takes no repo and runs nothing (theOwnersGitTakesNoRepoIn).
func refusedAGitFolderAt(t *testing.T, l gmRootLayout, profile, own, folder, rest, below string) {
	t.Helper()
	got := sandboxedSh(t, profile, own, homeGitTries, folder, filepath.Join(own, "fsm.sh"), l.planted)
	want := []string{"refused", "refused", "refused", "refused", "refused", rest, "refused", "refused", rest, rest, rest}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("at %s: mkdir .git, its HEAD, objects, refs, config; a link beside it, a link at it; HEAD, objects, refs, config:\n got %q\nwant %q",
			strings.TrimPrefix(folder, l.base), got, want)
	}
	theOwnersGitTakesNoRepoIn(t, l, below)
	got = sandboxedSh(t, profile, own, gitTries, folder, own)
	if strings.Join(got, " ") != strings.TrimSpace(strings.Repeat("refused ", 5)) {
		t.Errorf("at %s: a gitfile, a .git renamed in, a link at .git, a HEAD renamed in, a link at HEAD: %q, want each refused",
			strings.TrimPrefix(folder, l.base), got)
	}
}

// theOwnersGitTakesNoRepoIn wants the owner's git in dir, outside any sandbox and its search kept to the stand-in
// base, to find no repo, and nothing the desk planted to have run.
func theOwnersGitTakesNoRepoIn(t *testing.T, l gmRootLayout, dir string) {
	t.Helper()
	for _, args := range [][]string{{"status", "--porcelain"}, {"rev-parse", "--git-dir"}} {
		git := disclaim.Cmd(exec.Command("git", args...))
		git.Dir, git.Env = dir, append(gmRootEnv(), "GIT_CEILING_DIRECTORIES="+l.base)
		said, err := git.CombinedOutput()
		if err == nil || !strings.Contains(string(said), "not a git repository") {
			t.Errorf("the owner's git %s in %s took a repo above it: %v\n%s", args[0], strings.TrimPrefix(dir, l.base), err, said)
		}
	}
	if _, err := os.Lstat(l.planted); err == nil {
		t.Errorf("the owner's git in %s ran the desk's program, outside the sandbox", strings.TrimPrefix(dir, l.base))
	}
}

// codeLayout is Open 3's: the owner's repo ~/Code/clients/app, a hire in its worktree ~/Code/clients/app-hires/h3, and
// ~/Code/clients the folder for hires. Only the hire is open.
func codeLayout(t *testing.T) (gmRootLayout, Hire, string) {
	t.Helper()
	l := newGMRootLayout(t)
	clients := filepath.Join(homeDir, "Code", "clients")
	app := filepath.Join(clients, "app")
	tree := filepath.Join(hiresFolder(app, ""), "h3")
	ownerMade(t, app, "README")
	standInOwnersGit(t, app, "init", "-q", "-b", "main")
	standInOwnersGit(t, app, "add", "README")
	standInOwnersGit(t, app, "commit", "-q", "-m", "first")
	standInOwnersGit(t, app, "worktree", "add", "-q", "-b", "hire/h3", tree)
	h := Hire{ID: "h3", Name: "Janitor", Role: "janitor", Folder: keptPath(app), Repo: app, Worktree: keptPath(tree), Branch: "hire/h3", State: "working"}
	writeRegistryHires(t, h)
	setHireFolder(t, clients)
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	return l, h, tree
}

// Open 3: a hire in ~/Code/clients makes no ~/Code/.git, ~/Code/HEAD, nor one in the folder for hires; ~/Code's other
// files stay normal work. The owner's git in ~/Code/clients, in no repo, takes nothing of the hire's.
func TestAHireMakesNoGitFolderBetweenHomeAndItsPlace(t *testing.T) {
	l, h, tree := codeLayout(t)
	profile, s := l.profile(t, h)
	code, clients := filepath.Join(homeDir, "Code"), filepath.Join(homeDir, "Code", "clients")
	if s.places.Projects != clients {
		t.Fatalf("the hire's Places give the folder for hires as %q", s.places.Projects)
	}
	refusedAGitFolderAt(t, l, profile, tree, code, "ok", clients)
	refusedAGitFolderAt(t, l, profile, tree, clients, "refused", clients)
	l.run(t, profile, tree, []probeStep{
		{"mkdir", code + "/.git", "PermissionError"}, {"write", code + "/HEAD", "PermissionError"},
		{"write", code + "/notes.md", "ok"}, {"mkdir", code + "/pkg", "ok"}, {"write", tree + "/own.txt", "ok"},
	})
}

// Open 4, a plain hire whose place is the folder for hires, doesn't start on a Mac since FXHIREPROJECTSR4
// (hireprojectsr4_darwin_test.go): no row here makes it its own.

// workLayout is Open 1's: a GM with a workspace hired into ~/Work, a folder of repos that isn't the folder for hires
// (~/Projects); the owner's playbook, a brief, a CLAUDE.md and a .claude there; a hire in ~/Work/app-hires/h5.
func workLayout(t *testing.T) (gmRootLayout, Hire, string, string) {
	t.Helper()
	l := newGMRootLayout(t)
	work := filepath.Join(homeDir, "Work")
	app := filepath.Join(work, "app")
	tree := filepath.Join(hiresFolder(app, ""), "h5")
	ownerMade(t, work, "PLAYBOOK.md", "brief-h1.md", "CLAUDE.md", ".claude/agents/a.md", ".claude/commands/")
	ownerMade(t, app, "README")
	standInOwnersGit(t, app, "init", "-q", "-b", "main")
	standInOwnersGit(t, app, "add", "README")
	standInOwnersGit(t, app, "commit", "-q", "-m", "first")
	standInOwnersGit(t, app, "worktree", "add", "-q", "-b", "hire/h5", tree)
	l.gm.Folder = keptPath(work)
	h := Hire{ID: "h5", Name: "Janitor", Role: "janitor", Folder: keptPath(app), Repo: app, Worktree: keptPath(tree), Branch: "hire/h5", State: "working"}
	writeRegistryHires(t, l.gm, h)
	setHireFolder(t, l.projects)
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := makeDeskPlans(work, "gm1"); err != nil {
		t.Fatal(err)
	}
	return l, h, work, tree
}

// Open 1: a hire under that GM writes nothing the GM loads at the top of ~/Work (its playbook and briefs, a .claude's
// contents, a .git, a HEAD), nor the other repos there; it writes its own worktree. The GM's git in ~/Work takes no repo.
func TestAHireUnderAGMInAFolderOfReposMakesNothingThereTheGMLoads(t *testing.T) {
	l, h, work, tree := workLayout(t)
	l.profile(t, l.gm)
	profile, s := l.profile(t, h)
	if s.places.GMStart != work {
		t.Fatalf("the hire's Places give the GM's start folder as %q", s.places.GMStart)
	}
	refusedAGitFolderAt(t, l, profile, tree, work, "refused", work)
	const no = "PermissionError"
	l.run(t, profile, tree, append(startFolderSteps(work),
		probeStep{"write", work + "/.smolcorp/plans/gm1/p.md", no}, probeStep{"mkdir", work + "/lib", no},
		probeStep{"write", work + "/app/README", no}, probeStep{"hook", work + "/app/.git/hooks/post-checkout", no},
		probeStep{"write", tree + "/own.txt", "ok"}, probeStep{"mkdir", tree + "/pkg", "ok"}))
	if data, _ := os.ReadFile(work + "/PLAYBOOK.md"); string(data) != "the owner's\n" {
		t.Errorf("the GM's playbook is now %q", data)
	}
}

// ownGitWork is a desk's git work in its own place, run by /bin/sh in its sandbox: a commit on its branch, then a repo
// of its own tried below its place, which since FXHIREPROJECTSR6 is refused (no desk makes a .git below its place:
// rowGitBelow). $1 is a name for the commit.
const ownGitWork = `set -e
c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false -c init.defaultBranch=main"
echo own > own.txt
/usr/bin/git $c add own.txt
/usr/bin/git $c commit -q -m "$1"
mkdir sub
if (cd sub && /usr/bin/git $c init -q) 2>/dev/null; then echo made; else echo refused; fi
/usr/bin/git log --format=%s -1`

// The controls: a desk's own git work still runs under the held folders above it, in each layout and with its worktree
// inside the owner's checkout (whose .git is the desk's shared .git); and the owner's repos and worktrees find their own
// .git first, as before.
func TestADesksOwnGitWorkStillRunsUnderTheHeldFolders(t *testing.T) {
	t.Run("a hire in ~/Code/clients", func(t *testing.T) {
		l, h, tree := codeLayout(t)
		profile, _ := l.profile(t, h)
		if got := sandboxedSh(t, profile, tree, ownGitWork, "own"); strings.Join(got, " ") != "refused own" {
			t.Errorf("the hire's commit, and its own repo refused: %q", got)
		}
		app := filepath.Join(homeDir, "Code", "clients", "app")
		for dir, want := range map[string]string{app: ".git", tree: filepath.Join(app, ".git", "worktrees", "h3")} {
			if got := strings.TrimSpace(standInOwnersGit(t, dir, "rev-parse", "--git-dir")); got != want {
				t.Errorf("the owner's git in %s finds %q, want %q", strings.TrimPrefix(dir, l.base), got, want)
			}
		}
		if got := strings.TrimSpace(standInOwnersGit(t, app, "log", "--format=%s", "-1", "hire/h3")); got != "own" {
			t.Errorf("the owner's git reads the hire's branch as %q", got)
		}
	})
	t.Run("its worktree inside the owner's checkout", func(t *testing.T) {
		l := newGMRootLayout(t)
		tree := filepath.Join(hiresFolder(l.app, l.app), "h6")
		standInOwnersGit(t, l.app, "worktree", "add", "-q", "-b", "hire/h6", tree)
		h := Hire{ID: "h6", Name: "Janitor", Role: "janitor", Folder: keptPath(l.app), Repo: l.app, Worktree: keptPath(tree), Branch: "hire/h6", State: "working"}
		writeRegistryHires(t, h)
		setHireFolder(t, l.projects)
		if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
			t.Fatal(err)
		}
		s, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit, Worktrees: l.app}))
		if err != nil {
			t.Fatal(err)
		}
		profile := filepath.Join(hireFolder(supportDir, h.ID), "sandbox.sb")
		if err := sandbox.PrepareProfile(s.policy, profile, deskServices(s.places, deskSocketPath(h.ID, 0))); err != nil {
			t.Fatal(err)
		}
		if got := sandboxedSh(t, profile, tree, ownGitWork, "own"); strings.Join(got, " ") != "refused own" {
			t.Errorf("the hire's commit, and its own repo refused: %q", got)
		}
	})
	t.Run("a hire under a GM in ~/Work", func(t *testing.T) {
		l, h, _, tree := workLayout(t)
		l.profile(t, l.gm)
		profile, _ := l.profile(t, h)
		if got := sandboxedSh(t, profile, tree, ownGitWork, "own"); strings.Join(got, " ") != "refused own" {
			t.Errorf("the hire's commit, and its own repo refused: %q", got)
		}
	})
}
