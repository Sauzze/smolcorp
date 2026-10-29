package desktop

import (
	"encoding/json"
	"os"
	"slices"
	"time"
)

// Across a restart, as the Mac app's Deck and Sessions: the sessions running
// and their keys are written to sessions.json a moment after each change, and
// those still running when the app starts again are back on their keys.
// Sessions only report as something happens, so an idle one would otherwise
// stay away until it's next used.
//
// Still running is the pane's agent still in front in it, for tmux, WezTerm
// and kitty; the agent's process still there, for a headless run upstairs;
// and for a launcher, the launcher connecting again, as it does by itself a
// few seconds after the app is back. One that doesn't within
// launcherRestoreWait has ended meanwhile.

const (
	// How soon after a change the sessions are saved, so a burst of events
	// writes once.
	saveDelay = time.Second
	// Sessions saved this recently before a restart are shown as they were;
	// any older, a turn may have ended while nobody was listening.
	freshSave = 120_000
	// How long a launcher has to connect again after a restart for its
	// session to be back.
	launcherRestoreWait = 30 * time.Second
)

// savedDeck is what's written to keep the deck across a restart: the
// sessions, their keys, and when.
type savedDeck struct {
	SavedAt  float64        `json:"savedAt"`
	Sessions []savedSession `json:"sessions"`
	Keys     map[string]int `json:"keys"`
}

// savedSession is what's kept of a session while the app restarts, in the
// Mac app's names: no subagents or tools, which its next events tell again.
type savedSession struct {
	ID            string  `json:"id"`
	Agent         string  `json:"agent"`
	Folder        string  `json:"folder"`
	Project       string  `json:"project"`
	Tab           string  `json:"tab"`
	State         string  `json:"state"`
	Effort        string  `json:"effort"`
	Model         string  `json:"model"`
	Transcript    string  `json:"transcript"`
	StartedAt     float64 `json:"startedAt"`
	LastEventAt   float64 `json:"lastEventAt"`
	TurnStartedAt float64 `json:"turnStartedAt,omitempty"`
	PID           int     `json:"pid,omitempty"`
	Parent        string  `json:"parent,omitempty"`
}

// Saved is every session as it's kept across a restart.
func (s *Sessions) Saved() []savedSession {
	saved := []savedSession{}
	for _, session := range s.All() {
		saved = append(saved, savedSession{ID: session.ID, Agent: string(session.Agent), Folder: session.Folder, Project: session.Project,
			Tab: session.Tab, State: string(session.State), Effort: session.Effort, Model: session.Model, Transcript: session.Transcript,
			StartedAt: session.StartedAt, LastEventAt: session.LastEventAt, TurnStartedAt: session.TurnStartedAt, PID: session.PID,
			Parent: session.ParentFolder})
	}
	return saved
}

// Restore puts back sessions from before a restart, still running. Saved
// fresh, each is as it was; any older, a turn may have ended unheard, so
// each is shown at rest until it's heard from. It says whether any came back.
func (s *Sessions) Restore(saved []savedSession, fresh bool) bool {
	restored := false
	for _, entry := range saved {
		if _, known := s.byID[entry.ID]; known || entry.ID == "" || entry.Tab == "" {
			continue
		}
		if _, ended := s.endedAt[entry.ID]; ended {
			continue
		}
		agent := Agent(entry.Agent)
		if !slices.Contains(agents, agent) {
			continue
		}
		session := &Session{ID: entry.ID, Agent: agent, Folder: entry.Folder, Project: entry.Project, Tab: entry.Tab, State: Idle,
			Effort: entry.Effort, Model: entry.Model, Transcript: entry.Transcript, StartedAt: entry.StartedAt,
			LastEventAt: entry.LastEventAt, PID: entry.PID, ParentFolder: entry.Parent}
		if state := SessionState(entry.State); fresh && slices.Contains([]SessionState{Idle, Thinking, Asking, Unread, Failed}, state) {
			session.State = state
			if session.IsBusy() {
				session.TurnStartedAt = entry.TurnStartedAt
			}
		}
		s.byID[entry.ID] = session
		restored = true
	}
	return restored
}

// pendingRestore is what's waiting for launchers to connect again after a
// restart: their sessions, by launcher, until the deadline.
type pendingRestore struct {
	byLauncher map[string][]savedSession
	keys       map[string]int
	fresh      bool
	until      time.Time
}

