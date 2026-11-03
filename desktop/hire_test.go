package desktop

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// hiring stands in for this computer's terminals: a terminal "term" (and
// "other") installed, a desktop, tmux where tmux is given, and nothing
// actually started. Each window started, and each tmux call, is kept.
type hiring struct {
	windows [][]string
	dirs    []string
	tmux    [][]string
	fail    error
	// The agents looked for where hires run, and why one isn't there, if it isn't.
	lookedFor []string
	missing   string
}

func fakeHiring(t *testing.T, tmux string) *hiring {
	t.Helper()
	h := &hiring{}
	saved := []any{installedOpeners, tmuxPath, canOpenWindows, startHire, runTmux, agentMissing, hooksNotConnected}
	t.Cleanup(func() {
		installedOpeners = saved[0].(func() []hireOpener)
		tmuxPath = saved[1].(func() string)
		canOpenWindows = saved[2].(func() bool)
		startHire = saved[3].(func(*exec.Cmd) error)
		runTmux = saved[4].(func(...string) (string, error))
		agentMissing = saved[5].(func(string) string)
		hooksNotConnected = saved[6].(func(Agent) string)
	})
	hooksNotConnected = func(Agent) string { return "" } // connected, unless a test says not
	opener := func(id string) hireOpener {
		return hireOpener{id: id, name: "Term " + id, args: func(folder string, command []string) []string {
			return append([]string{id, "--cwd", folder}, command...)
		}}
	}
	installedOpeners = func() []hireOpener { return []hireOpener{opener("term"), opener("other")} }
	tmuxPath = func() string { return tmux }
	canOpenWindows = func() bool { return true }
	startHire = func(cmd *exec.Cmd) error {
		h.windows = append(h.windows, cmd.Args)
		h.dirs = append(h.dirs, cmd.Dir)
		return h.fail
	}
	runTmux = func(args ...string) (string, error) {
		h.tmux = append(h.tmux, args)
		return "%7 /tmp/tmux-1000/default", h.fail
	}
	agentMissing = func(program string) string {
		h.lookedFor = append(h.lookedFor, program)
		return h.missing
	}
	return h
}

// hiringDeck is a deck with hiring on and a folder for new hires.
func hiringDeck(t *testing.T, tmux string) (*Deck, *hiring, string) {
	t.Helper()
	h := fakeHiring(t, tmux)
	d := newTestDeck(t)
	folder := t.TempDir()
	d.config.HiringEnabled = true
	d.config.HireFolder = folder
	return d, h, folder
}

// hiredWith is the command a hire's window runs, past the terminal and the
// login shell, and its token.
func hiredWith(t *testing.T, window []string) (command []string, token string) {
	t.Helper()
	command = window[3:]
	if len(command) == 3 && strings.HasSuffix(command[1], "c") && strings.HasPrefix(command[2], "exec ") {
		// Through the login shell: its words, quoted.
		command = nil
		for _, word := range strings.Split(strings.TrimPrefix(window[5], "exec "), "' '") {
			command = append(command, strings.Trim(word, "'"))
		}
	}
	if len(command) < 4 || command[1] != "run" || command[2] != "--hire" || !hireToken.MatchString(command[3]) {
		t.Fatalf("the window runs %q, want the launcher with a hire's token", command)
	}
	return command, command[3]
}

func TestAHireOpensATerminalRunningTheLauncher(t *testing.T) {
	d, h, folder := hiringDeck(t, "")
	d.config.HireAgent = "codex"
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	if len(h.windows) != 1 || h.windows[0][0] != "term" || h.windows[0][2] != folder || h.dirs[0] != folder {
		t.Fatalf("windows opened: %q in %q, want one terminal in the folder for new hires", h.windows, h.dirs)
	}
	command, token := hiredWith(t, h.windows[0])
	if command[0] != commandLineProgram() || command[len(command)-1] != "codex" || slices.Contains(command, "--pinned") {
		t.Errorf("the hire runs %q, want this program running codex", command)
	}
	if _, ok := d.hires[token]; !ok {
		t.Errorf("the hire's token isn't kept for its launcher")
	}
	// A second tap, or a second phone, a moment later opens nothing more.
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusTooManyRequests || len(h.windows) != 1 {
		t.Errorf("a second hire at once: %d, %d windows", status, len(h.windows))
	}
	d.lastHire = time.Time{}
	d.config.HireTerminal = "other"
	d.Perform("hire", "", url.Values{})
	if len(h.windows) != 2 || h.windows[1][0] != "other" {
		t.Errorf("with another terminal chosen, the hire opens %q", h.windows[len(h.windows)-1])
	}
}

