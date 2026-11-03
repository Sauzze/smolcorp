package gm

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// The most text a task or a brief may hold, and a file for either.
const (
	maxTaskChars = 8000
	maxTextFile  = 1 << 20
)

// HireOptions is what `gm hire` asks for. Empty fields are left out of the request.
type HireOptions struct {
	Role, Folder, Account, Model, Nickname, Task string
	Floor                                        int // -1: not given
	Worktree, APIKey                             bool
}

// body is the request as POST /setup/gm/hire takes it.
func (o HireOptions) body() map[string]any {
	m := map[string]any{"role": o.Role}
	for k, v := range map[string]string{"folder": o.Folder, "account": o.Account, "model": o.Model, "nickname": o.Nickname, "task": o.Task} {
		if v != "" {
			m[k] = v
		}
	}
	if o.Floor >= 0 {
		m["floor"] = o.Floor
	}
	if o.Worktree {
		m["worktree"] = true
	}
	if o.APIKey {
		m["apiKey"] = true
	}
	return m
}

// Hire asks the app to hire; raw is its answer as served.
func Hire(c *Client, o HireOptions) ([]byte, error) {
	return c.Post("/setup/gm/hire", nil, o.body())
}

// resolveHire turns what was typed into a hire's id: an id as it is, else the
// one team member whose nickname or name is that (any case).
func resolveHire(c *Client, ref string) (string, error) {
	t, _, err := Team(c)
	if err != nil {
		return "", err
	}
	var byName []Member
	for _, m := range t.Team {
		if m.ID == ref {
			return m.ID, nil
		}
		if strings.EqualFold(m.Nickname, ref) || strings.EqualFold(m.Name, ref) {
			byName = append(byName, m)
		}
	}
	switch len(byName) {
	case 1:
		return byName[0].ID, nil
	case 0:
		// Not on the team list: a `live:<session>` id is the app's to judge.
		if strings.HasPrefix(ref, "live:") {
			return ref, nil
		}
		return "", &Error{Code: exitApp, Msg: fmt.Sprintf("nobody on the team is called %q; see `smolcorp gm team`.", oneLine(ref))}
	}
	return "", &Error{Code: exitApp, Msg: fmt.Sprintf("%d people are called %q; use an id from `smolcorp gm team --json`.", len(byName), oneLine(ref))}
}

// Fire fires the hire; the app answers 204, or 200 with {kept, worktree, branch} when it was asked to remove the
// worktree and left it. A refusal comes back whole, the app's words as it wrote them (the unsaved files it lists,
// the question it sees), not cut to one line.
func Fire(c *Client, id string, force, removeWorktree bool) ([]byte, error) {
	body := map[string]any{"id": id}
	if force {
		body["force"] = true
	}
	if removeWorktree {
		body["removeWorktree"] = true
	}
	raw, err := c.Post("/setup/gm/fire", nil, body)
	var e *Error
	if errors.As(err, &e) && e.Status != 0 && e.Reason != "" {
		return nil, &Error{Code: e.Code, Status: e.Status, Msg: plainText(e.Reason), Reason: e.Reason}
	}
	return raw, err
}

// keptNote is " Its worktree and branch were kept: why." from a fire's answer, when the app says it left them.
func keptNote(raw []byte) string {
	var a struct {
		Kept, Worktree, Branch string
	}
	if json.Unmarshal(raw, &a) != nil || a.Kept == "" {
		return ""
	}
	where := ""
	if a.Worktree != "" {
		where = " (" + oneLine(a.Worktree) + ")"
	}
	return fmt.Sprintf(" Its worktree%s and branch were kept, not removed: %s", where, afterColon(oneLine(strings.TrimSuffix(a.Kept, ".")+".")))
}

// afterColon lowers the first letter of a reason that follows a colon ("It holds…" reads "it holds…"),
// unless the word is an acronym ("API…") or starts with a capital for good.
// The pronoun stays: "I can't…", "I'm…" and "I’ll…" are kept as written.
func afterColon(reason string) string {
	word, _, _ := strings.Cut(reason, " ")
	if word == "I" || strings.HasPrefix(word, "I'") || strings.HasPrefix(word, "I\u2019") {
		return reason
	}
	r := []rune(reason)
	if len(r) == 0 || !unicode.IsUpper(r[0]) || (len(r) > 1 && unicode.IsUpper(r[1])) {
		return reason
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// Brief adds text to the end of the working hire's brief; the app mails the hire to read it.
// A refusal comes back whole, as the app wrote it.
func Brief(c *Client, id, text string) ([]byte, error) {
	raw, err := c.Post("/setup/gm/brief", nil, map[string]any{"id": id, "text": text})
	var e *Error
	if errors.As(err, &e) && e.Status != 0 && e.Reason != "" {
		return nil, &Error{Code: e.Code, Status: e.Status, Msg: plainText(e.Reason), Reason: e.Reason}
	}
	return raw, err
}

// briefNote is what the app says when the hire could not be mailed (the part is in the brief all the same).
func briefNote(raw []byte) string {
	var a struct {
		Note string `json:"note"`
	}
	if json.Unmarshal(raw, &a) != nil || a.Note == "" {
		return ""
	}
	return " " + oneLine(a.Note)
}

// plainText is s without control characters but line ends, so an answer can't drive the terminal.
func plainText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, s)
}

