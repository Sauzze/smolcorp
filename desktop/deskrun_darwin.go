package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// deskRun's sandbox on a Mac (SA-X6): a desk whose start wrote its profile (<support>/hires/<id>/sandbox.sb, SA-M6;
// the desk can't write it, by its Policy) runs what the core runs for it through the same runner as its start:
// smolcorp-spawn, then /usr/bin/sandbox-exec with that profile, then `smolcorp sandbox enter` with a fresh canary and
// its rows, then the program (sandbox.MacCommand). The run waits for `enter`'s full, passing report before anything
// of the program counts, so a profile that no longer holds refuses the run.

// deskRunSmolcorp is the program that runs `sandbox enter`: the app's own.
var deskRunSmolcorp = commandLineProgram

// deskSandboxHeld is a run's start in its desk's sandbox: nil outside one.
type deskSandboxHeld struct{ start *sandbox.Start }

// wait is the start's report, read once its command started: nil only for a full, passing one.
func (h deskSandboxHeld) wait(within time.Duration) error {
	if h.start == nil {
		return nil
	}
	return h.start.Wait(within)
}

func (h deskSandboxHeld) close() {
	if h.start != nil {
		h.start.Close()
		_ = h.start.Canary.Remove()
	}
}

// deskRunsSandboxed is whether deskRun runs a program for desk id in its sandbox: a desk with a profile, of any
// kind (one that isn't a plain file refuses the run, never runs it outside).
func deskRunsSandboxed(id string) bool {
	if !plainHireID(id) {
		return false
	}
	_, err := os.Lstat(filepath.Join(hireFolder(supportDir, id), "sandbox.sb"))
	return !errors.Is(err, fs.ErrNotExist)
}

// deskSandboxCommand is program with args and env in desk id's sandbox, and its start; no command and no error for a
// desk with no profile, which has none. A profile that isn't a plain file, or a sandbox that can't be started, is an
// error: the run is refused.
func deskSandboxCommand(id string, env []string, program string, args ...string) (*exec.Cmd, deskSandboxHeld, error) {
	if !plainHireID(id) {
		return nil, deskSandboxHeld{}, fmt.Errorf("sandbox: %q isn't a desk's id", id)
	}
	own := hireFolder(supportDir, id)
	profile := filepath.Join(own, "sandbox.sb")
	info, err := os.Lstat(profile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, deskSandboxHeld{}, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, deskSandboxHeld{}, fmt.Errorf("sandbox: desk %s's profile isn't a file (%v)", id, err)
	}
	checks, err := deskRunChecks(id, profile)
	if err != nil {
		return nil, deskSandboxHeld{}, err
	}
	start, err := sandbox.NewStart(checks)
	if err != nil {
		return nil, deskSandboxHeld{}, err
	}
	held := deskSandboxHeld{start}
	cmd, err := sandbox.MacCommand(start, profile, deskRunSmolcorp(), env, program, args...)
	if err != nil {
		held.close()
		return nil, deskSandboxHeld{}, err
	}
	return cmd, held, nil
}

// deskRunChecks are what `enter` tries for a run in desk id's sandbox: its start's own (deskChecks, SA-M6), from what
// the registry records of the desk (its worktree, its brief), with the stand-in other server in the socket folder its
// start made (deskSocketParent), which no desk can write (§A5.2 step 4). A desk with a profile but no row, or a socket
// folder that isn't this user's alone, refuses the run.
func deskRunChecks(id, profile string) (sandbox.Checks, error) {
	h, ok := registryHire(id)
	if !ok {
		return sandbox.Checks{}, fmt.Errorf("sandbox: desk %s has a profile but no row", id)
	}
	parent := deskSocketParent()
	if !ownSocketFolder(parent) {
		return sandbox.Checks{}, fmt.Errorf("sandbox: the socket folder %s isn't this user's alone", parent)
	}
	return deskChecks(deskAtOf(h, "", nil, engineOptions{}), physical(parent), profile), nil
}
