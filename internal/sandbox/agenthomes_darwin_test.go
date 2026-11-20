package sandbox

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// FXPLANTSTERMR3, on this Mac with stand-ins only, under the real profile: what the R2 review's O4 found still open.
// OpenCode's data, cache and state, and the skills in ~/.agents, are written by no desk; no agent's sign-in or token
// file is read by one; and an agent's folder the owner's environment moves elsewhere (CODEX_HOME and the like) is held
// as its folder in the home folder is. A desk's Claude Code still works beside them.

// The places OpenCode runs or loads what's in, and the owner's skills: refused, and nothing of the desk's is there
// afterwards. Links in each are TestOnAMacNoLinkIsMadeInAHiredAgentsFolder's.
func TestOnAMacAHireCantPlantInOpenCodesStateOrTheOwnersSkills(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(append([]string{d.home}, parts...)...) }
	data, cache, state, skills := at(".local", "share", "opencode"), at(".cache", "opencode"), at(".local", "state", "opencode"), at(".agents", "skills")
	owners := map[string]string{
		filepath.Join(cache, "bin", "rg"):                       "#!/bin/sh\n# the owner's",
		filepath.Join(cache, "node_modules", "p", "index.js"):   "// the owner's plugin",
		filepath.Join(cache, "package.json"):                    "{}",
		filepath.Join(data, "opencode.db"):                      "the owner's",
		filepath.Join(data, "log", "opencode.log"):              "the owner's",
		filepath.Join(state, "kv.json"):                         "{}",
		filepath.Join(skills, "s", "SKILL.md"):                  "the owner's skill",
		filepath.Join(d.root, "staged", "SKILL.md"):             "a desk's",
		filepath.Join(d.root, "plugin", "node_modules", "x.js"): "a desk's",
	}
	for file, text := range owners {
		mustWrite(t, file, text)
	}
	d.check(t, []escapeRow{
		// Controls: the owner's files are read; the folders beside them are the desk's.
		{"control: a skill read", attempt{"read", filepath.Join(skills, "s", "SKILL.md"), ""}, "ok"},
		{"control: OpenCode's log read", attempt{"read", filepath.Join(data, "log", "opencode.log"), ""}, "ok"},
		{"control: another cache", attempt{"mkdir", at(".cache", "other"), ""}, "ok"},
		{"control: another program's data", attempt{"mkdir", at(".local", "share", "other"), ""}, "ok"},
		{"control: another program's state", attempt{"mkdir", at(".local", "state", "other"), ""}, "ok"},
		{"control: its worktree writes", attempt{"create", filepath.Join(d.root, "new.go"), ""}, "ok"},
		// OpenCode's cache: the programs it downloads and runs, and the plugins it installs and loads.
		{"OpenCode's rg", attempt{"write", filepath.Join(cache, "bin", "rg"), ""}, "EPERM"},
		{"a program of OpenCode's", attempt{"create", filepath.Join(cache, "bin", "fzf"), ""}, "EPERM"},
		{"an installed plugin", attempt{"write", filepath.Join(cache, "node_modules", "p", "index.js"), ""}, "EPERM"},
		{"a plugin installed", attempt{"mkdir", filepath.Join(cache, "node_modules", "q"), ""}, "EPERM"},
		{"the plugins' list", attempt{"write", filepath.Join(cache, "package.json"), ""}, "EPERM"},
		{"plugins brought in whole", attempt{"rename", filepath.Join(d.root, "plugin", "node_modules"), filepath.Join(cache, "node_modules", "r")}, "EPERM"},
		// OpenCode's data and state.
		{"OpenCode's database", attempt{"write", filepath.Join(data, "opencode.db"), ""}, "EPERM"},
		{"OpenCode's log", attempt{"write", filepath.Join(data, "log", "opencode.log"), ""}, "EPERM"},
		{"OpenCode's sign-in, made", attempt{"create", filepath.Join(data, "auth.json"), ""}, "EPERM"},
		{"OpenCode's state", attempt{"write", filepath.Join(state, "kv.json"), ""}, "EPERM"},
		{"a snapshot's git", attempt{"mkdir", filepath.Join(data, "snapshot"), ""}, "EPERM"},
		{"a plugin in ~/.opencode", attempt{"mkdir", at(".opencode"), ""}, "EPERM"},
		// The owner's skills.
		{"a skill", attempt{"write", filepath.Join(skills, "s", "SKILL.md"), ""}, "EPERM"},
		{"a skill made", attempt{"mkdir", filepath.Join(skills, "new"), ""}, "EPERM"},
		{"a skill brought in whole", attempt{"rename", filepath.Join(d.root, "staged"), filepath.Join(skills, "staged")}, "EPERM"},
		// Gemini CLI's folder in its own sandbox, missing here: not made.
		{"Gemini CLI's folder in its sandbox", attempt{"mkdir", at(".cache", ".gemini"), ""}, "EPERM"},
		// No link made in any of them, symbolic or hard, nor one of them made as a link (as ~/.codex's: R2's links row).
		{"a symlink in OpenCode's cache", attempt{"symlink", filepath.Join(cache, "bin", "fd"), at(".zshrc")}, "EPERM"},
		{"a hard link in OpenCode's cache", attempt{"link", filepath.Join(d.root, "new.go"), filepath.Join(cache, "bin", "linked")}, "EPERM"},
		{"a symlink in OpenCode's data", attempt{"symlink", filepath.Join(data, "made"), at(".zshrc")}, "EPERM"},
		{"a hard link in OpenCode's data", attempt{"link", filepath.Join(d.root, "new.go"), filepath.Join(data, "linked")}, "EPERM"},
		{"a symlink in OpenCode's state", attempt{"symlink", filepath.Join(state, "made"), at(".zshrc")}, "EPERM"},
		{"a symlink in the skills", attempt{"symlink", filepath.Join(skills, "made"), filepath.Join(d.root, "staged")}, "EPERM"},
		{"a hard link in the skills", attempt{"link", filepath.Join(d.root, "new.go"), filepath.Join(skills, "s", "linked.md")}, "EPERM"},
		{"~/.opencode made as a link", attempt{"symlink", at(".opencode"), filepath.Join(d.root, "plugin")}, "EPERM"},
		{"Gemini CLI's folder in its sandbox, made as a link", attempt{"symlink", at(".cache", ".gemini"), filepath.Join(d.root, "plugin")}, "EPERM"},
		// None of them, nor a folder above them, moved aside to be written under another name.
		{"OpenCode's cache moved aside", attempt{"rename", cache, at(".cache", "old-opencode")}, "EPERM"},
		{"~/.cache moved aside", attempt{"rename", at(".cache"), at("old-cache")}, "EPERM"},
		{"~/.local/share moved aside", attempt{"rename", at(".local", "share"), at(".local", "old-share")}, "EPERM"},
		{"OpenCode's state moved aside", attempt{"rename", state, at(".local", "state", "old-opencode")}, "EPERM"},
		{"~/.agents moved aside", attempt{"rename", at(".agents"), at("old-agents")}, "EPERM"},
	})
	for file, text := range owners {
		if got, err := os.ReadFile(file); err != nil || string(got) != text {
			t.Errorf("%s changed: %q (%v)", file, got, err)
		}
	}
	for _, planted := range []string{filepath.Join(cache, "bin", "fzf"), filepath.Join(cache, "node_modules", "q"),
		filepath.Join(cache, "node_modules", "r"), filepath.Join(data, "auth.json"), filepath.Join(skills, "new"),
		filepath.Join(skills, "staged"), at(".cache", ".gemini"), filepath.Join(data, "snapshot"), at(".opencode"),
		filepath.Join(cache, "bin", "fd"), filepath.Join(cache, "bin", "linked"), filepath.Join(data, "made"), filepath.Join(data, "linked"),
		filepath.Join(state, "made"), filepath.Join(skills, "made"), filepath.Join(skills, "s", "linked.md")} {
		if _, err := os.Lstat(planted); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is there after the desk ran (%v)", planted, err)
		}
	}
}