// readText reads a file for --task-file or --file; "-" is standard input.
func readText(path string, stdin io.Reader) (string, error) {
	var r io.Reader = stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			var pe *os.PathError
			if errors.As(err, &pe) {
				err = pe.Err
			}
			return "", fmt.Errorf("can't read %s: %v", path, err)
		}
		defer f.Close()
		r = f
	}
	data, err := io.ReadAll(io.LimitReader(r, maxTextFile+1))
	if err != nil {
		return "", fmt.Errorf("can't read %s: %v", path, err)
	}
	if len(data) > maxTextFile {
		return "", fmt.Errorf("%s is longer than 1 MB", path)
	}
	return strings.TrimSpace(string(data)), nil
}

// runDesk is `gm hire`, `gm brief` and `gm fire`. The hire's name comes first
// for brief and fire (`gm fire ada --force`), so flags may follow it.
func runDesk(cmd string, args []string, env Env, stdout, stderr io.Writer) int {
	var ref string
	if cmd != "hire" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ref, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	o := HireOptions{Floor: -1}
	var taskFile, text, file string
	var force, removeWorktree bool
	switch cmd {
	case "hire":
		fs.StringVar(&o.Role, "role", "", "the role to hire")
		fs.StringVar(&o.Folder, "folder", "", "the folder to hire into")
		fs.StringVar(&o.Account, "account", "", "the Claude account")
		fs.StringVar(&o.Model, "model", "", "the model")
		fs.StringVar(&o.Nickname, "nickname", "", "a nickname")
		fs.IntVar(&o.Floor, "floor", -1, "the floor, 0 to 5")
		fs.BoolVar(&o.Worktree, "worktree", false, "a worktree of its own")
		fs.BoolVar(&o.APIKey, "api-key", false, "on the API-key account")
		fs.StringVar(&o.Task, "task", "", "what to do")
		fs.StringVar(&taskFile, "task-file", "", "a file with what to do (- for standard input)")
	case "brief":
		fs.StringVar(&text, "text", "", "the brief")
		fs.StringVar(&file, "file", "", "a file with the brief (- for standard input)")
	case "fire":
		fs.BoolVar(&force, "force", false, "fire even with unsaved changes")
		fs.BoolVar(&removeWorktree, "remove-worktree", false, "delete its worktree")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("%s takes no argument %q", cmd, fs.Arg(0)))
	}
	if cmd != "hire" && strings.TrimSpace(ref) == "" {
		return usageError(stderr, fmt.Sprintf("%s needs the hire: an id, a name or a nickname", cmd))
	}
	switch cmd {
	case "hire":
		if strings.TrimSpace(o.Role) == "" {
			return usageError(stderr, "hire needs --role")
		}
		if o.Floor < -1 || o.Floor > 5 {
			return usageError(stderr, "--floor is 0 to 5")
		}
		if o.Task != "" && taskFile != "" {
			return usageError(stderr, "give --task or --task-file, not both")
		}
		if taskFile != "" {
			t, err := readText(taskFile, os.Stdin)
			if err != nil {
				return usageError(stderr, err.Error())
			}
			o.Task = t
		}
		if len([]rune(o.Task)) > maxTaskChars {
			return usageError(stderr, fmt.Sprintf("the task is at most %d characters", maxTaskChars))
		}
	case "brief":
		if (text == "") == (file == "") {
			return usageError(stderr, "brief needs --text or --file, one of them")
		}
		if file != "" {
			t, err := readText(file, os.Stdin)
			if err != nil {
				return usageError(stderr, err.Error())
			}
			text = t
		}
		if strings.TrimSpace(text) == "" {
			return usageError(stderr, "the brief is empty")
		}
	}
	c, err := NewClient(env)
	if err != nil {
		return finish(err, stderr)
	}
	if cmd == "brief" {
		id, err := resolveHire(c, ref)
		if err != nil {
			return finish(err, stderr)
		}
		raw, err := Brief(c, id, text)
		return finish(show(err, *asJSON, stdout, raw, func() error {
			_, err := fmt.Fprintf(stdout, "Added to %s's brief.%s\n", oneLine(ref), briefNote(raw))
			return err
		}), stderr)
	}
	if cmd == "hire" {
		raw, err := Hire(c, o)
		return finish(show(err, *asJSON, stdout, raw, func() error {
			_, err := fmt.Fprintf(stdout, "Hired a %s.%s\n", oneLine(o.Role), deskNote(raw))
			return err
		}), stderr)
	}
	id, err := resolveHire(c, ref)
	if err != nil {
		return finish(err, stderr)
	}
	raw, err := Fire(c, id, force, removeWorktree)
	if err == nil && !*asJSON {
		fmt.Fprintf(stdout, "Fired %s.%s\n", oneLine(ref), keptNote(raw))
	} else if err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		err = prettyJSON(stdout, raw)
	} else if err == nil {
		fmt.Fprintln(stdout, `{"ok": true}`)
	}
	return finish(err, stderr)
}

// deskNote is " At desk 3." from the hire's answer, when it names one.
func deskNote(raw []byte) string {
	var a struct {
		Desk  *int `json:"desk"`
		Floor *int `json:"floor"`
	}
	if json.Unmarshal(raw, &a) != nil || a.Desk == nil {
		return ""
	}
	return fmt.Sprintf(" At desk %d.", *a.Desk+1)
}
