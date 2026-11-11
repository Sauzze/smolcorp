package desktop

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// The hire start script (Phase 6 T602, with T704 and T705): what a hire's tmux
// window runs to start its agent in its worktree, with its own environment, as
// the Mac app's HireDesk.scriptLines and Role.startScript do. Nothing here is
// wired in yet: the Linux hire engine (worktree, ports, tmux window) builds a
// hireStart after making the worktree, calls writeHireStartScript, and has tmux
// run the file it names. The .claude.json editor (pre-trust, the API-key
// approval) is a separate change, claudejson.go.
//
// A hire never acts as the GM: the script unsets any SMOLCORP_GM_TOKEN it
// inherits from the app's environment before it starts the agent, and
// hireEnvironment (hire.go) strips it from the hire's tmux too. Only the GM's
// own script exports the token (GMToken, made new at its hire, as the Mac's
// HireDesk.scriptLines), and a GM never has a send key. Any other hire's own
// send key is the only secret it gets.

// shellQuoted is text as one word to a shell, whatever it holds.
func shellQuoted(text string) string { return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'" }

// testCopySources keeps the account's user settings out of a test copy's Claude Code.
const testCopySources = "--setting-sources=project,local"

// hireKeyEnvironment is the variable a hire signs its mail with (the CLI reads
// it, and with it set never reads a token file).
const hireKeyEnvironment = "SMOLCORP_HIRE_KEY"

// hireStart is everything one hire's start script holds.
type hireStart struct {
	Role    Role
	Message string // the first message
	// ConfigDir is the Claude account's config folder. With none, DefaultAccount
	// says ~/.claude's account, which clears any CLAUDE_CONFIG_DIR left over from
	// the terminal; with neither, no account was named and the script says nothing.
	ConfigDir      string
	DefaultAccount bool
	Settings       string // the hire's own Claude Code settings file, for Claude Code only
	CLIDir         string // the folder holding the app's own `smolcorp`, put on this agent's PATH
	Ports          string // "4100-4103"; "" for an unmanaged hire
	SendKey        string // the hire's own send key
	GMToken        string // the GM's token, for the gm role's script only
	APIKey         bool   // run on the API key in the secret store, never written down
	// A test copy: both set, and the port not one a real Smolcorp owns.
	TestPort       int
	TestSupportDir string
}

// realSmolcorpPorts are the ports of the real Smolcorps: a test copy never takes one.
var realSmolcorpPorts = []int{7710, 7720, 7730}

// agentOrClaude is the agent a program names, and Claude for one that isn't
// known, as the Mac does.
func agentOrClaude(program string) Agent {
	if agent := Agent(program); slices.Contains(agents, agent) {
		return agent
	}
	return Claude
}

// testCopyPort is the port of a test copy of Smolcorp, or 0 for the office's own.
func testCopyPort() int {
	if Port != portFromEnvironment("") {
		return Port
	}
	return 0
}

// testCopyRefusal is why a test copy won't hire program, or nil (T721). Claude
// on the default account, or on none named, would use the real office's login:
// refused before anything is made.
func testCopyRefusal(testPort int, program, configDir string) error {
	if testPort == 0 || agentOrClaude(program) != Claude {
		return nil
	}
	if configDir == "" {
		return &hireRefusal{Status: 409, Message: "This is a test copy of Smolcorp, so it never hires Claude on the default account: that is the real office's login. Hire from the real Smolcorp."}
	}
	// A Mac's test copy hires on a named account, as the Mac's Swift engine does (Deck.testCopyAccountRefusal):
	// its trust goes in that account's own .claude.json, and its hire reports through its settings file.
	if runtime.GOOS == "darwin" {
		return nil
	}
	return &hireRefusal{Status: 409, Message: "A test copy's hires on a named account aren't set up yet, so none is started."}
}

// sendKeyFor is the send key hire hireID's start script holds, made now by
// newSendKey (T726) against its registry row, so the hire must be written down
// first. The GM never has one (with a key and its token it would be taken for
// a hire, T713): "" with no error.
func sendKeyFor(role Role, hireID string) (string, error) {
	if role.ID == gmRole {
		return "", nil
	}
	return newSendKey(registryPath(), hireID)
}

// script is the start script, or why there isn't one (when in doubt, refuse).
func (h hireStart) script() (string, error) {
	agent := agentOrClaude(h.Role.Agent)
	claude := agent == Claude
	lines, err := h.environmentLines()
	if err != nil {
		return "", err
	}
	words := []string{string(agent)}
	if claude && h.Settings != "" {
		words = append(words, "--settings="+h.Settings)
	}
	// A test copy's hire doesn't load the account's own settings (the real app's hooks are in them):
	// its settings file reports to the copy.
	if claude && h.TestPort != 0 {
		words = append(words, testCopySources)
	}
	words = append(words, h.Role.arguments(h.Message)...)
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = shellQuoted(word)
	}
	environment := ""
	if claude {
		if h.ConfigDir != "" {
			environment = "export CLAUDE_CONFIG_DIR=" + shellQuoted(h.ConfigDir) + "\n"
		} else if h.DefaultAccount {
			environment = "unset CLAUDE_CONFIG_DIR\n"
		}
	}
	if h.CLIDir != "" {
		environment += "export PATH=" + shellQuoted(h.CLIDir) + ":\"$PATH\"\n"
	}
	return "#!/bin/sh\n# A hire from Smolcorp's office, as a " + startOneLine(h.Role.Name) + ": runs once.\nrm -f \"$0\"\nunset SMOLCORP_GM_TOKEN\n" +
		environment + lines + "exec " + strings.Join(quoted, " ") + "\n", nil
}

