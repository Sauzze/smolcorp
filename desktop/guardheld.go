package desktop

import (
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
// (off a Mac, Q7, and on a Mac until SK1 hands the app's record key in), the file the core finds at its start
// is the copy it holds.

// hireGuardSealFile is the guard file's seal, beside it.
const hireGuardSealFile = "hire-guard.seal"

// guardRecordKey is the record key guard seals are made with: nil when the core has none (see above). SK1 sets it.
var guardRecordKey = func() []byte { return nil }

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

// guardSeal is the seal of a guard file's bytes under key.
func guardSeal(key, data []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
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
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		held.readErr = err
	default:
		held.data = data
		if key := guardRecordKey(); key != nil {
			seal, _ := os.ReadFile(guardSealPath(path))
			if !hmac.Equal([]byte(strings.TrimSpace(string(seal))), []byte(guardSeal(key, data))) {
				held.data, held.unsealed = nil, true
			}
		}
	}
	heldGuards.byPath[path] = held
	return *held
}

// holdGuard writes data as the guard file at path, with its seal when there is a record key, and holds it.
// Nothing is held unless the file is written.
func holdGuard(path string, data []byte) error {
	if err := writeBriefPrivately(path, data); err != nil {
		return err
	}
	if key := guardRecordKey(); key != nil {
		if err := writePrivate(guardSealPath(path), []byte(guardSeal(key, data)+"\n")); err != nil {
			return err
		}
	}
	heldGuards.Lock()
	heldGuards.byPath[path] = &heldGuardFile{data: data}
	heldGuards.Unlock()
	return nil
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
