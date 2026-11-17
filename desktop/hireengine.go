//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// The hire engine (Phase 6 T602, part C1): a managed Claude hire made in order,
// each step ready before the next, as the Mac's HireDesk: the guard for a test
// copy, a tmux server of its own, the worktree and ports (prepareHire), the
// hire's own settings, its start ticket, then the session. If a step refuses,
// what the steps before it made is taken away. Then the hire is watched until
// it reports (waitForStart); one that never does is cleaned up (cleanUpHire).

// engineOptions are what the engine needs from outside, each with a test stand-in.
type engineOptions struct {
	Git       gitRunner
	PortFree  func(int) bool
	Key       func() string
	Guard     guardRunner
	IsExec    func(string) bool
	CLI       string // the app's own smolcorp command
	Tmux      tmuxRunner
	Worktrees string
	// Opens a window attached to the hire's session (its row as started), when there's a terminal.
	Attach    func(hire Hire) error
	HeardFrom func(Hire) bool
	Within    time.Duration
	Every     time.Duration
	Now       func() time.Time
	Pause     func(time.Duration)
	// The GM's fire, unless removeWorktree is asked: leave the worktree. Zero removes it when clean.
	KeepWorktree bool
	// Sandbox is a sandboxed desk's start-only runner, for the tests: nil is the system's own (SA-M6), with the real
	// tmux (Tmux nil). A stand-in Tmux with no Sandbox starts a desk as before, through the stand-in alone.
	Sandbox func(s *deskSandbox) tmuxStarter
	// SandboxOff: the owner turned the sandbox off in Settings (AQ1, sandboxswitch.go), as the deck's config held it
	// when this start was asked for. A desk then starts as before, and its row says it isn't in a sandbox.
	SandboxOff bool
	// DatabaseOptIn: AQ21's opt-in, as the deck's config held it when this start was asked for (sandboxchecks.go).
	DatabaseOptIn bool
}

// sandboxing is whether the engine starts this desk in its sandbox: on a Mac with the real tmux (deskSandboxed), or
// with a stand-in runner for it; never while the owner has the sandbox off.
func (o engineOptions) sandboxing() bool {
	return !o.SandboxOff && (o.Sandbox != nil || deskSandboxed && o.Tmux == nil)
}

// deskTmux is a new desk's tmux server: in its sandbox when the engine starts it there (newDeskTmux), else as before.
func (o engineOptions) deskTmux(id string, testPort int) (*hireTmux, error) {
	if o.sandboxing() {
		return newDeskTmux(id, testPort, o.Tmux)
	}
	if tmux := newHireTmux(id, testPort, o.Tmux); tmux != nil {
		return tmux, nil
	}
	return nil, errors.New("Smolcorp couldn't set up a tmux server of its own for this hire: tmux is missing, or its socket's path is too long.")
}

// sandboxDesk makes desk d's sandbox and gives tmux its start-only runner, for a sandboxed desk; nothing for another.
// The error is the owner's words.
func (o engineOptions) sandboxDesk(tmux *hireTmux, d deskAt) error {
	if !tmux.sandboxed {
		return nil
	}
	desk, err := makeDeskSandbox(d)
	if err != nil {
		return err
	}
	if o.Sandbox != nil {
		tmux.startOnly = onlyStarts(tmux.socket, o.Sandbox(desk))
	} else {
		tmux.startOnly = onlyStarts(tmux.socket, desk.starter(tmux.program, o.CLI))
	}
	return nil
}

// tmuxStartRefusal is a start tmux.start refused, as the engine answers it: the sandbox's words, or tmux's.
func tmuxStartRefusal(err error) (status int, message string) {
	if errors.Is(err, errTmuxDidntStart) {
		return http.StatusInternalServerError, "tmux didn't start the hire."
	}
	return http.StatusConflict, err.Error()
}

// worktrees is the folder hires' worktrees are made in, "" for beside the repo (hiresFolder).
func (o engineOptions) worktrees() string { return o.Worktrees }

