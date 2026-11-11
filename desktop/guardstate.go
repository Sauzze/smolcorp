package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// GET /setup/guard, as the Mac's GuardFile.state answers it: the hire guard's
// groups, each on or off with what it refuses and what hires can do once it's
// off, your own patterns, and one sentence on what the guard can't do. The
// file is hire-guard.json in the support folder, which the guard itself reads
// at each command (internal/guard). Only the page changes it, with
// POST /setup/guard (routes.go): a hire, or the GM, must never loosen its own
// fence. Reading it changes nothing.

// guardGroupAbout is one group the page switches, with what it refuses in plain
// words, and what hires can do once it is off ("Hires will be able to …").
type guardGroupAbout struct {
	name, about, allows string
}

// guardGroupsAbout are the six groups, in the guard's order. Your own patterns are the "extra" group.
func guardGroupsAbout(goos string) []guardGroupAbout {
	machine := "computer"
	if goos == "darwin" {
		machine = "Mac"
	}
	return []guardGroupAbout{
		{"push", "Sending code to a remote: git push and its relatives.", "send code to a remote with git push."},
		{"deploy", "Putting things live: Vercel, Netlify, Docker pushes, kubectl, terraform, EAS builds.",
			"deploy: put things live with Vercel, Netlify, Docker pushes, kubectl, terraform or EAS builds."},
		{"publish", "Publishing packages and changing things on GitHub: npm publish, pull requests, releases, secrets.",
			"publish packages and change things on GitHub: npm publish, pull requests, releases and secrets."},
		{"cloud", "Cloud accounts: aws, gcloud, az and their kin.", "use your cloud accounts with aws, gcloud, az and the like."},
		{"remote", "Reaching other machines: ssh, scp, sftp, rsync to a host.", "reach other machines with ssh, scp, sftp and rsync."},
		{"local", "Driving your " + machine + ": tmux keystrokes, the GM's commands, osascript, stored passwords.",
			"drive your " + machine + ": tmux keystrokes, the GM's commands, osascript and your stored passwords."},
	}
}

const (
	guardExtraGroup       = "extra"
	guardExtraMost        = 40
	guardPatternLongest   = 200
	guardLimits           = "The guard reads the commands a hire types, so it can't stop a script the hire writes and then runs, a command built from variables, or python -c."
	guardNotWrittenAnswer = "Couldn't write the guard file."
)

// guardStored is what the file holds, as the Mac's GuardFile.Stored: each
// group's on and commands, your own patterns, and the groups this page
// doesn't know, kept as they are.
type guardStored struct {
	on       map[string]bool
	commands map[string][]string
	extra    []string
	others   []json.RawMessage
}

// guardDefaults is every group on with the guard's own commands.
func guardDefaults() guardStored {
	stored := guardStored{on: map[string]bool{}, commands: map[string][]string{}, extra: []string{}}
	for _, group := range guard.DefaultGroups() {
		stored.on[group.Name] = true
		for _, rule := range group.Commands {
			stored.commands[group.Name] = append(stored.commands[group.Name], string(rule))
		}
	}
	return stored
}

// jsonObject is raw read as a JSON object, false if it is anything else (null included).
func jsonObject(raw []byte) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(raw, &object) != nil {
		return nil, false
	}
	return object, true
}

// jsonList is raw read as a JSON list, false if it is anything else (null included).
func jsonList(raw []byte) ([]json.RawMessage, bool) {
	var list []json.RawMessage
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal(raw, &list) != nil {
		return nil, false
	}
	return list, true
}

// swiftBool is raw as the Mac's `as? Bool` reads JSON: true or false, or a
// number that is exactly 0 or 1.
func swiftBool(raw json.RawMessage) (value, ok bool) {
	if json.Unmarshal(raw, &value) == nil {
		return value, true
	}
	text := strings.TrimSpace(string(raw))
	if number, err := strconv.ParseFloat(text, 64); err == nil && text != "" && (text[0] == '-' || text[0] >= '0' && text[0] <= '9') &&
		(number == 0 || number == 1) {
		return number == 1, true
	}
	return false, false
}

