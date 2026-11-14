//go:build !windows

package desktop

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The night shift's runner, first part (nightstart.go, NS1): an approval is a sealed record, given with the hash of
// what the page showed; a GM's proposal keeps the scope it was held to, and its runs are held to it; only an
// approved job passes the hire queue's approval, for its one hire; each start is checked again and how it went is
// kept for the morning report. Every hire here is a stand-in: nothing starts an agent or a terminal.

// waitNight waits for the job running to end: its queue's end is told on a goroutine of its own.
func waitNight(t *testing.T, d *Deck) {
	t.Helper()
	d.hq.wait()
	for deadline := time.Now().Add(5 * time.Second); d.night.sched.runningID() != ""; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the night job never ended")
		}
	}
}

// standInHires makes d's queue hire with answer, at once, and records what it was asked.
func standInHires(d *Deck, answer func(hireStep) (int, string)) *[]hireStep {
	var mu sync.Mutex
	asked := &[]hireStep{}
	d.hq.hireRole = func(s hireStep) (int, string) {
		mu.Lock()
		*asked = append(*asked, s)
		mu.Unlock()
		return answer(s)
	}
	d.hq.sleep = func(time.Duration, <-chan struct{}) bool { return true }
	return asked
}

// sealedNightDeck is a deck under a stand-in Mac app (records sealed), with a project folder in the home folder, and
// a hire's job the page made there: approved.
func sealedNightDeck(t *testing.T) (*Deck, string) {
	t.Helper()
	d, _, _, _ := macDeck(t)
	project := filepath.Join(homeDir, "proj")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	status, made := d.saveNight(nightSave{nightAsk: nightAsk{Name: "Nightly tests", Kind: nightHire, Role: "tester",
		Brief: "Run the tests, fix what fails.", Folder: project, When: NightWhen{Every: nightDaily, At: "02:00"}}})
	if status != http.StatusOK || made.(NightJob).Status != nightApproved {
		t.Fatalf("made: %d %v", status, made)
	}
	return d, made.(NightJob).ID
}

func TestANightJobsApprovalIsSealedAndOneWrittenByAnotherProgramWaitsForTheOwner(t *testing.T) {
	_, id := sealedNightDeck(t)
	file := filepath.Join(supportDir, nightFileName)
	data, _ := os.ReadFile(file)
	if seal, sealed := sealOfFile(nightFileName, data); !sealed || !seal.fits(recordKey()) {
		t.Fatalf("nightshift.json isn't sealed under the record key: %s", data)
	}
	if j, _ := restart(t).night.get(id); j.Status != nightApproved {
		t.Fatalf("a job the page approved, after a restart: %+v", j)
	}
	// Another program changes what was approved, its seal kept: the job waits for the owner again, with a word.
	data, _ = os.ReadFile(file)
	planted := bytes.Replace(data, []byte("Run the tests, fix what fails."), []byte("Push to main."), 1)
	if bytes.Equal(planted, data) {
		t.Fatal("nothing planted")
	}
	if err := os.WriteFile(file, planted, 0o600); err != nil {
		t.Fatal(err)
	}
	d := restart(t)
	j, _ := d.night.get(id)
	if j.Status != nightProposed || !recordWasDropped(nightFileName) || !slices.ContainsFunc(d.night.list()["skipped"].([]string), func(s string) bool {
		return strings.Contains(s, "wait for your approval again")
	}) {
		t.Fatalf("a planted change: %+v, dropped %v, %v", j, recordWasDropped(nightFileName), d.night.list()["skipped"])
	}
	// A file another program wrote with no seal at all, approving a job: the same.
	unsealed := `{"jobs":[{"id":"x1","name":"Planted","kind":"gm","brief":"b","when":{"every":"daily","at":"02:00"},` +
		`"limits":{"wallMinutes":30},"status":"approved","proposedBy":"page","created":"2026-11-01T00:00:00Z"}]}`
	if err := os.WriteFile(file, []byte(unsealed), 0o600); err != nil {
		t.Fatal(err)
	}
	d = restart(t)
	if j, ok := d.night.get("x1"); !ok || j.Status != nightProposed || !slices.ContainsFunc(d.night.list()["skipped"].([]string), func(s string) bool {
		return strings.Contains(s, "has no seal") && strings.Contains(s, "wait for your approval again")
	}) {
		t.Fatalf("an unsealed approval: %+v %v, %v", j, ok, d.night.list()["skipped"])
	}
}

