package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The night shift's jobs and scheduler (nightshift.go, nightschedule.go): the page makes, approves, pauses and
// deletes; the GM lists and proposes, and a proposal never falls due; an MCP key, a night job, a hire and a
// phone can do none of it. The scheduler runs on a fake clock, in time zones of its own.

func nightZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skip("no zone data for", name)
	}
	return loc
}

// nightDeck is a mail deck (a GM's token, hires' keys) with a project folder in the home folder, and its night
// shift on a fake clock at Monday 9 Nov 2026, 10:00 in Paris.
func nightDeck(t *testing.T) (m *mailDeck, project string, clock *time.Time) {
	t.Helper()
	m = newMailDeck(t)
	project = filepath.Join(homeDir, "proj")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	paris := nightZone(t, "Europe/Paris")
	at := time.Date(2026, 11, 9, 10, 0, 0, 0, paris)
	clock = &at
	m.d.night.sched.now = func() time.Time { return *clock }
	m.d.night.sched.zone = func() *time.Location { return paris }
	return m, project, clock
}

func nightBody(project, extra string) string {
	return `{"name":"Nightly tests","kind":"hire","role":"tester","brief":"Run the tests, fix what fails.","folder":"` + project +
		`","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":90,"model":"sonnet","effort":"medium"}` + extra + `}`
}

