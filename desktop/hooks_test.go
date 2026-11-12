package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Where an installed copy lives, as its hooks name it.
const installedCommand = "/home/someone/.local/bin/smolcorp hook"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s isn't valid JSON: %v", path, err)
	}
	return value
}

// commands are the hook commands an event runs, in a settings file's layout.
func commands(settings map[string]any, event string) []string {
	var found []string
	hooks, _ := settings["hooks"].(map[string]any)
	groups, _ := hooks[event].([]any)
	for _, group := range groups {
		handlers, _ := group.(map[string]any)["hooks"].([]any)
		for _, handler := range handlers {
			command, _ := handler.(map[string]any)["command"].(string)
			found = append(found, command)
		}
	}
	return found
}

func TestConnectingAddsTheDecksHooksAndKeepsTheRest(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	original := `{
  "model": "opus",
  "cleanupPeriodDays": 30.50,
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "say done"}]},
      {"hooks": [{"type": "command", "command": "` + installedCommand + `"}]}
    ]
  }
}`
	writeFile(t, settings, original)

	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	saved := readJSON(t, settings)
	for _, event := range entriesFor(Claude).events {
		ours := 0
		for _, command := range commands(saved, event) {
			if command == hookCommand(Claude) {
				ours++
			}
		}
		if ours != 1 {
			t.Errorf("%s runs the deck's hook %d times, want once", event, ours)
		}
	}
	stop := commands(saved, "Stop")
	if len(stop) != 2 || stop[0] != "say done" {
		t.Errorf("Stop runs %q, want your own hook kept and the old copy's replaced", stop)
	}
	if saved["model"] != "opus" {
		t.Errorf("model is %v, want it left as it was", saved["model"])
	}
	if text, _ := os.ReadFile(settings); !strings.Contains(string(text), "30.50") {
		t.Error("a number in the settings came back written differently")
	}
	if voice, _ := saved["voice"].(map[string]any); voice["mode"] != "tap" || voice["enabled"] != true {
		t.Errorf("voice is %v, want it on in tap mode, as the mic needs", saved["voice"])
	}
	if backup, _ := os.ReadFile(settings + ".agentdeck.bak"); string(backup) != original {
		t.Error("the backup isn't the settings as they were")
	}
}

func TestConnectingLeavesVoiceSetUpYourWay(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{"voice": {"enabled": false}}`)
	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	if voice, _ := readJSON(t, settings)["voice"].(map[string]any); voice["enabled"] != false || voice["mode"] != nil {
		t.Errorf("voice is %v, want it as you set it", voice)
	}
}

func TestDisconnectingRemovesOnlyTheDecksHooks(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".codex", "hooks.json")
	writeFile(t, settings, `{"hooks": {
  "Stop": [{"hooks": [{"type": "command", "command": "notify-send done"}]}, {"hooks": [{"type": "command", "command": "`+installedCommand+` codex"}]}],
  "SessionStart": [{"hooks": [{"type": "command", "command": "`+installedCommand+` codex"}]}]
}}`)
	if err := disconnectAgents(); err != nil {
		t.Fatal(err)
	}
	saved := readJSON(t, settings)
	if stop := commands(saved, "Stop"); len(stop) != 1 || stop[0] != "notify-send done" {
		t.Errorf("Stop runs %q, want only your own hook", stop)
	}
	if hooks := saved["hooks"].(map[string]any); hooks["SessionStart"] != nil {
		t.Error("an event left with nothing to run is still listed")
	}
}

func TestGrokGetsAHooksFileOfItsOwn(t *testing.T) {
	home := isolate(t)
	if err := connect(Grok, true); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, ".grok", "hooks", "smolcorp.json")
	if commands(readJSON(t, file), "Stop")[0] != hookCommand(Grok) {
		t.Fatal("Grok's hooks file doesn't run the deck's hook")
	}
	if err := disconnectAgents(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("disconnecting left Grok's hooks file behind")
	}
}

func TestOpenCodesPluginRunsTheWholeCommand(t *testing.T) {
	isolate(t)
	if err := writePlugin(); err != nil {
		t.Fatal(err)
	}
	plugin, err := os.ReadFile(openCodePlugin())
	if err != nil {
		t.Fatal(err)
	}
	command, _ := json.Marshal([]string{commandLineProgram(), "hook", "--port", fmt.Sprint(Port), "opencode"})
	if !strings.Contains(string(plugin), "const COMMAND = "+string(command)) || strings.Contains(string(plugin), "{{COMMAND}}") {
		t.Fatalf("the plugin doesn't run %s", command)
	}
}

func TestSettingsThatArentJSONAreLeftAlone(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".gemini", "settings.json")
	broken := `{"theme": "dark",`
	writeFile(t, settings, broken)
	_, err := connectAgents(true)
	if err == nil || !strings.Contains(err.Error(), "isn't valid JSON") {
		t.Fatalf("connecting gave %v, want it to name the broken file", err)
	}
	if text, _ := os.ReadFile(settings); string(text) != broken {
		t.Fatal("broken settings were rewritten")
	}
}

func TestSettingsThatCantBeReadAreLeftAlone(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file its owner can't read")
	}
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	original := `{"model": "opus", "permissions": {"allow": ["Bash(ls)"]}}`
	writeFile(t, settings, original)
	if err := os.Chmod(settings, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(settings, 0o644) })

	if _, err := connectAgents(true); err == nil || !strings.Contains(err.Error(), "can't be read") {
		t.Fatalf("connecting gave %v, want it to name the unreadable file", err)
	}
	os.Chmod(settings, 0o644)
	if text, _ := os.ReadFile(settings); string(text) != original {
		t.Fatalf("settings that couldn't be read were written over: %s", text)
	}
}

func TestALinkToNothingIsLeftAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need privileges on Windows")
	}
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "dotfiles", "gone.json"), settings); err != nil {
		t.Fatal(err)
	}
	if _, err := connectAgents(true); err == nil {
		t.Fatal("connected through a link to nothing")
	}
	if _, err := os.Readlink(settings); err != nil {
		t.Fatalf("the link was replaced: %v", err)
	}
}

func TestSettingsKeptInDotfilesStayLinked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links need privileges on Windows")
	}
	home := isolate(t)
	kept := filepath.Join(home, "dotfiles", "claude-settings.json")
	writeFile(t, kept, `{"model": "opus"}`)
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(kept, settings); err != nil {
		t.Fatal(err)
	}

	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(settings); err != nil || target != kept {
		t.Fatalf("settings.json is no longer a link to the dotfiles copy (%q, %v)", target, err)
	}
	saved := readJSON(t, kept)
	if saved["model"] != "opus" || len(commands(saved, "Stop")) != 1 {
		t.Fatalf("the dotfiles copy didn't get the hooks and keep the rest: %v", saved)
	}
}

func TestAStartWithNothingToChangeLeavesTheFileAndItsBackup(t *testing.T) {
	home := isolate(t)
	// An installed copy, so the deck knows its own entries when it starts again.
	saved := executable
	t.Cleanup(func() { executable = saved })
	executable = func() (string, error) { return filepath.Join(home, "bin", "smolcorp"), nil }
	settings := filepath.Join(home, ".claude", "settings.json")
	original := `{"model": "opus"}`
	writeFile(t, settings, original)
	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	connected, _ := os.ReadFile(settings)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(settings, old, old)

	refreshHooks()

	if backup, _ := os.ReadFile(settings + ".agentdeck.bak"); string(backup) != original {
		t.Fatalf("the backup no longer holds the file from before connecting: %s", backup)
	}
	if text, _ := os.ReadFile(settings); string(text) != string(connected) {
		t.Fatal("the settings changed on a start with nothing to change")
	}
	if info, _ := os.Stat(settings); !info.ModTime().Equal(old) {
		t.Fatal("the settings were written on a start with nothing to change")
	}
}

func TestTheFirstBackupKeepsYourSettingsFromBeforeSmolcorp(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".codex", "hooks.json")
	original := `{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]}}`
	writeFile(t, settings, original)
	if err := connect(Codex, false); err != nil {
		t.Fatal(err)
	}
	// You change the file, and the deck changes it again: its entries are put back.
	edited := `{"theme": "dark", "hooks": {}}`
	writeFile(t, settings, edited)
	if err := connect(Codex, false); err != nil {
		t.Fatal(err)
	}
	if err := disconnectAgents(); err != nil {
		t.Fatal(err)
	}
	if first, _ := os.ReadFile(settings + ".agentdeck.bak"); string(first) != original {
		t.Errorf("after three changes the first backup is %s, want the file from before Smolcorp", first)
	}
	last, _ := os.ReadFile(settings + ".agentdeck-last.bak")
	if hooks := readJSON(t, settings+".agentdeck-last.bak"); hooks["theme"] != "dark" || len(commands(hooks, "Stop")) != 1 {
		t.Errorf("the last backup is %s, want the file from before the last change: yours, with the deck's entries", last)
	}
}

func TestNothingIsWrittenWithoutItsBackups(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{"model": "opus"}`)
	// A backup that can't be made: a folder sits where it goes.
	for _, backup := range []string{".agentdeck.bak", ".agentdeck-last.bak"} {
		writeFile(t, settings+backup+string(filepath.Separator)+"inside", "")
	}
	if err := connect(Claude, false); err == nil {
		t.Error("connecting went ahead without its last backup")
	}
	if text, _ := os.ReadFile(settings); string(text) != `{"model": "opus"}` {
		t.Errorf("the settings are %s, want them as they were", text)
	}
}

