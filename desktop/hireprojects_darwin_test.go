package desktop

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXHIREPROJECTS (RT126FIXREVIEW-GMROOT's sibling repo, a 1.26.0 release blocker; SIERRA's option (a)) under the real
// profile, with stand-ins only: FXGMROOT's default layout (gmroot_darwin_test.go), the folder for hires set in Setup,
// and no GM open. A hire in app-hires/h1 writes its own worktree and its repo's shared .git, and none of the sibling
// repo lib: its hooks, its config, its sources. Nor a repo made there after its start. The owner's `git checkout` in
// lib runs no hook the hire tried to plant.

// newHireProjectsLayout is the default layout with the folder for hires set in Setup and only the hire open.
func newHireProjectsLayout(t *testing.T) gmRootLayout {
	t.Helper()
	l := newGMRootLayout(t)
	writeRegistryHires(t, l.hire)
	setHireFolder(t, l.projects)
	standInOwnersGit(t, l.lib, "branch", "develop")
	return l
}

// setHireFolder saves folder as Setup's folder for hires, in this test's own config.
func setHireFolder(t *testing.T, folder string) {
	t.Helper()
	if err := os.WriteFile(configFile, []byte(`{"hireFolder":`+jsonText(folder)+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

// siblingSteps are what a desk in the projects folder is refused in repo, one not its own, and still reads there.
func siblingSteps(repo string) []probeStep {
	const no = "PermissionError"
	return []probeStep{
		{"hook", repo + "/.git/hooks/post-checkout", no}, {"write", repo + "/.git/hooks/pre-commit", no},
		{"write", repo + "/.git/config", no}, {"write", repo + "/.git/refs/heads/main", no},
		{"write", repo + "/README", no}, {"write", repo + "/new.go", no}, {"mkdir", repo + "/pkg", no},
		{"read", repo + "/README", "ok"}, {"read", repo + "/.git/config", "ok"},
	}
}

func TestAHireIsRefusedASiblingReposGitPlacesAndSources(t *testing.T) {
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire)
	const no = "PermissionError"
	steps := append(siblingSteps(l.lib),
		probeStep{"mkdir", l.projects + "/new-project", no}, probeStep{"write", l.projects + "/notes.md", no},
		probeStep{"rename", l.lib, no}, probeStep{"chmod", l.projects, no}, probeStep{"rename", l.projects, no},
		// Its own worktree stays its own.
		probeStep{"write", l.hireTree + "/own.txt", "ok"}, probeStep{"mkdir", l.hireTree + "/pkg", "ok"})
	l.run(t, profile, l.hireTree, steps)
	if data, _ := os.ReadFile(l.lib + "/README"); string(data) != "the owner's\n" {
		t.Errorf("the owner's file in lib is now %q", data)
	}
}

func TestARepoMadeInTheProjectsFolderAfterAHiresStartIsRefusedToIt(t *testing.T) {
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire) // its start
	later := filepath.Join(l.projects, "later")
	if err := os.MkdirAll(later, 0o700); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, later, "init", "-q", "-b", "main")
	if err := os.WriteFile(later+"/README", []byte("the owner's\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, later, "add", "README")
	standInOwnersGit(t, later, "commit", "-q", "-m", "first")
	l.run(t, profile, l.hireTree, siblingSteps(later))
}

// The reviewer's repro (TestZZReviewSiblingRepo) as a row: the hire plants a post-checkout hook in lib, then the owner
// checks out a branch there, outside any sandbox. The hook isn't there, and nothing planted runs.
func TestTheOwnersCheckoutInASiblingRepoRunsNoHookAHirePlanted(t *testing.T) {
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire)
	hook := l.lib + "/.git/hooks/post-checkout"
	l.run(t, profile, l.hireTree, []probeStep{{"hook", hook, "PermissionError"}})
	standInOwnersGit(t, l.lib, "checkout", "-q", "develop")
	if _, err := os.Lstat(l.planted); !errors.Is(err, os.ErrNotExist) {
		t.Error("the owner's git checkout in lib ran a hook the hire planted, outside the sandbox")
	}
	if _, err := os.Lstat(hook); !errors.Is(err, os.ErrNotExist) {
		t.Error("the hire's hook is in lib's hooks folder")
	}
}

// The control: in its own worktree the hire still commits, fetches and rebases, all of which write its repo's shared
// .git (objects, its branch, FETCH_HEAD, its registration's rebase state).
func TestAHireStillCommitsFetchesAndRebasesInItsOwnWorktree(t *testing.T) {
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire)
	// The owner's new commit on main, outside any sandbox, for the hire to rebase onto.
	if err := os.WriteFile(l.app+"/CHANGES", []byte("the owner's\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, l.app, "add", "CHANGES")
	standInOwnersGit(t, l.app, "commit", "-q", "-m", "owner's")
	script := `set -e
git=/usr/bin/git
c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false"
echo own > own.txt
$git $c add own.txt
$git $c commit -q -m own
$git $c fetch -q "$1" main
$git $c rebase -q main
$git log --format=%s -3`
	cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c", script, "sh", l.lib))
	cmd.Dir, cmd.Env = l.hireTree, gmRootEnv()
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("the hire's commit, fetch and rebase: %v\n%s", err, out.Bytes())
	}
	if got := strings.Fields(out.String()); len(got) < 3 || got[0] != "own" || got[1] != "owner's" {
		t.Errorf("the hire's branch after its rebase: %q", out.String())
	}
}

// FXHIREPROJECTS' cases (its brief's item 3), each under the real profile: what the desk writes, and the sibling repo
// it doesn't.
func TestEachLayoutOfTheProjectsFolderKeepsADeskToItsOwnPlaceUnderTheProfile(t *testing.T) {
	for _, c := range []struct {
		name string
		// desk lays its desk out in l, outside any sandbox, and gives its row, its options and where it works.
		desk func(t *testing.T, l gmRootLayout) (Hire, engineOptions, string)
		// more are its other steps.
		more func(l gmRootLayout, own string) []probeStep
	}{
		{"the hires folder inside the projects folder", func(t *testing.T, l gmRootLayout) (Hire, engineOptions, string) {
			return l.hire, engineOptions{Git: runGit}, l.hireTree
		}, func(l gmRootLayout, own string) []probeStep {
			return []probeStep{{"write", l.app + "/README", "PermissionError"}, {"write", l.app + "-hires/h2.txt", "PermissionError"}}
		}},
		{"its worktree outside the projects folder", func(t *testing.T, l gmRootLayout) (Hire, engineOptions, string) {
			worktrees := filepath.Join(filepath.Dir(l.projects), "Worktrees")
			tree := filepath.Join(hiresFolder(l.app, worktrees), "h3")
			standInOwnersGit(t, l.app, "worktree", "add", "-q", "-b", "hire/h3", tree)
			h := Hire{ID: "h3", Name: "Janitor", Role: "janitor", Folder: l.app, Repo: l.app, Worktree: tree, Branch: "hire/h3", State: "working"}
			return h, engineOptions{Git: runGit, Worktrees: worktrees}, tree
		}, func(l gmRootLayout, own string) []probeStep {
			return []probeStep{{"write", l.app + "/README", "PermissionError"}, {"write", l.hireTree + "/README", "PermissionError"},
				{"write", l.app + "/.git/refs/heads/hire/h3", "ok"},
				// Another name beside its own branch is another desk's, held since FXGITROWS2 (FXSTACKG G22).
				{"write", l.app + "/.git/refs/heads/hire/h3-more", "PermissionError"}}
		}},
		{"the projects folder is itself a repo, and the hire's", func(t *testing.T, l gmRootLayout) (Hire, engineOptions, string) {
			standInOwnersGit(t, l.projects, "init", "-q", "-b", "main")
			if err := os.WriteFile(l.projects+"/README", []byte("the owner's\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			standInOwnersGit(t, l.projects, "add", "README")
			standInOwnersGit(t, l.projects, "commit", "-q", "-m", "first")
			tree := filepath.Join(hiresFolder(l.projects, ""), "h4")
			standInOwnersGit(t, l.projects, "worktree", "add", "-q", "-b", "hire/h4", tree)
			h := Hire{ID: "h4", Name: "Janitor", Role: "janitor", Folder: l.projects, Repo: l.projects, Worktree: tree, Branch: "hire/h4", State: "working"}
			return h, engineOptions{Git: runGit}, tree
		}, func(l gmRootLayout, own string) []probeStep {
			return []probeStep{{"write", l.projects + "/README", "PermissionError"}, {"hook", l.projects + "/.git/hooks/post-checkout", "PermissionError"},
				{"write", l.projects + "/.git/refs/heads/hire/h4", "ok"},
				// Another name beside its own branch is another desk's, held since FXGITROWS2 (FXSTACKG G22).
				{"write", l.projects + "/.git/refs/heads/hire/h4-more", "PermissionError"}}
		}},
		{"the GM hired into a repo worktree", func(t *testing.T, l gmRootLayout) (Hire, engineOptions, string) {
			tree := filepath.Join(hiresFolder(l.app, ""), "gm")
			standInOwnersGit(t, l.app, "worktree", "add", "-q", "-b", "hire/gm1", tree)
			h := Hire{ID: "gm1", Name: "General Manager", Role: gmRole, Folder: l.app, Repo: l.app, Worktree: tree, Branch: "hire/gm1", State: "working"}
			return h, engineOptions{Git: runGit}, tree
		}, func(l gmRootLayout, own string) []probeStep {
			return []probeStep{{"write", l.hireTree + "/README", "PermissionError"}, {"write", l.app + "/README", "PermissionError"}}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := newHireProjectsLayout(t)
			h, o, own := c.desk(t, l)
			writeRegistryHires(t, h)
			if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
				t.Fatal(err)
			}
			s, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, o))
			if err != nil {
				t.Fatalf("%s's sandbox: %v", h.ID, err)
			}
			profile := filepath.Join(hireFolder(supportDir, h.ID), "sandbox.sb")
			if err := sandbox.PrepareProfile(s.policy, profile, deskServices(s.places, deskSocketPath(h.ID, 0))); err != nil {
				t.Fatal(err)
			}
			steps := append([]probeStep{{"write", own + "/own.txt", "ok"}, {"mkdir", own + "/pkg", "ok"}}, siblingSteps(l.lib)...)
			l.run(t, profile, own, append(steps, c.more(l, own)...))
		})
	}
}
