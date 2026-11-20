package sandbox

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// FXPTYOWN (RT126SWEEP2 F2), on this Mac with stand-ins only: under its real profile a desk opens only the ptys it made
// itself (sbpl.go, "Terminals"). Every pty here is one the test made: a stand-in terminal outside any sandbox (the
// owner's Terminal, iTerm2 or Ghostty window, an attach window), and one made inside a stand-in desk; a path is opened
// only once the stand-in that made it has named it. Each refusal has its control outside the sandbox, where the same
// open works, so a refusal inside is the profile's. Nothing is typed into or read from a real terminal.

// terminalProbe is the desk's side and the stand-in terminals'. Its steps print one JSON line each.
const terminalProbe = `import ctypes, errno, fcntl, json, os, select, sys, termios, threading, time
# O_EVTONLY is in os only from Python 3.13; the Command Line Tools' python3 is 3.9. Its value is <sys/fcntl.h>'s.
O_EVTONLY = getattr(os, "O_EVTONLY", 0x8000)
libc = ctypes.CDLL(None, use_errno=True)
libc.ptsname.restype = ctypes.c_char_p
for f in ("sandbox_extension_issue_file", "sandbox_extension_issue_generic"):
    getattr(libc, f).restype = ctypes.c_char_p
def name(e):
    return errno.errorcode.get(e.errno, str(e.errno))
def make():
    m = libc.posix_openpt(os.O_RDWR | os.O_NOCTTY)
    if m < 0 or libc.grantpt(m) != 0 or libc.unlockpt(m) != 0:
        raise OSError(ctypes.get_errno(), "posix_openpt")
    return m, libc.ptsname(m).decode()
def read_until(fd, want):
    got, end = b"", time.time() + 3
    while want not in got and time.time() < end:
        if not select.select([fd], [], [], max(0, end - time.time()))[0]:
            break
        got += os.read(fd, 4096)
    return want in got
def tries(at):
    said = {}
    for mode, flag in (("read", os.O_RDONLY), ("write", os.O_WRONLY), ("both", os.O_RDWR), ("evtonly", O_EVTONLY)):
        try:
            os.close(os.open(at, flag | os.O_NOCTTY | os.O_NONBLOCK))
            said[mode] = "ok"
        except OSError as e:
            said[mode] = name(e)
    return said
def own():
    m, at = make()
    said = {}
    try:
        s = os.open(at, os.O_RDWR | os.O_NOCTTY)
        os.write(m, b"PING\n")
        ping = read_until(s, b"PING")
        os.write(s, b"PONG\n")
        said["own"] = "ok" if ping and read_until(m, b"PONG") else "no echo"
    except OSError as e:
        said["own"] = name(e)
    pid = os.fork()
    if pid == 0:
        try:
            os.close(os.open(at, os.O_RDWR | os.O_NOCTTY))
            os._exit(0)
        except OSError as e:
            os._exit(e.errno)
    status = os.waitpid(pid, 0)[1]  # os.waitstatus_to_exitcode is 3.9's
    code = os.WEXITSTATUS(status) if os.WIFEXITED(status) else -os.WTERMSIG(status)
    said["its child"] = "ok" if code == 0 else errno.errorcode.get(code, str(code))
    return said
def key(at):
    said = {}
    for kind, make_key in (("file", lambda c: libc.sandbox_extension_issue_file(c, at.encode(), 0)),
                           ("generic", lambda c: libc.sandbox_extension_issue_generic(c, 0))):
        token = make_key(b"com.apple.sandbox.pty")
        if token:
            libc.sandbox_extension_consume(token)
        said[kind] = ("made" if token else "not made") + ", then " + tries(at)["both"]
    return said
def ctty(at):
    r, w = os.pipe()
    pid = os.fork()
    if pid == 0:
        os.close(r)
        os.setsid()
        said = {}
        for what, path, flag in (("evtonly", at, O_EVTONLY), ("read", at, os.O_RDONLY),
                                 ("/dev/tty", "/dev/tty", os.O_RDWR)):
            try:
                os.open(path, flag | os.O_NONBLOCK)
                said[what] = "ok"
            except OSError as e:
                said[what] = name(e)
        os.write(w, json.dumps(said).encode())
        os._exit(0)
    os.close(w)
    got = b""
    while True:
        b = os.read(r, 4096)
        if not b:
            break
        got += b
    os.waitpid(pid, 0)
    return json.loads(got)
def revoke(at):
    if libc.revoke(at.encode()) == 0:
        return {"revoke": "ok"}
    e = ctypes.get_errno()
    return {"revoke": errno.errorcode.get(e, str(e))}
def hold():
    m, at = make()
    s = os.open(at, os.O_RDWR | os.O_NOCTTY)
    seen, lock = bytearray(), threading.Lock()
    def drain():
        while True:
            try:
                b = os.read(m, 4096)
            except OSError:
                return
            if not b:
                return
            with lock:
                seen.extend(b)
    threading.Thread(target=drain, daemon=True).start()
    print(json.dumps({"pty": at}), flush=True)
    for line in sys.stdin:
        typed = ""
        if line.startswith("type "):
            try:
                os.write(m, line[5:].rstrip("\n").replace("\\r", "\r").encode())
            except OSError as e:
                typed = name(e)
        with lock:
            print(json.dumps({"seen": seen.decode(errors="replace"), "typed": typed}), flush=True)
def handed(out):
    said = {}
    def step(what, do):
        try:
            do()
            said[what] = "ok"
        except (OSError, termios.error) as e:
            said[what] = name(e) if isinstance(e, OSError) else "termios %d" % e.args[0]
    step("tcsetattr", lambda: termios.tcsetattr(0, termios.TCSANOW, termios.tcgetattr(0)))
    step("window size", lambda: fcntl.ioctl(0, termios.TIOCGWINSZ, b"\0" * 8))
    step("write", lambda: os.write(1, b"HANDED-WROTE\n"))
    def read():
        if not read_until(0, b"OWNER-TYPED"):
            raise OSError(errno.EAGAIN, "nothing typed came")
    step("read", read)
    step("/dev/tty", lambda: os.close(os.open("/dev/tty", os.O_RDWR)))
    with open(out, "w") as f:
        json.dump(said, f)
do = {"own": own, "tries": tries, "key": key, "ctty": ctty, "revoke": revoke, "hold": hold, "handed": handed}
r = do[sys.argv[1]](*sys.argv[2:])
if r is not None:
    print(json.dumps(r), flush=True)`