// swiftString is raw as the Mac's `as? String` reads JSON: a string only.
func swiftString(raw json.RawMessage) (string, bool) {
	var text string
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '"' || json.Unmarshal(raw, &text) != nil {
		return "", false
	}
	return text, true
}

// swiftStrings is raw as the Mac's `as? [String]` reads JSON: a list of
// strings only, else nothing.
func swiftStrings(raw json.RawMessage) ([]string, bool) {
	list, ok := jsonList(raw)
	if !ok {
		return nil, false
	}
	texts := make([]string, 0, len(list))
	for _, item := range list {
		text, ok := swiftString(item)
		if !ok {
			return nil, false
		}
		texts = append(texts, text)
	}
	return texts, true
}

// loadGuard is what the file holds, or the defaults and what is wrong with
// it, as GuardFile.load reads it. A file that isn't there is not a problem.
func loadGuard(path string) (guardStored, string) {
	fresh := guardDefaults()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fresh, ""
	}
	if err != nil {
		return fresh, "can't read it"
	}
	object, ok := jsonObject(data)
	var list []json.RawMessage
	if ok && len(object) == 1 && object["groups"] != nil {
		list, ok = jsonList(object["groups"])
	}
	if !ok || len(object) != 1 || len(list) == 0 {
		return fresh, "it isn't a guard file"
	}
	entries := make([]map[string]json.RawMessage, len(list))
	for i, raw := range list {
		if entries[i], ok = jsonObject(raw); !ok {
			return fresh, "it isn't a guard file" // the Mac reads groups as a list of objects, or not at all
		}
	}
	stored := guardDefaults()
	seen := map[string]bool{}
	for i, entry := range entries {
		name, named := swiftString(entry["name"])
		on, hasOn := swiftBool(entry["on"])
		wrong := !named || trimSwiftWhitespace(name) == "" || !hasOn || seen[name]
		for key := range entry {
			wrong = wrong || key != "name" && key != "on" && key != "commands"
		}
		if wrong {
			return fresh, "a group in it is wrong"
		}
		seen[name] = true
		commands, _ := swiftStrings(entry["commands"]) // anything but a list of text is no commands, as on the Mac
		if commands == nil {
			commands = []string{}
		}
		for _, command := range commands {
			if len(strings.Fields(command)) == 0 {
				return fresh, "a group in it has an empty command"
			}
		}
		_, known := fresh.on[name]
		switch {
		case name == guardExtraGroup:
			stored.extra = commands
		case known:
			stored.on[name], stored.commands[name] = on, commands
		default:
			stored.others = append(stored.others, list[i])
		}
	}
	return stored, ""
}

// guardFileData is the file as it is written, in the Mac's GuardFile.data bytes
// (JSONSerialization's, swiftjson.go, and a newline): the six groups in the
// guard's order, then your patterns, then any group this page doesn't know, as it was.
func guardFileData(stored guardStored) []byte {
	list := []any{}
	for _, group := range guard.DefaultGroups() {
		on, ok := stored.on[group.Name]
		commands, has := stored.commands[group.Name]
		if !has {
			commands = guardDefaults().commands[group.Name]
		}
		list = append(list, map[string]any{"commands": commands, "name": group.Name, "on": !ok || on})
	}
	list = append(list, map[string]any{"commands": stored.extra, "name": guardExtraGroup, "on": true})
	for _, other := range stored.others {
		var kept any
		if json.Unmarshal(other, &kept) == nil {
			list = append(list, kept)
		}
	}
	return append(swiftJSON(map[string]any{"groups": list}), '\n')
}

// guardState is what GET /setup/guard answers.
func guardState(path, goos string) map[string]any {
	stored, problem := loadGuard(path)
	groups := []map[string]any{}
	for _, group := range guardGroupsAbout(goos) {
		isOn, ok := stored.on[group.name]
		groups = append(groups, map[string]any{"name": group.name, "on": !ok || isOn, "about": group.about, "allows": group.allows})
	}
	extra := stored.extra
	if extra == nil {
		extra = []string{}
	}
	state := map[string]any{"groups": groups, "extra": extra, "limits": guardLimits}
	if problem != "" {
		state["problem"] = "The guard file " + problem + ", so every group is on until you save here."
	}
	return state
}

