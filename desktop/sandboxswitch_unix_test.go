//go:build !windows

package desktop

import (
	"testing"
)

// SA-X13 on the engine: a desk started in its sandbox is tagged so in its row; one started while the owner has the
// sandbox off starts as before, outside it, untagged, and keeps that after the sandbox is on again.
func TestDesksStartedWhileTheSandboxIsOffKeepTheirTag(t *testing.T) {
	r := newSandboxRig(t)
	before, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	r.o.SandboxOff = true
	r.tmux.up = false
	off, err := runManagedHire(r.request("reviewer"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.desks) != 1 || len(r.starts) != 1 || !r.tmux.ran("new-session") {
		t.Errorf("a desk started with the sandbox off went through its sandbox: %d sandboxes, starts %q", len(r.desks), r.starts)
	}
	r.o.SandboxOff = false
	after, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]Hire{}
	for _, h := range registered(t) {
		rows[h.ID] = h
	}
	if h := rows[before.ID]; !h.Sandboxed || !before.Sandboxed || !inSandbox(h) {
		t.Errorf("started in its sandbox: %+v", h)
	}
	if h := rows[off.ID]; h.Sandboxed || off.Sandboxed || h.TmuxServer != 0 || sandboxTag(h) == "" {
		t.Errorf("started with the sandbox off: %+v", h)
	}
	if h := rows[after.ID]; !h.Sandboxed || len(r.desks) != 2 {
		t.Errorf("started with the sandbox on again: %+v", h)
	}
}

// Take over takes a desk out of its sandbox, with its worktree (C6): its tag goes in the same save. Hand back
// tags it as its resume started it: in its sandbox, or not.
func TestTakeOverUntagsADeskAndHandBackTagsItAsItsResume(t *testing.T) {
	k := newT3TakeOverRig(t)
	k.hire.Sandboxed, k.saved.Sandboxed = true, true
	if _, err := t3TakeOver(k.hire, k.session, k.ways()); err != nil {
		t.Fatal(err)
	}
	if k.saved.Sandboxed || inSandbox(k.saved) {
		t.Fatalf("taken over, still tagged in its sandbox: %+v", k.saved)
	}
	for _, sandboxed := range []bool{true, false} {
		k.log = nil
		h := k.takenOver()
		ways := k.ways()
		ways.Resume = func(Hire, string) (t3Resumed, error) {
			return t3Resumed{Socket: deskSocketPath(h.ID, 0), Server: 5151, Sandboxed: sandboxed}, nil
		}
		if _, err := t3HandBack(h, ways); err != nil {
			t.Fatal(err)
		}
		if k.saved.Sandboxed != sandboxed || inSandbox(k.saved) != sandboxed {
			t.Errorf("handed back, resumed in its sandbox %v: %+v", sandboxed, k.saved)
		}
	}
}

// A start is tagged in its sandbox only with the server the core checked at it (§A5.2 step 9): one that says it went
// through its sandbox but brought back no server, or a pid no desk's server has (0 or 1), isn't (RT126SAX13 N2).
func TestOnlyAStartWithItsCheckedServerIsTaggedInItsSandbox(t *testing.T) {
	for _, c := range []struct {
		tmux hireTmux
		want bool
	}{
		{hireTmux{sandboxed: true, server: 5151}, true},
		{hireTmux{sandboxed: true}, false},
		{hireTmux{sandboxed: true, server: 1}, false},
		{hireTmux{server: 5151}, false},
	} {
		if got := startedInSandbox(&c.tmux); got != c.want {
			t.Errorf("sandboxed %v, server %d: tagged %v", c.tmux.sandboxed, c.tmux.server, got)
		}
	}
}
