package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/acp"
)

// ACP1: an ACP desk's session, on a stand-in client: no agent is started, and nothing runs.

// standInACP is a stand-in for an agent's client. Its turn plays the agent's side of a prompt: it is called with the
// desk, as the client's reader would call it, and its updates reach the desk before the turn's answer, as they do.
type standInACP struct {
	desk    *acpDesk
	session string // the agent's id for its session: "s1" unless set
	turn    func(ctx context.Context, desk *acpDesk) (acp.StopReason, error)
	done    chan struct{}
	once    sync.Mutex
	closed  bool
	cancels []string
}

func (s *standInACP) Initialize(context.Context) (*acp.InitializeResponse, error) {
	return &acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersion}, nil
}
func (s *standInACP) NewSession(_ context.Context, cwd string) (string, error) {
	if s.session != "" {
		return s.session, nil
	}
	return "s1", nil
}
func (s *standInACP) Prompt(ctx context.Context, _ string, _ []acp.ContentBlock) (acp.StopReason, error) {
	if s.turn == nil {
		return "end_turn", nil
	}
	return s.turn(ctx, s.desk)
}
func (s *standInACP) Cancel(_ context.Context, session string) error {
	s.once.Lock()
	defer s.once.Unlock()
	s.cancels = append(s.cancels, session)
	return nil
}
func (s *standInACP) Close() error {
	s.once.Lock()
	defer s.once.Unlock()
	if !s.closed {
		s.closed = true
		close(s.done)
	}
	return nil
}
func (s *standInACP) Done() <-chan struct{} { return s.done }

// openACPDesk is a desk for the known agent gemini on a stand-in, its session open, closed at the end.
func openACPDesk(t *testing.T, d *Deck, turn func(context.Context, *acpDesk) (acp.StopReason, error)) (*acpDesk, *standInACP) {
	t.Helper()
	return openACPDeskOn(t, d, &standInACP{turn: turn, done: make(chan struct{})})
}

// openACPDeskOn is openACPDesk on conn. The desk's close at the end is given 5 seconds: a desk stuck on its own lock
// fails the test then, rather than hanging it until go test's timeout.
func openACPDeskOn(t *testing.T, d *Deck, conn *standInACP) (*acpDesk, *standInACP) {
	t.Helper()
	desk := newACPDesk(d, ACPAgent("gemini"), t.TempDir(), conn)
	conn.desk = desk
	t.Cleanup(func() {
		closed := make(chan struct{})
		go func() { desk.close(); close(closed) }()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
			t.Error("the desk never closed")
		}
	})
	if err := desk.open(context.Background()); err != nil {
		t.Fatal(err)
	}
	desk.settle()
	return desk, conn
}

// update is a session/update as the client reads it off the wire.
func update(session string, fields map[string]any) acp.SessionNotification {
	b, _ := json.Marshal(map[string]any{"sessionId": session, "update": fields})
	var n acp.SessionNotification
	if err := json.Unmarshal(b, &n); err != nil {
		panic(err)
	}
	return n
}

func chunk(text string) acp.SessionNotification {
	return update("s1", map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": text}})
}

func toolCall(id, status string) acp.SessionNotification {
	return update("s1", map[string]any{"sessionUpdate": "tool_call", "toolCallId": id, "title": "List the folder",
		"kind": "execute", "status": status, "rawInput": map[string]any{"command": "ls -la"}})
}

func toolDone(id, output string) acp.SessionNotification {
	return update("s1", map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "status": "completed",
		"content": []any{map[string]any{"type": "content", "content": map[string]any{"type": "text", "text": output}}}})
}

// sessionOf is the desk's session on the deck, as it is now (a copy), or nil.
func sessionOf(d *Deck, desk *acpDesk) *Session {
	return sessionByID(d, desk.tab)
}

// sessionByID is the deck's session id, as it is now (a copy), or nil.
func sessionByID(d *Deck, id string) *Session {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s := d.sessions.Get(id); s != nil {
		copied := *s
		return &copied
	}
	return nil
}

