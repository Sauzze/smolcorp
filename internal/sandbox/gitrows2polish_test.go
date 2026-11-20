//go:build unix

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// FXPOLISHA, RT126FIXREVIEW-GITROWS2R9's O1: an owner's git-lfs store kept as a link to another disk a desk writes
// (LFSOUT) refuses the start for the true reason, that the desk can write where it leads, not that the link is one a
// desk can change: the Policy denies its Move and Link, so no desk removes it, renames it away or makes a link at its
// name (TestOnAMacADeskCantReplaceGitLFSsFolderButItsLFSWorkRuns, under the real profile). A link in the desk's
// worktree on the way to it is still one the desk can change.
func TestOnAMacAnOwnersLFSLinkIsRefusedForWhereItLeadsNotAsChangeable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	for _, c := range []struct {
		name, via string // via: a link in the desk's worktree on the way, "" for none
	}{{"LFSOUT", ""}, {"LFSOUT by way of the desk's link", "ROOT"}} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, outside := refRows(t)
			big := filepath.Join(outside, "bigdisk", "lfs")
			if err := os.MkdirAll(big, 0o700); err != nil {
				t.Fatal(err)
			}
			to := big
			if c.via != "" {
				if err := os.MkdirAll(pl.Root, 0o700); err != nil {
					t.Fatal(err)
				}
				to = filepath.Join(pl.Root, "lnk")
				if err := os.Symlink(big, to); err != nil {
					t.Fatal(err)
				}
			}
			lfs := filepath.Join(git, "lfs")
			if err := os.Symlink(to, lfs); err != nil {
				t.Fatal(err)
			}
			err := mustNew(t, pl).CheckRefPlaces()
			var e *RefPlaceError
			if !errors.As(err, &e) || e.Ref != "lfs" || !e.Link {
				t.Fatalf("%v, want the start refused at $G/lfs", err)
			}
			switch {
			case c.via == "" && (e.Via != "" || e.Leads != big):
				t.Errorf("%+v: %v, want it refused for where it leads, %s", *e, err, big)
			case c.via != "" && e.Via != to:
				t.Errorf("%+v: %v, want it refused for the desk's link on the way, %s", *e, err, to)
			}
			if c.via == "" && strings.Contains(err.Error(), "a link a desk can change") {
				t.Errorf("the reason says the link is the desk's to change: %v", err)
			}
		})
	}
}

// FXPOLISHA, RT126FIXREVIEW-GITROWS2R9's O3 (SYMX): a held branch that is a symbolic ref into another desk's branches
// refuses this desk's start too, not only that desk's: that desk would move it with its next commit. SYMXHEAD is the
// same at the owner's checked-out branch. Another desk's symbolic ref among its own branches, and another worktree's
// HEAD on its own desk's branch, are that desk's, and pass.
func TestOnAMacAHeldBranchIntoAnotherDesksBranchesStopsEveryStart(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	const id = "1111111111111111111111111111111111111111\n"
	for _, c := range []struct {
		name, at, text string
		refused        bool
	}{
		{"SYMX: the owner's topic, into h2's work", "refs/heads/topic", "ref: refs/heads/hire/h2/work\n", true},
		{"SYMX: into h2's branches' folder", "refs/heads/topic", "ref: refs/heads/hire/h2\n", true},
		{"SYMXHEAD: the owner's checked-out branch, into h2's work", "refs/heads/topic/x", "ref: refs/heads/hire/h2/work\n", true},
		// Passed.
		{"h2's own symbolic ref among its own branches", "refs/heads/hire/h2/alias", "ref: refs/heads/hire/h2/work\n", false},
		{"another worktree's HEAD on its desk's branch", "worktrees/h2/HEAD", "ref: refs/heads/hire/h2/work\n", false},
		{"control: the owner's topic, into the owner's develop", "refs/heads/topic", "ref: refs/heads/develop\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, _ := refRows(t)
			for at, text := range map[string]string{"refs/heads/hire/h2/work": id, "refs/heads/develop": id, c.at: c.text} {
				file := filepath.Join(git, filepath.FromSlash(at))
				if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := mustNew(t, pl).CheckRefPlaces()
			var e *RefPlaceError
			to := strings.TrimSpace(strings.TrimPrefix(c.text, "ref:"))
			switch {
			case !c.refused && err != nil:
				t.Errorf("refused: %v", err)
			case !c.refused:
			case !errors.As(err, &e) || e.Ref != c.at || e.Symref != to || e.Moves != to:
				t.Errorf("%v, want it refused at %s, by way of %s", err, c.at, to)
			}
		})
	}
}
