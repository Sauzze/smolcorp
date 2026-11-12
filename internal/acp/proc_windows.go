//go:build windows

package acp

import (
	"os"
	"os/exec"
	"time"
)

// On Windows the agent itself is killed; what it started isn't reached
// (a job object would be needed, left for when ACP hires run there).
func ownGroup(cmd *exec.Cmd) {}

func killTree(p *os.Process) { p.Kill() }

func stopGroup(p *os.Process, grace time.Duration) {}
