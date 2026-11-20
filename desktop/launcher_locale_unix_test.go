//go:build !windows

package desktop

import "testing"

// A `smolcorp run` tab off Windows reads its code page from the locale (RT126WINSCREENR4 N3): with a French UTF-8
// locale Claude Code's box (─) reads known, and the phone's multi-line text is pasted as bracketed; with a Japanese
// one the screen reads unknown and the paste is not taken.
func TestALauncherTabOffWindowsReadsTheBoxInAWesternLocaleOnly(t *testing.T) {
	for _, c := range []struct {
		lang  string
		known bool
	}{
		{"fr_FR.UTF-8", true},
		{"ja_JP.UTF-8", false},
	} {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_CTYPE", "")
		t.Setenv("LANG", c.lang)
		screen := newLauncherScreen(60, 200, true, consoleCodePage())
		screen.Write([]byte(claudeStream(t, "the start screen and its box")))
		state, unknown := screen.Look()
		if unknown == c.known || (c.known && state != PaneIdle) {
			t.Errorf("LANG=%s: %v, unknown %v, why %q", c.lang, state, unknown, screen.screen.Why())
		}
		paste, ok := (&cursorMode{bracketed: screen.Bracketed}).pasteBytes("a note\non two lines")
		if ok != c.known || (ok && paste != pasteStart+"a note\non two lines"+pasteEnd) {
			t.Errorf("LANG=%s: the phone's text pasted as %q, %v", c.lang, paste, ok)
		}
	}
}
