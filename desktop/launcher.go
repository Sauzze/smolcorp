package desktop

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aymanbagabas/go-pty"
	"golang.org/x/term"
)

// The launcher is `smolcorp run <agent> [arguments]`. It starts the agent
// in a terminal of its own, inside the one you typed the command in: your
// typing goes in and the agent's screen comes out exactly as before, and the
// deck can press keys in it too, with no window to find and no permission to
// ask for. It sets SMOLCORP_SESSION, so the agent's hooks can say which
// launcher they run in.
//
// The launcher holds a connection to the app open, and the app sends the
// deck's keys down it as JSON lines. Without the app, the agent runs as if
// started plainly, and the launcher keeps trying to reach the app.

// launcherCommand is one line down that connection.
type launcherCommand struct {
	Press *Keystroke `json:"press,omitempty"`
	Type  string     `json:"type,omitempty"`
	// Paste writes the text and nothing else: no Enter. It is for mail only (the
	// phone's own sends use Type and Press), so the launcher refuses text that
	// isn't safe to paste, and multi-line text when the agent didn't ask for
	// bracketed paste. See pasteBytes.
	Paste string `json:"paste,omitempty"`
	// PhoneText pastes the phone's words as Paste does (one line, or several as one bracketed paste where the
	// agent asked for it), then answers the look numbered Look with whether it did: "pasted" or "not-taken".
	// An older launcher, without it, only looks: its answer says neither, and nothing is typed after it.
	PhoneText string `json:"phoneText,omitempty"`
	// Stop ends the agent: the office's firing.
	Stop bool `json:"stop,omitempty"`
	// Look asks for the state of the window, answered at /launcher/screen
	// with this number: see launcherlook.go.
	Look string `json:"look,omitempty"`
	// Secret is the first line of the stream, never a command: the launcher
	// keeps it to prove its answers are its own. See launcherlook.go.
	Secret string `json:"secret,omitempty"`
}

// How long a fired agent is given to end before it's made to.
const stopGrace = 3 * time.Second

// How long the launcher waits before reaching for the app again.
const reconnectDelay = 3 * time.Second

// RunLauncher returns the agent's exit code. `smolcorp run --role <id>`
// starts the role's agent on the role, as the Mac app's office hires it.
//
// The office's hires start it with `--hire <token>` first, and `--pinned` at
// a pinned project's desk: see hire.go. Those aren't for typing.
func RunLauncher(args []string) int {
	run, err := parseRun(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Usage: smolcorp run <agent> [arguments], for example: smolcorp run claude\n"+
			"   or: smolcorp run --role <role>, for example: smolcorp run --role tester")
		return 2
	}
	code := launch(run)
	// A hire's window closes with its agent: one that never started says why first.
	if run.hire != "" && code != 0 && !run.started {
		fmt.Fprint(os.Stderr, "Press Enter to close this window.")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	return code
}

// launcherRun is what `smolcorp run` was asked to start.
type launcherRun struct {
	// The agent and its arguments, or nothing for a role.
	command []string
	role    string
	// The office's token, for a hire, and whether it was at a pinned project's desk.
	hire   string
	pinned bool
	// For an engine hire (hirelauncher.go): its id, and its own settings file. The
	// launcher then asks the app for the hire's environment before it starts anything.
	hireID   string
	settings string
	// For a hire of a role, its agent as the office checked it could start: the
	// role must still have it.
	agent string
	// Whether the agent got as far as starting.
	started bool
}

