//go:build !windows

package desktop

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// deskHelperVariable makes this test binary act as `smolcorp desk-start`, for the row that starts a real pane on it.
const deskHelperVariable = "SMOLCORP_TEST_AS_DESK_START"

func init() {
	if os.Getenv(deskHelperVariable) == "1" && len(os.Args) > 1 && os.Args[1] == "desk-start" {
		os.Exit(RunDeskStart(os.Args[2:]))
	}
}

// issuedPlan is the plan hire hireID's start ticket holds, while it isn't taken, and whether there is one.
func issuedPlan(hireID string) (deskPlan, bool) {
	deskStarts.mu.Lock()
	defer deskStarts.mu.Unlock()
	for _, ticket := range deskStarts.tickets {
		if ticket.hireID == hireID {
			return ticket.plan, true
		}
	}
	return deskPlan{}, false
}

// startedSettings are the settings hire hireID's start gives its Claude Code, "" if none.
func startedSettings(t *testing.T, hireID string) string {
	t.Helper()
	if plan, ok := issuedPlan(hireID); ok {
		return scriptSettings(planText(plan))
	}
	return ""
}

// Off the Mac app, a desk on the API key reads it from the secret store when its pane asks (deskstart.go): it's never
// in the plan, a start without it stops, and the key replaces one the environment had.
func TestAnAPIKeyHireReadsTheKeyAsItStartsAndStopsWithoutIt(t *testing.T) {
	isolate(t)
	plan, _ := hireStart{Role: testHireRole(), Message: "go", APIKey: true}.plan()
	if !plan.APIKey || strings.Contains(planText(plan), "sk-ant") {
		t.Errorf("plan:\n%s", planText(plan))
	}
	somebodyElses := fakeAnthropicToken("api", "somebody-elses")
	base := []string{"PATH=/usr/bin", "ANTHROPIC_API_KEY=" + somebodyElses}
	for name, key := range map[string]func() string{"none": nil, "missing": func() string { return "" }, "not a key": func() string { return "two words" }} {
		if env, err := deskEnvironment(base, plan, key); err == nil || !strings.Contains(err.Error(), "couldn't read the API key") {
			t.Errorf("%s: a start without the key went on: %q %v", name, env, err)
		}
	}
	env, err := deskEnvironment(base, plan, func() string { return testAPIKey })
	if err != nil || !slices.Contains(env, "ANTHROPIC_API_KEY="+testAPIKey) || slices.Contains(env, "ANTHROPIC_API_KEY="+somebodyElses) {
		t.Errorf("the key should reach the agent, alone: %q %v", env, err)
	}
}

// paneTmux is a hire's tmux whose pane, asked, runs process pid.
func paneTmux(pid *int) *hireTmux {
	return &hireTmux{socket: "/s/smolcorp-sg2", run: func(args []string) (string, int, bool) {
		if slices.Contains(args, "display-message") && slices.Contains(args, "#{pane_pid}") {
			return strconv.Itoa(*pid), 0, true
		}
		return "", 0, true
	}}
}

// standInDeskProgram makes the agent a desk's ticket hands over the stand-in at program, for the test.
func standInDeskProgram(t *testing.T, program string) {
	t.Helper()
	saved := deskProgram
	deskProgram = func(string) (string, error) { return program, nil }
	t.Cleanup(func() { deskProgram = saved })
}

// standInHelper is the program a stand-in desk's pane runs as its helper: this test's process counts as running it,
// and no other process does (deskPeerRuns's own check is TestATicketOpensOnlyForTheProcessTmuxStartedForIt's).
const standInHelper = "/opt/smolcorp/bin/smolcorp"

func standInPeer(t *testing.T) {
	t.Helper()
	saved := deskPeerRuns
	deskPeerRuns = func(pid int, program string) bool { return pid == os.Getpid() && program == standInHelper }
	t.Cleanup(func() { deskPeerRuns = saved })
}

// noLoginShell keeps the desk's PATH from asking a login shell: it has said nothing.
func noLoginShell(t *testing.T) {
	t.Helper()
	loginPathMu.Lock()
	savedPath, savedKnown := loginPath, loginPathKnown.Load()
	loginPath = nil
	loginPathKnown.Store(true)
	loginPathMu.Unlock()
	t.Cleanup(func() {
		loginPathMu.Lock()
		loginPath = savedPath
		loginPathKnown.Store(savedKnown)
		loginPathMu.Unlock()
	})
}

