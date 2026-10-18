package desktop

import (
	"encoding/json"
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
