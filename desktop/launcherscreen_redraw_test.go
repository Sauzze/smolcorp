package desktop

import (
	"fmt"
	"strings"
	"testing"
)

// A 10 by 40 window whose box is drawn by cursor moves (CSI row;1H) at row r: a rule, line, a rule, the footer.
var boxRule = strings.Repeat("─", 40)

func boxAt(r int, line string) string {
	return fmt.Sprintf("\x1b[%d;1H%s\x1b[%d;1H%s\x1b[%d;1H%s\x1b[%d;1H? for shortcuts", r, boxRule, r+1, line, r+2, boxRule, r+3)
}

// boxScreen is the text of a 10-row window holding that box at row r (1-based), as a terminal shows it.
func boxScreen(r int, line string) string {
	l := make([]string, 10)
	l[r-1], l[r], l[r+1], l[r+2] = boxRule, line, boxRule, "? for shortcuts"
	return strings.Join(l, "\n")
}

// A full redraw (CSI 2J, then CSI H) puts back the cells and the cursor, but a terminal keeps through it the saved
// cursors, the scroll region, the tab stops, and the modes and character sets a soft reset touches. A break that
// may have changed any of those keeps the screen unknown through the redraw. Each stream leaves a real terminal
// (xterm, conhost) holding a draft, worked out by hand: the screen must read unknown, or that draft.
func TestScreenStaysUnknownWhenABreakChangedWhatARedrawKeeps(t *testing.T) {
	// REP (CSI 242b) repeats the x 242 times on a terminal: its cursor ends at row 7, column 4.
	rep := "\x1b[1;1Hx\x1b[242b"
	for _, c := range []struct{ name, out, real string }{
		{"a cursor saved (ESC 7) while broken, restored (ESC 8) after the redraw",
			rep + "\x1b7\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b8hello", boxScreen(6, "❯  hello")},
		{"a cursor saved (CSI s) while broken, restored (CSI u) after the redraw",
			rep + "\x1b[s\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b[uhello", boxScreen(6, "❯  hello")},
		{"a cursor saved (CSI ?1048h) while broken, restored after the redraw",
			rep + "\x1b[?1048h\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b[?1048lhello", boxScreen(6, "❯  hello")},
		{"the alternate screen's cursor saved on an earlier visit (xterm and conhost keep it)",
			"\x1b[?1049h\x1b[7;4H\x1b7\x1b[?1049l\x1b[?1049h\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b8hello", boxScreen(6, "❯  hello")},
		{"a soft reset (CSI ! p) put the scroll region back to the whole screen",
			"\x1b[1;5r\x1b[!p\x1b[2J\x1b[H\x1b[5;1H" + boxRule + "\r\n❯\r\n" + boxRule + "\r\n? for shortcuts\x1b[6;3Hhello",
			boxScreen(5, "❯ hello")},
		{"the tab stops cleared (CSI 3 g): a tab goes to the last column and the text wraps",
			"\x1b[3g\x1b[2J\x1b[H\x1b[6;1H\t  hello" + boxAt(6, "❯"), boxScreen(6, "❯hello")},
	} {
		if want := ReadPaneState(c.real); want != PaneDraft {
			t.Fatalf("%s: the real screen reads %v", c.name, want)
		}
		for _, chunk := range []int{1, 4096} {
			s := newTextScreen(10, 40, westernCP)
			for out := []byte(c.out); len(out) > 0; out = out[min(chunk, len(out)):] {
				s.Write(out[:min(chunk, len(out))])
			}
			if !s.Unknown() && s.PaneState() != PaneDraft {
				t.Errorf("%s (chunks of %d): %v, unknown false, why %q; a terminal shows a draft", c.name, chunk, s.PaneState(), s.Why())
			}
		}
	}

	// A sequence of unknown effect may have changed any of what a redraw keeps: only a full reset (ESC c) brings
	// the screen back. One known to change only cells and the cursor comes back at the redraw.
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, seq := range []string{
		"\x1b[!p", "\x1b[3g", "\x1b[g", "\x1bH", "\x1b[61;1\"p", "\x1b[?5W", "\x1b[5;1;1;1;1T", "\x1b#8", "\x1b[1:2H",
		"\x1b]0;" + strings.Repeat("x", 5000), "\x1b]0;a\x18", "\x1b[1\n2H", "\x1b[" + strings.Repeat("1", 100) + "H",
	} {
		s := drawn(t, 60, 200, idle+seq+"\x1b[2J\x1b[H"+idle)
		if !s.Unknown() {
			t.Errorf("%q, then a full redraw: known", seq)
		}
		if s.Write([]byte("\x1bc" + idle)); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%q, then a full reset: %v unknown %v", seq, s.PaneState(), s.Unknown())
		}
	}
	// A character set is followed as a mode: chosen back (ESC ( B), then redrawn, the screen is known.
	s := drawn(t, 60, 200, idle+"\x1b(0\x1b[2J\x1b[H"+idle)
	if !s.Unknown() {
		t.Error("line drawing, then a full redraw: known")
	}
	if s.Write([]byte("\x1b(B\x1b[2J\x1b[H" + idle)); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("ASCII again, then a full redraw: %v unknown %v", s.PaneState(), s.Unknown())
	}
	// A cursor saved while known and restored is drawn where a terminal puts it, but ESC 7 and ESC 8 are off the
	// allow-list (FXWINSCREENR4): unknown until the next full redraw.
	s = drawn(t, 10, 40, "\x1b[2J\x1b[H"+boxAt(6, "❯")+"\x1b[7;3H\x1b7\x1b[1;1Hspin\x1b8hello")
	if !s.Unknown() || !strings.Contains(s.Text(), "\n❯ hello\n") {
		t.Errorf("a cursor saved while known: %q unknown %v", s.Text(), s.Unknown())
	}
	if s.Write([]byte("\x1b[2J\x1b[H" + boxAt(6, "❯"))); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("a cursor saved while known, then a full redraw: %v unknown %v", s.PaneState(), s.Unknown())
	}
}

