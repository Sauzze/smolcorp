package desktop

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The GM's hire, fire and brief routes on the launcher's way: what Windows hires through (hirelauncher.go).
// No build tag, so the Windows CI job runs them; elsewhere they run with launcherHires switched on.
// Nothing real is opened, started or ended: the window opener, the launcher, git and the guard are stand-ins,
// and the hire has no worktree (the stand-in git says no folder is a repository).

// plainOpened counts the calls to the stand-in terminal's args: a hire in a plain terminal.
var plainOpened atomic.Int32

const winAsGM, winAsPage = "gm", "page"

func winRegistry(t *testing.T) []Hire { t.Helper(); return readRegistry(registryPath(), false).hires }

const winGuardPrinted = `{"groups":[{"name":"push","on":true,"commands":["git push"]}],"file":"x","problem":""}`

// stubLauncher is the launcher of one hire's window: it records what it is told and answers a look with what shows.
type stubLauncher struct {
	mu          sync.Mutex
	stops       int
	typed       []string
	pasted      []string
	pressed     int // keystrokes pressed: not typed text, not a paste
	shows       PaneState
	silent      bool // never answers a look
	staysOnStop bool // a stop that doesn't end it
}

func (s *stubLauncher) pressCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pressed
}

func (s *stubLauncher) seen() (stops int, typed, pasted []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stops, slices.Clone(s.typed), slices.Clone(s.pasted)
}

type winRig struct {
	t         *testing.T
	d         *Deck
	token     string
	mu        sync.Mutex
	opened    [][]string
	launchers map[string]*stubLauncher // by hire id
	shows     PaneState                // what a window opened from now on shows
	settings  map[string]string        // the settings each hire's start environment held, by hire id
}

func swap[T any](t *testing.T, target *T, with T) {
	t.Helper()
	was := *target
	*target = with
	t.Cleanup(func() { *target = was })
}

