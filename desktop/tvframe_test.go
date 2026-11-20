package desktop

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The TV's big preview (TV3) frames a desk's own page, http://localhost:<its port>, in the office's window. That page
// is anyone's code, so whatever it asks of the computer must get it nothing: no window route (it has no window key:
// the office's page is another origin, which it can't read), nothing of Setup's, and no page of the computer's it could
// load in its own frame, where its sandbox's allow-same-origin would make it the office's own origin.
//
// The office over the frame isn't always at localhost. The Mac's window is at 127.0.0.1:<Port>, Setup's own host, and so
// is a browser office opened from Setup's line, in a browser that holds Setup's cookie. That still gives the frame
// nothing of Setup's: the frame's own page is at localhost, another site from 127.0.0.1, so Setup's SameSite=Strict
// cookie is never sent with what it asks, whatever the office's address; frame-src lets the frame show
// http://localhost:* alone, so it never becomes a 127.0.0.1 page (nor does the Mac's window, OfficeWindow.decide); its
// referrer is never sent, so the window's key in the office's address doesn't reach the desk's server; and Setup's
// changes want its own header, whose preflight the core never answers. The Mac's window keeps nothing of Setup's either:
// its store is its own and not kept (.nonPersistent()). At localhost, Setup only sends the browser away.
//
// The frame's requests are made here as a browser makes them, from this computer, at localhost and at 127.0.0.1, with
// every header it may add, Setup's own included (as if a preflight had let it: the core answers none). Worst case, the
// cookie too: at localhost it's refused or sent away. The deck has no desks, and each request names one that isn't
// there, so nothing a route does reaches a terminal, a browser or a window.

// framedHeaders are what a page framed at localhost:3000 sends with a request it makes, plus Setup's custom header.
var framedHeaders = map[string]string{
	"Origin": "http://localhost:3000", "Referer": "http://localhost:3000/", "Sec-Fetch-Site": "same-site",
	"Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty", setupHeader: "1",
}

