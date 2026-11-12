//go:build !windows

package desktop

import (
	"bytes"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The engine's tests run on a short temporary support folder, temporary repos
// and homes, a stand-in tmux and a stand-in guard: no real claude, no real tmux
// server, no real git config or hooks.

const engineGuardPrinted = `{"groups":[{"name":"push","on":true,"commands":["git push"]}],"file":"x","problem":""}`

// fakeTmux answers tmux as a hire's server would, and notes what it was asked.
type fakeTmux struct {
	t      *testing.T
	asked  [][]string
	up     bool // whether the session is there
	starts bool // whether new-session works
}

func (f *fakeTmux) run(args []string) (string, int, bool) {
	f.asked = append(f.asked, args)
	switch args[2] {
	case "new-session":
		if !f.starts {
			return "", 1, true
		}
		f.up = true
		// A real server's socket is a socket file, which cleanup checks for.
		listenAt(f.t, args[1])
		return "%1 " + args[1], 0, true
	case "has-session":
		if f.up {
			return "", 0, true
		}
		return "", 1, true
	case "kill-session", "kill-server":
		f.up = false
	}
	return "", 0, true
}

func (f *fakeTmux) ran(verb string) bool {
	return slices.ContainsFunc(f.asked, func(a []string) bool { return a[2] == verb })
}

type engineRig struct {
	t    *testing.T
	home string
	repo string
	tmux *fakeTmux
	o    engineOptions
}

func newEngineRig(t *testing.T) *engineRig {
	t.Helper()
	shortSupport(t)
	home := homeDir
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "no-gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "no-gitconfig"))
	writeConfig(t, home, "{}")
	tmux := &fakeTmux{t: t, starts: true}
	return &engineRig{t: t, home: home, repo: placeRepo(t), tmux: tmux, o: engineOptions{
		Worktrees: t.TempDir(), PortFree: func(int) bool { return true }, Key: func() string { return placeTestKey },
		Guard:  func(string, []string) (int, string, bool) { return 0, engineGuardPrinted, true },
		IsExec: func(string) bool { return true }, CLI: filepath.Join(home, "bin", "smolcorp"), Tmux: tmux.run,
	}}
}

func (r *engineRig) request(roleID string) engineRequest {
	role := Role{ID: roleID, Name: strings.ToUpper(roleID[:1]) + roleID[1:], Agent: "claude", Model: "sonnet"}
	return engineRequest{Role: role, How: hiredIntoGeneralFolder, Folder: r.repo, DefaultAccount: true,
		GMToken: func() string { return "fake-gm-token-1" }}
}

// script is the start script a hire was given.
func (r *engineRig) script(h Hire) string {
	r.t.Helper()
	files, _ := filepath.Glob(filepath.Join(hireFolder(supportDir, h.ID), "hire-*.sh"))
	if len(files) != 1 {
		r.t.Fatalf("start scripts: %v", files)
	}
	data, _ := os.ReadFile(files[0])
	return string(data)
}

func registered(t *testing.T) []Hire { t.Helper(); return readRegistry(registryPath(), false).hires }

func TestAManagedHireIsMadeInOrderAndRegistered(t *testing.T) {
	r := newEngineRig(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if hire.State != "starting" || hire.Role != "janitor" || hire.Ports != "4100-4103" || hire.Repo != r.repo ||
		hire.Worktree == "" || !strings.HasPrefix(hire.TmuxSocket, supportDir) {
		t.Fatalf("hire = %+v", hire)
	}
	if got := registered(t); len(got) != 1 || got[0].ID != hire.ID || got[0].State != "starting" {
		t.Fatalf("registry = %+v", got)
	}
	if _, err := os.Stat(filepath.Join(hire.Worktree, "sub", "a.txt")); err != nil {
		t.Errorf("no worktree checkout: %v", err)
	}
	settings, _ := os.ReadFile(filepath.Join(hireFolder(supportDir, hire.ID), hireSettingsFile))
	if !strings.Contains(string(settings), "PreToolUse") {
		t.Errorf("no guard hook in the hire's settings: %s", settings)
	}
	script := r.script(hire)
	// Its own send key, in its script, and only the key's hash in the registry, against its row.
	if key := scriptSendKey(script); key == "" {
		t.Errorf("start script carries no send key:\n%s", script)
	} else if h := readRegistry(registryPath(), false).hireForSendKey(key); h == nil || h.ID != hire.ID {
		t.Errorf("the script's key isn't the hire's in the registry: %+v", h)
	}
	for _, want := range []string{"--settings=" + filepath.Join(hireFolder(supportDir, hire.ID), hireSettingsFile), "unset SMOLCORP_GM_TOKEN", "unset CLAUDE_CONFIG_DIR", "SMOLCORP_HIRE_PORTS='4100-4103'"} {
		if !strings.Contains(script, want) {
			t.Errorf("start script lacks %q:\n%s", want, script)
		}
	}
	// One tmux call, on the hire's own socket, in the worktree; the registry's folder is the project (C3 A5).
	if len(r.tmux.asked) != 1 || r.tmux.asked[0][0] != "-S" || r.tmux.asked[0][1] != hire.TmuxSocket || !slices.Contains(r.tmux.asked[0], hire.Worktree) ||
		hire.Folder != r.repo || startedIn(hire) != hire.Worktree {
		t.Errorf("tmux was asked %q", r.tmux.asked)
	}
	// The next hire gets the next ports.
	if next, err := runManagedHire(r.request("janitor"), r.o); err != nil || next.Ports != "4104-4107" {
		t.Errorf("second hire: %+v %v", next, err)
	}
}

func TestAnAPIKeyHireThroughTheEngineAlwaysApprovesItsKeyTail(t *testing.T) {
	r := newEngineRig(t)
	request := r.request("janitor")
	request.APIKey = true
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(r.home, ".claude.json"))
	if !isApproved("0123456789ABCDEFGHIJ", data) || !isTrusted(hire.Worktree, data) {
		t.Fatalf("the API-key hire's key tail or worktree isn't in .claude.json: %s", data)
	}
	if script := r.script(hire); !strings.Contains(script, "secret-tool lookup") || strings.Contains(script, placeTestKey) {
		t.Errorf("the key should be read from the secret store, never written down:\n%s", script)
	}
	// With no key in the store nothing is made.
	r2 := newEngineRig(t)
	r2.o.Key = func() string { return "" }
	request = r2.request("janitor")
	request.APIKey = true
	if _, err := runManagedHire(request, r2.o); err != errHireNoKey || branches(t, r2.repo) != "" || len(r2.tmux.asked) != 0 {
		t.Errorf("no key: %v, branches %q, tmux %q", err, branches(t, r2.repo), r2.tmux.asked)
	}
}

func TestWithNoClaudeJSONNothingIsMadeAndTheHireIsRefused(t *testing.T) {
	r := newEngineRig(t)
	os.Remove(filepath.Join(r.home, ".claude.json"))
	if _, err := runManagedHire(r.request("janitor"), r.o); err != errHireNoConfig {
		t.Fatalf("err = %v, want errHireNoConfig", err)
	}
	if _, err := os.Stat(filepath.Join(r.home, ".claude.json")); err == nil {
		t.Error("a .claude.json was made")
	}
	if branches(t, r.repo) != "" || len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
		t.Error("something was made for a refused hire")
	}
}

