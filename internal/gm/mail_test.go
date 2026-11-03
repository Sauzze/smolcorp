package gm

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeHireKey = "fake-hire-key-0123456789"

func body(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	data, _ := io.ReadAll(r.Body)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("body %q: %v", data, err)
	}
	return m
}

// fixtureField is one top-level field of a fixture file, as JSON.
func fixtureField(t *testing.T, file, field string) []byte {
	t.Helper()
	var f map[string]json.RawMessage
	if err := json.Unmarshal(fixture(t, file), &f); err != nil {
		t.Fatal(err)
	}
	if f[field] == nil {
		t.Fatalf("%s has no %q", file, field)
	}
	return f[field]
}

func TestSendFromFixture(t *testing.T) {
	var got map[string]any
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		got = body(t, r)
		serve(fixtureField(t, "send.json", "answer"), 200)(w, r)
	})
	code, out, errOut := runGM(app.env(t, withToken()), "send", "Rev", "--text", "Please look at the login change first.", "--id", "gm-2026-11-01-001")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "delivered to Rev") || !strings.Contains(out, "gm-2026-11-01-001") || !strings.Contains(out, "38 bytes") {
		t.Errorf("out = %q", out)
	}
	r := app.reqs[0]
	if r.Method != "POST" || r.Path != "/setup/gm/send" || r.Header.Get("X-AgentDeck-Setup") != "1" {
		t.Errorf("request = %+v", r)
	}
	if got["to"] != "Rev" || got["text"] != "Please look at the login change first." || got["id"] != "gm-2026-11-01-001" {
		t.Errorf("body = %v", got)
	}
	if r.Header.Get("x-smolcorp-gm-token") != fakeToken || r.Header.Get("x-smolcorp-hire-key") != "" {
		t.Error("a GM sends the GM token, and no hire key")
	}
}

func TestSendMakesAnIDAndFlagsMayFollowTheName(t *testing.T) {
	var got map[string]any
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		got = body(t, r)
		serve([]byte(`{"id":"x","state":"queued","to":"abc","size":2,"note":"It is waiting."}`), 200)(w, r)
	})
	code, out, _ := runGM(app.env(t, withToken()), "send", "--text", "hi", "Rev")
	id, _ := got["id"].(string)
	if code != 0 || !validSendID(id) || !strings.HasPrefix(id, "cli-") {
		t.Errorf("code %d id %q", code, id)
	}
	if !strings.Contains(out, "queued to Rev") || !strings.Contains(out, "It is waiting.") {
		t.Errorf("out = %q", out)
	}
}

