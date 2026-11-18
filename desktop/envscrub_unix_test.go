//go:build !windows

package desktop

import (
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// ENVSCRUB1's rows that start a stand-in agent: a program that writes down the environment it was started with,
// and nothing else (FAKEKEY values only). The markers are in this test's own environment, as in a Smolcorp opened
// inside a Claude Code session; none reaches the stand-in, and the owner's provider does.

// envStandIn is a stand-in agent at bin/name writing its environment to the file it gives, once it has run.
func envStandIn(t *testing.T, name string) (bin, written string) {
	t.Helper()
	bin, out := t.TempDir(), t.TempDir()
	written = filepath.Join(out, "env")
	writeFile(t, filepath.Join(bin, name), "#!/bin/sh\nenv > '"+written+".tmp'\nmv '"+written+".tmp' '"+written+"'\n"+
		"case \"$1\" in auth) echo '{\"loggedIn\":true,\"authMethod\":\"claude.ai\",\"subscriptionType\":\"max\"}';; esac\n")
	if err := os.Chmod(filepath.Join(bin, name), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, written
}

// standInSaw is the environment the stand-in wrote down.
func standInSaw(t *testing.T, written string) []string {
	t.Helper()
	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("the stand-in agent didn't run: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// Every kind of desk start (the GM, a hire, a test copy's, a resumed one, each other agent): the markers never reach
// it, the owner's provider does, the GM's memory is off by its own start (over the owner's 0), and a hire's is the
// owner's. And Smolcorp's own goes over what was scrubbed: the GM's token reaches the GM.
func TestEveryDeskStartHasNoClaudeSessionMarkerAndKeepsTheOwnersProvider(t *testing.T) {
	isolate(t)
	noLoginShell(t)
	openedInsideClaudeCode(t)
	for name, start := range everyKindOfDeskStart() {
		plan, err := start.plan()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		env, err := deskEnvironment(deskBaseEnvironment(), plan, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := maps.Clone(ownersProvider)
		if start.Role.ID == gmRole {
			want[claudeMemoryEnv] = "1"
		}
		noMarkerAndTheOwnersKept(t, name, env, want)
		if token, ok := environ(env, "SMOLCORP_GM_TOKEN", false); (start.Role.ID == gmRole) != (ok && token == start.GMToken && token != "") {
			t.Errorf("%s: the GM's token is there %v, want %v (its value isn't printed)", name, ok, start.Role.ID == gmRole)
		}
	}
}

// smolcorp run, the launcher, on each of its ways: the GM's and an engine hire's start through the app (the
// launcher rig's real start environments), a plain Claude hire, and a by-hand run of no hire. Names compared as
// each system does. The GM's token reaches the GM, and the launcher's own session reaches every agent.
func TestTheLaunchersAgentHasNoClaudeSessionMarkerAndKeepsTheOwnersProvider(t *testing.T) {
	r := newLRig(t)
	_, gmWindow := r.hire("gm")
	_, hireWindow := r.hire("janitor")
	openedInsideClaudeCode(t)
	for _, caseless := range []bool{false, true} {
		for _, c := range []struct {
			row          string
			hire, claude bool
			start        *hireStartEnv
		}{
			{"the GM through the app", true, true, &gmWindow.env},
			{"an engine hire through the app", true, true, &hireWindow.env},
			{"a plain Claude hire", true, true, nil},
			{"smolcorp run by hand", false, true, nil},
		} {
			env := agentEnvironment(os.Environ(), "fake-session", c.hire, c.claude, "", c.start, caseless)
			want := maps.Clone(ownersProvider)
			if c.start == &gmWindow.env {
				want[claudeMemoryEnv] = "1"
			}
			want["SMOLCORP_SESSION"] = "fake-session"
			noMarkerAndTheOwnersKept(t, c.row, env, want)
			token, ok := environ(env, "SMOLCORP_GM_TOKEN", caseless)
			if isGM := c.start == &gmWindow.env; isGM != (ok && token != "" && token == gmWindow.env.Set["SMOLCORP_GM_TOKEN"]) {
				t.Errorf("%s, caseless %v: the GM's token is there %v, want %v (its value isn't printed)", c.row, caseless, ok, isGM)
			}
		}
	}
}

// The GM and a hire, each in a real tmux on a socket of the test's own, started by desk-start from its ticket: the
// way every engine desk starts on a Mac and Linux (deskstart.go). The GM's token reaches the GM, never printed.
func TestADeskStartedByTicketGetsNoClaudeSessionMarker(t *testing.T) {
	path := needTmux(t)
	shortSupport(t)
	noLoginShell(t)
	sockets, err := os.MkdirTemp("", "es1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockets) })
	savedFolder, savedTmux := hireSocketFolder, tmuxPath
	hireSocketFolder, tmuxPath = func() string { return sockets }, func() string { return path }
	t.Cleanup(func() { hireSocketFolder, tmuxPath = savedFolder, savedTmux })
	t.Setenv(deskHelperVariable, "1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	openedInsideClaudeCode(t)
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	settings := writtenTestSettings(t, `{"x":"`+guard.SettingsSumBlank+`"}`)
	for i, c := range []struct {
		row   string
		start hireStart
	}{
		{"the GM", hireStart{Role: Role{ID: gmRole, Name: "General Manager", Agent: "claude"}, Message: "go", GMToken: testGMToken,
			Settings: settings, DefaultAccount: true}},
		{"a hire", hireStart{Role: testHireRole(), Message: "go", SendKey: "fake-send-key-es1", Settings: settings, DefaultAccount: true}},
	} {
		bin, written := envStandIn(t, "claude")
		// It stays, as an agent does, so its tmux is there to be asked (ended by its pid at the cleanup).
		appendFile(t, filepath.Join(bin, "claude"), "exec sleep 30\n")
		standInDeskProgram(t, filepath.Join(bin, "claude"))
		plan, err := c.start.plan()
		if err != nil {
			t.Fatal(err)
		}
		name := "es1-" + strconv.Itoa(i)
		h := newHireTmux(name, 0, nil)
		if h == nil {
			t.Fatal("no hire tmux")
		}
		command, err := deskStarts.issue(helper, name, h, plan, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, _, started, err := h.start(t.TempDir(), command)
		if err != nil {
			t.Fatalf("%s: tmux didn't start the desk", c.row)
		}
		deskStarts.pin(name, started)
		if server, err := strconv.Atoi(tmuxSays(t, path, h.socket, "display-message", "-p", "#{pid}")); err == nil && server > 1 {
			t.Cleanup(func() { _ = syscall.Kill(server, syscall.SIGTERM) })
		} else {
			t.Fatalf("the server's pid: %v", err)
		}
		untilHolds(t, c.row+"'s stand-in started", func() bool { return fileExists(written) })
		env := standInSaw(t, written)
		want := maps.Clone(ownersProvider)
		if c.start.Role.ID == gmRole {
			want[claudeMemoryEnv] = "1"
		}
		noMarkerAndTheOwnersKept(t, c.row, env, want)
		if token, ok := environ(env, "SMOLCORP_GM_TOKEN", false); (c.start.Role.ID == gmRole) != (ok && token == testGMToken) {
			t.Errorf("%s: the GM's token is there %v (its value isn't printed)", c.row, ok)
		}
		// tmux's own environment, which every pane on the server starts from, has none either.
		for _, line := range strings.Split(tmuxSays(t, path, h.socket, "show-environment", "-g"), "\n") {
			if name, _, set := strings.Cut(line, "="); set {
				if _, marker := inheritedMarkers[strings.ToUpper(name)]; marker {
					t.Errorf("%s: the desk's tmux holds %s", c.row, name)
				}
			}
		}
	}
}

// A plain Claude hire on Linux or Windows: its window, opened with hireEnvironment, runs the real launcher in a
// process of its own, which starts claude. And the launcher started by a terminal that kept the markers (one opened
// earlier inside the session, which Smolcorp didn't start) scrubs them itself.
func TestALaunchersClaudeGetsNoClaudeSessionMarker(t *testing.T) {
	d, h, _ := plainHireDeck(t)
	var windowEnv []string
	recorded := startHire
	startHire = func(cmd *exec.Cmd) error { windowEnv = cmd.Env; return recorded(cmd) }
	openedInsideClaudeCode(t)
	bin, written := envStandIn(t, "claude")
	// It says its version, before it writes anything down (a Mac starts it in its sandbox, its own switched off for
	// that version: SA-M10).
	script := filepath.Join(bin, "claude")
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, script, strings.Replace(string(body), "#!/bin/sh\n", "#!/bin/sh\n[ \"$1\" = --version ] && { echo '2.1.400 (Claude Code)'; exit 0; }\n", 1))
	args := plainClaudeHire(t, d, h, claudeDefaultName)
	if windowEnv == nil {
		t.Fatal("the window's environment wasn't seen")
	}
	noMarkerAndTheOwnersKept(t, "the hire's window", windowEnv, ownersProvider)
	for _, c := range []struct {
		row      string
		terminal []string
	}{
		{"the window Smolcorp opened", windowEnv},
		{"a terminal that kept the markers", os.Environ()},
	} {
		os.Remove(written)
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(append([]string{}, c.terminal...), launcherStandInVariable+"="+string(must(json.Marshal(args))),
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		cmd.Env = append(cmd.Env, launcherSandboxStandIn(t)...)
		cmd.Dir = t.TempDir()
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: the launcher: %v\n%s", c.row, err, output)
		}
		env := standInSaw(t, written)
		noMarkerAndTheOwnersKept(t, c.row, env, ownersProvider)
		if _, ok := environ(env, "SMOLCORP_SESSION", false); !ok {
			t.Errorf("%s: the launcher's own session doesn't reach claude", c.row)
		}
	}
}

// The GM and an engine hire through the launcher (the managed windows, and Windows' GM): the start the app hands
// the launcher, over this test's environment, as agentEnvironment makes it, given to a real stand-in. The GM's token
// reaches the GM.
func TestTheGMThroughTheLauncherGetsItsTokenAndNoClaudeSessionMarker(t *testing.T) {
	r := newLRig(t)
	_, gmWindow := r.hire("gm")
	_, hireWindow := r.hire("janitor")
	openedInsideClaudeCode(t)
	for _, c := range []struct {
		row   string
		start *hireStartEnv
		gm    bool
	}{{"the GM", &gmWindow.env, true}, {"a hire", &hireWindow.env, false}} {
		bin, written := envStandIn(t, "claude")
		cmd := exec.Command(filepath.Join(bin, "claude"))
		cmd.Env = agentEnvironment(os.Environ(), "fake-session", true, true, "", c.start, false)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", c.row, err, output)
		}
		env := standInSaw(t, written)
		want := maps.Clone(ownersProvider)
		if c.gm {
			want[claudeMemoryEnv] = "1"
		}
		noMarkerAndTheOwnersKept(t, c.row, env, want)
		token, ok := environ(env, "SMOLCORP_GM_TOKEN", false)
		if c.gm != (ok && token != "" && token == gmWindow.env.Set["SMOLCORP_GM_TOKEN"]) {
			t.Errorf("%s: the GM's token is there %v, want %v (its value isn't printed)", c.row, ok, c.gm)
		}
	}
}

// A summary's claude, and the sign-in check's, started with Smolcorp's environment.
func TestASummarysAndTheSignInChecksClaudeGetNoClaudeSessionMarker(t *testing.T) {
	openedInsideClaudeCode(t)
	bin, written := envStandIn(t, "claude")
	runSummaryCLI(providerClaude, filepath.Join(bin, "claude"), "SYSTEM", "Project: p", os.Environ(), providerReady)
	noMarkerAndTheOwnersKept(t, "a summary", standInSaw(t, written), map[string]string{"CLAUDE_CODE_GIT_BASH_PATH": "/fake/git/bash"})

	out := filepath.Join(t.TempDir(), "env")
	signInPlace(t, map[string]string{"claude": "env > " + out + "; echo '{\"loggedIn\":false}'; exit 1", "codex": "exit 1"})
	signInRows(t)
	noMarkerAndTheOwnersKept(t, "the sign-in check", standInSaw(t, out), map[string]string{"CLAUDE_CODE_GIT_BASH_PATH": "/fake/git/bash"})
}

// t3, which may start T3 Code's app, whose threads' Claude Code it starts itself.
func TestT3GetsNoClaudeSessionMarker(t *testing.T) {
	openedInsideClaudeCode(t)
	bin, written := envStandIn(t, "t3")
	if ran := runT3(filepath.Join(bin, "t3"), nil, 10*time.Second); !ran.Ended || ran.Code != 0 {
		t.Fatalf("t3: %+v", ran)
	}
	noMarkerAndTheOwnersKept(t, "t3", standInSaw(t, written), ownersProvider)
}

// The version check before each desk runs the claude a desk would start: it gets no marker either, though it only
// answers and ends (RT126ENVSCRUB1 N1).
func TestTheVersionChecksClaudeGetsNoClaudeSessionMarker(t *testing.T) {
	isolate(t)
	openedInsideClaudeCode(t)
	bin, written := envStandIn(t, "claude")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, ok := installedClaudeVersion(); !ok {
		t.Fatal("the stand-in claude didn't answer")
	}
	noMarkerAndTheOwnersKept(t, "the version check", standInSaw(t, written), ownersProvider)
}

// appendFile adds text at the end of the file at path.
func appendFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
