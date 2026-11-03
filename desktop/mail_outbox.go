package desktop

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The mail outbox: a port of the Mac's MailRouter store (Sources/AgentDeck/
// MailRouter.swift). A library only: no routes, no tmux, no timers. The
// routes (T727) call add and list; the pump and the typing (T728) call due,
// start and the attempt's commit and finish. Nothing here types anything.
//
// Every accepted message is written to outbox.json before anything is
// delivered. The same id twice is the same message: answered again, never
// delivered again. Nothing is ever re-sent by itself.

// mailSender is who a message is from.
type mailSender string

const (
	mailFromGM    mailSender = "gm"
	mailFromOwner mailSender = "owner"
	mailFromHire  mailSender = "hire"
)

// line is the words of the sender line for a gm or the owner (MailRouter.swift ~28-34).
func (s mailSender) line() string {
	switch s {
	case mailFromGM:
		return "the GM"
	case mailFromOwner:
		return "the owner"
	}
	return "a hire"
}

// mailState is where a message is. queued is the only one that is ever tried again.
type mailState string

const (
	mailQueued    mailState = "queued"
	mailDelivered mailState = "delivered"
	mailLeft      mailState = "left"
	mailFailed    mailState = "failed"
)

const (
	mailFileVersion      = 1
	mailDefaultMaxBytes  = 16 * 1024
	mailDefaultPerHour   = 200
	mailDefaultWaitHours = 24.0
	// mailRetryEvery is how often a message with real trouble is tried, and mailMaxAttempts how many tries before it is left (~102-103).
	mailRetryEvery   = 30 * time.Second
	mailMaxAttempts  = 6
	mailKeepMost     = 2000
	mailLongestID    = 64
	mailDefaultShown = 100
	mailGMNote       = "Kept for the GM to read; not typed into its session."
)

// mailMessage is one message in outbox.json, with the Mac's field names (~47-69).
type mailMessage struct {
	ID       string     `json:"id"`
	From     mailSender `json:"from"`
	To       string     `json:"to"`
	ToName   string     `json:"toName"`
	FromID   string     `json:"fromID,omitempty"`
	FromName string     `json:"fromName,omitempty"`
	Text     string     `json:"text"`
	// At is milliseconds since 1970.
	At    float64   `json:"at"`
	State mailState `json:"state"`
	// Size is the text's size in bytes.
	Size int `json:"size"`
	// Attempts is counted and saved before each try, so a crash can't make a second one for free.
	Attempts int `json:"attempts"`
	// NextAt is when delivery is tried next (milliseconds); 0 once it isn't.
	NextAt float64 `json:"nextAt"`
	// Note is why it is left or failed, in words for the GM.
	Note string `json:"note"`
}

// fromLine is who the first line says it is from. A hire is always "hire <name> [<id>]",
// so a nickname of "the GM" can't make its mail look like the GM's (~73-76).
func (m mailMessage) fromLine() string {
	if m.From != mailFromHire {
		return m.From.line()
	}
	name, id := m.FromName, m.FromID
	if name == "" {
		name = "?"
	}
	if id == "" {
		id = "?"
	}
	return fmt.Sprintf("hire %s [%s]", name, id)
}

// mailWire is the text a hire is given: the sender line, a blank line, the message (~321-323).
func mailWire(m mailMessage) string {
	return fmt.Sprintf("— from %s (Smolcorp), message %s\n\n%s", m.fromLine(), m.ID, m.Text)
}

// mailLimits are the settings mailMaxBytes, mailPerHour and mailWaitHours.
type mailLimits struct {
	MaxBytes  int
	PerHour   int
	WaitHours float64
}

// mailLimitsOf reads the limits from the config. A value that isn't set (under 1) is the default.
func mailLimitsOf(c *Config) mailLimits {
	l := mailLimits{MaxBytes: c.MailMaxBytes, PerHour: c.MailPerHour, WaitHours: c.MailWaitHours}
	if l.MaxBytes < 1 {
		l.MaxBytes = mailDefaultMaxBytes
	}
	if l.PerHour < 1 {
		l.PerHour = mailDefaultPerHour
	}
	if l.WaitHours < 1 {
		l.WaitHours = mailDefaultWaitHours
	}
	return l
}

