package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

// t3GMCall is a call from the GM, with its token.
func t3GMCall(d *Deck, target, body string) call {
	return call{method: "POST", target: target, host: "localhost:7710", from: thisComputer, body: body,
		withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: d.newGMToken()}}
}

// B1 (RPGS): the GM's hire with `engine: "t3"` (gm hire --t3) goes into T3 Code by Setup's own lane, as the Mac's
// /setup/gm/hire does: into the folder for new hires, with the answer of any setup hire. The GM-token rule comes
// first, and hiring off and the hooks are refused in the lane's own words, before anything is made.
func TestT3TheGMsHireWithEngineT3HiresIntoT3Code(t *testing.T) {
	r, d := t3DeckRig(t)
	repo := r.repo()
	homeDir = r.home // the folder must be inside the home folder, and the fence denies this home's ~/.claude
	d.mu.Lock()
	d.config.Roles = []Role{{ID: "reviewer", Name: "Reviewer", Agent: "claude", Brief: &RoleBrief{Kind: "library", ID: "tester"}}, {ID: "gm", Name: "General Manager", Agent: "claude"}}
	d.config.HireFolder = repo
	d.mu.Unlock()
	hire := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		return serve(t, d, t3GMCall(d, "/setup/gm/hire", body))
	}
	madeNothing := func(when string) {
		t.Helper()
		if len(r.runs()) > 0 || len(readRegistry(registryPath(), false).hires) != 0 {
			t.Fatalf("%s: something was made: %q", when, r.runs())
		}
	}
	if got := hire(`{"role":"gm","engine":"t3"}`); got.Code != http.StatusConflict || got.Body.String() != onlyOwnerHiresGMRefusal {
		t.Errorf("the gm role: %d %q", got.Code, got.Body)
	}
	if got := hire(`{"role":"reviewer","engine":"vim"}`); got.Code != http.StatusBadRequest || got.Body.String() != hireEngineRefusal {
		t.Errorf("another engine: %d %q", got.Code, got.Body)
	}
	madeNothing("the gm role, another engine")
	d.mu.Lock()
	d.config.HiringEnabled = false
	d.mu.Unlock()
	if got := hire(`{"role":"reviewer","engine":"t3"}`); got.Code != http.StatusForbidden || got.Body.String() != hiringOff {
		t.Errorf("hiring off: %d %q", got.Code, got.Body)
	}
	d.mu.Lock()
	d.config.HiringEnabled = true
	d.mu.Unlock()
	kept := hooksNotConnected
	hooksNotConnected = func(Agent) string {
		return "Connect your agents first: Claude Code isn't connected. Press Connect in setup."
	}
	got := hire(`{"role":"reviewer","engine":"t3"}`)
	hooksNotConnected = kept
	if got.Code != http.StatusConflict || !strings.HasPrefix(got.Body.String(), "Connect your agents first:") {
		t.Errorf("hooks: %d %q", got.Code, got.Body)
	}
	madeNothing("hiring off, the hooks")

	got = hire(`{"role":"reviewer","engine":"t3"}`)
	var answer map[string]any
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer["ok"] != true || answer["session"] != nil {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if _, found := answer["desk"]; !found {
		t.Errorf("no desk in %v", answer)
	}
	hires := readRegistry(registryPath(), false).hires
	if len(hires) != 1 || hires[0].Engine != t3Engine || hires[0].Role != "reviewer" || hires[0].T3Thread != "mcp:abc-1" || hires[0].Folder != repo {
		t.Fatalf("%+v", hires)
	}
	if firsts := r.firsts(); !reflect.DeepEqual(firsts[1:], [][]string{{"project", "add"}, {"thread", "start"}, {"thread", "show"}}) {
		t.Errorf("%q", r.runs())
	}
}

