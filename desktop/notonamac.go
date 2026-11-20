package desktop

import "slices"

// Codex, Gemini CLI, OpenCode and Grok hires on a Mac, in 1.26.0 (FXPLANTSTERMR2 and R3, the owner's rulings). A
// desk's sandbox can't write any of the owner's own folders for these agents (internal/sandbox's rowAgents, "a hired
// agent's own settings"): one that could would plant in what the owner's own sessions load. Codex and Gemini CLI don't
// start without writing there; Grok would lose its sessions and sign-in refresh; OpenCode writes its own state at
// start (1.19.6 makes its data, settings, state, log, bin and repos folders with no error caught, then writes its log
// and opens its database in its data folder). So their hires are refused, in words, before a window opens: on every
// hire route (hireWith) and again in the launcher (launch), and never started outside the sandbox instead. A home of
// their own in each desk lifts this in 1.26.1. Off a Mac, where no desk starts in a sandbox yet, or with the owner's
// switch off, they start as before.
var notOnAMacYetAgents = []Agent{Codex, Gemini, OpenCode, Grok}

// notOnAMacYet is why a hire of agent can't start where it would start in its sandbox (sandboxed: the core's
// deskSandboxInForce, the launcher's own sandbox for this hire), or "". With the owner's switch off, a hire starts
// without one, tagged, and isn't refused.
func notOnAMacYet(agent Agent, sandboxed bool) string {
	if !sandboxed || !slices.Contains(notOnAMacYetAgents, agent) {
		return ""
	}
	return agent.Name() + " hires can't start in Smolcorp's sandbox on a Mac yet, and with the sandbox on, Smolcorp never " +
		"starts a hire outside it. Hire a Claude Code role instead."
}
