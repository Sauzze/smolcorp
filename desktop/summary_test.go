package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ✨ Summary, on Windows and Linux: the transcript's tail, the opt-in, the
// key and everywhere it must never turn up. A stand-in secret store and a
// stand-in Anthropic: nothing is sent and no real key is used. The made-up
// key is put together here, so no file holds one whole.
var testSummaryKey = "sk-" + "ant-" + "api03-" + strings.Repeat("T3stK3y", 12) + "-xY9z"

// memoryStore is a secret store in memory, or one that fails as a computer with none does.
func memoryStore(missing bool) secretStore {
	var mu sync.Mutex
	key := ""
	return secretStore{
		get: func() (string, error) {
			mu.Lock()
			defer mu.Unlock()
			if missing {
				return "", errNoSecretStore
			}
			if key == "" {
				return "", errNotFoundForTests
			}
			return key, nil
		},
		set: func(k string) error {
			if missing {
				return errNoSecretStore
			}
			mu.Lock()
			key = k
			mu.Unlock()
			return nil
		},
		delete: func() error { mu.Lock(); key = ""; mu.Unlock(); return nil },
	}
}

var errNotFoundForTests = keyringNotFound()

// assertNoPartOfTheKey fails if any run of 12 characters of the key, past its common start, is in text.
func assertNoPartOfTheKey(t *testing.T, text, what string) {
	t.Helper()
	for start := 7; start+12 <= len(testSummaryKey); start++ {
		if strings.Contains(text, testSummaryKey[start:start+12]) {
			t.Fatalf("%s holds part of the key", what)
		}
	}
}

func transcriptFile(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func jsonLine(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func TestTheTailIsShortRedactedAndInTheUsersLanguage(t *testing.T) {
	lines := [][]byte{
		[]byte(jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "corrige le bug de connexion s'il te plaît"}})),
		[]byte(jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "thinking", "thinking": "private"},
			map[string]any{"type": "text", "text": "Je regarde."},
			map[string]any{"type": "tool_use", "name": "Bash", "input": map[string]any{"command": "export API_KEY=" + testSummaryKey}},
		}}})),
		[]byte(jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "content": "line one\nline two"},
		}}})),
	}
	extract := extractTranscript(lines, summaryBudget, nil, nil)
	want := "LATEST REQUEST: corrige le bug de connexion s'il te plaît\nCLAUDE: Je regarde.\nTOOL Bash: [redacted]\nRESULT: line one"
	if extract.Text != want || extract.Language != "French" {
		t.Fatalf("got %q in %s", extract.Text, extract.Language)
	}
	assertNoPartOfTheKey(t, extract.Text, "the excerpt")
}

func userLine(text, uuid string) string {
	entry := map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}}
	if uuid != "" {
		entry["uuid"] = uuid
	}
	return jsonLine(entry)
}

func claudeLine(text string) string {
	return jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{
		map[string]any{"type": "text", "text": text},
	}}})
}

func byteLines(lines ...string) [][]byte {
	out := make([][]byte, len(lines))
	for i, line := range lines {
		out[i] = []byte(line)
	}
	return out
}

func TestOnlyTheLatestAskIsMarked(t *testing.T) {
	extract := extractTranscript(byteLines(userLine("Fix the elevator", "u1"), claudeLine("Done."), userLine("Now the summary", "u2")), summaryBudget, nil, nil)
	if want := "USER: Fix the elevator\nCLAUDE: Done.\nLATEST REQUEST: Now the summary"; extract.Text != want || extract.Latest != "u2" {
		t.Fatalf("got %q, latest %q", extract.Text, extract.Latest)
	}
	if extract := extractTranscript(byteLines(claudeLine("Nothing asked yet.")), summaryBudget, nil, nil); extract.Latest != "" {
		t.Fatalf("no request, yet latest %q", extract.Latest)
	}
}

// A long session on one thing, then a question on something else.
func TestALongSessionStartsAtItsLatestRequest(t *testing.T) {
	log := []string{userLine("Fix the elevator so the room slides through one window", "")}
	for i := 1; i <= 300; i++ {
		log = append(log, claudeLine(fmt.Sprintf("Elevator step %d: moved the window, checked the slide, carried on.", i)))
	}
	log = append(log, claudeLine(strings.Repeat("Elevator notes. ", 80)+"Anything else?"),
		userLine("Why is my summary still about the elevator?", ""),
		jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/repo/summary.go"}},
		}}}),
		claudeLine("The prompt never says which request is the latest."))
	text := extractTranscript(byteLines(log...), summaryBudget, nil, nil).Text
	parts := strings.Split(text, "LATEST REQUEST: ")
	if len(parts) != 2 {
		t.Fatalf("marked %d times: %q", len(parts)-1, text)
	}
	// Only a short lead-in comes before it: the end of what Claude said last, where it asked.
	if n := len([]rune(parts[0])); n > summaryLeadIn || !strings.HasPrefix(parts[0], "…\nCLAUDE: …") || !strings.HasSuffix(parts[0], "Anything else?\n") {
		t.Fatalf("lead-in of %d characters: %q", n, parts[0])
	}
	if strings.Contains(text, "Elevator step") {
		t.Fatal("older work is sent")
	}
	want := "Why is my summary still about the elevator?\nTOOL Read: /repo/summary.go\nCLAUDE: The prompt never says which request is the latest."
	if parts[1] != want {
		t.Fatalf("from the request on: %q", parts[1])
	}
}

