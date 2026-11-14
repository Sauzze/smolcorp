package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The hire queue on stand-ins (hirequeue.go): no deck, no window, no agent. The deck's own gates are tried with
// the real hire path in hirequeue_deck_test.go.

// queueRig is a queue whose hires, clock and waits are stand-ins that write down what was asked.
type queueRig struct {
	q      *hireQueue
	file   string
	mu     sync.Mutex
	calls  []string        // "gm" or the role, in the order they were hired
	said   []string        // the event lines
	slept  []time.Duration // every wait asked for
	answer func(call int, s hireStep) (int, string)
	now    time.Time
	hired  func(hireStep, float64) bool
}

func newQueueRig(t *testing.T) *queueRig {
	t.Helper()
	r := &queueRig{file: filepath.Join(t.TempDir(), "hire-queue.json"), now: time.UnixMilli(1_800_000_000_000)}
	r.answer = func(int, hireStep) (int, string) { return http.StatusNoContent, "" }
	r.q = r.build()
	return r
}

// build is a queue on the rig's file, as a start of the app makes one: what an earlier queue kept is read again.
func (r *queueRig) build() *hireQueue {
	do := func(s hireStep) (int, string) {
		r.mu.Lock()
		r.calls = append(r.calls, s.Role)
		n, answer := len(r.calls), r.answer
		r.mu.Unlock()
		return answer(n, s)
	}
	return newHireQueue(r.file, hireQueueDeps{
		hireGM: do, hireRole: do,
		hired: func(s hireStep, since float64) bool { return r.hired != nil && r.hired(s, since) },
		say: func(text string) {
			r.mu.Lock()
			r.said = append(r.said, text)
			r.mu.Unlock()
		},
		sleep: func(d time.Duration, stop <-chan struct{}) bool {
			r.mu.Lock()
			r.slept = append(r.slept, d)
			r.mu.Unlock()
			select {
			case <-stop:
				return false
			default:
				return true
			}
		},
		now: func() time.Time { r.mu.Lock(); defer r.mu.Unlock(); return r.now },
	})
}

func (r *queueRig) roles() []string { r.mu.Lock(); defer r.mu.Unlock(); return slices.Clone(r.calls) }
func (r *queueRig) lines() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.said, "\n")
}

func (r *queueRig) run(t *testing.T) hireQueueRun {
	t.Helper()
	run, ok := r.q.snapshot()["run"].(hireQueueRun)
	if !ok {
		t.Fatal("no queue")
	}
	return run
}

func steps(roles ...string) []hireStep {
	var out []hireStep
	for _, role := range roles {
		out = append(out, hireStep{Role: role, Name: strings.ToUpper(role[:1]) + role[1:], Brief: "SECRET-BRIEF-LINE for " + role})
	}
	return out
}

func stepStates(run hireQueueRun) []string {
	var states []string
	for _, s := range run.Steps {
		states = append(states, s.State)
	}
	return states
}

func TestNothingIsHiredWithoutThePagesApproval(t *testing.T) {
	// The GM's token, an MCP key, a night job, a hire, a tool, nobody and an MCP key with no name: all refused,
	// before anything is hired, kept or said.
	for _, by := range []actor{actorGM, actorHire, actorTool, actorNobody, mcpActor("k1"), mcpActor(""), nightActor("nightly"), actor("")} {
		r := newQueueRig(t)
		_, err := r.q.start(steps("gm", "builder"), "Solo builder", by)
		if refusalStatus(err) != http.StatusForbidden || err == nil {
			t.Errorf("%q approved a queue: %v", by, err)
		}
		if err := r.q.resume(by); err == nil || refusalStatus(err) != http.StatusForbidden {
			t.Errorf("%q resumed: %v", by, err)
		}
		if err := r.q.cancel(by); err == nil || refusalStatus(err) != http.StatusForbidden {
			t.Errorf("%q cancelled: %v", by, err)
		}
		r.q.wait()
		if len(r.roles()) != 0 || r.q.snapshot()["run"] != nil || r.lines() != "" {
			t.Errorf("%q: something was hired, kept or said: %q %q", by, r.roles(), r.lines())
		}
		if _, err := os.Stat(r.file); err == nil {
			t.Errorf("%q: a refused approval was kept in a file", by)
		}
	}
}

