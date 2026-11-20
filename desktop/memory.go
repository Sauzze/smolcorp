package desktop

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sauzze/smolcorp/internal/procmem"
)

// protectMemory keeps this process's memory from your other programs, a hire among them, as far as the system lets
// a process do that for itself (internal/procmem): non-dumpable on Linux, the process's access list on Windows. It
// is the first step of RunApp and RunServer, the two ways this program becomes the core, before any key exists
// here, Setup's key and the office window's among them. A variable, for the tests.
var protectMemory = func() error {
	if err := procmem.NotDumpable(); err != nil {
		return err
	}
	return procmem.DenyMemoryAccess()
}

// unprotectedMessage is what the owner is told when protectMemory fails: nothing has started, and why.
const unprotectedMessage = "Smolcorp didn't start: it couldn't keep its memory from your other programs (%v), and its keys would be in that memory. " +
	"No key was made or read. Open Smolcorp again. If this message comes back, restart your computer, then open it."

// warnOwner shows a message that matters with no terminal to read it in (instance_windows.go): a variable, for the
// tests, which never put a dialog on the screen.
var warnOwner = notifyUser

// refuseUnprotected fails closed: said in the terminal, and in a dialog on Windows, whose app has none.
func refuseUnprotected(err error) int {
	message := fmt.Sprintf(unprotectedMessage, err)
	fmt.Fprintln(os.Stderr, "smolcorp: "+message)
	warnOwner(message)
	return 1
}

// The GM's memory store (1.26, track MEM, piece MEM1; PL126M §1): what this office has taught its GM, in gm-memory/
// in the support folder, so per office, and never anywhere else. One file per note, <id>.md, with its name, its
// one-line description and its type as front matter, then its body (a feedback note's three fields: the rule, why,
// and how to apply it). Beside them, manifest.json lists every note: its id, name, description, type, state
// (waiting or kept), where its words came from, when, and the SHA-256 of its file.
//
// The manifest is a sealed record (records.go, SK1): one more name in sealedRecordNames, sealed whole under the
// record key on a Mac, read once as the core starts and held in memory, then only written. A note's file counts only
// when its hash is in the sealed manifest, so a program running as the owner can neither add a note, nor change one,
// nor bring back one that was removed: every removal rotates the record key, so a manifest copied from before no
// longer fits. Each file is read once, at the start, and held, so a change made while the core runs counts for
// nothing. While the records are held in memory alone (recordsHeldOnly), or after a check that failed, every write is
// refused with SK1's sentence (recordsRefusal).
//
// Off a Mac there is no seal (SANDBOX-PLAN §3.8, Q7): what the files hold is held while the core runs, and a change
// made while it's stopped counts at the next start, as the other records do.
//
// Writes come only through the core's routes the GM calls (MEM4), never a hire's: no route reaches this file in
// MEM1. The guard refuses any line of a hire's or of the GM's that names gm-memory (internal/guard's always and
// protectedNames), and a hire's settings deny Read and Edit of the folder (hireDenyRules). Every note is checked
// as it's written, and again with the lists of the day each time it's used (kept), when one that fails is withheld:
// its source's quote on its own with MEM2's memoryFieldRefusal, then the note with MEM2's memoryRefusal, which reads
// the quote with the note's fields, so a secret's name in one and its value in the other is refused too.

const (
	// memoryFolder is the store's folder in the support folder.
	memoryFolder = "gm-memory"
	// memoryManifestName is the manifest's name as a sealed record: its path under the support folder.
	memoryManifestName = memoryFolder + "/manifest.json"
	memoryManifestFile = "manifest.json"

	// The store's caps: kept notes, notes waiting for the owner's tap, and the index the GM reads (memoryIndex), in
	// bytes. A note's body is MEM2's memoryBodyMost.
	memoryKeptMost    = 200
	memoryWaitingMost = 50
	memoryIndexMost   = 8 * 1024
	// The most read of a note's file, and of the manifest, as the core starts: a bigger one counts for nothing.
	memoryFileMost     = 4 * 1024
	memoryManifestMost = 512 * 1024

	memoryWaiting = "waiting" // proposed, never shown to the GM as memory
	memoryKept    = "kept"    // kept by the owner's tap (MEM5) or the owner's own words (MEM6)
)

