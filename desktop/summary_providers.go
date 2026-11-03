package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ✨ Summary made by a coding agent you already have, on your own plan,
// instead of with an API key: Claude Code, Codex, Gemini CLI, Grok Build,
// Cursor or OpenCode, run once per summary, headless and with no tools, as
// the Mac app's SummaryProviders.swift does.
//
// Each is run in an empty folder of its own, made for the run and deleted
// after it, with the same redacted excerpt and prompt the API key gets, on
// its standard input (never its command line), for at most its timeout and
// summaryOutputMost bytes. Its environment is Smolcorp's, less the providers'
// API keys, so it can only use the plan you signed in with, and with
// SMOLCORP_SUMMARY=1, which Smolcorp's own hook ignores, so the run never
// gets a desk. That one process is all this changes: your shell, your
// terminals, the agents Smolcorp hires and Smolcorp itself keep theirs.

// summaryRunVariable marks a summary's process, for the hook to ignore.
const summaryRunVariable = "SMOLCORP_SUMMARY"

type summaryProvider string

const (
	providerClaude   summaryProvider = "claude"
	providerCodex    summaryProvider = "codex"
	providerGemini   summaryProvider = "gemini"
	providerGrok     summaryProvider = "grok"
	providerCursor   summaryProvider = "cursor"
	providerOpenCode summaryProvider = "opencode"
	providerAPIKey   summaryProvider = "apikey"
)

// summaryAgents are the coding agents on this computer's system, in the
// order the first that works is picked. Grok Build has no Windows version,
// and Cursor's CLI runs on Windows only inside WSL: hidden there.
func summaryAgents(goos string) []summaryProvider {
	agents := []summaryProvider{providerClaude, providerCodex, providerGemini, providerGrok, providerCursor, providerOpenCode}
	if goos == "windows" {
		agents = slices.DeleteFunc(agents, func(p summaryProvider) bool { return p == providerGrok || p == providerCursor })
	}
	return agents
}

// supported is whether every tool it has can be turned off, verified: only
// these are run. The others' own docs say a headless run keeps some (Gemini's
// plan mode switches to YOLO and keeps web search, Codex's read-only sandbox
// still runs a shell that reads your home, Cursor's -p has every tool, Grok's
// is a deny list with MCP on and no network block on macOS), and an excerpt
// can carry text an agent fetched from a page.
func (p summaryProvider) supported() bool {
	return p == providerClaude || p == providerOpenCode || p == providerAPIKey
}

const unsupportedReason = "not supported yet: can't turn its tools off safely"

func validSummaryProvider(id string) (summaryProvider, bool) {
	p := summaryProvider(id)
	return p, p == providerAPIKey || slices.Contains(summaryAgents("darwin"), p)
}

func (p summaryProvider) name() string {
	return map[summaryProvider]string{providerClaude: "Claude Code", providerCodex: "Codex", providerGemini: "Gemini CLI",
		providerGrok: "Grok Build", providerCursor: "Cursor", providerOpenCode: "OpenCode", providerAPIKey: "API key"}[p]
}

// plan is what it costs, in setup's words.
func (p summaryProvider) plan() string {
	return map[summaryProvider]string{providerClaude: "uses your Claude plan", providerCodex: "uses your ChatGPT plan",
		providerGemini: "uses your Gemini plan", providerGrok: "uses your Grok plan", providerCursor: "uses your Cursor plan",
		providerOpenCode: "uses your OpenCode setup (its default model)", providerAPIKey: "Anthropic, billed to that key"}[p]
}

// destination is where the summary input goes, for setup's privacy line.
func (p summaryProvider) destination() string {
	if p == providerAPIKey {
		return "Anthropic's API, with your key"
	}
	return p.name() + " under your own account"
}

// programs are the program's names, the first found wins. Cursor's own
// installer names it agent now, and cursor-agent before; Grok Build installs
// an agent too.
func (p summaryProvider) programs() []string {
	switch p {
	case providerCursor:
		return []string{"cursor-agent", "agent"}
	case providerAPIKey:
		return nil
	}
	return []string{string(p)}
}

func (p summaryProvider) signIn() string {
	return map[summaryProvider]string{providerClaude: "run `claude` and log in", providerCodex: "run `codex login`",
		providerGemini: "run `gemini` once and sign in with Google", providerGrok: "run `grok login`",
		providerCursor: "run `cursor-agent login`", providerOpenCode: "run `opencode auth login`"}[p]
}

