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
// model, nickname, account, apiKey, task, engine}, each checked with the Mac's
// words, then hired as the office hires (hireAsWith), at the desk pinned to
// that folder if there's one, or at a free one. Answered with the desk it sits
// at, or, with Auto, the one it's expected to take as its agent reports.
//
// What reaches the hire: a role's Claude Code hire in tmux goes through the
// engine (hireengine.go) with its answers, task, nickname, model, account and
// fence; any other opens the old way, through the launcher, which asks the
// role's questions itself. Into T3 Code and on the API-key account aren't this
// engine's yet: refused in a sentence rather than hired another way.

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
}

// apply puts what setup asked for into an engine hire's request. Its worktree
// is the Mac's choice: a hire works in a worktree of its own when the manager
// is on or one is asked for; the General Manager only when asked.
func (h *setupHire) apply(r *engineRequest, managerOn bool) {
	r.Answers = h.Answers
	r.Task = h.Task
	r.Nickname = h.Nickname
	r.AccountName = h.Account
	if h.Model != "" {
		r.Role.Model = h.Model
	}
	if h.configDir != "" {
		r.ConfigDir, r.DefaultAccount = h.configDir, false
	}
	if r.Role.ID == gmRole {
		r.NoWorktree = !h.Worktree
	} else {
		r.NoWorktree = !managerOn && !h.Worktree
	}
}

const (
	setupHireModelLongest = 60
	setupHireNotT3        = "Hiring into T3 Code isn't on this engine yet: hire in a terminal, or from the Swift engine."
	setupHireNotAPIKey    = "The API-key account isn't on this engine yet: hire on a Claude account."
)

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
	switch engine := given["engine"].(type) {
	case nil:
	case string:
		if engine == "t3" {
			return setupHire{}, http.StatusConflict, setupHireNotT3
		}
		if engine != "" && engine != "terminal" {
			return setupHire{}, http.StatusBadRequest, `Where it works is "t3" (T3 Code), or left out for a terminal.`
		}
	default:
		return setupHire{}, http.StatusBadRequest, `Where it works is "t3" (T3 Code), or left out for a terminal.`
	}
	apiKey, _ := given["apiKey"].(bool)
	worktree, _ := given["worktree"].(bool)
	hire := setupHire{Role: id, Model: model, Nickname: nickname, Worktree: worktree}
	// With more than one Claude account, a hire on Claude Code says which; another agent has none to name.
	if agentOrClaude(role.Agent) == Claude {
		named, _ := given["account"].(string)
		named = strings.Trim(named, " \t")
		if !apiKey || named != "" {
			if status, why := claudeAccountRefusal(named, accounts); status != 0 {
				return setupHire{}, status, why
			}
		}
		hire.Account = named
		if account, ok := claudeAccountNamed(named, accounts); ok && account.Name != defaultClaudeAccount {
			hire.configDir = account.Dir
		}
	}
	if apiKey {
		return setupHire{}, http.StatusConflict, setupHireNotAPIKey
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

// hireFromSetupSheet hires h as the Mac's Deck.hire(as:in:answers:…): at the
// desk pinned to its folder if there's one, else at a free one (with a fixed
// number of desks, one must be free). The answer's body, or a status and why.
func (d *Deck) hireFromSetupSheet(h setupHire) (int, any, string) {
	desk, expected := "", any(nil)
	if h.Floor == 0 {
		d.mu.Lock()
		keys, _ := d.layout()
		fixed := d.config.Layout.Keys != Auto
		d.mu.Unlock()
		free, pinned := -1, -1
		for i, k := range keys {
			if k.pin == nil {
				if k.session == nil && free < 0 {
					free = i
				}
				continue
			}
			if folder, ok := existingFolder(*k.pin); ok && pinned < 0 && physical(folder) == h.Folder {
				pinned = i
			}
		}
		switch {
		case pinned >= 0:
			desk, expected = strconv.Itoa(pinned), pinned
		case fixed && free < 0:
			return http.StatusConflict, nil, "Every desk on the ground floor is taken. Fire someone there, or free a desk, to make room."
		case free >= 0:
			desk, expected = strconv.Itoa(free), free
		case len(keys) < mostKeys:
			desk, expected = strconv.Itoa(len(keys)), len(keys)
		}
	}
	status, message := d.hireAsWith(desk, h.Role, h.Floor, true, &h)
	if status != http.StatusNoContent {
		return status, nil, message
	}
	d.mu.Lock()
	d.config.noteHireFolder(h.Folder)
	d.config.save()
	d.mu.Unlock()
	if h.Floor > 0 {
		return http.StatusOK, map[string]any{"ok": true, "desk": nil, "floor": h.Floor, "session": nil}, ""
	}
	return http.StatusOK, map[string]any{"ok": true, "desk": expected, "session": nil}, ""
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
		http.Error(w, why, http.StatusConflict)
		return true
	}
	d.mu.Lock()
	roles, accounts := slices.Clone(d.config.Roles), claudeAccounts(d.config, homeDir)
	d.mu.Unlock()
	hire, status, why := setupHireRequest(data, roles, accounts, homeDir)
	if status != 0 {
		http.Error(w, why, status)
		return true
	}
	status, answer, why := d.hireFromSetupSheet(hire)
	if answer == nil {
		http.Error(w, why, status)
		return true
	}
	writeJSON(w, status, answer)
	return true
}

// noteHireFolder puts folder first among the folders hires last started in, as
// the Mac's Config.noteHire: what GET /setup/hire/folders offers next time.
func (c *Config) noteHireFolder(folder string) {
	c.RecentHireFolders = append([]string{folder}, slices.DeleteFunc(slices.Clone(c.RecentHireFolders), func(f string) bool { return f == folder })...)
	c.RecentHireFolders = c.RecentHireFolders[:min(len(c.RecentHireFolders), recentHireFoldersMost)]
}
