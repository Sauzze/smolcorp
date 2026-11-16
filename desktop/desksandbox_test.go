//go:build !windows

package desktop

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-M6's start wiring, with stand-ins: a sandboxed desk's socket in its own new folder, its start through its start-
// only runner alone, the guard variable the core's own at every call, and the engine's and Hand back's starts in the
// sandbox, the server's pid recorded. The Mac's runner itself is desksandbox_darwin_test.go's.

// deskSockets points the sandboxed desks' socket parent at a short temporary folder, not yet made: the start makes it.
// The core's PATH is the system's, and its login shell isn't asked.
func deskSockets(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("", "ds")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	parent := filepath.Join(physical(base), "p")
	saved, savedLogin := deskSocketParent, tmuxLoginPath
	deskSocketParent = func() string { return parent }
	// The programs a desk's start resolves are found on the system's own folders alone: nothing of the owner's home.
	tmuxLoginPath = func() []string { return nil }
	t.Cleanup(func() { deskSocketParent, tmuxLoginPath = saved, savedLogin })
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	return parent
}

func TestASandboxedDesksSocketIsInAFolderOfItsOwnInTheCoresParent(t *testing.T) {
	shortSupport(t)
	parent := deskSockets(t)
	own := deskSocketPath("ab12", 0)
	if own != filepath.Join(parent, "ab12", "s") || deskSocketPath("ab12", 7811) != filepath.Join(parent, "7811-ab12", "s") || deskSocketPath("Ab/..", 0) != "" {
		t.Fatalf("paths: %q %q", own, deskSocketPath("ab12", 7811))
	}
	for path, want := range map[string]string{
		own: "ab12", filepath.Join(parent, "ab12", "t"): "", filepath.Join(parent, "ab12", "x", "s"): "",
		filepath.Join(filepath.Dir(parent), "q", "ab12", "s"): "", filepath.Join(parent, "AB", "s"): "", "ab12/s": "",
	} {
		if got := deskSocketID(path, 0); got != want {
			t.Errorf("deskSocketID(%q) = %q, want %q", path, got, want)
		}
	}
	if deskSocketID(deskSocketPath("ab12", 7811), 7811) != "ab12" || deskSocketID(own, 7811) != "" {
		t.Error("a test copy owns only its own port's desks")
	}
	if !ownsHireSocket(own, 0) || !inHireSocketFolder(own) {
		t.Error("a sandboxed desk's socket isn't one of ours")
	}

	// The parent is made 0700; the desk's folder is new for its start, 0700.
	if err := makeDeskSocketFolder(own); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{parent, filepath.Dir(own)} {
		if info, err := os.Lstat(folder); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Errorf("%s: %v %v", folder, info.Mode(), err)
		}
	}
	// Anything at its folder refuses the start: the folder, or a socket in it.
	if err := makeDeskSocketFolder(own); err == nil {
		t.Error("a start went ahead with its folder there already")
	}
	listenAt(t, own)
	if err := makeDeskSocketFolder(own); err == nil {
		t.Error("a start went ahead with a socket there already")
	}
	// Its row's socket is its own only while it is that socket.
	if recordedOwnSocket(Hire{ID: "ab12", TmuxSocket: own}, 0) != own || recordedOwnSocket(Hire{ID: "cd34", TmuxSocket: own}, 0) != "" {
		t.Error("recordedOwnSocket")
	}
	// A server answers: its folder stays. One left by an ended server goes, and the folder with it.
	removeDeskSocketFolder(own)
	if _, err := os.Lstat(own); err != nil {
		t.Errorf("a live desk's socket was taken away: %v", err)
	}
	dead := deskSocketPath("dead", 0)
	os.Mkdir(filepath.Dir(dead), 0o700)
	ln, _ := net.Listen("unix", dead)
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	removeDeskSocketFolder(dead)
	if _, err := os.Lstat(filepath.Dir(dead)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an ended desk's socket folder stayed: %v", err)
	}

	// A parent that others can open, or a link, refuses every start.
	os.Chmod(parent, 0o755)
	if err := makeDeskSocketFolder(deskSocketPath("ef56", 0)); err == nil {
		t.Error("a parent others can open was used")
	}
	os.Chmod(parent, 0o700)
	linked := deskSockets(t)
	os.MkdirAll(filepath.Join(filepath.Dir(linked), "elsewhere"), 0o700)
	os.Symlink(filepath.Join(filepath.Dir(linked), "elsewhere"), linked)
	if err := makeDeskSocketFolder(deskSocketPath("ef56", 0)); err == nil {
		t.Error("a parent that is a link was used")
	}
}