// helpArguments ask for its help, whose text must still list every flag a summary passes.
func (p summaryProvider) helpArguments() []string {
	switch p {
	case providerCodex:
		return []string{"exec", "--help"}
	case providerOpenCode:
		return []string{"run", "--help"}
	}
	return []string{"--help"}
}

// MARK: One run

// cliFailure is why a coding agent made no summary.
type cliFailure int

const (
	cliOK cliFailure = iota
	cliMissing
	cliSignedOut
	cliQuota
	cliTimeout
	cliChanged
	cliEmpty
	cliFailed
	cliUnsupported
	cliUsedTool
	// Its login now pays another way than setup showed (a plan, or a Console account).
	cliLoginChanged
)

func (f cliFailure) err(p summaryProvider) *SummaryError {
	name := p.name()
	switch f {
	case cliMissing:
		return &SummaryError{name + " isn't on this computer any more. Pick another in setup.", 503}
	case cliSignedOut:
		return &SummaryError{name + " isn't signed in. Sign in on your computer: " + p.signIn() + ".", 502}
	case cliQuota:
		return &SummaryError{"Your " + name + " plan's limit is reached. Try again later.", 429}
	case cliTimeout:
		return &SummaryError{name + " didn't answer in time. Try again.", 504}
	case cliChanged:
		return &SummaryError{name + " has changed: summaries with it need an update of Smolcorp.", 502}
	case cliEmpty:
		return &SummaryError{name + " sent back nothing.", 502}
	case cliUnsupported:
		return &SummaryError{name + " isn't supported yet for summaries: pick another in setup.", 503}
	case cliUsedTool:
		return &SummaryError{name + " tried to use a tool, so its answer was thrown away.", 502}
	case cliLoginChanged:
		return &SummaryError{name + "'s login has changed since setup showed it: check it in setup, on your computer.", 409}
	}
	return &SummaryError{name + " couldn't make a summary.", 502}
}

// short is a few words for a summary made with the key instead.
func (f cliFailure) short(p summaryProvider) string {
	name := p.name()
	switch f {
	case cliMissing:
		return name + " wasn't found"
	case cliSignedOut:
		return name + " isn't signed in"
	case cliQuota:
		return "your " + name + " plan's limit is reached"
	case cliTimeout:
		return name + " didn't answer in time"
	case cliChanged:
		return name + " has changed"
	case cliUnsupported:
		return name + " isn't supported yet"
	case cliUsedTool:
		return name + " tried to use a tool"
	case cliLoginChanged:
		return name + "'s login has changed"
	}
	return name + " failed"
}

// summaryFallbackNote is the line a summary made with the key instead ends with.
func summaryFallbackNote(f cliFailure, p summaryProvider) string {
	why := []rune(f.short(p))
	return "(" + string(unicode.ToUpper(why[0])) + string(why[1:]) + ", so this was made with your API key.)"
}

// summaryInvocation is what a run needs: its arguments, what goes on its
// standard input, files written in its folder first, and where its answer is.
type summaryInvocation struct {
	args       []string
	input      []byte
	files      map[string][]byte
	outputFile string
}

const (
	summaryCLITimeout      = 45 * time.Second
	summaryOpenCodeTimeout = 90 * time.Second // a server of its own, and maybe a model on this computer
	summaryOutputMost      = 64 << 10
	summaryErrorMost       = 8 << 10
	summaryLinesMost       = 5
	summaryCharactersMost  = 600
)

// summaryRemoved are variables a summary's process never gets: with them, a
// CLI would bill that key rather than use your plan, or talk to somewhere
// else; and those that would have its hook take it for a desk's session.
// Only that process: nothing else Smolcorp starts loses them.
var summaryRemoved = []string{
	"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL",
	"OPENAI_API_KEY", "CODEX_API_KEY", "OPENAI_BASE_URL",
	"GEMINI_API_KEY", "GOOGLE_API_KEY", "XAI_API_KEY", "CURSOR_API_KEY",
	// Or another provider's cloud, billed elsewhere (and any ANTHROPIC_DEFAULT_*, below).
	"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "AWS_BEARER_TOKEN_BEDROCK",
	"ANTHROPIC_CUSTOM_HEADERS", "GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_CLOUD_PROJECT", "GOOGLE_APPLICATION_CREDENTIALS",
	"AZURE_OPENAI_API_KEY",
	"SMOLCORP_SESSION", "SAUZZEDECK_SESSION", "SMOLCORP_HEADLESS", "SMOLCORP_PARENT_FOLDER",
	"TMUX", "TMUX_PANE", "WEZTERM_PANE", "WEZTERM_UNIX_SOCKET", "KITTY_WINDOW_ID", "KITTY_LISTEN_ON",
}

