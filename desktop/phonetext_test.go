package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The phone's text on this app (I5), as the Mac's PhoneMicTests check it: the same text rules, the same
// gates, one line typed or several as one paste, never a Return part way.

func TestOneLineIsFlattenedAsBefore(t *testing.T) {
	for in, want := range map[string]string{
		"  fix the  login\tbug \r\n ": "fix the login bug",
		"café, déjà vu":               "café, déjà vu",
		// A vertical tab or a form feed is a control character, not a line break.
		"go\x1b[A\x0bon\x0c": "go [A on",
	} {
		if got, ok := preparedPhoneText(in); !ok || got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	if isPhoneLines("fix the login bug") {
		t.Error("one line read as several")
	}
}

func TestSeveralLinesAreKeptAsTheyWere(t *testing.T) {
	code := "Change this:\n\n    def f():\n\t\treturn 1;  \n« fin » 👩‍💻 $(touch x) `y` \\"
	for in, want := range map[string]string{
		"  fix the\nlogin\tbug \r\n ":     "fix the\nlogin\tbug",
		"a\r\nb\rc d e\u0085f":            "a\nb\nc\nd\ne\nf",
		"\n\n" + code + "\n\n":            code,
		"one\x1b[201~\ntwo\x07":           "one [201~\ntwo",
		"\ufeffkept\u00a0\nU+FEFF is Cf ": "\ufeffkept\u00a0\nU+FEFF is Cf",
	} {
		if got, ok := preparedPhoneText(in); !ok || got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	if !isPhoneLines("one\ntwo") {
		t.Error("several lines read as one")
	}
	// The same size limit, the line breaks counted; the "!" rule on the first code point, once trimmed.
	if _, ok := preparedPhoneText(strings.Repeat("a\n", phoneTextLongest/2) + "a"); ok {
		t.Error("2,001 characters with their line breaks went")
	}
	if _, ok := preparedPhoneText(strings.Repeat("a\n", phoneTextLongest/2-1) + "aa"); !ok {
		t.Error("2,000 characters with their line breaks were refused")
	}
	if phoneTextRefusal(strings.Repeat("a\n", phoneTextLongest)) != tooLong {
		t.Error("too long not said")
	}
	if words, _ := preparedPhoneText("\n \n!́rm -rf x\nmore"); !runsInShell(words) {
		t.Error("a bang after blank lines got through")
	}
}

func TestNothingOrTooMuchIsRefusedAndSaysSo(t *testing.T) {
	for _, in := range []string{"", " \n\t ", strings.Repeat("a", phoneTextLongest+1)} {
		if _, ok := preparedPhoneText(in); ok {
			t.Errorf("%.20q went", in)
		}
	}
	if _, ok := preparedPhoneText(strings.Repeat("a", phoneTextLongest)); !ok {
		t.Error("2,000 characters refused")
	}
	if phoneTextRefusal(" \n ") != "Nothing to type" || phoneTextRefusal(strings.Repeat("a", 2001)) != tooLong {
		t.Error("the wrong refusal")
	}
	// One character can carry any number of marks: the bytes are capped too.
	heavy := "e" + strings.Repeat("́", 20000)
	if characterCount(heavy) != 1 || phoneTextRefusal(heavy) != tooLong {
		t.Error("a heavy character")
	}
	// 2,000 emoji of several pieces still go; so do accents and the joiner of 👩‍💻.
	if _, ok := preparedPhoneText(strings.Repeat("👍🏽", phoneTextLongest)); !ok {
		t.Error("2,000 emoji with a skin tone refused")
	}
	french := "Café à l’été, ça va ? « Œuvre » é è à ç ô œ 🎉👍🏽 👩‍💻 ❤️ 👨‍👩‍👧 🇫🇷"
	if got, _ := preparedPhoneText(french); got != french || characterCount("👩‍💻🇫🇷👍🏽é") != 4 {
		t.Errorf("%q, %d characters", got, characterCount("👩‍💻🇫🇷👍🏽é"))
	}
}

func TestTextStartingWithABangIsRefused(t *testing.T) {
	for _, bang := range []string{"!rm -rf build", "!́x", "!‍x", "!️x"} {
		if !runsInShell(bang) {
			t.Errorf("%q", bang)
		}
	}
	for _, text := range []string{"ship it!", "¡hola!", "！x"} {
		if runsInShell(text) {
			t.Errorf("%q", text)
		}
	}
}

func TestThePeekIsTheLastLinesAsText(t *testing.T) {
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, "line "+strings.Repeat("x", i%3)+string(rune('a'+i%26)))
	}
	if got := peekOf(strings.Join(lines, "\n") + "\n\n   \n"); !reflect.DeepEqual(got, lines[20:]) {
		t.Errorf("%q", got)
	}
	for screen, want := range map[string][]string{
		"> fix the login bug\x1b[0m\x07": {"> fix the login bug[0m"},
		"":                               {},
		"\n\n":                           {},
		strings.Repeat("\n", 12) + "──\n❯ hi‍\n──\n": {"──", "❯ hi", "──"},
		"a\r\nb c": {"a", "", "b", "c"},
		strings.Repeat("─", 500)[:3*peekWidth] + "\n":        {strings.Repeat("─", peekWidth)},
		"top\n" + strings.Repeat("─", 500) + "\n" + "bottom": {"top", strings.Repeat("─", peekWidth), "bottom"},
	} {
		if got := peekOf(screen); !reflect.DeepEqual(got, want) {
			t.Errorf("%.30q: %q, want %q", screen, got, want)
		}
	}
}

func TestAnOlderPagesTextSendsFromThreeWordsWithTheSettingOn(t *testing.T) {
	if !phoneTextSends("fix the bug", true) || phoneTextSends("fix it", true) || phoneTextSends("fix the bug", false) {
		t.Error("the three-word rule")
	}
}

