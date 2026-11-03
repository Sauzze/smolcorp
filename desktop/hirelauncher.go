package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The hire engine on the launcher's way (Phase 6 T734, beta, Windows): a role's
// Claude hire made as the tmux engine makes it (hireengine.go), but in a window
// run by our launcher, `smolcorp run`, rather than tmux. The same order, each
// step ready before the next, and a refusal takes away what came before it;
// the same watch (a hook reports, or the hire is cleaned up); the same fire
// (hirefire.go). What differs is that a launcher has no start script and no
// socket: the hire's secrets reach the launcher over loopback, once, before
// it starts the agent (hirestartenv.go), and the launcher is known by the
// hire's id, which comes from the hire's one-time token, never from what the
// launcher says. When in doubt, refuse. Nothing here types or answers anything.
// It builds on every OS so it is tested on every OS; only the switch's
// default and the hand-over in hireengine_windows.go are Windows's.

// launcherEngineOptions are what the launcher engine needs from outside, each with a test stand-in.
type launcherEngineOptions struct {
	Git       gitRunner
	PortFree  func(int) bool
	Key       func() string
	Guard     guardRunner
	IsExec    func(string) bool
	CLI       string // the app's own smolcorp command
	Worktrees string
	// Open opens the hire's window, running command in folder.
	Open func(command []string, folder string) error
	// Expect tells the app a hire's launcher is coming: its one-time token, the hire's id and
	// what the launcher may ask for once. Forget takes the token away again.
	Expect    func(token, hireID string, env hireStartEnv)
	Forget    func(token string)
	HeardFrom func(Hire) bool
	Within    time.Duration
	Every     time.Duration
	// LookWait is how long a look at the window waits; StopWait how long a fire waits to see the launcher gone.
	LookWait time.Duration
	StopWait time.Duration
	// Grace is how long a window seen open may be missing before it counts as closed: a launcher
	// whose stream dropped connects again within a few seconds.
	Grace time.Duration
	Now   func() time.Time
	Pause func(time.Duration)
}

