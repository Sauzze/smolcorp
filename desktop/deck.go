package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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
	mu sync.Mutex
	// Held through Start here's GM hire (server.go), never with mu: one at a time.
	gmHiring sync.Mutex
	access   *Access
	gmToken  string // the GM's token made in this run, in memory; a GM started by hand asks for it (terminalask.go)
	// gmVerifier is the verifier of a GM token from an earlier run (SHA-256, in hex): all the core keeps between
	// runs (gmKept), and what it checks the GM's token against while gmToken is "" (gmCheck).
	gmVerifier string
	grants     *grantStore    // the owner's asks and grants (grants.go): its own lock, never mu
	gmKept     *gmTokenKeeper // where the token is kept between runs: the Keychain, a secret store or a file (gmtokenstore.go)
	gmHand     gmByHandState
	// What terminals ask for, waiting for the owner's answer in Smolcorp's window (terminalask.go): its own lock.
	asks terminalAsks
	// Floors from other Macs (peers_mac.go): the Macs paired here and the code to join with, under mu; where the
	// Macs this one joined are kept, with their keys (the secret store); joinMu, held through a join, never with mu.
	macPeers *macPeers
	peerKeys peerKeyStore
	// The VAPID key notifications are signed with (pushkey.go): under the Mac app its own item, through the bridge.
	// Its own lock, and it waits on the store: never asked with mu held.
	pushKeys *pushKeys
	joinMu   sync.Mutex
	config   *Config
	sessions *Sessions
	// Under the Mac app: what it last said of this Mac (see hostsetup.go, hostoffice.go).
	mac macState
	// Bots working in a cloud, each at a desk of its own (see cloudDesks).
	cloud     *cloudDesks
	streams   map[*eventStream]string // each open stream, with the phone that opened it
	launchers map[string]*launcherTab
	// The looks at a launcher's window waiting for an answer, by launcher and
	// look number: see launcherlook.go.
	looks map[string]chan PaneState
	// Whether the answer to each of those said bracketed paste is on, by the same key.
	lookBracketed map[string]bool
	// What a launcher said of the phone's text it was asked to paste, by the same key, only while that paste
	// waits for it: see pasteIntoLauncher.
	lookPasted map[string]string
	// The key each session was given, so it keeps it.
	keyOf map[string]int
	// The sessions hired at a desk the owner chose: with Auto they keep it,
	// even with free desks before it.
	chosenDesk map[string]bool
	// The desk and floor chosen for an engine hire in tmux, by its tmux socket,
	// until its session appears and is seated there.
	engineSeats map[string]engineSeat
	// The floor each session was put on, from the office or hired there: 0
	// for the ground floor, 1 for 2F up to 5 for 6F. It goes before its
	// folder's rule (see FloorRule), until the session ends.
	floorOf map[string]int
	// The desk each session has above the ground floor, so it keeps it.
	seatOf map[string]Seat
	// The tab recording started in, and when. One computer, one microphone.
	micPress *micPress
	// Under the Mac app, the recording its microphone makes for the office's mic, if any (hostdictation.go);
	// and how each desk's last dictation went, by session id, for a few seconds' note, and their count.
	dictating      *dictationRun
	voiceNotes     map[string]voiceNote
	voiceNoteCount int
	// When each session was last sent Return from the phone, so a tap that
	// lands twice sends it once.
	lastReturn map[string]time.Time
	// The prompt each session was last picked a choice of from the office's window, by when it began asking, so a
	// tap that lands twice picks once (talkpick.go).
	talkPicks map[string]int64
	// When each session's input line, and its prompt's details, were last looked at from the phone: once a second.
	lastPeek, lastChoiceLook map[string]time.Time
	// The hire queue and the plan it hires from (hirequeue.go, hireplan.go).
	hq *hireQueue
	// The desks with a photo kept for them (photo.go), by session id: each one's photo folder, a random name that only
	// the deck knows, so their photos go as they end.
	photoDesks map[string]string
	// The hires whose launcher hasn't connected yet, by their token, and
	// when the last hire was made: see hire.go.
	hires    map[string]*pendingHire
	lastHire time.Time
	// Where the hire made last works, for the GM route's answer (hiredInto).
	lastPlace hiredPlace
	// t3 follows the hires in T3 Code (t3deskdeck.go); t3Desks are their desks in the office (t3desks.go).
	t3      *t3Follower
	t3Desks *t3Desks
	// A hire is under way: its agent being looked for, or its terminal opening,
	// with the lock let go. No other starts until it's done.
	hiring bool
	// The engine's stand-ins, for the tests, and the hires it is still watching: see hireengine.go.
	engine engineOptions
	// The same for the launcher's way (hirelauncher.go).
	launcherEngine launcherEngineOptions
	watching       sync.WaitGroup
	// Whether a save of the sessions is coming, its timer, and the sessions
	// from before a restart still waiting for their launcher: see saved.go.
	savePending  bool
	saveTimer    *time.Timer
	saveMu       sync.Mutex // one save of the sessions at a time, from the lock to the written file
	restoring    *pendingRestore
	sessionsFile string
	// Whether the folder picker is open, from setup.
	picking bool
	// The sessions on tabs only the Mac app drives whose agent it last said had left them, by id, to that tab: gone
	// at a second look in a row.
	agentLeft map[string]string
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
	// The coding agents a summary can be made by instead, and whether each is here and signed in.
	summaryProviders *summaryProviders
	// Whether Claude, Codex and Gemini are signed in, as setup's Accounts asks (signinstatus.go).
	signInSeen signInStatus
	// The one sign-in to Claude, Codex or Gemini setup's Accounts may have running (signinrun.go).
	logins loginRunner
	// The portable office (beta): the office on a phone from anywhere, view only. Off at every start.
	portable *portableOffice
	// What the sessions did, kept in the support folder for the GM to read.
	events *eventLog
	// The mail outbox, one per Deck (see mail_routes.go).
	mail *mailOutbox
	// The owner's words to the GM, in memory only, for a rule to quote (ownerwords.go): its own lock, taken with or without mu.
	owner ownerWords
	// A look in the bots' mailboxes is out (see botmailbox.go).
	looking sync.Mutex
	// What the GM asked the app to watch, kept in the support folder (see watchers.go).
	watchers *watchers
	// The night shift's jobs and their scheduler (nightshift.go), kept in the support folder.
	night *nightShift
	// stopWatchers ends the watchers' two loops, the mail pump and the deck's rounds, and waits for them; the app lets them run as long as the process does, the tests' deck helpers call it.
	stopWatchers func()
	// The deck's timed rounds, and the looks it starts by itself: see deckRounds.
	rounds deckRounds
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
	// Before anything is served: a start script left by a core that stopped has no watcher.
	removeLeftStartScripts()
	// Before any record is read: the one-time change of every key (B1), and the record key from the Mac app
	// (records.go), with the records it finds unsealed sealed as found at the first sealed start.
	changeKeysOnce()
	startRecords(host)
	if recordsFirst() {
		sealGuardAsFound()
	}
	// The owner's guard, read as the core starts and held from then on (guardheld.go): a desk left running across a
	// restart can't change it before the first desk start or Settings view reads it.
	heldGuard(filepath.Join(supportDir, hireGuardFile))
	d := &Deck{
		access: NewAccess(), config: loadConfig(), sessions: NewSessions(),
		streams: map[*eventStream]string{}, launchers: map[string]*launcherTab{},
		keyOf: map[string]int{}, floorOf: map[string]int{}, seatOf: map[string]Seat{}, lastReturn: map[string]time.Time{}, voiceNotes: map[string]voiceNote{}, lastPeek: map[string]time.Time{}, lastChoiceLook: map[string]time.Time{}, hires: map[string]*pendingHire{}, sessionsFile: sessionsFile, work: make(chan func(), 64), OnChange: func() {},
		webhookSends: make(chan func(), webhookQueue), events: newEventLog(supportDir, eventsFileMost),
		rounds: deckRounds{stopped: make(chan struct{})},
		// The Mac app it runs under, or none, taken now: the deck reads only its own (hostsetup.go).
		mac: macState{host: host},
	}
	if settleListenScope(d.config, phonesKnown(d.access.Phones), runtime.GOOS) || recordsFirst() {
		d.config.save()
	}
	// The GM's token kept from the last run (gmtokenstore.go): a GM hired before it goes on working. Read before
	// anything is served, without d.mu; then, under the Mac app, what is known of a GM started by hand (gmbyhand.go),
	// and whatever the token is now is kept, but over an item that couldn't be read. A GM started by hand asks again
	// after a restart (smolcorp gm login): with no file to win over the store, a store that refused a fired GM's
	// replacement would otherwise give that GM's token back (1.26, SK2).
	d.gmKept = newGMTokenKeeper(gmStoreFor(d.mac.host))
	d.gmVerifier = d.gmKept.restore(readRegistry(registryPath(), false).gmIsHired())
	d.loadByHandGM()
	if !d.gmKept.readFailed() {
		d.keepGMToken()
	}
	d.grants = newGrantStore(filepath.Join(supportDir, "grants.jsonl"))
	d.mail = d.newDeckOutbox()
	d.hq = d.newHireQueue()
	// The folders are taken now, not at each look, so a look never reads a variable a test changes.
	registry, support := registryPath(), supportDir
	d.watchers = newWatchers(support, homeDir,
		func() []string {
			return watchRoots(readRegistry(registry, false).hires, filepath.Join(support, "hires"))
		},
		func() []Hire { return readRegistry(registry, false).hires },
		d.events.append, d.events.snapshot())
	// The app's own watches: wanted while the manager and the setting are on; a question is a session asking in a folder.
	d.watchers.setBuiltins(func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.config.ManagerEnabled && d.config.BuiltinWatches
	}, func() []string {
		d.mu.Lock()
		defer d.mu.Unlock()
		var folders []string
		for _, s := range d.sessions.All() {
			if s.State == Asking && s.Folder != "" {
				folders = append(folders, plainPath(s.Folder))
			}
		}
		return folders
	})
	d.stopWatchers = d.startWatchers()
	d.cloud = newCloudDesks(d.config.CloudBots, &d.mu)
	d.cloud.onChange = d.changed
	// The hires in T3 Code and their desks, before anything lays the office out.
	d.t3 = newT3Follower()
	d.t3Desks = newT3Desks(registry)
	// Floors from other Macs: those paired here, from the support folder; those joined, in the secret store.
	d.macPeers = loadMacPeers(macPeersFile())
	d.settleRecords()
	d.peerKeys = peerKeyStoreFor(d.mac.host)
	d.pushKeys = newPushKeys(pushKeyStoreFor(d.mac.host))
	// The night shift's jobs, read now. Its scheduler is started by the runner (N2): until then nothing falls due.
	d.night = newNightShift(filepath.Join(support, nightFileName), nil)
	// Under the Mac app, a pasted key is kept in its Keychain (hostshell.go).
	d.summaryKeys = newSummaryKeys(summaryStore(d.mac.host), os.Getenv)
	d.summaries = newSummaries(d.summaryKeys.Read)
	d.summaryProviders = newSummaryProviders()
	d.summaries.agent = d.runSummaryAgent
	d.summaries.savedKey = d.summaryKeys.Saved
	// Under the Mac app, the portable office's keys are its Keychain items, and it tells when you're back (hostshell.go).
	d.portable = newPortableOffice(viewerStoreFor(d.mac.host), d.config.Viewer, func(hideNames bool, alias func(string) string) map[string]any {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.viewerSnapshot(hideNames, alias)
	})
	d.portable.mac = macPresenceOf(d.mac.host)
	go d.sendWebhooks()
	// What's running as it starts is where the webhook starts from, so the first change after it is news
	d.mu.Lock()
	d.tellWebhook()
	d.mu.Unlock()
	d.restoreSessions()
	go func() {
		for job := range d.work {
			job()
		}
	}()
	d.rounds.every(pingInterval, func() {
		d.mu.Lock()
		for stream := range d.streams {
			stream.send(nil)
		}
		d.mu.Unlock()
	})
	d.rounds.every(pruneInterval, func() {
		d.pruneClosedTabs()
		d.pruneGoneAgents()
	})
	d.rounds.every(handOnInterval, func() {
		d.mu.Lock()
		if d.sessions.EndHandOns(nowMillis()) {
			d.changed()
		}
		d.mu.Unlock()
	})
	d.rounds.every(musicInterval, d.followMusic)
	// The photos from the last run, now, before the deck is anyone's (their folders' names went with it), then those
	// past their time as they come.
	clearPhotos(supportDir)
	d.rounds.every(photoSweepEvery, d.sweepPhotos)
	// The mail pump, after d.cloud is set; the watchers' stop ends it too, and the deck's rounds.
	stopWatching, stopMail := d.stopWatchers, d.startMailPump()
	d.stopWatchers = func() { stopMail(); d.rounds.stop(); stopWatching() }
	// Under the Mac app, its dictation languages and the one it suggests, asked once now, so the
	// office has a language before setup is first opened.
	if d.mac.host != nil {
		d.rounds.run(func() { d.macFactsNow() })
		// The Grok Bot app's bots, at their desks: found now, and looked for again every minute.
		d.rounds.run(func() { d.discoverBots(false) })
		d.rounds.every(botsDiscoverEvery, func() { d.discoverBots(false) })
	}
	// The hires in T3 Code, followed with the deck's rounds, so they stop with it.
	d.startT3Follow()
	return d
}

