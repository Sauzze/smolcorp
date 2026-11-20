package desktop

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// POST /setup/gm/nickname with {folder, name}: the General Manager names one person in
// the office, the way the setup page's outfit maker does: only that project's member
// name in the saved layout changes. The name is trimmed, one line of at most
// nicknameLimit characters; an empty one takes the nickname away. (The GM can already
// change any desk through /setup/layout, so this gives it no new power.)

// namePlace is where a session works now: its project, and the folder it is in.
type namePlace struct{ project, folder string }

// nameRefusal is a status and the words for it.
type nameRefusal struct {
	status  int
	message string
}

// setNickname names the person at folder in layout, or says why not. The person is whoever a session runs
// for in that folder (its project or where it is now) or the project pinned to a desk; the layout names
// them by the project's last folder name. It returns the project and the name as kept. held is the scope a GM
// held to one project is held to (request.heldTo), nil for none: its folder is the one its gate checked, links
// followed, and is still that folder and held here, and never resolved again (RT126POLISHBR2 N1).
func setNickname(layout *Layout, body []byte, places []namePlace, pins []*string, held *gmScope) (project, name string, refused *nameRefusal) {
	var given struct {
		Folder *string `json:"folder"`
		Name   *string `json:"name"`
	}
	if json.Unmarshal(body, &given) != nil || given.Folder == nil || given.Name == nil || strings.TrimSpace(*given.Folder) == "" {
		return "", "", &nameRefusal{400, "Send {folder, name}: the project's folder, and the nickname (empty for none)."}
	}
	name = strings.TrimSpace(*given.Name)
	if utf8.RuneCountInString(name) > nicknameLimit || strings.ContainsAny(name, "\n\v\f\r\u0085\u2028\u2029") {
		return "", "", &nameRefusal{400, "A nickname is one line of at most 16 characters."}
	}
	folder := strings.TrimSpace(*given.Folder)
	if held != nil {
		if real, ok := held.place(folder); !ok || real != folder {
			return "", "", &nameRefusal{403, "This General Manager " + held.namesOnly()}
		}
	}
	project = nicknameProject(folder, held != nil, places, pins)
	if project == "" {
		return "", "", &nameRefusal{404, "Nobody works in that folder and nothing is pinned to it, so there is no one to name."}
	}
	key := lastPart(project)
	member, known := layout.Room.Team[key]
	if !known {
		member = TeamMember{}.checked()
	}
	member.Name = name
	if member == (TeamMember{}).checked() {
		delete(layout.Room.Team, key)
		return project, name, nil
	}
	if !known && len(layout.Room.Team) >= teamLimit {
		return "", "", &nameRefusal{409, "The office remembers 50 people already; that is the most."}
	}
	if layout.Room.Team == nil {
		layout.Room.Team = map[string]TeamMember{}
	}
	layout.Room.Team[key] = member
	return project, name, nil
}

// officeName is the nickname the office gives whoever works in project: its member's name in team, kept under the
// project's last folder name as setNickname keeps it and the page's deskName reads it; "" for none.
func officeName(team map[string]TeamMember, project string) string {
	if project == "" {
		return ""
	}
	return team[lastPart(project)].Name
}

// nameRows gives the team rows the registry has no nickname for the one the office shows, so `gm team` shows what
// `gm nickname` set (FXGMNICK): a session nobody hired by its project, as the office names its desk; a hire with no
// nickname of its own by its session's project, or away from its desk by where it started, as hiredInto keeps it. A
// hire's own nickname wins, whole (FXGMNICKR1): it is what mail and the CLI look a hire up by, and the office keeps
// only its first 16 characters, and doesn't follow a rename in the registry.
func nameRows(rows []map[string]any, hires []Hire, live []liveSession, team map[string]TeamMember) {
	for _, row := range rows {
		if row["registered"] == true && row["nickname"] != "" {
			continue
		}
		if name := officeName(team, rowProject(row, hires, live)); name != "" {
			row["nickname"] = name
		}
	}
}

// rowProject is the project the office names a team row's desk by (nameRows): its session's, else, for a hire away
// from its desk, where it started; "" for none.
func rowProject(row map[string]any, hires []Hire, live []liveSession) string {
	project := ""
	if id, _ := row["sessionId"].(string); id != "" {
		for _, l := range live {
			if l.ID == id {
				project = l.Project
			}
		}
	} else if row["registered"] == true {
		for _, h := range hires {
			if h.ID == row["id"] {
				project = startedIn(h)
			}
		}
	}
	return project
}

