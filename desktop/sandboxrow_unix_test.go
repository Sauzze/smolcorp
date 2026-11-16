//go:build !windows

package desktop

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// SA-X14 on the engine: "Restart in the sandbox" (AQ17), the desks it's offered for, and what M12's start checks
// say in the row.

// oldDeskRig is a deck whose desk Janitor started while the sandbox was off (as one from before 1.26 runs), on a tmux
// server of its own, idle with nothing typed; the sandbox is on again. made is each sandbox made since.
func oldDeskRig(t *testing.T) (d *Deck, r *engineRig, old Hire, made *[]*deskSandbox) {
	t.Helper()
	playSandboxedHere(t, true)
	freshChecks(t)
	d, r, _ = engineDeck(t)
	deskSockets(t)
	made = &[]*deskSandbox{}
	r.o.Sandbox = func(s *deskSandbox) tmuxStarter {
		*made = append(*made, s)
		return func(args []string) (string, int, error) {
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, errors.New(takenPlace)
			}
			staleSocketAt(t, args[1])
			r.tmux.up = true
			return "%1 4242 " + args[1], 5151, nil
		}
	}
	r.o.SandboxOff = true
	old, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	r.o.SandboxOff = false
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine.Sandbox, d.engine.Tmux = r.o.Sandbox, r.o.Tmux
	d.config.Roles = append(d.config.Roles, r.request("janitor").Role)
	if len(*made) != 0 || old.Sandboxed {
		t.Fatalf("the old desk started in a sandbox: %+v", old)
	}
	return d, r, old, made
}

