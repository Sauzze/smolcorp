package desktop

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A caller of the phone's and the window's routes: where it asks from, to which name, with which key.
type routeCaller struct {
	name, host, from, local string
	key                     string // "phone", "window" or none
	proxied, mac, textOff   bool
	// Sends X-Forwarded-Proto: https alone, as a proxy would, or as a phone on the Wi-Fi would to look like one.
	spoofProto bool
}

// routeCallers is everyone who can knock on the routes past the hook: a stranger, a paired phone over the Wi-Fi,
// over Tailscale's WireGuard address and through `tailscale serve` (on a Mac and, since FR2, on Linux and Windows),
// someone sending a proxy's headers or a ts.net name from where only `tailscale serve` on this computer may,
// the office's own window, and the window's key where it isn't the window's.
var routeCallers = []routeCaller{
	{name: "stranger", host: deckOnWiFi, from: aPhone},
	{name: "phone-wifi", host: deckOnWiFi, from: aPhone, key: "phone"},
	{name: "phone-wireguard", host: "100.64.1.2:7710", from: "100.101.102.103:50000", local: "100.64.1.2:7710", key: "phone"},
	{name: "phone-serve-mac", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, mac: true},
	{name: "phone-serve-mac-text-off", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, mac: true, textOff: true},
	{name: "phone-serve-linux", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true},
	{name: "phone-serve-linux-text-off", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, textOff: true},
	{name: "phone-serve-linux-proto", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", spoofProto: true},
	{name: "spoof-proto-wifi", host: deckOnWiFi, from: aPhone, key: "phone", spoofProto: true},
	{name: "spoof-tsnet-wifi", host: "my-mac.tail1234.ts.net", from: aPhone, key: "phone", proxied: true, spoofProto: true},
	{name: "spoof-tsnet-wireguard", host: "my-mac.tail1234.ts.net", from: "100.101.102.103:50000", local: "100.64.1.2:7710", key: "phone", proxied: true, spoofProto: true},
	{name: "phone-proxied-localhost-other", host: "localhost:7710", from: thisComputer, key: "phone", proxied: true},
	{name: "phone-here", host: "localhost:7710", from: thisComputer, key: "phone"},
	{name: "window", host: "localhost:7710", from: thisComputer, key: "window"},
	{name: "window-proxied", host: "localhost:7710", from: thisComputer, key: "window", proxied: true},
	{name: "window-key-on-wifi", host: deckOnWiFi, from: aPhone, key: "window"},
}

// everyPhoneRoute is every route past setup and the hook (server.go's handle), each with a desk that isn't there,
// so nothing is typed, read or opened once a caller is let in. Wrong methods and an unknown path are in too.
var everyPhoneRoute = [][2]string{
	{"GET", "/icon.png"}, {"GET", "/favicon-16.png"}, {"GET", "/favicon-32.png"},
	{"POST", "/pair"}, {"GET", "/office.js"}, {"GET", "/guide.js"}, {"GET", "/licenses"}, {"GET", "/sounds/keys.m4a"},
	{"GET", "/office?o=wrong"}, {"GET", "/"}, {"GET", "/office"}, {"GET", "/events"}, {"POST", "/focus"},
	{"POST", "/act?action=settings"}, {"GET", "/hire/window"}, {"POST", "/hire/window"},
	{"GET", "/t3code"}, {"POST", "/t3code/open"}, {"POST", "/act?action=takeover"}, {"POST", "/act?action=handback"},
	{"POST", "/act?action=letgo"}, {"POST", "/act?action=stop"},
	{"POST", "/dictate"}, {"POST", "/peek"}, {"POST", "/choose?option=other"}, {"GET", "/choice"}, {"GET", "/approvals"},
	{"GET", "/conversation"}, {"GET", "/session/info"}, {"GET", "/desk/git"}, {"POST", "/summary"},
	{"GET", "/talk"}, {"GET", "/talk/asking"}, {"POST", "/talk/say"}, {"POST", "/talk/pick"},
	{"GET", "/dictate"}, {"GET", "/peek"}, {"POST", "/choice"}, {"POST", "/approvals"}, {"POST", "/conversation"},
	{"POST", "/talk"}, {"PUT", "/talk"}, {"POST", "/talk/asking"}, {"GET", "/talk/say"}, {"GET", "/talk/pick"},
	{"GET", "/nowhere"},
}

// probeRoute asks one route as one caller, through the whole router, and says the status and the text.
func probeRoute(t *testing.T, d *Deck, method, target, phoneKey string, c routeCaller) (int, string) {
	t.Helper()
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	r := httptest.NewRequest(method, target+sep+"session=nobody", strings.NewReader("{}"))
	r.Host, r.RemoteAddr = c.host, c.from
	// An event stream ends at once: its caller has already gone.
	ctx, cancel := context.WithCancel(r.Context())
	cancel()
	if c.local != "" {
		ip, _, _ := net.SplitHostPort(c.local)
		ctx = context.WithValue(ctx, http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP(ip), Port: 7710})
	}
	r = r.WithContext(ctx)
	d.mu.Lock()
	switch c.key {
	case "phone":
		r.Header.Set(hookKeyHeader, phoneKey)
	case "window":
		r.Header.Set(hookKeyHeader, d.access.WindowKey)
	}
	d.config.PhoneMicEnabled = !c.textOff
	d.mu.Unlock()
	if c.proxied {
		r.Header.Set("X-Forwarded-For", "100.101.102.103")
	}
	if c.spoofProto {
		r.Header.Set("X-Forwarded-Proto", "https")
	}
	macTailscale = c.mac
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w.Code, strings.TrimSpace(w.Body.String())
}

