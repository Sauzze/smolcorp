//go:build unix

package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

// SA-L7 (proxy.go): the network through the core, II. git's ssh reaches out through a CONNECT (`sandbox connect`, the
// desk's GIT_SSH_COMMAND), and is refused the computer's own addresses as any request is: `ssh localhost`, Docker's
// TCP, a name that isn't a host's. The app's port is forwarded inside on its own number, through the proxy, and the
// app can tell those connections from a program of the owner's. Everything here is on this computer's loopback and in
// temp folders; names outside it are stand-ins dialled to local stand-in servers.

// sshServer is a stand-in SSH server out in the world: it speaks first, as an SSH server does, then sends back what
// it is sent.
func sshServer(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				io.WriteString(c, "SSH-2.0-standin\r\n")
				io.Copy(c, c)
				c.Close()
			}()
		}
	}()
	return l.Addr().String()
}

// sshNames are the stand-in names ssh is asked for: a forge out in the world, this computer's loopback by its name,
// and a container engine's bridge address by a name.
func sshNames(_ context.Context, host string) ([]netip.Addr, error) {
	switch host {
	case "github.test":
		return []netip.Addr{worldV4}, nil
	case "localhost":
		return []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}, nil
	case "docker.test":
		return []netip.Addr{netip.MustParseAddr("172.17.0.1")}, nil
	}
	return nil, errors.New("no such stand-in name")
}

// The computer's own places with docker0 on it (172.17.0.1/16, a container's bridge, whole).
func ownWithDocker() ([]netip.Prefix, error) {
	own, _ := ownAddrs()
	return append(own, netip.MustParsePrefix("172.17.0.0/16")), nil
}

// `sandbox connect` carries ssh to a host out in the world through the core's proxy, the server's first words (sent
// with the proxy's 200) included; and refuses, in the proxy's words, `ssh localhost`, Docker's TCP ports on loopback
// and on docker0, and the computer's own LAN address: nothing dialled, nothing written to ssh.
func TestSSHGoesThroughTheCoresProxyAndNeverToTheComputersOwn(t *testing.T) {
	d := &dialLog{}
	socket := serveProxy(t, &Proxy{own: ownWithDocker, resolve: sshNames, dial: d.dialTo(sshServer(t)), Log: t.Logf})
	fwd := forwarder(t, socket)

	var out, errs bytes.Buffer
	if code := connectThrough(fwd, []string{"github.test", "22"}, strings.NewReader("SSH-2.0-client\r\n"), &out, &errs); code != 0 {
		t.Fatalf("ssh to a forge out in the world: exit %d, %q", code, errs.String())
	}
	if out.String() != "SSH-2.0-standin\r\nSSH-2.0-client\r\n" {
		t.Errorf("ssh got %q", out.String())
	}
	if dials := d.all(); !slices.Equal(dials, []netip.AddrPort{netip.AddrPortFrom(worldV4, 22)}) {
		t.Errorf("dialled %v, not the forge's address", dials)
	}

	for _, to := range [][]string{
		{"localhost", "22"}, {"127.0.0.1", "22"}, {"::1", "22"}, {"127.0.0.1", "2375"}, {"127.0.0.1", "2376"},
		{"172.17.0.1", "2375"}, {"docker.test", "2376"}, {"172.17.0.2", "2375"}, {"192.0.2.99", "22"},
		{"100.100.100.100", "22"}, {"169.254.169.254", "80"},
	} {
		out.Reset()
		errs.Reset()
		code := connectThrough(fwd, to, strings.NewReader("SSH-2.0-client\r\n"), &out, &errs)
		if code != 1 || !strings.Contains(errs.String(), "403") || !strings.Contains(errs.String(), "this computer's") || out.Len() != 0 {
			t.Errorf("ssh %s port %s: exit %d, ssh got %q, said %q", to[0], to[1], code, out.String(), errs.String())
		}
	}
	if dials := d.all(); len(dials) != 1 {
		t.Errorf("the proxy dialled the computer's own addresses: %v", dials[1:])
	}
}

