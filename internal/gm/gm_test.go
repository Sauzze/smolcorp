package gm

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const fakeToken = "fake-gm-token-0123456789-abcdefghijklmnop"

var fixtures = filepath.Join("..", "..", "Tests", "fixtures", "frontdesk")

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// teamAnswer is the "answer" half of team.json, which is what the route serves.
func teamAnswer(t *testing.T) []byte {
	t.Helper()
	var f struct {
		Answer json.RawMessage `json:"answer"`
	}
	if err := json.Unmarshal(fixture(t, "team.json"), &f); err != nil {
		t.Fatal(err)
	}
	return f.Answer
}

type seen struct {
	Method, Path, RawQuery string
	Header                 http.Header
}

type fakeApp struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []seen
}

func newFakeApp(t *testing.T, h http.HandlerFunc) *fakeApp {
	t.Helper()
	f := &fakeApp{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.reqs = append(f.reqs, seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone()})
		f.mu.Unlock()
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeApp) env(t *testing.T, vars map[string]string) Env {
	t.Helper()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(f.srv.URL, "http://"))
	m := map[string]string{portEnv: port}
	for k, v := range vars {
		m[k] = v
	}
	dir := t.TempDir()
	return Env{Getenv: func(k string) string { return m[k] }, SupportDir: func() (string, error) { return dir, nil }}
}

func serve(body []byte, status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(body)
	}
}

func runGM(env Env, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, env, &out, &errOut)
	return code, out.String(), errOut.String()
}

func withToken() map[string]string { return map[string]string{tokenEnv: fakeToken} }

func TestTeamTableFromFixture(t *testing.T) {
	app := newFakeApp(t, serve(teamAnswer(t), 200))
	code, out, errOut := runGM(app.env(t, withToken()), "team")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"NAME", "Rev", "reviewer", "work", "thinking", "10s", "0/1", "DONE: reviewed #12", "Janitor", "Bob", "asking", "2/4"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	r := app.reqs[0]
	if r.Method != "GET" || r.Path != "/setup/gm/team" {
		t.Errorf("asked %s %s", r.Method, r.Path)
	}
	if got := r.Header.Get("x-smolcorp-gm-token"); got != fakeToken {
		t.Errorf("token header = %q", got)
	}
	if r.Header.Get("X-Smolcorp-Setup-Key") != "" {
		t.Error("the setup key must not be sent")
	}
}

func TestJSONIsAsServed(t *testing.T) {
	raw := teamAnswer(t)
	app := newFakeApp(t, serve(raw, 200))
	code, out, _ := runGM(app.env(t, withToken()), "team", "--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var a, b any
	if json.Unmarshal(raw, &a) != nil || json.Unmarshal([]byte(out), &b) != nil {
		t.Fatal("not JSON")
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	if string(x) != string(y) {
		t.Errorf("--json changed the answer")
	}
}

func TestInboxFromFixture(t *testing.T) {
	app := newFakeApp(t, serve(fixture(t, "events.json"), 200))
	code, out, errOut := runGM(app.env(t, withToken()), "inbox", "--since", "40", "--wait", "3")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "#41") || !strings.Contains(out, "sessionStart") || !strings.Contains(out, "permissionDenied shop Bash: rm") {
		t.Errorf("events not shown:\n%s", out)
	}
	if !strings.HasSuffix(out, "next=42\n") {
		t.Errorf("last line should be next=42:\n%s", out)
	}
	if q := app.reqs[0].RawQuery; q != "since=40&wait=3" {
		t.Errorf("query = %q", q)
	}
}

func TestInboxEmpty(t *testing.T) {
	app := newFakeApp(t, serve(fixture(t, "events-empty.json"), 200))
	code, out, _ := runGM(app.env(t, withToken()), "inbox")
	if code != 0 || out != "next=42\n" {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestVersionAndAccounts(t *testing.T) {
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup/gm/version":
			serve([]byte(`{"app":"Smolcorp","version":"1.24.0","api":1,"features":["team","events"]}`), 200)(w, r)
		default:
			serve([]byte(`{"error":"nothing here"}`), 404)(w, r)
		}
	})
	env := app.env(t, withToken())
	code, out, _ := runGM(env, "version")
	if code != 0 || !strings.Contains(out, "Smolcorp 1.24.0") || !strings.Contains(out, "features: team, events") {
		t.Errorf("version: %d %q", code, out)
	}
	code, _, errOut := runGM(env, "accounts")
	if code != 1 || !strings.Contains(errOut, "no accounts route yet") {
		t.Errorf("accounts without a route: %d %q", code, errOut)
	}
}

func TestAccountsTable(t *testing.T) {
	// Both shapes: the brief's {"accounts"} and the fixture's {"profiles"}.
	for _, body := range []string{
		`{"accounts":[{"name":"default","dir":"/h/.claude","connected":true,"default":true},{"name":"work","dir":"/h/.claude-work","connected":false,"default":false}]}`,
		`{"profiles":[{"name":"default","dir":"/h/.claude","connected":true,"default":true},{"name":"work","dir":"/h/.claude-work","connected":false,"default":false}],"most":8}`,
	} {
		app := newFakeApp(t, serve([]byte(body), 200))
		code, out, _ := runGM(app.env(t, withToken()), "accounts")
		if code != 0 || !strings.Contains(out, "/h/.claude-work") || !strings.Contains(out, "work") || !strings.Contains(out, "CONNECTED") {
			t.Errorf("accounts: %d %q", code, out)
		}
		if r := app.reqs[0]; r.Method != "GET" || r.Path != "/setup/claude-accounts" {
			t.Errorf("request = %+v", r)
		}
	}
}

