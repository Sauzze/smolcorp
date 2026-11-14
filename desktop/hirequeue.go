package desktop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The hire queue (1.26's K2): a list of hires made one at a time, after the owner has approved them. It is what
// "Pick your company" hires a template with (the GM plans, hirequeue's plan; the owner approves, from the page),
// and what the night shift (N2) reuses: a list of hire steps, and who approved them.
//
//   - Nothing is hired before an approval, and an approval comes from the page alone (approvedBy is an actor,
//     and only the page's counts): never the GM's token, an MCP key, a night job or a hire.
//   - The GM first (Start here's own hire, with gmRefusal), then each role through hireWith as the GM route
//     makes it, fenced, in its folder, with the hire cooldown between them.
//   - It stops at the first refusal, and says that refusal's own sentence. The owner may resume it (the same
//     step is tried again) or cancel it.
//   - Each step is written to hire-queue.json in the support folder before and after it is tried, and its
//     progress is a line in the event log (eventKindHireQueue). After a restart a queue that was running is
//     paused, with a word, and one that waited longer than hireQueueStale is abandoned, with a word: never left
//     half-silent. A step that was being hired at the restart is looked for in the office before it is tried again.
//
// An event line says only who is being hired and how far along the queue is, or why it stopped: never a brief.

const (
	// eventKindHireQueue is a progress line of the hire queue.
	eventKindHireQueue = "hirequeue"
	// The most hires one queue makes.
	hireQueueMost = 12
	// A queue that waited this long (paused, or stopped at a refusal) is let go at the next start.
	hireQueueStale = 12 * time.Hour
	// A hire the deck turns away as too soon (its cooldown, or another hire under way) is tried again this
	// many times, this far apart, before the queue stops with the deck's own sentence.
	hireQueueRetries = 60
	hireQueueRetry   = time.Second
)

// What a queue, and each step of it, is doing.
const (
	hqRunning   = "running"
	hqPaused    = "paused" // a restart interrupted it: resume it or cancel it
	hqStopped   = "stopped"
	hqDone      = "done"
	hqCancelled = "cancelled"
	hqAbandoned = "abandoned"

	hqWaiting = "waiting"
	hqHiring  = "hiring"
	hqHired   = "hired"
	hqRefused = "refused"
	hqUnsure  = "unsure" // being hired when the app stopped
)

// hireStep is one hire in a queue: a role, the folder it works in ("" for the folder for new hires), and a line
// for its brief (the task the hire starts on). Its State, Why and Started are the queue's own.
type hireStep struct {
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`
	Folder  string `json:"folder,omitempty"`
	Brief   string `json:"brief,omitempty"`
	Account string `json:"account,omitempty"`
	Floor   int    `json:"floor,omitempty"`

	State   string  `json:"state,omitempty"`
	Why     string  `json:"why,omitempty"`
	Started float64 `json:"started,omitempty"` // when it began, in milliseconds, so a restart can look for its hire
}

// shown is the step as the owner reads it: its name, or its role.
func (s hireStep) shown() string {
	if s.Name != "" {
		return s.Name
	}
	return s.Role
}

// hireApproval is who said yes to a queue and when: the page, from the owner's own click.
type hireApproval struct {
	By string `json:"by"`
	At int64  `json:"at"`
}

// hireQueueRun is one queue: its steps, its approval and where it stands. Word is the sentence the owner is
// told when it stops, pauses, ends or is let go.
type hireQueueRun struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Approval hireApproval `json:"approval"`
	Steps    []hireStep   `json:"steps"`
	State    string       `json:"state"`
	Word     string       `json:"word,omitempty"`
	Saved    int64        `json:"saved"` // when it was last written, in milliseconds
}

// hireQueueFile is what is kept between runs: the queue, and the plan waiting for approval (hireplan.go).
type hireQueueFile struct {
	Run  *hireQueueRun `json:"run,omitempty"`
	Plan *hirePlan     `json:"plan,omitempty"`
}

// hireQueueDeps is what the queue is given to work with, so it can be tried on stand-ins.
type hireQueueDeps struct {
	// hireGM and hireRole make a hire and answer as hireWith does: 204 (or any 2xx) for made, else the status
	// and the sentence. check says why a step can't be hired at all (an unknown role, a folder that's no good).
	hireGM, hireRole func(hireStep) (int, string)
	check            func(hireStep) string
	// hired is whether a hire of the step's role, begun at or after since (milliseconds), is in the office.
	hired func(step hireStep, since float64) bool
	// say writes a progress line to the event log.
	say func(text string)
	// sleep waits, and says false if the queue was stopped meanwhile; now is the clock.
	sleep func(time.Duration, <-chan struct{}) bool
	now   func() time.Time
}

type hireQueue struct {
	mu   sync.Mutex
	file string
	hireQueueDeps
	run    *hireQueueRun
	plan   *hirePlan
	active bool          // a worker is working through the steps
	stop   chan struct{} // closed to cancel; nil when nothing is working
	wg     sync.WaitGroup
}

// sleepUntilStopped is deps.sleep: it waits for d, or until stop is closed (then false).
func sleepUntilStopped(d time.Duration, stop <-chan struct{}) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-stop:
		return false
	}
}

// newHireQueue reads what was kept in file and settles it for a new start: a queue that was running is paused,
// with a word, and one that waited too long is let go.
func newHireQueue(file string, deps hireQueueDeps) *hireQueue {
	if deps.sleep == nil {
		deps.sleep = sleepUntilStopped
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.say == nil {
		deps.say = func(string) {}
	}
	q := &hireQueue{file: file, hireQueueDeps: deps}
	var kept hireQueueFile
	if data, err := os.ReadFile(file); err == nil && json.Unmarshal(data, &kept) == nil {
		q.run, q.plan = kept.Run, kept.Plan
	}
	q.settle()
	return q
}

// settle is what a start makes of a queue the last run left.
func (q *hireQueue) settle() {
	run := q.run
	if run == nil || run.State != hqRunning && run.State != hqPaused && run.State != hqStopped {
		return
	}
	age := q.now().Sub(time.UnixMilli(run.Saved))
	switch {
	case age > hireQueueStale:
		q.end(hqAbandoned, "Left alone: "+run.Label+" waited more than "+strconv.Itoa(int(hireQueueStale.Hours()))+" hours, so nothing more is hired for it. Pick your company again if you still want it.")
	case run.State == hqRunning:
		for i := range run.Steps {
			if run.Steps[i].State == hqHiring {
				run.Steps[i].State = hqUnsure
				run.Steps[i].Why = "Smolcorp stopped while it was being hired: check the office for it."
			}
		}
		run.State = hqPaused
		run.Word = "Smolcorp restarted while " + run.Label + " was being hired (" + q.progress() + "). Resume it, or cancel it."
		q.keep()
		q.say(run.Word)
	}
}

// keep writes the queue and the plan to the file (q.mu held).
func (q *hireQueue) keep() {
	if q.run != nil {
		q.run.Saved = q.now().UnixMilli()
	}
	data, err := json.MarshalIndent(hireQueueFile{Run: q.run, Plan: q.plan}, "", "  ")
	if err == nil {
		_ = writePrivate(q.file, data)
	}
}

// progress is "2 of 4 hired" (q.mu held).
func (q *hireQueue) progress() string {
	done := 0
	for _, s := range q.run.Steps {
		if s.State == hqHired {
			done++
		}
	}
	return fmt.Sprintf("%d of %d hired", done, len(q.run.Steps))
}

// end finishes the queue in state, with a word (q.mu held).
func (q *hireQueue) end(state, word string) {
	run := q.run
	run.State, run.Word = state, word
	if state == hqCancelled || state == hqAbandoned {
		for i := range run.Steps {
			if run.Steps[i].State == hqWaiting {
				run.Steps[i].State = hqCancelled
			}
		}
	}
	q.keep()
	q.say(word)
}

// approvalRefusal is why an approval doesn't count, or nil: only the page's does. The GM's token, an MCP key, a
// night shift job, a hire and a tool are all refused, whatever else they hold.
func approvalRefusal(by actor) error {
	if by.kind() == "page" {
		return nil
	}
	return &hireRefusal{Status: http.StatusForbidden, Message: "Only you can approve a hire plan, from the setup page: the General Manager, a tool, an MCP key or a hire can't."}
}

// start approves steps for label and begins hiring them, or says why not. Nothing is made, or kept, unless the
// approval is the page's and every step is one that can be hired.
func (q *hireQueue) start(steps []hireStep, label string, by actor) (string, error) {
	if err := approvalRefusal(by); err != nil {
		return "", err
	}
	if len(steps) == 0 || len(steps) > hireQueueMost {
		return "", &hireRefusal{Status: http.StatusBadRequest, Message: "A queue hires 1 to " + strconv.Itoa(hireQueueMost) + " people."}
	}
	kept := make([]hireStep, len(steps))
	for i, s := range steps {
		if strings.TrimSpace(s.Role) == "" {
			return "", &hireRefusal{Status: http.StatusBadRequest, Message: "Every step needs a role."}
		}
		if s.Role == gmRole && i != 0 {
			return "", &hireRefusal{Status: http.StatusBadRequest, Message: "The General Manager is hired first."}
		}
		if q.check != nil {
			if why := q.check(s); why != "" {
				return "", &hireRefusal{Status: http.StatusBadRequest, Message: why}
			}
		}
		s.State, s.Why, s.Started = hqWaiting, "", 0
		kept[i] = s
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.run != nil && (q.run.State == hqRunning || q.run.State == hqPaused || q.run.State == hqStopped) {
		return "", &hireRefusal{Status: http.StatusConflict, Message: "A hire queue is already waiting (" + q.run.Label + "): cancel it first, or let it finish."}
	}
	now := q.now()
	q.run = &hireQueueRun{ID: newID(), Label: label, Approval: hireApproval{By: string(by), At: now.UnixMilli()}, Steps: kept, State: hqRunning}
	q.keep()
	q.say("Approved: " + label + ", " + strconv.Itoa(len(kept)) + " to hire, one at a time.")
	q.launch()
	return q.run.ID, nil
}

// resume goes on with a queue that was paused by a restart, or stopped at a refusal: the same step is tried again
// (one that was being hired when the app stopped is looked for in the office first). It is the owner's own tap.
func (q *hireQueue) resume(by actor) error {
	if err := approvalRefusal(by); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	run := q.run
	if run == nil || run.State != hqPaused && run.State != hqStopped || q.active {
		return &hireRefusal{Status: http.StatusConflict, Message: "There's no hire queue waiting to be resumed."}
	}
	for i := range run.Steps {
		step := &run.Steps[i]
		switch step.State {
		case hqUnsure:
			if q.hired != nil && q.hired(*step, step.Started) {
				step.State, step.Why = hqHired, ""
			} else {
				step.State, step.Why = hqWaiting, ""
			}
		case hqRefused:
			step.State, step.Why = hqWaiting, ""
		}
	}
	run.State, run.Word = hqRunning, ""
	q.keep()
	q.say("Resumed: " + run.Label + " (" + q.progress() + ").")
	q.launch()
	return nil
}

// cancel ends the queue: what is being hired at this moment is made, and nothing after it. The owner's own tap.
func (q *hireQueue) cancel(by actor) error {
	if err := approvalRefusal(by); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	run := q.run
	if run == nil || run.State != hqRunning && run.State != hqPaused && run.State != hqStopped {
		return &hireRefusal{Status: http.StatusConflict, Message: "There's no hire queue to cancel."}
	}
	if q.active {
		// The worker ends it, once the hire under way is made.
		if q.stop != nil {
			close(q.stop)
			q.stop = nil
		}
		return nil
	}
	q.end(hqCancelled, "Cancelled: "+run.Label+", "+q.progress()+".")
	return nil
}

// launch starts the worker (q.mu held).
func (q *hireQueue) launch() {
	q.active, q.stop = true, make(chan struct{})
	q.wg.Add(1)
	go q.work(q.run.ID, q.stop)
}

// wait is for the tests: it returns when no worker is working.
func (q *hireQueue) wait() { q.wg.Wait() }

// work hires the steps in order, one at a time.
func (q *hireQueue) work(id string, stop chan struct{}) {
	defer q.wg.Done()
	hiredOne := false
	for {
		q.mu.Lock()
		run := q.run
		if run == nil || run.ID != id || run.State != hqRunning {
			q.active = false
			q.mu.Unlock()
			return
		}
		cancelled := false
		select {
		case <-stop:
			cancelled = true
		default:
		}
		next := -1
		for i := range run.Steps {
			if run.Steps[i].State == hqWaiting {
				next = i
				break
			}
		}
		if cancelled {
			q.end(hqCancelled, "Cancelled: "+run.Label+", "+q.progress()+".")
			q.active = false
			q.mu.Unlock()
			return
		}
		if next < 0 {
			q.end(hqDone, "Done: "+run.Label+", "+q.progress()+".")
			q.active = false
			q.mu.Unlock()
			return
		}
		q.mu.Unlock()
		// The cooldown between one hire and the next: a second hire too soon is turned away by the deck anyway.
		if hiredOne && !q.sleep(hireCooldown, stop) {
			continue // cancelled: the top of the loop says so
		}
		q.mu.Lock()
		step := &run.Steps[next]
		step.State, step.Started = hqHiring, float64(q.now().UnixMilli())
		q.keep()
		q.say("Hiring " + step.shown() + " (" + strconv.Itoa(next+1) + " of " + strconv.Itoa(len(run.Steps)) + ").")
		snapshot := *step
		q.mu.Unlock()

		status, text := q.make(snapshot, stop)

		q.mu.Lock()
		step = &run.Steps[next]
		if status == 0 {
			// Cancelled while it waited to try again: nothing was made, and the top of the loop says so.
			step.State = hqWaiting
			q.keep()
			q.mu.Unlock()
			continue
		}
		if status >= http.StatusOK && status < http.StatusMultipleChoices {
			step.State, step.Why = hqHired, ""
			hiredOne = true
			q.keep()
			q.say("Hired " + step.shown() + " (" + strconv.Itoa(next+1) + " of " + strconv.Itoa(len(run.Steps)) + ").")
			q.mu.Unlock()
			continue
		}
		step.State, step.Why = hqRefused, text
		q.end(hqStopped, "Stopped at "+step.shown()+" ("+strconv.Itoa(next+1)+" of "+strconv.Itoa(len(run.Steps))+"): "+text)
		q.active = false
		q.mu.Unlock()
		return
	}
}

// make is one hire, through the GM's way or a role's; a hire turned away as too soon is tried again a while.
// Its status is 0 if the queue was cancelled while it waited to try again.
func (q *hireQueue) make(step hireStep, stop <-chan struct{}) (int, string) {
	hire := q.hireRole
	if step.Role == gmRole {
		hire = q.hireGM
	}
	status, text := hire(step)
	for tries := 0; status == http.StatusTooManyRequests && tries < hireQueueRetries; tries++ {
		if !q.sleep(hireQueueRetry, stop) {
			return 0, ""
		}
		status, text = hire(step)
	}
	return status, text
}

// snapshot is the queue and the plan as the page reads them.
func (q *hireQueue) snapshot() map[string]any {
	q.mu.Lock()
	defer q.mu.Unlock()
	answer := map[string]any{"run": nil, "plan": nil}
	if q.run != nil {
		copied := *q.run
		copied.Steps = append([]hireStep(nil), q.run.Steps...)
		answer["run"] = copied
	}
	if q.plan != nil {
		copied := *q.plan
		copied.Steps = append([]hireStep(nil), q.plan.Steps...)
		answer["plan"] = copied
	}
	return answer
}

// newHireQueue is the deck's queue, on its own file in the support folder, hiring as Setup's GM hire and the GM
// route's hire do, so every gate of hireWith holds for each step.
func (d *Deck) newHireQueue() *hireQueue {
	return newHireQueue(filepath.Join(supportDir, "hire-queue.json"), hireQueueDeps{
		hireGM:   func(s hireStep) (int, string) { return d.hireGMFromSetup(s.Account, false) },
		hireRole: d.hireQueueRole,
		check:    d.hireStepProblem,
		hired: func(s hireStep, since float64) bool {
			return slices.ContainsFunc(officeHires(), func(h Hire) bool { return h.Role == s.Role && isOpen(h) && h.Started >= since })
		},
		say: func(text string) {
			d.events.append(eventEntry{At: time.Now().UnixMilli(), Kind: eventKindHireQueue, Detail: cutEventDetail(text)})
		},
	})
}

// hireStepProblem is why a step can never be hired, or "": a role that isn't there, or a folder that's no good.
func (d *Deck) hireStepProblem(s hireStep) string {
	d.mu.Lock()
	known := slices.ContainsFunc(d.config.Roles, func(r Role) bool { return r.ID == s.Role })
	d.mu.Unlock()
	if !known {
		return "That role isn't there any more. Pick another."
	}
	_, why := gmHireFolder(s.Folder)
	return why
}

// hireQueueRole is one role's hire from the queue, as POST /setup/gm/hire makes it: the same hire path as the
// office's, fenced (never a plain terminal), in the step's folder (or the folder for new hires), with the step's
// brief line as the hire's task.
func (d *Deck) hireQueueRole(s hireStep) (int, string) {
	folder, why := gmHireFolder(s.Folder)
	if why != "" {
		return http.StatusBadRequest, why
	}
	seat := -1
	return d.hireWith(hireSpec{roleID: s.Role, floor: s.Floor, fromSetup: true, folder: folder, fenced: true, seat: &seat,
		account: s.Account, task: briefTask(s.Brief)})
}

// hireGMFromSetup is Start here's Hire your General Manager (POST /setup/hire/gm), once the route has judged its
// caller: the GM role, in the folder for your hires. The manager is switched on before the hire (the GM is a
// managed hire) and back if the hire fails, as the Mac does (DeckRouter.swift). d.mu is not held across the hire.
// Hire and fire from the office goes on with it too, as the GM hires its team, and back off the same way.
func (d *Deck) hireGMFromSetup(account string, worktree bool) (int, string) {
	// One at a time, as the Mac claims it: a second click while one runs would read the switches
	// the first turned on as its own "before", and could leave them on when both fail.
	if !d.gmHiring.TryLock() {
		return http.StatusConflict, gmBeingHired
	}
	defer d.gmHiring.Unlock()
	// In the folder for your hires, never your home folder: Start here's first step sets it.
	d.mu.Lock()
	if _, ok := existingFolder(d.config.HireFolder); !ok {
		d.mu.Unlock()
		return http.StatusConflict, chooseHireFolderFirst
	}
	before, hiring := d.config.ManagerEnabled, d.config.HiringEnabled
	d.config.ManagerEnabled = true
	d.config.HiringEnabled = true
	d.config.save()
	d.changed()
	d.mu.Unlock()
	// The Claude account named (with one account it is left out) and a worktree of its own, passed on as the Mac's.
	status, text := d.hireWith(hireSpec{roleID: gmRole, fromSetup: true, account: strings.Trim(account, " \t"), worktree: &worktree})
	if status >= http.StatusBadRequest {
		d.mu.Lock()
		d.config.ManagerEnabled = before
		d.config.HiringEnabled = hiring
		d.config.save()
		d.changed()
		d.mu.Unlock()
	}
	return status, text
}
