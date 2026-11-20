//go:build darwin && cgo

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FXFIREREAPR5's row (RT126FIXREVIEW-FIREREAPR4 B1): a sandbox extension, what Quick Look, Spotlight's importers and
// App Sandbox's services are handed a file by, never makes a process outside a desk's sandbox look like one of the
// desk's, and the reap never signals it. A desk can't issue one; a read extension for its profile, or for its own
// folder (which holds its mark), leaves another sandbox "not in"; and one handed to a desk's own process opens nothing.
// The stand-ins are this test's python processes: one outside every desk, which sandboxes itself as App Sandbox does
// (the support folder denied, reads by extension allowed), and desk processes under the desk's real profile.

// extensionPython is the stand-ins' ctypes preamble: libSystem's sandbox calls.
const extensionPython = `import ctypes, os, sys, time
lib = ctypes.CDLL("/usr/lib/libSystem.B.dylib")
lib.sandbox_extension_issue_file.restype = ctypes.c_char_p
lib.sandbox_extension_issue_file.argtypes = [ctypes.c_char_p, ctypes.c_char_p, ctypes.c_uint32]
lib.sandbox_extension_consume.restype = ctypes.c_int64
lib.sandbox_extension_consume.argtypes = [ctypes.c_char_p]
lib.sandbox_init.argtypes = [ctypes.c_char_p, ctypes.c_uint64, ctypes.POINTER(ctypes.c_char_p)]
def issue(path):
    token = lib.sandbox_extension_issue_file(b"com.apple.app-sandbox.read", path.encode(), 0)
    return token.decode() if token else ""
def reads(path):
    try:
        try:
            os.listdir(path)
        except NotADirectoryError:
            open(path, "rb").close()
        return "read ok"
    except OSError:
        return "read refused"
`

// extensionIssue prints a read extension for argv[1], "" when it's refused.
const extensionIssue = extensionPython + `sys.stdout.write(issue(sys.argv[1]))
`

// extensionKinds prints which of the other kinds of extension it could issue (FXFIREREAPR6): a generic read, a Mach
// service's and an IOKit class's, each "kind:true" or "kind:false".
const extensionKinds = `import ctypes
lib = ctypes.CDLL("/usr/lib/libSystem.B.dylib")
def kind(name, *args):
    f = getattr(lib, name)
    f.restype = ctypes.c_char_p
    f.argtypes = [ctypes.c_char_p] * len(args) + [ctypes.c_uint32]
    return bool(f(*[a.encode() for a in args], 0))
print("generic:%s mach:%s iokit:%s" % (
    kind("sandbox_extension_issue_generic", "com.apple.app-sandbox.read"),
    kind("sandbox_extension_issue_mach", "com.apple.app-sandbox.mach", "com.apple.system.logger"),
    kind("sandbox_extension_issue_iokit_registry_entry_class", "com.apple.app-sandbox.iokit-client", "IOSurfaceRootUserClient")))
`

// extensionHolder takes the read extension argv[3] for the place argv[2] (issued by its caller: a desk, or this test),
// then, unless argv[1] is "", sandboxes itself with that profile first; it writes its pid and whether it reads the place
// in argv[4], and waits to be ended.
const extensionHolder = extensionPython + `profile, place, token, ready = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
if profile:
    err = ctypes.c_char_p()
    if lib.sandbox_init(profile.encode(), 0, ctypes.byref(err)) != 0:
        sys.exit(3)
if lib.sandbox_extension_consume(token.encode()) < 0:
    sys.exit(5)
part = ready[:-3] + "-part.md"
with open(part, "w") as f:
    f.write("%d %s\n" % (os.getpid(), reads(place)))
os.rename(part, ready)
time.sleep(60)
`

// extensionIssued is a read extension for place, issued by python, under the desk's profile when profile isn't "".
func extensionIssued(t *testing.T, python, profile, place string) string {
	t.Helper()
	args := []string{python, "-I", "-c", extensionIssue, place}
	if profile != "" {
		args = append([]string{"/usr/bin/sandbox-exec", "-f", profile}, args...)
	}
	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		t.Fatalf("the issue of a read extension for %s didn't run: %v", place, err)
	}
	return string(out)
}

