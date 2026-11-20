package desktop

import (
	"fmt"
	"testing"
)

// FXPLANTSTERMR2 (B) and R3: on a Mac, while the sandbox is in force, Codex, Gemini CLI, OpenCode and Grok hires are
// refused in words (notonamac.go). These rows are the words and what Setup and the office show; notonamac_unix_test.go has the routes,
// launchersandbox_darwin_test.go the launcher's backstop.

// hiresOfEveryAgentHere lets a row about something else hire Codex, Gemini CLI, OpenCode or Grok as it did before: on a Mac,
// with the owner's sandbox switch off (the owner's Q8), where they start without one; elsewhere nothing changes.
func hiresOfEveryAgentHere(d *Deck) {
	if desksSandboxedHere {
		d.mu.Lock()
		d.config.SandboxOff = true
		d.mu.Unlock()
	}
}

func TestOnAMacOnlyClaudeCodeIsHiredInTheSandboxAndTheWordsSaySo(t *testing.T) {
	want := map[Agent]string{
		Codex: "Codex hires can't start in Smolcorp's sandbox on a Mac yet, and with the sandbox on, Smolcorp never starts " +
			"a hire outside it. Hire a Claude Code role instead.",
		Gemini: "Gemini CLI hires can't start in Smolcorp's sandbox on a Mac yet, and with the sandbox on, Smolcorp never " +
			"starts a hire outside it. Hire a Claude Code role instead.",
		OpenCode: "OpenCode hires can't start in Smolcorp's sandbox on a Mac yet, and with the sandbox on, Smolcorp never " +
			"starts a hire outside it. Hire a Claude Code role instead.",
		Grok: "Grok hires can't start in Smolcorp's sandbox on a Mac yet, and with the sandbox on, Smolcorp never starts " +
			"a hire outside it. Hire a Claude Code role instead.",
	}
	for _, agent := range append(append([]Agent{}, agents...), ACPAgent("gemini"), "") {
		if got := notOnAMacYet(agent, true); got != want[agent] {
			t.Errorf("%q in the sandbox: %q, want %q", agent, got, want[agent])
		}
		if got := notOnAMacYet(agent, false); got != "" {
			t.Errorf("%q without one: %q", agent, got)
		}
	}
}

// Setup's role editor and hire sheet mark an agent by the core's notOnMac, and the office's line for a role is the
// refusal's words: on a Mac with the switch on only. With it off, and off a Mac, both are as before.
func TestOnAMacSetupAndTheOfficeSayOnlyClaudeCodeIsHiredYet(t *testing.T) {
	d, _, _ := hiringDeck(t, "/usr/bin/tmux")
	d.config.HireInTmux = true
	d.config.Roles = []Role{{ID: "dev", Name: "Dev", Agent: "claude"}, {ID: "rev", Name: "Rev", Agent: "codex"},
		{ID: "doc", Name: "Doc", Agent: "gemini"}, {ID: "bot", Name: "Bot", Agent: "grok"}, {ID: "oc", Name: "OC", Agent: "opencode"}}
	d.config.HireAgent = "grok"
	look := func() (marked map[string]string, office map[string]any, anyone any) {
		t.Helper()
		setup := (&router{deck: d}).setupStateFor(callerPage)
		marked = map[string]string{}
		for _, a := range setup["roles"].(map[string]any)["agents"].([]map[string]any) {
			why, ok := a["notOnMac"].(string)
			if !ok {
				t.Fatalf("Setup's agent %v has no notOnMac", a["id"])
			}
			if why != "" {
				marked[fmt.Sprint(a["id"])] = why
			}
		}
		d.mu.Lock()
		s := d.hiringState()
		d.mu.Unlock()
		office = map[string]any{}
		for _, r := range s["roles"].([]map[string]any) {
			office[fmt.Sprint(r["id"])] = r["unfenced"]
		}
		return marked, office, s["unfenced"]
	}
	asBefore := func(when string) {
		t.Helper()
		marked, office, anyone := look()
		if len(marked) != 0 {
			t.Errorf("%s: Setup marks %v", when, marked)
		}
		for role, name := range map[string]string{"rev": "Codex", "doc": "Gemini CLI", "bot": "Grok", "oc": "OpenCode"} {
			if want := name + " hires don't get Smolcorp's fence: they run with their own settings."; office[role] != want {
				t.Errorf("%s: the office's line for %s: %v, want %q", when, role, office[role], want)
			}
		}
		if want := "Grok hires don't get Smolcorp's fence: they run with their own settings."; anyone != want {
			t.Errorf("%s: Anyone's line: %v", when, anyone)
		}
	}

	// A Mac, the switch on.
	playSandboxedHere(t, true)
	marked, office, anyone := look()
	for _, agent := range []Agent{Codex, Gemini, OpenCode, Grok} {
		if marked[string(agent)] != notOnAMacYet(agent, true) {
			t.Errorf("Setup's mark for %s: %q", agent.Name(), marked[string(agent)])
		}
	}
	if len(marked) != 4 {
		t.Errorf("Setup marks %v, want every agent but Claude Code", marked)
	}
	for role, agent := range map[string]Agent{"rev": Codex, "doc": Gemini, "oc": OpenCode, "bot": Grok} {
		if office[role] != notOnAMacYet(agent, true) {
			t.Errorf("the office's line for %s: %v", role, office[role])
		}
	}
	if anyone != notOnAMacYet(Grok, true) {
		t.Errorf("Anyone's line, on Grok: %v", anyone)
	}

	// A Mac, the switch off: they start without a sandbox, as any hire then does.
	d.mu.Lock()
	d.config.SandboxOff = true
	d.mu.Unlock()
	asBefore("the switch off")

	// Linux and Windows: no sandbox, nothing changes.
	d.mu.Lock()
	d.config.SandboxOff = false
	d.mu.Unlock()
	playSandboxedHere(t, false)
	asBefore("off a Mac")
}
