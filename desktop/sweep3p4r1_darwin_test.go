package desktop

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Round 1 on RT126SWEEP3 part 4's F1 (FXSWEEP3P4R1), with the real look at the processes holding an ask's connection
// (askprogram_darwin.go). The asks are this test's own, from no sandbox, as the owner's terminal's are; no desk runs.

// askRig is a deck under a stand-in Mac app, served on loopback. An ask sent with closedFirst is held until the test
// has closed its end, then answered as ever, and what it was answered is handed back on answered.
type askRig struct {
	d           *Deck
	address     string
	closedFirst chan struct{}
	answered    chan *httptest.ResponseRecorder
}

func newAskRig(t *testing.T) *askRig {
	t.Helper()
	rig := &askRig{d: newMacTestDeck(t), closedFirst: make(chan struct{}), answered: make(chan *httptest.ResponseRecorder, 1)}
	routes := &router{deck: rig.d}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Row-Closed-First") != "1" {
			routes.ServeHTTP(w, r)
			return
		}
		<-rig.closedFirst
		answer := httptest.NewRecorder()
		routes.ServeHTTP(answer, r)
		rig.answered <- answer
	}))
	t.Cleanup(server.Close)
	rig.address = server.Listener.Addr().String()
	return rig
}

// ownersAsk is a tool's ask as `smolcorp status` sends it, from this test, which holds its connection till the answer.
func (rig *askRig) ownersAsk(t *testing.T, folder string) (int, map[string]any) {
	t.Helper()
	request, err := http.NewRequest("POST", "http://"+rig.address+terminalAskPath+"?kind=tool&folder="+url.QueryEscape(folder), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(setupHeader, "1")
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var said map[string]any
	_ = json.NewDecoder(response.Body).Decode(&said)
	return response.StatusCode, said
}

// closedAsk is an ask whose asker closes its end as soon as it has sent it, before Smolcorp looks: what Smolcorp
// answered (which nobody reads).
func (rig *askRig) closedAsk(t *testing.T, folder string) *httptest.ResponseRecorder {
	t.Helper()
	conn, err := net.Dial("tcp", rig.address)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(conn, "POST %s?kind=tool&folder=%s HTTP/1.1\r\nHost: %s\r\n%s: 1\r\nX-Row-Closed-First: 1\r\nContent-Length: 0\r\n\r\n",
		terminalAskPath, url.QueryEscape(folder), rig.address, setupHeader)
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case rig.closedFirst <- struct{}{}:
	case <-time.After(10 * time.Second):
		t.Fatal("the ask never reached Smolcorp")
	}
	return <-rig.answered
}

// N2: an ask whose asker closed its end at once (no program holds it, so none could read its ticket) is refused in
// plain words, and the window lists nothing.
func TestOnAMacAnAskWhoseAskerClosedAtOnceIsRefused(t *testing.T) {
	rig := newAskRig(t)
	answer := rig.closedAsk(t, "/work/app")
	if answer.Code != http.StatusForbidden || strings.TrimSpace(answer.Body.String()) != sweepNoAskerWords {
		t.Errorf("the closed ask: %d %q", answer.Code, answer.Body.String())
	}
	if waiting := rig.d.asks.waiting(); len(waiting) != 0 {
		t.Errorf("the window lists %v", waiting)
	}
}

// N2: a desk's flood of asks it closes at once takes no place in the queue, so the owner's own `smolcorp status` ask
// after it is taken (no 429), listed plainly.
func TestOnAMacAFloodOfClosedAsksLeavesTheOwnersAskTaken(t *testing.T) {
	rig := newAskRig(t)
	for i := range asksMost + 2 {
		if answer := rig.closedAsk(t, "/work/flood"+strconv.Itoa(i)); answer.Code == http.StatusOK {
			t.Errorf("closed ask %d was taken: %s", i, answer.Body.Bytes())
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

// N1: a process of the owner's that has ended and waits for its parent (a zombie: an unreaped child, as any app may
// leave) holds no files, so the owner's ask beside it is still read as outside any sandbox, not "may be a hire".
func TestOnAMacAnUnreapedChildLeavesTheOwnersAskOutside(t *testing.T) {
	rig := newAskRig(t)
	child := exec.Command("/usr/bin/true")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Wait() })
	pid := strconv.Itoa(child.Process.Pid)
	for deadline := time.Now().Add(10 * time.Second); ; {
		state, _ := exec.Command("/bin/ps", "-o", "stat=", "-p", pid).Output()
		if strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the child %s isn't a zombie: %q", pid, state)
		}
		time.Sleep(10 * time.Millisecond)
	}
	code, said := rig.ownersAsk(t, "/work/mine")
	if code != http.StatusOK || said["ticket"] == nil {
		t.Fatalf("the owner's ask: %d %v", code, said)
	}
	if waiting := rig.d.asks.waiting(); len(waiting) != 1 || waiting[0]["mayBeAHire"] != nil {
		t.Errorf("with an unreaped child, the window lists %v", waiting)
	}
	// Still a zombie when it was looked at.
	if state, _ := exec.Command("/bin/ps", "-o", "stat=", "-p", pid).Output(); !strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
		t.Errorf("the child %s was reaped meanwhile: %q", pid, state)
	}
}
