package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/guard"
)

// A manager hire's own Claude Code settings, ported from the Mac's HireSettings.swift:
// made for that hire alone and given to its Claude Code in its command line
// (--settings '<json>'), never as a file a hire could change (SANDBOX-PLAN §3.4, SG1).
// Settings given there rank above the account's, the project's and the local ones,
// which are merged under them. They hold
//
//   - deny rules for the plain forms of what the guard refuses (push, deploy, publish…),
//     for reading .env files and Claude's session transcripts, and for editing Claude's
//     settings and this app's files;
//   - the guard (`smolcorp guard`) as a PreToolUse hook before every tool call, with the
//     core's copy of the owner's guard in its own command line (--groups);
//   - the guard as a ConfigChange hook (`smolcorp guard --config`), which refuses every
//     reload of the account's, the project's or the local settings during the session;
//   - every one of this app's hooks run without a shell (command and args), and with
//     onFailure "block": a guard that times out, is killed or fails refuses the call;
//   - pinned keys a lower file could otherwise loosen: the permission mode, always set,
//     bypass mode switched off, an empty shell prefix and a fixed shell, and
//     disableAllHooks:false;
//   - auto mode, where the account allows it.
//
// Deny rules only see the start of a command, so they stop the plain forms; the guard
// sees the rest. A deny rule beats an allow, so the hire's own STATUS.md, which it must
// write, is never covered by a rule: the support folder is denied piece by piece, never
// as a whole. The GM's file is the same with its own guard profile; a hire never gets
// the GM's token, and the rules below keep it out of the token, the setup key, the hook
// key, other hires' folders and the app's state.
//
// The tmux engine puts them in the hire's start script (hirestart.go); the Windows
// launcher gets them with the hire's start environment (hirestartenv.go). A desk is
// started only on a Claude Code that takes onFailure (claudeVersionRefusal).

// hireGuardFile is the app's guard rules file, in the support folder.
const hireGuardFile = "hire-guard.json"

// hireSettingsFile is the name of the settings file a hire had in its own folder before 1.26.
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
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// guardProfile chooses whose guard rules a file carries: a hire's, or the GM's own.
type guardProfile string

const (
	guardHire guardProfile = "hire"
	guardGM   guardProfile = "gm"
)

// arguments are the words after `guard` that choose it. A hire's has none.
func (p guardProfile) arguments() []string {
	if p == guardGM {
		return []string{"--profile", "gm"}
	}
	return nil
}

// hireRefusal is why a hire can't be started: the status and words the front desk gives.
type hireRefusal struct {
	Status  int
	Message string
	// keyTaken: a fire refused while the records' key can't be read, once the hire's session was ended and its send
	// key taken away for the run (heldFire).
	keyTaken bool
}

func (r *hireRefusal) Error() string { return r.Message }

var errGuardMissing = &hireRefusal{Status: 409, Message: "This copy of Smolcorp has no smolcorp command inside it, so a hire can't be guarded, and Smolcorp won't start one unguarded. Install a build of Smolcorp that includes it."}

// guardRunner runs `tool arguments`: its status and output, ok false if it couldn't be run.
type guardRunner func(tool string, arguments []string) (status int, output string, ok bool)

