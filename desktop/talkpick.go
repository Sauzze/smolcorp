package desktop

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// 💬 Talk bubble, pick (1.26, P3): POST /talk/pick?session=&n=&since= answers the prompt a desk began asking at
// since with its choice n, as a tap on a labelled button in the office's own window. A choice is a key, never text:
// AskUserQuestion's options and a permission prompt's Yes are the choice's digit, which Claude Code's menus take and
// answer at once; a permission prompt's No is the agent's Reject, as ✕ presses it.
//
// The guards: the desk still asking that same prompt (else 409, choiceGone), looked at again on the key worker right
// before the key; its choices offered by label (pickable, else 409: the four keys answer it); n one of them (else
// 400); one pick per prompt (a second is 204 and presses nothing); a headless run, a hire in T3 Code and, on a test
// copy, any tab but its own hires' tmux panes refused as the office's keys are (PerformFrom).
//
// And the screen (RT126P3 B1): what the hooks say is only what the deck believes. A prompt put away in its own
// terminal sends no hook, and one answered Yes there sends none until its tool ends, so the desk still looks like it
// asks. Right before the key, on the key worker, the tab's screen is read as the phone's peek reads it, and the key
// goes only while Claude Code's menu for that prompt is there (talkMenuOn). Only where the screen plainly ends with
// Claude Code's input box is the prompt known to be gone: the pick is 409 choiceGone and the desk stops asking, so the
// window's input wakes (idle, or thinking while the tool it allowed runs). With another prompt's menu, 409 and
// nothing more; with a screen it can't make out (RT126P3b B1: choice 1 off its top, words it doesn't know, nothing
// read), 409 with the keys' sentence and the desk still asking, so the phone's words still keep out of a live menu.
// A digit thus never lands in the agent's input box, and an Escape never stops a running tool. Where the screen can't
// be read at all, the card isn't pickable (talkShowsScreen): the four keys answer it.

const (
	// A permission prompt's choices on the card: the hooks don't say Claude Code's menu, so only these two (D4).
	talkPermissionYes = 1
	talkPermissionNo  = 2
	// Why a prompt whose choices aren't offered by label can't be picked: the four keys answer it.
	talkUseTheKeys = "Its choices can't be picked from here: answer it with the keys."
	talkNoChoice   = "Unknown choice"
)

// errTalkChoiceGone is a desk that stopped asking, or began asking something else, while its pick waited for the key
// worker, or whose screen doesn't show that prompt's menu: nothing is pressed.
var errTalkChoiceGone = errors.New(choiceGone)

// talkScreenLines is how many of a screen's last lines a pick reads for its menu: a question with its options and
// their descriptions, or a command's box. The Mac app gives as many at most (PhoneDictation.menuLines).
const talkScreenLines = 60

// talkShowsScreen says whether a pick can see the menu in tab: tmux, WezTerm and kitty by their command line;
// under the Mac app, Terminal and iTerm2 through it (Ghostty's screen it can't read); never a launcher's window,
// which says its state but not its text. A variable, for the tests.
var talkShowsScreen = func(tab string) bool {
	switch kind, _, _ := splitTab(tab); kind {
	case "tmux", "wezterm", "kitty":
		return true
	case "tty", "iterm":
		return host != nil
	}
	return false
}

// talkScreenOf reads the last lines tab shows, only where agent is known to be in front there. On the key worker.
// A variable, for the tests.
var talkScreenOf = func(t Tab, agent Agent) ([]string, error) { return readScreen(t, agent, talkScreenLines) }

// talkMenu is what a desk's screen shows of the prompt a pick answers.
type talkMenu int

const (
	talkCantTell  talkMenu = iota // nothing it can make out: maybe a menu, so the desk still asks
	talkBoxWaits                  // Claude Code's input box, waiting for a prompt: put away in its own terminal
	talkBoxWorks                  // the input box, a tool running: answered Yes in its own terminal
	talkOtherMenu                 // a menu, but not that prompt's
	talkThisMenu                  // that prompt's own menu
)

