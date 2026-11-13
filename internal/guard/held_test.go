package guard

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// SANDBOX-PLAN §3.4 (SG1): a desk's groups come from the app's copy, in the guard's own command line, and the
// file only adds to them.

// hookAnswer runs the guard as the hook with args on a Bash call of command: "deny" and why, or "" for allow.
func hookAnswer(t *testing.T, args []string, command string) string {
	t.Helper()
	call, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": command}})
	var out bytes.Buffer
	if code := Run(args, bytes.NewReader(call), &out, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("%q exited %d", args, code)
	}
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
		return ""
	}
	return out.String()
}

// held is the groups as the app passes them in --groups: the default ones, with push switched off by the owner
// and a rule of the owner's own.
func heldForTests() string {
	groups := DefaultGroups()
	groups[0].On = false // push
	groups = append(groups, Group{Name: "extra", On: true, Commands: []Rule{"terraform import"}})
	return GroupsArgument(groups)
}

// g1: whatever the file says, it only adds. A group switched off there, a rule taken away, a broken group's
// absence: none of it lets anything through that the app's copy refuses.
func TestTheGuardFileOnlyAddsToTheGroupsInItsCommandLine(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, fileName)
	args := []string{"--groups", heldForTests(), "--rules", rules}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(rules, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// No file: the app's copy alone. Push stays off as the owner switched it, though the default has it on.
	if why := hookAnswer(t, args, "git push origin main"); why != "" {
		t.Errorf("push is off in the app's copy, but with no file it is refused: %s", why)
	}
	for _, line := range []string{"vercel deploy", "terraform import x y"} {
		if hookAnswer(t, args, line) == "" {
			t.Errorf("%s ran with no file", line)
		}
	}
	// A hire loosens the file: deploy off, the owner's rule gone. Nothing loosens.
	write(`{"groups":[{"name":"deploy","on":false,"commands":[]},{"name":"extra","on":false,"commands":[]}]}`)
	for _, line := range []string{"vercel deploy", "terraform import x y"} {
		if hookAnswer(t, args, line) == "" {
			t.Errorf("%s ran once the file was loosened", line)
		}
	}
	// The owner adds in the file: push switched on, a new group, a new rule. Each counts at once.
	write(`{"groups":[{"name":"push","on":true,"commands":[]},{"name":"mine","on":true,"commands":["helm install"]},{"name":"extra","on":true,"commands":["pulumi import"]}]}`)
	for _, line := range []string{"git push origin main", "helm install x", "pulumi import y", "terraform import x y"} {
		if hookAnswer(t, args, line) == "" {
			t.Errorf("%s ran once the file added it", line)
		}
	}
	// A group the file names but leaves off adds nothing.
	write(`{"groups":[{"name":"mine","on":false,"commands":["helm install"]}]}`)
	if why := hookAnswer(t, args, "helm install x"); why != "" {
		t.Errorf("a group left off in the file refused: %s", why)
	}
	// A broken file refuses everything, as it always has: never "allow".
	write("not a guard file")
	if hookAnswer(t, args, "ls") == "" {
		t.Error("a broken file let ls through")
	}
}

// Groups in the command line that can't be read, or given twice, refuse everything.
func TestGroupsThatCantBeReadRefuseEverything(t *testing.T) {
	rules := filepath.Join(t.TempDir(), fileName)
	for name, args := range map[string][]string{
		"not json":     {"--groups", "{", "--rules", rules},
		"no groups":    {"--groups", `{"groups":[]}`, "--rules", rules},
		"no on":        {"--groups", `{"groups":[{"name":"push","commands":["git push"]}]}`, "--rules", rules},
		"given twice":  {"--groups", heldForTests(), "--groups", `{"groups":[{"name":"x","on":false}]}`, "--rules", rules},
		"no value":     {"--rules", rules, "--groups"},
		"unknown word": {"--groups", heldForTests(), "--rules", rules, "--loose"},
	} {
		if hookAnswer(t, args, "ls") == "" {
			t.Errorf("%s: ls ran", name)
		}
	}
}

