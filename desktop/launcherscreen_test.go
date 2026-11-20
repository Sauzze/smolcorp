package desktop

import (
	"strings"
	"testing"
	"time"
)

// ptyOutput is a capture as the launcher's pty would send it: CRLF line ends.
func ptyOutput(capture string) string { return strings.ReplaceAll(capture, "\n", "\r\n") }

// The console's code page, an input of the screen (screenCodePage): a Western one, on which Ambiguous characters
// are one cell, and a Japanese one, on which they may be two.
var (
	westernCP = screenCodePage{output: 1252, locale: localeNotCJK}
	cjkCP     = screenCodePage{output: 932, locale: localeCJK}
)

func drawn(t *testing.T, rows, cols int, out string) *textScreen {
	t.Helper()
	s := newTextScreen(rows, cols, westernCP)
	if n, err := s.Write([]byte(out)); err != nil || n != len(out) {
		t.Fatalf("Write: %d, %v", n, err)
	}
	return s
}

func TestScreenGivesTheStatesReadPaneStateGivesOnTheFixtures(t *testing.T) {
	for name, want := range map[string]PaneState{
		"idle-empty":      PaneIdle,
		"after-decline":   PaneIdle,
		"draft":           PaneDraft,
		"busy-1":          PaneBusy,
		"permission-menu": PaneMenu,
		"trust-menu":      PaneMenu,
	} {
		capture := pane(t, name)
		// Whole, a byte at a time (every UTF-8 character cut), and with
		// colour codes and mode switches around the text.
		decorated := "\x1b[?25l\x1b[?2004h\x1b]0;claude\x07\x1b[38;5;174m" +
			strings.ReplaceAll(ptyOutput(capture), "❯", "\x1b[1m❯\x1b[0m") + "\x1b[?25h"
		whole := drawn(t, 60, 200, ptyOutput(capture))
		bytewise := newTextScreen(60, 200, westernCP)
		for _, b := range []byte(decorated) {
			bytewise.Write([]byte{b})
		}
		for how, s := range map[string]*textScreen{"whole": whole, "bytewise": bytewise} {
			if s.Unknown() {
				t.Errorf("%s %s: unknown", name, how)
			}
			if got := s.PaneState(); got != want || got != ReadPaneState(capture) {
				t.Errorf("%s %s: %v, want %v", name, how, got, want)
			}
		}
		// Same text as the capture, line by line.
		if got, want := whole.Text(), capture; paneLinesTrimmed(got) != paneLinesTrimmed(want) {
			t.Errorf("%s: text differs:\n%s", name, got)
		}
	}
}

