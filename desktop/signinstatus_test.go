package desktop

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ✨ Sign-in status (1.26 S1): each state from stand-in programs of the tests'
// own, in a stand-in home. No real claude, codex or gemini is ever run, and no
// credential file or Keychain item is read.

// fakeAnthropicToken is a token of the kind given (oat, ort or api), with the body given: a string
// shaped as an Anthropic sign-in token, obviously fake. Its prefix is put together only when the tests
// run, so the public check (scripts/check-public.sh) never takes these files for a real key.
func fakeAnthropicToken(kind, body string) string {
	return "sk-" + "ant-" + kind + "01-" + body
}

// A token-shaped string, obviously fake: it must never reach an answer, a log or an event.
var fakeSecret = fakeAnthropicToken("oat", "FAKEFAKEFAKEFAKEFAKEFAKEFAKE0123456789")

// signInPlace makes stand-ins for the agents named (name: what its body does), in a stand-in home, and
// makes them the only programs found: an agent not named is not installed, and no real program is ever run.
func signInPlace(t *testing.T, bodies map[string]string) string {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	for name, body := range bodies {
		fakeCLI(t, dir, name, body)
	}
	// Only this folder is looked in: programNamed also asks the login shell's PATH and the usual places
	// (a real claude may be in them), and none of that is wanted here.
	saved := signInFind
	t.Cleanup(func() { signInFind = saved })
	signInFind = func(name string) string {
		if _, ok := bodies[name]; !ok {
			return ""
		}
		real, _ := filepath.EvalSymlinks(filepath.Join(dir, name))
		return real
	}
	return dir
}

// signInRows asks as the page does, and gives the rows by agent.
func signInRows(t *testing.T) map[string]agentSignIn {
	t.Helper()
	rows := map[string]agentSignIn{}
	for _, row := range probeSignIn() {
		rows[row.Agent] = row
	}
	if len(rows) != 3 {
		t.Fatalf("rows: %v", rows)
	}
	return rows
}

func TestEachSignInStateFromStandIns(t *testing.T) {
	claudeSays := func(exit int, json string) string { return "echo '" + json + "'; exit " + strconv.Itoa(exit) }
	codexSays := func(exit int, text string) string { return "echo '" + text + "'; exit " + strconv.Itoa(exit) }
	for _, c := range []struct {
		name   string
		claude string
		codex  string
		want   map[string]agentSignIn
	}{
		{"signed in", claudeSays(0, `{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"max","email":"ann@example.com","orgName":"Ann org"}`),
			codexSays(0, "Logged in using ChatGPT"),
			map[string]agentSignIn{
				"claude": {Agent: "claude", Installed: true, State: accountSignedIn, Account: "ann@example.com"},
				"codex":  {Agent: "codex", Installed: true, State: accountSignedIn},
			}},
		{"an API account, and an API key", claudeSays(0, `{"loggedIn":true,"authMethod":"apiKey"}`),
			codexSays(0, "Logged in using an API key - sk-proj-***abcd"),
			map[string]agentSignIn{
				"claude": {Agent: "claude", Installed: true, State: accountSignedIn},
				"codex":  {Agent: "codex", Installed: true, State: accountSignedIn},
			}},
		{"signed out", claudeSays(1, `{"loggedIn":false}`), codexSays(1, "Not logged in"),
			map[string]agentSignIn{
				"claude": {Agent: "claude", Installed: true, State: accountSignedOut},
				"codex":  {Agent: "codex", Installed: true, State: accountSignedOut},
			}},
		{"can't tell: something else", claudeSays(0, `Welcome back!`), codexSays(0, "All good, probably"),
			map[string]agentSignIn{
				"claude": {Agent: "claude", Installed: true, State: accountUnknown},
				"codex":  {Agent: "codex", Installed: true, State: accountUnknown},
			}},
		{"can't tell: no loggedIn, or text after the JSON", claudeSays(0, `{"email":"ann@example.com"}`), codexSays(0, "Logged in using ChatGPT\nNot logged in"),
			map[string]agentSignIn{
				"claude": {Agent: "claude", Installed: true, State: accountUnknown},
				"codex":  {Agent: "codex", Installed: true, State: accountUnknown},
			}},
		{"can't tell: it contradicts its exit", claudeSays(0, `{"loggedIn":true} extra`), codexSays(1, "Logged in using ChatGPT"),
			map[string]agentSignIn{
				"claude": {Agent: "claude", Installed: true, State: accountUnknown},
				"codex":  {Agent: "codex", Installed: true, State: accountUnknown},
			}},
		{"can't tell: a signed out that exits well", claudeSays(0, `{"loggedIn":maybe}`), codexSays(0, "Not logged in"),
			map[string]agentSignIn{
				"claude": {Agent: "claude", Installed: true, State: accountUnknown},
				"codex":  {Agent: "codex", Installed: true, State: accountUnknown},
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Gemini has no status command that asks nothing: it's here, and can't be told, and it's never run.
			gemini := filepath.Join(t.TempDir(), "gemini-was-run")
			signInPlace(t, map[string]string{"claude": c.claude, "codex": c.codex, "gemini": "touch " + gemini})
			rows := signInRows(t)
			for agent, want := range c.want {
				if rows[agent] != want {
					t.Errorf("%s: %+v, want %+v", agent, rows[agent], want)
				}
			}
			if rows["gemini"] != (agentSignIn{Agent: "gemini", Installed: true, State: accountUnknown}) {
				t.Errorf("gemini: %+v", rows["gemini"])
			}
			if _, err := os.Stat(gemini); err == nil {
				t.Error("gemini was run")
			}
		})
	}
}

