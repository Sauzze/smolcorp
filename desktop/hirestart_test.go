package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	testAPIKey  = "fake-key-0123456789ABCDEFGHIJ"
	testGMToken = "fake-gm-token-1"
)

func testHireRole() Role {
	return Role{ID: "janitor", Name: "Janitor", Agent: "claude", Model: "sonnet", Effort: "high"}
}

func TestTheStartScriptIsTheMacsForAHire(t *testing.T) {
	isolate(t)
	script, err := hireStart{Role: testHireRole(), Message: "go", Ports: "4100-4103", SendKey: "k-1_a", DefaultAccount: true}.startText()
	if err != nil {
		t.Fatal(err)
	}
	want := "unset SMOLCORP_GM_TOKEN\nunset CLAUDE_CONFIG_DIR\n" +
		"export SMOLCORP_HIRE_PORTS='4100-4103'\nexport SMOLCORP_HIRE_KEY='k-1_a'\n" +
		"exec 'claude' '--model=sonnet' '--effort=high' '--' 'go'\n"
	if script != want {
		t.Fatalf("script:\n%s\nwant:\n%s", script, want)
	}
	// The account line, settings, and PATH come as the Mac puts them, before the exports.
	own, _ := hireStart{Role: testHireRole(), Message: "it's", ConfigDir: "/a/b's", Settings: "/s.json", CLIDir: "/app/bin"}.startText()
	for _, line := range []string{"export CLAUDE_CONFIG_DIR='/a/b'\\''s'\nexport PATH='/app/bin':\"$PATH\"\n", "exec 'claude' '--settings=/s.json' ", "'it'\\''s'\n"} {
		if !strings.Contains(own, line) {
			t.Errorf("missing %q in\n%s", line, own)
		}
	}
	if def, _ := (hireStart{Role: testHireRole(), Message: "x", DefaultAccount: true}).startText(); !strings.Contains(def, "unset CLAUDE_CONFIG_DIR\n") {
		t.Error("~/.claude's account should clear CLAUDE_CONFIG_DIR")
	}
	// No account named is not the default account: the script says nothing about it.
	if none, _ := (hireStart{Role: testHireRole(), Message: "x"}).startText(); strings.Contains(none, "CLAUDE_CONFIG_DIR") {
		t.Errorf("no account named, but the script has an account line:\n%s", none)
	}
	// An agent that isn't known is Claude, as on the Mac: it gets Claude's settings and account line.
	odd, _ := hireStart{Role: Role{Name: "Odd", Agent: "bogus"}, Message: "x", Settings: "/s", DefaultAccount: true}.startText()
	if !strings.Contains(odd, "exec 'claude' '--settings=/s'") || !strings.Contains(odd, "unset CLAUDE_CONFIG_DIR") {
		t.Errorf("an unknown agent should start as Claude:\n%s", odd)
	}
	// Other agents get no account line and no settings, and an unmanaged hire no exports.
	codex, _ := hireStart{Role: Role{Name: "Co", Agent: "codex"}, Message: "x", ConfigDir: "/a", Settings: "/s"}.startText()
	if strings.Contains(codex, "CLAUDE_CONFIG_DIR") || strings.Contains(codex, "--settings") || strings.Contains(codex, "export") {
		t.Errorf("codex script: %s", codex)
	}
}

func TestAHireNeverGetsTheGMToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the start script is the tmux engine's (hireengine.go), Linux's: it runs with /bin/sh")
	}
	isolate(t)
	t.Setenv("SMOLCORP_GM_TOKEN", testGMToken)
	for _, apiKey := range []bool{false, true} {
		script, err := hireStart{Role: testHireRole(), Message: "go", Ports: "4100-4103", SendKey: "k-1_a", APIKey: apiKey,
			TestPort: 7745, TestSupportDir: "/t"}.startText()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(script, testGMToken) {
			t.Fatalf("the GM token's value is in a hire's script:\n%s", script)
		}
		// Run the script itself (never the real claude: a fake prints its environment) in an
		// environment that holds the GM token, as the app's does.
		bin := t.TempDir()
		if apiKey {
			fake := "#!/bin/sh\nprintf %s " + testAPIKey + "\n"
			if err := os.WriteFile(filepath.Join(bin, "secret-tool"), []byte(fake), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nenv\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		file, err := writeHireStartScript(t.TempDir(), script)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("/bin/sh", file)
		cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "SMOLCORP_GM_TOKEN=" + testGMToken}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("apiKey=%v: %v\n%s", apiKey, err, out)
		}
		if strings.Contains(string(out), "SMOLCORP_GM_TOKEN") || strings.Contains(string(out), testGMToken) {
			t.Errorf("apiKey=%v: the hire's environment holds the GM token:\n%s", apiKey, out)
		}
		if !strings.Contains(string(out), "\n"+hireKeyEnvironment+"=k-1_a\n") && !strings.HasPrefix(string(out), hireKeyEnvironment+"=k-1_a\n") {
			t.Errorf("apiKey=%v: the hire's own send key is missing:\n%s", apiKey, out)
		}
	}
}

