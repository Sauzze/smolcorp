package desktop

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Sealed records (SANDBOX-PLAN §3.2, SK1). Every file that says who is let in, or what they may do, is a sealed
// record: phones.json, mac-peers.json, registry.json's permissions, config.json's security settings and the core's
// copy of the guard. On a Mac each carries an HMAC-SHA-256 of what it holds under the record key: 32 random bytes
// in one Keychain item of the Mac app's, in the login keychain opened by its path, which only the app reads and
// hands to this core over the bridge. A program running as the owner can write these files, but not seal them, so
// what it writes counts for nothing. The core reads each record once, as it starts, holds it in memory, and from
// then on only writes it.
//
// The key's item is written once and never changed. The app checks it at every start (exactly one item, its
// modification date its creation date, read without a prompt), and reads every add back once its second is over, so
// a value another program put in it as it was added is never handed on (KeychainGate.swift, RT126SK1 B1). To rotate,
// the app adds a new item, the core seals every record again under it, and only then the app deletes the old one:
// add, seal, delete. A stop between the two leaves two items, and nothing is honoured until the owner resets in
// Setup, which says "an interrupted update" when a record sealed under one item names the other as the key before.
// A planted item and a record sealed under it look the same, so that wording can be faked; what counts can't:
// nothing does, either way.
//
// The records rotate at every removal of authority (§3.2, RT126SK1 B3): a phone or a peer unpaired, a hire let go,
// a send key renewed, a contact removed, a hire's role or scope changed, a security setting changed, the guard
// changed. The change itself is written under the new key, so a copy of any record from before it no longer fits.
// They also rotate as a start ends when a record, or the guard, didn't fit (or couldn't be read) though the check
// passed (RT126SK1R2 N1): the core holds less than the file's copy from before, so what the owner sets next isn't a
// tightening of what it holds, and only a new key keeps that copy from fitting again (recordDropped).
//
// Off a Mac there is no store only Smolcorp can write (§3.8, Q7): no key, nothing is sealed, and each file counts
// as the core finds it at its start.

// The record key's states, as Setup says them (recordsText).
const (
	recordsOff         = ""            // no record key: off a Mac, or a core without the app
	recordsOK          = "ok"          // the key's item checked out: sealed records count when their seal fits
	recordsChanged     = "changed"     // the item was overwritten, or another one planted: nothing counts
	recordsInterrupted = "interrupted" // a rotation stopped between its add and its delete: nothing counts
	recordsUnreadable  = "unreadable"  // the app couldn't read the item without a prompt: nothing counts
)

// recordsRotatedFile marks that this support folder had the one-time change of every key (B1, §3.9): not a
// secret. Deleting it only has every key changed once more, and every phone paired again.
const recordsRotatedFile = "keys-changed-1.26"

// phoneSealName is the file FS2 kept the phones' seal's key in: gone since 1.26 (changeKeysOnce), and still
// fenced from hires, so one an older build left is never read.
const phoneSealName = "phones-seal"

// recordSealVersion starts every seal's message, with the record's name: a seal of one file never fits another.
const recordSealVersion = "smolcorp-record-v1"

// bridgeRecordKey is one item of the record key, as the Mac app hands it over: its id (the item's account) and its
// 32 bytes in hex.
type bridgeRecordKey struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// recordKeeper is the record key this core holds, and what its start found.
type recordKeeper struct {
	mu    sync.Mutex
	host  hostBridge
	key   []byte
	id    string // the key's item
	prev  string // the item before it, named in each file sealed since the last rotation
	state string
	// first: this start found no sealed record, so the app made a new key (an update from 1.25, or a reset);
	// rotated: this start changed every key once (B1); stale: a rotation's new key couldn't be made, so a copy of a
	// record from before it would still fit at the next start (Setup says so).
	first, rotated, stale bool
	// settled: the core's start is over and every record has its writer (settleRecords); owed: a change, or a record
	// dropped, asked for a rotation before that, done then.
	settled, owed bool
	// dropped: the records that didn't fit at this start, though the check passed (recordDropped), as Setup names them.
	dropped []string
	// writing is held through each sealed write and a whole rotation (never under mu): nothing is written under a
	// key a rotation is leaving.
	writing sync.Mutex
	// writers write each record as it was last kept, under the key and previous key given: what a rotation calls.
	writers map[string]recordWriter
}

// recordWriter writes one record, as it was when the writer was made, sealed under key (prev, the key before it,
// named in the seal); as it is, with no key.
type recordWriter func(key []byte, prev string) error

