package desktop

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Smolcorp starting again: after an install, as the installed copy, or for a
// setting read at start, as the listening scope. The new one waits for this
// one to let go of the instance lock and the port (see waitForInstance).

// restartedVariable tells a Smolcorp it was started to take another's place.
const restartedVariable = "SMOLCORP_RESTARTED"

// installedFrom is the folder the copy that installed this one ran from,
// for the checklist to say it can go. Read once, at start.
var installedFrom string

// startProgram starts program, with env added to this one's environment.
func startProgram(program string, env ...string) error {
	return startProgramWith(program, nil, env...)
}

// startProgramWith starts program with args, and env added to this one's
// environment: a variable, for the tests.
//
// On a Mac it starts only Smolcorp itself (relaunch) or /usr/bin/open -b for
// its own app (openMacApp), so it is exempt, as a whole, from going through
// disclaim.Cmd (spawnsites_test.go). A new start added in here would pass that
// test unseen: start anything else elsewhere, through disclaim.Cmd.
var startProgramWith = func(program string, args []string, env ...string) error {
	cmd := exec.Command(program, args...)
	cmd.Env = append(os.Environ(), env...)
	detach(cmd)
	return cmd.Start()
}

// trayStarted is whether the tray's loop runs (newTray). On a Linux desktop
// with no host, it runs all the same, but nothing shows.
var trayStarted atomic.Bool

// hasTray is whether this copy has a tray to quit, that you can see: one
// started, with a host to show it in, asked now, so a panel that came after
// Smolcorp counts.
func hasTray() bool { return trayStarted.Load() && hasTrayHost() }

// relaunch has program take this one's place, as this one runs (the app, or
// smolcorp serve). With no tray, as serve or the app with no display, the
// program becomes the new one where the system allows it (Linux and the
// Mac): the same process, in the same terminal, which prints the new setup
// link there, stops with Ctrl+C, and stays a service's own under systemd.
// Otherwise it's started on its own, and this one leaves.
var relaunch = func(program string, env ...string) error {
	args, env := restartArgs(os.Args[1:]), append(env, restartedVariable+"=1")
	if !hasTray() && canReplaceProcess {
		// The answer to the page that asked goes first.
		time.AfterFunc(500*time.Millisecond, func() {
			err := replaceProcess(program, args, append(env, childrenNote()))
			// Still here: it couldn't, so a copy of its own takes this one's place.
			fmt.Fprintf(os.Stderr, "smolcorp: couldn't restart in place (%v), starting a new copy\n", err)
			if started := startProgramWith(program, args, env...); started != nil {
				couldntRestart(fmt.Errorf("%v; and as a new copy: %v", err, started))
				return
			}
			leave()
		})
		return nil
	}
	if err := startProgramWith(program, args, env...); err != nil {
		return err
	}
	time.AfterFunc(500*time.Millisecond, func() { leave() })
	return nil
}

// children are the programs this one started and waits for, by their pid:
// a hire's terminal, a browser opening a page. A restart in place hands
// them to the new program, which waits for them instead, so none that ends
// after it is left a zombie.
var children = struct {
	sync.Mutex
	pids map[int]bool
}{pids: map[int]bool{}}

// childrenVariable carries children's pids, a comma between, across a restart in place.
const childrenVariable = "SMOLCORP_CHILDREN"

// startAndWait starts cmd, and waits for it to end, without holding up the caller.
func startAndWait(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	children.Lock()
	children.pids[pid] = true
	children.Unlock()
	go func() {
		_ = cmd.Wait()
		children.Lock()
		delete(children.pids, pid)
		children.Unlock()
	}()
	return nil
}

// childrenNote is childrenVariable, set to the children still running.
func childrenNote() string {
	children.Lock()
	defer children.Unlock()
	pids := make([]string, 0, len(children.pids))
	for pid := range children.pids {
		pids = append(pids, strconv.Itoa(pid))
	}
	slices.Sort(pids)
	return childrenVariable + "=" + strings.Join(pids, ",")
}

// adoptChildren waits for the children the program this one replaced in place
// left running, as their parent now, each in its own time. They're its
// children as any other until they end, handed on again by a second restart.
func adoptChildren(note string) {
	for _, word := range strings.Split(note, ",") {
		// A pid is 32 bits: a larger number would reach the system as another,
		// -1, any child, say, and take what Go's own waits are for.
		if pid, err := strconv.Atoi(word); err == nil && pid > 0 && pid <= math.MaxInt32 {
			children.Lock()
			children.pids[pid] = true
			children.Unlock()
			go func() {
				waitForChild(pid)
				children.Lock()
				delete(children.pids, pid)
				children.Unlock()
			}()
		}
	}
}

// restartArgs are the arguments a copy taking this one's place starts with:
// serve's, for the server alone; none, for the app.
func restartArgs(args []string) []string {
	if len(args) > 0 && args[0] == "serve" {
		return []string{"serve"}
	}
	return nil
}

// leave ends this copy, for the one taking its place: the program exits,
// as serve, or the app on a computer with no desktop, have nothing else to
// end. With a tray, the tray quits instead, and with it RunApp (see newTray).
var leave = func() { os.Exit(0) }

// restartSoon has this copy start again.
var restartSoon = func() {
	program, err := executable()
	if err == nil {
		err = relaunch(program)
	}
	if err != nil {
		couldntRestart(err)
	}
}

// restartProblem is why this copy couldn't start again for a change of
// yours, if it couldn't: setup's checklist says so (see restartCheck).
var restartProblem atomic.Pointer[string]

func couldntRestart(err error) {
	fmt.Fprintf(os.Stderr, "smolcorp: couldn't start again: %v\n", err)
	said := err.Error()
	restartProblem.Store(&said)
}

// restartCheck is the checklist's line for a restart that didn't happen, or
// nil: the change waits for Smolcorp's next start.
func restartCheck() *check {
	problem := restartProblem.Load()
	if problem == nil {
		return nil
	}
	return &check{ID: "restart", Status: checkFix, Title: "Smolcorp started again for your change",
		Detail: "It couldn't (" + *problem + "), so the change waits for its next start. Quit Smolcorp and open it again, " +
			"or stop smolcorp serve and start it again."}
}

// waitForInstance takes the instance lock, waiting up to 15 s for the
// Smolcorp this one replaces to let go of it. Running is as acquireInstance
// says once that's over.
func waitForInstance() (running bool, err error) {
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(250 * time.Millisecond) {
		running, err = acquireInstance()
		if !running || time.Now().After(deadline) {
			return running, err
		}
	}
}

// takeStartNotes reads what the Smolcorp that started this one said, and
// leaves none of it for the agents this one starts.
func takeStartNotes() (restarted bool) {
	restarted = os.Getenv(restartedVariable) == "1"
	installedFrom = os.Getenv(installedFromVariable)
	adoptChildren(os.Getenv(childrenVariable))
	os.Unsetenv(restartedVariable)
	os.Unsetenv(installedFromVariable)
	os.Unsetenv(childrenVariable)
	return restarted
}
