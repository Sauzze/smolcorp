package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

// A Tab is where a session runs, and how its keys are pressed: the Mac app
// drives Terminal.app; here each kind of tab is driven its own way. The hook
// names a session's tab with a string:
//
//	launcher:<id>              a terminal `smolcorp run` started the agent in
//	tmux:<pane>@<socket>       a tmux pane: $TMUX_PANE and $TMUX
//	wezterm:<pane>@<socket>    a WezTerm pane: $WEZTERM_PANE and $WEZTERM_UNIX_SOCKET
//	kitty:<window>@<socket>    a kitty window: $KITTY_WINDOW_ID and $KITTY_LISTEN_ON
//	headless:<session id>      a headless claude run, started with SMOLCORP_HEADLESS=1
//
// A headless run has no tab at all: it is named so that it can't be mistaken
// for one, and nothing is ever pressed, typed or watched there.
//
// The tab commands run the terminal's own program with arguments, never
// through a shell, so nothing from an event is ever read as a command.
type Tab interface {
	Press(Keystroke) error
	// Type types text and presses Enter.
	Type(text string) error
	// Focus brings the tab forward, as far as its terminal allows.
	Focus() error
	// Scroll moves the view of the scrollback by lines, up when negative.
	Scroll(lines int) error
	ScrollToBottom() error
	// Close stops the agent in the tab, and closes the tab where the
	// terminal lets it: the office's firing. Anything else in front of the
	// tab is left alone.
	Close(agent Agent) error
}

var errUnsupported = errors.New("not possible in this terminal")

// keyGap separates repeated keys. Two Escapes sent back to back read as one
// Alt+Escape; OpenCode's double Esc needs a beat between them.
const keyGap = 120 * time.Millisecond

// isHeadless is whether a tab's name is a headless run's, in the one shape
// the hook sends.
func isHeadless(name string) bool {
	kind, id, _ := strings.Cut(name, ":")
	return kind == "headless" && headlessID.MatchString(id)
}

// splitTab is a tab's name in parts.
func splitTab(name string) (kind, id, socket string) {
	kind, rest, _ := strings.Cut(name, ":")
	id, socket, _ = strings.Cut(rest, "@")
	return kind, id, socket
}

// commandTab drives a terminal through its command-line interface.
type commandTab struct {
	kind, id, socket string
}

// terminalTimeout is as long as a terminal's command may take. Keys go one at
// a time, so one that hung, as kitty asking on its own screen whether to
// allow remote control, would hold up every phone's keys behind it.
var terminalTimeout = 5 * time.Second

// terminalCommand is a terminal's command line tool, stopped if it hangs.
func terminalCommand(name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), terminalTimeout)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second // and not held open by anything it started
	return cmd, cancel
}

func (t commandTab) run(args ...string) error {
	_, err := t.output(args...)
	return err
}

// output runs the terminal's command, and gives what it printed.
func (t commandTab) output(args ...string) ([]byte, error) {
	var cmd *exec.Cmd
	var cancel context.CancelFunc
	switch t.kind {
	case "tmux":
		cmd, cancel = terminalCommand("tmux", append([]string{"-S", t.socket}, args...)...)
	case "wezterm":
		cmd, cancel = terminalCommand("wezterm", append([]string{"cli"}, args...)...)
		if t.socket != "" {
			cmd.Env = append(cmd.Environ(), "WEZTERM_UNIX_SOCKET="+t.socket)
		}
	case "kitty":
		cmd, cancel = terminalCommand("kitty", append([]string{"@", "--to", t.socket}, args...)...)
	default:
		return nil, errUnsupported
	}
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		var failed *exec.ExitError
		if errors.As(err, &failed) {
			return nil, fmt.Errorf("%s: %s", t.kind, strings.TrimSpace(string(failed.Stderr)))
		}
		return nil, fmt.Errorf("%s: %v", t.kind, err)
	}
	return output, nil
}

// runs says whether the agent itself is in front in the pane, as the Mac
// app asks of a tab, and whether that could be known at all: an agent that
// crashed, or was killed, leaves its pane to the shell, or to whatever was
// started there next, and one stopped with Ctrl-Z or sent to the
// background leaves the shell in front; keys and text meant for it must
// reach none of them. Unknown is not the same as gone.
func (t commandTab) runs(agent Agent) (running, known bool) {
	front := t.front()
	switch front.is {
	case inFrontJob:
		return slices.ContainsFunc(front.job, agent.isItself), true
	case inFrontShell:
		// The pane's own process in front: the agent, if the pane was started with it.
		return agent.isItself(front.own), true
	}
	return false, false
}

