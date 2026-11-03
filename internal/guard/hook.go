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

// supportFile is where hire-guard.json lives: the Mac app's folder in
// Application Support, or the same folder desktop/paths.go uses.
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
	if problem != "" {
		// A broken file is never read as "allow": every command waits until it is fixed.
		return deny(stdout, "Refused by Smolcorp's hire guard: "+fileName+" is broken ("+problem+"). Ask the owner to fix it.")
	}
	if r := Check(*command, groups); r != nil {
		return deny(stdout, r.Error())
	}
	return exitOK
}

// Run is `smolcorp guard`:
//
//	guard [--profile P] [--rules FILE]
//	                               the hook: the tool call on stdin, the groups of FILE
//	                               (default: hire-guard.json in the support folder)
//	guard --print [--profile P] [--rules FILE]
//	                               the groups it uses now, as JSON, and what's wrong with FILE
//	guard --default                the default file, to start one from
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rules, profile, printing := "", ProfileHire, false
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
	groups, problem := Load(rules)
	if profile == ProfileGM {
		groups = ForGM(groups)
	}
	if printing {
		out, _ := json.MarshalIndent(map[string]any{"groups": groups, "file": rules, "problem": problem}, "", "  ")
		fmt.Fprintln(stdout, string(out))
		return exitOK
	}
	return Hook(stdin, stdout, groups, problem)
}