// The button ends an old desk's Claude Code (idle, nothing typed) and starts it again, fresh, in its sandbox, in the
// same worktree and on the same branch: the same row, tagged in its sandbox with its checked server, its own Claude
// folder, and a first message that says where it is. Then it's offered no more.
func TestRestartInTheSandboxStartsAnOldDeskAgainInItsPlace(t *testing.T) {
	d, r, old, made := oldDeskRig(t)
	freshGitPlants(t)
	// A hook the old desk could have left in the repo's shared .git, outside any sandbox: the restart's start finds
	// it, and the row shows it (RT126SAX14 B2).
	planted := filepath.Join(r.repo, ".git", "hooks", "post-commit")
	if err := os.WriteFile(planted, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if answer := sandboxAnswer(t, d); answer["unsandboxed"] != float64(1) || !offered(answer, old.ID) {
		t.Fatalf("before: %v", answer)
	}
	status, answer := d.restartInSandbox(old.ID)
	if status != http.StatusOK {
		t.Fatalf("%d %v", status, answer)
	}
	if !r.tmux.ran("kill-session") {
		t.Errorf("its old session wasn't ended: %q", r.tmux.asked)
	}
	rows := registered(t)
	row := rows[0]
	if len(rows) != 1 || row.ID != old.ID || !row.Sandboxed || !inSandbox(row) || row.TmuxServer != 5151 ||
		row.TmuxSocket != deskSocketPath(old.ID, 0) || row.Worktree != old.Worktree || row.Branch != old.Branch {
		t.Fatalf("the row after: %+v", rows)
	}
	if len(*made) != 1 || (*made)[0].places.Root != physical(old.Worktree) || (*made)[0].places.OwnID != old.ID {
		t.Fatalf("its sandbox: %d made", len(*made))
	}
	if _, err := os.Stat(old.Worktree); err != nil {
		t.Errorf("its worktree: %v", err)
	}
	if info, err := os.Stat(deskClaudeFolder(supportDir, old.ID)); err != nil || !info.IsDir() {
		t.Errorf("its own Claude folder: %v", err)
	}
	if script := r.script(row); !strings.Contains(script, "started you again, this time in its sandbox") {
		t.Errorf("its first message:\n%s", script)
	}
	// Its guard is told its kept copy as the engine's start tells it, as Hand back's is (RT126SAL2 S1, the release
	// gate's item 4b): on Linux only.
	pre, post := guardWordsIn(t, filepath.Join(hireFolder(supportDir, old.ID), hireSettingsFile))
	if kept := keptForThisSystem(old.ID); (kept != "") != slices.Contains(pre, "--held") ||
		(kept != "") != slices.Contains(post["PostToolUse"], kept) || (kept != "") != slices.Contains(post["PostToolUseFailure"], kept) {
		t.Errorf("the restart's guard words: before %q, after %q; kept copy %q", pre, post, kept)
	}
	if answer := sandboxAnswer(t, d); answer["unsandboxed"] != float64(0) || offered(answer, old.ID) {
		t.Errorf("after: %v", answer)
	}
	if row := rowPlants(t, d)[physical(planted)+" "]; row == nil || row["why"] != whyHook || row["removable"] != true {
		t.Errorf("the planted hook isn't shown: %v", rowPlants(t, d))
	}
	if _, err := os.Stat(planted); err != nil {
		t.Errorf("Smolcorp removed a plant by itself: %v", err)
	}
	if status, why := d.restartInSandbox(old.ID); status != http.StatusConflict || why != "Janitor runs in its sandbox already." {
		t.Errorf("again: %d %v", status, why)
	}
}

func offered(answer map[string]any, id string) bool {
	desks, _ := answer["desks"].([]any)
	for _, desk := range desks {
		if row := desk.(map[string]any); row["id"] == id {
			return row["restart"] == true
		}
	}
	return false
}

// A busy desk isn't stopped: nothing is changed, and the words say restart, not take over. With the sandbox off, the
// button isn't for anything, and nothing is stopped.
func TestABusyDeskOrTheSandboxOffChangesNothing(t *testing.T) {
	d, r, old, made := oldDeskRig(t)
	windowShows(r, pane(t, "busy-1"), true)
	d.engine.Tmux = r.o.Tmux
	status, why := d.restartInSandbox(old.ID)
	if status != http.StatusConflict || !strings.Contains(why.(string), "restart it once it's done") || strings.Contains(why.(string), "take it over") {
		t.Errorf("busy: %d %v", status, why)
	}
	if r.tmux.ran("kill-session") || len(*made) != 0 || registered(t)[0].Sandboxed {
		t.Errorf("a busy desk was touched: %q", r.tmux.asked)
	}
	d.mu.Lock()
	d.config.SandboxOff = true
	d.mu.Unlock()
	windowShows(r, pane(t, "idle-empty"), true)
	d.engine.Tmux = r.o.Tmux
	if status, why := d.restartInSandbox(old.ID); status != http.StatusConflict || why != sandboxOffToRestart || r.tmux.ran("kill-session") {
		t.Errorf("the sandbox off: %d %v", status, why)
	}
}

// While a restart runs, the desk is held (RT126SAX14 S1): a fire, a Take over and a second restart are refused, each
// told it's a restart, and nothing of the restart is disturbed. Once it's done, nothing holds the desk.
func TestADeskBeingRestartedIsHeldFromAFireATakeOverAndASecondRestart(t *testing.T) {
	d, r, old, _ := oldDeskRig(t)
	starter := d.engine.Sandbox
	var during []string
	d.engine.Sandbox = func(s *deskSandbox) tmuxStarter {
		start := starter(s)
		return func(args []string) (string, int, error) {
			status, why := d.restartInSandbox(old.ID)
			during = append(during, fmt.Sprint(status, " ", why))
			_, err := fireEngineHire(old, 0, r.o, true)
			during = append(during, fmt.Sprint(err))
			firing.Lock()
			during = append(during, fmt.Sprint("take over finds it held: ", firing.ids[old.ID] && firing.restarting[old.ID]))
			firing.Unlock()
			return start(args)
		}
	}
	if status, answer := d.restartInSandbox(old.ID); status != http.StatusOK {
		t.Fatalf("the restart: %d %v", status, answer)
	}
	want := []string{"409 Janitor is being restarted in its sandbox already.",
		"Janitor is being restarted in its sandbox right now, so it wasn't fired: try again once that's done.",
		"take over finds it held: true"}
	if !slices.Equal(during, want) {
		t.Errorf("during the restart:\n%q\nwant\n%q", during, want)
	}
	if rows := registered(t); len(rows) != 1 || !isOpen(rows[0]) || !inSandbox(rows[0]) {
		t.Errorf("the row after: %+v", rows)
	}
	firing.Lock()
	held := firing.ids[old.ID] || firing.restarting[old.ID]
	firing.Unlock()
	if held {
		t.Error("still held once the restart was done")
	}
}

// While the records' key can't be read, a restart is refused before anything is stopped: its new keys couldn't be
// saved, so the desk would be stopped and not started again (RT126SAX14 nit 2).
func TestWhileTheRecordsAreHeldARestartStopsNothing(t *testing.T) {
	d, r, old, made := oldDeskRig(t)
	records.mu.Lock()
	was := records.state
	records.state = recordsUnreadable
	records.mu.Unlock()
	t.Cleanup(func() { records.mu.Lock(); records.state = was; records.mu.Unlock() })
	if status, why := d.restartInSandbox(old.ID); status != http.StatusConflict || why != recordsHeldAnswer {
		t.Errorf("%d %v", status, why)
	}
	if r.tmux.ran("kill-session") || len(*made) != 0 || registered(t)[0].Sandboxed {
		t.Errorf("something was stopped or started: %q", r.tmux.asked)
	}
}

// Stopped, then refused at its start: said so, its socket folder taken away, its row left for Restart again or Fire.
func TestADeskStoppedButNotStartedAgainIsSaidSoAndKeepsItsRow(t *testing.T) {
	d, r, old, _ := oldDeskRig(t)
	starter := d.engine.Sandbox
	d.engine.Sandbox = func(s *deskSandbox) tmuxStarter {
		starter(s)
		return func(args []string) (string, int, error) {
			if err := makeDeskSocketFolder(args[1]); err != nil {
				return "", 0, err
			}
			removeDeskSocketFolder(args[1])
			return "", 0, errors.New(takenPlace)
		}
	}
	status, why := d.restartInSandbox(old.ID)
	if status != http.StatusInternalServerError || !strings.Contains(why.(string), "Janitor was stopped, and couldn't start again: "+takenPlace) ||
		!strings.Contains(why.(string), "press Restart in the sandbox again, or fire it") {
		t.Errorf("%d %v", status, why)
	}
	if rows := registered(t); len(rows) != 1 || !isOpen(rows[0]) || rows[0].Sandboxed {
		t.Errorf("its row: %+v", rows)
	}
	if _, err := os.Lstat(filepath.Dir(deskSocketPath(old.ID, 0))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("its socket folder stayed: %v", err)
	}
	if !r.tmux.ran("kill-session") {
		t.Error("not stopped")
	}
}

// Who the button is for, and what each other desk is told to do instead.
func TestRestartIsOfferedOnlyWhereItCanStartTheDeskInItsSandbox(t *testing.T) {
	playSandboxedHere(t, true)
	deskSockets(t)
	roles := []Role{{ID: "janitor", Name: "Janitor", Agent: "claude", Model: "sonnet"}, {ID: "coder", Name: "Coder", Agent: "codex"}}
	saved := []claudeProfile{{Name: "work", Dir: filepath.Join(homeDir, ".claude-work")}}
	own := func(h Hire) Hire { h.TmuxSocket = hireOwnSocket(h.ID, 0); return h }
	base := own(Hire{ID: "aaaa00000001", Name: "Janitor", Role: "janitor", State: "working", Model: "opus"})
	with := func(change func(*Hire)) Hire { h := base; change(&h); return h }
	inside := with(func(h *Hire) { h.Sandboxed, h.TmuxSocket = true, deskSocketPath(h.ID, 0) })
	cases := []struct {
		name  string
		h     Hire
		off   bool
		apply bool
		want  string
	}{
		{"an old desk", base, false, false, ""},
		{"the sandbox off", base, true, false, sandboxOffToRestart},
		{"in its sandbox", inside, false, false, "runs in its sandbox already"},
		{"in its sandbox, to apply a change", inside, false, true, ""},
		{"taken over", with(func(h *Hire) { h.T3Session = "s" }), false, false, "back from T3 Code"},
		{"in T3 Code", with(func(h *Hire) { h.Engine = t3Engine }), false, false, "runs in T3 Code"},
		{"the GM", with(func(h *Hire) { h.Role = gmRole }), false, false, "Your General Manager isn't restarted here"},
		{"in a window", with(func(h *Hire) { h.TmuxSocket = "" }), false, false, "a tmux of Smolcorp's own"},
		{"another desk's socket", with(func(h *Hire) { h.TmuxSocket = hireOwnSocket("bbbb00000002", 0) }), false, false, "a tmux of Smolcorp's own"},
		{"the API key", with(func(h *Hire) { h.Account = apiKeyAccountName }), false, false, "API key"},
		{"Codex", with(func(h *Hire) { h.Role = "coder" }), false, false, "isn't a Claude Code desk"},
		{"an account gone", with(func(h *Hire) { h.Account = "gone" }), false, false, "isn't in Setup's list any more"},
	}
	for _, c := range cases {
		role, _, why := restartRefusal(c.h, roles, saved, c.off, c.apply)
		if c.want == "" && (why != "" || role.ID != c.h.Role || role.Model != "opus") || c.want != "" && !strings.Contains(why, c.want) {
			t.Errorf("%s: %q, role %+v", c.name, why, role)
		}
	}
	if err := os.MkdirAll(saved[0].Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(saved[0].Dir) })
	if _, account, why := restartRefusal(with(func(h *Hire) { h.Account = "work" }), roles, saved, false, false); why != "" || filepath.Base(account) != ".claude-work" {
		t.Errorf("on a saved account: %q %q", account, why)
	}
	playSandboxedHere(t, false)
	if _, _, why := restartRefusal(base, roles, saved, false, false); why != noSandboxToRestart {
		t.Errorf("off a Mac: %q", why)
	}
}

