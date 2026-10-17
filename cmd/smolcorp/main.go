// Command smolcorp is Smolcorp for Windows and Linux.
//
//	smolcorp                   the app: the deck's server, with an icon in the tray
//	smolcorp serve             the server alone, for a computer without a desktop
//	smolcorp run <agent>       start an agent so the deck can press its keys
//	smolcorp hook [agent]      report a hook event; the agents run this
//	smolcorp version
package main

import (
	"fmt"
	"os"

	"github.com/Sauzze/agentdeck/desktop"
)

// version is set by scripts/release-desktop.sh.
var version = "dev"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(desktop.RunApp())
	}
	switch args[0] {
	case "serve":
		os.Exit(desktop.RunServer())
	case "run":
		os.Exit(desktop.RunLauncher(args[1:]))
	case "hook":
		os.Exit(desktop.RunHook(args[1:]))
	case "version", "--version":
		fmt.Println("Smolcorp", version)
	default:
		fmt.Fprintln(os.Stderr, "Usage: smolcorp [serve | run <agent> [arguments] | hook [agent] | version]")
		os.Exit(2)
	}
}
