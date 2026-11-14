//go:build !darwin

package main

import (
	"fmt"
	"os"

	"github.com/Sauzze/smolcorp/desktop"
)

// On Linux and Windows this program is the app.
func runApp() int    { return desktop.RunApp() }
func runServer() int { return desktop.RunServer() }

// runCore is the Mac app's engine only: here, this program is the app.
func runCore([]string) int {
	fmt.Fprintln(os.Stderr, "smolcorp core is the Mac app's engine. Here, run smolcorp.")
	return 2
}
