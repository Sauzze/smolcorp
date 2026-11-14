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
// a terminal), "fix" (a POST /setup/fix by name), "folder" (the folder step above) or "account" (the
// Claude accounts, for a Mac's test copy's line).
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

// gmNeedsAccount is a test copy's account line, in the Mac's words (GMPreflight.needsAccount), and what to do on a
// Mac, where a test copy hires on an account of its own: add one, or choose one beside Hire (the page matches these
// words for the refusal's button).
const (
	gmNeedsAccount     = "Your General Manager needs a Claude account of its own (this is a test copy)."
	gmAddAccountFix    = "Add one under Claude accounts: sign in to a second account there, then add it."
	gmChooseAccountFix = "Choose one other than default, beside Hire your General Manager below."
)

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

// claudeSignIn asks program whether it's signed in on the account whose config folder is configDir ("" for
// ~/.claude's), as a desk starts it, with the hire's environment, so a key or a cloud provider in it counts. A
// variable, for the tests.
var claudeSignIn = func(program, configDir string) signIn {
	// A Claude Code that isn't Anthropic's is never run (agentcheck.go): it can't say.
	if checkAgentProgram(string(Claude), program) != nil {
		return signInUnknown
	}
	command := signInCommand(program)
	return signInFrom(quickRun(command[0], command[1:], signInEnvironment(configDir)).output)
}

// signInCommand is `claude auth status --json`, run as a desk's Claude Code is started: the program itself, no
// login shell (SANDBOX-PLAN §3.4, SG2), on the account the GM's hire would work on, which signInEnvironment names
// (CLAUDE_CONFIG_DIR set to the account's folder, or cleared for ~/.claude's, configDir ""). Checked before it
// runs, as every Claude Code the core starts (spawnCapped).
func signInCommand(program string) []string {
	return []string{program, "auth", "status", "--json"}
}

