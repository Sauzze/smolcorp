package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// The owner's side of the GM's memory (1.26, FXMEMKEEP: MEM4's missing half, the routes MEM5 and MEM6 planned). The
// GM proposes a note (memoryroutes.go); here the owner sees what waits, and keeps it, edits it then keeps it, or drops
// it; and sees what's kept, and forgets a note. Setup's "What your General Manager has learnt" asks these, and the
// office's own window asks how many notes wait, for its badge.
//
// What holds here:
//   - every route is the page's alone (routes.go: the rows under /setup/gm/memory/ and its catch-all), and a caller
//     must be one of the owner's channels (grants.go's isOwnerTap) again here. The GM, a hire, a tool, an MCP key and
//     a night job are refused before these run (the route table, gates.go's memoryRouteRefusal);
//   - under the Mac app, the page is Smolcorp's own Settings window: every change here is a POST, so a browser's Setup
//     only looks (routes.go's windowOnlyRefusal). Off a Mac it's the page's session, as every Strict route is;
//   - a keep or an edit carries the hash of the note as it was shown: one that changed since gets 409;
//   - an edit makes the words the owner's own (memoryFromOwner), checked again as a new note is (MEM2's checks), and
//     kept in one step. Nothing here widens what a note may hold;
//   - the store's own rules stand: its caps, its seal and rotation, recordsRefusal, and a kept note that fails
//     today's checks stays withheld from the GM (gmMemory.kept), whatever the page shows of it;
//   - the badge's count is a number alone, never a word of a note, for the office's own window (accessWindow): the
//     Mac's, a browser's on this computer, or Windows' own window (its paired device, let in since FXTALKFIX).
//
// No phone route keeps a note yet: MEM5's phone card isn't built, and the phone keeps nothing it hasn't shown.

const (
	memoryNotesPath  = "/setup/gm/memory/notes"
	memoryKeepPath   = "/setup/gm/memory/keep"
	memoryEditPath   = "/setup/gm/memory/edit"
	memoryDropPath   = "/setup/gm/memory/drop"
	memoryForgetPath = "/setup/gm/memory/forget"
	// memoryWaitingPath is the office window's: how many notes wait for the owner.
	memoryWaitingPath = "/memory/waiting"

	memoryOwnerOnly = "Only the owner keeps, changes or forgets the General Manager's notes, from Setup."

	// The owner's words for the store's caps and for an edit that changes nothing (FXMEMKEEPR1). memory.go's errors
	// stay the GM's words.
	memoryOwnerKeptFull  = "Your General Manager already keeps 200 notes. Forget some before you keep this one."
	memoryOwnerIndexFull = "Your General Manager's notes are too long to read in one go. Forget some before you keep this one."
	memoryOwnerUnchanged = "Nothing changed: use Keep instead."

	// How the owner's refusal opens when the store won't take a note for what it says (FXMEMKEEPR2 O1): a Keep's, on
	// the GM's words, and an Edit's, on the owner's own. memoryRefused's "Memory can't keep this note" stays the GM's.
	memoryOwnerCantKeep     = "Smolcorp can't keep this note"
	memoryOwnerCantKeepEdit = "Smolcorp can't keep your version"
)

// memoryOwnerLabels are the starts of the store's reasons that name a note's field (memoryFields, memoryFieldRefusal,
// unshowable), each with the field as Setup's card and edit form label it. A body too long is the note's words
// together (rule, why and how for a rule), so it names no one field.
var memoryOwnerLabels = [][2]string{
	{"its body is longer than ", "its words are longer than "},
	{"its name ", "the “Name” field "},
	{"its description ", "the “One-line summary” field "},
	{"its body ", "the “Note” field "},
	{"its rule ", "the “Rule” field "},
	{"its why ", "the “Why” field "},
	{"its how to apply it ", "the “How to apply it” field "},
	{"its source ", "the quote it came from "},
}

// memoryOwnerLabelled is the store's reason with the field it names as Setup labels it: the list's refused, which
// the card and the edit form show in brackets.
func memoryOwnerLabelled(reason string) string {
	for _, label := range memoryOwnerLabels {
		if rest, found := strings.CutPrefix(reason, label[0]); found {
			return label[1] + rest
		}
	}
	return reason
}

