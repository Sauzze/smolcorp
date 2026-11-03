package gm

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"text/tabwriter"
	"time"
)

// idMost is the longest message id the app takes; ids use letters, digits, - _ and . only.
const idMost = 64

// validSendID says whether the app would take id.
func validSendID(id string) bool {
	if id == "" || len(id) > idMost {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// newSendID makes an id for a message sent without one: the time and some luck.
func newSendID(now time.Time) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return fmt.Sprintf("cli-%d-%s", now.UnixMilli(), hex.EncodeToString(b))
}

// SendReply is the answer of POST /setup/gm/send.
type SendReply struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	To        string `json:"to"`
	Size      int    `json:"size"`
	Duplicate bool   `json:"duplicate"`
	Note      string `json:"note"`
}

// Send sends text to a hire (or, from a hire, to the GM), under id.
func Send(c *Client, to, text, id string) (SendReply, []byte, error) {
	raw, err := c.Post("/setup/gm/send", nil, map[string]string{"to": to, "text": text, "id": id})
	if err != nil {
		return SendReply{}, nil, err
	}
	var r SendReply
	if err := json.Unmarshal(raw, &r); err != nil {
		return SendReply{}, nil, decodeError(err)
	}
	return r, raw, nil
}

// RenderSend says what became of a message.
func RenderSend(w io.Writer, r SendReply, to string) error {
	line := fmt.Sprintf("%s to %s: message %s, %d bytes", orDash(oneLine(r.State)), oneLine(to), oneLine(r.ID), r.Size)
	if r.Duplicate {
		line += " (sent before under this id; not sent again)"
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return err
	}
	if n := oneLine(r.Note); n != "" {
		_, err := fmt.Fprintln(w, n)
		return err
	}
	return nil
}

// Message is one line of the outbox (GET /setup/gm/outbox).
type Message struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	FromName string `json:"fromName"`
	To       string `json:"to"`
	ToName   string `json:"toName"`
	At       Millis `json:"at"`
	State    string `json:"state"`
	Size     int    `json:"size"`
	Attempts int    `json:"attempts"`
	Note     string `json:"note"`
	Text     string `json:"text"`
}

// Outbox asks for the messages sent after since (a time in milliseconds; 0
// for all). The app has no "since" of its own, so the answer is cut here: raw
// is the app's answer with only the later messages left, for --json.
func Outbox(c *Client, since int64) ([]Message, []byte, error) {
	raw, err := c.Get("/setup/gm/outbox", nil, requestTimeout)
	if err != nil {
		return nil, nil, err
	}
	var list struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, nil, decodeError(err)
	}
	var shown []Message
	var kept []json.RawMessage
	for _, item := range list.Messages {
		var m Message
		if err := json.Unmarshal(item, &m); err != nil {
			return nil, nil, decodeError(err)
		}
		if int64(m.At) > since {
			shown = append(shown, m)
			kept = append(kept, item)
		}
	}
	if since > 0 {
		if raw, err = replaceList(raw, "messages", kept); err != nil {
			return nil, nil, err
		}
	}
	return shown, raw, nil
}

// replaceList is raw with the array under key swapped for items, all else as served.
func replaceList(raw []byte, key string, items []json.RawMessage) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, decodeError(err)
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	m[key], _ = json.Marshal(items)
	out, err := json.Marshal(m)
	if err != nil {
		return nil, decodeError(err)
	}
	return out, nil
}

// RenderOutbox prints the messages as a table, oldest first, times in loc.
func RenderOutbox(w io.Writer, msgs []Message, loc *time.Location) error {
	if len(msgs) == 0 {
		_, err := fmt.Fprintln(w, "No messages.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tFROM\tTO\tAT\tSTATE\tSIZE\tTRIES\tTEXT\tNOTE")
	for _, m := range msgs {
		from, to := oneLine(m.From), oneLine(m.ToName)
		if from == "hire" && oneLine(m.FromName) != "" {
			from = oneLine(m.FromName)
		}
		if to == "" {
			to = oneLine(m.To)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\n",
			oneLine(m.ID), orDash(from), orDash(to), time.UnixMilli(int64(m.At)).In(loc).Format("01-02 15:04:05"),
			orDash(oneLine(m.State)), m.Size, m.Attempts, orDash(truncate(oneLine(m.Text), 40)), truncate(oneLine(m.Note), 60))
	}
	return tw.Flush()
}

// Watch is one watch (GET /setup/gm/watches). Built-ins are the ones the app
// set for itself; they carry a builtin name.
type Watch struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Target    string  `json:"target"`
	Pattern   string  `json:"pattern"`
	Repeat    bool    `json:"repeat"`
	By        string  `json:"by"`
	CreatedAt Millis  `json:"createdAt"`
	Fired     int     `json:"fired"`
	Active    bool    `json:"active"`
	Minutes   float64 `json:"minutes"`
	Due       Millis  `json:"due"`
	Builtin   string  `json:"builtin"`
}