var records = &recordKeeper{}

// recordsVia is the bridge the record key is asked through: the Mac app's own. Tests put a stand-in Keychain in
// front of a stand-in app that has none.
var recordsVia = func(h hostBridge) hostBridge { return h }

// recordKey is the key records are sealed with now: nil without one, or after a check that failed.
func recordKey() []byte {
	records.mu.Lock()
	defer records.mu.Unlock()
	return records.key
}

// recordsState is the record key's state, for Setup and the loaders.
func recordsState() string {
	records.mu.Lock()
	defer records.mu.Unlock()
	return records.state
}

// recordsFailed is whether the record key failed its check as the core started: nothing sealed counts.
func recordsFailed() bool {
	state := recordsState()
	return state != recordsOff && state != recordsOK
}

// recordsHeldOnly is whether the records are held in memory alone (RT126SK1R3 N3): the app couldn't read the
// record key, so a record written now would carry no seal and be dropped at the next start, once the owner has
// unlocked the keychain, with what it held from before. Nothing is written until then: what's added or switched on
// changes in memory for this run; the guard, pairing, the GM's new token and whatever takes something away (a
// phone or Mac removed, a contact removed, any setting but a plain one switched off or changed: records_heldconfig.go)
// are refused (recordsHeldAnswer, RT126SK1R4 follow-ups 2 and 3, RT126SK1F B1), since they would seem to apply, then
// come back at that restart. A fire ends the session and takes the hire's key away for the run (hireshared.go's
// heldFire).
func recordsHeldOnly() bool { return recordsState() == recordsUnreadable }

// What a change the records can't take now is refused with, in Setup's words: while the key can't be read
// (recordsHeldOnly), and after a check that failed otherwise, until the owner resets.
const (
	recordsHeldAnswer = "Smolcorp can't read the key that seals its records from your login keychain, so it can't " +
		"save this now. Unlock the keychain and restart Smolcorp, then try again."
	recordsResetFirstAnswer = "Smolcorp's records don't count until you reset them in Setup, so this can't be saved now."
)

// errRecordsHeld is a change refused while the records are held in memory alone (recordsHeldOnly).
var errRecordsHeld = errors.New(recordsHeldAnswer)

// refusedWhileHeld answers a change that takes something away (a phone unpaired, a Mac revoked, a contact removed, a
// hire fired, a setting switched off) while the records are held in memory alone, and says whether it did: refused
// with Setup's reason, as the guard is (RT126SK1R4 follow-up 2). Held in memory, it would seem to apply, then come
// back at the restart after the owner unlocks the keychain, since the files keep what they held. While the key can
// be read it answers nothing.
func refusedWhileHeld(w http.ResponseWriter) bool {
	if !recordsHeldOnly() {
		return false
	}
	writeStatus(w)(http.StatusConflict, recordsHeldAnswer)
	return true
}

// recordsRefusal is what a change the records can't take now is refused with: "" while they can take it.
func recordsRefusal() string {
	switch state := recordsState(); {
	case state == recordsUnreadable:
		return recordsHeldAnswer
	case state != recordsOff && recordKey() == nil:
		return recordsResetFirstAnswer
	}
	return ""
}

// recordsStale is whether a rotation couldn't make its new key (recordKeeper.stale).
func recordsStale() bool {
	records.mu.Lock()
	defer records.mu.Unlock()
	return records.stale
}

// recordsFirst is whether this start sealed the records for the first time: what the files hold counts as found,
// once, except the keys, which all change (B1).
func recordsFirst() bool {
	records.mu.Lock()
	defer records.mu.Unlock()
	return records.first
}

// hostRecordKey asks the Mac app h one of the record key's actions (record-key-look, -new, -add, -retire), the
// last for the item named: its answer, the app's no as an error.
func hostRecordKey(h hostBridge, action, item string) (bridgeAnswer, error) {
	return ask(h, bridgeRequest{Action: action, Item: item})
}

// keyFromBridge is an item's 32 bytes, or nil.
func keyFromBridge(item bridgeRecordKey) []byte {
	key, err := hex.DecodeString(item.Value)
	if err != nil || len(key) != 32 || item.ID == "" {
		return nil
	}
	return key
}