// A host that isn't a name or an address (an abstract socket's "@…", a path, a unix: address, an ssh option) or a
// port that isn't one is never asked of the core: nothing reaches the way out.
func TestSSHAsksForAHostAndAPortOnly(t *testing.T) {
	port, got := countingServer(t) // the way out, counting what reaches it
	at := "127.0.0.1:" + strconv.Itoa(port)
	for _, args := range [][]string{
		{"@smolcorp-standin", "22"}, {"@/tmp/.X11-unix/X0", "6000"}, {"/run/docker.sock", "22"}, {"unix:/run/docker.sock", "22"},
		{"-oProxyCommand=sh", "22"}, {"github.test:22", "22"}, {"github test", "22"}, {"github.test\r\nHost: x", "22"},
		{"", "22"}, {"github.test", "0"}, {"github.test", "65536"}, {"github.test", "ssh"}, {"github.test"}, {},
	} {
		var errs bytes.Buffer
		if code := connectThrough(at, args, strings.NewReader(""), io.Discard, &errs); code != 2 {
			t.Errorf("%q: exit %d (%s)", args, code, errs.String())
		}
	}
	time.Sleep(50 * time.Millisecond)
	if n := got.Load(); n != 0 {
		t.Errorf("the way out was asked %d times", n)
	}
	for _, host := range []string{"github.com", "ssh.dev.azure.com", "gitlab_ci.local", "192.0.2.10", "2001:db8::1"} {
		if !hostWord(host) {
			t.Errorf("%q isn't taken as a host", host)
		}
	}
}

