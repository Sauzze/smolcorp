package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// What's held for you from hires' Claude plants (CLAUDEPLANT1, sandboxstart.go), kept across a restart (SIERRA, 15
// Nov): claude-held.json in the support folder, a sealed record (records.go). On a Mac it carries the seal; one
// another program changed, or that can't be read, counts for nothing and Setup names it (recordDropped). Then no
// desk has a baseline: an open hire's place is listed whole again against your checkout at the next gate, and a gone
// hire's too, since none is on record as listed (claudeGate). Off a Mac it counts as found, as every record there.
// It's read at the core's start (settleRecords), or the first time it's asked for, and from then on only written: at
// that start, so it's there from the first desk on (each desk's settings deny what the support folder holds), then at
// each listing that changes it, Remove and Keep, under a new record key when more is held.

// claudeHeldName is the record's file, in the support folder.
const claudeHeldName = "claude-held.json"

// claudeHeldMost is the longest record read: a longer one isn't, and counts for nothing.
const claudeHeldMost = 16 << 20

// claudeHeld is what's held, by ID, each desk's baseline and what you kept, by path, and the gone desks on record as
// listed once their session ended (claudeHireEnded), as read from file. With them, the listings asked for and not
// done yet, by desk (claudePending, for the record pendingOf names), how many were asked for, the one worker that
// lists them (its number, 0 for none; workers counts them), and done, closed each time one is done or it stops.
var claudeHeld = struct {
	sync.Mutex
	file      string
	plants    map[string]claudePlant
	baselines map[string]map[string]claudePlant
	kept      map[string]claudePlant
	ended     map[string]bool

	pending         map[string]claudePending
	pendingOf       string
	asked           uint64
	worker, workers uint64
	done            chan struct{}
}{}

// claudeHeldRecord is the record as its file holds it.
type claudeHeldRecord struct {
	Held      []claudePlant            `json:"held"`
	Baselines map[string][]claudePlant `json:"baselines"`
	Kept      []claudePlant            `json:"kept"`
	Ended     []string                 `json:"ended"`
	Pending   []claudePending          `json:"pending,omitempty"`
}

// A hire's listing is no longer made in the registry's save, under frontDeskMu, nor at the gate under the Deck's lock
// (FXCLAUDESUMOPENR3 B, SIERRA's ruling on FXCLAUDESUMOPENR2's proposal): a big place takes seconds to list. The save
// asks for it, with a mark on this record (claudePending), and lets the lock go; one worker lists what's asked for,
// one hire at a time, with the walk and bounds of claudeWalk, and only then takes the mark off. Until then, your
// Claude Code isn't started in, above or below that place: Smolcorp says it's still checking, and a start you asked
// for goes ahead by itself once it's done (Deck.startOnceChecked). A mark the save made is on record, so a quit or a
// crash before it's done leaves the place refused, never trusted, until the next start lists it again.

// claudePending is a listing asked for and not done yet: desk's place, against reference (its checkout; what
// listClaudePlantsOf lists), and whether its session is known to have ended (claudeHireEnded: once it's done, the
// desk is on record as listed). Only one a save asked for is on record (onRecord). ask is which asking it is: a
// listing that started before it was asked for again doesn't take it off.
type claudePending struct {
	Desk      string `json:"desk"`
	Place     string `json:"place"`
	Reference string `json:"reference,omitempty"`
	Ended     bool   `json:"ended,omitempty"`

	onRecord bool
	ask      uint64
}

// claudeCheckWait is how long a gate, or a look at what's held, waits for a listing still asked for in or around its
// folder before it answers that Smolcorp is still checking. claudeListingWaitMost is how long a fire waits for its
// own, and a start for the listing it was held for: past what one may take (claudeWalk's bounds, about 30 s), and
// then the place stays refused until it's done. Tests set them.
var (
	claudeCheckWait       = 3 * time.Second
	claudeListingWaitMost = 2 * time.Minute
)

// claudeListingStarts is told the desk whose place the worker is about to list. Tests set it, to hold the worker
// there, as a quit or crash in the middle would.
var claudeListingStarts = func(desk string) {}

