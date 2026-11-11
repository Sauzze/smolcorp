package desktop

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// POST /setup/gm/brief, as the Mac's BriefMore: {id, text}: the dated text goes to the end of the hire's BRIEF file, then a mail (as /setup/gm/send's) tells the hire. Nothing is typed.

const briefNote = "Your brief has a new part: read "

func (rt *router) gmBrief(w http.ResponseWriter, r *request) {
	d, refuse := rt.deck, writeStatus(w)
	asked := gmAsked(r)
	id, _ := asked["id"].(string)
	text, hasText := asked["text"].(string)
	most := d.mailLimits().MaxBytes
	h, found := hireByID(id)
	file := briefFileOf(h)
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
		if err := appendBriefPart(file, text, time.Now()); err != nil {
			refuse(http.StatusInternalServerError, "Couldn't add to the brief file.")
			return
		}
		name := file // by file name in the hire's own folder, else by whole path
		if physical(filepath.Dir(file)) == physical(h.Folder) {
			name = filepath.Base(file)
		}
		answer := map[string]any{"ok": true, "file": file}
		added, refusal := d.mail.add(mailRequest{From: mailSenderOf(r.caller), To: h.ID, Text: briefNote + name, ID: briefMailID(), IDGiven: true,
			Resolve: func() (mailDest, *mailError) { return d.mailTarget(h.ID) }})
		if refusal != nil {
			answer["note"] = "The new part is in the brief, but the hire couldn't be mailed: " + refusal.Message
		} else {
			answer["mail"] = added.Answer
		}
		writeJSON(w, http.StatusOK, answer)
	}
}

// briefFileOf is the plain BRIEF file a hire reads: its registered path, else a managed hire's first BRIEF*.md in the support folder; "" when none.
func briefFileOf(h Hire) string {
	names := []string{h.Brief}
	if h.Brief == "" && h.Worktree != "" {
		names, _ = filepath.Glob(filepath.Join(hireFolder(supportDir, h.ID), "BRIEF*.md"))
	}
	for _, name := range names {
		if info, err := os.Lstat(name); err == nil && info.Mode().IsRegular() {
			return name
		}
	}
	return ""
}

// appendBriefPart adds the dated part to file in one write; never makes the file, refuses a link.
func appendBriefPart(file, text string, at time.Time) error {
	before, err := os.Lstat(file)
	if err != nil || !before.Mode().IsRegular() {
		return fmt.Errorf("not a plain file")
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	if open, err := f.Stat(); err != nil || !os.SameFile(before, open) {
		return fmt.Errorf("the file changed")
	}
	_, err = f.WriteString("\n\n## New part, " + at.Format("2006-01-02 15:04") + "\n\n" + strings.TrimSpace(text) + "\n")
	return err
}

func briefMailID() string { return "brief-" + strings.TrimPrefix(newMailID(), "m-") }
