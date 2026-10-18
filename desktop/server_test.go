package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Where requests come from: this computer, and a phone on the same Wi-Fi.
const (
	thisComputer = "127.0.0.1:50000"
	aPhone       = "192.168.1.20:50000"
	deckOnWiFi   = "192.168.1.10:7710"
)

type call struct {
	method, target, host, from string
	headers                    map[string]string
	body                       string
	// Setup's calls carry the setup key, as the page does, unless this says not to.
	withoutSetupKey bool
}

func serve(t *testing.T, d *Deck, c call) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(c.method, c.target, strings.NewReader(c.body))
	r.Host, r.RemoteAddr = c.host, c.from
	if strings.HasPrefix(c.target, "/setup/") && !c.withoutSetupKey {
		r.Header.Set(setupKeyHeader, d.access.SetupKey)
	}
	for name, value := range c.headers {
		r.Header.Set(name, value)
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w
}

func newTestDeck(t *testing.T) *Deck {
	t.Helper()
	isolate(t)
	return NewDeck()
}

// pair asks for a code on this computer and pairs a phone with it, as the
// setup page and the pairing page do.
func pair(t *testing.T, d *Deck) string {
	t.Helper()
	asked := serve(t, d, call{method: "POST", target: "/setup/pair", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	var state struct{ Pairing struct{ Code string } }
	if err := json.Unmarshal(asked.Body.Bytes(), &state); err != nil || state.Pairing.Code == "" {
		t.Fatalf("asking for a code gave %d %s", asked.Code, asked.Body)
	}
	paired := serve(t, d, call{method: "POST", target: "/pair", host: deckOnWiFi, from: aPhone,
		headers: map[string]string{"User-Agent": iPhone}, body: `{"code": "` + state.Pairing.Code + `"}`})
	var answer struct{ Key string }
	if err := json.Unmarshal(paired.Body.Bytes(), &answer); err != nil || answer.Key == "" {
		t.Fatalf("pairing gave %d %s", paired.Code, paired.Body)
	}
	return answer.Key
}

func TestTheSetupPageOpensOnlyOnThisComputer(t *testing.T) {
	d := newTestDeck(t)
	for _, c := range []struct {
		host, from string
		want       int
	}{
		{"localhost:7710", thisComputer, http.StatusOK},
		{deckOnWiFi, aPhone, http.StatusNotFound},
		// A web page whose name was pointed at this computer, to read the page through your browser.
		{"evil.example:7710", thisComputer, http.StatusNotFound},
	} {
		if got := serve(t, d, call{method: "GET", target: "/setup", host: c.host, from: c.from}).Code; got != c.want {
			t.Errorf("GET /setup for %s from %s: %d, want %d", c.host, c.from, got, c.want)
		}
	}
}

func TestSetupButtonsNeedTheSetupPagesHeader(t *testing.T) {
	d := newTestDeck(t)
	// What a form on another website could send: the browser adds no custom header.
	forged := serve(t, d, call{method: "POST", target: "/setup/unpair?all=1", host: "localhost:7710", from: thisComputer})
	if forged.Code != http.StatusForbidden {
		t.Fatalf("a setup button pressed without the header: %d, want %d", forged.Code, http.StatusForbidden)
	}
}

func TestAPairedPhoneGetsTheDeck(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	page := serve(t, d, call{method: "GET", target: "/?t=" + key, host: deckOnWiFi, from: aPhone})
	if !strings.Contains(page.Body.String(), `id="office-canvas"`) {
		t.Fatal("a paired phone didn't get the deck")
	}
	if page.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("the deck can be shown inside another site's page")
	}
	act := serve(t, d, call{method: "POST", target: "/act?action=send&session=nobody&t=" + key, host: deckOnWiFi, from: aPhone})
	if act.Code != http.StatusNotFound {
		t.Errorf("a paired phone's control for a session that isn't there: %d, want %d", act.Code, http.StatusNotFound)
	}
}

func TestAPhoneWithoutAKeyGetsOnlyThePairingPage(t *testing.T) {
	d := newTestDeck(t)
	pair(t, d)
	if page := serve(t, d, call{method: "GET", target: "/", host: deckOnWiFi, from: aPhone}); !strings.Contains(page.Body.String(), "Pair with your computer") {
		t.Error("a phone without a key didn't get the pairing page")
	}
	for _, target := range []string{"/act?action=send&session=s1", "/act?action=send&session=s1&t=a-made-up-key", "/focus?session=s1"} {
		if got := serve(t, d, call{method: "POST", target: target, host: deckOnWiFi, from: aPhone}).Code; got != http.StatusUnauthorized {
			t.Errorf("POST %s: %d, want %d", target, got, http.StatusUnauthorized)
		}
	}
}

func TestAPhonesKeyWorksOnlyAtTheDecksOwnAddress(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	got := serve(t, d, call{method: "GET", target: "/?t=" + key, host: "evil.example:7710", from: aPhone}).Code
	if got != http.StatusForbidden {
		t.Fatalf("the deck answered a page that rebound its own name to it: %d, want %d", got, http.StatusForbidden)
	}
}

func TestOnlyTheHookReportsEvents(t *testing.T) {
	d := newTestDeck(t)
	phoneKey := pair(t, d)
	report := func(from, key string) int {
		return serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: from,
			headers: map[string]string{hookKeyHeader: key, "X-AgentDeck-Tab": tab, "X-AgentDeck-At": "100"},
			body:    `{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api-server"}`}).Code
	}
	if got := report(thisComputer, ""); got != http.StatusUnauthorized {
		t.Errorf("an event with no key: %d, want %d", got, http.StatusUnauthorized)
	}
	if got := report(thisComputer, phoneKey); got != http.StatusUnauthorized {
		t.Errorf("an event with a phone's key: %d, want %d", got, http.StatusUnauthorized)
	}
	if got := report(aPhone, d.access.HookKey); got != http.StatusNotFound {
		t.Errorf("an event from another device: %d, want %d", got, http.StatusNotFound)
	}
	if d.sessions.Get("s1") != nil {
		t.Fatal("a refused event reached the deck")
	}
	if got := report(thisComputer, d.access.HookKey); got != http.StatusNoContent || d.sessions.Get("s1") == nil {
		t.Fatalf("the hook's own event: %d, and the session isn't on the deck", got)
	}
}

