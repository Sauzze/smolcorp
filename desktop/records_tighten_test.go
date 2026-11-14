package desktop

import (
	"bytes"
	"errors"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// RT126SK1's blockers, each as its probe ran (P1–P6), and the rotation stopped at each of its steps. "Another
// program" is played with plain file reads and writes, and with the stand-in Mac app's Keychain (hostsetup_test.go).

// sealedCopies are the sealed records in the support folder as they are now, by name, the guard and its seal with
// them: what a program would copy, to put back later.
func sealedCopies(t *testing.T) map[string][]byte {
	t.Helper()
	copies := map[string][]byte{}
	for _, name := range append(sealedRecordNames(), hireGuardFile, hireGuardSealFile) {
		if data, err := os.ReadFile(filepath.Join(supportDir, name)); err == nil {
			copies[name] = data
		}
	}
	return copies
}

// putBack writes copies over the support folder's files, as that program would.
func putBack(t *testing.T, copies map[string][]byte) {
	t.Helper()
	for name, data := range copies {
		if err := os.WriteFile(filepath.Join(supportDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// P1 (B4): the registry is held in memory on every system. A copy put back in the same run, from before a send key
// was renewed, doesn't bring the old key back; nor does a key another program adds to the file.
func TestProbeRegistryReplayInRun(t *testing.T) {
	for _, mac := range []bool{true, false} {
		t.Run(map[bool]string{true: "on a Mac", false: "off a Mac"}[mac], func(t *testing.T) {
			if mac {
				macDeck(t)
			} else {
				isolate(t)
				newTestDeck(t)
			}
			old := withAHire(t)
			copied, err := os.ReadFile(registryPath())
			if err != nil {
				t.Fatal(err)
			}
			if fresh, err := newSendKey(registryPath(), "hire01"); err != nil || fresh == "" {
				t.Fatalf("a new send key: %v", err)
			}
			if err := os.WriteFile(registryPath(), copied, 0o600); err != nil {
				t.Fatal(err)
			}
			if isHireKey(registryPath(), old) {
				t.Error("after the replay, in the same run, the old send key works")
			}
			rewrite(t, registryName, func(file map[string]any) {
				file["keys"] = map[string]any{"hire02": hashKey("FAKEKEY-planted-send-key")}
			})
			if isHireKey(registryPath(), "FAKEKEY-planted-send-key") {
				t.Error("a send key another program wrote in works in the same run")
			}
		})
	}
}

// P2–P3 and the rest of B3: every change that takes authority away rotates the records, so a copy of every sealed
// record from before it no longer fits at the next start: a hire let go, a send key renewed, a contact removed, a
// GM's scope narrowed, a security setting changed, a guard rule added (and a phone and a peer unpaired, as before).
func TestOnAMacACopyFromBeforeEachTighteningDoesntFit(t *testing.T) {
	guardPath := func() string { return filepath.Join(supportDir, hireGuardFile) }
	editRegistry := func(change func(*registryFile)) {
		frontDeskMu.Lock()
		defer frontDeskMu.Unlock()
		file := readRegistry(registryPath(), true)
		change(&file)
		if err := file.save(registryPath()); err != nil {
			t.Fatal(err)
		}
	}
	cases := map[string]func(d *Deck){
		"a hire let go": func(*Deck) {
			editRegistry(func(f *registryFile) { f.hires[1].State = "fired" })
		},
		"a send key renewed": func(*Deck) {
			if _, err := newSendKey(registryPath(), "hire01"); err != nil {
				t.Fatal(err)
			}
		},
		"a contact removed": func(*Deck) { editRegistry(func(f *registryFile) { f.contacts = nil }) },
		"a GM's scope narrowed": func(*Deck) {
			editRegistry(func(f *registryFile) { f.hires[2].Scope = filepath.Join(homeDir, "one-project") })
		},
		"keystrokes and the listener turned off": func(d *Deck) {
			d.mu.Lock()
			d.config.KeystrokesAllowed, d.config.Listen = false, "tailscale"
			d.config.save()
			d.mu.Unlock()
		},
		"a guard rule added": func(*Deck) {
			if err := holdGuard(guardPath(), []byte(`{"groups":[{"name":"push","on":true}],"extra":["rm -rf"]}`)); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, tighten := range cases {
		t.Run(name, func(t *testing.T) {
			d, _, _, _ := macDeck(t)
			old := withAHire(t)
			editRegistry(func(f *registryFile) {
				f.hires = append(f.hires, Hire{ID: "gm0001", Name: "GM", Role: gmRole, Folder: homeDir, State: "working"})
			})
			d.mu.Lock()
			d.config.KeystrokesAllowed, d.config.Listen = true, "all"
			d.config.save()
			d.mu.Unlock()
			if err := holdGuard(guardPath(), []byte(`{"groups":[{"name":"push","on":false}]}`)); err != nil {
				t.Fatal(err)
			}
			before := sealedCopies(t)
			tighten(d)
			putBack(t, before)
			again := restart(t)
			if state := recordsState(); state != recordsOK {
				t.Fatalf("the state after a tightening: %q", state)
			}
			for record, data := range before {
				if seal, sealed := sealOfFile(record, data); sealed && seal.fits(recordKey()) {
					t.Errorf("%s from before still fits", record)
				}
			}
			if isHireKey(registryPath(), old) {
				t.Error("the send key from before works")
			}
			again.mu.Lock()
			keys, listen := again.config.KeystrokesAllowed, again.config.Listen
			again.mu.Unlock()
			if keys || listen == "all" {
				t.Errorf("P2: at the next start keystrokesAllowed=%v listen=%q", keys, listen)
			}
			if held := heldGuard(guardPath()); held.data != nil && string(held.data) == string(before[hireGuardFile]) {
				t.Error("P3: at the next start the guard held is the loose copy")
			}
		})
	}
}

// P4 (B2): with a GM hired, a verifier another program puts in the app's GM item (it can: K7, K11) never counts
// at the next start: on a Mac the verifier is sealed under the record key. One the core kept itself still works.
func TestOnAMacAPlantedGMVerifierDoesntCount(t *testing.T) {
	d, mac, _, _ := macDeck(t)
	withHiredGM(t, "working")
	gm := d.newGMToken()
	d.gmKept.flush(gmTokenFlushWait)
	if sealed, _ := mac.sealedItem(); !strings.HasPrefix(sealed, keptAs(gm)+":") {
		t.Fatalf("the GM's item isn't sealed: %q", sealed)
	}
	if again := restart(t); !worksAsGM(again, gm) {
		t.Fatal("the GM's own token doesn't work after a restart")
	}
	mac.mu.Lock()
	mac.secrets[gmTokenItem] = keptAs("FAKEKEY-another-programs-gm-token")
	mac.mu.Unlock()
	if worksAsGM(restart(t), "FAKEKEY-another-programs-gm-token") {
		t.Error("at the next start the other program's token is the GM's")
	}
}

// P5 (B1): another program overwrites the record key's new item in the second of the add, at an unpairing the GM
// can call for. The app reads every add back and refuses that one (KeychainGate.swift; the Swift test plays it on a
// real keychain file), leaving a witness beside it: the rotation doesn't happen, Setup says a copy from before
// still fits, and at the next start the two items count for nothing, so the program's own phone isn't let in.
func TestOnAMacARecordKeyOverwrittenAsItWasAddedLetsNothingIn(t *testing.T) {
	d, mac, phoneKey, _ := macDeck(t)
	kept := func() string {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.access.Phones.Pair("iPad", true)
	}()
	mac.addOverwritten = true
	d.mu.Lock()
	d.unpair(d.access.Phones.WithKey(phoneKey).ID)
	d.mu.Unlock()
	if said, _ := pageState(t, d)["records"].(string); !strings.Contains(said, "would still count") {
		t.Errorf("Setup says %q after a rotation that couldn't make its key", said)
	}
	// The program's phones.json, sealed under the value it put in the item.
	planted := keyFromBridge(bridgeRecordKey{ID: "key-overwritten", Value: strings.Repeat("41", 32)})
	if err := os.WriteFile(phonesFile, sealBody(planted, "", "phones.json", []byte(`[{"id":"ph1","name":"iPhone","keyHash":"`+
		hashKey("FAKEKEY-planted-phone")+`","signs":true}]`)), 0o600); err != nil {
		t.Fatal(err)
	}
	again := restart(t)
	if state := recordsState(); state == recordsOK {
		t.Fatal("records state \"ok\" after an item overwritten as it was added")
	}
	if again.access.Phones.WithKey("FAKEKEY-planted-phone") != nil || again.access.Phones.WithKey(kept) != nil {
		t.Error("a phone is let in")
	}
	// The same at a reset: the key isn't made, and a restart still counts nothing.
	mac.addOverwritten = true
	if code, _, _ := fromPage(t, again, http.MethodPost, "/setup/records/reset", nil); code != http.StatusConflict {
		t.Errorf("a reset whose key was overwritten as it was added: %d", code)
	}
	restart(t)
	if state := recordsState(); state == recordsOK {
		t.Error("records state \"ok\" after a reset whose key was overwritten")
	}
}

// P6 (B5): a planted item fails the check, and the owner's Reset after it keeps the team: every hire, its folder
// and worktree, with only its keys gone, and a GM or a hire held to a project no longer open. Setup says what Reset
// clears.
func TestOnAMacAResetAfterAFailedCheckKeepsTheTeam(t *testing.T) {
	_, mac, _, _ := macDeck(t)
	withAHire(t)
	frontDeskMu.Lock()
	file := readRegistry(registryPath(), true)
	file.hires = append(file.hires, Hire{ID: "gm0001", Name: "GM", Role: gmRole, Folder: homeDir, State: "working", Scope: homeDir},
		Hire{ID: "hire03", Name: "Cy", Folder: homeDir, Worktree: filepath.Join(homeDir, "wt-cy"), Branch: "cy", State: "idle"})
	_ = file.save(registryPath())
	frontDeskMu.Unlock()
	mac.recordItems = append(mac.recordItems, newRecordItem("key-planted"))
	d := restart(t)
	if state := recordsState(); state != recordsChanged {
		t.Fatalf("state after the plant: %q", state)
	}
	check := func(when string, f registryFile) {
		t.Helper()
		if len(f.hires) != 4 || len(f.keys) != 0 || len(f.contacts) != 0 {
			t.Fatalf("%s: %d hires, %d keys, %d contacts", when, len(f.hires), len(f.keys), len(f.contacts))
		}
		for _, h := range f.hires {
			if open := isOpen(h); open == (h.Role == gmRole) {
				t.Errorf("%s: %s is open %v", when, h.Name, open)
			}
		}
		if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == "hire03" }); i < 0 || f.hires[i].Worktree == "" {
			t.Errorf("%s: a hire's worktree is gone", when)
		}
	}
	check("the team the core goes by", readRegistry(registryPath(), false))
	if said, _ := pageState(t, d)["records"].(string); !strings.Contains(said, "Your team, its desks and worktrees") {
		t.Errorf("Setup doesn't say what Reset keeps: %q", said)
	}
	if code, _, _ := fromPage(t, d, http.MethodPost, "/setup/records/reset", nil); code != http.StatusOK {
		t.Fatalf("reset: %d", code)
	}
	restart(t)
	if state := recordsState(); state != recordsOK {
		t.Fatalf("after the reset: %q", state)
	}
	check("registry.json after the reset", readRegistry(registryPath(), false))
}

// errStopped is the core stopping in the middle of a rotation.
var errStopped = errors.New("the core stopped here")

// The rotation is crash-safe at every one of its steps: the core stopped right after the add, after each record is
// written under the new key, and after the delete. A phone unpaired before the stop never comes back; until the
// delete is done the next start counts nothing (two items), and after it, everything kept counts.
func TestOnAMacTheRotationIsCrashSafeAtEachStep(t *testing.T) {
	var steps []string
	func() {
		d, _, phoneKey, _ := macDeck(t)
		withAHire(t)
		saved := rotationStep
		t.Cleanup(func() { rotationStep = saved })
		rotationStep = func(step string) { steps = append(steps, step) }
		d.mu.Lock()
		d.unpair(d.access.Phones.WithKey(phoneKey).ID)
		d.mu.Unlock()
		rotationStep = saved
	}()
	if len(steps) < 4 || steps[0] != "add" || steps[len(steps)-1] != "retire" {
		t.Fatalf("a rotation's steps: %q", steps)
	}
	for _, stop := range steps {
		t.Run(stop, func(t *testing.T) {
			d, _, phoneKey, _ := macDeck(t)
			hireKey := withAHire(t)
			d.mu.Lock()
			kept := d.access.Phones.Pair("iPad", true)
			d.mu.Unlock()
			before := sealedCopies(t)
			saved := rotationStep
			t.Cleanup(func() { rotationStep = saved })
			rotationStep = func(step string) {
				if step == stop {
					panic(errStopped)
				}
			}
			func() {
				defer func() {
					if r := recover(); r != errStopped {
						panic(r)
					}
				}()
				d.mu.Lock()
				defer d.mu.Unlock()
				d.unpair(d.access.Phones.WithKey(phoneKey).ID)
			}()
			rotationStep = saved
			again := restart(t)
			if stop == "retire" {
				if state := recordsState(); state != recordsOK || again.access.Phones.WithKey(kept) == nil || !isHireKey(registryPath(), hireKey) {
					t.Errorf("stopped after the delete: %q, the phone kept counts %v", state, again.access.Phones.WithKey(kept) != nil)
				}
			} else if state := recordsState(); state == recordsOK || again.access.Phones.WithKey(kept) != nil {
				t.Errorf("stopped after %q: %q, and a phone counts %v", stop, state, again.access.Phones.WithKey(kept) != nil)
			}
			// The list from before put back: the unpaired phone never comes back.
			putBack(t, map[string][]byte{"phones.json": before["phones.json"]})
			if restart(t).access.Phones.WithKey(phoneKey) != nil {
				t.Errorf("stopped after %q: the unpaired phone is let in", stop)
			}
		})
	}
}

// RT126SK1R2 N1: a record that didn't fit at a start whose check passed is dropped, and the records rotate as the
// start ends. For each record: a program copies it, then breaks its seal; the next start drops it and Setup names it;
// the owner sets something that takes nothing away from what the core then holds; the copy is put back. It no longer
// fits, and nothing it held comes back.
func TestOnAMacACopyFromBeforeADroppedRecordDoesntFit(t *testing.T) {
	zeros := strings.Repeat("0", 64)
	breakSeal := func(name, field string) func() {
		return func() { rewrite(t, name, func(f map[string]any) { f[field] = map[string]any{"mac": zeros} }) }
	}
	guardPath := func() string { return filepath.Join(supportDir, hireGuardFile) }
	loose := []byte(`{"groups":[{"name":"push","on":false}]}`)
	type row struct {
		record string
		before func(d *Deck) string            // the owner's record as the copy holds it: what it lets in
		broken func()                          // another program breaks the record
		owner  func(d *Deck)                   // what the owner sets after the drop
		counts func(d *Deck, what string) bool // whether what the copy let in counts again
	}
	rows := map[string]row{
		"phones": {record: "phones.json",
			before: func(*Deck) string { return "" }, // macDeck's phone
			broken: breakSeal("phones.json", "seal"),
			owner: func(d *Deck) {
				d.mu.Lock()
				d.access.Phones.Pair("iPad", true)
				d.mu.Unlock()
			},
			counts: func(d *Deck, key string) bool { return d.access.Phones.WithKey(key) != nil }},
		"peers": {record: "mac-peers.json",
			before: func(d *Deck) string {
				d.mu.Lock()
				defer d.mu.Unlock()
				return d.macPeers.pair("Studio Mac")
			},
			broken: breakSeal("mac-peers.json", "seal"),
			owner: func(d *Deck) {
				d.mu.Lock()
				d.macPeers.pair("Laptop")
				d.mu.Unlock()
			},
			counts: func(d *Deck, key string) bool { return d.macPeers.withKey(key) != nil }},
		"registry": {record: registryName,
			before: func(*Deck) string { return withAHire(t) },
			broken: breakSeal(registryName, sealedPartField),
			owner: func(*Deck) {
				if fresh, err := newSendKey(registryPath(), "hire01"); err != nil || fresh == "" {
					t.Fatalf("a new send key: %v", err)
				}
			},
			counts: func(_ *Deck, key string) bool { return isHireKey(registryPath(), key) }},
		"registry unreadable": {record: registryName,
			before: func(*Deck) string { return withAHire(t) },
			broken: func() { _ = os.WriteFile(registryPath(), []byte("{"), 0o600) },
			owner: func(*Deck) {
				frontDeskMu.Lock()
				defer frontDeskMu.Unlock()
				file := readRegistry(registryPath(), true)
				file.hires = append(file.hires, Hire{ID: "hire09", Name: "Di", Folder: homeDir, State: "working"})
				if err := file.save(registryPath()); err != nil {
					t.Fatal(err)
				}
			},
			counts: func(_ *Deck, key string) bool { return isHireKey(registryPath(), key) }},
		"config": {record: "config.json",
			before: func(d *Deck) string {
				d.mu.Lock()
				defer d.mu.Unlock()
				d.config.KeystrokesAllowed, d.config.Listen = true, "all"
				d.config.save()
				return ""
			},
			broken: breakSeal("config.json", sealedPartField),
			owner: func(d *Deck) {
				d.mu.Lock()
				d.config.Logo = "keys" // no security setting: not a change of the sealed part
				d.config.save()
				d.mu.Unlock()
			},
			counts: func(d *Deck, _ string) bool {
				d.mu.Lock()
				defer d.mu.Unlock()
				return d.config.KeystrokesAllowed || d.config.Listen == "all"
			}},
		"guard": {record: hireGuardFile,
			before: func(*Deck) string {
				if err := holdGuard(guardPath(), loose); err != nil {
					t.Fatal(err)
				}
				return ""
			},
			broken: func() { _ = os.WriteFile(guardSealPath(guardPath()), []byte(zeros+"\n"), 0o600) },
			owner: func(*Deck) {
				if err := holdGuard(guardPath(), []byte(`{"groups":[{"name":"push","on":true}]}`)); err != nil {
					t.Fatal(err)
				}
			},
			counts: func(*Deck, string) bool { return bytes.Equal(heldGuard(guardPath()).data, loose) }},
	}
	for name, r := range rows {
		t.Run(name, func(t *testing.T) {
			d, _, phoneKey, _ := macDeck(t)
			what := r.before(d)
			if what == "" {
				what = phoneKey
			}
			copies := sealedCopies(t)
			if copies[r.record] == nil {
				t.Fatalf("no %s to copy", r.record)
			}
			r.broken()
			dropped := restart(t)
			if state := recordsState(); state != recordsOK {
				t.Fatalf("after the break: %q", state)
			}
			if r.counts(dropped, what) {
				t.Fatalf("the broken %s still counts", r.record)
			}
			if said, _ := pageState(t, dropped)["recordsDropped"].(string); said != "A record didn't fit and was dropped: "+r.record+
				". Another program may have changed it; its copies from before no longer count." {
				t.Errorf("Setup says of the drop: %q", said)
			}
			r.owner(dropped)
			if name == "registry unreadable" {
				// RT126SK1R3 N4: the file that couldn't be read was set aside whole at the edit, never written over.
				if aside, err := os.ReadFile(filepath.Join(supportDir, "registry.broken.json")); err != nil || string(aside) != "{" {
					t.Errorf("registry.broken.json isn't the file that couldn't be read: %v %q", err, aside)
				}
			}
			back := map[string][]byte{r.record: copies[r.record]}
			if r.record == hireGuardFile {
				back[hireGuardSealFile] = copies[hireGuardSealFile]
			}
			putBack(t, back)
			again := restart(t)
			if state := recordsState(); state != recordsOK {
				t.Fatalf("after the copy is put back: %q", state)
			}
			if r.counts(again, what) {
				t.Errorf("the copy of %s from before the drop counts again", r.record)
			}
			if r.record == hireGuardFile {
				if seal := strings.TrimSpace(string(copies[hireGuardSealFile])); seal == guardSeal(recordKey(), copies[hireGuardFile]) {
					t.Error("the guard's copy fits the key")
				}
			} else if seal, ok := sealOfFile(r.record, copies[r.record]); ok && seal.fits(recordKey()) {
				t.Errorf("the copy of %s fits the key", r.record)
			}
		})
	}
}

// RT126SK1R2 N2: Reset's confirm says exactly what it does, in the records' state: after a check that passed only
// keys go; after a failed one, what the check dropped is lost for good (and, for a key that couldn't be read,
// restarting once the keychain is unlocked loses nothing). The page asks the core's words, not its own.
func TestSetupsResetConfirmSaysWhatAResetLoses(t *testing.T) {
	loses := "Every phone and Mac pairs again, each hire gets a new key, and what the failed check dropped is lost for good: " +
		"who may write to whom, your security settings (the front desk, hiring and the webhook too), any guard group you " +
		"switched off, and the General Manager and any hire held to a project, whom you'll hire again."
	for _, c := range []struct {
		state string
		stale bool
		want  string
	}{
		{recordsOK, true, "Reset Smolcorp's records? Every phone and Mac pairs again and each hire and the General Manager get " +
			"new keys when they next start, while your team, who may write to whom, your settings and your guard stay as they are."},
		{recordsChanged, false, "Reset Smolcorp's records? " + loses},
		{recordsInterrupted, false, "Reset Smolcorp's records? " + loses},
		{recordsUnreadable, false, "Reset Smolcorp's records instead of unlocking the keychain and restarting, which loses nothing " +
			"you saved before? " + loses},
		{recordsOK, false, ""},
		{recordsOff, false, ""},
	} {
		if got := recordsResetConfirm(c.state, c.stale); got != c.want {
			t.Errorf("%q stale %v: the confirm asks %q", c.state, c.stale, got)
		}
		if said := recordsText(c.state, c.stale); (said == "") != (c.want == "") {
			t.Errorf("%q stale %v: Setup says %q, with a confirm of %q", c.state, c.stale, said, c.want)
		}
	}
	if said := recordsText(recordsUnreadable, false); !strings.Contains(said, "restart Smolcorp: that loses nothing") {
		t.Errorf("Setup doesn't say restarting loses nothing: %q", said)
	}
	// RT126SK1R2 nit 1: a stop in the middle of a tightening can leave the change unmade after a reset.
	if said := recordsText(recordsInterrupted, false); !strings.Contains(said, "check after the reset that it's still gone") {
		t.Errorf("Setup doesn't say to check what was just taken away: %q", said)
	}
	page, err := os.ReadFile(filepath.Join("..", "Resources", "web", "setup.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "const ask = state.recordsReset || '';\n    if (ask && confirm(wording(ask)))") ||
		strings.Contains(string(page), "Only keys are cleared") {
		t.Error("setup.html's Reset doesn't confirm with the core's words")
	}
}

// RT126SK1R2 N2: after a failed check, a Reset keeps what Setup says it keeps and loses what it says it loses: the
// owner's looser settings and guard group are gone, the guard in force is sealed (the built-in one with what the
// file adds), and nothing is dropped again at the next start.
func TestOnAMacAResetAfterAFailedCheckLosesWhatItSays(t *testing.T) {
	d, mac, _, _ := macDeck(t)
	withAHire(t)
	d.mu.Lock()
	d.config.KeystrokesAllowed, d.config.Listen = true, "all"
	d.config.save()
	d.mu.Unlock()
	guardPath := filepath.Join(supportDir, hireGuardFile)
	if err := holdGuard(guardPath, []byte(`{"groups":[{"name":"push","on":false},{"name":"mine","on":true,"commands":["rm -rf"]}]}`)); err != nil {
		t.Fatal(err)
	}
	mac.recordItems = append(mac.recordItems, newRecordItem("key-planted"))
	failed := restart(t)
	state := pageState(t, failed)
	if ask, _ := state["recordsReset"].(string); ask != recordsResetConfirm(recordsChanged, false) || !strings.Contains(ask, "lost for good") {
		t.Errorf("the confirm asks %q", ask)
	}
	if code, _, _ := fromPage(t, failed, http.MethodPost, "/setup/records/reset", nil); code != http.StatusOK {
		t.Fatalf("reset: %d", code)
	}
	again := restart(t)
	if state := recordsState(); state != recordsOK {
		t.Fatalf("after the reset: %q", state)
	}
	if said := recordsDroppedText(); said != "" {
		t.Errorf("a record is dropped at the start after the reset: %q", said)
	}
	again.mu.Lock()
	keys, listen := again.config.KeystrokesAllowed, again.config.Listen
	again.mu.Unlock()
	f := readRegistry(registryPath(), false)
	if keys || listen == "all" || len(f.contacts) != 0 || len(f.hires) != 2 {
		t.Errorf("after the reset: keystrokes %v, listen %q, %d contacts, %d hires", keys, listen, len(f.contacts), len(f.hires))
	}
	groups := heldGuardGroups(guardPath)
	push := slices.IndexFunc(groups, func(g guard.Group) bool { return g.Name == "push" })
	mine := slices.IndexFunc(groups, func(g guard.Group) bool { return g.Name == "mine" })
	if heldGuard(guardPath).data == nil || push < 0 || !groups[push].On || mine < 0 || !groups[mine].On {
		t.Errorf("the guard after the reset: %+v", groups)
	}
	if _, problem := loadGuard(guardPath); problem != "" {
		t.Errorf("Settings reads the guard after the reset as: %s", problem)
	}
}

// RT126SK1R3 N3: while the records' key can't be read, nothing is written: a desk started, a setting changed, the
// guard set in Settings or a Mac paired then leaves every sealed file as it was (the desk's key and the setting in
// memory for the run, the guard and the Mac refused with Setup's reason). So once the owner unlocks the keychain and
// restarts, as Setup says, nothing is lost: the contacts, the GM, the settings and the guard from before all count,
// and no record is dropped.
func TestOnAMacWhileTheKeyIsUnreadableARestartLosesNothing(t *testing.T) {
	guardPath := func() string { return filepath.Join(supportDir, hireGuardFile) }
	loose := []byte(`{"groups":[{"name":"push","on":false}]}`)
	rows := map[string]func(t *testing.T, d *Deck){
		"a desk started": func(t *testing.T, d *Deck) {
			if key, err := newSendKey(registryPath(), "hire02"); err != nil || key == "" || !isHireKey(registryPath(), key) {
				t.Errorf("the desk's new send key: %q %v", key, err)
			}
		},
		"a setting changed": func(t *testing.T, d *Deck) {
			d.mu.Lock()
			d.config.Logo, d.config.Listen = "keys", "all"
			d.config.save()
			listen := d.config.Listen
			d.mu.Unlock()
			if listen != "all" {
				t.Errorf("the setting isn't held for the run: %q", listen)
			}
		},
		"the guard set": func(t *testing.T, d *Deck) {
			w := serve(t, d, call{method: http.MethodPost, target: "/setup/guard", host: "localhost:7710", from: thisComputer,
				body: `{"groups":[{"name":"push","on":true}]}`, headers: map[string]string{setupHeader: "1"}})
			if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), recordsHeldAnswer) {
				t.Errorf("the guard while the key can't be read: %d %s", w.Code, w.Body)
			}
		},
		"a Mac paired": func(t *testing.T, d *Deck) {
			d.mu.Lock()
			d.config.FloorsFromOtherMacs = true
			key := d.macPeers.pair("Laptop")
			d.mu.Unlock()
			if key != "" {
				t.Error("a Mac was paired while the key can't be read")
			}
			if w := peerPage(t, d, http.MethodPost, "/setup/peers/code", ""); w.Code != http.StatusConflict ||
				!strings.Contains(w.Body.String(), recordsHeldAnswer) {
				t.Errorf("a code for a Mac while the key can't be read: %d %s", w.Code, w.Body)
			}
		},
	}
	for name, step := range rows {
		t.Run(name, func(t *testing.T) {
			d, mac, phoneKey, _ := macDeck(t)
			withAHire(t)
			frontDeskMu.Lock()
			file := readRegistry(registryPath(), true)
			file.hires = append(file.hires, Hire{ID: "gm0001", Name: "GM", Role: gmRole, Folder: homeDir, State: "working"})
			err := file.save(registryPath())
			frontDeskMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			d.mu.Lock()
			d.config.KeystrokesAllowed, d.config.Listen = true, "all"
			d.config.save()
			d.mu.Unlock()
			if err := holdGuard(guardPath(), loose); err != nil {
				t.Fatal(err)
			}
			before := sealedCopies(t)
			mac.recordLook = "unreadable"
			held := restart(t)
			if said, _ := pageState(t, held)["records"].(string); !strings.Contains(said, "restart Smolcorp: that loses nothing") {
				t.Fatalf("Setup says: %q", said)
			}
			step(t, held)
			if after := sealedCopies(t); !maps.EqualFunc(before, after, bytes.Equal) {
				t.Errorf("a sealed file was written while the key can't be read: %v", slices.Sorted(maps.Keys(after)))
			}
			mac.recordLook = ""
			again := restart(t)
			if state := recordsState(); state != recordsOK {
				t.Fatalf("after the unlocked restart: %q", state)
			}
			if said := recordsDroppedText(); said != "" {
				t.Errorf("a record is dropped after the unlocked restart: %q", said)
			}
			f := readRegistry(registryPath(), false)
			gm := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == "gm0001" })
			again.mu.Lock()
			keys, listen, phone := again.config.KeystrokesAllowed, again.config.Listen, again.access.Phones.WithKey(phoneKey) != nil
			again.mu.Unlock()
			if len(f.contacts) != 1 || gm < 0 || !isOpen(f.hires[gm]) || !keys || listen != "all" || !phone ||
				!bytes.Equal(heldGuard(guardPath()).data, loose) {
				t.Errorf("after the unlocked restart: %d contacts, GM %d, keystrokes %v, listen %q, phone %v, guard %s",
					len(f.contacts), gm, keys, listen, phone, heldGuard(guardPath()).data)
			}
		})
	}
}

// RT126SK1R3 N4: a registry.json the core can't read (here, one a newer Smolcorp wrote) is never written over as the
// core starts, though the drop owes a rotation then, nor by a Reset: it stays where it is, and is set aside whole as
// registry.broken.json at the next edit, to be mended.
func TestOnAMacARegistryThatCantBeReadIsKeptForMending(t *testing.T) {
	newer := func(t *testing.T) []byte {
		t.Helper()
		withAHire(t)
		rewrite(t, registryName, func(f map[string]any) { f["version"] = 99 })
		data, _ := os.ReadFile(registryPath())
		return data
	}
	aside := func(t *testing.T, want []byte) {
		t.Helper()
		if data, err := os.ReadFile(filepath.Join(supportDir, "registry.broken.json")); err != nil || !bytes.Equal(data, want) {
			t.Errorf("registry.broken.json isn't the file that couldn't be read: %v %s", err, data)
		}
	}
	t.Run("the start, then an edit", func(t *testing.T) {
		macDeck(t)
		data := newer(t)
		id := recordKeyID()
		restart(t)
		if recordKeyID() == id {
			t.Error("the drop rotated nothing")
		}
		if now, _ := os.ReadFile(registryPath()); !bytes.Equal(now, data) {
			t.Fatalf("the start wrote over registry.json: %s", now)
		}
		frontDeskMu.Lock()
		f := readRegistry(registryPath(), true)
		f.hires = append(f.hires, Hire{ID: "hire09", Name: "Di", Folder: homeDir, State: "working"})
		err := f.save(registryPath())
		frontDeskMu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		aside(t, data)
	})
	t.Run("a reset", func(t *testing.T) {
		_, mac, _, _ := macDeck(t)
		data := newer(t)
		mac.recordItems = append(mac.recordItems, newRecordItem("key-planted"))
		failed := restart(t)
		if code, _, _ := fromPage(t, failed, http.MethodPost, "/setup/records/reset", nil); code != http.StatusOK {
			t.Fatalf("reset: %d", code)
		}
		aside(t, data)
	})
}

// RT126SK1R3 nit 1: a guard dropped at a start is sealed as the guard in force (the built-in one with what the file
// adds), so the next start drops nothing and rotates nothing. RT126SK1R4 follow-up 1: only by the rotation the drop
// owes, under its new key. A stop before that rotation's add (the core killed while the Keychain makes the new key:
// the files as they are at "add", the Keychain's items as they were before it) leaves the guard broken, so the next
// start drops it again; a stop once the guard is written under the new key leaves two items, an interrupted update.
func TestOnAMacADroppedGuardIsntDroppedAgain(t *testing.T) {
	path := func() string { return filepath.Join(supportDir, hireGuardFile) }
	// broken is a support folder whose guard's seal doesn't fit, under a deck started as the app starts it.
	broken := func(t *testing.T) *macStandIn {
		t.Helper()
		_, mac, _, _ := macDeck(t)
		if err := holdGuard(path(), []byte(`{"groups":[{"name":"push","on":false},{"name":"mine","on":true,"commands":["make release"]}]}`)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(guardSealPath(path()), []byte(strings.Repeat("0", 64)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return mac
	}
	// stoppedAt is the start that drops the guard, stopped at the rotation's first step named at: the files as they are
	// then, put back, and the Keychain's items as they were before the start (beforeAdd) or as they are then.
	stoppedAt := func(t *testing.T, mac *macStandIn, at string, beforeAdd bool) {
		t.Helper()
		mac.mu.Lock()
		items := slices.Clone(mac.recordItems)
		mac.mu.Unlock()
		var files map[string][]byte
		saved := rotationStep
		rotationStep = func(step string) {
			if step == at && files == nil {
				files = sealedCopies(t)
				if !beforeAdd {
					mac.mu.Lock()
					items = slices.Clone(mac.recordItems)
					mac.mu.Unlock()
				}
			}
		}
		restart(t)
		rotationStep = saved
		if files == nil {
			t.Fatalf("the start didn't rotate through %q", at)
		}
		for _, name := range append(sealedRecordNames(), hireGuardFile, hireGuardSealFile) {
			if data, ok := files[name]; ok {
				_ = os.WriteFile(filepath.Join(supportDir, name), data, 0o600)
			} else {
				_ = os.Remove(filepath.Join(supportDir, name))
			}
		}
		mac.mu.Lock()
		mac.recordItems = items
		mac.mu.Unlock()
	}
	t.Run("the next start", func(t *testing.T) {
		broken(t)
		restart(t)
		if said := recordsDroppedText(); !strings.Contains(said, hireGuardFile) {
			t.Fatalf("the broken guard isn't dropped: %q", said)
		}
		id := recordKeyID()
		restart(t)
		if said := recordsDroppedText(); said != "" || recordKeyID() != id {
			t.Errorf("the next start: dropped %q, rotated %v", said, recordKeyID() != id)
		}
		groups := heldGuardGroups(path())
		push := slices.IndexFunc(groups, func(g guard.Group) bool { return g.Name == "push" })
		mine := slices.IndexFunc(groups, func(g guard.Group) bool { return g.Name == "mine" })
		if push < 0 || !groups[push].On || mine < 0 || !groups[mine].On {
			t.Errorf("the guard held: %+v", groups)
		}
	})
	t.Run("a stop before the rotation's add", func(t *testing.T) {
		mac := broken(t)
		stoppedAt(t, mac, "add", true)
		restart(t)
		if state, said := recordsState(), recordsDroppedText(); state != recordsOK || !strings.Contains(said, hireGuardFile) {
			t.Errorf("the start after the stop: %q, dropped %q", state, said)
		}
	})
	t.Run("a stop after the guard is written under the new key", func(t *testing.T) {
		mac := broken(t)
		stoppedAt(t, mac, "wrote "+guardRecord(path()), false)
		restart(t)
		if state := recordsState(); state != recordsInterrupted {
			t.Errorf("the start after the stop: %q", state)
		}
	})
}

// recordKeyID is the record key's item now: a rotation changes it.
func recordKeyID() string {
	records.mu.Lock()
	defer records.mu.Unlock()
	return records.id
}
