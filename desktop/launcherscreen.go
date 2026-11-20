package desktop

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// textScreen is a small rows-by-cols text screen: what the Windows launcher's
// pty output (go-pty, ConPTY) looks like once drawn, as the text
// `tmux capture-pane -p` gives on Linux, so ReadPaneState can read it. It is
// not a terminal emulator.
//
// It is known only for what is on its allow-list (below): what it draws
// exactly as xterm and conhost (on Windows 10 and 11) both draw it. Anything
// else breaks the page: it reads PaneUnknown, never PaneIdle, so in doubt
// nothing is typed. Breaks are of two kinds:
//   - Known to change only cells and the cursor (a character off the list,
//     a cursor move or an erase off the list, a report asked of the
//     terminal): a full redraw brings the page back, that is an erase of the
//     whole screen (CSI 2J) and then the cursor put at a known place (CSI H),
//     or the alternate screen entered fresh (CSI ?1049h). A cursor saved
//     meanwhile (ESC 7 while the page is broken) may be elsewhere in the
//     terminal than on the screen: restoring it breaks the page again, even
//     after the redraw.
//   - Of unknown effect (any other sequence, a mode not on the list, a
//     string other than a title): it may have changed what a terminal keeps
//     through a redraw (the saved cursors, the scroll region, the tab stops,
//     the modes), so only a full reset (ESC c) brings the screen back.
//
// A few modes off the list are followed rather than of unknown effect (no
// autowrap, origin, insert, new line, a character set other than ASCII):
// while one is on, no redraw brings the page back; once it's off again, the
// next full redraw does. Reset while already off, one is skipped only when
// that moves nothing on xterm and conhost (?7h, 4l, 20l, ESC ( B, ESC ) B);
// origin's set or reset puts the cursor home on both, so it breaks the page
// every time (RT126WINSCREENR4 B1).
//
// What a character's width is can depend on the console's code page
// (screenCodePage), which the launcher reads and gives the screen: an input
// of the screen, not something it guesses.
//
// # The allow-list
//
// Each entry is drawn the same on xterm and on conhost (the Windows 10 and 11
// console, which ConPTY's output reaches through the launcher's copy). What
// is said of conhost is read from its source and documentation, not run here.
//
// Characters:
//   - Printable ASCII (0x20..0x7E): one cell everywhere, by every width table.
//   - East Asian Wide and Fullwidth ideographs, kana, Hangul syllables, CJK
//     punctuation and fullwidth forms, in the BMP (screenCJK): two cells in
//     xterm's wcwidth and in conhost's width table since its first VT build,
//     whatever the font; the marks among them (U+302A..302F, U+3099, U+309A)
//     are left out.
//   - Claude Code's own glyphs that are East Asian Neutral and not emoji
//     (screenGlyphs): conhost measures by the font only the Ambiguous ones,
//     and draws emoji wide in some builds; a Neutral non-emoji character is
//     one cell in its table, in xterm's and in Windows Terminal's.
//   - French typography's spaces and guillemets (screenFrenchSpacing): the
//     narrow no-break space and the thin space, and « »; with the no-break
//     space already among Claude Code's glyphs. Each is East Asian Neutral,
//     so one cell in the same tables, on any code page (FXFRENCHSPACES).
//   - Only while the console's code page isn't a CJK one (screenCodePage):
//     the East Asian Ambiguous characters in the BMP (screenAmbiguous: the box
//     drawing ─, the dot ·, the arrows, …, é, à and the rest), the accented
//     Latin letters (ç, ï and the other Neutral ones), and the emoji drawn as
//     text (screenTextEmoji: ⏺ ⏸ ✳ and the others whose default is text, with
//     no variation selector). Conhost measures an Ambiguous character by the
//     font, which is one cell in every font a Western code page uses (Consolas,
//     Lucida Console, Cascadia), and two in a CJK one; xterm and Windows
//     Terminal draw them one cell unless told to draw them wide for CJK. On a
//     CJK code page, or one that couldn't be read, they break the page. The
//     width that counts is the inner ConPTY's, which lays out the stream read
//     here and counts them one cell on any code page (as read, not run here);
//     the owner's console only redraws a copy, so the CJK rule is a
//     conservative stand-in (RT126WINSCREENR4 N1).
//   - Nothing else: no combining mark, variation selector, joiner, emoji
//     drawn as an emoji (Emoji_Presentation, VS16, a ZWJ sequence, a flag, a
//     skin tone), private use character, nor a character outside the BMP.
//
// Controls:
//   - CR: to column 1, everywhere.
//   - LF: down a row, or the scroll region scrolls at its bottom row; a
//     deferred wrap pending is followed (wrapUnsure).
//   - BS inside a row: left one column. At column 1 or with a wrap pending,
//     it differs (an older console goes up a row).
//   - TAB over blanks to a stop inside the window, with the tab stops
//     default (every 8 columns: anything that sets or clears them is of
//     unknown effect, healed only by ESC c, which sets them back): xterm
//     moves the cursor, conhost moves it or writes blanks, which is the same
//     over blanks.
//
// CSI sequences:
//   - CUP and HVP (CSI r;c H, f), CUU CUD CUF CUB (A B C D), CHA (G) and VPA
//     (d), with at most the numbers they take and a target inside the
//     screen: put the cursor there and end a pending wrap, on both. A target
//     past an edge is clamped by both, but isn't on the list. CUF and CUB
//     with a wrap pending are off it too.
//   - EL (CSI 0/1/2 K) and ED (CSI 0/1/2/3 J): blank the cells, the cursor
//     stays (3 clears the scrollback only, which a capture doesn't show);
//     an erase whose edge cuts a wide character in two is off the list.
//   - ECH (CSI n X), ICH (CSI n @), DCH (CSI n P), SU (CSI n S), SD (CSI n T)
//     and DECSTBM (CSI top;bottom r), with at most one number (two for
//     DECSTBM): see eraseChars, shiftCells, scrollRegion and region for why
//     both draw them alike, and where they stop being on the list (SU and SD
//     only while no region of part of the screen is set).
//   - SGR (m), colon forms included: styles only, no cell moves. Faint is
//     kept for Claude Code's grey suggestion (panestyle.go).
//   - The questions Claude Code asks as it starts, each only in the exact
//     form it sends (2.1.329): the device's attributes (DA1, CSI c or
//     CSI 0c), which xterm and conhost both answer on input; the terminal's
//     version (XTVERSION, CSI >q or CSI >0q), which xterm answers and conhost
//     ignores as a CSI it doesn't know; and the kitty keyboard's flags (CSI ?u,
//     no number), which xterm and conhost read with ? as a private marker, so
//     never as CSI u (the cursor restored), and answer or ignore. No cell
//     changes on either. Any other spelling (CSI 00c, CSI >1q, CSI ?0u) stays
//     off the list, and a ?u with a number in it (?5u) is no question.
//   - The private modes known to draw nothing, set or reset: the cursor
//     shown (?25), bracketed paste (?2004), synchronized output (?2026),
//     focus reports (?1004), the mouse reports (?9, ?1000, ?1002, ?1003,
//     ?1005, ?1006, ?1015, ?1016) and the colour palette's updates (?2031,
//     a theme change reported as CSI ?997;1n or 2n; Claude Code sets it as
//     it starts from 2.1.327 at least). They change what the terminal sends,
//     or when it paints, never a cell.
//   - The alternate screen (?1049h from the main screen, ?1049l back from
//     it): entering saves the cursor and shows a blank page with the cursor
//     where it was; leaving shows the main page as it was and restores the
//     cursor. Each page has its own cells, cursor, deferred wrap and saved
//     cursor. A switch while a followed mode or a scroll region is set is of
//     unknown effect (conhost keeps autowrap, the new-line mode and, on
//     Windows 10, the margins per screen buffer; xterm keeps one of each).
//   - The window's size said (CSI 8;rows;cols t) at the screen's own size,
//     as ConPTY says it when it starts and after each resize: xterm ignores
//     it or resizes to the size it has, conhost resizes to the size it has.
//
// Other:
//   - A title (OSC 0 or OSC 2, ended by BEL or ST, with no control inside):
//     xterm and conhost both set the window's title and draw nothing.
//   - The same, ended so and with no control inside, for the other strings
//     Claude Code sends as it starts (2.1.329), known to draw nothing on xterm
//     and on conhost: the background colour asked (OSC 11, its body exactly
//     11;?), which a terminal answers or ignores; Claude Code's own question
//     (OSC 7501, its body exactly 7501;?), a number both ignore, as they
//     ignore any OSC they don't know; and a link opened (OSC 8;params;URI)
//     or closed (OSC 8;;), whose text between the two is ordinary text,
//     drawn as text. Any other body of OSC 11 or 7501 (a colour set, say),
//     and any other OSC, stays off the list.
//   - The cursor saved and restored (ESC 7, ESC 8: DECSC, DECRC), with the
//     screen's own saved cursor per page. Both terminals keep the position
//     (xterm by screen row; conhost relative to the viewport, which in ConPTY
//     is the page), and text that scrolls moves it on neither. With nothing
//     saved, ESC 8 goes home in the plain style with origin and the character
//     sets off, on xterm and on current conhost (an empty saved cursor there
//     is row 1, column 1, default attributes). What the screen can't be sure
//     a terminal restores alike stays off the list, so the restore breaks the
//     page until a full redraw: a cursor saved while the page was broken,
//     with a wrap unsure, before a resize, or on the alternate page before
//     leaving it (unsure); one saved after the last column (whether the
//     deferred wrap comes back differs); on the alternate page, one it never
//     saved; a restore with a scroll region set (both restore by absolute
//     position with origin off, but kept off to be safe); and a saved style
//     other than the pen (xterm and current conhost restore it, Windows 10's
//     conhost may keep the position only, and faint is what tells a draft
//     from a suggestion), so a known restore leaves the pen as it was on any
//     of them. A save while the page is known carries no followed mode (one
//     on means the page is broken), so a known restore turns none on; one
//     that turns a mode off still breaks the page.
//   - The full redraw: CSI 2J, then CSI H (or CSI f). Every visible cell is
//     blank on both and the cursor is put at a known place.
//   - The full reset, ESC c: both reset every mode, the region, the tab
//     stops and the character sets, and leave the alternate screen, and
//     clear the saved cursor.
//
// Off the list still: the cursor saved and restored by CSI s and CSI u. CSI s
// is also how a terminal with left and right margins (?69) sets them, and
// CSI u also the kitty keyboard's spelling, so each breaks the page until a
// full redraw, and a CSI s with numbers leaves the saved cursor unsure.
//
// The launcher feeds it through a launcherScreen (launcherlook.go), which
// holds the lock. Not safe for concurrent use by itself.
type textScreen struct {
	rows, cols int
	// The page drawn now: the main screen, or the alternate one while it's on.
	screenPage
	// main: the main screen, kept as it was while the alternate one is on; nil otherwise.
	main *screenPage
	// top, bottom: the scroll region's first and last rows (the whole screen unless set).
	top, bottom int
	// pen: the style the next character is drawn in.
	pen paneCell
	// styleUnsure: an SGR sequence couldn't be read, so the styles may be
	// wrong and are not used (every character counts as typed); only Reset
	// clears it.
	styleUnsure bool

	mode    screenMode
	params  []byte // the CSI parameters and intermediates being read
	nums    []int  // scratch for the numbers in params
	osc     int    // bytes of the string (OSC, DCS…) skipped so far
	oscC2   bool   // the last byte in the string was 0xC2 (a C1 control follows if 80..9F)
	oscBel  bool   // the string ends at BEL too (an OSC; a DCS, SOS, PM or APC ends only at ST)
	str     byte   // the kind of string: ] OSC, P DCS, X SOS, ^ PM, _ APC
	strHead []byte // a string's first bytes, which say what it is
	strCtl  bool   // the string held a control byte
	pending []byte // the start of a UTF-8 character cut at the end of a write

	// lost: why the screen can never be trusted (a console of no known size, a
	// window too big); "" when it can. Only a Reset to a good size clears it.
	lost string
	// needsReset: why a sequence of unknown effect broke the screen; "" if none
	// did. Only a full reset (ESC c) clears it.
	needsReset string
	// odd: the modes on that change how things are drawn and that the screen
	// follows but doesn't draw (no autowrap, origin, insert, a character set
	// other than ASCII…). While any is on, a full redraw doesn't bring the
	// page back.
	odd screenModes
	// page: the console's code page, which says how wide an Ambiguous
	// character is; kept through a reset.
	page screenCodePage

	// bracketed: the agent asked for bracketed paste (mode 2004). Only said; it
	// never changes how the screen reads.
	bracketed bool
}