// Remote Login as the last desk start found it, in the row; a desk started before it answered says Restart to apply,
// and is offered the button; Start here counts it.
func TestRemoteLoginAndRestartToApplyAreShownInTheRow(t *testing.T) {
	playSandboxedHere(t, true)
	freshChecks(t)
	deskSockets(t)
	d := newTestDeck(t)
	in := Hire{ID: "in0000000001", Name: "Ada", Role: "janitor", Sandboxed: true, State: "working", TmuxSocket: deskSocketPath("in0000000001", 0)}
	writeRegistryRows(t, in)
	if answer := sandboxAnswer(t, d); answer["remoteLogin"] != "" || answer["unsandboxed"] != float64(0) {
		t.Errorf("before any start: %v", answer)
	}
	deskStartChecks(in.ID, false) // nothing on the stand-in port 22
	if answer := sandboxAnswer(t, d); answer["remoteLogin"] != "off" || offered(answer, in.ID) {
		t.Errorf("Remote Login off: %v", answer)
	}
	saved := remoteLoginPort
	t.Cleanup(func() { remoteLoginPort = saved })
	remoteLoginPort = checkStandIn(t, func(net.Conn) {})
	answer := sandboxAnswer(t, d)
	desk := answer["desks"].([]any)[0].(map[string]any)
	words := "Remote Login was turned on after this desk started"
	if runtime.GOOS != "darwin" {
		words = "an SSH server answers on this computer now"
	}
	if answer["remoteLogin"] != "on" || !strings.Contains(desk["apply"].(string), words) || desk["restart"] != true {
		t.Errorf("Remote Login on: %v", answer)
	}
	for _, c := range d.selfTest() {
		if c.ID == "sandbox" && (c.Status != checkTodo || !strings.HasPrefix(c.Detail, "1 desk started before a change")) {
			t.Errorf("Start here: %+v", c)
		}
	}
}

