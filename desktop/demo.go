package desktop

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
)

// smolcorp demo: a real General Manager flow on a small sample project, ready to
// watch or record. It copies the bundled sample (demosample/, MIT) into a new
// folder of its own, uses the office (starting Smolcorp if it isn't running),
// asks before it spends anything, then hires a General Manager on the owner's
// own Claude Code account. The GM plans, hires one agent per task on its own
// branch, checks their proof, gets each change reviewed, and stops before
// anything ships, while the office shows the team at work.
//
// Everything stays in the demo folder: the project is <folder>/app, the hires'
// worktrees <folder>/app-hires (the office makes them beside the project). The
// project has no remote and a pre-push hook that refuses, and its committed
// .claude/settings.json runs the demo's fence (demofence.go) for every Claude
// Code session in it, the GM's and each hire's.
//
//	smolcorp demo                 the real thing, after your OK
//	smolcorp demo --dry-run       the same steps with stand-ins: no office, no Claude Code, nothing spent
//	smolcorp demo --in FOLDER     make the demo folder in FOLDER (default: your home folder)
//	smolcorp demo --no-open       print the link to the demo's page instead of opening it
//
// The demo never hires: the tool's key it holds can't (the route table). It gets that key by asking the app, once you
// click Allow in Smolcorp's window (demooffice.go); no file holds it. After your yes it makes the folder,
// and Smolcorp opens Setup in your browser at the demo (demopage.go): you check it there and click Hire, which
// hires the General Manager held to the project. The demo then follows the team.

//go:embed demosample
var demoSample embed.FS

// The project's folder in the demo folder, and the file the GM ends the demo with.
const (
	demoRepoName   = "app"
	demoReportName = "DEMO-REPORT.md"
	demoFolderName = "smolcorp-demo"
)

// demoAfterInit runs on the new project right after `git init`: the tests stop its signing there.
var demoAfterInit = func(repo string) {}

// demoFolder is a demo's folder, its project, and the project's first commit.
type demoFolder struct {
	root, repo, start string
}

func (f demoFolder) report() string { return filepath.Join(f.root, demoReportName) }

// demoTeam is who does the work: the office with real Claude Code, or the dry run's stand-ins.
type demoTeam interface {
	// ready checks before anything is made, and says which roles the GM may hire.
	ready(out io.Writer) (roles []string, err error)
	// work runs the flow in folder f until the GM's report, or until ctx ends.
	work(ctx context.Context, f demoFolder, roles []string, out io.Writer) error
	// off is the switches the demo would turn on, read by ready: the question names them.
	off() []demoSwitch
	// leftOn is the switches the demo turned on and left on, for the last word of the run.
	leftOn() []demoSwitch
}

// demoSwitch is one of Smolcorp's switches the real demo turns on when it's off.
type demoSwitch struct {
	name, route, setting string // as the demo says it; its route; its name in Setup's Hires and roles
}

var demoSwitches = []demoSwitch{
	{"Smolcorp's manager", "/setup/gm/manager", "Let a General Manager run your team"},
	{"hiring", "/setup/hiring", "Let the office hire and fire"},
}

// demoSwitchNames is "Smolcorp's manager and hiring", and whether there's more than one.
func demoSwitchNames(switches []demoSwitch, suffix string) (string, bool) {
	var names []string
	for _, s := range switches {
		names = append(names, s.name+suffix)
	}
	return strings.Join(names, " and "), len(switches) > 1
}

// demoSwitchesOff is the question's sentence on the switches that are off: which, and that they stay on.
func demoSwitchesOff(off []demoSwitch) string {
	if len(off) == 0 {
		return "Smolcorp's manager and hiring are on already."
	}
	names, many := demoSwitchNames(off, " is off")
	it, stays := "it", "it stays"
	if many {
		it, stays = "them", "they stay"
	}
	return strings.ToUpper(names[:1]) + names[1:] + ": the demo's page turns " + it + " on when you click Hire, and " + stays + " on until you turn " + it + " off in Setup."
}

// demoSwitchesLeftOn is the run's last word on the switches it turned on: they're still on, and where to turn them off.
func demoSwitchesLeftOn(on []demoSwitch) string {
	names, many := demoSwitchNames(on, "")
	them, settings := "it", []string{}
	if many {
		them = "them"
	}
	for _, s := range on {
		settings = append(settings, s.setting)
	}
	return fmt.Sprintf("The demo's page turned on %s, and left %s on for the team. Once you've fired the demo's team, turn %s off in Setup, Hires and roles: %s.",
		names, them, them, strings.Join(settings, "; "))
}

