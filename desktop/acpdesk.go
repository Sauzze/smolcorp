package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Sauzze/smolcorp/internal/acp"
)

// An ACP desk (ACP1): an agent that speaks the Agent Client Protocol, held by the core through a client of its own
// (internal/acp), with no terminal and no hook. What it does becomes the deck's events in Claude Code's words
// (Agent.canonical), so one state machine in Sessions serves it as every agent, and what is said goes in a
// transcript in Claude Code's shape (acptranscript.go).
//
// This is the session's plumbing only. Nothing starts an ACP desk yet: hiring one (ACP5) starts the agent, through
// the sandbox's runner, and hands its client in; typing into it is its Tab's (ACP2); a permission ask is answered
// by the owner's tap alone (ACP3). The desk answers no file or terminal request (N4): it is neither an
// acp.FileHandler nor an acp.TerminalHandler, so the agent is told the client can't, and asking gets
// "method not found".
//
// The client calls Update on the one goroutine that reads the agent, and nothing the agent sends is read while it
// runs: Update only puts the update in the desk's queue and returns. The desk's worker takes them from there, in
// order, writes the transcript and raises the events, taking the deck's lock as it goes.
//
// The deck knows the desk's session by the desk's tab, an acp: name the core chose (Sessions.ApplyACP), never by the
// session id the agent answered session/new with: that one is the agent's choice, so it is kept for the wire (what
// the desk sends, and which of the agent's messages it takes) and the transcript's lines alone. An agent that names
// another desk's session reaches only its own; and no hook event takes an acp: id or tab (Sessions.Apply).

// acpConn is what a desk needs of its agent's client: an *acp.Client, or a stand-in in tests.
type acpConn interface {
	Initialize(ctx context.Context) (*acp.InitializeResponse, error)
	NewSession(ctx context.Context, cwd string) (string, error)
	Prompt(ctx context.Context, sessionID string, prompt []acp.ContentBlock) (acp.StopReason, error)
	Cancel(ctx context.Context, sessionID string) error
	Close() error
	Done() <-chan struct{}
}

var _ acpConn = (*acp.Client)(nil)

// acpDeskPrefix is how an ACP desk's tab, and so its session's id on the deck, starts: the rest is the core's own
// random name for it. No hook's session id or tab looks like this.
const acpDeskPrefix = "acp:"

const (
	// Updates waiting for the worker, at most, by count and by size: past either, an update is left out (and
	// counted) rather than holding up the agent or filling the memory. The size counts the batch the worker is on
	// too, until it is done with it. The desk's own items always go in.
	acpQueueMost      = 4096
	acpQueueBytesMost = 32 << 20
	// Tool calls under way whose kind the worker remembers, at most: a call's later updates seldom say it again.
	acpKindsMost = 1024
)

var (
	errACPNotOpen    = errors.New("the ACP desk has no session open")
	errACPBusy       = errors.New("the ACP desk is already in a turn")
	errACPNotItsTurn = errors.New("an ask outside the desk's own session and turn")
)

// acpItem is one thing for the worker, in the order it happened.
type acpItem struct {
	kind    string // the event's name, as acpCanonical reads it
	at      time.Time
	update  acp.SessionUpdate // session/update
	ask     acp.ToolCall      // session/request_permission
	text    string            // session/prompt: what was sent
	reason  acp.StopReason    // session/prompt_done
	err     error             // session/prompt_done
	opened  *acpTranscript    // session/new
	reached chan struct{}     // "": closed once the worker gets here
}

// acpDesk is one ACP agent's desk: its client, its session and its transcript.
type acpDesk struct {
	deck   *Deck
	agent  Agent
	tab    string // acp:<id>, the core's name for it: its session's tab and id on the deck, no terminal's
	folder string
	conn   acpConn

	mu         sync.Mutex
	session    string // the agent's own id for its session: for the wire and the transcript only
	turn       bool   // a prompt is out, as the caller sees it
	ended      bool
	queue      []acpItem
	queueBytes int // the queue's updates and the batch the worker is on, by size
	dropped    int // updates left out for want of room
	wake       chan struct{}
	done       chan struct{} // the worker has stopped
}