func TestTheLeadInIsWholeLinesWhenTheyFit(t *testing.T) {
	var log []string
	for i := 1; i <= 50; i++ {
		log = append(log, claudeLine(fmt.Sprintf("Old %d.", i)))
	}
	log = append(log, claudeLine("Shall I?"), userLine("yes", ""))
	text := extractTranscript(byteLines(log...), summaryBudget, nil, nil).Text
	lead := strings.Split(text, "LATEST REQUEST: ")[0]
	if !strings.HasPrefix(text, "…\n") || !strings.HasSuffix(text, "CLAUDE: Old 50.\nCLAUDE: Shall I?\nLATEST REQUEST: yes") ||
		strings.Contains(text, "Old 1.") || len([]rune(lead)) > summaryLeadIn {
		t.Fatalf("got %q", text)
	}
}

func TestWhenWhatFollowsIsTooLongTheRequestAndTheEndAreKept(t *testing.T) {
	log := []string{userLine("Migrate the database to Postgres", "")}
	for i := 1; i <= 400; i++ {
		log = append(log, claudeLine(fmt.Sprintf("Step %d: moved one more table across, checked it, carried on.", i)))
	}
	text := extractTranscript(byteLines(log...), 2_000, nil, nil).Text
	if len([]rune(text)) > 2_000 || !strings.HasPrefix(text, "LATEST REQUEST: Migrate the database to Postgres\n…") ||
		!strings.HasSuffix(text, "Step 400: moved one more table across, checked it, carried on.") || strings.Contains(text, "Step 1:") {
		t.Fatalf("got %q", text)
	}
}

