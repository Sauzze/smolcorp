package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// The Claude accounts a hire can work on, as the Mac's ClaudeProfiles: the
// default one (~/.claude), then the ones saved in config.json's claudeProfiles
// that are well formed and still folders. Read only: Settings on the Mac
// writes the list (POST /setup/claude-accounts isn't this engine's yet), and a
// test copy, which never hires on the default account, names one of these.

const (
	defaultClaudeAccount = "default"
	longestAccountName   = 30
)

// claudeAccounts are the accounts now: the default, then the saved ones kept.
func claudeAccounts(c *Config, home string) []claudeProfile {
	first := claudeProfile{Name: defaultClaudeAccount, Dir: filepath.Join(home, ".claude")}
	list := []claudeProfile{first}
	var saved []struct {
		Name string `json:"name"`
		Dir  string `json:"dir"`
	}
	if raw, ok := c.unknown["claudeProfiles"]; ok {
		// A list that isn't one is none: the Mac drops a bad entry the same way.
		if json.Unmarshal(raw, &saved) != nil {
			var lossy []json.RawMessage
			_ = json.Unmarshal(raw, &lossy)
			saved = saved[:0]
			for _, entry := range lossy {
				var one struct {
					Name string `json:"name"`
					Dir  string `json:"dir"`
				}
				if json.Unmarshal(entry, &one) == nil {
					saved = append(saved, one)
				}
			}
		}
	}
	for _, s := range saved {
		profile, ok := checkedClaudeAccount(s.Name, s.Dir, home)
		if !ok {
			continue
		}
		taken := false
		for _, p := range list {
			taken = taken || p.Name == profile.Name || p.Dir == profile.Dir
		}
		if info, err := os.Stat(profile.Dir); taken || err != nil || !info.IsDir() {
			continue
		}
		list = append(list, profile)
	}
	return list
}

// checkedClaudeAccount is a saved account as the Mac's ClaudeProfiles.check
// keeps it: a name of letters, digits, dashes and underscores, a full path (~
// for home) that isn't ~/.claude itself.
func checkedClaudeAccount(name, dir, home string) (claudeProfile, bool) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > longestAccountName {
		return claudeProfile{}, false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return claudeProfile{}, false
		}
	}
	path := strings.TrimSpace(dir)
	if path == "~" || strings.HasPrefix(path, "~/") {
		path = home + path[1:]
	}
	if !strings.HasPrefix(path, "/") || strings.ContainsRune(path, 0) {
		return claudeProfile{}, false
	}
	path = filepath.Clean(path)
	if name == defaultClaudeAccount || path == filepath.Join(home, ".claude") {
		return claudeProfile{}, false
	}
	return claudeProfile{Name: name, Dir: path}, true
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
