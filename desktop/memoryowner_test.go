package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// FXMEMKEEP: the owner's side of the GM's memory (memoryowner.go). What the GM proposes shows in Setup, and the owner
// keeps it, edits it then keeps it, or drops it; a kept note can be forgotten. Only the owner's page does any of it,
// under the Mac app its own Settings window; the store's caps, checks and seal stand. On the tree before (8647cf56)
// every route here is 404, and nothing the GM proposes is ever kept.

// ownerList is GET /setup/gm/memory/notes as the page, decoded.
func ownerList(t *testing.T, d *Deck) memoryOwnerList {
	t.Helper()
	code, text := memoryAsk(t, d, callerPage, "", "", "GET", memoryNotesPath, "")
	var list memoryOwnerList
	if code != http.StatusOK || json.Unmarshal([]byte(text), &list) != nil {
		t.Fatalf("the owner's list: %d %s", code, text)
	}
	return list
}

// proposeAsGM is POST /setup/gm/memory/propose as the GM, with the new note's id.
func proposeAsGM(t *testing.T, d *Deck, gm string, fields map[string]string) string {
	t.Helper()
	code, text := memoryAsk(t, d, callerGM, gm, "", "POST", memoryProposePath, proposal(fields))
	var got struct{ ID string }
	if code != http.StatusOK || json.Unmarshal([]byte(text), &got) != nil || got.ID == "" {
		t.Fatalf("the GM's proposal: %d %s", code, text)
	}
	return got.ID
}

// docsFact is a fact the GM saw, which it may propose without the owner's words.
var docsFact = map[string]string{"name": "Docs folder", "description": "Where the docs are", "type": "reference",
	"body": "The user guide is in the docs folder.", "seen": "ls docs"}

// waitingShown is the waiting note id in the owner's list, as the page shows it.
func waitingShown(t *testing.T, d *Deck, id string) memoryOwnerNote {
	t.Helper()
	list := ownerList(t, d)
	i := slices.IndexFunc(list.Waiting, func(n memoryOwnerNote) bool { return n.ID == id })
	if i < 0 {
		t.Fatalf("%s isn't waiting in the owner's list: %+v", id, list)
	}
	return list.Waiting[i]
}

// The GM proposes; the owner's list shows the note whole, with where its words came from and the hash to keep it
// by; the GM's read doesn't have it; a keep on another hash is refused; the keep stands; the GM's next read has it.
func TestTheOwnerKeepsWhatTheGMProposedAndTheGMReadsIt(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	id := proposeAsGM(t, d, gm, docsFact)
	shown := waitingShown(t, d, id)
	if shown.Name != "Docs folder" || shown.Body != docsFact["body"] || shown.From != memoryFromSeen || shown.Said != "ls docs" ||
		shown.Shown == "" || shown.Refused != "" {
		t.Errorf("the note as the owner sees it: %+v", shown)
	}
	if _, answer, _ := readAsGM(t, d, gm, ""); len(answer.Notes) != 0 {
		t.Errorf("a waiting note read as memory: %+v", answer.Notes)
	}
	keep := func(shownHash string) (int, string) {
		return memoryAsk(t, d, callerPage, "", "", "POST", memoryKeepPath+"?id="+id+"&shown="+shownHash, "")
	}
	if code, text := keep(strings.Repeat("0", 64)); code != http.StatusConflict || !strings.Contains(text, "changed since it was shown") {
		t.Errorf("a keep on another hash: %d %s", code, text)
	}
	if code, text := keep(shown.Shown); code != http.StatusOK || !strings.Contains(text, `"kept":true`) {
		t.Fatalf("the owner's keep: %d %s", code, text)
	}
	code, answer, text := readAsGM(t, d, gm, "?name=Docs+folder")
	if code != http.StatusOK || answer.Header != memoryHeader || len(answer.Notes) != 1 || answer.Notes[0].Body != docsFact["body"] {
		t.Errorf("the GM's next read: %d %s", code, text)
	}
	if list := ownerList(t, d); len(list.Waiting) != 0 || len(list.Kept) != 1 || list.Kept[0].ID != id || list.Kept[0].Shown != "" {
		t.Errorf("the owner's list after the keep: %+v", list)
	}
	if code, _ := keep(shown.Shown); code != http.StatusNotFound {
		t.Errorf("kept twice: %d", code)
	}
}

