//go:build !windows

package desktop

import "os/exec"

// hideGitWindow is for Windows only.
func hideGitWindow(*exec.Cmd) {}
