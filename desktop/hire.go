package desktop

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Hiring from the office, as the Mac app's Deck.hire: a new terminal window
// with the agent set up for new hires at work in it, or a role's agent,
// started on the role. Hired at a desk whose key is pinned to a project, it
// starts in that project's folder, so it sits down at that desk; otherwise in
// the folder set up for new hires.
//
// The window runs the launcher, `smolcorp run --hire <token> <agent>`, or
// `--role <id>` for a role, so the deck can press its keys as for any agent
// started through Smolcorp. The token is the deck's own, made for this hire:
// the launcher hands it back as it connects, and the desk it was hired at
// goes to the first session in it. Only ids and folders are on its command
// line, never a role's instructions, which the launcher reads from setup.
//
// Where the window opens is a terminal installed here (see hireOpeners), or
// a tmux session of its own, with a window showing it; with no desktop to
// open a window on, as over SSH, the tmux session alone.
//
// On Windows, with the beta switch on, a Claude role's hire goes through the
// launcher engine instead (hirelauncher.go): the window runs the launcher, which
// asks the app for the hire's start environment once, so no secret is on a command line.

const (
	// How long after a hire the next is refused. A second tap, or a second
	// phone, would otherwise open a second window before the first new hire
	// has reached the deck.
	hireCooldown = 5 * time.Second
	// How long a hire's launcher has to connect and its agent to report, to
	// take its desk: long enough for a trust prompt left waiting a while.
	hiredTabWait = 15 * time.Minute
)

// pendingHire is a hire whose launcher hasn't connected yet.
type pendingHire struct {
	// The desk it was hired at, with a set number of desks, or -1; and its
	// floor, 0 for the ground floor, whose desks are the keys.
	desk  int
	floor int
	at    time.Time
	// For an engine hire (hirelauncher.go): its id in the registry, which its launcher
	// then carries; and what its launcher may ask for once, before the agent starts.
	hireID string
	env    *hireStartEnv
}

// hireToken is the only shape of token a launcher may hand back: newID's.
var hireToken = regexp.MustCompile(`^[0-9A-F]{32}$`)

// hireOpener is a terminal a hire's window can open in.
type hireOpener struct {
	id, name string
	// program is what's looked for on PATH to know it's installed.
	program string
	// args are the terminal's arguments to run command in folder.
	args func(folder string, command []string) []string
	// open, where set, opens the window itself, in place of args: the Mac
	// app's Terminal, iTerm2 and Ghostty under its engine (hire_darwin.go).
	// It gives the window's record, or "" for none.
	open func(folder string, command []string) (window string, err error)
}

// installedOpeners are the terminals here that can open a hire's window, in
// the order they're offered. A variable, for the tests.
var installedOpeners = func() []hireOpener {
	var found []hireOpener
	for _, opener := range hireOpeners {
		if opener.program == "" {
			found = append(found, opener)
		} else if _, err := exec.LookPath(opener.program); err == nil {
			found = append(found, opener)
		}
	}
	return found
}

// tmuxPath is tmux, where it's installed and this system has it; a variable,
// for the tests.
var tmuxPath = func() string {
	if !hasTmuxHires {
		return ""
	}
	path, _ := exec.LookPath("tmux")
	return path
}

// canOpenWindows is whether there's a desktop to open a hire's window on: a
// variable, for the tests.
var canOpenWindows = hasDesktop

// startHire starts a hire's terminal, and lets it run: a variable, for the
// tests.
var startHire = startAndWait

