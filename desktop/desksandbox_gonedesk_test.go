//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// MSYSAM6's rows: where GMEDIT1's stops of a desk whose row says gone meet SA-M6's sandbox. A sandboxed desk stopped
// that way is let go as at every stop (its sandbox, its socket folder), its row stays as it was and off
// CLAUDEPLANT1's record; and one whose start couldn't see its session gone stays reachable by the fire its words ask
// for, through the bundled tmux told only the server the core checked.

// stopListed counts the desks listed at their stop (deskStopListing) for the test.
func stopListed(t *testing.T) *[]*deskSandbox {
	t.Helper()
	listed := &[]*deskSandbox{}
	saved := deskStopListing
	t.Cleanup(func() { deskStopListing = saved })
	deskStopListing = func(s *deskSandbox) { *listed = append(*listed, s) }
	return listed
}

// stillClock keeps the engine's waits off the wall clock: each pause moves it on.
func stillClock(r *engineRig) {
	now := time.Unix(1_700_000_000, 0)
	r.o.Now = func() time.Time { return now }
	r.o.Pause = func(d time.Duration) { now = now.Add(d) }
}

func markedGoneByAnotherProgram(t *testing.T) {
	t.Helper()
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State, f.hires[0].Notes = "fired", "by another program" }); err != nil {
		t.Error(err)
	}
}

