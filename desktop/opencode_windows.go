package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// versionCommand asks program its version. An OpenCode npm installed is a
// .cmd script, which only cmd.exe can start, given a command line it reads
// as meant: see cmdLine.
func versionCommand(ctx context.Context, program string) *exec.Cmd {
	ext := strings.ToLower(filepath.Ext(program))
	if ext != ".cmd" && ext != ".bat" {
		return exec.CommandContext(ctx, program, "--version")
	}
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = `C:\Windows\System32\cmd.exe`
	}
	cmd := exec.CommandContext(ctx, shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine(shell, program, []string{"--version"})}
	return cmd
}
