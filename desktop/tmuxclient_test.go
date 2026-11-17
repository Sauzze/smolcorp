//go:build !windows

package desktop

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// SA-M7: the core's tmux clients. Every one-shot call goes through tmuxClient (told the server it may talk to, a clean
// environment, stdin from /dev/null); every start through tmuxStartCall, or a sandboxed desk's runner. The rows here:
// the helper's refusals, a row that reads the core's code for any other tmux exec, a respawned pane and a pid used
// again (RT126SG2R2 N1, N2), an old desk's recorded tmux, and the team grid. The gate's own run of every site is
// tmuxgate_test.go's.

// Only a pid the core checked (more than 1, as it prints) or "unchecked" is a guard a one-shot call is run with.
func TestTmuxClientRunsOnlyWithAGuardOfTheCores(t *testing.T) {
	program, log := tmuxGateStandIn(t)
	for _, guard := range []string{"", "off", "0", "1", "-5", "abc", "0123", "5151 ", "+5151", "Unchecked", "5151\nTMUX=x"} {
		if cmd, _, err := tmuxClient(tmuxCallWait, program, guard, "-S", "/x", "has-session"); err != errNoTmuxGuard || cmd != nil {
			t.Errorf("guard %q: %v", guard, err)
		}
	}
	if _, _, err := tmuxClient(tmuxCallWait, "", "5151"); err != errNoTmuxGuard {
		t.Errorf("no program: %v", err)
	}
	if runs := tmuxGateRuns(t, log, 0); len(runs) != 0 {
		t.Fatalf("a refused call ran: %+v", runs)
	}
	// What the core has of its own never reaches the client: its guard variable, TMUX, a token, its HOME.
	t.Setenv(tmuxGuardVariable, "4242")
	t.Setenv("TMUX", "/tmp/tmux-501/default,1,0")
	t.Setenv("SMOLCORP_GM_TOKEN", "fake-gm-token-1")
	for _, guard := range []string{"5151", tmuxUnchecked} {
		cmd, cancel, err := tmuxClient(tmuxCallWait, program, guard, "-S", "/x", "has-session")
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cancel()
	}
	runs := tmuxGateRuns(t, log, 0)
	if len(runs) != 2 || runs[0].guard != "5151" || runs[1].guard != tmuxUnchecked {
		t.Fatalf("runs %+v", runs)
	}
	for _, run := range runs {
		if !slices.Equal(run.env, []string{"LANG", "PATH", tmuxGuardVariable, "TERM"}) || run.stdin != "null" || run.args != "-f /dev/null -S /x has-session" {
			t.Errorf("run %+v", run)
		}
	}
}

// FXTMUXDIR: an unchecked client is told the server pids the rows record for the desks in their sandbox, which the
// guard then never talks to: a one-shot call's and an attach's alike. A desk outside the sandbox (its socket in
// tmux's own folder, its tabs driven unchecked) isn't one; a gone desk's server still is; a checked guard is as it is.
func TestAnUncheckedClientIsToldTheDesksServers(t *testing.T) {
	shortSupport(t)
	program, log := tmuxGateStandIn(t)
	deskSockets(t)
	if got := clientGuard(tmuxUnchecked); got != tmuxUnchecked {
		t.Fatalf("with no desk recorded: %q", got)
	}
	writeRegistryHires(t,
		Hire{ID: "cd34", State: "working", TmuxSocket: deskSocketPath("cd34", 0), TmuxServer: 4242, Sandboxed: true},
		Hire{ID: "ef56", State: "gone", TmuxSocket: deskSocketPath("ef56", 0), TmuxServer: 6464, Sandboxed: true},
		Hire{ID: "ab12", State: "working", TmuxSocket: filepath.Join(tmuxSocketFolder(), "smolcorp-ab12"), TmuxServer: 5353},
		Hire{ID: "gh78", State: "working", TmuxSocket: deskSocketPath("gh78", 0), TmuxServer: 1},
		Hire{ID: "ij90", State: "working", TmuxSocket: filepath.Join(deskSocketParent(), "ij90", "t"), TmuxServer: 7575})
	if got := clientGuard(tmuxUnchecked); got != "unchecked:4242,6464" {
		t.Errorf("the unchecked guard is %q, want unchecked:4242,6464", got)
	}
	if got := clientGuard("5151"); got != "5151" {
		t.Errorf("a checked guard is %q", got)
	}
	cmd, cancel, err := tmuxClient(tmuxCallWait, program, tmuxUnchecked, "-S", "/x", "has-session")
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cancel()
	if runs := tmuxGateRuns(t, log, 0); len(runs) != 1 || runs[0].guard != "unchecked:4242,6464" {
		t.Errorf("the one-shot call ran %+v", runs)
	}
	a, err := attachClient(program, tmuxUnchecked, "-S", "/x", "attach-session", "-t", "=s")
	if err != nil {
		t.Fatal(err)
	}
	a.done()
	if !slices.Contains(a.cmd.Env, tmuxGuardVariable+"=unchecked:4242,6464") {
		t.Errorf("the attach's environment is %q", a.cmd.Env)
	}
}

// tmuxPickers say which tmux to run. tmuxPickedIn is every function that may use one, with what it does with it: run
// it only through tmuxClient or tmuxStartCall, record it, or say whether there is one. No window's or pane's line names
// a tmux: each runs `smolcorp attach` (SA-M8), which runs it through tmuxClient, or in the desk's sandbox.
var tmuxPickers = []string{"findTmux", "tmuxPath", "tmuxProgram", "tmuxProgramFor", "tmuxDriving", "bundledTmux",
	"recordedTmux", "recordedTmuxChecked", "recordedDeskTmux", "tmuxFor"}

var tmuxPickedIn = map[string]string{
	"hire.go tmuxPath":                  "a picker",
	"hire.go findTmux":                  "a picker",
	"hire.go recordedTmux":              "a picker",
	"hire.go tmuxDriving":               "a picker",
	"hire.go tmuxProgram":               "a picker",
	"hire.go tmuxProgramFor":            "a picker",
	"hiretmux.go tmuxFor":               "a picker, and the guard for it",
	"hireengine_windows.go tmuxFor":     "a picker: no tmux runs on Windows",
	"hiretmux.go newHireTmux":           "runs it through oneShotTmux and tmuxStartCall (hireTmuxUsing)",
	"hiretmux.go newDeskTmux":           "runs it through oneShotTmux; its start through the desk's sandbox",
	"hiretmux.go hireTmuxAt":            "runs it through oneShotTmux and tmuxStartCall (hireTmuxUsing)",
	"hiretmux.go hireTmuxUsing":         "runs it through oneShotTmux and tmuxStartCall",
	"tabs.go outputWith":                "runs it through tmuxClient",
	"tabs.go alive":                     "runs it through tmuxClient",
	"gmtile_unix.go showTeamGrid":       "runs it through tmuxStartCall and oneShotTmux; its panes and window run smolcorp attach",
	"attach.go attachCommand":           "runs it through tmuxClient (attachClient): the team grid's, a plain hire's",
	"gmpreflight.go tmuxAvailable":      "says which tmux this computer has",
	"hire.go hireWays":                  "says whether there is a tmux",
	"server.go Serve":                   "finds it early, so a login shell is asked in the background; runs nothing",
	"t3takeover_tmux.go t3StopInTmux":   "says whether there is a tmux",
	"t3takeover_tmux.go t3ResumeInTmux": "records it",
	"t3takeover.go handBackFromT3":      "records it",
	"hostbridge.go deskOfPane":          "runs nothing: the guard alone, for the Mac app's TmuxClient (SA-M9)",
}

// The grep row (release gate item 1) reads the core's code for every place it starts a program (RT126SAM7 B1: not a
// name match, which a variable or a field got past):
//   - exec.Command, exec.CommandContext, os.StartProcess, syscall.Exec, ForkExec and StartProcess, by whatever name
//     their package is imported as, called or only named;
//   - an exec.Cmd written out with a Path, and a Path set on anything;
//   - a pseudo-terminal's Command and CommandContext;
//   - every call of a function that starts the program it's handed (terminalCommand, tmuxStartCall, quickRun…: its
//     program is one of its parameters, an element or field of one, or a local copied from one; a package variable's
//     function literal counts as a function of that name), found by the same reading, in the core and in the packages
//     it imports, until there are no more;
//   - every call through what holds such a function taken as a value (RT126SAM7R2 D2): a field it's set to (t3Machine's
//     Run, told from another type's field of that name by the receiver's type, as far as the reading can tell it), a
//     local, the parameter of a function it's handed to, a package variable;
//   - every place a desk's start-only runner (tmuxStarter) is named, made, kept or called (RT126SAM7R2 B1′).
// Every package under desktop and internal is read, one inside another too (D3): the core's (desktop and the packages
// inside it) are checked against the list; another's, where it can run a tmux.
// Each must be on tmuxExecSites, a closed list by file, function, way and program, with what it runs: a new site, or one
// more of a listed one, fails until someone lists it and says what it runs, and a listed one that's gone fails too. And
// whatever the list says, a site whose program can be tmux (its own words, the locals it was set from, the type of a
// parameter or receiver it comes from, or its function's name say tmux) is one only tmuxClient, tmuxStartCall and a call
// of tmuxStartCall (a start outside a sandbox) may be. tmuxClient is the one way a one-shot call runs: its own calls
// aren't sites, since it refuses any guard but a checked pid or "unchecked". A wrapper taken as a value (a test's
// stand-in can replace it) is a site of its own, listed with what it's called with, and the calls through what holds it
// are sites too; where the reading can't follow it, it's a site that must be listed. A window's attach line (SA-M8's) is
// the pickers' list below.