// summaryEnvironment is a summary's process's environment, from base (Smolcorp's).
func summaryEnvironment(base []string) []string {
	env := slices.DeleteFunc(slices.Clone(base), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		name = strings.ToUpper(name)
		return slices.Contains(summaryRemoved, name) || strings.HasPrefix(name, "ANTHROPIC_DEFAULT_") || name == summaryRunVariable
	})
	return append(env, summaryRunVariable+"=1")
}

func summaryCLITimeoutOf(p summaryProvider) time.Duration {
	if p == providerOpenCode {
		return summaryOpenCodeTimeout
	}
	return summaryCLITimeout
}

// summaryInvocationOf is the run of p for a summary, in folder, on this computer's system.
func summaryInvocationOf(p summaryProvider, system, prompt, folder string) summaryInvocation {
	return summaryInvocationFor(runtime.GOOS, p, system, prompt, folder)
}

// summaryInvocationFor is the run of p on goos. On Windows, cmd.exe can't
// carry the instructions' several lines, so they go first on its input.
func summaryInvocationFor(goos string, p summaryProvider, system, prompt, folder string) summaryInvocation {
	// For those with no flag for the instructions, they come first on its input.
	both := []byte(system + "\n\n" + prompt)
	switch p {
	case providerClaude:
		// --setting-sources "" also leaves out your hooks, --tools "" every tool, and --bare can't be used:
		// it ignores the login, which is the plan.
		args := []string{
			"--print", "--model", "haiku", "--tools", "", "--strict-mcp-config", "--setting-sources", "",
			"--disable-slash-commands", "--no-session-persistence", "--permission-mode", "dontAsk",
			"--output-format", "json",
		}
		if goos == "windows" {
			return summaryInvocation{args: args, input: both}
		}
		return summaryInvocation{args: append(args, "--system-prompt", system), input: []byte(prompt)}
	case providerCodex:
		// Read-only is exec's own sandbox; --ignore-user-config leaves out your config.toml's MCP servers.
		return summaryInvocation{args: []string{
			"exec", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--ignore-user-config",
			"--ignore-rules", "--cd", folder, "--output-last-message", filepath.Join(folder, "summary.txt"), "-",
		}, input: both, outputFile: "summary.txt"}
	case providerGemini:
		// Plan mode is read-only; no extension and no MCP server ("none" names none of them).
		return summaryInvocation{args: []string{
			"--prompt", "Write the summary, as the instructions above say.", "--approval-mode", "plan",
			"--extensions", "none", "--allowed-mcp-server-names", "none", "--output-format", "json",
		}, input: both}
	case providerGrok:
		return summaryInvocation{args: []string{
			"--prompt-file", filepath.Join(folder, "prompt.txt"), "--cwd", folder, "--sandbox", "strict",
			"--disallowed-tools", "run_terminal_cmd,write,search_replace,read_file,grep,list_dir,web_search,web_fetch,task,Agent",
			"--disable-web-search", "--no-subagents", "--max-turns", "1",
			"--system-prompt-override", system, "--output-format", "json",
		}, files: map[string][]byte{"prompt.txt": []byte(prompt)}}
	case providerCursor:
		// Ask mode is read-only; without --force or --approve-mcps, nothing is written and no MCP server runs.
		return summaryInvocation{args: []string{
			"--print", "--mode", "ask", "--sandbox", "enabled", "--trust", "--workspace", folder, "--output-format", "json",
		}, input: both}
	case providerOpenCode:
		// OpenCode 2 ignores a config given for one run, so its own summary agent, which every tool is denied.
		return summaryInvocation{args: []string{"run", "--standalone", "--agent", "summary", "--format", "json"}, input: both}
	}
	return summaryInvocation{}
}