// A turn's updates become Claude Code's events: the desk's session starts idle, thinks through the turn with the
// tool the agent runs shown as Claude Code's, and ends unread; what was said is in its transcript, which the
// conversation reads as Claude Code's.
func TestAnACPTurnIsClaudeCodesEventsAndTranscript(t *testing.T) {
	d := newTestDeck(t)
	midTurn, carryOn := make(chan struct{}), make(chan struct{})
	desk, _ := openACPDesk(t, d, func(ctx context.Context, desk *acpDesk) (acp.StopReason, error) {
		desk.Update(chunk("Hello "))
		desk.Update(chunk("there"))
		desk.Update(toolCall("t1", "pending"))
		close(midTurn)
		<-carryOn
		desk.Update(toolDone("t1", "file.txt\nother.txt"))
		desk.Update(chunk("Done."))
		return "end_turn", nil
	})
	s := sessionOf(d, desk)
	if s == nil || s.Agent != ACPAgent("gemini") || s.State != Idle || s.ID != desk.tab || s.Tab != desk.tab || s.Transcript == "" {
		t.Fatalf("after open: %+v", s)
	}
	finished := make(chan error, 1)
	go func() { _, err := desk.prompt(context.Background(), "list the files"); finished <- err }()
	<-midTurn
	waitFor(t, "the tool shown", func() bool {
		s := sessionOf(d, desk)
		return s.State == Thinking && s.Tool == "Bash" && s.ToolUseID == "t1"
	})
	if _, err := desk.prompt(context.Background(), "again"); !errors.Is(err, errACPBusy) {
		t.Errorf("a second prompt during a turn: %v", err)
	}
	close(carryOn)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	desk.settle()
	if s := sessionOf(d, desk); s.State != Unread || s.Tool != "" {
		t.Errorf("after the turn: state %s, tool %q", s.State, s.Tool)
	}

	page, ok := readConversation(s.Transcript, nil, conversationPageMost)
	if !ok {
		t.Fatal("no conversation")
	}
	var got []string
	for _, item := range page.Items {
		got = append(got, item.Kind+":"+item.Text)
	}
	want := []string{"user:list the files", "agent:Hello there", "tool:" + toolLine(map[string]any{"name": "Bash",
		"input": map[string]any{"command": "ls -la"}}), "result:file.txt", "agent:Done."}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the conversation:\n got %q\nwant %q", got, want)
	}
	if runtime.GOOS != "windows" {
		file, _ := os.Stat(s.Transcript)
		folder, _ := os.Stat(filepath.Dir(s.Transcript))
		if file.Mode().Perm() != 0o600 || folder.Mode().Perm() != 0o700 {
			t.Errorf("transcript %v, folder %v", file.Mode(), folder.Mode())
		}
	}
	if filepath.Dir(s.Transcript) != filepath.Join(supportDir, acpTranscriptFolder) || strings.Contains(s.Transcript, "s1") {
		t.Errorf("the transcript is at %s: not the core's own name in the support folder", s.Transcript)
	}
}

// Updates that come after their turn ended (a cancelled turn's stragglers) go in the transcript, but change nothing
// on the deck: the session stays as its turn's end left it, never "working" again.
func TestLateACPUpdatesChangeNothing(t *testing.T) {
	d := newTestDeck(t)
	desk, conn := openACPDesk(t, d, func(ctx context.Context, desk *acpDesk) (acp.StopReason, error) {
		desk.Update(toolCall("t1", "in_progress"))
		return acp.StopCancelled, nil
	})
	if _, err := desk.prompt(context.Background(), "go"); err != nil {
		t.Fatal(err)
	}
	desk.Update(toolCall("late", "pending"))
	desk.Update(toolDone("t1", "out"))
	desk.Update(chunk("Too late"))
	desk.settle()
	s := sessionOf(d, desk)
	if s.State != Idle || s.Tool != "" {
		t.Errorf("after a late update: state %s, tool %q", s.State, s.Tool)
	}
	if err := desk.cancel(context.Background()); err != nil || len(conn.cancels) != 1 || conn.cancels[0] != "s1" {
		t.Errorf("cancel: %v, %q", err, conn.cancels)
	}
	desk.close()
	data, _ := os.ReadFile(s.Transcript)
	if !strings.Contains(string(data), `"id":"late"`) || !strings.Contains(string(data), "Too late") {
		t.Errorf("the late updates aren't in the transcript:\n%s", data)
	}
}