func TestOnlyAgentsWithASettingsFolderAreFound(t *testing.T) {
	home := isolate(t)
	if _, err := connectAgents(true); err != errNoAgents {
		t.Fatalf("with no agents installed, connecting gave %v", err)
	}
	for _, folder := range []string{".claude", ".codex"} {
		if err := os.Mkdir(filepath.Join(home, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if found := foundAgents(); len(found) != 2 || found[0] != Claude || found[1] != Codex {
		t.Fatalf("found %v, want Claude Code and Codex", found)
	}
}

func TestTheDecksHooksAreRecognisedWhereverTheyPoint(t *testing.T) {
	for command, ours := range map[string]bool{
		installedCommand:             true,
		installedCommand + " gemini": true,
		// Written by the app under the name it had before, so an update replaces them
		"/home/someone/.local/bin/sauzzedeck hook": true,
		// On Windows, in the short form of C:\Program Files\Smolcorp\smolcorp-cli.exe, and of the old name
		"C:/PROGRA~1/SMOLCO~1/SMOLCO~1.EXE hook codex":       true,
		"C:/PROGRA~1/SAUZZE~1/SAUZZE~1.EXE hook codex":       true,
		"C:/Program Files/Smolcorp/smolcorp-cli.exe hook":    true,
		`"/home/some one/.local/bin/smolcorp" hook`:          true,
		`"C:\\Users\\me\\SAUZZE~1.EXE" hook`:                 true,
		`"/opt/SauzzeDeck Pro/SauzzeDeck Pro.exe" hook grok`: true,
		"say done":                       false,
		"/usr/local/bin/other-tool hook": false,
		"/home/someone/.local/bin/smolcorp run claude": false,
		// Hooks of your own that mention Smolcorp, SIERRA's among them
		`notify-send "smolcorp hook done"`:                    false,
		"~/code/smolcorp/scripts/notify.sh hook":              false,
		"/home/sauzze/bin/log hook-start":                     false,
		"~/bin/smolcorp-log.sh hook-start":                    false,
		"/home/someone/bin/smolcorp-notify hook":              false,
		installedCommand + " done":                            false,
		installedCommand + " codex --verbose":                 false,
		installedCommand + "; rm -rf ~/notes":                 false,
		`"/home/someone/.local/bin/smolcorp hook"`:            false,
		`"/home/someone/.local/bin/smolcorp"hook`:             false,
		"/home/someone/.local/bin/smolcorp  hook":             false,
		"/home/someone/.local/bin/smolcorp hook claude":       true,
		"cd /tmp && /home/someone/.local/bin/smolcorp hook":   false,
		"/usr/bin/env /home/someone/.local/bin/smolcorp hook": false,
		"smolcorp hook": false,
		// Windows writes a path with no space as it is: any folder's name may hold ' ( ) & !
		"C:/Users/O'Brien/AppData/Local/Smolcorp/smolcorp-cli.exe hook": true,
		// Unquoted with a &, only this copy's own path counts: see TestThisCopysUnquotedPathWithAnAmpersand
		"C:/Users/A&B (Work)!/AppData/Local/Smolcorp/smolcorp-cli.exe hook codex": false,
		`C:/tools/notify.exe&..\Smolcorp\smolcorp-cli.exe hook`:                   false,
		`C:/tools/notify.exe&.\smolcorp-cli.exe hook`:                             false,
		"C:/tools/notify.exe&Smolcorp/smolcorp-cli.exe hook":                      false,
		"C:/tools/notify.exe&~/smolcorp-cli.exe hook":                             false,
		"C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook done":           false,
		// A hook of yours chained to the deck's command, unquoted on Windows: yours, kept
		"C:/tools/notify.exe & C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook":          false,
		"C:/tools/log.exe x.txt; C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook claude": false,
		"C:/tools/a.exe | C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook":               false,
		"C:/tools/a.exe&&C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook":                false,
		"C:/tools/a.exe C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook":                 false,
		"C:/tools/a.exe > C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook":               false,
		"C:/tools/notify.exe&C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe hook":            false,
		"C:/tools/notify.exe&%LOCALAPPDATA%/Smolcorp/smolcorp-cli.exe hook":                       false,
		"C:/tools/notify.exe&/Smolcorp/smolcorp-cli.exe hook":                                     false,
		`C:\tools\notify.exe&\Smolcorp\smolcorp-cli.exe hook`:                                     false,
		`"C:/Users/me/$env:X/smolcorp-cli.exe" hook`:                                              false,
		`"C:/Users/O'Brien & Co/AppData/Local/Smolcorp/smolcorp-cli.exe" hook`:                    true,
		`"C:/Program Files (x86)/Smolcorp/smolcorp-cli.exe" hook gemini`:                          true,
		`"/home/someone/$(whoami)/smolcorp" hook`:                                                 false,
	} {
		if got := isOurs(map[string]any{"type": "command", "command": command}); got != ours {
			t.Errorf("isOurs(%q) = %v, want %v", command, got, ours)
		}
	}
}

func TestAHookOfYourOwnThatMentionsSmolcorpIsKept(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{"hooks": {"Stop": [
  {"hooks": [{"type": "command", "command": "notify-send \"smolcorp hook done\""}]},
  {"hooks": [{"type": "command", "command": "~/code/smolcorp/scripts/notify.sh hook"}]}
]}}`)
	if err := connect(Claude, false); err != nil {
		t.Fatal(err)
	}
	if err := disconnectAgents(); err != nil {
		t.Fatal(err)
	}
	if stop := commands(readJSON(t, settings), "Stop"); len(stop) != 2 {
		t.Errorf("after connecting and disconnecting, Stop runs %q, want both your hooks", stop)
	}
}

// A copy under a name of its own, as one built by hand, still knows its hooks.
func TestThisCopysOwnHooksAreReplaced(t *testing.T) {
	home := isolate(t)
	// Not one of the deck's names, so only being this copy makes them its own.
	saved := executable
	t.Cleanup(func() { executable = saved })
	program := filepath.Join(home, "tools", "mydeck")
	writeFile(t, program, "")
	executable = func() (string, error) { return program, nil }
	if !isOurs(map[string]any{"type": "command", "command": hookCommand(Codex)}) {
		t.Fatalf("this copy's own hook, %s, isn't recognised", hookCommand(Codex))
	}
	if err := connect(Codex, false); err != nil {
		t.Fatal(err)
	}
	if err := connect(Codex, false); err != nil {
		t.Fatal(err)
	}
	if stop := commands(readJSON(t, entriesFor(Codex).file), "Stop"); len(stop) != 1 || stop[0] != hookCommand(Codex) {
		t.Errorf("connected twice, Stop runs %q, want this copy's hook once", stop)
	}
}

func TestAnUpdateMovesGrokAndOpenCodeToTheNewName(t *testing.T) {
	home := isolate(t)
	writeFile(t, filepath.Join(home, ".grok", "hooks", "sauzzedeck-pro.json"), `{"hooks": {}}`)
	writeFile(t, filepath.Join(home, ".config", "opencode", "plugins", "sauzzedeck-pro.js"), "// the old plugin")
	if !isConnected(OpenCode) {
		t.Fatal("OpenCode's plugin under the old name doesn't count as connected")
	}
	refreshHooks()
	for _, old := range []string{oldGrokHooks(), oldOpenCodePlugin()} {
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Errorf("%s is still there after the update", old)
		}
	}
	// Under the test program's own name, Grok's file is there to see rather than its hooks to recognise
	if _, err := os.Stat(filepath.Join(home, ".grok", "hooks", "smolcorp.json")); err != nil || !isConnected(OpenCode) {
		t.Errorf("after the update: Grok's new file %v, OpenCode connected %v; want both", err, isConnected(OpenCode))
	}
	// Disconnecting takes out both names
	writeFile(t, oldGrokHooks(), `{"hooks": {}}`)
	if err := disconnectAgents(); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{oldGrokHooks(), filepath.Join(home, ".grok", "hooks", "smolcorp.json"), openCodePlugin()} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("disconnecting left %s behind", file)
		}
	}
}

func TestAWindowsCopyBuiltByHandRunsItsOwnHooks(t *testing.T) {
	folder := t.TempDir()
	app := filepath.Join(folder, "smolcorp.exe")
	writeFile(t, app, "")
	if got := consoleBuildBeside(app); got != app {
		t.Errorf("with no smolcorp-cli.exe beside it, the hooks run %s, want the app itself", got)
	}
	console := filepath.Join(folder, "smolcorp-cli.exe")
	writeFile(t, console, "")
	if got := consoleBuildBeside(app); got != console {
		t.Errorf("with smolcorp-cli.exe beside it, the hooks run %s, want it", got)
	}
}

func TestAWindowsHooksPathIsQuotedWhereCmdAndGitBashNeedIt(t *testing.T) {
	for path, quoted := range map[string]bool{
		"C:/Users/me/AppData/Local/Smolcorp/smolcorp-cli.exe":       false,
		"C:/PROGRA~1/SMOLCO~1/SMOLCO~1.EXE":                         false,
		"C:/Program Files/Smolcorp/smolcorp-cli.exe":                true,
		"C:/Users/A&B/AppData/Local/Smolcorp/smolcorp-cli.exe":      true,
		"C:/Users/A&B(WO~1/AppData/Local/Smolcorp/smolcorp-cli.exe": true,
		"C:/Users/O'Brien/AppData/Local/Smolcorp/smolcorp-cli.exe":  true,
		"C:/Users/Up^Down/AppData/Local/Smolcorp/smolcorp-cli.exe":  true,
		"C:/Users/Yes!/AppData/Local/Smolcorp/smolcorp-cli.exe":     false,
	} {
		if got := windowsPathNeedsQuotes(path); got != quoted {
			t.Errorf("%s quoted: %v, want %v", path, got, quoted)
		}
		// Quoted or not, the deck knows it as its own.
		command := path + " hook"
		if quoted {
			command = `"` + path + `" hook`
		}
		if !isOurs(map[string]any{"type": "command", "command": command}) {
			t.Errorf("%s isn't recognised as the deck's", command)
		}
	}
}

// An earlier version wrote this copy's path unquoted on Windows, & and all:
// that, and only that, is the deck's.
func TestThisCopysUnquotedPathWithAnAmpersand(t *testing.T) {
	saved := executable
	t.Cleanup(func() { executable = saved })
	here := "C:/Users/A&B (Work)!/AppData/Local/Smolcorp/smolcorp-cli.exe"
	executable = func() (string, error) { return here, nil }
	if runtime.GOOS == "windows" {
		t.Skip("on Windows, this copy's path is its real one, in its short form")
	}
	for command, ours := range map[string]bool{
		here + " hook":       true,
		here + " hook codex": true,
		"C:/tools/notify.exe&C:/Users/A&B (Work)!/AppData/Local/Smolcorp/smolcorp-cli.exe hook": false,
		"C:/Users/Other&Co/AppData/Local/Smolcorp/smolcorp-cli.exe hook":                        false,
	} {
		if got := isOurs(map[string]any{"type": "command", "command": command}); got != ours {
			t.Errorf("isOurs(%q) = %v, want %v", command, got, ours)
		}
	}
}

func ours(command string) bool {
	return isOurs(map[string]any{"type": "command", "command": command})
}

// The paths a Linux copy runs from, installed or portable, as the deck writes them (quoted) and as earlier versions did.
func TestTheDecksHooksAreRecognisedOnLinuxPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("these are Linux paths")
	}
	for command, want := range map[string]bool{
		// Installed
		`"/usr/bin/smolcorp" hook`:                 true,
		`"/usr/local/bin/smolcorp" hook codex`:     true,
		`"/opt/Smolcorp/smolcorp" hook gemini`:     true,
		`"/home/me/.local/bin/smolcorp" hook`:      true,
		`"/home/me/.local/bin/smolcorp" hook grok`: true,
		"/usr/bin/smolcorp hook opencode":          true,
		// Portable, wherever it was unpacked, spaces and all
		`"/home/me/Downloads/Smolcorp 1.23/smolcorp" hook`: true,
		`"/media/me/usb stick/smolcorp-cli" hook claude`:   true,
		"/tmp/portable/Smolcorp hook":                      true,
		// Old name
		`"/usr/bin/sauzzedeck" hook`:                      true,
		"/home/me/bin/sauzzedeck hook claude":             true,
		`"/opt/SauzzeDeck Pro/sauzzedeck-cli" hook codex`: true,
		`"/opt/SauzzeDeck Pro/SauzzeDeck Pro" hook`:       true,
		// Unquoted from before, but with a space, a shell would split it
		"/home/me/Downloads/Smolcorp 1.23/smolcorp hook": false,
		// Not a program of the deck's
		`"/usr/bin/smolcorp-notify" hook`:    false,
		`"/usr/bin/notify-send" hook`:        false,
		`"/home/me/smolcorp/notify" hook`:    false,
		`"/home/me/sauzzedeck/bin/log" hook`: false,
		`"/usr/bin/smolcorp.sh" hook`:        false,
		// Not a path from the root
		`"smolcorp" hook`:                  false,
		`"~/.local/bin/smolcorp" hook`:     false,
		`"./smolcorp" hook`:                false,
		`"$HOME/.local/bin/smolcorp" hook`: false,
		// The shell would run more than the path
		`"/usr/bin/smolcorp` + "`id`" + `" hook`: false,
		`/usr/bin/smolcorp;id hook`:              false,
		"/usr/bin/smolcorp\nid hook":             false,
		`"/usr/bin/smolcorp hook`:                false,
		// Your own hooks that mention Smolcorp
		`notify-send "smolcorp hook done"`:                  false,
		`notify-send 'smolcorp hook'`:                       false,
		`/usr/bin/notify-send "Smolcorp" "hook claude"`:     false,
		`echo smolcorp hook >> "$HOME/smolcorp.log"`:        false,
		`bash -c '"/usr/bin/smolcorp" hook'`:                false,
		`/usr/bin/smolcorp hook claude && notify-send done`: false,
		`"/usr/bin/smolcorp" hook && notify-send done`:      false,
		`sudo /usr/bin/smolcorp hook`:                       false,
	} {
		if got := ours(command); got != want {
			t.Errorf("isOurs(%q) = %v, want %v", command, got, want)
		}
	}
}

func TestOnlyAWholePathFromTheRootIsAProgramsPath(t *testing.T) {
	for _, c := range []struct {
		path   string
		quoted bool
		want   bool
	}{
		{"/usr/bin/smolcorp", false, true},
		{"/usr/bin/smolcorp", true, true},
		{"/home/me/.local/bin/smolcorp", false, true},
		// A space and a shell's own characters need quotes, and only some are safe in them
		{"/home/me/my tools/smolcorp", false, false},
		{"/home/me/my tools/smolcorp", true, true},
		{"/home/me/it's/smolcorp", false, false},
		{"/home/me/it's/smolcorp", true, true},
		{"/home/me/a&b/smolcorp", false, false},
		{"/home/me/a&b/smolcorp", true, true},
		{"/home/me/(x)/smolcorp", true, true},
		{"/home/me/$HOME/smolcorp", false, false},
		{"/home/me/$HOME/smolcorp", true, false},
		{"/home/me/`id`/smolcorp", true, false},
		{"/home/me/a;b/smolcorp", false, false},
		{"/home/me/a|b/smolcorp", false, false},
		{"/home/me/a*/smolcorp", false, false},
		{"/home/me/#/smolcorp", false, false},
		{"/home/me/!/smolcorp", false, false},
		// Never a quote or a line break, whichever way
		{`/home/me/a"b/smolcorp`, true, false},
		{`/home/me/a"b/smolcorp`, false, false},
		{"/home/me/a\nb/smolcorp", true, false},
		// Not from the root
		{"smolcorp", false, false},
		{"smolcorp", true, false},
		{"./smolcorp", true, false},
		{"~/smolcorp", false, false},
		{"", false, false},
		{"", true, false},
	} {
		if got := isAProgramsPath(c.path, c.quoted); got != c.want {
			t.Errorf("isAProgramsPath(%q, quoted %v) = %v, want %v", c.path, c.quoted, got, c.want)
		}
	}
}

func TestTheDecksProgramsAreKnownByTheirNames(t *testing.T) {
	for path, want := range map[string]bool{
		"/usr/bin/smolcorp":                  true,
		"/opt/Smolcorp/Smolcorp":             true,
		"/opt/Smolcorp/smolcorp-cli":         true,
		"/opt/Smolcorp/smolcorp-cli.exe":     true,
		"/usr/bin/sauzzedeck":                true,
		"/usr/bin/sauzzedeck-cli":            true,
		"/opt/SauzzeDeck Pro/SauzzeDeck Pro": true,
		"/PROGRA~1/SMOLCO~1/SMOLCO~1.EXE":    true,
		"C:/PROGRA~1/SAUZZE~1/SAUZZE~12.EXE": true,
		`C:\Smolcorp\smolcorp-cli.exe`:       true,
		"smolcorp":                           true,
		"/usr/bin/smolcorp.sh":               false,
		"/usr/bin/smolcorp-notify":           false,
		"/usr/bin/smolcorp2":                 false,
		"/usr/bin/my-smolcorp":               false,
		"/usr/bin/smolcorp/notify":           false,
		"/usr/bin/other~1":                   false,
		"/usr/bin/smolco~":                   false,
		"/usr/bin/":                          false,
		"":                                   false,
	} {
		if got := isDecksProgram(path); got != want {
			t.Errorf("isDecksProgram(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestConnectingSaysWhenItTurnsVoiceOn(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{}`)
	notice, err := connectAgents(true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notice, "voice") {
		t.Errorf("notice is %q, want it to say voice was turned on", notice)
	}
	writeFile(t, settings, `{"voice": {"enabled": false}}`)
	if notice, _ := connectAgents(true); notice != "" {
		t.Errorf("notice is %q, want none when voice was already set", notice)
	}
}

func TestDisconnectingTurnsVoiceOffWhenTheDeckTurnedItOn(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{"model": "opus"}`)
	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	if err := disconnectAgents(); err != nil {
		t.Fatal(err)
	}
	saved := readJSON(t, settings)
	if _, set := saved["voice"]; set || saved["model"] != "opus" {
		t.Errorf("settings are %v, want voice gone and the rest kept", saved)
	}
}

func TestDisconnectingLeavesYourOwnVoiceSetting(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	for name, original := range map[string]string{
		"set before connecting": `{"voice": {"enabled": true, "mode": "tap"}}`,
		"changed after":         "",
	} {
		writeFile(t, settings, `{}`)
		os.Remove(voiceMarker())
		if original != "" {
			writeFile(t, settings, original)
		}
		if err := connect(Claude, true); err != nil {
			t.Fatal(err)
		}
		if original == "" {
			saved := readJSON(t, settings)
			saved["voice"] = map[string]any{"enabled": true, "mode": "hold"}
			data, _ := json.Marshal(saved)
			writeFile(t, settings, string(data))
		}
		if err := disconnectAgents(); err != nil {
			t.Fatal(err)
		}
		if voice, _ := readJSON(t, settings)["voice"].(map[string]any); voice == nil || voice["enabled"] != true {
			t.Errorf("%s: voice is %v, want it left as it was", name, voice)
		}
	}
}

func TestAPathIsQuotedForTheShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on Windows, a path is quoted only where cmd and Git Bash need it")
	}
	for path, want := range map[string]string{
		"/usr/bin/smolcorp":                         `'/usr/bin/smolcorp'`,
		"/home/me/my tools/smolcorp":                `'/home/me/my tools/smolcorp'`,
		"/home/me/Downloads/Smolcorp 1.23/smolcorp": `'/home/me/Downloads/Smolcorp 1.23/smolcorp'`,
		"/home/me/O'Brien/smolcorp":                 `'/home/me/O'\''Brien/smolcorp'`,
	} {
		if got := shellPath(path); got != want {
			t.Errorf("shellPath(%q) = %s, want %s", path, got, want)
		}
	}
}

// What the deck writes for an installed or a portable Linux copy, in a folder
// of any name, is what it takes out again, and nothing of yours with it.
func TestALinuxCopysHooksAreReplacedAndYoursAreKept(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("these are Linux paths")
	}
	saved := executable
	t.Cleanup(func() { executable = saved })
	for _, path := range []string{"/usr/bin/smolcorp", "/home/me/Downloads/Smolcorp 1.23/smolcorp", "/opt/odd & ends/smolcorp"} {
		executable = func() (string, error) { return path, nil }
		for _, agent := range []Agent{Claude, Codex, Gemini, Grok} {
			command := hookCommand(agent)
			if !strings.HasPrefix(command, `'`+path+`' hook`) {
				t.Errorf("%s runs %s, want the quoted path then hook", agent, command)
			}
			if !ours(command) {
				t.Errorf("%s's own hook, %s, isn't recognised", agent, command)
			}
		}
	}
	yours := []string{
		`notify-send "smolcorp hook done"`,
		`notify-send 'smolcorp hook'`,
		"~/code/smolcorp/scripts/notify.sh hook",
		`"/usr/local/bin/smolcorp-notify" hook`,
	}
	var handlers []any
	for _, command := range append(yours, `"/usr/bin/smolcorp" hook`, "/home/me/bin/sauzzedeck hook claude") {
		handlers = append(handlers, map[string]any{"type": "command", "command": command})
	}
	kept := withoutOurs(map[string]any{"Stop": []any{map[string]any{"matcher": "x", "hooks": handlers}}})
	groups, _ := kept["Stop"].([]any)
	if len(groups) != 1 {
		t.Fatalf("Stop keeps %d groups, want 1", len(groups))
	}
	if got := commands(map[string]any{"hooks": kept}, "Stop"); !equalStrings(got, yours) {
		t.Errorf("Stop keeps %q, want only %q", got, yours)
	}
	if groups[0].(map[string]any)["matcher"] != "x" {
		t.Errorf("the group lost its matcher: %v", groups[0])
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A path with a shell's own characters must come back from a real shell as it
// went in, and the deck must know the command as its own, as well as the
// double-quoted form earlier versions wrote.
func TestShellPathSurvivesARealShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows paths are quoted for cmd and PowerShell, not sh")
	}
	saved := executable
	t.Cleanup(func() { executable = saved })
	home := t.TempDir()
	for _, path := range []string{
		"/home/someone/smolcorp",
		"/home/some one/smolcorp",
		`/home/say "hi"/smolcorp`,
		"/home/$HOME/$(whoami)/smolcorp",
		"/home/back`tick`/smolcorp",
		`/home/back\slash/smolcorp`,
		"/home/O'Brien/smolcorp",
		"/home/'; touch pwned; '/smolcorp",
	} {
		cmd := exec.Command("/bin/sh", "-c", "printf %s "+shellPath(path))
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
		out, err := cmd.Output()
		if err != nil || string(out) != path {
			t.Errorf("sh printed %q (%v) for %q, quoted as %s", out, err, path, shellPath(path))
		}
		executable = func() (string, error) { return path, nil }
		for _, command := range []string{
			shellPath(path) + " hook",
			shellPath(path) + " hook codex",
			`"` + path + `" hook`, // earlier versions
		} {
			// The old form only reads back while nothing in it is a shell's own.
			old := strings.HasPrefix(command, `"`)
			want := !old || !strings.ContainsAny(path, "\"$`")
			if got := isOurs(map[string]any{"type": "command", "command": command}); got != want {
				t.Errorf("isOurs(%q) = %v, want %v", command, got, want)
			}
		}
	}
	if _, err := os.Stat("pwned"); err == nil {
		t.Error("a path ran as a command")
	}
}

func TestIsOursRefusesLooseSingleQuotes(t *testing.T) {
	for _, command := range []string{
		`'/home/me/smolcorp hook`,
		`'/home/me/smolcorp'x hook`,
		`'/home/me/smolcorp' '/bin/rm' hook`,
		`'/home/me/'smolcorp hook`,
		`'/home/me/notify' hook`,
		`'smolcorp' hook`,
	} {
		if isOurs(map[string]any{"type": "command", "command": command}) {
			t.Errorf("isOurs(%q) = true, want false", command)
		}
	}
}

func TestAStaleMarkerNeverLetsRemovalDeleteYourOwnVoiceSetting(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{}`)
	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	// The hooks are edited out by hand; the voice entry stays, and the marker with it
	saved := readJSON(t, settings)
	delete(saved, "hooks")
	data, _ := json.Marshal(saved)
	writeFile(t, settings, string(data))
	// Later you set the very same value yourself, and connect again
	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	if err := disconnectAgents(); err != nil {
		t.Fatal(err)
	}
	if voice, _ := readJSON(t, settings)["voice"].(map[string]any); voice == nil || voice["mode"] != "tap" {
		t.Errorf("voice is %v, want your own setting kept", voice)
	}
}

func TestTheVoiceMarkerStaysWhenRemovalCantWriteTheSettings(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder its owner can't write to")
	}
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{}`)
	if err := connect(Claude, true); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Dir(settings)
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(folder, 0o755) })
	if err := disconnectAgents(); err == nil {
		t.Skip("the settings could be written anyway")
	}
	if _, err := os.Stat(voiceMarker()); err != nil {
		t.Errorf("the marker went although the settings weren't written: %v", err)
	}
}

