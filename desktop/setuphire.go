package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// POST /setup/hire, setup's own hire sheet, as the Mac's BriefRoutes.hireRequest
// and Deck.hire(as:in:answers:…): {role, folder, answers, floor, worktree,
// model, nickname, account, apiKey, task, engine, scope}, each checked with the Mac's
// words, then hired as the office hires (hireAsWith), at the desk pinned to
// that folder if there's one, or at a free one. Answered with the desk it sits
// at, or, with Auto, the one it's expected to take as its agent reports.
//
// What reaches the hire: a role's Claude Code hire in tmux goes through the
// engine (hireengine.go) with its answers, task, nickname, model, account and
// fence; any other opens the old way, through the launcher, which asks the
// role's questions itself. Into T3 Code (`engine: "t3"`, experimental), the
// T3 Code lane (Deck.hireIntoT3), as the Mac's. On the API-key account
// (hireapikey.go), only a Claude Code role, only while the key is offered, and
// only through the engine.

// setupHire is what setup's hire asks for besides the role, checked.
type setupHire struct {
	Role    string
	Folder  string
	Answers []string
	Floor   int
	// The account by name ("" when there's one, or the role isn't Claude Code's), and its config folder ("" for the default).
	Account   string
	configDir string
	Model     string
	Nickname  string
	Worktree  bool
	Task      string
	// T3: into T3 Code. A named account (Account) or the API-key account (APIKey) is kept for T3 Code's own refusal.
	// Otherwise APIKey is a Claude Code hire on the API-key account.
	T3     bool
	APIKey bool
	// Scope: the GM alone, the project it is held to (gmscope.go), the folder it is hired into; "" for none.
	Scope string
}

// apply puts what setup asked for into an engine hire's request. Its worktree
// is the Mac's choice: a hire works in a worktree of its own when the manager
// is on or one is asked for; the General Manager only when asked.
func (h *setupHire) apply(r *engineRequest, managerOn bool) {
	r.Answers = h.Answers
	r.Task = h.Task
	r.Nickname = h.Nickname
	r.AccountName = h.Account
	r.Scope = h.Scope
	if h.Model != "" {
		r.Role.Model = h.Model
	}
	if h.configDir != "" {
		r.ConfigDir, r.DefaultAccount = h.configDir, false
	}
	if h.APIKey && !h.T3 {
		r.APIKey, r.AccountName = true, apiKeyAccountName
	}
	if r.Role.ID == gmRole {
		r.NoWorktree = !h.Worktree
	} else {
		r.NoWorktree = !managerOn && !h.Worktree
	}
}

const setupHireModelLongest = 60

