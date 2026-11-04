package desktop

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// ✨ Summary made by a coding agent: the runs' arguments and environment,
// their answers, and the run itself, against fake CLIs of the tests' own.
// No real provider is ever called.

const promptMarker = "PROMPT-MARKER-7f3a"

// fakeCLI is a program printing what body does: a bash script, or on Windows a .cmd one.
func fakeCLI(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake CLIs are bash scripts")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/bash\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEachSummaryRunIsHeadlessWithNoToolsAndThePromptNeverOnItsCommandLine(t *testing.T) {
	for _, p := range summaryAgents("darwin") {
		call := summaryInvocationOf(p, "SYSTEM", "Project: p\n"+promptMarker, "/tmp/f")
		given := string(call.input)
		for _, data := range call.files {
			given += string(data)
		}
		if !strings.Contains(given, promptMarker) {
			t.Errorf("%s: the excerpt isn't given", p)
		}
		for _, arg := range call.args {
			if strings.Contains(arg, promptMarker) {
				t.Errorf("%s: the excerpt is on the command line", p)
			}
			if slices.Contains([]string{"--yolo", "--force", "--always-approve", "--dangerously-skip-permissions", "--approve-mcps", "--auto"}, arg) {
				t.Errorf("%s may run tools: %s", p, arg)
			}
		}
	}
}

func TestTheSummaryRunsMatchTheMacApps(t *testing.T) {
	want := []string{"--print", "--model", "haiku", "--tools", "", "--strict-mcp-config", "--setting-sources", "",
		"--disable-slash-commands", "--no-session-persistence", "--permission-mode", "dontAsk",
		"--output-format", "json", "--system-prompt", "S"}
	// Windows's own form, the instructions on the input, has its test below.
	if got := summaryInvocationFor("darwin", providerClaude, "S", "P", "/f").args; !slices.Equal(got, want) {
		t.Errorf("claude: %q", got)
	}
	if got := summaryInvocationOf(providerOpenCode, "S", "P", "/f").args; !slices.Equal(got, []string{"run", "--standalone", "--agent", "summary", "--format", "json"}) {
		t.Errorf("opencode: %q", got)
	}
	if got := summaryFlags(providerOpenCode); !slices.Equal(got, []string{"--standalone", "--agent", "--format"}) {
		t.Errorf("opencode's flags: %q", got)
	}
}

func TestWindowsHasNoGrokOrCursor(t *testing.T) {
	if got := summaryAgents("windows"); slices.Contains(got, providerGrok) || slices.Contains(got, providerCursor) || got[0] != providerClaude {
		t.Errorf("windows: %v", got)
	}
	if got := summaryAgents("linux"); len(got) != 6 {
		t.Errorf("linux: %v", got)
	}
}

func TestOnlyTheSummaryRunLosesTheProvidersKeys(t *testing.T) {
	base := []string{"ANTHROPIC_API_KEY=sk-ant-test0", "openai_api_key=x", "XAI_API_KEY=x", "PATH=/usr/bin", "HOME=/home/me",
		"SMOLCORP_SESSION=abc", "SMOLCORP_SUMMARY=0"}
	env := summaryEnvironment(base)
	for _, variable := range env {
		name, _, _ := strings.Cut(variable, "=")
		if slices.Contains([]string{"ANTHROPIC_API_KEY", "openai_api_key", "XAI_API_KEY", "SMOLCORP_SESSION"}, name) {
			t.Errorf("the summary's process gets %s", name)
		}
	}
	if !slices.Contains(env, "PATH=/usr/bin") || !slices.Contains(env, "HOME=/home/me") || !slices.Contains(env, "SMOLCORP_SUMMARY=1") ||
		slices.Contains(env, "SMOLCORP_SUMMARY=0") {
		t.Errorf("env: %q", env)
	}
	if base[0] != "ANTHROPIC_API_KEY=sk-ant-test0" {
		t.Error("the environment given was changed")
	}
}

