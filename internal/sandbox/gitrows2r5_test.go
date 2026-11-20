//go:build unix

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// FXGITROWS2R5 (RT126FIXREVIEW-GITROWS2R4's N1 and N2) in the Policy, on a Mac desk h1 in its own worktree. Its
// real-profile rows are in gitrows2r5_darwin_test.go.

// N1: refs/ and logs/refs/ whole as an allowlist. A desk writes only its own branches and the remote-tracking ones
// (origin/HEAD apart), with their locks and reflogs; every other ref is held by rowNotOwnRefs where no narrower row names
// it: refs/<name>, refs/hire/…, refs/stash, refs/notes, refs/replace (rowSubstitutions names it), any new folder.
// N2: every worktree's registration but its own, one added after the start included, by rowOtherWorktrees.
func TestOnAMacThePolicyHoldsEveryRefButADesksOwnBranchesAndTheRemoteTrackingOnes(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.CheckoutBranch, pl.OwnBranches = "topic/x", []string{"hire/ada"}
	p := mustNew(t, pl)
	for _, c := range []struct {
		at  string
		v   Verdict
		row string
	}{
		{"refs/feature", Deny, rowNotOwnRefs}, {"refs/feature.lock", Deny, rowNotOwnRefs}, {"refs/hire/h2/work", Deny, rowNotOwnRefs},
		{"refs/hire/h1/work", Deny, rowNotOwnRefs}, {"refs/hire", Deny, rowNotOwnRefs}, {"refs/stash", Deny, rowNotOwnRefs},
		{"refs/stash.lock", Deny, rowNotOwnRefs}, {"logs/refs/stash", Deny, rowNotOwnRefs}, {"refs/notes/commits", Deny, rowNotOwnRefs},
		{"refs/notes", Deny, rowNotOwnRefs}, {"logs/refs/notes/commits", Deny, rowNotOwnRefs}, {"refs/prefetch/remotes/origin/main", Deny, rowNotOwnRefs},
		{"refs/bisect/bad", Deny, rowNotOwnRefs}, {"refs/new/x", Deny, rowNotOwnRefs}, {"REFS/Feature", Deny, rowNotOwnRefs},
		{"logs/refs/.tmp-renamed-log", Deny, rowNotOwnRefs}, {"logs/refs", Deny, rowNotOwnRefs},
		{"refs/replace/0123456789abcdef0123456789abcdef01234567", Deny, rowSubstitutions}, {"refs/replace", Deny, rowSubstitutions},
		{"refs/remotes/origin/HEAD", Deny, rowDefaultName}, {"refs/heads/feature", Deny, rowNotOwnBranches}, {"refs/tags/v1", Deny, rowTags},
		{"worktrees/later/HEAD", Deny, rowOtherWorktrees}, {"worktrees/later/index", Deny, rowOtherWorktrees},
		{"worktrees/later/logs/HEAD", Deny, rowOtherWorktrees}, {"worktrees/h2/HEAD", Deny, "other desks' places: their worktree's registration"},
		{"worktrees/H1x/HEAD", Deny, rowOtherWorktrees}, {"worktrees", Deny, rowOtherWorktrees},
		{"worktrees/h1/commondir", Deny, "the shared git places: where a worktree finds them"},
		// The desk's own.
		{"refs/heads/hire/h1/work", Allow, ""}, {"refs/heads/hire/h1/work.lock", Allow, ""}, {"refs/heads/hire/h1.lock", Allow, ""},
		{"logs/refs/heads/hire/h1/work", Allow, ""}, {"refs/heads/hire/ada/x", Allow, ""}, {"refs/remotes/origin/main", Allow, ""},
		{"refs/remotes/origin/main.lock", Allow, ""}, {"refs/remotes/upstream/x", Allow, ""}, {"logs/refs/remotes/origin/main", Allow, ""},
		{"worktrees/h1/HEAD", Allow, ""}, {"worktrees/h1/index", Allow, ""}, {"worktrees/h1/refs/bisect/bad", Allow, ""},
		{"worktrees/H1/logs/HEAD", Allow, ""}, {"worktrees/h1/rebase-merge/git-rebase-todo", Allow, ""}, {"packed-refs.lock", Allow, ""},
	} {
		if v, by := p.Decide(gitDir+"/"+c.at, Write); v != c.v || c.row != "" && by.Row != c.row {
			t.Errorf("%s: Write %v by %q, want %v by %q", c.at, v, by.Row, c.v, c.row)
		}
	}
	// A desk at the project has no registration of its own: every one is held.
	pl.Root, pl.Checkout, pl.OwnWorktree, pl.CheckoutBranch = repo, "", "", ""
	if v, by := mustNew(t, pl).Decide(gitDir+"/worktrees/later/HEAD", Write); v != Deny || by.Row != rowOtherWorktrees {
		t.Errorf("a desk at the project: worktrees/later/HEAD %v by %q", v, by.Row)
	}
	pl.GOOS = "linux"
	for _, r := range mustBuild(t, pl).Rules() {
		if r.Row == rowNotOwnRefs || r.Row == rowOtherWorktrees {
			t.Errorf("Linux's view has %q", r.Row)
		}
	}
}

