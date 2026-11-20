package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-M1 on this Mac, with stand-ins only (owner Q5 (a)): the real profile, made by PrepareProfile for a stand-in hire
// whose places are all in the test's temp folder, run by /usr/bin/sandbox-exec -f from its file at
// hires/<id>/sandbox.sb. A stand-in hire (python) inside tries each row of the profile's: the support folder whole
// (a stand-in key read, a file added, the folder listed, the canary read, serve.sock connected to, another desk's
// brief) and its exceptions (the guard file and its brief read, its status written, its Claude folder, the desk-start
// socket); the rename guards on the support folder, its worktree, the project and the folders above them; no new
// .claude, in any case; the socket folder written only in its own folder, and no link in that either; the plants, the
// ssh keys, the account folders, the shared git places and the other desks' places. Absent names are tried in another
// case and decomposed (NFD), as the Mac's volume takes them for the same file. Each check is only "ok" or the error's
// name; the stand-in key is never printed. The "ok" rows show the stand-in really runs.

// sbProbe does each step of its JSON argument, a list of [what, path, (path)], and prints a JSON list of "ok" or the
// error's name, one per step.
const sbProbe = `import json, os, socket, sys
said = []
for step in json.loads(sys.argv[1]):
    what, at = step[0], step[1]
    try:
        if what == "read":
            with open(at) as f:
                f.read()
        elif what == "write":
            with open(at, "w") as f:
                f.write("standin")
        elif what == "list":
            os.listdir(at)
        elif what == "stat":
            os.stat(at)
        elif what == "mkdir":
            os.mkdir(at)
        elif what == "rmdir":
            os.rmdir(at)
        elif what == "unlink":
            os.unlink(at)
        elif what == "rename":
            os.rename(at, step[2])
        elif what == "link":
            os.link(at, step[2])
        elif what == "symlink":
            os.symlink(step[2], at)
        elif what == "chmod":
            os.chmod(at, 0o700)
        elif what == "connect":
            s = socket.socket(socket.AF_UNIX)
            s.connect(at)
            s.close()
        said.append("ok")
    except OSError as e:
        said.append(type(e).__name__)
print(json.dumps(said))`

// nfd is s with its é decomposed, as HFS+ and many tools write it.
func nfd(s string) string { return strings.ReplaceAll(s, "é", "é") }