func TestATestCopyHireExportsItsPortAndFolder(t *testing.T) {
	isolate(t)
	script, err := hireStart{Role: testHireRole(), Message: "go", TestPort: 7745, TestSupportDir: "/tmp/it's"}.startText()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "export SMOLCORP_PORT='7745'\nexport SMOLCORP_TEST_SUPPORT_DIR='/tmp/it'\\''s'\nexec ") {
		t.Errorf("script:\n%s", script)
	}
	if !strings.Contains(script, "'--setting-sources=project,local' ") {
		t.Errorf("a test copy's Claude hire doesn't load the account's own settings:\n%s", script)
	}
	plain, _ := hireStart{Role: testHireRole(), Message: "go"}.startText()
	if strings.Contains(plain, "--setting-sources") {
		t.Errorf("a real copy's hire loads its settings as it does:\n%s", plain)
	}
	if strings.Contains(plain, "SMOLCORP_PORT") || strings.Contains(plain, "SUPPORT_DIR") {
		t.Errorf("a real copy's hire has no test variables:\n%s", plain)
	}
	// Half a test copy, or a real Smolcorp's port, is refused.
	for _, h := range []hireStart{{TestPort: 7745}, {TestSupportDir: "/t"}, {TestPort: 7710, TestSupportDir: "/t"},
		{TestPort: 7720, TestSupportDir: "/t"}, {TestPort: 7730, TestSupportDir: "/t"}, {TestPort: 70000, TestSupportDir: "/t"}} {
		h.Role, h.Message = testHireRole(), "go"
		if got, err := h.startText(); err == nil {
			t.Errorf("%+v should be refused, got:\n%s", h, got)
		}
	}
}

// The team grid's script (gmtile_unix.go) is the one start script left: a file only its owner can read or run, in
// a folder only its owner can open, that removes itself as it starts. No desk's start is a file any more.
func TestTheStartScriptFileIsPrivateAndRemovesItself(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the team grid is tmux's: it runs with /bin/sh")
	}
	isolate(t)
	folder := filepath.Join(t.TempDir(), "hires")
	file, err := writeHireStartScript(folder, "#!/bin/sh\nrm -f \"$0\"\nexec true\n")
	if err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0o700 {
		t.Errorf("script mode %v", info.Mode().Perm())
	}
	if info, _ := os.Stat(folder); info.Mode().Perm() != 0o700 {
		t.Errorf("folder mode %v", info.Mode().Perm())
	}
	execShell(t, t.TempDir(), file)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Error("the script should remove itself as it starts")
	}
}

