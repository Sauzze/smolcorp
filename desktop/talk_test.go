package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// 💬 Talk bubble, read side (talk.go): who may ask, a desk's card, and the waiting desks in order.

// talkAsk asks the window's route, as the office's own window does.
func talkAsk(t *testing.T, d *Deck, target string) *httptest.ResponseRecorder {
	t.Helper()
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	return serve(t, d, call{method: "GET", target: target + sep + "t=" + window, host: "127.0.0.1:7710", from: thisComputer})
}

// talkCardOf is the window's card of session id.
func talkCardOf(t *testing.T, d *Deck, id string) map[string]any {
	t.Helper()
	got := talkAsk(t, d, "/talk?session="+id)
	var card map[string]any
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &card) != nil {
		t.Fatalf("the card of %s: %d %s", id, got.Code, got.Body)
	}
	return card
}

// talkHook applies a hook event to the deck's sessions, one second after the last.
func talkHook(d *Deck, at float64, event map[string]any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions.Apply(event, Claude, "tab-"+event["session_id"].(string), "", at)
}

// talkAskQuestion has session id ask AskUserQuestion with these questions.
func talkAskQuestion(d *Deck, id string, at float64, questions ...any) {
	talkHook(d, at, map[string]any{"hook_event_name": "PreToolUse", "session_id": id, "cwd": "/work/" + id, "tool_name": "AskUserQuestion",
		"tool_input": map[string]any{"questions": questions}})
}

func talkQuestionOf(text string, multi bool, labels ...string) map[string]any {
	options := []any{}
	for _, label := range labels {
		options = append(options, map[string]any{"label": label, "description": "means " + label})
	}
	return map[string]any{"question": text, "header": "Pick", "multiSelect": multi, "options": options}
}

