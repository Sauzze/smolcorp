//go:build darwin

package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// scriptedHost stands in for the Mac app, answering each action its own way.
// It never tells a terminal anything.
type scriptedHost struct {
	asked   []bridgeRequest
	answers map[string]func(bridgeRequest) (bridgeAnswer, error)
}

func (s *scriptedHost) call(r bridgeRequest) (bridgeAnswer, error) {
	s.asked = append(s.asked, r)
	if answer, ok := s.answers[r.Action]; ok {
		return answer(r)
	}
	return bridgeAnswer{OK: false, Error: "not scripted"}, nil
}

func (s *scriptedHost) actions() []string {
	var names []string
	for _, r := range s.asked {
		names = append(names, r.Action)
	}
	return names
}

// withMacOpeners puts the Mac's openers in place, on a stand-in app, for one test.
func withMacOpeners(t *testing.T, installed []string) *scriptedHost {
	t.Helper()
	saved := installedOpeners
	t.Cleanup(func() { installedOpeners = saved })
	app := &scriptedHost{answers: map[string]func(bridgeRequest) (bridgeAnswer, error){
		"terminals": func(bridgeRequest) (bridgeAnswer, error) { return bridgeAnswer{OK: true, Terminals: installed}, nil },
		"open": func(r bridgeRequest) (bridgeAnswer, error) {
			return bridgeAnswer{OK: true, Tab: "tty:/dev/ttys004", Window: "tty:/dev/ttys004#w1234"}, nil
		},
	}}
	useMacOpeners(app)
	return app
}

// The terminals offered are the app's, in setup's order, and the list is asked once in a while, not at every look.
func TestTheMacsTerminalsAreTheAppsInstalledOnes(t *testing.T) {
	isolate(t)
	app := withMacOpeners(t, []string{"ghostty", "terminal", "xterm"})
	for range 3 {
		openers := installedOpeners()
		var ids []string
		for _, o := range openers {
			ids = append(ids, o.id+"="+o.name)
		}
		if strings.Join(ids, " ") != "terminal=Terminal ghostty=Ghostty" {
			t.Fatalf("openers %v", ids)
		}
	}
	if got := app.actions(); len(got) != 1 {
		t.Errorf("the app was asked %v", got)
	}
	// The app can't say: no terminal, so a hire goes to tmux alone.
	failing := &scriptedHost{answers: map[string]func(bridgeRequest) (bridgeAnswer, error){
		"terminals": func(bridgeRequest) (bridgeAnswer, error) { return bridgeAnswer{}, errors.New("gone") },
	}}
	useMacOpeners(failing)
	if openers := installedOpeners(); len(openers) != 0 {
		t.Errorf("openers without the app: %v", openers)
	}
}

// A hire's window: a script only you can read or run, in the hires folder,
// that removes itself and runs the command quoted word by word; the app opens
// the chosen terminal on it, and its window's record comes back. AppleScript
// is the app's alone: nothing here names it.
func TestAMacWindowOpensOnAScriptThroughTheApp(t *testing.T) {
	isolate(t)
	app := withMacOpeners(t, []string{"terminal", "iterm"})
	folder := t.TempDir()
	var iterm hireOpener
	for _, o := range installedOpeners() {
		if o.id == "iterm" {
			iterm = o
		}
	}
	if iterm.open == nil {
		t.Fatal("no iTerm2 opener")
	}
	var script string
	app.answers["open"] = func(r bridgeRequest) (bridgeAnswer, error) {
		script = r.Script
		data, err := os.ReadFile(r.Script)
		info, _ := os.Stat(r.Script)
		if err != nil || info.Mode().Perm() != 0o700 || filepath.Dir(r.Script) != filepath.Join(supportDir, "hires") ||
			r.Terminal != "iterm" || r.Folder != folder {
			t.Errorf("opened %+v (%v)", r, err)
		}
		want := "#!/bin/sh\nrm -f -- \"$0\"\nexec '/usr/bin/tmux' '-S' '/s/its socket' 'attach' '-t' '=hire'\n"
		if testCopyPort() != 0 {
			want = ""
		}
		if want != "" && string(data) != want {
			t.Errorf("the script:\n%s", data)
		}
		return bridgeAnswer{OK: true, Tab: "iterm:AB12", Window: "iterm:AB12"}, nil
	}
	window, err := showWindow(iterm, folder, []string{"/usr/bin/tmux", "-S", "/s/its socket", "attach", "-t", "=hire"})
	if err != nil || window != "iterm:AB12" {
		t.Fatalf("%q %v", window, err)
	}
	if !slices.Contains(app.actions(), "open") || script == "" {
		t.Errorf("the app was asked %v", app.actions())
	}
	// The window didn't open: its script goes, as nothing ran it.
	app.answers["open"] = func(r bridgeRequest) (bridgeAnswer, error) {
		script = r.Script
		return bridgeAnswer{OK: false, Error: "The window didn't open."}, nil
	}
	if _, err := showWindow(iterm, folder, []string{"true"}); err == nil {
		t.Fatal("a window that didn't open was taken as open")
	}
	if _, err := os.Stat(script); err == nil {
		t.Error("the script of a window that didn't open was left")
	}
}

