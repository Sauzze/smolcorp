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
	"net/url"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A caller of the phone's and the window's routes: where it asks from, to which name, with which key.
type routeCaller struct {
	name, host, from, local string
	key                     string // "phone", "window", "browser", "window-device" or none
	// signs: the phone's key signs the request, never sent, as a phone on the Wi-Fi asks since 1.25 (sign.js).
	proxied, mac, textOff, signs bool
	// Sends X-Forwarded-Proto: https alone, as a proxy would, or as a phone on the Wi-Fi would to look like one.
	spoofProto bool
	// macApp: the deck runs under the Mac app (d.mac.host), as only a Mac's does.
	macApp bool
	// linux: the deck runs on Linux, where `tailscale serve`'s hand-on is let in too since OLD-SERVE1 (linuxTailscale).
	// Neither mac nor linux: Windows, where it is let in too since OLD-SERVE2 (windowsTailscale), through the serve
	// port. off: none of the three, a system where the hand-on isn't let in (none Smolcorp ships for), as Windows was
	// before OLD-SERVE2.
	linux, off bool
	// tailscale: the caller came through serve.sock, its other end identified as Tailscale's own process (a stand-in
	// peer, SN1): the real `tailscale serve`. Every other caller from the loopback is a program here.
	tailscale bool
}

