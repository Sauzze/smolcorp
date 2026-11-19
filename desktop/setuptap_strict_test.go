package desktop

import (
	"net/http"
	"strings"
	"testing"
)

// The owner's Strict, with the tap (FXBROWSERSTRICT: "your phone can"): a session the tap opened is a browser's, never
// the window's. Where Smolcorp has no Settings window of its own (Linux, Windows without WebView2), it reads and
// changes Setup as a link's session does; beside the window it only looks, with the same words a link's gets. On
// every page route it gets exactly what a link's session gets; it can't clear the "used first" warning, and it isn't
// the window's.
func TestATappedSessionHasALinksPowersAndNoMore(t *testing.T) {
	d, _ := tapDeck(t)
	d.mu.Lock()
	tapped := d.access.NewTappedSetupSession()
	d.mu.Unlock()
	link := browserSession(d)
	for _, own := range []bool{false, true} {
		if own {
			onWindowsWithWebView2(t)
		} else {
			withoutWebView2(t)
		}
		differ := 0
		for _, route := range pageRoutes() {
			method, path := route[0], route[1]
			asTapped := d.windowOnlyRefusal(sessionRequest(method, path, tapped), callerPage)
			asLink := d.windowOnlyRefusal(sessionRequest(method, path, link), callerPage)
			if asTapped != asLink || !own && asTapped != "" {
				differ++
				t.Errorf("own window %v: %s %s: the tapped session is told %q, a link's %q", own, method, path, asTapped, asLink)
			}
		}
		state := setupStateOf(t, withSession(t, d, tapped, "GET", "/setup/state", ""))
		pair := withSession(t, d, tapped, "POST", "/setup/pair", "")
		t.Logf("own window %v: %d page routes, %d told otherwise than a link's; GET /setup/state's note %v; POST /setup/pair %d",
			own, len(pageRoutes()), differ, state["windowOnly"], pair.Code)
		if want := map[bool]int{false: http.StatusOK, true: http.StatusForbidden}[own]; pair.Code != want {
			t.Errorf("own window %v: the tapped session's POST /setup/pair: %d, want %d", own, pair.Code, want)
		}
		if (state["windowOnly"] != nil) != own {
			t.Errorf("own window %v: the tapped session reads the note %v", own, state["windowOnly"])
		}
	}
	withoutWebView2(t)
	if got := withSession(t, d, tapped, "POST", "/setup/used-first/seen", ""); got.Code != http.StatusForbidden ||
		strings.TrimSpace(got.Body.String()) != usedFirstClearElsewhere(false) {
		t.Errorf("the tapped session clears the used-first warning: %d %q", got.Code, got.Body)
	}
	d.mu.Lock()
	window := d.access.IsWindowSession(tapped)
	d.mu.Unlock()
	if window {
		t.Error("the tapped session is the window's")
	}
}
