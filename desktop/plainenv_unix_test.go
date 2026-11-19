//go:build !windows

package desktop

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// PLAINENV1, through the real start path: the deck's plain hire, its command run by the real launcher in a process
// of its own, whose terminal's environment names another Claude folder. A stand-in claude writes down the
// CLAUDE_CONFIG_DIR it was started with.
func TestAPlainClaudeHiresLauncherStartsItOnItsOwnAccount(t *testing.T) {
	d, h, work := plainHireDeck(t)
	bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "config-dir")
	// It says its version (a Mac starts it in its sandbox, its own switched off for that version: SA-M10).
	writeFile(t, filepath.Join(bin, "claude"), "#!/bin/sh\n[ \"$1\" = --version ] && { echo '2.1.400 (Claude Code)'; exit 0; }\n"+
		"printf '%s' \"${CLAUDE_CONFIG_DIR-(unset)}\" > '"+marker+"'\n")
	if err := os.Chmod(filepath.Join(bin, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(homeDir, ".claude-other")
	for _, c := range []struct{ row, account, want string }{
		{"the default account", claudeDefaultName, "(unset)"},
		{"a confirmed account", "work", work},
	} {
		os.Remove(marker)
		args := plainClaudeHire(t, d, h, c.account)
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), launcherStandInVariable+"="+string(must(json.Marshal(args))),
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "CLAUDE_CONFIG_DIR="+other,
			deskStandInVariable+"=1", testDeskSocketsVariable+"="+shortDeskSockets(t))
		cmd.Dir = t.TempDir()
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: the launcher %q: %v\n%s", c.row, args, err, output)
		}
		got, err := os.ReadFile(marker)
		if err != nil {
			t.Fatalf("%s: the stand-in claude didn't run: %v\n%s", c.row, err, output)
		}
		if string(got) != c.want {
			t.Errorf("%s: claude started with CLAUDE_CONFIG_DIR %q, want %q (the terminal's was %q)", c.row, got, c.want, other)
		}
		if strings.Contains(string(output), "smolcorp:") {
			t.Errorf("%s: the launcher said %q", c.row, output)
		}
	}
}

// shortDeskSockets is a socket folder short enough for a desk's socket, removed after the test.
func shortDeskSockets(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("", "lp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	return filepath.Join(physical(base), "p")
}
