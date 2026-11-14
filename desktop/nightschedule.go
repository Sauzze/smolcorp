package desktop

import (
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// The night shift's scheduler (1.26 N1): when each approved job is next due, by the computer's clock and time
// zone as they are at each look, and a "due" told to the runner N2 gives it, one job at a time.
//
//   - A slot is a date at the job's time, where the computer is now. A date runs once (LastSlot), so a clock put
//     back, an hour that comes twice in the autumn, or a move west never runs a date's job again; a time the
//     clocks skip in the spring runs when they reach the other side.
//   - A slot passed by more than nightLate was missed (the computer slept, or Smolcorp was closed): the job runs
//     once at the next look, with a notice, however many slots went by, and goes on from then.
//   - Smolcorp changes no system setting for it: no wake timer, no launchd job. The look is a plain timer.

const (
	nightEvery = 30 * time.Second // how often the scheduler looks
	nightLate  = 5 * time.Minute  // a slot passed by more than this was missed
)

// nightDue is a job fallen due, as the runner is told it: the job as it is now, the actor it runs as
// (night:<job>), the slot it was due at, and, for a missed one, the notice to show.
type nightDue struct {
	Job    NightJob
	Actor  actor
	Slot   time.Time
	Missed bool
	Notice string
}

// nightScheduler looks for what falls due. Lock order: the store's lock, then this one, never the other way.
type nightScheduler struct {
	store *nightShift
	now   func() time.Time
	zone  func() *time.Location

	mu sync.Mutex
	// due is told each job that falls due, and done to call once it has ended (or never started). It must not
	// wait for the run: the next job waits for done. nil until start: nothing falls due.
	due     func(due nightDue, done func())
	queue   []nightDue
	running string // the id of the job running, "" for none
}

func newNightScheduler(store *nightShift, now func() time.Time) *nightScheduler {
	if now == nil {
		now = time.Now
	}
	return &nightScheduler{store: store, now: func() time.Time { return now().Round(0) }, zone: systemZone}
}

// start looks every nightEvery, and once now, until stop is closed, telling due what falls due. N2 calls it.
func (s *nightScheduler) start(due func(nightDue, func()), stop <-chan struct{}) {
	s.mu.Lock()
	s.due = due
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(nightEvery)
		defer ticker.Stop()
		for {
			s.tick()
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

// runningID is the job running now, "" for none.
func (s *nightScheduler) runningID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// tick is one look: each approved job whose slot has come is marked run (from now, its slot's date) and put in
// line, and the line moves if nothing runs.
func (s *nightScheduler) tick() {
	s.mu.Lock()
	started := s.due != nil
	s.mu.Unlock()
	if !started {
		return
	}
	now, loc := s.now(), s.zone()
	n := s.store
	var fresh []nightDue
	n.mu.Lock()
	jobs := slices.Clone(n.jobs)
	for i, j := range jobs {
		if j.Status != nightApproved || j.When.Every == nightTrigger {
			continue
		}
		slot, ok := nextNightSlot(j, loc)
		if !ok || now.Before(slot) {
			continue
		}
		jobs[i].Since, jobs[i].LastSlot = now, slot.In(loc).Format(time.DateOnly)
		due := nightDue{Job: jobs[i], Actor: j.actor(), Slot: slot}
		if now.Sub(slot) > nightLate {
			due.Missed = true
			due.Notice = j.Name + " was due " + slot.In(loc).Format("Mon 15:04") + " while this " + machineHere() +
				" slept or Smolcorp was closed: it runs once now."
		}
		fresh = append(fresh, due)
	}
	if len(fresh) > 0 {
		// Kept even if the file can't be written: a job is never run again at each look for want of a save.
		n.jobs = jobs
		_ = n.saveLocked()
	}
	n.mu.Unlock()
	s.mu.Lock()
	for _, due := range fresh {
		// A job already in line or running runs once: the slot that came while it waited is that run.
		if s.running != due.Job.ID && !slices.ContainsFunc(s.queue, func(q nightDue) bool { return q.Job.ID == due.Job.ID }) {
			s.queue = append(s.queue, due)
		}
	}
	s.mu.Unlock()
	s.next()
}

// next starts the first job in line, if none runs: as it is now, and only if it is still approved.
func (s *nightScheduler) next() {
	for {
		s.mu.Lock()
		if s.running != "" || len(s.queue) == 0 || s.due == nil {
			s.mu.Unlock()
			return
		}
		due := s.queue[0]
		s.queue = s.queue[1:]
		s.mu.Unlock()
		j, ok := s.store.get(due.Job.ID)
		if !ok || j.Status != nightApproved {
			continue // deleted or paused while it waited
		}
		due.Job = j
		s.mu.Lock()
		if s.running != "" {
			s.queue = append([]nightDue{due}, s.queue...)
			s.mu.Unlock()
			return
		}
		s.running = j.ID
		tell := s.due
		s.mu.Unlock()
		var once sync.Once
		tell(due, func() {
			once.Do(func() {
				s.mu.Lock()
				s.running = ""
				s.mu.Unlock()
				s.next()
			})
		})
		return
	}
}

// nextNightSlot is j's first slot after j.Since (its creation if never looked from), in loc, on a date after its
// last run's: false for a job with no clock, or none found in the ten dates looked at.
func nextNightSlot(j NightJob, loc *time.Location) (time.Time, bool) {
	from := j.Since
	if from.IsZero() {
		from = j.Created
	}
	hour, minute, ok := nightClock(j.When.At)
	if !ok || (j.When.Every != nightDaily && j.When.Every != nightWeekly) {
		return time.Time{}, false
	}
	y, m, d := from.In(loc).Date()
	for k := range 10 {
		date := time.Date(y, m, d+k, 12, 0, 0, 0, time.UTC)
		if j.When.Every == nightWeekly && nightWeekdays[date.Weekday()] != j.When.Day {
			continue
		}
		if j.LastSlot != "" && date.Format(time.DateOnly) <= j.LastSlot {
			continue
		}
		if slot := wallSlot(date, hour, minute, loc); slot.After(from) {
			return slot, true
		}
	}
	return time.Time{}, false
}

// nightClock is "HH:MM" as an hour and a minute.
func nightClock(at string) (int, int, bool) {
	if !nightAtPattern.MatchString(at) {
		return 0, 0, false
	}
	return int(at[0]-'0')*10 + int(at[1]-'0'), int(at[3]-'0')*10 + int(at[4]-'0'), true
}

// wallSlot is date at hour:minute in loc. A time the clocks skip is the moment they jump over it; a time that
// comes twice is its first.
func wallSlot(date time.Time, hour, minute int, loc *time.Location) time.Time {
	y, m, d := date.Date()
	t := time.Date(y, m, d, hour, minute, 0, 0, loc)
	asked := time.Date(y, m, d, hour, minute, 0, 0, time.UTC)
	wall := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
	}
	start, end := t.ZoneBounds()
	switch {
	case wall(t).Before(asked) && !end.IsZero():
		return end // read before the gap: the clocks jump at the zone's end
	case wall(t).After(asked) && !start.IsZero():
		return start // read after the gap: they jumped at its start
	case !start.IsZero():
		_, offset := t.Zone()
		_, before := start.Add(-time.Second).Zone()
		if earlier := t.Add(time.Duration(offset-before) * time.Second); before > offset && earlier.Before(start) && wall(earlier.In(loc)).Equal(asked) {
			return earlier
		}
	}
	return t
}

// zoneNamed keeps the last zone read, so a look reads the zone files only when the zone changes.
var zoneNamed struct {
	sync.Mutex
	name string
	loc  *time.Location
}

// systemZone is the computer's time zone as it is now, not as it was when Smolcorp started (time.Local): TZ if
// set, else where /etc/localtime points (a Mac and most Linux). Elsewhere, or if it can't be read, time.Local.
func systemZone() *time.Location {
	name, set := os.LookupEnv("TZ")
	if set {
		name = strings.TrimPrefix(name, ":")
	} else if runtime.GOOS != "windows" {
		target, err := os.Readlink("/etc/localtime")
		_, after, found := strings.Cut(target, "zoneinfo/")
		if err != nil || !found {
			return time.Local
		}
		name = after
	} else {
		return time.Local
	}
	zoneNamed.Lock()
	defer zoneNamed.Unlock()
	if zoneNamed.loc != nil && zoneNamed.name == name {
		return zoneNamed.loc
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	zoneNamed.name, zoneNamed.loc = name, loc
	return loc
}