// mailError is a refusal the routes map to a status (400/403/409/413/429/500).
type mailError struct {
	Status  int
	Message string
}

func (e *mailError) Error() string { return e.Message }

// mailDest is where a message is for, once the routes have found it.
type mailDest struct {
	// ID and Name: the hire's id and what it is called, or "bot:<id>".
	ID, Name string
	// GM: a hire writing to the GM. Kept for the GM to read, never delivered.
	GM bool
}

// mailRequest is one send, after the route has found the caller. The route finds
// the sender from the key, never from what the body says.
type mailRequest struct {
	From mailSender
	// SignerID and SignerName: the hire a message from a hire is from.
	SignerID, SignerName string
	// To is who it is for as asked; blank is a bad request. Resolve finds where it goes (it runs after the
	// id, size and rate checks, as on the Mac. It runs with the outbox unlocked, so it may take the Deck's lock
	// or call the outbox; the checks are made again afterwards).
	To      string
	Text    string
	ID      string
	IDGiven bool
	Resolve func() (mailDest, *mailError)
}

// mailAdded is what add came to: the answer to send, and what the route and the pump need.
type mailAdded struct {
	// Answer is the body of POST /setup/gm/send.
	Answer map[string]any
	// Message is the message as kept.
	Message mailMessage
	// Duplicate: the same id again; nothing was added and nothing is to be delivered.
	Duplicate bool
	// ToGM: a new hire message to the GM (the route wakes the GM's inbox; nothing is to be delivered).
	ToGM bool
	// Deliver: a new queued message to try now.
	Deliver bool
}

// mailOutbox is the store. It has its own mutex (not the Deck's): routes and one pump goroutine use it.
type mailOutbox struct {
	// The outbox never takes another lock while it holds mu: the limits func and a request's Resolve
	// (which take the Deck's or the config's lock) are always called with mu released.
	mu       sync.Mutex
	file     string
	messages []mailMessage
	limits   func() mailLimits
	now      func() time.Time
	inFlight map[string]bool
}

// newMailOutbox opens the outbox at file. A file it can't read is moved to outbox.broken.json and a new one started (~545-548).
func newMailOutbox(file string, limits func() mailLimits, now func() time.Time) *mailOutbox {
	if now == nil {
		now = time.Now
	}
	o := &mailOutbox{file: file, limits: limits, now: now, inFlight: map[string]bool{}}
	o.messages = loadMailOutbox(file)
	return o
}

func loadMailOutbox(file string) []mailMessage {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var found struct {
		Version  int           `json:"version"`
		Messages []mailMessage `json:"messages"`
	}
	ok := json.Unmarshal(data, &found) == nil && found.Messages != nil
	for _, m := range found.Messages {
		switch m.State {
		case mailQueued, mailDelivered, mailLeft, mailFailed:
		default:
			ok = false
		}
		switch m.From {
		case mailFromGM, mailFromOwner, mailFromHire:
		default:
			ok = false
		}
	}
	if !ok {
		// Set aside, so the next save doesn't wipe what might be mended.
		_ = os.Rename(file, filepath.Join(filepath.Dir(file), "outbox.broken.json"))
		return nil
	}
	return found.Messages
}

// save writes the file privately and atomically, keys sorted like the Mac's (~533-540), and keeps the latest 2000.
func (o *mailOutbox) save() bool {
	if len(o.messages) > mailKeepMost {
		o.messages = append([]mailMessage(nil), o.messages[len(o.messages)-mailKeepMost:]...)
	}
	raw, err := json.Marshal(struct {
		Version  int           `json:"version"`
		Messages []mailMessage `json:"messages"`
	}{mailFileVersion, o.messages})
	if err != nil {
		return false
	}
	// Through a generic value, whose keys come out sorted.
	var generic any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&generic) != nil {
		return false
	}
	data, err := json.MarshalIndent(generic, "", "  ")
	if err != nil {
		return false
	}
	return writePrivate(o.file, data) == nil
}

