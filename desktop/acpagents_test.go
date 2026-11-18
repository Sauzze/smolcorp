package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestAnACPAgentKindRoundTrips(t *testing.T) {
	agent := ACPAgent("gemini")
	if string(agent) != "acp:gemini" || !agent.IsACP() || agent.Name() != "Gemini CLI" {
		t.Fatalf("ACPAgent(gemini) is %q, ACP %v, named %q", agent, agent.IsACP(), agent.Name())
	}
	if Claude.IsACP() || Gemini.IsACP() || Agent("").IsACP() || Agent("acp").IsACP() {
		t.Error("an agent that isn't an ACP one was read as one")
	}
	if slices.Contains(agents, agent) {
		t.Error("an ACP agent is among the agents that report through hooks")
	}
	// A role keeps it through JSON, as the setup page sends it and the config file keeps it.
	given := Role{ID: "navigator", Name: "Navigator", Agent: "acp:gemini", Model: "some-model", Effort: "high"}
	data, err := json.Marshal(given)
	if err != nil {
		t.Fatal(err)
	}
	var read Role
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	roles := checkedRoles([]Role{read})
	if len(roles) != 1 || roles[0].Agent != "acp:gemini" || roles[0].Effort != "" {
		t.Fatalf("the role came back as %+v: want the agent kept and no effort (an ACP agent has no effort dial)", roles)
	}
	// Through the config file, whole.
	isolate(t)
	config := loadConfig()
	config.Roles = append(config.Roles, roles[0])
	config.save()
	saved := loadConfig()
	i := slices.IndexFunc(saved.Roles, func(r Role) bool { return r.ID == "navigator" })
	if i < 0 || saved.Roles[i].Agent != "acp:gemini" {
		t.Fatalf("the saved config lost the ACP role (%d roles)", len(saved.Roles))
	}
	// One the list doesn't have is read as Claude Code, as any agent Smolcorp doesn't know, so the fence covers it.
	other := checkedRoles([]Role{{ID: "navigator", Name: "Navigator", Agent: "acp:not-on-the-list"}})
	if len(other) != 1 || other[0].Agent != string(Claude) {
		t.Errorf("an ACP agent that isn't known became %+v, want Claude Code", other)
	}
}

