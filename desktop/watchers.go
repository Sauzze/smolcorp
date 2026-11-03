package desktop

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// What the GM asks the app to keep an eye on, kept in watchers.json in the
// support folder so a watch outlasts the GM's restarts and the app's. A port of
// the Mac's Watchers (Sources/AgentDeck/Watchers.swift); the wire formats are
// Tests/fixtures/frontdesk/watch.json and watches.json.
//
// The kinds:
//   - file: a new line matching a pattern in a file inside a folder the app
//     manages (a hire's folder or worktree, or the support hires folder);
//   - event: a line of that kind in the event log;
//   - idle: a hire's folder has been quiet since its turn ended, for N minutes;
//   - exit: a hire was fired or failed, or is gone from the registry;
//   - time: N minutes pass, once, or every N minutes.
//
// Only NEW lines fire. A firing is a line in the event log (kind watcherFired),
// which gm inbox reads. No mail is sent to wake the GM (the Mac has no way wired
// either). The app's own watches (the Mac's built-ins) are in
// watchers_builtin.go; they are checked and worded in the same places as the others.
//
// Locking: w.mu guards the watches and what is known of the event log, and is
// never held while a file is read, the registry is read, or another lock is
// taken, except the event log's (a leaf lock) when a firing is written down.

const (
	watchersFileVersion = 1
	// How often files and timers are looked at.
	watchInterval = 5 * time.Second
	// The most one look reads of a file, and the longest line looked at.
	watchReadMost = 1 << 20
	watchLineMost = 4000
	watchTextMost = 500
	watchesMost   = 50
	// watchMinutesMost is a year in minutes: more would make a due time that can't be saved.
	watchMinutesMost = 525600
	watchTailSize    = 64
)

// watchKinds are the kinds a watch may be, in the order the Mac lists them.
var watchKinds = []string{"file", "event", "idle", "exit", "time"}

// eventKindsWatchable are the event kinds an event watch may name (the Mac's
// EventLog.Kind without watcherFired, in its order).
var eventKindsWatchable = []string{"sessionStart", "sessionStop", "prompt", "permissionAsked", "permissionDenied", "notification", "idle", "mail", "keystroke"}

// privateWatchFolders hold keys, settings or history: never watched, wherever they sit (Watchers.privateFolders).
var privateWatchFolders = []string{".ssh", ".gnupg", ".aws", ".git", ".config"}

// watch is one watch, with the Mac's field names (Watchers.Watch).
type watch struct {
	ID   string
	Kind string
	// file: the full path. event: the event kind. idle, exit: the hire (its id for exit, its folder for idle).
	// time: a few words the GM wants back.
	Target string
	// A regular expression: matched against each new line, or an event's detail.
	Pattern string
	// event: only for events in this folder or under it.
	Folder string
	// idle, time: how many minutes.
	Minutes   float64
	Repeats   bool
	By        string // "gm" or "owner"
	CreatedAt float64
	FiredAt   float64
	Fired     int
	// time: when it is next due, in ms.
	Due float64
	// file: how far it has read, and the file's inode (0 while it has not been there).
	Offset int
	Inode  uint64
	// file: a hash of the (up to 64) bytes just before Offset, so a file rewritten in place is told from one only added to. 0: not known.
	Tail uint64
	// idle: the event log line it last fired for, so one quiet spell fires once.
	LastEvent int64
	// The app's own watch: what it is for, else "" (watchers_builtin.go).
	Builtin string
	// A built-in's hire (its id).
	Hire string
}

// active is whether it still watches: fired once, it is done (and kept, fired, till taken away) unless it repeats.
func (w watch) active() bool { return w.Repeats || w.FiredAt == 0 }

// watchFields is the file's spelling of a watch: "repeat" in and out, as POST takes it.
type watchFields struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Target    string  `json:"target"`
	Pattern   string  `json:"pattern"`
	Folder    string  `json:"folder"`
	Minutes   float64 `json:"minutes"`
	Repeats   bool    `json:"repeat"`
	By        string  `json:"by"`
	CreatedAt float64 `json:"createdAt"`
	FiredAt   float64 `json:"firedAt"`
	Fired     int     `json:"fired"`
	Due       float64 `json:"due"`
	Offset    int     `json:"offset"`
	Inode     uint64  `json:"inode"`
	Tail      uint64  `json:"tail"`
	LastEvent int64   `json:"lastEvent"`
	Builtin   string  `json:"builtin"`
	Hire      string  `json:"hire"`
}

func (w watch) MarshalJSON() ([]byte, error) { return json.Marshal(watchFields(w)) }