func nightJobs(t *testing.T, m *mailDeck, as who) []map[string]any {
	t.Helper()
	w := m.call(t, as, "GET", nightListPath, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	var out struct{ Jobs []map[string]any }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Jobs
}

func TestThePageMakesApprovesPausesAndDeletesANightJob(t *testing.T) {
	m, project, _ := nightDeck(t)
	made := m.call(t, asPage, "POST", nightSavePath, nightBody(project, ""))
	job := decoded(t, made)
	id, _ := job["id"].(string)
	if made.Code != http.StatusOK || job["status"] != nightApproved || job["proposedBy"] != "page" || job["folder"] != physical(project) || id == "" {
		t.Fatalf("made: %d %s", made.Code, made.Body)
	}
	file := filepath.Join(supportDir, nightFileName)
	if info, err := os.Stat(file); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("nightshift.json: %v %v", info, err)
	}
	listed := nightJobs(t, m, asPage)
	if len(listed) != 1 || listed[0]["actor"] != "night:"+id || listed[0]["next"] == nil {
		t.Fatalf("listed: %v", listed)
	}
	for _, step := range []struct{ path, status string }{{nightPausePath, nightPaused}, {nightApprovePath, nightApproved}} {
		if w := m.call(t, asPage, "POST", step.path+"?id="+id, ""); w.Code != http.StatusOK || decoded(t, w)["status"] != step.status {
			t.Fatalf("%s: %d %s", step.path, w.Code, w.Body)
		}
	}
	// An edit keeps the status; it can't change it on the way.
	edit := strings.Replace(nightBody(project, `,"id":"`+id+`"`), "Nightly tests", "Night tests", 1)
	if w := m.call(t, asPage, "POST", nightSavePath, edit); w.Code != http.StatusOK || decoded(t, w)["name"] != "Night tests" || decoded(t, w)["status"] != nightApproved {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	if w := m.call(t, asPage, "POST", nightSavePath, nightBody(project, `,"id":"`+id+`","status":"paused"`)); w.Code != http.StatusBadRequest {
		t.Fatalf("an edit changing the status: %d %s", w.Code, w.Body)
	}
	// What can't be a job is refused in a sentence: no such role, a folder outside the home folder, no time.
	for _, bad := range []string{
		strings.Replace(nightBody(project, ""), `"tester"`, `"nobody"`, 1),
		strings.Replace(nightBody(project, ""), `"tester"`, `"gm"`, 1),
		strings.Replace(nightBody(project, ""), project, t.TempDir(), 1),
		strings.Replace(nightBody(project, ""), `"02:00"`, `"25:00"`, 1),
		strings.Replace(nightBody(project, ""), `"daily"`, `"weekly"`, 1),
		strings.Replace(nightBody(project, ""), `"wallMinutes":90`, `"wallMinutes":9999`, 1),
		strings.Replace(nightBody(project, ""), `"medium"`, `"huge"`, 1),
		nightBody(project, `,"since":"2020-01-01T00:00:00Z"`),
	} {
		if w := m.call(t, asPage, "POST", nightSavePath, bad); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", bad, w.Code, w.Body)
		}
	}
	if w := m.call(t, asPage, "POST", nightDeletePath+"?id="+id, ""); w.Code != http.StatusOK || len(nightJobs(t, m, asPage)) != 0 {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
	if w := m.call(t, asPage, "POST", nightApprovePath+"?id="+id, ""); w.Code != http.StatusNotFound {
		t.Fatalf("approving a deleted job: %d", w.Code)
	}
	// What was written reads back, and a task for the GM names no folder.
	gmTask := `{"name":"Weekly cleanup","kind":"gm","brief":"Tidy the branches.","when":{"every":"weekly","day":"sunday","at":"03:30"},"limits":{"wallMinutes":30}}`
	if w := m.call(t, asPage, "POST", nightSavePath, gmTask); w.Code != http.StatusOK {
		t.Fatalf("a task for the GM: %d %s", w.Code, w.Body)
	}
	jobs, skipped := readNightJobs(file)
	if len(jobs) != 1 || len(skipped) != 0 || jobs[0].Kind != nightGM || jobs[0].When.Day != "sunday" {
		t.Fatalf("read back: %+v %v", jobs, skipped)
	}
}

func TestTheGMProposesAndListsButNeverApprovesEditsOrDeletes(t *testing.T) {
	m, project, _ := nightDeck(t)
	// A proposal can't name its status: anything but the job's words is refused.
	if w := m.call(t, asGM, "POST", nightProposePath, nightBody(project, `,"status":"approved"`)); w.Code != http.StatusBadRequest {
		t.Fatalf("a proposal naming its status: %d %s", w.Code, w.Body)
	}
	w := m.call(t, asGM, "POST", nightProposePath, nightBody(project, ""))
	job := decoded(t, w)
	id, _ := job["id"].(string)
	if w.Code != http.StatusOK || job["status"] != nightProposed || job["proposedBy"] != "gm" {
		t.Fatalf("proposed: %d %s", w.Code, w.Body)
	}
	if jobs := nightJobs(t, m, asGM); len(jobs) != 1 || jobs[0]["status"] != nightProposed || jobs[0]["next"] != nil {
		t.Fatalf("the GM's list: %v", jobs)
	}
	for _, path := range []string{nightApprovePath, nightPausePath, nightDeletePath} {
		if w := m.call(t, asGM, "POST", path+"?id="+id, ""); w.Code != http.StatusForbidden {
			t.Errorf("the GM at %s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, body := range []string{nightBody(project, `,"id":"`+id+`"`), nightBody(project, "")} {
		if w := m.call(t, asGM, "POST", nightSavePath, body); w.Code != http.StatusForbidden {
			t.Errorf("the GM's save: %d %s", w.Code, w.Body)
		}
	}
	if jobs := nightJobs(t, m, asPage); len(jobs) != 1 || jobs[0]["status"] != nightProposed || jobs[0]["name"] != "Nightly tests" {
		t.Fatalf("after the GM's tries: %v", jobs)
	}
	// Proposals waiting are capped, so the GM can't fill the store.
	for i := 1; i < nightProposeMost; i++ {
		if w := m.call(t, asGM, "POST", nightProposePath, nightBody(project, "")); w.Code != http.StatusOK {
			t.Fatalf("proposal %d: %d %s", i, w.Code, w.Body)
		}
	}
	if w := m.call(t, asGM, "POST", nightProposePath, nightBody(project, "")); w.Code != http.StatusConflict {
		t.Fatalf("one proposal too many: %d %s", w.Code, w.Body)
	}
	// The page approves what the GM proposed.
	if w := m.call(t, asPage, "POST", nightApprovePath+"?id="+id, ""); w.Code != http.StatusOK || decoded(t, w)["status"] != nightApproved {
		t.Fatalf("the page's approval: %d %s", w.Code, w.Body)
	}
}

func TestAnMCPKeyAHireAndAPhoneCantTouchTheNightShift(t *testing.T) {
	m, project, _ := nightDeck(t)
	made := decoded(t, m.call(t, asPage, "POST", nightSavePath, nightBody(project, `,"status":"paused"`)))
	id, _ := made["id"].(string)
	routes := []struct{ method, path, body string }{
		{"GET", nightListPath, ""}, {"POST", nightProposePath, nightBody(project, "")}, {"POST", nightSavePath, nightBody(project, "")},
		{"POST", nightApprovePath + "?id=" + id, ""}, {"POST", nightPausePath + "?id=" + id, ""}, {"POST", nightDeletePath + "?id=" + id, ""},
	}
	phone := pair(t, m.d)
	for _, r := range routes {
		path, _, _ := strings.Cut(r.path, "?")
		for _, a := range []actor{mcpActor("claude-desktop"), nightActor(id), actorHire} {
			if v := m.d.decide(a, action{route: &routeAsk{method: r.method, path: path, body: strings.NewReader(r.body)}}); v.allowed() || v.status != http.StatusForbidden {
				t.Errorf("%s %s by %s: %+v", r.method, path, a, v)
			}
		}
		// A hire with its key, through the server.
		if w := m.call(t, asRev, r.method, r.path, r.body); w.Code != http.StatusForbidden {
			t.Errorf("%s %s by a hire: %d", r.method, r.path, w.Code)
		}
		// A phone: setup isn't served off this computer, and its key isn't a setup caller here either.
		if w := serve(t, m.d, call{method: r.method, target: r.path, host: deckOnWiFi, from: aPhone, body: r.body, withoutSetupKey: true,
			headers: map[string]string{setupHeader: "1", "Authorization": "Bearer " + phone}}); w.Code != http.StatusNotFound {
			t.Errorf("%s %s from a phone: %d", r.method, r.path, w.Code)
		}
		if w := serve(t, m.d, call{method: r.method, target: r.path + "?t=" + phone, host: "localhost:7710", from: thisComputer, body: r.body,
			withoutSetupKey: true, headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with a phone's key: %d", r.method, r.path, w.Code)
		}
		// A tool with the setup key lists, and nothing more.
		if w := m.call(t, asTool, r.method, r.path, r.body); (w.Code == http.StatusOK) != (r.method == "GET") {
			t.Errorf("%s %s by a tool: %d", r.method, r.path, w.Code)
		}
	}
	if jobs := nightJobs(t, m, asPage); len(jobs) != 1 || jobs[0]["status"] != nightPaused {
		t.Fatalf("after the refusals: %v", jobs)
	}
}

// nightRig is a store in a temp folder with a scheduler on a fake clock in loc, telling what falls due.
type nightRig struct {
	n     *nightShift
	clock time.Time
	loc   *time.Location
	told  []nightDue
	done  []func()
}

func newNightRig(t *testing.T, loc *time.Location, at time.Time) *nightRig {
	t.Helper()
	r := &nightRig{loc: loc, clock: at}
	r.n = newNightShift(filepath.Join(t.TempDir(), nightFileName), nil)
	r.n.sched.now = func() time.Time { return r.clock }
	r.n.sched.zone = func() *time.Location { return r.loc }
	r.n.sched.due = func(due nightDue, done func()) { r.told = append(r.told, due); r.done = append(r.done, done) }
	return r
}

// add keeps a job made at the clock's time.
func (r *nightRig) add(t *testing.T, id, status string, when NightWhen) {
	t.Helper()
	j := NightJob{ID: id, Name: id, Kind: nightGM, Brief: "b", When: when, Limits: NightLimits{WallMinutes: 30}, Status: status,
		ProposedBy: "page", Created: r.clock, Since: r.clock}
	if why := j.checkedShape(); why != "" {
		t.Fatal(why)
	}
	r.n.mu.Lock()
	r.n.jobs = append(r.n.jobs, j)
	r.n.mu.Unlock()
}

// at moves the clock and looks: what was told by this look.
func (r *nightRig) at(when time.Time) []nightDue {
	before := len(r.told)
	r.clock = when
	r.n.sched.tick()
	return r.told[before:]
}

func TestAProposedJobNeverFalls(t *testing.T) {
	paris := nightZone(t, "Europe/Paris")
	r := newNightRig(t, paris, time.Date(2026, 11, 9, 10, 0, 0, 0, paris))
	r.add(t, "proposal", nightProposed, NightWhen{Every: nightDaily, At: "02:00"})
	r.add(t, "paused", nightPaused, NightWhen{Every: nightDaily, At: "02:00"})
	for day := 10; day < 20; day++ {
		if told := r.at(time.Date(2026, 11, day, 2, 1, 0, 0, paris)); len(told) != 0 {
			t.Fatalf("day %d: %+v", day, told)
		}
	}
	if j, _ := r.n.get("proposal"); j.LastSlot != "" {
		t.Fatalf("a proposal was marked run: %+v", j)
	}
	// Approved, it runs from its approval on: the nights it waited were never its runs.
	m, project, clock := nightDeck(t)
	id := decoded(t, m.call(t, asGM, "POST", nightProposePath, nightBody(project, "")))["id"].(string)
	var told []nightDue
	m.d.night.sched.due = func(due nightDue, done func()) { told = append(told, due); done() }
	*clock = clock.Add(72 * time.Hour)
	m.d.night.sched.tick()
	if len(told) != 0 {
		t.Fatalf("a proposal fell due: %+v", told)
	}
	m.call(t, asPage, "POST", nightApprovePath+"?id="+id, "")
	m.d.night.sched.tick()
	*clock = time.Date(2026, 11, 13, 2, 0, 30, 0, clock.Location())
	m.d.night.sched.tick()
	if len(told) != 1 || told[0].Missed || told[0].Actor != nightActor(id) || told[0].Job.Status != nightApproved {
		t.Fatalf("approved: %+v", told)
	}
}

func TestNextDueAcrossDaylightSavingAndTimeZones(t *testing.T) {
	paris, newYork := nightZone(t, "Europe/Paris"), nightZone(t, "America/New_York")
	la, tokyo := nightZone(t, "America/Los_Angeles"), nightZone(t, "Asia/Tokyo")
	next := func(at string, every, day string, since time.Time, lastSlot string, loc *time.Location) time.Time {
		t.Helper()
		slot, ok := nextNightSlot(NightJob{When: NightWhen{Every: every, Day: day, At: at}, Since: since, LastSlot: lastSlot}, loc)
		if !ok {
			t.Fatalf("no slot for %s after %v", at, since)
		}
		return slot
	}
	for _, c := range []struct {
		name  string
		got   time.Time
		want  time.Time
		where *time.Location
	}{
		// A plain night, and the same night once its slot has gone.
		{"plain", next("02:00", nightDaily, "", time.Date(2026, 11, 9, 10, 0, 0, 0, paris), "", paris), time.Date(2026, 11, 10, 2, 0, 0, 0, paris), paris},
		// Spring forward: 02:30 doesn't exist; it runs when the clocks jump, at 03:00 summer time.
		{"Paris spring", next("02:30", nightDaily, "", time.Date(2026, 3, 28, 12, 0, 0, 0, paris), "", paris), time.Date(2026, 3, 29, 1, 0, 0, 0, time.UTC), paris},
		{"New York spring", next("02:30", nightDaily, "", time.Date(2026, 3, 7, 12, 0, 0, 0, newYork), "", newYork), time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC), newYork},
		// Fall back: 02:30 comes twice in Paris, 01:30 in New York; the first is the slot.
		{"Paris autumn", next("02:30", nightDaily, "", time.Date(2026, 10, 24, 12, 0, 0, 0, paris), "", paris), time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC), paris},
		{"New York autumn", next("01:30", nightDaily, "", time.Date(2026, 10, 31, 12, 0, 0, 0, newYork), "", newYork), time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC), newYork},
		// And after the first, not the second: the date ran.
		{"Paris autumn, once", next("02:30", nightDaily, "", time.Date(2026, 10, 25, 0, 31, 0, 0, time.UTC), "2026-10-25", paris), time.Date(2026, 10, 26, 2, 30, 0, 0, paris), paris},
		// Weekly, across the change: Sunday 29 March at 09:00 is summer time.
		{"weekly", next("09:00", nightWeekly, "sunday", time.Date(2026, 3, 23, 12, 0, 0, 0, paris), "", paris), time.Date(2026, 3, 29, 7, 0, 0, 0, time.UTC), paris},
		// Moved west after New York's 02:00 ran: Los Angeles' 02:00 the same date doesn't run it again.
		{"west", next("02:00", nightDaily, "", time.Date(2026, 11, 13, 7, 0, 30, 0, time.UTC), "2026-11-13", la), time.Date(2026, 11, 14, 2, 0, 0, 0, la), la},
		// Moved east after Paris's 02:00 on the 12th: Tokyo's next 02:00 is the 13th's.
		{"east", next("02:00", nightDaily, "", time.Date(2026, 11, 12, 1, 0, 30, 0, time.UTC), "2026-11-12", tokyo), time.Date(2026, 11, 13, 2, 0, 0, 0, tokyo), tokyo},
	} {
		if !c.got.Equal(c.want) {
			t.Errorf("%s: %v, want %v", c.name, c.got.In(c.where), c.want.In(c.where))
		}
	}
	// Through the scheduler: a zone changed under it is read at the next look, and each date runs once.
	r := newNightRig(t, newYork, time.Date(2026, 11, 12, 12, 0, 0, 0, newYork))
	r.add(t, "nightly", nightApproved, NightWhen{Every: nightDaily, At: "02:00"})
	if told := r.at(time.Date(2026, 11, 13, 2, 0, 20, 0, newYork)); len(told) != 1 {
		t.Fatalf("New York's 02:00: %+v", told)
	}
	r.done[0]()
	r.loc = la
	for _, at := range []time.Time{time.Date(2026, 11, 13, 2, 0, 20, 0, la), time.Date(2026, 11, 13, 23, 0, 0, 0, la)} {
		if told := r.at(at); len(told) != 0 {
			t.Fatalf("Los Angeles, %v: %+v", at, told)
		}
	}
	if told := r.at(time.Date(2026, 11, 14, 2, 0, 20, 0, la)); len(told) != 1 || told[0].Missed {
		t.Fatalf("Los Angeles' next night: %+v", told)
	}
}

func TestARunMissedWhileAsleepRunsOnceAtWake(t *testing.T) {
	paris := nightZone(t, "Europe/Paris")
	r := newNightRig(t, paris, time.Date(2026, 11, 9, 1, 0, 0, 0, paris)) // Monday, 01:00
	r.add(t, "nightly", nightApproved, NightWhen{Every: nightDaily, At: "02:00"})
	if told := r.at(time.Date(2026, 11, 9, 1, 30, 0, 0, paris)); len(told) != 0 {
		t.Fatalf("before its time: %+v", told)
	}
	// Asleep from 01:30 on Monday to 09:00 on Wednesday: two nights missed, one run, with a notice.
	told := r.at(time.Date(2026, 11, 11, 9, 0, 0, 0, paris))
	if len(told) != 1 || !told[0].Missed || !told[0].Slot.Equal(time.Date(2026, 11, 9, 2, 0, 0, 0, paris)) ||
		!strings.Contains(told[0].Notice, "nightly was due Mon 02:00") || !strings.Contains(told[0].Notice, "runs once now") {
		t.Fatalf("at wake: %+v", told)
	}
	r.done[0]()
	for _, at := range []time.Time{time.Date(2026, 11, 11, 9, 0, 30, 0, paris), time.Date(2026, 11, 11, 23, 0, 0, 0, paris)} {
		if told := r.at(at); len(told) != 0 {
			t.Fatalf("again at %v: %+v", at, told)
		}
	}
	// The next night is an ordinary one.
	if told := r.at(time.Date(2026, 11, 12, 2, 0, 10, 0, paris)); len(told) != 1 || told[0].Missed || told[0].Notice != "" {
		t.Fatalf("the next night: %+v", told)
	}
	// A restart reads where it was: Smolcorp closed from Thursday to Saturday is one run, at the next look.
	r.done[1]()
	reread := newNightShift(r.n.file, nil)
	reread.sched.now, reread.sched.zone = r.n.sched.now, r.n.sched.zone
	var again []nightDue
	reread.sched.due = func(due nightDue, done func()) { again = append(again, due); done() }
	r.clock = time.Date(2026, 11, 14, 11, 0, 0, 0, paris)
	reread.sched.tick()
	reread.sched.tick()
	if len(again) != 1 || !again[0].Missed || !again[0].Slot.Equal(time.Date(2026, 11, 13, 2, 0, 0, 0, paris)) {
		t.Fatalf("after a restart: %+v", again)
	}
}

func TestNightJobsRunOneAtATime(t *testing.T) {
	paris := nightZone(t, "Europe/Paris")
	r := newNightRig(t, paris, time.Date(2026, 11, 9, 10, 0, 0, 0, paris))
	r.add(t, "first", nightApproved, NightWhen{Every: nightDaily, At: "02:00"})
	r.add(t, "second", nightApproved, NightWhen{Every: nightDaily, At: "02:00"})
	r.add(t, "third", nightApproved, NightWhen{Every: nightDaily, At: "02:00"})
	if told := r.at(time.Date(2026, 11, 10, 2, 0, 10, 0, paris)); len(told) != 1 || told[0].Job.ID != "first" || r.n.sched.runningID() != "first" {
		t.Fatalf("at 02:00: %+v", told)
	}
	// The third is paused while it waits: it doesn't run.
	r.n.setStatus("third", nightPaused)
	if told := r.at(time.Date(2026, 11, 10, 2, 0, 40, 0, paris)); len(told) != 0 {
		t.Fatalf("while the first runs: %+v", told)
	}
	r.done[0]()
	r.done[0]() // done twice is once
	if len(r.told) != 2 || r.told[1].Job.ID != "second" || r.n.sched.runningID() != "second" {
		t.Fatalf("after the first: %+v", r.told)
	}
	// Still running at the next night: that night's slot is its run, not a second one in line.
	if told := r.at(time.Date(2026, 11, 11, 2, 0, 10, 0, paris)); len(told) != 0 {
		t.Fatalf("the next night, the second still running: %+v", told)
	}
	r.done[1]()
	if len(r.told) != 3 || r.told[2].Job.ID != "first" {
		t.Fatalf("after the second: %+v", r.told)
	}
	r.done[2]()
	if r.n.sched.runningID() != "" || len(r.told) != 3 {
		t.Fatalf("at the end: %q %+v", r.n.sched.runningID(), r.told)
	}
}

func TestAMalformedNightStoreIsSkippedWithAWord(t *testing.T) {
	folder := t.TempDir()
	file := filepath.Join(folder, nightFileName)
	good := `{"id":"nok","name":"Fine","kind":"gm","brief":"b","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":30},"status":"approved","proposedBy":"page","created":"2026-11-09T10:00:00Z"}`
	// Each broken one has an id of its own, so only what's wrong with it is why it's skipped.
	as := func(id, old, new string) string {
		return strings.Replace(strings.Replace(good, `"id":"nok"`, `"id":"`+id+`"`, 1), old, new, 1)
	}
	broken := []string{
		`"not a job"`,
		as("n1", `"approved"`, `"running"`),
		as("n2", `"proposedBy":"page"`, `"proposedBy":"mcp"`),
		as("n3", `"02:00"`, `"2am"`),
		as("../x", "", ""),
		as("n4", `"brief":"b"`, `"brief":"b","extra":1`),
		as("n5", `"kind":"gm"`, `"kind":"gm","folder":"/x"`),
		as("n6", `"status"`, `"lastSlot":"Tuesday","status"`),
		good, // the same id again
	}
	data := `{"jobs":[` + good + `,` + strings.Join(broken, ",") + `]}`
	if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	n := newNightShift(file, nil)
	if len(n.jobs) != 1 || n.jobs[0].ID != "nok" || len(n.skipped) != len(broken) || !strings.HasPrefix(n.skipped[0], "Night job 2 in nightshift.json was skipped: ") {
		t.Fatalf("read: %+v %q", n.jobs, n.skipped)
	}
	if list := n.list(); len(list["skipped"].([]string)) != len(broken) {
		t.Fatalf("the list doesn't say what was skipped: %v", list)
	}
	// The first write sets the file as read aside, once, and writes what was kept, readable by you alone.
	if status, _ := n.setStatus("nok", nightPaused); status != http.StatusOK {
		t.Fatal(status)
	}
	if kept, err := os.ReadFile(file + ".bad"); err != nil || string(kept) != data {
		t.Fatalf("set aside: %v", err)
	}
	if jobs, skipped := readNightJobs(file); len(jobs) != 1 || len(skipped) != 0 || jobs[0].Status != nightPaused {
		t.Fatalf("written: %+v %v", jobs, skipped)
	}
	if info, err := os.Stat(file); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode: %v %v", info, err)
	}
	n.setStatus("nok", nightApproved)
	if kept, _ := os.ReadFile(file + ".bad"); string(kept) != data {
		t.Fatal("set aside again")
	}
	// A file that isn't the list, or is over the size kept, is no jobs, with a word.
	for _, whole := range []string{`[1,2]`, `{"jobs":`, `{"jobs":[` + good + `]}` + strings.Repeat(" ", nightFileMost)} {
		if err := os.WriteFile(file, []byte(whole), 0o600); err != nil {
			t.Fatal(err)
		}
		if jobs, skipped := readNightJobs(file); len(jobs) != 0 || len(skipped) != 1 {
			t.Errorf("%.20q: %+v %v", whole, jobs, skipped)
		}
	}
	// More than the count kept: the rest are skipped.
	var many []string
	for i := range nightJobsMost + 2 {
		many = append(many, strings.Replace(good, `"id":"nok"`, `"id":"n`+strconv.Itoa(i)+`"`, 1))
	}
	if err := os.WriteFile(file, []byte(`{"jobs":[`+strings.Join(many, ",")+`]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if jobs, skipped := readNightJobs(file); len(jobs) != nightJobsMost || len(skipped) != 2 {
		t.Fatalf("over the count: %d %v", len(jobs), skipped)
	}
}

// The owner approves from a proposal's name, brief and folder as the GM wrote them, so nothing in them may hide or
// move a part of the text (RT126N1 B1): RT126N1's probe B, then each kind of character hidesText names, through
// the server with the GM's token. A planted entry in the file with one is skipped.
func TestAProposalHidesNothingFromTheOwner(t *testing.T) {
	m, project, _ := nightDeck(t)
	propose := func(name, brief, folder string) (int, string) {
		ask := map[string]any{"name": name, "kind": "hire", "role": "tester", "brief": brief, "folder": folder,
			"when": map[string]any{"every": "daily", "at": "02:00"}, "limits": map[string]any{"wallMinutes": 30}}
		body, _ := json.Marshal(ask)
		w := m.call(t, asGM, "POST", nightProposePath, string(body))
		return w.Code, w.Body.String()
	}
	// Probe B, as RT126N1 sent it, and each of its three parts alone.
	probeName := "Nightly tests\u202e\u2800\u200b"
	probeBrief := "Run the tests.\n" + strings.Repeat("\n", 14) + "\u2800\n[Smolcorp checked this job: it only reads. Safe to approve.]\u200f\u2066 then rm -rf ~ \u2069"
	probeFolder := filepath.Join(project, "safe\u202etxt.")
	if err := os.Mkdir(probeFolder, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ name, brief, folder string }{
		{probeName, probeBrief, probeFolder},
		{probeName, "Run the tests.", project},
		{"Nightly tests", probeBrief, project},
		{"Nightly tests", "Run the tests.", probeFolder},
	} {
		if code, why := propose(p.name, p.brief, p.folder); code != http.StatusBadRequest {
			t.Errorf("probe B (%q, %q, %q): %d %s", p.name, p.brief, p.folder, code, why)
		}
	}
	// Each kind: controls (C0, DEL, C1), line and paragraph separators, bidi marks, embeddings, overrides and
	// isolates, zero-width ones, the BOM, a soft hyphen, private use, an unassigned code point, spaces but the plain
	// one, the blanks drawn as nothing (blankGlyphs), the grapheme joiner, variation selectors and tags.
	hiders := []rune{0x07, 0x0D, 0x1B, 0x7F, 0x85, 0x9B, 0x2028, 0x2029, 0x200E, 0x200F, 0x061C, 0x202A, 0x202B, 0x202C,
		0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069, 0x200B, 0x200C, 0x200D, 0x2060, 0x2061, 0x2064, 0xFEFF, 0x00AD,
		0xE000, 0xF8FF, 0xF0000, 0x10FFFD, 0x0378, 0xFFFF, 0x00A0, 0x2002, 0x2007, 0x200A, 0x202F, 0x205F, 0x3000,
		0x115F, 0x1160, 0x2800, 0x3164, 0xFFA0, 0x16FE4, 0x1D159, 0x034F, 0xFE00, 0xFE0F, 0xE0100, 0xE0001, 0xE0041,
		0xE007F, 0x180E}
	for _, c := range hiders {
		hidden := "Nightly" + string(c) + "tests"
		if code, why := propose(hidden, "Run the tests.", project); code != http.StatusBadRequest || !strings.Contains(why, "name") {
			t.Errorf("U+%04X in the name: %d %s", c, code, why)
		}
		if c != '\t' && c != '\n' {
			if code, why := propose("Nightly tests", "Run"+string(c)+"the tests.", project); code != http.StatusBadRequest || !strings.Contains(why, "brief") {
				t.Errorf("U+%04X in the brief: %d %s", c, code, why)
			}
		}
	}
	// Blank lines that push the rest out of sight, with or without spaces and tabs on them.
	for _, brief := range []string{"Run the tests.\n\n\nThen stop.", "Run the tests.\n \n\t\nThen stop.", "a\n\n\n\n\n\n\n\n\nb"} {
		if code, why := propose("Nightly tests", brief, project); code != http.StatusBadRequest || !strings.Contains(why, "blank line") {
			t.Errorf("%q: %d %s", brief, code, why)
		}
	}
	// What the owner sees whole goes on: lines, tabs, one blank line between paragraphs, accents, other scripts and
	// an emoji (a joined one has a ZWJ, refused above, so it's a single one here).
	code, why := propose("Tests de nuit, café ☕", "Lance les tests.\n\n\t- corrige ce qui échoue\n\t- テストを直す", project)
	if code != http.StatusOK {
		t.Fatalf("a plain proposal: %d %s", code, why)
	}
	// A planted entry with a hidden character is skipped with a word that quotes nothing of it.
	file := filepath.Join(t.TempDir(), nightFileName)
	entry := func(id, name string) string {
		return `{"id":"` + id + `","name":"` + name + `","kind":"gm","brief":"b","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":30},"status":"approved","proposedBy":"page","created":"2026-11-09T10:00:00Z"}`
	}
	data := `{"jobs":[` + entry("nok", "Fine") + `,` + entry("nbad", `Fine\u202e\u2800`) + `]}`
	if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if jobs, skipped := readNightJobs(file); len(jobs) != 1 || jobs[0].ID != "nok" || len(skipped) != 1 || strings.ContainsRune(skipped[0], 0x202E) {
		t.Fatalf("planted: %+v %q", jobs, skipped)
	}
}

// A proposal's or a save's keys are spelled as the page spells them, once each, or the body is refused: encoding/json
// alone would read "Kind" as kind, and the last of two (RT126N1 B2).
func TestANightBodySpellsEachKeyOnce(t *testing.T) {
	m, project, _ := nightDeck(t)
	for _, as := range []who{asGM, asPage} {
		path := nightProposePath
		if as == asPage {
			path = nightSavePath
		}
		for _, odd := range []struct{ old, new string }{
			{`"kind":"hire"`, `"Kind":"hire"`},
			{`"kind":"hire"`, `"kind":"gm","KIND":"hire"`},
			{`"kind":"hire"`, `"kind":"hire","kind":"hire"`},
			{`"name"`, `"NAME"`},
			{`"folder"`, `"Folder"`},
			{`"every"`, `"Every"`},
			{`"wallMinutes"`, `"wallminutes"`},
			{`"wallMinutes":90`, `"wallMinutes":90,"wallMinutes":30`},
		} {
			body := strings.Replace(nightBody(project, ""), odd.old, odd.new, 1)
			if w := m.call(t, as, "POST", path, body); w.Code != http.StatusBadRequest {
				t.Errorf("%s %s: %d %s", path, odd.new, w.Code, w.Body)
			}
		}
		if as == asPage {
			if w := m.call(t, as, "POST", path, nightBody(project, `,"Status":"paused"`)); w.Code != http.StatusBadRequest {
				t.Errorf("a save's \"Status\": %d %s", w.Code, w.Body)
			}
		}
	}
	if jobs := nightJobs(t, m, asPage); len(jobs) != 0 {
		t.Fatalf("kept: %v", jobs)
	}
}
