package gm

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// `smolcorp status | desks | say | hire | open` (1.26, piece X3): the command line for a person at the computer
// Smolcorp runs on. It is the GM's client (client.go), asking the same app with the keys the owner's own tools hold,
// and nothing more:
//   - status and desks read the routes the owner's tool may read (the route table's "tool" column), with the setup-key
//     file, and with the GM token as well when this computer has one;
//   - hire is `gm hire`'s route, so the same gates answer: the route table (a tool alone is refused it), the manager's
//     switch, the fence, the hire cooldown and the folder rules. The app's refusal comes back whole, and the exit code is 1;
//   - say and open use the office window's own routes (POST /talk/say, POST /focus) with the window's key file. say
//     goes through the one copy of the typing rules, as the window and the phone do: never into a desk that is
//     asking (the card is read first, and the app refuses again right before the words), never "!" first, never a
//     T3 Code desk. It never types into a prompt, and it has no way to answer one.
// A hire (a $SMOLCORP_HIRE_KEY in its environment) is not a person: none of these commands runs for it, and no key
// is ever read for it. No key is printed, in an answer or an error.

const peopleUsageText = `Usage: smolcorp <command> [--json]

For you, on the computer Smolcorp runs on:

  status                 the office at a glance: the team, who is working, and
                         who is waiting on you
  desks                  the team, one line a desk: its number, who, what it is
                         doing and for how long
  say <desk> <words>     type words into a desk's agent and press Return, as the
                         office window does. It never types into a prompt: while
                         the desk asks you something, answer it first
  hire <role> [--folder F] [--task TEXT | --task-file F] [--nickname N]
       [--floor K] [--worktree]
                         hire an employee (the same as "smolcorp gm hire"). The
                         app's own rules decide: a refusal is printed whole and
                         the exit code is 1
  open <desk>            bring the desk's terminal window forward

<desk> is a number as the office shows it (3, or 2/3 for floor 2, desk 3), an
id, or a name or nickname. "smolcorp desks" lists them.

--json prints the app's answer as it was served. status, desks and hire use the
setup-key file in the app's support folder, and the GM token as well when you
have one ($SMOLCORP_GM_TOKEN, or the gm-token file). say and open use the office
window's key (the window-key file, which only Smolcorp on Windows keeps; on the
Mac and Linux, type in the office window for now). No key is ever printed. The port is
$SMOLCORP_PORT (default 7710).

Hiring from a terminal needs the GM token (on the Mac: "Allow a GM started by
hand" in setup); with only the setup key the app refuses, and hiring is done
from the office or from setup.

Exit codes: 0 ok, 1 refused or an error from the app, 2 usage, 3 app not running.
`

// windowKeyFileName is the office window's key on this computer (desktop/paths.go), kept beside the other keys.
const (
	setupKeyFileName  = "setup-key"
	windowKeyFileName = "window-key"
)

// RunPeople is `smolcorp status`, `desks`, `say`, `hire` and `open`; it returns the exit code.
func RunPeople(cmd string, args []string, stdout, stderr io.Writer) int {
	return runPeople(cmd, args, osEnv(), stdout, stderr)
}

func peopleUsageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "smolcorp: %s\n\n%s", msg, peopleUsageText)
	return exitUsage
}

