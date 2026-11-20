package desktop

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// FXPICKEDPROJECT: on a Mac whose hires start in their sandbox, the office asks which project a hire works on
// (hirepick.go) and makes its worktree there. The hire is then told that project, its own worktree and its branch,
// as a pinned desk's hire is told its desk's project, and asks only its role's own questions: never which project
// again. Where nothing was picked (the switch off, which is how Windows and Linux hire too, or a hire not into the
// folder for hires), it still asks; a pinned desk's hire is told as before.

// whichProject is the question a hire into the folder for hires asks first, in its brief or its first message.
const whichProject = "Which project should you work on"

// pickedWords is what a hire into its own worktree of project, at folder on branch, is told of where it starts.
func pickedWords(folder, project, branch string) string {
	return "You've started in " + tildePath(folder) + ", your own worktree of " + project +
		", the project you were hired for, on your own branch " + branch + "."
}

// startWords are the words hire hireID's start gives its agent, its first message among them.
func startWords(t *testing.T, hireID string) string {
	t.Helper()
	plan, ok := issuedPlan(hireID)
	if !ok {
		t.Fatalf("no start was issued for %s", hireID)
	}
	return strings.Join(plan.Words, "\n")
}

// An engine hire into a picked project: its brief tells it the project, its worktree and its branch, and asks only
// the janitor's own questions, the first of them first. A role on its own instructions is told so in its first
// message, with no "general folder" and no question which project.
func TestAnEngineHireIntoAPickedProjectIsToldItAndNotAskedWhich(t *testing.T) {
	d, _, _ := officeDeck(t)
	h := pickedHire(t, d, "lib")
	data, err := os.ReadFile(h.Brief)
	if err != nil {
		t.Fatalf("its brief: %v", err)
	}
	brief := string(data)
	if want := pickedWords(startFolder(h), "lib", h.Branch); !strings.Contains(brief, want) {
		t.Errorf("its brief doesn't say %q:\n%s", want, brief)
	}
	if want := "\n\n1. Which project or area should you tidy up?\n2. "; !strings.Contains(brief, want) {
		t.Errorf("its brief doesn't ask the janitor's own questions first (%q):\n%s", want, brief)
	}
	if strings.Contains(brief, whichProject) {
		t.Errorf("its brief still asks which project:\n%s", brief)
	}

	// On its own instructions: no brief, its first message says it all.
	d.mu.Lock()
	i := slices.IndexFunc(d.config.Roles, func(r Role) bool { return r.ID == "janitor" })
	d.config.Roles[i].Instructions = "Tidy what you're asked to."
	d.config.Roles[i].Brief = &RoleBrief{Kind: "text", Text: "Tidy what you're asked to."}
	d.mu.Unlock()
	own := pickedHire(t, d, "lib")
	if own.Brief != "" {
		t.Fatalf("a role on its own instructions got a brief: %s", own.Brief)
	}
	words := startWords(t, own.ID)
	want := pickedWords(startFolder(own), "lib", own.Branch) + " Your first reply must only ask me the questions below"
	if !strings.Contains(words, want) || !strings.Contains(words, "\n\n1. Which project or area should you tidy up?\n") ||
		!strings.Contains(words, "Once I've answered, as my Janitor: Tidy what you're asked to.") {
		t.Errorf("its first message doesn't say %q, then the janitor's questions and instructions:\n%s", want, words)
	}
	if strings.Contains(words, whichProject) || strings.Contains(words, "a general folder") {
		t.Errorf("its first message still asks which project:\n%s", words)
	}
}

// startFolder is where hire h's agent starts: its own worktree, or the folder it was hired into.
func startFolder(h Hire) string {
	if h.Worktree != "" {
		return h.Worktree
	}
	return h.Folder
}

