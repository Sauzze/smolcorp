package desktop

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// A General Manager held to one project (FXX1c). The owner's hire of the GM may name a scope: the project
// it is hired into (`scope` in POST /setup/hire, for the gm role alone; the GM's token never hires the gm
// role). The app keeps it in the GM's registry row, so it outlasts a restart, and holds every request made
// with that GM's token to it, whichever way it came (`smolcorp gm`, `smolcorp mcp`, curl):
//   - a hire only into the project itself or the top of one of its own worktrees (the folder is the top of
//     its checkout, and the checkout's git folder is the project's), a folder that is there, in a terminal
//     of Smolcorp's, on the Claude account the GM was hired on (filled in when left out), never the API key;
//   - send, brief, fire, contacts, nickname, pin, unpin, watch and unwatch only for its own hires, folders
//     and watches;
//   - nothing else that changes the office (its switches, roles, layout, windows…); what it reads is as before.
// Each refusal is one sentence. A GM with no scope, every other, is as it always was. `smolcorp demo`
// hires its GM so. Go only: the Mac's Swift engine has no scope (M7).

const gmScopeNeedsEngine = "A General Manager held to one project is hired only through Smolcorp's hire engine in tmux, which keeps its scope."

// checkedGMScope is setup's `scope` for hire h: "" when none is asked, else the folder h is hired into, or
// a status and why not.
func checkedGMScope(given any, h setupHire, home string) (string, int, string) {
	if given == nil {
		return "", 0, ""
	}
	text, isText := given.(string)
	switch {
	case !isText || strings.TrimSpace(text) == "":
		return "", http.StatusBadRequest, "The scope is the project's folder, as text."
	case h.Role != gmRole:
		return "", http.StatusBadRequest, "Only the General Manager is held to a scope."
	case h.T3:
		return "", http.StatusConflict, "A General Manager held to one project works in a terminal of Smolcorp's, not in T3 Code."
	}
	folder, status, why := checkedSetupFolder(text, home)
	if status != 0 {
		return "", status, why
	}
	if folder != h.Folder {
		return "", http.StatusBadRequest, "A General Manager's scope is the folder it's hired into."
	}
	return folder, 0, ""
}

// gmScopeNow is the scope of the GM whose token works now, "" for none.
func gmScopeNow() string { return gmNow().Scope }

// gmNow is the GM whose token works now, a zero Hire for none: the open GM hired last, as each GM's hire
// makes a new token and takes the older one's place.
func gmNow() Hire {
	var newest *Hire
	hires := officeHires()
	for i := range hires {
		if gmHired(hires[i]) && (newest == nil || hires[i].Started >= newest.Started) {
			newest = &hires[i]
		}
	}
	if newest == nil {
		return Hire{}
	}
	return *newest
}

// gmAccountWords is the Claude account a GM was hired on, as its refusal says it.
func gmAccountWords(account string) string {
	if account == "" || account == claudeDefaultName {
		return "your default Claude account, the one it was hired on"
	}
	return "the Claude account it was hired on, " + account
}

// gmScope is a scope as a request is held to it: the project's full path, as kept at the GM's hire
// (links already followed), and how git is asked.
type gmScope struct {
	root string
	git  gitRunner
}

// holds is whether folder is the project itself or the top of one of its own worktrees, and is there.
func (s gmScope) holds(folder string) bool { _, ok := s.place(folder); return ok }

