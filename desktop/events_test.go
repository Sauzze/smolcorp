package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The wire formats are the Mac's fixtures: they win, and are never edited.
func eventFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "frontdesk", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func asJSON(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("%v: %s", err, data)
	}
	return v
}

// A deck whose log's oldest kept line is 7 and whose newest are the fixture's 41 and 42.
func seededEventDeck(t *testing.T) *Deck {
	t.Helper()
	d := newTestDeck(t)
	managerOn(d)
	folder := t.TempDir()
	var seed strings.Builder
	for n := 7; n <= 40; n++ {
		line, _ := json.Marshal(eventEntry{ID: int64(n), At: 1793499000000, Session: "old", Folder: "/x", Agent: "claude", Kind: "prompt"})
		seed.Write(append(line, '\n'))
	}
	if err := os.WriteFile(filepath.Join(folder, "events.jsonl"), []byte(seed.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	d.events = newEventLog(folder, eventsFileMost)
	d.events.append(eventEntry{Agent: "claude", At: 1793500000000, Detail: "startup", Folder: "/Users/me/Projects/shop", Kind: "sessionStart", Session: "s-1f3a"})
	d.events.append(eventEntry{Agent: "claude", At: 1793500004000, Detail: "Bash: rm", Folder: "/Users/me/Projects/shop", Kind: "permissionDenied", Session: "s-1f3a"})
	return d
}

func getEvents(t *testing.T, d *Deck, query string) []byte {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: "/setup/gm/events" + query, host: "localhost:7710", from: thisComputer, asTool: true})
	if w.Code != 200 {
		t.Fatalf("GET /setup/gm/events%s: %d %s", query, w.Code, w.Body)
	}
	return w.Body.Bytes()
}

func TestTheFeedFixtureIsWhatTheRouteAnswers(t *testing.T) {
	d := seededEventDeck(t)
	if got, want := asJSON(t, getEvents(t, d, "?since=40&limit=10")), asJSON(t, eventFixture(t, "events.json")); !reflect.DeepEqual(got, want) {
		t.Errorf("answer %v, fixture %v", got, want)
	}
}

func TestTheEmptyAnswerFixtureIsWhatATimeoutAnswers(t *testing.T) {
	d := seededEventDeck(t)
	if got, want := asJSON(t, getEvents(t, d, "?since=42&wait=0.1")), asJSON(t, eventFixture(t, "events-empty.json")); !reflect.DeepEqual(got, want) {
		t.Errorf("answer %v, fixture %v", got, want)
	}
}

func TestTheLineFixtureIsWhatIsStored(t *testing.T) {
	raw := eventFixture(t, "event-line.json")
	var e eventEntry
	if err := json.Unmarshal(raw, &e); err != nil || e.ID != 42 || e.Kind != "permissionDenied" || e.Detail != "Bash: rm" {
		t.Fatalf("decoded %+v, %v", e, err)
	}
	// Written back the way the log writes it, it is the same line (the fields in the fixture's order).
	written, _ := json.Marshal(e)
	if string(written) != strings.TrimSpace(string(raw)) {
		t.Errorf("wrote %s, fixture %s", written, raw)
	}
}

func TestTheHookFixtureIsWhatEachHookEventBecomes(t *testing.T) {
	var cases []struct {
		Case   string
		Hook   map[string]any
		Agent  string
		Folder string
		At     float64
		Entry  map[string]any
	}
	if err := json.Unmarshal(eventFixture(t, "hook-events.json"), &cases); err != nil || len(cases) < 10 {
		t.Fatalf("%d cases, %v", len(cases), err)
	}
	for _, c := range cases {
		got, ok := eventFor(c.Hook, c.Agent, c.Folder, c.At)
		if c.Entry == nil {
			if ok {
				t.Errorf("%s: kept %+v", c.Case, got)
			}
			continue
		}
		stored, _ := json.Marshal(got)
		want := c.Entry
		want["id"] = float64(0)
		if !ok || !reflect.DeepEqual(asJSON(t, stored), any(want)) {
			t.Errorf("%s: got %s, want %v", c.Case, stored, want)
		}
	}
}

func TestAHookEventReachesTheFeedButNeverAPrompt(t *testing.T) {
	d := newTestDeck(t)
	managerOn(d)
	hook := func(event map[string]any) {
		event["session_id"], event["cwd"] = "s-1f3a", "/Users/me/Projects/shop"
		d.IngestFrom(event, "claude", "tab", "", 0, "", 1793500000000)
	}
	hook(map[string]any{"hook_event_name": "UserPromptSubmit", "prompt": "my secret plan"})
	hook(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash"})
	hook(map[string]any{"hook_event_name": "PermissionRequest", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "API_KEY=abc123 curl https://x.test"}})
	raw := getEvents(t, d, "")
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "abc123") {
		t.Errorf("the feed holds what it must not: %s", raw)
	}
	var reply struct {
		Events []eventEntry
		Next   int64
	}
	if err := json.Unmarshal(raw, &reply); err != nil || len(reply.Events) != 2 || reply.Next != 2 ||
		reply.Events[0].Kind != "prompt" || reply.Events[1].Detail != "Bash: curl" {
		t.Errorf("feed %s (%v)", raw, err)
	}
}