// Where a note's words came from (PL126M "Provenance"): the owner, through Smolcorp or in the GM's terminal (MEM3's
// ledger, quoted); the GM's own observation (a file:line or a command), for a fact alone; or the owner's own words,
// typed in Setup.
const (
	memoryFromSmolcorp = string(ownerViaSmolcorp)
	memoryFromTerminal = string(ownerViaTerminal)
	memoryFromSeen     = "seen"
	memoryFromOwner    = "owner"
)

// memorySource is where a note's words came from, and the words quoted, or what the GM saw them in. A quote from the
// owner-words ledger is the owner's whole sentence, or paragraphs after a negation, verbatim, and QuoteStart and
// QuoteEnd are where the GM's quote sits in it, in bytes (MEM4, memoryquote.go): the quote only points, the sentence
// is what the owner said.
type memorySource struct {
	Tag        string `json:"tag"`
	Quote      string `json:"quote,omitempty"`
	QuoteStart int    `json:"quoteStart,omitempty"`
	QuoteEnd   int    `json:"quoteEnd,omitempty"`
}

// gmNote is one note as the store holds it: its manifest line, and its body from its own file.
type gmNote struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        string       `json:"type"`
	State       string       `json:"state"`
	Source      memorySource `json:"source"`
	At          time.Time    `json:"at"`
	SHA256      string       `json:"sha256"`
	// The body, held from the note's file: never in the manifest.
	Body string `json:"-"`
	Rule string `json:"-"`
	Why  string `json:"-"`
	How  string `json:"-"`
}

// entry is the note as MEM2's checks read it.
func (n gmNote) entry() memoryEntry {
	return memoryEntry{Name: n.Name, Description: n.Description, Type: n.Type, Body: n.Body, Rule: n.Rule, Why: n.Why, How: n.How}
}

// gmMemory is the store this core holds. It has its own lock, never held with another but records.writing
// (through keepRecord), which takes none of the store's.
type gmMemory struct {
	mu    sync.Mutex
	dir   string
	notes []gmNote
	// aside are the files in the folder that counted for nothing as the core started, as Setup names them.
	aside []string
}

// memoryRefused is a note memory can't take, with the reason in the owner's words.
type memoryRefused struct{ reason string }

func (e *memoryRefused) Error() string { return "Memory can't keep this note: " + e.reason + "." }

var (
	errMemoryNotFound    = errors.New("there's no such note")
	errMemoryChanged     = errors.New("this note changed since it was shown")
	errMemoryWaitingFull = errors.New("50 notes are already waiting for the owner: memory takes no more until some are kept or dropped")
	errMemoryKeptFull    = errors.New("memory already keeps 200 notes: delete or merge some first")
	errMemoryIndexFull   = errors.New("the index the GM reads would be longer than 8 KB: delete or merge some notes first")
	errMemoryNameTaken   = errors.New("another note already has this name")
	errMemoryUnchanged   = errors.New("the edit changes nothing in the note")
	// errMemoryUnchangedRefused is an edit that changes nothing in a note today's checks refuse, which has no Keep
	// (FXMEMKEEPR2 O1): its own words, never "use Keep instead".
	errMemoryUnchangedRefused = errors.New("the edit changes nothing in a note the checks refuse")
)

// memoryIDShape is a note's id, as newID makes it: its file's name, so nothing else is ever opened.
var memoryIDShape = regexp.MustCompile(`^[0-9A-F]{32}$`)