// MARK: On a deck, its desk a Terminal tab under a stand-in Mac app

// phoneMac is a stand-in Mac app for the phone's text: it answers phone-text as told, and keeps what was asked.
// What the real app answers is phone-text.json's table (TestThePhonesTextTableThroughTheGoCore).
type phoneMac struct {
	mu     sync.Mutex
	asked  []bridgeRequest
	answer bridgeAnswer
	// during runs as the words are typed, as a desk starting to ask meanwhile.
	during func()
	// queued runs while phone-text waits for its terminal's queue, before the app asks the core again.
	queued func()
	// held counts the phone's texts the core said were to a desk asking, on the terminal's queue: nothing typed.
	held int
	// broken, if set, is how phone-text fails: the connection to the app, not the app's answer.
	broken error
}

// reply sets how phone-text is answered from now on.
func (m *phoneMac) reply(answer bridgeAnswer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answer = answer
}

// whileQueued sets what happens while the words wait for the terminal's queue.
func (m *phoneMac) whileQueued(queued func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queued = queued
}

// whileTyping sets what happens as the words are typed.
func (m *phoneMac) whileTyping(during func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.during = during
}

func (m *phoneMac) call(request bridgeRequest) (bridgeAnswer, error) {
	m.mu.Lock()
	m.asked = append(m.asked, request)
	answer, during, queued := m.answer, m.during, m.queued
	m.mu.Unlock()
	switch request.Action {
	case "phone-text":
		if m.broken != nil {
			return bridgeAnswer{}, m.broken
		}
		// As the Mac app: the agent's place, then on the terminal's queue the core asked the desk's state, right
		// before the words: asking, the app's no in the Swift engine's words.
		if !answer.OK {
			return answer, nil
		}
		if queued != nil {
			queued()
		}
		if request.deskState == nil || request.deskState() == Asking {
			m.mu.Lock()
			m.held++
			m.mu.Unlock()
			return bridgeAnswer{Error: answerFirst}, nil
		}
		if during != nil {
			during()
		}
		return answer, nil
	case "abilities":
		said := make([]tabAbilities, len(request.Tabs))
		for i := range said {
			said[i] = tabAbilities{Keys: true, Text: true, Fire: true, Screen: true}
		}
		return bridgeAnswer{OK: true, Abilities: said}, nil
	case "peek":
		return bridgeAnswer{OK: true, Peek: []string{"❯ hello"}}, nil
	}
	return bridgeAnswer{OK: true}, nil
}

// actions is what was asked of the app's terminal, as "action text" or "action key".
func (m *phoneMac) actions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var done []string
	for _, r := range m.asked {
		switch r.Action {
		case "phone-text":
			done = append(done, "phone-text "+r.Tab+" "+string(r.Agent)+" "+r.Text)
		case "press":
			done = append(done, "press "+string(r.Key))
		case "peek":
			done = append(done, "peek "+r.Tab)
		}
	}
	return done
}

// phoneDeck is a deck whose desk s1 is a Claude Code session, done with its turn, in Terminal's tab
// tty:/dev/ttys003, under the stand-in Mac app.
func phoneDeck(t *testing.T) (*Deck, *phoneMac) {
	t.Helper()
	mac := &phoneMac{answer: bridgeAnswer{OK: true, Pasted: "pasted"}}
	saved, settle := host, phoneTextSettle
	t.Cleanup(func() { host, phoneTextSettle = saved, settle })
	host, phoneTextSettle = mac, time.Millisecond
	d := newTestDeck(t)
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api"}, "claude", "tty:/dev/ttys003", "", 10)
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "tty:/dev/ttys003", "", 20)
	return d, mac
}

func asksPermission(d *Deck, at float64) {
	d.Ingest(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "touch x"}}, "claude", "tty:/dev/ttys003", "", at)
}

func sendIt() *bool { on := true; return &on }

func TestSeveralLinesGoAsOnePasteOrNotAtAll(t *testing.T) {
	text := "Deux lignes :\n    « ça marche » 👩‍💻 fin;"
	d, mac := phoneDeck(t)
	if status, message := d.TypePhoneText(text, "s1", sendIt()); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	if got := mac.actions(); !reflect.DeepEqual(got, []string{"phone-text tty:/dev/ttys003 claude " + text, "press enter"}) {
		t.Errorf("byte for byte, one paste, then Return once: %q", got)
	}
}

func TestSeveralLinesNotTakenTypeNothingAndSendNothing(t *testing.T) {
	d, mac := phoneDeck(t)
	mac.reply(bridgeAnswer{Error: noLines})
	if status, message := d.TypePhoneText("Deux lignes :\n    fin;", "s1", sendIt()); status != http.StatusConflict || message != noLines {
		t.Errorf("not taken: %d %s", status, message)
	}
	if got := mac.actions(); len(got) != 1 {
		t.Errorf("no Return after nothing typed: %q", got)
	}
}

// A desk asking something gets nothing, several lines or one.
func TestNothingGoesToADeskAskingYou(t *testing.T) {
	d, mac := phoneDeck(t)
	asksPermission(d, 30)
	for _, words := range []string{"Deux lignes :\n    fin;", "one line"} {
		if status, message := d.TypePhoneText(words, "s1", sendIt()); status != http.StatusConflict || message != answerFirst {
			t.Errorf("asking: %d %s", status, message)
		}
	}
	if got := mac.actions(); len(got) != 0 {
		t.Errorf("typed into a desk that asks: %q", got)
	}
}

