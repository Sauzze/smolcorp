package desktop

import (
	"bufio"
	"bytes"
	"context"
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
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/claudeenv"
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
	// For an engine hire (hirelauncher.go): its id. The launcher then asks the app for
	// the hire's environment and its settings before it starts anything.
	hireID string
	// For an office's plain hire on a Mac (hire.go, ruling (W)): the desk the office made its worktree for, whose
	// sandbox the launcher makes (newLauncherSandboxAt). "" for a new one.
	desk string
	// For such a hire of a role, the project the office picked for it, by its name in the folder for hires, and its
	// own worktree's branch ("" for a project that isn't a repo): it is told that project rather than asked which
	// (FXPICKEDPROJECT). "" for none picked.
	project, branch string
	// For a plain hire of Claude Code (hire.go) on an account other than ~/.claude: that account's folder, from
	// setup's confirmed list. See plainHireEnv.
	claudeConfig string
	// For a hire of a role, its agent as the office checked it could start: the
	// role must still have it.
	agent string
	// For a plain hire on a Mac made while the owner had the sandbox off (hire.go): its agent starts outside one, if
	// the switch still reads off as it starts (sandboxed).
	sandboxOff bool
	// Whether the agent got as far as starting.
	started bool
}

// parseRun reads `smolcorp run`'s arguments: [--hire <token> [--hire-id <id> |
// [--desk <desk> [--project <name> [--branch <branch>]]] [--claude-config <folder>]] [--pinned] [--sandbox-off]
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
		if len(args) >= 2 && args[0] == "--desk" {
			if run.hireID != "" || !plainDeskID(args[1]) {
				return nil, errors.New("not a plain hire's desk")
			}
			run.desk, args = args[1], args[2:]
			if len(args) >= 2 && args[0] == "--project" {
				if !plainProjectName(args[1]) {
					return nil, errors.New("not a project's name")
				}
				run.project, args = args[1], args[2:]
				if len(args) >= 2 && args[0] == "--branch" {
					// Checked against its role's once that's read, below.
					run.branch, args = args[1], args[2:]
				}
			}
		}
		if len(args) >= 2 && args[0] == "--claude-config" {
			// The app's own word for a plain hire's account, a full path: an engine hire's comes with its start settings.
			if run.hireID != "" || !filepath.IsAbs(args[1]) || filepath.Clean(args[1]) != args[1] {
				return nil, errors.New("not an account's folder")
			}
			run.claudeConfig, args = args[1], args[2:]
		}
		if len(args) > 0 && args[0] == "--pinned" {
			run.pinned, args = true, args[1:]
		}
		if len(args) > 0 && args[0] == "--sandbox-off" {
			// A plain hire's word alone: an engine hire's start goes by the engine's own (hireengine.go).
			if run.hireID != "" {
				return nil, errors.New("--sandbox-off is for a plain hire")
			}
			run.sandboxOff, args = true, args[1:]
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
		if len(args) != 2 {
			return nil, errors.New("a role, and nothing else")
		}
		run.role = args[1]
	default:
		run.command = args
	}
	// A branch told is the one prepareHire made for this desk's hire of this role, named as it names it, whatever the
	// role's id (FXPICKEDPROJECTR1, B2): hire/<desk>/<role>-<desk's 6>.
	if run.branch != "" && (run.role == "" || run.branch != hireBranch(run.desk, hireWorktreeName(run.role, run.desk))) {
		return nil, errors.New("not this desk's branch")
	}
	return run, nil
}

// sandboxed is whether this launcher starts its agent in a sandbox of its own: a plain hire's (an engine hire's
// starts in its desk's tmux, and one run by hand is in none), on a system whose launcher makes one, unless the owner
// turned the sandbox off (FXSWITCHPLAIN). Off only when both say so: the core, whose switch is sealed, at the hire
// (--sandbox-off), and the config as its file holds it now (plainHireSandboxed, as the engine reads the switch at
// each start). A file another program changed alone, or a word on the command line alone, never takes a sandbox away.
func (run *launcherRun) sandboxed() bool {
	if !launcherSandboxes || run.hire == "" || run.hireID != "" {
		return false
	}
	return !run.sandboxOff || plainHireSandboxed(*savedConfig())
}

