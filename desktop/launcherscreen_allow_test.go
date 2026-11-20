package desktop

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// The third review's streams (RT126WINSCREENR2 B1–B4): each leaves a real terminal holding a draft, worked out by
// hand for the terminal named, where an earlier screen read known idle. On the allow-list (FXWINSCREENR3) each
// must read unknown, or that draft.
func TestScreenReadsUnknownOnTheThirdReviewsStreams(t *testing.T) {
	byLineFeeds := "\x1b[5;1H" + boxRule + "\r\n❯\r\n" + boxRule + "\r\n? for shortcuts"
	for _, c := range []struct{ name, out, real, why string }{
		// B1: conhost keeps autowrap, the new-line mode and the scroll margins per screen buffer; xterm keeps one.
		{"P1: ?7l on the main screen, ?7h on the alternate one (conhost: the main one still doesn't wrap)",
			"\x1b[?7l\x1b[?1049h\x1b[?7h\x1b[?1049l\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b[7;40H b\x1b[8;1H" + boxRule,
			boxScreen(6, "❯"+strings.Repeat(" ", 38)+"b"), "CSI ?1049 with a mode or a scroll region set"},
		{"P2: 20h on the main screen, 20l on the alternate one (conhost: a line feed still returns)",
			"\x1b[20h\x1b[?1049h\x1b[20l\x1b[?1049l\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b[6;36H\n     hello\x1b[7;1H❯\x1b[8;1H" + boxRule,
			boxScreen(6, "❯    hello"), "CSI ?1049 with a mode or a scroll region set"},
		{"M1: a region on the main screen, then the alternate one (Windows 10's conhost: it has none)",
			"\x1b[1;5r\x1b[?1049h" + byLineFeeds + "\x1b[6;3Hhello", boxScreen(5, "❯ hello"), "CSI ?1049 with a mode or a scroll region set"},
		{"M2: a region set on the alternate screen, then the main one (Windows 10's conhost: it has none)",
			"\x1b[?1049h\x1b[1;5r\x1b[?1049l\x1b[2J\x1b[H" + byLineFeeds + "\x1b[6;3Hhello", boxScreen(5, "❯ hello"), "CSI ?1049 with a mode or a scroll region set"},
		// B2: widths that differ: Windows 10's conhost gives each combining mark a cell; Windows Terminal 1.22 and
		// later draws ⚠️ (with its variation selector) two cells wide.
		{"W1: 40 combining marks after an e, then '  hello', on the box's row",
			"\x1b[2J\x1b[H\x1b[6;1He" + strings.Repeat("́", 40) + "  hello" + boxAt(6, "❯"), boxScreen(6, "❯  hello"), "a character of unsure width"},
		{"W2: 20 ⚠️, then '  hello', on the box's row",
			"\x1b[2J\x1b[H\x1b[6;1H" + strings.Repeat("⚠️", 20) + "  hello" + boxAt(6, "❯"), boxScreen(6, "❯ hello"), "a character of unsure width"},
		// B3: conhost resizes its window on CSI 8;rows;cols t; xterm ignores it.
		{"T1: a draft box, CSI 8;5;40t, an idle box at rows 6-9 (conhost: 5 rows, so it lands on row 5)",
			"\x1b[2J\x1b[H" + boxAt(1, "❯ hello") + "\x1b[8;5;40t" + boxAt(6, "❯"),
			strings.Join([]string{boxRule, "❯ hello", boxRule, "? for shortcuts", "? for shortcuts" + strings.Repeat("─", 25)}, "\n"), "CSI 8;5;40t"},
		// B4: Windows 10's conhost has no SOS, PM, DCS or APC: it ignores the escape and prints the string's body.
		{"Q1: PM", boxAt(6, "❯") + "\x1b[7;3H\x1b^hello\x1b\\", boxScreen(6, "❯ hello"), "PM"},
		{"Q2: SOS", boxAt(6, "❯") + "\x1b[7;3H\x1bXhello\x1b\\", boxScreen(6, "❯ hello"), "SOS"},
		{"Q3: DCS $q", boxAt(6, "❯") + "\x1b[7;3H\x1bP$qm\x1b\\", boxScreen(6, "❯ $qm"), "DCS $q"},
		{"Q4: APC G a=q", boxAt(6, "❯") + "\x1b[7;3H\x1b_Ga=q\x1b\\", boxScreen(6, "❯ Ga=q"), "APC G"},
	} {
		if want := ReadPaneState(c.real); want != PaneDraft {
			t.Fatalf("%s: the real screen reads %v", c.name, want)
		}
		for _, chunk := range []int{1, 4096} {
			s := feedScreen(10, chunk, "", 0, c.out)
			if !s.Unknown() && s.PaneState() != PaneDraft {
				t.Errorf("%s (chunks of %d): %v, unknown false; a terminal shows a draft", c.name, chunk, s.PaneState())
			}
			if s.Why() != c.why { // for the reason named, not for something else in the stream
				t.Errorf("%s (chunks of %d): why %q, want %q", c.name, chunk, s.Why(), c.why)
			}
		}
	}
	// The control: the box alone reads idle.
	if s := drawn(t, 10, 40, "\x1b[2J\x1b[H"+boxAt(6, "❯")); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("the box alone: %v unknown %v, why %q", s.PaneState(), s.Unknown(), s.Why())
	}
}