// signInEnvironment is the hire's environment without CLAUDE_CONFIG_DIR, as the launcher drops it for the GM, and
// with the account's own folder when one is named.
func signInEnvironment(configDir string) []string {
	env := slices.DeleteFunc(hireEnvironment(), func(variable string) bool {
		name, _, _ := strings.Cut(variable, "=")
		return strings.EqualFold(name, "CLAUDE_CONFIG_DIR")
	})
	if configDir != "" {
		env = append(env, "CLAUDE_CONFIG_DIR="+configDir)
	}
	return env
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

// brewPlaces are where Homebrew's brew is besides this app's PATH, as tmux is looked for (tmuxPlaces): an
// app opened from the Finder or the Dock has no Homebrew on its PATH. A variable, for the tests.
var brewPlaces = []string{"/opt/homebrew/bin", "/usr/local/bin"}

// tmuxCameWithSmolcorp is whether path, the tmux this computer found, is the one that came with Smolcorp
// (bundled is where Smolcorp's own lookup puts it, "" if it has none). Only the Mac app carries a tmux, so
// only a Mac counts: on Linux a tmux beside the program is just the distro's (/usr/bin/tmux) or one the person
// built in ~/.local/bin, and calling it "bundled" would be false. Piece 5 widens this to Linux's own library
// folder once it ships a tmux there. goos is a parameter for the tests.
func tmuxCameWithSmolcorp(path, bundled, goos string) bool {
	return path != "" && goos == "darwin" && path == bundled
}

// tmuxAvailable is the tmux this computer has, and whether it's the one that came with Smolcorp (the Mac app
// carries its own, so a Mac needs no install step). A variable, for the tests.
var tmuxAvailable = func() (path string, bundled bool) {
	path = tmuxPath()
	return path, tmuxCameWithSmolcorp(path, bundledTmux(), runtime.GOOS)
}

// tmuxInstall is the command that installs tmux with this computer's package manager, "" if none is known.
func tmuxInstall() string {
	for _, pm := range [][2]string{{"apt-get", "sudo apt install tmux"}, {"dnf", "sudo dnf install tmux"}, {"pacman", "sudo pacman -S tmux"},
		{"zypper", "sudo zypper install tmux"}, {"brew", "brew install tmux"}} {
		if hasProgram(pm[0]) {
			return pm[1]
		}
	}
	for _, folder := range brewPlaces {
		if executableIn(folder, "brew") != "" {
			return "brew install tmux"
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

// gmPreflight is the report for a hire on no account named, without the lock held.
func (d *Deck) gmPreflight() preflightReport { return d.gmPreflightOn("") }

// gmPreflightOn is the report for a hire on account ("" none named, which is the only one there is when there's
// one), without the lock held: Claude Code's sign-in and hooks are those of the account the hire would work on,
// as the Mac checks them (GMPreflight.check). An account that isn't there is the route's refusal, before this.
func (d *Deck) gmPreflightOn(account string) preflightReport {
	d.mu.Lock()
	folder := strings.TrimSpace(d.config.HireFolder)
	chosen, inTmux := d.config.HireTerminal, d.config.HireInTmux
	accounts := claudeAccountList(d.config.ClaudeProfiles)
	d.mu.Unlock()
	profile, named := claudeAccountNamed(account, accounts)
	other := named && profile.Name != claudeDefaultName
	configDir := ""
	if other {
		configDir = profile.Dir
	}
	testCopy := testCopyPort() != 0
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
		add(preflightItem{ID: "claude", Title: "Claude Code isn't installed on this " + d.machine() + ".",
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
			state = claudeSignIn(at, configDir)
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
		// Another account reports through its own settings.json, which the Mac connects too; a test copy's hire
		// through its own settings, in its command line, whatever the account's hold (the Mac's SelfTest.testCopyHires).
		connected := hooksNotConnected(Claude) == ""
		if other {
			connected = testCopy || claudeConnectedIn(profile)
		}
		if leavesAgentsAlone() {
			// A test copy never connects Claude Code (it can't write its settings): no Connect to offer, which would
			// only end in a 409. Ticked, as the Mac's (GMPreflight.items); a copy that's on another port without being
			// marked a test copy says it isn't connected, and why (RFXC1a N3).
			if testCopyMarked() {
				add(preflightItem{ID: "hooks", OK: true, Title: testCopyHires})
			} else {
				add(preflightItem{ID: "hooks", Title: notConnectedOnPort()})
			}
		} else if !connected {
			add(preflightItem{ID: "hooks", Title: "Claude Code doesn't report to Smolcorp yet, so your General Manager would never reach a desk.",
				Fix: "Press Connect, then Check again.", Action: &preflightAction{Kind: "fix", Label: "Connect", Fix: fixRepair}, Todo: true})
		} else {
			add(preflightItem{ID: "hooks", OK: true, Title: "Claude Code reports to Smolcorp."})
		}
	}
	// A test copy never hires Claude Code on your default account (testCopyRefusal): said before Hire, not after.
	// A Mac's test copy hires on an account of its own (GMPreflight.accountItems): To do until one is picked,
	// ticked with its name once one is. Elsewhere it can't yet, so it's a line with no fix, red (!): no step to take.
	if testCopy && runtime.GOOS == "darwin" {
		if other {
			add(preflightItem{ID: "account", OK: true, Title: "Your General Manager works on the Claude account " + profile.Name + "."})
		} else if len(accounts) > 1 {
			add(preflightItem{ID: "account", Title: gmNeedsAccount, Fix: gmChooseAccountFix, Action: &preflightAction{Kind: "account", Label: "Choose an account"}, Todo: true})
		} else {
			add(preflightItem{ID: "account", Title: gmNeedsAccount, Fix: gmAddAccountFix, Action: &preflightAction{Kind: "account", Label: "Add an account"}, Todo: true})
		}
	} else if testCopy {
		add(preflightItem{ID: "account", Title: gmNeedsAccount, Fix: "A test copy on this computer can't use one yet: hire your General Manager from the real Smolcorp."})
	}
	// Terminals only: tmux comes from the seam below, so it's looked up once.
	var terminals []hireOpener
	if canOpenWindows() {
		terminals = installedOpeners()
	}
	tmuxFound, bundled := tmuxAvailable()
	tmux := tmuxFound != ""
	terminal, hasTerminal := hireTerminal(chosen, terminals)
	// The install step is only for a computer with no tmux: one that came with Smolcorp leaves nothing to install.
	var tmuxAction *preflightAction
	if !tmux {
		if tmuxCommand := tmuxInstall(); tmuxCommand != "" {
			tmuxAction = &preflightAction{Kind: "copy", Label: "Copy", Command: tmuxCommand}
		}
	}
	// The tmux line's words when tmux is there: one that came with Smolcorp says so, under the Mac app or not.
	tmuxThere := "tmux is installed."
	if bundled {
		tmuxThere = "tmux comes with Smolcorp."
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
		add(preflightItem{ID: "terminal", Title: "There's no terminal on this " + d.machine() + " to open your General Manager in.", Fix: fix, Action: tmuxAction})
	}
	if hasTmuxHires && hasTerminal && underMacApp(terminal, hasTerminal) {
		// Under the Mac app every General Manager is hired fenced, in tmux, or not at all (macHireNeedsTmux): so
		// tmux is a step of its own, whether or not "Start inside tmux" is on, and Hire isn't Ready without it.
		if tmux {
			add(preflightItem{ID: "tmux", OK: true, Title: tmuxThere})
		} else {
			fix := "Install tmux (it needs Homebrew: brew.sh), then press Check again."
			if tmuxAction != nil {
				fix = "Paste this in Terminal to install tmux (it needs Homebrew), then press Check again."
			}
			add(preflightItem{ID: "tmux", Title: "Your General Manager needs tmux on your Mac to start fenced, and it isn't installed.", Fix: fix, Action: tmuxAction, Todo: true})
		}
	} else if hasTmuxHires && inTmux && hasTerminal {
		if tmux {
			add(preflightItem{ID: "tmux", OK: true, Title: tmuxThere})
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
