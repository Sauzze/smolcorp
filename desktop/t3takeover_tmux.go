//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

// Take over's stop and Hand back's resume, in a hire's own tmux session (t3takeover.go). Nothing here types into
// the window or answers anything: the session is only ended, once its window is seen idle with nothing typed,
// and a new one is started with the hire's own start script, as the engine starts a hire (hireengine.go).

// t3StopInTmux ends hire h's Claude Code in its own tmux session, as a fire without force would: only when its
// window is idle with nothing typed (looked at twice), then seen gone. A hire whose tmux session has ended
// already has nothing to stop. A refusal changes nothing.
func (d *Deck) t3StopInTmux(h Hire) error {
	d.mu.Lock()
	o := d.engine.withDefaults()
	d.mu.Unlock()
	name, testPort := calledHire(h), testCopyPort()
	refuse := func(message string) error { return t3Refused(http.StatusConflict, message) }
	unreached := "Couldn't reach " + name + "'s tmux session, so nothing was changed."
	own := hireOwnSocket(h.ID, testPort)
	sandboxed := deskSocketID(h.TmuxSocket, testPort) == h.ID
	if sandboxed {
		own = deskSocketPath(h.ID, testPort) // a desk in its sandbox (SA-M6)
	}
	if own == "" {
		return refuse(unreached)
	}
	// A sandboxed desk with nothing of it running: its socket folder goes, as at its stop, so Hand back's resume can
	// make it new. What it started in the background is ended first, and seen ended, as at a fire (FXFIREREAP): one
	// that can't be refuses the stop, and Take over or the restart goes no further.
	ended := func() error {
		if sandboxed {
			if why := deskStopped(h, o); why != "" {
				return refuse(stopRefusal(h, why))
			}
		}
		return nil
	}
	tmux := hireOwnTmux(h, testPort, o.Tmux)
	if tmux == nil {
		// Its socket is gone with its tmux server: nothing of it runs to be stopped.
		if _, err := os.Lstat(filepath.Join(physical(filepath.Dir(own)), filepath.Base(own))); errors.Is(err, os.ErrNotExist) && (o.Tmux != nil || sandboxed || tmuxPath() != "") {
			return ended()
		}
		return refuse(unreached)
	}
	up, known := tmux.alive()
	if !known {
		return refuse(unreached)
	}
	if !up {
		return ended()
	}
	for range 2 {
		// Looked at twice: a question that comes up meanwhile stops it.
		screen, readable := tmux.capture(200)
		if why := t3WindowRefusal(name, ReadStyledPaneState(screen), readable); why != "" {
			return refuse(why)
		}
	}
	tmux.end()
	if still, known := tmux.alive(); !known || still {
		return refuse("Couldn't end " + name + "'s tmux session, so nothing was changed.")
	}
	// Its windows went with its session.
	forgetHireWindows(h.ID, windowsOfHire(h.ID))
	if tmux.sandboxed {
		// Its server goes too, and its socket folder with it: Hand back's resume makes that folder new.
		tmux.endServer()
	}
	return ended()
}

// t3NotSignedIn is a resume refused because the desk's own Claude folder isn't signed in (SA-X9), as Hand back says it.
const t3NotSignedIn = "it isn't signed in: sign in your hires once in Setup (Claude sign-in)."

