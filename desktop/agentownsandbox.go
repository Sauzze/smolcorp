package desktop

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"
)

// Each agent's own sandbox, switched off inside a desk's (SA-M10, SANDBOX-PLAN AQ7 and §A4.3's "nested" row). A hire
// that runs in Smolcorp's sandbox is walled off by it; the agent's own would be a second sandbox inside it, which
// macOS doesn't allow (sandbox-exec inside sandbox-exec fails), so the agent's every command would fail. So it is
// switched off, by words each agent's own documentation gives, pinned per version: an agent older than the oldest
// version a row was checked on, or one whose version can't be told, gets no desk, and says so. An agent with no row
// (OpenCode, Grok) has no sandbox of its own to switch off and starts as it is.
//
// Only a sandboxed desk's agent gets these: off a Mac, until each system's sandbox lands, an agent keeps its own.

// ownSandboxOff is how one agent's own sandbox is switched off, from a version on.
type ownSandboxOff struct {
	agent Agent
	// least is the oldest version these words were checked on.
	least [3]int
	// words go right after the agent's program, before its own arguments; set is added to its environment.
	words []string
	set   map[string]string
}

// ownSandboxesOff are the rows, each agent's in order of least: a version takes the last row it is no older than.
var ownSandboxesOff = []ownSandboxOff{
	// Claude Code: its settings' sandbox, in the settings given for this run alone (an engine desk's inline settings
	// say the same, hiresettings.go). It adds to the account's settings, whose hooks still report.
	{agent: Claude, least: [3]int{2, 0, 0}, words: []string{"--settings", `{"sandbox":{"enabled":false}}`}},
	// Codex: the config override, which every subcommand takes (a --sandbox flag is the interactive one's alone), and
	// which wins over config.toml and a project's own.
	{agent: Codex, least: [3]int{0, 20, 0}, words: []string{"-c", `sandbox_mode="danger-full-access"`}},
	// Gemini CLI: its variable, which wins over its --sandbox flag and every settings file (a project's .env doesn't
	// replace a variable already set).
	{agent: Gemini, least: [3]int{0, 1, 0}, set: map[string]string{"GEMINI_SANDBOX": "false"}},
}

// agentVersionWait is how long an agent has to say its version.
const agentVersionWait = 10 * time.Second

// agentVersionOf asks program its version (`--version`): what it said, and whether it said it. A variable, for the
// tests.
var agentVersionOf = func(program string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), agentVersionWait)
	defer cancel()
	var out bytes.Buffer
	command := versionCommand(ctx, program)
	command.Stdout = &out
	if command.Run() != nil {
		return "", false
	}
	return out.String(), true
}

// ownSandboxOffFor is how agent's own sandbox is switched off for program, as it says its version: the words to put
// before its arguments and the variables to set, or the refusal the owner sees. An agent with no row: none of either.
func ownSandboxOffFor(agent Agent, program string) (words []string, set map[string]string, refusal string) {
	var rows []ownSandboxOff
	for _, row := range ownSandboxesOff {
		if row.agent == agent {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return nil, nil, ""
	}
	least := versionText(rows[0].least)
	text, ok := agentVersionOf(program)
	if !ok || versionNumbers.FindString(text) == "" {
		return nil, nil, fmt.Sprintf("Update %s: Smolcorp couldn't tell which version is installed, and a desk in its sandbox "+
			"needs %s or later, whose own sandbox it can switch off. No desk was started.", agent.Name(), least)
	}
	found := -1
	for i, row := range rows {
		if claudeVersionAtLeast(text, row.least) {
			found = i
		}
	}
	if found < 0 {
		return nil, nil, fmt.Sprintf("Update %s: a desk in its sandbox needs %s or later, whose own sandbox it can switch off. "+
			"No desk was started.", agent.Name(), least)
	}
	return append([]string(nil), rows[found].words...), rows[found].set, ""
}

// versionText is v as x.y.z.
func versionText(v [3]int) string {
	return strconv.Itoa(v[0]) + "." + strconv.Itoa(v[1]) + "." + strconv.Itoa(v[2])
}
