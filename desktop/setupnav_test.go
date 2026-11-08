package desktop

import (
	"regexp"
	"strings"
	"testing"
)

// The setup page's sections, each in the side list and in the phone's menu,
// and each of Find a setting's links to something on the page; a section
// that has moved into another still opens, from where it's linked.
func TestEverySetupSectionAndFindASettingLinkLeadsSomewhere(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	panes := regexp.MustCompile(`<section class="pane[^"]*"[^>]* data-pane="(\w+)"`).FindAllStringSubmatch(page, -1)
	if len(panes) < 6 {
		t.Fatalf("the setup page has %d sections", len(panes))
	}
	for _, pane := range panes {
		if !strings.Contains(page, `<button data-go="`+pane[1]+`">`) {
			t.Errorf("section %s isn't in the side list", pane[1])
		}
		if !strings.Contains(page, `<option value="`+pane[1]+`">`) {
			t.Errorf("section %s isn't in the phone's menu", pane[1])
		}
	}
	jumps := regexp.MustCompile(`<button class="jump" data-go="(\w+)" data-at="([\w-]+)">`).FindAllStringSubmatch(page, -1)
	if len(jumps) < 5 {
		t.Fatalf("Find a setting has %d links", len(jumps))
	}
	for _, jump := range jumps {
		if !strings.Contains(page, `id="`+jump[2]+`"`) {
			t.Errorf("Find a setting links to #%s, which isn't on the page", jump[2])
		}
	}
	if !strings.Contains(page, "const MOVED = { profiles: ['office', 'profiles-section'] };") || !strings.Contains(page, `data-go="profiles"`) {
		t.Error("the Profiles link no longer leads to the saved looks")
	}
}