func TestTheQueueHiresTheGMFirstThenEachRoleInOrderWithTheCooldownBetween(t *testing.T) {
	r := newQueueRig(t)
	if _, err := r.q.start(steps("gm", "builder", "builder", "reviewer"), "Solo builder", actorPage); err != nil {
		t.Fatal(err)
	}
	r.q.wait()
	if got := r.roles(); !slices.Equal(got, []string{"gm", "builder", "builder", "reviewer"}) {
		t.Errorf("hired %q, want the GM first, then the roles in order", got)
	}
	run := r.run(t)
	if run.State != hqDone || run.Approval.By != "page" || run.Approval.At != r.now.UnixMilli() {
		t.Errorf("run %+v, want done, approved by the page", run)
	}
	if got := stepStates(run); !slices.Equal(got, []string{hqHired, hqHired, hqHired, hqHired}) {
		t.Errorf("steps %q", got)
	}
	// One cooldown between two hires: three waits for four hires, none before the first.
	if want := []time.Duration{hireCooldown, hireCooldown, hireCooldown}; !slices.Equal(r.slept, want) {
		t.Errorf("waits %v, want %v", r.slept, want)
	}
}

func TestTheGMIsHiredThroughTheGMsWayAndRolesThroughTheRolesWay(t *testing.T) {
	var way []string
	r := newQueueRig(t)
	rig := r.q
	rig.hireGM = func(s hireStep) (int, string) { way = append(way, "gm:"+s.Role); return http.StatusNoContent, "" }
	rig.hireRole = func(s hireStep) (int, string) { way = append(way, "role:"+s.Role); return http.StatusNoContent, "" }
	if _, err := rig.start(steps("gm", "tester"), "Bug squash", actorPage); err != nil {
		t.Fatal(err)
	}
	rig.wait()
	if !slices.Equal(way, []string{"gm:gm", "role:tester"}) {
		t.Errorf("ways %q", way)
	}
}

func TestAHireTurnedAwayAsTooSoonIsTriedAgainAndThenStops(t *testing.T) {
	r := newQueueRig(t)
	r.answer = func(n int, _ hireStep) (int, string) {
		if n <= 2 {
			return http.StatusTooManyRequests, "Hired a moment ago. Try again in a few seconds."
		}
		return http.StatusNoContent, ""
	}
	r.q.start(steps("builder"), "Solo builder", actorPage)
	r.q.wait()
	if run := r.run(t); run.State != hqDone || len(r.roles()) != 3 || !slices.Equal(r.slept, []time.Duration{hireQueueRetry, hireQueueRetry}) {
		t.Errorf("state %q after %d tries, waits %v", run.State, len(r.roles()), r.slept)
	}

	r = newQueueRig(t)
	r.answer = func(int, hireStep) (int, string) {
		return http.StatusTooManyRequests, "Hired a moment ago. Try again in a few seconds."
	}
	r.q.start(steps("builder", "tester"), "Solo builder", actorPage)
	r.q.wait()
	run := r.run(t)
	if run.State != hqStopped || !strings.Contains(run.Word, "Hired a moment ago") || len(r.roles()) != hireQueueRetries+1 {
		t.Errorf("state %q, word %q, %d tries", run.State, run.Word, len(r.roles()))
	}
}

