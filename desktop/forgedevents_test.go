package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The forged-event table (SA-X11) through the hook itself: `smolcorp hook` (RunHook), the client every desk's agent
// runs on every system, with the environment a desk's start gives it, posting over a real connection to a deck that
// answers as the core. The deck-side rows are X10's (hookkeys_test.go); these show what a desk can make the real hook
// send. scripts/release-desktop.sh stops a release unless these pass and are in each system's build
// (scripts/forgedevents).

// hookEnvironment are the variables RunHook reads to name its key, its tab and its folder: each row starts with none.
var hookEnvironment = []string{hookKeyEnvironment, testSupportVariable, summaryRunVariable, "SMOLCORP_SESSION",
	"SAUZZEDECK_SESSION", "SMOLCORP_HEADLESS", "TMUX", "TMUX_PANE", "WEZTERM_PANE", "WEZTERM_UNIX_SOCKET",
	"KITTY_WINDOW_ID", "KITTY_LISTEN_ON", "CLAUDE_PROJECT_DIR", "GEMINI_PROJECT_DIR"}

// hookThrough runs the hook with env, as a desk's start would set it, on event, against d, and gives what d answered
// (0: nothing reached it).
func hookThrough(t *testing.T, d *Deck, env map[string]string, event string) int {
	t.Helper()
	answers := make(chan int, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// As the core's own listener takes a request from this computer.
		r.Host, r.RemoteAddr = "localhost:7710", thisComputer
		recorded := httptest.NewRecorder()
		(&router{deck: d}).ServeHTTP(recorded, r)
		if recorded.Code == http.StatusForbidden && recorded.Body.String() != hookNotYourDesk {
			t.Errorf("the refusal said %q", recorded.Body.String())
		}
		answers <- recorded.Code
		w.WriteHeader(recorded.Code)
	}))
	defer server.Close()
	for _, name := range hookEnvironment {
		t.Setenv(name, "")
	}
	for name, value := range env {
		t.Setenv(name, value)
	}
	stdin := os.Stdin
	defer func() { os.Stdin = stdin }()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	write.WriteString(event)
	write.Close()
	os.Stdin = read
	if RunHook([]string{"--port", fmt.Sprint(mustPort(t, server.URL)), "claude"}) != 0 {
		t.Fatal("the hook failed")
	}
	select {
	case code := <-answers:
		return code
	default:
		return 0
	}
}

func hookEvent(name, id, cwd string) string {
	return fmt.Sprintf(`{"hook_event_name":%q,"session_id":%q,"cwd":%q,"prompt":"hi"}`, name, id, cwd)
}

