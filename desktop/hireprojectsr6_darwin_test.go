package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXHIREPROJECTSR6 (RT126FIXREVIEW-HIREPROJECTSR5 B1 and N2) under the real profile, with stand-ins
// only. On a Mac, a desk writes, links and renames nothing in a .git strictly below its place, nor makes or renames
// one in there, whatever its place's layout: a folder of repos under a projects folder or a home that is itself a
// repo (B1), a clone nested in a checkout (N2). The folders on the way to each .git found at its start are held, so
// none is moved out and back. A bare repo below any place, which no pattern can tell, refuses its hire and its start.
// Its own git work, .github and .gitignore, and a plain folder's work are unchanged. The walk's words say ~/… and
// 50,000 (O2). (R6's row for a plain hire's picker saying it can't commit went with ruling (W), FXHIREPROJECTSR7: a
// plain hire now commits in its own worktree; TestTheOfficesPlainHireWorksInAWorktreeOfItsOwn.)
//
// The rows use only what the tree before has (makeDeskSandbox, hirePlaceWords, the office), so they run there as they
// are, and fail.

// r6Desk is a desk with no worktree at folder, made as its start makes it: its profile's path.
func r6Desk(t *testing.T, l gmRootLayout, id, folder string) string {
	t.Helper()
	h := Hire{ID: id, Name: "Janitor", Role: "janitor", Folder: folder, State: "working"}
	writeRegistryHires(t, l.hire, h)
	if err := os.MkdirAll(hireFolder(supportDir, id), 0o700); err != nil {
		t.Fatal(err)
	}
	profile, _ := l.profile(t, h)
	return profile
}

// r6Tries is the shell a row's tries run in: t says ok or refused for each, hook writes a post-checkout hook that
// leaves the file $3 says.
const r6Tries = `t() { if "$@" 2>/dev/null; then echo ok; else echo refused; fi; }
hook() { printf '#!/bin/sh\necho planted > "%s"\n' "$2" > "$1" && chmod 755 "$1"; }
`

// nothingPlanted checks the owner's checkout of develop in each repo ran no hook, and no config there names a monitor.
func nothingPlanted(t *testing.T, l gmRootLayout, repos ...string) {
	t.Helper()
	for _, repo := range repos {
		standInOwnersGit(t, repo, "checkout", "-q", "develop")
		if config, _ := os.ReadFile(filepath.Join(repo, ".git", "config")); strings.Contains(string(config), "fsmonitor") {
			t.Errorf("%s's config is now %q", strings.TrimPrefix(repo, l.base), config)
		}
	}
	if _, err := os.Lstat(l.planted); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the owner's checkout ran a hook the desk planted")
	}
}

// B1: the projects folder, or the home folder, is itself a repo, so clients (a plain folder holding acme and beta)
// is in a repo, and its desk starts there; but under its profile it can't plant acme's hook or beta's config, nor
// move acme out of its place and back, nor rename acme's .git away. acme's own files are its place's.
func TestADeskAtAFolderOfReposInARepoCantWriteTheirGit(t *testing.T) {
	for _, above := range []string{"projects", "home"} {
		t.Run(above, func(t *testing.T) {
			l := newHireProjectsLayout(t)
			clients, acme, beta, _ := clientsLayout(t, l)
			top := map[string]string{"projects": l.projects, "home": homeDir}[above]
			standInOwnersGit(t, top, "init", "-q")
			for _, repo := range []string{acme, beta} {
				standInOwnersGit(t, repo, "branch", "develop")
			}
			if words := hirePlaceWords(clients, runGit); words != "" {
				t.Fatalf("its hire: %s", words)
			}
			profile := r6Desk(t, l, "r6b", clients)
			out := filepath.Join(l.base, "out")
			ownerMade(t, l.base, "out/")
			got := sandboxedSh(t, profile, clients, r6Tries+`t hook "$1/acme/.git/hooks/post-checkout" "$3"
t sh -c 'printf "[core]\n\tfsmonitor = /nowhere\n" >> "$0/beta/.git/config"' "$1"
t mv "$1/acme" "$2/acme"
t mv "$1/acme/.git" "$1/acme/old-git"
t ln "$1/beta/.git/config" "$1/config-link"
t sh -c 'echo x > "$0/acme/README"' "$1"
t sh -c 'echo x > "$0/plan.md"' "$1"`, clients, out, l.planted)
			if want := "refused refused refused refused refused ok ok"; strings.Join(got, " ") != want {
				t.Errorf("its tries: %q, want %q", got, want)
			}
			if _, err := os.Lstat(filepath.Join(out, "acme")); err == nil {
				t.Errorf("acme was moved out")
			}
			nothingPlanted(t, l, acme, beta)
		})
	}
}