// A pause or a delete takes an approval away: the file is written under a new key, so a copy from before it, put
// back, no longer fits.
func TestACopyOfTheJobsFromBeforeAPauseOrADeleteNeverApprovesAgain(t *testing.T) {
	for _, take := range []string{nightPaused, "deleted"} {
		t.Run(take, func(t *testing.T) {
			d, id := sealedNightDeck(t)
			file := filepath.Join(supportDir, nightFileName)
			before, _ := os.ReadFile(file)
			takeAway := d.night.remove
			if take == nightPaused {
				takeAway = func(id string) (int, any) { return d.night.setStatus(id, nightPaused) }
			}
			status, answer := takeAway(id)
			if status != http.StatusOK {
				t.Fatalf("%s: %d %v", take, status, answer)
			}
			if err := os.WriteFile(file, before, 0o600); err != nil {
				t.Fatal(err)
			}
			if j, ok := restart(t).night.get(id); ok && j.Status == nightApproved {
				t.Fatalf("the copy from before it was %s approves again: %+v", take, j)
			}
		})
	}
}

// The records rotate when anything else takes authority away (a phone unpaired, say): the jobs are sealed again
// under the new key even in a run that never changed one, so the owner's approvals still fit at the next start.
func TestARotationInARunThatChangedNoJobKeepsTheApprovals(t *testing.T) {
	_, id := sealedNightDeck(t)
	restart(t)
	rotateRecords()
	if j, _ := restart(t).night.get(id); j.Status != nightApproved {
		t.Fatalf("after a rotation: %+v", j)
	}
}

func TestTheOwnerApprovesANightJobAsThePageShowedIt(t *testing.T) {
	m, project, _ := nightDeck(t)
	id := decoded(t, m.call(t, asGM, "POST", nightProposePath, nightBody(project, "")))["id"].(string)
	shown := shownOf(t, m, id)
	for _, bad := range []struct {
		shown string
		code  int
	}{{"", http.StatusBadRequest}, {strings.Repeat("0", 64), http.StatusConflict}} {
		if w := m.call(t, asPage, "POST", nightApprovePath+"?id="+id+"&shown="+bad.shown, ""); w.Code != bad.code {
			t.Errorf("approved with %q: %d %s", bad.shown, w.Code, w.Body)
		}
	}
	// The page edits the proposal: what it showed before no longer approves it.
	edit := strings.Replace(nightBody(project, `,"id":"`+id+`"`), "Run the tests, fix what fails.", "Run the tests.", 1)
	if w := m.call(t, asPage, "POST", nightSavePath, edit); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	if w := m.call(t, asPage, "POST", nightApprovePath+"?id="+id+"&shown="+shown, ""); w.Code != http.StatusConflict {
		t.Fatalf("approved as it was before the edit: %d %s", w.Code, w.Body)
	}
	if j, _ := m.d.night.get(id); j.Status != nightProposed {
		t.Fatalf("approved anyway: %+v", j)
	}
	if w := m.call(t, asPage, "POST", nightApprovePath+"?id="+id+"&shown="+shownOf(t, m, id), ""); w.Code != http.StatusOK || decoded(t, w)["status"] != nightApproved {
		t.Fatalf("approved as shown: %d %s", w.Code, w.Body)
	}
	// Every field the owner approves from is in the hash; the scheduler's aren't.
	j, _ := m.d.night.get(id)
	for _, change := range []func(*NightJob){
		func(j *NightJob) { j.Name += "!" }, func(j *NightJob) { j.Brief += "!" }, func(j *NightJob) { j.Folder += "x" },
		func(j *NightJob) { j.Role = "builder" }, func(j *NightJob) { j.When.At = "03:00" }, func(j *NightJob) { j.Limits.WallMinutes++ },
		func(j *NightJob) { j.Scope = "/elsewhere" }, func(j *NightJob) { j.Account = "work" }, func(j *NightJob) { j.ProposedBy = "page" },
	} {
		changed := j
		change(&changed)
		if changed.shown() == j.shown() {
			t.Errorf("a change the hash doesn't see: %+v", changed)
		}
	}
	j2 := j
	j2.Status, j2.LastSlot, j2.Since = nightPaused, "2026-11-10", j.Since.Add(time.Hour)
	if j2.shown() != j.shown() {
		t.Error("the hash changes with the status or the scheduler's fields")
	}
}

