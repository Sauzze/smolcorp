package desktop

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// What a first General Manager hire needs, checked before Start here's Hire
// button: GET /setup/hire/gm/preflight, the same JSON as the Mac app's
// (GMPreflight.swift). Each line says what's wrong in one sentence and exactly
// what to do: a command to copy, a fix to press, or where to go on the page.
// Only looks: the fixes are the page's buttons.
//
// Signed in is what Claude Code's own `claude auth status` says, as Summary
// asks it: no credential, key or token is ever read.

// preflightAction is what a line's button does: "copy" (a command to paste in
// a terminal), "fix" (a POST /setup/fix by name) or "folder" (the folder step above).
// The Mac also has "account" (its Claude accounts), for a test copy's line.
type preflightAction struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Command string `json:"command,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

type preflightItem struct {
	ID     string           `json:"id"`
	OK     bool             `json:"ok"`
	Title  string           `json:"title"`
	Fix    string           `json:"fix"` // what to do, "" when there's nothing to
	Action *preflightAction `json:"action"`
	// Optional is a line not in the way of the hire: it starts without it.
	Optional bool `json:"optional"`
	// Todo is a step not taken yet, not a problem: the page shows it as To do, never in red (the Mac's Item.todo).
	Todo bool `json:"todo"`
}

type preflightReport struct {
	Ready bool            `json:"ready"` // nothing but an optional line left
	Items []preflightItem `json:"items"`
}

// chooseHireFolderFirst is Start here's GM hire with no folder for your hires: the page points
// at its folder step (#first-folder). The Mac says the same (DeckRouter.swift).
const chooseHireFolderFirst = "Choose the folder for your hires first: it's the first step of Start here."

// gmNeedsAccount is a test copy's account line, in the Mac's words (GMPreflight.needsAccount).
const gmNeedsAccount = "Your General Manager needs a Claude account of its own (this is a test copy)."

// gmBeingHired answers a GM hire while another one runs, as the Mac does (Deck.gmRefusal).
const gmBeingHired = "A General Manager is being hired right now. Give it a moment."

// Whether Claude Code is signed in, as it says itself.
type signIn int

const (
	signInUnknown signIn = iota
	signedIn
	signedOut
)

// Claude Code's own installer, for where hires run.
const (
	claudeInstallUnix    = "curl -fsSL https://claude.ai/install.sh | bash"
	claudeInstallWindows = "irm https://claude.ai/install.ps1 | iex"
)

// claudeFoundAt is where Smolcorp finds Claude Code, "" nowhere: a variable, for the tests.
var claudeFoundAt = func() string {
	if found := whereAppFinds("claude"); found != "" {
		return found
	}
	if runtime.GOOS == "windows" {
		if candidate := filepath.Join(homeDir, ".local", "bin", "claude.exe"); fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

// claudeSignIn asks program whether it's signed in, as a hire starts it: through your login shell, with
// the hire's environment, so a key or a cloud provider your shell's files set counts. A variable, for the tests.
var claudeSignIn = func(program string) signIn {
	command := signInCommand(program)
	return signInFrom(quickRun(command[0], command[1:], signInEnvironment()).output)
}

// signInCommand is `claude auth status --json` the way a hire's command is run (startHire, inLoginShell), on
// ~/.claude's account as the GM's hire is: its start script clears CLAUDE_CONFIG_DIR after your shell's files
// have run, so this clears it there too (the Mac's GMPreflight.signInArguments). Windows has no login shell.
func signInCommand(program string) []string {
	if runtime.GOOS == "windows" {
		return []string{program, "auth", "status", "--json"}
	}
	return inLoginShell([]string{"env", "-u", "CLAUDE_CONFIG_DIR", program, "auth", "status", "--json"})
}

// signInEnvironment is the hire's environment without CLAUDE_CONFIG_DIR, as the launcher drops it for the GM.
func signInEnvironment() []string {
	return slices.DeleteFunc(hireEnvironment(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return strings.EqualFold(name, "CLAUDE_CONFIG_DIR")
	})
}

// signInFrom reads `claude auth status --json`, after whatever your shell's files print as it starts.
func signInFrom(status []byte) signIn {
	text := string(status)
	for start := 0; start < len(text); {
		var said struct {
			LoggedIn *bool `json:"loggedIn"`
		}
		if text[start] == '{' && json.Unmarshal([]byte(text[start:]), &said) == nil && said.LoggedIn != nil {
			if *said.LoggedIn {
				return signedIn
			}
			return signedOut
		}
		next := strings.Index(text[start:], "\n{")
		if next < 0 {
			break
		}
		start += next + 1
	}
	return signInUnknown
}

// hasProgram is whether name is on this app's PATH: a variable, for the tests.
var hasProgram = func(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// tmuxInstall is the command that installs tmux with this computer's package manager, "" if none is known.
func tmuxInstall() string {
	for _, pm := range [][2]string{{"apt-get", "sudo apt install tmux"}, {"dnf", "sudo dnf install tmux"}, {"pacman", "sudo pacman -S tmux"},
		{"zypper", "sudo zypper install tmux"}, {"brew", "brew install tmux"}} {
		if hasProgram(pm[0]) {
			return pm[1]
		}
	}
	return ""
}

// pathLine is the command that adds folder to the PATH of shell, a login shell, in the file it reads.
// A folder with a quote, a $ or a space in its name still gives a line that works (the Mac's is the same).
func pathLine(shell, folder string) string {
	shell = filepath.Base(shell)
	if shell == "fish" {
		if strings.Trim(folder, plainPathCharacters) == "" {
			return "fish_add_path " + trimmed(folder)
		}
		return `fish_add_path '` + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(folder) + `'`
	}
	rest, underHome := strings.CutPrefix(folder, homeDir+"/")
	if !underHome {
		rest = folder
	}
	// In the profile, the folder sits inside "…"; on the command line, that line sits inside '…'.
	rest = strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "`", "\\`").Replace(rest)
	if underHome {
		rest = "$HOME/" + rest
	}
	line := `export PATH="` + rest + `:$PATH"`
	return `echo '` + strings.ReplaceAll(line, "'", `'\''`) + `' >> ` + loginFile(shell)
}

