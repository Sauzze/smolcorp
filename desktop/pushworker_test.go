package desktop

import (
	"encoding/json"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	agentdeck "github.com/Sauzze/smolcorp"
)

// The phone page's notifications worker and manifest (1.26 PUSH2): served before the key check, with no key, no
// state and nothing per phone; the worker holds no key; the page's key to turn notifications on shows only in a
// secure context, and on an iPhone or iPad only from the Home Screen.

// pushFiles are the two files, each with the type a browser takes it as only.
var pushFiles = map[string]string{
	"/sw.js":                "text/javascript; charset=utf-8",
	"/manifest.webmanifest": "application/manifest+json",
}

// Every caller who reaches the files gets the same bytes, the app's own, of their type, with nothing set: no cookie,
// nothing kept by the browser, no frame header a page would need.
func TestTheWorkerAndManifestAreTheSameFilesForEveryone(t *testing.T) {
	d, phoneKey := probeDeck(t)
	callers := map[string]routeCaller{}
	for _, c := range routeCallers {
		callers[c.name] = c
	}
	for path, kind := range pushFiles {
		want, err := fs.ReadFile(agentdeck.Files, "Resources/web"+path)
		if err != nil || len(want) == 0 {
			t.Fatalf("%s isn't built into the app: %v", path, err)
		}
		for _, name := range []string{"stranger", "phone-wifi", "phone-wireguard", "phone-serve-mac", "phone-serve-linux", "phone-here", "window"} {
			w := probeAnswer(t, d, http.MethodGet, path, phoneKey, callers[name])
			if w.Code != http.StatusOK || w.Body.String() != string(want) {
				t.Errorf("%s as %s: %d, %d bytes, want 200 and the app's %d", path, name, w.Code, w.Body.Len(), len(want))
			}
			if got := w.Header().Get("Content-Type"); got != kind {
				t.Errorf("%s as %s: Content-Type %q, want %q", path, name, got, kind)
			}
			if w.Header().Get("Set-Cookie") != "" || w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s as %s: Set-Cookie %q, Cache-Control %q", path, name, w.Header().Get("Set-Cookie"), w.Header().Get("Cache-Control"))
			}
		}
	}
}

// Served before any key is looked at: a phone's key sent with them is never read, so the phone isn't marked seen,
// as the same key on GET /paired marks it.
func TestTheWorkerAndManifestAreServedBeforeTheKeyCheck(t *testing.T) {
	d, phoneKey := probeDeck(t)
	here := routeCallers[slices.IndexFunc(routeCallers, func(c routeCaller) bool { return c.name == "phone-here" })]
	long := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	seen := func() time.Time {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.access.Phones.All[0].LastSeen
	}
	d.mu.Lock()
	d.access.Phones.All[0].LastSeen = long
	d.mu.Unlock()
	for path := range pushFiles {
		if w := probeAnswer(t, d, http.MethodGet, path, phoneKey, here); w.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, w.Code)
		}
		if !seen().Equal(long) {
			t.Errorf("%s read the phone's key: the phone was marked seen", path)
		}
	}
	if w := probeAnswer(t, d, http.MethodGet, "/paired", phoneKey, here); w.Code != http.StatusNoContent || seen().Equal(long) {
		t.Fatalf("the stand-in doesn't work: GET /paired %d, the phone not marked seen", w.Code)
	}
}

// The manifest is the app's name, look and scope, and nothing else: no start address, so a Home Screen app opens at
// the address it was added from, #k= with it (Q-F5 (a)); and an id of its own, which would otherwise be that address.
func TestTheManifestNamesNoStartAddress(t *testing.T) {
	data, err := fs.ReadFile(agentdeck.Files, "Resources/web/manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("the manifest isn't JSON: %v", err)
	}
	keys := slices.Sorted(maps.Keys(manifest))
	if want := []string{"background_color", "display", "icons", "id", "name", "scope", "short_name", "theme_color"}; !slices.Equal(keys, want) {
		t.Errorf("the manifest's keys are %v, want %v", keys, want)
	}
	if manifest["id"] != "/" || manifest["scope"] != "/" || manifest["display"] != "standalone" {
		t.Errorf("id %v, scope %v, display %v", manifest["id"], manifest["scope"], manifest["display"])
	}
}