func TestANewRequestWithin30SecondsIsSummarisedAfresh(t *testing.T) {
	log := []string{userLine("Fix the elevator", "u1"), claudeLine("On it.")}
	path := transcriptFile(t, log...)
	calls := 0
	clock := time.Unix(1_000, 0)
	s := newSummaries(func() string { return testSummaryKey })
	s.now = func() time.Time { return clock }
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) {
		calls++
		return fmt.Sprintf("Summary %d", calls), nil
	}
	write := func() {
		if err := os.WriteFile(path, []byte(strings.Join(log, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if text, _ := s.Summarize("s1", path, "p"); text != "Summary 1" {
		t.Fatalf("first: %q", text)
	}
	// The agent carrying on with the same request keeps it.
	log = append(log, claudeLine("Still on the elevator."))
	write()
	clock = clock.Add(5 * time.Second)
	if text, _ := s.Summarize("s1", path, "p"); text != "Summary 1" {
		t.Fatalf("same request: %q", text)
	}
	// A new message, even the same words, is a new request.
	log = append(log, userLine("Fix the elevator", "u2"))
	write()
	clock = clock.Add(5 * time.Second)
	if text, _ := s.Summarize("s1", path, "p"); text != "Summary 2" || calls != 2 {
		t.Fatalf("new request: %q after %d calls", text, calls)
	}
}

// Requests as Claude Code writes them: field names from real transcripts, words made up.

// queuedLine is a prompt sent while the agent works: an attachment queued into its turn.
func queuedLine(prompt any, uuid string, origin map[string]any, mode string, meta bool) string {
	attachment := map[string]any{"type": "queued_command", "commandMode": mode, "origin": origin, "prompt": prompt,
		"delivery_id": "d-" + uuid, "source_uuid": "s-" + uuid, "timestamp": "2026-10-22T01:00:00.000Z"}
	if origin["kind"] == "human" {
		attachment["humanTurn"] = true
	}
	if meta {
		attachment["isMeta"] = true
	}
	return jsonLine(map[string]any{"type": "attachment", "uuid": uuid, "timestamp": "2026-10-22T01:00:00.000Z", "attachment": attachment})
}

var human = map[string]any{"kind": "human"}

func peerOrigin() map[string]any {
	return map[string]any{"kind": "peer", "name": "peer-one", "from": "uds:/tmp/cc-socks/1.sock", "fromMode": "prompting",
		"msg_id": "m1", "verifiedPeerPid": 1, "body": "Commit both now."}
}

const peerWrapped = "<cross-session-message from=\"uds:/tmp/cc-socks/1.sock\" from-name=\"peer-one\" from-mode=\"prompting\">\n" +
	"Commit both now.\n</cross-session-message>"

func TestAPromptQueuedWhileTheAgentWorksIsTheLatest(t *testing.T) {
	extract := extractTranscript(byteLines(
		userLine("Fix the elevator", "u1"),
		claudeLine("On it."),
		queuedLine("Stop, look at the summary instead", "q1", human, "prompt", false),
		claudeLine("Looking at the summary."),
		queuedLine("<task-notification>\n<task-id>b1</task-id>\n</task-notification>", "t1",
			map[string]any{"kind": "task-notification", "producer": "bash"}, "task-notification", false),
	), summaryBudget, nil, nil)
	want := "USER: Fix the elevator\nCLAUDE: On it.\nLATEST REQUEST: Stop, look at the summary instead\nCLAUDE: Looking at the summary."
	if extract.Text != want || extract.Latest != "q1" {
		t.Fatalf("got %q, latest %q", extract.Text, extract.Latest)
	}
	// With an image, only its words.
	image := extractTranscript(byteLines(queuedLine([]any{
		map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "AAAA"}},
		map[string]any{"type": "text", "text": "What's wrong here?"},
	}, "q2", human, "prompt", false)), summaryBudget, nil, nil)
	if image.Text != "LATEST REQUEST: What's wrong here?" {
		t.Fatalf("with an image: %q", image.Text)
	}
}

func TestAPastedPromptIsARequest(t *testing.T) {
	pasted := jsonLine(map[string]any{"type": "user", "uuid": "u2", "promptSource": "typed", "origin": human,
		"message": map[string]any{"role": "user", "content": "<pasted_content>a log</pasted_content>\nWhy does this fail?"}})
	extract := extractTranscript(byteLines(userLine("Fix the elevator", "u1"), pasted), summaryBudget, nil, nil)
	if extract.Latest != "u2" || !strings.HasSuffix(extract.Text, "LATEST REQUEST: <pasted_content>a log</pasted_content>\nWhy does this fail?") {
		t.Fatalf("got %q, latest %q", extract.Text, extract.Latest)
	}
}

func TestACompactionSummaryIsNoRequest(t *testing.T) {
	compacted := jsonLine(map[string]any{"type": "user", "uuid": "c1", "isCompactSummary": true, "isVisibleInTranscriptOnly": true,
		"message": map[string]any{"role": "user", "content": "This session is being continued from a previous conversation that ran out of context. The elevator…"}})
	extract := extractTranscript(byteLines(userLine("Look at the summary", "u1"), claudeLine("Reading it."), compacted, claudeLine("Carrying on.")), summaryBudget, nil, nil)
	if want := "LATEST REQUEST: Look at the summary\nCLAUDE: Reading it.\nCLAUDE: Carrying on."; extract.Text != want || extract.Latest != "u1" {
		t.Fatalf("got %q, latest %q", extract.Text, extract.Latest)
	}
}

func TestAPeerMessageAfterTheRequestIsMarkedAsSuch(t *testing.T) {
	asUser := jsonLine(map[string]any{"type": "user", "uuid": "p1", "isMeta": true, "promptSource": "system", "origin": peerOrigin(),
		"message": map[string]any{"role": "user", "content": "Another Claude session sent a message:\n" + peerWrapped}})
	extract := extractTranscript(byteLines(userLine("Corrige le résumé, s'il te plaît", "u1"), claudeLine("Done."), asUser, claudeLine("Committing.")), summaryBudget, nil, nil)
	// The request stays the user's; the peer's message, newer, is marked as what it is. The language is the user's.
	want := "LATEST REQUEST: Corrige le résumé, s'il te plaît\nCLAUDE: Done.\nLATEST PEER MESSAGE (from peer-one): Commit both now.\nCLAUDE: Committing."
	if extract.Text != want || extract.Latest != "u1 p1" || extract.Language != "French" {
		t.Fatalf("got %q, latest %q, in %s", extract.Text, extract.Latest, extract.Language)
	}
	// Sent while the agent works: queued, its isMeta in the attachment. Only the newest is marked.
	queued := extractTranscript(byteLines(asUser, claudeLine("Committing."), queuedLine(peerWrapped, "p2", peerOrigin(), "prompt", true)), summaryBudget, nil, nil)
	if want := "PEER peer-one: Commit both now.\nCLAUDE: Committing.\nLATEST PEER MESSAGE (from peer-one): Commit both now."; queued.Text != want || queued.Latest != "p2" {
		t.Fatalf("queued: %q, latest %q", queued.Text, queued.Latest)
	}
	// One from before the request isn't marked.
	before := extractTranscript(byteLines(asUser, userLine("Now the README", "u2")), summaryBudget, nil, nil)
	if before.Text != "PEER peer-one: Commit both now.\nLATEST REQUEST: Now the README" || before.Latest != "u2" {
		t.Fatalf("before: %q, latest %q", before.Text, before.Latest)
	}
}

func handbackOrigin() map[string]any {
	return map[string]any{"kind": "peer", "name": "summary-fixer", "from": "uds:/tmp/cc-socks/2.sock", "handback": true,
		"handbackReportText": "All done, tests pass.", "body": "A background session finished:\nAll done, tests pass.", "senderTaskId": "t1"}
}

func TestAReportHandedBackIsNoRequest(t *testing.T) {
	report := jsonLine(map[string]any{"type": "user", "uuid": "r1", "isMeta": true, "promptSource": "system", "origin": handbackOrigin(),
		"message": map[string]any{"role": "user", "content": "<agent-message from=\"summary-fixer\">All done, tests pass.</agent-message>"}})
	extract := extractTranscript(byteLines(userLine("Fix the summary", "u1"), claudeLine("Sent it to a fixer."), report), summaryBudget, nil, nil)
	if want := "LATEST REQUEST: Fix the summary\nCLAUDE: Sent it to a fixer.\nREPORT from summary-fixer: All done, tests pass."; extract.Text != want || extract.Latest != "u1" {
		t.Fatalf("got %q, latest %q", extract.Text, extract.Latest)
	}
	queued := extractTranscript(byteLines(queuedLine("<agent-message>…</agent-message>", "r2", handbackOrigin(), "prompt", true)), summaryBudget, nil, nil)
	if queued.Text != "REPORT from summary-fixer: All done, tests pass." || queued.Latest != "" {
		t.Fatalf("queued: %q, latest %q", queued.Text, queued.Latest)
	}
}

func TestWhenTooLongTheRequestAndALaterPeerMessageBothLead(t *testing.T) {
	log := []string{userLine("Fix the elevator", "u1")}
	for i := 1; i <= 400; i++ {
		if i == 201 {
			log = append(log, jsonLine(map[string]any{"type": "user", "uuid": "p1", "isMeta": true, "origin": peerOrigin(),
				"message": map[string]any{"role": "user", "content": peerWrapped}}))
		}
		log = append(log, claudeLine(fmt.Sprintf("Step %d: moved one more table across, checked it, carried on.", i)))
	}
	extract := extractTranscript(byteLines(log...), 2_000, nil, nil)
	if !strings.HasPrefix(extract.Text, "LATEST REQUEST: Fix the elevator\n…\nLATEST PEER MESSAGE (from peer-one): Commit both now.\n…\n") ||
		!strings.HasSuffix(extract.Text, "Step 400: moved one more table across, checked it, carried on.") ||
		len([]rune(extract.Text)) > 2_000 || extract.Latest != "u1 p1" {
		t.Fatalf("got %d characters, latest %q, starting %q", len([]rune(extract.Text)), extract.Latest, extract.Text[:200])
	}
}

func TestANewerPeerMessageIsSummarisedAfresh(t *testing.T) {
	log := []string{userLine("Fix the elevator", "u1"), claudeLine("On it.")}
	path := transcriptFile(t, log...)
	calls := 0
	s := newSummaries(func() string { return testSummaryKey })
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) {
		calls++
		return fmt.Sprintf("Summary %d", calls), nil
	}
	if text, _ := s.Summarize("s1", path, "p"); text != "Summary 1" {
		t.Fatalf("first: %q", text)
	}
	log = append(log, queuedLine(peerWrapped, "p1", peerOrigin(), "prompt", true))
	if err := os.WriteFile(path, []byte(strings.Join(log, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if text, _ := s.Summarize("s1", path, "p"); text != "Summary 2" {
		t.Fatalf("after a peer's message: %q", text)
	}
}

func TestALineIsRedactedBeforeItsCut(t *testing.T) {
	// Each secret comes before the word that gives it away, which is past where its line is cut.
	pad := strings.Repeat
	text := extractTranscript(byteLines(
		claudeLine(pad("x", 1_480)+" hunter2a is the password"),
		jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "name": "Bash", "input": map[string]any{"command": "echo " + pad("x", 120) + " hunter2b # the password"}}}}}),
		jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "content": pad("x", 140) + " hunter2c is the password"}}}}),
		userLine(pad("x", 1_480)+" hunter2d is my password", ""),
	), summaryBudget, nil, nil).Text
	if want := "CLAUDE: [redacted]\nTOOL Bash: [redacted]\nRESULT: [redacted]\nLATEST REQUEST: [redacted]"; text != want {
		t.Fatalf("got %q", text)
	}
}

