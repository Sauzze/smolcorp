package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// PLAINENV1 (SIERRA's ruling on FXSAX5R1's B2 finding): a plain hire of Claude Code (hire.go: a pinned desk, another
// agent, no role) runs `smolcorp run --hire …` in the owner's terminal. It runs on the account it's hired on, never
// the one the terminal's shell names: CLAUDE_CONFIG_DIR is taken out for the default account, and set to the account's
// folder, from setup's confirmed list, for another.

// plainClaudeHire hires Anyone (Claude Code, no role) the plain way on account, and gives the launcher's arguments,
// as its window runs them after `smolcorp run`.
func plainClaudeHire(t *testing.T, d *Deck, h *hiring, account string) []string {
	t.Helper()
	d.lastHire = time.Time{}
	opened := len(h.windows)
	if status, message := d.hireWith(hireSpec{account: account}); status != http.StatusNoContent {
		t.Fatalf("hire on %q: %d %s", account, status, message)
	}
	if len(h.windows) != opened+1 {
		t.Fatalf("hire on %q opened %d windows", account, len(h.windows)-opened)
	}
	command, _ := hiredWith(t, h.windows[opened])
	if command[len(command)-1] != "claude" || slices.Contains(command, "--hire-id") {
		t.Fatalf("hire on %q runs %q, want a plain hire of claude", account, command)
	}
	return command[2:]
}

// plainHireDeck is a deck hiring the plain way (no tmux), not a test copy, with a confirmed account "work".
func plainHireDeck(t *testing.T) (*Deck, *hiring, string) {
	t.Helper()
	d, h, _ := hiringDeck(t, "")
	saved := Port
	t.Cleanup(func() { Port = saved })
	Port = portFromEnvironment("")
	work := filepath.Join(homeDir, ".claude-work")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: work}}
	return d, h, work
}

// environ is name's value in env, and whether it's there.
func environ(env []string, name string, caseless bool) (string, bool) {
	for _, entry := range env {
		if key, value, _ := strings.Cut(entry, "="); key == name || caseless && strings.EqualFold(key, name) {
			return value, true
		}
	}
	return "", false
}

func TestAPlainClaudeHireStartsOnItsOwnAccountWhateverTheTerminalSets(t *testing.T) {
	d, h, work := plainHireDeck(t)
	other := filepath.Join(homeDir, ".claude-other")
	for _, c := range []struct {
		row, account string
		want         string // "" for taken out
	}{
		{"the default account", claudeDefaultName, ""},
		{"a confirmed account", "work", work},
	} {
		args := plainClaudeHire(t, d, h, c.account)
		run, err := parseRun(args)
		if err != nil {
			t.Fatalf("%s: the launcher refused %q: %v", c.row, args, err)
		}
		if run.claudeConfig != c.want {
			t.Errorf("%s: the launcher is told the folder %q, want %q", c.row, run.claudeConfig, c.want)
		}
		// The terminal's shell names another folder; on Windows, in any case.
		for _, caseless := range []bool{false, true} {
			name := "CLAUDE_CONFIG_DIR"
			if caseless {
				name = "Claude_Config_Dir"
			}
			env := plainHireEnv([]string{"PATH=/bin", name + "=" + other, "HOME=" + homeDir}, run.claudeConfig, caseless)
			got, set := environ(env, "CLAUDE_CONFIG_DIR", caseless)
			if c.want == "" && set || c.want != "" && (!set || got != c.want) {
				t.Errorf("%s (caseless %v): the agent's CLAUDE_CONFIG_DIR is %q (there: %v), want %q", c.row, caseless, got, set, c.want)
			}
			if _, path := environ(env, "PATH", false); !path {
				t.Errorf("%s: the rest of the environment went: %q", c.row, env)
			}
		}
	}
}

