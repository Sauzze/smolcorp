package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

type approvalsFixture struct {
	Answer map[string]any
	Items  []struct {
		Agent    string
		Desk     *int
		Floor    *int
		Hire     string
		Input    string
		Kind     string
		Name     string
		Nickname string
		Notified bool
		Session  string
		Since    float64
		Tool     string
	}
}

func loadApprovalsFixture(t *testing.T) approvalsFixture {
	t.Helper()
	var f approvalsFixture
	if err := json.Unmarshal(frontDeskFixture(t, "approvals.json"), &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// The answer is approvals.json's, item for item, the longest waiting first.
func TestTheApprovalsAnswerIsWhatApprovalsJSONSays(t *testing.T) {
	f := loadApprovalsFixture(t)
	var items []approvalItem
	for _, i := range f.Items {
		items = append(items, approvalItem{session: i.Session, hire: i.Hire, name: i.Name, nickname: i.Nickname, agent: i.Agent,
			floor: i.Floor, desk: i.Desk, kind: i.Kind, tool: i.Tool, input: i.Input, since: i.Since, notified: i.Notified})
	}
	// An item with no kind is a permission, as the Mac's default.
	for i := range items {
		if items[i].kind == "" {
			items[i].kind = "permission"
		}
	}
	got := asJSONValue(t, approvalsAnswer(items, f.Answer["at"].(float64)))
	if !reflect.DeepEqual(got, f.Answer) {
		g, _ := json.MarshalIndent(got, "", " ")
		w, _ := json.MarshalIndent(f.Answer, "", " ")
		t.Fatalf("differs from approvals.json\n got: %s\nwant: %s", g, w)
	}
	for _, leak := range []string{"secret text", "TOKEN=abc", "--force", "Which database"} {
		if b, _ := json.Marshal(got); strings.Contains(string(b), leak) {
			t.Errorf("the answer gives out %q", leak)
		}
	}
}

// Hook events from the deck's sessions: a prompt waits while the session is asking, with its
// tool, and is gone once it is answered.
func TestTheApprovalsRouteListsTheSessionsAskingFromTheirHooks(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	at := 1761000540000.0
	hook := func(event map[string]any) {
		at += 1000
		d.mu.Lock()
		d.sessions.Apply(event, Claude, "tab-"+event["session_id"].(string), "", at)
		d.mu.Unlock()
	}
	get := func() (int, []map[string]any) {
		w := serve(t, d, call{method: "GET", target: "/setup/gm/approvals", host: "localhost:7710", from: thisComputer})
		var reply struct{ Approvals []map[string]any }
		if w.Code == http.StatusOK && json.Unmarshal(w.Body.Bytes(), &reply) != nil {
			t.Fatalf("not JSON: %s", w.Body)
		}
		return w.Code, reply.Approvals
	}
	hook(map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "cwd": "/work/app", "source": "startup"})
	hook(map[string]any{"hook_event_name": "SessionStart", "session_id": "s2", "cwd": "/work/api", "source": "startup"})
	if code, rows := get(); code != 200 || len(rows) != 0 {
		t.Fatalf("nobody asks yet: %d %v", code, rows)
	}
	hook(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/app", "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": "/work/app/Sources/Deck.swift", "old_string": "secret text"}})
	hook(map[string]any{"hook_event_name": "PreToolUse", "session_id": "s2", "cwd": "/work/api", "tool_name": "AskUserQuestion",
		"tool_input": map[string]any{"questions": []any{map[string]any{"question": "Which database?"}}}})
	code, rows := get()
	if code != 200 || len(rows) != 2 {
		t.Fatalf("two ask: %d %v", code, rows)
	}
	if rows[0]["session"] != "s1" || rows[0]["label"] != "Edit: Deck.swift" || rows[0]["kind"] != "permission" || rows[0]["name"] != "app" ||
		rows[1]["session"] != "s2" || rows[1]["label"] != "AskUserQuestion" || rows[1]["kind"] != "question" {
		t.Fatalf("unexpected rows %v", rows)
	}
	hook(map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": "/work/app", "tool_name": "Edit"})
	if _, rows := get(); len(rows) != 1 || rows[0]["session"] != "s2" {
		t.Fatalf("an answered prompt should leave the list: %v", rows)
	}
}

// Nothing in the route can change a prompt: only a GET is served, and a GET changes no state.
func TestTheApprovalsRouteIsReadOnlyAndAGetChangesNothing(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/app", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "git push"}}, Claude, "tab-s1", "", 1761000540000)
	d.mu.Unlock()
	snapshot := func() Session {
		d.mu.Lock()
		defer d.mu.Unlock()
		return *d.sessions.Get("s1")
	}
	before := snapshot()
	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		for _, c := range []call{{}, {asTool: true}} {
			c.method, c.target, c.host, c.from, c.body = method, "/setup/gm/approvals", "localhost:7710", thisComputer, `{"session":"s1","action":"allow"}`
			c.headers = map[string]string{setupHeader: "1"}
			if w := serve(t, d, c); w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusForbidden {
				t.Errorf("%s asTool=%v: %d, want 405 or 403", method, c.asTool, w.Code)
			}
		}
	}
	for i := 0; i < 3; i++ {
		if w := serve(t, d, call{method: "GET", target: "/setup/gm/approvals", host: "localhost:7710", from: thisComputer}); w.Code != 200 {
			t.Fatalf("GET: %d", w.Code)
		}
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("the session changed:\n%+v\n%+v", before, after)
	}
	if _, err := os.Stat(registryPath()); err == nil {
		t.Error("a GET wrote a registry")
	}
}

// routes.json: read for gm, page and tool; nobody without a key; a hire is never let in.
func TestWhoMayReadTheApprovalsIsWhatRoutesJSONSays(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	get := func(c call) int {
		c.method, c.target, c.host, c.from = "GET", "/setup/gm/approvals", "localhost:7710", thisComputer
		return serve(t, d, c).Code
	}
	if got := get(call{withoutSetupKey: true}); got != http.StatusUnauthorized {
		t.Errorf("no key: %d", got)
	}
	for _, c := range []call{{}, {asTool: true}} {
		if got := get(c); got != 200 {
			t.Errorf("asTool=%v: %d", c.asTool, got)
		}
	}
	for _, who := range []caller{callerPage, callerTool} {
		if !routeAllows(who, "GET", "/setup/gm/approvals") {
			t.Errorf("%s may read the approvals", who.name())
		}
	}
	if routeAllows(callerNobody, "GET", "/setup/gm/approvals") {
		t.Error("nobody may read the approvals")
	}
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		if routeAllows(callerTool, method, "/setup/gm/approvals") {
			t.Errorf("a tool may %s the approvals", method)
		}
	}
}
