package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The owner's Strict (SANDBOX-PLAN §9 Q8, B8; RT126SWEEP2 P4 B1 and F3): under the Mac app, Setup in a browser
// (Open Settings in Browser), whose cookie other programs on this Mac can read, only looks; Smolcorp's own Settings
// window changes things. And the GM may only turn the webhook off. These rows use what the tree had before the fix
// only, so they run on it too, and fail there (FXBROWSERSTRICT's fail-first); browserstrict_table_test.go holds the
// rows that walk the whole route table.

// The words a browser's Setup on a Mac is told (routes.go's windowOnlyChange and windowOnlyRead).
const (
	strictChangeWords = "Change this in Smolcorp's own Settings window, not in a browser."
	strictReadWords   = "Open this in Smolcorp's own Settings window, not in a browser."
)

// hostedByTheMacApp puts a test deck under the Mac app: a stand-in host that answers nothing.
func hostedByTheMacApp(d *Deck) {
	d.mu.Lock()
	d.mac.host = macAppHost{}
	d.mu.Unlock()
}

// browserSession is a Setup session as a browser's: opened with a link's token, as Open Settings in Browser's is.
func browserSession(d *Deck) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.NewSetupSessionFrom(d.access.NewSetupToken())
}

// windowSession is the session Smolcorp's own Settings window opens (core_darwin.go's setup-session).
func windowSession(d *Deck) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.access.NewSetupSession()
}

// withSession is a request to Setup carrying session's cookie, as a page's request does.
func withSession(t *testing.T, d *Deck, session, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{"Cookie": setupCookie() + "=" + session}
	if method != http.MethodGet && method != http.MethodHead {
		headers[setupHeader] = "1"
	}
	return serve(t, d, call{method: method, target: target, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
		headers: headers, body: body})
}

func setupStateOf(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var state map[string]any
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &state) != nil {
		t.Fatalf("GET /setup/state: %d %q", w.Code, w.Body)
	}
	return state
}

// B1's own routes: a browser's Setup on a Mac turns no sandbox off, pairs no phone, shows no pairing code, answers no
// ask and edits no guard, each in plain words; it still reads Setup, without the codes; Smolcorp's own Settings window
// does each; and off a Mac a browser's Setup is as it was.
func TestOnAMacSetupInABrowserOnlyLooks(t *testing.T) {
	turnOn(t, &spendAsksOn) // the ask it answers is a spend ask, off in this version (FXDORMANTOFF)
	d, _, id := deckWithAnAsk(t)
	hostedByTheMacApp(d)
	d.mu.Lock()
	d.config.Webhook.Secret = "fake-hook-secret"
	d.mu.Unlock()
	browser, window := browserSession(d), windowSession(d)

	for _, c := range []struct{ method, target, body, want string }{
		{"POST", "/setup/sandbox?on=0", "", strictChangeWords},
		{"POST", "/setup/pair", "", strictChangeWords},
		{"GET", "/setup/qr.png", "", strictReadWords},
		{"POST", "/setup/grants/answer?id=" + id + "&answer=yes", "", strictChangeWords},
		{"POST", "/setup/guard", `{}`, strictChangeWords},
	} {
		got := withSession(t, d, browser, c.method, c.target, c.body)
		t.Logf("browser on a Mac: %s %s: %d %s", c.method, c.target, got.Code, strings.TrimSpace(got.Body.String()))
		if got.Code != http.StatusForbidden || strings.TrimSpace(got.Body.String()) != c.want {
			t.Errorf("browser on a Mac: %s %s: %d %q, want 403 %q", c.method, c.target, got.Code, got.Body, c.want)
		}
	}
	d.mu.Lock()
	sandboxOff, code := d.config.SandboxOff, d.access.ActiveCode()
	d.mu.Unlock()
	if sandboxOff || code != "" {
		t.Errorf("after the browser's tries: sandbox off %v, a pairing code open %v", sandboxOff, code != "")
	}
	if state := grantState(t, d, id); state != grantOpen {
		t.Errorf("after the browser's tries the ask is %s", state)
	}

	// Smolcorp's own Settings window pairs, and the browser still reads Setup, with no code and no secret in it.
	if got := withSession(t, d, window, "POST", "/setup/pair", ""); got.Code != http.StatusOK {
		t.Fatalf("the window: POST /setup/pair: %d %q", got.Code, got.Body)
	}
	seen := setupStateOf(t, withSession(t, d, browser, "GET", "/setup/state", ""))
	pairing, _ := seen["pairing"].(map[string]any)
	webhook, _ := seen["webhook"].(map[string]any)
	t.Logf("browser on a Mac: GET /setup/state: pairing open %v, code %v, webhook secret %q, note %q",
		pairing["open"], pairing["code"], webhook["secret"], seen["windowOnly"])
	if _, shown := pairing["code"]; shown || pairing["open"] != true || webhook["secret"] != "" || seen["windowOnly"] == nil {
		t.Errorf("browser on a Mac: the state shows pairing %v, webhook secret %q, note %v", pairing, webhook["secret"], seen["windowOnly"])
	}
	own := setupStateOf(t, withSession(t, d, window, "GET", "/setup/state", ""))
	if ownPairing, _ := own["pairing"].(map[string]any); ownPairing["code"] == nil || own["windowOnly"] != nil {
		t.Errorf("the window's state: pairing %v, note %v", ownPairing, own["windowOnly"])
	}
	if got := withSession(t, d, window, "GET", "/setup/qr.png", ""); got.Code == http.StatusForbidden {
		t.Errorf("the window: GET /setup/qr.png: %d %q", got.Code, got.Body)
	}

	// It answers the ask, and turns the sandbox off where desks start in one.
	if got := withSession(t, d, window, "POST", "/setup/grants/answer?id="+id+"&answer=yes", ""); got.Code/100 != 2 {
		t.Errorf("the window: answering the ask: %d %q", got.Code, got.Body)
	}
	if state := grantState(t, d, id); state != grantGranted {
		t.Errorf("after the window's answer the ask is %s", state)
	}
	want := map[bool]int{true: http.StatusOK, false: http.StatusConflict}[desksSandboxedHere]
	if got := withSession(t, d, window, "POST", "/setup/sandbox?on=0", ""); got.Code != want {
		t.Errorf("the window: POST /setup/sandbox?on=0: %d %q, want %d", got.Code, got.Body, want)
	}

	// Off a Mac (Linux, and Setup in Windows' browser), a browser's Setup is the page, as before.
	d.mu.Lock()
	d.mac.host = nil
	d.mu.Unlock()
	if got := withSession(t, d, browser, "POST", "/setup/pair", ""); got.Code != http.StatusOK {
		t.Errorf("off a Mac: the browser's POST /setup/pair: %d %q", got.Code, got.Body)
	}
}

