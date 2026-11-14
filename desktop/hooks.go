package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	agentdeck "github.com/Sauzze/smolcorp"
)

// Hooks is where each coding agent is told to run the hook, as the Mac app's
// Hooks.swift: the same files, the same entries, with `smolcorp hook` in
// place of report.sh.
//
// User-level, so every session reports to the deck, whatever folder it
// starts in. Only agents with a settings folder are connected; one installed
// later is picked up by connecting again. Claude Code, Codex and Gemini CLI
// read hooks from a settings file they share with everything else: the
// deck's entries are added to it, the rest is kept as it was, and the file is
// backed up before each change (see writeSettings). Grok and OpenCode
// load whole files from a folder, so the deck has one of its own there.

// hookEntries are the deck's entries in an agent's settings, in the layout
// Claude Code set and the others follow: event → matcher groups → handlers.
type hookEntries struct {
	file      string
	events    []string
	handler   map[string]any
	isOwnFile bool // the file holds nothing but these entries
}

func agentFolder(agent Agent) string {
	return filepath.Join(homeDir, map[Agent]string{
		Claude: ".claude", Codex: ".codex", Gemini: ".gemini", OpenCode: filepath.Join(".config", "opencode"), Grok: ".grok",
	}[agent])
}

func openCodePlugin() string {
	return filepath.Join(agentFolder(OpenCode), "plugins", "smolcorp.js")
}

// Grok's and OpenCode's files under the name the app had before, moved to the
// new ones as the app connects or starts.
func oldOpenCodePlugin() string {
	return filepath.Join(agentFolder(OpenCode), "plugins", "sauzzedeck-pro.js")
}

func oldGrokHooks() string {
	return filepath.Join(agentFolder(Grok), "hooks", "sauzzedeck-pro.json")
}

// hookCommand is the command an agent runs. Grok's entries match Claude
// Code's to the letter: Grok also reads Claude Code's hooks, and runs a
// handler it finds twice only once. It names this office's port, so two
// offices on one computer never mix, whatever their agents inherit.
func hookCommand(agent Agent) string {
	command := fmt.Sprintf("%s hook --port %d", shellPath(commandLineProgram()), Port)
	if agent == Claude || agent == Grok {
		return command
	}
	return command + " " + string(agent)
}

// executable is this program, where tests name an installed copy instead.
var executable = os.Executable

// commandLineProgram is the program agents run: on Windows, the console
// build next to the app's, since the app itself has no console.
func commandLineProgram() string {
	program, _ := executable()
	if resolved, err := filepath.EvalSymlinks(program); err == nil {
		program = resolved
	}
	if runtime.GOOS == "windows" {
		return consoleBuildBeside(program)
	}
	return program
}

// consoleBuildBeside is smolcorp-cli.exe beside program, as the downloads
// have it. A copy built by hand with go build alone has none, but is a
// console program itself, so it runs the hooks.
func consoleBuildBeside(program string) string {
	console := filepath.Join(filepath.Dir(program), "smolcorp-cli.exe")
	if _, err := os.Stat(console); err != nil {
		return program
	}
	return console
}