// Each entry on the allow-list leaves the screen known: a stream of nothing else, on a console with a Western code
// page (box lines and all are drawn in ASCII here, so the text isn't read, only Unknown).
func TestEachEntryOnTheAllowListLeavesTheScreenKnown(t *testing.T) {
	for _, c := range []struct{ name, out string }{
		{"printable ASCII, CR, LF", " !~09azAZ\r\nnext"},
		{"a backspace inside a row", "ab\bc"},
		{"a tab over blanks to a stop inside the window", "a\tb"},
		{"CUP, HVP, CUU, CUD, CUF, CUB, CHA, VPA inside the screen", "\x1b[5;5Hx\x1b[2;2fy\x1b[1Az\x1b[2Bw\x1b[3Cv\x1b[2Du\x1b[9Gt\x1b[3ds\x1b[0fr\x1b[H\x1b[;5H"},
		{"EL 0 1 2, ED 0 1 2 3", "abc\x1b[2;2H\x1b[K\x1b[0K\x1b[1K\x1b[2K\x1b[J\x1b[0J\x1b[1J\x1b[3J\x1b[2J\x1b[H"},
		{"SGR, colon forms too", "\x1b[1;2;38;5;174mx\x1b[38:2::1:2:3m\x1b[0m\x1b[m"},
		{"the modes known to draw nothing", "\x1b[?25l\x1b[?25h\x1b[?2004h\x1b[?2004l\x1b[?2026h\x1b[?2026l\x1b[?1004h\x1b[?1004l" +
			"\x1b[?9h\x1b[?1000;1002;1003;1005;1006;1015;1016h\x1b[?1000;1002;1003;1005;1006;1015;1016l\x1b[?9l"},
		{"the alternate screen, there and back", "main\x1b[?1049h\x1b[Halt\x1b[?1049lx"},
		{"titles, ended by BEL or ST", "\x1b]0;✳ Claude Code\x07\x1b]2;a title\x1b\\x"},
		{"the full redraw and the full reset", "x\x1b[2J\x1b[Hy\x1bcz"},
		{"CJK", "漢字かなカナ한국어、。（）"},
		{"Claude Code's glyphs, Neutral", "❯\u00a0⎿⏵╌▐▛▜▝✢✶✻"},
		// FXWINSCREENR4.
		{"Claude Code's glyphs, Ambiguous", "─·█▀←↓…—◐✽"},
		{"Claude Code's glyphs, emoji drawn as text", "⏺⏸✳"},
		{"French", "éàçêèëïîôûùœÉÀÇÊ’“”–€"},
		{"other Ambiguous characters and emoji drawn as text", "│★αж™®©⚠‼✔❤➡"},
		{"ECH", "abcdef\x1b[1;2H\x1b[2X\x1b[X\x1b[0X\x1b[99X"},
		{"ICH, DCH", "abcdef\x1b[1;2H\x1b[2@\x1b[@\x1b[0P\x1b[2P\x1b[99P"},
		{"SU, SD", "a\r\nb\r\nc\x1b[S\x1b[2T\x1b[0S\x1b[10S"},
		{"DECSTBM, valid, a line feed at its bottom, and back to the whole screen", "\x1b[2;9r\x1b[2;1Hx\x1b[9;1H\n\x1b[r\x1b[1;10r\x1b[;r"},
		{"SU and SD in a region that is the whole screen", "\x1b[1;10r\x1b[2;1Hx\x1b[S\x1b[8T\x1b[r"},
		{"the window's size said at the screen's own", "\x1b[8;10;40t"},
	} {
		s := newTextScreen(10, 40, westernCP)
		if s.Write([]byte(c.out)); s.Unknown() {
			t.Errorf("%s: unknown, why %q", c.name, s.Why())
		}
	}
}