// Private modes are numbers, as a terminal reads them (CSI ?01049l is 1049); only the modes known to draw nothing
// are skipped, and any other breaks the screen.
func TestScreenReadsPrivateModesAsNumbersAndBreaksOnAnyItDoesntKnow(t *testing.T) {
	// The main screen holds a draft; the alternate one an idle box; ?01049l goes back to the main one.
	s := drawn(t, 10, 40, "\x1b[2J\x1b[H"+boxAt(6, "❯ hello")+"\x1b[?1049h\x1b[H"+boxAt(6, "❯ ")+"\x1b[?01049l")
	if !s.Unknown() && s.PaneState() != PaneDraft {
		t.Errorf("?01049l: %v, unknown false; a terminal is back on the main screen's draft", s.PaneState())
	}
	idle := ptyOutput(pane(t, "idle-empty"))
	// The alternate page kept from before (?47h) is not the screen's: unknown until it's redrawn.
	if s := drawn(t, 60, 200, "\x1b[?0047h"+idle); !s.Unknown() {
		t.Errorf("?0047h: %v, known", s.PaneState())
	}
	// Modes that change drawing, and modes nobody listed, stay unknown even through a full redraw.
	for _, seq := range []string{
		"\x1b[?07l", "\x1b[?06h", "\x1b[?02027h", "\x1b[?069h", "\x1b[?03h",
		"\x1b[?45h", "\x1b[?1045h", "\x1b[?045h", "\x1b[?12345h", "\x1b[?0h", "\x1b[?80h", "\x1b[?1;45h", "\x1b[?1:2h",
		// Off the allow-list since FXWINSCREENR3: the cursor keys, its blinking, alternate scroll, no number.
		"\x1b[?1h", "\x1b[?012h", "\x1b[?1007h", "\x1b[?h", "\x1b[h",
	} {
		if s := drawn(t, 60, 200, seq+idle); !s.Unknown() {
			t.Errorf("%q: %v, known", seq, s.PaneState())
		}
		if s := drawn(t, 60, 200, seq+"\x1b[2J\x1b[H"+idle); !s.Unknown() {
			t.Errorf("%q, then a full redraw: %v, known", seq, s.PaneState())
		}
	}
	// Reverse wraparound: a backspace at column 1 goes to the end of the row above, the box's line on xterm.
	if s := drawn(t, 10, 40, "\x1b[?45h\x1b[8;1H\bx"+boxAt(6, "❯")); !s.Unknown() {
		t.Errorf("?45h, then a backspace at column 1: %v, known", s.PaneState())
	}
	// Bracketed paste off with a leading zero is off.
	if s := drawn(t, 1, 5, "\x1b[?2004h\x1b[?02004l"); s.Bracketed() {
		t.Error("?02004l left bracketed paste on")
	}
	// The modes on the allow-list are skipped, with or without leading zeros, and autowrap set while it's on. Origin
	// reset while it's off is not: it puts the cursor home (TestScreenBreaksOnOriginModeSetOrResetEvenWhenItWasOff).
	for _, seq := range []string{
		"\x1b[?0025l", "\x1b[?1000;1006h", "\x1b[?01004h", "\x1b[?9h", "\x1b[?2026h\x1b[?2026l", "\x1b[?07h",
		"\x1b[?1002;1003;1005;1015;1016h",
	} {
		if s := drawn(t, 60, 200, seq+idle); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%q: %v unknown %v, why %q", seq, s.PaneState(), s.Unknown(), s.Why())
		}
	}
	// Win32 input (ConPTY's, as it starts) is off the list but draws nothing: unknown until a full redraw.
	s = drawn(t, 60, 200, "\x1b[?09001h"+idle)
	if !s.Unknown() {
		t.Error("?09001h: known")
	}
	if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() {
		t.Errorf("?09001h, then a full redraw: unknown, why %q", s.Why())
	}
	if s := drawn(t, 2, 10, "main\x1b[?01049h\x1b[Halt"); s.Text() != "alt\n" || s.Unknown() {
		t.Errorf("?01049h: %q", s.Text())
	}
}

