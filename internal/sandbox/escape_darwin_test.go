package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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

// SA-M4, the Mac escape table I (PL126A §2.2; SANDBOX-PLAN §A2 and §A6), on this Mac with stand-ins only (owner Q5
// (a)). A stand-in hire runs under the real /usr/bin/sandbox-exec with the real profile: PrepareProfile's, made from
// the Policy of a desk whose every place is in the test's temp folder (a stand-in home, support folder, repo and
// socket folder), with the lists' Mach services, IOKit classes and denied ports. Each row is one harmless attempt,
// expected to be refused, beside positive controls that must work, so a row can't pass because the stand-in failed
// for another reason. Nothing here reads a real secret: the "key" is FAKEKEY, the fence {"fence":1}.
//
// The rows, by their names in §A2 and the reviews (RSBXPLAN-verdict P1-P11, RSBXPLANb-verdict R1-R7):
//
//	P1, P2   an attach client outside runs what the hire's server asks: the server's own read of the key and the
//	         hire's script it runs are refused here (TestOnAMacAHiresTmuxServerReachesNothingOfTheOwners); the client
//	         outside is the guard's, X4's T6 (tmuxfake TestRealServerT6AnAttachedClientRunsNothing)
//	P3       the worktree's .git file rewritten or moved aside: refused here (the file rows)
//	P4       a link in its own folder to the key: made, but nothing inside reads through it; the core's reader that
//	         follows no desk link is X5's (desktop/hirefile.go)
//	P5, P6   a hard link made before the start: the sandbox checks paths, so X12's start-time check refuses the start
//	         (desktop/sandboxstart.go, checkLinks). Here: no new hard link can be made from inside (P7)
//	P7       a hard link to the key, the guard file or the fence, from inside: refused
//	P8       a descriptor opened before the start: `enter` closes it (TestOnAMacADescriptorOpenedBeforeTheStartIsntTheDesks)
//	P9       /.vol/<dev>/<ino>, case spellings, a new `.Claude`: refused
//	P10, P11 a nested sandbox: a hire can't put itself in a looser one (TestOnAMacAHireCantSwapItsSandboxForALooserOne);
//	         that SwiftPM and Chrome need --disable-sandbox and --no-sandbox is M5's normal-work list
//	R1-R3    the hooks a one-shot client outside runs: from inside refused here; the guarded client outside, in the
//	         tmux test (run only with the guarded build: SMOLCORP_TEST_TMUX or build/tmux)
//	R4       the shared git config and hooks, with real git (TestOnAMacAHireInALinkedWorktreeCantChangeWhatTheOwnersGitRuns)
//	         and by file (the file rows)
//	R5       localhost:22 and :2375 over IPv4 and IPv6: EPERM (the file rows)
//	R6, T8   another server in the socket folder, another desk's, a link at its own socket's path: EPERM
//	T1-T3    as R1-R3; T4: the link its socket would be replaced by can't be made; T5: the hire's script run by its
//	         server reaches nothing; T6 as P1
//	T7       a cost, not a refusal (X4)
//	T9       the socket folder's write rules
//	T10      the server check: SA-M3's TestOnAMacTheServerCheckKnowsOurServerFromTheOtherTwo
//	T11      as R4; T12 is the clone per hire, SB16, which the owner left out of 1.26 (Q1 (c))
//	T13      what normal programs need still works (TestOnAMacTheListsLetNormalProgramsRun)
//	T14      the desk's Claude folder, its links into the account's, the account's secrets
//	desk-start, serve.sock   the start helper's socket connects and can't be moved; the owner's tap can't be reached
//
// Skips: only when the Mac has no sandbox-exec, git or tmux to run, each with its reason; SMOLCORP_SANDBOX_REQUIRED=1
// (the Mac Studio runner) makes each such skip a failure.

const (
	escapeKey   = "FAKEKEY"
	escapeFence = `{"fence":1}`
	// standInEscape is the test program as the stand-in hire: its one argument is the attempts as JSON; it prints
	// what each gave as JSON, in order.
	standInEscape = "escape"
)

func init() { moreStandIns[standInEscape] = escapeStandIn }

// attempt is one thing the stand-in hire tries. Op is one of: read (open and read a byte), write (open for writing
// only, writing nothing), create (a new file), mkdir, link (a hard link At to To), symlink (a link at At pointing at
// To), rename (At to To), remove, connect (a unix socket), tcp (an address), fd (fstat descriptor At).
type attempt struct{ Op, At, To string }

func escapeStandIn(args []string) int {
	var tries []attempt
	if len(args) != 1 || json.Unmarshal([]byte(args[0]), &tries) != nil {
		fmt.Fprintln(os.Stderr, "escape: one argument, the attempts as JSON")
		return 2
	}
	said := make([]string, len(tries))
	for i, a := range tries {
		said[i] = errnoName(a.try())
	}
	if err := json.NewEncoder(os.Stdout).Encode(said); err != nil {
		return 3
	}
	return 0
}

