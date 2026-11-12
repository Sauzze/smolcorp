package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 💬 Talk bubble, pick (talkpick.go): who may pick, the guards, and the key each choice presses.

// talkPickAt picks choice n of session id's prompt that began at since, as the office's own window does.
func talkPickAt(t *testing.T, d *Deck, id string, n string, since int64) *httptest.ResponseRecorder {
	t.Helper()
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	return serve(t, d, call{method: "POST", target: fmt.Sprintf("/talk/pick?session=%s&n=%s&since=%d&t=%s", id, n, since, window),
		host: "127.0.0.1:7710", from: thisComputer})
}

// talkSinceOf is when session id began asking, as its card says it, or 0 when it isn't.
func talkSinceOf(d *Deck, id string) int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if asking := talkAskingOf(d.sessions.Get(id)); asking != nil {
		return asking.Since
	}
	return 0
}

// talkPermission has s1, in launcher L1, ask to run a command.
func talkPermission(d *Deck, at float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "touch x"}}, Claude, "launcher:L1", "", at)
}

// talkAsksColor has s1, in launcher L1, ask one question with three options, or several at once.
func talkAsksColor(d *Deck, at float64, multi bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PreToolUse", "session_id": "s1", "cwd": "/work/api", "tool_name": "AskUserQuestion",
		"tool_input": map[string]any{"questions": []any{talkQuestionOf("Which color?", multi, "Red", "Green", "Blue")}}}, Claude, "launcher:L1", "", at)
}

// nothingSent fails if anything reached the launcher.
func nothingSent(t *testing.T, tab *launcherTab, why string) {
	t.Helper()
	select {
	case command := <-tab.commands:
		t.Fatalf("%s: the launcher was told %+v", why, command)
	case <-time.After(100 * time.Millisecond):
	}
}

// The window's alone: a phone's key is the window's sentence, no key is 401, and only a POST.
func TestOnlyTheOfficesOwnWindowPicks(t *testing.T) {
	d, tab := madeButtonDeck(t)
	phone := pair(t, d)
	talkPermission(d, 200)
	since := talkSinceOf(d, "s1")
	target := fmt.Sprintf("/talk/pick?session=s1&n=1&since=%d", since)
	got := serve(t, d, call{method: "POST", target: target + "&t=" + phone, from: aPhone, host: deckOnWiFi})
	if got.Code != http.StatusForbidden || strings.TrimSpace(got.Body.String()) != "The talk bubble shows only in the office's window on your computer." {
		t.Errorf("a phone: %d %s", got.Code, got.Body)
	}
	if got := serve(t, d, call{method: "POST", target: target, host: "127.0.0.1:7710", from: thisComputer, withoutSetupKey: true}); got.Code != http.StatusUnauthorized {
		t.Errorf("no key: %d %s", got.Code, got.Body)
	}
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	if got := serve(t, d, call{method: "GET", target: target + "&t=" + window, host: "127.0.0.1:7710", from: thisComputer}); got.Code != http.StatusNotFound {
		t.Errorf("a GET: %d, want 404", got.Code)
	}
	nothingSent(t, tab, "refused picks")
}

// A question's choice is its digit, pressed once: a tap that lands twice is 204 and presses nothing more.
func TestAQuestionsChoiceIsItsDigitPressedOnce(t *testing.T) {
	d, tab := madeButtonDeck(t)
	talkAsksColor(d, 200, false)
	since := talkSinceOf(d, "s1")
	if got := talkPickAt(t, d, "s1", "2", since); got.Code != http.StatusNoContent {
		t.Fatalf("picking Green: %d %s", got.Code, got.Body)
	}
	if command := sent(t, tab); command.Press == nil || *command.Press != press("2") {
		t.Fatalf("picking Green sent %+v, want the digit 2", command)
	}
	for _, n := range []string{"2", "3"} {
		if got := talkPickAt(t, d, "s1", n, since); got.Code != http.StatusNoContent {
			t.Fatalf("a second pick (%s): %d %s", n, got.Code, got.Body)
		}
	}
	nothingSent(t, tab, "a second pick")
	// Still asking: the hook that it's answered (PostToolUse) is what ends it, not the pick.
	if talkSinceOf(d, "s1") != since {
		t.Fatal("a question's pick changed what the desk asks")
	}
}

