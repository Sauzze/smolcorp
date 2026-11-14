package desktop

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// ✨ Sign in from Smolcorp (1.26 S2): the login runner. The page starts an
// agent's own login command, one of a short list per agent and never one that
// prints a token (no `claude setup-token`), in a pseudo-terminal of the core's
// own, as the launcher runs agents: a resolved path, a cleaned environment, a
// time limit, and ended with all it started once it's done, cancelled or out of
// time. The credentials land where each CLI keeps them (the Keychain, its own
// file); Smolcorp never reads them.
//
// What the command prints is read by a filter that lets three things through
// and drops the rest: the sign-in link (https, on the agent's own hosts and
// paths, its parameters checked), a device code (a strict pattern, for the
// agents that print one), and whether it said it succeeded or failed (kept as
// a word of Smolcorp's own, never the line printed). redactText is a second net
// over what passes. Nothing printed is logged, nor put in an event, a
// transcript, a brief or a summary: it stays in this file's memory, and only
// the page's own routes read the filtered state. A code the page sends goes to
// that login's terminal alone, while it runs.

// loginWay is one command the runner may run for an agent.
type loginWay struct {
	agent, way string
	args       []string
	// Where its sign-in link may point: host, exactly, and the paths on it.
	hosts map[string][]string
	// A device code it prints, as a whole line, or nil when it prints none.
	deviceCode *regexp.Regexp
	// Whether it asks for a code pasted back (the page's "paste the code" box).
	takesCode bool
	// The start of a line that says it signed in, or that it didn't.
	succeeded, failed *regexp.Regexp
}

var (
	claudeLoginHosts = map[string][]string{
		"claude.ai":             {"/oauth/authorize"},
		"claude.com":            {"/cai/oauth/authorize"},
		"platform.claude.com":   {"/oauth/authorize"},
		"console.anthropic.com": {"/oauth/authorize"},
	}
	claudeLoginSucceeded = regexp.MustCompile(`^Login successful\b`)
	claudeLoginFailed    = regexp.MustCompile(`^(Login failed|OAuth login failed|Login interrupted|Input ended before a code was pasted)\b`)

	codexLoginHosts       = map[string][]string{"auth.openai.com": {"/oauth/authorize", "/codex/device"}}
	codexLoginSucceeded   = regexp.MustCompile(`^Successfully logged in\b`)
	codexLoginFailed      = regexp.MustCompile(`(?i)^(error|login failed|device code login failed)\b`)
	codexDeviceCode       = regexp.MustCompile(`^[A-Z0-9]{4,5}-[A-Z0-9]{4,5}$`)
	loginWaysNoneOfGemini = "Gemini CLI has no sign-in command of its own: open gemini in a terminal and choose /auth."
)

// loginWays are the only commands the runner runs. Claude Code: `claude auth
// login`, the documented command for /login outside a session (Claude
// subscription, or Anthropic Console). Codex: `codex login`, and `codex login
// --device-auth` for a browser on another device. Gemini CLI documents none (it
// signs in from its own /auth), so it has none. Never `claude setup-token`, nor
// `codex login --with-api-key`, nor anything that prints or reads a key.
var loginWays = []loginWay{
	{agent: "claude", way: "claudeai", args: []string{"auth", "login", "--claudeai"}, hosts: claudeLoginHosts,
		takesCode: true, succeeded: claudeLoginSucceeded, failed: claudeLoginFailed},
	{agent: "claude", way: "console", args: []string{"auth", "login", "--console"}, hosts: claudeLoginHosts,
		takesCode: true, succeeded: claudeLoginSucceeded, failed: claudeLoginFailed},
	{agent: "codex", way: "browser", args: []string{"login"}, hosts: codexLoginHosts,
		succeeded: codexLoginSucceeded, failed: codexLoginFailed},
	{agent: "codex", way: "device", args: []string{"login", "--device-auth"}, hosts: codexLoginHosts, deviceCode: codexDeviceCode,
		succeeded: codexLoginSucceeded, failed: codexLoginFailed},
}

