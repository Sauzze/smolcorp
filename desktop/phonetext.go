package desktop

import (
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The phone's text (I5): what the phone typed,
// or heard with its own mic, typed into a desk's agent by POST /dictate, as the
// Mac app's PhoneDictation.swift and Deck.typePhoneDictation do. One line goes
// as typed text; several lines as ONE paste (a bracketed paste, where the
// agent asked for one), never a Return part way; where the desk's terminal
// can't take that, nothing is typed. Return goes on its own, after, if asked.
// A typed answer to a menu (POST /choose) and a look at the input line (POST
// /peek, GET /choice) follow the same rules. Nothing is kept.

const (
	// phoneTextLongest is the most text one request may carry, in characters, and
	// phoneTextLongestBytes in bytes: a character can hide any number of marks.
	phoneTextLongest      = 2000
	phoneTextLongestBytes = 8 * phoneTextLongest
	// phoneTextWordsToSend: an older page's text goes with Return once it's this many words, with autoSend on.
	phoneTextWordsToSend = 3
	// The peek: this many of the tab's last lines, each cut to this many characters, once a second at most.
	peekLines = 10
	peekWidth = 240
	peekGap   = time.Second
)

// phoneTextSettle is how long the agent is given to take the text in before
// Return goes on its own: sent with the text, it would be a new line in a paste.
var phoneTextSettle = 250 * time.Millisecond

// The phone's sentences, as the Mac's (Deck.swift), with "computer" for "Mac".
const (
	phoneTextOff        = "The phone's text is off on this computer."
	phoneTextPhonesOnly = "Text is typed from a paired phone."
	answerFirst         = "It's asking you something: answer it on the deck first."
	noLines             = "This desk's terminal can't take several lines as one paste, so nothing was typed: send it as one line."
	oneLineAnswer       = "A typed answer goes as keys, where a new line would press Return, so nothing was typed: put it on one line."
	choiceGone          = "It's asking something else now, or nothing: look again."
	notHere             = "Smolcorp can't do that in this terminal yet."
	noBang              = "A typed answer can't start with “!”: Claude Code would take it for a shell command."
	noBangPrompt        = "Text can't start with “!”: the agent would run it as a shell command."
	typedNotSent        = "Typed, not sent: it began asking you something, or a Return had just gone. Look at the deck."
	tooLong             = "Too long: at most 2000 characters at a time."
	peekTooSoon         = "Too soon: one look a second."
	notInFront          = "That agent isn't in front in its terminal."
	cantShowScreen      = "This terminal can't show its input line."
	notAsking           = "It isn't asking you anything."
	peekTailnet         = "The input line shows only over Tailscale."
	choiceTailnet       = "The details show only over Tailscale."
	pickWithKeys        = "Pick an option with the deck's keys."
	notFrontFg          = "That agent isn't in front in its terminal: bring it back with fg."
	// A failure once the words were on their way: never "Tab not found".
	launcherLate = "The window didn't say in time whether it typed the text: it may have landed, but no Return went. Look at the desk before you try again."
	pasteUnsaid  = "The terminal didn't say it took the text, so nothing more went. Look at the desk before you try again."
)

// phoneLineBreaks end a line: a line feed, a return (CRLF is one), and Unicode's next-line, line and paragraph separators.
func phoneLineBreak(r rune) bool {
	return r == '\n' || r == '\r' || r == 0x85 || r == 0x2028 || r == 0x2029
}

// macWhitespace is Swift's CharacterSet.whitespacesAndNewlines: the Z categories, tab to return, and NEL.
func macWhitespace(r rune) bool {
	return (r >= 0x09 && r <= 0x0D) || r == 0x85 || unicode.In(r, unicode.Zs, unicode.Zl, unicode.Zp)
}

// flattenedPhoneText is the text as it will be typed. Of several lines: each
// line break a line feed, a tab kept, other control characters (Cc) spaces,
// spaces and blank lines round the whole trimmed. On one line: a tab a space
// too, runs of spaces one. Invisible format characters (Cf), as the joiner in
// 👩‍💻, stay. As the Mac's PhoneDictation.flattened.
func flattenedPhoneText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.Map(func(r rune) rune {
		switch {
		case phoneLineBreak(r):
			return '\n'
		case unicode.Is(unicode.Cc, r) && r != '\t':
			return ' '
		}
		return r
	}, text)
	lines := strings.TrimFunc(text, macWhitespace)
	if strings.Contains(lines, "\n") {
		return lines
	}
	var words []string
	for _, word := range strings.Split(strings.ReplaceAll(lines, "\t", " "), " ") {
		if word != "" {
			words = append(words, word)
		}
	}
	return strings.Join(words, " ")
}

