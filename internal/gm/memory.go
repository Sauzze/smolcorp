package gm

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// `smolcorp gm memory` (1.26, MEM4): what this office has taught its GM. The app's answer starts with its own header,
// which says what the notes are and aren't, and this command prints it first, as the MCP tool gives it. The GM reads
// and proposes here; only the owner keeps a note, in Setup or on the phone, and no command here keeps or deletes one.

// MemoryMark starts every line the memory command prints as text, as the MCP tool gives it too: the app's summary of
// the GM's transcript leaves out any result that holds it, so a note's words stay out of a summary even when the
// command's name and the header were cut away on the way (RT126MEM4 N1). The --json answer starts with the header.
const MemoryMark = "[memory] "

// markMemory is text with MemoryMark at the start of each of its lines (an empty line gets the mark alone).
func markMemory(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if line == "" {
			b.WriteString(strings.TrimSpace(MemoryMark) + "\n")
			continue
		}
		b.WriteString(MemoryMark + line + "\n")
	}
	return b.String()
}

// MemoryNote is a kept note as the GM reads it: its name, type and description, and with show its fields.
type MemoryNote struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Rule        string `json:"rule"`
	Why         string `json:"why"`
	How         string `json:"how"`
}

// MemoryReply is the answer of GET /setup/gm/memory.
type MemoryReply struct {
	Header   string       `json:"header"`
	Notes    []MemoryNote `json:"notes"`
	Withheld int          `json:"withheld"`
}

// MemoryProposal is a note proposed: its fields, and where its words came from, Quote (the owner's own words, which
// the app looks for in what the owner said to the GM) or Seen (what the GM saw them in: a file and line, a command).
type MemoryProposal struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Body        string `json:"body,omitempty"`
	Rule        string `json:"rule,omitempty"`
	Why         string `json:"why,omitempty"`
	How         string `json:"how,omitempty"`
	Quote       string `json:"quote,omitempty"`
	Seen        string `json:"seen,omitempty"`
}

// MemoryProposed is what the app says of a proposal.
type MemoryProposed struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	From  string `json:"from"`
	Said  string `json:"said"`
	Held  string `json:"held"`
}

// wholeReason is a refusal with the app's own words whole, which a note's checks may need more than a line for.
func wholeReason(err error) error {
	var e *Error
	if errors.As(err, &e) && e.Status != 0 && e.Reason != "" {
		return &Error{Code: e.Code, Status: e.Status, Msg: plainText(e.Reason), Reason: e.Reason}
	}
	return err
}

// Memory asks for the kept notes' index, or with name that note whole.
func Memory(c *Client, name string) (MemoryReply, []byte, error) {
	var query url.Values
	if name != "" {
		query = url.Values{"name": {name}}
	}
	raw, err := c.Get("/setup/gm/memory", query, requestTimeout)
	if err != nil {
		return MemoryReply{}, nil, wholeReason(err)
	}
	var r MemoryReply
	if err := json.Unmarshal(raw, &r); err != nil || r.Header == "" {
		return MemoryReply{}, raw, &Error{Code: exitApp, Msg: "the app's answer isn't one this tool reads."}
	}
	return r, raw, nil
}

// ProposeMemory proposes a note, which waits for the owner's tap.
func ProposeMemory(c *Client, p MemoryProposal) (MemoryProposed, []byte, error) {
	raw, err := c.Post("/setup/gm/memory/propose", nil, p)
	if err != nil {
		return MemoryProposed{}, nil, wholeReason(err)
	}
	var a MemoryProposed
	if json.Unmarshal(raw, &a) != nil {
		return MemoryProposed{}, raw, &Error{Code: exitApp, Msg: "the app's answer isn't one this tool reads."}
	}
	return a, raw, nil
}

// WithdrawMemory takes back a proposal still waiting.
func WithdrawMemory(c *Client, id string) ([]byte, error) {
	raw, err := c.Post("/setup/gm/memory/withdraw", url.Values{"id": {id}}, nil)
	return raw, wholeReason(err)
}