// SG2: a desk's start ticket opens its start only for the process that is its pane's own. Given by any other
// process (one that read it from ps), it's refused and spent, so the pane's own try afterwards is refused too,
// and the desk's watch says why. A ticket taken late, or one that isn't Smolcorp's, opens nothing.
func TestATicketRedeemedByAnotherPidIsRefusedAndSpent(t *testing.T) {
	shortSupport(t)
	noLoginShell(t)
	standInDeskProgram(t, "/opt/stand-in/claude")
	standInPeer(t)
	pane := os.Getpid() + 1
	plan := deskPlan{Agent: "claude", Words: []string{"--", "go"}, Changes: []envChange{{Name: hireKeyEnvironment, Value: "fake-send-key-sg2"}}}
	command, err := deskStarts.issue(standInHelper, "sg2-other", paneTmux(&pane), plan, nil)
	if err != nil || len(command) != 5 || command[1] != "desk-start" || command[3] != strconv.Itoa(os.Getpid()) {
		t.Fatalf("issue: %q %v", command, err)
	}
	deskStarts.pin("sg2-other", pane)
	if strings.Contains(strings.Join(command, " "), "fake-send-key-sg2") {
		t.Fatalf("the pane's command holds the key: %q", command)
	}
	// This test's process isn't the pane's: refused, over the real socket, by its pid.
	launch, err := takeDeskStart(command[2:])
	if err != nil || launch.Error != deskTicketNotPane || launch.Program != "" || len(launch.Env) != 0 {
		t.Fatalf("another pid: %+v %v", launch, err)
	}
	// Spent: the pane itself can't use it now.
	pane = os.Getpid()
	if launch, _ := takeDeskStart(command[2:]); launch.Error != deskTicketUnknown {
		t.Fatalf("a spent ticket: %+v", launch)
	}
	if why := deskStarts.forget("sg2-other"); why != deskTicketNotPane {
		t.Errorf("the watch is told %q", why)
	}

	// The pane's own process, with a new ticket: its whole start, once.
	command, _ = deskStarts.issue(standInHelper, "sg2-own", paneTmux(&pane), plan, nil)
	deskStarts.pin("sg2-own", pane)
	launch, err = takeDeskStart(command[2:])
	if err != nil || launch.Error != "" || launch.Program != "/opt/stand-in/claude" || !slices.Equal(launch.Args, plan.Words) ||
		!slices.Contains(launch.Env, hireKeyEnvironment+"=fake-send-key-sg2") {
		t.Fatalf("the pane's own: %+v %v", launch, err)
	}
	if again, _ := takeDeskStart(command[2:]); again.Error != deskTicketUnknown {
		t.Errorf("taken twice: %+v", again)
	}
	// Late, and made up.
	command, _ = deskStarts.issue(standInHelper, "sg2-late", paneTmux(&pane), plan, nil)
	deskStarts.mu.Lock()
	deskStarts.tickets[command[4]].made = time.Now().Add(-deskStartWait - time.Second)
	deskStarts.mu.Unlock()
	if late, _ := takeDeskStart(command[2:]); late.Error != deskTicketLate {
		t.Errorf("late: %+v", late)
	}
	if made, _ := takeDeskStart([]string{command[2], command[3], strings.Repeat("0", 64)}); made.Error != deskTicketUnknown {
		t.Errorf("made up: %+v", made)
	}
}