func TestHiringIsSetUpOnlyWhenEveryPartIsRight(t *testing.T) {
	d := newTestDeck(t)
	setUp := func(query string) int {
		return serve(t, d, call{method: "POST", target: "/setup/hiring?" + query, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}}).Code
	}
	writeFile(t, filepath.Join(homeDir, "work", "keep"), "")
	for _, query := range []string{
		"enabled=1&agent=codex&folder=work",        // a relative path
		"enabled=1&agent=codex&folder=~/nowhere",   // a folder that isn't there
		"enabled=1&agent=codex&folder=~/work/keep", // a file, not a folder
		"enabled=1&agent=skynet&folder=" + homeDir, // an agent there isn't
	} {
		if got := setUp(query); got != http.StatusBadRequest {
			t.Errorf("%s: %d, want %d", query, got, http.StatusBadRequest)
		}
		if c := d.config; c.HiringEnabled || c.HireAgent != string(Claude) || c.HireFolder != "" {
			t.Fatalf("%s was refused, yet hiring is now %v, %q, %q", query, c.HiringEnabled, c.HireAgent, c.HireFolder)
		}
	}
	if got := setUp("enabled=1&agent=codex&folder=~/work"); got != http.StatusOK || d.config.HireFolder != filepath.Join(homeDir, "work") ||
		!d.config.HiringEnabled || d.config.HireAgent != "codex" {
		t.Fatalf("~/work: %d, and hiring is %+v", got, d.config)
	}
	if got := setUp("folder="); got != http.StatusOK || d.config.HireFolder != "" || d.config.HireAgent != "codex" {
		t.Fatalf("an empty folder: %d, and the folder is %q, the agent %q; want it cleared and the agent kept", got, d.config.HireFolder, d.config.HireAgent)
	}
}

// The team cards in setup go by the project's name, which stays put as the
// agent moves into a subfolder.
func TestTheSetupPageNamesEachSessionsProject(t *testing.T) {
	d := newTestDeck(t)
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/work/api/cmd/server"},
		Claude, tab, "/work/api", 100)
	sessions := (&router{deck: d}).setupState()["sessions"].([]map[string]any)
	if len(sessions) != 1 || sessions[0]["name"] != "api" {
		t.Fatalf("sessions %v, want one named api", sessions)
	}
}

