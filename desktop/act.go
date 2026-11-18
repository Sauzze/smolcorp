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
	// ⚙︎ in the office's corner, from its own window on this computer: setup opens, as from the tray.
	case "settings":
		if phoneID != officeWindowID {
			writeStatus(w)(http.StatusForbidden, "Settings open on your "+d.machine()+".")
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
