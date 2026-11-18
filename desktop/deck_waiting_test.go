package desktop

import (
	"os"
	"strings"
	"testing"
)

// The Go app has no /approvals route yet, so the deck's "Waiting for you" list must stay hidden
// when the route says no, and answer only with the deck's own /act.
func TestTheDeckHidesWaitingWhereThereIsNoApprovalsRoute(t *testing.T) {
	page, err := os.ReadFile("../Resources/web/deck.html")
	if err != nil {
		t.Fatal(err)
	}
	src := string(page)
	for _, want := range []string{
		"get('/approvals')",
		"if (!res.ok) {",
		"clearInterval(waitingTimer);",
		"await send('/act', { session: item.session, action });",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("deck.html lost %q", want)
		}
	}
	if !strings.Contains(src, `<div class="waiting" id="waiting" role="status" aria-label="Waiting for you" hidden>`) {
		t.Error("the waiting list no longer starts hidden")
	}
	if strings.Contains(src, "/setup/gm") {
		t.Error("the deck must not use the GM's routes")
	}
	routes, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(routes), `"/approvals"`) {
		t.Log("the Go server now has /approvals: this test can also check it answers")
	}
}

// The office tags a waiting desk (the viewer draws with the same file).
func TestTheOfficeTagsAWaitingDesk(t *testing.T) {
	src := string(officeSource(t))
	if !strings.Contains(src, "s.session.state === 'asking' && !DESKS[desk].off") || !strings.Contains(src, "'WAITING'") {
		t.Error("Resources/web/office.js: a waiting desk has no tag")
	}
}