// tmuxExecSites is every start in the core's code (every system's files), by "file function way(program)", with what it
// runs. A second like one in the same function is listed again with " #2".
var tmuxExecSites = map[string]string{
	// The two ways a tmux runs, and the starts.
	"gmtile_unix.go showTeamGrid tmuxStartCall(tmux)":       "tmux, the team grid's server",
	"hiretmux.go hireTmuxUsing tmuxStartCall(program)":      "tmux, a desk's start on a computer with no sandbox yet",
	"tmuxclient.go tmuxClient exec.CommandContext(program)": "tmux, a one-shot client: guarded, a clean environment, stdin /dev/null",
	"tmuxclient.go tmuxStartCall exec.CommandContext(path)": "tmux, a start outside any sandbox, its guard off for that call alone",
	// A package the core imports, where it can run a tmux (RT126SAM7R2 D3).
	"internal/sandbox/enter_unix.go enter syscall.Exec(e.program)": "tmux, a sandboxed desk's start or attach: smolcorp sandbox enter becomes the bundled tmux in its sandbox",
	// A desk's start-only runner (RT126SAM7R2 B1′): every place one is named, made, kept or called. It's called by
	// hireTmux.start alone; it's kept only as onlyStarts made it, which refuses any call but that start.
	"hiretmux.go hireTmux names tmuxStarter(…)":                                                                               "the desk's start-only runner, its field",
	"hiretmux.go startingTmux names tmuxStarter(…)":                                                                           "makes a start-only runner of a tmux runner (a start outside the sandbox)",
	"hiretmux.go startingTmux names func([]string) (string, int, error)(…)":                                                   "makes a start-only runner of a tmux runner (a start outside the sandbox)",
	"hiretmux.go onlyStarts names tmuxStarter(…)":                                                                             "makes a start-only runner refuse any call but its desk's start",
	"hiretmux.go onlyStarts names tmuxStarter(…) #2":                                                                          "makes a start-only runner refuse any call but its desk's start",
	"hiretmux.go onlyStarts names func([]string) (string, int, error)(…)":                                                     "makes a start-only runner refuse any call but its desk's start",
	"hiretmux.go hireTmuxUsing startingTmux(withoutStartFile(tmuxStartCall(program)))":                                        "a desk's start outside the sandbox, its guard off for that call alone",
	"hiretmux.go hireTmuxUsing onlyStarts(h.socket, startingTmux(withoutStartFile(tmuxStartCall(program))))":                  "a desk's start outside the sandbox, its guard off for that call alone",
	"hiretmux.go hireTmuxUsing sets .startOnly(onlyStarts(h.socket, startingTmux(withoutStartFile(tmuxStartCall(program)))))": "a desk's start outside the sandbox, kept as onlyStarts made it",
	"hiretmux.go start names .startOnly(…)":                                                                                   "whether the desk has a start-only runner",
	"hiretmux.go start .startOnly(h.command(args...))":                                                                        "tmux, the desk's start: the one call through its start-only runner",
	"hireengine.go engineOptions names tmuxStarter(…)":                                                                        "the tests' stand-in for a sandboxed desk's start-only runner",
	"hireengine.go sandboxing names .Sandbox(…)":                                                                              "whether the tests gave a stand-in start-only runner",
	"hireengine.go sandboxDesk names .Sandbox(…)":                                                                             "whether the tests gave a stand-in start-only runner",
	"hireengine.go sandboxDesk .Sandbox(desk)":                                                                                "the tests' stand-in start-only runner, for a sandboxed desk",
	"hireengine.go sandboxDesk onlyStarts(tmux.socket, o.Sandbox(desk))":                                                      "the tests' stand-in start-only runner, kept as onlyStarts made it",
	"hireengine.go sandboxDesk sets .startOnly(onlyStarts(tmux.socket, o.Sandbox(desk)))":                                     "the tests' stand-in start-only runner, kept as onlyStarts made it",
	"hireengine.go sandboxDesk .starter(tmux.program, o.CLI)":                                                                 "a sandboxed desk's start-only runner (desksandbox.go)",
	"hireengine.go sandboxDesk onlyStarts(tmux.socket, desk.starter(tmux.program, o.CLI))":                                    "a sandboxed desk's start-only runner, kept as onlyStarts made it",
	"hireengine.go sandboxDesk sets .startOnly(onlyStarts(tmux.socket, desk.starter(tmux.program, o.CLI)))":                   "a sandboxed desk's start-only runner, kept as onlyStarts made it",
	"desksandbox_darwin.go starter names tmuxStarter(…)":                                                                      "a sandboxed desk's start-only runner on a Mac: the bundled tmux in sandbox-exec",
	"desksandbox_darwin.go starter names func([]string) (string, int, error)(…)":                                              "a sandboxed desk's start-only runner on a Mac: the bundled tmux in sandbox-exec",
	"desksandbox_other.go starter names tmuxStarter(…)":                                                                       "a sandboxed desk's start-only runner elsewhere: refuses, starts nothing",
	"desksandbox_other.go starter names func([]string) (string, int, error)(…)":                                               "a sandboxed desk's start-only runner elsewhere: refuses, starts nothing",
	// A wrapper kept as a value, called through what holds it (RT126SAM7R2 D2): smolcorp's guard tool, T3 Code.
	"hiresettings.go guardGroups run(tool)":                                       "smolcorp's own guard tool (runGuardTool, or a test's stand-in)",
	"hiresettings.go hireSettingsObject guardGroups(o.CLI)":                       "smolcorp's own guard tool",
	"hiresettings.go hireSettingsText hireSettingsObject(o)":                      "smolcorp's own guard tool",
	"settingsfile.go writeHireSettings hireSettingsText(o)":                       "smolcorp's own guard tool",
	"hireengine.go runManagedHire writeHireSettings(hireSettingsOptions{…})":      "smolcorp's own guard tool",
	"hirelauncher.go runLauncherHire writeHireSettings(hireSettingsOptions{…})":   "smolcorp's own guard tool",
	"t3deskdeck.go t3FenceSettings hireSettingsObject(hireSettingsOptions{…})":    "smolcorp's own guard tool",
	"t3takeover_tmux.go t3ResumeInTmux writeHireSettings(hireSettingsOptions{…})": "smolcorp's own guard tool",
	"sandboxrestart.go restartDesk writeHireSettings(hireSettingsOptions{…})":     "smolcorp's own guard tool (SA-X14's restart in the sandbox)",
	"t3code.go t3Detect .Run(program)":                                            "T3 Code's command line",
	"t3code.go t3Open .Run(program)":                                              "T3 Code's command line",
	"t3code.go t3Open .Run(program) #2":                                           "T3 Code's command line",
	"t3code.go t3Open t3Threads(program)":                                         "T3 Code's command line",
	"t3code.go t3Open t3Threads(program) #2":                                      "T3 Code's command line",
	"t3code.go t3Threads .Run(program)":                                           "T3 Code's command line",
	"t3desk.go round t3Show(program)":                                             "T3 Code's command line",
	"t3desk.go t3HireInto .Run(program)":                                          "T3 Code's command line",
	"t3desk.go t3HireInto .Run(program) #2":                                       "T3 Code's command line",
	"t3desk.go t3HireInto .Run(program) #3":                                       "T3 Code's command line",
	"t3desk.go t3HireInto t3Show(program)":                                        "T3 Code's command line",
	"t3desk.go t3Leave .Run(program)":                                             "T3 Code's command line",
	"t3desk.go t3Mail .Feed(program)":                                             "T3 Code's command line, fed on its standard input",
	"t3desk.go t3Show .Run(program)":                                              "T3 Code's command line",
	"t3internal.go t3Call .Run(program)":                                          "T3 Code's command line",
	"t3internal.go t3Call t3Revoke(program)":                                      "T3 Code's command line",
	"t3internal.go t3Call t3RevokeOurs(program)":                                  "T3 Code's command line",
	"t3internal.go t3Call t3RevokeOurs(program) #2":                               "T3 Code's command line",
	"t3internal.go t3Revoke .Run(program)":                                        "T3 Code's command line",
	"t3internal.go t3RevokeOurs .Run(program)":                                    "T3 Code's command line",
	"t3internal.go t3RevokeOurs t3Revoke(program)":                                "T3 Code's command line",
	"t3takeover.go t3HandBack .Run(program)":                                      "T3 Code's command line",
	"t3takeover.go t3HandingFor t3Call(program)":                                  "T3 Code's command line",
	"t3takeover.go t3TakeOverSaid .Run(program)":                                  "T3 Code's command line",
	"t3takeover.go t3TakeOverSaid t3Show(program)":                                "T3 Code's command line",
	// Everything else the core starts: no tmux.
	"agentcheck_darwin.go codesign exec.CommandContext(codesignProgram)":   "codesign (/usr/bin), reading or checking a program's signature",
	"claudeversion.go installedClaudeVersion exec.CommandContext(program)": "the claude a desk would start, checked first, for its --version",
	"deck.go runSummaryAgent runSummaryCLI(program)":                       "a summary provider's command line (claude, codex, gemini…), found and checked as the summaries find it",
	"desksandbox.go deskPlaces gitConfigFiles(git)":                        "git, asked where its config files are (gitConfigFiles)",
	"desksandbox.go gitConfigFiles exec.CommandContext(git)":               "git, for its config files' list",
	"desksignin.go deskSignInWith quickRun(command[0])":                    "claude, asked whether a desk's Claude folder (or one made like it) is signed in (SA-X9)",
	"deskstart.go RunDeskStart syscall.Exec(launch.Program)":               "a desk's helper becoming its agent in place, inside the desk's tmux (an exec, no new process)",
	"firewall_windows.go readFirewallRules exec.Command(\"powershell\")":   "powershell, reading Windows' firewall rules",
	"firewall_windows.go fixFirewallFor exec.Command(\"powershell\")":      "powershell, adding Smolcorp's firewall rule",
	"deskrun.go deskGit deskRun(git)":                                      "git (exec.LookPath, a whole path) with gitstate's read-only settings, on a hire's checkout: in the desk's sandbox when it has one",
	"deskrun.go deskRun exec.Command(program)":                             "what deskGit (git) and runGH (gh) hand it, where there's no desk or the desk has no sandbox",
	"ownergit.go gitRun deskRun(git)":                                      "git (exec.LookPath, a whole path) as the owner, outside any sandbox, on config the core wrote: runGit's and scopeGit's other folders, a desk's tree with no sandbox, the demo's project",
	"ghpr.go runGH deskRun(program)":                                       "gh, for a branch's pull request: in the hire's sandbox when it has one",
	"gitstate.go execGit exec.CommandContext(\"git\")":                     "git, with the read-only settings gitstate.go pins",
	"hire.go showWindow exec.Command(args[0])":                             "a terminal app (gnome-terminal, konsole…), opening a hire's window",
	"hire_unix.go findInLoginShell lookupCommand(args[0])":                 "the owner's login shell, asked where a program is",
	"t3treecheck.go t3TreeGit deskRun(git)":                                "git on a hire's worktree, in its sandbox, reading config for Take over's check (SA-X7)",
	"hiresettings.go runGuardTool exec.CommandContext(tool)":               "smolcorp's own guard tool, from inside this copy",
	"hiresettings.go hireSettingsObject names runGuardTool(…)":             "runGuardTool, kept as a test's stand-in can replace it: called with the guard tool only",
	"hook_mac.go psProcess exec.Command(\"/bin/ps\")":                      "/bin/ps, reading a process's parent and terminal",
	"hostphonemic.go askTailnet runTailscale(tool)":                        "tailscale, asked about the tailnet",
	"hostphonemic.go askTailnet runTailscale(tool) #2":                     "tailscale, asked about the tailnet",
	"hostphonemic.go runTailscale exec.CommandContext(tool)":               "tailscale's command line",
	"install_windows.go installAndRelaunch startProgram(installed)":        "Smolcorp itself, installed, started again",
	"install_windows.go installNow relaunch(installed)":                    "Smolcorp itself, installed, started again",
	"launcher.go launch pty.Command(program)":                              "smolcorp run's agent in a pseudo-terminal, given disclaim.Words's program",
	"macclock.go macTimePrefs exec.Command(\"/usr/bin/defaults\")":         "/usr/bin/defaults, reading the Mac's clock settings",
	"music_linux.go musicKey exec.Command(\"playerctl\")":                  "playerctl, a media key",
	"music_linux.go musicPlaying exec.Command(\"playerctl\")":              "playerctl, whether music plays",
	"music_linux.go getVolume exec.Command(\"wpctl\")":                     "wpctl or pactl, reading the volume",
	"music_linux.go getVolume exec.Command(\"pactl\")":                     "wpctl or pactl, reading the volume",
	"music_linux.go getVolume exec.Command(\"pactl\") #2":                  "wpctl or pactl, reading the volume",
	"music_linux.go setVolume exec.Command(\"wpctl\")":                     "wpctl or pactl, setting the volume",
	"music_linux.go setVolume exec.Command(\"wpctl\") #2":                  "wpctl or pactl, setting the volume",
	"music_linux.go setVolume exec.Command(\"pactl\")":                     "wpctl or pactl, setting the volume",
	"music_linux.go setVolume exec.Command(\"pactl\") #2":                  "wpctl or pactl, setting the volume",
	"network.go openURL exec.Command(\"rundll32\")":                        "the system's opener (rundll32, /usr/bin/open, xdg-open) for a page",
	"network.go openURL exec.Command(\"/usr/bin/open\")":                   "the system's opener (rundll32, /usr/bin/open, xdg-open) for a page",
	"network.go openURL exec.Command(\"xdg-open\")":                        "the system's opener (rundll32, /usr/bin/open, xdg-open) for a page",
	"opencode.go loginShellPath lookupCommand(shell)":                      "the owner's login shell, for its PATH",
	"opencode.go lookupCommand exec.CommandContext(shell)":                 "a login shell, with nothing of Smolcorp's in its environment",
	"opencode.go openCodeMajor versionCommand(program)":                    "opencode, for its --version",
	"agentownsandbox.go agentVersionOf versionCommand(program)":            "a plain hire's agent, for its --version (SA-M10)",
	"agentownsandbox.go ownSandboxOffFor agentVersionOf(program)":          "a plain hire's agent, for its --version (SA-M10)",
	"launcher.go launch ownSandboxOffFor(program)":                         "a plain hire's agent, for its --version (SA-M10)",
	"opencode_other.go versionCommand exec.CommandContext(program)":        "a program's --version",
	"opencode_other.go scriptCommand exec.CommandContext(program)":         "a program its caller found (opencode, T3 Code)",
	"opencode_windows.go versionCommand scriptCommand(program)":            "a program's --version, through scriptCommand",
	"opencode_windows.go scriptCommand exec.CommandContext(program)":       "a program its caller found, or cmd for a .cmd script",
	"opencode_windows.go scriptCommand exec.CommandContext(shell)":         "a program its caller found, or cmd for a .cmd script",
	"phonehttps.go runTailscaleServe exec.CommandContext(tool)":            "tailscale serve, on or off only",
	"phonehttps.go tailnetServe runTailscaleServe(tool)":                   "tailscale serve, on or off only",
	"picker_unix.go folderPicker exec.Command(path)":                       "zenity or kdialog, the desktop's folder picker",
	"picker_unix.go folderPicker exec.Command(path) #2":                    "zenity or kdialog, the desktop's folder picker",
	"picker_windows.go folderPicker exec.Command(\"powershell.exe\")":      "powershell.exe, Windows' folder picker",
	"presence_linux.go presence terminalCommand(\"loginctl\")":             "loginctl, whether the session is locked or idle",
	"preview_darwin.go runLsof exec.CommandContext(lsofProgram)":           "lsof, a process's open files",
	"restart.go startProgram startProgramWith(program)":                    "Smolcorp itself, or /usr/bin/open -b for its app",
	"restart.go startProgramWith exec.Command(program)":                    "Smolcorp itself, or /usr/bin/open -b for its app (exempt as a whole: spawnsites_test.go)",
	"restart.go relaunch replaceProcess(program)":                          "Smolcorp itself, again, for a restart",
	"restart.go relaunch startProgramWith(program)":                        "Smolcorp itself, again, for a restart",
	"restart.go relaunch startProgramWith(program) #2":                     "Smolcorp itself, again, for a restart",
	"restart.go restartSoon relaunch(program)":                             "Smolcorp itself, again, for a restart",
	"restart_unix.go replaceProcess syscall.Exec(program)":                 "Smolcorp becoming itself again, in place",
	"setupcli.go startApp startProgram(program)":                           "Smolcorp's own app, from smolcorp setup",
	"setupcli.go openMacApp startProgramWith(\"/usr/bin/open\")":           "/usr/bin/open -b for Smolcorp's own app",
	"signinrun.go start pty.Command(started)":                              "an agent's sign-in command in a pseudo-terminal, given disclaim.Words's program",
	"signinstatus.go probeSignIn probeAgentSignIn(signInFind(agent))":      "an agent's command line, asked whether it's signed in",
	"signinstatus.go probeAgentSignIn spawnCapped(program)":                "an agent's command line, asked whether it's signed in",
	"signinstatus.go probeAgentSignIn spawnCapped(program) #2":             "an agent's command line, asked whether it's signed in",
	"summary_process_windows.go kill exec.Command(\"taskkill\")":           "taskkill, ending a summary's process tree",
	"summary_providers.go runSummaryCLI spawnSummary(program)":             "a summary provider's command line",
	"summary_providers.go runSummaryCLI spawnSummary(program) #2":          "a summary provider's command line",
	"summary_providers.go runSummaryCLI spawnSummary(program) #3":          "a summary provider's command line",
	"summary_providers.go runSummaryCLI spawnSummary(program) #4":          "a summary provider's command line",
	"summary_providers.go spawnSummary spawnCapped(program)":               "a summary provider's command line",
	"summary_providers.go spawnCapped summaryCommand(program)":             "a summary provider's command line, its output capped",
	"summary_providers.go summaryCommand exec.CommandContext(shell)":       "a summary provider's command line, or a shell for its script",
	"summary_providers.go summaryCommand exec.CommandContext(program)":     "a summary provider's command line, or a shell for its script",
	"summary_providers.go checkProvider .helpText(program)":                "a summary provider's command line, for its --help and --version",
	"summary_providers.go checkProvider quickRun(program)":                 "a summary provider's command line, for its --help and --version",
	"summary_providers.go checkProvider quickRun(program) #2":              "a summary provider's command line, for its --help and --version",
	"summary_providers.go checkProvider quickRun(program) #3":              "a summary provider's command line, for its --help and --version",
	"summary_providers.go checkProvider quickRun(program) #4":              "a summary provider's command line, for its --help and --version",
	"summary_providers.go checkProvider quickRun(program) #5":              "a summary provider's command line, for its --help and --version",
	"summary_providers.go helpText quickRun(program)":                      "a summary provider's command line, for its --help",
	"summary_providers.go quickRun spawnSummary(program)":                  "a summary provider's command line, briefly",
	"summary_providers.go summaryProgram quickRun(program)":                "cursor's agent, for its --version",
	"t3code.go defaultT3Machine names runT3(…)":                            "T3 Code (runT3, feedT3 kept for a test's stand-in; its launcher off a Mac)",
	"t3code.go defaultT3Machine names feedT3(…)":                           "T3 Code (runT3, feedT3 kept for a test's stand-in; its launcher off a Mac)",
	"t3code.go defaultT3Machine exec.Command(launcher)":                    "T3 Code (runT3, feedT3 kept for a test's stand-in; its launcher off a Mac)",
	"t3code.go runT3 feedT3(program)":                                      "T3 Code's command line",
	"t3code.go feedT3 scriptCommand(program)":                              "T3 Code's command line, fed on its standard input",
	"tabs.go terminalCommand exec.CommandContext(name)":                    "a terminal's command line tool, as its callers name it",
	"tabs.go outputWith terminalCommand(\"wezterm\")":                      "wezterm or kitty, a tab's own command line",
	"tabs.go outputWith terminalCommand(\"kitty\")":                        "wezterm or kitty, a tab's own command line",
	"tabs.go front terminalCommand(\"ps\")":                                "ps, a window's terminal",
	"tabs.go frontOn terminalCommand(\"ps\")":                              "ps, what runs on a terminal",
}