func TestASandboxedDeskStartsOnlyThroughItsStartRunner(t *testing.T) {
	shortSupport(t)
	deskSockets(t)
	r := newRecorder()
	r.output = "%0 4242 x"
	h, err := newDeskTmux("ab12", 0, r.run)
	if err != nil || !h.sandboxed || h.socket != deskSocketPath("ab12", 0) {
		t.Fatalf("%+v %v", h, err)
	}
	command := []string{"/app/smolcorp", "desk-start", "/s/desk-start", "42", "ticket"}
	// With no runner of its own, nothing starts: never through the runner every call takes.
	if _, _, _, err := h.start("/w", command); err == nil || len(r.asked) != 0 {
		t.Fatalf("a sandboxed desk started without its runner: %v %q", err, r.asked)
	}
	var asked [][]string
	server, said := 4321, ""
	var refusal error
	h.startOnly = func(args []string) (string, int, error) {
		asked = append(asked, args)
		return said, server, refusal
	}
	said = "%3 777 " + h.socket
	pane, socket, pid, err := h.start("/w", command)
	if err != nil || pane != "%3" || socket != h.socket || pid != 777 || h.server != 4321 || h.guard() != "4321" {
		t.Fatalf("start: %q %q %d %v; server %d", pane, socket, pid, err, h.server)
	}
	want := append([]string{"-S", h.socket, "start-server", ";", "set-environment", "-gu", "SMOLCORP_TMUX_SERVER", ";",
		"new-session", "-d", "-P", "-F", "#{pane_id} #{pane_pid} #{socket_path}", "-s", "hire", "-c", "/w"}, command...)
	if len(asked) != 1 || !slices.Equal(asked[0], want) || len(r.asked) != 0 {
		t.Errorf("the start asked %q (and the runner %q),\nnot %q", asked, r.asked, want)
	}
	// A server the core didn't check, or a refusal, starts nothing; the refusal's words are the owner's.
	h.server, server = 0, 0
	if _, _, _, err := h.start("/w", command); !errors.Is(err, errTmuxDidntStart) {
		t.Errorf("an unchecked server: %v", err)
	}
	refusal = errors.New(sandbox.Refusal("darwin"))
	if _, _, _, err := h.start("/w", command); err == nil || err.Error() != sandbox.Refusal("darwin") {
		t.Errorf("the refusal: %v", err)
	}

	// The real one is the tmux bundled with Smolcorp, or none: never another on the PATH or Homebrew's.
	savedBundled, savedPath := bundledTmux, tmuxPath
	t.Cleanup(func() { bundledTmux, tmuxPath = savedBundled, savedPath })
	other := fakeTmuxScript(t, "other")
	tmuxPath = func() string { return other }
	bundledTmux = func() string { return "" }
	if h, err := newDeskTmux("ab12", 0, nil); h != nil || !errors.Is(err, errNoBundledTmux) {
		t.Errorf("no bundled tmux: %+v %v", h, err)
	}
	bundled := fakeTmuxScript(t, "bundled")
	bundledTmux = func() string { return bundled }
	if h, err := newDeskTmux("ab12", 0, nil); err != nil || h.program != physical(bundled) || h.startOnly != nil {
		t.Errorf("the bundled tmux: %+v %v", h, err)
	}
}

