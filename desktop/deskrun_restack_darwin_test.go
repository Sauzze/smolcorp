package desktop

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-X6 stacked on SA-M6 (MSYSAX6), on this Mac with stand-ins only: a run the core makes in a desk's sandbox checks
// what the desk's start checks (one source, deskChecks: RT126SAX6 N3), with its stand-in other server in the socket
// folder no desk can write (N4); and a registered desk's start, whose repo question runGit now routes into the
// profile of its last start, still finds its repo there, from the registry rather than its .git file.

// The run's checks are the start's, for the same desk: every fence, the brief, the status, and the other server in
// the socket folder.
func TestOnAMacADesksRunChecksWhatItsStartChecks(t *testing.T) {
	r := newDeskRunRig(t)
	profile := sandboxedHire(t, r)
	if err := os.MkdirAll(filepath.Join(r.tree, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.tree, ".claude", "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(hireFolder(supportDir, r.hire.ID), "BRIEF-ada.md")
	if err := os.WriteFile(brief, []byte("the brief\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.hire.Brief = brief
	writeRegistry(t, r.hire)

	// The start's own, as runManagedHire makes its desk (its worktree, its brief) and deskPlaces its socket parent.
	start := (&deskSandbox{at: deskAt{ID: r.hire.ID, Root: r.tree, Checkout: r.repo, Brief: brief},
		places: sandbox.Places{SocketParent: physical(deskSocketParent())}}).checks(profile)
	run, err := deskRunChecks(r.hire.ID, profile)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(run.NoWrite, start.NoWrite) || !slices.Equal(run.Read, start.Read) || !slices.Equal(run.Write, start.Write) || run.Support != start.Support {
		t.Errorf("the run's checks %+v aren't the start's %+v", run, start)
	}
	if !slices.Contains(run.NoWrite, filepath.Join(r.tree, ".claude", "settings.json")) || !slices.Equal(run.Read, []string{brief}) {
		t.Errorf("the run doesn't check the worktree's fence and the brief: %+v", run)
	}
	if filepath.Dir(run.Other) != physical(deskSocketParent()) || filepath.Dir(run.Other) != filepath.Dir(start.Other) {
		t.Errorf("the run's other server is at %s, not in the socket folder %s", run.Other, deskSocketParent())
	}
	if _, err := deskRunChecks("ffffffffffff", profile); err == nil {
		t.Error("a desk with no row was given checks")
	}
}

// A profile that lets the desk write its worktree's .claude/settings.json (the fence M6's start checks) refuses the
// core's run, which the same profile without that hole runs.
func TestOnAMacADesksRunIsRefusedWhereItsWorktreesFenceDoesntHold(t *testing.T) {
	r := newDeskRunRig(t)
	profile := sandboxedHire(t, r)
	settings := filepath.Join(r.tree, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, out, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); !ok || status != 0 || out != r.hire.Branch {
		t.Fatalf("the control: in the profile as made: %d %q %v", status, out, ok)
	}
	hole, err := os.OpenFile(profile, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = hole.WriteString("\n(allow file-write* (literal \"" + physical(settings) + "\"))\n")
	hole.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); ok {
		t.Error("the core's git ran in a sandbox that lets the desk write its worktree's fence")
	}
	if data, _ := os.ReadFile(settings); string(data) != "{}\n" {
		t.Errorf("the fence was changed: %q", data)
	}
}

// A brief the desk's profile can't read (one outside its own folder) refuses the run: `enter` reads the brief, as at
// the start; the same brief in its own folder runs.
func TestOnAMacADesksRunReadsItsBriefAsItsStartDoes(t *testing.T) {
	r := newDeskRunRig(t)
	sandboxedHire(t, r)
	inside := filepath.Join(hireFolder(supportDir, r.hire.ID), "BRIEF-ada.md")
	outside := filepath.Join(supportDir, "BRIEF-stray.md")
	for _, brief := range []string{inside, outside} {
		if err := os.WriteFile(brief, []byte("the brief\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { os.Remove(outside) })
	r.hire.Brief = inside
	writeRegistry(t, r.hire)
	if _, _, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); !ok {
		t.Fatal("the control: a brief in its own folder didn't run")
	}
	r.hire.Brief = outside
	writeRegistry(t, r.hire)
	if _, _, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); ok {
		t.Error("the run's check didn't read the brief, as the start's does")
	}
}

// The socket folder the run's other server goes in is the core's alone, as the start checks it: one others can open
// refuses the run.
func TestOnAMacADesksRunRefusesASocketFolderThatIsntTheCores(t *testing.T) {
	r := newDeskRunRig(t)
	sandboxedHire(t, r)
	if _, _, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); !ok {
		t.Fatal("the control: the run in the socket folder as made")
	}
	if err := os.Chmod(deskSocketParent(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD"); ok {
		t.Error("the run's other server went in a socket folder others can open")
	}
	if left, _ := os.ReadDir(deskSocketParent()); len(left) != 0 {
		t.Errorf("the runs left %d entries in the socket folder", len(left))
	}
}

// RT126SAX6R2's restack row: M6's start asks git where the desk's repo is through runGit, after the desk's row is in
// the registry, so a registered desk's start (Hand back's resume, a start after the core's restart) asks it in the
// profile its last start wrote. It still finds the repo, and from the registry: a .git file the desk rewrote to name
// another git folder doesn't move it. A last profile that doesn't hold refuses the start, never runs it outside.
func TestOnAMacARegisteredDesksStartFindsItsRepoInItsLastProfile(t *testing.T) {
	r := newDeskRunRig(t)
	profile := sandboxedHire(t, r)
	places := func() (sandbox.Places, error) {
		pl := sandbox.Places{Root: physical(r.tree)}
		err := deskGitPlaces(deskAt{ID: r.hire.ID, Root: r.tree, Checkout: r.repo, Git: runGit}, &pl)
		return pl, err
	}
	want := physical(filepath.Join(r.repo, ".git"))
	if pl, err := places(); err != nil || pl.GitDir != want || pl.OwnWorktree != "ada-abcdef" {
		t.Fatalf("in its last profile: %q %q %v, want %q ada-abcdef", pl.GitDir, pl.OwnWorktree, err, want)
	}

	decoy := filepath.Join(r.base, "decoy")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepoCmd(t, decoy, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(r.tree, ".git"), []byte("gitdir: "+filepath.Join(decoy, ".git")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := gitRepoCmd(t, r.tree, "rev-parse", "--path-format=absolute", "--git-common-dir"); !strings.Contains(out, "decoy") {
		t.Fatalf("the control: git in the tree as it is follows its .git to %q", out)
	}
	if pl, err := places(); err != nil || pl.GitDir != want || pl.OwnWorktree != "ada-abcdef" {
		t.Errorf("a rewritten .git moved the start's repo: %q %q %v", pl.GitDir, pl.OwnWorktree, err)
	}

	if err := os.WriteFile(profile, []byte("(version 1)\n(allow default)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if pl, err := places(); err == nil {
		t.Errorf("a last profile that doesn't hold: the start's repo question ran, %q", pl.GitDir)
	}
}
