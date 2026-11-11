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
