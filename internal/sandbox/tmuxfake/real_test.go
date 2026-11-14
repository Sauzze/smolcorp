//go:build unix

package tmuxfake

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// T1-T4 and T6 again, against a real tmux server: the hire's server is started by the guarded build with the guard
// off (as `sandbox enter` will start it), and set up the way a hire could set it up from inside (hooks, options).
// Then the client outside, stock and guarded, runs the core's ordinary command. These need the guarded build: on a
// Mac, scripts/build-tmux.sh's (found by itself in build/tmux) or SMOLCORP_TEST_TMUX; elsewhere they skip until a
// Linux build exists (SA-L5). On the Mac runner, SMOLCORP_SANDBOX_REQUIRED=1 turns that skip into a failure.
// T5 is the fake server's own row: a real server never sends MSG_SHELL to a plain has-session.

// realServer starts a tmux server of the guarded build on its own socket in dir, with one session "main" running
// cat, and ends it by its pid when the test does. It returns the socket and the server's pid.
func realServer(t *testing.T, program, dir, name string) (string, string) {
	t.Helper()
	socket := filepath.Join(dir, name)
	r := runProgram(t, []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C", "TERM=xterm", "SMOLCORP_TMUX_SERVER=off"}, "",
		program, "-f", "/dev/null", "-S", socket, "new-session", "-d", "-P", "-F", "#{pid}", "-s", "main", "cat")
	pid, err := strconv.Atoi(strings.TrimSpace(r.stdout))
	if r.status != 0 || err != nil {
		t.Fatalf("the server didn't start: %+v", r)
	}
	t.Cleanup(func() {
		syscall.Kill(pid, syscall.SIGTERM)
		for i := 0; i < 50 && syscall.Kill(pid, 0) == nil; i++ {
			time.Sleep(20 * time.Millisecond)
		}
		syscall.Kill(pid, syscall.SIGKILL)
	})
	return socket, strconv.Itoa(pid)
}

// inside runs a command on the hire's server as the hire would: the guarded build with the guard off.
func inside(t *testing.T, program, socket string, args ...string) result {
	t.Helper()
	return tmuxClient{name: "inside", path: program, off: true}.run(t, socket, "", "", nil, args...)
}

func realClients(t *testing.T) (string, []tmuxClient) {
	t.Helper()
	all := clients()
	all[1].need(t)
	return all[1].path, all
}

func TestRealServerT1AHookReadingAFileIsRefused(t *testing.T) {
	program, all := realClients(t)
	for _, c := range all {
		t.Run(c.name, func(t *testing.T) {
			dir := shortDir(t)
			socket, pid := realServer(t, program, dir, "s")
			key := filepath.Join(dir, "setup-key")
			writeFile(t, key, "FAKEKEY", 0o600)
			inside(t, program, socket, "set-hook", "-g", "after-capture-pane", "load-buffer -b k "+key)
			r := c.run(t, socket, pid, "", nil, "capture-pane", "-p", "-t", "main")
			buffer := inside(t, program, socket, "show-buffer", "-b", "k").stdout
			if c.guarded() != (r.refused() && buffer == "") || !c.guarded() && buffer != "FAKEKEY" {
				t.Fatalf("buffer %q, %+v", buffer, r)
			}
		})
	}
}

func TestRealServerT2AHookWritingAFileIsRefused(t *testing.T) {
	program, all := realClients(t)
	for _, c := range all {
		t.Run(c.name, func(t *testing.T) {
			dir := shortDir(t)
			socket, pid := realServer(t, program, dir, "s")
			fence := filepath.Join(dir, "fence.json")
			writeFile(t, fence, `{"fence":1}`, 0o600)
			inside(t, program, socket, "set-buffer", "-b", "f", `{"fence":0}`)
			inside(t, program, socket, "set-hook", "-g", "after-send-keys", "save-buffer -b f "+fence)
			r := c.run(t, socket, pid, "", nil, "send-keys", "-t", "main", "hello", "Enter")
			got := readFile(fence)
			if c.guarded() != (r.refused() && got == `{"fence":1}`) || !c.guarded() && got != `{"fence":0}` {
				t.Fatalf("fence %q, %+v", got, r)
			}
		})
	}
}

func TestRealServerT3TheClientsEnvironmentDoesntReachTheServer(t *testing.T) {
	program, all := realClients(t)
	for _, c := range all {
		t.Run(c.name, func(t *testing.T) {
			dir := shortDir(t)
			socket, pid := realServer(t, program, dir, "s")
			inside(t, program, socket, "set-option", "-g", "update-environment", "SBX_SECRET")
			inside(t, program, socket, "set-hook", "-g", "after-list-sessions", "new-session -d -s leak")
			r := c.run(t, socket, pid, "", []string{"SBX_SECRET=OWNERENV"}, "list-sessions")
			env := inside(t, program, socket, "show-environment", "-t", "leak", "SBX_SECRET")
			leaked := strings.Contains(env.stdout, "OWNERENV")
			if r.status != 0 || c.guarded() == leaked {
				t.Fatalf("leaked %v (%+v), list-sessions %+v", leaked, env, r)
			}
		})
	}
}

