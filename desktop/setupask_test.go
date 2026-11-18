package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

// Setup opened by a tap on the phone (setupask.go, OLD-TAP1; the owner's Q-F14).

// The page's route (POST asks, GET waits) and the owner's tap on the phone, as server.go, routes.go and the row write them.
const (
	setupTapPath       = "/setup/tap"
	setupTapAnswerPath = "/grants/setup"
)

// tapDeck is probeDeck (a paired phone, Tailscale at 100.64.1.2), off a Mac, with asks taken as fast as the test
// sends them and a short wait, and the phone's key.
func tapDeck(t *testing.T) (*Deck, string) {
	t.Helper()
	d, phoneKey := probeDeck(t)
	shortAskWait(t)
	saved := setupAskEvery
	setupAskEvery = 0
	t.Cleanup(func() { setupAskEvery = saved })
	return d, phoneKey
}

// askToOpenSetup is the setup page's POST /setup/tap, from this computer, with no key.
func askToOpenSetup(t *testing.T, d *Deck) (int, map[string]any) {
	t.Helper()
	w := serve(t, d, call{method: "POST", target: setupTapPath, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1"}})
	var said map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &said)
	return w.Code, said
}

// waitToOpenSetup is the page's GET /setup/tap with its ticket: the answer, its state, and the cookies it set.
func waitToOpenSetup(t *testing.T, d *Deck, ticket string) (int, string, []*http.Cookie) {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: setupTapPath + "?ticket=" + url.QueryEscape(ticket), host: "localhost:7710", from: thisComputer,
		withoutSetupKey: true, headers: map[string]string{setupHeader: "1"}})
	var said struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &said)
	return w.Code, said.State, w.Result().Cookies()
}

// callerNamed is one of routeCallers.
func callerNamed(t *testing.T, name string) routeCaller {
	t.Helper()
	i := slices.IndexFunc(routeCallers, func(c routeCaller) bool { return c.name == name })
	if i < 0 {
		t.Fatalf("no caller %s", name)
	}
	return routeCallers[i]
}