// A character off the allow-list breaks the screen until a full redraw, whatever the code page: widths differ
// between xterm and conhost's builds (RT126WINSCREENR2 B2). That includes an emoji drawn as one (VS16, a ZWJ
// sequence, a flag, a skin tone, Emoji_Presentation) and a Neutral symbol not on the list (‹ and the figure space
// U+2007 stand where « and U+202F stood before FXFRENCHSPACES put those on it).
func TestScreenBreaksOnACharacterOffTheList(t *testing.T) {
	for _, r := range []string{
		"e\u0301", "漢\u0301", "⚠️", "✳️", "🙂", "✅", "⭐", "☕", "a\u200db", "\U0001f1eb", "\U0001f3fd", "\u2028", "\u1160", "\u1100",
		"\U00020000", "\u303f", "\u3099", "\u302a", "\u00ad", "\ufe0f", "\ufe0e", "✓", "‹", "\u2007", "\ue000", "\U0001f170",
	} {
		for _, page := range []screenCodePage{westernCP, cjkCP} {
			s := newTextScreen(60, 200, page)
			want := "a character of unsure width"
			if first, _ := utf8.DecodeRuneInString(r); page.cjk() && screenOneCellOffCJK(first) { // ⚠ before its VS16
				want = page.ambiguousWhy()
			}
			if s.Write([]byte("\x1b[2;3Hab" + r)); !s.Unknown() || s.Why() != want {
				t.Errorf("%+q (code page %d): unknown %v, why %q, want %q", r, page.output, s.Unknown(), s.Why(), want)
			}
			if s.Write([]byte("\x1b[2J\x1b[Hab")); s.Unknown() {
				t.Errorf("%+q (code page %d), then a full redraw: unknown, why %q", r, page.output, s.Why())
			}
		}
	}
}

// An Ambiguous character (FXWINSCREENR4: the East Asian Ambiguous class, the accented Latin letters and the emoji
// drawn as text) is one cell on a console whose code page isn't a CJK one, and breaks the screen until a full
// redraw on one that is, or whose code page couldn't be read.
func TestScreenReadsAnAmbiguousCharacterByTheCodePage(t *testing.T) {
	chars := []rune("─·█▀←↓…—◐✽⏺⏸✳éàçêèïôùœÉÀÇ’“”–€│★αж™®©⚠‼✔❤➡")
	known := []screenCodePage{
		{output: 437, locale: localeUnknown}, {output: 850, locale: localeCJK}, westernCP, {output: 1251, locale: localeNotCJK},
		{output: 65001, locale: localeNotCJK}, {output: 54936, locale: localeNotCJK},
	}
	breaking := []screenCodePage{
		cjkCP, {output: 936, locale: localeNotCJK}, {output: 949}, {output: 950, locale: localeCJK},
		{output: 65001, locale: localeCJK}, {output: 65001, locale: localeUnknown}, {output: 54936, locale: localeCJK},
		{output: 54936}, {output: 51932, locale: localeNotCJK}, {},
	}
	for _, r := range chars {
		for _, page := range known {
			s := newTextScreen(1, 10, page)
			if s.Write([]byte("a" + string(r) + "b")); s.Unknown() || s.Text() != "a"+string(r)+"b" || s.col != 3 {
				t.Errorf("%q on code page %d (locale %d): %q, column %d, unknown %v (why %q)", r, page.output, page.locale, s.Text(), s.col, s.Unknown(), s.Why())
			}
		}
		for _, page := range breaking {
			s := newTextScreen(10, 40, page)
			why := fmt.Sprintf("an ambiguous-width character on code page %d", page.output)
			if page.output == 0 {
				why = "an ambiguous-width character, the console's code page unread"
			}
			if s.Write([]byte("\x1b[2;3H" + string(r))); !s.Unknown() || s.Why() != why {
				t.Errorf("%q on code page %d (locale %d): unknown %v, why %q", r, page.output, page.locale, s.Unknown(), s.Why())
			}
			if s.Write([]byte("\x1b[2J\x1b[H")); s.Unknown() {
				t.Errorf("%q on code page %d, then a full redraw: unknown, why %q", r, page.output, s.Why())
			}
		}
	}
	// On a CJK code page, CJK and Claude Code's Neutral glyphs stay on the list.
	s := newTextScreen(10, 40, cjkCP)
	if s.Write([]byte("漢字かな❯\u00a0⎿⏵╌▐▛▜▝✢✶✻")); s.Unknown() {
		t.Errorf("CJK and Claude Code's Neutral glyphs on code page 932: unknown, why %q", s.Why())
	}
}