// Edit: the owner's own words, checked again as a new note is, kept in one step under a new id; the GM's words go.
// A stale hash, a name another note has, a field the route doesn't know, and words the store refuses (an e-mail, a
// key's shape) each leave the proposal waiting as it was.
func TestTheOwnerEditsANoteThenKeepsIt(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	d.owner.submitted("gm1", "Please always run the tests with pnpm in this office.")
	id := proposeAsGM(t, d, gm, map[string]string{"name": "Tests run with pnpm", "description": "Which tool runs the tests",
		"type": "feedback", "rule": "Run the tests with pnpm", "why": "The owner said so", "how": "Every time",
		"quote": "always run the tests with pnpm"})
	keptNote(t, d, canaryNote)
	shown := waitingShown(t, d, id)
	if shown.From != memoryFromTerminal || shown.Said != "Please always run the tests with pnpm in this office." {
		t.Errorf("a quote's source as the owner sees it: %+v", shown)
	}
	edit := func(fields map[string]string) (int, string) {
		body := map[string]string{"id": id, "shown": shown.Shown, "name": "Tests run with pnpm", "description": "Which tool runs the tests",
			"type": "feedback", "rule": "Run the tests with pnpm, never npm", "why": "npm breaks the lock file here", "how": "Every time the tests run"}
		for k, v := range fields {
			body[k] = v
		}
		data, _ := json.Marshal(body)
		return memoryAsk(t, d, callerPage, "", "", "POST", memoryEditPath, string(data))
	}
	for what, row := range map[string]struct {
		fields map[string]string
		code   int
		said   string
	}{
		"a stale hash":          {map[string]string{"shown": strings.Repeat("0", 64)}, http.StatusConflict, "changed since it was shown"},
		"a name taken":          {map[string]string{"name": canaryNote.Name}, http.StatusConflict, "Another note already has this name"},
		"an e-mail":             {map[string]string{"why": "Write to alex.doe@example.com for access"}, http.StatusBadRequest, "e-mail address"},
		"a key's shape":         {map[string]string{"how": "Deploy with " + "ghp_" + "FAKEfakeFAKE"}, http.StatusBadRequest, memoryOwnerCantKeepEdit + ": "},
		"a field it won't take": {map[string]string{"state": "kept"}, http.StatusBadRequest, "nothing else"},
		"no such note":          {map[string]string{"id": "nope"}, http.StatusNotFound, "isn't here any more"},
	} {
		if code, text := edit(row.fields); code != row.code || !strings.Contains(text, row.said) {
			t.Errorf("%s: %d %s, want %d %q", what, code, text, row.code, row.said)
		}
	}
	if again := waitingShown(t, d, id); again.Shown != shown.Shown || again.Rule != "Run the tests with pnpm" {
		t.Fatalf("a refused edit changed the proposal: %+v", again)
	}
	code, text := edit(nil)
	var got struct {
		ID   string
		Kept bool
	}
	if code != http.StatusOK || json.Unmarshal([]byte(text), &got) != nil || !got.Kept || got.ID == "" || got.ID == id {
		t.Fatalf("the owner's edit: %d %s", code, text)
	}
	_, answer, read := readAsGM(t, d, gm, "?name=Tests+run+with+pnpm")
	if len(answer.Notes) != 1 || answer.Notes[0].Rule != "Run the tests with pnpm, never npm" || answer.Notes[0].Why != "npm breaks the lock file here" {
		t.Errorf("the GM reads the owner's words: %s", read)
	}
	list := ownerList(t, d)
	i := slices.IndexFunc(list.Kept, func(n memoryOwnerNote) bool { return n.ID == got.ID })
	if len(list.Waiting) != 0 || i < 0 || list.Kept[i].From != memoryFromOwner || list.Kept[i].Said != "" {
		t.Errorf("the owner's list after the edit: %+v", list)
	}
	if _, err := os.Stat(memoryPath(id + ".md")); !os.IsNotExist(err) {
		t.Errorf("the GM's words are still on disk: %v", err)
	}
	if code, _ := edit(nil); code != http.StatusNotFound {
		t.Errorf("the same proposal edited twice: %d", code)
	}
}