func entriesFor(agent Agent) *hookEntries {
	command := hookCommand(agent)
	switch agent {
	case Claude:
		// PermissionDenied is the one Claude Code sends when a permission is refused: the event log keeps it (Hooks.swift).
		return &hookEntries{file: filepath.Join(agentFolder(Claude), "settings.json"), events: []string{
			"SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PermissionDenied", "PostToolUse",
			"PostToolUseFailure", "Notification", "Stop", "StopFailure", "SessionEnd",
			"SubagentStart", "SubagentStop", "PreCompact", "PostCompact",
		}, handler: map[string]any{"type": "command", "command": command, "async": true}}
	case Codex:
		return &hookEntries{file: filepath.Join(agentFolder(Codex), "hooks.json"), events: []string{
			"SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse",
			"Stop", "Interrupt", "SessionEnd",
		}, handler: map[string]any{"type": "command", "command": command, "async": true, "timeout": 5}}
	case Gemini:
		// Gemini CLI waits for its hooks, and counts their timeout in milliseconds.
		return &hookEntries{file: filepath.Join(agentFolder(Gemini), "settings.json"), events: []string{
			"SessionStart", "BeforeAgent", "BeforeTool", "AfterTool", "Notification", "AfterAgent", "SessionEnd",
		}, handler: map[string]any{"type": "command", "command": command, "name": "smolcorp", "timeout": 5000}}
	case Grok:
		return &hookEntries{file: filepath.Join(agentFolder(Grok), "hooks", "smolcorp.json"), events: []string{
			"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
			"Notification", "Stop", "StopFailure", "StopCancelled", "SessionEnd",
		}, handler: map[string]any{"type": "command", "command": command}, isOwnFile: true}
	}
	return nil // OpenCode: a plugin, see writePlugin
}

// foundAgents are the agents with a settings folder here.
func foundAgents() []Agent {
	var found []Agent
	for _, agent := range agents {
		if _, err := os.Stat(agentFolder(agent)); err == nil {
			found = append(found, agent)
		}
	}
	return found
}

func isConnected(agent Agent) bool {
	entries := entriesFor(agent)
	if entries == nil {
		_, err := os.Stat(openCodePlugin())
		_, old := os.Stat(oldOpenCodePlugin())
		return err == nil || old == nil
	}
	settings, err := readSettings(entries.file)
	if err != nil {
		return false
	}
	hooks, _ := settings["hooks"].(map[string]any)
	for _, event := range entries.events {
		if !hasOurs(hooks[event]) {
			return false
		}
	}
	return true
}

// allConnected: every agent found is connected.
func allConnected() bool {
	found := foundAgents()
	for _, agent := range found {
		if !isConnected(agent) {
			return false
		}
	}
	return len(found) > 0
}

var errNoAgents = errors.New("No coding agent found on this computer. Install one, open it once, then connect again.")

// errNoAgentsMac is errNoAgents as the Mac says it (Hooks.swift), under the Mac app.
var errNoAgentsMac = errors.New("No coding agent found on this Mac. Install one, open it once, then connect again.")

// noAgentsFound is errNoAgents in this computer's words.
func noAgentsFound() error {
	if host != nil {
		return errNoAgentsMac
	}
	return errNoAgents
}

// errTestCopyAgents is a test copy's no, in the Mac's words (Hooks.HookError.testCopy): a copy on a port of its own
// never writes your agents' settings, which are the real Smolcorp's. Its hires report through a hook of their own.
var errTestCopyAgents = errors.New("This is a test copy of Smolcorp: it doesn't change your agents' settings. Report to it with a hook of its own.")

// leavesAgentsAlone is whether this copy never writes your agents' settings (~/.claude, ~/.codex, ~/.gemini,
// OpenCode's plugin, Grok's hooks): a test copy, on any computer. Every writer below asks it first.
func leavesAgentsAlone() bool {
	return testCopyPort() != 0
}

// connectAgents connects every agent found. One whose settings can't be read
// is left alone, and named in the error once the others are done.
// The notice returned says so when connecting turned Claude Code's voice on;
// with turnOnVoice false (the repair) it never does.
func connectAgents(turnOnVoice bool) (string, error) {
	if leavesAgentsAlone() {
		return "", errTestCopyAgents
	}
	found := foundAgents()
	if len(found) == 0 {
		return "", noAgentsFound()
	}
	var unreadable []string
	notice := ""
	for _, agent := range found {
		voice, err := connectVoice(agent, turnOnVoice)
		if errors.Is(err, errUnreadable) {
			unreadable = append(unreadable, entriesFor(agent).file)
		} else if err != nil {
			return notice, err
		} else if voice {
			notice = voiceNotice
		}
	}
	if len(unreadable) > 0 {
		return notice, unreadableError(unreadable)
	}
	return notice, nil
}

