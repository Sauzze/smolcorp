package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Deck is the deck itself: the sessions, how they sit on the six keys, what
// the phone's controls do to them, and the live feed of all that to phones,
// as the Mac app's Deck.swift.
//
// One mutex guards it all. Keys are pressed one at a time on a worker of
// their own, outside the lock, so a slow terminal never holds up the server
// and two quick notches arrive in the order they were turned.
type Deck struct {
	mu        sync.Mutex
	access    *Access
	config    *Config
	sessions  *Sessions
	streams   map[*eventStream]string // each open stream, with the phone that opened it
	launchers map[string]*launcherTab
	// The key each session was given, so it keeps it.
	keyOf map[string]int
	// The tab recording started in, and when. One computer, one microphone.
	micPress *micPress
	// When each session was last sent Return from the phone, so a tap that
	// lands twice sends it once.
	lastReturn map[string]time.Time
	// The hires whose launcher hasn't connected yet, by their token, and
	// when the last hire was made: see hire.go.
	hires    map[string]*pendingHire
	lastHire time.Time
	// Whether a save of the sessions is coming, and the sessions from before
	// a restart still waiting for their launcher: see saved.go.
	savePending bool
	restoring   *pendingRestore
	// Whether music is playing, for the office's jukebox, when it can be told.
	playing *bool
	// This computer's output volume, for the jukebox's volume bar, nil where
	// it can't be read or set.
	volume *musicVolume
	// What each session was when the webhook was last told, nil until the
	// deck first looks; and how the last delivery went, for the setup page.
	told         map[string]told
	webhookMu    sync.Mutex
	webhookLast  string
	webhookSends chan func() // the webhook's messages, sent in turn by sendWebhooks
	work         chan func()
	// ✨ Summary: its key, from the system's secret store or ANTHROPIC_API_KEY,
	// and the summaries made with it.
	summaryKeys *summaryKeys
	summaries   *summaries
	// OnChange is called on every change, for the tray.
	OnChange func()
}

type micPress struct {
	tab string
	at  time.Time
}

const (
	// How often to ping event streams: often enough that an idle one stays
	// open, as a browser may drop one that's quiet for a minute, and that the
	// page can tell a stream that died quietly.
	pingInterval = 40 * time.Second
	// How often to look for tabs that closed, and agents that exited, without
	// their session ending.
	pruneInterval = 10 * time.Second
	// Claude stops listening by itself after two minutes.
	voiceLimit = 120 * time.Second
	// Two Returns to one session closer than this are one tap landing twice.
	returnRepeat = 500 * time.Millisecond
	// How often to end the sessions ended for /clear or /resume that nothing took the place of.
	handOnInterval = 1500 * time.Millisecond
	// How often to look whether music is playing, with the jukebox on.
	musicInterval = 3 * time.Second
)

var effortLevels = []string{"low", "medium", "high", "xhigh", "max"}

func NewDeck() *Deck {
	d := &Deck{
		access: NewAccess(), config: loadConfig(), sessions: NewSessions(),
		streams: map[*eventStream]string{}, launchers: map[string]*launcherTab{},
		keyOf: map[string]int{}, lastReturn: map[string]time.Time{}, hires: map[string]*pendingHire{}, work: make(chan func(), 64), OnChange: func() {},
		webhookSends: make(chan func(), webhookQueue),
	}
	d.summaryKeys = newSummaryKeys(keyringStore, os.Getenv)
	d.summaries = newSummaries(d.summaryKeys.Read)
	go d.sendWebhooks()
	d.restoreSessions()
	// What's running as it starts is where the webhook starts from, so the first change after it is news
	d.tellWebhook()
	go func() {
		for job := range d.work {
			job()
		}
	}()
	go func() {
		for range time.Tick(pingInterval) {
			d.mu.Lock()
			for stream := range d.streams {
				stream.send(nil)
			}
			d.mu.Unlock()
		}
	}()
	go func() {
		for range time.Tick(pruneInterval) {
			d.pruneClosedTabs()
			d.pruneGoneAgents()
		}
	}()
	go func() {
		for range time.Tick(handOnInterval) {
			d.mu.Lock()
			if d.sessions.EndHandOns(nowMillis()) {
				d.changed()
			}
			d.mu.Unlock()
		}
	}()
	go func() {
		for range time.Tick(musicInterval) {
			d.followMusic()
		}
	}()
	return d
}

// followMusic looks whether music is playing, with the jukebox on, and tells
// the phones when that changes.
func (d *Deck) followMusic() {
	d.mu.Lock()
	enabled := d.config.MusicEnabled
	d.mu.Unlock()
	if !enabled {
		return
	}
	playing := musicPlaying()
	// The volume is looked at as often, to follow the keyboard's volume keys.
	volume := readMusicVolume()
	d.mu.Lock()
	defer d.mu.Unlock()
	if (playing == nil) != (d.playing == nil) || (playing != nil && *playing != *d.playing) ||
		(volume == nil) != (d.volume == nil) || (volume != nil && *volume != *d.volume) {
		d.playing = playing
		d.volume = volume
		d.changed()
	}
}

