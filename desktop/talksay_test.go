package desktop

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Talk Bubble, "say" (1.26): the office window's words into a desk, by the code /dictate runs. The rules are the
// phone's (phonetext_test.go checks them in full); what's pinned here is that the window gets every one of them,
// and the three things that differ: no phone-text switch (D2), no Mac terminal app's tab (D3), Return by default.

// windowDeck is a deck whose desk s1 is a Claude Code session, done with its turn, in a launcher's window
// (a desk the window may type into), with the fake launcher that takes what is typed.
func windowDeck(t *testing.T) (*Deck, *fakeLauncher) {
	t.Helper()
	saved, settle := host, phoneTextSettle
	host, phoneTextSettle = nil, time.Millisecond
	t.Cleanup(func() { host, phoneTextSettle = saved, settle })
	d := newTestDeck(t)
	f := newFakeLauncherAs(t, d, "L1", "/work/api", "", "", "idle-empty", true)
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 20)
	return d, f
}

func asksAt(d *Deck, tab string, at float64) {
	d.Ingest(map[string]any{"hook_event_name": "PermissionRequest", "session_id": "s1", "cwd": "/work/api", "tool_name": "Bash",
		"tool_input": map[string]any{"command": "touch x"}}, "claude", tab, "", at)
}

// pasted is a line as the launcher gets it: its fake's terminal asked for bracketed paste.
func pasted(words string) string { return pasteStart + words + pasteEnd }

func offBy(f *fakeLauncher) []string {
	time.Sleep(60 * time.Millisecond)
	return f.written()
}

// The window's Send is the words and Return, one line or several as one paste; Return is left out only if it says so.
func TestTheWindowTypesAndSendsByDefault(t *testing.T) {
	d, f := windowDeck(t)
	if status, message := d.TypeWindowText("fix the login", "s1", nil); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	if got := f.writtenAfter(2); !reflect.DeepEqual(got, []string{pasted("fix the login"), keyBytes[Enter]}) {
		t.Errorf("two words, no send said: still the words and Return: %q", got)
	}
	// The phone's two words, with nothing said, go without Return, as they always did.
	if status, _ := d.TypePhoneText("fix it", "s1", nil); status != http.StatusNoContent {
		t.Fatal(status)
	}
	if got := f.writtenAfter(3); !reflect.DeepEqual(got, []string{pasted("fix the login"), keyBytes[Enter], pasted("fix it")}) {
		t.Errorf("the phone's: %q", got)
	}
	d.mu.Lock()
	delete(d.lastReturn, "s1")
	d.mu.Unlock()
	text := "Deux lignes :\n    « ça marche » 👩‍💻 fin;"
	no := false
	if status, message := d.TypeWindowText(text, "s1", &no); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	if got := f.writtenAfter(4); !reflect.DeepEqual(got[3:], []string{pasted(text)}) {
		t.Errorf("several lines, one paste: %q", got)
	}
	if got := offBy(f); len(got) != 4 {
		t.Errorf("a Return went though the window said not: %q", got)
	}
}

// Every refusal of /dictate, for the window: nothing is typed and the sentence says why.
func TestTheWindowGetsEveryRefusalThePhoneGets(t *testing.T) {
	d, f := windowDeck(t)
	for _, c := range []struct {
		name, text string
		status     int
		says       string
	}{
		{"nothing", " \n ", http.StatusBadRequest, "Nothing to type"},
		{"too long", strings.Repeat("a", 2001), http.StatusBadRequest, tooLong},
		{"a bang", "  !ls -la", http.StatusBadRequest, noBangPrompt},
	} {
		if status, says := d.TypeWindowText(c.text, "s1", nil); status != c.status || says != c.says {
			t.Errorf("%s: %d %q", c.name, status, says)
		}
		if status, says := d.TypePhoneText(c.text, "s1", nil); status != c.status || says != c.says {
			t.Errorf("%s from a phone: %d %q", c.name, status, says)
		}
	}
	if status, _ := d.TypeWindowText("hello", "nobody", nil); status != http.StatusNotFound {
		t.Errorf("no such desk: %d", status)
	}
	if got := offBy(f); len(got) != 0 {
		t.Errorf("typed: %q", got)
	}
}

