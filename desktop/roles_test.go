package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRolesCometWithTheAppUntilYouChangeThem(t *testing.T) {
	isolate(t)
	if roles := loadConfig().Roles; !sameRoles(roles, shippedRoles) {
		t.Fatalf("a new config has roles %v, want the shipped ones", roles)
	}
	writeFile(t, configFile, `{"roles": []}`)
	if roles := loadConfig().Roles; roles == nil || len(roles) != 0 {
		t.Errorf("all roles taken out load as %v, want none", roles)
	}
	writeFile(t, configFile, `{"roles": "not a list"}`)
	if roles := loadConfig().Roles; !sameRoles(roles, shippedRoles) {
		t.Errorf("roles that can't be read load as %v, want the shipped ones", roles)
	}
}

func TestARoleKeepsOnlyWhatCanBeKept(t *testing.T) {
	var roles []Role
	if err := json.Unmarshal([]byte(`[
		{"id": "reviewer", "name": "  Reviewer\nwith a newline that is far too long  ", "agent": "nope", "model": "opus; rm -rf ~",
		 "effort": "max", "instructions": "a\u0007b\nc", "badge": "glasses"},
		{"id": "reviewer", "name": "Duplicate"}, {"id": "Bad Id", "name": "x"}, {"id": "own-1", "name": ""},
		{"id": "own-2", "name": "Gemini one", "agent": "gemini", "model": "gemini-2.5-pro", "effort": "high", "badge": "unknown"},
		{"id": "own-3", "name": "Long model", "model": "claude-opus-5-5[1m]"}, 5, "text"]`), &roles); err != nil {
		t.Fatal(err)
	}
	want := []Role{
		{ID: "reviewer", Name: "Reviewer with a newline", Agent: "claude", Effort: "max", Instructions: "ab\nc", Badge: "glasses"},
		{ID: "own-2", Name: "Gemini one", Agent: "gemini", Model: "gemini-2.5-pro"},
		{ID: "own-3", Name: "Long model", Agent: "claude", Model: "claude-opus-5-5[1m]"},
	}
	if got := checkedRoles(roles); !sameRoles(got, want) {
		t.Errorf("roles kept\n%+v\nwant\n%+v", got, want)
	}
	var many []Role
	for i := range 20 {
		many = append(many, Role{ID: fmt.Sprintf("own-%d", i), Name: "Own"})
	}
	if kept := checkedRoles(append(many, shippedRoles...)); len(kept) != ownRolesMost+len(shippedRoles) {
		t.Errorf("20 of your own and the shipped ones keep %d, want %d", len(kept), ownRolesMost+len(shippedRoles))
	}
}

func TestEachAgentStartsOnTheRole(t *testing.T) {
	for _, c := range []struct {
		role Role
		want []string
	}{
		{Role{Agent: "claude", Model: "opus", Effort: "high"}, []string{"--model=opus", "--effort=high", "--", "-hi"}},
		{Role{Agent: "claude"}, []string{"--", "-hi"}},
		{Role{Agent: "codex", Model: "gpt-5"}, []string{"--model=gpt-5", "--", "-hi"}},
		{Role{Agent: "gemini", Model: "pro"}, []string{"--model=pro", "--prompt-interactive=-hi"}},
		{Role{Agent: "opencode", Model: "anthropic/claude"}, []string{"--model=anthropic/claude", "--prompt=-hi"}},
		{Role{Agent: "grok", Model: "grok-4"}, nil},
	} {
		if got := c.role.arguments("-hi"); !slices.Equal(got, c.want) {
			t.Errorf("%s starts with %q, want %q", c.role.Agent, got, c.want)
		}
	}
}

// sameRoles is whether two lists hold the same roles, in the same order.
func sameRoles(a, b []Role) bool {
	return slices.EqualFunc(a, b, Role.sameAs)
}

func TestARoleOnlyAsksBeforeItStarts(t *testing.T) {
	isolate(t)
	// In this system's own separators: ~\work\api on Windows.
	homeDir = filepath.Join(string(filepath.Separator), "home", "someone")
	message := shippedRoles[1].firstMessage(filepath.Join(homeDir, "work", "api"))
	want := "Your first reply must only ask me the questions below, all together. Don't read code, run commands or start any " +
		"work until I've answered them.\n\n" +
		"1. You've started in " + filepath.Join("~", "work", "api") + ". Is that the one to work on?\n" +
		"2. Which tests should you run: all of them, a folder, or a single file?\n" +
		"3. Is there anything to set up or start first, such as a database, a server or environment variables?\n\n" +
		"Once I've answered, as my Tester: Run the tests I point you to."
	if !strings.HasPrefix(message, want) {
		t.Errorf("the tester is first told\n%s\nwant it to start\n%s", message, want)
	}
	if message := (Role{Name: "Blank"}).firstMessage("/srv/app"); !strings.HasSuffix(message, "\n\nOnce I've answered, ask me what to do.") ||
		!strings.Contains(message, "Is that the one to work on?") {
		t.Errorf("a role with no questions or instructions is told %q, want it to ask about the folder, then what to do", message)
	}
}