// N2: an owner's clone nested in a checkout (lib/vendor/x). A desk at lib can't plant x's hook or config, nor move
// vendor or x out and back, nor rename x away inside its place; x's own files are its place's.
func TestANestedCloneInACheckoutIsReadOnlyToItsDesk(t *testing.T) {
	l := newHireProjectsLayout(t)
	nested := filepath.Join(l.lib, "vendor", "x")
	ownerMade(t, nested, "README")
	standInOwnersGit(t, nested, "init", "-q", "-b", "main")
	standInOwnersGit(t, nested, "add", "README")
	standInOwnersGit(t, nested, "commit", "-q", "-m", "first")
	standInOwnersGit(t, nested, "branch", "develop")
	profile := r6Desk(t, l, "r6n", l.lib)
	out := filepath.Join(l.base, "out")
	ownerMade(t, l.base, "out/")
	got := sandboxedSh(t, profile, l.lib, r6Tries+`t hook "$1/vendor/x/.git/hooks/post-checkout" "$3"
t sh -c 'printf "[core]\n\tfsmonitor = /nowhere\n" >> "$0/vendor/x/.git/config"' "$1"
t mv "$1/vendor" "$2/vendor"
t mv "$1/vendor/x" "$1/x2"
t sh -c 'echo x > "$0/vendor/x/README"' "$1"`, l.lib, out, l.planted)
	if want := "refused refused refused refused ok"; strings.Join(got, " ") != want {
		t.Errorf("its tries: %q, want %q", got, want)
	}
	nothingPlanted(t, l, nested)
}

// No desk makes a .git below its place, in its worktree or in a plain folder: a folder, a file, a link, one renamed
// in, any case, or by git init; nor a .git or a HEAD at a plain folder's top. .github, .gitignore and plain folders
// are still its work (the controls).
func TestNoDeskMakesAGitFolderBelowItsPlace(t *testing.T) {
	l := newHireProjectsLayout(t)
	_, _, _, notes := clientsLayout(t, l)
	out := filepath.Join(l.base, "out")
	ownerMade(t, l.base, "out/evil/")
	makes := `t mkdir -p "$1/sub/.git"
t mv "$2/evil" "$1/sub/.git"
t sh -c 'mkdir -p "$0/sub" && echo "gitdir: $1" > "$0/sub/.GIT"' "$1" "$2"
t ln -s "$2" "$1/sub/.git"
t sh -c 'mkdir -p "$0/deep/er" && cd "$0/deep/er" && /usr/bin/git init -q' "$1"
t sh -c 'mkdir -p "$0/.github/workflows" && echo x > "$0/.github/workflows/ci.yml"' "$1"
t sh -c 'echo build >> "$0/.gitignore"' "$1"
t sh -c 'mkdir -p "$0/plain/sub" && echo x > "$0/plain/sub/f"' "$1"
t mv "$1/plain" "$1/plain2"`
	const want = "refused refused refused refused refused ok ok ok ok"
	if got := sandboxedSh(t, r6Desk(t, l, "r6m", notes), notes, r6Tries+makes+`
t mkdir "$1/.git"
t sh -c 'echo ref: refs/heads/main > "$0/HEAD"' "$1"`, notes, out); strings.Join(got, " ") != want+" refused refused" {
		t.Errorf("a desk at a plain folder: %q, want %q", got, want+" refused refused")
	}
	profile, _ := l.profile(t, l.hire)
	if got := sandboxedSh(t, profile, l.hireTree, r6Tries+makes, l.hireTree, out); strings.Join(got, " ") != want {
		t.Errorf("a hire in its worktree: %q, want %q", got, want)
	}
	for _, at := range []string{notes + "/sub/.git", notes + "/sub/.GIT", notes + "/deep/er/.git", notes + "/.git", notes + "/HEAD", l.hireTree + "/sub/.git"} {
		if _, err := os.Lstat(at); err == nil {
			t.Errorf("%s was made", strings.TrimPrefix(at, l.base))
		}
	}
}

