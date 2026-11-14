//go:build !windows

package desktop

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// On the engine tests' stand-ins (gmscope_test.go) and a real tmux (needTmux), which aren't built on Windows.

// A GM held to one project sets the preview of its own hires there alone, named as hires.
func TestAScopedGMSetsOnlyItsOwnHiresPreview(t *testing.T) {
	d, _, _, token, project := scopedGM(t)
	hires := append(registered(t), Hire{ID: "in1", Name: "Inside", Role: "builder", Folder: project, State: "working"},
		Hire{ID: "out1", Name: "Outside", Role: "builder", Folder: "/elsewhere", State: "working"})
	writeRegistry(t, hires...)
	for body, refused := range map[string]bool{
		`{"url":"http://localhost:3000","hire":"out1"}`:   true,
		`{"url":"http://localhost:3000","session":"any"}`: true,
		`{"url":"http://localhost:3000"}`:                 true,
		`{"url":"http://localhost:3000","hire":"Inside"}`: false, // let through: it has no desk, so 409
	} {
		code, text := scopedCall(t, d, token, "POST", "/setup/preview", body)
		if scopeRefused(code, text) != refused || !refused && code != http.StatusConflict {
			t.Errorf("%s: %d %q", body, code, text)
		}
	}
}

// TestPreviewStandIn is no test: it is the stand-in desk process a found test starts, this test binary run again.
// It listens on the first free one of this test's own loopback ports (7762-7799), and, given a launcher's address,
// connects there as a launcher does and holds the connection. It writes its port in its ready file, and stays until
// it is stopped, a minute at most.
func TestPreviewStandIn(t *testing.T) {
	ready := os.Getenv("PREVIEW_STANDIN_READY")
	if ready == "" {
		t.Skip("only as a found test's stand-in")
	}
	var listener net.Listener
	for port := 7762; port <= 7799 && listener == nil; port++ {
		listener, _ = net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	}
	if listener == nil {
		t.Fatal("no free test port in 7762-7799")
	}
	defer listener.Close()
	if address := os.Getenv("PREVIEW_STANDIN_LAUNCHER"); address != "" {
		request, _ := http.NewRequest(http.MethodGet, address, nil)
		request.Header.Set(hookKeyHeader, os.Getenv("PREVIEW_STANDIN_KEY"))
		response, err := http.DefaultClient.Do(request)
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("as a launcher: %v %v", response, err)
		}
		defer response.Body.Close()
	}
	if err := os.WriteFile(ready, []byte(strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Minute)
}

// standInPort waits for a stand-in's ready file, and is its address as a preview.
func standInPort(t *testing.T, ready string) string {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if port, err := os.ReadFile(ready); err == nil && len(port) > 0 {
			return "http://localhost:" + string(port)
		}
	}
	t.Fatal("the stand-in never got ready")
	return ""
}

