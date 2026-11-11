package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// A first GM's preflight with stand-ins: the terminals and tmux of fakeHiring, Claude Code found
// (or not) where told, and its sign-in as told. No real claude is run, nothing goes on the network.
func preflightDeck(t *testing.T) (*Deck, *hiring, *string, *signIn) {
	t.Helper()
	d, h, _ := hiringDeck(t, "")
	saved := []any{claudeFoundAt, claudeSignIn, hasProgram}
	t.Cleanup(func() {
		claudeFoundAt = saved[0].(func() string)
		claudeSignIn = saved[1].(func(string) signIn)
		hasProgram = saved[2].(func(string) bool)
	})
	at, state := "/opt/claude/bin/claude", signedIn
	claudeFoundAt = func() string { return at }
	claudeSignIn = func(program string) signIn {
		if program != at {
			t.Errorf("sign-in asked of %q, want %q", program, at)
		}
		return state
	}
	hasProgram = func(name string) bool { return name == "apt-get" }
	return d, h, &at, &state
}

func preflightLine(t *testing.T, report preflightReport, id string) preflightItem {
	t.Helper()
	for _, item := range report.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("no %q line in %+v", id, report.Items)
	return preflightItem{}
}

func preflightIDs(report preflightReport) []string {
	ids := []string{}
	for _, item := range report.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestTheGMPreflightIsReadyWhenEverythingIsThere(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	report := d.gmPreflight()
	if !report.Ready || !slices.Equal(preflightIDs(report), []string{"folder", "claude", "signin", "hooks", "terminal"}) {
		t.Fatalf("all there: ready %v, lines %q", report.Ready, preflightIDs(report))
	}
	for _, item := range report.Items {
		if !item.OK || item.Fix != "" || item.Action != nil {
			t.Errorf("line %+v, want ticked with nothing to do", item)
		}
	}
	if title := preflightLine(t, report, "terminal").Title; title != "Your hires open in Term term." {
		t.Errorf("terminal line %q", title)
	}
}

func TestTheGMPreflightPointsToTheFolderStep(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	d.config.HireFolder = ""
	report := d.gmPreflight()
	line := preflightLine(t, report, "folder")
	if report.Ready || line.OK || line.Action == nil || line.Action.Kind != "folder" || line.Title != "There's no folder for your hires yet." {
		t.Errorf("no folder: ready %v, %+v", report.Ready, line)
	}
	d.config.HireFolder = filepath.Join(t.TempDir(), "gone")
	if line := preflightLine(t, d.gmPreflight(), "folder"); line.OK || !strings.Contains(line.Title, "isn't there any more") || line.Action.Kind != "folder" {
		t.Errorf("a folder that's gone: %+v", line)
	}
}

func TestTheGMPreflightSaysHowToInstallClaudeCode(t *testing.T) {
	d, h, at, _ := preflightDeck(t)
	*at = ""
	h.missing = "claude isn't installed here."
	report := d.gmPreflight()
	line := preflightLine(t, report, "claude")
	// Windows installs it with its own line, pasted in PowerShell; a Mac or Linux with the script, in a terminal.
	install, shell := claudeInstallUnix, "Paste this in a terminal,"
	if runtime.GOOS == "windows" {
		install, shell = claudeInstallWindows, "Paste this in PowerShell,"
	}
	if report.Ready || line.OK || line.Action == nil || line.Action.Kind != "copy" || line.Action.Command != install {
		t.Fatalf("not installed: ready %v, %+v", report.Ready, line)
	}
	if line.Title != "Claude Code isn't installed on this computer." || !strings.HasPrefix(line.Fix, shell) || !strings.Contains(line.Fix, "Check again") {
		t.Errorf("not installed says %q / %q", line.Title, line.Fix)
	}
	if slices.Contains(preflightIDs(report), "signin") || slices.Contains(preflightIDs(report), "hooks") {
		t.Errorf("sign-in and hooks asked of a Claude Code that isn't there: %q", preflightIDs(report))
	}
}

func TestTheGMPreflightGivesTheLineForALoginShellThatDoesntFindClaude(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hires on Windows use Windows' own PATH: there's no PATH line")
	}
	d, h, at, _ := preflightDeck(t)
	*at = filepath.Join(homeDir, ".local", "bin", "claude")
	h.missing = "your login shell doesn't find claude"
	t.Setenv("SHELL", "/bin/zsh")
	line := preflightLine(t, d.gmPreflight(), "claude")
	want := `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zprofile`
	if line.OK || line.Action == nil || line.Action.Command != want || !strings.Contains(line.Title, "~/.local/bin") {
		t.Errorf("found elsewhere: %+v, want the command %q", line, want)
	}
	t.Setenv("SHELL", "/usr/bin/fish")
	if line := preflightLine(t, d.gmPreflight(), "claude"); line.Action.Command != "fish_add_path ~/.local/bin" {
		t.Errorf("fish: %q", line.Action.Command)
	}
}