func newWinRig(t *testing.T) *winRig {
	t.Helper()
	home := isolate(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "no-gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(home, "no-gitconfig"))
	writeConfig(t, home, "{}")
	// Where hires can open: one terminal and no tmux, as on Windows; every stand-in in before the deck starts.
	swap(t, &launcherHires, true)
	// A plain terminal is never opened by these routes: its args records the call, and each test checks none was made.
	swap(t, &installedOpeners, func() []hireOpener {
		return []hireOpener{{id: "term", name: "Term", args: func(folder string, command []string) []string {
			plainOpened.Add(1)
			return []string{os.Args[0], "-test.run=^$"} // if it ever runs, it runs no test and ends at once
		}}}
	})
	swap(t, &tmuxPath, func() string { return "" })
	swap(t, &canOpenWindows, func() bool { return true })
	swap(t, &agentMissing, func(string) string { return "" })
	swap(t, &hooksNotConnected, func(Agent) string { return "" })
	plainOpened.Store(0)
	t.Cleanup(func() {
		if n := plainOpened.Load(); n != 0 {
			t.Errorf("a plain terminal was opened %d time(s)", n)
		}
	})
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	w := &winRig{t: t, d: d, launchers: map[string]*stubLauncher{}, shows: PaneIdle}
	folder := filepath.Join(home, "proj")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	d.config.HiringEnabled, d.config.HireManagedWindows, d.config.HireFolder = true, true, physical(folder)
	d.config.Roles = []Role{{ID: "janitor", Name: "Janitor", Agent: "claude", Model: "sonnet"}, {ID: "gm", Name: "General Manager", Agent: "claude", Model: "sonnet"}}
	d.launcherEngine = launcherEngineOptions{
		Git:      func(string, ...string) (int, string, bool) { return 1, "", false },
		PortFree: func(int) bool { return true }, Key: func() string { return placeTestKey },
		Guard:  func(string, []string) (int, string, bool) { return 0, winGuardPrinted, true },
		IsExec: func(string) bool { return true }, CLI: filepath.Join(home, "bin", "smolcorp"), Worktrees: t.TempDir(),
		Open: w.open, HeardFrom: func(Hire) bool { return true },
		Within: time.Second, Every: 5 * time.Millisecond, LookWait: time.Second, StopWait: time.Second,
	}
	managerOn(d)
	w.token = d.newGMToken()
	return w
}

func after(command []string, flag string) string {
	if i := slices.Index(command, flag); i >= 0 && i+1 < len(command) {
		return command[i+1]
	}
	return ""
}

// open is the window opener: it records the command, and the launcher it would run connects with the hire's token.
func (w *winRig) open(command []string, folder string) error {
	token, id := after(command, "--hire"), after(command, "--hire-id")
	s := &stubLauncher{shows: w.shows}
	settings := launcherSettings(w.d, id)
	w.mu.Lock()
	if w.settings == nil {
		w.settings = map[string]string{}
	}
	w.settings[id] = settings
	w.opened = append(w.opened, slices.Clone(command))
	w.launchers[id] = s
	w.mu.Unlock()
	lid := "L-" + id
	tab := w.d.launcherConnected(lid, Claude, folder, after(command, "--role"), token)
	if tab == nil {
		w.t.Fatal("launcher not connected")
	}
	done := make(chan struct{})
	w.t.Cleanup(func() { close(done) })
	go func() {
		for {
			select {
			case c := <-tab.commands:
				ends := false
				s.mu.Lock()
				switch {
				case c.Look != "":
					if !s.silent {
						w.d.launcherScreenAnswered(lid, tab.screenSecret, screenAnswer{Look: c.Look, State: s.shows.String()})
					}
				case c.Stop:
					s.stops++
					ends = !s.staysOnStop
				case c.Paste != "":
					s.pasted = append(s.pasted, c.Paste)
				case c.Press != nil:
					s.pressed++
				default:
					s.typed = append(s.typed, c.Type)
				}
				s.mu.Unlock()
				if ends {
					w.d.launcherGone(lid, tab)
				}
			case <-tab.gone:
				return
			case <-done:
				return
			}
		}
	}()
	return nil
}

func (w *winRig) post(path string, body map[string]any, as string) (int, string) {
	w.t.Helper()
	data, _ := json.Marshal(body)
	c := call{method: "POST", target: path, host: "localhost", from: thisComputer, body: string(data), headers: map[string]string{setupHeader: "1"}}
	if as == winAsGM {
		c.withoutSetupKey, c.headers[gmTokenHeader] = true, w.token
	}
	r := serve(w.t, w.d, c)
	w.d.watching.Wait()
	return r.Code, strings.TrimSpace(r.Body.String())
}

// hire hires role through the route, and returns its registry row.
func (w *winRig) hire(role string) Hire {
	w.t.Helper()
	w.d.mu.Lock()
	w.d.lastHire = time.Time{}
	w.d.mu.Unlock()
	as := winAsGM
	if role == "gm" {
		as = winAsPage // only the page hires the gm role
	}
	if status, text := w.post("/setup/gm/hire", map[string]any{"role": role}, as); status != http.StatusOK {
		w.t.Fatalf("hire %s: %d %s", role, status, text)
	}
	rows := winRegistry(w.t)
	if len(rows) == 0 || rows[len(rows)-1].Role != role || !rows[len(rows)-1].ViaLauncher {
		w.t.Fatalf("registry %+v", rows)
	}
	return rows[len(rows)-1]
}

// With the beta switch on, the route's hire and the office's start the window with the same guard and deny settings: each hire-guard.json case, as is and each group turned.
func TestTheGMHireOnTheLauncherStartsItsWindowWithTheGuardOfAHireFromTheOffice(t *testing.T) {
	var file struct{ Groups []map[string]any }
	if err := json.Unmarshal(frontDeskFixture(t, "hire-guard.json"), &file); err != nil || len(file.Groups) == 0 {
		t.Fatalf("hire-guard.json: %d groups, %v", len(file.Groups), err)
	}
	cases := [][]map[string]any{file.Groups}
	for i, g := range file.Groups {
		turned := slices.Clone(file.Groups)
		turned[i] = map[string]any{"name": g["name"], "commands": g["commands"], "on": g["on"] != true}
		cases = append(cases, turned)
	}
	// Both hires of a case are made in one rig, so they share the support, home and repo folders and differ only by
	// the hire's id (plain letters and digits, the same in every path style): nothing here depends on how a path is written.
	hire := func(w *winRig, role string, groups []map[string]any, route bool) (settings string, guard []string) {
		printed, _ := json.Marshal(map[string]any{"groups": groups})
		w.d.launcherEngine.Guard = func(_ string, args []string) (int, string, bool) {
			guard = append(guard, strings.Join(args, " "))
			return 0, string(printed), true
		}
		before := len(winRegistry(t))
		if err := updateRegistry(func(f *registryFile) { // only one gm works at a time: the first hire of the case is over
			for i := range f.hires {
				f.hires[i].State = "fired"
			}
		}); err != nil {
			t.Fatal(err)
		}
		w.d.mu.Lock()
		w.d.lastHire = time.Time{}
		w.d.mu.Unlock()
		if route {
			w.hire(role)
		} else if status, text := w.d.Perform("hire", "", url.Values{"role": {role}}); status != http.StatusNoContent {
			t.Fatalf("desk hire of %s: %d %q", role, status, text)
		}
		w.d.watching.Wait()
		rows := winRegistry(t)
		if len(rows) != before+1 || len(w.opened) != before+1 || slices.ContainsFunc(w.opened[before], func(a string) bool { return strings.HasPrefix(a, "--settings") }) {
			t.Fatalf("%s: opened %q, registry %+v", role, w.opened, rows)
		}
		id := rows[before].ID
		w.mu.Lock()
		data := w.settings[id]
		w.mu.Unlock()
		os.RemoveAll(hireFolder(supportDir, id)) // a hire's settings deny the folders of the hires beside it: the next one finds none
		for i := range guard {
			guard[i] = strings.ReplaceAll(guard[i], id, "<hire>")
		}
		return strings.ReplaceAll(data, id, "<hire>"), guard
	}
	for _, role := range []string{"janitor", "gm"} {
		var first string
		for i, groups := range cases {
			w := newWinRig(t)
			// A hire keeps its brief in the support folder's briefs (M4a), and the deny rules list the folders
			// found there before the ones only known: made first, so both hires find the same folders.
			if err := os.MkdirAll(briefsKeptDir(), 0o700); err != nil {
				t.Fatal(err)
			}
			// The same for config.json, which a hire saves (setup's recent folders, hiredInto).
			w.d.mu.Lock()
			w.d.config.save()
			w.d.mu.Unlock()
			deskSettings, deskGuard := hire(w, role, groups, false)
			routeSettings, routeGuard := hire(w, role, groups, true)
			if deskSettings != routeSettings || !slices.Equal(deskGuard, routeGuard) || len(routeGuard) == 0 {
				t.Errorf("%s, case %d: settings or guard differ:\ndesk:  %s %q\nroute: %s %q", role, i, deskSettings, deskGuard, routeSettings, routeGuard)
			}
			if !strings.Contains(routeSettings, "PreToolUse") || (role == "gm") != strings.Contains(routeSettings, `"--profile","gm"`) || i > 0 && routeSettings == first {
				t.Errorf("%s, case %d: no guard hook, the wrong profile, or a group turned the other way changed nothing: %s", role, i, routeSettings)
			}
			if i == 0 {
				first = routeSettings
			}
		}
	}
}

// With the switch off the route refuses in one sentence that names it, and opens nothing: never a plain terminal.
func TestTheGMHireWithManagedWindowsOffRefusesNamingTheSwitchAndOpensNothing(t *testing.T) {
	w := newWinRig(t)
	w.d.config.HireManagedWindows = false
	status, text := w.post("/setup/gm/hire", map[string]any{"role": "janitor"}, winAsGM)
	if status != http.StatusConflict || !strings.Contains(text, "managed windows") || strings.Contains(text, "\n") {
		t.Errorf("%d %q, want 409 naming managed windows", status, text)
	}
	if len(w.opened) != 0 || len(winRegistry(t)) != 0 {
		t.Errorf("a refused hire opened %q, registry %+v", w.opened, winRegistry(t))
	}
}

// B2 (RPGS), the launcher's twin: on a floor with a folder of its own the route refuses in one sentence and opens
// nothing, with managed windows on; the office's hire there would open a plain window.
func TestTheGMHireOnTheLauncherOnAFloorWithItsOwnFolderIsRefusedAndOpensNothing(t *testing.T) {
	w := newWinRig(t)
	w.d.config.FloorRules = checkedFloorRules([]FloorRule{{Folder: w.d.config.HireFolder, Floor: 1}})
	status, text := w.post("/setup/gm/hire", map[string]any{"role": "janitor", "floor": 1}, winAsGM)
	if want := "The General Manager doesn't hire into a pinned desk's or a floor's own folder."; status != http.StatusConflict || text != want {
		t.Errorf("%d %q, want 409 %q", status, text, want)
	}
	if len(w.opened) != 0 || len(winRegistry(t)) != 0 {
		t.Errorf("a refused hire opened %q, registry %+v", w.opened, winRegistry(t))
	}
}

// standInWorktree gives the launcher hire h a worktree of its own in a stand-in repo, as the launcher would have
// made it, and a stand-in git that says it's clean and records what else it's asked: the row as it now reads,
// and what git was asked.
func (w *winRig) standInWorktree(h Hire) (Hire, *[]string) {
	w.t.Helper()
	repo := w.d.config.HireFolder
	tree := filepath.Join(hiresFolder(repo, w.d.launcherEngine.Worktrees), "janitor-"+h.ID)
	if err := os.MkdirAll(tree, 0o755); err != nil {
		w.t.Fatal(err)
	}
	h.Repo, h.Worktree, h.Branch = repo, physical(tree), "hire/"+filepath.Base(tree)
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			if f.hires[i].ID == h.ID {
				// Hired in a repository, it has no "isn't a git repository" note either.
				f.hires[i].Repo, f.hires[i].Worktree, f.hires[i].Branch, f.hires[i].Notes = h.Repo, h.Worktree, h.Branch, ""
			}
		}
	}); err != nil {
		w.t.Fatal(err)
	}
	var mu sync.Mutex
	asked := &[]string{}
	w.d.mu.Lock()
	w.d.launcherEngine.Git = func(dir string, args ...string) (int, string, bool) {
		switch {
		case slices.Equal(args, []string{"status", "--porcelain"}):
			return 0, "", true
		case len(args) > 0 && args[0] == "symbolic-ref":
			return 0, h.Branch, true
		}
		mu.Lock()
		*asked = append(*asked, strings.Join(args, " "))
		mu.Unlock()
		return 0, "", true
	}
	w.d.mu.Unlock()
	return h, asked
}