func TestTheRulesComeBeforeAnyKey(t *testing.T) {
	d, mac := phoneDeck(t)
	for text, want := range map[string]int{
		"  !ls -la":               http.StatusBadRequest,
		" \n ":                    http.StatusBadRequest,
		strings.Repeat("a", 2001): http.StatusBadRequest,
		"one line, typed only":    http.StatusNoContent,
		"Bonjour\nl’équipe 👩‍💻 ;": http.StatusNoContent,
	} {
		if status, message := d.TypePhoneText(text, "s1", nil); status != want {
			t.Errorf("%.20q: %d %s", text, status, message)
		}
	}
	if status, _ := d.TypePhoneText("hello", "nope", nil); status != http.StatusNotFound {
		t.Errorf("no such session: %d", status)
	}
	// Typed only (no send, an older page's two words): no Return.
	for _, action := range mac.actions() {
		if strings.HasPrefix(action, "press") {
			t.Errorf("a Return went: %q", mac.actions())
		}
	}
	d.mu.Lock()
	d.config.PhoneMicEnabled = false
	d.mu.Unlock()
	if status, message := d.TypePhoneText("hello", "s1", nil); status != http.StatusForbidden || message != phoneTextOff {
		t.Errorf("the switch off: %d %s", status, message)
	}
}

// The desk began asking while the words waited for the key worker: checked again right before them.
func TestADeskThatStartedAskingMeanwhileGetsNothing(t *testing.T) {
	d, mac := phoneDeck(t)
	hold := make(chan struct{})
	d.work <- func() { <-hold }
	answered := make(chan [2]any, 1)
	go func() {
		status, message := d.TypePhoneText("fix the login bug", "s1", sendIt())
		answered <- [2]any{status, message}
	}()
	time.Sleep(50 * time.Millisecond)
	asksPermission(d, 30)
	close(hold)
	if got := <-answered; got[0] != http.StatusConflict || got[1] != answerFirst {
		t.Errorf("%v", got)
	}
	if got := mac.actions(); len(got) != 0 {
		t.Errorf("typed: %q", got)
	}
}

// Under the Mac app, the desk began asking while the words waited for its terminal's queue: the app asks the
// core from there, right before them, as the Swift engine looks at its sessions on that queue. Nothing goes.
func TestUnderTheMacAppTheCoreIsAskedAgainOnTheTerminalsQueue(t *testing.T) {
	d, mac := phoneDeck(t)
	mac.whileQueued(func() { asksPermission(d, 30) })
	if status, message := d.TypePhoneText("fix the login bug", "s1", sendIt()); status != http.StatusConflict || message != answerFirst {
		t.Errorf("%d %s", status, message)
	}
	mac.mu.Lock()
	held := mac.held
	mac.mu.Unlock()
	if held != 1 {
		t.Errorf("held on the queue %d times, want once", held)
	}
	for _, action := range mac.actions() {
		if strings.HasPrefix(action, "press") {
			t.Errorf("a Return went: %q", mac.actions())
		}
	}
	// Still free there, the words go: TestSeveralLinesGoAsOnePasteOrNotAtAll, on the same stand-in.
}

// The phone's text on both engines, one table (Tests/fixtures/bridge/phone-text.json): through the Go core,
// with a stand-in Mac app that answers each request with the fixture's answer, as PhoneMicTests.swift shows
// the real app's Deck.typePhoneWords gives it. The core sends exactly the fixture's request, answers the app's
// questions with its own desk's state, tells the phone what the fixture says, and presses Return only after
// the words went.
func TestThePhonesTextTableThroughTheGoCore(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "bridge", "phone-text.json"))
	if err != nil {
		t.Fatal(err)
	}
	type said struct {
		Status int
		Says   string
	}
	var fixture struct {
		Cases []struct {
			Name, Agent, Desk string
			Asked             []SessionState
			Request, Answer   json.RawMessage
			Swift, Go         said
			Typed, Returns    bool
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Cases) < 6 {
		t.Fatalf("the table: %v, %d cases", err, len(fixture.Cases))
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var answer bridgeAnswer
			if err := json.Unmarshal(c.Answer, &answer); err != nil {
				t.Fatal(err)
			}
			mac := &tableMac{answer: answer, asks: len(c.Asked)}
			saved, settle := host, phoneTextSettle
			t.Cleanup(func() { host, phoneTextSettle = saved, settle })
			host, phoneTextSettle = mac, time.Millisecond
			d := newTestDeck(t)
			d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api"}, c.Agent, "tty:/dev/ttys003", "", 10)
			if c.Desk != string(Thinking) {
				d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, c.Agent, "tty:/dev/ttys003", "", 20)
			}
			if state := d.deskState("s1"); string(state) != c.Desk {
				t.Fatalf("the desk is %q, want %q", state, c.Desk)
			}
			var text struct{ Text string }
			_ = json.Unmarshal(c.Request, &text)
			status, message := d.TypePhoneText(text.Text, "s1", sendIt())
			if status != c.Go.Status || message != c.Go.Says {
				t.Errorf("told the phone %d %q, want %d %q", status, message, c.Go.Status, c.Go.Says)
			}
			// Only where the words may have gone (⌘V not read in time) do the engines say it in their own words.
			if c.Go != c.Swift && (c.Go.Status < 500 || c.Swift.Status < 500) {
				t.Errorf("the engines differ on a refusal or a success: %v, %v", c.Swift, c.Go)
			}
			mac.mu.Lock()
			defer mac.mu.Unlock()
			if len(mac.texts) != 1 {
				t.Fatalf("asked the app %d times", len(mac.texts))
			}
			sent, _ := json.Marshal(mac.texts[0])
			var asked, want map[string]any
			_ = json.Unmarshal(sent, &asked)
			_ = json.Unmarshal(c.Request, &want)
			if !reflect.DeepEqual(asked, want) {
				t.Errorf("the core asks %s, the fixture %s", sent, c.Request)
			}
			if !slices.Equal(mac.states, c.Asked) {
				t.Errorf("the core said the desk was %q, want %q", mac.states, c.Asked)
			}
			if mac.returns != map[bool]int{true: 1, false: 0}[c.Returns] {
				t.Errorf("%d Returns, want one only after the words went (%v)", mac.returns, c.Returns)
			}
		})
	}
}