// loadMemory is the store in dir, as the core starts. On a Mac the manifest counts only when its seal fits; one that
// doesn't counts for nothing, and Setup names it (recordDropped), with every note it listed. A note counts only when
// its file's hash is the manifest's, and only up to the caps, the index's too (memoryIndexMost). Every other file in
// the folder counts for nothing and is named too (asideText): left where it is, so the owner can see it, but never
// read again.
func loadMemory(dir string) *gmMemory {
	m := &gmMemory{dir: dir}
	var listed []gmNote
	if data, err := readMost(filepath.Join(dir, memoryManifestFile), memoryManifestMost); err == nil {
		body, counts := openBody(memoryManifestName, data)
		if !counts || json.Unmarshal(body, &listed) != nil {
			recordDropped(memoryManifestName, memoryManifestName+" was changed by another program, or its seal doesn't fit: "+
				"nothing the GM's memory held counts.")
			listed = nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		recordDropped(memoryManifestName, memoryManifestName+" couldn't be read: nothing the GM's memory held counts.")
	}
	counted := map[string]bool{memoryManifestFile: true}
	held, indexed := map[string]int{}, 0
	for _, note := range listed {
		file := note.ID + ".md"
		if !memoryIDShape.MatchString(note.ID) || counted[file] {
			continue
		}
		counted[file] = true
		data, err := readMost(filepath.Join(dir, file), memoryFileMost)
		body, ok := parseMemoryFile(data)
		line := 0
		if note.State == memoryKept {
			line = len(memoryIndexLine(note))
		}
		ok = ok && err == nil && indexed+line <= memoryIndexMost && note.SHA256 == memoryHash(data) && body.Name == note.Name &&
			body.Description == note.Description && body.Type == note.Type &&
			(note.State == memoryWaiting && held[memoryWaiting] < memoryWaitingMost || note.State == memoryKept && held[memoryKept] < memoryKeptMost)
		if !ok {
			m.aside = append(m.aside, file)
			continue
		}
		held[note.State]++
		indexed += line
		note.Body, note.Rule, note.Why, note.How = body.Body, body.Rule, body.Why, body.How
		m.notes = append(m.notes, note)
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !counted[entry.Name()] {
			m.aside = append(m.aside, entry.Name())
		}
	}
	slices.Sort(m.aside)
	return m
}

// readMost is the file at path, or an error if it's longer than most.
func readMost(path string, most int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, most+1))
	if err == nil && int64(len(data)) > most {
		err = errors.New("too long")
	}
	return data, err
}

func memoryHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// memoryFile is the file a note is kept in: its front matter, then its body, a line for each field. MEM2's checks
// refuse a line break in any field, so each stays one line.
func memoryFile(n gmNote) []byte {
	var b strings.Builder
	b.WriteString("---\nname: " + n.Name + "\ndescription: " + n.Description + "\ntype: " + n.Type + "\n---\n")
	if n.Type == "feedback" {
		b.WriteString("Rule: " + n.Rule + "\nWhy: " + n.Why + "\nHow to apply it: " + n.How + "\n")
	} else {
		b.WriteString(n.Body + "\n")
	}
	return []byte(b.String())
}

// parseMemoryFile is the note a file holds, and whether it is exactly what memoryFile writes for it.
func parseMemoryFile(data []byte) (gmNote, bool) {
	lines := strings.Split(string(data), "\n")
	if len(lines) < 7 || lines[0] != "---" || lines[4] != "---" {
		return gmNote{}, false
	}
	var n gmNote
	n.Name, _ = strings.CutPrefix(lines[1], "name: ")
	n.Description, _ = strings.CutPrefix(lines[2], "description: ")
	n.Type, _ = strings.CutPrefix(lines[3], "type: ")
	body := lines[5 : len(lines)-1]
	switch {
	case n.Type == "feedback" && len(body) == 3:
		n.Rule, _ = strings.CutPrefix(body[0], "Rule: ")
		n.Why, _ = strings.CutPrefix(body[1], "Why: ")
		n.How, _ = strings.CutPrefix(body[2], "How to apply it: ")
	case n.Type != "feedback" && len(body) == 1:
		n.Body = body[0]
	default:
		return gmNote{}, false
	}
	return n, bytes.Equal(memoryFile(n), data)
}

// memorySourceRefusal says why source can't go with a note of type kind, or "": a rule (feedback) or a fact about
// the owner (user) needs the owner's words; a fact about the work or where to find something may be the GM's own
// observation. Every note still needs the owner's tap to be kept. The quote is kept in the manifest, so it gets
// MEM2's own check of a field (memoryFieldRefusal): never a key, a token or personal data on disk.
func memorySourceRefusal(kind string, source memorySource) string {
	switch source.Tag {
	case memoryFromSmolcorp, memoryFromTerminal, memoryFromOwner:
	case memoryFromSeen:
		if kind == "feedback" || kind == "user" {
			return "a " + kind + " note needs the owner's own words, not what the GM saw"
		}
	default:
		return "it doesn't say where its words came from"
	}
	if source.Tag != memoryFromOwner && strings.TrimSpace(source.Quote) == "" {
		return "it doesn't quote where its words came from"
	}
	if refusal := quotePointerRefusal(source); refusal != "" {
		return refusal
	}
	// A line break in the owner's words is kept as they typed it, since what's kept after a negation is paragraphs
	// (SIERRA's backstop, FXMEM4R4; RT126MEM4R2 F3): the card shows it as one, so the checks read it as a space. A
	// carriage return, a tab, or line breaks enough to push the rest out of sight still refuse the note.
	quote := source.Quote
	if source.QuoteEnd != 0 {
		quote = strings.ReplaceAll(quote, "\n", " ")
	}
	return memoryFieldRefusal("source", quote, memoryBodyMost)
}