// kittyWindow is what kitty says of the window: its own process, and those
// in front of it. Nil if kitty answered without it.
func (t commandTab) kittyWindow() (*kittyWindow, error) {
	output, err := t.output("ls", "--match", "id:"+t.id)
	if err != nil {
		return nil, err
	}
	var windows []struct {
		Tabs []struct {
			Windows []kittyWindow `json:"windows"`
		} `json:"tabs"`
	}
	if err := json.Unmarshal(output, &windows); err != nil {
		return nil, err
	}
	for _, w := range windows {
		for _, tab := range w.Tabs {
			for _, window := range tab.Windows {
				if fmt.Sprint(window.ID) == t.id {
					return &window, nil
				}
			}
		}
	}
	return nil, nil
}

// kittyWindow is a window as kitty's `ls` lists it.
type kittyWindow struct {
	ID      int      `json:"id"`
	PID     int      `json:"pid"`
	Cmdline []string `json:"cmdline"`
}

func (t commandTab) tty() string {
	switch t.kind {
	case "tmux":
		output, err := t.output("display-message", "-p", "-t", t.id, "#{pane_tty}")
		if err == nil {
			return strings.TrimSpace(string(output))
		}
	case "wezterm":
		output, err := t.output("list", "--format", "json")
		var panes []struct {
			ID  int    `json:"pane_id"`
			TTY string `json:"tty_name"`
		}
		if err == nil && json.Unmarshal(output, &panes) == nil {
			for _, pane := range panes {
				if fmt.Sprint(pane.ID) == t.id {
					return pane.TTY
				}
			}
		}
	}
	return ""
}

// tmux names keys; WezTerm and kitty take the bytes the key sends.
var (
	tmuxKeys = map[Key]string{Enter: "Enter", Escape: "Escape", Space: "Space", Up: "Up", Down: "Down", CtrlC: "C-c", TabKey: "Tab", ShiftTab: "BTab"}
	keyBytes = map[Key]string{Enter: "\r", Escape: "\x1b", Space: " ", Up: "\x1b[A", Down: "\x1b[B", CtrlC: "\x03", TabKey: "\t", ShiftTab: "\x1b[Z"}
)

func (t commandTab) Press(k Keystroke) error {
	for i := 0; i < max(k.Times, 1); i++ {
		if i > 0 {
			time.Sleep(keyGap)
		}
		if err := t.sendKey(k.Key); err != nil {
			return err
		}
	}
	return nil
}

func (t commandTab) sendKey(key Key) error {
	switch t.kind {
	case "tmux":
		return t.run("send-keys", "-t", t.id, tmuxKeys[key])
	case "wezterm":
		return t.run("send-text", "--pane-id", t.id, "--no-paste", keyBytes[key])
	case "kitty":
		return t.run("send-text", "--match", "id:"+t.id, kittyEscaped(keyBytes[key]))
	}
	return errUnsupported
}

func (t commandTab) Type(text string) error {
	if args := t.typeArgs(text); args != nil {
		if err := t.run(args...); err != nil {
			return err
		}
	}
	return t.sendKey(Enter)
}

// typeArgs are the terminal's arguments for typing text as it is. "--" ends
// the options, so text starting with "-" is typed rather than read as one;
// and tmux takes an argument ending in ";" for the end of its command unless
// the ";" is escaped.
func (t commandTab) typeArgs(text string) []string {
	switch t.kind {
	case "tmux":
		if strings.HasSuffix(text, ";") {
			text = strings.TrimSuffix(text, ";") + `\;`
		}
		return []string{"send-keys", "-t", t.id, "-l", "--", text}
	case "wezterm":
		return []string{"send-text", "--pane-id", t.id, "--no-paste", "--", text}
	case "kitty":
		return []string{"send-text", "--match", "id:" + t.id, "--", kittyEscaped(text)}
	}
	return nil
}

