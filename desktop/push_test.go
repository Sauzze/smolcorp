package desktop

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := b64.DecodeString(s)
	if err != nil {
		t.Fatalf("not base64url: %q: %v", s, err)
	}
	return b
}

// RFC 8291's example (Appendix A), byte for byte: the keys, the salt, and the message they make.
func TestPushEncryptRFC8291Vector(t *testing.T) {
	plaintext := mustB64(t, "V2hlbiBJIGdyb3cgdXAsIEkgd2FudCB0byBiZSBhIHdhdGVybWVsb24")
	if string(plaintext) != "When I grow up, I want to be a watermelon" {
		t.Fatalf("plaintext: %q", plaintext)
	}
	as, err := ecdh.P256().NewPrivateKey(mustB64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	if got := b64.EncodeToString(as.PublicKey().Bytes()); got != "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8" {
		t.Fatalf("the application server's public key: %s", got)
	}
	uaPublic := mustB64(t, "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4")
	auth := mustB64(t, "BTBZMqHH6r4Tts7J_aSIgg")
	salt := mustB64(t, "DGv6ra1nlYgDCS1FRnbzlw")
	body, err := pushEncrypt(plaintext, uaPublic, auth, as, salt)
	if err != nil {
		t.Fatal(err)
	}
	const want = "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if got := b64.EncodeToString(body); got != want {
		t.Fatalf("the encrypted message isn't RFC 8291's:\n got %s\nwant %s", got, want)
	}
	// And the browser's half decrypts it, with its private key (RFC 8291's too).
	ua, err := ecdh.P256().NewPrivateKey(mustB64(t, "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"))
	if err != nil || !slices.Equal(ua.PublicKey().Bytes(), uaPublic) {
		t.Fatalf("the browser's keys don't match: %v", err)
	}
	if got := decryptPush(t, body, ua, auth); !slices.Equal(got, plaintext) {
		t.Fatalf("the browser reads %q", got)
	}
}

func TestPushEncryptRefusesWhatIsNotAKeyOrTooLong(t *testing.T) {
	as, _ := ecdh.P256().GenerateKey(nil)
	ua, _ := ecdh.P256().GenerateKey(nil)
	salt, auth := make([]byte, pushSaltSize), make([]byte, pushAuthSize)
	if _, err := pushEncrypt([]byte("x"), []byte("not a key"), auth, as, salt); !errors.Is(err, errPushSubscription) {
		t.Errorf("a key that isn't one: %v", err)
	}
	if _, err := pushEncrypt([]byte("x"), ua.PublicKey().Bytes(), auth[:8], as, salt); !errors.Is(err, errPushSubscription) {
		t.Errorf("a short auth secret: %v", err)
	}
	if _, err := pushEncrypt(make([]byte, pushRecordSize), ua.PublicKey().Bytes(), auth, as, salt); !errors.Is(err, errPushTooLong) {
		t.Errorf("a message over 4096 bytes: %v", err)
	}
}

// memoryPushStore stands in for the secret store: what it holds, and how many writes it took.
type memoryPushStore struct {
	mu      sync.Mutex
	value   string
	writes  int
	readErr error
}

func (m *memoryPushStore) store() gmTokenStore {
	return gmTokenStore{
		read: func() (string, error) {
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.value, m.readErr
		},
		write: func(v string) error {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.value = v
			m.writes++
			return nil
		},
	}
}

func TestPushKeyMadeOnceAndKept(t *testing.T) {
	kept := &memoryPushStore{}
	keys := newPushKeys(kept.store())
	first, err := keys.publicKey()
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := keys.publicKey(); again != first || kept.writes != 1 {
		t.Fatalf("asked again: %s, %d writes; want the same key, written once", again, kept.writes)
	}
	if public := mustB64(t, first); len(public) != 65 || public[0] != 4 {
		t.Errorf("the public key isn't an uncompressed P-256 point: %d bytes", len(public))
	}
	if strings.Contains(first, kept.value) {
		t.Error("the private key is in what's served")
	}
	// The next run reads it back, and writes nothing.
	next := newPushKeys(kept.store())
	if again, err := next.publicKey(); err != nil || again != first || kept.writes != 1 {
		t.Fatalf("the next run: %s, %v, %d writes; want the same key, never written again", again, err, kept.writes)
	}
}

