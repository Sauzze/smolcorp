//go:build unix

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The team's names, put together as the row runs, so this file names none of them (as a test's fake key is).
var checkPublicTeamNames = []string{"SIER" + "RA", "VIC" + "TOR", "JULI" + "ETT", "WHIS" + "KEY", "RO" + "MEO", "AL" + "PHA"}

// scripts/check-public.sh refuses a line added since the release base that names the team, as a whole word in
// capitals, and nothing else: a line the base already had (kept, or moved to another file), and the same letters in
// small letters, in a longer word or in a name joined by an underscore all pass. A clone without the base says so
// and checks the rest. The real script, copied into a stand-in repo whose base is given in SMOLCORP_RELEASE_BASE.
func TestCheckPublicRefusesATeamNameInALineAddedSinceTheReleaseBase(t *testing.T) {
	for _, tool := range []string{"bash", "git", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	script, err := os.ReadFile(filepath.Join("..", "scripts", "check-public.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	env := []string{"HOME=" + dir, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); name != "HOME" && !strings.HasPrefix(name, "GIT_") && name != "SMOLCORP_RELEASE_BASE" {
			env = append(env, kv)
		}
	}
	git := func(args ...string) string {
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
	write := func(name, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	check := func(base string) (bool, string) {
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
	// Without the base here, the rest is still checked and the run says what it couldn't check.
	missing := strings.Repeat("e", 40)
	if ok, out := check(missing); !ok || !strings.Contains(out, "the release base "+missing+" is not in this clone") {
		t.Errorf("no base: want a pass with a note, got:\n%s", out)
	}
}
