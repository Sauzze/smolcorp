package desktop

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// SA-T1: a desk runs inside Smolcorp's sandbox, where an agent's or a tool's own sandbox can't start (SANDBOX-PLAN
// §A4.3). So a desk's Claude Code settings switch Claude Code's own off, every desk starts with Claude Code's
// auto-update off (and husky's hooks as the owner has them: RT126SAT1 B1), and every brief Smolcorp writes says how
// to turn a developer tool's own off.

// compactJSON is value on one line, as hireSettingsLine writes it.
func compactJSON(t *testing.T, value any) []byte {
	t.Helper()
	var line bytes.Buffer
	encoder := json.NewEncoder(&line)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(line.Bytes(), []byte("\n"))
}

// The sandbox key as the settings line holds it.
const settingsSandboxOff = `"sandbox":{"enabled":false}`

// A hire's settings and the GM's both switch Claude Code's own sandbox off, as a key of their own with nothing else
// in it, so it can't be read as on, and a project's settings, which rank below the command line's, can't turn it on.
func TestADesksSettingsSwitchClaudeCodesOwnSandboxOff(t *testing.T) {
	r := newHireSettingsRig(t)
	for _, profile := range []guardProfile{guardHire, guardGM} {
		for _, auto := range []bool{false, true} {
			line, err := r.write("abc123", func(o *hireSettingsOptions) { o.Profile, o.Auto = profile, auto })
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(line, `"sandbox":`) != 1 || !strings.Contains(line, settingsSandboxOff) {
				t.Errorf("%s, auto %v: the settings don't switch Claude Code's sandbox off:\n%s", profile, auto, line)
			}
			sandbox, ok := r.json(line)["sandbox"].(map[string]any)
			if !ok || len(sandbox) != 1 || sandbox["enabled"] != false {
				t.Errorf("%s, auto %v: sandbox is %v", profile, auto, r.json(line)["sandbox"])
			}
		}
	}
}

