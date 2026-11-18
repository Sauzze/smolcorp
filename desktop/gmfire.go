package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
)

// POST /setup/gm/fire, as the Mac's Deck.fire(body:): {id, force?, removeWorktree?}. Found by registry id only; ended only by the office's fire
// (fireEngineHire or fireLauncherHire), whose refusals are the answers; nothing is typed, even with force.

const (
	gmFireNoHire  = "There's no hire with that id."
	gmFireNotSelf = "The GM can't fire itself. Ask the owner to fire it from Setup."
)

// gmAsked is a GM route's JSON body; the route table has already let only the GM or the page in.
func gmAsked(r *request) (asked map[string]any) {
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = json.Unmarshal(raw, &asked)
	return asked
}

func hireByID(id string) (Hire, bool) {
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.ID == id {
			return h, true
		}
	}
	return Hire{}, false
}

func (rt *router) gmFire(w http.ResponseWriter, r *request) {
	d, refuse := rt.deck, writeStatus(w)
	asked := gmAsked(r)
	id, _ := asked["id"].(string)
	d.mu.Lock()
	hiring, options := d.config.HiringEnabled, d.launcherEngine
	d.mu.Unlock()
	h, found := hireByID(id)
	status, message := 0, ""
	switch { // the GM's own token never fires the GM; the Mac has no such refusal, and the owner (the page) may
	case id == "":
		status, message = http.StatusBadRequest, "Send {id, force, removeWorktree}."
	case !hiring:
		status, message = http.StatusForbidden, hiringOff
	case !found:
		status, message = http.StatusNotFound, d.notAHire(id)
	case r.caller == callerGM && h.Role == gmRole:
		status, message = http.StatusConflict, gmFireNotSelf
	case gmT3FireRefusal(h) != "": // nothing is asked of it: no tmux or launcher is reached
		status, message = http.StatusConflict, gmT3FireRefusal(h)
	}
	if status != 0 {
		refuse(status, message)
		return
	}
	force := asked["force"] == true
	options.KeepWorktree = asked["removeWorktree"] != true // as the Mac: kept unless asked to remove
	launcher, _ := d.launcherOf(h.ID)
	var kept, note string
	var err error
	if h.ViaLauncher {
		if kept, err = d.fireLauncherHire(h, options, force); err == nil {
			note = heldFireNote(h)
		}
	} else {
		kept, note, err = d.fireTmuxHire(h, force, options.KeepWorktree)
	}
	if err != nil {
		var refused *hireRefusal
		if errors.As(err, &refused) && refused.Status == http.StatusOK { // its row already said gone: only its session was ended (gonedesk.go)
			d.afterGMFire(h, launcher)
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "note": refused.Message})
		} else if errors.As(err, &refused) {
			refuse(refused.Status, refused.Message)
		} else {
			refuse(http.StatusInternalServerError, err.Error())
		}
		return
	}
	d.afterGMFire(h, launcher)
	if note != "" { // marked fired with no window closed (hirefire_window.go): 200, saying so, and why a worktree stayed
		answer := map[string]any{"ok": true, "note": note}
		if kept != "" {
			answer["kept"], answer["worktree"], answer["branch"] = kept, h.Worktree, h.Branch
		}
		writeJSON(w, http.StatusOK, answer)
		return
	}
	if kept != "" && asked["removeWorktree"] == true { // fire.json: 200 only when removal was asked and the worktree had to stay
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "kept": kept, "worktree": h.Worktree, "branch": h.Branch})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// notAHire is a fire's answer for an id no hire has (FXGMNICK). A team row's "live:<session>" id names a session no
// hire works in, of any state: one the owner started by hand. That says what it is and how it stops, by the name the
// office gives it; the office's fire only where it can end that session's terminal (officeEnds). Any other id, and a
// session a hire fired or failed still works in, is gmFireNoHire.
func (d *Deck) notAHire(id string) string {
	session, live := strings.CutPrefix(id, "live:")
	if !live || session == "" {
		return gmFireNoHire
	}
	var found *liveSession
	for _, l := range d.frontDeskLive() {
		if l.ID == session {
			found = &l
		}
	}
	if found == nil {
		return gmFireNoHire
	}
	file := readRegistry(registryPath(), false)
	for _, h := range file.hires {
		if deskWorks(*found, h, file.hookKeys) {
			return gmFireNoHire
		}
	}
	d.mu.Lock()
	name := cmpOr(officeName(d.config.Layout.Room.Team, found.Project), lastPart(found.Project))
	ends := false
	if s := d.sessions.Get(session); s != nil {
		ends = d.officeEnds(s.Tab)
	}
	d.mu.Unlock()
	if name == "" {
		name = "That desk"
	}
	if ends {
		return name + " is a session you started yourself, not a hire, so the General Manager can't fire it. To stop it, quit it in its own window, or hold its desk in the office and fire it."
	}
	return name + " is a session you started yourself, not a hire, so the General Manager can't fire it. To stop it, quit it in its own window."
}

// officeEnds is whether the office's fire on a desk can end the session in tab, with the lock held: a launcher's window
// still connected, on every system; under the Mac app, any other tab the app says it can close (as the office shows
// its fire, addMacControls: a Terminal, iTerm2 or Ghostty tab too); elsewhere a tmux, WezTerm or kitty pane, away from
// Windows (where nothing tells what is in front of a pane, so Close leaves it). Any other terminal, the office answers
// that it can't.
func (d *Deck) officeEnds(tab string) bool {
	kind, id, _ := splitTab(tab)
	if kind == "launcher" {
		_, connected := d.launchers[id]
		return connected
	}
	if d.mac.host != nil {
		return d.mac.abilities[tab].Fire
	}
	switch kind {
	case "tmux", "wezterm", "kitty":
		return runtime.GOOS != "windows"
	}
	return false
}

// afterGMFire is what the office's fire does once a hire is gone (GM token stops, waiting mail fails, tab goes); launcher is taken before the fire.
func (d *Deck) afterGMFire(h Hire, launcher string) {
	if h.Role == gmRole {
		d.clearFiredGMToken()
	}
	if d.mail != nil {
		d.mail.failTo(h.ID)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for token, pending := range d.hires {
		if pending.hireID == h.ID {
			delete(d.hires, token)
		}
	}
	if d.sessions.Prune(func(tab string) bool {
		kind, id, socket := splitTab(tab)
		return !(kind == "launcher" && launcher != "" && id == launcher || kind == "tmux" && ownTmuxTab(h, socket))
	}, nowMillis()) {
		d.changed()
	}
}