// voiceNotice is what the setup page tells you when connecting turned voice on.
const voiceNotice = "Claude Code's voice input is now on, in tap mode, for the phone's mic. Disconnecting turns it off again; a voice setting of your own is never changed."

// voiceMarker is the file in Smolcorp's own folder that says Smolcorp turned
// Claude Code's voice on, so removing the hooks knows it may turn it off.
func voiceMarker() string { return filepath.Join(supportDir, "voice-set-by-smolcorp") }

// smolcorpVoice is the voice entry connecting writes.
func smolcorpVoice() map[string]any {
	return map[string]any{"enabled": true, "mode": "tap"}
}

var errUnreadable = errors.New("settings can't be read or aren't valid JSON")

// connect adds the deck's entries for agent, replacing any it had. With
// turnOnVoice, Claude Code's voice input is turned on in tap mode — which the
// mic key needs — unless voice is already set up some other way.
func connect(agent Agent, turnOnVoice bool) error {
	_, err := connectVoice(agent, turnOnVoice)
	return err
}

// connectVoice is connect, and says whether it turned voice on.
func connectVoice(agent Agent, turnOnVoice bool) (bool, error) {
	if leavesAgentsAlone() {
		return false, errTestCopyAgents
	}
	entries := entriesFor(agent)
	if entries == nil {
		if err := writePlugin(); err != nil {
			return false, err
		}
		// Under the name the app had before, it's gone now, so each event is reported once
		_ = os.Remove(oldOpenCodePlugin())
		return false, nil
	}
	if agent == Grok {
		_ = os.Remove(oldGrokHooks())
	}
	settings := map[string]any{}
	var before []byte
	if entries.isOwnFile {
		before, _ = os.ReadFile(entries.file)
	} else {
		read, err := readSettings(entries.file)
		if err != nil {
			return false, errUnreadable
		}
		settings = read
		before, _ = encodeSettings(read)
	}
	hooks := withoutOurs(settings["hooks"])
	for _, event := range entries.events {
		groups, _ := hooks[event].([]any)
		hooks[event] = append(groups, map[string]any{"hooks": []any{entries.handler}})
	}
	settings["hooks"] = hooks
	voiceOn := false
	if _, set := settings["voice"]; turnOnVoice && agent == Claude {
		if !set {
			settings["voice"] = smolcorpVoice()
			voiceOn = true
		} else {
			// A voice setting Smolcorp didn't just write: an old marker can't vouch for it
			_ = os.Remove(voiceMarker())
		}
	}
	// Already as they should be, as on most starts: left untouched, so the
	// backup still holds the file from before the deck's last change.
	if after, err := encodeSettings(settings); err == nil && bytes.Equal(after, before) {
		return false, nil
	}
	if err := writeSettings(settings, entries.file, !entries.isOwnFile); err != nil {
		return false, err
	}
	if voiceOn {
		// Settings first, then the marker: a failed marker leaves voice on, never turns off one that was yours
		_ = writePrivate(voiceMarker(), []byte("1"))
	}
	return voiceOn, nil
}

func disconnectAgents() error {
	if leavesAgentsAlone() {
		return errTestCopyAgents
	}
	for _, agent := range agents {
		entries := entriesFor(agent)
		switch {
		case entries == nil:
			forgetOpenCode()
		case entries.isOwnFile:
			_ = os.Remove(entries.file)
			_ = os.Remove(oldGrokHooks())
		default:
			if _, err := os.Stat(entries.file); err != nil {
				continue
			}
			settings, err := readSettings(entries.file)
			if err != nil {
				return unreadableError([]string{entries.file})
			}
			if hooks := withoutOurs(settings["hooks"]); len(hooks) > 0 {
				settings["hooks"] = hooks
			} else {
				delete(settings, "hooks")
			}
			// Voice goes off only if Smolcorp turned it on and it's still as written
			_, marked := os.Stat(voiceMarker())
			if marked == nil && agent == Claude && reflect.DeepEqual(settings["voice"], any(smolcorpVoice())) {
				delete(settings, "voice")
			}
			if err := writeSettings(settings, entries.file, true); err != nil {
				return err
			}
			// Only once the settings are written: a failed write keeps the marker
			if marked == nil && agent == Claude {
				_ = os.Remove(voiceMarker())
			}
		}
	}
	return nil
}