func (a attempt) try() error {
	switch a.Op {
	case "read":
		f, err := os.Open(a.At)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Read(make([]byte, 1)); err != nil && err != io.EOF {
			return err
		}
		return nil
	case "write":
		f, err := os.OpenFile(a.At, os.O_WRONLY, 0)
		if err == nil {
			f.Close()
		}
		return err
	case "create":
		f, err := os.OpenFile(a.At, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			f.Close()
		}
		return err
	case "mkdir":
		return os.Mkdir(a.At, 0o700)
	case "link":
		return os.Link(a.At, a.To)
	case "symlink":
		return os.Symlink(a.To, a.At)
	case "rename":
		return os.Rename(a.At, a.To)
	case "remove":
		return os.Remove(a.At)
	case "connect", "tcp":
		network := map[string]string{"connect": "unix", "tcp": "tcp"}[a.Op]
		c, err := net.DialTimeout(network, a.At, 5*time.Second)
		if err == nil {
			c.Close()
		}
		return err
	case "fd":
		fd, err := strconv.Atoi(a.At)
		if err != nil {
			return err
		}
		var st unix.Stat_t
		return unix.Fstat(fd, &st)
	}
	return fmt.Errorf("no such attempt %q", a.Op)
}

// errnoName is "ok", the error number's name (EPERM), or the error's words.
func errnoName(err error) string {
	var errno syscall.Errno
	switch {
	case err == nil:
		return "ok"
	case errors.As(err, &errno):
		return unix.ErrnoName(errno)
	}
	return err.Error()
}

// escapeDesk is a stand-in hire "h1" and everything around it, in one short temp folder: the owner's home with its
// account folder and ssh key, the support folder, the owner's repo with the hire's linked worktree and another
// desk's, and the socket folder. Five stand-in servers listen in the test's process: the desk-start socket, the
// owner's tap (serve.sock), the desk's own socket, the owner's tmux server in the socket folder and another desk's.
type escapeDesk struct {
	base, home, support, own, root, checkout, git, sockets, ownSocket string
	key, profile                                                      string
	pl                                                                Places
	checks                                                            Checks
	port                                                              int
}