// parseRun reads `smolcorp run`'s arguments: [--hire <token> [--pinned]
// [--agent <agent>]] then <agent> [arguments], or --role <id>.
func parseRun(args []string) (*launcherRun, error) {
	run := &launcherRun{}
	if len(args) >= 2 && args[0] == "--hire" {
		if !hireToken.MatchString(args[1]) {
			return nil, errors.New("not a hire's token")
		}
		run.hire, args = args[1], args[2:]
		if len(args) >= 2 && args[0] == "--hire-id" {
			if !plainHireID(args[1]) {
				return nil, errors.New("not a hire's id")
			}
			run.hireID, args = args[1], args[2:]
		}
		if len(args) > 0 && args[0] == "--pinned" {
			run.pinned, args = true, args[1:]
		}
		if len(args) > 0 && args[0] == "--agent" {
			if len(args) < 3 || args[2] != "--role" || args[1] == "" {
				return nil, errors.New("--agent is for a role's hire")
			}
			run.agent, args = args[1], args[2:]
		}
	}
	if run.hireID != "" && (len(args) == 0 || args[0] != "--role") {
		return nil, errors.New("an engine hire is a role's")
	}
	switch {
	case len(args) == 0, args[0] == "--hire", args[0] == "--pinned":
		return nil, errors.New("nothing to run")
	case args[0] == "--role":
		switch {
		case len(args) == 2:
		case run.hireID != "" && len(args) == 3 && settingsArgument(args[2]):
			run.settings = args[2]
		default:
			return nil, errors.New("a role, and nothing else")
		}
		run.role = args[1]
	default:
		run.command = args
	}
	return run, nil
}

func launch(run *launcherRun) int {
	args := run.command
	folder, _ := os.Getwd()
	// An engine hire's secrets come from the app, before anything starts: if they
	// can't be had, nothing is started and the window says why. With them comes
	// its first message, when the app wrote it a brief.
	var startEnv *hireStartEnv
	if run.hireID != "" {
		key, _ := os.ReadFile(hookKeyFile)
		got, err := fetchHireStartEnv(hireStartClient(), fmt.Sprintf("http://127.0.0.1:%d", Port), string(key), run.hire, run.hireID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: couldn't get this hire's start settings, so nothing was started: %v\n", err)
			return 1
		}
		startEnv = &got
	}
	if run.role != "" {
		how := startedHere
		if run.hire != "" {
			how = map[bool]roleStart{true: hiredAtPinnedDesk, false: hiredIntoGeneralFolder}[run.pinned]
		}
		message := ""
		if startEnv != nil {
			message = startEnv.Message
		}
		found, err := roleArguments(run.role, folder, how, message)
		if err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
			return 2
		}
		if err := sameAgentAsHired(run, found[0]); err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
			return 2
		}
		args = found
		if run.settings != "" {
			// The engine hire's own settings, for Claude Code only, right after the agent.
			if agentNamed(found[0]) != Claude {
				fmt.Fprintln(os.Stderr, "smolcorp: a hire's settings are for Claude Code, and this role's agent is another")
				return 2
			}
			args = append([]string{found[0], run.settings}, found[1:]...)
		}
	}
	program, programArgs, err := resolveProgram(args[0], args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
		return 127
	}
	if run.role != "" && program != args[0] && len(programArgs) > 0 && programArgs[0] == "/c" {
		if programArgs, err = forCmd(programArgs); err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
			return 2
		}
	}

	terminal, err := pty.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: no pseudo-terminal: %v\n", err)
		return 1
	}
	defer terminal.Close()

	id := newID()
	cmd := terminal.Command(program, programArgs...)
	useCmdLine(cmd, program, programArgs)
	cmd.Env = append(os.Environ(), "SMOLCORP_SESSION="+id)
	if startEnv != nil {
		// On the agent's process alone.
		cmd.Env = startEnvironment(cmd.Env, *startEnv, runtime.GOOS == "windows")
	}
	cmd.Dir = folder
	rows, cols, sized := consoleSize()
	screen := newLauncherScreen(rows, cols, sized)
	matchSize(terminal, screen)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
		return 127
	}
	run.started = true

	stdin := int(os.Stdin.Fd())
	if term.IsTerminal(stdin) {
		if saved, err := term.MakeRaw(stdin); err == nil {
			defer term.Restore(stdin, saved)
		}
	}
	prepareConsole()

	input := &lockedWriter{w: terminal}
	cursor := &cursorMode{bracketed: screen.Bracketed}
	go func() { _, _ = io.Copy(input, os.Stdin) }()
	go func() { _, _ = io.Copy(os.Stdout, io.TeeReader(terminal, io.MultiWriter(cursor, screen))) }()
	go followSize(terminal, screen)
	go serveDeck(id, agentNamed(args[0]), folder, run.role, run.hire, input, cursor, screen, func() { stopProcess(cmd.Process) })

	if err := cmd.Wait(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

// resolveProgram finds the agent on PATH. On Windows, npm installs agents as
// .cmd scripts, which a pseudo-console can't start by themselves.
func resolveProgram(name string, args []string) (string, []string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", nil, fmt.Errorf("%s isn't installed, or isn't on your PATH", name)
	}
	if runtime.GOOS == "windows" {
		if ext := strings.ToLower(filepath.Ext(path)); ext == ".cmd" || ext == ".bat" {
			shell := os.Getenv("ComSpec")
			if shell == "" {
				shell = `C:\Windows\System32\cmd.exe`
			}
			return shell, append([]string{"/c", path}, args...), nil
		}
	}
	return path, args, nil
}