// screenPage is one screen's cells and cursor: the main screen or the alternate one.
type screenPage struct {
	grid [][]rune
	// style: whether each cell of grid is faint.
	style [][]paneCell
	// dirty: the row may hold something other than blanks; a clear skips the
	// rows that don't, so a stream of clears costs little (kept per row
	// slice, so it moves with a scroll).
	dirty    []bool
	row, col int
	// wrap: the last column was just written; the next character starts a new
	// line (the deferred wrap of real terminals).
	wrap bool
	// wrapUnsure: a deferred wrap was pending when something came that
	// terminals may or may not end it with (an erase, a scroll, a line feed);
	// where the next character goes isn't certain until the cursor is put.
	wrapUnsure bool
	// saved: the cursor ESC 7, CSI s or CSI ?1049h saved, this page's own.
	saved savedCursor
	// broken: the page may not be what the window shows; why names the first
	// thing that did it (a sequence's own bytes or a kind of character, never
	// what the screen holds). erased: since it broke, the whole screen was
	// erased, so the page is right once the cursor is put at a known place.
	broken, erased bool
	why            string
}

type savedCursor struct {
	row, col  int
	pen       paneCell
	wrap, set bool
	// unsure: saved while the page was broken (or kept through something that
	// may have moved it), so the terminal's may be elsewhere.
	unsure bool
	// odd: the modes saved with the cursor and restored with it (origin, the
	// character sets), as xterm and conhost both do.
	odd screenModes
}

// screenModes are the modes that change drawing in a way the screen doesn't draw, but follows.
type screenModes uint16

const (
	modeNoWrap    screenModes = 1 << iota // CSI ?7l
	modeOrigin                            // CSI ?6h
	modeInsert                            // CSI 4h
	modeNewline                           // CSI 20h: a line feed also returns
	modeCharsetG0                         // ESC ( and a set other than ASCII (B)
	modeCharsetG1                         // ESC ) and a set other than ASCII

	// savedModes: the modes a saved cursor carries.
	savedModes = modeOrigin | modeCharsetG0 | modeCharsetG1
)

// screenTail is the second cell of a wide character.
const screenTail rune = -1

type screenMode int

const (
	modeGround screenMode = iota
	modeEsc
	modeCSI
	modeOSC
	modeOSCEsc
	modeCharset  // ESC (: the character set to choose for G0 comes next
	modeCharset1 // ESC ): the same for G1
)

const (
	screenMaxParam = 10000
	screenMaxOSC   = 4096
	screenMaxCSI   = 64
	screenMaxSize  = 1000 // rows and columns; a bigger window is Unknown
	screenMaxHead  = 64   // bytes of a string looked at to tell what it is
)

// screenCodePage is the console's output code page (GetConsoleOutputCP) and
// what the user's locale says, as the Windows launcher reads them when it
// starts and as it follows the console (launcher_windows.go): whether an East
// Asian Ambiguous character may be two cells wide. Conhost measures those by
// the font, and a CJK code page uses a CJK font. Off Windows, and in tests, it
// is given. The zero value is a code page that couldn't be read, taken as a CJK
// one.
type screenCodePage struct {
	output uint32 // 0: couldn't be read
	// locale: for UTF-8 (65001) and GB18030 (54936), which a console of any
	// language may use, whether the user's locale is a CJK one.
	locale screenLocale
}

type screenLocale int

const (
	localeUnknown screenLocale = iota
	localeNotCJK
	localeCJK
)

// cjk: an Ambiguous character may be two cells wide on this code page, or the
// code page couldn't be read. Japanese (932), Simplified Chinese (936), Korean
// (949) and Traditional Chinese (950) are conhost's East Asian code pages; the
// other East Asian code pages Windows knows are taken alike, to be safe; UTF-8
// and GB18030 count as CJK unless the locale is known not to be one.
func (c screenCodePage) cjk() bool {
	switch c.output {
	case 0, 932, 936, 949, 950,
		1361, 10001, 10002, 10003, 10008, 20000, 20932, 20936, 20949, 50220, 50221, 50222, 50225, 50227, 50229, 51932,
		51936, 51949, 52936:
		return true
	case 54936, 65001:
		return c.locale != localeNotCJK
	}
	return false
}

// ambiguousWhy is why an Ambiguous character breaks the page on this code
// page: its number, never the character.
func (c screenCodePage) ambiguousWhy() string {
	if c.output == 0 {
		return "an ambiguous-width character, the console's code page unread"
	}
	return fmt.Sprintf("an ambiguous-width character on code page %d", c.output)
}

// localeCodePage is the code page off Windows, where a terminal has none the
// launcher can read: the locale that says how characters are encoded (LC_ALL,
// else LC_CTYPE, else LANG, the first one set, as POSIX reads them). A UTF-8
// locale whose language isn't Chinese, Japanese or Korean counts as a Western
// code page, as a Unix terminal draws an Ambiguous character one cell unless
// told to draw them wide for CJK; a CJK one counts as CJK, and one unset, in
// another encoding or malformed as unread: both break the page on an
// Ambiguous character (RT126WINSCREENR4 N3). Its language is well formed only
// as 2 or 3 lowercase letters, or C (C.UTF-8): a space, a hyphen, a capital
// or a longer name breaks (RT126WINSCREENR5 N1). The CJK ones are the 2- and
// 3-letter codes for Japanese, Korean and Chinese (jpn, kor, zho and chi
// too), and Cantonese.
func localeCodePage(getenv func(string) string) screenCodePage {
	name, _, _ := strings.Cut(cmp.Or(getenv("LC_ALL"), getenv("LC_CTYPE"), getenv("LANG")), "@")
	language, codeset, _ := strings.Cut(name, ".")
	language, _, _ = strings.Cut(language, "_")
	if !localeLanguage(language) || !strings.EqualFold(strings.ReplaceAll(codeset, "-", ""), "utf8") {
		return screenCodePage{}
	}
	switch language {
	case "ja", "ko", "zh", "yue", "jpn", "kor", "zho", "chi":
		return screenCodePage{output: 65001, locale: localeCJK}
	}
	return screenCodePage{output: 65001, locale: localeNotCJK}
}