// sandboxOnAgainWords say why a plain hire the office made while the owner had the sandbox off can't start now in
// folder, or "" when it can (FXSTACKG G22): with the switch on again by its launcher's start (sandboxed: the file no
// longer says off), it would start in its sandbox, but it has no desk of its own (--desk): the office made no worktree
// for it, so in a repo it would be sandboxed in the owner's checkout, which ruling (W) (FXHIREPROJECTSR7) keeps every
// sandboxed plain hire out of. It never starts outside the sandbox either. A folder git finds no repo in it works in
// itself, as ruling (W) keeps for one (its start still refuses one that holds a repo, or the projects folder); any
// other answer is taken as a repo (gitFindsNoRepo). The GM's place is its own, with or without the switch.
func (run *launcherRun) sandboxOnAgainWords(sandboxed bool, folder string) string {
	if !sandboxed || !run.sandboxOff || run.desk != "" || run.role == gmRole {
		return ""
	}
	if gitFindsNoRepo(folder) {
		return "" // in no repo
	}
	return plainHireSandboxOnAgain
}

// gitNoRepoWords are git's own words when it finds no repo in a folder or any folder above it (in git's own
// language, LC_ALL=C). A .git file naming no git folder says "not a git repository: <path>", and a stop at another
// file system's mount "(or any parent up to mount point …)": neither is these words.
const gitNoRepoWords = "fatal: not a git repository (or any of the parent directories)"

// gitFindsNoRepo is whether git finds folder in no repo, by the launcher's refusal's own read (FXSTACKFIX5R1,
// RT126STACKFIX5 N1, O1, O2). It is a read as every other is (gitReadArgs, gitReadEnv, the owner's trusted folders),
// but it looks as far up as a repo goes: a launcher runs with its terminal's environment, and a ceiling there
// (GIT_CEILING_DIRECTORIES) has git answer "not a git repository" for a folder inside one, so this read drops it; and
// git stops at another file system's mount unless GIT_DISCOVERY_ACROSS_FILESYSTEM=1, so this read sets it. The
// terminal's GIT_TRACE settings are dropped too: their lines would come before git's own words (RT126STACKFIX5R1 O3).
// Only gitNoRepoWords, with git's exit 128, count as no repo; any other failure is taken as a repo: one git refuses as
// another account's (GIT_TEST_ASSUME_DIFFERENT_OWNER is kept, gitConfigNames), a .git file naming no git folder, git
// missing, too slow or saying too much. A hire's own tree is a repo, and git isn't run there outside its sandbox.
// Only this read: every other one keeps the owner's ceiling, so that, at a folder hired into, a start's own read of
// the repo (deskGitPlaces) takes a repo the owner hid from git for none, and the sandbox's rows hold its .git whole.
func gitFindsNoRepo(folder string) bool {
	args := []string{"rev-parse", "--show-toplevel"}
	if _, mine := deskGitForAs(folder, args); mine {
		return false
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return false
	}
	env := slices.DeleteFunc(gitReadEnv(os.Environ(), gitHere().owner), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		name = strings.ToUpper(name)
		return name == "GIT_CEILING_DIRECTORIES" || name == "GIT_DISCOVERY_ACROSS_FILESYSTEM" || name == "LC_ALL" ||
			strings.HasPrefix(name, "GIT_TRACE")
	})
	env = append(env, "GIT_DISCOVERY_ACROSS_FILESYSTEM=1", "LC_ALL=C")
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	var stdout, stderr gitOutput
	err = deskRun(ctx, "", folder, env, &stdout, &stderr, git, append(append([]string{"-C", folder}, gitReadArgs...), args...)...)
	var exit *exec.ExitError
	return errors.As(err, &exit) && ctx.Err() == nil && !stdout.over && !stderr.over && exit.ExitCode() == 128 &&
		strings.HasPrefix(strings.TrimSpace(stderr.String()), gitNoRepoWords)
}

