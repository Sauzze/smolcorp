package claudeenv

import (
	"slices"
	"strings"
	"testing"
)

// The markers a Claude Code session leaves in what it starts, the one from the Windows report first, and a made-up
// one a later Claude Code could add: every one is dropped, in any case (Windows reads names without it). The three
// outside the namespace are the ones RT126ENVSCRUB1 B1 saw a real session export.
var sessionMarkers = []string{
	"CLAUDE_CODE_CHILD_SESSION", "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SSE_PORT", "CLAUDE_CODE_SESSION_ID",
	"CLAUDE_CODE_ENVIRONMENT_KIND", "CLAUDE_CODE_REMOTE_TOOLS_PIN_STORED_LOGIN", "CLAUDE_CODE_REMOTE",
	"CLAUDE_CODE_PARENT_SESSION_FROM_A_LATER_VERSION", "CLAUDE_CODE_SOME_HOOK_WRAPPER",
	"CLAUDE_PID", "CLAUDE_EFFORT", "AI_AGENT",
	"claudecode", "Claude_Code_Child_Session", "claude_code_entrypoint", "claude_pid", "Claude_Effort", "ai_agent",
}

// The owner's configuration: kept, whatever its case. Provider switches by family, a new provider's included.
var ownerConfiguration = []string{
	"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "CLAUDE_CODE_USE_A_LATER_PROVIDER",
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH", "CLAUDE_CODE_SKIP_VERTEX_AUTH", "CLAUDE_CODE_SKIP_FOUNDRY_AUTH",
	"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_API_KEY_HELPER_TTL_MS",
	"CLAUDE_CODE_CLIENT_CERT", "CLAUDE_CODE_CLIENT_KEY", "CLAUDE_CODE_CLIENT_KEY_PASSPHRASE",
	"CLAUDE_CODE_GIT_BASH_PATH", "CLAUDE_CODE_SHELL", "CLAUDE_CODE_SHELL_PREFIX",
	"CLAUDE_CODE_DISABLE_AUTO_MEMORY", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC", "CLAUDE_CODE_ENABLE_TELEMETRY",
	"CLAUDE_CODE_MAX_OUTPUT_TOKENS", "claude_code_use_bedrock", "Claude_Code_Git_Bash_Path",
}

// Outside the namespace: never looked at, the provider's credentials and the account's folder among them.
var notClaudeCodes = []string{
	"CLAUDE_CONFIG_DIR", "CLAUDE_SECURESTORAGE_CONFIG_DIR", "ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL", "AWS_REGION",
	"AWS_BEARER_TOKEN_BEDROCK", "ANTHROPIC_VERTEX_PROJECT_ID", "CLOUD_ML_REGION", "ANTHROPIC_FOUNDRY_RESOURCE",
	"CLAUDECODE_NOT", "XCLAUDECODE", "CLAUDE_CODEX", "CLAUDE_CODE", "PATH", "HOME",
	// The session's names outside the namespace go by their exact name: nothing that only starts or ends like one.
	"CLAUDE_PIDS", "CLAUDE_EFFORT_X", "AI_AGENTS", "MY_AI_AGENT", "CLAUDE_", "CLAUDE",
	// Smolcorp's own, never the scrub's (SIERRA, 15 Nov): the GM's token above all, and the launcher's session.
	"SMOLCORP_GM_TOKEN", "SMOLCORP_SESSION", "SMOLCORP_HIRE_KEY", "SMOLCORP_HIRE_PORTS", "SMOLCORP_PORT",
	"SMOLCORP_TEST_SUPPORT_DIR", "SMOLCORP_CLAUDE_NAMES", "SMOLCORP_CLAUDE_CODE_X", "smolcorp_gm_token",
}

// Every SMOLCORP_* name, whatever follows, is never a marker: the token a GM runs `smolcorp gm` with goes through
// every scrub with its value as it was.
func TestTheScrubNeverTouchesASmolcorpVariable(t *testing.T) {
	for _, rest := range append(slices.Clone(sessionMarkers), "GM_TOKEN", "SESSION", "", "CLAUDECODE", "CLAUDE_CODE_CHILD_SESSION") {
		if name := "SMOLCORP_" + rest; Marker(name) || Marker(strings.ToLower(name)) {
			t.Errorf("%s is dropped", name)
		}
	}
	env := []string{"CLAUDECODE=1", "SMOLCORP_GM_TOKEN=fake-gm-token", "CLAUDE_CODE_CHILD_SESSION=1", "SMOLCORP_SESSION=fake-session"}
	if got := Scrub(env); !slices.Equal(got, []string{"SMOLCORP_GM_TOKEN=fake-gm-token", "SMOLCORP_SESSION=fake-session"}) {
		t.Errorf("Scrub changed Smolcorp's own: %d entries left of the 2 wanted", len(got))
	}
}

