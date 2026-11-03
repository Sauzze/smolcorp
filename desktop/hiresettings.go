package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// A manager hire's own Claude Code settings, ported from the Mac's HireSettings.swift:
// a file the app writes for that hire alone, <support>/hires/<id>/settings.json (private
// to you), which the hire's Claude Code starts with (--settings), merged on top of the
// account's own settings. It holds
//
//   - deny rules for the plain forms of what the guard refuses (push, deploy, publish…),
//     for reading .env files and Claude's session transcripts, and for editing Claude's
//     settings and this app's files;
//   - the guard (`smolcorp guard`) as a PreToolUse hook before every tool call;
//   - disableAllHooks:false, so a checkout's own settings can't turn the guard off;
//   - auto mode, where the account allows it.
//
// Deny rules only see the start of a command, so they stop the plain forms; the guard
// sees the rest. A deny rule beats an allow, so the hire's own STATUS.md, which it must
// write, is never covered by a rule: the support folder is denied piece by piece, never
// as a whole. The GM's file is the same with its own guard profile; a hire never gets
// the GM's token, and the rules below keep it out of the token, the setup key, the hook
// key, other hires' folders and the app's state.
//
// Nothing calls this yet. T602's hire engine (Linux tmux) will call writeHireSettings
// when it starts a managed Claude hire and pass the file as --settings=<path>; T604
// does the same from the Windows launcher.

// hireGuardFile is the app's guard rules file, in the support folder.
const hireGuardFile = "hire-guard.json"

// hireSettingsFile is the name of a hire's settings file in its own folder.
const hireSettingsFile = "settings.json"

// hireKnownFolders are the folders the app makes in its support folder, besides
// hires: denied even when absent, so one made later is covered too.
var hireKnownFolders = []string{"briefs", "recaps"}

// guardGroup is one group the guard refuses, as `smolcorp guard --print` says it.
type guardGroup struct {
	Name     string   `json:"name"`
	On       bool     `json:"on"`
	Commands []string `json:"commands"`
}

// claudeProfile is a Claude Code account's config folder (CLAUDE_CONFIG_DIR).
type claudeProfile struct {
	Name string
	Dir  string
}

// guardProfile chooses whose guard rules a file carries: a hire's, or the GM's own.
type guardProfile string

const (
	guardHire guardProfile = "hire"
	guardGM   guardProfile = "gm"
)

// arguments are the words after `guard` that choose it. A hire's has none.
func (p guardProfile) arguments() string {
	if p == guardGM {
		return "--profile gm "
	}
	return ""
}

// hireRefusal is why a hire can't be started: the status and words the front desk gives.
type hireRefusal struct {
	Status  int
	Message string
}

func (r *hireRefusal) Error() string { return r.Message }

var errGuardMissing = &hireRefusal{Status: 409, Message: "This copy of Smolcorp has no smolcorp command inside it, so a hire can't be guarded, and Smolcorp won't start one unguarded. Install a build of Smolcorp that includes it."}

// guardRunner runs `tool arguments`: its status and output, ok false if it couldn't be run.
type guardRunner func(tool string, arguments []string) (status int, output string, ok bool)

func runGuardTool(tool string, arguments []string) (int, string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out bytes.Buffer
	command := exec.CommandContext(ctx, tool, arguments...)
	command.Stdout = &out
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), out.String(), true
		}
		return 0, "", false
	}
	return 0, out.String(), true
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode()&0o111 != 0
}

// hireFolder is the hire's own folder in the support folder, where its settings, brief and status go.
func hireFolder(support, hireID string) string {
	return filepath.Join(support, "hires", hireID)
}

// guardGroups is the groups in force, as the guard itself reads rules: the app and the
// hook never disagree. Nil if it can't be run or says something we can't read.
func guardGroups(tool, rules string, profile guardProfile, run guardRunner) []guardGroup {
	arguments := []string{"guard", "--print"}
	if profile == guardGM {
		arguments = append(arguments, "--profile", "gm")
	}
	status, output, ok := run(tool, append(arguments, "--rules", rules))
	if !ok || status != 0 {
		return nil
	}
	var answer struct {
		Groups []struct {
			Name     string   `json:"name"`
			On       *bool    `json:"on"`
			Commands []string `json:"commands"`
		} `json:"groups"`
	}
	if json.Unmarshal([]byte(output), &answer) != nil || answer.Groups == nil {
		return nil
	}
	groups := make([]guardGroup, 0, len(answer.Groups))
	for _, g := range answer.Groups {
		// A group that doesn't say whether it is on is refused, not read as off: the Mac does the same.
		if g.On == nil {
			return nil
		}
		groups = append(groups, guardGroup{Name: g.Name, On: *g.On, Commands: g.Commands})
	}
	return groups
}

