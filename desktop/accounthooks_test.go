package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// ownersSettings is a stand-in for an account's settings.json, full of the owner's own hooks and keys, written the
// way the Swift engine's JSONSerialization and a hand both write: " : ", escaped slashes, \u escapes, a number as
// written, tabs, a CRLF, no newline at the end. The marks are JSON strings: {{SWIFT}} and {{OLD}} Smolcorp's own
// entries (report.sh as the Swift engine wrote it, this program at the place it had before), {{STATUS}} the same
// report.sh as a status line, which is no hook, {{MINE}} the owner's report.sh with an argument of their own.
const ownersSettings = `{
  "$schema" : "https:\/\/json.schemastore.org\/claude-code-settings.json",
  "env" : {
    "GREETING" : "café 😀 \"quoted\" <b>&amp;</b>",
    "SIZE" : 1.50e+3,{{CRLF}}    "EMPTY" : ""
  },
  "hooks" : {
    "PreToolUse" : [
      {
        "matcher" : "Bash|Write",
        "hooks" : [
          { "type": "command", "command": "~/bin/my-guard.sh \"$TOOL\" | tee -a ~/log", "timeout": 30 },
          {
            "async" : true,
            "command" : {{SWIFT}},
            "type" : "command"
          }
        ]
      }
    ],
	"Stop":[{"hooks":[{"command":{{SWIFT}},"type":"command","async":true},{"type":"command","command":"say smolcorp hook done"},{"type":"command","command":{{MINE}}}]}],
    "SessionStart" : [ { "hooks" : [ { "type" : "command", "command" : {{OLD}}, "async" : true } ] },
                       { "matcher" : "startup", "hooks" : [ { "type" : "prompt", "prompt" : "Say hi." } ] } ],
    "Notification" : [ { "hooks" : [ { "type" : "command", "command" : "\/elsewhere\/report.sh" } ] } ]
  },
  "statusLine" : { "type" : "command", "command" : {{STATUS}} },
  "permissions" : { "allow" : [ "Bash(git status)", "Read(~\/notes\/**)" ], "deny" : [ ], "defaultMode" : "default" },
  "voice" : { "enabled" : false, "mode" : "hold" },
  "model" : "opus"
}`

// swiftEscaped is s as a JSON string the way JSONSerialization writes it, with its slashes escaped.
func swiftEscaped(t *testing.T, s string) string {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "/", `\/`)
}

func fill(template string, marks map[string]string) string {
	for mark, value := range marks {
		template = strings.ReplaceAll(template, "{{"+mark+"}}", value)
	}
	return template
}