// runTmux runs tmux and gives what it printed: a variable, for the tests.
var runTmux = func(args ...string) (string, error) {
	cmd, cancel := terminalCommand(tmuxPath(), args...)
	defer cancel()
	cmd.Env = hireEnvironment()
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

// hireWays is where hires can open: the terminals installed, and whether
// tmux is there. With no desktop, no terminal can open a window.
func hireWays() (terminals []hireOpener, tmux bool) {
	if canOpenWindows() {
		terminals = installedOpeners()
	}
	return terminals, tmuxPath() != ""
}

// hireTerminal is the terminal hires open in: the one chosen in setup while
// it's installed, else the first that is, or none.
func hireTerminal(chosen string, terminals []hireOpener) (hireOpener, bool) {
	if i := slices.IndexFunc(terminals, func(o hireOpener) bool { return o.id == chosen }); i >= 0 {
		return terminals[i], true
	}
	if len(terminals) == 0 {
		return hireOpener{}, false
	}
	return terminals[0], true
}

// canHireHere is whether a hire could open anywhere at all.
func canHireHere() bool {
	terminals, tmux := hireWays()
	return len(terminals) > 0 || tmux
}

// unfencedNote is the office's line for a hire that won't get Smolcorp's
// fence (Claude Code's deny rules and the guard, hiresettings.go), or "" for
// one that will. It follows hireAs: the fence is written only for a role's
// Claude Code hire made by the engine, in tmux (hireengine.go), or on Windows
// in the launcher's window behind the beta switch (hirelauncher.go). A hire at
// a pinned desk, or on a floor with a folder of its own, takes the old way too;
// the snapshot can't tell which desk will be held, so those get no line.
func (d *Deck) unfencedNote(roleID, program string, terminals []hireOpener, tmux bool) string {
	agent := Agent(program)
	if agent != Claude {
		if name := agent.Name(); name != "" {
			return name + " hires don't get Smolcorp's fence: they run with their own settings."
		}
		return "This hire doesn't get Smolcorp's fence: it runs with its own settings."
	}
	terminal, hasTerminal := hireTerminal(d.config.HireTerminal, terminals)
	if roleID == "" {
		// Under the Mac app, with the manager on, it isn't started at all (hireAsWith).
		if d.config.ManagerEnabled && underMacApp(terminal, hasTerminal) {
			return macHireNoRoleUnfenced
		}
		return "A hire with no role doesn't get Smolcorp's fence: it runs with its own settings."
	}
	inTmux := tmux && (d.config.HireInTmux || !hasTerminal || underMacApp(terminal, hasTerminal))
	if hasTmuxHires && inTmux || !inTmux && launcherHires && d.config.HireManagedWindows && hasTerminal {
		return ""
	}
	if !hasTmuxHires {
		return "On Windows, only managed hires (a beta, off by default) get Smolcorp's fence: this one runs with its own settings."
	}
	return "Outside tmux, this hire doesn't get Smolcorp's fence: it runs with its own settings. Start inside tmux, in setup, gives it the fence."
}

// hire opens a new hire's terminal, as the Mac app's: see the top of this
// file. desk and roleID are as the phone sent them, either empty.
func (d *Deck) hire(desk, roleID string) (int, string) {
	return d.hireAs(desk, roleID, 0, false)
}

// hireUpstairs is a hire at an empty desk on floor, 1 for 2F up to 5 for 6F,
// from the office: it sits there. It starts in the floor's folder when the
// floor has one rule (see FloorRule), else in the folder for new hires.
func (d *Deck) hireUpstairs(floor, desk, roleID string) (int, string) {
	n, err := strconv.Atoi(floor)
	if err != nil || n < 0 || n >= floorCount {
		return http.StatusBadRequest, "Unknown floor"
	}
	return d.hireAs(desk, roleID, n, false)
}

// hireFromSetup is a hire of roleID asked for on the setup page, on this
// computer: hiring needn't be on for the phones. With no folder for new hires
// it's refused (setupHireNeedsFolder), never started in your home folder.
func (d *Deck) hireFromSetup(roleID string) (int, string) {
	return d.hireAs("", roleID, 0, true)
}

// hooksNotConnected is why a hire of agent can't go ahead, or "" when that
// agent reports to this Smolcorp, as the setup checklist says. A session only
// exists once a hook posts, so a hire made without hooks would never show up
// at a desk. Other agents' hooks don't matter to this hire, and an agent with
// no settings folder yet, which Connect can't reach, is left to its terminal.
// Only Connect in setup is named: smolcorp setup leaves agents you disconnected.
var hooksNotConnected = hooksRefusal

func hooksRefusal(agent Agent) string {
	if !slices.Contains(foundAgents(), agent) {
		return ""
	}
	if c := hooksCheck([]Agent{agent}, isConnected, hooksPointHere); c.Status == checkFix {
		return "Connect your agents first: " + c.Detail + " Press Connect in setup."
	}
	return ""
}

// setupHireNeedsFolder is a hire from the setup page with no folder for your
// hires, or one that's gone: the page points at Start here's folder step. The
// same words as the GM route's refusal (POST /setup/hire/gm) on both apps.
const setupHireNeedsFolder = "Choose the folder for your hires first: it's the first step of Start here."

// Refusals of a hire that would otherwise start without its fence, or on the wrong Claude account.
const (
	macHireNeedsTmux      = "Hires on this engine need tmux on your Mac to start fenced: install it, or switch back to the Swift engine."
	macHirePinnedUnfenced = "On this engine, a hire at a desk or floor pinned to a folder can't start fenced yet: hire at another desk, or switch back to the Swift engine."
	// The GM's route names a floor, never a desk: only a floor's own folder pins it, and another floor is its way out.
	macGMHirePinnedUnfenced = "On this engine, a hire on a floor with a folder of its own can't start fenced yet: hire on another floor, or switch back to the Swift engine."
	// Setup's sheet picks the desk pinned to the folder itself, so another desk isn't its way out.
	macSetupHirePinnedUnfenced = "On this engine, a hire in a folder a desk or floor is pinned to can't start fenced yet: unpin that folder, or switch back to the Swift engine."
	// A role whose agent isn't one Smolcorp knows: the Mac starts it as Claude Code, fenced.
	macHireUnknownAgent = "On this engine, a role whose agent Smolcorp doesn't know can't start fenced yet: set its agent to Claude Code in setup, or switch back to the Swift engine."
	// Anyone's hire (no role) with the manager on: the Mac fences it, the engine only hires a role.
	macHireNoRoleUnfenced   = "On this engine, a hire with no role can't start fenced yet: pick a role, or switch back to the Swift engine."
	plainHireDefaultAccount = "This hire would start on your default Claude account, so it wasn't started: hire in tmux, at a desk that isn't pinned."
	// Setup's hire that went the plain way, said with its answer (setuphire.go).
	setupHireNotPassedOn = "It starts the old way: your task and answers weren't passed on, and it asks its role's questions itself."
)

// underMacApp is whether hires open through the Mac app (`smolcorp core`,
// hire_darwin.go): there, a role's Claude hire is always the engine's.
func underMacApp(terminal hireOpener, hasTerminal bool) bool {
	return host != nil || hasTerminal && terminal.open != nil
}

func (d *Deck) hireAs(desk, roleID string, floor int, fromSetup bool) (int, string) {
	return d.hireAsWith(desk, roleID, floor, fromSetup, nil)
}

// hireAsWith is hireAs, with what setup's own hire says besides (setuphire.go),
// or nil: its folder, answers, task, account, model and nickname.
func (d *Deck) hireAsWith(desk, roleID string, floor int, fromSetup bool, extra *setupHire) (int, string) {
	return d.hireWith(hireSpec{desk: desk, roleID: roleID, floor: floor, fromSetup: fromSetup, extra: extra})
}

// hireSpec is a hire's request, as hireAs, setup's hire and POST /setup/gm/hire make it: the one hire path.
type hireSpec struct {
	desk, roleID string
	floor        int
	fromSetup    bool
	extra        *setupHire // setup's own hire (setuphire.go), or nil
	folder       string     // asked for and checked by the caller; "" is the folder for new hires
	fenced       bool       // only a fenced way (the tmux engine, or the launcher's): 409 where a plain terminal would open
	seat         *int       // when set, is given the desk the hire takes
}

// fenceRefusal is why a fenced hire of program can't be made, or "": a plain terminal has no guard.
// Under the Mac app, a missing tmux (M4's words) and a pinned folder are said in the engine's own words.
func fenceRefusal(program, pinned string, tmux, launcher, macApp bool) string {
	switch {
	case Agent(program) != Claude:
		return "The General Manager hires only Claude roles: Smolcorp can't fence another agent."
	case pinned != "" && macApp:
		return macGMHirePinnedUnfenced
	case pinned != "":
		return "The General Manager doesn't hire into a pinned desk's or a floor's own folder."
	case tmux, launcherHires && launcher:
		return ""
	case macApp:
		return macHireNeedsTmux
	case launcherHires:
		return "Turn on managed windows in Setup: the General Manager hires only into a hire Smolcorp can fence."
	}
	return "Install tmux: the General Manager hires only into a hire Smolcorp can fence."
}

// hireWith is hireAs for a hireSpec.
func (d *Deck) hireWith(s hireSpec) (int, string) {
	desk, roleID, floor, fromSetup, extra := s.desk, s.roleID, s.floor, s.fromSetup, s.extra
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.config.HiringEnabled && (!fromSetup || s.fenced) {
		return http.StatusForbidden, hiringOff
	}
	pinned := ""
	// Any free desk can be hired at, and the hire sits there; with no desk
	// chosen, it takes the first free one.
	held := -1
	if floor > 0 {
		// Upstairs, at a desk nobody sits at: its floor's folder, if the floor has one.
		_, overflow := d.layout()
		seated, _ := d.seatedBuilding(overflow)
		seatedAt := func(i int) bool {
			return slices.ContainsFunc(seated[floor], func(p seatedPerson[*Session]) bool { return p.desk == i })
		}
		index, err := strconv.Atoi(desk)
		if desk == "" && (extra != nil || s.fenced) {
			// From setup or the GM route, which name a floor and no desk: its first free one.
			index, err = 0, nil
			for index < desksPerFloor && seatedAt(index) {
				index++
			}
			if index == desksPerFloor && s.fenced {
				return http.StatusConflict, checkedFloors(d.config.Floors)[floor].Name + " is full."
			}
		}
		if err != nil || index < 0 || index >= desksPerFloor {
			return http.StatusBadRequest, "Unknown desk"
		}
		if seatedAt(index) {
			return http.StatusConflict, "Someone's at that desk already."
		}
		if !d.hasRoom(floor, "") {
			return http.StatusConflict, checkedFloors(d.config.Floors)[floor].Name + " is full."
		}
		held = index
		if s.seat != nil {
			*s.seat = index
		}
		pinned, _ = existingFolder(floorFolder(d.config.FloorRules, floor))
	} else if desk != "" {
		keys, _ := d.layout()
		// With Auto, the office also offers the desks after the last: a hire adds them.
		index, err := strconv.Atoi(desk)
		if err != nil || index < 0 || index >= mostKeys {
			return http.StatusBadRequest, "Unknown desk"
		}
		if index < len(keys) && keys[index].pin != nil {
			pinned, _ = existingFolder(*keys[index].pin)
		}
		if index >= len(keys) || keys[index].pin == nil {
			if index < len(keys) && keys[index].session != nil {
				return http.StatusConflict, "Someone's at that desk already."
			}
			// Any free desk can be hired at, and the hire sits there. The first free one (or the
			// one after the last) is where a hire with no desk goes anyway, so it isn't held.
			first := slices.IndexFunc(keys, func(k deckKey) bool { return k.pin == nil && k.session == nil })
			if first < 0 {
				first = len(keys)
			}
			if index != first {
				held = index
			}
		}
	} else if s.seat != nil {
		if !d.hasFreeKey() {
			return http.StatusConflict, "Every desk on the ground floor is taken. Fire someone there, or free a desk, to make room."
		}
		keys, _ := d.layout()
		if *s.seat = slices.IndexFunc(keys, func(k deckKey) bool { return k.pin == nil && k.session == nil }); *s.seat < 0 {
			*s.seat = len(keys)
		}
	}
	folder := pinned
	if extra != nil && extra.Folder != "" {
		// The folder setup chose, checked already, before a desk's or a floor's.
		folder = extra.Folder
	} else if folder == "" && s.folder != "" {
		// The GM route's folder, checked already; a pinned one is refused (fenceRefusal).
		folder = s.folder
	}
	if folder == "" {
		var ok bool
		if folder, ok = existingFolder(d.config.HireFolder); !ok && fromSetup {
			return http.StatusConflict, setupHireNeedsFolder
		}
		if !ok {
			return http.StatusConflict, "Choose a folder for new hires in setup, on your computer."
		}
	}
	program := d.config.HireAgent
	if roleID != "" {
		i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == roleID })
		if i < 0 {
			return http.StatusNotFound, "That role isn't there any more. Pick another."
		}
		program = d.config.Roles[i].Agent
	}
	// T721, on every hire route, before anything is made or the cooldown is spent. Only setup's
	// hire names an account (a test copy never hires on the default one).
	configDir := ""
	if extra != nil {
		configDir = extra.configDir
	}
	if err := testCopyRefusal(testCopyPort(), program, configDir); err != nil {
		return refusalStatus(err), err.Error()
	}
	// One GM per office, by whichever route it is hired.
	if roleID == "gm" {
		if err := d.gmRefusal(); err != nil {
			return refusalStatus(err), err.Error()
		}
	}
	if d.hiring || time.Since(d.lastHire) < hireCooldown {
		return http.StatusTooManyRequests, "Hired a moment ago. Try again in a few seconds."
	}
	terminals, tmux := hireWays()
	terminal, hasTerminal := hireTerminal(d.config.HireTerminal, terminals)
	if !hasTerminal && !tmux {
		return http.StatusConflict, "There's no terminal here to open a hire in."
	}
	macApp, claudeRole := underMacApp(terminal, hasTerminal), roleID != "" && agentOrClaude(program) == Claude
	// A fenced hire (the GM route's) that no fenced way could take is refused here, before anything is made.
	if why := fenceRefusal(program, pinned, tmux, d.config.HireManagedWindows && hasTerminal, macApp); s.fenced && why != "" {
		return http.StatusConflict, why
	}
	d.lastHire = time.Now()
	// Until it's done, however long the lock is let go, which may be longer
	// than the cooldown: no second hire takes the same desk meanwhile.
	d.hiring = true
	defer func() { d.hiring = false }() // with the lock held again
	// Its agent must be there where the hire runs, or setup says why here,
	// rather than the hire's window in a line of its own.
	d.mu.Unlock()
	missing := agentMissing(program)
	d.mu.Lock()
	if missing != "" {
		d.lastHire = time.Time{}
		return http.StatusConflict, missing
	}
	// After the agent is known to be there: not installed is the clearer thing
	// to say. Without the lock: it reads the agents' settings files.
	// A test copy's Claude hire reports through its own settings file, not the account's hooks (as the Mac's).
	d.mu.Unlock()
	why := ""
	if testCopyPort() == 0 || agentOrClaude(program) != Claude {
		why = hooksNotConnected(Agent(program))
	}
	d.mu.Lock()
	if why != "" {
		d.lastHire = time.Time{}
		return http.StatusConflict, why
	}

	// In tmux when chosen, or when there's no window to open. Agent windows in
	// the background (T796): in tmux, with no window, as without a terminal.
	// Under the Mac app, a role's Claude hire is the engine's whatever the tmux
	// choice: it runs in its own tmux server and its window is attached through
	// the app, so it is fenced as the Swift engine's are. A fenced hire (the GM
	// route's) is in tmux whenever there is tmux.
	background := backgroundHires(d.config, tmux)
	inTmux := tmux && (d.config.HireInTmux || !hasTerminal || background || macApp && claudeRole || s.fenced)
	showWindow := hasTerminal && !background
	// A role's Claude hire goes through the engine, at the desk chosen too (it
	// is seated there when its session appears); the rest open as they did.
	// A pinned desk stays on the old path: the engine's hire sits in a
	// worktree, whose session is a project of its own, so it wouldn't be seated
	// at its pinned desk.
	if roleID != "" && pinned == "" && Agent(program) == Claude {
		if inTmux {
			if status, message, handled := d.managedHireWith(roleID, folder, terminal, showWindow, held, floor, extra); handled {
				return status, message
			}
		} else if launcherHires && d.config.HireManagedWindows && hasTerminal {
			// Beta: in a window run by the launcher, only where that is made and the switch is on.
			if status, message, handled := d.managedLauncherHire(roleID, folder, terminal, hasTerminal, held, floor); handled {
				return status, message
			}
		}
	}
	// The plain path, through the launcher: it writes no fence and starts on the default account.
	// Under the Mac app, a hire the Mac would manage (the manager on, a worktree asked for, or the
	// GM) starts fenced or not at all, as the Swift engine's Deck.open does.
	if macApp && claudeRole && (d.config.ManagerEnabled || extra != nil && extra.Worktree || roleID == gmRole) {
		d.lastHire = time.Time{}
		switch {
		case !tmux:
			return http.StatusConflict, macHireNeedsTmux
		case Agent(program) != Claude:
			// Not pinned: the engine only took it as Claude Code if its agent says so.
			return http.StatusConflict, macHireUnknownAgent
		case extra != nil:
			return http.StatusConflict, macSetupHirePinnedUnfenced
		}
		return http.StatusConflict, macHirePinnedUnfenced
	}
	// A hire with no role isn't the engine's: under the Mac app, with the manager on, it would start
	// unfenced. Agent(program): the hire agent always loads as a known one (loadConfig, as the
	// Mac's Config), so this is the Mac's hiredAgent(role: nil).
	if macApp && roleID == "" && Agent(program) == Claude && d.config.ManagerEnabled {
		d.lastHire = time.Time{}
		return http.StatusConflict, macHireNoRoleUnfenced
	}
	// The backstop of a fenced hire (the GM route's) that no fenced way took: never a plain terminal.
	// fenceRefusal has already refused every such hire before anything was made, so this isn't reached
	// while the two agree: it is there in depth, so a change to one can't open a plain terminal unseen.
	if s.fenced {
		d.lastHire = time.Time{}
		return http.StatusConflict, "The General Manager hires only into a hire Smolcorp can fence, and this one couldn't be."
	}
	// The launcher can't start on a named Claude account, and a test copy never hires on the default one.
	if agentOrClaude(program) == Claude && (configDir != "" || testCopyPort() != 0) {
		d.lastHire = time.Time{}
		return http.StatusConflict, plainHireDefaultAccount
	}
	token := newID()
	command := []string{commandLineProgram(), "run", "--hire", token}
	if pinned != "" && floor == 0 {
		command = append(command, "--pinned")
	}
	if roleID != "" {
		// The launcher reads the role again: it must still have the agent looked for.
		command = append(command, "--agent", program, "--role", roleID)
	} else {
		// The agent looked for, whatever setup changed meanwhile.
		command = append(command, program)
	}
	// A plain hire (not the engine's: a pinned desk, another agent, no role)
	// isn't on a tmux server of its own, so in the background nothing but
	// `tmux attach` would show it, and it has no "Open a window": it opens in
	// its window as before (T796).
	plainInTmux := tmux && (d.config.HireInTmux || !hasTerminal)
	// Opened outside the lock: the terminal may take a moment to start.
	d.mu.Unlock()
	err := openHire(command, folder, terminal, hasTerminal, plainInTmux)
	d.mu.Lock()
	if err != nil {
		// Nobody was hired, so the next try needn't wait.
		d.lastHire = time.Time{}
		name := terminal.name
		if plainInTmux {
			name = "tmux"
		}
		return http.StatusInternalServerError, name + " didn't open"
	}
	for kept, pending := range d.hires {
		if time.Since(pending.at) > hiredTabWait {
			delete(d.hires, kept)
		}
	}
	d.hires[token] = &pendingHire{desk: held, floor: floor, at: time.Now()}
	if extra != nil {
		// Setup's sheet is told: only the engine's hires get its task and answers, in a BRIEF.
		return http.StatusNoContent, setupHireNotPassedOn
	}
	return http.StatusNoContent, ""
}