// guardHookCommand is the hook's command line: the tool, quoted for the shell, then
// `guard` and the rules file.
func guardHookCommand(tool, rules string, profile guardProfile) string {
	return shellPath(tool) + " guard " + profile.arguments() + "--rules " + shellPath(rules)
}

// hireCommandRules are the deny rules the groups in force give: each command with its
// words, each alternative (eas build, eas submit…), plain and through npx. A rule that
// needs an option, its words anywhere or a remote host is the guard's alone: a deny
// rule can't say it.
func hireCommandRules(groups []guardGroup) []string {
	var rules []string
	seen := map[string]bool{}
	for _, group := range groups {
		if !group.On {
			continue
		}
		for _, rule := range group.Commands {
			words := strings.Fields(rule)
			skip := len(words) == 0
			for _, word := range words {
				if word == "..." || strings.HasPrefix(word, "@") || strings.HasPrefix(word, "-") {
					skip = true
				}
			}
			if skip {
				continue
			}
			forms := []string{""}
			for _, word := range words {
				var next []string
				for _, start := range forms {
					for _, alternative := range strings.Split(word, "|") {
						if alternative == "" {
							continue
						}
						if start == "" {
							next = append(next, alternative)
						} else {
							next = append(next, start+" "+alternative)
						}
					}
				}
				forms = next
			}
			for _, form := range forms {
				for _, rule := range []string{"Bash(" + form + ":*)", "Bash(npx " + form + ":*)"} {
					if !seen[rule] {
						seen[rule] = true
						rules = append(rules, rule)
					}
				}
			}
		}
	}
	return rules
}

// absoluteRulePath is a path as Claude Code's rules write an absolute one: // then the
// path. On Windows the drive stays (C:/Users/…), with forward slashes; that form is not
// yet checked on Claude Code for Windows (T604).
func absoluteRulePath(path string) string {
	path = filepath.ToSlash(path)
	return "//" + strings.TrimPrefix(path, "/")
}

// hireDenyRules is every deny rule for hire hireID. others are the names of the other
// hires' folders under <support>/hires, entries what sits in the support folder now (a
// folder named here is denied whole, except hires, which is denied hire by hire so this
// hire's own STATUS.md stays writable; hireKnownFolders are listed too when absent).
func hireDenyRules(hireID string, groups []guardGroup, support string, profiles []claudeProfile, others, entries []string) []string {
	rules := hireCommandRules(groups)
	// Claude Code's Edit rules cover every tool that changes a file (Write too); a Write(path) rule never matches and warns at every start.
	both := func(pattern string) { rules = append(rules, "Edit("+pattern+")") }

	// The hire's secrets and Claude's own settings, wherever they are.
	rules = append(rules, "Read(**/.env*)", "Read(//**/.env*)")
	both("**/.env*")
	both("//**/.env*")
	both("~/.claude*/settings*.json")
	both("~/.claude*.json")
	both("**/.claude/settings*.json")
	both("//**/.claude/settings*.json")
	rules = append(rules, "Read(~/.claude*/projects/**)")
	for _, profile := range profiles {
		dir := filepath.Clean(profile.Dir)
		if !filepath.IsAbs(dir) {
			continue
		}
		both(absoluteRulePath(dir) + "/settings*.json")
		rules = append(rules, "Read("+absoluteRulePath(dir)+"/projects/**)")
	}

	// This app's folder, piece by piece: never a rule that would cover the hire's own STATUS.md.
	root := absoluteRulePath(support)
	for _, pattern := range []string{"*.json", "*.sh", "*.md", "*.log", "*.jsonl", "gm-token", "setup-key", "hook-header", "hook-key", "window-key", "token"} {
		both(root + "/" + pattern)
	}
	has := func(list []string, name string) bool {
		for _, item := range list {
			if item == name {
				return true
			}
		}
		return false
	}
	for _, entry := range entries {
		if entry != "hires" && !strings.HasPrefix(entry, ".") && !strings.Contains(entry, "/") {
			both(root + "/" + entry + "/**")
		}
	}
	// The folders the app is known to make, listed even when absent now.
	for _, name := range hireKnownFolders {
		if !has(entries, name) {
			both(root + "/" + name + "/**")
		}
	}
	both(root + "/hires/*.sh")
	// Every hire's folder, by file type and not by name: a hire started after this file
	// was written is covered too. STATUS.md is .md, so it stays writable.
	for _, pattern := range []string{"*.json", "*.sh", "*.jsonl", "*.log", "settings*"} {
		both(root + "/hires/*/" + pattern)
	}
	for _, other := range others {
		if other != hireID && !strings.Contains(other, "/") && !strings.HasPrefix(other, ".") {
			both(root + "/hires/" + other + "/**")
		}
	}
	// Nothing secret or app-state can be read either.
	for _, name := range []string{"gm-token", "setup-key", "hook-header", "hook-key", "window-key", "token", hireGuardFile, "phones.json", "sessions.json", "config.json", "registry.json"} {
		rules = append(rules, "Read("+root+"/"+name+")")
	}
	return rules
}