// Every sign-in and token file each agent keeps in its folder (FXPLANTSTERMR3, from each one's source): a desk reads
// none, as it reads neither Codex's auth.json nor Gemini CLI's oauth_creds.json, nor takes one out under another name.
// The agents' settings beside them are still read.
func TestOnAMacAHireCantReadAnAgentsSignIn(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(append([]string{d.home}, parts...)...) }
	signIns := []string{
		at(".codex", "auth.json"), at(".codex", ".credentials.json"), at(".codex", ".env"), at(".codex", "secrets", "local.age"),
		at(".gemini", "oauth_creds.json"), at(".gemini", "mcp-oauth-tokens.json"), at(".gemini", "a2a-oauth-tokens.json"),
		at(".gemini", "gemini-credentials.json"), at(".gemini", ".env"), at(".cache", ".gemini", "oauth_creds.json"),
		at(".grok", "auth.json"), at(".grok", "auth.json.corrupt.1763400000000"), at(".grok", "mcp_credentials.json"),
		at(".grok", "user-settings.json"), at(".grok", "wallet.json"),
		at(".local", "share", "opencode", "auth.json"), at(".local", "share", "opencode", "mcp-auth.json"),
		at(".local", "share", "opencode", "opencode.db"), at(".local", "share", "opencode", "opencode.db-wal"),
	}
	for _, file := range signIns {
		mustWrite(t, file, escapeKey)
	}
	mustWrite(t, at(".grok", "config.toml"), "# the owner's")
	mustWrite(t, at(".local", "share", "opencode", "log", "opencode.log"), "the owner's")
	rows := []escapeRow{
		{"control: Grok's settings read", attempt{"read", at(".grok", "config.toml"), ""}, "ok"},
		{"control: OpenCode's log read", attempt{"read", at(".local", "share", "opencode", "log", "opencode.log"), ""}, "ok"},
	}
	for _, file := range signIns {
		rel, _ := filepath.Rel(d.home, file)
		rows = append(rows, escapeRow{"read ~/" + rel, attempt{"read", file, ""}, "EPERM"})
	}
	rows = append(rows, escapeRow{"Grok's sign-in, by a second name", attempt{"link", at(".grok", "auth.json"), filepath.Join(d.root, "grok-auth")}, "EPERM"})
	d.check(t, rows)
	if _, err := os.Lstat(filepath.Join(d.root, "grok-auth")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Grok's sign-in has a second name in the worktree (%v)", err)
	}
}