func (t commandTab) Focus() error {
	switch t.kind {
	case "tmux":
		if err := t.run("select-window", "-t", t.id); err != nil {
			return err
		}
		return t.run("select-pane", "-t", t.id)
	case "wezterm":
		return t.run("activate-pane", "--pane-id", t.id)
	case "kitty":
		return t.run("focus-window", "--match", "id:"+t.id)
	}
	return errUnsupported
}

// Scroll uses tmux's copy mode, which -e leaves by itself once scrolled back
// to the bottom, and kitty's own scrolling. WezTerm's CLI has none.
func (t commandTab) Scroll(lines int) error {
	switch t.kind {
	case "tmux":
		if err := t.run("copy-mode", "-e", "-t", t.id); err != nil {
			return err
		}
		command := "scroll-down"
		if lines < 0 {
			command = "scroll-up"
		}
		return t.run("send-keys", "-t", t.id, "-X", "-N", fmt.Sprint(abs(lines)), command)
	case "kitty":
		amount := fmt.Sprintf("%dl", abs(lines))
		if lines < 0 {
			amount += "-"
		}
		return t.run("scroll-window", "--match", "id:"+t.id, amount)
	}
	return errUnsupported
}

func (t commandTab) ScrollToBottom() error {
	switch t.kind {
	case "tmux":
		// Outside copy mode there is nothing to leave, and that's fine.
		_ = t.run("send-keys", "-t", t.id, "-X", "cancel")
		return nil
	case "kitty":
		return t.run("scroll-window", "--match", "id:"+t.id, "end")
	}
	return errUnsupported
}

// Close fires the agent, as the Mac app does: the agent in front of the
// pane is stopped, then the pane closed, and a pane left to its shell is
// closed. A pane with anything else in front, as a dev server or an editor
// started there after the agent crashed, is left as it is, and so is one
// that can't be looked at: errNotStopped. If, once the agent has stopped,
// another job is in front, the pane is that job's now, and stays.
func (t commandTab) Close(agent Agent) error {
	if t.kind != "tmux" && t.kind != "wezterm" && t.kind != "kitty" {
		return errUnsupported
	}
	if !t.alive() {
		return errors.New("the pane has closed")
	}
	front := t.front()
	switch front.is {
	case inFrontJob:
		if !slices.ContainsFunc(front.job, agent.isItself) || !stopGroup(front.group) {
			return errNotStopped
		}
		// Stopped. The pane closes only if its shell is all that's left in it.
		if after := t.front(); after.is != inFrontShell || after.behind {
			return nil
		}
	case inFrontShell:
		// The pane's own process: its shell, or the agent when the pane was
		// started with it. Anything else is someone's work, and so is anything
		// the shell runs in the background, which closing would end too.
		program := strings.Trim(strings.ToLower(baseName(strings.Fields(front.own + " ")[0])), "-")
		if !slices.Contains(shells, program) && !agent.isItself(front.own) || front.behind {
			return errNotStopped
		}
	default:
		return errNotStopped
	}
	switch t.kind {
	case "tmux":
		return t.run("kill-pane", "-t", t.id)
	case "wezterm":
		return t.run("kill-pane", "--pane-id", t.id)
	default:
		return t.run("close-window", "--match", "id:"+t.id)
	}
}

// errNotStopped is a pane firing leaves open: see Close.
var errNotStopped = errors.New("couldn't stop the agent")

// shells are what a pane's own process is when it's only a shell.
var shells = []string{"zsh", "bash", "sh", "fish", "dash", "ksh", "tcsh", "csh", "nu", "login"}

const (
	inFrontUnknown = iota
	inFrontShell
	inFrontJob
)

// inFront is what's in front of a pane: its own process, or a job of others
// in front of it, and their command lines; or that it couldn't be told.
type inFront struct {
	is     int
	own    string   // the pane's own process's command line
	group  int      // the job's process group
	job    []string // its processes' command lines
	behind bool     // whether anything else runs on the tty, as jobs in the background
}

