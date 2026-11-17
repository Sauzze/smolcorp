//go:build !windows

package desktop

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXRELPATHS (RT126GMSCOPEPINR2 F2, SIERRA's ruling, part 2): a scoped GM's hire in a project whose git writes
// relative paths is made and held like any other, and a scoped GM's hire that is refused once its worktree is made
// leaves no hire/* branch and no worktree of its own in the project, whatever form the project's git writes.

// relpathsLeft is what a refused hire left in project, which had trees worktrees before it ("" for nothing).
func relpathsLeft(t *testing.T, project string, trees int) string {
	t.Helper()
	branches := strings.TrimSpace(gitOut(t, project, "branch", "--list", "hire/*"))
	list := gitOut(t, project, "worktree", "list", "--porcelain")
	if branches != "" || strings.Count(list, "worktree ") != trees {
		return fmt.Sprintf("branches %q, worktrees %q", branches, list)
	}
	return ""
}

// relpathsAfterAdd runs swap(path) just after the hire's `worktree add` returns, path the worktree git was asked for;
// silent: then the add is answered as a git that didn't answer (stopped for taking too long, once it made the tree).
func relpathsAfterAdd(t *testing.T, d *Deck, silent bool, swap func(path string)) *bool {
	t.Helper()
	git, done := d.engine.Git, new(bool)
	if git == nil {
		git = runGit
	}
	d.engine.Git = func(dir string, args ...string) (int, string, bool) {
		status, out, ok := git(dir, args...)
		if !*done && len(args) > 4 && args[0] == "worktree" && args[1] == "add" {
			*done = true
			swap(args[4])
			if silent {
				return 0, "", false
			}
		}
		return status, out, ok
	}
	return done
}

func TestAScopedGMsHireInAProjectWithRelativePathsIsMadeAndHeld(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	gitOut(t, project, "config", "worktree.useRelativePaths", "true")
	code, text := rtHire(t, d, token, project)
	d.watching.Wait()
	if code != http.StatusOK {
		t.Fatalf("the hire: %d %q; left in the project: %s", code, text, relpathsLeft(t, project, 1))
	}
	var tree string
	for _, h := range registered(t) {
		if h.Role != gmRole {
			tree = h.Worktree
		}
	}
	if tree == "" || !(gmScope{root: project, git: scopeGit}).holds(tree) {
		t.Fatalf("the scope doesn't hold the hire's worktree %q", tree)
	}
	if block := relpathsBlock(gitOut(t, project, "worktree", "list", "--porcelain"), tree); block == "" || strings.Contains(block, "prunable") {
		t.Errorf("the project's git lists the hire's worktree as %q", block)
	}
	if data, err := os.ReadFile(filepath.Join(tree, ".git")); err != nil || !strings.HasPrefix(string(data), "gitdir: ../") {
		t.Errorf("the hire's .git is %q (%v), want it relative as the project's git writes it", data, err)
	}
}

func TestAScopedGMsHireRefusedOnceItsWorktreeIsMadeLeavesNothingInTheProject(t *testing.T) {
	cases := []struct {
		name           string
		fromWorktree   bool // the hire is asked for in a worktree of the project, beside it
		ownCommit      bool // that worktree is on a commit the project's HEAD doesn't have
		swapAsked      bool // that worktree is swapped (D3), else the hire's own new worktree
		silent         bool // git didn't answer the add (discardHalfMade), else layer 4 refuses it
		wantStatusCode int
	}{
		{"the folder asked for swapped for a link (D3)", true, false, true, false, http.StatusForbidden},
		{"the folder asked for, on a commit of its own, swapped for a link (D3)", true, true, true, false, http.StatusForbidden},
		{"the hire's own worktree swapped for a link", false, false, false, false, http.StatusForbidden},
		{"the hire's own worktree swapped for a link, from a worktree on a commit of its own", true, true, false, false, http.StatusForbidden},
		{"the hire's own worktree swapped for a link, and git didn't answer", false, false, false, true, http.StatusConflict},
	}
	for _, setting := range []string{"true", ""} {
		for _, c := range cases {
			t.Run(c.name+", useRelativePaths "+map[string]string{"": "not set"}[setting]+setting, func(t *testing.T) {
				d, _, _, token, project := scopedGM(t)
				if setting != "" {
					gitOut(t, project, "config", "worktree.useRelativePaths", setting)
				}
				other := placeRepo(t)
				folder, trees := project, 1
				if c.fromWorktree {
					folder, trees = rtBeside(t, project), 2
					if c.ownCommit {
						gitOut(t, folder, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "two")
					}
				}
				swapped := relpathsAfterAdd(t, d, c.silent, func(path string) {
					if c.swapAsked {
						path = folder
					}
					rtSwap(t, path, other)()
				})
				code, text := rtHire(t, d, token, folder)
				d.watching.Wait()
				if !*swapped {
					t.Fatalf("no worktree was made: %d %q", code, text)
				}
				if code != c.wantStatusCode {
					t.Errorf("answered %d %q, want %d", code, text, c.wantStatusCode)
				}
				if got := rtInOther(t, other); got != "" {
					t.Errorf("in the other repo: %s", got)
				}
				if got := relpathsLeft(t, project, trees); got != "" {
					t.Errorf("the refused hire (%d %q) left in the project: %s", code, text, got)
				}
				for _, h := range registered(t) {
					if h.Role != gmRole {
						t.Errorf("a row: %+v", h)
					}
				}
			})
		}
	}
}