// musicKeys is what a jukebox's keys do, as the office names them.
var musicKeys = []string{"playpause", "previous", "next"}

// pressMusicKey presses one of musicKeys: a variable, for the tests.
var pressMusicKey = musicKey

// playMusic plays or pauses this computer's music, or skips a song either
// way, from a jukebox in the office.
func (d *Deck) playMusic(key string) (int, string) {
	if !contains(musicKeys, key) {
		return http.StatusBadRequest, "A jukebox has no " + key + " key."
	}
	d.mu.Lock()
	enabled := d.config.MusicEnabled
	d.mu.Unlock()
	if !enabled {
		return http.StatusForbidden, musicOff
	}
	if err := pressMusicKey(key); err != nil {
		return http.StatusInternalServerError, "Couldn't reach the music here."
	}
	// The player takes a moment to say it has.
	time.AfterFunc(600*time.Millisecond, d.followMusic)
	return http.StatusNoContent, ""
}

// readMusicVolume and setMusicVolume read and set the output volume:
// variables, for the tests.
var (
	readMusicVolume = getVolume
	setMusicVolume  = setVolume
)

// setMusicLevel sets this computer's output volume from the jukebox's bar:
// level is 0 to 100.
func (d *Deck) setMusicLevel(text string) (int, string) {
	level, ok := volumeLevel(text)
	if !ok {
		return http.StatusBadRequest, "The volume goes from 0 to 100."
	}
	d.mu.Lock()
	enabled := d.config.MusicEnabled
	d.mu.Unlock()
	if !enabled {
		return http.StatusForbidden, musicOff
	}
	if err := setMusicVolume(level); err != nil {
		return http.StatusInternalServerError, "Couldn't reach the volume here."
	}
	go d.followMusic()
	return http.StatusNoContent, ""
}

// headlessNoControls is what the phone says when a control is tried on a
// headless run's desk.
const headlessNoControls = "Nothing to type into: it runs headless."

// musicOff is what the phone says when a jukebox is tapped with the control off.
const musicOff = "The jukebox can't play your music: turn it on in setup."

// MARK: Hook events

// Ingest takes an event from the hook. reported is the agent the hook was
// set up for; see agentOf for why that isn't always the sender.
func (d *Deck) Ingest(event map[string]any, reported, tab, startedIn string, at float64) {
	d.IngestFrom(event, reported, tab, startedIn, 0, "", at)
}

// IngestFrom is Ingest with the pid of the agent's process, where the hook
// sent it, or 0.
func (d *Deck) IngestFrom(event map[string]any, reported, tab, startedIn string, pid int, parent string, at float64) {
	agent := agentOf(event, reported)
	canonical := agent.canonical(event)
	if canonical == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if tab == "" {
		tab = d.launcherFor(canonical, agent)
	}
	changed := d.sessions.Apply(canonical, agent, tab, startedIn, at)
	if id, _ := canonical["session_id"].(string); pid > 0 || parent != "" {
		if session := d.sessions.Get(id); session != nil {
			if pid > 0 {
				session.PID = pid
			}
			if parent != "" {
				session.ParentFolder = parent
			}
		}
	}
	if d.noteMessage(canonical, agent) || changed {
		d.changed()
	}
}

// noteMessage marks a Claude Code session sending another a message, as its
// SendMessage tool is about to, with when and to whom, for the office to walk
// it over, and says whether it did. A recipient that isn't at a desk, or
// can't be told apart, is left out. It changes nothing else. With the lock held.
func (d *Deck) noteMessage(event map[string]any, agent Agent) bool {
	if agent != Claude || event["hook_event_name"] != "PreToolUse" || event["tool_name"] != "SendMessage" {
		return false
	}
	id, _ := event["session_id"].(string)
	input, _ := event["tool_input"].(map[string]any)
	to, _ := input["to"].(string)
	sender := d.sessions.Get(id)
	recipient := claudePeer(to)
	if sender == nil || recipient == "" || recipient == id || d.sessions.Get(recipient) == nil {
		return false
	}
	sender.MessagedAt, sender.Messaged = nowMillis(), recipient
	return true
}