// How Claude Code draws its menus (2.1): a permission prompt asks "Do you want to proceed?" (or about a file), its
// choices numbered under it, ❯ at the one chosen, and "Esc to cancel" under them; a question's text, its options
// numbered, and "Enter to select" under them. Either is the last thing on the screen while it asks. With no prompt,
// its input box is: a rule, ❯ and what's typed, a rule, and its status under it ("esc to interrupt" while it works).
const (
	talkProceed       = "Do you want to proceed?"
	talkEscToCancel   = "Esc to cancel"
	talkEnterToSelect = "Enter to select"
	talkEscInterrupts = "esc to interrupt"
	// A footer, or the input box's lower rule, is among the screen's last non-blank lines; a menu's choices and what
	// it is about, this many lines above it at most; the input box, this many lines tall at most.
	talkFooterNear = 3
	talkMenuMost   = 40
	talkBoxMost    = 20
	// How long a line of the screen must be, at least, to be taken for the start of a question or a command.
	talkMatchLeast = 8
	// And a file's line, at least (a.go).
	talkFileLeast = 4
)

var (
	talkCursorLine = regexp.MustCompile(`^❯\s*[1-9]\.\s`)
	talkFirstLine  = regexp.MustCompile(`^(❯\s*)?1\.\s`)
	talkChoiceLine = regexp.MustCompile(`^(❯\s*)?[1-9]\.\s`)
)

// talkMenuOn says what lines, the end of a desk's screen, show of asking: its own menu, another, Claude Code's
// input box, or nothing it can make out.
// A permission prompt's menu: "Esc to cancel" at its foot, or "Do you want to proceed?" right over its ❯ on
// choice 1, with a ❯ on a choice and a choice 1; and, for a Bash command, the command's first line in its box, for
// Write and Edit the file's path as its file line; any other tool's menu can't be told (RT126P3c N1). A
// question's: "Enter to select" at its foot, a choice 1, and the question's text in its box over it.
func talkMenuOn(lines []string, asking *talkAsking) talkMenu {
	text := make([]string, len(lines))
	for i, line := range lines {
		text[i] = strings.Join(strings.Fields(line), " ")
	}
	// The input box last, whatever is typed in it: no prompt asks.
	if box, works := talkInputBoxAtEnd(text); box && works {
		return talkBoxWorks
	} else if box {
		return talkBoxWaits
	}
	footer, kind := -1, ""
	for i, near := len(text)-1, 0; i >= 0 && near < talkFooterNear && footer < 0; i-- {
		if text[i] == "" {
			continue
		}
		near++
		switch {
		case strings.Contains(text[i], talkEnterToSelect):
			footer, kind = i, "question"
		case strings.Contains(text[i], talkEscToCancel):
			footer, kind = i, "permission"
		}
	}
	top := 0
	if footer < 0 {
		// A permission menu with no foot (Claude Code before 2.1): asked last, ❯ on choice 1 right under it, no
		// rule of the input box's after it, and a choice the last line: a foot in words it doesn't know can't tell.
		proceed := lastIndexFunc(text, func(line string) bool { return strings.Contains(line, talkProceed) })
		if proceed < 0 || proceed < len(text)-talkMenuMost {
			return talkCantTell
		}
		next := proceed + 1
		for next < len(text) && text[next] == "" {
			next++
		}
		last := lastIndexFunc(text, func(line string) bool { return line != "" })
		if next == len(text) || !talkCursorLine.MatchString(text[next]) || slices.ContainsFunc(text[proceed:], talkRule) ||
			!talkChoiceLine.MatchString(text[last]) {
			return talkCantTell
		}
		footer, kind, top = len(text), "permission", max(0, proceed-talkMenuMost)
	} else {
		top = max(0, footer-talkMenuMost)
	}
	menu := text[top:footer]
	// Its choices: a choice 1, and ❯ on one of them. Choice 1 off the screen's top: it can't tell.
	first := lastIndexFunc(menu, talkFirstLine.MatchString)
	if first < 0 || !slices.ContainsFunc(menu[first:], talkCursorLine.MatchString) {
		return talkCantTell
	}
	if kind != asking.Kind {
		return talkOtherMenu
	}
	// What it is about, over its first choice: the question's text, a command from its first line on screen, or a
	// file's path as its menu's file line gives it. Any other tool's menu can't be told from another's (RT126P3c N1).
	about, cut, file := "", false, false
	switch {
	case kind == "question" && len(asking.Questions) > 0:
		about, cut = strings.CutSuffix(asking.Questions[0].Question, "…")
	case kind == "permission" && asking.Tool == "Bash":
		about = asking.command
	case kind == "permission" && (asking.Tool == "Write" || asking.Tool == "Edit"):
		about, file = asking.file, true
	case kind == "permission":
		return talkCantTell
	}
	if about = strings.Join(strings.Fields(about), " "); about == "" {
		if kind == "permission" {
			return talkCantTell
		}
		return talkThisMenu
	}
	// The menu's box only, under the rule at its top: never the turn above it.
	above := menu[:first]
	above = above[lastIndexFunc(above, talkRule)+1:]
	for _, line := range above {
		if file {
			// A file's line is its path as given, or as Claude Code shortens it: under the folder it works in, or
			// with the start cut off with …. The whole line, so a diff's or a typed line can't pass for it.
			shown, cutAtStart := strings.CutPrefix(line, "…")
			if len([]rune(shown)) >= talkFileLeast && (shown == about || strings.HasSuffix(about, "/"+shown) || cutAtStart && strings.HasSuffix(about, shown)) {
				return talkThisMenu
			}
			continue
		}
		line = strings.TrimSuffix(line, "…")
		// The screen's line is the start of what the card asks (wrapped, or cut with …), or, for a question cut on
		// the card, the card's text the start of the line.
		if len([]rune(line)) >= min(len([]rune(about)), talkMatchLeast) && (strings.HasPrefix(about, line) || cut && strings.HasPrefix(line, about)) {
			return talkThisMenu
		}
	}
	return talkOtherMenu
}

