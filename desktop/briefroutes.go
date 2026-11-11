package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Setup's brief routes, as the Mac's BriefRoutes answers them: the library,
// one brief whole, a role's brief as a hire would get it, "Restore defaults",
// and the folders a hire from setup can start in. Each takes what a request
// sent and gives what it's answered, so they can be tried without a deck.

// The most recent hire folders kept, as the Mac app keeps them (Config.recentHireFoldersMost).
const recentHireFoldersMost = 8

// briefRouteAnswer is what one of these routes answers: JSON, or a status with a message.
type briefRouteAnswer struct {
	status  int
	json    any
	message string
}

func (a briefRouteAnswer) write(w http.ResponseWriter) {
	if a.json != nil {
		writeJSON(w, a.status, a.json)
		return
	}
	writeStatus(w)(a.status, a.message)
}

func briefJSON(value any) briefRouteAnswer {
	return briefRouteAnswer{status: http.StatusOK, json: value}
}

func briefRefusal(status int, message string) briefRouteAnswer {
	return briefRouteAnswer{status: status, message: message}
}

// briefsList is GET /setup/briefs: the library, each brief's id, name and
// summary, with the questions it suggests for a role of your own.
func briefsList(library briefLibrary) briefRouteAnswer {
	return briefJSON(map[string]any{"briefs": library.entries()})
}

// briefMarkdown is GET /setup/briefs/<id>: the whole brief, as a picked file would be.
func briefMarkdown(id string, library briefLibrary) briefRouteAnswer {
	markdown, ok := library.template(id)
	if !ok {
		return briefRefusal(http.StatusNotFound, "There's no brief "+id+".")
	}
	return briefJSON(map[string]any{"markdown": markdown})
}

// briefsRestore is POST /setup/briefs/restore, "Restore defaults": the app's
// own briefs over those you edited. Only from the setup page (routes.go).
func briefsRestore(source fs.FS, folder string) briefRouteAnswer {
	count := restoreBriefDefaults(source, folder)
	if count == 0 {
		return briefRefusal(http.StatusInternalServerError, "Couldn't put the briefs back.")
	}
	return briefJSON(map[string]any{"ok": true, "restored": count})
}

// hireFolders is GET /setup/hire/folders: the folder set up for new hires
// (empty if none), and the folders hires last started in that are still there.
func hireFolders(folder string, recent []string) briefRouteAnswer {
	there := []string{}
	for _, path := range recent {
		if found, ok := existingFolder(path); ok {
			there = append(there, found)
		}
	}
	return briefJSON(map[string]any{"default": folder, "recent": there})
}

// briefAnswers are the answers a request sent: nil if it sent none, or an
// error for anything but a list of text.
func briefAnswers(given any) ([]string, error) {
	if given == nil {
		return nil, nil
	}
	list, ok := given.([]any)
	if !ok || len(list) > roleQuestionsMost {
		return nil, errors.New("Answers are a list of at most 8 texts.")
	}
	answers := make([]string, len(list))
	for i, item := range list {
		if answers[i], ok = item.(string); !ok {
			return nil, errors.New("Answers are a list of at most 8 texts.")
		}
	}
	return answers, nil
}

// briefRoleProblem is why a role's brief, as sent, can't be kept, in words
// setup shows as they are; "" when it can.
func briefRoleProblem(role Role) string {
	if role.Brief == nil {
		return ""
	}
	name := role.Name
	if name == "" {
		name = role.ID
	}
	switch role.Brief.Kind {
	case "library":
		if !slices.Contains(briefLibraryIDs, role.Brief.ID) {
			return name + ": there's no brief " + role.Brief.ID + "."
		}
	case "file":
		if _, err := briefFile(role.Brief.Text, role.Brief.FileName); err != nil {
			return name + ": " + err.Error()
		}
	}
	return ""
}

// briefPreviewInput is what POST /setup/briefs/preview is answered from, besides its body.
type briefPreviewInput struct {
	roles      []Role
	library    briefLibrary
	hireFolder string
	pins       []*string
	date       time.Time
	kept       string
}

