//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// The demo's page where Smolcorp has a Settings window of its own (FXDEMOMAC): under the Mac app, and on Windows with
// WebView2, a browser's Setup only looks (routes.go's windowOnlyRefusal), so the demo's page opens in that window,
// whose session turns the switches on and hires. On Linux it opens in the browser, as before. These rows walk the
// page as setup.html does, with the session the page really gets, and no window and no browser opens.

// demoOpenedAt is where POST /setup/demo/open had the demo's page open, and the session that page then has.
type demoOpenedAt struct {
	where   string // "browser", "the Mac's window", "Windows' window", or "" for nowhere
	address string // the page's address, as opened
	session string
}

// openTheDemosPage is the demo's command asking the app to open its page (a tool's key), and the page arriving
// there: a browser's link hands its token over (followLink), the Mac's Settings window opens its own session as it
// loads (core_darwin.go's setup-session: windowSession), and Windows' window hands over the token its script holds.
func openTheDemosPage(t *testing.T, d *Deck, f demoFolder, mac *fakeHost, window *standInSettings) (int, string, demoOpenedAt) {
	t.Helper()
	links := browserOpens(t)
	askedBefore := 0
	if mac != nil {
		askedBefore = len(mac.asked)
	}
	code, body := demoPageCall(t, d, true, "POST", "/setup/demo/open?root="+url.QueryEscape(f.root), "")
	var opened []demoOpenedAt
	for _, link := range *links {
		opened = append(opened, demoOpenedAt{where: "browser", address: link, session: followLink(t, d, link)})
	}
	if mac != nil {
		for _, asked := range mac.asked[askedBefore:] {
			if asked.Action == "settings" {
				opened = append(opened, demoOpenedAt{where: "the Mac's window", address: "/setup#" + bridgeThen(asked), session: windowSession(d)})
			}
		}
	}
	if window != nil {
		runs, forwards := window.seen()
		for _, run := range runs {
			session, _ := openWindowSession(t, d, run)
			opened = append(opened, demoOpenedAt{where: "Windows' window", address: run.url, session: session})
		}
		for _, then := range forwards {
			opened = append(opened, demoOpenedAt{where: "Windows' window, brought forward", address: "/setup#" + then})
		}
	}
	if len(opened) > 1 {
		t.Fatalf("the demo's page opened %d times: %+v", len(opened), opened)
	}
	if len(opened) == 0 {
		return code, body, demoOpenedAt{}
	}
	return code, body, opened[0]
}

// bridgeThen is what the bridge's settings was asked to open Setup at ("" for Setup as it is), as the app reads it
// from the request's JSON.
func bridgeThen(asked bridgeRequest) string {
	data, _ := json.Marshal(asked)
	var fields struct{ Then string }
	json.Unmarshal(data, &fields)
	return fields.Then
}