// preparedPhoneText is the text as typed, and false if nothing's left or it's too long (phoneTextRefusal says which).
func preparedPhoneText(text string) (string, bool) {
	words := flattenedPhoneText(text)
	if words == "" || characterCount(words) > phoneTextLongest || len(words) > phoneTextLongestBytes {
		return "", false
	}
	return words, true
}

// phoneTextRefusal is why preparedPhoneText gave nothing.
func phoneTextRefusal(text string) string {
	if words := flattenedPhoneText(text); characterCount(words) > phoneTextLongest || len(words) > phoneTextLongestBytes {
		return tooLong
	}
	return "Nothing to type"
}

// isPhoneLines says whether prepared text is of several lines.
func isPhoneLines(words string) bool { return strings.Contains(words, "\n") }

// runsInShell: Claude Code, Codex, Gemini CLI and OpenCode run a prompt starting with "!" in a shell. By
// its first code point, as they look: "!" with an accent on it is still "!" to them.
func runsInShell(words string) bool { return strings.HasPrefix(words, "!") }

// phoneTextSends says whether text goes with Return, for an older page that doesn't say: with autoSend
// on, and once it's a few words.
func phoneTextSends(words string, autoSend bool) bool {
	return autoSend && len(strings.FieldsFunc(words, unicode.IsSpace)) >= phoneTextWordsToSend
}

// graphemeStarts are where text's characters start, as a person counts them and Swift's String does:
// a mark, a joiner and what it joins, a variation selector, a skin tone, a tag, and the second of a
// flag's two letters belong to the character before. Near enough to Unicode's own rules for counting
// to 2,000 and cutting a peek's line; a rare script may count a little differently at the limit.
func graphemeStarts(text string) []int {
	var starts []int
	var previous rune
	regional := 0
	for i, r := range text {
		joins := unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc) || r == 0x200D || previous == 0x200D ||
			(r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF) || (r >= 0x1F3FB && r <= 0x1F3FF) ||
			(r >= 0xE0020 && r <= 0xE007F)
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			if regional%2 == 1 {
				joins = true
			}
			regional++
		} else {
			regional = 0
		}
		if !joins || len(starts) == 0 {
			starts = append(starts, i)
		}
		previous = r
	}
	return starts
}

func characterCount(text string) int { return len(graphemeStarts(text)) }

// characterPrefix is text's first n characters.
func characterPrefix(text string, n int) string {
	if starts := graphemeStarts(text); len(starts) > n {
		return text[:starts[n]]
	}
	return text
}

