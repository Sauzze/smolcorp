package desktop

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Claude Code 2.1.329's own start, recorded in a pty (60x200, TERM=xterm-256color, a stand-in home, no sign-in,
// its first 8 seconds, nothing typed): testdata/claude-2.1.329-onboarding.jsonl is the first screen before any
// sign-in (the text style asked), testdata/claude-2.1.329-repl.jsonl the REPL, with the onboarding done, the folder
// trusted and a made-up key. One JSON string per line, so the bytes keep their line ends in the repo. Besides the
// palette's updates (FXSCREEN2031), both ask things that draw nothing on xterm and conhost: the background colour
// (OSC 11;?), the kitty keyboard's flags (CSI ?u), Claude Code's own question (OSC 7501;?), and in the REPL a link
// (OSC 8). Before FXSCREENSTART126 each one left a hire's screen unknown until a full reset, so mail to it never
// pasted, even after Claude Code's /clear. Until FXSCREENSTART126R2 the cursor saved and restored (ESC 7, ESC 8),
// DA1 and XTVERSION asked (CSI c, CSI >0q), the kitty keyboard's flags asked (CSI ?u) and two block glyphs (░, ▟)
// still left it unknown until that /clear, so a fresh hire's first mail waited.

// claudeRecording is a stream recorded from Claude Code, from testdata.
func claudeRecording(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var b strings.Builder
	lines := bufio.NewScanner(f)
	lines.Buffer(nil, 1<<20)
	for lines.Scan() {
		var l string
		if err := json.Unmarshal(lines.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		b.WriteString(l)
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// Each recording, fed whole or in chunks, draws what Claude Code shows and is readable at its end with nothing
// left to heal (FXSCREENSTART126R2): the onboarding's theme picker known (the reader names no state for it), the
// REPL idle. Both start with ESC 7 CSI r ESC 8 and ask DA1 (CSI c), XTVERSION (CSI >0q) and the kitty keyboard's
// flags (CSI ?u); the onboarding draws ░, the REPL's logo ▟. Each stays readable through Claude Code's next full
// redraw (its /clear).
func TestScreenReadsClaudeCode2129sOwnStartAtOnceAndAtItsNextFullRedraw(t *testing.T) {
	idle, trust := pane(t, "idle-empty"), pane(t, "trust-menu")
	start := []string{"\x1b[?2031h", "\x1b[?u", "\x1b]7501;?\x07", "\x1b7\x1b[r\x1b8", "\x1b[c", "\x1b[>0q"}
	for _, c := range []struct {
		name  string
		ends  PaneState // what the page reads at the end, known
		asked []string  // what the recording must hold, so the row reads what it says
		shows []string  // text the screen holds at the end
		next  string    // Claude Code's next page, after its /clear
		want  PaneState
	}{
		{"claude-2.1.329-onboarding", PaneUnknown, append([]string{"░", "\x1b]11;?\x07"}, start...),
			[]string{"Welcome to Claude Code", "Choose the text style that looks best with your terminal", "Dark mode (colorblind-friendly)"},
			trust, PaneMenu},
		{"claude-2.1.329-repl", PaneIdle, append([]string{"▟", "\x1b]8;id=", "\x1b]8;;\x07"}, start...),
			[]string{"~/ada-project", "https://code.claude.com/docs/en/permission-modes", "❯ Try \"fix typecheck errors\""},
			idle, PaneIdle},
	} {
		out := claudeRecording(t, c.name)
		for _, seq := range c.asked {
			if !strings.Contains(out, seq) {
				t.Fatalf("%s: the recording no longer holds %q", c.name, seq)
			}
		}
		for _, chunk := range []int{1, 7, 4096} {
			s := newTextScreen(60, 200, westernCP)
			for rest := []byte(out); len(rest) > 0; rest = rest[min(chunk, len(rest)):] {
				s.Write(rest[:min(chunk, len(rest))])
			}
			if s.Unknown() || s.Why() != "" || s.needsReset != "" || s.PaneState() != c.ends {
				t.Errorf("%s, chunks of %d: %v, unknown %v, why %q (until a full reset: %q), want %v with the screen known", c.name, chunk, s.PaneState(), s.Unknown(), s.Why(), s.needsReset, c.ends)
			}
			for _, text := range c.shows {
				if !strings.Contains(s.Text(), text) {
					t.Errorf("%s, chunks of %d: the screen doesn't show %q", c.name, chunk, text)
				}
			}
			if s.Write([]byte("\x1b[2J\x1b[3J\x1b[H" + ptyOutput(c.next))); s.Unknown() || s.PaneState() != c.want {
				t.Errorf("%s, chunks of %d, then a /clear: %v, unknown %v (why %q), want %v", c.name, chunk, s.PaneState(), s.Unknown(), s.Why(), c.want)
			}
		}
	}
}

// claudeStartQuestionSteps is claudeSteps with 2.1.329's start: the kitty keyboard's flags asked (CSI ?u, where
// claudeStart has >1u), the background colour and Claude Code's own question asked as it starts and again while a
// draft shows, and the footer drawn inside a link once the reply is done.
func claudeStartQuestionSteps(t *testing.T, asked string) []claudeStep {
	steps := claudeSteps(t)
	if !strings.Contains(steps[0].out, "\x1b[>1u") || !strings.Contains(steps[4].out, claudeFooter) || steps[2].want != PaneDraft {
		t.Fatal("claudeSteps changed: no >1u at its start, no footer drawn when done, or no draft third")
	}
	steps[0].out = strings.Replace(steps[0].out, "\x1b[>1u", "\x1b]11;?\x07\x1b[?u"+asked+"\x1b]7501;?\x1b\\", 1)
	steps[2].out += "\x1b]11;?\x1b\\\x1b]7501;?\x07"
	steps[4].out = strings.Replace(steps[4].out, claudeFooter, "\x1b]8;id=a1;https://example.invalid/docs\x07"+claudeFooter+"\x1b]8;;\x07", 1)
	return steps
}

func TestScreenReadsClaudeStepsWithClaudeCode2129sStartQuestions(t *testing.T) {
	for _, chunk := range []int{1, 7, 4096} {
		s := newTextScreen(60, 200, westernCP)
		for _, step := range claudeStartQuestionSteps(t, "") {
			for out := []byte(step.out); len(out) > 0; out = out[min(chunk, len(out)):] {
				s.Write(out[:min(chunk, len(out))])
			}
			if s.Unknown() || s.PaneState() != step.want {
				t.Errorf("chunks of %d, %s: %v, unknown %v (why %q), want %v", chunk, step.name, s.PaneState(), s.Unknown(), s.Why(), step.want)
			}
			if got := paneLinesTrimmed(s.Text()); got != paneLinesTrimmed(step.text) {
				t.Errorf("chunks of %d, %s: the screen is\n%s", chunk, step.name, got)
			}
		}
	}
	// A link's text is ordinary text, drawn where the link opened; the link draws nothing.
	if s := drawn(t, 2, 20, "\x1b]8;;https://example.invalid/\x07link\x1b]8;;\x1b\\ end"); s.Text() != "link end\n" || s.Unknown() {
		t.Errorf("a link: %q, unknown %v (why %q)", s.Text(), s.Unknown(), s.Why())
	}
}

// The controls: every other body of OSC 11 and OSC 7501 (a colour set, another question), the kitty keyboard's
// CSI ? with a number, the colour questions 2.1.329 doesn't send (OSC 10;?, OSC 4;n;?) and a link with a control
// inside or no params, in the same place: each still breaks the screen for good, through every step, until a full
// reset.
func TestScreenStillBreaksOnTheStringsAndCSIsOffTheListAtClaudeCodesStart(t *testing.T) {
	for _, c := range []struct{ seq, why string }{
		{"\x1b]7501;1\x07", "OSC 7501"},
		{"\x1b]7501;?;x\x1b\\", "OSC 7501"},
		{"\x1b]11;#000000\x07", "OSC 11"},
		{"\x1b]11;rgb:0000/0000/0000\x1b\\", "OSC 11"},
		{"\x1b]11;??\x07", "OSC 11"},
		{"\x1b[?5u", "CSI ?5u"},
		{"\x1b[?1;2u", "CSI ?1;2u"},
		{"\x1b]10;?\x07", "OSC 10"},
		{"\x1b]4;1;?\x07", "OSC 4"},
		{"\x1b]8;;http://x\ty\x07", "OSC 8"},
		{"\x1b]8;http://x\x07", "OSC 8"},
	} {
		s := newTextScreen(60, 200, westernCP)
		for _, step := range claudeStartQuestionSteps(t, c.seq) {
			s.Write([]byte(step.out))
			if !s.Unknown() || s.PaneState() != PaneUnknown || s.Why() != c.why {
				t.Errorf("%q, %s: %v, unknown %v, why %q, want %q", c.seq, step.name, s.PaneState(), s.Unknown(), s.Why(), c.why)
			}
		}
		if s.Write([]byte("\x1bc" + ptyOutput(pane(t, "idle-empty")))); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%q, then a full reset: %v, unknown %v (why %q)", c.seq, s.PaneState(), s.Unknown(), s.Why())
		}
	}
	// One of the four cut off by another sequence, as a title cut off: unknown until a full redraw.
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, c := range []struct{ seq, why string }{
		{"\x1b]11;?\x1b[H", "OSC 11 cut off by another sequence"},
		{"\x1b]7501;?\x1b[H", "OSC 7501 cut off by another sequence"},
		{"\x1b]8;;http://x\x1b[H", "OSC 8 cut off by another sequence"},
	} {
		s := drawn(t, 60, 200, idle+c.seq+idle)
		if !s.Unknown() || s.needsReset != "" || s.Why() != c.why {
			t.Errorf("%q: unknown %v, why %q (until a full reset: %q)", c.seq, s.Unknown(), s.Why(), s.needsReset)
		}
		if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%q, then a full redraw: %v, unknown %v (why %q)", c.seq, s.PaneState(), s.Unknown(), s.Why())
		}
	}
}

// The questions Claude Code asks as it starts are on the list only in its own spelling (FXSCREENSTART126R2): DA1
// (CSI c, 0c), XTVERSION (CSI >q, >0q) and the kitty keyboard's flags (CSI ?u). Spelled otherwise they break as
// before: until a full redraw, or as of unknown effect until a full reset.
func TestScreenTakesClaudeCodesStartQuestionsOnlyInItsOwnSpelling(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, c := range []struct {
		seqs  []string
		heals int // 0: on the list; 1: until a full redraw; 2: until a full reset
	}{
		{[]string{"\x1b[c", "\x1b[0c", "\x1b[>q", "\x1b[>0q", "\x1b[?u"}, 0},
		{[]string{"\x1b[00c", "\x1b[>00q", "\x1b[>1q", "\x1b[>0;0q", "\x1b[>u"}, 1},
		{[]string{"\x1b[1c", "\x1b[>c", "\x1b[=c", "\x1b[?c", "\x1b[0;0c", "\x1b[?0u", "\x1b[?5u", "\x1b[=u"}, 2},
	} {
		for _, seq := range c.seqs {
			s := drawn(t, 60, 200, idle+seq)
			if s.Unknown() != (c.heals > 0) || (c.heals == 0 && s.PaneState() != PaneIdle) {
				t.Errorf("%q: %v, unknown %v, why %q", seq, s.PaneState(), s.Unknown(), s.Why())
			}
			if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() != (c.heals > 1) {
				t.Errorf("%q, then a full redraw: unknown %v, why %q", seq, s.Unknown(), s.Why())
			}
		}
	}
}

// ESC 7 and ESC 8 are on the list (FXSCREENSTART126R2), but a restore breaks the page until a full redraw where a
// terminal may restore otherwise than the screen: with a scroll region set at the restore, and with a saved style
// other than the pen (Windows 10's conhost may restore the position only). Either way the cursor is where xterm and
// conhost put it, and the text is the same.
func TestScreenBreaksAtAnESC8WithARegionSetOrAnotherStyleSaved(t *testing.T) {
	box := "\x1b[2J\x1b[H" + boxAt(6, "❯")
	for _, c := range []struct{ name, out, why string }{
		{"a region set at the restore", box + "\x1b[7;3H\x1b7\x1b[2;9r\x1b8hello\x1b[r", "ESC 8 with a scroll region set"},
		{"a region set at the restore, and another style saved",
			box + "\x1b[7;3H\x1b[2m\x1b7\x1b[22m\x1b[2;9r\x1b8\x1b[22mhello\x1b[r", "ESC 8 with a scroll region set"},
		{"saved faint, restored over a plain pen", box + "\x1b[7;3H\x1b[2m\x1b7\x1b[22m\x1b[1;1Hx\x1b8\x1b[22mhello", "ESC 8 with another style saved"},
		{"saved plain, restored over a faint pen", box + "\x1b[7;3H\x1b7\x1b[2m\x1b8hello", "ESC 8 with another style saved"},
		{"nothing saved, a faint pen", box + "\x1b[2m\x1b8\x1b[22m\x1b[7;3Hhello", "ESC 8 with another style saved"},
		{"a region set at the save only", box + "\x1b[2;9r\x1b[7;3H\x1b7\x1b[r\x1b8hello", ""},
		{"Claude Code's ESC 7 CSI r ESC 8", box + "\x1b[7;3H\x1b7\x1b[r\x1b8hello", ""},
		{"saved and restored faint", box + "\x1b[2m\x1b[7;3H\x1b7\x1b[1;1Hx\x1b8\x1b[22mhello", ""},
	} {
		s := drawn(t, 10, 40, c.out)
		if s.Unknown() != (c.why != "") || s.Why() != c.why || s.needsReset != "" {
			t.Errorf("%s: unknown %v, why %q (until a full reset: %q), want %q", c.name, s.Unknown(), s.Why(), s.needsReset, c.why)
		}
		if !strings.Contains(s.Text(), "\n❯ hello\n") {
			t.Errorf("%s: the screen is %q, want the draft where xterm and conhost put it", c.name, s.Text())
		}
		if s.Write([]byte("\x1b[r\x1b[22m\x1b[2J\x1b[H" + boxAt(6, "❯"))); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%s, then a full redraw: %v, unknown %v, why %q", c.name, s.PaneState(), s.Unknown(), s.Why())
		}
	}
}

