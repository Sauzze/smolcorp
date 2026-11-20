package desktop

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/gm"
)

// The CLI for people (1.26 X3: status and desks) against the REAL router, with the key it really gets (1.26, SK2: it
// asks, and the owner allows it in Setup with the code): what each command asks must be a route the owner's tool may
// use, and the answer must be the app's, not a refusal.

type seenCall struct {
	method, path string
	header       http.Header
	status       int
}

// recordingRouter is the real router, with each request and its status kept.
type recordingRouter struct {
	deck *Deck
	mu   sync.Mutex
	seen []seenCall
}

func (rr *recordingRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := httptest.NewRecorder()
	(&router{deck: rr.deck}).ServeHTTP(rec, r)
	for k, v := range rec.Header() {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.Code)
	w.Write(rec.Body.Bytes())
	rr.mu.Lock()
	rr.seen = append(rr.seen, seenCall{r.Method, r.URL.Path, r.Header.Clone(), rec.Code})
	rr.mu.Unlock()
}

func TestStatusAndDesksAskOnlyRoutesAToolMayUseOfTheRealRouter(t *testing.T) {
	// Under the Mac app: off a Mac no terminal is given a key (TestOffAMacNoTerminalIsGivenAKey).
	d := newMacTestDeck(t)
	managerOn(d)
	rr := &recordingRouter{deck: d}
	server := httptest.NewServer(rr)
	defer server.Close()
	// Old key files are in its folder, and a GM token and a session id are in the environment: the commands must send
	// the key the owner allowed them and nothing else.
	gmToken := d.newGMToken()
	if err := os.WriteFile(filepath.Join(supportDir, "setup-key"), []byte("fake-setup-key-0123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(supportDir, "window-key"), []byte("fake-window-key-4567\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{
		"SMOLCORP_PORT":          server.URL[strings.LastIndex(server.URL, ":")+1:],
		"SMOLCORP_GM_TOKEN":      gmToken,
		"CLAUDE_CODE_SESSION_ID": "11111111-2222-3333-4444-555555555555",
	}
	env := gm.Env{
		Getenv:     func(k string) string { return vars[k] },
		SupportDir: func() (string, error) { return supportDir, nil },
	}
	// The owner, in Smolcorp's Settings window: each ask is answered Allow, with the code the page shows.
	savedWait := askWait
	askWait = 20 * time.Millisecond
	defer func() { askWait = savedWait }()
	d.mu.Lock()
	page := d.access.NewSetupSession()
	d.mu.Unlock()
	stop, allowed := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(allowed)
		for {
			for _, ask := range d.asks.waiting() {
				request, _ := http.NewRequest(http.MethodPost, server.URL+"/setup/terminal/answer?id="+url.QueryEscape(ask["id"].(string))+
					"&code="+url.QueryEscape(ask["code"].(string))+"&answer=allow", nil)
				request.Host = "127.0.0.1"
				request.Header.Set(setupHeader, "1")
				request.AddCookie(&http.Cookie{Name: setupCookie(), Value: page})
				if response, err := server.Client().Do(request); err == nil {
					response.Body.Close()
				}
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	defer func() { close(stop); <-allowed }()
	run := func(cmd string, args ...string) (int, string, string) {
		var out, errOut strings.Builder
		code := gm.RunPeopleIn(env, cmd, args, &out, &errOut)
		return code, out.String(), errOut.String()
	}
	for _, c := range [][]string{{"status"}, {"status", "--json"}, {"desks"}, {"desks", "--json"}} {
		if code, out, errOut := run(c[0], c[1:]...); code != 0 || out == "" || strings.Contains(errOut, "smolcorp:") || !strings.Contains(errOut, "Check that it shows this code") {
			t.Errorf("%v: exit %d %q %q", c, code, out, errOut)
		}
	}
	d.mu.Lock()
	given := slices.Clone(d.access.toolKeys)
	d.mu.Unlock()
	if len(given) != 4 || slices.Contains(given, d.access.SetupKey) {
		t.Fatalf("%d keys given, one the setup key itself %v", len(given), slices.Contains(given, d.access.SetupKey))
	}
	reads := 0
	for _, s := range rr.seen {
		if s.path == terminalAskPath || s.path == "/setup/terminal/answer" {
			continue // the ask and the owner's answer
		}
		reads++
		row := routeFor(s.method, s.path)
		if row == nil || !slices.Contains(row.callers, "tool") || !strings.HasPrefix(s.path, "/setup/") {
			t.Errorf("%s %s is not a setup route the table lets a tool use: %v", s.method, s.path, row)
		}
		if s.status != http.StatusOK {
			t.Errorf("%s %s was answered %d, not served", s.method, s.path, s.status)
		}
		// The key it was given alone: no GM token, no GM session, no window key, not the old file's.
		for _, h := range []string{gmTokenHeader, gmSessionHeader, "X-AgentDeck-Token"} {
			if s.header.Get(h) != "" {
				t.Errorf("%s %s sent %s", s.method, s.path, h)
			}
		}
		if !slices.Contains(given, s.header.Get(setupKeyHeader)) {
			t.Errorf("%s %s did not send the key it was given", s.method, s.path)
		}
	}
	if reads < 4 {
		t.Fatalf("only %d reads reached the router", reads)
	}
	rr.mu.Lock()
	before := len(rr.seen)
	rr.mu.Unlock()
	// say, open and hire make no request at all, with every key in place.
	for _, cmd := range []string{"say", "open", "hire"} {
		if code, out, errOut := run(cmd, "ada", "hello"); code != 1 || out != "" || !strings.Contains(errOut, "comes with app keys in Settings") {
			t.Errorf("%s: exit %d %q %q", cmd, code, out, errOut)
		}
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if len(rr.seen) != before {
		t.Errorf("say, open or hire reached the router: %d requests", len(rr.seen)-before)
	}
}