func TestOnAMacTheProfileHoldsForAStandInHire(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	// A short folder: a unix socket's path must fit in 104 bytes.
	made, err := os.MkdirTemp("", "sbx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(made) })
	base, err := filepath.EvalSymlinks(made)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "hé")
	support := filepath.Join(home, "L", "AD")
	own := filepath.Join(support, "hires", "h1")
	hires := filepath.Join(home, "P", "app-hires")
	root, other := filepath.Join(hires, "h1"), filepath.Join(hires, "h2")
	repo := filepath.Join(home, "P", "app")
	git := filepath.Join(repo, ".git")
	sockets := filepath.Join(base, "k")
	files := map[string]string{
		filepath.Join(support, "setup-key"): "FAKEKEY", filepath.Join(support, "sandbox-canary-1"): "standin",
		filepath.Join(support, "hire-guard.json"): `{"fence":1}`, filepath.Join(own, "BRIEF.md"): "brief", filepath.Join(own, "settings.json"): `{"sealed":1}`,
		filepath.Join(support, "hires", "h2", "BRIEF.md"): "another brief", filepath.Join(own, "claude", "todos.json"): "[]",
		filepath.Join(root, ".claude", "commands", "x.md"): "x", filepath.Join(other, "main.go"): "package main",
		filepath.Join(repo, "main.go"): "package main", filepath.Join(git, "config"): "[core]\n", filepath.Join(git, "HEAD"): "ref: refs/heads/main\n",
		filepath.Join(git, "refs", "heads", "main"): "1111\n", filepath.Join(git, "objects", "info", "packs"): "",
		filepath.Join(git, "worktrees", "h1", "HEAD"): "1111\n", filepath.Join(home, ".ssh", "id_x"): "FAKEKEY",
		filepath.Join(home, ".ssh", "id_x.pub"): "ssh-ed25519 standin", filepath.Join(home, ".claude", "CLAUDE.md"): "notes",
		filepath.Join(home, ".claude", ".credentials.json"): "FAKEKEY", filepath.Join(home, ".claude.json"): "{}",
		filepath.Join(sockets, "h2", "s"): "", filepath.Join(sockets, "h1", "f"): "", filepath.Join(base, "elsewhere", "codex", "auth.json"): "standin",
		filepath.Join(home, ".gemini", "settings.json"): "{}",
	}
	for at, text := range files {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A link in its place to its settings file (SETTINGSFILE1): written through, it's the file's own row that answers.
	if err := os.Symlink(filepath.Join(own, "settings.json"), filepath.Join(root, "fence-link")); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(root, "sub"), filepath.Join(root, "pkg")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, at := range []string{filepath.Join(support, "desk-start"), filepath.Join(support, "serve.sock")} {
		l, err := net.Listen("unix", at)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", DeskStart: filepath.Join(support, "desk-start"),
		Root: root, Checkout: repo, GitDir: git, OwnWorktree: "h1", OtherWorktrees: []string{"h1", "h2"}, DefaultBranch: "dé",
		HiresFolder: hires, SocketParent: sockets, OwnSockets: filepath.Join(sockets, "h1"), RemoteLogin: true})

	// PrepareProfile makes the Policy's Hold and writes the profile, owner-only, through a rename.
	file := filepath.Join(own, "sandbox.sb")
	if err := os.Symlink(filepath.Join(base, "elsewhere", "followed.sb"), file); err != nil {
		t.Fatal(err)
	}
	if err := PrepareProfile(p, file); err != nil {
		t.Fatal(err)
	}
	want, _ := p.Profile()
	info, err := os.Lstat(file)
	if data, _ := os.ReadFile(file); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || string(data) != want {
		t.Fatalf("the profile's file: %v %v, its text the profile's: %v", info.Mode(), err, string(data) == want)
	}
	if _, err := os.Lstat(filepath.Join(base, "elsewhere", "followed.sb")); err == nil {
		t.Error("PrepareProfile followed a link at the profile's place")
	}
	if names, _ := os.ReadDir(own); len(names) != 4 {
		t.Errorf("a temporary file was left beside the profile: %v", names)
	}
	gemini, held := filepath.Join(home, ".gemini"), filepath.Join(home, ".config", "systemd")
	if info, err := os.Lstat(held); err != nil || !info.IsDir() {
		t.Fatalf("the Hold folder %s wasn't made: %v", held, err)
	}

	type step struct {
		what, at, to, want string
	}
	const ok, no = "ok", "PermissionError"
	steps := []step{
		// The support folder, whole: a key, a file added, the folder listed, the canary (EPERM, not ENOENT), the owner's
		// tap, another desk's brief, a hard link out of it. A file's metadata stays readable.
		{"read", support + "/setup-key", "", no}, {"write", support + "/added", "", no}, {"list", support, "", no},
		{"read", support + "/sandbox-canary-1", "", no}, {"connect", support + "/serve.sock", "", no},
		{"read", support + "/hires/h2/BRIEF.md", "", no}, {"link", support + "/setup-key", root + "/key", no},
		{"stat", support + "/setup-key", "", ok}, {"read", nfd(support) + "/SETUP-KEY", "", no},
		// …its exceptions: the guard file read, not written; its brief and fence read, not written; its status written;
		// its Claude folder, not its settings; the desk-start socket.
		{"read", support + "/hire-guard.json", "", ok}, {"write", support + "/hire-guard.json", "", no},
		{"read", own + "/BRIEF.md", "", ok}, {"write", own + "/BRIEF.md", "", no}, {"read", own + "/sandbox.sb", "", ok},
		{"write", own + "/sandbox.sb", "", no}, {"write", own + "/STATUS.md", "", ok}, {"write", own + "/claude/todos.json", "", ok},
		{"write", own + "/claude/settings.json", "", no}, {"connect", support + "/desk-start", "", ok},
		// …its settings file (SETTINGSFILE1): read, never written, replaced, moved, removed, linked or written through a link.
		{"read", own + "/settings.json", "", ok}, {"write", own + "/settings.json", "", no}, {"chmod", own + "/settings.json", "", no},
		{"rename", own + "/STATUS.md", own + "/settings.json", no}, {"rename", own + "/settings.json", own + "/NOTES.md", no},
		{"unlink", own + "/settings.json", "", no}, {"link", own + "/settings.json", own + "/NOTES.md", no},
		{"symlink", own + "/NOTES.md", own + "/settings.json", ok}, {"write", own + "/NOTES.md", "", no}, {"write", root + "/fence-link", "", no},
		// The rename guards: the support folder, its worktree, the project, the folders above them, a Hold folder.
		{"rename", support, base + "/elsewhere/AD", no}, {"rename", root, hires + "/h1-moved", no}, {"rename", hires, base + "/elsewhere/h", no},
		{"rename", repo, base + "/elsewhere/app", no}, {"rename", home + "/L", base + "/elsewhere/L", no}, {"rmdir", held, "", no},
		{"rename", nfd(home) + "/P", base + "/elsewhere/P", no}, {"rename", root + "/pkg", root + "/pkg2", ok},
		// No new .claude, in any case; its fences.
		{"mkdir", root + "/sub/.claude", "", no}, {"mkdir", root + "/sub/.CLAUDE", "", no}, {"mkdir", root + "/sub/.claudex", "", ok},
		{"write", root + "/.claude/settings.json", "", no}, {"write", root + "/.claude/Settings.Local.json", "", no},
		{"write", root + "/.claude/commands/y.md", "", ok}, {"rename", root + "/.claude", root + "/claude-moved", no},
		// The socket folder: only its own folder, and no link in that.
		{"mkdir", sockets + "/h9", "", no}, {"unlink", sockets + "/h2/s", "", no}, {"write", sockets + "/h1/g", "", ok},
		{"symlink", sockets + "/h1/l", base + "/elsewhere", no}, {"link", sockets + "/h1/f", sockets + "/h1/f2", no},
		// The plants, absent ones in another case and decomposed; the ssh keys while Remote Login answers; the account
		// folders; another agent's sign-in, which the Hold folder above it can't bring in.
		{"write", home + "/.zshrc", "", no}, {"write", nfd(home) + "/.ZSHRC", "", no}, {"write", home + "/.zshrc-notes", "", ok},
		{"read", home + "/.ssh/id_x", "", no}, {"read", home + "/.ssh/id_x.pub", "", ok}, {"write", home + "/.ssh/authorized_keys", "", no},
		{"read", home + "/.claude/CLAUDE.md", "", ok}, {"write", home + "/.claude/CLAUDE.md", "", no},
		{"read", home + "/.claude/.credentials.json", "", no}, {"read", home + "/.claude.json", "", no},
		{"write", gemini + "/oauth_creds.json", "", no}, {"rename", base + "/elsewhere/codex", home + "/.codex", no},
		// The shared git places, the base branches (the default one spelled in another case and decomposed), and the
		// rest of the shared .git; other desks' places.
		{"write", git + "/config", "", no}, {"write", git + "/refs/heads/main", "", no}, {"write", git + "/refs/heads/" + nfd("dé"), "", no},
		{"write", git + "/refs/heads/DÉ", "", no}, {"chmod", git + "/refs", "", no}, {"write", git + "/objects/info/alternates", "", no},
		// A branch of its own, hire/h1, as every real hire has, since a desk writes no other from FXGITROWS2R4 (this
		// stand-in moved in FXGITROWS2R6).
		{"write", git + "/refs/heads/hire/h1", "", ok}, {"write", git + "/worktrees/h1/HEAD", "", ok},
		{"write", other + "/main.go", "", no}, {"write", repo + "/main.go", "", no}, {"write", root + "/main.go", "", ok},
	}
	var args [][]string
	for _, s := range steps {
		arg := []string{s.what, s.at}
		if s.to != "" {
			arg = append(arg, s.to)
		}
		args = append(args, arg)
	}
	encoded, _ := json.Marshal(args)
	cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", file, python, "-I", "-c", sbProbe, string(encoded)))
	cmd.Dir, cmd.Env = base, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the stand-in didn't run under the profile: %v %s", err, stderr.Bytes())
	}
	var said []string
	if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in said %q (%v)", out.Bytes(), err)
	}
	for i, s := range steps {
		if said[i] != s.want {
			t.Errorf("%s %s %s: %s, want %s", s.what, strings.TrimPrefix(s.at, base), strings.TrimPrefix(s.to, base), said[i], s.want)
		}
	}
	if data, _ := os.ReadFile(filepath.Join(support, "hire-guard.json")); string(data) != `{"fence":1}` {
		t.Errorf("the guard file is %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(own, "settings.json")); string(data) != `{"sealed":1}` {
		t.Errorf("the settings file is %q", data)
	}
}

