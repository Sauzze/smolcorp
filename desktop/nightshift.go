package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// The night shift's jobs (1.26 N1): what to do, when, with what limits, and whether the owner said yes. Kept in
// nightshift.json in the support folder, written whole (a temp file put in its place), readable by you alone.
//
// Who may do what (answer 9: the GM proposes, the owner approves each job):
//   - the page (the owner) creates, edits, approves, pauses and deletes;
//   - the GM lists jobs and proposes one, which never runs until the page approves it;
//   - a tool with the setup key lists; an MCP key, a night job, a hire and a phone can do none of it.
//
// Nothing here runs a job: the scheduler (nightschedule.go) says when one is due, and N2 runs it as the actor
// night:<job>, through the gate service (gates.go).

const (
	nightFileName    = "nightshift.json"
	nightFileMost    = 512 << 10 // bytes of nightshift.json read; a bigger file is not read at all
	nightBodyMost    = 64 << 10  // bytes of a request's body
	nightJobsMost    = 50        // jobs kept, approved, paused and proposed together
	nightProposeMost = 10        // proposals waiting for the owner at once
	nightNameMost    = 80        // characters of a job's name
	nightBriefMost   = 4000      // characters of a brief or a GM task
	nightFolderMost  = 1024      // bytes of a folder's path
	nightWallLeast   = 5         // minutes a job may run, at least
	nightWallMost    = 8 * 60    // and at most
	nightWallDefault = 60
)

// A job's status. Only an approved job is ever due.
const (
	nightProposed = "proposed"
	nightApproved = "approved"
	nightPaused   = "paused"
)

// What a job is: a hire of a role, with a brief, in a folder; or a task for the GM.
const (
	nightHire = "hire"
	nightGM   = "gm"
)

// When it runs: every day or one day a week at a time, or when its named trigger is called (N3).
const (
	nightDaily   = "daily"
	nightWeekly  = "weekly"
	nightTrigger = "trigger"
)

// The night shift's routes (goOnlyRoutes lists who may use each).
const (
	nightListPath    = "/setup/night"
	nightProposePath = "/setup/night/propose"
	nightSavePath    = "/setup/night/save"
	nightApprovePath = "/setup/night/approve"
	nightPausePath   = "/setup/night/pause"
	nightDeletePath  = "/setup/night/delete"
)

// isNightRoute is whether path is one of the night shift's routes.
func isNightRoute(path string) bool {
	return path == nightListPath || strings.HasPrefix(path, nightListPath+"/")
}

