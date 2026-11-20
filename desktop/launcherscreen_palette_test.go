package desktop

import (
	"strings"
	"testing"
)

// Claude Code from 2.1.327 at least asks for the colour palette's updates as it starts (CSI ?2031h, between bracketed
// paste and focus reports: recorded from 2.1.329's start in a pty), so the terminal reports a theme change as
// CSI ?997;1n or 2n. It draws nothing, and before FXSCREEN2031 it left a hire's screen unknown for good, even after
// its /clear, so mail to it never pasted.

// claudePaletteSteps is claudeSteps with the palette's updates asked for at the start, Claude Code's /clear after
// the reply, and the mode reset as it leaves.
func claudePaletteSteps(t *testing.T, mode string) []claudeStep {
	steps := claudeSteps(t)
	if !strings.Contains(steps[0].out, "\x1b[?2004h\x1b[?1004h") {
		t.Fatal("claudeStart no longer asks for bracketed paste, then focus reports")
	}
	steps[0].out = strings.Replace(steps[0].out, "\x1b[?2004h\x1b[?1004h", "\x1b[?2004h"+mode+"h\x1b[?1004h", 1)
	idle := pane(t, "idle-empty")
	return append(steps,
		claudeStep{"its /clear", "\x1b[2J\x1b[3J\x1b[H" + inkFrame("", idle), PaneIdle, idle},
		claudeStep{"the mode reset", mode + "l", PaneIdle, idle},
	)
}

func TestScreenReadsClaudeCodesStartThatAsksForThePalettesUpdates(t *testing.T) {
	for _, chunk := range []int{1, 7, 4096} {
		s := newTextScreen(60, 200, westernCP)
		for _, step := range claudePaletteSteps(t, "\x1b[?2031") {
			for out := []byte(step.out); len(out) > 0; out = out[min(chunk, len(out)):] {
				s.Write(out[:min(chunk, len(out))])
			}
			if s.Unknown() || s.PaneState() != step.want {
				t.Errorf("chunks of %d, %s: %v, unknown %v (why %q), want %v", chunk, step.name, s.PaneState(), s.Unknown(), s.Why(), step.want)
			}
		}
	}
	// Set and reset with other modes on the list, as one sequence.
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, seq := range []string{"\x1b[?2004;2031;1004h", "\x1b[?2031;25l"} {
		if s := drawn(t, 60, 200, seq+"\x1b[2J\x1b[H"+idle); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%q: %v, unknown %v (why %q)", seq, s.PaneState(), s.Unknown(), s.Why())
		}
	}
}

// The control: a mode not on the list, in the same place, still breaks the screen for good, through Claude Code's
// clear and every frame after it, until a full reset.
func TestScreenStillBreaksOnAModeNotOnTheListAtClaudeCodesStart(t *testing.T) {
	s := newTextScreen(60, 200, westernCP)
	for _, step := range claudePaletteSteps(t, "\x1b[?2032") {
		s.Write([]byte(step.out))
		if !s.Unknown() || s.PaneState() != PaneUnknown || s.Why() != "CSI ?2032h" {
			t.Errorf("%s: %v, unknown %v, why %q", step.name, s.PaneState(), s.Unknown(), s.Why())
		}
	}
	if s.Write([]byte("\x1bc" + ptyOutput(pane(t, "idle-empty")))); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("after a full reset: %v, unknown %v (why %q)", s.PaneState(), s.Unknown(), s.Why())
	}
}
