package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The lane through the deck (t3deskdeck.go): a hire into T3 Code, its row on the team, the lane's line on
// Setup, Let go by its route, and the office's fire refused, with the fake t3 of t3desk_test.go.

// t3DeckRig is a started deck with T3 Code on, the fake t3 its machine, and a stand-in guard.
func t3DeckRig(t *testing.T, flags ...string) (*t3Rig, *Deck) {
	t.Helper()
	r := newT3Rig(t)
	r.fake(flags...)
	keptMachine, keptStand := newT3Machine, t3Stand
	newT3Machine = r.machine
	t3Stand.PortFree = func(int) bool { return true }
	t3Stand.CLI = "/opt/smolcorp/bin/smolcorp"
	t3Stand.IsExec = func(string) bool { return true }
	t3Stand.Guard = func(string, []string) (int, string, bool) { return 0, t3TestPrinted, true }
	t3Stand.Leaving = func() t3Leaving { leave, _ := r.leaving(); return leave }
	t3Kept.Lock()
	t3Kept.status, t3Kept.at = t3Status{}, time.Time{}
	t3Kept.Unlock()
	d := startDeck(t)
	// Put back once the deck has stopped (cleanups run last first).
	t.Cleanup(func() { newT3Machine, t3Stand = keptMachine, keptStand })
	d.summaryProviders.set(map[summaryProvider]providerState{})
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.config.HiringEnabled = true
	d.config.ManagerEnabled = true
	d.mu.Unlock()
	return r, d
}

func t3PageCall(method, target, body string) call {
	return call{method: method, target: target, host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}, body: body}
}

