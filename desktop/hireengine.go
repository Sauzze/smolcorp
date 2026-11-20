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
	// StoreLook is the fire's look at the repo's stored objects before its merged look (firefsck.go): nil is the real
	// one. A test that stands in Git for a repo that isn't one stands this in too.
	StoreLook storeLook
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

// hiresSandboxed is whether a role's Claude hire made now starts in its sandbox: on a Mac, with the owner's switch
// as held now (AQ1), as managedHireWith gives the engine its options. With the lock held.
func (d *Deck) hiresSandboxed() bool {
	o := d.engine
	o.SandboxOff = d.config.SandboxOff
	return o.sandboxing()
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

// hirePlaceCheck is the engine's check of a hire's place (hireRequest.Check), for desk id in role, started in its
// sandbox when sandboxed: hirePlaceWordsAs on its worktree, or else on the folder as prepareHire's rule judged it
// (judged), never the folder asked for, whose links would be resolved again for git, after the rule's look
// (FXGMHIRENOGIT126R3, B1).
func hirePlaceCheck(sandboxed bool, id, role string, o engineOptions) func(hirePlace, string) error {
	return func(p hirePlace, judged string) error {
		if !sandboxed {
			return nil
		}
		at := deskAtOf(Hire{ID: id, Role: role, Folder: keptPath(judged), Worktree: keptPath(p.Worktree), Repo: p.Repo}, "", nil, o)
		if at.Projects != "" { // a GM's workspace is its place
			return nil
		}
		if words := hirePlaceWordsAs(at.Root, o.Git, placedInGMFolder(p.Note)); words != "" {
			return &hireRefusal{Status: http.StatusConflict, Message: words}
		}
		return nil
	}
}

// hirePlansRoot opens the folder a hire made at place starts in, for its plans (makeDeskPlansIn), on the place its hire
// checked, never by the folder's path again (FXGMHIRENOGIT126R5): in its worktree, through the root the carry kept
// (carriedTreeRoot), else the worktree as made, opened with no link on its way (openResolvedRoot); in a folder with no
// worktree, the folder as the rule judged it (place.Resolved), opened so. A GM with a workspace starts in the folder it
// reads, by its path, as at every start of it (R15: it only reads there, and its plans folder there is its own); so
// does a folder whose links couldn't be resolved, and any folder off a Mac, where no desk starts in a sandbox yet.
func hirePlansRoot(place hirePlace, gmWorkspace bool) (*os.Root, error) {
	switch {
	case place.Worktree != "":
		sub, err := filepath.Rel(place.Worktree, place.Folder)
		if err != nil || !filepath.IsLocal(sub) && sub != "." {
			return nil, errDeskClaude
		}
		if root, found, err := carriedTreeRoot(place.Branch, sub); found {
			return root, err
		}
		return openResolvedRoot(place.Folder)
	case place.Resolved != "" && !gmWorkspace:
		return openResolvedRoot(place.Resolved)
	}
	return os.OpenRoot(place.Folder)
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
	// A desk in its sandbox on a Mac never works in the whole folder for hires, nor in a folder that holds it
	// (FXHIREPROJECTSR4), nor in a folder in no repo that holds one (FXHIREPROJECTSR5): refused once its place is
	// known, before anything is trusted or written there, by whatever route it came (Setup's folder, a recent one, the
	// GM's); the office's own hire picks a project first (hireWith).
	// Its place's check and making have one deadline (hirePlaceDeadline): no git of them starts past it, so nothing
	// there holds the hire, and the deck's hires with it, unbounded (RT126SWEEP4 part 3's B1). Its undos aren't held to
	// it: they take back what was made.
	placed := o
	placed.Git = gitUntil(o.Git, time.Now().Add(hirePlaceDeadline))
	check := hirePlaceCheck(tmux.sandboxed, id, r.Role.ID, placed)
	place, err := prepareHire(hireRequest{HireID: id, Name: name, Folder: r.Folder, Claude: true, ConfigDir: r.ConfigDir, Held: r.Held,
		APIKey: r.APIKey, Worktree: !r.NoWorktree, Check: check, KeepCarried: true, GM: r.Role.ID == gmRole},
		hirePlaceOptions{Worktrees: o.Worktrees, Taken: takenPorts(), Git: placed.Git, PortFree: o.PortFree, Key: key, Hold: true})
	defer releaseHirePorts(place.Ports) // its row holds them by then, or it was refused
	if err != nil {
		return Hire{}, err
	}
	// What the core carried for its worktree stays on record until this hire ends (FXGMHIRENOGIT126R5, R14): every
	// undo below takes the tree back through the carry's roots (takeBackTree), never by its path, which whoever can
	// write the folder for hires could have turned into a link since prepareHire checked it; git's status, then its
	// removal, would run where that link leads. Once its agent may have worked there (its start), a tree holding
	// changes is kept, as before, and one turned since gets no git (takeBackWorkedTree).
	defer forgetCarried(place.Branch)
	agentMayHaveRun := false
	undo := func() {
		if place.Worktree != "" {
			if agentMayHaveRun {
				takeBackWorkedTree(place.Repo, place.Worktree, place.Branch, o.Git)
			} else {
				takeBackTree(place.Repo, place.Worktree, place.Branch, o.Git)
			}
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
	// A GM hired into a folder that isn't a repo works in its sandbox in a workspace of its own, and reads the folder
	// it starts in (FXGMROOT): its settings let Claude Code work there, and its introduction says so.
	workspace := ""
	if tmux.sandboxed {
		workspace = gmWorkspace(r.Role.ID == gmRole, id, place.Repo, place.Worktree)
	}
	// A Claude Code desk gets its own Claude folder (SA-X8), made below, and its plans kept outside it.
	claude := agentOrClaude(r.Role.Agent) == Claude
	settings, err := writeHireSettings(hireSettingsOptions{HireID: id, Auto: engineAutoMode, Profile: profile, CLI: o.CLI, Support: supportDir,
		ReportPort: r.TestPort, Profiles: r.Profiles, Run: o.Guard, IsExec: o.IsExec, OwnClaude: claude, Kept: deskKeptFolder(runtime.GOOS, tmux.sandboxed, id),
		Workspace: workspace, ReadsOwnFolder: !engineAutoMode})
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
	hire := Hire{ID: id, Name: r.Role.Name, Nickname: r.Nickname, Account: r.AccountName, Role: r.Role.ID, Folder: keptPath(r.Folder), Branch: place.Branch, Worktree: keptPath(place.Worktree), Repo: place.Repo,
		TmuxSocket: tmux.socket, TmuxProgram: tmux.program, Model: r.Role.Model, Ports: place.Ports, Brief: brief, Started: float64(o.Now().UnixMilli()), State: "starting", Notes: place.Note,
		Scope: r.Scope, ScopeRoles: r.ScopeRoles}
	if place.Worktree == "" {
		hire.FolderResolved, hire.FolderID = place.Resolved, place.ResolvedID // checked at its every start (deskPlacedWords)
	}
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
		plans := "" // its plans folder where it starts, named in its words with its workspace
		if workspace != "" {
			plans = filepath.Join(place.Folder, filepath.FromSlash(deskPlansFor(id)))
		}
		start.Message += "\n\n" + gmIntroduction(place.Folder, workspace, plans, start.GMPlugin != "" && start.GMCLI == "")
		if deskSandboxed && o.SandboxOff {
			start.Message += " " + gmSandboxOffLine // told it starts outside its sandbox (§A5.2, AQ1)
		}
	}
	// A desk in its sandbox is told plainly which branches are its own (FXGITROWS2R5): the sandbox refuses the others,
	// tags and git stash with git's "Operation not permitted" alone.
	start.Message += deskStartWords(deskBranchWords(runtime.GOOS, tmux.sandboxed, r.Role.ID == gmRole, place.Branch))
	// A desk in its sandbox is told plainly that only its own place in the folder for hires is its to change
	// (FXHIREPROJECTS): the sandbox refuses the rest with no words of its own. A GM with a workspace has its own words.
	if tmux.sandboxed && workspace == "" {
		root := place.Folder
		if place.Worktree != "" {
			root = place.Worktree
		}
		if words := deskProjectsWords(root, place.Worktree != ""); words != "" {
			start.Message += "\n\n" + words
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
		// Its folder trusted as prepareHire trusted it in the account's file: the worktree, links resolved, in the very
		// spelling checked at its hire (trustedSpelling), never its links resolved again (FXGMHIRENOGIT126R5).
		desk := deskClaude{Account: r.ConfigDir, Trusted: trustedSpelling(place), MCP: r.MCPServers}
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
		// Its plans, in its place (deskPlansDirectory): the fence denies its tools its Claude folder. Made through a
		// root on the place checked at its hire (hirePlansRoot), never by the folder's path again, which a link put on
		// its way since would lead elsewhere (FXGMHIRENOGIT126R5); undone through the same root.
		plans, err := hirePlansRoot(place, workspace != "")
		if err != nil {
			return fail(http.StatusInternalServerError, "Smolcorp couldn't make the hire's plans folder.")
		}
		defer plans.Close()
		if makeDeskPlansIn(plans, id) != nil {
			return fail(http.StatusInternalServerError, "Smolcorp couldn't make the hire's plans folder.")
		}
		if place.Worktree == "" { // in a folder others share: a start refused from here takes its plans folder too
			undoMade := undo
			undo = func() {
				_ = removeDeskPlansIn(plans, id)
				undoMade()
			}
		}
	}
	// Its sandbox, made from where it works, checked before anything of it runs (desksandbox.go): a Mac's every desk.
	if tmux.sandboxed {
		undoPlaced := undo
		undo = func() {
			// Its sandbox let go; its socket folder, when nothing answers. First, what a refused start left running in
			// its sandbox is ended (FXFIREREAPR1): one that can't be keeps its row, marked failed, its worktree and its
			// folders, for a fire to end it.
			if why := deskStopped(hire, o); why != "" {
				failHire(id, why+startReapKept)
				return
			}
			undoPlaced()
		}
	}
	desk := deskAt{ID: id, GM: r.Role.ID == gmRole, Root: place.Folder, Socket: tmux.socket, Accounts: r.Profiles, Brief: brief, Git: o.Git,
		DatabaseOptIn: o.DatabaseOptIn, InGMFolder: placedInGMFolder(place.Note), Worktrees: o.Worktrees}
	if place.Worktree != "" {
		desk.Root, desk.Checkout = place.Worktree, place.Repo
		// Its tree checked as at every start after, from its row (deskAtOf): still the very folder made at its hire, so
		// a swap since prepareHire's check is refused here, at its first start (FXGMHIRENOGIT126R5, R13).
		desk.Tree, desk.TreeRepo = place.Worktree, place.Repo
	}
	if place.Repo != "" {
		desk.Hires = hiresFolder(place.Repo, o.Worktrees)
	}
	if workspace != "" {
		desk.Root, desk.Projects = workspace, place.Folder // made with its sandbox (makeGMWorkspace)
	}
	if place.Worktree == "" && workspace == "" {
		// Its folder checked as at every start after, from its row (deskAtOf): still the one the rule judged.
		desk.CheckPlaced, desk.Placed, desk.PlacedID = true, hire.FolderResolved, hire.FolderID
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
	agentMayHaveRun = true // its pane is asked for now: from here its agent may have worked in its tree
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
		// Seen gone: its sandbox let go, and its socket folder (SA-M6), once what it started is ended (FXFIREREAPR1).
		if why := deskStopped(hire, o); why != "" {
			return Hire{}, goneWhileStarting(hire, "its tmux session was ended. "+why+strings.TrimSuffix(startReapKept, "."))
		}
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
			if why := deskStopped(hire, o); why != "" {
				failHire(id, "Its window didn't open. "+why+startReapKept)
			} else {
				undoMade()
			}
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
	// Its sandbox, and a sandboxed desk's socket folder once nothing answers there. One with a program still running
	// in its sandbox keeps its folders, as cleanUpHire kept its worktree (FXFIREREAPR1).
	if deskStopped(hire, o) != "" {
		return
	}
	// Its own Claude folder goes, as at a fire (SA-X8), and its plans from a folder others share.
	_ = removeDeskClaude(supportDir, hire.ID)
	if hire.Worktree.none() {
		removeHirePlans(hire)
	}
	if hire.Role == gmRole {
		removeGMWorkspace(hire.ID) // a GM's workspace too (FXGMROOT)
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
	return d.managedHireWith(roleID, folder, "", terminal, hasTerminal, held, floor, nil)
}

// managedHireWith is managedHire with the project the office picked for it in the folder for hires, by its name there
// (folder is then that project; "" for none picked), and what setup's own hire says besides, or nil.
//
// gitBy, when given and not zero, is the deadline of a hire the General Manager asked for (gmHireDeadline): no git of
// the hire starts after it (gitUntil).
func (d *Deck) managedHireWith(roleID, folder, picked string, terminal hireOpener, hasTerminal bool, held, floor int, extra *setupHire, gitBy ...time.Time) (status int, message string, handled bool) {
	i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == roleID })
	if i < 0 {
		return 0, "", false
	}
	role, how := d.config.Roles[i], hiredIntoGeneralFolder
	options, testPort := d.engine, testCopyPort()
	options.SandboxOff = d.config.SandboxOff // the owner's switch, as held now (AQ1)
	options.DatabaseOptIn = d.config.LocalDatabaseAsksPassword
	options.Worktrees = d.worktreesFolder(options.Worktrees)
	if len(gitBy) > 0 && !gitBy[0].IsZero() {
		options.Git = gitUntil(options.Git, gitBy[0])
	}
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
	request := engineRequest{Role: role, How: how, Folder: folder, Picked: picked, DefaultAccount: true, TestPort: testPort, Desk: briefDesk(held, floor)}
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