func TestAHireThatDidntOpenCanBeTriedAgain(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	h.fail = errors.New("no such terminal")
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusInternalServerError || message != "Term term didn't open" {
		t.Errorf("a terminal that didn't open: %d %q", status, message)
	}
	h.fail = nil
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent || len(d.hires) != 1 {
		t.Errorf("trying again at once: %d, %d hires waiting", status, len(d.hires))
	}
}

func TestAHireNeedsAFolderARoleAndATerminal(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	d.config.HireFolder = filepath.Join(t.TempDir(), "gone")
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusConflict || !strings.Contains(message, "Choose a folder") {
		t.Errorf("no folder: %d %q", status, message)
	}
	d.config.HireFolder = t.TempDir()
	if status, _ := d.Perform("hire", "", url.Values{"role": {"nobody"}}); status != http.StatusNotFound {
		t.Errorf("a role that's gone: %d", status)
	}
	installedOpeners = func() []hireOpener { return nil }
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusConflict || len(h.windows) != 0 {
		t.Errorf("nowhere to open a hire: %d", status)
	}
	d.config.HiringEnabled = false
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusForbidden {
		t.Errorf("with hiring off: %d", status)
	}
}

func TestAHireByRoleStartsTheRole(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	d.config.Roles = slices.Clone(shippedRoles)
	if status, message := d.Perform("hire", "", url.Values{"role": {"tester"}}); status != http.StatusNoContent {
		t.Fatalf("hire a tester: %d %s", status, message)
	}
	command, _ := hiredWith(t, h.windows[0])
	tester := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == "tester" })
	if !slices.Equal(command[4:], []string{"--agent", d.config.Roles[tester].Agent, "--role", "tester"}) {
		t.Errorf("a tester's hire runs %q, want the launcher on the role, with its agent as checked", command)
	}
}

func TestAHireAtAPinnedDeskStartsInItsProject(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	project := t.TempDir()
	d.config.Pin(project, 2)
	if status, message := d.Perform("hire", "", url.Values{"desk": {"2"}}); status != http.StatusNoContent {
		t.Fatalf("hire at the pinned desk: %d %s", status, message)
	}
	command, _ := hiredWith(t, h.windows[0])
	if h.dirs[0] != project || command[4] != "--pinned" {
		t.Errorf("a hire at a pinned desk runs %q in %s, want --pinned in %s", command, h.dirs[0], project)
	}
}

func TestAHireSitsAtTheDeskItWasHiredAt(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	d.config.Layout.Keys = 6
	running(d, 1)
	if status, _ := d.Perform("hire", "", url.Values{"desk": {"0"}}); status != http.StatusConflict {
		t.Errorf("hire at a desk someone's at: %d", status)
	}
	if status, message := d.Perform("hire", "", url.Values{"desk": {"4"}}); status != http.StatusNoContent {
		t.Fatalf("hire at desk 4: %d %s", status, message)
	}
	_, token := hiredWith(t, h.windows[0])
	// Someone else comes in meanwhile, and takes the first free desk, not the hire's.
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "other", "cwd": "/work/o"},
		Claude, "tmux:%9@/tmp/tmux", "", 150)
	d.launcherConnected("H1", Claude, d.config.HireFolder, "", token)
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "hired", "cwd": d.config.HireFolder},
		Claude, "launcher:H1", "", 200)
	keys, _ := d.layout()
	if keys[4].session == nil || keys[4].session.ID != "hired" || keys[1].session == nil || keys[1].session.ID != "other" {
		t.Errorf("the hire isn't at desk 4: %v", keys)
	}
	// A token is good once.
	if desk, _, _ := d.hiredDesk(token); desk != -1 {
		t.Errorf("a hire's token was taken twice")
	}
}