// A permission prompt: Yes is 1, No is the agent's Reject, which leaves the desk free to be told what to do.
func TestAPermissionsYesIsOneAndNoRejectsAndClearsAsking(t *testing.T) {
	d, tab := madeButtonDeck(t)
	talkPermission(d, 200)
	if got := talkPickAt(t, d, "s1", "1", talkSinceOf(d, "s1")); got.Code != http.StatusNoContent || *sent(t, tab).Press != press("1") {
		t.Fatalf("Yes: %d %s", got.Code, got.Body)
	}
	talkPermission(d, 300)
	if got := talkPickAt(t, d, "s1", "2", talkSinceOf(d, "s1")); got.Code != http.StatusNoContent || *sent(t, tab).Press != press(Escape) {
		t.Fatalf("No: %d %s", got.Code, got.Body)
	}
	d.mu.Lock()
	state := d.sessions.Get("s1").State
	d.mu.Unlock()
	if state != Idle {
		t.Fatalf("after No the desk is %s, want idle", state)
	}
}

// The guards, each before any key: the prompt gone or another one (409), a choice it hasn't (400), choices not
// offered by label (409, the keys), a session gone (404).
func TestAPickOfAPromptThatIsntThereOrAChoiceItHasntPressesNothing(t *testing.T) {
	d, tab := madeButtonDeck(t)
	// Not asking.
	if got := talkPickAt(t, d, "s1", "1", 100); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != choiceGone {
		t.Errorf("not asking: %d %s", got.Code, got.Body)
	}
	talkAsksColor(d, 200, false)
	since := talkSinceOf(d, "s1")
	for _, c := range []struct {
		n     string
		since int64
		want  int
	}{
		{"1", since - 1, http.StatusConflict}, // an older prompt
		{"1", since + 1000, http.StatusConflict},
		{"0", since, http.StatusBadRequest},
		{"4", since, http.StatusBadRequest}, // three options
		{"x", since, http.StatusBadRequest},
		{"", since, http.StatusBadRequest},
	} {
		if got := talkPickAt(t, d, "s1", c.n, c.since); got.Code != c.want {
			t.Errorf("n=%q since=%d: %d %s, want %d", c.n, c.since, got.Code, got.Body, c.want)
		}
	}
	if got := serve(t, d, call{method: "POST", target: "/talk/pick?session=s1&n=1&t=" + d.access.WindowKey, host: "127.0.0.1:7710", from: thisComputer}); got.Code != http.StatusBadRequest {
		t.Errorf("no since: %d", got.Code)
	}
	// A permission prompt has two.
	talkPermission(d, 300)
	if got := talkPickAt(t, d, "s1", "3", talkSinceOf(d, "s1")); got.Code != http.StatusBadRequest {
		t.Errorf("a permission's third choice: %d %s", got.Code, got.Body)
	}
	// Several choices at once: the keys answer it.
	talkAsksColor(d, 400, true)
	if got := talkPickAt(t, d, "s1", "1", talkSinceOf(d, "s1")); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != talkUseTheKeys {
		t.Errorf("multi-select: %d %s", got.Code, got.Body)
	}
	// Asking with nothing said (a notification alone).
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, Claude, "launcher:L1", "", 500)
	d.sessions.Apply(map[string]any{"hook_event_name": "Notification", "session_id": "s1", "cwd": "/work/api", "notification_type": "permission_prompt",
		"message": "Claude needs your permission"}, Claude, "launcher:L1", "", 600)
	d.mu.Unlock()
	if since := talkSinceOf(d, "s1"); since == 0 {
		t.Fatal("a notification alone didn't make the desk ask")
	} else if got := talkPickAt(t, d, "s1", "1", since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != talkUseTheKeys {
		t.Errorf("asking with nothing said: %d %s", got.Code, got.Body)
	}
	if got := talkPickAt(t, d, "gone", "1", since); got.Code != http.StatusNotFound {
		t.Errorf("a session that has gone: %d %s", got.Code, got.Body)
	}
	nothingSent(t, tab, "refused picks")
}

