package gm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// X3's tests run against a stand-in for the app (a local test server, a fake key), never a real office.

const (
	fakeSetupKey  = "fake-setup-key-0123"
	fakeWindowKey = "fake-window-key-4567"
)

// peopleTeam is a team of three: Ada works at desk 3, Bo asks at desk 5, Cy isn't running.
const peopleTeam = `{"at":1,"contacts":[],"team":[
 {"id":"h1","name":"Ada","nickname":"ada","role":"reviewer","registered":true,"live":true,"liveState":"thinking","sessionId":"s-ada","desk":2,"floor":null,"state":"working","status":"on the importer","idleFor":5},
 {"id":"h2","name":"Bo","nickname":"","role":"tester","registered":true,"live":true,"liveState":"asking","sessionId":"s-bo","desk":4,"floor":null,"state":"working","status":"","idleFor":125},
 {"id":"h3","name":"Cy","nickname":"","role":"builder","registered":true,"live":false,"liveState":"gone","sessionId":"","desk":null,"floor":null,"state":"fired","status":""},
 {"id":"h4","name":"Di","nickname":"","role":"docs","registered":true,"live":true,"liveState":"idle","sessionId":"s-di","desk":2,"floor":2,"state":"working","status":""}]}`

const peopleVersion = `{"app":"Smolcorp","version":"1.26.0","api":5,"features":[]}`

const peopleApprovals = `{"at":1,"approvals":[{"session":"s-bo","hire":"h2","name":"Bo","nickname":"","agent":"claude","floor":null,"desk":4,"kind":"permission","tool":"Bash","label":"Bash: npm test","since":1,"waitingSeconds":125,"notified":false}]}`

// standIn answers the routes the people's commands use, as the app does; say and open's calls are recorded in the
// fakeApp's requests. card is what GET /talk says, say what POST /talk/say answers, with status.
type standIn struct {
	card      string
	sayStatus int
	sayText   string
	hire      func(w http.ResponseWriter, r *http.Request)
	bodies    map[string][]byte
}

func (s *standIn) handler(t *testing.T) http.HandlerFunc {
	s.bodies = map[string][]byte{}
	if s.card == "" {
		s.card = `{"state":"thinking","asking":null,"last":"","size":0,"typing":{"ok":true,"lines":true,"why":""}}`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			s.bodies[r.Method+" "+r.URL.Path], _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /setup/gm/team":
			w.Write([]byte(peopleTeam))
		case "GET /setup/gm/version":
			w.Write([]byte(peopleVersion))
		case "GET /setup/gm/approvals":
			w.Write([]byte(peopleApprovals))
		case "GET /talk":
			w.Write([]byte(s.card))
		case "POST /talk/say":
			if s.sayStatus != 0 {
				w.WriteHeader(s.sayStatus)
				w.Write([]byte(s.sayText))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case "POST /focus":
			w.WriteHeader(http.StatusNoContent)
		case "POST /setup/gm/hire":
			s.hire(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// personEnv is an environment with the keys given kept in files in its support folder, as the app keeps them.
func personEnv(t *testing.T, app *fakeApp, vars map[string]string, files map[string]string) Env {
	t.Helper()
	env := app.env(t, vars)
	dir, _ := env.SupportDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return env
}

func bothKeys() map[string]string {
	return map[string]string{setupKeyFileName: fakeSetupKey, windowKeyFileName: fakeWindowKey}
}

func runPeopleCmd(env Env, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := runPeople(args[0], args[1:], env, &out, &errOut)
	return code, out.String(), errOut.String()
}

func (f *fakeApp) callsTo(method, path string) []seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []seen
	for _, r := range f.reqs {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func TestStatusSaysTheTeamAndWhoWaits(t *testing.T) {
	si := &standIn{}
	app := newFakeApp(t, si.handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "status")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d %q", code, errOut)
	}
	for _, want := range []string{"Smolcorp 1.26.0. 4 on the team: 1 working, 1 asking you, 1 idle, 1 not running.", "Waiting for you:", "Bo", "Bash: npm test"} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
	// Only reads, only routes the owner's tool may read, with the setup key and no window key.
	for _, r := range app.reqs {
		if r.Method != "GET" || !strings.HasPrefix(r.Path, "/setup/gm/") {
			t.Errorf("status called %s %s", r.Method, r.Path)
		}
		if r.Header.Get(setupKeyHeader) != fakeSetupKey || r.Header.Get(windowKeyHeader) != "" || r.Header.Get(tokenHeader) != "" {
			t.Errorf("status sent the wrong keys: %v", r.Header)
		}
	}
}

func TestStatusJSONHoldsTheThreeAnswersAsServed(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	code, out, _ := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "status", "--json")
	var got map[string]json.RawMessage
	if code != 0 || json.Unmarshal([]byte(out), &got) != nil || len(got) != 3 {
		t.Fatalf("exit %d %q", code, out)
	}
	var v VersionInfo
	if json.Unmarshal(got["version"], &v) != nil || v.Version != "1.26.0" {
		t.Errorf("version = %s", got["version"])
	}
}

func TestStatusOnAnEmptyOffice(t *testing.T) {
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup/gm/team":
			w.Write([]byte(`{"team":[]}`))
		case "/setup/gm/approvals":
			w.Write([]byte(`{"approvals":[]}`))
		default:
			w.Write([]byte(`{"app":"Smolcorp","version":"1.26.0"}`))
		}
	})
	code, out, _ := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "status")
	if code != 0 || !strings.Contains(out, "Nobody on the team yet.") || !strings.Contains(out, "Nobody is waiting for you.") {
		t.Errorf("exit %d %q", code, out)
	}
}

