package desktop

import (
	"bytes"
	"cmp"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Web Push (1.26 W2): the sender that rings a phone whose page has turned notifications on, through the push
// service of its browser (Apple, Google, Mozilla or Microsoft). Only the standard library: the application server's
// VAPID key signs an ES256 token (RFC 8292), and what is sent is encrypted for that one browser (RFC 8291, aes128gcm
// of RFC 8188), so the push service carries it without reading it. What is sent is a pushNote and nothing else: an
// event word, a desk's display name and its floor. Never a prompt, a conversation line, a folder, a branch, a diff
// or a file name: no secrets and no code. The routes, the service worker and the subscriptions kept per phone are
// W1 and W3.

// pushNote is everything a notification says: which event, at which desk, on which floor (0 the office itself). A
// test pins its JSON keys; add a field only with the owner's OK.
type pushNote struct {
	Event string `json:"event"`
	Desk  string `json:"desk"`
	Floor int    `json:"floor"`
}

// pushEvents are the event words a phone can be rung for, the webhook's own (webhookEvent): a desk waiting on you,
// done, or failed.
var pushEvents = []string{"waiting", "done", "error"}

// pushDeskMost is the longest desk name a notification carries, in characters.
const pushDeskMost = 64

// newPushNote is the note for event at the desk named desk on floor: the name trimmed, and cut to pushDeskMost.
func newPushNote(event, desk string, floor int) pushNote {
	desk = strings.TrimSpace(desk)
	if utf8.RuneCountInString(desk) > pushDeskMost {
		desk = string([]rune(desk)[:pushDeskMost-1]) + "…"
	}
	return pushNote{Event: event, Desk: desk, Floor: floor}
}

// valid is whether n is one this app sends: a known event, a desk named in plain text, a floor of the building.
func (n pushNote) valid() bool {
	if !slices.Contains(pushEvents, n.Event) || n.Floor < 0 || n.Floor >= floorCount {
		return false
	}
	if n.Desk == "" || !utf8.ValidString(n.Desk) || utf8.RuneCountInString(n.Desk) > pushDeskMost {
		return false
	}
	// Plain text only: no control characters, no format characters (text direction, zero-width) and no line or
	// paragraph separators, which could make a name read as something else.
	return !strings.ContainsFunc(n.Desk, func(r rune) bool {
		return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp)
	})
}

// urgency is the Urgency header for n (RFC 8030): a desk waiting on you or failed is worth waking the phone for.
func (n pushNote) urgency() string {
	if n.Event == "done" {
		return "normal"
	}
	return "high"
}

// pushSubscription is a browser's subscription, as its PushSubscription.toJSON() gives it: the push service's
// address for it, the browser's P-256 key and its 16-byte auth secret, both base64url.
type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// pushHosts are the push services sent to, by host, and nothing else: Google's (Chrome, Android), Mozilla's
// (Firefox) and Apple's (Safari on a Mac, an iPhone or an iPad). Microsoft's (Edge) gives each browser an address
// on one of its servers: any one name under pushWindowsHost, as wns2-par02p.notify.windows.com. The owner said yes
// to Microsoft's beside the other three (1.26 F4, 14 Nov), on the same rule: no secrets and no code.
var pushHosts = []string{"fcm.googleapis.com", "updates.push.services.mozilla.com", "web.push.apple.com"}

const pushWindowsHost = ".notify.windows.com"

// pushEndpointMost is the longest push address taken.
const pushEndpointMost = 2048