// Never while the desk asks: as the request comes, and again on the key worker right before the words.
func TestTheWindowTypesNothingIntoADeskThatAsks(t *testing.T) {
	d, f := windowDeck(t)
	asksAt(d, "launcher:L1", 30)
	for _, words := range []string{"one line", "Deux lignes :\n    fin;"} {
		if status, says := d.TypeWindowText(words, "s1", nil); status != http.StatusConflict || says != windowAnswerFirst {
			t.Errorf("asking: %d %q", status, says)
		}
	}
	// The phone's sentence still says "the deck".
	if status, says := d.TypePhoneText("one line", "s1", nil); status != http.StatusConflict || says != answerFirst {
		t.Errorf("a phone: %d %q", status, says)
	}
	// Free again, then asking while the words wait for the key worker.
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "launcher:L1", "", 40)
	hold := make(chan struct{})
	d.work <- func() { <-hold }
	answered := make(chan [2]any, 1)
	go func() {
		status, says := d.TypeWindowText("fix the login bug", "s1", nil)
		answered <- [2]any{status, says}
	}()
	time.Sleep(50 * time.Millisecond)
	asksAt(d, "launcher:L1", 50)
	close(hold)
	if got := <-answered; got[0] != http.StatusConflict || got[1] != windowAnswerFirst {
		t.Errorf("asked meanwhile: %v", got)
	}
	if got := offBy(f); len(got) != 0 {
		t.Errorf("typed: %q", got)
	}
}

// A Return that just went is not repeated: the words are typed, Return withheld, and it says so.
func TestTheWindowsReturnIsWithheldAfterAReturnJustWent(t *testing.T) {
	d, f := windowDeck(t)
	d.mu.Lock()
	d.lastReturn["s1"] = time.Now()
	d.mu.Unlock()
	if status, says := d.TypeWindowText("fix the login bug", "s1", nil); status != http.StatusConflict || says != windowTypedNotSent {
		t.Errorf("%d %q", status, says)
	}
	if got := f.writtenAfter(1); !reflect.DeepEqual(got, []string{pasted("fix the login bug")}) {
		t.Errorf("%q", got)
	}
	if got := offBy(f); len(got) != 1 {
		t.Errorf("a Return went: %q", got)
	}
}

// D2: the phone-text switch is about phones. Off, a phone is refused and the window still types.
func TestThePhoneTextSwitchRefusesPhonesAndNotTheWindow(t *testing.T) {
	d, f := windowDeck(t)
	d.mu.Lock()
	d.config.PhoneMicEnabled = false
	d.mu.Unlock()
	if status, says := d.TypePhoneText("hello there", "s1", nil); status != http.StatusForbidden || says != phoneTextOff {
		t.Errorf("a phone: %d %q", status, says)
	}
	if status, says := d.TypeWindowText("hello there", "s1", nil); status != http.StatusNoContent {
		t.Errorf("the window: %d %q", status, says)
	}
	if got := f.writtenAfter(2); !reflect.DeepEqual(got, []string{pasted("hello there"), keyBytes[Enter]}) {
		t.Errorf("%q", got)
	}
}

// D3: a desk in Terminal.app, iTerm2 or Ghostty is typed into only by the Mac app, through the clipboard and
// ⌘V, which brings that window forward. The window never does that; a phone's text still goes.
func TestTheWindowNeverTypesIntoAMacTerminalApp(t *testing.T) {
	d, mac := phoneDeck(t)
	for _, tab := range []string{"tty:/dev/ttys003", "iterm:AB12-CD34", "ghostty:/dev/ttys005"} {
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", tab, "", 25)
		if status, says := d.TypeWindowText("fix the login bug", "s1", nil); status != http.StatusConflict || says != typeInItsOwnWindow {
			t.Errorf("%s: %d %q", tab, status, says)
		}
		if typing, found := d.windowTyping("s1"); !found || typing != (deskTyping{Why: typeInItsOwnWindow}) {
			t.Errorf("%s: the card says %+v", tab, typing)
		}
	}
	if got := mac.actions(); len(got) != 0 {
		t.Errorf("the Mac app was asked: %q", got)
	}
	d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", "tty:/dev/ttys003", "", 26)
	if status, says := d.TypePhoneText("fix the login bug", "s1", sendIt()); status != http.StatusNoContent {
		t.Fatalf("a phone: %d %q", status, says)
	}
	if got := mac.actions(); !reflect.DeepEqual(got, []string{"phone-text tty:/dev/ttys003 claude fix the login bug", "press enter"}) {
		t.Errorf("the phone's text still goes through the Mac app: %q", got)
	}
}