// The owner runs some terminals on an API key and others on a plan: a hire
// keeps ANTHROPIC_API_KEY, Smolcorp's own environment keeps it, and only the
// summary's process goes without.
func TestAHireKeepsANTHROPIC_API_KEYWhileTheSummaryRunHasNone(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-hire0")
	if !slices.Contains(hireEnvironment(), "ANTHROPIC_API_KEY=sk-ant-hire0") {
		t.Error("a hire's environment lost the key")
	}
	for _, variable := range summaryEnvironment(os.Environ()) {
		if strings.HasPrefix(strings.ToUpper(variable), "ANTHROPIC_API_KEY=") {
			t.Error("the summary's process has the key")
		}
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "sk-ant-hire0" {
		t.Error("Smolcorp's own environment lost the key")
	}
}

func TestTheHookIgnoresASummarysRun(t *testing.T) {
	t.Setenv(summaryRunVariable, "1")
	stdin := os.Stdin
	defer func() { os.Stdin = stdin }()
	read, write, _ := os.Pipe()
	write.WriteString(`{"session_id":"s","hook_event_name":"SessionStart"}`)
	write.Close()
	os.Stdin = read
	// No key file is read and nothing is sent: it returns at once.
	if RunHook([]string{"codex"}) != 0 {
		t.Fatal("the hook failed")
	}
}

func TestTheSummaryAnswers(t *testing.T) {
	for _, c := range []struct {
		p      summaryProvider
		ran    summaryRan
		text   string
		reason cliFailure
	}{
		{providerClaude, summaryRan{output: []byte(`{"type":"result","is_error":false,"result":"Asked: dark mode.\nWaiting on you."}`)}, "Asked: dark mode.\nWaiting on you.", cliOK},
		{providerClaude, summaryRan{status: 1, output: []byte(`{"type":"result","is_error":true,"result":"Claude AI usage limit reached|1793400000"}`)}, "", cliQuota},
		{providerClaude, summaryRan{status: 1, output: []byte(`{"type":"result","is_error":true,"result":"Not logged in · Please run /login"}`)}, "", cliSignedOut},
		// As grok 1.0.70 printed it, with no login on this computer.
		{providerGrok, summaryRan{status: 1, output: []byte(`{"type":"error","message":"Not signed in. To authenticate without a browser, run:\n  grok login --device-code"}`)}, "", cliSignedOut},
		{providerOpenCode, summaryRan{output: []byte("{\"type\":\"step_start\",\"sessionID\":\"ses_a1\"}\n{\"type\":\"text\",\"sessionID\":\"ses_a1\",\"part\":{\"type\":\"text\",\"text\":\"Asked: x.\"}}\n")}, "Asked: x.", cliOK},
		{providerCodex, summaryRan{output: []byte("progress…"), file: []byte("Asked: tests.\n")}, "Asked: tests.", cliOK},
		{providerCodex, summaryRan{status: 2, errors: []byte("error: unexpected argument '--ignore-rules' found")}, "", cliChanged},
		{providerGemini, summaryRan{output: []byte(`{"response":"Asked: y.","stats":{}}`)}, "Asked: y.", cliOK},
		{providerGemini, summaryRan{status: 1, output: []byte(`{"error":{"message":"Quota exceeded for quota metric"}}`)}, "", cliQuota},
		{providerCursor, summaryRan{output: []byte(`{"type":"result","result":"Asked: z."}`)}, "Asked: z.", cliOK},
		{providerClaude, summaryRan{output: []byte(`{"result":"  "}`)}, "", cliEmpty},
		{providerClaude, summaryRan{timedOut: true}, "", cliTimeout},
	} {
		if text, reason := summaryAnswer(c.p, c.ran); text != c.text || reason != c.reason {
			t.Errorf("%s %q: got %q, %d", c.p, c.ran.output, text, reason)
		}
	}
	if got := openCodeSession([]byte(`{"sessionID":"ses_1; rm -rf ~"}`)); got != "" {
		t.Errorf("a session id that isn't one: %q", got)
	}
}

func TestTheSummaryIsCleaned(t *testing.T) {
	if got := cleanSummary("Here's the summary:\n```\nOne\n\nTwo\n```"); got != "One\nTwo" {
		t.Errorf("got %q", got)
	}
	if got := cleanSummary("<thinking>hmm</thinking>A\nB\nC\nD\nE\nF"); got != "A\nB\nC\nD\nE" {
		t.Errorf("got %q", got)
	}
	if got := []rune(cleanSummary(strings.Repeat("é", 900))); len(got) != summaryCharactersMost {
		t.Errorf("%d characters", len(got))
	}
}

func TestASummaryRunGetsTheExcerptOnItsInputInAnEmptyFolderItsOwnDeletedAfter(t *testing.T) {
	dir := t.TempDir()
	seen := filepath.Join(dir, "seen")
	claude := fakeCLI(t, dir, "claude", `if [ "$1" = auth ]; then echo '{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"max"}'; exit; fi
{ echo "ARGS: $*"; echo "DIR: $PWD"; ls -A; echo "KEY: ${ANTHROPIC_API_KEY:-none}"; echo "INPUT:"; cat; } > `+seen+`
echo '{"type":"result","is_error":false,"result":"Asked: dark mode."}'`)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-run0")
	if text, reason := runSummaryCLI(providerClaude, claude, "SYSTEM", "Project: p\n"+promptMarker, os.Environ(), providerReady); text != "Asked: dark mode." || reason != cliOK {
		t.Fatalf("got %q, %d", text, reason)
	}
	report, _ := os.ReadFile(seen)
	lines := strings.Split(string(report), "\n")
	if strings.Contains(lines[0], promptMarker) || !strings.Contains(string(report), "INPUT:\nProject: p\n"+promptMarker) ||
		!strings.Contains(string(report), "KEY: none") {
		t.Fatalf("report: %s", report)
	}
	folder := strings.TrimPrefix(lines[1], "DIR: ")
	if !strings.Contains(folder, "smolcorp-summary-") || lines[2] != "KEY: none" {
		t.Errorf("not an empty folder of its own: %q", lines[1:3])
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Error("its folder is left behind")
	}
}

func TestASummaryRunThatHangsIsEndedWithAnythingItStarted(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child")
	slow := fakeCLI(t, dir, "claude", "sleep 30 & echo $! > "+pidFile+"; wait")
	started := time.Now()
	ran := spawnSummary(slow, nil, nil, summaryEnvironment(os.Environ()), dir, time.Second)
	if !ran.timedOut || time.Since(started) > 6*time.Second {
		t.Fatalf("timed out %v after %v", ran.timedOut, time.Since(started))
	}
	text, _ := os.ReadFile(pidFile)
	child, _ := strconv.Atoi(strings.TrimSpace(string(text)))
	time.Sleep(200 * time.Millisecond)
	if child == 0 || !processGone(child) {
		t.Error("what it started is still running")
	}
}

func TestASummaryRunsOutputIsCut(t *testing.T) {
	dir := t.TempDir()
	loud := fakeCLI(t, dir, "claude", "head -c 2000000 /dev/zero | tr '\\0' 'x'; head -c 100000 /dev/zero >&2")
	ran := spawnSummary(loud, nil, nil, nil, dir, 10*time.Second)
	if len(ran.output) != summaryOutputMost || len(ran.errors) != summaryErrorMost || ran.status != 0 {
		t.Errorf("%d, %d, %d", len(ran.output), len(ran.errors), ran.status)
	}
	deaf := fakeCLI(t, dir, "deaf", "exit 3")
	if ran := spawnSummary(deaf, nil, make([]byte, 4_000_000), nil, dir, 10*time.Second); ran.status != 3 {
		t.Errorf("status %d", ran.status)
	}
}

func TestGroksAgentIsNeverTakenForCursor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	grok := fakeCLI(t, dir, "agent", "echo 'grok 1.0.70 (f50904eaee6c)'")
	resolved, _ := filepath.EvalSymlinks(grok)
	if got := summaryProgram(providerCursor); got == resolved {
		t.Error("Grok's agent was taken for Cursor")
	}
	os.Remove(grok)
	cursor, _ := filepath.EvalSymlinks(fakeCLI(t, dir, "agent", "echo '2025.10.28-0a1b2c3'"))
	if got := summaryProgram(providerCursor); got != cursor {
		t.Errorf("got %q", got)
	}
	if _, err := exec.LookPath("cursor-agent"); err == nil {
		t.Log("a real cursor-agent is installed")
	}
}