// loadClaudeHeldLocked reads the record, once for each support folder (tests change it). The caller holds claudeHeld.
func loadClaudeHeldLocked() {
	file := filepath.Join(supportDir, claudeHeldName)
	if claudeHeld.file == file {
		return
	}
	claudeHeld.file = file
	claudeHeld.plants, claudeHeld.baselines, claudeHeld.kept = map[string]claudePlant{}, map[string]map[string]claudePlant{}, map[string]claudePlant{}
	claudeHeld.ended = map[string]bool{}
	claudeHeld.pending, claudeHeld.pendingOf, claudeHeld.worker = map[string]claudePending{}, file, 0 // a worker from before lists for nothing
	defer kickClaudeListingsLocked()
	data, err := readMost(file, claudeHeldMost)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	var rec claudeHeldRecord
	body, counts := []byte(nil), false
	if err == nil {
		body, counts = openBody(claudeHeldName, data)
	}
	if !counts || json.Unmarshal(body, &rec) != nil {
		recordDropped(claudeHeldName, claudeHeldName+" was changed by another program, or can't be read: before it starts "+
			"your Claude Code, Smolcorp looks again at what hires left named .claude or .mcp.json, and holds what isn't as in your checkout.")
		return
	}
	for _, p := range rec.Held {
		claudeHeld.plants[p.ID] = p
	}
	for desk, found := range rec.Baselines {
		claudeHeld.baselines[desk] = map[string]claudePlant{}
		for _, p := range found {
			claudeHeld.baselines[desk][p.File] = p
		}
	}
	for _, p := range rec.Kept {
		claudeHeld.kept[p.File] = p
	}
	for _, desk := range rec.Ended {
		claudeHeld.ended[desk] = true
	}
	for _, p := range rec.Pending { // asked for before a quit or crash, and not done: listed again now
		if p.Desk != "" && p.Place != "" {
			claudeHeld.asked++
			p.onRecord, p.ask = true, claudeHeld.asked
			claudeHeld.pending[p.Desk] = p
		}
	}
}

// saveClaudeHeldLocked writes the record as held now (records.go's keepRecord): under a new record key when tighten.
// The caller holds claudeHeld, so writes keep their order.
func saveClaudeHeldLocked(tighten bool) {
	_ = keepRecord(claudeHeldName, claudeHeldWriterLocked(), tighten)
}

// claudeHeldWriterLocked writes the record as held now, sealed under the key it's given. The caller holds claudeHeld.
func claudeHeldWriterLocked() recordWriter {
	byFile := func(m map[string]claudePlant) []claudePlant {
		out := slices.Collect(maps.Values(m))
		slices.SortFunc(out, func(a, b claudePlant) int { return strings.Compare(a.File+"\x00"+a.ID, b.File+"\x00"+b.ID) })
		return out
	}
	rec := claudeHeldRecord{Held: byFile(claudeHeld.plants), Baselines: map[string][]claudePlant{}, Kept: byFile(claudeHeld.kept),
		Ended: slices.Sorted(maps.Keys(claudeHeld.ended))}
	for _, desk := range slices.Sorted(maps.Keys(claudePendingLocked())) {
		if p := claudeHeld.pending[desk]; p.onRecord {
			rec.Pending = append(rec.Pending, p)
		}
	}
	for desk, found := range claudeHeld.baselines {
		rec.Baselines[desk] = byFile(found)
	}
	file := claudeHeld.file
	return func(key []byte, prev string) error {
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return err
		}
		return writePrivate(file, sealBody(key, prev, claudeHeldName, data))
	}
}

// markClaudeListed puts gone desk on record as listed once its session ended (claudeHireEnded), if it isn't yet. A
// baseline alone isn't that: any listing makes one.
func markClaudeListed(desk string) {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	if !claudeHeld.ended[desk] {
		claudeHeld.ended[desk] = true
		saveClaudeHeldLocked(false)
	}
}

// claudeListed is whether desk is on record as listed once its session ended, once a listing asked for it is done
// (up to claudeCheckWait).
func claudeListed(desk string) bool {
	waitClaudeListings(func(p claudePending) bool { return p.Desk == desk }, claudeCheckWait)
	return claudeListedNow(desk)
}

// claudeListedNow is claudeListed as the record says now.
func claudeListedNow(desk string) bool {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	return claudeHeld.ended[desk]
}

// settleClaudeHeld reads the record again as the core starts (settleRecords), so one that doesn't fit is named, and
// writes it as held, under the key of the moment, keeping its writer for the rotations to come.
func settleClaudeHeld() {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	claudeHeld.file = ""
	loadClaudeHeldLocked()
	saveClaudeHeldLocked(false)
}

// resealClaudeHeld writes the record as held now, under the key of the moment (Setup's reset).
func resealClaudeHeld() {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	saveClaudeHeldLocked(false)
}

// claudePendingLocked is the listings asked for and not done yet, for the record of the moment (tests change it).
// The caller holds claudeHeld.
func claudePendingLocked() map[string]claudePending {
	if claudeHeld.pending == nil || claudeHeld.pendingOf != claudeHeld.file {
		claudeHeld.pending, claudeHeld.pendingOf, claudeHeld.worker = map[string]claudePending{}, claudeHeld.file, 0
	}
	return claudeHeld.pending
}

// askClaudeListingLocked asks for a listing of h's place against its checkout (ended: its session is known to have
// ended), on record when onRecord. One asked for again while it's being listed is listed again after. It says
// whether it asked: a hire with no place has nothing to list. The caller holds claudeHeld, and writes the record.
func askClaudeListingLocked(h Hire, ended, onRecord bool) bool {
	if h.place() == "" {
		return false
	}
	pending := claudePendingLocked()
	was := pending[h.ID]
	claudeHeld.asked++
	pending[h.ID] = claudePending{Desk: h.ID, Place: h.place(), Reference: h.Repo, Ended: was.Ended || ended,
		onRecord: was.onRecord || onRecord, ask: claudeHeld.asked}
	kickClaudeListingsLocked()
	return true
}