func runPeople(cmd string, args []string, env Env, stdout, stderr io.Writer) int {
	// The desk (or the role) comes first for say, open and hire, so flags may follow it.
	var ref string
	if cmd != "status" && cmd != "desks" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ref, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	o := HireOptions{Floor: -1}
	var taskFile string
	if cmd == "hire" {
		fs.StringVar(&o.Folder, "folder", "", "the folder to hire into")
		fs.StringVar(&o.Nickname, "nickname", "", "a nickname")
		fs.IntVar(&o.Floor, "floor", -1, "the floor, 0 to 5")
		fs.BoolVar(&o.Worktree, "worktree", false, "a worktree of its own")
		fs.StringVar(&o.Task, "task", "", "what to do")
		fs.StringVar(&taskFile, "task-file", "", "a file with what to do (- for standard input)")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, peopleUsageText)
			return exitOK
		}
		return peopleUsageError(stderr, err.Error())
	}
	rest := fs.Args()
	// A flag before the desk or the role (`say --json ada ...`) leaves it among the arguments.
	if ref == "" && cmd != "status" && cmd != "desks" && len(rest) > 0 {
		ref, rest = rest[0], rest[1:]
	}
	words := strings.TrimSpace(strings.Join(rest, " "))
	switch cmd {
	case "status", "desks":
		if fs.NArg() > 0 {
			return peopleUsageError(stderr, fmt.Sprintf("%s takes no argument %q", cmd, fs.Arg(0)))
		}
	case "say":
		if strings.TrimSpace(ref) == "" {
			return peopleUsageError(stderr, "say needs the desk, then the words")
		}
		if words == "" {
			return peopleUsageError(stderr, "say needs the words to type")
		}
	case "open":
		if strings.TrimSpace(ref) == "" {
			return peopleUsageError(stderr, "open needs the desk")
		}
		if words != "" {
			return peopleUsageError(stderr, fmt.Sprintf("open takes no argument %q", words))
		}
	case "hire":
		o.Role = strings.TrimSpace(ref)
		if o.Role == "" {
			return peopleUsageError(stderr, "hire needs the role")
		}
		if words != "" {
			return peopleUsageError(stderr, fmt.Sprintf("hire takes no argument %q", words))
		}
		if o.Floor < -1 || o.Floor > 5 {
			return peopleUsageError(stderr, "--floor is 0 to 5")
		}
		if o.Task != "" && taskFile != "" {
			return peopleUsageError(stderr, "give --task or --task-file, not both")
		}
		if taskFile != "" {
			t, err := readText(taskFile, os.Stdin)
			if err != nil {
				return peopleUsageError(stderr, err.Error())
			}
			o.Task = t
		}
		if len([]rune(o.Task)) > maxTaskChars {
			return peopleUsageError(stderr, fmt.Sprintf("the task is at most %d characters", maxTaskChars))
		}
	}
	switch cmd {
	case "say", "open":
		return finish(runAtDesk(cmd, ref, words, *asJSON, env, stdout), stderr)
	}
	c, err := newPersonClient(env)
	if err != nil {
		return finish(err, stderr)
	}
	switch cmd {
	case "status":
		return finish(runStatus(c, *asJSON, stdout), stderr)
	case "desks":
		t, raw, err := Team(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { return renderDesks(stdout, t) }), stderr)
	}
	// hire: the app's refusal comes back whole, as the app wrote it (the unsaved files it lists, the rule it names).
	raw, err := Hire(c, o)
	var e *Error
	if errors.As(err, &e) && e.Status != 0 && e.Reason != "" {
		err = &Error{Code: e.Code, Status: e.Status, Msg: plainText(e.Reason), Reason: e.Reason}
	}
	return finish(show(err, *asJSON, stdout, raw, func() error {
		_, err := fmt.Fprintf(stdout, "Hired a %s.%s\n", oneLine(o.Role), deskNote(raw))
		return err
	}), stderr)
}

// readKeyFile reads a key kept in the support folder: "" when there is no such file, an error for anything else
// that goes wrong. A key that couldn't go in a header is no key.
func readKeyFile(e Env, name string) (string, error) {
	dir, err := e.SupportDir()
	if err != nil {
		return "", &Error{Code: exitApp, Msg: err.Error()}
	}
	data, err := e.readFile(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		return "", &Error{Code: exitApp, Msg: fmt.Sprintf("can't read %s in %s: %v", name, dir, err)}
	}
	key := strings.TrimSpace(string(data))
	for _, r := range key {
		if r < 0x21 || r > 0x7e {
			return "", &Error{Code: exitApp, Msg: fmt.Sprintf("%s in %s is malformed.", name, dir)}
		}
	}
	return key, nil
}

// refuseHire is what a hire's environment gets for every command here: a hire isn't a person, and no key is read.
func refuseHire(e Env) error {
	if strings.TrimSpace(e.Getenv(hireKeyEnv)) != "" {
		return &Error{Code: exitApp, Msg: "these commands are for you, not for a hire. A hire can only `smolcorp gm send`."}
	}
	return nil
}

// newPersonClient is the client for status, desks and hire: the owner's setup key from its file, and the GM
// token too when this computer has one. Anything the app lets neither of them do, it refuses.
func newPersonClient(e Env) (*Client, error) {
	if err := refuseHire(e); err != nil {
		return nil, err
	}
	port, err := e.port()
	if err != nil {
		return nil, &Error{Code: exitUsage, Msg: err.Error()}
	}
	c := newClient(fmt.Sprintf("http://127.0.0.1:%d", port), "")
	token, err := e.token()
	switch {
	case err == nil:
		c.token = token
		c.session = validSession(e.Getenv(sessionEnv))
	case errors.Is(err, errBadToken):
		return nil, err
	default:
		// No GM token here, or its file can't be read: the setup key alone is what a person has.
	}
	if c.setupKey, err = readKeyFile(e, setupKeyFileName); err != nil {
		return nil, err
	}
	if c.token == "" && c.setupKey == "" {
		dir, _ := e.SupportDir()
		return nil, &Error{Code: exitApp, Msg: fmt.Sprintf("no key for Smolcorp here: run this as the person Smolcorp runs for, on that computer (the %s file in %s), or set $%s.", setupKeyFileName, dir, tokenEnv)}
	}
	return c, nil
}