func TestTheFirstProviderThatWorksIsPickedAndOneSavedIsNeverSwapped(t *testing.T) {
	ready, out := providerState{kind: providerReady}, providerState{kind: providerSignedOut}
	for _, c := range []struct {
		saved  string
		optIn  bool
		states map[summaryProvider]providerState
		hasKey bool
		want   summaryProvider
	}{
		{"", false, map[summaryProvider]providerState{providerOpenCode: ready, providerClaude: ready}, true, providerClaude},
		{"", false, map[summaryProvider]providerState{providerClaude: out, providerOpenCode: ready}, false, providerOpenCode},
		// Not supported: never picked, nor kept when saved.
		{"", false, map[summaryProvider]providerState{providerCodex: ready, providerGrok: ready}, false, ""},
		{"gemini", true, map[summaryProvider]providerState{providerGemini: ready}, false, ""},
		{"", false, map[summaryProvider]providerState{providerClaude: out}, true, providerAPIKey},
		{"", false, map[summaryProvider]providerState{providerClaude: out}, false, ""},
		// One who used the key before keeps it.
		{"", true, map[summaryProvider]providerState{providerClaude: ready}, true, providerAPIKey},
		{"opencode", true, map[summaryProvider]providerState{providerClaude: ready, providerOpenCode: out}, true, providerOpenCode},
		// A Console login bills an API account: never picked by itself.
		{"", false, map[summaryProvider]providerState{providerClaude: {kind: providerAPIAccount}}, false, ""},
		{"claude", true, map[summaryProvider]providerState{providerClaude: {kind: providerAPIAccount}}, false, providerClaude},
		{"apikey", true, map[summaryProvider]providerState{providerClaude: ready}, false, ""},
		{"nonsense", false, map[summaryProvider]providerState{providerClaude: ready}, false, providerClaude},
	} {
		if got := chosenSummaryProvider(c.saved, c.optIn, c.states, c.hasKey); got != c.want {
			t.Errorf("%+v: got %q", c, got)
		}
	}
}