func newEscapeDesk(t *testing.T) *escapeDesk {
	t.Helper()
	base := shortTemp(t)
	home := filepath.Join(base, "h")
	d := &escapeDesk{base: base, home: home, support: filepath.Join(home, "AD"), checkout: filepath.Join(home, "app"),
		sockets: filepath.Join(base, "k")}
	d.own = filepath.Join(d.support, "hires", "h1")
	d.root = filepath.Join(home, "app-hires", "h1")
	d.git = filepath.Join(d.checkout, ".git")
	d.ownSocket = filepath.Join(d.sockets, "h1", "s")
	d.key = filepath.Join(d.support, "setup-key")
	for at, text := range map[string]string{
		d.key: escapeKey, filepath.Join(d.support, "hire-guard.json"): escapeFence,
		filepath.Join(d.own, "BRIEF.md"): "the brief", filepath.Join(d.own, "settings.json"): escapeFence,
		filepath.Join(d.support, "hires", "h2", "claude", "notes.md"): "another desk's",
		filepath.Join(home, ".claude", "CLAUDE.md"):                   "the owner's instructions",
		filepath.Join(home, ".claude", "skills", "s", "SKILL.md"):     "a skill",
		filepath.Join(home, ".claude", "plugins", "p", "hooks.json"):  "{}",
		filepath.Join(home, ".claude", "ide", "1.lock"):               escapeKey,
		filepath.Join(home, ".claude", "shell-snapshots", "snap.sh"):  "# the owner's shell",
		filepath.Join(home, ".claude.json"):                           escapeKey,
		filepath.Join(home, ".ssh", "id_ed25519"):                     escapeKey, filepath.Join(home, ".ssh", "id_ed25519.pub"): "ssh-ed25519 stand-in",
		filepath.Join(d.checkout, "README"): "the owner's checkout",
		filepath.Join(d.git, "config"):      "[core]\n\tbare = false\n", filepath.Join(d.git, "HEAD"): "ref: refs/heads/main\n",
		filepath.Join(d.git, "refs", "heads", "main"): strings.Repeat("0", 40) + "\n",
		filepath.Join(d.git, "info", "exclude"):       "", filepath.Join(d.git, "objects", "info", "packs"): "",
		filepath.Join(d.git, "hooks", "pre-commit.sample"):   "",
		filepath.Join(d.git, "worktrees", "h1", "gitdir"):    filepath.Join(d.root, ".git") + "\n",
		filepath.Join(d.git, "worktrees", "h1", "commondir"): "../..\n",
		filepath.Join(d.git, "worktrees", "h1", "HEAD"):      "ref: refs/heads/h1\n",
		filepath.Join(d.git, "worktrees", "h2", "HEAD"):      "ref: refs/heads/h2\n",
		filepath.Join(d.root, ".git"):                        "gitdir: " + filepath.Join(d.git, "worktrees", "h1") + "\n",
		filepath.Join(d.root, ".claude", "settings.json"):    "{}", filepath.Join(d.root, "main.go"): "package main",
		filepath.Join(d.root, "sub", "x.txt"):             "a folder with no .claude",
		filepath.Join(home, "app-hires", "h2", "main.go"): "another desk's work",
	} {
		mustWrite(t, at, text)
	}
	// The links the core puts in the desk's own Claude folder, into the account's (NB6).
	claude := filepath.Join(d.own, "claude")
	if err := os.MkdirAll(claude, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CLAUDE.md", "skills", "plugins"} {
		if err := os.Symlink(filepath.Join(home, ".claude", name), filepath.Join(claude, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, folder := range []string{filepath.Join(d.sockets, "h1"), filepath.Join(d.sockets, "h2")} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, at := range []string{filepath.Join(d.support, "desk-start"), filepath.Join(d.support, "serve.sock"), d.ownSocket,
		filepath.Join(d.sockets, "default"), filepath.Join(d.sockets, "h2", "s")} {
		listenUnix(t, at)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go acceptAll(l)
	d.port = l.Addr().(*net.TCPAddr).Port

	d.pl = Places{GOOS: "darwin", Subject: Hire, Home: home, Support: d.support, OwnID: "h1",
		DeskStart: filepath.Join(d.support, "desk-start"), Root: d.root, Checkout: d.checkout, GitDir: d.git, OwnWorktree: "h1",
		OtherWorktrees: []string{"h1", "h2"}, DefaultBranch: "main", HiresFolder: filepath.Join(home, "app-hires"),
		GMFolder: filepath.Join(home, "app-gm"), SocketParent: d.sockets, OwnSockets: filepath.Join(d.sockets, "h1"), RemoteLogin: true}
	d.profile = filepath.Join(d.own, "sandbox.sb")
	d.prepare(t)
	d.checks = Checks{Support: d.support,
		NoWrite: []string{filepath.Join(d.support, "hire-guard.json"), filepath.Join(d.own, "BRIEF.md"), filepath.Join(d.root, ".claude", "settings.json")},
		Other:   filepath.Join(d.sockets, "o"), Read: []string{filepath.Join(d.own, "BRIEF.md")}, Write: []string{filepath.Join(d.own, "STATUS.md")}}
	return d
}

// prepare writes the desk's real profile for its Places, with the lists as a start would give them.
func (d *escapeDesk) prepare(t *testing.T) {
	t.Helper()
	p := mustNew(t, d.pl)
	svc := Services{OwnSocket: d.ownSocket, DeniedPorts: DeniedPorts(false),
		Sockets: SocketPaths(Sockets("darwin"), d.home, func(string) string { return "" }, false, false)}
	for _, m := range MachServices() {
		svc.Mach = append(svc.Mach, m.Name)
	}
	for _, c := range IOKitClasses() {
		svc.IOKit = append(svc.IOKit, IOKitOpen{Class: c.Class, UserClient: c.UserClient})
	}
	if err := PrepareProfile(p, d.profile, svc); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, at, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// listenUnix is a stand-in server at the path that takes connections and drops them, until the test ends.
func listenUnix(t *testing.T, at string) {
	t.Helper()
	l, err := net.Listen("unix", at)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go acceptAll(l)
}

func acceptAll(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		c.Close()
	}
}

// ran is what a program run as the stand-in hire gave.
type ran struct {
	status         int
	stdout, stderr string
}

// inside runs program in the desk's sandbox, from its worktree, with a small environment and extra, for at most
// twenty seconds. startRefusal comes first: nothing that opens or drives an app, or names an app bundle, but the Mac
// Studio's (escape3_darwin_test.go).
func (d *escapeDesk) inside(t *testing.T, extra []string, program string, args ...string) ran {
	t.Helper()
	if err := startRefusal(program, args); err != nil {
		t.Fatal(err)
	}
	sandboxExec := sandboxExecOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, sandboxExec, append([]string{"-f", d.profile, program}, args...)...))
	cmd.Dir = d.root
	cmd.Env = append([]string{"HOME=" + d.home, "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "TERM=xterm", enterTestVar + "=1"}, extra...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := ran{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		t.Fatalf("%s %q didn't end within twenty seconds (%s)", program, args, r.stderr)
	case errors.As(err, &exitErr):
		r.status = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("%s %q: %v", program, args, err)
	}
	return r
}

// tries runs the attempts as the stand-in hire in its sandbox, and gives what each gave.
func (d *escapeDesk) tries(t *testing.T, tries []attempt) []string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(tries)
	if err != nil {
		t.Fatal(err)
	}
	r := d.inside(t, nil, self, standInEscape, string(data))
	var said []string
	if r.status != 0 || json.Unmarshal([]byte(r.stdout), &said) != nil || len(said) != len(tries) {
		t.Fatalf("the stand-in hire didn't run: %+v", r)
	}
	return said
}

// escapeRow is one attempt and what it must give.
type escapeRow struct {
	row  string
	try  attempt
	want string
}

func (d *escapeDesk) check(t *testing.T, rows []escapeRow) {
	t.Helper()
	tries := make([]attempt, len(rows))
	for i, r := range rows {
		tries[i] = r.try
	}
	for i, got := range d.tries(t, tries) {
		if got != rows[i].want {
			t.Errorf("%s: %s %s %s gave %s, not %s", rows[i].row, rows[i].try.Op, rows[i].try.At, rows[i].try.To, got, rows[i].want)
		}
	}
}

func TestOnAMacAHireReachesNoneOfTheOwnersFilesSocketsOrPorts(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(parts...) }
	claude, account := at(d.own, "claude"), at(d.home, ".claude")
	guard, fence := at(d.support, "hire-guard.json"), at(d.root, ".claude", "settings.json")
	rows := []escapeRow{
		// The positive controls: what a hire works with, under the same profile.
		{"control: its brief reads", attempt{"read", at(d.own, "BRIEF.md"), ""}, "ok"},
		{"control: its status writes", attempt{"create", at(d.own, "STATUS.md"), ""}, "ok"},
		{"control: its worktree writes", attempt{"create", at(d.root, "new.go"), ""}, "ok"},
		{"control: the guard file reads", attempt{"read", guard, ""}, "ok"},
		{"control: its own socket", attempt{"connect", d.ownSocket, ""}, "ok"},
		{"control: a file in its own socket folder (T9)", attempt{"create", at(d.sockets, "h1", "f"), ""}, "ok"},
		{"control: a local port not on the list", attempt{"tcp", fmt.Sprintf("127.0.0.1:%d", d.port), ""}, "ok"},
		{"control: the owner's CLAUDE.md through its link (T14)", attempt{"read", at(claude, "CLAUDE.md"), ""}, "ok"},
		{"control: the owner's skills through their link (T14)", attempt{"read", at(claude, "skills", "s", "SKILL.md"), ""}, "ok"},
		{"control: its own Claude folder (T14)", attempt{"mkdir", at(claude, "shell-snapshots"), ""}, "ok"},
		{"control: its own shell snapshot (T14)", attempt{"create", at(claude, "shell-snapshots", "snap.sh"), ""}, "ok"},
		{"control: the desk-start socket", attempt{"connect", at(d.support, "desk-start"), ""}, "ok"},

		// P1, P2, R1, T1, T5: what the hire's server reads for itself.
		{"P1/R1: the key", attempt{"read", d.key, ""}, "EPERM"},
		// P3, R4, T11: what the owner's git and the core's run.
		{"P3: the worktree's .git file rewritten", attempt{"write", at(d.root, ".git"), ""}, "EPERM"},
		{"P3: the worktree's .git file moved aside", attempt{"rename", at(d.root, ".git"), at(d.root, "git.old")}, "EPERM"},
		{"R4: the shared config", attempt{"write", at(d.git, "config"), ""}, "EPERM"},
		{"R4: the shared config, by git's lock and a rename", attempt{"create", at(d.git, "config.lock"), ""}, "ok"},
		{"R4: the shared config, by git's lock and a rename", attempt{"rename", at(d.git, "config.lock"), at(d.git, "config")}, "EPERM"},
		{"R4: a hook in the shared hooks", attempt{"create", at(d.git, "hooks", "pre-commit"), ""}, "EPERM"},
		{"R4: its own registration's config", attempt{"create", at(d.git, "worktrees", "h1", "config.worktree"), ""}, "EPERM"},
		{"R4: where its worktree finds the shared .git", attempt{"write", at(d.git, "worktrees", "h1", "commondir"), ""}, "EPERM"},
		{"T11: a base branch", attempt{"write", at(d.git, "refs", "heads", "main"), ""}, "EPERM"},
		{"T11: another desk's registration", attempt{"create", at(d.git, "worktrees", "h2", "x"), ""}, "EPERM"},
		{"N9: another desk's worktree", attempt{"create", at(d.home, "app-hires", "h2", "x"), ""}, "EPERM"},
		{"N9: the owner's checkout", attempt{"create", at(d.checkout, "x"), ""}, "EPERM"},
		// P4: a link to the key in its own folder. Its status files are its own to make; nothing inside reads
		// through the link (the core's reader is X5's).
		{"P4: a link to the key, in its own folder", attempt{"symlink", at(d.own, "REPORT.md"), d.key}, "ok"},
		{"P4: the key, through it", attempt{"read", at(d.own, "REPORT.md"), ""}, "EPERM"},
		// P7 (and P5, P6 from inside): a hard link to a denied file.
		{"P7: a hard link to the key", attempt{"link", d.key, at(d.root, "k")}, "EPERM"},
		{"P7: a hard link to the guard file", attempt{"link", guard, at(d.root, "g")}, "EPERM"},
		{"P7: a hard link to the fence", attempt{"link", fence, at(d.root, "f")}, "EPERM"},
		{"P7: a hard link to its brief", attempt{"link", at(d.own, "BRIEF.md"), at(d.root, "b")}, "EPERM"},
		{"R2/T2: the guard file", attempt{"write", guard, ""}, "EPERM"},
		{"R2/T2: the fence", attempt{"write", fence, ""}, "EPERM"},
		{"R2/T2: a new file in the support folder", attempt{"create", at(d.support, "fence.json"), ""}, "EPERM"},
		// R5: the local ports on the list, both ways.
		{"R5: ssh on 127.0.0.1", attempt{"tcp", "127.0.0.1:22", ""}, "EPERM"},
		{"R5: ssh on ::1", attempt{"tcp", "[::1]:22", ""}, "EPERM"},
		{"R5: Docker on 127.0.0.1", attempt{"tcp", "127.0.0.1:2375", ""}, "EPERM"},
		{"R5: Docker on ::1", attempt{"tcp", "[::1]:2375", ""}, "EPERM"},
		// R6, T8: the socket allowlist by its exact path.
		{"R6/T8: the owner's server in the same folder", attempt{"connect", at(d.sockets, "default"), ""}, "EPERM"},
		{"T8: another desk's server", attempt{"connect", at(d.sockets, "h2", "s"), ""}, "EPERM"},
		// T9 and T4: the socket folder's write rules.
		{"T9: another desk's socket removed", attempt{"remove", at(d.sockets, "h2", "s"), ""}, "EPERM"},
		{"T9: a folder beside its own", attempt{"mkdir", at(d.sockets, "h3"), ""}, "EPERM"},
		{"T9/T4: a link in its own socket folder, to the owner's server", attempt{"symlink", at(d.sockets, "h1", "l"), at(d.sockets, "default")}, "EPERM"},
		// T14: the account folder through the desk's links, and its secrets.
		{"T14: the owner's CLAUDE.md through its link", attempt{"write", at(claude, "CLAUDE.md"), ""}, "EPERM"},
		{"T14: a plugin's hook through its link", attempt{"create", at(claude, "plugins", "p", "hook.sh"), ""}, "EPERM"},
		{"T14: a new skill through its link", attempt{"mkdir", at(claude, "skills", "new"), ""}, "EPERM"},
		{"T14: the link itself replaced", attempt{"remove", at(claude, "CLAUDE.md"), ""}, "EPERM"},
		{"T14: the owner's shell snapshot", attempt{"write", at(account, "shell-snapshots", "snap.sh"), ""}, "EPERM"},
		{"T14: the IDE lock", attempt{"read", at(account, "ide", "1.lock"), ""}, "EPERM"},
		{"T14: ~/.claude.json", attempt{"read", at(d.home, ".claude.json"), ""}, "EPERM"},
		{"T14: another desk's Claude folder", attempt{"create", at(d.support, "hires", "h2", "claude", "x"), ""}, "EPERM"},
		{"C5: an ssh key, while Remote Login answers", attempt{"read", at(d.home, ".ssh", "id_ed25519"), ""}, "EPERM"},
		// The desk-start socket: connected to above, and never moved or replaced; the owner's tap, never reached.
		{"desk-start: removed", attempt{"remove", at(d.support, "desk-start"), ""}, "EPERM"},
		{"desk-start: moved aside", attempt{"rename", at(d.support, "desk-start"), at(d.own, "ds")}, "EPERM"},
		{"serve.sock: connected to", attempt{"connect", at(d.support, "serve.sock"), ""}, "EPERM"},
		{"serve.sock: removed", attempt{"remove", at(d.support, "serve.sock"), ""}, "EPERM"},
		{"serve.sock: a link to it", attempt{"symlink", at(d.root, "tap"), at(d.support, "serve.sock")}, "ok"},
		{"serve.sock: connected to through the link", attempt{"connect", at(d.root, "tap"), ""}, "EPERM"},
	}
	rows = append(rows, d.spellings(t)...)
	d.check(t, rows)

	for name, want := range map[string]string{
		d.key: escapeKey, guard: escapeFence, fence: "{}", at(d.git, "config"): "[core]\n\tbare = false\n",
		at(account, "CLAUDE.md"): "the owner's instructions",
	} {
		if data, err := os.ReadFile(name); err != nil || string(data) != want {
			t.Errorf("%s is %q now (%v)", name, data, err)
		}
	}
	for _, name := range []string{at(d.git, "hooks", "pre-commit"), at(d.root, "k"), at(d.sockets, "h3"), at(claude, "skills", "new"),
		at(d.support, "fence.json"), at(d.home, "app-hires", "h2", "x")} {
		if _, err := os.Lstat(name); err == nil {
			t.Errorf("%s was made", name)
		}
	}

	// T8's last row: the desk's own socket path, now a link to the owner's server (put there from outside: inside
	// it can't be made, above). Seatbelt checks where the link leads.
	if err := os.Remove(d.ownSocket); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(at(d.sockets, "default"), d.ownSocket); err != nil {
		t.Fatal(err)
	}
	d.check(t, []escapeRow{{"T8: a link at its own socket's path, to the owner's server", attempt{"connect", d.ownSocket, ""}, "EPERM"}})
}

// spellings are P9's rows: the key by its volume and inode, the support folder and the fences spelled in other
// cases, and a .claude folder made in another case. On a volume that doesn't fold case, another spelling is another
// (missing) file, so those rows want ENOENT; /.vol's row needs a Mac that still serves /.vol.
func (d *escapeDesk) spellings(t *testing.T) []escapeRow {
	t.Helper()
	var rows []escapeRow
	var st unix.Stat_t
	if err := unix.Stat(d.key, &st); err != nil {
		t.Fatal(err)
	}
	vol := fmt.Sprintf("/.vol/%d/%d", st.Dev, st.Ino)
	if data, err := os.ReadFile(vol); err == nil && string(data) == escapeKey {
		rows = append(rows, escapeRow{"P9: the key by /.vol", attempt{"read", vol, ""}, "EPERM"})
	} else {
		t.Logf("P9: this Mac doesn't serve the key at %s (%v): a desk can't either", vol, err)
		rows = append(rows, escapeRow{"P9: the key by /.vol", attempt{"read", vol, ""}, errnoName(err)})
	}
	rel, err := filepath.Rel(d.base, d.key)
	if err != nil {
		t.Fatal(err)
	}
	upper := filepath.Join(d.base, strings.ToUpper(rel))
	folds := false
	if data, err := os.ReadFile(upper); err == nil && string(data) == escapeKey {
		folds = true
	} else {
		t.Logf("P9: this volume doesn't fold case (%v): another spelling is another file", err)
	}
	want := map[bool]string{true: "EPERM", false: "ENOENT"}[folds]
	rows = append(rows,
		escapeRow{"P9: the key, spelled in capitals", attempt{"read", upper, ""}, want},
		escapeRow{"P9: the guard file, spelled in capitals", attempt{"write", filepath.Join(d.base, "H", "ad", "HIRE-GUARD.JSON"), ""}, want},
		escapeRow{"P9: the fence, at .CLAUDE", attempt{"write", filepath.Join(d.root, ".CLAUDE", "settings.json"), ""}, want},
		escapeRow{"P9: the fence, as SETTINGS.json", attempt{"write", filepath.Join(d.root, ".claude", "SETTINGS.json"), ""}, want},
		escapeRow{"P9: a .Claude folder where there's none", attempt{"mkdir", filepath.Join(d.root, "sub", ".Claude"), ""}, "EPERM"},
		escapeRow{"P9: a .claude folder where there's none", attempt{"mkdir", filepath.Join(d.root, "sub", ".claude"), ""}, "EPERM"},
	)
	return rows
}

// P8: a descriptor the core holds open on a secret when it starts a desk isn't the desk's. Through the whole start
// (MacCommand, the real profile, `enter`): every descriptor above 2 is closed, the handshake's too, before the desk's
// program runs, and the core still takes the report.
func TestOnAMacADescriptorOpenedBeforeTheStartIsntTheDesks(t *testing.T) {
	sandboxExecOrSkip(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d := newEscapeDesk(t)
	s := mustStart(t, d.checks)
	key, err := os.Open(d.key)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	var tries []attempt
	for fd := 3; fd <= 9; fd++ {
		tries = append(tries, attempt{"fd", strconv.Itoa(fd), ""})
	}
	tries = append(tries, attempt{"read", "/dev/fd/4", ""}, attempt{"read", d.key, ""})
	data, err := json.Marshal(tries)
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + d.home, "PATH=/usr/bin:/bin", enterTestVar + "=1"}
	cmd, err := MacCommand(s, d.profile, self, env, self, standInEscape, string(data))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Dir = d.root
	cmd.ExtraFiles = append(cmd.ExtraFiles, key, key) // fds 4 and 5, after the handshake's 3
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := s.Wait(30 * time.Second)
	if err := cmd.Wait(); err != nil || waited != nil {
		t.Fatalf("the start: %v, the core's answer: %v (%s)", err, waited, stderr.String())
	}
	var said []string
	if err := json.Unmarshal(stdout.Bytes(), &said); err != nil || len(said) != len(tries) {
		t.Fatalf("the desk's program said %q (%v)", stdout.String(), err)
	}
	for i, got := range said[:7] {
		if got != "EBADF" {
			t.Errorf("descriptor %s: %s", tries[i].At, got)
		}
	}
	if said[7] == "ok" || said[8] != "EPERM" {
		t.Errorf("/dev/fd/4: %s; the key by its name: %s", said[7], said[8])
	}
	if strings.Contains(stdout.String()+stderr.String(), escapeKey) {
		t.Error("the key was printed")
	}
}

// P10, P11: macOS refuses a sandbox inside a sandbox, so a hire can't run itself again under a looser profile.
func TestOnAMacAHireCantSwapItsSandboxForALooserOne(t *testing.T) {
	sandboxExec := sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	r := d.inside(t, nil, sandboxExec, "-p", "(version 1)\n(allow default)\n", "/bin/cat", d.key)
	if r.status == 0 || strings.Contains(r.stdout, escapeKey) || !strings.Contains(r.stderr, "Operation not permitted") {
		t.Errorf("a looser sandbox inside: %+v", r)
	}
	if r := d.inside(t, nil, "/bin/cat", filepath.Join(d.own, "BRIEF.md")); r.status != 0 || r.stdout != "the brief" {
		t.Errorf("the control, its brief: %+v", r)
	}
}

// T13: under the lists (the Mach services, the IOKit classes), the programs a hire's work needs still run.
func TestOnAMacTheListsLetNormalProgramsRun(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	for _, words := range [][]string{
		{"/usr/bin/uname", "-a"},
		{"/usr/sbin/sysctl", "-n", "hw.ncpu"},
		{"/usr/sbin/ioreg", "-c", "IOPlatformExpertDevice", "-d", "1"},
		{"/usr/bin/id", "-un"},
		{filepath.Join(runtime.GOROOT(), "bin", "go"), "version"},
	} {
		if r := d.inside(t, []string{"GOCACHE=" + filepath.Join(d.root, ".gocache")}, words[0], words[1:]...); r.status != 0 || r.stdout == "" {
			t.Errorf("%q: %+v", words, r)
		}
	}
}

// gitOrSkip is a git to run here, or a skip with its reason. /usr/bin/git is Apple's shim, which with no developer
// tools asks to install them (a dialog), so it is used only when xcode-select names a developer folder.
func gitOrSkip(t *testing.T) string {
	t.Helper()
	found, err := exec.LookPath("git")
	if err == nil && found == "/usr/bin/git" {
		if e := disclaim.Cmd(exec.Command("/usr/bin/xcode-select", "-p")).Run(); e != nil {
			err = fmt.Errorf("no developer tools (%v): /usr/bin/git would ask to install them", e)
		}
	}
	if err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("SMOLCORP_SANDBOX_REQUIRED=1 and no git: %v", err)
		}
		t.Skipf("SKIPPED, no git here: %v", err)
	}
	found, err = filepath.Abs(found)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// R4 and T11 with real git: a linked worktree, made as the core makes one, of a repo in the test's folder. From
