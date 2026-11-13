package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// talkScreenShows has every desk's tab a screen a pick reads, showing screen until show changes it.
func talkScreenShows(t *testing.T, screen string) (show func(string)) {
	t.Helper()
	shows, reads := talkShowsScreen, talkScreenOf
	t.Cleanup(func() { talkShowsScreen, talkScreenOf = shows, reads })
	var mu sync.Mutex
	talkShowsScreen = func(string) bool { return true }
	talkScreenOf = func(Tab, Agent) ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		return screenEnd(screen, talkScreenLines), nil
	}
	return func(now string) {
		mu.Lock()
		defer mu.Unlock()
		screen = now
	}
}

// talkTop is what a screen shows over a menu: the turn so far.
const talkTop = `▐▛███▜▌   Claude Code v2.1.322
❯ Use the Bash tool to run exactly this command. Do nothing else and do not explain.
  Creating the file
  ⎿  $ touch x
`

// permissionMenu is Claude Code's permission prompt for a Bash command, as 2.1.322 draws it.
func permissionMenu(command string) string {
	return talkTop + strings.Repeat("─", 60) + `
 Bash command
 Tip: auto mode handles these prompts for you — choose "switch to auto mode" below
 Runs the command
` + strings.Repeat("╌", 60) + "\n " + command + "\n" + strings.Repeat("╌", 60) + `
 Do you want to proceed?
 ❯ 1. Yes
   2. Yes, and always allow access to /work/api from this project
   3. Yes, and switch to auto mode · auto mode handles these prompts for you
   4. No
 Esc to cancel · Tab to amend


`
}

// fileMenu is Claude Code's permission prompt for Write (create) or Edit (edit) on note.txt, as 2.1.322 draws it:
// taken from a real capture (TestRealClaudeMenusTakeTheDigit, steps 7 and 8).
func fileMenu(edit bool) string {
	tool, title, diff, ask := "Write", "Create file", "  1 hello", "Do you want to create note.txt?"
	if edit {
		tool, title, ask = "Update", "Edit file", "Do you want to make this edit to note.txt?"
		diff = " 1 -hello\n 1   No newline at end of file\n 2 +goodbye\n 3   No newline at end of file"
	}
	return talkTop + "⏺ " + tool + "(note.txt)\n\n" + strings.Repeat("─", 60) + "\n " + title + "\n note.txt\n" + strings.Repeat("╌", 60) + "\n" +
		diff + "\n" + strings.Repeat("╌", 60) + "\n " + ask + `
 ❯ 1. Yes
   2. Yes, and switch to accept edits (auto-approve file edits and common file commands) for this session (shift+tab)
   3. No

 Esc to cancel · Tab to amend
`
}

// questionMenu is Claude Code's menu for AskUserQuestion's one question, as 2.1.322 draws it.
func questionMenu(question string, options ...string) string {
	menu := talkTop + strings.Repeat("─", 60) + "\n ☐ Colour\n" + question + "\n"
	for i, option := range options {
		cursor := "  "
		if i == 0 {
			cursor = "❯ "
		}
		menu += fmt.Sprintf("%s%d. %s\n     means %s\n", cursor, i+1, option, option)
	}
	return menu + fmt.Sprintf("  %d. Type something.\n%s\n  %d. Chat about this\nEnter to select · ↑/↓ to navigate · Esc to cancel\n",
		len(options)+1, strings.Repeat("─", 60), len(options)+2)
}

// talkInputBox is the end of a screen where Claude Code waits for a prompt, its menu put away: input then lands in its box.
const talkInputBox = talkTop + `  ⎿  Interrupted · What should Claude do instead?
✻ Sautéed for 0s · done 1:45 AM
────────────────────────────────────────────────────────────
❯
────────────────────────────────────────────────────────────
  ⏸ manual mode on · ? for shortcuts
`