func TestDesksListsEachDeskAsTheOfficeNumbersThem(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "desks")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d %q", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 || !strings.HasPrefix(lines[0], "DESK") {
		t.Fatalf("table = %q", out)
	}
	for i, want := range []string{"3  ada  reviewer  thinking  5s  on  the  importer", "5  Bo  tester  asking  2m", "-  Cy  builder", "2/3  Di  docs  idle"} {
		if got := strings.Join(strings.Fields(lines[i+1]), "  "); !strings.HasPrefix(got, want) {
			t.Errorf("line %d = %q, want it to start %q", i+1, got, want)
		}
	}
	_, raw, _ := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "desks", "--json")
	var t2 TeamReply
	if json.Unmarshal([]byte(raw), &t2) != nil || len(t2.Team) != 4 {
		t.Errorf("--json = %q", raw)
	}
}

func TestSayTypesIntoADeskThroughTheWindowsRoute(t *testing.T) {
	si := &standIn{}
	app := newFakeApp(t, si.handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "say", "ada", "run", "the tests")
	if code != 0 || out != "Sent to ada.\n" || errOut != "" {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	say := app.callsTo("POST", "/talk/say")
	if len(say) != 1 || say[0].RawQuery != "session=s-ada" {
		t.Fatalf("say calls = %+v", say)
	}
	// The window's key goes to the window's routes only; the setup key never does.
	if say[0].Header.Get(windowKeyHeader) != fakeWindowKey || say[0].Header.Get(setupKeyHeader) != "" || say[0].Header.Get(tokenHeader) != "" {
		t.Errorf("say's keys = %v", say[0].Header)
	}
	var body map[string]any
	if json.Unmarshal(si.bodies["POST /talk/say"], &body); body["text"] != "run the tests" || body["send"] != true {
		t.Errorf("say's body = %s", si.bodies["POST /talk/say"])
	}
	for _, r := range app.callsTo("GET", "/setup/gm/team") {
		if r.Header.Get(windowKeyHeader) != "" {
			t.Errorf("the window's key went to the team route: %v", r.Header)
		}
	}
}

func TestSayIsRefusedWhileTheDeskIsAsking(t *testing.T) {
	si := &standIn{card: `{"state":"asking","asking":{"kind":"permission","tool":"Bash","label":"Bash: rm -rf build"},"typing":{"ok":false,"lines":false,"why":"It's asking you something: answer it first, above."}}`}
	app := newFakeApp(t, si.handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "say", "5", "yes")
	if code != 1 || out != "" || !strings.Contains(errOut, "Bo is asking you something (Bash: rm -rf build): answer it first") || !strings.Contains(errOut, "Nothing was typed.") {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	if n := len(app.callsTo("POST", "/talk/say")); n != 0 {
		t.Errorf("typed into an asking desk: %d say calls", n)
	}
}

func TestSayAnswersTheAppsRefusalWhenTheDeskBeganAskingMeanwhile(t *testing.T) {
	// The card was clear, but the desk asked before the words went: the app's own check refuses.
	si := &standIn{sayStatus: 409, sayText: windowAnswerFirstForTests}
	app := newFakeApp(t, si.handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "say", "ada", "hello")
	if code != 1 || out != "" || !strings.Contains(errOut, "409") || !strings.Contains(errOut, windowAnswerFirstForTests) {
		t.Errorf("exit %d %q %q", code, out, errOut)
	}
}

// windowAnswerFirstForTests is desktop's windowAnswerFirst, which the app answers with.
const windowAnswerFirstForTests = "It's asking you something: answer it first, above."

func TestSayRefusesADeskTheWindowCantTypeInto(t *testing.T) {
	si := &standIn{card: `{"state":"thinking","asking":null,"typing":{"ok":false,"lines":false,"why":"Type in its own window."}}`}
	app := newFakeApp(t, si.handler(t))
	code, _, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "say", "ada", "hello")
	if code != 1 || !strings.Contains(errOut, "can't type into ada: Type in its own window. Nothing was typed.") {
		t.Errorf("exit %d %q", code, errOut)
	}
	if len(app.callsTo("POST", "/talk/say")) != 0 {
		t.Error("said into a desk the window can't type into")
	}
}

func TestSayAtAClosedDeskAndAtNoDesk(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	env := personEnv(t, app, nil, bothKeys())
	if code, _, errOut := runPeopleCmd(env, "say", "cy", "hi"); code != 1 || !strings.Contains(errOut, "Cy has no open desk") {
		t.Errorf("exit %d %q", code, errOut)
	}
	if code, _, errOut := runPeopleCmd(env, "say", "nobody", "hi"); code != 1 || !strings.Contains(errOut, `no desk is called "nobody"`) {
		t.Errorf("exit %d %q", code, errOut)
	}
	if n := len(app.callsTo("POST", "/talk/say")); n != 0 {
		t.Errorf("%d say calls", n)
	}
}

func TestSayWithoutTheWindowKeySaysSoAndAsksNothing(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	env := personEnv(t, app, nil, map[string]string{setupKeyFileName: fakeSetupKey})
	code, _, errOut := runPeopleCmd(env, "say", "ada", "hi")
	if code != 1 || !strings.Contains(errOut, "no office window key") || len(app.reqs) != 0 {
		t.Errorf("exit %d %q, %d requests", code, errOut, len(app.reqs))
	}
}

func TestOpenBringsTheDeskForward(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "open", "3")
	if code != 0 || out != "Brought ada's terminal forward.\n" || errOut != "" {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	focus := app.callsTo("POST", "/focus")
	if len(focus) != 1 || focus[0].RawQuery != "session=s-ada" || focus[0].Header.Get(windowKeyHeader) != fakeWindowKey {
		t.Errorf("focus calls = %+v", focus)
	}
	// A floor's desk, as "floor/desk".
	if code, out, _ := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "open", "2/3"); code != 0 || out != "Brought Di's terminal forward.\n" {
		t.Errorf("exit %d %q", code, out)
	}
	if code, _, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "open", "7"); code != 1 || !strings.Contains(errOut, `no desk is called "7"`) {
		t.Errorf("exit %d %q", code, errOut)
	}
}

