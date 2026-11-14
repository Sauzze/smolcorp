package desktop

import (
	"log"
	"slices"
	"strings"
	"time"
)

// A setup link used first by another program (SANDBOX-PLAN §3.8, SK2). Off a Mac, the tray opens Setup in the
// browser with the link on the browser's command line, which any program of yours can read (/proc/<pid>/cmdline),
// and nothing on this computer can show the owner a code a hire couldn't read too. So the race is kept short and
// made seen: the link works once, for a minute (setupTokenLifetime); the core remembers each used token and the
// session it opened; and when a second request brings a token already used, the session it opened ends at once,
// and the page that brought it says "Another program on this computer opened Setup with this link first", with
// what that session changed. Smolcorp can't tell which of the two was the owner's browser, so the session ends
// either way, and the owner opens Setup again from the menu.
//
// The first user can't hide that report (RT126SK2 B1). A used token stays used, with what its session had changed,
// until it would have been forgotten anyway: every later use is told the same, so presenting the token again
// itself, before the owner's browser arrives, ends its own session and leaves the 409 waiting for the owner. And the
// report doesn't wait on the owner's browser at all: it is kept in the core (usedFirstReport), shown at the top of
// every Setup page and in Smolcorp's menu, and only the owner's own channel clears it, never a page a link opened:
// off a Mac the tray's "I've seen it", under the Mac app Smolcorp's own Settings window.

// setupSession is one of the setup page's sessions, in memory only.
type setupSession struct {
	id string
	// token is the link's token that opened it, "" for Smolcorp's own window (core_darwin.go's setup-session).
	token string
	// window: opened by Smolcorp's own window, never by a link a browser was handed.
	window bool
	// changes is what it changed, as "POST /setup/hooks", oldest first, at most setupChangesKept.
	changes []string
}

// usedSetupToken is a token already used: the session it opened, kept until a while after it would have run out.
// ended: a second use came and that session ended; changes is what it had changed by then, said to every later use.
type usedSetupToken struct {
	session string
	until   time.Time
	ended   bool
	changes []string
}

// usedFirstReport is the warning that a link was used first, until the owner clears it in their own channel: when the
// last one came, how many, and what the sessions ended for it had changed, in plain words.
type usedFirstReport struct {
	at      time.Time
	times   int
	changes []string
}

// setupChangesKept is the most changes a session remembers for the "used first" report.
const setupChangesKept = 50

// usedTokenKept is how long a used token is remembered past its lifetime: the owner's browser may come late.
const usedTokenKept = 10 * time.Minute

// endedSessionKept is how long an ended session's cookie is told why, rather than only refused.
const endedSessionKept = time.Hour

// setupLinkUsedFirst is what the page that brings a token already used says.
const setupLinkUsedFirst = "Another program on this computer opened Setup with this link first. Smolcorp ended that session at once. Open Setup again from Smolcorp's menu."

// usedFirstClearElsewhere is what a page that may not clear the warning is told: where the owner clears it.
func usedFirstClearElsewhere(mac bool) string {
	if mac {
		return "Clear this in Smolcorp's own Settings window (menu bar icon, then Settings…), not in a browser: the program that used the link could be this page."
	}
	return "Clear this from Smolcorp's menu (the tray icon, then I've seen it), not here: the program that used the link could be this page."
}

// setupSessionEnded is what a page whose session ended that way is told, in the 401's header: its link was used twice.
const setupSessionEnded = "used-first"

// setupSessionOf is the open session with id, or nil.
func (a *Access) setupSessionOf(id string) *setupSession {
	var found *setupSession
	for i := range a.setupSessions {
		if constantTimeEqual(id, a.setupSessions[i].id) {
			found = &a.setupSessions[i]
		}
	}
	return found
}

// IsWindowSession is whether id is an open session Smolcorp's own window opened.
func (a *Access) IsWindowSession(id string) bool {
	s := a.setupSessionOf(id)
	return s != nil && s.window
}

// noteTokenUsed remembers that token opened session.
func (a *Access) noteTokenUsed(token, session string) {
	now := time.Now()
	if a.usedSetupTokens == nil {
		a.usedSetupTokens = map[string]usedSetupToken{}
	}
	for used, kept := range a.usedSetupTokens {
		if now.After(kept.until) {
			delete(a.usedSetupTokens, used)
		}
	}
	a.usedSetupTokens[token] = usedSetupToken{session: session, until: now.Add(setupTokenLifetime + usedTokenKept)}
}

// NoteSetupChange records what a page session changed (or read of a key), for the "used first" report, in its words.
func (a *Access) NoteSetupChange(id, method, path string) {
	s := a.setupSessionOf(id)
	if s == nil || s.window {
		return
	}
	what := setupChangeSaid(method, path)
	if len(s.changes) > 0 && s.changes[len(s.changes)-1] == what {
		return // the same again, as a switch flipped twice: said once
	}
	s.changes = append(s.changes, what)
	if len(s.changes) > setupChangesKept {
		s.changes = s.changes[len(s.changes)-setupChangesKept:]
	}
}