func TestRotateTokenIsNotACommandAndTheHelpSaysWhoRotates(t *testing.T) {
	app := newFakeApp(t, serve(teamAnswer(t), 200))
	code, _, errOut := runGM(app.env(t, withToken()), "rotate-token")
	if code != exitUsage || !strings.Contains(errOut, "unknown command") || len(app.reqs) != 0 {
		t.Errorf("rotate-token: %d %q, %d requests", code, errOut, len(app.reqs))
	}
	_, out, _ := runGM(app.env(t, withToken()), "help")
	if !strings.Contains(out, "ask the owner to rotate it in Smolcorp's setup") {
		t.Errorf("help = %q", out)
	}
}

func TestTilePostsAndSaysWhatHappened(t *testing.T) {
	body := []byte(`{"method":"windows","placed":["a","b"],"missed":["c"],"needsPermission":false,"message":"2 placed; 1 not."}`)
	app := newFakeApp(t, serve(body, 200))
	env := app.env(t, withToken())
	code, out, _ := runGM(env, "tile")
	if code != 0 || !strings.Contains(out, "2 placed; 1 not. (2 placed, 1 not)") {
		t.Errorf("plain: %d %q", code, out)
	}
	code, out, _ = runGM(env, "tile", "--json")
	if code != 0 || !strings.Contains(out, `"method": "windows"`) {
		t.Errorf("json: %d %q", code, out)
	}
	r := app.reqs[0]
	if r.Method != "POST" || r.Path != "/setup/gm/tile" || r.Header.Get("X-AgentDeck-Setup") != "1" {
		t.Errorf("request = %+v", r)
	}
}

func TestTileOnAnOldAppSaysSo(t *testing.T) {
	app := newFakeApp(t, serve(nil, 404))
	code, _, errOut := runGM(app.env(t, withToken()), "tile")
	if code != 1 || !strings.Contains(errOut, "can't tile windows yet") {
		t.Errorf("%d %q", code, errOut)
	}
}

func TestScreensListsAndMarksTheTeamsScreen(t *testing.T) {
	body := []byte(`{"screens":[{"id":"1","name":"Built-in","main":true,"portrait":false,"team":false},{"id":"2","name":"PA278CV","main":false,"portrait":true,"team":true}],"chosen":"2","ask":true}`)
	app := newFakeApp(t, serve(body, 200))
	env := app.env(t, withToken())
	code, out, _ := runGM(env, "screens")
	if code != 0 || !strings.Contains(out, "* 2  PA278CV portrait") || !strings.Contains(out, "  1  Built-in main") {
		t.Errorf("plain: %d %q", code, out)
	}
	if r := app.reqs[0]; r.Method != "GET" || r.Path != "/setup/gm/screens" {
		t.Errorf("request = %+v", r)
	}
}

func TestScreenChoosesOneAndNeedsExactlyOneId(t *testing.T) {
	body := []byte(`{"screens":[{"id":"1","name":"A","main":true,"portrait":false,"team":true}],"chosen":"1","ask":false}`)
	app := newFakeApp(t, serve(body, 200))
	env := app.env(t, withToken())
	if code, _, e := runGM(env, "screen", "1"); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	r := app.reqs[0]
	if r.Method != "POST" || r.Path != "/setup/gm/screen" || r.RawQuery != "id=1" || r.Header.Get("X-AgentDeck-Setup") != "1" {
		t.Errorf("request = %+v", r)
	}
	if code, _, e := runGM(env, "screen"); code != 2 || !strings.Contains(e, "one screen id") {
		t.Errorf("no id: %d %q", code, e)
	}
	if code, _, _ := runGM(env, "screens", "x"); code != 2 {
		t.Errorf("screens takes no argument: %d", code)
	}
}

func TestScreenOnAnOldAppSaysSo(t *testing.T) {
	app := newFakeApp(t, serve(nil, 404))
	code, _, errOut := runGM(app.env(t, withToken()), "screens")
	if code != 1 || !strings.Contains(errOut, "can't choose a screen yet") {
		t.Errorf("%d %q", code, errOut)
	}
}

func TestTokenFileWhenNoEnv(t *testing.T) {
	app := newFakeApp(t, serve(teamAnswer(t), 200))
	env := app.env(t, nil)
	dir, _ := env.SupportDir()
	os.WriteFile(filepath.Join(dir, "gm-token"), []byte(fakeToken+"\n"), 0o600)
	if code, _, e := runGM(env, "team"); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	if got := app.reqs[0].Header.Get("x-smolcorp-gm-token"); got != fakeToken {
		t.Errorf("token = %q", got)
	}
}

