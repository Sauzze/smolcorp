package desktop

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Sauzze/smolcorp/internal/acp"
)

// Agent is a coding agent the deck follows, as in the Mac app's Agent.swift.
//
// Every agent reports through the same hook, each in its own dialect.
// canonical translates them all into Claude Code's, so one state machine in
// Sessions serves every agent.
type Agent string

const (
	Claude   Agent = "claude"
	Codex    Agent = "codex"
	Gemini   Agent = "gemini"
	OpenCode Agent = "opencode"
	Grok     Agent = "grok"
)

var agents = []Agent{Claude, Codex, Gemini, OpenCode, Grok}

// acpKind is the agent kind of an agent that speaks the Agent Client Protocol (internal/acp): an Agent value is
// "acp:" and the id of its entry in knownACPAgents, as "acp:gemini". It isn't in agents: that list is the agents that
// report through hooks, and an ACP agent has no hook, no terminal and no keys. Smolcorp talks to it over its stdin
// and stdout, and its Tab (A3) answers Interrupt and Reject through the protocol, not by pressing keys.
const acpKind = "acp:"

// ACPAgent is the Agent for the known ACP agent with id.
func ACPAgent(id string) Agent { return Agent(acpKind + id) }

// IsACP says whether a is an ACP agent, known or not.
func (a Agent) IsACP() bool { return strings.HasPrefix(string(a), acpKind) }

// acpKnown is a's entry in the list of known ACP agents. Not an ACP agent, or one the list doesn't have: ok false.
func (a Agent) acpKnown() (known KnownACPAgent, ok bool) {
	if !a.IsACP() {
		return KnownACPAgent{}, false
	}
	return knownACPAgent(strings.TrimPrefix(string(a), acpKind))
}

// acpLaunch is the program and the arguments that start a as an ACP agent: the entry's own, never through a shell.
// The arguments are a copy.
func (a Agent) acpLaunch() (command string, args []string, ok bool) {
	known, ok := a.acpKnown()
	if !ok {
		return "", nil, false
	}
	return known.Command, slices.Clone(known.Args), true
}

func (a Agent) Name() string {
	if a.IsACP() {
		known, _ := a.acpKnown()
		return known.Name
	}
	return map[Agent]string{
		Claude: "Claude Code", Codex: "Codex", Gemini: "Gemini CLI", OpenCode: "OpenCode", Grok: "Grok",
	}[a]
}

// Key is a key the deck presses in a session.
type Key string

const (
	Enter  Key = "enter"
	Escape Key = "escape"
	Space  Key = "space"
	Up     Key = "up"
	Down   Key = "down"
	CtrlC  Key = "ctrl-c"
	// For buttons made in setup: Shift+Tab switches Claude Code's mode.
	TabKey   Key = "tab"
	ShiftTab Key = "shift-tab"
)

// digitKey is the digit n, 1 to 9: a menu's choice picked by its number (talkpick.go).
func digitKey(n int) (Key, bool) {
	if n < 1 || n > 9 {
		return "", false
	}
	return Key(strconv.Itoa(n)), true
}

// Keystroke is a key pressed Times times in a row.
type Keystroke struct {
	Key   Key `json:"key"`
	Times int `json:"times"`
}

func press(key Key) Keystroke { return Keystroke{Key: key, Times: 1} }

// Interrupt stops a running turn. Grok cancels on Ctrl+C and ignores Esc;
// OpenCode asks for Esc twice, so a single stray press can't stop it.
func (a Agent) Interrupt() Keystroke {
	switch a {
	case Grok:
		return press(CtrlC)
	case OpenCode:
		return Keystroke{Key: Escape, Times: 2}
	}
	return press(Escape)
}

// Reject declines a permission prompt. In Grok, Esc only moves focus off the
// prompt; Ctrl+C is what cancels the request.
func (a Agent) Reject() Keystroke {
	if a == Grok {
		return press(CtrlC)
	}
	return press(Escape)
}