// summaryFlags are the long flags a run passes, which its help must still list.
func summaryFlags(p summaryProvider) []string {
	var flags []string
	for _, arg := range summaryInvocationOf(p, "", "", "f").args {
		if strings.HasPrefix(arg, "--") {
			flags = append(flags, arg)
		}
	}
	return flags
}

// MARK: Its answer

type summaryRan struct {
	status   int // -1: it never ran, or was killed
	timedOut bool
	output   []byte
	errors   []byte
	file     []byte
}

// summaryAnswer is the summary from what the run printed, or why there's none.
func summaryAnswer(p summaryProvider, ran summaryRan) (string, cliFailure) {
	if ran.timedOut {
		return "", cliTimeout
	}
	var text, failure string
	switch p {
	case providerCodex:
		text = string(ran.file)
	case providerOpenCode:
		// A tool it ran, though its summary agent is denied every one: nothing it said is kept.
		if openCodeUsedTool(string(ran.output)) {
			return "", cliUsedTool
		}
		// Cut short or unreadable, it can't be told whether a tool ran: nothing is kept either.
		if len(ran.output) >= summaryOutputMost || openCodeUnreadable(string(ran.output)) {
			return "", cliFailed
		}
		text, failure = openCodeAnswerText(string(ran.output))
	default:
		text, failure = jsonAnswerText(string(ran.output))
	}
	if ran.status != 0 || failure != "" {
		return "", classifyCLIFailure(failure + "\n" + string(ran.errors) + "\n" + text)
	}
	// Its answer is redacted as the excerpt was, before it's shown or kept.
	if cleaned := cleanSummary(redactText(text)); cleaned != "" {
		return cleaned, cliOK
	}
	return "", cliEmpty
}

// jsonAnswerText is the text of a JSON answer (Claude Code, Grok, Gemini,
// Cursor): one object, or lines of them, the last with text winning. Also an
// error's message, when it says it's one.
func jsonAnswerText(printed string) (text, failure string) {
	var objects []map[string]any
	if object, ok := parseObject(printed); ok {
		objects = []map[string]any{object}
	} else {
		for _, line := range strings.Split(printed, "\n") {
			if object, ok := parseObject(line); ok {
				objects = append(objects, object)
			}
		}
	}
	for _, object := range objects {
		if object["type"] == "error" || object["is_error"] == true {
			failure = firstString(object["message"], object["result"], object["error"], "error")
			continue
		}
		if e, ok := object["error"]; ok && e != nil {
			if m, ok := e.(map[string]any); ok {
				failure = firstString(m["message"], "error")
			} else {
				failure = firstString(e, "error")
			}
		}
		if found := firstString(object["result"], object["response"], object["text"]); found != "" {
			text = found
		}
	}
	// Not JSON at all: taken as it is, as a CLI that changed its output might print plain text.
	if len(objects) == 0 && strings.TrimSpace(printed) != "" {
		text = printed
	}
	return text, failure
}

// openCodeAnswerText is OpenCode's answer: lines of events, its text in "text" events' parts.
func openCodeAnswerText(printed string) (text, failure string) {
	var parts []string
	for _, line := range strings.Split(printed, "\n") {
		event, ok := parseObject(line)
		if !ok {
			continue
		}
		switch event["type"] {
		case "text":
			if part, ok := event["part"].(map[string]any); ok {
				if t := firstString(part["text"]); t != "" {
					parts = append(parts, t)
				}
			}
		case "error":
			if e, ok := event["error"].(map[string]any); ok {
				failure = firstString(e["message"], "error")
			} else {
				failure = firstString(event["message"], "error")
			}
		}
	}
	return strings.Join(parts, "\n"), failure
}

// openCodeUsedTool is whether OpenCode's events show a tool being run.
func openCodeUsedTool(printed string) bool {
	for _, line := range strings.Split(printed, "\n") {
		event, ok := parseObject(line)
		if !ok {
			continue
		}
		kinds := []any{event["type"]}
		if part, ok := event["part"].(map[string]any); ok {
			kinds = append(kinds, part["type"])
		}
		for _, kind := range kinds {
			if k, ok := kind.(string); ok && strings.Contains(strings.ToLower(k), "tool") {
				return true
			}
		}
	}
	return false
}

// openCodeUnreadable is whether a line of OpenCode's events starts as JSON and doesn't read as it.
func openCodeUnreadable(printed string) bool {
	for _, line := range strings.Split(printed, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "{") {
			if _, ok := parseObject(line); !ok {
				return true
			}
		}
	}
	return false
}