// tmuxExecAllowed are the sites whose program may be tmux: the helper's own and the start's own. A call of tmuxStartCall
// may be too (by its way), and must be listed above.
var tmuxExecAllowed = map[string]string{
	"tmuxclient.go tmuxClient exec.CommandContext(program)":        "the one helper",
	"tmuxclient.go tmuxStartCall exec.CommandContext(path)":        "the one start outside a sandbox",
	"hiretmux.go start .startOnly(h.command(args...))":             "a desk's start, through its start-only runner (onlyStarts refuses any other call)",
	"internal/sandbox/enter_unix.go enter syscall.Exec(e.program)": "a sandboxed desk's start (its guard off for that start alone) or attach (told the checked server, SA-M8): smolcorp sandbox enter becomes the bundled tmux",
	// Not a tmux: the reading says so by its function's name (it resumes a desk in its tmux); it runs the guard tool.
	"t3takeover_tmux.go t3ResumeInTmux writeHireSettings(hireSettingsOptions{…})": "not a tmux: smolcorp's own guard tool",
	// Not a tmux: its words name tmux only by reading tmux.sandboxed, for the desk's kept folder (SA-L2); it runs the guard tool.
	"hireengine.go runManagedHire writeHireSettings(hireSettingsOptions{…})": "not a tmux: smolcorp's own guard tool",
	// Not a tmux: the same read of tmux.sandboxed, for the restarted desk's kept folder (RT126SAL2 S1); it runs the guard tool.
	"sandboxrestart.go restartDesk writeHireSettings(hireSettingsOptions{…})": "not a tmux: smolcorp's own guard tool",
}

// execStarters are the standard library's ways to start a program, by package path, each with the argument that is the
// program.
var execStarters = map[string]map[string]int{
	"os/exec": {"Command": 0, "CommandContext": 1},
	"os":      {"StartProcess": 0},
	"syscall": {"Exec": 0, "ForkExec": 0, "StartProcess": 0},
}

// execLaunchers are programs that run what their words say: a start of one whose words come from its function's
// parameters starts what that function is handed.
var execLaunchers = []string{"sh", "bash", "zsh", "dash", "env", "xargs", "nohup", "script", "sudo", "osascript", "cmd", "cmd.exe", "powershell", "pwsh"}

// tmuxOneShotHelper is the core's one helper whose calls aren't sites.
const tmuxOneShotHelper = "tmuxClient"

// A desk's start-only runner (RT126SAM7R2 B1′) is a value of tmuxStarterType, or of its function spelled without
// names (tmuxStarterShape), which can hold one as well. The reading knows every name one goes by from the declarations
// (a field or method, a function or package variable whose type is one or that makes one), and every place one is
// named, made, kept, handed on or called is a site of its own on the closed list: so a call through a field, a local or
// a parameter that holds one can't be spelled anywhere new. A call through one is a tmux start, which only
// hireTmux.start may make; a field one is kept in is set only to what tmuxStartGuard made (it refuses anything but a
// start); and what tmuxStartCall gives is called at once, or handed to startingTmux, never kept as a one-shot runner.
const (
	tmuxStarterType  = "tmuxStarter"
	tmuxStarterShape = "func([]string) (string, int, error)"
	tmuxStartGuard   = "onlyStarts"
)

// execSite is one start found.
type execSite struct {
	file, function string
	owner          string // its function as funcKey names it (a method's "Type.name")
	way, program   string // how it starts (exec.Command, terminalCommand, …), and its program's words, spaces closed up
	line           int
	from           []int  // the parameters of its function its program comes from: that function starts what it's handed
	tmux           bool   // its program can be tmux
	note           string // why its program can't be followed further, said when it isn't listed
	never          string // why it fails whatever the list says
}

func (s execSite) key() string {
	return s.file + " " + s.function + " " + s.way + "(" + s.program + ")"
}

// execWrapper is a function that starts the program it's handed, at the parameters from.
type execWrapper struct {
	from     []int
	params   int
	variadic bool
	method   bool
}

// fits says whether a call with args arguments can be a call of w: a holder's name (".Run") is matched by its calls
// alone, so a call with another count is something else of that name (cmd.Run()).
func (w execWrapper) fits(args []ast.Expr) bool {
	n := len(args)
	if n == 1 && w.params > 1 {
		if _, ok := ast.Unparen(args[0]).(*ast.CallExpr); ok {
			return true // f(g()), g's several results
		}
	}
	return n == w.params || w.variadic && n >= w.params-1
}

// execPackage is one package's files, read.
type execPackage struct {
	path     string
	fset     *token.FileSet
	names    []string
	files    map[string]*ast.File
	srcs     map[string][]byte
	funcs    map[string][]execFunc // its functions by name and methods by "Type.name", for a parameter a wrapper is handed to
	starters map[string]string     // the names a start-only runner goes by: ".field" or ".method", or a function's or variable's name -> "holds" or "makes"
	// What the reading knows of its types, to tell one field or method from another of the same name (".run"): its
	// types, each struct's fields' types, what a function ("name") or method ("Type.name") of one result gives, and its
	// package variables' types; each by its type's name, "" when it isn't one of the package's own.
	types   map[string]bool
	fields  map[string]map[string]string
	results map[string]string
	vars    map[string]string
}

// baseType is the name of the package's type t is, through a pointer or type arguments, or "".
func baseType(t ast.Expr) string {
	switch x := t.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return baseType(x.X)
	case *ast.IndexExpr:
		return baseType(x.X)
	case *ast.IndexListExpr:
		return baseType(x.X)
	}
	return ""
}

// funcKey is how a function is named in the reading's maps: its name, or a method's "Type.name".
func funcKey(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	return baseType(decl.Recv.List[0].Type) + "." + decl.Name.Name
}

// byOwner is what m has for name owned by owner ("owner.name", and ".name", owned by a type the reading didn't know),
// or, with owner "" (a receiver whose type the reading can't tell), for name owned by any type: in order of key.
func byOwner[V any](m map[string]V, owner, name string) []V {
	var found []V
	for _, key := range slices.Sorted(maps.Keys(m)) {
		at := strings.LastIndex(key, ".")
		if at < 0 || key[at+1:] != name {
			continue
		}
		if owner == "" || key[:at] == "" || key[:at] == owner {
			found = append(found, m[key])
		}
	}
	return found
}

// execFunc is one function of a package, in its file.
type execFunc struct {
	file string
	decl *ast.FuncDecl
}

// textIn gives a node's source in file name, whitespace closed up.
func (p *execPackage) textIn(name string) func(ast.Node) string {
	src := p.srcs[name]
	return func(n ast.Node) string {
		return strings.Join(strings.Fields(string(src[p.fset.Position(n.Pos()).Offset:p.fset.Position(n.End()).Offset])), " ")
	}
}

// readExecPackage reads every non-test .go file in dir (every system's), as the package at path, each file named as
// prefix and its name.
func readExecPackage(t testing.TB, dir, path, prefix string) *execPackage {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		files[prefix+filepath.Base(name)] = src
	}
	p, err := parseExecPackage(path, files)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// parseExecPackage parses files (name -> source) as the package at path.
func parseExecPackage(path string, files map[string][]byte) (*execPackage, error) {
	p := &execPackage{path: path, fset: token.NewFileSet(), files: map[string]*ast.File{}, srcs: files, funcs: map[string][]execFunc{},
		types: map[string]bool{}, fields: map[string]map[string]string{}, results: map[string]string{}, vars: map[string]string{}}
	for name, src := range files {
		file, err := parser.ParseFile(p.fset, name, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		p.names = append(p.names, name)
		p.files[name] = file
	}
	sort.Strings(p.names)
	for _, name := range p.names {
		for _, decl := range execFunctions(p.files[name]) {
			p.funcs[funcKey(decl)] = append(p.funcs[funcKey(decl)], execFunc{file: name, decl: decl})
			if results := decl.Type.Results; results != nil && len(results.List) == 1 && len(results.List[0].Names) <= 1 {
				p.results[funcKey(decl)] = baseType(results.List[0].Type)
			}
		}
		for _, decl := range p.files[name].Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					p.types[s.Name.Name] = true
					if st, ok := s.Type.(*ast.StructType); ok {
						p.fields[s.Name.Name] = map[string]string{}
						for _, field := range st.Fields.List {
							for _, id := range field.Names {
								p.fields[s.Name.Name][id.Name] = baseType(field.Type)
							}
						}
					}
				case *ast.ValueSpec:
					for i, id := range s.Names {
						switch {
						case s.Type != nil:
							p.vars[id.Name] = baseType(s.Type)
						case i < len(s.Values):
							if lit, ok := ast.Unparen(s.Values[i]).(*ast.CompositeLit); ok {
								p.vars[id.Name] = baseType(lit.Type)
							}
						}
					}
				}
			}
		}
	}
	p.starters = startOnlyNames(p)
	return p, nil
}

// startOnlyNames are the names a start-only runner goes by in p, from its declarations: a struct's field or a method
// (".name"), a function or package variable (name), whose type is one ("holds") or makes one ("makes").
func startOnlyNames(p *execPackage) map[string]string {
	names := map[string]string{}
	for _, name := range p.names {
		text := p.textIn(name)
		mentions := func(n ast.Expr) bool { return mentionsStartOnly(n, text) }
		listMentions := func(fields *ast.FieldList) bool { return fields != nil && mentionsStartOnly(fields, text) }
		kind := func(t ast.Expr) string {
			if ft, ok := t.(*ast.FuncType); ok && funcShape(ft, text) != tmuxStarterShape && listMentions(ft.Results) {
				return "makes"
			}
			if mentions(t) {
				return "holds"
			}
			return ""
		}
		for _, decl := range p.files[name].Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if k := kind(d.Type); k != "" {
					key := d.Name.Name
					if d.Recv != nil {
						key = "." + key
					}
					names[key] = k
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if st, ok := s.Type.(*ast.StructType); ok {
							for _, field := range st.Fields.List {
								for _, id := range field.Names {
									if k := kind(field.Type); k != "" {
										names["."+id.Name] = k
									}
								}
							}
						}
					case *ast.ValueSpec:
						for i, id := range s.Names {
							switch {
							case s.Type != nil && kind(s.Type) != "":
								names[id.Name] = kind(s.Type)
							case i < len(s.Values) && mentions(s.Values[i]):
								names[id.Name] = "holds"
							}
						}
					}
				}
			}
		}
	}
	return names
}

// mentionsStartOnly says whether n names a start-only runner's type, or spells its function without names.
func mentionsStartOnly(n ast.Node, text func(ast.Node) string) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.Ident:
			found = found || x.Name == tmuxStarterType
		case *ast.FuncType:
			found = found || funcShape(x, text) == tmuxStarterShape
		}
		return !found
	})
	return found
}

// funcShape is a function type spelled without its parameters' and results' names.
func funcShape(ft *ast.FuncType, text func(ast.Node) string) string {
	list := func(fields *ast.FieldList) []string {
		var types []string
		if fields == nil {
			return nil
		}
		for _, f := range fields.List {
			for range max(1, len(f.Names)) {
				types = append(types, text(f.Type))
			}
		}
		return types
	}
	shape := "func(" + strings.Join(list(ft.Params), ", ") + ")"
	switch results := list(ft.Results); len(results) {
	case 0:
	case 1:
		shape += " " + results[0]
	default:
		shape += " (" + strings.Join(results, ", ") + ")"
	}
	return shape
}

// execSitesOf reads packages and gives each one's starts, following the functions that start what they're handed
// across all of them (an exported one into the packages that import it), and into what holds one taken as a value (a
// field, a local or parameter, a package variable: RT126SAM7R2 D2), until no more are found.
func execSitesOf(packages []*execPackage) map[string][]execSite {
	wrappers := map[string]map[string]execWrapper{}
	holders := map[string]map[string][]execWrapper{}
	for {
		grew := false
		sites := map[string][]execSite{}
		for _, p := range packages {
			for _, name := range p.names {
				found, kept := execSitesIn(p, name, wrappers, holders[p.path])
				sites[p.path] = append(sites[p.path], found...)
				for key, ws := range kept {
					if holders[p.path] == nil {
						holders[p.path] = map[string][]execWrapper{}
					}
					for _, w := range ws {
						if !slices.ContainsFunc(holders[p.path][key], func(h execWrapper) bool {
							return slices.Equal(h.from, w.from) && h.params == w.params && h.variadic == w.variadic
						}) {
							holders[p.path][key], grew = append(holders[p.path][key], w), true
						}
					}
				}
			}
		}
		for _, p := range packages {
			for _, name := range p.names {
				for fn, w := range execWrappersIn(p, name, sites[p.path]) {
					if wrappers[p.path] == nil {
						wrappers[p.path] = map[string]execWrapper{}
					}
					if _, ok := wrappers[p.path][fn]; !ok {
						wrappers[p.path][fn], grew = w, true
					}
				}
			}
		}
		if !grew {
			return sites
		}
	}
}

