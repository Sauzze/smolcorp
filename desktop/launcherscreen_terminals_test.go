package desktop

import (
	"strings"
	"testing"
)

// screenOf is the text of a window of the given rows holding lines at the given rows (1-based), as a terminal shows it.
func screenOf(rows int, at map[int]string) string {
	l := make([]string, rows)
	for r, line := range at {
		l[r-1] = line
	}
	return strings.Join(l, "\n")
}

// feedScreen writes pre and then out to a screen of rows by 40, in chunks, growing it by grow rows in between.
func feedScreen(rows, chunk int, pre string, grow int, out string) *textScreen {
	s := newTextScreen(rows, 40, westernCP)
	for _, part := range []string{pre, out} {
		for b := []byte(part); len(b) > 0; b = b[min(chunk, len(b)):] {
			s.Write(b[:min(chunk, len(b))])
		}
		if grow > 0 && part == pre {
			s.Resize(rows+grow, 40)
		}
	}
	return s
}

// Where xterm and conhost (under ConPTY) act differently on a sequence, or where it isn't certain what one does,
// the screen reads unknown (RT126WINSCREENR1 B1–B4). Each stream leaves at least one of them holding a draft,
// worked out by hand: the screen must read unknown, or that draft. Where both act alike, it reads the draft.
func TestScreenReadsUnknownWhereTerminalsDiffer(t *testing.T) {
	box := boxAt(6, "❯")
	byLineFeeds := "\x1b[2J\x1b[H\x1b[5;1H" + boxRule + "\r\n❯\r\n" + boxRule + "\r\n? for shortcuts\x1b[3;3Hhello"
	for _, c := range []struct {
		name, pre string
		grow      int
		out, real string
		known     bool // both terminals act alike: the screen reads the draft, known
	}{
		// B1: ?1049l on the main screen restores the cursor on xterm and conhost alike, but it isn't on the
		// allow-list (FXWINSCREENR3): unknown.
		{name: "S3: ?1049l on the main screen restores the cursor (ESC 7)",
			out: box + "\x1b[7;3H\x1b7\x1b[1;1H\x1b[?1049lhello", real: boxScreen(6, "❯ hello")},
		{name: "S3b: ?1049l on the main screen restores the cursor (CSI s)",
			out: box + "\x1b[7;3H\x1b[s\x1b[1;1H\x1b[?1049lhello", real: boxScreen(6, "❯ hello")},
		// B1: ?1049h on the alternate screen clears it (xterm and conhost); where the cursor goes differs.
		{name: "S4: ?1049h again on the alternate screen clears it",
			out: "\x1b[?1049h" + box + "\x1b[?1049h" + boxAt(1, "❯ hello"), real: boxScreen(1, "❯ hello")},
		// B4, related: a console with one saved cursor for both screens.
		{name: "S1: a cursor saved on the alternate screen, then ?1049l",
			out: box + "\x1b[1;1H\x1b[?1049h\x1b[7;3H\x1b7\x1b[?1049lhello", real: boxScreen(6, "❯ hello")},
		{name: "S2: a cursor restored on the alternate screen, which saved none",
			out: "\x1b[7;3H\x1b[?1049h" + box + "\x1b8hello", real: boxScreen(6, "❯ hello")},
		// B2: the scroll region.
		{name: "R2: an invalid region (5;5) leaves the cursor on xterm",
			out: box + "\x1b[7;3H\x1b[5;5rhello", real: boxScreen(6, "❯ hello")},
		{name: "R2b: an invalid region (8;3) leaves the cursor on xterm",
			out: box + "\x1b[7;3H\x1b[8;3rhello", real: boxScreen(6, "❯ hello")},
		{name: "R3: a region's bottom past the screen, which conhost ignores",
			out: "\x1b[1;5r\x1b[1;99r" + byLineFeeds, real: boxScreen(2, "❯ hello")},
		{name: "R4: a region kept through a resize, as conhost keeps it", pre: "\x1b[1;5r", grow: 2,
			out: byLineFeeds, real: screenOf(12, map[int]string{2: boxRule, 3: "❯ hello", 4: boxRule, 5: "? for shortcuts"})},
		// B3: C1 controls sent as UTF-8, which xterm acts on.
		{name: "S5: U+009B is CSI on xterm",
			out: box + "\x1b[?1049h\u009b?1049l\x1b[2J\x1b[H" + boxAt(6, "❯ hello") + "\x1b[?1049l", real: boxScreen(6, "❯ hello")},
		{name: "S6: U+0088 sets a tab stop on xterm",
			out:  "\x1b[1;36H\u0088\x1b[2J\x1b[H" + box + "\x1b[7;33H\thello",
			real: screenOf(10, map[int]string{6: boxRule, 7: "❯" + strings.Repeat(" ", 34) + "hello", 8: boxRule, 9: "? for shortcuts"})},
		// B4: modes 47, 1047 and 1048, which a console that knows only 1049 ignores.
		{name: "S7: ?47h and ?47l", out: box + "\x1b[?47h\x1b[2J\x1b[H" + boxAt(6, "❯ hello") + "\x1b[?47l", real: boxScreen(6, "❯ hello")},
		{name: "S7b: ?1047h and ?1047l", out: box + "\x1b[?1047h\x1b[2J\x1b[H" + boxAt(6, "❯ hello") + "\x1b[?1047l", real: boxScreen(6, "❯ hello")},
		{name: "S8: ?1048h", out: box + "\x1b[7;3H\x1b7\x1b[1;1H\x1b[?1048h\x1b8hello", real: boxScreen(6, "❯ hello")},
		// The sweep (FXWINSCREENR2): an escape or a C1 control inside a character set's choice starts a new
		// sequence on a terminal; here, ?1049h.
		{name: "an escape where a character set should be",
			out: boxAt(6, "❯ hello") + "\x1b(\x1b[?1049h\x1b(B\x1b[2J\x1b[H" + box + "\x1b[?1049l", real: boxScreen(6, "❯ hello")},
		{name: "U+009B where a character set should be",
			out: boxAt(6, "❯ hello") + "\x1b(\u009b?1049h\x1b(B\x1b[2J\x1b[H" + box + "\x1b[?1049l", real: boxScreen(6, "❯ hello")},
		// CSI s with parameters: xterm saves the cursor (no left and right margins on); a console may not.
		{name: "CSI s with parameters",
			out: box + "\x1b[7;3H\x1b7\x1b[1;1H\x1b[1;5s\x1b8hello", real: boxScreen(6, "❯ hello")},
	} {
		if want := ReadPaneState(c.real); want != PaneDraft {
			t.Fatalf("%s: the real screen reads %v", c.name, want)
		}
		for _, chunk := range []int{1, 4096} {
			s := feedScreen(10, chunk, c.pre, c.grow, c.out)
			switch {
			case c.known && (s.Unknown() || s.PaneState() != PaneDraft):
				t.Errorf("%s (chunks of %d): %v, unknown %v, why %q; both terminals show a draft", c.name, chunk, s.PaneState(), s.Unknown(), s.Why())
			case !s.Unknown() && s.PaneState() != PaneDraft && s.PaneState() != PaneUnknown:
				t.Errorf("%s (chunks of %d): %v, unknown false; a terminal shows a draft", c.name, chunk, s.PaneState())
			}
		}
	}

	// S4 heals at a full redraw; the other breaks of B1 and B4's restore heal at a full redraw too.
	s := drawn(t, 10, 40, "\x1b[?1049h"+box+"\x1b[?1049h"+"\x1b[2J\x1b[H"+boxAt(6, "❯ hello"))
	if s.Unknown() || s.PaneState() != PaneDraft {
		t.Errorf("?1049h again, then a full redraw: %v unknown %v, why %q", s.PaneState(), s.Unknown(), s.Why())
	}
	// B2: an invalid region breaks only the cells (the region is the same everywhere); a bottom past the screen
	// and a resize with a region of a part of the screen are of unknown effect, until a full reset.
	idle := ptyOutput(pane(t, "idle-empty"))
	if s := drawn(t, 60, 200, idle+"\x1b[5;5r\x1b[2J\x1b[H"+idle); s.Unknown() {
		t.Errorf("an invalid region, then a full redraw: unknown, why %q", s.Why())
	}
	for _, seq := range []string{"\x1b[1;99r", "\x1b[1;61r", "\x1b[70;99r"} {
		s := drawn(t, 60, 200, idle+seq+"\x1b[2J\x1b[H"+idle)
		if !s.Unknown() {
			t.Errorf("%q, then a full redraw: known", seq)
		}
		if s.Write([]byte("\x1bc" + idle)); s.Unknown() {
			t.Errorf("%q, then a full reset: unknown", seq)
		}
	}
	s = drawn(t, 60, 200, idle+"\x1b[1;30r")
	s.Resize(62, 200)
	if s.Write([]byte("\x1b[2J\x1b[H" + idle)); !s.Unknown() {
		t.Error("a resize with a region, then a full redraw: known")
	}
	s = drawn(t, 60, 200, idle+"\x1b[1;30r\x1b[r")
	s.Resize(62, 200)
	if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() {
		t.Errorf("a resize with the whole screen's region, then a full redraw: unknown, why %q", s.Why())
	}
	// B3: every C1 control sent as UTF-8 is of unknown effect: unknown through a full redraw.
	for r := rune(0x80); r <= 0x9f; r++ {
		if s := drawn(t, 60, 200, idle+string(r)+"\x1b[2J\x1b[H"+idle); !s.Unknown() {
			t.Errorf("U+%04X, then a full redraw: known", r)
		}
	}
	// B4: 47, 1047 and 1048, set or reset, alone or with other modes, are of unknown effect.
	for _, seq := range []string{"\x1b[?47h", "\x1b[?47l", "\x1b[?1047h", "\x1b[?1047l", "\x1b[?1048h", "\x1b[?1048l", "\x1b[?25;1048h", "\x1b[?0047l"} {
		if s := drawn(t, 60, 200, idle+seq+"\x1b[2J\x1b[H"+idle); !s.Unknown() {
			t.Errorf("%q, then a full redraw: known", seq)
		}
	}
}

