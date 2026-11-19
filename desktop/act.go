package desktop

import (
	"net/http"
	"runtime"
)

// act answers POST /act, past the router's own checks (handle's phoneID: a paired phone's key or the window's):
// a desk's keys, floors, styles, music, a hire's desk and role (Perform), and the office's own window's actions,
// each checked by hand here (N2). Its action is the query's, never a phone's free text.
func (d *Deck) act(w http.ResponseWriter, r *request, phoneID, session string) {
	switch action := r.URL.Query().Get("action"); action {
	// ⚙︎ in the office's corner, from its own window on this computer: setup opens, as from the tray. Under the Mac
	// app, Smolcorp's own Settings window, as the menu's Settings… opens it: Setup in a browser only looks on a Mac.
	case "settings":
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, "Settings open on your "+d.machine()+".")
			return
		}
		if h := d.macHost(); h != nil {
			if err := hostShowSettings(h); err != nil {
				writeStatus(w)(http.StatusBadGateway, "Couldn't open Smolcorp's Settings ("+err.Error()+"). Instead: "+askWhere(true, true)+".")
				return
			}
			writeStatus(w)(http.StatusNoContent, "")
			return
		}
		if err := openSetupPage(d); err != nil {
			writeStatus(w)(http.StatusBadGateway, "Couldn't open your browser ("+err.Error()+"). Instead: "+setupAdvice(runtime.GOOS))
			return
		}
		writeStatus(w)(http.StatusNoContent, "")
	// Take over in T3 Code and Hand back (EXPERIMENTAL, t3takeover.go), from a desk's card in the office's own window
	// on this computer, never a phone: the hire is found by its desk, never by anything sent.
	case "takeover", "handback":
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, "Take over in T3 Code is on your "+t3Here()+".")
			return
		}
		if d.officeInABrowser(r, phoneID) {
			writeStatus(w)(http.StatusForbidden, officeWindowOnly)
			return
		}
		status, answer, ok := d.takeOverAtDesk(action, session)
		if !ok {
			writeStatus(w)(http.StatusNotFound, "Only a hire Smolcorp runs can be taken over in T3 Code, or handed back.")
			return
		}
		if text, isText := answer.(string); isText {
			writeStatus(w)(status, text)
			return
		}
		writeJSON(w, status, answer)
	// Let go of a hire in T3 Code from its desk's card, after its confirm (t3desks.go): the lane's answer, as Setup's.
	case "letgo":
		if d.officeInABrowser(r, phoneID) {
			writeStatus(w)(http.StatusForbidden, officeWindowOnly)
			return
		}
		status, answer, ok := d.letGoAtDesk(session)
		if !ok {
			writeStatus(w)(d.Perform("letgo", session, r.URL.Query()))
			return
		}
		if text, isText := answer.(string); isText {
			writeStatus(w)(status, text)
			return
		}
		writeJSON(w, status, answer)
	default:
		writeStatus(w)(d.PerformFrom(phoneID, action, session, r.URL.Query()))
	}
}

// officeWindowOnly is what the office in a browser is told, under the Mac app, for what only its own window does.
const officeWindowOnly = "Do this in Smolcorp's own office window on your Mac, not in a browser."

// officeInABrowser is whether a request from the office (phoneID officeWindowID) comes from the office in a browser
// under the Mac app (RT126SWEEP3 P4 F3): its key is one the browser's history keeps, which other programs can read, so
// taking a hire over or letting it go, and opening a hire's window or T3 Code, are the Mac app's own window's alone
// (r.ownWindow). A phone isn't the office (it is let in or not as before), and off the Mac app nothing changes.
func (d *Deck) officeInABrowser(r *request, phoneID string) bool {
	return phoneID == officeWindowID && !r.ownWindow && d.macHost() != nil
}
