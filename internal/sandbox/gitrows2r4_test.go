//go:build unix

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// FXGITROWS2R4 (RT126FIXREVIEW-GITROWS2R3's B1 and O2) in the Policy, on refRows' stand-in .git: a Mac desk h1 in its
// own worktree, the owner's checkout on topic/x, the default branch team/main. Its real-profile rows are in
// gitrows2r4_darwin_test.go.

// The allowlist, in the Policy: in refs/heads and logs/refs/heads a desk writes only its own branches (hire/h1, and
// Places.OwnBranches), with their locks and reflogs; every other branch, a new name, a tag and their reflogs are
// denied, by rowNotOwnBranches and rowTags where no narrower row names them. The other refs, since FXGITROWS2R5, by
// rowNotOwnRefs.
func TestOnAMacADeskWritesNoBranchButItsOwnAndNoTag(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.CheckoutBranch, pl.OwnBranches = "topic/x", []string{"hire/ada"}
	p := mustNew(t, pl)
	for _, c := range []struct {
		at  string
		v   Verdict
		row string
	}{
		{"refs/heads/topic/y", Deny, rowNotOwnBranches}, {"refs/heads/topic/y.lock", Deny, rowNotOwnBranches},
		{"refs/heads/topic", Deny, rowNotOwnBranches}, {"refs/heads/new", Deny, rowNotOwnBranches},
		{"refs/heads", Deny, rowNotOwnBranches}, {"refs/heads/hire1", Deny, rowNotOwnBranches},
		{"logs/refs/heads/topic/y", Deny, rowNotOwnBranches}, {"logs/refs/heads/new", Deny, rowNotOwnBranches},
		{"refs/heads/HIRE/H1x", Deny, rowOtherBranches}, {"refs/heads/hire/h2/work", Deny, rowOtherBranches},
		{"refs/heads/topic/x", Deny, rowCheckoutBranch}, {"refs/heads/main", Deny, "the base branches"},
		{"refs/tags/v1", Deny, rowTags}, {"refs/tags/v1.lock", Deny, rowTags}, {"refs/tags/x/y", Deny, rowTags},
		{"refs/tags", Deny, rowTags}, {"logs/refs/tags/v1", Deny, rowTags},
		{"refs/heads/hire/h1", Allow, ""}, {"refs/heads/hire/h1/work", Allow, ""}, {"refs/heads/hire/h1/work.lock", Allow, ""},
		{"refs/heads/hire/h1.lock", Allow, ""}, {"refs/heads/HIRE/H1/new", Allow, ""}, {"logs/refs/heads/hire/h1/work", Allow, ""},
		{"logs/refs/heads/hire/h1/work.lock", Allow, ""}, {"refs/heads/hire/ada/x", Allow, ""}, {"refs/heads/hire/ada.lock", Allow, ""},
		{"refs/remotes/origin/main", Allow, ""}, {"packed-refs.lock", Allow, ""},
		// Since FXGITROWS2R5 the other refs are no desk's either (rowNotOwnRefs; gitrows2r5_test.go).
		{"refs/stash", Deny, rowNotOwnRefs}, {"logs/refs/stash", Deny, rowNotOwnRefs}, {"refs/notes/commits", Deny, rowNotOwnRefs},
		{"logs/refs/.tmp-renamed-log", Deny, rowNotOwnRefs},
	} {
		if v, by := p.Decide(gitDir+"/"+c.at, Write); v != c.v || c.row != "" && by.Row != c.row {
			t.Errorf("%s: Write %v by %q, want %v by %q", c.at, v, by.Row, c.v, c.row)
		}
	}
	pl.GOOS = "linux"
	for _, r := range mustBuild(t, pl).Rules() {
		if r.Row == rowNotOwnBranches || r.Row == rowTags {
			t.Errorf("Linux's view has %q", r.Row)
		}
	}
}