// refreshHooks rewrites the entries of the agents already connected, so they
// point at this copy of the program wherever it has been moved or updated to,
// and follow every event this version does, even those an earlier version
// didn't. Voice is left alone: only connecting turns it on. An agent whose
// entries name another office's port is left as it is, so two offices on one
// computer never move each other's hooks: only connecting replaces them.
// A test copy refreshes none: they're the real Smolcorp's.
func refreshHooks() {
	if leavesAgentsAlone() {
		return
	}
	for _, agent := range foundAgents() {
		if namesAnotherOffice(agent) {
			continue
		}
		// Grok's hooks in their own file under the name the app had before count too
		_, old := os.Stat(oldGrokHooks())
		if hasEntries(agent) || (agent == Grok && old == nil) {
			_ = connect(agent, false)
		}
	}
}

// namesAnotherOffice: one of the deck's entries for agent names an explicit,
// valid port other than this office's.
func namesAnotherOffice(agent Agent) bool {
	entries := entriesFor(agent)
	if entries == nil {
		plugin, _ := os.ReadFile(openCodePlugin())
		match := openCodePortPattern.FindSubmatch(plugin)
		if match == nil {
			return false
		}
		port := portInCommand(" hook --port " + string(match[1]))
		return port != 0 && port != Port
	}
	settings, err := readSettings(entries.file)
	if err != nil {
		return false
	}
	hooks, _ := settings["hooks"].(map[string]any)
	for _, groups := range hooks {
		list, _ := groups.([]any)
		for _, group := range list {
			g, _ := group.(map[string]any)
			handlers, _ := g["hooks"].([]any)
			for _, handler := range handlers {
				h, _ := handler.(map[string]any)
				command, _ := h["command"].(string)
				if port := portInCommand(command); isOurs(handler) && port != 0 && port != Port {
					return true
				}
			}
		}
	}
	return false
}

// openCodePortPattern finds the port in the command of OpenCode's plugin.
var openCodePortPattern = regexp.MustCompile(`"--port",\s*"(\d+)"`)

// portInCommand is the valid port a hook command names after hook --port, or 0.
func portInCommand(command string) int {
	i := strings.LastIndex(command, " hook --port ")
	if i < 0 {
		return 0
	}
	number, _, _ := strings.Cut(command[i+len(" hook --port "):], " ")
	if n, err := strconv.Atoi(number); err == nil && n >= 1024 && n <= 65535 {
		return n
	}
	return 0
}

// hasEntries: the deck is in agent's hooks for at least one event.
func hasEntries(agent Agent) bool {
	entries := entriesFor(agent)
	if entries == nil {
		return isConnected(agent)
	}
	settings, err := readSettings(entries.file)
	if err != nil {
		return false
	}
	hooks, _ := settings["hooks"].(map[string]any)
	for _, groups := range hooks {
		if hasOurs(groups) {
			return true
		}
	}
	return false
}

// writePlugin writes OpenCode's plugin, which runs the hook: OpenCode has no
// command hooks. It's written for the OpenCode installed, see
// openCodePluginTemplate, whole or not at all, and only when it changes, as
// OpenCode loads it again each time it does.
func writePlugin() error {
	openCodeMu.Lock()
	defer openCodeMu.Unlock()
	return writePluginLocked()
}