// The client's reader calls Update, and reads nothing from the agent while it runs: Update never waits for the deck,
// even with its lock held for long, and past the queue's room an update is left out, not waited for.
func TestACPUpdateNeverWaitsForTheDeck(t *testing.T) {
	d := newTestDeck(t)
	returned := make(chan struct{})
	n := toolCall("t1", "pending")
	desk, _ := openACPDesk(t, d, func(ctx context.Context, desk *acpDesk) (acp.StopReason, error) {
		// The turn's start is already stuck behind the deck's lock: every update of it waits in the queue.
		for range 3 * acpQueueMost {
			desk.Update(n)
		}
		close(returned)
		return "end_turn", nil
	})
	d.mu.Lock()
	go desk.prompt(context.Background(), "go")
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		d.mu.Unlock()
		t.Fatal("Update waited for the deck's lock")
	}
	d.mu.Unlock()
	desk.mu.Lock()
	dropped := desk.dropped
	desk.mu.Unlock()
	if dropped == 0 {
		t.Error("nothing was left out past the queue's room")
	}
	// The room the worker's batches took is freed once it is done with them (RT126ACP1 N2).
	desk.settle()
	desk.mu.Lock()
	left := desk.queueBytes
	desk.mu.Unlock()
	if left != 0 {
		t.Errorf("%d bytes still counted once the queue was empty", left)
	}
}

// A permission ask is shown on its own desk's session only, while its turn is out; any other is refused at once.
// Nothing answers it: it waits until its turn is cancelled, and is then "cancelled", never a pick.
func TestAnACPAskIsShownAndNeverAnswered(t *testing.T) {
	d := newTestDeck(t)
	asked := make(chan acp.PermissionOutcome, 1)
	ask := acp.PermissionRequest{SessionID: "s1", ToolCall: acp.ToolCall{ToolCallID: "t1", Kind: "execute",
		RawInput: json.RawMessage(`{"command":"rm -rf build"}`)},
		Options: []acp.PermissionOption{{OptionID: "yes", Name: "Allow", Kind: "allow_once"}}}
	desk, _ := openACPDesk(t, d, func(ctx context.Context, desk *acpDesk) (acp.StopReason, error) {
		other := ask
		other.SessionID = "s2"
		if err := refusedAtOnce(desk, other); err != nil {
			t.Errorf("an ask for another session: %v", err)
		}
		askCtx, cancel := context.WithCancel(ctx)
		go func() {
			o, _ := desk.RequestPermission(askCtx, ask)
			asked <- o
		}()
		waitFor(t, "the ask shown", func() bool {
			s := sessionOf(d, desk)
			return s.State == Asking && s.WaitingFor == "Bash"
		})
		select {
		case o := <-asked:
			t.Errorf("the ask was answered by itself: %+v", o)
			cancel()
			return acp.StopCancelled, nil
		case <-time.After(200 * time.Millisecond):
		}
		cancel() // as the client does when the turn is cancelled
		select {
		case o := <-asked:
			if o.OptionID != "" {
				t.Errorf("a cancelled ask was answered %q", o.OptionID)
			}
		case <-time.After(3 * time.Second):
			t.Error("a cancelled ask didn't end")
		}
		return acp.StopCancelled, nil
	})
	if err := refusedAtOnce(desk, ask); err != nil {
		t.Errorf("an ask outside a turn: %v", err)
	}
	if _, err := desk.prompt(context.Background(), "clean up"); err != nil {
		t.Fatal(err)
	}
	desk.settle()
	if s := sessionOf(d, desk); s.State != Idle {
		t.Errorf("after the cancelled turn: %s", s.State)
	}
}

// refusedAtOnce asks, and says why it wasn't refused straight away: an ask shown would wait for its turn's end.
func refusedAtOnce(desk *acpDesk, req acp.PermissionRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	_, err := desk.RequestPermission(ctx, req)
	if !errors.Is(err, errACPNotItsTurn) || time.Since(start) > 500*time.Millisecond {
		return fmt.Errorf("answered %v after %s", err, time.Since(start))
	}
	return nil
}

