package desktop

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// homeFiles is every file under the home folder, by path, with its contents: what a test copy must leave as it was.
func homeFiles(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(homeDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// A test copy (on a port of its own, any computer) never writes your agents' settings, as the Mac's test copy
// (Hooks.swift's testCopy): connecting, disconnecting and the self-test's repair are refused in the Mac's words, and
// the start-up refresh and OpenCode's look leave them be, even an entry it would have upgraded. Its self-test line is
// the Mac's: ticked, with no Connect to press. (C1's A1: a Go test copy rewrote or removed the real app's hooks.)
func TestATestCopyLeavesYourAgentsSettingsAlone(t *testing.T) {
	d := newTestDeck(t)
	// The real office's hooks, on the default port, in every agent's settings; Claude Code's Stop as an earlier
	// version wrote it, with no port, which a refresh upgrades; Grok's file under the name the app had before.
	onDefaultPort(t)
	for _, agent := range agents {
		if err := os.MkdirAll(agentFolder(agent), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := connectAgents(false); err != nil {
		t.Fatal(err)
	}
	settings := entriesFor(Claude).file
	writeFile(t, settings, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"'/home/me/.local/bin/smolcorp' hook","async":true}]}]},"theme":"mine"}`)
	writeFile(t, oldGrokHooks(), "{}")
	before := homeFiles(t)

	onPort(t, 7799)
	host := "localhost:7799"
	for _, target := range []string{"/setup/hooks", "/setup/hooks?remove=1", "/setup/fix?fix=repair", "/setup/fix?fix=repair&auto=1"} {
		w := serve(t, d, call{method: "POST", target: target, host: host, from: thisComputer, headers: map[string]string{setupHeader: "1"}})
		if w.Code != http.StatusConflict || w.Body.String() != errTestCopyAgents.Error() {
			t.Errorf("POST %s on a test copy: %d %q, want 409 and the Mac's words", target, w.Code, w.Body)
		}
	}
	refreshHooks()
	lookAtOpenCode()
	forgetOpenCode()
	if _, err := connectAgents(true); err != errTestCopyAgents {
		t.Errorf("connecting on a test copy: %v", err)
	}
	if err := disconnectAgents(); err != errTestCopyAgents {
		t.Errorf("disconnecting on a test copy: %v", err)
	}
	if err := connect(Claude, false); err != errTestCopyAgents {
		t.Errorf("connecting Claude Code on a test copy: %v", err)
	}
	if err := writePlugin(); err != errTestCopyAgents {
		t.Errorf("writing OpenCode's plugin on a test copy: %v", err)
	}
	if err := writeSettings(map[string]any{}, settings, true); err != errTestCopyAgents {
		t.Errorf("writing settings on a test copy: %v", err)
	}
	after := homeFiles(t)
	for path, was := range before {
		if after[path] != was {
			t.Errorf("a test copy changed %s", strings.TrimPrefix(path, homeDir))
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("a test copy wrote %s", strings.TrimPrefix(path, homeDir))
		}
	}
	d.mu.Lock()
	disconnected := d.config.HooksDisconnected
	d.mu.Unlock()
	if disconnected {
		t.Error("a refused disconnect was kept as disconnected")
	}
	// Marked a test copy (a support folder beside the port), as the Mac's is.
	t.Setenv(testSupportVariable, t.TempDir())
	for _, c := range d.selfTest() {
		if c.ID == "hooks" && (c.Status != checkOK || c.Fix != "" || !strings.HasPrefix(c.Detail, "Your hires report to this test copy through its own hook.")) {
			t.Errorf("a test copy's hooks line is %+v, want the Mac's: ticked, with nothing to press", c)
		}
	}
}

// On the default port, the office's own: connecting still writes, as before.
func TestTheOfficeOnTheDefaultPortStillConnects(t *testing.T) {
	d := newTestDeck(t)
	onDefaultPort(t)
	writeFile(t, filepath.Join(homeDir, ".claude", "settings.json"), "{}")
	w := serve(t, d, call{method: "POST", target: "/setup/hooks", host: "localhost:7710", from: thisComputer, headers: map[string]string{setupHeader: "1"}})
	if w.Code != http.StatusOK || !isConnected(Claude) {
		t.Fatalf("connecting on the default port: %d %s", w.Code, w.Body)
	}
}
