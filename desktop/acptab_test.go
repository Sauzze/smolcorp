package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/acp"
)

// ACP2: the ACP Tab, on a stand-in client: no agent is started, and nothing runs.

// tabACP is a stand-in for an ACP agent's client that keeps what the desk sent it: each prompt as "session:text",
// each cancel by session, and how many sessions it was asked for (s1, s2, …). Its turn, if set, plays the agent's
// side of a prompt; a cancel is also put on cancelled, as the agent hears it.
type tabACP struct {
	mu        sync.Mutex
	news      int
	prompts   []string
	cancels   []string
	cancelled chan string
	turn      func(ctx context.Context, session string) (acp.StopReason, error)
	done      chan struct{}
	closed    bool
}

func newTabACP() *tabACP { return &tabACP{cancelled: make(chan string, 8), done: make(chan struct{})} }

func (c *tabACP) Initialize(context.Context) (*acp.InitializeResponse, error) {
	return &acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersion}, nil
}
func (c *tabACP) NewSession(context.Context, string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.news++
	return fmt.Sprintf("s%d", c.news), nil
}
func (c *tabACP) Prompt(ctx context.Context, session string, prompt []acp.ContentBlock) (acp.StopReason, error) {
	c.mu.Lock()
	var texts []string
	for _, block := range prompt {
		texts = append(texts, block.Text)
	}
	c.prompts = append(c.prompts, session+":"+strings.Join(texts, ""))
	turn := c.turn
	c.mu.Unlock()
	if turn == nil {
		return "end_turn", nil
	}
	return turn(ctx, session)
}
func (c *tabACP) Cancel(_ context.Context, session string) error {
	c.mu.Lock()
	c.cancels = append(c.cancels, session)
	c.mu.Unlock()
	c.cancelled <- session
	return nil
}
func (c *tabACP) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.done)
	}
	return nil
}
func (c *tabACP) Done() <-chan struct{} { return c.done }

func (c *tabACP) sent() (prompts, cancels []string, news int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.prompts...), append([]string{}, c.cancels...), c.news
}