// Signed out is said, never in the way: an API key or a cloud provider may work where the check can't see it.
func TestTheGMPreflightAsksClaudeCodeIfItIsSignedIn(t *testing.T) {
	d, _, _, state := preflightDeck(t)
	*state = signedOut
	report := d.gmPreflight()
	line := preflightLine(t, report, "signin")
	if !report.Ready || line.OK || !line.Optional || line.Action == nil || line.Action.Command != "claude" || !strings.Contains(line.Fix, "/login") ||
		!strings.HasPrefix(line.Fix, "Hire anyway: its window asks you to sign in.") {
		t.Errorf("signed out: ready %v, %+v", report.Ready, line)
	}
	// Couldn't tell: said, but not in the way.
	*state = signInUnknown
	report = d.gmPreflight()
	if line := preflightLine(t, report, "signin"); !report.Ready || line.OK || !line.Optional {
		t.Errorf("sign-in unknown: ready %v, %+v", report.Ready, line)
	}
}

func TestSignInIsWhatClaudeCodeSays(t *testing.T) {
	for said, want := range map[string]signIn{
		`{"loggedIn":true,"authMethod":"claude.ai"}`: signedIn,
		`{"loggedIn":false}`:                         signedOut,
		`{}`:                                         signInUnknown,
		`error: unknown command 'auth'`:              signInUnknown,
		``:                                           signInUnknown,
		// Asked through your login shell, whose files may print first.
		"Welcome back {you}\n{\"loggedIn\":true}\n": signedIn,
		"motd\n{\n  \"loggedIn\": false\n}\n":       signedOut,
	} {
		if got := signInFrom([]byte(said)); got != want {
			t.Errorf("%q: %v, want %v", said, got, want)
		}
	}
}

func TestTheGMPreflightOffersConnectWhenClaudeDoesntReport(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	hooksNotConnected = func(agent Agent) string {
		if agent == Claude {
			return "Claude Code isn't connected."
		}
		return ""
	}
	report := d.gmPreflight()
	line := preflightLine(t, report, "hooks")
	if report.Ready || line.OK || line.Action == nil || line.Action.Kind != "fix" || line.Action.Fix != fixRepair || line.Action.Label != "Connect" {
		t.Errorf("hooks not connected: ready %v, %+v", report.Ready, line)
	}
}

// A step not taken yet (no folder, Claude Code to install, Connect to press) is To do on the page, never red;
// a real problem (a folder gone, no terminal) isn't. The Mac says the same (GMPreflightTests).
func TestTheGMPreflightTellsAStepToTakeFromAProblem(t *testing.T) {
	d, h, at, _ := preflightDeck(t)
	d.config.HireFolder = ""
	if line := preflightLine(t, d.gmPreflight(), "folder"); !line.Todo {
		t.Errorf("no folder yet is a step to take: %+v", line)
	}
	d.config.HireFolder = filepath.Join(t.TempDir(), "gone")
	if line := preflightLine(t, d.gmPreflight(), "folder"); line.Todo {
		t.Errorf("a folder that's gone is a problem: %+v", line)
	}
	hooksNotConnected = func(agent Agent) string { return "Claude Code isn't connected." }
	if line := preflightLine(t, d.gmPreflight(), "hooks"); !line.Todo {
		t.Errorf("Connect not pressed yet is a step to take: %+v", line)
	}
	// The same word on the button as Start here's own line for the agents.
	if c := hooksCheck([]Agent{Claude}, func(Agent) bool { return false }, func(Agent) (bool, string) { return true, "" }); c.Label != "Connect" {
		t.Errorf("Start here's agents line says %q, the preflight says Connect", c.Label)
	}
	canOpenWindows = func() bool { return false }
	if line := preflightLine(t, d.gmPreflight(), "terminal"); line.OK || line.Todo {
		t.Errorf("no terminal is a problem: %+v", line)
	}
	*at = ""
	h.missing = "claude isn't installed here."
	if line := preflightLine(t, d.gmPreflight(), "claude"); !line.Todo {
		t.Errorf("Claude Code to install is a step to take: %+v", line)
	}
	said, _ := json.Marshal(d.gmPreflight())
	if !strings.Contains(string(said), `"todo":true`) {
		t.Errorf("the page reads todo: %s", said)
	}
}

