package desktop

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// A hire in T3 Code is a desk in the office (t3desks.go, as the Mac's T3DeskOfficeTests): made from Smolcorp's own
// registry alone (engine t3, its thread and worktree), never from a tabless hook session, a name or a folder. Its
// Claude Code's own hook events make no second desk and never land on a terminal's. Nothing types into it; a tap
// opens its thread in T3 Code; Let go is the lane's; and it sits where it sat after a restart, as the registry kept it.

const t3DeskTestID = "abcdef12-0000-4000-8000-00000000d35c"

// t3DeskHire is a hire in T3 Code as the lane registers it once its thread runs, its worktree beside its repo.
func t3DeskHire(home, id string) Hire {
	repo := filepath.Join(home, "repo")
	return Hire{ID: id, Name: "Reviewer", Nickname: "Rev", Role: "reviewer", Folder: repo, Branch: "hire/rev-" + id[:6],
		Worktree: filepath.Join(home, "repo-hires", "rev-"+id[:6]), Repo: repo, Model: "claude-sonnet-5-5", Started: 1_761_000_000_000,
		State: "working", Engine: t3Engine, T3Thread: "mcp:abc-1", T3Link: "https://app.t3.codes/threads/env-1/mcp%3Aabc-1"}
}

func t3Registered(t *testing.T, hires ...Hire) {
	t.Helper()
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, hires...) }); err != nil {
		t.Fatal(err)
	}
}