func TestWhatSetupReadsAndChangesNeedsTheSetupKey(t *testing.T) {
	d := newTestDeck(t)
	local := func(c call) int {
		c.host, c.from = "localhost:7710", thisComputer
		return serve(t, d, c).Code
	}
	// Any program on this computer can send the page's header, and another person signed in to it can too.
	if got := local(call{method: "GET", target: "/setup/state", withoutSetupKey: true}); got != http.StatusUnauthorized {
		t.Errorf("setup's state without the key: %d, want %d", got, http.StatusUnauthorized)
	}
	if got := local(call{method: "POST", target: "/setup/pair", withoutSetupKey: true, headers: map[string]string{setupHeader: "1"}}); got != http.StatusUnauthorized {
		t.Errorf("a pairing code asked for without the key: %d, want %d", got, http.StatusUnauthorized)
	}
	if got := local(call{method: "GET", target: "/setup/state", headers: map[string]string{setupKeyHeader: "a-made-up-key"}}); got != http.StatusUnauthorized {
		t.Errorf("setup's state with a made-up key: %d, want %d", got, http.StatusUnauthorized)
	}
	// The page itself holds nothing, and opens to say where the key is.
	if got := local(call{method: "GET", target: "/setup"}); got != http.StatusOK {
		t.Errorf("the setup page: %d, want %d", got, http.StatusOK)
	}
	// A tool of your own sends the key from its file.
	saved, err := os.ReadFile(setupKeyFile)
	if err != nil || strings.TrimSpace(string(saved)) != d.access.SetupKey {
		t.Fatalf("the setup-key file holds %q (%v), want the key", saved, err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(setupKeyFile); info.Mode().Perm() != 0o600 {
			t.Errorf("the setup-key file can be read by others: %v", info.Mode().Perm())
		}
	}
	if got := local(call{method: "GET", target: "/setup/state", withoutSetupKey: true,
		headers: map[string]string{setupKeyHeader: strings.TrimSpace(string(saved))}}); got != http.StatusOK {
		t.Errorf("setup's state with the key from its file: %d, want %d", got, http.StatusOK)
	}
}

func TestTheTraysLinkGivesThePageACookie(t *testing.T) {
	d := newTestDeck(t)
	if link := d.setupLink("pair"); !strings.HasSuffix(link, "/setup#k="+d.access.SetupKey+"&pair") {
		t.Fatalf("the tray's link is %s", link)
	}
	session := serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}})
	cookies := session.Result().Cookies()
	if session.Code != http.StatusNoContent || len(cookies) != 1 {
		t.Fatalf("handing the key over: %d, cookies %v", session.Code, cookies)
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/setup" {
		t.Errorf("the cookie can be read by scripts, sent by other sites or to other pages: %+v", cookie)
	}
	state := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{"Cookie": cookie.Name + "=" + cookie.Value}})
	if state.Code != http.StatusOK {
		t.Errorf("setup's state with the page's cookie: %d, want %d", state.Code, http.StatusOK)
	}
	// Each start has a new key, so a link or cookie from before stops working.
	if again := NewDeck(); again.access.SetupKey == d.access.SetupKey {
		t.Error("a new start kept the setup key")
	}
}

func TestOneDeviceCantTakeEveryConnection(t *testing.T) {
	l := &cappedListener{byAddress: map[string]int{}}
	var held []func()
	for range mostPerAddress {
		release, ok := l.take("192.168.1.66")
		if !ok {
			t.Fatal("a device was refused before it reached its share")
		}
		held = append(held, release)
	}
	if _, ok := l.take("192.168.1.66"); ok {
		t.Fatalf("one device got more than %d connections", mostPerAddress)
	}
	// Other devices fill the rest, but never the ones kept for this computer.
	for i := 0; ; i++ {
		release, ok := l.take(fmt.Sprintf("192.168.1.%d", 100+i/mostPerAddress))
		if !ok {
			break
		}
		held = append(held, release)
	}
	if l.open != maxConnections-keptForThisComputer {
		t.Fatalf("other devices hold %d connections, want %d", l.open, maxConnections-keptForThisComputer)
	}
	// The hook still gets in, more than once.
	for range keptForThisComputer {
		release, ok := l.take("127.0.0.1")
		if !ok {
			t.Fatal("the hook was locked out by other devices")
		}
		held = append(held, release)
	}
	for _, release := range held {
		release()
	}
	if l.open != 0 || len(l.byAddress) != 0 {
		t.Fatalf("connections closed but still counted: %d open, %v", l.open, l.byAddress)
	}
}