func TestADeskCantSendAnEventAsAnotherDeskThroughItsHook(t *testing.T) {
	d, keys, sockets := keyedOffice(t)
	ann, bea := keys["aaa111"], keys["bbb222"]
	d.mu.Lock()
	d.launchers["L-ann"] = &launcherTab{agent: Claude, folder: "/proj", hireID: "aaa111"}
	d.launchers["L-bea"] = &launcherTab{agent: Claude, folder: "/proj", hireID: "bbb222"}
	d.launchers["L-cy"] = &launcherTab{agent: Claude, folder: "/proj", hireID: "ccc333"}
	d.mu.Unlock()
	// The owner's sessions' key is in its file, where the hook reads it when its start gave it no key of its own.
	if saved, err := os.ReadFile(hookKeyFile); err != nil || len(saved) == 0 {
		t.Fatalf("the owner's key file: %q %v", saved, err)
	}
	inLauncher := func(key, launcher string) map[string]string {
		env := map[string]string{"SMOLCORP_SESSION": launcher, "CLAUDE_PROJECT_DIR": "/proj"}
		if key != "" {
			env[hookKeyEnvironment] = key
		}
		return env
	}

	// Each desk from its own window, with its own key.
	if got := hookThrough(t, d, inLauncher(ann, "L-ann"), hookEvent("SessionStart", "s-ann", "/proj")); got != http.StatusNoContent {
		t.Fatalf("Ann from her own window: %d", got)
	}
	if s := d.sessions.Get("s-ann"); s == nil || s.Tab != "launcher:L-ann" {
		t.Errorf("Ann's session: %+v", s)
	}
	if got := hookThrough(t, d, inLauncher(bea, "L-bea"), hookEvent("SessionStart", "s-bea", "/proj")); got != http.StatusNoContent {
		t.Fatalf("Bea from her own window: %d", got)
	}
	// Ann's hook, told it is in Bea's window: refused, and nothing made.
	if got := hookThrough(t, d, inLauncher(ann, "L-bea"), hookEvent("SessionStart", "s-forged", "/proj")); got != http.StatusForbidden || d.sessions.Get("s-forged") != nil {
		t.Errorf("Ann's hook in Bea's window: %d", got)
	}
	// From her own window, naming Bea's session: refused, and Bea's session is as it was.
	if got := hookThrough(t, d, inLauncher(ann, "L-ann"), hookEvent("UserPromptSubmit", "s-bea", "/proj")); got != http.StatusForbidden {
		t.Errorf("Ann's hook naming Bea's session: %d", got)
	}
	if s := d.sessions.Get("s-bea"); s == nil || s.Tab != "launcher:L-bea" || s.IsBusy() {
		t.Errorf("Bea's session after Ann's hook: %+v", s)
	}
	// With a key of its own the hook never sends the owner's sessions' key, which the file beside it holds: in the
	// window of a desk from before the update, where the owner's key would land, Ann's key is refused.
	if got := hookThrough(t, d, inLauncher(ann, "L-cy"), hookEvent("SessionStart", "s-ann-on-cy", "/proj")); got != http.StatusForbidden {
		t.Errorf("Ann's hook in Cy's window: %d (the owner's key was sent?)", got)
	}
	// The owner's key, from a hook given none: refused in Bea's window; in Cy's, as before.
	if got := hookThrough(t, d, inLauncher("", "L-bea"), hookEvent("SessionStart", "s-owner-on-bea", "/proj")); got != http.StatusForbidden {
		t.Errorf("the owner's key in Bea's window: %d", got)
	}
	if got := hookThrough(t, d, inLauncher("", "L-cy"), hookEvent("SessionStart", "s-cy", "/proj")); got != http.StatusNoContent {
		t.Errorf("the owner's key in Cy's window: %d", got)
	}
	// A key carrying a header of its own (Bea's window, after a line break) sends nothing at all; a made-up key and a
	// fired desk's are refused as no key.
	smuggled := ann + "\r\nX-AgentDeck-Tab: launcher:L-bea"
	if got := hookThrough(t, d, inLauncher(smuggled, "L-ann"), hookEvent("SessionStart", "s-smuggled", "/proj")); got != 0 || d.sessions.Get("s-smuggled") != nil {
		t.Errorf("a key with a header in it: %d", got)
	}
	if got := hookThrough(t, d, inLauncher("made-up-key", "L-bea"), hookEvent("SessionStart", "s-made-up", "/proj")); got != http.StatusUnauthorized {
		t.Errorf("a made-up key: %d", got)
	}

	// Off a Mac a tmux desk's hook names its pane from the environment alone (on a Mac, from its tty and only then
	// the environment, which X10's deck-side rows and the Mac's own hook tests cover).
	if runtime.GOOS == "darwin" {
		return
	}
	inPane := func(key, socket string) map[string]string {
		return map[string]string{hookKeyEnvironment: key, "TMUX": socket + ",123,0", "TMUX_PANE": "%1", "CLAUDE_PROJECT_DIR": "/proj"}
	}
	annWork := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	if got := hookThrough(t, d, inPane(ann, sockets["aaa111"]), hookEvent("SessionStart", "s-ann-pane", annWork)); got != http.StatusNoContent {
		t.Errorf("Ann from her own pane: %d", got)
	}
	if got := hookThrough(t, d, inPane(ann, sockets["bbb222"]), hookEvent("SessionStart", "s-ann-in-bea", annWork)); got != http.StatusForbidden || d.sessions.Get("s-ann-in-bea") != nil {
		t.Errorf("Ann's hook in Bea's pane: %d", got)
	}
	if got := hookThrough(t, d, inPane(bea, sockets["bbb222"]), hookEvent("UserPromptSubmit", "s-ann-pane", annWork)); got != http.StatusForbidden {
		t.Errorf("Bea's hook naming Ann's session: %d", got)
	}
}