// plainPathCharacters are those a folder can have without quoting it in a shell.
const plainPathCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+~@%:,"

// gmPreflight is the report, without the lock held.
func (d *Deck) gmPreflight() preflightReport {
	d.mu.Lock()
	folder := strings.TrimSpace(d.config.HireFolder)
	chosen, inTmux := d.config.HireTerminal, d.config.HireInTmux
	d.mu.Unlock()
	items := []preflightItem{}
	add := func(item preflightItem) { items = append(items, item) }
	show := &preflightAction{Kind: "folder", Label: "Show me"}
	if folder == "" {
		add(preflightItem{ID: "folder", Title: "There's no folder for your hires yet.", Fix: "Choose it in the first step above.", Action: show, Todo: true})
	} else if _, ok := existingFolder(folder); !ok {
		add(preflightItem{ID: "folder", Title: "The folder for your hires isn't there any more: " + trimmed(folder) + ".",
			Fix: "Choose it again in the first step above.", Action: show})
	} else {
		add(preflightItem{ID: "folder", OK: true, Title: "Your hires work in " + trimmed(folder) + "."})
	}
	windows := runtime.GOOS == "windows"
	shellName := "a terminal"
	install := claudeInstallUnix
	if windows {
		shellName, install = "PowerShell", claudeInstallWindows
	}
	missing := agentMissing("claude") != "" // where hires start, your login shell, doesn't find it
	at := claudeFoundAt()
	switch {
	case at == "" && (missing || windows):
		add(preflightItem{ID: "claude", Title: "Claude Code isn't installed on this computer.",
			Fix: "Paste this in " + shellName + ", wait for it to finish, then press Check again.", Action: &preflightAction{Kind: "copy", Label: "Copy", Command: install}, Todo: true})
	case missing:
		where := filepath.Dir(at)
		add(preflightItem{ID: "claude", Title: "Claude Code is in " + trimmed(where) + ", where your terminal doesn't look for it.",
			Fix:    "Paste this in a terminal, open a new terminal window, then press Check again.",
			Action: &preflightAction{Kind: "copy", Label: "Copy", Command: pathLine(loginShell(), where)}})
	default:
		add(preflightItem{ID: "claude", OK: true, Title: "Claude Code is installed."})
		state := signInUnknown
		if at != "" {
			state = claudeSignIn(at)
		}
		switch state {
		case signedIn:
			add(preflightItem{ID: "signin", OK: true, Title: "Claude Code is signed in."})
		case signedOut:
			// Said, never in the way: an API key or a cloud provider can work where this can't see it.
			add(preflightItem{ID: "signin", Optional: true, Title: "Claude Code isn't signed in.",
				Fix:    "Hire anyway: its window asks you to sign in. Or sign in first: paste this in " + shellName + " (type /login if it doesn't ask), then press Check again.",
				Action: &preflightAction{Kind: "copy", Label: "Copy", Command: "claude"}})
		default:
			add(preflightItem{ID: "signin", Optional: true, Title: "Smolcorp couldn't tell if Claude Code is signed in.",
				Fix: "You can hire anyway: if your General Manager asks you to sign in, type /login in its window."})
		}
		if hooksNotConnected(Claude) != "" {
			add(preflightItem{ID: "hooks", Title: "Claude Code doesn't report to Smolcorp yet, so your General Manager would never reach a desk.",
				Fix: "Press Connect, then Check again.", Action: &preflightAction{Kind: "fix", Label: "Connect", Fix: fixRepair}, Todo: true})
		} else {
			add(preflightItem{ID: "hooks", OK: true, Title: "Claude Code reports to Smolcorp."})
		}
	}
	// A test copy never hires Claude Code on this computer (testCopyRefusal): said before Hire, not after. The Mac's
	// test copy can hire on a second account (GMPreflight.accountItems); this one can't yet, so it's a line with no fix here.
	// Not Todo, unlike the Mac's: there's no step to take here, so it stays red (!).
	if testCopyPort() != 0 {
		add(preflightItem{ID: "account", Title: gmNeedsAccount, Fix: "A test copy on this computer can't use one yet: hire your General Manager from the real Smolcorp."})
	}
	terminals, tmux := hireWays()
	terminal, hasTerminal := hireTerminal(chosen, terminals)
	tmuxCommand := tmuxInstall()
	var tmuxAction *preflightAction
	if tmuxCommand != "" {
		tmuxAction = &preflightAction{Kind: "copy", Label: "Copy", Command: tmuxCommand}
	}
	switch {
	case hasTerminal:
		add(preflightItem{ID: "terminal", OK: true, Title: "Your hires open in " + terminal.name + "."})
	case tmux:
		add(preflightItem{ID: "terminal", OK: true, Title: "Your hires run in tmux: there's no desktop here to open a window on."})
	default:
		fix := "Install tmux with your system's package manager, then press Check again."
		if tmuxAction != nil {
			fix = "Paste this in a terminal to install tmux, then press Check again."
		}
		add(preflightItem{ID: "terminal", Title: "There's no terminal on this computer to open your General Manager in.", Fix: fix, Action: tmuxAction})
	}
	if hasTmuxHires && inTmux && hasTerminal {
		if tmux {
			add(preflightItem{ID: "tmux", OK: true, Title: "tmux is installed."})
		} else {
			fix := "You can hire anyway, in a plain " + terminal.name + " window. For tmux, install it with your system's package manager, then press Check again."
			if tmuxAction != nil {
				fix = "You can hire anyway, in a plain " + terminal.name + " window. For tmux: paste this in a terminal, then press Check again."
			}
			add(preflightItem{ID: "tmux", Optional: true, Title: "tmux isn't installed, so your hires open without it.", Fix: fix, Action: tmuxAction})
		}
	}
	ready := true
	for _, item := range items {
		ready = ready && (item.OK || item.Optional)
	}
	return preflightReport{Ready: ready, Items: items}
}