func (o launcherEngineOptions) withDefaults() launcherEngineOptions {
	if o.Git == nil {
		o.Git = runGit
	}
	if o.Key == nil {
		o.Key = func() string { key, _ := keyringStore.get(); return key }
	}
	if o.CLI == "" {
		o.CLI = commandLineProgram()
	}
	if o.Within == 0 {
		o.Within = hireStartWithin
	}
	if o.Every == 0 {
		o.Every = time.Second
	}
	if o.LookWait == 0 {
		o.LookWait = 3 * time.Second
	}
	if o.StopWait == 0 {
		o.StopWait = 5 * time.Second
	}
	if o.Grace == 0 {
		o.Grace = 3 * reconnectDelay
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Pause == nil {
		o.Pause = time.Sleep
	}
	if o.HeardFrom == nil {
		o.HeardFrom = func(Hire) bool { return false }
	}
	return o
}

// runLauncherHire makes a managed Claude hire in a window of the launcher and
// starts it, or refuses with nothing left behind: no row, key, worktree,
// pending token or GM token. The hire is in the registry as "starting".
// token is the hire's one-time token, for the watch to take away.
func runLauncherHire(r engineRequest, o launcherEngineOptions) (hire Hire, token string, err error) {
	o = o.withDefaults()
	// Before anything is made.
	if err := testCopyRefusal(r.TestPort, string(Claude), r.ConfigDir); err != nil {
		return Hire{}, "", err
	}
	if r.APIKey {
		return Hire{}, "", &hireRefusal{Status: http.StatusConflict, Message: "A hire on the API key isn't set up through the launcher yet, so none is started."}
	}
	if o.Open == nil || o.Expect == nil || o.Forget == nil {
		return Hire{}, "", &hireRefusal{Status: http.StatusInternalServerError, Message: "The hire's window can't be opened here."}
	}
	id := strings.ToLower(newID())[:12]
	place, err := prepareHire(hireRequest{HireID: id, Name: r.Role.Name, Folder: r.Folder, Claude: true, ConfigDir: r.ConfigDir,
		APIKey: r.APIKey, Worktree: true},
		hirePlaceOptions{Worktrees: o.Worktrees, Taken: takenPorts(), Git: o.Git, PortFree: o.PortFree, Key: o.Key})
	if err != nil {
		return Hire{}, "", err
	}
	undo := func() {
		if place.Worktree != "" {
			removeWorkspace(place.Repo, place.Worktree, place.Branch, o.Git)
		}
		_ = os.RemoveAll(hireFolder(supportDir, id))
	}
	fail := func(status int, message string) (Hire, string, error) {
		undo()
		return Hire{}, "", &hireRefusal{Status: status, Message: message}
	}
	profile := guardHire
	if r.Role.ID == gmRole {
		profile = guardGM
	}
	settings, err := writeHireSettings(hireSettingsOptions{HireID: id, Profile: profile, CLI: o.CLI, Support: supportDir, ReportPort: r.TestPort, Run: o.Guard, IsExec: o.IsExec})
	if err != nil {
		undo()
		return Hire{}, "", err
	}
	// The hire is written down before its key is made (a key's hash is kept against
	// its row). It has no tmux socket: its launcher is known by its id.
	hire = Hire{ID: id, Name: r.Role.Name, Role: r.Role.ID, Folder: place.Folder, Branch: place.Branch, Worktree: place.Worktree, Repo: place.Repo,
		ViaLauncher: true, Model: r.Role.Model, Ports: place.Ports, Started: float64(o.Now().UnixMilli()), State: "starting", Notes: place.Note}
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, hire) }); err != nil {
		return fail(http.StatusInternalServerError, "Couldn't save the hire in the registry.")
	}
	undoMade := undo
	undo = func() {
		forgetHire(id)
		undoMade()
	}
	sendKey, err := sendKeyFor(r.Role, id)
	if err != nil {
		return fail(http.StatusInternalServerError, "Couldn't make the hire's send key.")
	}
	gmToken := ""
	if r.Role.ID == gmRole {
		if r.GMToken != nil {
			gmToken = r.GMToken()
		}
		if gmToken == "" {
			return fail(http.StatusInternalServerError, "Couldn't make the General Manager's token.")
		}
	}
	// Unreachable today (sendKeyFor makes no key for the GM); kept so a change there can't give the GM a key.
	if gmToken != "" && sendKey != "" {
		return fail(http.StatusInternalServerError, "A hire never holds a send key with the General Manager's token.")
	}
	// What the launcher sets on the agent's process, and only there.
	env := hireStartEnv{Set: map[string]string{}, PathFirst: filepath.Dir(o.CLI)}
	if place.Ports != "" {
		env.Set["SMOLCORP_HIRE_PORTS"] = place.Ports
	}
	if sendKey != "" {
		env.Set[hireKeyEnvironment] = sendKey
	}
	if gmToken != "" {
		env.Set["SMOLCORP_GM_TOKEN"] = gmToken
	} else {
		env.Unset = append(env.Unset, "SMOLCORP_GM_TOKEN") // a hire never acts as the GM
	}
	if r.ConfigDir != "" {
		env.Set["CLAUDE_CONFIG_DIR"] = r.ConfigDir
	} else if r.DefaultAccount {
		env.Unset = append(env.Unset, "CLAUDE_CONFIG_DIR")
	}
	token = newID()
	o.Expect(token, id, env)
	// Only ids, names and the settings file's path are on the command line.
	command := []string{o.CLI, "run", "--hire", token, "--hire-id", id, "--agent", string(Claude), "--role", r.Role.ID, "--settings=" + settings}
	if err := o.Open(command, place.Folder); err != nil {
		// It was written down, so its row stays, marked failed (its key with it: a failed hire's key works no more).
		o.Forget(token)
		setHire(id, "failed", "Its window didn't open.")
		undoMade()
		return Hire{}, "", &hireRefusal{Status: http.StatusInternalServerError, Message: "The terminal didn't open."}
	}
	return hire, token, nil
}

// launcherOf is the id of the launcher that is hire hireID's window, as the
// hire's token made it (never as a launcher said), or "".
func (d *Deck) launcherOf(hireID string) (id string, tab *launcherTab) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, tab := range d.launchers {
		if tab.hireID == hireID {
			return id, tab
		}
	}
	return "", nil
}

// waitForLauncher waits for hire to report: started once a hook has, else not
// started once its window, seen open, has been missing for longer than grace
// (a launcher whose stream dropped comes back within a few seconds), or when
// within is up.
func waitForLauncher(alive func() (alive, known bool), heardFrom func() bool, within, grace, every time.Duration,
	now func() time.Time, pause func(time.Duration)) (started bool, reason string) {
	const ended = "its window closed before it reported"
	var missingSince time.Time // zero while the window is up or has not been seen
	closed := func() bool {
		up, known := alive()
		if !known || up {
			missingSince = time.Time{}
			return false
		}
		if missingSince.IsZero() {
			missingSince = now()
		}
		return now().Sub(missingSince) > grace
	}
	end := now().Add(within)
	for {
		if closed() {
			return false, ended
		}
		if heardFrom() {
			if closed() {
				return false, ended
			}
			return true, ""
		}
		if !now().Before(end) {
			return false, "no word from it in " + strconv.Itoa(int(within.Seconds())) + " seconds"
		}
		pause(every)
	}
}