func TestAFailedAgentFallsBackToTheKeyAndSaysSo(t *testing.T) {
	path := transcriptFile(t, userLine("Fix the elevator", "u1"))
	s := newSummaries(func() string { return testSummaryKey })
	s.savedKey = func() string { return testSummaryKey }
	s.agent = func(summaryProvider, string, string) (string, cliFailure) { return "", cliSignedOut }
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) { return "From the key.", nil }
	text, err := s.SummarizeWith("s1", path, "p", providerClaude)
	note := "(Claude Code isn't signed in, so this was made with your API key.)"
	if err != nil || text != "From the key.\n"+note || s.LastFallback() != note {
		t.Fatalf("got %q, %v, %q", text, err, s.LastFallback())
	}
}

func TestAFailedAgentWithNoKeySaysWhy(t *testing.T) {
	path := transcriptFile(t, userLine("Fix the elevator", "u1"))
	s := newSummaries(func() string { return "" })
	s.agent = func(summaryProvider, string, string) (string, cliFailure) { return "", cliQuota }
	asked := false
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) { asked = true; return "", nil }
	if _, err := s.SummarizeWith("s1", path, "p", providerCodex); asked || err == nil || err.Message != cliQuota.err(providerCodex).Message {
		t.Fatalf("asked %v, %v", asked, err)
	}
}

func TestAnAgentThatWorksSendsNothingToAnthropic(t *testing.T) {
	path := transcriptFile(t, userLine("Fix the elevator", "u1"))
	s := newSummaries(func() string { return testSummaryKey })
	given := ""
	s.agent = func(_ summaryProvider, system, prompt string) (string, cliFailure) {
		given = system + prompt
		return "From the agent.", cliOK
	}
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) {
		t.Error("Anthropic was called")
		return "", nil
	}
	if text, err := s.SummarizeWith("s1", path, "p", providerOpenCode); text != "From the agent." || err != nil {
		t.Fatalf("got %q, %v", text, err)
	}
	if !strings.Contains(given, "LATEST REQUEST") || s.LastFallback() != "" {
		t.Errorf("given %q", given)
	}
}