// demoHireBody is the hire the page sends: the one the app read, as is.
func demoHireBody(t *testing.T, hire map[string]any) string {
	t.Helper()
	data, err := json.Marshal(hire)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// demoRootAt is the demo folder the page reads from its address, as setup.html's demoFromAddress does: the query's
// ?demo=, or, in a Settings window, which is opened on what follows #, the #demo= there.
func demoRootAt(address string) string {
	link, err := url.Parse(address)
	if err != nil {
		return ""
	}
	if root := link.Query().Get("demo"); root != "" {
		return root
	}
	fragment, _ := url.ParseQuery(link.Fragment)
	return fragment.Get("demo")
}

// walkTheDemosPage is the page, with session, as setup.html walks it: Setup's state, the demo's hire, "the page is
// open", then your Hire: the switches it needs on, and its own POST /setup/hire. Each answer is logged; a refusal of
// Setup's reads or of your Hire ends the walk, as it ends the page's, and is what it returns ("" when the GM was hired).
func walkTheDemosPage(t *testing.T, d *Deck, f demoFolder, session string) string {
	t.Helper()
	step := func(method, target, body string) (int, string) {
		w := withSession(t, d, session, method, target, body)
		d.watching.Wait()
		answer := strings.TrimSpace(w.Body.String())
		if len(answer) > 160 {
			answer = answer[:160] + "…"
		}
		t.Logf("  %s %s: %d %s", method, target, w.Code, answer)
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	if code, body := step("GET", "/setup/state", ""); code != http.StatusOK {
		return "GET /setup/state: " + body
	}
	code, body := step("GET", "/setup/demo?root="+url.QueryEscape(f.root), "")
	if code != http.StatusOK {
		return "GET /setup/demo: " + body
	}
	demo := readDemoPage(t, d, true, f.root).Demo
	// The page doesn't wait on this one, nor read its answer (setup.html's demoPageSays): it's said, and the walk goes on.
	if code, body := step("POST", "/setup/demo/page?root="+url.QueryEscape(f.root)+"&state=open", ""); code != http.StatusNoContent {
		t.Errorf("the page saying it's open: %d %s", code, body)
	}
	for _, s := range demo.Switches {
		if !s.On {
			if code, body := step("POST", s.Route+"?enabled=1", ""); code >= 300 {
				return s.Route + ": " + body
			}
		}
	}
	hire := demoHireBody(t, demo.Hire)
	if code, body := step("POST", "/setup/hire", hire); code != http.StatusOK {
		return "POST /setup/hire: " + body
	}
	return ""
}

// The step-1 row (FXDEMOMAC): `smolcorp demo` asks the app to open its page, and you click Hire there. Under the Mac
// app the page opens in Smolcorp's own Settings window (the bridge's settings, at #demo=), on Windows with WebView2 in
// its window, and on Linux in the browser; in each, the page's walk hires the demo's General Manager, held to the
// demo's project. Before the fix the page opened in the browser everywhere, and on the Mac and Windows its first
// change was refused (403, "Change this in Smolcorp's own Settings window, not in a browser.").
func TestTheDemosPageHiresWhereItOpens(t *testing.T) {
	for _, c := range []struct {
		system, want string
	}{
		{"the Mac app", "the Mac's window"},
		{"Windows with WebView2", "Windows' window"},
		{"Linux", "browser"},
	} {
		t.Run(c.system, func(t *testing.T) {
			d, f := demoPageDeck(t)
			var mac *fakeHost
			var window *standInSettings
			switch c.system {
			case "the Mac app":
				withoutWebView2(t)
				mac = &fakeHost{answer: bridgeAnswer{OK: true}}
				d.mu.Lock()
				d.mac.host = mac
				d.mu.Unlock()
			case "Windows with WebView2":
				window = onWindowsWithWebView2(t)
			default:
				withoutWebView2(t)
			}
			code, body, at := openTheDemosPage(t, d, f, mac, window)
			t.Logf("%s: POST /setup/demo/open: %d %q; opened in %q at %s", c.system, code, body, at.where, at.address)
			if code != http.StatusNoContent || at.where != c.want || demoRootAt(at.address) != f.root || at.session == "" {
				t.Errorf("%s: the demo's page: %d %q, opened in %q at %q (a session: %v); want 204, in %q at the demo",
					c.system, code, body, at.where, at.address, at.session != "", c.want)
			}
			if at.session == "" {
				t.FailNow()
			}
			// The page's walk, with the session it got wherever it opened: logged, request by request.
			if refused := walkTheDemosPage(t, d, f, at.session); refused != "" {
				t.Fatalf("%s: the demo's page couldn't hire: %s", c.system, refused)
			}
			hires := registered(t)
			if len(hires) != 1 || hires[0].Role != gmRole || hires[0].Scope != f.repo {
				t.Fatalf("%s: after the page's Hire: %+v", c.system, hires)
			}
			// BROWSERSTRICT's rule stays where there's a window: a browser's Setup beside it still hires nobody.
			if c.system != "Linux" {
				d.mu.Lock()
				d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
				d.mu.Unlock()
				demo := readDemoPage(t, d, true, f.root).Demo
				got := withSession(t, d, browserSession(d), "POST", "/setup/hire", demoHireBody(t, demo.Hire))
				t.Logf("%s: a browser's POST /setup/hire beside the window: %d %s", c.system, got.Code, strings.TrimSpace(got.Body.String()))
				if got.Code != http.StatusForbidden || strings.TrimSpace(got.Body.String()) != strictChangeWords {
					t.Errorf("%s: a browser's hire beside the window: %d %q, want 403 %q", c.system, got.Code, got.Body, strictChangeWords)
				}
			}
		})
	}
}

// Where Smolcorp has a Settings window of its own, the demo's page never falls back to the browser, whose Setup
// only looks there: Windows' window already open is brought forward at the demo (settingsWindowReload loads it there),
// and when the Mac app can't show its window, the command is told why, nothing opens, and the demo may ask again.
func TestTheDemosPageNeverFallsBackToTheBrowserBesideAWindow(t *testing.T) {
	t.Run("Windows' window already open", func(t *testing.T) {
		d, f := demoPageDeck(t)
		window := onWindowsWithWebView2(t)
		if err := d.showSettingsWindow(""); err != nil {
			t.Fatal(err)
		}
		links := browserOpens(t)
		code, body := demoPageCall(t, d, true, "POST", "/setup/demo/open?root="+url.QueryEscape(f.root), "")
		runs, forwards := window.seen()
		t.Logf("POST /setup/demo/open: %d %q; windows opened %d, brought forward on %q, the browser opened %v", code, body, len(runs), forwards, *links)
		if code != http.StatusNoContent || len(runs) != 1 || len(forwards) != 1 || forwards[0] != "demo="+url.QueryEscape(f.root) || len(*links) != 0 {
			t.Fatalf("the open window: %d %q, %d runs, forwards %q, browser %v", code, body, len(runs), forwards, *links)
		}
		if demoRootAt("/setup#"+forwards[0]) != f.root || !strings.Contains(settingsWindowReload(forwards[0]), "location.reload()") {
			t.Errorf("brought forward at %q: %s", forwards[0], settingsWindowReload(forwards[0]))
		}
	})
	t.Run("the Mac app can't", func(t *testing.T) {
		d, f := demoPageDeck(t)
		withoutWebView2(t)
		mac := &fakeHost{answer: bridgeAnswer{OK: false, Error: "no window yet"}}
		d.mu.Lock()
		d.mac.host = mac
		d.mu.Unlock()
		links := browserOpens(t)
		target := "/setup/demo/open?root=" + url.QueryEscape(f.root)
		code, body := demoPageCall(t, d, true, "POST", target, "")
		t.Logf("the app couldn't: %d %s; the browser opened %v", code, body, *links)
		if code != http.StatusBadGateway || !strings.Contains(body, "no window yet") || !strings.Contains(body, "Settings window") ||
			strings.Contains(body, "--no-open") || len(*links) != 0 {
			t.Fatalf("the app couldn't: %d %q, the browser opened %v", code, body, *links)
		}
		mac.answer = bridgeAnswer{OK: true}
		if code, body := demoPageCall(t, d, true, "POST", target, ""); code != http.StatusNoContent || len(*links) != 0 {
			t.Errorf("asked again: %d %q, the browser opened %v", code, body, *links)
		}
		var thens []string
		for _, asked := range mac.asked {
			if asked.Action == "settings" {
				thens = append(thens, bridgeThen(asked))
			}
		}
		if want := "demo=" + url.QueryEscape(f.root); len(thens) != 2 || thens[0] != want || thens[1] != want {
			t.Errorf("the bridge's settings asked at %q, want twice at %q", thens, want)
		}
	})
}
