package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXPLANTSTERMR4 (the R3 review's N1), as a hire's start makes its sandbox, in a stand-in home folder: a long-used
// OpenCode's snapshot gits (the review's 200,100 files) no longer stop a Claude Code hire, which got "Update Smolcorp"
// for every hire. A folder at the top of an agent's own that is still too big to look through refuses the start in
// words that say which folder and what to do, never "Update Smolcorp". None of the owner's own variables that move an
// agent's folder is read here: each is set empty for the test.
func TestOnAMacABigAgentFolderDoesntStopAClaudeCodeHireAndATooBigOneSaysWhatToDo(t *testing.T) {
	shortSupport(t)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	saved := homeDir
	homeDir = home
	t.Cleanup(func() { homeDir = saved })
	deskSockets(t)
	for _, variable := range []string{"CODEX_HOME", "CODEX_SQLITE_HOME", "GEMINI_CLI_HOME", "GEMINI_CLI_TRUSTED_FOLDERS_PATH",
		"GEMINI_CLI_SYSTEM_SETTINGS_PATH", "GEMINI_CLI_SYSTEM_DEFAULTS_PATH", "GROK_HOME", "GROK_CONFIG_PATH", "XDG_CONFIG_HOME",
		"XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG", "GROK_AUTH_PATH",
		"GROK_TRACE_UPLOAD_CREDENTIALS_FILE", "OPENCODE_DB"} {
		t.Setenv(variable, "")
	}
	if homes, signIns := sandbox.AgentPlaces(os.Getenv); len(homes)+len(signIns) != 0 {
		t.Fatalf("an agent's place from the environment is still read: %q %q", homes, signIns)
	}
	fill := func(folder string, folders, files int) {
		t.Helper()
		for i := range folders {
			dir := filepath.Join(folder, fmt.Sprintf("%02x", i%256), fmt.Sprintf("d%d", i))
			if folders == 1 {
				dir = folder
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			for j := range files {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("o%d", j)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	start := func(id string) error {
		t.Helper()
		if err := os.MkdirAll(hireFolder(supportDir, id), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := makeDeskSandbox(deskAt{ID: id, Root: t.TempDir(), Socket: deskSocketPath(id, 0), Git: runGit})
		return err
	}

	fill(filepath.Join(home, ".local", "share", "opencode", "snapshot", "p", "objects"), 2001, 100)
	if err := start("ab12"); err != nil {
		t.Fatalf("200,100 files in OpenCode's snapshots stop a Claude Code hire: %v", err)
	}

	fill(filepath.Join(home, ".grok", "big"), 1, 50_001)
	err = start("ab13")
	if err == nil {
		t.Fatal("a folder at the top of ~/.grok with 50,001 files in it didn't stop the start")
	}
	words := err.Error()
	for _, want := range []string{"Smolcorp didn't start this hire.", "it stopped in " + filepath.Join("~", ".grok", "big") + " after 50,000 files",
		"Delete or move out what you no longer need there", "then start the hire again."} {
		if !strings.Contains(words, want) {
			t.Errorf("the words don't say %q: %q", want, words)
		}
	}
	if strings.Contains(words, "Update Smolcorp") || words == sandboxNotMade {
		t.Errorf("the words tell the owner to update Smolcorp, which wouldn't help: %q", words)
	}
}