// loginWayOf is the way named for agent ("" is its first), or nil.
func loginWayOf(agent, way string) *loginWay {
	for i := range loginWays {
		if w := &loginWays[i]; w.agent == agent && (way == "" || w.way == way) {
			return w
		}
	}
	return nil
}

// loginParams are the link's parameters the agents send, each with what its
// value may be. Any other is read by redactText, and a link it flags is dropped.
var loginParams = map[string]*regexp.Regexp{
	"code":                       regexp.MustCompile(`^true$`),
	"client_id":                  regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`),
	"response_type":              regexp.MustCompile(`^code$`),
	"redirect_uri":               regexp.MustCompile(`^https?://[a-z0-9.-]+(:[0-9]{1,5})?/[A-Za-z0-9/_.-]*$`),
	"scope":                      regexp.MustCompile(`^[a-z0-9:_. -]{1,400}$`),
	"code_challenge":             regexp.MustCompile(`^[A-Za-z0-9_-]{43,128}$`),
	"code_challenge_method":      regexp.MustCompile(`^S256$`),
	"state":                      regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`),
	"id_token_add_organizations": regexp.MustCompile(`^true$`),
	"codex_cli_simplified_flow":  regexp.MustCompile(`^true$`),
	"originator":                 regexp.MustCompile(`^[a-z_]{1,40}$`),
}

const (
	// The longest link passed on, line read, and code taken from the page.
	loginURLMost  = 4 << 10
	loginLineMost = 8 << 10
	loginCodeMost = 512
	// What the command prints is looked at up to this much; past it, it's read and dropped.
	loginOutputMost = 256 << 10
	// A wide terminal, so a long link comes on one line.
	loginColumns, loginRows = 2000, 40
	// How long the reading may go on once the command has ended.
	loginDrain = 2 * time.Second
)

// A sign-in waits for you in the browser, and a device code lasts 15 minutes:
// past this it's ended (a variable, for the tests).
var loginTimeLimit = 10 * time.Minute

// loginFind is where an agent's program is, "" nowhere: a variable, for the
// tests, which look only in a folder of their own, so no real CLI is ever run.
var loginFind = programNamed

type loginState string

const (
	loginRunning   loginState = "running"
	loginSucceeded loginState = "succeeded"
	loginFailed    loginState = "failed"
	loginTimedOut  loginState = "timedOut"
	loginCancelled loginState = "cancelled"
)

// loginView is all the page is told: no line the command printed is in it.
type loginView struct {
	ID    string     `json:"id"`
	Agent string     `json:"agent"`
	Way   string     `json:"way"`
	State loginState `json:"state"`
	// While it runs: the link to open, and a device code to enter there.
	URL  string `json:"url,omitempty"`
	Code string `json:"code,omitempty"`
}

// loginRun is one login command, running or ended.
type loginRun struct {
	id       string
	way      *loginWay
	terminal pty.Pty
	group    processGroup
	readDone chan struct{}
	done     chan struct{}

	mu       sync.Mutex
	finished bool       // ended and reaped: nothing more goes to its terminal
	ending   loginState // cancelled or timed out, "" when it ended by itself
	state    loginState
	url      string
	code     string
	saidOK   bool
	saidNot  bool
}

// loginRunner holds the one login there may be at a time. The zero value is ready.
type loginRunner struct {
	mu  sync.Mutex
	run *loginRun
}

// start runs agent's way to sign in: its id, or a status and why not.
func (lr *loginRunner) start(agent, way string) (string, int, string) {
	w := loginWayOf(agent, way)
	if w == nil {
		if agent == "gemini" {
			return "", http.StatusConflict, loginWaysNoneOfGemini
		}
		return "", http.StatusBadRequest, "There's no sign-in for that here."
	}
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if lr.run != nil && lr.run.view().State == loginRunning {
		return "", http.StatusConflict, "A sign-in is already running: finish it or cancel it first."
	}
	program := loginFind(agent)
	if program == "" {
		return "", http.StatusConflict, agentDisplayName(agent) + " isn't installed on this computer."
	}
	// A Claude Code that isn't Anthropic's is never started (agentcheck.go).
	if err := checkAgentProgram(agent, program); err != nil {
		return "", http.StatusConflict, err.Error()
	}
	program, args := loginCommand(program, w.args)
	// On a Mac the login runs through smolcorp-spawn, so it holds none of Smolcorp's permissions (SANDBOX-PLAN
	// §3.5, ST1): a hire can replace an agent's CLI. useCmdLine still gets the program itself (Windows only).
	started, words, err := disclaim.Words(program, args)
	if err != nil {
		return "", http.StatusInternalServerError, "The sign-in couldn't start. " + err.Error() + "."
	}
	terminal, err := pty.New()
	if err != nil {
		return "", http.StatusInternalServerError, "The sign-in couldn't start: no pseudo-terminal."
	}
	_ = terminal.Resize(loginColumns, loginRows)
	cmd := terminal.Command(started, words...)
	useCmdLine(cmd, program, args)
	cmd.Env = loginEnvironment(os.Environ())
	cmd.Dir = os.TempDir()
	if err := cmd.Start(); err != nil {
		terminal.Close()
		return "", http.StatusInternalServerError, "The sign-in couldn't start."
	}
	// The core's copy of the terminal's other end goes: once the command and all it started have
	// ended, the reading ends, after all they printed.
	if unix, ok := terminal.(pty.UnixPty); ok {
		_ = unix.Slave().Close()
	}
	run := &loginRun{id: newID(), way: w, terminal: terminal, group: groupOf(cmd.Process),
		readDone: make(chan struct{}), done: make(chan struct{}), state: loginRunning}
	lr.run = run
	go run.read(terminal)
	go run.wait(cmd)
	return run.id, http.StatusOK, ""
}

// current is the run with that id ("" is the last one), or nil.
func (lr *loginRunner) current(id string) *loginRun {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if lr.run == nil || (id != "" && !constantTimeEqual(id, lr.run.id)) {
		return nil
	}
	return lr.run
}

// loginCommand is the program and arguments to start: on Windows, an npm .cmd
// script goes through cmd.exe, as the launcher starts one (useCmdLine).
func loginCommand(program string, args []string) (string, []string) {
	if runtime.GOOS == "windows" {
		if ext := strings.ToLower(filepath.Ext(program)); ext == ".cmd" || ext == ".bat" {
			shell := os.Getenv("ComSpec")
			if shell == "" {
				shell = `C:\Windows\System32\cmd.exe`
			}
			return shell, append([]string{"/c", program}, args...)
		}
	}
	return program, slices.Clone(args)
}

// loginEnvironment is a login's environment, from base (Smolcorp's): a
// summary's (no API key that would stand in for the sign-in, no other
// provider's cloud, nothing that takes it for a desk), and no colours.
func loginEnvironment(base []string) []string {
	return append(summaryEnvironment(base), "NO_COLOR=1")
}

// wait ends the run: when the command ends, is cancelled, or runs out of time.
func (run *loginRun) wait(cmd *pty.Cmd) {
	defer close(run.done)
	timer := time.AfterFunc(loginTimeLimit, func() { run.end(loginTimedOut) })
	err := cmd.Wait()
	timer.Stop()
	run.mu.Lock()
	run.finished = true
	run.mu.Unlock()
	// Anything it left running.
	run.group.close()
	if _, unix := run.terminal.(pty.UnixPty); unix {
		run.drained()
		run.terminal.Close()
	} else {
		run.terminal.Close()
		run.drained()
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	switch {
	case run.ending != "":
		run.state = run.ending
	case err == nil && run.saidOK && !run.saidNot:
		run.state = loginSucceeded
	default:
		run.state = loginFailed
	}
	// The link and the code were for this sign-in only.
	run.url, run.code = "", ""
}

// drained waits for the reading to end, for a short time at most.
func (run *loginRun) drained() {
	select {
	case <-run.readDone:
	case <-time.After(loginDrain):
	}
}

// end ends a running login, with all it started, as cancelled or timed out.
func (run *loginRun) end(why loginState) bool {
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.finished || run.ending != "" {
		return false
	}
	run.ending = why
	_ = run.group.kill()
	return true
}

// view is what the page is told.
func (run *loginRun) view() loginView {
	run.mu.Lock()
	defer run.mu.Unlock()
	return loginView{ID: run.id, Agent: run.way.agent, Way: run.way.way, State: run.state, URL: run.url, Code: run.code}
}

// typeCode sends a code the page was given to this login's terminal: only while it
// runs, and only to a way that asks for one. 204, or a status and why not.
func (run *loginRun) typeCode(code string) (int, string) {
	if len(code) == 0 || len(code) > loginCodeMost {
		return http.StatusBadRequest, "That code doesn't look right."
	}
	for i := 0; i < len(code); i++ {
		if code[i] <= ' ' || code[i] >= 0x7f {
			return http.StatusBadRequest, "That code doesn't look right."
		}
	}
	if !run.way.takesCode {
		return http.StatusConflict, "This sign-in doesn't take a code."
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.finished || run.ending != "" {
		return http.StatusConflict, "That sign-in has ended."
	}
	if _, err := io.WriteString(run.terminal, code+"\r"); err != nil {
		return http.StatusConflict, "That sign-in has ended."
	}
	return http.StatusNoContent, ""
}

// read reads what the command prints, a line at a time, and keeps only what
// the filter lets through. The rest is dropped as it's read.
func (run *loginRun) read(from io.Reader) {
	defer close(run.readDone)
	buffer := make([]byte, 4096)
	line := make([]byte, 0, 256)
	skipping, total := false, 0
	defer func() { clear(buffer); clear(line[:cap(line)]) }()
	for {
		n, err := from.Read(buffer)
		if total < loginOutputMost {
			for _, b := range buffer[:n] {
				switch {
				case b == '\n' || b == '\r':
					if !skipping {
						run.look(line)
					}
					clear(line)
					line, skipping = line[:0], false
				case skipping:
				case len(line) >= loginLineMost:
					clear(line)
					line, skipping = line[:0], true
				default:
					line = append(line, b)
				}
			}
		}
		total += n
		if err != nil {
			if !skipping && len(line) > 0 {
				run.look(line)
			}
			return
		}
	}
}

// look keeps what one printed line holds that the filter lets through.
func (run *loginRun) look(raw []byte) {
	text := strings.TrimSpace(plainLine(raw))
	if text == "" {
		return
	}
	link := ""
	for _, field := range strings.Fields(text) {
		if found, ok := run.way.signInURL(field); ok {
			link = found
			break
		}
	}
	code := ""
	if run.way.deviceCode != nil && run.way.deviceCode.MatchString(text) && redactText(text) == text {
		code = text
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.url == "" && link != "" && !run.finished {
		run.url = link
	}
	if run.code == "" && code != "" && !run.finished {
		run.code = code
	}
	run.saidOK = run.saidOK || run.way.succeeded.MatchString(text)
	run.saidNot = run.saidNot || run.way.failed.MatchString(text)
}

// terminalEscape is a terminal's escape sequence: CSI (colours, cursor moves),
// OSC (titles, links) and the other strings, charset picks, and the two-byte ones.
var terminalEscape = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)?|[PX^_][^\x1b]*(?:\x1b\\)?|[()*+][0-9A-Za-z]|[@-Z\\-_])`)