// talkInputBoxAtEnd says whether text, the end of a screen with its spaces folded, ends with Claude Code's input box:
// its lower rule among the last few non-blank lines, under it only its status and no menu's footer, over it ❯ (and
// what's typed) right under its upper rule. And whether its status says a tool or the turn is running.
func talkInputBoxAtEnd(text []string) (box, works bool) {
	lower, status := len(text)-1, 0
	for ; lower >= 0 && !talkRule(text[lower]); lower-- {
		if text[lower] == "" {
			continue
		}
		if status++; status > talkFooterNear || talkFooter(text[lower]) {
			return false, false
		}
		works = works || strings.Contains(text[lower], talkEscInterrupts)
	}
	upper := lower - 1
	for ; upper >= 0 && lower-upper <= talkBoxMost && !talkRule(text[upper]); upper-- {
	}
	if lower < 0 || upper < 0 || lower-upper > talkBoxMost || !strings.HasPrefix(text[upper+1], "❯") {
		return false, false
	}
	return true, works
}

// talkFooter says whether line is a menu's footer.
func talkFooter(line string) bool {
	return strings.Contains(line, talkEnterToSelect) || strings.Contains(line, talkEscToCancel)
}

// talkMenuMissed notes, in a debug build only, what a pick found at a screen's end instead of its menu: each of its
// last non-blank lines by its shape alone, never its text. Claude Code changing its menus' words shows there first
// (RT126P3b N4); the opt-in real-menu test is the check (TestRealClaudeMenusTakeTheDigit).
func talkMenuMissed(menu talkMenu, lines []string) {
	if Version != "dev" || menu == talkBoxWaits || menu == talkBoxWorks {
		return
	}
	var shapes []string
	for i := len(lines) - 1; i >= 0 && len(shapes) < talkFooterNear; i-- {
		if line := strings.Join(strings.Fields(lines[i]), " "); line != "" {
			shapes = append(shapes, talkShape(line))
		}
	}
	if len(shapes) == 0 {
		shapes = []string{"nothing"}
	}
	slices.Reverse(shapes)
	why := map[talkMenu]string{talkCantTell: "nothing it knows", talkOtherMenu: "another prompt's menu"}[menu]
	log.Printf("smolcorp: a pick found %s on the screen, ending %s", why, strings.Join(shapes, " / "))
}

