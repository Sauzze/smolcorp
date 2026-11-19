// Package claudeenv takes out of an environment what a Claude Code session leaves in the environment of everything
// it starts (ENVSCRUB1). Smolcorp opened from a terminal inside a Claude Code session inherits that session's markers
// (CLAUDECODE, CLAUDE_CODE_ENTRYPOINT, CLAUDE_CODE_SSE_PORT, CLAUDE_CODE_CHILD_SESSION, CLAUDE_CODE_SESSION_ID…),
// and a desk's Claude Code started with them takes itself for that session's child: on Windows 1.24.2 a GM said
// "Transcript saving is off: inherited CLAUDE_CODE_CHILD_SESSION marker". Every start of an agent goes through
// Scrub, on the environment it inherits, before Smolcorp sets its own variables over it.
//
// The rule keeps a list, not drops one: CLAUDECODE and every CLAUDE_CODE_* name go, except the owner's own
// configuration below. So do the few names a session exports outside that namespace (sessionNames: CLAUDE_PID,
// CLAUDE_EFFORT, AI_AGENT), each by its exact name. A marker a later Claude Code adds is dropped without a change here; a setting of the owner's
// missing from the list makes a desk run on Claude Code's default for it, which is the safer of the two mistakes.
// The provider switches go by family (CLAUDE_CODE_USE_*, CLAUDE_CODE_SKIP_*_AUTH), so a new provider stays on.
// Their credentials (AWS_*, ANTHROPIC_*, GOOGLE_*, CLOUD_ML_REGION…) aren't CLAUDE_CODE_* names and are never
// looked at, and nor is CLAUDE_CONFIG_DIR: what each start sets or unsets of those is its own.
//
// It never touches a SMOLCORP_* variable (SIERRA, 15 Nov): SMOLCORP_GM_TOKEN above all, without which a GM can run
// no `smolcorp gm` command, and SMOLCORP_SESSION. What each start keeps or drops of those is its own, as before.
//
// Only the environment a start inherits is scrubbed: what Smolcorp sets on purpose (CLAUDE_CODE_DISABLE_AUTO_MEMORY
// for the GM, CLAUDE_CONFIG_DIR, a desk's settings' CLAUDE_CODE_SHELL) goes over it afterwards. Smolcorp's own
// process keeps its environment: `smolcorp gm`, run inside the GM's Claude Code, reads CLAUDE_CODE_SESSION_ID.
package claudeenv

import (
	"slices"
	"strings"
)

// kept are the CLAUDE_CODE_* names that are the owner's configuration, never a session's: a credential, where Git
// Bash is on Windows, the shell (which a desk's own settings set over, and agentcheck.go's wrapsHooks takes out of
// a desk's start), and switches the owner chose. Upper case.
var kept = []string{
	"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_API_KEY_HELPER_TTL_MS",
	"CLAUDE_CODE_CLIENT_CERT", "CLAUDE_CODE_CLIENT_KEY", "CLAUDE_CODE_CLIENT_KEY_PASSPHRASE",
	"CLAUDE_CODE_GIT_BASH_PATH",
	"CLAUDE_CODE_SHELL", "CLAUDE_CODE_SHELL_PREFIX",
	"CLAUDE_CODE_DISABLE_AUTO_MEMORY", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
	"CLAUDE_CODE_ENABLE_TELEMETRY", "CLAUDE_CODE_MAX_OUTPUT_TOKENS",
}

// sessionNames are what a Claude Code session exports to everything it starts outside the CLAUDE_CODE_* namespace
// (RT126ENVSCRUB1 B1): CLAUDECODE, the pid of the session's own claude, the session's effort level, and AI_AGENT,
// which names it. Each agent sets its own AI_AGENT, so a desk loses nothing without the inherited one. Exact names:
// CLAUDE_CONFIG_DIR and every other CLAUDE_* name stay the owner's. Upper case.
var sessionNames = []string{"CLAUDECODE", "CLAUDE_PID", "CLAUDE_EFFORT", "AI_AGENT"}

// definite are the markers only a running session sets, never an owner's shell profile: CLAUDECODE is in every
// session's children. Setup's line goes by these (Definite), so a Claude Code setting of the owner's that kept
// lacks is dropped from desks, as any CLAUDE_CODE_* name is, but never taken for a session. Upper case.
var definite = []string{
	"CLAUDECODE", "CLAUDE_PID", "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_ENTRYPOINT",
	"CLAUDE_CODE_SSE_PORT",
}

// Marker is whether a variable called name is a Claude Code session's, which no start of Smolcorp's passes on.
// Names are compared in upper case, as Windows does; on the other systems a lower-case one is dropped too.
func Marker(name string) bool {
	name = strings.ToUpper(name)
	if slices.Contains(sessionNames, name) {
		return true
	}
	rest, ours := strings.CutPrefix(name, "CLAUDE_CODE_")
	switch {
	case !ours:
		return false
	case slices.Contains(kept, name):
		return false
	case strings.HasPrefix(rest, "USE_"): // Bedrock, Vertex, Foundry, and any provider after them
		return false
	case strings.HasPrefix(rest, "SKIP_") && strings.HasSuffix(rest, "_AUTH"): // a provider's sign-in, done elsewhere
		return false
	}
	return true
}

// Definite is whether a variable called name is set only inside a running Claude Code session: what tells that
// Smolcorp itself was started in one. Every such name is a Marker.
func Definite(name string) bool {
	return slices.Contains(definite, strings.ToUpper(name))
}

// Scrub is env without a Claude Code session's markers. env isn't changed.
func Scrub(env []string) []string {
	return slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return Marker(name)
	})
}

// Found are the names of the markers in env, sorted, once each: none when it was started outside Claude Code.
func Found(env []string) []string {
	var found []string
	for _, entry := range env {
		if name, _, _ := strings.Cut(entry, "="); Marker(name) && !slices.Contains(found, name) {
			found = append(found, name)
		}
	}
	slices.Sort(found)
	return found
}