func TestPushKeyNeverMadeOverOneThatCantBeRead(t *testing.T) {
	kept := &memoryPushStore{value: "kept", readErr: errors.New("locked")}
	if _, err := newPushKeys(kept.store()).key(); err == nil || kept.writes != 0 || kept.value != "kept" {
		t.Fatalf("a store that can't be read: %v, %d writes", err, kept.writes)
	}
	// Nowhere to keep one: no key, so none that would break the subscriptions at the next run.
	if _, err := newPushKeys(noPushKeyStore).key(); !errors.Is(err, errNoPushKeyStore) {
		t.Fatalf("no store: %v", err)
	}
}

// The VAPID token verifies with the public key served, says which push service it's for, and ends within 12 hours.
func TestVAPIDTokenVerifiesWithThePublicKey(t *testing.T) {
	keys := newPushKeys((&memoryPushStore{}).store())
	key, err := keys.key()
	if err != nil {
		t.Fatal(err)
	}
	served, _ := keys.publicKey()
	now := time.Unix(1_800_000_000, 0)
	header, err := vapidAuthorization(key, "https://fcm.googleapis.com", now)
	if err != nil {
		t.Fatal(err)
	}
	token, k, ok := strings.Cut(strings.TrimPrefix(header, "vapid t="), ", k=")
	if !ok || !strings.HasPrefix(header, "vapid t=") || k != served {
		t.Fatalf("not vapid t=…, k=<the key served>: %s", header)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %s", token)
	}
	var head map[string]string
	var claims map[string]any
	if json.Unmarshal(mustB64(t, parts[0]), &head) != nil || json.Unmarshal(mustB64(t, parts[1]), &claims) != nil {
		t.Fatal("the JWT's parts aren't JSON")
	}
	if head["alg"] != "ES256" || head["typ"] != "JWT" {
		t.Errorf("header: %v", head)
	}
	if claims["aud"] != "https://fcm.googleapis.com" || claims["sub"] != vapidSubject {
		t.Errorf("claims: %v", claims)
	}
	if exp := int64(claims["exp"].(float64)); exp <= now.Unix() || exp > now.Add(12*time.Hour).Unix() {
		t.Errorf("exp %d isn't within 12 hours of %d", exp, now.Unix())
	}
	public, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), mustB64(t, k))
	if err != nil {
		t.Fatal(err)
	}
	signature := mustB64(t, parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])
	if len(signature) != 64 || !ecdsa.Verify(public, digest[:], r, s) {
		t.Fatal("the token doesn't verify with the public key")
	}
	other, _ := ecdsa.GenerateKey(elliptic.P256(), nil)
	if ecdsa.Verify(&other.PublicKey, digest[:], r, s) {
		t.Fatal("the token verifies with another key")
	}
}