// A hired GM's environment holds its token and no hire key, so its GM commands run.
func TestGMTokenAloneRunsGMCommands(t *testing.T) {
	app := newFakeApp(t, serve(teamAnswer(t), 200))
	if code, _, e := runGM(app.env(t, withToken()), "team"); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	if got := app.reqs[0].Header.Get("x-smolcorp-gm-token"); got != fakeToken {
		t.Errorf("token = %q", got)
	}
	// A hire key still wins over a token: that environment can't act as the GM.
	env := app.env(t, map[string]string{tokenEnv: fakeToken, hireKeyEnv: "fake-hire-key"})
	n := len(app.reqs)
	if code, _, e := runGM(env, "team"); code != 1 || !strings.Contains(e, "for the GM") || len(app.reqs) != n {
		t.Errorf("code %d %q reqs %d", code, e, len(app.reqs)-n)
	}
}

func TestNoTokenAnywhere(t *testing.T) {
	app := newFakeApp(t, serve(nil, 200))
	code, _, errOut := runGM(app.env(t, nil), "team")
	if code != 1 || !strings.Contains(errOut, "no GM token") {
		t.Errorf("code %d %q", code, errOut)
	}
	// At most one look, with no credentials, at which app is answering.
	if len(app.reqs) > 1 || (len(app.reqs) == 1 && (app.reqs[0].Path != "/setup/state" || app.reqs[0].Header.Get(tokenHeader) != "")) {
		t.Errorf("reqs %+v", app.reqs)
	}
}

func TestRefusedPrintsServerText(t *testing.T) {
	app := newFakeApp(t, serve([]byte(`{"error":"for the GM only"}`), 403))
	code, _, errOut := runGM(app.env(t, withToken()), "team")
	if code != 1 || !strings.Contains(errOut, "403") || !strings.Contains(errOut, "for the GM only") {
		t.Errorf("code %d %q", code, errOut)
	}
}

func TestTokenNeverInMessages(t *testing.T) {
	app := newFakeApp(t, serve([]byte("bad "+fakeToken), 500))
	_, _, errOut := runGM(app.env(t, withToken()), "team")
	if strings.Contains(errOut, fakeToken) {
		t.Errorf("token leaked: %q", errOut)
	}
}

func TestAppNotRunning(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	env := Env{
		Getenv:     func(k string) string { return map[string]string{portEnv: strconv.Itoa(port), tokenEnv: fakeToken}[k] },
		SupportDir: func() (string, error) { return t.TempDir(), nil },
	}
	code, _, errOut := runGM(env, "team")
	if code != 3 || !strings.Contains(errOut, "isn't running") {
		t.Errorf("code %d %q", code, errOut)
	}
}