func TestTheNoticeSurvivesConnectingThatPartlyFails(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file its owner can't read")
	}
	d := newTestDeck(t)
	home := homeDir
	writeFile(t, filepath.Join(home, ".claude", "settings.json"), `{}`)
	codex := filepath.Join(home, ".codex", "hooks.json")
	writeFile(t, codex, `{}`)
	if err := os.Chmod(codex, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(codex, 0o644) })
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	writeFile(t, setupKeyFile, d.access.SetupKey+"\n")
	response := pagePost(t, d, server.URL, "/setup/hooks")
	defer response.Body.Close()
	said, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusInternalServerError || !strings.Contains(string(said), "can't be read") || !strings.Contains(string(said), voiceNotice) {
		t.Errorf("answer %d %q, want the error and the voice notice", response.StatusCode, said)
	}
}

func TestTheSetupPageAnswerCarriesTheVoiceNoticeAndRepairNeverTurnsVoiceOn(t *testing.T) {
	d := newTestDeck(t)
	settings := filepath.Join(homeDir, ".claude", "settings.json")
	writeFile(t, settings, `{}`)
	server := httptest.NewServer(&router{deck: d})
	defer server.Close()
	writeFile(t, setupKeyFile, d.access.SetupKey+"\n")
	client := setupClient{base: server.URL, http: server.Client()}

	// The repair path connects, but leaves voice alone, so there's nothing to tell
	if err := client.fix(fixRepair); err != nil {
		t.Fatal(err)
	}
	if _, set := readJSON(t, settings)["voice"]; set {
		t.Error("repair turned voice on")
	}
	if _, err := os.Stat(voiceMarker()); err == nil {
		t.Error("repair left a voice marker")
	}

	// Connect turns it on, and the answer says so
	response := pagePost(t, d, server.URL, "/setup/hooks")
	defer response.Body.Close()
	var state map[string]any
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil || state["notice"] != voiceNotice {
		t.Errorf("the answer's notice is %v (%v), want the voice notice", state["notice"], err)
	}
}

