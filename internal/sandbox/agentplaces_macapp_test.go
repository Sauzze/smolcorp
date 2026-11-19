package sandbox

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// FXAGENTHOMEMAC: on a Mac the app starts the core with only the variables GoCore.inherited names
// (Sources/AgentDeck/Engine.swift). Every variable AgentPlaces reads must be among them, or the core never sees an
// agent's folder the owner moved and a desk's sandbox doesn't hold it. The app's list, GoCore.agentPlaces, is this
// package's agentHomeVariables (each name once), agentSignInVariables, then shellPlaceVariables (ZDOTDIR, FXSWEEP3P1), in
// that order.
func TestTheMacAppPassesTheCoreEveryAgentPlaceVariable(t *testing.T) {
	var want []string
	for _, v := range agentHomeVariables {
		if !slices.Contains(want, v.Variable) {
			want = append(want, v.Variable)
		}
	}
	want = append(want, agentSignInVariables...)
	want = append(want, shellPlaceVariables...)
	if !slices.Equal(AgentPlaceVariables(), want) {
		t.Errorf("AgentPlaceVariables: %q, want %q", AgentPlaceVariables(), want)
	}

	data, err := os.ReadFile(filepath.Join("..", "..", "Sources", "AgentDeck", "Engine.swift"))
	if err != nil {
		t.Fatal(err)
	}
	swift := string(data)
	at := strings.Index(swift, "static let agentPlaces = [")
	if at < 0 {
		t.Fatal("GoCore.agentPlaces is gone from Engine.swift: update this test")
	}
	list := swift[at:]
	list = list[:strings.Index(list, "]")+1]
	var got []string
	for _, m := range regexp.MustCompile(`"([^"]*)"`).FindAllStringSubmatch(list, -1) {
		got = append(got, m[1])
	}
	if !slices.Equal(got, want) {
		t.Errorf("the Mac app passes the core %q, but the core reads %q", got, want)
	}
	// …and the app's environment keeps them: GoCore.inherited ends with the list.
	if !regexp.MustCompile(`static let inherited = \[[^\]]*\] \+ agentPlaces\n`).MatchString(swift) {
		t.Error("GoCore.inherited no longer takes in agentPlaces")
	}
}

// FXAGENTHOMEMACR2 (RT126AGENTHOMEMACR1 N1): a place a start couldn't look through is put down to the variable whose
// whole-path value holds it, as given or resolved; not to one that holds home when the place is in home, nor any place
// in the agents' own folders or ~/.config, held without any variable, nor to a value that isn't a whole path.
func TestAPlaceTheStartCantLookThroughIsPutDownToItsVariable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the Mac's: its words are for a Mac's start, on its volume's names")
	}
	home := "/Users/ada"
	env := map[string]string{
		"XDG_CONFIG_HOME": "/Users", "CODEX_HOME": "/elsewhere/codex", "GROK_HOME": "rel/grok",
		"GROK_AUTH_PATH": "/linked/auth.json", "OPENCODE_DB": "",
	}
	resolve := func(p string) string {
		if rest, ok := strings.CutPrefix(p, "/linked"); ok {
			return "/private/real" + rest
		}
		return p
	}
	for place, want := range map[string]string{
		"/elsewhere/codex":                 "CODEX_HOME",
		"/elsewhere/codex/sessions/x":      "CODEX_HOME",
		"/Elsewhere/Codex/x":               "CODEX_HOME", // the Mac's volume ignores case
		"/elsewhere/codexes":               "",
		"/Users/bob/x":                     "XDG_CONFIG_HOME",
		"/Users/ada/dotfiles/x":            "", // in home: held whatever XDG_CONFIG_HOME says
		"/Users/ada/.codex/x":              "",
		"/Users/ada/.config/git":           "",
		"/private/real/auth.json":          "GROK_AUTH_PATH",
		"/linked/auth.json":                "GROK_AUTH_PATH",
		filepath.Join(home, "..", "bob/x"): "XDG_CONFIG_HOME",
		"/elsewhere/rel/grok":              "",
	} {
		if got := AgentPlaceOf(func(name string) string { return env[name] }, resolve, home, place); got != want {
			t.Errorf("AgentPlaceOf(%q) = %q, want %q", place, got, want)
		}
	}
}