func TestAPhoneThatFallsBehindGetsTheNewestState(t *testing.T) {
	s := newEventStream()
	for i := range 20 {
		s.send([]byte(fmt.Sprint(i)))
	}
	s.send(nil) // a ping with no room
	var last []byte
	for len(s.messages) > 0 {
		last = <-s.messages
	}
	if string(last) != "19" {
		t.Fatalf("the last state waiting is %q, want the newest, 19", last)
	}
}

func TestTheTraysLinkWorksOnceAndCarriesNoKey(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	if link := setupAddress("o="+token, "pair"); strings.Contains(link, d.access.SetupKey) {
		t.Fatal("the tray's link, on the browser's command line, carries the setup key")
	}
	handOver := func(token string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupTokenHeader: token}})
	}
	first := handOver(token)
	if first.Code != http.StatusNoContent || len(first.Result().Cookies()) != 1 || first.Result().Cookies()[0].Value != d.access.SetupKey {
		t.Fatalf("the link's token gave %d, cookies %v", first.Code, first.Result().Cookies())
	}
	// Read from another user's view of the command line, it's used up already.
	if again := handOver(token); again.Code != http.StatusUnauthorized {
		t.Errorf("the token worked a second time: %d", again.Code)
	}
	// And one never used runs out.
	d.mu.Lock()
	late := d.access.NewSetupToken()
	d.access.setupTokens[late] = time.Now().Add(-time.Second)
	d.mu.Unlock()
	if expired := handOver(late); expired.Code != http.StatusUnauthorized {
		t.Errorf("a token that had run out worked: %d", expired.Code)
	}
	// A token opens only the handover, not setup itself.
	d.mu.Lock()
	other := d.access.NewSetupToken()
	d.mu.Unlock()
	state := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupTokenHeader: other}})
	if state.Code != http.StatusUnauthorized {
		t.Errorf("a token read setup's state: %d", state.Code)
	}
}

// streamWriter is a response the test can read as it's written: each flush
// passes on what came since the last.
type streamWriter struct {
	header  http.Header
	pending strings.Builder
	flushed chan string
}

func (w *streamWriter) Header() http.Header         { return w.header }
func (w *streamWriter) WriteHeader(int)             {}
func (w *streamWriter) Write(b []byte) (int, error) { return w.pending.Write(b) }
func (w *streamWriter) Flush() {
	w.flushed <- w.pending.String()
	w.pending.Reset()
}

func TestAPhoneHearsThePingAsAnEvent(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	r := httptest.NewRequest("GET", "/events?t="+key, nil)
	r.Host, r.RemoteAddr = deckOnWiFi, aPhone
	ctx, hangUp := context.WithCancel(r.Context())
	defer hangUp()
	w := &streamWriter{header: http.Header{}, flushed: make(chan string, 8)}
	go (&router{deck: d}).ServeHTTP(w, r.WithContext(ctx))
	// Once the phone's stream is open, ping it as the deck does every pingInterval.
	for open := false; !open; time.Sleep(10 * time.Millisecond) {
		d.mu.Lock()
		open = len(d.streams) > 0
		d.mu.Unlock()
	}
	d.mu.Lock()
	for stream := range d.streams {
		stream.send(nil)
	}
	d.mu.Unlock()
	// The state comes first; then the ping, as a named event with data: a
	// comment, or an event without data, never reaches the page.
	for {
		select {
		case got := <-w.flushed:
			if got == "event: ping\ndata: 1\n\n" {
				return
			}
			if !strings.HasPrefix(got, "data: ") {
				t.Fatalf("the ping came as %q, want a named event with data", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("no ping came")
		}
	}
}