func paneLinesTrimmed(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		out = append(out, strings.TrimRight(l, " "))
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func TestScreenRedrawsInPlaceWithCursorMovesAndErases(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	// A busy screen, then Claude Code redraws the footer in place: up, to the
	// column, erase the line, write.
	busy := ptyOutput(paneBox("  ⏵⏵ auto mode on · esc to interrupt", "❯"))
	s := drawn(t, 60, 200, busy)
	if got := s.PaneState(); got != PaneBusy {
		t.Fatalf("busy: %v", got)
	}
	s.Write([]byte("\r\x1b[2K" + paneAuto))
	if got := s.PaneState(); got != PaneIdle {
		t.Errorf("footer redrawn: %v", got)
	}
	// Home, erase all, draw again.
	s = drawn(t, 60, 200, busy+"\x1b[H\x1b[2J"+idle)
	if got := s.PaneState(); got != PaneIdle {
		t.Errorf("after a full erase: %v", got)
	}
	// Absolute moves, erase to the end, erase characters.
	s = drawn(t, 5, 20, "abcdefghij\x1b[1;4H\x1b[K\x1b[3;1Hxyz\x1b[3;2H\x1b[1X\x1b[5;3Hq\x1b[2Dr\x1b[3Cs\x1b[3Gt\x1b[2Aa\x1b[1Bb\b\bc")
	if got, want := s.Text(), "abc\n\nx za\n   cb\n rt  s"; got != want {
		t.Errorf("moves:\n%q\nwant\n%q", got, want)
	}
}

func TestScreenScrollsAtTheBottomWrapsAndTabs(t *testing.T) {
	s := drawn(t, 3, 5, "one\r\ntwo\r\nthree\r\nfour")
	if got := s.Text(); got != "two\nthree\nfour" {
		t.Errorf("scroll: %q", got)
	}
	s = drawn(t, 3, 5, "abcdefgh")
	if got := s.Text(); got != "abcde\nfgh\n" {
		t.Errorf("wrap: %q", got)
	}
	// A line of exactly the width doesn't wrap until the next character.
	s = drawn(t, 3, 5, "abcde\r\nx")
	if got := s.Text(); got != "abcde\nx\n" {
		t.Errorf("exact width: %q", got)
	}
	s = drawn(t, 2, 40, "a\tb\t\tc")
	if got := s.Text(); got != "a       b               c\n" {
		t.Errorf("tabs: %q", got)
	}
	s = drawn(t, 2, 10, "\x1b]0;a title\x1b\\\x1b]2;another\x07hi")
	if got := s.Text(); got != "hi\n" || s.Unknown() {
		t.Errorf("titles skipped: %q, unknown %v", got, s.Unknown())
	}
	// Any OSC but a title is off the allow-list (FXWINSCREENR3): of unknown effect.
	if s = drawn(t, 2, 10, "\x1b]8;;http://x\x07hi"); s.Text() != "hi\n" || !s.Unknown() {
		t.Errorf("a link: %q, unknown %v", s.Text(), s.Unknown())
	}
}

func TestScreenResizes(t *testing.T) {
	// Terminals differ on which lines a resized window shows (FXWINSCREENR2): unknown until a full redraw.
	s := drawn(t, 4, 10, "a\r\nb\r\nc\r\nd")
	s.Resize(2, 10)
	if got := s.Text(); got != "c\nd" || !s.Unknown() {
		t.Errorf("fewer rows: %q, unknown %v", got, s.Unknown())
	}
	s.Resize(3, 20)
	s.Write([]byte("e"))
	if got := s.Text(); got != "c\nde\n" {
		t.Errorf("larger: %q", got)
	}
	if !s.Unknown() {
		t.Error("larger window is known")
	}
	// A narrower window may rewrap: unknown until the screen is erased.
	s.Resize(3, 5)
	if !s.Unknown() {
		t.Error("narrower window is known")
	}
	s.Write([]byte("\x1b[2J\x1b[H")) // the cursor too may have moved with the rewrap: put at a known place
	if s.Unknown() {
		t.Error("full erase didn't clear the narrower window")
	}
}

// What the screen can't draw faithfully reads unknown, never idle, even when
// the text looks like an idle window.
func TestScreenNeverReadsIdleWhenItCannotDrawFaithfully(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	if got := drawn(t, 60, 200, idle).PaneState(); got != PaneIdle {
		t.Fatalf("control: %v", got)
	}
	for _, c := range []struct{ name, seq string }{
		// (FXWINSCREEN: the alternate screen 1049, scroll regions, insert lines, next line, index up, save
		// cursor, unknown erase modes, wide characters, emoji and combining marks are drawn now:
		// launcherscreen_claude_test.go.)
		{"alternate screen kept from before (old form)", "\x1b[?47h"},
		{"unknown private mode (no autowrap)", "\x1b[?7l"},
		{"origin mode", "\x1b[?6h"},
		{"insert mode", "\x1b[4h"},
		{"unknown CSI that may move cells (repeat)", "\x1b[3b"},
		{"tab stop set", "\x1bH"},
		{"colon parameter", "\x1b[4:3H"},
		{"unknown control byte", "\x0e"},
		{"bad UTF-8", "\xff"},
		{"CSI longer than the parameter limit", "\x1b[" + strings.Repeat("1", 100) + "H"},
		{"line separator", "\u2028"}, {"Hangul Jamo vowel", "\u1160"},
		{"zero width joiner", "\u200d"}, {"flag letter", "\U0001f1eb"}, {"skin tone", "\U0001f3fd"},
		{"endless OSC", "\x1b]0;" + strings.Repeat("x", 5000)},
	} {
		s := drawn(t, 60, 200, c.seq+idle)
		if !s.Unknown() || s.PaneState() != PaneUnknown {
			t.Errorf("%s: unknown %v, state %v", c.name, s.Unknown(), s.PaneState())
		}
	}
	// An unfinished UTF-8 character at the end is held, not drawn.
	if s := drawn(t, 60, 200, idle+"\xe6\xbc"); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("unfinished UTF-8: unknown %v, state %v", s.Unknown(), s.PaneState())
	}
	// A scroll region set to the whole screen is not a region; leaving the
	// alternate screen is cleared by a full erase; Reset clears everything.
	if s := drawn(t, 60, 200, "\x1b[r"+idle); s.Unknown() {
		t.Error("whole-screen scroll region is unknown")
	}
	// Setting the region homes the cursor, as a terminal does.
	if s := drawn(t, 5, 20, "\x1b[3;7H\x1b[rx"); s.Text() != "x\n\n\n\n" || s.Unknown() {
		t.Errorf("whole-screen scroll region didn't home the cursor: %q", s.Text())
	}
	if s := drawn(t, 60, 200, "\x1b[?1049h\x1b[?1049l\x1b[2J"+idle); s.Unknown() || s.PaneState() != PaneIdle {
		t.Error("alternate screen left and erased stays unknown")
	}
	// The alternate screen is drawn (FXWINSCREEN): an erase on it is known.
	if s := drawn(t, 60, 200, "\x1b[?1049h\x1b[2J"+idle); s.Unknown() || s.PaneState() != PaneIdle {
		t.Error("alternate screen on is unknown after an erase")
	}
	// An erase alone doesn't clear an unknown sequence: the cursor's place isn't known until it is put somewhere.
	s := drawn(t, 60, 200, "\x0e"+idle)
	s.Write([]byte("\x1b[2J"))
	if !s.Unknown() {
		t.Error("an unknown sequence was cleared by an erase")
	}
	s.Reset(60, 200)
	s.Write([]byte(idle))
	if s.Unknown() || s.PaneState() != PaneIdle {
		t.Error("Reset didn't clear unknown")
	}
}