// cmdLine is the command line that has cmd.exe start a .cmd script with its
// arguments, escaped as cross-spawn escapes them for Node on Windows
// (github.com/moxystudio/node-cross-spawn, lib/util/escape.js):
//
//   - each argument is quoted, with its quotes and the backslashes before
//     them escaped as programs' own parsing reads them;
//   - cmd.exe's own characters, as & | < > ^ % and quotes, are each preceded
//     by ^, so cmd.exe passes them on rather than acting on them; twice for a
//     .cmd script, as npm's are, which hand %* to cmd.exe a second time;
//   - /d skips AutoRun, and with /s cmd.exe takes off just the quotes
//     around all the rest.
//
// So a script under a user folder with a space, as C:\Users\Jean Dupont, and
// an argument with quotes or & in it both come through as they were.
func cmdLine(shell, script string, args []string) string {
	twice := strings.EqualFold(filepath.Ext(script), ".cmd")
	line := cmdEscaped(script)
	for _, arg := range args {
		arg = cmdQuotes.ReplaceAllString(arg, `$1$1\"`)
		arg = cmdTrailingBackslashes.ReplaceAllString(arg, `$1$1`)
		arg = cmdEscaped(`"` + arg + `"`)
		if twice {
			arg = cmdEscaped(arg)
		}
		line += " " + arg
	}
	return `"` + shell + `" /d /s /c "` + line + `"`
}

var (
	cmdMeta                = regexp.MustCompile(`([()\][%!^"` + "`" + `<>&|;, *?])`)
	cmdQuotes              = regexp.MustCompile(`(\\*)"`)
	cmdTrailingBackslashes = regexp.MustCompile(`(\\*)$`)
)

func cmdEscaped(s string) string { return cmdMeta.ReplaceAllString(s, "^$1") }

// sameAgentAsHired is an error when a role's agent is no longer the one its
// hire checked could start: the role was changed in setup meanwhile.
func sameAgentAsHired(run *launcherRun, agent string) error {
	if run.agent == "" || run.agent == agent {
		return nil
	}
	return fmt.Errorf("the role's agent was changed to %s after it was hired, and only %s was checked. Hire it again", agent, run.agent)
}

// roleArguments are the agent and its arguments for the role with the id
// given, as set up on the setup page, started in folder as how says. message,
// unless it's empty, is its first message instead of the role's own: the one
// the app made with the hire's brief.
func roleArguments(id, folder string, how roleStart, message string) ([]string, error) {
	roles := loadConfig().Roles
	for _, role := range roles {
		if role.ID == id {
			if message == "" {
				message = role.firstMessage(folder, how)
			}
			return append([]string{role.Agent}, role.arguments(message)...), nil
		}
	}
	names := []string{}
	for _, role := range roles {
		names = append(names, role.ID)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("there's no role %q, and no roles at all: add some on the setup page", id)
	}
	return nil, fmt.Errorf("there's no role %q. Yours are: %s", id, strings.Join(names, ", "))
}

// forCmd readies a role's arguments for an agent npm installed as a .cmd
// script, which only cmd.exe can start. cmd.exe reads the command line again,
// so the message's lines become spaces, and characters it would act on, such
// as & or %, are refused rather than passed to it.
func forCmd(args []string) ([]string, error) {
	ready := make([]string, len(args))
	for i, arg := range args {
		arg = strings.Join(strings.FieldsFunc(arg, func(c rune) bool { return c == '\n' || c == '\r' }), " ")
		if strings.ContainsAny(arg, `"%&|<>^`) {
			return nil, fmt.Errorf("this agent starts through cmd.exe, which would act on the role's \" %% & | < > or ^: take those out of its instructions on the setup page")
		}
		ready[i] = arg
	}
	return ready, nil
}