func TestOpenAnswersTheAppsRefusal(t *testing.T) {
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/focus" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("This terminal can't do that"))
			return
		}
		(&standIn{}).handler(t)(w, r)
	})
	code, _, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "open", "ada")
	if code != 1 || !strings.Contains(errOut, "This terminal can't do that") {
		t.Errorf("exit %d %q", code, errOut)
	}
}

func TestHireGoesToTheHireRouteAndPrintsTheAnswer(t *testing.T) {
	si := &standIn{hire: func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":true,"desk":3,"floor":0}`)) }}
	app := newFakeApp(t, si.handler(t))
	env := personEnv(t, app, withToken(), bothKeys())
	code, out, errOut := runPeopleCmd(env, "hire", "reviewer", "--folder", "/work/app", "--nickname", "Ada", "--worktree", "--task", "Review the importer.")
	if code != 0 || out != "Hired a reviewer. At desk 4.\n" || errOut != "" {
		t.Fatalf("exit %d %q %q", code, out, errOut)
	}
	var body map[string]any
	json.Unmarshal(si.bodies["POST /setup/gm/hire"], &body)
	if body["role"] != "reviewer" || body["folder"] != "/work/app" || body["worktree"] != true || body["nickname"] != "Ada" || body["task"] != "Review the importer." || len(body) != 5 {
		t.Errorf("body = %v", body)
	}
	// With the GM token and the setup key, the app tells which is which; the window's key stays home.
	r := app.callsTo("POST", "/setup/gm/hire")[0]
	if r.Header.Get(tokenHeader) != fakeToken || r.Header.Get(setupKeyHeader) != fakeSetupKey || r.Header.Get(windowKeyHeader) != "" || r.Header.Get("X-AgentDeck-Setup") != "1" {
		t.Errorf("hire's headers = %v", r.Header)
	}
}

func TestAHireTheGatesRefuseExitsNonZeroWithTheRefusalWhole(t *testing.T) {
	refusal := "Choose a folder for new hires in setup, on your computer.\nThe fence only covers Claude Code roles."
	si := &standIn{hire: func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(refusal))
	}}
	app := newFakeApp(t, si.handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, withToken(), bothKeys()), "hire", "builder")
	if code != 1 || out != "" || !strings.Contains(errOut, refusal) {
		t.Errorf("exit %d out %q err %q", code, out, errOut)
	}
}

