package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/Sauzze/smolcorp/internal/gm"
	"github.com/Sauzze/smolcorp/internal/guard"
)

// The demo's own fence: `smolcorp demo fence --root R --roles a,b`, a PreToolUse
// hook in the demo project's committed .claude/settings.json. Every Claude Code
// session that works in the demo reads that file (the General Manager in the
// project, each hire in its worktree, since a worktree checks out the same file),
// so each runs this before every tool call, on top of the fence Smolcorp gives a
// managed hire. It refuses:
//
//   - changing a file outside the demo folder (Edit, Write, MultiEdit, NotebookEdit),
//     or one in a .claude or .git folder in it, where the fence and the hooks are;
//   - a hire anywhere but the demo's project: `smolcorp gm hire` (or the MCP tool hire)
//     must name as --folder, written plainly, the project itself or the top of one of
//     its worktrees in the demo folder, a folder that is there (the folders that hold
//     this file), and one of the roles given, which are the Claude Code roles, so every
//     hire reads this file too; never into T3 Code, on the API-key account, or on a
//     Claude account but the default;
//   - a hire by any other way (a request to the app's hire routes), and a line that
//     names smolcorp (or gm and a word) and builds words with $ or a backtick;
//   - smolcorp for anything but `smolcorp gm …`, version and help: its MCP server on
//     standard input, `run`, `setup`, `serve` and the rest would hire, or start an agent,
//     where the fence can't see; a word that holds smolcorp but isn't the program or a
//     path in the demo (X=smolcorp, alias h=smolcorp); a command whose name the shell
//     builds ($X mcp);
//   - a send, brief or fire (or contacts) for anyone but a hire the demo made: one whose
//     folder is inside the demo, as the office's team says;
//   - a line or a path that names the setup key: it opens the owner's whole setup;
//   - git config writes, and git checkout or switch: a branch with another fence; a
//     git worktree added outside the demo folder;
//   - whatever the hire guard refuses the GM by default: push, deploy, publish,
//     cloud, remote shells, typing into another session.
//
// Like the guard, it reads one command line and never the scripts it runs: a
// Bash command that writes outside the folder isn't caught here (the office's
// own fence and the owner's permission prompts stay the other layers). The wall
// is the app's: the demo hires its GM held to the project (gmscope.go), so every
// hire, send, brief or fire its token asks for, by whatever way, stays in it.

// demoCall is the part of Claude Code's PreToolUse input the fence reads.
type demoCall struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Cwd       string          `json:"cwd"`
}

// demoFence is what a demo's fence holds its calls to.
type demoFence struct {
	root  string   // the demo folder, resolved
	roles []string // the roles the GM may hire
	home  string
	// team is the office's team as the GM's own token reads it: who a send, brief or fire is for.
	team func() ([]demoMember, error)
	// hireSession: the call is a hire's (its send key is set). The app lets a hire write only to
	// its GM, or to a hire the GM put in its contacts, which this fence checks on the GM's side.
	hireSession bool
	git         gitRunner // nil: git itself
}

// demoFenceRefusal starts every refusal, so the GM and the owner know whose it is.
const demoFenceRefusal = "Refused by the Smolcorp demo's fence: "

