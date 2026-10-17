package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAWebhookGoesOnlyToYourNetworkUnlessAllowed(t *testing.T) {
	for _, address := range []string{
		"http://homeassistant.local:8123/api/webhook/deck", "http://192.168.1.5/hook", "http://10.0.0.2/hook",
		"http://172.20.0.1/hook", "http://127.0.0.1:9000/", "http://localhost/hook", "http://nas/hook", "http://[::1]:8080/",
	} {
		if problem := webhookProblem(address, false); problem != "" {
			t.Errorf("%s: %q, want it allowed", address, problem)
		}
	}
	for _, address := range []string{"https://example.com/hook", "http://8.8.8.8/", "http://172.32.0.1/"} {
		if webhookProblem(address, false) == "" {
			t.Errorf("%s allowed without ticking outside addresses", address)
		}
		if problem := webhookProblem(address, true); problem != "" {
			t.Errorf("%s with outside addresses allowed: %q", address, problem)
		}
	}
	for _, address := range []string{"", "homeassistant.local", "ftp://nas/file", "http://"} {
		if webhookProblem(address, true) == "" {
			t.Errorf("%q taken for a web address", address)
		}
	}
}

func TestAWebhookFromAnotherVersionKeepsWhatItKnows(t *testing.T) {
	var w Webhook
	if err := json.Unmarshal([]byte(`{"enabled": true, "url": "http://nas/hook", "events": ["ended", "teleported", "waiting"]}`), &w); err != nil {
		t.Fatal(err)
	}
	if !w.Enabled || w.URL != "http://nas/hook" || !slices.Equal(w.Events, []string{"waiting", "ended"}) {
		t.Fatalf("loaded %+v", w)
	}
	w = Webhook{}
	_ = json.Unmarshal([]byte(`{"url": "http://nas/hook"}`), &w)
	if !slices.Equal(w.Events, standardEvents) {
		t.Fatalf("no events given loaded %v, want the standard ones", w.Events)
	}
}

func TestTheWebhookHearsWhatChanged(t *testing.T) {
	at := time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC)
	before := map[string]told{
		"a": {state: Thinking, project: "api", agent: Claude, key: 1},
		"b": {state: Thinking, project: "web", agent: Codex, key: 2},
		"c": {state: Idle, project: "docs", agent: Claude, key: 3},
	}
	now := map[string]told{
		"a": {state: Asking, project: "api", agent: Claude, key: 1},
		"b": {state: Failed, project: "web", agent: Codex, key: 2, kind: "rate_limit"},
		"d": {state: Thinking, project: "new", agent: Gemini, key: 4},
	}
	got := map[string]map[string]any{}
	for _, m := range webhookMessages(before, now, at) {
		session := m["session"].(map[string]any)
		got[m["event"].(string)+":"+session["project"].(string)] = session
		if m["at"] != "2026-10-17T20:00:00Z" {
			t.Errorf("at %v", m["at"])
		}
	}
	if len(got) != 4 || got["waiting:api"] == nil || got["error:web"] == nil || got["started:new"] == nil || got["ended:docs"] == nil {
		t.Fatalf("messages %v", got)
	}
	if got["error:web"]["errorKind"] != "rate_limit" || got["error:web"]["previous"] != Thinking || got["error:web"]["agentName"] != Codex.Name() {
		t.Errorf("error message %v", got["error:web"])
	}
	if got["started:new"]["previous"] != nil || got["waiting:api"]["key"] != 1 {
		t.Errorf("started %v, waiting %v", got["started:new"], got["waiting:api"])
	}
	if len(webhookMessages(now, now, at)) != 0 {
		t.Error("nothing changed, yet something was sent")
	}
}

// listener is a tool waiting for the webhook: each message it's sent, with the secret it came with.
func listener(t *testing.T) (*httptest.Server, chan map[string]any, chan string) {
	t.Helper()
	messages, secrets := make(chan map[string]any, 16), make(chan string, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var message map[string]any
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &message)
		secrets <- r.Header.Get("X-Smolcorp-Secret")
		messages <- message
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	return server, messages, secrets
}

func setWebhook(t *testing.T, d *Deck, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, d, call{method: "POST", target: "/setup/webhook", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}, body: body})
}

func TestTheSetupPageSavesAWebhookAndSendsATest(t *testing.T) {
	d := newTestDeck(t)
	server, messages, secrets := listener(t)
	if refused := setWebhook(t, d, `{"enabled": true, "url": "https://example.com/hook"}`); refused.Code != http.StatusBadRequest {
		t.Fatalf("an outside address without the tick: %d", refused.Code)
	}
	if d.config.Webhook.Enabled {
		t.Fatal("a refused webhook was kept")
	}
	saved := setWebhook(t, d, `{"enabled": true, "url": " `+server.URL+` ", "secret": "s3cret", "events": ["done", "ended"]}`)
	if saved.Code != http.StatusOK || d.config.Webhook.URL != server.URL || !slices.Equal(d.config.Webhook.Events, []string{"done", "ended"}) {
		t.Fatalf("saving gave %d, kept %+v", saved.Code, d.config.Webhook)
	}
	if again := loadConfig(); again.Webhook.URL != server.URL {
		t.Fatalf("after loading again: %+v", again.Webhook)
	}
	tested := serve(t, d, call{method: "POST", target: "/setup/webhook/test", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	if message := <-messages; message["event"] != "test" || <-secrets != "s3cret" {
		t.Fatalf("the test sent %v", message)
	}
	var state struct{ Webhook struct{ Last string } }
	_ = json.Unmarshal(tested.Body.Bytes(), &state)
	if !strings.HasPrefix(state.Webhook.Last, "Sent: 204") {
		t.Fatalf("the setup page says %q", state.Webhook.Last)
	}
}

func TestADeckTellsTheWebhookAsSessionsChange(t *testing.T) {
	d := newTestDeck(t)
	server, messages, _ := listener(t)
	setWebhook(t, d, `{"enabled": true, "url": "`+server.URL+`", "events": ["waiting", "done", "started"]}`)
	apply := func(event map[string]any) {
		d.mu.Lock()
		d.sessions.Apply(event, Claude, "launcher:"+event["session_id"].(string), "", float64(time.Now().UnixMilli()))
		d.changed()
		d.mu.Unlock()
	}
	// Already running when the deck first looks: not news
	apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api"})
	apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s2", "cwd": "/work/web"})
	apply(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"})
	got := []string{}
	for range 2 {
		select {
		case m := <-messages:
			got = append(got, m["event"].(string)+":"+m["session"].(map[string]any)["project"].(string))
		case <-time.After(3 * time.Second):
			t.Fatalf("only %v arrived", got)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"done:api", "started:web"}) {
		t.Fatalf("the webhook heard %v", got)
	}
}