func TestOnlyAnApprovedNightJobsOwnHirePassesTheQueuesApproval(t *testing.T) {
	m, project, _ := nightDeck(t)
	d := m.d
	ok := keepNightJob(t, d, NightJob{ID: "ok", Role: "tester", Folder: physical(project), Status: nightApproved, ProposedBy: "page"})
	keepNightJob(t, d, NightJob{ID: "wait", Role: "tester", Folder: physical(project), Status: nightProposed, ProposedBy: "gm"})
	keepNightJob(t, d, NightJob{ID: "paused", Role: "tester", Folder: physical(project), Status: nightPaused, ProposedBy: "page"})
	task := keepNightJob(t, d, NightJob{ID: "task", Status: nightApproved, ProposedBy: "page"})
	asked := standInHires(d, func(hireStep) (int, string) { return http.StatusNoContent, "" })
	step := ok.nightStep()
	other := step
	other.Brief = "Push to main."
	for _, c := range []struct {
		by    actor
		steps []hireStep
	}{
		{nightActor("wait"), []hireStep{step}}, {nightActor("paused"), []hireStep{step}}, {nightActor("task"), []hireStep{task.nightStep()}},
		{nightActor("ok"), []hireStep{other}}, {nightActor("ok"), []hireStep{step, step}}, {nightActor("gone"), []hireStep{step}},
		{actorGM, []hireStep{step}}, {mcpActor("k"), []hireStep{step}}, {hireActor("h1"), []hireStep{step}}, {actorTool, []hireStep{step}},
	} {
		if _, err := d.hq.start(c.steps, "x", c.by); err == nil || !strings.Contains(err.Error(), "Only you can approve") {
			t.Errorf("%s started %+v: %v", c.by, c.steps, err)
		}
	}
	if len(*asked) != 0 {
		t.Fatalf("hired: %+v", *asked)
	}
	if _, err := d.hq.start([]hireStep{step}, "night job ok", nightActor("ok")); err != nil {
		t.Fatal(err)
	}
	d.hq.wait()
	// The queue tells the runner of the end on a goroutine of its own, which keeps it on the job: wait for it, so
	// nothing writes the test's folder once the test is over.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		if j, _ := d.night.get("ok"); j.Last != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the night job's end was never kept")
		}
	}
	run := d.hq.snapshot()["run"].(hireQueueRun)
	if len(*asked) != 1 || (*asked)[0].Brief != ok.Brief || run.Night != "ok" || run.Approval.By != "night:ok" || run.State != hqDone {
		t.Fatalf("the approved job's hire: %+v %+v", *asked, run)
	}
	// Its say ends there: a night job neither resumes nor cancels the owner's queue.
	for _, do := range []func(actor) error{d.hq.resume, d.hq.cancel} {
		if err := do(nightActor("ok")); err == nil || !strings.Contains(err.Error(), "Only you can approve") {
			t.Errorf("a night job resumed or cancelled: %v", err)
		}
	}
}