// N3 (RPGS): the launcher's fire keeps a clean worktree unless asked to remove it, as the Mac's: a plain fire
// removes nothing and writes no note; removal asked for is git's worktree remove. Either way 204.
func TestTheGMFireOnTheLauncherKeepsTheCleanWorktreeUnlessAskedToRemoveIt(t *testing.T) {
	for _, c := range []struct {
		extra  map[string]any
		remove bool
	}{{map[string]any{}, false}, {map[string]any{"removeWorktree": false}, false}, {map[string]any{"removeWorktree": true}, true}} {
		w := newWinRig(t)
		h, asked := w.standInWorktree(w.hire("janitor"))
		body := map[string]any{"id": h.ID}
		maps.Copy(body, c.extra)
		status, text := w.post("/setup/gm/fire", body, winAsGM)
		removed := slices.ContainsFunc(*asked, func(a string) bool { return strings.HasPrefix(a, "worktree remove") })
		rows := winRegistry(t)
		if status != http.StatusNoContent || text != "" || removed != c.remove || len(rows) != 1 || rows[0].State != "fired" || rows[0].Notes != "" {
			t.Errorf("%v: %d %q, git asked %q, registry %+v; want 204, removed %v, fired with no note", c.extra, status, text, *asked, rows, c.remove)
		}
		if _, err := os.Stat(h.Worktree); err != nil {
			t.Errorf("%v: the worktree folder went: %v", c.extra, err)
		}
	}
}

