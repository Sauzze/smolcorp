//go:build !windows

package desktop

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R-MEMDATA and R-NOLEAVE's canary (MEM4): a kept note, then a GM hired through the engine and a read of its memory
// through the route, and the GM's own hook events about it. The note's words are in the read's answer alone: not in
// the GM's start (its message, command line and environment), not in the office's snapshot, the viewer's or a peer's
// feed, not in any file of the GM's folders, and not in any file of the support folder but the store's own (the
// event log among them).
func TestTheCanaryReachesTheGMOnlyThroughTheRoute(t *testing.T) {
	d, r, _ := engineDeck(t)
	d.engine.HeardFrom = func(Hire) bool { return true }
	managerOn(d)
	keptNote(t, d, canaryNote)
	if status, message := d.Perform("hire", "", url.Values{"role": {"gm"}}); status != http.StatusNoContent {
		t.Fatalf("hire: %d %s", status, message)
	}
	d.watching.Wait()
	gm := registered(t)[0]
	script := r.script(gm)
	if strings.Contains(strings.ToLower(script), "pelican") {
		t.Errorf("the GM's start holds the canary:\n%s", script)
	}
	if message := scriptMessage(script); strings.Count(message, gmMemorySentence) != 1 || strings.Contains(message, memoryHeader) {
		t.Errorf("the GM's start message: %q", message)
	}

	_, answer, text := readAsGM(t, d, d.newGMToken(), "?name=Pelican+release+notes")
	if len(answer.Notes) != 1 || answer.Notes[0].Body != canaryNote.Body {
		t.Fatalf("the read: %s", text)
	}
	d.watching.Wait()

	// The GM's own hook events (RT126MEM4 N6, the reviewer's second probe), through the real /hook route: a Bash call
	// proposing the canary, asked for, then what it gave back under the header, and the MCP tool's call and result.
	// After each, the canary is in none of the office's snapshot, the viewer's or a peer's feed.
	gmDeskAt(t, d, "LG", "gm9")
	propose := map[string]any{"command": "smolcorp gm memory propose --name 'Pelican release notes' --body '" + canaryNote.Body + "'",
		"description": "Proposing the pelican release notes"}
	held := map[string]any{"stdout": memoryHeader + "\n\n- Pelican release notes (reference): " + canaryNote.Description}
	mcp := map[string]any{"name": canaryNote.Name, "description": canaryNote.Description, "type": "reference", "body": canaryNote.Body,
		"seen": "ls docs"}
	for _, event := range []map[string]any{
		{"hook_event_name": "PreToolUse", "tool_name": "Bash", "tool_input": propose},
		{"hook_event_name": "PermissionRequest", "tool_name": "Bash", "tool_input": propose},
		{"hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_input": propose, "tool_response": held},
		{"hook_event_name": "PreToolUse", "tool_name": "mcp__plugin_smolcorp-gm_smolcorp__memory_propose", "tool_input": mcp},
		{"hook_event_name": "PostToolUse", "tool_name": "mcp__plugin_smolcorp-gm_smolcorp__memory_propose", "tool_input": mcp,
			"tool_response": `{"id":"X","name":"` + canaryNote.Name + `"}`},
	} {
		event["session_id"], event["cwd"] = "gm9", "/work/office"
		got := serve(t, d, call{method: "POST", target: "/hook", host: "127.0.0.1:7710", from: thisComputer, body: jsonLine(event),
			headers: map[string]string{hookKeyHeader: d.access.HookKey, "X-AgentDeck-Agent": "claude", "X-AgentDeck-Tab": "launcher:LG"}})
		if got.Code != http.StatusNoContent {
			t.Fatalf("the hook's %s of %s: %d %s", event["hook_event_name"], event["tool_name"], got.Code, got.Body)
		}
		d.mu.Lock()
		office := d.snapshot()
		viewer, _ := json.Marshal(d.viewerSnapshot(false, func(id string) string { return id }))
		d.mu.Unlock()
		for what, data := range map[string][]byte{"the office's snapshot": office, "the viewer's": viewer, "a peer's feed": peerFeedOf(office)} {
			if strings.Contains(strings.ToLower(string(data)), "pelican") {
				t.Errorf("after the %s of %s, %s holds the canary: %s", event["hook_event_name"], event["tool_name"], what, data)
			}
		}
	}
	d.watching.Wait()

	store := filepath.Join(supportDir, memoryFolder)
	for _, root := range []string{supportDir, gm.Folder, gm.Worktree, hireFolder(supportDir, gm.ID)} {
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || path == store || strings.HasPrefix(path, store+string(filepath.Separator)) {
				return nil
			}
			if data, err := os.ReadFile(path); err == nil && strings.Contains(strings.ToLower(string(data)), "pelican") {
				t.Errorf("%s holds the canary", path)
			}
			return nil
		})
	}
}
