//go:build unix

package desktop

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shippedReportScript puts Resources/report.sh, exactly as it is, where the app ships it
// (<app>/Contents/Resources/report.sh, beside bin/smolcorp), and gives its path and the app's command's. Nothing is
// beside it but the command: no hook-header, no port filled in.
func shippedReportScript(t *testing.T) (script, program string) {
	t.Helper()
	for _, tool := range []string{"/bin/bash", "/usr/bin/curl", "/usr/bin/perl"} {
		if _, err := os.Stat(tool); err != nil {
			t.Skip("no " + tool)
		}
	}
	shipped, err := os.ReadFile(filepath.Join("..", "Resources", reportScriptName))
	if err != nil {
		t.Fatal(err)
	}
	// Its folder with links resolved, as the core names its own program (commandLineProgram).
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resources := filepath.Join(base, "Smolcorp.app", "Contents", "Resources")
	script, program = filepath.Join(resources, reportScriptName), filepath.Join(resources, "bin", "smolcorp")
	writeReleaseFile(t, script, string(shipped), 0o755)
	writeReleaseFile(t, program, "#!/bin/sh\n", 0o755)
	return script, program
}

// reportRun is one run of a report.sh: what it was given, and what reached the deck.
type reportRun struct {
	key    string // the hook key the request carried, "" when nothing came
	answer int    // the deck's answer, 0 when nothing came
}

// runReportScript runs script with args and env (and nothing else of this computer's environment) on event, and
// gives what reached d, through a listener that answers as the core: the key sent and the deck's answer (204 for
// every request when d is nil). Every run names its port, with --port or SMOLCORP_PORT, and never the app's: $PORT
// in args and env is the listener's.
func runReportScript(t *testing.T, d *Deck, script string, args []string, env map[string]string, event string) reportRun {
	t.Helper()
	runs := make(chan reportRun, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host, r.RemoteAddr = "localhost:7710", thisComputer
		recorded := httptest.NewRecorder()
		if d != nil {
			(&router{deck: d}).ServeHTTP(recorded, r)
		} else {
			recorded.WriteHeader(http.StatusNoContent)
		}
		runs <- reportRun{key: r.Header.Get(hookKeyHeader), answer: recorded.Code}
		w.WriteHeader(recorded.Code)
	}))
	defer server.Close()
	port := fmt.Sprint(mustPort(t, server.URL))
	run := exec.Command(script, append(append([]string{}, args...), "claude")...)
	run.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + homeDir}
	for name, value := range env {
		run.Env = append(run.Env, name+"="+strings.ReplaceAll(value, "$PORT", port))
	}
	for i, arg := range run.Args {
		run.Args[i] = strings.ReplaceAll(arg, "$PORT", port)
	}
	run.Stdin = strings.NewReader(event)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v %s", run.Args, err, out)
	}
	select {
	case got := <-runs:
		return got
	default:
		return reportRun{}
	}
}