func TestSetupSaysWhyEachProviderCantAndWhatTheChosenCosts(t *testing.T) {
	keys := newSummaryKeys(secretStore{get: func() (string, error) { return "", errNoSecretStore }, set: func(string) error { return nil },
		delete: func() error { return nil }}, func(string) string { return "" })
	state := summaryStateOf(false, "", keys, map[summaryProvider]providerState{
		providerClaude: {kind: providerReady, reason: "uses your Claude plan"}, providerCodex: {kind: providerMissing, reason: "not installed"},
	}, false, "")
	providers := state["providers"].([]map[string]any)
	if last := providers[len(providers)-1]; last["id"] != providerAPIKey || last["state"] != "missing" {
		t.Errorf("the key: %v", last)
	}
	if providers[1]["reason"] != "not installed" || providers[2]["state"] != "checking" {
		t.Errorf("providers: %v", providers)
	}
	if p := state["provider"].(map[string]any); p["id"] != providerClaude || state["enabled"] != false {
		t.Errorf("state: %v", state)
	}
}

func TestOnlyAKeyPastedInSetupIsAFallbackNeverTheEnvironments(t *testing.T) {
	path := transcriptFile(t, userLine("Fix the elevator", "u1"))
	// ANTHROPIC_API_KEY in Smolcorp's environment (key), nothing pasted (savedKey).
	s := newSummaries(func() string { return testSummaryKey })
	s.agent = func(summaryProvider, string, string) (string, cliFailure) { return "", cliSignedOut }
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) {
		t.Error("the environment's key was used")
		return "", nil
	}
	if _, err := s.SummarizeWith("s1", path, "p", providerClaude); err == nil || err.Message != cliSignedOut.err(providerClaude).Message {
		t.Fatalf("got %v", err)
	}
	keys := newSummaryKeys(memoryStore(false), func(string) string { return testSummaryKey })
	if keys.Saved() != "" || !keys.Present() {
		t.Error("the environment's key counts as saved")
	}
}

func TestTheFallbackKeepsWithinTheLimit(t *testing.T) {
	path := transcriptFile(t, userLine("Fix the elevator", "u1"))
	s := newSummaries(func() string { return testSummaryKey })
	s.savedKey = func() string { return testSummaryKey }
	s.agent = func(summaryProvider, string, string) (string, cliFailure) { return "", cliQuota }
	asked := 0
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) { asked++; return "From the key.", nil }
	for i := 0; i < summaryMost; i++ {
		s.SummarizeWith("s"+strconv.Itoa(i), path, "p", providerClaude)
	}
	if asked != summaryMost/2 {
		t.Errorf("the key was asked %d times for %d taps", asked, summaryMost)
	}
}

func TestOnlyClaudeCodeAndOpenCodeAreRun(t *testing.T) {
	p := newSummaryProviders()
	for _, provider := range []summaryProvider{providerCodex, providerGemini, providerCursor, providerGrok} {
		if state := p.checkProvider(provider); state.kind != providerUnsupported || state.reason != unsupportedReason {
			t.Errorf("%s: %+v", provider, state)
		}
		if summaryProviderWorks(provider, map[summaryProvider]providerState{provider: {kind: providerReady}}, false) {
			t.Errorf("%s works", provider)
		}
		if _, reason := runSummaryCLI(provider, "/usr/bin/true", "", "", nil, providerReady); reason != cliUnsupported {
			t.Errorf("%s ran", provider)
		}
	}
}

func TestOpenCodesSummaryAgentMustDenyEveryTool(t *testing.T) {
	agents := func(rules string) []byte {
		return []byte(`[{"id":"build","permissions":[{"action":"*","resource":"*","effect":"allow"}]},{"id":"summary","permissions":[` + rules + `]}]`)
	}
	rule := func(action, resource, effect string) string {
		return `{"action":"` + action + `","resource":"` + resource + `","effect":"` + effect + `"}`
	}
	// As OpenCode 2.0.32's debug agents lists it.
	if !openCodeSummaryAgentIsLocked(agents(strings.Join([]string{rule("*", "*", "allow"), rule("read", "*.env", "ask"), rule("*", "*", "deny"), rule("browser", "*", "deny")}, ","))) {
		t.Error("2.0.32's summary agent isn't taken")
	}
	for _, listed := range [][]byte{
		agents(rule("*", "*", "deny") + "," + rule("shell", "*", "allow")),
		agents(rule("*", "*", "allow")),
		[]byte(`[{"id":"build","permissions":[]}]`),
		[]byte("not json"),
	} {
		if openCodeSummaryAgentIsLocked(listed) {
			t.Errorf("taken: %s", listed)
		}
	}
	tool := summaryRan{output: []byte("{\"type\":\"tool_use\",\"part\":{\"type\":\"tool\",\"tool\":\"bash\"}}\n{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\"Asked: x.\"}}")}
	if _, reason := summaryAnswer(providerOpenCode, tool); reason != cliUsedTool {
		t.Errorf("a run that used a tool gave %d", reason)
	}
}

