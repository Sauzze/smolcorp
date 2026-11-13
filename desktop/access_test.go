package desktop

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

const iPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)"

// wrongCode is a code that isn't code.
func wrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func TestACodePairsOnePhoneOnce(t *testing.T) {
	isolate(t)
	a := NewAccess()
	code := a.NewCode()
	key, ok := a.Redeem(code, "192.168.1.20", iPhone)
	if !ok || key == "" {
		t.Fatal("the active code didn't pair the phone")
	}
	if phone := a.Phones.WithKey(key); phone == nil || phone.Name != "iPhone" {
		t.Fatalf("the paired phone is %+v, want an iPhone holding the key", phone)
	}
	if _, ok := a.Redeem(code, "192.168.1.21", iPhone); ok {
		t.Fatal("a spent code paired a second phone")
	}
}

func TestNoCodeUntilOneIsAskedFor(t *testing.T) {
	isolate(t)
	a := NewAccess()
	if a.ActiveCode() != "" {
		t.Fatal("a code exists before anyone asked for one")
	}
	if _, ok := a.Redeem("000000", "192.168.1.20", iPhone); ok {
		t.Fatal("a phone paired with no code asked for")
	}
}

func TestACodeRunsOut(t *testing.T) {
	isolate(t)
	a := NewAccess()
	code := a.NewCode()
	a.codeExpiry = time.Now().Add(-time.Second)
	if a.ActiveCode() != "" {
		t.Fatal("an expired code is still active")
	}
	if _, ok := a.Redeem(code, "192.168.1.20", iPhone); ok {
		t.Fatal("an expired code paired a phone")
	}
}

func TestWrongCodesLockOutTheAddressTheyCameFrom(t *testing.T) {
	isolate(t)
	a := NewAccess()
	code := a.NewCode()
	for range maxAttemptsPerAddress {
		a.Redeem(wrongCode(code), "192.168.1.66", "")
	}
	if _, ok := a.Redeem(code, "192.168.1.66", ""); ok {
		t.Fatal("a locked-out address paired with the right code")
	}
	if _, ok := a.Redeem(code, "192.168.1.20", iPhone); !ok {
		t.Fatal("one address's wrong codes locked out another")
	}
}

func TestTooManyWrongCodesWithdrawTheCode(t *testing.T) {
	isolate(t)
	a := NewAccess()
	code := a.NewCode()
	// A device that changes its address after every guess.
	for i := range maxAttempts {
		a.Redeem(wrongCode(code), fmt.Sprintf("10.0.0.%d", i), "")
	}
	if a.ActiveCode() != "" {
		t.Fatal("the code survived twenty wrong guesses")
	}
	if _, ok := a.Redeem(code, "192.168.1.20", iPhone); ok {
		t.Fatal("a withdrawn code paired a phone")
	}
}

func TestANewCodeLiftsLockouts(t *testing.T) {
	isolate(t)
	a := NewAccess()
	code := a.NewCode()
	for range maxAttemptsPerAddress {
		a.Redeem(wrongCode(code), "192.168.1.66", "")
	}
	if _, ok := a.Redeem(a.NewCode(), "192.168.1.66", iPhone); !ok {
		t.Fatal("a lockout outlived the code it was for")
	}
}

func TestOnlyAHashOfEachKeyIsKept(t *testing.T) {
	isolate(t)
	key := NewAccess().Phones.Pair("iPhone")
	saved, err := os.ReadFile(phonesFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), key) {
		t.Fatal("phones.json holds a phone's key")
	}
	if !strings.Contains(string(saved), hashKey(key)) {
		t.Fatal("phones.json doesn't hold the key's hash")
	}
	if loadPhones().WithKey(key) == nil {
		t.Fatal("a phone's key stopped working once saved and loaded again")
	}
}

func TestRevokingAPhoneLeavesTheOthers(t *testing.T) {
	isolate(t)
	phones := NewAccess().Phones
	revoked, kept := phones.Pair("iPhone"), phones.Pair("Android phone")
	phones.Remove(phones.WithKey(revoked).ID)
	if phones.WithKey(revoked) != nil {
		t.Fatal("a revoked phone's key still works")
	}
	if phones.WithKey(kept) == nil {
		t.Fatal("revoking one phone cut off another")
	}
	phones.RemoveAll()
	if phones.WithKey(kept) != nil || len(loadPhones().All) != 0 {
		t.Fatal("unpairing all phones left one paired")
	}
}