// execWrappersIn gives the functions of one file whose start takes its program from their parameters.
func execWrappersIn(p *execPackage, name string, sites []execSite) map[string]execWrapper {
	found := map[string]execWrapper{}
	for _, decl := range execFunctions(p.files[name]) {
		fn := decl
		if fn.Name.Name == tmuxOneShotHelper && p.path == corePackage {
			continue
		}
		var from []int
		for _, s := range sites {
			// A Path set on a command isn't where its program is chosen (disclaim.Cmd sets one on every command).
			if s.file == name && s.function == fn.Name.Name && s.owner == funcKey(fn) && !strings.HasSuffix(s.way, ".Path =") {
				from = append(from, s.from...)
			}
		}
		if len(from) == 0 {
			continue
		}
		sort.Ints(from)
		params, variadic := 0, false
		for _, field := range fn.Type.Params.List {
			params += max(1, len(field.Names))
			_, variadic = field.Type.(*ast.Ellipsis)
		}
		found[funcKey(fn)] = execWrapper{from: slices.Compact(from), params: params, variadic: variadic, method: fn.Recv != nil}
	}
	return found
}

// execFunctions are a file's functions as a wrapper is named: each declared function, and each package variable whose
// value is a function literal ("a variable, for the tests"), as a function of that name.
func execFunctions(file *ast.File) []*ast.FuncDecl {
	var functions []*ast.FuncDecl
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			functions = append(functions, d)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if fn := varFunction(spec); fn != nil {
					functions = append(functions, fn)
				}
			}
		}
	}
	return functions
}

// varFunction is a variable whose one value is a function literal, as a function of its name, or nil.
func varFunction(spec ast.Node) *ast.FuncDecl {
	vs, ok := spec.(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
		return nil
	}
	lit, ok := vs.Values[0].(*ast.FuncLit)
	if !ok {
		return nil
	}
	return &ast.FuncDecl{Name: vs.Names[0], Type: lit.Type, Body: lit.Body}
}

// corePackage is the core's own import path.
const corePackage = "github.com/Sauzze/smolcorp/desktop"

// execSitesIn gives the starts in one file of p, wrappers being the functions known to start what they're handed and
// holders what holds one (".field", "local@file function", or a package variable's name). It also gives what this file
// puts one in: a wrapper, or a holder, named without a call and kept in a field, a local, a parameter it's handed to.
func execSitesIn(p *execPackage, name string, wrappers map[string]map[string]execWrapper, holders map[string][]execWrapper) ([]execSite, map[string][]execWrapper) {
	file, fset := p.files[name], p.fset
	text := p.textIn(name)
	local := map[string]string{} // the name an imported package goes by here -> its path
	ptyHere := false
	var sites []execSite
	kept := map[string][]execWrapper{}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		as := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			as = spec.Name.Name
		}
		local[as] = path
		if slices.Contains(ptyPackages, path) {
			ptyHere = true
		}
		if _, starts := execStarters[path]; as == "." && (starts || len(wrappers[path]) > 0) {
			sites = append(sites, execSite{file: name, function: "its imports", way: "import .", program: path, line: fset.Position(spec.Pos()).Line,
				never: "a dot import of a package that starts programs: import it by name"})
		}
	}
	var stack []ast.Node
	// enclosing is the function the node on top of the stack is in (a package variable's function literal counts as a
	// function of the variable's name, a type's fields as the type's), and its declaration.
	enclosing := func() (string, *ast.FuncDecl) {
		for i := len(stack) - 1; i >= 0; i-- {
			switch outer := stack[i].(type) {
			case *ast.FuncDecl:
				return outer.Name.Name, outer
			case *ast.ValueSpec, *ast.TypeSpec:
				// A package's own variable or type, not one declared inside a function.
				if _, ok := stack[max(0, i-2)].(*ast.File); !ok {
					continue
				}
				if vs, ok := outer.(*ast.ValueSpec); ok {
					return vs.Names[0].Name, varFunction(vs)
				}
				return outer.(*ast.TypeSpec).Name.Name, nil
			}
		}
		return "", nil
	}
	// typeOf is the name of the package's type e is, as far as the reading can tell from here (a parameter's, the
	// receiver's, a local's from its declaration or what it's set from, a literal's, a field's, what a function or
	// method of one result gives, a package variable's), or "".
	var typeOf func(e ast.Expr, seen map[string]bool) string
	identType := func(id string, seen map[string]bool) string {
		if seen[id] {
			return ""
		}
		seen[id] = true
		_, decl := enclosing()
		for i := len(stack) - 1; i >= 0; i-- {
			if lit, ok := stack[i].(*ast.FuncLit); ok {
				if _, field := paramAt(lit.Type.Params, id); field != nil {
					return baseType(field.Type)
				}
			}
		}
		if decl == nil {
			return p.vars[id]
		}
		for _, list := range []*ast.FieldList{decl.Type.Params, decl.Recv} {
			if _, field := paramAt(list, id); field != nil {
				return baseType(field.Type)
			}
		}
		declared := ""
		if decl.Body != nil {
			ast.Inspect(decl.Body, func(node ast.Node) bool {
				if vs, ok := node.(*ast.ValueSpec); ok && vs.Type != nil && slices.ContainsFunc(vs.Names, func(n *ast.Ident) bool { return n.Name == id }) {
					declared = baseType(vs.Type)
				}
				return declared == ""
			})
		}
		if declared != "" {
			return declared
		}
		for _, value := range localValues(decl, id, false) {
			if found := typeOf(value, seen); found != "" {
				return found
			}
		}
		return p.vars[id]
	}
	typeOf = func(e ast.Expr, seen map[string]bool) string {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			return identType(x.Name, seen)
		case *ast.StarExpr:
			return typeOf(x.X, seen)
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				return typeOf(x.X, seen)
			}
		case *ast.CompositeLit:
			return baseType(x.Type)
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && local[id.Name] != "" {
				return ""
			}
			if owner := typeOf(x.X, seen); owner != "" {
				return p.fields[owner][x.Sel.Name]
			}
		case *ast.CallExpr:
			switch f := ast.Unparen(x.Fun).(type) {
			case *ast.Ident:
				if p.types[f.Name] {
					return f.Name
				}
				return p.results[f.Name]
			case *ast.SelectorExpr:
				if id, ok := f.X.(*ast.Ident); ok && local[id.Name] != "" {
					return ""
				}
				if owner := typeOf(f.X, seen); owner != "" {
					return p.results[owner+"."+f.Sel.Name]
				}
			}
		}
		return ""
	}
	ownerOf := func(e ast.Expr) string { return typeOf(e, map[string]bool{}) }
	// add adds a site: program is what it runs (for a wrapper's call, the arguments at its parameters), args the rest of
	// its words, read for tmux too. Only a launcher's (a shell, env…) words, which it runs, make its function one that
	// starts what it's handed.
	add := func(n ast.Node, way string, program, args []ast.Expr) *execSite {
		site := execSite{file: name, way: way, line: fset.Position(n.Pos()).Line, program: "…"}
		var decl *ast.FuncDecl
		site.function, decl = enclosing()
		if site.function == "" {
			site.function = "?"
		}
		if decl != nil {
			site.owner = funcKey(decl)
		}
		if len(program) > 0 {
			var said []string
			for _, e := range program {
				if lit, ok := e.(*ast.CompositeLit); ok && lit.Type != nil {
					said = append(said, text(lit.Type)+"{…}") // its words are still read below
					continue
				}
				said = append(said, text(e))
			}
			site.program = strings.Join(said, ", ")
			site.from, site.note = execFrom(stack, decl, program)
			if lit, ok := program[0].(*ast.BasicLit); ok && len(program) == 1 {
				if word, _ := strconv.Unquote(lit.Value); slices.Contains(execLaunchers, filepath.Base(word)) {
					site.from, site.note = execFrom(stack, decl, args)
				}
			}
			words := execWords(stack, decl, append(slices.Clone(program), args...), text)
			site.tmux = strings.Contains(strings.ToLower(words+" "+site.function), "tmux")
		}
		sites = append(sites, site)
		return &sites[len(sites)-1]
	}
	// topAt is where the node on top of the stack is, out of any parentheses.
	topAt := func() int {
		i := len(stack) - 1
		for i > 0 {
			if _, ok := stack[i-1].(*ast.ParenExpr); !ok {
				break
			}
			i--
		}
		return i
	}
	// top is the node on top of the stack, out of any parentheses, and what's under it.
	top := func() (ast.Node, ast.Node) {
		i := topAt()
		if i == 0 {
			return stack[i], nil
		}
		return stack[i], stack[i-1]
	}
	// called is whether the node on top of the stack is the function of the call under it.
	called := func() (*ast.CallExpr, bool) {
		node, under := top()
		call, ok := under.(*ast.CallExpr)
		return call, ok && call.Fun == node
	}
	// setTo is what the node on top of the stack is set to, when it's what an assignment sets or a literal's key.
	setTo := func() ast.Expr {
		node, under := top()
		switch parent := under.(type) {
		case *ast.AssignStmt:
			if at := slices.IndexFunc(parent.Lhs, func(e ast.Expr) bool { return e == node }); at >= 0 {
				if len(parent.Rhs) == len(parent.Lhs) {
					return parent.Rhs[at]
				}
				return parent.Rhs[0]
			}
		case *ast.KeyValueExpr:
			if parent.Key == node {
				return parent.Value
			}
		}
		return nil
	}
	// keep follows ws, a function that starts what it's handed named here without a call, into what holds it: a
	// literal's field or a field set (".name"), a local of this function, the parameter of a function of this package
	// it's handed to ("name@file function"), or a package variable. It says whether it could, or whether nothing holds
	// it (a nil check, the blank).
	keep := func(ws []execWrapper) bool {
		node, under := top()
		fn, _ := enclosing()
		scoped := func(id string) string {
			if fn == "" {
				return id
			}
			return id + "@" + name + " " + fn
		}
		put := func(key string) { kept[key] = append(kept[key], ws...) }
		switch parent := under.(type) {
		case *ast.KeyValueExpr:
			if parent.Value == node {
				owner := ""
				if at := topAt(); at >= 2 {
					if lit, ok := stack[at-2].(*ast.CompositeLit); ok {
						owner = baseType(lit.Type)
					}
				}
				put(owner + "." + exprName(parent.Key))
				return true
			}
		case *ast.AssignStmt:
			at := slices.IndexFunc(parent.Rhs, func(e ast.Expr) bool { return e == node })
			if at < 0 || len(parent.Lhs) != len(parent.Rhs) {
				return false
			}
			switch left := parent.Lhs[at].(type) {
			case *ast.SelectorExpr:
				put(ownerOf(left.X) + "." + left.Sel.Name)
			case *ast.Ident:
				if left.Name != "_" {
					put(scoped(left.Name))
				}
			default:
				return false
			}
			return true
		case *ast.ValueSpec:
			at := slices.IndexFunc(parent.Values, func(e ast.Expr) bool { return e == node })
			if at < 0 || len(parent.Names) != len(parent.Values) {
				return false
			}
			if parent.Names[at].Name != "_" {
				put(scoped(parent.Names[at].Name))
			}
			return true
		case *ast.CallExpr:
			at := slices.IndexFunc(parent.Args, func(e ast.Expr) bool { return e == node })
			var callees []execFunc
			switch f := parent.Fun.(type) {
			case *ast.Ident:
				callees = p.funcs[f.Name]
			case *ast.SelectorExpr:
				if id, ok := f.X.(*ast.Ident); !ok || local[id.Name] == "" {
					callees = slices.Concat(byOwner(p.funcs, ownerOf(f.X), f.Sel.Name)...)
				}
			}
			if at < 0 || len(callees) == 0 {
				return false
			}
			for _, f := range callees {
				if param := paramName(f.decl.Type.Params, at); param != "" {
					kept[param+"@"+f.file+" "+f.decl.Name.Name] = append(kept[param+"@"+f.file+" "+f.decl.Name.Name], ws...)
				}
			}
			return true
		case *ast.BinaryExpr:
			return parent.Op == token.EQL || parent.Op == token.NEQ
		}
		return false
	}
	// atWrapper adds a call of a wrapper: its program is the arguments at the wrapper's parameters. A wrapper named
	// without a call is a site of its own, and is followed into what holds it.
	atWrapper := func(n ast.Node, way string, w execWrapper) {
		call, ok := called()
		if !ok {
			add(n, "names "+way, nil, nil).note = "a function that starts what it's handed, taken as a value: what it's called with later isn't read here"
			keep([]execWrapper{w})
			return
		}
		var program, rest []ast.Expr
		for at, arg := range call.Args {
			if slices.Contains(w.from, at) || w.variadic && at >= w.params-1 && slices.Contains(w.from, w.params-1) {
				program = append(program, arg)
			} else {
				rest = append(rest, arg)
			}
		}
		site := add(n, way, program, rest)
		if way == "tmuxStartCall" && !startUsedAtOnce(stack, call) {
			site.never = "what tmuxStartCall gives is kept as a value: call it at once, or hand it to startingTmux for a start-only runner"
		}
	}
	// atHolder adds a call through what holds a wrapper (a field, a local, a parameter, a package variable), when its
	// arguments fit one it holds; named without a call, it's followed into what holds it next, or is a site of its own.
	atHolder := func(n ast.Node, way string, ws []execWrapper) {
		if setTo() != nil {
			return // what it's set to is read where it's named
		}
		if node, under := top(); under != nil {
			if sel, ok := under.(*ast.SelectorExpr); ok && sel.X == node {
				return // something of it: a function has none
			}
		}
		if call, ok := called(); ok {
			for _, w := range ws {
				if w.fits(call.Args) {
					atWrapper(n, way, w)
					return
				}
			}
			return
		}
		if !keep(ws) {
			add(n, "names "+way, nil, nil).note = "it holds a function that starts what it's handed, and goes where the reading doesn't follow it"
		}
	}
	// atStartOnly adds a place a start-only runner is named (RT126SAM7R2 B1′): set, called (a call through one that
	// holds one is a tmux start), made, or named.
	atStartOnly := func(n ast.Node, way, kind string) {
		if value := setTo(); value != nil {
			site := add(n, "sets "+way, []ast.Expr{value}, nil)
			site.tmux, site.from, site.note = false, nil, ""
			if call, ok := ast.Unparen(value).(*ast.CallExpr); kind == "holds" && (!ok || exprName(call.Fun) != tmuxStartGuard) {
				site.never = "a start-only runner kept as " + tmuxStartGuard + " didn't make it: it would run any call with its guard off"
			}
			return
		}
		if call, ok := called(); ok {
			site := add(n, way, call.Args, nil)
			site.tmux, site.from, site.note = kind == "holds", nil, ""
			return
		}
		add(n, "names "+way, nil, nil)
	}
	// makesStartOnly says whether f is a function that makes a start-only runner, or the conversion to one.
	makesStartOnly := func(f ast.Expr) bool {
		switch x := ast.Unparen(f).(type) {
		case *ast.Ident:
			return x.Name == tmuxStarterType || p.starters[x.Name] == "makes"
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			return (!ok || local[id.Name] == "") && p.starters["."+x.Sel.Name] == "makes"
		}
		return false
	}
	// declared says whether an identifier is a name being declared or a selector's: a function's, a variable's, a
	// type's, a parameter's or field's, a literal's key, or what follows a dot.
	declared := func(n *ast.Ident) bool {
		if len(stack) < 2 {
			return false
		}
		switch outer := stack[len(stack)-2].(type) {
		case *ast.FuncDecl:
			return outer.Name == n
		case *ast.ValueSpec:
			return slices.Contains(outer.Names, n)
		case *ast.TypeSpec:
			return outer.Name == n
		case *ast.Field:
			return slices.Contains(outer.Names, n)
		case *ast.KeyValueExpr:
			return outer.Key == n
		case *ast.SelectorExpr:
			return outer.Sel == n
		}
		return false
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, node)
		switch n := node.(type) {
		case *ast.CompositeLit:
			if sel, ok := n.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Cmd" && local[exprName(sel.X)] == "os/exec" {
				for _, element := range n.Elts {
					if kv, ok := element.(*ast.KeyValueExpr); ok && exprName(kv.Key) == "Path" {
						add(n, "exec.Cmd{Path}", []ast.Expr{kv.Value}, n.Elts)
					}
				}
			}
		case *ast.AssignStmt:
			for i, left := range n.Lhs {
				if sel, ok := left.(*ast.SelectorExpr); ok && sel.Sel.Name == "Path" && i < len(n.Rhs) {
					add(n, text(sel.X)+".Path =", []ast.Expr{n.Rhs[i]}, nil)
				}
			}
		case *ast.CallExpr:
			// A start-only runner made and called at once: o.Sandbox(desk)(args), tmuxStarter(f)(args).
			if inner, ok := ast.Unparen(n.Fun).(*ast.CallExpr); ok && makesStartOnly(inner.Fun) {
				site := add(n, text(inner)+" called", n.Args, nil)
				site.tmux, site.from, site.note = true, nil, ""
			}
		case *ast.FuncType:
			if spec, ok := stack[len(stack)-2].(*ast.TypeSpec); !(ok && spec.Name.Name == tmuxStarterType) && funcShape(n, text) == tmuxStarterShape {
				add(n, "names "+tmuxStarterShape, nil, nil)
			}
		case *ast.SelectorExpr:
			if id, ok := n.X.(*ast.Ident); ok && local[id.Name] != "" {
				path := local[id.Name]
				way := path[strings.LastIndex(path, "/")+1:] + "." + n.Sel.Name
				if at, ok := execStarters[path][n.Sel.Name]; ok {
					if call, ok := called(); !ok {
						add(n, "names "+way, nil, nil).note = "a way to start a program taken as a value: what it's called with later isn't read here"
					} else if at < len(call.Args) {
						add(n, way, call.Args[at:at+1], call.Args[at+1:])
					}
				} else if w, ok := wrappers[path][n.Sel.Name]; ok && !w.method && ast.IsExported(n.Sel.Name) {
					atWrapper(n, way, w)
				}
				return true
			}
			if kind := p.starters["."+n.Sel.Name]; kind != "" {
				atStartOnly(n, "."+n.Sel.Name, kind)
				return true
			}
			if call, ok := called(); ok && ptyHere && (n.Sel.Name == "Command" || n.Sel.Name == "CommandContext") {
				at := map[string]int{"Command": 0, "CommandContext": 1}[n.Sel.Name]
				if at < len(call.Args) {
					add(n, "pty."+n.Sel.Name, call.Args[at:at+1], call.Args[at+1:])
				}
				return true
			}
			owner := ownerOf(n.X)
			if methods := byOwner(wrappers[p.path], owner, n.Sel.Name); len(methods) > 0 {
				atWrapper(n, "."+n.Sel.Name, methods[0]) // a method value too (RT126SAM7R2 follow-up 5)
				return true
			}
			if ws := slices.Concat(byOwner(holders, owner, n.Sel.Name)...); len(ws) > 0 {
				atHolder(n, "."+n.Sel.Name, ws)
			}
		case *ast.Ident:
			if n.Name == tmuxStarterType && !declared(n) {
				add(n, "names "+tmuxStarterType, nil, nil)
				return true
			}
			if kind := p.starters["."+n.Name]; kind != "" && len(stack) >= 2 {
				if kv, ok := stack[len(stack)-2].(*ast.KeyValueExpr); ok && kv.Key == n {
					atStartOnly(n, "."+n.Name, kind) // a field set in a literal
					return true
				}
			}
			if declared(n) {
				return true
			}
			if kind := p.starters[n.Name]; kind != "" {
				atStartOnly(n, n.Name, kind)
				return true
			}
			if w, ok := wrappers[p.path][n.Name]; ok && !w.method {
				atWrapper(n, n.Name, w)
				return true
			}
			fn, _ := enclosing()
			if ws := append(slices.Clone(holders[n.Name+"@"+name+" "+fn]), holders[n.Name]...); len(ws) > 0 {
				atHolder(n, n.Name, ws)
			}
		}
		return true
	})
	return sites, kept
}

