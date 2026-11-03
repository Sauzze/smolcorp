package desktop

import (
	"context"
	"encoding/base64"
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
	// Setup's calls carry a page session's cookie, as the page does, unless this
	// says not to, or asTool: then they carry the setup key in a header, as a tool does.
	withoutSetupKey bool
	asTool          bool
}

func serve(t *testing.T, d *Deck, c call) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(c.method, c.target, strings.NewReader(c.body))
	r.Host, r.RemoteAddr = c.host, c.from
	if strings.HasPrefix(c.target, "/setup/") && !c.withoutSetupKey {
		if c.asTool {
			r.Header.Set(setupKeyHeader, d.access.SetupKey)
		} else {
			d.mu.Lock()
			r.AddCookie(&http.Cookie{Name: setupCookie, Value: d.access.NewSetupSession()})
			d.mu.Unlock()
		}
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
	d := NewDeck()
	t.Cleanup(d.stopWatchers)
	// No coding agent is looked for: the tests' own say which are here.
	d.summaryProviders.set(map[summaryProvider]providerState{})
	return d
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

// The Tailscale instructions name the port the page was served from (the server may run on another
// SMOLCORP_PORT) and a neutral machine name, not a fixed 7710 or "this-mac".
func TestTheTailscaleInstructionsDoNotHardcodeThePortOrTheMac(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7799", from: thisComputer}).Body.String()
	// Only the Mac app's own page says this-mac (see TestTheSetupPageSaysMacOnlyOnTheMac).
	if strings.Contains(strings.ReplaceAll(page, "onMac() ? 'this-mac' : 'this-computer'", ""), "this-mac") {
		t.Error("the setup page says this-mac outside the Mac")
	}
	if strings.Contains(page, "127.0.0.1:7710") {
		t.Error("the setup page still hardcodes 127.0.0.1:7710 in the tailscale serve command")
	}
	if !strings.Contains(page, "127.0.0.1:${location.port") {
		t.Error("the tailscale serve command does not use the port the page was served from")
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
	// On a computer with no tray, as a cloud VM's desktop, the page also says how to pair.
	if page := serve(t, d, call{method: "GET", target: "/", host: "localhost:7710", from: thisComputer}); !strings.Contains(page.Body.String(), "With no tray, open the setup link Smolcorp printed as it started") ||
		strings.Contains(page.Body.String(), "setup --open") {
		t.Error("the pairing page asks for the tray, and names no true way in without one")
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
	if got := setUp("folder="); got != http.StatusBadRequest || d.config.HireFolder == "" {
		t.Fatalf("clearing the folder with hiring on: %d, and the folder is %q; want it refused and kept", got, d.config.HireFolder)
	}
	if got := setUp("enabled=0&folder="); got != http.StatusOK || d.config.HireFolder != "" || d.config.HireAgent != "codex" {
		t.Fatalf("an empty folder: %d, and the folder is %q, the agent %q; want it cleared and the agent kept", got, d.config.HireFolder, d.config.HireAgent)
	}
}

func TestHiringCantBeTurnedOnWithNoFolderAndTheOfficeSaysWhy(t *testing.T) {
	d := newTestDeck(t)
	fakeHiring(t, "") // hiring can open here
	setUp := func(query string) int {
		return serve(t, d, call{method: "POST", target: "/setup/hiring?" + query, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}}).Code
	}
	state := func() map[string]any {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.hiringState()
	}
	for _, query := range []string{"enabled=1", "enabled=1&folder=", "enabled=1&folder=~/nowhere"} {
		if got := setUp(query); got != http.StatusBadRequest || d.config.HiringEnabled {
			t.Errorf("%s with no folder: %d, hiring on %v; want it refused", query, got, d.config.HiringEnabled)
		}
	}
	// A config that has hiring on with no folder (saved by an older version) says why.
	d.config.HiringEnabled = true
	if s := state(); s["hire"] != false || s["reason"] != hiringNeedsFolder {
		t.Errorf("hiring on with no folder: %v, want no hire and the reason", s)
	}
	d.config.HiringEnabled = false
	if s := state(); s["reason"] != nil {
		t.Errorf("hiring off: %v, want no reason", s)
	}
	// A floor's own folder is enough, as is the global one.
	d.config.FloorRules = floorRuleList{{t.TempDir(), 2, false}}
	if got := setUp("enabled=1"); got != http.StatusOK || !d.config.HiringEnabled || state()["reason"] != nil {
		t.Errorf("hiring with a floor's folder: %d, %v", got, state())
	}
	d.config.FloorRules = nil
	d.config.HiringEnabled = false
	if got := setUp("enabled=1&folder=" + t.TempDir()); got != http.StatusOK || !d.config.HiringEnabled || state()["reason"] != nil {
		t.Errorf("hiring with a global folder: %d, %v", got, state())
	}
}

// Where hiring can't open at all, the folder isn't what blocks it: the switch
// can still be turned on, for firing, and nothing says to choose a folder.
func TestWhereHiringCantOpenTheSwitchNeedsNoFolder(t *testing.T) {
	d := newTestDeck(t)
	fakeHiring(t, "")
	installedOpeners = func() []hireOpener { return nil }
	if canHireHere() {
		t.Fatal("canHireHere is true with no terminal and no tmux")
	}
	setUp := func(query string) int {
		return serve(t, d, call{method: "POST", target: "/setup/hiring?" + query, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}}).Code
	}
	if got := setUp("enabled=1"); got != http.StatusOK || !d.config.HiringEnabled {
		t.Fatalf("enabling with no folder and nowhere to hire: %d, hiring on %v; want it allowed", got, d.config.HiringEnabled)
	}
	d.mu.Lock()
	state := d.hiringState()
	d.mu.Unlock()
	if state["fire"] != true || state["hire"] != false {
		t.Errorf("state %v, want firing on and no hiring", state)
	}
	if reason, _ := state["reason"].(string); strings.Contains(strings.ToLower(reason), "folder") {
		t.Errorf("reason %q blames the folder, which isn't what blocks hiring", reason)
	}
	// With somewhere to open, the refusal and the reason are back.
	installedOpeners = func() []hireOpener { return []hireOpener{{id: "term", name: "Term"}} }
	d.config.HiringEnabled = false
	if got := setUp("enabled=1"); got != http.StatusBadRequest || d.config.HiringEnabled {
		t.Errorf("enabling with no folder where hiring can open: %d, hiring on %v; want it refused", got, d.config.HiringEnabled)
	}
	d.config.HiringEnabled = true
	d.mu.Lock()
	state = d.hiringState()
	d.mu.Unlock()
	if state["reason"] != hiringNeedsFolder {
		t.Errorf("state %v, want the folder reason", state)
	}
}

// The office asks before max effort until setup turns that off: kept in the
// config, said in setup's state, and told to the phones in the snapshot.
func TestAskingBeforeMaxEffortIsOnUntilTurnedOff(t *testing.T) {
	d := newTestDeck(t)
	asks := func() (setup, snapshot bool) {
		var state struct{ MaxConfirm struct{ Enabled bool } }
		var shown struct{ MaxConfirm bool }
		got := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
		if json.Unmarshal(got.Body.Bytes(), &state) != nil || json.Unmarshal(d.snapshot(), &shown) != nil {
			t.Fatalf("setup's state %q, or the snapshot, isn't JSON", got.Body.String())
		}
		return state.MaxConfirm.Enabled, shown.MaxConfirm
	}
	turn := func(enabled string) {
		got := serve(t, d, call{method: "POST", target: "/setup/maxconfirm?enabled=" + enabled, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}})
		if got.Code != http.StatusOK {
			t.Fatalf("enabled=%s: %d %s", enabled, got.Code, got.Body.String())
		}
	}
	if setup, snapshot := asks(); !setup || !snapshot || !d.config.MaxConfirm {
		t.Fatalf("a new config: setup says %v, the snapshot %v; want both on", setup, snapshot)
	}
	turn("0")
	if setup, snapshot := asks(); setup || snapshot || loadConfig().MaxConfirm {
		t.Fatalf("turned off: setup says %v, the snapshot %v, the saved config %v", setup, snapshot, loadConfig().MaxConfirm)
	}
	turn("1")
	if setup, snapshot := asks(); !setup || !snapshot || !loadConfig().MaxConfirm {
		t.Fatalf("turned on again: setup says %v, the snapshot %v, the saved config %v", setup, snapshot, loadConfig().MaxConfirm)
	}
	// Only from this computer, as every setting is.
	if got := serve(t, d, call{method: "POST", target: "/setup/maxconfirm?enabled=0", host: deckOnWiFi, from: aPhone,
		headers: map[string]string{setupHeader: "1"}}); got.Code == http.StatusOK || !d.config.MaxConfirm {
		t.Fatalf("a phone turned it off: %d", got.Code)
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
	if got := local(call{method: "GET", target: "/setup/state", withoutSetupKey: true, headers: map[string]string{setupKeyHeader: "a-made-up-key"}}); got != http.StatusUnauthorized {
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
	// The key is for the header only: as a cookie, it opens nothing.
	if got := local(call{method: "GET", target: "/setup/state", withoutSetupKey: true,
		headers: map[string]string{"Cookie": setupCookie + "=" + d.access.SetupKey}}); got != http.StatusUnauthorized {
		t.Errorf("setup's state with the key as a cookie: %d, want %d", got, http.StatusUnauthorized)
	}
	// Nor does the key open a session at /setup/session: only a token does.
	session := serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", setupKeyHeader: d.access.SetupKey}})
	if session.Code != http.StatusUnauthorized || len(session.Result().Cookies()) != 0 {
		t.Errorf("the key at /setup/session: %d, cookies %v; want %d and none", session.Code, session.Result().Cookies(), http.StatusUnauthorized)
	}
}

func TestAToolSeesPairingButOnlyThePageMakesIt(t *testing.T) {
	d := newTestDeck(t)
	local := func(c call) *httptest.ResponseRecorder {
		c.host, c.from = "localhost:7710", thisComputer
		if c.method == "POST" {
			c.headers = map[string]string{setupHeader: "1"}
		}
		return serve(t, d, c)
	}
	// A tool with the key refuses to pair, unpair or show a code, and shows the rest.
	for _, target := range []string{"/setup/pair", "/setup/unpair?all=1"} {
		if got := local(call{method: "POST", target: target, asTool: true}).Code; got != http.StatusForbidden {
			t.Errorf("%s from a tool with the key: %d, want %d", target, got, http.StatusForbidden)
		}
	}
	if got := local(call{method: "GET", target: "/setup/qr.png", asTool: true}).Code; got != http.StatusForbidden {
		t.Errorf("the pairing QR code for a tool with the key: %d, want %d", got, http.StatusForbidden)
	}
	if code := d.access.ActiveCode(); code != "" {
		t.Fatalf("a tool got a pairing code: %q", code)
	}
	// The page asks for a code and sees it. The tool sees that pairing is open, not the code.
	type pairingOf struct{ Pairing map[string]any }
	var page, tool pairingOf
	if w := local(call{method: "POST", target: "/setup/pair"}); w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &page) != nil || page.Pairing["code"] == nil || page.Pairing["code"] == "" {
		t.Fatalf("the page asking for a code: %d %s", w.Code, w.Body)
	}
	state := local(call{method: "GET", target: "/setup/state", asTool: true})
	if state.Code != http.StatusOK || json.Unmarshal(state.Body.Bytes(), &tool) != nil {
		t.Fatalf("a tool reading setup's state: %d %s", state.Code, state.Body)
	}
	if _, has := tool.Pairing["code"]; has || tool.Pairing["open"] != true || strings.Contains(state.Body.String(), page.Pairing["code"].(string)) {
		t.Errorf("a tool saw the pairing as %v, want open and no code", tool.Pairing)
	}
	// And a tool's other changes don't hand the code over in their answers.
	if w := local(call{method: "POST", target: "/setup/music?enabled=0", asTool: true}); w.Code != http.StatusOK || strings.Contains(w.Body.String(), page.Pairing["code"].(string)) {
		t.Errorf("a tool's change: %d, or it answered with the pairing code", w.Code)
	}
	// Unpairing is the page's too.
	pair(t, d)
	if got := local(call{method: "POST", target: "/setup/unpair?all=1", asTool: true}).Code; got != http.StatusForbidden || len(d.access.Phones.All) != 1 {
		t.Errorf("a tool unpairing: %d, %d phones; want %d and the phone kept", got, len(d.access.Phones.All), http.StatusForbidden)
	}
	if got := local(call{method: "POST", target: "/setup/unpair?all=1"}).Code; got != http.StatusOK || len(d.access.Phones.All) != 0 {
		t.Errorf("the page unpairing: %d, %d phones; want %d and none", got, len(d.access.Phones.All), http.StatusOK)
	}
}

func TestTheLinkGivesThePageASessionCookieThatIsNotTheKey(t *testing.T) {
	d := newTestDeck(t)
	link := d.setupLink("pair")
	_, rest, found := strings.Cut(link, "/setup#o=")
	token, then, _ := strings.Cut(rest, "&")
	if !found || then != "pair" || strings.Contains(link, d.access.SetupKey) {
		t.Fatalf("the link is %s, want a one-time token and no key", link)
	}
	session := serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", setupTokenHeader: token}})
	cookies := session.Result().Cookies()
	if session.Code != http.StatusNoContent || len(cookies) != 1 {
		t.Fatalf("handing the token over: %d, cookies %v", session.Code, cookies)
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/setup" {
		t.Errorf("the cookie can be read by scripts, sent by other sites or to other pages: %+v", cookie)
	}
	if cookie.Value == "" || cookie.Value == d.access.SetupKey {
		t.Errorf("the cookie is %q, which is empty or the setup key", cookie.Value)
	}
	state := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{"Cookie": cookie.Name + "=" + cookie.Value}})
	if state.Code != http.StatusOK {
		t.Errorf("setup's state with the page's cookie: %d, want %d", state.Code, http.StatusOK)
	}
	// The page can pair a phone.
	paired := serve(t, d, call{method: "POST", target: "/setup/pair", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", "Cookie": cookie.Name + "=" + cookie.Value}})
	if paired.Code != http.StatusOK {
		t.Errorf("the page asking for a pairing code: %d, want %d", paired.Code, http.StatusOK)
	}
	// Each start has a new key and no sessions, so a link or cookie from before stops working.
	again := NewDeck()
	if again.access.SetupKey == d.access.SetupKey || again.access.IsSetupSession(cookie.Value) {
		t.Error("a new start kept the setup key or a session")
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
	if first.Code != http.StatusNoContent || len(first.Result().Cookies()) != 1 || first.Result().Cookies()[0].Value == d.access.SetupKey {
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

func TestTheHomeScreenIconIsInThePageItself(t *testing.T) {
	d := newTestDeck(t)
	key := pair(t, d)
	for _, logo := range []string{"smolcorp", "keys"} {
		d.mu.Lock()
		d.config.Logo = logo
		d.mu.Unlock()
		png, err := os.ReadFile(filepath.Join("..", "Resources", "web", "icon-"+logo+".png"))
		if err != nil {
			t.Fatal(err)
		}
		want := `<link rel="apple-touch-icon" sizes="180x180" href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(png) + `">`
		for _, target := range []string{"/", "/?t=" + key} {
			page := serve(t, d, call{method: "GET", target: target, host: deckOnWiFi, from: aPhone}).Body.String()
			if !strings.Contains(page, want) {
				t.Errorf("%s with the %s logo: no icon in the page", target, logo)
			}
			if strings.Contains(page, `rel="apple-touch-icon" href="/icon.png"`) {
				t.Errorf("%s with the %s logo: an icon left for iOS to fetch", target, logo)
			}
		}
	}
}

func TestTheTraysOfficeIsTheOfficeOnThisComputerOnly(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	token := d.access.NewOfficeToken()
	d.mu.Unlock()
	opened := serve(t, d, call{method: "GET", target: "/office?o=" + token, host: "localhost:7710", from: thisComputer})
	if opened.Code != http.StatusSeeOther || opened.Header().Get("Location") != "/office?t="+d.access.WindowKey {
		t.Fatalf("the tray's link gave %d to %q, want the office with its own key", opened.Code, opened.Header().Get("Location"))
	}
	if again := serve(t, d, call{method: "GET", target: "/office?o=" + token, host: "localhost:7710", from: thisComputer}); again.Code == http.StatusSeeOther {
		t.Error("the tray's token worked twice")
	}
	office := serve(t, d, call{method: "GET", target: "/office?t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	if office.Code != http.StatusOK || !strings.Contains(office.Body.String(), "/events") {
		t.Errorf("the office with its own key gave %d, want the deck's page", office.Code)
	}
	if focus := serve(t, d, call{method: "POST", target: "/focus?session=none&t=" + d.access.WindowKey, host: "localhost:7710",
		from: thisComputer}); focus.Code != http.StatusNotFound {
		t.Errorf("the office's own key on this computer: %d, want it to reach the deck (404 for no session)", focus.Code)
	}
	// From the network, it's no key at all, and nor is a token.
	for _, c := range []call{
		{method: "POST", target: "/focus?session=none&t=" + d.access.WindowKey, host: deckOnWiFi, from: aPhone},
		{method: "POST", target: "/focus?session=none&t=" + d.access.WindowKey, host: deckOnWiFi, from: thisComputer},
	} {
		if response := serve(t, d, c); response.Code != http.StatusUnauthorized {
			t.Errorf("the office's own key from %s to %s: %d, want 401", c.from, c.host, response.Code)
		}
	}
	d.mu.Lock()
	token = d.access.NewOfficeToken()
	d.mu.Unlock()
	if response := serve(t, d, call{method: "GET", target: "/office?o=" + token, host: deckOnWiFi, from: aPhone}); response.Code == http.StatusSeeOther {
		t.Error("a phone swapped the tray's token for the office's key")
	}
	if len(d.access.Phones.All) != 0 {
		t.Error("the office on this computer was listed as a phone")
	}
}

func TestOpeningSetupMarksItCompleteOnlyWithTheSetupKey(t *testing.T) {
	d := newTestDeck(t)
	d.config.HasCompletedSetup = false
	open := func(headers map[string]string) {
		t.Helper()
		if w := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer, headers: headers}); w.Code != http.StatusOK {
			t.Fatalf("opening /setup gave %d", w.Code)
		}
	}
	open(nil)
	open(map[string]string{setupKeyHeader: "sk-ant-test0"})
	if d.config.HasCompletedSetup {
		t.Fatal("opening /setup without a valid setup key marked setup complete")
	}
	open(map[string]string{setupKeyHeader: d.access.SetupKey})
	if !d.config.HasCompletedSetup {
		t.Fatal("opening /setup with the setup key left setup incomplete")
	}
}

func TestSetupSessionMarksSetupCompleteOnlyWhenItIsLetIn(t *testing.T) {
	d := newTestDeck(t)
	d.config.HasCompletedSetup = false
	session := func(token string) int {
		t.Helper()
		return serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer,
			withoutSetupKey: true, headers: map[string]string{setupHeader: "1", setupTokenHeader: token}}).Code
	}
	if code := session("not-a-token"); code != http.StatusUnauthorized || d.config.HasCompletedSetup {
		t.Fatalf("a bad token gave %d, complete %v", code, d.config.HasCompletedSetup)
	}
	if code := session(d.access.NewSetupToken()); code != http.StatusNoContent || !d.config.HasCompletedSetup {
		t.Fatalf("a good token gave %d, complete %v", code, d.config.HasCompletedSetup)
	}
}

// Setup says which Smolcorp this is, and its page has the place that shows it.
func TestSetupShowsTheVersion(t *testing.T) {
	was := Version
	t.Cleanup(func() { Version = was })
	Version = "9.8.7-test"
	d := newTestDeck(t)
	var state struct{ Version string }
	got := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	if json.Unmarshal(got.Body.Bytes(), &state) != nil || state.Version != "9.8.7-test" {
		t.Fatalf("setup's state says version %q (%s); want 9.8.7-test", state.Version, got.Body.String())
	}
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer})
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `id="app-version"`) ||
		!strings.Contains(page.Body.String(), "state.version") {
		t.Fatalf("setup.html (%d) has no element or code that shows the version", page.Code)
	}
}

