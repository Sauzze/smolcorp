package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// SA-L3's Linux rows (PL126A §4: CI ubuntu, and the VM): the kernel's own /proc/net/unix read as mask.go reads it, and
// the mask on the real bwrap, through SA-L1's stand-in (TestTheLinuxViewStandIn).

// The test's own sockets are on the computer's list as they are bound: a listening one and a bound datagram one by
// their whole paths, one bound from its folder by its relative name, and an abstract one with its @. The list reads
// whole, as the kernel writes it, and the kernel's dump (NETLINK_SOCK_DIAG) has each by the same name, bound to the
// file at its name (FXSAL3R1), the abstract one bound to none and marked abstract (RT126SAL3R2 F6).
func TestTheComputersListOfUnixSocketsHasTheTestsOwn(t *testing.T) {
	dir := t.TempDir()
	listens, err := net.Listen("unix", filepath.Join(dir, "listens.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listens.Close()
	datagram, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: filepath.Join(dir, "datagram.sock"), Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer datagram.Close()
	abstract := fmt.Sprintf("@smolcorp-stand-in-%d", os.Getpid())
	inAbstract, err := net.Listen("unix", abstract)
	if err != nil {
		t.Fatal(err)
	}
	defer inAbstract.Close()
	t.Chdir(dir)
	relative, err := net.Listen("unix", "relative.sock")
	if err != nil {
		t.Fatal(err)
	}
	defer relative.Close()

	text, err := osView{}.unixSockets()
	if err != nil {
		t.Fatal(err)
	}
	listening, err := parseNetUnix(text)
	if err != nil {
		t.Fatalf("the computer's list doesn't read as the kernel writes it: %v", err)
	}
	dump, err := osView{}.unixDiag()
	if err != nil {
		t.Fatalf("the kernel's dump: %v", err)
	}
	files, err := parseUnixDiag(dump)
	if err != nil {
		t.Fatalf("the kernel's dump doesn't read: %v", err)
	}
	byName := map[string]netSocket{}
	for _, s := range listening {
		byName[s.name] = s
	}
	for _, at := range []string{filepath.Join(dir, "listens.sock"), filepath.Join(dir, "datagram.sock"), "relative.sock"} {
		s, ok := byName[at]
		if !ok {
			t.Errorf("%s isn't on the list", at)
			continue
		}
		d := files[s.ino]
		fi, err := os.Lstat(filepath.Join(dir, filepath.Base(at)))
		if err != nil {
			t.Fatal(err)
		}
		if d.name != at || !d.file || d.vfs != uint32(fileIno(fi)) {
			t.Errorf("%s: the kernel's dump has %+v, want its name and its file's inode %d", at, d, fileIno(fi))
		}
	}
	if s, ok := byName[abstract]; !ok {
		t.Errorf("the abstract %s isn't on the list", abstract)
	} else if d := files[s.ino]; d.name != abstract || !d.abstract || d.file {
		t.Errorf("%s: the kernel's dump has %+v, want it abstract, bound to no file", abstract, d)
	}
	if d := files[byName["relative.sock"].ino]; d.abstract {
		t.Errorf("relative.sock: the kernel's dump marks it abstract")
	}
}

// RT126SAL3 B4's kernel premise, and RT126SAL3R2's P8a, on this computer's kernel (CI ubuntu, and VM run 3): a stand-in
// socket listens in a folder the view brings back writable (~/work/sub). The start masks it. Once that folder is
// renamed (~/work/sub2), /proc/net/unix still lists the name it was bound by, and the kernel's dump the moved file's
// inode, so the start fails (ErrSocketMoved); and so it does with a link at the old name into the desk's own socket
// folder (P8a). A link at the old name to where it went leads the mask there (P8d): /dev/null goes over it. No bwrap
// runs: only the start's words are made.
func TestAMovedSocketFailsTheStartOnTheKernel(t *testing.T) {
	base := t.TempDir()
	if runner := os.Getenv("RUNNER_TEMP"); runner != "" {
		var err error
		if base, err = os.MkdirTemp(runner, "smolcorp-moved-"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(base) })
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, ".config", "sauzzedeck-pro")
	root := filepath.Join(home, "Projects", "app-hires", "h1")
	own := filepath.Join(base, "sockets", "smolcorp-h1.d")
	sub, sub2 := filepath.Join(home, "work", "sub"), filepath.Join(home, "work", "sub2")
	for _, folder := range []string{support + "/hires/h1", root, sub, own, base + "/tmp"} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	l, err := net.Listen("unix", sub+"/dev.sock")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", DeskStart: support + "/desk-start",
		Root: root, SocketParent: filepath.Dir(own), OwnSockets: own}
	start := func(name string, wantErr error, masked string) {
		t.Helper()
		args, err := prepareBwrap(pl, View{Log: t.Logf}, osView{}, base+"/tmp")
		switch {
		case wantErr != nil && !errors.Is(err, wantErr):
			t.Errorf("%s: %v, want %v\n%s", name, err, wantErr, strings.Join(args, " "))
		case wantErr == nil && err != nil:
			t.Errorf("%s: %v, want the start to go on", name, err)
		case masked != "" && !strings.Contains(strings.Join(args, "\n")+"\n", "--ro-bind\n/dev/null\n"+masked+"\n"):
			t.Errorf("%s: no /dev/null over %s:\n%s", name, masked, strings.Join(args, " "))
		default:
			t.Logf("%s: %v", name, err)
		}
	}
	start("where it was made", nil, sub+"/dev.sock")
	if err := os.Rename(sub, sub2); err != nil {
		t.Fatal(err)
	}
	start("its folder renamed (B4)", ErrSocketMoved, "")
	if err := os.Symlink(own, sub); err != nil {
		t.Fatal(err)
	}
	start("P8a a link at the old folder into its own socket folder", ErrSocketMoved, "")
	if err := os.Remove(sub); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sub2, sub); err != nil {
		t.Fatal(err)
	}
	start("P8d a link at the old folder to where it went", nil, sub2+"/dev.sock")
}