func TestAWaitIsAnsweredByTheNextEvent(t *testing.T) {
	l := newEventLog(t.TempDir(), eventsFileMost)
	got := make(chan []eventEntry, 1)
	go func() { got <- l.wait(context.Background(), 0, 10, 30*time.Second) }()
	select {
	case <-got:
		t.Fatal("answered with nothing to say")
	case <-time.After(100 * time.Millisecond):
	}
	l.append(eventEntry{Kind: "notification"})
	select {
	case found := <-got:
		if len(found) != 1 || found[0].Kind != "notification" {
			t.Errorf("got %+v", found)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not woken by the event")
	}
}

func TestAWaitEndsWhenTheCallerGoes(t *testing.T) {
	l := newEventLog(t.TempDir(), eventsFileMost)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.wait(ctx, 0, 10, 30*time.Second); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("still waiting after the caller went")
	}
	if len(l.waiters) != 0 {
		t.Errorf("%d waiters left", len(l.waiters))
	}
}

func TestTheQuestionIsKeptToWhatIsAllowed(t *testing.T) {
	check := func(query string, since int64, wait time.Duration, limit int) {
		t.Helper()
		u, _ := url.ParseQuery(query)
		s, w, l := eventQuestion(u)
		if s != since || w != wait || l != limit {
			t.Errorf("%q: %d %v %d, want %d %v %d", query, s, w, l, since, wait, limit)
		}
	}
	check("since=12&wait=999&limit=99999", 12, 30*time.Second, 1000)
	check("since=-4&wait=soon&limit=0", 0, 0, 1)
	check("", 0, 0, 200)
}

func TestNumberingGoesOnAcrossRestartsAndRotationKeepsTheNewest(t *testing.T) {
	folder := t.TempDir()
	l := newEventLog(folder, 600)
	for n := 1; n <= 30; n++ {
		l.append(eventEntry{Session: fmt.Sprintf("session-%d", n), Detail: "some words to fill the line"})
	}
	if _, err := os.Stat(l.earlier); err != nil {
		t.Fatal("not set aside once past the cap")
	}
	if l.oldestID <= 1 {
		t.Errorf("oldest %d: nothing dropped", l.oldestID)
	}
	if info, err := os.Stat(l.earlier); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("readable by you alone: %v %v", info, err)
	}
	again := newEventLog(folder, 600)
	if again.lastID != 30 || again.oldestID != l.oldestID {
		t.Errorf("restart: last %d oldest %d, want 30 and %d", again.lastID, again.oldestID, l.oldestID)
	}
	if e := again.append(eventEntry{}); e.ID != 31 {
		t.Errorf("next id %d", e.ID)
	}
	found, _, _ := again.entries(0, 1000)
	for i, e := range found {
		if e.ID != again.oldestID+int64(i) {
			t.Fatalf("a gap at %d: %d", i, e.ID)
		}
	}
	if found[len(found)-1].ID != 31 {
		t.Error("the newest is not kept")
	}
}

func TestOlderThanMemoryIsReadFromTheFilesAndASinceBeyondTheLastLineAnswersAtOnce(t *testing.T) {
	l := newEventLog(t.TempDir(), eventsFileMost)
	for n := 1; n <= eventsInMemory+10; n++ {
		l.append(eventEntry{})
	}
	found := l.wait(context.Background(), 0, 3, 0)
	if len(found) != 3 || found[0].ID != 1 || found[2].ID != 3 {
		t.Errorf("from the files: %+v", found)
	}
	start := time.Now()
	if found := l.wait(context.Background(), 99999, 10, 30*time.Second); len(found) != 0 || time.Since(start) > time.Second {
		t.Errorf("waited for ids that never come: %v", found)
	}
	if next := l.answer(nil, 99999)["next"]; next != int64(eventsInMemory+10) {
		t.Errorf("next %v: start again from the last line", next)
	}
}

func TestOnlyACallerWithTheSetupKeyOrPageReadsTheFeed(t *testing.T) {
	d := newTestDeck(t)
	if got := serve(t, d, call{method: "GET", target: "/setup/gm/events", host: "localhost:7710", from: thisComputer, withoutSetupKey: true}).Code; got != 401 {
		t.Errorf("no key: %d", got)
	}
	if got := serve(t, d, call{method: "GET", target: "/setup/gm/events", host: deckOnWiFi, from: aPhone}).Code; got != 404 {
		t.Errorf("from a phone: %d", got)
	}
}
