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
}

func fakeHiring(t *testing.T, tmux string) *hiring {
	t.Helper()
	h := &hiring{}
	saved := []any{installedOpeners, tmuxPath, canOpenWindows, startHire, runTmux}
	t.Cleanup(func() {
		installedOpeners = saved[0].(func() []hireOpener)
		tmuxPath = saved[1].(func() string)
		canOpenWindows = saved[2].(func() bool)
		startHire = saved[3].(func(*exec.Cmd) error)
		runTmux = saved[4].(func(...string) (string, error))
	})
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
	if !slices.Equal(command[4:], []string{"--role", "tester"}) {
		t.Errorf("a tester's hire runs %q, want the launcher on the role", command)
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
	if d.hiredDesk(token) != -1 {
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
		if err != nil || got.hire != c.want.hire || got.pinned != c.want.pinned || got.role != c.want.role || !slices.Equal(got.command, c.want.command) {
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
