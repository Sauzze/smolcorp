package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// A General Manager started by hand, and a new GM token, under the Mac app, as the Swift engine had them
// (GMIdentity.swift, DeckRouter.swift): POST /setup/gm/by-hand and POST /setup/gm/token/rotate, the page's
// alone. The GM's token is still made at each GM hire, kept in memory (frontdeskmail.go) and in the Keychain item
// (gmtokenstore.go); with "Allow a General Manager started by hand" on, it is also kept in <support>/gm-token,
// readable by you alone, where `smolcorp gm` reads it when $SMOLCORP_GM_TOKEN isn't set. The file is read back as
// the core starts, so a GM started by hand goes on working after a restart, and it wins over the Keychain's when the
// two differ: it is written at once at every change, while the Keychain's write comes after and may have failed (a
// fired GM's token never comes back from it). With the setting off the file is
// taken away, and so is the one an earlier build left. A GM that has used the token with the setting on is
// "started by hand" for setup, and the live session it named (X-Smolcorp-Session) has the GM's role, kept in
// <support>/gm-session (the id alone) until the GM is fired or the token changes.
//
// Linux and Windows have neither yet: their page and `smolcorp gm` say a GM gets its token at its hire.

// gmTokenFileName and gmSessionFileName are the files in the support folder, named as the Swift engine names them.
const (
	gmTokenFileName   = "gm-token"
	gmSessionFileName = "gm-session"
	// gmSessionHeader is the header a GM sends its own Claude Code session's id in (GMIdentity.sessionHeader).
	gmSessionHeader = "X-Smolcorp-Session"
)

// gmByHandState is what is known of a GM started by hand, in memory, beside the token.
type gmByHandState struct {
	// seen: the token has worked with the setting on since the core started or the token last changed.
	seen bool
	// session is the live session the GM said it runs in, while seen; remembered is the id kept in gm-session,
	// which counts as the GM's only once a live session has exactly that id (after a restart).
	session, remembered string
}

func gmTokenPath() string   { return filepath.Join(supportDir, gmTokenFileName) }
func gmSessionPath() string { return filepath.Join(supportDir, gmSessionFileName) }

// byHandAllowed is whether a GM started by hand is allowed: under the Mac app, with the setting on. The caller holds d.mu.
func (d *Deck) byHandAllowed() bool { return d.mac.host != nil && d.config.GMByHand }