// SA-M2 on this Mac, with stand-ins only (owner Q5 (a)): sockets, services, signals and network under the real profile,
// made by PrepareProfile with a stand-in desk's Services, and once outside any sandbox, where every step must work, so
// a refusal inside is the profile's. Each step is one harmless thing, said as "ok", an error's name, or a Mach or
// IOKit return code: a connect to a stand-in unix socket the test listens on (its own, through a link to it, another
// desk's, a link in its own folder to another desk's, one on its list spelled through /var, one not on it, an agent,
// the desk-start socket, the owner's tap); a stand-in ssh key read (C5); a connect to the test's own TCP listeners on
// ports 7750-7799 only, five standing in for :22, Docker's and the databases' through the same profile maker (the real
// ports' lines are in the goldens), and one not on the list (C4, below); a Mach service looked up, never sent
// anything; an IOKit service opened and closed at once; SIGCONT to a stand-in `sleep` outside, and SIGTERM to its own
// child, which ends by itself in 3 s if the signal doesn't reach it (C1); TIOCSTI on a pty of its own, echo off (S3).
// Nothing is typed into, sent to or read from a real program.
//
// C4 on each denied port (RT126SAM2 B1, SIERRA's ruling): 127.0.0.1, 0.0.0.0, ::ffff:127.0.0.1, ::ffff:0.0.0.0, ::1
// and :: are refused, and so are the Mac's own network addresses (Seatbelt's localhost is all of them), started
// without waiting and never listened on, so no firewall asks. A plain IPv4 remote isn't refused by the profile: a
// stand-in listener can't be one (every address here is localhost), so the remote is 224.0.0.1, which TCP itself
// refuses after the profile let it by (EAFNOSUPPORT, nothing sent), where an IPv6 one is EPERM. A stand-in client
// that does what ssh and libpq do (each address in turn, the next one on an error) gets EPERM for the IPv6 address at
// once and goes on to the IPv4 one. AQ21's opt-in leaves a port off the list: the port not on it answers on every
// address, IPv4-mapped too.

