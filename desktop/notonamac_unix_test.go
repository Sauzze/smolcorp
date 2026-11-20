//go:build !windows

package desktop

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// FXPLANTSTERMR2 (B) and R3: every hire route refuses a Codex, Gemini CLI, OpenCode or Grok hire on a Mac while the
// sandbox is in force, in words, before the cooldown is spent or anything is looked for or made. The GM's own routes
// keep the fence's sentence (the GM hires only Claude roles anywhere), and T3 Code its own (Claude Code only). With
// the owner's switch off they are hired as before.
func TestOnAMacAHireOfAnyAgentButClaudeCodeIsRefusedOnEveryRouteWhileTheSandboxIsOn(t *testing.T) {
	playSandboxedHere(t, true)
	const (
		fence = "The General Manager hires only Claude roles: Smolcorp can't fence another agent."
		t3    = "Only a Claude Code role is hired into T3 Code: the fence there is Claude Code's."
	)
	for _, agent := range []Agent{Codex, Gemini, OpenCode, Grok} {
		t.Run(agent.Name(), func(t *testing.T) {
			stopDecksOf(t)
			d, r, h, gm := acpRoleDeck(t, string(agent))
			words := notOnAMacYet(agent, true)
			body := `{"role":"navigator","folder":"` + r.repo + `","answers":[]}`
			asPage := call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: body,
				headers: map[string]string{setupHeader: "1"}}
			asGM := call{method: "POST", target: "/setup/hire", host: "localhost:7710", from: thisComputer, body: body,
				withoutSetupKey: true, headers: map[string]string{setupHeader: "1", gmTokenHeader: gm}}
			intoT3 := asPage
			intoT3.body = `{"role":"navigator","folder":"` + r.repo + `","answers":[],"engine":"t3"}`
			post := func(c call) (int, string) {
				w := serve(t, d, c)
				d.watching.Wait()
				return w.Code, strings.TrimSpace(w.Body.String())
			}
			for _, c := range []struct {
				name, want string
				hire       func() (int, string)
			}{
				{"the office's hire", words, func() (int, string) { return d.Perform("hire", "", url.Values{"role": {"navigator"}}) }},
				{"the office's hire upstairs", words, func() (int, string) { return d.hireUpstairs("1", "0", "navigator") }},
				{"the office's Anyone, on this agent", words, func() (int, string) {
					d.mu.Lock()
					d.config.HireAgent = string(agent)
					d.mu.Unlock()
					return d.Perform("hire", "", url.Values{})
				}},
				{"Setup's hire sheet", words, func() (int, string) { return post(asPage) }},
				{"Setup's hire sheet with the GM's token", words, func() (int, string) { return post(asGM) }},
				{"Setup's hire sheet into T3 Code", t3, func() (int, string) { return post(intoT3) }},
				{"Setup's own hire (Start here's)", words, func() (int, string) { return d.hireAs("", "navigator", 0, true) }},
				{"the GM's route", fence, func() (int, string) { return postGMHire(t, d, asGMCaller, gm, `{"role":"navigator"}`) }},
				{"the GM's night queue", fence, func() (int, string) { return d.hireQueueRole(hireStep{Role: "navigator", Folder: r.repo}) }},
			} {
				// Each route on its own: a route that opened a window would otherwise leave the next one the cooldown's 429.
				d.mu.Lock()
				d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
				d.mu.Unlock()
				if status, why := c.hire(); status != http.StatusConflict || why != c.want {
					t.Errorf("%s: %d %q, want 409 %q", c.name, status, why, c.want)
				}
			}
			// None looked for the agent, opened a window or tmux, or made a row or a worktree, or spent the cooldown.
			if len(h.lookedFor) != 0 {
				t.Errorf("a hire looked for %q on the computer", h.lookedFor)
			}
			nothingMade(t, r, h)
			d.mu.Lock()
			hired := d.hiring || !d.lastHire.IsZero()
			d.mu.Unlock()
			if hired {
				t.Error("a refused hire took the cooldown")
			}

			// The owner's switch off: hired as before, without a sandbox (the launcher follows the switch: FXSWITCHPLAIN).
			d.mu.Lock()
			d.config.SandboxOff = true
			d.mu.Unlock()
			if status, why := d.Perform("hire", "", url.Values{"role": {"navigator"}}); status != http.StatusNoContent ||
				len(h.windows) != 1 {
				t.Errorf("the switch off, the office's hire: %d %q, %d windows, %d in tmux", status, why, len(h.windows), len(h.tmux))
			}
			d.mu.Lock()
			d.lastHire, d.gmLastHire = time.Time{}, time.Time{}
			d.mu.Unlock()
			if status, why := post(asPage); status != http.StatusOK || len(h.windows) != 2 {
				t.Errorf("the switch off, Setup's hire sheet: %d %q, %d windows, %d in tmux", status, why, len(h.windows), len(h.tmux))
			}
		})
	}
}
