package desktop

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// mailDeck is a deck with the registry fixture's three hires, a General Manager, a failed hire and two hires of one name,
// the manager switched on, and the GM's token.
type mailDeck struct {
	d      *Deck
	gm     string
	keys   map[string]string // each open hire's send key, by id
	nobody string
}

func newMailDeck(t *testing.T) *mailDeck {
	t.Helper()
	d := newTestDeck(t)
	managerOn(d)
	var file map[string]any
	json.Unmarshal(frontDeskFixture(t, "registry.json"), &file)
	file["hires"] = append(file["hires"].([]any),
		map[string]any{"id": "g1", "name": "Boss", "folder": "/w", "role": "gm", "state": "working"},
		map[string]any{"id": "f1", "name": "Failed One", "folder": "/w3", "role": "builder", "state": "failed"},
		map[string]any{"id": "t1", "name": "Twin", "folder": "/w4", "role": "builder", "state": "idle"},
		map[string]any{"id": "t2", "name": "twin", "folder": "/w5", "role": "builder", "state": "working"})
	data, _ := json.Marshal(file)
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	m := &mailDeck{d: d, gm: d.newGMToken(), keys: map[string]string{}}
	for _, id := range []string{"a1b2c3", "d4e5f6", "t1"} {
		key, err := newSendKey(registryPath(), id)
		if err != nil || key == "" {
			t.Fatalf("send key for %s: %q %v", id, key, err)
		}
		m.keys[id] = key
	}
	return m
}

type who int

const (
	asGM who = iota
	asPage
	asTool
	asRev
	asJanitor
	asTwin
)

func (m *mailDeck) call(t *testing.T, as who, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	c := call{method: method, target: target, host: "localhost:7710", from: thisComputer, body: body,
		headers: map[string]string{setupHeader: "1"}}
	switch as {
	case asGM:
		c.withoutSetupKey, c.headers[gmTokenHeader] = true, m.gm
	case asTool:
		c.asTool = true
	case asRev:
		c.withoutSetupKey, c.headers[hireKeyHeader] = true, m.keys["a1b2c3"]
	case asJanitor:
		c.withoutSetupKey, c.headers[hireKeyHeader] = true, m.keys["d4e5f6"]
	case asTwin:
		c.withoutSetupKey, c.headers[hireKeyHeader] = true, m.keys["t1"]
	}
	return serve(t, m.d, c)
}

func (m *mailDeck) send(t *testing.T, as who, body string) *httptest.ResponseRecorder {
	t.Helper()
	return m.call(t, as, "POST", "/setup/gm/send", body)
}

func decoded(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("not an object: %d %s", w.Code, w.Body)
	}
	return out
}

func (m *mailDeck) outbox(t *testing.T, as who, query string) []map[string]any {
	t.Helper()
	w := m.call(t, as, "GET", "/setup/gm/outbox"+query, "")
	if w.Code != http.StatusOK {
		t.Fatalf("outbox: %d %s", w.Code, w.Body)
	}
	var rows []map[string]any
	for _, row := range decoded(t, w)["messages"].([]any) {
		rows = append(rows, row.(map[string]any))
	}
	return rows
}

func (m *mailDeck) limits(maxBytes, perHour int) {
	m.d.mu.Lock()
	defer m.d.mu.Unlock()
	if maxBytes > 0 {
		m.d.config.MailMaxBytes = maxBytes
	}
	if perHour > 0 {
		m.d.config.MailPerHour = perHour
	}
}

