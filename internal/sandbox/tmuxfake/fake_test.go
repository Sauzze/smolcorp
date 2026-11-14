//go:build unix

package tmuxfake

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// SANDBOX-PLAN §A2's T1-T6, against the fake server: each row is what a hire's server (here, the fake) asks a client
// outside the sandbox to do. Each runs the stock client, which does it (so the row proves something), then the
// guarded one, which must refuse it. Stand-ins only: FAKEKEY, a fence file, a marker a script would make.

// T1: a file read by name (a hook's load-buffer of the setup key, R1).
func TestT1AFileReadByNameIsRefused(t *testing.T) {
	for _, c := range clients() {
		t.Run(c.name, func(t *testing.T) {
			c.need(t)
			dir := shortDir(t)
			key := filepath.Join(dir, "setup-key")
			writeFile(t, key, "FAKEKEY", 0o600)
			srv := listen(t, dir, Step{After: MsgCommand, Send: []Message{ReadOpen(1, -1, key)}},
				Step{After: MsgReadDone, Send: []Message{Exit(0)}})
			r := c.run(t, srv.Path, ownPid, "", nil, "has-session")
			got := session(t, srv).Read(1)
			if !c.guarded() {
				if got != "FAKEKEY" {
					t.Fatalf("the stock client should have read the key for the server (else the row proves nothing): %q, %+v", got, r)
				}
				return
			}
			if !r.refused() || got != "" || !strings.Contains(r.stderr, "a file opened by name") {
				t.Fatalf("the guarded client read %q for the server, or didn't refuse: %+v", got, r)
			}
		})
	}
}

// T2: a file written by name (a hook's save-buffer onto the fence, R2).
func TestT2AFileWrittenByNameIsRefused(t *testing.T) {
	for _, c := range clients() {
		t.Run(c.name, func(t *testing.T) {
			c.need(t)
			dir := shortDir(t)
			fence := filepath.Join(dir, "fence.json")
			writeFile(t, fence, `{"fence":1}`, 0o600)
			srv := listen(t, dir, Step{After: MsgCommand, Send: []Message{WriteOpen(1, -1, int32(syscall.O_TRUNC), fence)}},
				Step{After: MsgWriteReady, Send: []Message{Write(1, `{"fence":0}`), WriteClose(1), Exit(0)}})
			r := c.run(t, srv.Path, ownPid, "", nil, "has-session")
			got := readFile(fence)
			if !c.guarded() {
				if got != `{"fence":0}` {
					t.Fatalf("the stock client should have written the fence (else the row proves nothing): %q, %+v", got, r)
				}
				return
			}
			if !r.refused() || got != `{"fence":1}` {
				t.Fatalf("the guarded client wrote the fence (%q), or didn't refuse: %+v", got, r)
			}
		})
	}
}

// T3: the client's environment, which the server keeps (update-environment, R3). A guarded client sends none.
func TestT3NoEnvironmentIsSent(t *testing.T) {
	for _, c := range clients() {
		t.Run(c.name, func(t *testing.T) {
			c.need(t)
			dir := shortDir(t)
			srv := listen(t, dir, Step{After: MsgCommand, Send: []Message{Exit(0)}})
			r := c.run(t, srv.Path, ownPid, "", []string{"SBX_SECRET=OWNERENV"}, "has-session")
			got := session(t, srv)
			leaked := strings.Contains(strings.Join(got.Environ(), "\n"), "SBX_SECRET=OWNERENV")
			if r.status != 0 || !got.Has(MsgIdentifyDone) || !got.Has(MsgCommand) {
				t.Fatalf("the client should have identified itself and sent its command: %+v", r)
			}
			if !c.guarded() {
				if !leaked {
					t.Fatalf("the stock client should have sent its environment (else the row proves nothing): %q", got.Environ())
				}
				return
			}
			if env := got.Environ(); len(env) != 0 {
				t.Fatalf("the guarded client sent %d environment entries: %q", len(env), env)
			}
		})
	}
}