func TestEverySessionMarkerIsDroppedAndTheOwnersConfigurationKept(t *testing.T) {
	for _, name := range sessionMarkers {
		if !Marker(name) {
			t.Errorf("%s is kept: a desk's Claude Code would take itself for that session's child", name)
		}
	}
	for _, name := range append(slices.Clone(ownerConfiguration), notClaudeCodes...) {
		if Marker(name) {
			t.Errorf("%s is dropped: it's the owner's, never a session's", name)
		}
	}
	// A family needs its whole shape: a SKIP_ that isn't a sign-in's, or a bare prefix, is a marker.
	for _, name := range []string{"CLAUDE_CODE_SKIP_SOMETHING", "CLAUDE_CODE_SKIP_", "CLAUDE_CODE_", "CLAUDE_CODE_USE", "CLAUDE_CODE_DISABLE_AUTO_MEMORY_X"} {
		if !Marker(name) {
			t.Errorf("%s is kept", name)
		}
	}
}

func TestScrubKeepsEverythingElseInItsOrderAndChangesNothingGiven(t *testing.T) {
	var env, want []string
	for i, name := range append(append(slices.Clone(sessionMarkers), ownerConfiguration...), notClaudeCodes...) {
		entry := name + "=fake-" + strings.Repeat("v", i%3) + "=x"
		env = append(env, entry)
		if !slices.Contains(sessionMarkers, name) {
			want = append(want, entry)
		}
	}
	given := slices.Clone(env)
	if got := Scrub(env); !slices.Equal(got, want) {
		t.Errorf("Scrub:\n got %q\nwant %q", got, want)
	}
	if !slices.Equal(env, given) {
		t.Error("the environment given was changed")
	}
	// An entry with no "=" (Windows' "=C:=C:\\" has an empty name) is never a marker.
	if got := Scrub([]string{"=C:=C:\\", "CLAUDECODE", "PATH=/bin"}); !slices.Equal(got, []string{"=C:=C:\\", "PATH=/bin"}) {
		t.Errorf("odd entries: %q", got)
	}
	if got := Scrub([]string{}); got == nil || len(got) != 0 {
		t.Errorf("an empty environment stays an empty one, never nil (the inherited one): %#v", got)
	}
}

func TestFoundNamesTheMarkersOnceEachAndNothingElse(t *testing.T) {
	env := []string{"PATH=/bin", "CLAUDE_CODE_SSE_PORT=1", "CLAUDECODE=1", "CLAUDE_CODE_USE_BEDROCK=1", "CLAUDECODE=1", "CLAUDE_CONFIG_DIR=/c", "CLAUDE_PID=1"}
	if got := Found(env); !slices.Equal(got, []string{"CLAUDECODE", "CLAUDE_CODE_SSE_PORT", "CLAUDE_PID"}) {
		t.Errorf("Found: %q", got)
	}
	if got := Found([]string{"PATH=/bin", "CLAUDE_CODE_USE_VERTEX=1", "CLAUDE_CODE_OAUTH_TOKEN=fake"}); len(got) != 0 {
		t.Errorf("the owner's configuration alone is found as a session: %q", got)
	}
}

// Setup's line goes by the markers only a running session sets (RT126ENVSCRUB1 N2): every one is a marker, and
// neither the owner's configuration nor a Claude Code setting the kept list lacks is taken for a session.
func TestOnlyASessionsOwnMarkersAreDefinite(t *testing.T) {
	for _, name := range []string{"CLAUDECODE", "claudecode", "CLAUDE_PID", "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_CODE_SESSION_ID",
		"CLAUDE_CODE_ENTRYPOINT", "Claude_Code_Sse_Port"} {
		if !Definite(name) || !Marker(name) {
			t.Errorf("%s: definite %v, a marker %v", name, Definite(name), Marker(name))
		}
	}
	for _, name := range append(append([]string{"CLAUDE_CODE_DISABLE_TERMINAL_TITLE", "CLAUDE_CODE_A_LATER_SETTING",
		"CLAUDE_EFFORT", "AI_AGENT"}, ownerConfiguration...), notClaudeCodes...) {
		if Definite(name) {
			t.Errorf("%s is taken for a session", name)
		}
	}
	for _, name := range definite {
		if !Marker(name) {
			t.Errorf("%s is definite and kept", name)
		}
	}
}