// startUsedAtOnce says whether call, a call of tmuxStartCall in stack, is called at once (as it is, or through
// withoutStartFile), or handed to startingTmux, which makes a start-only runner of it.
func startUsedAtOnce(stack []ast.Node, call *ast.CallExpr) bool {
	at := slices.IndexFunc(stack, func(n ast.Node) bool { return n == call })
	if at < 0 {
		return false
	}
	var cur ast.Node = call
	for i := at - 1; i >= 0; i-- {
		switch outer := stack[i].(type) {
		case *ast.ParenExpr:
			cur = outer
			continue
		case *ast.CallExpr:
			if outer.Fun == cur {
				return true
			}
			if slices.ContainsFunc(outer.Args, func(e ast.Expr) bool { return e == cur }) {
				switch exprName(outer.Fun) {
				case "withoutStartFile":
					cur = outer
					continue
				case "startingTmux":
					return true
				}
			}
		}
		return false
	}
	return false
}

// paramName is the name of the parameter at of list (the last one, variadic, takes the rest), or "".
func paramName(list *ast.FieldList, at int) string {
	var names []string
	variadic := false
	for _, field := range list.List {
		_, variadic = field.Type.(*ast.Ellipsis)
		if len(field.Names) == 0 {
			names = append(names, "")
		}
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
	}
	switch {
	case at < len(names):
		return names[at]
	case variadic && len(names) > 0:
		return names[len(names)-1]
	}
	return ""
}

// execWords are a start's words followed back through the locals they were set from, inside its function, with the
// types of the parameters and receiver they come from: what says whether it can be tmux.
func execWords(stack []ast.Node, decl *ast.FuncDecl, exprs []ast.Expr, text func(ast.Node) string) string {
	var b strings.Builder
	seen := map[string]bool{}
	var follow func(e ast.Node)
	name := func(id *ast.Ident) {
		if seen[id.Name] {
			return
		}
		seen[id.Name] = true
		for i := len(stack) - 1; i >= 0; i-- {
			if lit, ok := stack[i].(*ast.FuncLit); ok && (decl == nil || lit.Type != decl.Type) {
				if _, field := paramAt(lit.Type.Params, id.Name); field != nil {
					b.WriteString(text(field.Type) + " ")
					return
				}
			}
		}
		if decl == nil {
			return
		}
		if _, field := paramAt(decl.Type.Params, id.Name); field != nil {
			b.WriteString(text(field.Type) + " ")
			return
		}
		if _, field := paramAt(decl.Recv, id.Name); field != nil {
			b.WriteString(text(field.Type) + " ")
			return
		}
		for _, value := range localValues(decl, id.Name, false) {
			follow(value)
		}
	}
	follow = func(e ast.Node) {
		b.WriteString(text(e) + " ")
		ast.Inspect(e, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.SelectorExpr:
				// x.field: x is followed, the field's name is in the words already.
				ast.Inspect(n.X, func(inner ast.Node) bool {
					if id, ok := inner.(*ast.Ident); ok {
						name(id)
					}
					return true
				})
				return false
			case *ast.Ident:
				name(n)
			}
			return true
		})
	}
	for _, e := range exprs {
		follow(e)
	}
	return b.String()
}

// execFrom is which parameters of its function a start's program is (itself, an element, a part or a field of one, or
// a local copied from one: not what a call made of it), and why it can't be read (it's a function literal's parameter,
// which no name calls).
func execFrom(stack []ast.Node, decl *ast.FuncDecl, exprs []ast.Expr) (from []int, note string) {
	seen := map[string]bool{}
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.ParenExpr:
			walk(n.X)
		case *ast.StarExpr:
			walk(n.X)
		case *ast.UnaryExpr:
			walk(n.X)
		case *ast.IndexExpr:
			walk(n.X)
		case *ast.SliceExpr:
			walk(n.X)
		case *ast.SelectorExpr:
			walk(n.X)
		case *ast.Ident:
			if seen[n.Name] {
				return
			}
			seen[n.Name] = true
			for i := len(stack) - 1; i >= 0; i-- {
				if lit, ok := stack[i].(*ast.FuncLit); ok && (decl == nil || lit.Type != decl.Type) {
					if _, field := paramAt(lit.Type.Params, n.Name); field != nil {
						note = "it starts a program a function literal is handed: what that literal is called with isn't read here"
						return
					}
				}
			}
			if decl == nil {
				return
			}
			if at, field := paramAt(decl.Type.Params, n.Name); field != nil {
				from = append(from, at)
				return
			}
			for _, value := range localValues(decl, n.Name, true) {
				walk(value)
			}
		}
	}
	for _, e := range exprs {
		walk(e)
	}
	sort.Ints(from)
	return slices.Compact(from), note
}

// localValues are what the local called name is set from in decl: each value of an assignment or declaration of it
// (only the one at its place, when exact, else every value), and what a range over gives it.
func localValues(decl *ast.FuncDecl, name string, exact bool) []ast.Expr {
	var values []ast.Expr
	if decl.Body == nil {
		return nil
	}
	pick := func(names []ast.Expr, rhs []ast.Expr) {
		for i, left := range names {
			if exprName(left) != name {
				continue
			}
			switch {
			case len(rhs) == len(names):
				values = append(values, rhs[i])
			case !exact:
				values = append(values, rhs...)
			}
		}
	}
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			pick(n.Lhs, n.Rhs)
		case *ast.ValueSpec:
			var names []ast.Expr
			for _, id := range n.Names {
				names = append(names, id)
			}
			pick(names, n.Values)
		case *ast.RangeStmt:
			if exprName(n.Key) == name || n.Value != nil && exprName(n.Value) == name {
				values = append(values, n.X)
			}
		}
		return true
	})
	return values
}

// paramAt is the index of the parameter called name in list, and its field, or nil.
func paramAt(list *ast.FieldList, name string) (int, *ast.Field) {
	if list == nil {
		return -1, nil
	}
	at := 0
	for _, field := range list.List {
		if len(field.Names) == 0 {
			at++
		}
		for _, n := range field.Names {
			if n.Name == name {
				return at, field
			}
			at++
		}
	}
	return -1, nil
}

// modulePath is this module's import path.
const modulePath = "github.com/Sauzze/smolcorp"

// execPackageDirs are the folders under root's desktop and internal that hold a package's non-test Go files, every one,
// a package inside another too (RT126SAM7R2 D3: a wrapper in desktop/deskexec), by import path. Like go, it skips
// testdata and a folder whose name starts with "." or "_".
func execPackageDirs(t testing.TB, root string) map[string]string {
	t.Helper()
	dirs := map[string]string{}
	for _, top := range []string{"desktop", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}
			if base := d.Name(); path != filepath.Join(root, top) && (base == "testdata" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_")) {
				return filepath.SkipDir
			}
			names, err := filepath.Glob(filepath.Join(path, "*.go"))
			if err == nil && slices.ContainsFunc(names, func(n string) bool { return !strings.HasSuffix(n, "_test.go") }) {
				rel, _ := filepath.Rel(root, path)
				dirs[modulePath+"/"+filepath.ToSlash(rel)] = path
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return dirs
}

// moduleExecSites reads every package under root's desktop and internal, and gives the core's starts (its own package
// and every one inside it, their files named from desktop/) and the other packages' (named from the module's root).
func moduleExecSites(t testing.TB, root string) (core, others []execSite) {
	t.Helper()
	var packages []*execPackage
	for path, dir := range execPackageDirs(t, root) {
		prefix := ""
		if rel, ok := strings.CutPrefix(path, corePackage+"/"); ok {
			prefix = rel + "/"
		} else if path != corePackage {
			prefix = strings.TrimPrefix(path, modulePath+"/") + "/"
		}
		packages = append(packages, readExecPackage(t, dir, path, prefix))
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].path < packages[j].path })
	for path, sites := range execSitesOf(packages) {
		if path == corePackage || strings.HasPrefix(path, corePackage+"/") {
			core = append(core, sites...)
		} else {
			others = append(others, sites...)
		}
	}
	return core, others
}