// The report.sh the app ships runs from the app as it is (SA-X11). Its tab is a pane's when a tty is above this test
// (the variables name a desk's socket) and headless:<session id> when none is; each row below has the same answer
// either way. It sends a desk's own key in a desk, and then never reads hook-header; elsewhere the hook-header of
// the folder it is told, a test copy's, or the app's.
func TestTheAppsReportShSendsItsDesksKeyAndNoOthers(t *testing.T) {
	script, _ := shippedReportScript(t)
	before, _ := os.ReadFile(script)
	d, keys, sockets := keyedOffice(t)
	ann, owner := keys["aaa111"], d.access.HookKey
	annWork := filepath.Join(filepath.Dir(sockets["aaa111"]), "wt-aaa111")
	if got := hookAs(t, d, keys["bbb222"], tmuxTabOn(sockets["bbb222"]), "SessionStart", "s-bea", annWork); got != http.StatusNoContent {
		t.Fatalf("Bea's session: %d", got)
	}
	// The app's support folder as report.sh finds it from the app (the home's Library), with the owner's key.
	appSupport := filepath.Join(homeDir, "Library", "Application Support", "AgentDeck")
	writeReleaseFile(t, filepath.Join(appSupport, "hook-header"), hookKeyHeader+": "+owner+"\n", 0o600)
	inDesk := func(key, socket string) map[string]string {
		env := map[string]string{"SMOLCORP_HEADLESS": "1", "TMUX": socket + ",123,0", "TMUX_PANE": "%1", "CLAUDE_PROJECT_DIR": annWork}
		if key != "" {
			env[hookKeyEnvironment] = key
		}
		return env
	}
	port := []string{"--port", "$PORT"}

	// In Ann's desk: her own key, never the owner's, which hook-header holds.
	got := runReportScript(t, d, script, port, inDesk(ann, sockets["aaa111"]), hookEvent("SessionStart", "s-ann", annWork))
	if got.key != ann || got.answer != http.StatusNoContent {
		t.Errorf("in Ann's desk: sent %q, answered %d", got.key, got.answer)
	}
	// Naming Bea's session from there: refused.
	got = runReportScript(t, d, script, port, inDesk(ann, sockets["aaa111"]), hookEvent("UserPromptSubmit", "s-bea", annWork))
	if got.key != ann || got.answer != http.StatusForbidden {
		t.Errorf("Ann naming Bea's session: sent %q, answered %d", got.key, got.answer)
	}
	if s := d.sessions.Get("s-bea"); s == nil || s.IsBusy() {
		t.Errorf("Bea's session after Ann's report.sh: %+v", s)
	}
	// With hook-header gone, Ann's desk reports all the same: it never needed the file (a desk under the sandbox
	// can't read it).
	if err := os.Rename(filepath.Join(appSupport, "hook-header"), filepath.Join(appSupport, "hook-header.away")); err != nil {
		t.Fatal(err)
	}
	if got := runReportScript(t, d, script, port, inDesk(ann, sockets["aaa111"]), hookEvent("Stop", "s-ann", annWork)); got.key != ann || got.answer != http.StatusNoContent {
		t.Errorf("Ann with no hook-header: sent %q, answered %d", got.key, got.answer)
	}
	if err := os.Rename(filepath.Join(appSupport, "hook-header.away"), filepath.Join(appSupport, "hook-header")); err != nil {
		t.Fatal(err)
	}
	// A key that isn't a key's shape, as one with a line break and a header after it, sends nothing: not the owner's.
	smuggled := inDesk(ann+"\r\nX-AgentDeck-Tab: "+tmuxTabOn(sockets["bbb222"]), sockets["aaa111"])
	if got := runReportScript(t, d, script, port, smuggled, hookEvent("SessionStart", "s-smuggled", annWork)); got.key != "" || d.sessions.Get("s-smuggled") != nil {
		t.Errorf("a key with a header in it: sent %q, answered %d", got.key, got.answer)
	}

	// No key of its own: the owner's sessions' key, from the app's support folder. It reaches no desk's session.
	got = runReportScript(t, d, script, port, inDesk("", sockets["ccc333"]), hookEvent("SessionStart", "s-owner", annWork))
	if got.key != owner || got.answer != http.StatusNoContent {
		t.Errorf("the owner's session: sent %q, answered %d", got.key, got.answer)
	}
	if got := runReportScript(t, d, script, port, inDesk("", sockets["ccc333"]), hookEvent("UserPromptSubmit", "s-bea", annWork)); got.answer != http.StatusForbidden {
		t.Errorf("the owner's key naming Bea's session: %d", got.answer)
	}
	// The folder --support names; a test copy's (whose key the real app refuses); a relative folder sends nothing.
	named := t.TempDir()
	writeReleaseFile(t, filepath.Join(named, "hook-header"), hookKeyHeader+": named-folder-key\n", 0o600)
	if got := runReportScript(t, d, script, []string{"--port", "$PORT", "--support", named}, inDesk("", sockets["ccc333"]), hookEvent("SessionStart", "s-named", annWork)); got.key != "named-folder-key" {
		t.Errorf("--support: sent %q", got.key)
	}
	env := inDesk("", sockets["ccc333"])
	env[testSupportVariable] = named
	if got := runReportScript(t, d, script, port, env, hookEvent("SessionStart", "s-copy", annWork)); got.key != "named-folder-key" {
		t.Errorf("a test copy's folder: sent %q", got.key)
	}
	if got := runReportScript(t, d, script, []string{"--port", "$PORT", "--support", "relative"}, inDesk("", sockets["ccc333"]), hookEvent("SessionStart", "s-rel", annWork)); got.key != "" {
		t.Errorf("a relative --support: sent %q", got.key)
	}
	// The port: SMOLCORP_PORT with no --port; a --port that isn't one sends nothing (never on to another port).
	env = inDesk("", sockets["ccc333"])
	env["SMOLCORP_PORT"] = "$PORT"
	if got := runReportScript(t, d, script, nil, env, hookEvent("SessionStart", "s-env-port", annWork)); got.key != owner {
		t.Errorf("SMOLCORP_PORT: sent %q", got.key)
	}
	if got := runReportScript(t, d, script, []string{"--port", "80"}, env, hookEvent("SessionStart", "s-bad-port", annWork)); got.key != "" {
		t.Errorf("--port 80: sent %q", got.key)
	}

	// Run from the app it changes nothing there, and writes nothing beside itself.
	if after, _ := os.ReadFile(script); !bytes.Equal(after, before) {
		t.Error("report.sh changed as it ran")
	}
	if entries, _ := os.ReadDir(filepath.Dir(script)); len(entries) != 2 {
		t.Errorf("beside report.sh: %v", entries)
	}
}

