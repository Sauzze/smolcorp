package desktop

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/claudeenv"
)

// ENVSCRUB1: Smolcorp opened from a terminal inside a Claude Code session. Each row sets that session's markers in
// this test's own environment, with the owner's provider configuration beside them, and checks one launch path: no
// marker reaches the agent, and the owner's configuration does. The rows that start a stand-in agent are
// envscrub_unix_test.go's.

// inheritedMarkers are a Claude Code session's, the Windows report's first, one a later Claude Code could add, and
// the three a real session exports outside the CLAUDE_CODE_* namespace (RT126ENVSCRUB1 B1).
var inheritedMarkers = map[string]string{
	"CLAUDECODE": "1", "CLAUDE_CODE_CHILD_SESSION": "fake-child", "CLAUDE_CODE_ENTRYPOINT": "cli", "CLAUDE_CODE_SSE_PORT": "54321",
	"CLAUDE_CODE_SESSION_ID": "fake-session-id", "CLAUDE_CODE_ENVIRONMENT_KIND": "byoc", "CLAUDE_CODE_A_LATER_MARKER": "1",
	"CLAUDE_PID": "4242", "CLAUDE_EFFORT": "fake-effort", "AI_AGENT": "fake-agent",
}

// ownersProvider is the owner's configuration that must reach every desk: Bedrock and its credentials (FAKEKEY
// values only), Git Bash's place on Windows, and its own memory switch, which only the GM's start sets over.
var ownersProvider = map[string]string{
	"CLAUDE_CODE_USE_BEDROCK": "1", "CLAUDE_CODE_SKIP_BEDROCK_AUTH": "1", "AWS_REGION": "fake-region-1",
	"AWS_BEARER_TOKEN_BEDROCK": "FAKEKEY-bedrock", "CLAUDE_CODE_GIT_BASH_PATH": "/fake/git/bash",
	"CLAUDE_CODE_DISABLE_AUTO_MEMORY": "0",
}

// openedInsideClaudeCode puts the markers and the owner's configuration in this test's environment.
func openedInsideClaudeCode(t *testing.T) {
	t.Helper()
	for name, value := range inheritedMarkers {
		t.Setenv(name, value)
	}
	for name, value := range ownersProvider {
		t.Setenv(name, value)
	}
}

// openedOutsideClaudeCode takes every marker out of this test's environment, the session the tests run in included,
// and the names a Mac app opened inside one tells its core, which a desk of that app inherits (RT126ENVSCRUB1 N4).
func openedOutsideClaudeCode(t *testing.T) {
	t.Helper()
	for _, name := range append(claudeenv.Found(os.Environ()), claudeSessionVariable) {
		t.Setenv(name, "") // put back as it was after the test
		os.Unsetenv(name)
	}
}

// noMarkerAndTheOwnersKept fails the row when env has any marker (in any case), or lacks one of the owner's
// variables want names, with that value. Values are never printed: only names.
func noMarkerAndTheOwnersKept(t *testing.T, row string, env []string, want map[string]string) {
	t.Helper()
	if found := claudeenv.Found(env); len(found) != 0 {
		t.Errorf("%s: a Claude Code session's markers reach the agent: %q", row, found)
	}
	for name := range inheritedMarkers {
		if slices.ContainsFunc(env, func(e string) bool { key, _, _ := strings.Cut(e, "="); return strings.EqualFold(key, name) }) {
			t.Errorf("%s: %s reaches the agent", row, name)
		}
	}
	for name, value := range want {
		if got, ok := environ(env, name, false); !ok || got != value {
			t.Errorf("%s: the owner's %s doesn't reach the agent as it was (there: %v)", row, name, ok)
		}
	}
}

func TestHireEnvironmentHasNoClaudeSessionMarkerAndKeepsTheOwnersProvider(t *testing.T) {
	openedInsideClaudeCode(t)
	noMarkerAndTheOwnersKept(t, "hireEnvironment (tmux servers, hire windows, desk starts, the GM's sign-in check)", hireEnvironment(), ownersProvider)
	noMarkerAndTheOwnersKept(t, "the GM's sign-in check", signInEnvironment(""), ownersProvider)
	noMarkerAndTheOwnersKept(t, "t3 and T3 Code's app", t3Environment(), ownersProvider)
}