// The tester's B0, end to end: a fresh hire's first mail through the launcher, on Claude Code 2.1.329's recorded
// start (the REPL), with no full redraw and no resize before it, pastes at the first try (FXSCREENSTART126R2).
func TestAFreshHiresFirstMailPastesOnClaudeCode2129sStart(t *testing.T) {
	r := newPumpRig(t)
	d := newTestDeck(t)
	f := newFakeLauncher(t, d, "L1", "/work/api", "", "idle-empty", false)
	f.screen.Write([]byte("\x1bc")) // as fresh as a new hire's screen
	f.screen.Write([]byte(claudeRecording(t, "claude-2.1.329-repl")))
	if state, unknown, bracketed, why := f.screen.lookWhy(); state != PaneIdle || unknown || !bracketed {
		t.Errorf("before the mail: %v, unknown %v (why %q), bracketed %v", state, unknown, why, bracketed)
	}
	f.react = takesTheDraft(t)
	r.hires = []Hire{launcherHire()}
	r.pump.typeIn = func(h Hire, text string, commit func() bool) mailResult {
		return d.mailTypeIntoLauncher(h, text, commit, noPause)
	}
	mustAdd(t, r.o, toHire("abcdef123456", "Rev", "Please review T7", "m1"))
	r.pump.round()
	if m := r.msg("m1"); m.State != mailDelivered || m.Attempts != 1 {
		t.Fatalf("a fresh hire's first mail: %+v", m)
	}
	want := pasteStart + "— from the GM (Smolcorp), message m1\n\nPlease review T7" + pasteEnd
	if got := f.writtenAfter(2); len(got) != 2 || got[0] != want || got[1] != keyBytes[Enter] {
		t.Errorf("wrote %q, want %q then Return", got, want)
	}
}