// RenderMemory prints the header, then the index, or each note whole, each line marked (MemoryMark).
func RenderMemory(w io.Writer, r MemoryReply, whole bool) error {
	var b strings.Builder
	b.WriteString(oneLine(r.Header) + "\n\n")
	if len(r.Notes) == 0 {
		b.WriteString("No notes are kept yet.\n")
	}
	for _, n := range r.Notes {
		if !whole {
			fmt.Fprintf(&b, "- %s (%s): %s\n", oneLine(n.Name), oneLine(n.Type), oneLine(n.Description))
			continue
		}
		fmt.Fprintf(&b, "Name: %s\nType: %s\nDescription: %s\n", oneLine(n.Name), oneLine(n.Type), oneLine(n.Description))
		if n.Type == "feedback" {
			fmt.Fprintf(&b, "Rule: %s\nWhy: %s\nHow to apply it: %s\n\n", oneLine(n.Rule), oneLine(n.Why), oneLine(n.How))
		} else {
			fmt.Fprintf(&b, "%s\n\n", oneLine(n.Body))
		}
	}
	if r.Withheld > 0 {
		fmt.Fprintf(&b, "\n%d kept note(s) withheld: they don't pass Smolcorp's checks today. The owner sees them in Setup.\n", r.Withheld)
	}
	_, err := io.WriteString(w, markMemory(b.String()))
	return err
}

// runMemory is `memory`, `memory show <name>`, `memory propose …` and `memory withdraw <id>`.
func runMemory(args []string, env Env, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("memory", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	asJSON := fs.Bool("json", false, "the JSON as served")
	var p MemoryProposal
	for name, field := range map[string]*string{"name": &p.Name, "description": &p.Description, "type": &p.Type, "body": &p.Body,
		"rule": &p.Rule, "why": &p.Why, "how": &p.How, "quote": &p.Quote, "seen": &p.Seen} {
		fs.StringVar(field, name, "", name)
	}
	plain, err := parseMixed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usageText)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	sub := ""
	if len(plain) > 0 {
		sub = plain[0]
	}
	proposing := p != MemoryProposal{}
	switch {
	case sub == "" && len(plain) == 0 && !proposing:
	case sub == "show" && len(plain) == 2 && plain[1] != "" && !proposing:
	case sub == "withdraw" && len(plain) == 2 && plain[1] != "" && !proposing:
	case sub == "propose" && len(plain) == 1:
		if p.Name == "" || p.Description == "" || p.Type == "" || (p.Quote == "") == (p.Seen == "") {
			return usageError(stderr, "memory propose needs --name, --description, --type, and --quote or --seen (one of the two)")
		}
	default:
		return usageError(stderr, "memory takes show <name>, propose …, or withdraw <id>")
	}
	c, err := NewClient(env)
	if err != nil {
		return finish(err, stderr)
	}
	switch sub {
	case "propose":
		a, raw, err := ProposeMemory(c, p)
		return finish(show(err, *asJSON, stdout, raw, func() error {
			text := fmt.Sprintf("Proposed %q (id %s). %s\n", oneLine(a.Name), oneLine(a.ID), oneLine(a.Said))
			if a.Held != "" {
				text += oneLine(a.Held) + "\n"
			}
			_, err := io.WriteString(stdout, markMemory(text))
			return err
		}), stderr)
	case "withdraw":
		raw, err := WithdrawMemory(c, plain[1])
		return finish(show(err, *asJSON, stdout, raw, func() error {
			_, err := io.WriteString(stdout, markMemory(fmt.Sprintf("Withdrawn: %s.\n", oneLine(plain[1]))))
			return err
		}), stderr)
	}
	name := ""
	if sub == "show" {
		name = plain[1]
	}
	r, raw, err := Memory(c, name)
	return finish(show(err, *asJSON, stdout, raw, func() error { return RenderMemory(stdout, r, name != "") }), stderr)
}
