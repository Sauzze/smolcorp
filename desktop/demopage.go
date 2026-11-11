package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The demo's page (FXX1d). `smolcorp demo` holds only the setup key, which never hires and never opens a
// session of setup (the route table; server.go). So the demo's General Manager is hired by you, from Setup's
// Start here: the demo makes its folder, then asks the app to open Setup at the demo. The app makes the
// one-time link and opens it in the browser itself, as the tray's Open Setup… does, so the link's token never
// reaches the command. The page shows the demo's hire, which the app reads from the demo folder (the project, its
// scope, the task, the roles its fence allows), what it spends and the switches its Hire turns on; you click
// Hire, and the page's own POST /setup/hire carries the scope. The command waits for that GM, then
// follows the team.
//
//	GET  /setup/demo[?root=R]               page, tool: the office as the demo needs it (a GM hired, the hires
//	                                        open); with root, that demo's hire, its switches, its page and its GM
//	POST /setup/demo/open?root=R            page, tool: opens Setup at the demo in the browser, once per demo folder
//	POST /setup/demo/page?root=R&state=S    the page alone: it is open (S=open, every 15 s) or was closed (S=closed)
//
// None is in the route table (routes.go), which keeps 1.23's rule for a route it doesn't name (the page, or a tool
// with the setup key), and a GM's token gets none. Go only: the Mac's Swift engine doesn't serve them (404), and
// the demo says so.

// demoPage is what the app knows of one demo's page.
type demoPage struct {
	opened         bool
	state          string // "", "open", "closed": as the page last said
	seen, closedAt time.Time
}

var demoPages = struct {
	sync.Mutex
	at map[string]*demoPage // by the demo folder's full path
}{at: map[string]*demoPage{}}

// demoPageQuiet is how long the page may go unheard before it counts as closed: it says it's open every 15 s,
// which a browser may slow to once a minute in a tab out of sight.
var demoPageQuiet = 3 * time.Minute

// demoPageGrace: a page closed and open again within it (reloaded) is still open.
var demoPageGrace = 5 * time.Second

// demoPageNow is the page as the command reads it: "waiting" (not heard from yet), "open" or "closed".
func demoPageNow(root string) string {
	demoPages.Lock()
	defer demoPages.Unlock()
	page := demoPages.at[root]
	switch {
	case page == nil || page.state == "":
		return "waiting"
	case page.state == "closed" && time.Since(page.closedAt) >= demoPageGrace, time.Since(page.seen) >= demoPageQuiet:
		return "closed"
	}
	return "open"
}

// demoFolderNamed is whether name is one makeDemoFolder gives: smolcorp-demo, smolcorp-demo-2 and on.
func demoFolderNamed(name string) bool {
	if name == demoFolderName {
		return true
	}
	n, err := strconv.Atoi(strings.TrimPrefix(name, demoFolderName+"-"))
	return strings.HasPrefix(name, demoFolderName+"-") && err == nil && n > 1 && strconv.Itoa(n) == strings.TrimPrefix(name, demoFolderName+"-")
}

