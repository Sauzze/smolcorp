package desktop

import "testing"

// stubPlainHireTmux: no tmux on Windows, so no plain hire has a server to stand in for.
func stubPlainHireTmux(*testing.T, *hiring) {}

// launcherStandInPlaces: no sandbox and no tmux here, so nothing to point elsewhere.
func launcherStandInPlaces() int { return 0 }

// launcherSandboxStandIn: no plain hire starts in a sandbox here, so a test adds nothing.
func launcherSandboxStandIn(*testing.T) []string { return nil }