// sbServiceProbe does each step of its JSON argument and prints a JSON list of what each said.
const sbServiceProbe = `import ctypes, errno, fcntl, json, os, pwd, signal, socket, sys, termios, time
said = []
libc = ctypes.CDLL(None)
iokit = ctypes.CDLL("/System/Library/Frameworks/IOKit.framework/IOKit")
iokit.IOServiceMatching.restype = ctypes.c_void_p
iokit.IOServiceGetMatchingService.argtypes = [ctypes.c_uint, ctypes.c_void_p]
iokit.IOServiceOpen.argtypes = [ctypes.c_uint, ctypes.c_uint, ctypes.c_uint, ctypes.POINTER(ctypes.c_uint)]
def mach(name):
    port = ctypes.c_uint(0)
    kr = libc.bootstrap_look_up(ctypes.c_uint.in_dll(libc, "bootstrap_port"), name.encode(), ctypes.byref(port))
    return "ok" if kr == 0 else "kr%d" % kr
def io(name):
    service = iokit.IOServiceGetMatchingService(0, iokit.IOServiceMatching(name.encode()))
    if not service:
        return "no service"
    conn = ctypes.c_uint(0)
    kr = iokit.IOServiceOpen(service, ctypes.c_uint.in_dll(libc, "mach_task_self_").value, 0, ctypes.byref(conn))
    if kr == 0:
        iokit.IOServiceClose(conn)
        return "ok"
    return "kr%x" % (kr & 0xffffffff)
def unix(at):
    s = socket.socket(socket.AF_UNIX)
    s.connect(at)
    s.close()
def tcp(host, port):
    s = socket.socket(socket.AF_INET6 if ":" in host else socket.AF_INET)
    s.settimeout(3)
    s.connect((host, port))
    s.close()
def start(host, port):
    s = socket.socket(socket.AF_INET6 if ":" in host else socket.AF_INET)
    s.setblocking(False)
    try:
        s.connect((host, port))
    except OSError as e:
        if e.errno not in (errno.EINPROGRESS, errno.ECONNREFUSED):
            raise
    finally:
        s.close()
    return "started"
def dial(hosts, port):
    tried = []
    for host in hosts:
        try:
            tcp(host, port)
            return " then ".join(tried + ["ok"])
        except OSError as e:
            tried.append(errno.errorcode.get(e.errno, str(e.errno)))
    return " then ".join(tried)
def read(at):
    with open(at) as f:
        f.read()
def own_child():
    pid = os.fork()
    if pid == 0:
        os.closerange(0, 3)
        time.sleep(3)
        os._exit(0)
    os.kill(pid, signal.SIGTERM)
    os.waitpid(pid, 0)
def sti():
    m, s = os.openpty()
    a = termios.tcgetattr(s)
    a[3] &= ~termios.ECHO
    termios.tcsetattr(s, termios.TCSANOW, a)
    pid = os.fork()
    if pid == 0:
        try:
            os.setsid()
            fcntl.ioctl(os.open(os.ttyname(s), os.O_RDWR), termios.TIOCSTI, b" ")
            os._exit(0)
        except OSError as e:
            os._exit(e.errno)
    code = os.waitstatus_to_exitcode(os.waitpid(pid, 0)[1])
    return "ok" if code == 0 else errno.errorcode.get(code, str(code))
def late(*_):
    raise OSError(errno.ETIMEDOUT, "late")
signal.signal(signal.SIGALRM, late)
do = {"unix": unix, "tcp": tcp, "tcp start": start, "dial": dial, "read": read, "mach": mach, "io": io, "own child": own_child, "sti": sti,
      "signal": lambda pid: os.kill(pid, signal.SIGCONT), "whoami": lambda: pwd.getpwuid(os.getuid()) and None}
for step in json.loads(sys.argv[1]):
    signal.alarm(5)
    try:
        r = do[step[0]](*step[1:])
        said.append("ok" if r is None else r)
    except OSError as e:
        said.append(errno.errorcode.get(e.errno, str(e.errno)))
signal.alarm(0)
print(json.dumps(said))`

