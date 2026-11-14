package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// SK1 (SANDBOX-PLAN §3.1, §3.2): on a Mac, nothing a program running as the owner can read in the support folder is
// a key that opens anything, and nothing it writes there grants anything. These rows play that program with plain
// file reads and writes, and a stand-in Mac app whose stand-in Keychain holds the record key (hostsetup_test.go).

// recordWords catches what the records and the phones say, for one test.
func recordWords(t *testing.T) *[]string {
	t.Helper()
	var said []string
	savedRecords, savedPhones := sayRecords, sayPhones
	sayRecords = func(text string) { said = append(said, text) }
	sayPhones = func(text string) { said = append(said, text) }
	t.Cleanup(func() { sayRecords, sayPhones = savedRecords, savedPhones })
	return &said
}

// macDeck is a deck under a stand-in Mac app, with a phone paired over Tailscale and an ask of the GM's open: the
// deck, the stand-in, the phone's key and the ask's id.
func macDeck(t *testing.T) (*Deck, *macStandIn, string, string) {
	t.Helper()
	isolate(t)
	// servedUnrecorded too: probeRoute stands a peer in for each caller (SN1), as probeDeck puts it back.
	savedMac, savedAddresses, savedOpen, savedPeer := macTailscale, tailscaleAddresses, openSetupPage, servedUnrecorded
	t.Cleanup(func() {
		macTailscale, tailscaleAddresses, openSetupPage, servedUnrecorded = savedMac, savedAddresses, savedOpen, savedPeer
	})
	tailscaleAddresses = func() []string { return []string{"100.64.1.2"} }
	openSetupPage = func(*Deck) error { return nil }
	mac := withMac(t)
	d := recordsDeck(t)
	if state := recordsState(); state != recordsOK {
		t.Fatalf("the records' state at the start: %q", state)
	}
	d.mu.Lock()
	phoneKey := d.access.Phones.Pair("iPhone", true)
	d.mu.Unlock()
	askingGate(t, "")
	if v := d.decide(actorGM, action{shell: &shellAsk{line: "echo hi"}}); v.kind != verdictAskOwner {
		t.Fatalf("%+v", v)
	}
	return d, mac, phoneKey, d.grants.waiting()[0].ID
}

// restart is the core started again on the same folder and the same Mac app: nothing held from the run before.
func restart(t *testing.T) *Deck {
	t.Helper()
	forgetHeldRegistries()
	forgetHeldGuard(filepath.Join(supportDir, hireGuardFile))
	return recordsDeck(t)
}

// recordsDeck is a deck started as the app starts it, with no coding agent looked for (as newTestDeck's): a look
// still running after the test would read the next test's folders.
func recordsDeck(t *testing.T) *Deck {
	t.Helper()
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	return d
}

// withAHire puts an open hire in the registry, with a send key and a contact with the GM's row: its key.
func withAHire(t *testing.T) string {
	t.Helper()
	frontDeskMu.Lock()
	file := readRegistry(registryPath(), true)
	file.hires = append(file.hires, Hire{ID: "hire01", Name: "Ada", Folder: homeDir, State: "working"}, Hire{ID: "hire02", Name: "Bo", Folder: homeDir, State: "working"})
	file.contacts = []Contact{{From: "hire01", To: "hire02"}}
	err := file.save(registryPath())
	frontDeskMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	key, err := newSendKey(registryPath(), "hire01")
	if err != nil || key == "" {
		t.Fatalf("send key: %v", err)
	}
	return key
}