// UnmarshalJSON is field by field, so a file kept before a field was added
// still reads, and a field of the wrong type is a zero one. An id and a known
// kind are needed (Watch.init(from:)).
func (w *watch) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var out watch
	if json.Unmarshal(raw["id"], &out.ID) != nil || out.ID == "" || json.Unmarshal(raw["kind"], &out.Kind) != nil || !slices.Contains(watchKinds, out.Kind) {
		return fmt.Errorf("a watch needs an id and a kind")
	}
	for key, into := range map[string]any{
		"target": &out.Target, "pattern": &out.Pattern, "folder": &out.Folder, "by": &out.By, "builtin": &out.Builtin, "hire": &out.Hire,
		"minutes": &out.Minutes, "createdAt": &out.CreatedAt, "firedAt": &out.FiredAt, "due": &out.Due,
		"fired": &out.Fired, "offset": &out.Offset, "lastEvent": &out.LastEvent, "inode": &out.Inode, "tail": &out.Tail, "repeat": &out.Repeats,
	} {
		// A value of the wrong type is left as it was: zero.
		_ = json.Unmarshal(raw[key], into)
	}
	*w = out
	return nil
}

// watchFire is what a check says happened, before it is written down.
type watchFire struct {
	id     string
	folder string
	detail string
	// builtinReviewMark for a NEEDS-REVIEW line, else "" (Watchers.Fire.mark).
	mark string
	// A hire's DONE line: whose, for the digest.
	done string
	// The digest: how many finished hires it told of.
	told int
}

// watchLook is where a file watch stands after a look: the lines that are new.
type watchLook struct {
	offset int
	inode  uint64
	tail   uint64
	lines  []string
}

type watchers struct {
	mu   sync.Mutex
	file string
	all  []watch
	// roots are the folders a file watch may look inside; hires is the registry.
	// Both are called with w.mu let go.
	roots func() []string
	hires func() []Hire
	// log adds a line to the event log (the GM's inbox).
	log   func(eventEntry) eventEntry
	now   func() time.Time
	newID func() string
	home  func() string
	// The latest event of each folder, for idle, and the idle event that began each folder's quiet spell.
	latest    map[string]eventEntry
	idleSince map[string]eventEntry
	checking  bool
	// sync sets and takes away the app's own watches; called at the start of each look (nil: none).
	sync func()
	// asking is the folders with a session at a question now (nil: none); called with w.mu let go.
	asking func() []string
	// askingSince is when each of those was first seen asking, in ms; doneSince is the hires that finished since the last digest.
	askingSince map[string]float64
	doneSince   []string
	// builtinsOn is whether the app's own watches are wanted now (nil: no).
	builtinsOn func() bool
}

func newWatchers(folder, home string, roots func() []string, hires func() []Hire, log func(eventEntry) eventEntry, seed []eventEntry) *watchers {
	_ = os.MkdirAll(folder, 0o700)
	w := &watchers{
		file: filepath.Join(folder, "watchers.json"), roots: roots, hires: hires, log: log,
		now: time.Now, newID: randomWatchID, home: func() string { return home },
		latest: map[string]eventEntry{}, idleSince: map[string]eventEntry{},
	}
	w.all = loadWatchers(w.file)
	for _, e := range seed {
		w.note(e)
	}
	return w
}

// randomWatchID is "w" and eight hex digits, as the Mac's.
func randomWatchID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("no secure random source available")
	}
	return "w" + hex.EncodeToString(b)
}

// ---- Setting and taking away

