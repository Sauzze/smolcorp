package desktop

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// textScreen is a small rows-by-cols text screen: what the Windows launcher's
// pty output (go-pty, ConPTY) looks like once drawn, as the text
// `tmux capture-pane -p` gives on Linux, so ReadPaneState can read it.
//
// It draws printable text and handles \r \n \b \t, the cursor moves CSI A B C
// D G H f, the erases CSI J K X, and scrolls at the bottom. It ignores colours
// (SGR m) and the harmless private modes, and skips OSC strings. It is not a
// terminal emulator. Anything it can't draw faithfully (the alternate screen,
// scroll regions, wide or combining characters, unknown sequences, bad UTF-8,
// a narrower window) makes Unknown true, and then the pane reads as
// PaneUnknown, never PaneIdle: in doubt, nothing is typed.
//
// The launcher feeds it through a launcherScreen (launcherlook.go), which
// holds the lock. Not safe for concurrent use by itself.
type textScreen struct {
	rows, cols int
	grid       [][]rune
	row, col   int
	// wrap: the last column was just written; the next character starts a new
	// line (the deferred wrap of real terminals).
	wrap bool

	mode    screenMode
	params  []byte // the CSI parameters and intermediates being read
	nums    []int  // scratch for the numbers in params
	osc     int    // bytes of the OSC string skipped so far
	oscC2   bool   // the last byte in the OSC string was 0xC2 (a C1 control follows if 80..9F)
	pending []byte // the start of a UTF-8 character cut at the end of a write

	// broken: an unknown sequence or byte was seen; only Reset clears it.
	// stale: the screen no longer matches what was drawn (the alternate
	// screen was left, the window got narrower); a full erase clears it.
	// alt and region: a mode is on that the screen doesn't draw.
	broken, stale, alt, region bool

	// bracketed: the agent asked for bracketed paste (mode 2004). Only said; it
	// never changes how the screen reads.
	bracketed bool

	// dirty: the row may hold something other than blanks; a clear skips the
	// rows that don't, so a stream of clears costs little (kept per row
	// slice, so it moves with a scroll).
	dirty []bool
}

type screenMode int

const (
	modeGround screenMode = iota
	modeEsc
	modeCSI
	modeOSC
	modeOSCEsc
	modeCharset // ESC ( or ESC ): the character set to choose comes next
)

const (
	screenMaxParam = 10000
	screenMaxOSC   = 4096
	screenMaxCSI   = 64
	screenMaxSize  = 1000 // rows and columns; a bigger window is Unknown
)

func newTextScreen(rows, cols int) *textScreen {
	s := &textScreen{}
	s.Reset(rows, cols)
	return s
}

// Reset clears the screen and everything unknown about it, at a new size.
func (s *textScreen) Reset(rows, cols int) {
	*s = textScreen{rows: max(rows, 1), cols: max(cols, 1)}
	if s.rows > screenMaxSize || s.cols > screenMaxSize {
		s.rows, s.cols, s.broken = 1, 1, true
	}
	s.grid = s.blankGrid(s.rows, s.cols)
	s.dirty = make([]bool, s.rows)
}

func (s *textScreen) blankGrid(rows, cols int) [][]rune {
	g := make([][]rune, rows)
	for i := range g {
		g[i] = s.blankRow(cols)
	}
	return g
}

func (s *textScreen) blankRow(cols int) []rune {
	r := make([]rune, cols)
	for i := range r {
		r[i] = ' '
	}
	return r
}

// Unknown: the screen can't be trusted to be what the window shows.
func (s *textScreen) Unknown() bool { return s.broken || s.stale || s.alt || s.region }

// Text is the screen as lines, trailing spaces cut, no final newline.
func (s *textScreen) Text() string {
	lines := make([]string, s.rows)
	for i, r := range s.grid {
		lines[i] = strings.TrimRight(string(r), " ")
	}
	return strings.Join(lines, "\n")
}

// Bracketed says whether the agent turned bracketed paste on.
func (s *textScreen) Bracketed() bool { return s.bracketed }

// PaneState reads the screen like a tmux capture; Unknown reads PaneUnknown.
func (s *textScreen) PaneState() PaneState {
	if s.Unknown() {
		return PaneUnknown
	}
	return ReadPaneState(s.Text())
}

