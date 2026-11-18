package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// FXGITPLANTSWARN on a Mac's sandboxed start (makeDeskSandbox): a desk at a project whose .git holds a replace ref
// the owner hasn't answered doesn't start, with the words for them, and the ref is listed for Setup; once they keep
// it, the desk starts. A clean repo's desk starts with nothing asked.
func TestOnAMacADeskDoesntStartWhileASubstitutionIsUnanswered(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git here")
	}
	freshGitPlants(t)
	shortSupport(t)
	freshGitKept(t)
	homeDir = t.TempDir()
	r, clean := aSubstRepo(t), aSubstRepo(t)
	gitRepoCmd(t, r.dir, "replace", r.b, r.d)
	deskSockets(t)
	t.Setenv("PATH", filepath.Dir(git)+":"+os.Getenv("PATH")) // the same git as the repos were made with
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := func(root string) error {
		_, err := makeDeskSandbox(deskAt{ID: "ab12", Root: root, Socket: deskSocketPath("ab12", 0), Git: runGit})
		return err
	}

	err = start(r.dir)
	var refused *errSandboxNotMade
	held := heldAt(t, r.gitOf())
	if !errors.As(err, &refused) || len(held) != 1 || refused.words != gitSubstRefusal(held) {
		t.Fatalf("the start with a replace ref unanswered: %v (held %+v)", err, held)
	}
	if !slices.ContainsFunc(listedGitPlants(), func(p oldGitPlant) bool { return p.ID == held[0].ID }) {
		t.Error("the refused start didn't list it for Setup")
	}
	if err := removeGitPlant(held[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := start(r.dir); err != nil {
		t.Errorf("kept, the start: %v", err)
	}
	if slices.ContainsFunc(listedGitPlants(), func(p oldGitPlant) bool { return p.Held }) {
		t.Error("kept, but still listed")
	}
	if err := start(clean.dir); err != nil {
		t.Errorf("a clean repo's start: %v", err)
	}
}