// newWindowClient is the client for say and open: the office window's key, from its file, and nothing else.
func newWindowClient(e Env) (*Client, error) {
	if err := refuseHire(e); err != nil {
		return nil, err
	}
	port, err := e.port()
	if err != nil {
		return nil, &Error{Code: exitUsage, Msg: err.Error()}
	}
	key, err := readKeyFile(e, windowKeyFileName)
	if err != nil {
		return nil, err
	}
	if key == "" {
		dir, _ := e.SupportDir()
		return nil, &Error{Code: exitApp, Msg: fmt.Sprintf("there is no office window key in %s: say and open need it. Only Smolcorp on Windows keeps one in a file; on the Mac and Linux the office keeps it in memory, so for now type in the office window.", dir)}
	}
	c := newClient(fmt.Sprintf("http://127.0.0.1:%d", port), "")
	c.windowKey = key
	return c, nil
}

// resolveDesk is the team member a person means by ref: an id or session id; else a desk as the office shows it
// ("3", or "2/3" on floor 2); else a name or nickname (any case). The first of these to match anyone decides,
// and more than one match is asked about, never guessed.
func resolveDesk(c *Client, ref string) (Member, error) {
	t, _, err := Team(c)
	if err != nil {
		return Member{}, err
	}
	ref = strings.TrimSpace(ref)
	for _, match := range []func(Member) bool{
		func(m Member) bool { return m.ID == ref || (m.SessionID != "" && m.SessionID == ref) },
		func(m Member) bool { return m.Desk != nil && deskLabel(m) == ref },
		func(m Member) bool { return strings.EqualFold(m.Nickname, ref) || strings.EqualFold(m.Name, ref) },
	} {
		var found []Member
		for _, m := range t.Team {
			if match(m) {
				found = append(found, m)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0], nil
		}
		return Member{}, &Error{Code: exitApp, Msg: fmt.Sprintf("%d desks match %q; use an id from `smolcorp desks --json`.", len(found), oneLine(ref))}
	}
	return Member{}, &Error{Code: exitApp, Msg: fmt.Sprintf("no desk is called %q; see `smolcorp desks`.", oneLine(ref))}
}

// deskCard is what GET /talk says of a desk: whether it asks, and whether the window may type into it now.
type deskCard struct {
	Asking *struct {
		Label string `json:"label"`
	} `json:"asking"`
	Typing struct {
		OK  bool   `json:"ok"`
		Why string `json:"why"`
	} `json:"typing"`
}

// runAtDesk is `say` and `open`, which both act on one live desk through the office window's routes.
func runAtDesk(cmd, ref, words string, asJSON bool, env Env, stdout io.Writer) error {
	c, err := newWindowClient(env)
	if err != nil {
		return err
	}
	// The desks are read with the setup key, when the window's key alone doesn't open the team.
	team, err := newPersonClient(env)
	if err != nil {
		return err
	}
	m, err := resolveDesk(team, ref)
	if err != nil {
		return err
	}
	who := m.displayName()
	if m.SessionID == "" {
		return &Error{Code: exitApp, Msg: fmt.Sprintf("%s has no open desk: nobody is running there now.", who)}
	}
	q := url.Values{"session": {m.SessionID}}
	if cmd == "open" {
		raw, err := c.Post("/focus", q, nil)
		return showDone(err, asJSON, stdout, raw, fmt.Sprintf("Brought %s's terminal forward.", who))
	}
	// Say: look at the desk first. Asking something, it is never typed into: its menu would take the words.
	rawCard, err := c.Get("/talk", q, requestTimeout)
	if err != nil {
		return err
	}
	var card deskCard
	if err := json.Unmarshal(rawCard, &card); err != nil {
		return decodeError(err)
	}
	if card.Asking != nil {
		what := ""
		if l := oneLine(card.Asking.Label); l != "" {
			what = " (" + truncate(l, 60) + ")"
		}
		return &Error{Code: exitApp, Msg: fmt.Sprintf("%s is asking you something%s: answer it first, in the office. Nothing was typed.", who, what)}
	}
	if !card.Typing.OK {
		why := oneLine(card.Typing.Why)
		if why == "" {
			why = "its desk can't be typed into now"
		}
		return &Error{Code: exitApp, Msg: fmt.Sprintf("can't type into %s: %s Nothing was typed.", who, why)}
	}
	// The app checks everything again right before the words, so a desk that began asking meanwhile still refuses.
	raw, err := c.Post("/talk/say", q, map[string]any{"text": words, "send": true})
	return showDone(err, asJSON, stdout, raw, fmt.Sprintf("Sent to %s.", who))
}

// showDone prints a command's success line, or its answer as served with --json (an empty answer is {"ok": true}).
func showDone(err error, asJSON bool, stdout io.Writer, raw []byte, line string) error {
	switch {
	case err != nil:
		return err
	case !asJSON:
		_, err = fmt.Fprintln(stdout, line)
		return err
	case len(strings.TrimSpace(string(raw))) == 0:
		_, err = fmt.Fprintln(stdout, `{"ok": true}`)
		return err
	}
	return prettyJSON(stdout, raw)
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