// B1 (a) and (b) at the start, on a Mac: a ref the Policy holds that is a symbolic ref, and the owner checkout's HEAD,
// refuse the start when the way they point along leads to a ref the desk can write (its own branch, a remote-tracking
// one, a ref git reads in a worktree), leaves refs/, or doesn't end. A way that stays in refs no desk writes (master
// to main), origin/HEAD, and the desk's own symbolic refs are passed. O2: a folder the scan can't read fails the
// start, with an error that isn't a refusal's words.
func TestOnAMacASymbolicRefAtAHeldRefStopsTheStartWhenADeskCanMoveWhereItLeads(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	const id = "1111111111111111111111111111111111111111\n"
	type file struct{ at, text string }
	symref := func(at, to string) file { return file{at, "ref: " + to + "\n"} }
	for _, c := range []struct {
		name         string
		files        []file
		atProject    bool
		at, to, move string // the refusal's Ref, Symref and Moves; at "" when the start goes on
	}{
		{"the checkout branch, to the desk's own", []file{symref("refs/heads/topic/x", "refs/heads/hire/h1/work")},
			false, "refs/heads/topic/x", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work"},
		{"a base branch, to the desk's own", []file{symref("refs/heads/main", "refs/heads/hire/h1/work")},
			false, "refs/heads/main", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work"},
		{"another desk's branch, to the desk's own", []file{symref("refs/heads/hire/h2/work", "refs/heads/hire/h1")},
			false, "refs/heads/hire/h2/work", "refs/heads/hire/h1", "refs/heads/hire/h1"},
		{"an owner's branch no row names, to the desk's own", []file{symref("refs/heads/topic/y", "refs/heads/hire/h1/work")},
			false, "refs/heads/topic/y", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work"},
		{"a tag, to a remote-tracking branch", []file{symref("refs/tags/v1", "refs/remotes/origin/main")},
			false, "refs/tags/v1", "refs/remotes/origin/main", "refs/remotes/origin/main"},
		{"on through another", []file{symref("refs/heads/topic/y", "refs/heads/zz"), symref("refs/heads/zz", "refs/heads/hire/h1/work")},
			false, "refs/heads/topic/y", "refs/heads/zz", "refs/heads/hire/h1/work"}, // topic/y is looked at first
		{"on through origin/HEAD", []file{symref("refs/heads/up", "refs/remotes/origin/HEAD"), symref("refs/remotes/origin/HEAD", "refs/remotes/origin/main")},
			false, "refs/heads/up", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
		{"out of refs/", []file{symref("refs/heads/topic/y", "HEAD")}, false, "refs/heads/topic/y", "HEAD", ""},
		{"up out of refs/", []file{symref("refs/heads/topic/y", "refs/../config")}, false, "refs/heads/topic/y", "refs/../config", ""},
		{"round in a ring", []file{symref("refs/heads/a", "refs/heads/b"), symref("refs/heads/b", "refs/heads/a")},
			false, "refs/heads/a", "refs/heads/b", ""},
		{"a note, to the desk's own", []file{symref("refs/notes/x", "refs/heads/hire/h1/work")},
			false, "refs/notes/x", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work"},
		{"the owner checkout's HEAD, to a remote-tracking branch", []file{symref("HEAD", "refs/remotes/origin/main")},
			false, "HEAD", "refs/remotes/origin/main", "refs/remotes/origin/main"},
		{"the owner checkout's HEAD, to the desk's own", []file{symref("HEAD", "refs/heads/hire/h1/work")},
			false, "HEAD", "refs/heads/hire/h1/work", "refs/heads/hire/h1/work"},
		// Passed.
		{"master to main", []file{symref("refs/heads/master", "refs/heads/main"), {"refs/heads/main", id}}, false, "", "", ""},
		{"to a branch that's packed or not made", []file{symref("refs/heads/master", "refs/heads/main")}, false, "", "", ""},
		{"origin/HEAD", []file{symref("refs/remotes/origin/HEAD", "refs/remotes/origin/main")}, false, "", "", ""},
		{"the desk's own, to the owner's", []file{symref("refs/heads/hire/h1/z", "refs/heads/topic/x")}, false, "", "", ""},
		// FXGITROWS2R5: a note is held now too. FXGITROWS2R6 (RT126FIXREVIEW-GITROWS2R5's B1): a ref git reads per worktree
		// (refs/bisect/bad) is refused on any way: in another worktree git reads it from that worktree's registration,
		// which the shared .git's look can't stand for, and git never makes a symbolic ref to one.
		{"to a ref git reads in a worktree", []file{symref("refs/heads/topic/y", "refs/bisect/bad")}, false, "refs/heads/topic/y", "refs/bisect/bad", ""},
		{"the owner checkout's HEAD, on its branch", []file{symref("HEAD", "refs/heads/topic/x")}, false, "", "", ""},
		{"HEAD, at the project", []file{symref("HEAD", "refs/remotes/origin/main")}, true, "", "", ""},
		{"a lock", []file{symref("refs/heads/main.lock", "refs/heads/hire/h1/work")}, false, "", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, _ := refRows(t)
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
	t.Run("a link git reads as a symbolic ref, at HEAD", func(t *testing.T) {
		pl, git, _ := refRows(t)
		if err := os.Symlink("refs/heads/hire/h1/work", filepath.Join(git, "HEAD")); err != nil {
			t.Fatal(err)
		}
		var e *RefPlaceError
		if err := mustNew(t, pl).CheckRefPlaces(); !errors.As(err, &e) || e.Ref != "HEAD" || e.Moves != "refs/heads/hire/h1/work" {
			t.Errorf("%v, want it refused for HEAD", err)
		}
	})
	t.Run("a folder it can't read (O2)", func(t *testing.T) {
		pl, git, _ := refRows(t)
		shut := filepath.Join(git, "refs", "tags", "shut")
		if err := os.Mkdir(shut, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(shut, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(shut, 0o700) })
		err := mustNew(t, pl).CheckRefPlaces()
		var e *RefPlaceError
		if err == nil || errors.As(err, &e) || !errors.Is(err, os.ErrPermission) {
			t.Errorf("%v, want the start failed with a permission error", err)
		}
	})
}
