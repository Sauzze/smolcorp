//go:build unix

package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// FXGITROWS2R9 at the start, on a Mac desk h1 in its own worktree (refRows). It names no API of this round's, so it
// runs on the tree before it too.
//
// B1 (RT126FIXREVIEW-GITROWS2R8's B1): a link at one of git's own names is followed one link at a time, a link in a
// folder on the way too, and the start is refused when a link on the way is one the desk can change (in its worktree,
// or in a folder no row holds), wherever the way leads at the start; or when the way goes through more links than the
// kernel follows. A way out of refs/ at the top (`ref: rr-cache/x`) is followed the same way.
//
// B2 (B2): a file with a second name at one of git's own file names refuses the start.
//
// N1 (N1): a link that leads, or goes by way of a link, under a place Smolcorp gives some desk refuses the start,
// though this desk can't write there: the support folders and every hires/<id> in them, the socket folder, tmux's
// folder, the GM's folder, another hire's worktree, and in the shared .git another worktree's registration or another
// desk's branches.
func TestOnAMacALinkAtGitsOwnNamesIsFollowedOneLinkAtATimeToNoDesksPlace(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	id := strings.Repeat("1", 40)
	type file struct{ at, text string }
	// chain is rr-cache, then n+1 links, each in HELD, the folder git's config names for hooks, to a folder there: n+2
	// links in all, and the kernel follows 32.
	chain := func(n int) [][2]string {
		links := [][2]string{{"rr-cache", "HELD/l0"}}
		for i := range n {
			links = append(links, [2]string{fmt.Sprintf("HELD/l%d", i), fmt.Sprintf("HELD/l%d", i+1)})
		}
		return append(links, [2]string{fmt.Sprintf("HELD/l%d", n), "HELD/end"})
	}
	for _, c := range []struct {
		name  string
		files []file
		links [][2]string // [where, its text]: the places below; else from the .git
		hard  [][2]string // [where, the file it is a second name of]
		at    string      // the refusal's Ref; "" when the start goes on
	}{
		// B1: refused.
		{"UPMIDHELD: by way of a link in the desk's worktree, to a held folder", nil,
			[][2]string{{"ROOT/lnk", "HELD"}, {"rr-cache", "ROOT/lnk"}}, nil, "rr-cache"},
		{"UPMIDDIR: by way of a link at a folder on the way, in the desk's worktree", []file{{"HELD/rr/notes", ""}},
			[][2]string{{"ROOT/dl", "HELD"}, {"rr-cache", "ROOT/dl/rr"}}, nil, "rr-cache"},
		{"by way of a link in a folder no row holds, to a held folder", nil,
			[][2]string{{"OUT/l", "HELD"}, {"rr-cache", "OUT/l"}}, nil, "rr-cache"},
		{"UPMIDAUTO: MERGE_AUTOSTASH by way of the desk's link, to a held file", []file{{"HELD/as", id + "\n"}},
			[][2]string{{"ROOT/as", "HELD/as"}, {"MERGE_AUTOSTASH", "ROOT/as"}}, nil, "MERGE_AUTOSTASH"},
		{"a relative link in the desk's worktree on the way", []file{{"HELD/rr/notes", ""}},
			[][2]string{{"ROOT/dl", "../../held"}, {"rr-cache", "ROOT/dl/rr"}}, nil, "rr-cache"},
		{"33 links, more than the kernel follows, all in a held folder", []file{{"HELD/end/notes", ""}}, chain(31), nil, "rr-cache"},
		{"links in a loop", nil, [][2]string{{"OUT/a", "OUT/b"}, {"OUT/b", "OUT/a"}, {"rr-cache", "OUT/a"}}, nil, "rr-cache"},
		{"a way out of refs/ through a link at rr-cache, by way of the desk's link", []file{{"COMMIT_EDITMSG", "ref: rr-cache/x\n"}},
			[][2]string{{"ROOT/lnk", "HELD"}, {"rr-cache", "ROOT/lnk"}}, nil, "COMMIT_EDITMSG"},
		// B1: passed.
		{"a link to a held folder", nil, [][2]string{{"rr-cache", "HELD"}}, nil, ""},
		{"by way of a link in a held folder, to a held folder", []file{{"HELD/real/notes", ""}},
			[][2]string{{"HELD/l", "HELD/real"}, {"rr-cache", "HELD/l"}}, nil, ""},
		{"32 links, all in a held folder", []file{{"HELD/end/notes", ""}}, chain(30), nil, ""},
		// B2: refused.
		{"UPHFETCH: FETCH_HEAD, a second name of a file in the desk's worktree", []file{{"ROOT/fh", id + "\t\tbranch 'main' of ../up\n"}},
			nil, [][2]string{{"FETCH_HEAD", "ROOT/fh"}}, "FETCH_HEAD"},
		{"UPHAUTO: MERGE_AUTOSTASH, a second name", []file{{"ROOT/as", id + "\n"}}, nil, [][2]string{{"MERGE_AUTOSTASH", "ROOT/as"}}, "MERGE_AUTOSTASH"},
		{"COMMIT_EDITMSG, a second name", []file{{"ROOT/m", "the desk's\n"}}, nil, [][2]string{{"COMMIT_EDITMSG", "ROOT/m"}}, "COMMIT_EDITMSG"},
		{"config, a second name", []file{{"ROOT/cfg", "[core]\n"}}, nil, [][2]string{{"config", "ROOT/cfg"}}, "config"},
		{"packed-refs, a second name", []file{{"ROOT/pr", id + " refs/heads/feature\n"}}, nil, [][2]string{{"packed-refs", "ROOT/pr"}}, "packed-refs"},
		{"ORIG_HEAD, a second name of a file outside", []file{{"OUT/oh", id + "\n"}}, nil, [][2]string{{"ORIG_HEAD", "OUT/oh"}}, "ORIG_HEAD"},
		// B2: passed.
		{"FETCH_HEAD, a file of one name", []file{{"FETCH_HEAD", id + "\t\tbranch 'main' of ../up\n"}}, nil, nil, ""},
		// N1: refused.
		{"XPROJ: rr-cache into another project's desk's Claude folder", []file{{"SUP/hires/o1/claude/notes", ""}},
			[][2]string{{"rr-cache", "SUP/hires/o1/claude"}}, nil, "rr-cache"},
		{"MERGE_AUTOSTASH into another project's desk's folder", []file{{"SUP/hires/o1/as", id + "\n"}},
			[][2]string{{"MERGE_AUTOSTASH", "SUP/hires/o1/as"}}, nil, "MERGE_AUTOSTASH"},
		{"rr-cache into another hire's worktree", []file{{"HIRES/h2/rr/notes", ""}}, [][2]string{{"rr-cache", "HIRES/h2/rr"}}, nil, "rr-cache"},
		{"rr-cache into the socket folder", []file{{"SOCK/o1/notes", ""}}, [][2]string{{"rr-cache", "SOCK/o1"}}, nil, "rr-cache"},
		{"rr-cache into the GM's folder", []file{{"GM/rr/notes", ""}}, [][2]string{{"rr-cache", "GM/rr"}}, nil, "rr-cache"},
		{"rr-cache into tmux's folder", []file{{"TMUX/rr/notes", ""}}, [][2]string{{"rr-cache", "TMUX/rr"}}, nil, "rr-cache"},
		{"rr-cache into another support folder", []file{{"OTHERSUP/rr/notes", ""}}, [][2]string{{"rr-cache", "OTHERSUP/rr"}}, nil, "rr-cache"},
		{"rr-cache into another worktree's registration", []file{{"worktrees/h2/rr/notes", ""}}, [][2]string{{"rr-cache", "GIT/worktrees/h2/rr"}}, nil, "rr-cache"},
		{"rr-cache into another desk's branches", []file{{"refs/heads/hire/h2/work", id + "\n"}}, [][2]string{{"rr-cache", "GIT/refs/heads/hire/h2"}}, nil, "rr-cache"},
		{"by way of a link in another desk's folder, to a held folder", nil,
			[][2]string{{"SUP/hires/o1/l", "HELD"}, {"rr-cache", "SUP/hires/o1/l"}}, nil, "rr-cache"},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, outside := refRows(t)
			pl.SocketParent = filepath.Join(filepath.Dir(pl.Home), "k")
			pl.OwnSockets = filepath.Join(pl.SocketParent, "h1")
			pl.TmuxFolder = filepath.Join(filepath.Dir(pl.Home), "tmux-501")
			pl.GMFolder = filepath.Join(pl.Home, "app-gm")
			pl.OtherSupports = []string{filepath.Join(pl.Home, "AD-real")}
			held := filepath.Join(pl.Home, "held")
			pl.GitHooks = []string{held}
			places := strings.NewReplacer("ROOT", pl.Root, "HOME", pl.Home, "HIRES", pl.HiresFolder, "OUT", outside, "HELD", held,
				"OTHERSUP", pl.OtherSupports[0], "SUP", pl.Support, "SOCK", pl.SocketParent, "GM", pl.GMFolder, "TMUX", pl.TmuxFolder, "GIT", git)
			where := func(at string) string {
				if p := places.Replace(at); p != at {
					return p
				}
				return filepath.Join(git, filepath.FromSlash(at))
			}
			for _, folder := range []string{pl.Root, filepath.Join(pl.HiresFolder, "h2"), held, pl.OwnSockets, pl.TmuxFolder, pl.GMFolder} {
				if err := os.MkdirAll(folder, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			for _, f := range c.files {
				at := where(f.at)
				if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(at, []byte(f.text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, l := range c.links {
				at := where(l[0])
				if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(places.Replace(l[1]), at); err != nil {
					t.Fatal(err)
				}
			}
			for _, h := range c.hard {
				if err := os.Link(where(h[1]), where(h[0])); err != nil {
					t.Fatal(err)
				}
			}
			err := mustNew(t, pl).CheckRefPlaces()
			var e *RefPlaceError
			switch {
			case c.at == "" && err != nil:
				t.Errorf("refused: %v", err)
			case c.at == "":
			case !errors.As(err, &e) || e.Ref != c.at || e.At != git+"/"+c.at:
				t.Errorf("%v, want it refused at %s", err, c.at)
			}
		})
	}
}

// O2 (RT126FIXREVIEW-GITROWS2R8's O2): a Mac desk can't remove git-lfs's store, rename it away or make a link at its
// name, and the core makes it when missing (Hold); what is in it stays the desk's to write.
func TestOnAMacGitLFSsFolderItselfIsHeldAndWhatIsInItIsTheDesks(t *testing.T) {
	pl, git, _ := refRows(t)
	p := mustNew(t, pl)
	lfs := git + "/lfs"
	for _, op := range []Op{Move, Link} {
		if v, row := p.Decide(lfs, op); v != Deny {
			t.Errorf("%s of $G/lfs: %v (%s), want it denied", op, v, row.Row)
		}
	}
	for _, at := range []string{lfs + "/objects", lfs + "/objects/ab/cd/abcd", lfs + "/tmp/x"} {
		if v, row := p.Decide(at, Write); v == Deny {
			t.Errorf("writing %s: denied (%s), want it the desk's", at, row.Row)
		}
	}
	if !slices.Contains(p.Hold(), spell(pl.GOOS, lfs)) {
		t.Errorf("Hold() %q doesn't make $G/lfs", p.Hold())
	}
}