// check says why the call is refused, or "" to let it run.
func (f demoFence) check(c demoCall) string {
	var input map[string]any
	_ = json.Unmarshal(c.ToolInput, &input)
	// The setup key opens every setup route of the owner's office: no call of the demo's names it,
	// to read it (Read, Grep, cat) or to send it. Escapes taken away first, as JSON's \u002d.
	if namesSetupKey(c.ToolInput) {
		return demoFenceRefusal + "the setup key is the owner's: the demo never reads or sends it."
	}
	switch name := c.ToolName; {
	case name == "Edit" || name == "Write" || name == "MultiEdit" || name == "NotebookEdit":
		// Every path given is judged: NotebookEdit's own is notebook_path.
		var paths []string
		for _, key := range []string{"file_path", "notebook_path"} {
			if value, has := input[key]; has {
				path, _ := value.(string)
				paths = append(paths, path)
			}
		}
		if len(paths) == 0 {
			return demoFenceRefusal + "the demo only changes files inside " + f.root + "."
		}
		for _, path := range paths {
			if path == "" || !f.inside(path, c.Cwd) {
				return demoFenceRefusal + "the demo only changes files inside " + f.root + "."
			}
			if f.inFenceFolder(path, c.Cwd) {
				return demoFenceRefusal + "the .claude and .git folders hold the demo's fence and hooks: they stay as the demo made them."
			}
		}
	case strings.HasPrefix(name, "mcp__") && strings.HasSuffix(name, "__hire"):
		ask := demoHireAsk{}
		ask.role, _ = input["role"].(string)
		ask.folder, _ = input["folder"].(string)
		if value, has := input["account"]; has {
			account, isText := value.(string)
			ask.account = account
			if !isText {
				ask.account = "?"
			}
		}
		if value, has := input["api-key"]; has && value != nil && value != false {
			ask.apiKey = true
		}
		return f.hire(ask, c.Cwd)
	case strings.HasPrefix(name, "mcp__") && demoTargetTool(name) != "":
		tool := demoTargetTool(name)
		var refs []string
		for _, key := range map[string][]string{"send": {"to"}, "brief": {"hire"}, "fire": {"hire"}, "contacts": {"a", "b"}}[tool] {
			if value, has := input[key]; has {
				ref, isText := value.(string)
				if !isText {
					return demoFenceRefusal + "a " + tool + " it can't read."
				}
				refs = append(refs, ref)
			}
		}
		return f.targets(tool, refs)
	default:
		command, isText := input["command"].(string)
		if _, has := input["command"]; !has && name != "Bash" && name != "PowerShell" {
			return ""
		}
		if !isText {
			return demoFenceRefusal + "a command it can't read."
		}
		if name == "PowerShell" {
			return f.commandRead(command, c.Cwd, false)
		}
		return f.command(command, c.Cwd)
	}
	return ""
}

// command checks a shell command line: what the guard refuses, a hire's folder and role, and no other way to hire.
// It's read as bash would read it too: Claude Code's Bash runs bash, Git Bash on Windows.
func (f demoFence) command(line, cwd string) string { return f.commandRead(line, cwd, true) }