// The mask on Smolcorp's bwrap: a stand-in socket listening in the home folder isn't there (ENOENT), one in a folder
// the desk writes, a stream's and a datagram's, is refused (/dev/null over it: ECONNREFUSED); its desk-start socket
// and an agent on its list answer. Where this computer has them, the system D-Bus and Docker's socket aren't there
// (their folders covered), and the system log, on the list, is reached (a datagram socket: a stream's connect gets
// EPROTOTYPE). With SMOLCORP_LINUXCAPS_REQUIRED=1 a missing bwrap fails.
//
// SIERRA's class ruling (FXSAL3R1): a stand-in runtime folder's bus and systemd --user's private socket, each named an
// agent and one reached through a link the desk planted, aren't there (ENOENT); an abstract @/tmp/dbus-* stand-in, which
// answers outside, is refused (ECONNREFUSED: the view's own network namespace has none); and the desk's environment
// holds no DBUS_* the core had.
func TestTheMaskStartsOnBwrap(t *testing.T) {
	if _, err := os.Stat(BwrapPath); err != nil {
		if os.Getenv("SMOLCORP_LINUXCAPS_REQUIRED") == "1" {
			t.Fatalf("no bwrap on this runner: %v", err)
		}
		t.Skip("no bwrap here (the linuxcaps CI job installs it): ", err)
	}
	if err := CheckBwrap(); err != nil {
		t.Fatalf("Smolcorp's bwrap fails the start's check (smolcorp sandbox command puts it in place): %v", err)
	}
	base := t.TempDir()
	if runner := os.Getenv("RUNNER_TEMP"); runner != "" {
		var err error
		if base, err = os.MkdirTemp(runner, "smolcorp-mask-"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(base) })
	}
	home := filepath.Join(base, "home")
	support := filepath.Join(home, ".config", "sauzzedeck-pro")
	root := filepath.Join(home, "Projects", "app-hires", "h1")
	runtime := base + "/runtime"
	for _, folder := range []string{support + "/hires/h1", root, home + "/work", home + "/agent", base + "/tmp", runtime + "/systemd"} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	listen := func(at string) {
		l, err := net.Listen("unix", at)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				c.Close()
			}
		}()
	}
	deskStart := support + "/desk-start"
	for _, at := range []string{deskStart, home + "/stand-in.sock", home + "/work/dev.sock", home + "/agent/ssh.sock",
		runtime + "/bus", runtime + "/systemd/private"} {
		listen(at)
	}
	if err := os.Symlink(runtime+"/bus", home+"/agent/planted.sock"); err != nil {
		t.Fatal(err)
	}
	abstractBus := fmt.Sprintf("@/tmp/dbus-smolcorp-stand-in-%d", os.Getpid())
	listen(abstractBus)
	if c, err := net.Dial("unix", abstractBus); err != nil {
		t.Fatalf("the abstract stand-in doesn't answer outside: %v", err)
	} else {
		c.Close()
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:abstract="+strings.TrimPrefix(abstractBus, "@"))
	t.Setenv("DBUS_SMOLCORP_STAND_IN", "1")
	datagram, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: home + "/work/datagram.sock", Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer datagram.Close()

	rows := map[string]string{
		"connect a stand-in socket in ~": home + "/stand-in.sock", "connect a stand-in socket in a folder it writes": home + "/work/dev.sock",
		"connect a stand-in datagram socket in a folder it writes": home + "/work/datagram.sock",
		"connect desk-start": deskStart, "connect an agent on its list": home + "/agent/ssh.sock",
		"connect the session bus stand-in": runtime + "/bus", "connect systemd --user's stand-in": runtime + "/systemd/private",
		"connect a planted link to the bus": home + "/agent/planted.sock", "connect an abstract @/tmp/dbus-* stand-in": abstractBus,
		"env DBUS_SESSION_BUS_ADDRESS unset": "DBUS_SESSION_BUS_ADDRESS", "env DBUS_SMOLCORP_STAND_IN unset": "DBUS_SMOLCORP_STAND_IN",
	}
	want := map[string]string{
		"connect a stand-in socket in ~": "ENOENT", "connect a stand-in socket in a folder it writes": "ECONNREFUSED",
		"connect a stand-in datagram socket in a folder it writes": "ECONNREFUSED",
		"connect desk-start": "ok", "connect an agent on its list": "ok",
		"connect the session bus stand-in": "ENOENT", "connect systemd --user's stand-in": "ENOENT",
		"connect a planted link to the bus": "ENOENT", "connect an abstract @/tmp/dbus-* stand-in": "ECONNREFUSED",
		"env DBUS_SESSION_BUS_ADDRESS unset": "ok", "env DBUS_SMOLCORP_STAND_IN unset": "ok",
	}
	// As SocketPaths gave them before FXSAL3R1: the planted link and where it leads, and the bus named itself.
	agents := []string{home + "/agent/ssh.sock", home + "/agent/planted.sock", runtime + "/bus", runtime + "/systemd/private"}
	for name, at := range map[string]string{"the system D-Bus": "/run/dbus/system_bus_socket", "Docker's socket": "/run/docker.sock"} {
		if st, err := os.Stat(at); err == nil && st.Mode()&os.ModeSocket != 0 {
			rows["connect "+name] = at
			want["connect "+name] = "ENOENT"
		} else {
			t.Logf("no %s on this computer (%s): its row is left out", name, at)
		}
	}
	if st, err := os.Stat("/dev/log"); err == nil && st.Mode()&os.ModeSocket != 0 {
		rows["connect the system log"], want["connect the system log"] = "/dev/log", "EPROTOTYPE"
		agents = append(agents, "/dev/log")
	}

	pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", DeskStart: deskStart, Root: root}
	var logged []string
	v := View{Covered: []string{runtime}, Agents: agents, Log: func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }}
	args, err := prepareBwrap(pl, v, osView{}, base+"/tmp")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("the start logged: %q", logged)
	// VM run 4 (Docker installed and running; VM run 3 B1, B2): containerd's folder, and Docker's when a socket listens
	// in it, are folders this account can't open. The start went on, and named each one.
	list, _ := os.ReadFile(netUnix)
	for folder, seen := range map[string]bool{"/run/containerd": true,
		"/run/docker": strings.Contains(string(list), " /var/run/docker/") || strings.Contains(string(list), " /run/docker/")} {
		if _, err := os.ReadDir(folder); seen && errors.Is(err, fs.ErrPermission) &&
			!slices.ContainsFunc(logged, func(l string) bool { return strings.Contains(l, "can't open "+folder+",") }) {
			t.Errorf("%s, which this account can't open, isn't named in the log", folder)
		}
	}
	try, err := json.Marshal(viewTry{Support: support, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, BwrapPath, append(args, self, "-test.run=^TestTheLinuxViewStandIn$", "-test.v")...)
	cmd.Env = append(os.Environ(), viewStandIn+"="+string(try))
	out, err := cmd.CombinedOutput()
	t.Logf("bwrap %s\n%s", strings.Join(args, " "), out)
	if err != nil {
		t.Fatalf("the view didn't start, or the stand-in failed: %v", err)
	}
	_, line, ok := strings.Cut(string(out), "view-rows: ")
	if !ok {
		t.Fatal("the stand-in reported nothing")
	}
	line, _, _ = strings.Cut(line, "\n")
	var got map[string]string
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: got %s, want %s", name, got[name], w)
		}
	}
}