// accountUnderTheMacApp is a test under the Mac app on the default port, where report.sh is the deck's, with an account's
// folder in the stand-in home; it returns that folder's settings.json and the Swift engine's command.
func accountUnderTheMacApp(t *testing.T, folder string) (settings, swift string) {
	t.Helper()
	isolate(t)
	onDefaultPort(t)
	t.Cleanup(func() { hookHeaderFile = "" })
	hookHeaderFile = filepath.Join(supportDir, "hook-header")
	if err := os.MkdirAll(filepath.Join(homeDir, folder), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(homeDir, folder, "settings.json"), `"` + filepath.Join(supportDir, "report.sh") + `"`
}

// accountMacDeck is a test deck under the Mac app on the default port, and the Swift engine's command in its support folder.
func accountMacDeck(t *testing.T) (*Deck, string) {
	t.Helper()
	d := newTestDeck(t)
	onDefaultPort(t)
	t.Cleanup(func() { hookHeaderFile = "" })
	hookHeaderFile = filepath.Join(supportDir, "hook-header")
	return d, `"` + filepath.Join(supportDir, "report.sh") + `"`
}

func backupsOf(t *testing.T, file string) []string {
	t.Helper()
	found, err := filepath.Glob(file + ".smolcorp-*.bak")
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// SIERRA's row 3: the owner's own hooks, keys and formatting come out byte-identical; only the command of
// Smolcorp's own entries changes, where it stands. The file as it was is kept beside it, named for the time, first.
func TestAnAccountsRefreshChangesOnlySmolcorpsCommandsByteForByte(t *testing.T) {
	file, swift := accountUnderTheMacApp(t, ".claude-work")
	marks := map[string]string{
		"SWIFT":  swiftEscaped(t, swift),
		"OLD":    swiftEscaped(t, "/old/place/smolcorp hook"),
		"STATUS": swiftEscaped(t, swift),
		"MINE":   swiftEscaped(t, swift+" --mine"),
		"CRLF":   "\r\n",
	}
	before := fill(ownersSettings, marks)
	if !json.Valid([]byte(before)) {
		t.Fatal("the stand-in isn't valid JSON")
	}
	if err := os.WriteFile(file, []byte(before), 0o640); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 15, 7, 15, 0, 0, time.Local)
	if err := refreshAccountSettings(file, now); err != nil {
		t.Fatal(err)
	}
	ours, err := encodeSettingsValue(hookCommand(Claude))
	if err != nil {
		t.Fatal(err)
	}
	marks["SWIFT"], marks["OLD"] = string(ours), string(ours)
	want := fill(ownersSettings, marks)
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("the refreshed file isn't the owner's byte for byte but Smolcorp's commands:\n got: %q\nwant: %q", got, want)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("the file's mode: %v, %v; want 0640 as it was", info.Mode().Perm(), err)
	}
	backup := file + ".smolcorp-20261115-071500.bak"
	if kept, err := os.ReadFile(backup); err != nil || string(kept) != before {
		t.Errorf("the backup %s isn't the file as it was: %v", backup, err)
	}
	if left, _ := filepath.Glob(file + ".*.tmp"); len(left) > 0 {
		t.Errorf("files left beside it: %v", left)
	}
	// Brought up to date: the next start changes nothing and keeps no second copy.
	if err := refreshAccountSettings(file, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(file); string(again) != want || len(backupsOf(t, file)) != 1 {
		t.Errorf("a second start changed it again: %d backups", len(backupsOf(t, file)))
	}
}

// Nothing of Smolcorp's in it, or only what's up to date: the file isn't written, and no backup is made.
func TestAnAccountWithNothingToRefreshIsLeftAlone(t *testing.T) {
	file, swift := accountUnderTheMacApp(t, ".claude-work")
	ours, _ := encodeSettingsValue(hookCommand(Claude))
	for name, text := range map[string]string{
		"the owner's alone": fill(ownersSettings, map[string]string{
			"SWIFT": `"echo one"`, "OLD": `"echo two"`, "STATUS": swiftEscaped(t, swift), "MINE": swiftEscaped(t, swift+" --mine"), "CRLF": "\n",
		}),
		"already this copy's": fill(ownersSettings, map[string]string{
			"SWIFT": string(ours), "OLD": string(ours), "STATUS": `"x"`, "MINE": `"y"`, "CRLF": "\n",
		}),
	} {
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour).Truncate(time.Second)
		_ = os.Chtimes(file, old, old)
		if err := refreshAccountSettings(file, time.Now()); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		info, _ := os.Stat(file)
		if got, _ := os.ReadFile(file); string(got) != text || !info.ModTime().Equal(old) || len(backupsOf(t, file)) != 0 {
			t.Errorf("%s: the file was written (%v) or backed up (%d)", name, info.ModTime(), len(backupsOf(t, file)))
		}
	}
	// No settings.json at all: nothing is made.
	_ = os.Remove(file)
	if err := refreshAccountSettings(file, time.Now()); err != nil {
		t.Error(err)
	}
	if _, err := os.Lstat(file); !errors.Is(err, os.ErrNotExist) {
		t.Error("a settings.json was made where there was none")
	}
}

