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
// (gmtokenstore.go). No file holds it (1.26, SK2): with "Allow a General Manager started by hand" on, a GM you start
// yourself runs `smolcorp gm login`, and gets a new token from this process once you click Allow in Smolcorp's own
// window, after checking the code (terminalask.go). The gm-token file earlier versions kept it in is taken away at
// each start. A GM that has used the token with the setting on is "started by hand" for setup, and the live session
// it named (X-Smolcorp-Session) has the GM's role, kept in <support>/gm-session (the id alone, which opens nothing)
// until the GM is fired or the token changes.
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

// loadByHandGM is the core's start, under the Mac app: with the setting on, the session kept in its file, and a
// token made if there is none; with it off, the session's file taken away. The token file an earlier version kept is
// taken away either way, unread: a token anything could read opens nothing now. The caller holds d.mu.
func (d *Deck) loadByHandGM() {
	if d.mac.host == nil {
		return
	}
	if d.config.GMByHand {
		if saved, err := os.ReadFile(gmSessionPath()); err == nil && d.gmToken != "" {
			d.gmHand.remembered = validGMSession(strings.TrimSpace(string(saved)))
		}
	}
	d.applyByHand()
}

// applyByHand follows the setting: while it's allowed there is a token to give at a login (made if there is none);
// while it isn't, what was known of a GM started by hand goes. No token is made in place of one the Keychain had but
// couldn't read, as the Swift engine made none: until a new token or a GM hire, nothing is written over it. The
// token file of an earlier version goes either way. The caller holds d.mu.
func (d *Deck) applyByHand() error {
	if !d.byHandAllowed() {
		d.forgetByHand()
		return d.syncGMTokenFile()
	}
	// Not over the verifier of a hired GM's token from the last run: that GM goes on working, and a GM started by
	// hand needs a new token (rotate) to be given one, as the core no longer has the old one whole.
	if d.gmCheck() == "" && !d.gmKept.readFailed() {
		d.gmToken = randomKey()
		d.keepGMToken()
	}
	return d.syncGMTokenFile()
}

// syncGMTokenFile takes away the token file an earlier version wrote: no file holds the token, and none is read
// (SK2). It goes even while the records' key can't be read, where SK1F (nit 5) kept it for a GM started by hand to
// read after the restart: such a GM now asks again after every restart (smolcorp gm login), and a token left in the
// file could be a hired GM's that still works. The caller holds d.mu.
func (d *Deck) syncGMTokenFile() error {
	if err := os.Remove(gmTokenPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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
	// The token made now (or none) replaces any verifier kept from the last run.
	d.gmVerifier = ""
	if d.gmToken == "" && d.byHandAllowed() {
		d.gmToken = randomKey()
	}
	_ = d.syncGMTokenFile()
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
	if h.session == "" && h.remembered != "" && d.byHandAllowed() && d.gmCheck() != "" && d.sessions.Get(h.remembered) != nil {
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
// setting and whether a GM was started by hand (never the token). No file holds the token any more, so the file's
// fields are empty everywhere; the page says to start the GM with smolcorp gm login. The caller holds d.mu.
func (d *Deck) managerState() map[string]any {
	if d.mac.host == nil {
		return map[string]any{"enabled": d.config.ManagerEnabled, "gmByHand": false, "gmStartedByHand": false,
			"gmTokenFile": "", "gmTokenFileShown": ""}
	}
	return map[string]any{"enabled": d.config.ManagerEnabled, "gmByHand": d.config.GMByHand, "gmStartedByHand": d.startedByHand(),
		"gmTokenFile": "", "gmTokenFileShown": ""}
}

// gmRotated is what POST /setup/gm/token/rotate answers: that it was made, never the token, and what to do next.
var gmRotated = map[string]any{"rotated": true, "message": "The old token no longer works. Fire the General Manager and hire it again to give it the new one, or start it again with smolcorp gm login."}

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
		// Whether a GM started by hand may ask for a token, with `smolcorp gm login`: off by default. Works with the manager off.
		enabled := r.URL.Query().Get("enabled")
		if enabled != "1" && enabled != "0" {
			writeStatus(w)(http.StatusBadRequest, "Say enabled=1 or enabled=0.")
			return true
		}
		// Switching it off is a revocation: a GM started by hand is forgotten, with its session's role, and no login
		// may ask any more. Refused while the records' key can't be read, as every revocation is, since the setting
		// would come back at the restart (RT126SK1R4 follow-up 2).
		if refused := d.config.heldRefusal(func(c *Config) { c.GMByHand = enabled == "1" }); refused != "" {
			writeStatus(w)(http.StatusConflict, refused)
			return true
		}
		d.config.GMByHand = enabled == "1"
		d.config.save()
		_ = d.applyByHand()
		d.changed()
		writeJSON(w, http.StatusOK, map[string]any{"gmByHand": d.config.GMByHand})
		return true
	}
	// While the records' key can't be read, no new token is kept (gmTokenKeeper.writeNow): said with Setup's reason,
	// and the old one still works, now and after the restart (RT126SK1R4 follow-up 3).
	if recordsHeldOnly() {
		writeStatus(w)(http.StatusConflict, recordsHeldAnswer)
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
// setting on the old token file taken away; if the store can't be written, nothing changes and the old one still
// works, now and after a restart. It says whether the old one stopped working.
func (d *Deck) rotateGMToken() bool {
	d.mu.Lock()
	old := d.gmToken
	d.mu.Unlock()
	fresh := randomKey()
	if d.gmKept.writeNow(gmVerifierForm(hashKey(fresh))) != nil {
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
	_ = d.syncGMTokenFile()
	d.forgetByHand()
	return true
}