// N4: the desk answers no file or terminal request, so the client tells the agent it can't.
func TestAnACPDeskAnswersNoFileOrTerminal(t *testing.T) {
	var h acp.Handler = &acpDesk{}
	if _, ok := h.(acp.FileHandler); ok {
		t.Error("the desk is a FileHandler")
	}
	if _, ok := h.(acp.TerminalHandler); ok {
		t.Error("the desk is a TerminalHandler")
	}
}

// An ACP desk's session ends with its client, never by a look at tabs: its tab is no terminal's.
func TestAnACPSessionEndsWithItsClientOnly(t *testing.T) {
	d := newTestDeck(t)
	desk, conn := openACPDesk(t, d, nil)
	d.mu.Lock()
	d.sessions.Prune(func(string) bool { return false }, nowMillis())
	d.sessions.PruneIDs(map[string]string{desk.tab: desk.tab}, nowMillis())
	d.mu.Unlock()
	if sessionOf(d, desk) == nil {
		t.Fatal("a look at tabs dropped the ACP desk's session")
	}
	conn.Close() // the agent stopped by itself
	<-desk.done
	if sessionOf(d, desk) != nil {
		t.Error("the session outlived its agent")
	}
	if _, err := desk.prompt(context.Background(), "hello"); !errors.Is(err, errACPNotOpen) {
		t.Errorf("a prompt after the end: %v", err)
	}
}

// RT126ACP1 B1: the agent chooses its session's id, so the deck never keys the desk's session by it. An agent that
// answers session/new with another desk's id reaches only its own session, keyed by the desk's acp: tab: the other
// desk keeps its tab, its transcript and its life. And no hook event reaches the ACP desk's session, by its id or
// by its tab.
func TestAnACPAgentCantNameAnotherDesksSession(t *testing.T) {
	d := newTestDeck(t)
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": "victim-0001", "cwd": "/w",
		"source": "startup", "transcript_path": "/victim.jsonl"}, "claude", "%7", "/w", nowMillis())
	unchanged := func(when string) {
		t.Helper()
		if s := sessionByID(d, "victim-0001"); s == nil || s.Agent != Claude || s.Tab != "%7" || s.Transcript != "/victim.jsonl" || s.State != Idle {
			t.Errorf("%s, the other desk's session is %+v", when, s)
		}
	}
	unchanged("before")
	conn := &standInACP{session: "victim-0001", done: make(chan struct{}), turn: func(ctx context.Context, desk *acpDesk) (acp.StopReason, error) {
		desk.Update(update("victim-0001", map[string]any{"sessionUpdate": "tool_call", "toolCallId": "t1", "kind": "execute",
			"status": "pending"}))
		return "end_turn", nil
	}}
	desk, _ := openACPDeskOn(t, d, conn)
	if _, err := desk.prompt(context.Background(), "go"); err != nil {
		t.Fatal(err)
	}
	desk.settle()
	unchanged("after the ACP desk's turn")
	if s := sessionOf(d, desk); s == nil || s.ID != desk.tab || s.Agent != ACPAgent("gemini") || s.State != Unread {
		t.Fatalf("the ACP desk's own session: %+v", s)
	}

	// A hook event by the desk's id, or in its tab, is refused: it neither moves nor takes the place of its session.
	before := sessionOf(d, desk)
	linesBefore := len(d.events.snapshot())
	d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": desk.tab, "cwd": "/elsewhere"},
		"claude", "%8", "/elsewhere", nowMillis())
	d.Ingest(map[string]any{"hook_event_name": "SessionStart", "session_id": "intruder-0001", "cwd": "/elsewhere",
		"source": "startup"}, "claude", desk.tab, "/elsewhere", nowMillis())
	d.Ingest(map[string]any{"hook_event_name": "SessionEnd", "session_id": desk.tab, "reason": "other"},
		"claude", "", "", nowMillis())
	if s := sessionOf(d, desk); s == nil || s.Agent != before.Agent || s.Folder != before.Folder || s.State != before.State {
		t.Errorf("a hook event reached the ACP desk's session: %+v", s)
	}
	if sessionByID(d, "intruder-0001") != nil {
		t.Error("a hook event took the ACP desk's tab")
	}

	// RT126ACP1R2 B1-r: nor does one with a pid and a parent, as a headless run's hook sends them, set either on the
	// desk's session; and a prune by pids, told every process has gone, leaves it be.
	d.IngestFrom(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": desk.tab, "cwd": "/elsewhere"},
		"claude", "headless:probe", "/elsewhere", 424242, "/parent-probe", nowMillis())
	if s := sessionOf(d, desk); s == nil || s.PID != 0 || s.ParentFolder != "" {
		t.Errorf("a hook event set the ACP desk's session's pid or parent: %+v", s)
	}
	if n := len(d.events.snapshot()); n != linesBefore {
		t.Errorf("hook events filed %d lines in the ACP desk's name", n-linesBefore)
	}
	d.mu.Lock()
	d.sessions.PruneGone(func(int) bool { return true }, nowMillis())
	d.mu.Unlock()
	if s := sessionOf(d, desk); s == nil || s.PID != 0 || s.ParentFolder != "" || s.State != before.State {
		t.Errorf("after PruneGone, the ACP desk's session: %+v", s)
	}

	desk.close()
	if sessionOf(d, desk) != nil {
		t.Error("the ACP desk's session outlived its desk")
	}
	unchanged("after the ACP desk closed")
}