// validGMToken is a token as it's kept: 43 base64url characters (32 random bytes), else "".
func validGMToken(text string) string {
	token := strings.TrimSpace(text)
	if len(token) != 43 || strings.Trim(token, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != "" {
		return ""
	}
	return token
}

// validGMSession is a session id as it may come in the header: 1 to 64 letters, digits, "-" or "_", else "".
func validGMSession(text string) string {
	if text == "" || len(text) > 64 || strings.Trim(text, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != "" {
		return ""
	}
	return text
}

// loadByHandGM is the core's start, under the Mac app: with the setting on, the token kept in its file over the
// Keychain's, the session kept in its file, and a token made if there is none; with it off, the files taken away.
// The caller holds d.mu.
func (d *Deck) loadByHandGM() {
	if d.mac.host == nil {
		return
	}
	if d.config.GMByHand {
		if saved, err := os.ReadFile(gmTokenPath()); err == nil {
			if token := validGMToken(string(saved)); token != "" {
				d.gmToken = token
			}
		}
		if saved, err := os.ReadFile(gmSessionPath()); err == nil && d.gmToken != "" {
			d.gmHand.remembered = validGMSession(strings.TrimSpace(string(saved)))
		}
	}
	d.applyByHand()
}

// applyByHand puts the token file as the setting says: written with the token while it's allowed (making a
// token if there is none), gone while it isn't, with what was known of a GM started by hand. No token is made in
// place of one the Keychain had but couldn't read, as the Swift engine made none: until a new token or a GM hire,
// nothing is written over it. The caller holds d.mu.
func (d *Deck) applyByHand() error {
	if !d.byHandAllowed() {
		d.forgetByHand()
		_ = os.Remove(gmTokenPath())
		return nil
	}
	if d.gmToken == "" && !d.gmKept.readFailed() {
		d.gmToken = randomKey()
		d.keepGMToken()
	}
	return d.syncGMTokenFile()
}

// syncGMTokenFile writes the token to its file while a GM started by hand is allowed, and takes the file away
// otherwise. The caller holds d.mu.
func (d *Deck) syncGMTokenFile() error {
	if !d.byHandAllowed() || d.gmToken == "" {
		if err := os.Remove(gmTokenPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writePrivate(gmTokenPath(), []byte(d.gmToken+"\n"))
}

// forgetByHand: no GM started by hand is known any more, nor its session, whose file goes. The caller holds d.mu.
func (d *Deck) forgetByHand() {
	d.gmHand = gmByHandState{}
	_ = os.Remove(gmSessionPath())
}

// gmTokenChanged is called, under d.mu, once d.gmToken has been made or taken away: a GM started by hand with
// the old token is forgotten, and the file and the store follow. With the setting on there is always a token, so a
// GM can be started by hand at any time: one taken away (its GM fired, or its hire failed) is replaced by a new one.
func (d *Deck) gmTokenChanged() {
	d.forgetByHand()
	if d.gmToken == "" && d.byHandAllowed() {
		d.gmToken = randomKey()
	}
	if d.syncGMTokenFile() != nil {
		// A file that couldn't follow must not win at the next start with a token that is gone: without it, the store is read.
		_ = os.Remove(gmTokenPath())
	}
	d.keepGMToken()
}

// noteGM is a request the GM's token signed, under d.mu: with the setting on, the GM has started by hand, and a
// live session its header names is its session from now on. A header that names no live session changes nothing.
func (d *Deck) noteGM(header string) {
	if !d.byHandAllowed() {
		return
	}
	d.gmHand.seen = true
	id := validGMSession(header)
	if id == "" || d.sessions.Get(id) == nil {
		return
	}
	d.gmHand.session = id
	if d.gmHand.remembered != id {
		d.gmHand.remembered = id
		_ = writePrivate(gmSessionPath(), []byte(id+"\n"))
	}
}

// startedByHand is whether a GM started by hand has used the token with the setting on (or its session is back
// after a restart). The caller holds d.mu.
func (d *Deck) startedByHand() bool {
	d.restoreByHand()
	return d.byHandAllowed() && d.gmHand.seen
}

// byHandGMSession is the live session of a GM started by hand, or "": it has the GM's role, as the Swift engine
// gave it (Deck.byHandGMSession). The caller holds d.mu.
func (d *Deck) byHandGMSession() string {
	d.restoreByHand()
	if !d.byHandAllowed() || !d.gmHand.seen || d.gmHand.session == "" || d.sessions.Get(d.gmHand.session) == nil {
		return ""
	}
	return d.gmHand.session
}

// restoreByHand: after a restart, the remembered session counts as the GM started by hand once a live session has
// exactly that id, as if its token had just worked. Never another session, never with no token. The caller holds d.mu.
func (d *Deck) restoreByHand() {
	h := &d.gmHand
	if h.session == "" && h.remembered != "" && d.byHandAllowed() && d.gmToken != "" && d.sessions.Get(h.remembered) != nil {
		h.seen, h.session = true, h.remembered
	}
}

// movedByHandGM: the session took another's place in its tab (after /clear), and the GM goes with it. The caller holds d.mu.
func (d *Deck) movedByHandGM(old, replacement string) {
	if d.gmHand.session == old {
		d.gmHand.session = replacement
	}
	if d.gmHand.remembered == old {
		d.gmHand.remembered = replacement
		_ = writePrivate(gmSessionPath(), []byte(replacement+"\n"))
	}
}

// managerState is the setup state's manager, in the Mac's shape (DeckRouter.managerState): under the Mac app, the
// setting, whether a GM was started by hand, and where the token file is, for the page to show and copy (never the
// token). Elsewhere there is no GM started by hand and no token file, so those say so. The caller holds d.mu.
func (d *Deck) managerState() map[string]any {
	if d.mac.host == nil {
		return map[string]any{"enabled": d.config.ManagerEnabled, "gmByHand": false, "gmStartedByHand": false,
			"gmTokenFile": "", "gmTokenFileShown": ""}
	}
	return map[string]any{"enabled": d.config.ManagerEnabled, "gmByHand": d.config.GMByHand, "gmStartedByHand": d.startedByHand(),
		"gmTokenFile": gmTokenPath(), "gmTokenFileShown": trimmed(gmTokenPath())}
}

// gmRotated is what POST /setup/gm/token/rotate answers: that it was made, never the token, and what to do next.
var gmRotated = map[string]any{"rotated": true, "message": "The old token no longer works. Fire the General Manager and hire it again to give it the new one."}

// byHandRoutes answers POST /setup/gm/by-hand and POST /setup/gm/token/rotate under the Mac app, and says whether
// the request was one of them. Elsewhere they aren't served (404).
func (rt *router) byHandRoutes(w http.ResponseWriter, r *request) bool {
	d := rt.deck
	if r.Method != http.MethodPost || r.URL.Path != "/setup/gm/by-hand" && r.URL.Path != "/setup/gm/token/rotate" || d.macHost() == nil {
		return false
	}
	if r.URL.Path == "/setup/gm/by-hand" {
		d.mu.Lock()
		defer d.mu.Unlock()
		// Whether the GM's token is also kept in a file, for a GM started by hand: off by default. Works with the manager off.
		enabled := r.URL.Query().Get("enabled")
		if enabled != "1" && enabled != "0" {
			writeStatus(w)(http.StatusBadRequest, "Say enabled=1 or enabled=0.")
			return true
		}
		d.config.GMByHand = enabled == "1"
		d.config.save()
		_ = d.applyByHand()
		d.changed()
		writeJSON(w, http.StatusOK, map[string]any{"gmByHand": d.config.GMByHand})
		return true
	}
	if !d.rotateGMToken() {
		writeStatus(w)(http.StatusInternalServerError, "The new token couldn't be saved; the old one still works.")
		return true
	}
	writeJSON(w, http.StatusOK, gmRotated)
	return true
}

// rotateGMToken puts a new token in place of the old, which stops working at once: a running GM can't be given
// another, and a GM hired afterwards gets its own. No answer carries it. It is kept for the next run before it is
// given, as the Swift engine kept it (GMIdentity.make): written to the store at once, without d.mu, and with the
// setting on to the file too; if either can't be written, nothing changes and the old one still works, now and after
// a restart. It says whether the old one stopped working.
func (d *Deck) rotateGMToken() bool {
	d.mu.Lock()
	old := d.gmToken
	d.mu.Unlock()
	fresh := randomKey()
	if d.gmKept.writeNow(fresh) != nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.gmToken != old {
		// A GM was hired or fired meanwhile: its change stands, and the old token already doesn't work.
		d.keepGMToken()
		return true
	}
	d.gmToken = fresh
	if d.byHandAllowed() {
		if err := d.syncGMTokenFile(); err != nil {
			d.gmToken = old
			d.keepGMToken()
			return false
		}
	}
	d.forgetByHand()
	return true
}
