package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var paneRule = strings.Repeat("─", 60)

// pane loads a capture Claude Code 2.1.316 really made, from the fixtures the
// Mac tests use (Tests/fixtures/panes). A fixture is never edited.
func pane(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "Tests", "fixtures", "panes", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// paneBox is a window shaped like Claude Code's: old output, the box between
// two rules, then the footer.
func paneBox(footer string, lines ...string) string {
	return "old output\n" + paneRule + "\n" + strings.Join(lines, "\n") + "\n" + paneRule + "\n" + footer
}

const paneAuto = "  ⏵⏵ auto mode on (shift+tab to cycle)"

// paneShort is a run of ─ one short of a rule.
var paneShort = strings.Repeat("─", 19)

func TestRealPaneCapturesAreToldApartByTheirStructure(t *testing.T) {
	for name, want := range map[string]PaneState{
		"idle-empty":      PaneIdle,
		"after-decline":   PaneIdle,
		"draft":           PaneDraft,
		"busy-1":          PaneBusy,
		"permission-menu": PaneMenu,
		"trust-menu":      PaneMenu,
	} {
		if got := ReadPaneState(pane(t, name)); got != want {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
}

func TestQuestionWordsInTheScrollbackCountForNothing(t *testing.T) {
	words := "Do you want to proceed?\n❯ 1. Yes\nEsc to cancel\nEnter to confirm\n(y/n)\n"
	if got := ReadPaneState(words + pane(t, "idle-empty")); got != PaneIdle {
		t.Errorf("idle under old question words: %v", got)
	}
	// The old box-drawing look of an input box is not one.
	if got := ReadPaneState("│ > │"); got != PaneUnknown {
		t.Errorf("old box look: %v", got)
	}
}

func TestSyntheticPaneVariants(t *testing.T) {
	menu := "\n" + paneRule + "\n Do you want to proceed?\n ❯ 1. Yes\n   2. No\n Esc to cancel"
	for _, c := range []struct {
		name    string
		capture string
		want    PaneState
	}{
		{"long draft", paneBox(paneAuto, "❯ "+strings.Repeat("word ", 80)), PaneDraft},
		{"pasted text", paneBox(paneAuto, "❯ [Pasted text #1 +3 lines]"), PaneDraft},
		{"multi-line draft", paneBox(paneAuto, "❯ first line", "  second line", "  third line"), PaneDraft},
		{"empty first line, pasted second", paneBox(paneAuto, "❯", "  and a pasted second line"), PaneDraft},
		{"Try placeholder with a no-break space", paneBox(paneAuto, "❯ Try \"fix a bug\""), PaneIdle},
		{"empty box", paneBox(paneAuto, "❯"), PaneIdle},
		{"busy beats an empty box", paneBox("  ⏵⏵ auto mode on · esc to interrupt", "❯"), PaneBusy},
		{"busy beats a draft", paneBox("  ⏵⏵ auto mode on · esc to interrupt", "❯ half"), PaneBusy},
		{"no footer: cut off", paneBox("", "❯"), PaneUnknown},
		{"a screenful under the box is no box", paneBox("a\nb\nc\nd", "❯"), PaneUnknown},
		{"two-option menu", "text\n" + paneRule + "\n Overwrite the file?\n ❯ 1. Yes\n   2. No\n", PaneMenu},
		{"confirm footer", "Pick a folder\n ❯ Downloads\n   Desktop\n\n Enter to confirm · Esc to cancel", PaneMenu},
		{"old box with a menu under it", paneBox(paneAuto, "❯") + menu, PaneMenu},
		{"nothing like a box", "nothing like a box", PaneUnknown},
		{"nothing at all", "", PaneUnknown},
		{"CRLF capture", strings.ReplaceAll(paneBox(paneAuto, "❯ typed"), "\n", "\r\n"), PaneDraft},
	} {
		if got := ReadPaneState(c.capture); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// Guards for single rules of the reading, each checked to fail when its rule
// is taken out of panestate.go.
func TestPaneRulesOfTheReadingAreGuarded(t *testing.T) {
	short := "old output\n" + paneShort + "\n❯\n" + paneShort + "\n" + paneAuto
	exact := strings.ReplaceAll(short, paneShort, strings.Repeat("─", 20))
	list := "text\n" + paneRule + "\n Pick a model\n ❯ Option one\n   Option two\n"
	for _, c := range []struct {
		name    string
		capture string
		want    PaneState
	}{
		// The box's first line starts with ❯: plain text between two rules
		// with a one-line footer is not an empty box (the false-idle guard).
		{"two rules around plain text", paneBox(paneAuto, "just some text"), PaneUnknown},
		{"two rules, plain text, no selector first", paneBox(paneAuto, "plain", "❯"), PaneUnknown},
		// Try "..." must open and close with a quote to be the placeholder.
		{"unfinished Try text", paneBox(paneAuto, "❯ Try \"abc"), PaneDraft},
		{"Try text without its opening quote", paneBox(paneAuto, "❯ Try abc\""), PaneDraft},
		{"finished Try text", paneBox(paneAuto, "❯ Try \"abc\""), PaneIdle},
		// A choice list with no numbers is a question too (the Mac's
		// testAnUnnumberedChoiceListIsAsking), by either footer phrase.
		{"unnumbered list, select and navigate", list + " Enter to select · ↑/↓ to navigate · Esc to go back\n", PaneMenu},
		{"unnumbered list, select only", list + " Enter to select\n", PaneMenu},
		{"unnumbered list, navigate only", list + " ↑/↓ to navigate\n", PaneMenu},
		// A rule needs 20 characters: shorter runs of ─ make no box.
		{"rules of 19 characters", short, PaneUnknown},
		{"rules of 20 characters", exact, PaneIdle},
		// Footer words count only on the last line with words.
		{"footer words above the last line", "Enter to confirm\nEsc to cancel\nEnter to select\nto navigate\nsome output\nmore output", PaneUnknown},
		{"footer words on the last line", "some output\nEnter to confirm", PaneMenu},
	} {
		if got := ReadPaneState(c.capture); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