// F3: the GM may turn the webhook off, and nothing more: not on again, not to another address, not its secret, its
// reach or its events. The owner's page sets them.
func TestTheGMCanOnlyTurnTheWebhookOff(t *testing.T) {
	d := newTestDeck(t)
	gm := d.newGMToken()
	asGM := func(body string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/setup/webhook", host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{gmTokenHeader: gm, setupHeader: "1"}, body: body})
	}
	webhookNow := func() Webhook {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.config.Webhook
	}
	set := serve(t, d, call{method: "POST", target: "/setup/webhook", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}, body: `{"enabled":true,"url":"http://127.0.0.1:9/hook","secret":"fake-hook","outside":false}`})
	if set.Code != http.StatusOK || webhookNow().URL != "http://127.0.0.1:9/hook" || !webhookNow().Enabled {
		t.Fatalf("the page sets the webhook: %d %q, %+v", set.Code, set.Body, webhookNow())
	}
	before := webhookNow()
	for _, body := range []string{
		`{"enabled":true,"url":"http://127.0.0.1:8/elsewhere"}`,
		`{"url":"http://127.0.0.1:8/elsewhere"}`,
		`{"enabled":false,"url":"http://127.0.0.1:8/elsewhere"}`,
		`{"secret":"fake-other"}`,
		`{"outside":true}`,
		`{"enabled":false,"events":["stop"]}`,
		`{}`,
	} {
		got := asGM(body)
		t.Logf("GM %s: %d %s", body, got.Code, strings.TrimSpace(got.Body.String()))
		if got.Code != http.StatusForbidden {
			t.Errorf("GM %s: %d %q, want 403", body, got.Code, got.Body)
		}
		if now := webhookNow(); now.URL != before.URL || now.Secret != before.Secret || now.Enabled != before.Enabled || now.Outside != before.Outside {
			t.Fatalf("GM %s changed the webhook: %+v", body, now)
		}
	}
	got := asGM(`{"enabled":false}`)
	t.Logf(`GM {"enabled":false}: %d`, got.Code)
	if got.Code != http.StatusOK || webhookNow().Enabled || webhookNow().URL != before.URL {
		t.Errorf(`GM {"enabled":false}: %d %q, %+v`, got.Code, got.Body, webhookNow())
	}
	if again := asGM(`{"enabled":true}`); again.Code != http.StatusForbidden || webhookNow().Enabled {
		t.Errorf(`GM {"enabled":true}: %d %q, enabled %v`, again.Code, again.Body, webhookNow().Enabled)
	}
}

