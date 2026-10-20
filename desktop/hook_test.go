package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// A Write of a big file, as Claude Code reports it after the tool ran: its
// input holds the whole file, and its output says so again.
func bigEvent() []byte {
	content := strings.Repeat("A line of a big file.\n", 100_000)
	event, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse", "session_id": "big", "cwd": "/work/api", "tool_name": "Write",
		"tool_input":    map[string]any{"file_path": "/work/api/data.csv", "content": content},
		"tool_response": map[string]any{"filePath": "/work/api/data.csv", "content": content, "type": "create"},
		"elapsed_ms":    12.5,
	})
	return event
}

func TestABigEventIsTrimmedToWhatTheDeckTakes(t *testing.T) {
	event := bigEvent()
	if len(event) <= maxRequestSize {
		t.Fatalf("the event is only %d bytes, not bigger than the deck takes", len(event))
	}
	sent := trimmedEvent(event)
	if len(sent) > maxRequestSize {
		t.Fatalf("trimmed, it's still %d bytes", len(sent))
	}
	var fields map[string]any
	if err := json.Unmarshal(sent, &fields); err != nil {
		t.Fatal(err)
	}
	input, _ := fields["tool_input"].(map[string]any)
	if fields["session_id"] != "big" || fields["tool_name"] != "Write" || input["file_path"] != "/work/api/data.csv" || fields["elapsed_ms"] != 12.5 {
		t.Errorf("trimming lost what the deck shows: %v", fields)
	}
	if _, kept := fields["tool_response"]; kept {
		t.Error("the tool's output was kept")
	}
	small := []byte(`{"hook_event_name": "Stop", "session_id": "s1"}`)
	if string(trimmedEvent(small)) != string(small) {
		t.Error("a small event was changed")
	}
}

func TestABigEventStillReachesTheDeck(t *testing.T) {
	for _, c := range []struct {
		name  string
		event []byte
		want  bool
	}{{"as it was", bigEvent(), false}, {"trimmed", trimmedEvent(bigEvent()), true}} {
		d := newTestDeck(t)
		answer := serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: thisComputer,
			headers: map[string]string{hookKeyHeader: d.access.HookKey, "X-AgentDeck-Agent": "claude", "X-AgentDeck-Tab": "tmux:%1@/tmp/tmux-sock"}, body: string(c.event)})
		if answer.Code != http.StatusNoContent {
			t.Fatalf("%s: the hook got %d", c.name, answer.Code)
		}
		if got := d.sessions.Get("big") != nil; got != c.want {
			t.Errorf("%s: the session reached the deck: %v, want %v", c.name, got, c.want)
		}
	}
}

// A headless run is named by its session ID only when it says it is headless,
// has no terminal tab, and the ID has nothing else in it.
func TestAHeadlessRunIsNamedByItsSessionID(t *testing.T) {
	for _, name := range []string{"SMOLCORP_SESSION", "SAUZZEDECK_SESSION", "TMUX_PANE", "WEZTERM_PANE", "KITTY_WINDOW_ID"} {
		t.Setenv(name, "")
	}
	event := func(id string) []byte { return []byte(`{"hook_event_name":"Stop","session_id":"` + id + `"}`) }
	t.Setenv("SMOLCORP_HEADLESS", "")
	if tab := tabFromEnvironment(Claude, event("abc-123")); tab != "" {
		t.Fatalf("without the flag the tab is %q, want none", tab)
	}
	t.Setenv("SMOLCORP_HEADLESS", "1")
	if tab := tabFromEnvironment(Claude, event("abc-123")); tab != "headless:abc-123" {
		t.Fatalf("a headless run's tab is %q", tab)
	}
	for _, id := range []string{"", "abc 123", `abc\r\nX-Other: 1`, "abc@sock", "../x"} {
		if tab := tabFromEnvironment(Claude, event(id)); tab != "" {
			t.Errorf("a headless run with id %q has tab %q, want none", id, tab)
		}
	}
	if tab := tabFromEnvironment(Gemini, event("abc-123")); tab != "" {
		t.Errorf("another agent with the flag has tab %q, want none", tab)
	}
	t.Setenv("TMUX_PANE", "%3")
	t.Setenv("TMUX", "/tmp/tmux-sock,1,0")
	if tab := tabFromEnvironment(Claude, event("abc-123")); tab != "tmux:%3@/tmp/tmux-sock" {
		t.Errorf("a run in a terminal with the flag has tab %q: the terminal wins", tab)
	}
	if pid := headlessAgentPID(); pid <= 1 {
		t.Errorf("the agent's pid is %d", pid)
	}
}