// WatchSpec is what `watch` asks the app to keep an eye on.
type WatchSpec struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Pattern string `json:"pattern,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
	Repeat  bool   `json:"repeat,omitempty"`
}

// SetWatch asks the app to set a watch; it answers the watch as the app made it.
func SetWatch(c *Client, spec WatchSpec) (Watch, []byte, error) {
	raw, err := c.Post("/setup/gm/watch", nil, spec)
	if err != nil {
		return Watch{}, nil, err
	}
	var w Watch
	if err := json.Unmarshal(raw, &w); err != nil {
		return Watch{}, nil, decodeError(err)
	}
	return w, raw, nil
}

// Unwatch takes a watch away.
func Unwatch(c *Client, id string) ([]byte, error) {
	return c.Post("/setup/gm/unwatch", url.Values{"id": {id}}, nil)
}

// Watches lists the watches; the app's own built-ins are left out unless all.
// raw is the answer with them left out the same way, for --json.
func Watches(c *Client, all bool) ([]Watch, []byte, error) {
	raw, err := c.Get("/setup/gm/watches", nil, requestTimeout)
	if err != nil {
		return nil, nil, err
	}
	var list struct {
		Watches []json.RawMessage `json:"watches"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, nil, decodeError(err)
	}
	var shown []Watch
	var kept []json.RawMessage
	for _, item := range list.Watches {
		var w Watch
		if err := json.Unmarshal(item, &w); err != nil {
			return nil, nil, decodeError(err)
		}
		if w.Builtin == "" || all {
			shown = append(shown, w)
			kept = append(kept, item)
		}
	}
	if !all {
		if raw, err = replaceList(raw, "watches", kept); err != nil {
			return nil, nil, err
		}
	}
	return shown, raw, nil
}

// RenderWatches prints the watches as a table.
func RenderWatches(w io.Writer, list []Watch, loc *time.Location) error {
	if len(list) == 0 {
		_, err := fmt.Fprintln(w, "No watches.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tKIND\tTARGET\tPATTERN\tREPEAT\tBY\tFIRED\tACTIVE\tNEXT")
	for _, x := range list {
		by := oneLine(x.By)
		if x.Builtin != "" {
			by = "app: " + oneLine(x.Builtin)
		}
		next := ""
		switch {
		case !x.Active:
			// A watch that fired and is over has no next time, whatever minutes it was set to.
		case x.Kind == "time" && x.Due > 0:
			next = "due " + time.UnixMilli(int64(x.Due)).In(loc).Format("15:04:05")
		case x.Minutes > 0:
			next = strconv.FormatFloat(x.Minutes, 'f', -1, 64) + " min"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			oneLine(x.ID), orDash(oneLine(x.Kind)), orDash(truncate(oneLine(x.Target), 50)), orDash(truncate(oneLine(x.Pattern), 30)),
			yesNoWord(x.Repeat), orDash(by), x.Fired, yesNoWord(x.Active), next)
	}
	return tw.Flush()
}

func yesNoWord(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// Contact is a pair of hires (ids) of whom the first may write to the second.
type Contact struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// SetContacts allows (or, with remove, takes away) a pair; it answers the pairs now.
func SetContacts(c *Client, from, to string, oneWay, remove bool) ([]Contact, []byte, error) {
	body := map[string]any{"from": from, "to": to}
	if oneWay {
		body["oneWay"] = true
	}
	if remove {
		body["remove"] = true
	}
	raw, err := c.Post("/setup/gm/contacts", nil, body)
	if err != nil {
		return nil, nil, err
	}
	var r struct {
		Contacts []Contact `json:"contacts"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, nil, decodeError(err)
	}
	return r.Contacts, raw, nil
}

// nameOf is the display name of the team member with this id, else the id.
func nameOf(team []Member, id string) string {
	for _, m := range team {
		if m.ID == id {
			return m.displayName()
		}
	}
	return oneLine(id)
}

// RenderContacts prints who may write to whom, one line per pair, by name.
func RenderContacts(w io.Writer, pairs []Contact, team []Member) error {
	if len(pairs) == 0 {
		_, err := fmt.Fprintln(w, "No contacts: a hire may write only to the GM.")
		return err
	}
	for _, p := range pairs {
		if _, err := fmt.Fprintf(w, "%s -> %s\n", nameOf(team, p.From), nameOf(team, p.To)); err != nil {
			return err
		}
	}
	return nil
}

// readMessageFile reads the text of --file, at most the app's whole-body limit.
func readMessageFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", &Error{Code: exitUsage, Msg: fmt.Sprintf("can't read --file: %v", unwrapPathError(err))}
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBody+1))
	if err != nil {
		return "", &Error{Code: exitUsage, Msg: fmt.Sprintf("can't read --file: %v", unwrapPathError(err))}
	}
	if len(data) > maxBody {
		return "", &Error{Code: exitUsage, Msg: "--file is too big to send."}
	}
	return string(data), nil
}

func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