// tableMac is a stand-in Mac app giving one answer to phone-text, after asking the core the desk's state asks
// times, as the real app asks it from the terminal's queue. It keeps what it was asked.
type tableMac struct {
	mu      sync.Mutex
	answer  bridgeAnswer
	asks    int
	texts   []bridgeRequest
	states  []SessionState
	returns int
}

func (m *tableMac) call(request bridgeRequest) (bridgeAnswer, error) {
	switch request.Action {
	case "phone-text":
		var states []SessionState
		for range m.asks {
			states = append(states, request.deskState())
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.texts = append(m.texts, request)
		m.states = states
		return m.answer, nil
	case "press":
		m.mu.Lock()
		defer m.mu.Unlock()
		if request.Key == Enter {
			m.returns++
		}
	case "abilities":
		said := make([]tabAbilities, len(request.Tabs))
		for i := range said {
			said[i] = tabAbilities{Keys: true, Text: true, Fire: true, Screen: true}
		}
		return bridgeAnswer{OK: true, Abilities: said}, nil
	}
	return bridgeAnswer{OK: true}, nil
}

// A failure once the words were on their way says what happened, never "Tab not found".
func TestAFailureSaysWhatHappened(t *testing.T) {
	d, mac := phoneDeck(t)
	mac.reply(bridgeAnswer{Error: "Terminal isn't answering, so nothing was typed. Try again in a moment."})
	if status, message := d.TypePhoneText("hello there", "s1", sendIt()); status != http.StatusConflict ||
		message != "Terminal isn't answering, so nothing was typed. Try again in a moment." {
		t.Errorf("the Mac app's no: %d %s", status, message)
	}
	mac.reply(bridgeAnswer{OK: true, Pasted: "something else"})
	if status, message := d.TypePhoneText("hello there", "s1", sendIt()); status != http.StatusBadGateway || message != pasteUnsaid {
		t.Errorf("not said: %d %s", status, message)
	}
	for _, action := range mac.actions() {
		if strings.HasPrefix(action, "press") {
			t.Errorf("a Return went: %q", mac.actions())
		}
	}
}

// A failure that can come after the words went (the connection to the app lost, a pane's tool failing as it
// pastes or sends, a launcher gone once it was told) is a 502 with "may have landed, no Return went": never
// "Tab not found", which would have the phone put the words back and invite a second send.
func TestAFailureAfterTheWordsMayHaveLandedAnswers502(t *testing.T) {
	check := func(t *testing.T, label string, status int, message string) {
		t.Helper()
		if status != http.StatusBadGateway || message != pasteUnsaid {
			t.Errorf("%s: %d %s", label, status, message)
		}
	}
	t.Run("the connection to the Mac app", func(t *testing.T) {
		for name, broken := range map[string]error{
			"lost":      errors.New("read unix: connection reset by peer"),
			"deadline":  os.ErrDeadlineExceeded,
			"asked two": errors.New("the Mac app asked twice"),
		} {
			d, mac := phoneDeck(t)
			mac.mu.Lock()
			mac.broken = broken
			mac.mu.Unlock()
			status, message := d.TypePhoneText("hello there", "s1", sendIt())
			check(t, name, status, message)
			for _, action := range mac.actions() {
				if strings.HasPrefix(action, "press") {
					t.Errorf("%s: a Return went: %q", name, mac.actions())
				}
			}
		}
		// The app's own no, and an agent not in front, still say what they said (above).
	})
	t.Run("tmux, WezTerm and kitty", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("fake tools are shell scripts")
		}
		dir := t.TempDir()
		log := filepath.Join(dir, "log")
		// Each tool takes what it is given, and fails on the paste or send itself.
		script := "#!/bin/sh\necho \"$0 $*\" >> " + log + "\ncat > /dev/null\ncase \" $* \" in *\" paste-buffer \"*|*\" send-text \"*) exit 1;; esac\nexit 0\n"
		for _, tool := range []string{"tmux", "wezterm", "kitty"} {
			if err := os.WriteFile(filepath.Join(dir, tool), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		saved, settle := host, phoneTextSettle
		host, phoneTextSettle = nil, time.Millisecond
		t.Cleanup(func() { host, phoneTextSettle = saved, settle })
		for id, tab := range map[string]string{"t": "tmux:%1@" + filepath.Join(dir, "sock"), "w": "wezterm:3@" + filepath.Join(dir, "wez"), "k": "kitty:4@unix:" + filepath.Join(dir, "kit")} {
			d := newTestDeck(t)
			d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": id, "cwd": "/work/api"}, "claude", tab, "", 20)
			status, message := d.TypePhoneText("hello there", id, sendIt())
			check(t, tab, status, message)
		}
		logged, _ := os.ReadFile(log)
		if strings.Contains(string(logged), "send-keys") || strings.Contains(string(logged), "\r") {
			t.Errorf("a Return went: %q", logged)
		}
		if !strings.Contains(string(logged), "paste-buffer") || strings.Count(string(logged), "send-text") != 2 {
			t.Errorf("the tools weren't asked as meant: %q", logged)
		}
	})
	t.Run("a launcher gone once it was told", func(t *testing.T) {
		saved, settle, wait := host, phoneTextSettle, launcherPasteWait
		host, phoneTextSettle, launcherPasteWait = nil, time.Millisecond, 5*time.Second
		t.Cleanup(func() { host, phoneTextSettle, launcherPasteWait = saved, settle, wait })
		d := newTestDeck(t)
		tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 20)
		go func() {
			command := <-tab.commands
			if command.PhoneText == "" {
				return
			}
			d.launcherGone("L1", tab) // closed once the words were sent
		}()
		status, message := d.TypePhoneText("hello there", "s1", sendIt())
		check(t, "launcher", status, message)
	})
}