// peekOf is the end of what a tab shows, as the phone's peek gives it: its blank lines underneath dropped,
// its last peekLines, each cut to peekWidth, with every control and format character gone, as the Mac's
// PhoneDictation.peek.
func peekOf(screen string) []string {
	blank := func(line string) bool {
		return strings.TrimFunc(line, func(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }) == ""
	}
	var lines []string
	for _, line := range splitMacNewlines(screen) {
		line = strings.Map(func(r rune) rune {
			if unicode.In(r, unicode.Cc, unicode.Cf) {
				return -1
			}
			return r
		}, line)
		lines = append(lines, characterPrefix(line, peekWidth))
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	lines = lines[max(0, len(lines)-peekLines):]
	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	if lines == nil {
		lines = []string{}
	}
	return lines
}

// splitMacNewlines splits as Swift's components(separatedBy: .newlines): at each of LF, VT, FF, CR, NEL,
// LS and PS, a CRLF being two.
func splitMacNewlines(text string) []string {
	var lines []string
	start := 0
	for i, r := range text {
		if (r >= 0x0A && r <= 0x0D) || r == 0x85 || r == 0x2028 || r == 0x2029 {
			lines = append(lines, text[start:i])
			start = i + utf8.RuneLen(r)
		}
	}
	return append(lines, text[start:])
}

// MARK: The deck's side

// pasteResult is how a phone's text went into a tab.
type pasteResult int

const (
	// pasteFailed: the terminal didn't take it, or couldn't say; nothing more goes.
	pasteFailed pasteResult = iota
	pasteDone
	// pasteNotTaken: several lines where the tab can't take them as one paste now: nothing was typed.
	pasteNotTaken
)

var (
	errPasteFailed = errors.New("the terminal didn't take the text")
	errAgentBehind = errors.New("the agent isn't in front in its terminal")
)

// typesText says whether this app types the phone's text into a tab named so, without asking the Mac app:
// a tmux, WezTerm or kitty pane, or a launcher's window. Under the Mac app, its drivers' table says (hostoffice.go).
func typesText(name string) bool {
	kind, _, _ := splitTab(name)
	return kind == "tmux" || kind == "wezterm" || kind == "kitty" || kind == "launcher"
}

// takesLinesAtAll is false for a tab whose terminal can never take several lines as one paste: WezTerm's
// and kitty's send-text can't tell whether the agent asked for bracketed paste, and without it each line
// would press Return. Refused before waiting for the terminal, as nothing can come of it.
func takesLinesAtAll(name string) bool {
	kind, _, _ := splitTab(name)
	return kind != "wezterm" && kind != "kitty"
}

// phoneText types words into t, session tab name's, without Return: one line as typed text, several as
// one paste, where the tab takes one. On the key worker, right after the desk was seen not asking; the Mac
// app, whose terminals wait on queues of their own, asks deskState from there, as its own engine looks.
func (d *Deck) phoneText(name string, t Tab, agent Agent, words string, lines bool, deskState func() SessionState) (pasteResult, error) {
	switch tab := t.(type) {
	case *launcherTab:
		_, id, _ := splitTab(name)
		return d.pasteIntoLauncher(id, words)
	case macTab:
		if tab.pane != nil {
			return tab.pane.phoneText(words, lines)
		}
		return hostPhoneText(tab.host, tab.name, agent, words, deskState)
	case commandTab:
		return tab.phoneText(words, lines)
	}
	return pasteFailed, errUnsupported
}

// deskState is session id's state now, "" for one gone: Asking waits on you (a permission prompt or a
// question, whose menu the words would answer), Thinking is working. Called without the lock.
func (d *Deck) deskState(id string) SessionState {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s := d.sessions.Get(id); s != nil {
		return s.State
	}
	return ""
}

// claimReturn takes the session's Return for the phone's text, if it may go now: to a session still
// there, not asking you something, and not within returnRepeat of another Return, as /act sends them.
func (d *Deck) claimReturn(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.sessions.Get(id)
	if s == nil || s.State == Asking {
		return false
	}
	if at, ok := d.lastReturn[id]; ok && time.Since(at) < returnRepeat {
		return false
	}
	d.lastReturn[id] = time.Now()
	return true
}

// TypePhoneText is POST /dictate: the phone's words typed into session id's agent, with Return if send says
// so (the confirm box's Send ⏎), or, for an older page that says nothing, once they're a few words and
// autoSend is on. Only to the agent in front in its tab (toAgent), and not while it asks you something:
// checked as the request comes, and again right before the words: on the key worker, and under the Mac app
// on the terminal's own queue too, as the Mac app asks (hostPhoneText).
func (d *Deck) TypePhoneText(text, id string, send *bool) (int, string) {
	d.mu.Lock()
	on, autoSend := d.config.PhoneMicEnabled, d.config.PhoneMicAutoSend
	d.mu.Unlock()
	if !on {
		return http.StatusForbidden, phoneTextOff
	}
	words, ok := preparedPhoneText(text)
	if !ok {
		return http.StatusBadRequest, phoneTextRefusal(text)
	}
	// Typed into the agent, "!" would switch it to its shell, and Return run the rest there.
	if runsInShell(words) {
		return http.StatusBadRequest, noBangPrompt
	}
	d.mu.Lock()
	// A hire in T3 Code's desk is not typed into: its own sentence, as /act's send and the Mac say it.
	if hire, ok := d.t3HireAtDesk(id); ok {
		d.mu.Unlock()
		return http.StatusConflict, t3NoControls(calledHire(hire))
	}
	s := d.sessions.Get(id)
	if s == nil || s.Tab == "" {
		d.mu.Unlock()
		return http.StatusNotFound, "Session not found"
	}
	name, agent, asking, tab := s.Tab, s.Agent, s.State == Asking, d.tab(s.Tab)
	d.mu.Unlock()
	// A test copy types nothing into the owner's terminal tabs (RWTC N1).
	if why := testCopyKeepsTab(name); why != "" {
		return http.StatusConflict, why
	}
	// Asking you something, its menu would take the words: a digit picking an option, Return the one highlighted.
	if asking {
		return http.StatusConflict, answerFirst
	}
	if tab == nil {
		return http.StatusConflict, notHere
	}
	lines := isPhoneLines(words)
	if lines && !takesLinesAtAll(name) {
		return http.StatusConflict, noLines
	}
	sends := phoneTextSends(words, autoSend)
	if send != nil {
		sends = *send
	}
	deskState := func() SessionState { return d.deskState(id) }
	var asked, notTaken, withheld, unsupported bool
	var failure error
	status, message := d.toAgent(name, agent, func(t Tab) error {
		// Asked something while this waited for the worker: checked again right before the words.
		if deskState() == Asking {
			asked = true
			return nil
		}
		result, err := d.phoneText(name, t, agent, words, lines, deskState)
		switch {
		case errors.Is(err, errUnsupported):
			unsupported = true
			return nil
		case err != nil:
			failure = err
			return err
		case result == pasteNotTaken:
			notTaken = true
			return nil
		case result != pasteDone:
			failure = errPasteFailed
			return errPasteFailed
		}
		if !sends {
			return nil
		}
		// Return on its own once the words are in, as the ⏎ key presses it.
		time.Sleep(phoneTextSettle)
		// Asked something meanwhile, or a ⏎ just went: this Return would answer what nobody has seen.
		if !d.claimReturn(id) {
			withheld = true
			return nil
		}
		if err := t.Press(press(Enter)); err != nil {
			d.mu.Lock()
			delete(d.lastReturn, id)
			d.mu.Unlock()
			return err
		}
		return nil
	})
	switch {
	case asked:
		return http.StatusConflict, answerFirst
	case notTaken:
		return http.StatusConflict, noLines
	case unsupported:
		return http.StatusConflict, notHere
	case withheld && status < http.StatusBadRequest:
		return http.StatusConflict, typedNotSent
	}
	if failed, why, ok := phoneTextFailure(failure); ok {
		return failed, why
	}
	return status, message
}

// phoneTextFailure says what the phone is told of a failure to type its text, where the deck's words for a
// tab's (inTab) would say "Tab not found": a launcher's window that didn't answer in time, which may still
// type them; a terminal that answered without saying it took them, or the Mac app saying they may have gone
// all the same (as its ⌘V the terminal hadn't read in time); the Mac app's no, in its words. Past 500, the
// phone doesn't put the words back in its box: they may be in the terminal already.
func phoneTextFailure(err error) (int, string, bool) {
	var refusal appRefusal
	switch {
	case err == nil, errors.Is(err, errAgentGone), errors.Is(err, errAgentBehind):
		return 0, "", false
	case errors.Is(err, errLookTimedOut):
		return http.StatusServiceUnavailable, launcherLate, true
	case errors.Is(err, errPasteFailed):
		return http.StatusBadGateway, pasteUnsaid, true
	case errors.As(err, &refusal):
		return http.StatusConflict, string(refusal), true
	}
	return 0, "", false
}

// Peek is POST /peek: the last lines session id's tab shows, read by its terminal, for the phone to see
// what was typed. Read-only, kept nowhere; once a second at most, and only while the agent is known to be
// in front there, so a shell's screen is never read. Over Tailscale only (the router's).
func (d *Deck) Peek(id string) (int, []string, string) {
	d.mu.Lock()
	on := d.config.PhoneMicEnabled
	s := d.sessions.Get(id)
	if !on {
		d.mu.Unlock()
		return http.StatusForbidden, nil, phoneTextOff
	}
	if s == nil || s.Tab == "" {
		d.mu.Unlock()
		return http.StatusNotFound, nil, "Session not found"
	}
	if s.Headless() {
		d.mu.Unlock()
		return http.StatusConflict, nil, headlessNoControls
	}
	if !d.lookedAt(d.lastPeek, id) {
		d.mu.Unlock()
		return http.StatusTooManyRequests, nil, peekTooSoon
	}
	agent, tab, name := s.Agent, d.tab(s.Tab), s.Tab
	d.mu.Unlock()
	// A test copy reads nothing of the owner's terminal tabs (RWTC N1).
	if why := testCopyKeepsTab(name); why != "" {
		return http.StatusConflict, nil, why
	}
	if tab == nil {
		return http.StatusConflict, nil, notHere
	}
	type looked struct {
		lines []string
		err   error
	}
	answer := make(chan looked, 1)
	d.work <- func() {
		lines, err := readPeek(tab, agent)
		answer <- looked{lines, err}
	}
	got := <-answer
	var refusal appRefusal
	switch {
	case errors.Is(got.err, errAgentBehind), errors.Is(got.err, errAgentGone):
		return http.StatusConflict, nil, notInFront
	case errors.As(got.err, &refusal):
		return http.StatusConflict, nil, string(refusal)
	case got.err != nil:
		return http.StatusConflict, nil, cantShowScreen
	}
	return http.StatusOK, got.lines, ""
}

// isProxied says whether a proxy passed the request on, as `tailscale serve` does: it says whom for in these
// headers, and sets them over any the sender put in. As the Mac's Addressing.isProxied: there, even empty.
func isProxied(header http.Header) bool {
	for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "Forwarded", "Tailscale-User-Login"} {
		if header.Values(name) != nil {
			return true
		}
	}
	return false
}