// plainHireSandboxOnAgain is a plain hire's launcher's words when the sandbox was turned on again between its hire
// and its start (sandboxOnAgainWords).
const plainHireSandboxOnAgain = "This hire was made while the sandbox was off, and the sandbox is on again now, so it didn't start: " +
	"on this Mac a hire in its sandbox works in a worktree of its own, and this one has none. Close this window and hire again."

func launch(run *launcherRun) int {
	args := run.command
	folder, _ := os.Getwd()
	// Whether its agent starts in a sandbox of its own, read once, before its first message is made (FXSWITCHPLAIN).
	sandboxed := run.sandboxed()
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
	// A General Manager hired the plain way (hire.go) gets its token from the app the same way, and nothing else
	// with it: a GM can't work without it, so one that can't be had starts nothing (GMTOKENWIN1). Never through the
	// window's environment: a terminal may start its window with an environment of its own.
	gmToken := ""
	if run.hireID == "" && run.hire != "" && run.role == gmRole {
		key, _ := os.ReadFile(hookKeyFile)
		got, err := fetchHireStartEnv(hireStartClient(), fmt.Sprintf("http://127.0.0.1:%d", Port), string(key), run.hire, "")
		if gmToken = got.Set[gmTokenEnvironment]; err == nil && gmToken == "" {
			err = errors.New("Smolcorp gave no token")
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: couldn't get the General Manager's token, so nothing was started: %v\n", err)
			return 1
		}
	}
	if run.role != "" {
		message := ""
		if startEnv != nil {
			message = startEnv.Message
		}
		var told []string
		if run.role == gmRole && launcherSandboxes && run.hire != "" && run.hireID == "" && !sandboxed {
			told = append(told, gmSandboxOffLine) // told it starts outside its sandbox, as the engine's GM is (§A5.2, AQ1)
		}
		found, err := run.roleStartArguments(folder, message, told...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
			return 2
		}
		if err := sameAgentAsHired(run, found[0]); err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
			return 2
		}
		args = found
		if startEnv != nil {
			// The engine hire's own settings file, for Claude Code only, right after the agent, named with its
			// environment (SANDBOX-PLAN §3.4). Without it, nothing starts unguarded.
			if agentNamed(found[0]) != Claude {
				fmt.Fprintln(os.Stderr, "smolcorp: a hire's settings are for Claude Code, and this role's agent is another")
				return 2
			}
			// Its settings file, checked against its seal as late as can be (settingsfile.go): only the file's name rides here.
			if refusal := (sealedSettings{Path: startEnv.Settings, Sum: startEnv.SettingsSum}).refusal(); refusal != nil {
				fmt.Fprintln(os.Stderr, "smolcorp: "+refusal.Message)
				return 1
			}
			args = append([]string{found[0], "--settings=" + startEnv.Settings}, found[1:]...)
		}
	}
	if run.claudeConfig != "" && agentNamed(args[0]) != Claude {
		fmt.Fprintln(os.Stderr, "smolcorp: a Claude account is for Claude Code, and this hire's agent is another")
		return 2
	}
	program, programArgs, err := resolveProgram(args[0], args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
		return 127
	}
	// The agent found is checked as every agent CLI the core starts is (agentcheck.go; SANDBOX-PLAN §3.4, g7): a
	// hire can write the folders it's found in. A refused one never runs (RT126SG2 B3).
	if err := checkAgentProgram(string(agentNamed(args[0])), program); err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: %v No desk was started.\n", err)
		return 126
	}
	if run.role != "" && program != args[0] && len(programArgs) > 0 && programArgs[0] == "/c" {
		if programArgs, err = forCmd(programArgs); err != nil {
			fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
			return 2
		}
	}
	// cmd.exe takes a line of at most 8191 characters, and a desk's settings, escaped twice for npm's script, can be
	// longer: refused here rather than cut by cmd.exe.
	if startEnv != nil && len(programArgs) > 1 && programArgs[0] == "/c" && len(cmdLine(program, programArgs[1], programArgs[2:])) > cmdLongest {
		fmt.Fprintln(os.Stderr, "smolcorp: this Claude Code was installed with npm, and its script can't take a desk's settings on this computer. "+
			"Install Claude Code with Anthropic's own installer, then hire again.")
		return 2
	}
	// Windows starts a program with a command line of at most 32767 characters, a desk's settings among them: a
	// longer one is refused here, in words, rather than left to fail to start.
	if startEnv != nil && runtime.GOOS == "windows" && !(len(programArgs) > 0 && programArgs[0] == "/c") && len(exeLine(program, programArgs)) > exeLongest {
		fmt.Fprintln(os.Stderr, "smolcorp: this desk's settings and first message make a command line longer than Windows takes, so nothing was started. "+
			"Fewer guard rules or saved Claude accounts in Settings, or a shorter brief, make it shorter.")
		return 2
	}

	// A plain hire's agent starts in a sandbox of its own where this system has one (a Mac's), its own sandbox switched
	// off (launchersandbox_darwin.go, agentownsandbox.go): one that can't be is refused, never started outside it.
	// With Smolcorp's sandbox off (sandboxed), it starts as it would without Smolcorp, its own sandbox as the owner set it.
	id := newID()
	var walled *launcherSandbox
	var ownOff map[string]string
	if sandboxed {
		if why := run.sandboxOnAgainWords(sandboxed, folder); why != "" {
			fmt.Fprintln(os.Stderr, "smolcorp: "+why)
			return 126
		}
		// Codex, Gemini CLI, OpenCode and Grok can't start in it yet (notonamac.go): every hire route refuses them
		// before a window opens while the sandbox is in force, and this refuses one that got here anyway, before
		// anything of the desk is made. With the owner's switch off (sandboxed false) it starts without one, tagged.
		if why := notOnAMacYet(agentNamed(args[0]), sandboxed); why != "" {
			fmt.Fprintln(os.Stderr, "smolcorp: "+why)
			return 126
		}
		words, set, refusal := ownSandboxOffFor(agentNamed(args[0]), program)
		if refusal != "" {
			fmt.Fprintln(os.Stderr, "smolcorp: "+refusal)
			return 126
		}
		programArgs, ownOff = append(words, programArgs...), set
		if walled, err = newLauncherSandboxAt(run.desk, folder, run.role == gmRole, id, agentNamed(args[0]), program); err != nil {
			fmt.Fprintln(os.Stderr, "smolcorp: "+err.Error())
			return 1
		}
		defer walled.close()
		walled.closeOnHangup()
	}

	terminal, err := pty.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: no pseudo-terminal: %v\n", err)
		return 1
	}
	defer terminal.Close()

	// Not through disclaim.Words: smolcorp run runs only in a hire's terminal window or a desk's tmux (itself
	// started through disclaim.Cmd), so the agent is charged to that terminal or tmux, never to Smolcorp.app. This
	// function is exempt as a whole for it (spawnsites_test.go): a new start added in here would pass unseen.
	cmd := terminal.Command(program, programArgs...)
	useCmdLine(cmd, program, programArgs)
	cmd.Env = agentEnvironment(os.Environ(), id, run.hire != "", agentNamed(args[0]) == Claude, run.claudeConfig, startEnv, runtime.GOOS == "windows")
	if run.hire != "" && startEnv == nil {
		// A plain hire's own token, on the GM's process alone; any other plain hire never acts as the GM, whatever
		// its window inherited (an engine hire's start says the same: hirelauncher.go).
		cmd.Env = plainGMEnv(cmd.Env, gmToken, runtime.GOOS == "windows")
	}
	cmd.Dir = folder
	var agent agentStart = ptyAgent{cmd}
	if walled != nil {
		cmd.Env = walled.environment(cmd.Env, ownOff)
		if agent, err = walled.command(cmd, terminal); err != nil {
			fmt.Fprintln(os.Stderr, "smolcorp: "+err.Error())
			return 1
		}
	}
	rows, cols, sized := consoleSize()
	screen := newLauncherScreen(rows, cols, sized)
	matchSize(terminal, screen)
	if err := agent.Start(); err != nil {
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
	// A Mac's plain hire started outside its sandbox (the owner's switch off) says so each time it connects, so its desk
	// keeps its No sandbox tag after the core restarts, its token spent (FXSWITCHPLAIN).
	unsandboxed := launcherSandboxes && run.hire != "" && run.hireID == "" && walled == nil
	go serveDeck(id, agentNamed(args[0]), folder, run.role, run.hire, unsandboxed, input, cursor, screen, func() { stopProcess(agent.process()) })

	if err := agent.Wait(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		return 1
	}
	return 0
}

