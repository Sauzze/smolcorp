package desktop

import (
	"encoding/json"
	"net/http"
	"path/filepath"
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
	if report.Ready || line.OK || line.Action == nil || line.Action.Kind != "copy" || line.Action.Command != claudeInstallUnix {
		t.Fatalf("not installed: ready %v, %+v", report.Ready, line)
	}
	if line.Title != "Claude Code isn't installed on this computer." || !strings.Contains(line.Fix, "Check again") {
		t.Errorf("not installed says %q / %q", line.Title, line.Fix)
	}
	if slices.Contains(preflightIDs(report), "signin") || slices.Contains(preflightIDs(report), "hooks") {
		t.Errorf("sign-in and hooks asked of a Claude Code that isn't there: %q", preflightIDs(report))
	}
}

func TestTheGMPreflightGivesTheLineForALoginShellThatDoesntFindClaude(t *testing.T) {
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

func TestTheGMPreflightAsksClaudeCodeIfItIsSignedIn(t *testing.T) {
	d, _, _, state := preflightDeck(t)
	*state = signedOut
	report := d.gmPreflight()
	line := preflightLine(t, report, "signin")
	if report.Ready || line.OK || line.Optional || line.Action == nil || line.Action.Command != "claude" || !strings.Contains(line.Fix, "/login") {
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
		if !slices.Equal(keys, []string{"action", "fix", "id", "ok", "optional", "title"}) {
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