// The GM's profile is applied to the groups in force: its own smolcorp gm actions pass, the rest is refused.
func TestHeldGroupsTakeTheGMsProfile(t *testing.T) {
	rules := filepath.Join(t.TempDir(), fileName)
	args := []string{"--profile", ProfileGM, "--groups", GroupsArgument(DefaultGroups()), "--rules", rules}
	if why := hookAnswer(t, args, "smolcorp gm hire janitor"); why != "" {
		t.Errorf("the GM's own action refused: %s", why)
	}
	if hookAnswer(t, args, "git push") == "" {
		t.Error("the GM pushed")
	}
	if hookAnswer(t, args[2:], "smolcorp gm hire janitor") == "" {
		t.Error("a hire used the GM's action")
	}
}

// --print says the groups in force: the app's copy with what the file adds, so the deny rules the app makes and
// the hook's answers never disagree.
func TestPrintSaysTheGroupsInForce(t *testing.T) {
	rules := filepath.Join(t.TempDir(), fileName)
	if err := os.WriteFile(rules, []byte(`{"groups":[{"name":"push","on":true,"commands":[]},{"name":"deploy","on":false,"commands":[]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := Run([]string{"--print", "--groups", heldForTests(), "--rules", rules}, nil, &out, &bytes.Buffer{}); code != exitOK {
		t.Fatalf("exit %d", code)
	}
	var printed struct {
		Groups []Group `json:"groups"`
	}
	if err := json.Unmarshal(out.Bytes(), &printed); err != nil {
		t.Fatal(err)
	}
	on := map[string]bool{}
	for _, g := range printed.Groups {
		on[g.Name] = g.On
	}
	if !on["push"] || !on["deploy"] || !on["extra"] {
		t.Errorf("groups in force: %v", on)
	}
}

func TestOverKeepsTheHeldOrderAndAddsOnlyOnce(t *testing.T) {
	held := []Group{{Name: "a", On: true, Commands: []Rule{"x"}}, {Name: "b", On: false, Commands: []Rule{"y"}}}
	adds := []Group{{Name: "b", On: true, Commands: []Rule{"y", "z"}}, {Name: "a", On: false, Commands: []Rule{"x", "w"}}, {Name: "c", On: true, Commands: []Rule{"v"}}}
	got := Over(held, adds)
	want := []Group{{Name: "a", On: true, Commands: []Rule{"x", "w"}}, {Name: "b", On: true, Commands: []Rule{"y", "z"}}, {Name: "c", On: true, Commands: []Rule{"v"}}}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].On != want[i].On || !slices.Equal(got[i].Commands, want[i].Commands) {
			t.Errorf("%d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(held[0].Commands) != 1 {
		t.Error("the held groups were changed")
	}
}

// The ConfigChange hook refuses the account's, the project's and the local settings, and anything it can't read;
// managed settings (which can't be refused anyway) and skills pass.
func TestTheConfigChangeHookRefusesEverySettingsReload(t *testing.T) {
	for input, want := range map[string]int{
		`{"hook_event_name":"ConfigChange","source":"user_settings","file_path":"/h/.claude/settings.json"}`: exitBlock,
		`{"hook_event_name":"ConfigChange","source":"project_settings"}`:                                     exitBlock,
		`{"hook_event_name":"ConfigChange","source":"local_settings"}`:                                       exitBlock,
		`{"hook_event_name":"ConfigChange","source":"a_source_from_later"}`:                                  exitBlock,
		`{"hook_event_name":"ConfigChange","source":"policy_settings"}`:                                      exitOK,
		`{"hook_event_name":"ConfigChange","source":"skills"}`:                                               exitOK,
		`{"hook_event_name":"PreToolUse","source":"skills"}`:                                                 exitBlock,
		`not json`: exitBlock,
		``:         exitBlock,
	} {
		var stderr bytes.Buffer
		if got := Run([]string{"--config"}, strings.NewReader(input), &bytes.Buffer{}, &stderr); got != want {
			t.Errorf("%s: exit %d, want %d", input, got, want)
		}
		if want == exitBlock && !strings.Contains(stderr.String(), "next starts") {
			t.Errorf("%s: no reason given: %q", input, stderr.String())
		}
	}
	// --config with anything else is refused, never read as the tool-call hook.
	if got := Run([]string{"--config", "--rules", "x"}, strings.NewReader(`{"hook_event_name":"ConfigChange","source":"skills"}`), &bytes.Buffer{}, &bytes.Buffer{}); got != exitBlock {
		t.Errorf("--config with more words: exit %d", got)
	}
}