// talkToolRuns is the end of a screen where the command, allowed in its own terminal, runs: Escape would stop it.
const talkToolRuns = talkTop + `⏺ Bash(sleep 60 && make test)
  ⎿  Running…
✳ Sautéing… (12s · ↓ 93 tokens)
────────────────────────────────────────────────────────────
❯
────────────────────────────────────────────────────────────
  ⏸ manual mode on · esc to interrupt
`

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
	// A phone that sends its key (on this computer, as over Tailscale) is told the window's sentence.
	got := serve(t, d, call{method: "POST", target: target + "&t=" + phone, from: thisComputer, host: "127.0.0.1:7710"})
	if got.Code != http.StatusForbidden || strings.TrimSpace(got.Body.String()) != "The talk bubble shows only in the office's window on your computer." {
		t.Errorf("a phone: %d %s", got.Code, got.Body)
	}
	// On the Wi-Fi a phone signs (1.25), and its signature's nonce is n too: a second n is never well signed, so it is
	// refused before the table (401). Refused all the same; the window never signs.
	if got := serve(t, d, call{method: "POST", target: target + "&t=" + phone, from: aPhone, host: deckOnWiFi}); letIn(got.Code) {
		t.Errorf("a phone on the Wi-Fi: %d %s", got.Code, got.Body)
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
	talkScreenShows(t, questionMenu("Which color?", "Red", "Green", "Blue"))
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
	talkScreenShows(t, permissionMenu("touch x"))
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
	talkScreenShows(t, questionMenu("Which color?", "Red", "Green", "Blue"))
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
	talkScreenShows(t, permissionMenu("touch x"))
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

// RT126P3 B1: the hooks still say the desk asks, but its screen ends with Claude Code's input box. Put away in its
// own terminal (Esc there, no hook), or answered Yes there while its command runs (no hook till it ends): nothing is
// pressed, neither a digit into the input box nor an Escape into the running tool, and the desk stops asking so the
// input wakes: idle, or thinking while the tool runs (RT126P3b N1).
func TestAPickWithNoMenuOnItsScreenPressesNothingAndStopsAsking(t *testing.T) {
	for name, c := range map[string]struct {
		screen string
		ask    func(*Deck)
		n      string
		state  SessionState
	}{
		"Esc in its terminal, then Yes":        {talkInputBox, func(d *Deck) { talkPermission(d, 200) }, "1", Idle},
		"Yes in its terminal, then No":         {talkToolRuns, func(d *Deck) { talkPermission(d, 200) }, "2", Thinking},
		"a question put away, then its choice": {talkInputBox, func(d *Deck) { talkAsksColor(d, 200, false) }, "2", Idle},
	} {
		d, tab := madeButtonDeck(t)
		show := talkScreenShows(t, permissionMenu("touch x"))
		c.ask(d)
		since := talkSinceOf(d, "s1")
		show(c.screen)
		if got := talkPickAt(t, d, "s1", c.n, since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != choiceGone {
			t.Errorf("%s: %d %s", name, got.Code, got.Body)
		}
		nothingSent(t, tab, name)
		if since := talkSinceOf(d, "s1"); since != 0 {
			t.Errorf("%s: still asking", name)
		}
		d.mu.Lock()
		s := d.sessions.Get("s1")
		state, activity := s.State, s.Activity()
		d.mu.Unlock()
		if state != c.state || strings.Contains(activity, "Waiting for you") {
			t.Errorf("%s: %s %q, want %s", name, state, activity, c.state)
		}
	}
}

// A file's menu (Write, Edit), not Bash's: Yes is its digit 1 too (RT126P3b B1, 3).
func TestAPickAnswersAFilesMenu(t *testing.T) {
	for _, edit := range []bool{false, true} {
		d, tab := madeButtonDeck(t)
		talkScreenShows(t, fileMenu(edit))
		tool := map[bool]string{false: "Write", true: "Edit"}[edit]
		d.mu.Lock()
		d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": tool,
			"tool_input": map[string]any{"file_path": "/work/api/note.txt"}}, Claude, "launcher:L1", "", 200)
		d.mu.Unlock()
		if got := talkPickAt(t, d, "s1", "1", talkSinceOf(d, "s1")); got.Code != http.StatusNoContent || *sent(t, tab).Press != press("1") {
			t.Errorf("%s's Yes: %d %s", tool, got.Code, got.Body)
		}
	}
}

