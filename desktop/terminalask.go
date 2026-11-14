package desktop

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// A terminal asks for a key (SANDBOX-PLAN §3.1 and the owner's B3, SK2). No file holds a key that opens anything, so
// a program you start yourself in a terminal gets one only by asking Smolcorp, and only once you say yes in
// Smolcorp's own window:
//   - `smolcorp gm login`, a General Manager you start yourself (under the Mac app, with "Allow a General Manager
//     started by hand" on), is given a new GM token, at each start;
//   - `smolcorp demo`, `smolcorp status` and `smolcorp desks` are given a key that opens what a tool's opens, until
//     Smolcorp stops: the demo's office, and the owner's reads of the team.
//
// The terminal sends POST /terminal/ask and gets a short code to print (4-7-2-9) and a ticket only it holds. Setup
// shows the ask, with the folder the terminal says it runs in and the same code: Allow is offered only once you've
// ticked that the code in the terminal is the same. The terminal waits with GET /terminal/ask?ticket=…, and the key
// comes back on that ticket once, to that process, which keeps it in memory. Any program of yours can ask; none can
// see another ask's code or ticket, or answer one. Only Smolcorp's own window, where there is one, and the paired
// phone may answer, never a browser's Setup, whose cookie other programs can read, or whose link they can use first
// (the owner's B8 STRICT, 13 Nov). So under the Mac app its Settings window answers; off a Mac nothing answers yet
// (Windows' office window has no way to, and the phone's route is FS3's), and a terminal's ask is refused at once,
// in words that say so, as Setup says it too (terminalAskOff).

// terminalAskPath is the terminal's route: POST asks, GET waits for the answer.
const terminalAskPath = "/terminal/ask"

// The kinds of ask, as internal/gm's AskGM and AskTool.
const (
	askGM   = "gm"
	askTool = "tool"
)

// terminalAsk is one ask, in memory only.
type terminalAsk struct {
	id     string // the page names the ask by it
	ticket string // the asking process's alone
	kind   string
	folder string // as the terminal said it: shown, never trusted
	code   string // four digits, unique among the asks waiting
	asked  time.Time
	answer string // "", askAnswering, "allow", "deny"
	key    string // on allow, until the terminal takes it
	done   chan struct{}
}

// terminalAsks are the asks waiting, or answered and not yet taken.
type terminalAsks struct {
	mu  sync.Mutex
	all []*terminalAsk
}

const (
	// askLifetime is how long an ask waits for you, and an answer for its terminal.
	askLifetime = 3 * time.Minute
	// asksMost is how many asks may wait at once: more are refused, so a program can't bury yours in its own.
	asksMost = 3
	// askFolderMost is the most of a folder's path that is kept and shown.
	askFolderMost = 300
)

// askAnswering marks an ask the owner is answering: no second answer finds it (a double click on Allow, or a
// Deny racing it), and its terminal is still told "waiting".
const askAnswering = "answering"

// askWait is how long one GET waits for an answer before saying "waiting": a variable, for the tests.
var askWait = 20 * time.Second

// askCode is four random digits.
func askCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(10_000))
	return fmt.Sprintf("%04d", n.Int64())
}

// askCodeShown is a code as the terminal and the window show it: 4-7-2-9.
func askCodeShown(code string) string { return strings.Join(strings.Split(code, ""), "-") }

// askFolder is the folder as it is shown: printable, one line, cut short.
func askFolder(folder string) string {
	folder = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, folder)
	if len([]rune(folder)) > askFolderMost {
		folder = string([]rune(folder)[:askFolderMost]) + "…"
	}
	return folder
}

// prune drops what ran out. The caller holds a.mu.
func (a *terminalAsks) prune(now time.Time) {
	a.all = slices.DeleteFunc(a.all, func(ask *terminalAsk) bool { return now.Sub(ask.asked) > askLifetime })
}

// add makes an ask, or says why not.
func (a *terminalAsks) add(kind, folder string) (*terminalAsk, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	a.prune(now)
	waiting := 0
	for _, ask := range a.all {
		if ask.answer == "" {
			waiting++
		}
	}
	if waiting >= asksMost {
		return nil, "Smolcorp already has asks waiting for you: answer them in Smolcorp's window first."
	}
	ask := &terminalAsk{id: newID(), ticket: randomKey(), kind: kind, folder: askFolder(folder), asked: now, done: make(chan struct{})}
	// A code no other ask has, so a program can't ask until its code is yours.
	for ask.code = askCode(); slices.ContainsFunc(a.all, func(other *terminalAsk) bool { return other.code == ask.code }); {
		ask.code = askCode()
	}
	a.all = append(a.all, ask)
	return ask, ""
}