// Fire ends only the recorded launcher of the hire asked for, found by registry id, and types nothing, with or without force.
func TestTheGMFireOnTheLauncherEndsOnlyTheHiresOwnLauncherAndTypesNothing(t *testing.T) {
	for _, force := range []bool{false, true} {
		w := newWinRig(t)
		a, b := w.hire("janitor"), w.hire("janitor")
		if status, text := w.post("/setup/gm/fire", map[string]any{"id": a.ID, "force": force}, winAsGM); status != http.StatusNoContent {
			t.Fatalf("force %v: %d %s", force, status, text)
		}
		stopsA, typedA, pastedA := w.launchers[a.ID].seen()
		stopsB, typedB, pastedB := w.launchers[b.ID].seen()
		if stopsA != 1 || stopsB != 0 || len(typedA)+len(pastedA)+len(typedB)+len(pastedB) != 0 {
			t.Errorf("force %v: stops %d and %d, typed %q %q, pasted %q %q", force, stopsA, stopsB, typedA, typedB, pastedA, pastedB)
		}
		rows := map[string]string{}
		for _, h := range winRegistry(t) {
			rows[h.ID] = h.State
		}
		if rows[a.ID] != "fired" || rows[b.ID] != "working" {
			t.Errorf("force %v: states %v", force, rows)
		}
		if status, text := w.post("/setup/gm/fire", map[string]any{"id": a.ID}, winAsGM); status != http.StatusConflict || !strings.Contains(text, "already gone") {
			t.Errorf("fired again: %d %q", status, text)
		}
	}
}