// guardPatternProblem is why a pattern can't be kept, or "". A pattern is a
// command name (or names with |) and then the words that must follow it, as
// in the default groups. One that doesn't start with a command name would
// match nothing, or, written with * or an empty alternative, look as if it
// matched everything: both are refused.
func guardPatternProblem(pattern string) string {
	if utf8.RuneCountInString(pattern) > guardPatternLongest {
		return "A pattern is at most " + strconv.Itoa(guardPatternLongest) + " characters."
	}
	if strings.ContainsFunc(pattern, swiftControl) {
		return "A pattern is one line of plain words."
	}
	words := strings.Fields(pattern)
	if len(words) == 0 {
		return "A pattern can't be empty."
	}
	for _, name := range strings.Split(words[0], "|") {
		ok := name != "" && isASCIILetterOrDigit(name[0])
		for _, r := range name {
			ok = ok && (r < utf8.RuneSelf && isASCIILetterOrDigit(byte(r)) || strings.ContainsRune("._+-", r))
		}
		if !ok {
			return "A pattern starts with the command's name, such as \"terraform import\"; \"" + words[0] + "\" isn't one."
		}
	}
	return ""
}

func isASCIILetterOrDigit(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// guardSaving keeps two saves from reading the file at once and losing one's change.
var guardSaving sync.Mutex

// saveGuard is POST /setup/guard, as GuardFile.save: it checks a request,
// then writes the file whole or not at all, and answers the state as GET
// does. The body is {"groups": [{"name", "on"}], "extra": ["pattern"]}; a
// group left out keeps what it was, and so does extra if it is left out.
// A file that was broken is written fresh, with the change.
func saveGuard(body []byte, path, goos string) briefRouteAnswer {
	bad := func(why string) briefRouteAnswer { return briefRefusal(400, why) }
	given, ok := jsonObject(body)
	for key := range given {
		ok = ok && (key == "groups" || key == "extra")
	}
	if !ok {
		return bad("Send {groups: [{name, on}], extra: [pattern]}.")
	}
	guardSaving.Lock()
	defer guardSaving.Unlock()
	stored, _ := loadGuard(path)
	if raw, has := given["groups"]; has {
		list, ok := jsonList(raw)
		entries := make([]map[string]json.RawMessage, len(list))
		for i := range list {
			entries[i], ok = jsonObject(list[i])
			if !ok {
				break
			}
		}
		if !ok {
			return bad("groups is a list of {name, on}.")
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			name, named := swiftString(entry["name"])
			on, hasOn := swiftBool(entry["on"])
			if !named || !hasOn {
				return bad("Each group is {name, on}.")
			}
			if _, known := stored.on[name]; !known {
				return bad("There is no group called " + name + ".")
			}
			if seen[name] {
				return bad("There are two groups called " + name + ".")
			}
			seen[name] = true
			stored.on[name] = on
		}
	}
	if raw, has := given["extra"]; has {
		extra, ok := swiftStrings(raw)
		if !ok {
			return bad("extra is a list of patterns.")
		}
		patterns := make([]string, len(extra))
		for i, pattern := range extra {
			patterns[i] = trimSwiftWhitespace(pattern)
		}
		if len(patterns) > guardExtraMost {
			return bad("At most " + strconv.Itoa(guardExtraMost) + " patterns of your own.")
		}
		for _, pattern := range patterns {
			if why := guardPatternProblem(pattern); why != "" {
				return bad(why)
			}
		}
		unique := []string{}
		for _, pattern := range patterns {
			if !slices.Contains(unique, pattern) {
				unique = append(unique, pattern)
			}
		}
		stored.extra = unique
	}
	if writeBriefPrivately(path, guardFileData(stored)) != nil {
		return briefRefusal(500, guardNotWrittenAnswer)
	}
	return briefJSON(guardState(path, goos))
}
