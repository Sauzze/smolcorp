package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ▤ Conversation (conversation.go), as the Mac's ConversationTests had it: read from transcripts of the tests' own,
// its pages, and who may read it. No redacted reading for a phone is served, so none is tested.

func convUser(text string) string {
	return jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}, "timestamp": "2026-10-22T10:00:00.000Z"})
}

func convClaude(text string) string {
	return jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant",
		"content": []any{map[string]any{"type": "thinking", "thinking": "private musing"}, map[string]any{"type": "text", "text": text}}}})
}

func convToolUse(name string, input map[string]any, id string) string {
	return jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant",
		"content": []any{map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}}}})
}

func convToolResult(content any, id string, isError bool) string {
	return jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": content, "is_error": isError}}}})
}

func convPage(t *testing.T, path string, before *int64, most int) conversationPage {
	t.Helper()
	page, ok := readConversation(path, before, most)
	if !ok {
		t.Fatalf("no page of %s", path)
	}
	return page
}

func convTexts(page conversationPage) []string {
	texts := []string{}
	for _, item := range page.Items {
		texts = append(texts, item.Text)
	}
	return texts
}

func convKinds(page conversationPage) []string {
	kinds := []string{}
	for _, item := range page.Items {
		kinds = append(kinds, item.Kind)
	}
	return kinds
}

func TestTheConversationShowsTheUsersWordsTheAgentsRepliesAndEachToolInALine(t *testing.T) {
	path := transcriptFile(t,
		jsonLine(map[string]any{"type": "summary", "summary": "old"}),
		convUser("Fix the login bug please"),
		convClaude("Looking at the login code."),
		convToolUse("Read", map[string]any{"file_path": "/repo/Login.swift"}, "t1"),
		convToolResult("1\timport Foundation\n2\tstruct Login {}\n3\t// … 400 more lines", "t1", false),
		convToolUse("Write", map[string]any{"file_path": "/repo/Login.swift", "content": "the whole new file\nwith its lines"}, "t2"),
		convToolResult("File written", "t2", false),
		convClaude("Fixed."),
	)
	page := convPage(t, path, nil, conversationPageMost)
	if got := convKinds(page); !reflect.DeepEqual(got, []string{"user", "agent", "tool", "result", "tool", "result", "agent"}) {
		t.Fatalf("kinds %v", got)
	}
	want := []string{"Fix the login bug please", "Looking at the login code.", "Read: /repo/Login.swift", "1\timport Foundation",
		"Write: /repo/Login.swift", "File written", "Fixed."}
	if got := convTexts(page); !reflect.DeepEqual(got, want) {
		t.Fatalf("texts %q", got)
	}
	if page.Items[2].From != "Read" || page.Items[0].At == nil || page.Before != nil {
		t.Fatalf("from %q, at %v, before %v", page.Items[2].From, page.Items[0].At, page.Before)
	}
	// What a tool was given to write never shows; nor its thinking, nor more than a result's first line.
	sent, _ := json.Marshal(page)
	for _, hidden := range []string{"the whole new file", "private musing", "400 more lines"} {
		if strings.Contains(string(sent), hidden) {
			t.Errorf("%q sent", hidden)
		}
	}
}

func TestTheConversationsWireFormatIsTheMacs(t *testing.T) {
	path := transcriptFile(t, convUser("hi"), jsonLine(map[string]any{"type": "user",
		"origin": map[string]any{"kind": "peer", "name": "gm", "body": "from the GM"}, "message": map[string]any{"role": "user", "content": "x"}}))
	data, _ := json.Marshal(convPage(t, path, nil, conversationPageMost))
	var page map[string]any
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatal(err)
	}
	items, _ := page["items"].([]any)
	first, _ := items[0].(map[string]any)
	peer, _ := items[1].(map[string]any)
	if page["before"] != nil || page["size"] == nil || len(items) != 2 {
		t.Fatalf("%s", data)
	}
	if first["id"] != "0.0" || first["offset"] != 0.0 || first["n"] != 0.0 || first["kind"] != "user" || first["text"] != "hi" ||
		first["more"] != false || first["at"] != 1792663200000.0 || first["from"] != nil {
		t.Errorf("the user's: %v", first)
	}
	if peer["kind"] != "peer" || peer["text"] != "from the GM" || peer["from"] != "gm" || peer["at"] != nil {
		t.Errorf("a peer's: %v", peer)
	}
}