// ptyPath is a pty's slave as a stand-in names it.
var ptyPath = regexp.MustCompile(`^/dev/ttys[0-9]+$`)

// terminal is a stand-in pty's maker, held until the test ends: its slave's path, and what its master saw.
type terminal struct {
	pty string
	in  io.WriteCloser
	out *bufio.Scanner
}

// standInTerminal makes a pty with terminalProbe's hold: outside any sandbox when profile is "", else in that one.
// startRefusal comes first, on python and the probe's words (startSites).
func standInTerminal(t *testing.T, profile, python, probe, dir string, env []string) *terminal {
	t.Helper()
	args := []string{probe, "hold"}
	if err := startRefusal(python, args, profile != ""); err != nil {
		t.Fatal(err)
	}
	program := python
	if profile != "" {
		program, args = sandboxExecOrSkip(t), append([]string{"-f", profile, python}, args...)
	}
	cmd := disclaim.Cmd(exec.Command(program, args...))
	cmd.Dir, cmd.Env, cmd.Stderr = dir, env, os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { in.Close(); cmd.Process.Kill(); cmd.Wait() })
	term := &terminal{in: in, out: bufio.NewScanner(out)}
	var made struct{ Pty string }
	term.line(t, &made)
	if !ptyPath.MatchString(made.Pty) {
		t.Fatalf("the stand-in terminal named %q, not a pty", made.Pty)
	}
	term.pty = made.Pty
	return term
}

