package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// RT126FIXREVIEW-GITROWS2R1's B1 at a Mac desk's start (makeDeskSandbox), for a desk at a project whose default branch
// is team/main: with refs/heads/team a link to a folder elsewhere (a desk's worktree, say), made before by a desk or a
// program outside any sandbox, the owner's team/main is a file there, which the Policy's row for it, a path, wouldn't
// hold. The start is refused, with words naming the link, before anything of the desk runs, and nothing is made where
// it leads. Once the owner puts that folder in the link's place, the desk starts.
func TestOnAMacALinkOnTheWayToAHeldBranchRefusesTheStartInPlainWords(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	freshGitPlants(t)
	shortSupport(t)
	freshGitKept(t)
	homeDir = t.TempDir()
	r := aSubstRepo(t)
	link, elsewhere := filepath.Join(r.gitOf(), "refs", "heads", "team"), filepath.Join(physical(t.TempDir()), "team")
	if err := os.Mkdir(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, r.dir, "branch", "team/main") // written through the link
	gitRepoCmd(t, r.dir, "update-ref", "refs/remotes/origin/team/main", "team/main")
	gitRepoCmd(t, r.dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/team/main")
	deskSockets(t)
	t.Setenv("PATH", filepath.Dir(git)+":"+os.Getenv("PATH")) // the same git as the repo was made with
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := func() error {
		_, err := makeDeskSandbox(deskAt{ID: "ab12", Root: r.dir, Socket: deskSocketPath("ab12", 0), Git: runGit})
		return err
	}

	err = start()
	var refused *errSandboxNotMade
	if !errors.As(err, &refused) || !strings.HasPrefix(refused.words, link+" is a link, where git keeps this repository's branches") ||
		!strings.Contains(refused.words, "the desk didn't start") {
		t.Fatalf("the start with a link at %s: %v", link, err)
	}
	entries, err := os.ReadDir(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !slices.Equal(names, []string{"main"}) {
		t.Errorf("where the link leads: %q, want only git's main", names)
	}
	// The owner's answer: the folder it leads to, in its place.
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	if err := start(); err != nil {
		t.Errorf("with the folder in its place, the start: %v", err)
	}
}

// FXGITROWS2R4 (RT126FIXREVIEW-GITROWS2R3's B1) at a Mac desk's start: a desk ab12 at a project, whose master, a base
// branch, is a symbolic ref to the desk's own branch, made outside any sandbox (by a desk under an older profile, say).
// The desk's ordinary commit would move master. The start is refused with words that say where master points and how
// to make it a branch of its own, never to write the same text back; once the owner does what they say, the desk
// starts. A master that points at main, which no desk moves, is passed.
func TestOnAMacASymbolicRefAtABaseBranchRefusesTheStartInPlainWords(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	freshGitPlants(t)
	shortSupport(t)
	freshGitKept(t)
	homeDir = t.TempDir()
	r := aSubstRepo(t)
	deskSockets(t)
	t.Setenv("PATH", filepath.Dir(git)+":"+os.Getenv("PATH")) // the same git as the repo was made with
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := func() error {
		_, err := makeDeskSandbox(deskAt{ID: "ab12", Root: r.dir, Socket: deskSocketPath("ab12", 0), Git: runGit})
		return err
	}
	gitRepoCmd(t, r.dir, "branch", "hire/ab12/w")
	gitRepoCmd(t, r.dir, "symbolic-ref", "refs/heads/master", "refs/heads/hire/ab12/w")
	err = start()
	var refused *errSandboxNotMade
	if !errors.As(err, &refused) || !strings.HasPrefix(refused.words, "master isn't a ref of its own: it points at refs/heads/hire/ab12/w, which a desk can move") ||
		!strings.Contains(refused.words, "git update-ref --no-deref refs/heads/master <commit>") || strings.Contains(refused.words, "symbolic-ref") {
		t.Fatalf("the start with master pointing at the desk's branch: %v", err)
	}
	gitRepoCmd(t, r.dir, "symbolic-ref", "refs/heads/master", "refs/heads/main") // an alias of main: passed
	if err := start(); err != nil {
		t.Errorf("with master pointing at main, the start: %v", err)
	}
	gitRepoCmd(t, r.dir, "symbolic-ref", "refs/heads/master", "refs/heads/hire/ab12/w")
	gitRepoCmd(t, r.dir, "update-ref", "--no-deref", "refs/heads/master", "main") // the words' fix, at a commit the owner checked
	if err := start(); err != nil {
		t.Errorf("with master a branch of its own, the start: %v", err)
	}
}

// O2: the docs say what the keepers in a repository's .git are, and that they're safe to leave; and (FXGITROWS2R4's
// O3) that a link in refs/ or logs/ stops a desk's start.
func TestTheKeysPageSaysWhatTheKeepersInARepoAre(t *testing.T) {
	page := repoText(t, "docs", "keys-and-guard.md")
	mustSay(t, "docs/keys-and-guard.md", page, "`.smolcorp-keep`", "`.git/refs/heads/hire`", "safe to leave",
		"A link in a repository's `.git/refs` or `.git/logs` stops every desk's start there",       // FXGITROWS2R4, O3
		"a checkout's HEAD (yours, or another worktree's) that points at a branch a desk can move", // FXGITROWS2R5, R4's O3
		"An empty `.git/lfs` folder", "so no desk can put a link there") // FXGITROWS2R9, R8's O2
}