// The key is 28 characters, 32 quoted for Windows: a default hire's settings, every deny rule and the core's groups
// in them, still fit the cap TestADesksSettingsFitInACommandLine holds them to.
func TestTheSandboxKeyIsAllItAddsToTheSettingsLine(t *testing.T) {
	r := newHireSettingsRig(t)
	line, err := r.write("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	object, err := hireSettingsObject(hireSettingsOptions{HireID: "abc123", Auto: true, CLI: r.cli, Support: r.support,
		Profiles: []claudeProfile{r.work}, Run: func(string, []string) (int, string, bool) { return 0, hireGuardPrinted, true },
		IsExec: func(string) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	delete(object, "sandbox")
	without := string(compactJSON(t, object))
	if added := len(line) - len(without); added != len(","+settingsSandboxOff) {
		t.Errorf("the sandbox key adds %d characters, not %d", added, len(","+settingsSandboxOff))
	}
	if quoted := strings.Count(","+settingsSandboxOff, `"`) + len(","+settingsSandboxOff); quoted != 32 {
		t.Errorf("%d characters quoted", quoted)
	}
}

// A T3 Code desk isn't in Smolcorp's sandbox, so its fence leaves Claude Code's own as the owner set it: a fence
// that switched it off would take away a sandbox the owner turned on, with nothing in its place.
func TestAT3FenceLeavesClaudeCodesOwnSandboxAlone(t *testing.T) {
	r := newHireSettingsRig(t)
	settings, err := hireSettingsObject(hireSettingsOptions{HireID: "abc123", Auto: true, CLI: r.cli, Support: r.support,
		Run: func(string, []string) (int, string, bool) { return 0, hireGuardPrinted, true }, IsExec: func(string) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	if settings["sandbox"] == nil {
		t.Fatal("a hire's own settings have no sandbox key")
	}
	if fence := t3Fence(settings); fence["sandbox"] != nil || strings.Contains(string(swiftJSON(fence)), `"sandbox"`) {
		t.Errorf("the T3 fence switches Claude Code's sandbox: %v", fence["sandbox"])
	}
}

// Every brief Smolcorp writes ends with the lines that turn a tool's own sandbox off, once, whatever its template:
// the library's, at a pinned desk, kept in Smolcorp's briefs folder or in a managed hire's own folder, and a file
// the owner picked, with or without a line end at its end (the section's heading still has a blank line before it).
// Setup's preview, which writes nothing, shows the template's brief alone.
func TestEveryWrittenBriefSaysHowToTurnAToolsOwnSandboxOff(t *testing.T) {
	isolate(t)
	library := briefLibrary{files: bundledBriefs()}
	date := time.Date(2027, 1, 15, 8, 0, 0, 0, time.UTC)
	picked, err := briefFile("# My brief\n\nYou are the {{role}}. Report in `{{status_file}}`.\n", "mine.md")
	if err != nil {
		t.Fatal(err)
	}
	unended, err := briefFile("# Mine too\n\nYou are the {{role}}. Report in `{{status_file}}`.", "unended.md")
	if err != nil {
		t.Fatal(err)
	}
	roles := []Role{
		{ID: "tester", Name: "Tester", Agent: "claude", Brief: &RoleBrief{Kind: "library", ID: "tester"}},
		{ID: "mine", Name: "Mine", Agent: "codex", Brief: &picked},
		{ID: "unended", Name: "Unended", Agent: "claude", Brief: &unended},
	}
	tools := briefSandboxTools(runtime.GOOS)
	lines := []string{"`--no-sandbox`"}
	if runtime.GOOS == "darwin" {
		lines = append(lines, "`--disable-sandbox`", "`/usr/bin/swift`", "`ENABLE_USER_SCRIPT_SANDBOXING=NO`")
	}
	for _, role := range roles {
		for _, place := range []string{"pinned", "kept", "managed"} {
			request := briefRequest{role: role, library: library, folder: t.TempDir(), date: date, kept: briefsKeptDir(), write: true}
			switch place {
			case "pinned":
				request.pinned = true
			case "managed":
				request.managed = hireFolder(supportDir, "h-"+role.ID)
			}
			made, err := makeBrief(request)
			if err != nil || made == nil {
				t.Fatalf("%s, %s: not made: %v", role.ID, place, err)
			}
			data, err := os.ReadFile(made.names.brief)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if text != made.markdown || !strings.HasSuffix(text, "\n\n"+strings.TrimPrefix(tools, "\n")) ||
				strings.Count(text, "## Tools with a sandbox of their own") != 1 {
				t.Errorf("%s, %s: the brief doesn't end with the sandbox lines:\n%s", role.ID, place, text)
			}
			for _, line := range lines {
				if !strings.Contains(text, line) {
					t.Errorf("%s, %s: the brief doesn't say %s", role.ID, place, line)
				}
			}
		}
		preview, err := makeBrief(briefRequest{role: role, library: library, folder: t.TempDir(), date: date, kept: briefsKeptDir()})
		if err != nil || preview == nil || strings.Contains(preview.markdown, "--no-sandbox") {
			t.Errorf("%s: the preview: %v\n%v", role.ID, err, preview)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(supportDir, "hires")); len(entries) != len(roles) {
		t.Errorf("the managed briefs went in %d folders", len(entries))
	}
}

// Swift's and Xcode's own sandboxes are the Mac's: a Mac's briefs name them, a Linux or Windows computer's don't. The
// headless browser's line is in every system's.
func TestOnlyAMacsBriefNamesSwiftAndXcode(t *testing.T) {
	macOnly := []string{"`--disable-sandbox`", "`/usr/bin/swift`", "`ENABLE_USER_SCRIPT_SANDBOXING=NO`", "Xcode"}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		text := briefSandboxTools(goos)
		if !strings.HasPrefix(text, "\n## Tools with a sandbox of their own\n") || !strings.HasSuffix(text, "start it with `--no-sandbox`. Playwright already does.\n") {
			t.Errorf("%s: %q", goos, text)
		}
		for _, line := range macOnly {
			if strings.Contains(text, line) != (goos == "darwin") {
				t.Errorf("%s: %s named: %v", goos, line, strings.Contains(text, line))
			}
		}
	}
}
