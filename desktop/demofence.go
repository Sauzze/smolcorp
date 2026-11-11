package desktop

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
//   - changing a file outside the demo folder (Edit, Write, MultiEdit, NotebookEdit);
//   - a hire outside the demo folder: `smolcorp gm hire` (or the MCP tool hire)
//     must name a --folder inside it and one of the roles given, which are the
//     Claude Code roles, so every hire reads this file too; never into T3 Code;
//   - a hire by any other way (a request to the app's hire routes);
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
		path, _ := input["file_path"].(string)
		if name == "NotebookEdit" && path == "" {
			path, _ = input["notebook_path"].(string)
		}
		if path == "" || !f.inside(path, c.Cwd) {
			return demoFenceRefusal + "the demo only changes files inside " + f.root + "."
		}
	case strings.HasPrefix(name, "mcp__") && strings.HasSuffix(name, "__hire"):
		role, _ := input["role"].(string)
		folder, _ := input["folder"].(string)
		return f.hire(role, folder, false, c.Cwd)
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
	if strings.Contains(line, "/setup/hire") || strings.Contains(line, "/setup/gm/hire") {
		return demoFenceRefusal + "in the demo, hire with `smolcorp gm hire --folder <a folder in the demo>` only."
	}
	// Read twice, as the guard reads a line: a backslash escapes (bash), or it's part of a path
	// (Windows, C:\Users\…). A hire either reading refuses is refused.
	for _, words := range append(demoCommands(line, true, 0), demoCommands(line, false, 0)...) {
		for i := 0; i+1 < len(words); i++ {
			if words[i] != "gm" || words[i+1] != "hire" {
				continue
			}
			role, folder, t3 := demoHireFlags(words[i+2:])
			if why := f.hire(role, folder, t3, cwd); why != "" {
				return why
			}
		}
	}
	return ""
}

// hire checks one hire: into T3 Code never, a role from the list, a folder inside the demo.
func (f demoFence) hire(role, folder string, t3 bool, cwd string) string {
	switch {
	case t3:
		return demoFenceRefusal + "the demo hires in terminals, not T3 Code."
	case !slices.Contains(f.roles, role):
		return demoFenceRefusal + fmt.Sprintf("the demo hires these roles only, all on Claude Code: %s.", strings.Join(f.roles, ", "))
	case strings.TrimSpace(folder) == "":
		return demoFenceRefusal + "name the folder: --folder " + filepath.Join(f.root, demoRepoName) + "."
	case !f.inside(folder, cwd):
		return demoFenceRefusal + "that folder is outside the demo. Hire into " + filepath.Join(f.root, demoRepoName) + "."
	}
	return ""
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
	return within(f.root, resolveExisting(path))
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

// demoHireFlags reads `smolcorp gm hire`'s flags: --role, --folder and --t3, as --x v or --x=v.
func demoHireFlags(args []string) (role, folder string, t3 bool) {
	for i := 0; i < len(args); i++ {
		name, value, joined := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if !strings.HasPrefix(args[i], "-") {
			continue
		}
		if !joined && i+1 < len(args) && (name == "role" || name == "folder") {
			i++
			value = args[i]
		}
		switch name {
		case "role":
			role = value
		case "folder":
			folder = value
		case "t3":
			t3 = true
		}
	}
	return role, folder, t3
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
