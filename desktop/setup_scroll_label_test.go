package desktop

import (
	"regexp"
	"strings"
	"testing"
)

// On Linux the setup page retitles the scroll switch to "tmux and kitty";
// its accessible name has to say the same, or screen readers read "Terminal".
func TestTheScrollSwitchSaysWhatItsTitleSaysOnLinux(t *testing.T) {
	d := newTestDeck(t)
	page := serve(t, d, call{method: "GET", target: "/setup", host: "localhost:7710", from: thisComputer}).Body.String()

	title := regexp.MustCompile(`scroll\.querySelector\('\.title'\)\.textContent = '([^']+)';`).FindStringSubmatch(page)
	if title == nil {
		t.Fatal("setup page no longer sets the scroll switch's title in describeLauncher")
	}
	label := regexp.MustCompile(`\$\('scroll'\)\.setAttribute\('aria-label', '([^']+)'\)`).FindStringSubmatch(page)
	if label == nil {
		t.Fatalf("setup page sets the scroll switch's title to %q but not its aria-label", title[1])
	}
	if label[1] != title[1] {
		t.Errorf("scroll switch aria-label %q, want the title %q", label[1], title[1])
	}
	if !strings.Contains(page, `id="scroll" aria-label=`) {
		t.Error("scroll switch lost its aria-label")
	}
}
