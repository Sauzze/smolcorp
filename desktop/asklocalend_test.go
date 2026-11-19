package desktop

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

// FXRC2FIXB (RT126SWEEP4-P2 F1, the sweep's TestZZSweep4AskWithSmolcorpsEndOffLoopback): a request is this computer's
// only when both ends of its connection are on the loopback. A program here that binds 127.0.0.1 (or ::1) and connects
// to another of this computer's addresses (the Wi-Fi's, lo0's fe80::1), which the core listens on, isn't let ask for
// a key: on a Mac its holders couldn't be looked up, and its ask was listed as "may be a hire" instead of refused. In
// process only, on the real router: no socket is opened, on the loopback or off it.
func TestAnAskReachingSmolcorpOffTheLoopbackIsRefused(t *testing.T) {
	d := startDeck(t)
	d.summaryProviders.set(map[summaryProvider]providerState{})
	hostedByTheMacApp(d)
	rt := &router{deck: d}
	try := func(remote string, local *net.TCPAddr, host string) (int, string) {
		r := httptest.NewRequest(http.MethodPost, "http://"+host+terminalAskPath+"?kind=tool&folder=/Users/ada/Projects/app", nil)
		r.Host, r.RemoteAddr = host, remote
		r.Header.Set(setupHeader, "1")
		r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, local))
		w := httptest.NewRecorder()
		rt.ServeHTTP(w, r)
		return w.Code, strings.TrimSpace(w.Body.String())
	}
	for _, c := range []struct {
		remote, host string
		local        *net.TCPAddr
	}{
		{"127.0.0.1:1", "127.0.0.1:7710", &net.TCPAddr{IP: net.ParseIP("192.168.1.50"), Port: 7710}},
		{"[::1]:1", "[::1]:7710", &net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 7710, Zone: "lo0"}},
		{"127.0.0.1:1", "localhost:7710", &net.TCPAddr{IP: net.ParseIP("100.64.1.2"), Port: 7710}},
	} {
		if code, body := try(c.remote, c.local, c.host); code != http.StatusNotFound || body != "" {
			t.Errorf("asker %s, Smolcorp's end %s: %d %q, want 404 and nothing", c.remote, c.local, code, body)
		}
	}
	if waiting := d.asks.waiting(); len(waiting) != 0 {
		t.Fatalf("the window lists %v", waiting)
	}
	// The control: both ends on the loopback is this computer's, as before. On a Mac no program holds the stand-in
	// connection, so it is refused by the holders' look-up, as a desk's would be, not by the route.
	code, body := try("127.0.0.1:1", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7710}, "127.0.0.1:7710")
	if code == http.StatusNotFound || runtime.GOOS == "darwin" && (code != http.StatusForbidden || !strings.Contains(body, askNoAsker)) {
		t.Errorf("control, both ends on the loopback: %d %q", code, body)
	}
}