// RT126P3c N1: a card for a file or any other tool matches only its own menu. A Write card with a Bash menu on the
// screen presses nothing and is answered 409 choiceGone, still asking; a tool whose menu it can't tell is 409 with the
// keys' sentence, still asking.
func TestAFileCardPicksOnlyItsOwnMenu(t *testing.T) {
	writeCard := func(tool string, input map[string]any) func(*Deck) {
		return func(d *Deck) {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": tool,
				"tool_input": input}, Claude, "launcher:L1", "", 200)
		}
	}
	file := map[string]any{"file_path": "/work/api/note.txt"}
	for name, c := range map[string]struct {
		screen string
		ask    func(*Deck)
		n      string
		body   string
	}{
		"a Write card, a Bash menu":    {permissionMenu("rm -rf build"), writeCard("Write", file), "1", choiceGone},
		"a Write card, Bash's No":      {permissionMenu("rm -rf build"), writeCard("Write", file), "2", choiceGone},
		"an Edit card, a Bash menu":    {permissionMenu("touch x"), writeCard("Edit", file), "1", choiceGone},
		"a Write card, another file":   {fileMenu(false), writeCard("Write", map[string]any{"file_path": "/work/api/other.txt"}), "1", choiceGone},
		"a Write card, no path":        {fileMenu(false), writeCard("Write", map[string]any{}), "1", talkUseTheKeys},
		"a Bash card, no command":      {permissionMenu("touch x"), writeCard("Bash", map[string]any{}), "1", talkUseTheKeys},
		"a tool it can't tell, a menu": {permissionMenu("touch x"), writeCard("WebFetch", map[string]any{"url": "https://example.com"}), "1", talkUseTheKeys},
	} {
		d, tab := madeButtonDeck(t)
		talkScreenShows(t, c.screen)
		c.ask(d)
		since := talkSinceOf(d, "s1")
		if got := talkPickAt(t, d, "s1", c.n, since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != c.body {
			t.Errorf("%s: %d %s, want 409 %s", name, got.Code, got.Body, c.body)
		}
		nothingSent(t, tab, name)
		if talkSinceOf(d, "s1") != since {
			t.Errorf("%s: no longer asking", name)
		}
	}
}

// RT126P3c N2: clearing asking from the screen leaves the desk's last event where it was, so a hook raised before the
// look (a parallel prompt's) that lands after it isn't dropped as older: asking comes back, the safe side.
func TestAPickThatClearsAskingKeepsTheTimeOfTheLastEvent(t *testing.T) {
	for name, screen := range map[string]string{"waiting": talkInputBox, "working": talkToolRuns} {
		d, tab := madeButtonDeck(t)
		show := talkScreenShows(t, permissionMenu("touch x"))
		talkPermission(d, 200)
		since := talkSinceOf(d, "s1")
		show(screen)
		if got := talkPickAt(t, d, "s1", "1", since); got.Code != http.StatusConflict {
			t.Fatalf("%s: %d %s", name, got.Code, got.Body)
		}
		nothingSent(t, tab, name)
		if talkSinceOf(d, "s1") != 0 {
			t.Fatalf("%s: still asking", name)
		}
		// The next prompt's hook, stamped after the first's and before the pick's own look.
		d.mu.Lock()
		d.sessions.Apply(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": "Bash",
			"tool_input": map[string]any{"command": "touch y"}}, Claude, "launcher:L1", "", 300)
		d.mu.Unlock()
		if talkSinceOf(d, "s1") == 0 {
			t.Errorf("%s: a hook still on its way was dropped as older", name)
		}
	}
}