// A test copy's GM can't be hired on this computer (testCopyRefusal): its line says so before Hire, and Hire waits.
// The real app hires on your own account: no such line (the Mac's GMPreflightTests say the same for its test copy).
func TestATestCopySaysItsGMNeedsAnAccountBeforeHire(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	onPort(t, portFromEnvironment(""))
	if ids := preflightIDs(d.gmPreflight()); slices.Contains(ids, "account") {
		t.Errorf("the real app has an account line: %v", ids)
	}
	onPort(t, portFromEnvironment("")+1)
	report := d.gmPreflight()
	line := preflightLine(t, report, "account")
	// Red (!), not To do as on the Mac: there's no step to take here (R790).
	if line.OK || line.Optional || line.Todo || line.Action != nil || line.Title != gmNeedsAccount || !strings.Contains(line.Fix, "real Smolcorp") || report.Ready {
		t.Errorf("a test copy's account line: %+v (ready %v)", line, report.Ready)
	}
	if !strings.Contains(gmNeedsAccount, "Claude account of its own") {
		t.Errorf("not the Mac's words: %q", gmNeedsAccount)
	}
}

func TestTheGMPreflightWithoutATerminalSaysHowToInstallTmux(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	canOpenWindows = func() bool { return false }
	report := d.gmPreflight()
	line := preflightLine(t, report, "terminal")
	if report.Ready || line.OK || line.Action == nil || line.Action.Command != "sudo apt install tmux" {
		t.Errorf("no terminal, no tmux: ready %v, %+v", report.Ready, line)
	}
	hasProgram = func(string) bool { return false }
	if line := preflightLine(t, d.gmPreflight(), "terminal"); line.Action != nil || !strings.Contains(line.Fix, "package manager") {
		t.Errorf("no package manager known: %+v", line)
	}
	tmuxPath = func() string { return "/usr/bin/tmux" }
	report = d.gmPreflight()
	if line := preflightLine(t, report, "terminal"); !report.Ready || !line.OK || !strings.Contains(line.Title, "tmux") {
		t.Errorf("no desktop, with tmux: ready %v, %+v", report.Ready, line)
	}
}

func TestTheGMPreflightSaysTmuxIsMissingWithoutStoppingTheHire(t *testing.T) {
	if !hasTmuxHires {
		t.Skip("no tmux hires here")
	}
	d, _, _, _ := preflightDeck(t)
	if slices.Contains(preflightIDs(d.gmPreflight()), "tmux") {
		t.Error("a tmux line without hires in tmux")
	}
	d.config.HireInTmux = true
	report := d.gmPreflight()
	line := preflightLine(t, report, "tmux")
	if !report.Ready || line.OK || !line.Optional || line.Action == nil || line.Action.Command != "sudo apt install tmux" {
		t.Errorf("tmux missing: ready %v, %+v", report.Ready, line)
	}
	tmuxPath = func() string { return "/usr/bin/tmux" }
	if line := preflightLine(t, d.gmPreflight(), "tmux"); !line.OK {
		t.Errorf("tmux there: %+v", line)
	}
}

