package desktop

import (
	"os"
	"os/exec"
	"time"

	"fyne.io/systray"
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
	cmd := exec.Command(program)
	cmd.Env = append(os.Environ(), env...)
	detach(cmd)
	return cmd.Start()
}

// relaunch starts program to take this one's place, then leaves: the tray
// quits, and with it RunApp.
var relaunch = func(program string, env ...string) error {
	if err := startProgram(program, append(env, restartedVariable+"=1")...); err != nil {
		return err
	}
	// The answer to the page that asked goes first.
	time.AfterFunc(500*time.Millisecond, systray.Quit)
	return nil
}

// restartSoon has this copy start again.
var restartSoon = func() {
	if program, err := executable(); err == nil {
		_ = relaunch(program)
	}
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
	os.Unsetenv(restartedVariable)
	os.Unsetenv(installedFromVariable)
	return restarted
}