// commandRead is command, with bash's reading (a backslash escapes) or without it: PowerShell, whose escape
// is the backtick, leaves a backslash in a word as it is, so C:\Users\… is the path written.
func (f demoFence) commandRead(line, cwd string, bash bool) string {
	// The GM's profile: the GM hires and fires; a hire's own office fence refuses it that.
	if r := guard.Check(line, guard.ForGM(guard.DefaultGroups())); r != nil {
		return demoFenceRefusal + r.Error()
	}
	// A backslash before a line end joins the lines (bash): `gm hi\⏎re` is gm hire, and a hire
	// written over several lines is one command.
	line = strings.NewReplacer("\\\r\n", "", "\\\n", "").Replace(line)
	if namesHireRoute(line) {
		return demoFenceRefusal + "in the demo, hire with `smolcorp gm hire --folder <a folder in the demo>` only."
	}
	// Read twice, as the guard reads a line: a backslash escapes (bash), or it's part of a path
	// (Windows, C:\Users\…). A hire either reading refuses is refused. PowerShell's line is read the second way only.
	all := demoCommands(line, false, 0)
	if bash {
		all = append(demoCommands(line, true, 0), all...)
	}
	// A command whose name the shell builds ($X mcp, after X=smolcorp in another call) can't be read.
	for _, words := range demoSegments(line, bash) {
		if name := demoCommandName(words); strings.ContainsAny(name, "$`") {
			return demoFenceRefusal + "a command whose name holds $ or a backtick: write the program's name out plainly."
		}
	}
	if shellExpands(line) && slices.ContainsFunc(all, namesSmolcorp) {
		// The shell would build a word the fence can't read: smolcorp $'gm' hire, G=gm; smolcorp $G hire.
		return demoFenceRefusal + "a smolcorp command with $ or a backtick in its line: write it out plainly (text with a $ in single quotes, or in --task-file)."
	}
	for _, words := range all {
		if word := f.smolcorpInAWord(words, cwd); word != "" {
			why := demoFenceRefusal + fmt.Sprintf("%q holds smolcorp: in the demo, smolcorp is the program's name or a path in %s only.", word, f.root)
			if bash && strings.Contains(line, `\`) {
				// C:\Users\… unquoted is C:Users… to bash, Git Bash included: not the folder written.
				why += " In bash, a \\ escapes the letter after it, so C:\\Users reads C:Users: write a Windows path with / (" + demoShellFolder(f.root) + ") or in single quotes."
			}
			return why
		}
		if !demoSmolcorpAllowed(words) {
			return demoFenceRefusal + "in the demo, smolcorp runs `smolcorp gm …` (and version, help) only: its other commands hire or start agents where the fence can't see."
		}
		if why := f.gitRefusal(words, cwd); why != "" {
			return demoFenceRefusal + why
		}
		if tool, refs := demoTargets(words); tool != "" {
			if why := f.targets(tool, refs); why != "" {
				return why
			}
		}
		for i := 0; i+1 < len(words); i++ {
			if words[i] != "gm" || words[i+1] != "hire" {
				continue
			}
			if why := f.hire(demoHireFlags(words[i+2:]), cwd); why != "" {
				return why
			}
		}
	}
	return ""
}

// namesSmolcorp says whether a simple command's words name smolcorp (the program, by any path) or gm and a word.
func namesSmolcorp(words []string) bool {
	for i, word := range words {
		if isSmolcorp(word) || word == "gm" && i+1 < len(words) {
			return true
		}
	}
	return false
}

// isSmolcorp says whether a word names the smolcorp program, by any path, any case (the Mac's disk), .exe or not.
func isSmolcorp(word string) bool {
	name := strings.ToLower(strings.ReplaceAll(word, `\`, "/"))
	name = strings.TrimSuffix(name[strings.LastIndexByte(name, '/')+1:], ".exe")
	return name == "smolcorp" || name == "smolcorp-cli"
}

// demoCommandName is a simple command's program: its first word after any NAME=value.
func demoCommandName(words []string) string {
	for _, word := range words {
		if !demoAssignment.MatchString(word) {
			return word
		}
	}
	return ""
}

var demoAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// smolcorpInAWord is the first word that holds smolcorp (any case) without being the program or a path in the
// demo, "" for none. A NAME=value word (an assignment, an alias) or --flag=value is judged by its value, so
// X=smolcorp and alias h='smolcorp mcp' are refused: the shell could run that value later as smolcorp.
// A word with a blank in it is a line of its own, whose words are judged (demoCommands).
func (f demoFence) smolcorpInAWord(words []string, cwd string) string {
	for _, word := range words {
		if !strings.Contains(strings.ToLower(word), "smolcorp") || strings.ContainsAny(word, " \t\n") || isSmolcorp(word) {
			continue
		}
		value := word
		if _, after, assigned := strings.Cut(word, "="); assigned && (demoAssignment.MatchString(word) || strings.HasPrefix(word, "-")) {
			value = after
		}
		path := value == "~" || strings.HasPrefix(value, "~/") || filepath.IsAbs(value) || strings.ContainsAny(value, `/\`)
		if !path || !plainFolder(value) || !f.inside(value, cwd) {
			return word
		}
	}
	return ""
}

// demoSmolcorpAllowed says whether every smolcorp in a simple command's words is followed by gm, version or help:
// the demo's allowlist. mcp would hire from a file on standard input, run would start an agent in any folder,
// and setup, serve, core, hook, quit, guard, demo or a word smolcorp doesn't know are no work of the demo's.
// smolcorp as an argument (xargs smolcorp mcp, which smolcorp) is judged the same: the safe side.
func demoSmolcorpAllowed(words []string) bool {
	for i, word := range words {
		if isSmolcorp(word) && (i+1 == len(words) || !slices.Contains([]string{"gm", "version", "--version", "help", "--help", "-h"}, words[i+1])) {
			return false
		}
	}
	return true
}

// namesSetupKey says whether a tool call's input names the setup key: its file (setup-key) or its header
// (X-Smolcorp-Setup-Key), any case, JSON's escapes read.
func namesSetupKey(input json.RawMessage) bool {
	var value any
	text := string(input)
	if json.Unmarshal(input, &value) == nil {
		data, _ := json.Marshal(value)
		text = string(data)
	}
	return strings.Contains(strings.ToLower(text), "setup-key")
}

// demoTargetTool is send, brief, fire or contacts for the MCP tools of those names, else "".
func demoTargetTool(name string) string {
	for _, tool := range []string{"send", "brief", "fire", "contacts"} {
		if strings.HasSuffix(name, "__"+tool) {
			return tool
		}
	}
	return ""
}

// demoTargets reads who `gm send`, `gm brief`, `gm fire` and `gm contacts allow|remove` are for: every word
// that isn't one of their flags or a flag's value. A word the CLI would refuse is a target too: the safe side.
func demoTargets(words []string) (tool string, refs []string) {
	valued := map[string][]string{"send": {"text", "file", "id"}, "brief": {"text", "file"}, "fire": {}, "contacts": {}}
	flags := map[string][]string{"send": {"json"}, "brief": {"json"}, "fire": {"json", "force", "remove-worktree"}, "contacts": {"json", "one-way"}}
	for i := 0; i+1 < len(words); i++ {
		name := strings.ToLower(words[i+1])
		if words[i] != "gm" || valued[name] == nil {
			continue
		}
		tool = name
		var plain []string
		args := words[i+2:]
		for j := 0; j < len(args); j++ {
			flag, _, joined := strings.Cut(strings.TrimLeft(args[j], "-"), "=")
			switch {
			case args[j] == "--":
			case strings.HasPrefix(args[j], "-") && slices.Contains(valued[name], flag):
				if !joined {
					j++
				}
			case strings.HasPrefix(args[j], "-") && slices.Contains(flags[name], flag):
			default:
				plain = append(plain, args[j])
			}
		}
		if name == "contacts" && len(plain) > 0 && (plain[0] == "allow" || plain[0] == "remove") {
			plain = plain[1:]
		}
		refs = append(refs, plain...)
	}
	return tool, refs
}

// targets checks that a send, brief, fire or contacts is for the demo's own hires only. Mail to the
// GM goes: the app puts it nowhere but the GM's inbox. Anything the fence can't check is refused.
func (f demoFence) targets(tool string, refs []string) string {
	var team []demoMember
	read := false
	for _, ref := range refs {
		if asked := mailAsked(ref); tool == "send" && (asked == "gm" || asked == "the gm") || tool == "send" && f.hireSession {
			continue
		}
		if !read {
			var err error
			if f.team == nil {
				err = errors.New("no team to ask")
			} else {
				team, err = f.team()
			}
			if err != nil {
				return demoFenceRefusal + fmt.Sprintf("it couldn't read the team to see who %q is (%v).", ref, err)
			}
			read = true
		}
		if !f.demosOwn(ref, team) {
			return demoFenceRefusal + fmt.Sprintf("in the demo, send, brief and fire reach only the hires the demo made, working in %s: %q isn't one of them (see smolcorp gm team).", f.root, ref)
		}
	}
	return ""
}

// demosOwn says whether ref (an id, a name or a nickname, as the app and the CLI find them) names
// only hires of the demo's: hires (not a session alone) whose folder and worktree are the demo's
// project or a worktree of it (checkout).
func (f demoFence) demosOwn(ref string, team []demoMember) bool {
	asked := mailAsked(ref)
	named := func(name string) bool {
		return name != "" && (strings.EqualFold(name, ref) || strings.ToLower(name) == asked)
	}
	found := false
	for _, m := range team {
		if m.ID != ref && m.ID != strings.TrimSpace(ref) && !named(m.Nickname) && !named(m.Name) {
			continue
		}
		found = true
		if !m.Registered || !f.checkout(m.Folder, "") || m.Worktree != "" && !f.checkout(m.Worktree, "") {
			return false
		}
	}
	return found
}

// shellExpands says whether a line holds a $ or a backtick the shell would act on: one outside single quotes.
// A backslash before it doesn't count as keeping it (PowerShell reads \ as a letter); before a quote, it does
// (bash's it\'s), which can only take a quote away and so see more of the line as open.
func shellExpands(line string) bool {
	single, double := false, false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\\' && !single && i+1 < len(line):
			if line[i+1] == '$' || line[i+1] == '`' {
				return true
			}
			i++
		case c == '\'' && !double:
			single = !single
		case c == '"' && !single:
			double = !double
		case (c == '$' || c == '`') && !single:
			return true
		}
	}
	return false
}

