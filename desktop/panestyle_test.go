package desktop

import (
	"errors"
	"testing"
)

// T126GHOSTDRAFT1: Claude Code's prompt suggestion is drawn faint (ESC[2m … ESC[0m) in an idle box. Read with
// its escapes, a box holding only that is idle; any typed text in it is still a draft.

// faint is text as tmux -e writes a faint run of it.
func faint(s string) string { return "\x1b[2m" + s + "\x1b[0m" }

func TestAFaintSuggestionIsIdleAndAnyTypedTextIsStillADraft(t *testing.T) {
	for _, c := range []struct {
		name    string
		capture string
		want    PaneState
	}{
		{"suggestion only", paneBox(paneAuto, "❯ "+faint("run the tests")), PaneIdle},
		{"suggestion after a no-break space", paneBox(paneAuto, "❯ "+faint("run the tests")), PaneIdle},
		{"a suggestion wrapped on two lines", paneBox(paneAuto, "❯ \x1b[2mrun the tests and then", "  fix what fails\x1b[0m"), PaneIdle},
		{"typed text", paneBox(paneAuto, "❯ half typed"), PaneDraft},
		{"typed text with a suggestion after it", paneBox(paneAuto, "❯ half typed"+faint(" and run the tests")), PaneDraft},
		{"a suggestion with typed text after it", paneBox(paneAuto, "❯ "+faint("run the tests")+" typed"), PaneDraft},
		{"a suggestion over a typed second line", paneBox(paneAuto, "❯ "+faint("run the tests"), "  typed"), PaneDraft},
		{"pasted text is not faint", paneBox(paneAuto, "❯ [Pasted text #1 +3 lines]"), PaneDraft},
		// The other ways a terminal says faint, and ends it.
		{"faint with a colour, 2;37", paneBox(paneAuto, "❯ \x1b[2;37mrun the tests\x1b[0m"), PaneIdle},
		{"a colour then faint, 38;5;244;2", paneBox(paneAuto, "❯ \x1b[38;5;244;2mrun the tests\x1b[39;22m"), PaneIdle},
		{"faint with a colon colour", paneBox(paneAuto, "❯ \x1b[38:2::1:2:3;2mrun the tests\x1b[m"), PaneIdle},
		{"ended by 22", paneBox(paneAuto, "❯ \x1b[2mrun\x1b[22m the tests"), PaneDraft},
		{"ended by ESC[m", paneBox(paneAuto, "❯ \x1b[2mrun\x1b[m the tests"), PaneDraft},
		{"ended by 0 in a longer sequence", paneBox(paneAuto, "❯ \x1b[2mrun\x1b[0;37m the tests"), PaneDraft},
		{"bold and faint ended together by 22", paneBox(paneAuto, "❯ \x1b[1;2mrun\x1b[22m the tests"), PaneDraft},
		{"faint carried over from the line above", "old output\n\x1b[2m" + paneRule + "\n❯ run the tests\n\x1b[0m" + paneRule + "\n" + paneAuto, PaneIdle},
		// Grey that isn't faint, and a colour's own 2, count as typed.
		{"the 2 of a 38;2 colour is not faint", paneBox(paneAuto, "❯ \x1b[38;2;2;2;2mrun the tests\x1b[39m"), PaneDraft},
		{"the 2 of a 38;5 colour is not faint", paneBox(paneAuto, "❯ \x1b[38;5;2mrun the tests\x1b[39m"), PaneDraft},
		{"the 2 of a colon colour is not faint", paneBox(paneAuto, "❯ \x1b[38:2::2:2:2mrun the tests\x1b[39m"), PaneDraft},
		{"bright black grey", paneBox(paneAuto, "❯ \x1b[90mrun the tests\x1b[39m"), PaneDraft},
		{"typed text in 256-colour grey (P5)", paneBox(paneAuto, "❯ \x1b[38;5;244mmy real draft\x1b[0m"), PaneDraft},
		{"typed text hidden, SGR 8 (P6)", paneBox(paneAuto, "❯ \x1b[8mmy real draft\x1b[0m"), PaneDraft},
		// What else a suggestion may carry (P7, P8), and a typed second line under one (P13).
		{"faint and italic (P7)", paneBox(paneAuto, "❯ \x1b[2;3mrun the tests\x1b[0m"), PaneIdle},
		{"faint, underlined, an underline colour (P8)", paneBox(paneAuto, "❯ \x1b[2m\x1b[4m\x1b[58;2;1;2;3mrun\x1b[0m"), PaneIdle},
		{"a typed line under a suggestion (P13)", paneBox(paneAuto, "❯ \x1b[2mrun\x1b[0m", "  more"), PaneDraft},
		{"a paste chip drawn plain (P9)", paneBox(paneAuto, "❯ [Pasted text #1 +20 lines]"), PaneDraft},
		// A cursor that isn't faint is never let off (RT126GHOSTDRAFT1 B2): one typed character under the cursor with
		// a faint tail after it can't be told from a cursor drawn over a suggestion, so both block.
		{"one typed character under the cursor and a faint tail (P3)", paneBox(paneAuto, "❯ \x1b[7mr\x1b[0;2mun the tests\x1b[0m"), PaneDraft},
		{"a cursor that isn't faint on a suggestion", paneBox(paneAuto, "❯ \x1b[7mr\x1b[27;2mun the tests\x1b[0m"), PaneDraft},
		{"typed text, a cursor on a space, a faint hint (P11)", paneBox(paneAuto, "❯ /review\x1b[7m \x1b[0;2m[pr-number]\x1b[0m"), PaneDraft},
		{"a faint cursor on a suggestion (P12)", paneBox(paneAuto, "❯ \x1b[2;7mr\x1b[27mun\x1b[0m"), PaneIdle},
		{"the cursor on typed text", paneBox(paneAuto, "❯ \x1b[7mh\x1b[27malf typed"), PaneDraft},
		{"the cursor on one typed character", paneBox(paneAuto, "❯ \x1b[7mx\x1b[0m"), PaneDraft},
		{"reverse video after a suggestion", paneBox(paneAuto, "❯ "+faint("run")+"\x1b[7mx\x1b[0m"), PaneDraft},
		{"reverse video on a second line", paneBox(paneAuto, "❯ "+faint("run"), "  \x1b[7mx\x1b[0m"), PaneDraft},
		// A link around it is still read.
		{"a suggestion inside a link", paneBox(paneAuto, "❯ \x1b]8;;https://example.invalid/\x1b\\"+faint("run the tests")+"\x1b]8;;\a"), PaneIdle},
		// The rest of the reading still comes first.
		{"busy beats a suggestion", paneBox("  ⏵⏵ auto mode on · esc to interrupt", "❯ "+faint("run the tests")), PaneBusy},
		{"a suggestion with no footer is cut off", paneBox("", "❯ "+faint("run the tests")), PaneUnknown},
		{"faint text with no selector is no box", paneBox(paneAuto, faint("run the tests")), PaneUnknown},
		{"an empty box", paneBox(paneAuto, "❯"), PaneIdle},
		{"nothing at all", "", PaneUnknown},
		// The limit of this signal, which VICTOR checks once on a real capture: typed text the agent itself draws
		// faint can't be told from a suggestion (the assumption is that typed text is never faint).
		{"typed text the agent drew faint (P4, the known limit)", paneBox(paneAuto, "❯ \x1b[2mmy real draft\x1b[0m"), PaneIdle},
		{"a paste chip if drawn faint (P10, the known limit)", paneBox(paneAuto, "❯ \x1b[2m[Pasted text #1 +20 lines]\x1b[0m"), PaneIdle},
	} {
		if got := ReadStyledPaneState(c.capture); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Fail safe: escapes that can't all be read leave every character typed, as before the fix.
func TestASuggestionInACaptureThatCantBeReadIsStillADraft(t *testing.T) {
	suggestion := "❯ " + faint("run the tests")
	for _, c := range []struct{ name, capture string }{
		{"a cursor move", "\x1b[5A" + paneBox(paneAuto, suggestion)},
		{"a character set", "\x1b(B" + paneBox(paneAuto, suggestion)},
		{"a private sequence", "\x1b[?25l" + paneBox(paneAuto, suggestion)},
		{"an SGR with a letter in it", paneBox(paneAuto, "❯ \x1b[2;xmrun the tests\x1b[0m")},
		{"a colour cut short", paneBox(paneAuto, "❯ \x1b[2;38;5mrun the tests\x1b[0m")},
		{"a colour of four numbers", paneBox(paneAuto, "❯ \x1b[2;38;2;1;2mrun the tests\x1b[0m")},
		{"faint with a colon of its own", paneBox(paneAuto, "❯ \x1b[2:1mrun the tests\x1b[0m")},
		{"an escape cut off at the end", paneBox(paneAuto, suggestion) + "\x1b["},
		{"a link never ended", paneBox(paneAuto, suggestion) + "\x1b]8;;https://example.invalid/"},
		{"overline as tmux 3.8 writes it, in old output (P1)", "old \x1b[5:3mover\x1b[0m\n" + paneBox(paneAuto, suggestion)},
		{"an SGR with an intermediate byte (P14)", paneBox(paneAuto, "❯ \x1b[2$mrun the tests\x1b[0m")},
		{"a C1 CSI written as U+009B (P15)", paneBox(paneAuto, "❯ \u009b2mrun")},
	} {
		if got := ReadStyledPaneState(c.capture); got == PaneIdle {
			t.Errorf("%s: read idle", c.name)
		}
	}
	// A broken escape drops nothing after it (RT126GHOSTDRAFT1 B1): the real draft in the box below is read, never
	// the older idle box above it.
	idle, draft := paneBox(paneAuto, "❯ "), paneBox(paneAuto, "❯ real draft")
	for _, c := range []struct{ name, capture string }{
		{"ESC [ then a newline (P2)", idle + "\x1b[\n" + draft},
		{"ESC [ then a letter beyond ASCII (P2b)", idle + "\x1b[é" + draft},
		{"ESC then a newline", idle + "\x1b\n" + draft},
		{"a link left open across lines", idle + "\x1b]8;;https://example.invalid/\n" + draft + "\x1b]8;;\x1b\\"},
		{"a link left open by another escape", idle + "\x1b]8;;https://example.invalid/\x1b[2m\n" + draft + "\x1b]8;;\a"},
	} {
		if got := ReadStyledPaneState(c.capture); got != PaneDraft {
			t.Errorf("%s: %v, want draft", c.name, got)
		}
	}
	// A capture that is plain text reads as ReadPaneState reads it, the real fixtures included.
	for _, name := range []string{"idle-empty", "after-decline", "draft", "busy-1", "permission-menu", "trust-menu"} {
		if got, want := ReadStyledPaneState(pane(t, name)), ReadPaneState(pane(t, name)); got != want {
			t.Errorf("%s: %v, plain %v", name, got, want)
		}
	}
	// And a plain read never takes faint text for a suggestion: it doesn't know it is faint.
	if got := ReadPaneState(paneText(paneBox(paneAuto, suggestion))); got != PaneDraft {
		t.Errorf("plain read of a suggestion: %v", got)
	}
}

func TestPaneUnstyleKeepsTheTextAndLinesUpEachRuneWithItsStyle(t *testing.T) {
	text, styles, ok := paneUnstyle("a\x1b[2mé❯\x1b[0m\nb\x1b[7;2mc\x1b[27m\xffd\x1b]8;;x\ae")
	if !ok || text != "aé❯\nbc\xffde" {
		t.Fatalf("text %q ok %v", text, ok)
	}
	want := []paneCell{0, cellFaint, cellFaint, 0, 0, cellFaint, cellFaint, cellFaint, cellFaint}
	if len([]rune(text)) != len(styles) || len(styles) != len(want) {
		t.Fatalf("%d runes, %d styles", len([]rune(text)), len(styles))
	}
	for i := range want {
		if styles[i] != want[i] {
			t.Errorf("rune %d: %v, want %v", i, styles[i], want[i])
		}
	}
	if got := paneText("x\x1b[38;2;1;2;3my\x1b[0m"); got != "xy" {
		t.Errorf("paneText %q", got)
	}
	// Of a broken escape only its ESC goes: every character after it is kept, lined up with a style.
	text, styles, ok = paneUnstyle("a\x1b[\nb\x1b[éc\x1b\nd\x1b]8;x\ne\x1b(Bf\x1b")
	if ok || text != "a[\nb[éc\nd]8;x\neBf" || len(styles) != len([]rune(text)) {
		t.Errorf("broken escapes: text %q ok %v, %d styles", text, ok, len(styles))
	}
}

func TestMailGoesIntoABoxShowingOnlyASuggestionAndWaitsBehindTypedText(t *testing.T) {
	suggestion := paneBox(paneAuto, "❯ "+faint("run the tests"))
	mixed := paneBox(paneAuto, "❯ half typed"+faint(" and run the tests"))
	typed := paneBox(paneAuto, "❯ half typed")
	sent := paneBox(paneAuto, "❯ hi")
	if got, _ := mailGate(hireTarget(), suggestion, false); got != mailDeliver {
		t.Errorf("suggestion only: %v", got)
	}
	for name, capture := range map[string]string{"typed": typed, "mixed": mixed} {
		if got, why := mailGate(hireTarget(), capture, false); got != mailWait || why != "waiting: a draft is in its box" {
			t.Errorf("%s: %v %q", name, got, why)
		}
	}
	w := &fakeWindow{screens: []string{suggestion, suggestion, sent}}
	if r := mailType(hireTarget(), "hi", w, never, yes); r.Outcome != mailSent || len(w.pasted) != 1 || w.submits != 1 {
		t.Errorf("suggestion only: %+v, pasted %q, %d returns", r, w.pasted, w.submits)
	}
	w = &fakeWindow{screens: []string{mixed}}
	if r := mailType(hireTarget(), "hi", w, never, yes); r.Outcome != mailWaiting || r.Typed || len(w.pasted) != 0 {
		t.Errorf("mixed: %+v, pasted %q", r, w.pasted)
	}
	// After the paste the box must hold the message as typed text: a box still showing only the suggestion
	// gets no Return, so the suggestion is never sent.
	w = &fakeWindow{screens: []string{suggestion, suggestion, suggestion}}
	if r := mailType(hireTarget(), "hi", w, never, yes); r.Outcome != mailCannot || w.submits != 0 {
		t.Errorf("paste not shown: %+v, %d returns", r, w.submits)
	}
	// A window that can't be read types nothing.
	w = &fakeWindow{failCap: errors.New("no tmux")}
	if r := mailType(hireTarget(), "hi", w, never, yes); r.Outcome != mailTrouble || r.Typed {
		t.Errorf("unreadable: %+v", r)
	}
}

// The Windows launcher's screen keeps the same styles as it draws them from the agent's output.
func TestTheLaunchersScreenReadsAFaintSuggestionAsIdle(t *testing.T) {
	for _, c := range []struct {
		name, out string
		want      PaneState
	}{
		{"suggestion only", paneBox(paneAuto, "❯ "+faint("run the tests")), PaneIdle},
		{"faint with a colon colour", paneBox(paneAuto, "❯ \x1b[38:2::1:2:3;2mrun the tests\x1b[m"), PaneIdle},
		{"a faint cursor on a suggestion", paneBox(paneAuto, "❯ \x1b[2;7mr\x1b[27mun the tests\x1b[0m"), PaneIdle},
		{"a cursor that isn't faint on a suggestion", paneBox(paneAuto, "❯ \x1b[7mr\x1b[27;2mun the tests\x1b[0m"), PaneDraft},
		{"one typed character under the cursor and a faint tail", paneBox(paneAuto, "❯ \x1b[7mr\x1b[0;2mun the tests\x1b[0m"), PaneDraft},
		{"typed text", paneBox(paneAuto, "❯ half typed"), PaneDraft},
		{"typed text with a suggestion after it", paneBox(paneAuto, "❯ half typed"+faint(" and run the tests")), PaneDraft},
		{"ended by 22", paneBox(paneAuto, "❯ \x1b[2mrun\x1b[22m the tests"), PaneDraft},
		{"the 2 of a 38;2 colour", paneBox(paneAuto, "❯ \x1b[38;2;2;2;2mrun the tests\x1b[39m"), PaneDraft},
		{"an SGR it can't read", paneBox(paneAuto, "❯ \x1b[2;38;5mrun the tests\x1b[0m"), PaneDraft},
		// One it can't read while faint is on leaves its pen faint: the styles are then not used at all (m7).
		{"faint left on by an SGR it can't read", paneBox(paneAuto, "❯ \x1b[2mrun\x1b[0;38;5m typed"), PaneDraft},
		// The 2 of modifyOtherKeys (CSI >4;2m, sent by Claude Code as it starts) never turns the pen faint (L1). A
		// start setting leaves the screen unknown until a full redraw (FXWINSCREENR3), so these rows send Claude
		// Code's own clear after it, as a real start does, then the box: a real draft reads draft, and a box with only
		// a suggestion still reads idle (the setting leaves the pen neither faint nor plain).
		{"modifyOtherKeys, CSI > 4;2 m, Claude Code's clear, then typed text (L1)", "\x1b[>4;2m\x1b[2J\x1b[3J\x1b[H" + paneBox(paneAuto, "❯ my real draft"), PaneDraft},
		{"modifyOtherKeys, CSI > 4;2 m, Claude Code's clear, then a suggestion (L1)", "\x1b[>4;2m\x1b[2J\x1b[3J\x1b[H" + paneBox(paneAuto, "❯ "+faint("run the tests")), PaneIdle},
	} {
		if got := drawn(t, 24, 80, ptyOutput(c.out)).PaneState(); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// With no clear after it, the setting leaves the screen unknown, so mail waits: never idle over a real draft.
	noClear := drawn(t, 24, 80, ptyOutput("\x1b[>4;2m"+paneBox(paneAuto, "❯ my real draft")))
	if got := noClear.PaneState(); got != PaneUnknown || noClear.Why() != "CSI >4;2m" {
		t.Errorf("modifyOtherKeys, CSI > 4;2 m, then typed text with no clear (L1): %v, why %q, want unknown for CSI >4;2m", got, noClear.Why())
	}
	// Typed over a suggestion in place: the faint cells written over are typed now.
	s := drawn(t, 24, 80, ptyOutput(paneBox(paneAuto, "❯ "+faint("run the tests"))))
	if got := s.PaneState(); got != PaneIdle {
		t.Fatalf("before typing: %v", got)
	}
	s.Write([]byte("\x1b[3;3Hrun"))
	if got := s.PaneState(); got != PaneDraft {
		t.Errorf("typed over the suggestion: %v", got)
	}
	// Erased to the line's end, then typed (L2). Typed while the agent left its pen faint reads idle (L3): the same
	// known limit as P4.
	for _, c := range []struct {
		name, typed string
		want        PaneState
	}{
		{"erased to the line's end and typed (L2)", "\x1b[3;3H\x1b[Kmy draft", PaneDraft},
		{"typed with the pen left faint (L3, the known limit)", "\x1b[3;3H\x1b[2mmy draft", PaneIdle},
	} {
		s := drawn(t, 24, 80, ptyOutput(paneBox(paneAuto, "❯ "+faint("run the tests"))))
		s.Write([]byte(c.typed))
		if got := s.PaneState(); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// Erased and redrawn plain: an erase clears the style. Scrolled by a line: the style moves with its line.
	s = drawn(t, 24, 80, ptyOutput(paneBox(paneAuto, "❯ "+faint("run the tests"))))
	s.Write([]byte("\x1b[3;1H\x1b[2K❯ run the tests"))
	if got := s.PaneState(); got != PaneDraft {
		t.Errorf("erased and typed: %v", got)
	}
	s = drawn(t, 5, 80, ptyOutput(paneBox(paneAuto, "❯ "+faint("run the tests"))+"\n"))
	if got := s.PaneState(); got != PaneIdle {
		t.Errorf("scrolled: %v (%q)", got, s.Text())
	}
}
