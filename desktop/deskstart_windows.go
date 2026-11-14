package desktop

import (
	"fmt"
	"os"
)

// RunDeskStart: Windows has no tmux desks. Its launcher (hirelauncher.go) passes the environment straight to the
// agent it starts (SANDBOX-PLAN §3.4).
func RunDeskStart([]string) int {
	fmt.Fprintln(os.Stderr, "smolcorp: desk-start starts a tmux desk, and Windows starts desks through its launcher.")
	return 2
}