// localeLanguage says whether a locale's language is well formed: C, or 2 or 3
// letters a to z.
func localeLanguage(language string) bool {
	if language == "C" {
		return true
	}
	if len(language) < 2 || len(language) > 3 {
		return false
	}
	for _, r := range language {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func newTextScreen(rows, cols int, page screenCodePage) *textScreen {
	s := &textScreen{page: page}
	s.Reset(rows, cols)
	return s
}

// SetCodePage gives the screen the console's code page. A change in whether
// Ambiguous characters may be wide breaks the page until a full redraw: those
// already drawn may now be drawn otherwise.
func (s *textScreen) SetCodePage(page screenCodePage) {
	if page.cjk() != s.page.cjk() {
		s.breakCells("the console's code page changed")
	}
	s.page = page
}

// Reset clears the screen and everything unknown about it, at a new size.
func (s *textScreen) Reset(rows, cols int) {
	*s = textScreen{rows: max(rows, 1), cols: max(cols, 1), page: s.page}
	if s.rows > screenMaxSize || s.cols > screenMaxSize {
		s.rows, s.cols, s.lost = 1, 1, "a window over 1000 rows or columns"
	}
	s.screenPage = newScreenPage(s.rows, s.cols)
	s.top, s.bottom = 0, s.rows-1
}

func newScreenPage(rows, cols int) screenPage {
	p := screenPage{grid: make([][]rune, rows), style: make([][]paneCell, rows), dirty: make([]bool, rows)}
	for i := range p.grid {
		p.grid[i] = blankRow(cols)
		p.style[i] = make([]paneCell, cols)
	}
	return p
}

func blankRow(cols int) []rune {
	r := make([]rune, cols)
	for i := range r {
		r[i] = ' '
	}
	return r
}

// breakCells marks the page as not to be trusted until a full redraw, saying
// why: for something known to change only the cells and the cursor.
func (p *screenPage) breakCells(why string) {
	if !p.broken {
		p.why = why
	}
	p.broken, p.erased = true, false
}

// breaks is for a sequence of unknown effect: besides the cells and the
// cursor, it may have changed what a redraw keeps (the saved cursors of both
// pages, the scroll region, the tab stops, the modes), so the screen stays
// unknown until a full reset (ESC c).
func (s *textScreen) breaks(why string) {
	if s.needsReset == "" {
		s.needsReset = why
	}
	s.breakCells(why)
}

// redrawn: the whole page is known again, unless a mode the screen doesn't draw is still on.
func (s *textScreen) redrawn() {
	if s.odd == 0 {
		s.broken, s.erased, s.why = false, false, ""
	}
}

// Unknown: the screen can't be trusted to be what the window shows.
func (s *textScreen) Unknown() bool { return s.lost != "" || s.needsReset != "" || s.broken }

// Why names what made the screen unknown (a sequence's bytes or a kind of
// character, never the screen's text), or "" when it is known.
func (s *textScreen) Why() string {
	switch {
	case s.lost != "":
		return s.lost
	case s.needsReset != "":
		return s.needsReset
	case s.broken:
		return s.why
	}
	return ""
}

// line is row i's characters and the style of each: a wide character once,
// trailing blanks cut.
func (s *textScreen) line(i int) ([]rune, []paneCell) {
	runes, styles := make([]rune, 0, s.cols), make([]paneCell, 0, s.cols)
	for c, r := range s.grid[i] {
		if r == screenTail {
			continue
		}
		runes, styles = append(runes, r), append(styles, s.style[i][c])
	}
	n := len(runes)
	for n > 0 && runes[n-1] == ' ' {
		n--
	}
	return runes[:n], styles[:n]
}

// Text is the screen as lines, trailing spaces cut, no final newline.
func (s *textScreen) Text() string {
	text, _ := s.styledText()
	return text
}

// Bracketed says whether the agent turned bracketed paste on.
func (s *textScreen) Bracketed() bool { return s.bracketed }

// styledText is Text and the style of each of its runes, lined up as
// paneUnstyle gives them.
func (s *textScreen) styledText() (string, []paneCell) {
	var text []rune
	var styles []paneCell
	for i := range s.grid {
		runes, cells := s.line(i)
		text, styles = append(text, runes...), append(styles, cells...)
		if i < s.rows-1 {
			text, styles = append(text, '\n'), append(styles, 0) // the newline
		}
	}
	return string(text), styles
}

// PaneState reads the screen like a tmux capture taken with its escapes (a box
// holding only a faint suggestion is idle); Unknown reads PaneUnknown.
func (s *textScreen) PaneState() PaneState {
	if s.Unknown() {
		return PaneUnknown
	}
	text, styles := s.styledText()
	if s.styleUnsure {
		styles = nil
	}
	return readPaneState(text, styles)
}

// Resize changes the window's size, of both pages. Fewer rows push the top
// lines off when the cursor is at the bottom; more rows and columns add
// blanks. Terminals differ on what a resize does to the lines (xterm and
// Windows Terminal pull lines back from the scrollback as the window grows,
// conhost and Windows Terminal rewrap wrapped lines, and they keep different
// lines as it shrinks), so any change breaks both pages until the next full
// redraw, and a saved cursor, which may have moved with the lines, is unsure.
// A scroll region of part of the screen is of unknown effect: xterm sets it
// back to the whole screen, and conhost (as read, not run here) keeps it.
func (s *textScreen) Resize(rows, cols int) {
	rows, cols = max(rows, 1), max(cols, 1)
	if rows == s.rows && cols == s.cols {
		return
	}
	if rows > screenMaxSize || cols > screenMaxSize {
		s.lost = "a window over 1000 rows or columns"
		return
	}
	if s.top != 0 || s.bottom != s.rows-1 {
		s.breaks("a resized window with a scroll region")
	}
	s.screenPage.resize(s.rows, rows, s.cols, cols)
	if s.main != nil {
		s.main.resize(s.rows, rows, s.cols, cols)
	}
	s.rows, s.cols = rows, cols
	s.top, s.bottom = 0, rows-1
}

func (p *screenPage) resize(oldRows, rows, oldCols, cols int) {
	if cols < oldCols {
		p.breakCells("a narrower window")
	}
	p.breakCells("a resized window")
	p.saved.unsure = p.saved.unsure || p.saved.set
	if rows < oldRows {
		drop := max(p.row-rows+1, 0)
		p.grid, p.style = p.grid[drop:drop+rows], p.style[drop:drop+rows]
		p.row -= drop
	}
	for len(p.grid) < rows {
		p.grid = append(p.grid, blankRow(oldCols))
		p.style = append(p.style, make([]paneCell, oldCols))
	}
	for i, r := range p.grid {
		if cols < len(r) {
			p.grid[i], p.style[i] = r[:cols], p.style[i][:cols]
		}
		for len(p.grid[i]) < cols {
			p.grid[i] = append(p.grid[i], ' ')
			p.style[i] = append(p.style[i], 0)
		}
	}
	p.dirty = make([]bool, rows)
	for i := range p.dirty {
		p.dirty[i] = true
	}
	p.row, p.col = min(p.row, rows-1), min(p.col, cols-1)
	p.saved.row, p.saved.col = min(p.saved.row, rows-1), min(p.saved.col, cols-1)
	p.wrap, p.wrapUnsure = false, false
}

// Write draws the launcher's output. It always takes all of p.
func (s *textScreen) Write(p []byte) (int, error) {
	data := p
	if len(s.pending) > 0 {
		data = append(s.pending, p...)
		s.pending = nil
	}
	for len(data) > 0 {
		b := data[0]
		if b >= utf8.RuneSelf && s.mode == modeGround {
			if !utf8.FullRune(data) {
				s.pending = append([]byte(nil), data...)
				break
			}
			r, n := utf8.DecodeRune(data)
			if r == utf8.RuneError {
				s.breakCells("bad UTF-8")
			} else {
				s.put(r)
			}
			data = data[n:]
			continue
		}
		s.feed(b)
		data = data[1:]
	}
	return len(p), nil
}

// byteName is a byte of a sequence as the log shows it: itself if printable, else in hex.
func byteName(b byte) string {
	if b > 0x20 && b < 0x7f {
		return string(rune(b))
	}
	return fmt.Sprintf("0x%02x", b)
}

// csiName is the CSI sequence being read, by its own bytes (numbers and marks only).
func (s *textScreen) csiName(final byte) string {
	return "CSI " + string(s.params[:min(len(s.params), 24)]) + byteName(final)
}

func (s *textScreen) feed(b byte) {
	switch s.mode {
	case modeEsc:
		s.mode = modeGround
		switch b {
		case '[':
			s.mode, s.params = modeCSI, s.params[:0]
		case ']', 'P', 'X', '^', '_': // OSC, DCS, SOS, PM, APC: strings (stringEnded)
			s.mode, s.osc, s.oscC2, s.oscBel = modeOSC, 0, false, b == ']'
			s.str, s.strHead, s.strCtl = b, s.strHead[:0], false
		case '(':
			s.mode = modeCharset
		case ')':
			s.mode = modeCharset1
		case '7': // the cursor saved (DECSC): on the list, see above
			s.saveCursor()
		case '8': // restored (DECRC): on the list, but with a region set or another style saved (see above)
			region, style := s.top != 0 || s.bottom != s.rows-1, s.saved.pen != s.pen
			s.restoreCursor()
			switch {
			case region:
				s.breakCells("ESC 8 with a scroll region set")
			case style:
				s.breakCells("ESC 8 with another style saved")
			}
		case 'D': // index, next line, reverse index: off the list, cells and the cursor only
			s.breakCells("ESC D")
			s.endWrapUnsure()
			s.lineFeed()
		case 'E':
			s.breakCells("ESC E")
			s.col = 0
			s.endWrap()
			s.lineFeed()
		case 'M':
			s.breakCells("ESC M")
			s.endWrapUnsure()
			s.reverseIndex()
		case 'c': // a full reset: everything is known again, but a size that never was
			lost := s.lost
			s.Reset(s.rows, s.cols)
			s.lost = cmp.Or(s.lost, lost)
		default: // the keypad modes (ESC = >), HTS (ESC H), DECALN (ESC # 8), a stray ST and the rest: of unknown effect
			s.breaks("ESC " + byteName(b))
		}
	case modeCharset, modeCharset1:
		// ESC ( B and ESC ) B choose plain ASCII, which is what the screen
		// draws. Another set (as 0, line drawing) changes what the bytes
		// mean: followed as a mode, so the page stays unknown until ASCII is
		// chosen again and the page redrawn. A control byte or a byte past
		// ASCII there (an ESC, the start of a C1 control) acts or starts a
		// new sequence on a terminal: of unknown effect.
		g, name := modeCharsetG0, "ESC ( "
		if s.mode != modeCharset {
			g, name = modeCharsetG1, "ESC ) "
		}
		s.mode = modeGround
		if b < 0x20 || b >= 0x7f {
			s.breaks(name + byteName(b))
			return
		}
		s.oddMode(g, b != 'B', name+byteName(b))
	case modeCSI:
		if b >= 0x20 && b <= 0x3f {
			if len(s.params) < screenMaxCSI {
				s.params = append(s.params, b)
			} else {
				s.breaks("a CSI over 64 bytes")
			}
			return
		}
		s.mode = modeGround
		if b < 0x40 || b > 0x7e {
			s.breaks("a CSI cut by " + byteName(b))
			return
		}
		s.csi(b)
	case modeOSC, modeOSCEsc:
		if s.osc++; s.osc > screenMaxOSC {
			s.breaks("a string over 4096 bytes")
			s.mode = modeGround
			return
		}
		// CAN, SUB and the C1 controls (C2 80..9F, which include the string
		// terminator U+009C) end the string on a real terminal, which then
		// draws what follows. The screen can't follow that: it reads unknown.
		// A lone C2 is fine (C2 B7 and C2 A0 are common in titles).
		c1 := s.oscC2 && b >= 0x80 && b <= 0x9f
		s.oscC2 = b == 0xc2
		switch {
		case b == 0x18 || b == 0x1a || c1:
			s.breaks("a string ended by CAN, SUB or a C1 control")
			s.mode = modeGround
		case (b == 0x07 && s.oscBel) || (s.mode == modeOSCEsc && b == '\\'):
			s.mode = modeGround
			s.stringEnded(true)
		case s.mode == modeOSCEsc: // cut off by another sequence, as xterm does
			s.stringEnded(false)
			s.mode = modeEsc
			s.feed(b)
		case b == 0x1b:
			s.mode = modeOSCEsc
		default:
			s.strCtl = s.strCtl || b < 0x20 || b == 0x7f
			if len(s.strHead) < screenMaxHead {
				s.strHead = append(s.strHead, b)
			}
		}
	default:
		s.control(b)
	}
}

func (s *textScreen) control(b byte) {
	switch {
	case b == 0x1b:
		s.mode = modeEsc
	case b == '\r':
		s.col = 0
		s.endWrap()
	case b == '\n':
		s.endWrapUnsure()
		s.lineFeed()
	case b == '\b':
		// A backspace at the first column may go up to the end of the line
		// above on a console; after the last column, where it lands differs.
		if s.col == 0 || s.wrap || s.wrapUnsure {
			s.breakCells("a backspace at the first column or after the last")
		}
		s.col = max(s.col-1, 0)
		s.endWrap()
	case b == '\t':
		// A tab moves the cursor to the next stop (every 8 columns: what
		// changes them is of unknown effect). An older console writes blanks
		// up to it instead, and where a tab goes from the last stop or after
		// the last column differs: unsure unless it passes blanks only, to a
		// stop inside the window.
		stop := (s.col/8 + 1) * 8
		if s.wrap || s.wrapUnsure || stop > s.cols-1 || !s.blank(s.row, s.col, stop) {
			s.breakCells("a tab over text or past the last stop")
		}
		s.col = min(stop, s.cols-1)
		s.endWrap()
	case b < 0x20 || b == 0x7f:
		// The other C0 controls are off the list. None moves anything a
		// terminal keeps through a redraw: SO and SI shift to G1, which is
		// ASCII unless ESC ) said otherwise (followed as a mode). A console
		// may draw one (NUL, DEL) as a character, take VT and FF as line
		// feeds, or ring the bell (BEL), which xterm ignores or does otherwise.
		s.breakCells("control byte " + byteName(b))
	default:
		s.put(rune(b))
	}
}

// endWrap: the cursor was put, so no deferred wrap is pending, on any terminal.
func (s *textScreen) endWrap() { s.wrap, s.wrapUnsure = false, false }

// endWrapUnsure: what came may or may not end a pending deferred wrap,
// depending on the terminal; the cursor didn't move along the row.
func (s *textScreen) endWrapUnsure() {
	s.wrapUnsure = s.wrapUnsure || s.wrap
	s.wrap = false
}

// blank: cells from..to-1 of a row hold nothing.
func (s *textScreen) blank(row, from, to int) bool {
	for _, r := range s.grid[row][from:min(to, s.cols)] {
		if r != ' ' {
			return false
		}
	}
	return true
}

// lineFeed moves down a row; at the scroll region's bottom, the region
// scrolls up. At the last row below a region, xterm stays put and a console
// may scroll: unsure.
func (s *textScreen) lineFeed() {
	switch {
	case s.row == s.bottom:
		s.scrollUp(s.top, s.bottom, 1)
	case s.row < s.rows-1:
		s.row++
	default:
		s.breakCells("a line feed at the last row, below the scroll region")
	}
}

// reverseIndex moves up a row; at the scroll region's top, the region
// scrolls down. At the first row above a region, unsure, as for lineFeed.
func (s *textScreen) reverseIndex() {
	switch {
	case s.row == s.top:
		s.scrollDown(s.top, s.bottom, 1)
	case s.row > 0:
		s.row--
	default:
		s.breakCells("a reverse index at the first row, above the scroll region")
	}
}

// rotate turns rows lo..hi (inclusive) up by n: row lo+n becomes row lo, and
// the top n rows come round to the bottom. It moves row slices only.
func (s *textScreen) rotate(lo, hi, n int) {
	for _, r := range [][2]int{{lo, lo + n}, {lo + n, hi + 1}, {lo, hi + 1}} {
		slices.Reverse(s.grid[r[0]:r[1]])
		slices.Reverse(s.style[r[0]:r[1]])
		slices.Reverse(s.dirty[r[0]:r[1]])
	}
}

// scrollUp moves rows top..bottom up by n, blanking the n rows at the bottom.
func (s *textScreen) scrollUp(top, bottom, n int) {
	n = min(max(n, 1), bottom-top+1)
	s.rotate(top, bottom, n%(bottom-top+1))
	for i := bottom - n + 1; i <= bottom; i++ {
		s.fill(i, 0, s.cols)
	}
}

// scrollDown moves rows top..bottom down by n, blanking the n rows at the top.
func (s *textScreen) scrollDown(top, bottom, n int) {
	n = min(max(n, 1), bottom-top+1)
	s.rotate(top, bottom, (bottom-top+1-n)%(bottom-top+1))
	for i := top; i < top+n; i++ {
		s.fill(i, 0, s.cols)
	}
}

// fill blanks cells from..to of a row, in place. An edge that cuts a wide
// character in two blanks the other half too, as the screen draws it, but
// terminals differ there: the page breaks until a full redraw.
func (p *screenPage) fill(row, from, to int) {
	if !p.dirty[row] {
		return
	}
	g := p.grid[row]
	if (from > 0 && from < len(g) && g[from] == screenTail) || (to > 0 && to < len(g) && g[to] == screenTail) {
		p.breakCells("an erase through half a wide character")
	}
	if from > 0 && from < len(g) && g[from] == screenTail {
		from--
	}
	if to > 0 && to < len(g) && g[to] == screenTail {
		to++
	}
	for i := from; i < to; i++ {
		g[i] = ' '
		p.style[row][i] = 0
	}
	if from == 0 && to == len(g) {
		p.dirty[row] = false
	}
}

// screenGlyphs: Claude Code's own glyphs on the allow-list, each East Asian
// Neutral and not an emoji, so one cell in xterm's, conhost's and Windows
// Terminal's width tables alike, whatever the font: the no-break space after
// ❯, ⎿ (a tool's result), ⏵ (the footer's mode), ╌ (a dialog's dashed rule),
// the logo's ▐ ▛ ▜ ▝ and ▟ (2.1.329), the onboarding banner's light shade ░
// (2.1.329), the spinner's ✢ ✶ ✻, and ❯ (the box's prompt). ░'s neighbours
// ▒ ▓ and the blocks ▀..▏ are Ambiguous instead, so in screenAmbiguous.
var screenGlyphs = map[rune]bool{
	0x00a0: true, 0x23bf: true, 0x23f5: true, 0x254c: true, 0x2590: true, 0x2591: true, 0x259b: true, 0x259c: true,
	0x259d: true, 0x259f: true, 0x2722: true, 0x2736: true, 0x273b: true, 0x276f: true,
}

// screenFrenchSpacing: what a reply in proper French typography holds besides
// letters and Ambiguous punctuation, each East Asian Neutral (Unicode 16.0's
// EastAsianWidth.txt) and not an emoji, so one cell in xterm's wcwidth and in
// conhost's width table on any code page, as screenGlyphs are. The no-break
// space (U+00A0, before a colon) is in screenGlyphs already (FXFRENCHSPACES).
var screenFrenchSpacing = map[rune]bool{
	0x202f: true, // the narrow no-break space, before ? ! ; and inside « »: N, Zs
	0x2009: true, // the thin space, used for the same: N, Zs
	0x00ab: true, // «: N, Pi (not Ambiguous, unlike the curly quotes)
	0x00bb: true, // »: N, Pf
}

// screenAmbiguous: the East Asian Ambiguous characters (A) in the BMP, one
// cell on a code page that isn't a CJK one (see the allow-list). Source:
// Unicode 16.0's EastAsianWidth.txt (as Python 3.14's unicodedata has it),
// less the combining marks (Mn: U+0300..036F, the variation selectors
// U+FE00..FE0F), the soft hyphen (Cf) and the private use area (Co), which
// are off the list whatever the code page. It holds Claude Code's · — … ← ↓ ─
// ▀ █ ◐ ✽, and é, à, ê, è, œ, ’ and “ ”. U+FFFD is in it but never drawn:
// Write can't tell it from bad UTF-8, so it breaks the page as that.
var screenAmbiguous = &unicode.RangeTable{R16: []unicode.Range16{
	{0x00a1, 0x00a1, 1}, {0x00a4, 0x00a4, 1}, {0x00a7, 0x00a8, 1}, {0x00aa, 0x00aa, 1}, {0x00ae, 0x00ae, 1}, {0x00b0, 0x00b4, 1},
	{0x00b6, 0x00ba, 1}, {0x00bc, 0x00bf, 1}, {0x00c6, 0x00c6, 1}, {0x00d0, 0x00d0, 1}, {0x00d7, 0x00d8, 1}, {0x00de, 0x00e1, 1},
	{0x00e6, 0x00e6, 1}, {0x00e8, 0x00ea, 1}, {0x00ec, 0x00ed, 1}, {0x00f0, 0x00f0, 1}, {0x00f2, 0x00f3, 1}, {0x00f7, 0x00fa, 1},
	{0x00fc, 0x00fc, 1}, {0x00fe, 0x00fe, 1}, {0x0101, 0x0101, 1}, {0x0111, 0x0111, 1}, {0x0113, 0x0113, 1}, {0x011b, 0x011b, 1},
	{0x0126, 0x0127, 1}, {0x012b, 0x012b, 1}, {0x0131, 0x0133, 1}, {0x0138, 0x0138, 1}, {0x013f, 0x0142, 1}, {0x0144, 0x0144, 1},
	{0x0148, 0x014b, 1}, {0x014d, 0x014d, 1}, {0x0152, 0x0153, 1}, {0x0166, 0x0167, 1}, {0x016b, 0x016b, 1}, {0x01ce, 0x01ce, 1},
	{0x01d0, 0x01d0, 1}, {0x01d2, 0x01d2, 1}, {0x01d4, 0x01d4, 1}, {0x01d6, 0x01d6, 1}, {0x01d8, 0x01d8, 1}, {0x01da, 0x01da, 1},
	{0x01dc, 0x01dc, 1}, {0x0251, 0x0251, 1}, {0x0261, 0x0261, 1}, {0x02c4, 0x02c4, 1}, {0x02c7, 0x02c7, 1}, {0x02c9, 0x02cb, 1},
	{0x02cd, 0x02cd, 1}, {0x02d0, 0x02d0, 1}, {0x02d8, 0x02db, 1}, {0x02dd, 0x02dd, 1}, {0x02df, 0x02df, 1}, {0x0391, 0x03a1, 1},
	{0x03a3, 0x03a9, 1}, {0x03b1, 0x03c1, 1}, {0x03c3, 0x03c9, 1}, {0x0401, 0x0401, 1}, {0x0410, 0x044f, 1}, {0x0451, 0x0451, 1},
	{0x2010, 0x2010, 1}, {0x2013, 0x2016, 1}, {0x2018, 0x2019, 1}, {0x201c, 0x201d, 1}, {0x2020, 0x2022, 1}, {0x2024, 0x2027, 1},
	{0x2030, 0x2030, 1}, {0x2032, 0x2033, 1}, {0x2035, 0x2035, 1}, {0x203b, 0x203b, 1}, {0x203e, 0x203e, 1}, {0x2074, 0x2074, 1},
	{0x207f, 0x207f, 1}, {0x2081, 0x2084, 1}, {0x20ac, 0x20ac, 1}, {0x2103, 0x2103, 1}, {0x2105, 0x2105, 1}, {0x2109, 0x2109, 1},
	{0x2113, 0x2113, 1}, {0x2116, 0x2116, 1}, {0x2121, 0x2122, 1}, {0x2126, 0x2126, 1}, {0x212b, 0x212b, 1}, {0x2153, 0x2154, 1},
	{0x215b, 0x215e, 1}, {0x2160, 0x216b, 1}, {0x2170, 0x2179, 1}, {0x2189, 0x2189, 1}, {0x2190, 0x2199, 1}, {0x21b8, 0x21b9, 1},
	{0x21d2, 0x21d2, 1}, {0x21d4, 0x21d4, 1}, {0x21e7, 0x21e7, 1}, {0x2200, 0x2200, 1}, {0x2202, 0x2203, 1}, {0x2207, 0x2208, 1},
	{0x220b, 0x220b, 1}, {0x220f, 0x220f, 1}, {0x2211, 0x2211, 1}, {0x2215, 0x2215, 1}, {0x221a, 0x221a, 1}, {0x221d, 0x2220, 1},
	{0x2223, 0x2223, 1}, {0x2225, 0x2225, 1}, {0x2227, 0x222c, 1}, {0x222e, 0x222e, 1}, {0x2234, 0x2237, 1}, {0x223c, 0x223d, 1},
	{0x2248, 0x2248, 1}, {0x224c, 0x224c, 1}, {0x2252, 0x2252, 1}, {0x2260, 0x2261, 1}, {0x2264, 0x2267, 1}, {0x226a, 0x226b, 1},
	{0x226e, 0x226f, 1}, {0x2282, 0x2283, 1}, {0x2286, 0x2287, 1}, {0x2295, 0x2295, 1}, {0x2299, 0x2299, 1}, {0x22a5, 0x22a5, 1},
	{0x22bf, 0x22bf, 1}, {0x2312, 0x2312, 1}, {0x2460, 0x24e9, 1}, {0x24eb, 0x254b, 1}, {0x2550, 0x2573, 1}, {0x2580, 0x258f, 1},
	{0x2592, 0x2595, 1}, {0x25a0, 0x25a1, 1}, {0x25a3, 0x25a9, 1}, {0x25b2, 0x25b3, 1}, {0x25b6, 0x25b7, 1}, {0x25bc, 0x25bd, 1},
	{0x25c0, 0x25c1, 1}, {0x25c6, 0x25c8, 1}, {0x25cb, 0x25cb, 1}, {0x25ce, 0x25d1, 1}, {0x25e2, 0x25e5, 1}, {0x25ef, 0x25ef, 1},
	{0x2605, 0x2606, 1}, {0x2609, 0x2609, 1}, {0x260e, 0x260f, 1}, {0x261c, 0x261c, 1}, {0x261e, 0x261e, 1}, {0x2640, 0x2640, 1},
	{0x2642, 0x2642, 1}, {0x2660, 0x2661, 1}, {0x2663, 0x2665, 1}, {0x2667, 0x266a, 1}, {0x266c, 0x266d, 1}, {0x266f, 0x266f, 1},
	{0x269e, 0x269f, 1}, {0x26bf, 0x26bf, 1}, {0x26c6, 0x26cd, 1}, {0x26cf, 0x26d3, 1}, {0x26d5, 0x26e1, 1}, {0x26e3, 0x26e3, 1},
	{0x26e8, 0x26e9, 1}, {0x26eb, 0x26f1, 1}, {0x26f4, 0x26f4, 1}, {0x26f6, 0x26f9, 1}, {0x26fb, 0x26fc, 1}, {0x26fe, 0x26ff, 1},
	{0x273d, 0x273d, 1}, {0x2776, 0x277f, 1}, {0x2b56, 0x2b59, 1}, {0x3248, 0x324f, 1}, {0xfffd, 0xfffd, 1},
}}

// screenTextEmoji: the emoji whose default is text (Emoji=Yes and
// Emoji_Presentation=No) in the BMP that are East Asian Neutral, so not in
// screenAmbiguous or screenCJK, taken as Ambiguous characters (FXWINSCREENR4).
// Source: Unicode 16.0's emoji-data.txt, as Swift's Unicode.Scalar.Properties
// (isEmoji, isEmojiPresentation) has it. Emoji_Presentation=Yes characters are
// all East Asian Wide since Unicode 9 and in neither table, so they break the
// page, as VS16 (U+FE0F, a mark) after any of these does. It holds Claude
// Code's ⏸ ⏺ ✳, and ⚠ © ‼ ✔ ❤ ➡.
var screenTextEmoji = &unicode.RangeTable{R16: []unicode.Range16{
	{0x00a9, 0x00a9, 1}, {0x203c, 0x203c, 1}, {0x2049, 0x2049, 1}, {0x2139, 0x2139, 1}, {0x21a9, 0x21aa, 1}, {0x2328, 0x2328, 1},
	{0x23cf, 0x23cf, 1}, {0x23ed, 0x23ef, 1}, {0x23f1, 0x23f2, 1}, {0x23f8, 0x23fa, 1}, {0x25aa, 0x25ab, 1}, {0x25fb, 0x25fc, 1},
	{0x2600, 0x2604, 1}, {0x2611, 0x2611, 1}, {0x2618, 0x2618, 1}, {0x261d, 0x261d, 1}, {0x2620, 0x2620, 1}, {0x2622, 0x2623, 1},
	{0x2626, 0x2626, 1}, {0x262a, 0x262a, 1}, {0x262e, 0x262f, 1}, {0x2638, 0x263a, 1}, {0x265f, 0x265f, 1}, {0x2666, 0x2666, 1},
	{0x267b, 0x267b, 1}, {0x267e, 0x267e, 1}, {0x2692, 0x2692, 1}, {0x2694, 0x2697, 1}, {0x2699, 0x2699, 1}, {0x269b, 0x269c, 1},
	{0x26a0, 0x26a0, 1}, {0x26a7, 0x26a7, 1}, {0x26b0, 0x26b1, 1}, {0x2702, 0x2702, 1}, {0x2708, 0x2709, 1}, {0x270c, 0x270d, 1},
	{0x270f, 0x270f, 1}, {0x2712, 0x2712, 1}, {0x2714, 0x2714, 1}, {0x2716, 0x2716, 1}, {0x271d, 0x271d, 1}, {0x2721, 0x2721, 1},
	{0x2733, 0x2734, 1}, {0x2744, 0x2744, 1}, {0x2747, 0x2747, 1}, {0x2763, 0x2764, 1}, {0x27a1, 0x27a1, 1}, {0x2934, 0x2935, 1},
	{0x2b05, 0x2b07, 1},
}}

// screenAccentedLatin: an accented Latin letter (in Latin-1 Supplement, Latin
// Extended-A and -B, and Latin Extended Additional), taken as an Ambiguous
// character: one cell on a code page that isn't a CJK one. Many are Ambiguous
// already (é à ê è); the Neutral ones (ç ï ô û â ë î, the capitals) are one
// cell in every width table, and are kept to the same rule so that what a
// reply in French holds reads alike.
func screenAccentedLatin(r rune) bool {
	return (r >= 0xc0 && r <= 0x24f || r >= 0x1e00 && r <= 0x1eff) && unicode.IsLetter(r)
}

// screenOneCellOffCJK: a character one cell wide on a code page that isn't a
// CJK one (screenCodePage), and of unsure width on one that is.
func screenOneCellOffCJK(r rune) bool {
	return unicode.Is(screenAmbiguous, r) || unicode.Is(screenTextEmoji, r) || screenAccentedLatin(r)
}

// screenCJK: East Asian Wide and Fullwidth characters in the BMP that are two
// cells on every terminal: CJK punctuation (not the marks U+302A..302F),
// kana (not the marks U+3099, U+309A), the CJK ideographs, Hangul syllables,
// compatibility ideographs and the fullwidth forms, as Unicode had them by
// 2016, which conhost's first VT width table holds.
func screenCJK(r rune) bool {
	return (r >= 0x3000 && r <= 0x3029) || (r >= 0x3030 && r <= 0x303e) ||
		(r >= 0x3041 && r <= 0x3096) || (r >= 0x309b && r <= 0x30ff) ||
		(r >= 0x3400 && r <= 0x4db5) || (r >= 0x4e00 && r <= 0x9fd5) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfa6d) || (r >= 0xfa70 && r <= 0xfad9) ||
		(r >= 0xff01 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6)
}

// width is the cells a character on the allow-list takes; 0 for one off it.
func (s *textScreen) width(r rune) int {
	switch {
	case r >= 0x20 && r < 0x7f, screenGlyphs[r], screenFrenchSpacing[r]:
		return 1
	case screenCJK(r):
		return 2
	case screenOneCellOffCJK(r) && !s.page.cjk():
		return 1
	}
	return 0
}

func (s *textScreen) put(r rune) {
	if unicode.Is(unicode.Cc, r) {
		// A C1 control (U+0080..U+009F) sent as UTF-8: xterm acts on it (CSI,
		// a tab stop, a string…), a console may not. Of unknown effect.
		s.breaks(fmt.Sprintf("a C1 control U+%04X", r))
		return
	}
	width := s.width(r)
	if width == 0 {
		if screenOneCellOffCJK(r) {
			s.breakCells(s.page.ambiguousWhy())
		} else {
			s.breakCells("a character of unsure width")
		}
		return
	}
	if s.wrapUnsure {
		s.breakCells("a character after the last column, where terminals differ")
		s.wrapUnsure = false
	}
	if width > s.cols {
		s.breakCells("a wide character in a one-column window")
		return
	}
	s.erased = false // drawn before the cursor was put at a known place
	if s.wrap || s.col+width > s.cols {
		if !s.wrap { // a wide character that doesn't fit at the end of a row: off the list
			s.breakCells("a wide character at the last column")
		}
		s.col, s.wrap = 0, false
		s.lineFeed()
	}
	s.overwrite(s.row, s.col, width)
	s.grid[s.row][s.col] = r
	s.style[s.row][s.col] = s.pen
	if width == 2 {
		s.grid[s.row][s.col+1] = screenTail
		s.style[s.row][s.col+1] = s.pen
	}
	s.dirty[s.row] = true
	if s.col+width >= s.cols {
		s.col, s.wrap = s.cols-1, true
	} else {
		s.col += width
	}
}

// overwrite clears the other half of a wide character that cells
// col..col+n-1 of a row cut; terminals differ on what that half shows, so
// the page breaks until a full redraw.
func (s *textScreen) overwrite(row, col, n int) {
	g := s.grid[row]
	cut := func(i int) {
		g[i], s.style[row][i] = ' ', 0
		s.breakCells("a character over half a wide character")
	}
	if g[col] == screenTail && col > 0 {
		cut(col - 1)
	}
	if end := col + n; end < len(g) && g[end] == screenTail {
		cut(end)
	}
}

// csiParams reads "1;2;3" (no prefix) into numbers; a missing one is 0. ok is
// false for anything else (a ":" subparameter, an intermediate byte).
func csiParams(nums []int, p []byte) ([]int, bool) {
	if len(p) == 0 {
		return nums, true
	}
	n, have := 0, false
	for _, c := range p {
		switch {
		case c >= '0' && c <= '9':
			n, have = min(n*10+int(c-'0'), screenMaxParam), true
		case c == ';':
			nums, n, have = append(nums, n), 0, false
		default:
			return nil, false
		}
	}
	return append(nums, n), have || len(nums) > 0
}

// colonSGR: an SGR with ":" subparameters, as ESC[38:2::12:34:56m (a
// colour), which csiParams can't read; only digits, ";" and ":" are allowed,
// so nothing else rides along. On the list, as any SGR.
func colonSGR(params []byte) bool {
	for _, c := range params {
		if !(c >= '0' && c <= '9') && c != ';' && c != ':' {
			return false
		}
	}
	return len(params) > 0 && strings.IndexByte(string(params), ':') >= 0
}

// cursorShape: DECSCUSR, ESC[0 q, the cursor's shape (a digit or two, a
// space, "q"), which ConPTY may send as it starts.
func cursorShape(params []byte) bool {
	n := len(params)
	if n < 1 || n > 3 || params[n-1] != ' ' {
		return false
	}
	for _, c := range params[:n-1] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *textScreen) csi(final byte) {
	if p := s.params; len(p) > 0 && strings.IndexByte("?<=>", p[0]) >= 0 {
		s.csiPrivate(p[0], string(p[1:]), final)
		return
	}
	if final == 'm' {
		if pen, ok := paneSGR(string(s.params), s.pen); ok {
			s.pen = pen
		} else {
			s.styleUnsure = true
		}
		if colonSGR(s.params) {
			return
		}
	}
	if final == 'q' && cursorShape(s.params) { // off the list, no cell: until a full redraw
		s.breakCells(s.csiName(final))
		return
	}
	nums, ok := csiParams(s.nums[:0], s.params)
	s.nums = nums
	if !ok {
		s.breaks(s.csiName(final))
		return
	}
	arg := func(i, def int) int {
		if i < len(nums) && nums[i] > 0 {
			return nums[i]
		}
		return def
	}
	switch final {
	case 'm': // colours and styles move nothing, so keep a pending wrap
		return
	case 'n', 'c':
		// DA1 asked exactly as Claude Code asks it (CSI c, CSI 0c) is on
		// the list (see above). The reports ConPTY asks of the terminal as
		// it starts (DSR 5n, 6n), and DA1 spelled otherwise (CSI 00c): off
		// the list, but known to draw nothing, so until a full redraw. Any
		// other is of unknown effect.
		if final == 'c' && (len(s.params) == 0 || string(s.params) == "0") {
			return // DA1 asked as Claude Code asks it: on the list, see above
		}
		if len(nums) <= 1 && (final == 'n' && (arg(0, 0) == 5 || arg(0, 0) == 6) || final == 'c' && arg(0, 0) == 0) {
			s.breakCells(s.csiName(final))
		} else {
			s.breaks(s.csiName(final))
		}
		return
	case 't':
		s.windowOp(nums, final)
		return
	case 'h', 'l':
		// ANSI modes: insert (4) and new line (20) are followed. Any other
		// (keyboard lock 2, control characters shown 3, local echo 12…) is of
		// unknown effect: not every terminal knows them, or does them alike.
		if len(nums) == 0 {
			s.breaks(s.csiName(final))
		}
		for _, n := range nums {
			switch n {
			case 4:
				s.oddMode(modeInsert, final == 'h', s.csiName(final))
			case 20:
				s.oddMode(modeNewline, final == 'h', s.csiName(final))
			default:
				s.breaks(s.csiName(final))
			}
		}
		return
	case 's':
		// Save the cursor (off the list). With parameters it sets left and
		// right margins where ?69 is on; with ?69 off, xterm saves the
		// cursor, and a console may not: the saved cursor is unsure.
		s.breakCells(s.csiName(final))
		s.saveCursor()
		s.saved.unsure = s.saved.unsure || len(nums) > 0
		return
	case 'u':
		if len(nums) == 0 {
			s.breakCells(s.csiName(final))
			s.restoreCursor()
			return
		}
	}
	wrapped := s.wrap || s.wrapUnsure
	switch final {
	case 'J', 'K', 'X', '@', 'P', 'S', 'T':
		s.endWrapUnsure() // they don't move the cursor; whether a pending wrap survives them differs
	default:
		s.endWrap()
	}
	// The rows A and B (and E and F) stop at, inside the scroll region.
	upTo, downTo := 0, s.rows-1
	if s.row >= s.top {
		upTo = s.top
	}
	if s.row <= s.bottom {
		downTo = s.bottom
	}
	// On the list: a move with at most the numbers it takes, to a place
	// inside the screen (and for CUF and CUB, with no wrap pending).
	move := func(most int, inside bool) {
		if len(nums) > most || !inside {
			s.breakCells(s.csiName(final))
		}
	}
	switch final {
	case 'A':
		move(1, s.row-arg(0, 1) >= upTo)
		s.row = max(s.row-arg(0, 1), upTo)
	case 'B':
		move(1, s.row+arg(0, 1) <= downTo)
		s.row = min(s.row+arg(0, 1), downTo)
	case 'C':
		move(1, s.col+arg(0, 1) <= s.cols-1 && !wrapped)
		s.col = min(s.col+arg(0, 1), s.cols-1)
	case 'D':
		move(1, s.col-arg(0, 1) >= 0 && !wrapped)
		s.col = max(s.col-arg(0, 1), 0)
	case 'G':
		move(1, arg(0, 1) <= s.cols)
		s.col = min(arg(0, 1), s.cols) - 1
	case 'd':
		move(1, arg(0, 1) <= s.rows)
		s.row = min(arg(0, 1), s.rows) - 1
	case 'H', 'f':
		move(2, arg(0, 1) <= s.rows && arg(1, 1) <= s.cols)
		s.row, s.col = min(arg(0, 1), s.rows)-1, min(arg(1, 1), s.cols)-1
		if s.erased { // erased while broken, and now the cursor is at a known place
			s.redrawn()
		}
	case 'J', 'K': // an erase off the list (more numbers, another mode) may erase or not: nothing is erased here
		if len(nums) > 1 || arg(0, 0) > 3 || final == 'K' && arg(0, 0) > 2 {
			s.breakCells(s.csiName(final))
		} else if final == 'J' {
			s.eraseDisplay(arg(0, 0))
		} else {
			s.eraseLine(arg(0, 0))
		}
	case 'X':
		s.eraseChars(len(nums), arg(0, 1), final)
	case 'L', 'M': // insert or delete lines at the cursor, inside the scroll region: off the list
		s.breakCells(s.csiName(final))
		if s.row >= s.top && s.row <= s.bottom {
			if final == 'L' {
				s.scrollDown(s.row, s.bottom, arg(0, 1))
			} else {
				s.scrollUp(s.row, s.bottom, arg(0, 1))
			}
			s.col = 0
		}
	case 'S', 'T':
		if len(nums) > 1 { // CSI … T with more numbers is the old mouse highlight tracking, not a scroll
			s.breaks(s.csiName(final))
			return
		}
		s.scrollRegion(final == 'S', arg(0, 1), final)
	case '@', 'P':
		if len(nums) > 1 {
			s.breakCells(s.csiName(final))
		}
		s.shiftCells(final == '@', arg(0, 1))
	case 'E', 'F', '`': // next and previous line, column absolute: off the list
		s.breakCells(s.csiName(final))
		switch final {
		case 'E':
			s.row, s.col = min(s.row+arg(0, 1), downTo), 0
		case 'F':
			s.row, s.col = max(s.row-arg(0, 1), upTo), 0
		default:
			s.col = min(arg(0, 1), s.cols) - 1
		}
	case 'r':
		s.region(nums, final)
	case 'b', 'I', 'Z': // repeat, tab forward and back: they change only cells and the cursor
		s.breakCells(s.csiName(final))
	default: // of unknown effect (CSI g clears tab stops, for one)
		s.breaks(s.csiName(final))
	}
}

// eraseChars is ECH (CSI n X), which ConPTY sends for each run of blanks it
// repaints: n cells from the cursor blanked, no more than to the end of the
// row, the cursor not moved. On the list with at most one number:
//   - xterm's ctlseqs: "CSI Ps X  Erase Ps Character(s) (default = 1) (ECH)",
//     from the cursor and not past the right margin, which is the last column
//     unless DECLRMM is on (?69, of unknown effect here);
//   - conhost's adapter (AdaptDispatch::EraseCharacters, read, not run here):
//     n cells from the cursor filled with blanks in the current colours, cut
//     at the end of the row, the cursor left where it is; a count of 0 is 1.
//
// An edge that cuts a wide character in two is off the list (fill).
func (s *textScreen) eraseChars(numbers, n int, final byte) {
	if numbers > 1 {
		s.breakCells(s.csiName(final))
	}
	s.fill(s.row, s.col, min(s.col+n, s.cols))
}

// scrollRegion is SU (CSI n S, up is true) and SD (CSI n T), which ConPTY
// sends to scroll lines in above Claude Code's box: the scroll region's rows
// move up (or down) n rows, n blank rows come in at its bottom (or top), and
// the cursor stays. On the list with no region of part of the screen set and
// n no more than its rows:
//   - xterm's ctlseqs: "CSI Ps S  Scroll up Ps lines (default = 1) (SU)" and
//     "CSI Ps T  Scroll down Ps lines (default = 1) (SD)", inside the top and
//     bottom margins;
//   - conhost's adapter (AdaptDispatch::ScrollUp and ScrollDown through
//     _ScrollMovement, read, not run here): the rows between the margins (the
//     whole window when none is set) moved by n, the rows that come in blank
//     in the current colours, the cursor left where it is; 0 is 1.
//
// With a count past its rows, what either does isn't certain from what's read
// here: drawn as xterm draws it, the page breaks until a full redraw. So it
// does while a region of part of the screen is set, the cursor inside it or
// not: whether Windows 10's oldest conhost scrolls only inside the margins
// isn't known here, and neither ConPTY nor Claude Code is known to scroll
// inside one (RT126WINSCREENR4 N2).
func (s *textScreen) scrollRegion(up bool, n int, final byte) {
	if s.top != 0 || s.bottom != s.rows-1 || n > s.bottom-s.top+1 {
		s.breakCells(s.csiName(final))
	}
	if up {
		s.scrollUp(s.top, s.bottom, n)
	} else {
		s.scrollDown(s.top, s.bottom, n)
	}
}

// region is DECSTBM (CSI top;bottom r), the scroll region (neither ConPTY nor
// Claude Code is known to set one). On the list when it is valid,
// 1 <= top < bottom <= rows (CSI r alone is the whole screen): both set it and
// put the cursor home.
//   - xterm's ctlseqs: "CSI Ps ; Ps r  Set Scrolling Region [top;bottom]
//     (default = full size of window) (DECSTBM)"; xterm homes the cursor, as
//     a VT100 does;
//   - conhost's adapter (AdaptDispatch::SetTopBottomScrollingMargins, read,
//     not run here): a valid pair set as the margins and the cursor put home
//     (CursorPosition 1;1), as the VT100 does.
//
// With origin mode (followed, so the page is broken anyway) home is the
// region's top. A bottom past the screen is of unknown effect: xterm cuts it
// at the last row and sets it, and conhost ignores it all. One that isn't two
// rows apart at least is ignored by both, and the cursor stays on xterm;
// whether it does on conhost isn't known here, so the page breaks until the
// next full redraw. A region kept through a resize, or a switch between the
// screens, is of unknown effect (Resize, alternate).
func (s *textScreen) region(nums []int, final byte) {
	top, bottom := 1, s.rows
	if len(nums) > 0 && nums[0] > 0 {
		top = nums[0]
	}
	if len(nums) > 1 && nums[1] > 0 {
		bottom = nums[1]
	}
	switch {
	case len(nums) > 2 || bottom > s.rows:
		s.breaks(s.csiName(final))
	case top >= bottom:
		s.breakCells(s.csiName(final))
	default:
		s.top, s.bottom = top-1, bottom-1
		s.row, s.col = 0, 0
	}
}

// windowOp: CSI … t, the window operations. On the list: the window's size
// said at the screen's own size (8;rows;cols, both numbers given), which ConPTY
// sends as it starts and on each resize, after the launcher has followed the
// console's (launcher_windows.go); xterm ignores it or keeps the size it has,
// conhost resizes to the size it has. Any other draws nothing (a report, the
// title kept or put back) and is off the list, until a full redraw; one that
// resizes the window to a size not the screen's (8;rows;cols, which conhost
// follows and xterm ignores, or DECSLPP, 24 and up) is of unknown effect.
func (s *textScreen) windowOp(nums []int, final byte) {
	op := 0
	if len(nums) > 0 {
		op = nums[0]
	}
	if len(nums) == 3 && op == 8 && nums[1] == s.rows && nums[2] == s.cols {
		return
	}
	other := func(i, now int) bool { return i < len(nums) && nums[i] != 0 && nums[i] != now }
	if len(nums) > 3 || op >= 24 || (op == 8 && (other(1, s.rows) || other(2, s.cols))) {
		s.breaks(s.csiName(final))
		return
	}
	s.breakCells(s.csiName(final))
}

// shiftCells is ICH (CSI n @, insert is true): n blanks inserted at the
// cursor, pushing the rest of the row right, what goes past the end lost; or
// DCH (CSI n P): n cells deleted there, pulling the rest left, blanks coming
// in at the end. The cursor stays. On the list with at most one number:
//   - xterm's ctlseqs: "CSI Ps @  Insert Ps (Blank) Character(s) (default =
//     1) (ICH)" and "CSI Ps P  Delete Ps Character(s) (default = 1) (DCH)",
//     between the cursor and the right margin, the last column unless DECLRMM
//     is on (?69, of unknown effect here);
//   - conhost's adapter (AdaptDispatch::InsertCharacter and DeleteCharacter,
//     read, not run here): the rest of the row from the cursor shifted by n,
//     cut at the end of the row, blanks in the current colours, the cursor
//     left where it is; 0 is 1.
//
// A wide character cut in two (at the cursor, or pushed past the end) is
// blanked here, and terminals differ there: the page breaks until a full
// redraw.
func (s *textScreen) shiftCells(insert bool, n int) {
	row, col := s.row, s.col
	if !s.dirty[row] {
		return // a blank row stays blank
	}
	n = min(n, s.cols-col)
	g, st := s.grid[row], s.style[row]
	gap := col // where the blanks go
	if insert {
		copy(g[col+n:], g[col:s.cols-n])
		copy(st[col+n:], st[col:s.cols-n])
	} else {
		copy(g[col:], g[col+n:])
		copy(st[col:], st[col+n:])
		gap = s.cols - n
	}
	for i := gap; i < gap+n; i++ {
		g[i], st[i] = ' ', 0
	}
	for c, r := range g {
		if (r == screenTail && (c == 0 || !screenCJK(g[c-1]))) || (screenCJK(r) && (c+1 == len(g) || g[c+1] != screenTail)) {
			g[c] = ' '
			s.breakCells("a shift through half a wide character")
		}
	}
}

func (s *textScreen) eraseDisplay(mode int) {
	switch mode {
	case 0:
		s.eraseLine(0)
		for i := s.row + 1; i < s.rows; i++ {
			s.fill(i, 0, s.cols)
		}
	case 1:
		s.eraseLine(1)
		for i := 0; i < s.row; i++ {
			s.fill(i, 0, s.cols)
		}
	case 2:
		for i := range s.grid {
			s.fill(i, 0, s.cols)
		}
		// Every cell is known; the cursor may not be (it is once put at a known place).
		s.erased = s.broken
	}
	// 3 is the scrollback, which a capture doesn't show; others break the page (csi).
}

func (s *textScreen) eraseLine(mode int) {
	switch mode {
	case 0:
		s.fill(s.row, s.col, s.cols)
	case 1:
		s.fill(s.row, 0, s.col+1)
	case 2:
		s.fill(s.row, 0, s.cols)
	}
	// Others break the page (csi).
}

// saveCursor saves the cursor and the modes saved with it; saved while the
// page is broken, it may not be the terminal's.
func (s *textScreen) saveCursor() {
	s.saved = savedCursor{row: s.row, col: s.col, pen: s.pen, wrap: s.wrap, set: true, unsure: s.broken || s.wrapUnsure, odd: s.odd & savedModes}
}

// restoreCursor puts back the cursor saved and the modes saved with it
// (origin, the character sets); with none saved, the cursor goes home in the
// plain style and those modes go off, as xterm and conhost do. These break the
// page, as the terminal's cursor may be elsewhere: an unsure one; one saved
// after the last column (whether the deferred wrap comes back differs); and
// on the alternate screen, one it never saved (a console that keeps one
// saved cursor for both screens restores the main screen's).
func (s *textScreen) restoreCursor() {
	c := s.saved
	switch {
	case c.unsure:
		s.breakCells("a cursor restored that was saved while the screen couldn't be read")
	case !c.set && s.main != nil:
		s.breakCells("a cursor restored on the alternate screen, which saved none")
	case c.wrap:
		s.breakCells("a cursor restored that was saved after the last column")
	}
	s.row, s.col, s.pen, s.wrap, s.wrapUnsure = min(c.row, s.rows-1), min(c.col, s.cols-1), c.pen, c.wrap && c.set, false
	for _, m := range []screenModes{modeOrigin, modeCharsetG0, modeCharsetG1} {
		s.oddMode(m, c.odd&m != 0, "a cursor restored with its modes")
	}
}

// oddMode turns a mode the screen doesn't draw on or off. While it's on, the
// page is broken and no redraw brings it back; once it's off, what was drawn
// meanwhile may be wrong, so the page waits for the next full redraw.
func (s *textScreen) oddMode(mode screenModes, on bool, name string) {
	if on || s.odd&mode != 0 {
		s.breakCells(name)
	}
	if on {
		s.odd |= mode
	} else {
		s.odd &^= mode
	}
}

// alternate enters or leaves the alternate screen (CSI ?1049h, l):
//   - On the list: entering from the main page saves the cursor and gives a
//     blank page with the cursor where it was, known whenever the main one
//     was; leaving it puts the main page back as it was, and restores its
//     saved cursor. What broke the alternate page goes with it. A cursor
//     saved on the alternate page makes the main one's unsure (a console may
//     keep one for both).
//   - Off the list, until a full redraw: leaving while on the main page (both
//     restore the cursor, as drawn); entering again while on the alternate
//     page (it clears and saves its cursor; xterm leaves the cursor, conhost
//     takes the main page's; both saved cursors are unsure).
//   - Of unknown effect: either, while a followed mode or a scroll region is
//     set. Conhost keeps autowrap and the new-line mode (and on Windows 10
//     the margins) per screen buffer, xterm one of each for both screens.
func (s *textScreen) alternate(on bool) {
	if s.odd != 0 || s.top != 0 || s.bottom != s.rows-1 {
		s.breaks("CSI ?1049 with a mode or a scroll region set")
	}
	switch {
	case !on && s.main == nil:
		s.breakCells("CSI ?1049l on the main screen")
		s.restoreCursor()
	case !on:
		if s.saved.set {
			s.main.saved.unsure = true
		}
		s.screenPage, s.main = *s.main, nil
		s.restoreCursor()
	case s.main != nil:
		s.saveCursor()
		page := newScreenPage(s.rows, s.cols)
		page.row, page.col, page.saved, page.wrapUnsure = s.row, s.col, s.saved, s.wrap || s.wrapUnsure
		page.saved.unsure = true
		s.screenPage = page
		s.main.saved.unsure = s.main.saved.unsure || s.main.saved.set
		s.breakCells("CSI ?1049h on the alternate screen")
	default:
		s.saveCursor()
		main := s.screenPage
		s.main = &main
		s.screenPage = newScreenPage(s.rows, s.cols)
		s.row, s.col, s.wrapUnsure = main.row, main.col, main.wrap || main.wrapUnsure
		if main.broken { // the cursor came from a page that wasn't known: known once it's put somewhere
			s.broken, s.erased, s.why = true, true, main.why
		}
	}
}

// screenQuietModes: the private modes on the allow-list, known to draw
// nothing on xterm or conhost (see the list above): the cursor shown, focus
// reports, synchronized output, the mouse reports and the colour palette's
// updates. Bracketed paste (2004) and the alternate screen (1049) are apart.
var screenQuietModes = map[int]bool{
	25: true, 1004: true, 2026: true, 9: true, 1000: true, 1002: true, 1003: true, 1005: true, 1006: true,
	1015: true, 1016: true, 2031: true,
}

// csiPrivate: CSI ? ... h/l (modes), and the private CSIs ConPTY and Claude
// Code send as they start; any other is of unknown effect. The terminal's
// version asked exactly as Claude Code asks it (XTVERSION >q, >0q) and the
// kitty keyboard's flags asked (?u with no number, which Claude Code sends
// from 2.1.329 at least where it used to send >1u) are on the list (see
// above). The others are off the list but known to draw nothing, so they
// break the page until a full redraw: win32 input (?9001h, which conhost on
// Windows 11 takes and xterm and Windows 10's conhost ignore), a mode's state
// asked (DECRQM ?…$p), XTVERSION spelled otherwise (>00q, >1q), the kitty
// keyboard (>…u) and modifier keys (XTMODKEYS >…m). A ?u with a number in it
// (?0u, ?5u) is no question and stays of unknown effect.
func (s *textScreen) csiPrivate(prefix byte, params string, final byte) {
	name := "CSI " + string(prefix) + params[:min(len(params), 24)] + byteName(final)
	numbers := strings.Trim(params, "0123456789;") == ""
	switch {
	case prefix == '>' && final == 'q' && (params == "" || params == "0"),
		prefix == '?' && final == 'u' && params == "":
		return // XTVERSION and the kitty keyboard's flags asked: on the list, see above
	case prefix == '?' && (final == 'h' || final == 'l'):
		// Numbers, as a terminal reads them: ?01049l is 1049.
		on := final == 'h'
		nums, ok := csiParams(s.nums[:0], []byte(params))
		s.nums = nums
		if !ok || len(nums) == 0 { // a mode set with no number is no mode on the list
			s.breaks(name)
			return
		}
		for _, n := range nums {
			switch {
			case n == 1049:
				s.alternate(on)
			case n == 2004:
				s.bracketed = on
			case screenQuietModes[n]:
			case n == 6:
				// Set or reset, origin puts the cursor home on xterm and
				// conhost, whether or not it was on (RT126WINSCREENR4 B1): at
				// the region's top when on, row 1 when off. Drawn so, and the
				// page breaks every time, until a full redraw.
				s.oddMode(modeOrigin, on, name)
				s.breakCells(name)
				s.row, s.col = 0, 0
				if on {
					s.row = s.top
				}
				s.endWrap()
			case n == 7:
				s.oddMode(modeNoWrap, !on, name)
			case n == 9001:
				s.breakCells(name)
			default:
				// The cursor keys (1), 132 columns (3), reverse wraparound
				// (45, 1045), the old alternate screens (47, 1047), the cursor
				// saved by mode (1048), and every mode nobody listed.
				s.breaks(name)
			}
		}
		return
	case prefix == '?' && final == 'p' && strings.HasSuffix(params, "$") && strings.Trim(params[:len(params)-1], "0123456789;") == "",
		prefix == '>' && strings.IndexByte("qum", final) >= 0 && numbers:
		s.breakCells(name)
		return
	}
	s.breaks(name)
}

// stringEnded: a string (OSC, DCS, SOS, PM, APC) ended, at its terminator or
// cut off by another sequence (terminated false). An OSC known to draw
// nothing (screenQuietOSC, with no control byte in it) ended at its
// terminator is on the list; one cut off breaks the page until a full
// redraw. Any other string is of unknown effect: a DCS or APC may draw an
// image or restore modes where a terminal has it, and Windows 10's conhost,
// which has no DCS, SOS, PM or APC, prints the body.
func (s *textScreen) stringEnded(terminated bool) {
	head := string(s.strHead)
	n := strings.IndexFunc(head, func(r rune) bool { return r < '0' || r > '9' })
	if n < 0 {
		n = len(head)
	}
	if s.str == ']' && !s.strCtl && screenQuietOSC(head) {
		switch {
		case terminated:
		case strings.HasPrefix(head, "0;") || strings.HasPrefix(head, "2;"):
			s.breakCells("a title cut off by another sequence")
		default:
			s.breakCells("OSC " + head[:n] + " cut off by another sequence")
		}
		return
	}
	switch s.str {
	case ']':
		s.breaks("OSC " + head[:min(n, 8)])
	case 'P':
		s.breaks("DCS " + dcsName(s.strHead))
	case '_':
		name := "APC"
		if len(s.strHead) > 0 {
			name += " " + byteName(s.strHead[0])
		}
		s.breaks(name)
	case 'X':
		s.breaks("SOS")
	default:
		s.breaks("PM")
	}
}

// screenQuietOSC: an OSC's body (its first screenMaxHead bytes) is one on the
// list, known to draw nothing on xterm and conhost (see the list above): a
// title (0 or 2), a link opened or closed (8;params;URI, the second ; inside
// those bytes), or exactly the background colour asked (11;?) or Claude Code's
// own question (7501;?). A body as long as the bytes kept is never one of the
// last two, since it holds more than they do.
func screenQuietOSC(head string) bool {
	switch {
	case strings.HasPrefix(head, "0;"), strings.HasPrefix(head, "2;"):
		return true
	case strings.HasPrefix(head, "8;"):
		return strings.IndexByte(head[2:], ';') >= 0
	}
	return head == "11;?" || head == "7501;?"
}

// dcsIntro splits a DCS's start into its parameters (0x30..0x3F), its
// intermediates and final byte; ok is false if no final byte is in head.
func dcsIntro(head []byte) (params, rest string, ok bool) {
	i := 0
	for i < len(head) && head[i] >= 0x30 && head[i] <= 0x3f {
		i++
	}
	j := i
	for j < len(head) && head[j] >= 0x20 && head[j] <= 0x2f {
		j++
	}
	if j == len(head) || head[j] < 0x40 || head[j] > 0x7e {
		return "", "", false
	}
	return string(head[:i]), string(head[i : j+1]), true
}

// dcsName is a DCS by its own parameters and final byte (never its data).
func dcsName(head []byte) string {
	params, rest, ok := dcsIntro(head)
	if !ok {
		return "with no final byte"
	}
	return params[:min(len(params), 24)] + rest
}
