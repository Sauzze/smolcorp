package desktop

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// attachNotInSandbox is what the terminal shows when an attach to a desk in its sandbox can't be put in that sandbox.
const attachNotInSandbox = "Smolcorp couldn't show this desk inside its sandbox on this Mac, so it didn't show it."

// attachSandboxEnvironment is the environment `enter` gets for an attach, the guard's variable aside (enter sets it):
// an attach client's own (attachEnvironment). Tests add their stand-in's switch.
var attachSandboxEnvironment = attachEnvironment

// attachInSandbox is an attach to desk h, in its sandbox on a Mac (SANDBOX-PLAN §A3.1, §A5.2): the bundled tmux at
// program, through `smolcorp-spawn /usr/bin/sandbox-exec -f <its profile> <smolcorp> sandbox enter --attach <server> …`
// (sandbox.MacCommand), with its own fresh canary and the rows of the desk's start (deskSandbox.checks). `enter`
// becomes the guarded client, told server, the pid the core checked at the desk's start. Once started, the attach goes
// on only after `enter`'s full, passing report; the canary goes then, whichever way. The profile is the one the desk's
// start wrote, which its server runs under: the attach makes none of its own.
func attachInSandbox(h Hire, program string, server int, args []string) (attaching, error) {
	refuse := func(why string) (attaching, error) {
		log.Printf("sandbox: an attach to desk %s: %s", h.ID, why)
		return attaching{}, errors.New(attachNotInSandbox + " (" + why + ")")
	}
	profile := filepath.Join(hireFolder(supportDir, h.ID), "sandbox.sb")
	desk := &deskSandbox{at: deskAtOf(h, deskSocketPath(h.ID, testCopyPort()), nil, engineOptions{}),
		places: sandbox.Places{SocketParent: deskSocketParent()}}
	start, err := sandbox.NewAttach(desk.checks(profile), server)
	if err != nil {
		return refuse("its check: " + err.Error())
	}
	var once sync.Once
	finish := func() { once.Do(func() { start.Close(); _ = start.Canary.Remove() }) }
	cmd, err := sandbox.MacCommand(start, profile, commandLineProgram(), attachSandboxEnvironment(), program, append([]string{"-f", os.DevNull}, args...)...)
	if err != nil {
		finish()
		return refuse(err.Error())
	}
	return attaching{cmd: cmd, done: finish, started: func() error {
		defer finish()
		if err := start.Wait(sandboxStartWait); err != nil {
			return errors.New(sandbox.Refusal("darwin") + " The desk wasn't shown. (" + err.Error() + ")")
		}
		return nil
	}}, nil
}
