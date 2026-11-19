package sandbox

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-M5, the Mac escape table II and normal work (PL126A §2.2; SANDBOX-PLAN §A3.1, §A4.3, A0.1 C1-C3, N6), on this
// Mac with stand-ins only (owner Q5 (a)), on M4's stand-in desk (escape_darwin_test.go): the real
// /usr/bin/sandbox-exec, the real profile PrepareProfile makes from the desk's Policy, with the lists' Mach services,
// IOKit classes and denied ports. Every program here runs in the test's own temp folders, with a stand-in home; no
// real agent starts, nothing of the owner's ~/.claude is read, no key is real (FAKEKEY), and nothing reaches the
// network: the installs are offline, from folders and a wheel the test makes.
//
// The escape rows (RSBXPLANc-verdict U2-U6, A0.1 C1-C3):
//
//	U2       a hire signals the core and the owner's programs: SIGCONT to the core (this test), SIGTERM to a `sleep`
//	         outside: EPERM, and the `sleep` lives (TestOnAMacAHireCantOpenTheInspectorOfANodeOutside)
//	U3       SIGUSR1 to a Node outside started with --inspect-port=127.0.0.1:0: EPERM, and it opens no debugger; the
//	         same signal sent from outside opens one (the control)
//	U4       the hire's own Node child still gets SIGUSR1 (its debugger opens) and SIGTERM
//	U5       /bin/ps and /usr/bin/top are setuid: the sandbox refuses to start them (TestOnAMacPsAndTopDontStartInADesk);
//	         Claude Code's own `/bin/ps -o lstart= -p <pid>` is the one tried
//	C3       a planted library and a planted module, neither run (TestOnAMacAPlantedLibraryOrModuleNeverRuns): a real
//	         program loads its library through an @rpath folder before its own, and a real npm CLI its module; the
//	         plants are refused, and the programs, run outside afterwards, load what they loaded before
//	C2, U6   other items `security` made: a hire reads them as it reads its account's sign-in
//	         (TestOnAMacAHireReadsTheItemsSecurityMadeAsTheOwnerDoes). It makes a keychain, so it runs only where
//	         SMOLCORP_TEST_KEYCHAIN=1 (CI's Mac sandbox job), never on a Mac someone uses
//	TCC      the runner only (PL126A §4): a stand-in that holds Accessibility can't query, tap or open the HID system
//	         in a desk, and a copy of an app a hire made can't be opened nor use the grant
//	         (TestOnAMacAStandInWithAccessibilityCantUseItInADesk, testdata/tccstandin.c)
//
// The normal-work list (§A4.3 "Still works", PL126A SA-M5) runs in a desk's sandbox, Remote Login off (AQ12), the SSH
// agent on its list (TestOnAMacNormalWorkRunsInADesksSandbox): a commit signed through the agent and a push; npm, pnpm
// and bun installs; go test; cargo build; pip in a venv; swift build (and swift test, where allowed) with
// --disable-sandbox; headless Chrome with --no-sandbox; and Claude Code's own folder for a desk
// (TestOnAMacClaudeCodesDeskFolderWorksAndItsMarketplaceRefreshIsRefused). A tool that isn't on this Mac skips its
// row with the reason; SMOLCORP_SANDBOX_REQUIRED=1 makes that a failure. The same list runs again with no IOKit class
// at all (TestOnAMacNormalWorkNeedsNoIOKitClass): what of the IOKit list normal work needs, if anything.

const standInSignals = "signals"

func init() { moreStandIns[standInSignals] = signalsStandIn }

// signalWord is a signal the stand-in may send, by its short name.
var signalWord = map[string]syscall.Signal{"CONT": syscall.SIGCONT, "TERM": syscall.SIGTERM, "USR1": syscall.SIGUSR1}

// signalsStandIn sends each signal of its one argument, [[pid, "USR1"], …], and prints what each gave, as the escape
// stand-in does.
func signalsStandIn(args []string) int {
	var sends [][2]string
	if len(args) != 1 || json.Unmarshal([]byte(args[0]), &sends) != nil {
		fmt.Fprintln(os.Stderr, "signals: one argument, the signals as JSON")
		return 2
	}
	said := make([]string, len(sends))
	for i, s := range sends {
		pid, err := strconv.Atoi(s[0])
		sig, known := signalWord[s[1]]
		if err != nil || !known {
			said[i] = "not a signal"
			continue
		}
		said[i] = errnoName(unix.Kill(pid, sig))
	}
	if err := json.NewEncoder(os.Stdout).Encode(said); err != nil {
		return 3
	}
	return 0
}

// required fails the test where SMOLCORP_SANDBOX_REQUIRED=1, and skips it elsewhere, with why.
func required(t *testing.T, why string, args ...any) {
	t.Helper()
	if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
		t.Fatalf("SMOLCORP_SANDBOX_REQUIRED=1 and "+why, args...)
	}
	t.Skipf("SKIPPED, "+why, args...)
}

// tool is a program to run here, whole, or the test skips (or fails, where required). /usr/bin's developer tools
// (cc, swift, python3, git) are Apple's shims, which with no developer tools ask to install them (a dialog): they are
// used only when xcode-select names a developer folder.
func tool(t *testing.T, name string) string {
	t.Helper()
	found, err := exec.LookPath(name)
	if err == nil && strings.HasPrefix(found, "/usr/bin/") {
		if e := disclaim.Cmd(exec.Command("/usr/bin/xcode-select", "-p")).Run(); e != nil {
			err = fmt.Errorf("no developer tools (%v): %s would ask to install them", e, found)
		}
	}
	if err == nil {
		found, err = filepath.Abs(found)
	}
	if err != nil {
		required(t, "no %s here: %v", name, err)
	}
	return found
}

// pathOf is a PATH of the folders the programs are in, then the system's.
func pathOf(programs ...string) string {
	var folders []string
	for _, p := range programs {
		if dir := filepath.Dir(p); !slices.Contains(folders, dir) {
			folders = append(folders, dir)
		}
	}
	return "PATH=" + strings.Join(append(folders, "/usr/bin", "/bin", "/usr/sbin", "/sbin"), ":")
}

// prepareWith writes the desk's real profile again, as prepare does, with agents as the SSH agent's sockets: allowed
// while Remote Login doesn't answer (AQ12), as on the owner's Mac with it off. iokit is the IOKit list: IOKitClasses()
// but where a row asks what normal work needs of it (TestOnAMacNormalWorkNeedsNoIOKitClass).
func (d *escapeDesk) prepareWith(t *testing.T, agents []string, iokit []IOKitClass) {
	t.Helper()
	p := mustNew(t, d.pl)
	svc := Services{OwnSocket: d.ownSocket, DeniedPorts: DeniedPorts(false), Agents: agents,
		Sockets: SocketPaths(Sockets("darwin"), d.home, func(string) string { return "" }, false, false)}
	for _, m := range MachServices() {
		svc.Mach = append(svc.Mach, m.Name)
	}
	for _, c := range iokit {
		svc.IOKit = append(svc.IOKit, IOKitOpen{Class: c.Class, UserClient: c.UserClient})
	}
	if err := PrepareProfile(p, d.profile, svc); err != nil {
		t.Fatal(err)
	}
}

