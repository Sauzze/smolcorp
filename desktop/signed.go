package desktop

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Signed requests (SECWIFI): over plain http on the Wi-Fi, anyone on the same
// network can read every request. So a paired phone never sends its key: the
// deck page keeps it in its address's fragment, which a browser never sends,
// and signs each request with it instead (Resources/web/sign.js). A sniffed
// request can't be sent again: it names its time, which must be within a
// minute of this computer's, and a random nonce, which works once.
//
// The signature is HMAC-SHA256, its secret the SHA-256 of the phone's key (the
// hash phones.json keeps), over:
//
//	smolcorp-v1 \n METHOD \n PATH \n QUERY \n SHA-256 of the body, in hex
//
// where QUERY is the request's query exactly as sent, up to the "&sig=" that
// ends it, and holds ts (milliseconds since 1970) and n (the nonce).
//
// Over this computer's loopback and over Tailscale, which no one on the Wi-Fi
// can read, the key itself is still taken (the screen saver, the office's own
// window, a page bookmarked before signing): never over the Wi-Fi.

const (
	signedVersion = "smolcorp-v1"
	// How far a request's time may be from this computer's.
	signedWindow = 60 * time.Second
	// The most nonces remembered at once: past it, and once those run out are
	// let go, new requests wait. Only correctly signed requests are remembered.
	mostNonces = 4096
)

// When this app started. The nonces seen before are gone with the last start,
// so a request signed earlier, which could be one sniffed then, isn't taken.
var processStarted = time.Now()

// Why a signed request was refused, as the page is told (it acts on "clock").
const (
	signedBad    = "signature"
	signedClock  = "clock"
	signedReplay = "replay"
	signedBusy   = "busy"
)

// signedParts is what a signed request says of itself, before its signature is checked.
type signedParts struct {
	message []byte
	sig     []byte
	at      time.Time
	nonce   string
}

// isSigned is whether the request claims a signature, right or wrong.
func isSigned(r *http.Request) bool { return strings.Contains(r.URL.RawQuery, "sig=") }

// readSigned reads what r says of itself, or false if it isn't a well-formed
// signed request. Its body is read and put back for whoever reads it next.
// Called before the deck's lock is taken: a body that comes slowly, from
// anyone on the Wi-Fi, holds its own request only, until the request's
// deadline (the router's) ends it. A GET has no body: one that comes with
// one isn't read.
func readSigned(r *http.Request) (signedParts, bool) {
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && (r.ContentLength != 0 || len(r.TransferEncoding) > 0) {
		return signedParts{}, false
	}
	raw := r.URL.RawQuery
	end := strings.LastIndex(raw, "&sig=")
	if end < 0 {
		return signedParts{}, false
	}
	sig, err := hex.DecodeString(raw[end+len("&sig="):])
	if err != nil || len(sig) != sha256.Size {
		return signedParts{}, false
	}
	query := raw[:end]
	values, err := url.ParseQuery(query)
	// One time and one nonce, and no key or second signature hidden in what's signed.
	if err != nil || len(values["ts"]) != 1 || len(values["n"]) != 1 || values.Has("sig") || values.Has("t") {
		return signedParts{}, false
	}
	millis, err := strconv.ParseInt(values.Get("ts"), 10, 64)
	nonce := values.Get("n")
	if err != nil || !isNonce(nonce) {
		return signedParts{}, false
	}
	var body []byte
	if r.Body != nil {
		if body, err = io.ReadAll(r.Body); err != nil {
			return signedParts{}, false
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	bodySum := sha256.Sum256(body)
	message := strings.Join([]string{signedVersion, r.Method, r.URL.Path, query, hex.EncodeToString(bodySum[:])}, "\n")
	return signedParts{message: []byte(message), sig: sig, at: time.UnixMilli(millis), nonce: nonce}, true
}

// isNonce: 16 to 64 of base64url's letters, as sign.js makes them.
func isNonce(nonce string) bool {
	if len(nonce) < 16 || len(nonce) > 64 {
		return false
	}
	return strings.Trim(nonce, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") == ""
}

// signedBy is whether parts were signed with the key whose SHA-256, in hex, is keyHash.
func (parts signedParts) signedBy(keyHash string) bool {
	secret, err := hex.DecodeString(keyHash)
	if err != nil || len(secret) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(parts.message)
	return hmac.Equal(mac.Sum(nil), parts.sig)
}

// fresh says why a correctly signed request is refused, "" if it isn't: its
// time too far from now or from before this app started, or its nonce seen
// before. The nonce is used up.
func (a *Access) fresh(parts signedParts, now time.Time) string {
	if gap := now.Sub(parts.at); gap > signedWindow || gap < -signedWindow || parts.at.Before(a.started) {
		return signedClock
	}
	if a.nonces == nil {
		a.nonces = map[string]time.Time{}
	}
	if _, seen := a.nonces[parts.nonce]; seen {
		return signedReplay
	}
	if len(a.nonces) >= mostNonces {
		for nonce, until := range a.nonces {
			if now.After(until) {
				delete(a.nonces, nonce)
			}
		}
		if len(a.nonces) >= mostNonces {
			return signedBusy
		}
	}
	// Remembered for as long as its time would still be taken.
	a.nonces[parts.nonce] = parts.at.Add(signedWindow)
	return ""
}

// SignedPhone is the paired phone that signed parts (readSigned's, with its
// ok), or nil and why not. Only a phone whose key never crossed the Wi-Fi in
// clear may sign (PairedPhone.Signs).
func (a *Access) SignedPhone(parts signedParts, ok bool, now time.Time) (*PairedPhone, string) {
	if !ok {
		return nil, signedBad
	}
	for i := range a.Phones.All {
		if a.Phones.All[i].Signs && parts.signedBy(a.Phones.All[i].KeyHash) {
			if why := a.fresh(parts, now); why != "" {
				return nil, why
			}
			return &a.Phones.All[i], ""
		}
	}
	return nil, signedBad
}

// writeSignedRefusal answers a signed request that was refused, with this
// computer's time, so a phone whose clock is off can sign with this one's.
func writeSignedRefusal(w http.ResponseWriter, why string) {
	status := http.StatusUnauthorized
	if why == signedBusy {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"error": signedRefusals[why], "why": why, "now": time.Now().UnixMilli()})
}

var signedRefusals = map[string]string{
	signedBad:    "This phone isn't paired with this computer.",
	signedClock:  "This phone's clock is too far from your computer's.",
	signedReplay: "That request was already used.",
	signedBusy:   "Too many requests: try again in a minute.",
}