func TestAHireCanRunInTmux(t *testing.T) {
	d, h, folder := hiringDeck(t, "/usr/bin/tmux")
	d.config.HireInTmux = true
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("hire in tmux: %d %s", status, message)
	}
	if len(h.tmux) != 1 || h.tmux[0][0] != "new-session" || !slices.Contains(h.tmux[0], folder) {
		t.Fatalf("tmux was asked %q, want a session of its own in the folder", h.tmux)
	}
	session := h.tmux[0][slices.Index(h.tmux[0], "-s")+1]
	if want := []string{"/usr/bin/tmux", "-S", "/tmp/tmux-1000/default", "attach", "-t", session}; len(h.windows) != 1 || !slices.Equal(h.windows[0][3:], want) {
		t.Errorf("the window runs %q, want it to show the session", h.windows)
	}
	// With no desktop, as over SSH, the session alone: tmux attach shows it.
	canOpenWindows = func() bool { return false }
	d.lastHire = time.Time{}
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent || len(h.tmux) != 2 || len(h.windows) != 1 {
		t.Errorf("a hire with no desktop: %d, tmux %d, windows %d", status, len(h.tmux), len(h.windows))
	}
}

func TestTheOfficeOffersHiringOnceItCan(t *testing.T) {
	d, _, folder := hiringDeck(t, "")
	d.config.Roles = slices.Clone(shippedRoles[:2])
	var state struct {
		Hiring struct {
			Fire, Hire bool
			Roles      []map[string]any
		}
	}
	read := func() {
		state.Hiring.Roles = nil
		if err := json.Unmarshal(d.snapshot(), &state); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if !state.Hiring.Fire || !state.Hiring.Hire || len(state.Hiring.Roles) != 2 || state.Hiring.Roles[1]["id"] != "tester" ||
		state.Hiring.Roles[0]["instructions"] != nil {
		t.Errorf("the office is offered %+v", state.Hiring)
	}
	os.Remove(folder)
	if read(); state.Hiring.Hire || len(state.Hiring.Roles) != 0 {
		t.Errorf("with the folder gone, the office is offered %+v", state.Hiring)
	}
}

func TestSetupChoosesWhereHiresOpen(t *testing.T) {
	d, _, _ := hiringDeck(t, "")
	set := func(query string) int {
		return serve(t, d, call{method: "POST", target: "/setup/hiring?" + query, host: "localhost:7710", from: thisComputer,
			headers: map[string]string{setupHeader: "1"}}).Code
	}
	if code := set("terminal=other"); code != 200 || d.config.HireTerminal != "other" {
		t.Errorf("choosing another terminal: %d, %q", code, d.config.HireTerminal)
	}
	if code := set("terminal=nope"); code != 400 || d.config.HireTerminal != "other" {
		t.Errorf("a terminal that isn't here: %d", code)
	}
	if code := set("tmux=1"); code != 400 || d.config.HireInTmux {
		t.Errorf("tmux where there's none: %d", code)
	}
	tmuxPath = func() string { return "/usr/bin/tmux" }
	if code := set("tmux=1"); code != 200 || !d.config.HireInTmux {
		t.Errorf("tmux: %d", code)
	}
	response := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
	var state struct{ Hiring map[string]any }
	json.Unmarshal(response.Body.Bytes(), &state)
	h := state.Hiring
	if h["canHire"] != true || h["terminal"] != "other" || h["tmux"] != true || h["hasTmux"] != true || len(h["terminals"].([]any)) != 2 {
		t.Errorf("setup shows hiring as %v", h)
	}
}

func TestAHiresWindowNamesNothingFromWhereSmolcorpStarted(t *testing.T) {
	t.Setenv("SMOLCORP_SESSION", "ABC")
	t.Setenv("TMUX_PANE", "%1")
	t.Setenv("SMOLCORP_HEADLESS", "1")
	for _, variable := range hireEnvironment() {
		for _, name := range []string{"SMOLCORP_SESSION=", "TMUX_PANE=", "SMOLCORP_HEADLESS="} {
			if strings.HasPrefix(variable, name) {
				t.Errorf("a hire's window is given %s", variable)
			}
		}
	}
}

func TestTheLauncherKnowsAHire(t *testing.T) {
	token := strings.Repeat("AB", 16)
	for _, c := range []struct {
		args []string
		want *launcherRun
	}{
		{[]string{"claude", "--resume"}, &launcherRun{command: []string{"claude", "--resume"}}},
		{[]string{"--role", "tester"}, &launcherRun{role: "tester"}},
		{[]string{"--hire", token, "codex"}, &launcherRun{hire: token, command: []string{"codex"}}},
		{[]string{"--hire", token, "--pinned", "--role", "tester"}, &launcherRun{hire: token, pinned: true, role: "tester"}},
		{[]string{"--hire", token, "--agent", "codex", "--role", "tester"}, &launcherRun{hire: token, agent: "codex", role: "tester"}},
		{[]string{"--hire", token, "--agent", "codex", "claude"}, nil},
		{[]string{"--hire", token, "--agent", "--role", "tester"}, nil},
		{[]string{"--hire", "not-a-token", "claude"}, nil},
		{[]string{"--hire", token}, nil},
		{[]string{"--role"}, nil},
		{[]string{"--role", "a", "b"}, nil},
		{nil, nil},
	} {
		got, err := parseRun(c.args)
		if c.want == nil {
			if err == nil {
				t.Errorf("%q was taken as %+v", c.args, got)
			}
			continue
		}
		if err != nil || got.hire != c.want.hire || got.pinned != c.want.pinned || got.role != c.want.role || got.agent != c.want.agent || !slices.Equal(got.command, c.want.command) {
			t.Errorf("%q is %+v, %v; want %+v", c.args, got, err, c.want)
		}
	}
}

func TestAHiresRoleIsToldWhatTheMacAppTellsIt(t *testing.T) {
	isolate(t)
	homeDir = filepath.Join(string(filepath.Separator), "home", "someone")
	tester := shippedRoles[1]
	general := tester.firstMessage(filepath.Join(homeDir, "Projects"), hiredIntoGeneralFolder)
	want := "You've started in " + filepath.Join("~", "Projects") + ", a general folder, not a project. " +
		"Your first reply must only ask me the questions below, all together. Don't read code, run commands or start any " +
		"work until I've answered them.\n\n" +
		"1. Which project should you work on: a folder on this computer, or a GitHub repository?\n" +
		"2. Which tests should you run: all of them, a folder, or a single file?\n"
	if !strings.HasPrefix(general, want) {
		t.Errorf("a hire into the folder for new hires is told\n%s\nwant it to start\n%s", general, want)
	}
	pinned := tester.firstMessage(filepath.Join(homeDir, "work", "api"), hiredAtPinnedDesk)
	want = "Your first reply must only ask me the questions below, all together. Don't read code, run commands or start any " +
		"work until I've answered them.\n\n" +
		"1. You've started in " + filepath.Join("~", "work", "api") + ", this desk's project. Is that the one to work on?\n"
	if !strings.HasPrefix(pinned, want) {
		t.Errorf("a hire at a pinned desk is told\n%s\nwant it to start\n%s", pinned, want)
	}
}

func TestAHireUpstairsSitsAtTheDeskAndFloorItWasHiredAtInTheFloorsFolder(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	atlas := t.TempDir()
	d.config.FloorRules = floorRuleList{{atlas, 2, false}}
	if status, message := d.Perform("hire", "", url.Values{"desk": {"5"}, "floor": {"2"}}); status != http.StatusNoContent {
		t.Fatalf("hire upstairs: %d %s", status, message)
	}
	command, token := hiredWith(t, h.windows[0])
	if h.dirs[0] != atlas || slices.Contains(command, "--pinned") {
		t.Errorf("a hire on the Atlas floor runs %q in %s, want it in %s", command, h.dirs[0], atlas)
	}
	d.launcherConnected("H2", Claude, atlas, "", token)
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "hired", "cwd": "/elsewhere"},
		Claude, "launcher:H2", "", 200)
	desks := floorDesks(t, d, 2)
	if len(desks) != 1 || desks[0]["id"] != "hired" || desks[0]["desk"] != 5.0 {
		t.Errorf("3F is %v, want the hire at desk 5", desks)
	}
	// That desk taken, it can't be hired at again; and a floor there isn't, isn't.
	d.lastHire = time.Time{}
	if status, _ := d.Perform("hire", "", url.Values{"desk": {"5"}, "floor": {"2"}}); status != http.StatusConflict {
		t.Errorf("hire at a taken desk upstairs: %d", status)
	}
	if status, _ := d.Perform("hire", "", url.Values{"desk": {"5"}, "floor": {"8"}}); status != http.StatusBadRequest {
		t.Errorf("hire on a floor there isn't: %d", status)
	}
}