// extensionHolderStarted starts a holder of token for place, and waits for what it says: its pid, and whether it reads
// place. It runs in the desk's sandbox when desk (its profile) isn't "", else outside every desk, in a sandbox of its
// own (sandbox, a profile). It's registered for the reap, and ended at the test's end.
func extensionHolderStarted(t *testing.T, python, desk, sandbox, place, token string) (pid int, said string, ended <-chan error) {
	t.Helper()
	ready := filepath.Join(t.TempDir(), "ready.md")
	args := []string{python, "-I", "-c", extensionHolder, sandbox, place, token, ready}
	if desk != "" {
		ready = filepath.Join(filepath.Dir(desk), "holder.md") // in its own folder: a desk writes its .md files there
		args = []string{"/usr/bin/sandbox-exec", "-f", desk, python, "-I", "-c", extensionHolder, "", place, token, ready}
		_ = os.Remove(ready)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done, waited := make(chan error, 1), make(chan struct{})
	go func() { done <- cmd.Wait(); close(waited) }()
	t.Cleanup(func() {
		cmd.Process.Kill() // this test's child, its end not read yet: its pid is its own
		<-waited
	})
	for range 500 {
		if text, err := os.ReadFile(ready); err == nil {
			fields := strings.SplitN(strings.TrimSpace(string(text)), " ", 2)
			if len(fields) == 2 && fields[0] == strconv.Itoa(cmd.Process.Pid) {
				reapLooksAt(t, cmd.Process.Pid)
				return cmd.Process.Pid, fields[1], done
			}
			t.Fatalf("the holder said %q, not its pid %d", text, cmd.Process.Pid)
		}
		select {
		case err := <-done:
			t.Fatalf("the holder ended before it said anything: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the holder said nothing")
	return 0, "", nil
}

func TestASandboxExtensionNeverMakesAnotherProcessLookLikeTheDesks(t *testing.T) {
	needSandboxExec(t)
	python := deskPython(t)
	if python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal("no python3 to stand in with")
		}
		t.Skip("no python3 here to stand in with")
	}
	shortReap(t)
	s := standInDesk(t)
	socket := deskSocketPath(s.at.ID, 0)
	profile := physical(deskProfileFor(t, s, socket))
	// The desk's own folder. Not named own: the browser guard reads a name as the package's constant of that name, and
	// one test's own is a path in Smolcorp.app, so every start handed it would read as a start in a .app.
	deskFolder := filepath.Dir(profile)
	support := physical(supportDir)
	h := Hire{ID: s.at.ID, Name: "Janitor", TmuxSocket: socket}
	// The outside stand-in's own sandbox, as App Sandbox's: Smolcorp's support folder denied, reads by extension
	// allowed.
	appSandbox := `(version 1)(allow default)(deny file-read* file-write* (subpath ` + strconv.Quote(support) + `))` +
		`(allow file-read* (extension "com.apple.app-sandbox.read"))`
	// in is whether a look (only a look: nothing is signalled) finds pid in the desk's sandbox.
	in := func(pid int) bool {
		left, _ := reapSandboxAt(deskFolder, false)
		return slices.Contains(left, pid)
	}
	// leftAlone is whether the desk's stop (a reap that ends what it finds) leaves the process pid, ended by ended,
	// running.
	leftAlone := func(t *testing.T, pid int, ended <-chan error) {
		t.Helper()
		if why := deskReap(h); why != "" {
			t.Errorf("the desk's stop said %q, want none of its processes left", why)
		}
		select {
		case err := <-ended:
			t.Errorf("FINDING: the desk's stop ended process %d, outside its sandbox: %v", pid, err)
		case <-time.After(200 * time.Millisecond):
		}
	}

	t.Run("a desk can't issue one", func(t *testing.T) {
		places := []string{profile, deskFolder, support}
		if marks, err := os.ReadDir(filepath.Join(deskFolder, "reap.noindex")); err == nil && len(marks) == 1 {
			places = append(places, filepath.Join(deskFolder, "reap.noindex", marks[0].Name()))
		} else {
			t.Errorf("the desk has no mark: %v", err)
		}
		for _, place := range places {
			token := extensionIssued(t, python, profile, place)
			t.Logf("a desk process issued a read extension for %s: %t", strings.TrimPrefix(place, filepath.Dir(deskFolder)+"/"), token != "")
			if token != "" {
				t.Errorf("FINDING: a desk process issued a read extension for %s", place)
			}
		}
		if extensionIssued(t, python, "", profile) == "" {
			t.Error("the control: this test's own python couldn't issue one either")
		}
	})

	// Nor one of another kind, which reads no file of the mark's but would hand another process a right of the desk's.
	t.Run("nor one of another kind", func(t *testing.T) {
		kinds := func(profile string) string {
			args := []string{python, "-I", "-c", extensionKinds}
			if profile != "" {
				args = append([]string{"/usr/bin/sandbox-exec", "-f", profile}, args...)
			}
			out, err := exec.Command(args[0], args[1:]...).Output()
			if err != nil {
				t.Fatalf("the issue of the other kinds didn't run: %v", err)
			}
			return strings.TrimSpace(string(out))
		}
		if got := kinds(profile); got != "generic:False mach:False iokit:False" {
			t.Errorf("FINDING: a desk process issued an extension: %s", got)
		}
		if got := kinds(""); got != "generic:True mach:True iokit:True" {
			t.Errorf("the control: this test's own python issued %s, want every kind", got)
		}
	})

	for _, c := range []struct{ name, place string }{
		{"one for its profile, issued outside every desk", profile},
		{"one for its own folder, which holds its mark, issued outside every desk", deskFolder},
	} {
		t.Run(c.name, func(t *testing.T) {
			pid, said, ended := extensionHolderStarted(t, python, "", appSandbox, c.place, extensionIssued(t, python, "", c.place))
			t.Logf("the stand-in outside the desk's sandbox, with the extension: %s; a look finds it in: %t", said, in(pid))
			if said != "read ok" {
				t.Fatalf("the stand-in %s, want the extension to let it read %s", said, c.place)
			}
			if in(pid) {
				t.Errorf("FINDING: a look finds process %d, outside the desk's sandbox, in it", pid)
			}
			leftAlone(t, pid, ended)
		})
	}

	// The control, and the one way left: an extension for the mark alone. Only a process outside every desk can issue
	// it (none of the owner's apps is ever handed that file), and it's looked at only here: nothing is signalled.
	t.Run("one for exactly its mark, issued outside every desk", func(t *testing.T) {
		marks, err := os.ReadDir(filepath.Join(deskFolder, "reap.noindex"))
		if err != nil || len(marks) != 1 {
			t.Fatalf("the desk has no mark: %v", err)
		}
		mark := filepath.Join(deskFolder, "reap.noindex", marks[0].Name())
		pid, said, _ := extensionHolderStarted(t, python, "", appSandbox, mark, extensionIssued(t, python, "", mark))
		t.Logf("the stand-in outside the desk's sandbox, with the extension: %s; a look finds it in: %t", said, in(pid))
		if said != "read ok" || !in(pid) {
			t.Errorf("the stand-in %s; a look finds it in the desk's sandbox: %t, want both", said, in(pid))
		}
	})

	// The other way round: a desk's own process handed a read extension for the support folder still can't read it,
	// and is still found in its sandbox, and ended by its stop.
	t.Run("one for the support folder, handed to a desk's process", func(t *testing.T) {
		pid, said, ended := extensionHolderStarted(t, python, profile, "", support, extensionIssued(t, python, "", support))
		t.Logf("the desk's process, with the extension: %s; a look finds it in: %t", said, in(pid))
		if said != "read refused" {
			t.Errorf("the desk's process %s the support folder with an extension for it, want it refused", said)
		}
		if !in(pid) {
			t.Errorf("a look doesn't find the desk's process %d in its sandbox", pid)
		}
		if why := deskReap(h); why != "" {
			t.Errorf("the desk's stop said %q, want it to end the desk's process", why)
		}
		select {
		case <-ended:
		case <-time.After(2 * time.Second):
			t.Errorf("the desk's process %d still runs after its stop", pid)
		}
	})
}

// ptyHolder opens a pty as a desk's tmux does (openpty: posix_openpt and grantpt, so the kernel hands it macOS's key
// to that terminal), opens its slave again by its name, and tries to read each place in argv[2:]; it writes its pid and
// what it got in argv[1], and waits to be ended.
const ptyHolder = extensionPython + `ready = sys.argv[1]
master, slave = os.openpty()
try:
    os.close(os.open(os.ttyname(slave), os.O_RDWR | os.O_NOCTTY))
    said = ["pty ok"]
except OSError:
    said = ["pty refused"]
said += [reads(place) for place in sys.argv[2:]]
part = ready[:-3] + "-part.md"
with open(part, "w") as f:
    f.write("%d %s\n" % (os.getpid(), ",".join(said)))
os.rename(part, ready)
time.sleep(60)
`

// FXSTACKG's row, from the ruling that kept FXPTYOWN's terminals beside FXFIREREAPR5's deny of every extension's
// issue: macOS's pty key, the one thing a desk's profile allows by extension, opens the terminal the desk made and
// nothing more. A desk's process holding one can't read the support folder or another desk's mark, and a look for that
// other desk doesn't find it in its sandbox; a look for its own desk does (the control). Stand-ins only: python under
// each desk's real profile, a pty of its own, nothing signalled.
func TestADesksPtyKeyOpensItsTerminalAloneAndNoOtherDesksMark(t *testing.T) {
	needSandboxExec(t)
	python := deskPython(t)
	if python == "" {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatal("no python3 to stand in with")
		}
		t.Skip("no python3 here to stand in with")
	}
	shortReap(t)
	s := standInDesk(t)
	profile := physical(deskProfileFor(t, s, deskSocketPath(s.at.ID, 0)))
	deskFolder := filepath.Dir(profile)
	if err := os.MkdirAll(hireFolder(supportDir, "zz99"), 0o700); err != nil {
		t.Fatal(err)
	}
	other, err := makeDeskSandbox(deskAt{ID: "zz99", Root: t.TempDir(), Socket: deskSocketPath("zz99", 0), Git: runGit})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { keptSandboxes.Lock(); delete(keptSandboxes.of, "zz99"); keptSandboxes.Unlock() })
	otherFolder := filepath.Dir(physical(deskProfileFor(t, other, deskSocketPath("zz99", 0))))
	marks, err := os.ReadDir(filepath.Join(otherFolder, "reap.noindex"))
	if err != nil || len(marks) != 1 {
		t.Fatalf("the other desk has no mark: %v", err)
	}
	otherMark := filepath.Join(otherFolder, "reap.noindex", marks[0].Name())
	support := physical(supportDir)
	// The control: both places are there, and this test reads them.
	if _, err := os.ReadDir(support); err != nil {
		t.Fatalf("the control: this test can't read the support folder: %v", err)
	}
	if _, err := os.ReadFile(otherMark); err != nil {
		t.Fatalf("the control: this test can't read the other desk's mark: %v", err)
	}

	ready := filepath.Join(deskFolder, "pty.md") // in its own folder: a desk writes its .md files there
	_ = os.Remove(ready)
	cmd := exec.Command("/usr/bin/sandbox-exec", "-f", profile, python, "-I", "-c", ptyHolder, ready, support, otherMark)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done, waited := make(chan error, 1), make(chan struct{})
	go func() { done <- cmd.Wait(); close(waited) }()
	t.Cleanup(func() {
		cmd.Process.Kill() // this test's child, its end not read yet: its pid is its own
		<-waited
	})
	pid, said := cmd.Process.Pid, ""
	for range 500 {
		if text, err := os.ReadFile(ready); err == nil {
			fields := strings.SplitN(strings.TrimSpace(string(text)), " ", 2)
			if len(fields) != 2 || fields[0] != strconv.Itoa(pid) {
				t.Fatalf("the desk's process said %q, not its pid %d", text, pid)
			}
			said = fields[1]
			break
		}
		select {
		case err := <-done:
			t.Fatalf("the desk's process ended before it said anything: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if said == "" {
		t.Fatal("the desk's process said nothing")
	}
	reapLooksAt(t, pid)
	lookIn := func(folder string) bool {
		left, _ := reapSandboxAt(folder, false)
		return slices.Contains(left, pid)
	}
	t.Logf("the desk's process, holding its pty's key: %s (its pty, the support folder, the other desk's mark); a look finds it in its own desk: %t, in the other: %t",
		said, lookIn(deskFolder), lookIn(otherFolder))
	if pty, _, _ := strings.Cut(said, ","); pty != "pty ok" {
		t.Fatalf("the control: the desk's process got %q on the pty it made, want it to hold the key and open it", pty)
	}
	if said != "pty ok,read refused,read refused" {
		t.Errorf("FINDING: the desk's process holding a pty key got %q, want the support folder and the other desk's mark refused", said)
	}
	if lookIn(otherFolder) {
		t.Errorf("FINDING: a look for the other desk finds process %d, holding a pty key in desk %s's sandbox, in it", pid, s.at.ID)
	}
	if !lookIn(deskFolder) {
		t.Errorf("the control: a look for its own desk doesn't find process %d in it", pid)
	}
}
