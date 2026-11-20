package desktop

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// MEM4b: the GM's one memory is Smolcorp's. Its settings switch Claude Code's own memory off, in both of
// Claude Code's forms (the setting and the variable); a hire's settings leave it as the account has it.
func TestTheGMsSettingsSwitchClaudeCodesOwnMemoryOff(t *testing.T) {
	r := newHireSettingsRig(t)
	for _, system := range []string{"darwin", "linux", "windows"} {
		for _, auto := range []bool{true, false} {
			for _, profile := range []guardProfile{guardGM, guardHire, ""} {
				line, err := r.write("abc123", func(o *hireSettingsOptions) { o.Profile, o.Auto, o.System, o.Shell = profile, auto, system, "/bin/zsh" })
				if err != nil {
					t.Fatalf("%s %v %q: %v", system, auto, profile, err)
				}
				object := r.json(line)
				env := object["env"].(map[string]any)
				setting, set := object["autoMemoryEnabled"]
				variable, has := env[claudeMemoryEnv]
				name := fmt.Sprintf("%s, auto %v, profile %q", system, auto, profile)
				if profile == guardGM {
					if setting != false || variable != "1" || len(env) != 3 {
						t.Errorf("%s: autoMemoryEnabled %v, env %v", name, setting, env)
					}
				} else if set || has || len(env) != 2 {
					t.Errorf("%s: a hire's memory was changed: autoMemoryEnabled %v, env %v", name, setting, env)
				}
				if strings.Contains(line, `"--profile","gm"`) != (profile == guardGM) {
					t.Errorf("%s: the guard's profile isn't the settings'", name)
				}
			}
		}
	}
}