// deckRounds are what a deck runs by itself: its timed rounds, and the looks it
// starts after a control. The app lets them run as long as the process does;
// the tests stop them with their deck (stopWatchers), so a deck left from one
// test never reads the stand-ins the next one puts in.
type deckRounds struct {
	mu      sync.Mutex
	stopped chan struct{} // closed by stop
	ended   bool
	running sync.WaitGroup
}

// run runs job on a goroutine of its own, unless the rounds have stopped.
func (r *deckRounds) run(job func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.ended {
		r.running.Go(job)
	}
}

// every runs job every interval, until the rounds stop.
func (r *deckRounds) every(interval time.Duration, job func()) {
	r.run(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-r.stopped:
				return
			case <-ticker.C:
			}
			job()
		}
	})
}

// after runs job once, wait from now, unless the rounds stop first.
func (r *deckRounds) after(wait time.Duration, job func()) {
	r.run(func() {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-r.stopped:
		case <-timer.C:
			job()
		}
	})
}

// stop ends the rounds, and waits for one under way. It may be called more than once.
func (r *deckRounds) stop() {
	r.mu.Lock()
	if !r.ended {
		r.ended = true
		close(r.stopped)
	}
	r.mu.Unlock()
	r.running.Wait()
}

// followMusic looks whether music is playing, with the jukebox on, and tells
// the phones when that changes.
func (d *Deck) followMusic() {
	d.mu.Lock()
	enabled := d.config.MusicEnabled
	d.mu.Unlock()
	// A test copy never asks the music players (on a Mac, an Automation prompt), even with the jukebox on in a
	// settings file it was given (RWTC N2).
	if !enabled || testCopyPort() != 0 {
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
		// The Mac app's no, in its words: its keys need Accessibility, as the deck's other keys do.
		var refusal appRefusal
		if errors.As(err, &refusal) {
			return http.StatusForbidden, refusal.Error()
		}
		return http.StatusInternalServerError, "Couldn't reach the music here."
	}
	// The player takes a moment to say it has.
	d.rounds.after(600*time.Millisecond, d.followMusic)
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
	d.rounds.run(d.followMusic)
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
	// An acp: id or tab is an ACP desk's (acpdesk.go), which only its desk speaks for: a hook event naming one is
	// dropped whole, before Apply, so it neither sets that session's pid or parent nor files a line in its name.
	if id, _ := canonical["session_id"].(string); strings.HasPrefix(id, acpDeskPrefix) || strings.HasPrefix(tab, acpDeskPrefix) {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	// A hire in T3 Code's own Claude Code, heard through your hooks: never a desk, nor anyone else's. Its events
	// are kept in the event log, as any session's the deck has no desk for.
	if d.isT3HiresOwn(canonical, startedIn) {
		d.logEvent(canonical, agent, at)
		return
	}
	if tab == "" {
		tab = d.launcherFor(canonical, agent)
	}
	changed := d.sessions.Apply(canonical, agent, tab, startedIn, at)
	d.logEvent(canonical, agent, at)
	// A prompt of the GM's session: the owner's words where the core didn't type them (ownerwords.go). Never logged.
	d.heardInTerminal(canonical)
	// A photo its desk has just read goes at once (photo.go).
	d.notePhotoRead(canonical)
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

// logEvent keeps a line of the event log for a hook event of a session, where it
// is one of the kinds kept. With the lock held.
func (d *Deck) logEvent(event map[string]any, agent Agent, at float64) {
	id, _ := event["session_id"].(string)
	if id == "" {
		return
	}
	folder, _ := event["cwd"].(string)
	if session := d.sessions.Get(id); session != nil && session.Folder != "" {
		folder = session.Folder
	}
	if line, ok := eventFor(event, string(agent), folder, at); ok {
		d.events.append(line)
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
	d.forgetEndedPhotos()
	d.keepT3Seats()
	state := d.snapshot()
	for stream := range d.streams {
		stream.send(state)
	}
	go d.OnChange()
	d.scheduleSave()
	d.tellWebhook()
	d.portable.Changed()
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
// have no keys: they sit upstairs (see upstairs). Cloud bots on the ground
// floor take free desks as any new session does.
func (d *Deck) layout() ([]deckKey, []*Session) {
	remaining := slices.DeleteFunc(d.sessions.All(), (*Session).Headless)
	bots, sitting := d.cloud.seated()
	for i, s := range bots {
		if d.cloudFloor(sitting[i]) == 0 {
			remaining = append(remaining, s)
		}
	}
	// A hire in T3 Code on the ground floor takes a desk as anyone does: the one the registry kept for it, if it's free.
	for _, one := range d.t3Seated() {
		if t3FloorOf(one.hire) != 0 {
			continue
		}
		remaining = append(remaining, one.session)
		if _, has := d.keyOf[one.session.ID]; !has && one.hire.T3Desk != nil && *one.hire.T3Desk >= 0 && *one.hire.T3Desk < mostKeys {
			d.keyOf[one.session.ID] = *one.hire.T3Desk
			d.setChosen(one.session.ID)
		}
	}
	// A session that took another's place in its tab, as after /clear, takes its key and floor too.
	for new, old := range d.sessions.TakeReplacements() {
		if index, ok := d.keyOf[old]; ok {
			if _, has := d.keyOf[new]; !has {
				d.keyOf[new] = index
			}
		}
		if d.chosenDesk[old] {
			d.setChosen(new)
		}
		if floor, ok := d.floorOf[old]; ok {
			if _, has := d.floorOf[new]; !has {
				d.floorOf[new] = floor
			}
		}
		if seat, ok := d.seatOf[old]; ok {
			if _, has := d.seatOf[new]; !has {
				d.seatOf[new] = seat
			}
		}
		d.movedByHandGM(old, new)
	}
	// A hire's desk goes to the first session in the launcher it opened. It's
	// kept for it below if it's still free; if it was taken in the meantime,
	// the hire sits at the first free one, as anyone would.
	for _, s := range remaining {
		kind, id, socket := splitTab(s.Tab)
		if tab, ok := d.launchers[id]; kind == "launcher" && ok && tab.hired {
			if tab.floor > 0 {
				// Hired upstairs: on that floor, at that desk if it's still free.
				d.floorOf[s.ID] = tab.floor
				d.seatOf[s.ID] = Seat{tab.floor, tab.desk}
			} else {
				// Hired on the ground floor, it stays there, whatever its folder's rule says.
				d.floorOf[s.ID] = 0
				if tab.desk >= 0 {
					d.keyOf[s.ID] = tab.desk
					d.setChosen(s.ID)
				}
			}
			tab.desk, tab.hired = -1, false
		} else if _, put := d.floorOf[s.ID]; kind == "tmux" && d.hiredRoleAt(socket) != "" {
			// A hire the engine made has no launcher, and is hired on the ground
			// floor unless the office chose another: it stays there, whatever its
			// folder's rule says, until the office moves it.
			if seat, chosen := d.takeEngineSeat(socket); chosen {
				if seat.floor > 0 {
					d.floorOf[s.ID] = seat.floor
					d.seatOf[s.ID] = Seat{seat.floor, seat.desk}
				} else {
					d.floorOf[s.ID] = 0
					d.keyOf[s.ID] = seat.desk
					d.setChosen(s.ID)
				}
			} else if !put {
				d.floorOf[s.ID] = 0
			}
		}
	}
	live := map[string]bool{}
	for _, s := range remaining {
		live[s.ID] = true
	}
	// A session dropped as its tab seemed to close keeps its key and floor, should it come back.
	for id := range d.keyOf {
		if !live[id] && !d.sessions.WasDropped(id) {
			delete(d.keyOf, id)
		}
	}
	kept := func(id string) bool {
		s := d.sessions.Get(id)
		return live[id] || d.sessions.WasDropped(id) || s != nil && s.Headless()
	}
	for id := range d.chosenDesk {
		if !live[id] && !d.sessions.WasDropped(id) {
			delete(d.chosenDesk, id)
		}
	}
	for id := range d.floorOf {
		if !kept(id) {
			delete(d.floorOf, id)
		}
	}
	for id := range d.seatOf {
		_, cloud := d.cloud.botOf(id)
		if _, t3 := d.t3Desks.hireOf(id); !kept(id) && !cloud && !t3 {
			delete(d.seatOf, id)
		}
	}
	// Those put upstairs, by the office or by their folder's rule, have no key. A pinned key whose
	// project went upstairs stays theirs, empty, as it does while the project has no session.
	remaining = slices.DeleteFunc(remaining, func(s *Session) bool {
		if floor, ok := d.aboveGround(s); ok && floor > 0 {
			delete(d.keyOf, s.ID)
			delete(d.chosenDesk, s.ID)
			return true
		}
		return false
	})
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
		// A hire at a desk the owner chose keeps it, so the grid reaches it, free desks before it or not.
		var chosen []*Session
		reach := 0
		for _, s := range remaining {
			if i, ok := d.keyOf[s.ID]; ok && d.chosenDesk[s.ID] && i < mostKeys {
				chosen = append(chosen, s)
				reach = max(reach, i+1)
			}
		}
		keys = keys[:min(max(1, last, reach, pinned+len(remaining)), mostKeys)]
		for _, s := range chosen {
			if i := d.keyOf[s.ID]; i < len(keys) && keys[i].pin == nil && keys[i].session == nil {
				place(s, i)
			}
		}
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

// setChosen records that a session sits at a desk the owner chose. With the
// lock held.
func (d *Deck) setChosen(id string) {
	if d.chosenDesk == nil {
		d.chosenDesk = map[string]bool{}
	}
	d.chosenDesk[id] = true
}

// engineSeat is the desk (and floor) an engine hire in tmux was hired at.
type engineSeat struct {
	desk, floor int
	at          time.Time
}

// seatEngineHire keeps the desk chosen for the engine hire whose tmux server is
// socket, for its session to take when it appears. held is -1 for no choice.
// With the lock held.
func (d *Deck) seatEngineHire(socket string, held, floor int) {
	if held < 0 || socket == "" {
		return
	}
	if d.engineSeats == nil {
		d.engineSeats = map[string]engineSeat{}
	}
	for kept, seat := range d.engineSeats {
		if time.Since(seat.at) > hiredTabWait {
			delete(d.engineSeats, kept)
		}
	}
	d.engineSeats[filepath.Clean(socket)] = engineSeat{desk: held, floor: floor, at: time.Now()}
}

// takeEngineSeat gives, once, the desk chosen for the engine hire at socket.
// With the lock held.
func (d *Deck) takeEngineSeat(socket string) (engineSeat, bool) {
	key := filepath.Clean(socket)
	seat, ok := d.engineSeats[key]
	if ok {
		delete(d.engineSeats, key)
	}
	return seat, ok
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
	// Under the Mac app, what the desks' terminals let the office do, asked once for new tabs (hostoffice.go).
	d.askAbilities()
	keys, overflow := d.layout()
	entries := make([]map[string]any, len(keys))
	for i, key := range keys {
		entry := map[string]any{"pin": nil}
		if key.pin != nil {
			entry["pin"] = lastPathComponent(*key.pin)
		}
		if s := key.session; s != nil {
			if bot, ok := d.cloud.botOf(s.ID); ok {
				entry["session"] = d.cloudEntry(s, bot)
			} else if hire, ok := d.t3Desks.hireOf(s.ID); ok {
				entry["session"] = d.t3Entry(s, hire)
			} else {
				entry["session"] = d.sessionEntry(s)
			}
		}
		entries[i] = entry
	}
	// The six floors, the ground floor's desks being the keys, and how many
	// there's no floor for; and the second floor alone, for phones from before the others.
	floors, floorsOverflow, upstairs := d.floorsState(overflow)
	fields := map[string]any{
		"keys": entries, "overflow": len(overflow), "upstairs": upstairs,
		"floors": floors, "floorsOverflow": floorsOverflow,
		"theme": currentTheme(d.config), "scroll": d.config.ScrollEnabled,
		// Whether the office asks before max effort.
		"maxConfirm": d.config.MaxConfirm,
		// What the office's held desks offer: firing, and hiring once there's a folder to hire into, by role.
		"hiring": d.hiringState(),
		"music":  d.musicState(),
		"layout": d.config.Layout,
		// The phone's text: on, and whether an older page's goes with Return (see phonetext.go).
		"phoneMic": map[string]any{"on": d.config.PhoneMicEnabled, "autoSend": d.config.PhoneMicAutoSend},
		// This computer's clock, so the office can count turns on the phone's,
		// whichever of the two is right.
		"now": int64(nowMillis()),
	}
	d.addMacOffice(fields)
	state, _ := json.Marshal(fields)
	return state
}

// sessionEntry is what the phone is told of a session at a desk.
func (d *Deck) sessionEntry(s *Session) map[string]any {
	effort := s.Effort
	if s.PendingEffort != "" {
		effort = s.PendingEffort
	}
	entry := map[string]any{
		"id": s.ID, "name": s.Name(), "project": s.ProjectName(), "state": s.State, "effort": effort,
		"pending": s.PendingEffort != "", "agent": s.Agent, "agentName": s.Agent.Name(),
		"dial": s.Agent.HasEffortDial(), "mic": s.Agent.HasVoice(),
		"activity": orNull(s.Activity()), "tool": orNull(s.Tool), "model": orNull(s.Model),
		"subagents":    append([]Subagent{}, s.Subagents...),
		"maxPressedAt": millisOrNull(s.MaxPressedAt), "compacting": s.Compacting,
		"messagedAt": millisOrNull(s.MessagedAt), "messaged": orNull(s.Messaged),
		"turnStartedAt": millisOrNull(s.TurnStartedAt), "errorKind": orNull(s.ErrorKind),
		// When the session started, kept through /clear: its name tag counts from it.
		"startedAt": int64(s.StartedAt),
		// ✨ Summary: offered only when it's on, with a key, and a transcript to read.
		"summary": s.Transcript != "" && d.summaryOffered(),
		// ▤ Conversation: whether there's one to read, in the office's own window.
		"conversation": s.Transcript != "",
		"role":         d.roleOf(s),
		// Whether the phone's text can be typed here: on, and a terminal this app types into.
		"phoneMic": d.config.PhoneMicEnabled && typesText(s.Tab),
		// How its last dictation went, for a moment (hostdictation.go): none where nothing dictates.
		"voiceNote": d.voiceNoteOf(s.ID),
	}
	d.addMacControls(entry, s)
	return entry
}

// summaryState is ✨ Summary for the setup page: on or off, what makes
// summaries and each way there is, with why it can't when it can't; the
// key's source (never any part of the key), whether there's a secret
// store to keep one in, and the model, for the dialog. With the lock held.
func (d *Deck) summaryState() map[string]any {
	d.summaryProviders.Refresh(false)
	return summaryStateOf(d.config.SummaryOptIn, d.config.SummaryProvider, d.summaryKeys, d.summaryProviders.States(),
		d.summaryProviders.Checking(), d.summaries.LastFallback())
}

func summaryStateOf(optIn bool, saved string, keys *summaryKeys, states map[summaryProvider]providerState, checking bool,
	fallback string) map[string]any {
	source := keys.Shown()
	hasKey := source != ""
	var key any
	if hasKey {
		key = map[string]any{"source": source}
	}
	var providers []map[string]any
	for _, agent := range summaryAgents(runtime.GOOS) {
		state, known := states[agent]
		kind, reason := string(state.kind), state.reason
		if agent.supported() && state.works() {
			kind = string(providerReady)
		}
		if !known {
			kind, reason = "checking", "checking…"
		}
		providers = append(providers, map[string]any{"id": agent, "name": agent.name(), "state": kind, "reason": reason})
	}
	keyState, keyReason := "missing", "add a key below"
	if hasKey {
		keyState, keyReason = "ready", providerAPIKey.plan()
	} else if keys.NoStore() {
		keyReason = "no secret store here: set ANTHROPIC_API_KEY in Smolcorp's environment instead"
	}
	providers = append(providers, map[string]any{"id": providerAPIKey, "name": providerAPIKey.name(), "state": keyState, "reason": keyReason})
	var provider any
	chosen := chosenSummaryProvider(saved, optIn, states, hasKey)
	if chosen != "" {
		plan := chosen.plan()
		if state, ok := states[chosen]; ok && state.reason != "" {
			plan = state.reason
		}
		provider = map[string]any{"id": chosen, "name": chosen.name(), "plan": plan, "destination": chosen.destination(),
			"works": summaryProviderWorks(chosen, states, hasKey)}
	}
	var lastFallback any
	if fallback != "" {
		lastFallback = fallback
	}
	return map[string]any{
		"enabled": optIn && chosen != "", "provider": provider, "providers": providers, "checking": checking,
		"fallback": lastFallback, "key": key, "model": summaryModelName, "noStore": keys.NoStore(),
		// Only a key pasted here is a fallback: never one from Smolcorp's environment.
		"savedKey": source == "store",
		// What was picked, for saying when it's one that isn't supported any more.
		"saved": saved,
	}
}

// summaryProvider is what makes summaries now, or "". With the lock held.
func (d *Deck) summaryProvider() summaryProvider {
	return chosenSummaryProvider(d.config.SummaryProvider, d.config.SummaryOptIn, d.summaryProviders.States(), d.summaryKeys.Present())
}

// summaryRefusal is why provider can't make summaries now, or "" when it can.
func (d *Deck) summaryRefusal(provider summaryProvider) string {
	if provider == providerAPIKey {
		if d.summaryKeys.Present() {
			return ""
		}
		return "Add your Anthropic API key first."
	}
	state, known := d.summaryProviders.States()[provider]
	switch {
	case !known:
		return "Still looking for " + provider.name() + ". Try again in a moment."
	case !provider.supported() || !state.works():
		return provider.name() + ": " + state.reason + "."
	}
	return ""
}

// runSummaryAgent is the coding agent's run, unless it's known not to take Smolcorp's flags any more.
func (d *Deck) runSummaryAgent(provider summaryProvider, system, prompt string) (string, cliFailure) {
	if !provider.supported() {
		return "", cliUnsupported
	}
	state := d.summaryProviders.StatesNow()[provider]
	switch state.kind {
	case providerChanged:
		return "", cliChanged
	case providerUnsupported:
		return "", cliUnsupported
	}
	program := state.program
	if program == "" {
		program = summaryProgram(provider)
	}
	// Checked again right before it runs: the file may have changed since setup looked (agentcheck.go).
	if program == "" || checkAgentProgram(string(provider), program) != nil {
		return "", cliMissing
	}
	expected := state.kind
	if expected == "" {
		expected = providerReady
	}
	text, failure := runSummaryCLI(provider, program, system, prompt, os.Environ(), expected)
	// What setup shows is looked at again, as it may be out of date.
	if failure == cliLoginChanged || failure == cliUnsupported || failure == cliSignedOut {
		d.summaryProviders.Refresh(true)
	}
	return text, failure
}

// summaryOffered is whether ✨ summary is offered at all: turned on, and with
// something to make it. With the lock held.
func (d *Deck) summaryOffered() bool {
	return d.config.SummaryOptIn && d.summaryProvider() != ""
}

// Summarize is a few lines on what session id has been doing, from the end of
// its transcript. Nothing is sent with Summary off, or with nothing to make it.
func (d *Deck) Summarize(id string) (string, *SummaryError) {
	d.mu.Lock()
	if !d.config.SummaryOptIn {
		d.mu.Unlock()
		return "", errSummaryOff
	}
	provider := d.summaryProvider()
	if provider == "" {
		d.mu.Unlock()
		return "", errSummaryNoWay
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
	return d.summaries.SummarizeWith(id, path, project, provider)
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

// upstairs is the second floor: its desks (see floorsState), and how many sit
// above it or have no floor. Its desks show what each is doing, with every
// control off: there is nothing to type into. With the lock held.
func (d *Deck) upstairs() map[string]any {
	_, overflow := d.layout()
	_, _, upstairs := d.floorsState(overflow)
	return upstairs
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
	phone := d.streams[stream]
	delete(d.streams, stream)
	// Its last stream gone, as it sleeps or leaves the Wi-Fi: dictation it started stops.
	d.dictationPhoneLeft(phone)
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
// once there's a folder to hire into and somewhere to open it, by role: the
// folder for new hires, or a floor upstairs with a folder of its own, which
// is where a hire there starts. With the lock held.
func (d *Deck) hiringState() map[string]any {
	folder := hasHireFolder(d.config)
	terminals, tmux := hireWays()
	canHire := len(terminals) > 0 || tmux // canHireHere, with the ways kept for the fence's lines
	hire := d.config.HiringEnabled && folder && canHire
	roles := []map[string]any{}
	state := map[string]any{"fire": d.config.HiringEnabled, "hire": hire}
	if hire {
		// A hire that won't be fenced says so in the office as it's made: each role's line, and Anyone's.
		for _, role := range d.config.Roles {
			phone := role.forPhone()
			if note := d.unfencedNote(role.ID, role.Agent, terminals, tmux); note != "" {
				phone["unfenced"] = note
			}
			roles = append(roles, phone)
		}
		if note := d.unfencedNote("", d.config.HireAgent, terminals, tmux); note != "" {
			state["unfenced"] = note
		}
	}
	state["roles"] = roles
	if d.config.HiringEnabled && !folder && canHire {
		state["reason"] = hiringNeedsFolder
	}
	return state
}

// hiringNeedsFolder is why a free desk offers no hiring when it's on but
// there's no folder to hire into, where hiring could open at all.
const hiringNeedsFolder = "Hiring needs a folder: choose one in setup."

// hasHireFolder says whether there's a folder to hire into: the one for
// new hires, or a floor upstairs with a folder of its own.
func hasHireFolder(config *Config) bool {
	_, folder := existingFolder(config.HireFolder)
	for floor := 1; !folder && floor < floorCount; floor++ {
		_, folder = existingFolder(floorFolder(config.FloorRules, floor))
	}
	return folder
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
	if role, ok := d.sessionRole(s); ok {
		return role.forPhone()
	}
	return nil
}

// sessionRole is the role a session's launcher was started with, while
// setup has it.
func (d *Deck) sessionRole(s *Session) (Role, bool) {
	kind, id, socket := splitTab(s.Tab)
	roleID := ""
	if tab, ok := d.launchers[id]; kind == "launcher" && ok {
		roleID = tab.role
	} else if kind == "tmux" {
		roleID = d.hiredRoleAt(socket) // a hire the engine made: no launcher, its registry row has the role
	} else if hire, ok := d.t3Desks.hireOf(s.ID); ok {
		roleID = hire.Role // a hire in T3 Code's desk: its registry row has the role
	} else if s.ID == d.byHandGMSession() {
		roleID = gmRole // a GM started by hand was never hired, but has the role (Deck.roleID)
	}
	if roleID != "" {
		for _, role := range d.config.Roles {
			if role.ID == roleID {
				return role, true
			}
		}
	}
	return Role{}, false
}

// launcherConnected keeps a launcher that has connected. hire is the token it
// was hired with, if the office hired it, which brings the desk it was hired at.
// An id whose launcher is still connected is not given to a second one: it
// gets nil, and neither replaces the tab nor learns its secret. The id is free
// again when launcherGone has run.
func (d *Deck) launcherConnected(id string, agent Agent, folder, role, hire string) *launcherTab {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, taken := d.launchers[id]; taken {
		return nil
	}
	tab := &launcherTab{agent: agent, folder: folder, role: role, desk: -1, commands: make(chan launcherCommand, 32),
		screenSecret: randomHex(16), gone: make(chan struct{})}
	if hire != "" {
		tab.office = true
		tab.desk, tab.floor, tab.hireID, tab.hired = d.hiredPending(hire)
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
	close(tab.gone) // a look still waiting on it stops waiting
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
	// Under the Mac app, what macOS alone can do goes to it: see macTab.
	if tab := hostTab(name, kind, id, socket); tab != nil {
		return tab
	}
	return commandTab{kind: kind, id: id, socket: socket}
}

// pruneClosedTabs drops sessions whose pane has closed, and under the Mac app those in a Terminal, iTerm2 or
// Ghostty tab whose agent has left it, seen twice in a row, as the Swift engine's Deck.pruneClosedPanes did: such a
// tab's tty may be another tab's next, and the fire of a hire in a window of its own goes by its session's tab.
// Launchers say so themselves, as their connection drops.
func (d *Deck) pruneClosedTabs() {
	d.mu.Lock()
	panes := map[string]commandTab{}
	var ids, appTabs []string
	var agents []Agent
	for _, s := range d.sessions.All() {
		tab := d.tab(s.Tab)
		if pane, ok := paneOf(tab); ok {
			panes[s.Tab] = pane
		} else if _, ok := tab.(macTab); ok {
			ids, appTabs, agents = append(ids, s.ID), append(appTabs, s.Tab), append(agents, s.Agent)
		}
	}
	h := host
	d.mu.Unlock()
	d.pruneAgentsLeft(h, ids, appTabs, agents)

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

// pruneAgentsLeft is pruneClosedTabs' look at the tabs only the Mac app drives: read only, and unknown is not gone.
// ids are the sessions on tabs, by which they're judged and dropped: one that has taken a tab between the look and
// the drop is another session, and stays.
func (d *Deck) pruneAgentsLeft(h hostBridge, ids, tabs []string, agents []Agent) {
	if h == nil || len(tabs) == 0 {
		d.mu.Lock()
		d.agentLeft = nil
		d.mu.Unlock()
		return
	}
	ended, err := hostAgentsEnded(h, tabs, agents)
	if err != nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	left, gone := map[string]string{}, map[string]string{}
	for i, id := range ids {
		switch {
		case !ended[i]:
		case d.agentLeft[id] == tabs[i]:
			gone[id] = tabs[i] // seen twice in a row
		default:
			left[id] = tabs[i]
		}
	}
	d.agentLeft = left
	if d.sessions.PruneIDs(gone, nowMillis()) {
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
	// On a test copy, nothing in a tab that isn't its own (RWTC N1): each way in says so first, and this keeps any other out.
	if why := testCopyKeepsTab(name); why != "" {
		return http.StatusConflict, why
	}
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
		// Under the Mac app, as its Swift engine says it for a terminal Smolcorp can only watch (Deck.notHere);
		// Linux and Windows keep their own sentence.
		if d.mac.host != nil {
			return http.StatusConflict, notHere
		}
		return http.StatusNotFound, "This terminal can't do that"
	case errors.Is(err, errNotStopped):
		return http.StatusConflict, "Couldn't stop the agent, so its tab is left open."
	case errors.Is(err, errAgentBehind):
		return http.StatusConflict, notFrontFg
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

// What the phone is told elsewhere: in the Mac's words under the Mac app (macWords).
const (
	pairCodeRefusal     = "That code isn't valid. Get a new one on your computer."
	settingsElsewhere   = "Settings open on your computer."
	hireWindowElsewhere = "Hires' windows open on your computer."
	t3Elsewhere         = "T3 Code opens on your computer."
	scrollingOff        = "Scrolling is off. Turn it on in setup, on your computer."
	buttonTypesNothing  = "This button types nothing yet. Set it up in setup, on your computer."
	hireFolderFirst     = "Choose a folder for new hires in setup, on your computer."
)

var errAgentGone = errors.New("the agent no longer runs in its pane")

// toAgent runs op on the session's tab once its agent is known to run there
// still. An agent that ended without saying so, crashed or killed, leaves its
// pane to the shell, where its keys would land and a button's text would run
// as a command: its session goes instead. Launchers end with their agent,
// so only panes are asked.
func (d *Deck) toAgent(name string, agent Agent, op func(Tab) error) (int, string) {
	return d.inTab(name, func(t Tab) error {
		if pane, ok := paneOf(t); ok {
			if running, known := pane.runs(agent); known && !running {
				return errAgentGone
			}
		}
		return op(t)
	})
}

// typeInto types text into the tab named name, with Enter: every Type of the core's into a desk goes through it,
// so what it types into the GM's session, a made button's text whoever wrote it, is noted as the core's and never
// kept as the owner's words (coreTypes, ownerwords.go). On the key worker, without the lock.
func (d *Deck) typeInto(name string, t Tab, text string) error {
	d.coreTypes(name, text)
	return t.Type(text)
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
	// On the floor shown: the ground floor's keys, with no floor given, as from a page before floors had pins; or
	// a floor's desks upstairs.
	floor := 0
	if given := query.Get("floor"); given != "" {
		var err error
		if floor, err = strconv.Atoi(given); err != nil || floor < 0 || floor >= floorCount {
			return http.StatusBadRequest, "Unknown floor"
		}
	}
	keys, overflow := d.layout()
	if floor > 0 {
		seated, _ := d.seatedBuilding(overflow)
		keys = make([]deckKey, desksPerFloor)
		for desk := range keys {
			keys[desk].pin = d.config.pinAt(floor, desk)
		}
		for _, one := range seated[floor] {
			keys[one.desk].session = one.person
		}
	}
	a, errA := strconv.Atoi(query.Get("from"))
	b, errB := strconv.Atoi(query.Get("to"))
	if errA != nil || errB != nil || a == b || a < 0 || b < 0 || a >= len(keys) || b >= len(keys) {
		return http.StatusBadRequest, "Unknown desks"
	}
	// A hire in T3 Code's desk is no project's to pin: it stays where it sits.
	for _, i := range []int{a, b} {
		if s := keys[i].session; s != nil {
			if hire, ok := d.t3Desks.hireOf(s.ID); ok {
				return http.StatusConflict, calledHire(hire) + " runs in T3 Code: its desk stays where it is."
			}
		}
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
	d.config.SwapPinsOn(floor, first, a, second, b)
	d.changed()
	return http.StatusNoContent, ""
}

// dictationLanguages are the languages the Mac's dictation takes (Dictation.languages).
var dictationLanguages = []string{"fr_FR", "en_US"}

// Perform runs one of the phone's controls on a session.
func (d *Deck) Perform(action, id string, query url.Values) (int, string) {
	return d.PerformFrom("", action, id, query)
}

// PerformFrom runs one of the phone's controls on a session, for phone: the mic's dictation stops when
// the phone that holds it goes away.
func (d *Deck) PerformFrom(phone, action, id string, query url.Values) (int, string) {
	// Releasing the mic stops recording where it started, whatever is
	// selected now — and even if that session has since gone.
	if action == "mic" && query.Get("phase") != "down" {
		// Dictating, or getting ready to: the text goes into the tab it was for, without Return.
		if d.releaseDictation() {
			return http.StatusNoContent, ""
		}
		return d.releaseMic()
	}
	// The mic's other quick switch: Claude's clean-up on or off, for every phone.
	if action == "cleanup" {
		return d.setCleanup(query.Get("on"))
	}
	d.mu.Lock()
	hiring := d.config.HiringEnabled
	d.mu.Unlock()
	if action == "swap" {
		return d.swapKeys(query)
	}
	// Someone put on another floor, from the office: any session, a headless run or a cloud bot.
	if action == "move" {
		return d.move(id, query.Get("floor"), query.Get("remember") == "1")
	}
	// A cloud bot's card opened: its news seen.
	if action == "seen" {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.cloud.seen(id) {
			return http.StatusNoContent, ""
		}
		return http.StatusNotFound, ""
	}
	if action == "style" {
		return d.setStyle(query.Get("preset"))
	}
	// The mic's quick switch, as the Mac answers it: a language it takes is 204, any other 400. Under the Mac
	// app it is dictation's language for every phone, kept, saved and sent as the Mac's Deck does, checked
	// against the languages that Mac has (as setup's /setup/dictation). On Linux and Windows there is no
	// dictation: nothing is stored and nothing is started, so a known one only says so.
	if action == "language" {
		lang := query.Get("lang")
		d.mu.Lock()
		defer d.mu.Unlock()
		known := dictationLanguages
		if d.mac.host != nil && d.mac.languages != nil {
			known = d.mac.languages
		}
		if !slices.Contains(known, lang) {
			return http.StatusBadRequest, "Unknown language"
		}
		if d.mac.host != nil {
			d.config.DictationLanguage = lang
			d.config.save()
			d.changed()
		}
		return http.StatusNoContent, ""
	}
	if action == "music" {
		// A test copy plays nothing and leaves the volume as it is: they're the owner's (RWTC N2).
		if why := testCopyKeepsAway(testCopyKeepsMusic); why != "" {
			return http.StatusConflict, why
		}
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
		// A new hire has no session yet. With hiring off the answer is that, whatever desk was named (403, as the
		// Mac's hire says it first).
		if hiring && query.Has("desk") && query.Get("desk") == "" {
			return http.StatusBadRequest, "Unknown desk"
		}
		if floor := query.Get("floor"); floor != "" && floor != "0" {
			return d.hireUpstairs(floor, query.Get("desk"), query.Get("role"))
		}
		return d.hire(query.Get("desk"), query.Get("role"))
	}
	// A hire in T3 Code's desk: Let go, the lane's, from its card; nothing else reaches it, as there's nothing to type into.
	if action == "letgo" {
		if status, answer, ok := d.letGoAtDesk(id); ok {
			if text, isText := answer.(string); isText {
				return status, text
			}
			data, _ := json.Marshal(answer)
			return status, string(data)
		}
	}
	d.mu.Lock()
	if hire, ok := d.t3HireAtDesk(id); ok {
		d.mu.Unlock()
		return http.StatusConflict, t3NoControls(calledHire(hire))
	}
	session := d.sessions.Get(id)
	if session == nil || session.Tab == "" {
		d.mu.Unlock()
		return http.StatusNotFound, "Session not found"
	}
	if session.Headless() {
		d.mu.Unlock()
		return http.StatusConflict, headlessNoControls
	}
	// A test copy does nothing in the owner's terminal tabs, only in its own hires' tmux panes (RWTC N1).
	if why := testCopyKeepsTab(session.Tab); why != "" {
		d.mu.Unlock()
		return http.StatusConflict, why
	}
	tab, agent, busy := session.Tab, session.Agent, session.IsBusy()
	// Claude Code ends the turn when its prompt is put away, and no hook says so: see StoppedAsking.
	asking := session.State == Asking && agent == Claude
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
	case "reject", "esc":
		key := agent.Reject()
		if action == "esc" {
			key = agent.Interrupt()
		}
		status, message := d.pressTo(tab, agent, key)
		if asking && status < http.StatusBadRequest {
			d.mu.Lock()
			if d.sessions.StoppedAsking(id, nowMillis()) {
				d.changed()
			}
			d.mu.Unlock()
		}
		return status, message
	case "up":
		return d.pressTo(tab, agent, press(Up))
	case "down":
		return d.pressTo(tab, agent, press(Down))
	case "scroll":
		d.mu.Lock()
		enabled := d.config.ScrollEnabled
		d.mu.Unlock()
		if !enabled {
			return http.StatusForbidden, scrollingOff
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
		// Under the Mac app, dictation, as setup says, rather than the agent's own voice mode through Space.
		d.mu.Lock()
		dictates := d.dictatesFor(tab, agent)
		d.mu.Unlock()
		if dictates {
			return d.startDictation(tab, agent, phone)
		}
		if !agent.HasVoice() {
			return http.StatusBadRequest, agent.Name() + " has no voice input"
		}
		return d.pressMic(tab)
	case "fire":
		if !hiring {
			return http.StatusForbidden, hiringOff
		}
		// A hire in T3 Code is let go from Setup's team list, after a confirm that says what's removed.
		if h, found := d.hireInSession(id); found {
			if why := t3FireRefusal(h); why != "" {
				return http.StatusConflict, why
			}
		}
		if status, message, handled := d.fireTab(tab, query.Get("force") == "1"); handled {
			return status, message
		}
		if status, message := d.inTab(tab, func(t Tab) error { return t.Close(agent) }); status != http.StatusNotFound {
			return status, message
		}
		return http.StatusNotFound, "That agent's tab has gone."
	case "clear":
		if busy {
			return http.StatusConflict, "Busy — clear it when " + agent.Name() + " is done"
		}
		return d.toAgent(tab, agent, func(t Tab) error { return d.typeInto(tab, t, agent.NewConversation()) })
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
			return http.StatusBadRequest, buttonTypesNothing
		}
		return d.toAgent(tab, agent, func(t Tab) error { return d.typeInto(tab, t, button.Text) })
	case "effort":
		if !agent.HasEffortDial() {
			return http.StatusBadRequest, agent.Name() + " has no effort setting"
		}
		// Set by typing /effort, which a terminal Smolcorp can only watch can't take: under the Mac app, as its
		// Swift engine answers (409). Linux and Windows keep taking it, as their desks show the dial everywhere.
		if d.mac.host != nil && !d.macDrives(tab) {
			return http.StatusConflict, notHere
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
	// A test copy brings no other app forward (RTCH N1): the Grok app, T3 Code, a terminal, as the Mac's. A desk's
	// news is seen all the same, as the tap means it was looked at: only the open is left out (RWTC N9).
	d.mu.Lock()
	// A cloud bot's desk: its news seen. (The Mac app also brings forward
	// the app it works in, which is the Mac's only.)
	if d.cloud.seen(id) {
		grok := d.mac.host != nil && d.cloud.isGrokApp(id) // the lock is held, as macHost() takes it
		d.mu.Unlock()
		if why := testCopyKeepsAway(testCopyKeepsGrok); grok && why != "" {
			return http.StatusConflict, why
		}
		return http.StatusNoContent, ""
	}
	d.mu.Unlock()
	// A hire in T3 Code's desk: its done seen, and its own thread opened in T3 Code.
	if status, text, ok := d.focusT3(id); ok {
		return status, text
	}
	d.mu.Lock()
	tab := ""
	if session := d.sessions.Get(id); session != nil {
		tab = session.Tab // read while locked: a hook can move the session meanwhile
	}
	d.mu.Unlock()
	if tab == "" {
		return http.StatusNotFound, ""
	}
	// A terminal Smolcorp can only watch, or a headless run, has nothing to bring forward: under the Mac app the
	// tap only picks the desk, as its Swift engine has it (204). Linux and Windows keep their refusals.
	if d.mac.host != nil && !d.macDrives(tab) {
		return http.StatusNoContent, ""
	}
	if isHeadless(tab) {
		return http.StatusConflict, headlessNoControls
	}
	if why := testCopyKeepsAway(testCopyKeepsTerminal); why != "" {
		return http.StatusConflict, why
	}
	return d.inTab(tab, func(t Tab) error { return t.Focus() })
}

// macDrives is whether the Mac app has a driver for the tab's terminal, which every driver of its has (keys,
// lines, bringing it forward). A terminal Smolcorp can only watch, a headless run and a name nobody knows have none,
// whatever the app is asked; for one it has a driver for, the app's own word when it has said (a tmux that isn't
// installed, a Ghostty not learned yet), else yes. Without the lock.
func (d *Deck) macDrives(name string) bool {
	kind, _, _ := splitTab(name)
	switch kind {
	case "launcher":
		return true
	case "tty", "tmux", "iterm", "ghostty", "wezterm", "kitty":
	default:
		return false
	}
	d.mu.Lock()
	can, known := d.mac.abilities[name]
	d.mu.Unlock()
	return !known || can.Keys || can.Text
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
		level, name, tab, agent := session.PendingEffort, session.Tab, d.tab(session.Tab), session.Agent
		session.PendingEffort = ""
		// Nothing typed into a tab a test copy keeps away from (RWTC N1).
		if tab == nil || testCopyKeepsTab(session.Tab) != "" {
			continue
		}
		// Queued from a goroutine of its own: the lock is held here, and the
		// job takes it once done. One of the deck's rounds, which waits for the
		// job to have run on the key worker: a test's deck stops its rounds as
		// the test ends, so the job never runs into the next test, where
		// coreTypes would read the stand-in folders it puts in (FXMEM3RACE).
		d.rounds.run(func() {
			done := make(chan struct{})
			d.work <- func() {
				defer close(done)
				// Not into a pane its agent has left: see toAgent.
				if pane, ok := paneOf(tab); ok {
					if running, known := pane.runs(agent); known && !running {
						return
					}
				}
				if d.typeInto(name, tab, "/effort "+level) != nil {
					return
				}
				d.mu.Lock()
				defer d.mu.Unlock()
				session.Effort = level
				d.changed()
			}
			<-done
		})
	}
}

func nowMillis() float64 { return float64(time.Now().UnixMilli()) }

func clamp(n, limit int) int { return max(-limit, min(n, limit)) }

func lastPathComponent(path string) string {
	return (&Session{Folder: path}).Name()
}
