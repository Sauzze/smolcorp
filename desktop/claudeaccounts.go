package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// The Claude Code accounts Smolcorp follows (ClaudeProfiles.swift): ~/.claude, always there as "default", and
// the config folders (CLAUDE_CONFIG_DIR) you confirmed in setup. Only names and folders are looked at here:
// never a token, a key or a credentials file. A folder that merely looks like an account is offered ("found")
// and nothing more, until it's confirmed. Setup and the hires read the same list, config.json's claudeProfiles
// checked by claudeAccountList: an account setup shows is one a hire can work on, and one it leaves out (a
// folder outside the home folder, say) no hire takes either, though every hire is still denied its settings and
// transcripts (claudeDenyProfiles). A test copy, which never hires on the default account, names one of these.
const (
	claudeDefaultName = "default"
	claudeNameLongest = 30
	// More than anyone has; keeps the list in setup short.
	claudeAccountsMost = 8
)

// claudeDefaultProfile is ~/.claude, the account used when none is named.
func claudeDefaultProfile() claudeProfile {
	return claudeProfile{Name: claudeDefaultName, Dir: filepath.Join(homeDir, ".claude")}
}

func isFolder(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

var errClaudeFolder = fmt.Errorf("Give the account's folder as a full path, such as ~/.claude-work.")

// insideHome says whether dir (already cleaned) is the home folder or in it, by its name and, where it is there,
// by where its links lead: a folder reached through a link that goes out is outside.
func insideHome(dir string) bool {
	within := func(root, path string) bool {
		rel, err := filepath.Rel(root, path)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	}
	home := filepath.Clean(homeDir)
	if !within(home, dir) {
		return false
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return true // not there: it is refused as "isn't a folder", and nothing is read
	}
	realHome, err := filepath.EvalSymlinks(home)
	return err == nil && within(realHome, real)
}

// isNameRune: a letter, a mark or a number of any kind, as the Mac's CharacterSet.alphanumerics has them.
func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r)
}

// claudeFolderPath is an account's folder as a cleaned full path (~ for the home folder), from its text alone:
// nothing on disk is looked at. A network or device path (\\server\share, //server/share, \\?\C:\) is refused
// before anything touches it, as opening one could reach out to another computer; so is a relative one.
func claudeFolderPath(given string) (string, bool) {
	dir := strings.TrimSpace(given)
	if rest, ok := strings.CutPrefix(dir, "~"); ok && (rest == "" || rest[0] == '/' || rest[0] == filepath.Separator) {
		dir = homeDir + rest
	}
	if strings.HasPrefix(dir, `\\`) || strings.HasPrefix(dir, "//") || !filepath.IsAbs(dir) || strings.Contains(dir, "\x00") {
		return "", false
	}
	return filepath.Clean(dir), true
}

// checkClaudeProfile is p as it is kept: a name of letters, digits, dashes and underscores, the folder a full
// path (~ for the home folder) that isn't ~/.claude itself.
func checkClaudeProfile(p claudeProfile) (claudeProfile, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" || len([]rune(name)) > claudeNameLongest || strings.ContainsFunc(name, func(r rune) bool {
		return !isNameRune(r) && r != '-' && r != '_'
	}) {
		return p, fmt.Errorf("A name is up to %d letters, digits, dashes or underscores.", claudeNameLongest)
	}
	dir, ok := claudeFolderPath(p.Dir)
	if !ok {
		return p, errClaudeFolder
	}
	if name == claudeDefaultName || dir == claudeDefaultProfile().Dir {
		return p, fmt.Errorf("\"default\" is ~/.claude, which is always there.")
	}
	if !insideHome(dir) {
		return p, errClaudeFolder
	}
	return claudeProfile{Name: name, Dir: dir}, nil
}

