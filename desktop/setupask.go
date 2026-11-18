package desktop

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Setup opened by a tap on the phone (1.26, OLD-TAP1; the owner's Q-F14, "yes"). Off a Mac, the menu's link goes on
// the browser's command line, which any program of yours can read (setuplink.go), and nothing on this computer can
// show the owner a code another program couldn't read too: the only channel a hire can't read is another device. So
// once a phone is paired, Setup opens by a tap on it, and the link opens nothing (setupLinkGone): until the first
// phone, the menu's one-minute link stays, with its "used first" report.
//
// It is SK2's terminal ask (terminalask.go) with the browser as the terminal and the phone as the window. The setup
// page, opened with no link (/setup#tap), sends POST /setup/tap from this computer, with no key, and gets a code to
// show (4-7-2-9) and a ticket only that page holds, in its memory. The paired phone lists the ask in GET /grants,
// with this computer's name and the same code: "Open Setup on <computer>? code 4-7-2-9". The owner's tap on it,
// POST /grants/setup with the id and the code, is the only answer (accessOwnerTap: a paired phone over Tailscale, never
// the Wi-Fi, a browser's office, a window off a Mac, the GM, a tool, an MCP key, a hire or a night job). The page waits
// with GET /setup/tap?ticket=…, and on the owner's yes that answer, and only it, carries a new session's cookie,
// once: the ask goes as it is taken. Any program here can ask, and each ask has a code of its own; none can see
// another's ticket, nor answer one. An ask nobody answers runs out (askLifetime), and three may wait at once, one
// new every two seconds, so a program can't bury the owner's ask under its own. Nothing of it is written: the asks
// live in memory, and a restart drops them. Under the Mac app none of this is needed or taken: its Settings window
// opens Setup with no link.

// setupTapFragment is what the menu opens Setup with once its link is gone: the page asks for the tap at once.
const setupTapFragment = "tap"

// askSetup is a setup page's ask, as a terminal's are gm and tool.
const askSetup = "setup"

// setupAskEvery is how often a new setup ask is taken, from anyone: a variable, for the tests.
var setupAskEvery = 2 * time.Second

// setupAsks are the setup pages' asks, waiting or answered and not yet taken: SK2's asks, and when the last came.
type setupAsks struct {
	terminalAsks
	pace sync.Mutex
	last time.Time
}

// ask makes an ask, or says why not with its status: one every setupAskEvery, and at most asksMost waiting.
func (s *setupAsks) ask() (*terminalAsk, int, string) {
	s.pace.Lock()
	now := time.Now()
	if now.Sub(s.last) < setupAskEvery {
		s.pace.Unlock()
		return nil, http.StatusTooManyRequests, "Another page asked a moment ago. Try again in a few seconds."
	}
	s.last = now
	s.pace.Unlock()
	// The terminal's words name Smolcorp's window: a page is told where its asks are answered.
	ask, _ := s.add(askSetup, "")
	if ask == nil {
		return nil, http.StatusTooManyRequests, "Your phone already has asks to open Setup waiting: answer them there first, or wait for them to run out."
	}
	return ask, http.StatusOK, ""
}

// shown is the asks waiting, as the phone lists them: the id, the code and this computer's name, never a ticket.
func (s *setupAsks) shown() []map[string]any {
	rows := []map[string]any{}
	for _, ask := range s.waiting() {
		asked, _ := ask["askedAt"].(int64)
		rows = append(rows, map[string]any{"id": ask["id"], "code": ask["code"], "computer": setupComputerName(),
			"askedAt": asked, "expires": asked + askLifetime.Milliseconds()})
	}
	return rows
}

// setupComputerName is this computer's name, as the phone shows it: its host name, printable and cut short. A
// variable, for the tests.
var setupComputerName = func() string {
	name, _ := os.Hostname()
	name = askFolder(strings.TrimSuffix(name, ".local"))
	if name == "" {
		return "this computer"
	}
	return name
}

// setupLinkGone is whether a menu's link no longer opens Setup: off a Mac, once a phone is paired (the Windows
// window's own paired device is no phone). Setup then opens by a tap on the phone alone.
func (d *Deck) setupLinkGone() bool {
	if d.macHost() != nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return phonesOnly(d.access.Phones) > 0
}

// setupLinkGoneWords is what a link is told once a phone is paired.
const setupLinkGoneWords = "Your phone is paired, so on this computer Setup opens by a tap on it, never by a link: open Setup from the tray icon, then say yes on your phone."

// setupTapWhere is where the owner says yes, as the page is told.
const setupTapWhere = "your paired phone's Smolcorp page, over Tailscale's HTTPS address"

// setupTapRoute is POST and GET /setup/tap, from this computer only (server.go's /setup case), with the setup header.
// The path is under /setup/, so the cookie it hands over is set on 127.0.0.1, where Setup lives.
func (d *Deck) setupTapRoute(w http.ResponseWriter, r *request) {
	// A web page can't send this header to another site, so no page you visit can ask, or wait on a ticket.
	if r.Header.Get(setupHeader) != "1" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodPost:
		if d.macHost() != nil {
			writeStatus(w)(http.StatusForbidden, "On a Mac, Smolcorp's Settings window opens Setup: the menu bar icon, then Settings….")
			return
		}
		if !d.setupLinkGone() {
			writeStatus(w)(http.StatusConflict, "No phone is paired yet, so nothing can say yes: until one is, Setup opens from the tray icon's Open Setup….")
			return
		}
		ask, status, why := d.setupAsks.ask()
		if ask == nil {
			writeStatus(w)(status, why)
			return
		}
		d.Refresh() // the phone's page hears of it now
		writeJSON(w, http.StatusOK, map[string]any{"ticket": ask.ticket, "code": askCodeShown(ask.code),
			"expiresIn": int(askLifetime.Seconds()), "where": setupTapWhere})
	case http.MethodGet:
		ticket := r.URL.Query().Get("ticket")
		if done := d.setupAsks.doneOf(ticket); done != nil {
			select {
			case <-done:
			case <-time.After(askWait):
			case <-r.Context().Done():
			}
		}
		state, _, found := d.setupAsks.take(ticket)
		if !found {
			writeStatus(w)(http.StatusNotFound, "This ask ran out or was already answered. Ask again.")
			return
		}
		if state == "allowed" {
			// The owner's yes opens one session, for this answer alone: the ask went as it was taken.
			d.completeSetup()
			http.SetCookie(w, &http.Cookie{Name: setupCookie(), Value: d.newTappedSetupSession(), Path: "/setup", HttpOnly: true,
				SameSite: http.SameSiteStrictMode})
		}
		writeJSON(w, http.StatusOK, map[string]any{"state": state})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// newTappedSetupSession opens a page session for a browser the owner's tap let in: no link's token, and not the window's.
func (d *Deck) newTappedSetupSession() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.NewTappedSetupSession()
}

// The owner's tap. The phone reads the asks with the others, in GET /grants (grants.go), and answers here:
// POST /grants/setup?id=&code=&answer=allow|deny, the code the phone showed, which the owner ticked as the one on
// this computer's screen. The owner's tap alone (accessOwnerTap); everyone else is refused by the table.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/grants/setup", access: accessOwnerTap,
		refusal: says("Only your paired phone, over Tailscale's HTTPS address, opens Setup on this computer."),
		serve: func(d *Deck, w http.ResponseWriter, r *request, _ string) {
			query := r.URL.Query()
			answer := query.Get("answer")
			if answer != "allow" && answer != "deny" {
				writeStatus(w)(http.StatusBadRequest, "Say answer=allow or answer=deny.")
				return
			}
			// Claimed under the asks' lock: a second tap, or a no racing a yes, finds nothing (404).
			ask := d.setupAsks.claim(query.Get("id"), query.Get("code"))
			if ask == nil {
				writeStatus(w)(http.StatusNotFound, "This ask ran out, or was answered already.")
				return
			}
			d.setupAsks.settle(ask, answer, "")
			d.Refresh()
			writeJSON(w, http.StatusOK, map[string]any{"answered": answer})
		}})
}
