// Command smolcorp is Smolcorp for Windows and Linux.
//
//	smolcorp                   the app: the deck's server, with an icon in the tray
//	smolcorp serve             the server alone, for a computer without a desktop
//	smolcorp run <agent>       start an agent so the deck can press its keys
//	smolcorp hook [agent]      report a hook event; the agents run this
//	smolcorp setup             set up and check everything, and say what's left
//	smolcorp version
package main

import (
	"fmt"
	"os"

	"github.com/Sauzze/smolcorp/desktop"
)

// version is set by scripts/release-desktop.sh.
var version = "dev"

const usage = `Usage: smolcorp [command]

  (none)                  the app, with its icon in the tray
  serve                   the server alone, for a computer without a desktop
  run <agent> [args]      start an agent so the office can press its keys
  run --role <role>       start a role's agent on that role
  setup [--check|--open]  set up and check everything, and say what's left
  hook [agent]            report a hook event: your agents run this
  version                 which Smolcorp this is`

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
	case "setup":
		os.Exit(desktop.RunSetup(args[1:]))
	case "version", "--version":
		fmt.Println("Smolcorp", version)
	case "help", "--help", "-h":
		fmt.Println(usage)
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}
