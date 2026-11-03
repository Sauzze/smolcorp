//go:build darwin

package main

import (
	"fmt"
	"os"
)

// On the Mac the app is Smolcorp.app, and this program is the command inside
// it (gm, hook, guard): starting a second app from here would fight the first.
func runApp() int {
	fmt.Fprintln(os.Stderr, "On a Mac, Smolcorp is the app: open Smolcorp.app. This command is for `smolcorp gm`, `smolcorp hook` and the rest (smolcorp help).")
	return 2
}

func runServer() int { return runApp() }
