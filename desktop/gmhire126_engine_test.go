//go:build !windows

package desktop

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXGMHIRE126 on the tmux engine's way (Linux's, and a Mac's): the GM's hire takes the model and nickname it names,
// and a GM in a folder that isn't a repo keeps its brief and board in its own folder, where its fence lets it write.

// #4: `gm hire --model opus --nickname Ada` on the engine: 200, the row has both, the hire's worktree and branch are
// named for Ada, and its agent starts on opus (the janitor's own model is sonnet). A model that isn't one is refused.
func TestTheGMHireOnTheEngineHonoursAModelAndANickname(t *testing.T) {
	d, r, h, token := gmHireDeck(t)
	for _, body := range []string{`{"role":"janitor","model":"op us"}`, `{"role":"janitor","model":["opus"]}`} {
		if code, text := postGMHire(t, d, asGMCaller, token, body); code != http.StatusBadRequest || text != hireModelRefusal {
			t.Errorf("%s: %d %q, want 400 %q", body, code, text, hireModelRefusal)
		}
	}
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","nickname":"Ada Lovelace"}`); code != http.StatusBadRequest || text != hireNicknameRefusal {
		t.Errorf("a nickname of two lines: %d %q", code, text)
	}
	nothingMade(t, r, h)
	code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor","model":"opus","nickname":"Ada"}`)
	if code != http.StatusOK {
		t.Fatalf("hire: %d %q", code, text)
	}
	hires := registered(t)
	if len(hires) != 1 || hires[0].Model != "opus" || hires[0].Nickname != "Ada" {
		t.Fatalf("registry %+v", hires)
	}
	if !strings.Contains(filepath.Base(hires[0].Worktree.storedOnly()), "ada") || !strings.Contains(hires[0].Branch, "/ada-") {
		t.Errorf("the worktree %q and branch %q aren't named for the nickname", hires[0].Worktree, hires[0].Branch)
	}
	if script := r.script(hires[0]); !strings.Contains(script, "'--model=opus'") || strings.Contains(script, "--model=sonnet") {
		t.Errorf("the hire doesn't start on opus:\n%s", script)
	}
}

// #5: a GM hired on the engine into a folder that isn't a repo: no worktree, and its brief, status and TEAM-BOARD are
// in hires/<id>/, which its fence lets it edit; Smolcorp's briefs folder stays denied, and nothing is written there.
func TestAGMOnTheEngineInAFolderThatIsNoRepoKeepsItsBriefInItsOwnFolder(t *testing.T) {
	d, _, _, _ := gmHireDeck(t)
	notes := filepath.Join(homeDir, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"role": gmRole, "folder": notes})
	if code, text := postGMHire(t, d, asPageCaller, "", string(body)); code != http.StatusOK {
		t.Fatalf("the GM's hire: %d %q", code, text)
	}
	hires := registered(t)
	if len(hires) != 1 || hires[0].Role != gmRole || !hires[0].Worktree.none() {
		t.Fatalf("registry %+v", hires)
	}
	gm, own := hires[0], hireFolder(supportDir, hires[0].ID)
	if gm.Brief != filepath.Join(own, "BRIEF.md") {
		t.Fatalf("the GM's brief is %q, want it in %s", gm.Brief, own)
	}
	if brief, err := os.ReadFile(gm.Brief); err != nil || !strings.Contains(string(brief), tildePath(filepath.Join(own, "TEAM-BOARD.md"))) {
		t.Errorf("the brief doesn't name its board beside it (%v):\n%s", err, brief)
	}
	if left := briefsWritten(); len(left) != 0 {
		t.Errorf("a brief was written in Smolcorp's briefs folder: %v", left)
	}
	var settings struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(fileSettings(filepath.Join(own, hireSettingsFile))), &settings); err != nil {
		t.Fatal(err)
	}
	edits := toolRules(settings.Permissions.Deny, "Edit")
	for _, name := range []string{"STATUS.md", "TEAM-BOARD.md"} {
		if file := filepath.Join(own, name); readDenied(edits, homeDir, file) {
			t.Errorf("Edit of %s is denied", file)
		}
	}
	if !readDenied(edits, homeDir, filepath.Join(supportDir, "briefs", "0-tester-20261118-0900.md")) {
		t.Errorf("Edit of Smolcorp's briefs folder isn't denied")
	}
}

// FXGMHIRE126's note: an engine hire off a Mac starts in Claude Code's default mode (engineAutoMode), as a launcher
// hire does, and its brief in hires/<id>/ is outside its working folder: its settings let it read its own folder
// without a question, and no other hire's. On a Mac, in auto mode, they allow nothing, as before.
func TestAnEngineHireInDefaultModeReadsItsOwnFolderWithoutAsking(t *testing.T) {
	d, _, _, token := gmHireDeck(t)
	if code, text := postGMHire(t, d, asGMCaller, token, `{"role":"janitor"}`); code != http.StatusOK {
		t.Fatalf("hire: %d %q", code, text)
	}
	hires := registered(t)
	if len(hires) != 1 {
		t.Fatalf("registry %+v", hires)
	}
	own := hireFolder(supportDir, hires[0].ID)
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(fileSettings(filepath.Join(own, hireSettingsFile))), &settings); err != nil {
		t.Fatal(err)
	}
	allow := settings.Permissions.Allow
	if engineAutoMode {
		if len(allow) != 0 {
			t.Errorf("a hire in auto mode is allowed %q", allow)
		}
		return
	}
	if len(allow) != 1 || allow[0] != ownFolderReadRule(supportDir, hires[0].ID) {
		t.Fatalf("the hire's settings allow %q, want only its own folder's Read", allow)
	}
	for file, want := range map[string]bool{
		filepath.Join(own, "BRIEF.md"):                                  true,
		filepath.Join(own, "notes", "plan.md"):                          true,
		filepath.Join(supportDir, "hires", hires[0].ID+"x", "BRIEF.md"): false,
		filepath.Join(supportDir, "registry.json"):                      false,
	} {
		if got := readDenied(toolRules(allow, "Read"), homeDir, file); got != want {
			t.Errorf("Read of %s allowed: %v, want %v", file, got, want)
		}
	}
}