// pushEndpoint is endpoint as a push address this app sends to: https, on one of the push hosts, at the usual port,
// with no name and password in it. Anything else is errPushEndpoint.
func pushEndpoint(endpoint string) (*url.URL, error) {
	if len(endpoint) > pushEndpointMost {
		return nil, errPushEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Fragment != "" {
		return nil, errPushEndpoint
	}
	if port := u.Port(); port != "" && port != "443" {
		return nil, errPushEndpoint
	}
	if !pushHostAllowed(u.Hostname()) {
		return nil, errPushEndpoint
	}
	return u, nil
}

// pushHostAllowed is whether host is one of the push hosts, as written (lower case, no dot at the end).
func pushHostAllowed(host string) bool {
	if slices.Contains(pushHosts, host) {
		return true
	}
	server, ok := strings.CutSuffix(host, pushWindowsHost)
	if !ok || server == "" {
		return false
	}
	for _, r := range server {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

var (
	errPushEndpoint     = errors.New("not the address of a push service this app sends to")
	errPushSubscription = errors.New("the subscription's keys aren't a browser's")
	errPushNote         = errors.New("not a notification this app sends")
	errPushTooLong      = errors.New("too long to send as a notification")
	// errPushGone: the push service answered 404 or 410, so the subscription is no more and is to be dropped.
	errPushGone = errors.New("the push service no longer knows this subscription")
)

// pushBusy: the push service asked for a pause (429) or failed (5xx), so it isn't sent to again before until.
type pushBusy struct {
	status int
	until  time.Time
}

func (b *pushBusy) Error() string {
	if b.status == 0 {
		return "the push service asked for a pause: not sent"
	}
	return fmt.Sprintf("the push service answered %d: not sent again for a while", b.status)
}

// pushRefused: the push service answered something else than 2xx, 404, 410, 429 or 5xx.
type pushRefused struct{ status int }

func (r *pushRefused) Error() string { return fmt.Sprintf("the push service answered %d", r.status) }

// The message's encryption (RFC 8291 §3.4, RFC 8188 §2): one record of pushRecordSize at most, under a header of
// the salt, the record size and the application server's one-time public key.
const (
	pushRecordSize = 4096
	pushSaltSize   = 16
	pushAuthSize   = 16
	pushTagSize    = 16
)

// pushEncrypt encrypts plaintext for the browser with public key uaPublic and auth secret auth, with the one-time
// key as and salt, as RFC 8291 says: the body to post, with its header.
func pushEncrypt(plaintext, uaPublic, auth []byte, as *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	ua, err := ecdh.P256().NewPublicKey(uaPublic)
	if err != nil || len(auth) != pushAuthSize {
		return nil, errPushSubscription
	}
	if len(salt) != pushSaltSize || as.Curve() != ecdh.P256() {
		return nil, errors.New("not a salt and a key to encrypt a notification with")
	}
	asPublic := as.PublicKey().Bytes()
	header := make([]byte, 0, pushSaltSize+5+len(asPublic))
	header = append(header, salt...)
	header = binary.BigEndian.AppendUint32(header, pushRecordSize)
	header = append(header, byte(len(asPublic)))
	header = append(header, asPublic...)
	// One record, the last: the text and its delimiter, 2, then the tag.
	// RFC 8291 §4: the whole body, header and record, in 4096 bytes.
	if len(header)+len(plaintext)+1+pushTagSize > pushRecordSize {
		return nil, errPushTooLong
	}
	shared, err := as.ECDH(ua)
	if err != nil {
		return nil, errPushSubscription
	}
	keyInfo := "WebPush: info\x00" + string(ua.Bytes()) + string(asPublic)
	ikm, err := hkdf.Key(sha256.New, shared, auth, keyInfo, 32)
	if err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	record := append(slices.Clip(plaintext), 2)
	return gcm.Seal(header, nonce, record, nil), nil
}

// vapidSubject is who the push services may reach about this app's messages (RFC 8292 §2.1): the app's own page,
// the same for everyone, never anyone's address.
const vapidSubject = "https://github.com/Sauzze/smolcorp"

// vapidLifetime is how long a VAPID token is good for: 12 hours, under the 24 the push services take at most.
const vapidLifetime = 12 * time.Hour

var b64 = base64.RawURLEncoding

// vapidAuthorization is the Authorization header for the push service at audience (its origin, as
// https://fcm.googleapis.com), signed with key at now (RFC 8292 §3): the token, and the key's public half it
// verifies with.
func vapidAuthorization(key *ecdsa.PrivateKey, audience string, now time.Time) (string, error) {
	claims, err := json.Marshal(struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}{audience, now.Add(vapidLifetime).Unix(), vapidSubject})
	if err != nil {
		return "", err
	}
	signed := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signed))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	// ES256's signature is r then s, 32 bytes each (RFC 7518 §3.4), not the ASN.1 of X.509.
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	public, err := key.PublicKey.Bytes()
	if err != nil {
		return "", err
	}
	return "vapid t=" + signed + "." + b64.EncodeToString(signature) + ", k=" + b64.EncodeToString(public), nil
}

// The sender's limits: how long a notification is kept by the push service for a phone that's off (TTL), how long
// a send may take, how much of an answer is read, and how long a push service that's busy or failing is left alone,
// doubled each time it is again, unless it says how long (Retry-After).
const (
	pushTTL          = time.Hour
	pushDeadline     = 10 * time.Second
	pushAnswerMost   = 4 << 10
	pushHeadersMost  = 16 << 10
	pushBackoffFirst = 30 * time.Second
	pushBackoffMost  = 30 * time.Minute
)