func (o engineOptions) withDefaults() engineOptions {
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

// runManagedHire makes a managed Claude hire and starts it, or refuses with
// nothing left behind. The hire is in the registry as "starting".
func runManagedHire(r engineRequest, o engineOptions) (Hire, error) {
	o = o.withDefaults()
	// Before anything is made.
	if err := testCopyRefusal(r.TestPort, string(Claude), r.ConfigDir); err != nil {
		return Hire{}, err
	}
	// A Claude Code that can't make a stopped guard refuse gets no desk (SANDBOX-PLAN §3.4), nor does a Smolcorp
	// whose own files changed (ownseal.go).
	if refusal := claudeVersionRefusal(); refusal != nil {
		return Hire{}, refusal
	}
	if refusal := ownSealRefusal(); refusal != nil {
		return Hire{}, refusal
	}
	id := strings.ToLower(newID())[:12]
	tmux, err := o.deskTmux(id, r.TestPort)
	if err != nil {
		return Hire{}, &hireRefusal{Status: http.StatusConflict, Message: err.Error()}
	}
	name := r.Role.Name
	if r.Nickname != "" {
		name = r.Nickname
	}
	key := o.Key
	if r.APIKeyText != "" {
		key = func() string { return r.APIKeyText } // the key the Mac app gave for this hire
	}
	place, err := prepareHire(hireRequest{HireID: id, Name: name, Folder: r.Folder, Claude: true, ConfigDir: r.ConfigDir, Held: r.Held,
		APIKey: r.APIKey, Worktree: !r.NoWorktree},
		hirePlaceOptions{Worktrees: o.Worktrees, Taken: takenPorts(), Git: o.Git, PortFree: o.PortFree, Key: key})
	if err != nil {
		return Hire{}, err
	}
	undo := func() {
		if place.Worktree != "" {
			removeWorkspace(place.Repo, place.Worktree, place.Branch, o.Git)
		}
		_ = os.RemoveAll(hireFolder(supportDir, id))
	}
	fail := func(status int, message string) (Hire, error) {
		undo()
		return Hire{}, &hireRefusal{Status: status, Message: message}
	}
	profile := guardHire
	if r.Role.ID == "gm" {
		profile = guardGM
	}
	// A Claude Code desk gets its own Claude folder (SA-X8), made below, and its plans kept outside it.
	claude := agentOrClaude(r.Role.Agent) == Claude
	settings, err := writeHireSettings(hireSettingsOptions{HireID: id, Auto: engineAutoMode, Profile: profile, CLI: o.CLI, Support: supportDir,
		ReportPort: r.TestPort, Profiles: r.Profiles, Run: o.Guard, IsExec: o.IsExec, OwnClaude: claude, Kept: deskKeptFolder(runtime.GOOS, tmux.sandboxed, id)})
	if err != nil {
		undo()
		return Hire{}, err
	}
	// Its brief, as the Mac writes it: the first message then only says to read it.
	message, brief, err := engineBrief(r, place, id, o.Now())
	if err != nil {
		return fail(http.StatusInternalServerError, err.Error())
	}
	if brief != "" && place.Worktree == "" {
		undoSettings := undo
		undo = func() {
			_ = os.Remove(brief) // written for this hire a moment ago, and read by no one
			undoSettings()
		}
	}
	// The hire is written down before its start script, so it can be given its
	// own send key (T726: newSendKey keeps only the key's hash, against its row).
	// The socket is the one its tmux server will have; the one tmux reports is
	// kept below. Any refusal from here on takes the row and its key away again.
	// Its folder is the project it was hired into, its worktree where it works (the contract's registry.json).
	hire := Hire{ID: id, Name: r.Role.Name, Nickname: r.Nickname, Account: r.AccountName, Role: r.Role.ID, Folder: r.Folder, Branch: place.Branch, Worktree: place.Worktree, Repo: place.Repo,
		TmuxSocket: tmux.socket, TmuxProgram: tmux.program, Model: r.Role.Model, Ports: place.Ports, Brief: brief, Started: float64(o.Now().UnixMilli()), State: "starting", Notes: place.Note,
		Scope: r.Scope, ScopeRoles: r.ScopeRoles}
	// Its start is under way from its row until its server is up and its row read again: the edit never marks it
	// gone meanwhile (gonedesk.go).
	defer deskStarting(id)()
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
	// Every desk, the GM's too, reports with a hook key of its own (hookkeys.go).
	hookKey, err := newHookKey(registryPath(), id)
	if err != nil {
		return fail(http.StatusInternalServerError, "Couldn't make the hire's hook key.")
	}
	gmToken := ""
	if r.Role.ID == gmRole {
		if r.GMToken != nil {
			gmToken = r.GMToken()
		}
		if gmToken == "" && recordsHeldOnly() {
			// Its token can't be kept while the records' key can't be read (gmTokenKeeper.writeNow).
			return fail(http.StatusConflict, recordsHeldAnswer)
		}
		if gmToken == "" {
			return fail(http.StatusInternalServerError, "Couldn't make the General Manager's token.")
		}
	}
	start := hireStart{Role: r.Role, Message: message, ConfigDir: r.ConfigDir, DefaultAccount: r.DefaultAccount,
		Settings: settings, CLIDir: filepath.Dir(o.CLI), Ports: place.Ports, SendKey: sendKey, HookKey: hookKey, GMToken: gmToken, APIKey: r.APIKey, KeyText: r.APIKeyText}
	// The GM is told who it is, and starts with its rules, skills and tools where the app bundles them (GO-PARITY's
	// GM start, as the Mac's Swift engine started it: Deck.gmIntroduction, Role.startScript's plugin).
	if r.Role.ID == gmRole && agentOrClaude(r.Role.Agent) == Claude {
		isExec := o.IsExec
		if isExec == nil {
			isExec = isExecutableFile
		}
		if start.GMPlugin = gmPluginFolder(o.CLI); start.GMPlugin != "" && isExec(o.CLI) {
			start.GMCLI = o.CLI
		}
		start.Message += "\n\n" + gmIntroduction(place.Folder, start.GMPlugin != "" && start.GMCLI == "")
		if deskSandboxed && o.SandboxOff {
			start.Message += " " + gmSandboxOffLine // told it starts outside its sandbox (§A5.2, AQ1)
		}
	}
	if r.StartTyped != nil {
		r.StartTyped(start.Message)
	}
	if r.TestPort != 0 {
		start.TestPort, start.TestSupportDir = r.TestPort, supportDir
	}
	// Claude Code's own folder for this desk (SA-X8), made below; the undo above takes it away with the hire's folder.
	if claude {
		start.DeskClaude = deskClaudeFolder(supportDir, id)
	}
	plan, err := start.plan()
	if err != nil {
		return fail(http.StatusInternalServerError, err.Error())
	}
	// Its own folder, its owner's alone, where its brief and status go (the start script made it before SG2).
	if err := os.MkdirAll(hireFolder(supportDir, id), 0o700); err != nil {
		return fail(http.StatusInternalServerError, "Couldn't make the hire's own folder.")
	}
	if claude {
		// Its folder trusted as prepareHire trusted it in the account's file: the worktree, links resolved.
		desk := deskClaude{Account: r.ConfigDir, Trusted: physical(place.Folder), MCP: r.MCPServers}
		if place.Worktree != "" {
			desk.Trusted = physical(place.Worktree)
		}
		if r.APIKey {
			desk.KeyTail = keyApproval(key())
		}
		if err := makeDeskClaude(start.DeskClaude, desk); err != nil {
			if refused, ok := err.(*hireRefusal); ok {
				return fail(refused.Status, refused.Message)
			}
			return fail(http.StatusInternalServerError, err.Error()+".")
		}
		// Its sign-in, as its own folder sees it (SA-X9): one Claude Code says is missing refuses the start.
		if why := deskSignInRefusal(start.DeskClaude, r.ConfigDir, r.APIKey); why != "" {
			return fail(http.StatusConflict, why)
		}
		// Its plans, in its place (deskPlansDirectory): the fence denies its tools its Claude folder.
		if makeDeskPlans(place.Folder, id) != nil {
			return fail(http.StatusInternalServerError, "Smolcorp couldn't make the hire's plans folder.")
		}
		if place.Worktree == "" { // in a folder others share: a start refused from here takes its plans folder too
			undoMade := undo
			undo = func() {
				_ = removeDeskPlans(place.Folder, id)
				undoMade()
			}
		}
	}
	// Its sandbox, made from where it works, checked before anything of it runs (desksandbox.go): a Mac's every desk.
	if tmux.sandboxed {
		undoPlaced := undo
		undo = func() {
			deskStopped(Hire{ID: id, TmuxSocket: tmux.socket}, o) // its sandbox let go; its socket folder, when nothing answers
			undoPlaced()
		}
	}
	desk := deskAt{ID: id, GM: r.Role.ID == gmRole, Root: place.Folder, Socket: tmux.socket, Accounts: r.Profiles, Brief: brief, Git: o.Git,
		DatabaseOptIn: o.DatabaseOptIn}
	if place.Worktree != "" {
		desk.Root, desk.Checkout = place.Worktree, place.Repo
	}
	if place.Repo != "" {
		desk.Hires = hiresFolder(place.Repo, o.Worktrees)
	}
	if err := o.sandboxDesk(tmux, desk); err != nil {
		return fail(http.StatusConflict, err.Error())
	}
	// Its start goes to its pane alone, by a ticket (deskstart.go): no script, no shell.
	command, err := deskStarts.issue(o.CLI, id, tmux, plan, key)
	if err != nil {
		return fail(http.StatusInternalServerError, err.Error())
	}
	undoIssued := undo
	undo = func() {
		deskStarts.forget(id)
		undoIssued()
	}
	_, socket, started, err := tmux.start(place.Folder, command)
	if err != nil {
		return fail(tmuxStartRefusal(err))
	}
	deskStarts.pin(id, started)
	// Its row read again now its server is up: marked gone meanwhile (by another program, say), the session is ended
	// rather than left running behind it, and the row stays as it was (gonedesk.go, RT126GMEDIT1 B1).
	if !hireRowOpen(id) {
		deskStarts.forget(id) // its ticket first: an agent not yet started never is
		tmux.endServer()
		if up, known := tmux.alive(); !known || up {
			// The fire these words ask for must reach it: a sandboxed desk's tmux is run only told the server the core
			// checked (SA-M6), so its row records that one, as a started desk's does. Nothing else of the row changes.
			if tmux.server != 0 {
				_ = updateRegistry(func(f *registryFile) {
					if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == id }); i >= 0 {
						f.hires[i].TmuxSocket, f.hires[i].TmuxServer = socket, tmux.server
					}
				})
			}
			return Hire{}, goneWhileStarting(hire, "Smolcorp ended its tmux session but couldn't see it gone: fire "+calledHire(hire)+" to end it")
		}
		deskStopped(hire, o) // seen gone: its sandbox let go, and its socket folder (SA-M6)
		undoMade()
		return Hire{}, goneWhileStarting(hire, "its tmux session was ended")
	}
	// Its pane's process, as its start said it, and when that process started: what a respawned pane, or a pid used
	// again, is told from (SA-M7, RT126SG2R2 N1, N2).
	hire.TmuxPane, hire.TmuxPaneStarted = started, paneStartedAt(started)
	if socket != hire.TmuxSocket || tmux.server != 0 || hire.TmuxPane != 0 {
		// As tmux spells it (it may say /private/var for /var): the same file, which isHireSocket allows. A sandboxed
		// desk's server pid, as the core checked it (§A5.2 step 9), goes with it: its guarded tmux talks to that one.
		// Its tag goes with it too (SA-X13): in its sandbox only with that server checked.
		hire.TmuxSocket, hire.TmuxServer, hire.Sandboxed = socket, tmux.server, startedInSandbox(tmux)
		if err := updateRegistry(func(f *registryFile) {
			if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == id }); i >= 0 {
				f.hires[i].TmuxSocket, f.hires[i].TmuxServer, f.hires[i].Sandboxed = socket, tmux.server, hire.Sandboxed
				f.hires[i].TmuxPane, f.hires[i].TmuxPaneStarted = hire.TmuxPane, hire.TmuxPaneStarted
			}
		}); err != nil {
			tmux.end()
			return fail(http.StatusInternalServerError, "Couldn't save the hire in the registry.")
		}
	}
	if o.Attach != nil {
		if err := o.Attach(hire); err != nil {
			// It was started, so its row stays, marked failed; its key's hash stays but is inert
			// (hireForSendKey only finds open hires).
			ended := tmux.end()
			failHire(id, "Its window didn't open.")
			if ended { // its session is known to have ended (CLAUDEPLANT1)
				claudeHireEnded(id)
			}
			deskStopped(hire, o)
			undoMade()
			return Hire{}, &hireRefusal{Status: http.StatusInternalServerError, Message: "The terminal didn't open."}
		}
	}
	return hire, nil
}

