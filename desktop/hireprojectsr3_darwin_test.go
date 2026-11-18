package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXHIREPROJECTSR3 (RT126FIXREVIEW-HIREPROJECTSR2's N1 and N2) under the real profile, with stand-ins only:
//   - N1: a desk whose repo shares object files with another (a local clone) isn't started, with words, until the
//     owner gives it files of its own (`git repack -a -d && git prune`: RT126FIXREVIEW-HIREPROJECTSR3 N2);
//   - N2: no desk points the folder for hires' link elsewhere, so a later desk's start holds the same folder.
//
// Its B1 rows, for the office's plain hire whose place was the folder for hires, went with FXHIREPROJECTSR4: no such
// desk starts on a Mac (hireprojectsr4_darwin_test.go).

// N1: a desk whose repo's objects share files with another repo's (the owner's local clone of lib) isn't started, and
// is told why and what to do; once the owner has done it, it starts, and what it does to its own objects stays out of
// lib. lib holds a loose object nothing reaches (a `git add` of a file edited again before the commit leaves one), which
// the clone links too and a repack alone leaves linked (RT126FIXREVIEW-HIREPROJECTSR3 N2): the words name the prune.
func TestADeskWhoseRepoSharesObjectFilesWithAnotherIsntStarted(t *testing.T) {
	l := newHireProjectsLayout(t)
	clone := filepath.Join(l.projects, "copy")
	tree := filepath.Join(hiresFolder(clone, ""), "h7")
	if err := os.WriteFile(filepath.Join(l.base, "unreachable.txt"), []byte("an edit added, then edited again\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loose := strings.TrimSpace(standInOwnersGit(t, l.lib, "hash-object", "-w", filepath.Join(l.base, "unreachable.txt")))
	loose = "objects/" + loose[:2] + "/" + loose[2:]
	standInOwnersGit(t, l.projects, "clone", "-q", l.lib, clone)
	standInOwnersGit(t, clone, "worktree", "add", "-q", "-b", "hire/h7", tree)
	blob := "objects/" + strings.TrimSpace(standInOwnersGit(t, l.lib, "rev-parse", "HEAD:README"))
	blob = blob[:10] + "/" + blob[10:]
	for _, object := range []string{blob, loose} {
		if a, b := statOf(t, filepath.Join(l.lib, ".git", object)), statOf(t, filepath.Join(clone, ".git", object)); !os.SameFile(a, b) {
			t.Fatalf("the local clone's %s isn't lib's own file: no hard links here", object)
		}
	}
	h := Hire{ID: "h7", Name: "Janitor", Role: "janitor", Folder: clone, Repo: clone, Worktree: tree, Branch: "hire/h7", State: "working"}
	writeRegistryHires(t, h)
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	start := func() error {
		_, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
		return err
	}
	var words *errSandboxNotMade
	if err := start(); !errors.As(err, &words) || !strings.Contains(words.words, "has a second name") ||
		!strings.Contains(words.words, "Run git repack -a -d && git prune in "+clone+" to give it files of its own") {
		t.Fatalf("h7's start with lib's objects in its repo: %v", err)
	}
	// A repack alone leaves the loose object nothing reaches, with its second name: still refused, naming it.
	standInOwnersGit(t, clone, "repack", "-q", "-a", "-d")
	if err := start(); !errors.As(err, &words) || !strings.Contains(words.words, filepath.Join(clone, ".git", loose)) {
		t.Fatalf("h7's start after a repack alone: %v", err)
	}
	standInOwnersGit(t, clone, "prune")
	profile, _ := l.profile(t, h)
	// What it may still do to its own objects (the reviewer's rewrite, in place or renamed away) reaches no other repo.
	got := sandboxedSh(t, profile, tree, `for f in $(find "$1/.git/objects" -type f ! -path "*/objects/info/*"); do chmod u+w "$f" && echo planted > "$f" || exit 3; done; echo done`, clone)
	if strings.Join(got, " ") != "done" {
		t.Errorf("h7's rewrite of its own objects: %q", got)
	}
	if err := os.Remove(filepath.Join(l.lib, "README")); err != nil {
		t.Fatal(err)
	}
	standInOwnersGit(t, l.lib, "checkout", "--", "README")
	if data, _ := os.ReadFile(filepath.Join(l.lib, "README")); string(data) != "the owner's\n" {
		t.Errorf("lib's README after the owner's checkout: %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(l.lib, ".git", loose)); strings.Contains(string(data), "planted") {
		t.Error("lib's loose object is h7's now")
	}
}

func statOf(t *testing.T, at string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(at)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// N2: the folder for hires reached through a link (~/Projects -> <base>/real/Projects), as the reviewer's
// TestZZRVLinkedProjects: h1 can't point the link elsewhere, so a hire started after it holds the same folder.
func TestNoDeskPointsTheProjectsFoldersLinkElsewhere(t *testing.T) {
	l := newHireProjectsLayout(t)
	real, decoy := filepath.Join(l.base, "real", "Projects"), filepath.Join(homeDir, "Decoy")
	for _, folder := range []string{filepath.Dir(real), decoy} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(l.projects, real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, l.projects); err != nil {
		t.Fatal(err)
	}
	h8tree := filepath.Join(hiresFolder(l.app, ""), "h8")
	standInOwnersGit(t, l.app, "worktree", "add", "-q", "-b", "hire/h8", h8tree)
	h8 := Hire{ID: "h8", Name: "Tester", Role: "janitor", Folder: l.app, Repo: l.app, Worktree: h8tree, Branch: "hire/h8", State: "working"}
	writeRegistryHires(t, l.hire, h8)
	if err := os.MkdirAll(hireFolder(supportDir, h8.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	profile, s := l.profile(t, l.hire)
	if s.places.Projects != real || !slices.Equal(s.places.ProjectsLinks, []string{l.projects}) {
		t.Fatalf("h1's Places: Projects %q, links %q", s.places.Projects, s.places.ProjectsLinks)
	}
	got := sandboxedSh(t, profile, l.hireTree, `t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; }
t sh -c 'rm "$0" && ln -s "$1" "$0"' "$1" "$2"
t ln -sfh "$2" "$1"
t mv "$1" "$1.moved"
t sh -c 'echo x > "$0/lib/.git/hooks/post-checkout"' "$1"`, l.projects, decoy)
	if strings.Join(got, " ") != "refused refused refused refused" {
		t.Errorf("h1's tries at ~/Projects, the link: %q, want each refused", got)
	}
	if to, err := os.Readlink(l.projects); err != nil || to != real {
		t.Fatalf("~/Projects now leads to %q (%v)", to, err)
	}
	// A hire started after it holds the same folder.
	profile, s = l.profile(t, h8)
	if s.places.Projects != real {
		t.Errorf("h8's projects folder, started after h1: %q", s.places.Projects)
	}
	l.run(t, profile, h8tree, []probeStep{
		{"hook", real + "/lib/.git/hooks/post-checkout", "PermissionError"}, {"write", real + "/lib/README", "PermissionError"},
		{"write", h8tree + "/own.txt", "ok"},
	})
}
