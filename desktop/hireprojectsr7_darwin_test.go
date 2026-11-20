package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXHIREPROJECTSR7 (RT126FIXREVIEW-HIREPROJECTSR6 B1 (a), N1, N2, O2) under the real profile, with stand-ins only.
//   - B1 (a): a repo a desk makes outside its place and renames in passes the static .git rule, so each start keeps the
//     list of .git folders it found, and the next start refuses one it didn't list that the desk could have moved in.
//     A link below the place to a repo a desk can change refuses the start too.
//   - N1: a package manager's own clones and mirrors (SwiftPM's .build, Bundler's cache, a venv's src) neither stop a
//     desk nor are refused to it, so a hire's own `swift build` clones its dependencies.
//   - N2: a folder the walk can't read is to be made readable or moved out; O2: a bare repo made since the last start
//     is to be removed.
//
// The rows use only what the tree before has (makeDeskSandbox, hirePlaceWords, the profile), so they run there as
// they are, and fail.

// r7Start is desk h's start as makeDeskSandbox makes it: its words when refused, "" when it starts.
func r7Start(t *testing.T, h Hire) string {
	t.Helper()
	_, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
	var words *errSandboxNotMade
	if errors.As(err, &words) {
		return words.words
	}
	if err != nil {
		t.Fatalf("%s's start: %v", h.ID, err)
	}
	return ""
}

// r7Since is the words of a start refused for a repo that turned up in place since the last start.
func r7Since(place, repo string) string {
	return "This hire was set to work in " + tilde(place) + ", but a git repository turned up there since its last start, " + tilde(repo) + ", and the hire could have put it there with hooks your own git runs, so it didn't start. Remove " + tilde(repo) + ", or move it out of " + tilde(place) + ", then start it again. If you added it yourself and want it there, check its hooks first, then fire this hire and hire it again."
}

// r7Repo makes a repo at dir as the owner does, with a commit and a develop branch.
func r7Repo(t *testing.T, dir string) {
	t.Helper()
	ownerMade(t, dir, "README")
	standInOwnersGit(t, dir, "init", "-q", "-b", "main")
	standInOwnersGit(t, dir, "add", "README")
	standInOwnersGit(t, dir, "commit", "-q", "-m", "first")
	standInOwnersGit(t, dir, "branch", "develop")
}

// B1 (a): a desk makes a repo outside its place, plants its hook, and renames it in: the static rule can't see a
// rename of a folder, so the desk's tries pass, but its next start is refused, at a checkout and at a folder of repos
// in a repo. Its start before anything changed, and after the repo is gone, starts (the controls); a repo there at
// its first start is listed, not refused.
func TestARepoMovedIntoAPlaceStopsItsNextStart(t *testing.T) {
	for _, at := range []string{"checkout", "folder of repos"} {
		t.Run(at, func(t *testing.T) {
			l := newHireProjectsLayout(t)
			place := l.lib
			if at == "folder of repos" {
				place, _, _, _ = clientsLayout(t, l)
				standInOwnersGit(t, l.projects, "init", "-q")
			}
			stage := filepath.Join(homeDir, "stage")
			ownerMade(t, homeDir, "stage/")
			h := Hire{ID: "r7a", Name: "Janitor", Role: "janitor", Folder: place, State: "working"}
			profile := r6Desk(t, l, h.ID, place)
			if words := r7Start(t, h); words != "" {
				t.Fatalf("its start again, nothing changed: %s", words)
			}
			got := sandboxedSh(t, profile, place, r6Tries+`t sh -c 'mkdir "$0/y" && cd "$0/y" && /usr/bin/git init -q' "$2"
t hook "$2/y/.git/hooks/post-checkout" "$3"
t mv "$2/y" "$1/y"`, place, stage, l.planted)
			if strings.Join(got, " ") != "ok ok ok" {
				t.Fatalf("the desk's tries: %q", got)
			}
			y := filepath.Join(place, "y")
			if got, want := r7Start(t, h), r7Since(place, y); got != want {
				t.Errorf("its next start:\n got %q\nwant %q", got, want)
			}
			if got, want := r7Start(t, h), r7Since(place, y); got != want {
				t.Errorf("its start after that: %q", got)
			}
			if err := os.RemoveAll(y); err != nil {
				t.Fatal(err)
			}
			if words := r7Start(t, h); words != "" {
				t.Errorf("its start with y gone: %s", words)
			}
		})
	}
}

// B1 (b), with a desk at the owner's checkout (Setup's or the GM's way; no plain hire is there since (W)): the owner
// clones a repo into it after the desk's start. The core can't tell who put it there, and the desk could have moved
// it out, changed it and moved it back, so the next start is refused the same way.
func TestARepoAddedDuringASessionStopsTheNextStartToo(t *testing.T) {
	l := newHireProjectsLayout(t)
	h := Hire{ID: "r7b", Name: "Janitor", Role: "janitor", Folder: l.lib, State: "working"}
	r6Desk(t, l, h.ID, l.lib)
	late := filepath.Join(l.lib, "vendor", "y")
	r7Repo(t, late)
	if got, want := r7Start(t, h), r7Since(l.lib, late); got != want {
		t.Errorf("its next start:\n got %q\nwant %q", got, want)
	}
}