// startRecords takes the record key from the Mac app h, as the core starts, before any record is read. With no
// app there is no key (recordsOff). With no sealed record on disk, the app makes a new key, deleting any item
// under its name first (a planted one goes with it). Otherwise the item it finds must check out.
//
// What the files hold counts as found, once, only at the update to 1.26 itself: no sealed record on disk and no
// item of the key in the Keychain, which no other program can bring about (it can't delete the app's item: K9).
// Sealed files deleted later only start the records over: a guard or a setting another program wrote then is
// never sealed as found.
func startRecords(h hostBridge) {
	records.mu.Lock()
	defer records.mu.Unlock()
	if h != nil {
		h = recordsVia(h)
	}
	records.host, records.key, records.id, records.prev, records.first = h, nil, "", "", false
	records.state, records.settled, records.owed, records.writers, records.dropped = recordsOff, false, false, nil, nil
	if h == nil {
		return
	}
	if !sealedOnDisk() {
		look, err := hostRecordKey(h, "record-key-look", "")
		update := err == nil && look.Records == "none"
		item := ""
		if update {
			item = "update" // the app seals its items from before as found too (KeychainGate.swift's ItemSeal)
		}
		answer, err := hostRecordKey(h, "record-key-new", item)
		if err != nil || len(answer.RecordKeys) != 1 || keyFromBridge(answer.RecordKeys[0]) == nil {
			records.state = recordsUnreadable
			return
		}
		records.key, records.id, records.state, records.first = keyFromBridge(answer.RecordKeys[0]), answer.RecordKeys[0].ID, recordsOK, update
		return
	}
	answer, err := hostRecordKey(h, "record-key-look", "")
	switch {
	case err != nil:
		records.state = recordsUnreadable
	case answer.Records == "one" && len(answer.RecordKeys) == 1 && keyFromBridge(answer.RecordKeys[0]) != nil:
		records.key, records.id, records.state = keyFromBridge(answer.RecordKeys[0]), answer.RecordKeys[0].ID, recordsOK
	case answer.Records == "many" && interruptedRotation(answer.RecordKeys):
		records.state = recordsInterrupted
	case answer.Records == "unreadable":
		records.state = recordsUnreadable
	default:
		// None at all while sealed records are there, two the app didn't leave, or one overwritten.
		records.state = recordsChanged
	}
}

// interruptedRotation is whether two items are both the app's own: a record sealed under one names the other as
// the key before it. Only the words Setup says depend on it: either way nothing counts until the owner resets.
func interruptedRotation(items []bridgeRecordKey) bool {
	if len(items) != 2 {
		return false
	}
	for _, name := range sealedRecordNames() {
		data, err := os.ReadFile(filepath.Join(supportDir, name))
		if err != nil {
			continue
		}
		seal, ok := sealOfFile(name, data)
		if !ok {
			continue
		}
		for i, item := range items {
			other := items[1-i]
			if key := keyFromBridge(item); key != nil && seal.Prev == other.ID && seal.fits(key) {
				return true
			}
		}
	}
	return false
}

// sealedRecordNames are the files that carry a seal of their own, in the support folder.
func sealedRecordNames() []string {
	return []string{"phones.json", "mac-peers.json", "registry.json", "config.json"}
}

// sealedOnDisk is whether any record in the support folder carries a seal, or the guard's seal is there.
func sealedOnDisk() bool {
	if _, err := os.Lstat(filepath.Join(supportDir, hireGuardSealFile)); err == nil {
		return true
	}
	for _, name := range sealedRecordNames() {
		if data, err := os.ReadFile(filepath.Join(supportDir, name)); err == nil {
			if _, ok := sealOfFile(name, data); ok {
				return true
			}
		}
	}
	return false
}

// recordSeal is a record's seal as a file carries it: the key before the one it was sealed with, and the HMAC.
type recordSeal struct {
	Prev string `json:"prev,omitempty"`
	MAC  string `json:"mac"`
	name string
	body []byte
}

func (s recordSeal) fits(key []byte) bool {
	return key != nil && hmac.Equal([]byte(s.MAC), []byte(recordMAC(key, s.name, s.Prev, s.body)))
}