func TestUsageErrors(t *testing.T) {
	env := Env{Getenv: func(string) string { return "" }, SupportDir: func() (string, error) { return t.TempDir(), nil }}
	for _, args := range [][]string{{}, {"nope"}, {"team", "extra"}, {"inbox", "--wait", "31"}, {"inbox", "--since", "-1"}, {"team", "--bogus"}} {
		if code, _, _ := runGM(env, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	if code, out, _ := runGM(env, "help"); code != 0 || !strings.Contains(out, "Exit codes") {
		t.Errorf("help: %d", code)
	}
}

func TestBadPort(t *testing.T) {
	for _, p := range []string{"abc", "80", "1023", "0", "65536"} {
		env := Env{Getenv: func(k string) string { return map[string]string{portEnv: p, tokenEnv: fakeToken}[k] },
			SupportDir: func() (string, error) { return t.TempDir(), nil }}
		if code, _, _ := runGM(env, "team"); code != 2 {
			t.Errorf("port %q: exit %d", p, code)
		}
	}
}

func TestRoutesFixtureListsWhatTheCLICalls(t *testing.T) {
	var table struct {
		Routes []struct {
			Method, Path string
			Callers      []string
		} `json:"routes"`
	}
	if err := json.Unmarshal(fixture(t, "routes.json"), &table); err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct{ method, path string }{
		{"GET", "/setup/gm/team"}, {"GET", "/setup/gm/events"}, {"GET", "/setup/gm/version"}, {"GET", "/setup/gm/approvals"},
	} {
		found := false
		for _, r := range table.Routes {
			if r.Method == call.method && r.Path == call.path {
				for _, c := range r.Callers {
					found = found || c == "gm"
				}
			}
		}
		if !found {
			t.Errorf("%s %s is not a GM route in routes.json", call.method, call.path)
		}
	}
}

// approvalsAnswer is the "answer" half of approvals.json, which is what the route serves.
func approvalsAnswer(t *testing.T) []byte {
	t.Helper()
	var f struct {
		Answer json.RawMessage `json:"answer"`
	}
	if err := json.Unmarshal(fixture(t, "approvals.json"), &f); err != nil {
		t.Fatal(err)
	}
	return f.Answer
}

func TestApprovalsTableFromFixture(t *testing.T) {
	app := newFakeApp(t, serve(approvalsAnswer(t), 200))
	code, out, errOut := runGM(app.env(t, withToken()), "approvals")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"NAME", "Rev", "0/1", "1m", "told", "Bash: git", "api", "1/3", "AskUserQuestion", "Edit: Deck.swift"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	if r := app.reqs[0]; r.Method != "GET" || r.Path != "/setup/gm/approvals" {
		t.Errorf("asked %s %s", r.Method, r.Path)
	}
}

func TestApprovalsJSONIsAsServedAndEmptyListSaysSo(t *testing.T) {
	app := newFakeApp(t, serve(approvalsAnswer(t), 200))
	code, out, _ := runGM(app.env(t, withToken()), "approvals", "--json")
	if code != 0 || !strings.Contains(out, `"label": "Bash: git"`) || !strings.Contains(out, `"waitingSeconds": 60`) {
		t.Errorf("exit %d, out:\n%s", code, out)
	}
	empty := newFakeApp(t, serve([]byte(`{"at":1,"approvals":[]}`), 200))
	if code, out, _ := runGM(empty.env(t, withToken()), "approvals"); code != 0 || !strings.Contains(out, "Nobody is waiting for you.") {
		t.Errorf("exit %d, out: %s", code, out)
	}
}

// No command of this CLI answers a prompt: approvals only reads, and nothing in the package
// names the phone's answer routes.
func TestNoCommandAnswersAPrompt(t *testing.T) {
	app := newFakeApp(t, serve(approvalsAnswer(t), 200))
	for _, cmd := range []string{"team", "approvals", "inbox", "version", "accounts"} {
		runGM(app.env(t, withToken()), cmd)
	}
	for _, r := range app.reqs {
		if r.Method != "GET" {
			t.Errorf("%s %s: a read command that doesn't read", r.Method, r.Path)
		}
		for _, answer := range []string{"/act", "/choose", "/dictate"} {
			if r.Path == answer {
				t.Errorf("a command called %s", answer)
			}
		}
	}
	// Nor is there an "approve", "answer" or "choose" command to find.
	for _, cmd := range []string{"approve", "answer", "choose", "act", "allow", "deny"} {
		if code, _, _ := runGM(app.env(t, withToken()), cmd); code != 2 {
			t.Errorf("%q: exit %d, want 2 (unknown command)", cmd, code)
		}
	}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatal("no source found", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(f)
		for _, answer := range []string{`"/act`, `"/choose`, `"/dictate`} {
			if strings.Contains(string(src), answer) {
				t.Errorf("%s names the answer route %s", f, answer)
			}
		}
	}
}

func TestHireSendsTheFixtureRequest(t *testing.T) {
	var got []byte
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		var a struct {
			Body json.RawMessage `json:"body"`
		}
		json.Unmarshal(fixtureField(t, "hire.json", "answer"), &a)
		w.Write(a.Body)
	})
	taskFile := filepath.Join(t.TempDir(), "task.md")
	os.WriteFile(taskFile, []byte("Review the importer change and write the findings in STATUS.md.\n"), 0o600)
	code, out, errOut := runGM(app.env(t, withToken()), "hire", "--role", "reviewer", "--folder", "/Users/someone/Projects/app",
		"--account", "work", "--model", "opus", "--nickname", "Ada", "--floor", "2", "--worktree", "--task-file", taskFile)
	if code != 0 || !strings.Contains(out, "Hired a reviewer. At desk 4.") {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	r := app.reqs[0]
	if r.Method != "POST" || r.Path != "/setup/gm/hire" || r.Header.Get("X-AgentDeck-Setup") != "1" || r.Header.Get("x-smolcorp-gm-token") != fakeToken {
		t.Errorf("request = %+v", r)
	}
	var want, have map[string]any
	json.Unmarshal(fixtureField(t, "hire.json", "request"), &want)
	delete(want, "apiKey") // false is left out
	if err := json.Unmarshal(got, &have); err != nil || !reflect.DeepEqual(want, have) {
		t.Errorf("body = %s\nwant %v", got, want)
	}
}

func TestHireLeavesOutWhatIsNotGiven(t *testing.T) {
	var got []byte
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"ok":true,"desk":null}`))
	})
	code, out, _ := runGM(app.env(t, withToken()), "hire", "--role", "janitor", "--api-key", "--task", "tidy up", "--json")
	if code != 0 || !strings.Contains(out, `"ok": true`) || string(got) != `{"apiKey":true,"role":"janitor","task":"tidy up"}` {
		t.Errorf("exit %d out %q body %s", code, out, got)
	}
}

func TestHireRefusalIsPrinted(t *testing.T) {
	app := newFakeApp(t, serve([]byte("Choose a folder for new hires in setup, on your Mac."), 409))
	code, _, errOut := runGM(app.env(t, withToken()), "hire", "--role", "janitor")
	if code != 1 || !strings.Contains(errOut, "409") || !strings.Contains(errOut, "Choose a folder for new hires") {
		t.Errorf("code %d %q", code, errOut)
	}
}

func TestDeskUsageErrors(t *testing.T) {
	app := newFakeApp(t, serve(nil, 204))
	env := app.env(t, withToken())
	for _, args := range [][]string{
		{"hire"}, {"hire", "--role", "r", "--floor", "6"}, {"hire", "--role", "r", "--task", "a", "--task-file", "b"},
		{"hire", "--role", "r", "--task-file", "/nonexistent/task"}, {"hire", "--role", "r", "extra"},
		{"hire", "--role", "r", "--task", strings.Repeat("x", 8001)},
		{"fire"}, {"fire", "--force"}, {"brief", "ada"}, {"brief", "ada", "--text", "a", "--file", "b"}, {"brief", "--text", "x"},
		{"brief", "ada", "--text", "  "},
	} {
		if code, _, _ := runGM(env, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	if len(app.reqs) != 0 {
		t.Errorf("a bad command reached the app: %+v", app.reqs)
	}
}

func fireApp(t *testing.T, status int, body string) *fakeApp {
	team := teamAnswer(t)
	return newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write(team)
			return
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	})
}

func TestFireByIDNicknameAndFlags(t *testing.T) {
	var team TeamReply
	if err := json.Unmarshal(teamAnswer(t), &team); err != nil || len(team.Team) == 0 {
		t.Fatal(err)
	}
	m := team.Team[0]
	for _, ref := range []string{m.ID, strings.ToUpper(m.Nickname)} {
		app := fireApp(t, 204, "")
		code, out, errOut := runGM(app.env(t, withToken()), "fire", ref, "--force", "--remove-worktree") // flags may follow the name
		if code != 0 || !strings.Contains(out, "Fired") {
			t.Fatalf("%q: exit %d %q %q", ref, code, out, errOut)
		}
		r := app.reqs[len(app.reqs)-1]
		if r.Method != "POST" || r.Path != "/setup/gm/fire" || r.Header.Get("X-AgentDeck-Setup") != "1" {
			t.Errorf("request = %+v", r)
		}
	}
}

// The app keeps a worktree it was asked to remove when it holds changes; `gm fire` says so, and why.
func TestFireSaysWhenTheWorktreeWasKept(t *testing.T) {
	var team TeamReply
	json.Unmarshal(teamAnswer(t), &team)
	kept := `{"ok":true,"kept":"It holds changes nobody committed (new files count too), or git wouldn't remove it.","worktree":"/w/ada","branch":"ada/work"}`
	app := fireApp(t, 200, kept)
	code, out, errOut := runGM(app.env(t, withToken()), "fire", team.Team[0].ID, "--force", "--remove-worktree")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d %q", code, errOut)
	}
	if !strings.Contains(out, "worktree (/w/ada) and branch were kept, not removed: it holds changes nobody committed (new files count too), or git wouldn't remove it.") {
		t.Errorf("the reason should follow the colon in lower case: %q", out)
	}
	for _, want := range []string{"Fired ", "worktree (/w/ada) and branch were kept", "new files count too"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q lacks %q", out, want)
		}
	}
	// With --json the app's answer is passed on whole.
	app = fireApp(t, 200, kept)
	if _, out, _ := runGM(app.env(t, withToken()), "fire", team.Team[0].ID, "--remove-worktree", "--json"); !strings.Contains(out, `"kept"`) {
		t.Errorf("--json: %q", out)
	}
	// A plain 204 still says only that it fired.
	app = fireApp(t, 204, "")
	if _, out, _ := runGM(app.env(t, withToken()), "fire", team.Team[0].ID); strings.Contains(out, "kept") {
		t.Errorf("a removed worktree should not say kept: %q", out)
	}
}

func TestAfterColonLowersOnlyAPlainCapital(t *testing.T) {
	for in, want := range map[string]string{"It holds.": "it holds.", "API key.": "API key.", "lower": "lower", "": "", "I": "I", "I can't.": "I can't.", "I'll try.": "I'll try.", "I\u2019ll try.": "I\u2019ll try."} {
		if got := afterColon(in); got != want {
			t.Errorf("afterColon(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFireBodyMatchesFixture(t *testing.T) {
	var got []byte
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write(teamAnswer(t))
			return
		}
		got, _ = io.ReadAll(r.Body)
		w.WriteHeader(204)
	})
	var team TeamReply
	json.Unmarshal(teamAnswer(t), &team)
	if code, _, e := runGM(app.env(t, withToken()), "fire", team.Team[0].ID); code != 0 {
		t.Fatalf("exit %d %s", code, e)
	}
	var want, have map[string]any
	json.Unmarshal(fixtureField(t, "fire.json", "request"), &want)
	delete(want, "force")
	delete(want, "removeWorktree")
	want["id"] = team.Team[0].ID
	json.Unmarshal(got, &have)
	if !reflect.DeepEqual(want, have) {
		t.Errorf("body %s want %v", got, want)
	}
}

func TestFireRefusalsArePrintedAsTheAppWroteThem(t *testing.T) {
	var team TeamReply
	json.Unmarshal(teamAnswer(t), &team)
	id := team.Team[0].ID
	long := "The worktree has changes nobody committed:\n" + strings.Repeat(" M Sources/file.swift\n", 40) + "Send force to fire anyway."
	for _, text := range []string{long, "Its window shows a question or a menu: answer it first.", "That hire is fired already."} {
		app := fireApp(t, 409, text)
		code, _, errOut := runGM(app.env(t, withToken()), "fire", id)
		if code != 1 || !strings.Contains(errOut, text) {
			t.Errorf("code %d: %q", code, errOut)
		}
	}
	app := fireApp(t, 409, "bad \x1b[31m text "+fakeToken)
	_, _, errOut := runGM(app.env(t, withToken()), "fire", id)
	if strings.Contains(errOut, "\x1b") || strings.Contains(errOut, fakeToken) {
		t.Errorf("unsafe output %q", errOut)
	}
}

func TestFireNobodyOrManyNamed(t *testing.T) {
	app := fireApp(t, 204, "")
	code, _, errOut := runGM(app.env(t, withToken()), "fire", "nobody-here")
	if code != 1 || !strings.Contains(errOut, "nobody on the team") || len(app.reqs) != 1 {
		t.Errorf("code %d %q reqs %d", code, errOut, len(app.reqs))
	}
}

func TestBriefSendsIdAndTextAsTheFixtureHasThem(t *testing.T) {
	team := teamAnswer(t)
	var got []byte
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write(team)
			return
		}
		got, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{"ok":true,"file":"/s/hires/x/BRIEF.md","mail":{"id":"brief-1","state":"delivered"}}`))
	})
	var tm TeamReply
	json.Unmarshal(team, &tm)
	m := tm.Team[0]
	code, out, errOut := runGM(app.env(t, withToken()), "brief", m.ID, "--text", "also check the exporter")
	if code != 0 || !strings.Contains(out, "Added to") || errOut != "" {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	r := app.reqs[len(app.reqs)-1]
	if r.Method != "POST" || r.Path != "/setup/gm/brief" || r.Header.Get("x-smolcorp-gm-token") != fakeToken {
		t.Errorf("request = %+v", r)
	}
	var want, have map[string]any
	json.Unmarshal(fixtureField(t, "brief.json", "request"), &want)
	want["id"], want["text"] = m.ID, "also check the exporter"
	if err := json.Unmarshal(got, &have); err != nil || !reflect.DeepEqual(want, have) {
		t.Errorf("body = %s want %v", got, want)
	}
}

