package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

// A registry the Mac wrote: the fixture's hires, with the pairs and the keys' hashes it keeps beside them.
func macWrittenRegistry(t *testing.T) []byte {
	t.Helper()
	var file map[string]any
	if err := json.Unmarshal(frontDeskFixture(t, "registry.json"), &file); err != nil {
		t.Fatal(err)
	}
	file["contacts"] = []map[string]string{{"from": "a1b2c3", "to": "d4e5f6"}, {"from": "d4e5f6", "to": "a1b2c3"}}
	file["keys"] = map[string]string{"a1b2c3": hashKey("fake-key-rev"), "d4e5f6": hashKey("fake-key-jan"), "0a0b0c": hashKey("fake-key-bob")}
	data, _ := json.MarshalIndent(file, "", "  ")
	return data
}

func TestARegistryTheMacWroteKeepsItsKeysAndContactsThroughAnEdit(t *testing.T) {
	newTestDeck(t)
	original := macWrittenRegistry(t)
	if err := os.WriteFile(registryPath(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","notes":"same"}`)); refused != nil {
		t.Fatal(refused.message)
	}
	var before, after map[string]any
	now, _ := os.ReadFile(registryPath())
	json.Unmarshal(original, &before)
	json.Unmarshal(now, &after)
	before["hires"].([]any)[0].(map[string]any)["notes"] = "same"
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("the edit changed more than the notes:\n%s", now)
	}
	file := readRegistry(registryPath(), false)
	if len(file.contacts) != 2 || len(file.keys) != 3 || !file.mayWrite("a1b2c3", "d4e5f6") || file.mayWrite("a1b2c3", "0a0b0c") {
		t.Errorf("the pairs and keys read back as %v %v", file.contacts, file.keys)
	}
}

