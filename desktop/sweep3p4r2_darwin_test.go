package desktop

import (
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// Round 2 on RT126SWEEP3 part 4's F1 (FXSWEEP3P4R2, the review's B1): a desk's process that has closed every file
// it had is read as holding nothing, so it neither flags the owner's asks "may be a hire" nor gets a closed ask taken.

// deskWithNoFiles starts a stand-in desk process under the hire's real profile that closes every file it had and
// sleeps: running, the owner's, with nothing open. It's killed, by its own pid, when the test ends.
func deskWithNoFiles(t *testing.T) {
	t.Helper()
	l := newHireProjectsLayout(t)
	profile, _ := l.profile(t, l.hire)
	cmd := disclaim.Cmd(exec.Command("/usr/bin/sandbox-exec", "-f", profile, "/bin/sh", "-c", "exec /bin/sleep 60 <&- >&- 2>&-"))
	cmd.Dir, cmd.Env = l.hireTree, gmRootEnv()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	pid := strconv.Itoa(cmd.Process.Pid)
	for deadline := time.Now().Add(10 * time.Second); ; {
		state, _ := exec.Command("/bin/ps", "-o", "stat=,comm=", "-p", pid).Output()
		if fields := strings.Fields(string(state)); len(fields) == 2 && !strings.HasPrefix(fields[0], "Z") && fields[1] == "/bin/sleep" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the stand-in desk %s isn't sleeping: %q", pid, state)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// B1: beside a desk's process with no files open, the owner's own ask is still read as outside any sandbox: taken,
// and listed with no "may be a hire".
func TestOnAMacADeskProcessWithNoFilesLeavesTheOwnersAskOutside(t *testing.T) {
	deskWithNoFiles(t)
	rig := newAskRig(t)
	code, said := rig.ownersAsk(t, "/work/mine")
	if code != http.StatusOK || said["ticket"] == nil {
		t.Fatalf("the owner's ask: %d %v", code, said)
	}
	if waiting := rig.d.asks.waiting(); len(waiting) != 1 || waiting[0]["folder"] != "/work/mine" || waiting[0]["mayBeAHire"] != nil {
		t.Errorf("beside a desk's process with no files, the window lists %v", waiting)
	}
}

// B1: with a desk's process with no files open, a flood of asks closed at once is still refused, in plain words,
// and the owner's own ask after it is taken (no 429), listed plainly.
func TestOnAMacADeskProcessWithNoFilesGetsNoClosedAskTaken(t *testing.T) {
	deskWithNoFiles(t)
	rig := newAskRig(t)
	for i := range asksMost + 2 {
		if answer := rig.closedAsk(t, "/work/flood"+strconv.Itoa(i)); answer.Code != http.StatusForbidden || strings.TrimSpace(answer.Body.String()) != sweepNoAskerWords {
			t.Errorf("closed ask %d: %d %q", i, answer.Code, answer.Body.String())
		}
	}
	code, said := rig.ownersAsk(t, "/work/mine")
	if code != http.StatusOK || said["ticket"] == nil {
		t.Fatalf("the owner's ask after the flood: %d %v", code, said)
	}
	if waiting := rig.d.asks.waiting(); len(waiting) != 1 || waiting[0]["folder"] != "/work/mine" || waiting[0]["mayBeAHire"] != nil {
		t.Errorf("the window lists %v", waiting)
	}
}
