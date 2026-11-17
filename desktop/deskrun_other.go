//go:build !darwin

package desktop

import (
	"os/exec"
	"time"
)

// deskRun's sandbox off a Mac: none yet. Linux's runner is SA-L2's, and Windows' SA-W3's; until they land, what the
// core runs for a desk gets the hardened git alone (deskrun.go).

type deskSandboxHeld struct{}

func (deskSandboxHeld) wait(time.Duration) error { return nil }
func (deskSandboxHeld) close()                   {}

// deskRunsSandboxed: deskRun runs nothing in a desk's sandbox here yet.
func deskRunsSandboxed(string) bool { return false }

func deskSandboxCommand(string, []string, string, ...string) (*exec.Cmd, deskSandboxHeld, error) {
	return nil, deskSandboxHeld{}, nil
}