// execShell runs a start script with a fake bin folder first on a bare PATH
// and a temporary HOME, returning what it printed.
func execShell(t *testing.T, bin, file string) (string, error) {
	t.Helper()
	cmd := exec.Command("/bin/sh", file)
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestATestCopyNeverHiresClaudeOnTheDefaultAccount(t *testing.T) {
	for _, program := range []string{"claude", "", "bogus"} {
		err := testCopyRefusal(7799, program, "")
		if r, ok := err.(*hireRefusal); !ok || r.Status != 409 || !strings.Contains(r.Message, "never hires Claude on the default account") {
			t.Errorf("a test copy hiring %q: %v, want the T721 refusal", program, err)
		}
	}
	// On a Mac a test copy hires on a named account, as the Mac's Swift engine does (setuphire_test.go).
	if runtime.GOOS != "darwin" && testCopyRefusal(7799, "claude", "/acct") == nil {
		t.Error("a test copy's hire on a named account must wait for the reporting hooks")
	}
	if testCopyRefusal(0, "claude", "") != nil || testCopyRefusal(0, "codex", "") != nil {
		t.Error("the office's own hires are not refused")
	}
	// Another agent would report to the real office through its own hooks: refused, as the Mac's test copy does.
	for _, program := range []string{"codex", "gemini", "opencode"} {
		for _, configDir := range []string{"", "/acct"} {
			err := testCopyRefusal(7799, program, configDir)
			if r, ok := err.(*hireRefusal); !ok || r.Status != 409 || r.Message != testCopyOnlyClaude {
				t.Errorf("a test copy hiring %s (%q): %v, want the Mac's refusal", program, configDir, err)
			}
		}
	}
}

func TestAHireGetsItsOwnSendKeyAndTheGMNeverDoes(t *testing.T) {
	isolate(t)
	file := readRegistry(registryPath(), true)
	file.hires = append(file.hires, Hire{ID: "gm1", Name: "GM", Role: "gm", Folder: homeDir, State: "starting"}, Hire{ID: "jan1", Name: "Janitor", Role: "janitor", Folder: homeDir, State: "starting"})
	if err := file.save(registryPath()); err != nil {
		t.Fatal(err)
	}
	if key, err := sendKeyFor(Role{ID: "gm"}, "gm1"); key != "" || err != nil {
		t.Errorf("the GM was given the send key %q (%v)", key, err)
	}
	key, err := sendKeyFor(Role{ID: "janitor"}, "jan1")
	if key == "" || err != nil {
		t.Fatalf("a hire got no send key: %v", err)
	}
	if h := readRegistry(registryPath(), false).hireForSendKey(key); h == nil || h.ID != "jan1" {
		t.Errorf("the key isn't the hire's in the registry: %+v", h)
	}
	if strings.Contains(string(mustRead(t, registryPath())), key) {
		t.Error("the registry holds the key itself, not its hash")
	}
	if key, err := sendKeyFor(Role{ID: "janitor"}, "not-written-down"); key != "" || err == nil {
		t.Errorf("a hire not in the registry got the key %q", key)
	}
	// The key is exported for a hire and never for the GM's own script.
	gmKey, _ := sendKeyFor(Role{ID: "gm"}, "gm1")
	gm, _ := hireStart{Role: Role{ID: "gm", Name: "GM", Agent: "claude"}, Message: "x", SendKey: gmKey}.startText()
	other, _ := hireStart{Role: Role{ID: "janitor", Name: "Janitor", Agent: "claude"}, Message: "x", SendKey: key}.startText()
	if strings.Contains(gm, hireKeyEnvironment) || !strings.Contains(other, hireKeyEnvironment+"='"+key+"'") {
		t.Errorf("the GM's script has a key, or another's has none:\n%s\n%s", gm, other)
	}
	if strings.Contains(gm, "SMOLCORP_GM_TOKEN=") || strings.Contains(other, "SMOLCORP_GM_TOKEN=") {
		t.Error("a start script sets the GM token")
	}
}

func TestHireEnvironmentNeverCarriesTheGMToken(t *testing.T) {
	t.Setenv("SMOLCORP_GM_TOKEN", "fake-gm-token-1")
	t.Setenv("TMUX", "/tmp/x,1,0")
	for _, variable := range hireEnvironment() {
		if strings.HasPrefix(variable, "SMOLCORP_GM_TOKEN=") || strings.HasPrefix(variable, "TMUX=") {
			t.Errorf("a hire's environment has %q", variable)
		}
	}
}

// The office's own hires get what the Mac's engine passes on (Engine.swift inherited: the ssh agent, LOGNAME,
// LC_*), never the port and folder the Mac app names its engine by, nor a hire's send key. A test copy's hires
// keep its port and folder, so they never reach the real office (FXC3c).
func TestTheOfficesHireIsNotGivenTheEnginesPortFolderOrAHireKey(t *testing.T) {
	onPort(t, portFromEnvironment(""))
	t.Setenv("SMOLCORP_PORT", "7710")
	t.Setenv("SMOLCORP_TEST_SUPPORT_DIR", "/fake/support")
	t.Setenv(hireKeyEnvironment, "fake-send-key")
	t.Setenv("SSH_AUTH_SOCK", "/fake/agent.sock")
	t.Setenv("LOGNAME", "me")
	t.Setenv("LC_CTYPE", "UTF-8")
	env := hireEnvironment()
	for _, name := range []string{"SMOLCORP_PORT", "SMOLCORP_TEST_SUPPORT_DIR", hireKeyEnvironment} {
		if slices.ContainsFunc(env, func(v string) bool { return strings.HasPrefix(v, name+"=") }) {
			t.Errorf("the office's hire gets %s", name)
		}
	}
	for _, kept := range []string{"SSH_AUTH_SOCK=/fake/agent.sock", "LOGNAME=me", "LC_CTYPE=UTF-8"} {
		if !slices.Contains(env, kept) {
			t.Errorf("the office's hire lost %s", kept)
		}
	}
	onPort(t, portFromEnvironment("")+1)
	t.Setenv("SMOLCORP_PORT", strconv.Itoa(Port))
	env = hireEnvironment()
	for _, kept := range []string{"SMOLCORP_PORT=" + strconv.Itoa(Port), "SMOLCORP_TEST_SUPPORT_DIR=/fake/support"} {
		if !slices.Contains(env, kept) {
			t.Errorf("a test copy's hire lost %s", kept)
		}
	}
	if slices.ContainsFunc(env, func(v string) bool { return strings.HasPrefix(v, hireKeyEnvironment+"=") }) {
		t.Error("a test copy's hire gets another's send key")
	}
}

// The test-copy port comes from the default the app itself uses, and a refusal
// that isn't a hireRefusal still gives a status.
func TestTestCopyPortFollowsTheDefaultAndRefusalStatusHasAFallback(t *testing.T) {
	onPort(t, portFromEnvironment(""))
	if got := testCopyPort(); got != 0 {
		t.Errorf("the default port is a test copy: %d", got)
	}
	onPort(t, portFromEnvironment("")+1)
	if got := testCopyPort(); got != Port {
		t.Errorf("testCopyPort = %d, want %d", got, Port)
	}
	if got := refusalStatus(&hireRefusal{Status: 418, Message: "x"}); got != 418 {
		t.Errorf("status %d", got)
	}
	if got := refusalStatus(errors.New("plain")); got != 409 {
		t.Errorf("a plain error gave %d, want 409", got)
	}
}

// The General Manager starts with its plugin, where the app bundles it (Role.startScript, GMPlugin.Launch): its
// folder and its agent as the session's, the program its tools run, or none even from the terminal's, and it is
// told who it is. No other role ever gets the plugin.
func TestTheGMStartsWithItsPluginAndAgent(t *testing.T) {
	gm := Role{ID: gmRole, Name: "General Manager", Agent: "claude", Model: "opus"}
	start := hireStart{Role: gm, Message: "Read your brief.", GMToken: "t0ken", GMPlugin: "/Applications/Smolcorp.app/Contents/Resources/gm/plugin",
		GMCLI: "/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp"}
	script, err := start.startText()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`'--plugin-dir=/Applications/Smolcorp.app/Contents/Resources/gm/plugin' '--agent=smolcorp-gm:general-manager' '--model=opus'`,
		"export SMOLCORP_CLI='/Applications/Smolcorp.app/Contents/Resources/bin/smolcorp'\n"} {
		if !strings.Contains(script, want) {
			t.Errorf("the GM's script has no %q:\n%s", want, script)
		}
	}
	start.GMCLI = ""
	if script, _ := start.startText(); !strings.Contains(script, "unset SMOLCORP_CLI\n") || strings.Contains(script, "export SMOLCORP_CLI") {
		t.Errorf("no program: the variable is unset:\n%s", script)
	}
	start.Role.ID, start.GMToken = "reviewer", ""
	if _, err := start.startText(); err == nil {
		t.Error("a reviewer started with the GM's plugin")
	}
	if script, _ := (hireStart{Role: gm, Message: "m", GMToken: "t0ken"}).startText(); strings.Contains(script, "plugin") || strings.Contains(script, "SMOLCORP_CLI") {
		t.Errorf("no plugin bundled, none loaded:\n%s", script)
	}

	intro := gmIntroduction("/x/hires", false)
	if !strings.HasPrefix(intro, "You are the General Manager of this team. Your folder is /x/hires.") || strings.Contains(intro, "not available") {
		t.Errorf("the introduction: %q", intro)
	}
	if !strings.Contains(gmIntroduction("/x/hires", true), "The `smolcorp` tools are not available in this copy of Smolcorp") {
		t.Error("the GM isn't told its tools are missing")
	}
}