// Drop: a waiting note goes, its file with it; a kept note is never dropped.
func TestTheOwnerDropsANoteTheGMProposed(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	kept := keptNote(t, d, canaryNote)
	id := proposeAsGM(t, d, gm, docsFact)
	drop := func(id string) (int, string) {
		return memoryAsk(t, d, callerPage, "", "", "POST", memoryDropPath+"?id="+id, "")
	}
	if code, text := drop(kept.ID); code != http.StatusNotFound {
		t.Errorf("a kept note dropped: %d %s", code, text)
	}
	if code, text := drop(id); code != http.StatusOK || !strings.Contains(text, `"dropped":true`) {
		t.Fatalf("the owner's drop: %d %s", code, text)
	}
	if list := ownerList(t, d); len(list.Waiting) != 0 || len(list.Kept) != 1 {
		t.Errorf("after the drop: %+v", list)
	}
	if _, err := os.Stat(memoryPath(id + ".md")); !os.IsNotExist(err) {
		t.Errorf("a dropped note's file: %v", err)
	}
	if names := keptNames(d); !slices.Equal(names, []string{canaryNote.Name}) {
		t.Errorf("what the GM reads after the drop: %q", names)
	}
}

// Forget: a kept note goes, and the GM's next read doesn't have it; a waiting note is never forgotten.
func TestTheOwnerForgetsAKeptNote(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	kept := keptNote(t, d, canaryNote)
	id := proposeAsGM(t, d, gm, docsFact)
	forget := func(id string) (int, string) {
		return memoryAsk(t, d, callerPage, "", "", "POST", memoryForgetPath+"?id="+id, "")
	}
	if code, text := forget(id); code != http.StatusNotFound {
		t.Errorf("a waiting note forgotten: %d %s", code, text)
	}
	if code, text := forget(kept.ID); code != http.StatusOK || !strings.Contains(text, `"forgotten":true`) {
		t.Fatalf("the owner's forget: %d %s", code, text)
	}
	if _, answer, text := readAsGM(t, d, gm, ""); len(answer.Notes) != 0 || strings.Contains(strings.ToLower(text), "pelican") {
		t.Errorf("the GM's read after the forget: %s", text)
	}
	if list := ownerList(t, d); len(list.Kept) != 0 || len(list.Waiting) != 1 {
		t.Errorf("after the forget: %+v", list)
	}
}

