package guard

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The default refuses something from each of its groups, as a command
// word, inside a wrapper.
func TestTheDefaultRefusesEachGroup(t *testing.T) {
	cases := map[string][]string{
		"push":    {"git push", "sh -c 'git -C . push'"},
		"deploy":  {"vercel", "netlify deploy", "firebase deploy", "wrangler deploy", "fly deploy", "railway up", "heroku releases", "render deploy", "eas update", "supabase db push", "supabase functions deploy x", "kubectl -n x apply -f k.yaml", "kubectl delete pod x", "terraform -chdir=infra apply", "terraform destroy", "pulumi up", "docker push img", "npx netlify-cli deploy", "bash -c 'flyctl deploy'"},
		"publish": {"npm publish", "pnpm publish", "yarn publish", "cargo publish", "twine upload dist/*", "gem push x.gem", "fastlane beta", "xcrun altool --upload-app", "xcrun notarytool submit x", "gh pr merge 1", "gh release create v1", "env gh api -X POST x"},
		"cloud":   {"aws s3 ls", "gcloud run deploy", "az login", "eval 'aws sts get-caller-identity'"},
		"remote":  {"ssh host", "scp a host:b", "sftp host", "rsync -a . user@host:/srv", "rsync -a . rsync://host/x"},
		"local":   {"tmux send-keys -t x y", "smolcorp gm fire x", "security find-generic-password -w"},
	}
	groups := DefaultGroups()
	if len(groups) != len(cases) {
		t.Fatalf("%d default groups, %d tested", len(groups), len(cases))
	}
	for _, g := range groups {
		for _, line := range cases[g.Name] {
			r := Check(line, groups)
			if r == nil || r.Group != g.Name {
				t.Errorf("%s: %q gave %v", g.Name, line, r)
			}
		}
	}
	for _, line := range []string{"rsync -a src/ build/", "docker build .", "kubectl get pods", "terraform plan", "cargo build", "gem list"} {
		if r := Check(line, groups); r != nil {
			t.Errorf("refused %q: %v", line, r)
		}
	}
}

func TestAnEditedFileIsHonoured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hire-guard.json")
	edited := `{"groups": [
	  {"name": "push", "on": false, "commands": ["git push"]},
	  {"name": "mine", "on": true, "commands": ["make deploy", "just ship"]}
	]}`
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	groups, problem := Load(path)
	if problem != "" || len(groups) != 2 {
		t.Fatalf("%v, %q", groups, problem)
	}
	if r := Check("sh -c 'git push'", groups); r != nil {
		t.Errorf("push is off in the file: %v", r)
	}
	if r := Check("vercel", groups); r != nil {
		t.Errorf("vercel isn't in the file: %v", r)
	}
	if r := Check("true && make deploy", groups); r == nil || r.Group != "mine" {
		t.Errorf("the file's own rule: %v", r)
	}
	// The hook reads the file named on its command line.
	var out bytes.Buffer
	call := `{"tool_name":"Bash","tool_input":{"command":"just ship"}}`
	if code := Run([]string{"--rules", path}, strings.NewReader(call), &out, &bytes.Buffer{}); code != exitOK {
		t.Errorf("exit %d", code)
	}
	if ok, _ := denied(t, out.String()); !ok {
		t.Errorf("the edited file's rule: %q", out.String())
	}
	// What's always refused stays refused whatever the file says.
	if r := Check("cat gm-token", groups); r == nil {
		t.Error("the GM token")
	}
}

