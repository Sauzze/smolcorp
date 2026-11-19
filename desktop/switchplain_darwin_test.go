package desktop

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// FXSWITCHPLAIN on a Mac, end to end: a plain hire made while the owner had the sandbox off (--sandbox-off, from the
// core) starts its agent with the real launcher outside any sandbox, its own sandbox as the owner set it, as it would
// without Smolcorp. With the switch back on in the config's file as it starts, it starts in its sandbox as before, in
// a folder that holds no repo; in a repo it is refused before anything starts (sandboxOnAgainWords).
// Stand-ins only: a stand-in agent, secret, fence and app (launchersandbox_darwin_test.go).
func TestOnAMacWithTheSandboxOffAPlainHiresAgentStartsAsWithoutSmolcorp(t *testing.T) {
	macSandboxHere(t)
	token := strings.Repeat("AB", 16)
	app := &appStandIn{}
	port := app.serve(t)
	for _, c := range []struct {
		row, agent, version string
		fileOff             bool
		secret, args        string // what the agent found, and what it was started with
		geminiSandbox       string
		inRepo              bool // its folder a repo's top: refused, the agent never run
	}{
		// Its own sandbox left as the owner set it: no words, GEMINI_SANDBOX as inherited.
		{"Gemini CLI, the switch off", "gemini", "0.9.0", true, "read", "", "true", false},
		{"Codex, the switch off", "codex", "codex-cli 0.46.0", true, "read", "", "true", false},
		// Not asked for its version: nothing of its own sandbox is switched off, so an old one starts too.
		{"an old Codex, the switch off", "codex", "codex-cli 0.19.9", true, "read", "", "true", false},
		// The core said off at the hire, but the switch reads on again as it starts: in its sandbox, as before. Claude
		// Code's, the one plain hire that starts in a Mac's sandbox in 1.26.0 (FXPLANTSTERM's notonamac.go: a Codex is
		// refused there, TestOnAMacTheLauncherRefusesEveryAgentButClaudeCodeBeforeItStarts).
		{"Claude Code, the switch on again", "claude", "2.1.300 (Claude Code)", false, "denied", `--settings {"sandbox":{"enabled":false}}`, "true", false},
		// The same in a repo (RT126STACKP3 N1): the office made it no worktree, so in its sandbox it would work in the
		// owner's checkout, which ruling (W) keeps every sandboxed plain hire out of. Its launcher refuses it, says why,
		// and makes nothing of a desk.
		{"Claude Code, the switch on again, in a repo", "claude", "2.1.300 (Claude Code)", false, "", "", "", true},
	} {
		t.Run(c.row, func(t *testing.T) {
			bin, folder, sockets, files := plainProbeAgent(t, c.agent, c.version), t.TempDir(), shortDeskSockets(t), t.TempDir()
			if c.inRepo {
				standInOwnersGit(t, folder, "init", "-q")
			}
			config := filepath.Join(files, "config.json")
			writeFile(t, config, map[bool]string{true: `{"sandboxOff": true}`, false: `{}`}[c.fileOff])
			office := string(must(json.Marshal(map[string]string{"Config": config, "HookKey": filepath.Join(files, "hook-key")})))
			env := []string{testDeskSocketsVariable + "=" + sockets, testHookKeyVariable + "=fake-hook-key-1", testSecretVariable + "=1",
				"SMOLCORP_PORT=" + strconv.Itoa(port), "GEMINI_SANDBOX=true", launcherStandInOfficeVariable + "=" + office}
			probe, said := runPlainLauncher(t, []string{"--hire", token, "--sandbox-off", c.agent}, bin, folder, env...)
			switch {
			case c.inRepo:
				if !strings.Contains(said, plainHireSandboxOnAgain) {
					t.Errorf("the launcher said %q, want its refusal %q", said, plainHireSandboxOnAgain)
				}
				if _, err := os.Lstat(filepath.Join(folder, "probe")); !os.IsNotExist(err) {
					t.Errorf("the agent ran (its probe: %v): %q", err, probe)
				}
			case probe["secret"] != c.secret:
				t.Fatalf("the agent found the secret %q, want %q (the launcher said %q)", probe["secret"], c.secret, said)
			case probe["args"] != c.args || probe["gemini-sandbox"] != c.geminiSandbox:
				t.Errorf("its own sandbox: started with %q and GEMINI_SANDBOX %q, want %q and %q", probe["args"], probe["gemini-sandbox"], c.args, c.geminiSandbox)
			}
			if left, _ := filepath.Glob(filepath.Join(sockets, "*")); len(left) != 0 {
				t.Errorf("left in the socket folder: %q", left)
			}
		})
	}
}

// waitingAgent writes a stand-in agent called name into a folder of its own, which says version to --version, and
// otherwise runs until ./done is in the folder it runs in (half a minute at most).
func waitingAgent(t *testing.T, name, version string) string {
	t.Helper()
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, name), `#!/bin/sh
if [ "$1" = --version ]; then echo '`+version+`'; exit 0; fi
i=0
while [ ! -e ./done ] && [ "$i" -lt 300 ]; do /bin/sleep 0.1; i=$((i+1)); done
`)
	if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// A launcher whose plain hire started outside its sandbox says so (sandbox=off) each time it connects to the app, so
