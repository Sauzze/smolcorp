package gm

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
)

// PreviewAnswer is what the app says of a declared preview: the desk's session and the address ("" when taken away).
type PreviewAnswer struct {
	Session string `json:"session"`
	URL     string `json:"url"`
}

// Preview declares the address the office's TV shows for a desk (1.26 V1), or takes it away with address "": the
// GM names one of its hires; a hire (signed with its key) sets its own desk's, and hire is then "" or itself. The
// app checks the address (loopback http only) and the desk; a refusal comes back whole, as the app wrote it.
func Preview(c *Client, address, hire string) (PreviewAnswer, []byte, error) {
	body := map[string]any{"url": address}
	if hire != "" {
		body["hire"] = hire
	}
	raw, err := c.Post("/setup/preview", nil, body)
	var e *Error
	if errors.As(err, &e) && e.Status != 0 && e.Reason != "" {
		return PreviewAnswer{}, nil, &Error{Code: e.Code, Status: e.Status, Msg: plainText(e.Reason), Reason: e.Reason}
	}
	if err != nil {
		return PreviewAnswer{}, nil, err
	}
	var a PreviewAnswer
	if json.Unmarshal(raw, &a) != nil {
		return PreviewAnswer{}, raw, &Error{Code: exitApp, Msg: "the app's answer isn't one this tool reads."}
	}
	return a, raw, nil
}

// runPreview is `preview <url> [--hire H]` and `preview --clear [--hire H]`.
func runPreview(args []string, env Env, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	hire := fs.String("hire", "", "the hire whose desk it is")
	clear := fs.Bool("clear", false, "take the preview away")
	plain, err := parseMixed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	address := ""
	switch {
	case *clear && len(plain) == 0:
	case !*clear && len(plain) == 1 && plain[0] != "":
		address = plain[0]
	default:
		return usageError(stderr, "preview takes one address (http://localhost:<port>), or --clear")
	}
	c, err := NewSendClient(env)
	if err != nil {
		return finish(err, stderr)
	}
	if c.hireKey == "" && *hire == "" {
		return usageError(stderr, "say whose desk with --hire <hire> (an id, a name or a nickname)")
	}
	a, raw, err := Preview(c, address, *hire)
	return finish(show(err, *asJSON, stdout, raw, func() error {
		whose := "Your desk's"
		if *hire != "" && c.hireKey == "" {
			whose = oneLine(*hire) + "'s desk's"
		}
		if a.URL == "" {
			_, err := fmt.Fprintf(stdout, "%s preview is taken away; the TV shows what is found, if anything.\n", whose)
			return err
		}
		_, err := fmt.Fprintf(stdout, "%s preview is %s.\n", whose, oneLine(a.URL))
		return err
	}), stderr)
}
