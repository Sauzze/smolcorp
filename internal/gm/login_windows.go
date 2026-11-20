//go:build windows

package gm

import (
	"os"
	"os/exec"
)

// execGM runs the GM's command in this terminal and waits for it: Windows has no exec. The token stays in this
// process's memory until the GM ends, as no file holds it. (The app takes a GM's ask on Windows only with Smolcorp's
// own Settings window there, WebView2's, and "Allow a General Manager started by hand" on: FXWINTOOLASK.)
func execGM(program string, args, environment []string) error {
	cmd := exec.Command(program, args[1:]...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = environment, os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