// T4: a server that isn't the one checked (the hire's socket replaced by a link to another server). The guarded client
// is given another pid than the fake's: it stops before sending anything. With no pid, or one that isn't a number, it
// stops before it even connects.
func TestT4AServerWithAnotherPidIsNeverSentAnything(t *testing.T) {
	for _, c := range clients() {
		t.Run(c.name, func(t *testing.T) {
			c.need(t)
			dir := shortDir(t)
			srv := listen(t, dir, Step{After: MsgCommand, Send: []Message{Exit(0)}})
			r := c.run(t, srv.Path, "1", "", nil, "send-keys", "-t", "hire", "typed by the mail", "Enter")
			if !c.guarded() {
				if r.status != 0 || !session(t, srv).Has(MsgCommand) {
					t.Fatalf("the stock client should have sent its command to whatever answered: %+v", r)
				}
				return
			}
			if r.status != 98 || !strings.Contains(r.stderr, "smolcorp guard: server pid "+ownPid+", not 1") {
				t.Fatalf("the guarded client should have stopped at the pid: %+v", r)
			}
			time.Sleep(100 * time.Millisecond)
			for _, s := range srv.Sessions() {
				if len(s.Received) != 0 {
					t.Fatalf("the server with another pid was sent %d messages", len(s.Received))
				}
			}
			for _, guard := range []string{"", "abc", "0", "-5", "12x", "unchecked "} {
				extra := []string{}
				if guard != "" {
					extra = append(extra, "SMOLCORP_TMUX_SERVER="+guard)
				}
				r := c.run(t, srv.Path, "", "", extra, "has-session")
				if r.status != 98 || !strings.Contains(r.stderr, "smolcorp guard: no server pid given") {
					t.Errorf("SMOLCORP_TMUX_SERVER=%q should stop the client: %+v", guard, r)
				}
			}
			time.Sleep(100 * time.Millisecond)
			if n := len(srv.Sessions()); n != 1 {
				t.Errorf("a client with no pid connected: %d connections, not just the first", n)
			}
		})
	}
}

// T5: a plain has-session answered with MSG_SHELL naming a script the hire wrote. A stock client runs it, outside.
func TestT5AProgramTheServerNamesIsNotRun(t *testing.T) {
	for _, c := range clients() {
		t.Run(c.name, func(t *testing.T) {
			c.need(t)
			dir := shortDir(t)
			marker, script := filepath.Join(dir, "ran"), filepath.Join(dir, "hire-script")
			writeFile(t, script, "#!/bin/sh\n/usr/bin/touch "+marker+"\n", 0o755)
			srv := listen(t, dir, Step{After: MsgCommand, Send: []Message{Shell(script)}})
			r := c.run(t, srv.Path, ownPid, "", nil, "has-session")
			if !c.guarded() {
				if !exists(marker) {
					t.Fatalf("the stock client should have run the hire's script (else the row proves nothing): %+v", r)
				}
				return
			}
			if !r.refused() || exists(marker) || !strings.Contains(r.stderr, "a program to run") {
				t.Fatalf("the guarded client ran the script, or didn't refuse: %+v", r)
			}
		})
	}
}

// T6: an attached client told to run a command as it detaches (detach-client -E), to lock (lock-client), or to
// detach and SIGHUP its parent (MSG_DETACHKILL), which a guarded client takes as a plain detach.
func TestT6AnAttachedClientRunsNothingAndKillsNoParent(t *testing.T) {
	attach := Step{After: MsgCommand, Send: []Message{Simple(MsgReady)}}
	for _, c := range clients() {
		t.Run(c.name, func(t *testing.T) {
			c.need(t)
			dir := shortDir(t)
			marker := filepath.Join(dir, "exec-ran")
			srv := listen(t, dir, attach, Step{After: MsgResize, Send: []Message{Exec("/usr/bin/touch "+marker, "/bin/sh")}},
				Step{After: MsgExiting, Send: []Message{Simple(MsgExited)}})
			r := c.run(t, srv.Path, ownPid, "", nil, "attach-session", "-t", "hire")
			if ran := exists(marker); c.guarded() && (ran || !r.refused()) || !c.guarded() && !ran {
				t.Errorf("detach-client -E: ran %v, %+v", ran, r)
			}

			dir = shortDir(t)
			marker = filepath.Join(dir, "lock-ran")
			srv = listen(t, dir, attach, Step{After: MsgResize, Send: []Message{Lock("/usr/bin/touch " + marker)}},
				Step{After: MsgUnlock, Send: []Message{Exit(0)}}, Step{After: MsgExiting, Send: []Message{Simple(MsgExited)}})
			r = c.run(t, srv.Path, ownPid, "", nil, "attach-session", "-t", "hire")
			if ran := exists(marker); c.guarded() && (ran || !r.refused()) || !c.guarded() && !ran {
				t.Errorf("lock-client: ran %v, %+v", ran, r)
			}

			// The parent is a shell that says so if it outlives the client.
			dir = shortDir(t)
			srv = listen(t, dir, attach, Step{After: MsgResize, Send: []Message{Detach("hire", true)}},
				Step{After: MsgExiting, Send: []Message{Simple(MsgExited)}})
			env := []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "TERM=xterm", "SMOLCORP_TMUX_SERVER=" + ownPid}
			if c.off {
				env[3] = "SMOLCORP_TMUX_SERVER=off"
			}
			r = runProgram(t, env, "", "/bin/sh", "-c", `"$0" "$@"; echo "status $?, the parent is alive"`,
				c.path, "-f", "/dev/null", "-S", srv.Path, "attach-session", "-t", "hire")
			alive := strings.Contains(r.stdout, "status 0, the parent is alive")
			if c.guarded() != alive {
				t.Errorf("MSG_DETACHKILL: parent alive %v, %+v", alive, r)
			}
		})
	}
}

