package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// SANDBOX-PLAN §3.4, SG1: the owner's guard is held by the core, desks get it in their guard's command line, the
// settings are inline with their keys pinned, and a Claude Code that can't make a stopped guard refuse gets no desk.

// pushOn is whether push is on in groups.
func pushOn(groups []guard.Group) bool {
	i := slices.IndexFunc(groups, func(g guard.Group) bool { return g.Name == "push" })
	return i >= 0 && groups[i].On
}

// g1: the core reads the guard file once; a hire that changes it afterwards changes nothing the core holds: not
// Settings' state, not the groups the next desk starts with.
func TestTheCoreHoldsTheGuardItReadAtItsStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	t.Cleanup(func() { forgetHeldGuard(path) })
	owners := guardDefaults()
	owners.on["push"] = false
	if err := os.WriteFile(path, guardFileData(owners), 0o600); err != nil {
		t.Fatal(err)
	}
	if pushOn(heldGuardGroups(path)) {
		t.Fatal("the owner's push off wasn't read")
	}
	// The hire switches push back off and deploy off too, then makes the file a mess: the core holds what it read.
	loose := guardDefaults()
	loose.on["push"], loose.on["deploy"] = false, false
	for _, text := range [][]byte{guardFileData(loose), []byte("not a guard file")} {
		if err := os.WriteFile(path, text, 0o600); err != nil {
			t.Fatal(err)
		}
		groups := heldGuardGroups(path)
		if i := slices.IndexFunc(groups, func(g guard.Group) bool { return g.Name == "deploy" }); i < 0 || !groups[i].On {
			t.Errorf("the hire's file switched deploy off in the core: %+v", groups)
		}
		state := guardState(path, "darwin")
		if state["problem"] != nil || state["groups"].([]map[string]any)[1]["on"] != true {
			t.Errorf("Settings shows the hire's file: %v", state)
		}
	}
}

// The core reads the guard file as it starts, not at the first desk start or Settings view: a desk left running
// across a restart that loosens the file in between changes nothing the core holds.
func TestTheCoreReadsTheGuardFileAsItStarts(t *testing.T) {
	isolate(t)
	path := filepath.Join(supportDir, hireGuardFile)
	t.Cleanup(func() { forgetHeldGuard(path) })
	if err := os.WriteFile(path, guardFileData(guardDefaults()), 0o600); err != nil {
		t.Fatal(err)
	}
	startDeck(t)
	loose := guardDefaults()
	loose.on["push"] = false
	if err := os.WriteFile(path, guardFileData(loose), 0o600); err != nil {
		t.Fatal(err)
	}
	if !pushOn(heldGuardGroups(path)) {
		t.Error("a change made after the core started counted")
	}
}

// Settings changes the core's copy at once, and writes the file from it.
func TestSettingsChangesTheCoresCopyAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	t.Cleanup(func() { forgetHeldGuard(path) })
	if !pushOn(heldGuardGroups(path)) {
		t.Fatal("push isn't on with no file")
	}
	if answer := saveGuard([]byte(`{"groups":[{"name":"push","on":false}],"extra":["helm install"]}`), path, "darwin"); answer.status != 200 {
		t.Fatalf("save: %+v", answer)
	}
	groups := heldGuardGroups(path)
	if pushOn(groups) || !strings.Contains(guard.GroupsArgument(groups), `"helm install"`) {
		t.Errorf("the core's copy wasn't changed: %+v", groups)
	}
	written, _ := os.ReadFile(path)
	if !bytes.Equal(written, guardFileData(func() guardStored { s, _ := loadGuard(path); return s }())) {
		t.Errorf("the file isn't the core's copy:\n%s", written)
	}
}