// plainLine is a printed line without its escape sequences and control characters.
func plainLine(raw []byte) string {
	text := terminalEscape.ReplaceAllString(string(raw), "")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r < ' ' || r == 0x7f || (r >= 0x80 && r < 0xa0):
			return -1
		}
		return r
	}, text)
}

// signInURL is field as a sign-in link this way may pass on, or false: https,
// printable ASCII, no user, port or fragment, a host and path of the agent's
// own, each known parameter as it must be and not a key's shape, and the rest
// clean to redactText.
func (w *loginWay) signInURL(field string) (string, bool) {
	field = strings.TrimRight(field, ".,;:)]}>'\"")
	if len(field) > loginURLMost || !strings.HasPrefix(field, "https://") {
		return "", false
	}
	for i := 0; i < len(field); i++ {
		if field[i] <= ' ' || field[i] >= 0x7f {
			return "", false
		}
	}
	u, err := url.Parse(field)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Fragment != "" || strings.Contains(field, "#") {
		return "", false
	}
	paths, ok := w.hosts[u.Host]
	if !ok || u.RawPath != "" || !slices.Contains(paths, u.Path) {
		return "", false
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", false
	}
	rest := []string{u.Host + u.Path}
	for name, values := range query {
		pattern, known := loginParams[name]
		for _, value := range values {
			if !known {
				// Printable, once decoded: no escape or line break is let in through a %.
				if strings.ContainsFunc(name+value, func(r rune) bool { return r < ' ' || r >= 0x7f }) {
					return "", false
				}
				rest = append(rest, name+"="+value)
			} else if !pattern.MatchString(value) || looksLikeAKey(value) {
				return "", false
			}
		}
	}
	// The second net, over all a known parameter's pattern didn't check.
	if net := strings.Join(rest, "\n"); redactText(net) != net {
		return "", false
	}
	return field, true
}