// onPort makes Port what an office on that port has, for one test.
func onPort(t *testing.T, port int) {
	t.Helper()
	saved := Port
	t.Cleanup(func() { Port = saved })
	Port = port
}

func TestTheHookCommandCarriesThePort(t *testing.T) {
	isolate(t)
	onPort(t, 4321)
	program := shellPath(commandLineProgram())
	for agent, want := range map[Agent]string{
		Claude: program + " hook --port 4321",
		Grok:   program + " hook --port 4321",
		Codex:  program + " hook --port 4321 codex",
		Gemini: program + " hook --port 4321 gemini",
	} {
		if got := hookCommand(agent); got != want {
			t.Errorf("hookCommand(%s) = %q, want %q", agent, got, want)
		}
	}
	// Written by the office on the default port: a copy on another port never writes your agents' settings.
	onDefaultPort(t)
	writeFile(t, filepath.Join(homeDir, ".claude", "settings.json"), "{}")
	if err := connect(Claude, false); err != nil {
		t.Fatal(err)
	}
	if stop := commands(readJSON(t, entriesFor(Claude).file), "Stop"); len(stop) != 1 || stop[0] != program+" hook --port 7710" {
		t.Errorf("Stop runs %q, want the command with the port", stop)
	}
}

// onDefaultPort makes this office the one on the default port, for one test: the one that writes your agents'
// settings (leavesAgentsAlone).
func onDefaultPort(t *testing.T) {
	t.Helper()
	onPort(t, portFromEnvironment(""))
}

