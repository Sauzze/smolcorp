package gm

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// `smolcorp status | desks` (1.26, piece X3): the command line for a person at the computer Smolcorp runs on. It is
// the GM's client (client.go), asking the same app with the one key the owner's own tools hold, and nothing more:
//   - status and desks read the routes the owner's tool may read (the route table's "tool" column), with a tool's key
//     the app hands this process once you click Allow in its window, after checking the code printed here (1.26, SK2:
//     no file holds a key that opens anything; login.go's AskForKey). The key stays in this process's memory and goes
//     with it. The GM's token is never read or sent from here: a request carrying it would make the app note a GM
//     "started by hand" (and move the GM's role to the calling session), which a person's read must never do.
//   - say, open and hire are refused in one line, with no request and no key file read. They wait for X1's scoped
//     key: a named client minted in Settings, with "type", "open" and "hire" as scopes the owner ticks.
//     TODO(X1): when that key exists, say (through POST /talk/say, never into a prompt), open (POST /focus) and
//     hire (POST /setup/gm/hire) come back in this file with it. The first version of them, which asked the app for
//     the desk's card before typing, is in commit ded2ebd6.
// refuseHire is a polite refusal, not a fence: it reads an environment variable, which a hire's own command line can
// clear. What keeps a hire from these commands is the guard (internal/guard/rules.go, "smolcorp hire|say|open"), and
// the fact that none of them holds a key a hire couldn't already read. No key is ever printed, in an answer or an error.

const peopleUsageText = `Usage: smolcorp <command> [--json]

For you, on the computer Smolcorp runs on:

  status                 the office at a glance: the team, who is working, and
                         who is waiting on you
  desks                  the team, one line a desk: its number, who, what it is
                         doing and for how long

Not yet: say, hire and open come with app keys in Settings. Until then, type to
a desk, hire and bring a terminal forward from the office, and hire from Setup.

--json prints the app's answer as it was served. status and desks ask Smolcorp
for a key each time: it shows a code in its window, and you click Allow there
once it matches the one printed here. No key is ever printed or kept in a file.
The port is $SMOLCORP_PORT (default 7710).

Exit codes: 0 ok, 1 refused or an error from the app, 2 usage, 3 app not running.
`

// notYet is what say, open and hire answer, whatever follows them: they make no request and read no key file.
var notYet = map[string]string{
	"say":  "smolcorp say comes with app keys in Settings. For now, type to the desk in the office.",
	"open": "smolcorp open comes with app keys in Settings. For now, bring the terminal forward from the office.",
	"hire": "smolcorp hire comes with app keys in Settings. For now, hire from the office or from Setup.",
}

// RunPeople is `smolcorp status`, `desks`, `say`, `hire` and `open`; it returns the exit code.
func RunPeople(cmd string, args []string, stdout, stderr io.Writer) int {
	return runPeople(cmd, args, osEnv(), stdout, stderr)
}

// RunPeopleIn is RunPeople with the environment given: for a test that runs the commands against the real app's router
// (desktop/peoplecli_test.go), which has its own support folder and port.
func RunPeopleIn(env Env, cmd string, args []string, stdout, stderr io.Writer) int {
	return runPeople(cmd, args, env, stdout, stderr)
}

func peopleUsageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "smolcorp: %s\n\n%s", msg, peopleUsageText)
	return exitUsage
}

func runPeople(cmd string, args []string, env Env, stdout, stderr io.Writer) int {
	if msg, ok := notYet[cmd]; ok {
		return finish(&Error{Code: exitApp, Msg: msg}, stderr)
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, peopleUsageText)
			return exitOK
		}
		return peopleUsageError(stderr, err.Error())
	}
	if fs.NArg() > 0 {
		return peopleUsageError(stderr, fmt.Sprintf("%s takes no argument %q", cmd, fs.Arg(0)))
	}
	c, err := newPersonClient(env, stderr)
	if err != nil {
		return finish(err, stderr)
	}
	if cmd == "status" {
		return finish(runStatus(c, *asJSON, stdout), stderr)
	}
	t, raw, err := Team(c)
	return finish(show(err, *asJSON, stdout, raw, func() error { return renderDesks(stdout, t) }), stderr)
}

