package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
		status, message = http.StatusNotFound, gmFireNoHire
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
	var kept string
	var err error
	if h.ViaLauncher {
		kept, err = d.fireLauncherHire(h, options, force)
	} else {
		kept, err = d.fireTmuxHire(h, force, options.KeepWorktree)
	}
	if err != nil {
		var refused *hireRefusal
		if errors.As(err, &refused) {
			refuse(refused.Status, refused.Message)
		} else {
			refuse(http.StatusInternalServerError, err.Error())
		}
		return
	}
	d.afterGMFire(h, launcher)
	if kept != "" && asked["removeWorktree"] == true { // fire.json: 200 only when removal was asked and the worktree had to stay
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "kept": kept, "worktree": h.Worktree, "branch": h.Branch})
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
