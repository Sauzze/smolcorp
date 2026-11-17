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

func TestACarriedWorktreeIsRegisteredAsTheReposOwnGitWouldRegisterIt(t *testing.T) {
	for _, setting := range []string{"true", "false", ""} {
		for _, support := range []string{"as the tests have it", "deeper", "removed after the carry"} {
			t.Run("useRelativePaths "+map[string]string{"": "not set"}[setting]+setting+", the core's folder "+support, func(t *testing.T) {
				isolate(t)
				// The test's own git and the core's read no system or user config: the setting is the repo's alone.
				t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
				t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
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
				relative := setting == "true"
				if filepath.IsAbs(strings.TrimSpace(gitdir)) == relative || filepath.IsAbs(strings.TrimSpace(strings.TrimPrefix(dotGit, "gitdir: "))) == relative {
					t.Errorf("relative = %v, and the core wrote gitdir %q, .git %q", relative, gitdir, dotGit)
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