// An entry of Smolcorp's naming another office's port: that office's to refresh, so the file stays as it is.
func TestAnAccountWhoseEntriesNameAnotherOfficeIsLeftAlone(t *testing.T) {
	file, swift := accountUnderTheMacApp(t, ".claude-work")
	text := fill(ownersSettings, map[string]string{
		"SWIFT": swiftEscaped(t, swift), "OLD": swiftEscaped(t, anotherOfficesHook(t, 7799)), "STATUS": `"x"`, "MINE": `"y"`, "CRLF": "\n",
	})
	writeFile(t, file, text)
	if err := refreshAccountSettings(file, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != text || len(backupsOf(t, file)) != 0 {
		t.Error("another office's account was changed")
	}
}

// SIERRA: a file that can't be read or isn't valid JSON is left untouched, and setup says so on that account's line.
func TestAnAccountsUnreadableSettingsAreLeftUntouchedAndSaidInSetup(t *testing.T) {
	d, swift := accountMacDeck(t)
	entry := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":` + swiftEscaped(t, swift) + `}]}]}}`
	cases := map[string]string{
		"invalid":        `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":` + swiftEscaped(t, swift) + `}]}]}`,
		"trailing":       entry + ` {"more": 1}`,
		"array":          `[` + entry + `]`,
		"null":           `null`,
		"commented":      "// mine\n" + entry,
		"link-to-nobody": "",
	}
	if runtime.GOOS != "windows" {
		cases["unreadable"] = entry
	}
	var profiles []claudeProfile
	for name, text := range cases {
		dir := filepath.Join(homeDir, ".claude-"+name)
		file := filepath.Join(dir, "settings.json")
		claudeFolder(t, dir, ".claude.json")
		if name == "link-to-nobody" {
			if err := os.Symlink(filepath.Join(dir, "gone.json"), file); err != nil {
				t.Skip("no links here:", err)
			}
		} else {
			writeFile(t, file, text)
		}
		if name == "unreadable" {
			_ = os.Chmod(file, 0)
			t.Cleanup(func() { _ = os.Chmod(file, 0o600) })
			if _, err := os.ReadFile(file); err == nil {
				t.Skip("this user reads any file (root)")
			}
		}
		profiles = append(profiles, claudeProfile{Name: name, Dir: dir})
	}
	good := claudeFolder(t, filepath.Join(homeDir, ".claude-good"), ".claude.json")
	writeFile(t, filepath.Join(good, "settings.json"), entry)
	profiles = append(profiles, claudeProfile{Name: "good", Dir: good})
	d.mu.Lock()
	d.config.ClaudeProfiles = profiles
	d.mu.Unlock()

	d.refreshAccountHooks()
	for name, text := range cases {
		file := filepath.Join(homeDir, ".claude-"+name, "settings.json")
		_ = os.Chmod(file, 0o600)
		if got, _ := os.ReadFile(file); string(got) != text || len(backupsOf(t, file)) != 0 {
			t.Errorf("%s: the file was touched", name)
		}
		_ = os.Chmod(file, 0)
	}
	if commands(readJSON(t, filepath.Join(good, "settings.json")), "Stop")[0] != hookCommand(Claude) {
		t.Error("the good account wasn't refreshed beside the bad ones")
	}

	_, _, answer := claudeAccountsCall(t, d, "GET", "", false)
	for _, p := range listOf(t, answer, "profiles") {
		name, _ := p["name"].(string)
		problem, said := p["settingsProblem"].(string)
		_, bad := cases[name]
		if bad != said || (said && !strings.Contains(problem, "settings.json can't be read or isn't valid JSON")) {
			t.Errorf("%s's line says %q (%v), bad %v", name, problem, said, bad)
		}
	}
}

// The start refreshes each confirmed account's settings.json: not ~/.claude's (refreshHooks has it), not a folder
// that only looks like an account, not one in Smolcorp's own folder, and none at all in a test copy.
func TestTheStartRefreshesEveryConfirmedAccountAndNoOther(t *testing.T) {
	d, _ := accountMacDeck(t)
	// Smolcorp's own folder in the home folder, as it is on a Mac: an account confirmed in it is never refreshed. The
	// deck stops first, and still answers, so its own rounds never read supportDir as it changes (FXPUMPCLASS).
	stopDeck(d)
	saved := supportDir
	supportDir = filepath.Join(homeDir, "Library", "Smolcorp")
	t.Cleanup(func() { supportDir = saved })
	swift := `"` + filepath.Join(supportDir, "report.sh") + `"`
	entry := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":` + swiftEscaped(t, swift) + `}]}]}}`
	folders := map[string]string{}
	for _, name := range []string{"default", "work", "home", "found", "support"} {
		dir := filepath.Join(homeDir, ".claude-"+name)
		switch name {
		case "default":
			dir = filepath.Join(homeDir, ".claude")
		case "support":
			dir = filepath.Join(supportDir, "hires", "h1", "claude")
		}
		claudeFolder(t, dir)
		writeFile(t, filepath.Join(dir, "settings.json"), entry)
		folders[name] = dir
	}
	d.mu.Lock()
	d.config.ClaudeProfiles = []claudeProfile{{Name: "work", Dir: folders["work"]}, {Name: "home", Dir: folders["home"]}, {Name: "support", Dir: folders["support"]}}
	d.mu.Unlock()
	refreshed := func(name string) bool {
		got, _ := os.ReadFile(filepath.Join(folders[name], "settings.json"))
		return string(got) != entry
	}

	onPort(t, portFromEnvironment("")+1) // a test copy
	d.refreshAccountHooks()
	for name := range folders {
		if refreshed(name) {
			t.Errorf("a test copy refreshed %s", name)
		}
	}
	onDefaultPort(t)
	d.refreshAccountHooks()
	for name, want := range map[string]bool{"default": false, "work": true, "home": true, "found": false, "support": false} {
		if refreshed(name) != want {
			t.Errorf("%s refreshed: %v, want %v", name, refreshed(name), want)
		}
	}
	got := commands(readJSON(t, filepath.Join(folders["work"], "settings.json")), "Stop")
	if !slices.Equal(got, []string{hookCommand(Claude)}) {
		t.Errorf("work's Stop runs %q", got)
	}
}

// A settings.json that is a link (a dotfiles folder): the file it leads to changes, the link stays, the backup is
// beside that file.
func TestAnAccountsLinkedSettingsChangeWhereTheyLead(t *testing.T) {
	file, swift := accountUnderTheMacApp(t, ".claude-work")
	target := filepath.Join(homeDir, "dotfiles", "claude-work.json")
	entry := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":` + swiftEscaped(t, swift) + `}]}]}}`
	writeFile(t, target, entry)
	if err := os.Symlink(target, file); err != nil {
		t.Skip("no links here:", err)
	}
	if err := refreshAccountSettings(file, time.Now()); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(file); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced by a file")
	}
	if commands(readJSON(t, target), "Stop")[0] != hookCommand(Claude) {
		t.Error("the file the link leads to wasn't refreshed")
	}
	if len(backupsOf(t, target)) != 1 || len(backupsOf(t, file)) != 0 {
		t.Error("the backup isn't beside the file the link leads to")
	}
}