// The GM's send, as send.json answers it. Nothing is typed in this change, so a message to a hire stays queued
// (the fixture's "delivered" is what the delivery will make of it).
func TestTheGMSendsAsSendJSONSays(t *testing.T) {
	m := newMailDeck(t)
	fixture := sendFixture(t)
	body, _ := json.Marshal(fixture["request"])
	w := m.send(t, asGM, string(body))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	want := fixture["answer"].(map[string]any)
	want["state"], want["to"] = "queued", "a1b2c3" // the fixture's hire id is made up
	if got := decoded(t, w); !reflect.DeepEqual(got, want) {
		t.Errorf("answered %v, want %v", got, want)
	}
	// The same id again: the fixture's answer with duplicate, and nothing added.
	again := fixture["same-id-again"].(map[string]any)
	again["state"], again["to"] = "queued", "a1b2c3"
	if got := decoded(t, m.send(t, asGM, string(body))); !reflect.DeepEqual(got, again) {
		t.Errorf("sent again, answered %v, want %v", got, again)
	}
	if rows := m.outbox(t, asGM, ""); len(rows) != 1 || rows[0]["state"] != "queued" || rows[0]["attempts"] != float64(0) {
		t.Errorf("the outbox holds %v", rows)
	}
}

func TestTheOwnerSendsFromThePageByNameNicknameOrID(t *testing.T) {
	m := newMailDeck(t)
	for i, to := range []string{"a1b2c3", "Rev", "reviewer", " REV "} {
		w := m.send(t, asPage, `{"to":"`+to+`","text":"Hello","id":"p-`+string(rune('a'+i))+`"}`)
		if w.Code != 200 || decoded(t, w)["to"] != "a1b2c3" {
			t.Errorf("to %q: %d %s", to, w.Code, w.Body)
		}
	}
	rows := m.outbox(t, asPage, "")
	if len(rows) != 4 || rows[0]["from"] != "owner" || rows[0]["toName"] != "Rev" {
		t.Errorf("the outbox holds %v", rows)
	}
	// A message without an id gets one, and it is answered.
	if id, _ := decoded(t, m.send(t, asPage, `{"to":"Rev","text":"x"}`))["id"].(string); !strings.HasPrefix(id, "m-") {
		t.Errorf("made id %q", id)
	}
}

func TestSendRefusalsAndTheirTexts(t *testing.T) {
	m := newMailDeck(t)
	const noGM = "Mail is never put into the GM's own session. The GM reads its mail with smolcorp gm inbox."
	const shape = "Send {to, text, id}: who it is for, what to say, and an id of your own so that sending twice delivers once."
	for _, c := range []struct {
		body   string
		status int
		text   string
	}{
		{`{}`, 400, shape}, {`not json`, 400, shape}, {`[]`, 400, shape}, {`{"to":"Rev"}`, 400, shape},
		{`{"to":"Rev","text":"  \n "}`, 400, shape}, {`{"to":" \t ","text":"x"}`, 400, shape},
		{`{"to":1,"text":"x"}`, 400, shape}, {`{"to":"Rev","text":null}`, 400, shape},
		{`{"to":"Rev","text":"x","id":"a b"}`, 400, "The id may use letters, digits, - _ and ., up to 64 of them."},
		{`{"to":"Rev","text":"x","id":5}`, 400, "The id may use letters, digits, - _ and ., up to 64 of them."},
		{`{"to":"Rev","text":"x","id":null}`, 400, "The id may use letters, digits, - _ and ., up to 64 of them."},
		{`{"to":"Rev","text":"x","id":""}`, 400, "The id may use letters, digits, - _ and ., up to 64 of them."},
		{`{"to":"Rev","text":"x","id":"` + strings.Repeat("a", 65) + `"}`, 400, "The id may use letters, digits, - _ and ., up to 64 of them."},
		{`{"to":"gm","text":"x"}`, 403, noGM}, {`{"to":" The GM ","text":"x"}`, 403, noGM}, {`{"to":"g1","text":"x"}`, 403, noGM},
		{`{"to":"Boss","text":"x"}`, 403, noGM},
		{`{"to":"nobody","text":"x"}`, 404, "There's no hire or bot called that. Look at the team for ids and names."},
		{`{"to":"twin","text":"x"}`, 409, "More than one hire is called that. Use the id."},
		{`{"to":"Bob","text":"x"}`, 409, "Builder is already gone."},
		{`{"to":"Failed One","text":"x"}`, 409, "Failed One is already gone."},
	} {
		for _, as := range []who{asGM, asPage} {
			w := m.send(t, as, c.body)
			if w.Code != c.status || w.Body.String() != c.text {
				t.Errorf("%v %.40s: %d %q, want %d %q", as, c.body, w.Code, w.Body, c.status, c.text)
			}
		}
	}
	if rows := m.outbox(t, asGM, ""); len(rows) != 0 {
		t.Errorf("refused mail was kept: %v", rows)
	}
}