// writePluginLocked is writePlugin, with openCodeMu held.
func writePluginLocked() error {
	if leavesAgentsAlone() {
		return errTestCopyAgents
	}
	stamp := openCodeStamp()
	template, err := fs.ReadFile(agentdeck.Files, openCodePluginTemplate())
	if err != nil {
		return err
	}
	command, _ := json.Marshal([]string{commandLineProgram(), "hook", "--port", fmt.Sprint(Port), string(OpenCode)})
	plugin := bytes.ReplaceAll(template, []byte(`["{{COMMAND}}"]`), command)
	openCodeWrittenFor = stamp
	if written, err := os.ReadFile(openCodePlugin()); err == nil && bytes.Equal(written, plugin) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(openCodePlugin()), 0o755); err != nil {
		return err
	}
	// Whole or not at all: OpenCode, watching the file, must never load half of it
	temp, err := os.CreateTemp(filepath.Dir(openCodePlugin()), ".smolcorp-*.js.tmp")
	if err != nil {
		return err
	}
	_, err = temp.Write(plugin)
	if closed := temp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Chmod(temp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(temp.Name(), openCodePlugin())
	}
	if err != nil {
		_ = os.Remove(temp.Name())
	}
	return err
}

// isOurs recognises the deck's handler by its command, as the deck writes
// it: a program of the deck's, quoted or not, then hook, then at most an
// agent's name. The program is this copy, or one by the deck's name,
// wherever it was when it was written, under this name or the one it had
// before, so an update replaces the old entries rather than adding to them.
// On Windows that path may be in its short form, where smolcorp-cli.exe reads
// SMOLCO~1.EXE and sauzzedeck.exe SAUZZE~1.EXE. A hook of your own that only
// mentions Smolcorp, as notify-send "smolcorp hook done", is yours.
func isOurs(handler any) bool {
	h, _ := handler.(map[string]any)
	command, _ := h["command"].(string)
	if isMacReportScript(command) {
		return true
	}
	program, found := strings.CutSuffix(command, " hook")
	if !found {
		i := strings.LastIndex(command, " hook ")
		if i < 0 || !isHookArguments(command[i+len(" hook "):]) {
			return false
		}
		program = command[:i]
	}
	if inside, found := strings.CutPrefix(program, `'`); found {
		// Single-quoted, as shellPath writes it elsewhere than Windows.
		path, ok := unquoteSingle(inside)
		return ok && strings.HasPrefix(path, "/") && (path == thisCopysPath() || isDecksProgram(path))
	}
	quoted := false
	if inside, found := strings.CutPrefix(program, `"`); found {
		if program, quoted = strings.CutSuffix(inside, `"`); !quoted {
			return false
		}
	}
	if !isAProgramsPath(program, quoted) {
		return false
	}
	// Unquoted with a & in it, as only an earlier version wrote a Windows path,
	// and only this copy's own: nothing else can tell a & in a folder's name
	// from a & before a program of the deck's, relative or not.
	if !quoted && strings.Contains(program, "&") {
		return program == thisCopysPath()
	}
	return program == thisCopysPath() || isDecksProgram(program)
}

// isMacReportScript is whether command is the Swift engine's own entry, under the Mac app only: its
// report.sh in this support folder, quoted, with the agent's name after it for any agent but Claude Code and
// Grok (Hooks.command(for:)). It counts as the deck's, so the start-up refresh puts `smolcorp hook` in its
// place, connecting replaces it rather than adding a second entry, and disconnecting takes it out. The
// script itself stays: a hire the Swift engine made still runs it from its own settings file.
func isMacReportScript(command string) bool {
	if hookHeaderFile == "" {
		return false
	}
	rest, found := strings.CutPrefix(command, `"`+filepath.Join(supportDir, "report.sh")+`"`)
	if !found {
		return false
	}
	agent, spaced := strings.CutPrefix(rest, " ")
	return rest == "" || spaced && Agent(agent) != Claude && Agent(agent) != Grok && slices.Contains(agents, Agent(agent))
}

