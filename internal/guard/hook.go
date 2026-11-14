package guard

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Sauzze/smolcorp/internal/appdirs"
)

// Exit codes: the hook itself always ends 0 (its answer is its JSON, or
// nothing for "allow"); 2 is a bad command line, which Claude Code also
// treats as a block.
const (
	exitOK    = 0
	exitUsage = 2
)

// fileName is the owner's guard file, in the app's support folder.
const fileName = "hire-guard.json"

// testSupportEnv names a test copy's own support folder (the Mac app's
// TestCopy.support, and SMOLCORP_TEST_SUPPORT_DIR in a hire's start script).
// The guard never reads it: a hire can set any variable for itself, so a
// test copy's desk names its folder's file in the guard's own command line
// (--rules), as every desk does (SANDBOX-PLAN §3.4, g4).
const testSupportEnv = "SMOLCORP_TEST_SUPPORT_DIR"

// supportFile is where hire-guard.json lives when no --rules names it: the
// Mac app's folder in Application Support, or the same folder desktop/paths.go
// uses. Never a folder from the environment.
func supportFile() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("can't find the app's support folder: %v", err)
	}
	return filepath.Join(base, appdirs.Name(runtime.GOOS), fileName), nil
}

// deny prints Claude Code's PreToolUse answer for a refusal. If the answer
// can't be written, the exit code says block (2): a silent 0 would read as allow.
func deny(stdout io.Writer, reason string) int {
	out, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":            "PreToolUse",
		"permissionDecision":       "deny",
		"permissionDecisionReason": reason,
	}})
	if _, err := fmt.Fprintln(stdout, string(out)); err != nil {
		return exitUsage
	}
	return exitOK
}

// Hook is `smolcorp guard`, run by Claude Code before each tool call with
// the call as JSON on stdin. The command of a Bash or PowerShell call is
// checked, and so is the `command` of any other tool that has one (Monitor
// runs a shell command too). An allowed call prints nothing. Anything that
// can't be told is denied; so is every command while the guard file is
// broken (problem says how).
func Hook(stdin io.Reader, stdout io.Writer, groups []Group, problem string) int {
	var call struct {
		ToolName  *string         `json:"tool_name"`
		ToolInput json.RawMessage `json:"tool_input"`
		Cwd       string          `json:"cwd"`
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 4<<20))
	if err != nil || json.Unmarshal(data, &call) != nil || call.ToolName == nil {
		return deny(stdout, "Refused by Smolcorp's hire guard: it couldn't read the tool call.")
	}
	shell := *call.ToolName == "Bash" || *call.ToolName == "PowerShell"
	var input map[string]json.RawMessage
	_ = json.Unmarshal(call.ToolInput, &input) // not an object: no command, which only a shell tool needs
	raw, has := input["command"]
	if !has && !shell {
		return exitOK // a tool with no command: nothing to check
	}
	var command *string
	if !has || json.Unmarshal(raw, &command) != nil || command == nil {
		// A shell tool without a readable command, or any tool whose "command" isn't text.
		return deny(stdout, "Refused by Smolcorp's hire guard: a "+*call.ToolName+" call with no command it can read.")
	}
	// A call made from inside a .claude folder or the app's own folder reaches what the guard keeps by a short name
	// (rm ../settings.json): Claude Code keeps a cd that ends in the project for the next call (RWIN9 B1). A broken
	// file is still said first (Refused), as before.
	if problem == "" && KeptCwd(call.Cwd) {
		return deny(stdout, "Refused by Smolcorp's hire guard ("+keptDirRule+"): this call runs in "+call.Cwd+". "+keptDirWhy)
	}
	if why := Refused(*command, At(groups, hookPlace(call.Cwd)), problem); why != "" {
		return deny(stdout, why)
	}
	return exitOK
}

// hookPlace is the repository a call runs in, for the merge group: the call's folder, unless the desk's environment
// (the hook's own, from Claude Code) names another git folder, work tree or object folder, which the guard doesn't
// follow.
func hookPlace(cwd string) Place {
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_NAMESPACE", "GIT_OBJECT_DIRECTORY"} {
		if os.Getenv(name) != "" {
			return Place{}
		}
	}
	return PlaceOf(cwd)
}

