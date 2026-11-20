package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// RT126SWEEP3 part 4's rows A2 and B (FXSWEEP3P4), under the real profile, with stand-ins only (newHireProjectsLayout).
// As in sweep3p4_test.go, they use only what the tree had before the fix, so they fail there.

// The words a program in a sandbox is told when it asks (askprogram.go's askFromASandbox).
const sweepAskFromASandbox = "This program runs in a sandbox, as Smolcorp's hires do, so Smolcorp doesn't take its ask. " +
	"Run it in your own terminal."

// Row A2 (B1): the GM's token tries both routes that save the folder for hires, and is refused; the next start of
// the hire still holds the owner's projects folder, so the hire plants no hook in the sibling repo lib and writes
// none of its files, and the owner's git checkout there, outside any sandbox, runs nothing a hire planted.
func TestTheNextStartStillHoldsTheOwnersProjectsAfterTheGMsMove(t *testing.T) {
	l := newHireProjectsLayout(t)
	elsewhere := homeDir + "/Elsewhere"
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	held := hireProjectsFolder()
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	gm := d.newGMToken()
	for _, move := range []string{
		"/setup/hire-folder?create=1&folder=" + url.QueryEscape(homeDir+"/NotYet"),
		"/setup/hire-folder?folder=" + url.QueryEscape(elsewhere),
		"/setup/hiring?folder=" + url.QueryEscape(elsewhere),
	} {
		got := serve(t, d, call{method: "POST", target: move, host: "localhost:7710", from: thisComputer, withoutSetupKey: true,
			headers: map[string]string{gmTokenHeader: gm, setupHeader: "1"}})
		if got.Code != http.StatusForbidden {
			t.Errorf("the GM's token, POST %s: %d %q", move, got.Code, got.Body)
		}
	}
	stopDeck(d)
	if got := hireProjectsFolder(); got != held || held != l.projects {
		t.Fatalf("a start holds %q, want the owner's %q (held %q before)", got, l.projects, held)
	}
	profile, _ := l.profile(t, l.hire) // the hire's next start, or a new hire the GM makes
	const no = "PermissionError"
	l.run(t, profile, l.hireTree, []probeStep{{"hook", l.lib + "/.git/hooks/post-checkout", no}, {"write", l.lib + "/README", no}})
	standInOwnersGit(t, l.lib, "checkout", "-q", "develop")
	if _, err := os.Lstat(l.planted); !errors.Is(err, os.ErrNotExist) {
		t.Error("the owner's checkout in lib ran a hook a hire planted")
	}
}

// Row B (F1): a stand-in desk under the hire's real profile asks for a tool's key over loopback, with a folder of its
// choosing, and is refused in plain words: the window lists nothing. The same stand-in outside any sandbox, as the
// owner's terminal runs it, is listed as before, with no "may be a hire".
func TestOnAMacADeskInItsSandboxCantAskForAKey(t *testing.T) {
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire)
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	hostedByTheMacApp(d)
	srv := httptest.NewServer(&router{deck: d})
	defer srv.Close()
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	const ask = `import http.client, json, sys
c = http.client.HTTPConnection("127.0.0.1", int(sys.argv[1]), timeout=10)
c.request("POST", "/terminal/ask?kind=tool&folder=" + sys.argv[2], headers={"X-AgentDeck-Setup": "1"})
r = c.getresponse(); b = r.read()
try:
    said = json.loads(b or b"{}")
except ValueError:
    said = {"text": b.decode("utf-8", "replace").strip()}
print(json.dumps({"status": r.status, "text": said.get("text", ""), "ticket": bool(said.get("ticket"))}))
`
	run := func(sandboxed bool) map[string]any {
		t.Helper()
		args := []string{deskPython(t), "-I", "-c", ask, port, url.QueryEscape(l.app)}
		if sandboxed {
			args = append([]string{"/usr/bin/sandbox-exec", "-f", profile}, args...)
		}
		cmd := disclaim.Cmd(exec.Command(args[0], args[1:]...))
		cmd.Dir, cmd.Env = l.hireTree, gmRootEnv()
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("the stand-in: %v %s", err, stderr.Bytes())
		}
		var said map[string]any
		if json.Unmarshal(out.Bytes(), &said) != nil {
			t.Fatalf("the stand-in said %q", out.Bytes())
		}
		return said
	}
	said := run(true)
	if said["status"] != float64(http.StatusForbidden) || said["ticket"] != false || said["text"] != sweepAskFromASandbox {
		t.Errorf("the desk's ask under its profile: %v", said)
	}
	if waiting := d.asks.waiting(); len(waiting) != 0 {
		t.Fatalf("the window lists %v", waiting)
	}
	said = run(false)
	if said["status"] != float64(http.StatusOK) || said["ticket"] != true {
		t.Fatalf("the same stand-in outside any sandbox: %v", said)
	}
	if waiting := d.asks.waiting(); len(waiting) != 1 || waiting[0]["mayBeAHire"] != nil {
		t.Errorf("the window lists %v", waiting)
	}
}