// launcherFor finds a session's launcher when its hook couldn't say: Codex
// runs its hooks in a shared background service, and Gemini CLI can hide
// the launcher's variable. It's the launcher running that agent in the
// session's folder, when exactly one does and no other session holds it.
// A session the deck knows keeps the tab it was found in.
func (d *Deck) launcherFor(event map[string]any, agent Agent) string {
	id, _ := event["session_id"].(string)
	folder, _ := event["cwd"].(string)
	if id == "" || folder == "" || d.sessions.Get(id) != nil {
		return ""
	}
	taken := map[string]bool{}
	for _, session := range d.sessions.All() {
		taken[session.Tab] = true
	}
	match := ""
	for launcherID, launcher := range d.launchers {
		name := "launcher:" + launcherID
		if launcher.agent == agent && launcher.folder == folder && !taken[name] {
			if match != "" {
				return "" // several fit: a guess would put a session on the wrong key
			}
			match = name
		}
	}
	return match
}

// changed re-sends the state, with the lock held.
func (d *Deck) changed() {
	d.applyQueuedEffort()
	state := d.snapshot()
	for stream := range d.streams {
		stream.send(state)
	}
	go d.OnChange()
	d.scheduleSave()
	d.tellWebhook()
}

// Refresh re-sends the state, for changes the sessions don't see (the setup
// page's).
func (d *Deck) Refresh() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.changed()
}

// MARK: Keys

type deckKey struct {
	pin     *string
	session *Session
}

// layout: a pinned key shows its project's session when one is running and
// stays reserved when none is. Every other session gets a free key. With a
// fixed number of keys, a session keeps its key until it ends, so keys don't
// shuffle as sessions come and go. With Auto, the keys grow and shrink with
// the sessions, and those still running keep their order when one ends.
// Sessions beyond the last key have none until one frees up. Headless runs
// have no keys: they sit upstairs (see upstairs).
func (d *Deck) layout() ([]deckKey, []*Session) {
	remaining := slices.DeleteFunc(d.sessions.All(), (*Session).Headless)
	// A session that took another's place in its tab, as after /clear, takes its key too.
	for new, old := range d.sessions.TakeReplacements() {
		if index, ok := d.keyOf[old]; ok {
			if _, has := d.keyOf[new]; !has {
				d.keyOf[new] = index
			}
		}
	}
	// A hire's desk goes to the first session in the launcher it opened. It's
	// kept for it below if it's still free; if it was taken in the meantime,
	// the hire sits at the first free one, as anyone would.
	for _, s := range remaining {
		kind, id, _ := splitTab(s.Tab)
		if tab, ok := d.launchers[id]; kind == "launcher" && ok && tab.desk >= 0 {
			d.keyOf[s.ID] = tab.desk
			tab.desk = -1
		}
	}
	live := map[string]bool{}
	for _, s := range remaining {
		live[s.ID] = true
	}
	// A session dropped as its tab seemed to close keeps its key, should it come back.
	for id := range d.keyOf {
		if !live[id] && !d.sessions.WasDropped(id) {
			delete(d.keyOf, id)
		}
	}
	fixed := d.config.Layout.Keys != Auto
	keys := make([]deckKey, mostKeys)
	if fixed {
		keys = keys[:d.config.Layout.Keys]
	}
	for i := range keys {
		keys[i].pin = d.config.Pins[i]
	}
	place := func(s *Session, i int) {
		keys[i].session = s
		d.keyOf[s.ID] = i
		remaining = slices.DeleteFunc(remaining, func(r *Session) bool { return r.ID == s.ID })
	}
	for i := range keys {
		if keys[i].pin == nil {
			continue
		}
		var candidates []*Session
		for _, s := range remaining {
			if s.Project == *keys[i].pin {
				candidates = append(candidates, s)
			}
		}
		if len(candidates) > 0 {
			match := candidates[0]
			for _, c := range candidates {
				if index, ok := d.keyOf[c.ID]; ok && index == i {
					match = c
				}
			}
			place(match, i)
		}
	}
	if fixed {
		for _, s := range slices.Clone(remaining) {
			if i, ok := d.keyOf[s.ID]; ok && i < len(keys) && keys[i].pin == nil && keys[i].session == nil {
				place(s, i)
			}
		}
	} else {
		// Every pinned key, and a free one for each other session.
		pinned, last := 0, 0
		for i, k := range keys {
			if k.pin != nil {
				pinned, last = pinned+1, i+1
			}
		}
		keys = keys[:min(max(1, last, pinned+len(remaining)), mostKeys)]
		// In the order their keys had, so the grid shrinks without shuffling.
		slices.SortStableFunc(remaining, func(a, b *Session) int { return d.previousKey(a) - d.previousKey(b) })
	}
	for _, s := range slices.Clone(remaining) {
		free := slices.IndexFunc(keys, func(k deckKey) bool { return k.pin == nil && k.session == nil })
		if free < 0 {
			break
		}
		place(s, free)
	}
	// A session left without a key forgets the one it had, or it would claim
	// it back from the session that has it now.
	for _, s := range remaining {
		delete(d.keyOf, s.ID)
	}
	return keys, remaining
}

