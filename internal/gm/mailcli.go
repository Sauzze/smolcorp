package gm

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// parseMixed parses flags that may come before, between or after the plain
// arguments (`send Rev --text hi` as well as `send --text hi Rev`), and
// returns the plain ones.
func parseMixed(fs *flag.FlagSet, args []string) ([]string, error) {
	var plain []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return plain, nil
		}
		plain = append(plain, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// runMail is send, outbox, watch, unwatch, watches and contacts.
func runMail(cmd string, args []string, env Env, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	var (
		text, file, id, pattern string
		since                   int64
		minutes                 int
		repeat, all, oneWay     bool
	)
	switch cmd {
	case "send":
		fs.StringVar(&text, "text", "", "the message")
		fs.StringVar(&file, "file", "", "a file holding the message")
		fs.StringVar(&id, "id", "", "the message id")
	case "outbox":
		fs.Int64Var(&since, "since", 0, "only messages after this time")
	case "watch":
		fs.StringVar(&pattern, "pattern", "", "what to look for")
		fs.IntVar(&minutes, "minutes", 0, "minutes")
		fs.BoolVar(&repeat, "repeat", false, "fire again and again")
	case "watches":
		fs.BoolVar(&all, "all", false, "include the app's own watches")
	case "contacts":
		fs.BoolVar(&oneWay, "one-way", false, "only from the first to the second")
	}
	given := map[string]bool{}
	plain, err := parseMixed(fs, args)
	if err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })

	need := func(n int, what string) bool {
		if len(plain) != n {
			usageError(stderr, fmt.Sprintf("%s takes %s", cmd, what))
			return false
		}
		return true
	}
	switch cmd {
	case "send":
		if !need(1, "one argument: who to send to") {
			return exitUsage
		}
		if given["text"] == given["file"] {
			return usageError(stderr, "send takes --text or --file, one of them")
		}
		if given["file"] {
			t, err := readMessageFile(file)
			if err != nil {
				return finish(err, stderr)
			}
			text = t
		}
		if strings.TrimSpace(text) == "" {
			return usageError(stderr, "there is nothing to send")
		}
		if given["id"] && !validSendID(id) {
			return usageError(stderr, fmt.Sprintf("--id uses letters, digits, - _ and . only, %d at most", idMost))
		}
		if !given["id"] {
			id = newSendID(time.Now())
		}
	case "outbox":
		if !need(0, "no argument") {
			return exitUsage
		}
		if since < 0 {
			return usageError(stderr, "--since can't be negative")
		}
	case "watch":
		if !need(2, "a kind and a target") {
			return exitUsage
		}
		if minutes < 0 {
			return usageError(stderr, "--minutes can't be negative")
		}
		if strings.TrimSpace(plain[1]) == "" {
			return usageError(stderr, "watch needs a target")
		}
	case "unwatch":
		if !need(1, "one argument: the watch's id") {
			return exitUsage
		}
	case "watches":
		if !need(0, "no argument") {
			return exitUsage
		}
	case "contacts":
		switch {
		case len(plain) == 0 && !oneWay:
		case len(plain) == 3 && (plain[0] == "allow" || plain[0] == "remove"):
		default:
			return usageError(stderr, "contacts takes nothing, or allow|remove <a> <b>")
		}
	}

	var c *Client
	if cmd == "send" {
		c, err = NewSendClient(env)
	} else {
		c, err = NewClient(env)
	}
	if err != nil {
		return finish(err, stderr)
	}

	switch cmd {
	case "send":
		r, raw, err := Send(c, plain[0], text, id)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderSend(stdout, r, plain[0]) }), stderr)
	case "outbox":
		msgs, raw, err := Outbox(c, since)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderOutbox(stdout, msgs, time.Local) }), stderr)
	case "watch":
		w, raw, err := SetWatch(c, WatchSpec{Kind: plain[0], Target: plain[1], Pattern: pattern, Minutes: minutes, Repeat: repeat})
		return finish(show(err, *asJSON, stdout, raw, func() error {
			_, err := fmt.Fprintf(stdout, "Watch %s set: %s %s. Firings come to `smolcorp gm inbox`.\n", oneLine(w.ID), oneLine(w.Kind), oneLine(w.Target))
			return err
		}), stderr)
	case "unwatch":
		raw, err := Unwatch(c, plain[0])
		return finish(show(err, *asJSON, stdout, raw, func() error {
			_, err := fmt.Fprintf(stdout, "Watch %s taken away.\n", oneLine(plain[0]))
			return err
		}), stderr)
	case "watches":
		list, raw, err := Watches(c, all)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderWatches(stdout, list, time.Local) }), stderr)
	}
	// contacts: the pairs now come with the team; allow and remove answer them.
	if len(plain) == 0 {
		t, raw, err := Team(c)
		return finish(show(err, *asJSON, stdout, raw, func() error { return RenderContacts(stdout, t.Contacts, t.Team) }), stderr)
	}
	pairs, raw, err := SetContacts(c, plain[1], plain[2], oneWay, plain[0] == "remove")
	return finish(show(err, *asJSON, stdout, raw, func() error {
		team, _, terr := Team(c)
		if terr != nil {
			team.Team = nil // the pairs by id then
		}
		return RenderContacts(stdout, pairs, team.Team)
	}), stderr)
}