// talkShape is a line's shape, with none of its words: a rule, a choice, the cursor, a footer, or its length.
func talkShape(line string) string {
	switch {
	case talkRule(line):
		return "rule"
	case talkCursorLine.MatchString(line):
		return "cursor on a choice"
	case talkFirstLine.MatchString(line):
		return "choice 1"
	case talkFooter(line):
		return "a footer"
	case strings.HasPrefix(line, "❯"):
		return "❯ line"
	}
	return strconv.Itoa(len([]rune(line))) + " characters"
}

// lastIndexFunc is the index of the last of lines that ok says yes to, or -1.
func lastIndexFunc(lines []string, ok func(string) bool) int {
	for i := len(lines) - 1; i >= 0; i-- {
		if ok(lines[i]) {
			return i
		}
	}
	return -1
}

// talkRule says whether line is a rule of Claude Code's: its input box's edges, or a menu's top.
func talkRule(line string) bool {
	return line != "" && strings.Trim(line, "─") == ""
}

// talkChoices is how many choices a pickable prompt has: its question's options, or Yes and No.
func talkChoices(asking *talkAsking) int {
	if asking.Kind == "question" {
		return len(asking.Questions[0].Options)
	}
	return talkPermissionNo
}

// talkStillAsks says whether session id still asks the prompt it began asking at since. Called without the lock.
func (d *Deck) talkStillAsks(id string, since int64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.sessions.Get(id)
	if s == nil {
		return false
	}
	asking := talkAskingOf(s)
	return asking != nil && asking.Since == since
}

// POST /talk/pick?session=&n=&since=: a tap on a labelled button in the office's own window, the window's key from
// this computer only, never a phone's (phoneroutes.go). The query says everything; there is no body.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/talk/pick", access: accessWindow, refusal: talkRefusal,
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			writeStatus(w)(d.talkPick(session, r.URL.Query()))
		}})
}