func TestT3AHireThroughTheDeckIsOnTheTeamAndLetGoByItsRoute(t *testing.T) {
	r, d := t3DeckRig(t)
	repo := r.repo()
	reviewer := Role{ID: "reviewer", Name: "Reviewer", Agent: "claude"}
	if status, why := d.hireIntoT3(t3Ask{Role: reviewer, Folder: repo, Nickname: "Ada", Task: "Review the README."}); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, why)
	}
	hires := readRegistry(registryPath(), false).hires
	if len(hires) != 1 {
		t.Fatalf("%+v", hires)
	}
	h := hires[0]
	if h.Engine != t3Engine || h.T3Thread != "mcp:abc-1" || h.T3Link != "https://app.t3.codes/threads/env-1/mcp%3Aabc-1" || h.State != "starting" ||
		h.Nickname != "Ada" || h.Role != "reviewer" || h.Model != t3DefaultModel || h.Branch == "" || h.Worktree == "" || h.Folder != repo {
		t.Errorf("%+v", h)
	}
	// The gate asked t3 its version, fresh, before anything was made.
	runs := r.runs()
	if firsts := r.firsts(); len(runs) != 4 || runs[0][0] != "--version" || !reflect.DeepEqual(firsts[1:], [][]string{{"project", "add"}, {"thread", "start"}, {"thread", "show"}}) ||
		!strings.HasPrefix(runs[1][4], "Ada · hire/ada-") || !strings.HasSuffix(strings.Join(runs[2], " "), "--provider claudeAgent --model "+t3DefaultModel+" --json") {
		t.Errorf("%q", runs)
	}
	// The fence is the hire's settings on the default account, written as the Mac writes it.
	fence, _ := os.ReadFile(filepath.Join(h.Worktree, ".claude", "settings.local.json"))
	var object map[string]any
	if err := json.Unmarshal(fence, &object); err != nil || string(fence) != string(swiftJSON(object)) || object["disableAllHooks"] != false ||
		!strings.Contains(string(fence), `"Bash(git push:*)"`) || !strings.Contains(string(fence), "/opt/smolcorp/bin/smolcorp' guard --rules ") {
		t.Errorf("the fence: %v\n%s", err, fence)
	}
	if !strings.Contains(string(fence), `"Read(`+absoluteRulePath(filepath.Join(homeDir, ".claude"))+`/projects/**)"`) {
		t.Error("the default account's transcripts aren't denied")
	}
	// On the team, with its desk.
	got := serve(t, d, t3PageCall("GET", "/setup/gm/team", ""))
	var team struct{ Team []map[string]any }
	if json.Unmarshal(got.Body.Bytes(), &team) != nil || len(team.Team) != 1 || team.Team[0]["engine"] != "t3" || team.Team[0]["t3Thread"] != "mcp:abc-1" {
		t.Fatalf("%s", got.Body)
	}
	desk, _ := team.Team[0]["t3"].(map[string]any)
	if desk["tag"] != t3Tag || desk["canOpen"] != true || desk["status"] != "unknown" {
		t.Errorf("%v", desk)
	}
	// A second hire straight after waits for the cooldown.
	if status, _ := d.hireIntoT3(t3Ask{Role: reviewer, Folder: repo}); status != http.StatusTooManyRequests {
		t.Errorf("%d", status)
	}
	// Let go, by its id: the project removed, the fence taken away, the registry told, the recap written.
	_ = os.Remove(r.calls)
	got = serve(t, d, t3PageCall("POST", "/setup/t3code/letgo", `{"id":"`+h.ID+`","worktree":"/etc"}`))
	var answer map[string]any
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer["ok"] != true || answer["worktree"] != h.Worktree ||
		answer["branch"] != h.Branch || answer["note"] != "" || answer["recap"] == "" {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if runs := r.runs(); !reflect.DeepEqual(runs, [][]string{{"project", "remove", h.Worktree, "--force"}}) {
		t.Errorf("the worktree comes from the registry: %q", runs)
	}
	if exists(filepath.Join(h.Worktree, ".claude", "settings.local.json")) || !exists(h.Worktree) {
		t.Error("the fence stays, or the worktree went")
	}
	after, _ := registryHire(h.ID)
	if after.State != "fired" || !strings.HasPrefix(after.Notes, "Let go from T3 Code: its thread there was removed; its worktree and branch were kept.") {
		t.Errorf("%+v", after)
	}
	// Twice: already gone. Nobody: not on the team.
	if got := serve(t, d, t3PageCall("POST", "/setup/t3code/letgo", `{"id":"`+h.ID+`"}`)); got.Code != http.StatusConflict || got.Body.String() != "Ada is already gone." {
		t.Errorf("%d %s", got.Code, got.Body)
	}
	if got := serve(t, d, t3PageCall("POST", "/setup/t3code/letgo", `{"id":"no-such-hire"}`)); got.Code != http.StatusNotFound || got.Body.String() != t3NotOnTeam {
		t.Errorf("%d %s", got.Code, got.Body)
	}
}

func TestT3AHireThroughTheDeckIsRefusedByTheGateAndTheRole(t *testing.T) {
	r, d := t3DeckRig(t)
	repo := r.repo()
	reviewer := Role{ID: "reviewer", Name: "Reviewer", Agent: "claude"}
	if status, why := d.hireIntoT3(t3Ask{Role: reviewer, Folder: repo, Account: "work"}); status != http.StatusConflict ||
		why != "A hire in T3 Code works on T3 Code's own Claude account, so no account is picked for it." {
		t.Errorf("%d %s", status, why)
	}
	d.mu.Lock()
	d.config.T3CodeEnabled = false
	d.mu.Unlock()
	if status, why := d.hireIntoT3(t3Ask{Role: reviewer, Folder: repo}); status != http.StatusConflict || why != t3SwitchedOff {
		t.Errorf("%d %s", status, why)
	}
	if len(r.runs()) > 1 || exists(filepath.Join(r.home, "repo-hires")) || len(readRegistry(registryPath(), false).hires) != 0 {
		t.Errorf("something was made: %q", r.runs())
	}
}