// add is POST /setup/gm/watch. by is "gm" or "owner".
func (w *watchers) add(given map[string]any, by string) (watch, *refusal) {
	text := func(key string) string { s, _ := given[key].(string); return oneLineLimit(s, watchTextMost) }
	kind := strings.ToLower(text("kind"))
	if !slices.Contains(watchKinds, kind) {
		return watch{}, &refusal{http.StatusBadRequest, "Say a kind: " + strings.Join(watchKinds, ", ") + "."}
	}
	if w.userWatches() >= watchesMost {
		return watch{}, &refusal{http.StatusConflict, fmt.Sprintf("That is %d watches, the most. Take one away first.", watchesMost)}
	}
	now, newID := w.clock()
	ms := math.Round(float64(now().UnixMilli()))
	made := watch{ID: newID(), Kind: kind, By: by, CreatedAt: ms, Pattern: text("pattern"), Target: text("target")}
	made.Repeats, _ = given["repeat"].(bool)
	if made.Pattern != "" && compileWatchPattern(made.Pattern) == nil {
		return watch{}, &refusal{http.StatusBadRequest, "That pattern isn't a regular expression."}
	}
	minutes, _ := given["minutes"].(float64)

	switch kind {
	case "file":
		if made.Pattern == "" {
			return watch{}, &refusal{http.StatusBadRequest, "Say the pattern to look for."}
		}
		path := expandTilde(made.Target, w.home())
		if !filepath.IsAbs(path) && text("hire") != "" {
			// A short path is inside the hire's folder.
			hire, refused := findWatchHire(text("hire"), w.hires())
			if refused != nil {
				return watch{}, refused
			}
			place, refused := watchFolder(hire)
			if refused != nil {
				return watch{}, refused
			}
			path = filepath.Join(place, path)
		}
		if !filepath.IsAbs(path) {
			return watch{}, &refusal{http.StatusBadRequest, "Give the file as a full path, or a path inside a hire's folder with \"hire\"."}
		}
		inside, base, ok := watchInsideRoot(path, w.roots(), w.home())
		if !ok {
			return watch{}, &refusal{http.StatusForbidden, "Only files inside a hire's folder or the hires folder can be watched."}
		}
		if hardLinked(base, inside) {
			// A second name for a file that may be somewhere else (a key, say) isn't a file of the hire's.
			return watch{}, &refusal{http.StatusForbidden, "Only files inside a hire's folder or the hires folder can be watched."}
		}
		made.Target = inside
		// What is there now never fires: start from its end.
		if size, inode, tail, ok := watchFileState(base, inside); ok {
			made.Offset, made.Inode, made.Tail = size, inode, tail
		}
	case "event":
		if !slices.Contains(eventKindsWatchable, made.Target) {
			return watch{}, &refusal{http.StatusBadRequest, "Say which event: " + strings.Join(eventKindsWatchable, ", ") + "."}
		}
		if folder := text("folder"); folder != "" {
			made.Folder = filepath.Clean(expandTilde(folder, w.home()))
		}
	case "idle":
		if minutes <= 0 {
			return watch{}, &refusal{http.StatusBadRequest, "Say how many \"minutes\" quiet."}
		}
		if minutes > watchMinutesMost {
			return watch{}, &refusal{http.StatusBadRequest, "Say how many \"minutes\" (a year at most)."}
		}
		made.Minutes = minutes
		hire, refused := findWatchHire(made.Target, w.hires())
		if refused != nil {
			return watch{}, refused
		}
		if made.Target, refused = watchFolder(hire); refused != nil {
			return watch{}, refused
		}
		// A quiet spell already running was not set by this watch.
		w.mu.Lock()
		made.LastEvent = w.latest[made.Target].ID
		w.mu.Unlock()
	case "exit":
		hire, refused := findWatchHire(made.Target, w.hires())
		if refused != nil {
			return watch{}, refused
		}
		if !isOpen(hire) {
			return watch{}, &refusal{http.StatusConflict, hire.Name + " is already gone."}
		}
		made.Target = hire.ID
		made.Repeats = false // a hire leaves once: a repeating exit would fire at every look
	case "time":
		if minutes <= 0 {
			return watch{}, &refusal{http.StatusBadRequest, "Say how many \"minutes\" from now."}
		}
		if minutes > watchMinutesMost {
			return watch{}, &refusal{http.StatusBadRequest, "Say how many \"minutes\" (a year at most)."}
		}
		if made.Repeats && minutes < 1 {
			return watch{}, &refusal{http.StatusBadRequest, "Repeating means every minute at the soonest."}
		}
		made.Minutes = minutes
		made.Due = ms + math.Round(minutes*60_000)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	// Counted again: another request may have set one while this was read.
	if w.countUser() >= watchesMost {
		return watch{}, &refusal{http.StatusConflict, fmt.Sprintf("That is %d watches, the most. Take one away first.", watchesMost)}
	}
	w.all = append(w.all, made)
	w.save()
	return made, nil
}

// remove is POST or DELETE /setup/gm/unwatch.
func (w *watchers) remove(id string) *refusal {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := slices.IndexFunc(w.all, func(x watch) bool { return x.ID == id })
	if i < 0 {
		return &refusal{http.StatusNotFound, "No watch " + id + "."}
	}
	if w.all[i].Builtin != "" {
		return &refusal{http.StatusForbidden, "That watch is the app's own. Switch the built-in watches off in settings to stop them."}
	}
	w.all = slices.Delete(w.all, i, i+1)
	w.save()
	return nil
}

// clock is the time and the ids in use, taken with w.mu so a test can swap them.
func (w *watchers) clock() (func() time.Time, func() string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.now, w.newID
}

func (w *watchers) userWatches() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.countUser()
}

// countUser is how many watches are not the app's own. With w.mu held.
func (w *watchers) countUser() int {
	n := 0
	for _, x := range w.all {
		if x.Builtin == "" {
			n++
		}
	}
	return n
}

