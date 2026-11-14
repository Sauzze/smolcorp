package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
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
//   - a line or a path that names the setup key: it opens the owner's whole setup; a Read, Grep or
//     Glob, or a word of a line (a pattern too), that reaches into the app's support folder, where it is,
//     but for a demo hire's brief and the files beside it, which a hire reads and its STATUS.md, which it
//     writes (WIN125d); a Glob or Grep from above that folder that could reach the key or a token file;
//   - fail-closed, what it doesn't read (WIN125c): in any line, eval, env -S (--split-string), a
//     nested shell's -c and cmd, powershell or pwsh, which build a command line of their own; and
//     in a line that runs smolcorp (or gm) or git worktree, outside quotes, a # comment, a
//     redirection, cd, pushd, popd, env -C, --git-dir, --work-tree, a GIT_…= setting, a $( ),
//     a backtick or a ^. Those lines are refused, not read: the GM runs the plain command alone;
//   - git config writes, and git checkout or switch: a branch with another fence; a
//     git worktree added outside the demo folder;
//   - whatever the hire guard refuses the GM by default: push, deploy, publish,
//     cloud, remote shells, typing into another session.
//
// Like the guard, it reads one command line and never the scripts it runs: a
// Bash command that writes outside the folder isn't caught here (the office's
// own fence and the owner's permission prompts stay the other layers). Nor is a
// command built at run time: source <(printf '\163molcorp …'), python3 -c
// "os.system('smol'+'corp mcp')", a copy of the program under another name. So
// the fence is never the wall for a hire. The wall is the app's: the demo hires
// its GM held to the project (gmscope.go), so every hire, send, brief or fire
// its token asks for, by whatever way, stays in it: the demo's roles alone, into
// the project or a worktree of it inside the demo folder.

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
	// support is the app's support folder, resolved, where the setup key is: "" for none to keep out of.
	support string
	// hires are the office's hires, as its registry holds them: whose brief in support is a demo hire's
	// (demosBriefFile). nil for none.
	hires func() []Hire
	// team is the office's team as the GM's own token reads it: who a send, brief or fire is for.
	team func() ([]demoMember, error)
	// hireSession: the call is a hire's (its send key is set). The app lets a hire write only to
	// its GM, or to a hire the GM put in its contacts, which this fence checks on the GM's side.
	hireSession bool
	git         gitRunner // nil: git itself
	// goos is the system whose calls these are: "" for this computer's. A test sets another to read
	// that system's calls by its path rules (fencePaths), Windows' on a Mac.
	goos string
}

func (f demoFence) paths() fencePaths { return fencePaths{goos: f.goos} }

// demoFenceRefusal starts every refusal, so the GM and the owner know whose it is.
const demoFenceRefusal = "Refused by the Smolcorp demo's fence: "

// demoSupportRefusal is why a path in the app's own folder is refused.
const demoSupportRefusal = "the app's own folder holds the setup key, which is the owner's: the demo reads only its hires' briefs there, and writes only their STATUS.md and reports."

