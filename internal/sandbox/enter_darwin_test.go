package sandbox

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SA-X3 on this Mac, with stand-ins only (owner Q5 (a)): the real `enter` (this test program) run by
// /usr/bin/sandbox-exec -f with the real profile (SA-M1's, made by PrepareProfile for a stand-in hire whose places
// are all in the test's temp folder). Its rows pass: the canary gives EPERM, the fences EPERM, the other server
// EPERM, its brief reads and its status writes. The core takes the report, and the desk's stand-in program holds no
// descriptor above 2, though the core passed two more, and no nonce, with the tmux guard off. A sandbox that isn't
// ours, one that lets the canary be read, or one that denies only the canary, is refused, and nothing runs.

func sandboxExecOrSkip(t *testing.T) string {
	t.Helper()
	const sandboxExec = "/usr/bin/sandbox-exec"
	if _, err := os.Stat(sandboxExec); err != nil {
		if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
			t.Fatalf("sandbox-exec missing: %v", err)
		}
		t.Skip("no sandbox-exec here")
	}
	return sandboxExec
}

func TestOnAMacEnterPassesInItsProfileAndTheDeskGetsNoDescriptor(t *testing.T) {
	sandboxExec := sandboxExecOrSkip(t)
	d := newStandInDesk(t)
	p := mustNew(t, Places{GOOS: "darwin", Home: d.home, Support: d.support, OwnID: "h1", Root: d.root,
		HiresFolder: filepath.Dir(d.root), SocketParent: filepath.Join(d.base, "k"), OwnSockets: filepath.Join(d.base, "k", "h1")})
	profile := filepath.Join(d.own, "sandbox.sb")
	if err := PrepareProfile(p, profile); err != nil {
		t.Fatal(err)
	}
	s := mustStart(t, d.checks)
	waited, stderr, out := runEnter(t, s, d, sandboxExec, "-f", profile)
	if waited != nil {
		t.Fatalf("the core refused enter in its profile: %v (%s)", waited, stderr)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the desk's program didn't run: %v (%s)", err, stderr)
	}
	var given struct {
		FDs   []int  `json:"fds"`
		Nonce bool   `json:"nonce"`
		Guard string `json:"guard"`
	}
	if err := json.Unmarshal(data, &given); err != nil || len(given.FDs) != 0 || given.Nonce || given.Guard != "off" {
		t.Errorf("the desk's program was given %s (%v)", data, err)
	}
	if _, err := os.Lstat(d.checks.Write[0]); err == nil {
		t.Error("a status file made by the check was left behind")
	}
	if data, _ := os.ReadFile(d.checks.NoWrite[0]); string(data) != `{"fence":1}` {
		t.Errorf("the guard file is %q", data)
	}
}

func TestOnAMacASandboxThatIsntOursIsRefused(t *testing.T) {
	sandboxExec := sandboxExecOrSkip(t)
	for name, profile := range map[string]string{
		"a sandbox that denies nothing": "(version 1)\n(allow default)\n",
		// The canary denied, and nothing else: the fences are open.
		"a sandbox that denies only the canary": "(version 1)\n(allow default)\n" +
			`(deny file-read-data (regex #"/sandbox-canary-[^/]*$"))` + "\n",
		// Everything denied in the support folder, the brief too: the positive control fails.
		"a sandbox that denies the whole support folder": "(version 1)\n(allow default)\n(deny file-read-data file-write* network-outbound (subpath \"SUPPORT\"))\n",
	} {
		d := newStandInDesk(t)
		file := filepath.Join(d.base, "other.sb")
		if err := os.WriteFile(file, []byte(strings.ReplaceAll(profile, "SUPPORT", d.support)), 0o600); err != nil {
			t.Fatal(err)
		}
		s := mustStart(t, d.checks)
		waited, stderr, out := runEnter(t, s, d, sandboxExec, "-f", file)
		var refused *Refused
		if !errors.As(waited, &refused) {
			t.Errorf("%s: the core took it: %v (%s)", name, waited, stderr)
			continue
		}
		t.Logf("%s: %v", name, refused)
		if _, err := os.Lstat(out); err == nil {
			t.Errorf("%s: the desk's program ran", name)
		}
		if data, _ := os.ReadFile(d.checks.NoWrite[0]); string(data) != `{"fence":1}` {
			t.Errorf("%s: the guard file is %q", name, data)
		}
	}
}
