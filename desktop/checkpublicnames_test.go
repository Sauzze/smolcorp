//go:build unix

package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The team's names, put together as the row runs, so this file names none of them (as a test's fake key is).
var checkPublicTeamNames = []string{"SIER" + "RA", "VIC" + "TOR", "JULI" + "ETT", "WHIS" + "KEY", "RO" + "MEO", "AL" + "PHA"}

// checkPublicRepo makes a stand-in repo holding the real scripts/check-public.sh, and gives its folder, its git, a way
// to write a file into it, and a run of the script with the release base given in SMOLCORP_RELEASE_BASE.
func checkPublicRepo(t *testing.T) (dir string, git func(args ...string) string, write func(name, text string), check func(base string) (bool, string)) {
	t.Helper()
	for _, tool := range []string{"bash", "git", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	script, err := os.ReadFile(filepath.Join("..", "scripts", "check-public.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	env := []string{"HOME=" + dir, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); name != "HOME" && !strings.HasPrefix(name, "GIT_") && name != "SMOLCORP_RELEASE_BASE" {
			env = append(env, kv)
		}
	}
	git = func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=ada", "-c", "user.email=ada@example.invalid",
			"-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write = func(name, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	check = func(base string) (bool, string) {
		t.Helper()
		cmd := exec.Command("bash", filepath.Join(dir, "scripts", "check-public.sh"))
		cmd.Dir, cmd.Env = dir, append(append([]string{}, env...), "SMOLCORP_RELEASE_BASE="+base)
		out, err := cmd.CombinedOutput()
		if _, exited := err.(*exec.ExitError); err != nil && !exited {
			t.Fatal(err)
		}
		return err == nil, string(out)
	}
	git("init", "-q")
	write("scripts/check-public.sh", string(script))
	return dir, git, write, check
}

// scripts/check-public.sh refuses a line added since the release base that names the team, as a whole word in
// capitals, and nothing else: a line the base already had (kept, or moved to another file), and the same letters in
// small letters, in a longer word or in a name joined by an underscore all pass. A clone without the base checks every
// line instead (TestCheckPublicRefusesATeamNameInAnyLineOfACloneWithoutTheReleaseBase). The real script, copied into
// a stand-in repo whose base is given in SMOLCORP_RELEASE_BASE.
func TestCheckPublicRefusesATeamNameInALineAddedSinceTheReleaseBase(t *testing.T) {
	dir, git, write, check := checkPublicRepo(t)
	old := "// the ruling " + checkPublicTeamNames[0] + " gave, before the release\n"
	write("old.go", "package x\n\n"+old)
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD^{tree}")

	if ok, out := check(base); !ok || !strings.HasSuffix(out, "Nothing personal in what is committed.\n") {
		t.Fatalf("nothing added: want a pass, got:\n%s", out)
	}
	// The base's own line, moved to another file with nothing changed: not an added line.
	git("mv", "old.go", "moved.go")
	if ok, out := check(base); !ok {
		t.Errorf("the base's line moved: want a pass, got:\n%s", out)
	}
	// The same letters, not a name of the team: in small letters, in a longer word, joined by an underscore.
	for _, name := range checkPublicTeamNames {
		lower := strings.ToLower(name)
		title := strings.ToUpper(lower[:1]) + lower[1:]
		write("words.go", "package x\n\n// "+title+", "+lower+", "+name+"S, X"+name+", "+name+"_KEY and "+lower+"_"+name+"2.\n")
		git("add", "words.go")
		if ok, out := check(base); !ok {
			t.Errorf("%s's letters in other words: want a pass, got:\n%s", title, out)
		}
	}
	git("rm", "-q", "--cached", "words.go")
	if err := os.Remove(filepath.Join(dir, "words.go")); err != nil {
		t.Fatal(err)
	}
	// Each name, as a whole word in capitals, in a line added in a comment, a string or a web page: refused, by its
	// file and line.
	for i, name := range checkPublicTeamNames {
		file := []string{"added.go", "page.html", "notes.md"}[i%3]
		write(file, "one line\n\nthe rule ("+name+"'s ruling): kept\n")
		git("add", file)
		ok, out := check(base)
		if ok || !strings.Contains(out, file+":3: a name of the team in a line added since the release base") {
			t.Errorf("%s in an added line of %s: want it refused at line 3, got:\n%s", strings.ToLower(name), file, out)
		}
		if strings.Contains(out, name) {
			t.Errorf("the refusal repeats the name:\n%s", out)
		}
		git("rm", "-q", "--cached", file)
		if err := os.Remove(filepath.Join(dir, file)); err != nil {
			t.Fatal(err)
		}
	}
	// A line changed to name the team is an added line too.
	write("moved.go", "package x\n\n"+strings.Replace(old, "the ruling", "the ruling "+checkPublicTeamNames[1]+" and", 1))
	if ok, out := check(base); ok || !strings.Contains(out, "moved.go:3:") {
		t.Errorf("a changed line naming the team: want it refused, got:\n%s", out)
	}
	// Without the base here, every line is checked instead: the base's own line, kept, is refused too, and the run
	// says why.
	missing := strings.Repeat("e", 40)
	if ok, out := check(missing); ok || !strings.Contains(out, "the release base "+missing+" is not in this clone") ||
		!strings.Contains(out, "moved.go:3: a name of the team, in a clone without the release base") {
		t.Errorf("no base: want moved.go's line refused, with a note, got:\n%s", out)
	}
}