// A change of code page, between one where Ambiguous characters are one cell and one where they may be two, breaks
// the screen until a full redraw (what was drawn may be drawn otherwise now); between two alike, nothing changes.
func TestScreenBreaksWhenTheCodePageChangesAmbiguousWidths(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, c := range []struct {
		from, to screenCodePage
		breaks   bool
	}{
		{westernCP, cjkCP, true}, {cjkCP, westernCP, true}, {westernCP, screenCodePage{}, true},
		{westernCP, screenCodePage{output: 65001, locale: localeUnknown}, true},
		{westernCP, screenCodePage{output: 437}, false}, {cjkCP, screenCodePage{output: 936}, false},
		{westernCP, screenCodePage{output: 65001, locale: localeNotCJK}, false},
	} {
		s := newTextScreen(60, 200, c.from)
		s.Write([]byte("\x1b[2J\x1b[H" + idle))
		s.SetCodePage(c.to)
		if s.Unknown() != (c.breaks || c.from.cjk()) {
			t.Errorf("%+v to %+v: unknown %v, why %q", c.from, c.to, s.Unknown(), s.Why())
		}
		if c.breaks && s.Why() != "the console's code page changed" && !c.from.cjk() {
			t.Errorf("%+v to %+v: why %q", c.from, c.to, s.Why())
		}
		if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() != c.to.cjk() {
			t.Errorf("%+v to %+v, then a full redraw: unknown %v, why %q", c.from, c.to, s.Unknown(), s.Why())
		}
	}
	// The launcher's screen takes the code page it's given, and a new one as the launcher reads it again.
	l := newLauncherScreen(60, 200, true, westernCP)
	l.Write([]byte("\x1b[2J\x1b[H" + idle))
	if state, unknown := l.Look(); unknown || state != PaneIdle {
		t.Errorf("a launcher's screen on code page 1252: %v unknown %v", state, unknown)
	}
	l.SetCodePage(cjkCP)
	if state, unknown := l.Look(); !unknown || state != PaneUnknown {
		t.Errorf("a launcher's screen moved to code page 932: %v unknown %v", state, unknown)
	}
}

// The sequences the ruling (FXWINSCREENR4) put on the allow-list, where they stop being on it, and what still breaks.
func TestScreenTakesTheSequencesConPTYSendsAndBreaksOnTheRest(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	box := "\x1b[2J\x1b[H" + idle
	for _, c := range []struct {
		name, out string
		heals     int // 0: on the list; 1: until a full redraw; 2: until a full reset
	}{
		{"ECH", "\x1b[3;3H\x1b[5X", 0},
		{"SU and SD, no region", "\x1b[3;3H\x1b[2S\x1b[2T", 0},
		{"DECSTBM, valid", "\x1b[2;60r\x1b[1;2r\x1b[r", 0},
		{"ICH and DCH", "\x1b[3;3H\x1b[2@\x1b[2P", 0},
		{"the window's size said, the screen's own", "\x1b[8;60;200t", 0},
		{"ESC 7, ESC 8", "\x1b[3;3H\x1b7\x1b8", 0},
		{"ESC 7 alone", "\x1b7", 0},
		{"ESC 8 with a scroll region set", "\x1b[2;59r\x1b[3;3H\x1b7\x1b8\x1b[r", 1},
		{"ESC 8 with another style saved", "\x1b[3;3H\x1b[2m\x1b7\x1b[22m\x1b8\x1b[22m", 1},
		{"CSI s, CSI u", "\x1b[3;3H\x1b[s\x1b[u", 1},
		{"SU and SD inside a region of part of the screen (RT126WINSCREENR4 N2)", "\x1b[1;30r\x1b[30;1H\x1b[3S\x1b[29T\x1b[r", 1},
		{"SU with the cursor outside the region", "\x1b[1;30r\x1b[40;1H\x1b[S\x1b[r", 1},
		{"SD past the region's rows", "\x1b[1;30r\x1b[2;1H\x1b[31T\x1b[r", 1},
		{"DECSTBM, top not above bottom", "\x1b[5;5r", 1},
		{"DECSTBM, top below bottom", "\x1b[9;3r", 1},
		{"ECH with two numbers", "\x1b[3;3H\x1b[1;1X", 1},
		{"ICH with two numbers", "\x1b[3;3H\x1b[1;1@", 1},
		{"DCH with two numbers", "\x1b[3;3H\x1b[1;1P", 1},
		{"the window's size said with a number left out", "\x1b[8;60t", 1},
		{"a window report asked", "\x1b[18t", 1},
		{"DECSTBM, past the screen", "\x1b[1;61r", 2},
		{"DECSTBM, three numbers", "\x1b[1;5;6r", 2},
		{"SD with more numbers (mouse tracking)", "\x1b[1;2;3;4;5T", 2},
		{"SU with two numbers", "\x1b[1;2S", 2},
		{"another window size asked", "\x1b[8;24;80t", 2},
		{"DECSLPP", "\x1b[24t", 2},
		{"a DCS", "\x1bP$qm\x1b\\", 2},
		{"an APC", "\x1b_Ga=q\x1b\\", 2},
		{"an SOS", "\x1bXa\x1b\\", 2},
		{"a PM", "\x1b^a\x1b\\", 2},
		{"an OSC off the list", "\x1b]52;c;aGk=\x1b\\", 2},
		{"a region kept through a switch of screens", "\x1b[1;30r\x1b[?1049h\x1b[?1049l\x1b[r", 2},
	} {
		s := drawn(t, 60, 200, box+c.out)
		if s.Unknown() != (c.heals > 0) {
			t.Errorf("%s: unknown %v, why %q", c.name, s.Unknown(), s.Why())
		}
		if s.Write([]byte(box)); s.Unknown() != (c.heals > 1) || (c.heals < 2 && s.PaneState() != PaneIdle) {
			t.Errorf("%s, then a full redraw: %v, unknown %v, why %q", c.name, s.PaneState(), s.Unknown(), s.Why())
		}
		if s.Write([]byte("\x1bc" + idle)); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%s, then a full reset: %v, unknown %v, why %q", c.name, s.PaneState(), s.Unknown(), s.Why())
		}
	}
}