func (term *terminal) line(t *testing.T, v any) {
	t.Helper()
	got := make(chan bool, 1)
	go func() { got <- term.out.Scan() }()
	select {
	case ok := <-got:
		if !ok {
			t.Fatalf("the stand-in terminal ended: %v", term.out.Err())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the stand-in terminal didn't answer within ten seconds")
	}
	if err := json.Unmarshal(term.out.Bytes(), v); err != nil {
		t.Fatalf("the stand-in terminal said %q: %v", term.out.Text(), err)
	}
}

// seen is what the terminal's master has read so far, after typing typed ("" types nothing).
func (term *terminal) seen(t *testing.T, typed string) string {
	t.Helper()
	seen, _ := term.typing(t, typed)
	return seen
}

// typing types typed on the terminal's master ("" types nothing), and says what the master has read so far and the
// typing's error ("" when it was typed).
func (term *terminal) typing(t *testing.T, typed string) (seen, failed string) {
	t.Helper()
	cmd := "seen"
	if typed != "" {
		cmd = "type " + typed
	}
	fmt.Fprintln(term.in, cmd)
	var said struct{ Seen, Typed string }
	term.line(t, &said)
	return said.Seen, said.Typed
}

// alive is whether the terminal still takes what is typed on it and echoes it back: mark, a line of its own. failed is
// the typing's error, as typing says it.
func (term *terminal) alive(t *testing.T, mark string) (ok bool, failed string) {
	t.Helper()
	seen, failed := term.typing(t, mark+`\r`)
	for i := 0; i < 30 && failed == "" && !strings.Contains(seen, mark); i++ {
		time.Sleep(100 * time.Millisecond)
		seen, failed = term.typing(t, "")
	}
	return failed == "" && strings.Contains(seen, mark), failed
}

// onTerminal starts program in profile with term's pty as its stdin, stdout, stderr and controlling terminal, as a
// terminal app starts its shell (and an attach client, or a plain hire's agent, then runs on it). startRefusal comes
// first, told the start is in a desk, as in work (escape2_darwin_test.go).
func onTerminal(t *testing.T, term *terminal, profile, dir string, env []string, program string, args ...string) *exec.Cmd {
	t.Helper()
	if err := startRefusal(program, args, true); err != nil {
		t.Fatal(err)
	}
	sandboxExec := sandboxExecOrSkip(t)
	f, err := os.OpenFile(term.pty, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cmd := disclaim.Cmd(exec.Command(sandboxExec, append([]string{"-f", profile, program}, args...)...))
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = f, f, f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd
}

// terminalDesk is a stand-in desk, the probe in its temp folder, and python3 and the environment to run it with.
func terminalDesk(t *testing.T) (d *escapeDesk, python, probe string, env []string) {
	t.Helper()
	sandboxExecOrSkip(t)
	if python = python3(t); python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal("SMOLCORP_SANDBOX_REQUIRED=1 and no python3")
		}
		t.Skip("no python3 here")
	}
	d = newEscapeDesk(t)
	probe = filepath.Join(d.base, "terminals.py")
	mustWrite(t, probe, terminalProbe)
	return d, python, probe, []string{"HOME=" + d.home, "PATH=" + filepath.Dir(python) + ":/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "TERM=xterm"}
}

// probeSaid is the JSON line a probe step printed, as one string per key.
func probeSaid(t *testing.T, r ran) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(r.stdout)), &m); err != nil {
		t.Fatalf("the probe said %+v: %v", r, err)
	}
	return m
}

