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

// FXGITROWS2R8 at the start, on a Mac desk h1 in its own worktree (refRows), or at the project. It names no API of this
// round's, so it runs on the tree before it too.
//
// B1 (RT126FIXREVIEW-GITROWS2R7's B1 and its "seen outside" hooks link): a link at any name at the top of the shared
// .git, git's own names included, is followed; the start is refused when it leads where this desk can write, or can't
// be followed. A desk's Policy denies only what it holds, so an owner's folder it doesn't hold (~/dotfiles/hooks) is
// one a desk can write, and refused; one it holds (a hooks folder git's config names, ~/.config/git) passes.
//
// O1: a symbolic ref at the top whose way leaves refs/ refuses the start only where it ends at a name the desk can
// write, or can't be followed. An all-capitals name at the top (COMMIT_EDITMSG) is read only by the main checkout's
// git, so its way ends in the shared .git; another name (feature) is read by every worktree's git, so a way to an
// all-capitals name (FETCH_HEAD) ends in each one's own registration, the desk's included.
func TestOnAMacALinkAtGitsOwnNamesIsFollowedAndAWayOutOfRefsOnlyStopsWhereADeskWrites(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	id := strings.Repeat("1", 40)
	type file struct{ at, text string }
	for _, c := range []struct {
		name      string
		files     []file
		links     [][2]string // [where, its text]: ROOT, CHECKOUT, HOME, HIRES and OUT are those places; else from the .git
		atProject bool
		hooks     string // a folder the owner's git config names for hooks (Places.GitHooks), as ReadGitConfig gives it
		at        string // the refusal's Ref; "" when the start goes on
	}{
		// B1: refused.
		{"UPRR2: rr-cache, a link into the desk's worktree", nil, [][2]string{{"rr-cache", "ROOT/rr"}}, false, "", "rr-cache"},
		{"UPAUTO: MERGE_AUTOSTASH, a link into the desk's worktree", []file{{"ROOT/as", id + "\n"}}, [][2]string{{"MERGE_AUTOSTASH", "ROOT/as"}}, false, "", "MERGE_AUTOSTASH"},
		{"UPFETCH: FETCH_HEAD, a link into the desk's worktree", []file{{"ROOT/fh", id + "\t\tbranch 'main' of ../up\n"}}, [][2]string{{"FETCH_HEAD", "ROOT/fh"}}, false, "", "FETCH_HEAD"},
		{"hooks, a link into the checkout, a desk at the project", []file{{"CHECKOUT/.githooks/post-commit", "#!/bin/sh\n"}}, [][2]string{{"hooks", "../.githooks"}}, true, "", "hooks"},
		{"hooks, a link to an owner's folder no row holds", []file{{"HOME/dotfiles/hooks/post-commit", "#!/bin/sh\n"}}, [][2]string{{"hooks", "HOME/dotfiles/hooks"}}, false, "", "hooks"},
		{"config, a link to an owner's file no row holds", []file{{"HOME/dotfiles/app-config", "[core]\n"}}, [][2]string{{"config", "HOME/dotfiles/app-config"}}, false, "", "config"},
		{"info, a link to a folder that holds the desk's own place", nil, [][2]string{{"info", "HIRES"}}, false, "", "info"},
		{"rr-cache, a link that leads nowhere", nil, [][2]string{{"rr-cache", "OUT/gone"}}, false, "", "rr-cache"},
		{"MERGE_RR, a link through a link into the desk's worktree", []file{{"ROOT/rr", ""}}, [][2]string{{"MERGE_RR", "OUT/l"}, {"OUT/l", "ROOT/rr"}}, false, "", "MERGE_RR"},
		// FXGITROWS2R9 (RT126FIXREVIEW-GITROWS2R8's N1): refused, no longer passed. The checkout is a place Smolcorp gives a
		// desk at the project (a Mac plain hire at the main checkout), which writes .githooks, though this desk can't.
		{"hooks, a link into the checkout, a desk in its own worktree", []file{{"CHECKOUT/.githooks/post-commit", "#!/bin/sh\n"}}, [][2]string{{"hooks", "../.githooks"}}, false, "", "hooks"},
		// B1: passed.
		{"hooks, a link to the folder git's config names for hooks", []file{{"HOME/dotfiles/hooks/post-commit", "#!/bin/sh\n"}}, [][2]string{{"hooks", "HOME/dotfiles/hooks"}}, false, "HOME/dotfiles/hooks", ""},
		{"config, a link to a file in ~/.config/git", []file{{"HOME/.config/git/app-config", "[core]\n"}}, [][2]string{{"config", "HOME/.config/git/app-config"}}, false, "", ""},
		{"a real rr-cache folder, with an entry", []file{{"rr-cache/" + id + "/preimage", "a\n"}}, nil, false, "", ""},
		// O1: refused.
		{"REFLFS: a symbolic ref to git-lfs's store", []file{{"feature", "ref: lfs/x\n"}}, nil, false, "", "feature"},
		{"REFOBJ: a symbolic ref into objects", []file{{"feature", "ref: objects/x\n"}}, nil, false, "", "feature"},
		{"an owner's message that is a way into git-lfs's store", []file{{"COMMIT_EDITMSG", "ref: lfs/x\n"}}, nil, false, "", "COMMIT_EDITMSG"},
		{"a way on, from a name the desk can't write into objects", []file{{"feature", "ref: tidy\n"}, {"tidy", "ref: objects/x\n"}}, nil, false, "", "feature"},
		{"a name every worktree reads, to FETCH_HEAD: the desk's own", []file{{"feature", "ref: FETCH_HEAD\n"}}, nil, false, "", "feature"},
		{"an owner's message that is a way to FETCH_HEAD, a desk at the project", []file{{"COMMIT_EDITMSG", "ref: FETCH_HEAD\n"}}, nil, true, "", "COMMIT_EDITMSG"},
		{"a way out of the .git", []file{{"feature", "ref: ../x\n"}}, nil, false, "", "feature"},
		{"a way to another worktree's own", []file{{"feature", "ref: worktrees/h2/x\n"}}, nil, false, "", "feature"},
		// O1: passed.
		{"an owner's message \"ref: cleanup\"", []file{{"COMMIT_EDITMSG", "ref: cleanup\n"}}, nil, false, "", ""},
		{"an owner's message \"ref: WIP\"", []file{{"MERGE_MSG", "ref: WIP\n"}}, nil, false, "", ""},
		{"an owner's message that is a way to FETCH_HEAD, a desk in its worktree", []file{{"COMMIT_EDITMSG", "ref: FETCH_HEAD\n"}, {"FETCH_HEAD", id + "\n"}}, nil, false, "", ""},
		{"a name the desk can't write, to another it can't write", []file{{"feature", "ref: cleanup\n"}, {"cleanup", "ref: refs/heads/topic/x\n"}}, nil, false, "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, outside := refRows(t)
			if c.atProject {
				pl.Root, pl.Checkout, pl.OwnWorktree, pl.CheckoutBranch = pl.Checkout, "", "", ""
			}
			places := strings.NewReplacer("ROOT", pl.Root, "CHECKOUT", filepath.Dir(git), "HOME", pl.Home, "HIRES", pl.HiresFolder, "OUT", outside)
			where := func(at string) string {
				if p := places.Replace(at); p != at {
					return p
				}
				return filepath.Join(git, filepath.FromSlash(at))
			}
			if c.hooks != "" {
				pl.GitHooks = []string{places.Replace(c.hooks)}
			}
			for _, folder := range []string{pl.Root, filepath.Join(pl.HiresFolder, "h2")} {
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