// check says why the call is refused, or "" to let it run.
func (f demoFence) check(c demoCall) string {
	var input map[string]any
	_ = json.Unmarshal(c.ToolInput, &input)
	// The setup key opens every setup route of the owner's office: no call of the demo's names it,
	// to read it (Read, Grep, cat) or to send it. Escapes taken away first, as JSON's \u002d.
	if namesSetupKey(c.ToolInput) {
		return demoFenceRefusal + "the setup key is the owner's: the demo never reads or sends it."
	}
	// A path not written plainly (a control character, ß, ﬁ, another script, an accent typed apart) may be another
	// name to the disk (ß is ss): refused, as the guard refuses it in a line (WIN125g). An accented Latin letter (é,
	// ç) is plain (WIN125h). Grep's pattern is text, not a path.
	for _, key := range []string{"file_path", "path", "notebook_path", "glob"} {
		if value, _ := input[key].(string); !demoPlainPath(value) {
			return demoFenceRefusal + "a path with a control character or a letter the disk may read as another name (ß, ﬁ, another script): write the path plainly."
		}
	}
	if pattern, _ := input["pattern"].(string); c.ToolName == "Glob" && !demoPlainPath(pattern) {
		return demoFenceRefusal + "a pattern with a control character or a letter the disk may read as another name (ß, ﬁ, another script): write the pattern plainly."
	}
	switch name := c.ToolName; {
	case name == "Read" || name == "Grep" || name == "Glob" || name == "LS" || name == "NotebookRead":
		// What they read: a path, and Glob's pattern from the folder it searches (its path, else cwd). In the
		// app's own folder, only a demo hire's brief and the files beside it (demosBriefFile).
		for _, key := range []string{"file_path", "path", "notebook_path"} {
			if value, _ := input[key].(string); value != "" && f.reachesSupport(value, c.Cwd) {
				return demoFenceRefusal + demoSupportRefusal
			}
		}
		searched := c.Cwd
		if folder, _ := input["path"].(string); folder != "" {
			searched, _ = f.whole(folder, c.Cwd)
		}
		pattern, _ := input["pattern"].(string)
		if name == "Glob" && pattern != "" && f.reachesSupport(pattern, searched) {
			return demoFenceRefusal + demoSupportRefusal
		}
		// From above the app's folder (home, ~/Library, ~/.config, AppData): a Glob whose last part could be the
		// setup key or a token file, and a Grep that reads every file below (no glob) or such a file (RWIN3 N1).
		// Below it, or beside it, a * is only a project's files.
		last := ""
		switch name {
		case "Glob":
			if whole, ok := f.whole(pattern, searched); ok {
				searched, last = f.fixedFolder(whole), pattern[strings.LastIndexAny(pattern, `/\`)+1:]
			}
		case "Grep":
			glob, _ := input["glob"].(string)
			if last = glob[strings.LastIndexAny(glob, `/\`)+1:]; glob == "" {
				last = "*"
			}
		}
		if last != "" && searched != "" && f.aboveSupport(searched) && demoSecretPattern(last) {
			return demoFenceRefusal + "the app's own folder, below " + searched + ", holds the setup key, which is the owner's: search the demo's folder instead."
		}
	case name == "PowerShell":
		// PowerShell builds words the fence can't read: ('smol'+'corp'), quotes like ‘ ’, --% with %VAR%. The
		// demo's GM works in Bash (Git Bash on Windows), as its task says.
		return demoFenceRefusal + "the demo works in Bash, not PowerShell: run the same command with the Bash tool."
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
			if path != "" && f.demosOwnFile(path, c.Cwd) {
				continue // a demo hire's STATUS.md (or report) beside its brief, in the app's own folder
			}
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
		if _, has := input["command"]; !has && name != "Bash" {
			return ""
		}
		if !isText {
			return demoFenceRefusal + "a command it can't read."
		}
		// A call made from a .claude or .git folder, or the app's own folder, reaches the fence or the app's files by a
		// short name (rm settings.json, ../settings.json): Claude Code keeps a cd for the next call (RWIN9 B1).
		if c.Cwd != "" && (guard.KeptCwd(c.Cwd) || f.inside(c.Cwd, "") && f.inFenceFolder(c.Cwd, "") || len(f.supportRests(c.Cwd, "")) > 0) {
			return demoFenceRefusal + "this call runs in " + c.Cwd + ", a .claude or .git folder or the app's own folder, where a short name reaches the demo's fence or the app's files: run it from the project's folder."
		}
		return f.command(command, c.Cwd)
	}
	return ""
}

// command checks a shell command line: what the guard refuses, a hire's folder and role, and no other way to hire.
// It's read as bash would read it too: Claude Code's Bash runs bash, Git Bash on Windows.
func (f demoFence) command(line, cwd string) string {
	// The GM's profile, on the guard's own groups (no rules file): the GM hires and fires; a hire's own office
	// fence refuses it that (gates.go). At the call's folder, as the hook places it: the merge group reads the
	// branch there.
	if v := decideShell(actorGM, shellAsk{line: line, cwd: cwd}); !v.allowed() {
		return demoFenceRefusal + v.why
	}
	// A backslash before a line end joins the lines (bash): `gm hi\⏎re` is gm hire, and a hire
	// written over several lines is one command.
	line = strings.NewReplacer("\\\r\n", "", "\\\n", "").Replace(line)
	if namesHireRoute(line) {
		return demoFenceRefusal + "in the demo, hire with `smolcorp gm hire --folder <a folder in the demo>` only."
	}
	// Read twice, as the guard reads a line: a backslash escapes (bash), or it's part of a path
	// (Windows, C:\Users\…). A hire either reading refuses is refused.
	bashRead, plainRead := demoCommands(line, true, 0), demoCommands(line, false, 0)
	all := append(slices.Clip(bashRead), plainRead...)
	// Fail-closed: what the fence doesn't read is refused, not read (WIN125c).
	if why := demoUnread(line); why != "" {
		return demoFenceRefusal + why + ", which builds a command line the fence doesn't read: run the plain command alone, in Bash."
	}
	if slices.ContainsFunc(all, func(words []string) bool { return namesSmolcorp(words) || gitWorktree(words) }) {
		if why := demoUnreadHere(line); why != "" {
			return demoFenceRefusal + "a smolcorp or git worktree line with " + why + " outside quotes, which the fence doesn't read: run the plain command alone, on a line of its own, with any text in single quotes."
		}
	}
	// A path the shell builds with $ or a backtick ($HOME/…, $(pwd)/…) could be anywhere, the app's own folder
	// too (RWIN5 B1): refused as a path into it is. The demo writes its paths out plainly.
	if word := demoBuiltPath(line); word != "" {
		return demoFenceRefusal + "the shell builds " + word + " with $ or a backtick, so the fence can't tell where it is, and the app's own folder holds the setup key, which is the owner's: write the path out plainly."
	}
	for _, words := range all {
		for _, word := range words {
			for _, path := range demoPathsIn(word) {
				// A path: from home, with a separator, or on a drive (C:x is that drive's own folder, drifts).
				if (strings.HasPrefix(path, "~") || strings.ContainsAny(path, `/\`) || f.paths().volume(path) != "") && (f.reachesSupport(path, cwd) || f.briefFolder(words, path, cwd)) {
					return demoFenceRefusal + demoSupportRefusal
				}
			}
		}
	}
	// A command whose name the shell builds ($X mcp, after X=smolcorp in another call) can't be read.
	for _, words := range demoSegments(line, true) {
		if name := demoCommandName(words); strings.ContainsAny(name, "$`") {
			return demoFenceRefusal + "a command whose name holds $ or a backtick: write the program's name out plainly."
		}
	}
	if shellExpands(line) && slices.ContainsFunc(all, namesSmolcorp) {
		// The shell would build a word the fence can't read: smolcorp $'gm' hire, G=gm; smolcorp $G hire.
		return demoFenceRefusal + "a smolcorp command with $ or a backtick in its line: write it out plainly (text with a $ in single quotes, or in --task-file)."
	}
	if shellGlobs(line, f.paths().windows()) && slices.ContainsFunc(all, namesSmolcorp) {
		// gm {hire,} and gm h?re are gm hire to the shell (h?re when a file named hire is there), not to the fence.
		return demoFenceRefusal + "a smolcorp command with {, *, ?, [ (or zsh's ^, # or ~ inside a word) outside quotes: write it out plainly, text in quotes."
	}
	for _, words := range all {
		if word := f.smolcorpInAWord(words, cwd); word != "" {
			// The word as it was read, not Go-quoted: a \ shows once, as typed.
			why := demoFenceRefusal + `"` + word + `" holds smolcorp: in the demo, smolcorp is the program's name or a path in ` + f.root + " only."
			if f.paths().windows() && !slices.EqualFunc(bashRead, plainRead, slices.Equal) {
				// C:\Users\… unquoted is C:Users… to bash, Git Bash included: not the folder written.
				why += " In bash, a \\ escapes the letter after it, so C:\\Users reads C:Users: write a Windows path with / (" + f.shellFolder(f.root) + ") or in single quotes."
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

// demoPathsIn are the paths a word may give a program: the word, what follows each = or @ in it (dd's
// if=…, --file=…, curl's @…, X=~/… as bash fills in ~ there too), and a short option's value written in the same
// word, from its first / \ or ~ (grep -f/…, -rf~/…).
func demoPathsIn(word string) []string {
	paths := []string{word}
	for i := 0; i < len(word); i++ {
		if word[i] == '=' || word[i] == '@' {
			paths = append(paths, word[i+1:])
		}
	}
	if strings.HasPrefix(word, "-") && !strings.HasPrefix(word, "--") {
		if at := strings.IndexAny(word, `/\~`); at > 0 {
			paths = append(paths, word[at:])
		}
	}
	return paths
}

// demoBuiltPath is the first word of a line that the shell builds with a $ or a backtick outside single quotes
// ($HOME/…, "${HOME}/…", $(pwd)/…, `pwd`/…, $'\x2f…', \$ too) and that holds a / or a \, "" for none (RWIN5 B1).
// A word runs on through quotes and through what $( ), ${ } and backticks hold, as the shell's does, and ends at
// a blank or one of ; & | < > outside them.
func demoBuiltPath(line string) string {
	start, single, double, tick, nest := 0, false, false, false, 0
	built, separator := false, false
	for i := 0; i <= len(line); i++ {
		if i == len(line) || !single && !double && !tick && nest == 0 && strings.IndexByte(" \t\r\n;&|<>", line[i]) >= 0 {
			if built && separator {
				return strings.TrimSpace(line[start:min(i, len(line))])
			}
			start, built, separator = i+1, false, false
			continue
		}
		switch c := line[i]; {
		case c == '/':
			separator = true
		case c == '\\':
			// A separator on Windows, and to bash the letter after it kept: a quote, a blank, a $ (counted all
			// the same, as shellExpands does).
			separator = true
			if !single && i+1 < len(line) {
				i++
				built = built || line[i] == '$' || line[i] == '`'
			}
		case single:
			if c == '\'' {
				single = false
			}
		case c == '\'':
			single = !double
		case c == '"':
			double = !double
		case c == '`':
			built, tick = true, !tick
		case c == '$':
			built = true
			if i+1 < len(line) && (line[i+1] == '(' || line[i+1] == '{') {
				nest++
				i++
			}
		case (c == ')' || c == '}') && nest > 0:
			nest--
		}
	}
	return ""
}

// demoPlainPath is whether a tool's path is written plainly: no control character, and a plain word to the guard
// (printable ASCII, and accented Latin letters typed as one letter: guard.PlainWord).
func demoPlainPath(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] < 0x20 || path[i] == 0x7f {
			return false
		}
	}
	return guard.PlainWord(path)
}

// demoTopWords are the words of a line's simple commands, as bash reads it and as a plain reading does (a \ a
// letter), quoted text one word: not read again as a line of its own.
func demoTopWords(line string) []string {
	var words []string
	for _, escapes := range []bool{true, false} {
		for _, segment := range demoSegments(line, escapes) {
			words = append(words, segment...)
		}
	}
	return words
}

// demoProgram is a word as a program's name: its last path part, folded (guard.Fold), without .exe.
func demoProgram(word string) string {
	name := guard.Fold(strings.ReplaceAll(word, `\`, "/"))
	return strings.TrimSuffix(name[strings.LastIndexByte(name, '/')+1:], ".exe")
}

// demoShells are the shells whose -c runs a line of its own.
var demoShells = []string{"sh", "bash", "zsh", "dash", "ksh", "mksh", "fish", "csh", "tcsh", "busybox"}

// demoUnread is what in any line builds a command line the fence doesn't read, "" for nothing: eval, env -S
// (--split-string, where \_ is a blank), a shell's -c, cmd, powershell and pwsh (^, %VAR%, ('smol'+'corp')), and
// zsh's own ways to run a command (RWIN7 B2): emulate -c anywhere, and noglob, nocorrect, - and repeat where a
// command starts (demoZshStart). The demo's lines run their commands plainly.
func demoUnread(line string) string {
	for _, escapes := range []bool{true, false} {
		for _, words := range demoSegments(line, escapes) {
			if name := demoZshStart(words); name != "" {
				return "zsh's " + name
			}
			for i, word := range words {
				switch name := demoProgram(word); {
				case name == "eval":
					return "eval"
				case name == "emulate":
					return "zsh's emulate"
				case name == "cmd" || name == "powershell" || name == "pwsh":
					return name
				case name == "env" && slices.ContainsFunc(words[i+1:], func(a string) bool {
					return strings.HasPrefix(a, "--s") || !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && strings.Contains(a, "S")
				}):
					return "env -S"
				case slices.Contains(demoShells, name) && slices.ContainsFunc(words[i+1:], func(a string) bool {
					return strings.HasPrefix(a, "--c") || !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && strings.Contains(a, "c")
				}):
					return "a nested " + name + " -c"
				}
			}
		}
	}
	return ""
}

// demoZshStart is zsh's noglob, nocorrect, - or repeat where a simple command's words start a command: first, after
// NAME=value words, after a reserved word (if, then, do, !, …) or after another of them; "" for none. Each runs the
// words after it as a command the fence would read with the modifier as its program (nocorrect git -C <outside>
// status). As an argument (git checkout -, grep repeat), the word is no command.
func demoZshStart(words []string) string {
	reserved := []string{"if", "then", "else", "elif", "do", "while", "until", "time", "!", "{"}
	for _, word := range words {
		switch name := demoProgram(word); {
		case name == "noglob" || name == "nocorrect" || name == "-" || name == "repeat":
			return name
		case slices.Contains(reserved, name) || demoAssignment.MatchString(word):
		default:
			return ""
		}
	}
	return ""
}

// demoUnreadHere is what a smolcorp or git worktree line holds that the fence doesn't read, "" for nothing:
// outside quotes, a # (bash's comment, zsh's glob), < or > (a redirection, which can hide a flag or bring a
// file in), ^ (cmd's escape, zsh's glob), $( or a backtick; and as a word, cd, pushd, popd, env -C, --git-dir,
// --work-tree or a GIT_…= setting, which move where git and the folder written are read from. It reads the line
// both ways, a \ escaping the letter after it (bash) or not (a path's), and finds either's.
func demoUnreadHere(line string) string {
	for _, escapes := range []bool{true, false} {
		single, double := false, false
		for i := 0; i < len(line); i++ {
			switch c := line[i]; {
			case escapes && c == '\\' && !single && i+1 < len(line):
				// A quote or a \ after it is a letter; any other letter is still seen (\# is refused too).
				if strings.IndexByte(`'"\`, line[i+1]) >= 0 {
					i++
				}
			case c == '\'' && !double:
				single = !single
			case c == '"' && !single:
				double = !double
			case !single && (c == '`' || c == '$' && i+1 < len(line) && line[i+1] == '('):
				return "a command substitution"
			case single || double:
			case c == '#':
				return "a #"
			case c == '<' || c == '>':
				return "a redirection"
			case c == '^':
				return "a ^"
			}
		}
	}
	for _, escapes := range []bool{true, false} {
		for _, words := range demoSegments(line, escapes) {
			for i, word := range words {
				low := guard.Fold(word)
				switch name := demoProgram(word); {
				case name == "cd" || name == "pushd" || name == "popd":
					return name
				case strings.HasPrefix(low, "--git-dir") || strings.HasPrefix(low, "--work-tree"):
					return word
				case strings.HasPrefix(low, "git_") && strings.Contains(low, "="):
					return "a GIT_ setting"
				case name == "env" && slices.ContainsFunc(words[i+1:], func(a string) bool {
					return strings.HasPrefix(a, "--c") || !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && strings.Contains(a, "C")
				}):
					return "env -C"
				}
			}
		}
	}
	return ""
}

// gitWorktree says whether a simple command's words run git worktree.
func gitWorktree(words []string) bool {
	at := slices.IndexFunc(words, func(word string) bool { return demoProgram(word) == "git" })
	return at >= 0 && slices.ContainsFunc(words[at+1:], func(word string) bool { return strings.EqualFold(word, "worktree") })
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

// isSmolcorp says whether a word names the smolcorp program, by any path, any case (the Mac's disk, ſmolcorp too:
// guard.Fold), .exe or not.
func isSmolcorp(word string) bool {
	name := guard.Fold(strings.ReplaceAll(word, `\`, "/"))
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
		if !strings.Contains(guard.Fold(word), "smolcorp") || strings.ContainsAny(word, " \t\n") || isSmolcorp(word) {
			continue
		}
		value := word
		if _, after, assigned := strings.Cut(word, "="); assigned && (demoAssignment.MatchString(word) || strings.HasPrefix(word, "-")) {
			value = after
		}
		// A path with either separator (notes\smolcorp-plan.md on Windows, notes/… everywhere), judged as the
		// system whose call it is reads it.
		path := value == "~" || strings.HasPrefix(value, "~/") || f.paths().isAbs(value) || strings.ContainsAny(value, `/\`)
		if !path || !f.plainFolder(value) || !f.inside(value, cwd) {
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
// (X-Smolcorp-Setup-Key), any case (guard.Fold: ſetup-key too, RWIN5 B3), JSON's escapes read.
func namesSetupKey(input json.RawMessage) bool {
	var value any
	text := string(input)
	if json.Unmarshal(input, &value) == nil {
		data, _ := json.Marshal(value)
		text = string(data)
	}
	return strings.Contains(guard.Fold(text), "setup-key")
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

// shellGlobs says whether a line holds, outside quotes, a character the shell makes words with: bash's braces
// and globs ({hire,}, h?re, *, [), and off Windows zsh's extended globs too (^x, and a # or ~ inside a word:
// x#, a~b). On Windows, Claude Code's shell is Git Bash, and a short name's ~ (RUNNER~1) is a letter.
func shellGlobs(line string, windows bool) bool {
	single, double := false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		wordStart := i == 0 || strings.IndexByte(" \t\r\n;&|()", line[i-1]) >= 0
		switch {
		case c == '\\' && !single:
			i++
		case c == '\'' && !double:
			single = !single
		case c == '"' && !single:
			double = !double
		case single || double:
		case strings.IndexByte("{*?[", c) >= 0:
			return true
		case !windows && (c == '^' || (c == '#' || c == '~') && !wordStart):
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
	line = guard.Fold(strings.ReplaceAll(line, `\`, "/"))
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
		if demoProgram(word) != "git" {
			continue
		}
		sub, args, from := "", []string(nil), cwd
		for j := i + 1; j < len(words); j++ {
			a := words[j]
			if strings.HasPrefix(a, "-") {
				if !strings.Contains(a, "=") && slices.Contains([]string{"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--config-env"}, a) {
					if a == "-C" && j+1 < len(words) {
						from = f.from(words[j+1], from)
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
			if path := gitWorktreeAddPath(args[1:]); path == "" || !f.plainFolder(path) || !f.inside(path, from) {
				return "a worktree of the demo's stays in " + f.root + ": the office makes the hires' own."
			}
		}
	}
	return ""
}

// from is the folder git -C dir works from, read from where the command runs as whole reads it: "" when it
// can't be told.
func (f demoFence) from(dir, cwd string) string {
	whole, _ := f.whole(dir, cwd)
	return whole
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
	project := f.shellFolder(f.paths().join(f.root, demoRepoName))
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
		return demoFenceRefusal + "name the folder: --folder " + project + "."
	case !f.plainFolder(folder):
		// The shell would make it something else ($HOME, ~user, {a,b}, *, %VAR%): the fence can't know what.
		return demoFenceRefusal + "write the folder out plainly, with no $, `, {, *, ?, [, %, ! or ~user in it: --folder " + project + "."
	case !f.checkout(folder, cwd):
		return demoFenceRefusal + "hire into the demo's project, " + project + " (with --worktree), or the top of one of its worktrees: no other folder holds the demo's fence."
	}
	return ""
}

// checkout says whether folder (~ is home, a relative one is from cwd) is the demo's project, or the top of
// a worktree of it in the demo folder: the folders whose .claude/settings.json is the demo's fence. It must be
// there, so a link made in the same line can't stand in for it. The rule is the app's own (gmScope.holds).
// For another system's calls (a test's goos) there's no disk of that system to look at: never held.
func (f demoFence) checkout(folder, cwd string) bool {
	folder, ok := f.whole(folder, cwd)
	if !ok || !f.paths().here() {
		return false
	}
	git := f.git
	if git == nil {
		git = runGit
	}
	return f.inside(folder, "") && gmScope{root: filepath.Join(f.root, demoRepoName), git: git}.holds(folder)
}

// whole is path as a whole path: ~ is home, a relative one is from cwd. On Windows, Git Bash's drives are read
// first (/c/… is C:\…, gitBash). False when it can't be told: no cwd to start from, or the path's own doing
// (untold).
func (f demoFence) whole(path, cwd string) (string, bool) {
	whole, ok, _ := f.place(path, cwd)
	return whole, ok
}

// place is whole, and untold when what the path is can't be told whatever the cwd: \x or a /x on Windows that
// isn't a Git Bash drive (the top of some drive, Git Bash's own /tmp), C:x or a : after the drive (drifts: that
// drive's own folder, an NTFS stream), ~user (that user's home to bash, RWIN4 N1). Such a path could be the app's support folder (supportRests). Git
// Bash's /dev/null and the rest of /dev are neither told nor untold: no folder of the disk's.
func (f demoFence) place(path, cwd string) (whole string, ok, untold bool) {
	p := f.paths()
	path, cwd = p.gitBash(strings.TrimSpace(path)), p.gitBash(cwd)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		path = p.join(f.home, path[1:])
	}
	switch {
	case otherHome(path) || p.drifts(path):
		return "", false, true
	case p.isAbs(path):
		return path, true, false
	case p.gitBashDevice(path):
		return "", false, false
	case p.windows() && p.rooted(path):
		return "", false, true
	case cwd == "" || !p.isAbs(cwd):
		return "", false, false
	}
	return p.join(cwd, path), true, false
}

// otherHome says whether path starts with a ~ that isn't the home's own (~, ~/…, ~\…): ~user, ~+, ~-, a
// home or a folder bash fills in and the fence doesn't.
func otherHome(path string) bool {
	path = strings.TrimSpace(path)
	return strings.HasPrefix(path, "~") && path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`)
}

// plainFolder says whether the shell leaves a folder as written: no expansion in it but ~ or ~/… at its start.
// $ ` { * ? [ are bash's; % and ! are cmd's (%USERPROFILE%), and # and ^ zsh's globs (extended_glob). Off
// Windows, any other ~ is refused: ~user, and zsh's a~b. On Windows, where Claude Code's shell is Git Bash, a ~
// is a letter only in a short name's shape, a name's letters then ~ and digits (RUNNER~1, PROGRA~1).
func (f demoFence) plainFolder(folder string) bool { return plainFolderOn(f.paths().windows(), folder) }

func plainFolderOn(windows bool, folder string) bool {
	if strings.ContainsAny(folder, "$`{*?[%!#^") {
		return false
	}
	rest := folder
	if folder == "~" || strings.HasPrefix(folder, "~/") || strings.HasPrefix(folder, `~\`) {
		rest = folder[1:]
	}
	if !windows {
		return !strings.Contains(rest, "~")
	}
	return !strings.Contains(demoShortName.ReplaceAllString(rest, "$1"), "~")
}

// demoShortName is a Windows short name's ~ and digits, after a name's letter and before a separator, a dot or the end.
var demoShortName = regexp.MustCompile(`([^/\\=:~])~[0-9]+([/\\.]|$)`)

// demoShellFolder is a folder as the demo writes it in a command for the GM: as it is, and on Windows with /,
// which bash (Claude Code's Bash there is Git Bash) and smolcorp both read as the path, where bash would take
// a \ away; quoted for a space or a character a shell reads as its own.
func demoShellFolder(path string) string { return shellFolderOn(runtime.GOOS, path) }

func (f demoFence) shellFolder(path string) string {
	goos := f.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	return shellFolderOn(goos, path)
}

func shellFolderOn(goos, path string) string {
	if goos != "windows" {
		return path
	}
	path = strings.ReplaceAll(path, `\`, "/")
	switch {
	case strings.Contains(path, "'"):
		// In double quotes: a Windows path has no ", and a $ or ` in it the fence refuses anyway.
		return `"` + path + `"`
	case strings.ContainsAny(path, " &()^;,=!#%"):
		return "'" + path + "'"
	}
	return path
}

// inFenceFolder says whether path is in a .claude or .git folder inside the demo (the project's,
// a worktree's): where the fence, the hooks and the remotes are. Case aside, as the Mac's disk; on Windows
// a name's dots and blanks at its end aside too, as Windows drops them (.git. is .git).
func (f demoFence) inFenceFolder(path, cwd string) bool {
	p := f.paths()
	path, ok := f.whole(path, cwd)
	if !ok {
		return true
	}
	for _, resolved := range []string{p.resolveExisting(path), p.resolvePhysical(path)} {
		rest, ok := p.below(f.root, resolved)
		if !ok {
			return true
		}
		for _, part := range strings.Split(rest, "/") {
			if part = guard.Fold(part); p.windows() {
				part = strings.TrimRight(part, ". ")
			}
			if part == ".claude" || part == ".git" {
				return true
			}
		}
	}
	return false
}

// supportRests are where path (~ is home, a relative one is from cwd) is in the app's support folder: its rest
// below it, parts split by /, "." for the folder itself, once for each way its links are followed (a tool
// that cleans the path first, the system that walks it); none outside it. In any case where the disk reads
// names case aside, and by whatever name this computer's disk has for it (belowAnyCase, RWIN4 B2), each name as
// Win32 reads it on Windows (win32: registry.json::$DATA, SauzzeDeck Pro.). A path whole can't tell for its own
// reasons (~user, \x or /tmp on Windows, C:x: untold) is somewhere the fence doesn't read: the support folder,
// fail-closed (RWIN4 N1, RWIN5 B2). One it can't tell for want of a cwd isn't.
func (f demoFence) supportRests(path, cwd string) []string {
	p := f.paths()
	path, ok, untold := f.place(p.win32(p.gitBash(strings.TrimSpace(path))), cwd)
	switch {
	case f.support == "":
		return nil
	case untold:
		return []string{"."}
	case !ok:
		return nil
	}
	var rests []string
	for _, resolved := range []string{p.resolveExisting(path), p.resolvePhysical(path)} {
		if rest, in := p.belowAnyCase(f.support, resolved); in {
			rests = append(rests, rest)
		}
	}
	return rests
}

// reachesSupport says whether path (~ is home, a relative one is from cwd), or what it fills in as a pattern
// (patternInSupport), is in the app's support folder anywhere but a demo hire's brief and the files beside it.
func (f demoFence) reachesSupport(path, cwd string) bool {
	rests := f.supportRests(path, cwd)
	if rest, in := f.patternInSupport(path, cwd); in {
		rests = append(rests, rest)
	}
	return slices.ContainsFunc(rests, func(rest string) bool { return !f.demosBriefFile(rest, false) })
}

// briefFolder says whether a Bash word that is a demo hire's brief file (or one beside it) could be a folder to
// work from in the app's own folder (RWIN4 N2): it's there and isn't a file, or the command makes it a folder or
// goes into it (mkdir, cd, pushd). From such a folder, cd …/d.md && cat ../../../registry.json reads the app's state.
func (f demoFence) briefFolder(words []string, word, cwd string) bool {
	if len(f.supportRests(word, cwd)) == 0 {
		return false
	}
	switch demoProgram(demoCommandName(words)) {
	case "mkdir", "cd", "pushd":
		return true
	}
	path, ok := f.whole(word, cwd)
	if !ok || !f.paths().here() {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && !info.Mode().IsRegular()
}

// demosOwnFile says whether path is a file a demo hire writes in the app's own folder, whichever way its links
// are followed: its STATUS.md (or report) beside its brief.
func (f demoFence) demosOwnFile(path, cwd string) bool {
	rests := f.supportRests(path, cwd)
	return len(rests) == 2 && !strings.ContainsAny(path, demoWildcards) && f.demosBriefFile(rests[0], true) && f.demosBriefFile(rests[1], true)
}

// demoWildcards are the letters that make a path a pattern, to the shell or to Glob.
const demoWildcards = "*?[{"

// patternInSupport is a pattern's rest below the app's support folder when what it fills in could be there
// (cat ~/Lib*/App*/A*/s*, RWIN3 N7): each of its parts matches the folder's, case aside (demoPartMatches), a **
// part or one whose run could take a / (Lib* as Library/…) any number of them, and its plain folder before the
// first wildcard has its links followed and is compared as belowAnyCase does. false for a path with no wildcard, or one that stays above the folder or beside it.
func (f demoFence) patternInSupport(path, cwd string) (string, bool) {
	p := f.paths()
	path, ok := f.whole(p.win32(p.gitBash(strings.TrimSpace(path))), cwd)
	if f.support == "" || !ok || !strings.ContainsAny(path, demoWildcards) {
		return "", false
	}
	path = p.clean(path)
	cut := strings.IndexAny(path, demoWildcards)
	for cut > 0 && !p.separator(path[cut-1]) {
		cut--
	}
	folder, pattern := p.resolveExisting(path[:cut]), p.parts(path[cut:])
	if rest, in := p.belowAnyCase(f.support, folder); in {
		// In the folder already: the pattern fills in names below it.
		if rest != "." {
			pattern = append(strings.Split(rest, "/"), pattern...)
		}
		return strings.Join(pattern, "/"), true
	}
	// Above it: the pattern's parts against the folder's names below the plain folder.
	below, above := p.belowAnyCase(folder, f.support)
	if !above {
		return "", false
	}
	want := strings.Split(below, "/")
	for i, part := range pattern {
		switch {
		case i == len(want) || part == "**":
			return strings.Join(pattern[i:], "/"), true
		case guard.PatternCouldFill(part, want[i], true):
			// A run may take a / too (WIN125j): Lib* may be Library/Application Support/AgentDeck/….
			return strings.Join(pattern[i:], "/"), true
		case !demoPartMatches(part, want[i]):
			return "", false
		}
	}
	if len(pattern) == len(want) {
		return ".", true
	}
	return "", false
}

// demoPartMatches says whether one part of a pattern could fill in name, case aside, read as the guard reads a
// pattern (guard.PatternCouldFill, WIN125j): each character bash, zsh or Glob may give a meaning ([!x-[:x:]], {a,b},
// x^y) starts a run that may be anything, so a match is never narrower than the shell's or Glob's.
func demoPartMatches(pattern, name string) bool {
	return guard.PatternCouldFill(pattern, name, false)
}

// fixedFolder is a whole pattern's folder before its first wildcard: where a search for it starts.
func (f demoFence) fixedFolder(pattern string) string {
	p := f.paths()
	cut := strings.IndexAny(pattern, demoWildcards)
	if cut < 0 {
		cut = len(pattern)
	}
	for cut > 0 && !p.separator(pattern[cut-1]) {
		cut--
	}
	return p.clean(pattern[:cut])
}

// aboveSupport says whether folder (a whole path) holds the app's support folder, or is it, links followed, in
// any case where the disk reads names case aside (belowAnyCase).
func (f demoFence) aboveSupport(folder string) bool {
	p := f.paths()
	if f.support == "" || folder == "" {
		return false
	}
	folder = p.win32(folder)
	for _, resolved := range []string{p.resolveExisting(folder), p.resolvePhysical(folder)} {
		if _, in := p.belowAnyCase(resolved, f.support); in {
			return true
		}
	}
	return false
}

// demoSecretNames are the app's files that a Glob or Grep from above its folder mustn't reach by its last part:
// the setup key and the GM's token files (gmbyhand.go, gmtokenstore.go), the owner's grant record (grants.go) and
// the paired phones' seal's key (access.go).
var demoSecretNames = []string{"setup-key", gmTokenFileName, keptGMTokenName, gmSessionFileName, "grants.jsonl", "phones-seal"}

// demoSecretPattern says whether a pattern's last part could fill in one of demoSecretNames.
func demoSecretPattern(last string) bool {
	return slices.ContainsFunc(demoSecretNames, func(name string) bool { return demoPartMatches(last, name) })
}

// demoBriefTail is what follows a kept brief's stem in a file that isn't its own: -2.md, -2-STATUS.md, the
// next brief of the same desk and minute.
var demoBriefTail = regexp.MustCompile(`^[0-9]+(-|$)`)

// demosBriefFile says whether rest, below the app's support folder, is a demo hire's brief or a file beside it,
// as the registry says where each hire's brief is (briefs.go): a hire whose folder, and worktree if it has one,
// are in the demo folder. A managed hire's folder (hires/<id>) is its own: every .md in it is read, and all but
// a BRIEF written (its STATUS.md, its report). A brief kept in briefs/ (the GM's, a hire's with no worktree) is
// <stem>.md, read, beside <stem>-STATUS.md and its report, read and written. Nothing else there: the setup key,
// the GM's token, the app's state, another project's hire.
func (f demoFence) demosBriefFile(rest string, write bool) bool {
	dir, name := path.Split(rest)
	dir = strings.TrimSuffix(dir, "/")
	if f.hires == nil || !strings.EqualFold(path.Ext(name), ".md") {
		return false
	}
	p := f.paths()
	low := guard.Fold(name)
	for _, h := range f.hires() {
		if h.Brief == "" || !f.inside(h.Folder, "") || h.Worktree != "" && !f.inside(h.Worktree, "") {
			continue
		}
		brief, in := p.belowAnyCase(f.support, p.resolveExisting(h.Brief))
		briefDir, briefName := path.Split(brief)
		briefDir = strings.TrimSuffix(briefDir, "/")
		switch {
		case !in || !strings.EqualFold(dir, briefDir):
		case strings.EqualFold(briefDir, "hires/"+h.ID) && h.ID != "" && !strings.ContainsAny(h.ID, `/\`) && !strings.HasPrefix(h.ID, "."):
			if !write || !strings.HasPrefix(low, "brief") {
				return true
			}
		case strings.EqualFold(briefDir, "briefs"):
			stem := guard.Fold(strings.TrimSuffix(briefName, path.Ext(briefName))) + "-"
			if !write && strings.EqualFold(name, briefName) {
				return true
			}
			if tail := strings.TrimSuffix(strings.TrimPrefix(low, stem), ".md"); strings.HasPrefix(low, stem) && tail != "" && !demoBriefTail.MatchString(tail) {
				return true
			}
		}
	}
	return false
}

// inside says whether path (~ is home, a relative one is from cwd) is the demo folder or below it, links followed.
func (f demoFence) inside(path, cwd string) bool {
	p := f.paths()
	path, ok := f.whole(path, cwd)
	if !ok {
		return false
	}
	// Both ways: .. taken first (as a tool that cleans the path would), and .. after each link (as the system does).
	_, cleanIn := p.below(f.root, p.resolveExisting(path))
	_, physicalIn := p.below(f.root, p.resolvePhysical(path))
	return cleanIn && physicalIn
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
// away, and ; & | ( ) ` newlines and $( end a command, < and > a word; with escapes, a backslash keeps
// the next character (in double quotes, a " \ $ or ` only, as bash). Plain, as the fence needs: every "gm hire" in the line is found,
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
		case escapes && c == '"':
			// bash's double quotes: \" \\ \$ \` keep the letter, so "\" --folder x" is one word, not two.
			inWord = true
			for i++; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' && i+1 < len(line) && strings.IndexByte("\"\\$`", line[i+1]) >= 0 {
					i++
				}
				word.WriteByte(line[i])
			}
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
		case c == '<' || c == '>':
			// A redirection's operator ends a word, and its target, glued to it or not (>file, 2>file, <file), is
			// a word of its own, judged as a path (RWIN6 B1).
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
// demoFenceHere is a fence on this computer, for the session it runs in, before its root and roles are read.
func demoFenceHere() demoFence {
	return demoFence{home: homeDir, support: resolveExisting(supportDir), team: demoTeamHere, hireSession: strings.TrimSpace(os.Getenv(hireKeyEnvironment)) != "",
		hires: func() []Hire { return readRegistry(registryPath(), false).hires }}
}

func runDemoFence(args []string, stdin io.Reader, stdout io.Writer) int {
	f := demoFenceHere()
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