func TestAnEmptyRegistryLeavesKeysAndContactsOut(t *testing.T) {
	newTestDeck(t)
	if err := os.WriteFile(registryPath(), frontDeskFixture(t, "registry.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, refused := editHire(registryPath(), []byte(`{"id":"a1b2c3","notes":"x"}`)); refused != nil {
		t.Fatal(refused.message)
	}
	if data, _ := os.ReadFile(registryPath()); strings.Contains(string(data), `"keys"`) || strings.Contains(string(data), `"contacts"`) {
		t.Errorf("empty keys or contacts were written: %s", data)
	}
	if got := asJSONValue(t, readRegistry(registryPath(), false).contactsJSON()); !reflect.DeepEqual(got, []any{}) {
		t.Errorf("no pairs should read as [], got %v", got)
	}
}

func TestAKeyOrContactsThatCantBeReadSetTheRegistryAside(t *testing.T) {
	newTestDeck(t)
	for _, broken := range []string{`"contacts": "x"`, `"keys": [1]`, `"contacts": [{"from": 1}]`} {
		bad := `{"version": 1, "hires": [], ` + broken + `}`
		if err := os.WriteFile(registryPath(), []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := readRegistry(registryPath(), false); got.hires != nil || got.keys != nil {
			t.Errorf("%s read as %v", broken, got)
		}
		readRegistry(registryPath(), true)
		if _, err := os.Stat(registryPath()); err == nil {
			t.Errorf("%s: the unreadable file was left to be written over", broken)
		}
	}
}

func TestOnlyTheHashOfASendKeyIsKeptAndAGMHireGetsNone(t *testing.T) {
	newTestDeck(t)
	data := macWrittenRegistry(t)
	var file map[string]any
	json.Unmarshal(data, &file)
	hires := file["hires"].([]any)
	gmHire := map[string]any{"id": "9z9z9z", "name": "Boss", "folder": "/work/gm", "role": "gm", "state": "working"}
	file["hires"] = append(hires, gmHire)
	data, _ = json.Marshal(file)
	os.WriteFile(registryPath(), data, 0o600)

	key, err := newSendKey(registryPath(), "d4e5f6")
	if err != nil || len(key) < 32 {
		t.Fatalf("newSendKey: %q, %v", key, err)
	}
	saved, _ := os.ReadFile(registryPath())
	if strings.Contains(string(saved), key) || !strings.Contains(string(saved), hashKey(key)) {
		t.Error("the key is in the file, or its hash is not")
	}
	if got := readRegistry(registryPath(), false).hireForSendKey(key); got == nil || got.ID != "d4e5f6" {
		t.Errorf("the key finds %v", got)
	}
	// Asking again gives a new key and the old stops working.
	again, _ := newSendKey(registryPath(), "d4e5f6")
	if again == key || readRegistry(registryPath(), false).hireForSendKey(key) != nil {
		t.Error("the old key still works, or the new one is the same")
	}
	// The GM signs with its token: no key, nothing stored.
	if got, err := newSendKey(registryPath(), "9z9z9z"); got != "" || err != nil {
		t.Errorf("a GM-role hire got %q, %v", got, err)
	}
	if kept := readRegistry(registryPath(), false).keys; kept["9z9z9z"] != "" {
		t.Error("a GM-role hire has a key stored")
	}
	if _, err := newSendKey(registryPath(), "nobody"); err == nil {
		t.Error("a key was made for a hire that isn't there")
	}
}

func TestAFiredOrFailedHiresKeyIsRefused(t *testing.T) {
	newTestDeck(t)
	os.WriteFile(registryPath(), macWrittenRegistry(t), 0o600)
	file := readRegistry(registryPath(), false)
	if h := file.hireForSendKey("fake-key-rev"); h == nil || h.ID != "a1b2c3" {
		t.Fatalf("an open hire's key finds %v", h)
	}
	if file.hireForSendKey("fake-key-bob") != nil {
		t.Error("a fired hire's key was accepted")
	}
	for _, state := range []string{"fired", "failed"} {
		if _, refused := editHire(registryPath(), []byte(`{"id":"d4e5f6","state":"`+state+`"}`)); refused != nil {
			t.Fatal(refused.message)
		}
		if isHireKey(registryPath(), "fake-key-jan") {
			t.Errorf("a %s hire's key was accepted", state)
		}
	}
	for _, key := range []string{"", "fake-key", hashKey("fake-key-rev")} {
		if isHireKey(registryPath(), key) {
			t.Errorf("%q was taken for a hire's key (the hash must not be one)", key)
		}
	}
}

// Both secrets are compared with subtle, never with ==.
func TestTheGMTokenAndHireKeysAreComparedInConstantTime(t *testing.T) {
	src, err := os.ReadFile("access.go")
	if err != nil || !strings.Contains(string(src), "subtle.ConstantTimeCompare") {
		t.Fatal("constantTimeEqual should be subtle.ConstantTimeCompare")
	}
	mail, _ := os.ReadFile("frontdeskmail.go")
	for _, line := range strings.Split(string(mail), "\n") {
		if strings.Contains(line, "gmToken ==") || strings.Contains(line, "== d.gmToken") || strings.Contains(line, "kept ==") || strings.Contains(line, "== kept") {
			t.Errorf("a secret is compared with ==: %s", line)
		}
	}
	d := newTestDeck(t)
	token := d.newGMToken()
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.isGMToken(token) || d.isGMToken(token+"x") || d.isGMToken(token[:len(token)-1]) || d.isGMToken("") {
		t.Error("isGMToken")
	}
}

func TestTheContactsRouteAnswersAsContactsJSONSays(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	os.WriteFile(registryPath(), frontDeskFixture(t, "registry.json"), 0o600)
	var fixture struct {
		Callers []string
		Steps   []struct {
			Request any
			Status  int
			Answer  any
		}
	}
	if err := json.Unmarshal(frontDeskFixture(t, "contacts.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	gm := d.newGMToken()
	post := func(c call, body string) int {
		c.method, c.target, c.host, c.from, c.body = "POST", "/setup/gm/contacts", "localhost:7710", thisComputer, body
		c.headers = mergeHeaders(c.headers, map[string]string{setupHeader: "1"})
		return serve(t, d, c).Code
	}
	for i, step := range fixture.Steps {
		body, _ := json.Marshal(step.Request)
		w := serve(t, d, call{method: "POST", target: "/setup/gm/contacts", host: "localhost:7710", from: thisComputer, body: string(body),
			withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}})
		if w.Code != step.Status {
			t.Fatalf("step %d %s: %d %s, want %d", i, body, w.Code, w.Body, step.Status)
		}
		if step.Answer != nil {
			var got any
			if json.Unmarshal(w.Body.Bytes(), &got) != nil || !reflect.DeepEqual(got, step.Answer) {
				t.Errorf("step %d %s: answered %s, want %v", i, body, w.Body, step.Answer)
			}
		}
	}
	// What the file keeps after the last step: one pair, from Rev to Janitor.
	if file := readRegistry(registryPath(), false); !reflect.DeepEqual(file.contacts, []Contact{{"a1b2c3", "d4e5f6"}}) {
		t.Errorf("the file keeps %v", file.contacts)
	}
	// Only the GM and the page: a tool with the key and a hire are refused, nothing changes.
	if got := post(call{asTool: true}, `{"from":"Rev","to":"Janitor"}`); got != http.StatusForbidden {
		t.Errorf("a tool: %d, want 403", got)
	}
	if got := post(call{withoutSetupKey: true, headers: map[string]string{hireKeyHeader: "x"}}, `{"from":"Rev","to":"Janitor"}`); got != http.StatusUnauthorized {
		t.Errorf("a made-up hire key: %d, want 401", got)
	}
	if got := post(call{}, `{"from":"Rev","to":"Janitor","remove":true}`); got != http.StatusOK {
		t.Errorf("the page: %d, want 200", got)
	}
}

func TestContactsRefusalsAndHowANameIsFound(t *testing.T) {
	newTestDeck(t)
	var file map[string]any
	json.Unmarshal(frontDeskFixture(t, "registry.json"), &file)
	file["hires"] = append(file["hires"].([]any),
		map[string]any{"id": "g1", "name": "Boss", "folder": "/w", "role": "gm", "state": "working"},
		map[string]any{"id": "b2", "name": "Builder", "nickname": "", "folder": "/w2", "role": "builder", "state": "idle"},
		map[string]any{"id": "f1", "name": "Failed One", "folder": "/w3", "role": "builder", "state": "failed"},
		map[string]any{"id": "t1", "name": "Twin", "folder": "/w4", "role": "builder", "state": "idle"},
		map[string]any{"id": "t2", "name": "twin", "folder": "/w5", "role": "builder", "state": "working"})
	data, _ := json.Marshal(file)
	os.WriteFile(registryPath(), data, 0o600)
	for _, c := range []struct {
		body   string
		status int
	}{
		{`not json`, 400}, {`[]`, 400}, {`{"from":"Rev"}`, 400}, {`{"from":"Rev","to":null}`, 400}, {`{"from":1,"to":"Janitor"}`, 400},
		{`{"from":"Rev","to":"Janitor","oneWay":"yes"}`, 400}, {`{"from":"Rev","to":"Janitor","remove":null}`, 400},
		{`{"from":"Rev","to":"a1b2c3"}`, 400}, // the same hire, by name and by id
		{`{"from":"Rev","to":"Boss"}`, 400},   // either side the GM
		{`{"from":"g1","to":"Janitor"}`, 400},
		{`{"from":"Rev","to":"nobody"}`, 404}, {`{"nobody":1,"from":"x","to":"Rev"}`, 404},
		{`{"from":"Rev","to":"Bob"}`, 409},        // fired
		{`{"from":"Failed One","to":"Rev"}`, 409}, // failed
		{`{"from":"Rev","to":"Twin"}`, 200},       // two share a name, but one is open? both open: see below
	} {
		_, refused := setContacts(registryPath(), []byte(c.body))
		got := 200
		if refused != nil {
			got = refused.status
		}
		if c.body == `{"from":"Rev","to":"Twin"}` {
			if got != 409 {
				t.Errorf("%s: %d, want 409 (two open hires are called that)", c.body, got)
			}
			continue
		}
		if got != c.status {
			t.Errorf("%s: %d, want %d", c.body, got, c.status)
		}
	}
	// A fired hire's name may be used again once an open hire has it; removing a pair with a gone hire is allowed.
	if h, refused := findHire("  BOB ", readRegistry(registryPath(), false).hires); refused != nil || h.ID != "0a0b0c" {
		t.Errorf("a fired hire alone by its nickname, any case, spaces trimmed: %v %v", h, refused)
	}
	if h, refused := findHire("builder", readRegistry(registryPath(), false).hires); refused != nil || h.ID != "b2" {
		t.Errorf("the open Builder is preferred to the fired one: %v %v", h, refused)
	}
	if _, refused := setContacts(registryPath(), []byte(`{"from":"Bob","to":"Rev","remove":true}`)); refused != nil {
		t.Errorf("removing a pair with a fired hire: %d", refused.status)
	}
	if len(readRegistry(registryPath(), false).contacts) != 0 {
		t.Error("nothing should have been added by the refusals")
	}
}

func TestMayWriteIsTheExactPair(t *testing.T) {
	f := registryFile{contacts: []Contact{{"a", "b"}}}
	if !f.mayWrite("a", "b") || f.mayWrite("b", "a") || f.mayWrite("a", "c") || f.mayWrite("", "") {
		t.Error("mayWrite should allow exactly the pair")
	}
}

// A GM-row or page-only route the table doesn't give a tool stays out of its reach; a route not in it keeps 1.23's rule.
func TestATooIsNotLetInToTheGMsRoutes(t *testing.T) {
	if routeAllows(callerTool, "POST", "/setup/gm/hire") || routeAllows(callerTool, "GET", "/setup/gm/something-new") ||
		routeAllows(callerHire, "GET", "/setup/theme") || routeAllows(callerGM, "GET", "/setup/theme") {
		t.Error("a caller was let in where the table keeps it out")
	}
}

func TestGMHireRefusalRefusesOnlyTheGMAskingForTheGMRole(t *testing.T) {
	if status, text, refused := gmHireRefusal("gm", callerGM, false); !refused || status != http.StatusConflict || text != "Only the owner hires the General Manager, from Setup." {
		t.Errorf("no GM yet: %d %q %v", status, text, refused)
	}
	if status, text, refused := gmHireRefusal("gm", callerGM, true); !refused || status != http.StatusConflict || text != "This office already has a General Manager. The owner can replace it from Setup." {
		t.Errorf("a GM hired: %d %q %v", status, text, refused)
	}
	for _, who := range []caller{callerPage, callerTool, callerHire, callerNobody} {
		if _, _, refused := gmHireRefusal("gm", who, true); refused {
			t.Errorf("caller %v was refused by the GM check", who)
		}
	}
	if _, _, refused := gmHireRefusal("reviewer", callerGM, true); refused {
		t.Error("the GM was refused a role that isn't the GM")
	}
}

func TestTheGMTokenDoesNotHireOrReplaceAGeneralManager(t *testing.T) {
	registry := func(state string) string {
		if state == "" {
			return `{"version":1,"hires":[{"id":"aaa111","name":"Rev","folder":"/w","role":"reviewer","state":"working"}]}`
		}
		return `{"version":1,"hires":[{"id":"aaa111","name":"Rev","folder":"/w","role":"reviewer","state":"working"},{"id":"bbb222","name":"GM","folder":"/w","role":"gm","state":"` + state + `"}]}`
	}
	cases := []struct {
		name, state, text string
	}{
		{"no GM", "", "Only the owner hires the General Manager, from Setup."},
		{"an open GM", "working", "This office already has a General Manager. The owner can replace it from Setup."},
		{"a fired GM counts as none", "fired", "Only the owner hires the General Manager, from Setup."},
		{"a failed GM counts as none", "failed", "Only the owner hires the General Manager, from Setup."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, h, _ := hiringDeck(t, "")
			if err := os.WriteFile(registryPath(), []byte(registry(c.state)), 0o600); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(registryPath())
			gm := d.newGMToken()
			w := serve(t, d, call{method: "POST", target: "/setup/hire/gm", host: "localhost:7710", from: thisComputer,
				withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}})
			if w.Code != http.StatusConflict || strings.TrimSpace(w.Body.String()) != c.text {
				t.Fatalf("the GM's token: %d %q, want 409 %q", w.Code, w.Body, c.text)
			}
			after, _ := os.ReadFile(registryPath())
			if string(before) != string(after) || len(h.windows) != 0 || len(h.tmux) != 0 || len(d.hires) != 0 {
				t.Errorf("something was hired: registry %s, %d windows, %d tmux, %d hires", after, len(h.windows), len(h.tmux), len(d.hires))
			}
		})
	}
}

func TestThePageIsNotRefusedTheGMHireAndThePageAloneStartsOne(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	w := serve(t, d, call{method: "POST", target: "/setup/hire/gm", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusNoContent || len(h.windows) != 1 {
		t.Fatalf("the page hiring the GM: %d %q, %d windows, want 204 and one window", w.Code, w.Body, len(h.windows))
	}
}

func TestAGMOrHireKeyDoesNotCompleteSetup(t *testing.T) {
	for _, who := range []string{"gm", "hire"} {
		d := newTestDeck(t)
		d.config.HasCompletedSetup = false
		headers := map[string]string{}
		if who == "gm" {
			headers[gmTokenHeader] = d.newGMToken()
		} else {
			os.WriteFile(registryPath(), []byte(`{"version":1,"hires":[{"id":"aaa111","name":"Rev","folder":"/w","role":"reviewer","state":"working"}]}`), 0o600)
			key, err := newSendKey(registryPath(), "aaa111")
			if err != nil {
				t.Fatal(err)
			}
			headers[hireKeyHeader] = key
		}
		if w := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer, withoutSetupKey: true, headers: headers}); w.Code != http.StatusOK {
			t.Fatalf("%s: GET /setup %d", who, w.Code)
		}
		if d.config.HasCompletedSetup {
			t.Errorf("a %s caller marked setup as done", who)
		}
	}
	d := newTestDeck(t)
	d.config.HasCompletedSetup = false
	d.mu.Lock()
	cookie := d.access.NewSetupSession()
	d.mu.Unlock()
	serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{"Cookie": setupCookie() + "=" + cookie}})
	if !d.config.HasCompletedSetup {
		t.Error("the page no longer completes setup")
	}
}