func TestAPeersNameIsRedactedToo(t *testing.T) {
	origin := peerOrigin()
	origin["name"] = "deploy-token-abc123"
	text := extractTranscript(byteLines(queuedLine(peerWrapped, "p1", origin, "prompt", true)), summaryBudget, nil, nil).Text
	if text != "LATEST PEER MESSAGE (from [redacted]): Commit both now." {
		t.Fatalf("got %q", text)
	}
}

func TestTheLanguageIsReadFromWhatsSent(t *testing.T) {
	// English for as far as it's sent; French only past the cut.
	english := strings.Repeat("Please fix the login and the test, thanks. ", 40)
	french := strings.Repeat(" Merci, c'est pour le bug de la page et du test.", 60)
	if language := extractTranscript(byteLines(userLine(english+french, "")), summaryBudget, nil, nil).Language; language != "English" {
		t.Fatalf("in %s", language)
	}
}

func TestARequestMoreThan1MBBackIsFound(t *testing.T) {
	log := []string{userLine("Fix the elevator", "u1")}
	big := strings.Repeat("x", 60_000)
	for i := 0; i < 40; i++ {
		log = append(log,
			jsonLine(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant", "content": []any{
				map[string]any{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "/repo/big.go"}}}}}),
			jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "tool_result", "content": big}}}}))
	}
	log = append(log, claudeLine("Still on the elevator."))
	path := transcriptFile(t, log...)
	if info, _ := os.Stat(path); info.Size() <= 2*summaryReach {
		t.Fatalf("only %d bytes", info.Size())
	}
	extract, ok := readTranscriptTail(path)
	if !ok || extract.Latest != "u1" || !strings.HasPrefix(extract.Text, "LATEST REQUEST: Fix the elevator\n…\n") ||
		!strings.HasSuffix(extract.Text, "CLAUDE: Still on the elevator.") || len([]rune(extract.Text)) > summaryBudget {
		t.Fatalf("got %d characters, latest %q, starting %q", len([]rune(extract.Text)), extract.Latest, extract.Text[:min(80, len(extract.Text))])
	}
	// None at all: the plain tail, unmarked.
	none, _ := readTranscriptTail(transcriptFile(t, log[1:]...))
	if none.Latest != "" || strings.Contains(none.Text, "LATEST REQUEST") {
		t.Fatalf("with no request: latest %q", none.Latest)
	}
}