func TestAnAgentThatIsntHereIsNotInstalledAndCantBeTold(t *testing.T) {
	signInPlace(t, map[string]string{"codex": "echo 'Logged in using ChatGPT'"})
	rows := signInRows(t)
	for _, agent := range []string{"claude", "gemini"} {
		if rows[agent] != (agentSignIn{Agent: agent, State: accountUnknown}) {
			t.Errorf("%s: %+v", agent, rows[agent])
		}
	}
	if !rows["codex"].Installed || rows["codex"].State != accountSignedIn {
		t.Errorf("codex: %+v", rows["codex"])
	}
}

func TestAnAccountIsPassedOnOnlyAsAnEmail(t *testing.T) {
	for printed, want := range map[string]string{
		"ann@example.com":                "ann@example.com",
		" o'brien+x@mail.example.co.uk ": "o'brien+x@mail.example.co.uk",
		"Ann Smith":                      "",
		"ann@example.com extra":          "",
		"":                               "",
		fakeSecret:                       "",
		"sk-ant-oat01-" + "A1b2C3d4E5f6G7h8@example.com": "",
		strings.Repeat("a", 200) + "@example.com":        "",
		"ann@example.com\nsecond@example.com":            "",
	} {
		if got := accountOf(printed); got != want {
			t.Errorf("%q: %q, want %q", printed, got, want)
		}
	}
}

// A stand-in printing token-shaped strings, in every place a CLI could: none reaches the answer, a log or an event.
func TestATokenAStatusCommandPrintsNeverLeavesIt(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := signInPlace(t, map[string]string{
		"claude": "echo '{\"loggedIn\":true,\"authMethod\":\"claude.ai\",\"subscriptionType\":\"max\",\"email\":\"" + fakeSecret + "@example.com\"," +
			"\"accessToken\":\"" + fakeSecret + "\"}'; echo " + fakeSecret + " >&2",
		"codex":  "echo 'Logged in using an API key - " + fakeSecret + "'; echo " + fakeSecret + " >&2",
		"gemini": "echo " + fakeSecret,
	})
	d := newTestDeck(t)
	events := filepath.Join(supportDir, "events.jsonl")

	got := serve(t, d, call{method: "GET", target: "/setup/accounts/status?refresh=1", host: "localhost:7710", from: thisComputer})
	if got.Code != http.StatusOK {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	body := got.Body.String()
	var answer struct{ Agents []map[string]any }
	if err := json.Unmarshal(got.Body.Bytes(), &answer); err != nil || len(answer.Agents) != 3 {
		t.Fatalf("the answer: %v %s", err, body)
	}
	// Nothing but the four things: which agent, installed, its state, and the account when there is one.
	for _, row := range answer.Agents {
		for key := range row {
			if key != "agent" && key != "installed" && key != "state" && key != "account" {
				t.Errorf("an extra key %q", key)
			}
		}
		if row["account"] != nil {
			t.Errorf("an account from a token-shaped email: %v", row)
		}
	}
	// Whatever else, the three signed-in states say so with no account.
	if answer.Agents[0]["state"] != "signedIn" || answer.Agents[1]["state"] != "signedIn" || answer.Agents[2]["state"] != "unknown" {
		t.Errorf("the states: %s", body)
	}
	every := []string{body, logged.String()}
	data, _ := os.ReadFile(events)
	every = append(every, string(data))
	// Every file the deck wrote in its folders.
	for _, folder := range []string{supportDir, homeDir, dir} {
		filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && filepath.Base(path) != "claude" && filepath.Base(path) != "codex" && filepath.Base(path) != "gemini" {
				text, _ := os.ReadFile(path)
				every = append(every, string(text))
			}
			return nil
		})
	}
	for i, text := range every {
		if strings.Contains(text, "FAKEFAKE") || strings.Contains(text, "sk-ant") || strings.Contains(text, "sk-proj") {
			t.Errorf("a token-shaped string reached output %d: %.200s", i, text)
		}
	}
	// The same through the code that reads the printouts.
	if state := codexSignInOf(0, []byte("Logged in using an API key - "+fakeSecret), nil); state != accountSignedIn {
		t.Errorf("codex: %s", state)
	}
}