// its desk keeps its No sandbox tag when the core is back from a restart, its pending hire long gone
// (TestALauncherWithoutItsSandboxKeepsItsTagAfterARestart is the core's half). The real launcher, against an app
// stand-in that knows no pending hire and ends each stream as a core that restarts does, so the launcher connects
// again. With the switch on, its agent started in its sandbox, and it says nothing of the kind.
func TestOnAMacALauncherWithoutItsSandboxSaysSoEachTimeItConnects(t *testing.T) {
	macSandboxHere(t)
	token := strings.Repeat("AB", 16)
	for _, off := range []bool{true, false} {
		t.Run(map[bool]string{true: "the switch off", false: "the switch on"}[off], func(t *testing.T) {
			app := &appStandIn{}
			port := app.serve(t)
			// With the switch on, Claude Code: the one plain hire that starts in a Mac's sandbox in 1.26.0 (FXPLANTSTERM).
			agent, version := "codex", "codex-cli 0.46.0"
			if !off {
				agent, version = "claude", "2.1.300 (Claude Code)"
			}
			bin, folder, files := waitingAgent(t, agent, version), t.TempDir(), t.TempDir()
			config := filepath.Join(files, "config.json")
			writeFile(t, config, map[bool]string{true: `{"sandboxOff": true}`, false: `{}`}[off])
			office := string(must(json.Marshal(map[string]string{"Config": config, "HookKey": filepath.Join(files, "hook-key")})))
			args := []string{"--hire", token, agent}
			if off {
				args = []string{"--hire", token, "--sandbox-off", agent}
			}
			env := []string{testDeskSocketsVariable + "=" + shortDeskSockets(t), testHookKeyVariable + "=fake-hook-key-1",
				"SMOLCORP_PORT=" + strconv.Itoa(port), launcherStandInOfficeVariable + "=" + office}
			ended := make(chan string, 1)
			go func() {
				_, said := runPlainLauncher(t, args, bin, folder, env...)
				ended <- said
			}()
			connections := func() []url.Values {
				app.mu.Lock()
				defer app.mu.Unlock()
				return slices.Clone(app.launchers)
			}
			// Its first connection, and the one after the stream ended.
			for deadline := time.Now().Add(20 * time.Second); len(connections()) < 2 && time.Now().Before(deadline); {
				time.Sleep(50 * time.Millisecond)
			}
			writeFile(t, filepath.Join(folder, "done"), "")
			said := <-ended
			seen := connections()
			if len(seen) < 2 {
				t.Fatalf("the launcher connected %d times, want it back after its stream ended (it said %q)", len(seen), said)
			}
			want := map[bool]string{true: "off"}[off]
			for i, query := range seen {
				if query.Get("hire") != token || query.Get("id") != seen[0].Get("id") || query.Get("sandbox") != want {
					t.Errorf("connection %d: %v, want hire %s and sandbox %q", i+1, query, token, want)
				}
			}
		})
	}
}

// A General Manager hired the plain way while the sandbox was off starts outside it, and is told so as it starts, as
// the engine's is (setup.html: "The General Manager is told when it starts"). With the switch on, it starts in its
// sandbox, and isn't. The real core hires it and gives its launcher its token; the real launcher starts it; a stand-in
// Claude Code keeps the first message it was started with.
func TestOnAMacAPlainGMStartedWithoutItsSandboxIsToldSo(t *testing.T) {
	macSandboxHere(t)
	claude := t.TempDir()
	writeFile(t, filepath.Join(claude, "claude"), `#!/bin/sh
[ "$1" = --version ] && { echo '2.1.400 (Claude Code)'; exit 0; }
for word; do last=$word; done
printf '%s' "$last" > ./message
`)
	if err := os.Chmod(filepath.Join(claude, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, off := range []bool{true, false} {
		t.Run(map[bool]string{true: "the switch off", false: "the switch on"}[off], func(t *testing.T) {
			d, h := plainGMDeck(t)
			d.config.SandboxOff = off
			args, _ := plainRoleHire(t, d, h, gmRole)
			if slices.Contains(args, "--sandbox-off") != off {
				t.Fatalf("the GM's window runs %q", args)
			}
			server := httptest.NewServer(&router{deck: d})
			defer server.Close()
			port := server.URL[strings.LastIndex(server.URL, ":")+1:]
			n, err := strconv.Atoi(port)
			if err != nil {
				t.Fatal(err)
			}
			swap(t, &Port, n)
			config := filepath.Join(t.TempDir(), "config.json")
			writeFile(t, config, string(must(json.Marshal(map[string]any{"roles": d.config.Roles, "sandboxOff": off}))))
			office := string(must(json.Marshal(map[string]string{"config": config, "hookKey": hookKeyFile})))
			folder := t.TempDir()
			_, said := runPlainLauncher(t, args, claude, folder, "SMOLCORP_PORT="+port, launcherStandInOfficeVariable+"="+office,
				"SMOLCORP_HIRE_KEY=", testDeskSocketsVariable+"="+shortDeskSockets(t))
			message, err := os.ReadFile(filepath.Join(folder, "message"))
			if err != nil {
				t.Fatalf("the GM didn't start: %v (the launcher said %q)", err, said)
			}
			if told := strings.Contains(string(message), gmSandboxOffLine); told != off || off && !strings.HasSuffix(string(message), " "+gmSandboxOffLine) {
				t.Errorf("the GM's first message: %q; want it told the sandbox is off: %v", message, off)
			}
		})
	}
}