// A test copy's window points the smolcorp command at the copy: the terminal's
// environment is its own, not the engine's.
func TestATestCopysWindowTalksToTheCopy(t *testing.T) {
	lines, err := windowScriptLines([]string{"smolcorp", "run", "--hire", "AB"}, 7779, "/Users/me/test support")
	if err != nil {
		t.Fatal(err)
	}
	want := "#!/bin/sh\nrm -f -- \"$0\"\nexport SMOLCORP_PORT='7779'\nexport SMOLCORP_TEST_SUPPORT_DIR='/Users/me/test support'\nexec 'smolcorp' 'run' '--hire' 'AB'\n"
	if lines != want {
		t.Errorf("got\n%s\nwant\n%s", lines, want)
	}
	if _, err := windowScriptLines(nil, 0, ""); err == nil {
		t.Error("an empty command was written")
	}
}

// An engine hire's window, opened through the app, is recorded for the hire,
// as the Swift engine's Deck.hireWindows; and so is "Open a window" from Setup.
func TestAnEngineHiresMacWindowIsRecorded(t *testing.T) {
	d, _, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	app := withMacOpeners(t, []string{"terminal"})
	d.mu.Lock()
	d.config.HireTerminal = "terminal"
	d.mu.Unlock()
	if status, message := d.Perform("hire", "", map[string][]string{"role": {"janitor"}}); status != 204 {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 1 {
		t.Fatalf("registry %+v", got)
	}
	if windows := windowsOfHire(got[0].ID); len(windows) != 1 || windows[0] != "tty:/dev/ttys004#w1234" {
		t.Errorf("windows %v (app asked %v)", windows, app.actions())
	}
	forgetHireWindows(got[0].ID, []string{"tty:/dev/ttys004#w1234"})
	if windows := windowsOfHire(got[0].ID); len(windows) != 0 {
		t.Errorf("forgotten, yet %v", windows)
	}
}

// macApp puts the Mac's openers in place on a stand-in app that keeps what each
// window's script would run (it never tells a terminal anything), for one test.
func macApp(t *testing.T, d *Deck) (ran *[]string) {
	t.Helper()
	ran = &[]string{}
	saved := installedOpeners
	t.Cleanup(func() { installedOpeners = saved })
	useMacOpeners(&scriptedHost{answers: map[string]func(bridgeRequest) (bridgeAnswer, error){
		"terminals": func(bridgeRequest) (bridgeAnswer, error) {
			return bridgeAnswer{OK: true, Terminals: []string{"terminal"}}, nil
		},
		"open": func(r bridgeRequest) (bridgeAnswer, error) {
			script, _ := os.ReadFile(r.Script)
			*ran = append(*ran, string(script))
			return bridgeAnswer{OK: true, Tab: "tty:/dev/ttys004", Window: "tty:/dev/ttys004#w1234"}, nil
		},
	}})
	d.mu.Lock()
	// The Mac's defaults: hires not in tmux, their windows in front.
	d.config.HireTerminal, d.config.HireInTmux, d.config.HiresInBackground = "terminal", false, false
	d.mu.Unlock()
	return ran
}

// B1 (RM4): under the Mac app, with the Mac's defaults (not in tmux, windows in
// front), a role's Claude hire is the engine's: setup's janitor, setup's GM and
// the office's janitor each get a registry row whose settings hold the guard,
// and no window runs the launcher (`run --hire`), which writes no fence.
func TestUnderTheMacAppARolesClaudeHireIsAlwaysFenced(t *testing.T) {
	d, _, h := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	managerOn(d)
	ran := macApp(t, d)
	repo := homeRepo(t)
	page := func(body string) {
		t.Helper()
		d.mu.Lock()
		d.lastHire = time.Time{}
		d.mu.Unlock()
		w := serve(t, d, call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: body,
			headers: map[string]string{setupHeader: "1"}})
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body)
		}
		d.watching.Wait()
	}
	page(`{"role":"janitor","folder":"` + repo + `","answers":[],"worktree":true,"task":"Sweep."}`)
	page(`{"role":"gm","folder":"` + repo + `","answers":[]}`)
	d.mu.Lock()
	d.lastHire = time.Time{}
	d.mu.Unlock()
	if status, message := d.Perform("hire", "", map[string][]string{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("the office's hire: %d %s", status, message)
	}
	d.watching.Wait()
	got := registered(t)
	if len(got) != 3 {
		t.Fatalf("registry %+v", got)
	}
	for _, hire := range got {
		settings := startedSettings(t, hire.ID)
		guard := `"args":["guard","--groups",`
		if hire.Role == gmRole {
			guard = `"args":["guard","--profile","gm","--groups",` // the GM's own fence
		}
		if !strings.Contains(settings, guard) {
			t.Errorf("%s (%s) has no guard: %s", hire.ID, hire.Role, settings)
		}
	}
	if len(h.windows)+len(h.tmux) != 0 || len(*ran) != 3 {
		t.Errorf("windows: plain %q %q, through the app %q", h.windows, h.tmux, *ran)
	}
	for _, script := range *ran {
		// The launcher's window wraps its command in the login shell's quotes: --hire is all that's sure to be there.
		if strings.Contains(script, "--hire") || !strings.Contains(script, "'attach'") {
			t.Errorf("a window ran the launcher, or didn't attach to the hire:\n%s", script)
		}
	}
}