// Where a desk can't be typed into: a hire in T3 Code, a headless run, a desk with no terminal. Each in its sentence.
func TestTheWindowIsToldWhereADeskTakesNoText(t *testing.T) {
	d, _ := t3Deck(t, Hire{ID: t3DeskTestID})
	desk := t3DeskID(t3DeskTestID)
	hire, found := d.t3HireAtDesk(desk)
	if !found {
		t.Fatal("no desk for the hire")
	}
	want := t3NoControls(calledHire(hire))
	if status, says := d.TypeWindowText("one line", desk, nil); status != http.StatusConflict || says != want {
		t.Errorf("T3: %d %q", status, says)
	}
	if typing, found := d.windowTyping(desk); !found || typing != (deskTyping{Why: want}) {
		t.Errorf("T3's card: %+v", typing)
	}

	d = newTestDeck(t)
	headless(d, 1)
	if status, says := d.TypeWindowText("one line", "h0", nil); status != http.StatusConflict || says != headlessNoControls {
		t.Errorf("headless: %d %q", status, says)
	}
	if typing, _ := d.windowTyping("h0"); typing != (deskTyping{Why: headlessNoControls}) {
		t.Errorf("headless card: %+v", typing)
	}
	// The phone's sentence there is the one it always had.
	if status, says := d.TypePhoneText("one line", "h0", nil); status != http.StatusConflict || says != notHere {
		t.Errorf("headless from a phone: %d %q", status, says)
	}

	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "Stop", "session_id": "bare", "cwd": "/work/api"}, Claude, "tty:/dev/ttys090", "", 20)
	d.sessions.Get("bare").Tab = "" // a desk whose terminal is lost
	d.mu.Unlock()
	if status, _ := d.TypeWindowText("one line", "bare", nil); status != http.StatusNotFound {
		t.Errorf("no tab: %d", status)
	}
	if typing, found := d.windowTyping("bare"); !found || typing != (deskTyping{Why: notHere}) {
		t.Errorf("no tab's card: %+v", typing)
	}
	if _, found := d.windowTyping("nobody"); found {
		t.Error("a desk that isn't there has a card")
	}
}

// WezTerm and kitty take one line only: said on the card before anyone types, and refused at once if several are sent.
func TestTheWindowIsToldOfATerminalThatTakesOneLine(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	d := newTestDeck(t)
	for id, tab := range map[string]string{"w": "wezterm:3@/nowhere/wez", "k": "kitty:4@unix:/nowhere/kitty"} {
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": id, "cwd": "/work/api"}, "claude", tab, "", 20)
		if status, says := d.TypeWindowText("one\ntwo", id, nil); status != http.StatusConflict || says != noLines {
			t.Errorf("%s: %d %q", id, status, says)
		}
		if typing, _ := d.windowTyping(id); typing != (deskTyping{OK: true, Why: oneLineOnly}) {
			t.Errorf("%s: the card says %+v", id, typing)
		}
	}
	d, _ = windowDeck(t)
	if typing, _ := d.windowTyping("s1"); typing != (deskTyping{OK: true, Lines: true}) {
		t.Errorf("a launcher's card: %+v", typing)
	}
	asksAt(d, "launcher:L1", 30)
	if typing, _ := d.windowTyping("s1"); typing != (deskTyping{Why: windowAnswerFirst}) {
		t.Errorf("an asking desk's card: %+v", typing)
	}
}