// waiting is the asks waiting for an answer, as the page shows them: never a ticket or a key.
func (a *terminalAsks) waiting() []map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prune(time.Now())
	var shown []map[string]any
	for _, ask := range a.all {
		if ask.answer == "" {
			shown = append(shown, map[string]any{"id": ask.id, "kind": ask.kind, "folder": ask.folder,
				"code": askCodeShown(ask.code), "askedAt": ask.asked.UnixMilli()})
		}
	}
	return shown
}

// find is the ask waiting with id and code, or nil. The caller holds a.mu.
func (a *terminalAsks) find(id, code string) *terminalAsk {
	code = strings.ReplaceAll(code, "-", "")
	for _, ask := range a.all {
		if ask.answer == "" && constantTimeEqual(ask.id, id) && constantTimeEqual(ask.code, code) {
			return ask
		}
	}
	return nil
}

// byTicket is the ask with ticket, or nil. The caller holds a.mu.
func (a *terminalAsks) byTicket(ticket string) *terminalAsk {
	var found *terminalAsk
	for _, ask := range a.all {
		if ticket != "" && constantTimeEqual(ask.ticket, ticket) {
			found = ask
		}
	}
	return found
}

// claim is the ask waiting with id and code, marked as being answered so no other answer finds it; nil if none.
func (a *terminalAsks) claim(id, code string) *terminalAsk {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prune(time.Now())
	ask := a.find(id, code)
	if ask != nil {
		ask.answer = askAnswering
	}
	return ask
}

// release puts back an ask whose answer couldn't be given, to wait as before.
func (a *terminalAsks) release(ask *terminalAsk) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ask.answer == askAnswering {
		ask.answer = ""
	}
}

// settle gives a claimed ask its answer, and the key on allow.
func (a *terminalAsks) settle(ask *terminalAsk, answer, key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ask.answer != askAnswering {
		return
	}
	ask.answer, ask.key = answer, key
	close(ask.done)
}

// take is what the terminal holding ticket is told: "waiting", "allowed" with its key (once: the ask goes), or
// "denied". Not found: it ran out, was taken, or never was.
func (a *terminalAsks) take(ticket string) (state, key string, found bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prune(time.Now())
	ask := a.byTicket(ticket)
	if ask == nil {
		return "", "", false
	}
	switch ask.answer {
	case "", askAnswering:
		return "waiting", "", true
	case "deny":
		a.all = slices.DeleteFunc(a.all, func(other *terminalAsk) bool { return other == ask })
		return "denied", "", true
	}
	key, ask.key = ask.key, ""
	a.all = slices.DeleteFunc(a.all, func(other *terminalAsk) bool { return other == ask })
	return "allowed", key, true
}

// doneOf is the channel closed once the ask with ticket is answered, or nil.
func (a *terminalAsks) doneOf(ticket string) <-chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ask := a.byTicket(ticket); ask != nil {
		return ask.done
	}
	return nil
}

