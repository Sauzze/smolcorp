package desktop

import (
	"crypto/ecdh"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
)

// Web Push's subscriptions (1.26 PUSH1). A subscription lets whoever holds the app's VAPID key ring a phone, so it
// is something that grants (SK1): it is kept on the paired phone that posted it, in phones.json, the sealed record
// that phone is in (access.go), held in memory from the core's start and never read again while it runs. It goes
// with its phone when that phone is unpaired, with every phone when the list's seal doesn't fit, and is never kept
// while the records' key can't be read. Only the phone that posted one sees it, replaces it or turns it off: another
// phone posting its address is refused (setPush). Nothing here sends: that's PUSH3a's.
//
// The routes are a paired phone's over Tailscale alone (accessPhoneSecure): never the Wi-Fi's plain http, nor the
// office's window. Through `tailscale serve` that is HTTPS, which a page needs to subscribe at all, and it passes the
// door only with the phone's typing switch on, as every phone route there does (F3: notifications keep needing
// both, with no door of their own).
//
// Except on Windows (SIERRA's ruling (b), 1.26): there the signed-in user, and so a hire, can change `tailscale
// serve`, and nothing in 1.26 keeps a hire from the tailscale CLI, so a request through serve on Windows is refused on
// every /push route, on any method (pushRefusal). Over WireGuard, on a Mac and on Linux it's let in as above. To be
// reopened, with its own review, once design A's Windows pieces (SA-W2/W3) keep hires from the tailscale CLI and
// serve's settings there.

// pushSubscriptionAuth is the size of a browser's auth secret (RFC 8291), in bytes.
const pushSubscriptionAuth = 16

// usable is whether s is a subscription this app keeps: the address of a push service it sends to (pushEndpoint),
// the browser's P-256 key as a point on the curve, uncompressed, and an auth secret of 16 bytes, both base64url.
func (s pushSubscription) usable() bool {
	if _, err := pushEndpoint(s.Endpoint); err != nil {
		return false
	}
	public, err := b64.DecodeString(strings.TrimRight(s.Keys.P256dh, "="))
	if err != nil || len(public) != 65 {
		return false
	}
	if _, err := ecdh.P256().NewPublicKey(public); err != nil {
		return false
	}
	auth, err := b64.DecodeString(strings.TrimRight(s.Keys.Auth, "="))
	return err == nil && len(auth) == pushSubscriptionAuth
}

// pushOf is the subscription of the phone id, or nil. The caller holds d.mu.
func (p *Phones) pushOf(id string) *pushSubscription {
	for i := range p.All {
		if p.All[i].ID == id {
			return p.All[i].Push
		}
	}
	return nil
}

// errPushNoPhone: no paired phone has the id; errPushOtherPhones: another paired phone holds the address posted.
var (
	errPushNoPhone     = errors.New("no such paired phone")
	errPushOtherPhones = errors.New("another paired phone holds this subscription")
)

// setPush keeps subscription as the phone id's, the phone that posted it, in place of any it had. A subscription is
// its phone's alone (RT126PUSH1 B1): one at an address another paired phone holds is refused, and that phone's is
// left as it was, never cleared or moved. A browser paired again so rings each entry that holds it until the owner
// removes the old one in Setup, which takes its subscription with it; ringing one address once is the sender's
// (PUSH3a). One replaced is taken away, so it's written under a new record key, as dropPush's: a copy of phones.json
// from before doesn't bring it back. The same subscription posted again writes nothing. A subscription is never
// changed where it is, only put in place of another, so a writer made before (records.go) keeps what it held. The
// caller holds d.mu.
func (p *Phones) setPush(id string, subscription pushSubscription) error {
	i := slices.IndexFunc(p.All, func(phone PairedPhone) bool { return phone.ID == id })
	if i < 0 {
		return errPushNoPhone
	}
	address := pushAddress(subscription.Endpoint)
	for j := range p.All {
		if j != i && p.All[j].Push != nil && pushAddress(p.All[j].Push.Endpoint) == address {
			return errPushOtherPhones
		}
	}
	had := p.All[i].Push
	if had != nil && *had == subscription {
		return nil
	}
	p.All[i].Push = &subscription
	p.keep(had != nil)
	return nil
}

// pushAddress is the browser endpoint names, for telling whether two are the same: its host, its path as decoded and
// its query in order, so the same address written another way (":443", "%41" for "A", the query's parts in another
// order) is the same. Only for comparing: what's kept, and sent to, is the endpoint as the browser gave it.
func pushAddress(endpoint string) string {
	u, err := pushEndpoint(endpoint)
	if err != nil {
		return endpoint
	}
	return u.Hostname() + u.Path + "?" + u.Query().Encode()
}