func TestSendDuplicateAndFile(t *testing.T) {
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if body(t, r)["text"] != "from a file\n" {
			t.Errorf("text not read from the file")
		}
		serve(fixtureField(t, "send.json", "same-id-again"), 200)(w, r)
	})
	env := app.env(t, withToken())
	f := filepath.Join(t.TempDir(), "m.txt")
	os.WriteFile(f, []byte("from a file\n"), 0o600)
	code, out, _ := runGM(env, "send", "Rev", "--file", f, "--id", "gm-2026-11-01-001")
	if code != 0 || !strings.Contains(out, "not sent again") {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestSendAsAHire(t *testing.T) {
	app := newFakeApp(t, serve(fixtureField(t, "send.json", "answer"), 200))
	// No GM token, a hire key: the hire's header, and only that.
	code, _, errOut := runGM(app.env(t, map[string]string{hireKeyEnv: fakeHireKey}), "send", "gm", "--text", "Review done.", "--id", "h-001")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	h := app.reqs[0].Header
	if h.Get("x-smolcorp-hire-key") != fakeHireKey || h.Get("x-smolcorp-gm-token") != "" {
		t.Errorf("headers = %v", h)
	}
	// Both: the hire key wins, and the GM env token is never sent.
	app2 := newFakeApp(t, serve(fixtureField(t, "send.json", "answer"), 200))
	runGM(app2.env(t, map[string]string{hireKeyEnv: fakeHireKey, tokenEnv: fakeToken}), "send", "Rev", "--text", "x")
	h = app2.reqs[0].Header
	if h.Get("x-smolcorp-hire-key") != fakeHireKey || h.Get("x-smolcorp-gm-token") != "" {
		t.Errorf("both set: headers = %v", h)
	}
}

func TestHireNeverOpensTheTokenFile(t *testing.T) {
	app := newFakeApp(t, serve(fixtureField(t, "send.json", "answer"), 200))
	env := app.env(t, map[string]string{hireKeyEnv: fakeHireKey})
	opened := false
	env.ReadFile = func(string) ([]byte, error) { opened = true; return []byte(fakeToken), nil }
	if code, _, e := runGM(env, "send", "gm", "--text", "x"); code != 0 {
		t.Fatalf("exit %d: %s", code, e)
	}
	if opened {
		t.Error("a hire read the token file")
	}
	h := app.reqs[0].Header
	if h.Get("x-smolcorp-hire-key") != fakeHireKey || h.Get("x-smolcorp-gm-token") != "" {
		t.Errorf("headers = %v", h)
	}
	// A GM-only command is refused in a hire's environment, file untouched.
	if code, _, e := runGM(env, "team"); code != 1 || !strings.Contains(e, "for the GM") || opened || len(app.reqs) != 1 {
		t.Errorf("team: code %d %q opened %v reqs %d", code, e, opened, len(app.reqs))
	}
}

func TestNoHireKeyStillUsesTheGMToken(t *testing.T) {
	app := newFakeApp(t, serve(fixtureField(t, "send.json", "answer"), 200))
	runGM(app.env(t, withToken()), "send", "Rev", "--text", "x")
	h := app.reqs[0].Header
	if h.Get("x-smolcorp-gm-token") != fakeToken || h.Get("x-smolcorp-hire-key") != "" {
		t.Errorf("headers = %v", h)
	}
}

func TestHireKeyOnlyStandsInForSend(t *testing.T) {
	app := newFakeApp(t, serve(nil, 200))
	code, _, errOut := runGM(app.env(t, map[string]string{hireKeyEnv: fakeHireKey}), "outbox")
	if code != 1 || !strings.Contains(errOut, "for the GM") || len(app.reqs) != 0 {
		t.Errorf("code %d %q reqs %d", code, errOut, len(app.reqs))
	}
}

func TestHireKeyNeverInMessages(t *testing.T) {
	app := newFakeApp(t, serve([]byte(`{"error":"bad `+fakeHireKey+`"}`), 403))
	code, _, errOut := runGM(app.env(t, map[string]string{hireKeyEnv: fakeHireKey}), "send", "Rev", "--text", "x")
	if code != 1 || strings.Contains(errOut, fakeHireKey) {
		t.Errorf("code %d %q", code, errOut)
	}
}

func TestSendRefusalsShowTheAppsWords(t *testing.T) {
	app := newFakeApp(t, serve([]byte(`{"error":"A hire may write to the GM, and to another hire only when the GM or the owner has allowed that pair."}`), 403))
	code, _, errOut := runGM(app.env(t, withToken()), "send", "Janitor", "--text", "x")
	if code != 1 || !strings.Contains(errOut, "403") || !strings.Contains(errOut, "allowed that pair") {
		t.Errorf("code %d %q", code, errOut)
	}
}

func TestMailUsageErrors(t *testing.T) {
	app := newFakeApp(t, serve(nil, 200))
	env := app.env(t, withToken())
	for _, args := range [][]string{
		{"send"}, {"send", "Rev"}, {"send", "Rev", "--text", "a", "--file", "b"}, {"send", "Rev", "--text", " "},
		{"send", "Rev", "--text", "a", "--id", "a b"}, {"send", "Rev", "--text", "a", "--id", strings.Repeat("a", 65)},
		{"send", "Rev", "Bob", "--text", "a"}, {"send", "Rev", "--file", "/no/such/file"},
		{"outbox", "x"}, {"outbox", "--since", "-1"},
		{"watch"}, {"watch", "file"}, {"watch", "idle", "Rev", "--minutes", "-1"}, {"watch", "idle", "  "},
		{"unwatch"}, {"unwatch", "a", "b"}, {"watches", "x"},
		{"contacts", "allow", "a"}, {"contacts", "x", "a", "b"}, {"contacts", "--one-way"}, {"contacts", "a", "b", "c", "d"},
	} {
		if code, _, _ := runGM(env, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	if len(app.reqs) != 0 {
		t.Errorf("a usage error reached the app: %d requests", len(app.reqs))
	}
}

func TestOutboxTableAndSince(t *testing.T) {
	raw := fixtureField(t, "outbox.json", "answer")
	app := newFakeApp(t, serve(raw, 200))
	env := app.env(t, withToken())
	code, out, errOut := runGM(env, "outbox")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"ID", "gm-001", "delivered", "queued", "left", "failed", "Rev", "Please look at the login", "waiting for an answer"} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	if r := app.reqs[0]; r.Method != "GET" || r.Path != "/setup/gm/outbox" {
		t.Errorf("asked %s %s", r.Method, r.Path)
	}
	// --since is a time: only later messages, in the table and in --json.
	code, out, _ = runGM(env, "outbox", "--since", "1761000100000")
	if code != 0 || strings.Contains(out, "gm-001") || strings.Contains(out, "gm-002") || !strings.Contains(out, "gm-003") || !strings.Contains(out, "gm-004") {
		t.Errorf("since: %d\n%s", code, out)
	}
	_, out, _ = runGM(env, "outbox", "--since", "1761000100000", "--json")
	var j struct{ Messages []map[string]any }
	if err := json.Unmarshal([]byte(out), &j); err != nil || len(j.Messages) != 3 || j.Messages[0]["id"] != "gm-003" {
		t.Errorf("json since: %v %q", err, out)
	}
	code, out, _ = runGM(env, "outbox", "--since", "9999999999999")
	if code != 0 || !strings.Contains(out, "No messages.") {
		t.Errorf("none: %d %q", code, out)
	}
}

func TestOutboxShowsAHiresMessages(t *testing.T) {
	app := newFakeApp(t, serve([]byte(`{"messages":[{"id":"h-001","from":"hire","fromID":"abc","fromName":"Rev","to":"gm","toName":"the GM","at":1761000240000,"state":"delivered","size":12,"attempts":0,"text":"Review done."}]}`), 200))
	_, out, _ := runGM(app.env(t, withToken()), "outbox")
	if !strings.Contains(out, "Rev") || !strings.Contains(out, "the GM") || !strings.Contains(out, "Review done.") {
		t.Errorf("out = %q", out)
	}
}

func TestWatchSetsAndUnwatches(t *testing.T) {
	var sent []map[string]any
	answer := fixtureField(t, "watch.json", "answer-to-file")
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup/gm/watch":
			sent = append(sent, body(t, r))
			serve(answer, 200)(w, r)
		case "/setup/gm/unwatch":
			serve(fixtureField(t, "watch.json", "unwatch-answer"), 200)(w, r)
		}
	})
	env := app.env(t, withToken())
	code, out, errOut := runGM(env, "watch", "file", "/r/STATUS.md", "--pattern", `\bDONE\b`, "--repeat")
	if code != 0 || !strings.Contains(out, "Watch w-0001 set") {
		t.Fatalf("exit %d: %s %s", code, out, errOut)
	}
	if s := sent[0]; s["kind"] != "file" || s["target"] != "/r/STATUS.md" || s["pattern"] != `\bDONE\b` || s["repeat"] != true {
		t.Errorf("sent %v", s)
	}
	runGM(env, "watch", "idle", "Rev", "--minutes", "10")
	if s := sent[1]; s["kind"] != "idle" || s["minutes"] != float64(10) || s["pattern"] != nil || s["repeat"] != nil {
		t.Errorf("sent %v", s)
	}
	code, out, _ = runGM(env, "unwatch", "w-0001")
	if code != 0 || !strings.Contains(out, "w-0001 taken away") {
		t.Errorf("unwatch: %d %q", code, out)
	}
	last := app.reqs[len(app.reqs)-1]
	if last.Method != "POST" || last.RawQuery != "id=w-0001" || last.Header.Get("X-AgentDeck-Setup") != "1" {
		t.Errorf("unwatch request = %+v", last)
	}
}

