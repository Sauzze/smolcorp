package desktop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// On this Mac, with stand-ins only (owner Q5 (a)): the real profile (M1's PrepareProfile) for a stand-in hire, run
// by /usr/bin/sandbox-exec. A hard link made before the start, as the hire's status file (which it may write, and the
// check looks at), to the stand-in guard file lets a write through: the sandbox checks the path, and the path is the
// hire's own. Once checkLinks has put a fresh copy in the guard file's place, the same write lands in the dead copy.
// Only the stand-in's contents are compared. (A name in its worktree, where the check doesn't look, refuses the start
// instead: X12LINK1.)
func TestOnAMacALinkMadeBeforeTheStartWritesNothingOnceReplaced(t *testing.T) {
	const sandboxExec = "/usr/bin/sandbox-exec"
	if _, err := os.Stat(sandboxExec); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec missing: %v", err)
		}
		t.Skip("no sandbox-exec here")
	}
	tr := newLinkTree(t)
	pol := tr.policy(t)
	profile := filepath.Join(tr.own, "sandbox.sb")
	if err := sandbox.PrepareProfile(pol, profile); err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(tr.support, "hire-guard.json")
	other := tr.linkMet(t, guard)
	write := func() {
		t.Helper()
		cmd := disclaim.Cmd(exec.Command(sandboxExec, "-f", profile, "/bin/sh", "-c", `printf standin > "$1"`, "sh", other))
		cmd.Dir, cmd.Env = tr.root, []string{"HOME=" + tr.home, "PATH=/usr/bin:/bin"}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("the stand-in's write under the profile: %v %s", err, stderr.Bytes())
		}
	}
	// The profile holds the guard file's own path...
	direct := disclaim.Cmd(exec.Command(sandboxExec, "-f", profile, "/bin/sh", "-c", `printf standin > "$1"`, "sh", guard))
	direct.Env = []string{"PATH=/usr/bin:/bin"}
	if err := direct.Run(); err == nil {
		t.Fatal("the profile let the guard file be written by its own path")
	}
	// ...but not its second name.
	write()
	if data, _ := os.ReadFile(guard); string(data) != "standin" {
		t.Fatalf("the write through the second name didn't reach the guard file (%q): the row proves nothing", data)
	}
	if err := os.WriteFile(guard, []byte(`{"fence":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	replaced, err := checkLinks(pol, tr.places())
	if err != nil || len(replaced) != 1 || replaced[0] != guard {
		t.Fatalf("checkLinks: %v, replaced %v", err, replaced)
	}
	write()
	if data, _ := os.ReadFile(guard); string(data) != `{"fence":1}` {
		t.Errorf("after the check, the write through the second name reached the guard file: %q", data)
	}
	if data, _ := os.ReadFile(other); string(data) != "standin" {
		t.Errorf("the dead copy holds %q", data)
	}
}