// looksLikeAKey is whether a known parameter's value has a key's shape, as
// redactText's patterns read one (sk-…, a JWT, "token"): all but its bare AWS
// key, which a random PKCE value would sometimes look like.
func looksLikeAKey(value string) bool {
	return slices.ContainsFunc(secretLine, func(pattern *regexp.Regexp) bool { return pattern.MatchString(value) })
}

// agentDisplayName is how the page names an agent.
func agentDisplayName(agent string) string {
	switch agent {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "gemini":
		return "Gemini CLI"
	}
	return agent
}

// accountsLogin answers the sign-in routes, the page's alone (goOnlyRoutes):
//
//	POST /setup/accounts/login         {"agent", "way"}  starts one: {"id"}
//	GET  /setup/accounts/login?id=     its state (loginView)
//	POST /setup/accounts/login/code    {"id", "code"}    a code for it to read
//	POST /setup/accounts/login/cancel  {"id"}            ends it
func (rt *router) accountsLogin(w http.ResponseWriter, r *request) {
	if r.caller != callerPage {
		writeStatus(w)(http.StatusForbidden, routeRefusal(r.Method, r.URL.Path))
		return
	}
	logins := &rt.deck.logins
	w.Header().Set("Cache-Control", "no-store")
	var asked struct{ Agent, Way, ID, Code string }
	if r.Method == http.MethodPost && json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&asked) != nil {
		writeStatus(w)(http.StatusBadRequest, "That request isn't one the page sends.")
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/setup/accounts/login":
		id, status, why := logins.start(asked.Agent, asked.Way)
		if why != "" {
			writeStatus(w)(status, why)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
	case r.Method == http.MethodGet && r.URL.Path == "/setup/accounts/login":
		run := logins.current(r.URL.Query().Get("id"))
		if run == nil {
			writeStatus(w)(http.StatusNotFound, "There's no such sign-in.")
			return
		}
		writeJSON(w, http.StatusOK, run.view())
	case r.Method == http.MethodPost && r.URL.Path == "/setup/accounts/login/code":
		run := logins.current(asked.ID)
		if asked.ID == "" || run == nil {
			writeStatus(w)(http.StatusNotFound, "There's no such sign-in.")
			return
		}
		writeStatus(w)(run.typeCode(asked.Code))
	case r.Method == http.MethodPost && r.URL.Path == "/setup/accounts/login/cancel":
		run := logins.current(asked.ID)
		if asked.ID == "" || run == nil {
			writeStatus(w)(http.StatusNotFound, "There's no such sign-in.")
			return
		}
		if run.end(loginCancelled) {
			<-run.done
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