func TestAFloorWithMoreThanOneRuleHiresIntoTheFolderForNewHires(t *testing.T) {
	if got := floorFolder([]FloorRule{{"/a", 2, false}, {"/b", 2, false}, {"/c", 3, false}}, 2); got != "" {
		t.Errorf("two rules gave %q", got)
	}
	if got := floorFolder([]FloorRule{{"/a", 2, false}, {"/c", 3, false}}, 3); got != "/c" {
		t.Errorf("one rule gave %q", got)
	}
}

func TestAHireOnTheGroundFloorStaysThereThoughItsFolderGoesUpstairs(t *testing.T) {
	d, h, folder := hiringDeck(t, "")
	d.config.FloorRules = floorRuleList{{folder, 3, false}}
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	_, token := hiredWith(t, h.windows[0])
	d.launcherConnected("H3", Claude, folder, "", token)
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "hired", "cwd": folder},
		Claude, "launcher:H3", "", 200)
	keys, _ := d.layout()
	if !slices.ContainsFunc(keys, func(k deckKey) bool { return k.session != nil && k.session.ID == "hired" }) {
		t.Errorf("the hire isn't on the ground floor: %v", keys)
	}
	// Anyone else in that folder goes up by its rule.
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "other", "cwd": folder},
		Claude, "tmux:%9@/tmp/tmux", "", 300)
	if desks := floorDesks(t, d, 3); len(desks) != 1 || desks[0]["id"] != "other" {
		t.Errorf("4F is %v, want the other session there by its rule", desks)
	}
}

