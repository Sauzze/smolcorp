package gm

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests run against a stand-in for the app (a local test server, a fake key), never a real office. The real
// router is asked in desktop/peoplecli_test.go.

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

// standIn answers the routes status and desks read, as the app does, and allows their ask for a key (allowing).
// Any other route is a 404 the test sees.
type standIn struct{}

func (s *standIn) handler(t *testing.T) http.HandlerFunc {
	return allowing(fakeSetupKey, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /setup/gm/team":
			w.Write([]byte(peopleTeam))
		case "GET /setup/gm/version":
			w.Write([]byte(peopleVersion))
		case "GET /setup/gm/approvals":
			w.Write([]byte(peopleApprovals))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// fakeAskCode is the code the stand-in gives a terminal's ask.
const fakeAskCode = "1-2-3-4"

// allowing answers a terminal's ask for a key as the app does once the owner clicks Allow: a ticket and a code, then
// key on that ticket. Anything else goes to h.
func allowing(key string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != terminalAskPath {
			h(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"ticket":"fake-ticket","code":"` + fakeAskCode + `","where":"Smolcorp's Settings"}`))
			return
		}
		if r.URL.Query().Get("ticket") != "fake-ticket" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"state":"allowed","key":"` + key + `"}`))
	}
}

// personEnv is an environment with files kept in its support folder, as earlier versions kept keys there: status and
// desks read none of them.
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
	return map[string]string{"setup-key": fakeSetupKey, "window-key": fakeWindowKey, "gm-token": fakeToken}
}

// readsOfTheTeam is the requests that aren't the ask for a key.
func readsOfTheTeam(app *fakeApp) []seen {
	var reads []seen
	for _, r := range app.reqs {
		if r.Path != terminalAskPath {
			reads = append(reads, r)
		}
	}
	return reads
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
	// The ask's code goes to the terminal, apart from the answer: --json stays the app's alone.
	if code != 0 || !strings.Contains(errOut, "Check that it shows this code: "+fakeAskCode) {
		t.Fatalf("exit %d %q", code, errOut)
	}
	for _, want := range []string{"Smolcorp 1.26.0. 4 on the team: 1 working, 1 asking you, 1 idle, 1 not running.", "Waiting for you:", "Bo", "Bash: npm test"} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
	// Only reads, only routes the owner's tool may read, with the key the ask was given and nothing else.
	for _, r := range readsOfTheTeam(app) {
		if r.Method != "GET" || !strings.HasPrefix(r.Path, "/setup/gm/") {
			t.Errorf("status called %s %s", r.Method, r.Path)
		}
		if r.Header.Get(setupKeyHeader) != fakeSetupKey || r.Header.Get("X-AgentDeck-Token") != "" || r.Header.Get(tokenHeader) != "" || r.Header.Get(sessionHeader) != "" {
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
	app := newFakeApp(t, allowing(fakeSetupKey, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup/gm/team":
			w.Write([]byte(`{"team":[]}`))
		case "/setup/gm/approvals":
			w.Write([]byte(`{"approvals":[]}`))
		default:
			w.Write([]byte(`{"app":"Smolcorp","version":"1.26.0"}`))
		}
	}))
	code, out, _ := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "status")
	if code != 0 || !strings.Contains(out, "Nobody on the team yet.") || !strings.Contains(out, "Nobody is waiting for you.") {
		t.Errorf("exit %d %q", code, out)
	}
}

