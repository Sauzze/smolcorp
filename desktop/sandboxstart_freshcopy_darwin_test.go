package desktop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// FXFRESHCOPYCAP's premise, on this Mac, with stand-ins only: can a desk in its sandbox make a second name of a file
// freshCopy copies? The real profile (PrepareProfile) for a stand-in hire, run by /usr/bin/sandbox-exec. From inside,
// /bin/ln (link(2)) tries each file checkLinks would replace, into each folder the desk may write: its worktree, its
// own folder (a .md name) and its Claude folder. A plain write to each of those names goes through, so a refusal is
// the link's. Seatbelt's answer for each is logged; every one must be refused, and nothing made.
func TestOnAMacADeskCantMakeASecondNameOfAFileTheCheckWouldCopy(t *testing.T) {
	const sandboxExec = "/usr/bin/sandbox-exec"
	if _, err := os.Stat(sandboxExec); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec missing: %v", err)
		}
		t.Skip("no sandbox-exec here")
	}
	tr := newLinkTree(t)
	claude := filepath.Join(tr.own, deskClaudeName)
	for _, at := range []string{filepath.Join(tr.own, "settings.json"), filepath.Join(claude, "settings.json"), filepath.Join(claude, "settings.local.json")} {
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pol := tr.policy(t)
	profile := filepath.Join(tr.own, "sandbox.sb")
	if err := sandbox.PrepareProfile(pol, profile); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(tr.support, "hire-guard.json"), filepath.Join(tr.support, "nightshift.json"),
		filepath.Join(tr.own, "BRIEF.md"), filepath.Join(tr.own, "settings.json"), profile,
		filepath.Join(claude, "settings.json"), filepath.Join(claude, "settings.local.json"),
	}
	for _, f := range files {
		readV, _ := pol.Decide(f, sandbox.Read)
		writeV, _ := pol.Decide(f, sandbox.Write)
		if readV == sandbox.Deny || writeV != sandbox.Deny || !inside(tr.support, f) {
			t.Fatalf("%s isn't a file the check would replace (read %v, write %v)", f, readV, writeV)
		}
		info, _ := os.Stat(f)
		t.Logf("%s: %d bytes", strings.TrimPrefix(f, tr.base), info.Size())
	}
	run := func(script string, args ...string) (string, error) {
		cmd := disclaim.Cmd(exec.Command(sandboxExec, append([]string{"-f", profile, "/bin/sh", "-c", script, "sh"}, args...)...))
		cmd.Dir, cmd.Env = tr.root, []string{"HOME=" + tr.home, "PATH=/usr/bin:/bin"}
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		return strings.TrimSpace(out.String()), err
	}
	places := map[string]func(i int) string{
		"its worktree":      func(i int) string { return filepath.Join(tr.root, "second-"+string(rune('a'+i))) },
		"its own folder":    func(i int) string { return filepath.Join(tr.own, "second-"+string(rune('a'+i))+".md") },
		"its Claude folder": func(i int) string { return filepath.Join(claude, "second-"+string(rune('a'+i))) },
	}
	for place, name := range places {
		// The desk may write there: a plain file of that kind of name is made.
		probe := name(len(files))
		if out, err := run(`printf standin > "$1"`, probe); err != nil {
			t.Fatalf("a plain write in %s (%s) was refused: %v %s; the row proves nothing", place, probe, err, out)
		}
		for i, f := range files {
			second := name(i)
			out, err := run(`/bin/ln "$1" "$2"`, f, second)
			_, made := os.Lstat(second)
			t.Logf("link(2) %s → %s (%s): %v %q", strings.TrimPrefix(f, tr.base), strings.TrimPrefix(second, tr.base), place, err, out)
			if err == nil || made == nil || !strings.Contains(out, "Operation not permitted") {
				t.Errorf("the desk made a second name of %s in %s (%v %q)", f, place, err, out)
			}
			if info, _ := os.Lstat(f); fileLinks(nil, info) != 1 {
				t.Errorf("%s has %d names", f, fileLinks(nil, info))
			}
		}
	}
}
