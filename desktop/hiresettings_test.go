package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The Go port of HireSettingsTests.swift: temporary support, worktree and Claude folders only,
// and a stand-in guard that prints the groups; nothing real is run.

const hireGuardPrinted = `{"groups":[{"name":"push","on":true,"commands":["git push|send-pack"]},
           {"name":"deploy","on":true,"commands":["vercel","eas build|submit","fly","docker buildx build|bake --push"]},
           {"name":"cloud","on":false,"commands":["aws"]},
           {"name":"remote","on":true,"commands":["rsync @remote","tmux ... send-keys"]}],
 "file":"x","problem":""}`

type hireSettingsRig struct {
	t       *testing.T
	root    string
	support string
	cli     string
	work    claudeProfile
	asked   [][]string
}

func newHireSettingsRig(t *testing.T) *hireSettingsRig {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	support := filepath.Join(root, "support")
	for _, dir := range []string{"hires/other11", "events"} {
		if err := os.MkdirAll(filepath.Join(support, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &hireSettingsRig{t: t, root: root, support: support, cli: filepath.Join(root, "bin", "smolcorp"),
		work: claudeProfile{Name: "work", Dir: filepath.Join(root, "claude-work")}}
}

func (r *hireSettingsRig) write(id string, mutate func(*hireSettingsOptions)) (string, error) {
	o := hireSettingsOptions{HireID: id, Auto: true, CLI: r.cli, Support: r.support, Profiles: []claudeProfile{r.work},
		Run: func(tool string, arguments []string) (int, string, bool) {
			r.asked = append(r.asked, append([]string{tool}, arguments...))
			return 0, hireGuardPrinted, true
		},
		IsExec: func(string) bool { return true }}
	if mutate != nil {
		mutate(&o)
	}
	return writeHireSettings(o)
}

func (r *hireSettingsRig) json(path string) map[string]any {
	r.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		r.t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		r.t.Fatal(err)
	}
	return object
}

func (r *hireSettingsRig) deny(path string) []string {
	r.t.Helper()
	var out []string
	for _, rule := range r.json(path)["permissions"].(map[string]any)["deny"].([]any) {
		out = append(out, rule.(string))
	}
	return out
}

func (r *hireSettingsRig) hook(path string) string {
	r.t.Helper()
	hooks := r.json(path)["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(hooks) != 1 || hooks[0].(map[string]any)["matcher"] != "*" {
		r.t.Fatalf("hooks = %v", hooks)
	}
	return hooks[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
}

// hireGlob is just enough of gitignore's globbing for the rules: ** crosses folders, * doesn't.
func hireGlob(pattern, path string) bool {
	var re strings.Builder
	re.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch {
		case pattern[i] == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			re.WriteString(".*")
			i++
		case pattern[i] == '*':
			re.WriteString("[^/]*")
		default:
			re.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	return regexp.MustCompile(re.String() + "$").MatchString(path)
}

// hireDenied: whether a rule for tool covers the absolute path, as the Mac's test reads the rules.
func hireDenied(deny []string, path, tool string) bool {
	path = filepath.ToSlash(path)
	if tool == "Write" { // Claude Code checks Edit rules for every tool that changes a file
		tool = "Edit"
	}
	for _, rule := range deny {
		if !strings.HasPrefix(rule, tool+"(") || !strings.HasSuffix(rule, ")") {
			continue
		}
		pattern := strings.TrimSuffix(strings.TrimPrefix(rule, tool+"("), ")")
		if strings.HasPrefix(pattern, "//") && hireGlob(pattern[1:], path) {
			return true
		}
	}
	return false
}

func TestHireSettingsHoldTheDenyRulesTheGuardAndAutoMode(t *testing.T) {
	r := newHireSettingsRig(t)
	path, err := r.write("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(r.support, "hires", "abc123", "settings.json"); path != want {
		t.Fatalf("path = %s, want %s", path, want)
	}
	settings := r.json(path)
	if settings["permissions"].(map[string]any)["defaultMode"] != "auto" {
		t.Fatal("auto mode missing")
	}
	deny := r.deny(path)
	for _, rule := range []string{"Bash(git push:*)", "Bash(git send-pack:*)", "Bash(vercel:*)", "Bash(npx vercel:*)", "Bash(eas build:*)", "Bash(eas submit:*)", "Bash(fly:*)"} {
		if !slices.Contains(deny, rule) {
			t.Errorf("missing %s", rule)
		}
	}
	for _, rule := range []string{"Bash(aws:*)", "Bash(rsync:*)", "Bash(tmux:*)", "Bash(docker buildx build:*)"} {
		if slices.Contains(deny, rule) {
			t.Errorf("unexpected %s", rule)
		}
	}
	work, base := "/"+strings.TrimPrefix(filepath.ToSlash(r.work.Dir), "/"), "/"+strings.TrimPrefix(filepath.ToSlash(r.support), "/")
	for _, rule := range []string{"Read(**/.env*)", "Edit(**/.env*)", "Edit(//**/.env*)", "Edit(~/.claude*/settings*.json)", "Edit(**/.claude/settings*.json)",
		"Read(~/.claude*/projects/**)", "Edit(/" + work + "/settings*.json)", "Read(/" + work + "/projects/**)",
		"Edit(/" + base + "/*.json)", "Read(/" + base + "/gm-token)", "Edit(/" + base + "/gm-token)",
		"Edit(/" + base + "/hires/*/*.json)", "Edit(/" + base + "/hires/other11/**)", "Edit(/" + base + "/events/**)"} {
		if !slices.Contains(deny, rule) {
			t.Errorf("missing %s", rule)
		}
	}
	// Edit rules cover Write too; a Write(path) rule never matches and makes Claude Code warn at every hire start.
	for _, rule := range deny {
		if strings.HasPrefix(rule, "Write(") {
			t.Errorf("Write rule %s", rule)
		}
	}
	// The guard runs before every tool, reading the app's hire-guard.json.
	if got, want := r.hook(path), shellPath(r.cli)+" guard --rules "+shellPath(filepath.Join(r.support, "hire-guard.json")); got != want {
		t.Errorf("hook = %s, want %s", got, want)
	}
	if v, ok := settings["disableAllHooks"].(bool); !ok || v {
		t.Errorf("disableAllHooks = %v, want false", settings["disableAllHooks"])
	}
}

// The exact rule list, order and forms, on a fixed layout (Linux paths).
func TestHireDenyRulesAreInTheMacsOrderAndForms(t *testing.T) {
	groups := []guardGroup{{Name: "push", On: true, Commands: []string{"git push|send-pack"}}, {Name: "cloud", On: false, Commands: []string{"aws"}}}
	got := hireDenyRules("abc123", groups, "/s", []claudeProfile{{Name: "work", Dir: "/c/work/"}, {Name: "bad", Dir: "rel"}},
		[]string{"abc123", "other11", ".hid"}, []string{"events", "hires", ".x", "briefs"})
	var want []string
	both := func(p string) { want = append(want, "Edit("+p+")") }
	want = append(want, "Bash(git push:*)", "Bash(npx git push:*)", "Bash(git send-pack:*)", "Bash(npx git send-pack:*)",
		"Read(**/.env*)", "Read(//**/.env*)")
	for _, p := range []string{"**/.env*", "//**/.env*", "~/.claude*/settings*.json", "~/.claude*.json", "**/.claude/settings*.json", "//**/.claude/settings*.json"} {
		both(p)
	}
	want = append(want, "Read(~/.claude*/projects/**)")
	both("//c/work/settings*.json")
	want = append(want, "Read(//c/work/projects/**)")
	for _, p := range []string{"*.json", "*.sh", "*.md", "*.log", "*.jsonl", "gm-token", "setup-key", "hook-header", "hook-key", "window-key", "token"} {
		both("//s/" + p)
	}
	both("//s/events/**")
	both("//s/briefs/**")
	both("//s/recaps/**")
	both("//s/hires/*.sh")
	for _, p := range []string{"*.json", "*.sh", "*.jsonl", "*.log", "settings*"} {
		both("//s/hires/*/" + p)
	}
	both("//s/hires/other11/**")
	for _, n := range []string{"gm-token", "setup-key", "hook-header", "hook-key", "window-key", "token", "hire-guard.json", "phones.json", "sessions.json", "config.json", "registry.json"} {
		want = append(want, "Read(//s/"+n+")")
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rules differ:\n got %q\nwant %q", got, want)
	}
}

func TestHireGuardHookUsesTheGMProfileAndAHiresDoesNot(t *testing.T) {
	r := newHireSettingsRig(t)
	rules := filepath.Join(r.support, "hire-guard.json")
	gm, err := r.write("gm0001", func(o *hireSettingsOptions) { o.Profile = guardGM })
	if err != nil {
		t.Fatal(err)
	}
	if got, want := r.hook(gm), shellPath(r.cli)+" guard --profile gm --rules "+shellPath(rules); got != want {
		t.Errorf("gm hook = %s, want %s", got, want)
	}
	if want := []string{r.cli, "guard", "--print", "--profile", "gm", "--rules", rules}; !slices.Equal(r.asked[0], want) {
		t.Errorf("asked %q, want %q", r.asked[0], want)
	}
	hire, err := r.write("hire01", nil)
	if err != nil {
		t.Fatal(err)
	}
	if hook := r.hook(hire); strings.Contains(hook, "--profile") {
		t.Errorf("a hire's hook has a profile: %s", hook)
	}
}

// The GM keeps a hire's deny rules, and may read its team's STATUS files (no rule denies reading them).
func TestHireGMKeepsTheDenyRulesAndMayReadTheTeamsStatusFiles(t *testing.T) {
	r := newHireSettingsRig(t)
	path, err := r.write("gm0001", func(o *hireSettingsOptions) { o.Profile = guardGM })
	if err != nil {
		t.Fatal(err)
	}
	deny := r.deny(path)
	root := "/" + strings.TrimPrefix(filepath.ToSlash(r.support), "/")
	for _, rule := range []string{"Edit(/" + root + "/*.json)", "Edit(~/.claude*/settings*.json)", "Read(~/.claude*/projects/**)",
		"Read(/" + root + "/hire-guard.json)", "Edit(/" + root + "/hires/other11/**)", "Edit(/" + root + "/hires/*/settings*)",
		"Bash(git push:*)", "Bash(vercel:*)"} {
		if !slices.Contains(deny, rule) {
			t.Errorf("missing %s", rule)
		}
	}
	for _, rule := range deny {
		if strings.HasPrefix(rule, "Read(") && strings.Contains(rule, "/hires/") || strings.Contains(rule, "STATUS") {
			t.Errorf("reading the team's STATUS.md files must not be denied: %s", rule)
		}
	}
}

func TestHireSettingsLeaveAutoModeOutWhenNotWanted(t *testing.T) {
	r := newHireSettingsRig(t)
	path, err := r.write("abc123", func(o *hireSettingsOptions) { o.Auto = false })
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.json(path)["permissions"].(map[string]any)["defaultMode"]; ok {
		t.Fatal("defaultMode present")
	}
}

func TestHireSettingsFileIsPrivateAndReplacedWhole(t *testing.T) {
	r := newHireSettingsRig(t)
	path, err := r.write("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		file, _ := os.Stat(path)
		folder, _ := os.Stat(filepath.Dir(path))
		if file.Mode().Perm() != 0o600 || folder.Mode().Perm() != 0o700 {
			t.Errorf("modes: file %v, folder %v", file.Mode().Perm(), folder.Mode().Perm())
		}
	}
	if _, err := r.write("abc123", func(o *hireSettingsOptions) { o.Auto = false }); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.json(path)["permissions"].(map[string]any)["defaultMode"]; ok {
		t.Fatal("not replaced whole")
	}
}

// A deny rule beats any allow, so no rule may cover the hire's own STATUS.md, while the rest of the app's folder is covered.
func TestHireOwnStatusFileIsNotDeniedButItsSettingsAndTheRestAre(t *testing.T) {
	r := newHireSettingsRig(t)
	path, _ := r.write("abc123", nil)
	deny := r.deny(path)
	base := r.support
	for _, tool := range []string{"Edit", "Write"} {
		if hireDenied(deny, base+"/hires/abc123/STATUS.md", tool) {
			t.Errorf("%s: its own status file must stay writable", tool)
		}
		for _, p := range []string{"hires/abc123/settings.json", "hire-guard.json", "gm-token", "frontdesk.json", "events/outbox.log", "hires/other11/STATUS.md", "hires/hire-1234.sh",
			"hires/late99/settings.json", "hires/late99/settings.local.json", "hires/late99/run.sh"} {
			if !hireDenied(deny, base+"/"+p, tool) {
				t.Errorf("%s %s not denied", tool, p)
			}
		}
		if hireDenied(deny, base+"/hires/late99/STATUS.md", tool) {
			t.Errorf("%s: a later hire's status file is covered", tool)
		}
	}
	for _, name := range []string{"gm-token", "setup-key", "hook-header", "hook-key", "window-key", "token", "hire-guard.json", "phones.json", "sessions.json", "config.json", "registry.json"} {
		if !hireDenied(deny, base+"/"+name, "Read") {
			t.Errorf("%s can be read", name)
		}
	}
	if !hireDenied(deny, r.work.Dir+"/projects/x/session.jsonl", "Read") || !hireDenied(deny, r.work.Dir+"/settings.json", "Edit") {
		t.Error("the Claude account's transcripts or settings are open")
	}
}

func TestHireKnownFoldersAreDeniedEvenWhenAbsent(t *testing.T) {
	r := newHireSettingsRig(t)
	path, _ := r.write("abc123", nil)
	root := "/" + strings.TrimPrefix(filepath.ToSlash(r.support), "/")
	for _, name := range hireKnownFolders {
		if _, err := os.Stat(filepath.Join(r.support, name)); err == nil {
			t.Fatalf("%s exists", name)
		}
		if !slices.Contains(r.deny(path), "Edit(/"+root+"/"+name+"/**)") {
			t.Errorf("%s not denied", name)
		}
	}
}

func TestHireNoSmolcorpCommandMeansNoHire(t *testing.T) {
	r := newHireSettingsRig(t)
	_, err := r.write("abc123", func(o *hireSettingsOptions) { o.IsExec = func(string) bool { return false } })
	refusal, ok := err.(*hireRefusal)
	if !ok || refusal.Message != errGuardMissing.Message || !strings.Contains(refusal.Message, "unguarded") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.support, "hires", "abc123")); err == nil {
		t.Fatal("something was left behind")
	}
}

func TestHireAGuardThatCantBeRunMeansNoHire(t *testing.T) {
	isolate(t)
	r := newHireSettingsRig(t)
	runs := map[string]guardRunner{
		"not json":     func(string, []string) (int, string, bool) { return 0, "not json", true },
		"no groups":    func(string, []string) (int, string, bool) { return 0, `{"problem":"x"}`, true },
		"not run":      func(string, []string) (int, string, bool) { return 0, "", false },
		"bad status":   func(string, []string) (int, string, bool) { return 1, hireGuardPrinted, true },
		"null groups":  func(string, []string) (int, string, bool) { return 0, `{"groups":null}`, true },
		"wrong shapes": func(string, []string) (int, string, bool) { return 0, `{"groups":[1]}`, true },
		"no on field": func(string, []string) (int, string, bool) {
			return 0, `{"groups":[{"name":"push","on":true,"commands":["git push"]},{"name":"cloud","commands":["aws"]}]}`, true
		},
	}
	for name, run := range runs {
		if _, err := r.write("abc123", func(o *hireSettingsOptions) { o.Run = run }); err == nil {
			t.Errorf("%s: no refusal", name)
		}
	}
	if _, err := os.Stat(filepath.Join(r.support, "hires", "abc123", "settings.json")); err == nil {
		t.Fatal("a file was written")
	}
}

func TestHireGuardIsAskedAboutTheAppsOwnRulesFile(t *testing.T) {
	r := newHireSettingsRig(t)
	if _, err := r.write("abc123", nil); err != nil {
		t.Fatal(err)
	}
	if want := []string{r.cli, "guard", "--print", "--rules", filepath.Join(r.support, "hire-guard.json")}; !slices.Equal(r.asked[0], want) {
		t.Fatalf("asked %q, want %q", r.asked[0], want)
	}
}

// A hire's name is one folder name: nothing that reaches out of hires/.
func TestHireSettingsRefuseANameThatIsNotAFolderName(t *testing.T) {
	r := newHireSettingsRig(t)
	for _, id := range []string{"", "..", "../x", "a/b", `a\b`, ".hid"} {
		if _, err := r.write(id, nil); err == nil {
			t.Errorf("%q accepted", id)
		}
	}
}

// The real runner, with a fake guard program in a temp folder: its status and output come back, and a missing one is "not run".
func TestHireGuardRunnerRunsAFakeProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script stands in for the guard")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "smolcorp")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho '{\"groups\":[{\"name\":\"push\",\"on\":true,\"commands\":[\"git push\"]}]}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	path, err := writeHireSettings(hireSettingsOptions{HireID: "abc123", CLI: fake, Support: filepath.Join(dir, "support")})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains((&hireSettingsRig{t: t}).deny(path), "Bash(git push:*)") {
		t.Fatal("rule missing")
	}
	if _, _, ok := runGuardTool(filepath.Join(dir, "missing"), nil); ok {
		t.Fatal("a missing program reported as run")
	}
	if _, err := writeHireSettings(hireSettingsOptions{HireID: "abc123", CLI: filepath.Join(dir, "missing"), Support: dir}); err != errGuardMissing {
		t.Fatalf("err = %v", err)
	}
}