// appended is a copy of the desk's profile, named name, with rule after the rest: the last rule that matches wins, so
// rule alone is what changes.
func (d *escapeDesk) appended(t *testing.T, name, rule string) string {
	t.Helper()
	data, err := os.ReadFile(d.profile)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(d.base, name)
	if err := os.WriteFile(file, append(data, "\n"+rule+"\n"...), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

// work runs program in the desk's sandbox (under profile, d.profile when ""), in dir, with HOME and env, for at most
// limit. startRefusal comes first, as in inside (escape3_darwin_test.go), told the start is in a desk. With d.noTerminal,
// program starts with no controlling terminal.
func (d *escapeDesk) work(t *testing.T, profile, dir string, limit time.Duration, env []string, program string, args ...string) ran {
	t.Helper()
	if err := startRefusal(program, args, true); err != nil {
		t.Fatal(err)
	}
	sandboxExec := sandboxExecOrSkip(t)
	if profile == "" {
		profile = d.profile
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, sandboxExec, append([]string{"-f", profile, program}, args...)...))
	cmd.Dir, cmd.WaitDelay = dir, 5*time.Second
	if d.noTerminal {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	cmd.Env = append([]string{"HOME=" + d.home, "LANG=C", "TERM=xterm", "TMPDIR=" + filepath.Join(d.base, "t") + "/"}, env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := ran{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("%s %q didn't end within %s (%s)", program, args, limit, tail(r.stderr))
	case errors.As(err, &exitErr):
		r.status = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("%s %q: %v", program, args, err)
	}
	return r
}

// outside runs program as the owner would, outside any sandbox, and fails the test if it fails. startRefusal comes
// first, told the start isn't in a desk.
func outside(t *testing.T, dir string, env []string, program string, args ...string) string {
	t.Helper()
	if err := startRefusal(program, args, false); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, program, args...))
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, env, 5*time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("outside: %s %q: %v (%s)", program, args, err, tail(string(out)))
	}
	return string(out)
}

// tail is the end of a program's words, for a failure's message.
func tail(s string) string {
	if len(s) > 1500 {
		return "…" + s[len(s)-1500:]
	}
	return s
}

// worked fails the test, with what was said, unless r ended well.
func worked(t *testing.T, what string, r ran) {
	t.Helper()
	if r.status != 0 {
		t.Errorf("%s in the desk: status %d\n%s\n%s", what, r.status, tail(r.stdout), tail(r.stderr))
	}
}

// startOutside starts a program of the owner's outside any sandbox, its words in files, and stops it at the end.
// startRefusal comes first, told the start isn't in a desk.
func startOutside(t *testing.T, base string, env []string, program string, args ...string) (cmd *exec.Cmd, stdout, stderr string) {
	t.Helper()
	if err := startRefusal(program, args, false); err != nil {
		t.Fatal(err)
	}
	o, err := os.CreateTemp(base, "out")
	if err != nil {
		t.Fatal(err)
	}
	e, err := os.CreateTemp(base, "err")
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr = o.Name(), e.Name()
	cmd = disclaim.Cmd(exec.Command(program, args...))
	cmd.Env, cmd.Stdout, cmd.Stderr = env, o, e
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	o.Close()
	e.Close()
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	return cmd, stdout, stderr
}

// waitFor waits up to limit for file to hold text.
func waitFor(file, text string, limit time.Duration) bool {
	for end := time.Now().Add(limit); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if data, _ := os.ReadFile(file); strings.Contains(string(data), text) {
			return true
		}
	}
	return false
}

// alive is whether cmd's process still runs.
func alive(cmd *exec.Cmd) bool {
	return cmd.ProcessState == nil && unix.Kill(cmd.Process.Pid, 0) == nil
}

const debuggerOpened = "Debugger listening"