// coreExecSites reads the core and every other package of this module under desktop and internal, and gives the
// core's starts, and the others' starts that can run a tmux (RT126SAM7R2 D3: a tmux exec inside an internal package,
// its program picked there), which the lists must hold and allow like the core's.
func coreExecSites(t testing.TB) []execSite {
	t.Helper()
	core, others := moduleExecSites(t, "..")
	return append(core, internalTmux(others)...)
}

// internalTmux are the sites of others that can run a tmux.
func internalTmux(others []execSite) []execSite {
	return slices.DeleteFunc(slices.Clone(others), func(s execSite) bool { return !s.tmux })
}

// tmuxExecProblems is what's wrong with sites against the lists: a site not listed or listed and gone, a site that
// can't be read, and a tmux outside the helper, the start and the attach lines.
func tmuxExecProblems(sites []execSite, listed, tmuxAllowed map[string]string) []string {
	var problems []string
	found := map[string]bool{}
	count := map[string]int{}
	for _, s := range sites {
		key := s.key()
		if count[key]++; count[key] > 1 {
			key += " #" + strconv.Itoa(count[key])
		}
		found[key] = true
		at := s.file + ":" + strconv.Itoa(s.line) + " " + key
		switch {
		case s.never != "":
			problems = append(problems, at+": "+s.never)
		case strings.HasPrefix(listed[key], "tmux") && tmuxAllowed[key] == "" && s.way != "tmuxStartCall":
			// Listed honestly as a tmux, but not where one may run (RT126SAM7R2 follow-up 4).
			problems = append(problems, at+" is listed as running a tmux, which only tmuxClient and the starts may")
		case listed[key] == "":
			why := ""
			if s.note != "" {
				why = " (" + s.note + ")"
			}
			problems = append(problems, at+" starts a program and isn't on the list"+why+": run tmux only through tmuxClient (a "+
				"start through tmuxStartCall), and list any other start with what it runs")
		}
		if s.tmux && tmuxAllowed[key] == "" && s.way != "tmuxStartCall" {
			problems = append(problems, at+" runs a tmux outside tmuxClient and tmuxStartCall")
		}
	}
	for key := range listed {
		if !found[key] {
			problems = append(problems, key+" is listed but isn't there now: take it off")
		}
	}
	for key := range tmuxAllowed {
		if !found[key] {
			problems = append(problems, key+" may run a tmux but isn't there now: take it off")
		}
	}
	sort.Strings(problems)
	return problems
}

// tmuxPicks reads one file and gives where it uses a picker, each as "file function".
func tmuxPicks(name string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var stack []ast.Node
	var picks []string
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, node)
		n, ok := node.(*ast.Ident)
		if !ok || !slices.Contains(tmuxPickers, n.Name) {
			return true
		}
		// Its own declaration isn't a use.
		if len(stack) >= 2 {
			switch outer := stack[len(stack)-2].(type) {
			case *ast.FuncDecl:
				if outer.Name == n {
					return true
				}
			case *ast.ValueSpec:
				if slices.Contains(outer.Names, n) {
					return true
				}
			}
		}
		for i := len(stack) - 1; i >= 0; i-- {
			switch outer := stack[i].(type) {
			case *ast.FuncDecl:
				picks = append(picks, name+" "+outer.Name.Name)
				return true
			case *ast.ValueSpec:
				picks = append(picks, name+" "+outer.Names[0].Name)
				return true
			}
		}
		picks = append(picks, name+" ?")
		return true
	})
	return picks, nil
}

// The grep row (release gate item 1): the core's code, read as it is (every system's files), starts nothing that isn't
// on the closed list, runs a tmux nowhere but in tmuxClient and tmuxStartCall (and the listed calls of tmuxStartCall),
// and uses a picker nowhere but in the functions listed with what they do with it.
func TestTheCoreRunsTmuxOnlyThroughItsHelper(t *testing.T) {
	if _, ok := execPackageDirs(t, "..")[modulePath+"/internal/sandbox/tmuxfake"]; !ok {
		t.Error("a package inside another isn't read")
	}
	sites := coreExecSites(t)
	if len(sites) < 40 {
		t.Fatalf("only %d starts found: the reading is looking in the wrong place", len(sites))
	}
	for _, problem := range tmuxExecProblems(sites, tmuxExecSites, tmuxExecAllowed) {
		t.Error(problem)
	}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var picks []string
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		p, err := tmuxPicks(name, src)
		if err != nil {
			t.Fatal(err)
		}
		picks = append(picks, p...)
	}
	sort.Strings(picks)
	for _, at := range slices.Compact(picks) {
		if _, ok := tmuxPickedIn[at]; !ok {
			t.Errorf("%s uses a tmux picker: run it through tmuxClient (or a start through tmuxStartCall), then list it in tmuxPickedIn with what it does", at)
		}
	}
	for at := range tmuxPickedIn {
		if !slices.Contains(picks, at) {
			t.Errorf("%s uses no tmux picker: the list is stale", at)
		}
	}
}

// tmuxRowCore is a small core for the row's own checks: the two tmux helpers, a terminal's tool, a desk's tmux with its
// start-only runner (made through onlyStarts, called by its start alone), a start and a ps, all listed (tmuxRowListed),
// and nothing else.
const tmuxRowCore = `package desktop

import (
	"context"
	"os/exec"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

func tmuxClient(program, guard string, args ...string) *exec.Cmd {
	return disclaim.Cmd(exec.CommandContext(context.Background(), program, args...))
}

func tmuxStartCall(path string) func(args []string) error {
	return func(args []string) error { return disclaim.Cmd(exec.Command(path, args...)).Run() }
}

func terminalCommand(name string, args ...string) *exec.Cmd {
	return disclaim.Cmd(exec.CommandContext(context.Background(), name, args...))
}

type tmuxStarter func(args []string) (output string, server int, err error)

type hireTmux struct {
	program, socket string
	startOnly       tmuxStarter
}

func startingTmux(run func(args []string) error) tmuxStarter {
	return func(args []string) (string, int, error) { return "", 0, run(args) }
}

func onlyStarts(socket string, start tmuxStarter) tmuxStarter {
	return func(args []string) (string, int, error) { return start(args) }
}

func newHireTmux(program string) *hireTmux {
	h := &hireTmux{program: program}
	h.startOnly = onlyStarts(h.socket, startingTmux(tmuxStartCall(program)))
	return h
}

func (h *hireTmux) command(args ...string) []string { return append([]string{"-S", h.socket}, args...) }

func (h *hireTmux) start() error {
	if h.startOnly == nil {
		return nil
	}
	_, _, err := h.startOnly(h.command("new-session"))
	return err
}

func tmuxPath() string    { return "/opt/tmux" }
func bundledTmux() string { return "/opt/smolcorp/tmux" }

func (h *hireTmux) alive() error { return tmuxClient(h.program, "unchecked", "-S", h.socket, "has-session").Run() }

func startGrid(program string) error { return tmuxStartCall(program)([]string{"new-session"}) }

func status() error { return terminalCommand("ps", "-p", "1").Run() }
`

var tmuxRowListed = map[string]string{
	"x.go tmuxClient exec.CommandContext(program)":   "the helper",
	"x.go tmuxStartCall exec.Command(path)":          "the start",
	"x.go terminalCommand exec.CommandContext(name)": "what its callers hand it",
	"x.go startGrid tmuxStartCall(program)":          "a start",
	`x.go status terminalCommand("ps")`:              "ps",
	// The start-only runner: its type's uses, its making, its keeping, its one call.
	"x.go hireTmux names tmuxStarter(…)":                                                           "its field",
	"x.go startingTmux names tmuxStarter(…)":                                                       "makes one",
	"x.go startingTmux names " + tmuxStarterShape + "(…)":                                          "makes one",
	"x.go onlyStarts names tmuxStarter(…)":                                                         "makes one",
	"x.go onlyStarts names tmuxStarter(…) #2":                                                      "makes one",
	"x.go onlyStarts names " + tmuxStarterShape + "(…)":                                            "makes one",
	"x.go newHireTmux tmuxStartCall(program)":                                                      "a desk's start",
	"x.go newHireTmux startingTmux(tmuxStartCall(program))":                                        "a desk's start",
	"x.go newHireTmux onlyStarts(h.socket, startingTmux(tmuxStartCall(program)))":                  "a desk's start",
	"x.go newHireTmux sets .startOnly(onlyStarts(h.socket, startingTmux(tmuxStartCall(program))))": "a desk's start",
	"x.go start names .startOnly(…)":                                                               "whether it has one",
	`x.go start .startOnly(h.command("new-session"))`:                                              "tmux, the desk's start",
}

var tmuxRowAllowed = map[string]string{
	"x.go tmuxClient exec.CommandContext(program)":    "the helper",
	"x.go tmuxStartCall exec.Command(path)":           "the start",
	`x.go start .startOnly(h.command("new-session"))`: "the start",
}

// tmuxRowProblems is what the row says of the small core, edited by edit and with more (another file, y.go) and sub
// (desktop/deskexec, a package inside the core's folder, or one more file of internal/runner, by its package clause),
// and with listed (and tmux allowed at) the sites of tmuxRowListed
// plus list's.
func tmuxRowProblems(t *testing.T, edit func(string) string, more, sub string, list map[string]string) []string {
	t.Helper()
	files := map[string][]byte{"x.go": []byte(edit(tmuxRowCore))}
	if more != "" {
		files["y.go"] = []byte(more)
	}
	core, err := parseExecPackage(corePackage, files)
	if err != nil {
		t.Fatal(err)
	}
	runnerFiles := map[string][]byte{"internal/runner/runner.go": []byte(`package runner

import "os/exec"

// Run starts what it's handed: a wrapper in another package.
func Run(program string, args ...string) error { return exec.Command(program, args...).Run() }
`)}
	if strings.HasPrefix(sub, "package runner") {
		runnerFiles["internal/runner/more.go"] = []byte(sub)
	}
	runner, err := parseExecPackage("github.com/Sauzze/smolcorp/internal/runner", runnerFiles)
	if err != nil {
		t.Fatal(err)
	}
	packages := []*execPackage{core, runner}
	if strings.HasPrefix(sub, "package deskexec") {
		inner, err := parseExecPackage(corePackage+"/deskexec", map[string][]byte{"deskexec/deskexec.go": []byte(sub)})
		if err != nil {
			t.Fatal(err)
		}
		packages = append(packages, inner)
	}
	listed := maps.Clone(tmuxRowListed)
	maps.Copy(listed, list)
	found := execSitesOf(packages)
	return tmuxExecProblems(slices.Concat(found[corePackage], found[corePackage+"/deskexec"], internalTmux(found[runner.path])), listed, tmuxRowAllowed)
}