// The core's copy is a sealed record: with a record key, a file whose seal doesn't fit at the next start isn't
// honoured (the built-in guard, plus what the file adds at each call), and Settings says so. Without a key, the
// file found at the start counts, as off a Mac.
func TestTheGuardFileIsHonouredAtTheNextStartOnlyIfItsSealFits(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	t.Cleanup(func() { forgetHeldGuard(path) })
	saved := guardRecordKey
	t.Cleanup(func() { guardRecordKey = saved })
	key := []byte("FAKEKEY-record-sg1-0123456789abc")
	guardRecordKey = func() []byte { return key }

	if answer := saveGuard([]byte(`{"groups":[{"name":"push","on":false}]}`), path, "darwin"); answer.status != 200 {
		t.Fatalf("save: %+v", answer)
	}
	seal, err := os.ReadFile(filepath.Join(filepath.Dir(path), hireGuardSealFile))
	if err != nil || len(strings.TrimSpace(string(seal))) != 64 {
		t.Fatalf("no seal written: %q %v", seal, err)
	}
	// The next start: the seal fits, the owner's push off holds.
	forgetHeldGuard(path)
	if pushOn(heldGuardGroups(path)) {
		t.Fatal("a sealed file wasn't honoured")
	}
	// A program changes the file (an added group switched off, a rule taken away) or copies the seal of another:
	// at the next start nothing of it is honoured.
	data, _ := os.ReadFile(path)
	for name, text := range map[string][]byte{
		"changed":   bytes.Replace(data, []byte(`"on" : true`), []byte(`"on" : false`), 1),
		"appended":  append(slices.Clone(data), ' '),
		"other key": data,
	} {
		if err := os.WriteFile(path, text, 0o600); err != nil {
			t.Fatal(err)
		}
		if name == "other key" {
			guardRecordKey = func() []byte { return []byte("FAKEKEY-another-record-key-00000") }
		}
		forgetHeldGuard(path)
		groups := heldGuardGroups(path)
		if guard.GroupsArgument(groups) != guard.GroupsArgument(guard.DefaultGroups()) {
			t.Errorf("%s: the file was honoured: %+v", name, groups)
		}
		if problem, _ := guardState(path, "darwin")["problem"].(string); !strings.Contains(problem, "another program") {
			t.Errorf("%s: Settings doesn't say so: %q", name, problem)
		}
		guardRecordKey = func() []byte { return key }
	}
	// Without a record key, the file found at the start counts.
	guardRecordKey = func() []byte { return nil }
	forgetHeldGuard(path)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if pushOn(heldGuardGroups(path)) {
		t.Error("with no record key the file wasn't read")
	}
}

// The settings a desk starts with pin what a lower settings file could loosen (g3, g10, g13): the permission
// mode, bypass mode off, the shell prefix empty and the shell fixed, hooks on, and every one of this app's hooks
// in the exec form with onFailure "block": the guard before every tool, and before every settings reload.
func TestADesksSettingsPinWhatALowerFileCouldLoosen(t *testing.T) {
	r := newHireSettingsRig(t)
	for _, auto := range []bool{true, false} {
		line, err := r.write("abc123", func(o *hireSettingsOptions) { o.Auto, o.ReportPort, o.Shell = auto, 7799, "/bin/zsh" })
		if err != nil {
			t.Fatal(err)
		}
		object := r.json(line)
		permissions := object["permissions"].(map[string]any)
		if mode := permissions["defaultMode"]; mode != map[bool]string{true: "auto", false: "default"}[auto] || permissions["disableBypassPermissionsMode"] != "disable" {
			t.Errorf("auto %v: permissions %v", auto, permissions)
		}
		if env := object["env"].(map[string]any); len(env) != 2 || env["CLAUDE_CODE_SHELL_PREFIX"] != "" || env["CLAUDE_CODE_SHELL"] != "/bin/zsh" {
			t.Errorf("env %v", env)
		}
		if object["disableAllHooks"] != false {
			t.Error("hooks aren't kept on")
		}
		hooks := object["hooks"].(map[string]any)
		config := hooks["ConfigChange"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
		if config["command"] != r.cli || !slices.Equal(anyStrings(config["args"]), []string{"guard", "--config"}) {
			t.Errorf("ConfigChange hook %v", config)
		}
		// Every hook of this app's, on every event: exec form, an absolute program, onFailure "block".
		for event, groups := range hooks {
			for _, group := range groups.([]any) {
				for _, raw := range group.(map[string]any)["hooks"].([]any) {
					hook := raw.(map[string]any)
					if hook["command"] != r.cli || hook["args"] == nil || hook["onFailure"] != "block" || hook["type"] != "command" {
						t.Errorf("%s: hook %v", event, hook)
					}
				}
			}
		}
	}
}

func anyStrings(value any) []string {
	var out []string
	list, _ := value.([]any)
	for _, item := range list {
		text, _ := item.(string)
		out = append(out, text)
	}
	return out
}

// The shell a desk's Bash tool is pinned to: yours when it is the system's bash or zsh, else the system's.
func TestTheDesksShellIsTheSystems(t *testing.T) {
	all := func(string) bool { return true }
	onlyBash := func(path string) bool { return path == "/bin/bash" }
	cases := []struct {
		goos, yours string
		exists      func(string) bool
		want        string
	}{
		{"darwin", "/bin/zsh", all, "/bin/zsh"},
		{"darwin", "/bin/bash", all, "/bin/bash"},
		{"darwin", "/opt/homebrew/bin/fish", all, "/bin/bash"},
		{"darwin", "/opt/homebrew/bin/zsh", all, "/bin/zsh"},
		{"linux", "/usr/local/bin/zsh", onlyBash, "/bin/bash"},
		{"darwin", "/Users/x/evil/bash", all, "/bin/bash"},
		{"linux", "/usr/bin/zsh", all, "/usr/bin/zsh"},
		{"linux", "/bin/zsh", onlyBash, "/bin/bash"},
		{"linux", "", func(string) bool { return false }, ""},
		{"windows", "/bin/bash", all, ""},
	}
	for _, c := range cases {
		if got := pinnedShell(c.goos, c.yours, c.exists); got != c.want {
			t.Errorf("%s, %q: %q, want %q", c.goos, c.yours, got, c.want)
		}
	}
}

func TestClaudeCodeVersionsAreComparedAsNumbers(t *testing.T) {
	least := [3]int{2, 1, 314}
	for text, want := range map[string]bool{
		"2.1.314 (Claude Code)": true, "2.1.323 (Claude Code)": true, "2.2.0": true, "3.0.1": true, "2.1.1000": true,
		"2.1.313 (Claude Code)": false, "2.0.999": false, "1.9.400": false, "": false, "Claude Code": false, "2.1": false,
	} {
		if got := claudeVersionAtLeast(text, least); got != want {
			t.Errorf("%q: %v", text, got)
		}
	}
}

// RSBXPLANf N1: on Linux the guard ignores an LD_PRELOAD set through settings only because its build is static.
// The release builds it so.
func TestTheLinuxBuildStaysStatic(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "scripts", "release-desktop.sh"))
	if err != nil {
		t.Fatal(err)
	}
	build := regexp.MustCompile(`(?m)^build\(\) \{[^\n]*\n\s*CGO_ENABLED=0 GOOS="\$1" GOARCH="\$2" go build `)
	if !build.Match(script) {
		t.Error("release-desktop.sh's build() no longer sets CGO_ENABLED=0")
	}
	if strings.Contains(string(script), "CGO_ENABLED=1") {
		t.Error("release-desktop.sh turns cgo on somewhere")
	}
	var linux int
	for _, line := range strings.Split(string(script), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "build linux ") {
			linux++
		}
		if strings.Contains(line, "go build") && !strings.Contains(line, "CGO_ENABLED=0") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("a go build without CGO_ENABLED=0: %s", line)
		}
	}
	if linux == 0 {
		t.Error("no Linux build through build()")
	}
}

