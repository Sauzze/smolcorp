//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

// GMEDIT1 (gonedesk.go) for each kind of session: a tmux session on the hire's own server (through the GM's own
// routes), a launcher's window, and a T3 Code agent. The edit is refused while it runs; a desk whose row already
// says gone (as an edit before GMEDIT1 left it) is ended by the fire, its row kept; the start lists it.

// recapsWritten is how many recaps the support folder holds.
func recapsWritten() int {
	recaps, _ := os.ReadDir(filepath.Join(supportDir, "recaps"))
	return len(recaps)
}

// goneDeskLines are the start sweep's lines in d's event log.
func goneDeskLines(d *Deck) []string {
	var lines []string
	found, _, _ := d.events.entries(0, eventsLimitMost)
	for _, e := range found {
		if e.Kind == eventKindGoneDesk {
			lines = append(lines, e.Detail)
		}
	}
	return lines
}

func TestTheGMsEditOfARunningTmuxDeskIsRefusedAndPointsToTheFire(t *testing.T) {
	g := newGMRig(t, "Ada")
	for _, asGM := range []bool{true, false} {
		for _, state := range []string{"fired", "failed"} {
			w := g.post(t, asGM, "/setup/gm/registry", map[string]any{"id": g.hire.ID, "state": state})
			want := "Ada's tmux session is still running, so it wasn't marked " + state + ": fire Ada, which ends it first."
			if w.Code != http.StatusConflict || w.Body.String() != want {
				t.Errorf("GM %v, %s: %d %s", asGM, state, w.Code, w.Body)
			}
		}
	}
	if now := registered(t)[0]; !isOpen(now) || g.r.tmux.ran("kill-server") {
		t.Fatalf("a refused edit changed something: %+v", now)
	}
	// Its nickname and notes still change while it runs.
	if w := g.post(t, true, "/setup/gm/registry", map[string]any{"id": g.hire.ID, "notes": "on the README"}); w.Code != http.StatusOK {
		t.Errorf("notes: %d %s", w.Code, w.Body)
	}
	// Once its session has ended, the edit goes ahead.
	g.r.tmux.up = false
	if w := g.post(t, true, "/setup/gm/registry", map[string]any{"id": g.hire.ID, "state": "fired"}); w.Code != http.StatusOK || registered(t)[0].State != "fired" {
		t.Errorf("ended: %d %s", w.Code, w.Body)
	}
}

func TestTheFireOfATmuxDeskMarkedGoneEndsItsSessionAndKeepsItsRow(t *testing.T) {
	for _, state := range []string{"fired", "failed"} {
		t.Run(state, func(t *testing.T) {
			g := newGMRig(t, "Ada")
			// As an edit before GMEDIT1 left it: the row gone, the session running.
			if err := updateRegistry(func(f *registryFile) { f.hires[0].State, f.hires[0].Notes = state, "by hand" }); err != nil {
				t.Fatal(err)
			}
			w := g.fire(t, true, nil)
			var answer map[string]any
			want := "Ada was already marked " + state + ", but its tmux session was still running: it's ended now, and its row stays as it was."
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &answer) != nil || answer["ok"] != true || answer["note"] != want {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
			if !g.r.tmux.ran("kill-server") || g.r.tmux.up {
				t.Errorf("the session wasn't ended: %q", g.asked())
			}
			if now := registered(t)[0]; now.State != state || now.Notes != "by hand" {
				t.Errorf("the row changed: %+v", now)
			}
			if _, err := os.Stat(g.hire.Worktree.storedOnly()); err != nil || recapsWritten() != 0 {
				t.Errorf("worktree %v, recaps %d", err, recapsWritten())
			}
			// Again: nothing is left to end.
			if w := g.fire(t, true, nil); w.Code != http.StatusConflict || w.Body.String() != "Ada is already gone." {
				t.Errorf("again: %d %s", w.Code, w.Body)
			}
		})
	}
}