// refuseHire is what a hire's environment gets from status and desks: a hire isn't a person, and no key is read.
// It is a polite refusal, not a fence: a command line can empty or unset $SMOLCORP_HIRE_KEY first, and the guard's
// rules (not this check) are what stand in the way of that.
func refuseHire(e Env) error {
	if strings.TrimSpace(e.Getenv(hireKeyEnv)) != "" {
		return &Error{Code: exitApp, Msg: "these commands are for you, not for a hire. A hire can only `smolcorp gm send` and `smolcorp gm preview`."}
	}
	return nil
}

// newPersonClient is the client for status and desks: a tool's key, asked for and allowed in the app's window (its
// code said in said), and nothing else. It never reads the GM token, and it sends no GM session id, so no request
// from it can pass for the GM.
func newPersonClient(e Env, said io.Writer) (*Client, error) {
	if err := refuseHire(e); err != nil {
		return nil, err
	}
	port, err := e.port()
	if err != nil {
		return nil, &Error{Code: exitUsage, Msg: err.Error()}
	}
	c := newClient(fmt.Sprintf("http://127.0.0.1:%d", port), "")
	folder, _ := os.Getwd()
	if c.setupKey, err = AskForKey(e, AskTool, folder, said); err != nil {
		return nil, err
	}
	return c, nil
}

// renderDesks is the team as a person reads it: one line a desk, its number as the office shows it.
func renderDesks(w io.Writer, t TeamReply) error {
	if len(t.Team) == 0 {
		_, err := fmt.Fprintln(w, "Nobody on the team yet.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DESK\tNAME\tROLE\tSTATE\tIDLE\tSTATUS")
	for _, m := range t.Team {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", deskLabel(m), m.displayName(), orDash(oneLine(m.Role)),
			orDash(m.stateName()), orDash(formatIdle(m.IdleFor)), truncate(oneLine(m.Status), 60))
	}
	return tw.Flush()
}

// teamCounts says how many on the team are working, asking you, idle, in error, or not running.
func teamCounts(team []Member) string {
	var working, asking, idle, failed, away int
	for _, m := range team {
		switch {
		case !m.Live:
			away++
		case m.LiveState == "thinking":
			working++
		case m.LiveState == "asking":
			asking++
		case m.LiveState == "error":
			failed++
		default:
			idle++
		}
	}
	var parts []string
	for _, p := range []struct {
		n    int
		what string
	}{{working, "working"}, {asking, "asking you"}, {idle, "idle"}, {failed, "in error"}, {away, "not running"}} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.what))
		}
	}
	return strings.Join(parts, ", ")
}

// runStatus is `smolcorp status`: the app, the team's states, and who waits on you. --json gives the three
// answers as served, under version, team and approvals.
func runStatus(c *Client, asJSON bool, stdout io.Writer) error {
	v, rawV, err := Version(c)
	if err != nil {
		return err
	}
	t, rawT, err := Team(c)
	if err != nil {
		return err
	}
	a, rawA, err := Approvals(c)
	if err != nil {
		return err
	}
	if asJSON {
		return prettyJSON(stdout, mustJSON(map[string]json.RawMessage{"version": rawV, "team": rawT, "approvals": rawA}))
	}
	app := strings.TrimSpace(oneLine(v.App + " " + v.Version))
	if app == "" {
		app = appName
	}
	if len(t.Team) == 0 {
		fmt.Fprintf(stdout, "%s. Nobody on the team yet.\n", app)
	} else {
		fmt.Fprintf(stdout, "%s. %d on the team: %s.\n", app, len(t.Team), teamCounts(t.Team))
	}
	if len(a.Approvals) == 0 {
		_, err = fmt.Fprintln(stdout, "Nobody is waiting for you.")
		return err
	}
	fmt.Fprintln(stdout, "\nWaiting for you:")
	return RenderApprovals(stdout, a)
}

// mustJSON is v as JSON; a map of raw answers always encodes.
func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