// What the launcher reports decides: a question or a window it can't read is refused even with force; a busy window or a draft only without it. Nothing is ended or typed.
func TestTheGMFireOnTheLauncherRefusesFromTheLaunchersStateAndAnUnknownIdAndEndsNothing(t *testing.T) {
	for _, c := range []struct {
		name   string
		shows  PaneState
		silent bool
		force  bool
		want   int
		says   string
	}{
		{"busy", PaneBusy, false, false, 409, "working right now"},
		{"draft", PaneDraft, false, false, 409, "isn't sent yet"},
		{"question", PaneMenu, false, true, 409, "waiting for an answer"},
		{"unreadable", PaneIdle, true, false, 409, "Couldn't tell"},
		{"unreadable, forced", PaneIdle, true, true, 409, "Couldn't tell"},
		{"busy, forced", PaneBusy, false, true, 204, ""},
	} {
		w := newWinRig(t)
		w.shows = c.shows
		h := w.hire("janitor")
		w.launchers[h.ID].mu.Lock()
		w.launchers[h.ID].silent = c.silent
		w.launchers[h.ID].mu.Unlock()
		status, text := w.post("/setup/gm/fire", map[string]any{"id": h.ID, "force": c.force}, winAsGM)
		stops, typed, pasted := w.launchers[h.ID].seen()
		if status != c.want || !strings.Contains(text, c.says) || (stops == 1) != (c.want == 204) || len(typed)+len(pasted) != 0 {
			t.Errorf("%s: %d %q, stops %d, typed %q %q, want %d %q", c.name, status, text, stops, typed, pasted, c.want, c.says)
		}
	}
	w := newWinRig(t)
	h := w.hire("janitor")
	for _, c := range []struct {
		body map[string]any
		want int
	}{{map[string]any{"id": "nosuchhire00"}, 404}, {map[string]any{"id": "live:Janitor"}, 404}, {map[string]any{}, 400}} {
		if status, text := w.post("/setup/gm/fire", c.body, winAsGM); status != c.want {
			t.Errorf("%v: %d %q, want %d", c.body, status, text, c.want)
		}
	}
	if stops, _, _ := w.launchers[h.ID].seen(); stops != 0 {
		t.Errorf("a refused fire stopped the launcher %d times", stops)
	}
}

// A hire that isn't a launcher's window (a tmux one) is refused on Windows in one sentence and nothing is ended; a launcher that stays up on a stop is no fire.
func TestTheGMFireRefusesAHireThatIsNoLaunchersWindowAndALauncherThatWontStop(t *testing.T) {
	w := newWinRig(t)
	other := Hire{ID: "tmuxhire0001", Name: "Janitor", Role: "janitor", State: "working", Folder: homeDir, TmuxSocket: "x.sock"}
	if err := updateRegistry(func(f *registryFile) { f.hires = append(f.hires, other) }); err != nil {
		t.Fatal(err)
	}
	h := w.hire("janitor")
	if status, text := w.post("/setup/gm/fire", map[string]any{"id": other.ID, "force": true}, winAsGM); status != http.StatusConflict || !strings.Contains(text, "Couldn't reach") {
		t.Errorf("a tmux hire: %d %q, want 409", status, text)
	}
	w.launchers[h.ID].mu.Lock()
	w.launchers[h.ID].staysOnStop = true
	w.launchers[h.ID].mu.Unlock()
	if status, text := w.post("/setup/gm/fire", map[string]any{"id": h.ID}, winAsGM); status != http.StatusConflict || !strings.Contains(text, "Couldn't end") {
		t.Errorf("a launcher that stays: %d %q, want 409", status, text)
	}
	for _, row := range winRegistry(t) {
		if row.State == "fired" {
			t.Errorf("%s was marked fired", row.ID)
		}
	}
}