// The plugin is found beside the program as Smolcorp.app bundles it (bin/smolcorp and gm/plugin in Resources),
// and the repo's own plugin is the one the start line names: its manifest's name, its agent, and its tools'
// server run through SMOLCORP_CLI.
func TestTheGMPluginIsFoundBesideTheProgramAsTheAppBundlesIt(t *testing.T) {
	resources := t.TempDir()
	cli := filepath.Join(resources, "bin", "smolcorp")
	if gmPluginFolder(cli) != "" || gmPluginFolder("") != "" {
		t.Error("a plugin found where there's none")
	}
	writeFile(t, filepath.Join(resources, "gm", "plugin", ".claude-plugin", "plugin.json"), "{}")
	if got := gmPluginFolder(cli); got != filepath.Join(resources, "gm", "plugin") {
		t.Errorf("found %q", got)
	}
	plugin := filepath.Join("..", "Resources", "gm", "plugin")
	var manifest struct{ Name string }
	data, err := os.ReadFile(filepath.Join(plugin, ".claude-plugin", "plugin.json"))
	if err != nil || json.Unmarshal(data, &manifest) != nil || manifest.Name != gmPluginName {
		t.Errorf("the plugin's manifest: %v %q", err, manifest.Name)
	}
	if _, err := os.Stat(filepath.Join(plugin, "agents", gmPluginAgent+".md")); err != nil {
		t.Errorf("the plugin's agent: %v", err)
	}
	if tools, _ := os.ReadFile(filepath.Join(plugin, ".mcp.json")); !strings.Contains(string(tools), "${"+gmCLIEnvironment+"}") {
		t.Errorf("the tools' server doesn't run through $%s: %s", gmCLIEnvironment, tools)
	}
}