// namesHireRoute says whether a line names the app's hire routes, as the server would read the
// path: percent escapes decoded, // and /./ and /x/../ taken away, case aside.
func namesHireRoute(line string) bool {
	for {
		decoded, err := url.PathUnescape(line)
		if err != nil || decoded == line {
			break
		}
		line = decoded
	}
	line = strings.ToLower(strings.ReplaceAll(line, `\`, "/"))
	for {
		squeezed := demoDotSegment.ReplaceAllString(strings.ReplaceAll(strings.ReplaceAll(line, "//", "/"), "/./", "/"), "/")
		if squeezed == line {
			break
		}
		line = squeezed
	}
	return strings.Contains(line, "/setup/hire") || strings.Contains(line, "/setup/gm/hire")
}

var demoDotSegment = regexp.MustCompile(`/[^/\s'"]+/\.\./`)

// gitRefusal says why a git command is refused: a config write (hooks, remotes), a branch switch (a
// branch of the GM's could carry another fence into every hire made after it), or a worktree added
// outside the demo folder (from cwd, or the folder -C names).
func (f demoFence) gitRefusal(words []string, cwd string) string {
	for i, word := range words {
		name := strings.ToLower(strings.ReplaceAll(word, `\`, "/"))
		if strings.TrimSuffix(name[strings.LastIndexByte(name, '/')+1:], ".exe") != "git" {
			continue
		}
		sub, args, from := "", []string(nil), cwd
		for j := i + 1; j < len(words); j++ {
			a := words[j]
			if strings.HasPrefix(a, "-") {
				if !strings.Contains(a, "=") && slices.Contains([]string{"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--config-env"}, a) {
					if a == "-C" && j+1 < len(words) {
						from = demoFrom(words[j+1], from, f.home)
					}
					j++
				}
				continue
			}
			sub, args = strings.ToLower(a), words[j+1:]
			break
		}
		switch {
		case sub == "checkout" || sub == "switch":
			return "the demo doesn't switch branches: each hire has a worktree of its own (git restore puts a file back)."
		case sub == "config" && !gitConfigReads(args):
			return "git config only reads in the demo: the project's hooks and remotes stay as the demo made them."
		case sub == "worktree" && len(args) > 0 && strings.ToLower(args[0]) == "add":
			if path := gitWorktreeAddPath(args[1:]); path == "" || !plainFolder(path) || !f.inside(path, from) {
				return "a worktree of the demo's stays in " + f.root + ": the office makes the hires' own."
			}
		}
	}
	return ""
}

// demoFrom is the folder git -C dir works from, read from where the command runs: "" when it can't be told.
func demoFrom(dir, cwd, home string) string {
	switch {
	case dir == "~" || strings.HasPrefix(dir, "~/"):
		return filepath.Join(home, dir[1:])
	case filepath.IsAbs(dir):
		return dir
	case cwd == "" || !filepath.IsAbs(cwd):
		return ""
	}
	return filepath.Join(cwd, dir)
}

// gitWorktreeAddPath is the folder `git worktree add`'s arguments make: the first that isn't an option or an
// option's value. "" when there's none.
func gitWorktreeAddPath(args []string) string {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-b" || a == "-B" || a == "--reason":
			i++
		case a == "--":
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		case !strings.HasPrefix(a, "-"):
			return a
		}
	}
	return ""
}

// gitConfigReads says whether git config's arguments only read: a get or list, or one name alone.
func gitConfigReads(args []string) bool {
	var plain []string
	reads := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--get" || a == "--get-all" || a == "--get-regexp" || a == "--get-urlmatch" || a == "--get-color" ||
			a == "--get-colorbool" || a == "-l" || a == "--list" || a == "--show-origin" || a == "--show-scope" || a == "--name-only":
			reads = reads || a != "--show-origin" && a != "--show-scope" && a != "--name-only"
		case slices.Contains([]string{"-f", "--file", "--blob", "--type", "--default", "--comment"}, a):
			i++
		case a == "--local" || a == "--global" || a == "--system" || a == "--worktree" || a == "-z" || a == "--null" ||
			a == "--bool" || a == "--int" || a == "--path" || a == "--includes" || a == "--no-includes" || strings.HasPrefix(a, "--type=") ||
			strings.HasPrefix(a, "--file=") || strings.HasPrefix(a, "--default="):
		case strings.HasPrefix(a, "-"):
			return false // --unset, --add, --edit, --rename-section…: a write, or one the fence doesn't know
		default:
			plain = append(plain, a)
		}
	}
	if len(plain) > 0 && (plain[0] == "get" || plain[0] == "list") {
		return true
	}
	return reads || len(plain) == 1
}

// demoHireAsk is what a hire asks for, as the fence reads it.
type demoHireAsk struct {
	role, folder, account string
	t3, apiKey            bool
}

// hire checks one hire: into T3 Code never, nor on the API-key account or a Claude account named (the app fills
// in the GM's own); a role from the list; a folder inside the demo, written as a plain path.
func (f demoFence) hire(ask demoHireAsk, cwd string) string {
	folder := strings.TrimSpace(ask.folder)
	switch {
	case ask.t3:
		return demoFenceRefusal + "the demo hires in terminals, not T3 Code."
	case ask.apiKey:
		return demoFenceRefusal + "the demo hires on your Claude account's plan, never the API-key account: leave out --api-key."
	case ask.account != "" && ask.account != claudeDefaultName:
		return demoFenceRefusal + "the demo's hires work on the General Manager's own Claude account, which Smolcorp fills in: leave out --account."
	case !slices.Contains(f.roles, ask.role):
		return demoFenceRefusal + fmt.Sprintf("the demo hires these roles only, all on Claude Code: %s.", strings.Join(f.roles, ", "))
	case folder == "":
		return demoFenceRefusal + "name the folder: --folder " + demoShellFolder(filepath.Join(f.root, demoRepoName)) + "."
	case !plainFolder(folder):
		// The shell would make it something else ($HOME, ~user, {a,b}, *): the fence can't know what.
		return demoFenceRefusal + "write the folder out plainly, with no $, `, {, *, ?, [ or ~user in it: --folder " + demoShellFolder(filepath.Join(f.root, demoRepoName)) + "."
	case !f.checkout(folder, cwd):
		return demoFenceRefusal + "hire into the demo's project, " + demoShellFolder(filepath.Join(f.root, demoRepoName)) + " (with --worktree), or the top of one of its worktrees: no other folder holds the demo's fence."
	}
	return ""
}

// checkout says whether folder (~ is home, a relative one is from cwd) is the demo's project, or the top of
// a worktree of it in the demo folder: the folders whose .claude/settings.json is the demo's fence. It must be
// there, so a link made in the same line can't stand in for it. The rule is the app's own (gmScope.holds).
func (f demoFence) checkout(folder, cwd string) bool {
	folder = strings.TrimSpace(folder)
	if folder == "~" || strings.HasPrefix(folder, "~/") || strings.HasPrefix(folder, `~\`) {
		folder = filepath.Join(f.home, folder[1:])
	}
	if !filepath.IsAbs(folder) {
		if cwd == "" || !filepath.IsAbs(cwd) || strings.HasPrefix(folder, "/") || strings.HasPrefix(folder, `\`) {
			return false
		}
		folder = filepath.Join(cwd, folder)
	}
	git := f.git
	if git == nil {
		git = runGit
	}
	return f.inside(folder, "") && gmScope{root: filepath.Join(f.root, demoRepoName), git: git}.holds(folder)
}

// plainFolder says whether the shell leaves a folder as written: no expansion in it but ~ or ~/… at its start.
// A ~ inside a name, as in a Windows short name (C:/Users/RUNNER~1/…), is a letter to every shell; one that
// starts a name (~user, app/~x, a=~x, a:~x) is refused, as bash may read it as someone's home.
func plainFolder(folder string) bool {
	if strings.ContainsAny(folder, "$`{*?[") {
		return false
	}
	rest := folder
	if folder == "~" || strings.HasPrefix(folder, "~/") || strings.HasPrefix(folder, `~\`) {
		rest = "x" + folder[1:]
	}
	for i := strings.IndexByte(rest, '~'); i >= 0; i = strings.IndexByte(rest, '~') {
		if i == 0 || strings.IndexByte(`/\=:`, rest[i-1]) >= 0 {
			return false
		}
		rest = rest[i+1:]
	}
	return true
}

// demoShellFolder is a folder as the demo writes it in a command for the GM: as it is, and on Windows with /,
// which bash (Claude Code's Bash there is Git Bash), PowerShell and smolcorp all read as the path, where bash
// would take a \ away; in single quotes for a space or a character a shell reads as its own.
func demoShellFolder(path string) string { return shellFolderOn(runtime.GOOS, path) }

func shellFolderOn(goos, path string) string {
	if goos != "windows" {
		return path
	}
	path = strings.ReplaceAll(path, `\`, "/")
	if strings.ContainsAny(path, " &()'^;,=!#%") && !strings.Contains(path, "'") {
		return "'" + path + "'"
	}
	return path
}

// inFenceFolder says whether path is in a .claude or .git folder inside the demo (the project's,
// a worktree's): where the fence, the hooks and the remotes are. Case aside, as the Mac's disk.
func (f demoFence) inFenceFolder(path, cwd string) bool {
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "~") {
		path = filepath.Join(cwd, path)
	} else if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		path = filepath.Join(f.home, path[1:])
	}
	for _, resolved := range []string{resolveExisting(path), resolvePhysical(path)} {
		rest, err := filepath.Rel(f.root, resolved)
		if err != nil {
			return true
		}
		for _, part := range strings.Split(filepath.ToSlash(rest), "/") {
			if part = strings.ToLower(part); part == ".claude" || part == ".git" {
				return true
			}
		}
	}
	return false
}

// inside says whether path (~ is home, a relative one is from cwd) is the demo folder or below it, links followed.
func (f demoFence) inside(path, cwd string) bool {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		path = filepath.Join(f.home, path[1:])
	}
	if !filepath.IsAbs(path) {
		// On Windows, \x is the top of the drive: never the demo's.
		if cwd == "" || !filepath.IsAbs(cwd) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) {
			return false
		}
		path = filepath.Join(cwd, path)
	}
	// Both ways: .. taken first (as a tool that cleans the path would), and .. after each link (as the system does).
	return within(f.root, resolveExisting(path)) && within(f.root, resolvePhysical(path))
}

// resolvePhysical resolves path as the system walks it: each part's links followed before a .. after it.
func resolvePhysical(path string) string {
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	for _, part := range strings.FieldsFunc(path[len(volume):], func(r rune) bool { return r == '/' || r == filepath.Separator }) {
		switch part {
		case ".":
		case "..":
			current = filepath.Dir(current)
		default:
			current = filepath.Join(current, part)
			if resolved, err := filepath.EvalSymlinks(current); err == nil {
				current = resolved
			}
		}
	}
	return current
}

// resolveExisting resolves the links of the longest part of path that's there, and keeps the rest:
// a file about to be written isn't there yet, but the folder it goes into is.
func resolveExisting(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for dir := path; ; dir = filepath.Dir(dir) {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, rest)
		}
		if filepath.Dir(dir) == dir {
			return path
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// demoCommands are the line's simple commands, and those of any word that holds a command
// line of its own (sh -c '…', bash -lc "…"), a few levels down.
func demoCommands(line string, escapes bool, depth int) [][]string {
	segments := demoSegments(line, escapes)
	if depth >= 4 {
		return segments
	}
	for _, words := range segments {
		for _, word := range words {
			if strings.ContainsAny(word, " \t\n;&|`") {
				segments = append(segments, demoCommands(word, escapes, depth+1)...)
			}
		}
	}
	return segments
}

// demoHireFlags reads `smolcorp gm hire`'s flags, as --x v or --x=v: --role, --folder, --account,
// --t3 and --api-key (with any value: either is refused). A flag that takes a value takes the next
// word whatever it is, as Go's flag package does, so --nickname --folder can't hide a folder.
func demoHireFlags(args []string) (ask demoHireAsk) {
	takesValue := []string{"role", "folder", "account", "model", "nickname", "floor", "task", "task-file"}
	for i := 0; i < len(args); i++ {
		name, value, joined := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if !strings.HasPrefix(args[i], "-") {
			continue
		}
		if !joined && i+1 < len(args) && slices.Contains(takesValue, name) {
			i++
			value = args[i]
		}
		switch strings.ReplaceAll(strings.ToLower(name), "_", "-") {
		case "role":
			ask.role = value
		case "folder":
			ask.folder = value
		case "account":
			ask.account = strings.TrimSpace(value)
		case "t3":
			ask.t3 = true
		case "api-key", "apikey":
			ask.apiKey = true
		}
	}
	return ask
}

// demoSegments splits a command line into simple commands' words: quotes are taken
// away, and ; & | ( ) ` newlines and $( end a command; with escapes, a backslash keeps
// the next character. Plain, as the fence needs: every "gm hire" in the line is found,
// whatever runs it.
func demoSegments(line string, escapes bool) [][]string {
	var segments [][]string
	var words []string
	var word strings.Builder
	inWord := false
	end := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endSegment := func() {
		end()
		if len(words) > 0 {
			segments = append(segments, words)
			words = nil
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\'' || c == '"':
			inWord = true
			j := strings.IndexByte(line[i+1:], c)
			if j < 0 {
				j = len(line) - i - 1
			}
			word.WriteString(line[i+1 : i+1+j])
			i += j + 1
		case escapes && c == '\\' && i+1 < len(line):
			inWord = true
			i++
			word.WriteByte(line[i])
		case c == ' ' || c == '\t':
			end()
		case strings.IndexByte(";&|()`\n\r", c) >= 0 || c == '$' && i+1 < len(line) && line[i+1] == '(':
			endSegment()
		default:
			inWord = true
			word.WriteByte(c)
		}
	}
	endSegment()
	return segments
}

// runDemoFence is `smolcorp demo fence --root R --roles a,b`: the call on stdin, an answer only to refuse.
// Anything it can't read is refused: a fence that fails open isn't one.
func runDemoFence(args []string, stdin io.Reader, stdout io.Writer) int {
	f := demoFence{home: homeDir, team: demoTeamHere, hireSession: strings.TrimSpace(os.Getenv(hireKeyEnvironment)) != ""}
	for i := 0; i+1 < len(args); i += 2 {
		switch args[i] {
		case "--root":
			f.root = resolveExisting(args[i+1])
		case "--roles":
			f.roles = strings.Split(args[i+1], ",")
		}
	}
	var c demoCall
	data, err := io.ReadAll(io.LimitReader(stdin, 4<<20))
	why := ""
	switch {
	case len(args)%2 != 0 || f.root == "" || !filepath.IsAbs(f.root) || len(f.roles) == 0:
		why = demoFenceRefusal + "its own command line is wrong."
	case err != nil || json.Unmarshal(data, &c) != nil || c.ToolName == "":
		why = demoFenceRefusal + "it couldn't read the tool call."
	default:
		why = f.check(c)
	}
	if why == "" {
		return 0
	}
	out, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": why,
	}})
	if _, err := fmt.Fprintln(stdout, string(out)); err != nil {
		fmt.Fprintln(os.Stderr, why)
		return 2 // Claude Code reads 2 as block
	}
	return 0
}

// demoTeamHere is the office's team, asked with the GM's own token from the fence's environment
// (a GM hired from Smolcorp has it), as `smolcorp gm team` asks it.
func demoTeamHere() ([]demoMember, error) {
	reply, err := gm.TeamHere()
	if err != nil {
		return nil, err
	}
	var team []demoMember
	for _, m := range reply {
		team = append(team, demoMember{ID: m.ID, Name: m.Name, Nickname: m.Nickname, Role: m.Role, Folder: m.Folder,
			Worktree: m.Worktree, Registered: m.Registered, Branch: m.Branch, State: m.State})
	}
	return team, nil
}