// demoAt is the demo folder at root as `smolcorp demo` made it, with the roles its fence lets the GM hire, or why
// it isn't one: inside your home folder, named as the demo names its folders, its project a repository whose
// committed settings run the demo's fence for that folder.
func demoAt(root string) (demoFolder, []string, string) {
	if strings.TrimSpace(root) == "" {
		return demoFolder{}, nil, "Say which demo folder (root)."
	}
	folder, status, why := checkedSetupFolder(root, homeDir)
	if status != 0 {
		return demoFolder{}, nil, why
	}
	notOne := root + " isn't a folder smolcorp demo made."
	if !demoFolderNamed(filepath.Base(folder)) {
		return demoFolder{}, nil, notOne
	}
	f := demoFolder{root: folder, repo: filepath.Join(folder, demoRepoName)}
	if info, err := os.Stat(filepath.Join(f.repo, ".git")); err != nil || !info.IsDir() {
		return demoFolder{}, nil, notOne
	}
	data, err := os.ReadFile(filepath.Join(f.repo, ".claude", "settings.json"))
	if err != nil {
		return demoFolder{}, nil, notOne
	}
	var settings struct {
		Hooks struct {
			PreToolUse []struct {
				Hooks []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return demoFolder{}, nil, notOne
	}
	// As demoSettings writes it: `<smolcorp> demo fence --root <root> --roles <roles>`.
	marker := " demo fence --root " + shellPath(folder) + " --roles "
	for _, entry := range settings.Hooks.PreToolUse {
		for _, hook := range entry.Hooks {
			if _, list, found := strings.Cut(hook.Command, marker); found {
				var roles []string
				for _, role := range strings.Split(strings.Trim(list, `'" `), ",") {
					if role != "" {
						roles = append(roles, role)
					}
				}
				if len(roles) > 0 {
					return f, roles, ""
				}
			}
		}
	}
	return demoFolder{}, nil, notOne
}

// demoGMOf is the open General Manager hired into demo f's project, or nil.
func demoGMOf(f demoFolder, hires []Hire) *Hire {
	for i := range hires {
		h := hires[i]
		if gmHired(h) && (h.Scope == f.repo || resolveExisting(h.Folder) == f.repo) {
			return &hires[i]
		}
	}
	return nil
}

// demoPageLink is Setup at demo root, with the one-time token after #o= when given (the app opens that one).
func demoPageLink(root, token string) string {
	link := fmt.Sprintf("http://localhost:%d/setup?demo=%s", Port, url.QueryEscape(root))
	if token != "" {
		link += "#o=" + token
	}
	return link
}

// demoAnswer is GET /setup/demo.
func (d *Deck) demoAnswer(root string) (map[string]any, int, string) {
	hires := officeHires()
	open, gmIn := []string{}, false
	for _, h := range hires {
		if !isOpen(h) {
			continue
		}
		name := h.Nickname
		if name == "" {
			name = h.Name
		}
		open = append(open, name)
		gmIn = gmIn || h.Role == gmRole
	}
	answer := map[string]any{"gmHired": gmIn, "open": open}
	if root == "" {
		return answer, http.StatusOK, ""
	}
	f, roles, why := demoAt(root)
	if why != "" {
		return nil, http.StatusBadRequest, why
	}
	d.mu.Lock()
	on := map[string]bool{"/setup/gm/manager": d.config.ManagerEnabled, "/setup/hiring": d.config.HiringEnabled}
	d.mu.Unlock()
	switches := []map[string]any{}
	for _, s := range demoSwitches {
		switches = append(switches, map[string]any{"name": s.name, "route": s.route, "setting": s.setting, "on": on[s.route]})
	}
	var gm map[string]any
	if h := demoGMOf(f, hires); h != nil {
		gm = map[string]any{"id": h.ID, "state": h.State}
	}
	answer["demo"] = map[string]any{"root": f.root, "folder": f.repo, "scope": f.repo, "roles": roles, "hire": demoGMHire(f, roles),
		"switches": switches, "page": demoPageNow(f.root), "gm": gm}
	return answer, http.StatusOK, ""
}

// demoRoute serves the demo's routes, and says whether path was one.
func (d *Deck) demoRoute(w http.ResponseWriter, r *request) bool {
	path, query := r.URL.Path, r.URL.Query()
	switch {
	case r.Method == http.MethodGet && path == "/setup/demo":
		answer, status, why := d.demoAnswer(query.Get("root"))
		if status != http.StatusOK {
			writeStatus(w)(status, why)
			return true
		}
		writeJSON(w, http.StatusOK, answer)

	case r.Method == http.MethodPost && path == "/setup/demo/open":
		f, _, why := demoAt(query.Get("root"))
		if why != "" {
			writeStatus(w)(http.StatusBadRequest, why)
			return true
		}
		if demoGMOf(f, officeHires()) != nil || readRegistry(registryPath(), false).gmIsHired() {
			writeStatus(w)(http.StatusConflict, "A General Manager is already hired: the demo's page has nothing to hire.")
			return true
		}
		demoPages.Lock()
		page := demoPages.at[f.root]
		if page != nil && page.opened {
			demoPages.Unlock()
			writeStatus(w)(http.StatusConflict, "The demo's page was already opened for "+f.root+": run smolcorp demo again for a new one.")
			return true
		}
		demoPages.Unlock()
		d.mu.Lock()
		token := d.access.NewSetupToken()
		d.mu.Unlock()
		if err := openURL(demoPageLink(f.root, token)); err != nil {
			writeStatus(w)(http.StatusBadGateway, fmt.Sprintf("The browser didn't open (%v): run smolcorp demo --no-open for the demo's link.", err))
			return true
		}
		demoPages.Lock()
		if page = demoPages.at[f.root]; page == nil {
			page = &demoPage{}
			demoPages.at[f.root] = page
		}
		page.opened = true
		demoPages.Unlock()
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodPost && path == "/setup/demo/page":
		if r.caller != callerPage {
			writeStatus(w)(http.StatusForbidden, "Only the setup page says whether it's open.")
			return true
		}
		state := query.Get("state")
		if state != "open" && state != "closed" {
			writeStatus(w)(http.StatusBadRequest, "The state is open or closed.")
			return true
		}
		f, _, why := demoAt(query.Get("root"))
		if why != "" {
			writeStatus(w)(http.StatusBadRequest, why)
			return true
		}
		demoPages.Lock()
		page := demoPages.at[f.root]
		if page == nil {
			page = &demoPage{}
			demoPages.at[f.root] = page
		}
		page.state = state
		if state == "open" {
			page.seen = time.Now()
		} else {
			page.closedAt = time.Now()
		}
		demoPages.Unlock()
		w.WriteHeader(http.StatusNoContent)

	default:
		return false
	}
	return true
}