// watchJSON is a watch as GET /setup/gm/watches answers it: as set, without where it has read to.
func watchJSON(x watch) map[string]any {
	out := map[string]any{"id": x.ID, "kind": x.Kind, "target": x.Target, "pattern": x.Pattern, "repeat": x.Repeats,
		"by": x.By, "createdAt": int64(math.Floor(x.CreatedAt)), "fired": x.Fired, "active": x.active()}
	if x.FiredAt > 0 {
		out["firedAt"] = int64(math.Floor(x.FiredAt))
	}
	if x.Folder != "" {
		out["folder"] = x.Folder
	}
	if x.Minutes > 0 {
		out["minutes"] = x.Minutes
	}
	if x.Builtin != "" {
		out["builtin"] = x.Builtin
	}
	if x.Kind == "time" && x.active() {
		out["due"] = int64(math.Floor(x.Due))
	}
	return out
}

// list is GET /setup/gm/watches: a list, never null.
func (w *watchers) list() []map[string]any {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]map[string]any, 0, len(w.all))
	for _, x := range w.all {
		out = append(out, watchJSON(x))
	}
	return out
}

// get is the watch with this id, if it is there.
func (w *watchers) get(id string) (watch, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := slices.IndexFunc(w.all, func(x watch) bool { return x.ID == id })
	if i < 0 {
		return watch{}, false
	}
	return w.all[i], true
}

// findWatchHire is the hire a name means: its id, or its nickname or name (an
// open one first). The GM is never one (Watchers.find; its words, not
// findHire's).
func findWatchHire(name string, all []Hire) (Hire, *refusal) {
	asked := strings.ToLower(strings.TrimFunc(name, func(r rune) bool { return r == '\t' || unicode.Is(unicode.Zs, r) }))
	if asked == "" {
		return Hire{}, &refusal{http.StatusBadRequest, "Say whom, as \"target\"."}
	}
	var found, named, open []Hire
	for _, h := range all {
		if h.Role == gmRole {
			continue
		}
		if h.ID == name {
			found = append(found, h)
		}
		if strings.ToLower(h.Nickname) == asked || strings.ToLower(h.Name) == asked {
			named = append(named, h)
			if isOpen(h) {
				open = append(open, h)
			}
		}
	}
	if len(found) == 0 {
		found = named
		if len(open) > 0 {
			found = open
		}
	}
	if len(found) == 0 {
		return Hire{}, &refusal{http.StatusNotFound, "There's no hire called " + name + "."}
	}
	if len(found) > 1 {
		return Hire{}, &refusal{http.StatusConflict, "More than one hire is called that. Use the id."}
	}
	return found[0], nil
}

// watchFolder is where a hire works: its worktree, else its folder.
func watchFolder(h Hire) (string, *refusal) {
	place := h.Worktree
	if place == "" {
		place = h.Folder
	}
	if place == "" {
		return "", &refusal{http.StatusConflict, h.Name + " has no folder."}
	}
	return place, nil
}

// watchRoots are the folders a file watch may look inside (Deck.watchers' roots):
// the folders of the hires at work, except the GM's, and the support hires folder
// (hiresDir).
func watchRoots(hires []Hire, hiresDir string) []string {
	var roots []string
	for _, h := range hires {
		if h.Role == gmRole || !isOpen(h) {
			continue
		}
		if h.Worktree != "" {
			roots = append(roots, h.Worktree)
		} else {
			roots = append(roots, h.Folder)
		}
	}
	return append(roots, hiresDir)
}

// watchInside is path made plain (links followed) if it is a file inside one of
// roots, else false. A root that is the home folder, or above it, is too broad
// to be a hire's folder and counts for nothing. Never a secrets file or a keys
// folder, whatever folder it is in. Looked at again at every read, so a link
// swapped in later doesn't lead out (Watchers.inside).
func watchInside(path string, roots []string, home string) (string, bool) {
	resolved, _, ok := watchInsideRoot(path, roots, home)
	return resolved, ok
}

// watchInsideRoot is watchInside, with the root the path was found in: a file
// is only ever opened through that root, so no part of its path can lead out.
func watchInsideRoot(path string, roots []string, home string) (resolved, base string, ok bool) {
	resolved = plainPath(path)
	for _, part := range strings.Split(filepath.ToSlash(resolved), "/") {
		if strings.HasPrefix(part, ".env") || slices.Contains(privateWatchFolders, part) {
			return "", "", false
		}
	}
	house := plainPath(home)
	sep := string(filepath.Separator)
	for _, root := range roots {
		if root == "" {
			continue
		}
		base := plainPath(root)
		// A hire registered at ~ or above it would open every client's files.
		if filepath.Dir(base) == base || base == house || strings.HasPrefix(house, strings.TrimSuffix(base, sep)+sep) {
			continue
		}
		if resolved == base || strings.HasPrefix(resolved, base+sep) {
			return resolved, base, true
		}
	}
	return "", "", false
}

// plainPath is path cleaned, with the links of the part that exists followed;
// what is not there yet is kept as it is.
func plainPath(path string) string {
	path = filepath.Clean(path)
	rest, cur := "", path
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return path
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

func expandTilde(path, home string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[1:])
	}
	return path
}

