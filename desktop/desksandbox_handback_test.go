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
)

// RT126SAM6's rows: the bundled tmux never run without the server pid the core checked (B2, the release gate), a
// sandboxed desk's Take over stop and Hand back resume (B2), §A5.2's words from Hand back (B4), a desk's stop listed
// and let go at the fire, after a core restart too (B3), and the inherited guard variable never handed on (G1).

// staleSocketAt leaves a socket file at path that nothing answers: a server that ended without taking it away.
func staleSocketAt(t *testing.T, path string) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
}

// sandboxedForHandBack is a deck whose engine starts its desks in their sandbox through a stand-in start-only runner
// (its socket folder made new, a server checked at pid 5151), and one desk hired there. starts counts that runner's
// starts.
func sandboxedForHandBack(t *testing.T) (d *Deck, r *engineRig, hire Hire, starts *int) {
	t.Helper()
	d, r, _ = engineDeck(t)
	deskSockets(t)
	starts = new(int)
	r.o.Sandbox = func(*deskSandbox) tmuxStarter {
		return func(args []string) (string, int, error) {
			*starts++
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, errors.New(takenPlace)
			}
			staleSocketAt(t, args[1])
			r.tmux.up = true
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	d.engine.Sandbox = r.o.Sandbox
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	return d, r, hire, starts
}

// guardLoggingTmux is a stand-in bundled tmux: it logs the guard variable it got ("UNSET" for none) and its words,
// exits 98 without one as the guard does, says its session is up while the file up is there, shows screen, and ends
// its session and server by taking up away.
func guardLoggingTmux(t *testing.T, screen string) (program, log, up string) {
	t.Helper()
	dir := t.TempDir()
	program, log, up = filepath.Join(dir, "bin", "tmux"), filepath.Join(dir, "log"), filepath.Join(dir, "up")
	shown := filepath.Join(dir, "screen")
	os.MkdirAll(filepath.Dir(program), 0o755)
	os.WriteFile(shown, []byte(screen), 0o600)
	script := `#!/bin/sh
echo "${SMOLCORP_TMUX_SERVER-UNSET} $*" >> '` + log + `'
[ -n "${SMOLCORP_TMUX_SERVER+x}" ] || exit 98
for word; do
	case "$word" in
	has-session) [ -e '` + up + `' ] && exit 0; exit 1;;
	capture-pane) cat '` + shown + `'; exit 0;;
	kill-session|kill-server) rm -f '` + up + `'; exit 0;;
	esac
done
exit 0
`
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return program, log, up
}

func loggedCalls(t *testing.T, log string) []string {
	t.Helper()
	data, _ := os.ReadFile(log)
	return strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' })
}

func TestASandboxedDeskNeverRunsTheBundledTmuxWithoutItsCheckedServer(t *testing.T) {
	shortSupport(t)
	deskSockets(t)
	t.Setenv("SMOLCORP_TMUX_SERVER", "inherited")
	program, log, up := guardLoggingTmux(t, "")
	os.WriteFile(up, nil, 0o600)
	saved := bundledTmux
	t.Cleanup(func() { bundledTmux = saved })
	bundledTmux = func() string { return program }

	// Before its start has checked a server, nothing of it runs: not a look, not an end.
	h, err := newDeskTmux("ab12", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, known := h.alive(); known {
		t.Error("its server was asked before its start checked one")
	}
	h.capture(10)
	h.end()
	h.endServer()
	if calls := loggedCalls(t, log); len(calls) != 0 {
		t.Errorf("the bundled tmux ran before its start: %q", calls)
	}
	// Driven from its row: told the pid its row records, and not run at all for a row that records none.
	socket := deskSocketPath("ab12", 0)
	writeRegistryHires(t, Hire{ID: "ab12", State: "working", TmuxSocket: socket, TmuxProgram: program})
	if desk := hireTmuxAt(socket, 0, nil); desk == nil {
		t.Fatal("no client for its row")
	} else if _, known := desk.alive(); known {
		t.Error("a row with no checked server was asked")
	}
	if calls := loggedCalls(t, log); len(calls) != 0 {
		t.Errorf("the bundled tmux ran with no checked server: %q", calls)
	}
	writeRegistryHires(t, Hire{ID: "ab12", State: "working", TmuxSocket: socket, TmuxProgram: program, TmuxServer: 99})
	if up, known := hireTmuxAt(socket, 0, nil).alive(); !up || !known {
		t.Errorf("its checked server: %v %v", up, known)
	}
	if calls := loggedCalls(t, log); len(calls) != 1 || !strings.HasPrefix(calls[0], "99 ") {
		t.Errorf("its calls: %q", calls)
	}
}

// RT126SAM6 nit 2: a start the core refuses after its server was checked ends that server.
func TestARefusedSandboxedStartEndsTheServerItChecked(t *testing.T) {
	shortSupport(t)
	deskSockets(t)
	r := newRecorder()
	h, err := newDeskTmux("ab12", 0, r.run)
	if err != nil {
		t.Fatal(err)
	}
	h.startOnly = func([]string) (string, int, error) { return "not a pane", 4321, nil }
	if _, _, _, err := h.start("/w", []string{"/app/smolcorp", "desk-start"}); !errors.Is(err, errTmuxDidntStart) {
		t.Fatalf("%v", err)
	}
	if !slices.ContainsFunc(r.asked, func(a []string) bool { return slices.Contains(a, "kill-server") }) {
		t.Errorf("its checked server was left running: %q", r.asked)
	}
}

func TestASandboxedHandBackStopsAndResumesTheDeskInItsSandbox(t *testing.T) {
	d, r, hire, starts := sandboxedForHandBack(t)
	row := registered(t)[0]
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine.Tmux = r.o.Tmux

	// Take over's stop: its session ended, then its server, its socket folder taken away, its sandbox let go.
	if err := d.t3StopInTmux(row); err != nil {
		t.Fatal(err)
	}
	if !r.tmux.ran("kill-session") || !r.tmux.ran("kill-server") || r.tmux.up {
		t.Errorf("not stopped: %q", r.tmux.asked)
	}
	if _, err := os.Lstat(filepath.Dir(row.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its socket folder stayed: %v", err)
	}
	if _, ok := deskSandboxOf(hire.ID); ok {
		t.Error("a stopped desk's sandbox is still kept")
	}

	// Hand back's resume: in its sandbox, through its start-only runner alone, nothing asked of tmux before it.
	before := len(r.tmux.asked)
	resumed, err := d.t3ResumeInTmux(row, t3TakeOverSession, nil)
	if err != nil || resumed.Socket != deskSocketPath(hire.ID, 0) || resumed.Server != 5151 || resumed.Pane <= 1 { // its pane's process too (SA-M7)
		t.Fatalf("%+v %v", resumed, err)
	}
	if *starts != 2 || len(r.tmux.asked) != before {
		t.Errorf("%d starts; tmux asked besides: %q", *starts, r.tmux.asked[before:])
	}
	if _, ok := deskSandboxOf(hire.ID); !ok {
		t.Error("the resumed desk's sandbox isn't kept for its stop")
	}

	// A desk whose session ended by itself has nothing to stop, and its folder goes too: the next resume makes it new.
	r.tmux.up = false
	if err := d.t3StopInTmux(row); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Dir(row.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an ended desk's socket folder stayed: %v", err)
	}
	if _, err := d.t3ResumeInTmux(row, t3TakeOverSession, nil); err != nil {
		t.Errorf("resumed again: %v", err)
	}
}

// The probe of RT126SAM6 B2, as a row: with the core's own guard variable planted, Take over's stop and Hand back's
// resume of a sandboxed desk run the bundled tmux only told the pid the core checked.
func TestASandboxedHandBackRunsTheBundledTmuxOnlyWithItsCheckedServer(t *testing.T) {
	d, _, hire, _ := sandboxedForHandBack(t)
	t.Setenv("SMOLCORP_TMUX_SERVER", "inherited")
	program, log, up := guardLoggingTmux(t, pane(t, "idle-empty"))
	os.WriteFile(up, nil, 0o600)
	saved := bundledTmux
	t.Cleanup(func() { bundledTmux = saved })
	bundledTmux = func() string { return program }
	if err := updateRegistry(func(f *registryFile) { f.hires[0].TmuxProgram = program }); err != nil {
		t.Fatal(err)
	}
	d.engine.Tmux = nil
	row := registered(t)[0]
	if row.TmuxServer != 5151 {
		t.Fatalf("%+v", row)
	}

	if err := d.t3StopInTmux(row); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(up); !errors.Is(err, os.ErrNotExist) {
		t.Error("its session wasn't ended")
	}
	resumed, err := d.t3ResumeInTmux(row, t3TakeOverSession, nil)
	if err != nil || resumed.Program != recordableTmux(program) || resumed.Server != 5151 || resumed.Socket != deskSocketPath(hire.ID, 0) {
		t.Fatalf("%+v %v", resumed, err)
	}
	calls := loggedCalls(t, log)
	for _, verb := range []string{"has-session", "capture-pane", "kill-session", "kill-server"} {
		if !slices.ContainsFunc(calls, func(c string) bool { return strings.Contains(c, " "+verb) }) {
			t.Errorf("no %s: %q", verb, calls)
		}
	}
	for _, c := range calls {
		if !strings.HasPrefix(c, "5151 ") {
			t.Errorf("the bundled tmux ran without the server the core checked: %q", c)
		}
	}
}

// RT126SAM6 B4: Hand back says a missing bundled tmux in §A5.2's own words.
func TestASandboxedHandBackWithNoBundledTmuxSaysSo(t *testing.T) {
	d, _, _, starts := sandboxedForHandBack(t)
	saved := bundledTmux
	t.Cleanup(func() { bundledTmux = saved })
	bundledTmux = func() string { return "" }
	d.engine.Tmux = nil
	_, err := d.t3ResumeInTmux(registered(t)[0], t3TakeOverSession, nil)
	if err == nil || err.Error() != "This copy of Smolcorp is missing its tmux. Reinstall Smolcorp." || *starts != 1 {
		t.Errorf("%v, %d starts", err, *starts)
	}
}

// RT126SAM6 B3: the engine's fire, once it saw the session gone, lists the desk at its stop with its sandbox (the one
// kept, or after a core restart one made again from its row) and lets it go: nothing stays kept, no socket folder
// stays.
func TestTheFireListsADesksSandboxAtItsStopEvenAfterARestart(t *testing.T) {
	_, r, first, _ := sandboxedForHandBack(t)
	windowShows(r, pane(t, "idle-empty"), true)
	var listed []*deskSandbox
	saved := deskStopListing
	t.Cleanup(func() { deskStopListing = saved })
	deskStopListing = func(s *deskSandbox) { listed = append(listed, s) }

	kept, ok := deskSandboxOf(first.ID)
	if !ok {
		t.Fatal("no sandbox kept")
	}
	if _, err := fireEngineHire(first, 0, r.o, true); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0] != kept {
		t.Fatalf("listed at its stop: %v", listed)
	}
	if _, ok := deskSandboxOf(first.ID); ok {
		t.Error("a fired desk's sandbox is still kept")
	}
	if _, err := os.Lstat(filepath.Dir(first.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a fired desk's socket folder stayed: %v", err)
	}

	// The core restarted: nothing is kept, and its stop makes its sandbox again from its row, without keeping it.
	second, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	keptSandboxes.Lock()
	clear(keptSandboxes.of)
	keptSandboxes.Unlock()
	if _, err := fireEngineHire(second, 0, r.o, true); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("not listed after a restart: %d", len(listed))
	}
	pl := listed[1].places
	if listed[1].policy == nil || pl.OwnID != second.ID || pl.Root != physical(second.Worktree) || pl.Checkout != physical(r.repo) ||
		pl.OwnSockets != physical(filepath.Dir(second.TmuxSocket)) || pl.Home != physical(homeDir) || pl.Subject != listed[0].places.Subject {
		t.Errorf("its sandbox made again: %+v", pl)
	}
	keptSandboxes.Lock()
	left := len(keptSandboxes.of)
	keptSandboxes.Unlock()
	if left != 0 {
		t.Errorf("%d sandboxes kept after the fires", left)
	}
	// A desk whose row records no checked start has nothing to list.
	if rowDeskSandbox("nosuchdesk", r.o) != nil {
		t.Error("a sandbox made for no desk")
	}
}

// For CLAUDEPLANT1's "remove only while the desk is stopped": from its row alone, a sandboxed desk is stopped only
// when its checked server is gone and its socket doesn't answer.
func TestADeskIsKnownStoppedOnlyWhenItsCheckedServerIsGoneAndItsSocketIsSilent(t *testing.T) {
	shortSupport(t)
	deskSockets(t)
	socket := deskSocketPath("ab12", 0)
	const gone = 99999999 // no process has it
	for _, c := range []struct {
		name string
		h    Hire
		want bool
	}{
		{"its server gone, nothing at its socket", Hire{ID: "ab12", TmuxSocket: socket, TmuxServer: gone}, true},
		{"its server running", Hire{ID: "ab12", TmuxSocket: socket, TmuxServer: os.Getpid()}, false},
		{"no checked server", Hire{ID: "ab12", TmuxSocket: socket}, false},
		{"not a sandboxed desk", Hire{ID: "ab12", TmuxSocket: filepath.Join(hireSocketFolder(), "smolcorp-ab12"), TmuxServer: gone}, false},
		{"another desk's socket", Hire{ID: "cd34", TmuxSocket: socket, TmuxServer: gone}, false},
	} {
		if got := deskStoppedNow(c.h); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if err := makeDeskSocketFolder(socket); err != nil {
		t.Fatal(err)
	}
	listenAt(t, socket)
	if deskStoppedNow(Hire{ID: "ab12", TmuxSocket: socket, TmuxServer: gone}) {
		t.Error("a desk whose socket answers is said to be stopped")
	}
}

// RT126SAM6 G1: the launcher's and plain hires' tmux (each plain hire's own server, SA-M10) and windows never get the
// core's inherited guard.
func TestTheLaunchersTmuxNeverGetsTheCoresInheritedGuard(t *testing.T) {
	shortSupport(t)
	t.Setenv("SMOLCORP_TMUX_SERVER", "inherited")
	fake := fakeTmuxScript(t, "launcher")
	saved, savedFolder := tmuxPath, hireSocketFolder
	t.Cleanup(func() { tmuxPath, hireSocketFolder = saved, savedFolder })
	tmuxPath = func() string { return fake }
	hireSocketFolder = func() string { return filepath.Join(supportDir, "t") }
	tmux := plainHireTmux(plainHireTmuxID())
	if tmux == nil || tmux.startOnly == nil {
		t.Fatal("no server of its own for a plain hire")
	}
	// Its session's start is a server start: the guard off for it alone (SA-M7). Any other call is told "unchecked".
	if out, _, err := tmux.startOnly(tmux.command(tmuxServerStart("new-session", "-d")...)); err != nil || out != "off" {
		t.Errorf("the launcher's tmux start got %q (%v)", out, err)
	}
	if out, _, _ := tmux.run(tmux.command("kill-server")); out != tmuxUnchecked {
		t.Errorf("the launcher's tmux call got %q", out)
	}
	if slices.ContainsFunc(hireEnvironment(), func(v string) bool { return strings.HasPrefix(strings.ToUpper(v), "SMOLCORP_TMUX_SERVER=") }) {
		t.Error("a hire's environment carries the core's guard variable")
	}
}

// The probe of RT126SAM6's release gate table, engine row, as a row: with the core's own guard variable planted, a
// sandboxed desk's start, registry save and watch (one that never reports, so its clean-up runs too) run the bundled
// tmux only told the pid the core checked; its start goes to the start-only runner alone.
func TestASandboxedEngineDeskRunsTheBundledTmuxOnlyWithItsCheckedServer(t *testing.T) {
	r := newEngineRig(t)
	deskSockets(t)
	t.Setenv("SMOLCORP_TMUX_SERVER", "inherited")
	program, log, up := guardLoggingTmux(t, pane(t, "idle-empty"))
	saved := bundledTmux
	t.Cleanup(func() { bundledTmux = saved })
	bundledTmux = func() string { return program }
	var started [][]string
	r.o.Tmux = nil
	r.o.Sandbox = func(*deskSandbox) tmuxStarter {
		return func(args []string) (string, int, error) {
			started = append(started, args)
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, err
			}
			staleSocketAt(t, args[1])
			os.WriteFile(up, nil, 0o600)
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if row := registered(t)[0]; row.TmuxServer != 5151 || row.TmuxProgram != recordableTmux(program) || len(started) != 1 {
		t.Fatalf("%+v, %d starts", row, len(started))
	}
	r.o.Within, r.o.Every = time.Millisecond, time.Millisecond
	watchManagedHire(registered(t)[0], 0, r.o)
	if row := registered(t)[0]; row.State != "failed" {
		t.Errorf("a desk that never reported: %+v", row)
	}
	if _, err := os.Lstat(filepath.Dir(hire.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its socket folder stayed: %v", err)
	}
	calls := loggedCalls(t, log)
	if !slices.ContainsFunc(calls, func(c string) bool { return strings.Contains(c, " has-session") }) {
		t.Errorf("its watch didn't look: %q", calls)
	}
	for _, c := range calls {
		if !strings.HasPrefix(c, "5151 ") || strings.Contains(c, " new-session") {
			t.Errorf("the bundled tmux ran without the server the core checked, or started it itself: %q", c)
		}
	}
}