// The controls: a hire still commits in its own worktree, and a desk at a checkout still writes its own .git's
// objects; its commit there is refused as it was before (the shared git places hold the index: N1).
func TestADesksOwnGitWorkIsUnchanged(t *testing.T) {
	l := newHireProjectsLayout(t)
	commit := `set -e
c="-c user.name=Hire -c user.email=hire@example.invalid -c commit.gpgsign=false"
echo own > own.txt
/usr/bin/git $c add own.txt
/usr/bin/git $c commit -q -m own
/usr/bin/git log --format=%s -1`
	profile, _ := l.profile(t, l.hire)
	if got := sandboxedSh(t, profile, l.hireTree, commit); strings.Join(got, " ") != "own" {
		t.Errorf("the hire's commit in its worktree: %q", got)
	}
	got := sandboxedSh(t, r6Desk(t, l, "r6c", l.lib), l.lib, r6Tries+`t sh -c 'echo x | /usr/bin/git hash-object -w --stdin'
t sh -c 'echo own > own.txt'
t /usr/bin/git -c user.name=H -c user.email=h@example.invalid -c commit.gpgsign=false commit -q --allow-empty -m own`)
	if len(got) != 4 || got[1] != "ok" || got[2] != "ok" || got[3] != "refused" {
		t.Errorf("a desk at the checkout: %q, want an object, ok, ok, refused", got)
	}
}

// A bare repo below a place in a repo (clients, the projects folder a repo) refuses its hire and its start: no
// pattern tells it, and the owner's push into it runs its hooks.
func TestABareRepoBelowAPlaceInARepoRefusesItsHireAndStart(t *testing.T) {
	l := newHireProjectsLayout(t)
	clients, _, _, _ := clientsLayout(t, l)
	standInOwnersGit(t, l.projects, "init", "-q")
	standInOwnersGit(t, clients, "init", "-q", "--bare", "mirror")
	mirror := filepath.Join(clients, "mirror")
	at := tilde(clients) + " holds a bare git repository, " + tilde(mirror) + ", whose hooks your own git runs"
	if got, want := hirePlaceWords(clients, runGit), "On this Mac, each hire works in one project of its own, and "+at+". Hire it into a folder that holds none."; got != want {
		t.Errorf("its hire: %q, want %q", got, want)
	}
	h := Hire{ID: "r6r", Name: "Janitor", Role: "janitor", Folder: clients, State: "working"}
	writeRegistryHires(t, l.hire, h)
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := makeDeskSandbox(deskAtOf(h, deskSocketPath(h.ID, 0), nil, engineOptions{Git: runGit}))
	var words *errSandboxNotMade
	want := "This hire was set to work in " + tilde(clients) + ", but " + at + ", and on this Mac each hire works in one project of its own, so it didn't start. Fire it, then hire it again into a folder that holds none."
	if !errors.As(err, &words) || words.words != want {
		t.Errorf("its start: %v, want %q", err, want)
	}
}

// O2: the walk's bound in the owner's words has a comma for each thousand.
func TestTheWalksBoundIsSaidAsTheOwnerReadsIt(t *testing.T) {
	l := newHireProjectsLayout(t)
	big := filepath.Join(l.projects, "big")
	for i := range 1300 {
		ownerMade(t, big, fmt.Sprintf("f%04d/", i))
	}
	saved := gitBelowMost
	t.Cleanup(func() { gitBelowMost = saved })
	gitBelowMost = 1234
	want := "On this Mac, each hire works in one project of its own, and Smolcorp couldn't make sure " + tilde(big) + " holds no git repository: " + tilde(big) + " has more than 1,234 folders, more than Smolcorp looks through. Hire it into the repository you want it to work on, or into a smaller folder that holds none."
	if got := hirePlaceWords(big, runGit); got != want {
		t.Errorf("its hire: %q, want %q", got, want)
	}
}
