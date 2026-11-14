package desktop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// fs3Repo is a repository's git folder made by hand in a temp folder (no git runs), its HEAD on branch, with the
// objects/ and refs/ folders git wants.
func fs3Repo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	for _, folder := range []string{"objects", "refs"} {
		if err := os.MkdirAll(filepath.Join(dir, ".git", folder), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// hookSaysIn is hookSays for a call made in the folder cwd.
func hookSaysIn(t *testing.T, line, rules, profile, cwd string) string {
	t.Helper()
	call, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": line}, "cwd": cwd})
	var out bytes.Buffer
	guard.Run([]string{"--rules", rules, "--profile", profile}, bytes.NewReader(call), &out, &bytes.Buffer{})
	var answer struct {
		Out struct {
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	_ = json.Unmarshal(out.Bytes(), &answer)
	return answer.Out.Reason
}

// FS3 (Q10): a merge onto a base branch is refused for a hire, the GM, an MCP key and a night job, as the hook
// refuses it; the owner's own pass. The guard file Settings wrote before FS3 has no merge group: it holds anyway.
func TestAMergeOntoABaseBranchIsRefusedForEveryoneButTheOwner(t *testing.T) {
	onMain, onFeature := fs3Repo(t, "main"), fs3Repo(t, "feature")
	before := guardFile(t) // the six groups, as Settings writes them
	held := []actor{actorHire, hireActor("abc123"), actorGM, mcpActor("k"), nightActor("j")}
	for _, line := range []string{"git merge feature", "git pull", "git rebase feature", "git branch -f main x", "git fetch origin x:master"} {
		for _, rules := range []string{before, ""} {
			for _, a := range held {
				profile := guard.ProfileHire
				if a.heldAsGM() {
					profile = guard.ProfileGM
				}
				v := decideShell(a, shellAsk{line: line, rules: rules, cwd: onMain})
				if v.kind != verdictRefuse || !strings.Contains(v.why, "(merge: ") {
					t.Errorf("%q by %s: %+v", line, a, v)
				}
				if rules != "" && v.why != hookSaysIn(t, line, rules, profile, onMain) {
					t.Errorf("%q by %s: the hook says otherwise than %q", line, a, v.why)
				}
			}
			for _, a := range []actor{actorPage, actorTool} {
				if v := decideShell(a, shellAsk{line: line, rules: rules, cwd: onMain}); !v.allowed() {
					t.Errorf("%q by %s, the owner's own: %+v", line, a, v)
				}
			}
		}
	}
	// On a feature branch the hire merges main in, and with no folder the gate can't tell, so it refuses.
	if v := decideShell(actorHire, shellAsk{line: "git merge main", rules: before, cwd: onFeature}); !v.allowed() {
		t.Errorf("on a feature branch: %+v", v)
	}
	if v := decideShell(actorGM, shellAsk{line: "git merge main", rules: before}); v.allowed() || !strings.Contains(v.why, "can't tell") {
		t.Errorf("no folder: %+v", v)
	}
}

// Spend's real gate is the owner's OK: a line only the spend group refuses waits for the owner's tap, and the
// grant is for that line, once. A line that is refused for something else as well is refused.
func TestASpendLineAsksTheOwner(t *testing.T) {
	d := newTestDeck(t)
	rules, line := guardFile(t), "stripe --live refunds create --charge ch_1"
	for _, a := range []actor{actorHire, actorGM, mcpActor("k"), nightActor("j")} {
		v := decideShell(a, shellAsk{line: line, rules: rules})
		if v.kind != verdictAskOwner || v.what != spendAsk+line || v.status != http.StatusForbidden || v.allowed() {
			t.Errorf("%s: %+v", a, v)
		}
		profile := guard.ProfileHire
		if a.heldAsGM() {
			profile = guard.ProfileGM
		}
		if why := hookSays(t, line, rules, profile); !strings.Contains(why, "(spend: ") {
			t.Errorf("%s: the hook, which can't ask yet, says %q", a, why)
		}
	}
	for _, both := range []string{line + " && git push", "git push; " + line, line + "; vercel deploy"} {
		if v := decideShell(actorGM, shellAsk{line: both, rules: rules}); v.kind != verdictRefuse || strings.Contains(v.why, "(spend: ") {
			t.Errorf("%q: %+v", both, v)
		}
	}
	if v := decideShell(actorPage, shellAsk{line: line, rules: rules}); !v.allowed() {
		t.Errorf("the owner's own: %+v", v)
	}
	// Through the record: an ask waits, and nothing but the owner's tap grants it.
	if v := d.decide(nightActor("j"), action{shell: &shellAsk{line: line, rules: rules}}); v.allowed() || v.kind != verdictAskOwner {
		t.Errorf("the record: %+v", v)
	}
	// Switched off by the owner's file, spend asks nothing.
	off := guardFile(t)
	data, _ := os.ReadFile(off)
	data = bytes.Replace(data, []byte(`"groups":[`), []byte(`"groups":[{"name":"spend","on":false,"commands":[]},`), 1)
	if err := os.WriteFile(off, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if v := decideShell(actorGM, shellAsk{line: line, rules: off}); !v.allowed() {
		t.Errorf("spend off: %+v", v)
	}
}

// The gate groups add no deny rule: a deny rule would refuse a spend line before the hook could ask for it, and
// can't say "onto a base branch".
func TestTheGateGroupsAddNoDenyRule(t *testing.T) {
	var groups []guardGroup
	for _, g := range guard.GateGroups() {
		var commands []string
		for _, rule := range g.Commands {
			commands = append(commands, string(rule))
		}
		groups = append(groups, guardGroup{Name: g.Name, On: true, Commands: commands})
	}
	if rules := hireCommandRules(groups); len(rules) != 0 {
		t.Errorf("deny rules: %v", rules)
	}
}