func TestAQuestionShowsWithItsOptionsAndItsAnswer(t *testing.T) {
	path := transcriptFile(t,
		convToolUse("AskUserQuestion", map[string]any{"questions": []any{map[string]any{"question": "Which colour?",
			"options": []any{map[string]any{"label": "Red"}, map[string]any{"label": "Blue"}}}}}, "q1"),
		convToolResult(`User has answered your questions: "Which colour?"="Blue". You can now continue with the user's answers in mind.`, "q1", false),
	)
	page := convPage(t, path, nil, conversationPageMost)
	if got := convKinds(page); !reflect.DeepEqual(got, []string{"question", "answer"}) {
		t.Fatalf("kinds %v", got)
	}
	if got := convTexts(page); !reflect.DeepEqual(got, []string{"Which colour?\n1. Red\n2. Blue", `"Which colour?"="Blue"`}) {
		t.Fatalf("texts %q", got)
	}
}

func TestCommandsAndInterruptionsAreNotesAndToolErrorsSaySo(t *testing.T) {
	path := transcriptFile(t,
		convUser("<command-name>/clear</command-name>"),
		convUser("[Request interrupted by user]"),
		convToolUse("Bash", map[string]any{"command": "swift test"}, "t1"),
		convToolResult("error: no such module\nmore", "t1", true),
	)
	page := convPage(t, path, nil, conversationPageMost)
	if got := convKinds(page); !reflect.DeepEqual(got, []string{"note", "note", "tool", "result"}) {
		t.Fatalf("kinds %v", got)
	}
	if got := convTexts(page); !reflect.DeepEqual(got, []string{"ran /clear", "interrupted", "Bash: swift test", "Error: error: no such module"}) {
		t.Fatalf("texts %q", got)
	}
}

func TestALongMessageIsCutWithMore(t *testing.T) {
	page := convPage(t, transcriptFile(t, convUser(strings.Repeat("word ", 2_000)), convClaude("short")), nil, conversationPageMost)
	if len([]rune(page.Items[0].Text)) != conversationTextMost || !page.Items[0].More || page.Items[1].More {
		t.Fatalf("%d runes, more %v %v", len([]rune(page.Items[0].Text)), page.Items[0].More, page.Items[1].More)
	}
}

// The office's own window reads it as it was written: nothing leaves the computer.
func TestTheWindowReadsTheConversationAsItWasWritten(t *testing.T) {
	path := transcriptFile(t, convUser("deploy with\nsk-"+"ant-abc123"), convClaude("the password is swordfish"),
		convToolUse("Bash", map[string]any{"command": "ls"}, "t1"), convToolResult("API_KEY=hunter2\nmore", "t1", false))
	want := []string{"deploy with\nsk-" + "ant-abc123", "the password is swordfish", "Bash: ls", "API_KEY=hunter2"}
	if got := convTexts(convPage(t, path, nil, conversationPageMost)); !reflect.DeepEqual(got, want) {
		t.Fatalf("%q", got)
	}
}

func TestPagesGoBackWithoutGapsOrRepeats(t *testing.T) {
	var lines []string
	for n := 1; n <= 250; n++ {
		if n%2 == 0 {
			lines = append(lines, convClaude(fmt.Sprintf("reply %d", n)))
		} else {
			lines = append(lines, convUser(fmt.Sprintf("ask %d", n)))
		}
	}
	// Lines that show nothing in between, as a transcript has many.
	for n := 240; n > 0; n -= 10 {
		lines = append(lines[:n], append([]string{jsonLine(map[string]any{"type": "progress", "n": n})}, lines[n:]...)...)
	}
	path := transcriptFile(t, lines...)
	var seen []string
	var before *int64
	pages := 0
	for {
		page := convPage(t, path, before, 40)
		if len(page.Items) == 0 || len(page.Items) > 40 {
			t.Fatalf("page %d has %d items", pages, len(page.Items))
		}
		seen = append(convTexts(page), seen...)
		if page.Before != nil && before != nil && *page.Before >= *before {
			t.Fatalf("went forward: %d after %d", *page.Before, *before)
		}
		before = page.Before
		pages++
		if before == nil || pages >= 20 {
			break
		}
	}
	if before != nil || pages != 7 {
		t.Fatalf("%d pages, ending at %v", pages, before)
	}
	for n := 1; n <= 250; n++ {
		want := fmt.Sprintf("ask %d", n)
		if n%2 == 0 {
			want = fmt.Sprintf("reply %d", n)
		}
		if seen[n-1] != want {
			t.Fatalf("item %d: %q, want %q", n, seen[n-1], want)
		}
	}
}

