package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// The update to 1.26 from an earlier Smolcorp, as Setup says it (UPGRADE1; SANDBOX-PLAN §3.9, §A5.3). The first
// start of 1.26 on a support folder changes every key once (changeKeysOnce). When the folder already held an office
// from before (a 1.24 install: any of olderOfficeFiles), that start also notes when it ran and what it let go, in the
// same marker file (keys-changed-1.26), and Start here says it in one line, "Updated to Smolcorp 1.26":
//   - what was kept: the team, who may write to whom, the Claude accounts, the office and the settings;
//   - what reset: every key; the phones and Macs paired before, to pair again; any record that fell back to its safe
//     defaults at that start, in the words it was dropped with (recordDropped); a GM started by hand, which needs a
//     new token;
//   - the hires from before the update, by name, to fire and hire again: they still run as they started, with keys
//     that no longer work and without 1.26's protections, and Smolcorp never stops a desk by itself (§3.9).
//
// Typing over Wi-Fi stays off after the update, as a new setup has it (config.go): 1.24 had no switch for it, its
// Linux and Windows app no phone text at all. While the update's line shows, a second one says so and where to turn
// it on, with the phone's text on and the switch still off.
//
// The line is a step To do while a hire from before is open, and says what happened, for updateNoteDays, after
// that. It grants nothing and takes nothing away: a program that rewrites the note changes only what Setup says of
// the update, never a setting, a key or who is let in; the hires to hire again are read from the registry the core
// holds, not from the note; and deleting the note has every key changed once more, as it always had.

// updateNoteDays is how long Start here keeps the line once no hire from before is open.
const updateNoteDays = 14

// olderOfficeFiles are what a support folder holds once an earlier Smolcorp has run in it (1.24 writes its setup key
// and hook key at every start, the Mac's Swift engine its hook header): any of them there at the first start of 1.26
// makes that start the update. Not config.json, which a new install's Mac app may write before its core first starts.
// A folder whose config.json 1.26's core already wrote (newerConfigFields) isn't one: its marker was deleted to change
// every key again, as keys-and-guard.md says to.
var olderOfficeFiles = []string{"setup-key", "hook-key", "hook-header", "token", "registry.json", "phones.json", "mac-peers.json", "sessions.json"}

// newerConfigFields are in every config.json 1.26's core writes (Typing over Wi-Fi is always saved, a record's seal
// on a Mac) and in no 1.24's, on either engine.
var newerConfigFields = []string{"phoneTextOverWiFi", sealedPartField}

// keysChangedText is the marker's first line, as it has always been.
const keysChangedText = "Every key Smolcorp keeps was changed once here, by 1.26. Delete this file to change them all again.\n"

// updateNote is what the update's start found, kept as one line of JSON after the marker's text.
type updateNote struct {
	At     int64    `json:"at"`               // when the update's start ran, in milliseconds since 1970, as a hire's Started
	Phones int      `json:"phones,omitempty"` // phones paired before, let go by the key change
	Macs   int      `json:"macs,omitempty"`   // Macs paired before, let go too
	Reset  []string `json:"reset,omitempty"`  // what fell back to its safe defaults at that start, in Setup's words
}

// updating is the update's note while its start runs (changeKeysOnce to settleRecords): nil at any other start.
var updating struct {
	sync.Mutex
	note *updateNote
}

// noteUpdate is changeKeysOnce's look at the folder dir before any key changes: the note of an update from an
// earlier Smolcorp, or nil for a folder nothing ran in before.
func noteUpdate(dir string, now time.Time) *updateNote {
	if !slices.ContainsFunc(olderOfficeFiles, func(name string) bool {
		_, err := os.Lstat(filepath.Join(dir, name))
		return err == nil
	}) || writtenBy126(filepath.Join(dir, "config.json")) {
		return nil
	}
	return &updateNote{At: now.UnixMilli(), Phones: pairedIn(dir, "phones.json"), Macs: pairedIn(dir, "mac-peers.json")}
}

