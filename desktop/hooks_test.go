package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	command, _ := json.Marshal([]string{commandLineProgram(), "hook", "opencode"})
	if !strings.Contains(string(plugin), "const COMMAND = "+string(command)) || strings.Contains(string(plugin), "{{COMMAND}}") {
		t.Fatalf("the plugin doesn't run %s", command)
	}
}

func TestSettingsThatArentJSONAreLeftAlone(t *testing.T) {
	home := isolate(t)
	settings := filepath.Join(home, ".gemini", "settings.json")
	broken := `{"theme": "dark",`
	writeFile(t, settings, broken)
	err := connectAgents()
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

	if err := connectAgents(); err == nil || !strings.Contains(err.Error(), "can't be read") {
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
	if err := connectAgents(); err == nil {
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

func TestOnlyAgentsWithASettingsFolderAreFound(t *testing.T) {
	home := isolate(t)
	if err := connectAgents(); err != errNoAgents {
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
		"C:/PROGRA~1/SMOLCO~1/SMOLCO~1.EXE hook codex":    true,
		"C:/PROGRA~1/SAUZZE~1/SAUZZE~1.EXE hook codex":    true,
		"C:/Program Files/Smolcorp/smolcorp-cli.exe hook": true,
		"say done":                       false,
		"/usr/local/bin/other-tool hook": false,
		"/home/someone/.local/bin/smolcorp run claude": false,
	} {
		if got := isOurs(map[string]any{"type": "command", "command": command}); got != ours {
			t.Errorf("isOurs(%q) = %v, want %v", command, got, ours)
		}
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
