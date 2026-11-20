package desktop

import (
	"slices"
	"strings"
	"testing"
)

// FXGMHIRE126 #3a: on Windows (faked here) a hire's folder is trusted under its own spelling, C:\…, which a tester's
// .claude.json shows Claude Code keys its projects by, with the drive letter in upper case whatever case it came in,
// and, while trustBothWindowsSpellings holds, with forward slashes too (C:/…); elsewhere under its path alone, as before.
func TestOnWindowsAFolderIsTrustedUnderTheKeyClaudeCodeReads(t *testing.T) {
	isolate(t)
	const worktree = `C:\Users\ada\proj-hires\janitor-0123456789ab`
	const slashed = "C:/Users/ada/proj-hires/janitor-0123456789ab"
	want := []string{worktree}
	if trustBothWindowsSpellings {
		want = append(want, slashed)
	}
	for _, asked := range []string{worktree, `c:\Users\ada\proj-hires\janitor-0123456789ab`} {
		if got := claudeProjectKeys(asked, "windows"); !slices.Equal(got, want) {
			t.Errorf("on Windows %q is trusted as %q, want %q", asked, got, want)
		}
	}
	file := newClaudeJSON(t, claudeJSONBefore)
	if err := preTrustOn("windows", worktree, file, nil); err != nil {
		t.Fatal(err)
	}
	data := mustRead(t, file)
	for _, key := range want {
		if !isTrusted(key, data) {
			t.Errorf("%q isn't trusted:\n%s", key, data)
		}
	}
	if !isTrusted(worktree, data) || strings.Contains(string(data), `"c:`) {
		t.Errorf("Claude Code's spelling %q isn't trusted, or a drive is in lower case:\n%s", worktree, data)
	}
	// Once more is no change: each key is there already, and nothing else in the file moved.
	if err := preTrustOn("windows", worktree, file, nil); err != nil || string(mustRead(t, file)) != string(data) {
		t.Errorf("trusting again changed the file (%v):\n%s", err, mustRead(t, file))
	}
	// With an API key, the key's tail too, in the same write.
	tail := keyTail(placeTestKey)
	if err := preTrustOn("windows", worktree+"-2", file, &tail); err != nil || !isApproved(tail, mustRead(t, file)) || !isTrusted(worktree+"-2", mustRead(t, file)) {
		t.Errorf("an API-key hire on Windows: %v\n%s", err, mustRead(t, file))
	}
	// Linux and the Mac: the path as it is, whatever it holds.
	for _, goos := range []string{"linux", "darwin"} {
		if got := claudeProjectKeys(`/home/ada/odd\name`, goos); !slices.Equal(got, []string{`/home/ada/odd\name`}) {
			t.Errorf("on %s the keys are %q, want the path alone", goos, got)
		}
	}
	linux := newClaudeJSON(t, `{"theme":"dark"}`)
	if err := preTrustOn("linux", "/home/ada/proj-hires/janitor", linux, nil); err != nil || !isTrusted("/home/ada/proj-hires/janitor", mustRead(t, linux)) {
		t.Errorf("on Linux: %v\n%s", err, mustRead(t, linux))
	}
	if strings.Contains(string(mustRead(t, linux)), "C:") {
		t.Errorf("a Windows key on Linux:\n%s", mustRead(t, linux))
	}
}

// FXGMHIRE126 #4: a launcher hire starts on the model it was hired on (its start environment's), in place of the role's
// as the config has it; none keeps the role's. hireModelAllowed is the check both the core and the launcher make.
func TestALaunchersRoleStartsOnTheModelItWasHiredOn(t *testing.T) {
	isolate(t)
	writeFile(t, configFile, `{"roles": [{"id": "tester", "name": "Tester", "agent": "claude", "model": "sonnet", "instructions": "Run the tests."}]}`)
	args, err := roleArgumentsOn("tester", "/work", hiredIntoGeneralFolder, "go", "opus")
	if err != nil || !slices.Contains(args, "--model=opus") || slices.Contains(args, "--model=sonnet") || args[len(args)-1] != "go" {
		t.Errorf("hired on opus, the tester starts as %q, %v", args, err)
	}
	if args, err := roleArgumentsOn("tester", "/work", hiredIntoGeneralFolder, "go", ""); err != nil || !slices.Contains(args, "--model=sonnet") {
		t.Errorf("hired on its own model, the tester starts as %q, %v", args, err)
	}
	for model, allowed := range map[string]bool{"": true, "opus": true, "claude-sonnet-5-5": true, "claude-opus-5-5[1m]": true, "us.anthropic.x:0": true,
		"op us": false, "opus;x": false, "--settings=x": false, "a/b": false, "é": false, strings.Repeat("o", setupHireModelLongest+1): false} {
		if hireModelAllowed(model) != allowed {
			t.Errorf("hireModelAllowed(%q) = %v, want %v", model, !allowed, allowed)
		}
	}
}

// FXGMHIRE126 #3a: a hire with no worktree starts, on Windows (faked here), in its folder as physical resolves it,
// the spelling preTrust trusted, never the spelling it was asked in: a working folder keeps that one there, and
// Claude Code looks its trust up by it. Elsewhere it starts in the folder asked for, as before.
func TestOnWindowsAHireWithNoWorktreeStartsInTheFolderItIsTrustedUnder(t *testing.T) {
	const asked, resolved = `c:\users\RUNNER~1\proj`, `C:\Users\runneradmin\proj`
	if got := startFolder(asked, resolved, "windows"); got != resolved {
		t.Errorf("on Windows it starts in %q, want %q", got, resolved)
	}
	if got := claudeProjectKeys(startFolder(asked, resolved, "windows"), "windows"); got[0] != resolved {
		t.Errorf("its first key is %q, want the folder it starts in, %q", got[0], resolved)
	}
	for _, goos := range []string{"linux", "darwin"} {
		if got := startFolder("/home/ada/proj", "/data/ada/proj", goos); got != "/home/ada/proj" {
			t.Errorf("on %s it starts in %q, want the folder asked for", goos, got)
		}
	}
	if got := startFolder(asked, "", "windows"); got != asked {
		t.Errorf("with nothing resolved it starts in %q, want %q", got, asked)
	}
}