// anotherOfficesHook is the deck's Claude Code entry for Stop as the office on port wrote it, for a test to put in
// the settings by hand: only the office on the default port writes them now.
func anotherOfficesHook(t *testing.T, port int) string {
	t.Helper()
	saved := Port
	Port = port
	command := hookCommand(Claude)
	Port = saved
	return command
}

// What isOurs takes as the deck's: every form the deck ever wrote, and the
// new one, with any valid port (see isHookArguments).
func TestHookCommandsOfEveryFormAreTheDecks(t *testing.T) {
	for command, want := range map[string]bool{
		"'/home/me/.local/bin/smolcorp' hook --port 7710":         true,
		"'/home/me/.local/bin/smolcorp' hook --port 7720 codex":   true,
		"'/home/me/.local/bin/smolcorp' hook":                     true, // m2, single-quoted, no port
		"'/home/me/.local/bin/smolcorp' hook gemini":              true,
		`"/home/me/.local/bin/smolcorp" hook --port 7710`:         true,
		`"/home/me/.local/bin/smolcorp" hook`:                     true, // double-quoted, before m2
		`"/home/me/.local/bin/smolcorp" hook codex`:               true,
		"/home/me/.local/bin/smolcorp hook":                       true, // unquoted, oldest
		"/home/me/.local/bin/smolcorp hook codex":                 true,
		"/home/me/.local/bin/smolcorp hook --port 4321":           true,
		"'/home/me/.local/bin/smolcorp' hook --port":              false,
		"'/home/me/.local/bin/smolcorp' hook --port 80":           false,
		"'/home/me/.local/bin/smolcorp' hook --port x":            false,
		"'/home/me/.local/bin/smolcorp' hook --port 7710 nobody":  false,
		"'/home/me/.local/bin/smolcorp' hook --port 7710 codex x": false,
		"'/home/me/bin/notify' hook --port 7710":                  false,
	} {
		if got := ours(command); got != want {
			t.Errorf("isOurs(%q) = %v, want %v", command, got, want)
		}
	}
}