// terminalAskRoute is POST and GET /terminal/ask, from this computer only.
func (d *Deck) terminalAskRoute(w http.ResponseWriter, r *request) {
	switch r.Method {
	case http.MethodPost:
		// A web page can't send this header to another site, so no page you visit can ask.
		if r.Header.Get(setupHeader) != "1" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		kind := r.URL.Query().Get("kind")
		if kind != askGM && kind != askTool {
			writeStatus(w)(http.StatusBadRequest, "Say kind=gm or kind=tool.")
			return
		}
		// Off a Mac nothing may answer it yet (the owner's B8 STRICT): said now, not after three minutes of waiting.
		if kind == askTool && d.macHost() == nil {
			writeStatus(w)(http.StatusForbidden, terminalAskOff(runtime.GOOS))
			return
		}
		if kind == askGM {
			d.mu.Lock()
			allowed := d.byHandAllowed()
			d.mu.Unlock()
			if !allowed {
				writeStatus(w)(http.StatusForbidden, gmLoginOff(d.macHost() != nil))
				return
			}
		}
		ask, why := d.asks.add(kind, r.URL.Query().Get("folder"))
		if ask == nil {
			writeStatus(w)(http.StatusTooManyRequests, why)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ticket": ask.ticket, "code": askCodeShown(ask.code),
			"expiresIn": int(askLifetime.Seconds()), "where": askWhere(d.macHost() != nil)})
	case http.MethodGet:
		ticket := r.URL.Query().Get("ticket")
		if done := d.asks.doneOf(ticket); done != nil {
			select {
			case <-done:
			case <-time.After(askWait):
			case <-r.Context().Done():
			}
		}
		state, key, found := d.asks.take(ticket)
		if !found {
			writeStatus(w)(http.StatusNotFound, "This ask ran out or was already answered. Ask again.")
			return
		}
		answer := map[string]any{"state": state}
		if key != "" {
			answer["key"] = key
		}
		writeJSON(w, http.StatusOK, answer)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// gmLoginOff is why `smolcorp gm login` isn't taken.
func gmLoginOff(mac bool) string {
	if !mac {
		return "A General Manager you start yourself is the Mac's only: here, hire your General Manager from Setup."
	}
	return "Turn on Allow a General Manager started by hand in Smolcorp's Settings, Hires and roles, then run smolcorp gm login again."
}

// terminalAskOff is why, off a Mac, no terminal is given a key, as the terminal and Setup say it: only Smolcorp's own
// window, where there is one, and your paired phone may say yes to it (the owner's B8 STRICT), and neither can yet.
func terminalAskOff(goos string) string {
	who := "only your paired phone may let a command in a terminal into Smolcorp, and it can't yet: that comes in a later update."
	if goos == "windows" {
		who = "only Smolcorp's own window or your paired phone may let a command in a terminal into Smolcorp, and neither can yet: that comes in a later update."
	}
	return "On this computer, " + who + " A setup page in a browser can't, as another program could be using it. " +
		"Until then, smolcorp demo, status and desks don't run here: see your team and set things up in Setup (the tray icon, then Open Setup…)."
}

// askWhere is where the terminal tells you to look.
func askWhere(mac bool) string {
	if mac {
		return "Smolcorp's Settings (menu bar icon, then Settings…)"
	}
	return "Smolcorp's setup page (the tray icon, then Open Setup…)"
}

// answerTerminalAsk is POST /setup/terminal/answer?id=…&code=…&answer=allow|deny, the page's alone, and only Smolcorp's
// own Settings window's, under the Mac app: never a browser's Setup, and off a Mac nobody's (terminalAskOff). The code
// is the one the page showed, which you ticked as the terminal's.
func (rt *router) answerTerminalAsk(w http.ResponseWriter, r *request) {
	d := rt.deck
	if d.macHost() == nil {
		writeStatus(w)(http.StatusForbidden, terminalAskOff(runtime.GOOS))
		return
	}
	if !d.isWindowPage(r.Request) {
		writeStatus(w)(http.StatusForbidden, "Answer this in Smolcorp's own Settings window, not in a browser.")
		return
	}
	query := r.URL.Query()
	answer := query.Get("answer")
	if answer != "allow" && answer != "deny" {
		writeStatus(w)(http.StatusBadRequest, "Say answer=allow or answer=deny.")
		return
	}
	// Claimed under the asks' lock: a second Allow, or a Deny racing this one, finds nothing (404), so the GM's token
	// is rotated once, and only for the answer that hands it out.
	ask := d.asks.claim(query.Get("id"), query.Get("code"))
	if ask == nil {
		writeStatus(w)(http.StatusNotFound, "This ask ran out, or was answered already.")
		return
	}
	if answer == "deny" {
		d.asks.settle(ask, "deny", "")
		writeJSON(w, http.StatusOK, map[string]any{"answered": "deny"})
		return
	}
	key, why := d.keyForAsk(ask.kind)
	if key == "" {
		d.asks.release(ask)
		writeStatus(w)(http.StatusConflict, why)
		return
	}
	d.asks.settle(ask, "allow", key)
	writeJSON(w, http.StatusOK, map[string]any{"answered": "allow"})
}

// toolKeysMost is how many terminals may hold a tool's key at once: an older one stops working.
const toolKeysMost = 4

// keyForAsk is the key an allowed ask is given, or "" and why not.
func (d *Deck) keyForAsk(kind string) (string, string) {
	if kind == askTool {
		key := randomKey()
		d.mu.Lock()
		d.access.toolKeys = append(d.access.toolKeys, key)
		if len(d.access.toolKeys) > toolKeysMost {
			d.access.toolKeys = d.access.toolKeys[len(d.access.toolKeys)-toolKeysMost:]
		}
		d.mu.Unlock()
		return key, ""
	}
	// A General Manager you start yourself gets a new token at each start: one an earlier start held stops working.
	d.mu.Lock()
	allowed := d.byHandAllowed()
	d.mu.Unlock()
	if !allowed {
		return "", gmLoginOff(d.macHost() != nil)
	}
	// While the records' key can't be read no new token is kept (gmTokenKeeper.writeNow), so none is given: refused
	// with Setup's reason, as a rotation in Setup is (RT126SK1R4 follow-up 3), and the token from before goes on
	// working, now and after the restart.
	if recordsHeldOnly() {
		return "", recordsHeldAnswer
	}
	if !d.rotateGMToken() {
		return "", "The General Manager's new token couldn't be saved. Try again."
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.gmToken, ""
}