func TestTheHookKeyIsNotAPhoneKey(t *testing.T) {
	isolate(t)
	a := NewAccess()
	phoneKey := a.Phones.Pair("iPhone")
	if a.IsHookKey(phoneKey) || a.IsHookKey("") {
		t.Fatal("a phone's key, or none, passed as the hook's")
	}
	if !a.IsHookKey(a.HookKey) || a.Phones.WithKey(a.HookKey) != nil {
		t.Fatal("the hook key isn't the hook's alone")
	}
	if NewAccess().HookKey != a.HookKey {
		t.Fatal("the hook key changed from one start to the next")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(hookKeyFile)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("the hook key's file is %v, want readable by its owner only", info.Mode().Perm())
		}
	}
}

func TestPhoneNames(t *testing.T) {
	for userAgent, want := range map[string]string{
		iPhone: "iPhone",
		"Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X)":           "iPad",
		"Mozilla/5.0 (Linux; Android 15; Pixel 9) Chrome/140.0.0": "Android phone",
		"curl/8.7.1": "Phone",
		// The Mac's screen saver pairs from its Options (ScreenSaver/Pairing.swift).
		"SauzzeDeckScreenSaver/1.25.0": "Screen saver",
	} {
		if got := phoneName(userAgent); got != want {
			t.Errorf("phoneName(%q) = %q, want %q", userAgent, got, want)
		}
	}
}

func TestSetupSessionsAreRandomKeptToTwentyAndNotTheKey(t *testing.T) {
	a := &Access{SetupKey: "fake-setup-key"}
	first := a.NewSetupSession()
	if first == "" || first == a.SetupKey || !a.IsSetupSession(first) {
		t.Fatalf("a session is %q: empty, the key, or not open", first)
	}
	if a.IsSetupSession(a.SetupKey) || a.IsSetupSession("") || a.IsSetupSession("made-up") {
		t.Error("the key, nothing or a made-up id counted as a session")
	}
	for i := 0; i < maxSetupSessions; i++ {
		a.NewSetupSession()
	}
	if len(a.setupSessions) != maxSetupSessions || a.IsSetupSession(first) {
		t.Errorf("%d sessions kept, the oldest still open: %v; want %d and the oldest gone", len(a.setupSessions), a.IsSetupSession(first), maxSetupSessions)
	}
}

func TestSetupTokensWorkOnceForFiveMinutes(t *testing.T) {
	a := &Access{}
	token := a.NewSetupToken()
	if left := time.Until(a.setupTokens[token]); left < 4*time.Minute+50*time.Second || left > 5*time.Minute {
		t.Errorf("a setup token lasts %v, want five minutes", left)
	}
	if !a.RedeemSetupToken(token) || a.RedeemSetupToken(token) {
		t.Error("a setup token did not work exactly once")
	}
	late := a.NewSetupToken()
	a.setupTokens[late] = time.Now().Add(-time.Second)
	if a.RedeemSetupToken(late) {
		t.Error("a setup token that had run out worked")
	}
}

// sealWords catches what loadPhones says, for one test.
func sealWords(t *testing.T) *[]string {
	t.Helper()
	var said []string
	saved := sayPhones
	sayPhones = func(text string) { said = append(said, text) }
	t.Cleanup(func() { sayPhones = saved })
	return &said
}