func TestReconnectingReplacesOldHooksWithTheOneWithThePort(t *testing.T) {
	home := isolate(t)
	onDefaultPort(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	old := func(command string) string {
		return `{"type":"command","command":"` + command + `","async":true}`
	}
	var groups []string
	for _, command := range []string{
		"'/home/me/.local/bin/smolcorp' hook", `\"/home/me/.local/bin/smolcorp\" hook`, "/home/me/.local/bin/smolcorp hook",
		"'/home/me/.local/bin/smolcorp' hook --port 7720", "notify-send mine",
	} {
		groups = append(groups, `{"hooks":[`+old(command)+`]}`)
	}
	writeFile(t, settings, `{"hooks":{"Stop":[`+strings.Join(groups, ",")+`]}}`)
	if err := connect(Claude, false); err != nil {
		t.Fatal(err)
	}
	stop := commands(readJSON(t, settings), "Stop")
	want := []string{"notify-send mine", hookCommand(Claude)}
	if len(stop) != 2 || stop[0] != want[0] || stop[1] != want[1] {
		t.Errorf("Stop runs %q, want %q", stop, want)
	}
}

// Another office's port is the deck's too: an agent's settings hold one
// office's hook per event, so the one that connects last has it, never two.
func TestAnotherOfficesPortIsReplacedNotKeptBesideIt(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	other := anotherOfficesHook(t, 4322)
	writeFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"`+inJSON(other)+`","async":true}]}]}}`)
	onDefaultPort(t)
	if err := connect(Claude, false); err != nil {
		t.Fatal(err)
	}
	if stop := commands(readJSON(t, settings), "Stop"); len(stop) != 1 || stop[0] == other || stop[0] != hookCommand(Claude) {
		t.Errorf("Stop runs %q, want only the office on 7710", stop)
	}
}

// The hook reports to the port in its command, not to the one it inherits.
func TestTheHookReportsToThePortInItsCommand(t *testing.T) {
	isolate(t)
	t.Setenv(summaryRunVariable, "")
	writeFile(t, hookKeyFile, "test-key\n")
	got := make(chan string, 4)
	named := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.URL.Path + " " + r.Header.Get("X-AgentDeck-Agent") + " " + r.Header.Get(hookKeyHeader)
	}))
	defer named.Close()
	inherited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the hook reported to the inherited port")
	}))
	defer inherited.Close()
	onPort(t, mustPort(t, inherited.URL))
	stdin := os.Stdin
	defer func() { os.Stdin = stdin }()
	read, write, _ := os.Pipe()
	write.WriteString(`{"session_id":"s","hook_event_name":"SessionStart"}`)
	write.Close()
	os.Stdin = read
	if RunHook([]string{"--port", fmt.Sprint(mustPort(t, named.URL)), "codex"}) != 0 {
		t.Fatal("the hook failed")
	}
	select {
	case report := <-got:
		if report != "/hook codex test-key" {
			t.Errorf("the hook sent %q", report)
		}
	default:
		t.Error("nothing reached the port in the command")
	}
}

// A test copy's hire reports with the copy's own key, from the folder its start script names: the app's
// key file is never read, and isn't even there to read (FXM1's N11, end to end). The test itself reads
// only its own temporary folders.
func TestAHookInATestCopySendsTheCopysKeyNeverTheApps(t *testing.T) {
	isolate(t)
	t.Setenv(summaryRunVariable, "")
	// A launcher's session names its tab without a terminal, so the hook sends whether or not this
	// test runs under a tty (on a Mac, a hook with no tab sends nothing, as report.sh).
	t.Setenv("SMOLCORP_SESSION", "test-copy-hook")
	copyFolder := t.TempDir()
	writeFile(t, filepath.Join(copyFolder, "hook-key"), "copy-key\n")
	t.Setenv(testSupportVariable, copyFolder)
	// The app's own key, in this test's support folder: never what's sent.
	writeFile(t, hookKeyFile, "app-key\n")
	got := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get(hookKeyHeader)
	}))
	defer server.Close()
	send := func() string {
		stdin := os.Stdin
		defer func() { os.Stdin = stdin }()
		read, write, _ := os.Pipe()
		write.WriteString(`{"session_id":"s","hook_event_name":"SessionStart"}`)
		write.Close()
		os.Stdin = read
		if RunHook([]string{"--port", fmt.Sprint(mustPort(t, server.URL)), "claude"}) != 0 {
			t.Fatal("the hook failed")
		}
		select {
		case key := <-got:
			return key
		default:
			return ""
		}
	}
	if key := send(); key != "copy-key" {
		t.Errorf("a test copy's hook sent %q, want the copy's key", key)
	}
	// With no key in the copy's folder, nothing is sent: the hook never falls back to the app's key.
	if err := os.Remove(filepath.Join(copyFolder, "hook-key")); err != nil {
		t.Fatal(err)
	}
	if key := send(); key != "" {
		t.Errorf("with no key in the copy's folder, the hook sent %q", key)
	}
}

func mustPort(t *testing.T, url string) int {
	t.Helper()
	n, err := strconv.Atoi(url[strings.LastIndex(url, ":")+1:])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// The start-up refresh leaves another office's hooks on its port, so a second
// office on a shared HOME never moves the first's.
func TestRefreshKeepsAnotherOfficesPort(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	other := anotherOfficesHook(t, 4322)
	writeFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"`+inJSON(other)+`","async":true}]}]}}`)
	before, _ := os.ReadFile(settings)
	onDefaultPort(t)
	refreshHooks()
	if after, _ := os.ReadFile(settings); string(after) != string(before) {
		t.Errorf("the refresh changed another office's settings:\n%s", after)
	}
	if stop := commands(readJSON(t, settings), "Stop"); len(stop) != 1 || stop[0] != other {
		t.Errorf("Stop runs %q, want %q", stop, other)
	}
}