func TestAHireWhoseAgentIsntFoundSaysWhyAndOpensNothing(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	d.config.HireAgent = "claude"
	h.missing = "Can't hire: your login shell (bash), where hires start, doesn't find claude."
	if status, message := d.hireFromSetup("gm"); status != http.StatusConflict || message != h.missing {
		t.Errorf("a hire whose agent isn't found: %d %q, want %d and why", status, message, http.StatusConflict)
	}
	if len(h.windows) != 0 || len(d.hires) != 0 {
		t.Errorf("a hire whose agent isn't found opened %q", h.windows)
	}
	// The GM's agent is looked for, and once it's there, the hire needn't wait.
	h.missing = ""
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent || len(h.windows) != 1 {
		t.Errorf("hiring again once the agent is found: %d, %d windows", status, len(h.windows))
	}
	gm := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == "gm" })
	if want := []string{d.config.Roles[gm].Agent, "claude"}; !slices.Equal(h.lookedFor, want) {
		t.Errorf("looked for %q, want the role's agent, then the one for new hires", h.lookedFor)
	}
}

func TestAnAgentNotFoundWhereHiresStartIsExplained(t *testing.T) {
	home := isolate(t)
	local := filepath.Join(home, ".local", "bin", "claude")
	for _, c := range []struct {
		shell, foundAt string
		want           []string
	}{
		{"/bin/bash", local, []string{"(bash)", "~/.profile", `export PATH="$HOME/.local/bin:$PATH"`, "~/.local/bin"}},
		{"/usr/bin/zsh", local, []string{"(zsh)", "~/.zprofile", `export PATH="$HOME/.local/bin:$PATH"`}},
		{"/usr/bin/fish", local, []string{"(fish)", "~/.config/fish/config.fish", "fish_add_path ~/.local/bin"}},
		{"/bin/bash", "/opt/agents/bin/claude", []string{`export PATH="/opt/agents/bin:$PATH"`}},
		{"/bin/bash", "", []string{"Claude Code isn't installed", "doesn't find claude", "~/.profile"}},
	} {
		message := agentNotFoundMessage("claude", c.shell, c.foundAt)
		for _, want := range c.want {
			if !strings.Contains(message, want) {
				t.Errorf("for %s with claude at %q: %q doesn't say %q", c.shell, c.foundAt, message, want)
			}
		}
	}
	// Bash reads ~/.bash_profile instead of ~/.profile when there's one.
	writeFile(t, filepath.Join(home, ".bash_profile"), "")
	if message := agentNotFoundMessage("claude", "/bin/bash", local); !strings.Contains(message, "~/.bash_profile") {
		t.Errorf("with a ~/.bash_profile: %q", message)
	}
}