// openCodeSummaryAgentIsLocked is whether OpenCode's agents, as `opencode
// debug agents` lists them, have a summary agent denied every tool: a rule
// denying everything, and no rule after it allowing or asking anything.
// Without one, --agent summary could quietly run another agent, with tools.
func openCodeSummaryAgentIsLocked(listed []byte) bool {
	var agents []struct {
		ID          string `json:"id"`
		Permissions []struct {
			Action, Resource, Effect string
		} `json:"permissions"`
	}
	if json.Unmarshal(listed, &agents) != nil {
		return false
	}
	for _, agent := range agents {
		if agent.ID != "summary" {
			continue
		}
		all := -1
		for i, rule := range agent.Permissions {
			if rule.Action == "*" && rule.Resource == "*" && rule.Effect == "deny" {
				all = i
			}
		}
		if all < 0 {
			return false
		}
		for _, rule := range agent.Permissions[all+1:] {
			if rule.Effect != "deny" {
				return false
			}
		}
		return true
	}
	return false
}

// claudeLogin is what a Claude Code login pays with, from `claude auth
// status --json`: a plan, an API account, or "" when not signed in.
func claudeLogin(status []byte) providerKind {
	var login struct {
		LoggedIn         bool    `json:"loggedIn"`
		AuthMethod       string  `json:"authMethod"`
		SubscriptionType *string `json:"subscriptionType"`
	}
	if json.Unmarshal(status, &login) != nil || !login.LoggedIn {
		return ""
	}
	if login.AuthMethod == "claude.ai" && login.SubscriptionType != nil && strings.TrimSpace(*login.SubscriptionType) != "" {
		return providerReady
	}
	return providerAPIAccount
}

// helpLists is whether help lists flag as a flag of its own, not inside another's name.
func helpLists(help, flag string) bool {
	return regexp.MustCompile(`(^|[\s,\[|(])` + regexp.QuoteMeta(flag) + `($|[\s,=\]|)<])`).MatchString(help)
}

var openCodeSessionID = regexp.MustCompile(`^ses_[A-Za-z0-9]+$`)

// openCodeSession is the session OpenCode kept for the run, to delete after it.
func openCodeSession(printed []byte) string {
	for _, line := range strings.Split(string(printed), "\n") {
		if event, ok := parseObject(line); ok {
			if id, _ := event["sessionID"].(string); openCodeSessionID.MatchString(id) {
				return id
			}
		}
	}
	return ""
}

func parseObject(text string) (map[string]any, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "{") {
		return nil, false
	}
	var object map[string]any
	return object, json.Unmarshal([]byte(text), &object) == nil
}

// firstString is the first of values that's a string with something in it.
func firstString(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

// classifyCLIFailure is what a failed run's words say went wrong. They're only matched, never shown.
func classifyCLIFailure(words string) cliFailure {
	words = strings.ToLower(words)
	has := func(phrases ...string) bool {
		return slices.ContainsFunc(phrases, func(p string) bool { return strings.Contains(words, p) })
	}
	switch {
	case has("unknown option", "unexpected argument", "unrecognized", "unknown flag", "invalid option", "unknown argument"):
		return cliChanged
	case has("usage limit", "rate limit", "rate_limit", "quota", "limit reached", "too many requests", "429", "resource_exhausted"):
		return cliQuota
	case has("not signed in", "not logged in", "please log in", "please login", "login required", "sign in", "authenticate",
		"unauthorized", "401", "invalid api key", "no credentials", "/login"):
		return cliSignedOut
	}
	return cliFailed
}

// cleanSummary is the summary as kept: no code fences, no "Here's…" line, no
// thinking, at most summaryLinesMost lines and summaryCharactersMost characters.
func cleanSummary(text string) string {
	for {
		open := strings.Index(text, "<thinking>")
		if open < 0 {
			break
		}
		end := strings.Index(text[open:], "</thinking>")
		if end < 0 {
			break
		}
		text = text[:open] + text[open+end+len("</thinking>"):]
	}
	var lines []string
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "```") {
			lines = append(lines, line)
		}
	}
	if len(lines) > 0 {
		first := strings.ToLower(lines[0])
		if strings.HasSuffix(first, ":") && slices.ContainsFunc([]string{"summary", "here", "sure", "voici", "résumé"},
			func(p string) bool { return strings.HasPrefix(first, p) }) {
			lines = lines[1:]
		}
	}
	kept := strings.Join(lines[:min(len(lines), summaryLinesMost)], "\n")
	if runes := []rune(kept); len(runes) > summaryCharactersMost {
		return string(runes[:summaryCharactersMost-1]) + "…"
	}
	return kept
}