// A launcher that doesn't say in time may still paste: the phone is told the words may have landed, and no
// Return goes. Its late answer finds nothing, and an older launcher's look-only answer types nothing more.
func TestALauncherThatDoesntSaySaysSo(t *testing.T) {
	saved, settle, wait := host, phoneTextSettle, launcherPasteWait
	host, phoneTextSettle, launcherPasteWait = nil, time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { host, phoneTextSettle, launcherPasteWait = saved, settle, wait })
	d := newTestDeck(t)
	tab := d.launcherConnected("L1", Claude, "/work/api", "", "")
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 20)
	if status, message := d.TypePhoneText("hello there", "s1", sendIt()); status != http.StatusServiceUnavailable || message != launcherLate {
		t.Errorf("late: %d %s", status, message)
	}
	command := <-tab.commands
	if command.PhoneText != "hello there" || command.Look == "" {
		t.Fatalf("%+v", command)
	}
	if d.launcherScreenAnswered("L1", tab.screenSecret, screenAnswer{Look: command.Look, State: "idle", Pasted: "pasted"}) {
		t.Error("a late answer was taken")
	}
	go func() {
		command := <-tab.commands
		d.launcherScreenAnswered("L1", tab.screenSecret, screenAnswer{Look: command.Look, State: "idle"})
	}()
	if status, message := d.TypePhoneText("hello there", "s1", sendIt()); status != http.StatusBadGateway || message != pasteUnsaid {
		t.Errorf("an older launcher: %d %s", status, message)
	}
	select {
	case extra := <-tab.commands:
		t.Errorf("more went: %+v", extra)
	default:
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.lookPasted)+len(d.looks)+len(d.lookBracketed) != 0 {
		t.Errorf("left behind: %v %v %v", d.lookPasted, d.looks, d.lookBracketed)
	}
}

// A launcher's ordinary looks, mail's and hiring's, leave nothing where a phone paste's answer goes, before or after one.
func TestALaunchersLooksLeaveNothingBehind(t *testing.T) {
	saved, settle := host, phoneTextSettle
	host, phoneTextSettle = nil, time.Millisecond
	t.Cleanup(func() { host, phoneTextSettle = saved, settle })
	d := newTestDeck(t)
	newFakeLauncherAs(t, d, "L1", "/work/api", "", "", "idle-empty", true)
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 20)
	look := func() {
		if _, err := d.lookAtLauncher("L1", 2*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	look()
	if status, message := d.TypePhoneText("one line", "s1", nil); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	for range 3 {
		look()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.lookPasted)+len(d.looks)+len(d.lookBracketed) != 0 {
		t.Errorf("left behind: %v %v %v", d.lookPasted, d.looks, d.lookBracketed)
	}
}

// Asking once the words were in: they stay, and the Return is withheld.
func TestAReturnIsWithheldFromADeskThatBeganAsking(t *testing.T) {
	d, mac := phoneDeck(t)
	mac.whileTyping(func() { asksPermission(d, 40) })
	if status, message := d.TypePhoneText("fix the login bug", "s1", sendIt()); status != http.StatusConflict || message != typedNotSent {
		t.Errorf("%d %s", status, message)
	}
	if got := mac.actions(); len(got) != 1 {
		t.Errorf("a Return went to a desk that asks: %q", got)
	}
}

func TestAnAgentThatEndedGetsNothing(t *testing.T) {
	d, mac := phoneDeck(t)
	mac.reply(bridgeAnswer{Error: "That agent has ended.", Place: "gone"})
	if status, message := d.TypePhoneText("hello there", "s1", sendIt()); status != http.StatusGone {
		t.Errorf("gone: %d %s", status, message)
	}
	if got := mac.actions(); len(got) != 1 {
		t.Errorf("a Return went: %q", got)
	}
}

func TestAnAgentBehindItsShellGetsNothing(t *testing.T) {
	d, mac := phoneDeck(t)
	mac.reply(bridgeAnswer{Error: "That agent isn't in front in its terminal: bring it back with fg.", Place: "behind"})
	if status, message := d.TypePhoneText("hello there", "s1", sendIt()); status != http.StatusConflict || message != notFrontFg {
		t.Errorf("behind: %d %s", status, message)
	}
	if got := mac.actions(); len(got) != 1 {
		t.Errorf("a Return went: %q", got)
	}
}

// WezTerm and kitty can never take several lines as one paste: refused before the terminal is asked.
// A terminal this app doesn't type into says so.
func TestTerminalsThatCantTakeLinesRefuseAtOnce(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	for id, tab := range map[string]string{"w": "wezterm:3@/nowhere/wez", "k": "kitty:4@unix:/nowhere/kitty", "x": "tty:/dev/ttys009"} {
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": id, "cwd": "/work/api"}, "claude", tab, "", 20)
	}
	for _, id := range []string{"w", "k"} {
		if status, message := d.TypePhoneText("one\ntwo", id, nil); status != http.StatusConflict || message != noLines {
			t.Errorf("%s: %d %s", id, status, message)
		}
	}
	if status, message := d.TypePhoneText("one line", "x", nil); status != http.StatusConflict || message != notHere {
		t.Errorf("a Terminal tab with no Mac app: %d %s", status, message)
	}
}

// A launcher's window (Windows and Linux): the launcher pastes, as for mail, and says whether it did.
func TestTheLauncherPastesThePhonesTextAndSaysSo(t *testing.T) {
	saved, settle := host, phoneTextSettle
	host, phoneTextSettle = nil, time.Millisecond
	t.Cleanup(func() { host, phoneTextSettle = saved, settle })
	text := "Deux lignes :\n    « ça marche » 👩‍💻 fin;"
	for _, bracketed := range []bool{true, false} {
		d := newTestDeck(t)
		f := newFakeLauncherAs(t, d, "L1", "/work/api", "", "", "idle-empty", bracketed)
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 20)
		status, message := d.TypePhoneText(text, "s1", sendIt())
		if bracketed {
			if status != http.StatusNoContent {
				t.Fatalf("%d %s", status, message)
			}
			want := []string{pasteStart + text + pasteEnd, keyBytes[Enter]}
			if got := f.writtenAfter(2); !reflect.DeepEqual(got, want) {
				t.Errorf("one paste, then Return: %q", got)
			}
			continue
		}
		if status != http.StatusConflict || message != noLines {
			t.Errorf("no bracketed paste: %d %s", status, message)
		}
		if got := f.writtenAfter(1); len(got) != 0 {
			t.Errorf("typed without bracketed paste: %q", got)
		}
		// One line goes as typed text there.
		if status, message := d.TypePhoneText("one line only", "s1", nil); status != http.StatusNoContent {
			t.Errorf("one line: %d %s", status, message)
		}
		if got := f.writtenAfter(1); !reflect.DeepEqual(got, []string{"one line only"}) {
			t.Errorf("%q", got)
		}
	}
}

