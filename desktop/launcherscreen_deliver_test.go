package desktop

import (
	"strings"
	"testing"
)

// Mail is delivered to a window Claude Code drew as it does on Windows (a try counted), waits with no try
// while the screen can't be read, and goes once it's redrawn whole.
func TestMailIsDeliveredToAWindowDrawnAsClaudeCodeDrawsIt(t *testing.T) {
	r := newPumpRig(t)
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", true)
	f.screen.Write([]byte(claudeStream(t, "done: the spinner gone, the box idle")))
	f.react = takesTheDraft(t)
	r.hires = []Hire{launcherHire()}
	r.pump.typeIn = func(h Hire, text string, commit func() bool) mailResult {
		return d.mailTypeIntoLauncher(h, text, commit, noPause)
	}
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "Please review T7", "m1"))
	r.pump.round()
	if m := r.msg("m1"); m.State != mailDelivered || m.Attempts != 1 {
		t.Fatalf("to Claude Code's window: %+v", m)
	}
	if got := f.writtenAfter(2); len(got) != 2 || !strings.HasSuffix(got[0], "Please review T7"+pasteEnd) || got[1] != keyBytes[Enter] {
		t.Errorf("wrote %q", got)
	}

	// The screen breaks: the mail waits, nothing typed, no try counted.
	f.show("", pane(t, "idle-empty"))
	f.screen.Write([]byte("\x1b[3b"))
	before := len(f.written())
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "second", "m2"))
	r.pump.round()
	if m := r.msg("m2"); m.State == mailDelivered || m.Attempts != 0 || len(f.written()) != before {
		t.Errorf("on a broken screen: %+v, wrote %q", m, f.written()[before:])
	}
	// Claude Code clears and redraws: the next round delivers it.
	f.screen.Write([]byte("\x1b[2J\x1b[3J\x1b[H" + ptyOutput(pane(t, "idle-empty"))))
	r.clock.advance(mailPumpEvery)
	r.pump.round()
	if m := r.msg("m2"); m.State != mailDelivered || m.Attempts != 1 {
		t.Errorf("after the redraw: %+v", m)
	}
}