func TestRolesAsShippedIn171AreBroughtUpToDate(t *testing.T) {
	isolate(t)
	edited := shippedIn171[1]
	edited.Model = "haiku"
	saved, _ := json.Marshal(map[string]any{"roles": []any{shippedIn171[0], edited, shippedIn171[4],
		map[string]any{"id": "own-1", "name": "Security", "questions": []any{"Which repo?", "", "  Any area?  ", 5}}}})
	writeFile(t, configFile, string(saved))
	roles := loadConfig().Roles
	if len(roles) != 4 || !roles[0].sameAs(shippedRoles[0]) || !roles[2].sameAs(shippedRoles[4]) {
		t.Errorf("1.7.1's reviewer and refactorer load as\n%+v\n%+v\nwant today's", roles[0], roles[2])
	}
	if !roles[1].sameAs(func() Role { r, _ := edited.checked(); return r }()) {
		t.Errorf("a tester you'd changed loads as %+v, want it as you had it", roles[1])
	}
	if !slices.Equal(roles[3].Questions, []string{"Which repo?", "Any area?"}) {
		t.Errorf("your own role's questions load as %q", roles[3].Questions)
	}
}

func TestARoleIsStartedFromSetupsRoles(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"roles": [{"id": "tester", "name": "Tester", "agent": "codex", "instructions": "Run the tests."}]}`)
	args, err := roleArguments("tester")
	if err != nil || len(args) != 3 || args[0] != "codex" || args[1] != "--" || !strings.HasSuffix(args[2], "Once I've answered, as my Tester: Run the tests.") {
		t.Errorf("the tester starts as %q, %v", args, err)
	}
	if _, err := roleArguments("nobody"); err == nil || !strings.Contains(err.Error(), "Yours are: tester") {
		t.Errorf("an unknown role gave %v, want the ones there are", err)
	}
}

func TestCmdIsNeverHandedWhatItWouldActOn(t *testing.T) {
	ready, err := forCmd([]string{"/c", `C:\npm\claude.cmd`, "--", "line one\n\nline two: don't stop"})
	if err != nil || ready[3] != "line one line two: don't stop" {
		t.Errorf("a plain message is readied as %q, %v", ready, err)
	}
	for _, message := range []string{`say "hi"`, "100%", "a & b", "a | b", "<b>", "a^b"} {
		if _, err := forCmd([]string{"/c", "claude.cmd", "--", message}); err == nil {
			t.Errorf("%q would go to cmd.exe", message)
		}
	}
}

func TestADeskShowsTheRoleItsLauncherWasStartedWith(t *testing.T) {
	d := newTestDeck(t)
	d.config.Roles = slices.Clone(shippedRoles)
	d.launcherConnected("L1", Claude, "/work/api", "tester")
	d.launcherConnected("L2", Claude, "/work/web", "")
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s1", "cwd": "/work/api"},
		Claude, "launcher:L1", "", 100)
	d.sessions.Apply(map[string]any{"hook_event_name": "SessionStart", "source": "startup", "session_id": "s2", "cwd": "/work/web"},
		Claude, "launcher:L2", "", 101)
	var state struct {
		Keys []struct {
			Session *struct {
				ID   string         `json:"id"`
				Role map[string]any `json:"role"`
			} `json:"session"`
		} `json:"keys"`
	}
	read := func() map[string]map[string]any {
		if err := json.Unmarshal(d.snapshot(), &state); err != nil {
			t.Fatal(err)
		}
		roles := map[string]map[string]any{}
		for _, key := range state.Keys {
			if key.Session != nil {
				roles[key.Session.ID] = key.Session.Role
			}
		}
		return roles
	}
	roles := read()
	if role := roles["s1"]; role["id"] != "tester" || role["name"] != "Tester" || role["badge"] != "clipboard" || role["instructions"] != nil {
		t.Errorf("the tester's desk has role %v, want its id, name and badge only", role)
	}
	if roles["s2"] != nil {
		t.Errorf("a plain agent's desk has role %v", roles["s2"])
	}
	d.config.Roles = nil
	if role := read()["s1"]; role != nil {
		t.Errorf("a role taken out of setup still shows as %v", role)
	}
}

func TestSetupKeepsTheRolesItIsGiven(t *testing.T) {
	d := newTestDeck(t)
	response := serve(t, d, call{method: "POST", target: "/setup/roles", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"},
		body:    `[{"id": "own-1", "name": "Security", "agent": "claude", "model": "opus", "instructions": "Look for leaks.", "badge": "shield"}]`})
	if response.Code != 200 || len(d.config.Roles) != 1 || d.config.Roles[0].Badge != "shield" {
		t.Fatalf("saving a role gave %d, roles %+v", response.Code, d.config.Roles)
	}
	data, _ := os.ReadFile(filepath.Join(supportDir, "config.json"))
	if !strings.Contains(string(data), `"instructions": "Look for leaks."`) {
		t.Errorf("the role isn't in the saved config:\n%s", data)
	}
	if response := serve(t, d, call{method: "POST", target: "/setup/roles", host: "localhost:7710", from: thisComputer,
		headers: map[string]string{setupHeader: "1"},
		body:    `{"not": "a list"}`}); response.Code != 400 || len(d.config.Roles) != 1 {
		t.Errorf("something that isn't roles gave %d, roles %+v", response.Code, d.config.Roles)
	}
}

func TestCmdExeStartsAScriptWhosePathHasASpace(t *testing.T) {
	got := cmdLine(`C:\Windows\System32\cmd.exe`, `C:\Users\Jean Dupont\AppData\Roaming\npm\codex.cmd`,
		[]string{"--model", "o4", "Ask which folder to work in", ""})
	want := `"C:\Windows\System32\cmd.exe" /s /c ""C:\Users\Jean Dupont\AppData\Roaming\npm\codex.cmd" --model o4 "Ask which folder to work in" """`
	if got != want {
		t.Fatalf("the command line is\n%s\nwant\n%s", got, want)
	}
}