// RunDemo is `smolcorp demo`.
func RunDemo(args []string) int {
	if len(args) > 0 && args[0] == "fence" {
		return runDemoFence(args[1:], os.Stdin, os.Stdout)
	}
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "stand-ins for the office and Claude Code: nothing is spent")
	in := fs.String("in", homeDir, "where to make the demo folder")
	noOpen := fs.Bool("no-open", false, "print the link to the demo's page instead of opening it in your browser")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "smolcorp demo takes no argument %q (smolcorp demo --help)\n", fs.Arg(0))
		return 2
	}
	office := newOfficeTeam()
	office.noOpen = *noOpen
	var team demoTeam = office
	if *dry {
		team = standInTeam{}
	}
	interrupt := func(ctx context.Context) (context.Context, context.CancelFunc) {
		return signal.NotifyContext(ctx, os.Interrupt)
	}
	return runDemo(context.Background(), demoRun{in: *in, dry: *dry, team: team, stdin: os.Stdin, out: os.Stdout, interrupt: interrupt})
}

// demoRun is one run of the demo.
type demoRun struct {
	in    string
	dry   bool
	team  demoTeam
	stdin io.Reader
	out   io.Writer
	// interrupt makes Ctrl-C end ctx, once the question is answered (before, Ctrl-C ends the demo as any program).
	interrupt func(context.Context) (context.Context, context.CancelFunc)
}

func runDemo(ctx context.Context, r demoRun) int {
	out := r.out
	if r.dry {
		fmt.Fprintln(out, "Smolcorp demo, dry run: stand-ins play the office and Claude Code. Nothing is spent, nothing leaves the demo folder.")
	} else {
		fmt.Fprintln(out, "Smolcorp demo: a General Manager runs a small team on a sample project, in a new folder of its own.")
	}
	roles, err := r.team.ready(out)
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	if !r.dry {
		fmt.Fprintf(out, `
This starts real Claude Code sessions on your default Claude account (or the
one you pick on the demo's page; the hires work on the same one): a General
Manager (Opus) and about four hires, for some minutes each. That uses your
plan's usage, as any Claude Code session does, never an API key.
%s
Nothing is pushed, merged or published: the project has no remote, and the
team stops before anything ships.

After your yes, the demo makes its folder, and Setup opens in your browser at
the demo: check it there, and click Hire to hire the General Manager.

Go on? [y/N] `, demoSwitchesOff(r.team.off()))
		answer, _ := bufio.NewReader(r.stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			fmt.Fprintln(out, "Nothing was made or started.")
			return 1
		}
	}
	if r.interrupt != nil {
		var stop context.CancelFunc
		ctx, stop = r.interrupt(ctx)
		defer stop()
	}
	f, err := makeDemoFolder(r.in, !r.dry, roles)
	if err != nil {
		fmt.Fprintln(out, "The demo folder couldn't be made:", err)
		return 1
	}
	fmt.Fprintf(out, "\nThe demo folder: %s\nThe project: %s (no remote; its own fence for every hire)\n\n", f.root, f.repo)
	code := 1
	if err := r.team.work(ctx, f, roles, out); err != nil {
		fmt.Fprintln(out, err)
		if errors.Is(err, context.Canceled) {
			code = demoShipped(f, out)
		}
	} else {
		code = demoShipped(f, out)
	}
	// At the end, whichever: the report, Ctrl-C, Smolcorp gone.
	if on := r.team.leftOn(); len(on) > 0 {
		fmt.Fprintln(out, demoSwitchesLeftOn(on))
	}
	return code
}

// makeDemoFolder makes a new demo folder in parent (smolcorp-demo, else -2, -3…), never one already there,
// with the sample project in it. A real run's must be one the office hires into: inside your home folder.
func makeDemoFolder(parent string, real bool, roles []string) (demoFolder, error) {
	parent = resolveExisting(parent)
	if real && parent != resolveExisting(homeDir) {
		// Checked before anything is made there.
		if _, _, why := checkedSetupFolder(parent, homeDir); why != "" {
			return demoFolder{}, errors.New(why)
		}
	}
	var root string
	for n := 1; n < 100 && root == ""; n++ {
		name := demoFolderName
		if n > 1 {
			name += "-" + strconv.Itoa(n)
		}
		if err := os.Mkdir(filepath.Join(parent, name), 0o755); err == nil {
			root = filepath.Join(parent, name)
		} else if !errors.Is(err, fs.ErrExist) {
			return demoFolder{}, err
		}
	}
	if root == "" {
		return demoFolder{}, fmt.Errorf("%s already holds 99 demo folders: remove some", parent)
	}
	f := demoFolder{root: root, repo: filepath.Join(root, demoRepoName)}
	if real {
		if _, _, why := checkedSetupFolder(root, homeDir); why != "" {
			os.Remove(root)
			return demoFolder{}, errors.New(why)
		}
	}
	if err := writeDemoProject(f, roles); err != nil {
		return demoFolder{}, err
	}
	start, err := demoGit(f.repo, "rev-parse", "HEAD")
	f.start = start
	return f, err
}