func TestBriefRefusalIsPrintedWholeAndNoteWhenNotMailed(t *testing.T) {
	app := fireApp(t, 409, "Ada is already gone.")
	var tm TeamReply
	json.Unmarshal(teamAnswer(t), &tm)
	code, _, errOut := runGM(app.env(t, withToken()), "brief", tm.Team[0].ID, "--text", "more")
	if code != 1 || !strings.Contains(errOut, "already gone") {
		t.Errorf("code %d %q", code, errOut)
	}
	if got := briefNote([]byte(`{"ok":true,"note":"The new part is in the brief, but the hire couldn't be mailed: x\u001b[31m"}`)); !strings.Contains(got, "couldn't be mailed") || strings.Contains(got, "\x1b") {
		t.Errorf("note %q", got)
	}
	if briefNote([]byte(`{"ok":true}`)) != "" {
		t.Error("a note from nothing")
	}
}

func TestDeskRoutesAreGMRoutesInTheTable(t *testing.T) {
	var table struct {
		Routes []struct {
			Method, Path string
			Callers      []string
		} `json:"routes"`
	}
	json.Unmarshal(fixture(t, "routes.json"), &table)
	for _, path := range []string{"/setup/gm/hire", "/setup/gm/fire", "/setup/gm/brief"} {
		ok := false
		for _, r := range table.Routes {
			if r.Method == "POST" && r.Path == path {
				for _, c := range r.Callers {
					ok = ok || c == "gm"
				}
			}
		}
		if !ok {
			t.Errorf("%s is not a GM route in routes.json", path)
		}
	}
}