// The start checks reach the desk's sandbox: Remote Login answering denies its SSH keys (C5), and the databases stay
// denied unless the opt-in passed at its start (C4); its stop forgets them.
func TestADesksStartChecksGoIntoItsSandbox(t *testing.T) {
	r := newSandboxRig(t)
	freshChecks(t)
	saved := remoteLoginPort
	t.Cleanup(func() { remoteLoginPort = saved })
	remoteLoginPort = checkStandIn(t, func(net.Conn) {})
	r.o.DatabaseOptIn = true // with no database place to try, the check passes
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if !r.desks[0].places.RemoteLogin || !deskCheckedOf(hire.ID).RemoteLogin || !deskCheckedOf(hire.ID).Database {
		t.Errorf("its start checks: %+v, %+v", r.desks[0].places, deskCheckedOf(hire.ID))
	}
	deskStopped(hire, r.o)
	if c := deskCheckedOf(hire.ID); c.RemoteLogin || c.Database {
		t.Errorf("kept after its stop: %+v", c)
	}
}

// The GM is told in its introduction when it starts with the sandbox off (§A5.2), on a Mac.
func TestTheGMIsToldWhenTheSandboxIsOff(t *testing.T) {
	r := newSandboxRig(t)
	for _, off := range []bool{false, true} {
		r.o.SandboxOff = off
		r.tmux.up = false
		gm, err := runManagedHire(r.request(gmRole), r.o)
		if err != nil {
			t.Fatal(err)
		}
		if told := strings.Contains(r.script(gm), "sandbox is off: the owner turned it off in Setup"); told != (off && deskSandboxed) {
			t.Errorf("off %v: told %v", off, told)
		}
		forgetHire(gm.ID)
	}
}
