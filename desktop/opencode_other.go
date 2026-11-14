//go:build !windows

package desktop

import (
	"context"
	"os/exec"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// versionCommand asks program its version.
func versionCommand(ctx context.Context, program string) *exec.Cmd {
	return disclaim.Cmd(exec.CommandContext(ctx, program, "--version"))
}

// scriptCommand runs program with args as they are, no shell (t3code.go).
func scriptCommand(ctx context.Context, program string, args []string) *exec.Cmd {
	return disclaim.Cmd(exec.CommandContext(ctx, program, args...))
}