// previousKey is the key a session last had, or one past the last for a
// session new to the deck.
func (d *Deck) previousKey(s *Session) int {
	if i, ok := d.keyOf[s.ID]; ok {
		return i
	}
	return mostKeys
}

// snapshot is what the phone draws: the keys, the theme and the layout. No
// tabs or paths.
func (d *Deck) snapshot() []byte {
	keys, overflow := d.layout()
	entries := make([]map[string]any, len(keys))
	for i, key := range keys {
		entry := map[string]any{"pin": nil}
		if key.pin != nil {
			entry["pin"] = lastPathComponent(*key.pin)
		}
		if s := key.session; s != nil {
			entry["session"] = d.sessionEntry(s)
		}
		entries[i] = entry
	}
	state, _ := json.Marshal(map[string]any{
		"keys": entries, "overflow": len(overflow), "upstairs": d.upstairs(),
		"theme": currentTheme(d.config), "scroll": d.config.ScrollEnabled,
		// What the office's held desks offer: firing, and hiring once there's a folder to hire into, by role.
		"hiring": d.hiringState(),
		"music":  d.musicState(),
		"layout": d.config.Layout,
		// This computer's clock, so the office can count turns on the phone's,
		// whichever of the two is right.
		"now": int64(nowMillis()),
	})
	return state
}

// sessionEntry is what the phone is told of a session at a desk.
func (d *Deck) sessionEntry(s *Session) map[string]any {
	effort := s.Effort
	if s.PendingEffort != "" {
		effort = s.PendingEffort
	}
	return map[string]any{
		"id": s.ID, "name": s.Name(), "project": s.ProjectName(), "state": s.State, "effort": effort,
		"pending": s.PendingEffort != "", "agent": s.Agent, "agentName": s.Agent.Name(),
		"dial": s.Agent.HasEffortDial(), "mic": s.Agent.HasVoice(),
		"activity": orNull(s.Activity()), "tool": orNull(s.Tool), "model": orNull(s.Model),
		"subagents":    append([]Subagent{}, s.Subagents...),
		"maxPressedAt": millisOrNull(s.MaxPressedAt), "compacting": s.Compacting,
		"messagedAt": millisOrNull(s.MessagedAt), "messaged": orNull(s.Messaged),
		"turnStartedAt": millisOrNull(s.TurnStartedAt), "errorKind": orNull(s.ErrorKind),
		// ✨ Summary: offered only when it's on, with a key, and a transcript to read.
		"summary": s.Transcript != "" && d.summaryOffered(),
		"role":    d.roleOf(s),
	}
}

// summaryState is ✨ Summary for the setup page: on or off, the key's hint and
// where it's from (never the key), whether there's a secret store to keep one
// in, and the model, for the dialog. With the lock held.
func (d *Deck) summaryState() map[string]any {
	source, hint := d.summaryKeys.Shown()
	var key any
	if source != "" {
		key = map[string]any{"source": source, "hint": hint}
	}
	return map[string]any{
		"enabled": d.config.SummaryOptIn && source != "", "key": key, "model": summaryModelName,
		"noStore": d.summaryKeys.NoStore(),
	}
}

// summaryOffered is whether ✨ summary is offered at all: turned on, and with
// a key. With the lock held.
func (d *Deck) summaryOffered() bool {
	return d.config.SummaryOptIn && d.summaryKeys.Present()
}

// Summarize is a few lines on what session id has been doing, from the end of
// its transcript. Nothing is sent with Summary off, or without a key.
func (d *Deck) Summarize(id string) (string, *SummaryError) {
	d.mu.Lock()
	if !d.config.SummaryOptIn {
		d.mu.Unlock()
		return "", errSummaryOff
	}
	if !d.summaryKeys.Present() {
		d.mu.Unlock()
		return "", errSummaryNoKey
	}
	s := d.sessions.Get(id)
	if s == nil || s.Headless() {
		d.mu.Unlock()
		return "", errSummaryNoSession
	}
	path, project := s.Transcript, s.ProjectName()
	d.mu.Unlock()
	if path == "" {
		return "", errSummaryNoHistory
	}
	return d.summaries.Summarize(id, path, project)
}

// hiredBy is who started a headless run, for its card upstairs: the
// ground-floor session working in the folder it was launched from, the
// oldest, by its id and name; else that folder's name with no id; else nil,
// when the launcher didn't say. Only names leave the computer, never the
// folder. With the lock held.
func (d *Deck) hiredBy(s *Session) map[string]any {
	parent := strings.TrimSuffix(s.ParentFolder, "/")
	if parent == "" {
		return nil
	}
	for _, other := range d.sessions.All() {
		if !other.Headless() && (strings.TrimSuffix(other.Project, "/") == parent || strings.TrimSuffix(other.Folder, "/") == parent) {
			return map[string]any{"id": other.ID, "name": other.ProjectName()}
		}
	}
	return map[string]any{"id": nil, "name": lastPart(parent)}
}