// phoneSetupAsks is GET /grants's setupAsks, as the paired phone over Tailscale reads them, and the whole body.
func phoneSetupAsks(t *testing.T, d *Deck, phoneKey string) ([]map[string]any, string) {
	t.Helper()
	w := probeAnswerWith(t, d, "GET", "/grants", "", phoneKey, callerNamed(t, "phone-wireguard"))
	var said struct {
		SetupAsks []map[string]any `json:"setupAsks"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &said) != nil {
		t.Fatalf("GET /grants: %d %s", w.Code, w.Body)
	}
	return said.SetupAsks, w.Body.String()
}

// tapSetup is POST /grants/setup as one caller.
func tapSetup(t *testing.T, d *Deck, phoneKey, id, code, answer string, c routeCaller) (int, string) {
	t.Helper()
	return probeRouteWith(t, d, "POST", setupTapAnswerPath+"?id="+url.QueryEscape(id)+"&code="+url.QueryEscape(code)+"&answer="+answer, "",
		phoneKey, c)
}

// openWithCookie is GET /setup/state with a session's cookie.
func openWithCookie(t *testing.T, d *Deck, session string) int {
	t.Helper()
	return serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", "Cookie": setupCookie() + "=" + session}}).Code
}

// handOverLink is POST /setup/session with a fresh menu link's token.
func handOverLink(t *testing.T, d *Deck) *httptest.ResponseRecorder {
	t.Helper()
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	return serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: map[string]string{setupHeader: "1", setupTokenHeader: token}})
}

// Off a Mac, before any phone is paired, the menu's link opens Setup and nothing can be asked of a phone; once one
// is paired, no link opens it (the menu opens Setup with no link, at #tap) and the page's ask, with no key, gets a
// code and a ticket. The phone lists the ask with this computer's name and the code, never the ticket. Only the
// owner's yes, with that code, opens Setup, and only for the page holding the ticket, once.
func TestOffAMacSetupOpensByThePhonesTapOnce(t *testing.T) {
	d := newTestDeck(t)
	shortAskWait(t)
	saved, savedName, savedOpen := setupAskEvery, setupComputerName, openURL
	setupAskEvery, setupComputerName = 0, func() string { return "studio-pc" }
	var opened []string
	openURL = func(link string) error { opened = append(opened, link); return nil }
	t.Cleanup(func() { setupAskEvery, setupComputerName, openURL = saved, savedName, savedOpen })

	// No phone yet: the link opens Setup, and an ask is refused, as nothing could answer it.
	if code, _ := askToOpenSetup(t, d); code != http.StatusConflict {
		t.Errorf("an ask with no phone paired: %d", code)
	}
	if w := handOverLink(t, d); w.Code != http.StatusNoContent || len(w.Result().Cookies()) != 1 {
		t.Fatalf("the link with no phone paired: %d", w.Code)
	}
	if err := d.openSetup(""); err != nil || len(opened) != 1 || !strings.Contains(opened[0], "/setup#o=") {
		t.Fatalf("the menu with no phone paired opened %v (%v)", opened, err)
	}
	// The Windows window's own paired device is no phone.
	d.mu.Lock()
	ensureWindowKey(d.access.Phones, &d.access.windowDeviceKey)
	d.mu.Unlock()
	if d.setupLinkGone() {
		t.Fatal("the window's own device took the link away")
	}

	d.mu.Lock()
	d.config.PhoneTextWiFi = true
	phoneKey := d.access.Phones.Pair("iPhone", true)
	d.mu.Unlock()
	saveTailscale := tailscaleAddresses
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	t.Cleanup(func() { tailscaleAddresses = saveTailscale })

	// A phone is paired: a link opens nothing, and the menu opens Setup with none.
	if w := handOverLink(t, d); w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0 || !strings.Contains(w.Body.String(), setupLinkGoneWords) {
		t.Errorf("a link once a phone is paired: %d %s", w.Code, w.Body)
	}
	opened = nil
	if err := d.openSetup("pair"); err != nil || len(opened) != 1 || !strings.HasSuffix(opened[0], "/setup#tap&pair") || strings.Contains(opened[0], "o=") {
		t.Errorf("the menu once a phone is paired opened %v (%v)", opened, err)
	}
	var printed strings.Builder
	printSetupLink(&printed, d)
	if strings.Contains(printed.String(), "o=") || !strings.Contains(printed.String(), "/setup#tap") {
		t.Errorf("with no tray, the address printed: %q", printed.String())
	}

	// A web page can't ask: it can't send the setup header.
	if w := serve(t, d, call{method: "POST", target: setupTapPath, host: "localhost:7710", from: thisComputer, withoutSetupKey: true}); w.Code != http.StatusForbidden {
		t.Errorf("an ask without the setup header: %d", w.Code)
	}
	// Nor anywhere but this computer's 127.0.0.1, where Setup's cookie lives.
	if w := serve(t, d, call{method: "POST", target: setupTapPath, host: "localhost:7710", from: thisComputer, withoutSetupKey: true, keepHost: true,
		headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusForbidden {
		t.Errorf("an ask at localhost: %d", w.Code)
	}
	if w := serve(t, d, call{method: "POST", target: setupTapPath, host: deckOnWiFi, from: aPhone, withoutSetupKey: true, bare: true,
		headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusNotFound {
		t.Errorf("an ask from the Wi-Fi: %d", w.Code)
	}

	code, asked := askToOpenSetup(t, d)
	ticket, _ := asked["ticket"].(string)
	shown, _ := asked["code"].(string)
	if code != http.StatusOK || ticket == "" || len(shown) != 7 || asked["expiresIn"] != float64(askLifetime.Seconds()) {
		t.Fatalf("the ask: %d %v", code, asked)
	}
	asks, body := phoneSetupAsks(t, d, phoneKey)
	if len(asks) != 1 || asks[0]["code"] != shown || asks[0]["computer"] != "studio-pc" || asks[0]["id"] == "" || asks[0]["expires"] == nil {
		t.Fatalf("the phone's list: %v", asks)
	}
	if strings.Contains(body, ticket) {
		t.Error("the phone's list carries the ticket")
	}
	// Nor does the setup page's state, nor the approvals a GM or a tool reads.
	for _, target := range []string{"/setup/state", "/setup/gm/approvals"} {
		if w := serve(t, d, call{method: "GET", target: target, host: "localhost:7710", from: thisComputer, asTool: true}); strings.Contains(w.Body.String(), ticket) ||
			strings.Contains(w.Body.String(), asks[0]["id"].(string)) {
			t.Errorf("%s carries the ask", target)
		}
	}
	id := asks[0]["id"].(string)

	// Waiting: no cookie.
	if status, state, cookies := waitToOpenSetup(t, d, ticket); status != http.StatusOK || state != "waiting" || len(cookies) != 0 {
		t.Errorf("before the tap: %d %q %d cookie(s)", status, state, len(cookies))
	}
	phone := callerNamed(t, "phone-wireguard")
	if status, _ := tapSetup(t, d, phoneKey, id, "0-0-0-0", "allow", phone); status != http.StatusNotFound && shown != "0-0-0-0" {
		t.Errorf("a yes with another code: %d", status)
	}
	if status, text := tapSetup(t, d, phoneKey, id, shown, "maybe", phone); status != http.StatusBadRequest {
		t.Errorf("an answer that isn't one: %d %s", status, text)
	}
	if status, text := tapSetup(t, d, phoneKey, id, shown, "allow", phone); status != http.StatusOK {
		t.Fatalf("the owner's yes: %d %s", status, text)
	}
	if status, _ := tapSetup(t, d, phoneKey, id, shown, "allow", phone); status != http.StatusNotFound {
		t.Errorf("a second yes: %d", status)
	}
	if asks, _ := phoneSetupAsks(t, d, phoneKey); len(asks) != 0 {
		t.Errorf("an answered ask is still listed: %v", asks)
	}
	// A ticket made up gets nothing.
	if status, _, cookies := waitToOpenSetup(t, d, "made-up-ticket"); status != http.StatusNotFound || len(cookies) != 0 {
		t.Errorf("a made-up ticket: %d, %d cookie(s)", status, len(cookies))
	}
	status, state, cookies := waitToOpenSetup(t, d, ticket)
	if status != http.StatusOK || state != "allowed" || len(cookies) != 1 || cookies[0].Name != setupCookie() || !cookies[0].HttpOnly ||
		cookies[0].Path != "/setup" || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("after the yes: %d %q %v", status, state, cookies)
	}
	if got := openWithCookie(t, d, cookies[0].Value); got != http.StatusOK {
		t.Errorf("the session the tap opened: %d", got)
	}
	d.mu.Lock()
	s := d.access.setupSessionOf(cookies[0].Value)
	tapped := s != nil && s.tapped && !s.window && s.token == ""
	d.mu.Unlock()
	if !tapped {
		t.Errorf("the session isn't the tap's: %+v", s)
	}
	// Once: the ticket opens nothing more.
	if status, _, cookies := waitToOpenSetup(t, d, ticket); status != http.StatusNotFound || len(cookies) != 0 {
		t.Errorf("the ticket again: %d, %d cookie(s)", status, len(cookies))
	}
}

// Only the owner's tap answers: a paired phone over Tailscale. The Wi-Fi's phone, the office's window or a browser's
// on this computer, the Windows window's paired device, a program here with a proxy's header, a stranger, and the
// GM's token, a tool's key or a hire's key leave the ask waiting, and its page without a cookie.
func TestOnlyTheOwnersTapOpensSetup(t *testing.T) {
	d, phoneKey := tapDeck(t)
	for _, c := range routeCallers {
		_, asked := askToOpenSetup(t, d)
		ticket := asked["ticket"].(string)
		asks, _ := phoneSetupAsks(t, d, phoneKey)
		var id, code string
		for _, ask := range asks {
			if ask["code"] == asked["code"] {
				id, code = ask["id"].(string), ask["code"].(string)
			}
		}
		status, _ := tapSetup(t, d, phoneKey, id, code, "allow", c)
		owner := slices.Contains(classLetsIn[accessOwnerTap], c.name)
		_, state, cookies := waitToOpenSetup(t, d, ticket)
		if owner != (status == http.StatusOK) || owner != (state == "allowed") || owner != (len(cookies) == 1) {
			t.Errorf("%s: %d, the page %q with %d cookie(s)", c.name, status, state, len(cookies))
		}
		if !owner {
			if deny, _ := tapSetup(t, d, phoneKey, id, code, "deny", callerNamed(t, "phone-wireguard")); deny != http.StatusOK {
				t.Fatalf("%s: the owner's no to clear it: %d", c.name, deny)
			}
			_, _, _ = waitToOpenSetup(t, d, ticket)
		}
	}
	// Keys that are none of the owner's, from this computer.
	_, asked := askToOpenSetup(t, d)
	asks, _ := phoneSetupAsks(t, d, phoneKey)
	id, code := asks[0]["id"].(string), asks[0]["code"].(string)
	d.mu.Lock()
	gmToken, toolKey := d.gmToken, d.access.SetupKey
	d.mu.Unlock()
	for name, header := range map[string][2]string{"the GM's token": {gmTokenHeader, gmToken}, "a tool's key": {setupKeyHeader, toolKey},
		"a hire's key": {hireKeyHeader, "fake-hire-key"}} {
		w := serve(t, d, call{method: "POST", target: setupTapAnswerPath + "?id=" + id + "&code=" + code + "&answer=allow", host: "localhost:7710",
			from: thisComputer, headers: map[string]string{header[0]: header[1]}})
		if letIn(w.Code) {
			t.Errorf("%s answered: %d", name, w.Code)
		}
	}
	if _, state, cookies := waitToOpenSetup(t, d, asked["ticket"].(string)); state != "waiting" || len(cookies) != 0 {
		t.Errorf("after the keys that aren't the owner's: %q, %d cookie(s)", state, len(cookies))
	}
}

// Several racing asks each get a code of their own; the phone lists each; the owner's yes to one opens that page
// alone, a no tells its page so with no cookie, and the rest wait. At most three wait, one new every two seconds.
func TestRacingSetupAsksEachGetTheirOwnCode(t *testing.T) {
	d, phoneKey := tapDeck(t)
	var tickets []string
	codes := map[string]string{} // code → ticket
	for i := 0; i < asksMost; i++ {
		status, asked := askToOpenSetup(t, d)
		if status != http.StatusOK {
			t.Fatalf("ask %d: %d", i+1, status)
		}
		tickets = append(tickets, asked["ticket"].(string))
		codes[asked["code"].(string)] = asked["ticket"].(string)
	}
	if len(codes) != asksMost {
		t.Errorf("two asks share a code: %v", codes)
	}
	if status, _ := askToOpenSetup(t, d); status != http.StatusTooManyRequests {
		t.Errorf("one ask too many: %d", status)
	}
	asks, _ := phoneSetupAsks(t, d, phoneKey)
	if len(asks) != asksMost {
		t.Fatalf("the phone lists %d", len(asks))
	}
	phone := callerNamed(t, "phone-wireguard")
	// One code's id with another's code is no ask.
	if status, _ := tapSetup(t, d, phoneKey, asks[0]["id"].(string), asks[1]["code"].(string), "allow", phone); status != http.StatusNotFound {
		t.Errorf("an id with another ask's code: %d", status)
	}
	yes, no := asks[0], asks[1]
	if status, _ := tapSetup(t, d, phoneKey, yes["id"].(string), yes["code"].(string), "allow", phone); status != http.StatusOK {
		t.Fatalf("the yes: %d", status)
	}
	if status, _ := tapSetup(t, d, phoneKey, no["id"].(string), no["code"].(string), "deny", phone); status != http.StatusOK {
		t.Fatalf("the no: %d", status)
	}
	for code, ticket := range codes {
		_, state, cookies := waitToOpenSetup(t, d, ticket)
		want := map[string]string{yes["code"].(string): "allowed", no["code"].(string): "denied"}[code]
		if want == "" {
			want = "waiting"
		}
		if state != want || (len(cookies) == 1) != (want == "allowed") {
			t.Errorf("the page with %s: %q, %d cookie(s), want %q", code, state, len(cookies), want)
		}
	}
	// One new ask every two seconds.
	setupAskEvery = time.Hour
	d.setupAsks.last = time.Time{}
	if status, _ := askToOpenSetup(t, d); status != http.StatusOK {
		t.Errorf("an ask with room: %d", status)
	}
	if status, _ := askToOpenSetup(t, d); status != http.StatusTooManyRequests {
		t.Errorf("an ask right after another: %d", status)
	}
}

// An ask nobody answers runs out: the phone no longer lists it, a yes finds nothing, and its page is told to ask again.
func TestASetupAskRunsOut(t *testing.T) {
	d, phoneKey := tapDeck(t)
	_, asked := askToOpenSetup(t, d)
	asks, _ := phoneSetupAsks(t, d, phoneKey)
	d.setupAsks.mu.Lock()
	for _, ask := range d.setupAsks.all {
		ask.asked = ask.asked.Add(-askLifetime - time.Second)
	}
	d.setupAsks.mu.Unlock()
	if again, _ := phoneSetupAsks(t, d, phoneKey); len(again) != 0 {
		t.Errorf("a run-out ask is listed: %v", again)
	}
	if status, _ := tapSetup(t, d, phoneKey, asks[0]["id"].(string), asks[0]["code"].(string), "allow", callerNamed(t, "phone-wireguard")); status != http.StatusNotFound {
		t.Errorf("a yes to a run-out ask: %d", status)
	}
	if status, _, cookies := waitToOpenSetup(t, d, asked["ticket"].(string)); status != http.StatusNotFound || len(cookies) != 0 {
		t.Errorf("the page of a run-out ask: %d, %d cookie(s)", status, len(cookies))
	}
}

// Under the Mac app none of it: its Settings window opens Setup with no link, no ask is taken, and a paired phone
// doesn't take the menu's link away.
func TestUnderTheMacAppSetupIsntAskedOfThePhone(t *testing.T) {
	d := newMacTestDeck(t)
	d.mu.Lock()
	d.access.Phones.Pair("iPhone", true)
	d.mu.Unlock()
	if d.setupLinkGone() {
		t.Error("the link is gone under the Mac app")
	}
	if code, _ := askToOpenSetup(t, d); code != http.StatusForbidden {
		t.Errorf("an ask under the Mac app: %d", code)
	}
	if w := handOverLink(t, d); w.Code != http.StatusNoContent {
		t.Errorf("the link under the Mac app with a phone paired: %d", w.Code)
	}
}

// A link used first before the phone was paired is still reported as used first once it is, not merely refused.
func TestALinkUsedFirstIsReportedAfterThePhoneIsPaired(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	token := d.access.NewSetupToken()
	d.mu.Unlock()
	handOver := func() *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/setup/session", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", setupTokenHeader: token}})
	}
	if first := handOver(); first.Code != http.StatusNoContent {
		t.Fatalf("the first use: %d", first.Code)
	}
	d.mu.Lock()
	d.access.Phones.Pair("iPhone", true)
	d.mu.Unlock()
	if again := handOver(); again.Code != http.StatusConflict || !strings.Contains(again.Body.String(), "usedFirst") {
		t.Errorf("the same link again, once a phone is paired: %d %s", again.Code, again.Body)
	}
}
