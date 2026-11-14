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

// The gate service (gates.go) decides as the gates it wraps decided before it: every caller, every route, both
// manager switches; every line through the guard hook itself; every fenced hire as fenceRefusal says.

// oldSetupGate is server.go's gate for a setup route as it was before FS1, for a deck with no GM held to a scope:
// the table, then the manager's switch. 0 is "go on".
func oldSetupGate(who caller, method, path string, managerOn bool) (int, string) {
	if path != "/setup" && !routeAllows(who, method, path) {
		return http.StatusForbidden, routeRefusal(method, path)
	}
	if managerRefuses(method, path, managerOn) {
		return http.StatusConflict, managerOffAnswer
	}
	return 0, ""
}

func TestEveryRouteIsDecidedAsTheGatesItWrapsDecideIt(t *testing.T) {
	d := newTestDeck(t)
	paths := []string{"/setup", "/setup/state", "/setup/unknown", "/setup/gm/anything", "/setup/gm/", "/bots/x"}
	for _, rows := range [][]routeRow{routeTable, goOnlyRoutes} {
		for _, row := range rows {
			// A prefix row is tried with a path under it.
			if prefix, ok := strings.CutSuffix(row.path, "*"); ok {
				paths = append(paths, prefix+"x")
			} else {
				paths = append(paths, row.path)
			}
		}
	}
	callers := []caller{callerNobody, callerTool, callerGM, callerHire, callerPage}
	checked := 0
	for _, on := range []bool{false, true} {
		d.mu.Lock()
		d.config.ManagerEnabled = on
		d.mu.Unlock()
		for _, path := range paths {
			for _, method := range []string{"GET", "POST", "DELETE"} {
				for _, who := range callers {
					status, why := oldSetupGate(who, method, path, on)
					got := d.decide(actorOf(who), action{route: &routeAsk{method: method, path: path}})
					if got.allowed() != (status == 0) || got.status != status || got.why != why || got.body != nil {
						t.Errorf("%s %s by %s (manager %v): got %+v, want %d %q", method, path, who.name(), on, got, status, why)
					}
					checked++
				}
				// An MCP key and a night job have the GM's powers, never the page's; the night shift's jobs, which the
				// GM itself lists and proposes, are refused them (nightshift.go).
				gm := d.decide(actorGM, action{route: &routeAsk{method: method, path: path}})
				if isNightRoute(path) && gm.allowed() {
					gm = refuse(http.StatusForbidden, nightRefusal)
				}
				for _, a := range []actor{mcpActor("claude-desktop"), nightActor("nightly-tests")} {
					if got := d.decide(a, action{route: &routeAsk{method: method, path: path}}); got != gm {
						t.Errorf("%s %s by %s: got %+v, the GM gets %+v", method, path, a, got, gm)
					}
				}
				// Who can't be told is nobody.
				nobody := d.decide(actorNobody, action{route: &routeAsk{method: method, path: path}})
				for _, a := range []actor{"", "mcp:", "night:", "boss", "gm:x", "page:x"} {
					if got := d.decide(a, action{route: &routeAsk{method: method, path: path}}); got != nobody {
						t.Errorf("%s %s by %q: got %+v, nobody gets %+v", method, path, a, got, nobody)
					}
				}
			}
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d checks", checked)
	}
}

// guardFile writes a rules file into a test folder: the guard's own, with the groups named turned off.
func guardFile(t *testing.T, off ...string) string {
	t.Helper()
	var file struct {
		Groups []map[string]any `json:"groups"`
	}
	if err := json.Unmarshal(guard.DefaultFile(), &file); err != nil {
		t.Fatal(err)
	}
	for _, g := range file.Groups {
		for _, name := range off {
			if g["name"] == name {
				g["on"] = false
			}
		}
	}
	data, _ := json.Marshal(file)
	path := filepath.Join(t.TempDir(), hireGuardFile)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// hookSays is what `smolcorp guard` itself answers a Bash call of line: "" to allow, else its reason.
func hookSays(t *testing.T, line, rules, profile string) string {
	t.Helper()
	call, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": line}})
	var out, errs bytes.Buffer
	if code := guard.Run([]string{"--rules", rules, "--profile", profile}, bytes.NewReader(call), &out, &errs); code != 0 {
		t.Fatalf("guard: %d %s", code, errs.String())
	}
	if out.Len() == 0 {
		return ""
	}
	var answer struct {
		Out struct {
			Reason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &answer); err != nil || answer.Out.Reason == "" {
		t.Fatalf("guard said %q", out.String())
	}
	return answer.Out.Reason
}

func TestALineIsDecidedAsTheGuardHookDecidesItForEachActor(t *testing.T) {
	broken := filepath.Join(t.TempDir(), hireGuardFile)
	if err := os.WriteFile(broken, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"missing":  filepath.Join(t.TempDir(), hireGuardFile),
		"default":  guardFile(t),
		"push off": guardFile(t, "push", "local"),
		"broken":   broken,
	}
	lines := []string{"ls -la", "git status", "git push origin main", "git commit -m x && git push", "vercel deploy --prod",
		"npm publish", "smolcorp gm hire --folder /tmp/x", "smolcorp gm fire 3", "tmux send-keys -t 1 ls Enter", "ssh host", "aws s3 ls"}
	for name, rules := range files {
		for _, line := range lines {
			hire, gm := hookSays(t, line, rules, guard.ProfileHire), hookSays(t, line, rules, guard.ProfileGM)
			for a, want := range map[actor]string{actorHire: hire, actorGM: gm, mcpActor("k"): gm, nightActor("j"): gm, actorPage: "", actorTool: ""} {
				got := decideShell(a, shellAsk{line: line, rules: rules})
				if got.allowed() != (want == "") || got.why != want || got.status != 0 {
					t.Errorf("%s, %q by %s: got %+v, the hook says %q", name, line, a, got, want)
				}
			}
			if got := decideShell(actorNobody, shellAsk{line: line, rules: rules}); got.allowed() {
				t.Errorf("%s, %q by nobody: allowed", name, line)
			}
		}
	}
	// No file named is the guard's own groups, as the demo's fence checked them before FS1.
	for _, line := range lines {
		want := ""
		if r := guard.Check(line, guard.ForGM(guard.DefaultGroups())); r != nil {
			want = r.Error()
		}
		if got := decideShell(actorGM, shellAsk{line: line}); got.why != want {
			t.Errorf("%q, the guard's own groups: got %q, want %q", line, got.why, want)
		}
	}
}

func TestAFencedHireIsDecidedAsFenceRefusalDecidesIt(t *testing.T) {
	d := newTestDeck(t)
	for _, program := range []string{"claude", "codex", ""} {
		for _, pinned := range []string{"", "/pinned"} {
			for bits := 0; bits < 8; bits++ {
				ask := fenceAsk{program: program, pinned: pinned, tmux: bits&1 != 0, launcher: bits&2 != 0, macApp: bits&4 != 0}
				why := fenceRefusal(ask.program, ask.pinned, ask.tmux, ask.launcher, ask.macApp)
				for _, a := range []actor{actorGM, mcpActor("k"), nightActor("j"), actorPage, actorTool, actorHire} {
					got := d.decide(a, action{hire: &ask})
					if got.allowed() != (why == "") || got.why != why || why != "" && got.status != http.StatusConflict {
						t.Errorf("%+v by %s: got %+v, want %q", ask, a, got, why)
					}
				}
				if d.decide("mcp:", action{hire: &ask}).allowed() {
					t.Errorf("%+v by nobody: allowed", ask)
				}
			}
		}
	}
	if d.decide(actorGM, action{}).allowed() {
		t.Error("an empty action was allowed")
	}
}

// A shell gate goes after the guard, for an actor it holds; what it hands back, an ask included, is the verdict.
// None is there today: FS3's merge and spend groups are the guard's own (merge_test.go, gates_fs3_test.go).
func TestAShellGateComesAfterTheGuardAndMayAskTheOwner(t *testing.T) {
	if len(shellGates) != 0 {
		t.Fatalf("no gate of its own: %d", len(shellGates))
	}
	asked := 0
	shellGates = []func(actor, string) verdict{func(a actor, line string) verdict {
		asked++
		if strings.Contains(line, "release") {
			return askOwner("a release")
		}
		return allow()
	}}
	t.Cleanup(func() { shellGates = nil })
	rules := guardFile(t)
	if v := decideShell(nightActor("j"), shellAsk{line: "make release", rules: rules}); v.kind != verdictAskOwner ||
		v.what != "a release" || v.allowed() || v.status != http.StatusForbidden || v.why != askOwnerAnswer+"a release" {
		t.Errorf("a release: %+v", v)
	}
	if v := decideShell(actorGM, shellAsk{line: "git push origin main", rules: rules}); v.kind != verdictRefuse || asked != 1 {
		t.Errorf("a push: %+v, the gate asked %d times", v, asked)
	}
	if v := decideShell(actorPage, shellAsk{line: "make release", rules: rules}); !v.allowed() || asked != 1 {
		t.Errorf("the owner's own: %+v, the gate asked %d times", v, asked)
	}
}