// Only the owner: the GM, a hire with its key and a tool with the setup key get 403 at every route here, an MCP key
// and a night job are refused by the gate, and no key at all gets 401. Under the Mac app a browser's Setup only
// looks: it reads the list, and each change is refused with the Strict words; Smolcorp's own Settings window keeps.
// The table gives these rows to the page alone, and the office's count to its window alone.
func TestOnlyTheOwnerKeepsEditsDropsOrForgets(t *testing.T) {
	d, gm, hireKey := memoryDeck(t)
	kept := keptNote(t, d, canaryNote)
	waiting := proposeNote(t, d, named("Waiting for the owner"))
	keepNightJob(t, d, NightJob{ID: "nightly-tests", Status: nightApproved, ProposedBy: "page"})
	edit := `{"id":"` + waiting.ID + `","shown":"` + waiting.SHA256 + `","name":"Mine","description":"Mine","type":"project","body":"Mine."}`
	asks := []struct{ method, target, body string }{
		{"GET", memoryNotesPath, ""},
		{"POST", memoryKeepPath + "?id=" + waiting.ID + "&shown=" + waiting.SHA256, ""},
		{"POST", memoryEditPath, edit},
		{"POST", memoryDropPath + "?id=" + waiting.ID, ""},
		{"POST", memoryForgetPath + "?id=" + kept.ID, ""},
	}
	for _, ask := range asks {
		path, _, _ := strings.Cut(ask.target, "?")
		if row := routeFor(ask.method, path); row == nil || row.path != path || !slices.Equal(row.callers, []string{"page"}) {
			t.Errorf("%s %s: the table's row %+v", ask.method, path, row)
		}
		for _, who := range []caller{callerGM, callerHire, callerTool} {
			if code, text := memoryAsk(t, d, who, gm, hireKey, ask.method, ask.target, ask.body); code != http.StatusForbidden ||
				strings.Contains(strings.ToLower(text), "pelican") {
				t.Errorf("%s %s as %s: %d %s", ask.method, ask.target, who.name(), code, text)
			}
		}
		if code, _ := memoryAsk(t, d, callerNobody, gm, hireKey, ask.method, ask.target, ask.body); code != http.StatusUnauthorized {
			t.Errorf("%s %s with no key: %d", ask.method, ask.target, code)
		}
		for _, a := range []actor{mcpActor("claude-desktop"), nightActor("nightly-tests")} {
			if v := d.decide(a, action{route: &routeAsk{method: ask.method, path: path}}); v.allowed() || v.status != http.StatusForbidden {
				t.Errorf("%s %s by %s: %+v", ask.method, path, a, v)
			}
		}
	}
	all := d.memory.all()
	if len(all) != 2 || all[0].State != memoryKept || all[1].State != memoryWaiting {
		t.Fatalf("a refused caller changed memory: %+v", all)
	}

	// Under the Mac app: a browser's Setup reads the list and changes nothing; the window keeps.
	hostedByTheMacApp(d)
	browser, window := browserSession(d), windowSession(d)
	if got := withSession(t, d, browser, "GET", memoryNotesPath, ""); got.Code != http.StatusOK {
		t.Errorf("a browser's Setup on a Mac reads the list: %d %s", got.Code, got.Body)
	}
	for _, ask := range asks[1:] {
		if got := withSession(t, d, browser, ask.method, ask.target, ask.body); got.Code != http.StatusForbidden ||
			strings.TrimSpace(got.Body.String()) != strictChangeWords {
			t.Errorf("%s %s from a browser on a Mac: %d %s", ask.method, ask.target, got.Code, got.Body)
		}
	}
	if all := d.memory.all(); len(all) != 2 || all[1].State != memoryWaiting {
		t.Fatalf("a browser on a Mac changed memory: %+v", all)
	}
	if got := withSession(t, d, window, "POST", asks[1].target, ""); got.Code != http.StatusOK {
		t.Errorf("Smolcorp's own Settings window keeps: %d %s", got.Code, got.Body)
	}
	if names := keptNames(d); len(names) != 2 {
		t.Errorf("after the window's keep: %q", names)
	}

	// The office's badge: a count for its own window alone, never a phone's.
	if row := phoneRouteFor("GET", memoryWaitingPath); row == nil || row.access != accessWindow {
		t.Errorf("the badge's route: %+v", row)
	}
}

