package desktop

import (
	"errors"
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
// goes only while Claude Code's menu for that prompt is there (talkMenuOn). With no menu at all, the pick is 409
// choiceGone and the desk stops asking, so the window's input wakes; with another prompt's menu, 409 and nothing
// more. A digit thus never lands in the agent's input box, and an Escape never stops a running tool. Where the
// screen can't be read, the card isn't pickable (talkShowsScreen): the four keys answer it.

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
	talkNoMenu    talkMenu = iota // no menu of Claude Code's: put away, or answered, in its own terminal
	talkOtherMenu                 // a menu, but not that prompt's
	talkThisMenu                  // that prompt's own menu
)

// How Claude Code draws its menus (2.1): a permission prompt asks "Do you want to proceed?" (or about a file), its
// choices numbered under it, ❯ at the one chosen, and "Esc to cancel" under them; a question's text, its options
// numbered, and "Enter to select" under them. Either is the last thing on the screen while it asks.
const (
	talkProceed       = "Do you want to proceed?"
	talkEscToCancel   = "Esc to cancel"
	talkEnterToSelect = "Enter to select"
	// A footer is among the screen's last non-blank lines; a menu's choices and what it is about, this many lines
	// above it at most.
	talkFooterNear = 3
	talkMenuMost   = 40
	// How long a line of the screen must be, at least, to be taken for the start of a question or a command.
	talkMatchLeast = 8
)

var (
	talkCursorLine = regexp.MustCompile(`^❯\s*[1-9]\.\s`)
	talkFirstLine  = regexp.MustCompile(`^(❯\s*)?1\.\s`)
)

// talkMenuOn says what lines, the end of a desk's screen, show of asking: its own menu, another, or none.
// A permission prompt's menu: "Esc to cancel" at its foot, or "Do you want to proceed?" right over its ❯ on
// choice 1, with a ❯ on a choice and a choice 1; and, for a Bash command, the command's first line in its box. A
// question's: "Enter to select" at its foot, a choice 1, and the question's text over it.
func talkMenuOn(lines []string, asking *talkAsking) talkMenu {
	text := make([]string, len(lines))
	for i, line := range lines {
		text[i] = strings.Join(strings.Fields(line), " ")
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
		// rule of the input box's after it.
		proceed := lastIndexFunc(text, func(line string) bool { return strings.Contains(line, talkProceed) })
		if proceed < 0 || proceed < len(text)-talkMenuMost {
			return talkNoMenu
		}
		next := proceed + 1
		for next < len(text) && text[next] == "" {
			next++
		}
		if next == len(text) || !talkCursorLine.MatchString(text[next]) || slices.ContainsFunc(text[proceed:], talkRule) {
			return talkNoMenu
		}
		footer, kind, top = len(text), "permission", max(0, proceed-talkMenuMost)
	} else {
		top = max(0, footer-talkMenuMost)
	}
	menu := text[top:footer]
	// Its choices: a choice 1, and ❯ on one of them.
	first := lastIndexFunc(menu, talkFirstLine.MatchString)
	if first < 0 || !slices.ContainsFunc(menu[first:], talkCursorLine.MatchString) {
		return talkNoMenu
	}
	if kind != asking.Kind {
		return talkOtherMenu
	}
	// What it is about, over its first choice: the question's text, or the command, from its first line on screen.
	about := ""
	switch {
	case kind == "question" && len(asking.Questions) > 0:
		about = strings.TrimSuffix(asking.Questions[0].Question, "…")
	case kind == "permission":
		about = asking.command
	}
	if about = strings.Join(strings.Fields(about), " "); about == "" {
		return talkThisMenu
	}
	above := menu[:first]
	if kind == "permission" {
		// The command's box only: under the rule over it.
		above = above[lastIndexFunc(above, talkRule)+1:]
	}
	for _, line := range above {
		line = strings.TrimSuffix(line, "…")
		if len([]rune(line)) >= min(len([]rune(about)), talkMatchLeast) && (strings.HasPrefix(about, line) || strings.HasPrefix(line, about)) {
			return talkThisMenu
		}
	}
	return talkOtherMenu
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
	gone, stale, unread := false, false, false
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
		switch talkMenuOn(lines, asking) {
		case talkNoMenu:
			gone, stale = true, true
			return errTalkChoiceGone
		case talkOtherMenu:
			gone = true
			return errTalkChoiceGone
		}
		return t.Press(key)
	})
	if gone || unread || status >= http.StatusBadRequest {
		// A pick that didn't go doesn't count, so the next tap goes.
		d.mu.Lock()
		if d.talkPicks[id] == since {
			delete(d.talkPicks, id)
		}
		// Its menu put away, or answered, in its own terminal, which no hook says: the desk no longer asks it.
		if s := d.sessions.Get(id); stale && s != nil {
			if asking := talkAskingOf(s); asking != nil && asking.Since == since && d.sessions.StoppedAsking(id, nowMillis()) {
				d.changed()
			}
		}
		d.mu.Unlock()
		switch {
		case unread:
			return http.StatusConflict, talkUseTheKeys
		case gone:
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