// t3Deck is a test deck with the hires registered (T3 Code off: nothing runs t3).
func t3Deck(t *testing.T, hires ...Hire) (*Deck, string) {
	t.Helper()
	d := newTestDeck(t)
	home, err := filepath.EvalSymlinks(homeDir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range hires {
		if hires[i].Repo == "" {
			hires[i] = t3DeskHire(home, hires[i].ID)
		}
	}
	t3Registered(t, hires...)
	return d, home
}

// hookOn is a hook event, as /hook gives it to the deck.
func hookOn(d *Deck, id, name, cwd, tab, startedIn string, at float64) {
	d.Ingest(map[string]any{"hook_event_name": name, "session_id": id, "cwd": cwd}, "claude", tab, startedIn, at)
}

// officeDesks are every desk in the snapshot, the keys' and every floor's.
func officeDesks(t *testing.T, d *Deck) []map[string]any {
	t.Helper()
	d.mu.Lock()
	data := d.snapshot()
	d.mu.Unlock()
	var state struct {
		Keys   []struct{ Session map[string]any }
		Floors []struct{ Desks []map[string]any }
	}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	var desks []map[string]any
	for _, k := range state.Keys {
		if k.Session != nil {
			desks = append(desks, k.Session)
		}
	}
	for _, f := range state.Floors {
		desks = append(desks, f.Desks...)
	}
	return desks
}

func deskIDs(desks []map[string]any) []string {
	ids := []string{}
	for _, desk := range desks {
		id, _ := desk["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func keyOfDesk(d *Deck, id string) int {
	keys, _ := seating(d)
	return slices.Index(keys, id)
}

func knownSession(d *Deck, id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sessions.Get(id) != nil
}

// MARK: The exception: a desk keyed by the registry

func TestT3DeskARegisteredHireIsADeskWithItsRoleNameBranchAndStatus(t *testing.T) {
	d, home := t3Deck(t, Hire{ID: t3DeskTestID})
	desk := t3DeskID(t3DeskTestID)
	var shown map[string]any
	for _, one := range officeDesks(t, d) {
		if one["id"] == desk {
			shown = one
		}
	}
	if shown == nil {
		t.Fatal("no desk for the hire")
	}
	role, _ := shown["role"].(map[string]any)
	if shown["name"] != "rev-abcdef" || shown["project"] != "rev-abcdef" || role["id"] != "reviewer" || shown["state"] != "thinking" ||
		shown["activity"] != "At work in T3 Code" || shown["agent"] != "claude" || shown["turnStartedAt"] != nil {
		t.Errorf("%v", shown)
	}
	lane, _ := shown["t3"].(map[string]any)
	if lane["tag"] != t3Tag || lane["experimental"] != true || lane["branch"] != "hire/rev-abcdef" || lane["canOpen"] != true ||
		lane["status"] != "unknown" || lane["thread"] != nil || len(lane) != 5 {
		t.Errorf("its t3: %v", lane)
	}
	d.mu.Lock()
	data := string(d.snapshot())
	d.mu.Unlock()
	if strings.Contains(data, filepath.Join(home, "repo-hires")) || strings.Contains(data, "mcp:abc-1") {
		t.Error("a path or the thread's id in the snapshot")
	}
	// One desk, on the ground floor, at a key.
	if ids := deskIDs(officeDesks(t, d)); !slices.Equal(ids, []string{desk}) || keyOfDesk(d, desk) < 0 {
		t.Errorf("%v", ids)
	}
}

func TestT3DeskItsLookFollowsTheThreadWorkingWaitingDone(t *testing.T) {
	isolate(t)
	h := t3DeskHire(homeDir, t3DeskTestID)
	t3Registered(t, h)
	desks := newT3Desks(registryPath())
	look := func(status string) *Session {
		seated := desks.seated("", func(string) (t3Glance, bool) { return t3Glance{Status: status}, true })
		if len(seated) != 1 {
			t.Fatalf("%v", seated)
		}
		return seated[0].session
	}
	for _, step := range []struct {
		status string
		want   SessionState
		tap    bool
	}{{"working", Thinking, false}, {"waiting", Asking, false}, {"done", Unread, true}, {"done", Idle, false},
		{"working", Thinking, false}, {"done", Unread, false}, {"unknown", Idle, false}} {
		s := look(step.status)
		if s.State != step.want {
			t.Errorf("%s: %s, want %s", step.status, s.State, step.want)
		}
		if step.tap {
			desks.tapped(t3DeskID(t3DeskTestID))
		}
	}
	if s := look("waiting"); s.Activity() != "Waiting for an answer in T3 Code" {
		t.Errorf("%q", s.Activity())
	}
	// Not seen yet: as the registry says.
	if t3DeskState("", "idle", false) != Idle || t3DeskState("", "working", false) != Thinking || t3DeskState("starting", "starting", false) != Thinking {
		t.Error("before the first look")
	}
}

func TestT3DeskOnlyAnOpenHireWithAThreadAndAWorktreeHasADesk(t *testing.T) {
	isolate(t)
	one := func(id string, change func(*Hire)) Hire {
		h := t3DeskHire(homeDir, id)
		change(&h)
		return h
	}
	t3Registered(t,
		one("00000000-a1", func(h *Hire) { h.T3Thread, h.State = "", "starting" }),
		one("00000000-a2", func(h *Hire) { h.Worktree = "" }),
		one("00000000-a3", func(h *Hire) { h.State = "fired" }),
		one("00000000-a4", func(h *Hire) { h.State = "failed" }),
		one("00000000-a5", func(h *Hire) { h.Engine = "" }),
		t3DeskHire(homeDir, t3DeskTestID))
	var ids []string
	for _, s := range newT3Desks(registryPath()).seated("", func(string) (t3Glance, bool) { return t3Glance{}, false }) {
		ids = append(ids, s.session.ID)
	}
	if !slices.Equal(ids, []string{t3DeskID(t3DeskTestID)}) {
		t.Errorf("%v", ids)
	}
}

func TestT3DeskAnUnregisteredTablessSessionIsStillDropped(t *testing.T) {
	d, home := t3Deck(t)
	// A plain hire in the registry (no engine) and a tabless session in its folder: no desk, as before.
	plain := t3DeskHire(home, "00000000-b1")
	plain.Engine, plain.T3Thread = "", ""
	t3Registered(t, plain)
	hookOn(d, "stray", "SessionStart", plain.Worktree, "", "", 10)
	hookOn(d, "elsewhere", "SessionStart", "/work/elsewhere", "", "", 10)
	// A T3 hire's name or folder alone makes nothing: only its registry entry does.
	hookOn(d, "named", "SessionStart", "/work/rev-abcdef", "", "", 10)
	if knownSession(d, "stray") || knownSession(d, "elsewhere") || knownSession(d, "named") || len(officeDesks(t, d)) != 0 {
		t.Errorf("%v", deskIDs(officeDesks(t, d)))
	}
}

func TestT3DeskTheHiresOwnClaudeSessionMakesNoSecondDesk(t *testing.T) {
	d, home := t3Deck(t, Hire{ID: t3DeskTestID})
	worktree := t3DeskHire(home, t3DeskTestID).Worktree
	desk := t3DeskID(t3DeskTestID)
	// Its own Claude Code, through your hooks, with no tab: dropped, its desk is the registry's.
	hookOn(d, "t3-claude", "SessionStart", worktree, "", worktree, 10)
	hookOn(d, "t3-claude", "PreToolUse", filepath.Join(worktree, "src"), "", "", 11)
	// Even with a tab named (a pane handed on in its environment), it's no desk of its own.
	hookOn(d, "t3-claude-2", "SessionStart", worktree, "tmux:%51@/nowhere/t3-desk-sock", "", 12)
	if knownSession(d, "t3-claude") || knownSession(d, "t3-claude-2") {
		t.Error("the hire's own session is a session")
	}
	if ids := deskIDs(officeDesks(t, d)); !slices.Equal(ids, []string{desk}) {
		t.Errorf("%v", ids)
	}
	// Its events are still in the event log, as any session's the deck has no desk for.
	d.mu.Lock()
	logged := d.events.snapshot()
	d.mu.Unlock()
	if !strings.Contains(string(t3MustJSON(t, logged)), "t3-claude") {
		t.Error("the hire's events aren't logged")
	}
}

func t3MustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// RM5a §7: T3 Code's server started from a terminal hands that terminal's pane to its Claude Code, so the hire's
// events come with the terminal's tab. They must not take that terminal's desk (a new session in a tab replaces the
// one there).
func TestT3DeskAServerStartedFromATerminalDoesntStealThatTerminalsDesk(t *testing.T) {
	d, home := t3Deck(t, Hire{ID: t3DeskTestID})
	worktree := t3DeskHire(home, t3DeskTestID).Worktree
	desk := t3DeskID(t3DeskTestID)
	pane := "tmux:%50@/nowhere/t3-desk-sock"
	hookOn(d, "terminal", "UserPromptSubmit", "/work/server", pane, "", 10)
	before := keyOfDesk(d, "terminal")
	if before < 0 {
		t.Fatal("the terminal has no desk")
	}
	hookOn(d, "t3-claude", "SessionStart", worktree, pane, worktree, 20)
	hookOn(d, "t3-claude", "UserPromptSubmit", worktree, pane, "", 21)
	d.mu.Lock()
	terminal := d.sessions.Get("terminal")
	tab := ""
	if terminal != nil {
		tab = terminal.Tab
	}
	d.mu.Unlock()
	if knownSession(d, "t3-claude") || tab != pane || keyOfDesk(d, "terminal") != before {
		t.Errorf("the terminal lost its desk: %q at %d", tab, keyOfDesk(d, "terminal"))
	}
	ids := deskIDs(officeDesks(t, d))
	slices.Sort(ids)
	if !slices.Equal(ids, []string{desk, "terminal"}) {
		t.Errorf("%v", ids)
	}
	// A session already at a desk that goes into the worktree stays as it was: only a new one is the hire's.
	hookOn(d, "terminal", "PreToolUse", worktree, pane, "", 22)
	d.mu.Lock()
	folder := d.sessions.Get("terminal").Folder
	d.mu.Unlock()
	if folder != worktree {
		t.Errorf("%q", folder)
	}
	// Let go, the worktree is nobody's: a session started there is a desk again, as any.
	if err := updateRegistry(func(f *registryFile) { f.hires[0].State = "fired" }); err != nil {
		t.Fatal(err)
	}
	hookOn(d, "after", "SessionStart", worktree, "tmux:%52@/nowhere/t3-desk-sock", "", 30)
	if !knownSession(d, "after") {
		t.Error("a session in a let-go hire's worktree has no desk")
	}
}

// A t3: id is a T3 desk's alone. A hook event naming one, even with a tab, is no session; nor is one saved.
func TestT3DeskAHookEventCantTakeItsID(t *testing.T) {
	d, _ := t3Deck(t, Hire{ID: t3DeskTestID})
	desk := t3DeskID(t3DeskTestID)
	hookOn(d, desk, "SessionStart", "/work/elsewhere", "tmux:%60@/nowhere/t3-desk-sock", "", 10)
	hookOn(d, desk, "UserPromptSubmit", "/work/elsewhere", "tmux:%60@/nowhere/t3-desk-sock", "", 11)
	if knownSession(d, desk) {
		t.Error("a hook event took a T3 desk's id")
	}
	if ids := deskIDs(officeDesks(t, d)); !slices.Equal(ids, []string{desk}) {
		t.Errorf("the hire's desk, once: %v", ids)
	}
	sessions := NewSessions()
	sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "session_id": "t3:anything", "cwd": "/work/x"}, Claude, "tmux:%61@/x", "", 1)
	sessions.Restore([]savedSession{{ID: "t3:saved", Agent: "claude", Folder: "/work/x", Tab: "tmux:%62@/x"}}, true)
	if sessions.Get("t3:anything") != nil || sessions.Get("t3:saved") != nil {
		t.Error("a t3: session")
	}
}

// A T3 hire's worktree counts (a desk, and new sessions there kept off every desk) only where Smolcorp makes hires'
// worktrees: in <repo>-hires beside its repo, or in the repo's folder inside the folder set for them.
func TestT3DeskOnlyAWorktreeWhereHiresAreMadeCounts(t *testing.T) {
	d, home := t3Deck(t)
	repo := filepath.Join(home, "repo")
	hire := t3DeskHire(home, t3DeskTestID)
	at := func(worktree, worktrees string) bool {
		hire.Worktree = worktree
		return t3InHiresFolder(hire, worktrees)
	}
	hires := filepath.Join(home, "w", "hires")
	for _, ok := range [][2]string{{repo + "-hires/rev-abcdef", ""}, {repo + "-hires/rev-abcdef/", ""},
		{filepath.Join(hires, "repo", "rev-abcdef"), hires}, {repo + "-hires/rev-abcdef", hires}} {
		if !at(ok[0], ok[1]) {
			t.Errorf("refused %v", ok)
		}
	}
	for _, no := range [][2]string{{filepath.Join(hires, "repo", "rev-abcdef"), ""}, {filepath.Join(hires, "other", "rev-abcdef"), hires},
		{repo + "-hires", ""}, {repo + "-hires/rev-abcdef/src", ""}, {repo + "-hires/../repo", ""}, {"/", ""}, {home, ""}, {repo, ""}} {
		if at(no[0], no[1]) {
			t.Errorf("accepted %v", no)
		}
	}
	hire.Repo = ""
	if at(repo+"-hires/rev-abcdef", "") {
		t.Error("no repo on record")
	}
	// A registry row naming a broad folder: no desk, and a session you start under it is a desk as any.
	broad := t3DeskHire(home, t3DeskTestID)
	broad.Worktree = home
	t3Registered(t, broad)
	if len(officeDesks(t, d)) != 0 {
		t.Errorf("%v", deskIDs(officeDesks(t, d)))
	}
	project := filepath.Join(home, "project")
	hookOn(d, "mine", "SessionStart", project, "tmux:%62@/nowhere/t3-desk-sock", project, 10)
	if !knownSession(d, "mine") {
		t.Error("your session under a broad row has no desk")
	}
}

// MARK: Nothing types into it

func TestT3DeskItsControlsAreAbsentAndRefusedNotBroken(t *testing.T) {
	d, _ := t3Deck(t, Hire{ID: t3DeskTestID})
	desk := t3DeskID(t3DeskTestID)
	withMac(t) // under the Mac app too, where a terminal's desk has its controls on
	var shown map[string]any
	for _, one := range officeDesks(t, d) {
		if one["id"] == desk {
			shown = one
		}
	}
	for _, control := range t3ControlsOff {
		if v, ok := shown[control].(bool); !ok || v {
			t.Errorf("%s: %v", control, shown[control])
		}
	}
	if v, has := shown["choice"]; !has || v != nil {
		t.Errorf("choice: %v", v)
	}
	d.mu.Lock()
	d.config.HiringEnabled = true
	d.mu.Unlock()
	for _, action := range []string{"accept", "send", "reject", "esc", "up", "down", "clear", "effort", "button", "scroll", "choose", "fire", "summary", "max"} {
		if status, why := d.Perform(action, desk, url.Values{"level": {"max"}, "phase": {"down"}}); status != http.StatusConflict || why != t3NoControls("Rev") {
			t.Errorf("%s: %d %s", action, status, why)
		}
	}
	// The mic pressed: refused too (its release stops nothing that started).
	if status, _ := d.Perform("mic", desk, url.Values{"phase": {"down"}}); status != http.StatusConflict {
		t.Errorf("mic: %d", status)
	}
	// It isn't the sessions' to find: no key or text can reach it.
	if knownSession(d, desk) {
		t.Error("a session")
	}
	// Its desk isn't carried to another key, which would pin its worktree.
	hookOn(d, "terminal", "UserPromptSubmit", "/work/other", "tmux:%53@/nowhere/t3-desk-sock", "", 10)
	key, other := keyOfDesk(d, desk), keyOfDesk(d, "terminal")
	if key < 0 || other < 0 {
		t.Fatalf("%d %d", key, other)
	}
	if status, why := swapped(d, strconv.Itoa(other), strconv.Itoa(key)); status != http.StatusConflict || why != "Rev runs in T3 Code: its desk stays where it is." {
		t.Errorf("%d %s", status, why)
	}
}

// MARK: Open and Let go

// t3DeskRig is t3DeckRig with a hire in T3 Code at its desk: its worktree fenced, beside its repo.
func t3DeskRig(t *testing.T, flags ...string) (*t3Rig, *Deck, Hire) {
	t.Helper()
	r, d := t3DeckRig(t, flags...)
	h := r.t3Hire(r.worktree())
	h.Nickname, h.Repo, h.T3Link = "Rev", filepath.Join(r.home, "repo"), "https://app.t3.codes/threads/env-1/mcp%3Aabc-1"
	if err := os.MkdirAll(filepath.Join(h.Worktree, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Worktree, ".claude", "settings.local.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t3Registered(t, h)
	return r, d, h
}

func TestT3DeskATapOpensItsOwnThreadInT3Code(t *testing.T) {
	r, d, h := t3DeskRig(t)
	desk := t3DeskID(h.ID)
	d.mu.Lock()
	d.config.T3CodeEnabled = false
	d.mu.Unlock()
	if status, why := d.Focus(desk); status != http.StatusConflict || why != t3Off || len(r.opened) != 0 {
		t.Errorf("T3 Code off: %d %s %v", status, why, r.opened)
	}
	d.mu.Lock()
	d.config.T3CodeEnabled = true
	d.mu.Unlock()
	if status, why := d.Focus(desk); status != http.StatusNoContent || !slices.Equal(r.opened, []string{"t3code://threads/env-1/mcp%3Aabc-1"}) {
		t.Errorf("%d %s %v", status, why, r.opened)
	}
	// Nothing ran t3 for it: the link opens T3 Code.
	for _, run := range r.firsts() {
		if !slices.Equal(run, []string{"thread", "show"}) && !slices.Equal(run, []string{"--version"}) {
			t.Errorf("ran %v", run)
		}
	}
}

func TestT3DeskLetGoFromTheDeskFreesItAndKeepsTheLanesRefusals(t *testing.T) {
	r, d, h := t3DeskRig(t, "removefail")
	desk := t3DeskID(h.ID)
	act := func() (int, string) {
		got := serve(t, d, call{method: "POST", target: "/act?action=letgo&session=" + url.QueryEscape(desk) + "&t=" + d.access.WindowKey,
			host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
		return got.Code, got.Body.String()
	}
	// T3 Code won't remove the project: refused, and the desk stays.
	if status, why := act(); status != http.StatusBadGateway ||
		!strings.HasPrefix(why, "T3 Code didn't remove Rev's project (Could not reach the T3 Code desktop app.)") {
		t.Errorf("%d %s", status, why)
	}
	if keyOfDesk(d, desk) < 0 {
		t.Error("the desk went")
	}
	// Then it goes: fired in the registry, its desk freed.
	r.fake()
	status, body := act()
	var answer map[string]any
	if status != http.StatusOK || json.Unmarshal([]byte(body), &answer) != nil || answer["ok"] != true || answer["worktree"] != h.Worktree {
		t.Fatalf("%d %s", status, body)
	}
	if after, _ := registryHire(h.ID); after.State != "fired" {
		t.Errorf("%+v", after)
	}
	if keyOfDesk(d, desk) >= 0 || len(officeDesks(t, d)) != 0 {
		t.Errorf("%v", deskIDs(officeDesks(t, d)))
	}
	// Again: the desk is gone, so there's nothing to let go.
	if status, _ := act(); status != http.StatusNotFound {
		t.Errorf("%d", status)
	}
}

// MARK: Placement

func TestT3DeskItSitsAtTheDeskTheRegistryKeptAfterARestart(t *testing.T) {
	isolate(t)
	floor, key := 0, 3
	h := t3DeskHire(homeDir, t3DeskTestID)
	h.T3Floor, h.T3Desk = &floor, &key
	t3Registered(t, h)
	desk := t3DeskID(t3DeskTestID)
	first := startDeck(t)
	if at := keyOfDesk(first, desk); at != 3 {
		t.Errorf("the desk held at hire: %d", at)
	}
	// A restart: a new deck reads the same registry.
	again := startDeck(t)
	if at := keyOfDesk(again, desk); at != 3 {
		t.Errorf("after a restart: %d", at)
	}
	// Upstairs: on its floor, at its desk.
	up, upDesk := 2, 4
	if err := updateRegistry(func(f *registryFile) { f.hires[0].T3Floor, f.hires[0].T3Desk = &up, &upDesk }); err != nil {
		t.Fatal(err)
	}
	upper := startDeck(t)
	if _, floors := seating(upper); !slices.Contains(floors[2], desk) {
		t.Errorf("%v", floors)
	}
	for _, one := range floorDesks(t, upper, 2) {
		if one["id"] == desk && one["desk"] != float64(4) {
			t.Errorf("%v", one["desk"])
		}
		if one["id"] == desk && (one["headless"] != false || one["drivable"] != true) {
			t.Errorf("picked by a tap: %v", one)
		}
	}
}

func TestT3DeskWhereItSitsIsKeptInTheRegistryAndAMoveToo(t *testing.T) {
	d, _ := t3Deck(t, Hire{ID: t3DeskTestID})
	desk := t3DeskID(t3DeskTestID)
	d.Refresh()
	key := keyOfDesk(d, desk)
	h, _ := registryHire(t3DeskTestID)
	if key < 0 || h.T3Floor == nil || *h.T3Floor != 0 || h.T3Desk == nil || *h.T3Desk != key {
		t.Fatalf("%d %+v", key, h)
	}
	// Put on 3F from its card: there it stays, kept in the registry.
	if status, why := d.Perform("move", desk, url.Values{"floor": {"2"}}); status != http.StatusNoContent {
		t.Fatalf("%d %s", status, why)
	}
	h, _ = registryHire(t3DeskTestID)
	if _, floors := seating(d); h.T3Floor == nil || *h.T3Floor != 2 || h.T3Desk == nil || !slices.Contains(floors[2], desk) || keyOfDesk(d, desk) >= 0 {
		t.Errorf("%+v %v", h, floors)
	}
	// The team pairs the desk with its hire, with no row of its own.
	rows := buildTeam(readRegistry(registryPath(), false).hires, d.frontDeskLive(), 0, func(string) string { return "" })
	if len(rows) != 1 || rows[0]["id"] != t3DeskTestID || rows[0]["sessionId"] != desk || rows[0]["floor"] != 2 {
		t.Errorf("%v", rows)
	}
	if found, ok := d.hireInSession(desk); !ok || found.ID != t3DeskTestID {
		t.Error("the desk's hire")
	}
	// And back down, at a free key.
	if status, why := d.Perform("move", desk, url.Values{"floor": {"0"}}); status != http.StatusNoContent || keyOfDesk(d, desk) < 0 {
		t.Errorf("%d %s", status, why)
	}
}

// FXT3Da B1 (RT3Da's P1): you already work in the folder the hire was made from. The hire's row is its own desk,
// not your session; your session is a row of its own; the desk has no second row. And no other hire takes a T3
// desk, even one made in its worktree (RT3Da2 n1).
func TestT3DeskAnOlderSessionInTheFolderItWasHiredFromIsntItsDesk(t *testing.T) {
	d, home := t3Deck(t)
	original := filepath.Join(home, "repo")
	h := t3DeskHire(home, t3DeskTestID)
	plain := Hire{ID: "00000000-c1", Name: "Plain", Role: "reviewer", Folder: h.Worktree, State: "working"}
	t3Registered(t, plain, h)
	desk := t3DeskID(t3DeskTestID)
	hookOn(d, "owner", "SessionStart", original, "tmux:%63@/nowhere/t3-desk-sock", original, 5)
	if !knownSession(d, "owner") {
		t.Fatal("your session")
	}
	rows := buildTeam(readRegistry(registryPath(), false).hires, d.frontDeskLive(), 0, func(string) string { return "" })
	byID := map[any]map[string]any{}
	for _, row := range rows {
		byID[row["id"]] = row
	}
	if len(rows) != 3 || byID[t3DeskTestID]["sessionId"] != desk || byID["00000000-c1"]["sessionId"] != "" ||
		byID["live:owner"]["registered"] != false || byID["live:"+desk] != nil {
		t.Errorf("%v", rows)
	}
	if found, ok := d.hireInSession(desk); !ok || found.ID != t3DeskTestID {
		t.Error("the desk's hire")
	}
	if _, ok := d.hireInSession("owner"); ok {
		t.Error("your session is a hire's")
	}
}

func TestT3DeskOnlyAT3HiresRecordSaysWhereItSits(t *testing.T) {
	isolate(t)
	floor, desk := 1, 2
	plain := t3DeskHire(homeDir, "00000000-d1")
	plain.Engine, plain.T3Thread, plain.T3Link = "", "", ""
	plain.T3Floor, plain.T3Desk = &floor, &desk
	t3 := t3DeskHire(homeDir, t3DeskTestID)
	t3.T3Floor, t3.T3Desk = &floor, &desk
	t3Registered(t, plain, t3)
	data, err := os.ReadFile(registryPath())
	if err != nil {
		t.Fatal(err)
	}
	var file struct{ Hires []map[string]any }
	if err := json.Unmarshal(data, &file); err != nil || len(file.Hires) != 2 {
		t.Fatalf("%v %s", err, data)
	}
	if _, has := file.Hires[0]["t3Floor"]; has {
		t.Error("a terminal hire's record has t3Floor")
	}
	if _, has := file.Hires[0]["t3Desk"]; has {
		t.Error("a terminal hire's record has t3Desk")
	}
	back, _ := registryHire(t3DeskTestID)
	if back.T3Floor == nil || *back.T3Floor != 1 || back.T3Desk == nil || *back.T3Desk != 2 {
		t.Errorf("%+v", back)
	}
}

// While T3 Code is off, t3 is never run: after the follow's first tick nothing can say the hire is at work, so its
// desk is white, as the Mac's is when T3 Code isn't there; the registry is left as it said.
func TestT3DeskIsWhileT3CodeIsOffAsTheMacsWhenT3CodeIsntThere(t *testing.T) {
	d, _ := t3Deck(t, Hire{ID: t3DeskTestID})
	desk := t3DeskID(t3DeskTestID)
	state := func() (any, any) {
		for _, one := range officeDesks(t, d) {
			if one["id"] == desk {
				return one["state"], one["activity"]
			}
		}
		t.Fatal("no desk for the hire")
		return nil, nil
	}
	// Before any look, as the registry says, as the Mac's first snapshot.
	if s, a := state(); s != "thinking" || a != "At work in T3 Code" {
		t.Errorf("before a look: %v %v", s, a)
	}
	d.t3FollowTick(registryPath())
	if s, a := state(); s != "idle" || a != nil {
		t.Errorf("T3 Code off: %v %v", s, a)
	}
	if hires := readRegistry(registryPath(), false).hires; len(hires) != 1 || hires[0].State != "working" {
		t.Errorf("the registry: %+v", hires)
	}
}