// An agent's folder the owner's environment moves (AgentPlaces, read by the core at each start: CODEX_HOME here, and
// Gemini CLI's trusted folders file): held as ~/.codex is, written by no desk, no link made there, its sign-ins
// unread, nor it or the folder above it moved aside. Its settings are still read, and its neighbours are the desk's.
func TestOnAMacAnAgentsFolderTheOwnersEnvironmentMovesIsHeldAsItsOwn(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	elsewhere := filepath.Join(d.base, "elsewhere")
	codex, trusted := filepath.Join(elsewhere, "codex-home"), filepath.Join(elsewhere, "gemini-trusted.json")
	for file, text := range map[string]string{
		filepath.Join(codex, "config.toml"): "# the owner's", filepath.Join(codex, "auth.json"): escapeKey,
		filepath.Join(codex, "secrets", "local.age"): escapeKey, filepath.Join(codex, "installation_id"): "0f8e6c1a",
		trusted: "{}", filepath.Join(d.root, "mine"): "a desk's", filepath.Join(d.root, "staged", "hooks.json"): "{}",
	} {
		mustWrite(t, file, text)
	}
	env := map[string]string{"CODEX_HOME": codex, "GEMINI_CLI_TRUSTED_FOLDERS_PATH": trusted}
	d.pl.AgentHomes, d.pl.AgentSignIns = AgentPlaces(func(v string) string { return env[v] })
	d.prepare(t)
	d.check(t, []escapeRow{
		{"control: its settings read", attempt{"read", filepath.Join(codex, "config.toml"), ""}, "ok"},
		{"control: a neighbour", attempt{"create", filepath.Join(elsewhere, "codex-home-notes"), ""}, "ok"},
		{"control: its worktree writes", attempt{"create", filepath.Join(d.root, "new.go"), ""}, "ok"},
		{"Gemini CLI's trusted folders", attempt{"write", trusted, ""}, "EPERM"},
		{"Gemini CLI's trusted folders, replaced", attempt{"rename", filepath.Join(d.root, "mine"), trusted}, "EPERM"},
		{"its settings", attempt{"write", filepath.Join(codex, "config.toml"), ""}, "EPERM"},
		{"its hooks", attempt{"create", filepath.Join(codex, "hooks.json"), ""}, "EPERM"},
		{"its sessions", attempt{"mkdir", filepath.Join(codex, "sessions"), ""}, "EPERM"},
		{"hooks brought in whole", attempt{"rename", filepath.Join(d.root, "staged"), filepath.Join(codex, "rules")}, "EPERM"},
		{"its sign-in read", attempt{"read", filepath.Join(codex, "auth.json"), ""}, "EPERM"},
		{"its secrets read", attempt{"read", filepath.Join(codex, "secrets", "local.age"), ""}, "EPERM"},
		{"a symlink there", attempt{"symlink", filepath.Join(codex, "made"), filepath.Join(d.home, ".zshrc")}, "EPERM"},
		{"a hard link there", attempt{"link", filepath.Join(d.root, "mine"), filepath.Join(codex, "linked")}, "EPERM"},
		{"installation_id removed", attempt{"remove", filepath.Join(codex, "installation_id"), ""}, "EPERM"},
		{"the folder moved aside", attempt{"rename", codex, filepath.Join(elsewhere, "old-codex")}, "EPERM"},
		{"the folder above it moved aside", attempt{"rename", elsewhere, filepath.Join(d.base, "old-elsewhere")}, "EPERM"},
	})
	for _, planted := range []string{filepath.Join(codex, "hooks.json"), filepath.Join(codex, "sessions"), filepath.Join(codex, "rules"),
		filepath.Join(codex, "made"), filepath.Join(codex, "linked")} {
		if _, err := os.Lstat(planted); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s is there after the desk ran (%v)", planted, err)
		}
	}
	if got, err := os.ReadFile(trusted); err != nil || string(got) != "{}" {
		t.Errorf("the trusted folders file changed: %q (%v)", got, err)
	}
}