// A request is one caller, in a fixed order: a valid setup key wins over the
// GM's token; a wrong one of the two never changes what the other gets alone.
func TestARequestWithBothTheSetupKeyAndTheGMTokenIsTheTool(t *testing.T) {
	d := newTestDeck(t)
	token := d.newGMToken()
	who := func(key, gm string) caller {
		r := httptest.NewRequest("GET", "/setup/state", nil)
		if key != "" {
			r.Header.Set(setupKeyHeader, key)
		}
		if gm != "" {
			r.Header.Set(gmTokenHeader, gm)
		}
		return d.callerOf(r)
	}
	d.mu.Lock()
	key := d.access.SetupKey
	d.mu.Unlock()
	for _, c := range []struct {
		name    string
		key, gm string
		want    caller
	}{
		{"both valid", key, token, callerTool},
		{"key, wrong token", key, "wrong", callerTool},
		{"wrong key, token", "wrong", token, callerGM},
		{"token alone", "", token, callerGM},
		{"both wrong", "wrong", "wrong", callerNobody},
	} {
		if got := who(c.key, c.gm); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupKeyHeader: key, gmTokenHeader: token}})
	if w.Code != http.StatusOK {
		t.Errorf("/setup/state with both: %d %s", w.Code, w.Body)
	}
}