func TestTheFireOfATmuxDeskMarkedGoneKeepsTheWindowRules(t *testing.T) {
	g := newGMRig(t, "Ada")
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" }); err != nil {
		t.Fatal(err)
	}
	g.showing(t, "permission-menu")
	for _, force := range []bool{false, true} {
		if w := g.fire(t, true, map[string]any{"force": force}); w.Code != http.StatusConflict {
			t.Errorf("force %v: %d %s", force, w.Code, w.Body)
		}
	}
	if g.r.tmux.ran("kill-server") || g.typed() || registered(t)[0].State != "fired" {
		t.Errorf("a refused fire ended or typed something: %q", g.asked())
	}
	// Changes nobody committed don't stop it: its worktree isn't removed.
	g.showing(t, "idle-empty")
	if err := os.WriteFile(filepath.Join(g.hire.Worktree.storedOnly(), "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := g.fire(t, true, nil); w.Code != http.StatusOK || !g.r.tmux.ran("kill-server") {
		t.Errorf("uncommitted: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(g.hire.Worktree.storedOnly(), "new.txt")); err != nil {
		t.Error(err)
	}
}

func TestTheStartListsATmuxDeskWhoseRowIsGoneWhileItsSessionRuns(t *testing.T) {
	d, r, _ := hiredForFire(t, "Ada")
	d.sweepGoneDesks()
	if lines := goneDeskLines(d); len(lines) != 0 {
		t.Fatalf("an open hire was listed: %q", lines)
	}
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" }); err != nil {
		t.Fatal(err)
	}
	d.sweepGoneDesks()
	want := []string{"Ada is marked fired, but its tmux session is still running. Fire Ada again to end it; its row stays as it is."}
	if lines := goneDeskLines(d); !reflect.DeepEqual(lines, want) {
		t.Errorf("%q", lines)
	}
	// C2: the owner reads it on Setup too, until the fire from the office's desk ends that session (nit 2: as a
	// fire, its tab pruned and its mail failed, answered with the note).
	if lines := setupGoneDesks(t, d); !reflect.DeepEqual(lines, want) {
		t.Errorf("Setup: %q", lines)
	}
	windowShows(r, pane(t, "idle-empty"), true)
	d.config.HiringEnabled = true
	d.engine = r.o
	tab := "tmux:%1@" + registered(t)[0].TmuxSocket
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": "orphan", "cwd": registered(t)[0].Worktree.storedOnly()}, Claude, tab, "", 1)
	status, message, handled := d.fireTab(tab, false)
	if note := "Ada was already marked fired, but its tmux session was still running: it's ended now, and its row stays as it was."; !handled || status != http.StatusOK || message != note || r.tmux.up {
		t.Fatalf("fireTab: %d %q %v", status, message, handled)
	}
	if slices.ContainsFunc(d.sessions.All(), func(s *Session) bool { return s.Tab == tab }) {
		t.Error("its tab's session wasn't pruned")
	}
	if lines := setupGoneDesks(t, d); len(lines) != 0 {
		t.Errorf("Setup after the fire: %q", lines)
	}
	d.sweepGoneDesks()
	if lines := goneDeskLines(d); len(lines) != 1 {
		t.Errorf("a session that ended was listed: %q", lines)
	}
}

func TestALauncherDeskIsNeverMarkedGoneWhileItsWindowRunsAndItsFireEndsAnOrphan(t *testing.T) {
	r := newLRig(t)
	h, w := r.hire("janitor")
	_, refused := editHire(registryPath(), []byte(`{"id":"`+h.ID+`","state":"fired"}`), r.d.deskSession)
	if want := "Janitor's window is still running, so it wasn't marked fired: fire Janitor, which ends it first."; refused == nil || refused.message != want {
		t.Fatalf("%+v", refused)
	}
	if !isOpen(registered(t)[0]) || w.stopped() != 0 {
		t.Fatal("a refused edit changed something")
	}
	// As an edit before GMEDIT1 left it.
	markFired(h.ID, "")
	_, err := r.d.fireLauncherHire(h, r.o, false)
	stopped, ok := err.(*hireRefusal)
	if want := "Janitor was already marked fired, but its window was still running: it's ended now, and its row stays as it was."; !ok || stopped.Status != http.StatusOK || stopped.Message != want {
		t.Fatalf("%v", err)
	}
	if id, _ := r.d.launcherOf(h.ID); id != "" || w.stopped() != 1 {
		t.Errorf("the window wasn't stopped: %d", w.stopped())
	}
	if _, err := os.Stat(h.Worktree.storedOnly()); err != nil || recapsWritten() != 0 || registered(t)[0].State != "fired" {
		t.Errorf("worktree %v, recaps %d, %+v", err, recapsWritten(), registered(t)[0])
	}
	r.fireRefused(h, false, "Janitor is already gone.")
}