// The route answers the page with the manager off, as on a first run, in the Mac's JSON (GMPreflight.swift).
func TestTheGMPreflightRouteAnswersBeforeTheManagerIsOn(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	d.config.HireFolder = ""
	code, text := pageCall(t, d, "GET", "/setup/hire/gm/preflight")
	if code != http.StatusOK || d.config.ManagerEnabled {
		t.Fatalf("preflight with the manager off: %d %q", code, text)
	}
	var report struct {
		Ready *bool            `json:"ready"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(text), &report); err != nil || report.Ready == nil || *report.Ready || len(report.Items) == 0 {
		t.Fatalf("answer %q: %v", text, err)
	}
	for _, item := range report.Items {
		keys := []string{}
		for key := range item {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, []string{"action", "fix", "id", "ok", "optional", "title", "todo"}) {
			t.Errorf("line %v has keys %q", item, keys)
		}
	}
	folder := report.Items[0]["action"].(map[string]any)
	if folder["kind"] != "folder" || folder["label"] != "Show me" {
		t.Errorf("the folder line's action: %v", folder)
	}
}

// One click: the GM role, in the folder for new hires, nothing asked.
func TestTheGMHireWithDefaultsStartsTheGMRoleInTheHiresFolder(t *testing.T) {
	d, h, _, _ := preflightDeck(t)
	if !d.gmPreflight().Ready {
		t.Fatal("the preflight isn't ready")
	}
	folder := d.config.HireFolder
	if code, text := pageCall(t, d, "POST", "/setup/hire/gm"); code != http.StatusNoContent || len(h.windows) != 1 {
		t.Fatalf("the GM hire: %d %q, %d windows", code, text, len(h.windows))
	}
	command, _ := hiredWith(t, h.windows[0])
	if h.dirs[0] != folder || !slices.Contains(command, "--role") || command[slices.Index(command, "--role")+1] != "gm" {
		t.Errorf("the GM hire runs %q in %q, want the gm role in %q", command, h.dirs[0], folder)
	}
}

// With no folder for your hires, the GM isn't hired in your home folder: refused, and nothing switched on.
func TestTheGMHireWithoutAHiresFolderIsRefused(t *testing.T) {
	for _, folder := range []string{"", filepath.Join(t.TempDir(), "gone")} {
		d, h, _, _ := preflightDeck(t)
		d.config.HireFolder = folder
		code, text := pageCall(t, d, "POST", "/setup/hire/gm")
		if code != http.StatusConflict || text != chooseHireFolderFirst || len(h.windows) != 0 {
			t.Errorf("folder %q: %d %q, %d windows, want 409 %q and none", folder, code, text, len(h.windows), chooseHireFolderFirst)
		}
		if d.config.ManagerEnabled || d.config.HiringEnabled != true {
			t.Errorf("folder %q: a refused hire changed the switches (manager %v, hiring %v)", folder, d.config.ManagerEnabled, d.config.HiringEnabled)
		}
	}
}

// Hire and fire from the office may be off for a newcomer: the GM's hire turns it on, and back off if it fails.
func TestTheGMHireTurnsHiringOnAndBackOffIfItFails(t *testing.T) {
	d, h, _, _ := preflightDeck(t)
	d.config.HiringEnabled = false
	if code, text := pageCall(t, d, "POST", "/setup/hire/gm"); code != http.StatusNoContent || len(h.windows) != 1 {
		t.Fatalf("the GM hire with hiring off: %d %q, %d windows", code, text, len(h.windows))
	}
	if !d.config.HiringEnabled || !loadConfig().HiringEnabled {
		t.Error("the GM hire left hiring off")
	}
	d, h, _, _ = preflightDeck(t)
	d.config.HiringEnabled = false
	h.missing = "claude isn't installed here."
	if code, _ := pageCall(t, d, "POST", "/setup/hire/gm"); code != http.StatusConflict {
		t.Fatalf("a GM hire that fails: %d, want 409", code)
	}
	if d.config.HiringEnabled || loadConfig().HiringEnabled {
		t.Error("a failed GM hire left hiring on")
	}
}

// Asked as the hire starts Claude Code: the same command shape (your login shell where there's one) and the
// same environment, keys and cloud providers kept, as the Mac asks it (GMPreflight.signInArguments). The GM
// works on ~/.claude's account, so CLAUDE_CONFIG_DIR is cleared, from the app's environment and after your
// shell's files, as the GM's start script and the launcher clear it.
func TestTheSignInIsAskedAsTheHireStartsIt(t *testing.T) {
	command := signInCommand("/x/bin/claude")
	if runtime.GOOS == "windows" {
		if !slices.Equal(command, []string{"/x/bin/claude", "auth", "status", "--json"}) {
			t.Errorf("sign-in command %q", command)
		}
	} else {
		if !slices.Equal(command, inLoginShell([]string{"env", "-u", "CLAUDE_CONFIG_DIR", "/x/bin/claude", "auth", "status", "--json"})) {
			t.Errorf("sign-in command %q", command)
		}
		if command[0] != loginShell() || command[1] != "-lic" {
			t.Errorf("sign-in not through the login shell: %q", command)
		}
	}
	t.Setenv("ANTHROPIC_API_KEY", "k-fake")
	t.Setenv("SMOLCORP_SESSION", "s1")
	t.Setenv("CLAUDE_CONFIG_DIR", "/x/other-account")
	env := signInEnvironment()
	if !slices.Contains(env, "ANTHROPIC_API_KEY=k-fake") || slices.Contains(env, "SMOLCORP_SESSION=s1") {
		t.Errorf("the hire's environment: key kept %v, session dropped %v", slices.Contains(env, "ANTHROPIC_API_KEY=k-fake"), !slices.Contains(env, "SMOLCORP_SESSION=s1"))
	}
	if slices.ContainsFunc(env, func(variable string) bool { return strings.HasPrefix(variable, "CLAUDE_CONFIG_DIR=") }) {
		t.Error("CLAUDE_CONFIG_DIR kept: the GM's hire runs on ~/.claude's account")
	}
}

// Whether a GM is hired is in the setup state with the manager off too, so Start here doesn't look again
// every 10 s for an office that has one (the Mac's "gmHired").
func TestTheSetupStateSaysWhetherAGMIsHired(t *testing.T) {
	d := newTestDeck(t)
	d.config.ManagerEnabled = false
	hired := func() any { return (&router{deck: d}).setupStateFor(callerPage)["gmHired"] }
	if got := hired(); got != false {
		t.Errorf("no registry: gmHired %v", got)
	}
	registry := func(hires ...Hire) {
		data, _ := json.Marshal(map[string]any{"version": 1, "hires": hires})
		if err := os.WriteFile(registryPath(), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registry(Hire{ID: "h1", Name: "General Manager", Folder: "/x/hires", Role: gmRole, State: "working"})
	if got := hired(); got != true {
		t.Errorf("a GM at work: gmHired %v", got)
	}
	registry(Hire{ID: "h1", Name: "General Manager", Folder: "/x/hires", Role: gmRole, State: "fired"},
		Hire{ID: "h2", Name: "Janitor", Folder: "/x/hires", Role: "janitor", State: "working"})
	if got := hired(); got != false {
		t.Errorf("a fired GM: gmHired %v", got)
	}
}

// The native installer's ~/.local/bin/claude is a link into a versions folder: the line names the link's
// folder, which holds a claude once it's on the PATH, never the versions folder, which holds none.
func TestTheGMPreflightNamesTheFolderOfALinkNotWhereItLeads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need rights on Windows, and its hires use Windows' own PATH")
	}
	root := t.TempDir()
	versions, bin := filepath.Join(root, "share", "claude", "versions"), filepath.Join(root, "bin")
	for _, folder := range []string{versions, bin} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	program := filepath.Join(versions, "2.1.319")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(program, filepath.Join(bin, "claude")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	found := claudeFoundAt()
	if found != filepath.Join(bin, "claude") {
		t.Fatalf("found at %q, want the link %q", found, filepath.Join(bin, "claude"))
	}
	d, h, at, _ := preflightDeck(t)
	*at = found
	h.missing = "your login shell doesn't find claude"
	t.Setenv("SHELL", "/bin/zsh")
	line := preflightLine(t, d.gmPreflight(), "claude")
	want := `echo 'export PATH="` + bin + `:$PATH"' >> ~/.zprofile`
	if line.Action == nil || line.Action.Command != want || strings.Contains(line.Title, "versions") {
		t.Errorf("a link: %+v, want the command %q", line, want)
	}
}

// A folder with a quote, a $, a backquote or a space: the line still writes that folder, whole.
func TestThePathLineQuotesAnOddFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hires on Windows use Windows' own PATH: there's no PATH line")
	}
	odd := filepath.Join(homeDir, "Bob's \"apps\"", "$bin`x`")
	got := pathLine("/bin/zsh", odd)
	want := `echo 'export PATH="$HOME/Bob'\''s \"apps\"/\$bin\` + "`x\\`" + `:$PATH"' >> ~/.zprofile`
	if got != want {
		t.Fatalf("pathLine:\n got %s\nwant %s", got, want)
	}
	if fish := pathLine("/usr/bin/fish", filepath.Join(homeDir, "Bob's apps")); fish != `fish_add_path '`+filepath.Join(homeDir, `Bob\'s apps`)+`'` {
		t.Errorf("fish: %s", fish)
	}
	// What the echo writes, read back by a shell as a profile would. Only sh, on a command made here: nothing is written.
	sh := func(command string, env ...string) string {
		cmd := exec.Command("/bin/sh", "-c", command)
		cmd.Env = append([]string{"PATH=/usr/bin:/bin"}, env...)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("sh -c %q: %v", command, err)
		}
		return strings.TrimRight(string(out), "\n")
	}
	written := sh(strings.TrimSuffix(got, " >> ~/.zprofile"))
	if path := sh(written+`; printf %s "$PATH"`, "HOME="+homeDir); path != odd+":/usr/bin:/bin" {
		t.Errorf("the profile's PATH: %q, want %q", path, odd+":/usr/bin:/bin")
	}
}