// testPorts are n TCP ports in 7750-7799 this test listens on, on 127.0.0.1 and, where this Mac has it, ::1 (v6).
func testPorts(t *testing.T, n int) (ports []int, v6 bool) {
	t.Helper()
	if l, err := net.Listen("tcp", "[::1]:0"); err == nil {
		l.Close()
		v6 = true
	}
	hosts := []string{"127.0.0.1"}
	if v6 {
		hosts = append(hosts, "::1")
	}
	for port := 7750; port < 7800 && len(ports) < n; port++ {
		var ls []net.Listener
		for _, host := range hosts {
			if l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port))); err == nil {
				ls = append(ls, l)
			}
		}
		t.Cleanup(func() {
			for _, l := range ls {
				l.Close()
			}
		})
		if len(ls) == len(hosts) {
			ports = append(ports, port)
		}
	}
	if len(ports) < n {
		t.Fatalf("only %d free ports in 7750-7799, want %d", len(ports), n)
	}
	return ports, v6
}

// ownAddresses are this Mac's own network addresses (not loopback or link-local), one IPv4 and one IPv6 where it has
// them. Nothing listens on them: the test only starts a connect to them.
func ownAddresses() (v4, v6 string) {
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || !n.IP.IsGlobalUnicast() {
			continue
		}
		if n.IP.To4() != nil && v4 == "" {
			v4 = n.IP.String()
		} else if n.IP.To4() == nil && v6 == "" {
			v6 = n.IP.String()
		}
	}
	return v4, v6
}