// smolcorp run on every system, Windows' (where the report came from) first among them: the GM's start as
// hirelauncher.go makes it (its token, its memory's start, the default account), over an environment that has the
// markers in any case. The GM's token reaches it, the owner's provider too, and no marker does. The launcher rig's
// real start environments are envscrub_unix_test.go's row.
func TestOnEverySystemTheLaunchersGMHasItsTokenAndNoClaudeSessionMarker(t *testing.T) {
	gmStart := hireStartEnv{Set: map[string]string{"SMOLCORP_GM_TOKEN": testGMToken, "DISABLE_AUTOUPDATER": "1"}, Unset: []string{"CLAUDE_CONFIG_DIR"}}
	for _, change := range claudeMemoryStart {
		if change.Unset {
			gmStart.Unset = append(gmStart.Unset, change.Name)
		} else {
			gmStart.Set[change.Name] = change.Value
		}
	}
	base := []string{"PATH=/fake/bin", "SMOLCORP_SESSION=fake-inherited", "claudecode=1", "Claude_Code_Child_Session=fake-child", "CLAUDE_CODE_SSE_PORT=1",
		"Claude_Pid=4242", "claude_effort=fake-effort", "AI_AGENT=fake-agent"}
	for name, value := range ownersProvider {
		base = append(base, name+"="+value)
	}
	for _, caseless := range []bool{false, true} {
		env := agentEnvironment(base, "fake-session", true, true, "", &gmStart, caseless)
		want := maps.Clone(ownersProvider)
		want[claudeMemoryEnv] = "1"
		noMarkerAndTheOwnersKept(t, "the GM", env, want)
		if token, ok := environ(env, "SMOLCORP_GM_TOKEN", caseless); !ok || token != testGMToken {
			t.Errorf("caseless %v: the GM's token is there %v (its value isn't printed)", caseless, ok)
		}
		if session, _ := environ(env, "SMOLCORP_SESSION", caseless); !strings.Contains(strings.Join(env, "\n"), "SMOLCORP_SESSION=fake-session") || session == "" {
			t.Errorf("caseless %v: the launcher's own session doesn't reach the GM", caseless)
		}
	}
}

// Summaries, sign-ins and the sign-in checks: no marker. Their provider rule is their own (summaryRemoved: a summary
// bills your plan, never a cloud), and unchanged; the rest of the owner's is kept.
func TestASummarysAndASignInsEnvironmentHaveNoClaudeSessionMarker(t *testing.T) {
	openedInsideClaudeCode(t)
	for row, env := range map[string][]string{"a summary": summaryEnvironment(os.Environ()), "a sign-in": loginEnvironment(os.Environ())} {
		noMarkerAndTheOwnersKept(t, row, env, map[string]string{"CLAUDE_CODE_GIT_BASH_PATH": "/fake/git/bash", "AWS_REGION": "fake-region-1"})
		if _, ok := environ(env, "CLAUDE_CODE_USE_BEDROCK", false); ok {
			t.Errorf("%s: a summary's own rule changed: it runs on Bedrock", row)
		}
	}
}

// MEM4b's GM start takes three launch modes out itself (claudeMemoryStart): each is one the scrub drops anyway, so
// the two can't disagree, and its memory switch is one the scrub keeps, so the owner's reaches a hire. The hook
// wrappers agentcheck.go keeps from a hire (wrapsHooks) are markers too, but the shell, which the owner's settings
// may name and a desk's settings set over.
func TestTheScrubAgreesWithTheGMsMemoryStartAndTheHookWrappers(t *testing.T) {
	for _, change := range claudeMemoryStart {
		if change.Unset != claudeenv.Marker(change.Name) {
			t.Errorf("%s: the GM's start unsets it %v, the scrub drops it %v", change.Name, change.Unset, claudeenv.Marker(change.Name))
		}
	}
	for _, name := range []string{"CLAUDE_CODE_SHELL", "CLAUDE_CODE_SHELL_PREFIX", "CLAUDE_CODE_PRE_HOOK", "CLAUDE_CODE_HOOKS_OFF"} {
		if !wrapsHooks(name) {
			t.Fatalf("%s: wrapsHooks changed", name)
		}
		if shell := strings.HasPrefix(name, "CLAUDE_CODE_SHELL"); claudeenv.Marker(name) == shell {
			t.Errorf("%s: the scrub drops it %v", name, claudeenv.Marker(name))
		}
	}
}

