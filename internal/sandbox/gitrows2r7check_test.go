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

// FXGITROWS2R7 (RT126FIXREVIEW-GITROWS2R6's B1 and O2) at the start, on a Mac desk h1 in its own worktree (refRows), or
// at the project. A name at the top of the shared .git that isn't git's own, or one in a folder there, refuses the
// start when it is a link (a file's or a folder's), a file with a second name, a symbolic ref to any name git takes for
// a ref (`ref: lfs/x`, `ref: FETCH_HEAD`) whose way doesn't stay on refs no desk moves, or a commit's id at a name git
// reads as a ref. It names no API of this round's, so it runs on the tree before it too.
//
// Passed: what a repo's own git keeps there (a merge in progress, a fetch's FETCH_HEAD, packed-refs, config), an
// owner's message that starts "ref: #12", and an id at a name git can't read as a ref. FXGITROWS2R8 changed three
// cases: links at git's own names to a folder a desk can write are refused (B1), and two ways out of refs/ that end at
// a name no desk writes pass (O1); a way from feature to FETCH_HEAD, the desk's own as its git reads it, is refused.
func TestOnAMacALinkAHardLinkAWayOutOfRefsOrACommitIDAtTheTopStopsTheStart(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("CheckRefPlaces looks only for the Policy of this system: a Mac's here")
	}
	id := strings.Repeat("1", 40)
	type file struct{ at, text string }
	for _, c := range []struct {
		name      string
		files     []file
		links     [][2]string // [where, its text]; a text that isn't whole is from the .git
		hard      string      // a file of files given a second name outside the .git
		atProject bool
		at        string // the refusal's Ref; "" when the start goes on
	}{
		{"LINKABS: a link at the top to a file elsewhere", nil, [][2]string{{"feature", "OUT/f"}}, "", false, "feature"},
		{"LINKREL: a link at the top to git-lfs's store", []file{{"lfs/x", id + "\n"}}, [][2]string{{"feature", "lfs/x"}}, "", false, "feature"},
		{"a link at the top whose text is a ref", nil, [][2]string{{"feature", "refs/heads/hire/h1/work"}}, "", false, "feature"},
		{"DIRLINK: a folder linked at the top", nil, [][2]string{{"hire", "OUT"}}, "", false, "hire"},
		{"a link in a folder at the top", []file{{"hire/h2/.keep", ""}}, [][2]string{{"hire/h2/work", "OUT/f"}}, "", false, "hire/h2/work"},
		{"HARDTOP: a file at the top with a second name", []file{{"feature", id + "-not-an-id\n"}}, nil, "feature", false, "feature"},
		{"REFLFS: a symbolic ref to git-lfs's store", []file{{"feature", "ref: lfs/x\n"}}, nil, "", false, "feature"},
		{"REFOBJ: a symbolic ref into objects", []file{{"feature", "ref: objects/x\n"}}, nil, "", false, "feature"},
		// FXGITROWS2R8 (O1): a way out of refs/ that ends at a name no desk writes passes now (below), and that ends
		// at the desk's own FETCH_HEAD is still refused: feature is read by every worktree's git, the desk's included.
		{"a symbolic ref to FETCH_HEAD, a desk in its worktree", []file{{"feature", "ref: FETCH_HEAD\n"}}, nil, "", false, "feature"},
		{"PROJFETCH: a symbolic ref to FETCH_HEAD, at the project", []file{{"feature", "ref: FETCH_HEAD\n"}}, nil, "", true, "feature"},
		{"PROJORIG: a symbolic ref to ORIG_HEAD, at the project", []file{{"feature", "ref: ORIG_HEAD\n"}}, nil, "", true, "feature"},
		{"git's own ref file, a symbolic ref out of refs/", []file{{"ORIG_HEAD", "ref: lfs/x\n"}}, nil, "", false, "ORIG_HEAD"},
		{"STATIC: a commit's id at the top", []file{{"feature", id + "\n"}}, nil, "", false, "feature"},
		{"a sha256 commit's id at the top", []file{{"feature", strings.Repeat("a", 64)}}, nil, "", false, "feature"},
		{"a commit's id with more after a space, which git reads", []file{{"feature", id + " the rest\n"}}, nil, "", false, "feature"},
		{"a commit's id in a folder at the top", []file{{"hire/h2/work", id + "\n"}}, nil, "", false, "hire/h2/work"},
		// Passed.
		{"a symbolic ref to a name at the top, with no space (FXGITROWS2R8, O1: feature2 is no desk's)", []file{{"feature", "ref:feature2\n"}}, nil, "", false, ""},
		{"a symbolic ref to FETCH_HEAD from a name only the main checkout's git reads, a desk in its worktree (FXGITROWS2R8, O1)",
			[]file{{"FEATURE_HEAD", "ref: FETCH_HEAD\n"}}, nil, "", false, ""},
		{"the owner's merge in progress", []file{{"MERGE_HEAD", id + "\n" + id + "\n"}, {"ORIG_HEAD", id + "\n"},
			{"MERGE_MSG", "Merge branch 'hire/h2/work' into feature\n"}, {"MERGE_MODE", "no-ff"}, {"AUTO_MERGE", id + "\n"}}, nil, "", false, ""},
		{"an owner's messages that start \"ref:\"", []file{{"COMMIT_EDITMSG", "ref: #12 the owner's fix\n"}, {"MERGE_MSG", "ref:\n"}},
			nil, "", false, ""},
		{"a fetch's, a pick's, a bisect's, a shallow clone's", []file{{"FETCH_HEAD", id + "\t\tbranch 'main' of ../up\n"},
			{"CHERRY_PICK_HEAD", id + "\n"}, {"BISECT_EXPECTED_REV", id + "\n"}, {"BISECT_START", "main\n"}, {"shallow", id + "\n"}}, nil, "", false, ""},
		{"what git init and pack-refs make", []file{{"packed-refs", "# pack-refs with: peeled fully-peeled sorted \n" + id + " refs/heads/main\n"},
			{"description", "Unnamed repository; edit this file 'description' to name the repository.\n"}, {"info/exclude", ""},
			{"hooks/pre-commit.sample", ""}}, nil, "", false, ""},
		// FXGITROWS2R8 (B1): OUT is a folder no row holds, which a desk can write, so these are refused now. Links at
		// git's own names that pass are in gitrows2r8check_test.go.
		{"links at git's own names, to a folder a desk can write", nil, [][2]string{{"hooks", "OUT"}, {"description", "OUT/f"}}, "", false, "description"},
		{"an id at a name git can't read as a ref", []file{{".tool-state", id + "\n"}, {"x..y", id + "\n"}, {"tool/.state", id + "\n"}}, nil, "", false, ""},
		{"no commit's id", []file{{"feature", strings.Repeat("1", 39) + "\n"}, {"other", id + "x\n"}, {"notes", "hello\n"}}, nil, "", false, ""},
		{"a symbolic ref to the owner's branch", []file{{"feature", "ref: refs/heads/topic/x\n"}}, nil, "", false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			pl, git, outside := refRows(t)
			if c.atProject {
				pl.Root, pl.Checkout, pl.OwnWorktree, pl.CheckoutBranch = pl.Checkout, "", "", ""
			}
			write := func(at, text string) {
				if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(outside, "f"), id+"\n")
			for _, f := range c.files {
				write(filepath.Join(git, filepath.FromSlash(f.at)), f.text)
			}
			for _, l := range c.links {
				to := strings.Replace(l[1], "OUT", outside, 1)
				if err := os.MkdirAll(filepath.Dir(filepath.Join(git, l[0])), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(to, filepath.Join(git, filepath.FromSlash(l[0]))); err != nil {
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
			case !errors.As(err, &e) || e.Ref != c.at || e.At != git+"/"+c.at:
				t.Errorf("%v, want it refused at %s", err, c.at)
			}
		})
	}
}