// memoryNoteRefusal says why a note the store holds can't stand today, or "": its source on its own, then MEM2's
// checks on its fields read with the source's quote (RT126MEM1R2 B3), each with the lists as they are now. The
// source goes first so a quote that fails on its own is named as the source, and one past its cap is never read
// with the fields.
func memoryNoteRefusal(n gmNote) string {
	if refusal := memorySourceRefusal(n.Type, n.Source); refusal != "" {
		return refusal
	}
	refusal, _ := memoryRefusal(n.entry(), n.Source.Quote)
	return refusal
}

// propose adds a note waiting for the owner's tap: checked as memoryNoteRefusal checks it, its file written whole
// then renamed, then the manifest, sealed. A stop between the two leaves a file the manifest doesn't list, which
// counts for nothing.
func (m *gmMemory) propose(entry memoryEntry, source memorySource, now time.Time) (gmNote, error) {
	if refusal := recordsRefusal(); refusal != "" {
		return gmNote{}, errors.New(refusal)
	}
	if refusal := memorySourceRefusal(entry.Type, source); refusal != "" {
		return gmNote{}, &memoryRefused{refusal}
	}
	if refusal, _ := memoryRefusal(entry, source.Quote); refusal != "" {
		return gmNote{}, &memoryRefused{refusal}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.count(memoryWaiting) >= memoryWaitingMost {
		return gmNote{}, errMemoryWaitingFull
	}
	note := gmNote{ID: newID(), Name: entry.Name, Description: entry.Description, Type: entry.Type, State: memoryWaiting,
		Source: source, At: now.UTC().Truncate(time.Second), Body: entry.Body, Rule: entry.Rule, Why: entry.Why, How: entry.How}
	data := memoryFile(note)
	note.SHA256 = memoryHash(data)
	path := filepath.Join(m.dir, note.ID+".md")
	if err := memoryMakeDir(m.dir); err != nil {
		return gmNote{}, err
	}
	if err := writePrivate(path, data); err != nil {
		return gmNote{}, err
	}
	notes := append(slices.Clone(m.notes), note)
	if err := keepRecord(memoryManifestName, m.writerOf(notes), false); err != nil {
		_ = os.Remove(path)
		m.writeFailed()
		return gmNote{}, err
	}
	m.notes = notes
	return note, nil
}

// keep makes a waiting note kept: what the owner's tap does (MEM5), on the hash of the note as it was shown. A note
// that changed since is refused (errMemoryChanged), as is one today's checks refuse, and one past the caps.
func (m *gmMemory) keep(id, shown string) error {
	if refusal := recordsRefusal(); refusal != "" {
		return errors.New(refusal)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.notes, func(n gmNote) bool { return n.ID == id && n.State == memoryWaiting })
	switch {
	case i < 0:
		return errMemoryNotFound
	case !constantTimeEqual(m.notes[i].SHA256, shown):
		return errMemoryChanged
	case m.count(memoryKept) >= memoryKeptMost:
		return errMemoryKeptFull
	}
	if refusal := memoryNoteRefusal(m.notes[i]); refusal != "" {
		return &memoryRefused{refusal}
	}
	notes := slices.Clone(m.notes)
	notes[i].State = memoryKept
	if len(memoryIndex(notes)) > memoryIndexMost {
		return errMemoryIndexFull
	}
	if err := keepRecord(memoryManifestName, m.writerOf(notes), false); err != nil {
		m.writeFailed()
		return err
	}
	m.notes = notes
	return nil
}

// keepEdited is the owner's Edit on a waiting note (MEM5, FXMEMKEEP): on the hash of the note as it was shown, its
// words replaced with the owner's own, checked again as a new note is, and kept in the same step. The note gets a new
// id and file; the GM's words go as a removal takes a note away, under a new record key, so a manifest copied from
// before never brings them back. A name another note already has is refused (errMemoryNameTaken), and so is an edit
// that changes nothing (errMemoryUnchanged, FXMEMKEEPR1): it would only make the GM's words the owner's. Where the
// waiting note fails today's checks (its source alone, say), Setup offers no Keep, so that refusal is
// errMemoryUnchangedRefused.
func (m *gmMemory) keepEdited(id, shown string, entry memoryEntry, now time.Time) (gmNote, error) {
	if refusal := recordsRefusal(); refusal != "" {
		return gmNote{}, errors.New(refusal)
	}
	source := memorySource{Tag: memoryFromOwner}
	if refusal := memorySourceRefusal(entry.Type, source); refusal != "" {
		return gmNote{}, &memoryRefused{refusal}
	}
	if refusal, _ := memoryRefusal(entry); refusal != "" {
		return gmNote{}, &memoryRefused{refusal}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	i := slices.IndexFunc(m.notes, func(n gmNote) bool { return n.ID == id && n.State == memoryWaiting })
	switch {
	case i < 0:
		return gmNote{}, errMemoryNotFound
	case !constantTimeEqual(m.notes[i].SHA256, shown):
		return gmNote{}, errMemoryChanged
	case m.notes[i].entry() == entry && memoryNoteRefusal(m.notes[i]) != "":
		return gmNote{}, errMemoryUnchangedRefused
	case m.notes[i].entry() == entry:
		return gmNote{}, errMemoryUnchanged
	case m.count(memoryKept) >= memoryKeptMost:
		return gmNote{}, errMemoryKeptFull
	}
	for j, other := range m.notes {
		if j != i && other.Name == entry.Name {
			return gmNote{}, errMemoryNameTaken
		}
	}
	note := gmNote{ID: newID(), Name: entry.Name, Description: entry.Description, Type: entry.Type, State: memoryKept,
		Source: source, At: now.UTC().Truncate(time.Second), Body: entry.Body, Rule: entry.Rule, Why: entry.Why, How: entry.How}
	data := memoryFile(note)
	note.SHA256 = memoryHash(data)
	notes := slices.Clone(m.notes)
	old := notes[i].ID
	notes[i] = note
	if len(memoryIndex(notes)) > memoryIndexMost {
		return gmNote{}, errMemoryIndexFull
	}
	path := filepath.Join(m.dir, note.ID+".md")
	if err := memoryMakeDir(m.dir); err != nil {
		return gmNote{}, err
	}
	if err := writePrivate(path, data); err != nil {
		return gmNote{}, err
	}
	if err := keepRecord(memoryManifestName, m.writerOf(notes), true); err != nil {
		_ = os.Remove(path)
		m.writeFailed()
		return gmNote{}, err
	}
	m.notes = notes
	_ = os.Remove(filepath.Join(m.dir, old+".md"))
	return note, nil
}

// removeKept takes the kept note id away, as remove does, never a waiting one: the owner's Forget (MEM6, FXMEMKEEP).
func (m *gmMemory) removeKept(id string) error {
	return m.removeWhere(func(n gmNote) bool { return n.ID == id && n.State == memoryKept }, false)
}

// writeFailed puts back, after a write that failed, the writer of the manifest as the store still holds it:
// keepRecord kept the new one before it wrote, and a later rotation would write a note that isn't there (RT126MEM1
// nit 5). The caller holds m.mu.
func (m *gmMemory) writeFailed() {
	keepWriter(memoryManifestName, m.writerOf(m.notes))
}

// remove takes the note id away, kept or waiting (only waiting with onlyWaiting: the GM withdrawing its own), under
// a new record key: a manifest copied from before no longer fits. Its file goes after the manifest is written.
func (m *gmMemory) remove(id string, onlyWaiting bool) error {
	return m.removeWhere(func(n gmNote) bool { return n.ID == id && (!onlyWaiting || n.State == memoryWaiting) }, false)
}

// forget takes every note away, under a new record key, and every file in the folder with them: "Forget everything".
func (m *gmMemory) forget() error {
	return m.removeWhere(func(gmNote) bool { return true }, true)
}

func (m *gmMemory) removeWhere(gone func(gmNote) bool, all bool) error {
	if refusal := recordsRefusal(); refusal != "" {
		return errors.New(refusal)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var files []string
	kept := slices.DeleteFunc(slices.Clone(m.notes), func(n gmNote) bool {
		if gone(n) {
			files = append(files, n.ID+".md")
			return true
		}
		return false
	})
	if len(files) == 0 && !all {
		return errMemoryNotFound
	}
	if all {
		files = append(files, m.aside...)
	}
	if err := keepRecord(memoryManifestName, m.writerOf(kept), true); err != nil {
		return err
	}
	m.notes = kept
	for _, file := range files {
		if filepath.Base(file) == file && file != memoryManifestFile {
			_ = os.Remove(filepath.Join(m.dir, file))
		}
	}
	if all {
		m.aside = nil
	}
	return nil
}

// save writes the manifest as the store holds it, under the record key of the moment: at Setup's reset (records.go),
// so what the store holds goes on counting under the new key. Nothing is written for a store that never was.
func (m *gmMemory) save() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := os.Lstat(filepath.Join(m.dir, memoryManifestFile)); err != nil && len(m.notes) == 0 {
		return nil
	}
	return keepRecord(memoryManifestName, m.writerOf(m.notes), false)
}

// writer writes the manifest as it is now (records.go's recordWriter), for the rotations to come.
func (m *gmMemory) writer() recordWriter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writerOf(m.notes)
}

// writerOf writes notes as the manifest, sealed under the key it's given. It takes no lock. The caller holds m.mu.
func (m *gmMemory) writerOf(notes []gmNote) recordWriter {
	list, dir := slices.Clone(notes), m.dir
	if list == nil {
		list = []gmNote{}
	}
	return func(key []byte, prev string) error {
		data, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return err
		}
		if err := memoryMakeDir(dir); err != nil {
			return err
		}
		return writePrivate(filepath.Join(dir, memoryManifestFile), sealBody(key, prev, memoryManifestName, data))
	}
}