// oneLineLimit drops control characters, trims blanks and keeps at most limit characters (String.oneLine).
func oneLineLimit(text string, limit int) string {
	kept := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
	kept = strings.TrimFunc(kept, func(r rune) bool { return r == ' ' || r == '\t' || unicode.Is(unicode.Zs, r) })
	if runes := []rune(kept); len(runes) > limit {
		return string(runes[:limit])
	}
	return kept
}

// ---- The file

// save writes watchers.json, readable by you alone. With w.mu held.
func (w *watchers) save() {
	data, err := json.MarshalIndent(map[string]any{"version": watchersFileVersion, "watches": w.all}, "", "  ")
	if err != nil {
		log.Printf("watchers: can't save %s: %v", w.file, err)
		return
	}
	if err := writePrivate(w.file, data); err != nil {
		log.Printf("watchers: can't save %s: %v", w.file, err)
	}
}

// loadWatchers reads the file. One that is there but can't be read is set aside
// as watchers.broken.json, as the registry's is, so the next save doesn't wipe
// what might be mended (Watchers.load).
func loadWatchers(file string) []watch {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var found struct {
		Version *int     `json:"version"`
		Watches *[]watch `json:"watches"`
	}
	if json.Unmarshal(data, &found) != nil || found.Watches == nil {
		aside := filepath.Join(filepath.Dir(file), "watchers.broken.json")
		_ = os.Remove(aside)
		_ = os.Rename(file, aside)
		return nil
	}
	return *found.Watches
}

// ---- Events

// note keeps the latest event of each folder, and where each quiet spell began.
func (w *watchers) note(e eventEntry) {
	if e.Kind == "watcherFired" || e.Folder == "" {
		return
	}
	if before, ok := w.latest[e.Folder]; ok && before.ID > e.ID {
		return
	}
	w.latest[e.Folder] = e
	if e.Kind == "idle" {
		w.idleSince[e.Folder] = e
	} else if endsIdle(e.Kind) {
		delete(w.idleSince, e.Folder)
	}
}

// endsIdle is whether a hire doing this has stopped being idle: it was given a
// prompt, a session began or ended, or it asked for (or was refused) a
// permission, which only happens while it works. A notification ("Claude is
// waiting for your input") comes about a minute after it went idle, and is not work.
func endsIdle(kind string) bool {
	switch kind {
	case "prompt", "sessionStart", "sessionStop", "permissionAsked", "permissionDenied":
		return true
	}
	return false
}

// observe is a line just added to the event log, for the watches on events.
func (w *watchers) observe(e eventEntry) {
	if e.Kind == "watcherFired" {
		return
	}
	w.mu.Lock()
	w.note(e)
	var fires []watchFire
	for _, x := range w.all {
		if !x.active() || x.Kind != "event" || x.Target != e.Kind {
			continue
		}
		if x.Folder != "" && e.Folder != x.Folder && !strings.HasPrefix(e.Folder, x.Folder+string(filepath.Separator)) {
			continue
		}
		if x.Pattern != "" && !watchMatches(compileWatchPattern(x.Pattern), e.Detail) {
			continue
		}
		detail := e.Kind
		if name := filepath.Base(e.Folder); e.Folder != "" && name != "." && name != "/" {
			detail += " in " + name
		}
		if e.Detail != "" {
			detail += ": " + e.Detail
		}
		fires = append(fires, watchFire{id: x.ID, folder: e.Folder, detail: detail})
	}
	w.apply(fires, nil)
	w.mu.Unlock()
}

// ---- Looking

// tick looks at files, timers, quiet folders and hires, files with nothing held,
// and fires what is due.
func (w *watchers) tick() {
	w.mu.Lock()
	if w.checking {
		w.mu.Unlock()
		return
	}
	w.checking = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.checking = false
		w.mu.Unlock()
	}()
	if w.sync != nil {
		w.sync() // the app's own watches are set and taken away here (Watchers.sync)
	}
	roots, hires, home := w.roots(), w.hires(), w.home()
	var asking []string
	if w.asking != nil {
		asking = w.asking()
	}
	w.mu.Lock()
	ms := float64(w.now().UnixMilli())
	// When each folder was first seen waiting; one that stopped waiting is forgotten.
	waiting := w.noteAsking(asking, ms)
	done := slices.Clone(w.doneSince)
	var watches []watch
	for _, x := range w.all {
		if x.active() {
			watches = append(watches, x)
		}
	}
	latest, spells := copyEvents(w.latest), copyEvents(w.idleSince)
	w.mu.Unlock()

	updated, fires := checkWatches(watches, roots, hires, home, latest, spells, ms, waiting, done)

	w.mu.Lock()
	w.apply(fires, updated)
	w.mu.Unlock()
}

