package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	// Run by the go pinned at the script's start, with its own settings only (SA-X11).
	build := regexp.MustCompile(`(?m)^build\(\) \{[^\n]*\n\s*pinned_env CGO_ENABLED=0 GOOS="\$1" GOARCH="\$2" "\$GO" build `)
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
	}
	for _, line := range goBuildsWithCgo(string(script)) {
		t.Errorf("a go build without CGO_ENABLED=0: %s", line)
	}
	if linux == 0 {
		t.Error("no Linux build through build()")
	}
}

// goBuild is a go build by the pinned go ("$GO", $GO, "${GO}", pinned_go) or by name.
var goBuild = regexp.MustCompile(`(^|[^A-Za-z_])("?\$\{?GO\}?"?|go|pinned_go)\s+build\b`)

// goBuildsWithCgo is the script's lines, outside comments, with a go build and no CGO_ENABLED=0.
func goBuildsWithCgo(script string) []string {
	var found []string
	for _, line := range strings.Split(script, "\n") {
		if goBuild.MatchString(line) && !strings.Contains(line, "CGO_ENABLED=0") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			found = append(found, line)
		}
	}
	return found
}

// RMSYSAX11 B1: the catch-all sees a go build by the pinned go as well as by name, and only a go build.
func TestTheLinuxBuildsCatchAllSeesThePinnedGo(t *testing.T) {
	for line, caught := range map[string]bool{
		// The reviewer's probes: the base's line by name (m5) and the pinned one (m4).
		`build_plain() { GOOS="$1" GOARCH="$2" go build -o "$3" ./cmd/smolcorp; }`:               true,
		`build_plain() { pinned_env GOOS="$1" GOARCH="$2" "$GO" build -o "$3" ./cmd/smolcorp; }`: true,
		`  pinned_env GOOS=linux $GO build -o out ./cmd/smolcorp`:                                true,
		`  pinned_env GOOS=linux "${GO}" build -o out ./cmd/smolcorp`:                            true,
		`  /usr/bin/env -i "${go_env[@]}" GOOS=linux "$GO"  build ./cmd/smolcorp`:                true,
		`go build ./cmd/smolcorp`:                 true,
		`  pinned_go build -o out ./cmd/smolcorp`: true,
		`  pinned_env CGO_ENABLED=0 GOOS="$1" GOARCH="$2" "$GO" build -trimpath -buildvcs=false \`: false,
		`# a comment on go build or "$GO" build`:                                                   false,
		`  pinned_go vet ./...`:                                                                    false,
		`  cargo build --release`:                                                                  false,
		`  echo "$GOOS" build`:                                                                     false,
		`  "$GO" test -run '^TestBuild' ./desktop`:                                                 false,
	} {
		if got := len(goBuildsWithCgo(line)) > 0; got != caught {
			t.Errorf("%s: caught %v, want %v", line, got, caught)
		}
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

// SIERRA's hard row (MSYSAX8), kept under SETTINGSFILE1: the command line Windows is given for the worst-case desk, as
// the launcher builds it (launcher.go: the agent, --settings=<its file>, the role's model and effort, then its first
// message, each quoted as Go quotes it there, exeLine), stays under 30,000 of Windows's 32,767. The settings ride in
// their file now, so no rule counts here. The worst case is RMSYSAX8 F1's, in full and in Windows's own lengths, so
// the figure doesn't hang on this test's temp folder: a 32-letter user, the support folder in %APPDATA%; the GM's
// plugin folder, its agent and its introduction; a briefed first message that asks its questions; and, in the file,
// every default guard group, setup's 8 accounts, a real support folder's 20 entries and 200 past hires' folders, the
// support folder in the home folder and not. The line is the same with them as without: it doesn't grow with hires.
func TestTheWorstCaseDesksWindowsCommandLineStaysUnder30000(t *testing.T) {
	const most = 30000
	r := newHireSettingsRig(t)
	saved := homeDir
	t.Cleanup(func() { homeDir = saved })
	groups := func(string, []string) (int, string, bool) {
		return 0, `{"groups":` + strings.TrimPrefix(guard.GroupsArgument(guard.DefaultGroups()), `{"groups":`), true
	}
	user := `C:\Users\` + strings.Repeat("n", 32)
	program, support := user+`\.local\bin\claude.exe`, user+`\AppData\Roaming\Smolcorp`
	role := Role{ID: gmRole, Name: "General Manager of the whole office", Agent: "claude", Model: "claude-opus-5-5[1m]", Effort: "max"}
	const id = "abc123def456"
	line := func(settingsPath string, gm bool) string {
		message := role.briefMessage(support+`\hires\`+id+`\BRIEF-`+strings.Repeat("b", 24)+".md", true)
		args := []string{"--settings=" + settingsPath}
		if gm {
			args = append(args, "--plugin-dir="+user+`\AppData\Local\Programs\Smolcorp\gm\plugin`, "--agent="+gmPluginName+":"+gmPluginAgent)
			message += "\n\n" + gmIntroduction(user+`\Projects\`+strings.Repeat("p", 40), false)
		}
		return exeLine(program, append(args, role.arguments(message)...))
	}
	worst, worstAt := 0, ""
	for _, past := range []int{0, 200} {
		for i := len(r.pastHires); i < past; i++ {
			r.pastHires = append(r.pastHires, fmt.Sprintf("%012x", 0xa00000000000+i))
			if err := os.MkdirAll(filepath.Join(r.support, "hires", r.pastHires[i]), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		for _, entry := range []string{"sessions.json", "events.jsonl", "events.1.jsonl", "grants.jsonl", "saved.json", "hire-queue.json",
			"outbox.json", "registry.json", "config.json", "phones.json", "checkpoints", "photos", "recaps", "briefs", "acp-transcripts",
			"hook-key", "setup-key", "window-key", "instance.lock", "smolcorp.pid"} {
			if past > 0 {
				_ = os.WriteFile(filepath.Join(r.support, entry), nil, 0o600)
			}
		}
		for _, inHome := range []bool{false, true} {
			homeDir = filepath.Join(r.root, "home")
			if inHome {
				homeDir = filepath.Dir(r.support) // the support folder in the home folder: both forms of the rules
			}
			accounts := []claudeProfile{{Name: "default", Dir: filepath.Join(homeDir, ".claude")}}
			for i := 1; i < claudeAccountsMost; i++ {
				accounts = append(accounts, claudeProfile{Name: "client" + strconv.Itoa(i), Dir: filepath.Join(homeDir, ".claude-client-"+strconv.Itoa(i)+"-northwind")})
			}
			for _, profile := range []guardProfile{guardHire, guardGM} {
				written, err := r.writeFile(id, func(o *hireSettingsOptions) {
					o.System, o.Profiles, o.Run, o.Profile, o.OwnClaude = "windows", accounts, groups, profile, true
				})
				if err != nil {
					t.Fatalf("%d past hires, in home %v, %v: %v", past, inHome, profile, err)
				}
				// Every rule is in the file, the last past hire's and the GM's memory switch among them; none is in the line.
				text := fileSettings(written.Path)
				if past > 0 && !strings.Contains(text, "/hires/"+r.pastHires[past-1]+"/**)") || profile == guardGM && !strings.Contains(text, `"autoMemoryEnabled":false`) {
					t.Errorf("%d past hires, %v: the file lacks rules (%d characters)", past, profile, len(text))
				}
				at := fmt.Sprintf("%d past hires, support in home %v, %v, the file %d characters", past, inHome, profile, len(text))
				n := len(line(support+`\hires\`+id+`\`+hireSettingsFile, profile == guardGM))
				t.Logf("%s: the Windows command line is %d characters", at, n)
				if n > worst {
					worst, worstAt = n, at
				}
			}
		}
	}
	t.Logf("worst case: %d characters (%s)", worst, worstAt)
	if worst >= most || worst != len(line(support+`\hires\`+id+`\`+hireSettingsFile, true)) {
		t.Errorf("the worst-case desk's Windows command line is %d characters (%s), not under %d, or it grows with the rules", worst, worstAt, most)
	}
}

// RT126SAX8R2 R2-a and R2-b: with the support folder in the home folder, as on every real Mac and Linux install, every
// desk's Claude folder is denied from ~ and, beside it, in full (a desk whose HOME isn't the owner's still meets it):
// a later desk's, the GM's and the reader's own, for a hire, the GM and a desk without its own folder.
func TestEveryDesksClaudeFolderIsDeniedWithTheSupportFolderInTheHomeFolder(t *testing.T) {
	r := newHireSettingsRig(t)
	saved := homeDir
	homeDir = filepath.Dir(r.support)
	t.Cleanup(func() { homeDir = saved })
	for _, reader := range []struct {
		id      string
		profile guardProfile
		own     bool
	}{{"hirea", guardHire, true}, {"gm0000000001", guardGM, true}, {"l1", guardHire, false}} {
		line, err := r.write(reader.id, func(o *hireSettingsOptions) { o.Profile, o.OwnClaude = reader.profile, reader.own })
		if err != nil {
			t.Fatal(err)
		}
		var settings struct {
			Permissions struct {
				Deny []string `json:"deny"`
			} `json:"permissions"`
		}
		if err := json.Unmarshal([]byte(line), &settings); err != nil {
			t.Fatal(err)
		}
		deny := settings.Permissions.Deny
		short := "~/" + filepath.Base(r.support) + "/hires/*/claude/**"
		full := absoluteRulePath(r.support) + "/hires/*/claude/**"
		for _, rule := range []string{"Read(" + short + ")", "Edit(" + short + ")", "Read(" + full + ")", "Edit(" + full + ")"} {
			if !slices.Contains(deny, rule) {
				t.Errorf("%s: no %s", reader.id, rule)
			}
		}
		var edits []string
		for _, rule := range deny {
			if pattern, ok := strings.CutPrefix(rule, "Edit("); ok {
				edits = append(edits, "Read("+pattern)
			}
		}
		for _, desk := range []string{"hireb", "gm0000000002", reader.id} {
			for _, store := range []string{"history.jsonl", "projects/-w/s1.jsonl", ".claude.json", "todos/t.json"} {
				at := filepath.Join(deskClaudeFolder(r.support, desk), filepath.FromSlash(store))
				if !readDenied(deny, homeDir, at) || !readDenied(edits, homeDir, at) {
					t.Errorf("%s: %s isn't denied to its Read and Edit", reader.id, at)
				}
			}
		}
	}
}

// RT126CMDLEN B1, under SETTINGSFILE1: each Claude account kept outside ~/.claude* adds its own deny rules, about
// 2,000 characters, and setup's 8 accounts in folders elsewhere once made settings Windows couldn't start a desk with,
// so that desk was refused. The rules now ride in the desk's file: the same desk starts on Windows, its file holds
// every account's rules, and its command line names the file alone.
func TestAWindowsDeskWithAccountsKeptElsewhereStartsWithItsRulesInItsFile(t *testing.T) {
	r := newHireSettingsRig(t)
	saved := homeDir
	homeDir = filepath.Join(r.root, "home") // a stand-in home: nothing is made in it, the rules only name it
	t.Cleanup(func() { homeDir = saved })
	elsewhere := []claudeProfile{{Name: "default", Dir: filepath.Join(homeDir, ".claude")}}
	for i := 1; i <= 7; i++ {
		elsewhere = append(elsewhere, claudeProfile{Name: "client" + strconv.Itoa(i), Dir: filepath.Join(homeDir, "Documents", "Claude accounts", "client-"+strconv.Itoa(i)+"-northwind")})
	}
	for _, profile := range []guardProfile{guardHire, guardGM} {
		written, err := r.writeFile("abc123", func(o *hireSettingsOptions) {
			o.System, o.Profiles, o.Profile = "windows", elsewhere, profile
			o.Run = func(string, []string) (int, string, bool) {
				return 0, `{"groups":` + strings.TrimPrefix(guard.GroupsArgument(guard.DefaultGroups()), `{"groups":`), true
			}
		})
		if err != nil {
			t.Fatalf("%v, 8 accounts elsewhere on Windows: %v", profile, err)
		}
		text := fileSettings(written.Path)
		for _, account := range elsewhere {
			if !strings.Contains(text, absoluteRulePath(account.Dir)+"/projects/**") {
				t.Errorf("%v: the file lacks %s's rules", profile, account.Name)
			}
		}
		if quoted := len(exeLine("--settings="+text, nil)); quoted <= exeLongest {
			t.Errorf("%v: the accounts don't push the settings past Windows's line (%d), so this row proves nothing", profile, quoted)
		}
		if n := len(exeLine("--settings="+written.Path, nil)); n > 300 {
			t.Errorf("%v: the line naming the file is %d characters", profile, n)
		}
		t.Logf("%v, 8 accounts elsewhere: the file %d characters, %d quoted as a line once was", profile, len(text), len(exeLine("--settings="+text, nil)))
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
