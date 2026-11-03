package gm

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// mostDesks is how many desks a pin can name (the app's own limit: 9 keys). The app judges it; this only
// keeps a typo from going out.
const mostDesks = 9

// absFolder is a folder as the app keeps it: absolute and clean; "" is the current directory.
func absFolder(f string) (string, error) {
	if strings.TrimSpace(f) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("can't tell which folder this is: %v", err)
		}
		return wd, nil
	}
	return filepath.Abs(f)
}

// sameFolder is whether a and b are one folder, as typed or as the real path (a link and its target).
func sameFolder(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// deskOfFolder is the desk (counted from 0) the live session in folder sits at on the ground floor now.
func deskOfFolder(c *Client, folder string) (int, error) {
	t, _, err := Team(c)
	if err != nil {
		return 0, err
	}
	found := map[int]bool{}
	elsewhere := false
	for _, m := range t.Team {
		if !m.Live || m.Desk == nil || !sameFolder(m.Folder, folder) {
			continue
		}
		if m.Floor != nil && *m.Floor != 0 {
			elsewhere = true
			continue
		}
		found[*m.Desk] = true
	}
	switch {
	case len(found) == 1:
		for d := range found {
			return d, nil
		}
	case len(found) > 1:
		return 0, &Error{Code: exitApp, Msg: fmt.Sprintf("more than one session in %s sits at a desk; say which with --desk N.", oneLine(folder))}
	case elsewhere:
		return 0, &Error{Code: exitApp, Msg: fmt.Sprintf("the session in %s is not on the ground floor, where desks are pinned; say a desk with --desk N.", oneLine(folder))}
	}
	return 0, &Error{Code: exitApp, Msg: fmt.Sprintf("no session in %s sits at a desk now; say which with --desk N (1 to %d).", oneLine(folder), mostDesks)}
}

// Pin pins folder to the desk (counted from 0): the app's /setup/pin, which the GM's own token opens.
func Pin(c *Client, folder string, desk int) ([]byte, error) {
	return c.Post("/setup/pin", url.Values{"key": {strconv.Itoa(desk)}, "folder": {folder}}, nil)
}

// Unpin takes whatever is pinned to the desk (counted from 0) away.
func Unpin(c *Client, desk int) ([]byte, error) {
	return c.Post("/setup/unpin", url.Values{"key": {strconv.Itoa(desk)}}, nil)
}

// Nickname names whoever works in folder (empty: no nickname), with the app's /setup/gm/nickname.
func Nickname(c *Client, folder, name string) ([]byte, error) {
	raw, err := c.Post("/setup/gm/nickname", nil, map[string]any{"folder": folder, "name": name})
	var e *Error
	if errors.As(err, &e) && e.Status != 0 && e.Reason != "" {
		return nil, &Error{Code: e.Code, Status: e.Status, Msg: plainText(e.Reason), Reason: e.Reason}
	}
	return raw, err
}

// runPlace is `gm pin`, `gm unpin` and `gm nickname`. The nickname comes first
// (`gm nickname Ada --folder F`), so flags may follow it.
func runPlace(cmd string, args []string, env Env, stdout, stderr io.Writer) int {
	var name string
	hasName := false
	if cmd == "nickname" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args, hasName = args[0], args[1:], true
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	var folder string
	var desk int
	if cmd != "unpin" {
		fs.StringVar(&folder, "folder", "", "the project's folder (default: this one)")
	}
	if cmd != "nickname" {
		fs.IntVar(&desk, "desk", 0, "the desk, as the office numbers them (from 1)")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	deskGiven := false
	fs.Visit(func(f *flag.Flag) { deskGiven = deskGiven || f.Name == "desk" })
	if cmd == "nickname" && !hasName {
		return usageError(stderr, "nickname needs the name first, before any flag (an empty one, \"\", takes the nickname away)")
	}
	if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("%s takes no argument %q", cmd, fs.Arg(0)))
	}
	if deskGiven && (desk < 1 || desk > mostDesks) {
		return usageError(stderr, fmt.Sprintf("--desk is 1 to %d, as the office numbers them", mostDesks))
	}
	if cmd == "unpin" && !deskGiven {
		return usageError(stderr, "unpin needs --desk N")
	}
	if cmd != "unpin" {
		abs, err := absFolder(folder)
		if err != nil {
			return usageError(stderr, err.Error())
		}
		folder = abs
	}
	if cmd == "nickname" {
		name = strings.TrimSpace(name)
		if len([]rune(name)) > 16 || strings.ContainsAny(name, "\r\n") {
			return usageError(stderr, "a nickname is one line of at most 16 characters")
		}
	}
	c, err := NewClient(env)
	if err != nil {
		return finish(err, stderr)
	}
	var raw []byte
	var said string
	switch cmd {
	case "pin":
		index := desk - 1
		if !deskGiven {
			if index, err = deskOfFolder(c, folder); err != nil {
				return finish(err, stderr)
			}
		}
		raw, err = Pin(c, folder, index)
		said = fmt.Sprintf("Pinned %s to desk %d.", oneLine(folder), index+1)
	case "unpin":
		raw, err = Unpin(c, desk-1)
		said = fmt.Sprintf("Took what was pinned to desk %d away.", desk)
	default:
		raw, err = Nickname(c, folder, name)
		said = fmt.Sprintf("Named whoever works in %s: %s.", oneLine(folder), oneLine(name))
		if name == "" {
			said = fmt.Sprintf("Took the nickname of whoever works in %s away.", oneLine(folder))
		}
	}
	if err != nil {
		return finish(err, stderr)
	}
	switch {
	case !*asJSON:
		fmt.Fprintln(stdout, said)
	case len(strings.TrimSpace(string(raw))) > 0 && json.Valid(raw):
		err = prettyJSON(stdout, raw)
	default:
		fmt.Fprintln(stdout, `{"ok": true}`)
	}
	return finish(err, stderr)
}
