//go:build !windows

package desktop

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// SA-T1 through the engine, as a managed hire is really made: the start its ticket holds switches Claude Code's own
// sandbox off in its settings and the auto-update off in its environment, and the brief written in its folder ends
// with the tools' lines.
func TestAnEngineHireStartsWithEveryOwnSandboxOff(t *testing.T) {
	r := newEngineRig(t)
	request := r.request("tester")
	request.Role.Brief = &RoleBrief{Kind: "library", ID: "tester"}
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	script := r.script(hire)
	if settings := scriptSettings(script); !strings.Contains(settings, settingsSandboxOff) {
		t.Errorf("the hire's settings don't switch Claude Code's sandbox off: %s", settings)
	}
	if !strings.Contains(script, "export DISABLE_AUTOUPDATER='1'\n") {
		t.Errorf("the hire's start doesn't switch the auto-update off:\n%s", script)
	}
	data, err := os.ReadFile(filepath.Join(hireFolder(supportDir, hire.ID), "BRIEF.md"))
	if err != nil || !strings.HasSuffix(string(data), strings.TrimPrefix(briefSandboxTools(runtime.GOOS), "\n")) {
		t.Errorf("the hire's brief: %v\n%s", err, data)
	}
}

// Every desk, whatever its agent, the GM's and a test copy's included, starts with DISABLE_AUTOUPDATER=1, over
// whatever the terminal had, once, after the desk's own variables. HUSKY isn't Smolcorp's to set (RT126SAT1 B1): no
// plan names it, so the terminal's passes through as it was, and a terminal without one gives a desk without one.
func TestEveryDeskStartsWithTheAutoUpdaterOffAndHuskyAsTheTerminalHadIt(t *testing.T) {
	isolate(t)
	for name, start := range everyKindOfDeskStart() {
		plan, err := start.plan()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if n := len(plan.Changes); n < 1 || plan.Changes[n-1] != (envChange{Name: "DISABLE_AUTOUPDATER", Value: "1"}) {
			t.Errorf("%s: the plan doesn't end with the auto-update's switch:\n%s", name, planText(plan))
		}
		if slices.ContainsFunc(plan.Changes, func(c envChange) bool { return strings.EqualFold(c.Name, "HUSKY") }) {
			t.Errorf("%s: the plan sets HUSKY:\n%s", name, planText(plan))
		}
		for _, terminal := range [][]string{{"PATH=/usr/bin", "HUSKY=1", "DISABLE_AUTOUPDATER=0", "HOME=/h"}, {"PATH=/usr/bin", "HOME=/h"}} {
			env, err := deskEnvironment(terminal, plan, nil)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for _, variable := range []string{"HUSKY", "DISABLE_AUTOUPDATER"} {
				want := []string{}
				if variable == "DISABLE_AUTOUPDATER" {
					want = []string{"DISABLE_AUTOUPDATER=1"}
				} else if slices.Contains(terminal, "HUSKY=1") {
					want = []string{"HUSKY=1"}
				}
				if found := slices.DeleteFunc(slices.Clone(env), func(v string) bool { return !strings.HasPrefix(v, variable+"=") }); !slices.Equal(found, want) {
					t.Errorf("%s, terminal %q: the desk's %s is %q, want %q", name, terminal, variable, found, want)
				}
			}
		}
	}
}

// everyKindOfDeskStart is a start of each kind plan() makes: a hire, the GM, a test copy's hire, a resumed one, and
// each agent other than Claude.
func everyKindOfDeskStart() map[string]hireStart {
	starts := map[string]hireStart{
		"a hire":      {Role: testHireRole(), Message: "go", Ports: "4100-4103", SendKey: "k-1_a", DefaultAccount: true, Settings: "{}"},
		"the GM":      {Role: Role{ID: gmRole, Name: "General Manager", Agent: "claude"}, Message: "go", GMToken: testGMToken, GMPlugin: "/p", GMCLI: "/c"},
		"a test copy": {Role: testHireRole(), Message: "go", ConfigDir: "/a", TestPort: 7745, TestSupportDir: "/t"},
		"a resumed":   {Role: testHireRole(), Resume: "s-1"},
	}
	for _, agent := range []Agent{Codex, Gemini, OpenCode, Grok} {
		starts[string(agent)] = hireStart{Role: Role{ID: "x", Name: "X", Agent: string(agent)}, Message: "go", Ports: "4100-4103", SendKey: "k-2_b"}
	}
	return starts
}

// The reviewer's probe for RT126SAT1 B1, as a test: a commit made with a desk's environment still runs the project's
// husky hooks. The repo's hooks are a husky-9-shaped stand-in (core.hooksPath=.husky/_, whose pre-commit sources _/h,
// which leaves at once when HUSKY=0 and otherwise runs .husky/pre-commit); its hook only writes a line to a marker
// outside the repo. The control commits with HUSKY=0 added and must find no line, so the stand-in really reads HUSKY.
func TestADesksCommitStillRunsTheProjectsHuskyHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	isolate(t)
	scratch := t.TempDir()
	repo, marker := filepath.Join(scratch, "repo"), filepath.Join(scratch, "ran")
	files := map[string]string{
		".husky/_/pre-commit": "#!/bin/sh\n. \"$(dirname \"$0\")/h\"\n",
		".husky/_/h": "#!/bin/sh\n" +
			"n=$(basename \"$0\")\n" +
			"s=$(dirname \"$(dirname \"$0\")\")/$n\n" +
			"[ ! -f \"$s\" ] && exit 0\n" +
			"[ \"${HUSKY-}\" = \"0\" ] && exit 0\n" +
			"sh -e \"$s\" \"$@\"\n",
		".husky/pre-commit": "echo \"$SMOLCORP_TEST_HOOK_NAME\" >> '" + marker + "'\n",
	}
	for name, text := range files {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	terminal := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + scratch, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + filepath.Join(scratch, "gitconfig")}
	git := func(env []string, args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		command.Dir, command.Env = repo, env
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(terminal, "init", "-q")
	git(terminal, "config", "core.hooksPath", ".husky/_")
	ran := func() []string {
		data, _ := os.ReadFile(marker)
		return strings.Fields(string(data))
	}
	// The control: with HUSKY=0, husky's runner skips the hook.
	git(append(slices.Clone(terminal), "SMOLCORP_TEST_HOOK_NAME=control", "HUSKY=0"), "commit", "-q", "--allow-empty", "-m", "control")
	if got := ran(); len(got) != 0 {
		t.Fatalf("the stand-in ran its hook with HUSKY=0 (%q), so this test proves nothing", got)
	}
	starts := everyKindOfDeskStart()
	names := slices.Sorted(maps.Keys(starts))
	for _, name := range names {
		plan, err := starts[name].plan()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		env, err := deskEnvironment(terminal, plan, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		git(append(env, "SMOLCORP_TEST_HOOK_NAME="+strings.ReplaceAll(name, " ", "-")), "commit", "-q", "--allow-empty", "-m", name)
	}
	want := make([]string, 0, len(names))
	for _, name := range names {
		want = append(want, strings.ReplaceAll(name, " ", "-"))
	}
	if got := ran(); !slices.Equal(got, want) {
		t.Errorf("the project's pre-commit ran for %q, want every desk: %q", got, want)
	}
}