func (o *mailOutbox) nowMs() float64 { return float64(o.now().UnixMilli()) }

// mailPlainID is whether id may be used: letters, digits, - _ and . only, up to 64 (~503).
func mailPlainID(id string) bool {
	if id == "" || len(id) > mailLongestID {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func newMailID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "m-" + hex.EncodeToString(b)
}

// mailAnswer is what POST /setup/gm/send answers (~509-514).
func mailAnswer(m mailMessage, duplicate bool) map[string]any {
	out := map[string]any{"id": m.ID, "state": string(m.State), "to": m.To, "size": m.Size}
	if duplicate {
		out["duplicate"] = true
	}
	if m.Note != "" {
		out["note"] = m.Note
	}
	return out
}

// add is the logic of send (~161-236). The message is saved before anything is delivered; if it can't
// be saved, nothing is stored and the error is a 500.
func (o *mailOutbox) add(req mailRequest) (mailAdded, *mailError) {
	// The limits are read before the outbox lock is taken (see mailOutbox).
	limit := o.limits()
	if req.From == mailFromHire && req.SignerID == "" {
		return mailAdded{}, &mailError{403, "This message isn't from a hire Smolcorp started: a hire sends with the key in its own environment."}
	}
	if trimBlank(req.To) == "" || trim(req.Text) == "" {
		return mailAdded{}, &mailError{400, "Send {to, text, id}: who it is for, what to say, and an id of your own so that sending twice delivers once."}
	}
	id := req.ID
	if req.IDGiven {
		if !mailPlainID(id) {
			return mailAdded{}, &mailError{400, fmt.Sprintf("The id may use letters, digits, - _ and ., up to %d of them.", mailLongestID)}
		}
	} else {
		id = newMailID()
	}

	// First look, under the lock: the id, the size and the rate.
	o.mu.Lock()
	if again, refusal := o.vet(req, id, limit); again != nil || refusal != nil {
		o.mu.Unlock()
		if refusal != nil {
			return mailAdded{}, refusal
		}
		return *again, nil
	}
	o.mu.Unlock()

	// The target is found with the lock released: finding it takes the Deck's lock, and the outbox
	// never takes another lock while it holds its own.
	dest := mailDest{ID: req.To, Name: req.To}
	if req.Resolve != nil {
		found, refusal := req.Resolve()
		if refusal != nil {
			return mailAdded{}, refusal
		}
		dest = found
	}

	// Second look: another send may have taken the id, or used up the hour, in the meantime.
	o.mu.Lock()
	defer o.mu.Unlock()
	if again, refusal := o.vet(req, id, limit); again != nil || refusal != nil {
		if refusal != nil {
			return mailAdded{}, refusal
		}
		return *again, nil
	}
	nowMs := o.nowMs()
	message := mailMessage{ID: id, From: req.From, To: dest.ID, ToName: dest.Name, Text: req.Text, At: nowMs, State: mailQueued, Size: len(req.Text)}
	if req.From == mailFromHire {
		message.FromID, message.FromName = req.SignerID, req.SignerName
	}
	if req.From == mailFromHire && dest.GM {
		// Kept for the GM to read; never typed into its session (~255-267).
		message.To, message.ToName = "gm", "the GM"
		message.State, message.Note = mailDelivered, mailGMNote
	}
	// Written down before anything is delivered; if it can't be, nothing is sent (~226-230).
	o.messages = append(o.messages, message)
	if !o.save() {
		o.messages = o.messages[:len(o.messages)-1]
		return mailAdded{}, &mailError{500, "The outbox couldn't be written, so nothing was sent."}
	}
	return mailAdded{Answer: mailAnswer(message, false), Message: message,
		ToGM: message.State == mailDelivered, Deliver: message.State == mailQueued}, nil
}

// vet is the checks add makes under the lock, in the Mac's order (~178-195): the same id (answered
// again, or a hire's 409), the size, then the hour's rate. It answers the stored message, or a
// refusal, or neither when the message may go on. o.mu must be held.
func (o *mailOutbox) vet(req mailRequest, id string, limit mailLimits) (*mailAdded, *mailError) {
	// The same id is the same message, whatever became of it: answered again, never sent again (~178-182).
	for _, seen := range o.messages {
		if seen.ID != id {
			continue
		}
		// Another sender's id isn't theirs to ask about.
		if req.From == mailFromHire && (seen.From != mailFromHire || seen.FromID != req.SignerID) {
			return nil, &mailError{409, "That id is already used. Pick another."}
		}
		return &mailAdded{Answer: mailAnswer(seen, true), Message: seen, Duplicate: true}, nil
	}
	size := len(req.Text)
	if size > limit.MaxBytes {
		return nil, &mailError{413, fmt.Sprintf("That message is %d bytes. The limit is %d bytes per message (setting mailMaxBytes).", size, limit.MaxBytes)}
	}
	nowMs := o.nowMs()
	// A hire's limit is its own, not the hires' together (~191-195).
	lastHour := 0
	for _, m := range o.messages {
		if m.From == req.From && m.FromID == req.SignerID && m.At > nowMs-3_600_000 {
			lastHour++
		}
	}
	if lastHour >= limit.PerHour {
		who := req.From.line()
		if req.From == mailFromHire {
			who = "hire " + req.SignerName
		}
		return nil, &mailError{429, fmt.Sprintf("That is %d messages from %s in the last hour, the limit (setting mailPerHour). Try again later.", limit.PerHour, who)}
	}
	return nil, nil
}

// trimBlank cuts spaces and tabs only, as the Mac does for "to" (.whitespace, ~164).
func trimBlank(s string) string { return strings.Trim(s, " \t") }

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// settleMail records what a try came to (a mailResult from mailType) in m (~371-438). counted is whether the attempt was already
// counted and saved by commit. Nothing is ever sent again by itself: only queued messages are tried.
func settleMail(m *mailMessage, r mailResult, currentMs float64, waitHours float64, counted bool) {
	switch r.Outcome {
	case mailSent:
		// Sent with words is the Mac's deliveredAs: delivered, but not quite as written. Without, it is delivered.
		m.State, m.NextAt, m.Note = mailDelivered, 0, r.Note
	case mailCannot:
		m.State, m.NextAt, m.Note = mailFailed, 0, r.Note
	case mailWaiting:
		// Nothing was typed and nothing counted: waiting is not an attempt.
		if currentMs-m.At >= waitHours*3_600_000 {
			m.State, m.NextAt = mailLeft, 0
			m.Note = fmt.Sprintf("Left after waiting %g hours for %s to be free, not sent again: %s", waitHours, m.ToName, r.Note)
		} else {
			m.NextAt = 0 // looked at again on the next pump
			m.Note = r.Note
		}
	case mailTrouble:
		// A failure before anything was typed was not counted by commit; it counts now.
		if !counted {
			m.Attempts++
		}
		if m.Attempts >= mailMaxAttempts {
			m.State, m.NextAt = mailLeft, 0
			m.Note = fmt.Sprintf("Left after %d tries, not sent again: %s", m.Attempts, r.Note)
		} else {
			m.NextAt = currentMs + float64(mailRetryEvery.Milliseconds())
			m.Note = r.Note
		}
	}
}

// due lists the queued messages that are ready to try, in outbox order, leaving out any whose
// delivery is out (one in flight per id). A waiting message has nextAt 0, so it is due on every call.
func (o *mailOutbox) due() []mailMessage {
	o.mu.Lock()
	defer o.mu.Unlock()
	current := o.nowMs()
	var out []mailMessage
	for _, m := range o.messages {
		if m.State == mailQueued && m.NextAt <= current && !o.inFlight[m.ID] {
			out = append(out, m)
		}
	}
	return out
}

// mailAttempt is one try at one message. The caller types only after Commit answers true.
type mailAttempt struct {
	o       *mailOutbox
	id      string
	Text    string
	counted bool
	over    bool
}

// start begins a try: nil if the message isn't queued, isn't there, or already has a try out.
// Call Finish exactly once when the try is over, whatever it came to.
func (o *mailOutbox) start(id string) *mailAttempt {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, m := range o.messages {
		if m.ID == id {
			if m.State != mailQueued || o.inFlight[id] {
				return nil
			}
			o.inFlight[id] = true
			return &mailAttempt{o: o, id: id, Text: mailWire(m)}
		}
	}
	return nil
}

// Commit counts the try and writes it down; call it just before the first thing that types (more than once
// is fine: it counts once). If it answers false nothing may be typed: the count couldn't be saved (~402-415).
func (a *mailAttempt) Commit() bool {
	o := a.o
	o.mu.Lock()
	defer o.mu.Unlock()
	if a.over {
		return false
	}
	if a.counted {
		return true
	}
	for i := range o.messages {
		if o.messages[i].ID != a.id {
			continue
		}
		before := o.messages[i]
		o.messages[i].Attempts++
		o.messages[i].NextAt = o.nowMs() + float64(mailRetryEvery.Milliseconds())
		if !o.save() {
			o.messages[i] = before
			return false
		}
		a.counted = true
		return true
	}
	return false
}

// Finish records what the try came to, and frees the message for the next one. It answers whether the file was written.
func (a *mailAttempt) Finish(r mailResult) bool {
	o := a.o
	waitHours := o.limits().WaitHours // before the lock, like add
	o.mu.Lock()
	defer o.mu.Unlock()
	if a.over {
		return false
	}
	a.over = true
	delete(o.inFlight, a.id)
	for i := range o.messages {
		if o.messages[i].ID == a.id {
			settleMail(&o.messages[i], r, o.nowMs(), waitHours, a.counted)
			return o.save()
		}
	}
	return false
}

// fail ends a queued message that can never go (its hire is gone, ~440-470). A try still out is left to finish.
func (o *mailOutbox) fail(id, note string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := range o.messages {
		if o.messages[i].ID == id && o.messages[i].State == mailQueued && !o.inFlight[id] {
			o.messages[i].State, o.messages[i].NextAt, o.messages[i].Note = mailFailed, 0, note
			return o.save()
		}
	}
	return false
}

// failTo ends every queued message to `to` (a hire that was fired), each with "<name> is gone.",
// as the pump would on its next round. A try still out is left to finish. Says how many.
func (o *mailOutbox) failTo(to string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	failed := 0
	for i := range o.messages {
		m := &o.messages[i]
		if m.To == to && m.State == mailQueued && !o.inFlight[m.ID] {
			m.State, m.NextAt, m.Note = mailFailed, 0, m.ToName+" is gone."
			failed++
		}
	}
	if failed > 0 {
		o.save()
	}
	return failed
}

// get is a copy of one message.
func (o *mailOutbox) get(id string) (mailMessage, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, m := range o.messages {
		if m.ID == id {
			return m, true
		}
	}
	return mailMessage{}, false
}

// list is the body of GET /setup/gm/outbox?limit=: the latest messages, oldest first, 100 by default (~517-528).
// includeText is false for the tool caller, which is not given the text.
//
// asked is nil when the caller gave none (then 100). A given limit is used as the Mac does,
// at least 1 and at most 2000 (outboxJSON(limit:), ~517-518).
func (o *mailOutbox) list(asked *int, includeText bool) map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	limit := mailDefaultShown
	if asked != nil {
		limit = *asked
	}
	limit = max(1, min(limit, mailKeepMost))
	shown := o.messages
	if len(shown) > limit {
		shown = shown[len(shown)-limit:]
	}
	out := make([]map[string]any, 0, len(shown))
	for _, m := range shown {
		row := map[string]any{"id": m.ID, "from": string(m.From), "to": m.To, "toName": m.ToName,
			"at": int64(math.Floor(m.At)), "state": string(m.State), "size": m.Size, "attempts": m.Attempts}
		if m.Note != "" {
			row["note"] = m.Note
		}
		if m.FromID != "" {
			row["fromID"] = m.FromID
		}
		if m.FromName != "" {
			row["fromName"] = m.FromName
		}
		if includeText {
			row["text"] = m.Text
		}
		out = append(out, row)
	}
	return map[string]any{"messages": out}
}