// Claude Code's screens on a console with a CJK code page, or one that couldn't be read: every step holds an
// Ambiguous glyph (─ in every box), so each reads unknown, never what it shows; so do the six fixtures. With a
// Western code page the same stream reads each state (TestScreenReadsClaudeCodesStreamAndTellsItsBoxApart).
func TestScreenReadsClaudeCodesScreensUnknownOnACJKCodePage(t *testing.T) {
	for _, page := range []screenCodePage{cjkCP, {output: 65001, locale: localeCJK}, {}} {
		for _, reply := range []claudeReply{claudeEnglish, claudeFrench} {
			s := newTextScreen(60, 200, page)
			for _, step := range claudeStepsReplying(t, reply) {
				s.Write([]byte(step.out))
				if !s.Unknown() || s.PaneState() != PaneUnknown {
					t.Errorf("code page %d, %s: %v, unknown %v", page.output, step.name, s.PaneState(), s.Unknown())
				}
				if !strings.ContainsFunc(step.text, func(r rune) bool { return screenOneCellOffCJK(r) && !screenGlyphs[r] }) {
					t.Errorf("code page %d, %s: holds no Ambiguous character, yet reads unknown (why %q)", page.output, step.name, s.Why())
				}
			}
		}
		for _, name := range []string{"idle-empty", "after-decline", "draft", "busy-1", "permission-menu", "trust-menu"} {
			s := newTextScreen(60, 200, page)
			if s.Write([]byte(ptyOutput(pane(t, name)))); !s.Unknown() || !strings.HasPrefix(s.Why(), "an ambiguous-width character") {
				t.Errorf("code page %d, %s: unknown %v, why %q", page.output, name, s.Unknown(), s.Why())
			}
		}
	}
}

// A switch between the screens while a followed mode or a scroll region is set is of unknown effect (conhost keeps
// them per screen buffer, RT126WINSCREENR2 B1): unknown through a full redraw, until a full reset.
func TestScreenBreaksOnASwitchWhileAModeOrARegionIsSet(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, set := range []string{"\x1b[?7l", "\x1b[20h", "\x1b[4h", "\x1b[?6h", "\x1b(0", "\x1b)0", "\x1b[1;5r"} {
		for _, sw := range []string{"\x1b[?1049h", "\x1b[?1049h\x1b[?1049l"} {
			s := drawn(t, 60, 200, idle+set+sw+"\x1b[2J\x1b[H"+idle)
			if s.Why() != "CSI ?1049 with a mode or a scroll region set" {
				t.Errorf("%q then %q, then a full redraw: why %q", set, sw, s.Why())
			}
			if s.Write([]byte("\x1bc" + idle)); s.Unknown() || s.PaneState() != PaneIdle {
				t.Errorf("%q then %q, then a full reset: %v unknown %v", set, sw, s.PaneState(), s.Unknown())
			}
		}
	}
	// Set and back to the default before the switch: the switch is on the list.
	if s := drawn(t, 60, 200, "\x1b[?7l\x1b[?7h\x1b[20h\x1b[20l\x1b[1;5r\x1b[r\x1b[?1049h\x1b[2J\x1b[H"+idle); s.Unknown() {
		t.Errorf("modes back to the default, then a switch: unknown, why %q", s.Why())
	}
}

