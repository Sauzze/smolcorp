package desktop

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// Claude Code's terminal UI as a Windows hire's launcher reads it: what Claude Code writes, as ConPTY passes it
// on. Nothing can run on Windows here, so the stream is built from the sequences the two are known to send, not
// recorded. What it assumes:
//   - ConPTY, at start: win32-input mode (CSI ?9001h), focus reports (CSI ?1004h), a cursor position report
//     asked (CSI 6n) and the window's size said (CSI 8;rows;cols t). When it repaints, each line is put at its
//     row (CSI row;1H), long runs of blanks are erased (CSI n X) and stepped over (CSI n C), and the rest of the
//     line is erased (CSI K). Lines scroll in above the box by the whole window scrolling (a line feed at its last
//     row, CSI n S), and the rows that moved are repainted; no scroll region is set (RT126WINSCREENR4 N2: as
//     recalled, ConPTY's repaint sends no CSI r, and SU or SD inside one breaks the screen).
//   - Claude Code, at start: bracketed paste (CSI ?2004h), focus reports, the kitty keyboard (CSI >1u), modifier
//     keys (CSI >4;2m), the cursor hidden (CSI ?25l), the terminal asked what it is (CSI c, CSI >0q) and whether
//     it has synchronized output (CSI ?2026$p), and its title (OSC 0). Its clear is CSI 2J CSI 3J CSI H (CSI 2J
//     CSI 0f on Windows). Each frame is drawn inside synchronized output (CSI ?2026h … ?2026l), the last frame
//     erased line by line going up (CSI 2K, CSI 1A) and the cursor put back at the first column (CSI G).
//   - A guess, not known to be how Ink draws (extra coverage; it proves nothing about Claude Code): a draft mended
//     with deleted and inserted characters (CSI P, CSI @). The spinner is redrawn in place at its row (CSI row;1H),
//     not between a saved and a restored cursor (ESC 7, ESC 8), which are off the allow-list.
//   - A reply holds CJK (two cells, on the allow-list) and Claude Code's own glyphs. The Ambiguous ones (─ · … ←)
//     and the emoji drawn as text (⏺ ✳) are one cell on a console whose code page isn't a CJK one (FXWINSCREENR4):
//     these tests give the screen a Western code page, and launcherscreen_allow_test.go a CJK one too. A reply in
//     French (é à ç ê) reads alike. A reply with an emoji drawn as one, or a combining mark, breaks the screen
//     until Claude Code's next full redraw (TestScreenBreaksOnACharacterOffTheList); widths differ there
//     (RT126WINSCREENR2 W1 and W2 read idle over a draft).
//
// Only the Windows release candidate's checks prove the stream: the tester sends the app's "screen can't be read
// since …" line if mail to a hire never goes.

const (
	conptyStart = "\x1b[?9001h\x1b[?1004h\x1b[6n\x1b[8;60;200t"
	claudeStart = "\x1b[?2004h\x1b[?1004h\x1b[>1u\x1b[>4;2m\x1b[?25l\x1b[c\x1b[>0q\x1b[?2026$p\x1b]0;✳ Claude Code\x07"
)

// inkFrame is a frame drawn as Claude Code draws it: the last frame's lines erased going up, then the new one.
func inkFrame(last, capture string) string {
	var b strings.Builder
	b.WriteString("\x1b[?2026h")
	if last != "" {
		b.WriteString(strings.Repeat("\x1b[2K\x1b[1A", strings.Count(last, "\n")) + "\x1b[2K\x1b[G")
	}
	b.WriteString(ptyOutput(capture) + "\x1b[?2026l")
	return b.String()
}

var conptyBlanks = regexp.MustCompile(` {8,}`)