// A clone without the release base (a shallow one, as CI's and the release machine's can be) can't tell an added line
// from an old one, so check-public.sh checks every line of its .go, .html, .js, .swift, .md and .yml files for the
// team's names instead, and says so: a planted name is still refused, by its file and line, whether it was there
// before the base or not, and the same letters in other words, or a name in another kind of file, pass.
func TestCheckPublicRefusesATeamNameInAnyLineOfACloneWithoutTheReleaseBase(t *testing.T) {
	_, git, write, check := checkPublicRepo(t)
	write("main.go", "package x\n\n// the rule, as the owner ruled it\n")
	git("add", "-A")
	git("commit", "-q", "-m", "first")
	missing := strings.Repeat("e", 40)
	note := "Note: the release base " + missing + " is not in this clone, so every line of its .go, .html, .js, .swift, .md, .yml files was checked for the team names, but for the lines the base had."

	if ok, out := check(missing); !ok || !strings.Contains(out, note) || !strings.HasSuffix(out, "Nothing personal in what is committed.\n") {
		t.Fatalf("no name anywhere: want a pass with the note, got:\n%s", out)
	}
	// The same letters in other words, and a name in a kind of file the rule doesn't read: a pass.
	for _, name := range checkPublicTeamNames {
		lower := strings.ToLower(name)
		title := strings.ToUpper(lower[:1]) + lower[1:]
		write("words.go", "package x\n\n// "+title+", "+lower+", "+name+"S, X"+name+", "+name+"_KEY and "+lower+"_"+name+"2.\n")
		write("data.txt", "the rule ("+name+"'s ruling)\n")
		git("add", "words.go", "data.txt")
		if ok, out := check(missing); !ok {
			t.Errorf("%s's letters in other words, or in a .txt file: want a pass, got:\n%s", title, out)
		}
	}
	// A line the release base had, listed beside the script: it passes, kept or moved, but not once it's changed.
	kept := "// the rule " + checkPublicTeamNames[0] + " gave, before the release"
	write("scripts/check-public-base-lines.txt", "# the base's lines\n"+checkPublicBaseLine("\t"+kept+"  ")+"\n")
	write("old.go", "package x\n\n\t"+kept+"\n")
	git("add", "-A")
	if ok, out := check(missing); !ok || !strings.Contains(out, note) {
		t.Errorf("a line the base had: want a pass with the note, got:\n%s", out)
	}
	git("mv", "old.go", "moved.md")
	if ok, out := check(missing); !ok {
		t.Errorf("a line the base had, moved: want a pass, got:\n%s", out)
	}
	write("moved.md", "package x\n\n"+strings.Replace(kept, "the rule", "the rule "+checkPublicTeamNames[1]+" and", 1)+"\n")
	if ok, out := check(missing); ok || !strings.Contains(out, "moved.md:3: a name of the team, in a clone without the release base") {
		t.Errorf("a line the base had, changed to name the team: want it refused, got:\n%s", out)
	}
	git("rm", "-q", "-f", "moved.md")

	// Each name, planted as a whole word in capitals in each kind of file the rule reads, committed or only staged:
	// refused, by its file and line, without repeating the name.
	files := []string{"planted.go", "page.html", "office.js", "App.swift", "notes.md", ".github/workflows/ci.yml"}
	for i, name := range checkPublicTeamNames {
		file := files[i%len(files)]
		write(file, "one line\n\nthe rule ("+name+"'s ruling): kept\n")
		git("add", file)
		if i%2 == 0 {
			git("commit", "-q", "-m", "planted")
		}
		ok, out := check(missing)
		if ok || !strings.Contains(out, note) || !strings.Contains(out, file+":3: a name of the team, in a clone without the release base") {
			t.Errorf("%s in %s: want it refused at line 3, got:\n%s", strings.ToLower(name), file, out)
		}
		if strings.Contains(out, name) {
			t.Errorf("the refusal repeats the name:\n%s", out)
		}
		git("rm", "-q", "-f", file)
		if i%2 == 0 {
			git("commit", "-q", "-m", "gone")
		}
	}
	if ok, out := check(missing); !ok {
		t.Errorf("every planted name gone: want a pass, got:\n%s", out)
	}
}

