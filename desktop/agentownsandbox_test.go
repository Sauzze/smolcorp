package desktop

import (
	"slices"
	"strings"
	"testing"
)

// AQ7: each agent's own sandbox is switched off by words pinned per version; an agent too old for them, or whose
// version can't be told, gets no desk; an agent with no row starts as it is.
func TestEachAgentsOwnSandboxIsSwitchedOffByWordsPinnedToItsVersion(t *testing.T) {
	saved := agentVersionOf
	t.Cleanup(func() { agentVersionOf = saved })
	asked := []string{}
	version := map[string]string{}
	agentVersionOf = func(program string) (string, bool) {
		asked = append(asked, program)
		text, ok := version[program]
		return text, ok
	}
	for _, c := range []struct {
		row     string
		agent   Agent
		said    string // what --version says; "-" for nothing
		words   []string
		set     map[string]string
		refusal string // a part of it
	}{
		{"Codex, the oldest pinned", Codex, "codex-cli 0.20.0", []string{"-c", `sandbox_mode="danger-full-access"`}, nil, ""},
		{"Codex, newer", Codex, "codex-cli 0.46.0\n", []string{"-c", `sandbox_mode="danger-full-access"`}, nil, ""},
		{"Codex, too old", Codex, "codex-cli 0.19.9", nil, nil, "Update Codex: a desk in its sandbox needs 0.20.0 or later"},
		{"Codex, no version", Codex, "-", nil, nil, "Update Codex: Smolcorp couldn't tell which version is installed"},
		{"Codex, says no number", Codex, "codex", nil, nil, "couldn't tell which version"},
		{"Gemini CLI", Gemini, "0.9.0", nil, map[string]string{"GEMINI_SANDBOX": "false"}, ""},
		{"Claude Code", Claude, "2.1.400 (Claude Code)", []string{"--settings", `{"sandbox":{"enabled":false}}`}, nil, ""},
		{"Claude Code, too old", Claude, "1.0.99 (Claude Code)", nil, nil, "Update Claude Code: a desk in its sandbox needs 2.0.0"},
		{"OpenCode: none of its own", OpenCode, "-", nil, nil, ""},
		{"Grok: none of its own", Grok, "-", nil, nil, ""},
	} {
		program := "/bin/" + string(c.agent) + "-" + strings.ReplaceAll(c.row, " ", "")
		if c.said != "-" {
			version[program] = c.said
		}
		before := len(asked)
		words, set, refusal := ownSandboxOffFor(c.agent, program)
		if !slices.Equal(words, c.words) || len(set) != len(c.set) || set["GEMINI_SANDBOX"] != c.set["GEMINI_SANDBOX"] ||
			c.refusal == "" && refusal != "" || c.refusal != "" && !strings.Contains(refusal, c.refusal) {
			t.Errorf("%s: %q %v %q", c.row, words, set, refusal)
		}
		if c.refusal != "" && (words != nil || set != nil || !strings.HasSuffix(refusal, "No desk was started.")) {
			t.Errorf("%s: refused, yet given %q %v: %q", c.row, words, set, refusal)
		}
		// Only an agent with a row is asked its version, and it's the program the desk starts that's asked.
		if wantAsked := c.agent == Codex || c.agent == Gemini || c.agent == Claude; (len(asked) > before) != wantAsked ||
			wantAsked && asked[len(asked)-1] != program {
			t.Errorf("%s: asked %q", c.row, asked[before:])
		}
	}
	// The words handed out are a copy: a launcher that adds to them can't change the table.
	version["/bin/codex"] = "codex-cli 0.46.0"
	words, _, _ := ownSandboxOffFor(Codex, "/bin/codex")
	words[0] = "--changed"
	if again, _, _ := ownSandboxOffFor(Codex, "/bin/codex"); again[0] != "-c" {
		t.Errorf("the table changed: %q", again)
	}
}

// A version takes the last row it's no older than, so a later row pinned for a newer version wins over an older one.
func TestAnAgentsNewerRowWinsForANewerVersion(t *testing.T) {
	saved, savedRows := agentVersionOf, ownSandboxesOff
	t.Cleanup(func() { agentVersionOf, ownSandboxesOff = saved, savedRows })
	ownSandboxesOff = []ownSandboxOff{
		{agent: Codex, least: [3]int{0, 20, 0}, words: []string{"old"}},
		{agent: Codex, least: [3]int{0, 50, 0}, words: []string{"new"}},
	}
	for said, want := range map[string]string{"0.20.1": "old", "0.49.9": "old", "0.50.0": "new", "1.0.0": "new"} {
		agentVersionOf = func(string) (string, bool) { return said, true }
		if words, _, refusal := ownSandboxOffFor(Codex, "/bin/codex"); len(words) != 1 || words[0] != want || refusal != "" {
			t.Errorf("%s: %q %q, want %q", said, words, refusal, want)
		}
	}
}
