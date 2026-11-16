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
)

// "Restart in the sandbox" (SA-X14, SANDBOX-PLAN §A5.3, AQ17): the owner's button, on Setup's Sandbox row, for a
// desk that runs without its sandbox (started before this version, or while the sandbox was off). Its Claude Code
// is ended as Take over ends it, only while its window is idle with nothing typed, and started again, fresh, in the
// same place (its worktree and branch), as the engine starts a desk now: its own Claude folder, new keys, and its
// sandbox. Its conversation isn't carried over: a sandboxed desk can't read the transcripts kept outside its own
// folder. Smolcorp never restarts a desk by itself; only this button does.

// restartedMessage is a restarted desk's first message: where it is, and where to pick up from.
func restartedMessage(h Hire) string {
	text := "Smolcorp started you again, this time in its sandbox, in the same folder and on the same branch as before. " +
		"Your earlier conversation isn't here, so look at the work so far (git status, git log) before going on."
	if h.Brief != "" {
		text += " Your brief is " + h.Brief + "."
	}
	return text
}

// restartDesk ends desk h (stop: Take over's, which refuses a busy window and changes nothing then) and starts it
// again in its sandbox, in the place it worked, on account (its config folder, "" for ~/.claude's), with profiles
// fenced off. What its start found is what its row is saved with. An error after the stop says the desk was stopped:
// its place is as it left it, and its row stays, for Restart again or Fire.
func restartDesk(h Hire, role Role, account string, profiles []claudeProfile, o engineOptions, stop func(Hire) error) (t3Resumed, error) {
	o = o.withDefaults()
	if !o.sandboxing() {
		return t3Resumed{}, t3Refused(http.StatusConflict, sandboxOffToRestart)
	}
	if refusal := claudeVersionRefusal(); refusal != nil {
		return t3Resumed{}, refusal
	}
	if refusal := ownSealRefusal(); refusal != nil {
		return t3Resumed{}, refusal
	}
	// While the records' key can't be read, its new keys couldn't be saved: refused before the stop, so the desk
	// isn't left stopped and not started again (RT126SAX14 nit 2).
	if recordsHeldOnly() {
		return t3Resumed{}, t3Refused(http.StatusConflict, recordsHeldAnswer)
	}
	if err := stop(h); err != nil {
		return t3Resumed{}, restartStopWords(err)
	}
	name := calledHire(h)
	stopped := func(why string) (t3Resumed, error) {
		return t3Resumed{}, t3Refused(http.StatusInternalServerError, name+" was stopped, and couldn't start again: "+why+
			" Its worktree and branch are as it left them: press Restart in the sandbox again, or fire it.")
	}
	testPort := testCopyPort()
	tmux, err := o.deskTmux(h.ID, testPort)
	if err != nil {
		return stopped(err.Error())
	}
	// Its guard told its kept copy as the engine's start tells it (--held, L1GUARD1, the release gate's item 4b): on
	// Linux, once desks start in their sandbox there (RT126SAL2 S1).
	settings, err := writeHireSettings(hireSettingsOptions{HireID: h.ID, Auto: engineAutoMode, Profile: guardHire, CLI: o.CLI,
		Support: supportDir, ReportPort: testPort, Profiles: profiles, Run: o.Guard, IsExec: o.IsExec, OwnClaude: true,
		Kept: deskKeptFolder(runtime.GOOS, tmux.sandboxed, h.ID)})
	if err != nil {
		return stopped("its settings, its fence, couldn't be made.")
	}
	sendKey, err := sendKeyFor(role, h.ID)
	if err != nil {
		return stopped("its send key couldn't be made.")
	}
	hookKey, err := newHookKey(registryPath(), h.ID)
	if err != nil {
		return stopped("its hook key couldn't be made.")
	}
	start := hireStart{Role: role, Message: restartedMessage(h), ConfigDir: account, DefaultAccount: account == "", Settings: settings,
		CLIDir: filepath.Dir(o.CLI), Ports: h.Ports, SendKey: sendKey, HookKey: hookKey, DeskClaude: deskClaudeFolder(supportDir, h.ID)}
	if testPort != 0 {
		start.TestPort, start.TestSupportDir = testPort, supportDir
	}
	plan, err := start.plan()
	if err != nil {
		return stopped(err.Error())
	}
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		return stopped("its own folder couldn't be made.")
	}
	// Its own Claude folder, made new (SA-X8): one from a start without the sandbox goes, transcripts and all, as at
	// a fire. Trusted where it works, as its hire trusted it.
	trusted := h.Folder
	if h.Worktree != "" {
		trusted = h.Worktree
	}
	if removeDeskClaude(supportDir, h.ID) != nil {
		return stopped("its Claude folder couldn't be made new.")
	}
	// No MCP servers: nothing records which ones were ticked at its hire (no caller sets engineRequest.MCPServers yet,
	// and a row doesn't keep them). Once a row does, they go here as the engine's start passes them (RT126SAX14 nit 1).
	if err := makeDeskClaude(start.DeskClaude, deskClaude{Account: account, Trusted: physical(trusted)}); err != nil {
		return stopped(strings.TrimSuffix(err.Error(), ".") + ".")
	}
	if makeDeskPlans(startedIn(h), h.ID) != nil {
		return stopped("Smolcorp couldn't make its plans folder.")
	}
	// Its sandbox, made from where it works and checked before anything of it runs (X12's checks among them).
	if err := o.sandboxDesk(tmux, deskAtOf(h, tmux.socket, profiles, o)); err != nil {
		return stopped(err.Error())
	}
	command, err := deskStarts.issue(o.CLI, h.ID, tmux, plan, o.Key)
	if err != nil {
		deskStopped(Hire{ID: h.ID, TmuxSocket: tmux.socket}, o)
		return stopped(err.Error())
	}
	_, socket, started, err := tmux.start(startedIn(h), command)
	if err != nil {
		deskStarts.forget(h.ID)
		deskStopped(Hire{ID: h.ID, TmuxSocket: tmux.socket}, o)
		_, words := tmuxStartRefusal(err)
		return stopped(words)
	}
	deskStarts.pin(h.ID, started)
	return t3Resumed{Socket: socket, Program: tmux.program, Server: tmux.server, Sandboxed: startedInSandbox(tmux)}, nil
}