// The allowlist, message by message, for each kind of client: what it takes goes on (the client may still end
// for its own reasons, never 97), anything else is refused before it acts. MSG_SUSPEND to an attached client is
// left out: the client stops itself (SIGTSTP), as it should, and the row would wait for ever.
func TestTheGuardTakesOnlyItsKindsMessages(t *testing.T) {
	guarded := clients()[1]
	guarded.need(t)
	type row struct {
		name  string
		msg   Message
		taken bool
	}
	unknown := Message{Type: 999}
	oneShot := []row{
		{"exit", Exit(0), true}, {"exited", Simple(MsgExited), true}, {"shutdown", Simple(MsgShutdown), true},
		{"version", Simple(MsgVersion), true}, {"flags", Flags(0), true},
		{"stdin", ReadOpen(1, 0, ""), true}, {"stdout", WriteOpen(1, 1, 0, ""), true}, {"stderr", WriteOpen(1, 2, 0, ""), true},
		{"read cancel", Message{MsgReadCancel, ints(5)}, true}, {"write", Write(5, "x"), true}, {"write close", WriteClose(5), true},
		{"read by name", ReadOpen(1, -1, "/dev/null"), false}, {"read fd 3", ReadOpen(1, 3, ""), false},
		{"read by name, no path", ReadOpen(1, -1, ""), false}, {"write by name", WriteOpen(1, -1, 0, "/dev/null"), false},
		{"write fd 5", WriteOpen(1, 5, 0, ""), false}, {"shell", Shell("/bin/sh"), false},
		{"ready", Simple(MsgReady), false}, {"detach", Detach("s", false), false}, {"detachkill", Detach("s", true), false},
		{"suspend", Simple(MsgSuspend), false}, {"lock", Lock("/usr/bin/true"), false}, {"exec", Exec("true", "/bin/sh"), false},
		{"unlock", Simple(MsgUnlock), false}, {"wakeup", Simple(MsgWakeup), false}, {"resize", Simple(MsgResize), false},
		{"command", Message{MsgCommand, ints(0)}, false}, {"old stderr", Simple(MsgOldStderr), false}, {"unknown", unknown, false},
	}
	attached := []row{
		{"exit", Exit(0), true}, {"exited", Simple(MsgExited), true}, {"shutdown", Simple(MsgShutdown), true},
		{"flags", Flags(0), true}, {"detach", Detach("s", false), true}, {"detachkill", Detach("s", true), true},
		{"exec", Exec("true", "/bin/sh"), false}, {"lock", Lock("/usr/bin/true"), false}, {"shell", Shell("/bin/sh"), false},
		{"ready again", Simple(MsgReady), false}, {"version", Simple(MsgVersion), false},
		{"stdout", WriteOpen(1, 1, 0, ""), false}, {"read by name", ReadOpen(1, -1, "/dev/null"), false},
		{"write", Write(5, "x"), false}, {"unlock", Simple(MsgUnlock), false}, {"unknown", unknown, false},
	}
	check := func(t *testing.T, r row, command string, steps ...Step) {
		dir := shortDir(t)
		srv := listen(t, dir, steps...)
		res := guarded.run(t, srv.Path, ownPid, "", nil, command)
		if r.taken && (res.status == 97 || strings.Contains(res.stderr, "smolcorp guard")) {
			t.Errorf("%s (%s): should be taken, was refused: %+v", r.name, command, res)
		}
		if !r.taken && !res.refused() {
			t.Errorf("%s (%s): should be refused: %+v", r.name, command, res)
		}
	}
	for _, r := range oneShot {
		check(t, r, "has-session", Step{After: MsgCommand, Send: []Message{r.msg, Exit(0)}})
	}
	// MSG_READY only for a command that attaches: has-session refuses it (above), attach-session takes it.
	check(t, row{"ready", Simple(MsgReady), true}, "attach-session", Step{After: MsgCommand, Send: []Message{Simple(MsgReady)}},
		Step{After: MsgResize, Send: []Message{Exit(0)}}, Step{After: MsgExiting, Send: []Message{Simple(MsgExited)}})
	check(t, row{"ready", Simple(MsgReady), true}, "new-session", Step{After: MsgCommand, Send: []Message{Simple(MsgReady)}},
		Step{After: MsgResize, Send: []Message{Exit(0)}}, Step{After: MsgExiting, Send: []Message{Simple(MsgExited)}})
	for _, r := range attached {
		check(t, r, "attach-session", Step{After: MsgCommand, Send: []Message{Simple(MsgReady)}},
			Step{After: MsgResize, Send: []Message{r.msg, Exit(0)}}, Step{After: MsgExiting, Send: []Message{Simple(MsgExited)}})
	}
}