// conptyPaint is a capture as ConPTY repaints it: each line at its row, its long runs of blanks erased and
// stepped over, the rest of each line and the rows below erased, on a screen of 60 rows.
func conptyPaint(capture string) string {
	var b strings.Builder
	lines := strings.Split(strings.TrimSuffix(capture, "\n"), "\n")
	for i, line := range lines {
		line = conptyBlanks.ReplaceAllStringFunc(line, func(run string) string {
			return fmt.Sprintf("\x1b[%dX\x1b[%dC", len(run), len(run))
		})
		fmt.Fprintf(&b, "\x1b[%d;1H%s\x1b[K", i+1, line)
	}
	if len(lines) < 60 {
		fmt.Fprintf(&b, "\x1b[%d;1H\x1b[J", len(lines)+1)
	}
	return b.String()
}

// A step of Claude Code's stream: what it sends, the state it then reads, and the screen's text ("" isn't checked).
type claudeStep struct {
	name, out string
	want      PaneState
	text      string
}

// The busy and done screens (60 rows): a long reply scrolled in above the box, the spinner, the box.
const (
	claudeReply1 = "⏺ 完了しました"
	claudeReply4 = "  ⎿  $ go test ./desktop (ok)"
	claudeFooter = "  ⏵⏵ auto mode on (shift+tab to cycle)"
)

// claudeReply holds a reply's second and third lines: in English, and in French.
type claudeReply struct{ line2, line3 string }

var (
	claudeEnglish = claudeReply{"  All tests pass, one file changed", "  one warning · one note · 漢字と한국어"}
	claudeFrench  = claudeReply{"  Les tests passent déjà, ça marche : à vérifier", "  un avertissement · une note · fenêtre et tâche"}
)

func claudeScreen(reply claudeReply, spinner, footer string) string {
	rows := make([]string, 60)
	// The first reply line has "(12s)" put at column 40: the 14 cells before it hold 8 characters.
	rows[49], rows[50], rows[51], rows[52] = claudeReply1+strings.Repeat(" ", 25)+"(12s)", reply.line2, reply.line3, claudeReply4
	rows[54], rows[56], rows[57], rows[58], rows[59] = spinner, paneRule, "❯", paneRule, footer
	return strings.Join(rows, "\n")
}

// claudeSteps is Claude Code's start in a folder not trusted yet, the trust dialog, the start screen and its
// box, a draft, a long reply (in English) with its spinner, and the box idle again.
func claudeSteps(t *testing.T) []claudeStep { return claudeStepsReplying(t, claudeEnglish) }

// claudeStepsReplying is claudeSteps with the reply given.
func claudeStepsReplying(t *testing.T, r claudeReply) []claudeStep {
	trust, idle, draft := pane(t, "trust-menu"), pane(t, "idle-empty"), pane(t, "draft")
	lines := strings.Split(draft, "\n")
	row := 0
	for i, l := range lines { // the fixture has a no-break space after ❯
		if strings.HasPrefix(l, "❯\u00a0half typed note") {
			row = i + 1
		}
	}
	if row == 0 {
		t.Fatal("the draft fixture has no draft line")
	}
	slip := strings.Replace(draft, "❯\u00a0half typed note", "❯\u00a0half xxnote", 1)
	busyText := claudeScreen(r, "✳ Thinking… (2s · ↓ 1.2k tokens)", claudeFooter+" · esc to interrupt")
	reply := "\x1b[?2004h" + conptyPaint(strings.Join([]string{"❯ half typed note", strings.Repeat("\n", 54), paneRule, "❯", paneRule, claudeFooter + " · esc to interrupt"}, "\n")) +
		// The whole window scrolls up four rows (two line feeds at its last row, then CSI 2S), the draft's line
		// going off the top, and the rows that moved are repainted: the reply, then the box.
		"\x1b[60;1H\r\n\r\n\x1b[2S" +
		"\x1b[50;1H" + claudeReply1 + "\x1b[K\x1b[50;40H(12s)" + "\x1b[51;1H" + r.line2 + "\x1b[K" +
		"\x1b[52;1H" + r.line3 + "\x1b[K\x1b[53;1H" + claudeReply4 + "\x1b[K" +
		"\x1b[54;1H\x1b[K\x1b[55;1H\x1b[K\x1b[56;1H\x1b[K\x1b[57;1H" + paneRule + "\x1b[58;1H❯\x1b[K\x1b[59;1H" + paneRule +
		"\x1b[60;1H" + claudeFooter + " · esc to interrupt\x1b[K"
	for _, frame := range []string{"· Thinking…", "✢ Thinking… (1s)", "✳ Thinking… (2s · ↓ 1.2k tokens)"} {
		reply += "\x1b[55;1H\x1b[2K" + frame
	}
	return []claudeStep{
		{"the trust dialog", conptyStart + claudeStart + "\x1b[2J\x1b[0f" + inkFrame("", trust), PaneMenu, trust},
		{"the start screen and its box", inkFrame(trust, "") + "\x1b[2J\x1b[3J\x1b[H" + inkFrame("", idle) + "\x1b[?25h", PaneIdle, idle},
		{"a draft, its slip mended", conptyPaint(slip) + fmt.Sprintf("\x1b[%d;8H\x1b[2P\x1b[6@typed ", row), PaneDraft, draft},
		{"a long reply with CJK, and the spinner", reply, PaneBusy, busyText},
		{"done: the spinner gone, the box idle", "\x1b[55;1H\x1b[2K\x1b[60;1H\x1b[2K" + claudeFooter + "\x1b]0;✳ Claude Code\x07", PaneIdle,
			claudeScreen(r, "", claudeFooter)},
	}
}

