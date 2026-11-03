//go:build !darwin

package main

import "github.com/Sauzze/smolcorp/desktop"

// On Linux and Windows this program is the app.
func runApp() int    { return desktop.RunApp() }
func runServer() int { return desktop.RunServer() }