// watchLauncherHire waits for hire to report, marks it working, or cleans it up.
func (d *Deck) watchLauncherHire(hire Hire, token string, o launcherEngineOptions) {
	o = o.withDefaults()
	seen := false // its window has been open at some point
	started, reason := waitForLauncher(
		func() (bool, bool) {
			if d.launcherHolds(hire) {
				seen = true
				return true, true
			}
			return false, seen
		},
		func() bool { return o.HeardFrom(hire) }, o.Within, o.Grace, o.Every, o.Now, o.Pause)
	if started {
		setHire(hire.ID, "working", hire.Notes)
		return
	}
	d.cleanUpLauncherHire(hire, token, reason, o)
}

// cleanUpLauncherHire ends a hire that never started: its token is taken away
// (a window opening late gets nothing), its launcher told to stop and seen
// gone, its worktree removed when it is new and clean and the launcher is
// gone, then it is marked failed and why.
func (d *Deck) cleanUpLauncherHire(hire Hire, token, reason string, o launcherEngineOptions) {
	o.Forget(token)
	if !d.stopLauncherOf(hire.ID, o.StopWait) {
		setHire(hire.ID, "failed", firstRunes("Didn't start: "+reason+". Its window didn't close, so its worktree was left as it is.", longestNotes))
		return
	}
	if d.launcherHolds(hire) {
		// An id-less window of this hire is still open (it connected again): not
		// failed, not stopped, nothing removed. Kept working, so the fire's refusal
		// and gmRefusal keep counting it.
		setHire(hire.ID, "working", firstRunes("Its window connected again without its id, so it couldn't be checked or ended from here. Close it yourself, or fire it from its window.", longestNotes))
		return
	}
	left := ""
	if hire.Worktree != "" && !removeLauncherHireWorktree(o.Git, o.Worktrees, d.launcherGoneFor)(hire) {
		left = " Its worktree " + hire.Worktree + " was kept: it has changes, or git wouldn't remove it."
	}
	setHire(hire.ID, "failed", firstRunes("Didn't start: "+reason+"."+left, longestNotes))
}

// hireDir is where hire h runs: its worktree, or its folder when it has none
// (a folder that isn't a git repo).
func hireDir(h Hire) string {
	if h.Worktree != "" {
		return h.Worktree
	}
	return h.Folder
}

// launcherHolds is whether a connected launcher is hire h's window: the one its
// token made, or, whatever id it carries (one that connected again has none),
// one in the hire's worktree, or its folder when it has no worktree.
func (d *Deck) launcherHolds(h Hire) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	tree := ""
	if dir := hireDir(h); dir != "" {
		tree = physical(dir)
	}
	for _, tab := range d.launchers {
		if tab.hireID == h.ID || (tree != "" && within(tree, physical(tab.folder))) {
			return true
		}
	}
	return false
}

// launcherGoneFor is whether no connected launcher is hire's window (launcherHolds).
func (d *Deck) launcherGoneFor(h Hire) bool {
	return !d.launcherHolds(h)
}

// hiresWorkingIn are the launcher engine's hires, starting or working, whose
// worktree (or folder, with none) is folder or holds it (physical paths): the hires a launcher that
// lost its hire id may be the window of.
func hiresWorkingIn(hires []Hire, folder string) []Hire {
	var rows []Hire
	if folder == "" {
		return rows
	}
	where := physical(folder)
	for _, h := range hires {
		if dir := hireDir(h); h.ViaLauncher && dir != "" && (h.State == "starting" || h.State == "working") && within(physical(dir), where) {
			rows = append(rows, h)
		}
	}
	return rows
}

// engineHireWorksIn is whether an open launcher-engine hire's worktree is folder or holds it.
func engineHireWorksIn(folder string) bool {
	return len(hiresWorkingIn(officeHires(), folder)) > 0
}

// stopLauncherOf tells hire hireID's launcher, if one is connected, to stop and
// waits up to wait to see it gone. true is only "no launcher of the hire's is connected".
func (d *Deck) stopLauncherOf(hireID string, wait time.Duration) bool {
	_, tab := d.launcherOf(hireID)
	if tab == nil {
		return true
	}
	if tab.send(launcherCommand{Stop: true}) != nil {
		return false
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-tab.gone:
		return true
	case <-timer.C:
		return false
	}
}