// Brief is a file and a mail: no BRIEF file is 409; with one, the dated part is added, the answer carries the queued mail, and nothing is typed in the window.
func TestTheGMBriefOnTheLauncherAddsToTheFileAndTypesNothing(t *testing.T) {
	w := newWinRig(t)
	h := w.hire("janitor")
	body := map[string]any{"id": h.ID, "text": "Also check the logs."}
	if status, text := w.post("/setup/gm/brief", body, winAsGM); status != http.StatusConflict || !strings.Contains(text, "no BRIEF file") {
		t.Fatalf("no BRIEF file: %d %q", status, text)
	}
	brief := filepath.Join(t.TempDir(), "BRIEF.md")
	if err := os.WriteFile(brief, []byte("# Brief\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := updateRegistry(func(f *registryFile) {
		for i := range f.hires {
			f.hires[i].Brief = brief
		}
	}); err != nil {
		t.Fatal(err)
	}
	status, text := w.post("/setup/gm/brief", body, winAsGM)
	var answer struct {
		OK   bool
		Mail struct{ ID, State, To string }
	}
	if err := json.Unmarshal([]byte(text), &answer); err != nil || status != http.StatusOK || !answer.OK || answer.Mail.State != "queued" || answer.Mail.To != h.ID || !strings.HasPrefix(answer.Mail.ID, "brief-") {
		t.Fatalf("brief: %d %q, want 200 with a queued mail to %s", status, text, h.ID)
	}
	data, _ := os.ReadFile(brief)
	if !strings.Contains(string(data), "## New part, ") || !strings.Contains(string(data), "Also check the logs.") {
		t.Errorf("the file reads %q", data)
	}
	_, typed, pasted := w.launchers[h.ID].seen()
	if len(typed) != 0 || w.launchers[h.ID].pressCount() != 0 {
		t.Errorf("typed %q and pressed %d keys in the window", typed, w.launchers[h.ID].pressCount())
	}
	for _, p := range pasted {
		if strings.Contains(p, "Also check the logs.") {
			t.Errorf("the brief's text was pasted into the window: %q", p)
		}
	}
}

// Every saved account's settings and transcripts are denied to a hire through the launcher too, from the office and
// from the GM's route, as in tmux: one inside the home folder (an account a hire may work on) and one outside it
// (no account a hire takes, but fenced off all the same). A badly formed entry gets no rule.
func TestEverySavedAccountsFolderIsDeniedToALauncherHire(t *testing.T) {
	w := newWinRig(t)
	work := claudeFolder(t, filepath.Join(homeDir, "accounts", "work"), "settings.json")
	outside := claudeFolder(t, filepath.Join(physical(t.TempDir()), "volume", ".claude-ext"), "settings.json")
	w.d.mu.Lock()
	w.d.config.ClaudeProfiles = append([]claudeProfile{{Name: "work", Dir: work}, {Name: "ext", Dir: outside}}, badlyFormedAccounts...)
	w.d.mu.Unlock()
	for _, route := range []bool{false, true} {
		where := map[bool]string{false: "the office's hire", true: "the GM's route"}[route]
		before := len(winRegistry(t))
		w.d.mu.Lock()
		w.d.lastHire = time.Time{}
		w.d.mu.Unlock()
		if route {
			// With two accounts it names one, as it must on the Mac (C05).
			if status, text := w.post("/setup/gm/hire", map[string]any{"role": "janitor", "account": "default"}, winAsGM); status != http.StatusOK {
				t.Fatalf("%s: %d %s", where, status, text)
			}
		} else {
			// The office's hire can't name one: with two it is asked which, in the Mac's words, and nothing is made.
			if status, text := w.d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusBadRequest ||
				text != "Say which Claude account to hire on: default, work." || len(winRegistry(t)) != before {
				t.Fatalf("%s with two accounts: %d %q", where, status, text)
			}
			// With work not saved, only the default is left to work on: hired, and the one outside still fenced off.
			w.d.mu.Lock()
			saved := w.d.config.ClaudeProfiles
			w.d.config.ClaudeProfiles = slices.DeleteFunc(slices.Clone(saved), func(p claudeProfile) bool { return p.Name == "work" })
			w.d.lastHire = time.Time{}
			w.d.mu.Unlock()
			status, text := w.d.Perform("hire", "", url.Values{"role": {"janitor"}})
			w.d.mu.Lock()
			w.d.config.ClaudeProfiles = saved
			w.d.mu.Unlock()
			if status != http.StatusNoContent {
				t.Fatalf("%s: %d %q", where, status, text)
			}
		}
		w.d.watching.Wait()
		rows := winRegistry(t)
		if len(rows) != before+1 || !rows[before].ViaLauncher {
			t.Fatalf("%s: registry %+v", where, rows)
		}
		w.mu.Lock()
		settings := w.settings[rows[before].ID]
		w.mu.Unlock()
		if settings == "" {
			t.Fatalf("%s: no settings with the start environment", where)
		}
		if route {
			deniesTheAccounts(t, where, settings, work, outside)
		} else {
			deniesTheAccounts(t, where, settings, outside)
		}
	}
}

