package gm

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

const usageText = `Usage: smolcorp gm <command> [--json]

Asks the Smolcorp running on this computer about the team.

Commands:
  team                          who is on the team and what each is doing
  approvals                     who is waiting on a permission prompt, for you
                                to answer (it only lists; the GM can't answer)
  inbox [--since N] [--wait S]  the events after N, waiting up to S seconds
                                (0-30) for one; the last line is next=<id>
  version                       the app, its API and its features
  accounts                      the Claude accounts the app knows
  hire --role R [--folder F] [--account A] [--model M] [--nickname N]
       [--floor K] [--worktree] [--api-key] [--t3] [--task TEXT | --task-file F]
                                hire an employee (the folder defaults to the
                                one set up for new hires). Not the gm role:
                                the owner hires the GM from Setup. --t3 hires
                                into T3 Code (experimental; needs T3 Code and its
                                t3 command): its own worktree and a T3 Code thread
  brief <hire> (--text TEXT | --file F)
                                send a later brief to a hire already working
                                (it comes to the hire as mail)
  fire <hire> [--force] [--remove-worktree]
                                fire an employee; the app refuses while its
                                worktree has unsaved changes or it is at a question
                                (<hire> is an id, or a name or nickname)
  send <to> (--text T | --file F) [--id ID]
                                a message to a hire (a hire: to the GM, or to
                                a hire the GM allowed). The same --id twice
                                is delivered once; without one, one is made.
  outbox [--since N]            the messages sent, oldest first; N is a time
                                in milliseconds since 1970
  watch <kind> <target> [--pattern P] [--minutes M] [--repeat]
                                ask the app to watch: kind is file, event,
                                idle, exit or time; firings come to inbox
  unwatch <id>                  take a watch away
  watches [--all]               the watches (the app's own only with --all)
  contacts [allow|remove <a> <b> [--one-way]]
                                which hires may write to each other; with no
                                argument, the pairs now
  pin [--desk N] [--folder F]   pin a project to a desk (N counts from 1, as the
                                office shows; without --desk, the desk its session
                                sits at now; the folder is this one unless given)
  unpin --desk N                take what is pinned to desk N away
  nickname <name> [--folder F]  name whoever works in the folder (an empty name,
                                "", takes the nickname away; 16 characters at most)
  tile                          put each hire's window over its desk (a read-only
                                team grid in one window where the terminal
                                can't place windows)
  screens                       the screens, and which one holds the team
  screen <id>                   choose the screen that holds the team (an id from
                                screens; only matters with two or more)

On Linux and Windows, tile, screens, screen and accounts
aren't served yet.

To get a new token: on the Mac, ask the owner to rotate it in Smolcorp's setup;
on Linux and Windows, ask the owner to hire the GM again from Setup.

--json prints the app's answer as it was served. The port is $SMOLCORP_PORT
(default 7710); the token is $SMOLCORP_GM_TOKEN. On the Mac it can also come from
the app's gm-token file (which exists only if "Allow a GM started by hand" is on
in Smolcorp's setup); on Linux and Windows the GM hired from Setup has the variable.
With $SMOLCORP_HIRE_KEY set you are a hire: only send works, signed with that
key; the GM token is never read or sent.

Exit codes: 0 ok, 1 refused or an error from the app, 2 usage, 3 app not running.
`

// Run is `smolcorp gm <args>`; it returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, osEnv(), stdout, stderr)
}

func run(args []string, env Env, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return exitUsage
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usageText)
		return exitOK
	case "team", "approvals", "inbox", "version", "accounts", "tile", "screens", "screen":
	case "hire", "brief", "fire":
		return runDesk(cmd, rest, env, stdout, stderr)
	case "pin", "unpin", "nickname":
		return runPlace(cmd, rest, env, stdout, stderr)
	case "send", "outbox", "watch", "unwatch", "watches", "contacts":
		return runMail(cmd, rest, env, stdout, stderr)
	default:
		return usageError(stderr, fmt.Sprintf("unknown command %q", cmd))
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	var since int64
	var wait int
	if cmd == "inbox" {
		fs.Int64Var(&since, "since", 0, "only events after this id")
		fs.IntVar(&wait, "wait", 0, "seconds to wait for an event")
	}
	if err := fs.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	if cmd == "screen" {
		if fs.NArg() != 1 {
			return usageError(stderr, "screen takes one screen id (see: smolcorp gm screens)")
		}
	} else if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("%s takes no argument %q", cmd, fs.Arg(0)))
	}
	if since < 0 {
		return usageError(stderr, "--since can't be negative")
	}
	if wait < 0 || wait > maxWait {
		return usageError(stderr, fmt.Sprintf("--wait is 0 to %d seconds", maxWait))
	}
	c, err := NewClient(env)
	if err != nil {
		return finish(err, stderr)
	}
	switch cmd {
	case "team":
		t, raw, err := Team(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderTeam(stdout, t) }), stderr)
	case "approvals":
		r, raw, err := Approvals(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderApprovals(stdout, r) }), stderr)
	case "inbox":
		r, raw, err := Events(c, since, wait, 0)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderEvents(stdout, r, time.Local) }), stderr)
	case "version":
		v, raw, err := Version(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { RenderVersion(stdout, v); return nil }), stderr)
	case "tile":
		r, raw, err := Tile(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { RenderTile(stdout, r); return nil }), stderr)
	case "screens":
		r, raw, err := Screens(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { RenderScreens(stdout, r); return nil }), stderr)
	case "screen":
		r, raw, err := ChooseScreen(c, fs.Arg(0))
		return finish(show(err, *asJSON, stdout, raw, func() error { RenderScreens(stdout, r); return nil }), stderr)
	default: // accounts
		raw, err := Accounts(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderAccounts(stdout, raw) }), stderr)
	}
}

// show prints an answer, as served with --json, else as the table render makes it.
func show(err error, asJSON bool, stdout io.Writer, raw []byte, render func() error) error {
	switch {
	case err != nil:
		return err
	case asJSON:
		return prettyJSON(stdout, raw)
	}
	return render()
}

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "smolcorp gm: %s\n\n%s", msg, usageText)
	return exitUsage
}

// finish says what went wrong, if anything, and returns the exit code.
func finish(err error, stderr io.Writer) int {
	if err == nil {
		return exitOK
	}
	var e *Error
	if errors.As(err, &e) {
		fmt.Fprintf(stderr, "smolcorp gm: %s\n", e.Msg)
		return e.Code
	}
	fmt.Fprintf(stderr, "smolcorp gm: %v\n", err)
	return exitApp
}
