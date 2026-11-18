// Command smolcorp is Smolcorp for Windows and Linux.
//
//	smolcorp                          the app: the deck's server, with an icon in the tray
//	smolcorp serve                    the server alone, for a computer without a desktop
//	smolcorp run <agent>              start an agent so the deck can press its keys
//	smolcorp hook [--port N] [agent]  report a hook event; the agents run this
//	smolcorp setup                    set up and check everything, and say what's left
//	smolcorp demo [--dry-run]         a General Manager and its team on a sample project
//	smolcorp gm <command>             ask the running app about the team (the GM's hands)
//	smolcorp status | desks           the team, for you at the computer (see: smolcorp status --help)
//	smolcorp attach <desk>            show a desk in this terminal, through its sandbox where it has one
//	smolcorp guard [--profile gm] [--rules FILE]     the hire guard: Claude Code runs it before each Bash command
//	smolcorp quit                     stop the running copy, by the pid it recorded
//	smolcorp hold on [MINUTES] | off  on a Mac, hold what the office types into desks while tests run there
//	smolcorp version
package main

import (
	"fmt"
	"os"

	"github.com/Sauzze/smolcorp/desktop"
	"github.com/Sauzze/smolcorp/internal/gm"
	"github.com/Sauzze/smolcorp/internal/guard"
	"github.com/Sauzze/smolcorp/internal/procmem"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// version is set by scripts/release-desktop.sh.
var version = "dev"

const usage = `Usage: smolcorp [command]

  (none)                  the app, with its icon in the tray
  serve                   the server alone, for a computer without a desktop
  run <agent> [args]      start an agent so the office can press its keys
  run --role <role>       start a role's agent on that role
  setup [--check|--open]  set up and check everything, and say what's left
  demo [--dry-run] [--in FOLDER]
                          watch a General Manager run a team on a sample
                          project, in a new folder (asks before it spends)
  hook [--port N] [agent] report a hook event: your agents run this
  gm <command>            ask the running Smolcorp about the team (gm help)
  status                  the office at a glance: the team, who waits on you
  desks                   the team, one line a desk
  attach <desk>           show a desk's session in this terminal (its window's
                          own line: tmux attach doesn't reach a desk)
  say, hire, open         come with app keys in Settings (not yet: use the office)
  guard [--profile gm] [--rules FILE]    the hire guard, a PreToolUse hook: reads the call on stdin
  mcp                     serve the gm commands as MCP tools on standard input and output
  quit                    stop the running Smolcorp (the one of this user)
  hold on [MINUTES] | off on a Mac, hold what the office types into desks
                          while tests run there (15 minutes at most)
  version                 which Smolcorp this is`

// `smolcorp gm login` checks the program it starts as the GM as Smolcorp checks every agent it starts (SG2,
// RMSY126k nit 6): internal/gm can't import desktop, so the check is put there here.
func init() { gm.CheckProgram = desktop.CheckAgentStart }

func main() {
	desktop.Version = version
	gm.AppVersion = version
	args := os.Args[1:]
	if len(args) == 0 {
		// The app and the server keep their memory from your other programs as their own first step, and don't
		// start if they can't (desktop/memory.go).
		os.Exit(runApp())
	}
	command := args[0]
	if len(args) > 1 && command == "sandbox" && args[1] == "connect" {
		command, args = "sandbox connect", args[1:]
	}
	switch command {
	case "serve":
		os.Exit(runServer())
	case "run":
		os.Exit(desktop.RunLauncher(args[1:]))
	case "hook":
		os.Exit(desktop.RunHook(args[1:]))
	case "desk-start":
		// Not in the usage: Smolcorp starts a desk's tmux pane on it, with that desk's ticket (deskstart.go).
		os.Exit(desktop.RunDeskStart(args[1:]))
	case "attach":
		// Every window, and every pane of the team grid, Smolcorp opens on a desk runs this with exec (desktop/attach.go).
		os.Exit(desktop.RunAttach(args[1:]))
	case "sandbox connect":
		// Not in the usage: a Linux desk's git runs it as ssh's ProxyCommand (GIT_SSH_COMMAND), inside the desk's
		// sandbox, to reach a host through the core's proxy (internal/sandbox/proxy.go). Its bytes are ssh's: stdin and
		// stdout carry the connection.
		os.Exit(sandbox.RunConnect(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "sandbox":
		// Not in the usage: Smolcorp starts `sandbox enter` inside a desk's sandbox, before anything of the desk's,
		// to check the sandbox holds and tell the core so (internal/sandbox/enter.go). On Linux, `sandbox check` and
		// `sandbox command` are the installer's: whether hires can start in their sandbox, and the one command that
		// sets it up (internal/sandbox/apparmor.go).
		os.Exit(sandbox.Run(args[1:], os.Stdout, os.Stderr))
	case "setup":
		os.Exit(desktop.RunSetup(args[1:]))
	case "demo":
		os.Exit(desktop.RunDemo(args[1:]))
	case "gm":
		os.Exit(gm.Run(args[1:], os.Stdout, os.Stderr))
	case "status", "desks", "say", "hire", "open":
		os.Exit(gm.RunPeople(args[0], args[1:], os.Stdout, os.Stderr))
	case "guard":
		// A debugger in the guard could change its answer.
		protect(procmem.NotDumpable)
		os.Exit(guard.Run(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "mcp":
		os.Exit(gm.RunMCP(args[1:], os.Stdin, os.Stdout, os.Stderr))
	case "quit":
		os.Exit(desktop.RunQuit())
	case "hold":
		os.Exit(desktop.RunHold(args[1:]))
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

// protect keeps the guard's memory from your other programs, as far as the system lets it (internal/procmem), before
// it reads anything: its answer is what lets a command run. A step that fails is said and the guard goes on: nothing
// is less protected than before. The app and the server, which hold keys, do it themselves and fail closed
// (desktop/memory.go).
func protect(steps ...func() error) {
	for _, step := range steps {
		if err := step(); err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: its memory couldn't be kept from other programs (%v)\n", err)
		}
	}
}