func TestWhenPreTrustRefusesTheEngineMakesNothing(t *testing.T) {
	r := newEngineRig(t)
	file := filepath.Join(r.home, ".claude.json")
	// The config turns into a shape that can't be edited just as the worktree is made.
	r.o.Git = func(dir string, args ...string) (int, string, bool) {
		status, out, ok := runGit(dir, args...)
		if len(args) > 1 && args[1] == "add" {
			os.WriteFile(file, []byte("[]"), 0o600)
		}
		return status, out, ok
	}
	if _, err := runManagedHire(r.request("janitor"), r.o); err != errHireTrust {
		t.Fatalf("err = %v, want errHireTrust", err)
	}
	if branches(t, r.repo) != "" || len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
		t.Error("a worktree, session or registry row was left")
	}
}

func TestAFailedStepTakesAwayWhatTheStepsBeforeItMade(t *testing.T) {
	r := newEngineRig(t)
	r.tmux.starts = false
	_, err := runManagedHire(r.request("janitor"), r.o)
	if refused, ok := err.(*hireRefusal); !ok || refused.Status != http.StatusInternalServerError {
		t.Fatalf("err = %v", err)
	}
	if branches(t, r.repo) != "" || len(registered(t)) != 0 || len(readRegistry(registryPath(), false).keys) != 0 {
		t.Errorf("left behind: %q %v, keys %v", branches(t, r.repo), registered(t), readRegistry(registryPath(), false).keys)
	}
	if entries, _ := os.ReadDir(filepath.Join(supportDir, "hires")); len(entries) != 0 {
		t.Errorf("the hire's own folder was left: %v", entries)
	}
	// A guard that can't be run refuses before the session.
	r = newEngineRig(t)
	r.o.Guard = func(string, []string) (int, string, bool) { return 1, "", false }
	if _, err := runManagedHire(r.request("janitor"), r.o); err == nil || len(r.tmux.asked) != 0 || branches(t, r.repo) != "" {
		t.Errorf("an unguarded hire: %v, tmux %q", err, r.tmux.asked)
	}
}

func TestATestCopyNeverHiresBeforeAnythingIsMade(t *testing.T) {
	r := newEngineRig(t)
	for _, account := range []string{"", "/some/account"} {
		request := r.request("janitor")
		request.TestPort, request.ConfigDir, request.DefaultAccount = 7799, account, account == ""
		if _, err := runManagedHire(request, r.o); err == nil {
			t.Fatalf("a test copy hired on account %q", account)
		}
	}
	if branches(t, r.repo) != "" || len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
		t.Error("something was made for a test copy's refused hire")
	}
	if entries, _ := os.ReadDir(supportDir); len(entries) > 1 { // .claude.json is in the home, not here
		t.Errorf("files were made in the support folder: %v", entries)
	}
}