// A desk opens the ptys it made itself, and its children do, and nothing else: not a stand-in terminal outside (O_EVTONLY
// either), not through a link, not by a key it makes itself, not as a new session's controlling terminal, not by an old
// BSD name, not another desk's, not one made in another start of its own profile. A terminal it is handed works whole.
// Outside any sandbox, each of those opens works.
func TestOnAMacADeskOpensOnlyTheTerminalsItMadeItself(t *testing.T) {
	d, python, probe, env := terminalDesk(t)
	other := newEscapeDesk(t)
	inDesk := func(desk *escapeDesk, args ...string) map[string]string {
		t.Helper()
		return probeSaid(t, desk.work(t, "", desk.root, 20*time.Second, env, python, append([]string{probe}, args...)...))
	}
	outsideTries := func(at string) map[string]string {
		t.Helper()
		return probeSaid(t, ran{stdout: outside(t, d.base, env, python, probe, "tries", at)})
	}
	refused := map[string]string{"read": "EPERM", "write": "EPERM", "both": "EPERM", "evtonly": "EPERM"}
	opened := map[string]string{"read": "ok", "write": "ok", "both": "ok", "evtonly": "ok"}
	same := func(a, b map[string]string) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

	if got := inDesk(d, "own"); got["own"] != "ok" || got["its child"] != "ok" {
		t.Errorf("its own pty, read and written from both ends, and opened by its child: %v", got)
	}

	owners := standInTerminal(t, "", python, probe, d.base, env)
	if got := outsideTries(owners.pty); !same(got, opened) {
		t.Fatalf("control: outside, the stand-in terminal %s opens: %v", owners.pty, got)
	}
	if got := inDesk(d, "tries", owners.pty); !same(got, refused) {
		t.Errorf("a terminal outside, %s: %v", owners.pty, got)
	}
	link := filepath.Join(d.root, "tty")
	if err := os.Symlink(owners.pty, link); err != nil {
		t.Fatal(err)
	}
	if got := inDesk(d, "tries", link); !same(got, refused) {
		t.Errorf("a terminal outside, through a link in its worktree: %v", got)
	}
	if got := inDesk(d, "key", owners.pty); got["file"] != "not made, then EPERM" || got["generic"] != "not made, then EPERM" {
		t.Errorf("a terminal outside, by a key it makes itself: %v", got)
	}

	// As its controlling terminal: a new session opens it without O_NOCTTY, then opens /dev/tty (FXPTYOWNR1, the
	// review's O1). Outside, the stand-in becomes that session's and /dev/tty opens; the session's end cuts it off, so
	// each side has a stand-in of its own.
	unowned := standInTerminal(t, "", python, probe, d.base, env)
	if got := inDesk(d, "ctty", unowned.pty); !same(got, map[string]string{"evtonly": "EPERM", "read": "EPERM", "/dev/tty": "ENXIO"}) {
		t.Errorf("a terminal outside, as its controlling terminal %s: %v", unowned.pty, got)
	}
	control := standInTerminal(t, "", python, probe, d.base, env)
	if got := probeSaid(t, ran{stdout: outside(t, d.base, env, python, probe, "ctty", control.pty)}); !same(got, map[string]string{"evtonly": "ok", "read": "ok", "/dev/tty": "ok"}) {
		t.Errorf("control: outside, the stand-in %s as a new session's controlling terminal: %v", control.pty, got)
	}

	// The old BSD names (FXPTYOWNR1, the review's O1): their masters can't be opened on macOS 26 (EAGAIN), so nothing
	// holds the slaves, and outside they open.
	for _, old := range []string{"/dev/ttyp0", "/dev/ttywf"} {
		if got := outsideTries(old); !same(got, opened) {
			t.Fatalf("control: outside, %s opens: %v", old, got)
		}
		if got := inDesk(d, "tries", old); !same(got, refused) {
			t.Errorf("an old BSD name, %s: %v", old, got)
		}
	}

	desks := standInTerminal(t, d.profile, python, probe, d.root, env)
	if got := outsideTries(desks.pty); !same(got, opened) {
		t.Fatalf("control: outside, the desk's pty %s opens: %v", desks.pty, got)
	}
	if got := inDesk(other, "tries", desks.pty); !same(got, refused) {
		t.Errorf("another desk opens the desk's pty %s: %v", desks.pty, got)
	}
	if got := inDesk(d, "tries", desks.pty); !same(got, refused) {
		t.Errorf("another start of the desk's own profile opens its pty %s: %v", desks.pty, got)
	}

	// Handed a terminal made outside: as an attach client's window, or a plain hire's pty its launcher made.
	window := standInTerminal(t, "", python, probe, d.base, env)
	window.seen(t, `OWNER-TYPED\r`)
	out := filepath.Join(d.root, "handed.json")
	cmd := onTerminal(t, window, d.profile, d.root, env, python, probe, "handed", out)
	if err := waitWithin(cmd, 15*time.Second); err != nil {
		t.Errorf("the handed step: %v", err)
	}
	data, _ := os.ReadFile(out)
	if want := `{"tcsetattr": "ok", "window size": "ok", "write": "ok", "read": "ok", "/dev/tty": "ok"}`; string(data) != want {
		t.Errorf("a terminal it is handed: %s, want %s", data, want)
	}
	if seen := window.seen(t, ""); !strings.Contains(seen, "HANDED-WROTE") {
		t.Errorf("the handed terminal shows %q", seen)
	}
}