// A test copy's desk names its folder's guard file in the guard's command line: the guard never reads it from
// the environment (g4).
func TestADesksGuardNamesItsFolderInItsCommandLine(t *testing.T) {
	r := newHireSettingsRig(t)
	line, err := r.write("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	hook := r.hook(line)
	if i := slices.Index(hook, "--rules"); i < 0 || hook[i+1] != filepath.Join(r.support, hireGuardFile) {
		t.Errorf("hook %q", hook)
	}
	var object map[string]any
	_ = json.Unmarshal([]byte(line), &object)
	if strings.Contains(line, testSupportVariable) {
		t.Error("the settings name the test folder's variable")
	}
}

// A desk's settings ride in its command line, on Windows too, where a command line holds at most 32767
// characters: a default hire's, with every deny rule and the core's groups, stays well under that. (25000 since
// RT126SG2F B1's four rules for claude-install's temp file and folder forms, about 580 characters here: this test's
// support folder is a long temp path, which every rule of the folder repeats.)
func TestADesksSettingsFitInACommandLine(t *testing.T) {
	r := newHireSettingsRig(t)
	line, err := r.write("abc123", func(o *hireSettingsOptions) {
		o.Run = func(string, []string) (int, string, bool) {
			return 0, `{"groups":` + strings.TrimPrefix(guard.GroupsArgument(guard.DefaultGroups()), `{"groups":`), true
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	// Quoted for Windows, each quote takes a backslash.
	if quoted := len(line) + strings.Count(line, `"`); quoted > 25000 {
		t.Errorf("the settings are %d characters quoted", quoted)
	}
	t.Logf("a default hire's settings: %d characters, %d quoted", len(line), len(line)+strings.Count(line, `"`))
}

// RT126CMDLEN B1: each Claude account kept outside ~/.claude* adds its own deny rules, about 2,000 characters, so
// setup's 8 accounts in folders elsewhere make settings Windows can't start a desk with. On Windows that hire is
// refused before it starts, in a sentence that names those accounts and says what to do; the same accounts as
// ~/.claude-<name> fit, and no other system refuses anything.
func TestAWindowsDeskWhoseSettingsDontFitIsRefusedNamingItsAccounts(t *testing.T) {
	r := newHireSettingsRig(t)
	saved := homeDir
	homeDir = filepath.Join(r.root, "home") // a stand-in home: nothing is made in it, the rules only name it
	t.Cleanup(func() { homeDir = saved })
	elsewhere := []claudeProfile{{Name: "default", Dir: filepath.Join(homeDir, ".claude")}}
	tidy := slices.Clone(elsewhere)
	for i := 1; i <= 7; i++ {
		name := "client" + strconv.Itoa(i)
		elsewhere = append(elsewhere, claudeProfile{Name: name, Dir: filepath.Join(homeDir, "Documents", "Claude accounts", "client-"+strconv.Itoa(i)+"-northwind")})
		tidy = append(tidy, claudeProfile{Name: name, Dir: filepath.Join(homeDir, ".claude-client-"+strconv.Itoa(i)+"-northwind")})
	}
	write := func(system string, profiles []claudeProfile) (string, error) {
		return r.write("abc123", func(o *hireSettingsOptions) {
			o.System, o.Profiles = system, profiles
			o.Run = func(string, []string) (int, string, bool) {
				return 0, `{"groups":` + strings.TrimPrefix(guard.GroupsArgument(guard.DefaultGroups()), `{"groups":`), true
			}
		})
	}
	quoted := func(line string) int { return len(exeLine("--settings="+line, nil)) }

	_, err := write("windows", elsewhere)
	var refusal *hireRefusal
	if !errors.As(err, &refusal) || refusal.Status != 409 {
		t.Fatalf("8 accounts elsewhere on Windows: %v", err)
	}
	want := "Windows can't start this desk: the Claude accounts client1, client2, client3, client4, client5, client6 and client7 are kept " +
		"outside a ~/.claude-<name> folder, which makes its settings too long, so move them to ~/.claude-<name> folders or remove " +
		"some under Claude accounts in Settings, then hire again."
	if refusal.Message != want {
		t.Errorf("the refusal says %q", refusal.Message)
	}
	for _, system := range []string{"darwin", "linux"} {
		line, err := write(system, elsewhere)
		if err != nil {
			t.Fatalf("%s refused: %v", system, err)
		}
		if system == "linux" && quoted(line) <= exeLongest {
			t.Fatalf("the accounts don't push it over: %d", quoted(line))
		}
		t.Logf("%s, 8 accounts elsewhere: %d quoted", system, quoted(line))
	}
	line, err := write("windows", tidy)
	if err != nil {
		t.Fatalf("8 ~/.claude-<name> accounts on Windows: %v", err)
	}
	t.Logf("windows, 8 ~/.claude-<name> accounts: %d quoted", quoted(line))

	// One account elsewhere is named alone; settings too long with none elsewhere still say what to do.
	if err := settingsTooLong(strings.Repeat("x", exeLongest), elsewhere[:2], "windows"); err == nil ||
		!strings.HasSuffix(err.Error(), ": the Claude account client1 is kept outside a ~/.claude-<name> folder, which makes its settings too long, "+
			"so move it to one or remove it under Claude accounts in Settings, then hire again.") {
		t.Errorf("one account: %v", err)
	}
	if err := settingsTooLong(strings.Repeat("x", exeLongest), tidy, "windows"); err == nil || !strings.Contains(err.Error(), "turn off some guard rules") {
		t.Errorf("no account elsewhere: %v", err)
	}
}

// B4: a rule the owner adds in Settings reaches a desk already running at its next call (the file adds it); a
// group the owner switches off loosens that desk only at its next start, which takes the core's new copy.
func TestTheOwnersRulesReachARunningDeskAtOnceAndLooseningAtItsNextStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), hireGuardFile)
	t.Cleanup(func() { forgetHeldGuard(path) })
	running := guard.GroupsArgument(heldGuardGroups(path)) // what a desk started now has in --groups
	refuses := func(held, line string) bool {
		groups, problem := guard.Held(held, path, guard.ProfileHire)
		return guard.Refused(line, groups, problem) != ""
	}
	if refuses(running, "helm install x") || !refuses(running, "git push") {
		t.Fatal("the defaults aren't as expected")
	}
	if answer := saveGuard([]byte(`{"groups":[{"name":"push","on":false}],"extra":["helm install"]}`), path, "darwin"); answer.status != 200 {
		t.Fatalf("save: %+v", answer)
	}
	if !refuses(running, "helm install x") {
		t.Error("the owner's new rule didn't reach the running desk")
	}
	if !refuses(running, "git push") {
		t.Error("switching push off loosened the running desk")
	}
	next := guard.GroupsArgument(heldGuardGroups(path))
	if refuses(next, "git push") || !refuses(next, "helm install x") {
		t.Error("the next desk doesn't start with the owner's guard")
	}
}