// shownNames is the name `gm team` shows for each hire, by its id, worked out as the team route does (nameRows): the
// registry's nickname, else the office's name for its desk. Mail, contacts, watches, a desk's preview and a GM's scope
// find a hire with no nickname of its own by that office name too, after its id, nickname and name, so a name that
// found a hire before still finds that one (FXGMNICKR1 O1). Without the deck's lock held.
func (d *Deck) shownNames(hires []Hire) map[string]string {
	live := d.frontDeskLive()
	d.mu.Lock()
	team := maps.Clone(d.config.Layout.Room.Team)
	d.mu.Unlock()
	rows := buildTeam(hires, live, 0, func(string) string { return "" })
	nameRows(rows, hires, live, team)
	shown := map[string]string{}
	for _, row := range rows {
		id, _ := row["id"].(string)
		if name, _ := row["nickname"].(string); row["registered"] == true && name != "" {
			shown[id] = name
		}
	}
	return shown
}

// shownMatch is the hires a name means by shownNames' office name alone: those with no nickname of their own whose
// desk the office calls that (asked, lowered, as findHire reads it), the open ones first.
func shownMatch(asked string, all []Hire, shown map[string]string, skip func(Hire) bool) []Hire {
	var named, open []Hire
	for _, h := range all {
		if h.Nickname != "" || shown[h.ID] == "" || strings.ToLower(shown[h.ID]) != asked || skip != nil && skip(h) {
			continue
		}
		named = append(named, h)
		if isOpen(h) {
			open = append(open, h)
		}
	}
	if len(open) > 0 {
		return open
	}
	return named
}

// keepTheirNicknames is what `gm nickname` on project's folder doesn't change in `gm team`: the nicknames of the open
// hires the registry gives one, whose desk the office names by that project's last folder name (rowProject). The
// registry names a hire by its id, with a nickname of up to 40 characters, and wins for hires (FXGMNICKR1); the
// office names a folder's desk, in 16, for every session there. So the folder can't say which hire to rename, and a
// rename from it would cut a long nickname: the answer says where a hire is renamed instead (FXGMNICKR1 O2).
func keepTheirNicknames(project string, rows []map[string]any, hires []Hire, live []liveSession) []string {
	var names []string
	for _, row := range rows {
		name, _ := row["nickname"].(string)
		if row["registered"] != true || name == "" {
			continue
		}
		i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == row["id"] })
		if p := rowProject(row, hires, live); i >= 0 && isOpen(hires[i]) && p != "" && lastPart(p) == lastPart(project) {
			names = append(names, name)
		}
	}
	return names
}

// nicknameKeptNote is the answer's note when a hire there keeps its own nickname (keepTheirNicknames), or "". held is
// whether a GM held to one project asked: its scope refuses the registry's edit (gmScopeRefusal), so it's told to
// ask the owner rather than pointed at that route (RT126POLISHB O1).
func nicknameKeptNote(names []string, held bool) string {
	if len(names) == 0 {
		return ""
	}
	who := names[0] + " keeps its own nickname"
	if len(names) > 1 {
		who = strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " keep their own nicknames"
	}
	rename := "To rename a hire, change its nickname in the registry (POST /setup/gm/registry with its id and nickname)."
	if held {
		rename = "To rename a hire, ask the owner to change its nickname in the registry."
	}
	return "Only the office's name for the desk changed: " + who + " from the registry, which gm team, mail and the gm " +
		"commands go by. " + rename
}

// nicknameProject is the project a typed folder stands for: a session's project or its folder
// (compared as the real path, so a link and its target agree), else a pinned project; "" for none. checked is a
// folder a held GM's scope checked, its links followed there: taken as it is, never followed again here.
func nicknameProject(folder string, checked bool, places []namePlace, pins []*string) string {
	same := func(a, b string) bool {
		if a == "" || b == "" {
			return false
		}
		if filepath.Clean(a) == filepath.Clean(b) {
			return true
		}
		ra, errA := filepath.EvalSymlinks(a)
		rb, errB := filepath.EvalSymlinks(b)
		if checked {
			rb, errB = b, nil // the folder as checked, links already followed
		}
		return errA == nil && errB == nil && ra == rb
	}
	for _, p := range places {
		if same(p.project, folder) {
			return p.project
		}
	}
	for _, p := range places {
		if same(p.folder, folder) {
			return p.project
		}
	}
	for _, pin := range pins {
		if pin != nil && same(*pin, folder) {
			return *pin
		}
	}
	return ""
}

// hireNamed is findWatchHire over the registry's hires, with the names `gm team` shows for them (shownNames). Without
// the deck's lock held.
func (d *Deck) hireNamed(name string) (Hire, *refusal) {
	hires := readRegistry(registryPath(), false).hires
	return findWatchHire(name, hires, d.shownNames(hires))
}
