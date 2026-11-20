package desktop

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The end of a plain hire that works in a worktree of its own (FXHIREPROJECTSR8, RT126FIXREVIEW-HIREPROJECTSR7 N2). On
// a Mac with its sandbox, a plain hire works in its own worktree, on its own branch hire/<desk>/… (hire.go, ruling
// (W)), but it has no registry row, so nothing fired it: its worktree and branch stayed, and no recap said where its
// work was. Now each one the office makes is kept on record (plainHireRecord), and once its launcher is gone, whichever
// way it ended (its fire from the office, which closes its window; its window closed by the owner; its agent ended; its
// launcher killed, or gone while Smolcorp was stopped), it gets the engine's fire's clean-up (endPlainHire):
//   - its worktree is taken away only when it is the hire's own, clean, and its launcher gone (removeOwnWorktree, the
//     checks the engine's fire makes), and its branch only once your checkout's HEAD holds it (removeWorkspace);
//   - its recap is written, with its branch and whether its worktree was kept (writeRecap);
//   - a worktree with work in it stays, on its branch, for you to merge and remove.
//
// A launcher whose stream dropped connects again within a few seconds, without its token: one in the hire's worktree is
// taken for its window (plainDeskOf), and the end waits for it to go too.

// plainHiresSwept is whether a new deck ends, as it starts, the plain hires whose launcher went while it was stopped:
// the app's always does. The tests' decks don't (isolate): a test ends its own.
var plainHiresSwept = true

// plainHireGrace is how long a plain hire's launcher may be gone before its hire is ended: a launcher whose stream
// dropped connects again within a few seconds. A variable, for the tests.
var plainHireGrace = 3 * reconnectDelay

// plainHiresFolder is where the plain hires with a worktree of their own are kept on record, one file each, named for
// its desk: in the support folder, which no desk writes but its *.md files and its Claude folder.
func plainHiresFolder() string { return filepath.Join(supportDir, "plain-hires") }

// keepPlainHire keeps h, a plain hire with a worktree of its own (its desk's id, plainDeskID's shape), on record until
// its end.
func keepPlainHire(h Hire) error {
	if !plainDeskID(h.ID) {
		return os.ErrInvalid
	}
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(plainHiresFolder(), 0o700); err != nil {
		return err
	}
	return writePrivate(filepath.Join(plainHiresFolder(), h.ID+".json"), data)
}

// plainHireRecords are the plain hires on record: each file named for a plain desk, whose record is that desk's.
func plainHireRecords() []Hire {
	entries, err := os.ReadDir(plainHiresFolder())
	if err != nil {
		return nil
	}
	var hires []Hire
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !plainDeskID(id) || !e.Type().IsRegular() {
			continue
		}
		var h Hire
		if data, err := os.ReadFile(filepath.Join(plainHiresFolder(), e.Name())); err != nil || json.Unmarshal(data, &h) != nil || h.ID != id {
			log.Printf("hire: plain hire %s's record can't be read", id)
			continue
		}
		hires = append(hires, h)
	}
	return hires
}

// plainHireRecord is plain desk id's record, if it has one.
func plainHireRecord(id string) (Hire, bool) {
	for _, h := range plainHireRecords() {
		if h.ID == id {
			return h, true
		}
	}
	return Hire{}, false
}

// plainDeskOf is the plain desk whose worktree holds folder, a launcher's, from the records: how a launcher that
// connected again without its token is known for that desk's window. "" for none.
func plainDeskOf(folder string) string {
	if folder == "" {
		return ""
	}
	where := physical(folder)
	for _, h := range plainHireRecords() {
		if !h.Worktree.none() && within(physical(h.Worktree.storedOnly()), where) { // compared only
			return h.ID
		}
	}
	return ""
}

// plainDeskHeld is whether a connected launcher is plain hire h's window: the one its token made, or one in its
// worktree. With the lock held.
func (d *Deck) plainDeskHeld(h Hire) bool {
	tree := physical(h.Worktree.storedOnly()) // compared only
	for _, tab := range d.launchers {
		if tab.plainDesk == h.ID || tab.folder != "" && within(tree, physical(tab.folder)) {
			return true
		}
	}
	return false
}

// plainHireLeft is said once plain desk id's launcher is gone: after plainHireGrace, with no launcher of it connected
// again, its hire is ended.
func (d *Deck) plainHireLeft(id string) {
	d.rounds.after(plainHireGrace, func() {
		h, ok := plainHireRecord(id)
		if !ok {
			return
		}
		d.mu.Lock()
		held := d.plainDeskHeld(h)
		d.mu.Unlock()
		if !held {
			d.endPlainHire(h)
		}
	})
}

// sweepPlainHires ends, once as the core starts, every plain hire on record whose launcher is gone: one that ended
// while Smolcorp was stopped. A launcher still running connects again within plainHireGrace, so the sweep waits that
// long first.
func (d *Deck) sweepPlainHires() {
	d.rounds.after(plainHireGrace, func() {
		for _, h := range plainHireRecords() {
			d.mu.Lock()
			held := d.plainDeskHeld(h)
			d.mu.Unlock()
			if !held {
				d.endPlainHire(h)
			}
		}
	})
}

// endPlainHire ends plain hire h, whose launcher is gone, as the engine's fire ends a hire: its worktree removed when
// it is the hire's own and clean, its branch when your checkout's HEAD holds it, its recap written. Then it is off
// the record. One being ended already is left to that end.
func (d *Deck) endPlainHire(h Hire) {
	firing.Lock()
	if firing.ids[h.ID] {
		firing.Unlock()
		return
	}
	firing.ids[h.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()
	if _, ok := plainHireRecord(h.ID); !ok {
		return // ended meanwhile
	}
	d.mu.Lock()
	worktrees := d.worktreesFolder("")
	d.mu.Unlock()
	hasTree := !h.Worktree.none()
	removed := hasTree && removeOwnWorktree(runGit, worktrees, h)
	recap := writeRecapMoved(h, "", removed, hasTree && !removed && hireTreeMoved(h), time.Now())
	if err := os.Remove(filepath.Join(plainHiresFolder(), h.ID+".json")); err != nil {
		log.Printf("hire: plain hire %s's record: %v", h.ID, err)
	}
	if !removed && hasTree {
		log.Printf("hire: plain hire %s ended; its work is on %s, in %s (recap %s)", h.ID, h.Branch, h.Worktree.storedOnly(), recap)
	}
}