func TestSendLimitsAre413And429(t *testing.T) {
	m := newMailDeck(t)
	m.limits(10, 2)
	w := m.send(t, asGM, `{"to":"Rev","text":"12345678901"}`)
	if w.Code != 413 || w.Body.String() != "That message is 11 bytes. The limit is 10 bytes per message (setting mailMaxBytes)." {
		t.Errorf("%d %q", w.Code, w.Body)
	}
	for _, id := range []string{"a", "b"} {
		if w := m.send(t, asGM, `{"to":"Rev","text":"x","id":"`+id+`"}`); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	}
	w = m.send(t, asGM, `{"to":"Rev","text":"x","id":"c"}`)
	if w.Code != 429 || w.Body.String() != "That is 2 messages from the GM in the last hour, the limit (setting mailPerHour). Try again later." {
		t.Errorf("%d %q", w.Code, w.Body)
	}
	// The GM's hour is its own: the owner is not held back by it.
	if w := m.send(t, asPage, `{"to":"Rev","text":"x","id":"d"}`); w.Code != 200 {
		t.Errorf("the owner: %d %s", w.Code, w.Body)
	}
}

// The Go app has no bot mailbox yet: a bot is found as the Mac finds it and told it has none.
func TestABotHasNoMailboxYet(t *testing.T) {
	m := newMailDeck(t)
	m.d.mu.Lock()
	m.d.cloud.bots = []CloudBot{{ID: "grok-1", Name: "Grokky"}, {ID: "dup", Name: "Rev"}, {ID: "dup2", Name: "Same"}, {ID: "dup3", Name: "same"}}
	m.d.mu.Unlock()
	const noBox = " has no mailbox folder. The owner sets one on the setup page."
	for _, c := range []struct {
		to     string
		status int
		text   string
	}{
		{"grok-1", 409, "Grokky" + noBox}, {"Grokky", 409, "Grokky" + noBox}, {"bot:grok-1", 409, "Grokky" + noBox},
		{"bot:Grokky", 404, "There's no hire or bot called that. Look at the team for ids and names."},
		// A hire and a bot of one name, and two bots of one name, are a clash; bot:<id> is the way out.
		{"Rev", 409, "More than one hire or bot is called that. Use the id (bot:<id> for a bot)."},
		{"same", 409, "More than one hire or bot is called that. Use the id (bot:<id> for a bot)."},
		{"bot:dup", 409, "Rev" + noBox},
	} {
		w := m.send(t, asGM, `{"to":"`+c.to+`","text":"x"}`)
		if w.Code != c.status || w.Body.String() != c.text {
			t.Errorf("to %q: %d %q, want %d %q", c.to, w.Code, w.Body, c.status, c.text)
		}
	}
	// A hire never writes to a bot.
	if w := m.send(t, asRev, `{"to":"grok-1","text":"x"}`); w.Code != 403 {
		t.Errorf("a hire to a bot: %d %s", w.Code, w.Body)
	}
	if rows := m.outbox(t, asGM, ""); len(rows) != 0 {
		t.Errorf("mail to a bot was kept: %v", rows)
	}
}

