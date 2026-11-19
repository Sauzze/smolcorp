package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	for _, want := range []string{"Smolcorp didn't start this hire.",
		"it stopped in " + filepath.Join("~", ".grok", "big") + " after 50,000 files and folders in " + filepath.Join("~", ".grok") + ", the most",
		"Delete or move out what you no longer need there", "then start the hire again."} {
		if !strings.Contains(words, want) {
			t.Errorf("the words don't say %q: %q", want, words)
		}
	}
	if strings.Contains(words, "Update Smolcorp") || words == sandboxNotMade {
		t.Errorf("the words tell the owner to update Smolcorp, which wouldn't help: %q", words)
	}
}

// FXPOLISHA (the PLANTSTERMR4 review's O4; the owner is a heavy Claude Code user): a long-used ~/.claude, 300,000 files
// in Claude Code's own stores (its transcripts, file history, to-do lists, shell snapshots, debug logs and hooks'
// environments), stood in for in a stand-in home, no longer stops a hire's start: neither the link walk that makes its
// Policy nor the link check after it. The owner's settings, kept as a link at the top of ~/.claude into a dotfiles
// folder, are still held where the link leads. And a big skills folder (60,000 files, a skill's node_modules, say),
// which is looked into whole (FXPOLISHAR1), doesn't stop it either: its files count on the budget of every folder
// looked into whole, as before FXPOLISHA, not on ~/.claude's own (FXPOLISHAR2, RT126POLISHAR1's N1). A skill's file kept
// as a link five levels in is held where it leads. Nor, in another stand-in home, do 25,000 files in skills after
// 30,000 in paste-cache, which together are past ~/.claude's own 50,000.
func TestOnAMacABigClaudeFolderDoesntStopAHireAndATopLinkIsStillHeld(t *testing.T) {
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
	claude := filepath.Join(home, ".claude")
	made := 0
	fill := func(folder string, folders, files int) {
		t.Helper()
		for i := range folders {
			dir := filepath.Join(claude, folder, fmt.Sprintf("s%d", i))
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			for j := range files {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d", j)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				made++
			}
		}
	}
	fill("file-history", 1000, 100)
	fill("todos", 1, 60_000)
	fill("shell-snapshots", 1, 40_000)
	fill("debug", 1, 40_000)
	fill("projects", 3, 10_000)
	fill("session-env", 300, 100)
	if made != 300_000 {
		t.Fatalf("made %d files, not 300,000", made)
	}
	// skills makes n skills in claude, each 10 folders of 100 files four levels in (a node_modules, say).
	skills := func(claude string, n int) {
		t.Helper()
		made := 0
		for i := range n {
			for j := range 10 {
				dir := filepath.Join(claude, "skills", fmt.Sprintf("s%d", i), "refs", fmt.Sprintf("r%d", j))
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				for k := range 100 {
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.md", k)), nil, 0o600); err != nil {
						t.Fatal(err)
					}
					made++
				}
			}
		}
		if made != n*1000 {
			t.Fatalf("made %d skill files, not %d", made, n*1000)
		}
	}
	skills(claude, 60)
	settings := filepath.Join(home, "dotfiles", "claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../dotfiles/claude/settings.json", filepath.Join(claude, "settings.json")); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(home, "dotfiles", "claude", "skill.md")
	if err := os.WriteFile(skill, []byte("the owner's"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../../../../dotfiles/claude/skill.md", filepath.Join(claude, "skills", "s49", "refs", "r9", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(hireFolder(supportDir, "ab14"), 0o700); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	box, err := makeDeskSandbox(deskAt{ID: "ab14", Root: t.TempDir(), Socket: deskSocketPath("ab14", 0), Git: runGit})
	if err != nil {
		t.Fatalf("300,000 files and a 60,000-file skills folder in ~/.claude stop a Claude Code hire: %v", err)
	}
	t.Logf("the start with 300,000 + 60,000 took %v", time.Since(began).Round(time.Millisecond))
	for _, at := range []string{filepath.Join(claude, "settings.json"), settings, skill} {
		if v, by := box.policy.Decide(at, sandbox.Write); v != sandbox.Deny {
			t.Errorf("the owner's file at %s: %v (%s), want held", at, v, by.Row)
		}
	}
	if v, by := box.policy.Decide(filepath.Join(home, "dotfiles", "notes"), sandbox.Write); v != sandbox.Allow {
		t.Errorf("control: the rest of the dotfiles folder: %v (%s), want the desk's", v, by.Row)
	}

	// 25,000 files in skills, after 30,000 in a folder at the top of ~/.claude that is no bulk store (paste-cache).
	other, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	homeDir = other
	claude = filepath.Join(other, ".claude")
	pastes := filepath.Join(claude, "paste-cache")
	if err := os.MkdirAll(pastes, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := range 30_000 {
		if err := os.WriteFile(filepath.Join(pastes, fmt.Sprintf("p%d.txt", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	skills(claude, 25)
	if err := os.MkdirAll(hireFolder(supportDir, "ab15"), 0o700); err != nil {
		t.Fatal(err)
	}
	began = time.Now()
	if _, err := makeDeskSandbox(deskAt{ID: "ab15", Root: t.TempDir(), Socket: deskSocketPath("ab15", 0), Git: runGit}); err != nil {
		t.Fatalf("25,000 files in ~/.claude/skills after 30,000 in its paste-cache stop a Claude Code hire: %v", err)
	}
	t.Logf("the start with 25,000 + 30,000 took %v", time.Since(began).Round(time.Millisecond))
}

// FXPOLISHAR3 (RT126POLISHAR2's N1, option a; the owner is a heavy Claude Code user): the rest of ~/.claude,
// looked into two levels deep, and where a link there leads, count on the budget of every folder held whole, as before
// FXPOLISHA, when ~/.claude was looked into whole, not on 50,000 of its own. So none of these stand-in homes, each of
// which started a hire then, stops one now: 60,000 files flat in paste-cache; 30,000 there and 30,000 in plans; and
// the plugins' big2 kept as a link to a folder of 60,000 files, which is still held where it leads.
func TestOnAMacTheRestOfABigClaudeFolderDoesntStopAHire(t *testing.T) {
	shortSupport(t)
	saved := homeDir
	t.Cleanup(func() { homeDir = saved })
	deskSockets(t)
	for _, variable := range []string{"CODEX_HOME", "CODEX_SQLITE_HOME", "GEMINI_CLI_HOME", "GEMINI_CLI_TRUSTED_FOLDERS_PATH",
		"GEMINI_CLI_SYSTEM_SETTINGS_PATH", "GEMINI_CLI_SYSTEM_DEFAULTS_PATH", "GROK_HOME", "GROK_CONFIG_PATH", "XDG_CONFIG_HOME",
		"XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "OPENCODE_CONFIG_DIR", "OPENCODE_CONFIG", "GROK_AUTH_PATH",
		"GROK_TRACE_UPLOAD_CREDENTIALS_FILE", "OPENCODE_DB"} {
		t.Setenv(variable, "")
	}
	// fill makes n files flat in folder.
	fill := func(folder string, n int) {
		t.Helper()
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
		for i := range n {
			if err := os.WriteFile(filepath.Join(folder, fmt.Sprintf("f%d", i)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, c := range []struct {
		name string
		make func(home string)
		held string // a place where a link leads, held after the start
	}{
		{"60,000 files flat in ~/.claude/paste-cache", func(h string) { fill(filepath.Join(h, ".claude", "paste-cache"), 60_000) }, ""},
		{"30,000 in ~/.claude/paste-cache and 30,000 in ~/.claude/plans", func(h string) {
			fill(filepath.Join(h, ".claude", "paste-cache"), 30_000)
			fill(filepath.Join(h, ".claude", "plans"), 30_000)
		}, ""},
		{"~/.claude/plugins/big2 kept as a link to a folder of 60,000 files", func(h string) {
			fill(filepath.Join(h, "code", "big2"), 60_000)
			if err := os.MkdirAll(filepath.Join(h, ".claude", "plugins"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../../code/big2", filepath.Join(h, ".claude", "plugins", "big2")); err != nil {
				t.Fatal(err)
			}
		}, filepath.Join("code", "big2", "new.json")},
	} {
		home, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		homeDir = home
		c.make(home)
		id := fmt.Sprintf("ab2%d", i)
		if err := os.MkdirAll(hireFolder(supportDir, id), 0o700); err != nil {
			t.Fatal(err)
		}
		began := time.Now()
		box, err := makeDeskSandbox(deskAt{ID: id, Root: t.TempDir(), Socket: deskSocketPath(id, 0), Git: runGit})
		if err != nil {
			t.Fatalf("a Claude Code hire with %s: the start is refused: %v", c.name, err)
		}
		t.Logf("the start with %s took %v", c.name, time.Since(began).Round(time.Millisecond))
		if c.held != "" {
			if v, by := box.policy.Decide(filepath.Join(home, c.held), sandbox.Write); v != sandbox.Deny {
				t.Errorf("%s: a write where the link leads: %v (%s), want held", c.name, v, by.Row)
			}
		}
		if err := os.RemoveAll(home); err != nil {
			t.Fatal(err)
		}
	}
}

// FXPOLISHA (the PLANTSTERMR4 review's O3): the count in the too-big words is the folder's own when the folder was
// looked through on its own budget, and said to be everything's when it was looked through with the others. And what
// to do fits the folder (FXPOLISHAR2, RT126POLISHAR1's O1): old sessions, caches and logs for a store; for Claude
// Code's skills, commands, agents and hooks, or where a link in one leads, a skill's own dependencies, which it looks
// through wherever a link leads, so they are removed or kept where no link leads (FXPOLISHAR3, RT126POLISHAR2's O1). A
// folder's own count stopped in a folder inside it says whose count it is (FXPOLISHAR3, RT126POLISHAR2's O2).
func TestTheTooBigWordsSayWhoseCountItIs(t *testing.T) {
	own := homeTooBig(&sandbox.HomeTooBigError{Folder: "/h/.grok", Inside: "/h/.grok/big", Most: 50_000})
	top := homeTooBig(&sandbox.HomeTooBigError{Folder: "/h/.grok", Inside: "/h/.grok", Most: 50_000})
	shared := homeTooBig(&sandbox.HomeTooBigError{Folder: "/h/.config/fish", Inside: "/h/.config/fish/functions", Most: 200_000, InAll: true})
	skills := homeTooBig(&sandbox.HomeTooBigError{Folder: "/h/.claude", Inside: "/h/.claude/skills", Account: "/h/.claude", Most: 200_000,
		InAll: true, Loaded: true})
	linked := homeTooBig(&sandbox.HomeTooBigError{Folder: "/h/dotfiles/kit", Inside: "/h/dotfiles/kit/node_modules", Account: "/h/.claude-work",
		Most: 200_000, InAll: true, Loaded: true})
	stores := "Delete or move out what you no longer need there (old sessions, caches, logs), then start the hire again."
	loaded := "It looks through all of your skills, commands, agents and hooks, and wherever a link in them leads, a skill's own " +
		"dependencies included (a node_modules folder, a Python venv). Remove what you don't need, or keep it somewhere no link in "
	for _, c := range []struct{ words, want, not string }{
		{own, "it stopped in /h/.grok/big after 50,000 files and folders in /h/.grok, the most it looks through.", "in all"},
		{top, "it stopped in /h/.grok after 50,000 files and folders, the most it looks through.", "in /h/.grok,"},
		{shared, " after looking through 200,000 files and folders in all, the most it looks through.", "after 200,000 files"},
		{own, stores, "node_modules"},
		{top, stores, "node_modules"},
		{shared, stores, "node_modules"},
		{skills, "it stopped in /h/.claude/skills after looking through 200,000 files and folders in all, the most it looks through. " +
			loaded + "/h/.claude leads, then start the hire again.", "old sessions"},
		{linked, "it stopped in /h/dotfiles/kit/node_modules after looking through 200,000 files and folders in all, the most it looks through. " +
			loaded + "/h/.claude-work leads, then start the hire again.", "old sessions"},
		{skills, loaded, "outside"},
		{linked, loaded, "outside"},
	} {
		if !strings.Contains(c.words, c.want) || strings.Contains(c.words, c.not) {
			t.Errorf("the words: %q, want %q and not %q", c.words, c.want, c.not)
		}
	}
}