func TestNoSecondHireWhileTheFirstsAgentIsLookedFor(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	d.config.HireAgent = "codex"
	var second int
	agentMissing = func(program string) string {
		// The check takes longer than the cooldown: a second hire, and a change in setup, come meanwhile.
		d.mu.Lock()
		d.lastHire = time.Now().Add(-time.Hour)
		d.config.HireAgent = "gemini"
		d.mu.Unlock()
		second, _ = d.Perform("hire", "", url.Values{})
		return ""
	}
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent {
		t.Fatalf("the first hire: %d %s", status, message)
	}
	if second != http.StatusTooManyRequests || len(h.windows) != 1 {
		t.Errorf("a hire while the first's agent was looked for: %d, %d windows; want %d and one", second, len(h.windows), http.StatusTooManyRequests)
	}
	// The hire runs the agent that was looked for.
	if command, _ := hiredWith(t, h.windows[0]); command[len(command)-1] != "codex" {
		t.Errorf("the hire runs %q, want codex, the agent looked for", command)
	}
	// Once it's done, the next can go.
	agentMissing = func(string) string { return "" }
	d.lastHire = time.Time{}
	if status, _ := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent {
		t.Errorf("the next hire, once the first is done: %d", status)
	}
}

func TestARoleHireRunsOnlyTheAgentItChecked(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	gm := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == "gm" })
	checked := d.config.Roles[gm].Agent
	if status, message := d.hireFromSetup("gm"); status != http.StatusNoContent {
		t.Fatalf("hiring the GM: %d %s", status, message)
	}
	command, _ := hiredWith(t, h.windows[0])
	if i := slices.Index(command, "--agent"); i < 0 || command[i+1] != checked || command[i+2] != "--role" {
		t.Fatalf("the hire runs %q, want the agent checked, %s, before its role", command, checked)
	}
	// The role's agent changed in setup after the check: the launcher won't run it.
	run := &launcherRun{role: "gm", hire: strings.Repeat("AB", 16), agent: checked}
	if err := sameAgentAsHired(run, checked); err != nil {
		t.Errorf("the agent checked: %v", err)
	}
	if err := sameAgentAsHired(run, "an-agent-never-checked"); err == nil || !strings.Contains(err.Error(), "Hire it again") {
		t.Errorf("an agent never checked: %v, want it refused", err)
	}
	// Started from a terminal, not hired, a role runs whatever agent it has.
	if err := sameAgentAsHired(&launcherRun{role: "gm"}, "anything"); err != nil {
		t.Errorf("a role started by hand: %v", err)
	}
}

// A hire from the setup page waits at Claude's trust and API-key prompts, which
// default to No, so the page's message after the hire says to answer Yes.
func TestTheSetupPageTellsAHireToAnswerYesAtClaudesPrompts(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	i := strings.Index(page, "const advice = '")
	if i < 0 {
		t.Fatal("no advice after a hire in the setup page")
	}
	said := page[i:]
	said = said[:strings.Index(said, "';")]
	for _, want := range []string{"trust the folder", "API key", "answer Yes to both"} {
		if !strings.Contains(said, want) {
			t.Errorf("the hire message %q lacks %q", said, want)
		}
	}
}

// The page may say a hire is at a desk only once a new General Manager session
// has a desk (the hire's hook posts it later), not as soon as the hire is
// accepted with a 204.
func TestTheSetupPageSaysHiredOnlyOnceTheHireIsAtADesk(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	start := strings.Index(page, "$('start-gm').onclick")
	if start < 0 {
		t.Fatal("no Hire your General Manager button handler in the setup page")
	}
	handler := page[start:]
	handler = handler[:strings.Index(handler, "\n  };\n")]
	accepted := strings.Index(handler, "if (!res.ok)")
	seated := strings.Index(handler, "startSaid(`Hired:")
	if accepted < 0 || seated < 0 {
		t.Fatalf("handler lacks the 204 check or the Hired message:\n%s", handler)
	}
	// Straight after the 204, it says the hire is starting, before any Hired.
	if starting := strings.Index(handler[accepted:], "is starting"); starting < 0 || accepted+starting > seated {
		t.Error("after the 204 the page doesn't say the hire is starting before it says Hired")
	}
	// Hired is said only from the wait for a new General Manager with a desk, polled from /setup/state.
	if !strings.Contains(handler[:seated], "await seatedGms()") || !strings.Contains(handler[:seated], "!before.includes(id)") ||
		!strings.Contains(handler[seated-60:seated], "if (now &&") && !strings.Contains(handler[seated-120:seated], "now.some(") {
		t.Error("Hired isn't said only after a new General Manager session shows up")
	}
	helper := page[strings.Index(page, "async function seatedGms()"):start]
	for _, want := range []string{"/setup/state", "s.role === 'gm'", "s.key != null"} {
		if !strings.Contains(helper, want) {
			t.Errorf("the seated check doesn't use %q", want)
		}
	}
	if !strings.Contains(handler[seated:], "still starting") {
		t.Error("no message for a hire that hasn't shown up in time, after the wait")
	}
}

