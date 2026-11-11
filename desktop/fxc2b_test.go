package desktop

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The rows C2's second round left (FXC2b): where the Mac's Swift engine answers one way and the Go engine another,
// the Go engine answers as the Swift engine does. What Linux and Windows keep as it was is tested beside each.

// hookWith posts to /hook from this computer with the hook's key, and gives each header as many times as it lists.
func hookWith(t *testing.T, d *Deck, headers map[string][]string) int {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"twice","cwd":"/work/a"}`))
	r.Host, r.RemoteAddr = "localhost:7710", thisComputer
	r.Header.Set(hookKeyHeader, d.access.HookKey)
	for name, values := range headers {
		for _, value := range values {
			r.Header.Add(name, value)
		}
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	if w.Code == http.StatusBadRequest && w.Body.String() != "A hook header came twice" {
		t.Errorf("the refusal said %q", w.Body.String())
	}
	return w.Code
}

// A hook header that comes twice is one smuggled in after another, as a line break in a variable the hook was given
// would make it: refused (400, as the Mac's), whichever of them is repeated, and nothing is taken from the request.
// The same on every OS: the Go core's own hook never sends one twice.
func TestAHookHeaderSentTwiceIsRefused(t *testing.T) {
	for _, mac := range []bool{false, true} {
		if mac {
			withMac(t)
		}
		d := newTestDeck(t)
		for _, name := range []string{"X-AgentDeck-Tab", "X-AgentDeck-Agent", "X-AgentDeck-Project", "X-AgentDeck-PID"} {
			if got := hookWith(t, d, map[string][]string{name: {"one", "two"}}); got != http.StatusBadRequest {
				t.Errorf("mac %v: %s twice: %d", mac, name, got)
			}
		}
		if len(d.sessions.All()) != 0 {
			t.Errorf("mac %v: a refused hook made a session", mac)
		}
		// Once is as it was; a header that isn't a hook's may come twice.
		if got := hookWith(t, d, map[string][]string{"X-AgentDeck-Tab": {"launcher:one"}, "Accept": {"a", "b"}}); got != http.StatusNoContent {
			t.Errorf("mac %v: one header each: %d", mac, got)
		}
		// Without the key it is 401 first, as the Mac checks the key before the headers.
		r := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{}`))
		r.Host, r.RemoteAddr = "localhost:7710", thisComputer
		r.Header.Add("X-AgentDeck-Tab", "a")
		r.Header.Add("X-AgentDeck-Tab", "b")
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("mac %v: no key: %d", mac, w.Code)
		}
	}
}

