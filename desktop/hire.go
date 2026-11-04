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
// computer: hiring needn't be on for the phones, and with no folder for new
// hires it starts in your home folder.
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

func (d *Deck) hireAs(desk, roleID string, floor int, fromSetup bool) (int, string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !fromSetup && !d.config.HiringEnabled {
		return http.StatusForbidden, hiringOff
	}
	pinned := ""
	// Any free desk can be hired at, and the hire sits there; with no desk
	// chosen, it takes the first free one.
	held := -1
	if floor > 0 {
		// Upstairs, at a desk nobody sits at: its floor's folder, if the floor has one.
		index, err := strconv.Atoi(desk)
		if err != nil || index < 0 || index >= desksPerFloor {
			return http.StatusBadRequest, "Unknown desk"
		}
		_, overflow := d.layout()
		seated, _ := d.seatedBuilding(overflow)
		if slices.ContainsFunc(seated[floor], func(p seatedPerson[*Session]) bool { return p.desk == index }) {
			return http.StatusConflict, "Someone's at that desk already."
		}
		if !d.hasRoom(floor, "") {
			return http.StatusConflict, checkedFloors(d.config.Floors)[floor].Name + " is full."
		}
		held = index
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
	}
	folder := pinned
	if folder == "" {
		var ok bool
		if folder, ok = existingFolder(d.config.HireFolder); !ok && fromSetup {
			folder, ok = existingFolder(homeDir)
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
	// T721, on every hire route, before anything is made or the cooldown is spent.
	if err := testCopyRefusal(testCopyPort(), program, ""); err != nil {
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
	d.mu.Unlock()
	why := hooksNotConnected(Agent(program))
	d.mu.Lock()
	if why != "" {
		d.lastHire = time.Time{}
		return http.StatusConflict, why
	}

	// In tmux when chosen, or when there's no window to open.
	inTmux := tmux && (d.config.HireInTmux || !hasTerminal)
	// A role's Claude hire goes through the engine, at the desk chosen too (it
	// is seated there when its session appears); the rest open as they did.
	// A pinned desk stays on the old path: the engine's hire sits in a
	// worktree, whose session is a project of its own, so it wouldn't be seated
	// at its pinned desk.
	if roleID != "" && pinned == "" && Agent(program) == Claude {
		if inTmux {
			if status, message, handled := d.managedHire(roleID, folder, terminal, hasTerminal, held, floor); handled {
				return status, message
			}
		} else if launcherHires && d.config.HireManagedWindows && hasTerminal {
			// Beta: in a window run by the launcher, only where that is made and the switch is on.
			if status, message, handled := d.managedLauncherHire(roleID, folder, terminal, hasTerminal, held, floor); handled {
				return status, message
			}
		}
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
	// Opened outside the lock: the terminal may take a moment to start.
	d.mu.Unlock()
	err := openHire(command, folder, terminal, hasTerminal, inTmux)
	d.mu.Lock()
	if err != nil {
		// Nobody was hired, so the next try needn't wait.
		d.lastHire = time.Time{}
		name := terminal.name
		if inTmux {
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
	args := terminal.args(folder, command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = folder
	cmd.Env = hireEnvironment()
	prepareHireWindow(cmd, terminal.id)
	return startHire(cmd)
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