// inside, the hire's git can't set what the owner's git runs (core.fsmonitor, core.hooksPath, a remote), can't move
// the base branch, and can't put a hook in; it still commits on its own branch. The owner's git then finds nothing
// of the hire's in the shared config.
func TestOnAMacAHireInALinkedWorktreeCantChangeWhatTheOwnersGitRuns(t *testing.T) {
	sandboxExecOrSkip(t)
	git := gitOrSkip(t)
	d := newEscapeDesk(t)
	// The repo is made again by git itself, in place of the hand-made one.
	for _, gone := range []string{d.checkout, d.root, filepath.Join(d.home, "app-hires", "h2")} {
		if err := os.RemoveAll(gone); err != nil {
			t.Fatal(err)
		}
	}
	gitEnv := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Stand In",
		"GIT_AUTHOR_EMAIL=standin@example.invalid", "GIT_COMMITTER_NAME=Stand In", "GIT_COMMITTER_EMAIL=standin@example.invalid",
		"PATH=" + filepath.Dir(git) + ":/usr/bin:/bin"}
	outside := func(dir string, args ...string) string {
		t.Helper()
		cmd := disclaim.Cmd(exec.Command(git, args...))
		cmd.Dir, cmd.Env = dir, append([]string{"HOME=" + d.home}, gitEnv...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v (%s)", args, err, out)
		}
		return string(out)
	}
	if err := os.MkdirAll(d.checkout, 0o700); err != nil {
		t.Fatal(err)
	}
	outside(d.checkout, "init", "-q", "-b", "main")
	outside(d.checkout, "config", "commit.gpgsign", "false")
	mustWrite(t, filepath.Join(d.checkout, "README"), "the owner's checkout")
	outside(d.checkout, "add", "README")
	outside(d.checkout, "commit", "-q", "-m", "first")
	outside(d.checkout, "worktree", "add", "-q", "-b", "h1", d.root)
	outside(d.checkout, "worktree", "add", "-q", "-b", "h2", filepath.Join(d.home, "app-hires", "h2"))
	mustWrite(t, filepath.Join(d.root, ".claude", "settings.json"), "{}")
	d.prepare(t)
	configBefore := outside(d.checkout, "config", "--local", "--list")
	mainBefore := outside(d.checkout, "rev-parse", "main")

	hireGit := func(args ...string) ran {
		t.Helper()
		return d.inside(t, gitEnv, git, args...)
	}
	mustWrite(t, filepath.Join(d.root, "work.go"), "package work")
	if r := hireGit("add", "work.go"); r.status != 0 {
		t.Fatalf("control: the hire's git add: %+v", r)
	}
	if r := hireGit("commit", "-q", "-m", "the hire's work"); r.status != 0 {
		t.Fatalf("control: the hire's commit on its own branch: %+v", r)
	}
	// After its commit, so that moving main to its HEAD is a real change.
	for _, args := range [][]string{
		{"config", "core.fsmonitor", filepath.Join(d.root, "watch.sh")},
		{"config", "core.hooksPath", d.root},
		{"remote", "add", "x", "https://example.invalid/x.git"},
		{"update-ref", "refs/heads/main", "HEAD"},
	} {
		// update-ref says only "couldn't set" (its rename was refused): main's place is checked below.
		r := hireGit(args...)
		if r.status == 0 || args[0] != "update-ref" && !strings.Contains(r.stderr, "Operation not permitted") {
			t.Errorf("the hire's git %q: %+v", args, r)
		}
	}
	if got := outside(d.checkout, "config", "--local", "--list"); got != configBefore {
		t.Errorf("the shared config is now %q, was %q", got, configBefore)
	}
	if got := outside(d.checkout, "rev-parse", "main"); got != mainBefore {
		t.Errorf("main moved: %s, was %s", got, mainBefore)
	}
	if got := outside(d.checkout, "log", "-1", "--format=%s", "h1"); got != "the hire's work\n" {
		t.Errorf("the hire's branch: %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(d.git, "hooks")); slices.ContainsFunc(entries, func(e os.DirEntry) bool {
		return !strings.HasSuffix(e.Name(), ".sample")
	}) {
		t.Errorf("a hook appeared: %v", entries)
	}
}