func TestAConsoleLoginSaysItBillsTheAPIAccount(t *testing.T) {
	for status, want := range map[string]providerKind{
		`{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"max"}`: providerReady,
		`{"loggedIn":true,"authMethod":"console","subscriptionType":null}`:   providerAPIAccount,
		`{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":""}`:   providerAPIAccount,
		`{"loggedIn":false}`: "",
	} {
		if got := claudeLogin([]byte(status)); got != want {
			t.Errorf("%s: %q", status, got)
		}
	}
}

func TestOtherCloudsSettingsDontReachTheRun(t *testing.T) {
	env := summaryEnvironment([]string{"CLAUDE_CODE_USE_BEDROCK=1", "AWS_BEARER_TOKEN_BEDROCK=x", "ANTHROPIC_DEFAULT_SONNET_MODEL=x",
		"ANTHROPIC_DEFAULT_NEW_MODEL=x", "ANTHROPIC_CUSTOM_HEADERS=x", "GOOGLE_APPLICATION_CREDENTIALS=x", "GOOGLE_GENAI_USE_VERTEXAI=1",
		"GOOGLE_CLOUD_PROJECT=p", "AZURE_OPENAI_API_KEY=x", "CLAUDE_CODE_USE_VERTEX=1", "CLAUDE_CODE_USE_FOUNDRY=1", "LANG=fr_FR.UTF-8"})
	if !slices.Equal(env, []string{"LANG=fr_FR.UTF-8", "SMOLCORP_SUMMARY=1"}) {
		t.Errorf("env: %q", env)
	}
}

func TestItsAnswerIsRedactedLikeTheExcerpt(t *testing.T) {
	ran := summaryRan{output: []byte(`{"result":"Asked: deploy.\nRan: export API_KEY=sk-live-123\nNot waiting on you."}`)}
	if text, _ := summaryAnswer(providerClaude, ran); text != "Asked: deploy.\n[redacted]\nNot waiting on you." {
		t.Errorf("got %q", text)
	}
}

func TestTheHelpMustListEachFlagExactly(t *testing.T) {
	for help, want := range map[string]bool{
		"  --tools <tools...>  Specify": true, "--format choice": true, "  -p, --print   Print": true,
		"  --allowed-tools <t>": false, "  --output-format <f>": false, "  --toolset <t>": false,
	} {
		flag := "--tools"
		if strings.Contains(help, "format") {
			flag = "--format"
		} else if strings.Contains(help, "print") {
			flag = "--print"
		}
		if helpLists(help, flag) != want {
			t.Errorf("%q lists %s: want %v", help, flag, want)
		}
	}
}

func TestOnWindowsTheInstructionsGoOnInputNotTheCommandLine(t *testing.T) {
	call := summaryInvocationFor("windows", providerClaude, "LINE ONE\nLINE TWO", "Project: p", `C:\f`)
	for _, arg := range call.args {
		if strings.Contains(arg, "LINE") || arg == "--system-prompt" {
			t.Errorf("on the command line: %q", arg)
		}
	}
	if !strings.HasPrefix(string(call.input), "LINE ONE\nLINE TWO\n\nProject: p") {
		t.Errorf("input: %q", call.input)
	}
	if mac := summaryInvocationFor("darwin", providerClaude, "S", "P", "/f"); mac.args[len(mac.args)-2] != "--system-prompt" {
		t.Errorf("elsewhere: %q", mac.args)
	}
}

func TestAProcessGroupIsEndedOnlyThroughItself(t *testing.T) {
	if (processGroup{}).kill() != nil {
		t.Error("a group never joined was killed by pid 0")
	}
}

