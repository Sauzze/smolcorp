package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
	tf "github.com/Sauzze/smolcorp/internal/sandbox/tmuxfake"
)

// SA-M8's T4-T6 through a real attach line (SANDBOX-PLAN §A2, §A3.1): `smolcorp attach <desk>`, as a window's line runs
// it, on a desk in its sandbox. It runs the real /usr/bin/sandbox-exec with the desk's real profile, `smolcorp sandbox
// enter --attach` (this test program as smolcorp, as the start's test does: owner Q5 (a)), and the guarded bundled
// tmux (SMOLCORP_TEST_TMUX, or this checkout's scripts/build-tmux.sh build), against a fake server of the repo's
// (internal/sandbox/tmuxfake) at the desk's own socket, listening in this process, so the pid the core checked is this
// test's. Nothing is read but stand-ins; no terminal, window or real tmux server of the owner's is touched.

// guardedTmuxHere is the guarded tmux to run, or the test is skipped (failed under SMOLCORP_SANDBOX_REQUIRED=1).
func guardedTmuxHere(t *testing.T) string {
	t.Helper()
	path := os.Getenv("SMOLCORP_TEST_TMUX")
	if path == "" {
		found, _ := filepath.Glob("../build/tmux/*/tmux")
		if len(found) > 0 {
			path, _ = filepath.Abs(found[len(found)-1])
		}
	}
	why := ""
	if err := sandbox.CheckSandboxExec(); err != nil {
		why = err.Error()
	} else if path == "" {
		why = "set SMOLCORP_TEST_TMUX to a guarded tmux, or run scripts/build-tmux.sh"
	} else if _, err := os.Stat(path); err != nil {
		why = err.Error()
	}
	if why != "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal(why)
		}
		t.Skip("SKIPPED: " + why)
	}
	return path
}

// attachedDeskRig is desk ab12 in its sandbox as its start leaves it: its profile written as the start writes it, its
// own socket folder, and its row naming the guarded tmux and the server pid the core checked (server).
func attachedDeskRig(t *testing.T, server int) (socket string, profile string) {
	t.Helper()
	tmux := guardedTmuxHere(t)
	s := standInDesk(t)
	socket = filepath.Join(s.places.OwnSockets, deskSocketFile)
	profile = filepath.Join(hireFolder(supportDir, "ab12"), "sandbox.sb")
	if err := sandbox.PrepareProfile(s.policy, profile, deskServices(s.places, socket)); err != nil {
		t.Fatal(err)
	}
	if err := makeDeskSocketFolder(socket); err != nil {
		t.Fatal(err)
	}
	writeRegistryHires(t, Hire{ID: "ab12", Name: "Sandboxed", State: "working", Folder: s.at.Root, TmuxSocket: socket,
		TmuxProgram: tmux, TmuxServer: server})
	saved := attachSandboxEnvironment
	attachSandboxEnvironment = func() []string { return append(attachEnvironment(), deskStandInVariable+"=1") }
	t.Cleanup(func() { attachSandboxEnvironment = saved })
	return socket, profile
}

// attachLineRun runs the attach line the engine writes for desk ab12 (attachLine), as its window would, against a fake
// server at socket playing steps: its exit status, what it said, and what the server was sent.
func attachLineRun(t *testing.T, socket string, steps ...tf.Step) (int, string, []tf.Session) {
	t.Helper()
	srv, err := tf.Listen(socket, steps...)
	if err != nil {
		t.Fatal(err)
	}
	line := attachLine(commandLineProgram(), "ab12")
	var said bytes.Buffer
	status := runAttach(line[2:], nil, &said, &said)
	srv.Close()
	if left := leftCanaries(); len(left) != 0 {
		t.Errorf("the attach's canary stayed: %q", left)
	}
	return status, said.String(), srv.Sessions()
}