// pushWorkerSource is sw.js as the app has it.
func pushWorkerSource(t *testing.T) string {
	t.Helper()
	data, err := fs.ReadFile(agentdeck.Files, "Resources/web/sw.js")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The worker reaches nothing a key could be in, and asks nothing of anyone: its code (its comments aside) names none
// of the ways a worker reads an address, a store or the network, or opens or messages a page (PUSH4's to add).
func TestTheWorkerHoldsNoKey(t *testing.T) {
	code := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`).ReplaceAllString(pushWorkerSource(t), "")
	for _, word := range []string{"fetch", "location", "XMLHttpRequest", "WebSocket", "EventSource", "sendBeacon",
		"importScripts", "import", "indexedDB", "caches", "localStorage", "cookie", "openWindow", "postMessage",
		"onmessage", "matchAll", "clients.get", "#k", "k="} {
		if strings.Contains(code, word) {
			t.Errorf("sw.js has %q", word)
		}
	}
}

// Run in node, on a stand-in for a worker's self: the worker listens for install, activate and push alone (no fetch,
// so it answers no request; no message, so no page hands it anything), and what a push shows is its event word, desk
// and floor, each checked, and nothing else it brought. Without node the test is skipped.
func TestTheWorkerShowsOnlyAPushNotesThreeFields(t *testing.T) {
	runNode(t, `
const listeners = {}, shown = [];
const self = {
  addEventListener(type, fn) { if (listeners[type]) throw new Error('two ' + type + ' listeners'); listeners[type] = fn; },
  skipWaiting() {}, clients: { claim: async () => {} },
  registration: { showNotification: async (title, options) => { shown.push([title, options]); } },
};
`+pushWorkerSource(t)+`
const same = (got, want, what) => { if (JSON.stringify(got) !== JSON.stringify(want)) throw new Error(what + ': got ' + JSON.stringify(got) + ', want ' + JSON.stringify(want)); };
same(Object.keys(listeners).sort(), ['activate', 'install', 'push'], 'the listeners');
const push = (data, raw) => {
  shown.length = 0;
  listeners.push({ data: data === undefined ? null : { json: () => (raw ? JSON.parse(raw) : data) }, waitUntil() {} });
  return shown[0];
};
const generic = ['Smolcorp', { body: 'Something in the office needs a look.', icon: '/icon.png' }];
same(push({ event: 'waiting', desk: 'Ada', floor: 0 }), ['Smolcorp', { body: 'Ada needs you', icon: '/icon.png' }], 'waiting');
same(push({ event: 'done', desk: ' Ada ', floor: 2 }), ['Smolcorp', { body: 'Ada is done · floor 3', icon: '/icon.png' }], 'done upstairs');
same(push({ event: 'error', desk: 'Ada', floor: 9 }), ['Smolcorp', { body: 'Ada stopped with an error', icon: '/icon.png' }], 'a floor not in the building');
// Anything else a push brought is never shown.
const extra = push({ event: 'waiting', desk: 'Ada', floor: 1, prompt: 'PROMPT-X', folder: '/Users/x/secret', branch: 'BRANCH-X', diff: 'DIFF-X' });
if (/PROMPT-X|secret|BRANCH-X|DIFF-X/.test(JSON.stringify(extra))) throw new Error('shown: ' + JSON.stringify(extra));
same(extra, ['Smolcorp', { body: 'Ada needs you · floor 2', icon: '/icon.png' }], 'extra fields');
// One that isn't ours still shows a notification, with words of its own.
same(push({ event: 'toString', desk: 'Ada' }), generic, 'an event word that is no event');
same(push({ event: 'waiting', desk: { toString: () => 'Ada' } }), generic, 'a desk that is no text');
same(push({ event: 'waiting', desk: '  ' }), generic, 'no desk');
same(push(undefined), generic, 'no data');
same(push(null, '{not json'), generic, 'not JSON');
same(push('waiting'), generic, 'not an object');
// A desk's name is cut to 64 characters, as push.go's pushDeskMost.
same(push({ event: 'done', desk: '😀'.repeat(70), floor: 0 })[1].body, '😀'.repeat(64) + ' is done', 'a long name');
`)
}

// pagePiece is the part of page from start to the first end after it.
func pagePiece(t *testing.T, page, start, end string) string {
	t.Helper()
	at := strings.Index(page, start)
	if at < 0 {
		t.Fatalf("deck.html no longer has %q: update this test", start)
	}
	stop := strings.Index(page[at:], end)
	if stop < 0 {
		t.Fatalf("deck.html's %q no longer ends with %q: update this test", start, end)
	}
	return page[at : at+stop+len(end)]
}

// Run in node on deck.html's own pushShown and pushWhyNot: the key to turn notifications on shows only in a secure
// context with the push API, and on an iPhone or iPad only from the Home Screen, even where its Safari has the API in
// a tab. The page registers the worker by its bare address, only where the key shows, on a paired phone's page
// alone; it links the manifest on an iPhone or iPad alone; and nothing it hands the worker has the key. Without
// node, the node half is skipped.
func TestTheTurnOnKeyShowsOnlyInASecureContext(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "Resources", "web", "deck.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(data)
	block := pagePiece(t, page, "/* ------------------------------------------------------------ 🔔 notifications", "\n  function pushSameBytes(")
	for _, want := range []string{
		"const pushPage = !preview && !whole && !!phoneKey;",
		"const pushWorker = pushPage && pushShown() ? navigator.serviceWorker.register('/sw.js', { scope: '/', updateViaCache: 'none' })",
		"if (pushPage && pushApple()) {\n    document.head.append(Object.assign(document.createElement('link'), { rel: 'manifest', href: '/manifest.webmanifest' }));",
		"  function pushRows(row, button, rows) {\n    if (!pushPage) return;",
		"    if (!pushShown()) {",
		"    pushRows(row, button, rows);",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("deck.html lost %q", want)
		}
	}
	if strings.Count(page, "serviceWorker.register(") != 1 || strings.Contains(page, `rel="manifest"`) {
		t.Error("deck.html registers a worker, or links the manifest, outside the notifications' block")
	}
	if strings.Count(block, "phoneKey") != 1 || strings.Contains(block, "postMessage") || strings.Contains(block, "controller") {
		t.Error("the notifications' block hands something to the worker, or reads the key past pushPage")
	}
	script := pagePiece(t, page, "  function pushApple(", "\n  }\n") + pagePiece(t, page, "  function pushShown(", "\n  }\n") +
		pagePiece(t, page, "  function pushWhyNot(", "\n  }\n") + `
const api = { isSecureContext: true, PushManager() {}, Notification() {} };
const browser = { serviceWorker: {}, maxTouchPoints: 0 };
const iphone = (standalone, push = standalone) => [push ? api : { isSecureContext: true, Notification() {} }, { ...browser, maxTouchPoints: 5, standalone }];
const cases = [
  ['over the Wi-Fi\'s plain http', [{ ...api, isSecureContext: false }, { ...browser, maxTouchPoints: 5 }], false, 'Notifications need HTTPS: see Tailscale in setup.'],
  ['an iPhone over plain http, from the Home Screen', [{ ...api, isSecureContext: false }, { ...browser, maxTouchPoints: 5, standalone: true }], false, 'Notifications need HTTPS: see Tailscale in setup.'],
  ['Android over HTTPS', [api, { ...browser, maxTouchPoints: 5 }], true, ''],
  ['a computer\'s browser over HTTPS', [api, browser], true, ''],
  ['an iPhone\'s Safari tab over HTTPS', iphone(false), false, 'Notifications turn on in Smolcorp opened from your Home Screen.'],
  ['an iPhone\'s Safari tab with the push API', iphone(false, true), false, 'Notifications turn on in Smolcorp opened from your Home Screen.'],
  ['an iPhone from the Home Screen', iphone(true), true, ''],
  ['an iPhone from the Home Screen with no push API', iphone(true, false), false, ''],
  ['a browser with no worker', [api, { maxTouchPoints: 0 }], false, ''],
  ['a browser with no notifications', [{ isSecureContext: true, PushManager() {} }, browser], false, ''],
];
for (const [what, [w, n], shown, why] of cases) {
  if (pushShown(w, n) !== shown) throw new Error(what + ': the key ' + (shown ? "doesn't show" : 'shows'));
  if (!shown && pushWhyNot(w, n) !== why) throw new Error(what + ': says ' + JSON.stringify(pushWhyNot(w, n)));
}
`
	runNode(t, script)
}