func TestWatchRefusal(t *testing.T) {
	app := newFakeApp(t, serve([]byte(`{"error":"That file is a secrets file."}`), 403))
	code, _, errOut := runGM(app.env(t, withToken()), "watch", "file", "/r/.env", "--pattern", "x")
	if code != 1 || !strings.Contains(errOut, "secrets file") {
		t.Errorf("code %d %q", code, errOut)
	}
}

func TestWatchesHidesBuiltinsUnlessAll(t *testing.T) {
	list := `{"watches":[` +
		`{"id":"w-0001","kind":"file","target":"/r/STATUS.md","pattern":"DONE","repeat":false,"by":"gm","createdAt":1,"fired":2,"active":true},` +
		`{"id":"w-0002","kind":"time","target":"check","pattern":"","repeat":true,"by":"owner","createdAt":1,"fired":0,"active":true,"minutes":30,"due":1761001800000},` +
		`{"id":"w-0003","kind":"idle","target":"abc","pattern":"","repeat":true,"by":"app","createdAt":1,"fired":0,"active":true,"minutes":15,"builtin":"wait","hire":"abc"}]}`
	app := newFakeApp(t, serve([]byte(list), 200))
	env := app.env(t, withToken())
	code, out, _ := runGM(env, "watches")
	if code != 0 || !strings.Contains(out, "w-0001") || !strings.Contains(out, "w-0002") || strings.Contains(out, "w-0003") || !strings.Contains(out, "due ") {
		t.Errorf("default: %d\n%s", code, out)
	}
	_, out, _ = runGM(env, "watches", "--all")
	if !strings.Contains(out, "w-0003") || !strings.Contains(out, "app: wait") {
		t.Errorf("all:\n%s", out)
	}
	_, out, _ = runGM(env, "watches", "--json")
	if strings.Contains(out, "w-0003") || !strings.Contains(out, "w-0001") {
		t.Errorf("json hides built-ins too:\n%s", out)
	}
	_, out, _ = runGM(env, "watches", "--all", "--json")
	if !strings.Contains(out, "w-0003") {
		t.Errorf("json --all:\n%s", out)
	}
	// The shared fixture's two watches, no built-ins.
	app = newFakeApp(t, serve(fixtureField(t, "watches.json", "answer"), 200))
	if code, out, _ = runGM(app.env(t, withToken()), "watches"); code != 0 || !strings.Contains(out, "check the build") {
		t.Errorf("fixture: %d %q", code, out)
	}
	app = newFakeApp(t, serve([]byte(`{"watches":[]}`), 200))
	if _, out, _ = runGM(app.env(t, withToken()), "watches"); !strings.Contains(out, "No watches.") {
		t.Errorf("empty: %q", out)
	}
}