// A desk's Claude Code beside the agents' folders (FXPLANTSTERMR3): what it writes as it works is its worktree, its
// own Claude folder, its caches and the home folder's other places, none of which the new rows reach; and it still
// reads the owner's instructions and skills. A stand-in does those writes under the real profile.
func TestOnAMacADesksClaudeCodeStillWorksBesideTheAgentsFolders(t *testing.T) {
	sandboxExecOrSkip(t)
	d := newEscapeDesk(t)
	at := func(parts ...string) string { return filepath.Join(append([]string{d.home}, parts...)...) }
	mustWrite(t, at(".agents", "skills", "s", "SKILL.md"), "the owner's skill")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	steps := []agentStep{
		{"read", at(".claude", "CLAUDE.md"), ""}, {"read", at(".claude", "skills", "s", "SKILL.md"), ""},
		{"read", at(".agents", "skills", "s", "SKILL.md"), ""},
		{"append", filepath.Join(d.root, "main.go"), ""}, {"replace", filepath.Join(d.root, "go.mod"), filepath.Join(d.root, "go.mod.tmp")},
		{"mkdirall", filepath.Join(d.root, "internal", "x"), ""},
		{"mkdirall", filepath.Join(d.own, "claude", "projects", "p"), ""}, {"append", filepath.Join(d.own, "claude", "projects", "p", "s.jsonl"), ""},
		{"mkdirall", at("Library", "Caches", "claude-cli-nodejs", "p"), ""},
		{"append", at("Library", "Caches", "claude-cli-nodejs", "p", "mcp.log"), ""},
		{"mkdirall", at(".cache", "go-build", "ab"), ""}, {"mkdirall", at(".local", "state", "other"), ""},
		{"mkdirall", at(".local", "share", "other"), ""}, {"mkdirall", at(".npm", "_cacache"), ""},
	}
	data, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	r := d.inside(t, nil, self, standInAgent, string(data))
	var said []string
	if json.Unmarshal([]byte(r.stdout), &said) != nil || len(said) != len(steps) {
		t.Fatalf("the stand-in didn't run: %+v", r)
	}
	for i, got := range said {
		if got != "ok" {
			t.Errorf("Claude Code's work in a desk: %s %s gave %s", steps[i].Op, steps[i].At, got)
		}
	}
}