// front says what's in front of the pane, from the processes on its tty.
// tmux and kitty say which process is the pane's own; WezTerm doesn't, and
// it's the one on the pane's tty that none of the others started.
func (t commandTab) front() inFront {
	switch t.kind {
	case "tmux":
		output, err := t.output("display-message", "-p", "-t", t.id, "#{pane_tty} #{pane_pid}")
		tty, pid, _ := strings.Cut(strings.TrimSpace(string(output)), " ")
		own, bad := strconv.Atoi(pid)
		if err != nil || bad != nil || tty == "" {
			return inFront{}
		}
		return frontOn(tty, own)
	case "wezterm":
		if tty := t.tty(); tty != "" {
			return frontOn(tty, 0)
		}
	case "kitty":
		// Its window's own process, and the tty it's on.
		window, err := t.kittyWindow()
		if err != nil || window == nil || window.PID == 0 {
			return inFront{}
		}
		cmd, cancel := terminalCommand("ps", "-p", strconv.Itoa(window.PID), "-o", "tty=")
		defer cancel()
		output, err := cmd.Output()
		tty := strings.TrimSpace(string(output))
		if err != nil || tty == "" || strings.HasPrefix(tty, "?") {
			return inFront{}
		}
		return frontOn("/dev/"+tty, window.PID)
	}
	return inFront{}
}

// frontOn says what's in front on tty, whose pane's own process is own, or
// for 0, the one there that none of the others started.
func frontOn(tty string, own int) inFront {
	cmd, cancel := terminalCommand("ps", "-t", strings.TrimPrefix(tty, "/dev/"), "-o", "pid=,ppid=,pgid=,tpgid=,args=")
	defer cancel()
	output, err := cmd.Output()
	var none *exec.ExitError
	if err != nil && !(errors.As(err, &none) && none.Exited() && len(output) == 0) {
		return inFront{} // ps exits 1 when nothing runs on the tty: that's an answer
	}
	return frontOf(string(output), own)
}

// frontOf reads ps's lines, as frontOn asks for them.
func frontOf(lines string, own int) inFront {
	type row struct {
		pid, ppid, pgid, tpgid int
		args                   string
	}
	var rows []row
	pids := map[int]bool{}
	for _, line := range strings.Split(lines, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		var r row
		numbers := []*int{&r.pid, &r.ppid, &r.pgid, &r.tpgid}
		ok := true
		for i, n := range numbers {
			var err error
			if *n, err = strconv.Atoi(fields[i]); err != nil {
				ok = false
			}
		}
		if !ok {
			continue
		}
		r.args = strings.Join(fields[4:], " ")
		rows = append(rows, r)
		pids[r.pid] = true
	}
	var shell *row
	for i := range rows {
		mine := rows[i].pid == own
		if own == 0 {
			mine = !pids[rows[i].ppid]
		}
		if mine {
			if shell != nil {
				return inFront{} // two, and no telling which is the pane's
			}
			shell = &rows[i]
		}
	}
	if len(rows) == 0 {
		return inFront{is: inFrontShell} // nothing at all: the shell has ended too
	}
	if shell == nil {
		return inFront{}
	}
	front := inFront{is: inFrontShell, own: shell.args}
	// Anything not the shell's, nor the job's in front, nor login's: in the background.
	for _, r := range rows {
		if r.pgid != shell.pgid && r.pgid != shell.tpgid && baseName(strings.Fields(r.args + " ")[0]) != "login" {
			front.behind = true
		}
	}
	if shell.tpgid <= 0 || shell.tpgid == shell.pgid {
		return front
	}
	front.is, front.group = inFrontJob, shell.tpgid
	for _, r := range rows {
		if r.pgid == shell.tpgid {
			front.job = append(front.job, r.args)
		}
	}
	return front
}

// alive asks the terminal whether the pane is still there. Unknown counts as
// alive: a session is only dropped when its tab is known to be gone.
func (t commandTab) alive() bool {
	switch t.kind {
	case "tmux":
		cmd, cancel := terminalCommand("tmux", "-S", t.socket, "display-message", "-p", "-t", t.id, "#{pane_id}")
		defer cancel()
		output, err := cmd.Output()
		var answered *exec.ExitError
		if errors.As(err, &answered) && answered.Exited() {
			return false // tmux is there, and says the pane isn't; stopped for taking too long, it said nothing
		}
		return err != nil || strings.TrimSpace(string(output)) == t.id
	}
	return true
}

// kittyEscaped writes control characters the way kitty's send-text reads
// them: Python escapes, so a backslash itself must be doubled.
func kittyEscaped(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r < 0x20:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