// agentStart is the agent's process as the launcher starts it: in its pseudo-terminal (ptyAgent), or in its sandbox
// there (launcherSandbox.command).
type agentStart interface {
	Start() error
	Wait() error
	process() *os.Process
}

// ptyAgent is the agent started in its pseudo-terminal as it is.
type ptyAgent struct{ *pty.Cmd }

func (a ptyAgent) process() *os.Process { return a.Process }

// agentEnvironment is what smolcorp run starts its agent with: base, its own environment, never with the markers of
// a Claude Code session its terminal was opened in (claudeenv, ENVSCRUB1), and its session's id. A hire's
// (hire), without what wraps hooks, then its start's own variables (startEnv, on the agent's process alone), or a plain
// Claude hire's account. caseless: names compared as Windows does.
func agentEnvironment(base []string, id string, hire, claude bool, claudeConfig string, startEnv *hireStartEnv, caseless bool) []string {
	env := append(claudeenv.Scrub(base), "SMOLCORP_SESSION="+id)
	if hire {
		// A hire's window may have run your shell's start files before this (the terminal's own shell): what they
		// set to wrap or switch off its hooks isn't passed on (§4 g10).
		env = slices.DeleteFunc(env, func(variable string) bool { name, _, _ := strings.Cut(variable, "="); return wrapsHooks(name) })
	}
	if startEnv != nil {
		return startEnvironment(env, *startEnv, caseless)
	}
	if hire && claude {
		return plainHireEnv(env, claudeConfig, caseless)
	}
	return env
}