// RT126FS2b N2: a phone can't be added except by the owner's own pairing. An entry written into phones.json by
// anyone else (a key of its own, a made-up seal, the Windows window's device renamed as a phone) is left out at the
// next start, said, and never let in, while the phones paired here go on working.
func TestAPlantedPhoneIsNeverLetInAndIsSaid(t *testing.T) {
	isolate(t)
	said := sealWords(t)
	phones := loadPhones()
	real := phones.Pair("iPhone")
	window := ensureWindowKey(phones, windowKeyFile)
	var listed []map[string]any
	data, _ := os.ReadFile(phonesFile)
	if err := json.Unmarshal(data, &listed); err != nil || len(listed) != 2 {
		t.Fatalf("%v %s", err, data)
	}
	listed[1]["name"] = "iPhone" // the window's device, renamed as a phone
	listed = append(listed,
		map[string]any{"id": "PLANTED1", "name": "iPhone", "keyHash": hashKey("fake-planted-key")},
		map[string]any{"id": "PLANTED2", "name": "iPhone", "keyHash": hashKey("fake-planted-key-2"), "seal": strings.Repeat("ab", 32)},
		map[string]any{"id": "PLANTED3", "name": "iPhone", "keyHash": hashKey("fake-planted-key-3"), "seal": listed[0]["seal"]})
	planted, _ := json.Marshal(listed)
	if err := os.WriteFile(phonesFile, planted, 0o600); err != nil {
		t.Fatal(err)
	}
	again := loadPhones()
	for _, key := range []string{"fake-planted-key", "fake-planted-key-2", "fake-planted-key-3", window} {
		if again.WithKey(key) != nil {
			t.Errorf("a planted entry is let in (%.12s…)", key)
		}
	}
	if again.WithKey(real) == nil {
		t.Error("the phone paired here stopped working")
	}
	// The unsealed one waits for the owner; the three whose seal doesn't fit are dropped, to pair again.
	if len(again.Refused) != 4 || len(again.Waiting()) != 1 || len(*said) != 1 ||
		!strings.Contains((*said)[0], "1 device(s) this app can't vouch for") || !strings.Contains((*said)[0], "3 device(s) this app didn't pair") {
		t.Errorf("refused %d, waiting %d, said %q", len(again.Refused), len(again.Waiting()), *said)
	}
	// Through the router: the planted key answers no ask, from a phone over Tailscale, and the GM gets nothing.
	d, _, id := deckWithAnAsk(t)
	if err := os.WriteFile(phonesFile, planted, 0o600); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.access.Phones = loadPhones()
	d.mu.Unlock()
	for _, caller := range []string{"phone-wireguard", "phone-serve-mac"} {
		if status, text := probeRoute(t, d, "POST", "/grants/answer?id="+id+"&answer=yes", "fake-planted-key", ownerCall(caller)); status != http.StatusUnauthorized {
			t.Errorf("%s with a planted key: %d %q, want 401", caller, status, text)
		}
	}
	if state := grantState(t, d, id); state != grantOpen {
		t.Errorf("a planted phone changed the ask to %s", state)
	}
	if v := d.decide(actorGM, action{shell: &shellAsk{line: "echo hi"}}); v.allowed() {
		t.Errorf("the GM's next action: %+v", v)
	}
}