// A test copy's hires read the copy's support folder, never the real app's.
func TestSupportDirIsTheTestCopysWhenNamed(t *testing.T) {
	copyDir := t.TempDir()
	t.Setenv("SMOLCORP_TEST_SUPPORT_DIR", copyDir)
	got, err := supportDir()
	if err != nil || got != copyDir {
		t.Fatalf("supportDir() = %q, %v; want the copy's %q", got, err, copyDir)
	}
	t.Setenv("SMOLCORP_TEST_SUPPORT_DIR", "")
	got, err = supportDir()
	if err != nil || got == copyDir {
		t.Fatalf("supportDir() = %q, %v; want the app's own folder once the variable is gone", got, err)
	}
}

func TestHelpDoesNotSayBriefIsMissing(t *testing.T) {
	app := newFakeApp(t, serve(teamAnswer(t), 200))
	_, out, _ := runGM(app.env(t, withToken()), "help")
	if strings.Contains(out, "not in the app yet") || strings.Contains(out, "says so") {
		t.Errorf("help still says brief is missing: %q", out)
	}
	if !strings.Contains(out, "brief <hire>") {
		t.Errorf("help lost the brief line: %q", out)
	}
}

// fakeMac answers like the Mac app: a locked /setup/state says "mac", and a bad token is a 401.
func fakeMacHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/setup/state" {
		w.Header().Set("X-Smolcorp-App", "mac")
	}
	w.WriteHeader(http.StatusUnauthorized)
}

// fakeGoApp answers like the Go app: the locked /setup/state names its OS, and no GM caller exists.
func fakeGoAppHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/setup/state" {
		w.Header().Set("X-Smolcorp-App", "linux")
	}
	w.WriteHeader(http.StatusUnauthorized)
}