// B1 (RM4): under the Mac app, a hire the Mac would manage (the manager on, a
// worktree asked for, or the GM) is refused rather than started unfenced: with
// no tmux, or at a desk pinned to a folder. A hire it wouldn't manage still
// starts the old way, as the Swift engine's does.
func TestUnderTheMacAppAManagedHireThatCantBeFencedIsRefused(t *testing.T) {
	d, r, h := engineDeck(t)
	ran := macApp(t, d)
	repo := homeRepo(t)
	hire := func(desk, role string, extra *setupHire) (int, string) {
		d.mu.Lock()
		d.lastHire = time.Time{}
		d.mu.Unlock()
		return d.hireAsWith(desk, role, 0, extra != nil, extra)
	}
	// What a check opened is cleared after it, so one failure doesn't make every later check fail too (RM4d's N5).
	nothing := func(what string) {
		t.Helper()
		if len(*ran)+len(h.windows)+len(h.tmux)+len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
			t.Errorf("%s: something was opened or made: %q %q %q", what, *ran, h.windows, r.tmux.asked)
		}
		*ran, h.windows, h.tmux, r.tmux.asked = nil, nil, nil, nil
	}
	// The deck's config is changed under its lock, as the deck's own rounds and saves read it (RSY3's nit 4).
	set := func(change func(c *Config)) {
		d.mu.Lock()
		defer d.mu.Unlock()
		change(d.config)
	}
	// The manager off, at a desk pinned to a folder: not managed, so the old way, as before.
	pin := r.repo
	set(func(c *Config) { c.Pins[1] = &pin })
	if status, message := hire("1", "janitor", nil); status != http.StatusNoContent || len(*ran) != 1 {
		t.Fatalf("an unmanaged hire at a pinned desk: %d %s, windows %q", status, message, *ran)
	}
	// It ran the launcher, and the other test's check for that (--hire) sees it.
	if !strings.Contains((*ran)[0], "--hire") {
		t.Errorf("the launcher's window doesn't show --hire:\n%s", (*ran)[0])
	}
	*ran = nil
	for _, c := range []struct {
		name, desk, role string
		extra            *setupHire
		manager          bool
		want             string
	}{
		{"the manager on", "1", "janitor", nil, true, macHirePinnedUnfenced},
		// Setup's sheet chose the pinned desk itself: another desk isn't its way out.
		{"a worktree asked for", "1", "janitor", &setupHire{Role: "janitor", Folder: repo, Worktree: true}, false, macSetupHirePinnedUnfenced},
		{"the GM", "1", gmRole, nil, false, macHirePinnedUnfenced},
	} {
		set(func(c2 *Config) { c2.ManagerEnabled = c.manager })
		if status, message := hire(c.desk, c.role, c.extra); status != http.StatusConflict || message != c.want {
			t.Errorf("%s, pinned: %d %s", c.name, status, message)
		}
		nothing(c.name + ", pinned")
	}
	set(func(c *Config) { c.Pins[1], c.ManagerEnabled, c.HireAgent = nil, true, "claude" })
	// Anyone (no role), Claude, the manager on: the Mac fences it, so here it's refused (B1-a, RM4b).
	if status, message := hire("", "", nil); status != http.StatusConflict || message != macHireNoRoleUnfenced {
		t.Errorf("Anyone, the manager on: %d %s", status, message)
	}
	nothing("Anyone, the manager on")
	terminals, tmux := hireWays()
	note := func() string {
		d.mu.Lock()
		agent := d.config.HireAgent
		d.mu.Unlock()
		return d.unfencedNote("", agent, terminals, tmux)
	}
	if got := note(); got != macHireNoRoleUnfenced {
		t.Errorf("the office's line for Anyone: %q", got)
	}
	// A hire agent the Mac reads as Claude Code but not spelled claude, from a config.json edited
	// by hand: the whole config is loaded again, and the hire agent loads as Claude, as on the Mac,
	// so a hire with no role is refused the same at a free desk, upstairs and at a pinned desk
	// (B1-b, RM4c; RM4d's N3).
	d.mu.Lock()
	d.config.save()
	d.mu.Unlock()
	var whole map[string]any
	if data, err := os.ReadFile(configFile); err != nil || json.Unmarshal(data, &whole) != nil {
		t.Fatalf("the deck's config.json: %v", err)
	}
	ways := []struct {
		name string
		hire func() (int, string)
	}{
		{"a free desk", func() (int, string) { return hire("", "", nil) }},
		{"upstairs", func() (int, string) {
			d.mu.Lock()
			d.lastHire = time.Time{}
			d.mu.Unlock()
			return d.Perform("hire", "", url.Values{"floor": {"1"}, "desk": {"0"}})
		}},
		{"a pinned desk", func() (int, string) {
			set(func(c *Config) { c.Pins[1] = &pin })
			defer set(func(c *Config) { c.Pins[1] = nil })
			return hire("1", "", nil)
		}},
	}
	for _, saved := range []string{"Claude", "/opt/x/bin/claude", ""} {
		whole["hireAgent"] = saved
		data, _ := json.Marshal(whole)
		writeFile(t, configFile, string(data))
		set(func(c *Config) { *c = *loadConfig() })
		for _, way := range ways {
			if status, message := way.hire(); status != http.StatusConflict || message != macHireNoRoleUnfenced {
				t.Errorf("Anyone on %q at %s, the manager on: %d %s", saved, way.name, status, message)
			}
			nothing(fmt.Sprintf("Anyone on %q at %s, the manager on", saved, way.name))
		}
		if got := note(); got != macHireNoRoleUnfenced {
			t.Errorf("the office's line for Anyone on %q: %q", saved, got)
		}
	}
	// A role on an agent Smolcorp doesn't know, at an unpinned desk: its own words, not the pinned desk's (N-c).
	var i int
	set(func(c *Config) {
		c.Roles = slices.Clone(c.Roles)
		i = slices.IndexFunc(c.Roles, func(r Role) bool { return r.ID == "janitor" })
		c.Roles[i].Agent = "robot"
	})
	if status, message := hire("", "janitor", nil); status != http.StatusConflict || message != macHireUnknownAgent {
		t.Errorf("a role on an unknown agent: %d %s", status, message)
	}
	nothing("a role on an unknown agent")
	set(func(c *Config) { c.Roles[i].Agent, c.ManagerEnabled = "claude", true })
	tmuxPath = func() string { return "" }
	for _, c := range []struct {
		name  string
		extra *setupHire
	}{{"the office's", nil}, {"setup's", &setupHire{Role: "janitor", Folder: repo}}} {
		if status, message := hire("", "janitor", c.extra); status != http.StatusConflict || message != macHireNeedsTmux {
			t.Errorf("%s hire, no tmux: %d %s", c.name, status, message)
		}
		nothing(c.name + ", no tmux")
	}
	d.mu.Lock()
	cooling := !d.lastHire.IsZero()
	d.mu.Unlock()
	if cooling {
		t.Error("a refused hire spent the cooldown")
	}
}

