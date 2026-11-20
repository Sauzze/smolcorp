//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXRELPATHS (RT126GMSCOPEPINR2 F2, SIERRA's ruling): the worktree of a hire the core makes in a git folder of its own
// is carried into the repo registered as the repo's own git registers one, in the form the owner chose
// (worktree.useRelativePaths): relative between the real places when it is on, whole when it is off or not set. Never
// relative to the core's folder, which is elsewhere, at any depth, and removed once the carry is done.
// FXRELPATHSR1: "as the repo's own git" holds for the git at hand. Git reads the key only from 2.48 on, and an older
// git (2.43 on Ubuntu 24.04) writes whole paths whatever it says. So the form each row wants is the one a plain
// `worktree add` of the test's own git writes with the same setting, in a stand-in repo, before the hire is made.

// relpathsBlock is what `worktree list --porcelain` says of the worktree at tree ("" when it isn't listed).
func relpathsBlock(list, tree string) string {
	for _, block := range strings.Split(list, "\n\n") {
		if strings.HasPrefix(block, "worktree "+tree+"\n") {
			return block
		}
	}
	return ""
}

func relpathsRead(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// relpathsControl says whether the test's own git writes a worktree's two links relative when the repo's
// worktree.useRelativePaths is setting ("" for not set): a plain `worktree add` in a stand-in repo, in the test's
// environment. It fails if git wrote one link relative and not the other, or a form its version can't have chosen:
// relative exactly when the setting is true and git is 2.48 or later.
func relpathsControl(t *testing.T, setting string) bool {
	t.Helper()
	repo := placeRepo(t)
	if setting != "" {
		gitOut(t, repo, "config", "worktree.useRelativePaths", setting)
	}
	tree := filepath.Join(filepath.Dir(repo), "control")
	gitOut(t, repo, "worktree", "add", "-q", "-b", "control", tree)
	gitdir := relpathsRead(t, filepath.Join(repo, ".git", "worktrees", "control", "gitdir"))
	dotGit := relpathsRead(t, filepath.Join(tree, ".git"))
	relative := !filepath.IsAbs(strings.TrimSpace(gitdir))
	if filepath.IsAbs(strings.TrimSpace(strings.TrimPrefix(dotGit, "gitdir: "))) == relative {
		t.Fatalf("the control's git wrote gitdir %q and .git %q, one relative and one not", gitdir, dotGit)
	}
	version := gitOut(t, repo, "version")
	if want := setting == "true" && gitVersionAtLeast(version, 2, 48); relative != want {
		t.Fatalf("the control's git (%s) wrote relative = %v with useRelativePaths %q, want %v", version, relative, setting, want)
	}
	if setting == "true" && !relative {
		t.Logf("%s doesn't read worktree.useRelativePaths (2.48 and later only): its own worktree add writes whole paths, so whole paths are wanted", version)
	}
	return relative
}

func TestACarriedWorktreeIsRegisteredAsTheReposOwnGitWouldRegisterIt(t *testing.T) {
	for _, setting := range []string{"true", "false", ""} {
		for _, support := range []string{"as the tests have it", "deeper", "removed after the carry"} {
			t.Run("useRelativePaths "+map[string]string{"": "not set"}[setting]+setting+", the core's folder "+support, func(t *testing.T) {
				isolate(t)
				// The test's own git and the core's read no system or user config: the setting is the repo's alone.
				t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
				t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
				relative := relpathsControl(t, setting)
				if support == "deeper" {
					supportDir = filepath.Join(supportDir, "a", "b", "c", "d", "e", "f", "g")
				}
				repo := placeRepo(t)
				if setting != "" {
					gitOut(t, repo, "config", "worktree.useRelativePaths", setting)
				}
				place, err := prepareHire(hireRequest{HireID: "abcdef123456", Name: "janitor", Folder: repo, Worktree: true}, placeOptions(t))
				if err != nil {
					t.Fatalf("prepareHire: %v", err)
				}
				if support == "removed after the carry" {
					if err := os.RemoveAll(gitReadsFolder()); err != nil {
						t.Fatal(err)
					}
				}
				registration := filepath.Join(repo, ".git", "worktrees", filepath.Base(place.Worktree))
				gitdir := relpathsRead(t, filepath.Join(registration, "gitdir"))
				dotGit := relpathsRead(t, filepath.Join(place.Worktree, ".git"))
				t.Logf("carried: gitdir %q, .git %q", gitdir, dotGit)

				block := relpathsBlock(gitOut(t, repo, "worktree", "list", "--porcelain"), place.Worktree)
				if block == "" || strings.Contains(block, "prunable") {
					t.Errorf("the repo's git lists the hire's worktree as %q, want it listed and not prunable", block)
				}
				if got := gitOut(t, place.Worktree, "rev-parse", "--path-format=absolute", "--git-common-dir"); got != filepath.Join(repo, ".git") {
					t.Errorf("the hire's worktree resolves to %s, want %s", got, filepath.Join(repo, ".git"))
				}
				if filepath.IsAbs(strings.TrimSpace(gitdir)) == relative || filepath.IsAbs(strings.TrimSpace(strings.TrimPrefix(dotGit, "gitdir: "))) == relative {
					t.Errorf("the control's git wrote relative = %v, and the core wrote gitdir %q, .git %q", relative, gitdir, dotGit)
				}

				// Byte for byte as the repo's own git writes them at that same place: it removes the worktree and makes
				// it again there, under the same registration name.
				gitOut(t, repo, "worktree", "remove", "--force", place.Worktree)
				gitOut(t, repo, "worktree", "add", "-q", "-b", "control", place.Worktree)
				if want := relpathsRead(t, filepath.Join(registration, "gitdir")); gitdir != want {
					t.Errorf("the carried gitdir is %q, git's own %q", gitdir, want)
				}
				if want := relpathsRead(t, filepath.Join(place.Worktree, ".git")); dotGit != want {
					t.Errorf("the carried .git is %q, git's own %q", dotGit, want)
				}
			})
		}
	}
}