// T4: the hire replaces its socket with a link to the owner's server, then the core's mail types into "its" desk.
func TestRealServerT4ALinkToAnotherServerIsNotTypedInto(t *testing.T) {
	program, all := realClients(t)
	for _, c := range all {
		t.Run(c.name, func(t *testing.T) {
			dir := shortDir(t)
			owner, _ := realServer(t, program, dir, "owner")
			hire, hirePid := realServer(t, program, dir, "hire")
			if err := os.Remove(hire); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(owner, hire); err != nil {
				t.Fatal(err)
			}
			r := c.run(t, hire, hirePid, "", nil, "send-keys", "-t", "main", "typed-by-the-mail", "Enter")
			time.Sleep(300 * time.Millisecond)
			pane := inside(t, program, owner, "capture-pane", "-p", "-t", "main").stdout
			typed := strings.Contains(pane, "typed-by-the-mail")
			if c.guarded() && (typed || r.status != 98 || !strings.Contains(r.stderr, ", not "+hirePid)) || !c.guarded() && !typed {
				t.Fatalf("typed into the owner's pane %v, %+v", typed, r)
			}
		})
	}
}

// T6: a client attached in a pseudo-terminal (script(1), no window), then detach-client -E and lock-client from
// inside.
func TestRealServerT6AnAttachedClientRunsNothing(t *testing.T) {
	program, all := realClients(t)
	for _, c := range all {
		t.Run(c.name, func(t *testing.T) {
			for _, how := range []string{"detach-client -E", "lock-client"} {
				dir := shortDir(t)
				socket, pid := realServer(t, program, dir, "s")
				marker := filepath.Join(dir, "ran")
				attached := attachInPty(t, c, socket, pid)
				if how == "lock-client" {
					inside(t, program, socket, "set-option", "-g", "lock-command", "/usr/bin/touch "+marker)
					inside(t, program, socket, "lock-session", "-t", "main")
				} else {
					inside(t, program, socket, "detach-client", "-s", "main", "-E", "/usr/bin/touch "+marker)
				}
				out := attached()
				ran := exists(marker)
				refused := strings.Contains(out, "smolcorp guard: refused")
				if c.guarded() && (ran || !refused) || !c.guarded() && !ran {
					t.Errorf("%s: ran %v, refused %v, the terminal showed %q", how, ran, refused, out)
				}
			}
		})
	}
}

// attachInPty attaches the client to "main" inside script(1)'s pseudo-terminal and waits until the server lists it.
// The function it returns waits (up to five seconds) for the client to end, or ends it, and gives what the
// terminal showed.
func attachInPty(t *testing.T, c tmuxClient, socket, pid string) func() string {
	t.Helper()
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + filepath.Dir(socket), "LANG=C", "TERM=xterm"}
	switch {
	case c.off:
		env = append(env, "SMOLCORP_TMUX_SERVER=off")
	case c.guarded():
		env = append(env, "SMOLCORP_TMUX_SERVER="+pid)
	}
	client := []string{c.path, "-f", "/dev/null", "-S", socket, "attach-session", "-t", "main"}
	args := append([]string{"-q", "/dev/null"}, client...)
	if runtime.GOOS == "linux" {
		args = []string{"-qfec", strings.Join(client, " "), "/dev/null"}
	}
	cmd := exec.Command("/usr/bin/script", args...)
	cmd.Env = env
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	stdin, err := cmd.StdinPipe() // held open: script ends at its input's end
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	t.Cleanup(func() { stdin.Close(); cmd.Process.Kill(); <-done })
	program := clients()[1].path
	for i := 0; ; i++ {
		if inside(t, program, socket, "list-clients", "-F", "#{client_name}").stdout != "" {
			break
		}
		if i == 100 {
			t.Fatalf("the client never attached: %q", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	return func() string {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-done
		}
		return out.String()
	}
}

// Streams on a real server: load-buffer from stdin, save-buffer and capture-pane to stdout.
func TestRealServerStreamsStillWork(t *testing.T) {
	program, _ := realClients(t)
	guarded := clients()[1]
	dir := shortDir(t)
	socket, pid := realServer(t, program, dir, "s")
	if r := guarded.run(t, socket, pid, "streamed in", nil, "load-buffer", "-b", "s", "-"); r.status != 0 {
		t.Fatalf("load-buffer -: %+v", r)
	}
	if r := guarded.run(t, socket, pid, "", nil, "save-buffer", "-b", "s", "-"); r.status != 0 || r.stdout != "streamed in" {
		t.Fatalf("save-buffer -: %+v", r)
	}
	inside(t, program, socket, "send-keys", "-t", "main", "seen in the pane", "Enter")
	time.Sleep(300 * time.Millisecond)
	if r := guarded.run(t, socket, pid, "", nil, "capture-pane", "-p", "-t", "main"); r.status != 0 || !strings.Contains(r.stdout, "seen in the pane") {
		t.Fatalf("capture-pane -p: %+v", r)
	}
	// Unchecked mode talks to it too, and still refuses a hook's file read.
	if r := guarded.run(t, socket, "unchecked", "", nil, "has-session", "-t", "main"); r.status != 0 {
		t.Fatalf("unchecked: %+v", r)
	}
	key := filepath.Join(dir, "setup-key")
	writeFile(t, key, "FAKEKEY", 0o600)
	inside(t, program, socket, "set-hook", "-g", "after-capture-pane", "load-buffer -b k "+key)
	if r := guarded.run(t, socket, "unchecked", "", nil, "capture-pane", "-p", "-t", "main"); !r.refused() {
		t.Fatalf("unchecked: a hook's file read should be refused: %+v", r)
	}
}
