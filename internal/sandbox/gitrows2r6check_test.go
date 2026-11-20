//go:build unix

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// FXGITROWS2R6 (RT126FIXREVIEW-GITROWS2R5's B1 and N2) at the start, on a Mac desk h1 in its own worktree (refRows). It
// names no API of this round's, so it runs on the tree before it too.
//   - B1: a HEAD (the owner checkout's, another worktree's) whose way goes through a ref git reads per worktree is
//     refused, whatever that ref holds; and a HEAD the desk can't write with a second name is refused.
//   - N2: a symbolic ref at the rest of the shared .git that git takes before a branch is followed as a held ref is.
//
// Passed: an owner's commit message that starts "ref:", a FETCH_HEAD, git's own folders, a link at hooks.
func TestOnAMacAPerWorktreeWayAHardLinkedHEADOrANameAtTheTopStopsTheStart(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	const id = "1111111111111111111111111111111111111111\n"
	type file struct{ at, text string }
	symref := func(at, to string) file { return file{at, "ref: " + to + "\n"} }
	for _, c := range []struct {
		name         string
		files        []file
		hard         string // a file of files given a second name outside the .git
		atProject    bool
		at, to, move string // the refusal's Ref, Symref and Moves; at "" when the start goes on
		isHard       bool
	}{
		{"another worktree's HEAD, by way of refs/worktree", []file{symref("worktrees/review/HEAD", "refs/worktree/w"),
			symref("worktrees/review/refs/worktree/w", "refs/heads/hire/h1/work")}, "", false, "worktrees/review/HEAD", "refs/worktree/w", "", false},
		{"another worktree's HEAD, by way of refs/bisect", []file{symref("worktrees/review/HEAD", "refs/bisect/bad"),
			symref("worktrees/review/refs/bisect/bad", "refs/heads/hire/h1/work")}, "", false, "worktrees/review/HEAD", "refs/bisect/bad", "", false},
		{"another worktree's HEAD, by way of refs/rewritten, nothing there", []file{symref("worktrees/review/HEAD", "refs/rewritten/x")},
			"", false, "worktrees/review/HEAD", "refs/rewritten/x", "", false},
		{"the owner checkout's HEAD, by way of refs/worktree", []file{symref("HEAD", "refs/worktree/w")},
			"", false, "HEAD", "refs/worktree/w", "", false},
		{"another worktree's HEAD, a hard link", []file{symref("worktrees/review/HEAD", "refs/heads/review")},
			"worktrees/review/HEAD", false, "worktrees/review/HEAD", "", "", true},
		{"the owner checkout's HEAD, a hard link", []file{symref("HEAD", "refs/heads/topic/x")}, "HEAD", false, "HEAD", "", "", true},
		{"HEAD at the project, a hard link", []file{symref("HEAD", "refs/heads/topic/x")}, "HEAD", true, "HEAD", "", "", true},
		{"a name at the top, to the desk's own", []file{symref("feature", "refs/heads/hire/h1/work")},
			"", false, "feature", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work", false},
		{"a name in a folder at the top, to the desk's own", []file{symref("hire/h2/work", "refs/heads/hire/h1")},
			"", false, "hire/h2/work", "refs/heads/hire/h1", "refs/heads/hire/h1", false},
		{"a name at the top, to a remote-tracking branch", []file{symref("FEATURE_HEAD", "refs/remotes/origin/main")},
			"", false, "FEATURE_HEAD", "refs/remotes/origin/main", "refs/remotes/origin/main", false},
		{"a name at the top, to a checkout's HEAD", []file{symref("feature", "worktrees/h1/HEAD")}, "", false, "feature", "worktrees/h1/HEAD", "", false},
		// Passed.
		{"another worktree's HEAD, on its branch", []file{symref("worktrees/review/HEAD", "refs/heads/review")}, "", false, "", "", "", false},
		{"a name at the top, to the owner's branch", []file{symref("feature", "refs/heads/topic/x")}, "", false, "", "", "", false},
		{"an owner's commit message", []file{{"COMMIT_EDITMSG", "ref: #12 the owner's fix\n"}, {"MERGE_MSG", "ref:\n"}}, "", false, "", "", "", false},
		{"a fetch's and a reset's", []file{{"FETCH_HEAD", id}, {"ORIG_HEAD", id}}, "", false, "", "", "", false},
		{"git's own folders", []file{symref("rr-cache/ab/preimage", "refs/heads/hire/h1/work"), symref("modules/sub/HEAD", "refs/heads/hire/h1/work"),
			symref("lfs/tmp/x", "refs/heads/hire/h1/work")}, "", false, "", "", "", false},
		{"a ref of a desk's at the project", []file{symref("FETCH_HEAD", "refs/heads/hire/h1/work")}, "", true, "", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, outside := refRows(t)
			if c.atProject {
				pl.Root, pl.Checkout, pl.OwnWorktree, pl.CheckoutBranch = pl.Checkout, "", "", ""
			}
			for _, f := range c.files {
				at := filepath.Join(git, filepath.FromSlash(f.at))
				if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(at, []byte(f.text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if c.hard != "" {
				if err := os.Link(filepath.Join(git, filepath.FromSlash(c.hard)), filepath.Join(outside, "second")); err != nil {
					t.Fatal(err)
				}
			}
			err := mustNew(t, pl).CheckRefPlaces()
			var e *RefPlaceError
			switch {
			case c.at == "" && err != nil:
				t.Errorf("refused: %v", err)
			case c.at == "":
			case !errors.As(err, &e) || e.Ref != c.at || e.At != git+"/"+c.at || e.Symref != c.to || e.Moves != c.move || e.Link || e.Hard != c.isHard:
				t.Errorf("%v (%+v), want it refused at %s (to %q, moving %q, hard %v)", err, e, c.at, c.to, c.move, c.isHard)
			}
		})
	}
	t.Run("a link at hooks", func(t *testing.T) {
		pl, git, outside := refRows(t)
		// FXGITROWS2R8 (B1): a folder the desk can write would refuse it now; this one is held, as git's config names it.
		pl.GitHooks = []string{outside}
		if err := os.Symlink(outside, filepath.Join(git, "hooks")); err != nil {
			t.Fatal(err)
		}
		if err := mustNew(t, pl).CheckRefPlaces(); err != nil {
			t.Errorf("refused: %v", err)
		}
	})
}