// hooksOff points the real hooks check at a temporary HOME with Claude Code's
// folder but no hooks, so hires must be refused.
func hooksOff(t *testing.T) {
	t.Helper()
	saved := homeDir
	homeDir = t.TempDir()
	t.Cleanup(func() { homeDir = saved })
	if err := os.MkdirAll(agentFolder(Claude), 0o755); err != nil {
		t.Fatal(err)
	}
	hooksNotConnected = hooksRefusal
}

func TestAHireWithoutHooksConnectedIsRefusedAndOpensNothing(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	hooksOff(t)
	for name, hire := range map[string]func() (int, string){
		"office": func() (int, string) { return d.Perform("hire", "", url.Values{}) },
		"floor":  func() (int, string) { return d.Perform("hire", "", url.Values{"floor": {"1"}, "desk": {"0"}}) },
		"setup":  func() (int, string) { return d.hireFromSetup("gm") },
	} {
		status, message := hire()
		if status != http.StatusConflict || !strings.HasPrefix(message, "Connect your agents first") || !strings.Contains(message, "Press Connect in setup.") || strings.Contains(message, "smolcorp setup") {
			t.Errorf("%s hire without hooks: %d %q, want %d and Connect your agents first, with how", name, status, message, http.StatusConflict)
		}
	}
	if len(h.windows) != 0 || len(h.tmux) != 0 || len(d.hires) != 0 {
		t.Errorf("a hire without hooks went on: windows %q, tmux %q, hires %d", h.windows, h.tmux, len(d.hires))
	}
}

func TestAHireWithHooksConnectedGoesAhead(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	hooksOff(t)
	hooksNotConnected = func(Agent) string { return "" }
	if status, message := d.hireFromSetup("gm"); status != http.StatusNoContent {
		t.Fatalf("hire with hooks connected: %d %s", status, message)
	}
	if len(h.windows) != 1 {
		t.Errorf("windows opened: %q, want one", h.windows)
	}
}

