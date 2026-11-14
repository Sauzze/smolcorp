//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
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
	if own == "" {
		return refuse(unreached)
	}
	tmux := hireOwnTmux(h, testPort, o.Tmux)
	if tmux == nil {
		// Its socket is gone with its tmux server: nothing of it runs to be stopped.
		if _, err := os.Lstat(filepath.Join(physical(filepath.Dir(own)), filepath.Base(own))); errors.Is(err, os.ErrNotExist) && (o.Tmux != nil || tmuxPath() != "") {
			return nil
		}
		return refuse(unreached)
	}
	up, known := tmux.alive()
	if !known {
		return refuse(unreached)
	}
	if !up {
		return nil
	}
	for range 2 {
		// Looked at twice: a question that comes up meanwhile stops it.
		screen, readable := tmux.capture(200)
		if why := t3WindowRefusal(name, ReadPaneState(screen), readable); why != "" {
			return refuse(why)
		}
	}
	tmux.end()
	if still, known := tmux.alive(); !known || still {
		return refuse("Couldn't end " + name + "'s tmux session, so nothing was changed.")
	}
	// Its windows went with its session.
	forgetHireWindows(h.ID, windowsOfHire(h.ID))
	return nil
}

// t3ResumeInTmux starts hire h's Claude Code again on session, in a tmux session of its own made as the engine
// makes one: its own settings (its fence, made afresh, in its command line), its ports, a new send key, the default account
// (the one T3 Code's agent worked on), in the folder it started in. No window is opened: Setup's Open a window
// shows it. Its socket as tmux spells it, which Hand back's last save keeps, as the engine keeps it (checked there,
// not dropped here: FXM5c).
func (d *Deck) t3ResumeInTmux(h Hire, session string, profiles []claudeProfile) (string, error) {
	d.mu.Lock()
	o := d.engine.withDefaults()
	roles := slices.Clone(d.config.Roles)
	d.mu.Unlock()
	// A Claude Code that can't make a stopped guard refuse gets no desk (SANDBOX-PLAN §3.4).
	if refusal := claudeVersionRefusal(); refusal != nil {
		return "", errors.New(refusal.Message)
	}
	testPort := testCopyPort()
	tmux := newHireTmux(h.ID, testPort, o.Tmux)
	if tmux == nil {
		return "", errors.New("tmux is missing, or its socket's path is too long.")
	}
	if up, known := tmux.alive(); !known || up {
		return "", errors.New("its tmux session is there already, or tmux didn't answer.")
	}
	role := Role{ID: h.Role, Name: h.Name, Agent: string(Claude)}
	if i := slices.IndexFunc(roles, func(r Role) bool { return r.ID == h.Role }); i >= 0 {
		role = roles[i]
	}
	if agentOrClaude(role.Agent) != Claude {
		return "", errors.New("its role isn't a Claude Code one any more.")
	}
	role.Model = cmpOr(h.Model, role.Model)
	settings, err := hireSettingsLine(hireSettingsOptions{HireID: h.ID, Auto: engineAutoMode, Profile: guardHire, CLI: o.CLI,
		Support: supportDir, ReportPort: testPort, Profiles: profiles, Run: o.Guard, IsExec: o.IsExec})
	if err != nil {
		return "", errors.New("its settings, its fence, couldn't be made.")
	}
	sendKey, err := sendKeyFor(role, h.ID)
	if err != nil {
		return "", errors.New("its send key couldn't be made.")
	}
	start := hireStart{Role: role, Resume: session, DefaultAccount: true, Settings: settings, CLIDir: filepath.Dir(o.CLI),
		Ports: h.Ports, SendKey: sendKey}
	if testPort != 0 {
		start.TestPort, start.TestSupportDir = testPort, supportDir
	}
	plan, err := start.plan()
	if err != nil {
		return "", err
	}
	if refusal := ownSealRefusal(); refusal != nil {
		return "", errors.New(refusal.Message)
	}
	// Its own folder, its owner's alone, as the engine makes it.
	if err := os.MkdirAll(hireFolder(supportDir, h.ID), 0o700); err != nil {
		return "", errors.New("its own folder couldn't be made.")
	}
	// Its start goes to its pane alone, by a ticket (deskstart.go): no script, no shell.
	command, err := deskStarts.issue(o.CLI, h.ID, tmux, plan, o.Key)
	if err != nil {
		return "", err
	}
	// In the folder the session started in (its worktree), never the project the registry names.
	_, socket, started, ok := tmux.start(startedIn(h), command)
	if !ok {
		deskStarts.forget(h.ID)
		return "", errors.New("tmux didn't start it.")
	}
	deskStarts.pin(h.ID, started)
	return socket, nil
}