// rewrite reads name in the support folder as JSON, lets change edit it, and writes it back: another program's
// write, which keeps whatever seal the file carried.
func rewrite(t *testing.T, name string, change func(map[string]any)) {
	t.Helper()
	path := filepath.Join(supportDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	change(file)
	out, _ := json.Marshal(file)
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// A stand-in phones.json holds no key a program could use: neither a phone's key nor the secret it signs with
// (its key's SHA-256). The list is sealed, and a phone still works across a restart.
func TestOnAMacPhonesJSONHoldsNoKeyThatOpensAnything(t *testing.T) {
	d, _, phoneKey, id := macDeck(t)
	data, err := os.ReadFile(phonesFile)
	if err != nil {
		t.Fatal(err)
	}
	for what, secret := range map[string]string{"the phone's key": phoneKey, "the secret it signs with": hashKey(phoneKey)} {
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("phones.json holds %s", what)
		}
	}
	if seal, ok := sealOfFile("phones.json", data); !ok || !seal.fits(recordKey()) {
		t.Errorf("phones.json isn't sealed: %s", data)
	}
	_, _ = d, id
	if restart(t).access.Phones.WithKey(phoneKey) == nil {
		t.Error("the paired phone stopped working across a restart")
	}
}

// Each sealed record changed by another program (an entry added, a field changed) counts for nothing at the next
// start: no phone, no peer, no hire key, no contact, no looser setting, no looser guard. What the core wrote itself
// goes on counting.
func TestOnAMacARecordAnotherProgramChangesCountsForNothing(t *testing.T) {
	d, _, phoneKey, id := macDeck(t)
	sendKey := withAHire(t)
	d.mu.Lock()
	peerKey := d.macPeers.pair("Studio Mac")
	d.config.KeystrokesAllowed = false
	d.config.save()
	d.mu.Unlock()
	guardPath := filepath.Join(supportDir, hireGuardFile)
	if err := holdGuard(guardPath, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	// Untouched, every record counts after a restart.
	again := restart(t)
	if again.access.Phones.WithKey(phoneKey) == nil || again.macPeers.withKey(peerKey) == nil || !isHireKey(registryPath(), sendKey) {
		t.Fatal("an untouched record stopped counting across a restart")
	}

	plantedPhone, plantedPeer, plantedSend := "fake-planted-phone", "fake-planted-peer", "fake-planted-send"
	said := recordWords(t)
	rewrite(t, "phones.json", func(f map[string]any) {
		f["body"] = append(f["body"].([]any), map[string]any{"id": "PLANTED", "name": "iPhone", "keyHash": hashKey(plantedPhone), "signs": true})
	})
	rewrite(t, "mac-peers.json", func(f map[string]any) {
		f["body"] = append(f["body"].([]any), map[string]any{"id": "PLANTED", "name": "Mac", "keyHash": hashKey(plantedPeer)})
	})
	rewrite(t, registryName, func(f map[string]any) {
		f["keys"].(map[string]any)["hire02"] = hashKey(plantedSend)
		f["contacts"] = append(f["contacts"].([]any), map[string]any{"from": "hire02", "to": "hire01"})
	})
	rewrite(t, "config.json", func(f map[string]any) { f["keystrokesAllowed"] = true; f["listen"] = "all" })
	if err := os.WriteFile(guardPath, []byte(`{"version":1,"off":["everything"]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	changed := restart(t)
	for what, ok := range map[string]bool{
		"the planted phone":      changed.access.Phones.WithKey(plantedPhone) != nil,
		"the owner's phone":      changed.access.Phones.WithKey(phoneKey) != nil,
		"the planted peer":       changed.macPeers.withKey(plantedPeer) != nil,
		"the planted send key":   isHireKey(registryPath(), plantedSend),
		"the hire's send key":    isHireKey(registryPath(), sendKey),
		"the planted contact":    readRegistry(registryPath(), false).mayWrite("hire02", "hire01"),
		"keys to other apps":     changed.config.KeystrokesAllowed,
		"the planted listen":     changed.config.Listen == "all",
		"the planted guard file": heldGuard(guardPath).data != nil,
	} {
		if ok {
			t.Errorf("%s counts after another program changed its record", what)
		}
	}
	for _, key := range []string{plantedPhone, phoneKey} {
		noOwnerYes(t, changed, id, key, "a phone from a changed phones.json")
	}
	if len(*said) < 4 {
		t.Errorf("what was left out wasn't said: %q", *said)
	}
}

// A copy of a record from before the last rotation (a phone or a Mac unpaired) doesn't fit, whole files put back
// together included: the restore of a support folder brings back no phone and no peer it unpaired since.
func TestOnAMacACopyFromBeforeTheLastRotationDoesntFit(t *testing.T) {
	d, mac, phoneKey, id := macDeck(t)
	sendKey := withAHire(t)
	d.mu.Lock()
	peerKey := d.macPeers.pair("Studio Mac")
	d.mu.Unlock()
	names := []string{"phones.json", "mac-peers.json", registryName, "config.json"}
	before := map[string][]byte{}
	for _, name := range names {
		before[name], _ = os.ReadFile(filepath.Join(supportDir, name))
	}
	itemBefore := slices.Clone(mac.recordItems)
	d.mu.Lock()
	d.unpair(d.access.Phones.WithKey(phoneKey).ID)
	d.mu.Unlock()
	if len(mac.recordItems) != 1 || mac.recordItems[0].ID == itemBefore[0].ID {
		t.Fatalf("unpairing didn't rotate the record key in the order add, delete: %+v", mac.recordItems)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(supportDir, name), before[name], 0o600); err != nil {
			t.Fatal(err)
		}
	}
	again := restart(t)
	if again.access.Phones.WithKey(phoneKey) != nil || again.macPeers.withKey(peerKey) != nil || isHireKey(registryPath(), sendKey) {
		t.Error("a copy from before the rotation counts")
	}
	noOwnerYes(t, again, id, phoneKey, "an unpaired phone put back with its old list")
}

// Nothing another program writes from scratch is let in: a phone, a peer, a record or a grant of its own, sealed
// with a key it made up.
func TestOnAMacAProgramCantMintAPhoneAPeerARecordOrAGrant(t *testing.T) {
	d, _, phoneKey, id := macDeck(t)
	fakeKey := []byte("FAKEKEY-made-up-record-key-00000")
	mint := func(name string, body []byte) []byte {
		out, _ := json.Marshal(sealedFile{Record: name, Seal: recordSeal{MAC: recordMAC(fakeKey, name, "", body)}, Body: body})
		return out
	}
	phones, _ := json.Marshal([]PairedPhone{{ID: "MINTED", Name: "iPhone", KeyHash: hashKey("fake-minted-phone"), Signs: true}})
	peers, _ := json.Marshal([]MacPeer{{ID: "MINTED", Name: "Mac", KeyHash: hashKey("fake-minted-peer")}})
	for name, data := range map[string][]byte{"phones.json": mint("phones.json", phones), "mac-peers.json": mint("mac-peers.json", peers)} {
		if err := os.WriteFile(filepath.Join(supportDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registry := registryFile{hires: []Hire{{ID: "mint01", Name: "Minted", Folder: homeDir, State: "working", Role: gmRole}},
		keys: map[string]string{"mint01": hashKey("fake-minted-send")}}
	records.mu.Lock()
	saved := records.key
	records.key = fakeKey
	records.mu.Unlock()
	_ = registry.save(registryPath())
	records.mu.Lock()
	records.key = saved
	records.mu.Unlock()
	// A grant of its own: a line in grants.jsonl, signed with a key it made up.
	line, _ := json.Marshal(grantRecord{ID: "g-minted", Actor: "gm", What: "echo hi", Key: grantKey("gm", "echo hi", ""), State: grantGranted,
		At: time.Now().UnixMilli(), Expires: time.Now().Add(time.Hour).UnixMilli(), Sig: strings.Repeat("ab", 32)})
	if err := os.WriteFile(filepath.Join(supportDir, "grants.jsonl"), append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	again := restart(t)
	if again.access.Phones.WithKey("fake-minted-phone") != nil || again.macPeers.withKey("fake-minted-peer") != nil ||
		isHireKey(registryPath(), "fake-minted-send") || readRegistry(registryPath(), false).gmIsHired() {
		t.Error("a minted record counts")
	}
	if v := again.decide(actorGM, action{shell: &shellAsk{line: "echo hi"}}); v.allowed() {
		t.Errorf("a minted grant lets the GM act: %+v", v)
	}
	noOwnerYes(t, again, id, "fake-minted-phone", "a minted phone")
	_, _ = d, phoneKey
}

// The rotation is crash-safe. A stop after the new item is added and the records are sealed under it, before the old
// item is deleted, leaves two items: the next start honours nothing, Setup says an update was interrupted, and the
// owner's reset starts the records over, after which a phone pairs and answers again. A stop right after the add
// (nothing sealed yet) is the same, said as another program's change. Neither lets anything in.
func TestOnAMacTheRotationIsCrashSafe(t *testing.T) {
	for _, stop := range []string{"before the delete", "right after the add"} {
		t.Run(stop, func(t *testing.T) {
			d, mac, phoneKey, id := macDeck(t)
			kept := func() string {
				d.mu.Lock()
				defer d.mu.Unlock()
				return d.access.Phones.Pair("iPad", true)
			}()
			if stop == "before the delete" {
				mac.refuse = map[string]string{"record-key-retire": "The Mac app stopped."}
				d.mu.Lock()
				d.unpair(d.access.Phones.WithKey(phoneKey).ID)
				d.mu.Unlock()
			} else if _, err := hostRecordKey(mac, "record-key-add", ""); err != nil {
				t.Fatal(err)
			}
			if len(mac.recordItems) != 2 {
				t.Fatalf("items after the stop: %d", len(mac.recordItems))
			}
			mac.refuse = nil
			again := restart(t)
			want := map[string]string{"before the delete": recordsInterrupted, "right after the add": recordsChanged}[stop]
			if state := recordsState(); state != want {
				t.Errorf("the state after the stop: %q, want %q", state, want)
			}
			if again.access.Phones.WithKey(kept) != nil || again.access.Phones.WithKey(phoneKey) != nil {
				t.Error("a phone counts after an interrupted rotation")
			}
			noOwnerYes(t, again, id, kept, "after an interrupted rotation")
			again.mu.Lock()
			refused := again.access.Phones.Pair("iPhone", true) == ""
			again.mu.Unlock()
			if !refused {
				t.Error("a phone was paired while the records' key failed its check")
			}
			state := pageState(t, again)
			if said, _ := state["records"].(string); !strings.Contains(said, map[string]string{recordsInterrupted: "interrupted", recordsChanged: "Another program"}[want]) {
				t.Errorf("Setup says %q", said)
			}
			if code, _, _ := fromPage(t, again, http.MethodPost, "/setup/records/reset", nil); code != http.StatusOK {
				t.Fatalf("reset: %d", code)
			}
			if recordsState() != recordsOK || len(mac.recordItems) != 1 {
				t.Errorf("after the reset: %q, %d items", recordsState(), len(mac.recordItems))
			}
			again.mu.Lock()
			fresh := again.access.Phones.Pair("iPhone", true)
			again.mu.Unlock()
			if fresh == "" || restart(t).access.Phones.WithKey(fresh) == nil {
				t.Error("a phone paired after the reset doesn't count at the next start")
			}
		})
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

// noOwnerYes checks that key, from a phone over Tailscale (WireGuard, and `tailscale serve` on a Mac), answers no ask
// (401), that the ask stays open, and that the GM's next identical action is still not allowed.
func noOwnerYes(t *testing.T, d *Deck, id, key, what string) {
	t.Helper()
	for _, caller := range []string{"phone-wireguard", "phone-serve-mac"} {
		if status, text := probeRoute(t, d, "POST", "/grants/answer?id="+id+"&answer=yes", key, ownerCall(caller)); status != http.StatusUnauthorized {
			t.Errorf("%s, %s: %d %q, want 401", what, caller, status, text)
		}
	}
	if v := d.decide(actorGM, action{shell: &shellAsk{line: "echo hi"}}); v.allowed() {
		t.Errorf("%s: the GM's next action: %+v", what, v)
	}
}

// When the record key's item checks out wrong (overwritten, unreadable), nothing sealed counts: no phone, no hire
// key, no GM, the security settings at their safe defaults.
func TestOnAMacARecordKeyThatFailsItsCheckLetsNothingIn(t *testing.T) {
	for _, look := range []string{"moved", "unreadable", "none"} {
		t.Run(look, func(t *testing.T) {
			d, mac, phoneKey, _ := macDeck(t)
			sendKey := withAHire(t)
			d.mu.Lock()
			d.config.KeystrokesAllowed = true
			d.config.save()
			d.mu.Unlock()
			token := d.newGMToken()
			d.gmKept.flush(time.Minute)
			withHiredGM(t, "working")
			if look == "none" {
				mac.recordItems = nil
			} else {
				mac.recordLook = look
			}
			again := restart(t)
			if recordsState() == recordsOK || recordsText(recordsState(), false) == "" {
				t.Fatalf("state %q", recordsState())
			}
			if again.access.Phones.WithKey(phoneKey) != nil || isHireKey(registryPath(), sendKey) || worksAsGM(again, token) || again.config.KeystrokesAllowed {
				t.Error("a sealed record counts while the record key failed its check")
			}
			if data, _ := os.ReadFile(phonesFile); bytes.Contains(data, []byte(hashKey(phoneKey))) {
				t.Error("a phone's secret was written in clear")
			}
		})
	}
}

// The one-time change of every key (B1, §3.9), at the first start of 1.26 under the Mac app: every phone and Mac
// pairs again, the hires' send keys and the GM's token stop working, the hook's key is new, the files that held keys
// in clear go, and what the update found that grants nothing new (the team, its contacts, the settings, the guard)
// is sealed as found and goes on counting. A second start changes nothing more.
func TestOnAMacTheFirstStartChangesEveryKeyOnce(t *testing.T) {
	isolate(t)
	said := recordWords(t)
	phone, peer, send, gm, hook := "fake-125-phone", "fake-125-peer", "fake-125-send", "fake-gm-token-from-1-25-0000000000000000000", "fake-125-hook"
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(supportDir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("phones.json", `[{"id":"A1","name":"iPhone","keyHash":"`+hashKey(phone)+`","signs":true,"seal":"`+strings.Repeat("cd", 32)+`"}]`)
	write(phoneSealName, strings.Repeat("ab", 32)+"\n")
	write("token", "fake-1.0-token\n")
	write("mac-peers.json", `[{"id":"P1","name":"Mac","keyHash":"`+hashKey(peer)+`"}]`)
	write(registryName, `{"version":1,"hires":[{"id":"h1","name":"Ada","folder":"`+homeDir+`","state":"working"},{"id":"h2","name":"Bo","folder":"`+homeDir+`","state":"working"}],`+
		`"contacts":[{"from":"h1","to":"h2"}],"keys":{"h1":"`+hashKey(send)+`"}}`)
	write("config.json", `{"keystrokesAllowed":true,"keystrokeApps":["com.example.notes"]}`)
	write(hireGuardFile, `{"version":1}`)
	write("hook-key", hook+"\n")
	mac := withMac(t)
	mac.secrets = map[string]string{gmTokenItem: gm}

	d := recordsDeck(t)
	d.gmKept.flush(time.Minute)
	if recordsState() != recordsOK {
		t.Fatalf("state %q", recordsState())
	}
	if d.access.Phones.WithKey(phone) != nil || d.macPeers.withKey(peer) != nil || isHireKey(registryPath(), send) || worksAsGM(d, gm) || d.access.HookKey == hook {
		t.Error("a key from before the update still works")
	}
	for _, name := range []string{phoneSealName, "token"} {
		if _, err := os.Lstat(filepath.Join(supportDir, name)); err == nil {
			t.Errorf("%s is still there", name)
		}
	}
	if !readRegistry(registryPath(), false).mayWrite("h1", "h2") || !d.config.KeystrokesAllowed || heldGuard(filepath.Join(supportDir, hireGuardFile)).data == nil {
		t.Error("what the update found that grants nothing new was lost")
	}
	if len(*said) == 0 || !strings.Contains(strings.Join(*said, " "), "Pair each phone again") {
		t.Errorf("the change wasn't said: %q", *said)
	}
	for _, name := range []string{"phones.json", "mac-peers.json", registryName, "config.json"} {
		data, _ := os.ReadFile(filepath.Join(supportDir, name))
		if seal, ok := sealOfFile(name, data); !ok || !seal.fits(recordKey()) {
			t.Errorf("%s isn't sealed after the first start", name)
		}
	}

	// The second start: the team, the settings and the guard still count, a phone paired since too.
	d.mu.Lock()
	paired := d.access.Phones.Pair("iPhone", true)
	d.mu.Unlock()
	again := restart(t)
	if again.access.Phones.WithKey(paired) == nil || !readRegistry(registryPath(), false).mayWrite("h1", "h2") || !again.config.KeystrokesAllowed ||
		heldGuard(filepath.Join(supportDir, hireGuardFile)).data == nil || recordsRotated() {
		t.Error("the second start lost what the first sealed, or changed the keys again")
	}
}

// Off a Mac there is no record key (§3.8, Q7): nothing is sealed, a file counts as the core finds it at its start,
// and the one-time change of every key happens all the same.
func TestOffAMacEveryKeyChangesOnceAndFilesCountAsFound(t *testing.T) {
	isolate(t)
	phone := "fake-125-phone"
	if err := os.WriteFile(phonesFile, []byte(`[{"id":"A1","name":"iPhone","keyHash":"`+hashKey(phone)+`","signs":true}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := recordsDeck(t)
	if recordsState() != recordsOff || d.access.Phones.WithKey(phone) != nil {
		t.Fatal("off a Mac, a phone from before the update still works")
	}
	d.mu.Lock()
	key := d.access.Phones.Pair("iPhone", true)
	d.mu.Unlock()
	data, _ := os.ReadFile(phonesFile)
	if _, sealed := sealOfFile("phones.json", data); sealed || bytes.Contains(data, []byte(key)) {
		t.Errorf("off a Mac: sealed %v, or the key in the file", sealed)
	}
	if restart(t).access.Phones.WithKey(key) == nil {
		t.Error("off a Mac, a phone paired since the update stopped working")
	}
}

// The inventory of the support folder (§3.2): every file the core writes, after a phone, a peer, a hire's key, the
// GM's token, the settings and the guard are all made, has one of four classes, and a new file without one fails.
// No file holds any of the keys made, nor a phone's signing secret. A sealed record carries a seal that fits.
func TestOnAMacEveryFileInTheSupportFolderHasAClass(t *testing.T) {
	const (
		verifier  = "verifier"
		notSecret = "not secret"
		checkOnly = "check-only key"
		sealed    = "sealed record"
		// What SK1 leaves readable, by design or until SK2: the hook's key (§5: the owner's own sessions need it),
		// and the setup page's key, new at each start (SK2 moves it to memory, with a check-only key for tools).
		readableKey = "readable key (§5, SK2)"
	)
	classes := map[string]string{
		"phones.json": sealed, "mac-peers.json": sealed, registryName: sealed, "config.json": sealed, hireGuardFile: sealed,
		hireGuardSealFile: notSecret, recordsRotatedFile: notSecret, "instance.lock": notSecret, "sessions.json": notSecret,
		"events.jsonl": notSecret, "grants.jsonl": notSecret, "hires": notSecret, "hook-key": readableKey, "hook-header": readableKey,
		"setup-key": readableKey, "hire-queue.json": notSecret, "saved.json": notSecret,
		// Not written on a Mac, where the Keychain item holds the GM's verifier; off a Mac, the verifier alone.
		keptGMTokenName: verifier,
		// SK2's check-only key for `smolcorp setup`: it opens the self-test and nothing else.
		"selftest-key": checkOnly,
		// ACP1: the ACP desks' transcripts, what was said (acptranscript.go). They grant nothing.
		acpTranscriptFolder: notSecret,
	}
	d, mac, phoneKey, _ := macDeck(t)
	sendKey := withAHire(t)
	d.mu.Lock()
	peerKey := d.macPeers.pair("Studio Mac")
	d.config.save()
	d.mu.Unlock()
	token := d.newGMToken()
	d.gmKept.flush(time.Minute)
	if err := holdGuard(filepath.Join(supportDir, hireGuardFile), []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	if transcript, err := newACPTranscript(supportDir, "s1"); err != nil {
		t.Fatal(err)
	} else {
		transcript.user("hello", time.Now())
		transcript.close(time.Now())
	}
	if kept, _ := mac.keptItem(); kept != keptAs(token) {
		t.Errorf("the GM's Keychain item holds %q, not the token's verifier", kept)
	}
	keys := map[string]string{"a phone's key": phoneKey, "a phone's signing secret": hashKey(phoneKey), "a peer's key": peerKey,
		"a hire's send key": sendKey, "the GM's token": token}
	entries, err := os.ReadDir(supportDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		class, ok := classes[name]
		if !ok {
			t.Errorf("%s has no class in the inventory", name)
			continue
		}
		if entry.IsDir() {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(supportDir, name))
		for what, key := range keys {
			if bytes.Contains(data, []byte(key)) {
				t.Errorf("%s (%s) holds %s", name, class, what)
			}
		}
		if class == sealed && name != hireGuardFile {
			if seal, ok := sealOfFile(name, data); !ok || !seal.fits(recordKey()) {
				t.Errorf("%s is a sealed record without a seal that fits", name)
			}
		}
	}
}

// A record the core wrote itself always fits at the next start, empty lists and maps included, which the files
// leave out: a seal never breaks on its own and drops the owner's settings or team.
func TestOnAMacARecordTheCoreWroteFitsAtTheNextStart(t *testing.T) {
	d, _, _, _ := macDeck(t)
	withAHire(t)
	frontDeskMu.Lock()
	file := readRegistry(registryPath(), true)
	file.keys, file.contacts = map[string]string{}, []Contact{}
	err := file.save(registryPath())
	frontDeskMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.config.KeystrokeApps, d.config.KeystrokesAllowed, d.config.Listen = []string{}, true, "all"
	d.config.save()
	d.mu.Unlock()
	said := recordWords(t)
	again := restart(t)
	if !again.config.KeystrokesAllowed || again.config.Listen != "all" || len(readRegistry(registryPath(), false).hires) != 2 || len(*said) != 0 {
		t.Errorf("a record the core wrote didn't fit: %q", *said)
	}
}

// Deleting every sealed record (and the guard's seal) only starts the records over: with the record key's item
// already in the Keychain, the next start isn't the update to 1.26, so a guard or a setting another program wrote
// meanwhile is never sealed as found. Every phone pairs again: a denial of service, not a way in.
func TestOnAMacDeletingTheSealedRecordsSealsNothingAsFound(t *testing.T) {
	d, _, phoneKey, _ := macDeck(t)
	guardPath := filepath.Join(supportDir, hireGuardFile)
	if err := holdGuard(guardPath, []byte(`{"version":1,"groups":["strict"]}`)); err != nil {
		t.Fatal(err)
	}
	_ = d
	for _, name := range []string{"phones.json", "mac-peers.json", registryName, hireGuardSealFile} {
		_ = os.Remove(filepath.Join(supportDir, name))
	}
	if err := os.WriteFile(guardPath, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configFile, []byte(`{"keystrokesAllowed":true,"listen":"all"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	again := restart(t)
	if recordsState() != recordsOK || recordsFirst() {
		t.Fatalf("state %q, first %v", recordsState(), recordsFirst())
	}
	if heldGuard(guardPath).data != nil || again.config.KeystrokesAllowed || again.config.Listen == "all" || again.access.Phones.WithKey(phoneKey) != nil {
		t.Error("a file written after the sealed records were deleted was sealed as found")
	}
}