func TestHireWithOnlyTheSetupKeyIsRefusedByTheRouteTable(t *testing.T) {
	// The route table keeps hiring from a tool: with the setup key alone, the app answers 403 and the CLI says so.
	si := &standIn{hire: func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(tokenHeader) == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Only the setup page or the GM can do this."))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}}
	app := newFakeApp(t, si.handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "hire", "reviewer")
	if code != 1 || out != "" || !strings.Contains(errOut, "Only the setup page or the GM can do this.") {
		t.Errorf("exit %d out %q err %q", code, out, errOut)
	}
}

func TestHireTheGMIsLeftToTheAppsRule(t *testing.T) {
	si := &standIn{hire: func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte("Only the owner hires the GM, from Setup."))
	}}
	app := newFakeApp(t, si.handler(t))
	code, _, errOut := runPeopleCmd(personEnv(t, app, withToken(), bothKeys()), "hire", "gm")
	if code != 1 || !strings.Contains(errOut, "Only the owner hires the GM") {
		t.Errorf("exit %d %q", code, errOut)
	}
}

func TestHireFlagsMayComeBeforeTheRole(t *testing.T) {
	si := &standIn{hire: func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":true}`)) }}
	app := newFakeApp(t, si.handler(t))
	code, out, _ := runPeopleCmd(personEnv(t, app, withToken(), bothKeys()), "hire", "--folder", "/work/app", "tester")
	if code != 0 || !strings.Contains(out, "Hired a tester.") {
		t.Errorf("exit %d %q", code, out)
	}
}

func TestAHireIsNotAPersonAndNoKeyIsReadForIt(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	env := personEnv(t, app, map[string]string{hireKeyEnv: "fake-hire-key-89"}, bothKeys())
	read := false
	env.ReadFile = func(string) ([]byte, error) { read = true; return nil, os.ErrNotExist }
	for _, args := range [][]string{{"status"}, {"desks"}, {"say", "ada", "hi"}, {"hire", "tester"}, {"open", "ada"}} {
		code, out, errOut := runPeopleCmd(env, args...)
		if code != 1 || out != "" || !strings.Contains(errOut, "not for a hire") {
			t.Errorf("%v: exit %d %q %q", args, code, out, errOut)
		}
	}
	if read || len(app.reqs) != 0 {
		t.Errorf("a hire's call read a key (%v) or reached the app (%d requests)", read, len(app.reqs))
	}
}

func TestNoKeyAnywhereSaysWhatIsMissing(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	code, _, errOut := runPeopleCmd(personEnv(t, app, nil, nil), "status")
	if code != 1 || !strings.Contains(errOut, "no key for Smolcorp here") || len(app.reqs) != 0 {
		t.Errorf("exit %d %q, %d requests", code, errOut, len(app.reqs))
	}
}

func TestNoKeyIsEverPrinted(t *testing.T) {
	// An app that echoes what it was sent, in an error, in the answer of every command.
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("bad " + r.Header.Get(setupKeyHeader) + " " + r.Header.Get(windowKeyHeader) + " " + r.Header.Get(tokenHeader)))
	})
	env := personEnv(t, app, withToken(), bothKeys())
	for _, args := range [][]string{{"status"}, {"desks"}, {"hire", "tester"}} {
		_, out, errOut := runPeopleCmd(env, args...)
		for _, key := range []string{fakeSetupKey, fakeWindowKey, fakeToken} {
			if strings.Contains(out+errOut, key) {
				t.Errorf("%v printed a key: %q %q", args, out, errOut)
			}
		}
	}
	si := &standIn{sayStatus: 500, sayText: "bad " + fakeWindowKey}
	app2 := newFakeApp(t, si.handler(t))
	for _, args := range [][]string{{"say", "ada", "hi"}} {
		_, out, errOut := runPeopleCmd(personEnv(t, app2, nil, bothKeys()), args...)
		if strings.Contains(out+errOut, fakeWindowKey) {
			t.Errorf("%v printed the window key: %q %q", args, out, errOut)
		}
	}
	// A malformed key file is said to be, without its contents.
	app3 := newFakeApp(t, serve(nil, 200))
	_, _, errOut := runPeopleCmd(personEnv(t, app3, nil, map[string]string{setupKeyFileName: "has a space"}), "status")
	if !strings.Contains(errOut, "setup-key") || strings.Contains(errOut, "has a space") {
		t.Errorf("malformed key: %q", errOut)
	}
}