// briefPreview is POST /setup/briefs/preview, with {role, answers?, folder?,
// pinned?}: the role's BRIEF.md as a hire would have it, or for a role on its
// own instructions, what its agent is first told. role is a role as the page
// has it, or the id of one of roles. A folder one of pins is, or pinned, is a
// pinned desk's, where its brief would be written.
func briefPreview(body []byte, in briefPreviewInput) briefRouteAnswer {
	var given map[string]json.RawMessage
	if json.Unmarshal(body, &given) != nil || given == nil {
		return briefRefusal(http.StatusBadRequest, "Send {role, answers}.")
	}
	var role Role
	found := false
	var id *string
	var object map[string]any
	if json.Unmarshal(given["role"], &id) == nil && id != nil {
		id := *id
		i := slices.IndexFunc(in.roles, func(r Role) bool { return r.ID == id })
		if i < 0 {
			return briefRefusal(http.StatusNotFound, "There's no role "+id+".")
		}
		role, found = in.roles[i], true
	} else if json.Unmarshal(given["role"], &object) == nil && object != nil {
		var decoded Role
		if json.Unmarshal(given["role"], &decoded) == nil {
			if problem := briefRoleProblem(decoded); problem != "" {
				return briefRefusal(http.StatusBadRequest, problem)
			}
			role, found = syncedRoles([]Role{decoded}, in.roles)[0].checked()
		}
	}
	if !found {
		return briefRefusal(http.StatusBadRequest, "That isn't a role this app can use.")
	}
	source := role.briefSource()
	role.Brief = &source
	var sent any
	if raw, ok := given["answers"]; ok {
		_ = json.Unmarshal(raw, &sent)
	}
	answers, err := briefAnswers(sent)
	if err != nil {
		return briefRefusal(http.StatusBadRequest, err.Error())
	}
	var folder string
	if raw, ok := given["folder"]; ok && json.Unmarshal(raw, &folder) == nil && folder != "" {
		folder = expandTilde(folder, homeDir)
	} else if saved, ok := existingFolder(in.hireFolder); ok {
		folder = saved
	} else {
		folder = hireFolderSuggestion(homeDir)
	}
	var sentPinned *bool
	_ = json.Unmarshal(given["pinned"], &sentPinned)
	pinned := sentPinned != nil && *sentPinned
	if sentPinned == nil {
		if here, ok := existingFolder(folder); ok {
			pinned = slices.ContainsFunc(in.pins, func(pin *string) bool {
				if pin == nil {
					return false
				}
				there, ok := existingFolder(*pin)
				return ok && there == here
			})
		}
	}
	made, err := makeBrief(briefRequest{role: role, library: in.library, folder: folder, pinned: pinned, answers: answers,
		date: in.date, kept: in.kept})
	if err != nil {
		return briefRefusal(http.StatusBadRequest, err.Error())
	}
	if made == nil {
		message := role.firstMessage(folder, map[bool]roleStart{true: hiredAtPinnedDesk, false: hiredIntoGeneralFolder}[pinned])
		if answers != nil {
			message = role.firstMessageAnswered(folder, answers)
		}
		return briefJSON(map[string]any{"kind": "text", "markdown": message, "firstMessage": message})
	}
	return briefJSON(map[string]any{"kind": source.Kind, "markdown": made.markdown, "firstMessage": made.firstMessage})
}

// briefRoute answers one of the brief routes, let in already as setup's own;
// false if the request isn't one of theirs.
func (d *Deck) briefRoute(w http.ResponseWriter, method, path string, body io.Reader) bool {
	switch {
	case method == http.MethodGet && path == "/setup/briefs":
		briefsList(editableBriefs(bundledBriefs(), briefTemplatesDir())).write(w)
	case method == http.MethodPost && path == "/setup/briefs/preview":
		data, _ := io.ReadAll(io.LimitReader(body, 1<<20))
		d.mu.Lock()
		in := briefPreviewInput{roles: d.config.Roles, hireFolder: d.config.HireFolder, pins: d.config.Pins}
		d.mu.Unlock()
		in.library, in.date, in.kept = editableBriefs(bundledBriefs(), briefTemplatesDir()), time.Now(), briefsKeptDir()
		briefPreview(data, in).write(w)
	case method == http.MethodPost && path == "/setup/briefs/restore":
		briefsRestore(bundledBriefs(), briefTemplatesDir()).write(w)
	case method == http.MethodGet && path == "/setup/hire/folders":
		d.mu.Lock()
		folder, recent := d.config.HireFolder, slices.Clone(d.config.RecentHireFolders)
		d.mu.Unlock()
		hireFolders(folder, recent).write(w)
	case method == http.MethodGet && strings.HasPrefix(path, "/setup/briefs/"):
		briefMarkdown(strings.TrimPrefix(path, "/setup/briefs/"), editableBriefs(bundledBriefs(), briefTemplatesDir())).write(w)
	default:
		return false
	}
	return true
}