// Every stream the three reviews built (RT126WINSCREEN, R1, R2; R2's own are above), replayed: each reads unknown,
// or what the real screen they worked out by hand shows.
func TestScreenReadsEveryEarlierReviewsStreamUnknownOrRight(t *testing.T) {
	at := func(r, c int) string { return fmt.Sprintf("\x1b[%d;%dH", r, c) }
	with := func(r int, line string, extra map[int]string) string {
		l := strings.Split(boxScreen(r, line), "\n")
		for k, v := range extra {
			l[k] = v
		}
		return strings.Join(l, "\n")
	}
	byLF := func(line string) string {
		return at(5, 1) + boxRule + "\r\n" + line + "\r\n" + boxRule + "\r\n? for shortcuts"
	}
	xAtTheEnd := "❯" + strings.Repeat(" ", 38) + "x"
	for _, c := range []struct{ name, out, real string }{
		// The first review's.
		{"?01049l", "\x1b[2J\x1b[H" + boxAt(6, "❯ hello") + "\x1b[?1049h\x1b[H" + boxAt(6, "❯ ") + "\x1b[?01049l", boxScreen(6, "❯ hello")},
		{"a cursor saved while broken", "\x1b[1;1Hx\x1b[242b\x1b7\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b8hello", boxScreen(6, "❯  hello")},
		{"DECSTR", "\x1b[1;5r\x1b[!p\x1b[2J\x1b[H" + byLF("❯") + "\x1b[6;3Hhello", boxScreen(5, "❯ hello")},
		{"CSI 3g", "\x1b[3g\x1b[2J\x1b[H\x1b[6;1H\t  hello" + boxAt(6, "❯"), boxScreen(6, "❯hello")},
		{"ESC H", "\x1b[1;1H\x1bH\x1b[3g\x1b[2J\x1b[H\x1b[6;1H\t  hello" + boxAt(6, "❯"), boxScreen(6, "❯hello")},
		{"?45h", "\x1b[?45h\x1b[8;1H\bx" + boxAt(6, "❯"), boxScreen(6, xAtTheEnd)},
		{"?1045h", "\x1b[?1045h\x1b[8;1H\bx" + boxAt(6, "❯"), boxScreen(6, xAtTheEnd)},
		{"?45h, then 2J H", "\x1b[?45h\x1b[2J\x1b[H\x1b[8;1H\bx" + boxAt(6, "❯"), boxScreen(6, xAtTheEnd)},
		// R1's: the cursor saved on both screens, the region, C1 controls, the old alternate screens.
		{"S0", boxAt(6, "❯") + at(7, 3) + "\x1b[?1049h\xff\x1b7\x1b[2J\x1b[H\x1b[?1049lhello", boxScreen(6, "❯ hello")},
		{"S1", "\x1b[2J\x1b[H" + boxAt(6, "❯") + at(1, 1) + "\x1b[?1049h" + at(7, 3) + "\x1b7\x1b[?1049lhello", boxScreen(6, "❯ hello")},
		{"S2", at(7, 3) + "\x1b[?1049h" + boxAt(6, "❯") + "\x1b8hello", boxScreen(6, "❯ hello")},
		{"S3", boxAt(6, "❯") + at(7, 3) + "\x1b7" + at(1, 1) + "\x1b[?1049lhello", boxScreen(6, "❯ hello")},
		{"S3b", boxAt(6, "❯") + at(7, 3) + "\x1b[s" + at(1, 1) + "\x1b[?1049lhello", boxScreen(6, "❯ hello")},
		{"S4", "\x1b[?1049h" + boxAt(6, "❯") + "\x1b[?1049h" + boxAt(1, "❯ hello"), boxScreen(1, "❯ hello")},
		{"S5", "\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b[?1049h\u009b?1049l\x1b[2J\x1b[H" + boxAt(6, "❯ hello") + "\x1b[?1049l", boxScreen(6, "❯ hello")},
		{"S6", at(1, 36) + "\u0088\x1b[2J\x1b[H" + at(7, 33) + "\t hello" + boxAt(6, "❯"), boxScreen(6, "❯"+strings.Repeat(" ", 35)+"hell")},
		{"S7", boxAt(6, "❯") + "\x1b[?47h\x1b[2J\x1b[H" + boxAt(6, "❯ hello") + "\x1b[?47l", boxScreen(6, "❯ hello")},
		{"S7b", boxAt(6, "❯") + "\x1b[?1047h\x1b[2J\x1b[H" + boxAt(6, "❯ hello") + "\x1b[?1047l", boxScreen(6, "❯ hello")},
		{"S8", boxAt(6, "❯") + at(7, 3) + "\x1b7" + at(1, 1) + "\x1b[?1048h\x1b8hello", boxScreen(6, "❯ hello")},
		{"R1", "\x1b[1;9r\x1b[2J\x1b[H" + boxAt(6, "❯") + at(9, 1) + "\n" + at(6, 3) + "hello", boxScreen(5, "❯ hello")},
		{"R1b", "x\x1b[3b\x1b[1;9r\x1b[2J\x1b[H" + boxAt(6, "❯") + at(9, 1) + "\n" + at(6, 3) + "hello", boxScreen(5, "❯ hello")},
		{"R2", boxAt(6, "❯") + at(7, 3) + "\x1b[5;5rhello", boxScreen(6, "❯ hello")},
		{"R2b", boxAt(6, "❯") + at(7, 3) + "\x1b[8;3rhello", boxScreen(6, "❯ hello")},
		{"R3", "\x1b[1;5r\x1b[1;99r\x1b[2J\x1b[H" + byLF("❯") + at(3, 3) + "hello", boxScreen(2, "❯ hello")},
		// R1's full reset against the full redraw.
		{"DECSTR, then 2J H", "\x1b[!p\x1b[2J\x1b[H" + boxAt(6, "❯"), boxScreen(6, "❯")},
		{"DECSTR, then ESC c", "\x1b[!p\x1bc" + boxAt(6, "❯"), boxScreen(6, "❯")},
		{"a region, then ESC c", "\x1b[1;5r\x1bc" + byLF("❯") + "\x1b[6;3Hhello", boxScreen(5, "❯ hello")},
		{"an unknown CSI on the alternate screen", boxAt(6, "❯") + "\x1b[?1049h\x1b[3g\x1b[?1049l", boxScreen(6, "❯")},
		{"ESC c on the alternate screen", boxAt(6, "❯ hello") + "\x1b[?1049h\x1bc" + boxAt(6, "❯"), boxScreen(6, "❯")},
		{"a cursor saved, ESC c, ESC 8", boxAt(6, "❯") + at(7, 3) + "\x1b7\x1bc" + boxAt(6, "❯") + "\x1b8hello", with(6, "❯", map[int]string{0: "hello"})},
		{"G1 line drawing, SO, ESC ) B, 2J H", "\x1b)0\x0e\x1b)B\x1b[2J\x1b[H" + boxAt(6, "❯"), boxScreen(6, "❯")},
	} {
		for _, chunk := range []int{1, 4096} {
			s := feedScreen(10, chunk, "", 0, c.out)
			if want := ReadPaneState(c.real); !s.Unknown() && s.PaneState() != want {
				t.Errorf("%s (chunks of %d): %v, unknown false; the real screen reads %v", c.name, chunk, s.PaneState(), want)
			}
		}
	}
	// R4: a region kept through a resize.
	s := newTextScreen(10, 40, westernCP)
	s.Write([]byte("\x1b[1;5r"))
	s.Resize(12, 40)
	if s.Write([]byte("\x1b[2J\x1b[H" + byLF("❯") + at(3, 3) + "hello")); !s.Unknown() {
		t.Errorf("R4: %v, known", s.PaneState())
	}
	// The first review's images, and R1's allow-list streams: all off the list now, but the cursor shown.
	for _, seq := range []string{
		"\x1bPq#0;2;0;0;0#0~~~~~~-~~~~~~-~~~~~~\x1b\\", "\x1b_Ga=T,f=100;AAAA\x1b\\",
		"\x1b[?25;45h", "\x1b[?1;1000;1006h", "\x1b[?;1049h", "\x1b[?100001049l", "\x1b[?66585l",
		"\x1b[3h", "\x1b[2h", "\x1b[12l", "\x1b[34h", "\x1b[=7l", "\x1b[>1;2T", "\x1b[=5h",
		"\x1bP$qm\x1b\\", "\x1bP1$qm\x1b\\", "\x1bP+p544e\x1b\\", "\x1bP\n$qm\x1b\\", "\x1bP$q" + strings.Repeat("x", 100) + "\x1b\\",
		"\x1bPq#0~\x1b[H", "\x1bP$qm\x18", "\x1b_Ga=q\x1b\\", "\x1b_Gi=1,a=q;AAAA\x1b\\", "\x1b_Ga=q,a=T\x1b\\", "\x1b_Ga =T\x1b\\",
		"\x1b_G" + strings.Repeat("i=1,", 20) + "a=q\x1b\\", "\x1b_Gi=1\x1b\\", "\x1b_Ga=q" + strings.Repeat(",i=1", 20) + "\x1b\\",
		"\x1b]1337;File=inline=1;size=4:AAAA\x07",
	} {
		if s := drawn(t, 10, 40, "\x1b[3;3H"+seq); !s.Unknown() {
			t.Errorf("%q: known", seq)
		}
	}
	if s := drawn(t, 10, 40, "\x1b[3;3H\x1b[?0025l"); s.Unknown() {
		t.Errorf("?0025l: unknown, why %q", s.Why())
	}
}