func runGuardTool(tool string, arguments []string) (int, string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out bytes.Buffer
	command := disclaim.Cmd(exec.CommandContext(ctx, tool, arguments...))
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

// guardGroups is the groups in force, as the guard itself reads them, from held (the
// core's copy, as --groups gives it) and what the rules file adds: the app and the hook
// never disagree. Nil if it can't be run or says something we can't read.
func guardGroups(tool, rules, held string, profile guardProfile, run guardRunner) []guardGroup {
	arguments := append([]string{"guard", "--print"}, profile.arguments()...)
	status, output, ok := run(tool, append(arguments, "--groups", held, "--rules", rules))
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

// smolcorpHook is one of this app's hooks, run without a shell: the tool and its
// arguments as they are, so no shell start file, BASH_ENV or shell prefix wraps it, and
// a failure (a timeout, a kill, a crash) blocks what the event guards (SANDBOX-PLAN
// §3.4). timeout 0 leaves Claude Code's own.
func smolcorpHook(tool string, arguments []string, timeout int) map[string]any {
	hook := map[string]any{"type": "command", "command": tool, "args": arguments, "onFailure": "block"}
	if timeout > 0 {
		hook["timeout"] = timeout
	}
	return hook
}

// guardHook is the PreToolUse hook: `guard`, its profile, the core's copy of the
// groups (held) and the rules file, which only adds to them.
func guardHook(tool, rules, held string, profile guardProfile) map[string]any {
	arguments := append(append([]string{"guard"}, profile.arguments()...), "--groups", held, "--rules", rules)
	return smolcorpHook(tool, arguments, 10)
}

// configHook is the ConfigChange hook: `guard --config`, which refuses a reload of the
// account's, the project's and the local settings.
func configHook(tool string) map[string]any {
	return smolcorpHook(tool, []string{"guard", "--config"}, 10)
}

// pinnedShell is the shell a desk's Bash tool runs, pinned in its settings so a
// project's settings can't name another (CLAUDE_CODE_SHELL, then SHELL, come first
// in Claude Code's choice): yours when it is the system's bash or zsh, else the
// system's of the same name as yours (a Homebrew zsh gets /bin/zsh), else the
// system's bash, else its zsh. "" off Mac and Linux, and when none is there.
func pinnedShell(goos, yours string, exists func(string) bool) string {
	if goos != "darwin" && goos != "linux" {
		return ""
	}
	system := []string{"/bin/bash", "/bin/zsh", "/usr/bin/bash", "/usr/bin/zsh"}
	if slices.Contains(system, yours) && exists(yours) {
		return yours
	}
	if name := filepath.Base(yours); name == "bash" || name == "zsh" {
		for _, shell := range system {
			if filepath.Base(shell) == name && exists(shell) {
				return shell
			}
		}
	}
	for _, shell := range system {
		if exists(shell) {
			return shell
		}
	}
	return ""
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
// path. On Windows Claude Code makes a path POSIX before matching it, C:\Users\ana
// becoming /c/Users/ana (its permissions docs), so the drive is a folder of its own,
// in lower case: //c/Users/ana. A rule that kept C: would never match.
func absoluteRulePath(path string) string {
	if len(path) >= 2 && path[1] == ':' && ('A' <= path[0] && path[0] <= 'Z' || 'a' <= path[0] && path[0] <= 'z') {
		path = "/" + strings.ToLower(path[:1]) + path[2:]
	}
	path = filepath.ToSlash(path)
	return "//" + strings.TrimPrefix(path, "/")
}

// hireDenyRules is every deny rule for hire hireID. others are the names of the other
// hires' folders under <support>/hires, entries what sits in the support folder now (a
// folder named here is denied whole, except hires, which is denied hire by hire so this
// hire's own STATUS.md stays writable; hireKnownFolders are listed too when absent). system
// is Go's name for the system the hire runs on.
func hireDenyRules(hireID string, groups []guardGroup, support string, profiles []claudeProfile, others, entries []string, system string) []string {
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
	// The secrets of both engines a Mac may run (the Swift app's gm-session and hook-header, the Go
	// engine's hook-key and window-key), in the order the Mac's fence has always had (the fixtures,
	// Tests/fixtures/frontdesk): one file either way.
	// Then the owner's grant record (grants.go): its key's name, kept off the disk, and the file it is written
	// through (grants.jsonl itself is a *.jsonl); the paired phones' seal's key (access.go: phones.json itself
	// is a *.json); and which Claude Code the owner has (agentcheck.go: one a hire wrote "npm" in would let an
	// unsigned Claude Code start after a signed one, at the next start), and the file it is written through,
	// whether or not they are there yet.
	for _, pattern := range []string{"*.json", "*.sh", "*.md", "*.log", "*.jsonl", "gm-token", "gm-session", "setup-key", "hook-header", "hook-key", "window-key", "token", "grants-key", "grants.jsonl.tmp", phoneSealName, claudeInstallName, claudeInstallTemp} {
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
	// An entry is denied as a file and as a folder: a /** rule alone doesn't match a file.
	for _, entry := range entries {
		if entry != "hires" && entry != keptGMTokenName && entry != keptGMTokenTemp && !strings.HasPrefix(entry, ".") && !strings.Contains(entry, "/") {
			if file := "Edit(" + root + "/" + entry + ")"; !has(rules, file) {
				rules = append(rules, file)
			}
			both(root + "/" + entry + "/**")
		}
	}
	// The folders the app is known to make, listed even when absent now.
	for _, name := range hireKnownFolders {
		if !has(entries, name) {
			both(root + "/" + name + "/**")
		}
	}
	// Which Claude Code the owner has, and the file it is written through, as folders too, listed even when absent
	// now: a folder a hire left in either's place would stop "signed" from ever reaching the disk (RT126SG2F B1).
	for _, name := range []string{claudeInstallName, claudeInstallTemp} {
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
	for _, name := range []string{"gm-token", "gm-session", "setup-key", "hook-header", "hook-key", "window-key", "token", hireGuardFile, "phones.json", "sessions.json", "config.json", "registry.json", "grants-key", "grants.jsonl", "grants.jsonl.tmp", phoneSealName, claudeInstallName, claudeInstallTemp} {
		rules = append(rules, "Read("+root+"/"+name+")")
	}
	// The GM's token, kept in a file on a computer whose secret store can't keep it (gmtokenstore.go), and the file
	// it is written through: on Linux and Windows always, whether or not it is there yet, since a GM hired after this
	// hire makes it. A Mac never keeps it there, so its fence is unchanged. So is the push key (pushkey.go), which
	// signs every notification, and the file it is written through.
	if system != "darwin" {
		for _, name := range []string{keptGMTokenName, keptGMTokenTemp, keptPushKeyName, keptPushKeyName + ".tmp"} {
			rules = append(rules, "Edit("+root+"/"+name+")", "Read("+root+"/"+name+")")
		}
	}
	return rules
}

// hireSettingsValue is the settings' content. Auto mode goes in as the default mode,
// where Claude Code lets the account have it; otherwise the default mode is pinned too.
// guard and config are the PreToolUse and ConfigChange hooks; shell the pinned shell.
// reporting is only for a test copy's hire: the hooks that make it report to the
// copy, after the guard (T704); nil for a real install's, which reports through the
// account's own settings to the default port.
func hireSettingsValue(deny []string, guard, config map[string]any, shell string, auto bool, reporting map[string]any) map[string]any {
	permissions := map[string]any{"deny": deny, "defaultMode": "default", "disableBypassPermissionsMode": "disable"}
	if auto {
		permissions["defaultMode"] = "auto"
	}
	// Hooks stay on: these settings sit above the project's and local settings, so a checkout can't turn the guard off.
	hooks := map[string]any{
		"PreToolUse":   []any{map[string]any{"matcher": "*", "hooks": []any{guard}}},
		"ConfigChange": []any{map[string]any{"hooks": []any{config}}},
	}
	for event, groups := range reporting {
		existing, _ := hooks[event].([]any)
		hooks[event] = append(existing, groups.([]any)...)
	}
	// No prefix wraps a command, and the Bash tool's shell is the one named here.
	env := map[string]any{"CLAUDE_CODE_SHELL_PREFIX": "", "CLAUDE_CODE_SHELL": shell}
	return map[string]any{"permissions": permissions, "disableAllHooks": false, "hooks": hooks, "env": env}
}

// reportingHooks are a test copy's hire's hooks: Claude's usual events, running
// the app's own hook command aimed at the copy's port, never the default one.
func reportingHooks(cli string, port int) map[string]any {
	hooks := map[string]any{}
	for _, event := range entriesFor(Claude).events {
		hook := smolcorpHook(cli, []string{"hook", "--port", strconv.Itoa(port)}, 0)
		hook["async"] = true
		hooks[event] = []any{map[string]any{"hooks": []any{hook}}}
	}
	return hooks
}

// hireSettingsOptions are what hireSettingsLine needs from outside, each with a test stand-in.
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
	// Shell is the shell the Bash tool is pinned to; "" asks pinnedShell.
	Shell string
	// System is Go's name for the system the desk starts on; "" is this one.
	System string
}

// hireSettingsLine is hire HireID's settings as its Claude Code takes them in its
// command line, after --settings: refused if the guard can't be run. Each start that
// uses it has checked Claude Code's version first (claudeVersionRefusal).
func hireSettingsLine(o hireSettingsOptions) (string, error) {
	object, err := hireSettingsObject(o)
	if err != nil {
		return "", err
	}
	var line bytes.Buffer
	encoder := json.NewEncoder(&line)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(object) != nil {
		return "", &hireRefusal{Status: 500, Message: "Couldn't make the hire's settings, so Smolcorp won't start it unguarded."}
	}
	settings := strings.TrimSuffix(line.String(), "\n")
	if err := settingsTooLong(settings, o.Profiles, cmpOr(o.System, runtime.GOOS)); err != nil {
		return "", err
	}
	return settings, nil
}

// settingsTooLong refuses, on Windows only, settings Windows couldn't start a desk with: they ride in its command
// line (--settings=<json>, quoted as Windows takes it), which holds at most 32767 characters. Each Claude account
// kept outside ~/.claude* adds about 2,000 of them (promptStoreRules), so the sentence names those accounts.
func settingsTooLong(settings string, profiles []claudeProfile, system string) error {
	if system != "windows" || len(exeLine("--settings="+settings, nil)) <= exeLongest {
		return nil
	}
	var names []string
	for _, profile := range profiles {
		if _, ok := claudeStoreFolder(profile.Dir); ok {
			names = append(names, cmpOr(profile.Name, profile.Dir))
		}
	}
	if len(names) == 0 {
		return &hireRefusal{Status: 409, Message: "Windows can't start this desk: its settings are too long, so turn off some guard rules in Settings, then hire again."}
	}
	accounts, fix := "the Claude account "+names[0]+" is", "move it to one or remove it"
	if len(names) > 1 {
		accounts = "the Claude accounts " + strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1] + " are"
		fix = "move them to ~/.claude-<name> folders or remove some"
	}
	return &hireRefusal{Status: 409, Message: "Windows can't start this desk: " + accounts + " kept outside a ~/.claude-<name> folder, " +
		"which makes its settings too long, so " + fix + " under Claude accounts in Settings, then hire again."}
}

// hireSettingsObject is what hire HireID's settings hold (deny rules, the guard as a
// PreToolUse and a ConfigChange hook, the pinned keys, auto mode), as hireSettingsLine
// gives them: refused if the guard can't be run. A hire in T3 Code gets the same, as its worktree's fence (t3desk.go), as
// the Mac's HireSettings.object gives both.
func hireSettingsObject(o hireSettingsOptions) (map[string]any, error) {
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
		return nil, &hireRefusal{Status: 400, Message: "That isn't a hire's name, so Smolcorp won't write settings for it."}
	}
	if !isExec(o.CLI) {
		return nil, errGuardMissing
	}
	rules := filepath.Join(o.Support, hireGuardFile)
	held := guard.GroupsArgument(heldGuardGroups(rules))
	groups := guardGroups(o.CLI, rules, held, profile, run)
	if groups == nil {
		return nil, &hireRefusal{Status: 500, Message: "The guard couldn't say which commands a hire is refused, so Smolcorp won't start one unguarded."}
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
	deny := hireDenyRules(o.HireID, groups, o.Support, o.Profiles, others, names(o.Support), cmpOr(o.System, runtime.GOOS))
	deny = append(deny, photoFenceRules(o.Support, o.Profiles, deny)...)
	var reporting map[string]any
	if o.ReportPort != 0 {
		reporting = reportingHooks(o.CLI, o.ReportPort)
	}
	shell := o.Shell
	if shell == "" {
		shell = pinnedShell(runtime.GOOS, os.Getenv("SHELL"), isExecutableFile)
	}
	return hireSettingsValue(deny, guardHook(o.CLI, rules, held, profile), configHook(o.CLI), shell, o.Auto, reporting), nil
}
