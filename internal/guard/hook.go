package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sauzze/smolcorp/internal/appdirs"
	"github.com/Sauzze/smolcorp/internal/sandbox"
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

// stop prints Claude Code's answer that ends the session, for the PreToolUse hook (the call denied too) or after a
// call (PostToolUse, PostToolUseFailure: the call blocked). If it can't be written, the exit code says block (2).
func stop(stdout io.Writer, after bool, reason string) int {
	answer := map[string]any{"continue": false, "stopReason": reason}
	if after {
		answer["decision"], answer["reason"] = "block", reason
	} else {
		answer["hookSpecificOutput"] = map[string]any{"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": reason}
	}
	out, _ := json.Marshal(answer)
	if _, err := fmt.Fprintln(stdout, string(out)); err != nil {
		return exitUsage
	}
	return exitOK
}

// keptStop is why a Linux desk stops (L1GUARD1): a place no mount holds isn't as its start kept it in the folder
// held, or the copy can't be read. "" when all is as kept. Nothing found is touched here (only the core puts back,
// sandbox.RestoreKept and SaveKept): what stopped the desk is noted for its next start.
func keptStop(held string) string {
	changes, err := sandbox.CheckKept(held)
	why := ""
	if len(changes) > 0 {
		why = "Smolcorp stopped this desk. " + sandbox.KeptWords(changes)
		if nerr := sandbox.NoteKeptStop(held, changes); nerr != nil {
			why += "\n(What stopped it couldn't be noted for its next start: " + nerr.Error() + ".)"
		}
	}
	if err != nil {
		why = strings.TrimSpace(why + "\nSmolcorp stopped this desk: the places it may not change couldn't all be checked " +
			"against its start's copy (" + err.Error() + ").")
	}
	return why
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
//	guard ... --held DIR           the hook as above, but first a Linux desk's places no mount
//	                               holds compared with its start's copy in DIR (L1GUARD1): one
//	                               changed stops the desk, and is noted beside DIR (never touched)
//	guard --print [--profile P] [--groups JSON] [--rules FILE]
//	                               the groups it uses now, as JSON, and what's wrong with FILE
//	guard ... --settings FILE --sum HEX
//	                               the hook as above, but first the desk's settings file
//	                               checked against its seal (settings.go): one changed stops the desk
//	guard --after --settings FILE --sum HEX [--held DIR]
//	                               the PostToolUse hook: those checks alone, after each call; the
//	                               settings file must be named there, --held or not
//	guard --config                 the ConfigChange hook: a settings change on stdin, refused
//	                               unless it is managed settings' or a skill's (held.go)
//	guard --default                the default file, to start one from
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rules, profile, printing := "", ProfileHire, false
	held, holding := "", false
	settings, sum, kept, after := "", "", "", false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--settings", "--sum":
			if i+1 >= len(args) {
				return deny(stdout, "Refused by Smolcorp's hire guard: "+args[i]+" needs its value.")
			}
			if args[i] == "--settings" {
				settings = args[i+1]
			} else {
				sum = args[i+1]
			}
			i++
		case "--held":
			if i+1 >= len(args) || kept != "" || args[i+1] == "" {
				return deny(stdout, "Refused by Smolcorp's hire guard: --held needs its kept copy's folder, once.")
			}
			i++
			kept = args[i]
		case "--after":
			after = true
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
	if after { // the call's answer, read to its end so Claude Code's write of it never meets a closed pipe
		_, _ = io.Copy(io.Discard, stdin)
	}
	// Both checks below run before either stops the desk, and the stop gives every reason found: keptStop notes what
	// it found for the core to put back, so a settings file out of step must not keep it from running (RMSYLCHAIN B2).
	var whys []string
	// The desk's settings file, on both sides of each call: named with its seal, or not at all; after a call it must
	// be named, --held or not (SETTINGSFILE1: a guard told to check nothing after a call stops the desk).
	if settings != "" || sum != "" || after {
		var err error = errors.New("its name or its seal is missing from the guard's own command line")
		if settings != "" && sum != "" {
			err = CheckSettings(settings, sum)
		}
		if err != nil {
			whys = append(whys, settingsStop(err))
		}
	}
	// A Linux desk's places no mount holds, on both sides of each call too (a long Bash call can write after the check
	// before it), each check on its own: a settings file as sealed never excuses a kept place changed (L1GUARD1).
	if kept != "" {
		if why := keptStop(kept); why != "" {
			whys = append(whys, why)
		}
	}
	if len(whys) > 0 {
		return stop(stdout, after, strings.Join(whys, "\n"))
	}
	// After a call, nothing else is checked.
	if after {
		return exitOK
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