// Found: the ports a desk's own processes listen on, from where the app itself knows they start, never the hook's
// pid. Real stand-in trees: a tmux pane on a socket of the test's own runs a stand-in that listens (found by the
// pane's process, and again by the pane's tty as a terminal tab's desk); a launcher's stand-in connects to the app
// and listens (found by the process holding that connection). A desk whose hook names a process that listens, on a
// tab the app can't find processes for, has none, and a desk with no listener has no preview.
func TestAPreviewIsFoundAmongTheDesksOwnListeners(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the system's tables are read on a Mac and on Linux here; Windows' reader is checked by its parser")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := previewDeck(t)
	d := m.d
	scratch, err := os.MkdirTemp("", "pv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(scratch) })

	// A tmux pane, as a hire's: the stand-in is the pane's own process.
	tmuxPath := needTmux(t)
	socket, paneReady := filepath.Join(scratch, "s"), filepath.Join(scratch, "pane")
	tmux := func(args ...string) string {
		out, err := exec.Command(tmuxPath, append([]string{"-S", socket, "-f", "/dev/null"}, args...)...).Output()
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	tmux("new-session", "-d", "env", "PREVIEW_STANDIN_READY="+paneReady, self, "-test.run=^TestPreviewStandIn$")
	t.Cleanup(func() { _ = exec.Command(tmuxPath, "-S", socket, "kill-server").Run() })
	inPane := standInPort(t, paneReady)
	pane, paneTTY := tmux("display-message", "-p", "#{pane_id}"), tmux("display-message", "-p", "#{pane_tty}")

	// A launcher: the stand-in connects to this app as one, and listens.
	srv := httptest.NewServer(&router{deck: d})
	defer srv.Close()
	launcherReady := filepath.Join(scratch, "launcher")
	launcher := exec.Command(self, "-test.run=^TestPreviewStandIn$")
	launcher.Env = append(os.Environ(), "PREVIEW_STANDIN_READY="+launcherReady, "PREVIEW_STANDIN_KEY="+d.access.HookKey,
		"PREVIEW_STANDIN_LAUNCHER="+srv.URL+"/launcher?id=pv-launch&agent=claude&folder=/work/l")
	if err := launcher.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = launcher.Process.Kill(); _ = launcher.Wait() }()
	inLauncher := standInPort(t, launcherReady)

	// The hook says this process for a desk in iTerm2, which the app finds no processes for: never believed.
	hook := serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: d.access.HookKey, "X-AgentDeck-Agent": "claude",
			"X-AgentDeck-Tab": "iterm:ABCD-1234", "X-AgentDeck-PID": strconv.Itoa(launcher.Process.Pid)},
		body: `{"hook_event_name":"UserPromptSubmit","session_id":"pv-forged","cwd":"/work/forged"}`})
	d.mu.Lock()
	forged := d.sessions.Get("pv-forged")
	if hook.Code != http.StatusNoContent || forged == nil || forged.PID != 0 {
		d.mu.Unlock()
		t.Fatalf("the hook's pid for a desk that isn't headless was kept: %d %+v", hook.Code, forged)
	}
	d.sessions.Get("pv-rev").Tab = "tmux:" + pane + "@" + socket
	d.sessions.Get("pv-jan").Tab = "launcher:pv-launch"
	d.sessions.byID["pv-tty"] = &Session{ID: "pv-tty", Agent: Claude, Folder: "/work/tty", Tab: "tty:" + paneTTY, StartedAt: 3}
	d.sessions.byID["pv-none"] = &Session{ID: "pv-none", Agent: Claude, Folder: "/work/none", Tab: "tmux:%999@" + socket, StartedAt: 4}
	d.mu.Unlock()
	t.Cleanup(func() { previews.forget("pv-tty"); previews.forget("pv-none"); previews.forget("pv-forged") })

	// In this order: the terminal tab's desk first, by its tty alone.
	for _, desk := range []struct{ session, want string }{{"pv-tty", inPane}, {"pv-rev", inPane}, {"pv-jan", inLauncher}} {
		code, a := windowPreview(t, d, desk.session)
		if code != 200 || a.Source != "found" || !slices.Equal(a.Found, []string{desk.want}) || a.URL != desk.want {
			t.Errorf("%s: %d %+v, want %s alone", desk.session, code, a, desk.want)
		}
	}
	// The pane's process now known to start pv-rev's desk, the tty's desk leaves it out: it's that desk's.
	previews.forget("pv-tty")
	if _, a := windowPreview(t, d, "pv-tty"); len(a.Found) != 0 {
		t.Errorf("another desk's process is still the tty desk's: %+v", a)
	}
	for _, session := range []string{"pv-forged", "pv-none"} {
		if _, a := windowPreview(t, d, session); a.URL != "" || a.Source != "" || len(a.Found) != 0 {
			t.Errorf("%s has a preview: %+v", session, a)
		}
	}
	// Declared wins; found stays the suggestion.
	if code := m.call(t, asPage, "POST", "/setup/preview", `{"url":"http://localhost:3999","session":"pv-rev"}`).Code; code != 200 {
		t.Fatal("declare")
	}
	if _, a := windowPreview(t, d, "pv-rev"); a.URL != "http://localhost:3999" || a.Source != "declared" || !slices.Contains(a.Found, inPane) {
		t.Errorf("declared and found: %+v", a)
	}
}
