package desktop

import (
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// processesMac stands in for the Mac app on the bridge's "processes" (HostBridge.swift): each call takes the next
// of answers, the last one kept; err is a bridge that couldn't be reached.
type processesMac struct {
	mu      sync.Mutex
	asked   []bridgeRequest
	answers []bridgeAnswer
	err     error
}

func (m *processesMac) call(request bridgeRequest) (bridgeAnswer, error) {
	if request.Action != "processes" {
		// Anything else a test deck asks of it: a terminal it can't drive.
		return bridgeAnswer{Error: "This terminal can't do that.", Unsupported: true}, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asked = append(m.asked, request)
	if m.err != nil {
		return bridgeAnswer{}, m.err
	}
	answer := m.answers[min(len(m.asked), len(m.answers))-1]
	return answer, nil
}

func (m *processesMac) requests() []bridgeRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.asked)
}

// withProcessesMac puts the stand-in Mac app in place for one test.
func withProcessesMac(t *testing.T, answers ...bridgeAnswer) *processesMac {
	t.Helper()
	saved := host
	t.Cleanup(func() { host = saved })
	mac := &processesMac{answers: answers}
	host = mac
	return mac
}

func macFound(pids ...int) bridgeAnswer {
	answer := bridgeAnswer{OK: true}
	for _, pid := range pids {
		answer.Processes = append(answer.Processes, bridgeProcess{PID: pid, Name: "claude", Why: "folder"})
	}
	return answer
}

// Without the Mac app (a bare core on a Mac) this engine can't read a process's folder: even a process given
// the folder with --add-dir isn't looked for, and the answer is "can't look", so Let go keeps the fence.
func TestT3OnAMacWithoutTheAppTheGoCoreCantLook(t *testing.T) {
	saved := host
	host = nil
	t.Cleanup(func() { host = saved })
	wt := t.TempDir()
	adder := exec.Command("/bin/sh", "-c", "/bin/sleep 30; :", "sh", "--add-dir", wt)
	if err := adder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adder.Process.Kill(); _ = adder.Wait() })
	if found, looked := processesWorkingIn(wt); looked || found != nil {
		t.Errorf("found %v, looked %v", found, looked)
	}
	if found, looked := processesHolding(wt, t3TakeOverSession); looked || found != nil {
		t.Errorf("found %v, looked %v", found, looked)
	}
}

// Under the Mac app, the core asks it (the bridge's read-only "processes"), with the folder and, for Take over
// and Hand back, the session; what it names is what's found. Its no, a bridge that can't be reached, or an
// answer longer than the app gives, or one naming a pid no process has (RM5c N4), are all "can't look": nothing
// is made up.
func TestT3OnAMacTheGoCoreAsksTheMacAppWhichProcessesWork(t *testing.T) {
	wt := "/Users/ada/repo-hires/ada-abcdef"
	mac := withProcessesMac(t, macFound(4242, 4343))
	if pids, looked := processesWorkingIn(wt); !looked || !slices.Equal(pids, []int{4242, 4343}) {
		t.Errorf("%v %v", pids, looked)
	}
	if pids, looked := processesHolding(wt, t3TakeOverSession); !looked || !slices.Equal(pids, []int{4242, 4343}) {
		t.Errorf("%v %v", pids, looked)
	}
	want := []bridgeRequest{{Action: "processes", Folder: wt}, {Action: "processes", Folder: wt, Session: t3TakeOverSession}}
	if got := mac.requests(); !reflect.DeepEqual(got, want) {
		t.Errorf("asked %+v", got)
	}

	withProcessesMac(t, macFound())
	if pids, looked := processesWorkingIn(wt); !looked || len(pids) != 0 {
		t.Errorf("none there: %v %v", pids, looked)
	}
	for _, malformed := range []int{0, -1} {
		withProcessesMac(t, macFound(4242, malformed))
		if pids, looked := processesHolding(wt, t3TakeOverSession); looked || pids != nil {
			t.Errorf("pid %d: %v %v", malformed, pids, looked)
		}
	}
	withProcessesMac(t, bridgeAnswer{Error: "This Mac couldn't list its processes."})
	if pids, looked := processesHolding(wt, t3TakeOverSession); looked || pids != nil {
		t.Errorf("the app's no: %v %v", pids, looked)
	}
	unreached := withProcessesMac(t)
	unreached.err = errors.New("the bridge's socket isn't the app's that started this engine")
	if pids, looked := processesWorkingIn(wt); looked || pids != nil {
		t.Errorf("no app to ask: %v %v", pids, looked)
	}
	many := make([]int, bridgeProcessesMost+1)
	for i := range many {
		many[i] = 1000 + i
	}
	withProcessesMac(t, macFound(many...))
	if pids, looked := processesWorkingIn(wt); looked || pids != nil {
		t.Errorf("more than the app answers: %v %v", pids, looked)
	}
}