func TestAHireThatNeverReportsIsCleanedUp(t *testing.T) {
	r := newEngineRig(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	r.o.Now = func() time.Time { return now }
	r.o.Pause = func(d time.Duration) { now = now.Add(d) }
	recordHireWindow(hire.ID, "tty:/dev/ttys004#w1234") // its window, as the Mac's attach would record it
	t.Cleanup(func() { forgetHireWindows(hire.ID, windowsOfHire(hire.ID)) })
	watchManagedHire(hire, 0, r.o)
	if !r.tmux.ran("kill-session") || r.tmux.up {
		t.Error("its session wasn't ended")
	}
	if windows := windowsOfHire(hire.ID); len(windows) != 0 {
		t.Errorf("its windows' records were kept: %q", windows)
	}
	if branches(t, r.repo) != "" {
		t.Error("its clean worktree and branch weren't removed")
	}
	if left, _ := filepath.Glob(filepath.Join(hireFolder(supportDir, hire.ID), "hire-*.sh")); len(left) != 0 {
		t.Errorf("its start script, with its send key, was left: %v", left)
	}
	got := registered(t)
	if len(got) != 1 || got[0].State != "failed" || !strings.Contains(got[0].Notes, "Didn't start: no word from it in 45 seconds.") {
		t.Errorf("registry = %+v", got)
	}
}

func TestAHireThatReportsIsWorking(t *testing.T) {
	r := newEngineRig(t)
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	r.o.HeardFrom = func(Hire) bool { return true }
	watchManagedHire(hire, 0, r.o)
	if got := registered(t); got[0].State != "working" || r.tmux.ran("kill-session") || branches(t, r.repo) == "" {
		t.Errorf("registry %+v, killed %v", got, r.tmux.ran("kill-session"))
	}
}

// engineDeck is a deck set up to hire a role in tmux, through the engine's stand-ins.
func engineDeck(t *testing.T) (*Deck, *engineRig, *hiring) {
	t.Helper()
	h := fakeHiring(t, "/usr/bin/tmux")
	r := newEngineRig(t)
	// As newTestDeck starts one, but on the rig's own support folder and home, every stand-in in first (see startDeck).
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	d.config.HiringEnabled, d.config.HireInTmux, d.config.HireFolder = true, true, r.repo
	r.o.Worktrees = ""
	d.engine = r.o
	d.engine.Within, d.engine.Every = 20*time.Millisecond, 5*time.Millisecond
	return d, r, h
}

func TestARoleHireInTmuxGoesThroughTheEngine(t *testing.T) {
	d, r, h := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	if len(h.tmux) != 0 || len(h.windows) == 0 {
		// the old path's tmux (runTmux) wasn't used; the window attached to the hire's session
		t.Errorf("old tmux calls %q, windows %q", h.tmux, h.windows)
	}
	got := registered(t)
	if len(got) != 1 || got[0].State != "working" || got[0].Role != "janitor" || !r.tmux.ran("new-session") {
		t.Fatalf("registry %+v, tmux %q", got, r.tmux.asked)
	}
	attach := h.windows[0]
	if !slices.Contains(attach, "attach") || !slices.Contains(attach, got[0].TmuxSocket) {
		t.Errorf("the window runs %q, want an attach on the hire's socket", attach)
	}
}

func TestOtherHiresOpenAsTheyDid(t *testing.T) {
	// Anyone (no role), another agent, and no tmux: none goes through the engine
	// (a held or pinned desk is TestAHireAtAHeldOrPinnedDeskKeepsTheOldPath).
	d, r, h := engineDeck(t)
	d.config.HireAgent = "codex"
	for _, c := range []struct {
		name  string
		query url.Values
		setup func()
	}{
		{"no role", url.Values{}, func() {}},
		{"another agent", url.Values{"role": {"accountant"}}, func() {
			d.config.Roles[slices.IndexFunc(d.config.Roles, func(x Role) bool { return x.ID == "accountant" })].Agent = "codex"
		}},
		{"no tmux", url.Values{"role": {"janitor"}}, func() { tmuxPath = func() string { return "" } }},
	} {
		d.lastHire = time.Time{}
		c.setup()
		before := len(h.windows) + len(h.tmux)
		status, message := d.Perform("hire", "", c.query)
		if len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
			t.Errorf("%s: went through the engine (%d %s)", c.name, status, message)
		}
		if c.name != "no tmux" && status == http.StatusNoContent && len(h.windows)+len(h.tmux) == before {
			t.Errorf("%s: opened nothing", c.name)
		}
	}
}

// A role hire in tmux keeps its role label on the desk and to the phone, as a
// launcher's hire does, though its session has no launcher: its registry row
// has the role, and it is found by the hire's own socket in the session's tab.
func TestARoleHireInTmuxKeepsItsRoleLabel(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	hire := registered(t)[0]
	apply := func(id, tab string) *Session {
		d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": hire.Folder}, Claude, tab, "", 100)
		return d.sessions.Get(id)
	}
	s := apply("s1", "tmux:%1@"+hire.TmuxSocket)
	if role := d.roleOf(s); role == nil || role["id"] != "janitor" {
		t.Fatalf("the hire's role = %v, want janitor", role)
	}
	// It stays across a restart of the deck: the registry is where it is kept.
	if d.hires = nil; d.roleOf(s) == nil {
		t.Error("the role label was lost")
	}
	// No other tmux session gets a role: not another socket, not one outside the support folder.
	for _, tab := range []string{"tmux:%1@" + filepath.Join(supportDir, "smolcorp-ffffffffffff"), "tmux:%1@/tmp/tmux-1000/default", "tmux:%1"} {
		if role := d.roleOf(apply("s-"+tab[len(tab)-4:], tab)); role != nil {
			t.Errorf("tab %q was given the role %v", tab, role)
		}
	}
	// Once setup no longer has the role, the label goes, as a launcher's does.
	d.config.Roles = slices.DeleteFunc(slices.Clone(d.config.Roles), func(r Role) bool { return r.ID == "janitor" })
	if d.roleOf(s) != nil {
		t.Error("a role that setup no longer has was shown")
	}
}

