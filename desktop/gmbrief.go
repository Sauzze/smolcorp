package desktop

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// POST /setup/gm/brief, as the Mac's BriefMore: {id, text}: the dated text goes to the end of the hire's BRIEF file, then a mail (as /setup/gm/send's) tells the hire. Nothing is typed.
// The file's side (which file, the dated part, one write) is briefmore.go's, shared with the Mac's fixtures.

func (rt *router) gmBrief(w http.ResponseWriter, r *request) {
	d, refuse := rt.deck, writeStatus(w)
	asked := gmAsked(r)
	id, _ := asked["id"].(string)
	text, hasText := asked["text"].(string)
	most := d.mailLimits().MaxBytes
	h, found := hireByID(id)
	file := briefFileOf(h, filepath.Join(supportDir, "hires"))
	switch {
	case strings.TrimSpace(id) == "" || !hasText:
		refuse(http.StatusBadRequest, "Send {id, text}: which hire, and the new part of its brief.")
	case strings.TrimSpace(text) == "":
		refuse(http.StatusBadRequest, "The brief is empty.")
	case !found:
		refuse(http.StatusNotFound, gmFireNoHire)
	case h.Role == gmRole:
		refuse(http.StatusConflict, "The GM has no brief to add to; it is the one who writes them.")
	case !isOpen(h):
		refuse(http.StatusConflict, calledHire(h)+" is already gone.")
	case len(text) > most:
		refuse(http.StatusRequestEntityTooLarge, fmt.Sprintf("That brief is %d bytes. The limit is %d bytes (setting mailMaxBytes).", len(text), most))
	case file == "":
		refuse(http.StatusConflict, calledHire(h)+" has no BRIEF file to add to.")
	default:
		// Words for a desk on one side of the sandbox from the GM on the other: neither the brief nor its mail
		// (AQ10, sandboxswitch.go).
		if refusal := d.mailSandboxRefusal(r.caller, nil, mailDest{ID: h.ID, Name: calledHire(h)}); refusal != nil {
			refuse(refusal.Status, refusal.Message)
			return
		}
		if err := appendBriefPart(file, text, time.Now()); err != nil {
			refuse(http.StatusInternalServerError, "Couldn't add to the brief file.")
			return
		}
		// A brief is the GM's words, never the owner's (ownerwords.go).
		d.owner.notOwners(text)
		name := file // by file name in the hire's own folder, else by whole path
		if physical(filepath.Dir(file)) == physical(h.Folder) {
			name = filepath.Base(file)
		}
		answer := map[string]any{"ok": true, "file": file}
		added, refusal := d.mail.add(mailRequest{From: mailSenderOf(r.caller), To: h.ID, Text: briefMoreNote(name), ID: briefMailID(), IDGiven: true,
			Resolve: func() (mailDest, *mailError) { return d.mailTarget(h.ID) }})
		if refusal != nil {
			answer["note"] = "The new part is in the brief, but the hire couldn't be mailed: " + refusal.Message
		} else {
			answer["mail"] = added.Answer
		}
		writeJSON(w, http.StatusOK, answer)
	}
}

func briefMailID() string { return "brief-" + strings.TrimPrefix(newMailID(), "m-") }