// An OSC that is cut off by another escape sequence ends there; the new
// sequence is run, so what follows is not hidden (idle must not be read off a
// busy window).
func TestScreenAnAbortedOSCDoesNotHideWhatFollows(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	busy := ptyOutput(paneBox("  ⏵⏵ auto mode on · esc to interrupt", "❯"))
	s := drawn(t, 60, 200, idle)
	s.Write([]byte("\x1b]0;t\x1b[H\x1b[2J" + busy))
	// A title cut off is off the allow-list (FXWINSCREENR3): unknown, never idle, until the next full redraw.
	if got := s.PaneState(); got != PaneUnknown || s.Text() == "" {
		t.Errorf("busy after an aborted OSC: %v", got)
	}
	if s.Write([]byte("\x1b[2J\x1b[H" + busy)); s.PaneState() != PaneBusy {
		t.Errorf("busy after an aborted OSC and a full redraw: %v, why %q", s.PaneState(), s.Why())
	}
}

func TestScreenTooBigASizeIsUnknownNotMemory(t *testing.T) {
	s := newTextScreen(100000, 100000, westernCP)
	if !s.Unknown() || s.PaneState() != PaneUnknown {
		t.Error("a huge size is known")
	}
	s = newTextScreen(10, 10, westernCP)
	s.Resize(1<<30, 1<<30)
	if !s.Unknown() {
		t.Error("a huge resize is known")
	}
	if s := newTextScreen(1000, 1000, westernCP); s.Unknown() {
		t.Error("the largest size is unknown")
	}
}

func TestScreenColoursDoNotCancelAPendingWrap(t *testing.T) {
	s := drawn(t, 3, 5, "abcde\x1b[31m\x1b[0mf")
	if got := s.Text(); got != "abcde\nf\n" {
		t.Errorf("wrap after colours: %q", got)
	}
	s = drawn(t, 3, 5, "abcde\x1b[Cf")
	if got := s.Text(); got != "abcdf\n\n" {
		t.Errorf("a cursor move keeps cancelling the wrap: %q", got)
	}
}

func TestScreenClearsInPlace(t *testing.T) {
	s := drawn(t, 60, 200, strings.Repeat("x\r\n", 70))
	clears := []byte("\x1b[H\x1b[2J\x1b[J\x1b[1J\x1b[2K\x1b[K\x1b[1K\n\x1b[H")
	if n := testing.AllocsPerRun(1000, func() { s.Write(clears) }); n != 0 {
		t.Errorf("%v allocations per clear, want none", n)
	}
	if s.Unknown() || strings.TrimSpace(s.Text()) != "" {
		t.Error("the screen wasn't cleared")
	}
}

// A string that a real terminal ends at CAN, SUB or a C1 control (and then
// draws what follows) must not hide a busy footer: it reads unknown.
func TestScreenAnOSCEndedByCancelOrC1DoesNotHideWhatFollows(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	footer := "\r  ⏵⏵ auto mode on · esc to interrupt"
	for _, c := range []struct{ name, end string }{
		{"CAN", "\x18"}, {"SUB", "\x1a"}, {"C1 string terminator", "\u009c"},
		{"first C1", "\u0080"}, {"last C1", "\u009f"},
	} {
		out := "\x1b]0;t" + c.end + footer
		s := drawn(t, 60, 200, idle)
		s.Write([]byte(out))
		if s.PaneState() == PaneIdle || !s.Unknown() {
			t.Errorf("%s: state %v, unknown %v", c.name, s.PaneState(), s.Unknown())
		}
		s = drawn(t, 60, 200, idle)
		for _, b := range []byte(out) { // a byte at a time: the C2 is in another write
			s.Write([]byte{b})
		}
		if s.PaneState() == PaneIdle || !s.Unknown() {
			t.Errorf("%s bytewise: state %v, unknown %v", c.name, s.PaneState(), s.Unknown())
		}
	}
	// C2 alone, with ordinary characters after it, is a normal title.
	s := drawn(t, 60, 200, "\x1b]0;claude · a\u00a0b\x07"+idle)
	if s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("middle dot and no-break space in a title: unknown %v, state %v", s.Unknown(), s.PaneState())
	}
}

// Clears rewrite only rows that aren't blank, so a stream of them at the
// largest size stays fast (it took minutes per MiB when every clear rewrote
// every cell). The bound is generous: the work is milliseconds.
func TestScreenAStreamOfClearsIsBounded(t *testing.T) {
	for _, c := range []struct{ name, unit string }{
		{"erase display", "\x1b[2J"}, {"erase below", "\x1b[J"}, {"erase above", "\x1b[1J"},
		{"one row dirtied, then cleared", "\x1b[Hx\x1b[2J"},
	} {
		s := newTextScreen(1000, 1000, westernCP)
		in := []byte(strings.Repeat(c.unit, 64<<10/len(c.unit)))
		start := time.Now()
		s.Write(in)
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("%s: %d bytes took %v", c.name, len(in), d)
		}
	}
}