// A plain hire of a role into a picked project (the manager off, no tmux for it): its launcher is told the project
// and its worktree's branch, and the role's first message, as its launcher makes it, says them and asks only the
// role's own questions.
func TestAPlainHireIntoAPickedProjectIsToldItAndNotAskedWhich(t *testing.T) {
	d, l, h := r7wDeck(t, true)
	d.mu.Lock()
	d.config.HireInTmux = false // a window of its own: the plain way
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusOK || !strings.Contains(said, `"pick"`) {
		t.Fatalf("the plain hire with no project: %d %s", status, said)
	}
	d.mu.Lock()
	d.lastHire = time.Time{}
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}, "project": {"lib"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire into lib: %d %s", status, said)
	}
	if len(h.windows) != 1 || len(h.dirs) != 1 {
		t.Fatalf("windows opened: %q", h.windows)
	}
	folder, window := h.dirs[0], h.windows[0]
	if filepath.Dir(folder) != hiresFolder(l.lib, "") {
		t.Fatalf("its window opened in %s, not a worktree of lib", folder)
	}
	at := slices.Index(window, "run")
	if at < 0 {
		t.Fatalf("its window runs %q", window)
	}
	run, err := parseRun(window[at+1:])
	if err != nil {
		t.Fatalf("its launcher's arguments %q: %v", window[at+1:], err)
	}
	branch := ""
	if i := slices.Index(window, "--branch"); i >= 0 && i+1 < len(window) {
		branch = window[i+1]
	}
	if i := slices.Index(window, "--project"); i < 0 || window[i+1] != "lib" || branch == "" || run.role != "janitor" {
		t.Fatalf("its launcher wasn't told the project lib and its branch: %q", window)
	}
	args, err := run.roleStartArguments(folder, "")
	if err != nil {
		t.Fatal(err)
	}
	message := args[len(args)-1]
	if want := pickedWords(folder, "lib", branch) + " Your first reply must only ask me the questions below"; !strings.HasPrefix(message, want) ||
		!strings.Contains(message, "\n\n1. Which project or area should you tidy up?\n") {
		t.Errorf("its first message doesn't start %q, then the janitor's questions:\n%s", want, message)
	}
	if strings.Contains(message, whichProject) || strings.Contains(message, "a general folder") {
		t.Errorf("its first message still asks which project:\n%s", message)
	}
}

// Nothing picked, nothing changes: with the switch off (as Windows and Linux hire, with no sandbox), the engine's hire
// into the folder for hires asks which project in its brief; so does a plain hire whose launcher starts no sandbox,
// whose launcher is told no project.
func TestAHireWithNoProjectPickedStillAsksWhich(t *testing.T) {
	t.Run("the engine's, the switch off", func(t *testing.T) { engineHireAsksWhich(t) })
	t.Run("a plain hire, no sandbox", func(t *testing.T) { plainHireAsksWhich(t) })
}

func engineHireAsksWhich(t *testing.T) {
	d, l, _ := officeDeck(t)
	d.mu.Lock()
	d.config.SandboxOff = true
	d.mu.Unlock()
	if status, said := d.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("the sandbox off: %d %s", status, said)
	}
	d.watching.Wait()
	got := registered(t)
	row := got[len(got)-1]
	if row.Folder != l.projects || row.Worktree != "" {
		t.Fatalf("the sandbox off: %+v", row)
	}
	data, err := os.ReadFile(row.Brief)
	if err != nil {
		t.Fatalf("its brief: %v", err)
	}
	if want := "1. " + whichProject + ": a folder on this computer, or a GitHub repository?\n2. Which project or area should you tidy up?"; !strings.Contains(string(data), want) {
		t.Errorf("its brief doesn't ask %q:\n%s", want, data)
	}
	if strings.Contains(string(data), "the project you were hired for") {
		t.Errorf("its brief tells it a project:\n%s", data)
	}
}

// plainHireAsksWhich: the plain way, with no sandbox for its launcher: no picker, no project told, and it asks.
func plainHireAsksWhich(t *testing.T) {
	plain, _, h := r7wDeck(t, false)
	plain.mu.Lock()
	plain.config.HireInTmux = false
	plain.mu.Unlock()
	if status, said := plain.Perform("hire", "", url.Values{"role": {"janitor"}}); status != http.StatusNoContent {
		t.Fatalf("the plain hire with no sandbox: %d %s", status, said)
	}
	if len(h.windows) != 1 || slices.Contains(h.windows[0], "--project") {
		t.Fatalf("windows opened: %q", h.windows)
	}
	window := h.windows[0]
	run, err := parseRun(window[slices.Index(window, "run")+1:])
	if err != nil {
		t.Fatal(err)
	}
	args, err := run.roleStartArguments(h.dirs[0], "")
	if err != nil {
		t.Fatal(err)
	}
	if message := args[len(args)-1]; !strings.Contains(message, "a general folder, not a project.") || !strings.Contains(message, "1. "+whichProject) {
		t.Errorf("the plain hire's first message doesn't ask which project:\n%s", message)
	}
}