// agentMissing is why a hire of an agent's program can't start, or "": a
// variable, for the tests.
var agentMissing = hireAgentMissing

// agentNotFoundMessage says that the login shell hires open in doesn't find
// program, and how to put that right: with the folder Smolcorp itself finds
// it in, foundAt, when there's one.
func agentNotFoundMessage(program, shell, foundAt string) string {
	name := Agent(program).Name()
	if name == "" {
		name = program
	}
	shell = filepath.Base(shell)
	file := loginFile(shell)
	if foundAt == "" {
		return fmt.Sprintf("Can't hire: %s isn't installed, or your login shell (%s), where hires start, doesn't find %s. "+
			"Install %s, or add the folder it's in to your PATH in %s, then hire again.", name, shell, program, name, file)
	}
	folder := filepath.Dir(foundAt)
	line := fmt.Sprintf(`export PATH="%s:$PATH"`, strings.Replace(folder, homeDir, "$HOME", 1))
	if shell == "fish" {
		line = "fish_add_path " + strings.Replace(folder, homeDir, "~", 1)
	}
	return fmt.Sprintf("Can't hire: your login shell (%s), where hires start, doesn't find %s, which is in %s. "+
		"Add this line at the end of %s, then hire again: %s", shell, program, trimmed(folder), file, line)
}