func TestOnAMacTheServicesHoldForAStandInDesk(t *testing.T) {
	sandboxExec, python := "/usr/bin/sandbox-exec", python3(t)
	if _, err := os.Stat(sandboxExec); err != nil || python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec (%v) or python3 (%q) missing", err, python)
		}
		t.Skip("no sandbox-exec or python3 here")
	}
	// A short folder: a unix socket's path must fit in 104 bytes. made is spelled as the system gives it (/var/...).
	made, err := os.MkdirTemp("", "sbs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(made) })
	base, err := filepath.EvalSymlinks(made)
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "h")
	support := filepath.Join(home, "AD")
	own := filepath.Join(support, "hires", "h1")
	root := filepath.Join(base, "w", "h1")
	sockets := filepath.Join(base, "k")
	elsewhere := filepath.Join(base, "e")
	for _, dir := range []string{own, root, filepath.Join(sockets, "h1"), filepath.Join(sockets, "h2"), elsewhere, filepath.Join(home, ".ssh")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "id_x"), []byte("FAKEKEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	at := map[string]string{
		"own": filepath.Join(sockets, "h1", "s"), "other desk's": filepath.Join(sockets, "h2", "s"), "listed": filepath.Join(elsewhere, "listed"),
		"unlisted": filepath.Join(elsewhere, "unlisted"), "agent": filepath.Join(elsewhere, "agent"),
		"desk-start": filepath.Join(support, "desk-start"), "the owner's tap": filepath.Join(support, "serve.sock"),
	}
	for _, sock := range at {
		l, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
	}
	// Links made before the desk: one to its own socket, and one in its own folder to another desk's server (T8).
	toOwn, toOther := filepath.Join(elsewhere, "to-own"), filepath.Join(sockets, "h1", "l")
	if err := os.Symlink(at["own"], toOwn); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(at["other desk's"], toOther); err != nil {
		t.Fatal(err)
	}
	ports, v6 := testPorts(t, 6)
	denied := ports[:5] // the last one is not on the list
	outside := disclaim.Cmd(exec.Command("/bin/sleep", "60"))
	if err := outside.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { outside.Process.Kill(); outside.Wait() })

	svc := Services{OwnSocket: at["own"], Sockets: []string{filepath.Join(made, "e", "listed")}, Agents: []string{at["agent"]},
		Mach: []string{"com.apple.system.opendirectoryd.libinfo"}, IOKit: []IOKitOpen{{Class: "RootDomainUserClient"}}, DeniedPorts: denied}
	if !strings.HasPrefix(svc.Sockets[0], "/var/") {
		t.Logf("the listed socket isn't spelled through /var here: %s", svc.Sockets[0])
	}
	type step struct {
		what string
		args []any
		// outside, inside with Remote Login off, inside with it on
		want [3]string
	}
	const ok, no = "ok", "EPERM"
	steps := []step{
		{"unix", []any{at["own"]}, [3]string{ok, ok, ok}},
		{"unix", []any{toOwn}, [3]string{ok, ok, ok}},
		{"unix", []any{at["other desk's"]}, [3]string{ok, no, no}},
		{"unix", []any{toOther}, [3]string{ok, no, no}},
		{"unix", []any{at["listed"]}, [3]string{ok, ok, ok}},
		{"unix", []any{at["unlisted"]}, [3]string{ok, no, no}},
		{"unix", []any{at["agent"]}, [3]string{ok, ok, no}},
		{"unix", []any{at["desk-start"]}, [3]string{ok, ok, ok}},
		{"unix", []any{at["the owner's tap"]}, [3]string{ok, no, no}},
		{"read", []any{filepath.Join(home, ".ssh", "id_x")}, [3]string{ok, ok, no}},
		{"mach", []any{"com.apple.system.opendirectoryd.libinfo"}, [3]string{ok, ok, ok}},
		{"mach", []any{"com.apple.trustd.agent"}, [3]string{ok, "kr1100", "kr1100"}},
		{"whoami", nil, [3]string{ok, ok, ok}},
		{"io", []any{"IOPMrootDomain"}, [3]string{ok, ok, ok}},
		{"io", []any{"IOSurfaceRoot"}, [3]string{ok, "kre00002e2", "kre00002e2"}},
		{"signal", []any{outside.Process.Pid}, [3]string{ok, no, no}},
		{"own child", nil, [3]string{ok, ok, ok}},
		{"sti", nil, [3]string{ok, no, no}},
	}
	// C4, on each denied port and the one not on the list (see above).
	lan4, lan6 := ownAddresses()
	if lan4 == "" {
		t.Log("no IPv4 network address here: its rows are left out")
	}
	for i, port := range ports {
		// What a connect says inside: refused on a denied port, as outside on the one not on the list.
		const tcpOnly = "EAFNOSUPPORT"
		in, started, bothLocal, v6Remote := no, no, no+" then "+no, no
		if i == len(denied) {
			in, started, bothLocal, v6Remote = ok, "started", ok, tcpOnly
		}
		local := []string{"127.0.0.1", "0.0.0.0", "::ffff:127.0.0.1", "::ffff:0.0.0.0"}
		if v6 {
			local = append(local, "::1", "::")
			steps = append(steps, step{"dial", []any{[]string{"::1", "127.0.0.1"}, port}, [3]string{ok, bothLocal, bothLocal}})
		}
		for _, host := range local {
			steps = append(steps, step{"tcp", []any{host, port}, [3]string{ok, in, in}})
		}
		var own []string
		if lan4 != "" {
			own = append(own, lan4, "::ffff:"+lan4)
		}
		if lan6 != "" {
			own = append(own, lan6)
		}
		for _, host := range own {
			steps = append(steps, step{"tcp start", []any{host, port}, [3]string{"started", started, started}})
		}
		steps = append(steps,
			step{"tcp start", []any{"224.0.0.1", port}, [3]string{tcpOnly, tcpOnly, tcpOnly}},
			step{"tcp start", []any{"ff02::1", port}, [3]string{tcpOnly, v6Remote, v6Remote}},
			step{"dial", []any{[]string{"ff02::1", "224.0.0.1"}, port}, [3]string{tcpOnly + " then " + tcpOnly, v6Remote + " then " + tcpOnly, v6Remote + " then " + tcpOnly}})
	}
	var args [][]any
	for _, s := range steps {
		args = append(args, append([]any{s.what}, s.args...))
	}
	encoded, _ := json.Marshal(args)
	run := func(name string, argv ...string) []string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := disclaim.Cmd(exec.CommandContext(ctx, argv[0], append(argv[1:], python, "-I", "-c", sbServiceProbe, string(encoded))...))
		cmd.WaitDelay = 5 * time.Second
		cmd.Dir, cmd.Env = base, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1"}
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s: the stand-in didn't run: %v %s", name, err, stderr.Bytes())
		}
		var said []string
		if err := json.Unmarshal(out.Bytes(), &said); err != nil || len(said) != len(steps) {
			t.Fatalf("%s: the stand-in said %q (%v)", name, out.Bytes(), err)
		}
		return said
	}
	for i, remoteLogin := range []bool{false, false, true} {
		name, argv := "outside", []string{"/usr/bin/env"}
		if i > 0 {
			p := mustNew(t, Places{GOOS: "darwin", Home: home, Support: support, OwnID: "h1", DeskStart: at["desk-start"], Root: root,
				HiresFolder: filepath.Dir(root), SocketParent: sockets, OwnSockets: filepath.Join(sockets, "h1"), RemoteLogin: remoteLogin})
			file := filepath.Join(own, "sandbox.sb")
			if err := PrepareProfile(p, file, svc); err != nil {
				t.Fatal(err)
			}
			name, argv = fmt.Sprintf("inside, Remote Login answering: %v", remoteLogin), []string{sandboxExec, "-f", file}
		}
		said := run(name, argv...)
		for j, s := range steps {
			if said[j] != s.want[i] {
				t.Errorf("%s: %s %v: %s, want %s", name, s.what, s.args, said[j], s.want[i])
			}
		}
	}
}