// A pinned desk's hire is told as before, by its launcher (--pinned) and in a brief made for it: its desk's project,
// asked to confirm, then the role's questions.
func TestAPinnedDesksHireIsToldItsProjectAsBefore(t *testing.T) {
	isolate(t)
	token := strings.Repeat("A", 32)
	api := filepath.Join(homeDir, "work", "api")
	run, err := parseRun([]string{"--hire", token, "--pinned", "--agent", "claude", "--role", "janitor"})
	if err != nil {
		t.Fatal(err)
	}
	args, err := run.roleStartArguments(api, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "Your first reply must only ask me the questions below, all together. Don't read code, run commands or start any " +
		"work until I've answered them.\n\n1. You've started in " + filepath.Join("~", "work", "api") + ", this desk's project. Is that the one to work on?\n" +
		"2. Which project or area should you tidy up?\n"
	if message := args[len(args)-1]; !strings.HasPrefix(message, want) {
		t.Errorf("a pinned desk's hire is told\n%s\nwant it to start\n%s", message, want)
	}
	janitor := shippedRoles[slices.IndexFunc(shippedRoles, func(r Role) bool { return r.ID == "janitor" })]
	made, err := makeBrief(briefRequest{role: janitor, library: editableBriefs(bundledBriefs(), t.TempDir()), folder: api, pinned: true, date: time.Now(), kept: t.TempDir()})
	if err != nil || made == nil {
		t.Fatalf("its brief: %v", err)
	}
	if want := "1. You've started in " + filepath.Join("~", "work", "api") + ", this desk's project. Is that the one to work on?\n2. Which project or area should you tidy up?"; !strings.Contains(made.markdown, want) {
		t.Errorf("a pinned desk's brief doesn't ask %q:\n%s", want, made.markdown)
	}
}

// A plain hire's launcher takes a project picked only after its desk, by a name within the folder for hires, and a
// branch only of that desk's; told a project that isn't a repo, with no branch, its role's first message says the
// project's folder is where it works, once (FXPICKEDPROJECTR1, N1). A name with a line break, which the office never
// picks, is told escaped on one line (B1).
func TestAPlainHiresLauncherIsToldOnlyAProjectInTheFolderAndItsDesksBranch(t *testing.T) {
	isolate(t)
	token := strings.Repeat("A", 32)
	desk := "plain-0a1b2c3d4e5f"
	for _, args := range [][]string{
		{"--hire", token, "--desk", desk, "--project", "../lib", "--agent", "claude", "--role", "janitor"},
		{"--hire", token, "--desk", desk, "--project", "/abs/lib", "--agent", "claude", "--role", "janitor"},
		{"--hire", token, "--desk", desk, "--project", "lib", "--branch", "main", "--agent", "claude", "--role", "janitor"},
		{"--hire", token, "--desk", desk, "--project", "lib", "--branch", "hire/plain-ffffffffffff/janitor-0a1b2c", "--agent", "claude", "--role", "janitor"},
	} {
		if run, err := parseRun(args); err == nil {
			t.Errorf("%q was taken as %+v", args, run)
		}
	}
	// A project that isn't a repo: no branch, and it is told the project's folder is where it works.
	run, err := parseRun([]string{"--hire", token, "--desk", desk, "--project", "notes", "--agent", "claude", "--role", "janitor"})
	if err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(homeDir, "Projects", "notes")
	args, err := run.roleStartArguments(notes, "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "You've started in " + filepath.Join("~", "Projects", "notes") + ", the project you were hired for. Your first reply"; !strings.HasPrefix(args[len(args)-1], want) {
		t.Errorf("a hire into a project that isn't a repo is told\n%s\nwant it to start\n%s", args[len(args)-1], want)
	}
	// A line break in its name: told escaped, on one line.
	run, err = parseRun([]string{"--hire", token, "--desk", desk, "--project", "li\nb", "--branch", "hire/" + desk + "/janitor-0a1b2c",
		"--agent", "claude", "--role", "janitor"})
	if err != nil {
		t.Fatal(err)
	}
	if args, err = run.roleStartArguments(notes, ""); err != nil {
		t.Fatal(err)
	}
	if want := ", your own worktree of li\\nb, the project you were hired for, on your own branch hire/" + desk + "/janitor-0a1b2c. Your first reply"; !strings.Contains(args[len(args)-1], want) {
		t.Errorf("a name with a line break is told\n%s\nwant\n%s", args[len(args)-1], want)
	}
}