// A desk can't revoke() a terminal it didn't make, which cuts off every holder of it, the owner's window too (FXPTYOWNR1,
// the review's N1): the stand-in outside still echoes what is typed on it. Its own work doesn't revoke: a script(1) in
// it runs (its tmux, and a script(1) in a pane, are TestOnAMacADesksTmuxWorksOnlyWithItsOwnTerminals). Outside, the
// same revoke cuts a stand-in off.
func TestOnAMacADeskCantCutOffATerminalItDidntMake(t *testing.T) {
	d, python, probe, env := terminalDesk(t)
	owners := standInTerminal(t, "", python, probe, d.base, env)
	if got := probeSaid(t, d.work(t, "", d.root, 20*time.Second, env, python, probe, "revoke", owners.pty)); got["revoke"] != "EPERM" {
		t.Errorf("the desk revokes a terminal outside, %s: %v", owners.pty, got)
	}
	if ok, failed := owners.alive(t, "STILL-HERE"); !ok {
		t.Errorf("the terminal outside after the desk's revoke: typing said %q, and no echo", failed)
	}
	if r := d.work(t, "", d.root, 20*time.Second, env, "/usr/bin/script", "-q", "/dev/null", "/bin/sh", "-c", "echo SCRIPT-$((4*4))"); r.status != 0 || !strings.Contains(r.stdout, "SCRIPT-16") {
		t.Errorf("a script(1) in the desk: %+v", r)
	}

	cut := standInTerminal(t, "", python, probe, d.base, env)
	if got := probeSaid(t, ran{stdout: outside(t, d.base, env, python, probe, "revoke", cut.pty)}); got["revoke"] != "ok" {
		t.Fatalf("control: outside, revoke %s: %v", cut.pty, got)
	}
	if ok, failed := cut.alive(t, "CUT-OFF"); ok || failed != "EIO" {
		t.Errorf("control: the stand-in after its revoke outside: alive %v, typing said %q", ok, failed)
	}
}

// waitWithin waits for cmd, killing it after limit.
func waitWithin(cmd *exec.Cmd, limit time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(limit):
		cmd.Process.Kill()
		<-done
		return fmt.Errorf("killed after %s", limit)
	}
}

