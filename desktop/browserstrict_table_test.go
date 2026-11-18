package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// pageRoutes is every route the page may use, from the table (routeTable and goOnlyRoutes), and POST /setup/fix, the
// one the table's unlisted rule lets the page in to that B1 names, as a request a row stands for (concreteRoute).
func pageRoutes() [][2]string {
	var routes [][2]string
	for _, row := range slices.Concat(routeTable, goOnlyRoutes) {
		if !slices.Contains(row.callers, "page") {
			continue
		}
		method, path := concreteRoute(fixtureRow{Method: row.method, Path: row.path, Callers: row.callers})
		routes = append(routes, [2]string{method, path})
	}
	return append(routes, [2]string{http.MethodPost, "/setup/fix"})
}

// sessionRequest is a request carrying session's cookie, for asking the class's gate alone.
func sessionRequest(method, path, session string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.AddCookie(&http.Cookie{Name: setupCookie(), Value: session})
	return r
}

// Every route in the class, from the table: under the Mac app, a browser's Setup gets 403 and the plain words, the
// class's gate asked first so no handler runs unless it refuses; Smolcorp's own Settings window passes the gate; and
// off a Mac a browser's Setup passes it too. Every other page route lets the browser read.
func TestEveryWindowOnlyRouteRefusesABrowserOnAMac(t *testing.T) {
	saved := startHire
	t.Cleanup(func() { startHire = saved })
	startHire = func(*exec.Cmd) error { return nil }
	d := newTestDeck(t)
	hostedByTheMacApp(d)
	browser, window := browserSession(d), windowSession(d)
	var inClass [][2]string
	refused, reads := 0, 0
	for _, route := range pageRoutes() {
		method, path := route[0], route[1]
		gate := d.windowOnlyRefusal(sessionRequest(method, path, browser), callerPage)
		if !pageNeedsTheWindow(method, path) {
			reads++
			if gate != "" {
				t.Errorf("%s %s: a read outside the class, refused to the browser: %q", method, path, gate)
			}
			continue
		}
		want := strictChangeWords
		if method == http.MethodGet {
			want = strictReadWords
		}
		if gate != want {
			t.Errorf("%s %s: the class's gate tells the browser %q, want %q", method, path, gate, want)
			continue // never served: its handler would run
		}
		target := path
		if path == "/setup/fix" {
			target += "?fix=homewifi"
		}
		got := withSession(t, d, browser, method, target, "")
		if got.Code != http.StatusForbidden || strings.TrimSpace(got.Body.String()) != want {
			t.Errorf("%s %s: the browser on a Mac got %d %q, want 403 %q", method, target, got.Code, got.Body, want)
		}
		if why := d.windowOnlyRefusal(sessionRequest(method, path, window), callerPage); why != "" {
			t.Errorf("%s %s: the window is refused: %q", method, path, why)
		}
		inClass = append(inClass, route)
		refused++
	}
	// Off a Mac with no window of Smolcorp's own (Linux, a Windows without WebView2), the class's gate lets a browser's
	// Setup through everywhere.
	d.mu.Lock()
	d.mac.host = nil
	d.mu.Unlock()
	for _, route := range inClass {
		if why := d.windowOnlyRefusal(sessionRequest(route[0], route[1], browser), callerPage); why != "" {
			t.Errorf("%s %s: off a Mac a browser's Setup is refused: %q", route[0], route[1], why)
		}
	}
	// Windows with WebView2 has Smolcorp's own Settings window (FXOWNSETTINGS): the same class, in the same words,
	// and that window's session passes.
	onWindowsWithWebView2(t)
	for _, route := range inClass {
		want := strictChangeWords
		if route[0] == http.MethodGet {
			want = strictReadWords
		}
		if why := d.windowOnlyRefusal(sessionRequest(route[0], route[1], browser), callerPage); why != want {
			t.Errorf("%s %s: on Windows with its window, the browser is told %q, want %q", route[0], route[1], why, want)
		}
		if why := d.windowOnlyRefusal(sessionRequest(route[0], route[1], window), callerPage); why != "" {
			t.Errorf("%s %s: on Windows the window is refused: %q", route[0], route[1], why)
		}
	}
	t.Logf("under the Mac app, a browser's Setup: %d routes refused with the plain words, %d reads let in", refused, reads)
	if refused < 100 || reads < 15 {
		t.Errorf("%d refused, %d reads: not the table this test knows", refused, reads)
	}
}

// The route dump (FR1's way: every row, and who it lets in) with the new class: the page's rows that change
// something are "window on a Mac", all of them, and of its reads exactly the five that show a code or hand over a key.
func TestTheRouteDumpShowsTheWindowOnlyClass(t *testing.T) {
	var dump strings.Builder
	var marked, unmarked []string
	for _, row := range slices.Concat(routeTable, goOnlyRoutes) {
		method, path := concreteRoute(fixtureRow{Method: row.method, Path: row.path, Callers: row.callers})
		class := strings.Join(row.callers, ", ")
		inClass := slices.Contains(row.callers, "page") && pageNeedsTheWindow(method, path)
		if inClass {
			class = strings.Replace(class, "page", "page (window on a Mac)", 1)
			marked = append(marked, row.method+" "+row.path)
		} else {
			unmarked = append(unmarked, row.method+" "+row.path)
		}
		fmt.Fprintf(&dump, "%s %s: %s\n", row.method, row.path, class)
		if slices.Contains(row.callers, "page") && method != http.MethodGet && !inClass {
			t.Errorf("%s %s changes something, and isn't in the class", row.method, row.path)
		}
	}
	t.Logf("the route table, with the class:\n%s", dump.String())
	var reads []string
	for _, route := range marked {
		if strings.HasPrefix(route, "GET ") {
			reads = append(reads, route)
		}
	}
	slices.Sort(reads)
	if want := []string{"GET /setup/accounts/login", "GET /setup/peers", "GET /setup/qr.png", "GET /setup/viewer/qr.png"}; !slices.Equal(reads, want) {
		t.Errorf("the reads in the class: %v, want %v", reads, want)
	}
	// GET /setup/viewer is in the class too, but not a row of the table (the unlisted rule lets the page read it).
	if !pageNeedsTheWindow(http.MethodGet, "/setup/viewer") || pageNeedsTheWindow(http.MethodGet, "/setup/state") || pageNeedsTheWindow(http.MethodGet, "/setup") {
		t.Error("GET /setup/viewer must be in the class, GET /setup/state and /setup not")
	}
	for _, read := range []string{"GET /setup/sandbox", "GET /setup/guard", "GET /setup/tailnet/status", "GET /setup/gm/team"} {
		if !slices.Contains(unmarked, read) {
			t.Errorf("%s: a plain read, marked", read)
		}
	}
	t.Logf("%d rows in the class, %d not", len(marked), len(unmarked))
}