func TestTheGMRoleThroughTheEngineHasTheGMGuardAndNoSendKey(t *testing.T) {
	r := newEngineRig(t)
	var asked [][]string
	r.o.Guard = func(tool string, args []string) (int, string, bool) {
		asked = append(asked, args)
		return 0, engineGuardPrinted, true
	}
	gm, err := runManagedHire(r.request("gm"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) == 0 || !slices.Contains(asked[0], "gm") || !slices.Contains(asked[0], "--profile") {
		t.Errorf("the guard was asked %q, want the gm profile", asked)
	}
	settings, _ := os.ReadFile(filepath.Join(hireFolder(supportDir, gm.ID), hireSettingsFile))
	if !strings.Contains(string(settings), "guard --profile gm ") {
		t.Errorf("the gm's settings hook has no gm profile: %s", settings)
	}
	if script := r.script(gm); strings.Contains(script, hireKeyEnvironment) {
		t.Errorf("the gm's start script carries a send key:\n%s", script)
	}
	// A hire of any other role gets the hire's profile and a send key of its own.
	other, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ = os.ReadFile(filepath.Join(hireFolder(supportDir, other.ID), hireSettingsFile))
	if strings.Contains(string(settings), "--profile") || scriptSendKey(r.script(other)) == "" {
		t.Errorf("a hire's settings took the gm's profile, or its script has no send key: %s", settings)
	}
}

// A pinned desk keeps the old path: the engine's worktree session is a
// project of its own, so it wouldn't sit at a pinned desk. (A desk chosen
// goes through the engine: see TestARoleHireAtAChosenDeskGoesThroughTheEngine.)
func TestAHireAtAPinnedDeskKeepsTheOldPath(t *testing.T) {
	d, r, h := engineDeck(t)
	pin := r.repo
	d.config.Pins[1] = &pin
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}, "desk": {"1"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	if len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
		t.Errorf("went through the engine: tmux %q, registry %+v", r.tmux.asked, registered(t))
	}
	if len(h.windows)+len(h.tmux) == 0 {
		t.Error("the old path opened nothing")
	}
}

// hireHeardFrom runs for real: only a Claude session in the hire's own folder
// (or inside it), started since the hire, counts.
func TestHireHeardFromIsAClaudeSessionInItsFolderSinceItWasHired(t *testing.T) {
	d := newTestDeck(t)
	folder := t.TempDir()
	hire := Hire{ID: "h1", Folder: folder, Started: 5000}
	apply := func(id string, agent Agent, cwd string, at float64) {
		d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": id, "cwd": cwd}, agent, "tmux:"+id, "", at)
	}
	apply("codex", Codex, folder, 6000)
	apply("elsewhere", Claude, t.TempDir(), 6000)
	sibling := folder + "-2" // made, so a folder's path with symlinks resolves as its neighbour's does
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	apply("sibling", Claude, sibling, 6000)
	apply("before", Claude, folder, 4000)
	if d.hireHeardFrom(hire) {
		t.Fatal("heard from a session that isn't the hire's")
	}
	inside := filepath.Join(folder, "sub")
	os.Mkdir(inside, 0o755)
	apply("inside", Claude, inside, 5000)
	if !d.hireHeardFrom(hire) {
		t.Error("not heard from a Claude session inside its folder, started at the same millisecond")
	}
	d2 := newTestDeck(t)
	d2.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s", "cwd": folder}, Claude, "tmux:s", "", 6000)
	if !d2.hireHeardFrom(hire) {
		t.Error("not heard from a Claude session in its folder")
	}
}

// The engine's whole watch, with the deck's own heard-from and no stub.
func TestARoleHireInTmuxIsWorkingOnceItsSessionReportsToTheDeck(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.Within, d.engine.Every = 5*time.Second, 5*time.Millisecond
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	hire := registered(t)[0]
	// Under the deck's lock, as its ingest path holds it: the watcher reads the sessions meanwhile.
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": startedIn(hire)}, Claude, "tmux:s1", "", hire.Started+1)
	d.mu.Unlock()
	d.watching.Wait()
	if got := registered(t); got[0].State != "working" {
		t.Errorf("registry %+v", got)
	}
}

// The watcher moves a hire only from "starting": one the owner fired or
// changed meanwhile is left as it is.
func TestTheWatcherLeavesAHireTheOwnerChangedAlone(t *testing.T) {
	for _, heard := range []bool{false, true} {
		r := newEngineRig(t)
		hire, err := runManagedHire(r.request("janitor"), r.o)
		if err != nil {
			t.Fatal(err)
		}
		if err := updateRegistry(func(f *registryFile) { f.hires[0].State, f.hires[0].Notes = "fired", "fired by the owner" }); err != nil {
			t.Fatal(err)
		}
		now := time.Unix(1000, 0)
		r.o.Now = func() time.Time { return now }
		r.o.Pause = func(d time.Duration) { now = now.Add(d) }
		r.o.HeardFrom = func(Hire) bool { return heard }
		watchManagedHire(hire, 0, r.o)
		if got := registered(t); got[0].State != "fired" || got[0].Notes != "fired by the owner" {
			t.Errorf("heard %v: registry %+v", heard, got)
		}
	}
}

// A hire on the ground floor stays there, as a launcher's hire does, though a
// rule for its folder says another floor: an engine hire's tmux tab isn't a
// pending launcher hire, so the deck seats it on the ground floor itself.
func TestAnEngineHireOnTheGroundFloorStaysThereThoughItsFolderGoesUpstairs(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	hire := registered(t)[0]
	// The rule is for the folder above the hire's worktree.
	d.mu.Lock()
	d.config.FloorRules = floorRuleList{{filepath.Dir(hire.Folder), 3, false}}
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "hired", "cwd": hire.Folder},
		Claude, "tmux:%1@"+hire.TmuxSocket, "", hire.Started+1)
	keys, _ := d.layout()
	d.mu.Unlock()
	if !slices.ContainsFunc(keys, func(k deckKey) bool { return k.session != nil && k.session.ID == "hired" }) {
		t.Errorf("the hire isn't on the ground floor: %v", keys)
	}
	if desks := floorDesks(t, d, 3); len(desks) != 0 {
		t.Errorf("4F is %v, want nobody there", desks)
	}
	// Anyone else in that folder goes up by its rule.
	d.mu.Lock()
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "other", "cwd": hire.Folder},
		Claude, "tmux:%9@/tmp/tmux", "", hire.Started+2)
	d.mu.Unlock()
	if desks := floorDesks(t, d, 3); len(desks) != 1 || desks[0]["id"] != "other" {
		t.Errorf("4F is %v, want the other session there by its rule", desks)
	}
}

