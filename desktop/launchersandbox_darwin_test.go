package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aymanbagabas/go-pty"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-M10 on a Mac: a plain hire's agent, started by the real launcher in a process of its own, runs in its sandbox
// through the real /usr/bin/sandbox-exec, its profile and `enter` (owner Q5 (a): stand-ins only). A stand-in agent
// tries a stand-in secret and a stand-in fence in the launcher's support folder, says what it was started with, and
// reports one event through its hook. The same agent run by hand (`smolcorp run`, no hire) is the control: there it
// reads the secret, so the probe can fail.

// plainProbeAgent writes a stand-in agent called name into a folder of its own, which says version to --version, and
// otherwise writes what it found into ./probe in the folder it runs in.
func plainProbeAgent(t *testing.T, name, version string) string {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = --version ]; then echo '` + version + `'; exit 0; fi
{
printf 'args:%s\n' "$*"
printf 'gemini-sandbox:%s\n' "${GEMINI_SANDBOX-unset}"
printf 'relay:%s\n' "${SMOLCORP_HOOK_RELAY:+set}"
printf 'hook-key:%s\n' "${SMOLCORP_HOOK_KEY:+set}"
if /bin/cat "$SMOLCORP_TEST_SECRET" >/dev/null 2>&1; then echo secret:read; else echo secret:denied; fi
if ( : >> "$SMOLCORP_TEST_FENCE" ) 2>/dev/null; then echo fence:written; else echo fence:denied; fi
if echo '{"hook_event_name":"Stop","session_id":"s1"}' | "$SMOLCORP_TEST_SELF" hook ` + name + `; then echo hook:ran; fi
} > ./probe 2>&1
`
	writeFile(t, filepath.Join(bin, name), script)
	if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// appStandIn is the app's /hook, as a plain hire's relay reaches it: each event's headers and body are kept.
type appStandIn struct {
	mu     sync.Mutex
	events []http.Header
	bodies []string
}

func (a *appStandIn) serve(t *testing.T) int {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hook" {
			http.NotFound(w, r) // the launcher's own stream (/launcher) isn't this test's
			return
		}
		body, _ := io.ReadAll(r.Body)
		a.mu.Lock()
		a.events, a.bodies = append(a.events, r.Header.Clone()), append(a.bodies, string(body))
		a.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	port, _ := strconv.Atoi(server.URL[strings.LastIndex(server.URL, ":")+1:])
	return port
}

// runPlainLauncher runs `smolcorp run args…`, the real launcher, in folder, with bin first on its PATH, as a plain
// hire's window runs it, and gives what the agent wrote down and what the launcher said.
func runPlainLauncher(t *testing.T, args []string, bin, folder string, more ...string) (probe map[string]string, said string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), launcherStandInVariable+"="+string(must(json.Marshal(args))),
		"PATH="+bin+string(os.PathListSeparator)+"/usr/bin:/bin", deskStandInVariable+"=1", "SMOLCORP_TEST_SELF="+os.Args[0])
	cmd.Env = append(cmd.Env, more...)
	cmd.Dir = folder
	output, _ := cmd.CombinedOutput()
	probe = map[string]string{}
	if text, err := os.ReadFile(filepath.Join(folder, "probe")); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(text)), "\n") {
			if key, value, ok := strings.Cut(line, ":"); ok {
				probe[key] = value
			}
		}
	}
	return probe, string(output)
}

// macSandboxHere skips (or, on the runner, fails) where this Mac's sandbox-exec isn't Apple's alone.
func macSandboxHere(t *testing.T) {
	t.Helper()
	if err := sandbox.CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(err)
		}
		t.Skipf("this Mac's sandbox-exec isn't Apple's alone: %v", err)
	}
}

func TestOnAMacAPlainHiresAgentRunsInItsSandboxAndReportsThroughItsLauncher(t *testing.T) {
	macSandboxHere(t)
	token := strings.Repeat("AB", 16)
	app := &appStandIn{}
	port := app.serve(t)
	for _, c := range []struct {
		row, agent, version string
		args                []string // what the agent is started with
		geminiSandbox       string
	}{
		{"Gemini CLI", "gemini", "0.9.0", nil, "false"},
		{"Codex", "codex", "codex-cli 0.46.0", []string{"-c", `sandbox_mode="danger-full-access"`}, "true"}, // not Codex's to change
	} {
		t.Run(c.row, func(t *testing.T) {
			bin, folder, sockets := plainProbeAgent(t, c.agent, c.version), t.TempDir(), shortDeskSockets(t)
			env := []string{testDeskSocketsVariable + "=" + sockets, testHookKeyVariable + "=fake-hook-key-1", testSecretVariable + "=1",
				"SMOLCORP_PORT=" + strconv.Itoa(port), "GEMINI_SANDBOX=true"}
			probe, said := runPlainLauncher(t, []string{"--hire", token, c.agent}, bin, folder, env...)
			if probe["secret"] != "denied" || probe["fence"] != "denied" {
				t.Fatalf("in its sandbox the agent found the secret %q and the fence %q (the launcher said %q)", probe["secret"], probe["fence"], said)
			}
			if probe["args"] != strings.Join(c.args, " ") || probe["gemini-sandbox"] != c.geminiSandbox {
				t.Errorf("its own sandbox: started with %q and GEMINI_SANDBOX %q, want %q and %q", probe["args"], probe["gemini-sandbox"], c.args, c.geminiSandbox)
			}
			if probe["relay"] != "set" || probe["hook-key"] != "" || probe["hook"] != "ran" {
				t.Errorf("its hook: %+v", probe)
			}
			// What it planted on the way is gone with it: its own folder in the socket folder.
			if left, _ := filepath.Glob(filepath.Join(sockets, "*")); len(left) != 0 {
				t.Errorf("left in the socket folder: %q", left)
			}
		})
	}
	// Each event reached the app as its launcher's own: the launcher's key, its own agent, its own tab (none for
	// Codex, as its hook says), whatever the hook would have named.
	app.mu.Lock()
	defer app.mu.Unlock()
	if len(app.events) != 2 {
		t.Fatalf("the app had %d events, want 2: %q", len(app.events), app.bodies)
	}
	for i, agent := range []string{"gemini", "codex"} {
		h := app.events[i]
		tab := h.Get("X-AgentDeck-Tab")
		if h.Get(hookKeyHeader) != "fake-hook-key-1" || h.Get("X-AgentDeck-Agent") != agent || !strings.Contains(app.bodies[i], `"session_id":"s1"`) ||
			agent == "gemini" && !strings.HasPrefix(tab, "launcher:") || agent == "codex" && tab != "" {
			t.Errorf("%s's event: %v %q", agent, h, app.bodies[i])
		}
	}
}

// The control: the same agent, run by hand, is in no sandbox, and reads the secret. And a plain hire's agent too old
// to switch its own sandbox off is refused before anything of it starts.
func TestOnAMacTheProbeReadsTheSecretOutsideTheSandboxAndAnOldAgentIsRefused(t *testing.T) {
	macSandboxHere(t)
	bin, folder := plainProbeAgent(t, "gemini", "0.9.0"), t.TempDir()
	env := []string{testDeskSocketsVariable + "=" + shortDeskSockets(t), testSecretVariable + "=1"}
	if probe, said := runPlainLauncher(t, []string{"gemini"}, bin, folder, env...); probe["secret"] != "read" || probe["relay"] != "" {
		t.Fatalf("by hand: %+v (the launcher said %q)", probe, said)
	}
	old := plainProbeAgent(t, "codex", "codex-cli 0.19.9")
	folder = t.TempDir()
	probe, said := runPlainLauncher(t, []string{"--hire", strings.Repeat("AB", 16), "codex"}, old, folder, env...)
	if len(probe) != 0 || !strings.Contains(said, "Update Codex: a desk in its sandbox needs 0.20.0 or later") {
		t.Errorf("an old Codex: %+v, said %q", probe, said)
	}
}

// Each step of newLauncherSandbox refuses with the plan's words and leaves nothing behind: something already at its
// place, and a start whose handshake never comes.
func TestOnAMacAPlainHiresSandboxRefusesAndLeavesNothing(t *testing.T) {
	shortSupport(t)
	homeDir = t.TempDir()
	deskSockets(t)
	folder := t.TempDir()
	program := filepath.Join(t.TempDir(), "gemini")
	writeFile(t, program, "#!/bin/sh\n")
	left := func() []string {
		found, _ := filepath.Glob(filepath.Join(deskSocketParent(), "plain-*"))
		more, _ := filepath.Glob(filepath.Join(supportDir, "hires", "plain-*"))
		return append(append(found, more...), leftCanaries()...)
	}

	// A place already taken: refused before anything is made, and what was there stays.
	savedID := plainHireIDFor
	t.Cleanup(func() { plainHireIDFor = savedID })
	plainHireIDFor = func() string { return "plain-000000000001" }
	taken := filepath.Dir(deskSocketPath("plain-000000000001", 0))
	if err := os.MkdirAll(taken, 0o700); err != nil {
		t.Fatal(err)
	}
	if s, err := newLauncherSandbox(folder, false, "L1", Gemini, program); s != nil || err == nil || err.Error() != sandbox.NotOurServerRefusal {
		t.Fatalf("a taken place: %v %v", s, err)
	}
	if _, err := os.Lstat(taken); err != nil {
		t.Errorf("what was at its place was taken away: %v", err)
	}
	os.Remove(taken)

	// No handshake: refused in §A5.2's words, what started is ended, and its folders and canary are gone at close.
	plainHireIDFor = savedID
	s, err := newLauncherSandbox(folder, false, "L1", Gemini, program)
	if err != nil {
		t.Fatalf("made: %v", err)
	}
	profile, socket := s.profile, s.socket
	text, _ := os.ReadFile(profile)
	if !strings.Contains(string(text), sbLiteral(socket)) || !strings.Contains(string(text), "(deny signal)") {
		t.Errorf("its profile lacks its own socket or the signal rule:\n%s", text)
	}
	// Its relay answers on its own socket.
	if conn, err := net.Dial("unix", socket); err != nil {
		t.Errorf("its relay: %v", err)
	} else {
		conn.Close()
	}
	saved := launcherStartCommand
	t.Cleanup(func() { launcherStartCommand = saved })
	var made []string
	launcherStartCommand = func(st *sandbox.Start, profile, smolcorp string, env []string, program string, args ...string) (*exec.Cmd, error) {
		made = append([]string{profile, program}, args...)
		return exec.Command("/bin/sleep", "30"), nil // never hands its report back
	}
	terminal, err := pty.New()
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	cmd := terminal.Command(program, "--x")
	cmd.Dir = folder
	agent, err := s.command(cmd, terminal)
	if err != nil {
		t.Fatal(err)
	}
	savedWait := sandboxStartWait
	t.Cleanup(func() { sandboxStartWait = savedWait })
	sandboxStartWait = 200 * time.Millisecond
	if err := agent.Start(); err == nil || err.Error() != sandbox.Refusal("darwin") {
		t.Errorf("no handshake: %v", err)
	}
	if len(made) != 3 || made[0] != profile || made[1] != program || made[2] != "--x" {
		t.Errorf("the start was %q", made)
	}
	if p := agent.process(); p == nil || !errors.Is(p.Signal(syscall.Signal(0)), os.ErrProcessDone) {
		t.Errorf("what started is still running")
	}
	s.close()
	if l := left(); len(l) != 0 {
		t.Errorf("left behind: %q", l)
	}
}

// A plain hire's window closed under it (SIGHUP to its launcher), or its launcher killed (SIGTERM): what its sandbox
// made is taken away all the same.
func TestOnAMacAPlainHiresSandboxIsTakenAwayWhenItsWindowCloses(t *testing.T) {
	macSandboxHere(t)
	for _, end := range []syscall.Signal{syscall.SIGHUP, syscall.SIGTERM} {
		bin, folder, sockets := t.TempDir(), t.TempDir(), shortDeskSockets(t)
		writeFile(t, filepath.Join(bin, "gemini"), "#!/bin/sh\n[ \"$1\" = --version ] && { echo 0.9.0; exit 0; }\n: > ./started\nexec /bin/sleep 30\n")
		if err := os.Chmod(filepath.Join(bin, "gemini"), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), launcherStandInVariable+"="+string(must(json.Marshal([]string{"--hire", strings.Repeat("AB", 16), "gemini"}))),
			"PATH="+bin+":/usr/bin:/bin", deskStandInVariable+"=1", testDeskSocketsVariable+"="+sockets)
		cmd.Dir = folder
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100; i++ {
			if _, err := os.Stat(filepath.Join(folder, "started")); err == nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		made, _ := filepath.Glob(filepath.Join(sockets, "plain-*"))
		if len(made) != 1 {
			cmd.Process.Kill()
			cmd.Wait()
			t.Fatalf("%v: the agent didn't start in its sandbox: %q", end, made)
		}
		cmd.Process.Signal(end)
		cmd.Wait()
		if left, _ := filepath.Glob(filepath.Join(sockets, "*")); len(left) != 0 {
			t.Errorf("%v: left in the socket folder: %q", end, left)
		}
	}
}
