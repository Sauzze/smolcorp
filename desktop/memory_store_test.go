package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// MEM1 (PL126M §1): the GM's memory store, gm-memory/, a sealed record held from the start. "Another program" is
// played with plain file reads and writes, against a stand-in Mac app whose stand-in Keychain holds the record key
// (records_test.go's macDeck).

// fromTheOwner is where a note's words came from in these rows: the owner, through Smolcorp, quoted.
var fromTheOwner = memorySource{Tag: memoryFromSmolcorp, Quote: "tests here run with pnpm, always"}

// proposeNote proposes entry on d's store and fails the test if it's refused.
func proposeNote(t *testing.T, d *Deck, entry memoryEntry) gmNote {
	t.Helper()
	note, err := d.memory.propose(entry, fromTheOwner, time.Now())
	if err != nil {
		t.Fatalf("propose %q: %v", entry.Name, err)
	}
	return note
}

// keptNote proposes entry and keeps it, as the owner's tap would.
func keptNote(t *testing.T, d *Deck, entry memoryEntry) gmNote {
	t.Helper()
	note := proposeNote(t, d, entry)
	if err := d.memory.keep(note.ID, note.SHA256); err != nil {
		t.Fatalf("keep %q: %v", entry.Name, err)
	}
	return note
}

// memoryPath is a file in the test's gm-memory folder.
func memoryPath(name string) string { return filepath.Join(supportDir, memoryFolder, name) }

// keptNames are the names of the notes the GM would read.
func keptNames(d *Deck) []string {
	shown, _ := d.memory.kept()
	var names []string
	for _, note := range shown {
		names = append(names, note.Name)
	}
	return names
}

// droppedSaid is what Setup's state says in recordsDropped now, as the page reads it.
func droppedSaid(d *Deck) string {
	return strings.TrimSpace(recordsDroppedText() + " " + d.memory.asideText())
}

func named(name string) memoryEntry {
	entry := memoryNote("Tests run with pnpm")
	entry.Name = name
	return entry
}