// memoryOwnerRefusal is the store's reason for refusing a note, in the owner's words: lead, then the reason with its
// field named as Setup labels it, and the reason's own colon made a second sentence, so two never follow each other
// ("Smolcorp can't keep your version: the “Why” field has an e-mail address. A note never holds personal data.").
func memoryOwnerRefusal(lead, reason string) string {
	reason = memoryOwnerLabelled(reason)
	if said, why, found := strings.Cut(reason, ": "); found && why != "" {
		why = strings.Replace(why, "memory never keeps ", "a note never holds ", 1)
		reason = said + ". " + strings.ToUpper(why[:1]) + why[1:]
	}
	if !strings.HasSuffix(reason, ".") {
		reason += "."
	}
	return lead + ": " + reason
}

// memoryOwnerNote is a note as the owner's list shows it: every field whole, where its words came from, and for a
// waiting note the hash a keep or an edit carries back (shown).
type memoryOwnerNote struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Body        string `json:"body,omitempty"`
	Rule        string `json:"rule,omitempty"`
	Why         string `json:"why,omitempty"`
	How         string `json:"how,omitempty"`
	From        string `json:"from"`
	Said        string `json:"said,omitempty"`
	At          string `json:"at"`
	Shown       string `json:"shown,omitempty"`
	// Held is why a waiting note needs a close look (MEM2's hold, a quote that leaves out a negation); Refused is why
	// today's checks would refuse it, so a keep can't stand and a kept note is withheld from the GM.
	Held    string `json:"held,omitempty"`
	Refused string `json:"refused,omitempty"`
}

// memoryOwnerList is GET /setup/gm/memory/notes.
type memoryOwnerList struct {
	Waiting     []memoryOwnerNote `json:"waiting"`
	Kept        []memoryOwnerNote `json:"kept"`
	WaitingMost int               `json:"waitingMost"`
	KeptMost    int               `json:"keptMost"`
}