// An earlier version's copy of report.sh in the support folder, which an account connected then may still run,
// becomes a pointer (a shebang, two comment lines and an exec) that runs the app's own, with this core's port and folder (SA-X11). With no copy, none is made;
// a link there is replaced, not followed; a test copy leaves it alone.
func TestAnEarlierCopyOfReportShRunsTheApps(t *testing.T) {
	script, program := shippedReportScript(t)
	macSupportFolder(t)
	a := NewAccess()
	saved := executable
	t.Cleanup(func() { executable = saved })
	executable = func() (string, error) { return program, nil }
	copied := filepath.Join(supportDir, reportScriptName)
	if testCopyPort() != 0 {
		t.Fatalf("the test runs as a test copy (port %d)", Port)
	}

	// No copy: nothing made.
	pointReportScriptAtTheApp()
	if _, err := os.Lstat(copied); err == nil {
		t.Fatal("a copy made where there was none")
	}
	// A copy an earlier version left, with its port filled in: it becomes the pointer, the owner's alone. A temp file
	// a start that stopped halfway left is removed; a file of another name is not.
	writeReleaseFile(t, copied, "#!/bin/bash\n# report.sh as 1.21 left it\ncurl http://127.0.0.1:7710/hook\n", 0o755)
	leftBehind, notOurs := filepath.Join(supportDir, ".report.sh-1234567"), filepath.Join(supportDir, ".report.sh-mine")
	writeReleaseFile(t, leftBehind, "half a pointer", 0o600)
	writeReleaseFile(t, notOurs, "kept", 0o600)
	pointReportScriptAtTheApp()
	if _, err := os.Lstat(leftBehind); err == nil {
		t.Error("a start's leftover temp file is still there")
	}
	if text, _ := os.ReadFile(notOurs); string(text) != "kept" {
		t.Errorf("a file of another name was touched: %q", text)
	}
	os.Remove(notOurs)
	want := reportScriptPointer(script, Port, supportDir)
	if text, _ := os.ReadFile(copied); string(text) != want {
		t.Errorf("the copy became:\n%s\nwant:\n%s", text, want)
	}
	if info, err := os.Lstat(copied); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the pointer's mode: %v %v", info.Mode(), err)
	}
	if !strings.Contains(want, shellQuoted(script)) || !strings.Contains(want, "--support "+shellQuoted(supportDir)) {
		t.Errorf("the pointer doesn't name the app's script and the folder: %s", want)
	}
	// The app moved: the next start points at it where it is now.
	moved, movedProgram := shippedReportScript(t)
	executable = func() (string, error) { return movedProgram, nil }
	pointReportScriptAtTheApp()
	if text, _ := os.ReadFile(copied); string(text) != reportScriptPointer(moved, Port, supportDir) {
		t.Errorf("after the app moved:\n%s", text)
	}

	// The pointer, run (made for a stand-in core's port, which each run fills in, never the app's): it sends what the
	// app's script sends, the owner's key from this folder's hook-header, or a desk's own.
	stand := filepath.Join(t.TempDir(), reportScriptName)
	writeReleaseFile(t, stand, strings.Replace(reportScriptPointer(moved, 1111, supportDir), "--port 1111", `--port "$STAND_IN_PORT"`, 1), 0o700)
	for _, row := range []struct{ name, key, want string }{
		{"the owner's session", "", a.HookKey},
		{"a desk", "a-desks-own-key", "a-desks-own-key"},
	} {
		env := map[string]string{"STAND_IN_PORT": "$PORT", "SMOLCORP_HEADLESS": "1"}
		if row.key != "" {
			env[hookKeyEnvironment] = row.key
		}
		if got := runReportScript(t, nil, stand, nil, env, hookEvent("SessionStart", "s-pointer", "/proj")); got.key != row.want {
			t.Errorf("%s through the pointer: sent %q, want %q", row.name, got.key, row.want)
		}
	}

	// A link put where the copy is: replaced by the pointer, and what it led to is untouched.
	target := filepath.Join(t.TempDir(), "elsewhere.sh")
	writeReleaseFile(t, target, "#!/bin/sh\n", 0o755)
	os.Remove(copied)
	if err := os.Symlink(target, copied); err != nil {
		t.Fatal(err)
	}
	pointReportScriptAtTheApp()
	if info, err := os.Lstat(copied); err != nil || !info.Mode().IsRegular() {
		t.Errorf("the link is still there: %v %v", info.Mode(), err)
	}
	if text, _ := os.ReadFile(target); string(text) != "#!/bin/sh\n" {
		t.Errorf("the link was written through: %q", text)
	}
	// With no report.sh in the app (a core built by hand), the copy is left as it is.
	os.Remove(moved)
	writeReleaseFile(t, copied, "left\n", 0o755)
	pointReportScriptAtTheApp()
	if text, _ := os.ReadFile(copied); string(text) != "left\n" {
		t.Errorf("with no report.sh in the app: %q", text)
	}
	// A test copy leaves it alone.
	executable = func() (string, error) { return program, nil }
	onPort(t, 7751)
	pointReportScriptAtTheApp()
	if text, _ := os.ReadFile(copied); string(text) != "left\n" {
		t.Errorf("a test copy changed it: %q", text)
	}
	// No temp file is left behind in the support folder.
	if matches, _ := filepath.Glob(filepath.Join(supportDir, ".report.sh-*")); len(matches) != 0 {
		t.Errorf("left behind: %v", matches)
	}
}
