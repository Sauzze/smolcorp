package desktop

import "testing"

// Round 2 on RT126SWEEP3 part 4 (FXSWEEP3P4R2): the docs say what the GM can't change on /setup/hiring (N4), and
// which asks a Mac refuses and which it names (O2).
func TestTheDocsSayWhatTheGMAndAnAskOnAMacCantDo(t *testing.T) {
	api := repoText(t, "docs", "manager", "API.md")
	mustSay(t, "docs/manager/API.md", api, "On `/setup/hiring` the GM can turn hiring off but not back on, "+
		"and can't set the folder for hires or confirm an unsigned Claude Code: those are the owner's, in Setup.")
	mustNotSay(t, "docs/manager/API.md", api, "the GM can only turn hiring off")
	keys := repoText(t, "docs", "keys-and-guard.md")
	mustSay(t, "docs/keys-and-guard.md", keys, "an ask from a program in a sandbox is refused, and so is one no program "+
		"is still waiting on; when macOS can't tell whether the program that asked is in a sandbox, the window says it may be a hire.")
	mustNotSay(t, "docs/keys-and-guard.md", keys, "when macOS can't say which program asked")
}