// Setup's line on the hire sheet: whether a hire may go into T3 Code, or why not.
func TestT3SetupSaysWhetherTheLaneIsOpen(t *testing.T) {
	_, d := t3DeckRig(t)
	got := serve(t, d, t3PageCall("GET", "/setup/t3code?fresh=1", ""))
	var answer struct {
		State string
		Lane  map[string]any
	}
	if json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer.State != t3Found || !reflect.DeepEqual(answer.Lane, map[string]any{"ok": true, "why": ""}) {
		t.Errorf("%s", got.Body)
	}
	if err := os.Remove(filepath.Join(newT3Machine().Home, ".local", "bin", "t3")); err != nil {
		t.Fatal(err)
	}
	t3Kept.Lock()
	t3Kept.at = time.Time{}
	t3Kept.Unlock()
	got = serve(t, d, t3PageCall("GET", "/setup/t3code?fresh=1", ""))
	if json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer.Lane["ok"] != false ||
		answer.Lane["why"] != "Hiring into T3 Code needs its t3 command: in T3 Code, open Settings → About and install it." {
		t.Errorf("%s", got.Body)
	}
}

// The office (or a phone) doesn't fire a hire in T3 Code: it's let go from Setup's team list.
func TestT3TheOfficesFireOfAT3HireIsRefused(t *testing.T) {
	r, d := t3DeckRig(t)
	wt := r.fencedWorktree()
	h := r.t3Hire(wt)
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, h) }); err != nil {
		t.Fatal(err)
	}
	// A session working in the hire's worktree, in a terminal: the team pairs it with the hire. (T3 Code's own agent
	// has no terminal tab, so, as on the Mac, its events alone don't make a desk.)
	event := `{"hook_event_name":"SessionStart","session_id":"t3s","cwd":` + jsonText(wt) + `}`
	if got := serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: thisComputer,
		headers: map[string]string{hookKeyHeader: d.access.HookKey, "X-AgentDeck-Agent": "claude", "X-AgentDeck-Tab": "tmux:%9@/nowhere/t3-test-sock"},
		body:    event}); got.Code != http.StatusNoContent {
		t.Fatalf("the hook got %d", got.Code)
	}
	if status, why := d.Perform("fire", "t3s", url.Values{}); status != http.StatusConflict || why != "Ada runs in T3 Code: let it go from Setup's team list." {
		t.Errorf("%d %s", status, why)
	}
	if len(r.runs()) != 0 || !exists(filepath.Join(wt, ".claude", "settings.local.json")) {
		t.Error("something was run or removed")
	}
	// The office's card says so too, with its desk and branch.
	got := serve(t, d, call{method: "GET", target: "/t3code?session=t3s&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	var answer struct{ Lane map[string]any }
	if json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer.Lane["tag"] != t3Tag || answer.Lane["branch"] != "hire/ada-abcdef" {
		t.Errorf("%d %s", got.Code, got.Body)
	}
}

// At launch, a hire left part way is let go and marked failed, saying what was done.
func TestT3ADeckStartingLetsGoOfAHireLeftPartWay(t *testing.T) {
	r := newT3Rig(t)
	r.fake("gone")
	wt := r.fencedWorktree()
	left := r.t3Hire(wt)
	left.State, left.T3Thread = "starting", ""
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, left) }); err != nil {
		t.Fatal(err)
	}
	keptMachine, keptStand := newT3Machine, t3Stand
	newT3Machine = r.machine
	t3Stand.Leaving = func() t3Leaving { leave, _ := r.leaving(); return leave }
	d := startDeck(t)
	t.Cleanup(func() { newT3Machine, t3Stand = keptMachine, keptStand })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if h, _ := registryHire(left.ID); h.State == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	d.stopWatchers()
	h, _ := registryHire(left.ID)
	if h.State != "failed" || !strings.HasPrefix(h.Notes, "Smolcorp closed while hiring it into T3 Code, so at the next launch its T3 Code project (if it had one) was removed and its fence taken away.") {
		t.Errorf("%+v", h)
	}
	if exists(filepath.Join(wt, ".claude", "settings.local.json")) {
		t.Error("the fence stays")
	}
}

func jsonText(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}