// GMEDIT1's fire of a desk whose row says gone, on a sandboxed desk: its session is ended and seen gone, the answer
// is GMEDIT1's, its row stays as it was and off CLAUDEPLANT1's record, and SA-M6 lets its sandbox and socket folder go.
func TestAFireOfASandboxedDeskWhoseRowSaysGoneLetsItsSandboxGo(t *testing.T) {
	_, r, hire, _ := sandboxedForHandBack(t)
	windowShows(r, pane(t, "idle-empty"), true)
	stillClock(r)
	listed := stopListed(t)
	kept, ok := deskSandboxOf(hire.ID)
	if !ok {
		t.Fatal("no sandbox kept")
	}
	markedGoneByAnotherProgram(t)
	gone := registered(t)[0]

	_, err := fireEngineHire(hire, 0, r.o, false)
	refused, isRefusal := err.(*hireRefusal)
	if !isRefusal || refused.Status != http.StatusOK || refused.Message != goneStoppedNote(gone, "tmux session") {
		t.Fatalf("%v", err)
	}
	if r.tmux.up || !r.tmux.ran("kill-server") {
		t.Errorf("its session wasn't ended: %q", r.tmux.asked)
	}
	// SA-M6: listed at its stop with the sandbox its start made, then let go, and its socket folder taken away.
	if len(*listed) != 1 || (*listed)[0] != kept {
		t.Errorf("listed at its stop: %v", *listed)
	}
	if _, ok := deskSandboxOf(hire.ID); ok {
		t.Error("its sandbox is still kept")
	}
	if _, err := os.Lstat(filepath.Dir(hire.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its socket folder stayed: %v", err)
	}
	// GMEDIT1 and CLAUDEPLANT1: its row as the other program left it, and not on record (it was made gone another way).
	if row := registered(t)[0]; row.State != "fired" || row.Notes != "by another program" {
		t.Errorf("its row changed: %+v", row)
	}
	if claudeListed(hire.ID) {
		t.Error("a desk made gone another way was put on record")
	}
}

// GMEDIT1's start that finds its row marked gone once its server is up, on a sandboxed desk that sees its session
// gone: GMEDIT1's answer, ticket and row; and SA-M6's start through its start-only runner alone, its sandbox and
// socket folder let go.
func TestASandboxedStartBehindARowMarkedGoneMeanwhileLetsItsSandboxGo(t *testing.T) {
	_, r, _ := engineDeck(t)
	deskSockets(t)
	listed := stopListed(t)
	var socket string
	starts := 0
	r.o.Sandbox = func(*deskSandbox) tmuxStarter {
		return func(args []string) (string, int, error) {
			starts++
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, err
			}
			staleSocketAt(t, args[1])
			socket, r.tmux.up = args[1], true
			markedGoneByAnotherProgram(t) // another program marks it gone as its server starts
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	request := r.request("janitor")
	request.Nickname = "Ada"
	_, err := runManagedHire(request, r.o)
	refused, ok := err.(*hireRefusal)
	if want := "Ada was marked gone while it was starting, so its tmux session was ended."; !ok || refused.Status != http.StatusConflict || refused.Message != want {
		t.Fatalf("%v", err)
	}
	if starts != 1 || r.tmux.ran("new-session") || !r.tmux.ran("kill-server") || r.tmux.up {
		t.Errorf("%d starts; its server: %q", starts, r.tmux.asked)
	}
	rows := registered(t)
	if len(rows) != 1 || rows[0].State != "fired" || rows[0].Notes != "by another program" {
		t.Errorf("%+v", rows)
	}
	if _, ticket := issuedPlan(rows[0].ID); ticket {
		t.Error("its start ticket is still there")
	}
	if len(*listed) != 1 || (*listed)[0].places.OwnID != rows[0].ID {
		t.Errorf("listed at its stop: %v", *listed)
	}
	if _, ok := deskSandboxOf(rows[0].ID); ok {
		t.Error("its sandbox is still kept")
	}
	if _, err := os.Lstat(filepath.Dir(socket)); socket == "" || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its socket folder stayed: %q %v", socket, err)
	}
}

// heldTmux is guardLoggingTmux's stand-in bundled tmux, whose server won't end while the file hold is there.
func heldTmux(t *testing.T, screen string) (program, log, up, hold string) {
	t.Helper()
	program, log, up = guardLoggingTmux(t, screen)
	hold = filepath.Join(filepath.Dir(filepath.Dir(program)), "hold")
	script, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	held := strings.Replace(string(script), "kill-session|kill-server) rm -f", "kill-session|kill-server) [ -e '"+hold+"' ] || rm -f", 1)
	if held == string(script) {
		t.Fatal("the stand-in's end wasn't found")
	}
	if err := os.WriteFile(program, []byte(held), 0o755); err != nil {
		t.Fatal(err)
	}
	return program, log, up, hold
}

// GMEDIT1's start that can't see its session gone tells the owner to fire it: on a sandboxed desk that fire reaches
// it, through the bundled tmux told only the server the core checked (SA-M6's release gate), since its row records
// that server. The fire then ends it as GMEDIT1's does and lets its sandbox go.
func TestASandboxedStartThatCouldntSeeItsSessionGoneCanStillBeFired(t *testing.T) {
	r := newEngineRig(t)
	deskSockets(t)
	stillClock(r)
	listed := stopListed(t)
	t.Setenv("SMOLCORP_TMUX_SERVER", "inherited")
	program, log, up, hold := heldTmux(t, pane(t, "idle-empty"))
	saved := bundledTmux
	t.Cleanup(func() { bundledTmux = saved })
	bundledTmux = func() string { return program }
	os.WriteFile(hold, nil, 0o600)
	r.o.Tmux = nil
	r.o.Sandbox = func(*deskSandbox) tmuxStarter {
		return func(args []string) (string, int, error) {
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, err
			}
			staleSocketAt(t, args[1])
			os.WriteFile(up, nil, 0o600)
			markedGoneByAnotherProgram(t)
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	request := r.request("janitor")
	request.Nickname = "Ada"
	_, err := runManagedHire(request, r.o)
	refused, ok := err.(*hireRefusal)
	if want := "Ada was marked gone while it was starting, so Smolcorp ended its tmux session but couldn't see it gone: fire Ada to end it."; !ok || refused.Status != http.StatusConflict || refused.Message != want {
		t.Fatalf("%v", err)
	}
	row := registered(t)[0]
	if row.State != "fired" || row.Notes != "by another program" || row.TmuxServer != 5151 || row.TmuxSocket != deskSocketPath(row.ID, 0) {
		t.Fatalf("its row: %+v", row)
	}
	if _, kept := deskSandboxOf(row.ID); !kept || len(*listed) != 0 {
		t.Errorf("a desk that may still run was let go: kept %v, listed %d", kept, len(*listed))
	}

	os.Remove(hold)
	_, err = fireEngineHire(row, 0, r.o, false)
	refused, ok = err.(*hireRefusal)
	if !ok || refused.Status != http.StatusOK || refused.Message != goneStoppedNote(row, "tmux session") {
		t.Fatalf("its fire: %v", err)
	}
	if _, err := os.Lstat(up); !errors.Is(err, os.ErrNotExist) {
		t.Error("its session wasn't ended")
	}
	if _, kept := deskSandboxOf(row.ID); kept || len(*listed) != 1 {
		t.Errorf("not let go at its fire: kept %v, listed %d", kept, len(*listed))
	}
	if _, err := os.Lstat(filepath.Dir(row.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its socket folder stayed: %v", err)
	}
	calls := loggedCalls(t, log)
	if !slices.ContainsFunc(calls, func(c string) bool { return strings.Contains(c, " kill-server") }) {
		t.Errorf("never ended: %q", calls)
	}
	for _, c := range calls {
		if !strings.HasPrefix(c, "5151 ") || strings.Contains(c, " new-session") {
			t.Errorf("the bundled tmux ran without the server the core checked, or started it itself: %q", c)
		}
	}
}

// sandboxedRig is an engine rig whose desks start in their sandbox through a stand-in start-only runner (its socket
// folder made new, a server checked at pid 5151).
func sandboxedRig(t *testing.T) *engineRig {
	t.Helper()
	_, r, _ := engineDeck(t)
	deskSockets(t)
	r.o.Sandbox = func(*deskSandbox) tmuxStarter {
		return func(args []string) (string, int, error) {
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, err
			}
			staleSocketAt(t, args[1])
			r.tmux.up = true
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	return r
}

// Where the merge put CLAUDEPLANT1's and SA-M6's calls side by side, each holds on its own: the engine's fire of a
// sandboxed desk puts it on CLAUDEPLANT1's record (claudeHireEnded) and lets its sandbox go (deskStopped). Each
// side's checks are its own, so a mutant that drops one call fails that side's lines alone.
func TestAFireOfASandboxedDeskPutsItOnRecordAndLetsItsSandboxGo(t *testing.T) {
	r := sandboxedRig(t)
	stillClock(r)
	listed := stopListed(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	windowShows(r, pane(t, "idle-empty"), true)
	if _, err := fireEngineHire(hire, 0, r.o, true); err != nil {
		t.Fatal(err)
	}
	if row := registered(t)[0]; row.State != "fired" || !claudeListed(hire.ID) {
		t.Errorf("CLAUDEPLANT1: fired, its session seen gone, not on record: %+v", row)
	}
	if _, kept := deskSandboxOf(hire.ID); kept || len(*listed) != 1 {
		t.Errorf("SA-M6: not let go at its fire: kept %v, listed %d", kept, len(*listed))
	}
	if _, err := os.Lstat(filepath.Dir(hire.TmuxSocket)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("SA-M6: its socket folder stayed: %v", err)
	}
}

// The same at a start whose window didn't open: its session ended, it's failed and on CLAUDEPLANT1's record, and
// SA-M6 lets its sandbox and socket folder go.
func TestASandboxedStartWhoseWindowDidntOpenIsOnRecordAndLetGo(t *testing.T) {
	r := sandboxedRig(t)
	listed := stopListed(t)
	var socket string
	r.o.Attach = func(h Hire) error { socket = h.TmuxSocket; return errors.New("no terminal") }
	_, err := runManagedHire(r.request("janitor"), r.o)
	if refused, ok := err.(*hireRefusal); !ok || refused.Message != "The terminal didn't open." {
		t.Fatalf("%v", err)
	}
	row := registered(t)[0]
	if row.State != "failed" || !claudeListed(row.ID) {
		t.Errorf("CLAUDEPLANT1: failed, its session ended, not on record: %+v", row)
	}
	if _, kept := deskSandboxOf(row.ID); kept || len(*listed) != 1 {
		t.Errorf("SA-M6: not let go: kept %v, listed %d", kept, len(*listed))
	}
	if _, err := os.Lstat(filepath.Dir(socket)); socket == "" || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("SA-M6: its socket folder stayed: %q %v", socket, err)
	}
}
