package desktop

import (
	"encoding/json"
	"slices"
)

// KnownACPAgent is an agent that speaks the Agent Client Protocol, as the app ships it: how to start it, how it signs
// in, and whether Smolcorp's fence reaches what it does.
//
// The list is curated by hand and short. An entry says in Unsure whatever in it wasn't confirmed against the agent's own
// documentation (every entry does for now), never guessed at: an entry with an Unsure line is listed pending a check,
// and A5 must not hire it as if it were checked. A command is a program name looked
// up on the computer's PATH like any agent's: nothing here installs anything.
type KnownACPAgent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
	// Args follow Command. Each is a whole argument: they are never joined into a line or read by a shell.
	Args []string `json:"args"`
	// AuthMethods are the ids of the ways the agent is documented to sign in, for `authenticate`. The agent also
	// lists its own in its answer to `initialize`, and that list wins; this one is what setup can say before an agent
	// has been started. Empty where it isn't documented.
	AuthMethods []string `json:"authMethods"`
	// Unfenced: the agent's tools run inside itself, so Smolcorp's fence (the guard and the deny rules) never sees
	// them. Its hires carry unfencedNote's line, and the General Manager doesn't hire it (fenceRefusal).
	Unfenced bool `json:"unfenced"`
	// Unsure is what in the entry wasn't confirmed, in a sentence, or "" when all of it was.
	Unsure string `json:"unsure,omitempty"`
}

// knownACPAgents is the curated list, in the order setup shows it.
//
// Every agent here runs its own tools (Unfenced): none is known to send its commands and file access through the
// client's terminal/* and fs/* methods, which are the only ones A4's fence can see.
// Not listed, for want of a command and arguments that could be confirmed: Pi and Muse Code (the plan names them),
// and the adapters that wrap Claude Code or Codex. They can come with the registry, or in a later list.
var knownACPAgents = []KnownACPAgent{
	{
		ID: "gemini", Name: "Gemini CLI", Command: "gemini", Args: []string{"--experimental-acp"},
		AuthMethods: []string{"oauth-personal", "gemini-api-key", "vertex-ai"}, Unfenced: true,
		Unsure: "Newer Gemini CLI versions may name the flag --acp instead of --experimental-acp, and the sign-in ids " +
			"come from memory: check both against the version you have installed.",
	},
	{
		ID: "opencode", Name: "OpenCode", Command: "opencode", Args: []string{"acp"}, Unfenced: true,
		Unsure: "Its sign-in ids aren't documented here: the agent's own list in `initialize` is what counts.",
	},
	{
		ID: "kiro", Name: "Kiro", Command: "kiro-cli", Args: []string{"acp"}, Unfenced: true,
		Unsure: "The command and its argument are from memory, not from Kiro's documentation; its sign-in ids aren't " +
			"documented here.",
	},
	{
		ID: "goose", Name: "Goose", Command: "goose", Args: []string{"acp"}, Unfenced: true,
		Unsure: "The command and its argument are from memory, not from Goose's documentation; its sign-in ids aren't " +
			"documented here.",
	},
}

// knownACPAgent is the entry with id.
func knownACPAgent(id string) (KnownACPAgent, bool) {
	i := slices.IndexFunc(knownACPAgents, func(k KnownACPAgent) bool { return k.ID == id })
	if i < 0 {
		return KnownACPAgent{}, false
	}
	known := knownACPAgents[i]
	known.Args = slices.Clone(known.Args)
	known.AuthMethods = slices.Clone(known.AuthMethods)
	return known, true
}

// acpUnfenced says whether agent is a known ACP agent whose tools run inside itself.
func acpUnfenced(agent Agent) bool {
	known, ok := agent.acpKnown()
	return ok && known.Unfenced
}

// acpNotHiredYet is why an ACP agent isn't hired: hiring one is A5's. Said to the owner, and by the launcher and the
// start script, which have no caller to tell apart.
const acpNotHiredYet = "An ACP agent can't be hired yet: ACP hiring comes later in 1.26."

// acpHireRefusal is why a hire of program, an ACP agent, is refused. asGM: the hire is the General Manager's (the GM
// route, or the GM's token), which gets the fence's sentence, as its route would give it; the owner gets
// acpNotHiredYet, with the agent's name when the list has it. Never "": a hire of an ACP agent is always refused.
func acpHireRefusal(program string, asGM bool) string {
	if asGM {
		if why := fenceRefusal(program, "", false, false, false); why != "" {
			return why
		}
	}
	if name := Agent(program).Name(); name != "" {
		return name + " can't be hired yet: ACP hiring comes later in 1.26."
	}
	return acpNotHiredYet
}

// ACPRegistryEntry is one agent in the Agent Client Protocol's public registry, as setup's "browse the registry" will
// list it once something fetches that. A2 only gives it a shape: no code here reads the network. The registry's own
// format wasn't checked against its published schema, so what isn't listed is kept as it came (Distribution) and a
// field that is missing is empty, never an error.
type ACPRegistryEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Repository  string `json:"repository,omitempty"`
	// Distribution is how the registry says to get and start the agent (a package to run, or a binary per
	// platform), kept as it was sent. An entry from the registry is never started from it as it stands: it is
	// shown to the owner first, and an agent that isn't on the known list is taken to run its own tools.
	Distribution map[string]json.RawMessage `json:"distribution,omitempty"`
}

// mayBrowseACPRegistry says whether the owner turned on browsing the ACP registry in Settings. It is off by default,
// and while it is off nothing may look the registry up.
func (c *Config) mayBrowseACPRegistry() bool { return c.ACPRegistry }
