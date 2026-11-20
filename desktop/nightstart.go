package desktop

import (
	"slices"
	"time"
)

// The night shift's runner, its first part (1.26 NS1): a job that falls due starts its hire.
//
//   - The scheduler starts with the deck (nightShiftStarts) and looks with its rounds: a job falls due only once
//     the owner approved it, and it runs as night:<job>.
//   - A hire's job is put to the hire queue as exactly its one hire, and the queue lets it in only while the store
//     holds the job approved (hirequeue.go's startRefusal, Q-F1 (a)): its seal fitted at the start, or the page
//     approved it in this run. Every gate of a hire holds for it as for the GM route's: fenced Claude roles only
//     (fenceRefusal), the hiring switch, SG2's own-files and agent checks.
//   - Before that, the job is checked again as it is now (startRefusal): the General Manager switched on (Q-F2:
//     switched off, it stops night jobs), the role still the office's and a Claude one, the folder still the one
//     approved, inside the home folder, and inside the project its proposer was held to (RT126N1 C3).
//   - How each start went, a refusal's own sentence included, is kept on the job (NightJob.Last) for the morning
//     report. The next job starts once this one's hire is made or refused.
//
// Not yet: a task for the General Manager (said so, and not run), the wall-clock stop and the report (NS2).

// nightShiftStarts is whether a new deck starts the night shift's scheduler: the app's always does. The tests'
// decks don't (isolate), so a look never races a test's stand-in clock; a test starts one itself.
var nightShiftStarts = true

// startNight starts the night shift's scheduler with the deck's rounds: from now on an approved job falls due.
func (d *Deck) startNight() {
	s := d.night.sched
	s.begin(d.nightDue)
	d.rounds.run(func() { s.loop(d.rounds.stopped) })
}

// nightDue starts the job fallen due, or keeps why not; done is called once it has been hired or refused.
func (d *Deck) nightDue(due nightDue, done func()) {
	j, n := due.Job, d.night
	run := NightRun{At: time.Now(), Slot: due.Slot, Missed: due.Missed}
	fail := func(why string) {
		run.Word = "Night job " + j.Name + " didn't start: " + why
		n.ran(j.ID, run)
		done()
	}
	if why := d.nightStartRefusal(j); why != "" {
		fail(why)
		return
	}
	// Held until its done is kept: the queue's end, told on a goroutine of its own, waits for it.
	n.starting.Lock()
	id, err := d.hq.start([]hireStep{j.nightStep()}, "night job "+j.Name, j.actor())
	if err != nil {
		n.starting.Unlock()
		fail(lowerFirst(err.Error()))
		return
	}
	n.pending[id] = done
	run.Word = "Night job " + j.Name + " is hiring its " + j.Role + "."
	n.ran(j.ID, run)
	n.starting.Unlock()
}

// nightEnded is told a night job's queue run as it ends: how it went is kept on its job, and the next job may go.
func (d *Deck) nightEnded(run hireQueueRun) {
	n := d.night
	n.starting.Lock()
	done := n.pending[run.ID]
	delete(n.pending, run.ID)
	n.starting.Unlock()
	if j, ok := n.get(run.Night); ok {
		last := NightRun{At: time.Now()}
		if j.Last != nil {
			last = *j.Last
		}
		last.Hired = slices.ContainsFunc(run.Steps, func(s hireStep) bool { return s.State == hqHired })
		last.Word = run.Word
		n.ran(j.ID, last)
	}
	if done != nil {
		done()
	}
}

// nightStartRefusal is why job can't start now, as it is now, or "".
func (d *Deck) nightStartRefusal(j NightJob) string {
	d.mu.Lock()
	managerOn := d.config.ManagerEnabled
	i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == j.Role })
	var role Role
	if i >= 0 {
		role = d.config.Roles[i]
	}
	d.mu.Unlock()
	switch {
	case !managerOn:
		return "the General Manager is switched off, and night jobs stop with it. Switch it on in Setup."
	case j.Kind != nightHire:
		return "a task for the General Manager doesn't run at night yet: only a hire's job does."
	case i < 0 || j.Role == gmRole:
		return "its role isn't one of the office's any more."
	case Agent(role.Agent) != Claude:
		return "its role isn't a Claude Code one any more, and a night job hires only fenced Claude roles."
	}
	folder, _, why := checkedSetupFolder(j.Folder, homeDir)
	switch {
	case why != "":
		return lowerFirst(why)
	case folder != j.Folder:
		return "its folder isn't the one you approved any more: a link on its way changed."
	}
	if j.Scope != "" {
		s := gmScopeOf(Hire{Scope: j.Scope, ScopeRoles: j.ScopeRoles})
		if !s.holds(folder) {
			return "its folder is outside " + j.Scope + ", the project the General Manager that proposed it is held to."
		}
		if s.roles != nil && !slices.Contains(s.roles, j.Role) {
			return "its role isn't one the General Manager that proposed it may hire."
		}
	}
	return ""
}
