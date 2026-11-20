//go:build !darwin

package desktop

import (
	"errors"

	"github.com/aymanbagabas/go-pty"
)

// No plain hire's agent starts in a sandbox here until each system's sandbox lands (SA-L4 on Linux; Windows has none
// in 1.26): launcherSandboxes is false, so the launcher never makes one, and these are never reached.

const launcherSandboxes = false

type launcherSandbox struct{}

func newLauncherSandbox(string, bool, string, Agent, string) (*launcherSandbox, error) {
	return nil, errors.New("no sandbox on this computer yet")
}

func newLauncherSandboxAt(string, string, string, bool, string, Agent, string) (*launcherSandbox, error) {
	return nil, errors.New("no sandbox on this computer yet")
}

func (s *launcherSandbox) environment(env []string, _ map[string]string) []string { return env }

func (s *launcherSandbox) command(*pty.Cmd, pty.Pty) (agentStart, error) {
	return nil, errors.New("no sandbox on this computer yet")
}

func (s *launcherSandbox) close() {}

func (s *launcherSandbox) ownHookKey() string { return "" }

func (s *launcherSandbox) noteConnected() {}

func (s *launcherSandbox) closeOnHangup() {}