// MARK: Through the router

// phoneAsk sends a request as a phone would, from host (an address the server reads as the Wi-Fi's).
func phoneAsk(t *testing.T, d *Deck, method, path, key string, body any, header http.Header) (int, string) {
	t.Helper()
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Host = "192.168.1.20:7710"
	r.RemoteAddr = "192.168.1.30:50000"
	if key != "" {
		r.Header.Set(hookKeyHeader, key)
	}
	for name, values := range header {
		r.Header[name] = values
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w.Code, strings.TrimSpace(w.Body.String())
}

func TestTheWiFiTypesWithAPairedPhonesKeyOnly(t *testing.T) {
	d, mac := phoneDeck(t)
	d.mu.Lock()
	key := d.access.Phones.Pair("Phone")
	window := d.access.WindowKey
	d.mu.Unlock()
	text := map[string]any{"text": "first\n  second 👩‍💻", "send": false}
	for _, who := range []string{"", "wrong-key-wrong-key", window} {
		if status, _ := phoneAsk(t, d, http.MethodPost, "/dictate?session=s1", who, text, nil); status != http.StatusUnauthorized {
			t.Errorf("key %q: %d", who, status)
		}
	}
	if status, message := phoneAsk(t, d, http.MethodPost, "/dictate?session=s1", key, text, nil); status != http.StatusNoContent {
		t.Fatalf("the paired key: %d %s", status, message)
	}
	if status, _ := phoneAsk(t, d, http.MethodPost, "/dictate?session=s1", key, map[string]any{"text": "!ls"}, nil); status != http.StatusBadRequest {
		t.Errorf("a bang: %d", status)
	}
	if got := mac.actions(); !reflect.DeepEqual(got, []string{"phone-text tty:/dev/ttys003 claude first\n  second 👩‍💻"}) {
		t.Errorf("%q", got)
	}
	// Reading a terminal stays Tailscale's: never over the Wi-Fi.
	if status, message := phoneAsk(t, d, http.MethodPost, "/peek?session=s1", key, nil, nil); status != http.StatusForbidden || message != peekTailnet {
		t.Errorf("peek: %d %s", status, message)
	}
	if status, _ := phoneAsk(t, d, http.MethodGet, "/choice?session=s1", key, nil, nil); status != http.StatusForbidden {
		t.Errorf("choice: %d", status)
	}
	// A typed answer: one line, option=other only; no menu is followed here, so none is answered.
	for path, want := range map[string]int{"/choose?session=s1": 400, "/choose?session=s1&option=1": 400} {
		if status, _ := phoneAsk(t, d, http.MethodPost, path, key, map[string]any{"text": "yes"}, nil); status != want {
			t.Errorf("%s: %d", path, status)
		}
	}
	if status, message := phoneAsk(t, d, http.MethodPost, "/choose?session=s1&option=other&choice=c1", key, map[string]any{"text": "a\nb"}, nil); status != 400 || message != oneLineAnswer {
		t.Errorf("several lines: %d %s", status, message)
	}
	if status, message := phoneAsk(t, d, http.MethodPost, "/choose?session=s1&option=other&choice=c1", key, map[string]any{"text": "use main"}, nil); status != 409 || message != choiceGone {
		t.Errorf("one line: %d %s", status, message)
	}
}

// The office's own window, on this computer, types nothing: it never did.
func TestTheOfficeWindowTypesNothing(t *testing.T) {
	d, mac := phoneDeck(t)
	server := newLocalServer(t, d)
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	for _, path := range []string{"/dictate?session=s1", "/choose?session=s1&option=other"} {
		request, _ := http.NewRequest(http.MethodPost, server.URL+path+"&t="+window, strings.NewReader(`{"text":"hello"}`))
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Errorf("%s: %d", path, response.StatusCode)
		}
	}
	if got := mac.actions(); len(got) != 0 {
		t.Errorf("%q", got)
	}
}

