package desktop

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// Role is one the office can hire, as the Mac app's Role: its name, the
// agent it is and how that agent starts (its model, and its effort where it
// has one), the instructions it starts on, and the badge it wears in the
// office. Plain text you own, kept on this computer: nothing runs until you
// start one with `smolcorp run --role`. The instructions are the agent's
// first message, there to read in its terminal, and nothing changes what the
// agent may do without asking.
type Role struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Agent string `json:"agent"`
	// Empty for the agent's own default.
	Model string `json:"model"`
	// Empty for the agent's own, or one of effortLevels for an agent with an
	// effort setting.
	Effort       string `json:"effort"`
	Instructions string `json:"instructions"`
	Badge        string `json:"badge"`
}

const (
	roleNameLimit         = 24
	roleModelLimit        = 64
	roleInstructionsLimit = 4000
	// The most roles of your own, besides the ones that come with the app.
	ownRolesMost = 12
)

var (
	// What a role can wear in the office: the shipped roles' own, then some
	// for yours. office.js draws them.
	roleBadges = []string{"glasses", "clipboard", "magnifier", "quill", "wrench", "star", "bulb", "headset", "flag", "shield",
		"rocket", "coffee", "heart"}
	// The roles that come with the app. Each reports and asks before it
	// changes anything.
	shippedRoles = []Role{
		{ID: "reviewer", Name: "Reviewer", Agent: "claude", Model: "opus", Effort: "high", Badge: "glasses",
			Instructions: "Review the changes on this branch against main. List the problems you find, most serious first, each with " +
				"its file and line and why it matters. Don't edit any files: report, and wait for me to say which to fix."},
		{ID: "tester", Name: "Tester", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "clipboard",
			Instructions: "Run this project's tests. If any fail, find out why, and explain each failure with the fix you'd make. " +
				"Don't change any code until I say so."},
		{ID: "bughunter", Name: "Bug hunter", Agent: "claude", Model: "opus", Effort: "high", Badge: "magnifier",
			Instructions: "Look through this project for bugs: wrong results, crashes, edge cases that aren't handled. Report each " +
				"one with how to reproduce it and how sure you are. Don't fix anything until I say which."},
		{ID: "docs", Name: "Docs writer", Agent: "claude", Model: "sonnet", Effort: "medium", Badge: "quill",
			Instructions: "Check the README and the docs against what the code actually does. List what's missing, wrong or out of " +
				"date, with the changes you'd make. Wait for my OK before editing any file."},
		{ID: "refactorer", Name: "Refactorer", Agent: "claude", Model: "opus", Effort: "high", Badge: "wrench",
			Instructions: "Find the code in this project that's hardest to follow or change. Propose one refactor at a time: what " +
				"you'd change, why, and what could break. Wait for my OK before editing, and keep each change small."},
	}
)

// UnmarshalJSON reads a role field by field: anything missing or of the
// wrong kind is empty, and checked decides what's kept.
func (r *Role) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)
	text := func(key string) string {
		var s string
		json.Unmarshal(raw[key], &s)
		return s
	}
	*r = Role{ID: text("id"), Name: text("name"), Agent: text("agent"), Model: text("model"), Effort: text("effort"),
		Instructions: text("instructions"), Badge: text("badge")}
	return nil
}