// MARK: Running it

// runSummaryCLI runs p for a summary, in a folder made for it and deleted
// after. Its login and its summary agent are checked again just before:
// either may have changed since setup looked. expected is how setup showed a
// Claude Code login paying.
func runSummaryCLI(p summaryProvider, program, system, prompt string, base []string, expected providerKind) (string, cliFailure) {
	if !p.supported() {
		return "", cliUnsupported
	}
	folder, err := os.MkdirTemp("", "smolcorp-summary-")
	if err != nil {
		return "", cliFailed
	}
	defer os.RemoveAll(folder)
	call := summaryInvocationOf(p, system, prompt, folder)
	for name, data := range call.files {
		if os.WriteFile(filepath.Join(folder, name), data, 0o600) != nil {
			return "", cliFailed
		}
	}
	env := summaryEnvironment(base)
	switch p {
	case providerClaude:
		login := claudeLogin(spawnSummary(program, []string{"auth", "status", "--json"}, nil, env, folder, 15*time.Second).output)
		if login == "" {
			return "", cliSignedOut
		}
		if login != expected {
			return "", cliLoginChanged
		}
	case providerOpenCode:
		if !openCodeSummaryAgentIsLocked(spawnSummary(program, []string{"debug", "agents"}, nil, env, folder, 15*time.Second).output) {
			return "", cliUnsupported
		}
	}
	ran := spawnSummary(program, call.args, call.input, env, folder, summaryCLITimeoutOf(p))
	if call.outputFile != "" {
		ran.file, _ = os.ReadFile(filepath.Join(folder, call.outputFile))
	}
	if p == providerOpenCode {
		if session := openCodeSession(ran.output); session != "" {
			spawnSummary(program, []string{"session", "delete", session}, nil, env, folder, 15*time.Second)
		}
	}
	return summaryAnswer(p, ran)
}

