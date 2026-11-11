package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// The front desk: the registry of hires, kept in registry.json in the support
// folder, joined with the sessions the deck follows into the team that
// GET /setup/gm/team answers. Mirrors Sources/AgentDeck/FrontDesk.swift; the
// wire formats are Tests/fixtures/frontdesk/team.json, registry.json and
// registry-edit.json. Nothing here writes a new hire: that is the hire engine's.

const (
	registryVersion = 1
	longestNickname = 40
	longestNotes    = 2000
)

var hireStates = []string{"starting", "working", "idle", "fired", "failed"}

// frontDeskMu serializes edits of the registry file.
var frontDeskMu sync.Mutex

// Hire is what the app knows of one hire. Only nickname, state and notes are
// ever edited from outside.
type Hire struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Nickname   string `json:"nickname"`
	Role       string `json:"role"`
	Folder     string `json:"folder"`
	Branch     string `json:"branch"`
	Worktree   string `json:"worktree"`
	Repo       string `json:"repo"`
	TmuxSocket string `json:"tmuxSocket"`
	// ViaLauncher: the hire engine made it in a window run by the launcher, which is known by this hire's id.
	ViaLauncher bool    `json:"viaLauncher,omitempty"`
	Account     string  `json:"account"`
	Model       string  `json:"model"`
	Ports       string  `json:"ports"`
	Brief       string  `json:"brief"`
	Started     float64 `json:"started"`
	State       string  `json:"state"`
	Notes       string  `json:"notes"`
	// Where its agent runs: "" in a terminal of Smolcorp's, t3Engine ("t3") in T3 Code (t3desk.go), with its
	// thread's id and link there. Written only when set, so every other hire's row is as it always was.
	Engine   string `json:"engine,omitempty"`
	T3Thread string `json:"t3Thread,omitempty"`
	T3Link   string `json:"t3Link,omitempty"`
	// Where a hire in T3 Code's desk is in the office (t3desks.go): its floor (0 for the ground floor, 1 for 2F up
	// to 5 for 6F) and its desk there, so it sits there again after a restart; nil for where there's room. Written
	// only for a hire in T3 Code, as the Mac writes them.
	T3Floor *int `json:"t3Floor,omitempty"`
	T3Desk  *int `json:"t3Desk,omitempty"`
}

// liveSession is a session as the deck follows it, with where it sits.
type liveSession struct {
	ID, Agent, Folder, Project, State, Model string
	Floor, Desk                              *int
	LastEventAt                              float64
}

// Contact is one pair who may write to each other: from may write to to
// (FrontDesk.swift Contact). A two-way pair is two of these.
type Contact struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// registryFile holds the hires, the pairs who may write to each other, the
// hash of each hire's send key (hire id to SHA-256 hex, never the key), and,
// kept untouched, whatever else the file has.
type registryFile struct {
	hires    []Hire
	contacts []Contact
	keys     map[string]string
	rest     map[string]json.RawMessage
}

// readRegistry loads the file. One that is there but can't be read gives an
// empty registry; with aside set (the edit path only) it is also moved to
// registry.broken.json, so a later save can't wipe what might be mended. A read
// route passes false and leaves the file where it is.
func readRegistry(path string, aside bool) registryFile {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return registryFile{}
		}
		return unreadable(path, aside)
	}
	var raw map[string]json.RawMessage
	var version int
	var hires []Hire
	var contacts []Contact
	var keys map[string]string
	// Contacts and keys that can't be read make the file unreadable, as on the Mac (FrontDesk.swift load).
	if json.Unmarshal(data, &raw) != nil || json.Unmarshal(raw["version"], &version) != nil || version > registryVersion ||
		(raw["hires"] != nil && json.Unmarshal(raw["hires"], &hires) != nil) ||
		(raw["contacts"] != nil && json.Unmarshal(raw["contacts"], &contacts) != nil) ||
		(raw["keys"] != nil && json.Unmarshal(raw["keys"], &keys) != nil) {
		return unreadable(path, aside)
	}
	for _, h := range hires {
		if h.ID == "" || h.Name == "" || h.Folder == "" {
			return unreadable(path, aside)
		}
	}
	delete(raw, "version")
	delete(raw, "hires")
	delete(raw, "contacts")
	delete(raw, "keys")
	return registryFile{hires: hires, contacts: contacts, keys: keys, rest: raw}
}

