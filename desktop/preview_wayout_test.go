//go:build !windows

package desktop

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// A connection a Linux desk's way out made to the app (SA-L7, RT126SAL6 N1) names no process for a launcher's
// preview: the system's tables would name the core, which holds the proxy's end, and the core's processes are every
// desk's. A direct connection still names its own process (this test's, on a Mac and on Linux).
func TestALauncherThroughADesksWayOutNamesNoProcess(t *testing.T) {
	pids := make(chan [2]int, 2)
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tab := &launcherTab{}
		if local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
			tab.connected(r.RemoteAddr, local.String())
		}
		way := 0
		if sandbox.ThroughADesksWayOut(r.RemoteAddr) {
			way = 1
		}
		pids <- [2]int{way, tab.process()}
		io.WriteString(w, "ok")
	}))
	t.Cleanup(app.Close)
	appPort := app.Listener.Addr().(*net.TCPAddr).Port

	dir, err := os.MkdirTemp("", "sx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	l, err := sandbox.ListenProxy(filepath.Join(dir, sandbox.ProxySocketName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go (&sandbox.Proxy{AppPort: appPort, Log: t.Logf}).Serve(l)

	c, err := net.Dial("unix", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(20 * time.Second))
	in := bufio.NewReader(c)
	at := app.Listener.Addr().String()
	io.WriteString(c, "CONNECT "+at+" HTTP/1.1\r\nHost: "+at+"\r\n\r\n")
	if resp, err := http.ReadResponse(in, &http.Request{Method: http.MethodConnect}); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("the way out to the app's port: %v %v", resp, err)
	}
	io.WriteString(c, "GET /launcher HTTP/1.1\r\nHost: "+at+"\r\n\r\n")
	if got := <-pids; got != [2]int{1, 0} {
		t.Errorf("through a desk's way out: known as one %v, its process %d, not none", got[0] == 1, got[1])
	}

	resp, err := http.Get(app.URL + "/launcher")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	got := <-pids
	if got[0] != 0 {
		t.Error("a direct connection is known as a desk's")
	}
	if (runtime.GOOS == "darwin" || runtime.GOOS == "linux") && got[1] != os.Getpid() {
		t.Errorf("a direct connection names %d, not this test's own process %d", got[1], os.Getpid())
	}
}
