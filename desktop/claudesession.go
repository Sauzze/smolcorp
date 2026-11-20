package desktop

import (
	"os"
	"strings"

	"github.com/Sauzze/smolcorp/internal/claudeenv"
)

// Smolcorp opened from inside a Claude Code session (ENVSCRUB1). Every start of an agent leaves that session's markers
// out (claudeenv), so nothing depends on this; Setup only says so, in a line, so the owner knows why a desk doesn't
// behave like the terminal it was opened from.

// claudeSessionVariable is how the Mac app tells its core the names, never the values, of the CLAUDECODE and
// CLAUDE_CODE_* variables it was opened with (Engine.swift, GoCore.environment): the core gets only a closed list of
// the app's environment, so it can't see them itself. Which of them are a session's markers is claudeenv's rule
// alone. Comma-separated. Names only, so a desk that inherits it learns nothing: the scrub never touches a SMOLCORP_*
// variable (SIERRA, 15 Nov).
const claudeSessionVariable = "SMOLCORP_CLAUDE_NAMES"

// claudeSessionLine is Setup's line, in English and in French, as a held note's reason is (memoryHoldReasons).
var claudeSessionLine = [2]string{
	"Smolcorp was started from inside a Claude Code session. It doesn't pass that session's variables on to its desks: each desk's Claude Code starts as a session of its own.",
	"Smolcorp a été lancé depuis une session Claude Code. Il ne transmet pas les variables de cette session aux agents qu'il démarre : chacun ouvre sa propre session Claude Code.",
}

// openedInClaudeSession is whether this app was started inside a Claude Code session: with a marker only a running
// session sets (claudeenv.Definite), in its own environment (Linux, Windows, a bare core) or named by the Mac app
// (claudeSessionVariable). A Claude Code setting in the owner's profile is dropped from desks all the same, but
// never makes the line (RT126ENVSCRUB1 N2).
func openedInClaudeSession(getenv func(string) string, environ []string) bool {
	for _, entry := range environ {
		if name, _, _ := strings.Cut(entry, "="); claudeenv.Definite(name) {
			return true
		}
	}
	for _, name := range strings.Split(getenv(claudeSessionVariable), ",") {
		if claudeenv.Definite(strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

// claudeSessionState is the hire state's line (hireState), or nil when Smolcorp wasn't started inside a session.
func claudeSessionState() map[string]string {
	if !openedInClaudeSession(os.Getenv, os.Environ()) {
		return nil
	}
	return map[string]string{"en": claudeSessionLine[0], "fr": claudeSessionLine[1]}
}