func TestDesksListsEachDeskAsTheOfficeNumbersThem(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "desks")
	if code != 0 || strings.Contains(errOut, "smolcorp:") {
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

// The GM's token, a session id and the window key are never sent, and no key file is read (1.26, SK2: the key comes
// from the ask): a request with the GM's token would make the app note a GM "started by hand" and move its role to
// the calling session.
func TestStatusAndDesksSendTheAllowedKeyAloneAndReadNoKeyFile(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	vars := withToken()
	vars[sessionEnv] = "11111111-2222-3333-4444-555555555555"
	env := personEnv(t, app, vars, bothKeys())
	real := env.readFile
	var read []string
	env.ReadFile = func(name string) ([]byte, error) { read = append(read, filepath.Base(name)); return real(name) }
	for _, cmd := range []string{"status", "desks"} {
		if code, _, errOut := runPeopleCmd(env, cmd); code != 0 {
			t.Fatalf("%s: exit %d %q", cmd, code, errOut)
		}
	}
	if len(read) != 0 {
		t.Errorf("files read: %v, want none", read)
	}
	if len(app.callsTo(http.MethodPost, terminalAskPath)) != 2 {
		t.Errorf("each command asks once: %d asks", len(app.callsTo(http.MethodPost, terminalAskPath)))
	}
	if len(readsOfTheTeam(app)) == 0 {
		t.Fatal("no request reached the app")
	}
	for _, r := range readsOfTheTeam(app) {
		for h := range r.Header {
			switch strings.ToLower(h) {
			case "x-smolcorp-setup-key", "accept", "user-agent", "content-type", "accept-encoding", "content-length":
			default:
				t.Errorf("%s %s sent the header %s", r.Method, r.Path, h)
			}
		}
	}
}

// say, open and hire are one-line refusals: no request, no key file read, whatever follows the word or is in the
// environment (a hire's key set, empty or unset).
func TestSayOpenAndHireAreRefusedInOneLineAndTouchNothing(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	for _, vars := range []map[string]string{nil, {hireKeyEnv: ""}, {hireKeyEnv: "fake-hire-key-89"}, withToken()} {
		env := personEnv(t, app, vars, bothKeys())
		read := false
		env.ReadFile = func(string) ([]byte, error) { read = true; return nil, os.ErrNotExist }
		for _, args := range [][]string{
			{"say", "ada", "hi"}, {"say"}, {"say", "--json"}, {"open", "3"}, {"open", "--help"}, {"open"},
			{"hire", "tester"}, {"hire", "tester", "--folder", "/work", "--task-file", "-"}, {"hire", "--help"}, {"hire"},
		} {
			code, out, errOut := runPeopleCmd(env, args...)
			if code != 1 || out != "" || strings.Count(strings.TrimSpace(errOut), "\n") != 0 ||
				!strings.Contains(errOut, "comes with app keys in Settings") || !strings.Contains(errOut, "smolcorp "+args[0]) {
				t.Errorf("%v: exit %d %q %q", args, code, out, errOut)
			}
		}
		if read {
			t.Errorf("a refusal read a file (environment %v)", vars)
		}
	}
	if len(app.reqs) != 0 {
		t.Errorf("a refusal reached the app: %d requests", len(app.reqs))
	}
}

// A hire's key in the environment is a polite refusal for status and desks, with no key read and no ask made. It is
// not a fence (the variable can be emptied: the guard stands in the way of that), so what an emptied one does is only
// what a person's does: ask, and wait for the owner's Allow with the code.
func TestAHireIsRefusedStatusAndDesksPolitelyAndNoKeyIsReadForIt(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	env := personEnv(t, app, map[string]string{hireKeyEnv: "fake-hire-key-89"}, bothKeys())
	read := false
	env.ReadFile = func(string) ([]byte, error) { read = true; return nil, os.ErrNotExist }
	for _, args := range [][]string{{"status"}, {"desks"}} {
		code, out, errOut := runPeopleCmd(env, args...)
		if code != 1 || out != "" || !strings.Contains(errOut, "not for a hire") {
			t.Errorf("%v: exit %d %q %q", args, code, out, errOut)
		}
	}
	if read || len(app.reqs) != 0 {
		t.Errorf("a hire's call read a key (%v) or reached the app (%d requests)", read, len(app.reqs))
	}
}

// The owner's no in the window: nothing is read, and the terminal says so.
func TestANoInTheWindowReadsNothing(t *testing.T) {
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"ticket":"fake-ticket","code":"` + fakeAskCode + `","where":"Smolcorp's Settings"}`))
			return
		}
		w.Write([]byte(`{"state":"denied"}`))
	})
	code, out, errOut := runPeopleCmd(personEnv(t, app, nil, bothKeys()), "status")
	if code != 1 || out != "" || !strings.Contains(errOut, "you said no") || len(readsOfTheTeam(app)) != 0 {
		t.Errorf("exit %d %q %q, %d reads", code, out, errOut, len(readsOfTheTeam(app)))
	}
}

func TestNoKeyIsEverPrinted(t *testing.T) {
	// An app that echoes what it was sent, in an error, in the answer of every command.
	app := newFakeApp(t, allowing(fakeSetupKey, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("bad " + r.Header.Get(setupKeyHeader) + " " + r.Header.Get("X-AgentDeck-Token") + " " + r.Header.Get(tokenHeader)))
	}))
	env := personEnv(t, app, withToken(), bothKeys())
	for _, args := range [][]string{{"status"}, {"desks"}} {
		_, out, errOut := runPeopleCmd(env, args...)
		for _, key := range []string{fakeSetupKey, fakeWindowKey, fakeToken} {
			if strings.Contains(out+errOut, key) {
				t.Errorf("%v printed a key: %q %q", args, out, errOut)
			}
		}
	}
}

func TestPeopleUsageErrorsAndHelp(t *testing.T) {
	app := newFakeApp(t, (&standIn{}).handler(t))
	env := personEnv(t, app, nil, bothKeys())
	for _, args := range [][]string{{"status", "extra"}, {"desks", "extra"}, {"status", "--nope"}} {
		if code, _, errOut := runPeopleCmd(env, args...); code != 2 || !strings.Contains(errOut, "Usage: smolcorp") {
			t.Errorf("%v: exit %d %q", args, code, errOut)
		}
	}
	if len(app.reqs) != 0 {
		t.Errorf("a usage error reached the app: %d requests", len(app.reqs))
	}
	for _, cmd := range []string{"status", "desks"} {
		code, out, _ := runPeopleCmd(env, cmd, "--help")
		if code != 0 || !strings.HasPrefix(out, "Usage: smolcorp <command>") || !strings.Contains(out, "\n  "+cmd+" ") {
			t.Errorf("%s --help: exit %d %q", cmd, code, out)
		}
	}
}