// The app's own port is forwarded inside on its own number: a request to it in the view reaches the app through the
// core's proxy, and the app can tell that connection is a desk's, while it is open; a direct one isn't. Where the
// proxy refuses (not the app's port), nothing reaches the service there.
func TestTheAppsPortIsForwardedInsideAndKnownAsADesks(t *testing.T) {
	var marked, direct atomic.Value
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/direct" {
			direct.Store(ThroughADesksWayOut(r.RemoteAddr))
		} else {
			marked.Store(ThroughADesksWayOut(r.RemoteAddr))
		}
		io.WriteString(w, "the app: "+r.URL.Path)
	}))
	t.Cleanup(app.Close)
	appURL, _ := url.Parse(app.URL)
	appPort, _ := strconv.Atoi(appURL.Port())
	socket := serveProxy(t, &Proxy{AppPort: appPort, own: ownAddrs})
	inView, err := net.Listen("tcp", "127.0.0.1:0") // the view's 127.0.0.1:<the app's port>, its own loopback
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inView.Close() })
	go ForwardTo(inView, socket, "127.0.0.1:"+appURL.Port())

	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
	for _, path := range []string{"/hook", "/direct"} {
		at := inView.Addr().String()
		if path == "/direct" {
			at = appURL.Host
		}
		resp, err := client.Get("http://" + at + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != "the app: "+path {
			t.Errorf("%s: the app said %q", path, body)
		}
	}
	if marked.Load() != true || direct.Load() != false {
		t.Errorf("a desk's connection known as one: %v; a direct one: %v", marked.Load(), direct.Load())
	}
	// A client that stops writing once it has asked still gets its answer: its end is passed on as an end of writing.
	half, err := net.Dial("tcp", inView.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	half.SetDeadline(time.Now().Add(10 * time.Second))
	io.WriteString(half, "GET /half HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n")
	half.(*net.TCPConn).CloseWrite()
	if back, _ := io.ReadAll(half); !strings.HasSuffix(string(back), "the app: /half") {
		t.Errorf("after the client's end of writing came %q", back)
	}
	half.Close()
	// Its mark goes with it.
	deadline := time.Now().Add(5 * time.Second)
	for marks() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := marks(); n != 0 {
		t.Errorf("%d marks left after the connections closed", n)
	}

	// Through a proxy whose app port is another, the service there is never reached.
	other, got := countingServer(t)
	refusing := serveProxy(t, &Proxy{AppPort: appPort, own: ownAddrs})
	inView2, _ := net.Listen("tcp", "127.0.0.1:0")
	t.Cleanup(func() { inView2.Close() })
	go ForwardTo(inView2, refusing, "127.0.0.1:"+strconv.Itoa(other))
	c, err := net.Dial("tcp", inView2.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c.SetDeadline(time.Now().Add(10 * time.Second))
	if b, _ := io.ReadAll(c); len(b) != 0 {
		t.Errorf("through a refused forward came %q", b)
	}
	c.Close()
	if n := got.Load(); n != 0 {
		t.Errorf("the service at another loopback port was reached %d times", n)
	}
}

// marks counts the connections marked as a desk's at the app.
func marks() int {
	n := 0
	atTheApp.Range(func(any, any) bool { n++; return true })
	return n
}

// A mark is the connection's own: once it closes, a later connection that took the same address keeps its own mark.
func TestAMarkAtTheAppIsTheConnectionsOwn(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	first := &appConn{Conn: a, key: "127.0.0.1:50000"}
	atTheApp.Store(first.key, first)
	second := &appConn{Conn: a, key: "127.0.0.1:50000"}
	atTheApp.Store(second.key, second) // the address taken again
	first.Close()
	if !ThroughADesksWayOut("127.0.0.1:50000") {
		t.Error("the first connection's close took the second's mark")
	}
	second.Close()
	if ThroughADesksWayOut("127.0.0.1:50000") {
		t.Error("a mark outlived its connection")
	}
}

// The app's port in the proxy row: the core gives it with a way out only, `enter` takes it once, as a port that isn't
// the forwarder's own, and the forwarder is asked to listen on it too; the row fails when it doesn't.
func TestTheAppsPortInTheProxyRow(t *testing.T) {
	d := newStandInDesk(t)
	socket := serveProxy(t, &Proxy{own: ownAddrs})
	checks := d.checks
	checks.Proxy, checks.AppPort = socket, 7799
	s := mustStart(t, checks)
	words := s.Words("/bin/echo")[2:]
	e, err := parseEnter(words)
	if err != nil || e.appPort != 7799 || !slices.Contains(words, "--app-port") {
		t.Fatalf("the words %q: %+v %v", words, e, err)
	}
	var asked []string
	sys := holding("linux", s, d)
	sys.forwardFn = func(at, addr, app string) (string, error) {
		asked = append(asked, at, addr, app)
		return forwarder(t, at), nil
	}
	if results, held := runRows("linux", sys, e); !held ||
		!slices.Equal(asked, []string{socket, "127.0.0.1:" + strconv.Itoa(ForwardPort), "127.0.0.1:7799"}) {
		t.Fatalf("held %v, %q, the forwarder asked %q", held, results, asked)
	}
	sys.forwardFn = func(_, _, app string) (string, error) {
		return "", fmt.Errorf("no listener at %s: %w", app, syscall.EADDRINUSE)
	}
	if results, held := runRows("linux", sys, e); held || results[len(results)-2] == passed {
		t.Errorf("the forwarder didn't listen on the app's port, and the row passed: %q", results)
	}

	for _, c := range []Checks{{AppPort: 7799}, {Proxy: socket, AppPort: ForwardPort}, {Proxy: socket, AppPort: 70000}, {Proxy: socket, AppPort: -1}} {
		bad := d.checks
		bad.Proxy, bad.AppPort = c.Proxy, c.AppPort
		if s, err := NewStart(bad); err == nil {
			s.Close()
			s.Canary.Remove()
			t.Errorf("a start with the app's port %d and the way out %q was made", c.AppPort, c.Proxy)
		}
	}
	plain := mustStart(t, d.checks)
	base := plain.Words("/bin/echo")[2:]
	dash := slices.Index(base, "--")
	withProxy := slices.Concat(base[:dash], []string{"--proxy", socket}, base[dash:])
	dash2 := slices.Index(withProxy, "--")
	for _, extra := range [][]string{{"--app-port", "0"}, {"--app-port", "70000"}, {"--app-port", "x"},
		{"--app-port", strconv.Itoa(ForwardPort)}, {"--app-port", "7799", "--app-port", "7798"}} {
		if _, err := parseEnter(slices.Concat(withProxy[:dash2], extra, withProxy[dash2:])); err == nil {
			t.Errorf("%q was taken", extra)
		}
	}
	if _, err := parseEnter(slices.Concat(base[:dash], []string{"--app-port", "7799"}, base[dash:])); err == nil {
		t.Error("the app's port with no proxy row was taken")
	}
}

// git's ssh is Smolcorp's `sandbox connect`, through two shells (git's, then ssh's for its ProxyCommand, after its
// %-tokens): a program path with a space, a quote, a '%' and a '$' arrives whole, and ssh gets its own words and
// git's, with no shared connection. Stand-ins for ssh and Smolcorp's program; nothing connects anywhere.
func TestGitsSSHRunsSmolcorpsConnectThroughBothShells(t *testing.T) {
	dir := shortTemp(t)
	bin, odd := filepath.Join(dir, "bin"), filepath.Join(dir, "a b'c%d $HOME")
	for _, folder := range []string{bin, odd} {
		if err := os.MkdirAll(folder, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	saysItsWords := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$OUT\"\n"
	self := filepath.Join(odd, "smolcorp")
	for _, program := range []string{filepath.Join(bin, "ssh"), self} {
		if err := os.WriteFile(program, []byte(saysItsWords), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	run := func(out string, words ...string) []string {
		t.Helper()
		cmd := disclaim.Cmd(exec.Command("/bin/sh", words...))
		cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "OUT=" + out, "HOME=" + dir}
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%q: %v %s", words, err, b)
		}
		said, _ := os.ReadFile(out)
		return strings.Split(strings.TrimSuffix(string(said), "\n"), "\n")
	}
	ssh := sshCommand(self)
	// git runs GIT_SSH_COMMAND with a shell, the command and its words after it.
	sshWords := run(filepath.Join(dir, "ssh.out"), "-c", ssh+` "$@"`, ssh, "git@github.test", "git-upload-pack 'owner/app.git'")
	if len(sshWords) != 6 || sshWords[0] != "-o" || !strings.HasPrefix(sshWords[1], "ProxyCommand=") ||
		!slices.Equal(sshWords[2:], []string{"-o", "ControlPath=none", "git@github.test", "git-upload-pack 'owner/app.git'"}) {
		t.Fatalf("ssh got %q", sshWords)
	}
	// ssh expands its %-tokens in the ProxyCommand, then runs it with a shell.
	proxy, err := sshTokens(strings.TrimPrefix(sshWords[1], "ProxyCommand="), "github.test", "22")
	if err != nil {
		t.Fatal(err)
	}
	if got := run(filepath.Join(dir, "self.out"), "-c", "exec "+proxy); !slices.Equal(got, []string{"sandbox", "connect", "github.test", "22"}) {
		t.Errorf("Smolcorp's program got %q", got)
	}
	for _, bad := range []string{"smolcorp", "", "/usr/bin/smol\ncorp"} {
		if got := sshCommand(bad); got != "" {
			t.Errorf("%q: %q", bad, got)
		}
	}
}

// sshTokens is ssh's own expansion of a ProxyCommand's %-tokens: %h, %p and %%; any other is an error, as ssh's.
func sshTokens(s, host, port string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		if i++; i == len(s) {
			return "", errors.New("a % at the end")
		}
		switch s[i] {
		case 'h':
			b.WriteString(host)
		case 'p':
			b.WriteString(port)
		case '%':
			b.WriteByte('%')
		default:
			return "", fmt.Errorf("unknown token %%%c", s[i])
		}
	}
	return b.String(), nil
}