func unreadable(path string, aside bool) registryFile {
	if !aside {
		return registryFile{}
	}
	return setAside(path)
}

func setAside(path string) registryFile {
	aside := filepath.Join(filepath.Dir(path), "registry.broken.json")
	_ = os.Remove(aside)
	_ = os.Rename(path, aside)
	return registryFile{}
}

func (f registryFile) save(path string) error {
	hires := []Hire{}
	for _, h := range f.hires {
		// Where a desk sits is a hire in T3 Code's alone, as the Mac writes it.
		if h.Engine == "" {
			h.T3Floor, h.T3Desk = nil, nil
		}
		hires = append(hires, h)
	}
	out := map[string]any{"version": registryVersion, "hires": hires}
	for k, v := range f.rest {
		out[k] = v
	}
	// Left out while empty, so a registry without them is written as it was.
	if len(f.contacts) > 0 {
		out["contacts"] = f.contacts
	}
	if len(f.keys) > 0 {
		out["keys"] = f.keys
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(path, data)
}

// contactsJSON is the pairs as gm team and POST /setup/gm/contacts answer them:
// a list, never null.
func (f registryFile) contactsJSON() []Contact {
	if f.contacts == nil {
		return []Contact{}
	}
	return f.contacts
}

// isNull says whether a raw JSON value is null, which would otherwise decode
// into a string as "" and wipe the field.
func isNull(v json.RawMessage) bool { return string(bytes.TrimSpace(v)) == "null" }

type refusal struct {
	status  int
	message string
}

// editHire changes the nickname, state or notes of the hire `id`, and nothing
// else; anything more is refused whole, with nothing changed.
func editHire(path string, body []byte) (Hire, *refusal) {
	var given map[string]json.RawMessage
	if json.Unmarshal(body, &given) != nil {
		return Hire{}, &refusal{http.StatusBadRequest, "Send {id, nickname, state, notes}."}
	}
	var others []string
	for k := range given {
		if k != "id" && k != "nickname" && k != "state" && k != "notes" {
			others = append(others, k)
		}
	}
	slices.Sort(others)
	if len(others) > 0 {
		return Hire{}, &refusal{http.StatusBadRequest, "Only a hire's nickname, state and notes can be changed here, not " + strings.Join(others, ", ") + "."}
	}
	var id string
	if json.Unmarshal(given["id"], &id) != nil || id == "" {
		return Hire{}, &refusal{http.StatusBadRequest, "Say which hire, by its id."}
	}
	if len(given) < 2 {
		return Hire{}, &refusal{http.StatusBadRequest, "Give a nickname, a state or notes to change."}
	}
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(path, true)
	i := slices.IndexFunc(file.hires, func(h Hire) bool { return h.ID == id })
	if i < 0 {
		return Hire{}, &refusal{http.StatusNotFound, "There's no hire with that id."}
	}
	hire := file.hires[i]
	if v, ok := given["nickname"]; ok {
		var text string
		if isNull(v) || json.Unmarshal(v, &text) != nil {
			return Hire{}, &refusal{http.StatusBadRequest, "A nickname is text."}
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > longestNickname || strings.ContainsAny(text, "\r\n") {
			return Hire{}, &refusal{http.StatusBadRequest, "A nickname is one line of at most 40 characters."}
		}
		hire.Nickname = text
	}
	if v, ok := given["state"]; ok {
		var text string
		if json.Unmarshal(v, &text) != nil || !slices.Contains(hireStates, text) {
			return Hire{}, &refusal{http.StatusBadRequest, "State is one of " + strings.Join(hireStates, ", ") + "."}
		}
		hire.State = text
	}
	if v, ok := given["notes"]; ok {
		var text string
		if isNull(v) || json.Unmarshal(v, &text) != nil || utf8.RuneCountInString(text) > longestNotes {
			return Hire{}, &refusal{http.StatusBadRequest, "Notes are text of at most 2000 characters."}
		}
		hire.Notes = text
	}
	file.hires[i] = hire
	if file.save(path) != nil {
		return Hire{}, &refusal{http.StatusInternalServerError, "Couldn't save the change."}
	}
	return hire, nil
}

// physical is a path, absolute and with its links resolved, as a session's
// folder may be written one way and a worktree kept the other. "" stays "", and
// a path that isn't there yet comes back absolute and cleaned, unresolved.
func physical(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// works says whether a live session works where hire does: in its worktree if
// it has one, else in its folder, or the project it was started in.
func works(s liveSession, h Hire) bool {
	places := map[string]bool{}
	for _, p := range []string{h.Worktree, h.Folder} {
		if p != "" {
			places[p], places[physical(p)] = true, true
		}
	}
	for _, p := range []string{s.Folder, s.Project} {
		if p != "" && (places[p] || places[physical(p)]) {
			return true
		}
	}
	return false
}

func seatOrNull(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}

// buildTeam is one row for every hire in the registry, then one for every live
// session no hire matches. A hire that is fired or failed claims no session;
// each session is claimed by at most one hire and a hire by at most one session,
// the first listed. now is in milliseconds; status gives a folder's last STATUS line.
func buildTeam(hires []Hire, live []liveSession, now float64, status func(string) string) []map[string]any {
	claimed := map[string]bool{}
	rows := []map[string]any{}
	for _, h := range hires {
		var session *liveSession
		if h.State != "fired" && h.State != "failed" {
			for i := range live {
				// A hire in T3 Code claims only its own desk, never a session in the folder it was hired from; and
				// no other hire claims a T3 desk (FXT3Da B1, RT3Da2 n1).
				mine := works(live[i], h) && !strings.HasPrefix(live[i].ID, t3DeskPrefix)
				if h.Engine == t3Engine {
					mine = live[i].ID == t3DeskID(h.ID)
				}
				if !claimed[live[i].ID] && mine {
					session = &live[i]
					claimed[session.ID] = true
					break
				}
			}
		}
		rows = append(rows, teamRow(&h, session, now, status))
	}
	for i := range live {
		if !claimed[live[i].ID] {
			rows = append(rows, teamRow(nil, &live[i], now, status))
		}
	}
	return rows
}

func teamRow(h *Hire, s *liveSession, now float64, status func(string) string) map[string]any {
	var folder, place string
	row := map[string]any{"registered": h != nil, "live": s != nil}
	if h != nil {
		folder, place = h.Folder, cmpOr(h.Worktree, h.Folder)
		row["id"], row["name"], row["nickname"], row["role"] = h.ID, h.Name, h.Nickname, h.Role
		row["branch"], row["worktree"], row["tmuxSocket"], row["account"] = h.Branch, h.Worktree, h.TmuxSocket, h.Account
		row["started"], row["state"], row["notes"] = h.Started, h.State, h.Notes
		row["model"] = h.Model
		// A hire in T3 Code says so, with its thread; nobody else's row changes.
		if h.Engine != "" {
			row["engine"], row["t3Thread"] = h.Engine, h.T3Thread
		}
	} else {
		folder, place = s.Project, s.Folder
		row["id"], row["name"], row["nickname"], row["role"] = "live:"+s.ID, lastPart(folder), "", ""
		row["branch"], row["worktree"], row["tmuxSocket"], row["account"] = "", "", "", ""
		row["started"], row["state"], row["notes"] = nil, "", ""
		row["model"] = ""
	}
	if s != nil && row["model"] == "" {
		row["model"] = s.Model
	}
	row["folder"] = folder
	row["status"] = status(place)
	row["sessionId"], row["agent"], row["liveState"] = "", "", "gone"
	row["floor"], row["desk"], row["lastEventAt"], row["idleFor"] = nil, nil, nil, nil
	if s != nil {
		row["sessionId"], row["agent"], row["liveState"] = s.ID, s.Agent, s.State
		row["floor"], row["desk"] = seatOrNull(s.Floor), seatOrNull(s.Desk)
		if s.LastEventAt > 0 {
			row["lastEventAt"] = int64(s.LastEventAt)
			row["idleFor"] = int64(max(0, now-s.LastEventAt) / 1000)
		}
	}
	return row
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// statusLine is the last line with words in it of the STATUS.md a hire keeps,
// in its folder or its .team folder, cut to 200 characters; "" without one.
// Only a plain file's last few kilobytes are read; a pipe or any other kind
// of file is passed over.
func statusLine(folder string) string {
	if !filepath.IsAbs(folder) && !strings.HasPrefix(folder, "/") {
		return ""
	}
	for _, name := range []string{"STATUS.md", filepath.Join(".team", "STATUS.md")} {
		path := filepath.Join(folder, name)
		// Look before opening, as the Mac does (links followed): opening a FIFO
		// would block for good.
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			continue
		}
		offset := max(0, info.Size()-4096)
		data, _ := io.ReadAll(io.NewSectionReader(file, offset, 4096))
		file.Close()
		lines := bytes.FieldsFunc(data, func(r rune) bool { return r == '\n' || r == '\r' })
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(string(lines[i]))
			if line == "" {
				continue
			}
			if r := []rune(line); len(r) > 200 {
				return string(r[:199]) + "…"
			}
			return line
		}
	}
	return ""
}

// frontDeskLive is the sessions the deck follows with the floor and desk each
// sits at: the ground floor's by its key, the others' by the building. A hire
// in T3 Code's desk is its live session here, so the team pairs it by its id.
func (d *Deck) frontDeskLive() []liveSession {
	d.mu.Lock()
	defer d.mu.Unlock()
	keys, overflow := d.layout()
	type seat struct{ floor, desk int }
	seats := map[string]seat{}
	for i, key := range keys {
		if key.session != nil {
			seats[key.session.ID] = seat{0, i}
		}
	}
	above, _ := d.seatedBuilding(overflow)
	for floor, people := range above {
		if floor == 0 {
			continue
		}
		for _, p := range people {
			seats[p.person.ID] = seat{floor, p.desk}
		}
	}
	var out []liveSession
	all := d.sessions.All()
	for _, one := range d.t3Seated() {
		all = append(all, one.session)
	}
	for _, s := range all {
		l := liveSession{ID: s.ID, Agent: string(s.Agent), Folder: s.Folder, Project: s.Project,
			State: string(s.State), Model: s.Model, LastEventAt: s.LastEventAt}
		if at, ok := seats[s.ID]; ok {
			floor, desk := at.floor, at.desk
			l.Floor = &floor
			if desk >= 0 {
				l.Desk = &desk
			}
		}
		out = append(out, l)
	}
	return out
}

// team answers GET /setup/gm/team.
func (rt *router) team(w http.ResponseWriter) {
	live := rt.deck.frontDeskLive()
	file := readRegistry(registryPath(), false)
	now := float64(time.Now().UnixMilli())
	writeJSON(w, http.StatusOK, map[string]any{
		"at": int64(now), "team": t3Decorate(buildTeam(file.hires, live, now, statusLine), file.hires, rt.deck.t3), "contacts": file.contactsJSON(),
	})
}

// registryPath is where the registry is kept; a var-free function so a test's
// temporary support folder applies.
func registryPath() string { return filepath.Join(supportDir, "registry.json") }
