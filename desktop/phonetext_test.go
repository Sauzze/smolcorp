package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
type phoneMac struct {
	mu     sync.Mutex
	asked  []bridgeRequest
	answer bridgeAnswer
	// during runs as the words are typed, as a desk starting to ask meanwhile.
	during func()
}

// reply sets how phone-text is answered from now on.
func (m *phoneMac) reply(answer bridgeAnswer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answer = answer
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
	answer, during := m.answer, m.during
	m.mu.Unlock()
	switch request.Action {
	case "phone-text":
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
	mac.reply(bridgeAnswer{OK: true, Pasted: "not-taken"})
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