// environmentLines are the hire's exports: its ports, its send key, a test
// copy's port and folder (T704), and on the API-key account the key read from
// the secret store as the script runs, so it stops rather than fall back to
// the subscription.
func (h hireStart) environmentLines() (string, error) {
	lines := ""
	if h.GMToken != "" {
		if h.Role.ID != gmRole || h.SendKey != "" {
			return "", errors.New("only the General Manager's script holds its token, and never with a send key")
		}
		lines += "export SMOLCORP_GM_TOKEN=" + shellQuoted(h.GMToken) + "\n"
	}
	if h.Ports != "" {
		lines += "export SMOLCORP_HIRE_PORTS=" + shellQuoted(h.Ports) + "\n"
	}
	if h.SendKey != "" {
		lines += "export " + hireKeyEnvironment + "=" + shellQuoted(h.SendKey) + "\n"
	}
	if h.TestPort != 0 || h.TestSupportDir != "" {
		if h.TestPort <= 0 || h.TestPort > 65535 || h.TestSupportDir == "" {
			return "", errors.New("a test copy's hire needs both its port and its support folder")
		}
		for _, live := range realSmolcorpPorts {
			if h.TestPort == live {
				return "", fmt.Errorf("a test copy never uses port %d", live)
			}
		}
		lines += "export SMOLCORP_PORT=" + shellQuoted(strconv.Itoa(h.TestPort)) + "\n" +
			"export SMOLCORP_TEST_SUPPORT_DIR=" + shellQuoted(h.TestSupportDir) + "\n"
	}
	if h.APIKey {
		lines += "ANTHROPIC_API_KEY=\"$(secret-tool lookup service " + shellQuoted(summaryKeyService) +
			" username " + shellQuoted(summaryKeyAccount) + ")\" && [ -n \"$ANTHROPIC_API_KEY\" ] || " +
			"{ echo \"Smolcorp couldn't read the API key from the secret store.\"; exit 1; }\n" +
			"export ANTHROPIC_API_KEY\n"
	}
	return lines, nil
}

// oneLine keeps a role's name on the comment line it goes in.
func startOneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// writeHireStartScript writes script into folder (made 0700 if need be) as a
// file only its owner can read or run, and returns its path.
func writeHireStartScript(folder, script string) (string, error) {
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(folder, "hire-*.sh")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := file.Chmod(0o700); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	if _, err := file.WriteString(script); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