// DCS and APC strings break the screen: a sixel image (DCS q) and a kitty image shown (APC G) draw over cells and
// move the cursor on a terminal that has them, and Windows 10's conhost prints the body of any (RT126WINSCREENR2 B4).
func TestScreenBreaksOnAStringThatMayDraw(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, seq := range []string{
		"\x1b[3;3H\x1bPq#0;2;0;0;0#0~~~~~~-~~~~~~-~~~~~~\x1b\\",  // sixel
		"\x1bP0;1;0q\"1;1;6;6#0~\x1b\\",                          // sixel with its parameters
		"\x1b[3;3H\x1b_Ga=T,f=100;AAAA\x1b\\",                    // kitty: send and show
		"\x1b_Ga=p,i=1\x1b\\",                                    // kitty: show one sent before
		"\x1b_Gf=100;AAAA\x1b\\",                                 // kitty: no action said
		"\x1b_Ga=q,a=T;AAAA\x1b\\",                               // two actions
		"\x1b_G" + strings.Repeat("i=1,", 20) + "a=q;AAAA\x1b\\", // the action past what the screen looks at
		"\x1bP1|17/ab\x1b\\",                                     // user-defined keys
		"\x1bP1$t1;1;1;@;@;@;0;2;@;BBBB\x1b\\",                   // presentation state restored
		"\x1bPq~~\x1b[H",                                         // a sixel cut off by another sequence
		"\x1b_hello\x1b\\",                                       // an APC nobody knows
		"\x1bP\x1b\\",                                            // an empty DCS
		// The reports that draw nothing on xterm, which FXWINSCREENR2 skipped.
		"\x1bP$qm\x1b\\", "\x1bP$q\"p\x1b\\", "\x1bP+q544e;636f6c73\x1b\\", "\x1b_Ga=q\x1b\\", "\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\",
	} {
		s := drawn(t, 60, 200, idle+seq)
		if !s.Unknown() {
			t.Errorf("%q: known", seq)
		}
		if s.Write([]byte("\x1b[2J\x1b[H" + idle)); !s.Unknown() {
			t.Errorf("%q, then a full redraw: known", seq)
		}
	}
}

// The window's size is the console's own (the launcher follows it); a size the agent asks for (CSI 8;r;c t)
// doesn't change it, but conhost resizes to it and xterm doesn't, so the screen reads unknown until a full reset
// (RT126WINSCREENR2 B3). The screen's own size, both numbers given, is on the allow-list (FXWINSCREENR4); with a
// number left out, until a full redraw. A console made narrower reads again at the next full redraw.
func TestScreenFollowsTheConsolesSizeNotOneTheAgentAsksFor(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	s := drawn(t, 60, 200, idle+"\x1b[8;24;80t\x1b[2J\x1b[H"+idle)
	if len(s.grid) != 60 || len(s.grid[0]) != 200 || !s.Unknown() {
		t.Errorf("a size asked for: %dx%d, %v unknown %v", len(s.grid), len(s.grid[0]), s.PaneState(), s.Unknown())
	}
	if s := drawn(t, 60, 200, idle+"\x1b[8;60;200t"); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("the screen's own size: %v unknown %v, why %q", s.PaneState(), s.Unknown(), s.Why())
	}
	for _, seq := range []string{"\x1b[8;0;200t", "\x1b[8;60t", "\x1b[8t"} { // a number left out
		if s := drawn(t, 60, 200, idle+seq); !s.Unknown() {
			t.Errorf("%q: known", seq)
		}
	}
	for _, seq := range []string{"\x1b[8;60;200t", "\x1b[8;0;200t", "\x1b[8;60t", "\x1b[8t"} { // the screen's own size, or none
		if s := drawn(t, 60, 200, idle+seq+"\x1b[2J\x1b[H"+idle); s.Unknown() {
			t.Errorf("%q, then a full redraw: unknown, why %q", seq, s.Why())
		}
	}
	for _, seq := range []string{"\x1b[8;61;200t", "\x1b[8;60;201t", "\x1b[24t", "\x1b[61t", "\x1b[8;60;200;1t"} {
		if s := drawn(t, 60, 200, idle+seq+"\x1b[2J\x1b[H"+idle); !s.Unknown() {
			t.Errorf("%q, then a full redraw: known", seq)
		}
	}
	s = drawn(t, 60, 200, idle)
	s.Resize(60, 180)
	if !s.Unknown() {
		t.Error("a narrower console is known")
	}
	if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() || s.PaneState() != PaneIdle || len(s.grid[0]) != 180 {
		t.Errorf("a narrower console redrawn: %v unknown %v, %d columns", s.PaneState(), s.Unknown(), len(s.grid[0]))
	}
}
