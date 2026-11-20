package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

// RT126CRLFRULING's recommendation (FXLINEENDS): the fire's look at a sandboxed Mac desk's tree (deskGit in the
// desk's sandbox, on the repo's own config) carries the owner's line ends as well, below the repo's own setting as
// git has it, so it agrees with the owner's git. Stand-ins as lineends_test.go's, and the sandbox as
// deskrun_darwin_test.go's.
func TestOnAMacTheFiresLookInADesksSandboxAgreesWithTheOwnersGitOnLineEnds(t *testing.T) {
	lineEndOwner(t)
	r := newDeskRunRig(t)
	sandboxedHire(t, r) // the home is now isolate's own: the owner's ~/.gitconfig below is written there
	file := filepath.Join(r.tree, "a.txt")
	// The owner's environment asks for CRLF, and the hire's own git restores a file: CRLF, and clean to the owner.
	ownerEnvironment(t, "core.autocrlf", "true")
	os.Remove(file)
	ownersGit(r.tree, "checkout", "--", "a.txt")
	laterStamp(t, file)
	if got, own := lineEndsOf(t, file), ownersGit(r.tree, "status", "--porcelain"); got != "CRLF" || own != "" {
		t.Fatalf("the hire's own git restored a.txt as %s, and calls the tree %q: this row proves nothing", got, own)
	}
	if message := uncommittedRefusal(r.hire, runGit); message != "" {
		t.Errorf("the fire's look, in the desk's sandbox, at a tree the owner's git calls clean: %q", message)
	}
	// The owner's global core.autocrlf=true under the repo's own false: the owner's git calls the CRLF file changed,
	// and so must the core.
	ownerEnvironment(t)
	os.Unsetenv("GIT_CONFIG_COUNT")
	if err := os.WriteFile(filepath.Join(homeDir, ".gitconfig"), []byte("[core]\n\tautocrlf = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, r.repo, "config", "core.autocrlf", "false")
	laterStamp(t, file)
	own := ownersGit(r.tree, "status", "--porcelain")
	if own == "" {
		t.Fatal("the owner's git calls the CRLF file unchanged under the repo's own core.autocrlf=false: this row proves nothing")
	}
	if status, out, ok := runGit(r.tree, "status", "--porcelain"); !ok || status != 0 || out != own {
		t.Errorf("the core's status in the desk's sandbox: %d %q %v, want %q as the owner's git says", status, out, ok, own)
	}
}