// Only an ACP desk's own events, by its own acp: tab, go through ApplyACP; and only a session made that way is held
// by the core, out of Prune's reach (RT126ACP1 N3).
func TestOnlyAnACPDesksOwnSessionIsHeldByTheCore(t *testing.T) {
	s := NewSessions()
	gemini, tab := ACPAgent("gemini"), acpDeskPrefix+"0123456789abcdef"
	start := func(id string) map[string]any {
		return map[string]any{"hook_event_name": "SessionStart", "session_id": id, "cwd": "/w", "source": "startup"}
	}
	for _, c := range []struct {
		why     string
		agent   Agent
		id, tab string
	}{
		{"not an ACP agent", Claude, tab, tab},
		{"the agent's own id", gemini, "victim-0001", tab},
		{"another tab than its id", gemini, tab, acpDeskPrefix + "fedcba9876543210"},
		{"a terminal's tab", gemini, "%7", "%7"},
	} {
		if s.ApplyACP(start(c.id), c.agent, c.tab, "/w", 1) {
			t.Errorf("ApplyACP took an event of %s", c.why)
		}
	}
	if s.Apply(start(tab), Claude, "%7", "/w", 1) || s.Apply(start("x"), Claude, tab, "/w", 1) {
		t.Error("Apply took an acp: id or tab")
	}
	if !s.ApplyACP(start(tab), gemini, tab, "/w", 1) || !s.Get(tab).heldByCore() {
		t.Fatal("the desk's own session isn't held by the core")
	}
	if !s.Apply(start("stray"), gemini, "%9", "/w", 1) || s.Get("stray").heldByCore() {
		t.Error("an ACP agent's session on a terminal's tab is held by the core")
	}
	s.Prune(func(string) bool { return false }, 2)
	if s.Get(tab) == nil || s.Get("stray") != nil {
		t.Errorf("after Prune: the desk's %v, the stray one's %v", s.Get(tab), s.Get("stray"))
	}
	// Nor PruneGone's, should a pid ever reach it (RT126ACP1R2 B1-r): its client, not a process, says when it ends.
	s.Get(tab).PID = 424242
	s.PruneGone(func(int) bool { return true }, 3)
	if s.Get(tab) == nil {
		t.Error("PruneGone dropped the desk's own session")
	}
}

