//go:build !windows

package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	case "kill-session":
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
	// One tmux call, on the hire's own socket, in the worktree.
	if len(r.tmux.asked) != 1 || r.tmux.asked[0][0] != "-S" || r.tmux.asked[0][1] != hire.TmuxSocket || !slices.Contains(r.tmux.asked[0], hire.Folder) {
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
	watchManagedHire(hire, 0, r.o)
	if !r.tmux.ran("kill-session") || r.tmux.up {
		t.Error("its session wasn't ended")
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
	d := newTestDeck(t)
	// newTestDeck isolated again: the rig's support folder and home are the ones to use.
	shortSupport(t)
	r.home = homeDir
	t.Setenv("HOME", r.home)
	writeConfig(t, r.home, "{}")
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
	d.sessions.Apply(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": hire.Folder}, Claude, "tmux:s1", "", hire.Started+1)
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

// hiredRoleAt finds a role only for a socket in the support folder.
func TestHiredRoleAtOnlyLooksUpASocketInTheSupportFolder(t *testing.T) {
	d, _, _ := engineDeck(t)
	inside := filepath.Join(supportDir, "hire-x.sock")
	outside := filepath.Join(t.TempDir(), "hire-x.sock")
	for _, socket := range []string{inside, outside} {
		if err := updateRegistry(func(f *registryFile) {
			f.hires = append(f.hires, Hire{ID: newID(), Name: "Janitor", Folder: "/x", Role: "janitor", TmuxSocket: socket, State: "working"})
		}); err != nil {
			t.Fatal(err)
		}
	}
	if got := d.hiredRoleAt(inside); got != "janitor" {
		t.Errorf("a socket in the support folder: %q, want janitor", got)
	}
	if got := d.hiredRoleAt(outside); got != "" {
		t.Errorf("a socket outside it: %q, want none", got)
	}
	if got := d.hiredRoleAt(""); got != "" {
		t.Errorf("no socket: %q", got)
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