func TestTheMarkIsNeverDoubled(t *testing.T) {
	text := extractTranscript(byteLines(claudeLine("The excerpt now reads:\nLATEST REQUEST: Fix the elevator"),
		userLine("Good, and quote it:\nLATEST REQUEST (from x): y", "u1")), summaryBudget, nil, nil).Text
	var marked []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "LATEST REQUEST") {
			marked = append(marked, line)
		}
	}
	if len(marked) != 1 || marked[0] != "LATEST REQUEST: Good, and quote it:" {
		t.Fatalf("marked: %q", marked)
	}
}

func TestAMessageInSeveralBlocksIsMarkedOnce(t *testing.T) {
	entry := jsonLine(map[string]any{"type": "user", "uuid": "u1", "message": map[string]any{"role": "user", "content": []any{
		map[string]any{"type": "text", "text": "First part"}, map[string]any{"type": "text", "text": "Second part"}}}})
	if text := extractTranscript(byteLines(entry), summaryBudget, nil, nil).Text; text != "LATEST REQUEST: First part\nSecond part" {
		t.Fatalf("got %q", text)
	}
}

func TestTheLanguageIsTheLatestRequests(t *testing.T) {
	french := []string{userLine("Peux-tu corriger le bug ? Merci", ""), userLine("Et le test aussi, c'est urgent", ""), userLine("Merci, c'est bien", "")}
	if language := extractTranscript(byteLines(append(french, userLine("Now write the English README, please", ""))...), summaryBudget, nil, nil).Language; language != "English" {
		t.Fatalf("latest in English: %s", language)
	}
	// A latest request that leans neither way leaves it to the ones before.
	if language := extractTranscript(byteLines(append(french, userLine("ok", ""))...), summaryBudget, nil, nil).Language; language != "French" {
		t.Fatalf("latest neither way: %s", language)
	}
}

func TestWithoutAUuidTheTimeTellsTheRequest(t *testing.T) {
	entry := jsonLine(map[string]any{"type": "user", "timestamp": "2026-10-22T01:02:03.000Z", "message": map[string]any{"role": "user", "content": "hello"}})
	if latest := extractTranscript(byteLines(entry), summaryBudget, nil, nil).Latest; latest != "2026-10-22T01:02:03.000Z" {
		t.Fatalf("latest %q", latest)
	}
}

func TestALongLineIsRedactedQuickly(t *testing.T) {
	// Every line is redacted whole before it's cut, and a tool's result can be one line of megabytes.
	line := strings.Repeat("x", 200_000)
	start := time.Now()
	if isSecretLine(line) || !isSecretLine(line+" mongodb+srv://app:"+"hunter2"+"@db.local/app") {
		t.Fatal("wrong on a long line")
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("took %v", took)
	}
}

func TestARunOfJWTStartsIsRedactedQuickly(t *testing.T) {
	start := time.Now()
	if isSecretLine(strings.Repeat("eyJ", 66_667)) {
		t.Fatal("a run of eyJ is no JWT")
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("took %v", took)
	}
	// A JWT is still caught, alone or after a word; put together here, so no file holds one whole.
	jwt := "eyJ" + "hbGciOiJIUzI1NiJ9" + "." + "eyJ" + "zdWIiOiIxMjM0In0" + "." + "c2lnbmF0dXJl"
	for _, line := range []string{jwt, "jwt " + jwt, `"` + jwt + `"`, "cookie=" + jwt} {
		if !isSecretLine(line) {
			t.Errorf("missed %q", line)
		}
	}
}