func TestTheQueueStopsAtTheFirstRefusalWithItsSentence(t *testing.T) {
	r := newQueueRig(t)
	const sentence = "Hires need tmux on your Mac to start fenced: install it, then hire again."
	r.answer = func(n int, s hireStep) (int, string) {
		if s.Role == "reviewer" {
			return http.StatusConflict, sentence
		}
		return http.StatusNoContent, ""
	}
	r.q.start(steps("gm", "builder", "reviewer", "tester", "docs"), "Small agency", actorPage)
	r.q.wait()
	run := r.run(t)
	if run.State != hqStopped || !strings.Contains(run.Word, sentence) || !strings.Contains(run.Word, "Reviewer") || !strings.Contains(run.Word, "3 of 5") {
		t.Errorf("state %q, word %q", run.State, run.Word)
	}
	if got := stepStates(run); !slices.Equal(got, []string{hqHired, hqHired, hqRefused, hqWaiting, hqWaiting}) || run.Steps[2].Why != sentence {
		t.Errorf("steps %q, why %q", got, run.Steps[2].Why)
	}
	if got := r.roles(); !slices.Equal(got, []string{"gm", "builder", "reviewer"}) {
		t.Errorf("hired %q: nothing after the refusal may be tried", got)
	}
	if !strings.Contains(r.lines(), "Stopped at Reviewer") {
		t.Errorf("events say %q", r.lines())
	}
	// A second queue can't start over a stopped one, and resuming tries the refused step again, then goes on.
	if _, err := r.q.start(steps("builder"), "Another", actorPage); refusalStatus(err) != http.StatusConflict {
		t.Errorf("a second queue over a stopped one: %v", err)
	}
	r.answer = func(int, hireStep) (int, string) { return http.StatusNoContent, "" }
	if err := r.q.resume(actorPage); err != nil {
		t.Fatal(err)
	}
	r.q.wait()
	if got, run := r.roles(), r.run(t); run.State != hqDone || !slices.Equal(got, []string{"gm", "builder", "reviewer", "reviewer", "tester", "docs"}) {
		t.Errorf("after resume: %q, %q", run.State, got)
	}
}

func TestTheOwnerCancelsAQueueAndWhatIsBeingHiredIsMadeButNothingAfterIt(t *testing.T) {
	r := newQueueRig(t)
	inside, release := make(chan struct{}), make(chan struct{})
	r.answer = func(n int, _ hireStep) (int, string) {
		if n == 2 {
			close(inside)
			<-release
		}
		return http.StatusNoContent, ""
	}
	r.q.start(steps("gm", "builder", "reviewer", "tester"), "Solo builder", actorPage)
	<-inside
	if err := r.q.cancel(actorPage); err != nil {
		t.Fatal(err)
	}
	if err := r.q.cancel(actorPage); err != nil { // twice is fine
		t.Fatal(err)
	}
	close(release)
	r.q.wait()
	run := r.run(t)
	if run.State != hqCancelled || !strings.Contains(run.Word, "2 of 4 hired") {
		t.Errorf("state %q, word %q", run.State, run.Word)
	}
	if got := stepStates(run); !slices.Equal(got, []string{hqHired, hqHired, hqCancelled, hqCancelled}) {
		t.Errorf("steps %q", got)
	}
	if got := r.roles(); !slices.Equal(got, []string{"gm", "builder"}) {
		t.Errorf("hired %q", got)
	}
	// Nothing is left to cancel, and a new queue may begin.
	if err := r.q.cancel(actorPage); refusalStatus(err) != http.StatusConflict {
		t.Errorf("cancelling a cancelled queue: %v", err)
	}
	if _, err := r.q.start(steps("tester"), "Again", actorPage); err != nil {
		t.Errorf("a new queue after a cancelled one: %v", err)
	}
	r.q.wait()
}

func TestAStoppedQueueIsCancelledAtOnce(t *testing.T) {
	r := newQueueRig(t)
	r.answer = func(int, hireStep) (int, string) { return http.StatusConflict, "No." }
	r.q.start(steps("builder", "tester"), "Solo builder", actorPage)
	r.q.wait()
	if err := r.q.cancel(actorPage); err != nil {
		t.Fatal(err)
	}
	if run := r.run(t); run.State != hqCancelled || !slices.Equal(stepStates(run), []string{hqRefused, hqCancelled}) {
		t.Errorf("run %+v", run)
	}
}