// PGS: the GM route's hire (fenced) under the Mac app is refused in the engine's
// own words (M4's) where it can't start fenced, with nothing opened or made and
// the cooldown not spent, whether the manager is on or off.
func TestUnderTheMacAppTheGMRoutesHireIsRefusedInTheEnginesWords(t *testing.T) {
	d, r, h := engineDeck(t)
	ran := macApp(t, d)
	pin := r.repo
	for _, manager := range []bool{false, true} {
		for _, c := range []struct {
			name  string
			floor int
			setup func()
			want  string
		}{
			// The route names a floor, never a desk: its words send the GM to another floor (N5, RPGS).
			{"a floor with its own folder", 1, func() { d.config.FloorRules = checkedFloorRules([]FloorRule{{Folder: pin, Floor: 1}}) }, macGMHirePinnedUnfenced},
			{"no tmux", 0, func() { tmuxPath = func() string { return "" } }, macHireNeedsTmux},
		} {
			saved := tmuxPath
			d.config.ManagerEnabled, d.config.FloorRules = manager, nil
			c.setup()
			d.lastHire = time.Time{}
			seat := -1
			status, message := d.hireWith(hireSpec{floor: c.floor, roleID: "janitor", fromSetup: true, fenced: true, seat: &seat})
			tmuxPath = saved
			if status != http.StatusConflict || message != c.want {
				t.Errorf("manager %v, %s: %d %s, want 409 %q", manager, c.name, status, message, c.want)
			}
			if len(*ran)+len(h.windows)+len(h.tmux)+len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
				t.Errorf("manager %v, %s: something was opened or made: %q %q %q", manager, c.name, *ran, h.windows, r.tmux.asked)
			}
			if !d.lastHire.IsZero() {
				t.Errorf("manager %v, %s: a refused hire spent the cooldown", manager, c.name)
			}
		}
	}
}