// removeLauncherHireWorktree is cleanUpLauncherHire's and the fire's callback:
// removeOwnWorktree's checks, for a hire the launcher engine made, whose
// id-carrying launcher is gone (where the tmux way needs its own socket).
func removeLauncherHireWorktree(git gitRunner, worktrees string, gone func(Hire) bool) func(Hire) bool {
	return func(h Hire) bool {
		if !h.ViaLauncher || !gone(h) {
			return false
		}
		return removeOwnWorktree(git, worktrees, h)
	}
}

// expectLauncher tells the deck a hire's launcher is coming: its one-time token
// (kept in memory only), the hire's id and what the launcher may ask for once.
func (d *Deck) expectLauncher(token, hireID string, env hireStartEnv) {
	d.expectLauncherAt(token, hireID, env, -1, 0)
}

// expectLauncherAt is expectLauncher for a hire at the desk held (-1 for none) on floor.
func (d *Deck) expectLauncherAt(token, hireID string, env hireStartEnv, held, floor int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for kept, pending := range d.hires {
		if time.Since(pending.at) > hiredTabWait {
			delete(d.hires, kept)
		}
	}
	d.hires[token] = &pendingHire{desk: held, floor: floor, at: time.Now(), hireID: hireID, env: &env}
}

// forgetLauncher takes a hire's token away: a window that opens late gets nothing.
func (d *Deck) forgetLauncher(token string) {
	d.mu.Lock()
	delete(d.hires, token)
	d.mu.Unlock()
}

// managedLauncherHire is hireAs's way in for the launcher: with the lock held,
// it hires role roleID through the launcher engine and says what to answer,
// or handled false: the switch is off, no terminal, or no such role, so it
// opens as it did. The lock is let go while the engine works.
func (d *Deck) managedLauncherHire(roleID, folder string, terminal hireOpener, hasTerminal bool, held, floor int) (status int, message string, handled bool) {
	i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == roleID })
	if i < 0 || !d.config.HireManagedWindows || !hasTerminal {
		return 0, "", false
	}
	role := d.config.Roles[i]
	options, testPort := d.launcherEngine, testCopyPort()
	if options.HeardFrom == nil {
		options.HeardFrom = d.hireHeardFrom
	}
	if options.Open == nil {
		options.Open = func(command []string, where string) error { return openWindow(terminal, where, inLoginShell(command)) }
	}
	// The desk chosen (held, on floor) comes with the launcher's token, as it does for any hire.
	options.Expect = func(token, hireID string, env hireStartEnv) { d.expectLauncherAt(token, hireID, env, held, floor) }
	options.Forget = d.forgetLauncher
	request := engineRequest{Role: role, How: hiredIntoGeneralFolder, Folder: folder, DefaultAccount: true, TestPort: testPort}
	// The GM gets a new token at its hire, handed to its launcher alone; the one it
	// was given is taken away again if the hire doesn't go ahead or never starts.
	gmToken := ""
	if role.ID == gmRole {
		request.GMToken = func() string { gmToken = d.newGMToken(); return gmToken }
	}
	d.mu.Unlock()
	hire, token, err := runLauncherHire(request, options)
	d.mu.Lock()
	if err != nil {
		if gmToken != "" && constantTimeEqual(d.gmToken, gmToken) {
			d.gmToken = "" // d.mu is held: clearGMToken would take it again
		}
		d.lastHire = time.Time{} // nobody was hired, so the next try needn't wait
		if refused, ok := err.(*hireRefusal); ok {
			return refused.Status, refused.Message, true
		}
		return http.StatusInternalServerError, err.Error(), true
	}
	d.watching.Add(1)
	go func() {
		defer d.watching.Done()
		d.watchLauncherHire(hire, token, options)
		if gmToken != "" && !hireIsOpen(hire.ID) {
			d.clearGMTokenIf(gmToken)
		}
	}()
	return http.StatusNoContent, "", true
}

