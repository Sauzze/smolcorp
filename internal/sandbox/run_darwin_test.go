package sandbox

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-M3 on this Mac, with stand-ins only (owner Q5 (a)): the runner's pinned sandbox-exec, the start's words through
// disclaim.Cmd, T10's three servers told apart by the server check, and the spike row: responsibility stays disclaimed
// across sandbox-exec. The servers are this test program run again ("serve"); the stand-in that says who is
// responsible for it is disclaim's testdata/standin.c, built with cc in the test's folder.

// pinnedAs has CheckSandboxExec see the paths as given (mode, owner), and the rest of the path as Apple's: root's
// folders, 0755.
func pinnedAs(t *testing.T, given map[string][2]uint32) {
	t.Helper()
	old := pinnedStat
	pinnedStat = func(path string) (fs.FileMode, uint32, error) {
		if g, ok := given[path]; ok {
			if g[0] == 0 {
				return 0, 0, syscall.ENOENT
			}
			return fs.FileMode(g[0]), g[1], nil
		}
		if path == SandboxExecPath {
			return 0o755, 0, nil
		}
		return fs.ModeDir | 0o755, 0, nil
	}
	t.Cleanup(func() { pinnedStat = old })
}

func TestOnAMacSandboxExecIsPinnedRootsAllTheWayDown(t *testing.T) {
	if err := CheckSandboxExec(); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("this Mac's sandbox-exec: %v", err)
		}
		t.Skipf("this Mac's sandbox-exec isn't Apple's alone: %v", err)
	}
	pinnedAs(t, nil)
	if err := CheckSandboxExec(); err != nil {
		t.Fatalf("Apple's layout refused: %v", err)
	}
	dir := uint32(fs.ModeDir)
	for name, given := range map[string]map[string][2]uint32{
		"sandbox-exec missing":        {SandboxExecPath: {0, 0}},
		"sandbox-exec a link":         {SandboxExecPath: {uint32(fs.ModeSymlink) | 0o755, 0}},
		"sandbox-exec the owner's":    {SandboxExecPath: {0o755, 501}},
		"sandbox-exec group-writable": {SandboxExecPath: {0o775, 0}},
		"sandbox-exec not a program":  {SandboxExecPath: {0o644, 0}},
		"sandbox-exec a folder":       {SandboxExecPath: {dir | 0o755, 0}},
		"/usr/bin the owner's":        {"/usr/bin": {dir | 0o755, 501}},
		"/usr/bin writable by others": {"/usr/bin": {dir | 0o757, 0}},
		"/usr a link":                 {"/usr": {uint32(fs.ModeSymlink) | 0o755, 0}},
		"/usr a file":                 {"/usr": {0o755, 0}},
		"/ group-writable":            {"/": {dir | 0o775, 0}},
	} {
		pinnedAs(t, given)
		if err := CheckSandboxExec(); !errors.Is(err, ErrNoSandboxExec) {
			t.Errorf("%s: %v", name, err)
		}
		cmd, err := MacCommand(&Start{}, "/p.sb", "/smolcorp", nil, "/tmux")
		if cmd != nil || !errors.Is(err, ErrNoSandboxExec) {
			t.Errorf("%s: a start was made: %v", name, err)
		}
	}
}