// watchManagedHire waits for hire to report, marks it working, or cleans it up.
func watchManagedHire(hire Hire, testPort int, o engineOptions) {
	o = o.withDefaults()
	started, reason := waitForStart(
		func() (bool, bool) {
			if t := hireOwnTmux(hire, testPort, o.Tmux); t != nil {
				return t.alive()
			}
			return false, false
		},
		func() bool { return o.HeardFrom(hire) }, o.Within, o.Every, o.Now, o.Pause)
	if started {
		setHire(hire.ID, "working", hire.Notes)
		return
	}
	// Its ticket goes; a start the core refused (its ticket, its program, Smolcorp's own files) says why.
	if refused := deskStarts.forget(hire.ID); refused != "" {
		reason = strings.TrimSuffix(refused, ".")
	}
	cleanUpHire(hire, reason, hireOwnTmux(hire, testPort, o.Tmux), removeHireWorktree(o.Git, testPort, o.Worktrees),
		func(id, why string) { setHire(id, "failed", firstRunes(why, longestNotes)) })
	// Its windows' records go too, as a fired hire's do (hirewindows.go).
	forgetHireWindows(hire.ID, windowsOfHire(hire.ID))
	deskStopped(hire, o) // its sandbox, and a sandboxed desk's socket folder once nothing answers there
	// Its own Claude folder goes, as at a fire (SA-X8), and its plans from a folder others share.
	_ = removeDeskClaude(supportDir, hire.ID)
	if hire.Worktree == "" {
		_ = removeDeskPlans(startedIn(hire), hire.ID)
	}
	// A start script left from before 1.26, never run, still holds its send key or token: it goes too.
	scripts, _ := filepath.Glob(filepath.Join(hireFolder(supportDir, hire.ID), "hire-*.sh"))
	for _, script := range scripts {
		_ = os.Remove(script)
	}
}