// looksLikeClaudeAccount: a Claude Code config folder holds a settings.json, a .claude.json or a sessions folder.
func looksLikeClaudeAccount(dir string) bool {
	for _, name := range []string{"settings.json", ".claude.json", "sessions"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// checkedClaudeProfiles is the list setup sends, checked whole: each entry well formed, a folder that is there
// and looks like an account, no name or folder twice. Nothing is kept if one fails.
func checkedClaudeProfiles(given []claudeProfile) ([]claudeProfile, error) {
	if len(given) > claudeAccountsMost {
		return nil, fmt.Errorf("That's the most accounts there can be: %d.", claudeAccountsMost)
	}
	list := []claudeProfile{}
	for _, p := range given {
		ok, err := checkClaudeProfile(p)
		if err != nil {
			return nil, err
		}
		switch {
		case !isFolder(ok.Dir):
			return nil, fmt.Errorf("%s isn't a folder on this computer.", ok.Dir)
		case !looksLikeClaudeAccount(ok.Dir):
			return nil, fmt.Errorf("%s doesn't look like a Claude Code folder: it has no settings.json, .claude.json or sessions.", ok.Dir)
		case slices.ContainsFunc(list, func(q claudeProfile) bool { return q.Name == ok.Name }):
			return nil, fmt.Errorf("Two accounts are called %s. Give each its own name.", ok.Name)
		case slices.ContainsFunc(list, func(q claudeProfile) bool { return q.Dir == ok.Dir }):
			return nil, fmt.Errorf("%s is listed twice.", ok.Dir)
		}
		list = append(list, ok)
	}
	return list, nil
}

// claudeAccountList is the default account, then the confirmed ones that are still folders: one badly formed,
// repeating a name or a folder, or whose folder has gone is left out.
func claudeAccountList(saved []claudeProfile) []claudeProfile {
	list := []claudeProfile{claudeDefaultProfile()}
	for _, p := range saved {
		ok, err := checkClaudeProfile(p)
		if err != nil || slices.ContainsFunc(list, func(q claudeProfile) bool { return q.Name == ok.Name || q.Dir == ok.Dir }) || !isFolder(ok.Dir) {
			continue
		}
		list = append(list, ok)
	}
	return list
}

// claudeDenyProfiles is every account whose settings and transcripts a hire is denied: claudeAccountList's, then
// each other saved folder given as a full path, whether or not a hire may work on it. A folder outside the home
// folder (one the Mac kept, say) is still fenced off, as the Mac fences it (HireSettings.swift), though no hire
// takes it. Those others are taken from their text alone (claudeFolderPath): a deny rule never reads its folder, so
// nothing on disk or on a share is touched for them.
func claudeDenyProfiles(saved []claudeProfile) []claudeProfile {
	list := claudeAccountList(saved)
	for _, p := range saved {
		dir, ok := claudeFolderPath(p.Dir)
		if !ok || slices.ContainsFunc(list, func(q claudeProfile) bool { return q.Dir == dir }) {
			continue
		}
		list = append(list, claudeProfile{Name: strings.TrimSpace(p.Name), Dir: dir})
	}
	return list
}

// claudeAccountNamed is the account a hire names; with none named, the only
// one there is (ClaudeProfiles.profile(named:)).
func claudeAccountNamed(name string, among []claudeProfile) (claudeProfile, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		if len(among) == 1 {
			return among[0], true
		}
		return claudeProfile{}, false
	}
	for _, p := range among {
		if p.Name == name {
			return p, true
		}
	}
	return claudeProfile{}, false
}

// claudeAccountRefusal is why a hire can't go ahead on account, or 0: with more
// than one account, it must name one that is there (ClaudeProfiles.hireRefusal).
func claudeAccountRefusal(account string, among []claudeProfile) (int, string) {
	names := make([]string, len(among))
	for i, p := range among {
		names[i] = p.Name
	}
	if name := strings.TrimSpace(account); name != "" {
		if _, ok := claudeAccountNamed(name, among); ok {
			return 0, ""
		}
		return http.StatusNotFound, "There's no Claude account called " + name + ". Pick one of: " + strings.Join(names, ", ") + "."
	}
	if len(among) > 1 {
		return http.StatusBadRequest, "Say which Claude account to hire on: " + strings.Join(names, ", ") + "."
	}
	return 0, ""
}

// suggestedClaudeName is "work" for .claude-work: a name of letters, digits and dashes.
func suggestedClaudeName(folder string) string {
	rest := strings.TrimLeft(strings.TrimPrefix(folder, ".claude"), "-_.")
	name := []rune(strings.Map(func(r rune) rune {
		if isNameRune(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, rest))
	if len(name) > claudeNameLongest {
		name = name[:claudeNameLongest]
	}
	if len(name) == 0 || string(name) == claudeDefaultName {
		return "account"
	}
	return string(name)
}

// foundClaudeAccounts is the folders in the home folder that look like a Claude Code config folder, ~/.claude-work
// say, with a settings.json or .claude.json inside: offered, never used until confirmed. Not ~/.claude, and not
// what is already confirmed.
func foundClaudeAccounts(confirmed []claudeProfile) []map[string]any {
	found := []map[string]any{}
	entries, _ := os.ReadDir(homeDir) // sorted by name
	for _, entry := range entries {
		folder := entry.Name()
		dir := filepath.Join(homeDir, folder)
		if !strings.HasPrefix(folder, ".claude") || folder == ".claude" || !isFolder(dir) ||
			slices.ContainsFunc(confirmed, func(p claudeProfile) bool { return p.Dir == dir }) {
			continue
		}
		for _, name := range []string{"settings.json", ".claude.json"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				found = append(found, map[string]any{"name": suggestedClaudeName(folder), "dir": dir})
				break
			}
		}
	}
	return found
}

// claudeConnectedIn says whether Smolcorp's hooks are in the account's settings.json, as isConnected says it of
// ~/.claude's: only what that check confirms is true.
func claudeConnectedIn(p claudeProfile) bool {
	if p.Dir == claudeDefaultProfile().Dir {
		return isConnected(Claude)
	}
	entries := entriesFor(Claude)
	if entries == nil {
		return false
	}
	settings, err := readSettings(filepath.Join(p.Dir, "settings.json"))
	if err != nil {
		return false
	}
	hooks, _ := settings["hooks"].(map[string]any)
	for _, event := range entries.events {
		if !hasOurs(hooks[event]) {
			return false
		}
	}
	return true
}

// claudeAccountsState is what setup and the tools are told: each account, whether its folder is there, and
// whether Smolcorp's hooks are in it.
func claudeAccountsState(saved []claudeProfile) map[string]any {
	profiles := []map[string]any{}
	list := claudeAccountList(saved)
	for _, p := range list {
		profiles = append(profiles, map[string]any{"name": p.Name, "dir": p.Dir, "default": p.Name == claudeDefaultName, "connected": claudeConnectedIn(p)})
	}
	return map[string]any{"profiles": profiles, "found": foundClaudeAccounts(list), "most": claudeAccountsMost}
}

// claudeAccounts answers GET and POST /setup/claude-accounts. A tool may read the list; only the page changes it
// (routes.go). A POST sends {profiles: [{name, dir}]}, the accounts besides ~/.claude, whole. This app keeps the
// list and nothing more: it doesn't put its hooks into an added account, nor take them out of one removed.
func (rt *router) claudeAccounts(w http.ResponseWriter, r *request) {
	d := rt.deck
	if r.Method == http.MethodPost {
		data, _ := io.ReadAll(r.Body)
		var given struct {
			Profiles []map[string]any `json:"profiles"`
		}
		if json.Unmarshal(data, &given) != nil || given.Profiles == nil {
			writeStatus(w)(http.StatusBadRequest, "Send {profiles: [{name, dir}]}.")
			return
		}
		asked := make([]claudeProfile, len(given.Profiles))
		for i, p := range given.Profiles {
			asked[i].Name, _ = p["name"].(string)
			asked[i].Dir, _ = p["dir"].(string)
		}
		profiles, err := checkedClaudeProfiles(asked)
		if err != nil {
			writeStatus(w)(http.StatusBadRequest, err.Error())
			return
		}
		d.mu.Lock()
		d.config.ClaudeProfiles = profiles
		d.config.save()
		d.mu.Unlock()
	}
	d.mu.Lock()
	saved := slices.Clone(d.config.ClaudeProfiles)
	d.mu.Unlock()
	writeJSON(w, http.StatusOK, claudeAccountsState(saved))
}