// For a route a tool may not use, a valid setup key beside the GM's token is the GM; a wrong token is the key alone.
func TestAKeyAndATokenTogetherAreTheGMForAGMActionAndTheToolForARead(t *testing.T) {
	d := newTestDeck(t)
	token := d.newGMToken()
	d.mu.Lock()
	key := d.access.SetupKey
	d.mu.Unlock()
	whoIs := func(method, path, gm string) caller {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set(setupKeyHeader, key)
		r.Header.Set(gmTokenHeader, gm)
		return d.routeCallerOf(r, method, path)
	}
	for _, path := range []string{"/setup/gm/send", "/setup/gm/hire"} {
		if got := whoIs("POST", path, token); got != callerGM {
			t.Errorf("%s with both: %v, want the GM", path, got)
		}
		if got := whoIs("POST", path, "wrong"); got != callerTool {
			t.Errorf("%s with the key and a wrong token: %v, want the tool", path, got)
		}
		both := serve(t, d, call{method: "POST", target: path, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupKeyHeader: key, gmTokenHeader: token}})
		keyOnly := serve(t, d, call{method: "POST", target: path, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupKeyHeader: key, gmTokenHeader: "wrong"}})
		alone := serve(t, d, call{method: "POST", target: path, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupKeyHeader: key}})
		if both.Code == http.StatusForbidden {
			t.Errorf("%s with both is refused: %s", path, both.Body)
		}
		if keyOnly.Code != http.StatusForbidden || keyOnly.Body.String() != alone.Body.String() || alone.Code != http.StatusForbidden {
			t.Errorf("%s: key and wrong token %d %q, key alone %d %q: want the same refusal", path, keyOnly.Code, keyOnly.Body, alone.Code, alone.Body)
		}
	}
	if got := whoIs("GET", "/setup/state", token); got != callerTool {
		t.Errorf("/setup/state with both: %v, want the tool", got)
	}
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupKeyHeader: key, gmTokenHeader: token}})
	if w.Code != http.StatusOK {
		t.Errorf("/setup/state with both: %d %s", w.Code, w.Body)
	}
}

// A wrong hire key carries no weight, as on the Mac: beside the GM's token it is the GM,
// and beside the setup key and the token on a GM action it is the GM too.
func TestAWrongHireKeyChangesNothing(t *testing.T) {
	d := newTestDeck(t)
	token := d.newGMToken()
	d.mu.Lock()
	key := d.access.SetupKey
	d.mu.Unlock()
	req := func(method, path, setupKey string) *http.Request {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set(hireKeyHeader, "not-a-hire-key")
		r.Header.Set(gmTokenHeader, token)
		if setupKey != "" {
			r.Header.Set(setupKeyHeader, setupKey)
		}
		return r
	}
	if got := d.callerOf(req("GET", "/setup/gm/team", "")); got != callerGM {
		t.Errorf("a wrong hire key and the token: %v, want the GM", got)
	}
	if got := d.routeCallerOf(req("POST", "/setup/gm/send", key), "POST", "/setup/gm/send"); got != callerGM {
		t.Errorf("a wrong hire key, the key and the token on /setup/gm/send: %v, want the GM", got)
	}
}