// Through Tailscale the peek reads the input line, once a second at most.
func TestThePeekOverTailscaleOnceASecond(t *testing.T) {
	d, mac := phoneDeck(t)
	d.mu.Lock()
	key := d.access.Phones.Pair("Phone")
	d.mu.Unlock()
	// As `tailscale serve` hands a request on: from this computer's loopback, with a proxy's headers.
	r := httptest.NewRequest(http.MethodPost, "/peek?session=s1", nil)
	r.Host, r.RemoteAddr = "127.0.0.1:7710", "127.0.0.1:50000"
	r.Header.Set(hookKeyHeader, key)
	r.Header.Set("X-Forwarded-For", "100.101.102.103")
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"lines":["❯ hello"]}` {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if status, _, message := d.Peek("s1"); status != http.StatusTooManyRequests || message != peekTooSoon {
		t.Errorf("a second look at once: %d %s", status, message)
	}
	if got := mac.actions(); !reflect.DeepEqual(got, []string{"peek tty:/dev/ttys003"}) {
		t.Errorf("%q", got)
	}
	if status, message := d.ChoiceDetail("s1"); status != http.StatusNotFound || message != notAsking {
		t.Errorf("choice: %d %s", status, message)
	}
}

func TestTailscaleIsToItsAddressFromATailnetAddress(t *testing.T) {
	saved := tailscaleAddresses
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	t.Cleanup(func() { tailscaleAddresses = saved })
	ask := func(remote, local string, header map[string]string) bool {
		r := httptest.NewRequest(http.MethodPost, "/peek", nil)
		r.RemoteAddr = remote
		if local != "" {
			r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP(local), Port: 7710}))
		}
		for name, value := range header {
			r.Header.Set(name, value)
		}
		return fromTailnet(r)
	}
	if !ask("100.101.102.103:5000", "100.64.1.2", nil) {
		t.Error("a tailnet device to this computer's Tailscale address")
	}
	if ask("100.101.102.103:5000", "192.168.1.20", nil) {
		t.Error("a 100.64/10 address on the Wi-Fi isn't Tailscale")
	}
	if ask("192.168.1.30:5000", "192.168.1.20", nil) || ask("127.0.0.1:5000", "127.0.0.1", nil) {
		t.Error("the Wi-Fi, or this computer, isn't Tailscale")
	}
	if !ask("127.0.0.1:5000", "127.0.0.1", map[string]string{"X-Forwarded-For": "100.101.102.103"}) {
		t.Error("tailscale serve's hand-on")
	}
}

// On a Mac, Tailscale's adapter is a utunN like any VPN's: it's told by its addresses, 100.64/10 with
// Tailscale's own IPv6 range. And `tailscale serve` hands the phone's request on to its ts.net name, let in
// as the Mac app lets it in: from this computer's loopback, with the phone's text on.
func TestOnAMacTailscaleIsToldByItsAddressesAndServedByItsName(t *testing.T) {
	saved := macTailscale
	t.Cleanup(func() { macTailscale = saved })
	macTailscale = true
	ip := net.ParseIP
	utun := []net.IP{ip("fe80::1"), ip("100.101.102.103"), ip("fd7a:115c:a1e0::1234")}
	for _, c := range []struct {
		name string
		up   bool
		ips  []net.IP
		want []string
	}{
		{"utun4", true, utun, []string{"100.101.102.103"}},
		{"utun4", false, utun, nil},
		// Another VPN's, or a provider's, 100.64/10 alone isn't Tailscale's; nor is a neighbouring IPv6 range.
		{"utun4", true, utun[:2], nil},
		{"utun5", true, []net.IP{ip("100.101.102.103"), ip("fd7a:115c:a1e1::1")}, nil},
		{"en0", true, []net.IP{ip("192.168.1.20"), ip("fd7a:115c:a1e0::1")}, nil},
	} {
		if got := tailscaleFrom(c.name, c.up, c.ips); !slices.Equal(got, c.want) {
			t.Errorf("%s up=%v %v: %v, want %v", c.name, c.up, c.ips, got, c.want)
		}
	}
	d, _ := phoneDeck(t)
	d.mu.Lock()
	key := d.access.Phones.Pair("Phone")
	d.mu.Unlock()
	peek := func(host, remote string) (int, string) {
		r := httptest.NewRequest(http.MethodPost, "/peek?session=s1", nil)
		r.Host, r.RemoteAddr = host, remote
		r.Header.Set(hookKeyHeader, key)
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	if status, body := peek("my-mac.tail1234.ts.net", "127.0.0.1:50000"); status != http.StatusOK || body != `{"lines":["❯ hello"]}` {
		t.Fatalf("tailscale serve's hand-on: %d %s", status, body)
	}
	// Refused before the peek is counted: a ts.net name from elsewhere, another name, the phone's text off.
	for _, c := range [][2]string{{"my-mac.tail1234.ts.net", "100.101.102.103:5000"}, {"my-mac.example.net", "127.0.0.1:50000"}, {"ts.net", "127.0.0.1:50000"}} {
		if status, _ := peek(c[0], c[1]); status != http.StatusForbidden {
			t.Errorf("%v: %d", c, status)
		}
	}
	d.mu.Lock()
	d.config.PhoneMicEnabled = false
	d.mu.Unlock()
	if status, _ := peek("my-mac.tail1234.ts.net", "127.0.0.1:50000"); status != http.StatusForbidden {
		t.Errorf("the phone's text off: %d", status)
	}
	// Elsewhere than a Mac, as before: a ts.net name isn't let in.
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	macTailscale = false
	if status, _ := peek("my-mac.tail1234.ts.net", "127.0.0.1:50000"); status != http.StatusForbidden {
		t.Errorf("not a Mac: %d", status)
	}
}

// A proxy's hand-on is never this computer's own request, as on the Mac (Addressing.isProxied): the office
// window's key isn't taken with a proxy's headers, so it reads no peek, and setup isn't there.
func TestAProxiedRequestIsntThisComputers(t *testing.T) {
	d, mac := phoneDeck(t)
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	ask := func(method, path string, proxied bool) int {
		r := httptest.NewRequest(method, path, nil)
		r.Host, r.RemoteAddr = "localhost:7710", "127.0.0.1:50000"
		r.Header.Set(hookKeyHeader, window)
		if proxied {
			r.Header["Forwarded"] = []string{""} // even empty, as the Mac reads it
		}
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		return w.Code
	}
	for _, mac := range []bool{true, false} {
		saved := macTailscale
		macTailscale = mac
		if status := ask(http.MethodPost, "/peek?session=s1", true); status != http.StatusUnauthorized {
			t.Errorf("mac=%v: the window's key with a proxy's header: %d", mac, status)
		}
		if status := ask(http.MethodGet, "/setup", true); status != http.StatusNotFound {
			t.Errorf("mac=%v: setup with a proxy's header: %d", mac, status)
		}
		macTailscale = saved
	}
	// The window itself, with no proxy: this computer's, and still no peek, as it isn't Tailscale.
	if status := ask(http.MethodPost, "/peek?session=s1", false); status != http.StatusForbidden {
		t.Errorf("the window: %d", status)
	}
	if got := mac.actions(); len(got) != 0 {
		t.Errorf("%q", got)
	}
}

func TestTheSnapshotSaysWhereThePhonesTextGoes(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "tmux:%3@/nowhere/s", "", 20)
	look := func() (map[string]any, any) {
		d.mu.Lock()
		defer d.mu.Unlock()
		var snapshot map[string]any
		_ = json.Unmarshal(d.snapshot(), &snapshot)
		for _, key := range snapshot["keys"].([]any) {
			if session, ok := key.(map[string]any)["session"].(map[string]any); ok {
				return snapshot["phoneMic"].(map[string]any), session["phoneMic"]
			}
		}
		return snapshot["phoneMic"].(map[string]any), nil
	}
	if mic, desk := look(); mic["on"] != true || mic["autoSend"] != false || desk != true {
		t.Errorf("on by default, as on the Mac: %v, %v", mic, desk)
	}
	d.mu.Lock()
	d.config.PhoneMicEnabled = false
	d.mu.Unlock()
	if mic, desk := look(); mic["on"] != false || desk != false {
		t.Errorf("off: %v, %v", mic, desk)
	}
}

func TestThePhoneMicSettingIsReadUnderTheMacsNames(t *testing.T) {
	isolate(t)
	if c := loadConfig(); !c.PhoneMicEnabled || c.PhoneMicAutoSend {
		t.Error("on by default, and asks before sending")
	}
	if err := os.WriteFile(configFile, []byte(`{"phoneMicEnabled": false, "phoneMicSkipsConfirm": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if c := loadConfig(); c.PhoneMicEnabled || !c.PhoneMicAutoSend {
		t.Error("the file's settings")
	}
}