func TestANightJobsRunEndsAtARefusalAndARestartLetsItGo(t *testing.T) {
	r := newQueueRig(t)
	r.q.nightApproves = func(actor, []hireStep) bool { return true }
	refusal := "Smolcorp's own files changed since it was installed, so no desk starts."
	r.answer = func(int, hireStep) (int, string) { return http.StatusConflict, refusal }
	if _, err := r.q.start([]hireStep{{Role: "tester"}}, "night job n", nightActor("n")); err != nil {
		t.Fatal(err)
	}
	r.q.wait()
	run := r.q.snapshot()["run"].(hireQueueRun)
	if run.State != hqEnded || !strings.Contains(run.Word, refusal) {
		t.Fatalf("refused: %+v", run)
	}
	// Nobody resumes it, so it holds nothing: the owner's queue starts.
	r.answer = func(int, hireStep) (int, string) { return http.StatusNoContent, "" }
	if _, err := r.q.start([]hireStep{{Role: "tester"}}, "the page's", actorPage); err != nil {
		t.Fatalf("after a night job's refusal: %v", err)
	}
	r.q.wait()
	// A night job's run the app stopped in the middle of is let go at the next start, never left to be resumed.
	r.q.mu.Lock()
	r.q.run = &hireQueueRun{ID: "r1", Label: "night job n", Night: "n", State: hqRunning, Steps: []hireStep{{Role: "tester", State: hqHiring}}}
	r.q.keep()
	r.q.mu.Unlock()
	again := r.build()
	if run := again.snapshot()["run"].(hireQueueRun); run.State != hqAbandoned {
		t.Fatalf("after a restart: %+v", run)
	}
}

func TestTheRunnerStartsAnApprovedJobsHireAndKeepsHowItWent(t *testing.T) {
	m, project, clock := nightDeck(t)
	d := m.d
	answer := struct {
		status int
		text   string
	}{http.StatusNoContent, ""}
	asked := standInHires(d, func(hireStep) (int, string) { return answer.status, answer.text })
	id := decoded(t, m.call(t, asPage, "POST", nightSavePath, nightBody(project, "")))["id"].(string)
	d.night.sched.begin(d.nightDue)
	*clock = time.Date(2026, 11, 10, 2, 0, 30, 0, clock.Location())
	d.night.sched.tick()
	waitNight(t, d)
	j, _ := d.night.get(id)
	want := hireStep{Role: "tester", Folder: physical(project), Brief: "Run the tests, fix what fails."}
	// The queue hands the hire its step as it works on it (its state, when it began): what was asked is the rest.
	got := hireStep{}
	if len(*asked) == 1 {
		a := (*asked)[0]
		got = hireStep{Role: a.Role, Name: a.Name, Folder: a.Folder, Brief: a.Brief, Account: a.Account, Floor: a.Floor}
	}
	if len(*asked) != 1 || got != want || j.Last == nil || !j.Last.Hired || !strings.HasPrefix(j.Last.Word, "Done:") {
		t.Fatalf("the hire: %+v, last %+v", *asked, j.Last)
	}
	// SG2's refusal stops the hire: its own sentence is kept for the morning report, and the next night tries again.
	answer.status, answer.text = http.StatusConflict, "Smolcorp's own files changed since it was installed, so no desk starts."
	*clock = clock.Add(24 * time.Hour)
	d.night.sched.tick()
	waitNight(t, d)
	j, _ = d.night.get(id)
	if len(*asked) != 2 || j.Last.Hired || !strings.Contains(j.Last.Word, answer.text) || d.hq.snapshot()["run"].(hireQueueRun).State != hqEnded {
		t.Fatalf("refused: %+v, %+v", j.Last, d.hq.snapshot()["run"])
	}
	// What's kept is read back as the job's, its seal fitting.
	if j, _ := restart(t).night.get(id); j.Status != nightApproved || j.Last == nil || !strings.Contains(j.Last.Word, answer.text) {
		t.Fatalf("after a restart: %+v", j)
	}
}