func TestAFramedPreviewReachesNoWindowRouteNorSetup(t *testing.T) {
	d := newTestDeck(t)
	routes := probedRoutes()
	for _, page := range []string{"/", "/?deck", "/office", "/office?o=wrong", "/setup", "/licenses"} {
		routes = append(routes, [2]string{"GET", page})
	}
	headersWith := func(extra map[string]string) map[string]string {
		all := map[string]string{}
		for name, value := range framedHeaders {
			all[name] = value
		}
		for name, value := range extra {
			all[name] = value
		}
		return all
	}
	// Each asked as the browser would, from this computer, the host kept; one let in by mistake to an event stream or a
	// long poll ends at once, as probeAnswerWith's do (its caller has already gone), so a regression fails, never hangs.
	ask := func(method, target, host string, headers map[string]string) *http.Response {
		return askHere(d, method, target, host, headers)
	}
	refused := func(code int) bool { return code == http.StatusUnauthorized || code == http.StatusForbidden }
	d.mu.Lock()
	cookie := setupCookie() + "=" + d.access.NewSetupSession()
	windowKey := d.access.WindowKey
	d.mu.Unlock()

	windowRows, setupProbes, pages := 0, 0, 0
	for _, route := range routes {
		method, target := route[0], route[1]
		path, _, _ := strings.Cut(target, "?")
		// A desk that isn't there, as the route dump asks (probeAnswer): a route let in, the window's own control
		// below included, finds nobody to type to, open or read for.
		if strings.Contains(target, "?") {
			target += "&session=nobody"
		} else {
			target += "?session=nobody"
		}
		row := phoneRouteFor(method, path)
		isSetup := path == "/setup" || strings.HasPrefix(path, "/setup/")
		for _, host := range []string{"localhost:7710", "127.0.0.1:7710"} {
			framed := ask(method, target, host, headersWith(nil))
			// The frame's headers let it in nowhere: it's answered as any program here with no key is (Setup's header only
			// turns the refusal for a missing header, 403, into the one for a missing session, 401).
			if bare := ask(method, target, host, nil); bare.StatusCode != framed.StatusCode && !(refused(bare.StatusCode) && refused(framed.StatusCode)) {
				t.Errorf("%s %s at %s: framed %d, bare %d", method, target, host, framed.StatusCode, bare.StatusCode)
			}
			if framed.Header.Get("Set-Cookie") != "" {
				t.Errorf("%s %s at %s: a framed page was given a cookie: %q", method, target, host, framed.Header.Get("Set-Cookie"))
			}
			// No page of the computer's can be put in the frame (a redirect's own few words are never shown: it's followed).
			if strings.HasPrefix(framed.Header.Get("Content-Type"), "text/html") && (framed.StatusCode < 300 || framed.StatusCode >= 400) {
				pages++
				if framed.Header.Get("X-Frame-Options") != "DENY" || !strings.Contains(framed.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
					t.Errorf("%s %s at %s: a page a frame could show: %q %q", method, target, host, framed.Header.Get("X-Frame-Options"), framed.Header.Get("Content-Security-Policy"))
				}
			}
			if row != nil && row.access == accessWindow {
				windowRows++
				if !refused(framed.StatusCode) {
					t.Errorf("%s %s at %s: a framed page let into a window route: %d", method, target, host, framed.StatusCode)
				}
				// The window, with its key, is let in: the frame is kept out for the key it can't have.
				if code := ask(method, target, host, map[string]string{hookKeyHeader: windowKey}).StatusCode; refused(code) {
					t.Errorf("%s %s at %s: the window itself refused (%d): the test proves nothing", method, target, host, code)
				}
			}
			if isSetup && path != "/setup" {
				setupProbes++
				for what, headers := range map[string]map[string]string{"framed": headersWith(nil), "framed, with the cookie": headersWith(map[string]string{"Cookie": cookie})} {
					// 127.0.0.1 with the cookie is Setup's own page asking. A framed page never is: it's at localhost, another site,
					// so the cookie isn't sent with its requests, even under an office at 127.0.0.1 (see the top of this file).
					if host == "127.0.0.1:7710" && what != "framed" {
						continue
					}
					res := framed
					if what != "framed" {
						res = ask(method, target, host, headers)
					}
					switch {
					case res.StatusCode >= 200 && res.StatusCode < 300:
						t.Errorf("%s %s at %s, %s: Setup answered %d", method, target, host, what, res.StatusCode)
					case res.StatusCode == http.StatusFound && (host != "localhost:7710" || !strings.HasPrefix(res.Header.Get("Location"), setupBase()+"/setup")):
						t.Errorf("%s %s at %s, %s: sent to %q", method, target, host, what, res.Header.Get("Location"))
					case res.Header.Get("Set-Cookie") != "":
						t.Errorf("%s %s at %s, %s: a cookie set", method, target, host, what)
					}
				}
			}
		}
	}
	want := 0
	for _, row := range phoneRouteTable {
		if row.access == accessWindow {
			want += 2
		}
	}
	if windowRows != want || want < 24 || setupProbes < 200 || pages < 4 {
		t.Errorf("probed %d window routes (want %d, at least 24), %d of Setup's, %d pages: not the tables this test knows", windowRows, want, setupProbes, pages)
	}
}

// askHere asks the router as a browser or a program on this computer would, the host kept, with the headers given. Its
// caller has already gone, so a stream or a long poll let in by mistake ends at once, as probeAnswerWith's do.
func askHere(d *Deck, method, target, host string, headers map[string]string) *http.Response {
	r := httptest.NewRequest(method, target, strings.NewReader(""))
	r.Host, r.RemoteAddr = host, thisComputer
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	ctx, cancel := context.WithCancel(r.Context())
	cancel()
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r.WithContext(ctx))
	return w.Result()
}

