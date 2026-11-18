package desktop

import (
	"encoding/json"
	"path/filepath"
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
// them by the project's last folder name. It returns the project and the name as kept.
func setNickname(layout *Layout, body []byte, places []namePlace, pins []*string) (project, name string, refused *nameRefusal) {
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
	project = nicknameProject(strings.TrimSpace(*given.Folder), places, pins)
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
		if name := officeName(team, project); name != "" {
			row["nickname"] = name
		}
	}
}

// nicknameProject is the project a typed folder stands for: a session's project or its folder
// (compared as the real path, so a link and its target agree), else a pinned project; "" for none.
func nicknameProject(folder string, places []namePlace, pins []*string) string {
	same := func(a, b string) bool {
		if a == "" || b == "" {
			return false
		}
		if filepath.Clean(a) == filepath.Clean(b) {
			return true
		}
		ra, errA := filepath.EvalSymlinks(a)
		rb, errB := filepath.EvalSymlinks(b)
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