func TestAQueueRunningAtARestartIsPausedWithAWordAndResumedByTheOwner(t *testing.T) {
	r := newQueueRig(t)
	inside, release := make(chan struct{}), make(chan struct{})
	r.answer = func(n int, _ hireStep) (int, string) {
		if n == 2 {
			close(inside)
			<-release
		}
		return http.StatusNoContent, ""
	}
	first := r.q
	first.start(steps("gm", "builder", "reviewer"), "Solo builder", actorPage)
	<-inside
	// "Restart": a queue built again on the same file while the first is in the middle of the second hire.
	r.answer = func(int, hireStep) (int, string) { return http.StatusNoContent, "" }
	second := r.build()
	run := func() hireQueueRun { return second.snapshot()["run"].(hireQueueRun) }()
	if run.State != hqPaused || !slices.Equal(stepStates(run), []string{hqHired, hqUnsure, hqWaiting}) {
		t.Fatalf("after the restart: %q %q", run.State, stepStates(run))
	}
	if !strings.Contains(run.Word, "restarted") || !strings.Contains(run.Word, "Resume it, or cancel it") || !strings.Contains(r.lines(), "restarted") {
		t.Errorf("the restart said %q (events %q)", run.Word, r.lines())
	}
	// Nothing goes on by itself.
	second.wait()
	if got := r.roles(); !slices.Equal(got, []string{"gm", "builder"}) {
		t.Errorf("hired %q before anyone resumed", got)
	}
	// Only the owner resumes it.
	if err := second.resume(actorGM); refusalStatus(err) != http.StatusForbidden {
		t.Errorf("the GM resumed: %v", err)
	}
	// The hire under way when the app stopped is in the office: it isn't hired twice.
	r.hired = func(s hireStep, since float64) bool { return s.Role == "builder" && since > 0 }
	if err := second.resume(actorPage); err != nil {
		t.Fatal(err)
	}
	second.wait()
	if got := r.roles(); !slices.Equal(got, []string{"gm", "builder", "reviewer"}) {
		t.Errorf("hired %q: the builder was already there", got)
	}
	if run := second.snapshot()["run"].(hireQueueRun); run.State != hqDone || !slices.Equal(stepStates(run), []string{hqHired, hqHired, hqHired}) {
		t.Errorf("after resume: %+v", run)
	}
	close(release)
	first.cancel(actorPage)
	first.wait()
}

func TestAHireNotFoundAfterARestartIsTriedAgain(t *testing.T) {
	r := newQueueRig(t)
	inside, release := make(chan struct{}), make(chan struct{})
	r.answer = func(int, hireStep) (int, string) { close(inside); <-release; return http.StatusNoContent, "" }
	r.q.start(steps("builder"), "Solo builder", actorPage)
	<-inside
	r.answer = func(int, hireStep) (int, string) { return http.StatusNoContent, "" }
	second := r.build()
	if err := second.resume(actorPage); err != nil {
		t.Fatal(err)
	}
	second.wait()
	if got := r.roles(); !slices.Equal(got, []string{"builder", "builder"}) {
		t.Errorf("hired %q, want the builder tried again", got)
	}
	close(release)
	r.q.wait()
}

func TestAQueueLeftWaitingTooLongIsLetGoWithAWord(t *testing.T) {
	r := newQueueRig(t)
	r.answer = func(int, hireStep) (int, string) { return http.StatusConflict, "No." }
	r.q.start(steps("builder", "tester"), "Solo builder", actorPage)
	r.q.wait()
	r.mu.Lock()
	r.now = r.now.Add(hireQueueStale + time.Minute)
	r.mu.Unlock()
	second := r.build()
	run := second.snapshot()["run"].(hireQueueRun)
	if run.State != hqAbandoned || !strings.Contains(run.Word, "Left alone") || !strings.Contains(run.Word, "12 hours") || !strings.Contains(r.lines(), "Left alone") {
		t.Errorf("run %q %q", run.State, run.Word)
	}
	if err := second.resume(actorPage); refusalStatus(err) != http.StatusConflict {
		t.Errorf("an abandoned queue was resumed: %v", err)
	}
	// A queue that finished isn't touched by a restart.
	done := newQueueRig(t)
	done.q.start(steps("builder"), "Solo builder", actorPage)
	done.q.wait()
	done.mu.Lock()
	done.now = done.now.Add(100 * time.Hour)
	done.mu.Unlock()
	if run := done.build().snapshot()["run"].(hireQueueRun); run.State != hqDone {
		t.Errorf("a finished queue became %q", run.State)
	}
}

