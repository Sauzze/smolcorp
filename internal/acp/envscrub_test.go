package acp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// standInEnvOut names the file the stand-in writes the environment it was started with to (TestMain).
const standInEnvOut = "SMOLCORP_ACP_STANDIN_ENV_OUT"

// ENVSCRUB1: an ACP agent (which may be Claude Code, or start it) never gets a Claude Code session's markers from the
// environment its caller hands it; the rest of that environment, Smolcorp's own and the owner's provider, it does.
func TestAnACPAgentGetsNoClaudeSessionMarker(t *testing.T) {
	written := filepath.Join(t.TempDir(), "env")
	t.Setenv(standInEnvOut, written)
	for name, value := range map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_CHILD_SESSION": "fake-child", "CLAUDE_CODE_ENTRYPOINT": "cli",
		"CLAUDE_PID": "4242", "CLAUDE_EFFORT": "fake-effort", "AI_AGENT": "fake-agent", "CLAUDE_CONFIG_DIR": "/fake/config",
		"CLAUDE_CODE_USE_BEDROCK": "1", "AWS_BEARER_TOKEN_BEDROCK": "FAKEKEY-bedrock", "SMOLCORP_GM_TOKEN": "fake-gm-token"} {
		t.Setenv(name, value)
	}
	startStandIn(t, "plain", nil, Options{})
	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("the stand-in didn't write its environment: %v", err)
	}
	env := strings.Split(string(data), "\n")
	for _, name := range []string{"CLAUDECODE=", "CLAUDE_CODE_CHILD_SESSION=", "CLAUDE_CODE_ENTRYPOINT=", "CLAUDE_PID=", "CLAUDE_EFFORT=", "AI_AGENT="} {
		if slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, name) }) {
			t.Errorf("the agent got %s", strings.TrimSuffix(name, "="))
		}
	}
	for _, kept := range []string{"CLAUDE_CODE_USE_BEDROCK=1", "AWS_BEARER_TOKEN_BEDROCK=FAKEKEY-bedrock", "SMOLCORP_GM_TOKEN=fake-gm-token",
		"CLAUDE_CONFIG_DIR=/fake/config", standInEnv + "=plain"} {
		if !slices.Contains(env, kept) {
			t.Errorf("the agent lacks %s", strings.SplitN(kept, "=", 2)[0])
		}
	}
}