// The row's own checks (RT126SAM7 B1): the small core passes, and each way of running a tmux outside the helper,
// the reviewer's two (MA, MB) first, is caught, with the words that say where.
func TestTheTmuxRowCatchesEachWayToRunTmux(t *testing.T) {
	same := func(s string) string { return s }
	if problems := tmuxRowProblems(t, same, "", "", nil); len(problems) != 0 {
		t.Fatalf("the small core: %q", problems)
	}
	const head = "package desktop\n\nimport (\n\t\"context\"\n\t\"os\"\n\t\"os/exec\"\n\t\"syscall\"\n\n\t\"github.com/Sauzze/smolcorp/internal/disclaim\"\n\t\"github.com/Sauzze/smolcorp/internal/runner\"\n)\n\n"
	for _, c := range []struct {
		name, more, sub string
		edit            func(string) string
		list            map[string]string
		want            []string
	}{
		{name: "MA: a variable, in a function already reading a tmux", more: head + `type commandTab struct{ socket string }

func tmuxFor(socket string) (string, string) { return "/opt/tmux", "unchecked" }

func (t commandTab) alive() bool {
	program, _ := tmuxFor(t.socket)
	cmd := terminalCommand(program, "-S", t.socket, "display-message", "-p", "#{pane_id}")
	return cmd.Run() == nil
}
`, want: []string{"y.go alive terminalCommand(program) starts a program and isn't on the list", "y.go alive terminalCommand(program) runs a tmux"}},
		{name: "MB: a new multi-line exec.CommandContext on a field, disclaimed", more: head + `func (h *hireTmux) snapshot(ctx context.Context) ([]byte, error) {
	return disclaim.Cmd(exec.CommandContext(
		ctx,
		h.program,
		"-S", h.socket, "capture-pane", "-p",
	)).Output()
}
`, want: []string{"y.go snapshot exec.CommandContext(h.program) starts a program", "y.go snapshot exec.CommandContext(h.program) runs a tmux"}},
		{name: "MB, listed as something else", more: head + `func (h *hireTmux) snapshot(ctx context.Context) ([]byte, error) {
	return disclaim.Cmd(exec.CommandContext(ctx, h.program, "-S", h.socket, "capture-pane", "-p")).Output()
}
`, list: map[string]string{"y.go snapshot exec.CommandContext(h.program)": "git, honest"},
			want: []string{"y.go snapshot exec.CommandContext(h.program) runs a tmux"}},
		{name: "a local set from a picker", more: head + "func f() { p := tmuxPath(); _ = exec.Command(p, \"ls\").Run() }\n",
			want: []string{"y.go f exec.Command(p) runs a tmux"}},
		{name: "a local set from a local, and a range", more: head + "func f() {\n\tvar q = bundledTmux()\n\tp := q\n\tfor _, r := range []string{p} {\n\t\t_ = exec.Command(r).Run()\n\t}\n}\n",
			want: []string{"y.go f exec.Command(r) runs a tmux"}},
		{name: "a function's result", more: head + "func f() { _ = exec.Command(bundledTmux(), \"ls\").Run() }\n",
			want: []string{"y.go f exec.Command(bundledTmux()) runs a tmux"}},
		{name: "a receiver's field through a local", more: head + "func (h *hireTmux) look() { p := h.program; _ = exec.Command(p).Run() }\n",
			want: []string{"y.go look exec.Command(p) runs a tmux"}},
		{name: "a parameter's field", more: head + "func look(desk *hireTmux) { _ = exec.Command(desk.program).Run() }\n",
			want: []string{"y.go look exec.Command(desk.program) runs a tmux"}},
		{name: "the start's helper used for a one-shot call", more: head + "func (h *hireTmux) peek() { _ = tmuxStartCall(h.program)([]string{\"-S\", h.socket, \"capture-pane\"}) }\n",
			want: []string{"y.go peek tmuxStartCall(h.program) starts a program and isn't on the list"}},
		{name: "a wrapper of its own", more: head + "func runIt(p string, args ...string) error { return exec.Command(p, args...).Run() }\n\nfunc (h *hireTmux) snap() { _ = runIt(h.program, \"-S\", h.socket) }\n",
			want: []string{"y.go runIt exec.Command(p) starts a program", "y.go snap runIt(h.program) runs a tmux"}},
		{name: "a wrapper of a wrapper", more: head + "func viaTerminal(p string) error { return terminalCommand(p).Run() }\n\nfunc twice(p string) error { return viaTerminal(p) }\n\nfunc f() { _ = twice(tmuxPath()) }\n",
			want: []string{"y.go f twice(tmuxPath()) runs a tmux"}},
		{name: "a method that wraps", more: head + "func (h *hireTmux) exec(p string) error { return exec.Command(p).Run() }\n\nfunc (h *hireTmux) f() { _ = h.exec(h.program) }\n",
			want: []string{"y.go f .exec(h.program) runs a tmux"}},
		{name: "another package's wrapper", more: head + "func (h *hireTmux) f() { _ = runner.Run(h.program, \"-S\", h.socket) }\n",
			want: []string{"y.go f runner.Run(h.program) runs a tmux"}},
		{name: "exec.Command taken as a value", more: head + "func f() { run := exec.Command; _ = run(tmuxPath()).Run() }\n",
			want: []string{"y.go f names exec.Command(…) starts a program and isn't on the list (a way to start a program taken as a value"}},
		{name: "a wrapper taken as a value", more: head + "func f() { run := terminalCommand; _ = run(tmuxPath()).Run() }\n",
			want: []string{"y.go f names terminalCommand(…) starts a program and isn't on the list (a function that starts what it's handed, taken as a value"}},
		{name: "a function literal handed the program", more: head + "func f() {\n\trun := func(p string) error { return exec.Command(p).Run() }\n\t_ = run(tmuxPath())\n}\n",
			want: []string{"y.go f exec.Command(p) starts a program and isn't on the list (it starts a program a function literal is handed"}},
		{name: "a variable's function, for the tests", more: head + "var later = func(p string) error { return exec.Command(p).Run() }\n\nfunc f() { _ = later(tmuxPath()) }\n",
			want: []string{"y.go later exec.Command(p) starts a program", "y.go f later(tmuxPath()) runs a tmux"}},
		{name: "a renamed import", more: "package desktop\n\nimport run \"os/exec\"\n\nfunc (h *hireTmux) f() { _ = run.Command(h.program).Run() }\n",
			want: []string{"y.go f exec.Command(h.program) runs a tmux"}},
		{name: "a dot import", more: "package desktop\n\nimport . \"os/exec\"\n\nfunc f() { _ = Command(tmuxPath()).Run() }\n",
			want: []string{"y.go its imports import .(os/exec): a dot import"}},
		{name: "an exec.Cmd written out", more: head + "func (h *hireTmux) f() { _ = (&exec.Cmd{Path: h.program, Args: []string{\"tmux\"}}).Run() }\n",
			want: []string{"y.go f exec.Cmd{Path}(h.program) runs a tmux"}},
		{name: "a Path set on a command", more: head + "func (h *hireTmux) f() {\n\tcmd := exec.Command(\"true\")\n\tcmd.Path = h.program\n\t_ = cmd.Run()\n}\n",
			want: []string{"y.go f cmd.Path =(h.program) runs a tmux"}},
		{name: "syscall.Exec", more: head + "func (h *hireTmux) f() { _ = syscall.Exec(h.program, nil, nil) }\n",
			want: []string{"y.go f syscall.Exec(h.program) runs a tmux"}},
		{name: "syscall.ForkExec", more: head + "func (h *hireTmux) f() { _, _ = syscall.ForkExec(h.program, nil, nil) }\n",
			want: []string{"y.go f syscall.ForkExec(h.program) runs a tmux"}},
		{name: "os.StartProcess", more: head + "func (h *hireTmux) f() { _, _ = os.StartProcess(h.program, nil, nil) }\n",
			want: []string{"y.go f os.StartProcess(h.program) runs a tmux"}},
		{name: "a pseudo-terminal's Command", more: "package desktop\n\nimport \"github.com/aymanbagabas/go-pty\"\n\nfunc (h *hireTmux) f(terminal pty.Pty) { _ = terminal.Command(h.program, \"attach\") }\n",
			want: []string{"y.go f pty.Command(h.program) runs a tmux"}},
		{name: "a shell told to", more: head + "func f() { _ = exec.Command(\"sh\", \"-c\", \"tmux -S /x kill-server\").Run() }\n",
			want: []string{`y.go f exec.Command("sh") runs a tmux`}},
		{name: "a shell wrapper", more: head + "func shell(script string) error { return exec.Command(\"/bin/sh\", \"-c\", script).Run() }\n\nfunc f() { _ = shell(tmuxPath() + \" kill-server\") }\n",
			want: []string{`y.go f shell(tmuxPath() + " kill-server") runs a tmux`}},
		{name: "a program by another name, in a tmux function", more: head + "func runTmuxAgain(p string) { _ = exec.Command(p).Run() }\n",
			want: []string{"y.go runTmuxAgain exec.Command(p) runs a tmux"}},
		{name: "a ps, harmless but not listed", more: head + "func g() { _ = exec.Command(\"ps\").Run() }\n",
			want: []string{`y.go g exec.Command("ps") starts a program and isn't on the list`}},
		{name: "one more of a listed site", edit: func(s string) string {
			return strings.Replace(s, `return terminalCommand("ps", "-p", "1").Run()`, `_ = terminalCommand("ps", "-p", "1").Run(); return terminalCommand("ps", "-p", "1").Run()`, 1)
		}, want: []string{`x.go status terminalCommand("ps") #2 starts a program and isn't on the list`}},
		{name: "a listed site gone", edit: func(s string) string {
			return strings.Replace(s, `return terminalCommand("ps", "-p", "1").Run()`, `return nil`, 1)
		}, want: []string{`x.go status terminalCommand("ps") is listed but isn't there now`}},
		// RT126SAM7R2 B1′: the start-only runner, any way but its start's one call.
		{name: "B1′: a call through the start-only runner the desk holds (the reviewer's window)", more: head + `// window opens a second window in the desk's session, beside its first.
func (h *hireTmux) window(folder string) error {
	_, _, err := h.startOnly(h.command("new-window", "-d", "-c", folder))
	return err
}
`, want: []string{`y.go window .startOnly(h.command("new-window", "-d", "-c", folder)) starts a program and isn't on the list`,
			`y.go window .startOnly(h.command("new-window", "-d", "-c", folder)) runs a tmux outside`}},
		{name: "B1′, listed as something else", more: head + "func (h *hireTmux) window() { _, _, _ = h.startOnly(h.command(\"new-window\")) }\n",
			list: map[string]string{`y.go window .startOnly(h.command("new-window"))`: "a window, honest"},
			want: []string{`y.go window .startOnly(h.command("new-window")) runs a tmux outside`}},
		{name: "B1′: one more call in the start itself", edit: func(s string) string {
			return strings.Replace(s, "return err\n}", "_, _, _ = h.startOnly(h.command(\"kill-server\"))\n\treturn err\n}", 1)
		}, want: []string{`x.go start .startOnly(h.command("kill-server")) starts a program and isn't on the list`,
			`x.go start .startOnly(h.command("kill-server")) runs a tmux outside`}},
		{name: "B1′: the runner kept in a local first", more: head + "func (h *hireTmux) f() {\n\trun := h.startOnly\n\t_, _, _ = run(h.command(\"capture-pane\"))\n}\n",
			want: []string{"y.go f names .startOnly(…) starts a program and isn't on the list"}},
		{name: "B1′: handed to a parameter", more: head + "func peek(s tmuxStarter, args []string) { _, _, _ = s(args) }\n",
			want: []string{"y.go peek names tmuxStarter(…) starts a program and isn't on the list"}},
		{name: "B1′: held by a function type spelled out", more: head + "func (h *hireTmux) f() {\n\tvar keep func([]string) (string, int, error)\n\t_ = keep\n}\n",
			want: []string{"y.go f names " + tmuxStarterShape + "(…) starts a program and isn't on the list"}},
		{name: "B1′: a field of another type", more: head + "type spare struct{ again tmuxStarter }\n\nfunc (s spare) f() { _, _, _ = s.again(nil) }\n",
			want: []string{"y.go spare names tmuxStarter(…) starts a program and isn't on the list", "y.go f .again(nil) runs a tmux outside"}},
		{name: "B1′: a method that hands it out", more: head + "func (h *hireTmux) runner() tmuxStarter { return h.startOnly }\n",
			want: []string{"y.go runner names tmuxStarter(…) starts", "y.go runner names .startOnly(…) starts"}},
		{name: "B1′: one made and called at once", more: head + "func (h *hireTmux) f() { _, _, _ = startingTmux(tmuxStartCall(h.program))(h.command(\"capture-pane\")) }\n",
			want: []string{`y.go f startingTmux(tmuxStartCall(h.program)) called(h.command("capture-pane")) runs a tmux outside`}},
		{name: "B1′: kept without onlyStarts", edit: func(s string) string {
			return strings.Replace(s, "h.startOnly = onlyStarts(h.socket, startingTmux(tmuxStartCall(program)))", "h.startOnly = startingTmux(tmuxStartCall(program))", 1)
		}, want: []string{"x.go newHireTmux sets .startOnly(startingTmux(tmuxStartCall(program))): a start-only runner kept as onlyStarts didn't make it"}},
		{name: "B1′: set in a literal without onlyStarts", more: head + "func desk(p string) *hireTmux { return &hireTmux{program: p, startOnly: startingTmux(tmuxStartCall(p))} }\n",
			list: map[string]string{"y.go desk sets .startOnly(startingTmux(tmuxStartCall(p)))": "listed, still caught"},
			want: []string{"y.go desk sets .startOnly(startingTmux(tmuxStartCall(p))): a start-only runner kept as onlyStarts didn't make it"}},
		{name: "B1′: what tmuxStartCall gives kept as a one-shot runner", more: head + "func keepIt(p string) func([]string) error { return tmuxStartCall(p) }\n",
			list: map[string]string{"y.go keepIt tmuxStartCall(p)": "listed, still caught"},
			want: []string{"y.go keepIt tmuxStartCall(p): what tmuxStartCall gives is kept as a value"}},
		// RT126SAM7R2 D2: a wrapper kept as a value, called later through what holds it.
		{name: "D2: through a field that holds a wrapper (the reviewer's machine.Run)", more: head + `type t3Machine struct {
	Run func(program string, args []string, wait int) error
}

type Hire struct{ TmuxProgram, TmuxSocket string }

func runT3(program string, args []string, wait int) error { return exec.Command(program, args...).Run() }

func defaultT3Machine() t3Machine { return t3Machine{Run: runT3} }

func t3EndOldSession(machine t3Machine, h Hire) {
	_ = machine.Run(h.TmuxProgram, []string{"-S", h.TmuxSocket, "kill-server"}, 5)
}
`, list: map[string]string{"y.go runT3 exec.Command(program)": "T3 Code", "y.go defaultT3Machine names runT3(…)": "T3 Code"},
			want: []string{"y.go t3EndOldSession .Run(h.TmuxProgram) starts a program and isn't on the list", "y.go t3EndOldSession .Run(h.TmuxProgram) runs a tmux outside"}},
		{name: "D2: through a local", more: head + "func (h *hireTmux) f() { run := terminalCommand; _ = run(h.program).Run() }\n",
			list: map[string]string{"y.go f names terminalCommand(…)": "listed"},
			want: []string{"y.go f run(h.program) runs a tmux outside"}},
		{name: "D2: through a parameter it's handed to", more: head + "func use(run func(string, ...string) *exec.Cmd, p string) { _ = run(p).Run() }\n\nfunc (h *hireTmux) f() { use(terminalCommand, h.program) }\n",
			list: map[string]string{"y.go f names terminalCommand(…)": "listed"},
			want: []string{"y.go use run(p) starts a program and isn't on the list", "y.go f use(h.program) runs a tmux outside"}},
		{name: "D2: a field copied to a local", more: head + `type machine struct{ Run func(program string, args []string) error }

func runIt(program string, args []string) error { return exec.Command(program, args...).Run() }

func (h *hireTmux) f() {
	m := machine{Run: runIt}
	run := m.Run
	_ = run(h.program, nil)
}
`, list: map[string]string{"y.go runIt exec.Command(program)": "listed", "y.go f names runIt(…)": "listed"},
			want: []string{"y.go f run(h.program) runs a tmux outside"}},
		{name: "D2: a holder handed out where the reading stops", more: head + `type machine struct{ Run func(program string, args []string) error }

func runIt(program string, args []string) error { return exec.Command(program, args...).Run() }

func newMachine() machine { return machine{Run: runIt} }

func later(m machine) func(string, []string) error { return m.Run }
`, list: map[string]string{"y.go runIt exec.Command(program)": "listed", "y.go newMachine names runIt(…)": "listed"},
			want: []string{"y.go later names .Run(…) starts a program and isn't on the list (it holds a function that starts what it's handed"}},
		{name: "D2: a call of another thing by a holder's name isn't one", more: head + `type machine struct{ Run func(program string, args []string) error }

func runIt(program string, args []string) error { return exec.Command(program, args...).Run() }

func newMachine() machine { return machine{Run: runIt} }

type other struct{ Run func(name string, values []string) error }

func g(o other) error {
	_ = o.Run("x", nil) // another type's Run, of the same count
	return exec.Command("ps").Run()
}
`, list: map[string]string{"y.go runIt exec.Command(program)": "listed", "y.go newMachine names runIt(…)": "listed", `y.go g exec.Command("ps")`: "ps"},
			want: nil},
		{name: "a method wrapper taken as a value (RT126SAM7R2 follow-up 5)", more: head + "func (h *hireTmux) exec(p string) error { return exec.Command(p).Run() }\n\nfunc (h *hireTmux) f() { run := h.exec; _ = run(h.program) }\n",
			list: map[string]string{"y.go exec exec.Command(p)": "listed"},
			want: []string{"y.go f names .exec(…) starts a program and isn't on the list", "y.go f run(h.program) runs a tmux outside"}},
		// RT126SAM7R2 D3: a wrapper in a package inside the core's folder.
		{name: "D3: a wrapper in desktop/deskexec", sub: "package deskexec\n\nimport \"os/exec\"\n\n// Output runs program.\nfunc Output(program string, args ...string) ([]byte, error) { return exec.Command(program, args...).Output() }\n",
			more: "package desktop\n\nimport \"github.com/Sauzze/smolcorp/desktop/deskexec\"\n\nfunc (h *hireTmux) snap() { _, _ = deskexec.Output(h.program, \"-S\", h.socket, \"capture-pane\", \"-p\") }\n",
			want: []string{"deskexec/deskexec.go Output exec.Command(program) starts a program and isn't on the list", "y.go snap deskexec.Output(h.program) runs a tmux outside"}},
		{name: "D3: a tmux exec inside an internal package, its program picked there",
			sub:  "package runner\n\nimport \"os/exec\"\n\nfunc tmuxBinary() string { return \"/opt/tmux\" }\n\n// Snap runs a tmux it picks itself.\nfunc Snap() error { return exec.Command(tmuxBinary(), \"capture-pane\").Run() }\n",
			more: "package desktop\n\nimport \"github.com/Sauzze/smolcorp/internal/runner\"\n\nfunc snap() error { return runner.Snap() }\n",
			want: []string{"internal/runner/more.go Snap exec.Command(tmuxBinary()) starts a program and isn't on the list",
				"internal/runner/more.go Snap exec.Command(tmuxBinary()) runs a tmux outside"}},
		{name: "a site listed as a tmux where none may run (RT126SAM7R2 follow-up 4)", more: head + "func g() { _ = exec.Command(\"ps\").Run() }\n",
			list: map[string]string{`y.go g exec.Command("ps")`: "tmux, honest"},
			want: []string{`y.go g exec.Command("ps") is listed as running a tmux`}},
	} {
		edit := c.edit
		if edit == nil {
			edit = same
		}
		problems := tmuxRowProblems(t, edit, c.more, c.sub, c.list)
		if c.want == nil && len(problems) != 0 {
			t.Errorf("%s: %q", c.name, problems)
		}
		for _, want := range c.want {
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) }) {
				t.Errorf("%s: no %q in %q", c.name, want, problems)
			}
		}
	}
	// Every package under desktop and internal is read, one inside another too, but no testdata (RT126SAM7R2 D3).
	root := t.TempDir()
	for _, name := range []string{"desktop/x.go", "desktop/deskexec/d.go", "desktop/testdata/t.go", "desktop/tests/o_test.go",
		"internal/a/a.go", "internal/a/b/b.go", "internal/_old/s.go", "internal/.hidden/h.go"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := slices.Sorted(maps.Keys(execPackageDirs(t, root)))
	if want := []string{corePackage, corePackage + "/deskexec", modulePath + "/internal/a", modulePath + "/internal/a/b"}; !slices.Equal(read, want) {
		t.Errorf("the packages read: %q, not %q", read, want)
	}
	// A picker kept for later is a use of it, in its function.
	if picks, err := tmuxPicks("x.go", []byte("package desktop\nfunc g() { p := recordedDeskTmux; _ = p }")); err != nil || !slices.Equal(picks, []string{"x.go g"}) {
		t.Errorf("a picker kept for later: %q %v", picks, err)
	}
}