// memoryEdit is POST /setup/gm/memory/edit's body: the note shown, and the owner's words for it.
type memoryEdit struct {
	ID          string `json:"id"`
	Shown       string `json:"shown"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Body        string `json:"body"`
	Rule        string `json:"rule"`
	Why         string `json:"why"`
	How         string `json:"how"`
}

// memoryOwnerRoute serves the owner's memory routes, and whether path was one. memoryRoute hands it every path
// under /setup/gm/memory/ that isn't the GM's own.
func (d *Deck) memoryOwnerRoute(w http.ResponseWriter, r *request) bool {
	path := r.URL.Path
	if !isMemoryOwnerRoute(path) {
		return false
	}
	if r.caller != callerPage || !isOwnerTap(actorOf(r.caller)) {
		writeStatus(w)(http.StatusForbidden, memoryOwnerOnly)
		return true
	}
	if d.memory == nil {
		writeStatus(w)(http.StatusConflict, "Smolcorp has no memory for its GM here.")
		return true
	}
	id := r.URL.Query().Get("id")
	switch {
	case r.Method == http.MethodGet && path == "/setup/gm/memory/notes":
		writeJSON(w, http.StatusOK, d.memoryOwnerList())
	case r.Method == http.MethodPost && path == "/setup/gm/memory/keep":
		err := d.memory.keep(id, r.URL.Query().Get("shown"))
		writeAnswer(w)(memoryOwnerAnswer(err, memoryOwnerCantKeep, map[string]any{"id": id, "kept": true}))
	case r.Method == http.MethodPost && path == "/setup/gm/memory/edit":
		var e memoryEdit
		decoder := json.NewDecoder(io.LimitReader(r.Body, memoryProposalMost+1))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&e) != nil || decoder.More() {
			writeStatus(w)(http.StatusBadRequest, "Send the note as JSON: id, shown, name, description, type, then body (or rule, why "+
				"and how for a rule), nothing else.")
			return true
		}
		// One name, one note, as a proposal's: the GM can't propose this name while the edit takes it.
		memoryProposing.Lock()
		note, err := d.memory.keepEdited(e.ID, e.Shown, memoryEntry{Name: e.Name, Description: e.Description, Type: e.Type,
			Body: e.Body, Rule: e.Rule, Why: e.Why, How: e.How}, time.Now())
		memoryProposing.Unlock()
		writeAnswer(w)(memoryOwnerAnswer(err, memoryOwnerCantKeepEdit, map[string]any{"id": note.ID, "name": note.Name, "kept": true}))
	case r.Method == http.MethodPost && path == "/setup/gm/memory/drop":
		err := d.memory.remove(id, true)
		writeAnswer(w)(memoryOwnerAnswer(err, memoryOwnerCantKeep, map[string]any{"id": id, "dropped": true}))
	case r.Method == http.MethodPost && path == "/setup/gm/memory/forget":
		err := d.memory.removeKept(id)
		writeAnswer(w)(memoryOwnerAnswer(err, memoryOwnerCantKeep, map[string]any{"id": id, "forgotten": true}))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	return true
}

// isMemoryOwnerRoute is whether path is one of the owner's memory routes.
func isMemoryOwnerRoute(path string) bool {
	switch path {
	case memoryNotesPath, memoryKeepPath, memoryEditPath, memoryDropPath, memoryForgetPath:
		return true
	}
	return false
}

// memoryOwnerAnswer is a change's answer: done, or what the store said, in the owner's words. lead opens a refusal of
// the note for what it says (memoryOwnerRefusal), which only a keep or an edit gets.
func memoryOwnerAnswer(err error, lead string, done map[string]any) (int, any) {
	var refused *memoryRefused
	switch {
	case err == nil:
		return http.StatusOK, done
	case errors.Is(err, errMemoryNotFound):
		return http.StatusNotFound, "That note isn't here any more: the list is up to date again."
	case errors.Is(err, errMemoryChanged):
		return http.StatusConflict, "That note changed since it was shown: look at it again before you keep it."
	case errors.Is(err, errMemoryNameTaken):
		return http.StatusConflict, "Another note already has this name: give this one another."
	case errors.Is(err, errMemoryKeptFull):
		return http.StatusConflict, memoryOwnerKeptFull
	case errors.Is(err, errMemoryIndexFull):
		return http.StatusConflict, memoryOwnerIndexFull
	case errors.Is(err, errMemoryUnchanged):
		return http.StatusBadRequest, memoryOwnerUnchanged
	case errors.As(err, &refused):
		return http.StatusBadRequest, memoryOwnerRefusal(lead, refused.reason)
	case recordsRefusal() != "":
		return http.StatusConflict, err.Error()
	}
	return http.StatusInternalServerError, "The note couldn't be saved: " + err.Error()
}

// memoryOwnerList is every note the store holds, as the owner's list shows it: those waiting, then those kept, each
// in the order it came. Each is checked with today's lists, as the GM's read checks it.
func (d *Deck) memoryOwnerList() memoryOwnerList {
	list := memoryOwnerList{Waiting: []memoryOwnerNote{}, Kept: []memoryOwnerNote{}, WaitingMost: memoryWaitingMost, KeptMost: memoryKeptMost}
	for _, n := range d.memory.all() {
		one := memoryOwnerNote{ID: n.ID, Name: n.Name, Type: n.Type, Description: n.Description, Body: n.Body, Rule: n.Rule,
			Why: n.Why, How: n.How, From: n.Source.Tag, Said: n.Source.Quote, At: n.At.Format(time.RFC3339),
			Refused: memoryOwnerLabelled(memoryNoteRefusal(n))}
		if n.State == memoryKept {
			list.Kept = append(list.Kept, one)
			continue
		}
		one.Shown = n.SHA256
		if hold := memoryQuoteHold(n.Source); hold.English != "" {
			one.Held = hold.English
		} else if _, hold := memoryRefusal(n.entry(), n.Source.Quote); hold.English != "" {
			one.Held = hold.English
		}
		list.Waiting = append(list.Waiting, one)
	}
	return list
}

// The office's own window asks how many notes wait for the owner, for the badge on its ⚙︎: a number alone. Never a
// phone (accessWindow), and 0 while the manager is off, when Setup shows no list. Windows' own office window reads the
// office as its paired device ("Smolcorp (window)"), which accessWindow lets in since FXTALKFIX
// (request.windowsOwnWindow), so it gets the count here too, as at every window-only route (docs/windows-linux.md).
func init() {
	registerPhoneRoute(phoneRoute{method: http.MethodGet, path: "/memory/waiting", access: accessWindow,
		refusal: says("Your General Manager's notes are counted in the office's own window."),
		serve: func(d *Deck, w http.ResponseWriter, _ *request, _ string) {
			d.mu.Lock()
			on := d.config.ManagerEnabled
			d.mu.Unlock()
			waiting := 0
			if on && d.memory != nil {
				for _, n := range d.memory.all() {
					if n.State == memoryWaiting {
						waiting++
					}
				}
			}
			writeJSON(w, http.StatusOK, map[string]int{"waiting": waiting})
		}})
}
