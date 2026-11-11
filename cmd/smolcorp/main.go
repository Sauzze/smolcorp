// Command smolcorp is Smolcorp for Windows and Linux.
//
//	smolcorp                          the app: the deck's server, with an icon in the tray
//	smolcorp serve                    the server alone, for a computer without a desktop
//	smolcorp run <agent>              start an agent so the deck can press its keys
//	smolcorp hook [--port N] [agent]  report a hook event; the agents run this
//	smolcorp setup                    set up and check everything, and say what's left
//	smolcorp gm <command>             ask the running app about the team (the GM's hands)
//	smolcorp guard [--profile gm] [--rules FILE]     the hire guard: Claude Code runs it before each Bash command
//	smolcorp quit                     stop the running copy, by the pid it recorded
//	smolcorp version
package main

import (
	"fmt"
	"os"

	"github.com/Sauzze/smolcorp/desktop"
	"github.com/Sauzze/smolcorp/internal/gm"
	"github.com/Sauzze/smolcorp/internal/guard"
)

// version is set by scripts/release-desktop.sh.
var version = "dev"

const usage = `Usage: smolcorp [command]

  (none)                  the app, with its icon in the tray
  serve                   the server alone, for a computer without a desktop
  run <agent> [args]      start an agent so the office can press its keys
  run --role <role>       start a role's agent on that role
  setup [--check|--open]  set up and check everything, and say what's left
  hook [--port N] [agent] report a hook event: your agents run this
  gm <command>            ask the running Smolcorp about the team (gm help)
  guard [--profile gm] [--rules FILE]    the hire guard, a PreToolUse hook: reads the call on stdin
  mcp                     serve the gm commands as MCP tools on standard input and output
  quit                    stop the running Smolcorp (the one of this user)
  version                 which Smolcorp this is`

func main() {
	desktop.Version = version
	gm.AppVersion = version
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runApp())
	}
	switch args[0] {
	case "serve":
		os.Exit(runServer())
	case "run":
		os.Exit(desktop.RunLauncher(args[1:]))
	case "hook":
		os.Exit(desktop.RunHook(args[1:]))
	case "setup":
		os.Exit(desktop.RunSetup(args[1:]))
	case "gm":
		os.Exit(gm.Run(args[1:], os.Stdout, os.Stderr))
	case "guard":
		os.Exit(guard.Run(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "mcp":
		os.Exit(gm.RunMCP(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "quit":
		os.Exit(desktop.RunQuit())
	case "core":
		// Not in the usage: Smolcorp.app starts it, as its engine (start_darwin.go).
		os.Exit(runCore(args[1:]))
	case "version", "--version":
		fmt.Println("Smolcorp", version)
	case "help", "--help", "-h":
		fmt.Println(usage)
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}
