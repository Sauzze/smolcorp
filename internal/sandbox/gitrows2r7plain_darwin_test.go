package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGITROWS2R7 (RT126FIXREVIEW-GITROWS2R6's N1, ruled: no new opening): a Mac plain hire, a Codex, Gemini CLI, OpenCode
// or Grok one with its Places as newLauncherSandbox gives them (its folder as Root, no Checkout, no branch of its own
// there), changes files but can't commit:
//   - at the project's main checkout: its index and HEAD are the shared git places' (true on the signed base too);
//   - in an owner's linked worktree on the owner's branch: that branch is no desk's to move (rowNotOwnBranches).
//
// The owner's branch stays where it was, and the hire's change waits in the folder for the owner to commit. It uses
// only newGitDesk's stand-ins, so it runs on the signed base as it is.
func TestOnAMacAPlainHireChangesFilesButCantCommit(t *testing.T) {
	git := gitOrSkip(t)
	sandboxExecOrSkip(t)
	for _, inWorktree := range []bool{false, true} {
		name, branch := "at the project's main checkout", "main"
		if inWorktree {
			name, branch = "in an owner's linked worktree", "review"
		}
		t.Run(name, func(t *testing.T) {
			d, owner := newGitDesk(t, git)
			owner(d.checkout, "worktree", "remove", "--force", d.root)
			d.root, d.pl.OwnWorktree, d.pl.OtherWorktrees = d.checkout, "", []string{"h2"}
			if inWorktree {
				d.root = filepath.Join(d.home, "app-review")
				owner(d.checkout, "worktree", "add", "-q", "-b", "review", d.root)
				d.pl.OwnWorktree = "app-review"
			}
			d.pl.Root, d.pl.Checkout = d.root, ""
			d.prepare(t)
			was := owner(d.checkout, "rev-parse", branch)
			if r := d.inside(t, nil, "/bin/sh", "-c", "echo the hire\\'s change >> README"); r.status != 0 {
				t.Fatalf("the hire's change to README: %+v", r)
			}
			if data, _ := os.ReadFile(filepath.Join(d.root, "README")); !strings.Contains(string(data), "the hire's change") {
				t.Errorf("README after the hire's change: %q", data)
			}
			if r := d.inside(t, gitTestEnv(git), git, "commit", "-q", "-a", "-m", "the hire's"); r.status == 0 {
				t.Errorf("the hire committed on %s: %+v", branch, r)
			} else {
				t.Logf("the hire's commit: exit %d, %s", r.status, strings.TrimSpace(r.stderr))
			}
			if now := owner(d.checkout, "rev-parse", branch); now != was {
				t.Errorf("%s moved: %s, was %s", branch, now, was)
			}
			if status := owner(d.root, "status", "--porcelain", "--untracked-files=no"); !strings.Contains(status, "README") {
				t.Errorf("the hire's change isn't left for the owner: %q", status)
			}
		})
	}
}