// SIERRA 1.24 F-1: each app's wording, told apart by what the server says.
func TestGMWordingFollowsTheAppThatAnswers(t *testing.T) {
	const macNoToken = "Allow a GM started by hand"
	mac := newFakeApp(t, fakeMacHandler)
	goApp := newFakeApp(t, fakeGoAppHandler)

	// No token: the Mac's whole message, with its own temp folder.
	macEnv := mac.env(t, nil)
	dir, _ := macEnv.SupportDir()
	_, _, errOut := runGM(macEnv, "team")
	wantMac := "smolcorp gm: no GM token: set $SMOLCORP_GM_TOKEN (a GM hired from Smolcorp has it), or turn on 'Allow a GM started by hand' so gm-token is made in " + dir + ".\n"
	if errOut != wantMac {
		t.Errorf("Mac, no token: %q, want %q", errOut, wantMac)
	}
	code, _, errOut := runGM(goApp.env(t, nil), "team")
	if code != exitApp || errOut != "smolcorp gm: there's no GM token here: the GM gets one when the owner hires it from Setup, in its own environment ($SMOLCORP_GM_TOKEN).\n" || strings.Contains(errOut, macNoToken) {
		t.Errorf("Go app, no token: %d %q", code, errOut)
	}

	// With a token: the Mac's 401 is as it was, the Go app's is one plain sentence.
	_, _, errOut = runGM(mac.env(t, withToken()), "team")
	if errOut != "smolcorp gm: Smolcorp answered 401: Unauthorized\n" {
		t.Errorf("Mac, token: %q", errOut)
	}
	code, _, errOut = runGM(goApp.env(t, withToken()), "team")
	want := "smolcorp gm: this token isn't the one Smolcorp gave its GM (it changes when the GM is hired again, and Smolcorp forgets it when it restarts). Ask the owner to hire the GM again from Setup.\n"
	if code != exitApp || errOut != want || strings.Contains(errOut, fakeToken) {
		t.Errorf("Go app, token: %d %q", code, errOut)
	}

	// The same sentences through the POST paths (hire, send).
	posts := [][]string{{"hire", "--role", "janitor", "--task", "tidy up"}, {"send", "Rev", "--text", "hi"}}
	for _, args := range posts {
		_, _, errOut = runGM(mac.env(t, withToken()), args...)
		if errOut != "smolcorp gm: Smolcorp answered 401: Unauthorized\n" {
			t.Errorf("Mac, token, %v: %q", args, errOut)
		}
		code, _, errOut = runGM(goApp.env(t, withToken()), args...)
		if code != exitApp || errOut != want {
			t.Errorf("Go app, token, %v: %d %q", args, code, errOut)
		}
	}
}

// An app that says nothing, or isn't there, keeps the Mac's wording.
func TestGMWordingStaysTheMacsWhenTheAppIsUnclear(t *testing.T) {
	silent := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	_, _, errOut := runGM(silent.env(t, nil), "team")
	if !strings.Contains(errOut, "Allow a GM started by hand") {
		t.Errorf("silent app: %q", errOut)
	}
	gone := newFakeApp(t, fakeGoAppHandler)
	env := gone.env(t, nil)
	gone.srv.Close()
	code, _, errOut := runGM(env, "team")
	if code != exitApp || !strings.Contains(errOut, "Allow a GM started by hand") {
		t.Errorf("no app: %d %q", code, errOut)
	}
}

// The look at which app answers carries no credentials, and a hire's 401 gets no look:
// the stale-token sentence is for a GM's token only.
func TestTheAppLookSendsNoCredentialsAndSkipsAHire(t *testing.T) {
	goApp := newFakeApp(t, fakeGoAppHandler)
	runGM(goApp.env(t, withToken()), "team")
	looks := 0
	for _, r := range goApp.reqs {
		if r.Path == "/setup/state" {
			looks++
			if r.Header.Get(tokenHeader) != "" || r.Header.Get(hireHeader) != "" {
				t.Errorf("the look sent credentials: %v", r.Header)
			}
		}
	}
	if looks != 1 {
		t.Errorf("looks = %d, want 1", looks)
	}

	hire := newFakeApp(t, fakeGoAppHandler)
	_, _, errOut := runGM(hire.env(t, map[string]string{hireKeyEnv: fakeHireKey}), "send", "gm", "--text", "hi")
	if len(hire.reqs) != 1 || hire.reqs[0].Path == "/setup/state" || strings.Contains(errOut, "GM token") {
		t.Errorf("hire key: %d requests, %q", len(hire.reqs), errOut)
	}
}

// Neither rotate-token nor conventions is a Mac command, so both apps answer "unknown command".
func TestRotateTokenAndConventionsAreUnknownOnBothApps(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{"Mac": fakeMacHandler, "Go app": fakeGoAppHandler} {
		app := newFakeApp(t, h)
		for _, cmd := range []string{"rotate-token", "conventions"} {
			code, _, errOut := runGM(app.env(t, withToken()), cmd)
			if code != exitUsage || !strings.Contains(errOut, "unknown command \""+cmd+"\"") || len(app.reqs) != 0 {
				t.Errorf("%s, %s: %d %q, %d requests", name, cmd, code, errOut, len(app.reqs))
			}
		}
	}
}