// hooksOn is hooksOff with Claude Code hooked up, and Codex's folder there but
// empty: its agents' hooks are the real check's to read, not the fake's.
func hooksOn(t *testing.T) {
	t.Helper()
	hooksOff(t)
	if err := os.MkdirAll(agentFolder(Codex), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := connect(Claude, false); err != nil {
		t.Fatal(err)
	}
}

func TestAClaudeHireGoesAheadWhileAnotherAgentIsntHooked(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	hooksOn(t)
	if isConnected(Codex) {
		t.Fatal("Codex's empty folder counts as connected")
	}
	if status, message := d.hireFromSetup(""); status != http.StatusNoContent {
		t.Fatalf("Claude hire with Claude hooked and ~/.codex empty: %d %s", status, message)
	}
	if len(h.windows) != 1 {
		t.Errorf("windows opened: %q, want one", h.windows)
	}
}

func TestAHireOfAnUnhookedAgentIsRefusedThoughAnotherIsHooked(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	hooksOn(t)
	d.config.HireAgent = string(Codex)
	status, message := d.hireFromSetup("")
	if status != http.StatusConflict || !strings.HasPrefix(message, "Connect your agents first") || !strings.Contains(message, "Codex") {
		t.Errorf("Codex hire with Codex unhooked: %d %q, want %d and Connect your agents first naming Codex", status, message, http.StatusConflict)
	}
	if len(h.windows) != 0 {
		t.Errorf("the refused hire went on: windows %q", h.windows)
	}
}

func TestAnAgentNotInstalledIsSaidSoBeforeHooksNotConnected(t *testing.T) {
	d, h, _ := hiringDeck(t, "")
	hooksOff(t)
	h.missing = "Can't hire: your login shell (bash), where hires start, doesn't find claude."
	status, message := d.hireFromSetup("gm")
	if status != http.StatusConflict || message != h.missing || strings.Contains(message, "Connect your agents first") {
		t.Errorf("hire of an agent not installed, hooks off: %d %q, want %d and %q", status, message, http.StatusConflict, h.missing)
	}
	if !d.lastHire.IsZero() || len(h.windows) != 0 {
		t.Errorf("a refused hire cost the cooldown or opened a window: %v %q", d.lastHire, h.windows)
	}
}

func TestAFloorWithItsOwnFolderIsHiredOntoWithNoFolderForNewHires(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d, h, _ := hiringDeck(t, "")
	d.config.HireFolder = ""
	d.config.Roles = []Role{{ID: "dev", Name: "Dev", Agent: "claude"}}
	hiring := func() map[string]any {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.hiringState()
	}
	// With neither folder, nothing is offered, and a hire says to choose one.
	if state := hiring(); state["hire"] != false || len(state["roles"].([]map[string]any)) != 0 {
		t.Errorf("with no folder anywhere, hiring is %v", state)
	}
	if status, _ := d.Perform("hire", "", url.Values{"desk": {"5"}, "floor": {"2"}}); status != http.StatusConflict || len(h.windows) != 0 {
		t.Errorf("hire upstairs with no folder anywhere: %d, %d windows", status, len(h.windows))
	}
	// A floor whose folder is gone offers nothing either.
	d.config.FloorRules = floorRuleList{{filepath.Join(t.TempDir(), "gone"), 2, false}}
	if state := hiring(); state["hire"] != false {
		t.Errorf("with a floor folder that's gone, hiring is %v", state)
	}
	// A floor with a folder of its own: hiring is offered, by role, and goes ahead there.
	atlas := t.TempDir()
	d.config.FloorRules = floorRuleList{{atlas, 2, false}}
	if state := hiring(); state["hire"] != true || len(state["roles"].([]map[string]any)) != 1 {
		t.Errorf("with a floor folder, hiring is %v, want it offered with the role", state)
	}
	if status, message := d.Perform("hire", "", url.Values{"desk": {"5"}, "floor": {"2"}}); status != http.StatusNoContent {
		t.Fatalf("hire onto the floor with a folder: %d %s", status, message)
	}
	if len(h.windows) != 1 || h.dirs[0] != atlas {
		t.Errorf("the hire opened %q in %q, want one in %s", h.windows, h.dirs, atlas)
	}
	// The ground floor still needs the folder for new hires.
	d.lastHire = time.Time{}
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusConflict || !strings.Contains(message, "Choose a folder") {
		t.Errorf("hire on the ground floor: %d %q, want to be told to choose a folder", status, message)
	}
}

// An agent with no settings folder yet can't be connected, so its hire isn't
// refused for that: its terminal says what's missing, as on the Mac.
func TestAHireOfAnAgentWithNoSettingsFolderIsNotRefusedForHooks(t *testing.T) {
	hiringDeck(t, "")
	hooksOff(t)
	if why := hooksRefusal(Codex); why != "" {
		t.Errorf("Codex, never run here: %q, want no refusal", why)
	}
	if why := hooksRefusal(Claude); !strings.HasPrefix(why, "Connect your agents first") {
		t.Errorf("Claude, there but not connected: %q", why)
	}
}

func TestATestCopyRefusesAClaudeHireOnEveryRouteBeforeAnythingIsMade(t *testing.T) {
	d, h, _ := hiringDeck(t, "/usr/bin/tmux")
	onPort(t, 7799)
	d.config.Floors = checkedFloors(d.config.Floors)
	for _, query := range []url.Values{{}, {"role": {"janitor"}}, {"role": {"gm"}}, {"role": {"janitor"}, "floor": {"1"}, "desk": {"0"}}} {
		d.lastHire = time.Time{}
		status, message := d.Perform("hire", "", query)
		if status != http.StatusConflict || !strings.Contains(message, "never hires Claude on the default account") {
			t.Errorf("hire %v: %d %q", query, status, message)
		}
	}
	d.lastHire = time.Time{}
	if status, message := d.hireFromSetup("gm"); status != http.StatusConflict || !strings.Contains(message, "test copy") {
		t.Errorf("setup's GM hire: %d %q", status, message)
	}
	if len(h.windows)+len(h.tmux) != 0 || len(d.hires) != 0 || !d.lastHire.IsZero() {
		t.Error("something was opened, or the cooldown was spent, for a refused hire")
	}
	// Another agent on a test copy opens as it did.
	d.config.HireAgent = "codex"
	if status, message := d.Perform("hire", "", url.Values{}); status != http.StatusNoContent || len(h.windows) != 1 {
		t.Errorf("a test copy's codex hire: %d %q, %d windows", status, message, len(h.windows))
	}
}