// Origin mode set or reset (CSI ?6h, ?6l) puts the cursor home on xterm and conhost, whether or not it was on
// (RT126WINSCREENR4 B1): it breaks the page every time, until a full redraw, and the cursor goes home as both put
// it. Each stream leaves both terminals holding a draft (home, down 6 and right 2 is 7;3, in the box).
func TestScreenBreaksOnOriginModeSetOrResetEvenWhenItWasOff(t *testing.T) {
	stream := func(mode string) string {
		return "\x1b[2J\x1b[H" + boxAt(6, "❯") + "\x1b[3;1H" + mode + "\x1b[6B\x1b[2Chello"
	}
	for _, c := range []struct{ name, out string }{
		{"O1: ?6l with origin off", stream("\x1b[?6l")},
		{"O2: ?25;6l, which reads as hiding the cursor", stream("\x1b[?25;6l")},
		{"O3: ?6l on the alternate screen", "\x1b[?1049h" + stream("\x1b[?6l")},
		{"?06l, a leading zero", stream("\x1b[?06l")},
		{"?6h, then ?6l", stream("\x1b[?6h\x1b[?6l")},
	} {
		for _, chunk := range []int{1, 4096} {
			s := feedScreen(10, chunk, "", 0, c.out)
			if !s.Unknown() || s.Text() != boxScreen(6, "❯ hello") {
				t.Errorf("%s (chunks of %d): %v, unknown %v, the screen is\n%s", c.name, chunk, s.PaneState(), s.Unknown(), s.Text())
			}
			// Origin is off again, so Claude Code's next full redraw brings the page back.
			if s.Write([]byte("\x1b[2J\x1b[H" + boxAt(6, "❯"))); s.Unknown() || s.PaneState() != PaneIdle {
				t.Errorf("%s (chunks of %d), then a full redraw: %v, unknown %v, why %q", c.name, chunk, s.PaneState(), s.Unknown(), s.Why())
			}
		}
	}
	// The control (O4): with no ?6l, hello lands on the footer row on both terminals, and the box reads idle.
	if s := feedScreen(10, 4096, "", 0, stream("")); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("O4: %v, unknown %v, why %q", s.PaneState(), s.Unknown(), s.Why())
	}
	// Origin set homes the cursor at the region's top, and stays unknown through a full redraw while it's on.
	s := drawn(t, 10, 40, "\x1b[3;9r\x1b[5;5H\x1b[?6hx")
	if !s.Unknown() || s.Text() != "\n\nx\n\n\n\n\n\n\n" {
		t.Errorf("?6h in a region: unknown %v, the screen is %q", s.Unknown(), s.Text())
	}
	if s.Write([]byte("\x1b[2J\x1b[H" + boxAt(6, "❯"))); !s.Unknown() {
		t.Errorf("?6h, then a full redraw: %v, known", s.PaneState())
	}
}