// untilCancelled is a turn that works until the agent is told to cancel it.
func (c *tabACP) untilCancelled(ctx context.Context, _ string) (acp.StopReason, error) {
	select {
	case <-c.cancelled:
		return acp.StopCancelled, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// acpTabDeck is a deck with no Mac app, Return right after the words, and a desk for the known agent gemini on conn,
// its session open and held where the deck finds its Tab. Closed at the end.
func acpTabDeck(t *testing.T, conn *tabACP) (*Deck, *acpDesk) {
	t.Helper()
	saved, settle := host, phoneTextSettle
	host, phoneTextSettle = nil, time.Millisecond
	t.Cleanup(func() { host, phoneTextSettle = saved, settle })
	d := newTestDeck(t)
	desk := newACPDesk(d, ACPAgent("gemini"), t.TempDir(), conn)
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
	d.mu.Lock()
	d.holdACPDesk(desk)
	d.mu.Unlock()
	return d, desk
}

// forgetReturn lets the next Return go at once: a test's taps come closer together than a person's.
func forgetReturn(d *Deck, id string) {
	d.mu.Lock()
	delete(d.lastReturn, id)
	d.mu.Unlock()
}

func draftOf(desk *acpDesk) string {
	desk.mu.Lock()
	defer desk.mu.Unlock()
	return desk.draft
}

// waitPrompts waits for the stand-in to have had want, exactly.
func waitPrompts(t *testing.T, conn *tabACP, want ...string) {
	t.Helper()
	waitFor(t, fmt.Sprintf("the prompts %q", want), func() bool {
		prompts, _, _ := conn.sent()
		return reflect.DeepEqual(prompts, want)
	})
}

// Words reach an ACP desk only by typeText's rules, as its prompt: the window's Send and the phone's, one line or
// several, and words typed without Return only once ⏎ sends them. Nothing the core types itself (Type) prompts.
func TestWordsReachAnACPDeskOnlyAsItsPrompt(t *testing.T) {
	conn := newTabACP()
	d, desk := acpTabDeck(t, conn)
	id := desk.tab
	if _, ok := d.tab(id).(acpTab); !ok {
		t.Fatalf("the desk's tab is %T", d.tab(id))
	}
	if status, message := d.TypeWindowText("fix the login", id, nil); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	waitPrompts(t, conn, "s1:fix the login")
	desk.settle()
	if s := sessionOf(d, desk); s.State == Thinking || s.State == Asking {
		t.Errorf("after the turn: %s", s.State)
	}

	// Typed without Return: kept as the desk's draft, sent by ⏎ alone.
	forgetReturn(d, id)
	no := false
	// Each piece goes in as typed, after the last, as into a terminal's line (typeText trims each one's edges).
	if status, message := d.TypeWindowText("and the logout", id, &no); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	if status, message := d.TypeWindowText("?", id, &no); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	time.Sleep(30 * time.Millisecond)
	waitPrompts(t, conn, "s1:fix the login")
	if status, message := d.Perform("send", id, nil); status != http.StatusNoContent {
		t.Fatalf("⏎: %d %s", status, message)
	}
	waitPrompts(t, conn, "s1:fix the login", "s1:and the logout?")
	if got := draftOf(desk); got != "" {
		t.Errorf("the draft after it went: %q", got)
	}
	desk.settle()

	// The phone's, with its switch on, several lines as one prompt.
	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	forgetReturn(d, id)
	yes := true
	if status, message := d.TypePhoneText("one\ntwo", id, &yes); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	waitPrompts(t, conn, "s1:fix the login", "s1:and the logout?", "s1:one\ntwo")
	desk.settle()

	// typeText's own rules still say no first.
	forgetReturn(d, id)
	if status, _ := d.TypeWindowText("!ls", id, nil); status != http.StatusBadRequest {
		t.Errorf("a shell line: %d", status)
	}
	// The core's own lines (a made button's text, /effort, /clear) type nothing here.
	if status, _ := d.toAgent(id, desk.agent, func(tab Tab) error { return d.typeInto(id, tab, "/clear") }); status == http.StatusNoContent {
		t.Error("Type went through")
	}
	time.Sleep(30 * time.Millisecond)
	waitPrompts(t, conn, "s1:fix the login", "s1:and the logout?", "s1:one\ntwo")
	if got := draftOf(desk); got != "" {
		t.Errorf("something was typed into the draft: %q", got)
	}
}

// Nothing is typed or sent into an ACP desk while its agent asks: not by the window, the phone or ⏎, and not by the
// desk itself in the moment before the deck shows the ask. Reject ends the ask through the protocol (session/cancel),
// never as a pick.
func TestAnACPDeskIsNeverTypedIntoWhileItAsks(t *testing.T) {
	conn := newTabACP()
	ask := acp.PermissionRequest{SessionID: "s1", ToolCall: acp.ToolCall{ToolCallID: "t1", Kind: "execute",
		RawInput: json.RawMessage(`{"command":"rm -rf build"}`)},
		Options: []acp.PermissionOption{{OptionID: "yes", Name: "Allow", Kind: "allow_once"}}}
	answered := make(chan acp.PermissionOutcome, 1)
	var desk *acpDesk
	conn.turn = func(ctx context.Context, session string) (acp.StopReason, error) {
		// As the client: a cancel ends the turn's asks, then the agent ends the turn "cancelled".
		askCtx, endAsk := context.WithCancel(ctx)
		defer endAsk()
		go func() { o, _ := desk.RequestPermission(askCtx, ask); answered <- o }()
		select {
		case <-conn.cancelled:
			endAsk()
			time.Sleep(10 * time.Millisecond)
			return acp.StopCancelled, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	d, held := acpTabDeck(t, conn)
	desk = held
	id := desk.tab
	if status, message := d.TypeWindowText("clean up", id, nil); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	waitFor(t, "the ask shown", func() bool { return sessionOf(d, desk).State == Asking })

	d.mu.Lock()
	d.config.PhoneMicEnabled = true
	d.mu.Unlock()
	forgetReturn(d, id)
	yes := true
	if status, message := d.TypeWindowText("yes", id, &yes); status != http.StatusConflict || message != windowAnswerFirst {
		t.Errorf("the window while asking: %d %s", status, message)
	}
	if status, message := d.TypePhoneText("yes", id, &yes); status != http.StatusConflict || message != answerFirst {
		t.Errorf("the phone while asking: %d %s", status, message)
	}
	asking := "Gemini CLI is asking you something. Answer it first."
	for _, action := range []string{"accept", "send"} {
		forgetReturn(d, id)
		if status, message := d.Perform(action, id, nil); status != http.StatusConflict || message != asking {
			t.Errorf("%s while asking: %d %s", action, status, message)
		}
	}
	// The desk's own refusal, whatever the deck shows: an ask is the desk's before it is the deck's.
	tab := acpTab{desk: desk}
	var refusal acpRefusal
	if _, err := tab.phoneText("yes"); !errors.As(err, &refusal) || string(refusal) != asking {
		t.Errorf("words into the draft while asking: %v", err)
	}
	if err := tab.Press(press(Enter)); !errors.As(err, &refusal) || string(refusal) != asking {
		t.Errorf("Return while asking: %v", err)
	}
	if prompts, _, _ := conn.sent(); len(prompts) != 1 || draftOf(desk) != "" {
		t.Errorf("typed while asking: %q, draft %q", prompts, draftOf(desk))
	}
	// Even with words already in its box, Return never sends while asking: they stay there.
	desk.mu.Lock()
	desk.draft = "left over"
	desk.mu.Unlock()
	if err := tab.Press(press(Enter)); !errors.As(err, &refusal) || string(refusal) != asking {
		t.Errorf("Return on a draft while asking: %v", err)
	}
	if prompts, _, _ := conn.sent(); len(prompts) != 1 || draftOf(desk) != "left over" {
		t.Errorf("a draft sent while asking: %q, draft %q", prompts, draftOf(desk))
	}
	desk.mu.Lock()
	desk.draft = ""
	desk.mu.Unlock()
	select {
	case o := <-answered:
		t.Fatalf("the ask was answered: %+v", o)
	default:
	}

	if status, message := d.Perform("reject", id, nil); status != http.StatusNoContent {
		t.Fatalf("reject: %d %s", status, message)
	}
	select {
	case o := <-answered:
		if o.OptionID != "" {
			t.Errorf("reject picked %q", o.OptionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reject didn't end the ask")
	}
	if _, cancels, _ := conn.sent(); !reflect.DeepEqual(cancels, []string{"s1"}) {
		t.Errorf("reject sent %q", cancels)
	}
	waitFor(t, "the turn over", func() bool { return sessionOf(d, desk).State == Idle })
}

// Esc stops an ACP desk's turn through the protocol (session/cancel); no key reaches it. While a turn is out nothing
// is typed; with none, Esc empties the draft, as it clears a terminal's line.
func TestEscStopsAnACPTurnThroughTheProtocol(t *testing.T) {
	conn := newTabACP()
	conn.turn = conn.untilCancelled
	d, desk := acpTabDeck(t, conn)
	id := desk.tab
	if status, message := d.TypeWindowText("long job", id, nil); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	waitFor(t, "the turn under way", func() bool { return sessionOf(d, desk).State == Thinking })
	forgetReturn(d, id)
	busy := "Gemini CLI is still working. Send this once it's done, or stop it with Esc."
	if status, message := d.TypeWindowText("more", id, nil); status != http.StatusConflict || message != busy {
		t.Errorf("words during a turn: %d %s", status, message)
	}
	if got := draftOf(desk); got != "" {
		t.Errorf("words during a turn were kept: %q", got)
	}
	// Keys mean nothing to it: not Esc, not the arrows.
	if err := (acpTab{desk: desk}).Press(press(Escape)); !errors.Is(err, errUnsupported) {
		t.Errorf("Esc as a key: %v", err)
	}
	for _, action := range []string{"up", "down"} {
		if status, _ := d.Perform(action, id, nil); status == http.StatusNoContent {
			t.Errorf("%s went", action)
		}
	}
	if _, cancels, _ := conn.sent(); len(cancels) != 0 {
		t.Fatalf("a key cancelled: %q", cancels)
	}
	if status, message := d.Perform("esc", id, nil); status != http.StatusNoContent {
		t.Fatalf("esc: %d %s", status, message)
	}
	waitFor(t, "the turn stopped", func() bool { return sessionOf(d, desk).State == Idle })
	if prompts, cancels, _ := conn.sent(); !reflect.DeepEqual(cancels, []string{"s1"}) || len(prompts) != 1 {
		t.Errorf("esc: prompts %q, cancels %q", prompts, cancels)
	}

	// No turn out: Esc empties the draft and tells the agent nothing.
	conn.mu.Lock()
	conn.turn = nil
	conn.mu.Unlock()
	forgetReturn(d, id)
	no := false
	if status, message := d.TypeWindowText("half a thought", id, &no); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, message)
	}
	if status, _ := d.Perform("esc", id, nil); status != http.StatusNoContent || draftOf(desk) != "" {
		t.Errorf("esc with a draft: %d, draft %q", status, draftOf(desk))
	}
	if status, _ := d.Perform("send", id, nil); status != http.StatusNoContent {
		t.Errorf("⏎ with nothing: %d", status)
	}
	time.Sleep(30 * time.Millisecond)
	if prompts, cancels, _ := conn.sent(); len(prompts) != 1 || len(cancels) != 1 {
		t.Errorf("after esc on a draft: prompts %q, cancels %q", prompts, cancels)
	}
}

// New conversation on an ACP desk is a new session/new, never /clear or /new typed: the desk keeps its place on the
// deck, idle, with a transcript of its own, and the old session's updates are dropped. Not while it works.
func TestNewConversationOnAnACPDeskIsANewSession(t *testing.T) {
	conn := newTabACP()
	d, desk := acpTabDeck(t, conn)
	id := desk.tab
	if status, _ := d.TypeWindowText("first", id, nil); status != http.StatusNoContent {
		t.Fatal(status)
	}
	waitPrompts(t, conn, "s1:first")
	desk.settle()
	before := sessionOf(d, desk).Transcript
	forgetReturn(d, id)
	no := false
	if status, _ := d.TypeWindowText("left over", id, &no); status != http.StatusNoContent {
		t.Fatal(status)
	}

	if status, message := d.Perform("clear", id, nil); status != http.StatusNoContent {
		t.Fatalf("clear: %d %s", status, message)
	}
	desk.Update(update("s1", map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": "old news"}}))
	desk.settle()
	s := sessionOf(d, desk)
	if _, _, news := conn.sent(); news != 2 || desk.session != "s2" {
		t.Errorf("sessions asked for: %d, now %q", news, desk.session)
	}
	if s == nil || s.ID != id || s.State != Idle || s.Transcript == "" || s.Transcript == before {
		t.Fatalf("the desk after clear: %+v (transcript was %s)", s, before)
	}
	if got := draftOf(desk); got != "" {
		t.Errorf("the draft outlived the conversation: %q", got)
	}
	forgetReturn(d, id)
	if status, _ := d.TypeWindowText("second", id, nil); status != http.StatusNoContent {
		t.Fatal(status)
	}
	waitPrompts(t, conn, "s1:first", "s2:second")
	desk.settle()
	data, err := os.ReadFile(s.Transcript)
	if err != nil || !strings.Contains(string(data), "second") || strings.Contains(string(data), "old news") ||
		strings.Contains(string(data), "first") {
		t.Errorf("the new transcript (%v): %s", err, data)
	}

	conn.mu.Lock()
	conn.turn = conn.untilCancelled
	conn.mu.Unlock()
	forgetReturn(d, id)
	if status, _ := d.TypeWindowText("third", id, nil); status != http.StatusNoContent {
		t.Fatal(status)
	}
	waitFor(t, "the turn under way", func() bool { return sessionOf(d, desk).State == Thinking })
	if status, message := d.Perform("clear", id, nil); status != http.StatusConflict {
		t.Errorf("clear during a turn: %d %s", status, message)
	}
	// Below the deck's own busy check, the desk says no too.
	if err := desk.refusal(desk.renew(context.Background())); err == nil {
		t.Error("a new conversation during a turn")
	}
	if prompts, _, news := conn.sent(); news != 2 || slicesHasPrefix(prompts, "/") {
		t.Errorf("clear typed or renewed: %q, %d sessions", prompts, news)
	}
	if status, _ := d.Perform("esc", id, nil); status != http.StatusNoContent {
		t.Error(status)
	}
	waitFor(t, "the turn stopped", func() bool { return sessionOf(d, desk).State == Idle })
}

// slicesHasPrefix says whether any prompt's text starts with prefix.
func slicesHasPrefix(prompts []string, prefix string) bool {
	for _, p := range prompts {
		if _, text, _ := strings.Cut(p, ":"); strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// The deck finds an ACP desk's Tab by its name alone, never a terminal's or the Mac app's; the phone may type into
// it, nothing dictates into it, and fired, it is let go.
func TestTheDeckFindsAnACPDesksTabAndNoTerminal(t *testing.T) {
	conn := newTabACP()
	d, desk := acpTabDeck(t, conn)
	id := desk.tab
	d.mu.Lock()
	unknown := d.tab("acp:0123456789abcdef")
	d.mu.Unlock()
	if unknown != nil {
		t.Errorf("an acp: name with no desk: %T", unknown)
	}
	stopDecksOf(t)
	withHost(t, bridgeAnswer{OK: true})
	if tab := hostTab(id, "acp", strings.TrimPrefix(id, acpDeskPrefix), ""); tab != nil {
		t.Errorf("the Mac app was given an ACP desk: %T", tab)
	}
	d.mu.Lock()
	tab := d.tab(id)
	d.mu.Unlock()
	if _, ok := tab.(acpTab); !ok {
		t.Errorf("under the Mac app: %T", tab)
	}
	if !typesText(id) {
		t.Error("the phone can't type into an ACP desk")
	}
	if d.dictatesFor(id, desk.agent) {
		t.Error("dictation types into an ACP desk")
	}
	for _, err := range []error{tab.Type("x"), tab.Focus(), tab.Scroll(3), tab.ScrollToBottom()} {
		if !errors.Is(err, errUnsupported) {
			t.Errorf("a terminal's action: %v", err)
		}
	}

	d.mu.Lock()
	d.config.HiringEnabled = true
	d.mu.Unlock()
	if status, message := d.Perform("fire", id, nil); status != http.StatusNoContent {
		t.Fatalf("fire: %d %s", status, message)
	}
	d.mu.Lock()
	gone := d.tab(id)
	d.mu.Unlock()
	if gone != nil {
		t.Errorf("a fired desk's tab: %T", gone)
	}
	conn.mu.Lock()
	closed := conn.closed
	conn.mu.Unlock()
	if !closed {
		t.Error("fired, its client wasn't closed")
	}
}