func TestTheNewestPageEndsAtTheEndAndSaysTheFilesSize(t *testing.T) {
	var lines []string
	for n := 1; n <= 100; n++ {
		lines = append(lines, convUser(fmt.Sprintf("ask %d", n)))
	}
	path := transcriptFile(t, lines...)
	page := convPage(t, path, nil, conversationPageMost)
	info, _ := os.Stat(path)
	if len(page.Items) != conversationPageMost || page.Items[len(page.Items)-1].Text != "ask 100" || page.Items[0].Text != "ask 41" ||
		page.Size != info.Size() {
		t.Fatalf("%d items, %q to %q, size %d of %d", len(page.Items), page.Items[0].Text, page.Items[len(page.Items)-1].Text, page.Size, info.Size())
	}
	// Ids stay put as the file grows, for the window to add only what's new.
	var ids []string
	for _, item := range page.Items {
		ids = append(ids, item.ID)
	}
	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = file.WriteString(convUser("ask 101") + "\n")
	file.Close()
	grown := convPage(t, path, nil, conversationPageMost)
	var grownIDs []string
	for _, item := range grown.Items {
		grownIDs = append(grownIDs, item.ID)
	}
	if grown.Items[len(grown.Items)-1].Text != "ask 101" || !reflect.DeepEqual(grownIDs[:len(grownIDs)-1], ids[1:]) {
		t.Fatalf("grown: %v, was %v", grownIDs, ids)
	}
}

func TestAToolsWholeOutputIsSkippedNotRead(t *testing.T) {
	huge := convToolResult(strings.Repeat("x", conversationLineMost+10), "t1", false)
	page := convPage(t, transcriptFile(t, convUser("before"), huge, convUser("after")), nil, conversationPageMost)
	if got := convTexts(page); !reflect.DeepEqual(got, []string{"before", "after"}) {
		t.Fatalf("%q", got)
	}
}

func TestAHalfWrittenLastLineIsLeftForLater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "half.jsonl")
	_ = os.WriteFile(path, []byte(convUser("done")+"\n"+`{"type":"user","message":{"ro`), 0o600)
	if got := convTexts(convPage(t, path, nil, conversationPageMost)); !reflect.DeepEqual(got, []string{"done"}) {
		t.Fatalf("%q", got)
	}
}

func TestNoFileIsNoPage(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing.jsonl")} {
		if _, ok := readConversation(path, nil, conversationPageMost); ok {
			t.Errorf("a page of %q", path)
		}
	}
}

func TestAPagesPlaceIsDigitsOnly(t *testing.T) {
	if place, ok := conversationPlace(map[string][]string{}, "before"); place != nil || !ok {
		t.Error("none given")
	}
	if place, ok := conversationPlace(map[string][]string{"before": {"1234"}}, "before"); !ok || place == nil || *place != 1234 {
		t.Error("1234")
	}
	for _, wrong := range []string{"", "-1", "12a", "1e5", "99999999999999999999999", " 12"} {
		if _, ok := conversationPlace(map[string][]string{"before": {wrong}}, "before"); ok {
			t.Errorf("%q taken", wrong)
		}
	}
}

// The route: the office's own window on this computer only, never a phone, over the Wi-Fi or Tailscale. A page,
// 204 while the file hasn't grown, 400 for a place that isn't one, 404 for no agent or no transcript.
func TestOnlyTheOfficesOwnWindowReadsTheConversation(t *testing.T) {
	d := newTestDeck(t)
	phone := pair(t, d)
	path := transcriptFile(t, convUser("fix login"), convClaude("Done."))
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api", "transcript_path": path},
		"claude", "tty:/dev/ttys003", "", 10)
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s2", "cwd": "/work/web"}, "codex", "tty:/dev/ttys004", "", 10)
	d.mu.Lock()
	window, snapshot := d.access.WindowKey, string(d.snapshot())
	d.mu.Unlock()
	if !strings.Contains(snapshot, `"conversation":true`) || !strings.Contains(snapshot, `"conversation":false`) {
		t.Fatalf("the snapshot doesn't say which desk has a conversation: %s", snapshot)
	}
	ask := func(query string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "GET", target: "/conversation?" + query + "&t=" + window, host: "127.0.0.1:7710", from: thisComputer})
	}
	got := ask("session=s1")
	var page conversationPage
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &page) != nil || !reflect.DeepEqual(convTexts(page), []string{"fix login", "Done."}) {
		t.Fatalf("the window: %d %s", got.Code, got.Body)
	}
	if got := ask(fmt.Sprintf("session=s1&size=%d", page.Size)); got.Code != http.StatusNoContent {
		t.Errorf("not grown: %d", got.Code)
	}
	for query, want := range map[string]int{"session=s1&before=x": 400, "session=s1&size=": 400, "session=s2": 404, "session=gone": 404} {
		if got := ask(query); got.Code != want {
			t.Errorf("%s: %d %s, want %d", query, got.Code, got.Body, want)
		}
	}
	refused := serve(t, d, call{method: "GET", target: "/conversation?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone})
	if refused.Code != http.StatusForbidden || strings.TrimSpace(refused.Body.String()) != "The conversation shows only in the office's window on your computer." {
		t.Errorf("a phone: %d %s", refused.Code, refused.Body)
	}
}
