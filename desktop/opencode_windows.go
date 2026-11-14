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
	return scriptCommand(ctx, program, []string{"--version"})
}

// scriptCommand runs program with args: a .cmd or .bat through cmd.exe, with a
// command line that reads each argument as one, never as a command (cmdLine).
// T3 Code's t3 is a .cmd too (t3code.go).
func scriptCommand(ctx context.Context, program string, args []string) *exec.Cmd {
	ext := strings.ToLower(filepath.Ext(program))
	if ext != ".cmd" && ext != ".bat" {
		return exec.CommandContext(ctx, program, args...)
	}
	shell := os.Getenv("ComSpec")
	if shell == "" {
		shell = `C:\Windows\System32\cmd.exe`
	}
	cmd := exec.CommandContext(ctx, shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine(shell, program, args)}
	return cmd
}