// A link below a desk's place to a repo a desk can change refuses its start: one the desk made outside, a folder
// inside it, and another desk's place. A link to an owner's project no desk writes, to a folder in no repo, to
// nothing, and within the place, start (the controls).
func TestALinkToARepoADeskCanChangeStopsItsStart(t *testing.T) {
	l := newHireProjectsLayout(t)
	stage := filepath.Join(homeDir, "stage")
	r7Repo(t, filepath.Join(stage, "z"))
	ownerMade(t, stage, "z/src/main.go", "plain/notes.md")
	ownerMade(t, l.lib, "docs/a.md")
	h := Hire{ID: "r7l", Name: "Janitor", Role: "janitor", Folder: l.lib, State: "working"}
	writeRegistryHires(t, l.hire, h) // the worktree hire h1 is another desk
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	if words := r7Start(t, h); words != "" {
		t.Fatalf("its first start: %s", words)
	}
	words := func(link, repo string) string {
		return "This hire was set to work in " + tilde(l.lib) + ", but " + tilde(link) + " is a link to a git repository a hire can change, " + tilde(repo) + ", and your own git runs that repository's hooks when it works in " + tilde(link) + ", so it didn't start. Remove the link " + tilde(link) + ", then start it again."
	}
	for _, c := range []struct{ name, link, to, repo string }{
		{"a repo the desk made", "z", filepath.Join(stage, "z"), filepath.Join(stage, "z")},
		{"a folder inside it", "deep/zs", filepath.Join(stage, "z", "src"), filepath.Join(stage, "z")},
		{"another desk's place", "h", l.hireTree, l.hireTree},
		{"an owner's project no desk writes", "app", l.app, ""},
		{"a folder in no repo", "plain", filepath.Join(stage, "plain"), ""},
		{"nothing", "gone", filepath.Join(stage, "gone"), ""},
		{"within its place", "docs-link", filepath.Join(l.lib, "docs"), ""},
	} {
		link := filepath.Join(l.lib, c.link)
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(c.to, link); err != nil {
			t.Fatal(err)
		}
		want := ""
		if c.repo != "" {
			want = words(link, c.repo)
		}
		if got := r7Start(t, h); got != want {
			t.Errorf("%s: its start:\n got %q\nwant %q", c.name, got, want)
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
}

// N1: a Swift package the owner built, with SwiftPM's bare mirror and checkout in .build, and Bundler's and a venv's
// own clones, starts; and a hire in its worktree does what `swift build` does, a mirror and a checkout of a
// dependency, as Bundler and pip do theirs, and starts again. The same names elsewhere are still refused.
func TestAPackageManagersOwnClonesDontStopItsDesk(t *testing.T) {
	l := newHireProjectsLayout(t)
	dep := filepath.Join(l.base, "dep")
	r7Repo(t, dep)
	mirror := filepath.Join(l.lib, ".build", "repositories", "dep-1a2b3c4d")
	ownerMade(t, l.lib, ".build/repositories/", ".build/checkouts/", "vendor/bundle/ruby/3.3.0/cache/bundler/git/", ".venv/pyvenv.cfg", ".venv/src/")
	// As the tools make them: SwiftPM checks a dependency out of its mirror with --shared (its objects stay the
	// mirror's), and Bundler's and pip's clones come from the dependency's own address, so no object file of theirs
	// has a second name (since FXHIREPROJECTSR8 they're held, and a held file with a second name stops a start).
	standInOwnersGit(t, l.lib, "clone", "-q", "--no-hardlinks", "--mirror", dep, mirror)
	standInOwnersGit(t, l.lib, "clone", "-q", "--shared", "--no-checkout", mirror, filepath.Join(l.lib, ".build", "checkouts", "dep"))
	standInOwnersGit(t, filepath.Join(l.lib, ".build", "checkouts", "dep"), "checkout", "-q", "main")
	standInOwnersGit(t, l.lib, "clone", "-q", "--no-hardlinks", "--mirror", dep, filepath.Join(l.lib, "vendor", "bundle", "ruby", "3.3.0", "cache", "bundler", "git", "gem-0a1b"))
	standInOwnersGit(t, l.lib, "clone", "-q", "--no-hardlinks", dep, filepath.Join(l.lib, ".venv", "src", "pkg"))
	if words := hirePlaceWords(l.lib, runGit); words != "" {
		t.Errorf("its hire at the built package: %s", words)
	}
	h := Hire{ID: "r7s", Name: "Janitor", Role: "janitor", Folder: l.lib, State: "working"}
	r6Desk(t, l, h.ID, l.lib)
	if words := r7Start(t, h); words != "" {
		t.Errorf("its start again: %s", words)
	}
	profile, _ := l.profile(t, l.hire)
	got := sandboxedSh(t, profile, l.hireTree, r6Tries+`g() { /usr/bin/git clone -q --no-hardlinks "$@"; }
t g --mirror "$2" "$1/.build/repositories/dep-5e6f7a8b"
t g "$1/.build/repositories/dep-5e6f7a8b" "$1/.build/checkouts/dep"
t g --mirror "$2" "$1/vendor/bundle/ruby/3.3.0/cache/bundler/git/gem-0a1b"
t g "$2" "$1/.venv/src/pkg"
t g "$2" "$1/.build/other"
t g "$2" "$1/build/checkouts/dep"
t g "$2" "$1/src/pkg"`, l.hireTree, dep)
	if want := "ok ok ok ok refused refused refused"; strings.Join(got, " ") != want {
		t.Errorf("the hire's clones: %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(l.hireTree, ".build", "checkouts", "dep", "README")); err != nil {
		t.Errorf("the dependency's checkout: %v", err)
	}
	for _, at := range []string{".build/other", "build/checkouts/dep", "src/pkg"} {
		if _, err := os.Lstat(filepath.Join(l.hireTree, at, ".git")); err == nil {
			t.Errorf("%s/.git was made", at)
		}
	}
	// Its worktree's walk, as its next start and a hire there look (this rig's sandboxed git doesn't answer a worktree
	// hire's second start in time, so the start itself is the desk at lib's, below).
	if words := hirePlaceWords(l.hireTree, runGit); words != "" {
		t.Errorf("the hire's worktree after its clones: %s", words)
	}
	got = sandboxedSh(t, r6Desk(t, l, h.ID, l.lib), l.lib, r6Tries+`/usr/bin/git clone -q --no-hardlinks --mirror "$2" "$1/.build/repositories/dep2-9c0d" && echo ok`, l.lib, dep)
	if strings.Join(got, " ") != "ok" {
		t.Errorf("the desk at lib's clone: %q", got)
	}
	if words := r7Start(t, h); words != "" {
		t.Errorf("the desk at lib's next start: %s", words)
	}
}

// N2: a folder the walk can't read, in a place in a repo, is to be made readable or moved out, at the hire and at a
// start.
func TestAFolderTheWalkCantReadInARepoSaysHowToFixIt(t *testing.T) {
	l := newHireProjectsLayout(t)
	pg := filepath.Join(l.lib, "data", "pg")
	ownerMade(t, l.lib, "data/pg/")
	if err := os.Chmod(pg, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(pg, 0o700) })
	lib, unread := tilde(l.lib), tilde(pg)
	if got, want := hirePlaceWords(l.lib, runGit), "On this Mac, each hire works in one project of its own, and Smolcorp couldn't make sure "+lib+" holds no bare git repository: it can't read "+unread+". Make "+unread+" readable, or move it out of "+lib+", then hire it again."; got != want {
		t.Errorf("its hire:\n got %q\nwant %q", got, want)
	}
	h := Hire{ID: "r7u", Name: "Janitor", Role: "janitor", Folder: l.lib, State: "working"}
	writeRegistryHires(t, l.hire, h)
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil { // its own folder, where its start keeps its mark (FXFIREREAP)
		t.Fatal(err)
	}
	if got, want := r7Start(t, h), "This hire was set to work in "+lib+", but Smolcorp can't read "+unread+", so it couldn't make sure "+lib+" holds no bare git repository, and it didn't start. Make "+unread+" readable, or move it out of "+lib+", then start it again."; got != want {
		t.Errorf("its start:\n got %q\nwant %q", got, want)
	}
}

// O2: a bare repo the desk made in its place since its last start is to be removed. One there at its first start
// keeps R6's words (TestABareRepoBelowAPlaceInARepoRefusesItsHireAndStart).
func TestABareRepoMadeSinceTheLastStartIsToBeRemoved(t *testing.T) {
	l := newHireProjectsLayout(t)
	h := Hire{ID: "r7o", Name: "Janitor", Role: "janitor", Folder: l.lib, State: "working"}
	profile := r6Desk(t, l, h.ID, l.lib)
	if got := sandboxedSh(t, profile, l.lib, r6Tries+`t /usr/bin/git init -q --bare "$1/m"`, l.lib); strings.Join(got, " ") != "ok" {
		t.Fatalf("the desk's git init --bare: %q", got)
	}
	m, lib := tilde(filepath.Join(l.lib, "m")), tilde(l.lib)
	if got, want := r7Start(t, h), "This hire was set to work in "+lib+", but "+lib+" holds a bare git repository, "+m+", that wasn't there at its last start, and your own git runs its hooks, so it didn't start. Remove "+m+", or move it out of "+lib+", then start it again."; got != want {
		t.Errorf("its next start:\n got %q\nwant %q", got, want)
	}
}