// RT126ACP1 N5: a tool call's later updates seldom say its kind again, so its end is shown as its start was.
func TestAnACPToolKeepsItsKindToItsEnd(t *testing.T) {
	kinds := map[string]string{}
	start := acp.SessionUpdate{Kind: "tool_call", ToolCallID: "t1", ToolKind: "execute", Status: "pending"}
	if got := acpKindOf(kinds, start); got != "execute" {
		t.Errorf("the start's kind: %q", got)
	}
	if got := acpKindOf(kinds, acp.SessionUpdate{Kind: "tool_call_update", ToolCallID: "t1", Status: "in_progress"}); got != "execute" {
		t.Errorf("an update's kind: %q", got)
	}
	if got := acpKindOf(kinds, acp.SessionUpdate{Kind: "tool_call_update", ToolCallID: "t1", Status: "completed"}); got != "execute" {
		t.Errorf("the end's kind: %q", got)
	}
	if len(kinds) != 0 {
		t.Errorf("a done call's kind is still kept: %v", kinds)
	}
	for i := range acpKindsMost + 10 {
		acpKindOf(kinds, acp.SessionUpdate{Kind: "tool_call", ToolCallID: fmt.Sprint(i), ToolKind: "read"})
	}
	if len(kinds) != acpKindsMost {
		t.Errorf("%d kinds kept, past the most", len(kinds))
	}
}

// The ACP dialect in Claude Code's words: only the desk's own fields are carried over, by Claude Code's names.
func TestACPEventsInClaudeCodesWords(t *testing.T) {
	gemini := ACPAgent("gemini")
	base := func(name string, fields map[string]any) map[string]any {
		event := map[string]any{"hook_event_name": name, "sessionId": "s1", "cwd": "/w", "transcriptPath": "/t.jsonl",
			"tool_name": "SendMessage", "agent_id": "x"}
		for k, v := range fields {
			event[k] = v
		}
		return event
	}
	tool := func(kind, status string) map[string]any {
		return map[string]any{"sessionUpdate": kind, "toolCallId": "t1", "kind": "read", "status": status}
	}
	for _, c := range []struct {
		event map[string]any
		want  string
	}{
		{base("session/new", nil), "SessionStart"},
		{base("session/closed", nil), "SessionEnd"},
		{base("session/prompt", nil), "UserPromptSubmit"},
		{base("session/prompt_done", map[string]any{"stopReason": "end_turn"}), "Stop"},
		{base("session/prompt_done", map[string]any{"stopReason": "refusal"}), "Stop"},
		{base("session/prompt_done", map[string]any{"stopReason": "cancelled"}), "Interrupt"},
		{base("session/prompt_done", map[string]any{"error": "acp"}), "StopFailure"},
		{base("session/request_permission", map[string]any{"toolCallId": "t1", "kind": "execute"}), "PermissionRequest"},
		{base("session/update", tool("tool_call", "pending")), "PreToolUse"},
		{base("session/update", tool("tool_call", "completed")), "PostToolUse"},
		{base("session/update", tool("tool_call_update", "in_progress")), ""},
		{base("session/update", tool("tool_call_update", "completed")), "PostToolUse"},
		{base("session/update", tool("tool_call_update", "failed")), "PostToolUseFailure"},
		{base("session/update", map[string]any{"sessionUpdate": "agent_message_chunk", "status": "completed"}), ""},
		{base("PreToolUse", nil), ""},
	} {
		got := gemini.canonical(c.event)
		name, _ := got["hook_event_name"].(string)
		if name != c.want || got != nil && (got["session_id"] != "s1" || got["transcript_path"] != "/t.jsonl" || got["agent_id"] != nil) {
			t.Errorf("%v → %v, want %s", c.event, got, c.want)
		}
		if id, _ := got["tool_use_id"].(string); id != "" && got["tool_name"] == "SendMessage" {
			t.Errorf("%v kept the event's own tool name", c.event)
		}
	}
	if got := gemini.canonical(base("session/update", tool("tool_call", "pending"))); got["tool_name"] != "Read" {
		t.Errorf("a read is %v", got["tool_name"])
	}
}

// The transcripts' folder is the support folder's own: never one a link points at somewhere else.
func TestTheACPTranscriptsFolderIsNoLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need a privilege on Windows")
	}
	support, elsewhere := t.TempDir(), t.TempDir()
	if err := os.Symlink(elsewhere, filepath.Join(support, acpTranscriptFolder)); err != nil {
		t.Fatal(err)
	}
	if _, err := newACPTranscript(support, "s1"); err == nil {
		t.Error("a transcript was made through a link")
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Errorf("written where the link points: %v", entries)
	}
}