// RT126SG2 B1: a ticket opens only for the process tmux started for it (its pid, as the call that started it
// said), while the pane still runs it, and only when that process runs Smolcorp's own program. A program of the
// owner's that drives the desk's tmux can swap what the pane runs and read the ticket from ps; what it starts is
// refused, and the ticket spent. Stand-ins only: a fake tmux says the pids, and each row checks a refusal's words.
func TestATicketOpensOnlyForTheProcessTmuxStartedForIt(t *testing.T) {
	shortSupport(t)
	noLoginShell(t)
	standInDeskProgram(t, "/opt/stand-in/claude")
	standInPeer(t)
	plan := deskPlan{Agent: "claude", Words: []string{"go"}}
	try := func(hireID string, started, pane int, cli string) deskLaunch {
		t.Helper()
		command, err := deskStarts.issue(cli, hireID, paneTmux(&pane), plan, nil)
		if err != nil {
			t.Fatal(err)
		}
		deskStarts.pin(hireID, started)
		launch, err := takeDeskStart(command[2:])
		if err != nil {
			t.Fatal(err)
		}
		if again, _ := takeDeskStart(command[2:]); again.Error != deskTicketUnknown {
			t.Errorf("%s: not spent: %+v", hireID, again)
		}
		return launch
	}
	me, helper := os.Getpid(), os.Getpid()+1
	// The helper tmux started was swapped out (respawned): the process the pane runs now is this one, and it gives
	// the ticket it read. It's the pane's own, but not the one the ticket is for.
	if got := try("sg2-swapped-in", helper, me, standInHelper); got.Error != deskTicketNotPane || got.Program != "" {
		t.Errorf("a process that became the pane: %+v", got)
	}
	// The process the ticket is for, after the pane was given to another.
	if got := try("sg2-swapped-out", me, helper, standInHelper); got.Error != deskTicketNotPane || got.Program != "" {
		t.Errorf("a pane that runs another process now: %+v", got)
	}
	// tmux said no pid it could have started.
	if got := try("sg2-no-pid", 0, me, standInHelper); got.Error != deskTicketNotPane {
		t.Errorf("no pid: %+v", got)
	}
	// The right process, in the pane, running a program that isn't Smolcorp's own (the system's check, below).
	deskPeerRuns = peerRunsProgram
	if got := try("sg2-not-ours", me, me, "/bin/sh"); got.Error != deskTicketNotOurs || got.Program != "" {
		t.Errorf("a program not Smolcorp's: %+v", got)
	}
	if why := deskStarts.forget("sg2-not-ours"); why != deskTicketNotOurs {
		t.Errorf("the watch is told %q", why)
	}
	// The system's check itself: this process runs this test's program, and not another.
	own, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !peerRunsProgram(me, own) || peerRunsProgram(me, "/bin/sh") || peerRunsProgram(me, filepath.Join(t.TempDir(), "gone")) {
		t.Error("the check of the process's program is wrong")
	}
}

// SG2: the helper gives its ticket only to the core that made it, known by its pid: a program listening on the
// socket's path in the core's place gets nothing.
func TestTheHelperGivesItsTicketOnlyToTheCore(t *testing.T) {
	folder, err := os.MkdirTemp("", "sg2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(folder) })
	path := filepath.Join(folder, "desk-start")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	heard := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 256)
		n, _ := conn.Read(buf)
		heard <- string(buf[:n])
	}()
	// This process listens; the helper is told the core is another.
	_, err = takeDeskStart([]string{path, strconv.Itoa(os.Getppid()), "the-ticket"})
	if err == nil || !strings.Contains(err.Error(), "isn't Smolcorp's") {
		t.Fatalf("an impostor's socket: %v", err)
	}
	if got := <-heard; got != "" {
		t.Errorf("the impostor was given %q", got)
	}
	for _, args := range [][]string{nil, {path, "1", "t"}, {path, "x", "t"}} {
		if _, err := takeDeskStart(args); err == nil {
			t.Errorf("%q went on", args)
		}
	}
}