// recordMAC is the seal of body, the record called name, under key: of its JSON as compact as it goes, so the
// indenting a file is written with never changes it.
func recordMAC(key []byte, name, prev string, body []byte) string {
	var compact bytes.Buffer
	if json.Compact(&compact, body) == nil {
		body = compact.Bytes()
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(recordSealVersion + "\x00" + name + "\x00" + prev + "\x00"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// sealedFile is a record whose whole body is sealed (phones.json, mac-peers.json).
type sealedFile struct {
	Record string          `json:"record"`
	Seal   recordSeal      `json:"seal"`
	Body   json.RawMessage `json:"body"`
}

// sealedPart is a record of which only a part is sealed (registry.json, config.json): the seal sits in the file
// beside the rest, under this field.
const sealedPartField = "recordSeal"

// sealOfFile is the seal the record called name carries, with what it covers, or false if it carries none.
func sealOfFile(name string, data []byte) (recordSeal, bool) {
	var whole sealedFile
	if json.Unmarshal(data, &whole) == nil && whole.Record == name && whole.Seal.MAC != "" {
		whole.Seal.name, whole.Seal.body = name, whole.Body
		return whole.Seal, true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields[sealedPartField] == nil {
		return recordSeal{}, false
	}
	var seal recordSeal
	if json.Unmarshal(fields[sealedPartField], &seal) != nil || seal.MAC == "" {
		return recordSeal{}, false
	}
	seal.name = name
	switch name {
	case "registry.json":
		seal.body = registrySecurityPart(readRegistryData(data))
	case "config.json":
		var config Config
		_ = json.Unmarshal(data, &config)
		seal.body = configSecurityPart(&config)
	}
	return seal, true
}

// sealBody is the file to write for the record called name whose body is body: sealed under key, or the body
// itself without one.
func sealBody(key []byte, prev, name string, body []byte) []byte {
	if key == nil {
		return body
	}
	out, err := json.MarshalIndent(sealedFile{Record: name, Seal: recordSeal{Prev: prev, MAC: recordMAC(key, name, prev, body)},
		Body: json.RawMessage(body)}, "", "  ")
	if err != nil {
		return body
	}
	return out
}

// withPartSeal is a record's file, fields, with the seal of its sealed part under key beside them (none without a
// key).
func withPartSeal(fields map[string]json.RawMessage, key []byte, prev, name string, part []byte) ([]byte, error) {
	out := maps.Clone(fields)
	if out == nil {
		out = map[string]json.RawMessage{}
	}
	if key != nil {
		out[sealedPartField], _ = json.Marshal(recordSeal{Prev: prev, MAC: recordMAC(key, name, prev, part)})
	}
	return json.MarshalIndent(out, "", "  ")
}

// openBody is the body of the record called name as data holds it, and whether it counts: without a key (off a
// Mac) whatever it holds; with one, only a body whose seal fits; after a failed check, nothing.
func openBody(name string, data []byte) ([]byte, bool) {
	state, key := recordsState(), recordKey()
	seal, sealed := sealOfFile(name, data)
	switch {
	case state == recordsOff && sealed:
		return seal.body, true
	case state == recordsOff:
		return data, true
	case sealed && seal.fits(key):
		return seal.body, true
	}
	return nil, false
}

// partCounts is whether the sealed part of the record called name, as data holds it, counts: always without a key,
// once as found at the first start, and otherwise only when its seal fits.
func partCounts(name string, data []byte) bool {
	switch recordsState() {
	case recordsOff:
		return true
	case recordsOK:
		if recordsFirst() {
			return true
		}
		seal, sealed := sealOfFile(name, data)
		return sealed && seal.fits(recordKey())
	}
	return false
}

// sayRecords says, once each, what a record that didn't count was.
var sayRecords = func(text string) { log.Printf("smolcorp: %s", text) }

var recordsSaid = struct {
	sync.Mutex
	said map[string]bool
}{said: map[string]bool{}}

func sayRecordOnce(text string) {
	recordsSaid.Lock()
	defer recordsSaid.Unlock()
	if !recordsSaid.said[text] {
		recordsSaid.said[text] = true
		sayRecords(text)
	}
}

// recordDropped says text, once, of the record called name, which didn't fit or couldn't be read as the core
// started, and was dropped. When the check passed (RT126SK1R2 N1), the record key still is the one its copy from
// before was sealed with: the records rotate as the start ends (settleRecords), so no copy under it fits again, and
// Setup names the record (recordsDroppedText). At the first sealed start the key is new already, and after a failed
// check nothing counts anyway. Only the core's own start counts: a file read again later in the run (loadConfig
// for the roles) changes nothing the core holds.
func recordDropped(name, text string) {
	if text != "" {
		sayRecordOnce(text)
	}
	records.mu.Lock()
	defer records.mu.Unlock()
	if records.state != recordsOK || records.key == nil || records.first || records.settled || slices.Contains(records.dropped, name) {
		return
	}
	records.dropped, records.owed = append(records.dropped, name), true
}

// recordWasDropped is whether the record called name was dropped at this start (recordDropped).
func recordWasDropped(name string) bool {
	records.mu.Lock()
	defer records.mu.Unlock()
	return slices.Contains(records.dropped, name)
}

// recordsDroppedText is the line Setup says of the records dropped at this start (recordDropped), "" for none.
func recordsDroppedText() string {
	records.mu.Lock()
	defer records.mu.Unlock()
	switch len(records.dropped) {
	case 0:
		return ""
	case 1:
		return "A record didn't fit and was dropped: " + records.dropped[0] + ". Another program may have changed it; its copies from before no longer count."
	}
	return "Records didn't fit and were dropped: " + strings.Join(records.dropped, ", ") + ". Another program may have changed them; their copies from before no longer count."
}

// keepRecord writes the record called name with write, under the record key of the moment, and keeps write for the
// rotations to come. A change that takes authority away (tighten) is written under a new key instead: the app adds
// it, every record is written under it, this one as changed, and only then is the old key deleted (rotate). A stop
// anywhere in between leaves either the records as they were, the change not made, or two items, which count for
// nothing until a reset. Without a key the record is written as it is, and nothing rotates; except while the key
// couldn't be read (recordsHeldOnly), when nothing is written at all: the caller's copy in memory is what the core
// goes by until it restarts, and the file keeps what it held, sealed, so that restart loses nothing (RT126SK1R3 N3).
// The caller may hold d.mu or frontDeskMu: writers take neither.
func keepRecord(name string, write recordWriter, tighten bool) error {
	records.writing.Lock()
	defer records.writing.Unlock()
	records.mu.Lock()
	if records.state == recordsUnreadable {
		records.mu.Unlock()
		return nil
	}
	key, prev := records.key, records.prev
	if key != nil {
		if records.writers == nil {
			records.writers = map[string]recordWriter{}
		}
		records.writers[name] = write
	}
	if key != nil && tighten && !records.settled {
		// Not every record has its writer yet: the rotation waits for the end of the start (settleRecords).
		records.owed, tighten = true, false
	}
	records.mu.Unlock()
	if key != nil && tighten {
		if errs, rotated := rotate(); rotated {
			return errs[name]
		}
		records.mu.Lock()
		key, prev = records.key, records.prev
		records.mu.Unlock()
	}
	return write(key, prev)
}

// keepWriter keeps write for the rotations to come, for a record the core holds but hasn't written in this run
// (settleRecords), or one written its own way (the GM's verifier).
func keepWriter(name string, write recordWriter) {
	records.mu.Lock()
	defer records.mu.Unlock()
	if records.writers == nil {
		records.writers = map[string]recordWriter{}
	}
	records.writers[name] = write
}

// rotationStep is told each step of a rotation as it's done: for the tests, which stop the core there.
var rotationStep = func(step string) {}

// rotate makes a new record key and writes every record under it, so a copy of any of them from before no longer
// fits: add, seal, delete. The app adds the new item; each record is written again; only then is the old item
// deleted. A stop between the add and the delete leaves two items, which the next start takes as an interrupted
// update. If the new key can't be made, the records stay under the old one, a copy from before still fits, and
// Setup says so (stale). Each record's error, by name, and whether it rotated. The caller holds records.writing.
func rotate() (map[string]error, bool) {
	records.mu.Lock()
	h, oldID, key := records.host, records.id, records.key
	records.mu.Unlock()
	if key == nil || h == nil {
		return nil, false
	}
	answer, err := hostRecordKey(h, "record-key-add", "")
	if err != nil || len(answer.RecordKeys) != 1 || keyFromBridge(answer.RecordKeys[0]) == nil || answer.RecordKeys[0].ID == oldID {
		records.mu.Lock()
		records.stale = true
		records.mu.Unlock()
		sayRecordOnce("A new key for Smolcorp's records couldn't be made in the Keychain: a copy of a record from before this change would still fit.")
		return nil, false
	}
	rotationStep("add")
	records.mu.Lock()
	newKey := keyFromBridge(answer.RecordKeys[0])
	records.key, records.id, records.prev = newKey, answer.RecordKeys[0].ID, oldID
	writers := maps.Clone(records.writers)
	records.mu.Unlock()
	errs := map[string]error{}
	for _, name := range slices.Sorted(maps.Keys(writers)) {
		errs[name] = writers[name](newKey, oldID)
		rotationStep("wrote " + name)
	}
	records.mu.Lock()
	records.stale = false
	records.mu.Unlock()
	if _, err := hostRecordKey(h, "record-key-retire", oldID); err != nil {
		sayRecordOnce("The old key of Smolcorp's records couldn't be deleted from the Keychain: at the next start Setup will say an update was interrupted.")
		return errs, true
	}
	rotationStep("retire")
	return errs, true
}

// rotateRecords rotates the record key with no record changed: every record is written again under a new key.
func rotateRecords() {
	records.writing.Lock()
	defer records.writing.Unlock()
	rotate()
}

// sealGuardAsFound seals the guard file as the core finds it at the first sealed start: it counted as it was before
// the update, and goes on counting.
func sealGuardAsFound() {
	path := filepath.Join(supportDir, hireGuardFile)
	if data, err := os.ReadFile(path); err == nil {
		if key := recordKey(); key != nil {
			_ = writePrivate(guardSealPath(path), []byte(guardSeal(key, data)+"\n"))
		}
	}
}

// resetRecords is Setup's reset, after a check failed: the app deletes every item of the record key and makes one,
// and every record starts over empty, sealed under it. Every phone and peer is paired again.
func resetRecords() error {
	records.mu.Lock()
	h := records.host
	records.mu.Unlock()
	if h == nil {
		return errors.New("only the Mac app keeps the records' key")
	}
	answer, err := hostRecordKey(h, "record-key-new", "")
	if err != nil || len(answer.RecordKeys) != 1 || keyFromBridge(answer.RecordKeys[0]) == nil {
		return errors.New("the Keychain didn't make a new key")
	}
	records.mu.Lock()
	records.key, records.id, records.prev, records.state = keyFromBridge(answer.RecordKeys[0]), answer.RecordKeys[0].ID, "", recordsOK
	records.stale = false
	records.mu.Unlock()
	return nil
}

// What Setup's Reset does, said beside its button and in its confirm (RT126SK1 B5, RT126SK1R2 N2). After a check
// that passed (stale) only keys go. After a failed check the core already holds the team without what the files
// granted, the security settings at their safe defaults and the guard as the built-in one plus what its file adds
// (withoutGrants, vouchConfig, heldGuard): Reset seals that, so what the check dropped is gone for good.
const (
	recordsResetKeeps = "A reset clears only keys: every phone and paired Mac pairs again, and each hire and the General " +
		"Manager get new keys when they next start. Your team, its desks and worktrees, who may write to whom, your " +
		"settings and your guard stay as they are."
	recordsResetLoses = "A reset pairs every phone and paired Mac again, and each hire gets a new key when it next starts. " +
		"Your team, its desks and worktrees stay, but what the failed check dropped is lost for good: who may write to " +
		"whom, your security settings (back to their safe defaults, with the front desk and hiring off and no webhook), " +
		"any guard group you switched off, and the General Manager and any hire held to a project, whom you'll hire again."
	// What every confirm loses after a failed check, after its question.
	recordsResetLosesInBrief = "Every phone and Mac pairs again, each hire gets a new key, and what the failed check dropped is " +
		"lost for good: who may write to whom, your security settings (the front desk, hiring and the webhook too), any " +
		"guard group you switched off, and the General Manager and any hire held to a project, whom you'll hire again."
)

// recordsResetConfirm is what Setup's confirm asks before a reset, in the state given: "" when there's no reset to
// offer (recordsText says nothing).
func recordsResetConfirm(state string, stale bool) string {
	switch {
	case state == recordsOK && stale:
		return "Reset Smolcorp's records? Every phone and Mac pairs again and each hire and the General Manager get new " +
			"keys when they next start, while your team, who may write to whom, your settings and your guard stay as they are."
	case state == recordsUnreadable:
		return "Reset Smolcorp's records instead of unlocking the keychain and restarting, which loses nothing you saved before? " +
			recordsResetLosesInBrief
	case state == recordsChanged || state == recordsInterrupted:
		return "Reset Smolcorp's records? " + recordsResetLosesInBrief
	}
	return ""
}

// recordsText is what Setup says of the records' key, "" when there's nothing to say. stale: a rotation couldn't
// make its new key, though the check passed at the start.
func recordsText(state string, stale bool) string {
	if state == recordsOK && stale {
		return "Smolcorp couldn't make a new key for its records in your login keychain after you took something away " +
			"(a phone, a Mac, a hire's key, a setting or a guard rule), so a copy of the records from before would still " +
			"count at the next start. Reset them to be sure. " + recordsResetKeeps
	}
	switch state {
	case recordsChanged:
		return "Another program changed Smolcorp's records or the key that seals them, so none of them counts: no phone, " +
			"no paired Mac, no hire's key and no General Manager is let in. Reset them, then pair your phones again. " + recordsResetLoses
	case recordsInterrupted:
		return "An update of Smolcorp's records was interrupted, so none of them counts for now: no phone, no paired Mac, " +
			"no hire's key and no General Manager is let in. Reset them, then pair your phones again. If you had just taken " +
			"something away (a phone, a Mac, a hire, a setting or a guard rule), check after the reset that it's still gone. " +
			recordsResetLoses
	case recordsUnreadable:
		// Nothing is written while the key can't be read (recordsHeldOnly, RT126SK1R3 N3), so the restart finds every
		// record as it was saved, sealed.
		return "Smolcorp couldn't read the key that seals its records from your login keychain without asking you, so none " +
			"of them counts, and it saves none until it can. What you add or switch on meanwhile (a desk's key, a hire, a " +
			"setting) lasts only until it restarts, as do changes to how the office looks. It refuses what would take " +
			"something away, since that wouldn't last either: removing a phone, a Mac, a contact or a bot, or switching off " +
			"or changing any other setting. A fire, with hiring on, ends the hire's session and stops its key until the " +
			"restart, but the hire stays on your team, unless you hired it meanwhile: that one is fired for good. Nor can " +
			"you change your guard, pair a phone or Mac, or give the General Manager a new token, so you can't hire one; " +
			"its token from before works again once you unlock the keychain and restart. Unlock the keychain and restart " +
			"Smolcorp: that loses nothing you saved before. Or reset them, and pair your phones again. " + recordsResetLoses
	}
	return ""
}

// The phones' signing secrets (signed.go) are what a phone signs with, so the core needs them whole: no verifier
// does. On a Mac phones.json keeps each one encrypted under a key made from the record key (AES-256-GCM, the phone's
// id as its associated data), so the file holds no key a program could sign with. Off a Mac there is nothing to
// encrypt it with that a program of the owner's couldn't read (§3.8): it stays in the file, readable by the owner
// alone, as the secrets the core presents stay in the system's store.

// phoneSecretsKey is the key the phones' secrets are encrypted with under the record key key: nil without one.
func phoneSecretsKey(key []byte) []byte {
	if key == nil {
		return nil
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("smolcorp phone secrets v1"))
	return mac.Sum(nil)
}

// encryptSecret is secret encrypted for the entry id, base64url, or "" if it can't be.
func encryptSecret(key []byte, id, secret string) string {
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(secret), []byte(id)))
}

// decryptSecret is what encryptSecret made for id, or "".
func decryptSecret(key []byte, id, sealed string) string {
	data, err := base64.RawURLEncoding.DecodeString(sealed)
	block, cipherErr := aes.NewCipher(key)
	if err != nil || cipherErr != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(data) < gcm.NonceSize() {
		return ""
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte(id))
	if err != nil {
		return ""
	}
	return string(plain)
}

// changeKeysOnce is the one-time change of every key (B1, §3.9), at the first start of 1.26 on any system: every
// phone and peer is paired again, every hire's send key and the GM's token stop working (a desk gets new ones when
// it starts again), the hook's key is made anew, and the files that held keys in clear go. Called before the
// records are read, so what they held is dropped as they load (recordsRotated).
func changeKeysOnce() {
	marker := filepath.Join(supportDir, recordsRotatedFile)
	_, err := os.Lstat(marker)
	records.mu.Lock()
	records.rotated = err != nil
	records.mu.Unlock()
	if err == nil {
		return
	}
	for _, name := range []string{"token", phoneSealName, "hook-key", "hook-header", keptGMTokenName} {
		_ = os.Remove(filepath.Join(supportDir, name))
	}
	_ = writePrivate(marker, []byte("Every key Smolcorp keeps was changed once here, by 1.26. Delete this file to change them all again.\n"))
}

// recordsRotated is whether this start changed every key (changeKeysOnce): the loaders drop the keys they find.
func recordsRotated() bool {
	records.mu.Lock()
	defer records.mu.Unlock()
	return records.rotated
}

// settleRecords is the end of the core's start for its records: at the one-time change of every key (B1) the
// hires' send keys go from the registry, and at the first sealed start every record is written sealed, as found
// (phones and Macs dropped already). From then on a record counts only with its seal. It also lists what writes
// each record again at a rotation. The caller is NewDeck, before anything is served.
func (d *Deck) settleRecords() {
	// Every key changed again once (the marker deleted) under a record key from before: the phones, Macs and send
	// keys the copies from before list are dropped, so the records rotate too (RT126SK1R2 N1).
	records.mu.Lock()
	if records.rotated && records.state == recordsOK && records.key != nil && !records.first {
		records.owed = true
	}
	records.mu.Unlock()
	frontDeskMu.Lock()
	if file := readRegistry(registryPath(), false); recordsRotated() || recordsFirst() {
		if recordsRotated() {
			// Unsealed before the update: nothing from before can fit, so this takes nothing away to rotate for.
			file.keys = nil
			holdRegistry(registryPath(), file)
		}
		// Not over a file that couldn't be read: it stays to be mended, and goes aside at the next edit (readRegistry).
		if _, err := os.Lstat(registryPath()); err == nil && !brokenRegistry(registryPath()) {
			_ = file.save(registryPath())
		}
	}
	frontDeskMu.Unlock()
	if recordsRotated() || recordsFirst() {
		d.access.Phones.save()
		d.macPeers.save()
	}
	records.mu.Lock()
	records.first, records.writers = false, nil
	records.mu.Unlock()
	// Every record the core holds, written again under each new key even if this run never changes it.
	if _, err := os.Lstat(phonesFile); err == nil {
		keepWriter("phones.json", d.access.Phones.writer())
	}
	if _, err := os.Lstat(d.macPeers.file); err == nil {
		keepWriter("mac-peers.json", d.macPeers.writer())
	}
	keepWriter("config.json", d.config.writer())
	frontDeskMu.Lock()
	// A file that couldn't be read is never written over by a rotation (RT126SK1R3 N4): the core holds an empty
	// registry for it, and the file is set aside as registry.broken.json at the next edit, so it can be mended.
	if _, err := os.Lstat(registryPath()); err == nil && !brokenRegistry(registryPath()) {
		keepWriter(registryName, heldRegistry(registryPath()).writer(registryPath()))
	}
	frontDeskMu.Unlock()
	guardPath := filepath.Join(supportDir, hireGuardFile)
	held := heldGuard(guardPath)
	data, inForce := []byte(nil), false
	if held.data == nil && held.unsealed && recordWasDropped(hireGuardFile) {
		data, inForce = guardInForce(guardPath)
	}
	switch {
	case inForce:
		// A guard dropped at this start is sealed as the guard in force, as Reset does (the built-in one with what the
		// file adds: it loosens nothing), so it isn't dropped again at every start (RT126SK1R3 nit 1). Only by the
		// rotation the drop owes, under its new key: a stop before that rotation's add leaves the file broken, to be
		// dropped again, and one after it two items, an interrupted update (RT126SK1R4 follow-up 1).
		holdGuardUnwritten(guardPath, data)
	case held.data != nil:
		keepWriter(guardRecord(guardPath), guardSealWriter(guardPath, held.data))
	}
	// The GM's verifier, in the Keychain item, sealed too (gmtokenstore.go).
	keepWriter(gmVerifierRecord, func([]byte, string) error { return d.gmKept.reseal() })
	records.mu.Lock()
	owed := records.owed
	records.settled, records.owed = true, false
	records.mu.Unlock()
	if owed {
		rotateRecords()
	}
}

// resetRecords is Setup's reset (records.go's resetRecords), with the keys started over (RT126SK1 B5): no phone,
// no paired Mac, no hire's send key, no GM token. The team (its hires, desks and worktrees), its contacts, the
// settings and the guard stay as the core holds them, sealed under the new key: after a failed check, that is
// without what the files granted (recordsResetLoses says so, RT126SK1R2 N2). A guard file that didn't fit is sealed
// as the guard in force: the built-in one with what the file adds, so it isn't dropped again at every start. The
// caller holds d.mu.
func (d *Deck) resetRecords() error {
	if err := resetRecords(); err != nil {
		return err
	}
	// Under a new key already: no rotation on top.
	d.access.Phones.All = nil
	d.access.Phones.save()
	for stream := range d.streams {
		stream.close()
	}
	d.macPeers.All = nil
	d.macPeers.save()
	d.gmToken = ""
	d.gmTokenChanged()
	d.config.save()
	frontDeskMu.Lock()
	// As an edit: a file that couldn't be read is set aside as registry.broken.json first, never saved over
	// (RT126SK1R3 N4).
	file := readRegistry(registryPath(), true)
	file.keys = nil
	_ = file.save(registryPath())
	frontDeskMu.Unlock()
	path := filepath.Join(supportDir, hireGuardFile)
	switch held := heldGuard(path); {
	case held.data != nil:
		_ = holdGuard(path, held.data)
	case held.unsealed:
		if data, ok := guardInForce(path); ok {
			_ = holdGuard(path, data)
		}
	}
	return nil
}
