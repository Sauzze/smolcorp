package desktop

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// The owner's guard, held by the core (SANDBOX-PLAN §3.4, SG1). The core reads hire-guard.json once, as it starts
// (NewDeck), and from then on holds it: Settings' guard routes change the core's copy and write the file
// from it, and each desk gets its groups from the core's copy in the guard's own command line (--groups). The
// file, which a hire can write, then only adds to them (internal/guard/held.go). Changes a hire makes to the
// file count for nothing in the core, now or later in this run.
//
// The copy is a sealed record (§3.2): when the core has a record key (guardRecordKey), every save writes the
// file's seal beside it (HMAC-SHA-256 of its bytes under that key), and a file whose seal doesn't fit isn't
// honoured at the next start: the guard is then the built-in one, plus whatever the file adds. Without a key
// (off a Mac, Q7), the file the core finds at its start is the copy it holds. On a Mac whose record key failed its
// check (records.go), no file counts: the guard is the built-in one plus whatever the file adds.

// hireGuardSealFile is the guard file's seal, beside it.
const hireGuardSealFile = "hire-guard.seal"

// guardRecordKey is the record key guard seals are made with: nil when the core has none (see above).
var guardRecordKey = recordKey

// heldGuardFile is one guard file as the core holds it: its bytes when the core first read it or last wrote it
// (nil: none, or not honoured), and why it can't be read, if it couldn't.
type heldGuardFile struct {
	data     []byte
	readErr  error
	unsealed bool // there, but its seal didn't fit: not honoured
}

// heldGuards are the guard files the core holds, by path.
var heldGuards = struct {
	sync.Mutex
	byPath map[string]*heldGuardFile
}{byPath: map[string]*heldGuardFile{}}

// guardSeal is the seal of a guard file's bytes under key: as every record's (records.go's recordMAC), with the
// record's name in it, but of the bytes exactly as the guard reads them.
func guardSeal(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(recordSealVersion + "\x00" + hireGuardFile + "\x00\x00"))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}

// guardRecord is the name the guard file at path is kept under among the records (records.go's keepRecord).
func guardRecord(path string) string { return "guard " + path }

// guardSealWriter writes the seal of data, the guard file at path, under the key given, else guardRecordKey's:
// nothing without one (records.go's recordWriter).
func guardSealWriter(path string, data []byte) recordWriter {
	return func(key []byte, _ string) error {
		if key == nil {
			key = guardRecordKey()
		}
		if key == nil {
			return nil
		}
		return writePrivate(guardSealPath(path), []byte(guardSeal(key, data)+"\n"))
	}
}

// guardSealPath is where the seal of the guard file at path is kept.
func guardSealPath(path string) string {
	return filepath.Join(filepath.Dir(path), hireGuardSealFile)
}

// heldGuard is the guard file at path as the core holds it, read the first time it is asked for: the support
// folder's own as the core starts.
func heldGuard(path string) heldGuardFile {
	heldGuards.Lock()
	defer heldGuards.Unlock()
	if held, ok := heldGuards.byPath[path]; ok {
		return *held
	}
	held := &heldGuardFile{}
	// Read as the guard reads it (guard.ReadFile, SG0): a link, a pipe or a file over 64 KiB can't be read, at once.
	data, err := guard.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		held.readErr = err
		// The guard the core kept is dropped: its copy from before mustn't fit again (records.go, RT126SK1R2 N1).
		recordDropped(filepath.Base(path), filepath.Base(path)+" can't be read, so the guard Smolcorp kept is dropped. Set your guard again in Settings.")
	default:
		held.data = data
		if key := guardRecordKey(); key != nil || recordsState() != recordsOff {
			seal, _ := os.ReadFile(guardSealPath(path))
			if key == nil || !hmac.Equal([]byte(strings.TrimSpace(string(seal))), []byte(guardSeal(key, data))) {
				held.data, held.unsealed = nil, true
				recordDropped(filepath.Base(path), filepath.Base(path)+" was changed by another program, or its seal doesn't fit: "+
					"the guard is the built-in one, plus what the file adds. Set your guard again in Settings.")
			}
		}
	}
	heldGuards.byPath[path] = held
	return *held
}

// holdGuard writes data as the guard file at path, with its seal when there is a record key, and holds it.
// Nothing is held unless the file is written. A change to a guard the core held is sealed under a new record key
// (records.go, RT126SK1 B3), so the file from before it no longer fits.
//
// While the records' key can't be read (records.go's recordsHeldOnly) the guard is refused, file and seal unwritten:
// a guard written then would carry no seal and be dropped at the restart after the owner unlocks the keychain, with
// the guard from before (RT126SK1R3 N3).
func holdGuard(path string, data []byte) error {
	if recordsHeldOnly() {
		return errRecordsHeld
	}
	heldGuards.Lock()
	before := heldGuards.byPath[path]
	heldGuards.Unlock()
	if err := writeBriefPrivately(path, data); err != nil {
		return err
	}
	changed := before != nil && before.data != nil && !bytes.Equal(before.data, data)
	if err := keepRecord(guardRecord(path), guardSealWriter(path, data), changed); err != nil {
		return err
	}
	heldGuards.Lock()
	heldGuards.byPath[path] = &heldGuardFile{data: data}
	heldGuards.Unlock()
	return nil
}

// holdGuardUnwritten holds data as the guard file at path, in memory alone, and keeps a writer of the file and its
// seal for the rotation to come: a guard dropped at the start, sealed as the guard in force (records.go's
// settleRecords). Nothing is written under the key the drop owes a rotation for, so a stop before that rotation's
// add leaves the file broken, to be dropped again, never healed under the key its copy from before fits (RT126SK1R4
// follow-up 1).
func holdGuardUnwritten(path string, data []byte) {
	heldGuards.Lock()
	heldGuards.byPath[path] = &heldGuardFile{data: data}
	heldGuards.Unlock()
	seal := guardSealWriter(path, data)
	keepWriter(guardRecord(path), func(key []byte, prev string) error {
		if err := writeBriefPrivately(path, data); err != nil {
			return err
		}
		return seal(key, prev)
	})
}

// guardInForce is the guard in force while the file at path isn't honoured: the built-in groups with what the file
// adds (guard.Over), as a guard file, and whether the file could be read as one. Sealing it loosens nothing.
func guardInForce(path string) ([]byte, bool) {
	data, err := guard.ReadFile(path)
	if err != nil {
		return nil, false
	}
	adds, problem := guard.Parse(data)
	if problem != "" {
		return nil, false
	}
	list := []any{}
	for _, group := range guard.Over(guard.DefaultGroups(), adds) {
		commands := []string{}
		for _, rule := range group.Commands {
			commands = append(commands, string(rule))
		}
		list = append(list, map[string]any{"commands": commands, "name": group.Name, "on": group.On})
	}
	return append(swiftJSON(map[string]any{"groups": list}), '\n'), true
}

// forgetHeldGuard drops the core's copy of the guard file at path, so the next ask reads it again: for tests,
// which stand for a new start of the core.
func forgetHeldGuard(path string) {
	heldGuards.Lock()
	delete(heldGuards.byPath, path)
	heldGuards.Unlock()
}

// heldGuardGroups are the groups of the core's copy of the guard file at path, as the guard reads a file: the
// default groups when there is none, or it can't be read or honoured.
func heldGuardGroups(path string) []guard.Group {
	held := heldGuard(path)
	if held.data == nil {
		return guard.DefaultGroups()
	}
	groups, problem := guard.Parse(held.data)
	if problem != "" {
		return guard.DefaultGroups()
	}
	return groups
}
