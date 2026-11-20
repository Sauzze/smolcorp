//go:build !windows

package desktop

import (
	"encoding/json"
	"strings"
	"testing"
)

// FXPICKEDPROJECT's Setup hints: where a role hired now starts in its sandbox (a Mac's, the switch on), the office
// asks which project first and tells the hire, so Setup's roles hint and each role card's say its first reply asks
// only its own questions. Setup's state says so (hiresSandboxed), as the hire itself is decided (Deck.hiresSandboxed):
// with a stand-in sandbox runner, true with the switch on and false with it off. Elsewhere the hints are as before.
func TestSetupSaysTheOfficeAsksWhichProjectFirstOnlyWhereHiresStartInTheirSandbox(t *testing.T) {
	d := newTestDeck(t)
	d.mu.Lock()
	d.engine.Sandbox = func(*deskSandbox) tmuxStarter { return nil } // never started here: only whether there's one
	d.mu.Unlock()
	for _, off := range []bool{false, true} {
		d.mu.Lock()
		d.config.SandboxOff = off
		want := d.hiresSandboxed()
		d.mu.Unlock()
		if want == off {
			t.Fatalf("the switch off %v: hires sandboxed %v", off, want)
		}
		var state struct {
			HiresSandboxed *bool `json:"hiresSandboxed"`
		}
		got := serve(t, d, call{method: "GET", target: "/setup/state", host: "localhost:7710", from: thisComputer})
		if json.Unmarshal(got.Body.Bytes(), &state) != nil || state.HiresSandboxed == nil || *state.HiresSandboxed != want {
			t.Errorf("the switch off %v: setup's state says hiresSandboxed %v, want %v", off, state.HiresSandboxed, want)
		}
	}
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	for _, want := range []string{
		"const picked = !!state.hiresSandboxed;",
		": state.launcher && picked\n",
		"own questions: the office asks which project first. It waits for your answers",
		"said.textContent = !state.hiresSandboxed ? 'Its first reply only asks you which project to work on, then these, one per line:'",
		"'Its first reply only asks you these, one per line (the office asks which project first):'",
		// As before, where the hire asks which project itself.
		"which project to work on and its own questions, and it waits for your answers before it works on its instructions.",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("setup.html has no %q", want)
		}
	}
}