// U2-U4 (A0.1 C1): `(deny signal)` with `(allow signal (target same-sandbox))`. A Node outside, started with
// --inspect-port=127.0.0.1:0 as an owner's dev server or Node CLI might be, opens its debugger on SIGUSR1; from a desk
// that signal is refused, so nothing of the owner's runs a hire's code. The core (this test) and a program of the
// owner's can't be stopped or killed either. The hire's own Node child still gets both signals.
func TestOnAMacAHireCantOpenTheInspectorOfANodeOutside(t *testing.T) {
	sandboxExecOrSkip(t)
	node := tool(t, "node")
	d := newEscapeDesk(t)
	env := []string{"HOME=" + d.home, pathOf(node)}
	nodeOut, nodeSaid, nodeErr := startOutside(t, d.base, env, node, "--inspect-port=127.0.0.1:0", "-e", `console.log("ready"); setInterval(() => {}, 1000)`)
	sleeper, _, _ := startOutside(t, d.base, env, "/bin/sleep", "60")
	// It must run before it's signalled: before Node sets its handler, SIGUSR1 would end it.
	if !waitFor(nodeSaid, "ready", 15*time.Second) {
		t.Fatal("the Node outside didn't start")
	}

	data, _ := json.Marshal([][2]string{
		{strconv.Itoa(nodeOut.Process.Pid), "USR1"},
		{strconv.Itoa(sleeper.Process.Pid), "TERM"},
		{strconv.Itoa(os.Getpid()), "CONT"},
	})
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := d.inside(t, nil, self, standInSignals, string(data))
	var said []string
	if r.status != 0 || json.Unmarshal([]byte(r.stdout), &said) != nil || len(said) != 3 {
		t.Fatalf("the stand-in hire didn't run: %+v", r)
	}
	for i, row := range []string{"U3: SIGUSR1 to a Node outside", "U2: SIGTERM to a program of the owner's", "U2: SIGCONT to the core"} {
		if said[i] != "EPERM" {
			t.Errorf("%s gave %s, not EPERM", row, said[i])
		}
	}
	time.Sleep(300 * time.Millisecond)
	if data, _ := os.ReadFile(nodeErr); strings.Contains(string(data), debuggerOpened) {
		t.Errorf("U3: the Node outside opened its debugger: %q", data)
	}
	if !alive(sleeper) || !alive(nodeOut) {
		t.Error("U2: a program of the owner's was ended from the desk")
	}
	// The control: the same signal from outside opens the debugger, so the row above isn't empty.
	if err := nodeOut.Process.Signal(syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	if !waitFor(nodeErr, debuggerOpened, 10*time.Second) {
		t.Error("control: SIGUSR1 from outside opened no debugger: the U3 row proves nothing")
	}

	// U4: the hire's own child gets SIGUSR1 (its debugger opens, on 127.0.0.1) and SIGTERM.
	const ownChild = `const {spawn} = require("child_process");
const c = spawn(process.execPath, ["--inspect-port=127.0.0.1:0", "-e", 'console.log("ready"); setInterval(() => {}, 1000)'], {stdio: ["ignore", "pipe", "pipe"]});
let out = "", err = "", sent = false, termed = false;
const done = (r) => { console.log(JSON.stringify(r)); process.exit(0); };
setTimeout(() => { c.kill("SIGKILL"); done({inspector: false, ended: "late"}); }, 15000);
c.stdout.on("data", (b) => { out += b; if (out.includes("ready") && !sent) { sent = true; process.kill(c.pid, "SIGUSR1"); } });
c.stderr.on("data", (b) => { err += b; if (err.includes("` + debuggerOpened + `") && !termed) { termed = true; c.kill("SIGTERM"); } });
c.on("exit", (code, sig) => done({inspector: err.includes("` + debuggerOpened + `"), ended: sig}));`
	r = d.work(t, "", d.root, 30*time.Second, []string{pathOf(node)}, node, "-e", ownChild)
	var child struct {
		Inspector bool
		Ended     string
	}
	if r.status != 0 || json.Unmarshal([]byte(r.stdout), &child) != nil || !child.Inspector || child.Ended != "SIGTERM" {
		t.Errorf("U4: the hire's own child: %+v (%+v)", child, r)
	}
}

// U5 (N6): /bin/ps and /usr/bin/top are setuid root, and the sandbox refuses to start a setuid program. Claude Code
// runs `/bin/ps -o lstart= -p <pid>` itself: in a desk it fails, at once, with the system's words (a cost, §A4.3).
func TestOnAMacPsAndTopDontStartInADesk(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	pid := strconv.Itoa(os.Getpid())
	if got := outside(t, d.base, nil, "/bin/ps", "-o", "lstart=", "-p", pid); strings.TrimSpace(got) == "" {
		t.Fatalf("control: /bin/ps outside said nothing")
	}
	for _, words := range [][]string{{"/bin/ps", "-o", "lstart=", "-p", pid}, {"/usr/bin/top", "-l", "1", "-n", "0"}} {
		if info, err := os.Stat(words[0]); err != nil || info.Mode()&os.ModeSetuid == 0 {
			t.Logf("U5: %s isn't setuid here (%v): the row tries it all the same", words[0], err)
		}
		r := d.work(t, "", d.root, 20*time.Second, []string{pathOf()}, words[0], words[1:]...)
		if r.status == 0 || !strings.Contains(r.stderr, "Operation not permitted") || strings.TrimSpace(r.stdout) != "" {
			t.Errorf("U5: %q in a desk: %+v", words, r)
		}
	}
}

// C3: a program Smolcorp runs loads a library through an @rpath folder that comes before the library's own (a folder
// in the home, which a desk otherwise writes), and an npm CLI loads its module from its own package. X2 denies both
// places; here the plants are tried in the real profile, and the real programs run outside afterwards, as the core
// would run them, loading what they loaded before. A library or module of the hire's own, beside, is made.
func TestOnAMacAPlantedLibraryOrModuleNeverRuns(t *testing.T) {
	sandboxExecOrSkip(t)
	cc, node := tool(t, "cc"), tool(t, "node")
	d := newEscapeDesk(t)
	at := func(rel string) string { return filepath.Join(d.base, filepath.FromSlash(rel)) }
	keg, early := at("brew/Cellar/x/1.0"), filepath.Join(d.home, "plugins")
	for _, dir := range []string{keg + "/lib", keg + "/bin", early, at("brew/bin")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, at("src/lib.c"), "const char *who(void) { return \"the real library\"; }\n")
	mustWrite(t, at("src/main.c"), "#include <stdio.h>\nconst char *who(void);\nint main(void) { puts(who()); return 0; }\n")
	ccEnv := []string{"HOME=" + d.home, pathOf(cc)}
	outside(t, d.base, ccEnv, cc, "-dynamiclib", "-install_name", "@rpath/libx.dylib", "-o", keg+"/lib/libx.dylib", at("src/lib.c"))
	outside(t, d.base, ccEnv, cc, "-o", keg+"/bin/x", at("src/main.c"), "-L"+keg+"/lib", "-lx", "-Wl,-rpath,"+early, "-Wl,-rpath,@loader_path/../lib")
	if err := os.Symlink("../Cellar/x/1.0/bin/x", at("brew/bin/x")); err != nil {
		t.Fatal(err)
	}
	npm := at("brew/lib/node_modules")
	pkg := filepath.Join(npm, "@standin", "cli")
	mustWrite(t, pkg+"/package.json", `{"name":"@standin/cli","version":"1.0.0","bin":{"cli":"bin/cli.js"}}`)
	mustWrite(t, pkg+"/bin/cli.js", "#!/usr/bin/env node\nconsole.log(require(\"dep\"))\n")
	mustWrite(t, pkg+"/node_modules/dep/index.js", "module.exports = \"the real module\"\n")
	if err := os.Chmod(pkg+"/bin/cli.js", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../lib/node_modules/@standin/cli/bin/cli.js", at("brew/bin/cli")); err != nil {
		t.Fatal(err)
	}

	onlyPrograms(t, ProgramName{Name: "x", Why: "a stand-in"}, ProgramName{Name: "cli", Why: "a stand-in"})
	ps := mustResolve(t, Finding{GOOS: "darwin", Home: d.home, Search: []string{at("brew/bin"), filepath.Dir(node)},
		DeveloperDir: at("no-developer-tools")})
	d.pl.InstallRoots = ps.InstallRoots()
	d.prepare(t)
	run := func() (string, string) {
		env := []string{"HOME=" + d.home, pathOf(node)}
		return strings.TrimSpace(outside(t, d.base, env, at("brew/bin/x"))), strings.TrimSpace(outside(t, d.base, env, at("brew/bin/cli")))
	}
	if lib, module := run(); lib != "the real library" || module != "the real module" {
		t.Fatalf("control: before the desk, the programs say %q and %q", lib, module)
	}

	d.check(t, []escapeRow{
		{"C3: a library where @rpath looks first", attempt{"create", filepath.Join(early, "libx.dylib"), ""}, "EPERM"},
		{"C3: its own library made, then moved there", attempt{"create", filepath.Join(d.root, "libx.dylib"), ""}, "ok"},
		{"C3: its own library made, then moved there", attempt{"rename", filepath.Join(d.root, "libx.dylib"), filepath.Join(early, "libx.dylib")}, "EPERM"},
		{"C3: a link where @rpath looks first", attempt{"symlink", filepath.Join(early, "libx.dylib"), filepath.Join(d.root, "libx.dylib")}, "EPERM"},
		{"C3: the library replaced in its keg", attempt{"write", keg + "/lib/libx.dylib", ""}, "EPERM"},
		{"C3: the library moved aside in its keg", attempt{"rename", keg + "/lib/libx.dylib", filepath.Join(d.root, "old.dylib")}, "EPERM"},
		{"C3: the module replaced in its package", attempt{"write", pkg + "/node_modules/dep/index.js", ""}, "EPERM"},
		{"C3: a module beside it in its package", attempt{"mkdir", pkg + "/node_modules/dep2", ""}, "EPERM"},
		{"C3: the package's own folder moved aside", attempt{"rename", pkg + "/node_modules", filepath.Join(d.root, "nm")}, "EPERM"},
		// Beside them, a desk still makes its own: a library elsewhere in the same folder, a module of the same name
		// at the npm root, which node takes only after the package's own.
		{"control: a library of its own, beside", attempt{"create", filepath.Join(early, "other.dylib"), ""}, "ok"},
		{"control: a module of the same name, at the npm root", attempt{"mkdir", filepath.Join(npm, "dep"), ""}, "ok"},
		{"control: a module of the same name, at the npm root", attempt{"create", filepath.Join(npm, "dep", "index.js"), ""}, "ok"},
	})
	// The module the hire made at the npm root says who it is, if it ever runs.
	mustWrite(t, filepath.Join(npm, "dep", "index.js"), "module.exports = \"the hire's module\"\n")
	if lib, module := run(); lib != "the real library" || module != "the real module" {
		t.Errorf("C3: after the desk, the programs say %q and %q", lib, module)
	}
	for _, kept := range []string{filepath.Join(early, "libx.dylib"), filepath.Join(pkg, "node_modules", "dep2")} {
		if _, err := os.Lstat(kept); err == nil {
			t.Errorf("C3: %s was made", kept)
		}
	}
}

// C2, U6: Claude Code reads its sign-in with `security find-generic-password … -w`, and an item `security` made trusts
// `security`, so a desk reads it with no prompt, as the owner's other programs do (said plainly in §A4.4). The row
// says the same of any other item `security` made, in a keychain of the test's own, unlocked. It makes a keychain (and
// macOS may add it to the search list), so it runs only where SMOLCORP_TEST_KEYCHAIN=1: CI's Mac sandbox job, on a
// throwaway runner. It never asks for an item made otherwise: that would ask the owner.
func TestOnAMacAHireReadsTheItemsSecurityMadeAsTheOwnerDoes(t *testing.T) {
	sandboxExecOrSkip(t)
	if os.Getenv("SMOLCORP_TEST_KEYCHAIN") != "1" {
		t.Skip("SKIPPED, it makes a keychain: only on a throwaway runner, with SMOLCORP_TEST_KEYCHAIN=1 (CI's Mac sandbox job)")
	}
	d := newEscapeDesk(t)
	keychain := filepath.Join(d.base, "standin.keychain-db")
	security := "/usr/bin/security"
	env := []string{"HOME=" + d.home, pathOf()}
	outside(t, d.base, env, security, "create-keychain", "-p", "standin", keychain)
	t.Cleanup(func() { disclaim.Cmd(exec.Command(security, "delete-keychain", keychain)).Run() })
	outside(t, d.base, env, security, "unlock-keychain", "-p", "standin", keychain)
	outside(t, d.base, env, security, "add-generic-password", "-a", "standin", "-s", "smolcorp-standin", "-w", escapeKey, "-T", security, keychain)
	r := d.work(t, "", d.root, 30*time.Second, []string{pathOf()}, security, "find-generic-password", "-a", "standin", "-s", "smolcorp-standin", "-w", keychain)
	if r.status != 0 || strings.TrimSpace(r.stdout) != escapeKey {
		t.Errorf("C2/U6: a desk reading an item `security` made: %+v (§A4.4 says it can, as today)", r)
	} else {
		t.Log("C2/U6: a desk reads an item `security` made, as the owner's other programs do (§A4.4, said plainly)")
	}
}

// workDesk is M4's stand-in desk as the owner's normal work finds it: Remote Login off (AQ12), a real SSH agent of the
// test's own on its list, holding a key made for the test, and the owner's git config signing commits with it. Its
// profile has iokit as its IOKit list.
type workDesk struct {
	*escapeDesk
	agent, signers string
	iokit          []IOKitClass
}

func newWorkDesk(t *testing.T, iokit []IOKitClass) *workDesk {
	t.Helper()
	sandboxExecOrSkip(t)
	w := &workDesk{escapeDesk: newEscapeDesk(t), iokit: iokit}
	w.pl.RemoteLogin = false
	if err := os.MkdirAll(filepath.Join(w.base, "t"), 0o700); err != nil {
		t.Fatal(err)
	}
	w.prepareWith(t, nil, w.iokit)
	return w
}

// startAgent starts an SSH agent of the test's own, with a key made for the test, and writes the owner's git config to
// sign every commit with it through the agent (gpg.format ssh), as this owner's does through 1Password (S6).
func (w *workDesk) startAgent(t *testing.T) {
	t.Helper()
	keygen, agent, add := tool(t, "ssh-keygen"), tool(t, "ssh-agent"), tool(t, "ssh-add")
	key := filepath.Join(w.base, "sk")
	outside(t, w.base, nil, keygen, "-q", "-t", "ed25519", "-N", "", "-C", "standin@example.invalid", "-f", key)
	w.agent = filepath.Join(w.base, "a.sock")
	a, _, _ := startOutside(t, w.base, []string{"HOME=" + w.home, pathOf()}, agent, "-D", "-a", w.agent)
	for end := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if _, err := os.Stat(w.agent); err == nil {
			break
		} else if time.Now().After(end) || !alive(a) {
			t.Fatalf("the SSH agent didn't start: %v", err)
		}
	}
	outside(t, w.base, []string{"HOME=" + w.home, "SSH_AUTH_SOCK=" + w.agent, pathOf()}, add, "-q", key)
	// Only the agent holds the key now: with the file beside the public one, ssh-keygen would sign from the file,
	// and a commit could be signed with no agent reached.
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	w.signers = filepath.Join(w.base, "allowed_signers")
	mustWrite(t, w.signers, "standin@example.invalid "+string(pub))
	mustWrite(t, filepath.Join(w.home, ".gitconfig"), "[user]\n\tname = Stand In\n\temail = standin@example.invalid\n\tsigningkey = "+key+".pub\n"+
		"[gpg]\n\tformat = ssh\n[commit]\n\tgpgsign = true\n[init]\n\tdefaultBranch = main\n")
	w.prepareWith(t, []string{w.agent}, w.iokit)
}