// The deck hires on the default account (the engine's own test sets it too):
// no CLAUDE_CONFIG_DIR is carried into the hire.
func TestTheDeckHiresARoleOnTheDefaultAccount(t *testing.T) {
	d, r, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	if script := r.script(registered(t)[0]); !strings.Contains(script, "unset CLAUDE_CONFIG_DIR") {
		t.Errorf("the hire's script doesn't use the default account:\n%s", script)
	}
}

// A window that won't open ends the hire's tmux session, answers 500, marks it
// failed, and spends no cooldown: the next hire needn't wait.
func TestAWindowThatWontOpenEndsTheEngineHireAndSpendsNoCooldown(t *testing.T) {
	d, r, h := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	h.fail = errors.New("no window")
	if status, _ := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusInternalServerError {
		t.Fatalf("hire: %d, want 500", status)
	}
	d.watching.Wait()
	if r.tmux.up || !r.tmux.ran("kill-session") {
		t.Errorf("the hire's session was left up: tmux %q", r.tmux.asked)
	}
	if got := registered(t); len(got) != 1 || got[0].State != "failed" {
		t.Errorf("registry %+v, want one failed row", got)
	}
	if !d.lastHire.IsZero() {
		t.Error("an engine refusal left the cooldown spent")
	}
	h.fail = nil
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Errorf("the next hire: %d %s", status, message)
	}
	d.watching.Wait()
}

// hiredRoleAt finds a role for a socket in a hire socket folder: where new hires go (tmux's own folder,
// FXC3 A8), or the support folder, where hires made before 1.25 are. A socket anywhere else is nobody's.
func TestHiredRoleAtLooksInEveryHireSocketFolder(t *testing.T) {
	d, _, _ := engineDeck(t)
	// New hires' sockets are not in the support folder: point their folder at another, as a real run has it.
	own := t.TempDir()
	saved := hireSocketFolder
	t.Cleanup(func() { hireSocketFolder = saved })
	hireSocketFolder = func() string { return own }
	if filepath.Clean(own) == filepath.Clean(supportDir) {
		t.Fatal("the stand-in socket folder is the support folder: the test proves nothing")
	}
	inOwn := filepath.Join(own, "smolcorp-hire-x")
	inSupport := filepath.Join(supportDir, "hire-x.sock")
	outside := filepath.Join(t.TempDir(), "hire-x.sock")
	for _, socket := range []string{inOwn, inSupport, outside} {
		if err := updateRegistry(func(f *registryFile) {
			f.hires = append(f.hires, Hire{ID: newID(), Name: "Janitor", Folder: "/x", Role: "janitor", TmuxSocket: socket, State: "working"})
		}); err != nil {
			t.Fatal(err)
		}
	}
	if got := d.hiredRoleAt(inOwn); got != "janitor" {
		t.Errorf("a socket in tmux's own folder: %q, want janitor", got)
	}
	if got := d.hiredRoleAt(inSupport); got != "janitor" {
		t.Errorf("a socket in the support folder: %q, want janitor", got)
	}
	if got := d.hiredRoleAt(outside); got != "" {
		t.Errorf("a socket outside both: %q, want none", got)
	}
	if got := d.hiredRoleAt(""); got != "" {
		t.Errorf("no socket: %q", got)
	}
}

// A hire the Mac's engine made gives its socket with the folders on the way resolved (/private/tmp for /tmp,
// FXC3c): it keeps its role, and is still ours to fire, once the Go engine runs the office.
func TestASocketSpelledThroughALinkIsStillInAHireSocketFolder(t *testing.T) {
	d, _, _ := engineDeck(t)
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "tmp")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	saved := hireSocketFolder
	t.Cleanup(func() { hireSocketFolder = saved })
	hireSocketFolder = func() string { return link } // tmux's own folder, as /tmp/tmux-501
	resolved := filepath.Join(physical(real), "smolcorp-hire-x")
	if filepath.Dir(resolved) == filepath.Clean(link) {
		t.Fatal("the socket's folder is spelled as the hire socket folder: the test proves nothing")
	}
	if err := updateRegistry(func(f *registryFile) {
		f.hires = append(f.hires, Hire{ID: "hire-x", Name: "Janitor", Folder: "/x", Role: "janitor", TmuxSocket: resolved, State: "working"})
	}); err != nil {
		t.Fatal(err)
	}
	if got := d.hiredRoleAt(resolved); got != "janitor" {
		t.Errorf("a socket spelled through the link's target: %q, want janitor", got)
	}
	if !ownsHireSocket(resolved, 0) {
		t.Error("a hire's socket spelled through the link's target isn't ours")
	}
	elsewhere := filepath.Join(physical(t.TempDir()), "smolcorp-hire-x")
	if ownsHireSocket(elsewhere, 0) || d.hiredRoleAt(elsewhere) != "" {
		t.Error("a socket in another folder is ours")
	}
}

// scriptSendKey is the send key a start script exports, or "".
func scriptSendKey(script string) string {
	for _, line := range strings.Split(script, "\n") {
		if rest, ok := strings.CutPrefix(line, "export "+hireKeyEnvironment+"='"); ok {
			return strings.TrimSuffix(rest, "'")
		}
	}
	return ""
}