// The sweep of every sequence the screen draws (FXWINSCREENR2): where xterm and conhost may differ, or it isn't
// certain what one does, the screen reads unknown; a full redraw heals what touched only the cells and the
// cursor, and a full reset what touched more.
func TestScreenBreaksOnEachSequenceWhoseEffectIsntCertain(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	redraw := "\x1b[2J\x1b[H" + idle
	full := strings.Repeat("a", 200) // fills a row: the next character's place is deferred
	for _, c := range []struct {
		name, out string
		heals     bool // a full redraw brings the screen back
	}{
		// The cursor saved with the modes it saves: line drawing and origin mode come back with it.
		{"line drawing restored with the cursor", "\x1b(0\x1b7\x1b(B\x1b[2J\x1b[H\x1b8", false},
		{"origin mode restored with the cursor", "\x1b[2;9r\x1b[?6h\x1b7\x1b[?6l\x1b[2J\x1b[H\x1b8", false},
		{"a cursor saved after the last column, restored", "\x1b[1;1H" + full + "\x1b7\x1b[5;1H\x1b8", true},
		// A deferred wrap that terminals may keep or end.
		{"an erase after the last column, then a character", "\x1b[1;1H" + full + "\x1b[Kb", true},
		{"a line feed after the last column, then a character", "\x1b[1;1H" + full + "\nb", true},
		{"a scroll after the last column, then a character", "\x1b[1;1H" + full + "\x1b[Sb", true},
		{"a backspace after the last column", "\x1b[1;1H" + full + "\b", true},
		{"a backspace at the first column", "\x1b[5;1H\b", true},
		{"a tab after the last column", "\x1b[1;1H" + full + "\t", true},
		{"a tab over text", "\x1b[20;1Hab\x1b[20;1H\t", true},
		{"a tab past the last tab stop", "\x1b[20;195H\t", true},
		{"a vertical tab", "\x0b", true}, {"a form feed", "\x0c", true}, {"NUL", "\x00", true},
		// The scroll region, with the cursor outside it.
		{"a line feed at the last row, below the region", "\x1b[1;5r\x1b[60;1H\n", true},
		{"a reverse index at the first row, above the region", "\x1b[3;8r\x1b[1;1H\x1bM", true},
		{"lines inserted outside the region", "\x1b[3;8r\x1b[1;1H\x1b[L", true},
		{"lines deleted outside the region", "\x1b[3;8r\x1b[20;1H\x1b[M", true},
		// ANSI modes but insert (4) and new line (20); private CSIs but the reports known to draw nothing.
		{"keyboard locked (CSI 2h)", "\x1b[2h", false}, {"control characters shown (CSI 3h)", "\x1b[3h", false},
		{"local echo (CSI 12l)", "\x1b[12l", false}, {"CSI 12h", "\x1b[12h", false}, {"CSI 34h", "\x1b[34h", false},
		{"CSI 4;2h", "\x1b[4;2h\x1b[4l", false},
		{"CSI =7l", "\x1b[=7l", false}, {"CSI =5h", "\x1b[=5h", false}, {"CSI >5W", "\x1b[>5W", false},
		{"CSI <1m", "\x1b[<1m", false}, {"CSI >1:2m", "\x1b[>1:2m", false}, {"CSI ?5$u", "\x1b[?5$u", false},
		// Reports and settings that draw nothing, off the allow-list since FXWINSCREENR3.
		{"the reports and settings FXWINSCREENR2 skipped", "\x1b[>1;2T\x1b[>0s\x1b[>1p\x1b[>4n\x1b[>0t\x1b[=0c\x1b[<u\x1b[?4m\x1b[?u\x1b[>c\x1b[=1;1u", false},
	} {
		s := drawn(t, 60, 200, idle+c.out+idle)
		if !s.Unknown() {
			t.Errorf("%s: known", c.name)
			continue
		}
		if s.Write([]byte(redraw)); s.Unknown() == c.heals {
			t.Errorf("%s, then a full redraw: unknown %v, why %q", c.name, s.Unknown(), s.Why())
		}
		if s.Write([]byte("\x1bc" + idle)); s.Unknown() {
			t.Errorf("%s, then a full reset: unknown, why %q", c.name, s.Why())
		}
	}
	// What terminals do alike stays known.
	clean := "\x1b[20;1H\x1b[2K\x1b[21;1H\x1b[2K"
	for _, c := range []struct{ name, out string }{
		{"a cursor saved with ASCII, restored after line drawing, then a redraw", "\x1b7\x1b(0\x1b8" + redraw},
		{"an erase after the last column, then the cursor put", "\x1b[20;1H" + full + "\x1b[K\x1b[21;1Hb" + clean},
		{"a new line after the last column", "\x1b[20;1H" + full + "\r\nb" + clean},
		{"a backspace", "\x1b[20;3H\bx" + clean},
		{"a tab over blanks", "\x1b[20;1Ha\tb" + clean},
		{"a line feed inside the region", "\x1b[1;5r\x1b[5;1H\n\x1b[r"},
		{"insert and new line modes, off and redrawn", "\x1b[4h\x1b[20h\x1b[4l\x1b[20l" + redraw},
	} {
		if s := drawn(t, 60, 200, idle+c.out+"\x1b[H"+idle); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%s: %v unknown %v, why %q", c.name, s.PaneState(), s.Unknown(), s.Why())
		}
	}
	// A resized window: xterm and Windows Terminal pull lines back from the scrollback when it grows, conhost
	// reflows wrapped lines, and they differ on which lines go when it shrinks. Unknown until a full redraw; a
	// cursor saved before it is the terminal's, not the screen's.
	for _, size := range [][2]int{{62, 200}, {60, 220}, {58, 200}} {
		s := drawn(t, 60, 200, idle+"\x1b[5;3H\x1b7")
		s.Resize(size[0], size[1])
		if !s.Unknown() {
			t.Errorf("resized to %v: known", size)
		}
		if s.Write([]byte(redraw)); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("resized to %v, then a full redraw: %v unknown %v, why %q", size, s.PaneState(), s.Unknown(), s.Why())
		}
		if s.Write([]byte("\x1b8")); !s.Unknown() {
			t.Errorf("resized to %v, a cursor saved before it restored: known", size)
		}
	}
}
