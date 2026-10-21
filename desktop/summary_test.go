package desktop

import (
	"context"
	"encoding/json"
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
	extract := extractTranscript(lines, summaryBudget)
	want := "USER: corrige le bug de connexion s'il te plaît\nCLAUDE: Je regarde.\nTOOL Bash: [redacted]\nRESULT: line one"
	if extract.Text != want || extract.Language != "French" {
		t.Fatalf("got %q in %s", extract.Text, extract.Language)
	}
	assertNoPartOfTheKey(t, extract.Text, "the excerpt")
}

func TestTheRequestIsHaikuWithoutThinkingAndOnlyTheExcerpt(t *testing.T) {
	params := summaryParams(transcriptExtract{Text: "USER: hi", Language: "English"}, "api")
	body, _ := json.Marshal(params)
	text := string(body)
	for _, want := range []string{`"model":"claude-haiku-5-5"`, `"max_tokens":300`, `"type":"disabled"`, "USER: hi", "Write in English"} {
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