// managedHire is hireAs's way in: with the lock held, it hires role roleID
// through the engine and says what to answer, or handled false for a hire the
// engine doesn't make (any other agent, a pinned desk, or no tmux: they open as
// they did). held is the desk chosen (-1 for none) on floor, where the hire's session
// is seated when it appears. The lock is let go while the engine works.
func (d *Deck) managedHire(roleID, folder string, terminal hireOpener, hasTerminal bool, held, floor int) (status int, message string, handled bool) {
	return d.managedHireWith(roleID, folder, terminal, hasTerminal, held, floor, nil)
}

// managedHireWith is managedHire with what setup's own hire says besides, or nil.
func (d *Deck) managedHireWith(roleID, folder string, terminal hireOpener, hasTerminal bool, held, floor int, extra *setupHire) (status int, message string, handled bool) {
	i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == roleID })
	if i < 0 {
		return 0, "", false
	}
	role, how := d.config.Roles[i], hiredIntoGeneralFolder
	options, testPort := d.engine, testCopyPort()
	options.SandboxOff = d.config.SandboxOff // the owner's switch, as held now (AQ1)
	options.DatabaseOptIn = d.config.LocalDatabaseAsksPassword
	options.Worktrees = d.worktreesFolder(options.Worktrees)
	if options.HeardFrom == nil {
		options.HeardFrom = d.hireHeardFrom
	}
	// The window shown, by its terminal's record (a Mac's): kept for the hire once it's made.
	window := ""
	if hasTerminal {
		// The window runs `smolcorp attach` with exec: the desk's guarded tmux, in its sandbox where it has one (SA-M8).
		options.Attach = func(hire Hire) (err error) {
			window, err = showWindow(terminal, folder, attachLine(commandLineProgram(), hire.ID))
			return err
		}
	}
	request := engineRequest{Role: role, How: how, Folder: folder, DefaultAccount: true, TestPort: testPort, Desk: briefDesk(held, floor)}
	// The accounts' settings and transcripts are denied to the hire, every saved folder's, even one no hire may
	// work on: the saved list is copied here, and its folders are looked in once the lock is let go (claudeaccounts.go).
	savedAccounts := slices.Clone(d.config.ClaudeProfiles)
	if extra != nil {
		extra.apply(&request, d.config.ManagerEnabled)
	}
	// The GM gets a new token at its hire, in its start script alone; the one it
	// was given is taken away again if the hire doesn't go ahead or never starts.
	token := ""
	if role.ID == gmRole {
		request.GMToken = func() string { token = d.newGMToken(); return token }
		request.StartTyped = d.owner.notOwners
	}
	d.mu.Unlock()
	request.Profiles = claudeDenyProfiles(savedAccounts)
	var hire Hire
	var err error
	// On the API-key account, under the Mac app: the key, from its Keychain through the bridge, for this hire alone.
	if request.APIKey {
		request.APIKeyText, err = d.hireAPIKeyText()
	}
	if err == nil {
		hire, err = runManagedHire(request, options)
	}
	request.APIKeyText = "" // not kept after the hire; copies stay until the garbage collector (hireapikey.go)
	d.mu.Lock()
	if err != nil {
		if token != "" && constantTimeEqual(d.gmToken, token) {
			d.gmToken = "" // d.mu is held: clearGMToken would take it again
			d.gmTokenChanged()
		}
		d.lastHire = time.Time{} // nobody was hired, so the next try needn't wait
		if refused, ok := err.(*hireRefusal); ok {
			return refused.Status, refused.Message, true
		}
		return http.StatusInternalServerError, err.Error(), true
	}
	recordHireWindow(hire.ID, window)
	d.seatEngineHire(hire.TmuxSocket, held, floor)
	d.hiredInto(hire)
	d.watching.Add(1)
	go func() {
		defer d.watching.Done()
		watchManagedHire(hire, testPort, options)
		if token != "" && !hireIsOpen(hire.ID) {
			d.clearGMTokenIf(token)
		}
	}()
	return http.StatusNoContent, "", true
}

// hireSessionUp is whether the hire's own tmux session is there. When tmux
// can't be asked it counts as there: when in doubt, no second GM.
func hireSessionUp(h Hire, testPort int, o engineOptions) bool {
	tmux := hireOwnTmux(h, testPort, o.withDefaults().Tmux)
	if tmux == nil {
		return false
	}
	up, known := tmux.alive()
	return up || !known
}
