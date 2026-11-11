package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
)

// POST /setup/gm/hire, for the GM and the setup page (the route table gates the callers): the Mac's hire, through
// the same hire path as the office (hireWith), and only into a fenced way, never a plain terminal, which has no guard.
// With `engine: "t3"`, into T3 Code through Setup's own lane (hireFromSetupSheetIntoT3), as the Mac's route does.
// The request is the fixture Tests/fixtures/frontdesk/hire.json.

// gmHireCantHonour is the 400 for a field the Go path can't honour yet. Under the Mac app it's "this engine",
// whose way back is the Swift one.
func gmHireCantHonour(field string, macApp bool) string {
	if macApp {
		return "On this engine, the General Manager's hire can't honour \"" + field + "\" yet: leave it out, or switch back to the Swift engine."
	}
	return "Smolcorp on Linux and Windows can't honour \"" + field + "\" in a hire yet: leave it out."
}

// hiresOpenThroughMacApp is whether a hire would open through the Mac app now, read as hireWith reads it.
func (d *Deck) hiresOpenThroughMacApp() bool {
	terminals, _ := hireWays()
	d.mu.Lock()
	chosen := d.config.HireTerminal
	d.mu.Unlock()
	return underMacApp(hireTerminal(chosen, terminals))
}

// gmHireFolder checks the folder asked for as Setup's sheet checks it (checkedSetupFolder, the Mac's
// BriefRoutes.checkedFolder, which the Mac's route uses too): a folder that's there, inside your home folder
// but not the home folder itself, nor ~/Library or a hidden folder at its top. A folder further down a hidden
// one (~/Projects/.team/app) is a project's, as on the Mac. "" is none, which is the folder for new hires.
func gmHireFolder(given any) (folder string, why string) {
	text, ok := given.(string)
	if given != nil && !ok {
		return "", "The folder is text."
	}
	if strings.TrimSpace(text) == "" {
		return "", ""
	}
	folder, _, why = checkedSetupFolder(text, homeDir)
	return folder, why
}

// gmHire answers POST /setup/gm/hire.
func (d *Deck) gmHire(w http.ResponseWriter, r *request) {
	reply := writeStatus(w)
	var body map[string]any
	_ = json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body)
	role, _ := body["role"].(string)
	// One GM per office, before anything is made, as /setup/hire/gm does.
	if status, text, refused := gmHireRefusal(role, r.caller, readRegistry(registryPath(), false).gmIsHired()); refused {
		reply(status, text)
		return
	}
	if role == "" {
		reply(http.StatusBadRequest, "Send {role}, the role to hire.")
		return
	}
	// A role that isn't there is said first, before anything else is looked at, as the Mac's hireRequest.
	d.mu.Lock()
	known := slices.ContainsFunc(d.config.Roles, func(r Role) bool { return r.ID == role })
	d.mu.Unlock()
	if !known {
		reply(http.StatusNotFound, "That role isn't there any more. Pick another.")
		return
	}
	if _, isText := body["task"].(string); body["task"] != nil && !isText {
		reply(http.StatusBadRequest, "The task is text.")
		return
	}
	// Where it works, in the Mac's words: a terminal (left out or "terminal") or T3 Code ("t3"), nothing else.
	t3, known := checkedHireEngine(body["engine"])
	if !known {
		reply(http.StatusBadRequest, hireEngineRefusal)
		return
	}
	// What the Go path can't honour yet is refused, never dropped: what's left out, null or already so is fine.
	set := func(field string, already ...any) bool {
		v := body[field]
		return v != nil && !slices.Contains(already, v)
	}
	floor, isNumber := body["floor"].(float64)
	answers, isList := body["answers"].([]any)
	for _, f := range []struct {
		field string
		bad   bool
	}{{"model", set("model", "")}, {"nickname", set("nickname", "")}, {"task", set("task", "")},
		{"apiKey", set("apiKey", false)}, {"worktree", set("worktree", true)}, // a hire is always in a worktree of its own
		{"answers", set("answers") && (!isList || len(answers) > 0)}} {
		if f.bad {
			reply(http.StatusBadRequest, gmHireCantHonour(f.field, d.hiresOpenThroughMacApp()))
			return
		}
	}
	if set("floor") && (!isNumber || floor != float64(int(floor)) || floor < 0 || floor >= floorCount) {
		reply(http.StatusBadRequest, "The floor is 0 to 5.")
		return
	}
	given := body["folder"]
	if text, isText := given.(string); given == nil || isText && strings.TrimSpace(text) == "" {
		// Left out (or only spaces): the folder for new hires, checked like any other. With none set, hireWith says so (setupHireNeedsFolder).
		d.mu.Lock()
		given = d.config.HireFolder
		d.mu.Unlock()
		if _, ok := existingFolder(given.(string)); !ok {
			given = nil
		}
	}
	folder, why := gmHireFolder(given)
	if why != "" {
		reply(http.StatusBadRequest, why)
		return
	}
	if t3 {
		d.gmHireIntoT3(w, setupHire{Role: role, Folder: folder, Floor: int(floor), Answers: []string{}, Worktree: true, T3: true})
		return
	}
	// The Claude account, by name, as the Mac's route reads it (anything but text is none): with more than
	// one saved, hireWith asks which.
	account, _ := body["account"].(string)
	seat := -1
	if status, text := d.hireWith(hireSpec{roleID: role, floor: int(floor), fromSetup: true, folder: folder, fenced: true, seat: &seat,
		account: strings.Trim(account, " \t")}); status != http.StatusNoContent {
		reply(status, text)
		return
	}
	if floor > 0 { // upstairs the answer is the floor, as the Mac's
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "desk": nil, "floor": int(floor), "session": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "desk": seat, "session": nil})
}

// gmHireIntoT3 is the route's hire into T3 Code: Setup's own lane, which refuses as the Mac does (the role, hiring
// off, the hooks, the cooldown, the lane) before anything is made, with the answer of any setup hire. With no
// folder for new hires, the route's own refusal, as for a hire in a terminal.
func (d *Deck) gmHireIntoT3(w http.ResponseWriter, h setupHire) {
	if h.Folder == "" {
		writeStatus(w)(http.StatusConflict, setupHireNeedsFolder)
		return
	}
	d.mu.Lock()
	roles := slices.Clone(d.config.Roles)
	d.mu.Unlock()
	status, answer, why := d.hireFromSetupSheetIntoT3(h, roles)
	if answer == nil {
		writeStatus(w)(status, why)
		return
	}
	writeJSON(w, status, answer)
}