// writeDemoProject copies the sample into the project, adds its fence, and makes it a repo
// with one commit, no remote, and a pre-push hook that refuses.
func writeDemoProject(f demoFolder, roles []string) error {
	err := fs.WalkDir(demoSample, "demosample", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := demoSample.ReadFile(path)
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(path, "demosample/")
		if name == "gitignore" { // embed leaves out dot files
			name = ".gitignore"
		}
		return writeNewFile(filepath.Join(f.repo, filepath.FromSlash(name)), data, 0o644)
	})
	if err != nil {
		return err
	}
	settings, err := demoSettings(f.root, roles)
	if err != nil {
		return err
	}
	if err := writeNewFile(filepath.Join(f.repo, ".claude", "settings.json"), settings, 0o644); err != nil {
		return err
	}
	if _, err := demoGit(f.repo, "init", "-q"); err != nil {
		return err
	}
	demoAfterInit(f.repo)
	hooks := filepath.Join(f.repo, ".git", "hooks")
	prePush := "#!/bin/sh\necho 'The Smolcorp demo never pushes: this project has no remote, on purpose.' >&2\nexit 1\n"
	if err := writeNewFile(filepath.Join(hooks, "pre-push"), []byte(prePush), 0o755); err != nil {
		return err
	}
	steps := [][]string{
		{"symbolic-ref", "HEAD", "refs/heads/main"},
		// Its own hooks folder, whatever core.hooksPath says elsewhere: the pre-push hook always runs.
		{"config", "--local", "core.hooksPath", hooks},
	}
	if email, _ := demoGit(f.repo, "config", "user.email"); email == "" {
		// No name of yours to commit with: the project's own, so the hires' commits work too.
		steps = append(steps, []string{"config", "--local", "user.name", "Smolcorp demo"},
			[]string{"config", "--local", "user.email", "demo@smolcorp.invalid"})
	}
	steps = append(steps, []string{"add", "-A"}, []string{"commit", "-q", "-m", "The sample project for Smolcorp's demo"})
	for _, step := range steps {
		if _, err := demoGit(f.repo, step...); err != nil {
			return err
		}
	}
	return nil
}

// demoSettings is the project's .claude/settings.json: the demo's fence before every tool call,
// and deny rules for the plain forms of what it refuses.
func demoSettings(root string, roles []string) ([]byte, error) {
	program := commandLineProgram() // links resolved; on Windows, smolcorp-cli.exe
	if program == "" {
		return nil, errors.New("couldn't find smolcorp's own program for the demo's fence")
	}
	command := fmt.Sprintf("%s demo fence --root %s --roles %s", shellPath(program), shellPath(root), shellPath(strings.Join(roles, ",")))
	return json.MarshalIndent(map[string]any{
		// PowerShell too: the demo works in Bash, and the fence can't read PowerShell's words (it refuses them).
		"permissions": map[string]any{"deny": []string{"Bash(git push:*)", "Bash(git remote add:*)", "Bash(gh:*)", "PowerShell"}},
		"hooks": map[string]any{"PreToolUse": []any{map[string]any{
			"matcher": "*",
			"hooks":   []any{map[string]any{"type": "command", "command": command, "timeout": 10}},
		}}},
	}, "", "  ")
}

// writeNewFile writes a file that must not be there yet, making its folders.
func writeNewFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// demoGit runs git in dir: its output, or an error that says which step failed.
func demoGit(dir string, args ...string) (string, error) {
	status, out, ok := runGit(dir, args...)
	switch {
	case !ok:
		return "", fmt.Errorf("git %s didn't answer", args[0])
	case status != 0 && !(len(args) == 2 && args[0] == "config"): // a config value that isn't set is no error
		return "", fmt.Errorf("git %s: %s", args[0], firstRunes(out, 300))
	case status != 0:
		return "", nil
	}
	return out, nil
}

// demoShipped checks and says that nothing shipped: no remote, main where the demo left it, and
// the branches waiting for you. 0 when so, 1 when not.
func demoShipped(f demoFolder, out io.Writer) int {
	remotes, err := demoGit(f.repo, "remote")
	main, err2 := demoGit(f.repo, "rev-parse", "refs/heads/main")
	branches, err3 := demoGit(f.repo, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err := errors.Join(err, err2, err3); err != nil {
		fmt.Fprintln(out, "Couldn't check the project:", err)
		return 1
	}
	if remotes != "" {
		fmt.Fprintf(out, "\nThe project has a remote now (%s): check it before anything else.\n", strings.Join(strings.Fields(remotes), ", "))
		return 1
	}
	if main != f.start {
		fmt.Fprintln(out, "\nmain has moved since the demo began: something was merged. Look at `git log main` in", f.repo)
		return 1
	}
	var waiting []string
	for _, b := range strings.Fields(branches) {
		if b != "main" {
			waiting = append(waiting, b)
		}
	}
	fmt.Fprintf(out, "\nNothing shipped: the project has no remote, and main is where the demo left it (%s).\n", firstRunes(f.start, 7))
	if len(waiting) > 0 {
		fmt.Fprintf(out, "Waiting for your OK: %s.\n", strings.Join(waiting, ", "))
	}
	fmt.Fprintf(out, "When you're done, fire the demo's team in the office, then delete %s.\n", f.root)
	return 0
}