// A desk's own tmux works with the ptys it makes: a session, a second window and a split pane, each running what it is
// sent; /dev/tty, stty and a read from it in a pane; a pane's own pty by its path; a nested script(1); and an attach
// client in the desk's profile, on a stand-in window made outside, shows the pane and types into it. A pane can't
// open the stand-in window, and outside the same open works.
func TestOnAMacADesksTmuxWorksOnlyWithItsOwnTerminals(t *testing.T) {
	d, python, probe, env := terminalDesk(t)
	program, _, _ := tmuxForTheServer(t)
	if err := os.Remove(d.ownSocket); err != nil { // the stand-in server makes way for the real one
		t.Fatal(err)
	}
	env = append(env, "SMOLCORP_TMUX_SERVER=off")
	tmux := func(args ...string) ran {
		t.Helper()
		return d.work(t, "", d.root, 20*time.Second, env, program, append([]string{"-f", "/dev/null", "-S", d.ownSocket}, args...)...)
	}
	if r := tmux("new-session", "-d", "-s", "main", "-x", "120", "-y", "40", "/bin/sh"); r.status != 0 {
		t.Fatalf("the desk's server didn't start in its sandbox: %+v", r)
	}
	t.Cleanup(func() { tmux("kill-server") })
	shows := func(target, want string) bool {
		t.Helper()
		for i := 0; i < 50; i++ {
			if strings.Contains(tmux("capture-pane", "-p", "-t", target).stdout, want) {
				return true
			}
			time.Sleep(100 * time.Millisecond)
		}
		return false
	}
	tmux("send-keys", "-t", "main", `echo HELLO-$((6*7))`, "Enter")
	if !shows("main", "HELLO-42") {
		t.Errorf("the first pane didn't run what it was sent")
	}
	for _, args := range [][]string{{"new-window", "-t", "main", "/bin/sh"}, {"split-window", "-t", "main", "/bin/sh"}} {
		if r := tmux(args...); r.status != 0 {
			t.Errorf("%s: %+v", args[0], r)
		}
	}
	tmux("send-keys", "-t", "main", `echo SPLIT-$((5*5))`, "Enter")
	if !shows("main", "SPLIT-25") {
		t.Errorf("the split pane didn't run what it was sent")
	}
	if r := tmux("list-panes", "-s", "-t", "main", "-F", "#{pane_tty}"); len(strings.Fields(r.stdout)) != 3 {
		t.Errorf("the panes: %+v", r)
	}

	window := standInTerminal(t, "", python, probe, d.base, env)
	pane := filepath.Join(d.root, "pane.txt")
	for _, line := range []string{
		fmt.Sprintf(`stty -a < /dev/tty > /dev/null 2>> %[1]s; echo "stty $?" >> %[1]s`, pane),
		fmt.Sprintf(`read x < /dev/tty; echo "read $x" >> %s`, pane),
		"typed-in-the-pane",
		fmt.Sprintf(`%s -c 'import os; os.close(os.open(os.ttyname(0), os.O_RDWR)); print("its own pty ok")' >> %s 2>&1`, python, pane),
		fmt.Sprintf(`script -q /dev/null sh -c 'echo nested-$((4*4))' >> %[1]s 2>&1; echo "script $?" >> %[1]s`, pane),
		fmt.Sprintf(`%s %s tries %s >> %s 2>&1; echo done >> %s`, python, probe, window.pty, pane, pane),
	} {
		tmux("send-keys", "-t", "main", line, "Enter")
		time.Sleep(200 * time.Millisecond)
	}
	var got string
	for i := 0; i < 50 && !strings.Contains(got, "done"); i++ {
		time.Sleep(100 * time.Millisecond)
		data, _ := os.ReadFile(pane)
		got = string(data)
	}
	for _, want := range []string{"stty 0\n", "read typed-in-the-pane\n", "its own pty ok\n", "nested-16", "script 0\n",
		`{"read": "EPERM", "write": "EPERM", "both": "EPERM", "evtonly": "EPERM"}`} {
		if !strings.Contains(got, want) {
			t.Errorf("the pane said %q, without %q", got, want)
		}
	}
	if got := outside(t, d.base, env, python, probe, "tries", window.pty); got != `{"read": "ok", "write": "ok", "both": "ok", "evtonly": "ok"}`+"\n" {
		t.Errorf("control: outside, the stand-in window %s opens: %q", window.pty, got)
	}

	// The attach: a client in the desk's profile, on the stand-in window.
	tmux("send-keys", "-t", "main", "clear", "Enter")
	client := onTerminal(t, window, d.profile, d.root, env, program, "-f", "/dev/null", "-S", d.ownSocket, "attach", "-t", "main")
	for i := 0; i < 50 && strings.TrimSpace(tmux("list-clients", "-F", "#{client_tty}").stdout) != window.pty; i++ {
		time.Sleep(100 * time.Millisecond)
	}
	tmux("send-keys", "-t", "main", `echo ATTACHED-$((2*21))`, "Enter")
	var seen string
	for i := 0; i < 50 && !strings.Contains(seen, "ATTACHED-42"); i++ {
		time.Sleep(100 * time.Millisecond)
		seen = window.seen(t, "")
	}
	if !strings.Contains(seen, "ATTACHED-42") {
		t.Errorf("the attached window doesn't show the pane: %q", seen)
	}
	window.seen(t, `echo TYPED-$((3*3))\r`)
	if !shows("main", "TYPED-9") {
		t.Errorf("what the attached window typed didn't reach the pane")
	}
	tmux("detach-client", "-s", "main")
	if err := waitWithin(client, 10*time.Second); err != nil {
		t.Errorf("the attach client after detach-client: %v", err)
	}
}