func TestAHireWritesToTheGMAndTheGMIsTold(t *testing.T) {
	m := newMailDeck(t)
	fixture := sendFixture(t)["from-a-hire"].(map[string]any)["to-the-gm"].(map[string]any)
	body, _ := json.Marshal(fixture["request"])
	w := m.send(t, asRev, string(body))
	if w.Code != 200 || !reflect.DeepEqual(decoded(t, w), fixture["answer"]) {
		t.Fatalf("%d %s, want %v", w.Code, w.Body, fixture["answer"])
	}
	// One mail line in the GM's inbox: who wrote (name and id), what, never the key.
	events, _, _ := m.d.events.entries(0, 100)
	if len(events) != 1 || events[0].Kind != "mail" || events[0].Folder != "/work/app" ||
		events[0].Detail != "Rev (hire a1b2c3) wrote to you: Review done." {
		t.Fatalf("events: %+v", events)
	}
	// Sending it again is the same message: no second line.
	if w := m.send(t, asRev, string(body)); w.Code != 200 || decoded(t, w)["duplicate"] != true {
		t.Errorf("again: %d %s", w.Code, w.Body)
	}
	if events, _, _ := m.d.events.entries(0, 100); len(events) != 1 {
		t.Errorf("a duplicate made %d lines", len(events))
	}
	// Other names for the GM, and the GM-role hire itself, all keep the mail for the GM.
	for i, to := range []string{"GM", "the gm", "Boss", "g1"} {
		w := m.send(t, asRev, `{"to":"`+to+`","text":"line one\nline\u0007two","id":"k`+string(rune('a'+i))+`"}`)
		if w.Code != 200 || decoded(t, w)["to"] != "gm" {
			t.Errorf("to %q: %d %s", to, w.Code, w.Body)
		}
	}
	events, _, _ = m.d.events.entries(0, 100)
	if len(events) != 5 || events[1].Detail != "Rev (hire a1b2c3) wrote to you: line one line two" {
		t.Errorf("events: %+v", events)
	}
	for _, e := range events {
		if strings.Contains(e.Detail, m.keys["a1b2c3"]) {
			t.Error("the hire's key is in an event")
		}
	}
	// The mail is in the outbox for the GM and the page, to read, with the Mac's fields.
	rows := m.outbox(t, asGM, "")
	first := rows[0]
	if first["from"] != "hire" || first["fromID"] != "a1b2c3" || first["fromName"] != "Rev" || first["to"] != "gm" ||
		first["toName"] != "the GM" || first["state"] != "delivered" || first["note"] != "Kept for the GM to read; not typed into its session." || first["text"] != "Review done." {
		t.Errorf("the outbox holds %v", first)
	}
}

func TestAHireWritesToAnotherHireOnlyIfAllowed(t *testing.T) {
	m := newMailDeck(t)
	const rule = "A hire may write to the GM, and to another hire only when the GM or the owner has allowed that pair. Ask the GM to allow it (smolcorp gm contacts)."
	// Not allowed, an unknown name, a fired hire, itself and a bot name all get the one answer.
	for _, to := range []string{"Janitor", "d4e5f6", "nobody", "Bob", "Failed One", "Rev", "a1b2c3", "twin", "bot:x"} {
		if w := m.send(t, asRev, `{"to":"`+to+`","text":"x"}`); w.Code != 403 || w.Body.String() != rule {
			t.Errorf("to %q: %d %q", to, w.Code, w.Body)
		}
	}
	if w := m.call(t, asGM, "POST", "/setup/gm/contacts", `{"from":"Rev","to":"Janitor","oneWay":true}`); w.Code != 200 {
		t.Fatalf("contacts: %d %s", w.Code, w.Body)
	}
	w := m.send(t, asRev, `{"to":"Janitor","text":"Hi there","id":"h-1"}`)
	if w.Code != 200 || decoded(t, w)["to"] != "d4e5f6" || decoded(t, w)["state"] != "queued" {
		t.Fatalf("allowed: %d %s", w.Code, w.Body)
	}
	// One way only: Janitor can't write back; and a hire's id is not another's to ask about.
	if w := m.send(t, asJanitor, `{"to":"Rev","text":"x"}`); w.Code != 403 {
		t.Errorf("the other way: %d", w.Code)
	}
	if w := m.send(t, asJanitor, `{"to":"gm","text":"x","id":"h-1"}`); w.Code != 409 || w.Body.String() != "That id is already used. Pick another." {
		t.Errorf("another hire's id: %d %q", w.Code, w.Body)
	}
	// What the page reads of it: the hire is the sender.
	for _, row := range m.outbox(t, asPage, "") {
		if row["id"] == "h-1" && (row["from"] != "hire" || row["fromName"] != "Rev" || row["toName"] != "Janitor") {
			t.Errorf("the outbox holds %v", row)
		}
	}
	// A message to another hire makes no line in the GM's inbox.
	if events, _, _ := m.d.events.entries(0, 100); len(events) != 0 {
		t.Errorf("events: %+v", events)
	}
	// A hire's hour is its own: Rev's count does not count Janitor's.
	m.limits(0, 1)
	if w := m.send(t, asRev, `{"to":"gm","text":"x"}`); w.Code != 429 || !strings.HasPrefix(w.Body.String(), "That is 1 messages from hire Rev in the last hour") {
		t.Errorf("Rev's second: %d %q", w.Code, w.Body)
	}
	if w := m.send(t, asJanitor, `{"to":"gm","text":"x"}`); w.Code != 200 {
		t.Errorf("Janitor's first: %d %s", w.Code, w.Body)
	}
}