// onOwnTmux is whether desk h was started on a tmux server of its own by the engine: its recorded socket is its own
// by name and folder, in its sandbox or not, whether or not its server still runs (its stop finds that out).
func onOwnTmux(h Hire) bool {
	testPort := testCopyPort()
	if deskSocketID(h.TmuxSocket, testPort) == h.ID {
		return true
	}
	name, socket := hireSocketName(h.ID, testPort), filepath.Clean(h.TmuxSocket)
	if name == "" || !filepath.IsAbs(h.TmuxSocket) || filepath.Base(socket) != name {
		return false
	}
	return slices.ContainsFunc(hireSocketFolders(), func(folder string) bool { return physical(filepath.Dir(socket)) == physical(folder) })
}

// restartStopWords is Take over's refusal to end a desk, said for a restart: nothing was changed.
func restartStopWords(err error) error {
	var refused *hireRefusal
	if !errors.As(err, &refused) {
		return err
	}
	words := strings.Replace(refused.Message, "take it over once it's done", "restart it once it's done", 1)
	return t3Refused(refused.Status, words)
}

// restartInSandbox is "Restart in the sandbox" for desk id, from Setup's Sandbox row: the refusals (restartRefusal),
// then restartDesk, with the desk held from fires and Take over meanwhile. 200 once it runs in its sandbox.
func (d *Deck) restartInSandbox(id string) (int, any) {
	h, found := registryHire(id)
	if id == "" || !found || !isOpen(h) {
		return http.StatusNotFound, t3NotOnTeam
	}
	d.mu.Lock()
	off := d.config.SandboxOff
	roles := slices.Clone(d.config.Roles)
	saved := slices.Clone(d.config.ClaudeProfiles)
	o := d.engine
	o.SandboxOff, o.DatabaseOptIn = off, d.config.LocalDatabaseAsksPassword
	o.Worktrees = d.worktreesFolder(o.Worktrees)
	d.mu.Unlock()
	apply := toApply(o.DatabaseOptIn, true)[id] != ""
	role, account, why := restartRefusal(h, roles, saved, off, apply)
	if why != "" {
		return http.StatusConflict, why
	}
	// Held from fires, Take over and a second restart until it's done: each is refused, and told it's a restart.
	firing.Lock()
	restarting, busy := firing.restarting[id], firing.ids[id] || firing.takingOver[id] || t3IsHiring(id)
	if !busy {
		firing.ids[id], firing.restarting[id] = true, true
	}
	firing.Unlock()
	if restarting {
		return http.StatusConflict, calledHire(h) + " is being restarted in its sandbox already."
	}
	if busy {
		return http.StatusConflict, calledHire(h) + " is being fired or taken over right now, so nothing was changed."
	}
	defer func() { firing.Lock(); delete(firing.ids, id); delete(firing.restarting, id); firing.Unlock() }()
	// Read again once it's held: a Take over or a fire that finished meanwhile is seen.
	if h, found = registryHire(id); !found || !isOpen(h) {
		return http.StatusNotFound, t3NotOnTeam
	}
	if _, _, why = restartRefusal(h, roles, saved, off, apply); why != "" {
		return http.StatusConflict, why
	}
	session, stopped := d.t3SessionOf(h), false
	resumed, err := restartDesk(h, role, account, claudeDenyProfiles(saved), o, func(h Hire) error {
		err := d.t3StopInTmux(h)
		stopped = err == nil
		return err
	})
	if stopped && session.ID != "" {
		// Its old session is over once it's stopped: the office doesn't keep it at its desk.
		d.mu.Lock()
		d.sessions.end(session.ID, nowMillis())
		d.mu.Unlock()
	}
	defer func() { d.mu.Lock(); d.changed(); d.mu.Unlock() }()
	if err != nil {
		return t3Answer(err)
	}
	if t3SaveOpen(id, func(r *Hire) {
		r.TmuxSocket, r.TmuxProgram, r.TmuxServer, r.Sandboxed = resumed.Socket, resumed.Program, resumed.Server, resumed.Sandboxed
	}) != nil {
		return http.StatusInternalServerError, calledHire(h) + " runs in its sandbox again, but the registry couldn't be written, so it may still show without one."
	}
	return http.StatusOK, map[string]any{"ok": true, "name": calledHire(h)}
}