func TestOnAMacNormalWorkRunsInADesksSandbox(t *testing.T) { normalWork(t, IOKitClasses()) }

// headlessChrome is the Chrome program startRefusal gives its desk form: SMOLCORP_TEST_CHROME, or "" when that's
// unset. It never looks for one: the owner's installed Google Chrome, which it used to fall back to, aborted in a
// desk's profile and put a "quit unexpectedly" dialog on the owner's screen at each run (FXCHROMEPOPUP). Normal work
// runs chrome-headless-shell only (headlessShell). Only the declarations on chromeCallers may call it
// (TestOnAMacNoRowHereOpensAnAppUngated).
func headlessChrome() string {
	return os.Getenv("SMOLCORP_TEST_CHROME")
}

// normalWork runs the normal-work list, each row in a desk whose IOKit list is iokit, and says how each row ended:
// "passed", "failed" or "skipped", by its name.
func normalWork(t *testing.T, iokit []IOKitClass) map[string]string {
	ended := map[string]string{}
	row := func(name string, f func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() {
				ended[name] = map[bool]string{true: "failed", false: "passed"}[t.Failed()]
				if t.Skipped() {
					ended[name] = "skipped"
				}
			})
			f(t)
		})
	}
	row("a commit signed through the agent, and a push", func(t *testing.T) {
		git := gitOrSkip(t)
		w := newWorkDesk(t, iokit)
		w.startAgent(t)
		gitEnv := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + filepath.Join(w.home, ".gitconfig"), "SSH_AUTH_SOCK=" + w.agent, pathOf(git)}
		owner := func(dir string, args ...string) string {
			t.Helper()
			return outside(t, dir, append([]string{"HOME=" + w.home}, gitEnv...), git, args...)
		}
		// The owner's repo, its remote, and the hire's linked worktree with its upstream preset (X7), as the core
		// makes them; then the desk's profile again, for the real repo.
		for _, gone := range []string{w.checkout, w.root, filepath.Join(w.home, "app-hires", "h2")} {
			if err := os.RemoveAll(gone); err != nil {
				t.Fatal(err)
			}
		}
		remote := filepath.Join(w.home, "remote.git")
		owner(w.home, "init", "-q", "--bare", remote)
		if err := os.MkdirAll(w.checkout, 0o700); err != nil {
			t.Fatal(err)
		}
		owner(w.checkout, "init", "-q")
		mustWrite(t, filepath.Join(w.checkout, "README"), "the owner's checkout")
		owner(w.checkout, "add", "README")
		owner(w.checkout, "commit", "-q", "-m", "first")
		owner(w.checkout, "remote", "add", "origin", remote)
		owner(w.checkout, "push", "-q", "origin", "main")
		// Its branch is hire/h1/…, as every real hire's (prepareHire), the only one a desk writes since FXGITROWS2R4
		// (this stand-in moved in FXGITROWS2R6).
		owner(w.checkout, "worktree", "add", "-q", "-b", "hire/h1/work", w.root)
		owner(w.checkout, "worktree", "add", "-q", "-b", "hire/h2/work", filepath.Join(w.home, "app-hires", "h2"))
		owner(w.checkout, "config", "branch.hire/h1/work.remote", "origin")
		owner(w.checkout, "config", "branch.hire/h1/work.merge", "refs/heads/hire/h1/work")
		mustWrite(t, filepath.Join(w.root, ".claude", "settings.json"), "{}")
		w.prepareWith(t, []string{w.agent}, w.iokit)

		hire := func(args ...string) ran {
			t.Helper()
			return w.work(t, "", w.root, time.Minute, gitEnv, git, args...)
		}
		mustWrite(t, filepath.Join(w.root, "work.go"), "package work")
		worked(t, "git add", hire("add", "work.go"))
		worked(t, "a signed git commit", hire("commit", "-q", "-m", "the hire's work"))
		worked(t, "git push, to the preset upstream", hire("push", "-q"))
		worked(t, "git fetch", hire("fetch", "-q"))
		worked(t, "git pull --rebase", hire("pull", "-q", "--rebase"))
		if got := owner(w.checkout, "-c", "gpg.ssh.allowedSignersFile="+w.signers, "log", "-1", "--format=%G? %s", "hire/h1/work"); got != "G the hire's work\n" {
			t.Errorf("the hire's commit, as the owner's git sees it: %q (G is a good signature)", got)
		}
		if got, want := owner(w.home, "--git-dir", remote, "rev-parse", "hire/h1/work"), owner(w.checkout, "rev-parse", "hire/h1/work"); got != want {
			t.Errorf("the remote's h1 is %s, the hire's %s", got, want)
		}
	})

	// An app with one dependency from a folder beside it, installed offline.
	jsApp := func(t *testing.T, w *workDesk, name string) string {
		t.Helper()
		dir := filepath.Join(w.root, name)
		mustWrite(t, filepath.Join(dir, "dep", "package.json"), `{"name":"dep","version":"1.0.0","main":"index.js"}`)
		mustWrite(t, filepath.Join(dir, "dep", "index.js"), `module.exports = "the dependency"`)
		mustWrite(t, filepath.Join(dir, "package.json"), `{"name":"app","version":"1.0.0","private":true,"dependencies":{"dep":"file:./dep"}}`)
		return dir
	}
	installed := func(t *testing.T, w *workDesk, node, dir string) {
		t.Helper()
		r := w.work(t, "", dir, 30*time.Second, []string{pathOf(node)}, node, "-e", `console.log(require("dep"))`)
		if r.status != 0 || strings.TrimSpace(r.stdout) != "the dependency" {
			t.Errorf("the installed dependency: %+v", r)
		}
	}
	row("npm install", func(t *testing.T) {
		node, npm := tool(t, "node"), tool(t, "npm")
		w := newWorkDesk(t, iokit)
		dir := jsApp(t, w, "npm-app")
		worked(t, "npm install", w.work(t, "", dir, 2*time.Minute, []string{pathOf(npm, node)}, npm, "install", "--offline", "--no-audit", "--no-fund",
			"--cache", filepath.Join(w.home, ".npm")))
		installed(t, w, node, dir)
	})
	row("pnpm install", func(t *testing.T) {
		node, pnpm := tool(t, "node"), tool(t, "pnpm")
		w := newWorkDesk(t, iokit)
		dir := jsApp(t, w, "pnpm-app")
		worked(t, "pnpm install", w.work(t, "", dir, 2*time.Minute, []string{pathOf(pnpm, node)}, pnpm, "install", "--offline",
			"--store-dir", filepath.Join(w.home, ".pnpm-store")))
		installed(t, w, node, dir)
	})
	row("bun install", func(t *testing.T) {
		node, bun := tool(t, "node"), tool(t, "bun")
		w := newWorkDesk(t, iokit)
		dir := jsApp(t, w, "bun-app")
		worked(t, "bun install", w.work(t, "", dir, 2*time.Minute, []string{pathOf(bun), "BUN_INSTALL_CACHE_DIR=" + filepath.Join(w.home, ".bun-cache")}, bun, "install"))
		installed(t, w, node, dir)
		// bun install has no offline flag. A folder's dependency needs no network, and this shows it (RT126SAM5's
		// probe): the same install, on the desk's profile with every remote address refused, its registry a closed
		// port and its cache empty.
		dir = jsApp(t, w, "bun-app-no-network")
		worked(t, "bun install, with no network", w.work(t, w.appended(t, "no-network.sb", "(deny network-outbound (remote ip))"), dir, 2*time.Minute,
			[]string{pathOf(bun), "BUN_INSTALL_CACHE_DIR=" + filepath.Join(w.home, ".bun-cache-2"), "BUN_CONFIG_REGISTRY=http://127.0.0.1:9/",
				"NPM_CONFIG_REGISTRY=http://127.0.0.1:9/"}, bun, "install"))
		installed(t, w, node, dir)
	})
	row("go test", func(t *testing.T) {
		w := newWorkDesk(t, iokit)
		goTool := filepath.Join(runtime.GOROOT(), "bin", "go")
		dir := filepath.Join(w.root, "gomod")
		mustWrite(t, filepath.Join(dir, "go.mod"), "module example.invalid/m\n\ngo 1.22\n")
		mustWrite(t, filepath.Join(dir, "m_test.go"), "package m\n\nimport \"testing\"\n\nfunc TestInADesk(t *testing.T) {}\n")
		r := w.work(t, "", dir, 3*time.Minute, []string{pathOf(goTool), "GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=", "GOENV=off",
			"GOCACHE=" + filepath.Join(w.home, "gocache"), "GOPATH=" + filepath.Join(w.home, "go")}, goTool, "test", "-count=1", "./...")
		if worked(t, "go test", r); !strings.Contains(r.stdout, "ok") {
			t.Errorf("go test said %q", r.stdout)
		}
	})
	row("cargo build", func(t *testing.T) {
		cargo := tool(t, "cargo")
		w := newWorkDesk(t, iokit)
		// rustup's cargo finds its toolchains by RUSTUP_HOME, which follows the real home: kept so, read only. Cargo
		// writes in CARGO_HOME (its package cache, its lock), so that one is the stand-in home's, never the real one.
		env := []string{pathOf(cargo), "CARGO_TARGET_DIR=" + filepath.Join(w.root, "target"), "CARGO_HOME=" + filepath.Join(w.home, ".cargo")}
		if v := os.Getenv("RUSTUP_HOME"); v != "" {
			env = append(env, "RUSTUP_HOME="+v)
		} else if real, _ := os.UserHomeDir(); real != "" {
			env = append(env, "RUSTUP_HOME="+filepath.Join(real, ".rustup"))
		}
		dir := filepath.Join(w.root, "crate")
		mustWrite(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"standin\"\nversion = \"0.1.0\"\nedition = \"2021\"\n")
		mustWrite(t, filepath.Join(dir, "src", "main.rs"), "fn main() { println!(\"in a desk\"); }\n")
		worked(t, "cargo build", w.work(t, "", dir, 5*time.Minute, env, cargo, "build", "--offline", "-q"))
	})
	row("pip install in a venv", func(t *testing.T) {
		python := python3(t)
		if python == "" {
			required(t, "no python3 here")
		}
		w := newWorkDesk(t, iokit)
		dir := filepath.Join(w.root, "py")
		wheel := filepath.Join(dir, "standin-0.1-py3-none-any.whl")
		writeWheel(t, wheel)
		env := []string{pathOf(python), "PIP_DISABLE_PIP_VERSION_CHECK=1", "PYTHONDONTWRITEBYTECODE=1"}
		worked(t, "python3 -m venv", w.work(t, "", dir, 2*time.Minute, env, python, "-m", "venv", "venv"))
		worked(t, "pip install", w.work(t, "", dir, 2*time.Minute, env, filepath.Join(dir, "venv", "bin", "pip"), "install", "--no-index", wheel))
		r := w.work(t, "", dir, time.Minute, env, filepath.Join(dir, "venv", "bin", "python"), "-c", "import standin; print(standin.X)")
		if r.status != 0 || strings.TrimSpace(r.stdout) != "in a desk" {
			t.Errorf("the installed package: %+v", r)
		}
	})
	row("swift build and test, with --disable-sandbox", func(t *testing.T) {
		swift := tool(t, "swift")
		w := newWorkDesk(t, iokit)
		dir := filepath.Join(w.root, "pkg")
		mustWrite(t, filepath.Join(dir, "Package.swift"), "// swift-tools-version:5.9\nimport PackageDescription\n"+
			"let package = Package(name: \"M\", targets: [.target(name: \"M\"), .testTarget(name: \"MTests\", dependencies: [\"M\"])])\n")
		mustWrite(t, filepath.Join(dir, "Sources", "M", "m.swift"), "public func inADesk() -> Bool { true }\n")
		mustWrite(t, filepath.Join(dir, "Tests", "MTests", "MTests.swift"), "import XCTest\n@testable import M\n"+
			"final class MTests: XCTestCase { func testInADesk() { XCTAssertTrue(inADesk()) } }\n")
		env := []string{pathOf(swift)}
		worked(t, "swift build --disable-sandbox", w.work(t, "", dir, 5*time.Minute, env, swift, "build", "--disable-sandbox"))
		// No `swift test` on a Mac someone uses until CLIP1 lands (_rules.md): the runner sets SMOLCORP_TEST_SWIFT_TEST.
		if os.Getenv("SMOLCORP_TEST_SWIFT_TEST") != "1" {
			t.Log("swift test: not here (SMOLCORP_TEST_SWIFT_TEST=1 runs it: the runner and CI)")
			return
		}
		worked(t, "swift test --disable-sandbox", w.work(t, "", dir, 10*time.Minute, env, swift, "test", "--disable-sandbox"))
	})
	row("headless Chrome with --no-sandbox", func(t *testing.T) {
		// chrome-headless-shell only, where SMOLCORP_TEST_CHROME names one (headlessShell): with no such flag the row
		// skips, before it looks at any path. Never the owner's installed Chrome (FXCHROMEPOPUP). It runs in a desk as
		// a hire's brief says, with its helpers inside it (headlessshell_darwin_test.go), and as that row runs it: its
		// own process group, a watchdog, and pgrep after (CHROMEKILL1's rules, RT126HEADLESSSHELL1 nit 2).
		chrome := headlessShell(t)
		w := newWorkDesk(t, iokit)
		if r := runHeadlessShell(t, w, w.profile, chrome, "--disable-gpu", "--dump-dom", "data:text/html,<p>in a desk</p>"); r.status != 0 || !strings.Contains(r.stdout, "<p>in a desk</p>") {
			t.Fatalf("chrome-headless-shell --single-process under the desk's profile: status %d (%s)", r.status, tail(r.stderr))
		}
	})
	return ended
}