func TestSetupStateNamesThePlatform(t *testing.T) {
	d := newTestDeck(t)
	var state struct{ Platform string }
	got := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	if json.Unmarshal(got.Body.Bytes(), &state) != nil || state.Platform != runtime.GOOS {
		t.Fatalf("setup's state says platform %q (%s); want %s", state.Platform, got.Body.String(), runtime.GOOS)
	}
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	if !strings.Contains(page, "return '/home/you/project'") || !strings.Contains(page, "state.platform === 'windows'") {
		t.Fatal("setup.html doesn't pick the example folder by the platform")
	}
}

func TestTheOfficePaneHeaderDoesNotNameTheMac(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	const head = "<h1>Office</h1><p>"
	i := strings.Index(page, head)
	if i < 0 {
		t.Fatal("the setup page has no Office pane header")
	}
	text, _, _ := strings.Cut(page[i+len(head):], "</p>")
	if strings.Contains(text, "Mac") {
		t.Errorf("the Office pane header names the Mac on every platform: %q", text)
	}
}

func TestTheRoleHintDoesNotLimitHoldingADeskToThePhone(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	for _, phoneOnly := range []string{"On your phone, hold a free desk", "on your phone hold a free desk"} {
		if strings.Contains(page, phoneOnly) {
			t.Errorf("the setup page still says %q, though the browser office can hire too", phoneOnly)
		}
	}
	if strings.Contains(page, "hire a role from a free desk on your phone") {
		t.Error("the Hire switch hint still limits hiring to the phone")
	}
	for _, line := range strings.Split(page, "\n") {
		hint := strings.Contains(line, "free desk") || strings.Contains(line, "hire a role")
		if hint && strings.Contains(line, "phone") && !strings.Contains(line, "browser") {
			t.Errorf("a hold-a-free-desk or hire hint is phone-only: %q", strings.TrimSpace(line))
		}
	}
}