// isHookArguments is what follows hook in the deck's command: at most
// --port and a port, then at most an agent's name. The port is any valid one,
// not only this office's: the program is what makes a hook the deck's, and an
// agent's settings hold one office's hook per event, so connecting from
// another office replaces it, as it replaced one with another path before.
func isHookArguments(rest string) bool {
	if after, found := strings.CutPrefix(rest, "--port "); found {
		number, agent, _ := strings.Cut(after, " ")
		if _, err := strconv.Atoi(number); err != nil || number != fmt.Sprint(portFromEnvironment(number)) {
			return false
		}
		rest = agent
		if rest == "" && !strings.HasSuffix(after, " ") {
			return true
		}
	}
	return slices.Contains(agents, Agent(rest))
}

// thisCopysPath is this copy's path as its hooks name it, unquoted: on
// Windows in its short form, with forward slashes (see shellPath).
func thisCopysPath() string {
	if inside, found := strings.CutPrefix(shellPath(commandLineProgram()), `'`); found {
		path, _ := unquoteSingle(inside)
		return path
	}
	return strings.TrimSuffix(strings.TrimPrefix(shellPath(commandLineProgram()), `"`), `"`)
}

// unquoteSingle undoes shellPath's single quotes on what follows the opening
// one: it reads up to the closing quote, taking quote, backslash, quote, quote as a quote of the path, and
// is false if anything but that closing quote ends it.
func unquoteSingle(inside string) (string, bool) {
	var path strings.Builder
	for {
		i := strings.IndexByte(inside, '\'')
		if i < 0 {
			return "", false
		}
		path.WriteString(inside[:i])
		inside = inside[i+1:]
		if inside == "" {
			return path.String(), true
		}
		rest, found := strings.CutPrefix(inside, `\''`)
		if !found {
			return "", false
		}
		path.WriteByte('\'')
		inside = rest
	}
}

// isAProgramsPath is whether path is a program's whole path, as the deck
// writes it: from the root or a drive, with nothing a shell would read as
// more than a path. On Windows the path is quoted for a space or & ( ) '
// (see shellPath); unquoted, from an earlier version, it may hold ( ) ' ! as
// any folder's name may, but ; | < > or a second program's path chain
// another command to it (a & is isOurs's to judge). Elsewhere it's quoted,
// and unquoted only in entries from before, with no space and none of a
// shell's own characters.
func isAProgramsPath(path string, quoted bool) bool {
	if strings.ContainsAny(path, "\"\n") {
		return false
	}
	switch {
	case quoted:
		// Between double quotes, only $ and ` are a shell's own.
		return (strings.HasPrefix(path, "/") || windowsDrivePath.MatchString(path)) && !strings.ContainsAny(path, "$`")
	case windowsDrivePath.MatchString(path):
		// No Windows name holds a colon: one past the drive's is another path's.
		return !strings.ContainsAny(path, ";|<>`") && !strings.Contains(path[2:], ":") && !secondDrivePath.MatchString(path)
	case strings.HasPrefix(path, "/"):
		return !strings.ContainsAny(path, " '#;&|<>(){}*?!$`")
	}
	return false
}

// windowsDrivePath is a path from a Windows drive, with either slash.
var windowsDrivePath = regexp.MustCompile(`^[A-Za-z]:[/\\]`)

// secondDrivePath is another path from a drive, past the first: an argument,
// or a program after one.
var secondDrivePath = regexp.MustCompile(`\s[A-Za-z]:[/\\]`)

// windowsPathNeedsQuotes is whether a Windows hook's path, in its short form,
// must be quoted to run through cmd (Codex) and Git Bash (Claude Code): a
// drive with short names turned off keeps its spaces, and a short name keeps
// & ( ) ', which both read as their own, and ^, which cmd takes out. A lone !
// is safe unquoted in both.
// PowerShell (Gemini CLI, Grok) runs neither a quoted path nor one with
// those in it: see STATUS, a later item.
func windowsPathNeedsQuotes(path string) bool { return strings.ContainsAny(path, " &()'^") }