// probeDeck is a deck with a paired phone, where nothing reaches a terminal or a browser, and Tailscale's address is
// 100.64.1.2; and the phone's key.
func probeDeck(t *testing.T) (*Deck, string) {
	t.Helper()
	savedHost, savedMac, savedAddresses, savedOpen := host, macTailscale, tailscaleAddresses, openSetupPage
	t.Cleanup(func() {
		host, macTailscale, tailscaleAddresses, openSetupPage = savedHost, savedMac, savedAddresses, savedOpen
	})
	host = nil
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	openSetupPage = func(*Deck) error { return nil }
	d := newTestDeck(t)
	d.mu.Lock()
	defer d.mu.Unlock()
	return d, d.access.Phones.Pair("Probe")
}

// letIn is whether an answer got past every gate: neither 401 nor 403.
func letIn(status int) bool {
	return status != http.StatusUnauthorized && status != http.StatusForbidden
}

// dumpPhoneRoutes is every route with each caller's answer, and the callers it lets in.
func dumpPhoneRoutes(t *testing.T) string {
	t.Helper()
	d, phoneKey := probeDeck(t)
	var dump strings.Builder
	for _, route := range everyPhoneRoute {
		var answers, in []string
		for _, c := range routeCallers {
			status, _ := probeRoute(t, d, route[0], route[1], phoneKey, c)
			answers = append(answers, fmt.Sprint(status))
			if letIn(status) {
				in = append(in, c.name)
			}
		}
		letIn := strings.Join(in, ", ")
		if letIn == "" {
			letIn = "nobody"
		}
		fmt.Fprintf(&dump, "%s %s: %s; let in: %s\n", route[0], route[1], strings.Join(answers, " "), letIn)
	}
	return dump.String()
}

// TestEveryRouteKeepsItsAccessThroughTheMove dumps every phone and window route with who it lets in, and holds it to
// the dump taken before FR1 moved routes into the table (phoneRouteAccess, from release/1.26.0 at 4209ca71), with FR2's
// one change: a `tailscale serve` hand-on is let in on every system, so Linux and Windows' column is the Mac's (checked
// column by column against the old dump: no other caller's answer moved). A new route adds its line there, as the talk bubble's did (1.26: the window only; a phone 403, anyone else 401).
func TestEveryRouteKeepsItsAccessThroughTheMove(t *testing.T) {
	after := dumpPhoneRoutes(t)
	if after != phoneRouteAccess {
		t.Fatalf("a route's access changed in the move.\nbefore:\n%s\nafter:\n%s", phoneRouteAccess, after)
	}
	t.Logf("the same access before and after the move, for %d routes and %d callers:\n%s", len(everyPhoneRoute), len(routeCallers), after)
}

