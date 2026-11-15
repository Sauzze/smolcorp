//go:build !windows

package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// SA-X8: a Claude folder per desk, on stand-in accounts in the test's own home. The keys and tokens here are fake.

// deskAccount makes a stand-in account folder at dir ("" for the default account, ~/.claude, whose .claude.json is
// in the home folder) with what an owner's has: notes, skills, plugins, user settings, and a .claude.json holding a
// sign-in's profile, a stand-in API key, the owner's own projects and two MCP servers, one with a stand-in key.
// It returns the folder and the settings' text.
func deskAccount(t *testing.T, dir string) (string, string) {
	t.Helper()
	file := filepath.Join(homeDir, ".claude.json")
	if dir == "" {
		dir = filepath.Join(homeDir, ".claude")
	} else {
		file = filepath.Join(dir, ".claude.json")
	}
	settings := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"true"}]}]},"autoMemoryEnabled":false}`
	for path, text := range map[string]string{
		filepath.Join(dir, "CLAUDE.md"): "the owner's notes", filepath.Join(dir, "skills", "s", "SKILL.md"): "a skill",
		filepath.Join(dir, "plugins", "installed_plugins.json"): "{}", filepath.Join(dir, "settings.json"): settings,
		file: `{"numStartups": 40, "userID": "u-1", "primaryApiKey": "sk-fake-x8-primary", "hasCompletedOnboarding": true,
  "lastOnboardingVersion": "2.1.325", "oauthAccount": {"accountUuid": "acct-x8", "emailAddress": "owner@example.com"},
  "projects": {"/owner/own/project": {"hasTrustDialogAccepted": true, "mcpServers": {"local": {"command": "x"}}}},
  "customApiKeyResponses": {"approved": ["owner-own-tail"]},
  "mcpServers": {"ticked": {"command": "ticked-mcp", "env": {"TICKED_KEY": "fake-ticked-key"}},
    "other": {"command": "other-mcp", "env": {"OTHER_KEY": "fake-other-key"}}}}`,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, settings
}

func readDeskJSON(t *testing.T, folder string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(folder, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	var seeded map[string]json.RawMessage
	if err := json.Unmarshal(data, &seeded); err != nil {
		t.Fatalf("the desk's .claude.json isn't JSON: %v", err)
	}
	return seeded
}

// macDeskLinks makes a desk's folder link to its account as a Mac's does, on any system (Linux copies: deskClaudeCopies).
func macDeskLinks(t *testing.T) {
	t.Helper()
	saved := deskClaudeCopies
	deskClaudeCopies = false
	t.Cleanup(func() { deskClaudeCopies = saved })
}

func TestADeskGetsItsOwnClaudeFolderLinkedToItsAccount(t *testing.T) {
	isolate(t)
	macDeskLinks(t)
	for _, named := range []bool{false, true} {
		dir := ""
		if named {
			dir = filepath.Join(homeDir, "accounts", "work")
		}
		account, settings := deskAccount(t, dir)
		id := map[bool]string{false: "aaaa00000001", true: "aaaa00000002"}[named]
		folder := deskClaudeFolder(supportDir, id)
		if folder != filepath.Join(supportDir, "hires", id, "claude") {
			t.Fatalf("the desk's folder is %s", folder)
		}
		if err := makeDeskClaude(folder, deskClaude{Account: dir, Trusted: "/work/tree", MCP: []string{"ticked", "absent"}}); err != nil {
			t.Fatalf("named %v: %v", named, err)
		}
		if info, err := os.Lstat(folder); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Fatalf("named %v: the folder %v %v", named, info, err)
		}
		// Links to what the account has, and none to what it hasn't (commands, agents, output-styles).
		names, _ := os.ReadDir(folder)
		var got []string
		for _, entry := range names {
			got = append(got, entry.Name())
		}
		if want := []string{".claude.json", "CLAUDE.md", "plugins", "settings.json", "skills"}; !slices.Equal(got, want) {
			t.Errorf("named %v: the folder holds %q, want %q", named, got, want)
		}
		for _, name := range []string{"CLAUDE.md", "plugins", "skills"} {
			if to, err := os.Readlink(filepath.Join(folder, name)); err != nil || to != filepath.Join(account, name) {
				t.Errorf("named %v: %s links to %q (%v), want the account's", named, name, to, err)
			}
		}
		if data, err := os.ReadFile(filepath.Join(folder, "skills", "s", "SKILL.md")); err != nil || string(data) != "a skill" {
			t.Errorf("named %v: the account's skill isn't read through the link: %q %v", named, data, err)
		}
		// The user settings: a copy, byte for byte, read-only; a real file, not a link the desk could follow.
		info, err := os.Lstat(filepath.Join(folder, "settings.json"))
		data, _ := os.ReadFile(filepath.Join(folder, "settings.json"))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o400 || string(data) != settings {
			t.Errorf("named %v: settings.json %v %v, %q", named, info.Mode(), err, data)
		}
		// Its .claude.json: only the allowlist. The sign-in's profile and the first-run flags, its folder trusted,
		// the ticked server alone; never the account's API key, its projects or its approvals.
		seeded := readDeskJSON(t, folder)
		keys := slices.Sorted(maps.Keys(seeded))
		if want := []string{"hasCompletedOnboarding", "lastOnboardingVersion", "mcpServers", "oauthAccount", "projects"}; !slices.Equal(keys, want) {
			t.Errorf("named %v: the desk's .claude.json holds %q, want %q", named, keys, want)
		}
		if string(seeded["hasCompletedOnboarding"]) != "true" || string(seeded["lastOnboardingVersion"]) != `"2.1.325"` ||
			!strings.Contains(string(seeded["oauthAccount"]), `"acct-x8"`) {
			t.Errorf("named %v: the first-run flags or the profile: %s", named, readFile(t, filepath.Join(folder, ".claude.json")))
		}
		var servers map[string]any
		if json.Unmarshal(seeded["mcpServers"], &servers) != nil || len(servers) != 1 || servers["ticked"] == nil {
			t.Errorf("named %v: the MCP servers %s, want the ticked one alone", named, seeded["mcpServers"])
		}
		whole, _ := os.ReadFile(filepath.Join(folder, ".claude.json"))
		if !isTrusted("/work/tree", whole) || strings.Contains(string(whole), "/owner/own/project") ||
			bytes.Contains(whole, []byte("sk-fake-x8-primary")) || bytes.Contains(whole, []byte("fake-other-key")) ||
			bytes.Contains(whole, []byte("owner-own-tail")) {
			t.Errorf("named %v: the desk's .claude.json:\n%s", named, whole)
		}
		if info, _ := os.Stat(filepath.Join(folder, ".claude.json")); info.Mode().Perm() != 0o600 {
			t.Errorf("named %v: .claude.json is %v", named, info.Mode())
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, _ := os.ReadFile(path)
	return string(data)
}

// An API-key hire's key is approved in its own file; no MCP server is copied unless ticked; an account without
// settings gives the desk none.
func TestADesksClaudeFolderApprovesItsKeyAndCopiesOnlyTickedServers(t *testing.T) {
	isolate(t)
	account, _ := deskAccount(t, "")
	if err := os.Remove(filepath.Join(account, "settings.json")); err != nil {
		t.Fatal(err)
	}
	folder := deskClaudeFolder(supportDir, "bbbb00000001")
	if err := makeDeskClaude(folder, deskClaude{Trusted: "/w", KeyTail: keyApproval(placeTestKey)}); err != nil {
		t.Fatal(err)
	}
	whole, _ := os.ReadFile(filepath.Join(folder, ".claude.json"))
	if !isApproved(keyTail(placeTestKey), whole) || !isTrusted("/w", whole) || strings.Contains(string(whole), "mcpServers") {
		t.Errorf("the desk's .claude.json:\n%s", whole)
	}
	if _, err := os.Lstat(filepath.Join(folder, "settings.json")); err == nil {
		t.Error("a settings.json was made for an account that has none")
	}
	// A subscription hire's file approves no key.
	other := deskClaudeFolder(supportDir, "bbbb00000002")
	if err := makeDeskClaude(other, deskClaude{Trusted: "/w"}); err != nil {
		t.Fatal(err)
	}
	if _, found := readDeskJSON(t, other)["customApiKeyResponses"]; found {
		t.Error("a subscription hire's file approves a key")
	}
	// An API-key hire with no key is refused, with nothing made.
	empty := ""
	if err := makeDeskClaude(deskClaudeFolder(supportDir, "bbbb00000003"), deskClaude{Trusted: "/w", KeyTail: &empty}); err != errHireNoKey {
		t.Errorf("no key: %v, want errHireNoKey", err)
	}
	if _, err := os.Lstat(deskClaudeFolder(supportDir, "bbbb00000003")); err == nil {
		t.Error("a refused desk's folder was left")
	}
}

// Nothing already at the desk's folder is written through, and an account whose file can't be read, or isn't
// Claude Code's shape, gives no folder at all.
func TestADesksClaudeFolderIsNewAndRefusedWhole(t *testing.T) {
	isolate(t)
	deskAccount(t, "")
	// A link planted where the folder goes: refused, and nothing lands where it leads.
	elsewhere := t.TempDir()
	folder := deskClaudeFolder(supportDir, "cccc00000001")
	if err := os.MkdirAll(filepath.Dir(folder), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, folder); err != nil {
		t.Fatal(err)
	}
	if err := makeDeskClaude(folder, deskClaude{Trusted: "/w"}); !errors.Is(err, errDeskClaude) {
		t.Errorf("a planted link: %v, want errDeskClaude", err)
	}
	if names, _ := os.ReadDir(elsewhere); len(names) != 0 {
		t.Errorf("written through a planted link: %v", names)
	}
	// The account's .claude.json missing, or not an object, or its servers not an object: nothing made.
	for name, text := range map[string]string{"missing": "", "a list": "[]", "servers a list": `{"mcpServers": []}`} {
		file := filepath.Join(homeDir, ".claude.json")
		if text == "" {
			os.Remove(file)
		} else if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		at := deskClaudeFolder(supportDir, "cccc00000002")
		err := makeDeskClaude(at, deskClaude{Trusted: "/w", MCP: []string{"x"}})
		if want := map[bool]error{true: errHireNoConfig, false: errHireTrust}[text == ""]; err != want {
			t.Errorf("%s: %v, want %v", name, err, want)
		}
		if _, err := os.Lstat(at); err == nil {
			t.Errorf("%s: a folder was made", name)
		}
	}
}

// The folder goes whole, whatever the desk wrote in it, and nothing a link in it leads to goes with it: not the
// account's notes and skills, nor a folder the desk linked its whole Claude folder's place to.
func TestRemovingADesksClaudeFolderNeverFollowsItsLinks(t *testing.T) {
	isolate(t)
	account, _ := deskAccount(t, "")
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "keep.txt"), []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	folder := deskClaudeFolder(supportDir, "dddd00000001")
	if err := makeDeskClaude(folder, deskClaude{Trusted: "/w"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(folder, "projects", "p"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(folder, "projects", "p", "out")); err != nil {
		t.Fatal(err)
	}
	if err := removeDeskClaude(supportDir, "dddd00000001"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(folder); err == nil {
		t.Error("the desk's folder is still there")
	}
	for _, kept := range []string{filepath.Join(account, "CLAUDE.md"), filepath.Join(account, "skills", "s", "SKILL.md"),
		filepath.Join(elsewhere, "keep.txt"), filepath.Join(supportDir, "hires", "dddd00000001")} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s went with the desk's folder: %v", kept, err)
		}
	}
	// The desk's folder's place made a link by the desk: the link goes, not what it leads to.
	if err := os.Symlink(elsewhere, folder); err != nil {
		t.Fatal(err)
	}
	if err := removeDeskClaude(supportDir, "dddd00000001"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "keep.txt")); err != nil {
		t.Errorf("a link at the folder's place was followed: %v", err)
	}
	// An ID that isn't one plain name removes nothing.
	for _, id := range []string{"", ".", "..", "../dddd00000001", "/abs"} {
		if err := removeDeskClaude(supportDir, id); !errors.Is(err, errDeskClaude) {
			t.Errorf("id %q: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(supportDir, "hires")); err != nil {
		t.Errorf("the hires folder went: %v", err)
	}
}

// The plan: the desk's folder as CLAUDE_CONFIG_DIR, and its account's sign-in through CLAUDE_SECURESTORAGE_CONFIG_DIR,
// set to "" (not unset) for ~/.claude's, in place of the account line; never without an account named.
func TestADesksPlanUsesItsOwnFolderAndItsAccountsSignIn(t *testing.T) {
	isolate(t)
	settingsOf := func(p deskPlan) string {
		for _, word := range p.Words {
			if rest, ok := strings.CutPrefix(word, "--settings="); ok {
				return rest
			}
		}
		return ""
	}
	for _, c := range []struct {
		name      string
		start     hireStart
		secure    string
		forbidden string
	}{
		{"the default account", hireStart{DefaultAccount: true}, "", ""},
		{"a named account", hireStart{ConfigDir: "/a/work"}, "/a/work", "/a/work"},
	} {
		start := c.start
		start.Role, start.Message, start.DeskClaude, start.Settings = testHireRole(), "go", "/s/hires/x/claude", `{"autoMemoryEnabled":false}`
		start.SendKey, start.HookKey = "fake-send-x8", "fake-hook-x8"
		plan, err := start.plan()
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var config, secure []envChange
		for _, change := range plan.Changes {
			switch change.Name {
			case "CLAUDE_CONFIG_DIR":
				config = append(config, change)
			case secureStorageEnvironment:
				secure = append(secure, change)
			}
		}
		if len(config) != 1 || config[0] != (envChange{Name: "CLAUDE_CONFIG_DIR", Value: "/s/hires/x/claude"}) ||
			len(secure) != 1 || secure[0] != (envChange{Name: secureStorageEnvironment, Value: c.secure}) {
			t.Errorf("%s: CLAUDE_CONFIG_DIR %+v, %s %+v", c.name, config, secureStorageEnvironment, secure)
		}
		// The environment the pane gets: the terminal's own values replaced, the desk's folder in force.
		env, err := deskEnvironment([]string{"CLAUDE_CONFIG_DIR=/terminal", secureStorageEnvironment + "=/terminal", "PATH=/bin"}, plan, nil)
		if err != nil || !slices.Contains(env, "CLAUDE_CONFIG_DIR=/s/hires/x/claude") || !slices.Contains(env, secureStorageEnvironment+"="+c.secure) ||
			slices.Contains(env, "CLAUDE_CONFIG_DIR=/terminal") || slices.Contains(env, secureStorageEnvironment+"=/terminal") {
			t.Errorf("%s: the pane's environment %q (%v)", c.name, env, err)
		}
		// The inline settings stay (MEM4b keeps the GM's memory off there): they outrank the folder's user settings.
		if settingsOf(plan) != start.Settings {
			t.Errorf("%s: the inline settings went: %q", c.name, plan.Words)
		}
	}
	// No account named: refused, never the terminal's folder's sign-in.
	if _, err := (hireStart{Role: testHireRole(), Message: "go", DeskClaude: "/s/hires/x/claude"}).plan(); err == nil {
		t.Error("a desk's folder with no account named was planned")
	}
	// Another agent gets no Claude folder, whatever it was given.
	codex, err := hireStart{Role: Role{Name: "Co", Agent: "codex"}, Message: "x", DefaultAccount: true, DeskClaude: "/s/c"}.plan()
	if err != nil || slices.ContainsFunc(codex.Changes, func(c envChange) bool {
		return c.Name == "CLAUDE_CONFIG_DIR" || c.Name == secureStorageEnvironment
	}) {
		t.Errorf("codex's plan: %+v %v", codex.Changes, err)
	}
}

// The engine makes the desk's folder for its start, on the account named, trusted at its worktree; a start that
// never reports and a fire take it away; a fire that's refused keeps it.
func TestAnEngineDesksClaudeFolderIsMadeAtItsStartAndGoesAtItsFire(t *testing.T) {
	d, r, hire := hiredForFire(t, "")
	folder := deskClaudeFolder(supportDir, hire.ID)
	whole, err := os.ReadFile(filepath.Join(folder, ".claude.json"))
	if err != nil || !isTrusted(hire.Worktree, whole) {
		t.Fatalf("the desk's .claude.json (%v) doesn't trust %s:\n%s", err, hire.Worktree, whole)
	}
	script := r.script(hire)
	for _, want := range []string{"export CLAUDE_CONFIG_DIR=" + shellQuoted(folder) + "\n", "export " + secureStorageEnvironment + "=''\n"} {
		if !strings.Contains(script, want) {
			t.Errorf("the start lacks %q:\n%s", want, script)
		}
	}
	// Refused at a question: nothing changes, the folder stays.
	windowShows(r, pane(t, "permission-menu"), true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err == nil {
		t.Fatal("a fire at a question went ahead")
	}
	if _, err := os.Stat(filepath.Join(folder, ".claude.json")); err != nil {
		t.Fatalf("a refused fire took the desk's folder: %v", err)
	}
	windowShows(r, pane(t, "idle-empty"), true)
	exitingPane(r, "0", true)
	if _, err := fireEngineHire(hire, 0, r.o, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(folder); err == nil {
		t.Error("the fired desk's Claude folder is still there")
	}
	if _, err := os.Stat(hireFolder(supportDir, hire.ID)); err != nil {
		t.Errorf("the hire's own folder (its recap) went too: %v", err)
	}
	_ = d

	// A start that never reports.
	r2 := newEngineRig(t)
	late, err := runManagedHire(r2.request("janitor"), r2.o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(deskClaudeFolder(supportDir, late.ID)); err != nil {
		t.Fatalf("no folder made: %v", err)
	}
	now := time.Unix(1000, 0)
	r2.o.Now = func() time.Time { return now }
	r2.o.Pause = func(d time.Duration) { now = now.Add(d) }
	watchManagedHire(late, 0, r2.o)
	if _, err := os.Lstat(deskClaudeFolder(supportDir, late.ID)); err == nil {
		t.Error("a start that never reported left its Claude folder")
	}
}

// An engine hire on a named account: its folder's sign-in is that account's, and a desk whose folder can't be made
// isn't started, with nothing left.
func TestAnEngineDeskOnANamedAccountAndOneWhoseFolderCantBeMade(t *testing.T) {
	r := newEngineRig(t)
	macDeskLinks(t)
	work, _ := deskAccount(t, filepath.Join(r.home, "accounts", "work"))
	request := r.request("janitor")
	request.ConfigDir, request.DefaultAccount, request.MCPServers = work, false, []string{"ticked"}
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	folder := deskClaudeFolder(supportDir, hire.ID)
	if script := r.script(hire); !strings.Contains(script, "export "+secureStorageEnvironment+"="+shellQuoted(work)+"\n") ||
		!strings.Contains(script, "export CLAUDE_CONFIG_DIR="+shellQuoted(folder)+"\n") || strings.Contains(script, "CLAUDE_CONFIG_DIR="+shellQuoted(work)) {
		t.Errorf("the start on the work account:\n%s", script)
	}
	if to, err := os.Readlink(filepath.Join(folder, "CLAUDE.md")); err != nil || to != filepath.Join(work, "CLAUDE.md") {
		t.Errorf("CLAUDE.md links to %q (%v)", to, err)
	}
	if servers := readDeskJSON(t, folder)["mcpServers"]; !strings.Contains(string(servers), "ticked-mcp") || strings.Contains(string(servers), "other-mcp") {
		t.Errorf("the ticked servers: %s", servers)
	}

	// A desk whose folder can't be made (the account's settings can't be read): refused, nothing left.
	r.tmux.asked = nil
	if err := os.Chmod(filepath.Join(work, "settings.json"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := runManagedHire(request, r.o); err == nil || !strings.Contains(err.Error(), "own Claude folder") {
		t.Fatalf("err = %v", err)
	}
	if r.tmux.ran("new-session") || len(registered(t)) != 1 {
		t.Errorf("a refused desk started or stayed: tmux %q, registry %+v", r.tmux.asked, registered(t))
	}
	if left, _ := os.ReadDir(filepath.Join(supportDir, "hires")); len(left) != 1 || left[0].Name() != hire.ID {
		t.Errorf("the hires folder holds %v, want the first desk's alone", left)
	}
}

// RT126SAX8 F1: every desk's Claude folder is behind every desk's fence, by pattern, its own included, so a desk
// started later is covered too and each desk reaches its conversation data through Claude Code alone. What still
// works: its plans, pinned in its place, open to its Write and Read and never in its commits; and its sessions
// (--resume), which Claude Code keeps in the folder its start names, denied to its tools only.
func TestAnEngineDesksClaudeFolderIsBehindEveryFenceButItsPlansAndSessionsWork(t *testing.T) {
	r := newEngineRig(t)
	first, err := runManagedHire(r.request("janitor"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	// Started after the first desk's settings were written.
	later, err := runManagedHire(r.request("tester"), r.o)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
		Plans string `json:"plansDirectory"`
	}
	if err := json.Unmarshal([]byte(startedSettings(t, first.ID)), &settings); err != nil {
		t.Fatal(err)
	}
	deny := settings.Permissions.Deny
	var edits []string
	for _, rule := range deny {
		if pattern, ok := strings.CutPrefix(rule, "Edit("); ok {
			edits = append(edits, "Read("+pattern)
		}
	}
	// The reviewer's probe (RT126SAX8: open to the first desk before this fix), the pictures pasted in a pane, the
	// default plans folder, and the GM's when hired after the first desk: denied to Read and Edit alike.
	stores := []string{"history.jsonl", "projects/-work-api/s1.jsonl", ".claude.json", "paste-cache/5f3a.txt",
		"todos/s1-agent-s1.json", "image-cache/s1/1.png", "plans/calm-river.md", "settings.json"}
	for _, desk := range []string{later.ID, first.ID, "gm0000000001"} {
		for _, store := range stores {
			at := filepath.Join(deskClaudeFolder(supportDir, desk), filepath.FromSlash(store))
			if !readDenied(deny, r.home, at) || !readDenied(edits, r.home, at) {
				t.Errorf("%s isn't denied to the first desk's Read and Edit", at)
			}
		}
	}

	// Plans: pinned in the desk's place, made ignored by git, open to its tools.
	if settings.Plans != deskPlansFor(first.ID) {
		t.Errorf("plansDirectory %q, want %q", settings.Plans, deskPlansFor(first.ID))
	}
	plan := filepath.Join(first.Worktree, filepath.FromSlash(deskPlansFor(first.ID)), "calm-river.md")
	if readDenied(deny, r.home, plan) || readDenied(edits, r.home, plan) {
		t.Errorf("the desk's own plan %s is denied", plan)
	}
	if err := os.WriteFile(plan, []byte("# Plan\n"), 0o600); err != nil {
		t.Fatalf("the plans folder wasn't made in the desk's place: %v", err)
	}
	// What a fire asks before it removes the worktree (removeWorkspace): a plan keeps it clean.
	if status, out, ok := runGit(first.Worktree, "status", "--porcelain"); !ok || status != 0 || out != "" {
		t.Errorf("a plan shows in the desk's worktree: %d %q", status, out)
	}

	// Sessions: Claude Code finds them in CLAUDE_CONFIG_DIR, the desk's folder, which the fence denies to tools only.
	if script := r.script(first); !strings.Contains(script, "export CLAUDE_CONFIG_DIR="+shellQuoted(deskClaudeFolder(supportDir, first.ID))+"\n") {
		t.Errorf("the start doesn't name the desk's folder:\n%s", script)
	}
	for _, rule := range deny {
		if tool, _, ok := strings.Cut(rule, "("); !ok || !slices.Contains([]string{"Read", "Edit", "Bash"}, tool) {
			t.Errorf("%q isn't a rule for a tool", rule)
		}
	}

	// A desk whose Claude folder isn't its own (the launcher's, T3's) keeps Claude Code's default plans folder.
	other, err := hireSettingsObject(hireSettingsOptions{HireID: "l1", CLI: r.o.CLI, Support: supportDir, Run: r.o.Guard, IsExec: r.o.IsExec})
	if err != nil || other["plansDirectory"] != nil {
		t.Errorf("another desk's plansDirectory %v (%v)", other["plansDirectory"], err)
	}
}

// makeDeskPlans makes its folder ignored by git, keeps a .gitignore already there, and never follows a link at its
// place.
func TestADesksPlansFolderIsIgnoredAndNeverALink(t *testing.T) {
	place := t.TempDir()
	if err := makeDeskPlans(place, "h1"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(place, ".smolcorp", ".gitignore")); err != nil || string(data) != "*\n" {
		t.Errorf(".gitignore %q (%v)", data, err)
	}
	if info, err := os.Lstat(filepath.Join(place, ".smolcorp", "plans", "h1")); err != nil || !info.IsDir() {
		t.Errorf("plans: %v", err)
	}
	// Made again: the owner's own .gitignore stays as it is.
	if err := os.WriteFile(filepath.Join(place, ".smolcorp", ".gitignore"), []byte("plans/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := makeDeskPlans(place, "h1"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(place, ".smolcorp", ".gitignore")); string(data) != "plans/\n" {
		t.Errorf(".gitignore rewritten: %q", data)
	}
	// A link at any of its names: refused, nothing made where it leads.
	for _, at := range []string{".smolcorp", ".smolcorp/plans", ".smolcorp/plans/h1"} {
		place, away := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(place, filepath.Dir(filepath.FromSlash(at))), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(away, filepath.Join(place, filepath.FromSlash(at))); err != nil {
			t.Fatal(err)
		}
		if err := makeDeskPlans(place, "h1"); err == nil {
			t.Errorf("a link at %s was taken", at)
		}
		if entries, _ := os.ReadDir(away); len(entries) != 0 {
			t.Errorf("a link at %s was followed: %v", at, entries)
		}
	}
}

// RT126SAX8 N1: a desk hired into a repo's subfolder works in its worktree's subfolder, but its .claude.json trusts the
// worktree, as prepareHire trusted it in the account's file; its plans are where it starts, which Claude Code's
// plansDirectory is relative to.
func TestADeskInARepoSubfolderTrustsItsWorktree(t *testing.T) {
	r := newEngineRig(t)
	request := r.request("janitor")
	request.Folder = filepath.Join(r.repo, "sub")
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	whole, err := os.ReadFile(filepath.Join(deskClaudeFolder(supportDir, hire.ID), ".claude.json"))
	if err != nil || !isTrusted(physical(hire.Worktree), whole) || isTrusted(physical(filepath.Join(hire.Worktree, "sub")), whole) {
		t.Errorf("the desk's .claude.json (%v) doesn't trust its worktree %s alone:\n%s", err, hire.Worktree, whole)
	}
	if info, err := os.Lstat(filepath.Join(hire.Worktree, "sub", filepath.FromSlash(deskPlansFor(hire.ID)))); err != nil || !info.IsDir() {
		t.Errorf("no plans folder where the desk starts: %v", err)
	}
}
