package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
)

// The owner's guard is held by the app, not by a file (SANDBOX-PLAN §3.4, SG1). Each desk the app starts gets
// its groups in the guard's own command line, `--groups <JSON>`, from the app's copy. hire-guard.json then only
// adds: a group it switches on and a rule it adds count at the next call, a group it switches off or a rule it
// takes away count for nothing, at once or later. So a hire that loosens the file loosens nothing.

// GroupsArgument is groups as `--groups` takes them: a guard file's own text, on one line.
func GroupsArgument(groups []Group) string {
	data, _ := json.Marshal(File{Groups: groups})
	return string(data)
}

// readGuardFile is the guard file's bytes, for Load and LoadAdds alike, read safely (SG0's ReadFile): a link, a
// pipe, a device, a hard link or a file over 64 KiB can't be read, at once.
func readGuardFile(path string) ([]byte, error) {
	return ReadFile(path)
}

// LoadAdds is what the file at path adds to the groups given in the command line, and what's wrong with it. A
// file that isn't there adds nothing (Load's default groups would put back a group the owner switched off).
func LoadAdds(path string) (groups []Group, problem string) {
	if path == "" {
		return nil, ""
	}
	data, err := readGuardFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, "can't read it: " + err.Error()
	}
	return Parse(data)
}

// Over is the groups in force: held, the app's copy, with what adds brings. A group is on if either has it on,
// and refuses every rule either lists; a group only the file names is added if the file has it on. Nothing in adds
// switches a group off or takes a rule away.
func Over(held, adds []Group) []Group {
	out := make([]Group, len(held))
	for i, g := range held {
		out[i] = g
		out[i].Commands = slices.Clone(g.Commands)
	}
	for _, g := range adds {
		i := slices.IndexFunc(out, func(o Group) bool { return o.Name == g.Name })
		if i < 0 {
			if g.On {
				out = append(out, Group{Name: g.Name, On: true, Commands: slices.Clone(g.Commands)})
			}
			continue
		}
		out[i].On = out[i].On || g.On
		for _, rule := range g.Commands {
			if !slices.Contains(out[i].Commands, rule) {
				out[i].Commands = append(out[i].Commands, rule)
			}
		}
	}
	return out
}

// Held is the groups in force for profile: held, as `--groups` gave them (a guard file's text), with what the
// file at path adds, and the merge and spend groups where neither names them (WithGates), and what's wrong with either. Groups that can't be read are a problem, as a broken file is:
// every command is refused.
func Held(held, path, profile string) (groups []Group, problem string) {
	core, why := Parse([]byte(held))
	if why != "" {
		return DefaultGroups(), "the groups in its own command line can't be read: " + why
	}
	adds, problem := LoadAdds(path)
	groups = WithGates(Over(core, adds))
	if profile == ProfileGM {
		groups = ForGM(groups)
	}
	return groups, problem
}

// The ConfigChange hook (`smolcorp guard --config`): Claude Code asks it before it applies a settings file that
// changed during a session. A change to the user's, the project's or the local settings is refused for the rest
// of the session: the desk's own settings are in its command line, and a hire that edits a settings file mid-
// session gets nothing from it until the desk's next start (the owner's B4). Managed settings can't be refused,
// and a skill that changed only adds, so both pass. Anything it can't read is refused.
const configRefusal = "Refused by Smolcorp's hire guard: this desk's settings don't change during its session. The change applies when the desk next starts."

// ConfigHook is the ConfigChange hook: 0 lets the change apply, 2 (with why on stderr) refuses it.
func ConfigHook(stdin io.Reader, stderr io.Writer) int {
	var change struct {
		Event  string `json:"hook_event_name"`
		Source string `json:"source"`
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err == nil && json.Unmarshal(data, &change) == nil && change.Event == "ConfigChange" &&
		(change.Source == "policy_settings" || change.Source == "skills") {
		return exitOK
	}
	fmt.Fprintln(stderr, configRefusal)
	return exitBlock
}

// exitBlock is Claude Code's "blocked" for a hook that answers with its exit code.
const exitBlock = 2