func TestOnAMacAnAttachLineRunsTheGuardedTmuxInTheDesksSandbox(t *testing.T) {
	self := strconv.Itoa(os.Getpid())
	socket, profile := attachedDeskRig(t, os.Getpid())
	dir, err := os.MkdirTemp("", "ao")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	dir = physical(dir)
	attached := tf.Step{After: tf.MsgCommand, Send: []tf.Message{tf.Simple(tf.MsgReady)}}
	exited := tf.Step{After: tf.MsgExiting, Send: []tf.Message{tf.Simple(tf.MsgExited)}}
	// A client that attaches where it must not is detached at once, so the row fails rather than waits.
	detached := tf.Step{After: tf.MsgResize, Send: []tf.Message{tf.Detach("hire", false)}}
	refused := func(status int, said string) bool {
		return status == 97 && strings.Contains(said, "smolcorp guard: refused")
	}

	// The control: it attaches, the server sees the attach, and no environment of the terminal's; a detach ends it.
	status, said, sessions := attachLineRun(t, socket, attached, detached, exited)
	if status != 0 || len(sessions) != 1 || !sessions[0].Has(tf.MsgCommand) || len(sessions[0].Environ()) != 0 {
		t.Fatalf("the attach itself: %d %q %+v", status, said, sessions)
	}

	// T5: the server asks the attach to run a program the desk wrote, read a file by name, or write one.
	marker, script := filepath.Join(dir, "ran"), filepath.Join(dir, "desk-script")
	key, fence := filepath.Join(dir, "setup-key"), filepath.Join(dir, "fence.json")
	for path, text := range map[string]string{script: "#!/bin/sh\n/usr/bin/touch " + marker + "\n", key: "FAKEKEY", fence: `{"fence":1}`} {
		if err := os.WriteFile(path, []byte(text), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if status, said, _ := attachLineRun(t, socket, tf.Step{After: tf.MsgCommand, Send: []tf.Message{tf.Shell(script)}}); !refused(status, said) || fileExists(marker) {
		t.Errorf("T5, a program the server names: %d %q, ran %v", status, said, fileExists(marker))
	}
	status, said, sessions = attachLineRun(t, socket, tf.Step{After: tf.MsgCommand, Send: []tf.Message{tf.ReadOpen(1, -1, key)}},
		tf.Step{After: tf.MsgReadDone, Send: []tf.Message{tf.Exit(0)}})
	if !refused(status, said) || len(sessions) != 1 || sessions[0].Read(1) != "" {
		t.Errorf("T5, a file read by name: %d %q %+v", status, said, sessions)
	}
	status, said, _ = attachLineRun(t, socket, tf.Step{After: tf.MsgCommand, Send: []tf.Message{tf.WriteOpen(1, -1, int32(syscall.O_TRUNC), fence)}},
		tf.Step{After: tf.MsgWriteReady, Send: []tf.Message{tf.Write(1, `{"fence":0}`), tf.WriteClose(1), tf.Exit(0)}})
	if data, _ := os.ReadFile(fence); !refused(status, said) || string(data) != `{"fence":1}` {
		t.Errorf("T5, a file written by name: %d %q, the fence %q", status, said, data)
	}

	// T6: attached, then detach-client -E and lock-client from the desk's server.
	for name, ask := range map[string]tf.Message{"detach-client -E": tf.Exec("/usr/bin/touch "+marker, "/bin/sh"), "lock-client": tf.Lock("/usr/bin/touch " + marker)} {
		status, said, _ := attachLineRun(t, socket, attached, tf.Step{After: tf.MsgResize, Send: []tf.Message{ask}},
			tf.Step{After: tf.MsgUnlock, Send: []tf.Message{tf.Exit(0)}}, exited)
		if !refused(status, said) || fileExists(marker) {
			t.Errorf("T6, %s: %d %q, ran %v", name, status, said, fileExists(marker))
		}
	}

	// T4: a server at the desk's place that isn't the one the core checked is sent nothing; a link put at its socket,
	// to another server, isn't the desk's socket at all.
	writeRegistryHires(t, Hire{ID: "ab12", Name: "Sandboxed", State: "working", TmuxSocket: socket, TmuxProgram: guardedTmuxHere(t), TmuxServer: os.Getppid()})
	status, said, sessions = attachLineRun(t, socket, attached, detached, exited)
	if status != 98 || !strings.Contains(said, "server pid "+self+", not "+strconv.Itoa(os.Getppid())) || len(sessions) > 1 ||
		len(sessions) == 1 && len(sessions[0].Received) != 0 {
		t.Errorf("T4, another server: %d %q %+v", status, said, sessions)
	}
	writeRegistryHires(t, Hire{ID: "ab12", Name: "Sandboxed", State: "working", TmuxSocket: socket, TmuxProgram: guardedTmuxHere(t), TmuxServer: os.Getpid()})
	owners, err := tf.Listen(filepath.Join(dir, "o"), attached, detached, exited)
	if err != nil {
		t.Fatal(err)
	}
	defer owners.Close()
	if err := os.Symlink(owners.Path, socket); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if status := runAttach([]string{"ab12"}, nil, &out, &out); status != 1 || len(owners.Sessions()) != 0 || !strings.Contains(out.String(), "no tmux session") {
		t.Errorf("T4, a link to another server: %d %q %+v", status, out.String(), owners.Sessions())
	}
	os.Remove(socket)

	// An attach whose sandbox doesn't hold (a looser profile in the desk's): `enter`'s rows fail, nothing attaches.
	if err := os.WriteFile(profile, []byte("(version 1)\n(allow default)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, said, sessions = attachLineRun(t, socket, attached, detached, exited)
	if status != 1 || len(sessions) != 0 || !strings.Contains(said, sandbox.Refusal("darwin")) {
		t.Errorf("a sandbox that doesn't hold: %d %q %+v", status, said, sessions)
	}
}