// Off means allowed (m6): a group turned off gives no deny rules, and back on gives them again.
func TestHireGroupTurnedOffDropsItsDenyRules(t *testing.T) {
	groups := func(deployOn bool) []guardGroup {
		return []guardGroup{{Name: "push", On: true, Commands: []string{"git push"}}, {Name: "deploy", On: deployOn, Commands: []string{"vercel", "eas build|submit"}}}
	}
	on := hireCommandRules(groups(true))
	if !slices.Contains(on, "Bash(vercel:*)") || !slices.Contains(on, "Bash(npx eas submit:*)") {
		t.Fatalf("deploy on: its rules are missing: %q", on)
	}
	if off, want := hireCommandRules(groups(false)), []string{"Bash(git push:*)", "Bash(npx git push:*)"}; !slices.Equal(off, want) {
		t.Fatalf("deploy off: got %q, want only the push rules %q", off, want)
	}
	if !slices.Equal(hireCommandRules(groups(true)), on) {
		t.Fatalf("deploy back on: its rules did not come back")
	}
}

func TestATestCopysHireSettingsReportToTheCopysPortAfterTheGuard(t *testing.T) {
	isolate(t)
	r := newHireSettingsRig(t)
	path, err := r.write("abc123", func(o *hireSettingsOptions) { o.ReportPort = 7799 })
	if err != nil {
		t.Fatal(err)
	}
	object := r.json(path)
	hooks, _ := object["hooks"].(map[string]any)
	events := entriesFor(Claude).events
	if len(hooks) != len(events) {
		t.Fatalf("hook events = %d, want %d: %v", len(hooks), len(events), hooks)
	}
	for _, event := range events {
		groups, _ := hooks[event].([]any)
		if len(groups) == 0 {
			t.Fatalf("no hook for %s", event)
		}
		text, _ := json.Marshal(groups[len(groups)-1])
		if !strings.Contains(string(text), " hook --port 7799") || strings.Contains(string(text), "7710") {
			t.Errorf("%s reports to %s", event, text)
		}
	}
	// The guard stays first on PreToolUse, and the file still keeps hooks on.
	pre, _ := hooks["PreToolUse"].([]any)
	first, _ := json.Marshal(pre[0])
	if len(pre) != 2 || !strings.Contains(string(first), " guard ") {
		t.Errorf("PreToolUse = %d groups, first %s", len(pre), first)
	}
	if object["disableAllHooks"] != false {
		t.Error("hooks were turned off")
	}
}

func TestARealInstallsHireSettingsHoldTheGuardAlone(t *testing.T) {
	isolate(t)
	r := newHireSettingsRig(t)
	path, err := r.write("abc123", nil)
	if err != nil {
		t.Fatal(err)
	}
	hooks, _ := r.json(path)["hooks"].(map[string]any)
	if len(hooks) != 1 || hooks["PreToolUse"] == nil || len(hooks["PreToolUse"].([]any)) != 1 {
		t.Errorf("a real install's hire has hooks of its own: %v", hooks)
	}
}