// claudeStream is Claude Code's stream from its start to the end of the step named.
func claudeStream(t *testing.T, upTo string) string {
	var out string
	for _, step := range claudeSteps(t) {
		out += step.out
		if step.name == upTo {
			return out
		}
	}
	t.Fatalf("no step %q", upTo)
	return ""
}

// Claude Code's screens on a console with a Western code page (FXWINSCREENR4): each step reads known, in the state it
// shows, with a reply in English and with one in French (é à ç ê).
func TestScreenReadsClaudeCodesStreamAndTellsItsBoxApart(t *testing.T) {
	for _, reply := range []claudeReply{claudeEnglish, claudeFrench} {
		for _, chunk := range []int{1, 7, 4096} {
			s := newTextScreen(60, 200, westernCP)
			for _, step := range claudeStepsReplying(t, reply) {
				for out := []byte(step.out); len(out) > 0; out = out[min(chunk, len(out)):] {
					s.Write(out[:min(chunk, len(out))])
				}
				if s.Unknown() || s.PaneState() != step.want {
					t.Errorf("%q, chunks of %d, %s: %v, unknown %v (why %q), want %v", reply.line2, chunk, step.name, s.PaneState(), s.Unknown(), s.Why(), step.want)
				}
				if got := ReadPaneState(step.text); got != step.want {
					t.Errorf("%s: the text alone reads %v", step.name, got)
				}
				if got := paneLinesTrimmed(s.Text()); got != paneLinesTrimmed(step.text) {
					t.Errorf("%q, chunks of %d, %s: the screen is\n%s", reply.line2, chunk, step.name, got)
				}
			}
			// The window made bigger, as the console's size follower and ConPTY say it: unknown, as terminals differ
			// on the lines a bigger window shows (FXWINSCREENR2), until Claude Code's clear redraws it.
			s.Resize(62, 220)
			s.Write([]byte("\x1b[8;62;220t"))
			if !s.Unknown() || len(s.grid) != 62 || len(s.grid[0]) != 220 {
				t.Errorf("chunks of %d, bigger: %v, unknown %v, %dx%d", chunk, s.PaneState(), s.Unknown(), len(s.grid), len(s.grid[0]))
			}
			s.Write([]byte("\x1b[2J\x1b[3J\x1b[H" + inkFrame("", pane(t, "idle-empty"))))
			if s.Unknown() || s.PaneState() != PaneIdle {
				t.Errorf("chunks of %d, bigger and redrawn: %v, unknown %v, why %q", chunk, s.PaneState(), s.Unknown(), s.Why())
			}
		}
	}
}

