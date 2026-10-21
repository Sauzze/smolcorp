package desktop

import (
	"net/http"
	"os"
	"os/exec"
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
	// The desk it was hired at, with a set number of desks, or -1.
	desk int
	at   time.Time
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
var startHire = func(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

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
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.config.HiringEnabled {
		return http.StatusForbidden, hiringOff
	}
	pinned := ""
	// With a set number of desks, any free one can be hired at, and the hire
	// sits there; with Auto they stay packed, so it takes the first free.
	held := -1
	if desk != "" {
		keys, _ := d.layout()
		// With Auto, the office also offers the desk after the last: a hire adds it.
		next := -1
		auto := d.config.Layout.Keys == Auto
		if auto && len(keys) < mostKeys {
			next = len(keys)
		}
		index, err := strconv.Atoi(desk)
		if err != nil || index < 0 || index >= len(keys) && index != next {
			return http.StatusBadRequest, "Unknown desk"
		}
		if index < len(keys) && keys[index].pin != nil {
			pinned, _ = existingFolder(*keys[index].pin)
		}
		if !auto && keys[index].pin == nil {
			if keys[index].session != nil {
				return http.StatusConflict, "Someone's at that desk already."
			}
			held = index
		}
	}
	folder := pinned
	if folder == "" {
		var ok bool
		if folder, ok = existingFolder(d.config.HireFolder); !ok {
			return http.StatusConflict, "Choose a folder for new hires in setup, on your computer."
		}
	}
	if roleID != "" && !slices.ContainsFunc(d.config.Roles, func(r Role) bool { return r.ID == roleID }) {
		return http.StatusNotFound, "That role isn't there any more. Pick another."
	}
	if time.Since(d.lastHire) < hireCooldown {
		return http.StatusTooManyRequests, "Hired a moment ago. Try again in a few seconds."
	}
	terminals, tmux := hireWays()
	terminal, hasTerminal := hireTerminal(d.config.HireTerminal, terminals)
	if !hasTerminal && !tmux {
		return http.StatusConflict, "There's no terminal here to open a hire in."
	}
	d.lastHire = time.Now()

	token := newID()
	command := []string{commandLineProgram(), "run", "--hire", token}
	if pinned != "" {
		command = append(command, "--pinned")
	}
	if roleID != "" {
		command = append(command, "--role", roleID)
	} else {
		command = append(command, d.config.HireAgent)
	}
	// In tmux when chosen, or when there's no window to open.
	inTmux := tmux && (d.config.HireInTmux || !hasTerminal)
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
	d.hires[token] = &pendingHire{desk: held, at: time.Now()}
	return http.StatusNoContent, ""
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
// started from, nor a headless run.
func hireEnvironment() []string {
	return slices.DeleteFunc(os.Environ(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return slices.Contains([]string{"SMOLCORP_SESSION", "SAUZZEDECK_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER",
			"TMUX", "TMUX_PANE", "WEZTERM_PANE", "WEZTERM_UNIX_SOCKET", "KITTY_WINDOW_ID", "KITTY_LISTEN_ON"}, strings.ToUpper(name))
	})
}

// hiredDesk takes the desk a hire was made at, for the launcher that hands
// its token back, or -1: each token once, while it's fresh. With the lock held.
func (d *Deck) hiredDesk(token string) int {
	pending, ok := d.hires[token]
	if !ok || !hireToken.MatchString(token) {
		return -1
	}
	delete(d.hires, token)
	if time.Since(pending.at) > hiredTabWait {
		return -1
	}
	return pending.desk
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
		"canHire": len(terminals) > 0 || tmux, "canChoose": false,
		"terminal": chosen.id, "terminals": choices,
		"tmux": d.config.HireInTmux && tmux, "hasTmux": tmux,
	}
}