// Another agent's menu isn't picked by number: its card isn't pickable, and a pick is refused with the keys' sentence.
func TestAnotherAgentsMenuIsAnsweredWithTheKeys(t *testing.T) {
	d := newTestDeck(t)
	tab := d.launcherConnected("L2", Codex, "/work/api", "", "")
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "c1", "cwd": "/work/api", "tool_name": "Bash"}, Codex, "launcher:L2", "", 200)
	asking := talkAskingOf(d.sessions.Get("c1"))
	d.mu.Unlock()
	if asking == nil || asking.Pickable {
		t.Fatalf("Codex's permission: %+v, want asking and not pickable", asking)
	}
	if got := talkPickAt(t, d, "c1", "1", asking.Since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != talkUseTheKeys {
		t.Errorf("Codex: %d %s", got.Code, got.Body)
	}
	nothingSent(t, tab, "Codex")
}

// The desk stopped asking while the pick waited for the key worker: looked at again right before the key, nothing
// is pressed, and the next prompt's pick still goes.
func TestADeskThatStoppedAskingMeanwhileGetsNoKey(t *testing.T) {
	d, tab := madeButtonDeck(t)
	talkPermission(d, 200)
	since := talkSinceOf(d, "s1")
	release := make(chan struct{})
	d.work <- func() { <-release }
	answered := make(chan *httptest.ResponseRecorder, 1)
	go func() { answered <- talkPickAt(t, d, "s1", "1", since) }()
	for deadline := time.Now().Add(5 * time.Second); len(d.work) == 0; {
		if time.Now().After(deadline) {
			t.Fatal("the pick never reached the worker")
		}
		time.Sleep(time.Millisecond)
	}
	// Answered in its own terminal meanwhile.
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "PostToolUse", "session_id": "s1", "cwd": "/work/api", "tool_name": "Bash"}, Claude, "launcher:L1", "", 300)
	d.mu.Unlock()
	close(release)
	if got := <-answered; got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != choiceGone {
		t.Fatalf("a pick of a prompt answered meanwhile: %d %s", got.Code, got.Body)
	}
	nothingSent(t, tab, "a prompt answered meanwhile")
	talkPermission(d, 400)
	if got := talkPickAt(t, d, "s1", "1", talkSinceOf(d, "s1")); got.Code != http.StatusNoContent || *sent(t, tab).Press != press("1") {
		t.Fatalf("the next prompt's pick: %d %s", got.Code, got.Body)
	}
}

// A test copy picks nothing in the owner's tabs (RWTC N1): its sentence, and nothing reaches the launcher.
func TestATestCopyPicksNothingInATabThatIsntItsOwn(t *testing.T) {
	d, tab := madeButtonDeck(t)
	talkPermission(d, 200)
	onMarkedPort(t, 7799)
	if got := talkPickAt(t, d, "s1", "1", talkSinceOf(d, "s1")); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != testCopyKeepsTerminal {
		t.Errorf("a test copy's pick: %d %s", got.Code, got.Body)
	}
	nothingSent(t, tab, "a test copy")
}

// Every terminal takes a digit: tmux by its name, WezTerm, kitty and a launcher by its byte.
func TestEveryTerminalTakesTheDigits(t *testing.T) {
	for n := 1; n <= 9; n++ {
		key, ok := digitKey(n)
		if !ok || tmuxKeys[key] != fmt.Sprint(n) || keyBytes[key] != fmt.Sprint(n) {
			t.Errorf("the digit %d: %q %v, tmux %q, bytes %q", n, key, ok, tmuxKeys[key], keyBytes[key])
		}
	}
	for _, n := range []int{0, 10, -1} {
		if _, ok := digitKey(n); ok {
			t.Errorf("%d is a digit key", n)
		}
		if _, ok := Claude.Pick(n); ok {
			t.Errorf("Claude picks %d", n)
		}
	}
	if k, ok := Claude.Pick(3); !ok || k != press("3") {
		t.Errorf("Claude's third choice: %+v %v", k, ok)
	}
	for _, a := range []Agent{Codex, Gemini, OpenCode, Grok} {
		if _, ok := a.Pick(1); ok {
			t.Errorf("%s picks by number", a)
		}
	}
}
