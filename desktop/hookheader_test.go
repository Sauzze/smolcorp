package desktop

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// macSupportFolder has the support folder as the Mac app's engine has it (useSupportDir): with hook-header.
func macSupportFolder(t *testing.T) {
	t.Helper()
	isolate(t)
	t.Cleanup(func() { hookHeaderFile = "" })
	hookHeaderFile = filepath.Join(supportDir, "hook-header")
}

// An upgrade from the Swift engine: the key in its hook-header is kept, so every agent it hooked reports on.
func TestUnderTheMacAppTheSwiftEnginesHookKeyIsKept(t *testing.T) {
	macSupportFolder(t)
	swift := "X-AgentDeck-Token: c3dpZnQtbWFkZS1rZXk\n"
	if err := os.WriteFile(hookHeaderFile, []byte(swift), 0o600); err != nil {
		t.Fatal(err)
	}
	a := NewAccess()
	if a.HookKey != "c3dpZnQtbWFkZS1rZXk" || !a.IsHookKey("c3dpZnQtbWFkZS1rZXk") {
		t.Fatalf("the hook key is %q, not the Swift engine's", a.HookKey)
	}
	if saved, _ := os.ReadFile(hookHeaderFile); string(saved) != swift {
		t.Errorf("hook-header became %q", saved)
	}
	// The engine's own hook reads hook-key: the same key.
	if saved, _ := os.ReadFile(hookKeyFile); string(saved) != "c3dpZnQtbWFkZS1rZXk\n" {
		t.Errorf("hook-key is %q", saved)
	}
}

// A first start under the Mac app writes hook-header as the Swift engine does: the header's line, yours alone.
func TestUnderTheMacAppTheHookKeyIsWrittenAsTheMacsHookHeader(t *testing.T) {
	macSupportFolder(t)
	// One that isn't the Mac's line isn't believed.
	if err := os.WriteFile(hookHeaderFile, []byte("X-Other: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewAccess()
	saved, err := os.ReadFile(hookHeaderFile)
	if err != nil || string(saved) != hookKeyHeader+": "+a.HookKey+"\n" || len(a.HookKey) < 32 {
		t.Fatalf("hook-header is %q (%v) for key %q", saved, err, a.HookKey)
	}
	if info, err := os.Stat(hookHeaderFile); err != nil || !modeIs(info, 0o600) {
		t.Fatalf("hook-header's mode: %v %v", info, err)
	}
	if NewAccess().HookKey != a.HookKey {
		t.Error("the hook key changed from one start to the next")
	}
	// Elsewhere there is none.
	isolate(t)
	hookHeaderFile = ""
	NewAccess()
	if _, err := os.Stat(filepath.Join(supportDir, "hook-header")); err == nil {
		t.Error("hook-header written outside the Mac app")
	}
}

// The Mac app's real report.sh, as an agent hooked by the Swift engine runs it, reaches the Go core with a key it takes.
func TestTheMacsReportShReachesTheGoCore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("report.sh is the Mac's")
	}
	for _, tool := range []string{"bash", "curl", "perl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	macSupportFolder(t)
	a := NewAccess()
	got := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		got <- r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	at, _ := url.Parse(server.URL)
	template, err := os.ReadFile("../Resources/report.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(supportDir, "report.sh")
	if err := os.WriteFile(script, bytes.ReplaceAll(template, []byte("{{PORT}}"), []byte(at.Port())), 0o700); err != nil {
		t.Fatal(err)
	}
	// Headless, so it reports with or without a tty above this test; nothing else of this computer's.
	run := exec.Command("bash", script, "claude")
	run.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + homeDir, "SMOLCORP_HEADLESS=1"}
	run.Stdin = strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"fxc2-e2e"}`)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("report.sh: %v %s", err, out)
	}
	select {
	case r := <-got:
		if !a.IsHookKey(r.Header.Get(hookKeyHeader)) {
			t.Fatal("report.sh's key isn't the Go core's")
		}
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/hook" || !strings.Contains(string(body), "fxc2-e2e") {
			t.Errorf("report.sh sent %s %s", r.URL.Path, body)
		}
	default:
		t.Fatal("report.sh sent nothing")
	}
}

// hookWith posts to /hook from this computer with the hook's key, and gives each header as many times as it lists.
func hookWith(t *testing.T, d *Deck, headers map[string][]string) int {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"twice","cwd":"/work/a"}`))
	r.Host, r.RemoteAddr = "localhost:7710", thisComputer
	r.Header.Set(hookKeyHeader, d.access.HookKey)
	for name, values := range headers {
		for _, value := range values {
			r.Header.Add(name, value)
		}
	}
	w := httptest.NewRecorder()
	(&router{deck: d}).ServeHTTP(w, r)
	if w.Code == http.StatusBadRequest && w.Body.String() != "A hook header came twice" {
		t.Errorf("the refusal said %q", w.Body.String())
	}
	return w.Code
}

// A hook header that comes twice is one smuggled in after another, as a line break in a variable the hook was given
// would make it: refused (400, as the Mac's), whichever of them is repeated, and nothing is taken from the request.
// The same on every OS: the Go core's own hook never sends one twice.
func TestAHookHeaderSentTwiceIsRefused(t *testing.T) {
	for _, mac := range []bool{false, true} {
		if mac {
			withMac(t)
		}
		d := newTestDeck(t)
		for _, name := range []string{"X-AgentDeck-Tab", "X-AgentDeck-Agent", "X-AgentDeck-Project", "X-AgentDeck-PID"} {
			if got := hookWith(t, d, map[string][]string{name: {"one", "two"}}); got != http.StatusBadRequest {
				t.Errorf("mac %v: %s twice: %d", mac, name, got)
			}
		}
		if len(d.sessions.All()) != 0 {
			t.Errorf("mac %v: a refused hook made a session", mac)
		}
		// Once is as it was; a header that isn't a hook's may come twice.
		if got := hookWith(t, d, map[string][]string{"X-AgentDeck-Tab": {"launcher:one"}, "Accept": {"a", "b"}}); got != http.StatusNoContent {
			t.Errorf("mac %v: one header each: %d", mac, got)
		}
		// Without the key it is 401 first, as the Mac checks the key before the headers.
		r := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{}`))
		r.Host, r.RemoteAddr = "localhost:7710", thisComputer
		r.Header.Add("X-AgentDeck-Tab", "a")
		r.Header.Add("X-AgentDeck-Tab", "b")
		w := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("mac %v: no key: %d", mac, w.Code)
		}
	}
}