// Setup's line: shown when Smolcorp itself was started with a session's markers (its own environment, or the names
// the Mac app gives), never for the owner's configuration alone, in English and in French.
func TestSetupSaysWhenSmolcorpWasStartedInsideAClaudeCodeSession(t *testing.T) {
	for _, c := range []struct {
		row     string
		environ []string
		names   string
		want    bool
	}{
		{"its own environment", []string{"PATH=/bin", "CLAUDE_CODE_CHILD_SESSION=1"}, "", true},
		{"CLAUDECODE, in every session's children", []string{"claudecode=1", "CLAUDE_CODE_A_LATER_MARKER=1"}, "", true},
		{"a session's claude's pid", []string{"CLAUDE_PID=4242"}, "", true},
		{"the Mac app's names", []string{"PATH=/bin"}, "CLAUDECODE,CLAUDE_CODE_ENTRYPOINT", true},
		{"the Mac app's names, the owner's alone", []string{"PATH=/bin"}, "CLAUDE_CODE_OAUTH_TOKEN, CLAUDE_CODE_USE_VERTEX", false},
		{"the owner's provider alone", []string{"CLAUDE_CODE_USE_BEDROCK=1", "AWS_REGION=x", "CLAUDE_CONFIG_DIR=/c"}, "", false},
		// RT126ENVSCRUB1 N2: a Claude Code setting in the owner's profile that the kept list lacks is dropped from
		// desks, but Smolcorp opened from that terminal wasn't opened inside a session.
		{"an owner's setting the kept list lacks", []string{"CLAUDE_CODE_DISABLE_TERMINAL_TITLE=1"}, "", false},
		{"the Mac app's names, an owner's setting the kept list lacks", []string{"PATH=/bin"}, "CLAUDE_CODE_DISABLE_TERMINAL_TITLE", false},
		{"an effort level or an agent's name alone", []string{"CLAUDE_EFFORT=x", "AI_AGENT=x"}, "", false},
		{"nothing", []string{"PATH=/bin"}, " , ", false},
	} {
		getenv := func(name string) string {
			if name == claudeSessionVariable {
				return c.names
			}
			return ""
		}
		if got := openedInClaudeSession(getenv, c.environ); got != c.want {
			t.Errorf("%s: %v, want %v", c.row, got, c.want)
		}
	}
	if en, fr := claudeSessionLine[0], claudeSessionLine[1]; en == "" || fr == "" || en == fr || !strings.Contains(fr, "session Claude Code") {
		t.Errorf("the line's languages: %q / %q", en, fr)
	}

	d := newTestDeck(t)
	// The line, and whether the key is there at all: outside a session it isn't, not even as null, since the Mac's
	// state has no such key (the contract, MSYSTACK2 F4).
	line := func() (any, bool) {
		t.Helper()
		w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
		var state struct {
			Hire map[string]any `json:"hiring"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil || state.Hire == nil {
			t.Fatalf("setup's state: %v %s", err, w.Body.String())
		}
		got, written := state.Hire["claudeSession"]
		return got, written
	}
	openedOutsideClaudeCode(t)
	if got, written := line(); written {
		t.Errorf("outside a session, setup's state has claudeSession: %v", got)
	}
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("CLAUDE_CODE_DISABLE_TERMINAL_TITLE", "1")
	if got, written := line(); written {
		t.Errorf("with the owner's Bedrock and a setting of theirs alone, setup's state has claudeSession: %v", got)
	}
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "fake-child")
	got, written := line()
	if line, _ := got.(map[string]any); !written || line["en"] != claudeSessionLine[0] || line["fr"] != claudeSessionLine[1] {
		t.Errorf("inside a session, setup says %v (written %v)", got, written)
	}
}
