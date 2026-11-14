package desktop

import (
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// realSignatureCheck puts the Mac's own check back for this test, with nothing kept from before.
func realSignatureCheck(t *testing.T) {
	t.Helper()
	savedCheck, savedVerdicts := agentSignatureCheck, signatureVerdicts
	agentSignatureCheck, signatureVerdicts = claudeSignatureRefusal, &verdictCache{verdicts: map[string]cachedVerdict{}}
	t.Cleanup(func() { agentSignatureCheck, signatureVerdicts = savedCheck, savedVerdicts })
}

// SANDBOX-PLAN §4 g7, RT126S2 D1: a claude a hire planted where Smolcorp looks (a script, or a link named claude to
// any file, as Anthropic's installer links it to one named by its version) is refused at every start of a Claude
// Code: a desk's, S2's sign-in, S1's probe, the summaries, the version check and the GM's preflight. It never runs.
// npm's Claude Code, and other agents' CLIs, can't be checked, and aren't (§5).
func TestAPlantedClaudeIsRefusedAtEachCLIStart(t *testing.T) {
	shortSupport(t)
	noLoginShell(t)
	realSignatureCheck(t)
	bin := t.TempDir()
	ran := filepath.Join(bin, "ran")
	wrapper := filepath.Join(bin, "claude")
	writeFile(t, wrapper, "#!/bin/sh\ntouch '"+ran+"'\necho 2.1.999\n")
	if err := os.Chmod(wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	refused := func(where string, err error) {
		t.Helper()
		if err == nil || err.Error() != errNotClaudeCode.Error() {
			t.Errorf("%s: %v", where, err)
		}
	}

	// A desk: through its ticket, from the pane's own process.
	if _, err := deskProgram("claude"); err == nil {
		t.Error("a desk's program wasn't checked")
	}
	pane := os.Getpid()
	standInPeer(t)
	command, err := deskStarts.issue(standInHelper, "sg2-planted", paneTmux(&pane), deskPlan{Agent: "claude"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	deskStarts.pin("sg2-planted", pane)
	desk, _ := takeDeskStart(command[2:])
	refused("a desk", errors.New(desk.Error))
	if desk.Program != "" || deskStarts.forget("sg2-planted") != errNotClaudeCode.Error() {
		t.Errorf("a desk: %+v", desk)
	}
	// S2's sign-in.
	savedFind := loginFind
	loginFind = func(string) string { return wrapper }
	t.Cleanup(func() { loginFind = savedFind })
	if _, status, why := (&loginRunner{}).start("claude", "claudeai"); status != http.StatusConflict {
		t.Errorf("S2's sign-in: %d %q", status, why)
	} else {
		refused("S2's sign-in", errors.New(why))
	}
	// S1's probe: can't tell, and nothing run.
	if row := probeAgentSignIn("claude", wrapper, nil); !row.Installed || row.State != accountUnknown {
		t.Errorf("S1's probe: %+v", row)
	}
	// The summaries: setup says why; a summary isn't made with it.
	if state := (&summaryProviders{helps: map[string]string{}}).checkProvider(providerClaude); state.kind != providerMissing {
		t.Errorf("the summaries: %+v", state)
	} else {
		refused("the summaries", errors.New(state.reason))
	}
	// The version check before a desk: refused in the same words.
	savedVersion := claudeVersion
	claudeVersion = installedClaudeVersion
	t.Cleanup(func() { claudeVersion = savedVersion })
	if refusal := claudeVersionRefusal(); refusal == nil || !strings.HasPrefix(refusal.Message, errNotClaudeCode.Error()) {
		t.Errorf("the version check: %v", refusal)
	}
	// The GM's preflight.
	if got := claudeSignIn(wrapper, ""); got != signInUnknown {
		t.Errorf("the GM's preflight: %v", got)
	}
	// A desk in a terminal window: the launcher finds the agent and checks it, and never runs it (RT126SG2 B3).
	if code := launch(&launcherRun{command: []string{"claude", "--version"}}); code != 126 {
		t.Errorf("a window's launcher: exit %d", code)
	}
	if fileExists(ran) {
		t.Fatal("the planted claude ran")
	}

	// A link named claude, to a file named as the installer names its versions: still checked, and refused.
	versions := filepath.Join(t.TempDir(), "versions")
	writeFile(t, filepath.Join(versions, "2.1.999"), "#!/bin/sh\ntouch '"+ran+"'\n")
	link := filepath.Join(t.TempDir(), "claude")
	if err := os.Symlink(filepath.Join(versions, "2.1.999"), link); err != nil {
		t.Fatal(err)
	}
	refused("a link", checkAgentProgram("claude", link))
	// npm's Claude Code and other agents' CLIs: nothing to check.
	npm := filepath.Join(t.TempDir(), "lib", "node_modules", "@anthropic-ai", "claude-code", "cli.js")
	writeFile(t, npm, "#!/usr/bin/env node\n")
	if err := checkAgentProgram("claude", npm); err != nil {
		t.Errorf("npm's Claude Code: %v", err)
	}
	if err := checkAgentProgram("codex", wrapper); err != nil {
		t.Errorf("codex: %v", err)
	}
}

// The check itself, on stand-ins: Apple's codesign must find Anthropic's team and the hardened runtime. A copy of
// an Apple program has neither; one signed here ad hoc, with the runtime and a name of the test's, passes a
// requirement naming it and still fails Anthropic's; without the runtime it fails both. A file changed after its
// verdict is checked again. (The real Claude Code's signature is checked by SG1's and RT126SG2's real sessions.)
func TestTheSignatureCheckWantsAnthropicsTeamAndTheHardenedRuntime(t *testing.T) {
	realSignatureCheck(t)
	folder := t.TempDir()
	copyOf := func(name string) string {
		t.Helper()
		from, err := os.Open("/bin/ls")
		if err != nil {
			t.Fatal(err)
		}
		defer from.Close()
		path := filepath.Join(folder, name)
		to, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o755)
		if err != nil {
			t.Fatal(err)
		}
		defer to.Close()
		if _, err := io.Copy(to, from); err != nil {
			t.Fatal(err)
		}
		return path
	}
	sign := func(path string, options ...string) {
		t.Helper()
		args := append([]string{"--force", "--sign", "-", "--identifier", "smolcorp.test.sg2"}, options...)
		if out, err := exec.Command(codesignProgram, append(args, path)...).CombinedOutput(); err != nil {
			t.Fatalf("codesign: %v %s", err, out)
		}
	}
	apple := copyOf("apple")
	if signedWith(apple, anthropicRequirement) == nil {
		t.Error("Apple's own program passed as Anthropic's")
	}
	if err := signedWith(apple, "anchor apple"); err == nil || err.Error() != "no hardened runtime" {
		t.Errorf("Apple's program without the runtime: %v", err)
	}
	hardened := copyOf("claude")
	sign(hardened, "--options", "runtime")
	if err := signedWith(hardened, `identifier "smolcorp.test.sg2"`); err != nil {
		t.Errorf("an ad hoc program with the runtime, by its own name: %v", err)
	}
	if claudeSignatureRefusal(hardened) == nil || checkAgentProgram("claude", hardened) == nil {
		t.Error("an ad hoc program passed as Anthropic's")
	}
	plain := copyOf("plain")
	sign(plain)
	if err := signedWith(plain, `identifier "smolcorp.test.sg2"`); err == nil || err.Error() != "no hardened runtime" {
		t.Errorf("an ad hoc program without the runtime: %v", err)
	}

	// The verdict is kept for the same file, and asked again once the file changed.
	asked := 0
	check := func(string) error { asked++; return nil }
	cache := &verdictCache{verdicts: map[string]cachedVerdict{}}
	for range 2 {
		if err := cache.verdict(hardened, check); err != nil || asked != 1 {
			t.Fatalf("kept: %v, asked %d", err, asked)
		}
	}
	file, _ := os.OpenFile(hardened, os.O_APPEND|os.O_WRONLY, 0)
	file.Write([]byte{0})
	file.Close()
	if _ = cache.verdict(hardened, check); asked != 2 {
		t.Errorf("a changed file wasn't checked again: asked %d", asked)
	}
	if cache.verdict(filepath.Join(folder, "gone"), check) == nil {
		t.Error("a program that isn't there passed")
	}
}

// RT126SG2 B2: whether a Claude Code may go unchecked isn't decided by its path, which a hire can choose. Once a
// signed one has passed, one under a folder named like npm's is refused, in words that say what to do, until the
// owner says yes on the setup page: a tool's key can't. The kind is kept across a start. Stand-ins only: the check
// that passes is TestMain's, and each row checks a status code or a refusal's words.
func TestAnUnsignedClaudeAfterASignedOneWaitsForTheOwnersYes(t *testing.T) {
	d := newTestDeck(t)
	signed := filepath.Join(t.TempDir(), "claude")
	writeFile(t, signed, "#!/bin/sh\n")
	npm := filepath.Join(t.TempDir(), "lib", "node_modules", "@anthropic-ai", "claude-code", "cli.js")
	writeFile(t, npm, "#!/usr/bin/env node\n")
	planted, _ := filepath.EvalSymlinks(npm)
	// npm's, seen first: it starts, as before.
	if err := checkAgentProgram("claude", npm); err != nil {
		t.Fatalf("npm's Claude Code, first: %v", err)
	}
	// A signed one passes, and is remembered.
	if err := checkAgentProgram("claude", signed); err != nil {
		t.Fatal(err)
	}
	refused := func(where string) {
		t.Helper()
		if err := checkAgentProgram("claude", npm); !errors.Is(err, errClaudeUnsigned) {
			t.Errorf("%s: %v", where, err)
		}
	}
	refused("after a signed one")
	if got := claudeInstalls.waiting(); got != planted {
		t.Errorf("the setup page is offered %q", got)
	}
	state := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	if !strings.Contains(state.Body.String(), `"claudeUnsigned":"`+planted+`"`) {
		t.Errorf("the setup page's state doesn't name it")
	}
	// Kept across a start: read again from the support folder.
	claudeInstalls.mu.Lock()
	claudeInstalls.path = ""
	claudeInstalls.mu.Unlock()
	refused("after a start")
	// A tool's key can't say yes; the page can.
	confirm := call{method: "POST", target: "/setup/hiring?claudeUnsigned=1", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}}
	asTool := confirm
	asTool.asTool = true
	if w := serve(t, d, asTool); w.Code != http.StatusForbidden {
		t.Errorf("a tool confirmed it: %d", w.Code)
	}
	refused("after a tool's try")
	if w := serve(t, d, confirm); w.Code != http.StatusOK {
		t.Fatalf("the page's yes: %d %s", w.Code, w.Body)
	}
	if err := checkAgentProgram("claude", npm); err != nil || claudeInstalls.waiting() != "" {
		t.Errorf("after the owner's yes: %v %q", err, claudeInstalls.waiting())
	}
}