// tmuxForTheServer is the tmux the hire's server runs as: the guarded build (SMOLCORP_TEST_TMUX, or the newest
// build/tmux one) with the guard off, as `sandbox enter` starts it, else a stock tmux on PATH. guarded is the guarded
// build, or "" with why there's none.
func tmuxForTheServer(t *testing.T) (server, guarded, why string) {
	t.Helper()
	guarded = os.Getenv("SMOLCORP_TEST_TMUX")
	if guarded == "" {
		found, _ := filepath.Glob("../../build/tmux/*/tmux")
		slices.SortFunc(found, func(a, b string) int { return modTimeOf(b).Compare(modTimeOf(a)) })
		if len(found) > 0 {
			guarded, _ = filepath.Abs(found[0])
		}
	}
	if guarded != "" {
		if _, err := os.Stat(guarded); err != nil {
			guarded, why = "", "the guarded build: "+err.Error()
		}
	} else {
		why = "set SMOLCORP_TEST_TMUX to a guarded tmux, or run scripts/build-tmux.sh"
	}
	if guarded != "" {
		return guarded, guarded, ""
	}
	stock, err := exec.LookPath("tmux")
	if err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("SMOLCORP_SANDBOX_REQUIRED=1 and no tmux: %s", why)
		}
		t.Skipf("SKIPPED, no tmux here: %s", why)
	}
	return stock, "", why
}