// UsedFirst is whether candidate is a link's token that was already used: then the session it opened ends, and
// what that session changed is said. The token stays used, and every later use is told the same changes, until the
// token would have been forgotten anyway (usedTokenKept): the first user can't use it again to hide the report.
func (a *Access) UsedFirst(candidate string) (changes []string, used bool) {
	if candidate == "" {
		return nil, false
	}
	at := ""
	for token, kept := range a.usedSetupTokens {
		if constantTimeEqual(candidate, token) && time.Now().Before(kept.until) {
			at = token
		}
	}
	if at == "" {
		return nil, false
	}
	kept := a.usedSetupTokens[at]
	if !kept.ended {
		if s := a.setupSessionOf(kept.session); s != nil {
			kept.changes = slices.Clone(s.changes)
		}
		kept.ended = true
		a.usedSetupTokens[at] = kept
		a.endSetupSession(kept.session)
		a.noteUsedFirst(kept.changes)
	}
	return slices.Clone(kept.changes), true
}

// noteUsedFirst keeps the warning for the owner, with what was changed, until they clear it.
func (a *Access) noteUsedFirst(changes []string) {
	if a.usedFirst == nil {
		a.usedFirst = &usedFirstReport{}
	}
	a.usedFirst.at = time.Now()
	a.usedFirst.times++
	for _, change := range changes {
		if !slices.Contains(a.usedFirst.changes, change) {
			a.usedFirst.changes = append(a.usedFirst.changes, change)
		}
	}
	if len(a.usedFirst.changes) > setupChangesKept {
		a.usedFirst.changes = a.usedFirst.changes[len(a.usedFirst.changes)-setupChangesKept:]
	}
}

// UsedFirstReport is the warning as Setup and the menu show it, or nil when there is none.
func (a *Access) UsedFirstReport() map[string]any {
	if a.usedFirst == nil {
		return nil
	}
	return map[string]any{"at": a.usedFirst.at.UnixMilli(), "times": a.usedFirst.times, "changed": append([]string{}, a.usedFirst.changes...)}
}

// ClearUsedFirst takes the warning away: the owner's own channel alone calls it (the tray, or the Mac's window).
func (a *Access) ClearUsedFirst() { a.usedFirst = nil }

// endSetupSession ends a session: its cookie opens nothing from now on, and is told why for a while.
func (a *Access) endSetupSession(id string) {
	a.setupSessions = slices.DeleteFunc(a.setupSessions, func(s setupSession) bool { return constantTimeEqual(s.id, id) })
	if a.endedSessions == nil {
		a.endedSessions = map[string]time.Time{}
	}
	now := time.Now()
	for ended, at := range a.endedSessions {
		if now.Sub(at) > endedSessionKept {
			delete(a.endedSessions, ended)
		}
	}
	a.endedSessions[id] = now
}

// EndedForUsedFirst is whether id is a session that ended because its link was used again.
func (a *Access) EndedForUsedFirst(id string) bool {
	if id == "" {
		return false
	}
	found := false
	for ended, at := range a.endedSessions {
		if constantTimeEqual(id, ended) && time.Since(at) <= endedSessionKept {
			found = true
		}
	}
	return found
}

// setupChangeWords is what a session did, as the "used first" report says it: plain words for what matters most, and
// the setting's own name for the rest.
var setupChangeWords = map[string]string{
	"GET /setup/viewer":             "read the portable office's link, which holds its key",
	"POST /setup/viewer/secret":     "saw the portable office's write secret",
	"POST /setup/viewer/rotate":     "changed the portable office's keys",
	"POST /setup/terminal/answer":   "answered a terminal's ask for a key",
	"POST /setup/pair":              "opened pairing for a new phone",
	"POST /setup/unpair":            "unpaired a phone",
	"POST /setup/phones/confirm":    "let in or revoked a phone from the list",
	"POST /setup/hooks":             "connected or disconnected your agents' reports",
	"POST /setup/grants/answer":     "answered an ask for your OK",
	"POST /setup/gm/token/rotate":   "made a new General Manager token",
	"POST /setup/gm/by-hand":        "changed Allow a General Manager started by hand",
	"POST /setup/gm/hire":           "hired someone",
	"POST /setup/gm/fire":           "fired someone",
	"POST /setup/hire":              "hired someone",
	"POST /setup/hire/gm":           "hired a General Manager",
	"POST /setup/guard":             "changed the hires' guard",
	"POST /setup/webhook":           "changed where events are sent (Tell other tools)",
	"POST /setup/wifityping":        "changed typing over Wi-Fi",
	"POST /setup/tailnet/serve":     "changed the office over Tailscale",
	"POST /setup/peers":             "changed Floors from other Macs",
	"POST /setup/peers/join":        "joined another Mac's floors",
	"POST /setup/keystrokes":        "changed keys in other apps",
	"POST /setup/summary/key":       "changed the summaries' API key",
	"POST /setup/claude-accounts":   "changed the Claude accounts",
	"POST /setup/accounts/login":    "signed in a Claude account",
	"POST /setup/hire/plan/approve": "approved a hiring plan",
}

// setupChangeSaid is one change in the report's words.
func setupChangeSaid(method, path string) string {
	if said, ok := setupChangeWords[method+" "+path]; ok {
		return said
	}
	return "changed " + strings.ReplaceAll(strings.TrimPrefix(path, "/setup/"), "/", " ") + " in Setup"
}

// setupReadsNoted are the reads the report counts: what hands the reader a key.
var setupReadsNoted = []string{"GET /setup/viewer"}

// usedFirstAnswer is POST /setup/session's answer to a token already used: 409, the words, and what changed.
func usedFirstAnswer(changes []string) map[string]any {
	if changes == nil {
		changes = []string{}
	}
	log.Printf("smolcorp: a setup link was used twice; the session it opened first was ended (%d change(s): %s)",
		len(changes), strings.Join(changes, ", "))
	return map[string]any{"usedFirst": true, "message": setupLinkUsedFirst, "changed": changes}
}
