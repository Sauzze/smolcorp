package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// POST /setup/gm/hire, for the General Manager and the setup page (the route
// table gates the callers): a hire as the Mac's BriefRoutes makes it, through
// the same hire path as a hire from the office (hireWith), and only into a
// fenced way: the tmux engine, or the launcher's behind its switch. Never a
// plain terminal, which has no guard. The request is the shared fixture
// Tests/fixtures/frontdesk/hire.json.

// gmHireCant is what a field the Go app can't honour yet is refused with, with its name.
func gmHireCant(field string) string {
	return "Smolcorp on Linux and Windows can't honour \"" + field + "\" in a hire yet: leave it out."
}

// gmHireFolder is the folder a request names, checked: inside your home
// folder, never the home folder itself, and one a hire can work in. "" is
// none named, which is the folder for new hires.
func gmHireFolder(given any) (folder string, status int, why string) {
	if given == nil {
		return "", 0, ""
	}
	text, ok := given.(string)
	if !ok {
		return "", http.StatusBadRequest, "The folder is text."
	}
	if strings.TrimSpace(text) == "" {
		return "", 0, ""
	}
	check := checkHireFolder(text, homeDir, choosingHireFolder)
	if check.folder != "" {
		rel, err := filepath.Rel(realPath(homeDir), check.folder)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return "", http.StatusBadRequest, "That folder isn't inside your home folder: the General Manager hires only in a folder of yours, not the home folder itself."
		}
	}
	if check.problem != "" {
		return "", http.StatusBadRequest, check.problem
	}
	return check.folder, 0, ""
}

// gmHire answers POST /setup/gm/hire.
func (d *Deck) gmHire(w http.ResponseWriter, r *request) {
	reply := writeStatus(w)
	var body map[string]any
	data, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err := json.Unmarshal(data, &body); err != nil || body == nil {
		reply(http.StatusBadRequest, "Send {role}, the role to hire.")
		return
	}
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
	if task, ok := body["task"]; ok && task != nil {
		if _, isText := task.(string); !isText {
			reply(http.StatusBadRequest, "The task is text.")
			return
		}
	}
	// What the Go path can't honour yet is refused, never dropped.
	for _, field := range []string{"account", "model", "nickname", "task"} {
		if v, ok := body[field]; ok && v != nil && v != "" {
			reply(http.StatusBadRequest, gmHireCant(field))
			return
		}
	}
	if v, ok := body["answers"]; ok && v != nil {
		if list, isList := v.([]any); !isList || len(list) > 0 {
			reply(http.StatusBadRequest, gmHireCant("answers"))
			return
		}
	}
	if v, ok := body["floor"]; ok && v != nil && v != float64(0) {
		reply(http.StatusBadRequest, gmHireCant("floor"))
		return
	}
	if v, ok := body["apiKey"]; ok && v != nil && v != false {
		reply(http.StatusBadRequest, gmHireCant("apiKey"))
		return
	}
	// A hire is always in a worktree of its own: asking for none can't be honoured.
	if v, ok := body["worktree"]; ok && v != nil && v != true {
		reply(http.StatusBadRequest, gmHireCant("worktree"))
		return
	}
	given := body["folder"]
	if given == nil || given == "" {
		// Left out: the folder for new hires. With none set, hireWith says so (setupHireNeedsFolder).
		d.mu.Lock()
		given = d.config.HireFolder
		d.mu.Unlock()
		if _, ok := existingFolder(given.(string)); !ok {
			given = nil
		}
	}
	folder, status, why := gmHireFolder(given)
	if status != 0 {
		reply(status, why)
		return
	}
	seat := -1
	status, text := d.hireWith(hireSpec{roleID: role, fromSetup: true, folder: folder, fenced: true, seat: &seat})
	if status != http.StatusNoContent {
		reply(status, text)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "desk": seat, "session": nil})
}