// The preflight is the page's alone: it starts your login shell and Claude Code, so a tool with the setup-key file can't.
func TestTheGMPreflightIsThePagesAlone(t *testing.T) {
	d, _, _, _ := preflightDeck(t)
	w := serve(t, d, call{method: "GET", target: "/setup/hire/gm/preflight", host: "localhost:7710", from: thisComputer, asTool: true,
		headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusForbidden {
		t.Errorf("a tool's preflight: %d %q, want 403", w.Code, w.Body.String())
	}
	if !routeAllows(callerPage, "GET", "/setup/hire/gm/preflight") || routeAllows(callerTool, "GET", "/setup/hire/gm/preflight") || routeAllows(callerGM, "GET", "/setup/hire/gm/preflight") {
		t.Error("the table doesn't keep the preflight to the page")
	}
}

// One GM hire at a time: a second, while the first runs, is refused before it reads or flips a switch.
func TestASecondGMHireWhileOneRunsIsRefusedAndLeavesTheSwitches(t *testing.T) {
	d, h, _, _ := preflightDeck(t)
	d.config.ManagerEnabled, d.config.HiringEnabled = false, false
	d.gmHiring.Lock()
	code, text := pageCall(t, d, "POST", "/setup/hire/gm")
	d.gmHiring.Unlock()
	if code != http.StatusConflict || text != gmBeingHired || len(h.windows) != 0 {
		t.Errorf("a GM hire while one runs: %d %q, %d windows, want 409 %q", code, text, len(h.windows), gmBeingHired)
	}
	if d.config.ManagerEnabled || d.config.HiringEnabled {
		t.Errorf("the switches changed (manager %v, hiring %v)", d.config.ManagerEnabled, d.config.HiringEnabled)
	}
	if code, text := pageCall(t, d, "POST", "/setup/hire/gm"); code != http.StatusNoContent || len(h.windows) != 1 {
		t.Errorf("the hire after it: %d %q, %d windows", code, text, len(h.windows))
	}
}

// The page the Linux/Windows app serves: General's agents and login rows say what Start here says, a step not
// taken yet is To do, a refusal under Hire carries a button to its fix, and the lines look again by themselves.
func TestStartHereAndGeneralAgreeAndPointAtTheFix(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer})
	if page.Code != http.StatusOK {
		t.Fatalf("the setup page: %d", page.Code)
	}
	html := page.Body.String()
	for _, want := range []string{
		"const sameAsStartHere = { hooks: 'step-hooks', accessibility: 'step-accessibility', login: 'step-login' };",
		`<div class="title">Your agents report to Smolcorp</div>`,
		`<div class="title">Smolcorp opens at login</div>`,
		"const status = item.ok ? 'done' : item.optional ? 'info' : item.todo ? 'todo' : 'fix';",
		"startSaid(why, true, hireRefusalFix(why));",
		"if (why.startsWith('Connect your agents first')) return { label: 'Connect',",
		"document.addEventListener('smolcorp:hire-folder', (e) => folderMoved(e.detail && e.detail.folder));",
		"<b>Change where hires work</b>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the page doesn't have %q", want)
		}
	}
	// The words the page answers with a button are this app's own.
	if !strings.HasPrefix(setupHireNeedsFolder, "Choose the folder for your hires first") || !strings.HasPrefix(chooseHireFolderFirst, "Choose the folder for your hires first") {
		t.Error("the folder refusal's words changed: setup.html's hireRefusalFix answers them")
	}
}