// newACPDesk is a desk for agent in folder (absolute), on conn, its worker started. open it next; close it always.
func newACPDesk(d *Deck, agent Agent, folder string, conn acpConn) *acpDesk {
	desk := &acpDesk{deck: d, agent: agent, tab: acpDeskPrefix + randomHex(8), folder: folder, conn: conn,
		wake: make(chan struct{}, 1), done: make(chan struct{})}
	go desk.work()
	return desk
}

// open agrees the protocol with the agent and starts its session in the desk's folder, with a new transcript.
func (desk *acpDesk) open(ctx context.Context) error {
	if _, err := desk.conn.Initialize(ctx); err != nil {
		return err
	}
	session, err := desk.conn.NewSession(ctx, desk.folder)
	if err != nil {
		return err
	}
	transcript, err := newACPTranscript(supportDir, session)
	if err != nil {
		return err
	}
	desk.mu.Lock()
	if desk.ended {
		desk.mu.Unlock()
		transcript.close(time.Now())
		return errACPNotOpen
	}
	desk.session = session
	desk.push(acpItem{kind: "session/new", opened: transcript})
	desk.mu.Unlock()
	go func() {
		<-desk.conn.Done() // the agent stopped, or was closed: the session is over
		desk.end()
	}()
	return nil
}

// prompt sends text as a turn and waits for its end, one turn at a time.
func (desk *acpDesk) prompt(ctx context.Context, text string) (acp.StopReason, error) {
	desk.mu.Lock()
	session := desk.session
	switch {
	case session == "" || desk.ended:
		desk.mu.Unlock()
		return "", errACPNotOpen
	case desk.turn:
		desk.mu.Unlock()
		return "", errACPBusy
	}
	desk.turn = true
	desk.push(acpItem{kind: "session/prompt", text: text})
	desk.mu.Unlock()
	reason, err := desk.conn.Prompt(ctx, session, []acp.ContentBlock{acp.Text(text)})
	desk.mu.Lock()
	desk.turn = false
	desk.push(acpItem{kind: "session/prompt_done", reason: reason, err: err})
	desk.mu.Unlock()
	return reason, err
}

// cancel stops the turn under way, at the agent: its prompt then ends, "cancelled".
func (desk *acpDesk) cancel(ctx context.Context) error {
	desk.mu.Lock()
	session := desk.session
	desk.mu.Unlock()
	if session == "" {
		return errACPNotOpen
	}
	return desk.conn.Cancel(ctx, session)
}

// close ends the agent and the session, and waits for the worker to have written and said everything.
func (desk *acpDesk) close() error {
	err := desk.conn.Close()
	desk.end()
	<-desk.done
	return err
}

// end puts the session's end in the queue, once: nothing from the agent is taken after it.
func (desk *acpDesk) end() {
	desk.mu.Lock()
	defer desk.mu.Unlock()
	if desk.ended {
		return
	}
	desk.ended = true
	desk.push(acpItem{kind: "session/closed"})
}

// Update is the client's: an update of the agent's, put in the queue for the worker. It never waits on anything but
// the desk's own short lock. One for another session than the desk's is dropped.
func (desk *acpDesk) Update(n acp.SessionNotification) {
	desk.mu.Lock()
	defer desk.mu.Unlock()
	if desk.ended || n.SessionID == "" || n.SessionID != desk.session {
		return
	}
	if len(desk.queue) >= acpQueueMost || desk.queueBytes+len(n.Update.Raw) > acpQueueBytesMost {
		desk.dropped++
		return
	}
	desk.queueBytes += len(n.Update.Raw)
	desk.push(acpItem{kind: "session/update", update: n.Update})
}

