package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// POST /setup/guard, as the Mac's GuardFile.save: setup switches the guard's
// groups and keeps your own patterns, the page alone (the route table: a hire
// or the GM must never loosen its own fence). The body is {"groups": [{"name",
// "on"}], "extra": ["pattern"]}; a group left out keeps what it was, and so
// does extra if it is left out. The file is written whole or not at all, as
// the Mac writes it (Tests/fixtures/frontdesk/hire-guard.json), and the answer
// is GET /setup/guard's.

const (
	guardExtraMost      = 40
	guardPatternLongest = 200
)

// storedGuard is what the guard file holds, as GuardFile.Stored: each known
// group's switch and commands, your patterns, and groups this page doesn't
// know, kept as they are.
type storedGuard struct {
	on       map[string]bool
	commands map[string][]string
	extra    []string
	others   []map[string]any
}

// loadGuardFile reads the file as GuardFile.load: the defaults when it isn't
// there or is broken (problem then says what's wrong).
func loadGuardFile(path string) (storedGuard, string) {
	fresh := storedGuard{on: map[string]bool{}, commands: map[string][]string{}, extra: []string{}}
	for _, g := range guard.DefaultGroups() {
		fresh.on[g.Name] = true
		fresh.commands[g.Name] = rulesText(g.Commands)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fresh, ""
	}
	if err != nil {
		return fresh, "can't read it"
	}
	var object map[string]any
	if json.Unmarshal(data, &object) != nil || len(object) != 1 {
		return fresh, "it isn't a guard file"
	}
	list, ok := object["groups"].([]any)
	if !ok || len(list) == 0 {
		return fresh, "it isn't a guard file"
	}
	stored := storedGuard{on: map[string]bool{}, commands: map[string][]string{}, extra: []string{}}
	for name, on := range fresh.on {
		stored.on[name], stored.commands[name] = on, fresh.commands[name]
	}
	seen := map[string]bool{}
	for _, item := range list {
		entry, ok := item.(map[string]any)
		name, named := entry["name"].(string)
		on, switched := entry["on"].(bool)
		wrong := !ok || !named || strings.Trim(name, " \t") == "" || !switched || seen[name]
		for key := range entry {
			wrong = wrong || key != "name" && key != "on" && key != "commands"
		}
		if wrong {
			return fresh, "a group in it is wrong"
		}
		seen[name] = true
		// Commands that aren't a list of text are none, as the Mac reads them.
		commands := []string{}
		if given, ok := entry["commands"].([]any); ok {
			for _, c := range given {
				text, isText := c.(string)
				if !isText {
					commands = []string{}
					break
				}
				commands = append(commands, text)
			}
		}
		for _, c := range commands {
			if len(strings.Fields(c)) == 0 {
				return fresh, "a group in it has an empty command"
			}
		}
		switch _, known := fresh.on[name]; {
		case name == guardExtraGroup:
			stored.extra = commands
		case known:
			stored.on[name], stored.commands[name] = on, commands
		default:
			stored.others = append(stored.others, entry)
		}
	}
	return stored, ""
}

func rulesText(rules []guard.Rule) []string {
	text := make([]string, len(rules))
	for i, r := range rules {
		text[i] = string(r)
	}
	return text
}

// guardFileData is the file as GuardFile.data writes it: the six groups in the
// guard's order, then your patterns, then any group this page doesn't know.
func guardFileData(stored storedGuard) []byte {
	list := []any{}
	for _, g := range guard.DefaultGroups() {
		on, ok := stored.on[g.Name]
		commands, has := stored.commands[g.Name]
		if !has {
			commands = rulesText(g.Commands)
		}
		list = append(list, map[string]any{"name": g.Name, "on": !ok || on, "commands": commands})
	}
	list = append(list, map[string]any{"name": guardExtraGroup, "on": true, "commands": stored.extra})
	for _, other := range stored.others {
		list = append(list, other)
	}
	return append(swiftJSON(map[string]any{"groups": list}), '\n')
}