func TestTheHookRouteKeepsAHeadlessRunsPID(t *testing.T) {
	d := newTestDeck(t)
	for i, c := range []struct {
		pid  string
		want int
	}{{"4242", 4242}, {"42a", 0}, {"-5", 0}, {"", 0}} {
		id := fmt.Sprintf("h%d", i)
		answer := serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: thisComputer,
			headers: map[string]string{hookKeyHeader: d.access.HookKey, "X-AgentDeck-Agent": "claude",
				"X-AgentDeck-Tab": "headless:" + id, "X-AgentDeck-PID": c.pid},
			body: `{"hook_event_name":"UserPromptSubmit","session_id":"` + id + `","cwd":"/work/api"}`})
		session := d.sessions.Get(id)
		if answer.Code != http.StatusNoContent || session == nil {
			t.Fatalf("the hook with pid %q gave %d and no session", c.pid, answer.Code)
		}
		if session.PID != c.want {
			t.Errorf("pid %q was kept as %d, want %d", c.pid, session.PID, c.want)
		}
	}
}

func TestAHeadlessRunSaysWhoHiredIt(t *testing.T) {
	d := newTestDeck(t)
	running(d, 1) // s0, in /work/p0
	for id, parent := range map[string]string{"h0": "/work/p0/", "h1": "/elsewhere/tool", "h2": ""} {
		answer := serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: thisComputer,
			headers: map[string]string{hookKeyHeader: d.access.HookKey, "X-AgentDeck-Agent": "claude",
				"X-AgentDeck-Tab": "headless:" + id, "X-AgentDeck-PID": "4242", "X-AgentDeck-Parent": parent},
			body: `{"hook_event_name":"UserPromptSubmit","session_id":"` + id + `","cwd":"/work/api"}`})
		if answer.Code != http.StatusNoContent {
			t.Fatalf("the hook for %s gave %d", id, answer.Code)
		}
	}
	hired := map[string]any{}
	for _, desk := range d.upstairs()["desks"].([]map[string]any) {
		hired[desk["id"].(string)] = desk["hiredBy"]
	}
	if by, _ := hired["h0"].(map[string]any); by["id"] != "s0" || by["name"] != "p0" {
		t.Errorf("hired from the folder of a desk downstairs: %v", hired["h0"])
	}
	if by, _ := hired["h1"].(map[string]any); by["id"] != nil || by["name"] != "tool" {
		t.Errorf("hired from a folder with nobody in it: %v", hired["h1"])
	}
	if by, _ := hired["h2"].(map[string]any); by != nil {
		t.Errorf("hired by nobody known: %v", hired["h2"])
	}
}

func TestTheHeadlessAgentIsTheNearestClaudeElseTheNearestNode(t *testing.T) {
	tree := func(names map[int]string) func(int) (int, string, bool) {
		return func(pid int) (int, string, bool) {
			name, ok := names[pid]
			return pid - 1, name, ok
		}
	}
	// 10 the hook's shell, 9 claude, 8 a node that started it.
	if pid := nearestAgent(10, tree(map[int]string{10: "bash", 9: "claude", 8: "node", 7: "init"})); pid != 9 {
		t.Fatalf("got %d, want claude's 9", pid)
	}
	// claude run by node: no claude by name, so the nearest node, not the one that started that.
	if pid := nearestAgent(10, tree(map[int]string{10: "bash", 9: "node", 8: "node", 7: "init"})); pid != 9 {
		t.Fatalf("got %d, want the nearest node, 9", pid)
	}
	// A claude above a nearer node is still the agent.
	if pid := nearestAgent(10, tree(map[int]string{10: "node", 9: "claude", 8: "init"})); pid != 9 {
		t.Fatalf("got %d, want claude's 9", pid)
	}
	// Neither: the process that ran the hook.
	if pid := nearestAgent(10, tree(map[int]string{10: "bash", 9: "init"})); pid != 10 {
		t.Fatalf("got %d, want the hook's parent, 10", pid)
	}
}