// RT126FS2c B1 (b): a list from before the seal (an update from 1.25: no entry sealed, no key kept) is never sealed on
// trust. Each entry waits, left out and said, until the owner confirms it on the setup page, one by one; an entry
// waiting outlasts a restart, and a revoked one is gone. Once phones carry seals, a list whose key is gone lets none
// of them in, and none of them waits: they pair again.
func TestPhonesFromBeforeTheSealWaitForTheOwner(t *testing.T) {
	isolate(t)
	said := sealWords(t)
	before := `[{"id":"A1","name":"iPhone","keyHash":"` + hashKey("fake-key-a") + `"},{"id":"B2","name":"iPad","keyHash":"` + hashKey("fake-key-b") + `"}]`
	if err := os.WriteFile(phonesFile, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	phones := loadPhones()
	if phones.WithKey("fake-key-a") != nil || phones.WithKey("fake-key-b") != nil || len(phones.Waiting()) != 2 {
		t.Fatalf("an unsealed list was trusted: %+v", phones)
	}
	if len(*said) != 1 || !strings.Contains((*said)[0], "2 device(s) this app can't vouch for") || !strings.Contains((*said)[0], "confirm it on the setup page") {
		t.Errorf("said %q", *said)
	}
	if info, err := os.Stat(phoneSealPath()); err != nil || !modeIs(info, 0o600) {
		t.Errorf("the seal's key: %v %v", info, err)
	}
	a, b := waitingEntry(t, phones, "A1"), waitingEntry(t, phones, "B2")
	// The tap names the row it showed: another row's digest, none, or one made up confirms nothing (RT126FS2R4 B1).
	for _, digest := range []string{EntryDigest(b), "", strings.Repeat("0", 64)} {
		if phones.Confirm("A1", digest) {
			t.Errorf("A1 confirmed with the digest %q", digest)
		}
	}
	if !phones.Confirm("A1", EntryDigest(a)) || phones.WithKey("fake-key-a") == nil || phones.WithKey("fake-key-b") != nil {
		t.Fatalf("confirming one: %+v", phones)
	}
	if phones.Confirm("A1", EntryDigest(a)) || phones.Confirm("nobody", EntryDigest(a)) {
		t.Error("confirmed twice, or one that isn't listed")
	}
	again := loadPhones()
	if again.WithKey("fake-key-a") == nil || again.WithKey("fake-key-b") != nil || len(again.Waiting()) != 1 || again.Waiting()[0].ID != "B2" {
		t.Errorf("after a restart: %+v", again)
	}
	again.Remove("B2")
	if third := loadPhones(); len(third.Refused) != 0 || third.WithKey("fake-key-a") == nil {
		t.Errorf("a revoked entry still waits, or the confirmed one went: %+v", third)
	}
	// The key gone, the sealed list can't be told from a planted one: none is let in, and none waits.
	if err := os.Remove(phoneSealPath()); err != nil {
		t.Fatal(err)
	}
	if gone := loadPhones(); len(gone.All) != 0 || len(gone.Refused) != 1 || len(gone.Waiting()) != 0 {
		t.Errorf("with the seal's key gone: %d let in, %d refused, %d waiting", len(gone.All), len(gone.Refused), len(gone.Waiting()))
	}
}

// waitingEntry is the entry with id that waits for the owner, as the setup page is shown it.
func waitingEntry(t *testing.T, phones *Phones, id string) PairedPhone {
	t.Helper()
	for _, phone := range phones.Waiting() {
		if phone.ID == id {
			return phone
		}
	}
	t.Fatalf("%s doesn't wait: %+v", id, phones.Refused)
	return PairedPhone{}
}

// confirmTarget is the page's confirm for phone: its id and its digest, as setup.html sends them.
func confirmTarget(phone PairedPhone) string {
	return "/setup/phones/confirm?phone=" + url.QueryEscape(phone.ID) + "&entry=" + EntryDigest(phone)
}

// plantPhone writes phones.json as a GM would: every seal stripped, an entry of its own added; and does to the seal's
// key what spoil says ("remove" or "junk").
func plantPhone(t *testing.T, spoil string) {
	t.Helper()
	var listed []map[string]any
	data, _ := os.ReadFile(phonesFile)
	if err := json.Unmarshal(data, &listed); err != nil {
		t.Fatal(err)
	}
	for _, entry := range listed {
		delete(entry, "seal")
	}
	listed = append(listed, map[string]any{"id": "PLANTED", "name": "iPhone", "keyHash": hashKey("fake-planted-key")})
	planted, _ := json.Marshal(listed)
	if err := os.WriteFile(phonesFile, planted, 0o600); err != nil {
		t.Fatal(err)
	}
	switch spoil {
	case "remove":
		if err := os.Remove(phoneSealPath()); err != nil {
			t.Fatal(err)
		}
	case "junk":
		if err := os.WriteFile(phoneSealPath(), []byte("not a key\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// reloadPhones is the deck starting again, as far as its phones go.
func reloadPhones(d *Deck) {
	d.mu.Lock()
	d.access.Phones = loadPhones()
	d.mu.Unlock()
}

// noOwnerYes checks that key, from a phone over Tailscale (WireGuard, and `tailscale serve` on a Mac), answers no ask
// (401), that the ask stays open, and that the GM's next identical action is still not allowed.
func noOwnerYes(t *testing.T, d *Deck, id, key, what string) {
	t.Helper()
	for _, caller := range []string{"phone-wireguard", "phone-serve-mac"} {
		if status, text := probeRoute(t, d, "POST", "/grants/answer?id="+id+"&answer=yes", key, ownerCall(caller)); status != http.StatusUnauthorized {
			t.Errorf("%s, %s: %d %q, want 401", what, caller, status, text)
		}
	}
	if state := grantState(t, d, id); state != grantOpen {
		t.Errorf("%s: the ask is %s", what, state)
	}
	if v := d.decide(actorGM, action{shell: &shellAsk{line: "echo hi"}}); v.allowed() {
		t.Errorf("%s: the GM's next action: %+v", what, v)
	}
}

// RT126FS2c P2, B1 (b): the update path. The GM deletes the seal's key, or writes junk into it, and strips every seal
// from phones.json with an entry of its own added. Nothing is sealed on trust: the planted entry waits, is said, and
// answers nothing, through the whole router; the owner's own phone waits too, until he confirms it on the page.
func TestP2TheUpdatePathSealsNothingOnTrust(t *testing.T) {
	for _, spoil := range []string{"remove", "junk"} {
		t.Run(spoil, func(t *testing.T) {
			said := sealWords(t)
			d, phoneKey, id := deckWithAnAsk(t)
			plantPhone(t, spoil)
			reloadPhones(d)
			d.mu.Lock()
			waiting := len(d.access.Phones.Waiting())
			d.mu.Unlock()
			if waiting != 2 || len(*said) != 1 || !strings.Contains((*said)[0], "None of them is let in until you confirm it") {
				t.Fatalf("waiting %d, said %q", waiting, *said)
			}
			noOwnerYes(t, d, id, "fake-planted-key", "the planted key")
			noOwnerYes(t, d, id, phoneKey, "the owner's phone, not yet confirmed")
			// The owner confirms his own phone on the page: it answers again. The planted one still doesn't.
			d.mu.Lock()
			owners := d.access.Phones.Waiting()[0]
			d.mu.Unlock()
			if w := serve(t, d, call{method: "POST", target: confirmTarget(owners), host: "localhost:7710", from: thisComputer,
				headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"waitingPhones":[{`) {
				t.Fatalf("the page confirms: %d %s", w.Code, w.Body.String())
			}
			noOwnerYes(t, d, id, "fake-planted-key", "the planted key, after the owner confirmed his own")
			if status, _ := probeRoute(t, d, "POST", "/grants/answer?id="+id+"&answer=yes", phoneKey, ownerCall("phone-wireguard")); status != http.StatusNoContent {
				t.Errorf("the owner's confirmed phone: %d, want 204", status)
			}
		})
	}
}

// RT126FS2c P3, B1 (c): replay. A phone unpaired (one, or all of them), its old entry copied back into phones.json, is
// not let in again: each unpairing makes a new seal's key. It doesn't wait for the owner either (its seal doesn't fit),
// the page can't confirm it, and it answers nothing. A phone kept goes on working, across a restart.
func TestP3AnUnpairedPhonePutBackIsNotLetIn(t *testing.T) {
	said := sealWords(t)
	d, phoneKey, id := deckWithAnAsk(t)
	d.mu.Lock()
	keptKey := d.access.Phones.Pair("Kept")
	old, _ := os.ReadFile(phonesFile)
	sealBefore, _ := os.ReadFile(phoneSealPath())
	probeID := d.access.Phones.WithKey(phoneKey).ID
	d.unpair(probeID)
	d.mu.Unlock()
	if sealAfter, _ := os.ReadFile(phoneSealPath()); string(sealAfter) == string(sealBefore) {
		t.Fatal("unpairing a phone kept the seal's key")
	}
	if err := os.WriteFile(phonesFile, old, 0o600); err != nil {
		t.Fatal(err)
	}
	reloadPhones(d)
	d.mu.Lock()
	refused, waiting, kept := len(d.access.Phones.Refused), len(d.access.Phones.Waiting()), d.access.Phones.WithKey(keptKey) != nil
	d.mu.Unlock()
	if refused != 2 || waiting != 0 || len(*said) != 1 || !strings.Contains((*said)[0], "seal doesn't fit") {
		t.Errorf("after unpair, old copy put back: refused %d, waiting %d, said %q", refused, waiting, *said)
	}
	if kept {
		t.Error("the old copy's entry of the kept phone fits the new key")
	}
	noOwnerYes(t, d, id, phoneKey, "the unpaired phone put back")
	d.mu.Lock()
	putBack := d.access.Phones.Refused[slices.IndexFunc(d.access.Phones.Refused, func(p PairedPhone) bool { return p.ID == probeID })]
	d.mu.Unlock()
	if w := serve(t, d, call{method: "POST", target: confirmTarget(putBack), host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusNotFound {
		t.Errorf("the page confirms an unpaired phone put back: %d, want 404", w.Code)
	}

	// The kept phone, saved since the unpairing, works across a restart; then Unpair All, and the old copy put back.
	d.mu.Lock()
	d.access.Phones = &Phones{}
	d.mu.Unlock()
	reloadFresh := func() string {
		d.mu.Lock()
		defer d.mu.Unlock()
		phones := loadPhones()
		key := phones.Pair("Fresh")
		d.access.Phones = phones
		return key
	}
	fresh := reloadFresh()
	reloadPhones(d)
	d.mu.Lock()
	if d.access.Phones.WithKey(fresh) == nil {
		t.Error("a phone paired after an unpairing is lost at the next start")
	}
	before, _ := os.ReadFile(phonesFile)
	d.unpair("")
	d.mu.Unlock()
	if err := os.WriteFile(phonesFile, before, 0o600); err != nil {
		t.Fatal(err)
	}
	reloadPhones(d)
	d.mu.Lock()
	letIn := d.access.Phones.WithKey(fresh) != nil
	d.mu.Unlock()
	if letIn {
		t.Error("after Unpair All, an old copy put back is let in")
	}
	noOwnerYes(t, d, id, fresh, "after Unpair All, the old copy put back")
}

// RemoveAll and Remove also forget an entry waiting for the owner; only the page confirms one, never a tool's key, the
// GM's token or nobody; and a phone kept through an unpairing is sealed again with the new key.
func TestOnlyThePageConfirmsAWaitingPhone(t *testing.T) {
	d, _ := probeDeck(t)
	plantPhone(t, "")
	reloadPhones(d)
	gm := d.newGMToken()
	target := "/setup/phones/confirm?phone=PLANTED"
	for name, c := range map[string]struct {
		c    call
		want int
	}{
		"a tool":    {call{asTool: true}, http.StatusForbidden},
		"the GM":    {call{withoutSetupKey: true, headers: map[string]string{gmTokenHeader: gm}}, http.StatusForbidden},
		"nobody":    {call{withoutSetupKey: true}, http.StatusUnauthorized},
		"key+token": {call{asTool: true, headers: map[string]string{gmTokenHeader: gm}}, http.StatusForbidden},
	} {
		c.c.method, c.c.target, c.c.host, c.c.from = "POST", target, "localhost:7710", thisComputer
		if c.c.headers == nil {
			c.c.headers = map[string]string{}
		}
		c.c.headers[setupHeader] = "1"
		if got := serve(t, d, c.c).Code; got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
	d.mu.Lock()
	letIn := d.access.Phones.WithKey("fake-planted-key") != nil
	d.mu.Unlock()
	if letIn {
		t.Fatal("someone but the page confirmed a phone")
	}
	for _, who := range []caller{callerTool, callerGM, callerHire, callerNobody} {
		if routeAllows(who, "POST", "/setup/phones/confirm") {
			t.Errorf("the route table lets %s confirm a phone", who.name())
		}
	}
	// A tool's view of the setup state never lists what waits.
	if w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, asTool: true}); strings.Contains(w.Body.String(), "waitingPhones") {
		t.Error("a tool sees the phones waiting")
	}
	d.mu.Lock()
	d.unpair("")
	waiting := len(d.access.Phones.Waiting())
	d.mu.Unlock()
	if waiting != 0 || len(loadPhones().Refused) != 0 {
		t.Error("Unpair All left an entry waiting")
	}
}

// RT126FS2c nit 3: with no seal's key to be had, Pair keeps nothing and hands out no key; the window's device writes
// no key file for a device that isn't there.
func TestAPhoneThatCantBeKeptGetsNoKey(t *testing.T) {
	isolate(t)
	if err := os.MkdirAll(phoneSealPath(), 0o700); err != nil { // a folder where the key's file goes: it can't be written
		t.Fatal(err)
	}
	phones := loadPhones()
	if key := phones.Pair("iPhone"); key != "" || len(phones.All) != 0 {
		t.Errorf("Pair with no seal's key: %q, %d kept", key, len(phones.All))
	}
	if key := ensureWindowKey(phones, windowKeyFile); key != "" {
		t.Errorf("the window's device: %q", key)
	}
	if _, err := os.Stat(windowKeyFile); !os.IsNotExist(err) {
		t.Errorf("a window key was written for no device: %v", err)
	}
	a := &Access{Phones: phones, misses: map[string]int{}}
	key, ok := a.Redeem(a.NewCode(), "192.168.1.20", iPhone)
	if !ok || key != "" {
		t.Errorf("Redeem: %q %v, want a right code and no key", key, ok)
	}
}

// ownerPhoneTwinned writes phones.json as on the update to 1.26 with a GM at work: every seal stripped, and ahead of
// the owner's own entry a copy of it (his phone's id, name and date) holding the GM's key's hash instead.
func ownerPhoneTwinned(t *testing.T) {
	t.Helper()
	var listed []map[string]any
	data, _ := os.ReadFile(phonesFile)
	if err := json.Unmarshal(data, &listed); err != nil || len(listed) != 1 {
		t.Fatalf("%v %s", err, data)
	}
	delete(listed[0], "seal")
	twin := map[string]any{}
	for k, v := range listed[0] {
		twin[k] = v
	}
	twin["keyHash"] = hashKey("fake-planted-key")
	planted, _ := json.Marshal([]map[string]any{twin, listed[0]})
	if err := os.WriteFile(phonesFile, planted, 0o600); err != nil {
		t.Fatal(err)
	}
}

// pageState is /setup/state as the page gets it.
func pageState(t *testing.T, d *Deck) map[string]any {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	var state map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatalf("%d %v %s", w.Code, err, w.Body.String())
	}
	return state
}

// RT126FS2R4 B1, probe A: two entries with one id. The GM puts a copy of the owner's waiting entry, with its own key's
// hash, ahead of his. Neither is offered: the page's rows are empty, a confirm of either (id and digest) is 404, the
// planted key and the owner's both get 401, the ask stays open and the GM isn't allowed; and it is said. The same for
// two rows alike in every field, and for one key's hash under two ids.
func TestTwoEntriesWithOneIdAreNeitherOffered(t *testing.T) {
	said := sealWords(t)
	d, phoneKey, id := deckWithAnAsk(t)
	ownerPhoneTwinned(t)
	reloadPhones(d)
	d.mu.Lock()
	refused := slices.Clone(d.access.Phones.Refused)
	waiting := len(d.access.Phones.Waiting())
	d.mu.Unlock()
	if len(refused) != 2 || refused[0].ID != refused[1].ID || waiting != 0 {
		t.Fatalf("refused %+v, waiting %d", refused, waiting)
	}
	if len(*said) != 1 || !strings.Contains((*said)[0], "2 device(s) that share an id or a key") || strings.Contains((*said)[0], "can't vouch for") {
		t.Errorf("said %q", *said)
	}
	if rows, ok := pageState(t, d)["waitingPhones"].([]any); !ok || len(rows) != 0 {
		t.Errorf("the page is offered %v", pageState(t, d)["waitingPhones"])
	}
	for _, entry := range refused {
		if w := serve(t, d, call{method: "POST", target: confirmTarget(entry), host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusNotFound {
			t.Errorf("the page confirms one of two rows with one id: %d, want 404", w.Code)
		}
	}
	noOwnerYes(t, d, id, "fake-planted-key", "the planted twin")
	noOwnerYes(t, d, id, phoneKey, "the owner's phone beside its twin")

	// Even a Confirm that skips the page's list: an id listed twice lets nothing in.
	d.mu.Lock()
	for _, entry := range refused {
		if d.access.Phones.Confirm(entry.ID, EntryDigest(entry)) {
			t.Error("Confirm took one of two entries with one id")
		}
	}
	d.mu.Unlock()

	alike := `{"id":"A1","name":"iPhone","keyHash":"` + hashKey("fake-key-a") + `"}`
	oneKey := `{"id":"B2","name":"iPad","keyHash":"` + hashKey("fake-key-a") + `"}`
	for name, list := range map[string]string{"alike in every field": "[" + alike + "," + alike + "]", "one key under two ids": "[" + alike + "," + oneKey + "]"} {
		if err := os.WriteFile(phonesFile, []byte(list), 0o600); err != nil {
			t.Fatal(err)
		}
		phones := loadPhones()
		if len(phones.Refused) != 2 || len(phones.Waiting()) != 0 || phones.Confirm("A1", EntryDigest(phones.Refused[0])) || phones.WithKey("fake-key-a") != nil {
			t.Errorf("%s: %d refused, %d waiting", name, len(phones.Refused), len(phones.Waiting()))
		}
	}
}

// RT126FS2R4 B2, probe C: the confirm never shows a planter's words. Only an entry under a name this app gives phones,
// with a key's hash in this app's form, waits; anything else (the probe's "Smolcorp checked this phone", markup,
// look-alikes, hidden marks, a long name, a hash that isn't one) is refused outright, said escaped, and never
// reaches the page. A waiting row carries the fixed name, the date, a digest and an 8-digit fingerprint, and no
// other field.
func TestAWaitingPhoneShowsNoWordsOfThePlanters(t *testing.T) {
	isolate(t)
	misleading := []string{
		"iPhone\u202e\n\nSmolcorp checked this phone: it is the one you paired. Safe to confirm.\u2800\u2800",
		strings.Repeat("iPhone", 470),
		`<img src=x onerror="alert(1)">`, "<b>iPhone</b>", "iPhone<br>Verified by Smolcorp",
		"\u0456Phone", "iPh\u043ene", "\uff49Phone", "iPhone\u200b", "iPhone ", " iPhone", "iphone", "IPHONE",
		"iPhone (checked by Smolcorp)", "Phone\u0301", "Android phone\u2800", "", "Smolcorp",
	}
	type row struct{ ID, Name, KeyHash string }
	var listed []row
	for i, name := range misleading {
		listed = append(listed, row{fmt.Sprintf("BAD%02d", i), name, hashKey(fmt.Sprintf("fake-bad-%d", i))})
	}
	listed = append(listed, row{"BADHASH", "iPhone", "<b>not a hash</b>"}, row{"UPPERHASH", "iPhone", strings.ToUpper(hashKey("fake-upper"))})
	for i, name := range waitingNames {
		listed = append(listed, row{fmt.Sprintf("OK%02d", i), name, hashKey(fmt.Sprintf("fake-ok-%d", i))})
	}
	data, _ := json.Marshal(listed)
	if err := os.WriteFile(phonesFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	said := sealWords(t)
	phones := loadPhones()
	var names []string
	for _, phone := range phones.Waiting() {
		names = append(names, phone.Name)
	}
	if !slices.Equal(names, waitingNames) {
		t.Errorf("waiting: %q, want %q", names, waitingNames)
	}
	for _, phone := range phones.Refused {
		if strings.HasPrefix(phone.ID, "BAD") || strings.HasSuffix(phone.ID, "HASH") {
			if digest := EntryDigest(phone); phones.Confirm(phone.ID, digest) {
				t.Errorf("%q was confirmed", phone.Name)
			}
		}
	}
	text := strings.Join(*said, " ")
	if !strings.Contains(text, fmt.Sprintf("%d device(s) under a name, or in a form, this app never gives a phone", len(misleading)+2)) {
		t.Errorf("said %q", text)
	}
	// What is said is escaped and cut: no hidden mark, no new line, no long name, none of the planter's sentence.
	if strings.ContainsAny(text, "\u202e\u2800\u200b\u0301\n") || strings.Contains(text, "Safe to confirm") || strings.Contains(text, strings.Repeat("iPhone", 10)) {
		t.Errorf("said raw: %q", text)
	}

	// Through the router: probe C's entry doesn't reach the page, and a waiting row holds only what's fixed.
	d, _ := probeDeck(t)
	if err := os.WriteFile(phonesFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	reloadPhones(d)
	rows, _ := pageState(t, d)["waitingPhones"].([]any)
	if len(rows) != len(waitingNames) {
		t.Fatalf("%d rows, want %d: %v", len(rows), len(waitingNames), rows)
	}
	for _, r := range rows {
		got := r.(map[string]any)
		keys := slices.Sorted(maps.Keys(got))
		name, _ := got["name"].(string)
		key, _ := got["key"].(string)
		if !slices.Equal(keys, []string{"entry", "id", "key", "name", "pairedAt"}) || !slices.Contains(waitingNames, name) || len(key) != 8 || !isKeyHash(key+strings.Repeat("0", 56)) {
			t.Errorf("a waiting row: %v", got)
		}
	}
	if w := serve(t, d, call{method: "POST", target: "/setup/phones/confirm?phone=BAD00&entry=" + EntryDigest(PairedPhone{ID: "BAD00", Name: misleading[0], KeyHash: hashKey("fake-bad-0")}),
		host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusNotFound {
		t.Errorf("probe C's entry confirmed: %d", w.Code)
	}
}

// RT126FS2R4 nits 1 and 2: phones waiting for the owner count where a count decides what to offer or keep open. A
// Linux owner coming from a build before the Tailscale default, every phone waiting, keeps every interface; the menu
// still offers Unpair All.
func TestWaitingPhonesCountForTheScopeAndTheMenu(t *testing.T) {
	d, _ := probeDeck(t)
	before := `[{"id":"A1","name":"iPhone","keyHash":"` + hashKey("fake-key-a") + `"},{"id":"B2","name":"iPad","keyHash":"` + hashKey("fake-key-b") + `"}]`
	if err := os.WriteFile(phonesFile, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	reloadPhones(d)
	d.mu.Lock()
	only, known := phonesOnly(d.access.Phones), phonesKnown(d.access.Phones)
	d.mu.Unlock()
	if only != 0 || known != 2 {
		t.Errorf("let in %d, known %d", only, known)
	}
	c := &Config{}
	if !settleListenScope(c, known, "linux") || c.Listen != listenAll {
		t.Errorf("the scope with two phones waiting: %q", c.Listen)
	}
	if got := d.menuState().Phones; got != 2 {
		t.Errorf("the menu counts %d phones", got)
	}
}
