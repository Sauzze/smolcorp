package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXSTACKG G22: RT126FIXREVIEW-HIREPROJECTSR9's O3, O4 and O5, with stand-ins only.

// O3: a gitlink only staged, never committed, spelled otherwise than its folder (in another case), over a repo with a
// core.fsmonitor, stops the next start with the staged words, as one in the folder's own spelling does: the index
// listing pins the spelling fix too. The same for an unmerged entry, stage 2 alone (`update-index --index-info`). The
// core's git ran nothing.
func TestAStagedGitlinkInAnotherSpellingOverARepoStopsItsNextStart(t *testing.T) {
	const plant = r6Tries + `t mkdir -p "$3"
t sh -c 'cd "$0" && /usr/bin/git init -q' "$3"
t sh -c 'printf "#!/bin/sh\necho planted > %s\n" "$1" > "$0" && chmod 755 "$0"' "$1" "$2"
t /usr/bin/git -C "$3" config core.fsmonitor "$1"
`
	for _, c := range []struct{ name, stage string }{
		{"staged", `t sh -c '/usr/bin/git update-index --add --cacheinfo 160000,$(/usr/bin/git rev-parse HEAD),"$0"' "$4"`},
		{"unmerged, stage 2", `t sh -c 'printf "160000 %s 2\t%s\n" "$(/usr/bin/git rev-parse HEAD)" "$0" | /usr/bin/git update-index --index-info' "$4"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(deskStandInVariable, "1") // its next start's git in its sandbox
			l := newHireProjectsLayout(t)
			profile, _ := l.profile(t, l.hire)
			fsm := filepath.Join(l.base, "fsm.sh")
			const folder, gitlink = ".build/checkouts/evil", ".build/checkouts/EVIL"
			if got := sandboxedSh(t, profile, l.hireTree, plant+c.stage, fsm, l.planted, folder, gitlink); strings.Join(got, " ") != "ok ok ok ok ok" {
				t.Fatalf("the hire's tries: %q", got)
			}
			if listed := standInOwnersGit(t, l.hireTree, "ls-files", "-s"); !strings.Contains(listed, "160000 ") || !strings.Contains(listed, gitlink) {
				t.Fatalf("the index: %q", listed)
			}
			tree, shown := tilde(l.hireTree), tilde(filepath.Join(l.hireTree, filepath.FromSlash(gitlink)))
			want := "This hire was set to work in " + tree + ", but it staged " + shown + " as a submodule, and " + shown + " holds a git repository your project doesn't have: your own git, run in " + tree + ", would run in that repository too, with its settings. So it didn't start. Delete the folder " + shown + ", then start it again. To unstage the submodule as well, run `git rm --cached " + gitlink + "` in " + tree + " afterwards."
			if got := r7Start(t, l.hire); got != want {
				t.Errorf("its next start:\n got %q\nwant %q", got, want)
			}
			if _, err := os.Lstat(l.planted); err == nil {
				t.Errorf("the core's git ran the hire's fsmonitor")
			}
		})
	}
}

// O4: the core makes a hire's submodule only in the empty folder git made for it in the worktree, each part of the
// way a folder: with a link on the way (libs → a folder outside the worktree, holding an empty sub), it refuses and
// writes nothing outside; with real folders, it makes it.
func TestAHiresSubmoduleIsNeverMadeThroughALinkOnItsWay(t *testing.T) {
	l := newHireProjectsLayout(t)
	sub := filepath.Join(l.base, "subsrc")
	r7Repo(t, sub)
	standInOwnersGit(t, l.app, "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub, "libs/sub")
	standInOwnersGit(t, l.app, "commit", "-q", "-m", "a submodule")
	commit := strings.TrimSpace(standInOwnersGit(t, l.app, "rev-parse", "HEAD:libs/sub"))
	common := physical(filepath.Join(l.app, ".git"))
	for _, linked := range []bool{true, false} {
		name := map[bool]string{true: "a link on the way", false: "folders all the way"}[linked]
		at := filepath.Join(l.base, map[bool]string{true: "linked", false: "plain"}[linked])
		tree, registration, outside := filepath.Join(at, "tree"), filepath.Join(at, "registration"), filepath.Join(at, "outside")
		for _, folder := range []string{registration, filepath.Join(outside, "sub"), tree} {
			if err := os.MkdirAll(folder, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if linked {
			if err := os.Symlink(outside, filepath.Join(tree, "libs")); err != nil {
				t.Fatal(err)
			}
		} else if err := os.MkdirAll(filepath.Join(tree, "libs", "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
		registry, err := os.OpenRoot(registration)
		if err != nil {
			t.Fatal(err)
		}
		treeRoot, err := os.OpenRoot(tree) // the worktree's folder, as the carry keeps it
		if err != nil {
			t.Fatal(err)
		}
		err = initHireSubmodule(context.Background(), l.app, treeRoot, registry, common, "libs/sub", commit)
		registry.Close()
		treeRoot.Close()
		left, _ := os.ReadDir(filepath.Join(outside, "sub"))
		switch {
		case linked && (err == nil || len(left) != 0):
			t.Errorf("%s: %v, and %d entries written outside the worktree", name, err, len(left))
		case !linked && err != nil:
			t.Errorf("%s: %v", name, err)
		case !linked:
			if _, err := os.Stat(filepath.Join(tree, "libs", "sub", "README")); err != nil {
				t.Errorf("%s: its files: %v", name, err)
			}
		}
	}
}

// O5: a hard-linked file in a Python virtual environment's clones says to make the environment again with its
// packages; in Swift's .build, to delete the folder and build again, as before.
func TestAHardLinkInAVirtualEnvironmentSaysToMakeItAgain(t *testing.T) {
	root := t.TempDir()
	for _, c := range []struct{ at, folder, then string }{
		{".venv/src/dep/.git/objects/ab/cdef", ".venv", "a Python virtual environment: delete it, make it again with its packages, then start the desk again."},
		{"venv/src/dep/.git/objects/ab/cdef", "venv", "a Python virtual environment: delete it, make it again with its packages, then start the desk again."},
		{".build/repositories/dep-1a2b/objects/ab/cdef", ".build", "a package manager's own folder: delete that folder and build again, then start it again."},
	} {
		at := filepath.Join(root, filepath.FromSlash(c.at))
		want := at + " has a second name on this " + machineHere() + " (a hard link), so a desk could read or change it through that name, and the desk didn't start. It's in " + filepath.Join(root, c.folder) + ", " + c.then
		if got := linkedRefusalIn(root, at).Error(); got != want {
			t.Errorf("%s:\n got %q\nwant %q", c.at, got, want)
		}
	}
}