// A respawned pane (RT126SG2R2 N2) and a pid used again (N1): the desk's row is marked, once, with why; a desk whose
// pane runs the process its start made, one whose tmux can't say, and one with nothing recorded, are not.
func TestARespawnedPaneOrAPidUsedAgainMarksTheDeskUnguarded(t *testing.T) {
	_, _, sandboxed, old := tmuxGateRig(t)
	saved := paneStartedAt
	t.Cleanup(func() { paneStartedAt = saved })
	started := int64(1000)
	paneStartedAt = func(pid int) int64 {
		if pid != 4242 {
			t.Errorf("asked when %d started", pid)
		}
		return started
	}
	// The stand-in's pane runs pid 4242.
	same, respawned, recycled, unknown, nothing := sandboxed, old, sandboxed, old, old
	same.TmuxPaneStarted = 1000
	respawned.TmuxPane = 4100
	recycled.ID, recycled.TmuxSocket, recycled.TmuxPaneStarted = "ab34", deskSocketPath("ab34", 0), 900
	unknown.ID, unknown.TmuxSocket, unknown.TmuxPane = "cd56", filepath.Join(hireSocketFolder(), "smolcorp-cd56"), 4100 // no server there
	nothing.ID, nothing.TmuxSocket, nothing.TmuxPane = "cd78", filepath.Join(hireSocketFolder(), "smolcorp-cd78"), 0
	listenIn(t, recycled.TmuxSocket)
	listenIn(t, nothing.TmuxSocket)
	writeRegistryHires(t, same, respawned, recycled, unknown, nothing)
	if why := paneRespawned(same, nil); why != "" {
		t.Errorf("the same process: %q", why)
	}
	if why := paneRespawned(respawned, nil); !strings.Contains(why, "pid 4242, not 4100") {
		t.Errorf("a respawned pane: %q", why)
	}
	if why := paneRespawned(recycled, nil); !strings.Contains(why, "used again") {
		t.Errorf("a pid used again: %q", why)
	}
	started = 0 // when it started can't be read: the pid alone is compared
	if why := paneRespawned(recycled, nil); why != "" {
		t.Errorf("an unread start time: %q", why)
	}
	started = 1000
	markRespawned(nil)
	marked := map[string]string{}
	for _, h := range registered(t) {
		marked[h.ID] = h.Unguarded
	}
	if marked[same.ID] != "" || marked[unknown.ID] != "" || marked[nothing.ID] != "" || marked[respawned.ID] == "" || marked[recycled.ID] == "" {
		t.Errorf("marked %q", marked)
	}
	// The mark takes authority away: a copy of the registry from before it no longer fits (records_registry.go).
	if unmarked := (registryFile{hires: []Hire{respawned}}); !registryTightens(unmarked, registryFile{hires: []Hire{{ID: respawned.ID, Role: respawned.Role, State: respawned.State, Unguarded: "x"}}}) {
		t.Error("marking a desk unguarded doesn't tighten the registry")
	}
	// Once: a marked row isn't looked at again, and one started again meanwhile keeps its own.
	before := registered(t)
	markRespawned(nil)
	if !slices.EqualFunc(before, registered(t), func(a, b Hire) bool { return a == b }) {
		t.Error("a second round changed the rows")
	}
}

// A mark that can't be saved (the records can't be written) is said once, not at every round; once it's saved, it's
// said again, with the mark (RT126SAM7 nit 3). The look at each pane has a short wait of its own (nit 4).
func TestARespawnedPaneIsSaidOnceWhileItsMarkCantBeSaved(t *testing.T) {
	_, _, _, old := tmuxGateRig(t)
	respawned := old
	respawned.TmuxPane = 4100 // the stand-in's pane runs 4242
	writeRegistryHires(t, respawned)
	t.Cleanup(respawnUnsaved.Clear)
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	folder := filepath.Dir(registryPath())
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(folder, 0o700) })
	markRespawned(nil)
	markRespawned(nil)
	if n := strings.Count(logged.String(), "couldn't mark it"); n != 1 || strings.Count(logged.String(), "pid 4242, not 4100") != 1 {
		t.Fatalf("said %d times while it can't be saved: %s", n, logged.String())
	}
	if err := os.Chmod(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	logged.Reset()
	markRespawned(nil)
	if got := registered(t); len(got) != 1 || got[0].Unguarded == "" || strings.Count(logged.String(), "pid 4242, not 4100") != 1 ||
		strings.Contains(logged.String(), "couldn't") {
		t.Fatalf("once saved: %+v %s", got, logged.String())
	}
	if respawnCheckWait >= tmuxCallWait {
		t.Errorf("the respawn look waits %v, as long as any call", respawnCheckWait)
	}
}

// A plain hire's session whose window didn't open is ended with its server, on the socket it was made on, its own
// (SA-M10; RT126SAM7 nit 1): never the owner's default server, and never by a session's name.
func TestAPlainHiresSessionIsEndedOnTheSocketItWasMadeOn(t *testing.T) {
	_, log, _, _ := tmuxGateRig(t)
	noWindow := hireOpener{open: func(string, []string) (string, error) { return "", errors.New("no window") }}
	if err := openHire([]string{"/bin/echo", "hi"}, t.TempDir(), noWindow, true, true); err == nil {
		t.Fatal("a window opened")
	}
	runs := tmuxGateRuns(t, log, 0)
	own := "-S " + filepath.Join(hireSocketFolder(), hireSocketPrefix+"plain-")
	if len(runs) != 2 || !strings.HasPrefix(runs[0].args, "-f /dev/null "+own) || !strings.Contains(runs[0].args, "new-session") ||
		!strings.HasPrefix(runs[1].args, "-f /dev/null "+own) || !strings.HasSuffix(runs[1].args, " kill-server") ||
		strings.Count(runs[0].args, own) != 1 || runs[0].args[:strings.Index(runs[0].args, " start-server")] != runs[1].args[:strings.Index(runs[1].args, " kill-server")] {
		t.Fatalf("runs %+v", runs)
	}
}

// A desk's start records its pane's process and when that started (Hand back's resume: desksandbox_handback_test.go).
func TestADesksStartRecordsItsPanesProcess(t *testing.T) {
	r := newEngineRig(t)
	saved := paneStartedAt
	t.Cleanup(func() { paneStartedAt = saved })
	paneStartedAt = func(pid int) int64 { return int64(pid) * 10 }
	hire, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	if got := registered(t); len(got) != 1 || got[0].TmuxPane != 4242 || got[0].TmuxPaneStarted != 42420 || hire.TmuxPane != 4242 {
		t.Fatalf("rows %+v", got)
	}
}

// An old desk, from before the sandbox, keeps its recorded tmux (a stock one, say, never swapped for the bundled one)
// and counts as outside the sandbox: its calls are told "unchecked", and its socket isn't a sandboxed desk's.
func TestAnOldDeskKeepsItsRecordedTmuxAndStaysUnsandboxed(t *testing.T) {
	_, _, _, old := tmuxGateRig(t)
	stock, log := tmuxGateStandIn(t)
	old.TmuxProgram = stock
	writeRegistryHires(t, old)
	tmux := hireOwnTmux(old, 0, nil)
	if tmux == nil || tmux.sandboxed || tmux.program != physical(stock) || tmux.guard() != tmuxUnchecked {
		t.Fatalf("%+v", tmux)
	}
	if program, guard := tmuxFor(old.TmuxSocket); program != stock || guard != tmuxUnchecked {
		t.Errorf("its tab: %q %q", program, guard)
	}
	tmux.alive()
	if runs := tmuxGateRuns(t, log, 0); len(runs) != 1 || runs[0].guard != tmuxUnchecked {
		t.Errorf("its call ran on %+v", runs)
	}
	if deskSocketID(old.TmuxSocket, 0) != "" {
		t.Error("an old desk's socket reads as a sandboxed desk's")
	}
}

// The team grid (SA-M7): no start script; its server reads no start file and is started with its guard off for that
// call alone; every later call is told the pid the start printed; its window opens on a window's script.
func TestTheTeamGridStartsWithNoStartScriptAndChecksItsServer(t *testing.T) {
	_, log, sandboxed, old := tmuxGateRig(t)
	var asked bridgeRequest
	opened := showTeamGrid(hostAnswering(func(r bridgeRequest) (bridgeAnswer, error) {
		asked = r
		return bridgeAnswer{OK: true}, nil
	}), []string{sandboxed.ID, old.ID}, "terminal", 0)
	if !opened {
		t.Fatal("the grid didn't open")
	}
	runs := tmuxGateRuns(t, log, 0)
	var start *tmuxGateRun
	for i, run := range runs {
		if strings.Contains(run.args, "new-session") {
			start = &runs[i]
		} else if start != nil && run.guard != "5252" {
			t.Errorf("a call after the start told %q: %q", run.guard, run.args)
		}
	}
	if start == nil || start.guard != "off" || !strings.HasPrefix(start.args, "-f /dev/null -S ") ||
		!strings.Contains(start.args, "set-environment -gu "+tmuxGuardVariable+" ;") {
		t.Fatalf("its start: %+v", start)
	}
	if asked.Action != "open" || !strings.HasPrefix(filepath.Base(asked.Script), "window-") {
		t.Fatalf("asked %+v", asked)
	}
	script, err := os.ReadFile(asked.Script)
	// Its window, and each pane, run `smolcorp attach` with exec (SA-M8): the window told the grid's server.
	if err != nil || !strings.Contains(string(script), "exec '"+commandLineProgram()+"' 'attach' '--team-grid' '5252'\n") {
		t.Errorf("its window's script: %s %v", script, err)
	}
	if !strings.Contains(start.args, "exec '"+commandLineProgram()+"' attach --read-only 'ab12'") ||
		!slices.ContainsFunc(runs, func(r tmuxGateRun) bool {
			return strings.Contains(r.args, "split-window") && strings.Contains(r.args, "attach --read-only 'cd34'")
		}) {
		t.Errorf("its panes: %+v", runs)
	}
	left, _ := filepath.Glob(filepath.Join(supportDir, "hires", "*", "hire-*.sh"))
	if more, _ := filepath.Glob(filepath.Join(supportDir, "hires", "hire-*.sh")); len(left)+len(more) > 0 {
		t.Errorf("a start script: %q %q", left, more)
	}
}