func modTimeOf(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// P1, P2, R1-R3, T1-T3, T5 with a real tmux server started in the hire's sandbox at its own socket. What the hire's
// own clients and its server try is refused by the sandbox: reading the key, writing the fence, a script that copies
// the key. What a client outside is asked to do is refused by the guard, given the server's pid; that half needs the
// guarded build, and fails under SMOLCORP_SANDBOX_REQUIRED=1 without it.
func TestOnAMacAHiresTmuxServerReachesNothingOfTheOwners(t *testing.T) {
	sandboxExecOrSkip(t)
	program, guarded, why := tmuxForTheServer(t)
	d := newEscapeDesk(t)
	if err := os.Remove(d.ownSocket); err != nil { // the stand-in server makes way for the real one
		t.Fatal(err)
	}
	env := []string{"SMOLCORP_TMUX_SERVER=off"}
	hireTmux := func(args ...string) ran {
		t.Helper()
		return d.inside(t, env, program, append([]string{"-f", "/dev/null", "-S", d.ownSocket}, args...)...)
	}
	r := hireTmux("new-session", "-d", "-P", "-F", "#{pid}", "-s", "main", "cat")
	pid, err := strconv.Atoi(strings.TrimSpace(r.stdout))
	if r.status != 0 || err != nil {
		t.Fatalf("the hire's server didn't start in its sandbox: %+v", r)
	}
	t.Cleanup(func() {
		syscall.Kill(pid, syscall.SIGTERM)
		for i := 0; i < 50 && syscall.Kill(pid, 0) == nil; i++ {
			time.Sleep(20 * time.Millisecond)
		}
		syscall.Kill(pid, syscall.SIGKILL)
	})
	guard := filepath.Join(d.support, "hire-guard.json")
	out := filepath.Join(d.root, "out")

	// From inside: its own client's read and write (the controls of R1 and R2), and what its server runs (P1, P2, T5).
	if r := hireTmux("load-buffer", "-b", "k", d.key); r.status == 0 {
		t.Errorf("R1/T1, its own client read the key: %+v", r)
	}
	if r := hireTmux("show-buffer", "-b", "k"); strings.Contains(r.stdout, escapeKey) {
		t.Errorf("R1/T1, the key is in its buffer: %+v", r)
	}
	hireTmux("set-buffer", "-b", "f", `{"fence":0}`)
	if r := hireTmux("save-buffer", "-b", "f", guard); r.status == 0 {
		t.Errorf("R2/T2, its own client wrote the guard file: %+v", r)
	}
	mustWrite(t, filepath.Join(d.root, "copy.sh"), "/bin/cat "+d.key+" > "+out+"\n")
	hireTmux("run-shell", "/bin/sh "+filepath.Join(d.root, "copy.sh"))
	if data, _ := os.ReadFile(out); strings.Contains(string(data), escapeKey) {
		t.Errorf("P1/T5, its server's script copied the key: %q", data)
	}
	if r := hireTmux("has-session", "-t", "main"); r.status != 0 {
		t.Errorf("control: its own server answers its own client: %+v", r)
	}

	// From outside: the core's one-shot calls through the guarded client, given the server's pid.
	t.Run("the guarded client outside", func(t *testing.T) {
		if guarded == "" {
			if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
				t.Fatalf("SMOLCORP_SANDBOX_REQUIRED=1 and no guarded tmux: %s", why)
			}
			t.Skipf("SKIPPED, no guarded tmux here: %s", why)
		}
		core := func(extra []string, args ...string) ran {
			t.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			// A shell parent, as X4's rows: a client that obeys MSG_DETACHKILL hangs up its parent, not the test.
			cmd := disclaim.Cmd(exec.CommandContext(ctx, "/bin/sh",
				append([]string{"-c", `"$0" "$@"; exit $?`, guarded, "-f", "/dev/null", "-S", d.ownSocket}, args...)...))
			cmd.Env = append([]string{"PATH=/usr/bin:/bin", "HOME=" + d.home, "LANG=C", "TERM=xterm", "SMOLCORP_TMUX_SERVER=" + strconv.Itoa(pid)}, extra...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			r := ran{stdout: stdout.String(), stderr: stderr.String()}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				r.status = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("the guarded client %q: %v", args, err)
			}
			return r
		}
		refused := func(r ran) bool { return r.status == 97 && strings.Contains(r.stderr, "smolcorp guard: refused") }

		hireTmux("set-hook", "-g", "after-capture-pane", "load-buffer -b k2 "+d.key)
		r := core(nil, "capture-pane", "-p", "-t", "main")
		if b := hireTmux("show-buffer", "-b", "k2"); !refused(r) || strings.Contains(b.stdout, escapeKey) {
			t.Errorf("R1/T1: %+v, the hire's buffer %q", r, b.stdout)
		}
		hireTmux("set-hook", "-gu", "after-capture-pane")

		hireTmux("set-hook", "-g", "after-send-keys", "save-buffer -b f "+guard)
		r = core(nil, "send-keys", "-t", "main", "hello", "Enter")
		if data, _ := os.ReadFile(guard); !refused(r) || string(data) != escapeFence {
			t.Errorf("R2/T2: %+v, the guard file %q", r, data)
		}
		hireTmux("set-hook", "-gu", "after-send-keys")

		hireTmux("set-option", "-g", "update-environment", "SBX_SECRET")
		hireTmux("set-hook", "-g", "after-list-sessions", "new-session -d -s leak")
		r = core([]string{"SBX_SECRET=OWNERENV"}, "list-sessions")
		if e := hireTmux("show-environment", "-t", "leak", "SBX_SECRET"); r.status != 0 || strings.Contains(e.stdout, "OWNERENV") {
			t.Errorf("R3/T3: %+v, the hire's environment %+v", r, e)
		}
	})
}