// The callers each class lets in, of routeCallers: those the router's own checks let reach the table, less those
// the class keeps out. Through `tailscale serve` is Tailscale's on every system, a Mac or not, as fromTailnet reads it;
// the same headers or ts.net name from the Wi-Fi, or from Tailscale's own address to a name, are no one's but the
// plain Wi-Fi phone's (spoof-proto-wifi, which is a phone, and not secure) or refused before the table.
var classLetsIn = map[access][]string{
	accessWindow:      {"window"},
	accessPhone:       {"phone-wifi", "phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-serve-linux-proto", "spoof-proto-wifi", "phone-proxied-localhost-other", "phone-here"},
	accessPhoneSecure: {"phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-serve-linux-proto", "phone-proxied-localhost-other"},
	accessPeer:        nil,
}

// reachTheTable are the callers the router's own checks let reach the table: the rest are refused before it.
var reachTheTable = []string{"phone-wifi", "phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-serve-linux-proto", "spoof-proto-wifi", "phone-proxied-localhost-other", "phone-here", "window"}

// Each row of the table lets in exactly the callers its class names, tells the others its own refusal, and is in the
// dump of every route.
func TestEachRowLetsInItsClassOnly(t *testing.T) {
	d, phoneKey := probeDeck(t)
	// FR1 moved 7; the talk bubble adds GET /talk, GET /talk/asking, POST /talk/say and POST /talk/pick (1.26).
	if len(phoneRouteTable) != 12 {
		t.Errorf("%d rows; want 12", len(phoneRouteTable))
	}
	for key, row := range phoneRouteTable {
		if !slices.ContainsFunc(everyPhoneRoute, func(route [2]string) bool {
			path, _, _ := strings.Cut(route[1], "?")
			return route[0] == row.method && path == row.path
		}) {
			t.Errorf("%s isn't in everyPhoneRoute, so its access isn't pinned", key)
		}
		var in []string
		for _, c := range routeCallers {
			status, text := probeRoute(t, d, row.method, row.path, phoneKey, c)
			if letIn(status) {
				in = append(in, c.name)
			} else if slices.Contains(reachTheTable, c.name) && (status != http.StatusForbidden || text != row.refusal(d)) {
				t.Errorf("%s, %s: %d %q, want 403 %q", key, c.name, status, text, row.refusal(d))
			}
		}
		if !slices.Equal(in, classLetsIn[row.access]) {
			t.Errorf("%s lets in %v, its class %v", key, in, classLetsIn[row.access])
		}
		if most, wait := requestLimits(row.method, row.path); most != maxRequestSize || wait != requestTimeout {
			t.Errorf("%s: %d bytes in %s, as before the move %d in %s", key, most, wait, maxRequestSize, requestTimeout)
		}
	}
	// Another Mac's class lets no one in until a piece gives it its checks.
	if accessPeer.lets(&request{Request: httptest.NewRequest("GET", "/", nil)}, "phone") {
		t.Error("the peer class let a phone in")
	}
}

