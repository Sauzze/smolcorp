package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// deskChecks is one list for a desk's start (SA-M6) and the core's runs in its sandbox (SA-X6): every check on it is
// held by a row on both sides (RMSYSAX6 B2). Each side gets a desk as a real install has it (the guard file in the
// support folder, the worktree's .claude/settings.json, a brief in its own folder), then the same profile with one
// hole: each refuses, through the real /usr/bin/sandbox-exec, on stand-ins only; the profile as made runs.

// fenced is what a desk's checks find on a real install: the guard file, the worktree's fence and the desk's brief.
type fenced struct{ guard, fence, brief, guardWas string }

// fenceDesk writes the guard file in the support folder (taken away at the end when this made it), root's
// .claude/settings.json, and a brief in desk id's own folder.
func fenceDesk(t *testing.T, id, root string) fenced {
	t.Helper()
	f := fenced{guard: filepath.Join(supportDir, hireGuardFile), fence: filepath.Join(root, ".claude", "settings.json"),
		brief: filepath.Join(hireFolder(supportDir, id), "BRIEF-ada.md")}
	if _, err := os.Lstat(f.guard); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(f.guard, []byte(`{"groups":[]}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Remove(f.guard) })
	} else if err != nil {
		t.Fatal(err)
	}
	was, err := os.ReadFile(f.guard)
	if err != nil {
		t.Fatal(err)
	}
	f.guardWas = string(was)
	if err := os.MkdirAll(filepath.Dir(f.fence), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(f.brief), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{f.fence, f.brief} {
		if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// unchanged is whether nothing in the sandbox wrote the guard file or the worktree's fence, hole or not.
func (f fenced) unchanged(t *testing.T) {
	t.Helper()
	for file, was := range map[string]string{f.guard: f.guardWas, f.fence: "{}\n"} {
		if data, err := os.ReadFile(file); err != nil || string(data) != was {
			t.Errorf("%s was changed: %q %v", file, data, err)
		}
	}
}

// deskHole is a profile that doesn't hold one check: a rule added after all of the profile's own, or a brief the
// desk can't read.
type deskHole struct {
	name string
	rule func(f fenced, profile, status, sockets string) string
	// strayBrief puts the brief in the support folder, outside the desk's own.
	strayBrief bool
}

func allowWrite(path string) string {
	return "(allow file-write* (literal \"" + physical(path) + "\"))"
}

var deskHoles = []deskHole{
	{name: "its profile", rule: func(_ fenced, profile, _, _ string) string { return allowWrite(profile) }},
	{name: "the guard file", rule: func(f fenced, _, _, _ string) string { return allowWrite(f.guard) }},
	{name: "the worktree's fence", rule: func(f fenced, _, _, _ string) string { return allowWrite(f.fence) }},
	{name: "its brief", strayBrief: true},
	{name: "its status", rule: func(_ fenced, _, status, _ string) string {
		// Its folder as it really is: the status isn't there yet, so physical can't resolve it whole.
		return "(deny file-write* (literal \"" + filepath.Join(physical(filepath.Dir(status)), filepath.Base(status)) + "\"))"
	}},
	{name: "the other server", rule: func(_ fenced, _, _, sockets string) string {
		return "(allow network-outbound (subpath \"" + physical(sockets) + "\"))"
	}},
}

// addToProfile adds rule at the end of the profile.
func addToProfile(t *testing.T, profile, rule string) {
	t.Helper()
	file, err := os.OpenFile(profile, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString("\n" + rule + "\n")
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
}

// strayBrief is a brief in the support folder, outside desk's own folder, which its profile doesn't let it read.
func strayBrief(t *testing.T) string {
	t.Helper()
	stray := filepath.Join(supportDir, "BRIEF-stray.md")
	if err := os.WriteFile(stray, []byte("the brief\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(stray) })
	return stray
}

// M6's side: a desk's start, end to end through the real sandbox-exec and `enter`, with the guard, the worktree's
// fence and its brief on its list; the start's profile with one hole refuses it with §A5.2's words.
func TestOnAMacADesksStartIsRefusedWhereAnyOfItsChecksDoesntHold(t *testing.T) {
	if err := sandbox.CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(err)
		}
		t.Skipf("this Mac's sandbox-exec isn't Apple's alone: %v", err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	start := func(t *testing.T, hole deskHole) error {
		t.Setenv(deskStandInVariable, "1")
		s := standInDesk(t)
		f := fenceDesk(t, s.at.ID, s.at.Root)
		s.at.Brief = f.brief
		if hole.strayBrief {
			s.at.Brief = strayBrief(t)
		}
		profile := filepath.Join(hireFolder(supportDir, s.at.ID), "sandbox.sb")
		status := filepath.Join(hireFolder(supportDir, s.at.ID), "STATUS.md")
		// What the start checks, from the one list: every fence and the brief.
		c := s.checks(profile)
		for _, want := range []string{profile, f.guard, f.fence} {
			if !containsPath(c.NoWrite, want) {
				t.Errorf("the start doesn't check %s: %+v", want, c)
			}
		}
		if len(c.Read) != 1 || c.Read[0] != s.at.Brief || len(c.Write) != 1 || c.Write[0] != status {
			t.Errorf("the start's read and write: %+v", c)
		}
		saved := deskStartRunner
		deskStartRunner.command = func(st *sandbox.Start, profile, smolcorp string, env []string, program string, args ...string) (startedCmd, error) {
			if hole.rule != nil {
				addToProfile(t, profile, hole.rule(f, profile, status, s.places.SocketParent))
			}
			return macStartCommand(st, profile, smolcorp, env, program, args...)
		}
		t.Cleanup(func() { deskStartRunner = saved })
		socket := filepath.Join(s.places.OwnSockets, deskSocketFile)
		_, server, err := s.starter(self, self)([]string{"-S", socket, "start-server", ";", "new-session", "-d"})
		if server > 0 {
			t.Cleanup(func() { syscall.Kill(server, syscall.SIGKILL) })
		}
		f.unchanged(t)
		return err
	}
	t.Run("the profile as made", func(t *testing.T) {
		if err := start(t, deskHole{}); err != nil {
			t.Fatalf("the control: the start was refused: %v", err)
		}
	})
	for _, hole := range deskHoles {
		t.Run(hole.name, func(t *testing.T) {
			if err := start(t, hole); err == nil || err.Error() != sandbox.Refusal("darwin") {
				t.Errorf("a profile that doesn't hold %s: %v, want refused with %q", hole.name, err, sandbox.Refusal("darwin"))
			}
		})
	}
}

// X6's side: the core's git on the desk's tree, run in the desk's sandbox with the same desk (its row: its worktree,
// its brief) and the guard file; the same profile with one hole refuses the run, and git never runs outside instead.
func TestOnAMacTheCoresRunIsRefusedWhereAnyOfItsDesksChecksDoesntHold(t *testing.T) {
	run := func(t *testing.T, hole deskHole) bool {
		r := newDeskRunRig(t)
		profile := sandboxedHire(t, r)
		f := fenceDesk(t, r.hire.ID, r.tree)
		r.hire.Brief = f.brief
		if hole.strayBrief {
			r.hire.Brief = strayBrief(t)
		}
		writeRegistry(t, r.hire)
		status := filepath.Join(hireFolder(supportDir, r.hire.ID), "STATUS.md")
		c, err := deskRunChecks(r.hire.ID, profile)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{profile, f.guard, f.fence} {
			if !containsPath(c.NoWrite, want) {
				t.Errorf("the run doesn't check %s: %+v", want, c)
			}
		}
		if len(c.Read) != 1 || c.Read[0] != r.hire.Brief || len(c.Write) != 1 || c.Write[0] != status {
			t.Errorf("the run's read and write: %+v", c)
		}
		if hole.rule != nil {
			addToProfile(t, profile, hole.rule(f, profile, status, deskSocketParent()))
		}
		_, _, ok := runGit(r.tree, "symbolic-ref", "--short", "-q", "HEAD")
		f.unchanged(t)
		return ok
	}
	t.Run("the profile as made", func(t *testing.T) {
		if !run(t, deskHole{}) {
			t.Fatal("the control: the core's git didn't run in the profile as made")
		}
	})
	for _, hole := range deskHoles {
		t.Run(hole.name, func(t *testing.T) {
			if run(t, hole) {
				t.Errorf("the core's git ran in a profile that doesn't hold %s", hole.name)
			}
		})
	}
}

// containsPath is whether paths has path, either as given or as it really is.
func containsPath(paths []string, path string) bool {
	for _, p := range paths {
		if p == path || physical(p) == physical(path) {
			return true
		}
	}
	return false
}