// A long sentence, in letters of more than one byte too, is cut to what the job keeps, never replaced (RT126NS1 N4).
func TestANightStartsLongSentenceIsCutToWhatTheJobKeeps(t *testing.T) {
	m, project, _ := nightDeck(t)
	d := m.d
	keepNightJob(t, d, NightJob{ID: "n", Role: "tester", Folder: physical(project), Status: nightApproved, ProposedBy: "page"})
	for _, word := range []string{strings.Repeat("é", 700), strings.Repeat("a", 700), strings.Repeat("日", 400)} {
		d.night.ran("n", NightRun{Word: word})
		j, _ := d.night.get("n")
		if j.Last == nil || !strings.HasSuffix(j.Last.Word, "…") || !strings.HasPrefix(j.Last.Word, word[:6]) || j.checkedShape() != "" {
			t.Errorf("%s…: kept %+v", word[:6], j.Last)
		}
	}
}

// Each start checks the job again as it is now, and a job that can't start never reaches the queue.
func TestANightJobIsCheckedAgainAsItStarts(t *testing.T) {
	m, project, _ := nightDeck(t)
	d := m.d
	asked := standInHires(d, func(hireStep) (int, string) { return http.StatusNoContent, "" })
	elsewhere := filepath.Join(homeDir, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	// A folder whose way in is later swapped for a link to somewhere else.
	moved := filepath.Join(homeDir, "real", "app")
	if err := os.MkdirAll(moved, 0o700); err != nil || os.MkdirAll(filepath.Join(elsewhere, "app"), 0o700) != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(homeDir, "gone")
	if err := os.MkdirAll(gone, 0o700); err != nil {
		t.Fatal(err)
	}
	jobs := map[string]NightJob{
		"off":     {Role: "tester", Folder: physical(project)},
		"task":    {},
		"codex":   {Role: "coder", Folder: physical(project)},
		"nobody":  {Role: "nobody", Folder: physical(project)},
		"gone":    {Role: "tester", Folder: physical(gone)},
		"moved":   {Role: "tester", Folder: physical(moved)},
		"outside": {Role: "tester", Folder: physical(elsewhere), Scope: physical(project)},
	}
	words := map[string]string{
		"off": "switched off", "task": "task for the General Manager", "codex": "Claude Code", "nobody": "isn't one of the office's",
		"gone": "no folder", "moved": "isn't the one you approved", "outside": "outside " + physical(project),
	}
	d.mu.Lock()
	d.config.Roles = append(d.config.Roles, Role{ID: "coder", Name: "Coder", Agent: "codex"}, Role{ID: "nobody", Name: "Nobody", Agent: "claude"})
	d.mu.Unlock()
	for id, j := range jobs {
		j.ID, j.Status, j.ProposedBy = id, nightApproved, "page"
		keepNightJob(t, d, j)
	}
	d.mu.Lock()
	d.config.Roles = slices.DeleteFunc(d.config.Roles, func(r Role) bool { return r.ID == "nobody" })
	d.mu.Unlock()
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(homeDir, "real")
	if err := os.Rename(real, real+".old"); err != nil || os.Symlink(elsewhere, real) != nil {
		t.Fatal("the folder's way in wasn't swapped")
	}
	for id := range jobs {
		managerOn(d)
		if id == "off" {
			d.mu.Lock()
			d.config.ManagerEnabled = false
			d.mu.Unlock()
		}
		j, _ := d.night.get(id)
		done := false
		d.nightDue(nightDue{Job: j, Actor: j.actor()}, func() { done = true })
		j, _ = d.night.get(id)
		if !done || j.Last == nil || j.Last.Hired || !strings.Contains(j.Last.Word, words[id]) {
			t.Errorf("%s: done %v, %+v", id, done, j.Last)
		}
	}
	if len(*asked) != 0 {
		t.Fatalf("hired: %+v", *asked)
	}
}

// RT126N1 C3: a GM held to one project proposes a job, which keeps that scope; its runs are held to it, whoever is
// the GM then. A job of the page's own is held to no project, though the GM of the moment is.
func TestAScopedGMsNightJobKeepsItsScopeAndItsRunsAreHeldToIt(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	gm := gmNow()
	body := `{"name":"n","kind":"hire","role":"tester","brief":"b","folder":"` + project + `","when":{"every":"daily","at":"02:00"},"limits":{"wallMinutes":30}}`
	code, text := queueCall(t, d, asGMCaller, token, "POST", nightProposePath, body)
	if code != http.StatusOK {
		t.Fatalf("proposed: %d %s", code, text)
	}
	var id string
	for _, j := range d.night.jobs {
		id = j.ID
	}
	j, _ := d.night.get(id)
	if j.Scope != project || j.Scope != gm.Scope || j.Account != gm.Account || j.ProposedBy != "gm" {
		t.Fatalf("the proposal: %+v (GM %+v)", j, gm)
	}
	d.night.mu.Lock()
	d.night.setStatusLocked(d.night.index(id), nightApproved)
	d.night.mu.Unlock()
	pageJob := keepNightJob(t, d, NightJob{ID: "mine", Status: nightApproved, ProposedBy: "page"})
	tile := func(a actor) verdict {
		return d.decide(a, action{route: &routeAsk{method: "POST", path: "/setup/gm/tile", body: strings.NewReader(`{}`)}})
	}
	if v := tile(nightActor(id)); v.allowed() || !strings.Contains(v.why, "works only in "+project) {
		t.Errorf("the GM's job, beyond its scope: %+v", v)
	}
	if v := tile(pageJob.actor()); !v.allowed() {
		t.Errorf("the page's own job, held to the GM of the moment's scope: %+v", v)
	}
	if v := d.decide(nightActor("nope"), action{route: &routeAsk{method: "GET", path: "/setup/gm/team"}}); v.allowed() {
		t.Errorf("a job that isn't there: %+v", v)
	}
	// At its start, a folder its proposer's project doesn't hold is refused (as one put in the file by hand would be).
	j, _ = d.night.get(id)
	other := filepath.Join(homeDir, "other")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	j.Folder = physical(other)
	if why := d.nightStartRefusal(j); !strings.Contains(why, "outside "+project) {
		t.Errorf("a folder outside the scope: %q", why)
	}
}

// The app's deck starts the scheduler: an approved job falls due without anything else asking.
func TestADeckStartsTheNightShift(t *testing.T) {
	isolate(t)
	nightShiftStarts = true
	d := startDeck(t)
	d.night.sched.mu.Lock()
	started := d.night.sched.due != nil
	d.night.sched.mu.Unlock()
	if !started {
		t.Fatal("the deck didn't start the night shift's scheduler")
	}
}

// While the record key can't be read, the approvals in the file wait, and nothing is changed that would vanish.
func TestNoNightJobChangesWhileTheRecordsAreHeldOnly(t *testing.T) {
	m, project, _ := nightDeck(t)
	id := decoded(t, m.call(t, asPage, "POST", nightSavePath, nightBody(project, "")))["id"].(string)
	records.mu.Lock()
	was := records.state
	records.state = recordsUnreadable
	records.mu.Unlock()
	t.Cleanup(func() { records.mu.Lock(); records.state = was; records.mu.Unlock() })
	for _, path := range []string{nightSavePath, nightProposePath, nightPausePath + "?id=" + id, nightDeletePath + "?id=" + id} {
		if w := m.call(t, asPage, "POST", path, nightBody(project, "")); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "login keychain") {
			t.Errorf("%s while held: %d %s", path, w.Code, w.Body)
		}
	}
	if j, _ := m.d.night.get(id); j.Status != nightApproved {
		t.Fatalf("changed while held: %+v", j)
	}
}