var (
	nightIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	nightAtPattern    = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	nightModelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\[\]-]{0,63}$`)
	nightWeekdays     = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
)

// NightWhen is when a job runs: At ("HH:MM", the computer's time) every day, or on Day (a weekday's English name,
// lower case) each week; or when the trigger named Trigger is called.
type NightWhen struct {
	Every   string `json:"every"`
	Day     string `json:"day,omitempty"`
	At      string `json:"at,omitempty"`
	Trigger string `json:"trigger,omitempty"`
}

// NightLimits are a job's limits: how long it may run, and the model and effort it runs on ("" for the role's).
// One job at a time is the scheduler's rule for every job.
type NightLimits struct {
	WallMinutes int    `json:"wallMinutes"`
	Model       string `json:"model,omitempty"`
	Effort      string `json:"effort,omitempty"`
}

// NightJob is one job as nightshift.json keeps it. Since and LastSlot are the scheduler's, never taken from a
// request: the moment from which the next run is looked for (the approval, an edit of when, the last run), and
// the date ("2006-01-02", the computer's) of the last run's slot, so a date runs once whatever the clock does.
type NightJob struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Role       string      `json:"role,omitempty"`
	Brief      string      `json:"brief"`
	Folder     string      `json:"folder,omitempty"`
	When       NightWhen   `json:"when"`
	Limits     NightLimits `json:"limits"`
	Status     string      `json:"status"`
	ProposedBy string      `json:"proposedBy"`
	Created    time.Time   `json:"created"`
	Since      time.Time   `json:"since,omitzero"`
	LastSlot   string      `json:"lastSlot,omitempty"`
}

// actor is who the job acts as, for the gate service: night:<id>.
func (j NightJob) actor() actor { return nightActor(j.ID) }

// nightAsk is what a request may say of a job: a proposal, or the page's create or edit. Anything else in the
// body is refused, so a proposal can't name its status, its id or the scheduler's fields.
type nightAsk struct {
	Name   string      `json:"name"`
	Kind   string      `json:"kind"`
	Role   string      `json:"role"`
	Brief  string      `json:"brief"`
	Folder string      `json:"folder"`
	When   NightWhen   `json:"when"`
	Limits NightLimits `json:"limits"`
}

// nightSave is the page's create (no id) or edit (an id): the ask, and for a create its status, approved
// unless it says paused.
type nightSave struct {
	nightAsk
	ID     string `json:"id"`
	Status string `json:"status"`
}

// checkedShape is why j's fields can't be a job ("" when they can), as a sentence: what a request and a read of
// the file both check. Whether the folder is there and the role is known is checkedAsk's, for a request.
func (j NightJob) checkedShape() string {
	switch {
	case !nightIDPattern.MatchString(j.ID):
		return "Its id isn't one Smolcorp makes."
	case !slices.Contains([]string{nightProposed, nightApproved, nightPaused}, j.Status):
		return "Its status is proposed, approved or paused."
	case j.ProposedBy != "gm" && j.ProposedBy != "page":
		return "It says neither the GM nor the page made it."
	case j.LastSlot != "" && !validDate(j.LastSlot):
		return "Its last run's date can't be read."
	}
	return nightAsk{Name: j.Name, Kind: j.Kind, Role: j.Role, Brief: j.Brief, Folder: j.Folder, When: j.When, Limits: j.Limits}.checkedWords()
}

func validDate(text string) bool { _, err := time.Parse(time.DateOnly, text); return err == nil }

// nightBlankLinesMost is how many blank lines (empty, or spaces and tabs only) a brief may have in a row: one, as
// between paragraphs, never a column of them that pushes what follows out of the page's view (RT126N1 B1).
const nightBlankLinesMost = 1

// blankLinesInARow is the most blank lines text has in a row (before its first line break and after its last count).
func blankLinesInARow(text string) int {
	most, run := 0, 0
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == "" {
			run++
			most = max(most, run)
		} else {
			run = 0
		}
	}
	return most
}

// checkedWords is why an ask can't be a job, its folder and role aside: "" when it can. The owner approves from the
// name, the brief and the folder as the GM wrote them, so none may hide a part of itself (RT126N1 B1): no character
// hidesText names (a brief keeps its line breaks and tabs), and no blank lines in a row that push the rest of a
// brief out of sight. A file's entry is checked the same way (checkedShape), so a planted one is skipped.
func (a nightAsk) checkedWords() string {
	oneLine := func(s string) bool { return !strings.ContainsAny(s, "\r\n\x00") }
	hidden := func(what, text, kept string) string {
		for _, c := range text {
			if hidesText(c) && !strings.ContainsRune(kept, c) {
				return fmt.Sprintf("The job's %s has a character the owner can't see as it is (U+%04X).", what, c)
			}
		}
		return ""
	}
	switch {
	case strings.TrimSpace(a.Name) == "" || utf8.RuneCountInString(a.Name) > nightNameMost || !oneLine(a.Name) || !utf8.ValidString(a.Name):
		return fmt.Sprintf("Give the job a name, on one line, of %d characters at most.", nightNameMost)
	case hidden("name", a.Name, "") != "":
		return hidden("name", a.Name, "")
	case strings.TrimSpace(a.Brief) == "" || utf8.RuneCountInString(a.Brief) > nightBriefMost || strings.ContainsRune(a.Brief, 0) || !utf8.ValidString(a.Brief):
		return fmt.Sprintf("Give the job a brief of %d characters at most.", nightBriefMost)
	case hidden("brief", a.Brief, "\n\t") != "":
		return hidden("brief", a.Brief, "\n\t")
	case blankLinesInARow(a.Brief) > nightBlankLinesMost:
		return fmt.Sprintf("The job's brief has more than %d blank line in a row, which could push the rest out of sight.", nightBlankLinesMost)
	case a.Kind == nightHire && (a.Role == "" || a.Folder == ""):
		return "A hire's job names a role and a folder."
	case a.Kind == nightHire && (len(a.Folder) > nightFolderMost || !oneLine(a.Folder) || !filepath.IsAbs(a.Folder)):
		return "Give the folder's full path."
	case !utf8.ValidString(a.Folder):
		return "The job's folder isn't text."
	case hidden("folder", a.Folder, "") != "":
		return hidden("folder", a.Folder, "")
	case a.Kind == nightGM && (a.Role != "" || a.Folder != ""):
		return "A task for the GM names no role or folder: the GM picks them."
	case a.Kind != nightHire && a.Kind != nightGM:
		return "A job is a hire (a role, a brief, a folder) or a task for the GM."
	case a.Limits.WallMinutes < nightWallLeast || a.Limits.WallMinutes > nightWallMost:
		return fmt.Sprintf("A job runs for %d minutes at least and %d hours at most.", nightWallLeast, nightWallMost/60)
	case a.Limits.Model != "" && !nightModelPattern.MatchString(a.Limits.Model):
		return "That model's name can't be one."
	case a.Limits.Effort != "" && !slices.Contains(effortLevels, a.Limits.Effort):
		return "The effort is low, medium, high, xhigh or max."
	}
	return a.When.checked()
}

// checked is why w can't say when a job runs: "" when it can.
func (w NightWhen) checked() string {
	switch w.Every {
	case nightDaily, nightWeekly:
		if !nightAtPattern.MatchString(w.At) {
			return "Say the time as HH:MM, from 00:00 to 23:59."
		}
		if w.Trigger != "" {
			return "A job runs at a time or by a trigger, not both."
		}
		if (w.Every == nightWeekly) != slices.Contains(nightWeekdays, w.Day) {
			return "A weekly job names its day (monday to sunday); a daily one names none."
		}
	case nightTrigger:
		if !nightIDPattern.MatchString(w.Trigger) || w.At != "" || w.Day != "" {
			return "A trigger's name is lower-case letters, digits and dashes, and it has no time or day."
		}
	default:
		return "A job runs daily, weekly, or by a trigger."
	}
	return ""
}

// nightShift is the job store, and the scheduler that reads it.
type nightShift struct {
	mu   sync.Mutex
	file string
	jobs []NightJob
	// skipped is a word for each entry of the file that couldn't be read, or for the file itself.
	skipped []string
	// unreadable is whether the file as read wasn't all kept: it is set aside as nightshift.json.bad, once,
	// before anything is written over it.
	unreadable bool
	sched      *nightScheduler
}

// newNightShift reads the jobs kept in file, skipping what can't be a job with a word.
func newNightShift(file string, now func() time.Time) *nightShift {
	n := &nightShift{file: file}
	n.jobs, n.skipped = readNightJobs(file)
	n.unreadable = len(n.skipped) > 0
	n.sched = newNightScheduler(n, now)
	return n
}

// readNightJobs is the jobs file holds, and a word for each entry skipped. No file is no jobs and no word.
func readNightJobs(file string) ([]NightJob, []string) {
	f, err := os.Open(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []string{"The night shift's jobs couldn't be read: " + nightFileName + " can't be opened."}
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() > nightFileMost {
		return nil, []string{fmt.Sprintf("The night shift's jobs weren't read: %s isn't a file of %d KB at most.", nightFileName, nightFileMost>>10)}
	}
	data, _ := io.ReadAll(io.LimitReader(f, nightFileMost+1))
	var kept struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if len(data) > nightFileMost || json.Unmarshal(data, &kept) != nil {
		return nil, []string{"The night shift's jobs weren't read: " + nightFileName + " isn't the list Smolcorp writes."}
	}
	var jobs []NightJob
	var skipped []string
	for i, raw := range kept.Jobs {
		var j NightJob
		why := ""
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		switch {
		case decoder.Decode(&j) != nil:
			why = "it isn't a job as Smolcorp writes one."
		case len(jobs) >= nightJobsMost:
			why = fmt.Sprintf("only %d jobs are kept.", nightJobsMost)
		case slices.ContainsFunc(jobs, func(o NightJob) bool { return o.ID == j.ID }):
			why = "another job has its id."
		default:
			why = lowerFirst(j.checkedShape())
		}
		if why != "" {
			skipped = append(skipped, fmt.Sprintf("Night job %d in %s was skipped: %s", i+1, nightFileName, why))
			continue
		}
		jobs = append(jobs, j)
	}
	return jobs, skipped
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// saveLocked writes the jobs whole: to a new temp file beside it, readable by you alone, then put in its place.
// A file read with entries skipped is first set aside as nightshift.json.bad, so nothing of it is lost unseen.
// With n.mu held.
func (n *nightShift) saveLocked() error {
	data, err := json.MarshalIndent(struct {
		Jobs []NightJob `json:"jobs"`
	}{Jobs: n.jobs}, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > nightFileMost {
		return errors.New("the night shift's jobs are over the size kept")
	}
	if n.unreadable {
		if err := os.Rename(n.file, n.file+".bad"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		n.unreadable = false
	}
	temp, err := os.CreateTemp(filepath.Dir(n.file), filepath.Base(n.file)+".*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	err = temp.Chmod(0o600)
	if err == nil {
		_, err = temp.Write(data)
	}
	if closed := temp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(name, n.file)
	}
	if err != nil {
		_ = os.Remove(name)
	}
	return err
}

// nightJobView is a job as the list shows it: with its actor, and when it's next due (approved, by the clock).
type nightJobView struct {
	NightJob
	Actor string     `json:"actor"`
	Next  *time.Time `json:"next,omitempty"`
}

// list is every job, the words for what was skipped, and the job running, if any.
func (n *nightShift) list() map[string]any {
	n.mu.Lock()
	jobs := slices.Clone(n.jobs)
	skipped := slices.Clone(n.skipped)
	n.mu.Unlock()
	loc := n.sched.zone()
	views := make([]nightJobView, 0, len(jobs))
	for _, j := range jobs {
		v := nightJobView{NightJob: j, Actor: string(j.actor())}
		if j.Status == nightApproved && j.When.Every != nightTrigger {
			if next, ok := nextNightSlot(j, loc); ok {
				v.Next = &next
			}
		}
		views = append(views, v)
	}
	if skipped == nil {
		skipped = []string{}
	}
	return map[string]any{"jobs": views, "skipped": skipped, "running": n.sched.runningID(), "most": nightJobsMost}
}

// get is the job with id, if there is one.
func (n *nightShift) get(id string) (NightJob, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if i := n.index(id); i >= 0 {
		return n.jobs[i], true
	}
	return NightJob{}, false
}

// index is where the job with id is, -1 when nowhere. With n.mu held.
func (n *nightShift) index(id string) int {
	return slices.IndexFunc(n.jobs, func(j NightJob) bool { return j.ID == id })
}

// newNightID is a fresh job id: "n" and 10 hex digits, none in use.
func (n *nightShift) newNightID() string {
	for {
		id := "n" + strings.ToLower(newID()[:10])
		if n.index(id) < 0 {
			return id
		}
	}
}

// checkedAsk is the ask as a job would keep it, or a status and why not: the words checked, a hire's role known
// to the office and its folder one a hire from setup may start in (links followed).
func (d *Deck) checkedAsk(a nightAsk) (nightAsk, int, string) {
	a.Name, a.Brief, a.Role = strings.TrimSpace(a.Name), strings.TrimSpace(a.Brief), strings.TrimSpace(a.Role)
	a.Folder = strings.TrimSpace(a.Folder)
	if a.Limits.WallMinutes == 0 {
		a.Limits.WallMinutes = nightWallDefault
	}
	if a.Kind == nightHire && a.Folder != "" {
		folder, status, why := checkedSetupFolder(a.Folder, homeDir)
		if status != 0 {
			return a, status, why
		}
		a.Folder = folder
	}
	if why := a.checkedWords(); why != "" {
		return a, http.StatusBadRequest, why
	}
	if a.Kind == nightHire {
		d.mu.Lock()
		known := slices.ContainsFunc(d.config.Roles, func(r Role) bool { return r.ID == a.Role })
		d.mu.Unlock()
		if !known || a.Role == gmRole {
			return a, http.StatusBadRequest, "That role isn't one of the office's (the GM's is a task for the GM)."
		}
	}
	return a, 0, ""
}

// decodedNight is the body read as v, refusing what v has no field for and any key not spelled as v's field is
// (exactKeys). The route and a scoped GM's check (gmscope.go) both read a proposal this way, so they read the same
// bytes the same way (RT126N1 B2).
func decodedNight(body io.Reader, v any) bool {
	data, err := io.ReadAll(io.LimitReader(body, nightBodyMost+1))
	if err != nil || len(data) > nightBodyMost || !exactKeys(json.NewDecoder(bytes.NewReader(data)), reflect.TypeOf(v)) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v) == nil
}

// exactKeys is whether the decoder's next value, read as t, is an object that names each of t's fields as its
// json tag spells it, at most once, and the same for the fields that are structs. encoding/json matches a key to
// a field case-blind and keeps the last of two, so "Kind", or "kind":"gm" then "KIND":"hire", would set a field
// that a check reading the key as spelled never saw. A value t has no fields in is only read past.
func exactKeys(decoder *json.Decoder, t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || reflect.PointerTo(t).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		var skipped json.RawMessage
		return decoder.Decode(&skipped) == nil
	}
	fields := map[string]reflect.Type{}
	jsonFields(t, fields)
	if open, err := decoder.Token(); err != nil || open != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, _ := token.(string)
		field, known := fields[key]
		if err != nil || !known || seen[key] || !exactKeys(decoder, field) {
			return false
		}
		seen[key] = true
	}
	end, err := decoder.Token()
	return err == nil && end == json.Delim('}')
}

// jsonFields adds t's fields to fields by the key encoding/json writes them under, an embedded struct's as its own.
func jsonFields(t reflect.Type, fields map[string]reflect.Type) {
	for _, f := range reflect.VisibleFields(t) {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch {
		case !f.IsExported() || name == "-":
		case f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct:
			// reflect.VisibleFields lists the embedded struct's fields too.
		case name == "":
			fields[f.Name] = f.Type
		default:
			fields[name] = f.Type
		}
	}
}

// nightRoute serves the night shift's routes, and whether path was one. The gate service has let in only the
// callers goOnlyRoutes names (and refused an MCP key and a night job): the routes check it again.
func (d *Deck) nightRoute(w http.ResponseWriter, r *request) bool {
	path := r.URL.Path
	if !isNightRoute(path) {
		return false
	}
	n := d.night
	page := r.caller == callerPage
	pageOnly := func() bool {
		if !page {
			writeStatus(w)(http.StatusForbidden, "Only the setup page can do this to a night job.")
		}
		return page
	}
	id := r.URL.Query().Get("id")
	switch {
	case r.Method == http.MethodGet && path == nightListPath:
		if !page && r.caller != callerGM && r.caller != callerTool {
			w.WriteHeader(http.StatusForbidden)
			return true
		}
		writeJSON(w, http.StatusOK, n.list())
	case r.Method == http.MethodPost && path == nightProposePath:
		if !page && r.caller != callerGM {
			writeStatus(w)(http.StatusForbidden, "Only the setup page or the GM can propose a night job.")
			return true
		}
		var ask nightAsk
		if !decodedNight(r.Body, &ask) {
			writeStatus(w)(http.StatusBadRequest, "Send the job as JSON: name, kind, role, brief, folder, when and limits, nothing else.")
			return true
		}
		writeAnswer(w)(d.proposeNight(ask, r.caller.name()))
	case r.Method == http.MethodPost && path == nightSavePath:
		if !pageOnly() {
			return true
		}
		var save nightSave
		if !decodedNight(r.Body, &save) {
			writeStatus(w)(http.StatusBadRequest, "Send the job as JSON: id, name, kind, role, brief, folder, when, limits and status.")
			return true
		}
		writeAnswer(w)(d.saveNight(save))
	case r.Method == http.MethodPost && path == nightApprovePath:
		if pageOnly() {
			writeAnswer(w)(n.setStatus(id, nightApproved))
		}
	case r.Method == http.MethodPost && path == nightPausePath:
		if pageOnly() {
			writeAnswer(w)(n.setStatus(id, nightPaused))
		}
	case r.Method == http.MethodPost && path == nightDeletePath:
		if pageOnly() {
			writeAnswer(w)(n.remove(id))
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
	return true
}

// proposeNight keeps a proposal: a job that waits for the owner's approval and never runs before it.
func (d *Deck) proposeNight(ask nightAsk, by string) (int, any) {
	ask, status, why := d.checkedAsk(ask)
	if status != 0 {
		return status, why
	}
	n := d.night
	n.mu.Lock()
	defer n.mu.Unlock()
	waiting := 0
	for _, j := range n.jobs {
		if j.Status == nightProposed {
			waiting++
		}
	}
	switch {
	case len(n.jobs) >= nightJobsMost:
		return http.StatusConflict, fmt.Sprintf("The night shift keeps %d jobs at most: delete one first.", nightJobsMost)
	case waiting >= nightProposeMost:
		return http.StatusConflict, fmt.Sprintf("%d proposals already wait for the owner.", nightProposeMost)
	}
	j := ask.job(n.newNightID(), nightProposed, by, n.sched.now())
	return n.keepLocked(append(slices.Clone(n.jobs), j), j)
}

// saveNight is the page's create or edit. A create is approved unless it says paused. An edit keeps the job's
// status, and a change of when looks for the next run from now, so the change never makes a run missed.
func (d *Deck) saveNight(save nightSave) (int, any) {
	ask, status, why := d.checkedAsk(save.nightAsk)
	if status != 0 {
		return status, why
	}
	n := d.night
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.sched.now()
	if save.ID == "" {
		switch save.Status {
		case "":
			save.Status = nightApproved
		case nightApproved, nightPaused:
		default:
			return http.StatusBadRequest, "A new job is approved or paused."
		}
		if len(n.jobs) >= nightJobsMost {
			return http.StatusConflict, fmt.Sprintf("The night shift keeps %d jobs at most: delete one first.", nightJobsMost)
		}
		j := ask.job(n.newNightID(), save.Status, "page", now)
		return n.keepLocked(append(slices.Clone(n.jobs), j), j)
	}
	i := n.index(save.ID)
	if i < 0 {
		return http.StatusNotFound, "There's no such night job."
	}
	if save.Status != "" && save.Status != n.jobs[i].Status {
		return http.StatusBadRequest, "An edit keeps the job's status: approve or pause it on its own."
	}
	old := n.jobs[i]
	j := ask.job(old.ID, old.Status, old.ProposedBy, old.Created)
	j.Since, j.LastSlot = old.Since, old.LastSlot
	if j.When != old.When {
		j.Since = now
	}
	jobs := slices.Clone(n.jobs)
	jobs[i] = j
	return n.keepLocked(jobs, j)
}

// job is the ask as a new job, looked for from at.
func (a nightAsk) job(id, status, by string, at time.Time) NightJob {
	return NightJob{ID: id, Name: a.Name, Kind: a.Kind, Role: a.Role, Brief: a.Brief, Folder: a.Folder, When: a.When,
		Limits: a.Limits, Status: status, ProposedBy: by, Created: at, Since: at}
}

// setStatus approves or pauses a job. An approval looks for the next run from now: what fell due while the job
// waited or was paused was never its run.
func (n *nightShift) setStatus(id, status string) (int, any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	i := n.index(id)
	if i < 0 {
		return http.StatusNotFound, "There's no such night job."
	}
	jobs := slices.Clone(n.jobs)
	if jobs[i].Status != status {
		jobs[i].Status = status
		if status == nightApproved {
			jobs[i].Since = n.sched.now()
		}
	}
	return n.keepLocked(jobs, jobs[i])
}

// remove deletes a job, and its run waiting in line, if any.
func (n *nightShift) remove(id string) (int, any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	i := n.index(id)
	if i < 0 {
		return http.StatusNotFound, "There's no such night job."
	}
	if status, answer := n.keepLocked(slices.Delete(slices.Clone(n.jobs), i, i+1), nil); status != http.StatusOK {
		return status, answer
	}
	return http.StatusOK, map[string]any{"deleted": id}
}

// keepLocked writes jobs and takes them as the store's, answering with j; with n.mu held. Nothing changes if the
// write fails.
func (n *nightShift) keepLocked(jobs []NightJob, j any) (int, any) {
	before := n.jobs
	n.jobs = jobs
	if err := n.saveLocked(); err != nil {
		n.jobs = before
		return http.StatusInternalServerError, "The night shift's jobs couldn't be saved."
	}
	return http.StatusOK, j
}