// ST1S's open row, safe on any Mac: `open -g -j -n` in a desk, on an app path that doesn't exist (_rules.md, "No
// LaunchServices on this Mac"), ends at once, fails, and launches nothing. With nothing at the path no hole can reach an
// app or Gatekeeper, and so this can't tell the sandbox's refusal from the missing app: what LaunchServices does with
// a real copy is TestOnAMacAStandInWithAccessibilityCantUseItInADesk's, where SMOLCORP_STUDIO_ROWS=1.
func TestOnAMacOpenInADeskLaunchesNothing(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	if err := os.MkdirAll(filepath.Join(d.base, "t"), 0o700); err != nil {
		t.Fatal(err)
	}
	missing, marker := filepath.Join(d.root, "Missing.app"), filepath.Join(d.root, "opened.json")
	if _, err := os.Lstat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s must not exist: %v", missing, err)
	}
	r := d.work(t, "", d.root, 30*time.Second, []string{pathOf()}, "/usr/bin/open", "-g", "-j", "-n", missing, "--args", marker)
	if r.status == 0 {
		t.Errorf("open, in a desk, on a missing app: %+v", r)
	}
	t.Logf("open, in a desk, on a missing app: status %d (%s)", r.status, strings.TrimSpace(tail(r.stderr)))
	if waitFor(marker, "{", time.Second) {
		t.Error("something ran, opened by LaunchServices")
	}
	if _, err := os.Lstat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s was made: %v", missing, err)
	}
}

