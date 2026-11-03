package desktop

import (
	"strings"
	"unicode"
)

// PaneState is what a Claude Code session shows in a captured tmux pane
// (the text `tmux capture-pane -p` gives). It is a port of the Mac's
// HireDesk.inputState and HireDesk.promptRefusal (Sources/AgentDeck/
// HireFire.swift), read from the window's structure and never from its
// words: a mail text or old output in the scrollback may say "do you want to
// proceed?" and mean nothing. The shape is from Claude Code 2.1.316; when in
// doubt a pane is PaneUnknown, never PaneIdle.
//
// Nothing here talks to tmux. Where it will be used: the Linux hire engine's
// mail delivery (type only into PaneIdle, looking again just before typing)
// and fire (refuse at PaneMenu, PaneDraft, PaneBusy and PaneUnknown unless
// forced).
type PaneState int

const (
	// PaneUnknown: no input box or menu can be told apart, the screen is cut
	// off or half drawn, or there was nothing to look at. Never type.
	PaneUnknown PaneState = iota
	// PaneIdle: the input box is empty (or shows only the "Try ..." placeholder).
	PaneIdle
	// PaneDraft: something is typed in the box that isn't sent yet.
	PaneDraft
	// PaneBusy: the session is at work ("esc to interrupt" in the footer).
	PaneBusy
	// PaneMenu: a question, a permission menu or the trust menu is waiting
	// for the owner's answer.
	PaneMenu
)

func (s PaneState) String() string {
	switch s {
	case PaneIdle:
		return "idle"
	case PaneDraft:
		return "draft"
	case PaneBusy:
		return "busy"
	case PaneMenu:
		return "menu"
	}
	return "unknown"
}

// paneBottom is how the window looks at the bottom.
type paneBottom struct {
	// box: the lines of the input box between the last two rules, when the
	// window ends in one (hasBox); its first line starts with "❯".
	box    []string
	hasBox bool
	// footer: the lines under the box.
	footer []string
	// tail: the last lines that have words in them, when there is no box.
	tail []string
}

// paneSpace is Swift's CharacterSet.whitespaces: category Zs (non-breaking
// space included) and tab. Claude Code puts a non-breaking space after "❯".
func paneSpace(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }

func paneTrim(s string) string { return strings.TrimFunc(s, paneSpace) }

func paneBlank(s string) bool { return paneTrim(s) == "" }

// paneLines splits on every kind of newline Swift's Character.isNewline
// knows (\r\n counts as one), keeping empty lines.
func paneLines(capture string) []string {
	var lines []string
	var cur strings.Builder
	rs := []rune(capture)
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; r {
		case '\r', '\n', '\v', '\f', '\u0085', ' ', ' ':
			if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			lines = append(lines, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	return append(lines, cur.String())
}

// paneIsRule: a full-width rule, a long run of "─" and nothing else.
func paneIsRule(line string) bool {
	text := []rune(paneTrim(line))
	if len(text) < 20 {
		return false
	}
	for _, r := range text {
		if r != '─' {
			return false
		}
	}
	return true
}

// paneAfterSelector: what follows Claude Code's "❯" selector, and whether
// the line starts with one.
func paneAfterSelector(line string) (string, bool) {
	rest, ok := strings.CutPrefix(paneTrim(line), "❯")
	if !ok {
		return "", false
	}
	return paneTrim(rest), true
}

// paneNumbered: the text after the selector starts "1. " or "2) ".
func paneNumbered(s string) bool {
	rs := []rune(s)
	i := 0
	for i < len(rs) && unicode.IsDigit(rs[i]) {
		i++
	}
	return i > 0 && i+1 < len(rs) && (rs[i] == '.' || rs[i] == ')') && unicode.IsSpace(rs[i+1])
}

// paneTryPlaceholder: exactly `Try "..."`. A capture has no colours, so any
// such text on the box's one line counts as the grey placeholder.
func paneTryPlaceholder(s string) bool {
	return len(s) >= len(`Try ""`) && strings.HasPrefix(s, `Try "`) && strings.HasSuffix(s, `"`)
}

func paneBottomOf(capture string) paneBottom {
	lines := paneLines(capture)
	for len(lines) > 0 && paneBlank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	var rules []int
	for i, l := range lines {
		if paneIsRule(l) {
			rules = append(rules, i)
		}
	}
	if len(rules) >= 2 {
		upper, lower := rules[len(rules)-2], rules[len(rules)-1]
		footer := lines[lower+1:]
		var inside []string
		for _, l := range lines[upper+1 : lower] {
			if !paneBlank(l) {
				inside = append(inside, l)
			}
		}
		// The box sits at the bottom: a footer of a line or two under it,
		// not a screenful.
		if len(footer) <= 3 && len(inside) > 0 {
			if _, ok := paneAfterSelector(inside[0]); ok {
				return paneBottom{box: inside, hasBox: true, footer: footer}
			}
		}
	}
	var words []string
	for _, l := range lines {
		if !paneBlank(l) {
			words = append(words, l)
		}
	}
	if len(words) > 15 {
		words = words[len(words)-15:]
	}
	return paneBottom{tail: words}
}

// paneAsking: the window shows a question or a menu. Told by its shape: no
// input box at the bottom, and either a numbered list with the "❯ N."
// selector or a confirm/cancel footer.
func paneAsking(b paneBottom) bool {
	if b.hasBox {
		return false
	}
	for _, l := range b.tail {
		if after, ok := paneAfterSelector(l); ok && paneNumbered(after) {
			return true
		}
	}
	last := ""
	if len(b.tail) > 0 {
		last = strings.ToLower(b.tail[len(b.tail)-1])
	}
	for _, w := range []string{"enter to confirm", "esc to cancel", "enter to select", "to navigate"} {
		if strings.Contains(last, w) {
			return true
		}
	}
	return false
}

// ReadPaneState says what state the session in a captured pane is in. A
// failed capture is the caller's to treat as PaneUnknown (an empty string
// gives it too).
func ReadPaneState(capture string) PaneState {
	b := paneBottomOf(capture)
	if paneAsking(b) {
		return PaneMenu
	}
	if !b.hasBox {
		return PaneUnknown
	}
	// Every real window has a footer under its box: with none, the screen
	// was cut off or half drawn, and "esc to interrupt" may be in the part
	// that's missing. In doubt, nothing is typed.
	allBlank := true
	for _, f := range b.footer {
		if !paneBlank(f) {
			allBlank = false
		}
		if strings.Contains(strings.ToLower(f), "esc to interrupt") {
			return PaneBusy
		}
	}
	if allBlank {
		return PaneUnknown
	}
	if len(b.box) > 1 {
		return PaneDraft
	}
	if typed, _ := paneAfterSelector(b.box[0]); typed == "" || paneTryPlaceholder(typed) {
		return PaneIdle
	}
	return PaneDraft
}
