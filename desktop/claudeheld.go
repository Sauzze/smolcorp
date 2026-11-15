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
// listed once their session ended (claudeHireEnded), as read from file.
var claudeHeld = struct {
	sync.Mutex
	file      string
	plants    map[string]claudePlant
	baselines map[string]map[string]claudePlant
	kept      map[string]claudePlant
	ended     map[string]bool
}{}

// claudeHeldRecord is the record as its file holds it.
type claudeHeldRecord struct {
	Held      []claudePlant            `json:"held"`
	Baselines map[string][]claudePlant `json:"baselines"`
	Kept      []claudePlant            `json:"kept"`
	Ended     []string                 `json:"ended"`
}

// loadClaudeHeldLocked reads the record, once for each support folder (tests change it). The caller holds claudeHeld.
func loadClaudeHeldLocked() {
	file := filepath.Join(supportDir, claudeHeldName)
	if claudeHeld.file == file {
		return
	}
	claudeHeld.file = file
	claudeHeld.plants, claudeHeld.baselines, claudeHeld.kept = map[string]claudePlant{}, map[string]map[string]claudePlant{}, map[string]claudePlant{}
	claudeHeld.ended = map[string]bool{}
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

// claudeListed is whether desk is on record as listed once its session ended.
func claudeListed(desk string) bool {
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