// setupHireRequest checks a request's body as the Mac's BriefRoutes.hireRequest:
// the request, or a status and why not.
func setupHireRequest(body []byte, roles []Role, accounts []claudeProfile, home string) (setupHire, int, string) {
	var given map[string]any
	if json.Unmarshal(body, &given) != nil || given == nil {
		return setupHire{}, http.StatusBadRequest, "Send {role, folder, answers}."
	}
	id, _ := given["role"].(string)
	if id == "" {
		return setupHire{}, http.StatusBadRequest, "Say which role to hire."
	}
	i := slices.IndexFunc(roles, func(r Role) bool { return r.ID == id })
	if i < 0 {
		return setupHire{}, http.StatusNotFound, "That role isn't there any more. Pick another."
	}
	role := roles[i]
	model, _ := given["model"].(string)
	model = strings.Trim(model, " \t")
	if utf8.RuneCountInString(model) > setupHireModelLongest || strings.IndexFunc(model, func(r rune) bool {
		return r > 127 || !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:[]", r))
	}) >= 0 {
		return setupHire{}, http.StatusBadRequest, "A model is a name such as claude-sonnet-5-5."
	}
	nickname, _ := given["nickname"].(string)
	nickname = trimSwiftSpace(nickname)
	if utf8.RuneCountInString(nickname) > longestNickname || strings.ContainsAny(nickname, "\r\n  \u0085") {
		return setupHire{}, http.StatusBadRequest, "A nickname is one line of at most 40 characters."
	}
	t3, known := checkedHireEngine(given["engine"])
	if !known {
		return setupHire{}, http.StatusBadRequest, hireEngineRefusal
	}
	apiKey, _ := given["apiKey"].(bool)
	worktree, _ := given["worktree"].(bool)
	hire := setupHire{Role: id, Model: model, Nickname: nickname, Worktree: worktree, T3: t3}
	// With more than one Claude account, a hire on Claude Code says which; another agent has none to name.
	// A hire in T3 Code works on T3 Code's own account: what was named is kept, for its own refusal.
	if t3 {
		named, _ := given["account"].(string)
		hire.Account, hire.APIKey = strings.Trim(named, " \t"), apiKey
	} else if agentOrClaude(role.Agent) == Claude {
		named, _ := given["account"].(string)
		named = strings.Trim(named, " \t")
		if !apiKey || named != "" {
			if status, why := claudeAccountRefusal(named, accounts); status != 0 {
				return setupHire{}, status, why
			}
		}
		hire.Account = named
		if account, ok := claudeAccountNamed(named, accounts); ok && account.Name != claudeDefaultName {
			hire.configDir = account.Dir
		}
	}
	// The API-key account is a Claude Code hire's alone (the Mac's Deck.hire); T3 Code refuses it in its own words.
	if apiKey && !t3 {
		if agentOrClaude(role.Agent) != Claude {
			return setupHire{}, http.StatusBadRequest, apiKeyNotClaude
		}
		hire.APIKey = true
	}
	path, _ := given["folder"].(string)
	if path == "" {
		return setupHire{}, http.StatusBadRequest, "Say which folder to hire into."
	}
	folder, status, why := checkedSetupFolder(path, home)
	if status != 0 {
		return setupHire{}, status, why
	}
	hire.Folder = folder
	// Only the owner's hire of the GM names a scope: the GM's token never hires the gm role.
	if hire.Scope, status, why = checkedGMScope(given["scope"], hire, home); status != 0 {
		return setupHire{}, status, why
	}
	switch task := given["task"].(type) {
	case nil:
	case string:
		if utf8.RuneCountInString(task) > briefTaskLimit {
			return setupHire{}, http.StatusBadRequest, "The task is at most 8000 characters."
		}
		hire.Task = briefTask(task)
	default:
		return setupHire{}, http.StatusBadRequest, "The task is text."
	}
	// Answers: none sent is none to give (the Mac sends the brief []); anything but a list of text is refused.
	hire.Answers = []string{}
	switch list := given["answers"].(type) {
	case nil:
	case []any:
		if len(list) > roleQuestionsMost {
			return setupHire{}, http.StatusBadRequest, "Answers are a list of at most 8 texts."
		}
		for _, item := range list {
			text, ok := item.(string)
			if !ok {
				return setupHire{}, http.StatusBadRequest, "Answers are a list of at most 8 texts."
			}
			hire.Answers = append(hire.Answers, text)
		}
	default:
		return setupHire{}, http.StatusBadRequest, "Answers are a list of at most 8 texts."
	}
	if floor, ok := given["floor"].(float64); ok && floor == float64(int(floor)) && int(floor) >= 0 && int(floor) < floorCount {
		hire.Floor = int(floor)
	}
	return hire, 0, ""
}

// hireEngineRefusal is the Mac's answer to an engine but a terminal or T3 Code (HireDesk.HireOptions.checked).
const hireEngineRefusal = `Where it works is "t3" (T3 Code), or left out for a terminal.`

// checkedHireEngine is where a hire asks to work, read as the Mac's HireOptions.checked reads `engine`: T3 Code
// ("t3"), or a terminal (left out, null, "" or "terminal"). known is false for anything else.
func checkedHireEngine(given any) (t3, known bool) {
	switch engine := given.(type) {
	case nil:
		return false, true
	case string:
		return engine == t3Engine, engine == t3Engine || engine == "" || engine == "terminal"
	}
	return false, false
}

