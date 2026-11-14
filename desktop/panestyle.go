package desktop

import (
	"strings"
	"unicode/utf8"
)

// Claude Code's prompt suggestion: when a session is idle, it may show a
// guess at what to ask next in its input box, in grey (faint, SGR 2). Nothing
// is typed, but a plain capture can't tell it from a draft, so mail waited
// for hours behind it and fire refused (T126GHOSTDRAFT1). A window is read
// with its escapes (`tmux capture-pane -p -e`; the Windows launcher's screen
// keeps the same styles as it draws), and a box that holds nothing but faint
// text is idle. Typed text is never faint, so any of it still makes a draft.
// Grey drawn as a colour (SGR 90, 38;5;n) is not faint and still counts as
// typed: in doubt, nothing is typed. So does every character when the escapes
// can't all be read, and so does a character drawn in reverse video but not
// faint, even the first one: a cursor parked on one typed character, with a
// faint tail after it, looks the same as a cursor drawn over a suggestion.

// paneCell is the style of one character of a window, as far as telling a
// suggestion from a draft goes.
type paneCell uint8

// cellFaint: drawn faint (SGR 2), the grey of a suggestion.
const cellFaint paneCell = 1

// paneSGR is pen after the SGR sequence ESC [ params m. Only faint is
// followed: 0 (and an empty parameter) ends it, 2 starts it, 22 ends it (with
// bold). A colour's own
// numbers (38;5;n, 38;2;r;g;b, or with ":") are skipped, so the 2 of 38;2;…
// is never faint. ok is false for anything that isn't an SGR sequence of
// numbers, ";" and ":": then the styles are not to be trusted.
func paneSGR(params string, pen paneCell) (paneCell, bool) {
	number := func(p string) (int, bool) {
		n := 0
		for _, c := range []byte(p) {
			if c < '0' || c > '9' {
				return 0, false
			}
			n = min(n*10+int(c-'0'), 1000)
		}
		return n, true
	}
	parts := strings.Split(params, ";")
	for k := 0; k < len(parts); k++ {
		code, sub, colon := strings.Cut(parts[k], ":")
		n, ok := number(code)
		if !ok {
			return pen, false
		}
		if colon {
			// Only a colour (38, 48, 58) or an underline's kind (4:3) carries its numbers after ":".
			if n != 38 && n != 48 && n != 58 && n != 4 {
				return pen, false
			}
			for _, c := range []byte(sub) {
				if (c < '0' || c > '9') && c != ':' {
					return pen, false
				}
			}
			continue
		}
		switch n {
		case 0:
			pen = 0
		case 2:
			pen |= cellFaint
		case 22:
			pen &^= cellFaint
		case 38, 48, 58:
			skip := 0
			if k+1 < len(parts) {
				switch parts[k+1] {
				case "5":
					skip = 2
				case "2":
					skip = 4
				}
			}
			if skip == 0 || k+skip >= len(parts) {
				return pen, false
			}
			for _, p := range parts[k+1 : k+skip+1] {
				if _, ok := number(p); !ok || p == "" {
					return pen, false
				}
			}
			k += skip
		}
	}
	return pen, true
}

// paneUnstyle reads a capture taken with its escapes: the text a plain
// capture gives, and the style of each of its runes ([]rune(text) and styles
// line up). The style carries on from line to line, as tmux writes it. Only
// SGR sequences and OSC strings (a link) are expected; ok is false for any
// other escape, a broken one or one cut off at the end. The text then still
// holds every character: of a broken escape only its ESC is dropped, and the
// rest is read on as text, so a box further down is never lost (RT126GHOSTDRAFT1
// B1: cut there, an older idle box above it was read instead).
func paneUnstyle(capture string) (text string, styles []paneCell, ok bool) {
	var b strings.Builder
	var pen paneCell
	ok = true
	for i := 0; i < len(capture); {
		if capture[i] != 0x1b {
			_, n := utf8.DecodeRuneInString(capture[i:])
			b.WriteString(capture[i : i+n])
			styles = append(styles, pen)
			i += n
			continue
		}
		if i+1 >= len(capture) {
			ok = false
			i++
			continue
		}
		switch capture[i+1] {
		case '[':
			j := i + 2
			for j < len(capture) && capture[j] >= 0x30 && capture[j] <= 0x3f {
				j++
			}
			params := capture[i+2 : j]
			for j < len(capture) && capture[j] >= 0x20 && capture[j] <= 0x2f {
				j++
			}
			if j >= len(capture) || capture[j] < 0x40 || capture[j] > 0x7e {
				ok = false
				i++
				continue
			}
			plain := j == i+2+len(params) && (params == "" || !strings.ContainsAny(params[:1], "<=>?"))
			if capture[j] != 'm' || !plain {
				ok = false
			} else if next, good := paneSGR(params, pen); good {
				pen = next
			} else {
				ok = false
			}
			i = j + 1
		case ']':
			// Ended by BEL or ESC \. Any other control character (a newline, another
			// ESC) first means it was never ended: it doesn't swallow the lines up
			// to a later link's end.
			j := i + 2
			for j < len(capture) && capture[j] >= 0x20 {
				j++
			}
			switch {
			case j < len(capture) && capture[j] == 0x07:
				i = j + 1
			case j+1 < len(capture) && capture[j] == 0x1b && capture[j+1] == '\\':
				i = j + 2
			default:
				ok = false
				i++
			}
		default:
			// A two-character escape (ESC 7, ESC =, a charset's ESC ( B) drops
			// its second character too; a newline or a letter beyond ASCII
			// after the ESC is kept.
			ok = false
			if c := capture[i+1]; c >= 0x20 && c <= 0x7e {
				i += 2
			} else {
				i++
			}
		}
	}
	return b.String(), styles, ok
}

// paneText is a capture taken with its escapes as plain text.
func paneText(capture string) string {
	text, _, _ := paneUnstyle(capture)
	return text
}

// ReadStyledPaneState is ReadPaneState for a capture taken with its escapes
// (`tmux capture-pane -p -e`): a box holding only Claude Code's faint
// suggestion is idle. A capture with no escapes reads as ReadPaneState reads
// it; one whose escapes can't all be read is read as plain text, so a
// suggestion in it still counts as typed.
func ReadStyledPaneState(capture string) PaneState {
	text, styles, ok := paneUnstyle(capture)
	if !ok {
		styles = nil
	}
	return readPaneState(text, styles)
}

// paneOnlySuggested: the box holds nothing but a suggestion. Everything in it
// after the selector, on every line, is faint or blank, and at least one
// character is faint. No character is let off, not even one under a cursor
// (RT126GHOSTDRAFT1 B2). With no styles, nothing is a suggestion.
func paneOnlySuggested(b paneBottom, styles []paneCell) bool {
	if styles == nil || len(b.box) != len(b.boxStarts) {
		return false
	}
	faint := 0
	for i, line := range b.box {
		rs := []rune(line)
		from := 0
		if i == 0 {
			at := strings.IndexRune(line, '❯')
			if at < 0 {
				return false
			}
			from = utf8.RuneCountInString(line[:at]) + 1
		}
		for j := from; j < len(rs); j++ {
			if paneSpace(rs[j]) {
				continue
			}
			if at := b.boxStarts[i] + j; at >= len(styles) || styles[at]&cellFaint == 0 {
				return false
			}
			faint++
		}
	}
	return faint > 0
}