// Pick is the key that picks choice n of a menu the agent shows, and whether there is one. Claude Code's menus, a
// permission prompt's and AskUserQuestion's, take the choice's digit and answer at once. Other agents' menus aren't
// picked by number from the office: their four keys are.
func (a Agent) Pick(n int) (Keystroke, bool) {
	key, ok := digitKey(n)
	if a != Claude || !ok {
		return Keystroke{}, false
	}
	return press(key), true
}

// NewConversation starts a fresh conversation in the same tab.
func (a Agent) NewConversation() string {
	if a == Claude || a == Gemini {
		return "/clear"
	}
	return "/new"
}

// Thinking effort set with /effort, and push-to-talk through /voice: both
// are Claude Code's own.
func (a Agent) HasEffortDial() bool { return a == Claude }
func (a Agent) HasVoice() bool      { return a == Claude }

// foregroundName is what the agent is called among the processes in its
// pane, as the Mac app's Agent.foregroundName: Gemini CLI says only node,
// which is enough inside the agent's own pane.
func (a Agent) foregroundName() string {
	if a == Gemini {
		return "node"
	}
	return string(a)
}

// isItself says whether commandLine, as `ps -o args=` gives it, is this
// agent: its own program, or Node or Bun running the agent's own package, as
// Claude Code from npm runs (node …/node_modules/@anthropic-ai/claude-code/cli.js)
// and Grok CLI from Bun. Node or Bun running anything else, as `npm run dev`
// or vite started in the pane after the agent crashed, is someone's work,
// not the agent. As the Mac app's Agent.isItself.
func (a Agent) isItself(commandLine string) bool {
	commandLine = strings.TrimSpace(commandLine)
	first, rest, _ := strings.Cut(commandLine, " ")
	if first == "" {
		return false
	}
	program := strings.Trim(strings.ToLower(baseName(first)), "-")
	if program != "node" && program != "bun" {
		return program == string(a) || program == a.foregroundName()
	}
	// The package the script is in, exactly: what follows the last
	// node_modules, with its scope. claude-code-router, say, isn't Claude Code.
	if i := strings.LastIndex(rest, "node_modules/"); i >= 0 {
		parts := strings.SplitN(strings.ToLower(rest[i+len("node_modules/"):]), "/", 3)
		pkg := parts[0]
		if strings.HasPrefix(pkg, "@") && len(parts) > 1 {
			pkg += "/" + parts[1]
		}
		return slices.Contains(a.packages(), pkg)
	}
	// A script of its own, outside any package: by its own name, exactly.
	for _, word := range strings.Fields(rest) {
		if !strings.HasPrefix(word, "-") {
			script := strings.ToLower(baseName(word))
			return strings.TrimSuffix(script, filepath.Ext(script)) == string(a)
		}
	}
	return false
}

// packages are the npm packages the agent is installed as, run by Node or Bun.
func (a Agent) packages() []string {
	return map[Agent][]string{
		Claude: {"@anthropic-ai/claude-code"}, Codex: {"@openai/codex"}, Gemini: {"@google/gemini-cli"},
		OpenCode: {"opencode-ai", "opencode"}, Grok: {"@vibe-kit/grok-cli"},
	}[a]
}