// tailnetName matches a MagicDNS name in a tailnet, as my-mac.tail1234.ts.net: as the Mac's Addressing.isTailnet.
var tailnetName = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.){2}ts\.net$`)

// isTailnetName says whether a Host, without its port, is a tailnet's name. Believed only from this computer's
// loopback, as `tailscale serve` hands it on: Tailscale, not a web page, says where ts.net names point.
func isTailnetName(host string) bool { return tailnetName.MatchString(strings.ToLower(host)) }

// lookedAt says whether a look at session id may go now, and if so counts it: once every peekGap. With the lock held.
func (d *Deck) lookedAt(last map[string]time.Time, id string) bool {
	now := time.Now()
	if at, ok := last[id]; ok && now.Sub(at) < peekGap {
		return false
	}
	last[id] = now
	return true
}

// readPeek reads the peek from tab, only where agent is known to be in front. On the key worker.
func readPeek(t Tab, agent Agent) ([]string, error) {
	switch tab := t.(type) {
	case macTab:
		if tab.pane == nil {
			return hostPeek(tab.host, tab.name, agent)
		}
		return tab.pane.peek(agent)
	case commandTab:
		return tab.peek(agent)
	}
	// A launcher's window says its state, never its text.
	return nil, errUnsupported
}

// AnswerOther is POST /choose with option=other: the menu's "Type something", with text, typed as keys
// once the field is open. One line only: as keys, a line feed would press Return. This app doesn't follow
// what a menu offers, so once the text passes every rule no question can be claimed, and nothing is typed:
// 409, as the Mac answers a question gone.
func (d *Deck) AnswerOther(text, choiceID, id string) (int, string) {
	d.mu.Lock()
	on := d.config.PhoneMicEnabled
	d.mu.Unlock()
	if !on {
		return http.StatusForbidden, phoneTextOff
	}
	words, ok := preparedPhoneText(text)
	if !ok {
		return http.StatusBadRequest, phoneTextRefusal(text)
	}
	// Where the field didn't open, the text would be a prompt, and one starting with "!" a shell command.
	if runsInShell(words) {
		return http.StatusBadRequest, noBang
	}
	if isPhoneLines(words) {
		return http.StatusBadRequest, oneLineAnswer
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.sessions.Get(id)
	if s == nil || s.Tab == "" {
		return http.StatusNotFound, "Session not found"
	}
	if s.Headless() {
		return http.StatusConflict, headlessNoControls
	}
	return http.StatusConflict, choiceGone
}

// ChoiceDetail is GET /choice: what a permission prompt is about. Once a second at most; with no menu
// followed here (see AnswerOther), it isn't asking anything this app can tell. Over Tailscale only (the router's).
func (d *Deck) ChoiceDetail(id string) (int, string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.config.PhoneMicEnabled {
		return http.StatusForbidden, phoneTextOff
	}
	s := d.sessions.Get(id)
	if s == nil || s.Tab == "" {
		return http.StatusNotFound, "Session not found"
	}
	if !d.lookedAt(d.lastChoiceLook, id) {
		return http.StatusTooManyRequests, peekTooSoon
	}
	return http.StatusNotFound, notAsking
}

// fromTailnet says whether a request came through Tailscale, where reading a terminal is given: to this
// computer's Tailscale address from another device's (WireGuard carries it encrypted), or passed on by
// `tailscale serve` on this computer (from its loopback, with a proxy's headers, or on a Mac to a ts.net
// name, as the Mac app's Request.isTailnet). Never the Wi-Fi's plain http.
func fromTailnet(r *http.Request) bool {
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(remote)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return isProxied(r.Header) || macTailscale && isTailnetName(portSuffix.ReplaceAllString(r.Host, ""))
	}
	local, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if local == nil || ip.To4() == nil || !tailnet.Contains(ip.To4()) {
		return false
	}
	host, _, _ := net.SplitHostPort(local.String())
	for _, own := range tailscaleAddresses() {
		if host == own {
			return true
		}
	}
	return false
}