// iokitStandIn opens, then closes, the user client of each IOKit service its arguments name, and says what each gave:
// "ok", "refused 0x…", or "none" where this Mac has no such service. It calls nothing on what it opened.
const iokitStandIn = `#include <IOKit/IOKitLib.h>
#include <stdio.h>
int main(int argc, char **argv) {
	for (int i = 1; i < argc; i++) {
		io_service_t s = IOServiceGetMatchingService(kIOMainPortDefault, IOServiceMatching(argv[i]));
		if (!s) { printf("%s none\n", argv[i]); continue; }
		io_connect_t c = 0;
		kern_return_t kr = IOServiceOpen(s, mach_task_self(), 0, &c);
		if (kr == KERN_SUCCESS) { IOServiceClose(c); printf("%s ok\n", argv[i]); }
		else printf("%s refused 0x%x\n", argv[i], kr);
		IOObjectRelease(s);
	}
	return 0;
}
`

// RT126SAM5 B2: which of the IOKit list's entries normal work needs. The list (lists.go) has three: IOSurface's two,
// named for headless Chrome's and Electron tests' drawing, and RootDomainUserClient, for power assertions. Here the
// whole normal-work list runs again in desks with no IOKit class at all. It passes where no row needs one, and a row
// that does fails here by name, with what it said: on CI's Mac sandbox job that settles cargo, swift test and Chrome
// too. The IOKIT line says which rows passed with none, for SIERRA to keep or narrow the list.
//
// First, the control: an empty list really refuses what the list lets through. A stand-in opens the user clients of
// IOSurface's root and of the power root domain: outside, then in a desk with the list, then in one with none.
func TestOnAMacNormalWorkNeedsNoIOKitClass(t *testing.T) {
	sandboxExecOrSkip(t)
	t.Run("control: an empty IOKit list refuses what the list lets through", func(t *testing.T) {
		cc := tool(t, "cc")
		d := newEscapeDesk(t)
		if err := os.MkdirAll(filepath.Join(d.base, "t"), 0o700); err != nil {
			t.Fatal(err)
		}
		src, prog := filepath.Join(d.base, "src", "iokit.c"), filepath.Join(d.root, "iokit")
		mustWrite(t, src, iokitStandIn)
		outside(t, d.base, []string{"HOME=" + d.home, pathOf(cc)}, cc, "-o", prog, src, "-framework", "IOKit")
		services := []string{"IOSurfaceRoot", "IOPMrootDomain"}
		const both = "IOSurfaceRoot ok\nIOPMrootDomain ok\n"
		if got := outside(t, d.base, nil, prog, services...); got != both {
			t.Fatalf("control: outside a desk, the stand-in says %q, not %q", got, both)
		}
		d.prepareWith(t, nil, IOKitClasses())
		if r := d.work(t, "", d.root, 30*time.Second, nil, prog, services...); r.status != 0 || r.stdout != both {
			t.Errorf("in a desk with the IOKit list: %+v, not %q", r, both)
		}
		d.prepareWith(t, nil, nil)
		r := d.work(t, "", d.root, 30*time.Second, nil, prog, services...)
		lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
		if r.status != 0 || len(lines) != len(services) {
			t.Fatalf("in a desk with no IOKit class: %+v", r)
		}
		for i, line := range lines {
			if !strings.HasPrefix(line, services[i]+" refused ") {
				t.Errorf("in a desk with no IOKit class: %q, not refused", line)
			}
		}
	})

	ended := normalWork(t, nil)
	by := map[string][]string{}
	for _, name := range slices.Sorted(maps.Keys(ended)) {
		by[ended[name]] = append(by[ended[name]], name)
	}
	t.Logf("IOKIT: with no IOKit class, these normal-work rows pass: %s. They fail without one: %s. Not run here: %s. "+
		"The IOKit list (lists.go: IOSurfaceRootUserClient, IOSurfaceSendRight, RootDomainUserClient) is SIERRA's to keep or narrow",
		commaList(by["passed"]), commaList(by["failed"]), commaList(by["skipped"]))
}