// syncOnly is the look while the manager is off: only the app's own watches are touched.
func (w *watchers) syncOnly() {
	if w.sync != nil {
		w.sync()
	}
}

func copyEvents(from map[string]eventEntry) map[string]eventEntry {
	out := make(map[string]eventEntry, len(from))
	for k, v := range from {
		out[k] = v
	}
	return out
}

// checkWatches returns each watch as a look left it (where it has read to, its
// next time), and what fired. It touches files and nothing else (Watchers.check).
func checkWatches(watches []watch, roots []string, hires []Hire, home string, latest, idleSince map[string]eventEntry, now float64, waiting map[string]float64, done []string) (updated []watch, fires []watchFire) {
	for _, x := range watches {
		before := x
		switch x.Kind {
		case "file":
			// The path is checked again each time: a link swapped in since doesn't lead out.
			path, base, ok := watchInsideRoot(x.Target, roots, home)
			if !ok {
				break
			}
			look := lookAtFile(base, path, x.Offset, x.Inode, x.Tail)
			x.Offset, x.Inode, x.Tail = look.offset, look.inode, look.tail
			re := compileWatchPattern(x.Pattern)
			var hits []string
			for _, line := range look.lines {
				if watchMatches(re, line) {
					hits = append(hits, line)
				}
			}
			if x.Builtin != "" {
				fires = append(fires, builtinFileFires(x, path, hits, hires)...)
			} else if len(hits) > 0 {
				extra := ""
				if len(hits) > 1 {
					extra = fmt.Sprintf(" (+%d more)", len(hits)-1)
				}
				fires = append(fires, watchFire{id: x.ID, folder: filepath.Dir(path),
					detail: filepath.Base(path) + ": " + cutEventDetail(hits[0]) + extra})
			}
		case "time":
			if x.Due > 0 && now >= x.Due {
				text := x.Target
				if text == "" {
					text = "the time came"
				}
				if x.Builtin == "digest" {
					fires = append(fires, watchFire{id: x.ID, detail: digestText(hires, latest, waiting, done), told: len(done)})
				} else {
					fires = append(fires, watchFire{id: x.ID, detail: text})
				}
				// Next one counted from now, so a long sleep doesn't fire in a burst.
				if x.Repeats {
					x.Due = now + math.Round(x.Minutes*60_000)
				}
			}
		case "idle":
			if x.Builtin == "wait" {
				// A question asked and still waiting: once for each wait.
				if fire, ok := waitFire(&x, hires, waiting, now); ok {
					fires = append(fires, fire)
				}
				break
			}
			// The hire is gone: nothing will ever go idle again, so the watch is over, as a finished one is.
			if !slices.ContainsFunc(hires, func(h Hire) bool {
				place := h.Worktree
				if place == "" {
					place = h.Folder
				}
				return isOpen(h) && place == x.Target
			}) {
				x.Repeats, x.FiredAt = false, now
				break
			}
			if e, ok := quietest(x.Target, idleSince); ok && e.ID > x.LastEvent && now-float64(e.At) >= x.Minutes*60_000 {
				x.LastEvent = e.ID
				fires = append(fires, watchFire{id: x.ID, folder: x.Target,
					detail: fmt.Sprintf("%s has been idle for %d min", filepath.Base(x.Target), int(x.Minutes))})
			}
		case "exit":
			i := slices.IndexFunc(hires, func(h Hire) bool { return h.ID == x.Target })
			if i < 0 || !isOpen(hires[i]) {
				name, folder, state := x.Target, "", "not in the registry"
				if i >= 0 {
					name = hires[i].Nickname
					if name == "" {
						name = hires[i].Name
					}
					folder, state = hires[i].Folder, hires[i].State
				}
				fires = append(fires, watchFire{id: x.ID, folder: folder, detail: fmt.Sprintf("%s is gone (%s)", name, state)})
			}
		}
		if x != before {
			updated = append(updated, x)
		}
	}
	return updated, fires
}

// quietest is the latest event among the folders at or under folder.
func quietest(folder string, latest map[string]eventEntry) (eventEntry, bool) {
	var best eventEntry
	found := false
	for key, e := range latest {
		if (key == folder || strings.HasPrefix(key, folder+string(filepath.Separator))) && (!found || e.ID > best.ID) {
			best, found = e, true
		}
	}
	return best, found
}