// A hire whose start fails after it was written down leaves neither its row nor its key.
func TestAHireRefusedAfterItWasWrittenDownLeavesNoRowAndNoKey(t *testing.T) {
	r := newEngineRig(t)
	r.tmux.starts = false
	if _, err := runManagedHire(r.request("janitor"), r.o); err == nil {
		t.Fatal("a hire whose tmux didn't start was made")
	}
	file := readRegistry(registryPath(), false)
	if len(file.hires) != 0 || len(file.keys) != 0 {
		t.Errorf("left in the registry: %+v, keys %v", file.hires, file.keys)
	}
	// The registry write is what the key stands on: a hire that started has both.
	r.tmux.starts = true
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if file := readRegistry(registryPath(), false); len(file.hires) != 1 || file.keys[hire.ID] == "" {
		t.Errorf("registry %+v, keys %v", file.hires, file.keys)
	}
}

// As the Mac's HireDesk: a managed hire's brief is written in its own folder, outside its worktree,
// written down with it, and its first message says only to read it.
func TestAnEngineHireIsToldToReadTheBriefWrittenForIt(t *testing.T) {
	r := newEngineRig(t)
	request := r.request("tester")
	request.Role.Brief = &RoleBrief{Kind: "library", ID: "tester"}
	request.Task = "Fix the login bug."
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(hireFolder(supportDir, hire.ID), "BRIEF.md")
	data, err := os.ReadFile(brief)
	if err != nil || !strings.Contains(string(data), "# Brief: Tester") || !strings.Contains(string(data), "The task, from whoever hired you:\n\nFix the login bug.") ||
		!strings.Contains(string(data), "`"+tildePath(hire.Worktree)+"`") {
		t.Fatalf("the brief: %v\n%s", err, data)
	}
	if hire.Brief != brief || registered(t)[0].Brief != brief {
		t.Errorf("the brief written down: %q, %q", hire.Brief, registered(t)[0].Brief)
	}
	if script := r.script(hire); !strings.Contains(script, "'Read `"+tildePath(brief)+"` and start: it'\\''s your brief, as my Tester") {
		t.Errorf("the first message doesn't name the brief:\n%s", script)
	}
	if entries, _ := os.ReadDir(hire.Worktree); slices.ContainsFunc(entries, func(e os.DirEntry) bool { return strings.HasPrefix(e.Name(), "BRIEF") }) {
		t.Error("the brief went in the worktree")
	}
	// A role on its own instructions is told them, as before.
	request = r.request("janitor")
	request.Role.Brief = &RoleBrief{Kind: "text", Text: "Tidy."}
	request.Role.Instructions = "Tidy."
	plain, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(hireFolder(supportDir, plain.ID), "BRIEF.md")); err == nil || plain.Brief != "" {
		t.Errorf("a role on its instructions got a brief: %q", plain.Brief)
	}
	if script := r.script(plain); !strings.Contains(script, "Once I'\\''ve answered, as my Janitor: Tidy.") {
		t.Errorf("the plain role's message:\n%s", script)
	}
}

// Outside git there's no worktree: the brief goes in Smolcorp's briefs folder, named for the desk, and is taken
// away again if the hire doesn't go ahead.
func TestAnEngineHireOutsideGitKeepsItsBriefInSmolcorpsFolder(t *testing.T) {
	r := newEngineRig(t)
	request := r.request("tester")
	request.Role.Brief = &RoleBrief{Kind: "library", ID: "tester"}
	request.Folder = t.TempDir()
	desk := 2
	request.Desk = &desk
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	if hire.Worktree != "" || filepath.Dir(hire.Brief) != briefsKeptDir() || !strings.HasPrefix(filepath.Base(hire.Brief), "2-tester-") {
		t.Fatalf("the hire: %+v", hire)
	}
	r.tmux.starts = false
	if _, err := runManagedHire(request, r.o); err == nil {
		t.Fatal("a hire tmux didn't start went ahead")
	}
	if kept, _ := filepath.Glob(filepath.Join(briefsKeptDir(), "*.md")); len(kept) != 1 || kept[0] != hire.Brief {
		t.Errorf("briefs kept: %v", kept)
	}
}

// The Mac's hireWorktrees, set by hand in config.json: a full path is where hires' worktrees are made, each
// repo's under a folder of its own name there, as the Mac's HireDesk.place makes them; the fire removes the
// worktree from there. Anything but a full path loads as "", beside the repo, as the Mac's Config reads it.
func TestHiresWorktreesGoInTheFolderConfigJSONSets(t *testing.T) {
	d, r, _ := engineDeck(t)
	for _, saved := range []string{"relative/hires", "~/hires", ""} {
		writeFile(t, configFile, `{"hireWorktrees": "`+saved+`"}`)
		if got := loadConfig().HireWorktrees; got != "" {
			t.Errorf("hireWorktrees %q loads as %q, want beside the repo", saved, got)
		}
	}
	folder := t.TempDir()
	writeFile(t, configFile, `{"hireWorktrees": "`+folder+`"}`)
	if got := loadConfig().HireWorktrees; got != folder {
		t.Fatalf("hireWorktrees loads as %q, want %q", got, folder)
	}
	d.engine.HeardFrom = func(Hire) bool { return true }
	d.mu.Lock()
	d.config.HireWorktrees = folder
	d.mu.Unlock()
	if status, message := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 1 || got[0].Worktree == "" {
		t.Fatalf("registry %+v", got)
	}
	hire := got[0]
	if parent := physical(filepath.Join(folder, filepath.Base(r.repo))); filepath.Dir(hire.Worktree) != parent {
		t.Fatalf("the worktree is %q, want it in %q", hire.Worktree, parent)
	}
	windowShows(r, pane(t, "idle-empty"), true)
	d.mu.Lock()
	d.engine.Tmux = r.o.Tmux
	d.mu.Unlock()
	if kept, _, err := d.fireTmuxHire(hire, false, false); err != nil || kept != "" {
		t.Fatalf("fire: kept %q, err %v", kept, err)
	}
	if _, err := os.Stat(hire.Worktree); err == nil || branches(t, r.repo) != "" {
		t.Error("the clean worktree in hireWorktrees, or its branch, was left")
	}
}

