//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FXA2 (RT126A2 B1): hiring an ACP agent is A5's, so a role of the kind "acp:<id>" is refused on every path that
// hires, before its name is looked for anywhere as a program.

const (
	acpOwnerRefusal = "Gemini CLI can't be hired yet: ACP hiring comes later in 1.26."
	acpGMRefusal    = "Gemini CLI runs its tools itself, so Smolcorp can't fence it: the General Manager doesn't hire it."
)

// acpRoleDeck is the engine's deck with a role on an ACP agent, and a GM token.
func acpRoleDeck(t *testing.T, agent string) (*Deck, *engineRig, *hiring, string) {
	t.Helper()
	d, r, h, gm := gmHireDeck(t)
	d.mu.Lock()
	d.config.Roles = append(d.config.Roles, Role{ID: "navigator", Name: "Navigator", Agent: agent, Instructions: "Find the way."})
	d.mu.Unlock()
	return d, r, h, gm
}

func TestAnACPRoleIsRefusedOnEveryHiringPath(t *testing.T) {
	d, r, h, gm := acpRoleDeck(t, "acp:gemini")
	body := `{"role":"navigator","folder":"` + r.repo + `","answers":[]}`
	asPage := call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: body,
		headers: map[string]string{setupHeader: "1"}}
	asGM := call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: body,
		withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}}
	withT3 := func(c call) call {
		c.body = `{"role":"navigator","folder":"` + r.repo + `","answers":[],"engine":"t3"}`
		return c
	}
	withKey := func(c call) call {
		c.body = `{"role":"navigator","folder":"` + r.repo + `","answers":[],"apiKey":true}`
		return c
	}
	for _, c := range []struct {
		name, want string
		call       call
	}{
		{"POST /setup/hire as the page", acpOwnerRefusal, asPage},
		{"POST /setup/hire as the GM", acpGMRefusal, asGM},
		// Into T3 Code too (FXA2b, RT126A2b B1): the same sentences, not T3 Code's "Claude Code only".
		{"POST /setup/hire into T3 Code as the page", acpOwnerRefusal, withT3(asPage)},
		{"POST /setup/hire into T3 Code as the GM", acpGMRefusal, withT3(asGM)},
		// The API-key field doesn't answer first (RT126A2b N1).
		{"POST /setup/hire with the API key as the page", acpOwnerRefusal, withKey(asPage)},
		{"POST /setup/hire with the API key as the GM", acpGMRefusal, withKey(asGM)},
	} {
		w := serve(t, d, c.call)
		d.watching.Wait()
		if w.Code != http.StatusConflict || strings.TrimSpace(w.Body.String()) != c.want {
			t.Errorf("%s: %d %q, want 409 %q", c.name, w.Code, w.Body, c.want)
		}
	}
	// The GM's own route.
	if status, why := postGMHire(t, d, asGMCaller, gm, `{"role":"navigator"}`); status != http.StatusConflict || why != acpGMRefusal {
		t.Errorf("POST /setup/gm/hire: %d %q, want 409 %q", status, why, acpGMRefusal)
	}
	// And its T3 Code lane (RT126A2b B1), which used to answer in T3 Code's words.
	if status, why := postGMHire(t, d, asGMCaller, gm, `{"role":"navigator","engine":"t3","folder":"`+r.repo+`"}`); status != http.StatusConflict || why != acpGMRefusal {
		t.Errorf("POST /setup/gm/hire into T3 Code: %d %q, want 409 %q", status, why, acpGMRefusal)
	}
	// The office's hire (a person at the page's desk), and Start here's: through hireWith, as the owner.
	if status, why := d.Perform("hire", "", url.Values{"role": {"navigator"}}); status != http.StatusConflict || why != acpOwnerRefusal {
		t.Errorf("the office's hire: %d %q, want 409 %q", status, why, acpOwnerRefusal)
	}
	if status, why := d.hireAs("", "navigator", 0, true); status != http.StatusConflict || why != acpOwnerRefusal {
		t.Errorf("setup's own hire: %d %q, want 409 %q", status, why, acpOwnerRefusal)
	}
	// None got as far as looking for the agent where hires run, a window, tmux, a row or a worktree.
	if len(h.lookedFor) != 0 {
		t.Errorf("a hire looked for %q on the computer", h.lookedFor)
	}
	nothingMade(t, r, h)
	// And the cooldown wasn't spent: a refused hire leaves the next one free to be made.
	d.mu.Lock()
	hired := d.hiring || !d.lastHire.IsZero()
	d.mu.Unlock()
	if hired {
		t.Error("a refused ACP hire took the cooldown")
	}
}