// RequestPermission is the client's: an ask of the agent's. Only one for the desk's own session, while its turn is
// out, is shown, as the session asking; any other is refused at once (RT126A1 N3). Nothing here answers it: only
// the owner's tap does (ACP3, R-TAP, R-ASK), and until that is built it waits for the turn to end or be cancelled,
// and the agent is then told "cancelled", never a pick.
func (desk *acpDesk) RequestPermission(ctx context.Context, req acp.PermissionRequest) (acp.PermissionOutcome, error) {
	desk.mu.Lock()
	ours := !desk.ended && desk.turn && req.SessionID != "" && req.SessionID == desk.session
	if ours {
		desk.push(acpItem{kind: "session/request_permission", ask: req.ToolCall})
	}
	desk.mu.Unlock()
	if !ours {
		return acp.PermissionOutcome{}, errACPNotItsTurn
	}
	<-ctx.Done()
	return acp.PermissionOutcome{}, ctx.Err()
}

// settle waits until everything queued before it has been written and said, and its room freed, or the worker has
// stopped.
func (desk *acpDesk) settle() {
	reached := make(chan struct{})
	desk.mu.Lock()
	desk.push(acpItem{reached: reached})
	desk.mu.Unlock()
	select {
	case <-reached:
	case <-desk.done:
	}
}

// push puts item in the queue, stamped now, and wakes the worker. With desk.mu held.
func (desk *acpDesk) push(item acpItem) {
	item.at = time.Now()
	desk.queue = append(desk.queue, item)
	select {
	case desk.wake <- struct{}{}:
	default:
	}
}

// work takes the queue in order until the session's end: it writes the transcript and raises the events. A turn is
// open from its prompt to its end, in the queue's order; an update that comes after its turn ended (the client lets
// a cancelled turn's stragglers through, see acp.Client.Prompt) still goes in the transcript, but changes nothing on
// the deck: the session stays as its turn's end left it.
func (desk *acpDesk) work() {
	defer close(desk.done)
	w := &acpWork{kinds: map[string]string{}}
	for range desk.wake {
		desk.mu.Lock()
		items := desk.queue
		desk.queue = nil
		desk.mu.Unlock()
		if desk.take(w, items) {
			return
		}
	}
}

// acpWork is what the worker keeps from one batch to the next.
type acpWork struct {
	transcript *acpTranscript
	turnOpen   bool
	kinds      map[string]string // the tool calls under way, by id: their kinds
}

// take does a batch of the worker's, in order, freeing its room in the queue as it goes: all of it at the end, and
// what is done so far at each settle. It says whether the session ended.
func (desk *acpDesk) take(w *acpWork, items []acpItem) bool {
	size := 0 // the room of the items done, not freed yet
	free := func() {
		desk.mu.Lock()
		desk.queueBytes -= size
		desk.mu.Unlock()
		size = 0
	}
	defer free()
	for _, item := range items {
		if item.reached != nil {
			free()
			close(item.reached)
			continue
		}
		size += len(item.update.Raw)
		event := map[string]any{"hook_event_name": item.kind}
		switch item.kind {
		case "session/new":
			w.transcript = item.opened
		case "session/prompt":
			w.turnOpen = true
			w.transcript.user(item.text, item.at)
		case "session/prompt_done":
			w.turnOpen = false
			w.transcript.flush(item.at)
			event["stopReason"] = string(item.reason)
			if item.err != nil && !errors.Is(item.err, context.Canceled) {
				event["error"] = "acp"
			} else if item.err != nil {
				event["stopReason"] = string(acp.StopCancelled)
			}
		case "session/request_permission":
			w.transcript.flush(item.at)
			event["toolCallId"], event["kind"] = item.ask.ToolCallID, item.ask.Kind
			event["rawInput"] = inputOf(item.ask.RawInput)
		case "session/update":
			u := item.update
			u.ToolKind = acpKindOf(w.kinds, u)
			if !acpWrite(w.transcript, u, item.at) || !w.turnOpen {
				continue
			}
			event["sessionUpdate"], event["toolCallId"], event["kind"], event["status"] = u.Kind, u.ToolCallID, u.ToolKind, u.Status
			event["rawInput"] = inputOf(acpField(u.Raw, "rawInput"))
		case "session/closed":
			w.transcript.close(item.at)
		}
		desk.raise(event, w.transcript, item.at)
		if item.kind == "session/closed" {
			return true
		}
	}
	return false
}