// commaList is names, comma separated, or "none".
func commaList(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// writeWheel writes a pure-Python wheel by hand: a package `standin`, nothing to fetch or build.
func writeWheel(t *testing.T, at string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(at)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	files := [][2]string{
		{"standin/__init__.py", "X = \"in a desk\"\n"},
		{"standin-0.1.dist-info/METADATA", "Metadata-Version: 2.1\nName: standin\nVersion: 0.1\n"},
		{"standin-0.1.dist-info/WHEEL", "Wheel-Version: 1.0\nGenerator: hand\nRoot-Is-Purelib: true\nTag: py3-none-any\n"},
		{"standin-0.1.dist-info/RECORD", "standin/__init__.py,,\nstandin-0.1.dist-info/METADATA,,\nstandin-0.1.dist-info/WHEEL,,\nstandin-0.1.dist-info/RECORD,,\n"},
	}
	for _, file := range files {
		part, err := z.Create(file[0])
		if err == nil {
			_, err = part.Write([]byte(file[1]))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// "Claude Code in a desk folder" (§A4.3, SIERRA's calls on X8): the desk's own Claude folder, as X8 makes it, with its
// links into the account's (a stand-in ~/.claude, with plugins installed from a marketplace that is a git clone), read
// and written as Claude Code does at its start and in its work. No real agent starts here (_rules.md): what Claude Code
// itself does when the refresh fails, and `claude --resume` on a desk folder, are the runner's rows (see the report).
//
//   - A skill with a supporting file, and a plugin's skill with one, read through the desk's links: the sandbox lets
//     both through, and the skill's script runs. (Whether Claude Code's own Read rules let the agent read them by the
//     desk's path is the settings' question, not the sandbox's: the report.)
//   - The marketplace refresh: Claude Code pulls the marketplace's clone and rewrites its records, all in the
//     account's plugins folder, which no desk writes (SIERRA, CALL B). A real git pull in the clone fails at once with
//     the system's words, and moving the clone aside (Claude Code's .bak) is refused; nothing of the account's changes.
//   - The desk's own stores (projects, todos, shell snapshots) are written in its own folder.
func TestOnAMacClaudeCodesDeskFolderWorksAndItsMarketplaceRefreshIsRefused(t *testing.T) {
	sandboxExecOrSkip(t)
	git := gitOrSkip(t)
	w := newWorkDesk(t, IOKitClasses())
	account, desk := filepath.Join(w.home, ".claude"), filepath.Join(w.own, "claude")
	plugins := filepath.Join(account, "plugins")
	clone := filepath.Join(plugins, "marketplaces", "m")
	cache := filepath.Join(plugins, "cache", "m", "p", "1.0")
	for at, text := range map[string]string{
		filepath.Join(account, "skills", "s", "reference.md"):      "the skill's supporting file",
		filepath.Join(account, "skills", "s", "scripts", "run.sh"): "echo \"the skill's script ran\"\n",
		filepath.Join(cache, "skills", "ps", "SKILL.md"):           "---\nname: ps\ndescription: a plugin's skill\n---\nRead reference.md.\n",
		filepath.Join(cache, "skills", "ps", "reference.md"):       "the plugin skill's supporting file",
		filepath.Join(cache, ".claude-plugin", "plugin.json"):      `{"name":"p","version":"1.0"}`,
		filepath.Join(plugins, "known_marketplaces.json"):          `{"m":{"source":{"source":"git"},"installLocation":"` + clone + `"}}`,
		filepath.Join(plugins, "installed_plugins.json"):           `{"version":2,"plugins":{"p@m":[{"installPath":"` + cache + `"}]}}`,
	} {
		mustWrite(t, at, text)
	}
	// The marketplace: a repo of the owner's, cloned into the account as Claude Code clones one, then moved on, so a
	// refresh has something to fetch.
	gitEnv := []string{"HOME=" + w.home, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Stand In",
		"GIT_AUTHOR_EMAIL=standin@example.invalid", "GIT_COMMITTER_NAME=Stand In", "GIT_COMMITTER_EMAIL=standin@example.invalid", pathOf(git)}
	source := filepath.Join(w.home, "marketplace")
	mustWrite(t, filepath.Join(source, "marketplace.json"), `{"plugins":[{"name":"p"}]}`)
	outside(t, source, gitEnv, git, "init", "-q", "-b", "main")
	outside(t, source, gitEnv, git, "add", ".")
	outside(t, source, gitEnv, git, "commit", "-q", "-m", "first")
	outside(t, w.home, gitEnv, git, "clone", "-q", source, clone)
	mustWrite(t, filepath.Join(source, "marketplace.json"), `{"plugins":[{"name":"p"},{"name":"q"}]}`)
	outside(t, source, gitEnv, git, "commit", "-q", "-am", "second")
	headBefore := outside(t, clone, gitEnv, git, "rev-parse", "HEAD")
	records, err := os.ReadFile(filepath.Join(plugins, "known_marketplaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	w.prepareWith(t, nil, w.iokit)

	through := func(rel ...string) string { return filepath.Join(append([]string{desk}, rel...)...) }
	w.check(t, []escapeRow{
		{"control: the skill, through the desk's link", attempt{"read", through("skills", "s", "SKILL.md"), ""}, "ok"},
		{"a skill's supporting file, through the desk's link", attempt{"read", through("skills", "s", "reference.md"), ""}, "ok"},
		{"a skill's supporting file, by the account's path", attempt{"read", filepath.Join(account, "skills", "s", "reference.md"), ""}, "ok"},
		{"the installed plugins' records", attempt{"read", through("plugins", "installed_plugins.json"), ""}, "ok"},
		{"a plugin's skill, by its install path", attempt{"read", filepath.Join(cache, "skills", "ps", "SKILL.md"), ""}, "ok"},
		{"a plugin skill's supporting file", attempt{"read", filepath.Join(cache, "skills", "ps", "reference.md"), ""}, "ok"},
		{"a plugin skill's supporting file, through the desk's link", attempt{"read", through("plugins", "cache", "m", "p", "1.0", "skills", "ps", "reference.md"), ""}, "ok"},
		{"its own transcript", attempt{"mkdir", through("projects"), ""}, "ok"},
		{"its own transcript", attempt{"create", through("projects", "s.jsonl"), ""}, "ok"},
		{"its own todos", attempt{"mkdir", through("todos"), ""}, "ok"},
		{"the refresh: the marketplace's records", attempt{"write", through("plugins", "known_marketplaces.json"), ""}, "EPERM"},
		{"the refresh: the clone moved aside, as Claude Code's .bak", attempt{"rename", clone, clone + ".bak"}, "EPERM"},
		{"the refresh: the clone's records", attempt{"write", filepath.Join(clone, ".git", "HEAD"), ""}, "EPERM"},
		{"the refresh: a new version of the plugin", attempt{"mkdir", filepath.Join(plugins, "cache", "m", "p", "2.0"), ""}, "EPERM"},
		{"the refresh: the plugins' records", attempt{"write", through("plugins", "installed_plugins.json"), ""}, "EPERM"},
	})
	if r := w.work(t, "", w.root, 30*time.Second, []string{pathOf()}, "/bin/sh", through("skills", "s", "scripts", "run.sh")); r.status != 0 ||
		strings.TrimSpace(r.stdout) != "the skill's script ran" {
		t.Errorf("the skill's script, through the desk's link: %+v", r)
	}
	// The refresh as git does it: refused at once, in the system's words, never left hanging.
	r := w.work(t, "", clone, time.Minute, gitEnv, git, "pull", "-q", "--ff-only")
	if r.status == 0 || !strings.Contains(r.stderr, "Operation not permitted") {
		t.Errorf("the marketplace's git pull in a desk: %+v", r)
	}
	if got := outside(t, clone, gitEnv, git, "rev-parse", "HEAD"); got != headBefore {
		t.Errorf("the marketplace's clone moved: %s, was %s", got, headBefore)
	}
	if got, _ := os.ReadFile(filepath.Join(plugins, "known_marketplaces.json")); !bytes.Equal(got, records) {
		t.Errorf("the marketplace's records changed: %q", got)
	}
	if _, err := os.Lstat(clone + ".bak"); err == nil {
		t.Error("the clone was moved aside")
	}
}

// The runner only (PL126A §4, A0.5 row 1, ST1S's open row): Smolcorp's grants. A stand-in that holds Accessibility,
// granted once by a person on the Mac Studio runner (never on the owner's Mac; GitHub's runners can't grant it), is
// named by SMOLCORP_TEST_TCC_STANDIN (built from testdata/tccstandin.c, which says how) and, wrapped in an app, by
// SMOLCORP_TEST_TCC_APP. Outside it is trusted, queries the focused app, makes an event tap and opens the HID system's
// connection IOHIDPostEvent posts through, posting nothing. In a desk, the Mach and IOKit lists leave it none of these.
// A hire's copy of the app can't be opened (lsopen), and run in the desk it is refused the same. That part copies an
// app, opens it through LaunchServices and runs the program inside it, so it runs only where SMOLCORP_STUDIO_ROWS=1
// (the Mac Studio, through VICTOR): on any other Mac a hole would launch a second app or show the owner a Gatekeeper
// dialog (_rules.md, "No LaunchServices on this Mac"). TestOnAMacOpenInADeskLaunchesNothing is its twin for here.
func TestOnAMacAStandInWithAccessibilityCantUseItInADesk(t *testing.T) {
	sandboxExecOrSkip(t)
	standIn := os.Getenv("SMOLCORP_TEST_TCC_STANDIN")
	if standIn == "" {
		// Not turned into a failure by SMOLCORP_SANDBOX_REQUIRED: only a person can grant it. Unproven without it (RSBX1).
		t.Skip("SKIPPED, the runner only: SMOLCORP_TEST_TCC_STANDIN names a stand-in a person granted Accessibility (testdata/tccstandin.c)")
	}
	d := newEscapeDesk(t)
	type said struct {
		Trusted bool `json:"trusted"`
		AX      int  `json:"ax"`
		Tap     bool `json:"tap"`
		HID     int  `json:"hid"`
	}
	read := func(where string, r ran) said {
		t.Helper()
		var s said
		if r.status != 0 || json.Unmarshal([]byte(r.stdout), &s) != nil {
			t.Fatalf("the stand-in %s: %+v", where, r)
		}
		return s
	}
	refused := func(where string, s said) {
		t.Helper()
		if s.AX == 0 || s.Tap || s.HID == 0 {
			t.Errorf("TCC: the stand-in %s used its grant: %+v", where, s)
		}
	}
	if s := read("outside", ran{stdout: outside(t, d.base, nil, standIn)}); !s.Trusted || s.AX != 0 || !s.Tap || s.HID != 0 {
		t.Fatalf("control: outside, the stand-in can't use its grant (%+v): grant it Accessibility first", s)
	}
	refused("in a desk", read("in a desk", d.work(t, "", d.root, 30*time.Second, []string{pathOf()}, standIn)))

	app := os.Getenv("SMOLCORP_TEST_TCC_APP")
	if app == "" {
		t.Log("ST1S's row (a hire-made copy of the app): SMOLCORP_TEST_TCC_APP names the stand-in wrapped in an app")
		return
	}
	if os.Getenv("SMOLCORP_STUDIO_ROWS") != "1" {
		// Not turned into a failure by SMOLCORP_SANDBOX_REQUIRED: it may run on the Mac Studio only.
		t.Skip("SKIPPED, ST1S's row opens a copy of an app through LaunchServices and runs the program inside it: " +
			"only where SMOLCORP_STUDIO_ROWS=1 (the Mac Studio), never on a Mac someone uses, where a hole would launch it " +
			"or show a Gatekeeper dialog. TestOnAMacOpenInADeskLaunchesNothing is its twin for here")
	}
	copied := filepath.Join(d.root, "Smolcorp.app")
	if r := d.work(t, "", d.root, time.Minute, []string{pathOf()}, "/bin/cp", "-R", app, copied); r.status != 0 {
		t.Fatalf("the hire's copy of the app: %+v", r)
	}
	marker := filepath.Join(d.root, "opened.json")
	if r := d.work(t, "", d.root, 30*time.Second, []string{pathOf()}, "/usr/bin/open", "-g", "-j", "-n", copied, "--args", marker); r.status == 0 {
		t.Errorf("ST1S: a desk opened its copy of the app: %+v", r)
	}
	if waitFor(marker, "{", 3*time.Second) {
		t.Error("ST1S: the desk's copy of the app ran, opened by LaunchServices")
	}
	program := filepath.Join(copied, "Contents", "MacOS", strings.TrimSuffix(filepath.Base(app), ".app"))
	refused("as the desk's copy of the app", read("as the desk's copy of the app", d.work(t, "", d.root, 30*time.Second, []string{pathOf()}, program)))
}