// N6 (RPGS): the GM's fire of a hire in T3 Code is refused in the Mac's words, and nothing is run or removed.
func TestT3TheGMsFireOfAT3HireIsRefusedInTheMacsWords(t *testing.T) {
	r, d := t3DeckRig(t)
	wt := r.fencedWorktree()
	h := r.t3Hire(wt)
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, h) }); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"id":` + jsonText(h.ID) + `}`, `{"id":` + jsonText(h.ID) + `,"force":true,"removeWorktree":true}`} {
		got := serve(t, d, t3GMCall(d, "/setup/gm/fire", body))
		if want := "Ada runs in T3 Code: let it go from Setup's team list or from its desk."; got.Code != http.StatusConflict || got.Body.String() != want {
			t.Errorf("%s: %d %q, want 409 %q", body, got.Code, got.Body, want)
		}
	}
	if len(r.runs()) != 0 || !exists(filepath.Join(wt, ".claude", "settings.local.json")) || readRegistry(registryPath(), false).hires[0].State == "fired" {
		t.Error("something was run, removed or marked fired")
	}
}

// The office (or a phone) doesn't fire a hire in T3 Code: it's let go from Setup's team list or its desk. Its desk
// (t3desks.go) is the only session it claims, and nothing there but Let go reaches it.
func TestT3TheOfficesFireOfAT3HireIsRefused(t *testing.T) {
	r, d := t3DeckRig(t)
	wt := r.worktree()
	if err := os.MkdirAll(filepath.Join(wt, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".claude", "settings.local.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := r.t3Hire(wt)
	h.Repo = filepath.Join(r.home, "repo")
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, h) }); err != nil {
		t.Fatal(err)
	}
	desk := t3DeskID(h.ID)
	if status, why := d.Perform("fire", desk, url.Values{}); status != http.StatusConflict || why != t3NoControls("Ada") {
		t.Errorf("%d %s", status, why)
	}
	if why := t3FireRefusal(h); why != "Ada runs in T3 Code: let it go from Setup's team list or from its desk." {
		t.Errorf("%s", why)
	}
	if len(r.runs()) != 0 || !exists(filepath.Join(wt, ".claude", "settings.local.json")) {
		t.Error("something was run or removed")
	}
	// The office's card says so too, with its desk and branch.
	got := serve(t, d, call{method: "GET", target: "/t3code?session=" + url.QueryEscape(desk) + "&t=" + d.access.WindowKey, host: "localhost:7710", from: thisComputer})
	var answer struct{ Lane map[string]any }
	if json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer.Lane["tag"] != t3Tag || answer.Lane["branch"] != "hire/ada-abcdef" {
		t.Errorf("%d %s", got.Code, got.Body)
	}
}

// A Let go whose registry can't be written says so, rather than answering ok for a hire still on the team; let go
// again once it can be, it's fired (its project already removed is no error then).
func TestT3LetGoWhoseRegistryCantBeWrittenSaysSo(t *testing.T) {
	r := newT3Rig(t)
	r.fake("gone")
	h := r.t3Hire(r.fencedWorktree())
	keptMachine, keptStand := newT3Machine, t3Stand
	newT3Machine = r.machine
	t3Stand.Leaving = func() t3Leaving { leave, _ := r.leaving(); return leave }
	t.Cleanup(func() { newT3Machine, t3Stand = keptMachine, keptStand })
	d := newTestDeck(t)
	t3Registered(t, h)
	// The registry's temp file can't be made: a folder is where it goes.
	blocker := registryPath() + ".tmp"
	if err := os.Mkdir(blocker, 0o700); err != nil {
		t.Fatal(err)
	}
	got := serve(t, d, t3PageCall("POST", "/setup/t3code/letgo", `{"id":"`+h.ID+`"}`))
	if got.Code != http.StatusInternalServerError || got.Body.String() != "Ada was let go from T3 Code, but the registry couldn't be written, so it's still on the team: let it go again." {
		t.Errorf("%d %s", got.Code, got.Body)
	}
	if still, _ := registryHire(h.ID); !isOpen(still) {
		t.Errorf("%+v", still)
	}
	_ = os.Remove(blocker)
	if got := serve(t, d, t3PageCall("POST", "/setup/t3code/letgo", `{"id":"`+h.ID+`"}`)); got.Code != http.StatusOK {
		t.Errorf("again: %d %s", got.Code, got.Body)
	}
	if after, _ := registryHire(h.ID); after.State != "fired" {
		t.Errorf("%+v", after)
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

// Setup's hire sheet with `engine: "t3"` hires through the T3 Code lane, as the Mac's Deck.hire: the Mac's
// answer for any setup hire, the folder put first among the recent ones; an account, the API-key account,
// hiring off and the hooks each refused in their own words, before anything is made.
func TestT3SetupsHireSheetHiresIntoT3Code(t *testing.T) {
	r, d := t3DeckRig(t)
	repo := r.repo()
	homeDir = r.home // the folder must be inside the home folder, and the fence denies this home's ~/.claude
	reviewer := Role{ID: "reviewer", Name: "Reviewer", Agent: "claude", Brief: &RoleBrief{Kind: "library", ID: "tester"}}
	d.mu.Lock()
	d.config.Roles = []Role{reviewer}
	d.mu.Unlock()
	hire := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		return serve(t, d, t3PageCall("POST", "/setup/hire", body))
	}
	madeNothing := func(when string) {
		t.Helper()
		if len(r.runs()) > 0 || len(readRegistry(registryPath(), false).hires) != 0 {
			t.Fatalf("%s: something was made: %q", when, r.runs())
		}
	}
	for _, body := range []string{
		`{"role":"reviewer","engine":"t3","folder":` + jsonText(repo) + `,"account":"work"}`,
		`{"role":"reviewer","engine":"t3","folder":` + jsonText(repo) + `,"apiKey":true}`,
	} {
		if got := hire(body); got.Code != http.StatusConflict || got.Body.String() != "A hire in T3 Code works on T3 Code's own Claude account, so no account is picked for it." {
			t.Errorf("%s: %d %q", body, got.Code, got.Body)
		}
	}
	madeNothing("an account")
	kept := hooksNotConnected
	hooksNotConnected = func(Agent) string {
		return "Connect your agents first: Claude Code isn't connected. Press Connect in setup."
	}
	got := hire(`{"role":"reviewer","engine":"t3","folder":` + jsonText(repo) + `}`)
	hooksNotConnected = kept
	if got.Code != http.StatusConflict || !strings.HasPrefix(got.Body.String(), "Connect your agents first:") {
		t.Errorf("hooks: %d %q", got.Code, got.Body)
	}
	madeNothing("the hooks")
	d.mu.Lock()
	d.config.HiringEnabled = false
	d.mu.Unlock()
	if got := hire(`{"role":"reviewer","engine":"t3","folder":` + jsonText(repo) + `}`); got.Code != http.StatusForbidden || got.Body.String() != hiringOff {
		t.Errorf("hiring off: %d %q", got.Code, got.Body)
	}
	d.mu.Lock()
	d.config.HiringEnabled = true
	d.mu.Unlock()
	madeNothing("hiring off")

	got = hire(`{"role":"reviewer","engine":"t3","folder":` + jsonText(repo) + `,"nickname":"Ada","model":"claude-opus-5-5","task":"Review the README.","answers":["the docs"]}`)
	var answer map[string]any
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &answer) != nil || answer["ok"] != true || answer["session"] != nil {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	if _, found := answer["desk"]; !found {
		t.Errorf("no desk in %v", answer)
	}
	hires := readRegistry(registryPath(), false).hires
	if len(hires) != 1 || hires[0].Engine != t3Engine || hires[0].Nickname != "Ada" || hires[0].Model != "claude-opus-5-5" || hires[0].T3Thread != "mcp:abc-1" || hires[0].Folder != repo {
		t.Fatalf("%+v", hires)
	}
	if firsts := r.firsts(); !reflect.DeepEqual(firsts[1:], [][]string{{"project", "add"}, {"thread", "start"}, {"thread", "show"}}) {
		t.Errorf("%q", r.runs())
	}
	brief, _ := os.ReadFile(hires[0].Brief)
	if !strings.Contains(string(brief), "Review the README.") {
		t.Errorf("the task isn't in the brief %s:\n%s", hires[0].Brief, brief)
	}
	d.mu.Lock()
	recent := slices.Clone(d.config.RecentHireFolders)
	d.mu.Unlock()
	if len(recent) == 0 || recent[0] != repo {
		t.Errorf("recent folders %q", recent)
	}
}

// A hire in T3 Code works on the default account, but its fence denies every saved account's settings and
// transcripts, as the Mac's HireSettings.forHire: one inside the home folder and one outside it. A badly formed
// entry gets no rule. With nothing saved, the fence denies the default account alone, as the Mac's fixture has it.
func TestT3TheFenceDeniesEverySavedAccount(t *testing.T) {
	r, d := t3DeckRig(t)
	repo := r.repo()
	homeDir = r.home // the folder must be inside the home folder
	if got := claudeDenyProfiles(nil); !reflect.DeepEqual(got, []claudeProfile{{Name: "default", Dir: filepath.Join(homeDir, ".claude")}}) {
		t.Errorf("with nothing saved: %+v", got)
	}
	work := claudeFolder(t, filepath.Join(homeDir, "accounts", "work"), "settings.json")
	outside := claudeFolder(t, filepath.Join(physical(t.TempDir()), "volume", ".claude-ext"), "settings.json")
	d.mu.Lock()
	d.config.Roles = []Role{{ID: "reviewer", Name: "Reviewer", Agent: "claude"}}
	d.config.ClaudeProfiles = append([]claudeProfile{{Name: "work", Dir: work}, {Name: "ext", Dir: outside}}, badlyFormedAccounts...)
	d.mu.Unlock()
	if got := serve(t, d, t3PageCall("POST", "/setup/hire", `{"role":"reviewer","engine":"t3","folder":`+jsonText(repo)+`}`)); got.Code != http.StatusOK {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	hires := readRegistry(registryPath(), false).hires
	if len(hires) != 1 || hires[0].Engine != t3Engine || hires[0].Worktree == "" {
		t.Fatalf("%+v", hires)
	}
	fence, err := os.ReadFile(filepath.Join(hires[0].Worktree, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	deniesTheAccounts(t, "the T3 fence", string(fence), filepath.Join(homeDir, ".claude"), work, outside)
}

// hireIntoT3 is safe on its own (RM5a N8): no role, or hiring off, is refused before anything is made.
func TestT3HireIntoT3RefusesNoRoleAndHiringOffOnItsOwn(t *testing.T) {
	r, d := t3DeckRig(t)
	repo := r.repo()
	if status, why := d.hireIntoT3(t3Ask{Folder: repo}); status != http.StatusConflict || why != "Pick a role: a hire in T3 Code starts on its brief." {
		t.Errorf("no role: %d %s", status, why)
	}
	d.mu.Lock()
	d.config.HiringEnabled = false
	d.mu.Unlock()
	if status, why := d.hireIntoT3(t3Ask{Role: Role{ID: "reviewer", Name: "Reviewer", Agent: "claude"}, Folder: repo}); status != http.StatusForbidden || why != hiringOff {
		t.Errorf("hiring off: %d %s", status, why)
	}
	if len(r.runs()) > 0 || len(readRegistry(registryPath(), false).hires) != 0 {
		t.Errorf("something was made: %q", r.runs())
	}
}

// While T3 Code is off, the follow runs no t3 at all (RM5a N7).
func TestT3TheFollowRestsWhileT3CodeIsOff(t *testing.T) {
	r, d := t3DeckRig(t)
	hire := r.t3Hire(r.fencedWorktree())
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, hire) }); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	d.config.T3CodeEnabled = false
	d.mu.Unlock()
	d.t3FollowTick(registryPath())
	d.rounds.stop() // any round it started has ended
	if runs := r.runs(); len(runs) != 0 {
		t.Errorf("t3 ran while off: %q", runs)
	}
}