// The notification's JSON keys, pinned: event, desk and floor, nothing else.
func TestPushNoteKeysArePinned(t *testing.T) {
	data, err := json.Marshal(newPushNote("waiting", "  widgets ", 2))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"event":"waiting","desk":"widgets","floor":2}` {
		t.Fatalf("the note is %s", data)
	}
	var fields []string
	for f := range reflect.TypeFor[pushNote]().Fields() {
		fields = append(fields, f.Tag.Get("json"))
	}
	if !slices.Equal(fields, []string{"event", "desk", "floor"}) {
		t.Fatalf("pushNote's fields: %v", fields)
	}
	if !slices.Equal(pushEvents, []string{"waiting", "done", "error"}) {
		t.Fatalf("the events: %v", pushEvents)
	}
	long := newPushNote("done", strings.Repeat("é", 100), 0)
	if !long.valid() || len([]rune(long.Desk)) != pushDeskMost {
		t.Errorf("a long name isn't cut to %d: %d", pushDeskMost, len([]rune(long.Desk)))
	}
	for _, bad := range []pushNote{
		{Event: "started", Desk: "widgets"}, {Event: "done", Desk: ""}, {Event: "done", Desk: "a\nb"},
		{Event: "done", Desk: "a\u202eb"}, {Event: "done", Desk: "a\u2066b"}, {Event: "done", Desk: "a\u2028b"},
		{Event: "done", Desk: "a\u2029b"}, {Event: "done", Desk: "a\u200bb"}, {Event: "done", Desk: "a\ufeffb"},
		{Event: "done", Desk: "a\x7fb"}, {Event: "done", Desk: "a\u0085b"},
		{Event: "done", Desk: "widgets", Floor: -1}, {Event: "done", Desk: "widgets", Floor: floorCount},
		{Event: "done", Desk: strings.Repeat("x", pushDeskMost+1)},
	} {
		if bad.valid() {
			t.Errorf("%+v is valid", bad)
		}
	}
}

func TestPushEndpointOnlyHTTPSOnAPushHost(t *testing.T) {
	for _, ok := range []string{
		"https://fcm.googleapis.com/fcm/send/abc:def",
		"https://updates.push.services.mozilla.com/wpush/v2/gAAA",
		"https://web.push.apple.com/QGuQyavXut",
		"https://wns2-par02p.notify.windows.com/w/?token=BQYAAA",
		"https://fcm.googleapis.com:443/wp/abc",
	} {
		if _, err := pushEndpoint(ok); err != nil {
			t.Errorf("%s refused: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"http://fcm.googleapis.com/fcm/send/abc",
		"https://fcm.googleapis.com.evil.example/fcm/send/abc",
		"https://evil.example/fcm.googleapis.com",
		"https://notify.windows.com/w/",
		"https://a.b.notify.windows.com/w/",
		"https://evilnotify.windows.com/w/",
		"https://FCM.googleapis.com/x",
		"https://fcm.googleapis.com./x",
		"https://fcm.googleapis.com:8443/x",
		"https://user:pass@fcm.googleapis.com/x",
		"https://127.0.0.1/x",
		"https://localhost/x",
		"wss://web.push.apple.com/x",
		"https://web.push.apple.com/x#frag",
		"//web.push.apple.com/x",
		"",
		"https://web.push.apple.com/" + strings.Repeat("a", pushEndpointMost),
	} {
		if _, err := pushEndpoint(bad); !errors.Is(err, errPushEndpoint) {
			t.Errorf("%q taken: %v", bad, err)
		}
	}
}

// testPushBrowser is a browser's subscription keys, made for the test.
func testPushBrowser(t *testing.T, endpoint string) (pushSubscription, *ecdh.PrivateKey, []byte) {
	t.Helper()
	ua, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	auth := []byte("0123456789abcdef")
	var sub pushSubscription
	sub.Endpoint = endpoint
	sub.Keys.P256dh = b64.EncodeToString(ua.PublicKey().Bytes())
	sub.Keys.Auth = b64.EncodeToString(auth)
	return sub, ua, auth
}

// testPushSender sends to server whatever the push host asked, and counts the requests it gets.
func testPushSender(t *testing.T, server *httptest.Server, clock *time.Time) *pushSender {
	t.Helper()
	sender := newPushSender(newPushKeys((&memoryPushStore{}).store()))
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	sender.client.Transport = transport
	sender.now = func() time.Time { return *clock }
	return sender
}

// decryptPush is the browser's side (RFC 8291 §3.4 from the other end): the note in body, decrypted with its key
// ua and auth secret.
func decryptPush(t *testing.T, body []byte, ua *ecdh.PrivateKey, auth []byte) []byte {
	t.Helper()
	salt, idLen := body[:16], int(body[20])
	if rs := binary.BigEndian.Uint32(body[16:20]); rs != pushRecordSize {
		t.Fatalf("record size %d", rs)
	}
	asPublic := body[21 : 21+idLen]
	as, err := ecdh.P256().NewPublicKey(asPublic)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := ua.ECDH(as)
	if err != nil {
		t.Fatal(err)
	}
	ikm, _ := hkdf.Key(sha256.New, shared, auth, "WebPush: info\x00"+string(ua.PublicKey().Bytes())+string(asPublic), 32)
	prk, _ := hkdf.Extract(sha256.New, ikm, salt)
	cek, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, body[21+idLen:], nil)
	if err != nil {
		t.Fatalf("the browser can't decrypt it: %v", err)
	}
	if len(plain) == 0 || plain[len(plain)-1] != 2 {
		t.Fatal("no last-record delimiter")
	}
	return plain[:len(plain)-1]
}

func TestPushSendPostsAnEncryptedSignedNote(t *testing.T) {
	var got *http.Request
	var body []byte
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	now := time.Unix(1_800_000_000, 0)
	sender := testPushSender(t, server, &now)
	sub, ua, auth := testPushBrowser(t, "https://web.push.apple.com/QGuQyavXut")
	if err := sender.send(context.Background(), sub, newPushNote("waiting", "widgets", 1)); err != nil {
		t.Fatal(err)
	}
	if got.Host != "web.push.apple.com" || got.URL.Path != "/QGuQyavXut" || got.Method != http.MethodPost {
		t.Errorf("sent to %s %s%s", got.Method, got.Host, got.URL.Path)
	}
	for header, want := range map[string]string{"Content-Encoding": "aes128gcm", "TTL": "3600", "Urgency": "high"} {
		if got.Header.Get(header) != want {
			t.Errorf("%s: %q, want %q", header, got.Header.Get(header), want)
		}
	}
	if !strings.HasPrefix(got.Header.Get("Authorization"), "vapid t=") {
		t.Errorf("Authorization: %q", got.Header.Get("Authorization"))
	}
	if plain := decryptPush(t, body, ua, auth); string(plain) != `{"event":"waiting","desk":"widgets","floor":1}` {
		t.Errorf("the browser reads %s", plain)
	}
	if strings.Contains(string(body), "widgets") {
		t.Error("the desk's name went in the clear")
	}
}

func TestPushSendRefusesBeforeTheNetwork(t *testing.T) {
	var asked atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { asked.Add(1) }))
	defer server.Close()
	now := time.Unix(1_800_000_000, 0)
	sender := testPushSender(t, server, &now)
	for _, endpoint := range []string{"http://fcm.googleapis.com/x", "https://push.example.com/x", server.URL + "/x"} {
		sub, _, _ := testPushBrowser(t, endpoint)
		if err := sender.send(context.Background(), sub, newPushNote("done", "widgets", 0)); !errors.Is(err, errPushEndpoint) {
			t.Errorf("%s: %v", endpoint, err)
		}
	}
	sub, _, _ := testPushBrowser(t, "https://fcm.googleapis.com/x")
	if err := sender.send(context.Background(), sub, pushNote{Event: "prompt", Desk: "widgets"}); !errors.Is(err, errPushNote) {
		t.Errorf("a note that isn't one: %v", err)
	}
	sub.Keys.P256dh = "nope"
	if err := sender.send(context.Background(), sub, newPushNote("done", "widgets", 0)); !errors.Is(err, errPushSubscription) {
		t.Errorf("a subscription without a key: %v", err)
	}
	if asked.Load() != 0 {
		t.Fatalf("%d requests reached the network", asked.Load())
	}
}

func TestPushSendGoneAndBusy(t *testing.T) {
	var status atomic.Int32
	var asked atomic.Int32
	retry := ""
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		if retry != "" {
			w.Header().Set("Retry-After", retry)
		}
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(strings.Repeat("x", 1<<20)))
	}))
	defer server.Close()
	now := time.Unix(1_800_000_000, 0)
	sender := testPushSender(t, server, &now)
	note := newPushNote("error", "widgets", 0)
	google, _, _ := testPushBrowser(t, "https://fcm.googleapis.com/fcm/send/abc")
	mozilla, _, _ := testPushBrowser(t, "https://updates.push.services.mozilla.com/wpush/v2/abc")

	for _, gone := range []int32{404, 410} {
		status.Store(gone)
		if err := sender.send(context.Background(), google, note); !errors.Is(err, errPushGone) {
			t.Errorf("%d: %v, want gone", gone, err)
		}
	}

	// 429 with Retry-After: left alone that long, without a request; another push host still sent to.
	status.Store(429)
	retry = "120"
	var busy *pushBusy
	if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) || busy.until != now.Add(2*time.Minute) {
		t.Fatalf("429: %v", err)
	}
	before := asked.Load()
	now = now.Add(time.Minute)
	if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) || asked.Load() != before {
		t.Fatalf("sent during the pause: %v, %d requests", err, asked.Load()-before)
	}
	status.Store(201)
	if err := sender.send(context.Background(), mozilla, note); err != nil {
		t.Fatalf("another host: %v", err)
	}

	// Over once the pause ends and it takes one again; then a 5xx without Retry-After: the first pause, then twice
	// as long.
	now = now.Add(2 * time.Minute)
	if err := sender.send(context.Background(), google, note); err != nil {
		t.Fatalf("after the pause: %v", err)
	}
	status.Store(503)
	retry = ""
	if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) || busy.until != now.Add(pushBackoffFirst) {
		t.Fatalf("503: %v", err)
	}
	now = busy.until
	if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) || busy.until != now.Add(2*pushBackoffFirst) {
		t.Fatalf("503 again: %v", err)
	}
	// A Retry-After past the longest pause is held to it.
	now = busy.until
	retry = "999999"
	if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) || busy.until != now.Add(pushBackoffMost) {
		t.Fatalf("a long Retry-After: %v", err)
	}
	// Taken again once the pause is over; another 4xx is refused, not paused.
	now = busy.until
	status.Store(201)
	if err := sender.send(context.Background(), google, note); err != nil {
		t.Fatalf("after the pause: %v", err)
	}
	status.Store(400)
	var refused *pushRefused
	if err := sender.send(context.Background(), google, note); !errors.As(err, &refused) || refused.status != 400 {
		t.Fatalf("400: %v", err)
	}
	if err := sender.send(context.Background(), google, note); !errors.As(err, &refused) {
		t.Fatalf("paused after a 400: %v", err)
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	for value, want := range map[string]time.Duration{
		"0": 0, "90": 90 * time.Second, "Thu, 12 Nov 2026 10:05:00 GMT": 5 * time.Minute, "Thu, 12 Nov 2026 09:00:00 GMT": 0,
	} {
		if got, ok := retryAfter(value, now); !ok || got != want {
			t.Errorf("%q: %v %v, want %v", value, got, ok, want)
		}
	}
	for _, bad := range []string{"", "soon", "-5", "1.5"} {
		if _, ok := retryAfter(bad, now); ok {
			t.Errorf("%q taken", bad)
		}
	}
}

// A hostile Retry-After (0, a past date, a negative or malformed value) never makes the pause shorter than the
// sender's own; a huge one is held to the longest pause; and repeated 429s never turn into a busy loop (RT126W2 B1).
func TestPushRetryAfterNeverTurnsTheBackoffOff(t *testing.T) {
	var asked atomic.Int32
	retry := ""
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		if retry != "" {
			w.Header().Set("Retry-After", retry)
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	now := time.Unix(1_800_000_000, 0)
	past := now.Add(-time.Hour).UTC().Format(http.TimeFormat)
	google, _, _ := testPushBrowser(t, "https://fcm.googleapis.com/fcm/send/abc")
	note := newPushNote("waiting", "widgets", 0)

	for name, c := range map[string]struct {
		value string
		want  time.Duration
	}{
		"zero":      {"0", pushBackoffFirst},
		"past date": {past, pushBackoffFirst},
		"negative":  {"-5", pushBackoffFirst},
		"malformed": {"soon", pushBackoffFirst},
		"fraction":  {"1.5", pushBackoffFirst},
		"shorter":   {"5", pushBackoffFirst},
		"longer":    {"90", 90 * time.Second},
		"huge":      {"4294967295", pushBackoffMost},
		"too big":   {"99999999999999999999", pushBackoffFirst},
	} {
		t.Run(name, func(t *testing.T) {
			retry = c.value
			sender := testPushSender(t, server, &now)
			var busy *pushBusy
			if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) || busy.until != now.Add(c.want) {
				t.Fatalf("Retry-After %q: %v, want a pause of %v", c.value, err, c.want)
			}
			// Left alone for the whole pause: no request goes out until it ends.
			before := asked.Load()
			for range 5 {
				if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) {
					t.Fatalf("sent during the pause: %v", err)
				}
			}
			if asked.Load() != before {
				t.Fatalf("%d requests during the pause", asked.Load()-before)
			}
		})
	}

	// Repeated 429s with "Retry-After: 0", each sent the moment its pause ends: the pause doubles to the longest
	// and stays there, never 0, and each pause costs one request, not a loop.
	retry = "0"
	sender := testPushSender(t, server, &now)
	want := pushBackoffFirst
	for i := range 10 {
		before := asked.Load()
		var busy *pushBusy
		if err := sender.send(context.Background(), google, note); !errors.As(err, &busy) || busy.until != now.Add(want) {
			t.Fatalf("429 #%d: %v, want a pause of %v", i, err, want)
		}
		for range 3 {
			_ = sender.send(context.Background(), google, note)
		}
		if got := asked.Load() - before; got != 1 {
			t.Fatalf("429 #%d: %d requests in the pause, want 1", i, got)
		}
		now = busy.until
		want = min(2*want, pushBackoffMost)
	}
	if want != pushBackoffMost {
		t.Fatalf("the pause ended at %v, want %v", want, pushBackoffMost)
	}
}

// The sender's own client takes no proxy from the environment and keeps an answer's headers short.
func TestPushSenderClientLimits(t *testing.T) {
	transport, ok := newPushSender(newPushKeys((&memoryPushStore{}).store())).client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.MaxResponseHeaderBytes != pushHeadersMost {
		t.Fatalf("the client's transport: %+v", transport)
	}
}
