package desktop

import "net/http"

// Talk Bubble, "say" (1.26): the words the office's own window types into a desk's agent, POST /talk/say.
// They go through typeText, phonetext.go, the code POST /dictate runs, so every rule has one copy: the text
// prepared and at most 2,000 characters, no "!" first, never while the desk asks (as the request comes and
// again right before the words), several lines as one paste only where the tab takes one, Return on its own
// after, through claimReturn, a T3 desk refused. Nothing is kept but the owner's words to the GM, from the Mac
// app's own window, in memory (ownerwords.go).

// POST /talk/say?session=: the office window's words typed into the session's agent, by the same code as /dictate.
// The window's key from this computer only, never a phone's (phoneroutes.go).
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/talk/say", access: accessWindow,
		refusal: func(d *Deck) string { return d.shownOnlyInTheWindow("The conversation") },
		serve: func(d *Deck, w http.ResponseWriter, r *request, session string) {
			body := decodeJSON(r.Body)
			// Send is the words and Return: the window's page says so, and an older one that doesn't gets it too.
			var send *bool
			if value, ok := body["send"].(bool); ok {
				send = &value
			}
			text := stringValue(body["text"])
			status, message := d.TypeWindowText(text, session, send)
			// The owner's words only from the Mac app's own window: never the office in a browser, nor a window off
			// a Mac (accessOwnerTap, ownerwords.go).
			d.heardOwner(textFromWindow, accessOwnerTap.lets(r, r.phoneID), session, text, status, message)
			writeStatus(w)(status, message)
		}})
}

// textFrom says who the words come from.
type textFrom int

const (
	// textFromPhone: a paired phone, with the phone-text switch on (POST /dictate).
	textFromPhone textFrom = iota
	// textFromWindow: the office's own window on this computer (POST /talk/say). It is the owner at this computer,
	// so the phone-text switch, which is about phones, doesn't ask (D2).
	textFromWindow
)

// The window's sentences. The phone's (phonetext.go) say "the deck"; the window has its bubble.
const (
	windowAnswerFirst  = "It's asking you something: answer it first, above."
	windowTypedNotSent = "Typed, not sent: it began asking you something, or a Return had just gone. Look at the desk."
	// typeInItsOwnWindow is a desk in a Mac terminal app (Terminal, iTerm2, Ghostty): typing there goes through
	// the clipboard and ⌘V, which brings that window forward (D3). The window never does.
	typeInItsOwnWindow = "The bubble can't type into a Terminal, iTerm2 or Ghostty tab: type in its own window."
	// oneLineOnly is said before anyone types, to a desk whose terminal (WezTerm, kitty) can't take a paste.
	oneLineOnly = "This desk's terminal takes one line at a time."
)

// answerFirst is what's said of a desk that asks you something.
func (from textFrom) answerFirst() string {
	if from == textFromWindow {
		return windowAnswerFirst
	}
	return answerFirst
}

// typedNotSent is what's said when the words went and Return didn't.
func (from textFrom) typedNotSent() string {
	if from == textFromWindow {
		return windowTypedNotSent
	}
	return typedNotSent
}

// macTerminalApp says whether a tab is in one of the Mac's terminal apps, which only the Mac app types into,
// through the clipboard: Terminal's (tty:), iTerm2's and Ghostty's.
func macTerminalApp(name string) bool {
	kind, _, _ := splitTab(name)
	return kind == "tty" || kind == "iterm" || kind == "ghostty"
}

// typeAim is the desk the words go to.
type typeAim struct {
	name  string
	agent Agent
	tab   Tab
}

// typeTarget is session id's desk as the words' rules see it, or the status and sentence that refuse it. The
// order is the phone's: a hire in T3 Code, no desk, a test copy's, asking, then (for the window only) a Mac
// terminal app and a headless run, and no terminal this app drives.
func (d *Deck) typeTarget(from textFrom, id string) (typeAim, int, string) {
	d.mu.Lock()
	// A hire in T3 Code's desk is not typed into: its own sentence, as /act's send and the Mac say it.
	if hire, ok := d.t3HireAtDesk(id); ok {
		d.mu.Unlock()
		return typeAim{}, http.StatusConflict, t3NoControls(calledHire(hire))
	}
	s := d.sessions.Get(id)
	if s == nil || s.Tab == "" {
		d.mu.Unlock()
		return typeAim{}, http.StatusNotFound, "Session not found"
	}
	aim, asking, headless, tab := typeAim{name: s.Tab, agent: s.Agent}, s.State == Asking, s.Headless(), d.tab(s.Tab)
	d.mu.Unlock()
	// A test copy types nothing into the owner's terminal tabs (RWTC N1).
	if why := testCopyKeepsTab(aim.name); why != "" {
		return typeAim{}, http.StatusConflict, why
	}
	// Asking you something, its menu would take the words: a digit picking an option, Return the one highlighted.
	if asking {
		return typeAim{}, http.StatusConflict, from.answerFirst()
	}
	if from == textFromWindow {
		if headless {
			return typeAim{}, http.StatusConflict, headlessNoControls
		}
		if macTerminalApp(aim.name) {
			return typeAim{}, http.StatusConflict, typeInItsOwnWindow
		}
	}
	if tab == nil {
		return typeAim{}, http.StatusConflict, notHere
	}
	aim.tab = tab
	return aim, 0, ""
}

// TypeWindowText is POST /talk/say: the office window's words typed into session id's agent, with Return unless
// send says not (the window says so: Send is the words and Return).
func (d *Deck) TypeWindowText(text, id string, send *bool) (int, string) {
	return d.typeText(textFromWindow, text, id, send)
}

// deskTyping is whether the window can type into a desk now, for the card GET /talk gives (`typing`): ok, whether
// several lines can go, and the sentence when something's wrong or limited. /talk/say checks everything again.
type deskTyping struct {
	OK    bool   `json:"ok"`
	Lines bool   `json:"lines"`
	Why   string `json:"why"`
}

// windowTyping is what the window may type into session id's desk now; false for a desk that isn't there.
func (d *Deck) windowTyping(id string) (deskTyping, bool) {
	aim, status, why := d.typeTarget(textFromWindow, id)
	if status == http.StatusNotFound {
		if d.deskState(id) == "" {
			return deskTyping{}, false
		}
		// A desk with no terminal of its own gets the sentence for a terminal this app can't type into.
		why = notHere
	}
	switch {
	case why != "":
		return deskTyping{Why: why}, true
	case !takesLinesAtAll(aim.name):
		return deskTyping{OK: true, Why: oneLineOnly}, true
	}
	return deskTyping{OK: true, Lines: true}, true
}