// The manifest is sealed under the record key, each file is the owner's alone and written whole, and what the core
// wrote counts at the next start: waiting stays waiting, kept stays kept, and only kept notes are memory.
func TestOnAMacTheGMsMemoryIsASealedRecordThatLasts(t *testing.T) {
	d, _, _, _ := macDeck(t)
	waiting := proposeNote(t, d, named("Waiting note"))
	kept := keptNote(t, d, memoryRule("Run the tests before saying a piece is done"))
	data, err := os.ReadFile(memoryPath(memoryManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if seal, ok := sealOfFile(memoryManifestName, data); !ok || !seal.fits(recordKey()) {
		t.Fatal("the manifest isn't sealed under the record key")
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{memoryManifestFile, waiting.ID + ".md", kept.ID + ".md"} {
			if info, err := os.Stat(memoryPath(name)); err != nil || info.Mode().Perm() != 0o600 {
				t.Errorf("%s: %v %v", name, info.Mode(), err)
			}
		}
		if info, err := os.Stat(filepath.Join(supportDir, memoryFolder)); err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("the folder: %v %v", info.Mode(), err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(supportDir, memoryFolder))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("%s left behind: a file is written whole, then renamed", entry.Name())
		}
	}
	body, _ := os.ReadFile(memoryPath(kept.ID + ".md"))
	want := "---\nname: A rule\ndescription: One line about it\ntype: feedback\n---\nRule: Run the tests before saying a piece is done\n" +
		"Why: The owner said so\nHow to apply it: Every time\n"
	if string(body) != want {
		t.Errorf("the note's file:\n%s", body)
	}
	if got := keptNames(d); !slices.Equal(got, []string{"A rule"}) {
		t.Errorf("memory before the restart: %q", got)
	}
	again := restart(t)
	all := again.memory.all()
	if len(all) != 2 || all[0].State != memoryWaiting || all[1].State != memoryKept || all[1].Rule != kept.Rule || all[1].Source != fromTheOwner {
		t.Fatalf("after the restart: %+v", all)
	}
	if got := keptNames(again); !slices.Equal(got, []string{"A rule"}) {
		t.Errorf("memory after the restart: %q", got)
	}
	if said := droppedSaid(again); said != "" {
		t.Errorf("Setup says %q of a store the core wrote", said)
	}
}

// The tamper rows: a note's file edited, a file added (alone, and listed in a manifest another program edited), and
// an old manifest copied back after a delete. Each counts for nothing on a Mac, and Setup names it.
func TestOnAMacAMemoryAnotherProgramChangesCountsForNothing(t *testing.T) {
	t.Run("an edited note", func(t *testing.T) {
		d, _, _, _ := macDeck(t)
		note := keptNote(t, d, named("Edited note"))
		keptNote(t, d, named("Untouched note"))
		path := memoryPath(note.ID + ".md")
		data, _ := os.ReadFile(path)
		if err := os.WriteFile(path, []byte(strings.Replace(string(data), "with pnpm", "with yarn", 1)), 0o600); err != nil {
			t.Fatal(err)
		}
		again := restart(t)
		if got := keptNames(again); !slices.Equal(got, []string{"Untouched note"}) {
			t.Errorf("memory: %q", got)
		}
		if said := droppedSaid(again); !strings.Contains(said, note.ID+".md") {
			t.Errorf("Setup says %q", said)
		}
	})
	t.Run("an added file", func(t *testing.T) {
		d, _, _, _ := macDeck(t)
		keptNote(t, d, named("Kept note"))
		planted := gmNote{ID: newID(), Name: "Planted note", Description: "One line about it", Type: "project", Body: "Tests run with yarn"}
		if err := os.WriteFile(memoryPath(planted.ID+".md"), memoryFile(planted), 0o600); err != nil {
			t.Fatal(err)
		}
		again := restart(t)
		if got := keptNames(again); !slices.Equal(got, []string{"Kept note"}) || len(again.memory.all()) != 1 {
			t.Errorf("memory: %q", got)
		}
		if said := droppedSaid(again); !strings.Contains(said, planted.ID+".md") {
			t.Errorf("Setup says %q", said)
		}
	})
	t.Run("an added file the manifest lists", func(t *testing.T) {
		d, _, _, _ := macDeck(t)
		keptNote(t, d, named("Kept note"))
		planted := gmNote{ID: newID(), Name: "Planted note", Description: "One line about it", Type: "project", State: memoryKept, Body: "Tests run with yarn"}
		data := memoryFile(planted)
		planted.SHA256 = memoryHash(data)
		if err := os.WriteFile(memoryPath(planted.ID+".md"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		rewrite(t, memoryManifestName, func(file map[string]any) {
			line, _ := json.Marshal(planted)
			var listed map[string]any
			_ = json.Unmarshal(line, &listed)
			file["body"] = append(file["body"].([]any), listed)
		})
		again := restart(t)
		if got := keptNames(again); len(got) != 0 || len(again.memory.all()) != 0 {
			t.Errorf("memory: %q", got)
		}
		if said := droppedSaid(again); !strings.Contains(said, memoryManifestName) || !strings.Contains(said, planted.ID+".md") {
			t.Errorf("Setup says %q", said)
		}
	})
	t.Run("an old manifest copied back after a delete", func(t *testing.T) {
		d, _, _, _ := macDeck(t)
		gone := keptNote(t, d, named("Deleted note"))
		keptNote(t, d, named("Kept note"))
		copies := map[string][]byte{}
		for _, name := range []string{memoryManifestFile, gone.ID + ".md"} {
			copies[name], _ = os.ReadFile(memoryPath(name))
		}
		if err := d.memory.remove(gone.ID, false); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(memoryPath(gone.ID + ".md")); err == nil {
			t.Error("the deleted note's file is still there")
		}
		for name, data := range copies {
			if err := os.WriteFile(memoryPath(name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		again := restart(t)
		if got := keptNames(again); len(got) != 0 {
			t.Errorf("the deleted note came back: %q", got)
		}
		if said := droppedSaid(again); !strings.Contains(said, memoryManifestName) {
			t.Errorf("Setup says %q", said)
		}
	})
}

// Every removal rotates the record key: the GM withdrawing its waiting note, a kept note deleted, and "Forget
// everything", which takes every file in the folder with it.
func TestOnAMacEveryMemoryRemovalRotatesTheRecords(t *testing.T) {
	d, _, _, _ := macDeck(t)
	waiting := proposeNote(t, d, named("Waiting note"))
	kept := keptNote(t, d, named("Kept note"))
	keptNote(t, d, named("Another note"))
	if err := d.memory.remove(kept.ID, true); !errors.Is(err, errMemoryNotFound) {
		t.Errorf("a kept note withdrawn as waiting: %v", err)
	}
	for _, step := range []struct {
		what   string
		remove func() error
	}{
		{"withdraw", func() error { return d.memory.remove(waiting.ID, true) }},
		{"delete", func() error { return d.memory.remove(kept.ID, false) }},
		{"forget", d.memory.forget},
	} {
		before := recordKeyID()
		if err := step.remove(); err != nil {
			t.Fatalf("%s: %v", step.what, err)
		}
		if recordKeyID() == before {
			t.Errorf("%s didn't rotate the records", step.what)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(supportDir, memoryFolder))
	if len(entries) != 1 || entries[0].Name() != memoryManifestFile || len(d.memory.all()) != 0 {
		t.Errorf("after forget: %v", entries)
	}
	if again := restart(t); len(again.memory.all()) != 0 {
		t.Error("a note came back after forget")
	}
}

// While the records are held in memory alone (the key unreadable), every write is refused with SK1's sentence, and
// nothing reaches the folder; after a failed check, until the owner resets.
func TestMemoryRefusesEveryWriteWhileTheRecordsAreHeld(t *testing.T) {
	d, _, _, _ := macDeck(t)
	kept := keptNote(t, d, named("Kept note"))
	waiting := proposeNote(t, d, named("Waiting note"))
	before, _ := os.ReadFile(memoryPath(memoryManifestFile))
	for state, answer := range map[string]string{recordsUnreadable: recordsHeldAnswer, recordsChanged: recordsResetFirstAnswer} {
		records.mu.Lock()
		was, key := records.state, records.key
		records.state = state
		if state == recordsChanged {
			records.key = nil
		}
		records.mu.Unlock()
		_, proposed := d.memory.propose(named("New note"), fromTheOwner, time.Now())
		for what, err := range map[string]error{"propose": proposed, "keep": d.memory.keep(waiting.ID, waiting.SHA256),
			"remove": d.memory.remove(kept.ID, false), "forget": d.memory.forget()} {
			if err == nil || err.Error() != answer {
				t.Errorf("%s, %s: %v", state, what, err)
			}
		}
		records.mu.Lock()
		records.state, records.key = was, key
		records.mu.Unlock()
	}
	after, _ := os.ReadFile(memoryPath(memoryManifestFile))
	entries, _ := os.ReadDir(filepath.Join(supportDir, memoryFolder))
	if string(after) != string(before) || len(entries) != 3 || len(d.memory.all()) != 2 {
		t.Errorf("something was written while the records were held: %d files", len(entries))
	}
}

// Off a Mac there's no seal: the store is held while the core runs (a file changed meanwhile counts for nothing, and
// once the core writes its manifest again, not at the next start either), and a file changed while it's stopped
// counts at the next start.
func TestOffAMacTheGMsMemoryIsHeldAndCountsAsFoundAtTheNextStart(t *testing.T) {
	isolate(t)
	d := recordsDeck(t)
	if recordsState() != recordsOff {
		t.Fatalf("state %q", recordsState())
	}
	note := keptNote(t, d, named("Kept note"))
	path := memoryPath(note.ID + ".md")
	edited := gmNote{Name: note.Name, Description: note.Description, Type: note.Type, Body: "Tests run with yarn"}
	editedFile := memoryFile(edited)
	edit := func() {
		if err := os.WriteFile(path, editedFile, 0o600); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(memoryPath(memoryManifestFile))
		var listed []map[string]any
		if err := json.Unmarshal(data, &listed); err != nil {
			t.Fatalf("off a Mac the manifest is a plain list: %v", err)
		}
		listed[0]["sha256"] = memoryHash(editedFile)
		out, _ := json.Marshal(listed)
		if err := os.WriteFile(memoryPath(memoryManifestFile), out, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	edit()
	if shown, _ := d.memory.kept(); len(shown) != 1 || shown[0].Body != "Tests run with pnpm" {
		t.Errorf("a change made while running counted: %+v", shown)
	}
	proposeNote(t, d, named("Another note")) // the manifest written again as the core holds it
	again := restart(t)
	if shown, _ := again.memory.kept(); len(shown) != 0 || !strings.Contains(again.memory.asideText(), note.ID+".md") {
		t.Errorf("after a write, the file changed while running counted: %+v", shown)
	}
	edit()
	last := restart(t)
	if shown, _ := last.memory.kept(); len(shown) != 1 || shown[0].Body != "Tests run with yarn" {
		t.Errorf("a change made while stopped didn't count at the next start: %+v", shown)
	}
}

// Quotes MEM2's checks refuse (harmless stand-ins): a key's shape, and an e-mail address.
var (
	keyQuote  = "my key is sk-" + "ant-api03-FAKEFAKEFAKEFAKEFAKEFAKE"
	mailQuote = "send it to someone" + "@example.com"
)

// quoteWithNote are notes and quotes that each stand alone, but not together (RT126MEM1R2 B3): a secret's name in
// the note and its value in the quote, or the other way round. The values are harmless stand-ins.
var quoteWithNote = []struct {
	what  string
	entry memoryEntry
	quote string
}{
	{"Password name", memoryEntry{Name: "Staging password", Description: "One line about it", Type: "project", Body: "It is in the team vault."}, "zebra" + "42"},
	{"Password value", memoryEntry{Name: "Vault note", Description: "One line about it", Type: "project", Body: "zebra" + "42"}, "the staging password"},
	{"PIN value", memoryEntry{Name: "Door note", Description: "One line about it", Type: "project", Body: "4321 opens it"}, "the door PIN"},
}

// The gate check on both ends (memoryFieldRefusal on the source's quote, then MEM2's memoryRefusal on the note read
// with its quote): a note it refuses is never written, and a kept note that fails the checks of the day is withheld
// from the GM, or refused at the owner's tap. A rule needs the owner's quoted words, a fact may be the GM's.
func TestMemoryChecksANoteAsItsWrittenAndAsItsRead(t *testing.T) {
	d, _, _, _ := macDeck(t)
	refusals := map[string]struct {
		entry  memoryEntry
		source memorySource
	}{
		"a key":                  {memoryNote("The key is sk-" + "ant-api03-FAKEFAKEFAKEFAKEFAKEFAKE"), fromTheOwner},
		"markup":                 {memoryNote("# Tests run with pnpm"), fromTheOwner},
		"a rule the GM saw":      {memoryRule("Run the tests before saying a piece is done"), memorySource{Tag: memoryFromSeen, Quote: "Makefile:3"}},
		"no quote":               {memoryRule("Run the tests before saying a piece is done"), memorySource{Tag: memoryFromTerminal}},
		"no source":              {memoryNote("Tests run with pnpm"), memorySource{}},
		"a body over 1000 bytes": {memoryNote(strings.TrimSpace(strings.Repeat("Tests run with pnpm here. ", 40))), fromTheOwner},
		"a key in the quote":     {memoryNote("Tests run with pnpm"), memorySource{Tag: memoryFromSmolcorp, Quote: keyQuote}},
		"an e-mail in the quote": {memoryNote("Tests run with pnpm"), memorySource{Tag: memoryFromTerminal, Quote: mailQuote}},
	}
	for _, row := range quoteWithNote {
		if refusal, _ := memoryRefusal(row.entry); refusal != "" {
			t.Fatalf("%s: the note alone is refused (%s), so the row shows nothing", row.what, refusal)
		}
		if refusal := memorySourceRefusal(row.entry.Type, memorySource{Tag: memoryFromSmolcorp, Quote: row.quote}); refusal != "" {
			t.Fatalf("%s: the quote alone is refused (%s), so the row shows nothing", row.what, refusal)
		}
		refusals[row.what+" with its quote"] = struct {
			entry  memoryEntry
			source memorySource
		}{row.entry, memorySource{Tag: memoryFromSmolcorp, Quote: row.quote}}
	}
	for what, row := range refusals {
		_, err := d.memory.propose(row.entry, row.source, time.Now())
		var refused *memoryRefused
		if !errors.As(err, &refused) {
			t.Errorf("%s: %v", what, err)
		}
	}
	if _, err := os.Stat(filepath.Join(supportDir, memoryFolder)); err == nil {
		t.Error("a refused note reached the folder")
	}
	if _, err := d.memory.propose(memoryNote("Tests run with pnpm"), memorySource{Tag: memoryFromSeen, Quote: "package.json:12"}, time.Now()); err != nil {
		t.Errorf("a fact the GM saw: %v", err)
	}
	stale := keptNote(t, d, named("Stale note"))
	staleKey := keptNote(t, d, named("Stale quote"))
	staleMail := keptNote(t, d, named("Stale mail quote"))
	var staleWith, waitingWith []gmNote
	wantWithheld := []string{"Stale note", "Stale quote", "Stale mail quote"}
	for _, row := range quoteWithNote {
		staleWith = append(staleWith, keptNote(t, d, row.entry))
		wantWithheld = append(wantWithheld, row.entry.Name)
	}
	keptNote(t, d, named("Good note"))
	waitingKey := proposeNote(t, d, named("Waiting quote"))
	waitingMail := proposeNote(t, d, named("Waiting mail quote"))
	for _, row := range quoteWithNote {
		waitingWith = append(waitingWith, proposeNote(t, d, row.entry))
	}
	// The lists change: today's checks refuse what yesterday's took, in the body or in the quote (played on the held
	// copy).
	d.memory.mu.Lock()
	change := func(id string, edit func(*gmNote)) {
		edit(&d.memory.notes[slices.IndexFunc(d.memory.notes, func(n gmNote) bool { return n.ID == id })])
	}
	change(stale.ID, func(n *gmNote) { n.Body = "The key is sk-" + "ant-api03-FAKEFAKEFAKEFAKEFAKEFAKE" })
	change(staleKey.ID, func(n *gmNote) { n.Source.Quote = keyQuote })
	change(staleMail.ID, func(n *gmNote) { n.Source.Quote = mailQuote })
	change(waitingKey.ID, func(n *gmNote) { n.Source.Quote = keyQuote })
	change(waitingMail.ID, func(n *gmNote) { n.Source.Quote = mailQuote })
	for i, row := range quoteWithNote {
		change(staleWith[i].ID, func(n *gmNote) { n.Source.Quote = row.quote })
		change(waitingWith[i].ID, func(n *gmNote) { n.Source.Quote = row.quote })
	}
	d.memory.mu.Unlock()
	shown, withheld := d.memory.kept()
	if len(shown) != 1 || shown[0].Name != "Good note" || !slices.Equal(withheld, wantWithheld) {
		t.Errorf("shown %+v, withheld %q", shown, withheld)
	}
	for _, note := range append([]gmNote{waitingKey, waitingMail}, waitingWith...) {
		var refused *memoryRefused
		if err := d.memory.keep(note.ID, note.SHA256); !errors.As(err, &refused) {
			t.Errorf("a tap on %q, whose quote today's checks refuse: %v", note.Name, err)
		}
	}
	// The owner's tap is on the note as it was shown: one that changed since is refused.
	waiting := proposeNote(t, d, named("Waiting note"))
	if err := d.memory.keep(waiting.ID, memoryHash([]byte("something else"))); !errors.Is(err, errMemoryChanged) {
		t.Errorf("a tap on what wasn't shown: %v", err)
	}
}

// The caps: 50 waiting, 200 kept, and the index the GM reads at most 8 KB.
func TestMemoryKeepsToItsCaps(t *testing.T) {
	d, _, _, _ := macDeck(t)
	for i := range memoryWaitingMost {
		proposeNote(t, d, named(fmt.Sprintf("Note %d", i)))
	}
	if _, err := d.memory.propose(named("One too many"), fromTheOwner, time.Now()); !errors.Is(err, errMemoryWaitingFull) {
		t.Fatalf("the 51st waiting note: %v", err)
	}
	for _, note := range d.memory.all() {
		if err := d.memory.keep(note.ID, note.SHA256); err != nil {
			t.Fatal(err)
		}
	}
	for i := memoryWaitingMost; i < memoryKeptMost; i++ {
		keptNote(t, d, named(fmt.Sprintf("Note %d", i)))
	}
	if len(memoryIndex(d.memory.all())) > memoryIndexMost {
		t.Fatal("the rows' notes are too long for this test")
	}
	extra := proposeNote(t, d, named("One too many"))
	if err := d.memory.keep(extra.ID, extra.SHA256); !errors.Is(err, errMemoryKeptFull) {
		t.Errorf("the 201st kept note: %v", err)
	}
	if err := d.memory.forget(); err != nil {
		t.Fatal(err)
	}
	long := strings.TrimSpace(strings.Repeat("The tests in this project run with pnpm here. ", 4))
	var err error
	for i := 0; err == nil && i < memoryKeptMost; i++ {
		entry := named(fmt.Sprintf("Note %d", i))
		entry.Description = long
		note := proposeNote(t, d, entry)
		err = d.memory.keep(note.ID, note.SHA256)
	}
	if !errors.Is(err, errMemoryIndexFull) || len(memoryIndex(d.memory.all())) > memoryIndexMost {
		t.Errorf("past 8 KB of index: %v, %d bytes", err, len(memoryIndex(d.memory.all())))
	}
}

// A hire's settings deny Read and Edit of the folder and all it holds, whether or not it is there yet.
func TestAHireCantReadOrEditTheGMsMemory(t *testing.T) {
	r := newHireSettingsRig(t)
	path, err := r.write("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	deny := r.deny(path)
	for _, name := range []string{memoryManifestFile, "0123456789ABCDEF0123456789ABCDEF.md", "x/y"} {
		for _, tool := range []string{"Read", "Edit", "Write"} {
			if !hireDenied(deny, filepath.Join(r.support, memoryFolder, name), tool) {
				t.Errorf("%s of gm-memory/%s isn't denied", tool, name)
			}
		}
	}
	if !hireDenied(deny, filepath.Join(r.support, memoryFolder), "Edit") {
		t.Error("a file in the folder's place isn't denied")
	}
}

// Setup's reset seals what the store holds under the new key: every note goes on counting; after a manifest that
// didn't fit, none, and each note's file is named as counting for nothing.
func TestOnAMacTheGMsMemoryAfterAReset(t *testing.T) {
	d, _, _, _ := macDeck(t)
	note := keptNote(t, d, named("Kept note"))
	records.mu.Lock()
	records.stale = true
	records.mu.Unlock()
	d.mu.Lock()
	err := d.resetRecords()
	d.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if got := keptNames(restart(t)); !slices.Equal(got, []string{"Kept note"}) {
		t.Errorf("after a reset when the check passed: %q", got)
	}
	// Another program's manifest: nothing counts, and the reset seals the store as the core holds it, empty.
	rewrite(t, memoryManifestName, func(file map[string]any) { file["body"] = []any{} })
	failed := restart(t)
	failed.mu.Lock()
	err = failed.resetRecords()
	failed.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	last := restart(t)
	if len(last.memory.all()) != 0 || !strings.Contains(droppedSaid(last), note.ID+".md") {
		t.Errorf("after a reset that followed a dropped manifest: %+v, %q", last.memory.all(), droppedSaid(last))
	}
}

// Off a Mac, a manifest another program edited while the core was stopped is read to the store's caps, the index
// the GM reads too, and a name its note's own file doesn't have counts for nothing (RT126MEM1 nits 1 and 4). The
// store's folder, found with looser permissions, is the owner's alone again at the next write (nit 3).
func TestOffAMacTheGMsMemoryIsReadToItsCapsAndItsFiles(t *testing.T) {
	isolate(t)
	dir := filepath.Join(supportDir, memoryFolder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var listed []gmNote
	for i := range 60 {
		note := gmNote{ID: fmt.Sprintf("%032X", i), Name: fmt.Sprintf("Note %d", i), Description: strings.Repeat("Tests run with pnpm. ", 9),
			Type: "project", State: memoryKept, Source: fromTheOwner, At: time.Now().UTC().Truncate(time.Second), Body: "Tests run with pnpm"}
		data := memoryFile(note)
		note.SHA256 = memoryHash(data)
		if err := os.WriteFile(filepath.Join(dir, note.ID+".md"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		listed = append(listed, note)
	}
	listed[0].Name = "Renamed note" // in the manifest alone: its file still says Note 0
	data, _ := json.Marshal(listed)
	if err := os.WriteFile(filepath.Join(dir, memoryManifestFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
	d := recordsDeck(t)
	if recordsState() != recordsOff {
		t.Fatalf("state %q", recordsState())
	}
	var names []string
	for _, note := range d.memory.all() {
		names = append(names, note.Name)
	}
	aside := d.memory.asideText()
	if slices.Contains(names, "Renamed note") || slices.Contains(names, "Note 0") || !strings.Contains(aside, listed[0].ID+".md") {
		t.Errorf("a name the note's file doesn't have counted: %q, aside %q", names, aside)
	}
	if index := memoryIndex(d.memory.all()); len(index) > memoryIndexMost || len(names) < 30 || !strings.Contains(aside, listed[59].ID+".md") {
		t.Errorf("the index read at the start is %d bytes for %d notes (most %d), aside %q", len(index), len(names), memoryIndexMost, aside)
	}
	if runtime.GOOS == "windows" {
		return
	}
	proposeNote(t, d, named("Fresh note"))
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("the folder after a write is %v, not the owner's alone", info.Mode().Perm())
	}
}