// checkedSetupFolder is where setup may hire into, as the Mac's
// BriefRoutes.checkedFolder: a folder that's there, inside your home folder
// but not your home folder itself, nor ~/Library or a hidden folder at its
// top, such as ~/.ssh. Links are followed first, so none leads out.
func checkedSetupFolder(given, home string) (string, int, string) {
	trimmed := trimSwiftSpace(given)
	expanded := trimmed
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		expanded = home + expanded[1:]
	}
	if !filepath.IsAbs(expanded) {
		return "", http.StatusBadRequest, "Give the folder's full path, such as ~/Projects/app."
	}
	folder, err := filepath.EvalSymlinks(filepath.Clean(expanded))
	if err != nil {
		return "", http.StatusBadRequest, "There's no folder at " + trimmed + "."
	}
	info, err := os.Stat(folder)
	if err != nil {
		return "", http.StatusBadRequest, "There's no folder at " + trimmed + "."
	}
	if !info.IsDir() {
		return "", http.StatusBadRequest, trimmed + " is a file, not a folder."
	}
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	home = filepath.Clean(home)
	if !strings.HasPrefix(folder, home+string(filepath.Separator)) {
		return "", http.StatusBadRequest, "Hire into a folder inside your home folder."
	}
	top, _, _ := strings.Cut(strings.TrimPrefix(folder, home+string(filepath.Separator)), string(filepath.Separator))
	if top == "Library" || strings.HasPrefix(top, ".") {
		machine := "computer's"
		if runtime.GOOS == "darwin" {
			machine = "Mac's"
		}
		return "", http.StatusBadRequest, "That's a folder for your " + machine + " or your apps' own files. Hire into a project folder."
	}
	return folder, 0, ""
}

// gmSetupHireRefusal is the Mac's BriefRoutes.gmHireRefusal: the GM's token
// never hires the gm role, whether or not a GM is in the registry.
func gmSetupHireRefusal(role string, who caller, hires []Hire) string {
	if who != callerGM || role != gmRole {
		return ""
	}
	if slices.ContainsFunc(hires, gmHired) {
		return secondGMRefusal
	}
	return onlyOwnerHiresGMRefusal
}

// setupSheetDesk is where setup's hire h sits, as the Mac's Deck.hire(as:in:answers:…)
// picks it: on the ground floor, the desk pinned to its folder if there's one
// (pinned), else a free one (with a fixed number of desks, one must be free);
// upstairs, none named. A status and why when there's no room.
func (d *Deck) setupSheetDesk(h setupHire) (desk string, expected any, pinned bool, status int, why string) {
	if h.Floor > 0 {
		return "", nil, false, 0, ""
	}
	d.mu.Lock()
	keys, _ := d.layout()
	fixed := d.config.Layout.Keys != Auto
	d.mu.Unlock()
	free, at := -1, -1
	for i, k := range keys {
		if k.pin == nil {
			if k.session == nil && free < 0 {
				free = i
			}
			continue
		}
		if folder, ok := existingFolder(*k.pin); ok && at < 0 && physical(folder) == h.Folder {
			at = i
		}
	}
	switch {
	case at >= 0:
		return strconv.Itoa(at), at, true, 0, ""
	case fixed && free < 0:
		return "", nil, false, http.StatusConflict, "Every desk on the ground floor is taken. Fire someone there, or free a desk, to make room."
	case free >= 0:
		return strconv.Itoa(free), free, false, 0, ""
	case len(keys) < mostKeys:
		return strconv.Itoa(len(keys)), len(keys), false, 0, ""
	}
	return "", nil, false, 0, ""
}

// setupSheetHired is setup's answer to hire h, hired: the desk it's expected
// at, or its floor. The folder it started in was put first among the recent
// ones by the hire itself (hiredInto, or hireWith's plain path).
func (d *Deck) setupSheetHired(h setupHire, expected any) map[string]any {
	if h.Floor > 0 {
		return map[string]any{"ok": true, "desk": nil, "floor": h.Floor, "session": nil}
	}
	return map[string]any{"ok": true, "desk": expected, "session": nil}
}

// hireFromSetupSheet hires h as the Mac's Deck.hire(as:in:answers:…): at the
// desk pinned to its folder if there's one, else at a free one (with a fixed
// number of desks, one must be free). The answer's body, or a status and why.
func (d *Deck) hireFromSetupSheet(h setupHire) (int, any, string) {
	desk, expected, _, status, why := d.setupSheetDesk(h)
	if status != 0 {
		return status, nil, why
	}
	status, message := d.hireAsWith(desk, h.Role, h.Floor, true, &h)
	if status != http.StatusNoContent {
		return status, nil, message
	}
	answer := d.setupSheetHired(h, expected)
	// A hire that went the plain way (another agent, a pinned desk): its task and answers weren't passed on.
	if message != "" {
		answer["note"] = message
	}
	return http.StatusOK, answer, ""
}