// An entry without a port, as an earlier version wrote it, is upgraded.
func TestRefreshUpgradesAnEntryWithoutAPort(t *testing.T) {
	home := isolate(t)
	onDefaultPort(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"'/home/me/.local/bin/smolcorp' hook","async":true}]}]}}`)
	refreshHooks()
	if stop := commands(readJSON(t, settings), "Stop"); len(stop) != 1 || stop[0] != hookCommand(Claude) {
		t.Errorf("Stop runs %q, want %q", stop, hookCommand(Claude))
	}
}

// An entry on this office's own port is refreshed as before: moved copy, new path.
func TestRefreshUpdatesAnEntryOnThisOfficesPort(t *testing.T) {
	home := isolate(t)
	onDefaultPort(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"'/home/me/.local/bin/smolcorp' hook --port 7710","async":true}]}]}}`)
	refreshHooks()
	if stop := commands(readJSON(t, settings), "Stop"); len(stop) != 1 || stop[0] != hookCommand(Claude) {
		t.Errorf("Stop runs %q, want %q", stop, hookCommand(Claude))
	}
}

// OpenCode's plugin names the port too: the refresh leaves another office's.
func TestRefreshKeepsAnotherOfficesOpenCodePlugin(t *testing.T) {
	isolate(t)
	if err := os.MkdirAll(agentFolder(OpenCode), 0o755); err != nil {
		t.Fatal(err)
	}
	onDefaultPort(t)
	before := anotherOfficesPlugin(t, 4322)
	refreshHooks()
	if after, _ := os.ReadFile(openCodePlugin()); string(after) != string(before) {
		t.Error("the refresh moved another office's OpenCode plugin")
	}
	if err := connect(OpenCode, false); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(openCodePlugin()); !strings.Contains(string(after), `"7710"`) {
		t.Error("connecting did not move the plugin to this office")
	}
}

