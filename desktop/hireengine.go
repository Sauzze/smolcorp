//go:build !windows

package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The hire engine (Phase 6 T602, part C1): a managed Claude hire made in order,
// each step ready before the next, as the Mac's HireDesk: the guard for a test
// copy, a tmux server of its own, the worktree and ports (prepareHire), the
// hire's own settings, its start script, then the session. If a step refuses,
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
	// Opens a window attached to the hire's session, when there's a terminal.
	Attach    func(socket string) error
	HeardFrom func(Hire) bool
	Within    time.Duration
	Every     time.Duration
	Now       func() time.Time
	Pause     func(time.Duration)
}

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
	id := strings.ToLower(newID())[:12]
	tmux := newHireTmux(id, r.TestPort, o.Tmux)
	if tmux == nil {
		return Hire{}, &hireRefusal{Status: http.StatusConflict, Message: "Smolcorp couldn't set up a tmux server of its own for this hire: tmux is missing, or its socket's path is too long."}
	}
	place, err := prepareHire(hireRequest{HireID: id, Name: r.Role.Name, Folder: r.Folder, Claude: true, ConfigDir: r.ConfigDir,
		APIKey: r.APIKey, Worktree: true},
		hirePlaceOptions{Worktrees: o.Worktrees, Taken: takenPorts(), Git: o.Git, PortFree: o.PortFree, Key: o.Key})
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
	settings, err := writeHireSettings(hireSettingsOptions{HireID: id, Profile: profile, CLI: o.CLI, Support: supportDir, ReportPort: r.TestPort, Run: o.Guard, IsExec: o.IsExec})
	if err != nil {
		undo()
		return Hire{}, err
	}
	// The hire is written down before its start script, so it can be given its
	// own send key (T726: newSendKey keeps only the key's hash, against its row).
	// The socket is the one its tmux server will have; the one tmux reports is
	// kept below. Any refusal from here on takes the row and its key away again.
	hire := Hire{ID: id, Name: r.Role.Name, Role: r.Role.ID, Folder: place.Folder, Branch: place.Branch, Worktree: place.Worktree, Repo: place.Repo,
		TmuxSocket: tmux.socket, Model: r.Role.Model, Ports: place.Ports, Started: float64(o.Now().UnixMilli()), State: "starting", Notes: place.Note}
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
	start := hireStart{Role: r.Role, Message: r.Role.firstMessage(place.Folder, r.How), ConfigDir: r.ConfigDir, DefaultAccount: r.DefaultAccount,
		Settings: settings, CLIDir: filepath.Dir(o.CLI), Ports: place.Ports, SendKey: sendKey, GMToken: gmToken, APIKey: r.APIKey}
	if r.TestPort != 0 {
		start.TestPort, start.TestSupportDir = r.TestPort, supportDir
	}
	script, err := start.script()
	if err != nil {
		return fail(http.StatusInternalServerError, err.Error())
	}
	file, err := writeHireStartScript(hireFolder(supportDir, id), script)
	if err != nil {
		return fail(http.StatusInternalServerError, "Couldn't write the hire's start script.")
	}
	_, socket, ok := tmux.start(place.Folder, loginShell(), file)
	if !ok {
		return fail(http.StatusInternalServerError, "tmux didn't start the hire.")
	}
	if socket != hire.TmuxSocket {
		// As tmux spells it (it may say /private/var for /var): the same file, which isHireSocket allows.
		hire.TmuxSocket = socket
		if err := updateRegistry(func(f *registryFile) {
			if i := slices.IndexFunc(f.hires, func(h Hire) bool { return h.ID == id }); i >= 0 {
				f.hires[i].TmuxSocket = socket
			}
		}); err != nil {
			tmux.end()
			return fail(http.StatusInternalServerError, "Couldn't save the hire in the registry.")
		}
	}
	if o.Attach != nil {
		if err := o.Attach(socket); err != nil {
			// It was started, so its row stays, marked failed; its key's hash stays but is inert
			// (hireForSendKey only finds open hires).
			tmux.end()
			setHire(id, "failed", "Its window didn't open.")
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
	cleanUpHire(hire, reason, hireOwnTmux(hire, testPort, o.Tmux), removeHireWorktree(o.Git, testPort, o.Worktrees),
		func(id, why string) { setHire(id, "failed", firstRunes(why, longestNotes)) })
	// A start script that never ran still holds its send key or token: it goes too.
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
	i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == roleID })
	if i < 0 {
		return 0, "", false
	}
	role, how := d.config.Roles[i], hiredIntoGeneralFolder
	options, testPort := d.engine, testCopyPort()
	if options.HeardFrom == nil {
		options.HeardFrom = d.hireHeardFrom
	}
	if hasTerminal {
		options.Attach = func(socket string) error {
			return openWindow(terminal, folder, []string{tmuxPath(), "-S", socket, "attach", "-t", "=" + hireSession})
		}
	}
	request := engineRequest{Role: role, How: how, Folder: folder, DefaultAccount: true, TestPort: testPort}
	// The GM gets a new token at its hire, in its start script alone; the one it
	// was given is taken away again if the hire doesn't go ahead or never starts.
	token := ""
	if role.ID == gmRole {
		request.GMToken = func() string { token = d.newGMToken(); return token }
	}
	d.mu.Unlock()
	hire, err := runManagedHire(request, options)
	d.mu.Lock()
	if err != nil {
		if token != "" && constantTimeEqual(d.gmToken, token) {
			d.gmToken = "" // d.mu is held: clearGMToken would take it again
		}
		d.lastHire = time.Time{} // nobody was hired, so the next try needn't wait
		if refused, ok := err.(*hireRefusal); ok {
			return refused.Status, refused.Message, true
		}
		return http.StatusInternalServerError, err.Error(), true
	}
	d.seatEngineHire(hire.TmuxSocket, held, floor)
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
