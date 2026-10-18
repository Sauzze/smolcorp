//go:build !windows

package desktop

import (
	"context"
	"os/exec"
)

// versionCommand asks program its version.
func versionCommand(ctx context.Context, program string) *exec.Cmd {
	return exec.CommandContext(ctx, program, "--version")
}
