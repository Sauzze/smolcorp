//go:build !windows

package desktop

import (
	"slices"
	"strings"
	"testing"
)

// gmMemoryStartLines are the GM's start's lines for Claude Code's own memory, as planText writes them (FXMEM4bR1 B1,
// SIERRA's musts): the variable set to 1, and the three launch modes that keep the settings' env out taken out.
var gmMemoryStartLines = []string{
	"export CLAUDE_CODE_DISABLE_AUTO_MEMORY='1'\n",
	"unset CLAUDE_CODE_REMOTE_TOOLS_PIN_STORED_LOGIN\n",
	"unset CLAUDE_CODE_ENTRYPOINT\n",
	"unset CLAUDE_CODE_ENVIRONMENT_KIND\n",
}

// claudeLaunchModes are the launch modes in which Claude Code (2.1.314 and 2.1.325) keeps a settings file's env from
// applying (claudeAutoMemoryOn), each as the variable that turns it on, and a plain terminal first.
var claudeLaunchModes = [][]string{
	nil,
	{"CLAUDE_CODE_ENTRYPOINT=claude-desktop"},
	{"CLAUDE_CODE_ENTRYPOINT=claude-desktop-3p"},
	{"CLAUDE_CODE_ENTRYPOINT=local-agent"},
	{"CLAUDE_CODE_ENVIRONMENT_KIND=byoc"},
	{"CLAUDE_CODE_REMOTE_TOOLS_PIN_STORED_LOGIN=1"},
}

// environmentMap is env, NAME=value entries, as a map.
func environmentMap(env []string) map[string]string {
	variables := map[string]string{}
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		variables[name] = value
	}
	return variables
}

// RT126MEM4b B1 and SIERRA's musts, through the engine's plan: Smolcorp started with CLAUDE_CODE_DISABLE_AUTO_MEMORY=0,
// alone or with any of the launch modes that keep the settings' env out, still starts the GM with 1 and none of the
// modes, so its memory is off (and the account's env saying 0 too). Without the start's lines the same terminal
// switches it on in every mode (the rows are live); each half alone, the 1 or the unsets, holds; a hire's start
// passes the terminal's on as it was.
func TestTheGMsStartKeepsItsMemoryOffWhateverSmolcorpInherited(t *testing.T) {
	r := newHireSettingsRig(t)
	settings := func(profile guardProfile) (string, map[string]any) {
		line, err := r.write("abc123", func(o *hireSettingsOptions) { o.Profile = profile })
		if err != nil {
			t.Fatal(err)
		}
		return line, r.json(line)
	}
	gmLine, gmSettings := settings(guardGM)
	hireLine, hireSettings := settings(guardHire)
	gm, err := hireStart{Role: Role{ID: gmRole, Name: "General Manager", Agent: "claude"}, Message: "go", GMToken: testGMToken,
		GMPlugin: "/p", GMCLI: "/c", Settings: gmLine}.plan()
	if err != nil {
		t.Fatal(err)
	}
	hire, err := hireStart{Role: testHireRole(), Message: "go", Ports: "4100-4103", SendKey: "k-1_a", DefaultAccount: true, Settings: hireLine}.plan()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range gmMemoryStartLines {
		if !strings.Contains(planText(gm), line) {
			t.Errorf("the GM's start lacks %q:\n%s", line, planText(gm))
		}
	}
	for _, name := range []string{claudeMemoryEnv, "CLAUDE_CODE_REMOTE_TOOLS_PIN_STORED_LOGIN", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_ENVIRONMENT_KIND"} {
		if strings.Contains(planText(hire), name) {
			t.Errorf("a hire's start names %s:\n%s", name, planText(hire))
		}
	}
	without := func(plan deskPlan, drop func(envChange) bool) deskPlan {
		plan.Changes = slices.DeleteFunc(slices.Clone(plan.Changes), drop)
		return plan
	}
	memoryLines := func(c envChange) bool { return slices.Contains(claudeMemoryStart, c) }
	pinOnly := without(gm, func(c envChange) bool { return memoryLines(c) && c.Unset })
	unsetsOnly := without(gm, func(c envChange) bool { return memoryLines(c) && !c.Unset })
	account := map[string]any{"env": map[string]any{claudeMemoryEnv: "0"}, "autoMemoryEnabled": true}
	for _, mode := range claudeLaunchModes {
		terminal := append([]string{"PATH=/usr/bin", "HOME=/h", claudeMemoryEnv + "=0"}, mode...)
		name := "a plain terminal"
		if mode != nil {
			name = strings.Join(mode, " ")
		}
		started := func(plan deskPlan) map[string]string {
			env, err := deskEnvironment(terminal, plan, nil)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			return environmentMap(env)
		}
		env := started(gm)
		if env[claudeMemoryEnv] != "1" {
			t.Errorf("%s: the GM starts with %s=%q", name, claudeMemoryEnv, env[claudeMemoryEnv])
		}
		for _, variable := range mode {
			if key, _, _ := strings.Cut(variable, "="); env[key] != "" {
				t.Errorf("%s: the GM starts with %s=%s", name, key, env[key])
			}
		}
		if claudeAutoMemoryOn(env, account, gmSettings) {
			t.Errorf("%s: the GM's memory is on", name)
		}
		for half, plan := range map[string]deskPlan{"the 1 alone": pinOnly, "the unsets alone": unsetsOnly} {
			if claudeAutoMemoryOn(started(plan), account, gmSettings) {
				t.Errorf("%s, %s: the GM's memory is on", name, half)
			}
		}
		// Without the start's lines, the verdict's attack: only a plain terminal is held, by the settings alone.
		if on := claudeAutoMemoryOn(started(without(gm, memoryLines)), account, gmSettings); on != (mode != nil) {
			t.Errorf("%s, without the start's lines: memory on is %v, want %v, so the row proves nothing", name, on, mode != nil)
		}
		hireEnv := started(hire)
		for _, variable := range append([]string{claudeMemoryEnv + "=0"}, mode...) {
			if key, value, _ := strings.Cut(variable, "="); hireEnv[key] != value {
				t.Errorf("%s: a hire's start changed the terminal's %s to %q", name, key, hireEnv[key])
			}
		}
		if !claudeAutoMemoryOn(hireEnv, account, hireSettings) {
			t.Errorf("%s: a hire's memory is off", name)
		}
	}
}
