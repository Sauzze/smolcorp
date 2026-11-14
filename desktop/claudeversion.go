package desktop

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// Claude Code's version, checked before each desk starts (SANDBOX-PLAN §3.4, SG1). A desk's guard holds only
// because its hooks carry onFailure "block": a guard that is stalled, killed or wrapped then refuses the call
// rather than letting it through. A Claude Code that doesn't know onFailure would read the same settings and let
// every call through a dead guard, so it gets no desk.

// claudeLeastVersion is the oldest Claude Code a desk starts on: the oldest on which SG1's real sessions saw a
// stalled, killed or failing guard refuse the call (desktop/guardreal_test.go).
var claudeLeastVersion = [3]int{2, 1, 314}

// claudeVersion is the installed Claude Code's version, as `claude --version` says it, and whether it could be
// had. A variable, for the tests.
var claudeVersion = installedClaudeVersion

// installedClaudeVersion asks the claude a desk would start, found as the summaries find it.
func installedClaudeVersion() (string, bool) {
	program := programNamed("claude")
	if program == "" || checkAgentProgram(string(Claude), program) != nil {
		return "", false // never run: claudeVersionRefusal says why
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var out bytes.Buffer
	command := disclaim.Cmd(exec.CommandContext(ctx, program, "--version"))
	command.Stdout = &out
	if command.Run() != nil {
		return "", false
	}
	return out.String(), true
}

// versionNumbers reads the first x.y.z in text.
var versionNumbers = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// claudeVersionAtLeast is whether text names a Claude Code version no older than least.
func claudeVersionAtLeast(text string, least [3]int) bool {
	found := versionNumbers.FindStringSubmatch(text)
	if found == nil {
		return false
	}
	for i := range 3 {
		n, err := strconv.Atoi(found[i+1])
		if err != nil {
			return false
		}
		if n != least[i] {
			return n > least[i]
		}
	}
	return true
}

// claudeVersionRefusal is why no desk starts on the installed Claude Code, or nil.
func claudeVersionRefusal() *hireRefusal {
	text, ok := claudeVersion()
	if ok && claudeVersionAtLeast(text, claudeLeastVersion) {
		return nil
	}
	least := strconv.Itoa(claudeLeastVersion[0]) + "." + strconv.Itoa(claudeLeastVersion[1]) + "." + strconv.Itoa(claudeLeastVersion[2])
	if !ok {
		if err := checkAgentProgram(string(Claude), programNamed("claude")); err != nil {
			return &hireRefusal{Status: 409, Message: err.Error() + " No desk was started."}
		}
		return &hireRefusal{Status: 409, Message: "Update Claude Code: Smolcorp couldn't tell which version is installed, and a desk needs " +
			least + " or later to stop a call when its guard can't answer. No desk was started."}
	}
	return &hireRefusal{Status: 409, Message: "Update Claude Code: a desk needs " + least +
		" or later, which stops a call when its guard can't answer. No desk was started."}
}