func TestAT3DeskIsNeverMarkedGoneWhileItsAgentWorksAndLetGoEndsAnOrphan(t *testing.T) {
	r, d := t3DeckRig(t)
	reviewer := Role{ID: "reviewer", Name: "Reviewer", Agent: "claude"}
	if status, why := d.hireIntoT3(t3Ask{Role: reviewer, Folder: r.repo(), Nickname: "Ada", Task: "Review the README."}); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, why)
	}
	h := registered(t)[0]
	working := func(looks ...[]int) {
		t3Stand.Leaving = func() t3Leaving { leave, _ := r.leaving(looks...); return leave }
	}
	working([]int{4242})
	got := serve(t, d, t3PageCall("POST", "/setup/gm/registry", `{"id":"`+h.ID+`","state":"fired"}`))
	if want := "Ada's T3 Code agent, or another program in its worktree, is still running, so it wasn't marked fired: let Ada go from Setup's team list, which ends it first."; got.Code != http.StatusConflict || got.Body.String() != want {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	// As an edit before GMEDIT1 left it; the start lists it.
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State, f.hires[0].Notes = "fired", "by hand" }); err != nil {
		t.Fatal(err)
	}
	d.sweepGoneDesks()
	if lines, want := goneDeskLines(d), []string{"Ada is marked fired, but its T3 Code agent, or another program in its worktree, is still running. Let Ada go again from Setup's team list to end it; its row stays as it is."}; !reflect.DeepEqual(lines, want) {
		t.Errorf("%q", lines)
	}
	// Let go again: its project removed, then nothing works there; the row and recap as they were.
	working([]int{4242}, []int{})
	_ = os.Remove(r.calls)
	got = serve(t, d, t3PageCall("POST", "/setup/t3code/letgo", `{"id":"`+h.ID+`"}`))
	var answer map[string]any
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer["recap"] != "" ||
		answer["note"] != "Ada was already marked fired, but its T3 Code agent was still running: it's ended now, and its row stays as it was." {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if runs := r.runs(); !reflect.DeepEqual(runs, [][]string{{"project", "remove", h.Worktree.storedOnly(), "--force"}}) {
		t.Errorf("%q", runs)
	}
	if now := registered(t)[0]; now.State != "fired" || now.Notes != "by hand" || recapsWritten() != 0 {
		t.Errorf("%+v, recaps %d", now, recapsWritten())
	}
	// Nothing works there now: already gone, and nothing is listed.
	working([]int{})
	if got := serve(t, d, t3PageCall("POST", "/setup/t3code/letgo", `{"id":"`+h.ID+`"}`)); got.Code != http.StatusConflict || got.Body.String() != "Ada is already gone." {
		t.Errorf("%d %s", got.Code, got.Body)
	}
	d.sweepGoneDesks()
	if lines := goneDeskLines(d); len(lines) != 1 {
		t.Errorf("%q", lines)
	}
}