// plainHireEnv is a plain hire's environment (hire.go: a pinned desk, another agent, no role), for Claude Code: on
// the account the office hired it on, never the one the terminal's shell names (PLAINENV1). folder is that account's,
// from setup's confirmed list, or "" for ~/.claude: CLAUDE_CONFIG_DIR is set to it, or taken out. So its transcripts
// are in a folder the deck takes them from (transcriptKnown), as an engine hire's are (hirestart.go).
func plainHireEnv(env []string, folder string, caseless bool) []string {
	change := hireStartEnv{Unset: []string{"CLAUDE_CONFIG_DIR"}}
	if folder != "" {
		change = hireStartEnv{Set: map[string]string{"CLAUDE_CONFIG_DIR": folder}}
	}
	return startEnvironment(env, change, caseless)
}

// plainGMEnv is a plain hire's environment with token, the General Manager's, set on it, or with any GM token it
// inherited taken out when token is "": of the hires the office makes the plain way, the GM alone gets one
// (GMTOKENWIN1). Names are compared as the OS does.
func plainGMEnv(env []string, token string, caseless bool) []string {
	change := hireStartEnv{Unset: []string{gmTokenEnvironment}}
	if token != "" {
		change = hireStartEnv{Set: map[string]string{gmTokenEnvironment: token}}
	}
	return startEnvironment(env, change, caseless)
}

// cmdLongest is the longest line cmd.exe runs.
const cmdLongest = 8191

// exeLongest is the longest command line Windows starts a program with: 32767 characters, its ending one included.
const exeLongest = 32766