// setupPathsInTheSource is every path under /setup/ the router's code names: each string in this package's Go files
// (every platform's, its tests left out) that starts with /setup/, cut at its query, a prefix's / or * given a last
// part. By the source itself, not by a list: a route added tomorrow, in any file, is in it. The route table's rows are
// strings of that source too (routes.go).
func setupPathsInTheSource(t *testing.T) []string {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("this package's Go files: %v", err)
	}
	found := map[string]bool{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.HasPrefix(text, "/setup/") {
				return true
			}
			text, _, _ = strings.Cut(text, "?")
			text, _, _ = strings.Cut(text, "#")
			if strings.ContainsAny(text, " %\"'") {
				return true // a sentence or a format, not a path
			}
			text = strings.Replace(text, "*", "probe", 1)
			if strings.HasSuffix(text, "/") {
				text += "probe"
			}
			found[text] = true
			return true
		})
	}
	return slices.Sorted(maps.Keys(found))
}

// Whatever another page asks of Setup, a frame of it included, is refused before any of Setup's handlers, for every path
// under /setup/ the router's source names (STUDIO37: /setup/tap answered a framed page itself, 404 and 409, where it
// refused a bare one, 403). Each browser sign that says so is enough alone; Setup's own page, and an address typed in,
// still get in, and so does /setup#tap's ask for the owner's tap.
func TestEverySetupPathRefusesAnotherPageFirst(t *testing.T) {
	d := newTestDeck(t)
	paths := setupPathsInTheSource(t)
	// The walk sees what the table knows, and the route that slipped past.
	for _, row := range append(append([]routeRow{}, routeTable...), goOnlyRoutes...) {
		path := strings.Replace(row.path, "*", "probe", 1)
		if strings.HasPrefix(path, "/setup/") && !slices.Contains(paths, path) {
			t.Errorf("the walk missed the table's %s %s", row.method, row.path)
		}
	}
	if !slices.Contains(paths, "/setup/tap") || len(paths) < 100 {
		t.Fatalf("the walk found %d paths under /setup/, /setup/tap among them: %v; not the source this test knows", len(paths), slices.Contains(paths, "/setup/tap"))
	}
	with := func(headers map[string]string) map[string]string {
		all := map[string]string{setupHeader: "1"} // as if a preflight had let it: the core answers none
		for name, value := range headers {
			all[name] = value
		}
		return all
	}
	others := map[string]map[string]string{
		"a framed page":                          framedHeaders,
		"Sec-Fetch-Site cross-site alone":        with(map[string]string{"Sec-Fetch-Site": "cross-site"}),
		"Sec-Fetch-Site same-site alone":         with(map[string]string{"Sec-Fetch-Site": "same-site"}),
		"an Origin at localhost alone":           with(map[string]string{"Origin": "http://localhost:3000"}),
		"an Origin at another port of 127.0.0.1": with(map[string]string{"Origin": "http://127.0.0.1:3000"}),
		"a null Origin alone":                    with(map[string]string{"Origin": "null"}),
		"a Referer at localhost alone":           with(map[string]string{"Referer": "http://localhost:3000/app/"}),
		"a Referer that isn't an address":        with(map[string]string{"Referer": "http://%zz/"}),
	}
	guarded := func(res *http.Response) bool {
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode == http.StatusForbidden && string(body) == inMacWords(setupFromAnotherPage)
	}
	refused := func(code int) bool { return code == http.StatusUnauthorized || code == http.StatusForbidden }
	asked := 0
	for _, path := range append(paths, "/setup") {
		for _, method := range []string{"GET", "HEAD", "POST", "PUT", "DELETE"} {
			read := method == "GET" || method == "HEAD"
			if path == "/setup" && read {
				continue // the page itself: any page may open it, none can frame or read it (TestAFramedPreviewReachesNoWindowRouteNorSetup)
			}
			target := path + "?session=nobody"
			for _, host := range []string{"localhost:7710", "127.0.0.1:7710"} {
				bare := askHere(d, method, target, host, nil)
				for what, headers := range others {
					res := askHere(d, method, target, host, headers)
					asked++
					// At localhost a read is only sent on to 127.0.0.1, framed or not, where it's asked again.
					if read && host == "localhost:7710" {
						if res.StatusCode != http.StatusFound || bare.StatusCode != http.StatusFound || res.Header.Get("Location") != setupBase()+target {
							t.Errorf("%s %s at %s, %s: %d to %q, bare %d", method, target, host, what, res.StatusCode, res.Header.Get("Location"), bare.StatusCode)
						}
						continue
					}
					if !guarded(res) {
						t.Errorf("%s %s at %s, %s: %d, not refused before Setup's handlers (bare %d)", method, target, host, what, res.StatusCode, bare.StatusCode)
					}
					if !refused(bare.StatusCode) {
						t.Errorf("%s %s at %s, bare: %d", method, target, host, bare.StatusCode)
					}
				}
			}
		}
	}
	d.mu.Lock()
	cookie := setupCookie() + "=" + d.access.NewSetupSession()
	d.mu.Unlock()
	own := map[string]string{"Origin": "http://127.0.0.1:7710", "Referer": "http://127.0.0.1:7710/setup", "Sec-Fetch-Site": "same-origin",
		"Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty", setupHeader: "1"}
	// Even with Setup's cookie (a framed page never has it: see the top of this file), another page gets nothing.
	for _, target := range []string{"/setup/state", "/setup/tap?ticket=none"} {
		framed := map[string]string{"Cookie": cookie}
		for name, value := range framedHeaders {
			framed[name] = value
		}
		if res := askHere(d, "GET", target, "127.0.0.1:7710", framed); !guarded(res) {
			t.Errorf("GET %s, framed with Setup's cookie: %d", target, res.StatusCode)
		}
	}
	// Setup's own page and an address typed in are let in, the gate's other checks after.
	for what, headers := range map[string]map[string]string{"Setup's own page": own, "an address typed in": {"Sec-Fetch-Site": "none"},
		"a tool, with no browser signs": {}} {
		headers := maps.Clone(headers)
		headers["Cookie"] = cookie
		if res := askHere(d, "GET", "/setup/state", "127.0.0.1:7710", headers); res.StatusCode != http.StatusOK {
			t.Errorf("GET /setup/state, %s, with its cookie: %d", what, res.StatusCode)
		}
	}
	// /setup#tap, opened with no link: its ask reaches the tap's own handler (409 here: no phone is paired).
	if res := askHere(d, "POST", "/setup/tap", "127.0.0.1:7710", own); guarded(res) || res.StatusCode != http.StatusConflict {
		t.Errorf("POST /setup/tap from Setup's own page: %d", res.StatusCode)
	}
	t.Logf("walked %d paths under /setup/ (and /setup's own changes), %d asks from another page", len(paths), asked)
	if asked < 100*4*len(others) {
		t.Errorf("asked %d times: not the walk this test knows", asked)
	}
}