func TestTheKnownACPAgentsAreWellFormed(t *testing.T) {
	if len(knownACPAgents) == 0 {
		t.Fatal("no known ACP agent")
	}
	seen := map[string]bool{}
	for _, k := range knownACPAgents {
		if k.ID == "" || strings.ContainsFunc(k.ID, func(c rune) bool { return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') }) {
			t.Errorf("id %q isn't lowercase letters, digits and dashes", k.ID)
		}
		if seen[k.ID] {
			t.Errorf("id %q is listed twice", k.ID)
		}
		seen[k.ID] = true
		if k.Name == "" {
			t.Errorf("%s has no name", k.ID)
		}
		// A program name, looked up on PATH: no path, no spaces, nothing a shell would read.
		if k.Command == "" || strings.ContainsAny(k.Command, " \t\n/\\;&|$`'\"<>*?()") {
			t.Errorf("%s: command %q isn't a bare program name", k.ID, k.Command)
		}
		for _, arg := range k.Args {
			if arg == "" || strings.ContainsAny(arg, " \t\n;&|$`'\"<>*?()") {
				t.Errorf("%s: argument %q isn't one plain word", k.ID, arg)
			}
		}
		for _, method := range k.AuthMethods {
			if method == "" || strings.ContainsAny(method, " \t\n") {
				t.Errorf("%s: sign-in method %q isn't an id", k.ID, method)
			}
		}
		// Every one runs its own tools until the fence can reach its client methods, and the list says so.
		if !k.Unfenced {
			t.Errorf("%s isn't marked unfenced: nothing in A2 fences an ACP agent", k.ID)
		}
		// What isn't sure is said: an entry with no sign-in ids has to say why.
		if len(k.AuthMethods) == 0 && k.Unsure == "" {
			t.Errorf("%s has no sign-in ids and doesn't say they are unknown", k.ID)
		}
		got, ok := knownACPAgent(k.ID)
		if !ok || !reflect.DeepEqual(got, k) {
			t.Errorf("knownACPAgent(%s) is %+v, %v", k.ID, got, ok)
		}
	}
	if _, ok := knownACPAgent("gemini"); !ok {
		t.Error("Gemini CLI, named by the plan, isn't on the list")
	}
	if _, ok := knownACPAgent("nobody"); ok {
		t.Error("an agent that isn't listed was found")
	}
	// What knownACPAgent gives is a copy: changing it leaves the list as it was.
	k, _ := knownACPAgent("gemini")
	k.Args[0] = "--changed"
	if again, _ := knownACPAgent("gemini"); again.Args[0] == "--changed" {
		t.Error("the list's own arguments were changed through a copy")
	}
}

func TestAnUnfencedACPAgentIsRefusedForTheGMWithTheSentence(t *testing.T) {
	d := newTestDeck(t)
	want := "Gemini CLI runs its tools itself, so Smolcorp can't fence it: the General Manager doesn't hire it."
	for bits := 0; bits < 8; bits++ {
		for _, pinned := range []string{"", "/pinned"} {
			ask := fenceAsk{program: "acp:gemini", pinned: pinned, tmux: bits&1 != 0, launcher: bits&2 != 0, macApp: bits&4 != 0}
			if got := fenceRefusal(ask.program, ask.pinned, ask.tmux, ask.launcher, ask.macApp); got != want {
				t.Errorf("%+v: fenceRefusal is %q, want %q", ask, got, want)
			}
			for _, a := range []actor{actorGM, mcpActor("k"), nightActor("j"), actorPage, actorTool, actorHire} {
				if got := d.decide(a, action{hire: &ask}); got.allowed() || got.status != http.StatusConflict || got.why != want {
					t.Errorf("%+v by %s: got %+v, want a 409 with %q", ask, a, got, want)
				}
			}
		}
	}
	// One the list doesn't have is refused as any agent that isn't Claude Code.
	generic := fenceRefusal("acp:not-on-the-list", "", true, true, false)
	if generic != "The General Manager hires only Claude roles: Smolcorp can't fence another agent." {
		t.Errorf("an ACP agent that isn't known: %q", generic)
	}
	// And its hire carries today's line in the office, by name.
	if got := d.unfencedNote("navigator", "acp:gemini", nil, true); got != "Gemini CLI hires don't get Smolcorp's fence: they run with their own settings." {
		t.Errorf("the office's line is %q", got)
	}
	if got := d.unfencedNote("navigator", "acp:not-on-the-list", nil, true); got != "This hire doesn't get Smolcorp's fence: it runs with its own settings." {
		t.Errorf("the office's line for one that isn't known is %q", got)
	}
}

func TestAnACPRolesArgumentsAreItsEntrys(t *testing.T) {
	role := Role{ID: "navigator", Name: "Navigator", Agent: "acp:gemini", Model: "a-model", Effort: "high"}
	// The model and the first message are asked for through the protocol, never put in the arguments.
	got := role.arguments("--rm -rf; a first message")
	if want := []string{"--experimental-acp"}; !reflect.DeepEqual(got, want) {
		t.Errorf("arguments are %q, want %q", got, want)
	}
	command, args, ok := ACPAgent("opencode").acpLaunch()
	if !ok || command != "opencode" || !reflect.DeepEqual(args, []string{"acp"}) {
		t.Errorf("opencode starts as %q %q, %v", command, args, ok)
	}
	// A copy: appending to or changing what arguments gave leaves the list alone.
	got[0] = "--changed"
	if again := role.arguments(""); again[0] != "--experimental-acp" {
		t.Errorf("the list's arguments were changed through a copy: %q", again)
	}
	// One the list doesn't have starts with nothing, and a Claude Code role keeps its own.
	if got := (Role{Agent: "acp:not-on-the-list", Model: "m"}).arguments("hi"); len(got) != 0 {
		t.Errorf("an unknown ACP agent has arguments %q", got)
	}
	if _, _, ok := Claude.acpLaunch(); ok {
		t.Error("Claude Code has an ACP launch")
	}
	if got := (Role{Agent: "claude", Model: "m", Effort: "high"}).arguments("hi"); !reflect.DeepEqual(got, []string{"--model=m", "--effort=high", "--", "hi"}) {
		t.Errorf("a Claude Code role's arguments changed: %q", got)
	}
}

func TestBrowsingTheACPRegistryIsOffUntilTurnedOn(t *testing.T) {
	isolate(t)
	config := loadConfig()
	if config.ACPRegistry || config.mayBrowseACPRegistry() {
		t.Fatal("the registry may be browsed on a fresh config")
	}
	// A config file from before it, or one that says false, leaves it off; saving doesn't turn it on.
	writeFile(t, configFile, `{"hireFolder": "/work"}`)
	config = loadConfig()
	config.save()
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, set := fields["acpRegistry"]; set || loadConfig().mayBrowseACPRegistry() {
		t.Errorf("an old config turned it on: %s", data)
	}
	// Only the key, set to true, turns it on; one of another kind leaves it off.
	writeFile(t, configFile, `{"acpRegistry": true}`)
	if !loadConfig().mayBrowseACPRegistry() {
		t.Error("the key set to true didn't turn it on")
	}
	writeFile(t, configFile, `{"acpRegistry": "yes"}`)
	if loadConfig().mayBrowseACPRegistry() {
		t.Error("a key that isn't true or false turned it on")
	}
	if !knownConfigKeys()["acpRegistry"] {
		t.Error("the key isn't one the config reads, so a save would treat it as unknown")
	}
}

func TestAnACPRegistryEntryReadsWhateverTheRegistrySends(t *testing.T) {
	var entry ACPRegistryEntry
	data := `{"id": "an-agent", "name": "An Agent", "version": "1.2.3", "something": "new",
		"distribution": {"npx": {"package": "an-agent@1.2.3"}}}`
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.ID != "an-agent" || entry.Name != "An Agent" || entry.Version != "1.2.3" || entry.Description != "" || len(entry.Distribution["npx"]) == 0 {
		t.Errorf("the entry is %+v", entry)
	}
	// An entry from the registry is no known agent: it is not on the list, so it isn't started or fenced as one.
	if _, ok := knownACPAgent(entry.ID); ok || acpUnfenced(ACPAgent(entry.ID)) {
		t.Error("an agent from the registry was taken for a known one")
	}
}