// setupHireRoute answers POST /setup/hire; false for any other request.
func (d *Deck) setupHireRoute(w http.ResponseWriter, method, path string, body io.Reader, who caller) bool {
	if method != http.MethodPost || path != "/setup/hire" {
		return false
	}
	data, _ := io.ReadAll(io.LimitReader(body, 1<<20))
	var named struct {
		Role string `json:"role"`
	}
	_ = json.Unmarshal(data, &named)
	if why := gmSetupHireRefusal(named.Role, who, officeHires()); why != "" {
		writeStatus(w)(http.StatusConflict, why)
		return true
	}
	d.mu.Lock()
	off := !d.config.HiringEnabled
	roles, saved := slices.Clone(d.config.Roles), slices.Clone(d.config.ClaudeProfiles)
	d.mu.Unlock()
	// The account folders are looked in without the lock, as setup's own list is (claudeaccounts.go).
	hire, status, why := setupHireRequest(data, roles, claudeAccountList(saved), homeDir)
	if status != 0 {
		writeStatus(w)(status, why)
		return true
	}
	// Hiring off is off for setup's sheet and the GM too: once the request is checked, the
	// Mac's Deck.hire refuses first, for every caller. Start here's own GM hire
	// (hireFromSetup) keeps its rule.
	if off {
		writeStatus(w)(http.StatusForbidden, hiringOff)
		return true
	}
	// On the API-key account: only while it's offered, refused in the Mac's words (or this computer's) before anything is made.
	if hire.APIKey && !hire.T3 {
		if offer := d.apiKeyOffer(); !offer.Available {
			writeStatus(w)(http.StatusConflict, offer.Refusal)
			return true
		}
	}
	hireInto := d.hireFromSetupSheet
	if hire.T3 {
		hireInto = func(h setupHire) (int, any, string) { return d.hireFromSetupSheetIntoT3(h, roles) }
	}
	status, answer, why := hireInto(hire)
	if answer == nil {
		writeStatus(w)(status, why)
		return true
	}
	writeJSON(w, status, answer)
	return true
}

// hireFromSetupSheetIntoT3 hires h into T3 Code, as the Mac's Deck.hire(as:in:answers:…)
// with `engine: "t3"`: through hireIntoT3, which refuses as the Mac does (the role, an
// account, hiring off, the hooks, the cooldown, the lane) before anything is made. The
// answer is the Mac's for any setup hire: the desk it's expected at, though a hire in
// T3 Code isn't seated at one (its agent reports with no tab).
func (d *Deck) hireFromSetupSheetIntoT3(h setupHire, roles []Role) (int, any, string) {
	i := slices.IndexFunc(roles, func(r Role) bool { return r.ID == h.Role })
	if i < 0 {
		return http.StatusNotFound, nil, "That role isn't there any more. Pick another."
	}
	_, expected, pinned, status, why := d.setupSheetDesk(h)
	if status != 0 {
		return status, nil, why
	}
	var desk *int
	if at, ok := expected.(int); ok {
		desk = &at
	}
	status, why = d.hireIntoT3(t3Ask{Role: roles[i], Folder: h.Folder, Nickname: h.Nickname, Model: h.Model, Task: h.Task,
		Answers: h.Answers, Pinned: pinned, Desk: desk, Floor: max(0, h.Floor), Account: h.Account, APIKey: h.APIKey})
	if status != http.StatusNoContent {
		return status, nil, why
	}
	// Into T3 Code, the folder asked for is put first among the recent ones, as the Mac's hireIntoT3 does.
	d.mu.Lock()
	d.config.noteHireFolder(h.Folder)
	d.config.save()
	d.mu.Unlock()
	return http.StatusOK, d.setupSheetHired(h, expected), ""
}

// noteHireFolder puts folder first among the folders hires last started in, as
// the Mac's Config.noteHire: what GET /setup/hire/folders offers next time.
func (c *Config) noteHireFolder(folder string) {
	c.RecentHireFolders = append([]string{folder}, slices.DeleteFunc(slices.Clone(c.RecentHireFolders), func(f string) bool { return f == folder })...)
	c.RecentHireFolders = c.RecentHireFolders[:min(len(c.RecentHireFolders), recentHireFoldersMost)]
}
