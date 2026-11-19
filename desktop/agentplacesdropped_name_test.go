//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// RT126AGENTHOMEMACR3 N2: the Mac app tells its core the names of the agent place variables it dropped in
// GoCore.agentPlacesDroppedVariable (Sources/AgentDeck/Engine.swift), and the core reads them from its own
// agentPlacesDroppedVariable. Each side's rows use its own spelling, so this row ties the two: were one renamed, a
// value the app drops would reach the core neither as a value nor as a name, and every desk would start without
// holding where it leads.
func TestTheMacAppAndTheCoreSpellTheDroppedPlacesVariableAlike(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "Sources", "AgentDeck", "Engine.swift"))
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^\s*static let agentPlacesDroppedVariable = "([^"]*)"$`).FindAllSubmatch(data, -1)
	if len(found) != 1 {
		t.Fatalf("Engine.swift spells GoCore.agentPlacesDroppedVariable %d times, want once: update this row", len(found))
	}
	if got := string(found[0][1]); got != agentPlacesDroppedVariable {
		t.Errorf("the Mac app passes the dropped names in %q, but the core reads %q", got, agentPlacesDroppedVariable)
	}
}
