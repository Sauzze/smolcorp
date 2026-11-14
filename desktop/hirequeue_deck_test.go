//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// The plan and the queue on the deck, through its routes and its real hire path on the engine tests' stand-ins
// (engineDeck): nothing starts a real agent or a real terminal.

// queueDeckOf is the GM hire's deck (manager on, a folder for hires in the home folder, a GM token), with the
// queue's waits made instant: the deck's own cooldown is let go with each wait, as five seconds would have.
func queueDeckOf(t *testing.T) (*Deck, *engineRig, *hiring, string) {
	t.Helper()
	d, r, h, token := gmHireDeck(t)
	d.hq.sleep = func(time.Duration, <-chan struct{}) bool {
		d.mu.Lock()
		d.lastHire = time.Time{}
		d.mu.Unlock()
		return true
	}
	return d, r, h, token
}

// Who asks a plan or queue route.
const (
	asPageWithGMToken = "page+gm" // the page's cookie, with the GM's token riding along
	asToolCaller      = "tool"
	asToolWithGMToken = "tool+gm" // the setup key and the GM's token: for a route a tool may not use, the GM
)

func queueCall(t *testing.T, d *Deck, as, token, method, path, body string) (int, string) {
	t.Helper()
	c := call{method: method, target: path, host: "localhost", from: thisComputer, body: body, headers: map[string]string{setupHeader: "1"}}
	switch as {
	case asPageCaller:
	case asPageWithGMToken:
		c.headers[gmTokenHeader] = token
	case asToolCaller:
		c.asTool = true
	case asToolWithGMToken:
		c.asTool, c.headers[gmTokenHeader] = true, token
	case asGMCaller:
		c.withoutSetupKey, c.headers[gmTokenHeader] = true, token
	case asHireCaller:
		c.withoutSetupKey, c.headers[hireKeyHeader] = true, token
	}
	w := serve(t, d, c)
	d.watching.Wait()
	return w.Code, strings.TrimSpace(w.Body.String())
}

func planOf(t *testing.T, text string) hirePlan {
	t.Helper()
	var plan hirePlan
	if err := json.Unmarshal([]byte(text), &plan); err != nil || plan.ID == "" {
		t.Fatalf("a plan was expected: %q, %v", text, err)
	}
	return plan
}

func planRoles(plan hirePlan) []string {
	var roles []string
	for _, s := range plan.Steps {
		roles = append(roles, s.Role)
	}
	return roles
}

// settled waits for the queue to stop and for the watchers of the hires it made.
func settled(d *Deck) {
	d.hq.wait()
	d.watching.Wait()
}

func queueRun(t *testing.T, d *Deck) hireQueueRun {
	t.Helper()
	run, _ := d.hq.snapshot()["run"].(hireQueueRun)
	return run
}

func queueEvents(d *Deck) string {
	var lines []string
	for _, e := range d.events.snapshot() {
		if e.Kind == eventKindHireQueue {
			lines = append(lines, e.Detail)
		}
	}
	return strings.Join(lines, "\n")
}