// place is folder, links followed, when it is held. Links are followed in folder, never in the project's
// path as kept: a project swapped for a link holds nothing.
func (s gmScope) place(folder string) (string, bool) {
	if folder == "" || !filepath.IsAbs(folder) {
		return "", false
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(folder))
	if err != nil {
		return "", false
	}
	if info, err := os.Stat(real); err != nil || !info.IsDir() {
		return "", false
	}
	if real == s.root {
		return real, true
	}
	if _, err := os.Lstat(filepath.Join(real, ".git")); err != nil {
		return "", false
	}
	status, out, ok := s.git(real, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	lines := strings.Split(out, "\n")
	held := ok && status == 0 && len(lines) == 2 && physical(strings.TrimSpace(lines[0])) == real &&
		physical(strings.TrimSpace(lines[1])) == filepath.Join(s.root, ".git")
	return real, held
}

// pinned is whether a desk's pin, as config.json keeps it, is held.
func (s gmScope) pinned(pin string) bool {
	folder, _ := existingFolder(pin)
	return s.holds(folder)
}

// covers is whether path is inside the project or one of its worktrees: the nearest folder holding it is held.
func (s gmScope) covers(path string) bool {
	if path == "" || !filepath.IsAbs(path) {
		return false
	}
	for dir := realPath(path); ; dir = filepath.Dir(dir) {
		if s.holds(dir) {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}

// hasHire is whether h is one of the GM's own: a hire, not the GM, working in the project or a worktree of it.
func (s gmScope) hasHire(h Hire) bool {
	return h.Role != gmRole && s.holds(h.Folder) && (h.Worktree == "" || s.holds(h.Worktree))
}

// named is the hire a name stands for, as the routes find it (by id, nickname or name), if it's one of the GM's.
func (s gmScope) named(name string) bool {
	h, refused := findHire(name, readRegistry(registryPath(), false).hires)
	return refused == nil && s.hasHire(h)
}

// gmScopeRefusal is why a request made with the GM's token goes beyond the GM's scope: a status and one
// sentence, or 0 to go on. A request it checks gets back, as its body, exactly what was checked.
func (d *Deck) gmScopeRefusal(r *request) (int, string) {
	gm := gmNow()
	root := gm.Scope
	if root == "" || r.Method == http.MethodGet {
		return 0, ""
	}
	s := gmScope{root: root, git: runGit}
	where := " in " + root
	refuse := func(text string) (int, string) { return http.StatusForbidden, "This General Manager " + text }
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	asked := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&asked)
	text := func(key string) string { v, _ := asked[key].(string); return v }
	resend := func(body any) {
		data, _ := json.Marshal(body)
		r.Body = io.NopCloser(bytes.NewReader(data))
	}
	resend(asked)
	path, query := r.URL.Path, r.URL.Query()
	post := r.Method == http.MethodPost
	switch {
	case post && path == "/setup/gm/hire":
		if v := asked["apiKey"]; v != nil && v != false {
			return refuse("never hires on the API key.")
		}
		// Its hires work on the account it was hired on (the one the demo's card showed): named or left out,
		// never another. Left out, the app fills it in, so a GM on another account than the default (a test
		// copy's, which never hires on the default one) hires on its own.
		own := gm.Account
		if own == claudeDefaultName {
			own = ""
		}
		named, isName := asked["account"].(string)
		named = strings.Trim(named, " \t")
		if v := asked["account"]; v != nil && !isName || named != "" && named != own && !(named == claudeDefaultName && own == "") {
			return refuse("hires only on " + gmAccountWords(gm.Account) + ".")
		}
		// The default one is named too, so the route never asks which (with more than one saved, it would).
		if own != "" {
			asked["account"] = own
		} else {
			asked["account"] = claudeDefaultName
		}
		if t3, known := checkedHireEngine(asked["engine"]); t3 || !known {
			return refuse("hires only in a terminal of Smolcorp's.")
		}
		folder, why := gmHireFolder(asked["folder"])
		real, held := s.place(folder)
		if why != "" || !held {
			return refuse("hires only into " + root + " or the top of one of its worktrees, a folder that is there.")
		}
		// The route hires into the folder as checked, links followed, never the words it was sent.
		asked["folder"] = real
		resend(asked)
	case post && path == "/setup/gm/send":
		if to := mailAsked(text("to")); to != "gm" && to != "the gm" && !s.named(text("to")) {
			return refuse("writes only to its own hires" + where + ".")
		}
	case post && (path == "/setup/gm/brief" || path == "/setup/gm/fire"):
		if h, found := hireByID(text("id")); !found || !s.hasHire(h) {
			return refuse("briefs and fires only its own hires" + where + ".")
		}
	case post && path == "/setup/gm/contacts":
		if !s.named(text("from")) || !s.named(text("to")) {
			return refuse("pairs only its own hires" + where + ".")
		}
	case post && path == "/setup/gm/nickname":
		var given struct {
			Folder *string `json:"folder"`
			Name   *string `json:"name"`
		}
		_ = json.Unmarshal(raw, &given)
		if given.Folder == nil || !s.holds(strings.TrimSpace(*given.Folder)) {
			return refuse("names only who works" + where + " or its worktrees.")
		}
		resend(given)
	case post && (path == "/setup/pin" || path == "/setup/unpin"):
		index, err := strconv.Atoi(query.Get("key"))
		d.mu.Lock()
		var pinned *string
		if err == nil && index >= 0 && index < len(d.config.Pins) {
			pinned = d.config.Pins[index]
		}
		d.mu.Unlock()
		folder, _ := existingFolder(query.Get("folder"))
		if pinned != nil && !s.pinned(*pinned) || path == "/setup/pin" && !s.holds(folder) {
			return refuse("pins only" + where + " and its worktrees, never over another desk's pin.")
		}
	case post && path == "/setup/gm/watch":
		// Read as the route reads it (watchers.add): the hire by id, nickname or name; a file by its full path,
		// or a path inside a hire's folder.
		line := func(key string) string { return oneLineLimit(text(key), watchTextMost) }
		hires := readRegistry(registryPath(), false).hires
		ours := func(name string) bool {
			h, refused := findWatchHire(name, hires)
			return refused == nil && s.hasHire(h)
		}
		ok := true
		switch strings.ToLower(line("kind")) {
		case "idle", "exit":
			ok = ours(line("target"))
		case "file":
			file := expandTilde(line("target"), homeDir)
			if !filepath.IsAbs(file) && line("hire") != "" {
				h, refused := findWatchHire(line("hire"), hires)
				ok = refused == nil && s.hasHire(h)
				file = filepath.Join(cmpOr(h.Worktree, h.Folder), file)
			}
			ok = ok && s.covers(file)
		}
		if !ok {
			return refuse("watches only its own hires and the files" + where + ".")
		}
	case (post || r.Method == http.MethodDelete) && path == "/setup/gm/unwatch":
		id := text("id")
		if query.Has("id") {
			id = query.Get("id")
		}
		if w, found := d.watchers.get(id); found && w.By != "gm" {
			return refuse("takes away only its own watches.")
		}
	case post && (path == "/setup/gm/tile" || path == "/setup/gm/screen"):
		return refuse("works only" + where + ": it doesn't move the office's windows.")
	default:
		return refuse("works only" + where + ": it can't change the office's settings.")
	}
	return 0, ""
}