func TestEachRunChecksAgainThatOpenCodesSummaryAgentDeniesEveryTool(t *testing.T) {
	dir := t.TempDir()
	ran := filepath.Join(dir, "ran")
	// Its summary agent given a tool since setup looked.
	opencode := fakeCLI(t, dir, "opencode", `if [ "$1" = debug ]; then echo '[{"id":"summary","permissions":[{"action":"*","resource":"*","effect":"deny"},{"action":"shell","resource":"*","effect":"allow"}]}]'; exit; fi
touch `+ran)
	if _, reason := runSummaryCLI(providerOpenCode, opencode, "S", "P", nil, providerReady); reason != cliUnsupported {
		t.Errorf("got %d", reason)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("it ran")
	}
}

func TestEachRunChecksAgainHowClaudeCodesLoginPays(t *testing.T) {
	dir := t.TempDir()
	console := fakeCLI(t, dir, "claude", `if [ "$1" = auth ]; then echo '{"loggedIn":true,"authMethod":"console","subscriptionType":null}'; exit; fi
echo '{"result":"Asked: x."}'`)
	// Setup showed a plan: a Console login now is refused, not billed.
	if _, reason := runSummaryCLI(providerClaude, console, "S", "P", nil, providerReady); reason != cliLoginChanged {
		t.Errorf("plan expected: %d", reason)
	}
	// Setup showed the Console account: it's used, as said.
	if text, reason := runSummaryCLI(providerClaude, console, "S", "P", nil, providerAPIAccount); text != "Asked: x." || reason != cliOK {
		t.Errorf("console expected: %q %d", text, reason)
	}
	out := fakeCLI(t, dir, "out", `if [ "$1" = auth ]; then echo '{"loggedIn":false}'; exit; fi
echo '{"result":"Asked: x."}'`)
	if _, reason := runSummaryCLI(providerClaude, out, "S", "P", nil, providerReady); reason != cliSignedOut {
		t.Errorf("signed out: %d", reason)
	}
}

func TestOpenCodeOutputCutShortOrUnreadableIsThrownAway(t *testing.T) {
	cut := summaryRan{output: []byte(strings.Repeat("x", summaryOutputMost))}
	broken := summaryRan{output: []byte("{\"type\":\"tool_use\",\"part\":{\"type\":\"to\n{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\"Asked: x.\"}}")}
	for _, ran := range []summaryRan{cut, broken} {
		if _, reason := summaryAnswer(providerOpenCode, ran); reason != cliFailed {
			t.Errorf("kept: %d", reason)
		}
	}
}

func TestASecretIsRedactedBeforeTheSummaryIsCut(t *testing.T) {
	result, _ := json.Marshal(map[string]string{"result": strings.Repeat("a", 590) + "\nthe password is hunter2"})
	if text, _ := summaryAnswer(providerClaude, summaryRan{output: result}); strings.Contains(text, "passw") || strings.Contains(text, "hunter") {
		t.Errorf("got %q", text)
	}
}

func TestAnUnsupportedPickSaysSo(t *testing.T) {
	keys := newSummaryKeys(memoryStore(true), func(string) string { return "" })
	state := summaryStateOf(true, "gemini", keys, map[summaryProvider]providerState{providerGemini: {kind: providerUnsupported, reason: unsupportedReason}}, false, "")
	if state["saved"] != "gemini" || state["provider"] != nil || state["enabled"] != false {
		t.Errorf("state: %v", state)
	}
}

func TestTheKeyRowSaysToSetTheEnvironmentWhenThereIsNoSecretStore(t *testing.T) {
	keyReasonWith := func(store secretStore) string {
		keys := newSummaryKeys(store, func(string) string { return "" })
		providers := summaryStateOf(false, "", keys, nil, false, "")["providers"].([]map[string]any)
		return providers[len(providers)-1]["reason"].(string)
	}
	if reason := keyReasonWith(memoryStore(true)); !strings.Contains(reason, summaryKeyEnv) || strings.Contains(reason, "add a key below") {
		t.Errorf("no store: %q", reason)
	}
	if reason := keyReasonWith(memoryStore(false)); reason != "add a key below" {
		t.Errorf("with a store: %q", reason)
	}
}