// busCommands are SIERRA's three (FXSAL3R1): each reaches systemd --user or the session bus, and fails without one.
var busCommands = map[string][]string{
	"run systemd-run --user true": {"systemd-run", "--user", "true"},
	"run busctl --user list":      {"busctl", "--user", "list"},
	"run gdbus call --session":    {"gdbus", "call", "--session", "--dest", "org.freedesktop.DBus", "--object-path", "/org/freedesktop/DBus", "--method", "org.freedesktop.DBus.ListNames"},
}

// VM run 3 (FXSAL3R1, SIERRA's class ruling): in a real desktop session, SIERRA's three commands reach systemd --user
// and the session bus outside (the positive control), and each fails inside the view, started as a desk's start
// starts it: the runtime folder covered, the agents from SocketPaths, the core's environment. Inside, each must fail
// as busFailures says, and no other way (RT126SAL3R2). Where the session has X (always in an X11 session, where
// Xwayland answers in a Wayland one), X11's own abstract socket answers outside and is refused inside (ECONNREFUSED:
// the view's network namespace has none). Runs only with SMOLCORP_DESKTOP_SESSION=1, set by hand in the session; each
// command's output, inside and out, is logged, in the C locale.
func TestNoSessionBusNorSystemdUserInsideTheView(t *testing.T) {
	if os.Getenv("SMOLCORP_DESKTOP_SESSION") != "1" {
		t.Skip("a desktop session's row: SMOLCORP_DESKTOP_SESSION=1 runs it (VM run 3)")
	}
	if err := CheckBwrap(); err != nil {
		t.Fatalf("Smolcorp's bwrap fails the start's check: %v", err)
	}
	cLocale := "LC_ALL=C.UTF-8"
	for name, words := range busCommands {
		cmd := exec.Command(words[0], words[1:]...)
		cmd.Env = append(os.Environ(), cLocale)
		out, err := cmd.CombinedOutput()
		t.Logf("outside, %s: %s: %s", name, errName(err), firstLines(string(out), 3))
		if err != nil {
			t.Errorf("outside, %s fails: the positive control needs the desktop session's bus", name)
		}
	}
	base := t.TempDir()
	home := filepath.Join(base, "home")
	support := filepath.Join(home, ".config", "sauzzedeck-pro")
	root := filepath.Join(home, "Projects", "app-hires", "h1")
	for _, folder := range []string{support + "/hires/h1", root, base + "/tmp"} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	realHome, _ := os.UserHomeDir()
	pl := Places{GOOS: "linux", Subject: Hire, Home: home, Support: support, OwnID: "h1", DeskStart: support + "/desk-start", Root: root}
	v := View{Covered: []string{os.Getenv("XDG_RUNTIME_DIR"), "/run/screen"},
		Agents: SocketPaths(Sockets("linux"), realHome, os.Getenv, true, false)}
	args, err := prepareBwrap(pl, v, osView{}, base+"/tmp")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for name, words := range busCommands {
		rows[name] = strings.Join(words, "\n")
	}
	const x11Row = "connect X11's abstract socket"
	if display := os.Getenv("DISPLAY"); display != "" {
		n, _, _ := strings.Cut(strings.TrimPrefix(display, ":"), ".")
		x11 := "@/tmp/.X11-unix/X" + n
		c, err := net.Dial("unix", x11)
		switch {
		case err == nil:
			c.Close()
			rows[x11Row] = x11
			t.Logf("outside, %s %s: ok", x11Row, x11)
		case os.Getenv("XDG_SESSION_TYPE") == "x11":
			t.Errorf("outside, %s %s doesn't answer in an X11 session: %v", x11Row, x11, err)
		default:
			t.Logf("outside, %s %s doesn't answer (%v): its row is left out", x11Row, x11, err)
		}
	}
	try, err := json.Marshal(viewTry{Support: support, Rows: rows})
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, BwrapPath, append(args, self, "-test.run=^TestTheLinuxViewStandIn$", "-test.v")...)
	cmd.Env = append(os.Environ(), cLocale, viewStandIn+"="+string(try))
	out, err := cmd.CombinedOutput()
	t.Logf("bwrap %s", strings.Join(args, " "))
	if err != nil {
		t.Fatalf("the view didn't start, or the stand-in failed: %v\n%s", err, out)
	}
	_, line, ok := strings.Cut(string(out), "view-rows: ")
	if !ok {
		t.Fatalf("the stand-in reported nothing:\n%s", out)
	}
	line, _, _ = strings.Cut(line, "\n")
	var got map[string]string
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	for name := range busCommands {
		t.Logf("inside, %s", firstLines(got[name], 3))
		if !busFailures[name].MatchString(got[name]) {
			t.Errorf("inside, %s doesn't fail as it does without a bus (any other failure is a FAIL): %q", name, got[name])
		}
	}
	if x11, ok := rows[x11Row]; ok {
		t.Logf("inside, %s %s: %s", x11Row, x11, got[x11Row])
		if got[x11Row] != "ECONNREFUSED" {
			t.Errorf("inside, %s %s: %s, want ECONNREFUSED", x11Row, x11, got[x11Row])
		}
	}
}

// firstLines is s's first n lines, on one.
func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[:min(n, len(lines))], " | ")
}