// Each sequence Claude Code and ConPTY send is drawn as a terminal draws it, and leaves the screen known.
func TestScreenDrawsEachSequenceClaudeCodeAndConPTYSend(t *testing.T) {
	for _, c := range []struct {
		name       string
		rows, cols int
		out, want  string
	}{
		{"insert characters", 1, 10, "abcdef\x1b[1;3H\x1b[2@XY", "abXYcdef"},
		{"delete characters", 1, 10, "abcdef\x1b[1;2H\x1b[2P", "adef"},
		{"erase characters", 1, 10, "abcdef\x1b[1;2H\x1b[2X", "a  def"},
		{"scroll up", 3, 5, "a\r\nb\r\nc\x1b[1S", "b\nc\n"},
		{"scroll down", 3, 5, "a\r\nb\r\nc\x1b[1T", "\na\nb"},
		{"a scroll region set homes the cursor", 3, 5, "\x1b[3;3H\x1b[1;2rx", "x\n\n"},
		{"the window's size said at the screen's own", 1, 5, "\x1b[8;1;5tab", "ab"},
		{"a line feed at a scroll region's bottom", 4, 5, "a\r\nb\r\nc\r\nd\x1b[2;3r\x1b[3;1H\nx", "a\nc\nx\nd"},
		{"SGR mouse modes", 1, 5, "\x1b[?1000h\x1b[?1002h\x1b[?1003h\x1b[?1006h\x1b[?1015h\x1b[?1016hab\x1b[?1006l", "ab"},
		{"the alternate screen", 2, 10, "main\x1b[?1049h\x1b[Halt", "alt\n"},
		{"the alternate screen left", 2, 10, "main\x1b[?1049h\x1b[Halt\x1b[?1049lx", "mainx\n"},
		{"the alternate screen entered at the cursor", 2, 10, "main\x1b[?1049halt", "    alt\n"},
		{"bracketed paste", 1, 5, "\x1b[?2004hab\x1b[?2004l", "ab"},
		{"cursor shown and hidden", 1, 5, "\x1b[?25lab\x1b[?25h", "ab"},
		{"synchronized output", 1, 5, "\x1b[?2026hab\x1b[?2026l", "ab"},
		{"wide characters take two cells", 1, 10, "漢字\x1b[1;6Hx", "漢字 x"},
		{"a full reset", 2, 5, "ab\x0e\x1bcxy", "xy\n"},
	} {
		s := drawn(t, c.rows, c.cols, c.out)
		if got := s.Text(); got != c.want || s.Unknown() {
			t.Errorf("%s: %q, unknown %v, want %q", c.name, got, s.Unknown(), c.want)
		}
	}
	// Off the allow-list (FXWINSCREENR3), though Claude Code or ConPTY may send it: drawn as a terminal draws it,
	// but the screen reads unknown until a full redraw.
	for _, c := range []struct {
		name       string
		rows, cols int
		out, want  string
	}{
		{"save and restore the cursor (ESC 7, ESC 8)", 3, 10, "ab\x1b7\x1b[3;5Hxy\x1b8c", "abc\n\n    xy"},
		{"restore with nothing saved goes home", 2, 10, "ab\x1b8c", "cb\n"},
		{"save and restore the cursor (CSI s, CSI u)", 3, 10, "ab\x1b[s\x1b[3;5Hxy\x1b[uc", "abc\n\n    xy"},
		{"insert lines", 4, 5, "a\r\nb\r\nc\r\nd\x1b[2;3H\x1b[1Lx", "a\nx\nb\nc"},
		{"delete lines", 4, 5, "a\r\nb\r\nc\r\nd\x1b[2;3H\x1b[2Mx", "a\nx\n\n"},
		{"insert lines inside a scroll region", 4, 5, "a\r\nb\r\nc\r\nd\x1b[1;3r\x1b[2;1H\x1b[L", "a\n\nb\nd"},
		{"scroll up and down inside a scroll region (RT126WINSCREENR4 N2)", 4, 5, "a\r\nb\r\nc\r\nd\x1b[2;3r\x1b[2;1H\x1b[S\x1b[2;1Hx\x1b[T", "a\n\nx\nd"},
		{"scroll down with the cursor outside the scroll region", 4, 5, "a\r\nb\r\nc\r\nd\x1b[2;3r\x1b[4;1H\x1b[T", "a\n\nb\nd"},
		{"scroll up past the scroll region's rows", 4, 5, "a\r\nb\r\nc\r\nd\x1b[2;3r\x1b[2;1H\x1b[3S", "a\n\n\nd"},
		{"insert characters through half a wide character", 1, 10, "漢字\x1b[1;2H\x1b[1@", "   字"},
		{"reverse index at the top", 2, 5, "a\r\nb\x1b[H\x1bMx", "x\na"},
		{"index and next line", 3, 5, "ab\x1bDc\x1bEd", "ab\n  c\nd"},
		{"next and previous line, row and column absolute", 4, 9, "\x1b[3dx\x1b[1Fy\x1b[2Ez\x1b[5`w", "\ny\nx\nz   w"},
		{"cursor position reports asked (DSR)", 1, 5, "\x1b[6n\x1b[5nab", "ab"},
		{"a mode's state asked (DECRQM)", 1, 5, "\x1b[?2026$pab", "ab"},
		{"window operations", 1, 5, "\x1b[22;0t\x1b[23;0tab", "ab"},
		{"the window's size said with no size", 1, 5, "\x1b[8tab", "ab"},
		{"a wide character at the last column goes to the next line", 2, 5, "abcd漢x", "abcd\n漢x"},
		{"half a wide character overwritten blanks the other half", 1, 10, "漢字\x1b[1;2Hx", " x字"},
		{"a wide character erased by half goes whole", 1, 10, "漢字\x1b[1;4H\x1b[1X", "漢"},
		{"erase modes nobody knows", 1, 5, "ab\x1b[7J\x1b[5K", "ab"},
		{"a cursor move past the edge", 2, 5, "\x1b[9;9Hx", "\n    x"},
	} {
		s := drawn(t, c.rows, c.cols, c.out)
		if got := s.Text(); got != c.want || !s.Unknown() {
			t.Errorf("%s: %q, unknown %v, want %q", c.name, got, s.Unknown(), c.want)
		}
		if s.Write([]byte("\x1b[2J\x1b[H")); s.Unknown() {
			t.Errorf("%s, then a full redraw: unknown, why %q", c.name, s.Why())
		}
	}
}