// mostUpstairs is as many desks as the second floor has.
const mostUpstairs = 12

// upstairs is the second floor: a desk for each headless run, oldest first,
// up to mostUpstairs, and how many more there are. Its desks show what each
// is doing, with every control off: there is nothing to type into.
func (d *Deck) upstairs() map[string]any {
	desks := []map[string]any{}
	overflow := 0
	for _, s := range d.sessions.All() {
		if !s.Headless() {
			continue
		}
		if len(desks) == mostUpstairs {
			overflow++
			continue
		}
		entry := d.sessionEntry(s)
		entry["headless"] = true
		for _, control := range []string{"keys", "fire", "mic", "dial", "dictates", "summary"} {
			entry[control] = false
		}
		entry["role"] = nil
		entry["startedAt"] = int64(s.StartedAt)
		entry["hiredBy"] = d.hiredBy(s)
		desks = append(desks, entry)
	}
	return map[string]any{"desks": desks, "overflow": overflow}
}

// musicState is what the office's jukeboxes know: whether music is playing,
// null when that can't be told, and the volume, null where it can't be set;
// or nothing at all with the control off.
func (d *Deck) musicState() any {
	if !d.config.MusicEnabled {
		return nil
	}
	state := map[string]any{"playing": d.playing, "volume": nil, "muted": false}
	if d.volume != nil {
		state["volume"], state["muted"] = d.volume.level, d.volume.muted
	}
	return state
}

// orNull and millisOrNull write nothing as null, as the Mac app does.
func orNull(text string) any {
	if text == "" {
		return nil
	}
	return text
}

func millisOrNull(at float64) any {
	if at == 0 {
		return nil
	}
	return int64(at)
}

// MARK: Phones

func (d *Deck) attach(phone string) *eventStream {
	d.mu.Lock()
	defer d.mu.Unlock()
	stream := newEventStream()
	d.streams[stream] = phone
	stream.send(d.snapshot())
	return stream
}

func (d *Deck) detach(stream *eventStream) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.streams, stream)
}

func (d *Deck) isConnected(phone string) bool {
	for _, owner := range d.streams {
		if owner == phone {
			return true
		}
	}
	return false
}

// hiringState is what the office's held desks offer: firing, and hiring
// once there's a folder to hire into and somewhere to open it, by role. With
// the lock held.
func (d *Deck) hiringState() map[string]any {
	_, folder := existingFolder(d.config.HireFolder)
	hire := d.config.HiringEnabled && folder && canHireHere()
	roles := []map[string]any{}
	if hire {
		for _, role := range d.config.Roles {
			roles = append(roles, role.forPhone())
		}
	}
	return map[string]any{"fire": d.config.HiringEnabled, "hire": hire, "roles": roles}
}

// unpair forgets a phone, or every phone when id is "", and hangs up on it
// now rather than at its next request. The lock is held.
func (d *Deck) unpair(id string) {
	if id == "" {
		d.access.Phones.RemoveAll()
	} else {
		d.access.Phones.Remove(id)
	}
	for stream, owner := range d.streams {
		if id == "" || owner == id {
			stream.close()
		}
	}
}

// MARK: Launchers

// roleOf is what the phone is told of the role a session's launcher was
// started with, or nil: also nil once that role is gone from setup.
func (d *Deck) roleOf(s *Session) map[string]any {
	kind, id, _ := splitTab(s.Tab)
	if tab, ok := d.launchers[id]; kind == "launcher" && ok && tab.role != "" {
		for _, role := range d.config.Roles {
			if role.ID == tab.role {
				return role.forPhone()
			}
		}
	}
	return nil
}

// launcherConnected keeps a launcher that has connected. hire is the token it
// was hired with, if the office hired it, which brings the desk it was hired at.
func (d *Deck) launcherConnected(id string, agent Agent, folder, role, hire string) *launcherTab {
	d.mu.Lock()
	defer d.mu.Unlock()
	tab := &launcherTab{agent: agent, folder: folder, role: role, desk: -1, commands: make(chan launcherCommand, 32)}
	if hire != "" {
		tab.desk = d.hiredDesk(hire)
	}
	d.launchers[id] = tab
	// Back after a restart: the sessions it held are back too.
	d.restoreLauncher(id)
	return tab
}

// launcherGone ends the sessions in it: the agent has exited, or its
// terminal has closed.
func (d *Deck) launcherGone(id string, tab *launcherTab) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.launchers[id] != tab {
		return // a newer connection from the same launcher has taken over
	}
	delete(d.launchers, id)
	if d.sessions.Prune(func(name string) bool { return name != "launcher:"+id }, nowMillis()) {
		d.changed()
	}
}

