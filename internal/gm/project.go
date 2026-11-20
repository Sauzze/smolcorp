package gm

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"
)

// projectWait is how long `gm project new --clone` waits for the app's answer: the app stops a clone after 10
// minutes and takes its folder away, so this is a little longer.
const projectWait = 11 * time.Minute

// ProjectAnswer is the app's answer to a new project: the folder made, and the address cloned into it, if any.
type ProjectAnswer struct {
	Folder string `json:"folder"`
	Cloned string `json:"cloned"`
}

// NewProject asks the app to make a new project folder called name in the GM's projects folder, a clone of address
// when it isn't "" (POST /setup/gm/project). The GM's sandbox can't make one itself.
func NewProject(c *Client, name, address string) (ProjectAnswer, []byte, error) {
	body := map[string]any{"name": name}
	if address != "" {
		body["clone"] = address
	}
	data, err := json.Marshal(body)
	if err != nil {
		return ProjectAnswer{}, nil, &Error{Code: exitApp, Msg: "can't encode the request: " + err.Error()}
	}
	timeout := requestTimeout
	if address != "" {
		timeout = projectWait
	}
	raw, err := c.do(http.MethodPost, "/setup/gm/project", nil, data, timeout)
	if err != nil {
		return ProjectAnswer{}, nil, err
	}
	var a ProjectAnswer
	if json.Unmarshal(raw, &a) != nil || a.Folder == "" {
		return ProjectAnswer{}, raw, &Error{Code: exitApp, Msg: "the app's answer isn't one this tool reads."}
	}
	return a, raw, nil
}

// runProject is `project new <name> [--clone <https address>]`.
func runProject(args []string, env Env, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "new" {
		return usageError(stderr, "project takes new: project new <name> [--clone <https address>]")
	}
	fs := flag.NewFlagSet("project", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	address := fs.String("clone", "", "an https address to clone into it")
	plain, err := parseMixed(fs, args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	if len(plain) != 1 || plain[0] == "" {
		return usageError(stderr, "project new takes one folder name")
	}
	c, err := NewClient(env)
	if err != nil {
		return finish(err, stderr)
	}
	a, raw, err := NewProject(c, plain[0], *address)
	return finish(show(err, *asJSON, stdout, raw, func() error {
		if a.Cloned != "" {
			_, err := fmt.Fprintf(stdout, "Made the project folder %s, a clone of %s.\n", oneLine(a.Folder), oneLine(a.Cloned))
			return err
		}
		_, err := fmt.Fprintf(stdout, "Made the project folder %s.\n", oneLine(a.Folder))
		return err
	}), stderr)
}