// keyMac stands in for the Mac app's API-key account (HostBridge's api-key-offer
// and api-key-read): the key only while it's offered.
type keyMac struct {
	mu      sync.Mutex
	offered bool
	asked   []string
}

const keyMacRefusal = "There's no Anthropic API key in your Keychain. Save one under Summaries, or hire on a Claude account."

func (m *keyMac) call(r bridgeRequest) (bridgeAnswer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asked = append(m.asked, r.Action)
	switch {
	case r.Action == "api-key-offer" && m.offered:
		return bridgeAnswer{OK: true, Available: true, Notice: apiKeyNotice}, nil
	case r.Action == "api-key-offer":
		return bridgeAnswer{OK: true, Notice: apiKeyNotice, Refusal: keyMacRefusal}, nil
	case r.Action == "api-key-read" && m.offered:
		return bridgeAnswer{OK: true, Found: true, Value: fakeHireKey}, nil
	case r.Action == "api-key-read":
		return bridgeAnswer{Error: keyMacRefusal}, nil
	}
	return bridgeAnswer{Error: "not asked of this stand-in"}, nil
}

// filesHolding are the files under the folders that hold text, by path.
func filesHolding(t *testing.T, text string, folders ...string) []string {
	t.Helper()
	var found []string
	for _, folder := range folders {
		filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.Type().IsRegular() {
				if data, _ := os.ReadFile(path); bytes.Contains(data, []byte(text)) {
					found = append(found, path)
				}
			}
			return nil
		})
	}
	return found
}

// M4c: under the Mac app, setup's hire on the API-key account gets the key from
// the app's Keychain through the bridge, once per hire, never from this
// computer's store. Its tail is approved; the key itself is only in that
// hire's start script (owner-only, removing itself as it runs): never in
// tmux's arguments, a window's, the registry, a log, or a file left behind by
// a hire refused or never started. Not offered, the Mac's refusal, nothing made.
func TestUnderTheMacAppAnAPIKeyHireGetsTheKeyThroughTheBridgeAndNowhereElse(t *testing.T) {
	d, r, h := engineDeck(t)
	mac := &keyMac{}
	d.mu.Lock()
	d.mac.host = mac
	d.mu.Unlock()
	d.engine.Key = func() string { t.Error("this computer's secret store was asked under the Mac app"); return "" }
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	repo := homeRepo(t)
	page := func(heard bool) *httptest.ResponseRecorder {
		t.Helper()
		d.mu.Lock()
		d.lastHire = time.Time{}
		d.engine.HeardFrom = func(Hire) bool { return heard }
		d.mu.Unlock()
		w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer,
			body: `{"role":"janitor","folder":"` + repo + `","answers":[],"apiKey":true}`, headers: map[string]string{setupHeader: "1"}})
		d.watching.Wait()
		return w
	}

	// Not offered: the Mac's words, the key never asked for, nothing made.
	if w := page(true); w.Code != http.StatusConflict || strings.TrimSpace(w.Body.String()) != keyMacRefusal {
		t.Fatalf("not offered: %d %s", w.Code, w.Body)
	}
	if len(registered(t)) != 0 || len(r.tmux.asked) != 0 || !slices.Equal(mac.asked, []string{"api-key-offer"}) {
		t.Fatalf("not offered: registry %+v, tmux %q, asked %q", registered(t), r.tmux.asked, mac.asked)
	}

	// Offered: hired, its tail approved, the key in its start script alone.
	mac.offered = true
	if w := page(true); w.Code != http.StatusOK {
		t.Fatalf("offered: %d %s", w.Code, w.Body)
	}
	hires := registered(t)
	if len(hires) != 1 || hires[0].Account != apiKeyAccountName || hires[0].State != "working" {
		t.Fatalf("registry %+v", hires)
	}
	if data, _ := os.ReadFile(filepath.Join(homeDir, ".claude.json")); !isApproved(keyTail(fakeHireKey), data) {
		t.Errorf("the key's tail isn't approved: %s", data)
	}
	script := r.script(hires[0])
	if !strings.Contains(script, "export ANTHROPIC_API_KEY='"+fakeHireKey+"'") || strings.Contains(script, "secret-tool") {
		t.Errorf("the script doesn't export the Mac app's key:\n%s", script)
	}
	scripts, _ := filepath.Glob(filepath.Join(hireFolder(supportDir, hires[0].ID), "hire-*.sh"))
	for _, path := range append(scripts, hireFolder(supportDir, hires[0].ID)) {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s isn't its owner's alone: %v %v", path, info.Mode(), err)
		}
	}

	// Refused once its script is written (tmux won't start), and never started: both take their scripts away.
	r.tmux.starts = false
	if w := page(true); w.Code != http.StatusInternalServerError {
		t.Errorf("tmux down: %d %s", w.Code, w.Body)
	}
	r.tmux.starts = true
	if w := page(false); w.Code != http.StatusOK {
		t.Fatalf("never started: %d %s", w.Code, w.Body)
	}
	if hires = registered(t); len(hires) != 2 || hires[1].State != "failed" {
		t.Fatalf("registry %+v", hires)
	}

	// Once per hire, through the bridge; and nowhere but the working hire's own script.
	if want := []string{"api-key-offer", "api-key-offer", "api-key-read", "api-key-offer", "api-key-read", "api-key-offer", "api-key-read"}; !slices.Equal(mac.asked, want) {
		t.Errorf("asked of the Mac app %q, want %q", mac.asked, want)
	}
	if found := filesHolding(t, fakeHireKey, supportDir, homeDir, r.o.Worktrees); len(found) != 1 || found[0] != scripts[0] {
		t.Errorf("files holding the key %q, want only %q", found, scripts)
	}
	for _, args := range append(r.tmux.asked, h.windows...) {
		if strings.Contains(strings.Join(args, " "), fakeHireKey) {
			t.Errorf("an argument holds the key: %q", args)
		}
	}
	if registry, _ := os.ReadFile(registryPath()); bytes.Contains(registry, []byte(fakeHireKey)) || bytes.Contains(logged.Bytes(), []byte(fakeHireKey)) {
		t.Error("the registry or the log holds the key")
	}
}