func TestContactsAllowAndRemove(t *testing.T) {
	steps := struct {
		Steps []struct {
			Request json.RawMessage `json:"request"`
			Status  int             `json:"status"`
			Answer  json.RawMessage `json:"answer"`
		} `json:"steps"`
	}{}
	if err := json.Unmarshal(fixture(t, "contacts.json"), &steps); err != nil {
		t.Fatal(err)
	}
	var sent []map[string]any
	app := newFakeApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/setup/gm/team" {
			serve(teamAnswer(t), 200)(w, r)
			return
		}
		sent = append(sent, body(t, r))
		serve(steps.Steps[0].Answer, 200)(w, r)
	})
	env := app.env(t, withToken())
	code, out, errOut := runGM(env, "contacts", "allow", "Rev", "Janitor")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	// The ids in the fixture's answer are not in team.json's table, so they show as ids.
	if !strings.Contains(out, "a1b2c3 -> d4e5f6") && !strings.Contains(out, "Rev -> Janitor") {
		t.Errorf("out = %q", out)
	}
	if s := sent[0]; s["from"] != "Rev" || s["to"] != "Janitor" || s["oneWay"] != nil || s["remove"] != nil {
		t.Errorf("allow sent %v", s)
	}
	runGM(env, "contacts", "remove", "--one-way", "a1b2c3", "d4e5f6")
	if s := sent[1]; s["oneWay"] != true || s["remove"] != true || s["from"] != "a1b2c3" {
		t.Errorf("remove sent %v", s)
	}
	if r := app.reqs[0]; r.Method != "POST" || r.Path != "/setup/gm/contacts" || r.Header.Get("X-AgentDeck-Setup") != "1" {
		t.Errorf("request = %+v", r)
	}
}

func TestContactsListedWithTheTeam(t *testing.T) {
	answer := `{"at":1,"contacts":[{"from":"a1b2c3","to":"d4e5f6"},{"from":"d4e5f6","to":"a1b2c3"}],` +
		`"team":[{"id":"a1b2c3","name":"Reviewer","nickname":"Rev"},{"id":"d4e5f6","name":"Janitor"}]}`
	app := newFakeApp(t, serve([]byte(answer), 200))
	env := app.env(t, withToken())
	code, out, _ := runGM(env, "contacts")
	if code != 0 || !strings.Contains(out, "Rev -> Janitor") || !strings.Contains(out, "Janitor -> Rev") {
		t.Errorf("contacts: %d %q", code, out)
	}
	if r := app.reqs[0]; r.Method != "GET" || r.Path != "/setup/gm/team" {
		t.Errorf("asked %s %s", r.Method, r.Path)
	}
	// And in gm team's table.
	_, out, _ = runGM(env, "team")
	if !strings.Contains(out, "CONTACTS") || !strings.Contains(out, "Rev -> Janitor") {
		t.Errorf("team lacks the contacts:\n%s", out)
	}
	// None: the team fixture has an empty list, so no section, and contacts says so.
	app = newFakeApp(t, serve(teamAnswer(t), 200))
	env = app.env(t, withToken())
	if _, out, _ = runGM(env, "team"); strings.Contains(out, "CONTACTS") {
		t.Errorf("empty contacts shown:\n%s", out)
	}
	if _, out, _ = runGM(env, "contacts"); !strings.Contains(out, "only to the GM") {
		t.Errorf("none: %q", out)
	}
}

func TestMailRoutesAreGMRoutesInTheFixture(t *testing.T) {
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
		{"POST", "/setup/gm/send"}, {"GET", "/setup/gm/outbox"}, {"GET", "/setup/gm/watches"},
		{"POST", "/setup/gm/watch"}, {"POST", "/setup/gm/unwatch"},
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