func TestTheQueueIsKeptInAPrivateFileAndAPlanSurvivesToo(t *testing.T) {
	r := newQueueRig(t)
	r.q.mu.Lock()
	r.q.plan = &hirePlan{ID: "P1", Template: "solo-builder", Name: "Solo builder", Status: hpProposed, Rev: 3, Steps: steps("builder")}
	r.q.keep()
	r.q.mu.Unlock()
	r.q.start(steps("builder"), "Solo builder", actorPage)
	r.q.wait()
	info, err := os.Stat(r.file)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file %v, %v", info, err)
	}
	again := r.build().snapshot()
	if plan, ok := again["plan"].(hirePlan); !ok || plan.ID != "P1" || plan.Rev != 3 {
		t.Errorf("the plan after a restart: %+v", again["plan"])
	}
}

func TestTheQueueTakesOneListAtATimeAndOnlyThoseThatCanBeHired(t *testing.T) {
	r := newQueueRig(t)
	if _, err := r.q.start(steps("builder", "gm"), "x", actorPage); refusalStatus(err) != http.StatusBadRequest {
		t.Errorf("the GM not first: %v", err)
	}
	if _, err := r.q.start(nil, "x", actorPage); refusalStatus(err) != http.StatusBadRequest {
		t.Errorf("no steps: %v", err)
	}
	if _, err := r.q.start(make([]hireStep, hireQueueMost+1), "x", actorPage); refusalStatus(err) != http.StatusBadRequest {
		t.Errorf("too many: %v", err)
	}
	if _, err := r.q.start([]hireStep{{Role: " "}}, "x", actorPage); refusalStatus(err) != http.StatusBadRequest {
		t.Errorf("no role: %v", err)
	}
	r.q.check = func(s hireStep) string {
		if s.Role == "ghost" {
			return "That role isn't there any more. Pick another."
		}
		return ""
	}
	if _, err := r.q.start(steps("builder", "ghost"), "x", actorPage); err == nil || err.Error() != "That role isn't there any more. Pick another." {
		t.Errorf("an unknown role: %v", err)
	}
	if len(r.roles()) != 0 || r.q.snapshot()["run"] != nil {
		t.Errorf("a refused list made a queue or a hire")
	}
	// One at a time.
	inside, release := make(chan struct{}), make(chan struct{})
	r.answer = func(int, hireStep) (int, string) { close(inside); <-release; return http.StatusNoContent, "" }
	if _, err := r.q.start(steps("builder"), "First", actorPage); err != nil {
		t.Fatal(err)
	}
	<-inside
	if _, err := r.q.start(steps("tester"), "Second", actorPage); refusalStatus(err) != http.StatusConflict {
		t.Errorf("a second queue while one runs: %v", err)
	}
	close(release)
	r.q.wait()
}

func TestTheProgressLinesNameWhoIsHiredAndNeverSayABrief(t *testing.T) {
	r := newQueueRig(t)
	r.q.start(steps("gm", "builder"), "Solo builder", actorPage)
	r.q.wait()
	lines := r.lines()
	for _, want := range []string{"Approved: Solo builder, 2 to hire", "Hiring Gm (1 of 2)", "Hired Gm (1 of 2)", "Hiring Builder (2 of 2)", "Hired Builder (2 of 2)", "Done: Solo builder, 2 of 2 hired"} {
		if !strings.Contains(lines, want) {
			t.Errorf("the progress lines %q lack %q", lines, want)
		}
	}
	if strings.Contains(lines, "SECRET-BRIEF-LINE") {
		t.Errorf("a brief is in the progress lines: %q", lines)
	}
}