// checkPublicBaseLine is a line's entry in scripts/check-public-base-lines.txt: the SHA-256 of its text, trimmed, with
// the list's own prefix.
func checkPublicBaseLine(line string) string {
	sum := sha256.Sum256([]byte("smolcorp-team-line:" + strings.TrimSpace(line)))
	return hex.EncodeToString(sum[:])
}

// scripts/check-public-base-lines.txt is exactly the lines of the default release base that name the team, in its
// .go, .html, .js, .swift, .md and .yml files: what check-public.sh lets through in a clone without the base, so no
// other line slips through there, and the base's own lines don't fail it. SMOLCORP_CHECK_PUBLIC_LINES=write remakes
// the list. A clone without the base (the Studio's) skips the row: CI's public job has it.
func TestCheckPublicsBaseLinesAreTheReleaseBases(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "scripts", "check-public.sh"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`os\.environ\.get\("SMOLCORP_RELEASE_BASE"\) or "([0-9a-f]{40})"`).FindSubmatch(script)
	if m == nil {
		t.Fatal("check-public.sh names no default release base")
	}
	base := string(m[1])
	git := func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = ".."
		return cmd.Output()
	}
	if _, err := git("cat-file", "-e", base+"^{tree}"); err != nil {
		t.Skipf("the release base %s is not in this clone", base)
	}
	args := []string{"-c", "core.quotePath=false", "grep", "-h", "-I", "-F"}
	for _, name := range checkPublicTeamNames {
		args = append(args, "-e", name)
	}
	args = append(args, base, "--", "*.go", "*.html", "*.js", "*.swift", "*.md", "*.yml")
	out, err := git(args...)
	if exit, ok := err.(*exec.ExitError); err != nil && !(ok && exit.ExitCode() == 1) {
		t.Fatal(err)
	}
	team := map[string]bool{}
	for _, name := range checkPublicTeamNames {
		team[name] = true
	}
	words := regexp.MustCompile(`[A-Za-z0-9_]+`)
	want := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		for _, word := range words.FindAllString(line, -1) {
			if team[word] {
				want[checkPublicBaseLine(line)] = true
				break
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("no line of the release base names the team: the grep found nothing")
	}
	sorted := make([]string, 0, len(want))
	for sum := range want {
		sorted = append(sorted, sum)
	}
	sort.Strings(sorted)

	listPath := filepath.Join("..", "scripts", "check-public-base-lines.txt")
	if os.Getenv("SMOLCORP_CHECK_PUBLIC_LINES") == "write" {
		text := "# The lines of the release base " + base + " that name the team that builds Smolcorp, each trimmed and\n" +
			"# listed as the SHA-256 of smolcorp-team-line: and its text. scripts/check-public.sh lets these through in a\n" +
			"# clone without the base. Remade by SMOLCORP_CHECK_PUBLIC_LINES=write go test ./desktop -run\n" +
			"# '^TestCheckPublicsBaseLinesAreTheReleaseBases$'.\n" + strings.Join(sorted, "\n") + "\n"
		if err := os.WriteFile(listPath, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			got = append(got, line)
		}
	}
	if strings.Join(got, "\n") != strings.Join(sorted, "\n") {
		t.Errorf("check-public-base-lines.txt lists %d lines, not the base's %d: remake it with SMOLCORP_CHECK_PUBLIC_LINES=write", len(got), len(sorted))
	}
}