// fireLauncherHire fires h, an engine hire whose window the launcher runs: a
// refusal says why and leaves everything as it was. The same refusals as
// fireEngineHire, in the same order and words. Otherwise its launcher is told
// to stop and seen gone, its worktree removed when clean, its recap written
// and it is marked fired. The window is read (a launcher sends its state,
// not its text, so the recap has no cost line); nothing is ever typed or
// answered in it, not even with force.
func (d *Deck) fireLauncherHire(h Hire, o launcherEngineOptions, force bool) (kept string, err error) {
	o = o.withDefaults()
	refuse := func(message string) (string, error) {
		return "", &hireRefusal{Status: http.StatusConflict, Message: message}
	}
	firing.Lock()
	if firing.ids[h.ID] {
		firing.Unlock()
		return refuse(calledHire(h) + " is being fired already.")
	}
	firing.ids[h.ID] = true
	firing.Unlock()
	defer func() { firing.Lock(); delete(firing.ids, h.ID); firing.Unlock() }()

	for _, now := range readRegistry(registryPath(), false).hires {
		if now.ID == h.ID && (now.State == "fired" || now.State == "failed") {
			return refuse(calledHire(h) + " is already gone.")
		}
	}
	launcher, tab := d.launcherOf(h.ID)
	if tab == nil || !h.ViaLauncher {
		return refuse("Couldn't reach " + calledHire(h) + "'s window, so nothing was changed.")
	}
	if h.Worktree != "" && !force {
		if message := uncommittedRefusal(h, o.Git); message != "" {
			return refuse(message)
		}
	}
	// A launcher that doesn't answer, is gone, or can't read its screen is a window that can't be looked at.
	look := func() (PaneState, bool) {
		state, err := d.lookAtLauncher(launcher, o.LookWait)
		return state, err == nil && state != PaneUnknown
	}
	state, readable := look()
	if message := stateRefusal(h, state, readable, force); message != "" {
		return refuse(message)
	}
	// Git may have taken a while: look again just before ending, so a question that came up meanwhile stops it.
	state, readable = look()
	if message := stateRefusal(h, state, readable, force); message != "" {
		return refuse(message)
	}
	// A stop that didn't end it is not a fire: the launcher must be seen gone.
	if !d.stopLauncherOf(h.ID, o.StopWait) {
		return refuse("Couldn't end " + calledHire(h) + "'s window, so nothing was changed.")
	}
	removed := h.Worktree != "" && removeLauncherHireWorktree(o.Git, o.Worktrees, d.launcherGoneFor)(h)
	writeRecap(h, "", removed, o.Now())
	if h.Worktree != "" && !removed {
		kept = "it has changes, or git wouldn't remove it"
		markFired(h.ID, "Its worktree "+h.Worktree+" was kept: "+kept+".")
		return kept, nil
	}
	markFired(h.ID, "")
	return "", nil
}

// fireLauncherTab is /act?action=fire for a launcher tab: handled is true when
// the tab is an engine hire's window, whatever the answer.
func (d *Deck) fireLauncherTab(tab string, force bool) (status int, message string, handled bool) {
	kind, id, _ := splitTab(tab)
	if kind != "launcher" {
		return 0, "", false
	}
	d.mu.Lock()
	launcher, ok := d.launchers[id]
	options := d.launcherEngine
	d.mu.Unlock()
	if !ok {
		return 0, "", false
	}
	// A launcher that connected again has no hire id (its token was used up). If it works
	// in an open engine hire's worktree it may be that hire's window: the fire rules can't
	// run, and a plain Stop would skip them, so nothing is changed.
	if launcher.hireID == "" {
		if launcher.office && engineHireWorksIn(launcher.folder) {
			return http.StatusConflict, "Couldn't tell which hire's window this is, so nothing was changed. Close it yourself, then hire again.", true
		}
		return 0, "", false
	}
	var h Hire
	found := false
	for _, row := range officeHires() {
		if row.ID == launcher.hireID {
			h, found = row, true
		}
	}
	if !found {
		return 0, "", false
	}
	if _, err := d.fireLauncherHire(h, options, force); err != nil {
		var refused *hireRefusal
		if errors.As(err, &refused) {
			return refused.Status, refused.Message, true
		}
		return http.StatusInternalServerError, err.Error(), true
	}
	// A fired GM's token stops working unless another GM is open by now
	// (clearFiredGMToken), and mail still waiting for the hire fails now rather than on the pump's next round.
	if h.Role == gmRole {
		d.clearFiredGMToken()
	}
	if d.mail != nil {
		d.mail.failTo(h.ID)
	}
	d.mu.Lock()
	for token, pending := range d.hires {
		if pending.hireID == h.ID {
			delete(d.hires, token)
		}
	}
	if d.sessions.Prune(func(t string) bool { return t != tab }, nowMillis()) {
		d.changed()
	}
	d.mu.Unlock()
	return http.StatusNoContent, "", true
}