func TestPeopleUsageErrorsAndHelp(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	env := personEnv(t, app, nil, bothKeys())
	for _, args := range [][]string{
		{"status", "extra"}, {"desks", "extra"}, {"say"}, {"say", "ada"}, {"open"}, {"open", "ada", "now"},
		{"hire"}, {"hire", "tester", "extra"}, {"hire", "tester", "--floor", "9"}, {"hire", "tester", "--task", "a", "--task-file", "b"},
		{"status", "--nope"},
	} {
		if code, _, errOut := runPeopleCmd(env, args...); code != 2 || !strings.Contains(errOut, "Usage: smolcorp") {
			t.Errorf("%v: exit %d %q", args, code, errOut)
		}
	}
	if len(app.reqs) != 0 {
		t.Errorf("a usage error reached the app: %d requests", len(app.reqs))
	}
	for _, cmd := range []string{"status", "desks", "say", "hire", "open"} {
		code, out, _ := runPeopleCmd(env, cmd, "--help")
		if code != 0 || !strings.HasPrefix(out, "Usage: smolcorp <command>") || !strings.Contains(out, "\n  "+cmd+" ") {
			t.Errorf("%s --help: exit %d %q", cmd, code, out)
		}
	}
}

func TestDeskReferencesAreAskedAboutNeverGuessed(t *testing.T) {
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"team":[
		 {"id":"a","name":"Sam","live":true,"sessionId":"s1","desk":0},
		 {"id":"b","name":"sam","live":true,"sessionId":"s2","desk":1}]}`))
	})
	code, _, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "open", "SAM")
	if code != 1 || !strings.Contains(errOut, `2 desks match "SAM"`) {
		t.Errorf("exit %d %q", code, errOut)
	}
	if n := len(app.callsTo("POST", "/focus")); n != 0 {
		t.Errorf("opened one of two: %d focus calls", n)
	}
}

func TestPeopleCommandsUseOnlyRoutesTheirKeysMayUse(t *testing.T) {
	var table struct {
		Routes []struct {
			Method, Path string
			Callers      []string
		} `json:"routes"`
	}
	if err := json.Unmarshal(fixture(t, "routes.json"), &table); err != nil {
		t.Fatal(err)
	}
	callers := func(method, path string) []string {
		for _, r := range table.Routes {
			if r.Method == method && r.Path == path {
				return r.Callers
			}
		}
		return nil
	}
	has := func(list []string, who string) bool {
		for _, c := range list {
			if c == who {
				return true
			}
		}
		return false
	}
	// status and desks: the owner's tool (the setup key) may read each.
	for _, path := range []string{"/setup/gm/team", "/setup/gm/version", "/setup/gm/approvals"} {
		if !has(callers("GET", path), "tool") {
			t.Errorf("GET %s is not a route a tool may use", path)
		}
	}
	// hire: the GM's route, which the table keeps from a tool alone.
	if c := callers("POST", "/setup/gm/hire"); !has(c, "gm") || has(c, "tool") || has(c, "hire") {
		t.Errorf("POST /setup/gm/hire callers = %v", c)
	}
	// say and open: the office window's routes, not setup's, so they aren't in the table: the app's window class
	// (desktop/talksay.go, talk.go) lets in the window's key alone, and /focus is the window's and a phone's.
	for file, want := range map[string][]string{
		filepath.Join("..", "..", "desktop", "talksay.go"): {`path: "/talk/say", access: accessWindow`},
		filepath.Join("..", "..", "desktop", "talk.go"):    {`path: "/talk", access: accessWindow`},
		filepath.Join("..", "..", "desktop", "server.go"):  {`r.URL.Path == "/focus"`},
	} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range want {
			if !strings.Contains(string(src), w) {
				t.Errorf("%s no longer has %s: say and open's route changed", file, w)
			}
		}
	}
}