func TestAPasswordOnlyURLIsRedacted(t *testing.T) {
	if !isSecretLine("REDIS_URL=redis://:"+"hunter2"+"@cache:6379") || isSecretLine("see https://example.com/a:b@c") {
		t.Fatal("wrong on a URL")
	}
}

func TestTheHeadsFitASmallBudget(t *testing.T) {
	log := []string{userLine(strings.Repeat("Fix the elevator. ", 80), "u1")}
	for i := 1; i <= 200; i++ {
		if i == 101 {
			log = append(log, queuedLine(strings.Repeat("Commit both now. ", 80), "p1", peerOrigin(), "prompt", true))
		}
		log = append(log, claudeLine(fmt.Sprintf("Step %d: moved one more table across, checked it, carried on.", i)))
	}
	for _, budget := range []int{700, 1_200, 1_500} {
		text := extractTranscript(byteLines(log...), budget, nil, nil).Text
		if len([]rune(text)) > budget || !strings.HasPrefix(text, "LATEST REQUEST: Fix the elevator.") ||
			!strings.Contains(text, "\nLATEST PEER MESSAGE (from peer-one): ") {
			t.Fatalf("budget %d: %d characters, %q", budget, len([]rune(text)), text[:min(120, len(text))])
		}
	}
}

func TestTheRequestIsHaikuWithoutThinkingAndOnlyTheExcerpt(t *testing.T) {
	params := summaryParams(transcriptExtract{Text: "USER: hi", Language: "English"}, "api")
	body, _ := json.Marshal(params)
	text := string(body)
	for _, want := range []string{`"model":"claude-haiku-5-5"`, `"max_tokens":300`, `"type":"disabled"`, "USER: hi", "Write in English",
		"If a line starts with LATEST REQUEST, it is what the user last asked: summarise that request and the work since.",
		"if it gives the agent its current task, say what that task is in one line.",
		"Lines starting REPORT are reports other sessions handed back, not requests.",
		"Lines before it are background only: mention older work only if it is still in progress.",
		"If no line starts with LATEST REQUEST, summarise the most recent work in the excerpt."} {
		if !strings.Contains(text, want) {
			t.Errorf("the request has no %s: %s", want, text)
		}
	}
}

// The stand-in Anthropic: answers as given, and keeps what it was sent.
type standIn struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	status   int
	reply    string
}

func newStandIn(t *testing.T, status int, reply string) (*standIn, string) {
	s := &standIn{status: status, reply: reply}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, r)
		s.bodies = append(s.bodies, string(body))
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.reply)
	}))
	t.Cleanup(server.Close)
	return s, server.URL
}

// The call: only the key given, in its header; never the SDK's own lookups,
// and never retried.
func TestTheCallUsesOnlyTheKeyGivenAndIsNeverRetried(t *testing.T) {
	stand, base := newStandIn(t, http.StatusUnauthorized,
		`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key `+testSummaryKey+`"}}`)
	// What the SDK would otherwise pick up by itself: none of it may be used.
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "a-token-the-user-never-gave-smolcorp")
	t.Setenv("ANTHROPIC_API_KEY", "")

	text, err := askClaudeAt(context.Background(), base, testSummaryKey, summaryParams(transcriptExtract{Text: "USER: hi", Language: "English"}, "api"))
	if text != "" || err != errSummaryBadKey {
		t.Fatalf("a refused key gave %q, %v", text, err)
	}
	assertNoPartOfTheKey(t, err.Message, "the error")
	if strings.Contains(err.Message, "authentication_error") {
		t.Fatal("a reply's own words came through")
	}
	if len(stand.requests) != 1 {
		t.Fatalf("%d requests: a refusal is never retried", len(stand.requests))
	}
	r := stand.requests[0]
	if r.Header.Get("X-Api-Key") != testSummaryKey || r.Header.Get("Authorization") != "" {
		t.Fatalf("headers: key %v, authorization %q", r.Header.Get("X-Api-Key") == testSummaryKey, r.Header.Get("Authorization"))
	}
	assertNoPartOfTheKey(t, stand.bodies[0], "the request's body")
	assertNoPartOfTheKey(t, r.URL.String(), "the request's address")

	for status, want := range map[int]*SummaryError{403: errSummaryBadKey, 429: errSummaryBusy, 529: errSummaryBusy} {
		stand.status = status
		if _, err := askClaudeAt(context.Background(), base, testSummaryKey, summaryParams(transcriptExtract{Text: "x"}, "api")); err != want {
			t.Errorf("%d gave %v", status, err)
		}
	}
	if len(stand.requests) != 4 {
		t.Fatalf("%d requests for 4 calls: busy and refused are never retried", len(stand.requests))
	}
}