// agentNamed is the agent a program name is, so the app can find Codex's
// launcher by folder: Codex runs its hooks elsewhere, without this
// launcher's variable.
func agentNamed(program string) Agent {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(program), filepath.Ext(program)))
	for _, a := range agents {
		if string(a) == base {
			return a
		}
	}
	return ""
}

// consoleSize is the console's rows and columns, and whether they could be read.
func consoleSize() (rows, cols int, ok bool) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 || height <= 0 {
		return 24, 80, false
	}
	return height, width, true
}

// matchSize gives the agent's terminal the console's size, and the screen the
// launcher reads the same one, first, so what the agent draws next lands at
// the new size.
func matchSize(terminal pty.Pty, screen *launcherScreen) {
	if rows, cols, ok := consoleSize(); ok {
		screen.Resize(rows, cols)
		_ = terminal.Resize(cols, rows)
	}
}

// lockedWriter keeps your typing and the deck's keys from interleaving.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// cursorMode watches the agent's screen for the switch between the two ways
// a terminal sends arrow keys (DECCKM). An agent in application mode expects
// ESC O A for Up, not ESC [ A.
type cursorMode struct {
	// bracketed says whether the agent has bracketed paste on, now (the screen's
	// mode 2004). Nil reads as off.
	bracketed func() bool

	mu          sync.Mutex
	application bool
	tail        []byte // the end of the last chunk, for a switch split across two
}

var (
	applicationMode = []byte("\x1b[?1h")
	normalMode      = []byte("\x1b[?1l")
)

func (c *cursorMode) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := append(c.tail, p...)
	if on, off := bytes.LastIndex(seen, applicationMode), bytes.LastIndex(seen, normalMode); on != off {
		c.application = on > off
	}
	c.tail = append([]byte(nil), seen[max(0, len(seen)-len(applicationMode)+1):]...)
	return len(p), nil
}

func (c *cursorMode) bytesFor(key Key) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.application && (key == Up || key == Down) {
		return "\x1bO" + keyBytes[key][2:]
	}
	return keyBytes[key]
}

// Bracketed paste's start and end.
const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// pasteBytes is what a mail paste writes to the agent: the text, inside a
// bracketed paste when the agent turned that on. No Enter, ever. Text with a
// control character other than a line feed or tab is refused (Escape could end
// the bracket early), and so is a line feed when the agent isn't taking
// bracketed paste, where it would be read as Enter. ok is false for a refusal.
func (c *cursorMode) pasteBytes(text string) (string, bool) {
	if text == "" || !mailPastable(text) {
		return "", false
	}
	on := c.bracketed != nil && c.bracketed()
	if on {
		return pasteStart + text + pasteEnd, true
	}
	if strings.ContainsAny(text, "\n\t") {
		return "", false // a Tab or a line feed would arrive as a key
	}
	return text, true
}

// serveDeck connects to the app and carries out its commands until the
// agent exits, reconnecting whenever the connection drops.
func serveDeck(id string, agent Agent, folder, role, hire string, input io.Writer, cursor *cursorMode, screen *launcherScreen, stop func()) {
	query := url.Values{"id": {id}, "agent": {string(agent)}, "folder": {folder}, "role": {role}, "hire": {hire}}
	base := fmt.Sprintf("http://127.0.0.1:%d", Port)
	address := base + "/launcher?" + query.Encode()
	for {
		if key, err := os.ReadFile(hookKeyFile); err == nil {
			request, _ := http.NewRequest(http.MethodGet, address, nil)
			secret := strings.TrimSpace(string(key))
			request.Header.Set(hookKeyHeader, secret)
			if response, err := http.DefaultClient.Do(request); err == nil {
				look := func(proof, number string) {
					postScreenAnswer(http.DefaultClient, base, secret, proof, id, number, screen)
				}
				pasted := func(proof, number, outcome string) {
					postPasteAnswer(http.DefaultClient, base, secret, proof, id, number, outcome, screen)
				}
				followWith(response.Body, input, cursor, look, pasted, stop)
				response.Body.Close()
			}
		}
		time.Sleep(reconnectDelay)
	}
}

