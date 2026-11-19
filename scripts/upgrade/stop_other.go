//go:build !windows

package main

import "os/exec"

// inItsOwnGroup: off Windows, serve starts as any program does; `smolcorp quit` finds it by its pid record.
func inItsOwnGroup(*exec.Cmd) {}

// askToQuit is `smolcorp quit`, as an owner stops a Smolcorp with no tray: it asks the running copy with SIGTERM,
// which serve takes as a clean stop (desktop/app.go, stopPortableOnSignalNow). How it asked, and what it printed.
func (o *office) askToQuit() (string, []byte, error) {
	quit := exec.Command(o.program, "quit")
	quit.Env, quit.Dir = o.env, o.home
	output, err := quit.CombinedOutput()
	return "smolcorp quit", output, err
}