func TestTheStatusRouteIsThePagesAlone(t *testing.T) {
	row := routeFor("GET", "/setup/accounts/status")
	if row == nil || !reflect.DeepEqual(row.callers, []string{"page"}) {
		t.Fatalf("the row: %+v", row)
	}
	for _, who := range []caller{callerTool, callerGM, callerHire, callerNobody} {
		if routeAllows(who, "GET", "/setup/accounts/status") {
			t.Errorf("%s may read it", who.name())
		}
	}
	if !routeAllows(callerPage, "GET", "/setup/accounts/status") {
		t.Error("the page may not")
	}

	d := newTestDeck(t)
	d.signInSeen.probe = func() []agentSignIn { return []agentSignIn{{Agent: "claude"}} }
	writeRegistryFile(registryPath(), []byte(`{"version":1,"hires":[{"id":"aaa111","name":"Rev","folder":"/w","role":"reviewer","state":"working"}]}`), 0o600)
	hireKey, err := newSendKey(registryPath(), "aaa111")
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]call{
		"the setup key (a tool)": {method: "GET", target: "/setup/accounts/status", host: "localhost:7710", from: thisComputer, asTool: true},
		"the GM's token":         {method: "GET", target: "/setup/accounts/status", host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: map[string]string{gmTokenHeader: d.newGMToken()}},
		"a hire's key":           {method: "GET", target: "/setup/accounts/status", host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: map[string]string{hireKeyHeader: hireKey}},
		"nobody":                 {method: "GET", target: "/setup/accounts/status", host: "localhost:7710", from: thisComputer, withoutSetupKey: true},
		"a phone":                {method: "GET", target: "/setup/accounts/status", host: deckOnWiFi, from: aPhone},
		"the page, by POST":      {method: "POST", target: "/setup/accounts/status", host: "localhost:7710", from: thisComputer},
	} {
		if got := serve(t, d, c); got.Code == http.StatusOK || strings.Contains(got.Body.String(), `"agents"`) {
			t.Errorf("%s: %d %s", name, got.Code, got.Body)
		}
	}
	if got := serve(t, d, call{method: "GET", target: "/setup/accounts/status", host: "localhost:7710", from: thisComputer}); got.Code != http.StatusOK ||
		got.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("the page: %d %v", got.Code, got.Header())
	}
	// Not an office or phone route either.
	for _, path := range []string{"/accounts/status", "/api/accounts/status"} {
		if got := serve(t, d, call{method: "GET", target: path, host: deckOnWiFi, from: aPhone}); got.Code == http.StatusOK {
			t.Errorf("%s: %d", path, got.Code)
		}
	}
}

func TestAStatusCommandThatHangsIsEndedWithAnythingItStarted(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child")
	signInPlace(t, map[string]string{"claude": "sleep 30 & echo $! > " + pidFile + "; wait"})
	saved := signInDeadline
	t.Cleanup(func() { signInDeadline = saved })
	child := func() int {
		text, _ := os.ReadFile(pidFile)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(text)))
		return pid
	}
	// As the summary's test does: longer on a busy computer, until the stand-in has started its child.
	for _, deadline := range []time.Duration{time.Second, 4 * time.Second, 16 * time.Second} {
		if child() != 0 {
			break
		}
		signInDeadline = deadline
		started := time.Now()
		row := probeAgentSignIn("claude", signInFind("claude"), summaryEnvironment(os.Environ()))
		if row.State != accountUnknown || !row.Installed || time.Since(started) > deadline+5*time.Second {
			t.Fatalf("%+v after %v", row, time.Since(started))
		}
	}
	pid := child()
	if pid == 0 {
		t.Fatal("it never started its child")
	}
	eventually(t, "stopped what it started", func() bool { return processGone(pid) })
}

