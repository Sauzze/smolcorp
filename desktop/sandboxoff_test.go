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
	playSandboxedHere(t, true) // as on a Mac, or on Linux once SA-L4 starts desks in theirs
	tools := briefSandboxTools(runtime.GOOS, true)
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

// Swift's and Xcode's own sandboxes are the Mac's: a Mac's briefs name them, a Linux computer's (once its desks start
// in their sandbox) doesn't. So is chrome-headless-shell with --single-process, how a hire looks at a page in a Mac
// desk (T126HEADLESSSHELL1): on a Mac that line takes the place of the headless browser line that ends every other
// sandboxed system's. Where desks start in no sandbox (Linux until SA-L4, Windows in 1.26), there is no section at
// all (RT126SAX14 B1).
func TestOnlyAMacsBriefNamesSwiftAndXcode(t *testing.T) {
	macOnly := []string{"`--disable-sandbox`", "`/usr/bin/swift`", "`ENABLE_USER_SCRIPT_SANDBOXING=NO`", "Xcode", "`chrome-headless-shell`",
		"`--no-sandbox --single-process --use-mock-keychain --password-store=basic`", "still being checked"}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		if text := briefSandboxTools(goos, false); text != "" {
			t.Errorf("%s, no sandbox: %q", goos, text)
		}
		text := briefSandboxTools(goos, true)
		end := "start it with `--no-sandbox`. Playwright already does.\n"
		if goos == "darwin" {
			end = "are still being checked here, so don't count on them.\n"
		}
		if !strings.HasPrefix(text, "\n## Tools with a sandbox of their own\n") || !strings.HasSuffix(text, end) {
			t.Errorf("%s: %q", goos, text)
		}
		for _, line := range macOnly {
			if strings.Contains(text, line) != (goos == "darwin") {
				t.Errorf("%s: %s named: %v", goos, line, strings.Contains(text, line))
			}
		}
	}
}

// RT126HEADLESSSHELL1 F1: a Mac's brief says only what was seen to work in a Mac desk. Chromium with --no-sandbox alone
// hangs there, so no line may tell a hire that it's enough (the other systems' line, "start it with `--no-sandbox`.
// Playwright already does.", is not in it); each `--no-sandbox` it names is with --single-process, or says it hangs
// alone. One line is about Chrome, and it names Puppeteer, Playwright and a DevTools connection only as still being
// checked: a CDP session in a desk was never run.
func TestAMacsBriefNeverSaysNoSandboxAloneRunsChromium(t *testing.T) {
	text := briefSandboxTools("darwin", true) // a Mac's desks start in their sandbox (deskSandboxed)
	for _, said := range []string{"Playwright already does", "start it with `--no-sandbox`", "take both in the browser's launch arguments"} {
		if strings.Contains(text, said) {
			t.Errorf("a Mac's brief says %q:\n%s", said, text)
		}
	}
	rest := strings.ReplaceAll(text, "`--no-sandbox --single-process --use-mock-keychain --password-store=basic`", "")
	rest = strings.ReplaceAll(rest, "`--no-sandbox` alone hangs", "")
	if strings.Contains(rest, "no-sandbox") {
		t.Errorf("a Mac's brief names --no-sandbox without --single-process, or not as hanging alone:\n%s", text)
	}
	var chrome []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "Chrom") || strings.Contains(line, "headless") {
			chrome = append(chrome, line)
		}
	}
	if len(chrome) != 1 {
		t.Fatalf("a Mac's brief has %d lines about headless Chrome, not one:\n%s", len(chrome), strings.Join(chrome, "\n"))
	}
	line := chrome[0]
	// The DOM's flag is pinned by its words here, not by its name: CHROMEKILL1's guard fails a browser's own flag in
	// any string of this package's tests (headlesschrome_guard_test.go's browserFlags).
	for _, said := range []string{"Google Chrome's own headless mode doesn't start", "print a page's DOM", "`--screenshot`", "not an app"} {
		if !strings.Contains(line, said) {
			t.Errorf("a Mac's headless Chrome line doesn't say %q: %s", said, line)
		}
	}
	checked := strings.Index(line, "are still being checked here")
	for _, way := range []string{"Puppeteer", "Playwright", "DevTools"} {
		if at := strings.Index(line, way); at < 0 || checked < 0 || at > checked || strings.Count(line, way) != 1 {
			t.Errorf("a Mac's headless Chrome line names %s other than as still being checked: %s", way, line)
		}
	}
	// The lines below it: none on a Mac. The other systems' line stays theirs, where their desks start in a sandbox (with
	// none, they have no section: TestOnlyAMacsBriefNamesSwiftAndXcode).
	if !strings.HasSuffix(text, line+"\n") {
		t.Errorf("a Mac's headless Chrome line isn't its brief's last:\n%s", text)
	}
	for _, goos := range []string{"linux", "windows"} {
		if other := briefSandboxTools(goos, true); !strings.Contains(other, "start it with `--no-sandbox`. Playwright already does.") || strings.Contains(other, "single-process") {
			t.Errorf("%s: %q", goos, other)
		}
	}
}