// The window's routes, from the router's own tables (the phone table's window rows, and the switch's routes its class
// gives the window), refuse a framed page as they refuse a bare one: it has no key, and Setup's header or a browser's
// signs give it none.
func TestEveryWindowRouteRefusesAFramedPage(t *testing.T) {
	d := newTestDeck(t)
	var routes [][2]string
	for key, row := range phoneRouteTable {
		if row.access == accessWindow {
			method, path, _ := strings.Cut(key, " ")
			routes = append(routes, [2]string{method, path})
		}
	}
	for path, class := range switchRoutes {
		if strings.HasPrefix(class, "window") || strings.HasPrefix(class, "phone or window") {
			routes = append(routes, [2]string{"GET", path}, [2]string{"POST", path})
		}
	}
	refused := func(code int) bool { return code == http.StatusUnauthorized || code == http.StatusForbidden }
	for _, route := range routes {
		method, target := route[0], strings.Replace(route[1], "*", "probe", 1)+"?session=nobody"
		for _, host := range []string{"localhost:7710", "127.0.0.1:7710"} {
			framed, bare := askHere(d, method, target, host, framedHeaders), askHere(d, method, target, host, nil)
			if !refused(framed.StatusCode) || !refused(bare.StatusCode) {
				t.Errorf("%s %s at %s: framed %d, bare %d", method, target, host, framed.StatusCode, bare.StatusCode)
			}
		}
	}
	t.Logf("walked %d window routes", len(routes))
	if len(routes) < 12+6+8 {
		t.Errorf("walked %d window routes: not the tables this test knows", len(routes))
	}
}