// hireSettingsValue is the settings file's content. Auto mode goes in as the default
// mode, where Claude Code lets the account have it.
// reporting is only for a test copy's hire: the hooks that make it report to the
// copy, after the guard (T704); nil for a real install's, which reports through the
// account's own settings to the default port.
func hireSettingsValue(deny []string, guardCommand string, auto bool, reporting map[string]any) map[string]any {
	permissions := map[string]any{"deny": deny}
	if auto {
		permissions["defaultMode"] = "auto"
	}
	// Hooks stay on: this file sits above the project's and local settings, so a checkout can't turn the guard off.
	hooks := map[string]any{"PreToolUse": []any{map[string]any{
		"matcher": "*",
		"hooks":   []any{map[string]any{"type": "command", "command": guardCommand, "timeout": 10}},
	}}}
	for event, groups := range reporting {
		existing, _ := hooks[event].([]any)
		hooks[event] = append(existing, groups.([]any)...)
	}
	return map[string]any{"permissions": permissions, "disableAllHooks": false, "hooks": hooks}
}

// reportingHooks are a test copy's hire's hooks: Claude's usual events, running
// the app's own hook command aimed at the copy's port, never the default one.
func reportingHooks(cli string, port int) map[string]any {
	command := fmt.Sprintf("%s hook --port %d", shellPath(cli), port)
	hooks := map[string]any{}
	for _, event := range entriesFor(Claude).events {
		hooks[event] = []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "async": true}}}}
	}
	return hooks
}

// hireSettingsOptions are what writeHireSettings needs from outside, each with a test stand-in.
type hireSettingsOptions struct {
	HireID  string
	Auto    bool
	Profile guardProfile
	CLI     string // the smolcorp command inside the app
	Support string // the app's support folder
	// A test copy's port: its hire's settings then carry hooks that report to it (0 for a real install's).
	ReportPort int
	Profiles   []claudeProfile // the Claude accounts whose settings and transcripts are denied
	Run        guardRunner     // nil: run the real tool
	IsExec     func(string) bool
}

// writeHireSettings writes hire HireID's settings and says where: refused if the guard
// can't be run. The file is private to you and replaced whole.
func writeHireSettings(o hireSettingsOptions) (string, error) {
	run, isExec := o.Run, o.IsExec
	if run == nil {
		run = runGuardTool
	}
	if isExec == nil {
		isExec = isExecutableFile
	}
	profile := o.Profile
	if profile == "" {
		profile = guardHire
	}
	if o.HireID == "" || strings.ContainsAny(o.HireID, `/\`) || strings.HasPrefix(o.HireID, ".") {
		return "", &hireRefusal{Status: 400, Message: "That isn't a hire's name, so Smolcorp won't write settings for it."}
	}
	if !isExec(o.CLI) {
		return "", errGuardMissing
	}
	rules := filepath.Join(o.Support, hireGuardFile)
	groups := guardGroups(o.CLI, rules, profile, run)
	if groups == nil {
		return "", &hireRefusal{Status: 500, Message: "The guard couldn't say which commands a hire is refused, so Smolcorp won't start one unguarded."}
	}
	names := func(dir string) []string {
		var list []string
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			list = append(list, entry.Name())
		}
		return list
	}
	var others []string
	for _, name := range names(filepath.Join(o.Support, "hires")) {
		if name != o.HireID {
			others = append(others, name)
		}
	}
	deny := hireDenyRules(o.HireID, groups, o.Support, o.Profiles, others, names(o.Support))
	var reporting map[string]any
	if o.ReportPort != 0 {
		reporting = reportingHooks(o.CLI, o.ReportPort)
	}
	object := hireSettingsValue(deny, guardHookCommand(o.CLI, rules, profile), o.Auto, reporting)

	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	failed := &hireRefusal{Status: 500, Message: "Couldn't write the hire's settings, so Smolcorp won't start it unguarded."}
	if encoder.Encode(object) != nil {
		return "", failed
	}
	folder := hireFolder(o.Support, o.HireID)
	if os.MkdirAll(folder, 0o700) != nil || os.Chmod(folder, 0o700) != nil {
		return "", failed
	}
	target := filepath.Join(folder, hireSettingsFile)
	if writePrivate(target, data.Bytes()) != nil {
		return "", failed
	}
	return target, nil
}
