package desktop

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// The Mac's start-only runner (SA-M6): each step's refusal with §A5.2's words and nothing left behind, with stand-ins;
// then one start end to end through the real /usr/bin/sandbox-exec, the desk's real profile and `enter`, on stand-ins
// of tmux and its server (owner Q5 (a), as SA-M3's tests).

// standInDesk is a desk's sandbox in temporary folders: its Places, Policy and own folder, made as its start makes them.
func standInDesk(t *testing.T) *deskSandbox {
	t.Helper()
	shortSupport(t)
	homeDir = t.TempDir()
	deskSockets(t)
	root := t.TempDir()
	if err := os.MkdirAll(hireFolder(supportDir, "ab12"), 0o700); err != nil {
		t.Fatal(err)
	}
	s, err := makeDeskSandbox(deskAt{ID: "ab12", Root: root, Socket: deskSocketPath("ab12", 0), Git: runGit})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// standInCmd is a start's command that never runs: it ends when it's killed, or at once with end.
type standInCmd struct {
	started, killed bool
	end             error
	done            chan struct{}
}

func (c *standInCmd) Start() error { c.started = true; return nil }
func (c *standInCmd) Wait() error {
	if c.end != nil {
		return c.end
	}
	<-c.done
	return errors.New("killed")
}
func (c *standInCmd) Kill() error                    { c.killed = true; close(c.done); return nil }
func (c *standInCmd) SetOutput(io.Writer, io.Writer) {}

// runnerWith has the start runner make its command with command, for the test.
func runnerWith(t *testing.T, command func(*sandbox.Start, string, string, []string, string, ...string) (startedCmd, error)) {
	t.Helper()
	saved := deskStartRunner
	deskStartRunner = deskStartRun{command: command, serverIsOurs: func(string, sandbox.Canary) (int, error) {
		t.Error("the server was checked after a start that didn't hold")
		return 0, errors.New("no")
	}}
	t.Cleanup(func() { deskStartRunner = saved })
}

// leftCanaries are the canaries in the support folder.
func leftCanaries() []string {
	found, _ := filepath.Glob(filepath.Join(supportDir, "sandbox-canary-*"))
	return found
}

func TestOnAMacEachStepOfASandboxedStartRefusesWithThePlansWords(t *testing.T) {
	s := standInDesk(t)
	socket := filepath.Join(s.places.OwnSockets, deskSocketFile)
	var made []string
	var gave []string
	cmd := &standInCmd{done: make(chan struct{})}
	runnerWith(t, func(st *sandbox.Start, profile, smolcorp string, env []string, program string, args ...string) (startedCmd, error) {
		made = append([]string{profile, smolcorp, program}, args...)
		gave = env
		return cmd, nil
	})
	t.Setenv("SMOLCORP_TMUX_SERVER", "4242")
	args := []string{"-S", socket, "new-session", "-d"}

	// Something at its place already: refused before anything is made.
	os.MkdirAll(filepath.Dir(socket), 0o700)
	if _, _, err := s.starter("/app/tmux", "/app/smolcorp")(args); err == nil || err.Error() != sandbox.NotOurServerRefusal || made != nil {
		t.Fatalf("a taken place: %v %q", err, made)
	}
	os.Remove(filepath.Dir(socket))

	// No handshake: the sandbox didn't hold. Its folder and canary go; its command was killed.
	_, _, err := s.starter("/app/tmux", "/app/smolcorp")(args)
	if err == nil || err.Error() != sandbox.Refusal("darwin") || !cmd.started || !cmd.killed {
		t.Fatalf("no handshake: %v %+v", err, cmd)
	}
	profile := filepath.Join(hireFolder(supportDir, "ab12"), "sandbox.sb")
	if want := append([]string{profile, "/app/smolcorp", "/app/tmux", "-f", os.DevNull}, args...); !slices.Equal(made, want) {
		t.Errorf("the start was %q, not %q", made, want)
	}
	if slices.ContainsFunc(gave, func(kv string) bool { return strings.HasPrefix(kv, "SMOLCORP_TMUX_SERVER=") }) {
		t.Errorf("the core's own guard went to the start: %q", gave)
	}
	if _, err := os.Lstat(filepath.Dir(socket)); !errors.Is(err, os.ErrNotExist) || len(leftCanaries()) != 0 {
		t.Errorf("left behind: the folder (%v), canaries %q", err, leftCanaries())
	}
	// The profile it ran under is the desk's, with what it may reach (RT126SAM2R2 N1: the ports' lines are in it).
	text, err := os.ReadFile(profile)
	info, _ := os.Stat(profile)
	for _, want := range []string{`(remote tcp4 "localhost:22")`, `(remote tcp6 "*:2375")`, "(deny signal)", "(deny mach-lookup)",
		sbLiteral(socket), sbLiteral(s.places.DeskStart)} {
		if err != nil || !strings.Contains(string(text), want) {
			t.Errorf("the profile lacks %s: %v", want, err)
		}
	}
	if info == nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the profile's mode: %v", info)
	}

	// sandbox-exec missing or not Apple's alone, or tmux's start failing: refused the same ways.
	runnerWith(t, func(*sandbox.Start, string, string, []string, string, ...string) (startedCmd, error) {
		return nil, sandbox.ErrNoSandboxExec
	})
	if _, _, err := s.starter("/app/tmux", "/app/smolcorp")(args); err == nil || err.Error() != sandbox.SandboxExecRefusal {
		t.Errorf("no sandbox-exec: %v", err)
	}
	if _, err := os.Lstat(filepath.Dir(socket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its folder stayed: %v", err)
	}
}

// sbLiteral is path as the profile names it.
func sbLiteral(path string) string { return `"` + path + `"` }

func TestOnAMacADeskStartsInItsRealSandboxAndOnlyItsCheckedServerIsKept(t *testing.T) {
	if err := sandbox.CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(err)
		}
		t.Skipf("this Mac's sandbox-exec isn't Apple's alone: %v", err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskStandInVariable, "1")
	s := standInDesk(t)
	socket := filepath.Join(s.places.OwnSockets, deskSocketFile)
	out, server, err := s.starter(self, self)([]string{"-S", socket, "start-server", ";", "new-session", "-d"})
	if err != nil {
		t.Fatalf("the start was refused: %v", err)
	}
	t.Cleanup(func() { syscall.Kill(server, syscall.SIGKILL) })
	if m := regexp.MustCompile(`^%0 (\d+) (.+)$`).FindStringSubmatch(out); m == nil || m[1] != strconv.Itoa(server) || m[2] != socket {
		t.Errorf("tmux said %q; the server checked is %d", out, server)
	}
	if len(leftCanaries()) != 0 {
		t.Errorf("its canary stayed: %q", leftCanaries())
	}
	// The server the core checked is in this desk's sandbox: asked again, macOS still says it may not read a canary.
	again, err := sandbox.NewCanary(supportDir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Remove()
	if pid, err := sandbox.ServerIsOurs(socket, again); err != nil || pid != server {
		t.Errorf("checked again: %d %v", pid, err)
	}

	// A start whose server isn't ours (here, the check says so) is refused, and nothing that answers is taken away.
	other := standInDesk(t)
	saved := deskStartRunner
	deskStartRunner.serverIsOurs = func(string, sandbox.Canary) (int, error) { return 0, &sandbox.Refused{Why: "a stand-in's"} }
	t.Cleanup(func() { deskStartRunner = saved })
	otherSocket := filepath.Join(other.places.OwnSockets, deskSocketFile)
	if _, _, err := other.starter(self, self)([]string{"-S", otherSocket, "new-session", "-d"}); err == nil || err.Error() != sandbox.NotOurServerRefusal {
		t.Errorf("a server that isn't ours: %v", err)
	}
	// Its server, found by a canary of its own support folder, so it can be ended.
	if c, err := sandbox.NewCanary(supportDir); err == nil {
		if pid, err := sandbox.ServerIsOurs(otherSocket, c); err == nil {
			t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
		} else {
			t.Errorf("the other start's server: %v", err)
		}
		c.Remove()
	}
	if _, err := os.Lstat(otherSocket); err != nil {
		t.Errorf("a socket that answers was taken away: %v", err)
	}
}