// A row's own body size and time are the router's for that route only; a route twice stops the start.
func TestARowsLimitsAreItsOwn(t *testing.T) {
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/test-photo", access: accessPhone, refusal: says("no"),
		maxBody: 8, timeout: 30 * time.Second, serve: func(_ *Deck, w http.ResponseWriter, r *request, _ string) {
			if _, err := io.ReadAll(r.Body); err != nil {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}})
	t.Cleanup(func() { delete(phoneRouteTable, "POST /test-photo") })
	if most, wait := requestLimits(http.MethodPost, "/test-photo"); most != 8 || wait != 30*time.Second {
		t.Errorf("%d bytes in %s", most, wait)
	}
	if most, wait := requestLimits(http.MethodGet, "/test-photo"); most != maxRequestSize || wait != requestTimeout {
		t.Errorf("another method: %d bytes in %s", most, wait)
	}
	d, phoneKey := probeDeck(t)
	for body, want := range map[string]int{"12345678": http.StatusNoContent, "123456789": http.StatusRequestEntityTooLarge} {
		r := httptest.NewRequest(http.MethodPost, "/test-photo", strings.NewReader(body))
		r.Host, r.RemoteAddr = deckOnWiFi, aPhone
		r.Header.Set(hookKeyHeader, phoneKey)
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("%d bytes: %d, want %d", len(body), w.Code, want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("a route registered twice was taken")
		}
	}()
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/test-photo", access: accessPhone, refusal: says("no"),
		serve: func(*Deck, http.ResponseWriter, *request, string) {}})
}

// Callers, in each line's order: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off,
// phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, spoof-proto-wifi, spoof-tsnet-wifi,
// spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi.
const phoneRouteAccess = `GET /icon.png: 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off, phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, spoof-proto-wifi, spoof-tsnet-wifi, spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /favicon-16.png: 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off, phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, spoof-proto-wifi, spoof-tsnet-wifi, spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /favicon-32.png: 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off, phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, spoof-proto-wifi, spoof-tsnet-wifi, spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
POST /pair: 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403; let in: nobody
GET /office.js: 200 200 200 200 403 200 403 200 200 403 403 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /guide.js: 200 200 200 200 403 200 403 200 200 403 403 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /licenses: 404 404 404 404 403 404 403 404 404 403 403 404 404 404 404 404; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /sounds/keys.m4a: 200 200 200 200 403 200 403 200 200 403 403 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /office?o=wrong: 200 200 200 200 403 200 403 200 200 403 403 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /: 200 200 200 200 403 200 403 200 200 403 403 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /office: 200 200 200 200 403 200 403 200 200 403 403 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi
GET /events: 401 200 200 200 403 200 403 200 200 403 403 200 200 200 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /focus: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /act?action=settings: 401 403 403 403 403 403 403 403 403 403 403 403 403 204 401 401; let in: window
GET /hire/window: 401 403 403 403 403 403 403 403 403 403 403 403 403 200 401 401; let in: window
POST /hire/window: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
GET /t3code: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
POST /t3code/open: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
POST /act?action=takeover: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
POST /act?action=handback: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
POST /act?action=letgo: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /act?action=stop: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /dictate: 401 400 400 400 403 400 403 400 400 403 403 400 400 403 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here
POST /peek: 401 403 404 404 403 404 403 404 403 403 403 404 403 403 401 401; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, phone-proxied-localhost-other
POST /choose?option=other: 401 400 400 400 403 400 403 400 400 403 403 400 400 403 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here
GET /choice: 401 403 404 404 403 404 403 404 403 403 403 404 403 403 401 401; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, phone-proxied-localhost-other
GET /approvals: 401 403 200 200 403 200 403 200 403 403 403 200 403 403 401 401; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, phone-proxied-localhost-other
GET /conversation: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
GET /session/info: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
GET /desk/git: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
POST /summary: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 401 401; let in: nobody
GET /talk: 401 403 403 403 403 403 403 403 403 403 403 403 403 404 401 401; let in: window
GET /talk/asking: 401 403 403 403 403 403 403 403 403 403 403 403 403 200 401 401; let in: window
POST /talk/say: 401 403 403 403 403 403 403 403 403 403 403 403 403 400 401 401; let in: window
POST /talk/pick: 401 403 403 403 403 403 403 403 403 403 403 403 403 400 401 401; let in: window
GET /dictate: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
GET /peek: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /choice: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /approvals: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /conversation: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /talk: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
PUT /talk: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
POST /talk/asking: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
GET /talk/say: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
GET /talk/pick: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
GET /nowhere: 401 404 404 404 403 404 403 404 404 403 403 404 404 404 401 401; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-linux-proto, spoof-proto-wifi, phone-proxied-localhost-other, phone-here, window
`