// The launcher takes an account's folder only as the app writes it: a full, clean path, on a plain hire alone. A
// non-Claude agent given one never starts.
func TestTheLauncherTakesAPlainHiresAccountFolderOnlyAsTheAppWritesIt(t *testing.T) {
	token := strings.Repeat("AB", 16)
	work := filepath.Join(homeDir, ".claude-work")
	for _, c := range []struct {
		args []string
		want string // the folder taken, or "-" for refused
	}{
		{[]string{"--hire", token, "--claude-config", work, "claude"}, work},
		{[]string{"--hire", token, "--claude-config", work, "--pinned", "claude"}, work},
		{[]string{"--hire", token, "claude"}, ""},
		{[]string{"--hire", token, "--claude-config", ".claude-work", "claude"}, "-"},
		{[]string{"--hire", token, "--claude-config", work + string(filepath.Separator) + ".." + string(filepath.Separator) + ".claude-x", "claude"}, "-"},
		{[]string{"--hire", token, "--claude-config", "", "claude"}, "-"},
		{[]string{"--hire", token, "--hire-id", "h1", "--claude-config", work, "--role", "tester"}, "-"},
		{[]string{"--claude-config", work, "claude"}, ""}, // not a hire's: the agent's own words, run as typed
	} {
		run, err := parseRun(c.args)
		switch {
		case c.want == "-" && err == nil:
			t.Errorf("%q was taken as %+v", c.args, run)
		case c.want != "-" && (err != nil || run.claudeConfig != c.want):
			t.Errorf("%q: %+v, %v; want the folder %q", c.args, run, err, c.want)
		}
	}
	if run, _ := parseRun([]string{"--claude-config", work, "claude"}); run == nil || !slices.Equal(run.command, []string{"--claude-config", work, "claude"}) {
		t.Errorf("typed by hand: %+v", run)
	}
	if code := launch(&launcherRun{hire: token, claudeConfig: work, command: []string{"codex"}}); code != 2 {
		t.Errorf("a Claude account given to codex: exit %d, want 2", code)
	}
}

// A test copy never starts a plain Claude hire, on any account: it would report to your own Smolcorp.
func TestATestCopyNeverStartsAPlainClaudeHire(t *testing.T) {
	d, h, _ := plainHireDeck(t)
	Port = 7799
	for _, account := range []string{claudeDefaultName, "work"} {
		d.lastHire = time.Time{}
		status, message := d.hireWith(hireSpec{account: account})
		if status != http.StatusConflict || account == "work" && message != plainHireInTestCopy {
			t.Errorf("a test copy's plain hire on %q: %d %s", account, status, message)
		}
	}
	if len(h.windows)+len(h.tmux) != 0 {
		t.Errorf("a test copy opened %q %q", h.windows, h.tmux)
	}
}

