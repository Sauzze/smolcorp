//go:build !windows

package desktop

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The relay (SA-M10): a sandboxed plain hire's hook hands its event to its launcher, which sends it on to the app with
// the launcher's key, as its own tab and agent, whatever the hire's hook said; nothing but a POST to /hook goes on.
func TestAPlainHiresRelaySendsItsEventsOnAsItsLaunchersOwn(t *testing.T) {
	type got struct {
		header http.Header
		body   string
	}
	var events []got
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		events = append(events, got{r.Header.Clone(), r.URL.Path + " " + string(body)})
		w.WriteHeader(http.StatusAccepted)
	}))
	defer app.Close()
	key := func() (string, error) { return "fake-hook-key-2", nil }
	dir, err := os.MkdirTemp("", "hr")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "s")
	relay, err := startHookRelay(socket, relayHandler("L1", Gemini, app.URL, key, app.Client()))
	if err != nil {
		t.Fatal(err)
	}
	defer relay.close()
	client := hookRelayClient(socket)
	send := func(method, path string, header map[string]string, body string) int {
		r, _ := http.NewRequest(method, "http://relay"+path, strings.NewReader(body))
		for k, v := range header {
			r.Header.Set(k, v)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	// What the hire's hook says of where it runs, and a key of its own choosing, count for nothing.
	claims := map[string]string{"X-AgentDeck-Tab": "tmux:%1@/tmp/tmux-501/default", "X-AgentDeck-TTY": "/dev/ttys001",
		"X-AgentDeck-PID": "1", "X-AgentDeck-Agent": "claude", hookKeyHeader: "a-guess", "X-AgentDeck-At": "1234",
		"X-AgentDeck-Project": "/w/project", "X-AgentDeck-Billing": billingAPIKey, "X-AgentDeck-Parent": "/elsewhere"}
	if status := send(http.MethodPost, "/hook", claims, `{"session_id":"s1"}`); status != http.StatusAccepted {
		t.Fatalf("an event: %d", status)
	}
	if len(events) != 1 {
		t.Fatalf("the app had %d events", len(events))
	}
	h := events[0].header
	if events[0].body != `/hook {"session_id":"s1"}` || h.Get(hookKeyHeader) != "fake-hook-key-2" || h.Get("X-AgentDeck-Tab") != "launcher:L1" ||
		h.Get("X-AgentDeck-Agent") != "gemini" || h.Get("X-AgentDeck-At") != "1234" || h.Get("X-AgentDeck-Project") != "/w/project" ||
		h.Get("X-AgentDeck-Billing") != billingAPIKey || h.Get("X-AgentDeck-TTY") != "" || h.Get("X-AgentDeck-PID") != "" || h.Get("X-AgentDeck-Parent") != "" {
		t.Errorf("sent on as %v %q", h, events[0].body)
	}
	// Only a POST to /hook; a too-long event; a project or billing that isn't one.
	for _, c := range []struct {
		method, path string
		body         string
		want         int
	}{
		{http.MethodGet, "/hook", "", http.StatusNotFound},
		{http.MethodPost, "/launcher", "{}", http.StatusNotFound},
		{http.MethodPost, "/hook/../setup", "{}", http.StatusNotFound},
		{http.MethodPost, "/hook", strings.Repeat("x", mostSent+1), http.StatusRequestEntityTooLarge},
	} {
		if status := send(c.method, c.path, nil, c.body); status != c.want {
			t.Errorf("%s %s: %d, want %d", c.method, c.path, status, c.want)
		}
	}
	send(http.MethodPost, "/hook", map[string]string{"X-AgentDeck-Project": "relative/x", "X-AgentDeck-Billing": "free", "X-AgentDeck-At": "soon"}, "{}")
	if len(events) != 2 {
		t.Fatalf("the app had %d events, want 2", len(events))
	}
	if h := events[1].header; h.Get("X-AgentDeck-Project") != "" || h.Get("X-AgentDeck-Billing") != "" || h.Get("X-AgentDeck-At") == "soon" || h.Get("X-AgentDeck-At") == "" {
		t.Errorf("sent on as %v", h)
	}

	// Codex's tab is none, as its own hook says it; and with no key to send, nothing is sent.
	codex := relayHandler("L2", Codex, app.URL, key, app.Client())
	w := httptest.NewRecorder()
	codex.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader("{}")))
	if len(events) != 3 || events[2].header.Get("X-AgentDeck-Tab") != "" || events[2].header.Get("X-AgentDeck-Agent") != "codex" {
		t.Errorf("Codex's: %v", events[len(events)-1].header)
	}
	none := relayHandler("L3", Gemini, app.URL, func() (string, error) { return "", errors.New("no key") }, app.Client())
	w = httptest.NewRecorder()
	none.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader("{}")))
	if w.Code != http.StatusServiceUnavailable || len(events) != 3 {
		t.Errorf("with no key: %d, %d events", w.Code, len(events))
	}
}

// The hook uses the relay only when its desk gave it no key of its own, and only a whole path.
func TestAHookUsesItsRelayOnlyWithoutAKeyOfItsOwn(t *testing.T) {
	for _, c := range []struct {
		key, relay, want string
	}{
		{"", "/tmp/p/plain-1/s", "/tmp/p/plain-1/s"},
		{"desk-key", "/tmp/p/plain-1/s", ""},
		{"", "relative/s", ""},
		{"", "/tmp/p/../x/s", ""},
		{"", "", ""},
	} {
		env := map[string]string{hookKeyEnvironment: c.key, hookRelayEnvironment: c.relay}
		if got := hookRelayFor(func(name string) string { return env[name] }); got != c.want {
			t.Errorf("key %q, relay %q: %q, want %q", c.key, c.relay, got, c.want)
		}
	}
}

// RunHook itself, with the relay named: the event goes to the relay with no key, and never reads the key file.
func TestAHookWithARelaySendsNoKey(t *testing.T) {
	isolate(t)
	writeFile(t, hookKeyFile, "fake-owner-key\n")
	dir, err := os.MkdirTemp("", "hr")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "s")
	var seen []http.Header
	relay, err := startHookRelay(socket, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = append(seen, r.Header.Clone()) }))
	if err != nil {
		t.Fatal(err)
	}
	defer relay.close()
	t.Setenv(hookRelayEnvironment, socket)
	t.Setenv(hookKeyEnvironment, "")
	t.Setenv("SMOLCORP_SESSION", "L9")
	stdin := os.Stdin
	t.Cleanup(func() { os.Stdin = stdin })
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() { _, _ = io.Copy(w, bytes.NewBufferString(`{"session_id":"s1"}`)); w.Close() }()
	RunHook([]string{"gemini"})
	if len(seen) != 1 || seen[0].Get(hookKeyHeader) != "" || seen[0].Get("X-AgentDeck-Tab") != "launcher:L9" {
		t.Errorf("the relay saw %v", seen)
	}
}