// guardPatternProblem is why a pattern of your own can't be kept, or "", as
// GuardFile.problem(withPattern:): a command's name (or names with |) first,
// then the words that must follow it.
func guardPatternProblem(pattern string) string {
	if utf8.RuneCountInString(pattern) > guardPatternLongest {
		return "A pattern is at most " + strconv.Itoa(guardPatternLongest) + " characters."
	}
	if strings.IndexFunc(pattern, swiftControl) >= 0 {
		return "A pattern is one line of plain words."
	}
	words := strings.Fields(pattern)
	if len(words) == 0 {
		return "A pattern can't be empty."
	}
	first := words[0]
	for _, name := range strings.Split(first, "|") {
		start, _ := utf8.DecodeRuneInString(name)
		ok := name != "" && (unicode.IsLetter(start) || unicode.IsNumber(start))
		for _, r := range name {
			ok = ok && (r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) || strings.ContainsRune("._+-", r))
		}
		if !ok {
			return `A pattern starts with the command's name, such as "terraform import"; "` + first + `" isn't one.`
		}
	}
	return ""
}

// saveGuard checks body, then writes the file whole: a status and why when refused.
func saveGuard(body []byte, path string) (int, string) {
	var given map[string]any
	bad := "Send {groups: [{name, on}], extra: [pattern]}."
	if json.Unmarshal(body, &given) != nil || given == nil {
		return http.StatusBadRequest, bad
	}
	for key := range given {
		if key != "groups" && key != "extra" {
			return http.StatusBadRequest, bad
		}
	}
	stored, _ := loadGuardFile(path)
	if value, ok := given["groups"]; ok {
		list, ok := value.([]any)
		if !ok {
			return http.StatusBadRequest, "groups is a list of {name, on}."
		}
		seen := map[string]bool{}
		for _, item := range list {
			entry, isObject := item.(map[string]any)
			if !isObject {
				return http.StatusBadRequest, "groups is a list of {name, on}."
			}
			name, named := entry["name"].(string)
			on, switched := entry["on"].(bool)
			if !named || !switched {
				return http.StatusBadRequest, "Each group is {name, on}."
			}
			if _, known := stored.on[name]; !known {
				return http.StatusBadRequest, "There is no group called " + name + "."
			}
			if seen[name] {
				return http.StatusBadRequest, "There are two groups called " + name + "."
			}
			seen[name] = true
			stored.on[name] = on
		}
	}
	if value, ok := given["extra"]; ok {
		list, isList := value.([]any)
		if !isList {
			return http.StatusBadRequest, "extra is a list of patterns."
		}
		patterns := []string{}
		for _, item := range list {
			text, isText := item.(string)
			if !isText {
				return http.StatusBadRequest, "extra is a list of patterns."
			}
			patterns = append(patterns, strings.Trim(text, " \t"))
		}
		if len(patterns) > guardExtraMost {
			return http.StatusBadRequest, "At most " + strconv.Itoa(guardExtraMost) + " patterns of your own."
		}
		unique := []string{}
		for _, p := range patterns {
			if why := guardPatternProblem(p); why != "" {
				return http.StatusBadRequest, why
			}
			if !containsString(unique, p) {
				unique = append(unique, p)
			}
		}
		stored.extra = unique
	}
	if writePrivate(path, guardFileData(stored)) != nil {
		return http.StatusInternalServerError, "Couldn't write the guard file."
	}
	return 0, ""
}

func containsString(list []string, text string) bool {
	for _, item := range list {
		if item == text {
			return true
		}
	}
	return false
}

// guardRoute answers POST /setup/guard; false for any other request.
func guardRoute(w http.ResponseWriter, method, path string, body io.Reader) bool {
	if method != http.MethodPost || path != "/setup/guard" {
		return false
	}
	data, _ := io.ReadAll(io.LimitReader(body, 1<<20))
	file := filepath.Join(supportDir, hireGuardFile)
	if status, why := saveGuard(data, file); status != 0 {
		http.Error(w, why, status)
		return true
	}
	writeJSON(w, http.StatusOK, guardState(file, runtime.GOOS))
	return true
}