func TestABrokenFileFallsBackToTheDefaultNeverToNothing(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"not-json":   "{groups: [",
		"empty":      "",
		"no-groups":  `{"groups": []}`,
		"other":      `{"rules": ["x"]}`,
		"no-name":    `{"groups": [{"on": true, "commands": ["x"]}]}`,
		"blank-rule": `{"groups": [{"name": "a", "on": true, "commands": ["  "]}]}`,
		"twice":      `{"groups": [{"name": "a", "on": true}, {"name": "a", "on": false}]}`,
		"wrong-type": `{"groups": [{"name": "a", "on": "yes", "commands": []}]}`,
	} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		groups, problem := Load(path)
		if problem == "" {
			t.Errorf("%s: no problem said", name)
		}
		if Check("sh -c 'vercel deploy'", groups) == nil || Check("git push", groups) == nil {
			t.Errorf("%s: not the default", name)
		}
	}
	// No file at all: the default, and nothing wrong.
	if groups, problem := Load(filepath.Join(dir, "none.json")); problem != "" || Check("vercel", groups) == nil {
		t.Errorf("missing: %q", problem)
	}
	// The default file reads back as the default.
	groups, problem := Parse(DefaultFile())
	if problem != "" || len(groups) != len(DefaultGroups()) {
		t.Errorf("%q", problem)
	}
	// --print says which groups and what's wrong, as JSON.
	var out bytes.Buffer
	Run([]string{"--print", "--rules", filepath.Join(dir, "not-json.json")}, nil, &out, &bytes.Buffer{})
	var printed struct {
		Groups  []Group `json:"groups"`
		Problem string  `json:"problem"`
	}
	if json.Unmarshal(out.Bytes(), &printed) != nil || printed.Problem == "" || len(printed.Groups) != len(DefaultGroups()) {
		t.Errorf("%s", out.String())
	}
	// A word the guard doesn't know refuses.
	out.Reset()
	if code := Run([]string{"--rulez", "x"}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &bytes.Buffer{}); code != exitOK {
		t.Errorf("exit %d", code)
	}
	if ok, _ := denied(t, out.String()); !ok {
		t.Errorf("an unknown word: %q", out.String())
	}
	// The hook itself, with a broken file: every Bash command is denied, never allowed.
	out.Reset()
	Run([]string{"--rules", filepath.Join(dir, "not-json.json")}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &bytes.Buffer{})
	if ok, why := denied(t, out.String()); !ok || !strings.Contains(why, "broken") {
		t.Errorf("a broken file: %q", out.String())
	}
	// A file with a group off, read by the hook.
	off := filepath.Join(dir, "off.json")
	os.WriteFile(off, []byte(`{"groups":[{"name":"push","on":false,"commands":["git push"]}]}`), 0o600)
	out.Reset()
	Run([]string{"--rules", off}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git push"}}`), &out, &bytes.Buffer{})
	if out.Len() != 0 {
		t.Errorf("push is off: %q", out.String())
	}
}

// A fresh rules file (none written yet) has every group on: nothing is allowed until the owner switches it.
func TestAFreshRulesFileHasEveryGroupOn(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "hire-guard.json")} {
		groups, problem := Load(path)
		if problem != "" || len(groups) == 0 {
			t.Fatalf("%q: %d groups, problem %q", path, len(groups), problem)
		}
		for _, g := range groups {
			if !g.On {
				t.Errorf("%q: group %q is off in a fresh file", path, g.Name)
			}
		}
	}
}

// A test copy's folder comes only from the guard's own command line (--rules), never from
// SMOLCORP_TEST_SUPPORT_DIR, which a hire can set for itself (SANDBOX-PLAN §3.4, g4).
func TestTheGuardNeverTakesItsFolderFromTheEnvironment(t *testing.T) {
	copyDir := t.TempDir()
	t.Setenv(testSupportEnv, copyDir)
	got, err := supportFile()
	if err != nil || strings.HasPrefix(got, copyDir) {
		t.Fatalf("supportFile() = %q, %v; the variable was followed", got, err)
	}
	// A looser file there is never read: push is still refused.
	loose := filepath.Join(copyDir, fileName)
	if err := os.WriteFile(loose, []byte(`{"groups":[{"name":"push","on":false,"commands":["git push"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	Run(nil, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git push"}}`), &out, &bytes.Buffer{})
	if !strings.Contains(out.String(), "deny") {
		t.Errorf("the variable's file was read: %q", out.String())
	}
	// Named in the command line, the copy's own file is the one read: a broken one there denies every command.
	if err := os.WriteFile(loose, []byte("it isn't a guard file"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	Run([]string{"--rules", loose}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`), &out, &bytes.Buffer{})
	if !strings.Contains(out.String(), "deny") || !strings.Contains(out.String(), "broken") {
		t.Errorf("the copy's broken file wasn't read: %q", out.String())
	}
}