// exeLine is the command line Windows is given for program and args, each quoted as Go quotes it there
// (syscall.EscapeArg), written here so its length can be told on any system.
func exeLine(program string, args []string) string {
	quoted := func(s string) string {
		if s == "" {
			return `""`
		}
		if !strings.ContainsAny(s, "\\\" \t") {
			return s
		}
		space := strings.ContainsAny(s, " \t")
		var b strings.Builder
		if space {
			b.WriteByte('"')
		}
		slashes := 0
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '\\':
				slashes++
			case '"':
				b.WriteString(strings.Repeat(`\`, slashes+1))
				slashes = 0
			default:
				slashes = 0
			}
			b.WriteByte(s[i])
		}
		if space {
			b.WriteString(strings.Repeat(`\`, slashes) + `"`)
		}
		return b.String()
	}
	line := quoted(program)
	for _, arg := range args {
		line += " " + quoted(arg)
	}
	return line
}

// resolveProgram finds the agent on PATH, or where the core finds agents (programNamed: your login shell's PATH,
// asked with nothing of yours around it, and the folders their installers use), since a hire's window no longer
// starts it through your login shell (RT126SG2 B3). On Windows, npm installs agents as
// .cmd scripts, which a pseudo-console can't start by themselves.
func resolveProgram(name string, args []string) (string, []string, error) {
	path, err := exec.LookPath(name)
	if err != nil && !strings.ContainsRune(name, filepath.Separator) {
		if found := programNamed(name); found != "" {
			path, err = found, nil
		}
	}
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

// roleStartArguments are the agent and its arguments for run's role, started in folder as its hire came to start
// there: from a terminal, at a pinned desk, into the folder for hires, or into the project the office picked for it.
// message, unless it's empty, is its first message instead of the role's own (roleArguments). told are lines added at
// its end.
func (run *launcherRun) roleStartArguments(folder, message string, told ...string) ([]string, error) {
	how := startedHere
	if run.hire != "" {
		how = map[bool]roleStart{true: hiredAtPinnedDesk, false: hiredIntoGeneralFolder}[run.pinned]
	}
	return roleArgumentsIn(run.role, folder, how, pickedStart{project: run.project, branch: run.branch}, message, told...)
}

// roleArguments are the agent and its arguments for the role with the id
// given, as set up on the setup page, started in folder as how says. message,
// unless it's empty, is its first message instead of the role's own: the one
// the app made with the hire's brief. told are lines added at its end.
func roleArguments(id, folder string, how roleStart, message string, told ...string) ([]string, error) {
	return roleArgumentsIn(id, folder, how, pickedStart{}, message, told...)
}

// roleArgumentsIn is roleArguments for a hire whose project the office picked, when picked names one: the role's own
// first message tells it so (Role.firstMessageIn).
func roleArgumentsIn(id, folder string, how roleStart, picked pickedStart, message string, told ...string) ([]string, error) {
	roles := loadConfig().Roles
	for _, role := range roles {
		if role.ID == id {
			// An ACP role's agent is "acp:<id>", not a program: hiring it is A5's, and until then a launcher
			// started by hand never looks that name up on PATH.
			if Agent(role.Agent).IsACP() {
				return nil, errors.New(acpNotHiredYet)
			}
			if message == "" {
				message = role.firstMessageIn(folder, how, picked)
			}
			for _, line := range told {
				message += " " + line
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

// consoleSize is the console's rows and columns, and whether they could be read. A variable for the launcher
// stand-in alone (voicewin_test.go), which has no console of its own and is given one's size.
var consoleSize = func() (rows, cols int, ok bool) {
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
func serveDeck(id string, agent Agent, folder, role, hire string, unsandboxed bool, input io.Writer, cursor *cursorMode, screen *launcherScreen, stop func()) {
	query := url.Values{"id": {id}, "agent": {string(agent)}, "folder": {folder}, "role": {role}, "hire": {hire}}
	if unsandboxed {
		query.Set("sandbox", "off")
	}
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
	hireID string
	// A plain hire the office made on a Mac while the owner had the sandbox off (hire.go): its agent runs outside a
	// sandbox, and Setup lists it so (plainHireTag). From its hire's token, nothing the launcher says.
	sandboxOff bool
	// The plain hire with a worktree of its own this launcher is the window of (plainhireend.go): from its token, or
	// from its folder once it connects again without it; "" for any other.
	plainDesk string
	commands  chan launcherCommand
	// The secret the launcher proves its screen answers with, sent down its own
	// stream only; and a channel closed when the launcher is gone.
	screenSecret string
	gone         chan struct{}
	// The ends of its connection to this app as the app saw them, and the launcher's own process, found from them
	// once it's asked for (process, in preview.go). Under conn.
	conn             sync.Mutex
	connFrom, connTo string
	pid              int
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