// RWTC N1: a test copy types only into its own hires' tmux panes, for the window as for a phone, and says so
// first on the card.
func TestATestCopyTypesNothingForTheWindowEither(t *testing.T) {
	d, mac := macTabDeck(t)
	onMarkedPort(t, 7799)
	for _, tab := range []string{"iterm:AB12-CD34", "tmux:%3@/another/socket"} {
		d.Ingest(map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/work/api"}, "claude", tab, "", 30)
		if status, says := d.TypeWindowText("hello there", "s1", nil); status != http.StatusConflict || says != testCopyKeepsTerminal {
			t.Errorf("%s: %d %q", tab, status, says)
		}
		if typing, _ := d.windowTyping("s1"); typing != (deskTyping{Why: testCopyKeepsTerminal}) {
			t.Errorf("%s: the card says %+v", tab, typing)
		}
	}
	if asked := mac.requests("phone-text"); len(asked) != 0 {
		t.Errorf("the Mac app was asked: %+v", asked)
	}
}

// POST /talk/say: the window's key from this computer only. A phone's key is refused in the window's sentence, no
// key is 401, and /dictate stays a phone's (TestTheOfficeWindowTypesNothing keeps that one).
func TestTalkSayIsTheWindowsAlone(t *testing.T) {
	// The proxied and ts.net asks below come through the real `tailscale serve` (a stand-in peer, SN1): even then, the
	// window's key isn't taken. From a program here they are refused before it (peerowner_test.go).
	standInTailscalePeer(t)
	d, f := windowDeck(t)
	d.mu.Lock()
	window := d.access.WindowKey
	d.mu.Unlock()
	phone := pair(t, d)
	say := func(key, host, from, body string, headers map[string]string) *httpAnswer {
		target := "/talk/say?session=s1"
		if key != "" {
			target += "&t=" + key
		}
		got := serve(t, d, call{method: "POST", target: target, host: host, from: from, body: body, headers: headers})
		return &httpAnswer{got.Code, got.Body.String()}
	}
	if got := say("", "localhost:7710", thisComputer, `{"text":"hello there"}`, nil); got.code != http.StatusUnauthorized {
		t.Errorf("no key: %d", got.code)
	}
	if got := say(phone, deckOnWiFi, aPhone, `{"text":"hello there"}`, nil); got.code != http.StatusForbidden || got.body != d.shownOnlyInTheWindow("The conversation") {
		t.Errorf("a phone's key: %d %q", got.code, got.body)
	}
	if got := say(window, deckOnWiFi, aPhone, `{"text":"hello there"}`, nil); got.code != http.StatusUnauthorized {
		t.Errorf("the window's key from another device: %d", got.code)
	}
	if got := say(window, "localhost:7710", thisComputer, `{"text":"hello there"}`, map[string]string{"X-Forwarded-For": "100.64.0.9"}); got.code != http.StatusUnauthorized {
		t.Errorf("the window's key through a proxy: %d", got.code)
	}
	if got := offBy(f); len(got) != 0 {
		t.Fatalf("typed by a request that wasn't the window's: %q", got)
	}
	if got := say(window, "localhost:7710", thisComputer, `{"text":"hello there"}`, nil); got.code != http.StatusNoContent {
		t.Fatalf("the window: %d %q", got.code, got.body)
	}
	if got := f.writtenAfter(2); !reflect.DeepEqual(got, []string{pasted("hello there"), keyBytes[Enter]}) {
		t.Errorf("%q", got)
	}
	if got := say(window, "localhost:7710", thisComputer, `{"text":"!ls"}`, nil); got.code != http.StatusBadRequest || got.body != noBangPrompt {
		t.Errorf("a bang: %d %q", got.code, got.body)
	}
	if got := say(window, "localhost:7710", thisComputer, `{"text":"again"}`, nil); got.code != http.StatusConflict || got.body != windowTypedNotSent {
		t.Errorf("a second Return at once: %d %q", got.code, got.body)
	}
}

type httpAnswer struct {
	code int
	body string
}