// The Tailscale line on the setup page names a long address with no spaces in
// it: its card has to wrap it, not stretch past the card.
func TestTheSetupPageWrapsTheTailscaleLines(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	for _, selector := range []string{".tailnet code.address", ".tailnet #tailnet-url", ".tailnet #tailnet-found"} {
		start := strings.Index(page, "  "+selector+" {")
		if start < 0 {
			t.Errorf("the setup page has no rule for %s", selector)
			continue
		}
		rule := page[start : start+strings.Index(page[start:], "}")]
		if !strings.Contains(rule, "word-break: break-all") && !strings.Contains(rule, "overflow-wrap: anywhere") {
			t.Errorf("the rule for %s doesn't wrap long addresses: %s", selector, rule)
		}
	}
}

// The Mac app serves this page too: there it says the Mac, and that its office is a window, where
// Windows and Linux say this computer and the browser (SIERRA 1.22.1 m6 and 1.23 m-10, kept native on each).
func TestTheSetupPageSaysMacOnlyOnTheMac(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	for _, want := range []string{
		"function onMac() { return !!(state.officeWindow && state.officeWindow.mac); }",
		"if (onMac()) return 'on your phone or in its window on this Mac';",
		"state.officeWindow.available ? 'on your phone or in its window' : 'on your phone or in the browser'",
		"`https://<${onMac() ? 'this-mac' : 'this-computer'}>.<your-tailnet>.ts.net`",
		"`Couldn't check ${onMac() ? 'this Mac' : 'this computer'}.",
		"if (onMac()) $('layout').querySelector('.pane-head p').textContent = 'How the office looks and lives, on your phone, the Mac and the viewer.';",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the setup page lacks %q", want)
		}
	}
	var state struct{ OfficeWindow struct{ Mac bool } }
	got := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	if json.Unmarshal(got.Body.Bytes(), &state) != nil || state.OfficeWindow.Mac {
		t.Errorf("Windows and Linux say they're the Mac: %s", got.Body.String())
	}
}

func TestTheWelcomeTextDoesNotAssumeAPairedPhone(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	for _, c := range []struct{ start, bad string }{
		{"Your sessions work in a little office", "your phone shows it"},
		{"Your office is ready", "your phone already shows it"},
	} {
		i := strings.Index(page, c.start)
		if i < 0 {
			t.Fatalf("the setup page has no %q sentence", c.start)
		}
		text, _, _ := strings.Cut(page[i:], "</p>")
		if strings.Contains(text, c.bad) || strings.Contains(text, "on your phone") {
			t.Errorf("setup text assumes a paired phone: %q", text)
		}
		// "this computer" is said "this Mac" on the Mac, where the office has a window of its own.
		if !strings.Contains(text, "this computer") {
			t.Errorf("setup text does not say the office is on this computer: %q", text)
		}
	}
}