// writtenBy126 is whether the config file at path is one 1.26's core wrote, read as the guard reads its file.
func writtenBy126(path string) bool {
	data, err := guard.ReadFile(path)
	if err != nil {
		return false
	}
	var fields map[string]json.RawMessage
	return json.Unmarshal(data, &fields) == nil && slices.ContainsFunc(newerConfigFields, func(name string) bool {
		_, ok := fields[name]
		return ok
	})
}

// pairedIn is how many devices the file name in dir lists, plain as an earlier Smolcorp wrote it, or sealed: the
// Windows window's own paired device isn't a phone. Read as the guard reads its file, so a pipe or a link in its
// place can't stall the start.
func pairedIn(dir, name string) int {
	data, err := guard.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return 0
	}
	if seal, sealed := sealOfFile(name, data); sealed {
		data = seal.body
	}
	// Only the names: the Mac's Swift engine wrote its dates as numbers.
	var listed []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &listed) != nil {
		return 0
	}
	n := 0
	for _, device := range listed {
		if !(PairedPhone{Name: device.Name}).isWindowDevice() {
			n++
		}
	}
	return n
}

// writeKeysChanged writes the marker at path: its text, and the update's note when there is one.
func writeKeysChanged(path string, note *updateNote) error {
	text := keysChangedText
	if note != nil {
		data, err := json.Marshal(note)
		if err != nil {
			return err
		}
		text += string(data) + "\n"
	}
	return writePrivate(path, []byte(text))
}

// readUpdateNote is the update's note in the marker, or nil: a folder 1.26 started new, or no marker.
func readUpdateNote() *updateNote {
	data, err := guard.ReadFile(filepath.Join(supportDir, recordsRotatedFile))
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		var note updateNote
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &note) == nil && note.At > 0 {
			return &note
		}
	}
	return nil
}

// updateFellBack keeps what a record that didn't count at the update's start said (recordDropped): Setup says it
// with the update, after that start's own line is gone. A record that says nothing is named.
func updateFellBack(name, text string) {
	if text == "" {
		text = name + " didn't fit, so what it granted is dropped."
	}
	updating.Lock()
	defer updating.Unlock()
	if updating.note != nil && !slices.Contains(updating.note.Reset, text) {
		updating.note.Reset = append(updating.note.Reset, text)
	}
}

// settleUpdateNote ends the update's start (settleRecords): the note is written again with what fell back, if
// anything did.
func settleUpdateNote() {
	updating.Lock()
	note := updating.note
	updating.note = nil
	updating.Unlock()
	if note != nil && len(note.Reset) > 0 {
		_ = writeKeysChanged(filepath.Join(supportDir, recordsRotatedFile), note)
	}
}

// hiresFromBefore are the open hires the update found, as Setup names them: started before it (at, in
// milliseconds), with no hook key of their own, which every desk 1.26 starts is given (hookkeys.go).
func hiresFromBefore(file registryFile, at int64) []string {
	var names []string
	for _, h := range file.hires {
		if !isOpen(h) || file.hookKeys[h.ID] != "" || int64(h.Started) >= at {
			continue
		}
		switch {
		case h.Role == gmRole:
			names = append(names, "the General Manager")
		case h.Nickname != "":
			names = append(names, h.Nickname)
		default:
			names = append(names, h.Name)
		}
	}
	return names
}

// updateCheck is Start here's line about the update to 1.26, or nil: no update from an earlier Smolcorp here, or
// updateNoteDays after it with no hire from before still open. Without the lock.
func (d *Deck) updateCheck(now time.Time) *check {
	note := readUpdateNote()
	if note == nil {
		return nil
	}
	before := hiresFromBefore(readRegistry(registryPath(), false), note.At)
	if len(before) == 0 && now.Sub(time.UnixMilli(note.At)) > updateNoteDays*24*time.Hour {
		return nil
	}
	d.mu.Lock()
	byHand := d.config.GMByHand
	d.mu.Unlock()
	c := check{ID: "update", Status: checkInfo, Title: updateTitle, Detail: updateWords(*note, byHand, before)}
	if len(before) > 0 {
		c.Status = checkTodo
	}
	return &c
}