// tab is the Tab a session's tab name stands for, with the lock held.
func (d *Deck) tab(name string) Tab {
	kind, id, socket := splitTab(name)
	if kind == "headless" {
		return nil // nothing to press, type into or watch
	}
	if kind == "launcher" {
		if tab, ok := d.launchers[id]; ok {
			return tab
		}
		return nil
	}
	return commandTab{kind: kind, id: id, socket: socket}
}

// pruneClosedTabs drops sessions whose pane has closed. Launchers say so
// themselves, as their connection drops.
func (d *Deck) pruneClosedTabs() {
	d.mu.Lock()
	panes := map[string]commandTab{}
	for _, s := range d.sessions.All() {
		if tab, ok := d.tab(s.Tab).(commandTab); ok {
			panes[s.Tab] = tab
		}
	}
	d.mu.Unlock()

	gone := map[string]bool{}
	for name, pane := range panes {
		if !pane.alive() {
			gone[name] = true
		}
	}
	if len(gone) == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sessions.Prune(func(name string) bool { return !gone[name] }, nowMillis()) {
		d.changed()
	}
}

// pruneGoneAgents drops sessions whose agent has exited, and headless runs
// long quiet: see Sessions.PruneGone.
func (d *Deck) pruneGoneAgents() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sessions.PruneGone(processGone, nowMillis()) {
		d.changed()
	}
}

// MARK: Controls

// inTab runs op on the session's tab, on the key worker, and waits for it.
// A tab you have been shown counts as seen. Called without the lock.
func (d *Deck) inTab(name string, op func(Tab) error) (int, string) {
	d.mu.Lock()
	tab := d.tab(name)
	d.mu.Unlock()
	if tab == nil {
		return http.StatusNotFound, "Tab not found"
	}
	result := make(chan error, 1)
	d.work <- func() { result <- op(tab) }
	switch err := <-result; {
	case errors.Is(err, errAgentGone):
		d.mu.Lock()
		if d.sessions.Prune(func(tab string) bool { return tab != name }, nowMillis()) {
			d.changed()
		}
		d.mu.Unlock()
		return http.StatusGone, "That agent has ended."
	case errors.Is(err, errUnsupported):
		return http.StatusNotFound, "This terminal can't do that"
	case errors.Is(err, errNotStopped):
		return http.StatusConflict, "Couldn't stop the agent, so its tab is left open."
	case err != nil:
		return http.StatusNotFound, "Tab not found"
	}
	d.mu.Lock()
	if d.sessions.MarkSeen(name) {
		d.changed()
	}
	d.mu.Unlock()
	return http.StatusNoContent, ""
}

const hiringOff = "Hiring and firing are off. Turn them on in setup, on your computer."

var errAgentGone = errors.New("the agent no longer runs in its pane")

// toAgent runs op on the session's tab once its agent is known to run there
// still. An agent that ended without saying so, crashed or killed, leaves its
// pane to the shell, where its keys would land and a button's text would run
// as a command: its session goes instead. Launchers end with their agent,
// so only panes are asked.
func (d *Deck) toAgent(name string, agent Agent, op func(Tab) error) (int, string) {
	return d.inTab(name, func(t Tab) error {
		if pane, ok := t.(commandTab); ok {
			if running, known := pane.runs(agent); known && !running {
				return errAgentGone
			}
		}
		return op(t)
	})
}

// pressTo presses keystroke for agent, in its tab: see toAgent.
func (d *Deck) pressTo(name string, agent Agent, keystroke Keystroke) (int, string) {
	return d.toAgent(name, agent, func(t Tab) error { return t.Press(keystroke) })
}

