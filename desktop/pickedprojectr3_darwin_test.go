package desktop

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXPICKEDPROJECTR3: a desk restarted in its sandbox is told its brief's path escaped, on one line, as its first
// message was (B1); and a role's name keeps no character a hire's words would show escaped, as the GM can write
// roles too (N1).

// A pinned desk at a project that isn't a repo, named with a line break and a made-up instruction, has its brief in
// that project. Restarted, its words have no line of the name's own and show the path escaped; a clean path is as
// before.
func TestARestartedDesksBriefInAFolderNamedWithALineBreakIsShownOnOneLine(t *testing.T) {
	isolate(t)
	folder := filepath.Join(homeDir, "Projects", lineBreakName)
	janitor := shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == "janitor" })]
	made, err := makeBrief(briefRequest{role: janitor, library: editableBriefs(bundledBriefs(), t.TempDir()), folder: folder, pinned: true,
		date: time.Now(), kept: t.TempDir()})
	if err != nil || made == nil {
		t.Fatalf("a pinned desk's brief: %v", err)
	}
	if !strings.HasPrefix(made.names.brief, folder+string(filepath.Separator)) {
		t.Fatalf("the brief isn't in the project: %q", made.names.brief)
	}
	words := restartedMessage(Hire{ID: "abc", Brief: made.names.brief, Folder: folder})
	noInjectedLine(t, "the restarted desk's words", words)
	if want := " Your brief is " + shownInWords(made.names.brief) + "."; !strings.HasSuffix(words, want) || !strings.Contains(want, lineBreakShown) {
		t.Errorf("the restarted desk's words don't end %q:\n%s", want, words)
	}

	clean := restartedMessage(Hire{ID: "abc", Brief: "/Users/ada/Projects/notes/BRIEF.md"})
	if want := "Smolcorp started you again, this time in its sandbox, in the same folder and on the same branch as before. " +
		"Your earlier conversation isn't here, so look at the work so far (git status, git log) before going on. " +
		"Your brief is /Users/ada/Projects/notes/BRIEF.md."; clean != want {
		t.Errorf("a clean path: %q, want %q", clean, want)
	}
}

// A role's name with a line end, a tab, an escape, a line or paragraph separator, NEL, DEL or a bidi control, as the
// GM or the page may send it, is kept with a space for it; so the first message and the brief message name it on one
// line, with none of them. A joiner, which emoji hold, stays.
func TestARolesNameKeepsNoCharacterAHiresWordsShowEscaped(t *testing.T) {
	isolate(t)
	for _, c := range []string{"\n", "\r", "\t", "\x1b[31m", "\u2028", "\u2029", "\u0085", "\x7f", "\u202e", "\u2066"} {
		role, ok := Role{ID: "x", Name: "Ab" + c + "Cd", Instructions: "do it"}.checked()
		if !ok {
			t.Errorf("%q: the role was refused", c)
			continue
		}
		if strings.ContainsFunc(role.Name, escapedInWords) {
			t.Errorf("%q: the name is kept as %q", c, role.Name)
		}
		first := role.firstMessageIn(filepath.Join(homeDir, "Projects", "n"), hiredIntoGeneralFolder, pickedStart{})
		brief := role.briefMessage(filepath.Join(homeDir, "b.md"), true)
		for what, text := range map[string]string{"the first message": first, "the brief message": brief} {
			if !strings.Contains(text, role.Name) || strings.Contains(text, "Ab"+c) {
				t.Errorf("%q: %s doesn't name %q on one line:\n%s", c, what, role.Name, text)
			}
		}
	}
	if role, _ := (Role{ID: "x", Name: "Dev \U0001F469\u200d\U0001F4BB"}).checked(); role.Name != "Dev \U0001F469\u200d\U0001F4BB" {
		t.Errorf("a name with a joiner is kept as %q", role.Name)
	}
}
