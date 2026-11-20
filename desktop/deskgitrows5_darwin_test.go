package desktop

import (
	"strings"
	"testing"
)

// FXGITROWS2R5 (the owner's ruling): a hire in its sandbox in a worktree of its own is told plainly, in its
// start words, which branches are its own, that other branches, tags and git stash aren't open to it, and what to do
// instead, with the stand-in start-only runner of desksandbox_test.go. Its sentence ends the start words' last
// paragraph, so FXHIREPROJECTS' sentence on the projects folder joins it in front (deskStartWords).
func TestOnAMacAHireInItsOwnWorktreeIsToldWhichBranchesAreItsOwn(t *testing.T) {
	r := newSandboxRig(t)
	var started string
	request := r.request("janitor")
	request.StartTyped = func(message string) { started = message }
	hire, err := runManagedHire(request, r.o)
	if err != nil {
		t.Fatal(err)
	}
	folder, _, ok := strings.Cut(strings.TrimPrefix(hire.Branch, "hire/"), "/")
	if !ok || hire.Worktree.none() {
		t.Fatalf("the hire's branch %q, worktree %q", hire.Branch, hire.Worktree)
	}
	want := "You can commit only to your own branches, " + hire.Branch + " and any new one under hire/" + folder +
		"/: other branches, tags and git stash aren't open to you here, so save work in progress as a commit on your branch," +
		" and ask the General Manager or the owner when you need another branch or a tag."
	paragraphs := strings.Split(started, "\n\n")
	if last := paragraphs[len(paragraphs)-1]; len(paragraphs) < 2 || !strings.HasSuffix(last, want) {
		t.Errorf("its start words' last paragraph doesn't end %q:\n%s", want, started)
	}

	// A hire with no worktree of its own can't commit in the shared .git: it isn't told of branches.
	started = ""
	request = r.request("janitor")
	request.NoWorktree = true
	request.StartTyped = func(message string) { started = message }
	if _, err := runManagedHire(request, r.o); err != nil {
		t.Fatal(err)
	}
	if started == "" || strings.Contains(started, "You can commit only") {
		t.Errorf("a hire with no worktree is told of branches:\n%s", started)
	}
}

// The words, alone: only on a Mac, whose rows these are; a branch from before branches had a folder of their own; and
// how they join the projects sentence at the stack, one paragraph.
func TestABranchSentenceJoinsTheOtherStartWordsInOneParagraph(t *testing.T) {
	if got := deskBranchWords("linux", true, false, "hire/ab12/work"); got != "" {
		t.Errorf("on Linux: %q", got)
	}
	if got := deskBranchWords("darwin", true, false, ""); got != "" {
		t.Errorf("with no branch: %q", got)
	}
	if got := deskBranchWords("darwin", true, false, "hire/ada"); !strings.HasPrefix(got, "You can commit only to your own branch, hire/ada: other branches") {
		t.Errorf("a branch with no folder: %q", got)
	}
	projects := "Your projects folder is /p. You can change files only in your own worktree here; other projects are read-only."
	branch := deskBranchWords("darwin", true, false, "hire/ab12/work")
	if got, want := deskStartWords(projects, branch), "\n\n"+projects+" You can commit only to your own branches, hire/ab12/work and any new one under hire/ab12/: "; !strings.HasPrefix(got, want) || strings.Count(got, "\n") != 2 {
		t.Errorf("joined: %q, want it to start %q, one paragraph", got, want)
	}
	if got := deskStartWords("", branch); got != "\n\n"+branch {
		t.Errorf("the branch sentence alone: %q", got)
	}
	if got := deskStartWords("", ""); got != "" {
		t.Errorf("no sentence: %q", got)
	}
}