// baseName is the last part of a path, with either separator.
func baseName(path string) string {
	return path[strings.LastIndexAny(path, `/\`)+1:]
}

// agentOf says which agent sent event. The hook names it, except for Grok:
// it also runs Claude Code's hooks, and tells itself apart by its camelCase
// envelope.
func agentOf(event map[string]any, reported string) Agent {
	if _, ok := event["hookEventName"]; ok {
		return Grok
	}
	for _, a := range agents {
		if string(a) == reported {
			return a
		}
	}
	return Claude
}

// canonical is event in Claude Code's words, or nil for one the deck
// should not act on.
func (a Agent) canonical(event map[string]any) map[string]any {
	if a.IsACP() {
		return acpCanonical(event)
	}
	switch a {
	case Gemini:
		return renaming(event, map[string]string{
			"BeforeAgent": "UserPromptSubmit", "AfterAgent": "Stop",
			"BeforeTool": "PreToolUse", "AfterTool": "PostToolUse",
			// Gemini CLI notifies only to ask for a tool's permission.
			"Notification": "PermissionRequest",
		})
	case Grok:
		// A subagent's events are not its session's.
		if _, ok := event["subagentType"]; ok {
			return nil
		}
		claude := map[string]any{}
		for k, v := range event {
			claude[k] = v
		}
		for camel, snake := range map[string]string{"sessionId": "session_id", "toolName": "tool_name", "notificationType": "notification_type"} {
			if v, ok := event[camel]; ok {
				claude[snake] = v
			}
		}
		// Grok also fires Stop as a session ends; only a turn's end counts.
		if claude["hook_event_name"] == "Stop" {
			if reason, ok := event["reason"].(string); ok && reason != "end_turn" {
				return nil
			}
		}
		return renaming(claude, map[string]string{"StopCancelled": "Interrupt"})
	}
	// Codex uses Claude Code's names, and OpenCode's plugin sends them.
	return event
}

func renaming(event map[string]any, names map[string]string) map[string]any {
	renamed := map[string]any{}
	for k, v := range event {
		renamed[k] = v
	}
	if name, ok := event["hook_event_name"].(string); ok {
		if claude, ok := names[name]; ok {
			renamed["hook_event_name"] = claude
		}
	}
	return renamed
}

// acpCanonical is an ACP desk's event (acpdesk.go) in Claude Code's words. An ACP agent has no hook: the desk names
// each event by the protocol's own method or update, with the protocol's own camelCase fields, and only these
// fields are carried over, so nothing else an agent put in an update can pass for a Claude Code field.
//   - session/new is the session starting; session/closed, its end (the agent stopped, or was closed).
//   - session/prompt is a turn starting; session/prompt_done, its end, by its stopReason, or failed with "error".
//   - session/update's tool_call and tool_call_update are a tool starting and ending, by their status: a call that
//     comes in already done is only its end. Every other update changes nothing on the deck.
//   - session/request_permission is a permission ask.
func acpCanonical(event map[string]any) map[string]any {
	str := func(field string) string { s, _ := event[field].(string); return s }
	claude := map[string]any{"session_id": str("sessionId"), "cwd": str("cwd")}
	if path := str("transcriptPath"); path != "" {
		claude["transcript_path"] = path
	}
	name := ""
	switch str("hook_event_name") {
	case "session/new":
		name, claude["source"] = "SessionStart", "startup"
	case "session/closed":
		name, claude["reason"] = "SessionEnd", "other"
	case "session/prompt":
		name = "UserPromptSubmit"
	case "session/prompt_done":
		switch {
		case str("error") != "":
			name, claude["error"] = "StopFailure", str("error")
		case str("stopReason") == string(acp.StopCancelled):
			name = "Interrupt"
		default: // end_turn, max_tokens, max_turn_requests, refusal: the turn is over, and its answer is there to read
			name = "Stop"
		}
	case "session/request_permission":
		name = "PermissionRequest"
	case "session/update":
		switch str("status") {
		case "completed":
			name = "PostToolUse"
		case "failed":
			name = "PostToolUseFailure"
		default:
			if str("sessionUpdate") == "tool_call" {
				name = "PreToolUse"
			}
		}
		if kind := str("sessionUpdate"); kind != "tool_call" && kind != "tool_call_update" {
			name = ""
		}
	}
	if name == "" {
		return nil
	}
	claude["hook_event_name"] = name
	if id := str("toolCallId"); id != "" {
		claude["tool_use_id"] = id
		claude["tool_name"] = acpToolName(str("kind"))
		if input, ok := event["rawInput"].(map[string]any); ok {
			claude["tool_input"] = input
		}
	}
	return claude
}

// acpToolName is the Claude Code tool an ACP tool call's kind stands for, where there is one, so the office shows it
// as it shows Claude Code's: the protocol's kinds are read, edit, delete, move, search, execute, think, fetch,
// switch_mode and other.
func acpToolName(kind string) string {
	if name, ok := map[string]string{
		"read": "Read", "edit": "Edit", "search": "Grep", "execute": "Bash", "fetch": "WebFetch",
	}[kind]; ok {
		return name
	}
	return "Tool"
}
