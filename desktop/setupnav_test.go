package desktop

import (
	"regexp"
	"strings"
	"testing"
)

// The setup page's sections, each in the side list and in the phone's menu,
// and each of Find a setting's links to something on the page, in sight, in
// the section it names; a section that has moved into another still opens,
// from where it's linked.
func TestEverySetupSectionAndFindASettingLinkLeadsSomewhere(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	panes := regexp.MustCompile(`<section class="pane[^"]*"[^>]* data-pane="(\w+)"`).FindAllStringSubmatchIndex(page, -1)
	if len(panes) < 6 {
		t.Fatalf("the setup page has %d sections", len(panes))
	}
	// Where each section starts and ends on the page
	paneOf := func(i int) string {
		for _, p := range panes {
			if start, end := elementAround(page, p[0]); start <= i && i < end {
				return page[p[2]:p[3]]
			}
		}
		return ""
	}
	for _, p := range panes {
		pane := page[p[2]:p[3]]
		if !strings.Contains(page, `<button data-go="`+pane+`">`) {
			t.Errorf("section %s isn't in the side list", pane)
		}
		if !strings.Contains(page, `<option value="`+pane+`">`) {
			t.Errorf("section %s isn't in the phone's menu", pane)
		}
	}
	away := strings.Index(page, `id="also-elsewhere"`)
	if away < 0 {
		t.Fatal("the copies kept out of sight aren't on the page")
	}
	awayStart, awayEnd := elementAround(page, away)
	keptAway := func(i int) bool { return awayStart <= i && i < awayEnd }

	jumps := regexp.MustCompile(`<button class="jump" data-go="(\w+)" data-at="([\w-]+)">`).FindAllStringSubmatch(page, -1)
	if len(jumps) < 5 {
		t.Fatalf("Find a setting has %d links", len(jumps))
	}
	for _, jump := range jumps {
		at := strings.Index(page, `id="`+jump[2]+`"`)
		switch {
		case at < 0:
			t.Errorf("Find a setting links to #%s, which isn't on the page", jump[2])
		case keptAway(at):
			t.Errorf("Find a setting links to #%s, a copy kept out of sight", jump[2])
		case paneOf(at) != jump[1]:
			t.Errorf("Find a setting says #%s is in %s, but it's in %q", jump[2], jump[1], paneOf(at))
		}
	}
	// Search skips the copies out of sight: none of them is one of its settings
	for _, m := range regexp.MustCompile(`data-search="([^"]*)"`).FindAllStringSubmatchIndex(page, -1) {
		if keptAway(m[0]) {
			t.Errorf("the copy kept out of sight of %q is a setting search finds", page[m[2]:m[3]])
		}
	}
	// The old Profiles section is now Office and look's saved looks
	if at := strings.Index(page, `id="profiles-section"`); at < 0 || paneOf(at) != "office" {
		t.Error("the saved looks aren't in Office and look, where the old Profiles link leads")
	}
}

// A copy kept out of sight isn't searched, so the words it was found by are
// on the choice in sight: searching them still finds it.
func TestTheWordsOfEachCopyOutOfSightFindTheChoiceInSight(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()
	away := strings.Index(page, `id="also-elsewhere"`)
	if away < 0 {
		t.Fatal("the copies kept out of sight aren't on the page")
	}
	start, end := elementAround(page, away)
	keywords := regexp.MustCompile(`data-keywords="([^"]*)"`)
	inSight := map[string]bool{}
	for _, m := range keywords.FindAllStringSubmatchIndex(page, -1) {
		if m[0] < start || m[0] >= end {
			for _, w := range strings.Fields(page[m[2]:m[3]]) {
				inSight[w] = true
			}
		}
	}
	hidden := keywords.FindAllStringSubmatch(page[start:end], -1)
	if len(hidden) == 0 {
		t.Fatal("no words found on the copies kept out of sight")
	}
	for _, m := range hidden {
		for _, w := range strings.Fields(m[1]) {
			if !inSight[w] {
				t.Errorf("%q, a word of a copy now out of sight, is on nothing in sight: searching it finds nothing", w)
			}
		}
	}
}

// elementAround is where the element whose opening tag holds page[i] starts
// and ends, its own tags counted, as a <section> within a section.
func elementAround(page string, i int) (start, end int) {
	start = strings.LastIndex(page[:i+1], "<")
	name := regexp.MustCompile(`^<(\w+)`).FindStringSubmatch(page[start:])[1]
	tags := regexp.MustCompile(`<(/?)` + name + `[\s>]`)
	depth := 0
	for _, m := range tags.FindAllStringSubmatchIndex(page[start:], -1) {
		if m[3] > m[2] {
			depth--
		} else {
			depth++
		}
		if depth == 0 {
			return start, start + m[1]
		}
	}
	return start, len(page)
}
