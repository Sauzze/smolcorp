package desktop

import (
	"fmt"
	"strings"
	"testing"
)

// A reply in proper French typography (FXFRENCHSPACES): the narrow no-break space (U+202F) before ? and ! and
// inside « », the no-break space (U+00A0) before a colon, the thin space (U+2009) before a semicolon, and ’ … — œ.
var claudeFrenchTypography = claudeReply{
	"  « Bonjour ! » Voici : …",
	"  C’est prêt ? Une note ; l’œuvre — à vérifier",
}

// French typography's three spaces: the no-break, the narrow no-break and the thin space.
var frenchSpaces = []rune{0x00a0, 0x202f, 0x2009}

// A Claude Code reply in French typography reads known, in the state each step shows, on a console with a Western
// code page, in chunks that split the spaces' UTF-8 too.
func TestScreenReadsAClaudeCodeReplyInFrenchTypographyOnAWesternCodePage(t *testing.T) {
	reply := claudeFrenchTypography.line2 + claudeFrenchTypography.line3
	for _, r := range append([]rune{'«', '»'}, frenchSpaces...) {
		if !strings.ContainsRune(reply, r) {
			t.Fatalf("the reply holds no %+q", r)
		}
	}
	for _, chunk := range []int{1, 2, 7, 4096} {
		s := newTextScreen(60, 200, westernCP)
		for _, step := range claudeStepsReplying(t, claudeFrenchTypography) {
			for out := []byte(step.out); len(out) > 0; out = out[min(chunk, len(out)):] {
				s.Write(out[:min(chunk, len(out))])
			}
			if s.Unknown() || s.PaneState() != step.want {
				t.Errorf("chunks of %d, %s: %v, unknown %v (why %q), want %v", chunk, step.name, s.PaneState(), s.Unknown(), s.Why(), step.want)
			}
			if got := ReadPaneState(step.text); got != step.want {
				t.Errorf("%s: the text alone reads %v", step.name, got)
			}
			if got := paneLinesTrimmed(s.Text()); got != paneLinesTrimmed(step.text) {
				t.Errorf("chunks of %d, %s: the screen is\n%s", chunk, step.name, got)
			}
		}
	}
}

// On a CJK code page, or one unread, the same reply reads unknown: ’ … — œ ê à è are Ambiguous. The spaces and « »
// are Neutral, so a line of only those and ASCII stays known there; each Ambiguous character in the reply breaks
// the screen, as an Ambiguous one.
func TestScreenReadsAReplyInFrenchTypographyUnknownOnACJKCodePage(t *testing.T) {
	for _, page := range []screenCodePage{cjkCP, {output: 65001, locale: localeCJK}, {}} {
		s := newTextScreen(60, 200, page)
		for _, step := range claudeStepsReplying(t, claudeFrenchTypography) {
			s.Write([]byte(step.out))
			if !s.Unknown() || s.PaneState() != PaneUnknown || s.Why() != page.ambiguousWhy() {
				t.Errorf("code page %d, %s: %v, unknown %v, why %q", page.output, step.name, s.PaneState(), s.Unknown(), s.Why())
			}
		}

		neutral := "« Bonjour ! » Voici : ok ; fin"
		s = newTextScreen(10, 40, page)
		if s.Write([]byte("\x1b[2;3H" + neutral)); s.Unknown() || paneLinesTrimmed(s.Text()) != paneLinesTrimmed("\n  "+neutral+strings.Repeat("\n", 8)) {
			t.Errorf("code page %d, only Neutral characters: unknown %v (why %q), the screen is\n%s", page.output, s.Unknown(), s.Why(), s.Text())
		}

		var ambiguous []rune
		for _, r := range claudeFrenchTypography.line2 + claudeFrenchTypography.line3 {
			if screenOneCellOffCJK(r) {
				ambiguous = append(ambiguous, r)
			}
		}
		if len(ambiguous) == 0 {
			t.Fatal("the reply holds no Ambiguous character")
		}
		for _, r := range ambiguous {
			s := newTextScreen(10, 40, page)
			if s.Write([]byte("\x1b[2;3H« " + string(r))); !s.Unknown() || s.Why() != page.ambiguousWhy() {
				t.Errorf("%q on code page %d: unknown %v, why %q", r, page.output, s.Unknown(), s.Why())
			}
		}
	}
}

// Each of French typography's spaces, and « », is one cell on any code page, a Western one, a CJK one or one unread.
func TestScreenDrawsEachFrenchSpaceOneCell(t *testing.T) {
	for _, r := range append(append([]rune{}, frenchSpaces...), '«', '»') {
		for _, page := range []screenCodePage{westernCP, cjkCP, {output: 65001, locale: localeCJK}, {}} {
			s := newTextScreen(1, 10, page)
			if s.Write([]byte("a" + string(r) + "b")); s.Unknown() || s.Text() != "a"+string(r)+"b" || s.col != 3 {
				t.Errorf("%+q on code page %d: %q, column %d, unknown %v (why %q)", r, page.output, s.Text(), s.col, s.Unknown(), s.Why())
			}
			// At the last column it takes that one cell and leaves the wrap pending, as an ASCII character does.
			s = newTextScreen(2, 3, page)
			if s.Write([]byte("ab" + string(r) + "c")); s.Unknown() || s.Text() != fmt.Sprintf("ab%c\nc", r) {
				t.Errorf("%+q on code page %d, at the last column: %q, unknown %v (why %q)", r, page.output, s.Text(), s.Unknown(), s.Why())
			}
		}
	}
}