// t3ResumeInTmux starts hire h's Claude Code again on session, in a tmux session of its own made as the engine
// makes one: its own settings (its fence, made afresh, in its command line), its ports, a new send key, the default account
// (the one T3 Code's agent worked on), in the folder it started in. No window is opened: Setup's Open a window
// shows it. Its socket as tmux spells it, which Hand back's last save keeps, as the engine keeps it (checked there,
// not dropped here: FXM5c), with the tmux that started it and, on a Mac, the server's pid as its sandbox's start
// checked it (SA-M6): the engine's start, sandbox and all.
func (d *Deck) t3ResumeInTmux(h Hire, session string, profiles []claudeProfile) (t3Resumed, error) {
	d.mu.Lock()
	o := d.engine.withDefaults()
	o.SandboxOff = d.config.SandboxOff // resumed as a start now would be (AQ1)
	o.DatabaseOptIn = d.config.LocalDatabaseAsksPassword
	roles := slices.Clone(d.config.Roles)
	d.mu.Unlock()
	// A Claude Code that can't make a stopped guard refuse gets no desk (SANDBOX-PLAN §3.4).
	if refusal := claudeVersionRefusal(); refusal != nil {
		return t3Resumed{}, errors.New(refusal.Message)
	}
	testPort := testCopyPort()
	tmux, err := o.deskTmux(h.ID, testPort)
	if err != nil {
		return t3Resumed{}, err // as it is: a missing bundled tmux has §A5.2's own words (RT126SAM6 B4)
	}
	// A sandboxed desk's isn't asked: nothing runs the bundled tmux before its start has checked a server (the release
	// gate), and its start refuses a socket folder that's there already (makeDeskSocketFolder).
	if !tmux.sandboxed {
		if up, known := tmux.alive(); !known || up {
			return t3Resumed{}, errors.New("its tmux session is there already, or tmux didn't answer.")
		}
	}
	role := Role{ID: h.Role, Name: h.Name, Agent: string(Claude)}
	if i := slices.IndexFunc(roles, func(r Role) bool { return r.ID == h.Role }); i >= 0 {
		role = roles[i]
	}
	if agentOrClaude(role.Agent) != Claude {
		return t3Resumed{}, errors.New("its role isn't a Claude Code one any more.")
	}
	role.Model = cmpOr(h.Model, role.Model)
	// Take over takes only a session on the default account (t3OnDefaultAccount), so it goes on there. A desk with its
	// own Claude folder (SA-X8) goes on in it, where Hand back brought its session back (t3BringBack), with its plans
	// kept outside it, as the engine starts it (RT126SAX8R2); one without, in ~/.claude, with Claude Code's own plans.
	own, ownClaude := deskOwnClaude(h.ID)
	settings, err := writeHireSettings(hireSettingsOptions{HireID: h.ID, Auto: engineAutoMode, Profile: guardHire, CLI: o.CLI,
		Support: supportDir, ReportPort: testPort, Profiles: profiles, Run: o.Guard, IsExec: o.IsExec, OwnClaude: ownClaude,
		Kept: deskKeptFolder(runtime.GOOS, tmux.sandboxed, h.ID), ReadsOwnFolder: !engineAutoMode})
	if err != nil {
		return t3Resumed{}, errors.New("its settings, its fence, couldn't be made: " + err.Error())
	}
	sendKey, err := sendKeyFor(role, h.ID)
	if err != nil {
		return t3Resumed{}, errors.New("its send key couldn't be made.")
	}
	hookKey, err := newHookKey(registryPath(), h.ID)
	if err != nil {
		return t3Resumed{}, errors.New("its hook key couldn't be made.")
	}
	start := hireStart{Role: role, Resume: session, DefaultAccount: true, Settings: settings, CLIDir: filepath.Dir(o.CLI),
		Ports: h.Ports, SendKey: sendKey, HookKey: hookKey}
	if ownClaude {
		start.DeskClaude = own
		// Its sign-in, as its own folder sees it, as at every start (SA-X9).
		if deskSignInRefusal(own, "", false) != "" {
			return t3Resumed{}, errors.New(t3NotSignedIn)
		}
		if makeDeskPlans(startedIn(h), h.ID) != nil {
			return t3Resumed{}, errors.New("its plans folder couldn't be made.")
		}
	}
	if testPort != 0 {
		start.TestPort, start.TestSupportDir = testPort, supportDir
	}
	plan, err := start.plan()
	if err != nil {
		return t3Resumed{}, err
	}
	if refusal := ownSealRefusal(); refusal != nil {
		return t3Resumed{}, errors.New(refusal.Message)
	}
	// Its own folder, its owner's alone, as the engine makes it.
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		return t3Resumed{}, errors.New("its own folder couldn't be made.")
	}
	// Its sandbox, as the engine makes it, in the place it worked (desksandbox.go).
	if err := o.sandboxDesk(tmux, deskAtOf(h, tmux.socket, profiles, o)); err != nil {
		return t3Resumed{}, err
	}
	// Its start goes to its pane alone, by a ticket (deskstart.go): no script, no shell.
	command, err := deskStarts.issue(o.CLI, h.ID, tmux, plan, o.Key)
	if err != nil {
		// Stopped as the restart's is (sandboxrestart.go), with its socket, so its sandbox is reaped and its socket
		// folder taken away; a program that stop couldn't end is said too (FXFIREREAPR3).
		if why := deskStopped(Hire{ID: h.ID, TmuxSocket: tmux.socket}, o); why != "" {
			return t3Resumed{}, errors.New(err.Error() + " " + why + ".")
		}
		return t3Resumed{}, err
	}
	// In the folder the session started in (its worktree), never the project the registry names.
	_, socket, started, err := tmux.start(startedIn(h), command)
	if err != nil {
		deskStarts.forget(h.ID)
		// What the refused start left running in its sandbox is ended first (FXFIREREAPR1): one that can't be says so.
		stopped := h
		stopped.TmuxSocket = tmux.socket
		if why := deskStopped(stopped, o); why != "" {
			return t3Resumed{}, errors.New(reapWordsAfterColon(why) + ".")
		}
		if errors.Is(err, errTmuxDidntStart) {
			return t3Resumed{}, errors.New("tmux didn't start it.")
		}
		return t3Resumed{}, err
	}
	deskStarts.pin(h.ID, started)
	program := tmux.program
	if program == "" && !tmux.sandboxed {
		program = recordableTmux(tmuxPath()) // a stand-in runner's: the tmux this system picks now, as before
	}
	return t3Resumed{Socket: socket, Program: program, Server: tmux.server, Pane: started, PaneStarted: paneStartedAt(started), Sandboxed: startedInSandbox(tmux)}, nil
}
