package desktop

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// PUSH1: a phone's subscription is a sealed record, the phone's own, and the key it's made for reaches the core
// only over the Mac app's bridge. Fake endpoints and fake keys only: nothing here is sent anywhere.

// fakeSubscription is a browser's subscription, as its toJSON() gives it, to a push service at endpoint: a P-256
// key made now, and a 16-byte auth secret.
func fakeSubscription(t *testing.T, endpoint string) string {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, pushSubscriptionAuth)
	_, _ = rand.Read(auth)
	body, _ := json.Marshal(map[string]any{"endpoint": endpoint, "expirationTime": nil,
		"keys": map[string]string{"p256dh": b64.EncodeToString(key.PublicKey().Bytes()), "auth": b64.EncodeToString(auth)}})
	return string(body)
}

// pushAsk asks a push route as the phone holding phoneKey, over Tailscale's WireGuard address (as probeRoute's
// phone-wireguard), with body.
func pushAsk(t *testing.T, d *Deck, method, path, body, phoneKey string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host, r.RemoteAddr = "100.64.1.2:7710", "100.101.102.103:50000"
	r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("100.64.1.2"), Port: 7710}))
	r.Header.Set(hookKeyHeader, phoneKey)
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	return w
}

// pushTurnedOn is what GET /push tells the phone holding phoneKey: whether it has a subscription kept, and the key.
func pushTurnedOn(t *testing.T, d *Deck, phoneKey string) (bool, string) {
	t.Helper()
	w := pushAsk(t, d, http.MethodGet, "/push", "", phoneKey)
	var answer struct {
		Key string `json:"key"`
		On  bool   `json:"on"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &answer) != nil {
		t.Fatalf("GET /push: %d %s", w.Code, w.Body)
	}
	return answer.On, answer.Key
}

// heldPush is the subscription the deck holds for the phone holding phoneKey: its endpoint, "" for none.
func heldPush(d *Deck, phoneKey string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if phone := d.access.Phones.WithKey(phoneKey); phone != nil && phone.Push != nil {
		return phone.Push.Endpoint
	}
	return ""
}

// On a Mac the subscription is the posting phone's alone, in the sealed phones.json, and outlasts a restart; another
// phone neither sees it, nor turns it off, nor takes or changes it by posting its address. The key it's made for is the Mac app's item, asked over the bridge only.
func TestOnAMacAPushSubscriptionIsThePhonesThatPostedIt(t *testing.T) {
	d, mac, phoneA, _ := macDeck(t)
	d.mu.Lock()
	phoneB := d.access.Phones.Pair("iPad", true)
	d.mu.Unlock()
	const endpoint = "https://web.push.apple.com/FAKE-endpoint-a"

	on, key := pushTurnedOn(t, d, phoneA)
	if on || key == "" {
		t.Fatalf("before any subscription: on=%v key=%q", on, key)
	}
	// The key is the Mac app's push-key item, made and kept through the bridge: never a file here.
	mac.mu.Lock()
	kept := mac.secrets[pushKeyItem]
	var items []string
	for _, asked := range mac.asked {
		if strings.HasPrefix(asked.Action, "secret-") {
			items = append(items, asked.Action+" "+asked.Item)
		}
	}
	mac.mu.Unlock()
	if !slices.Contains(items, "secret-read "+pushKeyItem) || !slices.Contains(items, "secret-write "+pushKeyItem) {
		t.Errorf("the push key wasn't asked of the Mac app: %q", items)
	}
	if public, err := newPushKeys(gmTokenStore{read: func() (string, error) { return kept, nil }}).publicKey(); err != nil || public != key {
		t.Errorf("the key served isn't the Mac app's item: %v", err)
	}
	if _, err := os.Lstat(keptPushKeyPath()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a Mac kept the push key in a file: %v", err)
	}

	ownBody := fakeSubscription(t, endpoint)
	if w := pushAsk(t, d, http.MethodPost, "/push", ownBody, phoneA); w.Code != http.StatusNoContent {
		t.Fatalf("POST /push: %d %s", w.Code, w.Body)
	}
	if on, _ := pushTurnedOn(t, d, phoneA); !on {
		t.Error("the phone that posted it has none")
	}
	if on, _ := pushTurnedOn(t, d, phoneB); on {
		t.Error("another phone has the subscription")
	}
	if w := pushAsk(t, d, http.MethodPost, "/push/off", "", phoneB); w.Code != http.StatusNoContent {
		t.Fatalf("POST /push/off: %d", w.Code)
	}
	if heldPush(d, phoneA) != endpoint {
		t.Error("another phone turned it off")
	}
	// Sealed, in the phone's own entry.
	data, _ := os.ReadFile(phonesFile)
	if seal, ok := sealOfFile("phones.json", data); !ok || !seal.fits(recordKey()) || !bytes.Contains(seal.body, []byte(endpoint)) {
		t.Errorf("the subscription isn't in the sealed phones.json: %s", data)
	}
	again := restart(t)
	if heldPush(again, phoneA) != endpoint || heldPush(again, phoneB) != "" {
		t.Error("the subscription didn't outlast a restart on its own phone")
	}
	// Another phone posting the same address, however it comes by it, is refused in words, and the phone's
	// subscription stays as it was, written nowhere (RT126PUSH1 B1). The second row is the reviewer's probe: keys
	// the other phone chose, which would have the phone's browser drop every notification unseen.
	before, _ := os.ReadFile(phonesFile)
	for _, row := range []struct{ name, body string }{
		{"the same browser, from another pairing", ownBody},
		{"its address, with keys the other phone chose", fakeSubscription(t, endpoint)},
		{"its address, with the port written", fakeSubscription(t, "https://web.push.apple.com:443/FAKE-endpoint-a")},
		{"its address, with the path encoded", fakeSubscription(t, "https://web.push.apple.com/FAKE-endpoint-%61")},
	} {
		w := pushAsk(t, again, http.MethodPost, "/push", row.body, phoneB)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), pushOtherPhones) {
			t.Errorf("%s: %d %q, want 409 and its words", row.name, w.Code, w.Body)
		}
		if heldPush(again, phoneA) != endpoint || heldPush(again, phoneB) != "" {
			t.Errorf("%s: the phone's subscription was changed or moved", row.name)
		}
		if after, _ := os.ReadFile(phonesFile); !bytes.Equal(after, before) {
			t.Errorf("%s: phones.json was written", row.name)
		}
	}
	// The phone itself posting it again keeps it, and writes nothing.
	if w := pushAsk(t, again, http.MethodPost, "/push", ownBody, phoneA); w.Code != http.StatusNoContent {
		t.Fatalf("POST /push again from its own phone: %d %s", w.Code, w.Body)
	}
	if after, _ := os.ReadFile(phonesFile); heldPush(again, phoneA) != endpoint || !bytes.Equal(after, before) {
		t.Error("the same subscription posted again changed what's kept")
	}
}

// Unpairing the phone drops its subscription, and so does turning it off, or posting another in its place, each
// under a new record key: a copy of phones.json from before doesn't fit at the next start, so nothing brings the
// subscription back.
func TestOnAMacAPushSubscriptionGoesWithItsPhone(t *testing.T) {
	for _, how := range []string{"unpaired", "turned off", "replaced"} {
		t.Run(how, func(t *testing.T) {
			d, mac, phoneKey, _ := macDeck(t)
			const endpoint = "https://fcm.googleapis.com/fcm/send/FAKE-endpoint"
			if w := pushAsk(t, d, http.MethodPost, "/push", fakeSubscription(t, endpoint), phoneKey); w.Code != http.StatusNoContent {
				t.Fatalf("POST /push: %d %s", w.Code, w.Body)
			}
			before := sealedCopies(t)
			itemBefore := mac.recordItems[0].ID
			const replacement = "https://fcm.googleapis.com/fcm/send/FAKE-another"
			switch how {
			case "unpaired":
				d.mu.Lock()
				d.unpair(d.access.Phones.WithKey(phoneKey).ID)
				d.mu.Unlock()
			case "turned off":
				if w := pushAsk(t, d, http.MethodPost, "/push/off", "", phoneKey); w.Code != http.StatusNoContent {
					t.Fatalf("POST /push/off: %d", w.Code)
				}
			case "replaced":
				if w := pushAsk(t, d, http.MethodPost, "/push", fakeSubscription(t, replacement), phoneKey); w.Code != http.StatusNoContent {
					t.Fatalf("POST /push in its place: %d %s", w.Code, w.Body)
				}
			}
			if held := heldPush(d, phoneKey); held != "" && (how != "replaced" || held != replacement) {
				t.Fatalf("still held: %q", held)
			}
			if len(mac.recordItems) != 1 || mac.recordItems[0].ID == itemBefore {
				t.Fatalf("the record key didn't rotate: %+v", mac.recordItems)
			}
			if data, _ := os.ReadFile(phonesFile); bytes.Contains(data, []byte(endpoint)) {
				t.Error("phones.json still holds the subscription")
			}
			putBack(t, before)
			if heldPush(restart(t), phoneKey) != "" {
				t.Error("a copy from before brought the subscription back")
			}
		})
	}
}

// RT126PUSH1 N2: while the records' key can't be read no phone is let in, so no route reaches subscribePush; asked
// anyway, it refuses with Setup's words and keeps nothing.
func TestNoPushSubscriptionIsKeptWhileTheRecordsKeyCantBeRead(t *testing.T) {
	d, _, phoneKey, _ := macDeck(t)
	d.mu.Lock()
	id := d.access.Phones.WithKey(phoneKey).ID
	d.mu.Unlock()
	records.mu.Lock()
	was := records.state
	records.state = recordsUnreadable
	records.mu.Unlock()
	t.Cleanup(func() { records.mu.Lock(); records.state = was; records.mu.Unlock() })
	before, _ := os.ReadFile(phonesFile)
	status, said := d.subscribePush(id, strings.NewReader(fakeSubscription(t, "https://web.push.apple.com/FAKE-held")))
	if status != http.StatusConflict || said != recordsHeldAnswer {
		t.Errorf("while the key can't be read: %d %q", status, said)
	}
	if after, _ := os.ReadFile(phonesFile); heldPush(d, phoneKey) != "" || !bytes.Equal(after, before) {
		t.Error("a subscription was kept while the key can't be read")
	}
}

// A phones.json another program changed, a subscription put in or moved, counts for nothing: no phone, so no
// subscription, is let in at the next start.
func TestOnAMacAPlantedPushSubscriptionCountsForNothing(t *testing.T) {
	d, _, phoneKey, _ := macDeck(t)
	d.mu.Lock()
	other := d.access.Phones.Pair("iPad", true)
	d.mu.Unlock()
	if w := pushAsk(t, d, http.MethodPost, "/push", fakeSubscription(t, "https://web.push.apple.com/FAKE-owner"), phoneKey); w.Code != http.StatusNoContent {
		t.Fatalf("POST /push: %d", w.Code)
	}
	said := recordWords(t)
	var planted map[string]any
	_ = json.Unmarshal([]byte(fakeSubscription(t, "https://web.push.apple.com/FAKE-planted")), &planted)
	rewrite(t, "phones.json", func(f map[string]any) {
		for _, entry := range f["body"].([]any) {
			entry.(map[string]any)["push"] = planted
		}
	})
	again := restart(t)
	if heldPush(again, phoneKey) != "" || heldPush(again, other) != "" {
		t.Error("a planted subscription counts")
	}
	if len(*said) == 0 {
		t.Error("what was left out wasn't said")
	}
}

// Only a browser's subscription to a push service this app sends to is kept: Apple's, Google's, Mozilla's, and
// Microsoft's (the owner's yes, F4). Anything else is a 400 and nothing kept; off a Mac, where phones.json counts as
// found, one in the file that isn't usable is left out, the phone kept.
func TestOnlyABrowsersSubscriptionToAPushServiceIsKept(t *testing.T) {
	d, phoneKey := probeDeck(t)
	good := []string{"https://web.push.apple.com/FAKE", "https://fcm.googleapis.com/fcm/send/FAKE",
		"https://updates.push.services.mozilla.com/wpush/v2/FAKE", "https://wns2-par02p.notify.windows.com/w/?token=FAKE"}
	for _, endpoint := range good {
		if w := pushAsk(t, d, http.MethodPost, "/push", fakeSubscription(t, endpoint), phoneKey); w.Code != http.StatusNoContent {
			t.Errorf("%s: %d %s", endpoint, w.Code, w.Body)
		}
		if heldPush(d, phoneKey) != endpoint {
			t.Errorf("%s isn't held", endpoint)
		}
	}
	valid := fakeSubscription(t, good[0])
	var sub map[string]any
	_ = json.Unmarshal([]byte(valid), &sub)
	keys := sub["keys"].(map[string]any)
	with := func(change func(m map[string]any, k map[string]any)) string {
		m := map[string]any{"endpoint": sub["endpoint"]}
		k := map[string]any{"p256dh": keys["p256dh"], "auth": keys["auth"]}
		change(m, k)
		m["keys"] = k
		out, _ := json.Marshal(m)
		return string(out)
	}
	offCurve := make([]byte, 65)
	offCurve[0] = 4
	for name, body := range map[string]string{
		"not json":              "{",
		"empty":                 "{}",
		"http":                  with(func(m, _ map[string]any) { m["endpoint"] = "http://web.push.apple.com/FAKE" }),
		"another host":          with(func(m, _ map[string]any) { m["endpoint"] = "https://push.example.com/FAKE" }),
		"a look-alike host":     with(func(m, _ map[string]any) { m["endpoint"] = "https://web.push.apple.com.example.com/FAKE" }),
		"a port":                with(func(m, _ map[string]any) { m["endpoint"] = "https://web.push.apple.com:8443/FAKE" }),
		"a short key":           with(func(_, k map[string]any) { k["p256dh"] = b64.EncodeToString(offCurve[:33]) }),
		"a key off the curve":   with(func(_, k map[string]any) { k["p256dh"] = b64.EncodeToString(offCurve) }),
		"an auth of 15 bytes":   with(func(_, k map[string]any) { k["auth"] = b64.EncodeToString(make([]byte, 15)) }),
		"an auth not base64url": with(func(_, k map[string]any) { k["auth"] = "!!!!!!!!!!!!!!!!!!!!!!" }),
	} {
		if w := pushAsk(t, d, http.MethodPost, "/push", body, phoneKey); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", name, w.Code)
		}
	}
	if heldPush(d, phoneKey) != good[len(good)-1] {
		t.Error("a refused subscription replaced the one kept")
	}
	// A padded key, as some browsers write base64url, is the same key, kept without its padding.
	padded := with(func(_, k map[string]any) { k["auth"] = k["auth"].(string) + "==" })
	if w := pushAsk(t, d, http.MethodPost, "/push", padded, phoneKey); w.Code != http.StatusNoContent {
		t.Errorf("padded: %d", w.Code)
	}

	// Off a Mac the file counts as found, but a subscription in it this app wouldn't keep is left out.
	if recordsState() != recordsOff {
		t.Fatalf("the records' state: %q", recordsState())
	}
	d.mu.Lock()
	phone := d.access.Phones.WithKey(phoneKey)
	phone.Push = &pushSubscription{Endpoint: "https://push.example.com/FAKE"}
	d.access.Phones.save()
	d.mu.Unlock()
	changeKeysOnce() // as the next start finds the folder: every key changed once already
	if loaded := loadPhones(); len(loaded.All) != 1 || loaded.All[0].Push != nil {
		t.Errorf("an unusable subscription in phones.json: %+v", loaded.All)
	}
}

// Every push route is a paired phone's over Tailscale: from the Wi-Fi it's refused with its own words, and so is the
// office's window; and through `tailscale serve` with the phone's typing off, as every phone route (F3).
func TestThePushRoutesAreTheTailnetsAndNeedTheTypingSwitch(t *testing.T) {
	d, phoneKey := probeDeck(t)
	byName := func(name string) routeCaller {
		return routeCallers[slices.IndexFunc(routeCallers, func(c routeCaller) bool { return c.name == name })]
	}
	for _, route := range [][2]string{{"GET", "/push"}, {"POST", "/push"}, {"POST", "/push/off"}} {
		for _, name := range []string{"phone-wifi", "window"} {
			if status, text := probeRoute(t, d, route[0], route[1], phoneKey, byName(name)); status != http.StatusForbidden || text != pushTailnet {
				t.Errorf("%s %s as %s: %d %q", route[0], route[1], name, status, text)
			}
		}
		if status, _ := probeRoute(t, d, route[0], route[1], phoneKey, byName("phone-serve-mac-text-off")); status != http.StatusForbidden {
			t.Errorf("%s %s through serve with typing off: %d", route[0], route[1], status)
		}
	}
	// In a test there's nowhere to keep the key: none is made, and the page is told so.
	if status, text := probeRoute(t, d, "GET", "/push", phoneKey, byName("phone-wireguard")); status != http.StatusServiceUnavailable || text != pushKeyMissing {
		t.Errorf("GET /push with no store: %d %q", status, text)
	}
	if _, err := os.Lstat(filepath.Join(supportDir, keptPushKeyName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a test kept a push key: %v", err)
	}
}

// Through `tailscale serve` on Windows, every push route is refused, its wrong methods too, with its own words: a hire
// can change serve there (SIERRA's ruling (b)). The same hand-on on a Mac and on Linux still gets through, and so does
// WireGuard on Windows; and Windows serve still reaches a phone's other routes over Tailscale (/approvals, /peek).
func TestWindowsServeIsRefusedOnThePushRoutes(t *testing.T) {
	d, phoneKey := probeDeck(t)
	byName := func(name string) routeCaller {
		return routeCallers[slices.IndexFunc(routeCallers, func(c routeCaller) bool { return c.name == name })]
	}
	windows := byName("phone-serve-windows")
	if windows.mac || windows.linux || windows.off || !windows.tailscale {
		t.Fatal("phone-serve-windows isn't Windows' hand-on through `tailscale serve`")
	}
	for _, route := range [][2]string{{"GET", "/push"}, {"POST", "/push"}, {"POST", "/push/off"}, {"PUT", "/push"}, {"GET", "/push/off"}} {
		if status, text := probeRoute(t, d, route[0], route[1], phoneKey, windows); status != http.StatusForbidden || text != pushNotWindowsServe {
			t.Errorf("%s %s through Windows serve: %d %q, want 403 %q", route[0], route[1], status, text, pushNotWindowsServe)
		}
		for _, name := range []string{"phone-serve-mac", "phone-serve-linux", "phone-wireguard"} {
			if status, text := probeRoute(t, d, route[0], route[1], phoneKey, byName(name)); !letIn(status) {
				t.Errorf("%s %s as %s: %d %q, want it let in", route[0], route[1], name, status, text)
			}
		}
	}
	for _, route := range [][2]string{{"GET", "/approvals"}, {"POST", "/peek"}} {
		if status, text := probeRoute(t, d, route[0], route[1], phoneKey, windows); !letIn(status) {
			t.Errorf("%s %s through Windows serve: %d %q, want it let in", route[0], route[1], status, text)
		}
	}
}

// pushSecrets is a stand-in secret store whose get answers getErr, and counts what's set.
type pushSecrets struct {
	getErr error
	set    []string
}

func (s *pushSecrets) store() secretStore {
	return secretStore{
		get:    func() (string, error) { return "", s.getErr },
		set:    func(v string) error { s.set = append(s.set, v); return nil },
		delete: func() error { return nil },
	}
}

// RT126W2 N3: off a Mac only the secret store's "not found" is "no push key". Locked, unreachable, no session bus:
// the read fails and no key is made, so none is kept over the one the store may hold. The GM's token reads as
// before: any error is none (gmtokenstore.go).
func TestOnlyNotFoundReadsAsNoPushKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), keptPushKeyName)
	for _, err := range []error{errors.New("the keyring is locked"), errors.New("no session bus"), fmt.Errorf("dbus: %w", errors.New("no reply"))} {
		secrets := &pushSecrets{getErr: err}
		if _, got := newPushKeys(storeOrFileWhere(secrets.store(), func() string { return path }, pushKeyNone)).key(); got == nil {
			t.Errorf("%v: a key was made", err)
		}
		if len(secrets.set) != 0 {
			t.Errorf("%v: a key was kept over the store's", err)
		}
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("%v: a key was kept in the file", err)
		}
		if token, got := storeOrFile(secrets.store(), func() string { return path }).read(); token != "" || got != nil {
			t.Errorf("%v: the GM's token read %q, %v; want none, as before", err, token, got)
		}
	}
	for _, err := range []error{keyring.ErrNotFound, fmt.Errorf("wrapped: %w", keyring.ErrNotFound)} {
		secrets := &pushSecrets{getErr: err}
		if _, got := newPushKeys(storeOrFileWhere(secrets.store(), func() string { return path }, pushKeyNone)).key(); got != nil || len(secrets.set) != 1 {
			t.Errorf("%v: %v, %d kept; want a new key kept in the store", err, got, len(secrets.set))
		}
	}
}

// On a Mac the push key is one of the app's own items (bridgeSecrets), and without the app there's no store at all:
// no file, no Keychain asked by this program. An item the Keychain couldn't read is an error, never "none".
func TestThePushKeyReachesTheCoreOnlyOverTheBridge(t *testing.T) {
	if !slices.Contains(bridgeSecrets, pushKeyItem) || pushKeyItem != "push-key" {
		t.Errorf("push-key isn't one of the app's items: %q", bridgeSecrets)
	}
	isolate(t)
	if runtime.GOOS == "darwin" {
		if _, err := newPushKeys(pushKeyStoreFor(nil)).key(); !errors.Is(err, errNoPushKeyStore) {
			t.Errorf("a Mac without the app: %v", err)
		}
	}
	mac := &macStandIn{unreadable: []string{pushKeyItem}, secrets: map[string]string{pushKeyItem: "FAKE-kept"}}
	if _, err := newPushKeys(pushKeyStoreFor(mac)).key(); err == nil || mac.secrets[pushKeyItem] != "FAKE-kept" {
		t.Errorf("an item that couldn't be read: %v, kept %q", err, mac.secrets[pushKeyItem])
	}
	failed := &macStandIn{lookFailed: map[string]bool{pushKeyItem: true}, secrets: map[string]string{pushKeyItem: "FAKE-kept"}}
	if _, err := newPushKeys(pushKeyStoreFor(failed)).key(); !errors.Is(err, errPushKeyUnreadable) || failed.secrets[pushKeyItem] != "FAKE-kept" {
		t.Errorf("an item the Keychain has but can't read: %v", err)
	}
	if _, err := os.Lstat(keptPushKeyPath()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the push key went in a file: %v", err)
	}
}