// Streams still work through a guarded client: what the server writes to its stdout (capture-pane -p), and its
// stdin read for the server (load-buffer -).
func TestStreamsStillWorkThroughAGuardedClient(t *testing.T) {
	for _, c := range clients() {
		t.Run(c.name, func(t *testing.T) {
			c.need(t)
			dir := shortDir(t)
			srv := listen(t, dir, Step{After: MsgCommand, Send: []Message{WriteOpen(1, 1, 0, ""), Write(1, "pane text\n"), WriteClose(1), Exit(0)}})
			if r := c.run(t, srv.Path, ownPid, "", nil, "capture-pane", "-p"); r.status != 0 || r.stdout != "pane text\n" {
				t.Errorf("stdout: %+v", r)
			}
			dir = shortDir(t)
			srv = listen(t, dir, Step{After: MsgCommand, Send: []Message{ReadOpen(2, 0, "")}}, Step{After: MsgReadDone, Send: []Message{Exit(0)}})
			r := c.run(t, srv.Path, ownPid, "buffer in", nil, "load-buffer", "-")
			if got := session(t, srv).Read(2); r.status != 0 || got != "buffer in" {
				t.Errorf("stdin: read %q, %+v", got, r)
			}
		})
	}
}

// Unchecked mode (N4): for a desk with no checked pid, any server of this user's is talked to, and the refusals stay.
func TestUncheckedModeSkipsOnlyThePid(t *testing.T) {
	guarded := clients()[1]
	guarded.need(t)
	dir := shortDir(t)
	srv := listen(t, dir, Step{After: MsgCommand, Send: []Message{Exit(0)}})
	if r := guarded.run(t, srv.Path, "unchecked", "", []string{"SBX_SECRET=OWNERENV"}, "has-session"); r.status != 0 {
		t.Fatalf("unchecked: %+v", r)
	}
	if got := session(t, srv); !got.Has(MsgCommand) || len(got.Environ()) != 0 {
		t.Fatalf("unchecked: the command should arrive, with no environment: %q", got.Environ())
	}
	dir = shortDir(t)
	key := filepath.Join(dir, "setup-key")
	writeFile(t, key, "FAKEKEY", 0o600)
	srv = listen(t, dir, Step{After: MsgCommand, Send: []Message{ReadOpen(1, -1, key)}}, Step{After: MsgReadDone, Send: []Message{Exit(0)}})
	if r := guarded.run(t, srv.Path, "unchecked", "", nil, "has-session"); !r.refused() || session(t, srv).Read(1) != "" {
		t.Fatalf("unchecked: a file read should still be refused: %+v", r)
	}
}

// A guarded client never starts a server, even for new-session or a bare `tmux`.
func TestAGuardedClientNeverStartsAServer(t *testing.T) {
	guarded := clients()[1]
	guarded.need(t)
	for _, args := range [][]string{{"new-session", "-d", "-s", "x"}, {"start-server"}, {}} {
		dir := shortDir(t)
		socket := filepath.Join(dir, "s")
		r := guarded.run(t, socket, "unchecked", "", nil, args...)
		if r.status == 0 || exists(socket) {
			t.Errorf("%v started a server: %+v", args, r)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%v left %d files", args, len(entries))
		}
	}
}