// The office's ⚙︎ on a Mac (FXBROWSERSTRICT, 03:3x): Setup in a browser only looks there, so the gear from the Mac's
// office window asks the app, through the bridge, for Smolcorp's own Settings window, as the menu's Settings… does,
// and opens no browser. If the app can't, the office is told why and where Settings are. Off a Mac, the browser as
// before (TestTheCogOpensSetupFromTheOfficesOwnWindowOnly).
func TestOnAMacTheCogOpensSmolcorpsOwnSettingsWindow(t *testing.T) {
	d := newTestDeck(t)
	opened := 0
	saved := openSetupPage
	t.Cleanup(func() { openSetupPage = saved })
	openSetupPage = func(*Deck) error { opened++; return nil }
	fake := &fakeHost{answer: bridgeAnswer{OK: true}}
	d.mu.Lock()
	d.mac.host = fake
	d.mu.Unlock()
	gear := func() *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: "/act?action=settings&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	}
	got := gear()
	var actions []string
	for _, asked := range fake.asked {
		actions = append(actions, asked.Action)
	}
	t.Logf("the gear on a Mac: %d, the bridge asked %v, the browser opened %d times", got.Code, actions, opened)
	if got.Code != http.StatusNoContent || strings.Join(actions, ",") != "settings" || opened != 0 {
		t.Errorf("the gear on a Mac: %d %q, the bridge asked %v, the browser opened %d times; want 204, [settings], 0",
			got.Code, got.Body, actions, opened)
	}

	fake.answer = bridgeAnswer{OK: false, Error: "no window yet"}
	failed := gear()
	t.Logf("the app couldn't: %d %s", failed.Code, strings.TrimSpace(failed.Body.String()))
	if failed.Code != http.StatusBadGateway || !strings.Contains(failed.Body.String(), "no window yet") ||
		!strings.Contains(failed.Body.String(), "menu bar icon, then Settings…") || opened != 0 {
		t.Errorf("the app couldn't: %d %q, the browser opened %d times", failed.Code, failed.Body, opened)
	}

	d.mu.Lock()
	d.mac.host = nil
	d.mu.Unlock()
	asked := len(fake.asked)
	if off := gear(); off.Code != http.StatusNoContent || opened != 1 || len(fake.asked) != asked {
		t.Errorf("off a Mac: %d, the browser opened %d times, the bridge asked %d more", off.Code, opened, len(fake.asked)-asked)
	}
}

// /raise?show=settings on a Mac (RT126FIXREVIEW-OWNSETTINGS N2): the route `smolcorp setup --open` asks on Windows.
// Under the Mac app it does what the office's ⚙︎ does: it asks the app, through the bridge, for Smolcorp's own
// Settings window, and answers 204. It never makes a Setup link and never opens the browser, and it never says
// there's no Settings window. If the app can't, the caller is told why.
func TestOnAMacTheCommandsRaiseOpensSmolcorpsOwnSettingsWindow(t *testing.T) {
	d := newTestDeck(t)
	links := browserOpens(t)
	fake := &fakeHost{answer: bridgeAnswer{OK: true}}
	d.mu.Lock()
	d.mac.host = fake
	d.mu.Unlock()
	raise := func() *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: raisePath + "?show=settings", host: "127.0.0.1:7710", from: thisComputer,
			headers: map[string]string{hookKeyHeader: d.access.HookKey}})
	}
	got := raise()
	var actions []string
	for _, asked := range fake.asked {
		actions = append(actions, asked.Action)
	}
	t.Logf("/raise?show=settings on a Mac: %d %q, the bridge asked %v, the browser opened %v", got.Code, strings.TrimSpace(got.Body.String()), actions, *links)
	if got.Code != http.StatusNoContent || strings.Join(actions, ",") != "settings" || len(*links) != 0 {
		t.Errorf("/raise?show=settings on a Mac: %d %q, the bridge asked %v, the browser opened %v; want 204, [settings], none",
			got.Code, got.Body, actions, *links)
	}

	fake.answer = bridgeAnswer{OK: false, Error: "no window yet"}
	failed := raise()
	t.Logf("the app couldn't: %d %s", failed.Code, strings.TrimSpace(failed.Body.String()))
	if failed.Code != http.StatusBadGateway || !strings.Contains(failed.Body.String(), "no window yet") || len(*links) != 0 ||
		strings.Contains(failed.Body.String(), "no Settings window of its own") {
		t.Errorf("the app couldn't: %d %q, the browser opened %v", failed.Code, failed.Body, *links)
	}
}
