package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The demo's page (FXX1d). `smolcorp demo` holds only the setup key, which never hires and never opens a
// session of setup (the route table; server.go). So the demo's General Manager is hired by you, from Setup's
// Start here: the demo makes its folder, then asks the app to open Setup at the demo. The app makes the
// one-time link and opens it in the browser itself, as the tray's Open Setup… does, so the link's token never
// reaches the command. The page shows the demo's hire, which the app builds from its own template (the project,
// its scope, the task, the roles), what it spends and the switches its Hire turns on; you click Hire, and the
// page's own POST /setup/hire carries the scope. The command waits for that GM, then follows the team.
//
// The app answers only for a demo folder the command registered with it in this run of the app (FXX1e, RX1d S1),
// never one it knows by its shape alone: a cloned repo or an unpacked download can have that shape. The roles are
// the ones registered, each a Claude role the office has; nothing the task or answers say is read from the folder.
//
//	GET  /setup/demo[?root=R]               page, tool: the office as the demo needs it (a GM hired, the hires
//	                                        open); with root, that demo's hire, its switches, its page and its GM
//	POST /setup/demo/register?root=R&roles=L the command alone (the setup key): records the folder it made, with
//	                                        the roles its fence lets the GM hire
//	POST /setup/demo/open?root=R            page, tool: opens Setup at the demo in the browser, once per demo folder
//	POST /setup/demo/page?root=R&state=S    the page alone: it is open (S=open, every 15 s) or was closed (S=closed)
//
// None is in the route table (routes.go), which keeps 1.23's rule for a route it doesn't name (the page, or a tool
// with the setup key), and a GM's token gets none. Go only: the Mac's Swift engine doesn't serve them (404), and
// the demo says so.

// demoPage is what the app knows of one demo: the roles it was registered with, and its page.
type demoPage struct {
	roles          []string
	registered     time.Time
	opened         bool
	state          string // "", "open", "closed": as the page last said
	seen, closedAt time.Time
}

var demoPages = struct {
	sync.Mutex
	at map[string]*demoPage // by the demo folder's full path; only registered ones
}{at: map[string]*demoPage{}}

// demoPagesMost is how many demos the app keeps: registering one more forgets the oldest.
const demoPagesMost = 8

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

// demoAt is the demo folder at root as `smolcorp demo` made it for roles, or why it isn't one: inside your home
// folder, named as the demo names its folders, its project a repository whose committed settings are the demo's
// fence for that folder and those roles, exactly as demoSettings writes it, and nothing else before every tool.
func demoAt(root string, roles []string) (demoFolder, string) {
	folder, why := demoRootOf(root)
	if why != "" {
		return demoFolder{}, why
	}
	notOne := root + " isn't a folder smolcorp demo made."
	f := demoFolder{root: folder, repo: filepath.Join(folder, demoRepoName)}
	if info, err := os.Stat(filepath.Join(f.repo, ".git")); err != nil || !info.IsDir() {
		return demoFolder{}, notOne
	}
	data, err := os.ReadFile(filepath.Join(f.repo, ".claude", "settings.json"))
	if err != nil {
		return demoFolder{}, notOne
	}
	var settings struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &settings) != nil || len(settings.Hooks.PreToolUse) != 1 || len(roles) == 0 {
		return demoFolder{}, notOne
	}
	entry := settings.Hooks.PreToolUse[0]
	if entry.Matcher != "*" || len(entry.Hooks) != 1 || entry.Hooks[0].Type != "command" {
		return demoFolder{}, notOne
	}
	// As demoSettings writes it: `<smolcorp> demo fence --root <root> --roles <roles>`, the program one word.
	program, found := strings.CutSuffix(entry.Hooks[0].Command,
		" demo fence --root "+shellPath(folder)+" --roles "+shellPath(strings.Join(roles, ",")))
	if !found || !demoProgramWord(program) {
		return demoFolder{}, notOne
	}
	return f, ""
}

// demoRootOf is root as a demo folder's full path, links followed, or why it can't be one: inside your home
// folder and named as makeDemoFolder names its folders.
func demoRootOf(root string) (string, string) {
	if strings.TrimSpace(root) == "" {
		return "", "Say which demo folder (root)."
	}
	folder, status, why := checkedSetupFolder(root, homeDir)
	if status != 0 {
		return "", why
	}
	if !demoFolderNamed(filepath.Base(folder)) {
		return "", root + " isn't a folder smolcorp demo made."
	}
	return folder, ""
}