// acpKindOf is a tool call's kind: the update's own, or else the one its call said before. A call's kind is
// remembered until it is done, for acpKindsMost calls at most.
func acpKindOf(kinds map[string]string, u acp.SessionUpdate) string {
	if u.Kind != "tool_call" && u.Kind != "tool_call_update" || u.ToolCallID == "" {
		return u.ToolKind
	}
	kind := u.ToolKind
	if kind == "" {
		kind = kinds[u.ToolCallID]
	}
	switch {
	case u.Status == "completed" || u.Status == "failed":
		delete(kinds, u.ToolCallID)
	case kind != "" && (len(kinds) < acpKindsMost || kinds[u.ToolCallID] != ""):
		kinds[u.ToolCallID] = kind
	}
	return kind
}

// raise gives the deck event, in Claude Code's words, as a hook's event is given.
func (desk *acpDesk) raise(event map[string]any, transcript *acpTranscript, at time.Time) {
	// The core's own name for the session, never the agent's: see Sessions.ApplyACP.
	event["sessionId"], event["cwd"] = desk.tab, desk.folder
	if transcript != nil {
		event["transcriptPath"] = transcript.path
	}
	claude := desk.agent.canonical(event)
	if claude == nil {
		return
	}
	d, ms := desk.deck, float64(at.UnixMilli())
	d.mu.Lock()
	defer d.mu.Unlock()
	changed := d.sessions.ApplyACP(claude, desk.agent, desk.tab, desk.folder, ms)
	d.logEvent(claude, desk.agent, ms)
	if changed {
		d.changed()
	}
}

// acpWrite writes an update into the transcript, and says whether it is a tool call's, the only kind the deck acts on.
func acpWrite(transcript *acpTranscript, u acp.SessionUpdate, at time.Time) bool {
	if transcript == nil {
		return false
	}
	switch u.Kind {
	case "agent_message_chunk":
		if u.Content != nil && u.Content.Type == "text" {
			transcript.said(u.Content.Text)
		}
		return false
	case "tool_call":
		transcript.toolUse(u.ToolCallID, acpToolName(u.ToolKind), acpField(u.Raw, "rawInput"), at)
	case "tool_call_update":
	default: // thoughts, plans, modes, commands, the user's own words replayed: left out
		return false
	}
	if u.Status == "completed" || u.Status == "failed" {
		transcript.toolResult(u.ToolCallID, acpResultText(u.Raw), u.Status == "failed", at)
	}
	return true
}

// acpField is one field of a raw update, or nil.
func acpField(raw json.RawMessage, name string) json.RawMessage {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil
	}
	return fields[name]
}

// acpResultText is a tool call's text output: its content's text blocks, one after another.
func acpResultText(raw json.RawMessage) string {
	var items []struct {
		Type    string `json:"type"`
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(acpField(raw, "content"), &items) != nil {
		return ""
	}
	var texts []string
	for _, item := range items {
		if item.Type == "content" && item.Content.Type == "text" {
			texts = append(texts, item.Content.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// inputOf is a tool call's raw input as an object, or nil when it isn't one.
func inputOf(raw json.RawMessage) any {
	var input map[string]any
	if len(raw) > acpToolTextMost || json.Unmarshal(raw, &input) != nil || input == nil {
		return nil
	}
	return input
}