// A hire whose key no longer finds it is a 403 with the Mac's text; the route is never the hire's word alone.
func TestAHireWithNoKeyIsRefused(t *testing.T) {
	m := newMailDeck(t)
	req := httptest.NewRequest("POST", "/setup/gm/send", strings.NewReader(`{"to":"gm","text":"x"}`))
	w := httptest.NewRecorder()
	(&router{deck: m.d}).mailSend(w, &request{Request: req, caller: callerHire})
	if w.Code != 403 || w.Body.String() != "This message isn't from a hire Smolcorp started: a hire sends with the key in its own environment." {
		t.Errorf("%d %q", w.Code, w.Body)
	}
	if rows := m.outbox(t, asGM, ""); len(rows) != 0 {
		t.Errorf("kept: %v", rows)
	}
}

func TestTheOutboxRouteGivesTheTextToTheGMAndThePageOnly(t *testing.T) {
	m := newMailDeck(t)
	var fixture struct {
		Callers []string
		Answer  struct{ Messages []map[string]any }
	}
	if err := json.Unmarshal(frontDeskFixture(t, "outbox.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		m.send(t, asGM, `{"to":"Rev","text":"message `+string(rune('0'+i))+`","id":"m`+string(rune('0'+i))+`"}`)
	}
	m.send(t, asRev, `{"to":"gm","text":"Review done.","id":"h-001"}`)
	for _, as := range []who{asGM, asPage} {
		rows := m.outbox(t, as, "")
		if len(rows) != 4 || rows[0]["id"] != "m1" || rows[3]["id"] != "h-001" || rows[0]["text"] != "message 1" {
			t.Errorf("%v: %v", as, rows)
		}
	}
	// The tool reads the same rows without any text, with the fixture's keys.
	rows := m.outbox(t, asTool, "")
	if len(rows) != 4 {
		t.Fatalf("tool: %v", rows)
	}
	for _, row := range rows {
		if _, has := row["text"]; has {
			t.Errorf("the tool was given a text: %v", row)
		}
	}
	for _, key := range []string{"id", "from", "to", "toName", "at", "state", "size", "attempts"} {
		if _, has := rows[0][key]; !has {
			t.Errorf("a row lacks %q", key)
		}
	}
	for key := range fixture.Answer.Messages[4] {
		if key == "text" || key == "note" || key == "fromID" || key == "fromName" {
			continue
		}
		if _, has := fixture.Answer.Messages[0][key]; !has {
			t.Errorf("the fixture's own rows lack %q", key)
		}
	}
	if row := m.outbox(t, asTool, "")[3]; row["fromID"] != "a1b2c3" || row["fromName"] != "Rev" || row["note"] == nil {
		t.Errorf("a hire's row: %v", row)
	}
	// limit: the latest ones, oldest first; no limit is 100; a bad one is none; under 1 is 1.
	for query, want := range map[string][]string{"?limit=2": {"m3", "h-001"}, "?limit=0": {"h-001"}, "?limit=-5": {"h-001"},
		"?limit=abc": {"m1", "m2", "m3", "h-001"}, "?limit=": {"m1", "m2", "m3", "h-001"}, "?limit=99999": {"m1", "m2", "m3", "h-001"}} {
		var got []string
		for _, row := range m.outbox(t, asGM, query) {
			got = append(got, row["id"].(string))
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", query, got, want)
		}
	}
}

// The route table is what keeps callers out of the two routes.
func TestWhoMayUseTheMailRoutes(t *testing.T) {
	m := newMailDeck(t)
	if w := m.send(t, asTool, `{"to":"Rev","text":"x"}`); w.Code != 403 {
		t.Errorf("a tool sending: %d", w.Code)
	}
	if w := m.call(t, asRev, "GET", "/setup/gm/outbox", ""); w.Code != 403 {
		t.Errorf("a hire reading the outbox: %d", w.Code)
	}
	if w := m.call(t, asRev, "POST", "/setup/gm/send", `{"to":"gm","text":"x"}`); w.Code != 200 {
		t.Errorf("a hire sending: %d", w.Code)
	}
	if w := serve(t, m.d, call{method: "GET", target: "/setup/gm/outbox", host: "localhost:7710", from: thisComputer, withoutSetupKey: true}); w.Code != 401 {
		t.Errorf("nobody reading the outbox: %d", w.Code)
	}
}

// The outbox is written to the support folder and read again by the next Deck.
func TestTheOutboxSurvivesARestart(t *testing.T) {
	m := newMailDeck(t)
	m.send(t, asGM, `{"to":"Rev","text":"remember me","id":"keep-1"}`)
	again := startDeck(t)
	row, ok := again.mail.get("keep-1")
	if !ok || row.Text != "remember me" || row.State != mailQueued {
		t.Errorf("after a restart: %+v %v", row, ok)
	}
}

// The CLI's own client, `smolcorp gm send` and `gm outbox`, against the Go app's router.
func TestTheGMClientWorksAgainstTheGoApp(t *testing.T) {
	m := newMailDeck(t)
	srv := httptest.NewServer(&router{deck: m.d})
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	env := func(vars map[string]string) gm.Env {
		vars["SMOLCORP_PORT"] = port
		return gm.Env{Getenv: func(k string) string { return vars[k] }, SupportDir: func() (string, error) { return t.TempDir(), nil }}
	}

	// The GM.
	client, err := gm.NewSendClient(env(map[string]string{"SMOLCORP_GM_TOKEN": m.gm}))
	if err != nil {
		t.Fatal(err)
	}
	reply, _, err := gm.Send(client, "Rev", "Please look at the login change first.", "gm-cli-1")
	if err != nil || reply.ID != "gm-cli-1" || reply.To != "a1b2c3" || reply.State != "queued" || reply.Size != 38 || reply.Duplicate {
		t.Fatalf("send: %+v %v", reply, err)
	}
	if again, _, err := gm.Send(client, "Rev", "Please look at the login change first.", "gm-cli-1"); err != nil || !again.Duplicate {
		t.Errorf("sent again: %+v %v", again, err)
	}
	if _, _, err := gm.Send(client, "gm", "x", "gm-cli-2"); err == nil || !strings.Contains(err.Error(), "never put into the GM's own session") {
		t.Errorf("to the gm: %v", err)
	}
	msgs, _, err := gm.Outbox(client, 0)
	if err != nil || len(msgs) != 1 || msgs[0].ID != "gm-cli-1" || msgs[0].Text != "Please look at the login change first." || msgs[0].ToName != "Rev" || msgs[0].State != "queued" {
		t.Fatalf("outbox: %+v %v", msgs, err)
	}

	// A hire, signed by its key.
	hire, err := gm.NewSendClient(env(map[string]string{"SMOLCORP_HIRE_KEY": m.keys["a1b2c3"]}))
	if err != nil {
		t.Fatal(err)
	}
	reply, _, err = gm.Send(hire, "gm", "Review done.", "h-cli-1")
	if err != nil || reply.State != "delivered" || reply.To != "gm" || reply.Note != "Kept for the GM to read; not typed into its session." {
		t.Fatalf("hire send: %+v %v", reply, err)
	}
	if msgs, _, _ := gm.Outbox(client, 0); len(msgs) != 2 || msgs[1].FromName != "Rev" || msgs[1].Text != "Review done." {
		t.Errorf("outbox after the hire: %+v", msgs)
	}
}