// MARK: Real tmux, on a server of the test's own

// rawReader is a pane running a reader that records every byte it's given, in raw mode, having asked for
// bracketed paste (as an agent does) or not.
func rawReader(t *testing.T, bracketed bool) (commandTab, string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("no tmux here")
	}
	// Short: a socket's path has a length limit. The system's temporary folder, never /tmp itself.
	dir, err := os.MkdirTemp("", "i5t")
	if err != nil {
		t.Fatal(err)
	}
	socket, out := filepath.Join(dir, "s"), filepath.Join(dir, "out")
	tmux := func(args ...string) string {
		output, err := exec.Command("tmux", append([]string{"-S", socket, "-f", "/dev/null"}, args...)...).Output()
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return strings.TrimSpace(string(output))
	}
	ask := ""
	if bracketed {
		ask = `printf '\033[?2004h'; `
	}
	tmux("new-session", "-d", "-x", "80", "-y", "24", "sh", "-c", "stty raw -echo; "+ask+"touch "+out+".ready; exec cat > "+out)
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-S", socket, "kill-server").Run()
		os.RemoveAll(dir)
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(out + ".ready"); err == nil {
			break
		}
	}
	time.Sleep(100 * time.Millisecond) // the reader's cat in front
	pane := commandTab{kind: "tmux", id: tmux("display-message", "-p", "#{pane_id}"), socket: socket}
	if bracketed && tmux("display-message", "-p", "-t", pane.id, "#{bracket_paste_flag}") != "1" {
		t.Skip("this tmux has no bracket_paste_flag (3.7 and later)")
	}
	return pane, out
}

func readAfter(t *testing.T, path string, want int) []byte {
	t.Helper()
	var data []byte
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		data, _ = os.ReadFile(path)
		if len(data) >= want {
			break
		}
	}
	if want == 0 {
		time.Sleep(300 * time.Millisecond)
		data, _ = os.ReadFile(path)
	}
	return data
}

func TestSeveralLinesReachATmuxPaneThatAskedForBracketedPasteByteForByte(t *testing.T) {
	text := "Deux lignes :\n    « ça marche »\tç 👩‍💻\n$(touch canary) `x` fin;"
	pane, out := rawReader(t, true)
	if result, err := pane.phoneText(text, true); result != pasteDone || err != nil {
		t.Fatalf("%v %v", result, err)
	}
	want := pasteStart + text + pasteEnd
	if got := readAfter(t, out, len(want)); string(got) != want {
		t.Errorf("%q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(out), "canary")); err == nil {
		t.Error("something ran")
	}

	plain, plainOut := rawReader(t, false)
	if result, err := plain.phoneText(text, true); result != pasteNotTaken || err != nil {
		t.Errorf("a pane that never asked: %v %v", result, err)
	}
	if got := readAfter(t, plainOut, 0); len(got) != 0 {
		t.Errorf("a pane that never asked got %q", got)
	}
	if buffers, _ := plain.output("list-buffers"); len(strings.TrimSpace(string(buffers))) != 0 {
		t.Errorf("a buffer was left: %s", buffers)
	}
	// One line goes as the Mac's Tmux.insert pastes it; an Escape is refused before tmux is asked.
	if result, err := plain.phoneText("one line;", false); result != pasteDone || err != nil {
		t.Errorf("one line: %v %v", result, err)
	}
	if got := readAfter(t, plainOut, len("one line;")); string(got) != "one line;" {
		t.Errorf("one line: %q", got)
	}
	if result, _ := plain.phoneText("one\x1b[201~\ntwo", true); result != pasteFailed {
		t.Error("an Escape went to tmux")
	}
}