// Only the office's own window, on this computer, with its key: a phone's key is the window's sentence, no key,
// the GM's token, a hire's key, the hook's key, the setup key or its cookie and a proxied request are refused.
func TestOnlyTheOfficesOwnWindowReadsTheTalkCard(t *testing.T) {
	// The proxied and ts.net asks below come through the real `tailscale serve` (a stand-in peer, SN1): even then, the
	// window's key isn't taken. From a program here they are refused before it (peerowner_test.go).
	standInTailscalePeer(t)
	d := newTestDeck(t)
	phone := pair(t, d)
	talkHook(d, 1761000541000, map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "cwd": "/work/s1", "source": "startup"})
	if err := writeRegistryFile(registryPath(), []byte(`{"version":1,"hires":[{"id":"aaa111","name":"Rev","folder":"/w","role":"reviewer","state":"working"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	hireKey, err := newSendKey(registryPath(), "aaa111")
	if err != nil {
		t.Fatal(err)
	}
	gmToken := d.newGMToken()
	d.mu.Lock()
	window, setupKey, hookKey, cookie := d.access.WindowKey, d.access.SetupKey, d.access.HookKey, d.access.NewSetupSession()
	d.mu.Unlock()

	for _, target := range []string{"/talk?session=s1", "/talk/asking"} {
		ask := func(c call) *httptest.ResponseRecorder {
			c.method, c.target = "GET", target+map[bool]string{true: "&", false: "?"}[strings.Contains(target, "?")]+"x=1"+c.target
			if c.host == "" {
				c.host = "127.0.0.1:7710"
			}
			if c.from == "" {
				c.from = thisComputer
			}
			c.withoutSetupKey = true
			return serve(t, d, c)
		}
		if got := ask(call{target: "&t=" + window}); got.Code != http.StatusOK {
			t.Errorf("%s: the window: %d %s", target, got.Code, got.Body)
		}
		got := ask(call{target: "&t=" + phone, from: aPhone, host: deckOnWiFi})
		if got.Code != http.StatusForbidden || strings.TrimSpace(got.Body.String()) != "The talk bubble shows only in the office's window on your computer." {
			t.Errorf("%s: a phone: %d %s", target, got.Code, got.Body)
		}
		for name, c := range map[string]call{
			"no key":                                  {},
			"the GM's token":                          {headers: map[string]string{gmTokenHeader: gmToken}},
			"a hire's key":                            {headers: map[string]string{hireKeyHeader: hireKey}},
			"the setup key":                           {headers: map[string]string{setupKeyHeader: setupKey}},
			"the setup cookie":                        {headers: map[string]string{"Cookie": setupCookie() + "=" + cookie}},
			"the hook's key":                          {headers: map[string]string{hookKeyHeader: hookKey}},
			"the GM's token + t=":                     {target: "&t=" + gmToken, headers: map[string]string{gmTokenHeader: gmToken}},
			"a hire's key as t=":                      {target: "&t=" + hireKey},
			"the setup key as t=":                     {target: "&t=" + setupKey},
			"the window's key, proxied":               {target: "&t=" + window, headers: map[string]string{"X-Forwarded-For": "100.64.0.7"}},
			"the window's key, from another computer": {target: "&t=" + window, from: aPhone, host: deckOnWiFi},
			"the window's key, by a name that isn't this computer's": {target: "&t=" + window, host: "my-mac.tail1234.ts.net"},
		} {
			if got := ask(c); got.Code != http.StatusUnauthorized {
				t.Errorf("%s, %s: %d %s, want 401", target, name, got.Code, got.Body)
			}
		}
		// Nothing but a GET.
		for _, method := range []string{"POST", "PUT", "DELETE"} {
			got := serve(t, d, call{method: method, target: target + map[bool]string{true: "&", false: "?"}[strings.Contains(target, "?")] + "t=" + window, host: "127.0.0.1:7710", from: thisComputer})
			if got.Code != http.StatusNotFound {
				t.Errorf("%s %s: %d, want 404", method, target, got.Code)
			}
		}
	}
}

// A desk that isn't asking: its state and its card's asking is null. A session that has gone is 404.
func TestADeskNotAskingHasNoAskingAndAGoneOneIs404(t *testing.T) {
	d := newTestDeck(t)
	talkHook(d, 1761000541000, map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "cwd": "/work/s1", "source": "startup"})
	card := talkCardOf(t, d, "s1")
	if card["asking"] != nil || card["state"] != "idle" || card["last"] != "" || card["size"] != float64(0) {
		t.Errorf("a quiet desk: %v", card)
	}
	if _, has := card["asking"]; !has {
		t.Errorf("asking should be there, null: %v", card)
	}
	if got := talkAsk(t, d, "/talk?session=gone"); got.Code != http.StatusNotFound {
		t.Errorf("a gone desk: %d", got.Code)
	}
	if got := talkAsk(t, d, "/talk"); got.Code != http.StatusNotFound {
		t.Errorf("no session: %d", got.Code)
	}
	// Answered: the question leaves the card.
	talkAskQuestion(d, "s1", 1761000542000, talkQuestionOf("Which?", false, "A", "B"))
	if talkCardOf(t, d, "s1")["asking"] == nil {
		t.Fatal("asking should show")
	}
	talkHook(d, 1761000543000, map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": "/work/s1", "tool_name": "AskUserQuestion"})
	if card := talkCardOf(t, d, "s1"); card["asking"] != nil {
		t.Errorf("answered: %v", card)
	}
}

// A question's card: its options numbered from 1, each cut, one question that isn't multi-select is pickable.
func TestAQuestionsCardHasItsNumberedOptionsCutAndIsPickable(t *testing.T) {
	talkScreenShows(t, "") // a terminal whose screen a pick reads
	d := newTestDeck(t)
	long := func(n int) string { return strings.Repeat("é", n) }
	talkAskQuestion(d, "s1", 1761000541000, map[string]any{"question": long(400), "header": "Colour", "multiSelect": false,
		"options": []any{
			map[string]any{"label": long(120), "description": long(300)},
			map[string]any{"label": "Blue", "description": "the sea"},
		}})
	card := talkCardOf(t, d, "s1")
	asking, _ := card["asking"].(map[string]any)
	if card["state"] != "asking" || asking["kind"] != "question" || asking["tool"] != "AskUserQuestion" || asking["since"] != float64(1761000541000) ||
		asking["pickable"] != true || asking["label"] != "AskUserQuestion" {
		t.Fatalf("the card: %v", card)
	}
	questions, _ := asking["questions"].([]any)
	if len(questions) != 1 {
		t.Fatalf("questions: %v", asking["questions"])
	}
	q, _ := questions[0].(map[string]any)
	options, _ := q["options"].([]any)
	first, _ := options[0].(map[string]any)
	second, _ := options[1].(map[string]any)
	count := func(v any) int { s, _ := v.(string); return len([]rune(s)) }
	if count(q["question"]) != talkQuestionMost || !strings.HasSuffix(q["question"].(string), "…") || q["header"] != "Colour" || q["multiSelect"] != false ||
		len(options) != 2 || first["n"] != float64(1) || count(first["label"]) != talkLabelMost || count(first["description"]) != talkDescriptionMost ||
		second["n"] != float64(2) || second["label"] != "Blue" || second["description"] != "the sea" {
		t.Errorf("the question: %v", q)
	}
}

// Not pickable, so the window shows the four keys: several questions, a multi-select one, no options, more
// options than a menu has digits. The question is still on the card.
func TestSeveralQuestionsOrAMultiSelectOrTooManyOptionsAreNotPickable(t *testing.T) {
	talkScreenShows(t, "") // a terminal whose screen a pick reads
	many := make([]string, 10)
	for i := range many {
		many[i] = "o" + string(rune('a'+i))
	}
	for name, questions := range map[string][]any{
		"several questions": {talkQuestionOf("One?", false, "A", "B"), talkQuestionOf("Two?", false, "C", "D")},
		"multi-select":      {talkQuestionOf("Which ones?", true, "A", "B")},
		"no options":        {talkQuestionOf("Anything?", false)},
		"ten options":       {talkQuestionOf("Which?", false, many...)},
	} {
		d := newTestDeck(t)
		talkAskQuestion(d, "s1", 1761000541000, questions...)
		asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any)
		if asking == nil || asking["kind"] != "question" || asking["pickable"] != false {
			t.Errorf("%s: %v", name, asking)
		}
		if list, _ := asking["questions"].([]any); len(list) == 0 {
			t.Errorf("%s: the question isn't on the card: %v", name, asking)
		}
	}
	// Nine is the most a menu takes by digit.
	d := newTestDeck(t)
	talkAskQuestion(d, "s1", 1761000541000, talkQuestionOf("Which?", false, many[:9]...))
	if asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any); asking["pickable"] != true {
		t.Errorf("nine options: %v", asking)
	}
	// A question's input that isn't what AskUserQuestion sends: no questions, the four keys, no crash.
	d = newTestDeck(t)
	talkHook(d, 1761000541000, map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": "/work/s1", "tool_name": "AskUserQuestion",
		"tool_input": map[string]any{"questions": "not a list"}})
	if asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any); asking["pickable"] != false || len(asking["questions"].([]any)) != 0 {
		t.Errorf("a strange input: %v", asking)
	}
}

// A permission prompt's card has the label, Yes and No (pickable) and never the tool's input.
func TestAPermissionCardHasItsLabelAndNeverTheRawInput(t *testing.T) {
	talkScreenShows(t, "") // a terminal whose screen a pick reads
	d := newTestDeck(t)
	talkHook(d, 1761000541000, map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/s1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "git push --force origin secretbranch", "description": "private words"}})
	got := talkAsk(t, d, "/talk?session=s1")
	for _, leak := range []string{"--force", "secretbranch", "private words", "command"} {
		if strings.Contains(got.Body.String(), leak) {
			t.Errorf("the card gives out %q: %s", leak, got.Body)
		}
	}
	asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any)
	if asking["kind"] != "permission" || asking["tool"] != "Bash" || asking["label"] != "Bash: git" || asking["pickable"] != true ||
		len(asking["questions"].([]any)) != 0 || asking["since"] != float64(1761000541000) {
		t.Errorf("the card: %v", asking)
	}
	// A file tool: its file's name.
	talkHook(d, 1761000542000, map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/s1", "tool_name": "Edit",
		"tool_input": map[string]any{"file_path": "/work/s1/Sources/Deck.swift"}})
	if asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any); asking["label"] != "Edit: Deck.swift" || asking["pickable"] != true {
		t.Errorf("a file tool: %v", asking)
	}
	// Any other tool's menu, or a Bash or Write prompt the hook didn't say the command or file of, can't be told from
	// another prompt's: the card offers the four keys up front, not a Yes that would only be refused (FXP3c).
	for tool, input := range map[string]map[string]any{"WebFetch": {"url": "https://example.com"}, "NotebookEdit": {"notebook_path": "/work/s1/a.ipynb"},
		"Bash": {}, "Write": {}} {
		talkHook(d, 1761000542500, map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/s1", "tool_name": tool,
			"tool_input": input})
		if asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any); asking["kind"] != "permission" || asking["tool"] != tool || asking["pickable"] != false {
			t.Errorf("%s with %v: %v", tool, input, asking)
		}
	}
	// Leaving plan mode: its choice 1 changes the mode too, more than a Yes says, so the keys answer it.
	talkHook(d, 1761000543000, map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/s1", "tool_name": "ExitPlanMode",
		"tool_input": map[string]any{"plan": "the plan"}})
	if asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any); asking["kind"] != "permission" || asking["pickable"] != false {
		t.Errorf("leaving plan mode: %v", asking)
	}
}

// Asking with nothing said (a notification, another agent): the kind is unknown, nothing to pick by label.
func TestAskingWithNothingSaidIsUnknownAndNotPickable(t *testing.T) {
	d := newTestDeck(t)
	talkHook(d, 1761000541000, map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "cwd": "/work/s1", "source": "startup"})
	d.mu.Lock()
	d.sessions.Get("s1").State = Asking
	d.sessions.Get("s1").LastEventAt = 1761000541500
	d.mu.Unlock()
	asking, _ := talkCardOf(t, d, "s1")["asking"].(map[string]any)
	if asking["kind"] != "unknown" || asking["pickable"] != false || asking["tool"] != "" || asking["label"] != "" ||
		len(asking["questions"].([]any)) != 0 || asking["since"] != float64(1761000541500) {
		t.Errorf("unknown: %v", asking)
	}
}

// The last line: the agent's last reply (or question), cut, from the transcript's end; the size is the file's; read
// again only once the file has changed.
func TestTheLastLineIsTheAgentsLastWordsCutAndReadAgainOnlyWhenTheFileGrows(t *testing.T) {
	d := newTestDeck(t)
	path := transcriptFile(t, convUser("fix login"), convClaude("First answer."),
		convToolUse("Bash", map[string]any{"command": "go test"}, "t1"), convToolResult("ok", "t1", false))
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api", "transcript_path": path},
		"claude", "tty:/dev/ttys003", "", 10)
	info, _ := os.Stat(path)
	card := talkCardOf(t, d, "s1")
	if card["last"] != "First answer." || card["size"] != float64(info.Size()) {
		t.Fatalf("the card: %v (file %d)", card, info.Size())
	}

	// Not read again while the file is as it was: a same-size rewrite with its time put back isn't seen.
	rewritten := strings.Replace(convClaude("First answer."), "First", "Other", 1)
	if len(rewritten) != len(convClaude("First answer.")) {
		t.Fatal("the stand-in must be as long")
	}
	hold, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(hold), convClaude("First answer."), rewritten, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if card := talkCardOf(t, d, "s1"); card["last"] != "First answer." {
		t.Errorf("read again with the file as it was: %v", card)
	}

	// Grown: read again, and the new size is said. A long reply is cut.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(convClaude(strings.Repeat("x", 500)) + "\n")
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	grown, _ := os.Stat(path)
	card = talkCardOf(t, d, "s1")
	last, _ := card["last"].(string)
	if len([]rune(last)) != talkLastMost || !strings.HasSuffix(last, "…") || card["size"] != float64(grown.Size()) {
		t.Errorf("grown: %d characters, size %v, want %d", len([]rune(last)), card["size"], grown.Size())
	}

	// The window's own /conversation takes that size: nothing new, 204.
	got := talkAsk(t, d, fmt.Sprintf("/conversation?session=s1&size=%d", int64(card["size"].(float64))))
	if got.Code != http.StatusNoContent {
		t.Errorf("/conversation with the card's size: %d", got.Code)
	}

	// A question the agent asked counts as its last words.
	file, _ = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	file.WriteString(convToolUse("AskUserQuestion", map[string]any{"questions": []any{talkQuestionOf("Which colour?", false, "Red")}}, "q1") + "\n")
	file.Close()
	if card := talkCardOf(t, d, "s1"); !strings.HasPrefix(card["last"].(string), "Which colour?") {
		t.Errorf("a question: %v", card["last"])
	}
}

// No transcript: the card still comes, with nothing to say and no size. Nor does a missing file break it.
func TestADeskWithNoTranscriptStillHasACard(t *testing.T) {
	d := newTestDeck(t)
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s2", "cwd": "/work/web"}, "codex", "tty:/dev/ttys004", "", 10)
	if card := talkCardOf(t, d, "s2"); card["last"] != "" || card["size"] != float64(0) || card["state"] != "thinking" {
		t.Errorf("no transcript: %v", card)
	}
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s3", "cwd": "/work/api",
		"transcript_path": "/no/such/folder/session.jsonl"}, "claude", "tty:/dev/ttys005", "", 10)
	if card := talkCardOf(t, d, "s3"); card["last"] != "" || card["size"] != float64(0) {
		t.Errorf("a file that isn't there: %v", card)
	}
}

// /talk/asking: every desk asking, the longest waiting first; none asking is an empty list.
func TestTheAskingListHasEveryWaitingDeskLongestWaitingFirst(t *testing.T) {
	talkScreenShows(t, "") // a terminal whose screen a pick reads
	d := newTestDeck(t)
	list := func() []map[string]any {
		got := talkAsk(t, d, "/talk/asking")
		var reply struct{ Asking []map[string]any }
		if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &reply) != nil || reply.Asking == nil {
			t.Fatalf("the list: %d %s", got.Code, got.Body)
		}
		return reply.Asking
	}
	if got := list(); len(got) != 0 {
		t.Fatalf("nobody asks: %v", got)
	}
	for _, id := range []string{"s1", "s2", "s3", "s4"} {
		talkHook(d, 1761000540000, map[string]any{"hook_event_name": "SessionStart", "session_id": id, "cwd": "/work/" + id, "source": "startup"})
	}
	talkAskQuestion(d, "s1", 1761000545000, talkQuestionOf("Third?", false, "A"))
	talkHook(d, 1761000541000, map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s2", "cwd": "/work/s2", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "make"}})
	talkAskQuestion(d, "s3", 1761000543000, talkQuestionOf("Second?", false, "A", "B"), talkQuestionOf("Also?", false, "C"))
	rows := list()
	var order []string
	for _, row := range rows {
		order = append(order, row["session"].(string))
	}
	if strings.Join(order, " ") != "s2 s3 s1" {
		t.Fatalf("the order: %v", order)
	}
	if rows[0]["kind"] != "permission" || rows[0]["label"] != "Bash: make" || rows[1]["pickable"] != false || rows[2]["pickable"] != true ||
		rows[2]["since"] != float64(1761000545000) || len(rows[1]["questions"].([]any)) != 2 {
		t.Errorf("the rows: %v", rows)
	}
	// An answered one leaves the list.
	talkHook(d, 1761000546000, map[string]any{"hook_event_name": "PostToolUse", "session_id": "s2", "cwd": "/work/s2", "tool_name": "Bash"})
	if rows := list(); len(rows) != 2 || rows[0]["session"] != "s3" {
		t.Errorf("after an answer: %v", rows)
	}
}

// The cache keeps a bounded number of transcripts, and starts over past it.
func TestTheLastLineCacheIsBounded(t *testing.T) {
	path := transcriptFile(t, convClaude("Hello."))
	talkLasts.Lock()
	talkLasts.byPath = map[string]talkLast{}
	for i := 0; i < talkLastKept; i++ {
		talkLasts.byPath["/old/"+string(rune('a'+i%26))+time.Duration(i).String()] = talkLast{}
	}
	talkLasts.Unlock()
	if _, text := talkLastOf(path); text != "Hello." {
		t.Errorf("last: %q", text)
	}
	talkLasts.Lock()
	n := len(talkLasts.byPath)
	talkLasts.Unlock()
	if n != 1 {
		t.Errorf("%d kept, want the cache started over with this one", n)
	}
}

// The card says whether the window can type into the desk now (P2's typing, talksay.go): the same answer as
// windowTyping, so the bubble greys its input with the reason before anyone types.
func TestTheCardSaysWhetherTheWindowCanTypeIntoTheDesk(t *testing.T) {
	d := newTestDeck(t)
	talkHook(d, 1761000541000, map[string]any{"hook_event_name": "SessionStart", "session_id": "s1", "cwd": "/work/s1", "source": "startup"})
	want, found := d.windowTyping("s1")
	if !found {
		t.Fatal("the desk isn't found")
	}
	typing, _ := talkCardOf(t, d, "s1")["typing"].(map[string]any)
	if typing["ok"] != want.OK || typing["lines"] != want.Lines || typing["why"] != want.Why {
		t.Errorf("the card's typing %v, want %+v", typing, want)
	}
	talkAskQuestion(d, "s1", 1761000542000, talkQuestionOf("Which?", false, "A", "B"))
	if want, _ = d.windowTyping("s1"); want.OK {
		t.Fatalf("an asking desk can be typed into: %+v", want)
	}
	if typing, _ := talkCardOf(t, d, "s1")["typing"].(map[string]any); typing["ok"] != false || typing["why"] != want.Why {
		t.Errorf("an asking desk's typing %v, want %+v", typing, want)
	}
}