// A redirect is never followed: the key's header would go with it.
func TestARedirectIsRefusedAndTheKeyGoesNowhereElse(t *testing.T) {
	elsewhere, elsewhereURL := newStandIn(t, http.StatusOK, `{"content":[{"type":"text","text":"stolen"}]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhereURL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(server.Close)
	text, err := askClaudeAt(context.Background(), server.URL, testSummaryKey, summaryParams(transcriptExtract{Text: "x"}, "api"))
	if text != "" || err == nil {
		t.Fatalf("a redirect was followed: %q", text)
	}
	if len(elsewhere.requests) != 0 {
		t.Fatal("the request, and its key, went where the redirect pointed")
	}
	assertNoPartOfTheKey(t, err.Message, "the redirect's error")
}

// Nothing in the app's environment can point Summary anywhere but Anthropic.
func TestNothingInTheEnvironmentChangesWhereTheKeyGoes(t *testing.T) {
	t.Setenv("SMOLCORP_TEST_SUPPORT_DIR", t.TempDir())
	t.Setenv("SMOLCORP_TEST_ANTHROPIC_URL", "http://127.0.0.1:7781")
	t.Setenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:7782")
	var host string
	recorder := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		host = r.URL.Host
		return nil, errNoSecretStore // nothing leaves this computer
	})}
	client := anthropic.NewClient(append(summaryClientOptions(testSummaryKey, ""), option.WithHTTPClient(recorder))...)
	_, _ = client.Messages.New(context.Background(), summaryParams(transcriptExtract{Text: "x"}, "api"))
	if host != "api.anthropic.com" {
		t.Fatalf("the request went to %q", host)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNoMoreThanTwentySummariesInTenMinutes(t *testing.T) {
	path := transcriptFile(t, jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "hello"}}))
	calls := 0
	clock := time.Unix(1_000, 0)
	s := newSummaries(func() string { return testSummaryKey })
	s.now = func() time.Time { return clock }
	s.ask = func(context.Context, string, anthropic.MessageNewParams) (string, *SummaryError) {
		calls++
		return "Summary", nil
	}
	for i := 0; i < summaryMost; i++ {
		if text, err := s.Summarize("s"+string(rune('a'+i)), path, "p"); text != "Summary" || err != nil {
			t.Fatalf("summary %d: %q %v", i, text, err)
		}
	}
	if _, err := s.Summarize("over", path, "p"); err != errSummaryTooMany || calls != summaryMost {
		t.Fatalf("over the limit: %v after %d calls", err, calls)
	}
	if text, err := s.Summarize("sa", path, "p"); text != "Summary" || err != nil || calls != summaryMost {
		t.Fatalf("a summary kept costs no call: %q %v after %d calls", text, err, calls)
	}
	clock = clock.Add(summaryWindow)
	if _, err := s.Summarize("later", path, "p"); err != nil || calls != summaryMost+1 {
		t.Fatalf("after the window: %v after %d calls", err, calls)
	}
}

// The setup page and the phone, end to end: off by default, on only from the
// dialog and with a key, the key shown only by its hint, and nowhere else.
func TestSummaryIsOffUntilTurnedOnWithAKeyAndTheKeyGoesNowhere(t *testing.T) {
	d := newTestDeck(t)
	d.summaryKeys = newSummaryKeys(memoryStore(false), func(string) string { return "" })
	d.summaries = newSummaries(d.summaryKeys.Read)
	d.summaries.ask = func(_ context.Context, key string, _ anthropic.MessageNewParams) (string, *SummaryError) {
		if key != testSummaryKey {
			t.Errorf("the summary was made with another key")
		}
		return "Asked: fix login.\nDid: ran the tests.\nNot waiting on you.", nil
	}
	setup := func(target, body string) *httptest.ResponseRecorder {
		return serve(t, d, call{method: "POST", target: target, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}, body: body})
	}
	if d.config.SummaryOptIn {
		t.Fatal("on by default")
	}
	if w := setup("/setup/summaries?enabled=1&confirmed=1", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("turned on without a key: %d", w.Code)
	}
	if w := setup("/setup/summary/key", `{"key":"not a key"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("kept something that isn't a key: %d", w.Code)
	}
	saved := setup("/setup/summary/key", `{"key":"`+testSummaryKey+`"}`)
	if saved.Code != http.StatusOK || !strings.Contains(saved.Body.String(), "sk-ant-…xY9z") {
		t.Fatalf("saving the key gave %d %s", saved.Code, saved.Body)
	}
	assertNoPartOfTheKey(t, saved.Body.String(), "setup's state")
	if w := setup("/setup/summaries?enabled=1", ""); w.Code != http.StatusBadRequest || d.config.SummaryOptIn {
		t.Fatalf("turned on without the dialog: %d", w.Code)
	}
	on := setup("/setup/summaries?enabled=1&confirmed=1", "")
	if on.Code != http.StatusOK || !d.config.SummaryOptIn {
		t.Fatalf("turning on from the dialog gave %d", on.Code)
	}
	assertNoPartOfTheKey(t, on.Body.String(), "setup's state")
	data, _ := os.ReadFile(configFile)
	if !strings.Contains(string(data), `"summaryOptIn": true`) {
		t.Fatalf("config.json didn't keep the setting: %s", data)
	}
	assertNoPartOfTheKey(t, string(data), "config.json")

	// A phone: ✨ is offered, the summary comes back, and the key never does.
	phone := pair(t, d)
	path := transcriptFile(t, jsonLine(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "fix login"}}))
	d.mu.Lock()
	d.sessions.Apply(event("UserPromptSubmit", "transcript_path", path), Claude, tab, "", 100)
	snapshot := string(d.snapshot())
	d.mu.Unlock()
	if !strings.Contains(snapshot, `"summary":true`) {
		t.Fatalf("✨ isn't offered: %s", snapshot)
	}
	assertNoPartOfTheKey(t, snapshot, "the phone's snapshot")
	got := serve(t, d, call{method: "POST", target: "/summary?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone})
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "Asked: fix login.") {
		t.Fatalf("the summary gave %d %s", got.Code, got.Body)
	}
	assertNoPartOfTheKey(t, got.Body.String(), "the summary's reply")

	// Removed: off, and ✨ gone.
	if w := setup("/setup/summary/key?remove=1", ""); w.Code != http.StatusOK || d.config.SummaryOptIn {
		t.Fatalf("removing the key left Summary on: %d", w.Code)
	}
	d.mu.Lock()
	snapshot = string(d.snapshot())
	d.mu.Unlock()
	if strings.Contains(snapshot, `"summary":true`) {
		t.Fatal("✨ is still offered with no key")
	}
	if w := serve(t, d, call{method: "POST", target: "/summary?session=s1&t=" + phone, host: deckOnWiFi, from: aPhone}); w.Code != http.StatusForbidden {
		t.Fatalf("a summary with Summary off gave %d", w.Code)
	}
}