// Refused is why the hook refuses a command line with groups, "" to let it run. While the guard file is
// broken (problem says how), every line is refused: a broken file is never read as "allow", and every
// command waits until it is fixed. The app's gate service (desktop/gates.go) decides lines with it too.
func Refused(command string, groups []Group, problem string) string {
	if problem != "" {
		return "Refused by Smolcorp's hire guard: " + fileName + " is broken (" + problem + "). Ask the owner to fix it."
	}
	if r := Check(command, groups); r != nil {
		return r.Error()
	}
	return ""
}

// Profiled is the groups from the rules file at path for profile (ProfileHire, or ProfileGM for the
// General Manager's), and what's wrong with the file, as `smolcorp guard --profile` reads them: with the merge and
// spend groups when the file doesn't name them (WithGates).
func Profiled(path, profile string) (groups []Group, problem string) {
	groups, problem = Load(path)
	if problem == "" { // a broken file's default groups refuse every line anyway
		groups = WithGates(groups)
	}
	if profile == ProfileGM {
		groups = ForGM(groups)
	}
	return groups, problem
}

// KeptCwd says whether a tool call's folder (the hook input's cwd, a real path: no pattern in it) is in a .claude
// folder or the app's support folder (keptFolderName): a command there reaches Claude Code's settings or the app's
// files by a short name. The demo's fence asks it too. So a hire started inside Claude Code's own .claude/worktrees/…
// can run nothing (Smolcorp starts none there today), and a cwd in .git isn't refused here as it is in the demo's
// fence, as nothing the guard keeps is in .git (RWIN10 N3).
func KeptCwd(cwd string) bool {
	return keptFolderName(cwd)
}

// Run is `smolcorp guard`:
//
//	guard [--profile P] [--groups JSON] [--rules FILE]
//	                               the hook: the tool call on stdin, the groups of FILE
//	                               (default: hire-guard.json in the support folder); with
//	                               --groups, those groups, which FILE only adds to (held.go)
//	guard --print [--profile P] [--groups JSON] [--rules FILE]
//	                               the groups it uses now, as JSON, and what's wrong with FILE
//	guard --config                 the ConfigChange hook: a settings change on stdin, refused
//	                               unless it is managed settings' or a skill's (held.go)
//	guard --default                the default file, to start one from
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rules, profile, printing := "", ProfileHire, false
	held, holding := "", false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--groups":
			if i+1 >= len(args) || holding {
				return deny(stdout, "Refused by Smolcorp's hire guard: --groups needs the groups, once.")
			}
			i++
			held, holding = args[i], true
		case "--config":
			if len(args) != 1 {
				fmt.Fprintln(stderr, configRefusal)
				return exitBlock
			}
			return ConfigHook(stdin, stderr)
		case "--rules":
			if i+1 >= len(args) {
				return deny(stdout, "Refused by Smolcorp's hire guard: --rules needs a file.")
			}
			i++
			rules = args[i]
		case "--profile":
			if i+1 >= len(args) || (args[i+1] != ProfileHire && args[i+1] != ProfileGM) {
				return deny(stdout, "Refused by Smolcorp's hire guard: --profile needs hire or gm.")
			}
			i++
			profile = args[i]
		case "--print":
			printing = true
		case "--default":
			stdout.Write(DefaultFile())
			return exitOK
		default:
			// The hook refuses on anything it doesn't understand, its own words included.
			return deny(stdout, fmt.Sprintf("Refused by Smolcorp's hire guard: unknown word %q in its own command line.", args[i]))
		}
	}
	if rules == "" {
		path, err := supportFile()
		if err != nil {
			if printing {
				fmt.Fprintln(stderr, err)
				return exitUsage
			}
			return deny(stdout, "Refused by Smolcorp's hire guard: "+err.Error()+".")
		}
		rules = path
	}
	// The file is read once per call: held, it only adds to the groups given (Held).
	var groups []Group
	var problem string
	if holding {
		groups, problem = Held(held, rules, profile)
	} else {
		groups, problem = Profiled(rules, profile)
	}
	if printing {
		out, _ := json.MarshalIndent(map[string]any{"groups": groups, "file": rules, "problem": problem}, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK
	}
	return Hook(stdin, stdout, groups, problem)
}