// A file the owner changed while it was being refreshed is left as they made it.
func TestAnAccountsSettingsChangedMeanwhileAreLeftAsTheOwnerMadeThem(t *testing.T) {
	file, _ := accountUnderTheMacApp(t, ".claude-work")
	writeFile(t, file, `{"model":"theirs"}`)
	err := replaceIfUnchanged(file, []byte(`{"model":"before"}`), []byte(`{"model":"ours"}`), 0o600)
	if !errors.Is(err, errAccountSettingsChanged) {
		t.Errorf("replaced over the owner's change: %v", err)
	}
	if got, _ := os.ReadFile(file); string(got) != `{"model":"theirs"}` {
		t.Errorf("the file holds %s", got)
	}
	if left, _ := filepath.Glob(file + ".*.tmp"); len(left) > 0 {
		t.Errorf("files left beside it: %v", left)
	}
}

// A backup is never written over: a second one in the same second gets a name of its own.
func TestAnAccountsBackupNeverReplacesAnother(t *testing.T) {
	file, _ := accountUnderTheMacApp(t, ".claude-work")
	now := time.Date(2026, 11, 15, 7, 15, 0, 0, time.Local)
	for _, text := range []string{"one", "two"} {
		if err := keepTimedBackup(file, []byte(text), 0o600, now); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := os.ReadFile(file + ".smolcorp-20261115-071500.bak")
	second, _ := os.ReadFile(file + ".smolcorp-20261115-071500-2.bak")
	if string(first) != "one" || string(second) != "two" {
		t.Errorf("backups: %q, %q", first, second)
	}
}