// loginFile is the file a login shell reads that's yours to change, as the
// home folder has it.
func loginFile(shell string) string {
	switch shell {
	case "zsh":
		return "~/.zprofile"
	case "fish":
		return "~/.config/fish/config.fish"
	case "bash":
		// Bash reads the first of these it finds, and ~/.profile only without either.
		for _, name := range []string{".bash_profile", ".bash_login"} {
			if fileExists(filepath.Join(homeDir, name)) {
				return "~/" + name
			}
		}
	}
	return "~/.profile"
}

// whereAppFinds is where program is, as Smolcorp sees it: on its own PATH,
// or in a folder agents' installers put them in. "" when it's in neither.
func whereAppFinds(program string) string {
	if found, err := exec.LookPath(program); err == nil {
		return found
	}
	for _, folder := range []string{".local/bin", ".npm-global/bin", ".bun/bin", ".claude/local"} {
		if candidate := filepath.Join(homeDir, folder, program); fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

// openHire opens command in folder: in a window of terminal, or in a tmux
// session of its own, shown in a window of terminal when there is one.
func openHire(command []string, folder string, terminal hireOpener, hasTerminal, inTmux bool) error {
	if !inTmux {
		return openWindow(terminal, folder, inLoginShell(command))
	}
	name := "smolcorp-" + strings.ToLower(newID()[:8])
	made, err := runTmux(tmuxSessionArguments(name, folder, inLoginShell(command))...)
	if err != nil {
		return err
	}
	if !hasTerminal {
		return nil // over SSH, say: `tmux attach` shows it
	}
	_, socket, _ := strings.Cut(made, " ")
	if socket == "" {
		_, _ = runTmux("kill-session", "-t", name)
		return errUnsupported
	}
	if err := openWindow(terminal, folder, []string{tmuxPath(), "-S", socket, "attach", "-t", name}); err != nil {
		_, _ = runTmux("kill-session", "-t", name)
		return err
	}
	return nil
}

// tmuxSessionArguments are tmux's arguments for a hire's session, running
// command, which print its pane and socket.
func tmuxSessionArguments(name, folder string, command []string) []string {
	return append([]string{"new-session", "-d", "-P", "-F", "#{pane_id} #{socket_path}", "-s", name, "-c", folder}, command...)
}

// openWindow starts terminal, showing command in folder.
func openWindow(terminal hireOpener, folder string, command []string) error {
	_, err := showWindow(terminal, folder, command)
	return err
}

// showWindow is openWindow, and the record of the window it opened, "" for
// none: only a terminal the Mac app opens gives one (hirewindows.go).
func showWindow(terminal hireOpener, folder string, command []string) (string, error) {
	if terminal.open != nil {
		return terminal.open(folder, command)
	}
	args := terminal.args(folder, command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = folder
	cmd.Env = hireEnvironment()
	prepareHireWindow(cmd, terminal.id)
	return "", startHire(cmd)
}

// hireEnvironment is this app's environment, less what says where it was
// started: a hire is in a terminal of its own, not the one Smolcorp was
// started from, nor a headless run. It never carries the GM's token.
func hireEnvironment() []string {
	return slices.DeleteFunc(os.Environ(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return slices.Contains([]string{"SMOLCORP_SESSION", "SAUZZEDECK_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER",
			"SMOLCORP_GM_TOKEN", "TMUX", "TMUX_PANE", "WEZTERM_PANE", "WEZTERM_UNIX_SOCKET", "KITTY_WINDOW_ID", "KITTY_LISTEN_ON"}, strings.ToUpper(name))
	})
}

// hiredDesk takes the desk a hire was made at, or -1, and its floor, for the
// launcher that hands its token back, and whether it was a hire: each token
// once, while it's fresh. With the lock held.
func (d *Deck) hiredDesk(token string) (int, int, bool) {
	desk, floor, _, ok := d.hiredPending(token)
	return desk, floor, ok
}

// hiredPending is hiredDesk and the id of the engine hire the token was made
// for, or "": that id is the launcher's, for good, from here on.
func (d *Deck) hiredPending(token string) (desk, floor int, hireID string, ok bool) {
	pending, found := d.hires[token]
	if !found || !hireToken.MatchString(token) {
		return -1, 0, "", false
	}
	delete(d.hires, token)
	if time.Since(pending.at) > hiredTabWait {
		return -1, 0, "", false
	}
	return pending.desk, pending.floor, pending.hireID, true
}

// hireState is what the setup page shows of hiring: the switch, the folder
// and agent, and where hires open. With the lock held.
func (d *Deck) hireState() map[string]any {
	terminals, tmux := hireWays()
	choices := []map[string]any{}
	for _, terminal := range terminals {
		choices = append(choices, map[string]any{"id": terminal.id, "name": terminal.name})
	}
	chosen, _ := hireTerminal(d.config.HireTerminal, terminals)
	return map[string]any{
		"enabled": d.config.HiringEnabled, "folder": d.config.HireFolder, "agent": d.config.HireAgent,
		"canHire": len(terminals) > 0 || tmux, "canChoose": hasFolderPicker(),
		"terminal": chosen.id, "terminals": choices,
		"tmux": d.config.HireInTmux && tmux, "hasTmux": tmux,
		// Beta: a role's Claude hire through the launcher, where that is made (Windows). Field only.
		"managedWindows": d.config.HireManagedWindows && launcherHires, "managedBeta": launcherHires,
	}
}

// hiredRoleAt is the role of the engine's hire whose tmux server is socket, or
// "" for any other: a session in its pane has no launcher to say which. Only a
// socket in the support folder is looked up.
func (d *Deck) hiredRoleAt(socket string) string {
	if socket == "" || filepath.Dir(filepath.Clean(socket)) != filepath.Clean(supportDir) {
		return ""
	}
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.TmuxSocket != "" && filepath.Clean(h.TmuxSocket) == filepath.Clean(socket) {
			return h.Role
		}
	}
	return ""
}

// refusalStatus is the status a refusal carries, or 409 for an error that isn't one.
func refusalStatus(err error) int {
	if refusal, ok := err.(*hireRefusal); ok {
		return refusal.Status
	}
	return http.StatusConflict
}