// Each template's plan is answer 13's roles, the GM first: Small agency has the Designer, not the Art Director.
func TestEachTemplatesPlanHasTheRolesOfAnswer13(t *testing.T) {
	d, _, _, _ := queueDeckOf(t)
	want := map[string][]string{
		"solo-builder": {"gm", "builder", "reviewer", "tester"},
		"small-agency": {"gm", "builder", "builder", "designer", "reviewer", "docs"},
		"bug-squash":   {"gm", "bughunter", "tester", "reviewer"},
	}
	if len(companyTemplates) != len(want) {
		t.Fatalf("%d templates, want %d", len(companyTemplates), len(want))
	}
	for _, template := range companyTemplates {
		code, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"`+template.ID+`"}`)
		if code != http.StatusOK {
			t.Fatalf("%s: %d %q", template.ID, code, text)
		}
		plan := planOf(t, text)
		if got := planRoles(plan); !slices.Equal(got, want[template.ID]) || plan.Status != hpProposed || plan.By != "smolcorp" {
			t.Errorf("%s: roles %q, %s by %s; want %q proposed by Smolcorp", template.ID, got, plan.Status, plan.By, want[template.ID])
		}
		for _, s := range plan.Steps {
			if s.Folder == "" || s.Brief == "" || s.Name == "" || len(s.Brief) > hirePlanBriefLimit {
				t.Errorf("%s: a step lacks its folder, name or brief line: %+v", template.ID, s)
			}
		}
	}
	// Small agency's two builders are told apart in their brief lines.
	_, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"small-agency"}`)
	plan := planOf(t, text)
	if !strings.Contains(plan.Steps[1].Brief, "(1 of 2)") || !strings.Contains(plan.Steps[2].Brief, "(2 of 2)") || strings.Contains(plan.Steps[3].Brief, "(1 of") {
		t.Errorf("brief lines %q / %q / %q", plan.Steps[1].Brief, plan.Steps[2].Brief, plan.Steps[3].Brief)
	}
	// With a GM up already, the plan is the roles alone.
	writeRegistry(t, Hire{ID: "g1", Name: "Boss", Folder: "/w", Role: "gm", State: "working"})
	_, text = queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"solo-builder","planner":"own"}`)
	if got := planRoles(planOf(t, text)); !slices.Equal(got, []string{"builder", "reviewer", "tester"}) {
		t.Errorf("with a GM up: %q", got)
	}
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"nope"}`); code != http.StatusNotFound {
		t.Errorf("an unknown template: %d", code)
	}
}

func TestNothingIsHiredUntilThePageApprovesAndNoOneElseCan(t *testing.T) {
	d, r, h, token := queueDeckOf(t)
	_, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"solo-builder"}`)
	plan := planOf(t, text)
	approve := `{"id":"` + plan.ID + `","rev":` + "1" + `}`
	// A plan alone hires nothing.
	settled(d)
	nothingMade(t, r, h)
	// A hire's key, the GM's token, a tool, a tool with the GM's token, and the page with the GM's token riding along.
	writeRegistry(t, Hire{ID: "a1b2c3", Name: "Rev", Folder: "/w", Role: "reviewer", State: "working"})
	key, err := newSendKey(registryPath(), "a1b2c3")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ as, token string }{
		{asHireCaller, key}, {asGMCaller, token}, {asToolCaller, ""}, {asToolWithGMToken, token}, {asPageWithGMToken, token},
	} {
		code, why := queueCall(t, d, c.as, c.token, "POST", "/setup/hire/plan/approve", approve)
		if code != http.StatusForbidden {
			t.Errorf("%s approved: %d %q", c.as, code, why)
		}
		for _, path := range []string{"/setup/hire/queue/resume", "/setup/hire/queue/cancel", "/setup/hire/plan/cancel"} {
			if code, _ := queueCall(t, d, c.as, c.token, "POST", path, "{}"); code != http.StatusForbidden {
				t.Errorf("%s used %s: %d", c.as, path, code)
			}
		}
		// Nor may any of them ask for a plan.
		if code, _ := queueCall(t, d, c.as, c.token, "POST", "/setup/hire/plan", `{"template":"bug-squash"}`); code != http.StatusForbidden {
			t.Errorf("%s asked for a plan: %d", c.as, code)
		}
	}
	settled(d)
	if run := queueRun(t, d); run.ID != "" || len(registered(t)) != 1 || queueEvents(d) != "" {
		t.Errorf("a refused approval started something: %+v, %+v, %q", run, registered(t), queueEvents(d))
	}
	// A plan that changed since it was shown is refused too, and a plan that isn't there.
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", `{"id":"`+plan.ID+`","rev":9}`); code != http.StatusConflict {
		t.Errorf("a stale rev: %d", code)
	}
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", `{"id":"nope","rev":1}`); code != http.StatusConflict {
		t.Errorf("another plan: %d", code)
	}
	settled(d)
	if len(registered(t)) != 1 {
		t.Errorf("a refused approval hired: %+v", registered(t))
	}
}

// The queue through the real hire path: the GM first, then each role, fenced, one at a time.
func TestApprovingAPlanHiresTheGMThenEachRoleThroughHireWith(t *testing.T) {
	d, r, h, _ := queueDeckOf(t)
	// The GM's own hire is the GM route's tests': here it is a stand-in.
	d.hq.hireGM = func(hireStep) (int, string) { return http.StatusNoContent, "" }
	_, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"solo-builder"}`)
	plan := planOf(t, text)
	if code, why := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", `{"id":"`+plan.ID+`","rev":1}`); code != http.StatusNoContent {
		t.Fatalf("approve: %d %q", code, why)
	}
	settled(d)
	run := queueRun(t, d)
	if run.State != hqDone || run.Approval.By != "page" {
		t.Fatalf("run %q %q (%+v)", run.State, run.Word, run.Steps)
	}
	var made []string
	for _, hire := range registered(t) {
		made = append(made, hire.Role)
	}
	if !slices.Equal(made, []string{"builder", "reviewer", "tester"}) {
		t.Errorf("hired %q, want the roles in order (the GM's own hire is a stand-in here)", made)
	}
	// Each in a worktree of its own, opened in a window of the fenced way, never a plain terminal.
	if !r.tmux.ran("new-session") || len(h.windows) != 3 {
		t.Errorf("tmux %q, %d windows", r.tmux.asked, len(h.windows))
	}
	for _, line := range []string{"Approved: Solo builder, 4 to hire", "Hired Builder (2 of 4)", "Done: Solo builder, 4 of 4 hired"} {
		if !strings.Contains(queueEvents(d), line) {
			t.Errorf("the events lack %q: %q", line, queueEvents(d))
		}
	}
	// The plan is approved, and what the page reads says so.
	if code, text := queueCall(t, d, asToolCaller, "", "GET", "/setup/hire/queue", ""); code != http.StatusOK || !strings.Contains(text, `"state": "done"`) && !strings.Contains(text, `"state":"done"`) {
		t.Errorf("GET queue: %d %q", code, text)
	}
}

