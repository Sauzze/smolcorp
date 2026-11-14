package desktop

import (
	"bytes"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// The portable office under the Mac app (RM7 B1, B2): its keys are the Swift engine's own Keychain items,
// through the bridge, and the app tells when you're back at the Mac. On stand-ins only: never a Keychain.

// macPortableDeck is a deck under the stand-in Mac app, holding items, whose portable office sends to a
// stand-in viewer.
func macPortableDeck(t *testing.T, items map[string]string) (*Deck, *macStandIn, *relay, string) {
	t.Helper()
	mac := withMac(t)
	mac.secrets = items
	d := newTestDeck(t)
	r := &relay{}
	base := strings.Replace(r.serve(t).URL, "127.0.0.1", "localhost", 1)
	d.config.Viewer.BaseURL = base
	d.portable.setSettings(d.config.Viewer)
	t.Cleanup(func() { d.portable.Stop(0) })
	return d, mac, r, base
}

// secretsAsked is the stand-in's requests of these actions, as "action item".
func (m *macStandIn) secretsAsked(actions ...string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var said []string
	for _, request := range m.asked {
		if slices.Contains(actions, request.Action) {
			said = append(said, request.Action+" "+request.Item)
		}
	}
	return said
}

// A 1.24 user's link keeps working after the upgrade: the id, key and write secret are read from the Swift
// engine's items, the office goes to that id under that secret, and nothing is made or written over.
func TestA124ViewerLinkKeepsWorkingUnderTheMacApp(t *testing.T) {
	id, key := "AbCdEfGhIjKlMnOpQrStUv", base64URL(bytes.Repeat([]byte{7}, 32))
	d, mac, r, base := macPortableDeck(t, map[string]string{"viewer-id": id, "viewer-key": key, "viewer-write-secret": "ws-fake-124"})
	if !d.portable.secrets.keychain || d.portable.mac == nil {
		t.Fatal("under the Mac app, the portable office isn't the app's")
	}
	link := d.portable.Link()
	if link != base+"/v/"+id+"#k="+key || !d.portable.IsSetUp() {
		t.Fatalf("the 1.24 link is %q, set up %v", link, d.portable.IsSetUp())
	}
	d.mu.Lock()
	running(d, 1)
	d.mu.Unlock()
	d.portable.Start()
	waitFor(t, "the first upload", func() bool { return slices.Contains(r.seen(), "PUT /api/s/"+id) })
	r.mu.Lock()
	body, auth := r.bodies["/api/s/"+id], r.auth[0]
	r.mu.Unlock()
	if auth != "Bearer ws-fake-124" {
		t.Errorf("sent with %q, want the kept write secret", auth)
	}
	opened(t, link, body)
	if written := mac.secretsAsked("secret-write", "secret-delete"); len(written) > 0 {
		t.Errorf("the kept items were changed: %v", written)
	}
}

// With none yet, the first write secret makes the id and key, all three kept as the app's items. A new key
// is written over the old one, and a wipe has the viewer forget: neither removes an item, which the app
// doesn't allow for these (as the Swift engine kept them).
func TestTheViewersFirstKeysAreMadeInTheMacAppsItemsAndNeverRemoved(t *testing.T) {
	d, mac, r, base := macPortableDeck(t, nil)
	if d.portable.IsSetUp() || d.portable.Link() != "" {
		t.Fatal("set up with nothing kept")
	}
	// found:false is surely none: a first identity is made, not refused as unreadable.
	code, text := pageCall(t, d, "POST", "/setup/viewer/secret")
	if code != http.StatusOK {
		t.Fatalf("the write secret: %d %s", code, text)
	}
	if made := mac.secretsAsked("secret-write"); !slices.Equal(made, []string{"secret-write viewer-write-secret", "secret-write viewer-id", "secret-write viewer-key"}) {
		t.Errorf("kept %v", made)
	}
	mac.mu.Lock()
	first := mac.secrets["viewer-id"]
	want := base + "/v/" + first + "#k=" + mac.secrets["viewer-key"]
	mac.mu.Unlock()
	if d.portable.Link() != want || !d.portable.IsSetUp() {
		t.Fatalf("the link %q, want the items' %q", d.portable.Link(), want)
	}

	d.portable.RotateKey()
	mac.mu.Lock()
	second := mac.secrets["viewer-id"]
	mac.mu.Unlock()
	if second == first || !strings.Contains(d.portable.Link(), "/v/"+second+"#") {
		t.Errorf("rotated, the link is %q", d.portable.Link())
	}
	waitFor(t, "the old id forgotten", func() bool { return slices.Contains(r.seen(), "DELETE /api/s/"+first) })
	d.portable.Wipe()
	if !slices.Contains(r.seen(), "DELETE /api/s") {
		t.Error("the viewer wasn't asked to forget every office")
	}
	if removed := mac.secretsAsked("secret-delete"); len(removed) > 0 {
		t.Errorf("removed %v", removed)
	}
	if !d.portable.IsSetUp() {
		t.Error("a wipe undid the setup")
	}
}

// A key that's there but can't be read (the app's no, never found:false) is never made again: every link
// given out would stop working. It's said in the Mac's words, as a write secret that can't be kept is.
func TestAViewerKeyTheMacCantReadIsNeverReplaced(t *testing.T) {
	d, mac, _, _ := macPortableDeck(t, map[string]string{"viewer-id": "AbCdEfGhIjKlMnOpQrStUv", "viewer-key": "kept", "viewer-write-secret": "ws-fake-2"})
	mac.mu.Lock()
	mac.unreadable = []string{"viewer-key"}
	mac.mu.Unlock()
	d.portable.Start()
	if d.portable.IsOn() {
		t.Fatal("on without its key")
	}
	if _, problem := d.portable.Status(); problem != "Can't read the viewer's key from the Keychain" {
		t.Errorf("the problem is %q", problem)
	}
	if written := mac.secretsAsked("secret-write"); len(written) > 0 {
		t.Errorf("an unreadable key was written over: %v", written)
	}
	mac.refusing(map[string]string{"secret-write": "It wasn't kept."})
	if code, text := pageCall(t, d, "POST", "/setup/viewer/secret"); code != http.StatusInternalServerError ||
		text != "Couldn't keep the write secret in the Keychain." {
		t.Errorf("a write secret not kept: %d %q", code, text)
	}
	// Off the Mac app, the same in this computer's words.
	if (viewerSecrets{}).notKept() != viewerNoStoreProblem || (viewerSecrets{}).unreadable() != "Can't read the viewer's key from your computer's secret store" {
		t.Error("the computer's words changed")
	}
}

// You're back at the Mac as the Swift engine's portable office had it: an unlock, or the keyboard or mouse
// used on an unlocked screen within 10 minutes of a wake. What the app saw before this start doesn't count.
func TestBackAtTheMacIsAnUnlockOrUseAfterAWake(t *testing.T) {
	at := time.Unix(1_800_000_000, 0)
	var w macReturn
	step := func(now macPresence, after time.Duration) bool { return w.see(now, at.Add(after)) }
	if step(macPresence{unlocks: 3, wakes: 2}, 0) {
		t.Fatal("the unlocks before this start counted")
	}
	if step(macPresence{unlocks: 3, wakes: 2, idle: time.Second}, 5*time.Second) {
		t.Error("in use with no wake: back, where the Swift engine waited for an unlock or a wake")
	}
	if !step(macPresence{unlocks: 4, wakes: 2, idle: time.Hour}, 10*time.Second) {
		t.Error("an unlock isn't being back")
	}
	if step(macPresence{unlocks: 4, wakes: 3, idle: time.Minute}, 15*time.Second) {
		t.Error("woken, not used yet: back")
	}
	if step(macPresence{unlocks: 4, wakes: 3, idle: time.Second, locked: true}, 20*time.Second) {
		t.Error("used on a locked screen: back, where it waits for the unlock")
	}
	if !step(macPresence{unlocks: 4, wakes: 3, idle: time.Second}, 25*time.Second) {
		t.Error("used after the wake on an unlocked screen isn't being back")
	}
	if step(macPresence{unlocks: 4, wakes: 3, idle: time.Second}, 15*time.Second+viewerWakeWatch) {
		t.Error("used 10 minutes after the wake still counted as coming back from it")
	}
	// An app whose counts started again: from here on, not an unlock.
	if step(macPresence{unlocks: 1, wakes: 0}, 11*time.Minute) || !step(macPresence{unlocks: 2}, 12*time.Minute) {
		t.Error("counts that started again")
	}
}

// Under the Mac app the portable office asks the app, not this system, as it starts and for the grace.
func TestThePortableOfficeAsksTheMacAppWhetherYoureBack(t *testing.T) {
	d, mac, _, _ := macPortableDeck(t, map[string]string{"viewer-id": "AbCdEfGhIjKlMnOpQrStUv",
		"viewer-key": base64URL(bytes.Repeat([]byte{9}, 32)), "viewer-write-secret": "ws-fake-3"})
	mac.mu.Lock()
	mac.presence = bridgeAnswer{Idle: 1.5, Locked: true, Unlocks: 2, Wakes: 1}
	mac.mu.Unlock()
	d.portable.presence = func() (time.Duration, bool, bool) {
		t.Error("this system's presence was asked under the Mac app")
		return 0, false, false
	}
	d.portable.Start()
	waitFor(t, "the app asked where you are", func() bool { return slices.Contains(mac.actions(), "presence") })
	if idle, locked, ok := d.portable.presenceNow(); !ok || !locked || idle != 1500*time.Millisecond {
		t.Errorf("the app's presence read as %v %v %v", idle, locked, ok)
	}
	mac.refusing(map[string]string{"presence": "Not this app's engine."})
	if _, _, ok := d.portable.presenceNow(); ok {
		t.Error("an app that said no was taken for an answer")
	}
}