// RX1e N2: the route's task and account reach a hire made through the launcher (managed windows on): the task is in
// the hire's brief, the account is the one it works on (the row's, and the launcher's environment), and a task
// of 8000 characters or an account that isn't there is refused with nothing opened, as on the engine's way.
func TestTheGMHireOnTheLauncherCarriesItsTaskAndAccountToTheHire(t *testing.T) {
	w := newWinRig(t)
	work := claudeFolder(t, filepath.Join(homeDir, "accounts", "work"), "settings.json", ".claude.json")
	w.d.mu.Lock()
	w.d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: work}}
	// A role with a brief of its own (the stand-in janitor has none to put a task in).
	w.d.config.Roles[0].Brief = &RoleBrief{Kind: "library", ID: "tester"}
	w.d.mu.Unlock()
	// What the launcher is told to set on the agent: read as the window is opened, before it connects and takes it.
	var env hireStartEnv
	open := w.d.launcherEngine.Open
	w.d.launcherEngine.Open = func(command []string, folder string) error {
		w.d.mu.Lock()
		if pending := w.d.hires[after(command, "--hire")]; pending != nil && pending.env != nil {
			env = *pending.env
		}
		w.d.mu.Unlock()
		return open(command, folder)
	}
	const task = "The lost cent is fixed; proof: python3 -m unittest passes."
	// Refused before anything is made.
	for _, c := range []struct {
		body map[string]any
		want string
	}{
		{map[string]any{"role": "janitor", "task": strings.Repeat("x", briefTaskLimit+1)}, "The task is at most 8000 characters."},
		{map[string]any{"role": "janitor", "account": "nobody"}, "There's no Claude account called nobody. Pick one of: default, work."},
	} {
		if status, text := w.post("/setup/gm/hire", c.body, winAsGM); status < 400 || text != c.want {
			t.Errorf("%.60v: %d %q, want a refusal %q", c.body, status, text, c.want)
		}
	}
	if len(w.opened) != 0 || len(winRegistry(t)) != 0 {
		t.Fatalf("a refused hire opened %q, registry %+v", w.opened, winRegistry(t))
	}
	// Hired, through the launcher, with both.
	status, text := w.post("/setup/gm/hire", map[string]any{"role": "janitor", "task": task, "account": "work"}, winAsGM)
	if status != http.StatusOK {
		t.Fatalf("hire: %d %q", status, text)
	}
	rows := winRegistry(t)
	if len(rows) != 1 || !rows[0].ViaLauncher || len(w.opened) != 1 {
		t.Fatalf("not made by the launcher: opened %q, registry %+v", w.opened, rows)
	}
	if rows[0].Account != "work" {
		t.Errorf("the row's account is %q, want work", rows[0].Account)
	}
	brief, err := os.ReadFile(rows[0].Brief)
	if err != nil || !strings.Contains(string(brief), task) {
		t.Errorf("the brief %q doesn't hold the task (%v):\n%s", rows[0].Brief, err, brief)
	}
	if env.Set["CLAUDE_CONFIG_DIR"] != work {
		t.Errorf("the launcher's environment sets CLAUDE_CONFIG_DIR to %q, want %q (%+v)", env.Set["CLAUDE_CONFIG_DIR"], work, env)
	}
}