// scheduleSave saves the sessions a moment from now. With the lock held.
func (d *Deck) scheduleSave() {
	if d.savePending {
		return
	}
	d.savePending = true
	time.AfterFunc(saveDelay, d.saveSessions)
}

func (d *Deck) saveSessions() {
	d.mu.Lock()
	d.savePending = false
	saved := savedDeck{SavedAt: nowMillis(), Sessions: d.sessions.Saved(), Keys: map[string]int{}}
	for id, key := range d.keyOf {
		saved.Keys[id] = key
	}
	// Those still waiting for their launcher are kept too, should the app stop again before it's back.
	if d.restoring != nil && time.Now().Before(d.restoring.until) {
		for _, entries := range d.restoring.byLauncher {
			saved.Sessions = append(saved.Sessions, entries...)
			for _, entry := range entries {
				if key, ok := d.restoring.keys[entry.ID]; ok {
					saved.Keys[entry.ID] = key
				}
			}
		}
	}
	file := d.sessionsFile
	d.mu.Unlock()
	if data, err := json.Marshal(saved); err == nil {
		_ = writePrivate(file, data)
	}
}

// restoreSessions reads the sessions saved before the app last stopped: a
// launcher's wait for it to connect, and the others are looked at in the
// background, the panes' terminals asked whether their agent still runs.
func (d *Deck) restoreSessions() {
	data, err := os.ReadFile(d.sessionsFile)
	if err != nil {
		return
	}
	var saved savedDeck
	if json.Unmarshal(data, &saved) != nil || len(saved.Sessions) == 0 {
		return
	}
	fresh := nowMillis()-saved.SavedAt < freshSave
	waiting := &pendingRestore{byLauncher: map[string][]savedSession{}, keys: saved.Keys, fresh: fresh,
		until: time.Now().Add(launcherRestoreWait)}
	var others []savedSession
	for _, entry := range saved.Sessions {
		if kind, id, _ := splitTab(entry.Tab); kind == "launcher" && id != "" {
			waiting.byLauncher[id] = append(waiting.byLauncher[id], entry)
		} else {
			others = append(others, entry)
		}
	}
	d.mu.Lock()
	d.restoring = waiting
	d.mu.Unlock()
	go func() {
		running := slices.DeleteFunc(others, func(entry savedSession) bool { return !stillRunning(entry, processGone) })
		d.mu.Lock()
		defer d.mu.Unlock()
		d.restore(running, saved.Keys, fresh)
	}()
}

// restore puts back sessions found still running, on the keys they had.
// With the lock held.
func (d *Deck) restore(running []savedSession, keys map[string]int, fresh bool) {
	if len(running) == 0 {
		return
	}
	for _, entry := range running {
		if key, ok := keys[entry.ID]; ok && key >= 0 && key < mostKeys {
			if _, has := d.keyOf[entry.ID]; !has {
				d.keyOf[entry.ID] = key
			}
		}
	}
	if d.sessions.Restore(running, fresh) {
		d.changed()
	}
}

// restoreLauncher puts back the sessions that were in launcher id before a
// restart, now it has connected again. With the lock held.
func (d *Deck) restoreLauncher(id string) {
	waiting := d.restoring
	if waiting == nil || waiting.byLauncher[id] == nil {
		return
	}
	entries := waiting.byLauncher[id]
	delete(waiting.byLauncher, id)
	if time.Now().After(waiting.until) {
		return
	}
	d.restore(entries, waiting.keys, waiting.fresh)
}

// paneRunsAgent is whether a pane is still open with agent in front in it: a
// variable, for the tests.
var paneRunsAgent = func(pane commandTab, agent Agent) bool {
	if !pane.alive() {
		return false
	}
	running, known := pane.runs(agent)
	return known && running
}

// stillRunning is whether a session saved before a restart still runs where
// it did: a pane's agent still in front in it, or a headless run's process
// still there. A launcher's is known only as it connects again: see
// restoreLauncher.
func stillRunning(entry savedSession, gone func(pid int) bool) bool {
	if isHeadless(entry.Tab) {
		return entry.PID > 0 && !gone(entry.PID)
	}
	kind, id, socket := splitTab(entry.Tab)
	if kind != "tmux" && kind != "wezterm" && kind != "kitty" {
		return false
	}
	return paneRunsAgent(commandTab{kind: kind, id: id, socket: socket}, Agent(entry.Agent))
}