// RT126P3b B1: a screen it can't make out may be a live menu: choice 1 off its top, a footer in words it doesn't
// know, neither a footer nor the input box, nothing read. Nothing is pressed and the desk still asks, so the phone's
// words still keep out of it and the four keys answer it.
func TestAPickOfAScreenItCantMakeOutPressesNothingAndStillAsks(t *testing.T) {
	// A question in a short pane: its options' descriptions push choice 1 off the screen's top.
	question := questionMenu("Which color?", "Red", "Green", "Blue")
	cutTop := question[strings.Index(question, "  2. Green"):]
	for name, c := range map[string]struct {
		screen string
		ask    func(*Deck)
		n      string
	}{
		"a screen with nothing on it":        {"", func(d *Deck) { talkPermission(d, 200) }, "1"},
		"a question's choice 1 cut off":      {cutTop, func(d *Deck) { talkAsksColor(d, 200, false) }, "2"},
		"an unknown footer word":             {strings.Replace(permissionMenu("touch x"), "Esc to cancel", "Esc to dismiss", 1), func(d *Deck) { talkPermission(d, 200) }, "1"},
		"an unknown footer word, No":         {strings.Replace(permissionMenu("touch x"), "Esc to cancel", "Esc to dismiss", 1), func(d *Deck) { talkPermission(d, 200) }, "2"},
		"neither a footer nor the input box": {talkTop + "✻ Sautéed for 0s · done 1:45 AM\n", func(d *Deck) { talkPermission(d, 200) }, "1"},
	} {
		d, tab := madeButtonDeck(t)
		show := talkScreenShows(t, permissionMenu("touch x"))
		c.ask(d)
		since := talkSinceOf(d, "s1")
		show(c.screen)
		if got := talkPickAt(t, d, "s1", c.n, since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != talkUseTheKeys {
			t.Errorf("%s: %d %s", name, got.Code, got.Body)
		}
		nothingSent(t, tab, name)
		if talkSinceOf(d, "s1") != since {
			t.Errorf("%s: no longer asking", name)
		}
	}
}

// Another prompt's menu on the screen: another command, another question, or a question where a permission was
// asked. Nothing is pressed, and the desk still asks (the four keys answer what it shows).
func TestAPickOfAnotherPromptsMenuPressesNothing(t *testing.T) {
	for name, c := range map[string]struct {
		screen string
		ask    func(*Deck)
		n      string
	}{
		"another command":               {permissionMenu("rm -rf build"), func(d *Deck) { talkPermission(d, 200) }, "1"},
		"another command, No":           {permissionMenu("rm -rf build"), func(d *Deck) { talkPermission(d, 200) }, "2"},
		"another question":              {questionMenu("Which fruit?", "Apple", "Banana", "Cherry"), func(d *Deck) { talkAsksColor(d, 200, false) }, "2"},
		"a question for a permission":   {questionMenu("Which color?", "Red", "Green", "Blue"), func(d *Deck) { talkPermission(d, 200) }, "1"},
		"a permission for the question": {permissionMenu("touch x"), func(d *Deck) { talkAsksColor(d, 200, false) }, "1"},
	} {
		d, tab := madeButtonDeck(t)
		talkScreenShows(t, c.screen)
		c.ask(d)
		since := talkSinceOf(d, "s1")
		if got := talkPickAt(t, d, "s1", c.n, since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != choiceGone {
			t.Errorf("%s: %d %s", name, got.Code, got.Body)
		}
		nothingSent(t, tab, name)
		if talkSinceOf(d, "s1") != since {
			t.Errorf("%s: no longer asking", name)
		}
	}
}

// Where the screen can't be read, the card isn't pickable and the four keys answer it: a launcher's window, Ghostty,
// a headless run; Terminal and iTerm2 only through the Mac app. A screen that can't be read at the pick: the keys too.
func TestAPickWhereTheScreenCantBeReadIsTheKeys(t *testing.T) {
	saved := host
	t.Cleanup(func() { host = saved })
	host = nil
	for tab, want := range map[string]bool{"tmux:%1@/tmp/tmux-501/default": true, "wezterm:3": true, "kitty:4": true,
		"launcher:L1": false, "ghostty:AB12": false, "headless:3f2a": false, "tty:/dev/ttys003": false, "iterm:AB12": false} {
		if got := talkShowsScreen(tab); got != want {
			t.Errorf("%s: shows its screen %v, want %v", tab, got, want)
		}
	}
	host = talkMacApp{}
	for _, tab := range []string{"tty:/dev/ttys003", "iterm:AB12"} {
		if !talkShowsScreen(tab) {
			t.Errorf("%s under the Mac app: its screen isn't read", tab)
		}
	}
	host = nil
	d, tab := madeButtonDeck(t)
	talkPermission(d, 200)
	d.mu.Lock()
	asking := talkAskingOf(d.sessions.Get("s1"))
	d.mu.Unlock()
	if asking == nil || asking.Pickable {
		t.Fatalf("a launcher's card: %+v, want asking and not pickable", asking)
	}
	if got := talkPickAt(t, d, "s1", "1", asking.Since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != talkUseTheKeys {
		t.Errorf("a launcher: %d %s", got.Code, got.Body)
	}
	// Read at the pick, and it couldn't be: nothing pressed, still asking.
	talkScreenShows(t, "")
	talkScreenOf = func(Tab, Agent) ([]string, error) { return nil, errUnsupported }
	if got := talkPickAt(t, d, "s1", "1", asking.Since); got.Code != http.StatusConflict || strings.TrimSpace(got.Body.String()) != talkUseTheKeys {
		t.Errorf("a screen not read: %d %s", got.Code, got.Body)
	}
	nothingSent(t, tab, "a screen not read")
	if talkSinceOf(d, "s1") != asking.Since {
		t.Error("a screen not read stopped the desk asking")
	}
}

// talkMacApp stands in for the Mac app's bridge, on any computer: it answers nothing.
type talkMacApp struct{}

func (talkMacApp) call(bridgeRequest) (bridgeAnswer, error) { return bridgeAnswer{}, nil }

// What the menu looks like, from what Claude Code draws: its own, another, or none.
func TestTheMenuIsKnownOnTheScreen(t *testing.T) {
	permission := &talkAsking{Kind: "permission", Tool: "Bash", command: "touch x"}
	question := &talkAsking{Kind: "question", Questions: []talkQuestion{{Question: "Which color?"}}}
	write := &talkAsking{Kind: "permission", Tool: "Write", file: "/work/api/note.txt"}
	edit := &talkAsking{Kind: "permission", Tool: "Edit", file: "/work/api/note.txt"}
	long := "cd /work/api && go test -count=1 -race -run 'TestTheMenuIsKnownOnTheScreen' ./desktop/ 2>&1 | tail -40"
	wrapped := strings.Replace(permissionMenu(long), long, long[:50]+"\n "+long[50:], 1)
	cursorMoved := strings.Replace(strings.Replace(permissionMenu("touch x"), "❯ 1. Yes", "  1. Yes", 1), "  3. Yes, and switch", "❯ 3. Yes, and switch", 1)
	question300 := strings.Repeat("Which colour should the office's carpet be, given the light? ", 6)
	for _, c := range []struct {
		name   string
		screen string
		asking *talkAsking
		want   talkMenu
	}{
		{"a permission", permissionMenu("touch x"), permission, talkThisMenu},
		{"its cursor moved to choice 3", cursorMoved, permission, talkThisMenu},
		{"a command wrapped over two lines", wrapped, &talkAsking{Kind: "permission", Tool: "Bash", command: long}, talkThisMenu},
		{"Write's menu", fileMenu(false), write, talkThisMenu},
		{"Edit's menu", fileMenu(true), edit, talkThisMenu},
		{"Write's menu, its path cut at the start", strings.Replace(fileMenu(false), "\n note.txt\n", "\n …api/note.txt\n", 1), write, talkThisMenu},
		{"Write's menu for another file", fileMenu(false), &talkAsking{Kind: "permission", Tool: "Write", file: "/work/api/other.txt"}, talkOtherMenu},
		{"Write's menu for a file of the same ending", fileMenu(false), &talkAsking{Kind: "permission", Tool: "Write", file: "/work/api/old-note.txt"}, talkOtherMenu},
		{"Write's card, a command's menu (RT126P3c N1)", permissionMenu("rm -rf build"), write, talkOtherMenu},
		{"Edit's card, a command's menu", permissionMenu("rm -rf build"), edit, talkOtherMenu},
		{"a file's card, a question's menu", questionMenu("Which color?", "Red"), write, talkOtherMenu},
		{"a Write card with no path", fileMenu(false), &talkAsking{Kind: "permission", Tool: "Write"}, talkCantTell},
		{"a Bash card with no command", permissionMenu("touch x"), &talkAsking{Kind: "permission", Tool: "Bash"}, talkCantTell},
		{"a tool whose menu it can't tell (WebFetch)", permissionMenu("touch x"), &talkAsking{Kind: "permission", Tool: "WebFetch"}, talkCantTell},
		{"a tool it can't tell, a file's menu", fileMenu(true), &talkAsking{Kind: "permission", Tool: "NotebookEdit"}, talkCantTell},
		{"text over 20 lines in the input box, for a Write card (RT126P3c N3)", talkTop + "────\n❯ 1. Yes\n" + strings.Repeat("a line\n", 21) + "Esc to cancel\n────\n  ? for shortcuts\n",
			write, talkOtherMenu},
		{"Edit's menu for a command's card", fileMenu(true), permission, talkOtherMenu},
		{"Edit's menu for a question", fileMenu(true), question, talkOtherMenu},
		{"another command", permissionMenu("touch y"), permission, talkOtherMenu},
		{"the command only in the turn above, not in the box", permissionMenu("touch y"), &talkAsking{Kind: "permission", Tool: "Bash", command: "$ touch x"}, talkOtherMenu},
		{"a question", questionMenu("Which color?", "Red", "Green"), question, talkThisMenu},
		{"a question cut on the card", questionMenu(question300[:120]+"\n"+question300[120:], "Red"),
			&talkAsking{Kind: "question", Questions: []talkQuestion{{Question: clip(question300, talkQuestionMost)}}}, talkThisMenu},
		{"another question", questionMenu("Which fruit?", "Apple"), question, talkOtherMenu},
		{"the question's words only in the turn above, not in the box", strings.Replace(questionMenu("Which fruit?", "Apple"), "Use the Bash tool",
			"Which color? Use the Bash tool", 1), question, talkOtherMenu},
		{"a longer command on the screen than the card's", permissionMenu("git push --force origin main"), &talkAsking{Kind: "permission", Tool: "Bash", command: "git push"},
			talkOtherMenu},
		{"the input box", talkInputBox, permission, talkBoxWaits},
		{"the input box, a tool running", talkToolRuns, permission, talkBoxWorks},
		{"the input box, for a question", talkInputBox, question, talkBoxWaits},
		{"the input box under a status of three lines", strings.Replace(talkInputBox, "? for shortcuts", "? for shortcuts\n  ⧉ In notes.md\n  ✓ Update installed", 1),
			permission, talkBoxWaits},
		{"an old menu's words above the input box", "Do you want to proceed?\n❯ 1. Yes\n  2. No\nEsc to cancel\n" + talkInputBox, permission, talkBoxWaits},
		{"typed into the input box", talkTop + "────\n❯ 1. Yes, Esc to cancel\n────\n  ? for shortcuts\n", permission, talkBoxWaits},
		{"nothing", "", permission, talkCantTell},
		{"neither a footer nor the input box", talkTop, permission, talkCantTell},
		{"a question's choice 1 off the screen's top", "  2. Green\n     means Green\n  3. Type something.\n────\n  4. Chat about this\nEnter to select · Esc to cancel\n",
			question, talkCantTell},
		{"choice 1 over 40 lines above the footer", strings.Replace(questionMenu("Which color?", "Red", "Green"), "  2. Green", strings.Repeat("     more about red\n", 40)+"  2. Green", 1),
			question, talkCantTell},
		{"a footer in words it doesn't know", strings.Replace(permissionMenu("touch x"), "Esc to cancel", "Esc to dismiss", 1), permission, talkCantTell},
		{"the input box's lower rule too far up", strings.Replace(talkInputBox, "? for shortcuts", "a\nb\nc\nd", 1), permission, talkCantTell},
		{"a footer under the input box", strings.Replace(talkInputBox, "? for shortcuts", "Esc to cancel", 1), permission, talkCantTell},
		{"a rule and ❯ under it, but no rule above", "❯\n────\n  ? for shortcuts\n", permission, talkCantTell},
		{"a menu with no foot (before 2.1)", talkTop + " Bash command\n   touch x\n Do you want to proceed?\n ❯ 1. Yes\n   2. No, and tell Claude what to do differently (esc)\n",
			permission, talkThisMenu},
		{"no foot, and the input box under it", talkTop + " Do you want to proceed?\n ❯ 1. Yes\n────\n❯\n────\n", permission, talkBoxWaits},
		{"no foot, and a rule under it", talkTop + " Do you want to proceed?\n ❯ 1. Yes\n   2. No\n────\n", permission, talkCantTell},
	} {
		if got := talkMenuOn(screenEnd(c.screen, talkScreenLines), c.asking); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}
