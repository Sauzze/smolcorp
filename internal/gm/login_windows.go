//go:build windows

package gm

import (
	"os"
	"os/exec"
)

// execGM runs the GM's command in this terminal and waits for it: Windows has no exec. (A GM started by hand is the
// Mac's only, so the app refuses the ask there before this is reached.)
func execGM(program string, args, environment []string) error {
	cmd := exec.Command(program, args[1:]...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = environment, os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