// swapKeys swaps two keys' projects, for a person carried from one desk to
// another in the office: each is pinned where the other was, so the order
// holds the next time too, as pins made in setup do. It only orders the keys,
// so any paired phone may.
// setStyle puts the office in one of its styles, from the phone, on every
// screen it's on. Only the style's name is kept: what you furnished stays
// under it.
func (d *Deck) setStyle(preset string) (int, string) {
	if !slices.Contains(roomStyles, preset) {
		return http.StatusBadRequest, "Unknown style"
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.config.Layout.Room.Preset == preset {
		return http.StatusNoContent, ""
	}
	d.config.Layout.Room.Preset = preset
	d.config.save()
	d.changed()
	return http.StatusNoContent, ""
}

func (d *Deck) swapKeys(query url.Values) (int, string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	keys, _ := d.layout()
	a, errA := strconv.Atoi(query.Get("from"))
	b, errB := strconv.Atoi(query.Get("to"))
	if errA != nil || errB != nil || a == b || a < 0 || b < 0 || a >= len(keys) || b >= len(keys) {
		return http.StatusBadRequest, "Unknown desks"
	}
	project := func(i int) *string {
		if s := keys[i].session; s != nil {
			p := s.Project
			return &p
		}
		return keys[i].pin
	}
	first, second := project(a), project(b)
	switch {
	case first == nil && second == nil:
		return http.StatusNoContent, ""
	case first != nil && second != nil && *first == *second:
		return http.StatusConflict, "Both desks are the same project, so they stay as they are."
	}
	d.config.SwapPins(first, a, second, b)
	d.changed()
	return http.StatusNoContent, ""
}

// Perform runs one of the phone's controls on a session.
func (d *Deck) Perform(action, id string, query url.Values) (int, string) {
	// Releasing the mic stops recording where it started, whatever is
	// selected now — and even if that session has since gone.
	if action == "mic" && query.Get("phase") != "down" {
		return d.releaseMic()
	}
	d.mu.Lock()
	hiring := d.config.HiringEnabled
	d.mu.Unlock()
	if action == "swap" {
		return d.swapKeys(query)
	}
	if action == "style" {
		return d.setStyle(query.Get("preset"))
	}
	if action == "music" {
		key := query.Get("do")
		if key == "volume" {
			return d.setMusicLevel(query.Get("level"))
		}
		if key == "" {
			key = "playpause"
		}
		return d.playMusic(key)
	}
	if action == "hire" {
		// A new hire has no session yet.
		if query.Has("desk") && query.Get("desk") == "" {
			return http.StatusBadRequest, "Unknown desk"
		}
		return d.hire(query.Get("desk"), query.Get("role"))
	}
	d.mu.Lock()
	session := d.sessions.Get(id)
	if session == nil || session.Tab == "" {
		d.mu.Unlock()
		return http.StatusNotFound, "Session not found"
	}
	if session.Headless() {
		d.mu.Unlock()
		return http.StatusConflict, headlessNoControls
	}
	tab, agent, busy := session.Tab, session.Agent, session.IsBusy()
	d.mu.Unlock()

	// Accept and send are both Enter. They are separate controls on the phone
	// because they mean different things, as do reject and esc, which are
	// often both Escape but not in every agent.
	switch action {
	case "accept", "send":
		if d.isRepeatedReturn(id) {
			return http.StatusNoContent, ""
		}
		status, message := d.pressTo(tab, agent, press(Enter))
		if status >= http.StatusBadRequest {
			// A Return that didn't go doesn't count, so the next tap goes.
			d.mu.Lock()
			delete(d.lastReturn, id)
			d.mu.Unlock()
		}
		return status, message
	case "reject":
		return d.pressTo(tab, agent, agent.Reject())
	case "esc":
		return d.pressTo(tab, agent, agent.Interrupt())
	case "up":
		return d.pressTo(tab, agent, press(Up))
	case "down":
		return d.pressTo(tab, agent, press(Down))
	case "scroll":
		d.mu.Lock()
		enabled := d.config.ScrollEnabled
		d.mu.Unlock()
		if !enabled {
			return http.StatusForbidden, "Scrolling is off. Turn it on in setup, on your computer."
		}
		if query.Get("to") == "bottom" {
			return d.inTab(tab, func(t Tab) error { return t.ScrollToBottom() })
		}
		lines, err := strconv.Atoi(query.Get("lines"))
		if err != nil {
			return http.StatusBadRequest, "Unknown amount"
		}
		return d.inTab(tab, func(t Tab) error { return t.Scroll(clamp(lines, maxScroll)) })
	case "mic":
		if !agent.HasVoice() {
			return http.StatusBadRequest, agent.Name() + " has no voice input"
		}
		return d.pressMic(tab)
	case "fire":
		if !hiring {
			return http.StatusForbidden, hiringOff
		}
		if status, message := d.inTab(tab, func(t Tab) error { return t.Close(agent) }); status != http.StatusNotFound {
			return status, message
		}
		return http.StatusNotFound, "That agent's tab has gone."
	case "clear":
		if busy {
			return http.StatusConflict, "Busy — clear it when " + agent.Name() + " is done"
		}
		return d.toAgent(tab, agent, func(t Tab) error { return t.Type(agent.NewConversation()) })
	case "button":
		d.mu.Lock()
		button, found := d.config.Layout.madeButton(query.Get("button"))
		d.mu.Unlock()
		switch {
		case !found:
			return http.StatusNotFound, "That button has gone. Your phone will catch up in a moment."
		case len(button.Agents) > 0 && !slices.Contains(button.Agents, string(agent)):
			return http.StatusBadRequest, "This button isn't for " + agent.Name()
		case button.WhenFree && busy:
			return http.StatusConflict, "Busy — try again when " + agent.Name() + " is done"
		case button.Key != "":
			return d.pressTo(tab, agent, buttonKeys[button.Key])
		case button.Text == "":
			return http.StatusBadRequest, "This button types nothing yet. Set it up in setup, on your computer."
		}
		return d.toAgent(tab, agent, func(t Tab) error { return t.Type(button.Text) })
	case "effort":
		if !agent.HasEffortDial() {
			return http.StatusBadRequest, agent.Name() + " has no effort setting"
		}
		level := query.Get("level")
		if !slices.Contains(effortLevels, level) {
			return http.StatusBadRequest, "Unknown level"
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		if session = d.sessions.Get(id); session == nil {
			return http.StatusNotFound, "Session not found"
		}
		// ☢ Max says so, where the dial reaching max doesn't: the office only
		// sends its agent to the button for a press.
		if query.Get("source") == "max" && level == "max" {
			session.MaxPressedAt = nowMillis()
		}
		session.PendingEffort = level
		if level == session.Effort {
			session.PendingEffort = ""
		}
		queued := session.PendingEffort != "" && session.IsBusy()
		d.changed()
		if queued {
			return http.StatusAccepted, ""
		}
		return http.StatusNoContent, ""
	}
	return http.StatusBadRequest, "Unknown action"
}

// The most lines one request may scroll. A notch of the phone's wheel is a
// few; anything beyond is clamped, not obeyed.
const maxScroll = 200

// isRepeatedReturn: Return was sent to this session a moment ago. A second
// one this soon is a tap landing twice, and let through it would answer
// whatever the agent asks next before you've seen it.
func (d *Deck) isRepeatedReturn(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	for session, at := range d.lastReturn {
		if now.Sub(at) >= returnRepeat {
			delete(d.lastReturn, session)
		}
	}
	if _, recent := d.lastReturn[id]; recent {
		return true
	}
	d.lastReturn[id] = now
	return false
}

func (d *Deck) press(tab string, keystroke Keystroke) (int, string) {
	return d.inTab(tab, func(t Tab) error { return t.Press(keystroke) })
}

// pressMic: push to talk through Claude's own /voice, in tap mode: Space
// starts listening, Space again stops and sends. Every start is matched by
// exactly one stop. A second press while recording, or a release with
// nothing recording, is ignored: an extra Space would flip Claude's state and
// leave the two out of step.
func (d *Deck) pressMic(tab string) (int, string) {
	d.mu.Lock()
	if d.micPress != nil && time.Since(d.micPress.at) < voiceLimit {
		d.mu.Unlock()
		return http.StatusNoContent, ""
	}
	d.micPress = &micPress{tab: tab, at: time.Now()}
	d.mu.Unlock()
	status, message := d.press(tab, press(Space))
	if status >= http.StatusBadRequest {
		// Nothing started, so the release must not send a Space either.
		d.mu.Lock()
		d.micPress = nil
		d.mu.Unlock()
	}
	return status, message
}

func (d *Deck) releaseMic() (int, string) {
	d.mu.Lock()
	started := d.micPress
	d.micPress = nil
	d.mu.Unlock()
	// After the limit Claude has stopped by itself, and a Space would start
	// it again.
	if started == nil || time.Since(started.at) >= voiceLimit {
		return http.StatusNoContent, ""
	}
	return d.press(started.tab, press(Space))
}

// Focus brings a session's tab forward, as far as its terminal allows.
func (d *Deck) Focus(id string) (int, string) {
	d.mu.Lock()
	tab := ""
	if session := d.sessions.Get(id); session != nil {
		tab = session.Tab // read while locked: a hook can move the session meanwhile
	}
	d.mu.Unlock()
	if tab == "" {
		return http.StatusNotFound, ""
	}
	if isHeadless(tab) {
		return http.StatusConflict, headlessNoControls
	}
	return d.inTab(tab, func(t Tab) error { return t.Focus() })
}

// applyQueuedEffort: typed into a running turn, /effort would be queued as a
// message, so it waits for the session to be free. No hook reports a slash
// command, so the new level is assumed; the next event's effort field
// corrects it. The lock is held.
func (d *Deck) applyQueuedEffort() {
	for _, session := range d.sessions.All() {
		if session.PendingEffort == "" || session.IsBusy() {
			continue
		}
		level, tab, agent := session.PendingEffort, d.tab(session.Tab), session.Agent
		session.PendingEffort = ""
		if tab == nil {
			continue
		}
		// Queued from a goroutine of its own: the lock is held here, and the
		// job takes it once done.
		go func() {
			d.work <- func() {
				// Not into a pane its agent has left: see toAgent.
				if pane, ok := tab.(commandTab); ok {
					if running, known := pane.runs(agent); known && !running {
						return
					}
				}
				if tab.Type("/effort "+level) != nil {
					return
				}
				d.mu.Lock()
				defer d.mu.Unlock()
				session.Effort = level
				d.changed()
			}
		}()
	}
}

func nowMillis() float64 { return float64(time.Now().UnixMilli()) }

func clamp(n, limit int) int { return max(-limit, min(n, limit)) }

func lastPathComponent(path string) string {
	return (&Session{Folder: path}).Name()
}