// pushSender posts notifications to the push services, signed with the app's VAPID key. Safe for use by several
// goroutines.
type pushSender struct {
	keys   *pushKeys
	client *http.Client
	now    func() time.Time
	mu     sync.Mutex
	// paused is, by push host, until when it isn't sent to and how long its next pause is.
	paused map[string]pushPause
}

type pushPause struct {
	until time.Time
	next  time.Duration
}

func newPushSender(keys *pushKeys) *pushSender {
	// Its own transport: no proxy from the environment, and an answer's headers held to a little more than the
	// answer's body.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxResponseHeaderBytes = pushHeadersMost
	return &pushSender{
		keys: keys,
		client: &http.Client{
			Transport: transport,
			Timeout:   pushDeadline,
			// A push service answers; it never sends a notification elsewhere.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now:    time.Now,
		paused: map[string]pushPause{},
	}
}

// send sends note to the browser of subscription, and says nil once its push service has taken it. Never a note
// that isn't valid, nor to an address that isn't a push host's. errPushGone says the subscription is to be dropped;
// a *pushBusy, that the push service isn't sent to until its time. It waits on the network, and on the secret store
// the first time: never call it with d.mu held.
func (s *pushSender) send(ctx context.Context, subscription pushSubscription, note pushNote) error {
	if !note.valid() {
		return errPushNote
	}
	endpoint, err := pushEndpoint(subscription.Endpoint)
	if err != nil {
		return err
	}
	uaPublic, err := b64.DecodeString(strings.TrimRight(subscription.Keys.P256dh, "="))
	if err != nil {
		return errPushSubscription
	}
	auth, err := b64.DecodeString(strings.TrimRight(subscription.Keys.Auth, "="))
	if err != nil {
		return errPushSubscription
	}
	host := endpoint.Hostname()
	if err := s.pausedFor(host); err != nil {
		return err
	}
	key, err := s.keys.key()
	if err != nil {
		return err
	}
	plaintext, err := json.Marshal(note)
	if err != nil {
		return err
	}
	as, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	salt := make([]byte, pushSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	body, err := pushEncrypt(plaintext, uaPublic, auth, as, salt)
	if err != nil {
		return err
	}
	authorization, err := vapidAuthorization(key, "https://"+host, s.now())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, pushDeadline)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return errPushEndpoint
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Encoding", "aes128gcm")
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("TTL", strconv.Itoa(int(pushTTL/time.Second)))
	request.Header.Set("Urgency", note.urgency())
	request.Header.Set("User-Agent", "Smolcorp")
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	// Only a little of the answer is read, so the connection can be used again, and nothing of it kept.
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, pushAnswerMost))
	response.Body.Close()
	return s.answered(host, response)
}

// pausedFor is a *pushBusy while host is left alone, else nil.
func (s *pushSender) pausedFor(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.paused[host]; ok && s.now().Before(p.until) {
		return &pushBusy{until: p.until}
	}
	return nil
}

// answered is what host's answer to a send says, and notes a pause for a push service that's busy or failing.
func (s *pushSender) answered(host string, response *http.Response) error {
	status := response.StatusCode
	s.mu.Lock()
	defer s.mu.Unlock()
	if status != http.StatusTooManyRequests && status < 500 {
		// It answered: the next time it's busy, it is left alone for the first pause again.
		delete(s.paused, host)
	}
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusNotFound || status == http.StatusGone:
		return errPushGone
	case status == http.StatusTooManyRequests || status >= 500 && status < 600:
		now := s.now()
		pause := cmp.Or(s.paused[host].next, pushBackoffFirst)
		// A Retry-After can ask for a longer pause than our own, never a shorter one: "0", a past date or a
		// negative number mustn't turn the back-off off. Either way, no longer than pushBackoffMost.
		wait := pause
		if asked, ok := retryAfter(response.Header.Get("Retry-After"), now); ok {
			wait = max(asked, pause)
		}
		wait = min(wait, pushBackoffMost)
		s.paused[host] = pushPause{until: now.Add(wait), next: min(2*pause, pushBackoffMost)}
		return &pushBusy{status: status, until: now.Add(wait)}
	}
	return &pushRefused{status: status}
}

// retryAfter is how long a Retry-After header asks to wait from now: seconds, or a date (RFC 9110 §10.2.3).
func retryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		return max(at.Sub(now), 0), true
	}
	return 0, false
}