// demoProgramWord is whether word is smolcorp's program as demoSettings puts it in the fence's line: one full
// path, quoted as shellPath quotes it, with nothing a shell would run besides.
func demoProgramWord(word string) bool {
	inner := word
	if runtime.GOOS == "windows" {
		inner = strings.TrimSuffix(strings.TrimPrefix(word, `"`), `"`)
		quoted, special := inner != word, "\"%!$`\r\n" // what a shell reads even inside double quotes
		if !quoted {
			special += " \t&|<>^;()"
		}
		return inner != "" && (!quoted || word == `"`+inner+`"`) && !strings.ContainsAny(inner, special) && filepath.IsAbs(filepath.FromSlash(inner))
	}
	if len(word) < 2 || word[0] != '\'' || word[len(word)-1] != '\'' {
		return false
	}
	inner = strings.ReplaceAll(word[1:len(word)-1], `'\''`, `'`)
	return shellPath(inner) == word && filepath.IsAbs(inner) && !strings.ContainsAny(inner, "\r\n")
}

// registeredDemo is the demo the command registered at root: its folder, still as it was made, and its roles.
func registeredDemo(root string) (demoFolder, []string, string) {
	folder, why := demoRootOf(root)
	if why != "" {
		return demoFolder{}, nil, why
	}
	demoPages.Lock()
	var roles []string
	if page := demoPages.at[folder]; page != nil {
		roles = page.roles
	}
	demoPages.Unlock()
	if roles == nil {
		return demoFolder{}, nil, root + " isn't a demo folder this Smolcorp was told about: run smolcorp demo for one."
	}
	f, why := demoAt(folder, roles)
	if why != "" {
		return demoFolder{}, nil, why
	}
	return f, roles, ""
}

// demoRolesRefusal is why roles can't be the demo's, or "": each a role the office has, on Claude Code, not the
// General Manager, named once.
func (d *Deck) demoRolesRefusal(roles []string) string {
	d.mu.Lock()
	office := slices.Clone(d.config.Roles)
	d.mu.Unlock()
	if len(roles) == 0 || len(roles) > len(office) {
		return "Say which roles the demo's General Manager may hire (roles), each a role of the office's."
	}
	for i, id := range roles {
		at := slices.IndexFunc(office, func(r Role) bool { return r.ID == id })
		if at < 0 || id == gmRole || agentOrClaude(office[at].Agent) != Claude || slices.Contains(roles[:i], id) {
			return "The demo's roles are roles the office has, on Claude Code, each once: " + id + " isn't one."
		}
	}
	return ""
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
	f, roles, why := registeredDemo(root)
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

	case r.Method == http.MethodPost && path == "/setup/demo/register":
		// The command alone: it made the folder a moment ago. The page and a link never register one.
		if r.caller != callerTool {
			writeStatus(w)(http.StatusForbidden, "Only smolcorp demo registers the folder it made.")
			return true
		}
		roles := strings.Split(query.Get("roles"), ",")
		if query.Get("roles") == "" {
			roles = nil
		}
		if why := d.demoRolesRefusal(roles); why != "" {
			writeStatus(w)(http.StatusBadRequest, why)
			return true
		}
		f, why := demoAt(query.Get("root"), roles)
		if why != "" {
			writeStatus(w)(http.StatusBadRequest, why)
			return true
		}
		demoPages.Lock()
		defer demoPages.Unlock()
		if page := demoPages.at[f.root]; page != nil {
			if !slices.Equal(page.roles, roles) {
				writeStatus(w)(http.StatusConflict, f.root+" is registered already, with other roles: run smolcorp demo again for a new folder.")
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		}
		for len(demoPages.at) >= demoPagesMost {
			oldest := ""
			for root, page := range demoPages.at {
				if oldest == "" || page.registered.Before(demoPages.at[oldest].registered) {
					oldest = root
				}
			}
			delete(demoPages.at, oldest)
		}
		demoPages.at[f.root] = &demoPage{roles: slices.Clone(roles), registered: time.Now()}
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodPost && path == "/setup/demo/open":
		f, _, why := registeredDemo(query.Get("root"))
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
		if page = demoPages.at[f.root]; page != nil {
			page.opened = true
		}
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
		f, _, why := registeredDemo(query.Get("root"))
		if why != "" {
			writeStatus(w)(http.StatusBadRequest, why)
			return true
		}
		demoPages.Lock()
		page := demoPages.at[f.root]
		if page == nil { // forgotten a moment ago (demoPagesMost)
			demoPages.Unlock()
			writeStatus(w)(http.StatusBadRequest, f.root+" isn't a demo folder this Smolcorp was told about: run smolcorp demo for one.")
			return true
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