// apply keeps what each watch now stands at, then, for each that fired, a line in
// the event log (the GM's inbox). With w.mu held.
func (w *watchers) apply(fires []watchFire, updated []watch) {
	if len(fires) == 0 && len(updated) == 0 {
		return
	}
	for _, x := range updated {
		// Taken away meanwhile, it stays away.
		if i := slices.IndexFunc(w.all, func(y watch) bool { return y.ID == x.ID }); i >= 0 {
			w.all[i] = x
		}
	}
	ms := math.Round(float64(w.now().UnixMilli()))
	for _, fire := range fires {
		i := slices.IndexFunc(w.all, func(y watch) bool { return y.ID == fire.id })
		if i < 0 || !w.all[i].active() {
			continue
		}
		w.all[i].FiredAt = ms
		w.all[i].Fired++
		detail := fmt.Sprintf("watch %s (%s): %s", fire.id, w.all[i].Kind, fire.detail)
		if w.all[i].Builtin != "" {
			// The app's own watches are worded "<builtin>: <detail>".
			detail = w.all[i].Builtin + ": " + fire.detail
		}
		if fire.done != "" {
			w.doneSince = append(w.doneSince, fire.done)
		}
		if w.all[i].Builtin == "digest" {
			w.doneSince = w.doneSince[min(fire.told, len(w.doneSince)):]
		}
		// The mark rides in agent, which a watcher's line has no other use for.
		w.log(eventEntry{At: int64(ms), Folder: fire.folder, Agent: fire.mark, Kind: "watcherFired", Detail: detail})
	}
	w.save()
}

// ---- Reading files

// openPlainFile opens path (already made plain by watchInsideRoot) through
// base, the hire's folder it was found in, or gives nil when what was opened
// isn't a plain file with one name that is the file at path. Opening through an
// os.Root walks the path one part at a time and refuses any part that leads
// out of base, so a folder swapped for a link at any moment can't lead to a
// file outside it (~/.ssh, the support folder). After the open, the file is
// checked to be the one path leads to now, and path to still be plain, which
// keeps a link swapped in inside base (to a private folder there) to a
// moment. A file with a second name (a hard link) may be one outside the hire's
// folder, so it is never read.
func openPlainFile(base, path string) (*os.File, os.FileInfo) {
	f, err := openWatchedIn(base, path)
	if err != nil {
		return nil, nil
	}
	info, err := f.Stat()
	if err == nil && info.Mode().IsRegular() && fileLinks(f, info) <= 1 {
		if at, err := os.Lstat(path); err == nil && at.Mode().IsRegular() && os.SameFile(info, at) && plainPath(path) == path {
			return f, info
		}
	}
	f.Close()
	return nil, nil
}

// openWatchedIn opens path for a look through the folder base: nothing outside
// base is ever opened, whatever links are in the way.
func openWatchedIn(base, path string) (*os.File, error) {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("not inside its folder")
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return openWatched(root, rel)
}