// dropPush forgets the phone id's subscription, if it has one, under a new record key, as any taking away is
// (records.go): a copy of phones.json from before would ring that phone again. The caller holds d.mu.
func (p *Phones) dropPush(id string) {
	for i := range p.All {
		if p.All[i].ID == id && p.All[i].Push != nil {
			p.All[i].Push = nil
			p.keep(true)
			return
		}
	}
}

// What the phone is told.
const (
	pushTailnet         = "Notifications are turned on only over Tailscale's HTTPS address."
	pushKeyMissing      = "Notifications can't be turned on now: the key they're signed with couldn't be read or kept here."
	pushNotSubscription = "That isn't a browser's subscription to a push service Smolcorp sends to."
	pushOtherPhones     = "Notifications from this browser are already on for another paired phone. Turn them off on that phone, or remove it in Setup, then try again."
	pushNotWindowsServe = "On Windows, notifications can't be turned on through tailscale serve yet."
)

// pushRefusal is what a request to a push route (/push and under it, any method) is told when it came through
// `tailscale serve` on Windows (request.windowsServe), else "". Asked at the table's gate, before the route's row, so
// the wrong methods are refused too.
func pushRefusal(r *request) string {
	if r.windowsServe && (r.URL.Path == "/push" || strings.HasPrefix(r.URL.Path, "/push/")) {
		return pushNotWindowsServe
	}
	return ""
}

// GET /push: the app's public VAPID key, for the page to subscribe with (applicationServerKey), and whether this
// phone has a subscription kept: `{key, on}`. Asking makes the key, the first time, in the store (pushkey.go); 503
// when there's none to read or keep (a test, a Mac without the app, a store that won't answer).
// POST /push: this phone's subscription, as the browser's PushSubscription.toJSON() gives it, in place of any it had;
// 204, 400 (not one this app keeps), 409 (another paired phone holds that address, or the records can't take it now).
// POST /push/off: this phone's subscription forgotten; 204, whether or not there was one.
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/push", access: accessPhoneSecure, refusal: says(pushTailnet),
		serve: func(d *Deck, w http.ResponseWriter, r *request, _ string) {
			// The store is waited on outside the deck's lock.
			key, err := d.pushKeys.publicKey()
			if err != nil {
				writeStatus(w)(http.StatusServiceUnavailable, pushKeyMissing)
				return
			}
			d.mu.Lock()
			on := d.access.Phones.pushOf(r.phoneID) != nil
			d.mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]any{"key": key, "on": on})
		}})
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/push", access: accessPhoneSecure, refusal: says(pushTailnet),
		serve: func(d *Deck, w http.ResponseWriter, r *request, _ string) {
			// Never from the Wi-Fi, as the class says already (RSECWIFI N3: a body from a phone asks).
			if !d.typesFrom(r) {
				writeStatus(w)(http.StatusForbidden, pushTailnet)
				return
			}
			writeStatus(w)(d.subscribePush(r.phoneID, r.Body))
		}})
	registerPhoneRoute(phoneRoute{method: http.MethodPost, path: "/push/off", access: accessPhoneSecure, refusal: says(pushTailnet),
		serve: func(d *Deck, w http.ResponseWriter, r *request, _ string) {
			d.mu.Lock()
			d.access.Phones.dropPush(r.phoneID)
			d.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}})
}

// subscribePush keeps the subscription in body as the phone phoneID's.
func (d *Deck) subscribePush(phoneID string, body io.Reader) (int, string) {
	var subscription pushSubscription
	if json.NewDecoder(body).Decode(&subscription) != nil || !subscription.usable() {
		return http.StatusBadRequest, pushNotSubscription
	}
	subscription.Keys.P256dh = strings.TrimRight(subscription.Keys.P256dh, "=")
	subscription.Keys.Auth = strings.TrimRight(subscription.Keys.Auth, "=")
	d.mu.Lock()
	defer d.mu.Unlock()
	// While the records' key can't be read, or after a check that failed, nothing is kept: no phone is let in then
	// anyway (loadPhones), and pairing is refused.
	if refusal := recordsRefusal(); refusal != "" {
		return http.StatusConflict, refusal
	}
	switch err := d.access.Phones.setPush(phoneID, subscription); {
	case errors.Is(err, errPushOtherPhones):
		return http.StatusConflict, pushOtherPhones
	case err != nil:
		return http.StatusNotFound, ""
	}
	return http.StatusNoContent, ""
}
