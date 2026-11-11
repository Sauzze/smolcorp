package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"runtime"
	"strings"
)

// GET /setup/guard, as the Mac's GuardFile.state answers it: the hire guard's
// groups, each on or off with what it refuses and what hires can do once it's
// off, your own patterns, and one sentence on what the guard can't do. The
// file is hire-guard.json in the support folder, which the guard itself reads
// at each command (internal/guard). Only the page may change it, a route of
// its own; reading it changes nothing.

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
	guardExtraGroup = "extra"
	guardLimits     = "The guard reads the commands a hire types, so it can't stop a script the hire writes and then runs, a command built from variables, or python -c."
)

// guardFileState is which groups the file has on, your own patterns, and
// what's wrong with the file, if anything. A file that isn't there is not a
// problem: every group is on.
func guardFileState(path string) (on map[string]bool, extra []string, problem string) {
	on, extra = map[string]bool{}, []string{}
	fresh := func(why string) (map[string]bool, []string, string) { return map[string]bool{}, []string{}, why }
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return on, extra, ""
	}
	if err != nil {
		return fresh("can't read it")
	}
	var object map[string]json.RawMessage
	var list []map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil || len(object) != 1 || object["groups"] == nil ||
		json.Unmarshal(object["groups"], &list) != nil || len(list) == 0 {
		return fresh("it isn't a guard file")
	}
	known := map[string]bool{}
	for _, group := range guardGroupsAbout(runtime.GOOS) {
		known[group.name] = true
	}
	seen := map[string]bool{}
	for _, entry := range list {
		var name string
		var isOn *bool
		wrong := entry == nil || json.Unmarshal(entry["name"], &name) != nil || strings.TrimSpace(name) == "" ||
			json.Unmarshal(entry["on"], &isOn) != nil || isOn == nil || seen[name]
		for key := range entry {
			wrong = wrong || key != "name" && key != "on" && key != "commands"
		}
		if wrong {
			return fresh("a group in it is wrong")
		}
		seen[name] = true
		var commands []string
		if raw, ok := entry["commands"]; ok {
			_ = json.Unmarshal(raw, &commands)
		}
		for _, command := range commands {
			if len(strings.Fields(command)) == 0 {
				return fresh("a group in it has an empty command")
			}
		}
		switch {
		case name == guardExtraGroup:
			extra = append([]string{}, commands...)
		case known[name]:
			on[name] = *isOn
		}
	}
	return on, extra, ""
}

// guardState is what GET /setup/guard answers.
func guardState(path, goos string) map[string]any {
	on, extra, problem := guardFileState(path)
	groups := []map[string]any{}
	for _, group := range guardGroupsAbout(goos) {
		isOn, ok := on[group.name]
		groups = append(groups, map[string]any{"name": group.name, "on": !ok || isOn, "about": group.about, "allows": group.allows})
	}
	state := map[string]any{"groups": groups, "extra": extra, "limits": guardLimits}
	if problem != "" {
		state["problem"] = "The guard file " + problem + ", so every group is on until you save here."
	}
	return state
}