// cappedBuffer keeps at most most bytes and drops the rest, so a run that
// prints without end is still read to its end and never blocks. Its buffer
// isn't embedded: io.Copy would use the buffer's own ReadFrom, uncapped.
type cappedBuffer struct {
	kept bytes.Buffer
	most int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.most - c.kept.Len(); room > 0 {
		c.kept.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (c *cappedBuffer) Bytes() []byte { return c.kept.Bytes() }

// spawnSummary starts program in a process group of its own, so a timeout
// ends it and anything it started, gives it input, and reads at most
// summaryOutputMost of its output and summaryErrorMost of its errors.
func spawnSummary(program string, args []string, input []byte, env []string, folder string, timeout time.Duration) summaryRan {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := summaryCommand(ctx, program, args)
	cmd.Dir = folder
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	output, errors := &cappedBuffer{most: summaryOutputMost}, &cappedBuffer{most: summaryErrorMost}
	cmd.Stdout, cmd.Stderr = output, errors
	ownGroup(cmd)
	// Ended, with all it started, only while it's still running: Cancel is
	// called only before Wait has reaped it, so its pid is still its own.
	var (
		mu    sync.Mutex
		group processGroup
	)
	cmd.Cancel = func() error {
		mu.Lock()
		defer mu.Unlock()
		return group.kill()
	}
	// Anything it left holding its output open is given up on soon after.
	cmd.WaitDelay = 2 * time.Second
	ran := summaryRan{status: -1}
	if cmd.Start() != nil {
		return ran
	}
	mu.Lock()
	group = joinGroup(cmd)
	mu.Unlock()
	err := cmd.Wait()
	// What it left running goes too: its group (Unix), whose id can't be
	// another's while any of it is left, or its job (Windows), never a pid.
	mu.Lock()
	group.close()
	mu.Unlock()
	ran.output, ran.errors, ran.timedOut = output.Bytes(), errors.Bytes(), ctx.Err() == context.DeadlineExceeded
	if err == nil {
		ran.status = 0
	} else if exit, ok := err.(*exec.ExitError); ok && !ran.timedOut {
		ran.status = exit.ExitCode()
	}
	return ran
}

// summaryCommand is the command for program: on Windows, an npm .cmd script
// goes through cmd.exe, which can't start one by itself, with the command
// line launcher.go's cmdLine makes (/d /s, each argument quoted and ^-escaped).
func summaryCommand(ctx context.Context, program string, args []string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		if ext := strings.ToLower(filepath.Ext(program)); ext == ".cmd" || ext == ".bat" {
			shell := os.Getenv("ComSpec")
			if shell == "" {
				shell = `C:\Windows\System32\cmd.exe`
			}
			cmd := exec.CommandContext(ctx, shell)
			useCommandLine(cmd, cmdLine(shell, program, args))
			return cmd
		}
	}
	return exec.CommandContext(ctx, program, args...)
}

// MARK: Which are here

type providerKind string

const (
	providerReady     providerKind = "ready"
	providerMissing   providerKind = "missing"
	providerSignedOut providerKind = "signedOut"
	providerChanged   providerKind = "changed"
	// Its tools can't be turned off safely: never run.
	providerUnsupported providerKind = "unsupported"
	// Signed in with an API account (a Console login): it works, billed to that account.
	providerAPIAccount providerKind = "apiAccount"
)

// works is whether a summary can be made with it.
func (s providerState) works() bool { return s.kind == providerReady || s.kind == providerAPIAccount }

// providerState is whether a coding agent is here and signed in, as setup shows it.
type providerState struct {
	kind    providerKind
	reason  string
	program string
}

// summaryProviders finds the coding agents, asks each whether it's signed
// in, and checks its help still lists every flag a summary passes, in the
// background, at most once a minute unless asked: setup asks every second or so.
type summaryProviders struct {
	mu        sync.Mutex
	known     map[summaryProvider]providerState
	checkedAt time.Time
	checking  bool
	done      chan struct{} // closed when the check going on ends
	helps     map[string]string
	check     func(summaryProvider) providerState
}

const summaryProvidersStale = time.Minute

func newSummaryProviders() *summaryProviders {
	p := &summaryProviders{helps: map[string]string{}}
	p.check = p.checkProvider
	return p
}

// States is what's known, or nil before the first check has finished.
func (p *summaryProviders) States() map[summaryProvider]providerState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.known
}

func (p *summaryProviders) Checking() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.checking
}

// Refresh checks again, in the background, if it's been a while or force.
func (p *summaryProviders) Refresh(force bool) {
	p.mu.Lock()
	if p.checking || (!force && time.Since(p.checkedAt) < summaryProvidersStale) {
		p.mu.Unlock()
		return
	}
	p.checking = true
	done := make(chan struct{})
	p.done = done
	p.mu.Unlock()
	go func() {
		found := map[summaryProvider]providerState{}
		for _, agent := range summaryAgents(runtime.GOOS) {
			found[agent] = p.check(agent)
		}
		p.mu.Lock()
		p.known, p.checkedAt, p.checking = found, time.Now(), false
		p.mu.Unlock()
		close(done)
	}()
}

// StatesNow is what's known, checked now if never yet.
func (p *summaryProviders) StatesNow() map[summaryProvider]providerState {
	if known := p.States(); known != nil {
		return known
	}
	p.Refresh(true)
	p.mu.Lock()
	done := p.done
	p.mu.Unlock()
	if done != nil {
		<-done
	}
	return p.States()
}

// set is for tests: what's known, set.
func (p *summaryProviders) set(states map[summaryProvider]providerState) {
	p.mu.Lock()
	p.known, p.checkedAt = states, time.Now()
	p.mu.Unlock()
}