// N5 (RPGS): under the Mac app, the route's 400 for a field it can't honour speaks of this engine, not of
// Linux and Windows; on its own, the words name those.
func TestUnderTheMacAppTheGMRoutesFieldRefusalSpeaksOfThisEngine(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	want := "The General Manager's hire can't honour \"model\" yet: leave it out."
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","model":"opus"}`); code != http.StatusBadRequest || !strings.HasPrefix(text, "Smolcorp on Linux and Windows") {
		t.Errorf("on its own: %d %q", code, text)
	}
	ran := macApp(t, d)
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","model":"opus"}`); code != http.StatusBadRequest || text != want {
		t.Errorf("under the Mac app: %d %q, want 400 %q", code, text, want)
	}
	if len(*ran) != 0 {
		t.Errorf("a refused hire opened %q", *ran)
	}
	nothingMade(t, r, h)
}

// B2 (RM4): the launcher starts on the default account, so a hire on a named
// account, or any Claude hire in a test copy, is refused on each plain way
// (tmux off, a pinned folder, no tmux) with nothing opened.
func TestAPlainHireNeverStartsOnTheDefaultAccountInsteadOfANamedOne(t *testing.T) {
	d, r, h := engineDeck(t)
	repo := homeRepo(t)
	account := filepath.Join(homeDir, ".claude-test")
	if err := os.MkdirAll(account, 0o755); err != nil {
		t.Fatal(err)
	}
	saved := Port
	t.Cleanup(func() { Port = saved })
	pin := r.repo
	for _, copy := range []bool{true, false} {
		Port = portFromEnvironment("")
		if copy {
			Port = 7799
		}
		for _, c := range []struct {
			name  string
			desk  string
			setup func()
		}{
			{"tmux off", "", func() { d.config.HireInTmux = false }},
			{"a pinned folder", "1", func() { d.config.HireInTmux, d.config.Pins[1] = true, &pin }},
			{"no tmux", "", func() { tmuxPath = func() string { return "" } }},
		} {
			saved := tmuxPath
			d.config.HireInTmux, d.config.Pins[1] = true, nil
			c.setup()
			d.lastHire = time.Time{}
			status, message := d.hireAsWith(c.desk, "janitor", 0, true,
				&setupHire{Role: "janitor", Folder: repo, Account: "test", configDir: account})
			tmuxPath = saved
			// A test copy's hires run in tmux with no window (RTCH N1), so with tmux off in setup it takes the engine's
			// way, not the plain one: the named account has no .claude.json here, and the engine says so.
			want := plainHireDefaultAccount
			if copy && c.name == "tmux off" {
				want = "Start Claude Code once on this account, so it has a .claude.json, then hire again: Smolcorp only edits that file, it never creates it."
			}
			if status != http.StatusConflict || message != want {
				t.Errorf("test copy %v, %s: %d %s", copy, c.name, status, message)
			}
			if len(h.windows)+len(h.tmux)+len(r.tmux.asked) != 0 || len(registered(t)) != 0 {
				t.Errorf("test copy %v, %s: opened %q %q %q", copy, c.name, h.windows, h.tmux, r.tmux.asked)
			}
		}
	}
	// A test copy's Claude hire through the office (no account named) is refused before, by T721's own words.
	Port, d.config.HireInTmux = 7799, false
	d.lastHire = time.Time{}
	if status, message := d.hireAsWith("", "janitor", 0, false, nil); status != http.StatusConflict || message == plainHireDefaultAccount {
		t.Errorf("a test copy's office hire: %d %s", status, message)
	}
}