// memoryMakeDir makes the store's folder, the owner's alone (0700): one already there with looser permissions is
// tightened.
func memoryMakeDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// count is how many notes are in state. The caller holds m.mu.
func (m *gmMemory) count(state string) int {
	n := 0
	for _, note := range m.notes {
		if note.State == state {
			n++
		}
	}
	return n
}

// kept is what the GM may read (MEM4): the kept notes that pass MEM2's checks as they are today, their source's
// quote too, in the order they were proposed, and the names of those withheld. A waiting note is never memory. A
// withheld note stays in the store, and on disk, until the owner deletes it (RT126MEM1R2 nit 1, for MEM6's list).
func (m *gmMemory) kept() (shown []gmNote, withheld []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, note := range m.notes {
		if note.State != memoryKept {
			continue
		}
		if memoryNoteRefusal(note) != "" {
			withheld = append(withheld, note.Name)
			continue
		}
		shown = append(shown, note)
	}
	return shown, withheld
}

// all is every note the store holds, waiting and kept, for the owner's lists (MEM5, MEM6). It checks none of them:
// what the GM reads comes from kept alone.
func (m *gmMemory) all() []gmNote {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.notes)
}

// memoryIndex is the index of the kept notes among notes, a line each: what the GM reads first (MEM4), and what
// memoryIndexMost caps.
func memoryIndex(notes []gmNote) string {
	var b strings.Builder
	for _, n := range notes {
		if n.State == memoryKept {
			b.WriteString(memoryIndexLine(n))
		}
	}
	return b.String()
}

// memoryIndexLine is a kept note's line in the index.
func memoryIndexLine(n gmNote) string {
	return "- " + n.Name + " (" + n.Type + "): " + n.Description + "\n"
}

// asideText is the line Setup says of the files in the folder that counted for nothing at this start, "" for none,
// and while nothing counts anyway (records.go's recordsText says why).
func (m *gmMemory) asideText() string {
	if m == nil || recordsFailed() {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.aside) == 0 {
		return ""
	}
	return "Files in the GM's memory didn't fit and count for nothing: " + strings.Join(m.aside, ", ") +
		". Another program may have changed or added them."
}