// talkPick answers POST /talk/pick for session id.
func (d *Deck) talkPick(id string, query url.Values) (int, string) {
	n, err := strconv.Atoi(query.Get("n"))
	if err != nil {
		return http.StatusBadRequest, talkNoChoice
	}
	since, err := strconv.ParseInt(query.Get("since"), 10, 64)
	if err != nil || since <= 0 {
		return http.StatusBadRequest, "Unknown prompt"
	}
	d.mu.Lock()
	if hire, ok := d.t3HireAtDesk(id); ok {
		d.mu.Unlock()
		return http.StatusConflict, t3NoControls(calledHire(hire))
	}
	s := d.sessions.Get(id)
	if s == nil || s.Tab == "" {
		d.mu.Unlock()
		return http.StatusNotFound, "That agent has gone."
	}
	if s.Headless() {
		d.mu.Unlock()
		return http.StatusConflict, headlessNoControls
	}
	// A test copy does nothing in the owner's terminal tabs, only in its own hires' tmux panes (RWTC N1).
	if why := testCopyKeepsTab(s.Tab); why != "" {
		d.mu.Unlock()
		return http.StatusConflict, why
	}
	asking := talkAskingOf(s)
	if asking == nil || asking.Since != since {
		d.mu.Unlock()
		return http.StatusConflict, choiceGone
	}
	if !asking.Pickable {
		d.mu.Unlock()
		return http.StatusConflict, talkUseTheKeys
	}
	if n < 1 || n > talkChoices(asking) {
		d.mu.Unlock()
		return http.StatusBadRequest, talkNoChoice
	}
	tab, agent := s.Tab, s.Agent
	reject := asking.Kind == "permission" && n == talkPermissionNo
	key := agent.Reject()
	if !reject {
		var ok bool
		if key, ok = agent.Pick(n); !ok {
			d.mu.Unlock()
			return http.StatusConflict, talkUseTheKeys
		}
	}
	// One pick per prompt: a tap that lands twice, or the window's second look, presses nothing more.
	if !d.claimTalkPick(id, since) {
		d.mu.Unlock()
		return http.StatusNoContent, ""
	}
	d.mu.Unlock()

	// Asked something else, or nothing, while this waited for the worker: looked at again right before the key, as
	// the phone's words are, and on the screen, so a digit never lands in the agent's own box and an Escape never
	// stops its turn or a running tool.
	gone, unread, looked, menu := false, false, false, talkCantTell
	status, message := d.toAgent(tab, agent, func(t Tab) error {
		if !d.talkStillAsks(id, since) {
			gone = true
			return errTalkChoiceGone
		}
		lines, err := talkScreenOf(t, agent)
		switch {
		case errors.Is(err, errAgentBehind), errors.Is(err, errAgentGone):
			return err
		case err != nil:
			unread = true
			return errTalkChoiceGone
		}
		looked = true
		if menu = talkMenuOn(lines, asking); menu != talkThisMenu {
			talkMenuMissed(menu, lines)
			return errTalkChoiceGone
		}
		return t.Press(key)
	})
	if gone || unread || looked && menu != talkThisMenu || status >= http.StatusBadRequest {
		// A pick that didn't go doesn't count, so the next tap goes.
		d.mu.Lock()
		if d.talkPicks[id] == since {
			delete(d.talkPicks, id)
		}
		// Its menu put away, or answered, in its own terminal, which no hook says, and only as its input box shows
		// it: the desk no longer asks it. Waiting for a prompt, it's idle; with the tool allowed there running, it
		// thinks, keeping its tool and its turn till the tool's own hook.
		if s := d.sessions.Get(id); looked && s != nil && (menu == talkBoxWaits || menu == talkBoxWorks) {
			if asking := talkAskingOf(s); asking != nil && asking.Since == since {
				// At 0, so the desk's last event keeps its time: a hook raised before this look, still on its way
				// (a parallel prompt's), then lands and can bring asking back, never be dropped as older (RT126P3c N2).
				stopped := false
				if menu == talkBoxWaits {
					stopped = d.sessions.StoppedAsking(id, 0)
				} else {
					stopped = d.sessions.AnsweredInItsTerminal(id, 0)
				}
				if stopped {
					d.changed()
				}
			}
		}
		d.mu.Unlock()
		switch {
		case gone:
			return http.StatusConflict, choiceGone
		case unread, looked && menu == talkCantTell:
			// Maybe a menu it can't make out: the four keys answer it.
			return http.StatusConflict, talkUseTheKeys
		case looked && menu != talkThisMenu:
			return http.StatusConflict, choiceGone
		}
		return status, message
	}
	// Claude Code ends the turn when its prompt is put away, and no hook says so: see StoppedAsking.
	if reject {
		d.mu.Lock()
		if d.sessions.StoppedAsking(id, nowMillis()) {
			d.changed()
		}
		d.mu.Unlock()
	}
	return status, message
}

// claimTalkPick takes the pick of the prompt session id began asking at since, if none was taken: false for a
// second. Prompts no longer asked are forgotten as it goes. With the lock held.
func (d *Deck) claimTalkPick(id string, since int64) bool {
	if d.talkPicks == nil {
		d.talkPicks = map[string]int64{}
	}
	for other, at := range d.talkPicks {
		if s := d.sessions.Get(other); s == nil || talkAskingOf(s) == nil || talkAskingOf(s).Since != at {
			delete(d.talkPicks, other)
		}
	}
	if d.talkPicks[id] == since {
		return false
	}
	d.talkPicks[id] = since
	return true
}