// The start is smolcorp-spawn, then sandbox-exec by its pinned path with the profile, then Smolcorp's `sandbox enter`
// with the start's words, then the program: through disclaim.Cmd, with the handshake and the nonce, and nothing of the
// core's environment that wasn't given.
func TestOnAMacTheStartIsSmolcorpSpawnThenSandboxExecThenEnter(t *testing.T) {
	if CheckSandboxExec() != nil {
		t.Skip("this Mac's sandbox-exec isn't Apple's alone")
	}
	d := newStandInDesk(t)
	s := mustStart(t, d.checks)
	profile := filepath.Join(d.own, "sandbox.sb")
	if err := os.WriteFile(profile, []byte("(version 1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spawn := filepath.Join(d.base, "bin", disclaim.SpawnName)
	os.MkdirAll(filepath.Dir(spawn), 0o700)
	if err := os.WriteFile(spawn, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	restore := disclaim.StandIn(true, disclaim.Place{Spawn: spawn, InApp: true})
	defer restore()

	t.Setenv("SMOLCORP_CORE_ONLY", "x")
	env := []string{"HOME=" + d.home, nonceVar + "=planted"}
	cmd, err := MacCommand(s, profile, "/app/bin/smolcorp", env, "/app/bin/tmux", "-S", "/k/h1/s", "new-session", "-d")
	if err != nil || cmd.Err != nil {
		t.Fatalf("%v %v", err, cmd.Err)
	}
	want := append([]string{spawn, SandboxExecPath, SandboxExecPath, "-f", profile, "/app/bin/smolcorp"},
		s.Words("/app/bin/tmux", "-S", "/k/h1/s", "new-session", "-d")...)
	if cmd.Path != spawn || !slices.Equal(cmd.Args, want) {
		t.Errorf("the start is %s %q,\nnot %q", cmd.Path, cmd.Args, want)
	}
	if i := slices.Index(cmd.Args, "enter"); i < 0 || cmd.Args[i-1] != "sandbox" || slices.Index(cmd.Args, "--") < i {
		t.Errorf("no `sandbox enter` before the program: %q", cmd.Args)
	}
	if wantEnv := []string{"HOME=" + d.home, s.Env()}; !slices.Equal(cmd.Env, wantEnv) {
		t.Errorf("the environment is %q, not %q", cmd.Env, wantEnv)
	}
	if !slices.Equal(cmd.ExtraFiles, s.Files()) {
		t.Errorf("the descriptors are %v, not the handshake", cmd.ExtraFiles)
	}
	if env[1] != nonceVar+"=planted" {
		t.Error("the caller's environment was changed")
	}
	cmd, _ = MacCommand(s, profile, "/app/bin/smolcorp", nil, "/app/bin/tmux")
	if !slices.Equal(cmd.Env, []string{s.Env()}) {
		t.Errorf("with no environment given, it has %q", cmd.Env)
	}

	// In an app with no smolcorp-spawn, nothing starts.
	os.Remove(spawn)
	cmd, err = MacCommand(s, profile, "/app/bin/smolcorp", nil, "/app/bin/tmux")
	if err != nil || cmd.Err == nil || !strings.Contains(cmd.Err.Error(), "smolcorp-spawn is missing") {
		t.Errorf("a start with no smolcorp-spawn: %v %v", err, cmd.Err)
	} else if err := cmd.Start(); err == nil {
		cmd.Process.Kill()
		t.Error("it started")
	}

	link := filepath.Join(d.own, "link.sb")
	os.Symlink(profile, link)
	for name, c := range map[string][3]string{
		"a relative profile":  {"sandbox.sb", "/app/bin/smolcorp", "/app/bin/tmux"},
		"a relative smolcorp": {profile, "smolcorp", "/app/bin/tmux"},
		"a relative program":  {profile, "/app/bin/smolcorp", "tmux"},
		"no profile":          {filepath.Join(d.own, "none.sb"), "/app/bin/smolcorp", "/app/bin/tmux"},
		"a profile by a link": {link, "/app/bin/smolcorp", "/app/bin/tmux"},
		"a profile a folder":  {d.own, "/app/bin/smolcorp", "/app/bin/tmux"},
	} {
		if cmd, err := MacCommand(s, c[0], c[1], nil, c[2]); cmd != nil || err == nil {
			t.Errorf("%s: a start was made", name)
		}
	}
	if cmd, err := MacCommand(nil, profile, "/app/bin/smolcorp", nil, "/app/bin/tmux"); cmd != nil || !errors.Is(err, ErrStartNotWhole) {
		t.Errorf("no start: %v", err)
	}
}

func TestTheOtherServersNameIsFreshEachTime(t *testing.T) {
	a, b := OtherServerIn("/k"), OtherServerIn("/k")
	if a == b || filepath.Dir(a) != "/k" || !regexp.MustCompile(`^other-[0-9a-f]{16}$`).MatchString(filepath.Base(a)) {
		t.Errorf("%q, %q", a, b)
	}
}

// deskProfile is a stand-in hire's real profile (SA-M1 and SA-M2), with its own socket at <socket folder>/h1/s: the
// profile's path and the socket's.
func deskProfile(t *testing.T, d standInDesk) (profile, socket string) {
	t.Helper()
	own := filepath.Join(d.base, "k", "h1")
	if err := os.MkdirAll(own, 0o700); err != nil {
		t.Fatal(err)
	}
	p := mustNew(t, Places{GOOS: "darwin", Home: d.home, Support: d.support, OwnID: "h1", Root: d.root,
		HiresFolder: filepath.Dir(d.root), SocketParent: filepath.Dir(own), OwnSockets: own})
	profile, socket = filepath.Join(d.own, "sandbox.sb"), filepath.Join(own, "s")
	if err := PrepareProfile(p, profile, Services{OwnSocket: socket}); err != nil {
		t.Fatal(err)
	}
	return profile, socket
}

// serving starts cmd, a stand-in server, and waits until it listens. It is killed when the test ends.
func serving(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	said := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(out).ReadString('\n')
		said <- line
		io.Copy(io.Discard, out)
	}()
	select {
	case line := <-said:
		if line != "listening\n" {
			t.Fatalf("the stand-in server said %q (%s)", line, stderr.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("the stand-in server didn't listen (%s)", stderr.String())
	}
}

// T10: the server check tells the server started in the desk's sandbox, through the whole start (MacCommand, `enter`,
// the handshake), from a server in no sandbox and one in a looser sandbox, at the same kind of socket. Nothing there,
// a link and a plain file are refused too.
func TestOnAMacTheServerCheckKnowsOurServerFromTheOtherTwo(t *testing.T) {
	sandboxExec := sandboxExecOrSkip(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d := newStandInDesk(t)
	profile, socket := deskProfile(t, d)
	s := mustStart(t, d.checks)
	env := []string{"HOME=" + d.home, "PATH=/usr/bin:/bin", enterTestVar + "=1"}
	cmd, err := MacCommand(s, profile, self, env, self, "serve", socket)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Dir = d.root
	serving(t, cmd)
	if err := s.Wait(30 * time.Second); err != nil {
		t.Fatalf("the core refused the start: %v", err)
	}
	pid, err := ServerIsOurs(socket, s.Canary)
	if err != nil {
		t.Fatalf("our server was refused: %v", err)
	}
	if pid != cmd.Process.Pid {
		t.Errorf("the server's pid is %d, not the start's %d: each step should become the next in place", pid, cmd.Process.Pid)
	}

	loose := filepath.Join(d.base, "loose.sb")
	if err := os.WriteFile(loose, []byte("(version 1)\n(allow default)\n(deny file-read-data (literal \"/nowhere\"))\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, words := range map[string][]string{
		"a server in no sandbox":       {self, "serve"},
		"a server in a looser sandbox": {sandboxExec, "-f", loose, self, "serve"},
	} {
		at := filepath.Join(d.base, "k", "h1", strconv.Itoa(len(words)))
		other := disclaim.Cmd(exec.Command(words[0], append(words[1:], at)...))
		other.Env = []string{"PATH=/usr/bin:/bin", enterTestVar + "=1"}
		serving(t, other)
		pid, err := ServerIsOurs(at, s.Canary)
		var refused *Refused
		if !errors.As(err, &refused) || !strings.Contains(refused.Why, "may read the canary") {
			t.Errorf("%s: %d, %v", name, pid, err)
		}
	}

	link, file := filepath.Join(d.base, "k", "link"), filepath.Join(d.base, "k", "file")
	os.Symlink(socket, link)
	os.WriteFile(file, nil, 0o600)
	for name, at := range map[string]string{"nothing there": filepath.Join(d.base, "k", "none"), "a link to our server": link, "a file": file} {
		if _, err := ServerIsOurs(at, s.Canary); !errors.As(err, new(*Refused)) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := ServerIsOurs(socket, Canary{}); !errors.As(err, new(*Refused)) {
		t.Errorf("no canary: %v", err)
	}
}

// What the server check reads, each part faked: only a listener of this user's, whose token names the pid it listened
// with, and that macOS says may not read the canary, passes.
func TestTheServerCheckNeedsTheUserThePidAndTheDenial(t *testing.T) {
	d := newStandInDesk(t)
	socket := filepath.Join(d.base, "s")
	serving(t, func() *exec.Cmd {
		self, _ := os.Executable()
		c := exec.Command(self, "serve", socket)
		c.Env = []string{enterTestVar + "=1"}
		return c
	}())
	me := uint32(os.Getuid())
	old := readServerPeer
	t.Cleanup(func() { readServerPeer = old })
	for name, c := range map[string]struct {
		peer serverPeer
		err  error
		ok   bool
	}{
		"ours":                       {serverPeer{uid: me, pid: 42, tokenPID: 42, denied: 1}, nil, true},
		"another user":               {serverPeer{uid: me + 1, pid: 42, tokenPID: 42, denied: 1}, nil, false},
		"a token for another pid":    {serverPeer{uid: me, pid: 42, tokenPID: 43, denied: 1}, nil, false},
		"no token":                   {serverPeer{uid: me, pid: 42, tokenPID: 0, denied: -2}, nil, false},
		"no pid":                     {serverPeer{uid: me, pid: 0, tokenPID: 0, denied: 1}, nil, false},
		"the canary allowed":         {serverPeer{uid: me, pid: 42, tokenPID: 42, denied: 0}, nil, false},
		"macOS didn't answer":        {serverPeer{uid: me, pid: 42, tokenPID: 42, denied: -1}, nil, false},
		"the credentials not read":   {serverPeer{}, errors.New("stand-in"), false},
		"read, and an error as well": {serverPeer{uid: me, pid: 42, tokenPID: 42, denied: 1}, errors.New("stand-in"), false},
	} {
		readServerPeer = func(int, string) (serverPeer, error) { return c.peer, c.err }
		pid, err := ServerIsOurs(socket, Canary{Path: "/c"})
		if c.ok != (err == nil) || c.ok && pid != 42 {
			t.Errorf("%s: %d, %v", name, pid, err)
		}
	}
}

// built is a C file compiled into folder, or the test skipped if this Mac can't build C. The developer folder is
// checked first: with none, /usr/bin/cc would offer to install the tools in a window.
func built(t *testing.T, folder, name, source string) string {
	t.Helper()
	if out, err := exec.Command("/usr/bin/xcode-select", "-p").Output(); err != nil {
		t.Skipf("no developer tools here: %v", err)
	} else if _, err := os.Stat(strings.TrimSpace(string(out))); err != nil {
		t.Skipf("the developer tools' folder isn't here: %v", err)
	}
	program := filepath.Join(folder, name)
	if out, err := exec.Command("/usr/bin/cc", "-Wall", "-Werror", "-o", program, source).CombinedOutput(); err != nil {
		t.Skipf("cc can't build %s here: %v\n%s", name, err, out)
	}
	return program
}

var saidResponsible = regexp.MustCompile(`pid=(\d+) responsible=(-?\d+)`)

// The spike row (PL126A M3; RT126ST1:96-99 showed it only through /bin/sh): a desk started through MacCommand, so
// through smolcorp-spawn, sandbox-exec and `enter`, is responsible for itself, not for Smolcorp. The holder stands in
// for Smolcorp.app as in disclaim's proof: started on its own account, so what it starts is charged to it unless
// disclaimed. It starts the stand-in three ways: as it is (charged to the holder: the check can see a charge), through
// sandbox-exec alone (still charged to the holder: sandbox-exec passes the charge on, so it is smolcorp-spawn that
// disclaims), and through MacCommand.
func TestOnAMacADeskStartedInItsSandboxIsResponsibleForItself(t *testing.T) {
	sandboxExecOrSkip(t)
	folder := t.TempDir()
	spawn := built(t, folder, disclaim.SpawnName, filepath.Join("..", "..", "packaging", "macos", "smolcorp-spawn.c"))
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--options", "runtime", spawn).CombinedOutput(); err != nil {
		t.Fatalf("signing the stand-in smolcorp-spawn ad hoc: %v\n%s", err, out)
	}
	standin := built(t, folder, "standin", filepath.Join("..", "disclaim", "testdata", "standin.c"))
	holder := exec.Command(spawn, os.Args[0], os.Args[0], "-test.run=^TestSandboxResponsibilityHolder$", "-test.count=1", "-test.v")
	holder.Env = append(os.Environ(), "SANDBOX_HOLDER_SPAWN="+spawn, "SANDBOX_HOLDER_STANDIN="+standin)
	out, err := holder.CombinedOutput()
	if err != nil {
		t.Fatalf("the holder: %v\n%s", err, out)
	}
	holderPid, said := holder.Process.Pid, string(out)
	saw := map[string][2]int{}
	for _, way := range []string{"as it is", "through sandbox-exec alone", "through MacCommand"} {
		_, after, ok := strings.Cut(said, "--- "+way+"\n")
		m := saidResponsible.FindStringSubmatch(after)
		if !ok || m == nil {
			t.Fatalf("the holder said nothing %s: %s", way, said)
		}
		pid, _ := strconv.Atoi(m[1])
		responsible, _ := strconv.Atoi(m[2])
		saw[way] = [2]int{pid, responsible}
	}
	if p := saw["as it is"]; p[1] != holderPid {
		t.Fatalf("as it is, the stand-in %d should be charged to the holder %d, not %d: the check sees no charge here\n%s",
			p[0], holderPid, p[1], said)
	}
	if p := saw["through sandbox-exec alone"]; p[1] != holderPid {
		t.Errorf("through sandbox-exec alone, the stand-in %d is charged to %d, not the holder %d", p[0], p[1], holderPid)
	}
	if p := saw["through MacCommand"]; p[1] != p[0] || p[1] == holderPid {
		t.Errorf("through MacCommand, the stand-in %d should be responsible for itself, not %d (the holder is %d)\n%s",
			p[0], p[1], holderPid, said)
	}
}

// TestSandboxResponsibilityHolder is the holder of the proof above, run only from it.
func TestSandboxResponsibilityHolder(t *testing.T) {
	spawn, standin := os.Getenv("SANDBOX_HOLDER_SPAWN"), os.Getenv("SANDBOX_HOLDER_STANDIN")
	if spawn == "" {
		t.Skip("run by TestOnAMacADeskStartedInItsSandboxIsResponsibleForItself")
	}
	defer disclaim.StandIn(true, disclaim.Place{Spawn: spawn, InApp: true})()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d := newStandInDesk(t)
	profile, _ := deskProfile(t, d)
	plain, err := exec.Command(standin, "show").Output()
	if err != nil {
		t.Fatal(err)
	}
	alone, err := exec.Command(SandboxExecPath, "-f", profile, standin, "show").Output()
	if err != nil {
		t.Fatal(err)
	}
	s := mustStart(t, d.checks)
	cmd, err := MacCommand(s, profile, self, []string{"HOME=" + d.home, "PATH=/usr/bin:/bin", enterTestVar + "=1"}, standin, "show")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != spawn {
		t.Fatalf("MacCommand starts %s, not smolcorp-spawn", cmd.Path)
	}
	cmd.Dir = d.root
	var through strings.Builder
	cmd.Stdout = &through
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := s.Wait(30 * time.Second)
	if err := errors.Join(waited, cmd.Wait()); err != nil {
		t.Fatalf("the start: %v", err)
	}
	fmt.Printf("--- as it is\n%s--- through sandbox-exec alone\n%s--- through MacCommand\n%s", plain, alone, through.String())
}

// SA-M11: ChildIsOurs, for a desk with no server (an ACP agent, its terminals): a child in no sandbox may read the
// canary and is refused; a pid that can't be ours is refused before macOS is asked. A child in its desk's sandbox
// passes it in desktop's end-to-end ACP start (acpsandbox_darwin_test.go).
func TestOnAMacAChildInNoSandboxIsntOurs(t *testing.T) {
	support := t.TempDir()
	c, err := NewCanary(support)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Remove()
	child := disclaim.Cmd(exec.Command("/bin/sleep", "30"))
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Process.Kill(); child.Wait() })
	var refused *Refused
	if err := ChildIsOurs(child.Process.Pid, c); !errors.As(err, &refused) || !strings.Contains(err.Error(), "may read the canary") {
		t.Errorf("a child in no sandbox: %v", err)
	}
	for _, pid := range []int{0, 1} {
		if err := ChildIsOurs(pid, c); !errors.As(err, &refused) {
			t.Errorf("pid %d: %v", pid, err)
		}
	}
	if err := ChildIsOurs(child.Process.Pid, Canary{}); !errors.As(err, &refused) {
		t.Errorf("no canary: %v", err)
	}
}