// SANDBOX-PLAN §4 g5 and §3.4, with a real tmux on a socket of the test's own and a stand-in agent: a desk starts
// with no start script and no shell (the agent's pid is the pane's: tmux ran the helper, which exec'd the agent),
// with its key in the agent's environment alone: not in tmux's session environment, not in the pane's command,
// not in any file. What Smolcorp's environment had to wrap hooks isn't passed on, and the ticket is spent.
func TestADeskStartsByItsTicketWithNoScriptNoShellAndNoKeyInTmux(t *testing.T) {
	path := needTmux(t)
	shortSupport(t)
	noLoginShell(t)
	sockets, err := os.MkdirTemp("", "sg2t")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockets) })
	savedFolder, savedTmux := hireSocketFolder, tmuxPath
	hireSocketFolder, tmuxPath = func() string { return sockets }, func() string { return path }
	t.Cleanup(func() { hireSocketFolder, tmuxPath = savedFolder, savedTmux })
	t.Setenv(deskHelperVariable, "1")
	t.Setenv("CLAUDE_CODE_SHELL_PREFIX", "/evil/wrap.sh")
	t.Setenv("BASH_ENV", "/evil/bashenv.sh")
	// A home of the test's own, whose tmux start file sets a mark: the hire's server must read none (RT126SG2 B1,
	// g6), and never the owner's own (N6).
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".tmux.conf"), "set -g @sg2-start-file read\n")
	writeFile(t, filepath.Join(home, ".config", "tmux", "tmux.conf"), "set -g @sg2-start-file read\n")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	out := t.TempDir()
	agent := filepath.Join(out, "agent")
	writeFile(t, agent, "#!/bin/sh\necho $$ > '"+out+"/pid.tmp'\nenv > '"+out+"/env'\nprintf '%s\\n' \"$@\" > '"+out+"/args'\n"+
		"mv '"+out+"/pid.tmp' '"+out+"/pid'\nexec sleep 30\n")
	if err := os.Chmod(agent, 0o700); err != nil {
		t.Fatal(err)
	}
	standInDeskProgram(t, agent)
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const key = "fake-send-key-sg2-real"
	h := newHireTmux("sg2-real", 0, nil)
	if h == nil {
		t.Fatal("no hire tmux")
	}
	plan, err := hireStart{Role: testHireRole(), Message: "go", SendKey: key, Settings: `{"x":1}`, DefaultAccount: true, CLIDir: "/opt/smolcorp/bin"}.plan()
	if err != nil {
		t.Fatal(err)
	}
	command, err := deskStarts.issue(helper, "sg2-real", h, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, started, ok := h.start(out, command)
	if !ok {
		t.Fatal("tmux didn't start the desk")
	}
	deskStarts.pin("sg2-real", started)
	// The server ends by its own pid, never by a name or a search.
	if server, err := strconv.Atoi(tmuxSays(t, path, h.socket, "display-message", "-p", "#{pid}")); err == nil && server > 1 {
		t.Cleanup(func() { _ = syscall.Kill(server, syscall.SIGTERM) })
	} else {
		t.Fatalf("the server's pid: %v", err)
	}
	untilHolds(t, "the stand-in agent started", func() bool { return fileExists(filepath.Join(out, "pid")) })
	if mark := tmuxSays(t, path, h.socket, "show-options", "-gqv", "@sg2-start-file"); mark != "" {
		t.Errorf("the hire's tmux read a start file: %q", mark)
	}

	pid, _ := os.ReadFile(filepath.Join(out, "pid"))
	if pane, ok := h.panePID(); !ok || pane != started || strconv.Itoa(pane) != strings.TrimSpace(string(pid)) {
		t.Errorf("the agent's pid %s isn't the pane's %d: something ran between tmux and the agent", pid, pane)
	}
	env, _ := os.ReadFile(filepath.Join(out, "env"))
	lines := strings.Split(string(env), "\n")
	if !slices.Contains(lines, hireKeyEnvironment+"="+key) {
		t.Error("the agent's environment lacks its key")
	}
	for _, kept := range []string{"TERM=", "TMUX_PANE="} { // the terminal's own, from tmux
		if !slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, kept) }) {
			t.Errorf("the agent's environment lacks %s", kept)
		}
	}
	for _, bad := range []string{"CLAUDE_CODE_SHELL_PREFIX=", "BASH_ENV=", "CLAUDE_CONFIG_DIR=", "SMOLCORP_GM_TOKEN="} {
		if slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, bad) }) {
			t.Errorf("the agent's environment has %s", bad)
		}
	}
	if !slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, "PATH=/opt/smolcorp/bin:") }) {
		t.Error("the app's own folder isn't first on the agent's PATH")
	}
	args, _ := os.ReadFile(filepath.Join(out, "args"))
	if !strings.Contains(string(args), "--settings={\"x\":1}\n") || !strings.HasSuffix(string(args), "--\ngo\n") {
		t.Errorf("the agent's words: %q", args)
	}
	// Nowhere else: tmux's environments, the pane's command, the support folder.
	for _, asked := range [][]string{{"show-environment", "-g"}, {"show-environment", "-t", "=" + hireSession}, {"list-panes", "-a", "-F", "#{pane_start_command}"}} {
		if said := tmuxSays(t, path, h.socket, asked...); strings.Contains(said, key) {
			t.Errorf("tmux %q holds the key:\n%s", asked, said)
		}
	}
	if found := filesHolding(t, key, supportDir, sockets); len(found) != 0 {
		t.Errorf("files hold the key: %q", found)
	}
	if _, left := issuedPlan("sg2-real"); left {
		t.Error("the ticket wasn't spent")
	}
}

// tmuxSays is what tmux on socket prints for args.
func tmuxSays(t *testing.T, tmux, socket string, args ...string) string {
	t.Helper()
	out, err := exec.Command(tmux, append([]string{"-S", socket}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %q: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