// setupGoneDesks are the start's lines Setup shows (hiring.goneDesks in /setup/state).
func setupGoneDesks(t *testing.T, d *Deck) []string {
	t.Helper()
	w := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	var state struct {
		Hiring struct {
			GoneDesks []string `json:"goneDesks"`
		} `json:"hiring"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &state) != nil || state.Hiring.GoneDesks == nil {
		t.Fatalf("/setup/state: %d %.200s", w.Code, w.Body)
	}
	return state.Hiring.GoneDesks
}

// RT126GMEDIT1 B1, tmux: (a) between its row and its server (no server to look at, no ticket yet), the edit refuses;
// (b) a row marked gone by another program meanwhile gets no session: the server started is ended, the ticket
// taken away, and the row stays as that program left it.
func TestATmuxStartIsNeverMarkedGoneWhileItStarts(t *testing.T) {
	d, r, _ := engineDeck(t)
	d.engine = r.o
	request := r.request("janitor")
	request.Nickname = "Ada"
	var meanwhile *refusal
	request.StartTyped = func(string) {
		rows := registered(t)
		_, meanwhile = editHire(registryPath(), []byte(`{"id":"`+rows[len(rows)-1].ID+`","state":"fired"}`), d.deskSession)
	}
	if _, err := runManagedHire(request, r.o); err != nil {
		t.Fatal(err)
	}
	if want := "Ada is still starting, so it wasn't marked fired: try again in a moment, or fire Ada once it's up."; meanwhile == nil || meanwhile.message != want {
		t.Fatalf("the edit while it started: %+v", meanwhile)
	}
	if !isOpen(registered(t)[0]) {
		t.Error("a refused edit changed the row")
	}
}

func TestATmuxStartStartsNothingBehindARowMarkedGoneMeanwhile(t *testing.T) {
	_, r, _ := engineDeck(t)
	run := r.o.Tmux
	r.o.Tmux = func(args []string) (string, int, bool) {
		if args[2] == "new-session" { // another program marks it gone as its server starts
			if err := updateRegistry(func(f *registryFile) { f.hires[0].State, f.hires[0].Notes = "fired", "by another program" }); err != nil {
				t.Error(err)
			}
		}
		return run(args)
	}
	request := r.request("janitor")
	request.Nickname = "Ada"
	_, err := runManagedHire(request, r.o)
	refused, ok := err.(*hireRefusal)
	if want := "Ada was marked gone while it was starting, so its tmux session was ended."; !ok || refused.Status != http.StatusConflict || refused.Message != want {
		t.Fatalf("%v", err)
	}
	if !r.tmux.ran("kill-server") || r.tmux.up {
		t.Errorf("its server wasn't ended: %q", r.tmux.asked)
	}
	rows := registered(t)
	if len(rows) != 1 || rows[0].State != "fired" || rows[0].Notes != "by another program" {
		t.Errorf("%+v", rows)
	}
	if _, ticket := issuedPlan(rows[0].ID); ticket {
		t.Error("its start ticket is still there")
	}
}

// RT126GMEDIT1 B1, launcher: (a) between its row and its token, and while its token waits for its window, the edit
// refuses; (b) a row marked gone by another program meanwhile: its window gets no environment, and a window that
// connects with its token isn't tied to it.
func TestALauncherStartIsNeverMarkedGoneAndStartsNothingBehindAGoneRow(t *testing.T) {
	r := newLRig(t)
	r.launches = false
	expect := r.o.Expect
	var meanwhile *refusal
	r.o.Expect = func(token, hireID string, env hireStartEnv) {
		_, meanwhile = editHire(registryPath(), []byte(`{"id":"`+hireID+`","state":"fired"}`), r.d.deskSession)
		expect(token, hireID, env)
	}
	request := r.request("janitor")
	request.Nickname = "Jo"
	h, token, err := runLauncherHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	want := "Jo is still starting, so it wasn't marked fired: try again in a moment, or fire Jo once it's up."
	if meanwhile == nil || meanwhile.message != want {
		t.Fatalf("before its token: %+v", meanwhile)
	}
	if _, refused := editHire(registryPath(), []byte(`{"id":"`+h.ID+`","state":"failed"}`), r.d.deskSession); refused == nil ||
		refused.message != "Jo is still starting, so it wasn't marked failed: try again in a moment, or fire Jo once it's up." {
		t.Fatalf("its token waiting: %+v", refused)
	}
	if !isOpen(registered(t)[0]) {
		t.Fatal("a refused edit changed the row")
	}
	// Marked gone by another program before its window asks.
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" }); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchHireStartEnv(r.srv.Client(), r.srv.URL, r.d.access.HookKey, token, h.ID); err == nil ||
		err.Error() != "this hire was marked gone before its window opened, so nothing was started" {
		t.Errorf("its window's environment: %v", err)
	}
	again := newID()
	r.d.expectLauncher(again, h.ID, hireStartEnv{})
	if tab := r.d.launcherConnected("L-late", Claude, h.storedPlace(), "janitor", again); tab == nil || tab.hireID != "" {
		t.Errorf("a window was tied to a gone row: %+v", tab)
	}
	if id, _ := r.d.launcherOf(h.ID); id != "" {
		t.Errorf("launcherOf %q", id)
	}
}

// RT126GMEDIT1 B2: a launcher's window that connected again without its id can't be told from here, so the edit
// refuses and says to close it there; once it's gone the edit goes ahead. A row already gone with such a window up
// is refused by the fire, which can't end it either, in words that say so.
func TestALauncherWindowThatConnectedAgainWithoutItsIDIsNeverMarkedGone(t *testing.T) {
	r := newLRig(t)
	h, w := r.hire("janitor")
	setHire(h.ID, "working", "")
	again := r.reconnect(h, w, flagAfter(r.opened[0], "--hire"))
	_, refused := editHire(registryPath(), []byte(`{"id":"`+h.ID+`","state":"fired"}`), r.d.deskSession)
	if want := "Smolcorp can't tell whether Janitor's window is still running, so it wasn't marked fired. A window connected again in its worktree without its id, and Smolcorp can't end that: close it there, then try again."; refused == nil || refused.message != want {
		t.Fatalf("%+v", refused)
	}
	if !isOpen(registered(t)[0]) {
		t.Fatal("a refused edit changed the row")
	}
	r.d.launcherGone(again.id, again.tab)
	if _, refused := editHire(registryPath(), []byte(`{"id":"`+h.ID+`","state":"fired"}`), r.d.deskSession); refused != nil || registered(t)[0].State != "fired" {
		t.Fatalf("its window gone: %+v", refused)
	}
	third := r.connect("L-third-"+h.ID, h.storedPlace(), "janitor", flagAfter(r.opened[0], "--hire"), "", w.env)
	_, err := r.d.fireLauncherHire(registered(t)[0], r.o, true)
	if want := "Janitor is already marked fired, but a window connected again in its worktree without its id, and Smolcorp can't end that: close it there."; err == nil || err.Error() != want {
		t.Errorf("the fire: %v", err)
	}
	if third.stopped() != 0 {
		t.Error("a window was stopped")
	}
}

// RT126GMEDIT1 B1, T3: its row read again right before its thread starts (the deck's own wiring): marked gone by
// another program meanwhile, no thread is started and its project is taken back.
func TestAT3StartStartsNoThreadBehindARowMarkedGoneMeanwhile(t *testing.T) {
	r, d := t3DeckRig(t)
	inner := newT3Machine
	newT3Machine = func() t3Machine {
		m := inner()
		run := m.Run
		m.Run = func(program string, args []string, within time.Duration) t3Ran {
			ran := run(program, args, within)
			if len(args) > 1 && args[0] == "project" && args[1] == "add" {
				_ = updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" })
			}
			return ran
		}
		return m
	}
	t.Cleanup(func() { newT3Machine = inner })
	reviewer := Role{ID: "reviewer", Name: "Reviewer", Agent: "claude"}
	status, why := d.hireIntoT3(t3Ask{Role: reviewer, Folder: r.repo(), Nickname: "Ada", Task: "Review the README."})
	if want := "Ada was marked gone while it was starting, so its thread wasn't started."; status != http.StatusConflict || why != want {
		t.Fatalf("%d %s", status, why)
	}
	runs := r.runs()
	if slices.ContainsFunc(runs, func(a []string) bool { return a[0] == "thread" }) ||
		!slices.ContainsFunc(runs, func(a []string) bool { return len(a) > 1 && a[0] == "project" && a[1] == "remove" }) {
		t.Errorf("%q", runs)
	}
	if now := registered(t)[0]; isOpen(now) {
		t.Errorf("%+v", now)
	}
}

// Verdict nit 3: the app's own start runs the sweep (NewDeck), and the owner reads its line on Setup.
func TestANewDeckListsAGoneDeskStillRunningOnSetup(t *testing.T) {
	isolate(t)
	goneDesksSwept = true // put back by isolate
	kept := t3Stand
	t.Cleanup(func() { t3Stand = kept }) // after the deck has stopped
	t3Stand.Leaving = func() t3Leaving { return t3Leaving{Working: func(string) ([]int, bool) { return []int{4242}, true }} }
	h := Hire{ID: "a1b2c3", Name: "Reviewer", Nickname: "Ada", Role: "reviewer", Folder: keptPath(homeDir), Worktree: keptPath(t.TempDir()), State: "fired", Engine: t3Engine}
	if err := updateRegistry(func(f *registryFile) { f.hires = []Hire{h} }); err != nil {
		t.Fatal(err)
	}
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{}) // no coding agent is looked for (newTestDeck)
	want := []string{"Ada is marked fired, but its T3 Code agent, or another program in its worktree, is still running. Let Ada go again from Setup's team list to end it; its row stays as it is."}
	for end := time.Now().Add(5 * time.Second); len(setupGoneDesks(t, d)) == 0 && time.Now().Before(end); {
		time.Sleep(10 * time.Millisecond)
	}
	if lines := setupGoneDesks(t, d); !reflect.DeepEqual(lines, want) {
		t.Errorf("Setup: %q", lines)
	}
	if lines := goneDeskLines(d); !reflect.DeepEqual(lines, want) {
		t.Errorf("events: %q", lines)
	}
}