func TestAStatusCommandsOutputIsCut(t *testing.T) {
	// A long printout is cut at the cap, so even a valid answer at its end is never read.
	pad := "head -c 2000000 /dev/zero | tr '\\0' ' '; "
	dir := signInPlace(t, map[string]string{
		"claude": pad + "echo '{\"loggedIn\":true,\"email\":\"ann@example.com\"}'",
		"codex":  "echo 'Logged in using ChatGPT' >&2; head -c 100000 /dev/zero | tr '\\0' 'x' >&2",
	})
	ran := spawnCapped(filepath.Join(dir, "claude"), nil, nil, nil, dir, 10*time.Second, signInOutputMost, signInErrorMost)
	if len(ran.output) != signInOutputMost || ran.status != 0 {
		t.Errorf("%d, %d", len(ran.output), ran.status)
	}
	rows := signInRows(t)
	if rows["claude"].State != accountUnknown || rows["claude"].Account != "" {
		t.Errorf("claude: %+v", rows["claude"])
	}
	if rows["codex"].State != accountUnknown {
		t.Errorf("codex: %+v", rows["codex"])
	}
	// What is read is at the cap exactly, never above it.
	if state, _ := claudeSignInOf(bytes.Repeat([]byte(" "), signInOutputMost)); state != accountUnknown {
		t.Errorf("a full read: %s", state)
	}
}

func TestTheStatusIsKeptAShortTimeAndRefreshedOnDemand(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	asked := 0
	s := &signInStatus{
		now: func() time.Time { mu.Lock(); defer mu.Unlock(); return now },
		probe: func() []agentSignIn {
			mu.Lock()
			asked++
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			return []agentSignIn{{Agent: "claude", State: accountUnknown}}
		},
	}
	later := func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
	count := func() int { mu.Lock(); defer mu.Unlock(); return asked }

	// Several asking at once run the commands once.
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() { defer wg.Done(); s.get(false) }()
	}
	wg.Wait()
	if count() != 1 {
		t.Fatalf("asked %d times at once", count())
	}
	later(10 * time.Second)
	s.get(false)
	if count() != 1 {
		t.Errorf("not kept: %d", count())
	}
	// A refresh asked right after a look is not a new run; a while after, it is.
	later(-9 * time.Second)
	s.get(true)
	if count() != 1 {
		t.Errorf("a refresh within a moment ran again: %d", count())
	}
	later(3 * time.Second)
	s.get(true)
	if count() != 2 {
		t.Errorf("a refresh did not run: %d", count())
	}
	later(signInKeep + time.Second)
	s.get(false)
	if count() != 3 {
		t.Errorf("an old answer was kept: %d", count())
	}
	// A clock set back is never a reason to keep an answer.
	later(-time.Hour)
	s.get(false)
	if count() != 4 {
		t.Errorf("after the clock went back: %d", count())
	}
}

func TestTheStatusCommandsRunWithAResolvedPathAndACleanedEnvironment(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env")
	signInPlace(t, map[string]string{
		"claude": "env > " + out + "; echo '{\"loggedIn\":false}'; exit 1",
		"codex":  "exit 1",
	})
	t.Setenv("ANTHROPIC_API_KEY", "fake-key-for-the-test")
	t.Setenv("OPENAI_API_KEY", "fake-key-for-the-test")
	t.Setenv("SMOLCORP_SESSION", "fake-session")
	signInRows(t)
	text, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ANTHROPIC_API_KEY=", "OPENAI_API_KEY=", "SMOLCORP_SESSION="} {
		if strings.Contains(string(text), name) {
			t.Errorf("the environment still has %s", name)
		}
	}
	if strings.Contains(string(text), "fake-key-for-the-test") || !strings.Contains(string(text), summaryRunVariable+"=1") {
		t.Errorf("the environment:\n%s", text)
	}
}