// With no secret store, as a headless Linux, a pasted key isn't kept anywhere,
// never in a file; ANTHROPIC_API_KEY still works.
func TestWithNoSecretStoreOnlyTheEnvironmentsKeyWorks(t *testing.T) {
	env := map[string]string{}
	keys := newSummaryKeys(memoryStore(true), func(name string) string { return env[name] })
	if err := keys.Save(testSummaryKey); err == nil || !strings.Contains(err.Message, "ANTHROPIC_API_KEY") {
		t.Fatalf("saving with no store gave %v", err)
	}
	if !keys.NoStore() || keys.Present() {
		t.Fatal("a missing store isn't said, or a key is said to be there")
	}
	env[summaryKeyEnv] = testSummaryKey
	if source, hint := keys.Shown(); source != "environment" || hint != "sk-ant-…xY9z" || keys.Read() != testSummaryKey {
		t.Fatalf("the environment's key: %s %s", source, hint)
	}
}

// keyringNotFound is the keyring's own "no such item", as a store with nothing in it answers.
func keyringNotFound() error { return keyringErrNotFound }

// The review's additions: Stripe keys, JWTs, a URL's password, a bare AWS
// secret key, and a private key from its BEGIN through its END. Each made up
// and put together here, so no file holds one whole.
func TestMoreKindsOfSecretAreRedacted(t *testing.T) {
	lines := []string{
		"STRIPE=" + "sk_" + "live_" + "4eC39HqLyjWDarjtT1zdp7dc",
		"the restricted one " + "rk_" + "live_" + "51H8abcDEF",
		"Authorization header " + "eyJ" + "hbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiIxMjM0In0" + "." + "abc123",
		"DATABASE_URL=postgres://app:" + "hunter2" + "@db.local:5432/app",
		"aws " + "wJalrXUtnFEMI/" + "K7MDENG/bPxRfiCY" + "EXAMPLEKEY",
		"-----BEGIN " + "OPENSSH PRIVATE KEY-----", "b3BlbnNzaC1rZXktdjEAAAAA", "AAAABG5vbmUAAAAEbm9uZQ",
		"-----END " + "OPENSSH PRIVATE KEY-----", "after the key",
		"commit 8d2f1c9a4b7e6d5c3b2a1f0e9d8c7b6a5f4e3d2c",
	}
	got := strings.Split(redactText(strings.Join(lines, "\n")), "\n")
	for i := 0; i < 9; i++ {
		if got[i] != redactedPlaceholder {
			t.Errorf("line %d kept: %q", i, got[i])
		}
	}
	if got[9] != "after the key" || got[10] != lines[10] {
		t.Errorf("redacted too much: %q, %q (a git commit's hash isn't a secret)", got[9], got[10])
	}
}
