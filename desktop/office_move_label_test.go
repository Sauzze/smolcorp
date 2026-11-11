package desktop

import (
	"strings"
	"testing"
)

// Moving an agent names it by its nickname, else its folder; the remember line is an exact-folder
// rule, so it says so rather than reading as if it were about that one agent.
func TestTheMoveFloorPickerSaysTheRememberRuleIsForTheFolder(t *testing.T) {
	src, name := string(officeSource(t)), "Resources/web/office.js"
	if !strings.Contains(src, "text: `Always put agents from ${moving.project} here`") {
		t.Errorf("%s: the move picker's remember line doesn't say it is for the agents of that folder", name)
	}
	if strings.Contains(src, "text: `Always put ${moving.project} here`") {
		t.Errorf("%s: the move picker's remember line still reads as if it were about one agent", name)
	}
	open := src[strings.Index(src, "function openMoving(id) {"):]
	open = open[:strings.Index(open, "held = null;")]
	if !strings.Contains(open, "memberOf(session)?.name || session.name") {
		t.Errorf("%s: the move picker no longer names an agent by its nickname, else its folder:\n%s", name, open)
	}
}
