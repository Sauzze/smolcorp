package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/guard"
)

func TestRolesCometWithTheAppUntilYouChangeThem(t *testing.T) {
	isolate(t)
	if roles := loadConfig().Roles; !sameRoles(roles, shippedRoles) {
		t.Fatalf("a new config has roles %v, want the shipped ones", roles)
	}
	offered, _ := json.Marshal(shippedRoleIDs())
	writeFile(t, configFile, `{"roles": [], "rolesOffered": `+string(offered)+`}`)
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
	// As many of your own as you make: there's no most.
	if kept := checkedRoles(append(many, shippedRoles...)); len(kept) != len(many)+len(shippedRoles) {
		t.Errorf("20 of your own and the shipped ones keep %d, want %d", len(kept), len(many)+len(shippedRoles))
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
	message := shippedRoles[1].firstMessage(filepath.Join(homeDir, "work", "api"), startedHere)
	want := "Your first reply must only ask me the questions below, all together. Don't read code, run commands or start any " +
		"work until I've answered them.\n\n" +
		"1. You've started in " + filepath.Join("~", "work", "api") + ". Is that the one to work on?\n" +
		"2. Which tests should you run: all of them, a folder, or a single file?\n" +
		"3. What's the main thing someone must be able to do, for you to try by hand too?\n" +
		"4. What must be set up or started first, such as a database or a server? Say what's needed; don't send keys or passwords.\n\n" +
		"Once I've answered, as my Tester: Run the tests I point you to."
	if !strings.HasPrefix(message, want) {
		t.Errorf("the tester is first told\n%s\nwant it to start\n%s", message, want)
	}
	if message := (Role{Name: "Blank"}).firstMessage("/srv/app", startedHere); !strings.HasSuffix(message, "\n\nOnce I've answered, ask me what to do.") ||
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
	// From before rolesOffered: the roles that came after the first five are added, at the end.
	roles := loadConfig().Roles
	if len(roles) != 4+len(shippedRoles)-5 || !roles[0].sameAs(shippedRoles[0]) || !roles[2].sameAs(shippedRoles[4]) {
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
	args, err := roleArguments("tester", "/work", startedHere, "")
	if err != nil || len(args) != 3 || args[0] != "codex" || args[1] != "--" || !strings.HasSuffix(args[2], "Once I've answered, as my Tester: Run the tests.") {
		t.Errorf("the tester starts as %q, %v", args, err)
	}
	if _, err := roleArguments("nobody", "/work", startedHere, ""); err == nil || !strings.Contains(err.Error(), "Yours are: tester") {
		t.Errorf("an unknown role gave %v, want the ones there are", err)
	}
	// An engine hire's launcher is handed the message the app made with its brief.
	if args, err := roleArguments("tester", "/work", hiredIntoGeneralFolder, "Read `~/BRIEF.md` and follow it."); err != nil ||
		args[len(args)-1] != "Read `~/BRIEF.md` and follow it." {
		t.Errorf("with the brief's message the tester starts as %q, %v", args, err)
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
	d.launcherConnected("L1", Claude, "/work/api", "tester", "")
	d.launcherConnected("L2", Claude, "/work/web", "", "")
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
	// As cross-spawn escapes it for Node, checked against its own code.
	got := cmdLine(`C:\Windows\System32\cmd.exe`, `C:\Users\Jean Dupont\AppData\Roaming\npm\codex.cmd`,
		[]string{"--model", "o4", "Ask which folder to work in", ""})
	want := `"C:\Windows\System32\cmd.exe" /d /s /c "C:\Users\Jean^ Dupont\AppData\Roaming\npm\codex.cmd ` +
		`^^^"--model^^^" ^^^"o4^^^" ^^^"Ask^^^ which^^^ folder^^^ to^^^ work^^^ in^^^" ^^^"^^^""`
	if got != want {
		t.Fatalf("the command line is\n%s\nwant\n%s", got, want)
	}
}

func TestCmdExeActsOnNothingInAnArgument(t *testing.T) {
	// Each of cmd.exe's characters is passed on, and quotes reach the agent as quotes.
	got := cmdLine("cmd.exe", `C:\npm\claude.cmd`, []string{`say "hi" & exit`, "100%", "a|b<c>d^e"})
	want := `"cmd.exe" /d /s /c "C:\npm\claude.cmd ^^^"say^^^ \^^^"hi\^^^"^^^ ^^^&^^^ exit^^^" ^^^"100^^^%^^^" ^^^"a^^^|b^^^<c^^^>d^^^^e^^^""`
	if got != want {
		t.Fatalf("the command line is\n%s\nwant\n%s", got, want)
	}
}

func TestBackslashesBeforeAQuoteAreDoubled(t *testing.T) {
	// Two backslashes, then a quote: four, then an escaped quote, so the argument
	// reads as two backslashes and a quote rather than ending there.
	got := cmdLine("cmd.exe", `C:\npm\gemini.bat`, []string{`two \\" end`, `ends \`})
	want := `"cmd.exe" /d /s /c "C:\npm\gemini.bat ^"two^ \\\\\^"^ end^" ^"ends^ \\^""`
	if got != want {
		t.Fatalf("the command line is\n%s\nwant\n%s", got, want)
	}
}

func TestRolesKeptBeforeBriefsAreGivenOne(t *testing.T) {
	isolate(t)
	edited := shippedRoles[2]
	edited.Instructions = "Hunt my way."
	saved, _ := json.Marshal(map[string]any{"roles": []any{shippedRoles[0], edited, shippedIn171[1],
		Role{ID: "own-1", Name: "Mine", Instructions: "Do it."}}})
	writeFile(t, configFile, string(saved))
	roles := loadConfig().Roles
	want := []RoleBrief{{Kind: "library", ID: "reviewer"}, {Kind: "text", Text: "Hunt my way."}, {Kind: "library", ID: "tester"},
		{Kind: "text", Text: "Do it."}}
	for i, brief := range want {
		if roles[i].Brief == nil || *roles[i].Brief != brief {
			t.Errorf("%s has the brief %+v, want %+v", roles[i].ID, roles[i].Brief, brief)
		}
	}
	if !roles[2].sameAs(shippedRoles[1]) {
		t.Errorf("1.7.1's tester loads as %+v, want today's", roles[2])
	}
	if fresh := checkedRoles(slices.Clone(shippedRoles)); fresh[0].Brief == nil || fresh[0].Brief.Kind != "library" {
		t.Errorf("a new config's reviewer has the brief %+v", fresh[0].Brief)
	}
}

// A config written by the Mac app keeps its briefs here, read and saved again.
func TestBriefsGoBackAndForthWithoutLoss(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"roles": [
		{"id": "reviewer", "name": "Reviewer", "instructions": "x", "brief": {"kind": "file", "text": "# Mine {{role}}\n", "fileName": "mine.md"}},
		{"id": "own-1", "name": "Mine", "instructions": "", "brief": {"kind": "library", "id": "security-auditor"}},
		{"id": "own-2", "name": "Two", "instructions": "", "brief": {"kind": "text", "text": "From the brief."}},
		{"id": "own-3", "name": "Three", "instructions": "Kept.", "brief": {"kind": "library", "id": "nope"}},
		{"id": "own-4", "name": "Four", "instructions": "Kept.", "brief": {"kind": "file", "text": "bin\u0000ary", "fileName": "a.md"}},
		{"id": "own-5", "name": "Five", "instructions": "Kept.", "brief": {"kind": "teleport"}}]}`)
	config := loadConfig()
	saved, err := json.Marshal(config.Roles)
	if err != nil {
		t.Fatal(err)
	}
	var back []Role
	json.Unmarshal(saved, &back)
	want := []RoleBrief{{Kind: "file", Text: "# Mine {{role}}\n", FileName: "mine.md"}, {Kind: "library", ID: "security-auditor"},
		{Kind: "text", Text: "From the brief."}, {Kind: "text", Text: "Kept."}, {Kind: "text", Text: "Kept."}, {Kind: "text", Text: "Kept."}}
	for i, brief := range want {
		if back[i].Brief == nil || *back[i].Brief != brief {
			t.Errorf("%s comes back with the brief %+v, want %+v", back[i].ID, back[i].Brief, brief)
		}
	}
	if back[2].Instructions != "From the brief." {
		t.Errorf("a text brief's role has the instructions %q", back[2].Instructions)
	}
	if !strings.Contains(string(saved), `"brief":{"kind":"file","text":"# Mine {{role}}\n","fileName":"mine.md"}`) {
		t.Errorf("the file brief is saved as\n%s", saved)
	}
}

func TestAPickedBriefFileIsCheckedAsTheMacAppChecksIt(t *testing.T) {
	if brief, ok := checkedBriefFile("\uFEFF# Mine\r\nLine\u0007\n", `C:\Briefs\mine.MD`); !ok || brief.Text != "# Mine\nLine\n" || brief.FileName != "mine.MD" {
		t.Errorf("a picked file is kept as %+v, %v", brief, ok)
	}
	for _, c := range []struct{ text, name string }{
		{" \n", "a.md"}, {strings.Repeat("x", roleBriefFileLimit+1), "a.md"}, {"bad \uFFFD", "a.md"}, {"# Mine", "a.pdf"}, {"# Mine", ""},
	} {
		if _, ok := checkedBriefFile(c.text, c.name); ok {
			t.Errorf("%q named %q is kept", c.text[:min(len(c.text), 10)], c.name)
		}
	}
}

func TestRolesThatComeWithAnUpdateAreAddedOnce(t *testing.T) {
	isolate(t)
	// A config from before the Art Director and the rest: given them once, at the end of yours.
	writeFile(t, configFile, `{"roles": [{"id": "own-1", "name": "Mine"}, {"id": "tester", "name": "Tester"}]}`)
	config := loadConfig()
	ids := []string{}
	for _, role := range config.Roles {
		ids = append(ids, role.ID)
	}
	want := append([]string{"own-1", "tester"}, shippedRoleIDs()[5:]...)
	if !slices.Equal(ids, want) {
		t.Errorf("an older config's roles load as %v, want %v", ids, want)
	}
	if !slices.Equal(config.RolesOffered, shippedRoleIDs()) {
		t.Errorf("it's saved as offered %v, want every shipped role", config.RolesOffered)
	}
	// One you then delete isn't given back.
	config.Roles = config.Roles[:1]
	config.save()
	if roles := loadConfig().Roles; len(roles) != 1 || roles[0].ID != "own-1" {
		t.Errorf("after deleting the shipped roles, roles load as %+v", roles)
	}
}

func TestARoleDressesItsAgents(t *testing.T) {
	var roles []Role
	if err := json.Unmarshal([]byte(`[{"id": "a", "name": "A", "outfit": "chef"}, {"id": "b", "name": "B", "outfit": "auto"},
		{"id": "c", "name": "C", "outfit": "ballgown"}, {"id": "d", "name": "D", "outfit": 3}, {"id": "e", "name": "E", "badge": "toque"}]`), &roles); err != nil {
		t.Fatal(err)
	}
	kept := checkedRoles(roles)
	for i, want := range []string{"chef", "", "", "", ""} {
		if kept[i].Outfit != want {
			t.Errorf("role %s wears %q, want %q", kept[i].ID, kept[i].Outfit, want)
		}
	}
	if kept[4].Badge != "toque" {
		t.Errorf("the Chef's badge became %q", kept[4].Badge)
	}
	if phone := kept[0].forPhone(); phone["outfit"] != "chef" || phone["instructions"] != nil {
		t.Errorf("the phone is told %v", phone)
	}
	for _, role := range shippedRoles {
		if role.Badge == "" || !slices.Contains(roleBadges, role.Badge) {
			t.Errorf("%s has badge %q, which isn't one a role can wear", role.ID, role.Badge)
		}
		if role.Outfit != "" && !slices.Contains(teamChoices["outfit"], role.Outfit) {
			t.Errorf("%s wears %q, which the office doesn't know", role.ID, role.Outfit)
		}
		if checked, ok := role.checked(); !ok || !checked.sameAs(role) {
			t.Errorf("%s isn't kept as it comes", role.ID)
		}
	}
}

func TestABriefsTextAndItsInstructionsStayTheSame(t *testing.T) {
	saved := []Role{{ID: "own-1", Name: "Mine", Instructions: "Old.", Brief: &RoleBrief{Kind: "text", Text: "Old."}}}
	oldPage := Role{ID: "own-1", Name: "Mine", Instructions: "Edited on the old page.", Brief: &RoleBrief{Kind: "text", Text: "Old."}}
	if got := syncedRoles([]Role{oldPage}, saved)[0]; got.Brief.Text != "Edited on the old page." {
		t.Errorf("an edit of the instructions gives the brief %+v", got.Brief)
	}
	newPage := Role{ID: "own-1", Name: "Mine", Instructions: "Old.", Brief: &RoleBrief{Kind: "text", Text: "Edited on the new page."}}
	if got := syncedRoles([]Role{newPage}, saved)[0]; got.Instructions != "Edited on the new page." {
		t.Errorf("an edit of the brief gives the instructions %q", got.Instructions)
	}
	if saved[0].Instructions != "Old." {
		t.Error("the saved roles changed")
	}
}

func TestEditingALibraryRolesInstructionsPutsItOnThem(t *testing.T) {
	saved := checkedRoles(slices.Clone(shippedRoles))
	edited := saved[0]
	edited.Instructions = "Review only the docs."
	if got := syncedRoles([]Role{edited}, saved)[0]; got.Brief == nil || *got.Brief != (RoleBrief{Kind: "text", Text: "Review only the docs."}) {
		t.Errorf("an edited library role has the brief %+v", got.Brief)
	}
	if got := syncedRoles([]Role{saved[1]}, saved)[0]; got.Brief.Kind != "library" {
		t.Errorf("an unchanged library role has the brief %+v", got.Brief)
	}
}

func TestShippedRolesKeptFromBeforeOutfitsGetTheirs(t *testing.T) {
	isolate(t)
	old := shippedBeforeOutfits()
	edited := old[1]
	edited.Name = "Docs"
	saved, _ := json.Marshal(map[string]any{"roles": []any{old[0], edited, old[2]}, "rolesOffered": shippedRoleIDs()})
	writeFile(t, configFile, string(saved))
	roles := loadConfig().Roles
	if roles[0].Outfit != "detective" || roles[2].Outfit != "hardhat" {
		t.Errorf("the Bug hunter and the Refactorer wear %q and %q, want detective and hardhat", roles[0].Outfit, roles[2].Outfit)
	}
	if roles[1].Outfit != "" || roles[1].Name != "Docs" {
		t.Errorf("a Docs writer you'd changed loads as %+v, want it as you had it", roles[1])
	}
}

// macRoles is the Mac app's Roles.swift, with Unix line ends: a Windows
// checkout may have turned them into \r\n.
func macRoles(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "Sources", "AgentDeck", "Roles.swift"))
	if err != nil {
		t.Skip("the Mac app's source isn't here")
	}
	return strings.ReplaceAll(string(source), "\r\n", "\n")
}

func TestTheRolesAreTheMacApps(t *testing.T) {
	// The Mac app's Roles.swift is the source: each shipped role, its badge and outfit, word for word.
	text := macRoles(t)
	// The instructions are Swift's lines, joined by their trailing backslashes.
	joined := regexp.MustCompile(` \\\n\s*`).ReplaceAllString(text, " ")
	for _, role := range shippedRoles {
		head := fmt.Sprintf(`Role(id: %q, name: %q, agent: %q, model: %q, effort: %q`, role.ID, role.Name, role.Agent, role.Model, role.Effort)
		if !strings.Contains(text, head) {
			t.Errorf("%s isn't as the Mac app has it", role.ID)
		}
		for _, question := range role.Questions {
			if !strings.Contains(text, fmt.Sprintf("%q", question)) {
				t.Errorf("%s asks %q, which the Mac app's doesn't", role.ID, question)
			}
		}
		tail := fmt.Sprintf(`badge: %q`, role.Badge)
		if role.Outfit != "" {
			tail += fmt.Sprintf(`, outfit: %q`, role.Outfit)
		}
		if !strings.Contains(text, tail+")") {
			t.Errorf("%s's badge and outfit aren't the Mac app's", role.ID)
		}
		if !strings.Contains(joined, role.Instructions) {
			t.Errorf("%s's instructions aren't the Mac app's", role.ID)
		}
	}
	for _, badge := range roleBadges {
		if !strings.Contains(text, fmt.Sprintf("%q", badge)) {
			t.Errorf("badge %s isn't the Mac app's", badge)
		}
	}
}

// The General Manager as it was saved before it ran a team, with its brief,
// comes up to date as it loads; one you changed doesn't.
func TestTheGeneralManagerFromBeforeTeamsIsBroughtUpToDate(t *testing.T) {
	saved := shippedBeforeTeams()[0]
	saved.Brief = &RoleBrief{Kind: "library", ID: "gm"}
	now := checkedRoles([]Role{saved})[0]
	gm := shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == "gm" })]
	if !slices.Equal(now.Questions, gm.Questions) || now.Instructions != gm.Instructions {
		t.Errorf("the old General Manager loads as %+v", now)
	}
	edited := saved
	edited.Instructions = "Plan only."
	if got := checkedRoles([]Role{edited})[0].Instructions; got != "Plan only." {
		t.Errorf("an edited General Manager loads with %q", got)
	}
}

// Every role the Mac app ships, Windows and Linux ship too, in its order:
// TestTheRolesAreTheMacApps checks each one here is the Mac's, this that none is missing.
func TestEveryRoleTheMacShipsShipsHere(t *testing.T) {
	text := macRoles(t)
	start := strings.Index(text, "static let shipped = [")
	if start < 0 {
		t.Fatal("Roles.swift has no \"static let shipped = [\": the test needs to learn its new shape")
	}
	end := strings.Index(text[start:], "\n    ]\n")
	if end < 0 {
		t.Fatal("Roles.swift's shipped roles have no closing \"    ]\" line: the test needs to learn its new shape")
	}
	var mac []string
	for _, found := range regexp.MustCompile(`Role\(id: "([a-z-]+)"`).FindAllStringSubmatch(text[start:start+end], -1) {
		mac = append(mac, found[1])
	}
	if here := shippedRoleIDs(); !slices.Equal(here, mac) {
		t.Errorf("Windows and Linux ship %v, the Mac %v", here, mac)
	}
}

// The Builder and the Designer are the shipped roles that make things, so
// they are held to the same fence as the rest: Claude Code (the only agent
// Smolcorp fences), they ask before they start, and their instructions have
// them wait for an OK before they change anything and never push, merge,
// publish or deploy by themselves.
func TestTheMakersAreFencedLikeTheOthers(t *testing.T) {
	for _, want := range []struct{ id, name string }{{"builder", "Builder"}, {"designer", "Designer"}} {
		i := slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == want.id })
		if i < 0 {
			t.Fatalf("no %s ships", want.name)
		}
		role := shippedRoles[i]
		if role.Name != want.name || role.Agent != "claude" {
			t.Errorf("the %s is %q on %q, want %s on Claude Code, the agent Smolcorp fences", want.name, role.Name, role.Agent, want.name)
		}
		if len(role.Questions) == 0 {
			t.Errorf("the %s asks nothing before it starts", want.name)
		}
		for _, fence := range []string{"wait for my OK", "Never push, merge, publish or deploy unless I tell you to"} {
			if !strings.Contains(role.Instructions, fence) {
				t.Errorf("the %s's instructions don't say %q: %q", want.name, fence, role.Instructions)
			}
		}
		if role.Brief != nil || role.defaultBrief().Kind != "library" || role.defaultBrief().ID != want.id {
			t.Errorf("the %s's brief comes as %+v, want the library's %s", want.name, role.defaultBrief(), want.id)
		}
	}
	// Like the others, they come last, in that order, before the Mini Manager, and a config from before them is given
	// them once.
	if ids := shippedRoleIDs(); !slices.Equal(ids[len(ids)-3:], []string{"builder", "designer", "minimanager"}) {
		t.Errorf("the roles end with %v, want the Builder, the Designer, then the Mini Manager", ids[len(ids)-3:])
	}
	isolate(t)
	writeFile(t, configFile, `{"roles": [{"id": "tester", "name": "Tester"}], "rolesOffered": `+
		`["reviewer","tester","bughunter","docs","refactorer","artdirector","lawyer","therapist","plumber","gm","janitor","firefighter",`+
		`"pilot","postman","accountant","marketer","securityguard","chef","thinker"]}`)
	roles := loadConfig().Roles
	if len(roles) != 4 || roles[1].ID != "builder" || roles[2].ID != "designer" || !roles[2].sameAs(shippedRoles[len(shippedRoles)-2]) ||
		roles[3].ID != "minimanager" {
		t.Errorf("a config from before the Builder loads %+v, want its tester, then the Builder, the Designer and the Mini Manager", roles)
	}
	writeFile(t, configFile, `{"roles": [{"id": "tester", "name": "Tester"}], "rolesOffered": `+
		`["reviewer","tester","bughunter","docs","refactorer","artdirector","lawyer","therapist","plumber","gm","janitor","firefighter",`+
		`"pilot","postman","accountant","marketer","securityguard","chef","thinker","builder"]}`)
	if roles := loadConfig().Roles; len(roles) != 3 || roles[1].ID != "designer" || roles[2].ID != "minimanager" {
		t.Errorf("a config from 1.26's Builder loads %+v, want its tester, then the Designer and the Mini Manager", roles)
	}
}

// The Mini Manager runs one project and hires no one itself: Claude Code (the
// agent Smolcorp fences), it asks before it starts, waits for an OK on its
// plan, asks for each hire, and never pushes, merges, publishes or deploys.
// Its brief is read-only and ends on a done line of its own; the guard refuses
// it, as any hire, the General Manager's own hiring and firing. A config from
// before it is given it once, at the end.
func TestTheMiniManagerHiresNoOneAndIsFenced(t *testing.T) {
	i := slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == "minimanager" })
	if i != len(shippedRoles)-1 {
		t.Fatalf("the Mini Manager is shipped at %d of %d, want last", i, len(shippedRoles))
	}
	role := shippedRoles[i]
	if role.Name != "Mini Manager" || role.Agent != "claude" || role.Badge != "flag" || len(role.Questions) != 3 {
		t.Errorf("the Mini Manager ships as %+v", role)
	}
	for _, fence := range []string{"wait for my OK", "you don't hire anyone", "ask the General Manager, or me, for each hire",
		"have a second agent review it", "Never push, merge, publish or deploy"} {
		if !strings.Contains(role.Instructions, fence) {
			t.Errorf("the Mini Manager's instructions don't say %q: %q", fence, role.Instructions)
		}
	}
	if brief := role.defaultBrief(); brief.Kind != "library" || brief.ID != "minimanager" {
		t.Errorf("the Mini Manager's brief comes as %+v, want the library's minimanager", brief)
	}
	text, ok := briefLibrary{files: bundledBriefs()}.source("minimanager")
	if !ok {
		t.Fatal("the Mini Manager's brief isn't in the library")
	}
	values := parseBriefFrontMatter(text).values
	if values["read_only"] != "true" || values["done_signal"] != "PROJECT DONE" || values["report"] != "PROJECT-BOARD" {
		t.Errorf("the Mini Manager's brief says %v", values)
	}
	for _, rule := range []string{"you hire no one yourself", "**wait for the user's OK** before you ask for a single hire",
		"Hire, fire or start anyone yourself", "never the user's approval"} {
		if !strings.Contains(text, rule) {
			t.Errorf("the Mini Manager's brief doesn't say %q", rule)
		}
	}
	for _, line := range []string{"smolcorp gm hire --role builder", "smolcorp gm fire builder-1", "smolcorp hire builder"} {
		if guard.Check(line, guard.DefaultGroups()) == nil {
			t.Errorf("a hire may run %q", line)
		}
	}
	isolate(t)
	writeFile(t, configFile, `{"roles": [{"id": "tester", "name": "Tester"}], "rolesOffered": `+mustJSON(t, shippedRoleIDs()[:len(shippedRoleIDs())-1])+`}`)
	if roles := loadConfig().Roles; len(roles) != 2 || !roles[1].sameAs(role) {
		t.Errorf("a config from 1.26's Designer loads %+v, want its tester and then the Mini Manager", roles)
	}
}

// The Bug hunter, the Tester, the Docs writer and the Firefighter as they came
// before 1.26 sharpened them, on their own brief or none, ask the new
// questions as they load; one you changed, or put on a brief of your own, is
// left as you have it. Their old questions are the Mac app's too.
func TestShippedRolesFromBefore126AskTheNewQuestions(t *testing.T) {
	old := shippedBefore126()
	if len(old) != 4 {
		t.Fatalf("%d roles from before 1.26, want 4", len(old))
	}
	text := macRoles(t)
	for _, role := range old {
		now := shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == role.ID })]
		if slices.Equal(role.Questions, now.Questions) || len(now.Questions) <= len(role.Questions) {
			t.Errorf("%s asks %v before 1.26 and %v now, want more now", role.ID, role.Questions, now.Questions)
		}
		for _, brief := range []*RoleBrief{nil, {Kind: "library", ID: role.ID}} {
			saved := role
			saved.Brief = brief
			if got := checkedRoles([]Role{saved})[0]; !got.sameAs(now) || got.Brief == nil || *got.Brief != (RoleBrief{Kind: "library", ID: role.ID}) {
				t.Errorf("%s from before 1.26, brief %+v, loads as %+v, want today's on its library brief", role.ID, brief, got)
			}
		}
		edited := role
		edited.Questions = append(slices.Clone(role.Questions), "And mine?")
		if got := checkedRoles([]Role{edited})[0]; !slices.Equal(got.Questions, edited.Questions) {
			t.Errorf("%s with a question of yours loads asking %v", role.ID, got.Questions)
		}
		own := role
		own.Brief = &RoleBrief{Kind: "text", Text: role.Instructions}
		if got := checkedRoles([]Role{own})[0]; !slices.Equal(got.Questions, role.Questions) || got.Brief.Kind != "text" {
			t.Errorf("%s on a brief of your own loads as %+v, want it as you had it", role.ID, got)
		}
		for _, question := range role.Questions {
			if !strings.Contains(text, fmt.Sprintf("%q", question)) {
				t.Errorf("%s asked %q before 1.26, which the Mac app's doesn't say", role.ID, question)
			}
		}
	}
	// One from before outfits too: it asked the old questions, and gets its outfit and the new ones.
	for _, role := range shippedBeforeOutfits() {
		if questions, ok := questionsBefore126[role.ID]; ok && !slices.Equal(role.Questions, questions) {
			t.Errorf("%s before outfits asks %v, want %v", role.ID, role.Questions, questions)
		}
		for _, question := range questionsBeforeOutfits[role.ID] {
			if !strings.Contains(text, fmt.Sprintf("%q", question)) {
				t.Errorf("%s asked %q before outfits, which the Mac app's doesn't say", role.ID, question)
			}
		}
		now := shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == role.ID })]
		if slices.Equal(role.Questions, now.Questions) {
			t.Errorf("%s before outfits asks today's questions, want the ones it asked then", role.ID)
		}
		if got := checkedRoles([]Role{role})[0]; !got.sameAs(now) {
			t.Errorf("%s from before outfits loads as %+v, want today's", role.ID, got)
		}
	}
}

// Every brief in the library ends on a done line of its own, so a watcher or a
// reader never takes one role's for another's: the Firefighter's is one line
// at the very end, never a second one after triage.
func TestEveryLibraryBriefHasADoneSignalOfItsOwn(t *testing.T) {
	library := briefLibrary{files: bundledBriefs()}
	seen := map[string]string{}
	for _, id := range briefLibraryIDs {
		text, ok := library.source(id)
		if !ok {
			t.Fatalf("%s isn't in the library", id)
		}
		done := parseBriefFrontMatter(text).values["done_signal"]
		if done == "" || strings.ToUpper(done) != done {
			t.Errorf("%s's done signal is %q, want one in capitals", id, done)
		}
		if other, ok := seen[done]; ok {
			t.Errorf("%s and %s both end on %q", other, id, done)
		}
		seen[done] = id
	}
	for _, gone := range []string{"FIRE OUT", "TRIAGE DONE", "HUNT DONE"} {
		if id, ok := seen[gone]; ok {
			t.Errorf("%s still ends on %q", id, gone)
		}
	}
	if seen["INCIDENT DONE"] != "firefighter" || seen["ISSUES SORTED"] != "issue-sorter" || seen["BUGS DONE"] != "bughunter" ||
		seen["PROJECT DONE"] != "minimanager" || seen["GOAL DONE"] != "gm" {
		t.Errorf("the Firefighter, the Issue sorter, the Bug hunter, the Mini Manager and the General Manager end on %v", seen)
	}
}

// Each company template is a General Manager and shipped roles only, so the
// hire queue (K2) and setup (K3) can hire it as it stands, and each is the
// owner's list.
func TestEveryCompanyTemplateHiresRolesThatExist(t *testing.T) {
	if len(companyTemplates) != 3 {
		t.Fatalf("%d templates ship, want 3", len(companyTemplates))
	}
	ids := map[string]bool{}
	for _, template := range companyTemplates {
		if template.ID == "" || ids[template.ID] || strings.TrimSpace(template.Name) == "" || strings.TrimSpace(template.Blurb) == "" {
			t.Errorf("template %+v needs a name, a blurb and an id of its own", template)
		}
		ids[template.ID] = true
		if template.GM != "gm" || !slices.Contains(shippedRoleIDs(), template.GM) {
			t.Errorf("%s's General Manager is %q, want the shipped gm", template.Name, template.GM)
		}
		if len(template.Roles) == 0 {
			t.Errorf("%s hires no roles besides the General Manager", template.Name)
		}
		for _, id := range template.Roles {
			if id == "gm" || !slices.Contains(shippedRoleIDs(), id) {
				t.Errorf("%s hires %q, which isn't a shipped role besides the General Manager", template.Name, id)
			}
		}
		// What setup sends and a hire reads must be a role as it comes: checked leaves it as it is.
		for _, id := range append([]string{template.GM}, template.Roles...) {
			role := shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == id })]
			if checked, ok := role.checked(); !ok || !checked.sameAs(role) {
				t.Errorf("%s's %s isn't kept as it comes", template.Name, id)
			}
		}
	}
	for _, want := range []struct {
		id    string
		name  string
		roles []string
	}{
		{"solo-builder", "Solo builder", []string{"builder", "reviewer", "tester"}},
		{"small-agency", "Small agency", []string{"builder", "builder", "designer", "reviewer", "docs"}},
		{"bug-squash", "Bug squash", []string{"bughunter", "tester", "reviewer"}},
	} {
		template, ok := companyTemplate(want.id)
		if !ok || template.Name != want.name || !slices.Equal(template.Roles, want.roles) {
			t.Errorf("%s is %+v, want %s with %v", want.id, template, want.name, want.roles)
		}
	}
	if _, ok := companyTemplate("nobody"); ok {
		t.Error("a template that doesn't exist was found")
	}
	// What the lookup gives is a copy: changing it leaves the templates alone.
	template, _ := companyTemplate("solo-builder")
	template.Roles[0] = "changed"
	if again, _ := companyTemplate("solo-builder"); again.Roles[0] != "builder" {
		t.Errorf("changing a looked-up template changed the shipped one: %v", again.Roles)
	}
}