// The badge's count: how many notes wait, a number alone; 0 while the manager is off.
func TestTheOfficeWindowCountsWhatWaits(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	count := func() string {
		w := serve(t, d, call{method: "GET", target: memoryWaitingPath + "?t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
		if w.Code != http.StatusOK {
			t.Fatalf("the window's count: %d %s", w.Code, w.Body)
		}
		return strings.TrimSpace(w.Body.String())
	}
	if got := count(); got != `{"waiting":0}` {
		t.Errorf("nothing proposed: %s", got)
	}
	proposeAsGM(t, d, gm, docsFact)
	keptNote(t, d, canaryNote)
	if got := count(); got != `{"waiting":1}` {
		t.Errorf("one proposed, one kept: %s", got)
	}
	d.mu.Lock()
	d.config.ManagerEnabled = false
	d.mu.Unlock()
	if got := count(); got != `{"waiting":0}` {
		t.Errorf("the manager off: %s", got)
	}
}

// The store's caps hold through the owner's routes: past 200 kept notes, a keep and an edit are refused in the
// owner's own words (FXMEMKEEPR1 O1: not the GM's "delete or merge"), and the proposal still waits.
func TestTheOwnersKeepHoldsTheStoresCaps(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	for i := range memoryKeptMost {
		keptNote(t, d, memoryEntry{Name: fmt.Sprintf("Fact %d", i), Description: "A fact", Type: "project", Body: "The docs are in the docs folder."})
	}
	id := proposeAsGM(t, d, gm, docsFact)
	shown := waitingShown(t, d, id)
	code, text := memoryAsk(t, d, callerPage, "", "", "POST", memoryKeepPath+"?id="+id+"&shown="+shown.Shown, "")
	if code != http.StatusConflict || text != memoryOwnerKeptFull {
		t.Errorf("a keep past 200: %d %s", code, text)
	}
	body := `{"id":"` + id + `","shown":"` + shown.Shown + `","name":"Docs","description":"Docs","type":"project","body":"The docs folder."}`
	if code, text := memoryAsk(t, d, callerPage, "", "", "POST", memoryEditPath, body); code != http.StatusConflict ||
		text != memoryOwnerKeptFull {
		t.Errorf("an edit past 200: %d %s", code, text)
	}
	if memoryOwnerKeptFull != "Your General Manager already keeps 200 notes. Forget some before you keep this one." {
		t.Errorf("the owner's words for 200 notes: %q", memoryOwnerKeptFull)
	}
	if list := ownerList(t, d); len(list.Waiting) != 1 || len(list.Kept) != memoryKeptMost || list.KeptMost != memoryKeptMost {
		t.Errorf("after the refusals: %d waiting, %d kept, most %d", len(list.Waiting), len(list.Kept), list.KeptMost)
	}
}

// The index the GM reads holds too: long descriptions fill its 8 KB long before 200 notes, and the keep that would
// pass it is refused in the owner's own words (FXMEMKEEPR1 O1: no "index" or "8 KB").
func TestTheOwnersKeepHoldsTheIndexCap(t *testing.T) {
	d, _, _ := memoryDeck(t)
	long := strings.TrimSpace(strings.Repeat("Where the docs and the guides are kept ", 5)[:190])
	for i := 0; ; i++ {
		note := proposeNote(t, d, memoryEntry{Name: fmt.Sprintf("Long fact %d", i), Description: long, Type: "project", Body: "The docs folder."})
		code, text := memoryAsk(t, d, callerPage, "", "", "POST", memoryKeepPath+"?id="+note.ID+"&shown="+note.SHA256, "")
		if code == http.StatusOK {
			continue
		}
		if code != http.StatusConflict || text != memoryOwnerIndexFull || i < 30 {
			t.Errorf("the keep that fills the index (note %d): %d %s", i, code, text)
		}
		break
	}
	if index := memoryIndex(d.memory.all()); len(index) > memoryIndexMost {
		t.Errorf("the index is %d bytes", len(index))
	}
	if memoryOwnerIndexFull != "Your General Manager's notes are too long to read in one go. Forget some before you keep this one." {
		t.Errorf("the owner's words for a full index: %q", memoryOwnerIndexFull)
	}
}

// FXMEMKEEPR1 O2: an edit that changes nothing is refused, so a tap on "Keep my version" can't turn the GM's words
// into the owner's own and drop where they came from. The proposal still waits as it was, and Keep still keeps it.
func TestAnEditThatChangesNothingIsRefused(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	id := proposeAsGM(t, d, gm, docsFact)
	shown := waitingShown(t, d, id)
	edit := func(body string) (int, string) {
		data, _ := json.Marshal(map[string]string{"id": id, "shown": shown.Shown, "name": shown.Name, "description": shown.Description,
			"type": shown.Type, "body": body})
		return memoryAsk(t, d, callerPage, "", "", "POST", memoryEditPath, string(data))
	}
	if code, text := edit(shown.Body); code != http.StatusBadRequest || text != "Nothing changed: use Keep instead." {
		t.Errorf("an edit that changes nothing: %d %s", code, text)
	}
	if again := waitingShown(t, d, id); again.Shown != shown.Shown || again.From != memoryFromSeen || again.Said != "ls docs" {
		t.Errorf("the refused edit changed the proposal: %+v", again)
	}
	if code, text := memoryAsk(t, d, callerPage, "", "", "POST", memoryKeepPath+"?id="+id+"&shown="+shown.Shown, ""); code != http.StatusOK {
		t.Fatalf("Keep after the refused edit: %d %s", code, text)
	}
	if list := ownerList(t, d); len(list.Kept) != 1 || list.Kept[0].From != memoryFromSeen || list.Kept[0].Said != "ls docs" {
		t.Errorf("the kept note lost where its words came from: %+v", list)
	}
}

// FXMEMKEEPR2 O1: what the store refuses for what a note says reaches the owner in Setup's words: its own lead-in
// (an Edit's "your version", a Keep's "this note"), each field named as the edit form labels it, and one colon. An
// empty field and an e-mail address on an edit, then a Keep on a waiting note that today's checks refuse, and the
// list's refused in the same labels. The labels are the form's own (setup.html's memoryEdit).
func TestTheOwnersRefusalsAreInSetupsWords(t *testing.T) {
	d, gm, _ := memoryDeck(t)
	id := proposeAsGM(t, d, gm, docsFact)
	shown := waitingShown(t, d, id)
	edit := func(fields map[string]string) (int, string) {
		body := map[string]string{"id": id, "shown": shown.Shown, "name": "Docs folder", "description": "Where the docs are",
			"type": "reference", "body": "The user guide is in the docs folder, with the API."}
		for k, v := range fields {
			body[k] = v
		}
		data, _ := json.Marshal(body)
		return memoryAsk(t, d, callerPage, "", "", "POST", memoryEditPath, string(data))
	}
	oneColon := func(what, text string) {
		t.Helper()
		if strings.Count(text, ":") != 1 || strings.Contains(text, "Memory can't") {
			t.Errorf("%s: %q, want the owner's lead-in and one colon", what, text)
		}
	}
	for what, row := range map[string]struct{ fields map[string]string }{
		"an empty field": {map[string]string{"description": ""}},
		"an e-mail":      {map[string]string{"body": "Write to ada@example.com for the guide."}},
	} {
		code, text := edit(row.fields)
		want := map[string]string{
			"an empty field": "Smolcorp can't keep your version: the “One-line summary” field is empty.",
			"an e-mail":      "Smolcorp can't keep your version: the “Note” field has an e-mail address. A note never holds personal data.",
		}[what]
		if code != http.StatusBadRequest || text != want {
			t.Errorf("%s: %d %q\nwant 400 %q", what, code, text, want)
		}
		oneColon(what, text)
	}
	if again := waitingShown(t, d, id); again.Shown != shown.Shown {
		t.Fatalf("a refused edit changed the proposal: %+v", again)
	}

	// A Keep on the GM's words, which today's checks refuse: the note's own lead-in, and the list says why in the
	// same labels.
	d.memory.mu.Lock()
	for i := range d.memory.notes {
		if d.memory.notes[i].ID == id {
			d.memory.notes[i].Body = "Write to ada@example.com for the guide."
		}
	}
	d.memory.mu.Unlock()
	if got := waitingShown(t, d, id).Refused; got != "the “Note” field has an e-mail address: memory never keeps personal data" {
		t.Errorf("the list's refused: %q", got)
	}
	code, text := memoryAsk(t, d, callerPage, "", "", "POST", memoryKeepPath+"?id="+id+"&shown="+shown.Shown, "")
	if want := "Smolcorp can't keep this note: the “Note” field has an e-mail address. A note never holds personal data."; code != http.StatusBadRequest || text != want {
		t.Errorf("a Keep today's checks refuse: %d %q\nwant 400 %q", code, text, want)
	}
	oneColon("a Keep", text)
	if names := keptNames(d); len(names) != 0 {
		t.Errorf("a refused keep kept: %q", names)
	}

	// Each label is one the edit form shows.
	page, err := os.ReadFile(filepath.Join("..", "Resources", "web", "setup.html"))
	if err != nil {
		t.Fatal(err)
	}
	for field, label := range map[string]string{"name": "Name", "description": "One-line summary", "body": "Note", "rule": "Rule",
		"why": "Why", "how": "How to apply it"} {
		form := fmt.Sprintf("['%s', '%s']", field, label)
		said := "the “" + label + "” field "
		if !strings.Contains(string(page), form) || !slices.ContainsFunc(memoryOwnerLabels, func(l [2]string) bool { return l[1] == said }) {
			t.Errorf("the %s field: the form's %s, the refusal's %q", field, form, said)
		}
	}
}

// FXMEMKEEPR2 O5: memoryOwnerRoute checks its caller itself, not only through the route table: called straight, as a
// table that let another caller in would, the GM, a hire, a tool and no one get 403 at every route, and nothing
// changes; the page gets its list.
func TestTheOwnersMemoryRoutesCheckTheCallerThemselves(t *testing.T) {
	d, _, _ := memoryDeck(t)
	kept := keptNote(t, d, canaryNote)
	waiting := proposeNote(t, d, named("Waiting for the owner"))
	edit := `{"id":"` + waiting.ID + `","shown":"` + waiting.SHA256 + `","name":"Mine","description":"Mine","type":"project","body":"Mine."}`
	asks := []struct{ method, target, body string }{
		{"GET", memoryNotesPath, ""},
		{"POST", memoryKeepPath + "?id=" + waiting.ID + "&shown=" + waiting.SHA256, ""},
		{"POST", memoryEditPath, edit},
		{"POST", memoryDropPath + "?id=" + waiting.ID, ""},
		{"POST", memoryForgetPath + "?id=" + kept.ID, ""},
	}
	straight := func(who caller, method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		if !d.memoryOwnerRoute(w, &request{Request: httptest.NewRequest(method, target, strings.NewReader(body)), caller: who}) {
			t.Fatalf("%s %s: not the owner's route", method, target)
		}
		return w
	}
	for _, ask := range asks {
		for _, who := range []caller{callerGM, callerHire, callerTool, callerNobody} {
			if w := straight(who, ask.method, ask.target, ask.body); w.Code != http.StatusForbidden ||
				strings.TrimSpace(w.Body.String()) != memoryOwnerOnly {
				t.Errorf("%s %s as %s, straight: %d %s", ask.method, ask.target, who.name(), w.Code, w.Body)
			}
		}
	}
	if all := d.memory.all(); len(all) != 2 || all[0].State != memoryKept || all[1].State != memoryWaiting || all[1].ID != waiting.ID {
		t.Fatalf("a refused caller changed memory: %+v", all)
	}
	if w := straight(callerPage, "GET", memoryNotesPath, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), waiting.ID) {
		t.Errorf("the page, straight: %d %s", w.Code, w.Body)
	}
}