// checked is the role as kept, and false without an id or a name. An
// unknown agent is Claude Code; a model with anything but a model name's
// characters, an effort the agent hasn't, or an unknown badge is none.
func (r Role) checked() (Role, bool) {
	id := strings.TrimSpace(r.ID)
	name := strings.TrimSpace(strings.Join(strings.FieldsFunc(r.Name, func(c rune) bool { return c == '\n' || c == '\r' }), " "))
	name = strings.TrimSpace(string([]rune(name)[:min(len([]rune(name)), roleNameLimit)]))
	if len(id) < 1 || len(id) > 32 || strings.ContainsFunc(id, func(c rune) bool { return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') }) || name == "" {
		return Role{}, false
	}
	agent := Agent(r.Agent)
	if !slices.Contains(agents, agent) {
		agent = Claude
	}
	model := strings.TrimSpace(r.Model)
	if len(model) > roleModelLimit || strings.ContainsFunc(model, func(c rune) bool {
		return !(c < unicode.MaxASCII && (unicode.IsLetter(c) || unicode.IsDigit(c) || strings.ContainsRune("-._/:[]", c)))
	}) {
		model = ""
	}
	effort := ""
	if agent.HasEffortDial() && slices.Contains(effortLevels, r.Effort) {
		effort = r.Effort
	}
	// Lines and tabs stay; other control characters go.
	instructions := strings.TrimSpace(strings.Map(func(c rune) rune {
		if c != '\n' && c != '\t' && unicode.IsControl(c) {
			return -1
		}
		return c
	}, r.Instructions))
	instructions = string([]rune(instructions)[:min(len([]rune(instructions)), roleInstructionsLimit)])
	badge := ""
	if slices.Contains(roleBadges, r.Badge) {
		badge = r.Badge
	}
	return Role{ID: id, Name: name, Agent: string(agent), Model: model, Effort: effort, Instructions: instructions, Badge: badge}, true
}

// checkedRoles is roles as kept: each checked, each id once, and as many of
// your own as there's room for, in their order. None is an empty list.
func checkedRoles(roles []Role) []Role {
	shipped := func(id string) bool {
		return slices.ContainsFunc(shippedRoles, func(r Role) bool { return r.ID == id })
	}
	kept := []Role{}
	own := 0
	for _, given := range roles {
		role, ok := given.checked()
		if !ok || slices.ContainsFunc(kept, func(r Role) bool { return r.ID == role.ID }) {
			continue
		}
		if !shipped(role.ID) {
			if own == ownRolesMost {
				continue
			}
			own++
		}
		kept = append(kept, role)
	}
	return kept
}

// firstMessage is what the agent is first told: to settle which folder it
// works in, then the role's instructions. Started from a terminal with
// `smolcorp run --role`, it's in the folder you started it in, so it only
// asks you to confirm.
func (r Role) firstMessage(folder string) string {
	if homeDir != "" {
		if rest, err := filepath.Rel(homeDir, folder); err == nil && !strings.HasPrefix(rest, "..") {
			folder = filepath.Join("~", rest)
		}
	}
	settle := "You've started in " + folder + ". Before anything else, tell me that's where you'll work and wait for me to confirm it."
	if r.Instructions == "" {
		return settle
	}
	return settle + "\n\nThen, as my " + r.Name + ": " + r.Instructions
}

// arguments are how the role's agent starts: its model, its effort, and
// message as its first. Each option is written whole, with its value after
// `=`, and the message after `--` where it stands alone, so text starting
// with a dash is never read as an option. Grok starts as it is: how it takes
// these hasn't been checked.
func (r Role) arguments(message string) []string {
	var args []string
	if r.Model != "" && Agent(r.Agent) != Grok {
		args = append(args, "--model="+r.Model)
	}
	switch Agent(r.Agent) {
	case Claude:
		if r.Effort != "" {
			args = append(args, "--effort="+r.Effort)
		}
		return append(args, "--", message)
	case Codex:
		return append(args, "--", message)
	case Gemini:
		return append(args, "--prompt-interactive="+message)
	case OpenCode:
		return append(args, "--prompt="+message)
	}
	return nil
}

// roleAgents are the agents a role can be, for the setup page: whether each
// has an effort, and whether it takes a first message.
func roleAgents() []map[string]any {
	list := []map[string]any{}
	for _, agent := range agents {
		list = append(list, map[string]any{"id": agent, "name": agent.Name(), "effort": agent.HasEffortDial(), "starts": agent != Grok})
	}
	return list
}

// forPhone is what the phone is told of a role: never its instructions.
func (r Role) forPhone() map[string]any {
	return map[string]any{"id": r.ID, "name": r.Name, "badge": r.Badge}
}