// anotherOfficesPlugin is OpenCode's plugin as the office on port wrote it, put in place by hand on the default
// port (only that office writes it), and the plugin's text.
func anotherOfficesPlugin(t *testing.T, port int) []byte {
	t.Helper()
	if err := writePlugin(); err != nil {
		t.Fatal(err)
	}
	plugin, _ := os.ReadFile(openCodePlugin())
	other := bytes.Replace(plugin, []byte(fmt.Sprintf(`"--port","%d"`, Port)), []byte(fmt.Sprintf(`"--port","%d"`, port)), 1)
	if bytes.Equal(other, plugin) {
		t.Fatal("the plugin names no port")
	}
	writeFile(t, openCodePlugin(), string(other))
	return other
}

// Connecting is deliberate: it replaces another office's port.
func TestConnectReplacesAnotherOfficesPort(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"'/home/me/.local/bin/smolcorp' hook --port 4322","async":true}]}]}}`)
	onDefaultPort(t)
	if err := connect(Claude, false); err != nil {
		t.Fatal(err)
	}
	if stop := commands(readJSON(t, settings), "Stop"); len(stop) != 1 || stop[0] != hookCommand(Claude) {
		t.Errorf("Stop runs %q, want %q", stop, hookCommand(Claude))
	}
}

// A port the hook can't use (out of range, not a number) is the inherited one.
func TestTheHookFallsBackFromABadPort(t *testing.T) {
	for _, bad := range []string{"80", "abc", "1023", "65536", ""} {
		t.Run(bad, func(t *testing.T) {
			isolate(t)
			t.Setenv(summaryRunVariable, "")
			writeFile(t, hookKeyFile, "test-key\n")
			reached := make(chan string, 4)
			inherited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached <- r.Header.Get("X-AgentDeck-Agent")
			}))
			defer inherited.Close()
			onPort(t, mustPort(t, inherited.URL))
			stdin := os.Stdin
			defer func() { os.Stdin = stdin }()
			read, write, _ := os.Pipe()
			write.WriteString(`{"session_id":"s","hook_event_name":"SessionStart"}`)
			write.Close()
			os.Stdin = read
			if RunHook([]string{"--port", bad, "codex"}) != 0 {
				t.Fatal("the hook failed")
			}
			select {
			case agent := <-reached:
				if agent != "codex" {
					t.Errorf("agent %q, want codex", agent)
				}
			default:
				t.Errorf("--port %q did not reach the inherited port", bad)
			}
		})
	}
}

// Under the Mac app, the Swift engine's own entries (its report.sh in the support folder, as Hooks.swift wrote
// them) are the deck's: the start-up refresh puts this program's in their place, every event once, and leaves
// the user's own and a look-alike alone. The script itself stays, for a hire the Swift engine made.
func TestUnderTheMacAppTheRefreshReplacesTheSwiftEnginesReportScript(t *testing.T) {
	home := isolate(t)
	onDefaultPort(t)
	t.Cleanup(func() { hookHeaderFile = "" })
	hookHeaderFile = filepath.Join(supportDir, "hook-header")
	script := filepath.Join(supportDir, "report.sh")
	writeFile(t, script, "#!/bin/sh\n")
	swift := `"` + script + `"`
	settings := filepath.Join(home, ".claude", "settings.json")
	// Encoded, not spliced in: a Windows path's backslashes are JSON's escapes.
	fixture, _ := json.Marshal(map[string]any{"hooks": map[string]any{"Stop": []any{
		map[string]any{"hooks": []any{map[string]any{"type": "command", "command": swift, "async": true}}},
		map[string]any{"hooks": []any{map[string]any{"type": "command", "command": swift + " --mine"}}},
		map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "say done"}}},
	}}})
	writeFile(t, settings, string(fixture))
	refreshHooks()
	stop := commands(readJSON(t, settings), "Stop")
	want := []string{`"` + script + `" --mine`, "say done", hookCommand(Claude)}
	if !slices.Equal(stop, want) {
		t.Errorf("Stop runs %q, want %q", stop, want)
	}
	if start := commands(readJSON(t, settings), "SessionStart"); !slices.Equal(start, []string{hookCommand(Claude)}) {
		t.Errorf("SessionStart runs %q: the refresh brings every event, as connecting does", start)
	}
	if _, err := os.Stat(script); err != nil {
		t.Error("the Swift engine's script was taken away")
	}

	// Codex's entry names its agent after the script.
	if !isOurs(map[string]any{"command": `"` + script + `" codex`}) || isOurs(map[string]any{"command": `"` + script + `" nobody`}) {
		t.Error("an agent's report.sh entry is the deck's only with an agent's name after it")
	}
	// Exact (RGAPC N2): the Swift engine wrote the bare path for Claude Code and Grok, and the agent's name only for
	// Codex, Gemini and OpenCode; `claude` or `grok` after it was never written, so it is the user's own.
	for _, command := range []string{`"` + script + `" claude`, `"` + script + `" grok`} {
		if isOurs(map[string]any{"command": command}) {
			t.Errorf("%s counts as the deck's, which Swift never wrote", command)
		}
	}
	for _, name := range []string{"codex", "gemini", "opencode"} {
		if !isOurs(map[string]any{"command": `"` + script + `" ` + name}) || !isOurs(map[string]any{"command": `"` + script + `"`}) {
			t.Errorf("an entry Swift wrote (%s) isn't the deck's", name)
		}
	}
	// Not under the Mac app (Linux, Windows), nor another folder's script: not the deck's.
	if isOurs(map[string]any{"command": `"/elsewhere/report.sh"`}) {
		t.Error("another folder's report.sh counts as the deck's")
	}
	hookHeaderFile = ""
	if isOurs(map[string]any{"command": `"` + script + `"`}) {
		t.Error("without the Mac app, report.sh counts as the deck's")
	}
}