// claudeAutoMemoryOn is whether Claude Code runs its own memory, from its strings at 2.1.314 (the floor) and 2.1.325
// (the installed binaries, never an owner's ~/.claude). launch is the environment it starts with, and files its
// settings files in its order (fi: the account's, the project's, the local one, the --settings line; managed
// settings, an organization's, aren't modelled).
//   - Each file's env is assigned over launch in turn (Go(), filterSettingsEnv), but none at all while launch has
//     CLAUDE_CODE_REMOTE_TOOLS_PIN_STORED_LOGIN on (withholdsEnvFrom: every file's env but managed settings'), and,
//     in a host-orchestrated launch (CLAUDE_CODE_ENTRYPOINT claude-desktop, claude-desktop-3p or local-agent, or
//     CLAUDE_CODE_ENVIRONMENT_KIND byoc), no key launch already has, in any case (ne(), hostSpawnEnvKeys). Claude
//     Code's other filters (an untrusted folder's, project scope, server-owned names) only drop more of an owner's
//     files, and a value here is taken as fmt.Sprint writes it where Claude Code may drop one that isn't a string:
//     both only add attacks.
//   - Then Hb(): CLAUDE_CODE_DISABLE_AUTO_MEMORY's Pe() words (1, true, yes or on, any case, trimmed: also how the
//     pin variable is read, H.bool) say off and ws()'s (0, false, no or off) forced on; otherwise hue()/HF() take
//     the last file's autoMemoryEnabled, on when no file sets it. (Safe mode, a restricted or a bare session switch
//     it off too; a desk is none of them.)
func claudeAutoMemoryOn(launch map[string]string, files ...map[string]any) bool {
	word := func(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
	on, off := []string{"1", "true", "yes", "on"}, []string{"0", "false", "no", "off"}
	variables, started := map[string]string{}, map[string]bool{}
	for name, value := range launch {
		variables[name], started[strings.ToUpper(name)] = value, true
	}
	pinned := slices.Contains(on, word(launch["CLAUDE_CODE_REMOTE_TOOLS_PIN_STORED_LOGIN"]))
	host := slices.Contains([]string{"claude-desktop", "claude-desktop-3p", "local-agent"}, launch["CLAUDE_CODE_ENTRYPOINT"]) ||
		launch["CLAUDE_CODE_ENVIRONMENT_KIND"] == "byoc"
	enabled := any(nil)
	for _, file := range files {
		env, _ := file["env"].(map[string]any)
		for name, value := range env {
			if !pinned && !(host && started[strings.ToUpper(name)]) {
				variables[name] = fmt.Sprint(value)
			}
		}
		if value, ok := file["autoMemoryEnabled"]; ok {
			enabled = value
		}
	}
	if value, set := variables[claudeMemoryEnv]; set {
		if slices.Contains(on, word(value)) {
			return false
		}
		if slices.Contains(off, word(value)) {
			return true
		}
	}
	if enabled != nil {
		return enabled != false
	}
	return true
}

// An owner's env can't switch the GM's memory back on: not the account's settings (under SA-X8, the copy in the
// desk's own Claude folder), the project's, the local ones or a plain terminal, in any form Claude Code reads as "on".
// (A terminal in one of the launch modes that keep the settings' env out is the GM's start's to answer:
// TestTheGMsStartKeepsItsMemoryOffWhateverSmolcorpInherited.)
// The same owner files do switch on a hire's (the rows are live), and the setting alone, without the variable,
// wouldn't hold (why the GM's settings set both).
func TestAnOwnersEnvCantSwitchTheGMsMemoryBackOn(t *testing.T) {
	r := newHireSettingsRig(t)
	settings := func(profile guardProfile) map[string]any {
		line, err := r.write("abc123", func(o *hireSettingsOptions) { o.Profile = profile })
		if err != nil {
			t.Fatal(err)
		}
		return r.json(line)
	}
	gm, hire := settings(guardGM), settings(guardHire)
	settingOnly := settings(guardGM)
	delete(settingOnly["env"].(map[string]any), claudeMemoryEnv)

	owner := func(env map[string]any, enabled any) map[string]any {
		file := map[string]any{"env": env}
		if enabled != nil {
			file["autoMemoryEnabled"] = enabled
		}
		return file
	}
	none := map[string]any{}
	rows := []struct {
		name     string
		terminal map[string]string
		account  map[string]any
		project  map[string]any
		local    map[string]any
	}{
		{"the owner sets nothing", nil, none, none, none},
		{"the account's env says 0", nil, owner(map[string]any{claudeMemoryEnv: "0"}, nil), none, none},
		{"the account's env says false, and memory on", nil, owner(map[string]any{claudeMemoryEnv: "false"}, true), none, none},
		{"the account's env says No", nil, owner(map[string]any{claudeMemoryEnv: " No "}, nil), none, none},
		{"the account's env says OFF", nil, owner(map[string]any{claudeMemoryEnv: "OFF"}, nil), none, none},
		// Claude Code may drop a value that isn't a string (RT126MEM4b nit 3): these two only add attacks.
		{"the account's env says 0 as a number", nil, owner(map[string]any{claudeMemoryEnv: 0}, nil), none, none},
		{"the account's env says false as a boolean", nil, owner(map[string]any{claudeMemoryEnv: false}, true), none, none},
		{"the project's env says 0", nil, none, owner(map[string]any{claudeMemoryEnv: "0"}, true), none},
		{"the local env says off", nil, none, none, owner(map[string]any{claudeMemoryEnv: "off"}, true)},
		{"the terminal says 0", map[string]string{claudeMemoryEnv: "0"}, owner(nil, true), none, none},
	}
	for _, row := range rows {
		if claudeAutoMemoryOn(row.terminal, row.account, row.project, row.local, gm) {
			t.Errorf("%s: the GM's memory is on", row.name)
		}
		if row.name == "the owner sets nothing" {
			if !claudeAutoMemoryOn(row.terminal, row.account, row.project, row.local, hire) {
				t.Errorf("%s: a hire's memory is off", row.name)
			}
			continue
		}
		if !claudeAutoMemoryOn(row.terminal, row.account, row.project, row.local, hire) {
			t.Errorf("%s: a hire's memory stays off, so the row proves nothing", row.name)
		}
		if !claudeAutoMemoryOn(row.terminal, row.account, row.project, row.local, settingOnly) {
			t.Errorf("%s: the setting alone held, so the row proves nothing about the variable", row.name)
		}
	}
}