// MARK: The switch's route set (FR1's review, N1)

// handlesRoutes is every route path server.go's handle names: each string starting with "/" in its body, and each
// *Path constant it compares with (screenAnswerPath, raisePath), and the sounds' list, by the source itself, not by
// what the table knows.
func handlesRoutes(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "handle" || fn.Recv == nil {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.BasicLit:
				if text, err := strconv.Unquote(n.Value); n.Kind == token.STRING && err == nil && strings.HasPrefix(text, "/") {
					found[text] = true
				}
			case *ast.Ident:
				if n.Name != "Path" && strings.HasSuffix(n.Name, "Path") || n.Name == "sounds" {
					found["ident "+n.Name] = true
				}
			}
			return true
		})
	}
	if len(found) == 0 {
		t.Fatal("server.go has no handle")
	}
	return slices.Sorted(maps.Keys(found))
}

// switchRoutes is the route set of handle's switch, frozen, each with the class it answers by: who may use it,
// and where that's decided. A route not in the table (registerPhoneRoute) is no one's by default, so a new route
// goes in the table, with its class and its refusal, which panics at start without them. It lands here only for
// what can't be a table row (setup, the hook, a page with no key), or the window's own routes still checked by hand
// (N2), and only with a class: that is what this test is for.
var switchRoutes = map[string]string{
	// No key at all: a file or a page that holds nothing of yours.
	"/icon.png": "open", "/favicon-16.png": "open", "/favicon-32.png": "open", "/office.js": "open", "/guide.js": "open",
	"/licenses": "open", "ident sounds": "open: the office's recordings",
	// The pairing page and the deck page: with no key the deck gives pair.html, and a code is how a phone gets a key.
	"/": "open: pair.html until paired", "/office": "open: pair.html until paired, the window's token here", "/pair": "open: a pairing code",
	// This computer only, with a key of its own: setup (the table routes.go says which caller), the hook, a launcher.
	"/setup": "local + setup caller", "/setup/": "local + setup caller", "/setup/session": "local + setup caller", "/setup/state": "local + setup caller",
	"/bots/": "local + setup caller", "/hook": "local + hook key", "/launcher": "local + hook key", "ident raisePath": "local + hook key",
	"ident hireStartPath": "local + hook key", "ident screenAnswerPath": "local + hook key + the launcher's secret",
	// Not a route: the tray's token swapped for the window's key, a redirect to /office.
	"/office?t=": "redirect to /office",
	// A paired phone's key or the window's, past the router's own checks (handle's phoneID): the route says the rest by hand.
	"/events": "phone or window", "/focus": "phone or window", "/act": "phone or window; settings, takeover, handback and letgo: window",
	"/summary": "phone or window",
	// The window's own routes, checked by hand in the switch (N2): the window only.
	"/hire/window": "window, by hand", "/t3code": "window, by hand", "/t3code/open": "window, by hand",
}

// A route in handle's switch that isn't pinned here, with its class, fails: adding one with no class is impossible
// without a line here that says which. And a line here for a route that has gone (to the table) fails too.
func TestTheSwitchHasNoRouteWithoutAClass(t *testing.T) {
	got := handlesRoutes(t)
	for _, path := range got {
		if class, ok := switchRoutes[path]; !ok || class == "" {
			t.Errorf("handle's switch takes %s with no class: put it in the route table (registerPhoneRoute, with its access class and refusal), or, if it can't be a row, pin its class in switchRoutes", path)
		}
	}
	for path := range switchRoutes {
		if !slices.Contains(got, path) {
			t.Errorf("%s is pinned in switchRoutes but handle no longer takes it: take it out", path)
		}
	}
	// A route in the table is not also taken by the switch: the table answers first, and a second answer would never be reached.
	for key := range phoneRouteTable {
		_, path, _ := strings.Cut(key, " ")
		if _, there := switchRoutes[path]; there {
			t.Errorf("%s is in the table and in switchRoutes", key)
		}
	}
}