func TestAnACPRoleOfAnUnknownAgentIsRefusedToo(t *testing.T) {
	d, r, h, gm := acpRoleDeck(t, "acp:not-on-the-list")
	// As the fence says of any agent it doesn't know; the owner's line has no name to give.
	if status, why := postGMHire(t, d, asGMCaller, gm, `{"role":"navigator"}`); status != http.StatusConflict ||
		why != "The General Manager hires only Claude roles: Smolcorp can't fence another agent." {
		t.Errorf("the GM: %d %q", status, why)
	}
	if status, why := d.hireAs("", "navigator", 0, true); status != http.StatusConflict || why != acpNotHiredYet {
		t.Errorf("the owner: %d %q, want 409 %q", status, why, acpNotHiredYet)
	}
	if len(h.lookedFor) != 0 {
		t.Errorf("a hire looked for %q on the computer", h.lookedFor)
	}
	nothingMade(t, r, h)
}

func TestEveryCheckReadsAnACPRoleAsNotClaude(t *testing.T) {
	for _, program := range []string{"acp:gemini", "acp:opencode", "acp:not-on-the-list", "acp:"} {
		if got := agentOrClaude(program); got == Claude || got != Agent(program) || !got.IsACP() {
			t.Errorf("agentOrClaude(%q) is %q, want it as it is, and never Claude Code", program, got)
		}
	}
	// What isn't known is still Claude Code, as before; and the agents that are known are themselves.
	for program, want := range map[string]Agent{"": Claude, "nonsense": Claude, "acp": Claude, "claude": Claude, "codex": Codex, "gemini": Gemini} {
		if got := agentOrClaude(program); got != want {
			t.Errorf("agentOrClaude(%q) is %q, want %q", program, got, want)
		}
	}
	// T3 Code takes a Claude Code role only, whichever way the role is asked for.
	for _, agent := range []string{"acp:gemini", "acp:not-on-the-list"} {
		role := &Role{ID: "navigator", Agent: agent}
		if why := t3HireRefusal(role, "", false, false); why == "" || !strings.Contains(why, "Only a Claude Code role") {
			t.Errorf("t3HireRefusal for %s: %q", agent, why)
		}
	}
	if why := t3HireRefusal(&Role{ID: "janitor", Agent: "claude"}, "", false, false); why != "" {
		t.Errorf("t3HireRefusal for a Claude Code role: %q", why)
	}
	// A test copy's own refusal says no to it, as it does to any agent that isn't Claude Code.
	if err := testCopyRefusal(7799, "acp:gemini", "/account"); err == nil || !strings.Contains(err.Error(), "only Claude Code hires here") {
		t.Errorf("a test copy's hire of an ACP agent: %v", err)
	}
	// And its start script is never made: it would exec "acp:gemini" as a program.
	if script, err := (hireStart{Role: Role{ID: "navigator", Agent: "acp:gemini"}}).startText(); err == nil || script != "" {
		t.Errorf("a start script for an ACP role: %q, %v", script, err)
	}
}

// The launcher never starts an "acp:<id>" role, not even where a file of that name is on PATH.
func TestTheLauncherNeverRunsAnACPRolesName(t *testing.T) {
	isolate(t)
	bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{"acp:gemini", "gemini", "acp:not-on-the-list"} {
		writeFile(t, filepath.Join(bin, name), "#!/bin/sh\necho ran > "+marker+"\n")
		if err := os.Chmod(filepath.Join(bin, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeFile(t, configFile, `{"roles": [{"id": "navigator", "name": "Navigator", "agent": "acp:gemini", "instructions": "Find the way."}]}`)
	// The role is kept as saved (a known ACP agent is no longer rewritten to Claude Code).
	if loadConfig().Roles[0].Agent != "acp:gemini" {
		t.Fatalf("the saved role is %+v", loadConfig().Roles[0])
	}
	args, err := roleArguments("navigator", "/work", startedHere, "")
	if err == nil || args != nil || err.Error() != acpNotHiredYet {
		t.Errorf("roleArguments gave %q, %v, want no arguments and %q", args, err, acpNotHiredYet)
	}
	// The whole launcher, as `smolcorp run --role navigator` and a hire's window run it, and with the agent a hire
	// would have checked: exit 2 before anything is looked up on PATH.
	for _, run := range []*launcherRun{{role: "navigator"}, {role: "navigator", hire: strings.Repeat("AB", 16), agent: "acp:gemini"}} {
		if code := launch(run); code == 0 {
			t.Errorf("the launcher ran %+v and exited 0", run)
		}
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a program named acp:gemini (or gemini) on PATH was run")
	}
}