// The gates hireWith has hold for each step: no folder set up, and hiring off, each stop the queue at its first step with
// the deck's own sentence, and nothing is made.
func TestEveryGateOfHireWithStillHoldsForAQueuedHire(t *testing.T) {
	for _, c := range []struct {
		name    string
		breakIt func(d *Deck)
		want    string
	}{
		{"no folder for hires", func(d *Deck) { d.config.HireFolder = "" }, setupHireNeedsFolder},
		{"hiring off", func(d *Deck) { d.config.HiringEnabled = false }, hiringOff},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, r, h, _ := queueDeckOf(t)
			// With the GM up, the first step is a role's. The folder goes before the plan is made, as it would have
			// before the owner picked the company (a plan names the folder it was made with).
			writeRegistry(t, Hire{ID: "g1", Name: "Boss", Folder: "/w", Role: "gm", State: "working"})
			d.mu.Lock()
			c.breakIt(d)
			d.mu.Unlock()
			_, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"bug-squash","planner":"own"}`)
			plan := planOf(t, text)
			if code, why := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", `{"id":"`+plan.ID+`","rev":1}`); code != http.StatusNoContent {
				t.Fatalf("approve: %d %q", code, why)
			}
			settled(d)
			run := queueRun(t, d)
			if run.State != hqStopped || !strings.Contains(run.Word, c.want) || run.Steps[0].State != hqRefused || run.Steps[1].State != hqWaiting {
				t.Errorf("run %q %q %q", run.State, run.Word, stepStates(run))
			}
			if got := registered(t); len(got) != 1 || r.tmux.ran("new-session") || len(h.windows) != 0 {
				t.Errorf("a refused queue made something: %+v, tmux %q, windows %q", got, r.tmux.asked, h.windows)
			}
			if !strings.Contains(queueEvents(d), "Stopped at Bug hunter (1 of 3)") {
				t.Errorf("events %q", queueEvents(d))
			}
		})
	}
}

// With a GM up, it is asked, and the owner sees the plan only once it proposes; only the owner approves it.
func TestAskedForAPlanTheGMProposesAndTheOwnerApproves(t *testing.T) {
	d, _, _, token := queueDeckOf(t)
	writeRegistry(t, Hire{ID: "g1", Name: "Boss", Folder: "/w", Role: "gm", State: "working"})
	code, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"small-agency"}`)
	plan := planOf(t, text)
	if code != http.StatusOK || plan.Status != hpAsked || plan.By != gmRole || len(plan.Steps) != 0 {
		t.Fatalf("%d %+v", code, plan)
	}
	asked := false
	for _, e := range d.events.snapshot() {
		asked = asked || e.Kind == eventKindMail && strings.Contains(e.Detail, "hire plan") && strings.Contains(e.Detail, plan.ID) && strings.Contains(e.Detail, "designer")
	}
	if !asked {
		t.Errorf("the GM was not asked in its inbox")
	}
	approve := `{"id":"` + plan.ID + `","rev":1}`
	if code, why := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", approve); code != http.StatusConflict || why != hirePlanGMDown {
		t.Errorf("approving before the GM proposed: %d %q", code, why)
	}
	// A plan that hires other roles than the template's, or a folder that's no good, is refused.
	home := d.config.HireFolder // a folder inside the home folder, never the home folder itself
	for _, body := range []string{
		`{"id":"` + plan.ID + `","steps":[{"role":"builder"},{"role":"reviewer"}]}`,
		`{"id":"` + plan.ID + `","steps":[{"role":"builder","folder":"/"},{"role":"builder"},{"role":"designer"},{"role":"reviewer"},{"role":"docs"}]}`,
		`{"id":"nope","steps":[]}`,
	} {
		if code, _ := queueCall(t, d, asGMCaller, token, "POST", "/setup/hire/plan/propose", body); code == http.StatusOK {
			t.Errorf("a bad proposal was taken: %s", body)
		}
	}
	// A tool alone can't propose.
	if code, _ := queueCall(t, d, asToolCaller, "", "POST", "/setup/hire/plan/propose", `{"id":"`+plan.ID+`","steps":[]}`); code != http.StatusForbidden {
		t.Errorf("a tool proposed: %d", code)
	}
	proposal := `{"id":"` + plan.ID + `","steps":[{"role":"builder","folder":"` + home + `","brief":"Take the API."},{"role":"builder","brief":"Take the app."},` +
		`{"role":"designer","brief":"Draw the screens."},{"role":"reviewer","brief":"Read each branch."},{"role":"docs","brief":"Keep the README true."}]}`
	code, text = queueCall(t, d, asGMCaller, token, "POST", "/setup/hire/plan/propose", proposal)
	// With a GM up the plan has no GM step: its roles are the template's, and the proposal above lists them.
	proposed := planOf(t, text)
	if code != http.StatusOK || proposed.Status != hpProposed || proposed.By != gmRole || proposed.Rev != 2 || proposed.Steps[0].Brief != "Take the API." || !strings.HasSuffix(proposed.Steps[0].Folder, "/proj") || proposed.Steps[1].Folder == "" {
		t.Fatalf("proposal: %d %+v", code, proposed)
	}
	// The old rev is the plan the owner was shown before it changed.
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", approve); code != http.StatusConflict {
		t.Errorf("approving the old rev: %d", code)
	}
	// The GM can't approve its own plan; the page can.
	if code, _ := queueCall(t, d, asGMCaller, token, "POST", "/setup/hire/plan/approve", `{"id":"`+plan.ID+`","rev":2}`); code != http.StatusForbidden {
		t.Errorf("the GM approved its own plan: %d", code)
	}
	if run := queueRun(t, d); run.ID != "" {
		t.Fatalf("a queue started before the owner approved: %+v", run)
	}
	if code, why := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", `{"id":"`+plan.ID+`","rev":2}`); code != http.StatusNoContent {
		t.Fatalf("approve: %d %q", code, why)
	}
	settled(d)
	if run := queueRun(t, d); run.Steps[0].Role != "builder" || run.Steps[0].Brief != "Take the API." {
		t.Errorf("the queue hires what the GM proposed: %+v", run.Steps)
	}
}

