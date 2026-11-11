package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
)

// POST /setup/gm/hire, for the GM and the setup page (the route table gates the callers): the Mac's hire, through
// the same hire path as the office (hireWith), and only into a fenced way, never a plain terminal, which has no guard.
// The request is the fixture Tests/fixtures/frontdesk/hire.json.

// gmHireFolder checks the folder asked for: inside your home folder, never the home folder
// itself, and one a hire can work in. "" is none, which is the folder for new hires.
func gmHireFolder(given any) (folder string, why string) {
	text, ok := given.(string)
	if given != nil && !ok {
		return "", "The folder is text."
	}
	if strings.TrimSpace(text) == "" {
		return "", ""
	}
	check := checkHireFolder(text, homeDir, choosingHireFolder)
	rel, err := filepath.Rel(realPath(homeDir), check.folder)
	if check.folder != "" && (err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel)) {
		return "", "That folder isn't inside your home folder: the General Manager hires only in a folder of yours, not the home folder itself."
	}
	return check.folder, check.problem
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
	if _, isText := body["task"].(string); body["task"] != nil && !isText {
		reply(http.StatusBadRequest, "The task is text.")
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
	}{{"account", set("account", "")}, {"model", set("model", "")}, {"nickname", set("nickname", "")}, {"task", set("task", "")},
		{"apiKey", set("apiKey", false)}, {"worktree", set("worktree", true)}, // a hire is always in a worktree of its own
		{"answers", set("answers") && (!isList || len(answers) > 0)}} {
		if f.bad {
			reply(http.StatusBadRequest, "Smolcorp on Linux and Windows can't honour \""+f.field+"\" in a hire yet: leave it out.")
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
	seat := -1
	if status, text := d.hireWith(hireSpec{roleID: role, floor: int(floor), fromSetup: true, folder: folder, fenced: true, seat: &seat}); status != http.StatusNoContent {
		reply(status, text)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "desk": seat, "session": nil})
}