// botNews posts a bot's event to /bots/<id>/event as the caller named, and gives the status.
func botNews(t *testing.T, d *Deck, as, target string, extra map[string]string) int {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"type":"seen"}`))
	r.Host, r.RemoteAddr = "localhost:7710", thisComputer
	r.Header.Set(setupHeader, "1")
	switch as {
	case "page":
		d.mu.Lock()
		r.AddCookie(&http.Cookie{Name: setupCookie(), Value: d.access.NewSetupSession()})
		d.mu.Unlock()
	case "tool":
		r.Header.Set(setupKeyHeader, d.access.SetupKey)
	}
	for name, value := range extra {
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w.Code
}

// The setup page's cookie may post a bot's news, as a tool's key may (the table's "/bots/*": page, tool): the Mac's
// gate lets the page through where the Go core answered 401. With nobody known it is 401 still.
func TestThePagesCookieMayPostABotsNews(t *testing.T) {
	d := newTestDeck(t)
	asked := serve(t, d, call{method: "POST", target: "/setup/bots", body: `{"bots":[{"id":"c2-bot","name":"C2 Bot"}]}`, host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	if asked.Code != http.StatusOK {
		t.Fatalf("the bot: %d %s", asked.Code, asked.Body)
	}
	for _, c := range []struct {
		as, target string
		want       int
	}{
		{"page", "/bots/c2-bot/event", http.StatusNoContent},
		{"tool", "/bots/c2-bot/event", http.StatusNoContent},
		{"page", "/bots/no-such-bot/event", http.StatusNotFound},
		{"", "/bots/c2-bot/event", http.StatusUnauthorized},
	} {
		if got := botNews(t, d, c.as, c.target, nil); got != c.want {
			t.Errorf("%q %s: %d, want %d", c.as, c.target, got, c.want)
		}
	}
}

// watchOnly is a deck with a Claude Code session in a terminal Smolcorp can only watch, a tmux one, and a headless run, and
// a paired phone: asks gives what each of the phone's controls answers.
type watchOnly struct {
	d     *Deck
	phone string
}

func newWatchOnly(t *testing.T) watchOnly {
	t.Helper()
	d := newTestDeck(t)
	hookOn(d, "watch", "SessionStart", "/work/watch", "other:/dev/ttys009", "/work/watch", 1793500000000)
	hookOn(d, "tmuxed", "SessionStart", "/work/tmuxed", "tmux:%1@/tmp/fxc2b-no-such-socket", "/work/tmuxed", 1793500000001)
	hookOn(d, "headless", "SessionStart", "/work/headless", "headless:3f2a9c1e", "/work/headless", 1793500000002)
	d.mu.Lock()
	d.config.ScrollEnabled = true
	d.mu.Unlock()
	return watchOnly{d: d, phone: pair(t, d)}
}

func (w watchOnly) post(t *testing.T, path, query string) (int, string) {
	t.Helper()
	got := serve(t, w.d, call{method: "POST", target: path + "?" + query + "&t=" + w.phone, host: deckOnWiFi, from: aPhone})
	return got.Code, got.Body.String()
}

// On a desk in a terminal Smolcorp can only watch, under the Mac app, every control but Let go says "Smolcorp can't do
// that in this terminal yet." (409), the effort dial included, and a tap on the desk (/focus) only picks it (204), as
// the Swift engine does. A terminal it has a driver for takes the effort; a headless run is as it was for the controls.
func TestAWatchOnlyDeskAnswersAsTheMacsEngineDoes(t *testing.T) {
	withMac(t)
	w := newWatchOnly(t)
	for _, action := range []string{"effort&level=high", "send", "accept", "reject", "esc", "up", "down", "scroll&lines=3", "scroll&to=bottom", "mic&phase=down", "clear"} {
		if status, said := w.post(t, "/act", "action="+action+"&session=watch"); status != http.StatusConflict || said != notHere {
			t.Errorf("%s: %d %q", action, status, said)
		}
	}
	// The terminal is looked at before the level, as the Mac's order is.
	if status, said := w.post(t, "/act", "action=effort&level=nope&session=watch"); status != http.StatusConflict || said != notHere {
		t.Errorf("a bad level on a watch-only desk: %d %q", status, said)
	}
	if status, said := w.post(t, "/focus", "session=watch"); status != http.StatusNoContent || said != "" {
		t.Errorf("focus: %d %q", status, said)
	}
	// A headless run: nothing to bring forward either (the Mac's 204), and its controls say it runs headless.
	if status, said := w.post(t, "/focus", "session=headless"); status != http.StatusNoContent || said != "" {
		t.Errorf("focus, headless: %d %q", status, said)
	}
	if status, said := w.post(t, "/act", "action=effort&level=high&session=headless"); status != http.StatusConflict || said != headlessNoControls {
		t.Errorf("effort, headless: %d %q", status, said)
	}
	// The effort is taken where the terminal has a driver, and queued until the agent is free.
	if status, said := w.post(t, "/act", "action=effort&level=high&session=tmuxed"); status != http.StatusNoContent && status != http.StatusAccepted || said != "" {
		t.Errorf("effort in tmux: %d %q", status, said)
	}
	// A watch-only desk's effort wasn't taken.
	w.d.mu.Lock()
	pending := w.d.sessions.Get("watch").PendingEffort
	w.d.mu.Unlock()
	if pending != "" {
		t.Errorf("the effort was kept: %q", pending)
	}
}

// Without the Mac app, Linux and Windows keep what they answered: the dial's effort is taken on any desk (their desks
// show the dial everywhere), a control the terminal can't do is 404 "This terminal can't do that", a tap on a watch-only
// desk is that 404 and on a headless run its own 409.
func TestWithoutTheMacAppAWatchOnlyDeskIsAsItWas(t *testing.T) {
	w := newWatchOnly(t)
	for _, action := range []string{"send", "accept", "esc", "scroll&lines=3", "mic&phase=down", "clear"} {
		if status, said := w.post(t, "/act", "action="+action+"&session=watch"); status != http.StatusNotFound || said != "This terminal can't do that" {
			t.Errorf("%s: %d %q", action, status, said)
		}
	}
	if status, said := w.post(t, "/act", "action=effort&level=high&session=watch"); status != http.StatusNoContent || said != "" {
		t.Errorf("effort: %d %q", status, said)
	}
	if status, said := w.post(t, "/focus", "session=watch"); status != http.StatusNotFound || said != "This terminal can't do that" {
		t.Errorf("focus: %d %q", status, said)
	}
	if status, said := w.post(t, "/focus", "session=headless"); status != http.StatusConflict || said != headlessNoControls {
		t.Errorf("focus, headless: %d %q", status, said)
	}
}

// The phone's text to a hire's desk in T3 Code (nothing types into it) says what /act's controls say, in the Mac's
// words: 409, not "Session not found". The same on every OS: nobody relies on the 404, and the sentence is T3 Code's.
func TestThePhonesTextToAT3DeskGetsTheT3Sentence(t *testing.T) {
	for _, mac := range []bool{false, true} {
		if mac {
			withMac(t)
		}
		d, _ := t3Deck(t, Hire{ID: t3DeskTestID})
		d.mu.Lock()
		d.config.PhoneMicEnabled = true
		d.mu.Unlock()
		key := pair(t, d)
		desk := t3DeskID(t3DeskTestID)
		hire, found := d.t3HireAtDesk(desk)
		if !found {
			t.Fatal("no desk for the hire")
		}
		want := t3NoControls(calledHire(hire))
		for _, send := range []string{"", `,"send":true`, `,"send":false`} {
			got := serve(t, d, call{method: "POST", target: "/dictate?session=" + url.QueryEscape(desk) + "&t=" + key, host: deckOnWiFi, from: aPhone,
				body: `{"text":"one line"` + send + `}`})
			if got.Code != http.StatusConflict || got.Body.String() != want {
				t.Errorf("mac %v %q: %d %q, want 409 %q", mac, send, got.Code, got.Body, want)
			}
		}
		// The checks before it keep their place (the Mac's order): the mic off, and a "!" first.
		bang := serve(t, d, call{method: "POST", target: "/dictate?session=" + url.QueryEscape(desk) + "&t=" + key, host: deckOnWiFi, from: aPhone, body: `{"text":"!ls"}`})
		if bang.Code != http.StatusBadRequest {
			t.Errorf("mac %v: a bang: %d %q", mac, bang.Code, bang.Body)
		}
		// And a desk that isn't there is still that.
		nobody := serve(t, d, call{method: "POST", target: "/dictate?session=nobody&t=" + key, host: deckOnWiFi, from: aPhone, body: `{"text":"one line"}`})
		if nobody.Code != http.StatusNotFound {
			t.Errorf("mac %v: no desk: %d %q", mac, nobody.Code, nobody.Body)
		}
	}
}

// Hiring off, an empty desk is 403 in the Mac's words, as the Mac's hire checks it first; with hiring on it is still
// "Unknown desk" (400). The same on every OS, with "computer" for "Mac" off the Mac app.
func TestHiringAtAnEmptyDeskWithHiringOffIsRefusedAsHiringOff(t *testing.T) {
	for _, mac := range []bool{false, true} {
		if mac {
			withMac(t)
		}
		d := newTestDeck(t)
		key := pair(t, d)
		hire := func() (int, string) {
			got := serve(t, d, call{method: "POST", target: "/act?action=hire&desk=&t=" + key, host: deckOnWiFi, from: aPhone})
			return got.Code, got.Body.String()
		}
		want := hiringOff
		if mac {
			want = "Hiring and firing are off. Turn them on in setup, on your Mac."
		}
		d.mu.Lock()
		d.config.HiringEnabled = false
		d.mu.Unlock()
		if status, said := hire(); status != http.StatusForbidden || said != want {
			t.Errorf("mac %v, hiring off: %d %q", mac, status, said)
		}
		d.mu.Lock()
		d.config.HiringEnabled = true
		d.mu.Unlock()
		if status, said := hire(); status != http.StatusBadRequest || said != "Unknown desk" {
			t.Errorf("mac %v, hiring on: %d %q", mac, status, said)
		}
	}
}

// A bot's floor sent as a number isn't refused: it changes nothing, as the Mac's reads only text (the page sends text).
func TestABotsFloorSentAsANumberIsIgnoredNotRefused(t *testing.T) {
	d := newTestDeck(t)
	page := func(path, body string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: path, body: body, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
	}
	if got := page("/setup/bots", `{"bots":[{"id":"c2-bot2","name":"C2 Bot Two"}]}`); got.Code != http.StatusOK {
		t.Fatalf("the bot: %d %s", got.Code, got.Body)
	}
	floor := func() string {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.config.CloudBots[0].Floor
	}
	before := floor()
	for _, body := range []string{`{"id":"c2-bot2","floor":3}`, `{"id":"c2-bot2","floor":null}`, `{"id":"c2-bot2","floor":["3"]}`, `{"id":"c2-bot2","floor":true}`} {
		if got := page("/setup/bots/override", body); got.Code != http.StatusOK {
			t.Errorf("%s: %d %s", body, got.Code, got.Body)
		}
	}
	if floor() != before {
		t.Errorf("the floor moved to %q from %q", floor(), before)
	}
	// As text it still moves it.
	want := cloudFloors[len(cloudFloors)-1]
	if got := page("/setup/bots/override", `{"id":"c2-bot2","floor":"`+want+`"}`); got.Code != http.StatusOK || floor() != want {
		t.Errorf("as text: %d, floor %q, want %q", got.Code, floor(), want)
	}
	// And a bot that isn't there is as it was.
	if got := page("/setup/bots/override", `{"id":"nobody","floor":3}`); got.Code != http.StatusNotFound {
		t.Errorf("no such bot: %d %s", got.Code, got.Body)
	}
}
