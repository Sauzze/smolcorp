package desktop

import (
	"bytes"
	"strings"
	"testing"
)

// Moving an agent names it by its nickname, else its folder; the remember line is an exact-folder
// rule, so it says so rather than reading as if it were about that one agent.
func TestTheMoveFloorPickerSaysTheRememberRuleIsForTheFolder(t *testing.T) {
	web, viewer := officeSources(t)
	for name, src := range map[string]string{"Resources/web/office.js": string(web), "viewer/public/office.js": string(viewer)} {
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
}

// The two copies of the office page are served from different places and must stay the same.
func TestTheTwoOfficePagesAreIdentical(t *testing.T) {
	web, viewer := officeSources(t)
	if !bytes.Equal(web, viewer) {
		t.Error("Resources/web/office.js and viewer/public/office.js differ")
	}
}