// hardLinked is whether path, inside base, is a file with more than one name.
func hardLinked(base, path string) bool {
	f, err := openWatchedIn(base, path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	return err == nil && fileLinks(f, info) > 1
}

// watchFileState is a file's size, inode and tail hash at its end, if it is a plain file.
func watchFileState(base, path string) (size int, inode, tail uint64, ok bool) {
	f, info := openPlainFile(base, path)
	if f == nil {
		return 0, 0, 0, false
	}
	defer f.Close()
	size = int(info.Size())
	return size, fileInode(f, info), tailHash(f, size), true
}

// tailHash is a hash (FNV-1a) of the up to watchTailSize bytes just before
// offset; 0 when there are none or they can't be read.
func tailHash(f *os.File, offset int) uint64 {
	count := min(watchTailSize, offset)
	if count <= 0 {
		return 0
	}
	buffer := make([]byte, count)
	if n, _ := f.ReadAt(buffer, int64(offset-count)); n != count {
		return 0
	}
	hash := uint64(0xcbf29ce484222325)
	for _, b := range buffer {
		hash = (hash ^ uint64(b)) * 0x100000001b3
	}
	if hash == 0 {
		return 1
	}
	return hash
}

// lookAtFile gives the new complete lines of path since offset of the file
// inode, and where the next look starts. A different file, or a shorter one, is
// taken again from its end: its old lines never fire. A file that was not there
// (inode 0) is read from its first line when it comes. A file rewritten in place
// (same inode, and as long or longer) is told by tail, the hash of the bytes
// before offset when it was last read: if they changed, it is taken again from
// its end. A last line without its newline waits for it (Watchers.look).
func lookAtFile(base, path string, offset int, inode, tail uint64) watchLook {
	unchanged := watchLook{offset: offset, inode: inode, tail: tail}
	f, info := openPlainFile(base, path)
	if f == nil {
		return unchanged
	}
	defer f.Close()
	size, now := int(info.Size()), fileInode(f, info)
	start := offset
	if inode == 0 {
		start = 0
	} else if now != inode || size < offset || (tail != 0 && tailHash(f, offset) != tail) {
		return watchLook{offset: size, inode: now, tail: tailHash(f, size)}
	}
	if size <= start {
		return watchLook{offset: start, inode: now, tail: tail}
	}
	count := min(size-start, watchReadMost)
	buffer := make([]byte, count)
	got, _ := f.ReadAt(buffer, int64(start))
	if got <= 0 {
		return watchLook{offset: start, inode: now, tail: tail}
	}
	buffer = buffer[:got]
	last := bytes.LastIndexByte(buffer, '\n')
	if last < 0 {
		// No whole line yet; a line longer than a look reads is let go.
		next := start
		if got >= watchReadMost {
			next = start + got
		}
		if next == start {
			return watchLook{offset: next, inode: now, tail: tail}
		}
		return watchLook{offset: next, inode: now, tail: tailHash(f, next)}
	}
	var lines []string
	for _, raw := range strings.Split(string(buffer[:last+1]), "\n") {
		if len(raw) > watchLineMost {
			raw = raw[:watchLineMost]
		}
		line := strings.Trim(string([]rune(raw)), "\r")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return watchLook{offset: start + last + 1, inode: now, tail: tailHash(f, start+last+1), lines: lines}
}

// ---- Patterns

// compileWatchPattern is the regular expression of a pattern, or nil when it is
// empty or isn't one. Go's syntax (RE2): no lookaround or back-references.
func compileWatchPattern(pattern string) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	return re
}

// watchMatches is whether the first watchLineMost characters of text match.
func watchMatches(re *regexp.Regexp, text string) bool {
	if re == nil {
		return false
	}
	if runes := []rune(text); len(runes) > watchLineMost {
		text = string(runes[:watchLineMost])
	}
	return re.MatchString(text)
}

// ---- The deck's

// startWatchers looks every watchInterval, from now on, while the manager is on,
// and hands the watches the event log's new lines as they come. Neither holds
// the deck's lock while a watch is touched.
func (d *Deck) startWatchers() (stop func()) {
	// Taken now, so the loops never read the deck's fields: a test may put its own in their place.
	watched, events := d.watchers, d.events
	since := func() int64 {
		events.mu.Lock()
		defer events.mu.Unlock()
		return events.lastID
	}()
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	var ended sync.WaitGroup
	ended.Add(2)
	// The event loop is not gated on the manager, on purpose: the Mac observes
	// the event log always (Deck.swift:250), so an event watch fires with the manager off too.
	go func() {
		defer ended.Done()
		for ctx.Err() == nil {
			started := time.Now()
			for _, e := range events.wait(ctx, since, eventsLimitDefault, eventsWaitMost) {
				watched.observe(e)
				since = max(since, e.ID)
			}
			if time.Since(started) < time.Second {
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
				}
			}
		}
	}()
	go func() {
		defer ended.Done()
		ticker := time.NewTicker(watchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			d.mu.Lock()
			on := d.config.ManagerEnabled
			d.mu.Unlock()
			if !on {
				// With the manager off, the app's own watches are taken away (Watchers.sync).
				watched.syncOnly()
				continue
			}
			watched.tick()
		}
	}()
	// The app never ends its deck (it lives as long as the process), so the stop
	// is called by the tests' deck helper; it waits for both loops to be gone.
	return func() {
		once.Do(cancel)
		ended.Wait()
	}
}

// watchRoutes are GET /setup/gm/watches, POST /setup/gm/watch and POST or
// DELETE /setup/gm/unwatch. The route table has already let the caller in.
func (rt *router) watchRoutes(w http.ResponseWriter, r *request, path string) {
	d := rt.deck
	switch {
	case r.Method == http.MethodGet && path == "/setup/gm/watches":
		writeJSON(w, http.StatusOK, map[string]any{"watches": d.watchers.list()})
	case r.Method == http.MethodPost && path == "/setup/gm/watch":
		data, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		var value any
		given, ok := map[string]any(nil), json.Unmarshal(data, &value) == nil
		if ok {
			given, ok = value.(map[string]any)
		}
		if !ok {
			writeStatus(w)(http.StatusBadRequest, "Send {kind, target, pattern?, minutes?, repeat?}. Kinds: file, event, idle, exit, time.")
			return
		}
		by := "owner"
		if r.caller == callerGM {
			by = "gm"
		}
		made, refused := d.watchers.add(given, by)
		if refused != nil {
			writeStatus(w)(refused.status, refused.message)
			return
		}
		writeJSON(w, http.StatusOK, watchJSON(made))
	default: // unwatch
		data, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		var given map[string]any
		_ = json.Unmarshal(data, &given)
		id, _ := given["id"].(string)
		if query := r.URL.Query(); query.Has("id") {
			id = query.Get("id")
		}
		if id == "" {
			writeStatus(w)(http.StatusBadRequest, "Say which watch, as id.")
			return
		}
		if refused := d.watchers.remove(id); refused != nil {
			writeStatus(w)(refused.status, refused.message)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "removed": true})
	}
}