// stopProcess asks the agent to end, and makes it after a moment. Windows
// can only make it.
func stopProcess(process *os.Process) {
	if process.Signal(syscall.SIGTERM) != nil {
		_ = process.Kill()
		return
	}
	time.AfterFunc(stopGrace, func() { _ = process.Kill() })
}

// follow carries out the app's commands. look answers a look at the window;
// it runs on its own, so a slow answer never holds up the keys.
func follow(commands io.Reader, input io.Writer, cursor *cursorMode, look func(proof, number string), stop func()) {
	followWith(commands, input, cursor, look, nil, stop)
}

// followWith is follow, with pasted to answer the phone's text: whether it was pasted, after it was.
func followWith(commands io.Reader, input io.Writer, cursor *cursorMode, look func(proof, number string),
	pasted func(proof, number, outcome string), stop func()) {
	lines := bufio.NewScanner(commands)
	proof := "" // the secret from the stream's first line
	for lines.Scan() {
		var command launcherCommand
		if json.Unmarshal(lines.Bytes(), &command) != nil {
			continue
		}
		switch {
		case command.Secret != "":
			proof = command.Secret // not a command
		case command.PhoneText != "":
			// Written before the answer, in this one goroutine: the answer is what was done.
			outcome := "not-taken"
			if text, ok := cursor.pasteBytes(command.PhoneText); ok {
				_, _ = io.WriteString(input, text)
				outcome = "pasted"
			}
			if pasted != nil && command.Look != "" {
				go pasted(proof, command.Look, outcome)
			}
		case command.Look != "":
			if look != nil {
				go look(proof, command.Look)
			}
		case command.Stop:
			stop()
		case command.Press != nil:
			for i := 0; i < max(command.Press.Times, 1); i++ {
				if i > 0 {
					time.Sleep(keyGap)
				}
				_, _ = io.WriteString(input, cursor.bytesFor(command.Press.Key))
			}
		case command.Paste != "":
			if text, ok := cursor.pasteBytes(command.Paste); ok {
				_, _ = io.WriteString(input, text)
			}
		case command.Type != "":
			_, _ = io.WriteString(input, command.Type)
			// A beat before Enter, or a TUI can take the text as a paste
			// and the Enter as part of it.
			time.Sleep(keyGap)
			_, _ = io.WriteString(input, keyBytes[Enter])
		}
	}
}

// launcherTab is the app's side: a launcher connected to it.
type launcherTab struct {
	agent  Agent
	folder string
	// The role it was started with, by its id, or none.
	role string
	// The desk the office hired it at, for the first session in it, or -1;
	// and its floor, 0 for the ground floor, whose desks are the keys.
	desk  int
	floor int
	// Whether the office hired it, so its first session sits where it was hired.
	hired bool
	// Whether it was started by the office with a hire token (the "hire" query of its own
	// stream). Only such a launcher can be a hire's window for mail; set once, never shown.
	office bool
	// The engine hire this launcher is the window of (hirelauncher.go), from the
	// hire's one-time token and nothing the launcher says; "" for any other.
	hireID   string
	commands chan launcherCommand
	// The secret the launcher proves its screen answers with, sent down its own
	// stream only; and a channel closed when the launcher is gone.
	screenSecret string
	gone         chan struct{}
}

func (t *launcherTab) send(command launcherCommand) error {
	select {
	case t.commands <- command:
		return nil
	default:
		return errors.New("the launcher isn't keeping up")
	}
}

func (t *launcherTab) Press(k Keystroke) error { return t.send(launcherCommand{Press: &k}) }
func (t *launcherTab) Type(text string) error  { return t.send(launcherCommand{Type: text}) }

// Paste puts text in the agent's box without sending it. Only mail uses it.
func (t *launcherTab) Paste(text string) error { return t.send(launcherCommand{Paste: text}) }

// Focus does nothing: the launcher can't tell which window it runs in, so
// selecting the key on the phone is all a tap can do.
func (t *launcherTab) Focus() error { return nil }

// Close ends the agent. The window it runs in is the terminal's own, which
// the launcher can't reach, so that stays open.
func (t *launcherTab) Close(Agent) error { return t.send(launcherCommand{Stop: true}) }

// Scrolling belongs to the terminal around the launcher, which it can't reach.
func (t *launcherTab) Scroll(int) error      { return errUnsupported }
func (t *launcherTab) ScrollToBottom() error { return errUnsupported }
