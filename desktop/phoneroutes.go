package desktop

import (
	"net"
	"net/http"
	"slices"
	"strings"
	"time"
)

// The phone's and the office window's routes, as a table, the way setup has its routeTable (routes.go): each row
// says a method and a path, who may use it (its access class), what anyone else is told, and how big and slow its
// request may be. A feature registers its own rows from its own file, in an init (registerPhoneRoute), so server.go's
// switch isn't every feature's. The table is read after the router's own checks, which stay as they were: the
// request addressed as a phone's (or handed on by `tailscale serve` with the phone's text on), and a paired phone's
// key or the window's on this computer. A route not in the table goes on to handle's switch.

// access is a route's class: who may use it.
type access int

const (
	// The office's own window on this computer, with its key: never a phone.
	accessWindow access = iota + 1
	// A paired phone, over the Wi-Fi or Tailscale: never the office's window.
	accessPhone
	// A paired phone over Tailscale (request.secure): never the Wi-Fi's plain http, nor the window.
	accessPhoneSecure
	// The owner's own device: the Mac app's own office window, under the Mac app, with its key, or a paired phone
	// over Tailscale (never the Wi-Fi's plain http). Never the office in a browser, whose key the browser keeps in
	// its history, so never a window off a Mac (RT126FS2b N1); never the paired device the Windows window holds,
	// whose key is in a file any program here can read; never a request from this computer's own Tailscale address,
	// nor, off a Mac, from its loopback with a proxy's headers. The routes that take the owner's tap (grants.go).
	accessOwnerTap
	// Another Mac paired for floors (peers_mac.go), with its peer key, over Tailscale, with the setting on: let in by
	// servePeer alone, before the router's other checks, never as a phone or the window. Its rows are under /peer/.
	accessPeer
)

// phoneRoute is one row of the table.
type phoneRoute struct {
	method, path string
	access       access
	// What a caller its class keeps out is told, with a 403.
	refusal func(d *Deck) string
	// The most its body may hold, and how long it may take to arrive; 0 is the server's own (maxRequestSize,
	// requestTimeout).
	maxBody int64
	timeout time.Duration
	serve   func(d *Deck, w http.ResponseWriter, r *request, session string)
}

// phoneRouteTable is the rows by method and path. Written only by the inits that register them, so read freely.
var phoneRouteTable = map[string]*phoneRoute{}

// registerPhoneRoute adds a row, from its feature's init. A route twice, or a row missing its class, refusal or
// handler, is a mistake that stops the program at its start, never a route let through.
func registerPhoneRoute(row phoneRoute) {
	key := row.method + " " + row.path
	if _, taken := phoneRouteTable[key]; taken || row.access == 0 || row.refusal == nil || row.serve == nil {
		panic("phone route registered twice, or without its class, refusal or handler: " + key)
	}
	// Another Mac's rows are under /peer/, where servePeer answers, and nobody else's are.
	if (row.access == accessPeer) != strings.HasPrefix(row.path, "/peer/") {
		panic("a peer route outside /peer/, or another class's inside it: " + key)
	}
	phoneRouteTable[key] = &row
}

// phoneRouteFor is the row for a method and path, or nil.
func phoneRouteFor(method, path string) *phoneRoute { return phoneRouteTable[method+" "+path] }

// requestLimits are how big a request's body may be and how long it may take to arrive: its row's, else the server's.
func requestLimits(method, path string) (int64, time.Duration) {
	size, wait := int64(maxRequestSize), requestTimeout
	if row := phoneRouteFor(method, path); row != nil {
		if row.maxBody > 0 {
			size = row.maxBody
		}
		if row.timeout > 0 {
			wait = row.timeout
		}
	}
	return size, wait
}

// secure says whether the request came over Tailscale: to this computer's tailnet address from another device's,
// which WireGuard carries encrypted, or handed on by `tailscale serve` (HTTPS) on this computer. Never the Wi-Fi's
// plain http. Asked only by a route that needs it, as reading Tailscale's addresses isn't free (fromTailnet).
func (r *request) secure() bool { return fromTailnet(r.Request) }

// ownerTapOverTailnet is secure, less what a program on this computer can say for itself: `tailscale serve` hands a
// phone on only where serveHandOnHere (a Mac and Linux through serve.sock, Windows through the serve port), so
// elsewhere a request from the loopback, or with no address, is a local program's, and only WireGuard's path is a
// phone's tap. A request from
// one of this computer's own Tailscale addresses is a program here too, on any system (RT126FS2b N2).
func (r *request) ownerTapOverTailnet() bool {
	ip := net.ParseIP(r.remoteAddress)
	if !serveHandOnHere() && (ip == nil || ip.IsLoopback()) {
		return false
	}
	if ip != nil && slices.Contains(tailscaleAddresses(), ip.String()) {
		return false
	}
	return r.secure()
}

// lets says whether a class lets in a caller already known: phoneID is a paired phone's, or the window's.
func (a access) lets(r *request, phoneID string) bool {
	switch a {
	case accessWindow:
		return phoneID == officeWindowID
	case accessPhone:
		return phoneID != officeWindowID
	case accessPhoneSecure:
		return phoneID != officeWindowID && r.secure()
	case accessOwnerTap:
		if phoneID == officeWindowID {
			return r.ownWindow
		}
		return !r.windowDevice && r.ownerTapOverTailnet()
	}
	// accessPeer: a phone or the window, never; another Mac is let in by servePeer.
	return false
}

// servePhoneRoute serves a request by its row, if the table has one, and says whether it did.
func (d *Deck) servePhoneRoute(w http.ResponseWriter, r *request, phoneID string) bool {
	// Before its row: the push routes, on any method, refuse a request through `tailscale serve` on Windows.
	if refusal := pushRefusal(r); refusal != "" {
		writeStatus(w)(http.StatusForbidden, refusal)
		return true
	}
	row := phoneRouteFor(r.Method, r.URL.Path)
	if row == nil {
		return false
	}
	if !row.access.lets(r, phoneID) {
		writeStatus(w)(http.StatusForbidden, row.refusal(d))
		return true
	}
	r.phoneID = phoneID
	row.serve(d, w, r, r.URL.Query().Get("session"))
	return true
}

// says is a refusal that is always the same words.
func says(text string) func(*Deck) string { return func(*Deck) string { return text } }
