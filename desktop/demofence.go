package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

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
//   - a hire outside the demo folder: `smolcorp gm hire` (or the MCP tool hire)
//     must name a --folder inside it, as a plain path, and one of the roles given,
//     which are the Claude Code roles, so every hire reads this file too; never
//     into T3 Code, on the API-key account, or on a Claude account but the default;
//   - a hire by any other way (a request to the app's hire routes), and a line that
//     names smolcorp (or gm and a word) and builds words with $ or a backtick;
//   - git config writes, and git checkout or switch: a branch with another fence;
//   - whatever the hire guard refuses the GM by default: push, deploy, publish,
//     cloud, remote shells, typing into another session.
//
// Like the guard, it reads one command line and never the scripts it runs: a
// Bash command that writes outside the folder isn't caught here (the office's
// own fence and the owner's permission prompts stay the other layers).

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
}

// demoFenceRefusal starts every refusal, so the GM and the owner know whose it is.
const demoFenceRefusal = "Refused by the Smolcorp demo's fence: "

// check says why the call is refused, or "" to let it run.
func (f demoFence) check(c demoCall) string {
	var input map[string]any
	_ = json.Unmarshal(c.ToolInput, &input)
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
	default:
		command, isText := input["command"].(string)
		if _, has := input["command"]; !has && name != "Bash" && name != "PowerShell" {
			return ""
		}
		if !isText {
			return demoFenceRefusal + "a command it can't read."
		}
		return f.command(command, c.Cwd)
	}
	return ""
}

// command checks a shell command line: what the guard refuses, a hire's folder and role, and no other way to hire.
func (f demoFence) command(line, cwd string) string {
	// The GM's profile: the GM hires and fires; a hire's own office fence refuses it that.
	if r := guard.Check(line, guard.ForGM(guard.DefaultGroups())); r != nil {
		return demoFenceRefusal + r.Error()
	}
	if namesHireRoute(line) {
		return demoFenceRefusal + "in the demo, hire with `smolcorp gm hire --folder <a folder in the demo>` only."
	}
	// Read twice, as the guard reads a line: a backslash escapes (bash), or it's part of a path
	// (Windows, C:\Users\…). A hire either reading refuses is refused.
	all := append(demoCommands(line, true, 0), demoCommands(line, false, 0)...)
	if shellExpands(line) && slices.ContainsFunc(all, namesSmolcorp) {
		// The shell would build a word the fence can't read: smolcorp $'gm' hire, G=gm; smolcorp $G hire.
		return demoFenceRefusal + "a smolcorp command with $ or a backtick in its line: write it out plainly (text with a $ in single quotes, or in --task-file)."
	}
	for _, words := range all {
		if why := demoGitRefusal(words); why != "" {
			return demoFenceRefusal + why
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
		name := strings.ToLower(strings.ReplaceAll(word, `\`, "/"))
		name = strings.TrimSuffix(name[strings.LastIndexByte(name, '/')+1:], ".exe")
		if name == "smolcorp" || name == "smolcorp-cli" || word == "gm" && i+1 < len(words) {
			return true
		}
	}
	return false
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

// demoGitRefusal says why a git command is refused: a config write (hooks, remotes), or a branch
// switch (a branch of the GM's could carry another fence into every hire made after it).
func demoGitRefusal(words []string) string {
	for i, word := range words {
		name := strings.ToLower(strings.ReplaceAll(word, `\`, "/"))
		if strings.TrimSuffix(name[strings.LastIndexByte(name, '/')+1:], ".exe") != "git" {
			continue
		}
		sub, args := "", []string(nil)
		for j := i + 1; j < len(words); j++ {
			a := words[j]
			if strings.HasPrefix(a, "-") {
				if !strings.Contains(a, "=") && slices.Contains([]string{"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--config-env"}, a) {
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

// hire checks one hire: into T3 Code never, nor on the API-key account or another Claude account;
// a role from the list; a folder inside the demo, written as a plain path.
func (f demoFence) hire(ask demoHireAsk, cwd string) string {
	folder := strings.TrimSpace(ask.folder)
	switch {
	case ask.t3:
		return demoFenceRefusal + "the demo hires in terminals, not T3 Code."
	case ask.apiKey:
		return demoFenceRefusal + "the demo hires on your Claude account's plan, never the API-key account: leave out --api-key."
	case ask.account != "" && ask.account != claudeDefaultName:
		return demoFenceRefusal + "the demo hires on your default Claude account only, as you were asked: leave out --account."
	case !slices.Contains(f.roles, ask.role):
		return demoFenceRefusal + fmt.Sprintf("the demo hires these roles only, all on Claude Code: %s.", strings.Join(f.roles, ", "))
	case folder == "":
		return demoFenceRefusal + "name the folder: --folder " + filepath.Join(f.root, demoRepoName) + "."
	case !plainFolder(folder):
		// The shell would make it something else ($HOME, ~user, {a,b}, *): the fence can't know what.
		return demoFenceRefusal + "write the folder out plainly, with no $, `, {, *, ?, [ or ~user in it: --folder " + filepath.Join(f.root, demoRepoName) + "."
	case !f.inside(folder, cwd):
		return demoFenceRefusal + "that folder is outside the demo. Hire into " + filepath.Join(f.root, demoRepoName) + "."
	}
	return ""
}

// plainFolder says whether the shell leaves a folder as written: no expansion in it but ~ or ~/… at its start.
func plainFolder(folder string) bool {
	if strings.ContainsAny(folder, "$`{*?[") {
		return false
	}
	rest := folder
	if folder == "~" || strings.HasPrefix(folder, "~/") || strings.HasPrefix(folder, `~\`) {
		rest = folder[1:]
	}
	return !strings.Contains(rest, "~")
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
	f := demoFence{home: homeDir}
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