// routeCallers is everyone who can knock on the routes past the hook: a stranger, a paired phone over the Wi-Fi
// (signing, with Typing over Wi-Fi on: probeDeck), over Tailscale's WireGuard address and through `tailscale serve`
// (on a Mac, on Linux and on Windows, OLD-SERVE2, each with its connection's other end Tailscale's own process, a
// stand-in peer), someone sending a proxy's headers or a ts.net name from where only
// `tailscale serve` on this computer may (from the Wi-Fi, signing as a paired phone there must), the office's own
// window, the window's key where it isn't the window's, and the paired device the Windows window holds (its key in a
// file here), from this computer as itself and with a proxy's header made up (RT126FS2 B2).
var routeCallers = []routeCaller{
	{name: "stranger", host: deckOnWiFi, from: aPhone},
	{name: "phone-wifi", host: deckOnWiFi, from: aPhone, key: "phone", signs: true},
	{name: "phone-wireguard", host: "100.64.1.2:7710", from: "100.101.102.103:50000", local: "100.64.1.2:7710", key: "phone"},
	{name: "phone-serve-mac", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, mac: true, tailscale: true},
	{name: "phone-serve-mac-text-off", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, mac: true, textOff: true, tailscale: true},
	{name: "phone-serve-linux", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, linux: true, tailscale: true},
	{name: "phone-serve-linux-text-off", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, linux: true, textOff: true, tailscale: true},
	{name: "phone-serve-linux-proto", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", spoofProto: true, linux: true, tailscale: true},
	{name: "phone-serve-windows", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, tailscale: true},
	{name: "phone-serve-windows-text-off", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", proxied: true, textOff: true, tailscale: true},
	{name: "phone-serve-windows-proto", host: "my-mac.tail1234.ts.net", from: thisComputer, key: "phone", spoofProto: true, tailscale: true},
	{name: "spoof-proto-wifi", host: deckOnWiFi, from: aPhone, key: "phone", signs: true, spoofProto: true},
	{name: "spoof-tsnet-wifi", host: "my-mac.tail1234.ts.net", from: aPhone, key: "phone", proxied: true, signs: true, spoofProto: true},
	{name: "spoof-tsnet-wireguard", host: "my-mac.tail1234.ts.net", from: "100.101.102.103:50000", local: "100.64.1.2:7710", key: "phone", proxied: true, spoofProto: true},
	{name: "phone-proxied-localhost-other", host: "localhost:7710", from: thisComputer, key: "phone", proxied: true},
	{name: "phone-here", host: "localhost:7710", from: thisComputer, key: "phone"},
	{name: "window", host: "localhost:7710", from: thisComputer, key: "window"},
	{name: "window-proxied", host: "localhost:7710", from: thisComputer, key: "window", proxied: true},
	{name: "window-key-on-wifi", host: deckOnWiFi, from: aPhone, key: "window"},
	{name: "window-device", host: "localhost:7710", from: thisComputer, key: "window-device"},
	{name: "window-device-proxied", host: "localhost:7710", from: thisComputer, key: "window-device", proxied: true},
}

// everyPhoneRoute is every route past setup and the hook (server.go's handle), each with a desk that isn't there,
// so nothing is typed, read or opened once a caller is let in. Wrong methods and an unknown path are in too.
var everyPhoneRoute = [][2]string{
	{"GET", "/icon.png"}, {"GET", "/favicon-16.png"}, {"GET", "/favicon-32.png"},
	{"POST", "/pair"}, {"GET", "/office.js"}, {"GET", "/guide.js"}, {"GET", "/licenses"}, {"GET", "/sounds/keys.m4a"},
	{"GET", "/office?o=wrong"}, {"GET", "/"}, {"GET", "/office"}, {"GET", "/paired"}, {"GET", "/events"}, {"POST", "/focus"},
	{"POST", "/act?action=settings"}, {"GET", "/hire/window"}, {"POST", "/hire/window"},
	{"GET", "/t3code"}, {"POST", "/t3code/open"}, {"POST", "/act?action=takeover"}, {"POST", "/act?action=handback"},
	{"POST", "/act?action=letgo"}, {"POST", "/act?action=stop"},
	{"POST", "/dictate"}, {"POST", "/peek"}, {"POST", "/choose?option=other"}, {"GET", "/choice"}, {"GET", "/approvals"},
	{"GET", "/conversation"}, {"GET", "/session/info"}, {"GET", "/desk/git"}, {"GET", "/desk/preview"}, {"POST", "/desk/preview"}, {"POST", "/summary"},
	{"POST", "/desk/pr/open"}, {"GET", "/talk"}, {"GET", "/talk/asking"}, {"POST", "/talk/say"}, {"POST", "/talk/pick"}, {"POST", "/talk/listen"},
	{"POST", "/photo"}, {"POST", "/talk/photo"},
	{"GET", "/dictate"}, {"GET", "/peek"}, {"POST", "/choice"}, {"POST", "/approvals"}, {"POST", "/conversation"},
	{"GET", "/desk/pr/open"}, {"POST", "/talk"}, {"PUT", "/talk"}, {"POST", "/talk/asking"}, {"GET", "/talk/say"}, {"GET", "/talk/pick"}, {"GET", "/talk/listen"},
	{"GET", "/photo"}, {"GET", "/talk/photo"},
	{"GET", "/peer/hello"}, {"POST", "/peer/pair"}, {"GET", "/peer/nowhere"},
	{"GET", "/nowhere"},
	// The owner's asks and their answer (grants.go, FS2).
	{"GET", "/grants"}, {"POST", "/grants/answer"}, {"PUT", "/grants/answer"},
	// A phone's notifications (pushsubs.go, PUSH1), and their wrong methods.
	{"GET", "/push"}, {"POST", "/push"}, {"POST", "/push/off"}, {"PUT", "/push"}, {"GET", "/push/off"},
}

// probeRoute asks one route as one caller, through the whole router, and says the status and the text.
func probeRoute(t *testing.T, d *Deck, method, target, phoneKey string, c routeCaller) (int, string) {
	t.Helper()
	w := probeAnswer(t, d, method, target, phoneKey, c)
	return w.Code, strings.TrimSpace(w.Body.String())
}

// probeAnswer is probeRoute's whole answer, its headers too.
func probeAnswer(t *testing.T, d *Deck, method, target, phoneKey string, c routeCaller) *httptest.ResponseRecorder {
	t.Helper()
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	return probeAnswerWith(t, d, method, target+sep+"session=nobody", "{}", phoneKey, c)
}

// probeRouteWith is probeRoute with the target's own query and a body of its own.
func probeRouteWith(t *testing.T, d *Deck, method, target, body, phoneKey string, c routeCaller) (int, string) {
	t.Helper()
	w := probeAnswerWith(t, d, method, target, body, phoneKey, c)
	return w.Code, strings.TrimSpace(w.Body.String())
}

// probeAnswerWith is probeAnswer with the target's own query and a body of its own: what probeAnswer and
// probeRouteWith both ask through.
func probeAnswerWith(t *testing.T, d *Deck, method, target, body, phoneKey string, c routeCaller) *httptest.ResponseRecorder {
	t.Helper()
	if c.signs {
		// Signed as sign.js signs it: the key never sent, and a GET with no body (a signed GET with one is refused).
		if method == http.MethodGet {
			body = ""
		}
		target = signedTarget(method, target+"&t="+url.QueryEscape(phoneKey), body, time.Now())
	}
	r := httptest.NewRequest(method, target, strings.NewReader(body))
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
	switch {
	case c.signs:
	case c.key == "phone":
		r.Header.Set(hookKeyHeader, phoneKey)
	case c.key == "window":
		r.Header.Set(hookKeyHeader, d.access.WindowKey)
	case c.key == "browser":
		r.Header.Set(hookKeyHeader, d.access.BrowserKey)
	case c.key == "window-device":
		r.Header.Set(hookKeyHeader, ensureWindowKey(d.access.Phones, &d.access.windowDeviceKey))
	}
	d.config.PhoneMicEnabled = !c.textOff
	if c.macApp {
		savedHost := d.mac.host
		d.mac.host = macAppHost{}
		defer func() {
			d.mu.Lock()
			d.mac.host = savedHost
			d.mu.Unlock()
		}()
	}
	d.mu.Unlock()
	if c.proxied {
		r.Header.Set("X-Forwarded-For", "100.101.102.103")
	}
	if c.spoofProto {
		r.Header.Set("X-Forwarded-Proto", "https")
	}
	macTailscale, linuxTailscale, windowsTailscale = c.mac, c.linux, !c.mac && !c.linux && !c.off
	servedUnrecorded = func(*http.Request) bool { return c.tailscale }
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w
}

// probeDeck is a deck with a paired phone, where nothing reaches a terminal or a browser, and Tailscale's address is
// 100.64.1.2; and the phone's key. Typing over Wi-Fi is on, so a phone on the Wi-Fi reaches what its class lets it
// (SECWIFI's switch, off by default, is signed_test.go's).
func probeDeck(t *testing.T) (*Deck, string) {
	t.Helper()
	savedHost, savedMac, savedLinux, savedWindows, savedAddresses, savedOpen, savedPeer := host, macTailscale, linuxTailscale, windowsTailscale, tailscaleAddresses, openSetupPage, servedUnrecorded
	t.Cleanup(func() {
		host, macTailscale, linuxTailscale, windowsTailscale, tailscaleAddresses, openSetupPage, servedUnrecorded = savedHost, savedMac, savedLinux, savedWindows, savedAddresses, savedOpen, savedPeer
	})
	host = nil
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	openSetupPage = func(*Deck) error { return nil }
	d := newTestDeck(t)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.config.PhoneTextWiFi = true
	return d, d.access.Phones.Pair("iPhone", true)
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
// the dump taken before FR1 moved routes into the table (phoneRouteAccess, from release/1.26.0 at 4209ca71). FR2's
// callers are in it, but not its opening: a `tailscale serve` hand-on on Linux and Windows was refused until SN1, so
// their three columns were 1.25's phone-serve-other, cell for cell (MSY126e; TestTheServeHandOnStaysTheMacsUntilSN1),
// and no other caller's answer moved against 1.25's dump. Since OLD-SERVE1 they are Linux's (below). A new route adds its line there, as the talk
// bubble's did (1.26: the window only; a phone 403, anyone else 401), and 1.25's GET /paired (any paired phone or the
// window, as /events; still handle's switch, as no one class is both). M1's three /peer/ lines are servePeer's, asked
// before anything else and alike for every caller here, none of which has a peer key: 403 on GET /peer/hello (no peer
// key) and POST /peer/pair (floors off), 404 on a /peer/ path that isn't a route. FS2 adds its three /grants lines (the
// owner's tap: a paired phone over WireGuard or through `tailscale serve` on a Mac, and no one else) and two callers,
// the paired device the Windows window holds, as itself and with a made-up proxy header; every other cell is as it was.
// PH1 adds its photo: POST /photo over Tailscale only (the owner's phone, as /peek; the Wi-Fi and the window 403),
// POST /talk/photo the window only, and their GETs, which aren't routes. SN1 moves two columns, and only to refusals:
// phone-proxied-localhost-other and window-device-proxied (a program here with a proxy's header made up, off a Mac)
// were taken as over Tailscale; no Tailscale process is at their other end, so their key isn't taken, and every
// route but the open icons and /peer/ answers them 401 (had 200, 204, 400, 403 or 404). The hand-on through the real
// `tailscale serve` (phone-serve-mac, a stand-in peer) keeps every cell; phone-serve-linux-proto and every spoof
// caller stay refused. OLD-SERVE1 moves one column, phone-serve-linux, and only to what phone-serve-mac answers: the
// hand-on is let in on Linux too, 36 cells from 403 to the Mac's 200, 204, 400 or 404; phone-serve-linux-text-off
// (403 everywhere but the open icons, as phone-serve-mac-text-off) and phone-serve-linux-proto don't move.
// PUSH1 adds five lines at the end, and moves no cell: a phone's notifications over Tailscale only (GET /push, POST
// /push and POST /push/off, as /approvals; the Wi-Fi and the window 403), and their wrong methods; phone-serve-linux
// answers them as phone-serve-mac, as it does every route since OLD-SERVE1.
// PUSH1's five /push lines (GET, POST, POST /push/off; PUT and GET /push/off 404): phone-serve-linux answers as
// phone-serve-mac there. PUSH1 was reviewed before OLD-SERVE1, with 403 in that column; SIERRA (GM) ruled 14 Nov
// that Linux serve, made the Mac's equal by OLD-SERVE1, is let in as the Mac's is (F3: HTTPS, typing on), on two
// conditions the merge review RMSYPUSH1 checked: no other cell moved, and Linux serve reaches /push only through
// SN1's verified path (serve.sock, the peer checked, the peer token), never a TCP 127.0.0.1 guess (that gets 403).
// OLD-SERVE2 adds three columns, phone-serve-windows, -text-off and -proto, each answering as its Mac (or Linux)
// twin, and lets the hand-on in on Windows, which every caller with no system named runs as. That moves three
// columns, and only to refusals: phone-proxied-localhost-other, window-proxied and window-device-proxied (a program
// here with a proxy's header made up on the loopback) are now refused at the door, 403, as on a Mac, on every route
// but the open icons and /peer/ (56 cells each before PUSH1's lines, from 401, or 200 and 404 on the seven open
// pages, which no longer let them in). No other cell moves.
// On PUSH1's five /push lines (MSYOS2), phone-serve-windows-text-off and -proto answer as their twins (403), and
// OLD-SERVE2's three refused columns get 403 there too (15 cells, from 401). phone-serve-windows does NOT answer as
// phone-serve-mac there: SIERRA (GM) ruled (b) for 1.26 that Windows serve stays refused on all five lines, 403 (GET,
// POST and POST /push/off, and the two wrong methods, from 503, 400, 204, 404 and 404). Why: on Windows the
// signed-in user, and so a hire, can change `tailscale serve`, and nothing in 1.26 keeps a hire from the tailscale
// CLI until design A's Windows pieces (SA-W2/W3) land; push wakes the owner's phone, and the owner chose Strict. The
// code refuses it at the table's gate, by request class (request.windowsServe, pushRefusal), on any method under
// /push; Mac and Linux serve, and WireGuard on Windows, are let in as before. No other cell moves. Reopen as (a), with
// its own review, once SA-W2/W3 deny hires the tailscale CLI and serve settings on Windows.
func TestEveryRouteKeepsItsAccessThroughTheMove(t *testing.T) {
	after := dumpPhoneRoutes(t)
	if after != phoneRouteAccess {
		t.Fatalf("a route's access changed in the move.\nbefore:\n%s\nafter:\n%s", phoneRouteAccess, after)
	}
	t.Logf("the same access before and after the move, for %d routes and %d callers:\n%s", len(everyPhoneRoute), len(routeCallers), after)
}

// The callers each class lets in, of routeCallers: those the router's own checks let reach the table, less those
// the class keeps out. Through the real `tailscale serve` is Tailscale's on a Mac, on Linux (OLD-SERVE1) and on
// Windows (OLD-SERVE2). The same headers or ts.net name from a program
// here (SN1), the Wi-Fi, or from Tailscale's own address to a name, are no one's but the plain Wi-Fi phone's
// (spoof-proto-wifi, which is a phone, and not secure) or refused before the table.
var classLetsIn = map[access][]string{
	accessWindow:      {"window"},
	accessPhone:       {"phone-wifi", "phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-serve-windows", "spoof-proto-wifi", "phone-here", "window-device"},
	accessPhoneSecure: {"phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-serve-windows"},
	accessOwnerTap:    {"phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-serve-windows"},
	accessPeer:        nil,
}

// reachTheTable are the callers the router's own checks let reach the table: the rest are refused before it.
var reachTheTable = []string{"phone-wifi", "phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-serve-windows", "spoof-proto-wifi", "phone-here", "window", "window-device"}

// Each row of the table lets in exactly the callers its class names, tells the others its own refusal, and is in the
// dump of every route.
func TestEachRowLetsInItsClassOnly(t *testing.T) {
	d, phoneKey := probeDeck(t)
	// FR1 moved 7; the talk bubble adds GET /talk, GET /talk/asking, POST /talk/say and POST /talk/pick (1.26), and
	// the dictation service (FD1) POST /talk/listen, and the pull request badge's click POST /desk/pr/open (R2);
	// floors from other Macs, GET /peer/hello (M1); FS2 adds GET /grants and POST /grants/answer; the TV's preview
	// GET and POST /desk/preview (V1); PH1 POST /photo and POST /talk/photo; PUSH1 GET and POST /push, POST /push/off.
	if len(phoneRouteTable) != 24 {
		t.Errorf("%d rows; want 24", len(phoneRouteTable))
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
			refusal := row.refusal(d)
			if c.name == "phone-serve-windows" && strings.HasPrefix(row.path, "/push") {
				refusal = pushNotWindowsServe
			}
			if letIn(status) {
				in = append(in, c.name)
			} else if slices.Contains(reachTheTable, c.name) && (status != http.StatusForbidden || text != refusal) {
				t.Errorf("%s, %s: %d %q, want 403 %q", key, c.name, status, text, refusal)
			}
		}
		// The push rows keep out Windows serve, past their class (SIERRA's ruling (b): pushsubs.go).
		class := classLetsIn[row.access]
		if strings.HasPrefix(row.path, "/push") {
			class = slices.DeleteFunc(slices.Clone(class), func(name string) bool { return name == "phone-serve-windows" })
		}
		if !slices.Equal(in, class) {
			t.Errorf("%s lets in %v, its class %v", key, in, class)
		}
		// A photo's rows are 8 MB and 30 s (photo_test.go); every other row is as before the move.
		if row.path == "/photo" || row.path == "/talk/photo" {
			continue
		}
		if most, wait := requestLimits(row.method, row.path); most != maxRequestSize || wait != requestTimeout {
			t.Errorf("%s: %d bytes in %s, as before the move %d in %s", key, most, wait, maxRequestSize, requestTimeout)
		}
	}
	// Another Mac's class never lets in a phone or the window: another Mac is let in by servePeer (peers_mac_test.go).
	for _, phoneID := range []string{"phone", officeWindowID} {
		if accessPeer.lets(&request{Request: httptest.NewRequest("GET", "/", nil)}, phoneID) {
			t.Errorf("the peer class let %s in", phoneID)
		}
	}
}

// The hand-on through `tailscale serve` (a ts.net Host, a proxy's headers or X-Forwarded-Proto alone, Tailscale's own
// process at the other end) answers on Linux (OLD-SERVE1) and on Windows (OLD-SERVE2) as it does on a Mac, cell for
// cell: the dump's three phone-serve-linux and three phone-serve-windows columns. Where it is off (a system that isn't
// one of the three, as Windows was until OLD-SERVE2), the same callers are refused as 1.25 refused them, on every
// route but the open icons, even with Tailscale's own process at the other end.
func TestTheServeHandOnStaysTheMacsUntilSN1(t *testing.T) {
	if serveHandOnHere() != slices.Contains([]string{"darwin", "linux", "windows"}, runtime.GOOS) {
		t.Fatalf("the hand-on is let in = %v on %s: it is a Mac's, Linux's and Windows' only", serveHandOnHere(), runtime.GOOS)
	}
	d, phoneKey := probeDeck(t)
	caller := func(name string) routeCaller {
		i := slices.IndexFunc(routeCallers, func(c routeCaller) bool { return c.name == name })
		if i < 0 {
			t.Fatalf("no caller %s", name)
		}
		return routeCallers[i]
	}
	open := []string{"/icon.png", "/favicon-16.png", "/favicon-32.png"}
	for _, name := range []string{"phone-serve-linux", "phone-serve-linux-text-off", "phone-serve-linux-proto",
		"phone-serve-windows", "phone-serve-windows-text-off", "phone-serve-windows-proto"} {
		off := caller(name)
		if off.mac || off.off || off.linux != strings.Contains(name, "linux") || !off.tailscale {
			t.Fatalf("%s: not that system's caller through `tailscale serve`", name)
		}
		off.linux, off.off = false, true
		for _, route := range everyPhoneRoute {
			want := http.StatusForbidden
			switch {
			case slices.Contains(open, route[1]):
				want = http.StatusOK
			case route[1] == "/peer/nowhere":
				// Under /peer/, servePeer answers first, alike for every caller with no peer key, a stranger
				// included: 403 on its routes, 404 on any other path (M1). Nothing opens.
				want = http.StatusNotFound
			}
			if status, _ := probeRoute(t, d, route[0], route[1], phoneKey, off); status != want {
				t.Errorf("%s %s as %s with the hand-on off: %d, want %d", route[0], route[1], name, status, want)
			}
		}
	}
	// On Linux and on Windows, the same hand-on answers as a Mac's; but on Windows never on the push routes (403,
	// SIERRA's ruling (b): pushsubs.go).
	for twin, mac := range map[string]string{
		"phone-serve-linux": "phone-serve-mac", "phone-serve-linux-text-off": "phone-serve-mac-text-off",
		"phone-serve-windows": "phone-serve-mac", "phone-serve-windows-text-off": "phone-serve-mac-text-off",
		"phone-serve-windows-proto": "phone-serve-linux-proto",
	} {
		for _, route := range everyPhoneRoute {
			onTwin, _ := probeRoute(t, d, route[0], route[1], phoneKey, caller(twin))
			onAMac, _ := probeRoute(t, d, route[0], route[1], phoneKey, caller(mac))
			if strings.Contains(twin, "windows") && strings.HasPrefix(route[1], "/push") {
				onAMac = http.StatusForbidden
			}
			if onTwin != onAMac {
				t.Errorf("%s %s: %s %d, %s %d: the hand-on answers as on a Mac", route[0], route[1], twin, onTwin, mac, onAMac)
			}
		}
	}
	// A program here with a proxy's header made up, on the loopback (no Tailscale process at its other end): on
	// Windows, now that the hand-on is let in there, refused as on a Mac, cell for cell.
	for _, name := range []string{"phone-proxied-localhost-other", "window-proxied", "window-device-proxied"} {
		onWindows := caller(name)
		if onWindows.mac || onWindows.linux || onWindows.off || onWindows.tailscale {
			t.Fatalf("%s: not Windows' program here", name)
		}
		onAMac := onWindows
		onAMac.mac = true
		for _, route := range everyPhoneRoute {
			windows, _ := probeRoute(t, d, route[0], route[1], phoneKey, onWindows)
			mac, _ := probeRoute(t, d, route[0], route[1], phoneKey, onAMac)
			if windows != mac {
				t.Errorf("%s %s as %s: on Windows %d, on a Mac %d", route[0], route[1], name, windows, mac)
			}
		}
	}
	for _, name := range []string{"phone-serve-mac", "phone-serve-linux", "phone-serve-windows"} {
		if status, _ := probeRoute(t, d, "GET", "/events", phoneKey, caller(name)); status != http.StatusOK {
			t.Errorf("%s: %d", name, status)
		}
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
		// The phone on this computer, its key sent: over the Wi-Fi it signs, and a body too big isn't read to be signed.
		r := httptest.NewRequest(http.MethodPost, "/test-photo", strings.NewReader(body))
		r.Host, r.RemoteAddr = "localhost:7710", thisComputer
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
// phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, phone-serve-windows,
// phone-serve-windows-text-off, phone-serve-windows-proto, spoof-proto-wifi, spoof-tsnet-wifi,
// spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi,
// window-device, window-device-proxied (RT126FS2 B2: the owner's tap refuses both, and off a Mac a phone's key from
// the loopback with a proxy's header; RT126FS2b N1: and the window, which is no deck under the Mac app here; SN1: a
// proxy's header made up on the loopback, with no Tailscale process at the other end, is refused its key, 401;
// OLD-SERVE1: the three phone-serve-linux columns are Linux's, where the hand-on answers as on a Mac; OLD-SERVE2:
// the three phone-serve-windows columns are Windows', where it does too, and every caller with no system named is
// Windows' with the hand-on let in).
const phoneRouteAccess = `GET /icon.png: 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off, phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, phone-serve-windows, phone-serve-windows-text-off, phone-serve-windows-proto, spoof-proto-wifi, spoof-tsnet-wifi, spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi, window-device, window-device-proxied
GET /favicon-16.png: 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off, phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, phone-serve-windows, phone-serve-windows-text-off, phone-serve-windows-proto, spoof-proto-wifi, spoof-tsnet-wifi, spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi, window-device, window-device-proxied
GET /favicon-32.png: 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200 200; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off, phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, phone-serve-windows, phone-serve-windows-text-off, phone-serve-windows-proto, spoof-proto-wifi, spoof-tsnet-wifi, spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi, window-device, window-device-proxied
POST /pair: 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403; let in: nobody
GET /office.js: 200 200 200 200 403 200 403 403 200 403 403 200 403 403 403 200 200 403 200 200 403; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-key-on-wifi, window-device
GET /guide.js: 200 200 200 200 403 200 403 403 200 403 403 200 403 403 403 200 200 403 200 200 403; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-key-on-wifi, window-device
GET /licenses: 404 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 404 404 403; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-key-on-wifi, window-device
GET /sounds/keys.m4a: 200 200 200 200 403 200 403 403 200 403 403 200 403 403 403 200 200 403 200 200 403; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-key-on-wifi, window-device
GET /office?o=wrong: 200 200 200 200 403 200 403 403 200 403 403 200 403 403 403 200 200 403 200 200 403; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-key-on-wifi, window-device
GET /: 200 200 200 200 403 200 403 403 200 403 403 200 403 403 403 200 200 403 200 200 403; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-key-on-wifi, window-device
GET /office: 200 200 200 200 403 200 403 403 200 403 403 200 403 403 403 200 200 403 200 200 403; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-key-on-wifi, window-device
GET /paired: 401 204 204 204 403 204 403 403 204 403 403 204 403 403 403 204 204 403 401 204 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /events: 401 200 200 200 403 200 403 403 200 403 403 200 403 403 403 200 200 403 401 200 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /focus: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /act?action=settings: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 204 403 401 403 403; let in: window
GET /hire/window: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 200 403 401 403 403; let in: window
POST /hire/window: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
GET /t3code: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
POST /t3code/open: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
POST /act?action=takeover: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
POST /act?action=handback: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
POST /act?action=letgo: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /act?action=stop: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /dictate: 401 400 400 400 403 400 403 403 400 403 403 400 403 403 403 400 403 403 401 400 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window-device
POST /peek: 401 403 404 404 403 404 403 403 404 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows
POST /choose?option=other: 401 400 400 400 403 400 403 403 400 403 403 400 403 403 403 400 403 403 401 400 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window-device
GET /choice: 401 403 404 404 403 404 403 403 404 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows
GET /approvals: 401 403 200 200 403 200 403 403 200 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows
GET /conversation: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
GET /session/info: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
GET /desk/git: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
GET /desk/preview: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
POST /desk/preview: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
POST /summary: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 401 403 403; let in: nobody
POST /desk/pr/open: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
GET /talk: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
GET /talk/asking: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 200 403 401 403 403; let in: window
POST /talk/say: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 400 403 401 403 403; let in: window
POST /talk/pick: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 400 403 401 403 403; let in: window
POST /talk/listen: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 400 403 401 403 403; let in: window
POST /photo: 401 403 404 404 403 404 403 403 404 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows
POST /talk/photo: 401 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 404 403 401 403 403; let in: window
GET /dictate: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /peek: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /choice: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /approvals: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /conversation: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /desk/pr/open: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /talk: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
PUT /talk: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
POST /talk/asking: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /talk/say: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /talk/pick: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /talk/listen: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /photo: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /talk/photo: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /peer/hello: 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403; let in: nobody
POST /peer/pair: 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403 403; let in: nobody
GET /peer/nowhere: 404 404 404 404 404 404 404 404 404 404 404 404 404 404 404 404 404 404 404 404 404; let in: stranger, phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-mac-text-off, phone-serve-linux, phone-serve-linux-text-off, phone-serve-linux-proto, phone-serve-windows, phone-serve-windows-text-off, phone-serve-windows-proto, spoof-proto-wifi, spoof-tsnet-wifi, spoof-tsnet-wireguard, phone-proxied-localhost-other, phone-here, window, window-proxied, window-key-on-wifi, window-device, window-device-proxied
GET /nowhere: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /grants: 401 403 200 200 403 200 403 403 200 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows
POST /grants/answer: 401 403 400 400 403 400 403 403 400 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows
PUT /grants/answer: 401 404 404 404 403 404 403 403 404 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, phone-serve-windows, spoof-proto-wifi, phone-here, window, window-device
GET /push: 401 403 503 503 403 503 403 403 403 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux
POST /push: 401 403 400 400 403 400 403 403 403 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux
POST /push/off: 401 403 204 204 403 204 403 403 403 403 403 403 403 403 403 403 403 403 401 403 403; let in: phone-wireguard, phone-serve-mac, phone-serve-linux
PUT /push: 401 404 404 404 403 404 403 403 403 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, spoof-proto-wifi, phone-here, window, window-device
GET /push/off: 401 404 404 404 403 404 403 403 403 403 403 404 403 403 403 404 404 403 401 404 403; let in: phone-wifi, phone-wireguard, phone-serve-mac, phone-serve-linux, spoof-proto-wifi, phone-here, window, window-device
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
	"/sign.js": "open: what signs a phone's requests (1.25)", "/licenses": "open", "ident sounds": "open: the office's recordings",
	// The pairing page and the deck page: with no key the deck gives pair.html, and a code is how a phone gets a key.
	"/": "open: pair.html until paired", "/office": "open: pair.html until paired, the window's token here", "/pair": "open: a pairing code",
	// This computer only, with a key of its own: setup (the table routes.go says which caller), the hook, a launcher.
	"/setup": "local + setup caller", "/setup/": "local + setup caller", "/setup/session": "local + setup caller", "/setup/state": "local + setup caller",
	"/bots/": "local + setup caller", "/hook": "local + hook key", "/launcher": "local + hook key", "ident raisePath": "local + hook key",
	"ident hireStartPath": "local + hook key", "ident screenAnswerPath": "local + hook key + the launcher's secret",
	// A terminal asks for a key with no key: it gets one only on the owner's Allow in Smolcorp's window (terminalask.go).
	"ident terminalAskPath": "local, no key: the owner's Allow gives the key",
	// Not a route: the tray's token swapped for the window's key, a redirect to /office.
	"/office?t=": "redirect to /office",
	// A paired phone's key or the window's, past the router's own checks (handle's phoneID): the route says the rest by hand.
	"/events": "phone or window", "/paired": "phone or window (1.25)", "/focus": "phone or window", "/act": "phone or window; settings, takeover, handback and letgo: window",
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