// fakeTmuxScript is a stand-in tmux that prints its guard variable as it got it, "unset" for none.
func fakeTmuxScript(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "tmux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho \"${SMOLCORP_TMUX_SERVER-unset}\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// The release gate's rule for these starts (RT126SAL5): the guard variable is the core's at every call, never one it
// inherited. The start of a desk outside the sandbox gives "off" for that call alone; a sandboxed desk's calls give
// the pid its row records; a desk's outside the sandbox give "unchecked" (SA-M7); a call with none isn't run.
func TestTheCoreGivesTheTmuxGuardItselfNeverItsOwn(t *testing.T) {
	shortSupport(t)
	deskSockets(t)
	t.Setenv("SMOLCORP_TMUX_SERVER", "4242")
	fake := fakeTmuxScript(t, "bin")
	if out, _, ran := oneShotTmux(fake, func() string { return "" })(nil); ran || out != "" {
		t.Errorf("a call with no guard of the core's was run: %q", out)
	}
	if out, _, _ := oneShotTmux(fake, func() string { return "77" })(nil); out != "77" {
		t.Errorf("the guard given: %q", out)
	}
	plain := hireTmuxUsing(filepath.Join(hireSocketFolder(), "smolcorp-ab12"), 0, nil, func(string) string { return fake })
	if plain == nil || plain.startOnly == nil {
		t.Fatal("no start-only runner for a desk outside the sandbox")
	}
	if out, _, err := plain.startOnly(plain.command(tmuxServerStart("new-session", "-d")...)); err != nil || out != "off" {
		t.Errorf("its start: %q %v", out, err)
	}
	// Any other call through it is refused, with nothing run (RT126SAM7R2 B1′): the stand-in would print "off".
	for _, args := range [][]string{nil, plain.command("new-window", "-d", "-c", "/w"),
		plain.command(tmuxServerStart("new-session", "-d", ";", "kill-server")...),
		plain.command(tmuxServerStart("new-session", "-d", "true;", "kill-server")...),
		append([]string{"-S", "/elsewhere"}, tmuxServerStart("new-session", "-d")...)} {
		if out, _, err := plain.startOnly(args); !errors.Is(err, errTmuxDidntStart) || out != "" {
			t.Errorf("a call that isn't its start, %q: %q %v", args, out, err)
		}
	}
	if out, _, _ := plain.run(nil); out != tmuxUnchecked {
		t.Errorf("its other calls: %q", out)
	}
	// A sandboxed desk is driven by its recorded tmux, told its recorded server, and by no other.
	socket := deskSocketPath("ab12", 0)
	writeRegistryHires(t, Hire{ID: "ab12", State: "working", TmuxSocket: socket, TmuxProgram: fake, TmuxServer: 99})
	desk := hireTmuxAt(socket, 0, nil)
	if desk == nil || !desk.sandboxed || desk.startOnly != nil {
		t.Fatalf("%+v", desk)
	}
	if out, _, _ := desk.run(nil); out != "99" {
		t.Errorf("a sandboxed desk's call: %q", out)
	}
	savedPath := tmuxPath
	t.Cleanup(func() { tmuxPath = savedPath })
	tmuxPath = func() string { return fake }
	writeRegistryHires(t, Hire{ID: "ab12", State: "working", TmuxSocket: socket, TmuxProgram: filepath.Join(t.TempDir(), "gone"), TmuxServer: 99})
	if hireTmuxAt(socket, 0, nil) != nil {
		t.Error("a sandboxed desk whose tmux is gone was driven by another")
	}
}

// writeRegistryHires puts hires in the registry, as the only ones.
func writeRegistryHires(t *testing.T, hires ...Hire) {
	t.Helper()
	if err := updateRegistry(func(f *registryFile) { f.hires = hires }); err != nil {
		t.Fatal(err)
	}
}

// sandboxRig is an engine rig whose desks start in their sandbox, through a stand-in start-only runner: what it was
// given is kept, and it answers as a checked server would, at pid 5151.
type sandboxRig struct {
	*engineRig
	desks   []*deskSandbox
	starts  [][]string
	refusal error
}

func newSandboxRig(t *testing.T) *sandboxRig {
	r := &sandboxRig{engineRig: newEngineRig(t)}
	deskSockets(t)
	r.o.Sandbox = func(s *deskSandbox) tmuxStarter {
		r.desks = append(r.desks, s)
		return func(args []string) (string, int, error) {
			r.starts = append(r.starts, args)
			// As the Mac's runner: its socket folder made new first, and taken away when the start is refused.
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, errors.New(takenPlace)
			}
			if r.refusal != nil {
				removeDeskSocketFolder(args[1])
				return "", 0, r.refusal
			}
			listenAt(t, args[1])
			r.tmux.up = true
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	return r
}

func TestEveryEngineDeskStartsInItsSandboxAndItsServerIsRecorded(t *testing.T) {
	r := newSandboxRig(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	row := registered(t)[0]
	if row.TmuxSocket != deskSocketPath(hire.ID, 0) || row.TmuxServer != 5151 || hire.TmuxServer != 5151 {
		t.Fatalf("the row: %+v", row)
	}
	if len(r.starts) != 1 || r.tmux.ran("new-session") || !slices.Contains(r.starts[0], "new-session") {
		t.Errorf("the start went %q; the runner every call takes was asked %q", r.starts, r.tmux.asked)
	}
	if len(r.desks) != 1 {
		t.Fatalf("%d sandboxes", len(r.desks))
	}
	pl := r.desks[0].places
	if pl.Subject != sandbox.Hire || pl.OwnID != hire.ID || pl.Root != physical(hire.Worktree) || pl.Checkout != physical(r.repo) ||
		pl.OwnSockets != physical(filepath.Dir(row.TmuxSocket)) || pl.SocketParent != physical(deskSocketParent()) ||
		pl.GitDir != physical(filepath.Join(r.repo, ".git")) || pl.OwnWorktree == "" || pl.Support != physical(supportDir) ||
		pl.HiresFolder != physical(hiresFolder(r.repo, r.o.Worktrees)) || pl.Home != physical(homeDir) || pl.DeskStart == "" || len(pl.InstallRoots) == 0 {
		t.Errorf("its places: %+v", pl)
	}
	if kept, ok := deskSandboxOf(hire.ID); !ok || kept != r.desks[0] {
		t.Error("its sandbox isn't kept for its stop")
	}
	// The GM is a desk in its sandbox too, as the GM.
	gm, err := runManagedHire(r.request(gmRole), r.o)
	if err != nil || r.desks[1].places.Subject != sandbox.GM {
		t.Errorf("the GM: %v", err)
	}
	// The release gate's item 4b and F4 (SA-L2): on Linux each desk's guard is told its kept copy before and after each
	// call, and its view knows it by the id its branch is named by, the GM's too; on a Mac, no copy.
	for i, desk := range []Hire{hire, gm} {
		before, after := guardWordsIn(t, filepath.Join(hireFolder(supportDir, desk.ID), hireSettingsFile))
		kept := keptForThisSystem(desk.ID)
		held := slices.Index(before, "--held")
		if kept == "" && held >= 0 || kept != "" && (held < 0 || before[held+1] != kept) {
			t.Errorf("%s: the guard's words before a call: %q, want the kept copy %q", desk.ID, before, kept)
		}
		for _, event := range []string{"PostToolUse", "PostToolUseFailure"} {
			words := after[event]
			if len(words) < 6 || words[2] != "--settings" || words[4] != "--sum" ||
				kept != "" && !slices.Equal(words[6:], []string{"--held", kept}) || kept == "" && len(words) != 6 {
				t.Errorf("%s: the guard's words after a call (%s): %q, want the kept copy %q", desk.ID, event, words, kept)
			}
		}
		own := r.desks[i].places.OwnID
		if row := registryRow(t, desk.ID); row.Branch != "" && !strings.HasPrefix(row.Branch, sandbox.HireBranch(own, "")) {
			t.Errorf("%s: its branch %q isn't in its view's branch folder (%q)", desk.ID, row.Branch, own)
		}
	}
}

// registryRow is the registry's row for id.
func registryRow(t *testing.T, id string) Hire {
	t.Helper()
	for _, row := range registered(t) {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("no row for %s", id)
	return Hire{}
}

func TestASandboxedStartRefusedTakesEverythingBack(t *testing.T) {
	r := newSandboxRig(t)
	r.refusal = errors.New(takenPlace)
	_, err := runManagedHire(r.request("janitor"), r.o)
	var refused *hireRefusal
	if !errors.As(err, &refused) || refused.Message != takenPlace {
		t.Fatalf("%v", err)
	}
	if len(registered(t)) != 0 || branches(t, r.repo) != "" || len(r.desks) != 1 {
		t.Errorf("left behind: %+v, branches %q", registered(t), branches(t, r.repo))
	}
	if _, ok := deskSandboxOf(r.desks[0].at.ID); ok {
		t.Error("a refused desk's sandbox is still kept")
	}
	if entries, _ := os.ReadDir(deskSocketParent()); len(entries) != 0 {
		t.Errorf("its socket folder stayed: %v", entries)
	}
}

func TestADeskWhoseSandboxCantBeMadeNeverStarts(t *testing.T) {
	r := newSandboxRig(t)
	// A file the desk may not read with a second name, made by something outside any sandbox: the link check refuses.
	secret := filepath.Join(supportDir, "events.jsonl")
	os.WriteFile(secret, []byte("{}\n"), 0o600)
	if err := os.Link(secret, filepath.Join(supportDir, "events-copy.jsonl")); err != nil {
		t.Fatal(err)
	}
	_, err := runManagedHire(r.request("janitor"), r.o)
	var refused *hireRefusal
	if !errors.As(err, &refused) || refused.Status != 409 || !strings.Contains(refused.Message, "events") {
		t.Fatalf("%v", err)
	}
	if len(r.desks) != 0 || len(r.starts) != 0 || len(registered(t)) != 0 || branches(t, r.repo) != "" {
		t.Errorf("a desk whose sandbox couldn't be made went ahead: %d %q %+v", len(r.desks), r.starts, registered(t))
	}
}

func TestADeskThatNeverReportsLetsGoOfItsSandbox(t *testing.T) {
	r := newSandboxRig(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	r.o.Within, r.o.Every = time.Millisecond, time.Millisecond
	watchManagedHire(hire, 0, r.o)
	if _, ok := deskSandboxOf(hire.ID); ok {
		t.Error("a desk that never started still has its sandbox kept")
	}
}

// takenPlace stands in for a runner's refusal, as the Mac's says it (sandbox.NotOurServerRefusal).
const takenPlace = "Something else was already using this hire's place, so Smolcorp didn't start it. Quit other copies of Smolcorp and try again."