// Resize changes the window's size. Fewer rows push the top lines off, as a
// terminal does when the cursor is at the bottom; more columns add blanks.
// Fewer columns cut the lines where a real terminal may rewrap them, so the
// screen is Unknown until the next full erase.
func (s *textScreen) Resize(rows, cols int) {
	rows, cols = max(rows, 1), max(cols, 1)
	if rows == s.rows && cols == s.cols {
		return
	}
	if rows > screenMaxSize || cols > screenMaxSize {
		s.broken = true
		return
	}
	if cols < s.cols {
		s.stale = true
	}
	if rows < s.rows {
		drop := max(s.row-rows+1, 0)
		s.grid = s.grid[drop:]
		s.row -= drop
		s.grid = s.grid[:rows]
	}
	for len(s.grid) < rows {
		s.grid = append(s.grid, s.blankRow(s.cols))
	}
	for i, r := range s.grid {
		if cols < len(r) {
			s.grid[i] = r[:cols]
		}
		for len(s.grid[i]) < cols {
			s.grid[i] = append(s.grid[i], ' ')
		}
	}
	s.rows, s.cols = rows, cols
	s.dirty = make([]bool, rows)
	for i := range s.dirty {
		s.dirty[i] = true
	}
	s.row, s.col = min(s.row, rows-1), min(s.col, cols-1)
	s.wrap = false
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
				s.broken = true
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

func (s *textScreen) feed(b byte) {
	switch s.mode {
	case modeEsc:
		s.mode = modeGround
		switch b {
		case '[':
			s.mode, s.params = modeCSI, s.params[:0]
		case ']':
			s.mode, s.osc, s.oscC2 = modeOSC, 0, false
		case '(', ')':
			s.mode = modeCharset
		case '=', '>', '\\': // keypad modes, a stray string end
		default:
			s.broken = true
		}
	case modeCharset:
		// ESC ( B and ESC ) B choose plain ASCII, which is what the screen
		// draws. Another set (as 0, line drawing) changes what the bytes
		// mean, so it reads unknown.
		s.mode = modeGround
		if b != 'B' {
			s.broken = true
		}
	case modeCSI:
		if b >= 0x20 && b <= 0x3f {
			if len(s.params) < screenMaxCSI {
				s.params = append(s.params, b)
			} else {
				s.broken = true
			}
			return
		}
		s.mode = modeGround
		if b < 0x40 || b > 0x7e {
			s.broken = true
			return
		}
		s.csi(b)
	case modeOSC, modeOSCEsc:
		if s.osc++; s.osc > screenMaxOSC {
			s.broken, s.mode = true, modeGround
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
			s.broken, s.mode = true, modeGround
		case b == 0x07 || (s.mode == modeOSCEsc && b == '\\'):
			s.mode = modeGround
		case s.mode == modeOSCEsc: // cut off by another sequence, as xterm does
			s.mode = modeEsc
			s.feed(b)
		case b == 0x1b:
			s.mode = modeOSCEsc
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
		s.col, s.wrap = 0, false
	case b == '\n':
		s.lineFeed()
		s.wrap = false
	case b == '\b':
		s.col, s.wrap = max(s.col-1, 0), false
	case b == '\t':
		s.col, s.wrap = min((s.col/8+1)*8, s.cols-1), false
	case b == 0 || b == 0x07:
	case b < 0x20 || b == 0x7f:
		s.broken = true
	default:
		s.put(rune(b))
	}
}

func (s *textScreen) lineFeed() {
	if s.row < s.rows-1 {
		s.row++
		return
	}
	top := s.grid[0]
	copy(s.grid, s.grid[1:])
	s.grid[s.rows-1] = top
	copy(s.dirty, s.dirty[1:])
	s.dirty[s.rows-1] = true
	s.fill(s.rows-1, 0, s.cols)
}

// fill blanks cells from..to of a row, in place.
func (s *textScreen) fill(row, from, to int) {
	if !s.dirty[row] {
		return
	}
	for i := from; i < to; i++ {
		s.grid[row][i] = ' '
	}
	if from == 0 && to == s.cols {
		s.dirty[row] = false
	}
}

func (s *textScreen) put(r rune) {
	if screenWide(r) || unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Cc) {
		s.broken = true
		return
	}
	if s.wrap {
		s.col, s.wrap = 0, false
		s.lineFeed()
	}
	s.grid[s.row][s.col] = r
	s.dirty[s.row] = true
	if s.col == s.cols-1 {
		s.wrap = true
	} else {
		s.col++
	}
}

// screenWide: characters that take (or may take) other than one cell: East
// Asian wide, the emoji that are wide by default, the whole 1F000 block, the
// Hangul Jamo (1100-11FF, wide or zero width), and the line and paragraph
// separators, so none of them is drawn one cell wide.
func screenWide(r rune) bool {
	return r >= 0x1100 && (r <= 0x11ff || r == 0x2028 || r == 0x2029 ||
		(r >= 0x2329 && r <= 0x232a) || (r >= 0xa960 && r <= 0xa97c) || (r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0x16fe0 && r <= 0x18d08) || (r >= 0x1aff0 && r <= 0x1b2fb) ||
		(r >= 0x231a && r <= 0x231b) || (r >= 0x23e9 && r <= 0x23ec) || r == 0x23f0 || r == 0x23f3 ||
		(r >= 0x25fd && r <= 0x25fe) || (r >= 0x2614 && r <= 0x2615) || (r >= 0x2648 && r <= 0x2653) ||
		r == 0x267f || r == 0x2693 || r == 0x26a1 || (r >= 0x26aa && r <= 0x26ab) ||
		(r >= 0x26bd && r <= 0x26be) || (r >= 0x26c4 && r <= 0x26c5) || r == 0x26ce || r == 0x26d4 ||
		r == 0x26ea || (r >= 0x26f2 && r <= 0x26f3) || r == 0x26f5 || r == 0x26fa || r == 0x26fd ||
		r == 0x2705 || (r >= 0x270a && r <= 0x270b) || r == 0x2728 || r == 0x274c || r == 0x274e ||
		(r >= 0x2753 && r <= 0x2755) || r == 0x2757 || (r >= 0x2795 && r <= 0x2797) ||
		r == 0x27b0 || r == 0x27bf || (r >= 0x2b1b && r <= 0x2b1c) || r == 0x2b50 || r == 0x2b55 ||
		(r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f000 && r <= 0x1ffff) ||
		(r >= 0x20000 && r <= 0x3fffd))
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

// harmlessCSI: sequences that draw nothing and move nothing, which ConPTY and
// Claude Code send as they start, and which csiParams can't read:
//   - SGR with ":" subparameters, as ESC[38:2::12:34:56m (a colour); only
//     digits, ";" and ":" are allowed, so nothing else rides along;
//   - DECSCUSR, ESC[0 q, the cursor's shape (one digit, a space, "q").
//
// Every other final byte with a ":" or an intermediate still reads unknown.
func harmlessCSI(params []byte, final byte) bool {
	switch final {
	case 'm':
		for _, c := range params {
			if !(c >= '0' && c <= '9') && c != ';' && c != ':' {
				return false
			}
		}
		return len(params) > 0 && strings.IndexByte(string(params), ':') >= 0
	case 'q':
		n := len(params)
		if n < 1 || params[n-1] != ' ' {
			return false
		}
		for _, c := range params[:n-1] {
			if c < '0' || c > '9' {
				return false
			}
		}
		return n-1 <= 2
	}
	return false
}

func (s *textScreen) csi(final byte) {
	if p := s.params; len(p) > 0 && strings.IndexByte("?<=>", p[0]) >= 0 {
		s.csiPrivate(p[0], string(p[1:]), final)
		return
	}
	if harmlessCSI(s.params, final) {
		return
	}
	nums, ok := csiParams(s.nums[:0], s.params)
	s.nums = nums
	if !ok {
		s.broken = true
		return
	}
	arg := func(i, def int) int {
		if i < len(nums) && nums[i] > 0 {
			return nums[i]
		}
		return def
	}
	if final == 'm' { // colours and styles move nothing, so keep a pending wrap
		return
	}
	s.wrap = false
	switch final {
	case 'A':
		s.row = max(s.row-arg(0, 1), 0)
	case 'B':
		s.row = min(s.row+arg(0, 1), s.rows-1)
	case 'C':
		s.col = min(s.col+arg(0, 1), s.cols-1)
	case 'D':
		s.col = max(s.col-arg(0, 1), 0)
	case 'G':
		s.col = min(arg(0, 1), s.cols) - 1
	case 'H', 'f':
		s.row, s.col = min(arg(0, 1), s.rows)-1, min(arg(1, 1), s.cols)-1
	case 'J':
		s.eraseDisplay(arg(0, 0))
	case 'K':
		s.eraseLine(arg(0, 0))
	case 'X':
		s.fill(s.row, s.col, min(s.col+arg(0, 1), s.cols))
	case 'r': // scroll region: only the whole screen is drawn faithfully
		top, bottom := arg(0, 1), arg(1, s.rows)
		s.region = top != 1 || bottom != s.rows
		s.row, s.col = 0, 0 // setting the region homes the cursor
	default:
		s.broken = true
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
		s.stale = false
	case 3: // the scrollback, which a capture doesn't show
	default:
		s.broken = true
	}
}

func (s *textScreen) eraseLine(mode int) {
	switch mode {
	case 0:
		s.fill(s.row, s.col, s.cols)
	case 1:
		s.fill(s.row, 0, s.col+1)
	case 2:
		s.fill(s.row, 0, s.cols)
	default:
		s.broken = true
	}
}

// csiPrivate: CSI ? ... h/l (modes), and the keyboard and style reports Claude
// Code sends (CSI > ... m, CSI > < = ? ... u).
func (s *textScreen) csiPrivate(prefix byte, params string, final byte) {
	switch {
	case final == 'm' || final == 'u':
		return
	case prefix == '?' && (final == 'h' || final == 'l'):
		for _, f := range strings.Split(params, ";") {
			switch f {
			case "47", "1047", "1049":
				s.alt = final == 'h'
				if final == 'l' {
					s.stale = true // the main screen was not kept
				}
			case "2004":
				s.bracketed = final == 'h'
			case "1", "12", "25", "1000", "1002", "1003", "1004", "1005", "1006", "2026", "9001":
			default:
				s.broken = true
			}
		}
		return
	}
	s.broken = true
}