// askClaudeListings asks for a listing of each of hires' places, as askClaudeListingLocked, and gives the desks
// asked for. Asked on record, the record is written once for all, before anything else is: under frontDeskMu,
// that write is all the registry's save adds (registryFile.save).
func askClaudeListings(hires []Hire, ended, onRecord bool) []string {
	claudeHeld.Lock()
	defer claudeHeld.Unlock()
	loadClaudeHeldLocked()
	var asked []string
	for _, h := range hires {
		if askClaudeListingLocked(h, ended, onRecord) {
			asked = append(asked, h.ID)
		}
	}
	if onRecord && len(asked) > 0 {
		saveClaudeHeldLocked(false)
	}
	return asked
}

// kickClaudeListingsLocked starts the worker if anything is asked for and none runs. The caller holds claudeHeld.
func kickClaudeListingsLocked() {
	if claudeHeld.worker != 0 || len(claudePendingLocked()) == 0 {
		return
	}
	claudeHeld.workers++
	claudeHeld.worker = claudeHeld.workers
	claudeWorkers.Add(1)
	go func(n uint64) {
		defer claudeWorkers.Done()
		listClaudePending(n)
	}(claudeHeld.worker)
}

// claudeWorkers are the workers running, the one listing and any a new record left behind. Tests wait for them.
var claudeWorkers sync.WaitGroup

// claudeListingDoneLocked tells whoever waits that a listing is done, or the worker stopped. The caller holds
// claudeHeld.
func claudeListingDoneLocked() {
	if claudeHeld.done != nil {
		close(claudeHeld.done)
		claudeHeld.done = nil
	}
}

// listClaudePending is worker n: it lists what's asked for, the first asked first, one at a time, until nothing is
// left, without claudeHeld while it walks. What it found is held and becomes the desk's baseline, as at any listing
// (holdClaudePlants); the mark comes off only if the desk wasn't asked for again meanwhile, and the desk is put on
// record as listed if its session was known to have ended when it was asked for. A worker the record was read again
// behind (a new start, the tests' new record) stops, and what it was listing counts for nothing.
func listClaudePending(n uint64) {
	for {
		claudeHeld.Lock()
		pending := claudePendingLocked()
		if claudeHeld.worker != n {
			claudeHeld.Unlock()
			return
		}
		var next claudePending
		for _, p := range pending {
			if next.Desk == "" || p.ask < next.ask {
				next = p
			}
		}
		if next.Desk == "" {
			claudeHeld.worker = 0
			claudeListingDoneLocked()
			claudeHeld.Unlock()
			return
		}
		claudeHeld.Unlock()
		claudeListingStarts(next.Desk)
		found := walkClaudePlants([]claudeLook{{path: next.Place, depth: -1}}, next.Reference)
		claudeHeld.Lock()
		if claudeHeld.worker != n || claudeHeld.pendingOf != claudeHeld.file {
			claudeHeld.Unlock()
			return
		}
		listed, changed := recordClaudePlantsLocked(next.Desk, found)
		if p, ok := claudeHeld.pending[next.Desk]; ok && p.ask == next.ask {
			delete(claudeHeld.pending, next.Desk)
			changed = changed || p.onRecord
			if p.Ended && !claudeHeld.ended[next.Desk] {
				claudeHeld.ended[next.Desk], changed = true, true
			}
		}
		if changed || len(listed) > 0 {
			saveClaudeHeldLocked(len(listed) > 0)
		}
		claudeListingDoneLocked()
		claudeHeld.Unlock()
	}
}

// waitClaudeListings waits, up to wait, until no listing still asked for is one still says, and says whether none is
// left. It holds no lock while it waits.
func waitClaudeListings(still func(claudePending) bool, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for {
		claudeHeld.Lock()
		loadClaudeHeldLocked()
		left := false
		for _, p := range claudePendingLocked() {
			left = left || still(p)
		}
		if left {
			kickClaudeListingsLocked()
		}
		if claudeHeld.done == nil {
			claudeHeld.done = make(chan struct{})
		}
		done := claudeHeld.done
		claudeHeld.Unlock()
		remaining := time.Until(deadline)
		if !left || remaining <= 0 {
			return !left
		}
		timer := time.NewTimer(remaining)
		select {
		case <-done:
		case <-timer.C:
		}
		timer.Stop()
	}
}

// waitClaudeListed waits, up to claudeListingWaitMost, until desks are listed as asked: a fire goes on once what its
// hire left is held for you, as when the listing was in its save.
func waitClaudeListed(desks []string) {
	if len(desks) > 0 {
		waitClaudeListings(func(p claudePending) bool { return slices.Contains(desks, p.Desk) }, claudeListingWaitMost)
	}
}