// A hire on the API-key account that the engine can't take (no tmux, a pinned
// desk) is refused before anything opens: the launcher can't give it the key,
// and a plain terminal would start it on the subscription.
func TestAnAPIKeyHireTheEngineCantTakeIsRefusedNeverPlain(t *testing.T) {
	d, r, h := engineDeck(t)
	d.mu.Lock()
	d.mac.host = &keyMac{offered: true}
	d.mu.Unlock()
	tmuxPath = func() string { return "" }
	repo := homeRepo(t)
	w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer,
		body: `{"role":"janitor","folder":"` + repo + `","answers":[],"apiKey":true}`, headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusConflict || strings.TrimSpace(w.Body.String()) != apiKeyNeedsEngine {
		t.Errorf("no tmux: %d %s", w.Code, w.Body)
	}
	if len(h.windows) != 0 || len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
		t.Errorf("something opened: windows %q, tmux %q", h.windows, r.tmux.asked)
	}
	// The same hire on a Claude account opens the old way, so it's the API key that refused it.
	d.mu.Lock()
	d.lastHire = time.Time{}
	d.mu.Unlock()
	if w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer,
		body: `{"role":"janitor","folder":"` + repo + `","answers":[]}`, headers: map[string]string{setupHeader: "1"}}); w.Code != http.StatusOK || len(h.windows) != 1 {
		t.Errorf("on a Claude account: %d %s, windows %q", w.Code, w.Body, h.windows)
	}
}

// A start script left by a core that stopped before the hire's shell ran it has no watcher: it is
// removed as the next core starts, and the hire it was for, still "starting", is marked failed.
func TestAStartScriptLeftByAStoppedCoreIsRemovedAtTheNextStart(t *testing.T) {
	isolate(t)
	const leftKey = "fk-left-77"
	scripts := map[string]string{}
	for _, id := range []string{"aaaaaaaaaaaa", "bbbbbbbbbbbb", "cccccccccccc"} {
		folder := hireFolder(supportDir, id)
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		scripts[id] = filepath.Join(folder, "hire-1234.sh")
		if err := os.WriteFile(scripts[id], []byte("#!/bin/sh\nrm -f \"$0\"\nexport ANTHROPIC_API_KEY='"+leftKey+"'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, "settings.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// "a" is still starting, "b" was fired meanwhile, "c" has no row at all.
	err := updateRegistry(func(f *registryFile) {
		f.hires = append(f.hires, Hire{ID: "aaaaaaaaaaaa", Name: "Janitor", Folder: "/work/a", State: "starting"}, Hire{ID: "bbbbbbbbbbbb", Name: "Cook", Folder: "/work/b", State: "fired"})
	})
	if err != nil {
		t.Fatal(err)
	}
	startDeck(t)
	for id, script := range scripts {
		if _, err := os.Stat(script); !os.IsNotExist(err) {
			t.Errorf("hire %s's start script, with a key, is still there after the start: %v", id, err)
		}
		if _, err := os.Stat(filepath.Join(hireFolder(supportDir, id), "settings.json")); err != nil {
			t.Errorf("hire %s's other files must stay: %v", id, err)
		}
	}
	got := map[string]Hire{}
	for _, h := range registered(t) {
		got[h.ID] = h
	}
	if h := got["aaaaaaaaaaaa"]; h.State != "failed" || h.Notes == "" || strings.Contains(h.Notes, leftKey) {
		t.Errorf("the hire that never ran its script should be failed with a plain sentence: %+v", h)
	}
	if h := got["bbbbbbbbbbbb"]; h.State != "fired" {
		t.Errorf("a fired hire is left as it is: %+v", h)
	}
}

// On a clean stop the core waits, for a while, for the hires it is still watching to finish cleaning.
func TestTheCoreWaitsABoundedWhileForHiresBeingWatched(t *testing.T) {
	d := &Deck{}
	d.watching.Add(1)
	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		d.watching.Done()
		close(done)
	}()
	if !d.waitForHires(5 * time.Second) {
		t.Error("it gave up on a watcher that finished in 50 ms")
	}
	<-done
	d.watching.Add(1) // one that never finishes
	t0 := time.Now()
	if d.waitForHires(100 * time.Millisecond) {
		t.Error("it said all were done with a watcher left")
	}
	if took := time.Since(t0); took > 2*time.Second {
		t.Errorf("the wait wasn't bounded: %v", took)
	}
	d.watching.Done()
}