// N2 at the start, on a Mac: every worktree's HEAD but the desk's own is followed as the owner checkout's is. One that
// points at a branch the desk can move refuses the start; one on its own branch, another desk's, the owner's, or
// detached, passes, and so does the desk's own HEAD. A registration that is a link refuses it; a file there passes.
func TestOnAMacAnotherWorktreesHEADStopsTheStartWhenADeskCanMoveWhereItLeads(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	const id = "1111111111111111111111111111111111111111\n"
	for _, c := range []struct {
		name, worktree, head string
		atProject            bool
		at, to, move         string // the refusal's Ref, Symref and Moves; at "" when the start goes on
	}{
		{"a later worktree, on the desk's branch", "review", "ref: refs/heads/hire/h1/work\n", false,
			"worktrees/review/HEAD", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work"},
		{"another desk's, on a remote-tracking branch", "h2", "ref: refs/remotes/origin/main\n", false,
			"worktrees/h2/HEAD", "refs/remotes/origin/main", "refs/remotes/origin/main"},
		{"out of refs/", "review", "ref: HEAD\n", false, "worktrees/review/HEAD", "HEAD", ""},
		{"at the project, on the desk's branch", "review", "ref: refs/heads/hire/h1/work\n", true,
			"worktrees/review/HEAD", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work"},
		// Passed.
		{"another desk's, on its own branch", "h2", "ref: refs/heads/hire/h2/work\n", false, "", "", ""},
		{"the owner's review, on its own branch", "review", "ref: refs/heads/review\n", false, "", "", ""},
		{"detached", "review", id, false, "", "", ""},
		{"the desk's own, anywhere", "h1", "ref: refs/remotes/origin/main\n", false, "", "", ""},
		{"no HEAD", "review", "", false, "", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, _ := refRows(t)
			if c.atProject {
				pl.Root, pl.Checkout, pl.OwnWorktree, pl.CheckoutBranch = pl.Checkout, "", "", ""
			}
			reg := filepath.Join(git, "worktrees", c.worktree)
			if err := os.MkdirAll(reg, 0o700); err != nil {
				t.Fatal(err)
			}
			if c.head != "" {
				if err := os.WriteFile(filepath.Join(reg, "HEAD"), []byte(c.head), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := mustNew(t, pl).CheckRefPlaces()
			var e *RefPlaceError
			switch {
			case c.at == "" && err != nil:
				t.Errorf("refused: %v", err)
			case c.at == "":
			case !errors.As(err, &e) || e.Ref != c.at || e.At != git+"/"+c.at || e.Symref != c.to || e.Moves != c.move || e.Link || e.Hard:
				t.Errorf("%v (%+v), want it refused for %s → %s, which a desk moves at %q", err, e, c.at, c.to, c.move)
			}
		})
	}
	t.Run("a registration that is a link", func(t *testing.T) {
		pl, git, outside := refRows(t)
		if err := os.MkdirAll(filepath.Join(git, "worktrees"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(git, "worktrees", "review")); err != nil {
			t.Fatal(err)
		}
		var e *RefPlaceError
		if err := mustNew(t, pl).CheckRefPlaces(); !errors.As(err, &e) || !e.Link || e.Ref != "worktrees/review" {
			t.Errorf("%v, want it refused for the link", err)
		}
	})
	t.Run("a file there", func(t *testing.T) {
		pl, git, _ := refRows(t)
		if err := os.MkdirAll(filepath.Join(git, "worktrees"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(git, "worktrees", "notes.txt"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := mustNew(t, pl).CheckRefPlaces(); err != nil {
			t.Errorf("refused: %v", err)
		}
	})
}