// A sequence off the allow-list that changes no cell (FXWINSCREENR3): the reports and settings ConPTY and Claude
// Code send as they start break the screen until a full redraw (their clear); any other report, setting or string
// until a full reset, as of unknown effect (Windows 10's conhost prints a DCS's, SOS's, PM's or APC's body).
func TestScreenSkipsAnUnknownSequenceThatDrawsNothing(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, c := range []struct {
		seqs  []string
		heals bool // a full redraw brings the screen back
	}{
		{[]string{"\x1b[c", "\x1b[0c", "\x1b[>0q", "\x1b[6n", "\x1b[5n", "\x1b[?2026$p", "\x1b[18t", "\x1b[8;60t", "\x1b[>4;2m",
			"\x1b[>1u", "\x1b[?9001h", "\x1b[0 q"}, true},
		{[]string{"\x1b[>c", "\x1b[=c", "\x1b[?996n", "\x1b[=1;1u", "\x1b[>1p", "\x1bP+q544e\x1b\\", "\x1bP$q\x07m\x1b\\",
			"\x1b_Gi=1,a=q\x1b\\", "\x1b^pm\x1b\\", "\x1bXsos\x1b\\", "\x1b]8;;http://x\x07", "\x1b]1;icon\x07"}, false},
	} {
		for _, seq := range c.seqs {
			s := drawn(t, 60, 200, seq+idle)
			if !s.Unknown() {
				t.Errorf("%q: known", seq)
			}
			if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() == c.heals {
				t.Errorf("%q, then a full redraw: unknown %v, why %q", seq, s.Unknown(), s.Why())
			}
			if s.Write([]byte("\x1bc" + idle)); s.Unknown() || s.PaneState() != PaneIdle {
				t.Errorf("%q, then a full reset: unknown %v, state %v", seq, s.Unknown(), s.PaneState())
			}
		}
	}
	// A DCS ends only at its string terminator: a BEL inside it doesn't end it, so what follows it is drawn.
	if s := drawn(t, 2, 20, "\x1bP$q\x07hidden\x1b\\shown"); s.Text() != "shown\n" || !s.Unknown() {
		t.Errorf("a DCS with a BEL: %q, unknown %v", s.Text(), s.Unknown())
	}
}