// The owner's own session on a Claude folder setup hasn't confirmed: its transcript stays refused, but not silently.
// Its ⓘ card says why, in one line, and setup's account list says one of your sessions is on such a folder. Confirming
// the folder takes the transcript at the next event and clears both. No line for a session on ~/.claude, for a desk's
// session (a desk is never nudged into being confirmed), for Smolcorp's own folders, or with no Claude folder at all.
func TestASessionOnAnUnconfirmedClaudeFolderSaysWhyItHasNoConversation(t *testing.T) {
	transcript := func(t *testing.T, path string) string {
		writeFile(t, path, reply("claude-opus-5-5")+"\n")
		return path
	}
	look := func(t *testing.T, d *Deck, id string) (taken bool, notice any, inUse any) {
		t.Helper()
		d.mu.Lock()
		taken = d.sessions.Get(id).Transcript != ""
		d.mu.Unlock()
		status, card := d.sessionInfo(id)
		if status != http.StatusOK {
			t.Fatalf("the card: %d %v", status, card)
		}
		accounts, _ := (&router{deck: d}).setupState()["claudeAccounts"].(map[string]any)
		return taken, card.(map[string]any)["notice"], accounts["unconfirmedInUse"]
	}
	for _, c := range []struct {
		row    string
		path   func(t *testing.T) string
		desk   bool
		notice bool
	}{
		{"an unconfirmed folder", func(t *testing.T) string {
			return transcript(t, filepath.Join(homeDir, ".claude-other", "projects", "-pe", "s1.jsonl"))
		}, false, true},
		{"~/.claude (the control)", func(t *testing.T) string {
			return transcript(t, filepath.Join(claudeProjects(t), "-pe", "s1.jsonl"))
		}, false, false},
		{"a desk's session on an unconfirmed folder", func(t *testing.T) string {
			return transcript(t, filepath.Join(homeDir, ".claude-other", "projects", "-pe", "s1.jsonl"))
		}, true, false},
		{"Smolcorp's own folder", func(t *testing.T) string {
			return transcript(t, filepath.Join(supportDir, "hires", "h1", "claude", "projects", "-pe", "s1.jsonl"))
		}, false, false},
		{"no Claude folder", func(t *testing.T) string {
			return transcript(t, filepath.Join(t.TempDir(), "s1.jsonl"))
		}, false, false},
	} {
		t.Run(c.row, func(t *testing.T) {
			d := newTestDeck(t)
			path := c.path(t)
			// A fresh map each time: the deck takes a refused transcript_path out of the one it's given.
			event := func() map[string]any {
				return map[string]any{"hook_event_name": "Stop", "session_id": "s1", "cwd": "/w", "transcript_path": path}
			}
			if c.desk {
				d.ingestAs(hookDesks{}, "h1", event(), "claude", "headless:pe-1", "", 0, "", 10)
			} else {
				d.Ingest(event(), "claude", "tty:/dev/ttys004", "", 10)
			}
			taken, notice, inUse := look(t, d, "s1")
			if c.notice {
				if taken || notice != unconfirmedFolderNotice || inUse != true {
					t.Errorf("taken %v, the card's line %q, setup's %v; want refused, %q and true", taken, notice, inUse, unconfirmedFolderNotice)
				}
			} else if notice != nil || inUse != false {
				t.Errorf("the card's line %q, setup's %v; want none", notice, inUse)
			}
			if !c.notice {
				return
			}
			// An event naming no transcript leaves the line as it was.
			d.Ingest(map[string]any{"hook_event_name": "UserPromptSubmit", "session_id": "s1", "cwd": "/w"}, "claude", "tty:/dev/ttys004", "", 20)
			if _, notice, inUse := look(t, d, "s1"); notice != unconfirmedFolderNotice || inUse != true {
				t.Errorf("after an event naming no transcript: %q, %v", notice, inUse)
			}
			// Confirmed in setup: the next event's transcript is taken, and neither line is left.
			d.mu.Lock()
			d.config.ClaudeProfiles = []claudeProfile{{Name: "other", Dir: filepath.Join(homeDir, ".claude-other")}}
			d.mu.Unlock()
			d.Ingest(event(), "claude", "tty:/dev/ttys004", "", 30)
			if taken, notice, inUse := look(t, d, "s1"); !taken || notice != nil || inUse != false {
				t.Errorf("once confirmed: taken %v, the card's line %q, setup's %v", taken, notice, inUse)
			}
		})
	}
}

// launcherStandInVariable makes this test program `smolcorp run` (TestMain), with the arguments it holds as JSON:
// the real launcher, in a process of its own, with a support folder of its own (no hook key: it never reaches an app).
const launcherStandInVariable = "SMOLCORP_LAUNCHER_STANDIN"

// launcherStandInOfficeVariable, when set, holds as JSON the config file and hook key file the stand-in reads in place
// of its own, {"config": …, "hookKey": …}: an office's, so it reaches that office's app, as a hire's launcher reaches
// Smolcorp (gmtokenwin_test.go).
const launcherStandInOfficeVariable = "SMOLCORP_LAUNCHER_STANDIN_OFFICE"

// launcherStandIn is the stand-in's run: RunLauncher with the arguments in the variable.
func launcherStandIn(encoded string) int {
	var args []string
	if json.Unmarshal([]byte(encoded), &args) != nil {
		return 3
	}
	if office := os.Getenv(launcherStandInOfficeVariable); office != "" {
		var files struct{ Config, HookKey string }
		if json.Unmarshal([]byte(office), &files) != nil || files.Config == "" || files.HookKey == "" {
			return 3
		}
		configFile, hookKeyFile = files.Config, files.HookKey
	}
	return RunLauncher(args)
}