// updateChecks are Start here's lines about the update: updateCheck's and, while it shows, one offering Typing over
// Wi-Fi when the phone's text is on and that switch off. Without the lock.
func (d *Deck) updateChecks(now time.Time) []check {
	update := d.updateCheck(now)
	if update == nil {
		return nil
	}
	checks := []check{*update}
	d.mu.Lock()
	offer := d.config.PhoneMicEnabled && !d.config.PhoneTextWiFi
	d.mu.Unlock()
	if offer {
		checks = append(checks, check{ID: "update-wifityping", Status: checkInfo, Title: updateWiFiTitle, Detail: updateWiFiWords})
	}
	return checks
}

// What Start here says of the update, in this order (updateWords). The CI's upgrade from 1.24.2 looks for these
// words in `smolcorp setup --check` (scripts/upgrade).
const (
	updateTitle   = "Updated to Smolcorp 1.26"
	updateKept    = "Kept as they were: your team and its worktrees, who may write to whom, your Claude accounts, your office and your settings."
	updateKeptBut = "Kept as they were, except as said below: your team and its worktrees, who may write to whom, your Claude accounts, your office and your settings."
	updateKeys    = "Every key Smolcorp keeps was changed once, so links and keys from before the update no longer work."
	updateByHand  = "A General Manager you start by hand needs a new token: start it again with smolcorp gm login."
	updateRehire  = "Re-hire your hires from 1.24 (%s): they still run as they started, with keys that no longer work and without 1.26's protections. Fire each one and hire it again. Smolcorp never stops a desk by itself."
	updatePhone   = "The phone paired before the update was let go: pair it again."
	updatePhones  = "The %d phones paired before the update were let go: pair each one again."
	updateMac     = "The Mac paired with this one before the update was let go: pair it again."
	updateMacs    = "The %d Macs paired with this one before the update were let go: pair each one again."
	// The line offering Typing over Wi-Fi (updateChecks), true on every system: since the update a phone types over
	// Tailscale only, where 1.24 typed over the Wi-Fi too (a Mac) or not at all (Linux and Windows).
	updateWiFiTitle = "Typing over Wi-Fi is off"
	updateWiFiWords = "Since the update, a paired phone types to agents over Tailscale only, as a new setup has it. Its keys still work over the Wi-Fi. To type over the Wi-Fi too, turn on Typing over Wi-Fi in Setup, under Paired phones: anyone on this Wi-Fi could then read what you type. The Tailscale link stays private."
)

// updateWords is the line's detail: what was kept, what reset and what's left to do, in plain words.
func updateWords(note updateNote, byHand bool, before []string) string {
	words := []string{updateKept, updateKeys}
	if len(note.Reset) > 0 {
		words[0] = updateKeptBut
	}
	words = append(words, countWords(note.Phones, updatePhone, updatePhones)...)
	words = append(words, countWords(note.Macs, updateMac, updateMacs)...)
	words = append(words, note.Reset...)
	if byHand {
		words = append(words, updateByHand)
	}
	if len(before) > 0 {
		words = append(words, fmt.Sprintf(updateRehire, andList(before)))
	}
	return strings.Join(words, " ")
}

// countWords is one for n == 1, many with n in it for more, nothing for none.
func countWords(n int, one, many string) []string {
	switch {
	case n == 1:
		return []string{one}
	case n > 1:
		return []string{fmt.Sprintf(many, n)}
	}
	return nil
}

// andList is names as a sentence lists them: "Ada", "Ada and Bo", "Ada, Bo and Cy".
func andList(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