func (p *summaryProviders) checkProvider(provider summaryProvider) providerState {
	if !provider.supported() {
		return providerState{kind: providerUnsupported, reason: unsupportedReason}
	}
	program := summaryProgram(provider)
	if program == "" {
		return providerState{kind: providerMissing, reason: "not installed"}
	}
	env := summaryEnvironment(os.Environ())
	help := p.helpText(provider, program, env)
	for _, flag := range summaryFlags(provider) {
		if !helpLists(help, flag) {
			return providerState{kind: providerChanged, program: program,
				reason: "updated, and no longer takes what Smolcorp passes: summaries with it need an update of Smolcorp"}
		}
	}
	signedIn := false
	exists := func(parts ...string) bool {
		_, err := os.Stat(filepath.Join(append([]string{homeDir}, parts...)...))
		return err == nil
	}
	switch provider {
	case providerClaude:
		switch claudeLogin(quickRun(program, []string{"auth", "status", "--json"}, env).output) {
		case providerReady:
			return providerState{kind: providerReady, reason: provider.plan(), program: program}
		case providerAPIAccount:
			return providerState{kind: providerAPIAccount, reason: "signed in with a Console account: bills your API account", program: program}
		}
	case providerCodex:
		signedIn = quickRun(program, []string{"login", "status"}, env).status == 0
	case providerGemini:
		signedIn = exists(".gemini", "oauth_creds.json")
	case providerGrok:
		signedIn = exists(".grok", "auth.json")
	case providerCursor:
		ran := quickRun(program, []string{"status"}, env)
		said := strings.ToLower(string(ran.output) + string(ran.errors))
		signedIn = ran.status == 0 && !strings.Contains(said, "not logged in") && !strings.Contains(said, "not authenticated")
	case providerOpenCode:
		// Any model it can use: an account it's connected, or one on this computer.
		ran := quickRun(program, []string{"models"}, env)
		signedIn = ran.status == 0 && strings.TrimSpace(string(ran.output)) != ""
		// Its summary agent must exist and be denied every tool, or it isn't run at all.
		if signedIn && !openCodeSummaryAgentIsLocked(quickRun(program, []string{"debug", "agents"}, env).output) {
			return providerState{kind: providerUnsupported, program: program,
				reason: "its summary agent isn't there, or may use tools: not supported with this version"}
		}
	}
	if !signedIn {
		return providerState{kind: providerSignedOut, reason: "installed, not signed in: " + provider.signIn(), program: program}
	}
	return providerState{kind: providerReady, reason: provider.plan(), program: program}
}

func (p *summaryProviders) helpText(provider summaryProvider, program string, env []string) string {
	stamp := string(provider) + " " + program
	if info, err := os.Stat(program); err == nil {
		stamp += " " + info.ModTime().String()
	}
	p.mu.Lock()
	help, ok := p.helps[stamp]
	p.mu.Unlock()
	if ok {
		return help
	}
	ran := quickRun(program, provider.helpArguments(), env)
	help = string(ran.output) + string(ran.errors)
	p.mu.Lock()
	p.helps[stamp] = help
	p.mu.Unlock()
	return help
}

func quickRun(program string, args []string, env []string) summaryRan {
	return spawnSummary(program, args, nil, env, os.TempDir(), 10*time.Second)
}

// summaryProgram is where provider's program is, or "". An agent that is Grok's isn't Cursor's.
func summaryProgram(provider summaryProvider) string {
	for _, name := range provider.programs() {
		program := programNamed(name)
		if program == "" {
			continue
		}
		if provider == providerCursor && name == "agent" {
			ran := quickRun(program, []string{"--version"}, summaryEnvironment(os.Environ()))
			if strings.Contains(strings.ToLower(string(ran.output)+string(ran.errors)), "grok") {
				continue
			}
		}
		return program
	}
	return ""
}

// MARK: The choice

// chosenSummaryProvider is the provider summaries use: the one saved, even
// when it stops working (it then says why, and never quietly sends
// elsewhere); else, for one who turned summaries on with a key before there
// was a choice, the key, as it was; else the first coding agent that works,
// then the key. "" for none.
func chosenSummaryProvider(saved string, optIn bool, states map[summaryProvider]providerState, hasKey bool) summaryProvider {
	if p, ok := validSummaryProvider(saved); ok {
		if !p.supported() || (p == providerAPIKey && !hasKey) {
			return ""
		}
		return p
	}
	if optIn && hasKey {
		return providerAPIKey
	}
	for _, agent := range summaryAgents(runtime.GOOS) {
		if agent.supported() && states[agent].kind == providerReady {
			return agent
		}
	}
	if hasKey {
		return providerAPIKey
	}
	return ""
}

// summaryProviderWorks is whether p can make a summary now.
func summaryProviderWorks(p summaryProvider, states map[summaryProvider]providerState, hasKey bool) bool {
	if p == providerAPIKey {
		return hasKey
	}
	return p.supported() && states[p].works()
}