// T735: a GM route the Go app doesn't serve says so; the Mac's 404 stays as it was.
func TestUnservedGMRoutesSayWhatTheGoAppLacks(t *testing.T) {
	team := teamAnswer(t)
	notFound := func(app string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Smolcorp-App", app)
			if r.URL.Path == "/setup/gm/team" { // fire and brief look the hire up first
				w.Write(team)
				return
			}
			if r.URL.Path == "/setup/state" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}
	cases := []struct {
		args          []string
		goMsg, macMsg string
	}{
		{[]string{"hire", "--role", "janitor", "--task", "tidy"}, "smolcorp gm: `gm hire` isn't available in Smolcorp on Linux and Windows yet: the owner hires from the office or Setup.\n", "smolcorp gm: Smolcorp answered 404: not found\n"},
		{[]string{"fire", "Rev"}, "smolcorp gm: `gm fire` isn't available in Smolcorp on Linux and Windows yet.\n", ""},
		{[]string{"brief", "Rev", "--text", "hi"}, "smolcorp gm: `gm brief` isn't available in Smolcorp on Linux and Windows yet.\n", ""},
		{[]string{"tile"}, "smolcorp gm: `gm tile` isn't available in Smolcorp on Linux and Windows yet.\n", "smolcorp gm: this Smolcorp can't tile windows yet; update it.\n"},
		{[]string{"screens"}, "smolcorp gm: `gm screens` isn't available in Smolcorp on Linux and Windows yet.\n", "smolcorp gm: this Smolcorp can't choose a screen yet; update it.\n"},
		{[]string{"accounts"}, "smolcorp gm: `gm accounts` isn't available in Smolcorp on Linux and Windows yet.\n", "smolcorp gm: this Smolcorp has no accounts route yet; update it.\n"},
		{[]string{"screen", "1"}, "smolcorp gm: `gm screen` isn't available in Smolcorp on Linux and Windows yet.\n", "smolcorp gm: this Smolcorp can't choose a screen yet; update it.\n"},
	}
	goApp := newFakeApp(t, notFound("linux"))
	mac := newFakeApp(t, notFound("mac"))
	for _, c := range cases {
		code, _, errOut := runGM(goApp.env(t, withToken()), c.args...)
		if code != exitApp || errOut != c.goMsg {
			t.Errorf("Go app, %v: %d %q", c.args, code, errOut)
		}
		if c.macMsg == "" {
			continue
		}
		_, _, errOut = runGM(mac.env(t, withToken()), c.args...)
		if errOut != c.macMsg {
			t.Errorf("Mac, %v: %q, want %q", c.args, errOut, c.macMsg)
		}
	}
	// Any other 404 on the Go app stays as it was.
	_, _, errOut := runGM(goApp.env(t, withToken()), "outbox")
	if errOut != "smolcorp gm: Smolcorp answered 404: not found\n" {
		t.Errorf("Go app, other 404: %q", errOut)
	}
}

// T735: the help names which app each fact is for, and the malformed-token line follows the app.
func TestHelpAndBadTokenNameTheirApp(t *testing.T) {
	app := newFakeApp(t, serve(teamAnswer(t), 200))
	_, out, _ := runGM(app.env(t, withToken()), "help")
	for _, want := range []string{"On Linux and Windows, hire, brief, fire, tile, screens, screen and accounts\naren't served yet.", "on the Mac, ask the owner to rotate it", "hire the GM again from Setup"} {
		if !strings.Contains(out, want) {
			t.Errorf("help lacks %q", want)
		}
	}
	bad := map[string]string{tokenEnv: "bad token"}
	_, _, errOut := runGM(newFakeApp(t, fakeMacHandler).env(t, bad), "team")
	if errOut != "smolcorp gm: the GM token is malformed; ask the owner to rotate it in Smolcorp's setup.\n" {
		t.Errorf("Mac: %q", errOut)
	}
	_, _, errOut = runGM(newFakeApp(t, fakeGoAppHandler).env(t, bad), "team")
	if errOut != "smolcorp gm: the GM token is malformed; ask the owner to hire the GM again from Setup.\n" {
		t.Errorf("Go app: %q", errOut)
	}
}

// The GM sends the id of the Claude Code session it runs in, so the Mac app can tell which session
// is a GM started by hand. Unset, empty or malformed, no header goes; a hire's key never sends it.
func TestSessionHeaderIsSentOnlyWhenWellFormed(t *testing.T) {
	const id = "0b1f6c1e-52aa-4d0e-9a57-3c1d2f4e8a90"
	for _, tc := range []struct{ name, value, want string }{
		{"set", id, id},
		{"unset", "", ""},
		{"spaces inside", "abc def", ""},
		{"control character", "abc\x01def", ""},
		{"newline inside", "abc\ndef", ""},
		{"not ASCII", "séance", ""},
		{"too long", strings.Repeat("a", 65), ""},
	} {
		app := newFakeApp(t, serve(teamAnswer(t), 200))
		env := withToken()
		if tc.value != "" {
			env[sessionEnv] = tc.value
		}
		if code, _, e := runGM(app.env(t, env), "team"); code != 0 {
			t.Fatalf("%s: exit %d: %s", tc.name, code, e)
		}
		if got := app.reqs[0].Header.Get("x-smolcorp-session"); got != tc.want {
			t.Errorf("%s: session header = %q, want %q", tc.name, got, tc.want)
		}
	}
	// A hire's environment signs in with its key alone.
	app := newFakeApp(t, serve(fixtureField(t, "send.json", "answer"), 200))
	runGM(app.env(t, map[string]string{hireKeyEnv: fakeHireKey, sessionEnv: id}), "send", "Rev", "--text", "x")
	if got := app.reqs[0].Header.Get("x-smolcorp-session"); got != "" {
		t.Errorf("a hire sent the session header: %q", got)
	}
}