func TestTheOwnerCanCancelAPlanAndAWaitingQueueFromThePage(t *testing.T) {
	d, _, _, _ := queueDeckOf(t)
	_, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"solo-builder"}`)
	plan := planOf(t, text)
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/cancel", "{}"); code != http.StatusNoContent {
		t.Errorf("cancel plan: %d", code)
	}
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan/approve", `{"id":"`+plan.ID+`","rev":1}`); code != http.StatusConflict {
		t.Errorf("approving a cancelled plan: %d", code)
	}
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/queue/cancel", "{}"); code != http.StatusConflict {
		t.Errorf("cancelling no queue: %d", code)
	}
	if code, _ := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/queue/resume", "{}"); code != http.StatusConflict {
		t.Errorf("resuming no queue: %d", code)
	}
}

// The route table: a tool, a hire and the GM are each refused the rows only the page may use, before any handler runs.
func TestTheQueueRoutesAreThePagesAloneExceptTheGMsProposalAndTheReading(t *testing.T) {
	for _, c := range []struct {
		method, path string
		allowed      []caller
	}{
		{"GET", "/setup/hire/queue", []caller{callerPage, callerGM, callerTool}},
		{"POST", "/setup/hire/plan", []caller{callerPage}},
		{"POST", "/setup/hire/plan/propose", []caller{callerPage, callerGM}},
		{"POST", "/setup/hire/plan/approve", []caller{callerPage}},
		{"POST", "/setup/hire/plan/cancel", []caller{callerPage}},
		{"POST", "/setup/hire/queue/resume", []caller{callerPage}},
		{"POST", "/setup/hire/queue/cancel", []caller{callerPage}},
	} {
		for _, who := range []caller{callerNobody, callerTool, callerGM, callerHire, callerPage} {
			if got := routeAllows(who, c.method, c.path); got != slices.Contains(c.allowed, who) {
				t.Errorf("%s %s for %s: %v", c.method, c.path, who.name(), got)
			}
		}
	}
}

// A restart of the deck reads the queue the last one kept: a plan waiting is still there, a running queue is paused.
func TestADeckStartedAgainFindsItsQueuePausedWithAWord(t *testing.T) {
	d, _, _, _ := queueDeckOf(t)
	_, text := queueCall(t, d, asPageCaller, "", "POST", "/setup/hire/plan", `{"template":"solo-builder"}`)
	plan := planOf(t, text)
	d.hq.mu.Lock()
	d.hq.run = &hireQueueRun{ID: "R1", Label: "Solo builder", State: hqRunning, Saved: time.Now().UnixMilli(),
		Steps: []hireStep{{Role: "builder", State: hqHired}, {Role: "reviewer", State: hqHiring}, {Role: "tester", State: hqWaiting}}}
	d.hq.keep()
	d.hq.mu.Unlock()
	again := d.newHireQueue()
	got := again.snapshot()
	if p, _ := got["plan"].(hirePlan); p.ID != plan.ID {
		t.Errorf("the plan after a restart: %+v", got["plan"])
	}
	run, _ := got["run"].(hireQueueRun)
	if run.State != hqPaused || !slices.Equal(stepStates(run), []string{hqHired, hqUnsure, hqWaiting}) || !strings.Contains(queueEvents(d), "restarted") {
		t.Errorf("run %q %q, events %q", run.State, stepStates(run), queueEvents(d))
	}
}