// SU and SD while a scroll region of part of the screen is set break the page until a full redraw: neither ConPTY
// nor Claude Code is known to scroll inside one, and a console whose SU ignores the margins would scroll the whole
// window (RT126WINSCREENR4 N2). With no region set, or one that is the whole screen, they stay on the list.
func TestScreenBreaksOnAScrollInsideARegionOfPartOfTheScreen(t *testing.T) {
	// R7': "  hello" at 8;1, a region of rows 1–5, SU, the region back to the whole screen, the box at 6 by cursor
	// moves. A console that scrolls the whole window shows "❯ hello"; xterm and a current conhost show "❯".
	r7 := "\x1b[2J\x1b[H\x1b[8;1H  hello\x1b[1;5r\x1b[S\x1b[r" + boxAt(6, "❯")
	for _, c := range []struct {
		name, out string
		known     bool
	}{
		{"R7': SU in a region of rows 1–5", r7, false},
		{"SD in a region of rows 1–5", strings.Replace(r7, "\x1b[S", "\x1b[T", 1), false},
		{"SU in a region of rows 2–10", "\x1b[2J\x1b[H\x1b[2;10r\x1b[3;1H\x1b[2S\x1b[r" + boxAt(6, "❯"), false},
		{"SU and SD, no region", "\x1b[2J\x1b[H\x1b[3;1H\x1b[S\x1b[2T" + boxAt(6, "❯"), true},
		{"SU and SD in a region that is the whole screen", "\x1b[2J\x1b[H\x1b[1;10r\x1b[3;1H\x1b[S\x1b[2T\x1b[r" + boxAt(6, "❯"), true},
		{"a line feed at a region's bottom (not SU)", "\x1b[2J\x1b[H\x1b[1;5r\x1b[5;1H\n\x1b[r" + boxAt(6, "❯"), true},
	} {
		s := drawn(t, 10, 40, c.out)
		if s.Unknown() == c.known {
			t.Errorf("%s: %v, unknown %v, why %q", c.name, s.PaneState(), s.Unknown(), s.Why())
		}
		if s.Write([]byte("\x1b[2J\x1b[H" + boxAt(6, "❯"))); s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%s, then a full redraw: %v, unknown %v, why %q", c.name, s.PaneState(), s.Unknown(), s.Why())
		}
	}
}