// A screen that broke is read again once it's redrawn whole: erased, and the cursor put at a known place.
func TestScreenComesBackAtTheNextFullRedraw(t *testing.T) {
	idle := ptyOutput(pane(t, "idle-empty"))
	for _, c := range []struct{ name, broke, redraw string }{
		{"an unknown control byte, Claude Code's clear", "\x0e", "\x1b[2J\x1b[3J\x1b[H"},
		{"an unknown CSI, the Windows clear", "\x1b[3b", "\x1b[2J\x1b[0f"},
		{"bad UTF-8", "\xff", "\x1b[2J\x1b[H"},
		{"a zero width joiner", "a‍", "\x1b[H\x1b[2J\x1b[H"},
		{"line drawing", "\x1b(0", "\x1b(B\x1b[2J\x1b[H"}, // ASCII chosen again first (FXWINSCREENR1)
	} {
		s := drawn(t, 60, 200, idle+c.broke)
		if !s.Unknown() {
			t.Errorf("%s: known", c.name)
		}
		s.Write([]byte("\x1b[2J" + idle))
		if !s.Unknown() {
			t.Errorf("%s: known after an erase, though drawn before the cursor was put anywhere", c.name)
		}
		s.Write([]byte(c.redraw + idle))
		if s.Unknown() || s.PaneState() != PaneIdle {
			t.Errorf("%s: after a full redraw, %v unknown %v", c.name, s.PaneState(), s.Unknown())
		}
	}
	// A narrower window, as a terminal may rewrap it.
	s := drawn(t, 60, 200, idle)
	s.Resize(60, 180)
	if !s.Unknown() {
		t.Error("a narrower window is known")
	}
	s.Write([]byte("\x1b[2J\x1b[H" + idle))
	if s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("a narrower window redrawn: %v unknown %v", s.PaneState(), s.Unknown())
	}
	// The alternate screen entered fresh is known; leaving it gives back the main page as it was.
	s = drawn(t, 60, 200, idle+"\x1b[3b")
	s.Write([]byte("\x1b[?1049h\x1b[H" + idle))
	if s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("the alternate screen entered after a break: %v unknown %v", s.PaneState(), s.Unknown())
	}
	if s.Write([]byte("\x1b[?1049l")); !s.Unknown() {
		t.Error("the main page that broke was known again on leaving the alternate screen")
	}
	s = drawn(t, 60, 200, idle+"\x1b[?1049h\x1b[3b\x1b[?1049l")
	if s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("a break on the alternate screen stayed on the main one: %v unknown %v", s.PaneState(), s.Unknown())
	}
	// A mode the screen doesn't draw keeps it unknown through a redraw, and once it's off, until the next one.
	s = drawn(t, 60, 200, "\x1b[?7l\x1b[2J\x1b[H"+idle)
	if !s.Unknown() {
		t.Error("no autowrap, redrawn, is known")
	}
	if s.Write([]byte("\x1b[?7h")); !s.Unknown() {
		t.Error("what was drawn without autowrap is known")
	}
	if s.Write([]byte("\x1b[2J\x1b[H" + idle)); s.Unknown() || s.PaneState() != PaneIdle {
		t.Errorf("autowrap back on and redrawn: %v unknown %v", s.PaneState(), s.Unknown())
	}
	// A console of no known size is never known, whatever is drawn.
	l := newLauncherScreen(24, 80, false, westernCP)
	l.Write([]byte("\x1b[2J\x1b[H\x1bc" + idle))
	if state, unknown := l.Look(); !unknown || state != PaneUnknown {
		t.Errorf("a console of no known size, redrawn: %v unknown %v", state, unknown)
	}
}