// decksShortName is a Windows short name of the deck's programs, by either name.
var decksShortName = regexp.MustCompile(`^(smolco|sauzze)~\d+$`)

// isDecksProgram is whether path names one of the deck's programs, now or
// under the name it had before: smolcorp, smolcorp-cli.exe, Smolcorp.exe,
// sauzzedeck, SauzzeDeck Pro.exe, or a short name of one.
func isDecksProgram(path string) bool {
	name := strings.ToLower(path[strings.LastIndexAny(path, `/\`)+1:])
	name = strings.TrimSuffix(name, ".exe")
	return slices.Contains([]string{"smolcorp", "smolcorp-cli", "sauzzedeck", "sauzzedeck-cli", "sauzzedeck pro"}, name) ||
		decksShortName.MatchString(name)
}

func hasOurs(groups any) bool {
	list, _ := groups.([]any)
	for _, group := range list {
		g, _ := group.(map[string]any)
		handlers, _ := g["hooks"].([]any)
		for _, handler := range handlers {
			if isOurs(handler) {
				return true
			}
		}
	}
	return false
}

func withoutOurs(hooks any) map[string]any {
	result := map[string]any{}
	events, _ := hooks.(map[string]any)
	for event, groups := range events {
		var kept []any
		list, _ := groups.([]any)
		for _, group := range list {
			g, ok := group.(map[string]any)
			if !ok {
				kept = append(kept, group)
				continue
			}
			var handlers []any
			list, _ := g["hooks"].([]any)
			for _, handler := range list {
				if !isOurs(handler) {
					handlers = append(handlers, handler)
				}
			}
			if len(handlers) > 0 {
				copied := map[string]any{}
				for k, v := range g {
					copied[k] = v
				}
				copied["hooks"] = handlers
				kept = append(kept, copied)
			}
		}
		if len(kept) > 0 {
			result[event] = kept
		}
	}
	return result
}

// readSettings reads an agent's settings. Only a file that isn't there at all
// is empty: one that is there but can't be read, or a link to nothing, is an
// error, so it's never written over with the deck's entries alone.
func readSettings(file string) (map[string]any, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		if _, missing := os.Lstat(file); errors.Is(err, fs.ErrNotExist) && errors.Is(missing, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber() // numbers go back exactly as they were
	settings := map[string]any{}
	if err := decoder.Decode(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// The backups of a settings file, beside it: the file as it was before the
// deck first changed it, kept from then on, and as it was before the latest
// change.
const (
	firstBackup = ".agentdeck.bak"
	lastBackup  = ".agentdeck-last.bak"
)

// writeSettings writes an agent's settings through a file beside them, so a
// crash leaves the old file or the new one, never half of one. A link is
// followed, so the file it points to, in a dotfiles folder say, is the one
// that changes. The backups are made first, or nothing is written.
func writeSettings(settings map[string]any, file string, backup bool) error {
	if leavesAgentsAlone() {
		return errTestCopyAgents
	}
	if resolved, err := filepath.EvalSymlinks(file); err == nil {
		file = resolved
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
		if backup {
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			if err := keepFirst(file+firstBackup, data, mode); err != nil {
				return err
			}
			if err := os.WriteFile(file+lastBackup, data, mode); err != nil {
				return err
			}
		}
	}
	out, err := encodeSettings(settings)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name()) // gone once renamed; left behind only by a failure
	if _, err := temp.Write(out); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(temp.Name(), file)
}

// keepFirst writes data to backup, whole, unless there's one there already.
func keepFirst(backup string, data []byte, mode fs.FileMode) error {
	if _, err := os.Lstat(backup); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(backup), filepath.Base(backup)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name()) // gone once renamed
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(temp.Name(), backup)
}

func encodeSettings(settings map[string]any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(settings); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func unreadableError(files []string) error {
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = trimmed(file)
	}
	return fmt.Errorf("%s can't be read or isn't valid JSON. Fix it, then try again.", strings.Join(names, " and "))
}