// Let go and Take over on the Go core under the Mac app use these, not a stand-in of their own: the deck's own
// ways ask the Mac app.
func TestT3OnAMacLetGoAndTakeOverLookThroughTheMacApp(t *testing.T) {
	d := newTestDeck(t)
	wt := t.TempDir()
	mac := withProcessesMac(t, macFound(4242))
	if pids, looked := defaultT3Leaving().Working(wt); !looked || !slices.Equal(pids, []int{4242}) {
		t.Errorf("Let go: %v %v", pids, looked)
	}
	if pids, looked := d.t3HandingFor(Hire{ID: "abcdef123456", Worktree: wt}).Holding(wt, t3TakeOverSession); !looked || !slices.Equal(pids, []int{4242}) {
		t.Errorf("Take over: %v %v", pids, looked)
	}
	if got := mac.requests(); len(got) != 2 || got[0].Session != "" || got[1].Session != t3TakeOverSession {
		t.Errorf("asked %+v", got)
	}
}

// Let go of a hire in T3 Code on the Mac's Go engine: T3's project removed, then the Mac app asked until nothing
// works in the worktree; only then is the fence taken away. A Mac app that can't look keeps the fence.
func TestT3OnAMacLetGoWaitsForTheMacAppToSeeTheWorktreeFree(t *testing.T) {
	r := newT3Rig(t)
	r.fake()
	wt := r.fencedWorktree()
	fence := filepath.Join(wt, ".claude", "settings.local.json")
	leave, _ := r.leaving()
	leave.Working = processesWorkingIn

	withProcessesMac(t, bridgeAnswer{Error: "This Mac couldn't list its processes."})
	_, err := t3LetGo(r.t3Hire(wt), leave, func(t3Gone) { t.Error("let go without looking") })
	if refusalOf(t, err).Message != "T3 Code removed the project, but Smolcorp can't see which processes work in Ada's worktree on this Mac, so its fence was kept and it isn't let go yet." || !exists(fence) {
		t.Errorf("%v", err)
	}

	mac := withProcessesMac(t, macFound(4242), macFound())
	letGone := false
	if _, err := t3LetGo(r.t3Hire(wt), leave, func(t3Gone) { letGone = true }); err != nil || !letGone {
		t.Fatalf("not let go: %v", err)
	}
	if asked := mac.requests(); len(asked) != 2 || asked[0].Folder != wt || asked[1].Folder != wt {
		t.Errorf("asked %+v", asked)
	}
	if exists(fence) {
		t.Error("the fence stayed")
	}
}

// Hand back on the Mac's Go engine: while the Mac app sees anything holding the session, it's never resumed;
// once it sees nothing, it is.
func TestT3OnAMacHandBackResumesOnlyOnceTheMacAppSeesNothingHolding(t *testing.T) {
	k := newT3TakeOverRig(t)
	h := k.takenOver()
	ways := k.ways()
	ways.Holding = processesHolding
	held := bridgeAnswer{OK: true, Processes: []bridgeProcess{{PID: 777, Name: "claude", Why: "session"}}}
	mac := withProcessesMac(t, held)
	_, err := t3HandBack(h, ways)
	if refused := refusalOf(t, err); refused.Status != http.StatusConflict || !strings.Contains(refused.Message, "PID 777") {
		t.Errorf("%+v", refused)
	}
	if slices.ContainsFunc(k.log, func(s string) bool { return strings.HasPrefix(s, "resume") }) || !exists(k.fence()) {
		t.Errorf("resumed with the session held: %q", k.log)
	}
	for _, asked := range mac.requests() {
		if asked.Action != "processes" || asked.Folder != h.Worktree || asked.Session != t3TakeOverSession {
			t.Errorf("asked %+v", asked)
		}
	}

	withProcessesMac(t, macFound())
	k.log = nil
	if _, err := t3HandBack(k.saved, ways); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(k.log, "resume "+t3TakeOverSession) {
		t.Errorf("%q", k.log)
	}
}
