package desktop

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// SA-M12: port 22 and the local databases. Every check here dials stand-ins of the test's own on 127.0.0.1, ports
// 7780 to 7799, never the owner's sshd or databases: init below points the checks away from port 22 and the
// databases' places before any test runs, and the reach probe at the closed 7799, so one that forgets to give its own
// stand-ins dials nothing real. (7780–7799
// is this file's own part of the test ports: SA-M2's profile tests in internal/sandbox, which may run at the same time,
// take theirs from 7750 up.) The owner's login name the checks try is a made-up one, so the real one is never sent to
// a stand-in or written in a failure.

// realDatabasePlaces is the checks' own list of places, kept before init swaps in none.
var realDatabasePlaces = databasePlaces

// testOwnerLogin is the owner's login name as the checks see it here.
const testOwnerLogin = "deskowner"

func init() {
	remoteLoginPort = 7799
	databasePlaces = func(string, []string) []dbPlace { return nil }
	ownAddresses = func() []string { return []string{"127.0.0.1"} }
	reachPort = func() int { return 7799 }
	ownerLogin = func() string { return testOwnerLogin }
}

// checkStandIn listens on 127.0.0.1 at the first free port of 7780–7798 and serves each connection with serve until
// the test ends. 7799 is left as the closed port init points Remote Login at.
func checkStandIn(t *testing.T, serve func(net.Conn)) int {
	t.Helper()
	for port := 7780; port < 7799; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		t.Cleanup(func() { l.Close() })
		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				go func() {
					defer conn.Close()
					_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
					serve(conn)
				}()
			}
		}()
		return port
	}
	t.Fatal("no free stand-in port in 7780–7798")
	return 0
}

// closedPort is a port of 7780–7798 nothing listens on as the test starts, looked for from the top down, so
// the stand-ins (from the bottom up) don't take it.
func closedPort(t *testing.T) int {
	t.Helper()
	for port := 7798; port >= 7780; port-- {
		if l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			l.Close()
			return port
		}
	}
	t.Fatal("no closed stand-in port in 7780–7798")
	return 0
}

// quickChecks makes the checks' waits short, for the stand-ins that never answer.
func quickChecks(t *testing.T) {
	wait, try := checkWait, checkTryWait
	checkWait, checkTryWait = 2*time.Second, 300*time.Millisecond
	t.Cleanup(func() { checkWait, checkTryWait = wait, try })
}

// onlyPlaces has the database check try places, and nothing else.
func onlyPlaces(t *testing.T, places ...dbPlace) {
	saved := databasePlaces
	databasePlaces = func(string, []string) []dbPlace { return places }
	t.Cleanup(func() { databasePlaces = saved })
}

// freshChecks forgets what earlier tests' starts found, and has Remote Login dialed again at each look.
func freshChecks(t *testing.T) {
	reset := func() {
		checked.Lock()
		clear(checked.desks)
		checked.databaseShut, checked.remoteLogin, checked.remoteLoginAt, checked.remoteLoginNew = false, false, time.Time{}, 0
		checked.Unlock()
	}
	reset()
	t.Cleanup(func() {
		reset()
		checked.Lock()
		checked.remoteLoginNew = 30 * time.Second
		checked.Unlock()
	})
}

// standInTLS is a server TLS config with a fresh self-signed certificate.
func standInTLS(t *testing.T) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "stand-in"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
}

// A Postgres stand-in: it answers SSLRequest with S (and TLS) when it has a certificate, else N, and each startup
// with what answer gives for its user and database.
type pgStandIn struct {
	cert   *tls.Config
	answer func(user, database string, overTLS bool) []byte
	mu     sync.Mutex
	starts []string
}

func (s *pgStandIn) serve(conn net.Conn) {
	overTLS := false
	for {
		head := make([]byte, 8)
		if _, err := io.ReadFull(conn, head); err != nil {
			return
		}
		size, code := binary.BigEndian.Uint32(head), binary.BigEndian.Uint32(head[4:])
		if code == 80877103 {
			if s.cert == nil {
				conn.Write([]byte("N"))
				continue
			}
			conn.Write([]byte("S"))
			tc := tls.Server(conn, s.cert)
			if tc.Handshake() != nil {
				return
			}
			conn, overTLS = tc, true
			continue
		}
		rest := make([]byte, size-8)
		if _, err := io.ReadFull(conn, rest); err != nil {
			return
		}
		params := strings.Split(string(rest), "\x00")
		got := map[string]string{}
		for i := 0; i+1 < len(params); i += 2 {
			got[params[i]] = params[i+1]
		}
		s.mu.Lock()
		s.starts = append(s.starts, fmt.Sprintf("%s/%s/%v", got["user"], got["database"], overTLS))
		s.mu.Unlock()
		conn.Write(s.answer(got["user"], got["database"], overTLS))
		io.Copy(io.Discard, conn)
		return
	}
}

func (s *pgStandIn) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.starts)
}

func pgMessage(kind byte, body []byte) []byte {
	return append(append([]byte{kind}, binary.BigEndian.AppendUint32(nil, uint32(len(body)+4))...), body...)
}

func pgAuth(code uint32) []byte { return pgMessage('R', binary.BigEndian.AppendUint32(nil, code)) }

func pgError(state string) []byte {
	return pgMessage('E', []byte("SFATAL\x00C"+state+"\x00Mstand-in\x00\x00"))
}

// A MySQL stand-in: a protocol 10 greeting with caps and plugin, TLS after an SSLRequest when it has a certificate,
// then answer for each round (nil closes); the client's packet after each answer is read, but after a fast auth's
// 0x01 0x03, where the server goes on by itself.
type mysqlStandIn struct {
	caps     uint32
	cert     *tls.Config
	plugin   string
	greeting []byte
	answer   func(user string, overTLS bool, round int) []byte
	mu       sync.Mutex
	switched [][]byte // what the client answered each auth switch with
}

func mysqlPacket(seq byte, p []byte) []byte {
	return append([]byte{byte(len(p)), byte(len(p) >> 8), byte(len(p) >> 16), seq}, p...)
}

func (s *mysqlStandIn) greet() []byte {
	if s.greeting != nil {
		return s.greeting
	}
	g := append([]byte{10}, "8.4.0-stand-in\x00"...)
	g = append(g, 1, 0, 0, 0)
	g = append(g, "abcdefgh"...)
	g = append(g, 0)
	g = binary.LittleEndian.AppendUint16(g, uint16(s.caps))
	g = append(g, 33, 2, 0)
	g = binary.LittleEndian.AppendUint16(g, uint16(s.caps>>16))
	g = append(g, 21)
	g = append(g, make([]byte, 10)...)
	g = append(g, "ijklmnopqrst\x00"...)
	return append(append(g, s.plugin...), 0)
}

func (s *mysqlStandIn) serve(conn net.Conn) {
	read := func() (byte, []byte, bool) { return mysqlRead(conn) }
	conn.Write(mysqlPacket(0, s.greet()))
	seq, p, ok := read()
	if !ok {
		return
	}
	overTLS := false
	if len(p) == 32 && binary.LittleEndian.Uint32(p)&mysqlSSL != 0 && s.cert != nil {
		tc := tls.Server(conn, s.cert)
		if tc.Handshake() != nil {
			return
		}
		conn, overTLS = tc, true
		if seq, p, ok = read(); !ok {
			return
		}
	}
	user, _, _ := strings.Cut(string(p[32:]), "\x00")
	for round := range 4 {
		reply := s.answer(user, overTLS, round)
		if reply == nil {
			return
		}
		seq++
		conn.Write(mysqlPacket(seq, reply))
		switch {
		case reply[0] == 0x00 || reply[0] == 0xFF:
			io.Copy(io.Discard, conn)
			return
		case reply[0] == 0x01 && len(reply) > 1 && reply[1] == 3:
			continue
		}
		var said []byte
		if seq, said, ok = read(); !ok {
			return
		}
		if reply[0] == 0xFE {
			s.mu.Lock()
			s.switched = append(s.switched, said)
			s.mu.Unlock()
		}
	}
}

func mysqlErr(code uint16) []byte {
	return append(binary.LittleEndian.AppendUint16([]byte{0xFF}, code), "#28000stand-in"...)
}

var mysqlOK = []byte{0x00, 0, 0, 2, 0, 0, 0}

const mysqlCaps = mysqlLongPassword | mysqlProtocol41 | mysqlSecureConnection | mysqlPluginAuth

// A check finds a local database that lets a login in with no password, whichever answer a real Postgres gives:
// trust for anyone, trust for the owner's own login only (Homebrew's superuser), for one database only (sameuser),
// or only over TLS (hostssl); and one that asks for a password, by any method, or refuses the login, passes.
// Anything it can't read (a server that never answers, or answers in another protocol, or is starting up) can't be
// told, and fails the opt-in as a login that got in does.
func TestAPasswordLessPostgresLoginIsFoundAndOnlyAFailedOnePasses(t *testing.T) {
	quickChecks(t)
	owner := ownerLogin()
	ok, shut := pgAuth(0), pgError("28000")
	cases := []struct {
		name    string
		cert    bool
		answer  func(user, database string, overTLS bool) []byte
		letIn   string // the login found, "" for none
		cantTel bool
	}{
		{"trust for anyone", false, func(string, string, bool) []byte { return ok }, "postgres", false},
		{"md5", false, func(string, string, bool) []byte { return pgAuth(5) }, "", false},
		{"SCRAM", false, func(string, string, bool) []byte { return pgAuth(10) }, "", false},
		{"cleartext", false, func(string, string, bool) []byte { return pgAuth(3) }, "", false},
		{"no such role", false, func(string, string, bool) []byte { return shut }, "", false},
		{"a notice first, then md5", false, func(string, string, bool) []byte { return append(pgMessage('N', []byte("\x00")), pgAuth(5)...) }, "", false},
		{"Kerberos V5", false, func(string, string, bool) []byte { return pgAuth(2) }, "", false},
		{"GSSAPI", false, func(string, string, bool) []byte { return pgAuth(7) }, "", false},
		{"SSPI", false, func(string, string, bool) []byte { return pgAuth(9) }, "", false},
		{"trust for the owner's own login", false, func(user, _ string, _ bool) []byte {
			return map[bool][]byte{true: ok, false: shut}[user == owner]
		}, owner, false},
		{"the owner's login, to postgres only", false, func(user, database string, _ bool) []byte {
			return map[bool][]byte{true: ok, false: shut}[user == owner && database == "postgres"]
		}, owner, false},
		{"trust over TLS only", true, func(_, _ string, overTLS bool) []byte {
			return map[bool][]byte{true: ok, false: shut}[overTLS]
		}, "postgres", false},
		{"TLS offered, md5 both ways", true, func(string, string, bool) []byte { return pgAuth(5) }, "", false},
		{"md5 in the clear, trust over TLS", true, func(_, _ string, overTLS bool) []byte {
			return map[bool][]byte{true: ok, false: pgAuth(5)}[overTLS]
		}, "postgres", false},
		// Postgres looks for the database only once the login is in: no database named for the owner says it got in.
		{"no database named for the owner's login", false, func(user, database string, _ bool) []byte {
			return map[bool][]byte{true: pgError("3D000"), false: shut}[user == owner && database == owner]
		}, owner, false},
		{"starting up", false, func(string, string, bool) []byte { return pgError("57P03") }, "", true},
		{"too many connections", false, func(string, string, bool) []byte { return pgError("53300") }, "", true},
		{"not Postgres", false, func(string, string, bool) []byte { return []byte("SSH-2.0-stand-in\r\n") }, "", true},
		{"silent", false, func(string, string, bool) []byte { time.Sleep(3 * time.Second); return nil }, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &pgStandIn{answer: c.answer}
			if c.cert {
				s.cert = standInTLS(t)
			}
			port := checkStandIn(t, s.serve)
			onlyPlaces(t, dbPlace{"Postgres", "tcp", fmt.Sprintf("127.0.0.1:%d", port)})
			found := checkDatabases(ownAddressesReached())
			wantLetIn := []string(nil)
			if c.letIn != "" {
				wantLetIn = []string{fmt.Sprintf("Postgres at 127.0.0.1:%d as %s", port, c.letIn)}
			}
			if !slices.Equal(found.LetIn, wantLetIn) || (len(found.CantTell) > 0) != c.cantTel {
				t.Errorf("found %+v; want let in %v, can't tell %v (startups %v)", found, wantLetIn, c.cantTel, s.starts)
			}
			if found.passed() != (c.letIn == "" && !c.cantTel) {
				t.Errorf("passed %v", found.passed())
			}
			if c.letIn != "" && !strings.Contains(found.refusal(), "logged in to "+wantLetIn[0]+" without a password") {
				t.Errorf("the words: %q", found.refusal())
			}
		})
	}
}

// The same for MySQL and MariaDB: root with no password, a login by socket that is the owner's (MariaDB's
// unix_socket, through an auth switch), caching_sha2_password's fast auth, and a server that wants TLS let in;
// access denied, and caching_sha2_password asking for full authentication (the password), pass. An error that isn't
// about the login, and the pre-4.1 password switch, can't be told.
func TestAPasswordLessMySQLLoginIsFoundAndOnlyAFailedOnePasses(t *testing.T) {
	quickChecks(t)
	owner := ownerLogin()
	cases := []struct {
		name     string
		caps     uint32
		cert     bool
		greeting []byte
		answer   func(user string, overTLS bool, round int) []byte
		letIn    string
		cantTell bool
	}{
		{"root with no password", mysqlCaps, false, nil, func(string, bool, int) []byte { return mysqlOK }, "root", false},
		{"access denied", mysqlCaps, false, nil, func(string, bool, int) []byte { return mysqlErr(1045) }, "", false},
		{"unix_socket for the owner", mysqlCaps, false, nil, func(user string, _ bool, round int) []byte {
			if round == 0 {
				return append([]byte{0xFE}, "unix_socket\x00"...)
			}
			return map[bool][]byte{true: mysqlOK, false: mysqlErr(1698)}[user == owner]
		}, owner, false},
		{"fast auth", mysqlCaps, false, nil, func(_ string, _ bool, round int) []byte {
			return [][]byte{{0x01, 3}, mysqlOK}[min(round, 1)]
		}, "root", false},
		{"full authentication wanted", mysqlCaps, false, nil, func(string, bool, int) []byte { return []byte{0x01, 4} }, "", false},
		{"fast auth, then denied", mysqlCaps, false, nil, func(_ string, _ bool, round int) []byte {
			return [][]byte{{0x01, 3}, mysqlErr(1045)}[min(round, 1)]
		}, "", false},
		{"a switch to native, then OK", mysqlCaps, false, nil, func(_ string, _ bool, round int) []byte {
			return [][]byte{append([]byte{0xFE}, "mysql_native_password\x00abcdefghijklmnopqrst\x00"...), mysqlOK}[min(round, 1)]
		}, "root", false},
		{"a switch to native, then denied", mysqlCaps, false, nil, func(_ string, _ bool, round int) []byte {
			return [][]byte{append([]byte{0xFE}, "mysql_native_password\x00abcdefghijklmnopqrst\x00"...), mysqlErr(1045)}[min(round, 1)]
		}, "", false},
		{"a switch to a clear password, then denied", mysqlCaps, false, nil, func(_ string, _ bool, round int) []byte {
			return [][]byte{append([]byte{0xFE}, "mysql_clear_password\x00"...), mysqlErr(1045)}[min(round, 1)]
		}, "", false},
		{"a plugin this login can't speak", mysqlCaps, false, nil, func(string, bool, int) []byte { return mysqlErr(1251) }, "", true},
		{"TLS required", mysqlCaps | mysqlSSL, true, nil, func(_ string, overTLS bool, _ int) []byte {
			return map[bool][]byte{true: mysqlOK, false: mysqlErr(3159)}[overTLS]
		}, "root", false},
		{"TLS offered, denied both ways", mysqlCaps | mysqlSSL, true, nil, func(string, bool, int) []byte { return mysqlErr(1045) }, "", false},
		{"TLS required, denied over TLS", mysqlCaps | mysqlSSL, true, nil, func(_ string, overTLS bool, _ int) []byte {
			return map[bool][]byte{true: mysqlErr(1045), false: mysqlErr(3159)}[overTLS]
		}, "", false},
		{"too many connections", mysqlCaps, false, nil, func(string, bool, int) []byte { return mysqlErr(1040) }, "", true},
		{"the old password switch", mysqlCaps, false, nil, func(string, bool, int) []byte { return []byte{0xFE} }, "", true},
		{"an error greeting", mysqlCaps, false, mysqlErr(1040), nil, "", true},
		{"a host not let in", mysqlCaps, false, mysqlErr(1130), nil, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &mysqlStandIn{caps: c.caps, plugin: "caching_sha2_password", greeting: c.greeting, answer: c.answer}
			if c.cert {
				s.cert = standInTLS(t)
			}
			port := checkStandIn(t, s.serve)
			onlyPlaces(t, dbPlace{"MySQL", "tcp", fmt.Sprintf("127.0.0.1:%d", port)})
			found := checkDatabases(ownAddressesReached())
			wantLetIn := []string(nil)
			if c.letIn != "" {
				wantLetIn = []string{fmt.Sprintf("MySQL at 127.0.0.1:%d as %s", port, c.letIn)}
			}
			if !slices.Equal(found.LetIn, wantLetIn) || (len(found.CantTell) > 0) != c.cantTell {
				t.Errorf("found %+v; want let in %v, can't tell %v", found, wantLetIn, c.cantTell)
			}
		})
	}
}

// Nothing listening passes, a place whose kind has no login written can't be told, and the words name the place.
func TestNothingThereRunsNoLoginButAnUnknownKindCantBeTold(t *testing.T) {
	quickChecks(t)
	port := closedPort(t)
	onlyPlaces(t, dbPlace{"Postgres", "tcp", fmt.Sprintf("127.0.0.1:%d", port)}, dbPlace{"MySQL", "tcp", fmt.Sprintf("127.0.0.1:%d", port)})
	if found := checkDatabases(ownAddressesReached()); !found.passed() || found.refusal() != "" {
		t.Errorf("nothing listening: %+v, %q", found, found.refusal())
	}
	s := &pgStandIn{answer: func(string, string, bool) []byte { return pgAuth(5) }}
	other := checkStandIn(t, s.serve)
	onlyPlaces(t, dbPlace{"Redis", "tcp", fmt.Sprintf("127.0.0.1:%d", other)})
	found := checkDatabases(ownAddressesReached())
	if found.passed() || !strings.Contains(found.refusal(), fmt.Sprintf("Redis at 127.0.0.1:%d", other)) {
		t.Errorf("an unknown kind: %+v, %q", found, found.refusal())
	}
}

// Every socket and port the opt-in opens, on a Mac and on Linux, is a place the check tries, of a kind it has a
// login for: one added to the lists without a login here fails this test (and the opt-in, as it can't be told).
func TestEveryPlaceTheOptInOpensIsTriedWithALogin(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		places := realDatabasePlaces(goos, []string{"127.0.0.1", "::1"})
		for _, s := range sandbox.Sockets(goos) {
			if s.Need == sandbox.DatabaseOptIn && !slices.Contains(places, dbPlace{s.Name, "unix", s.At}) {
				t.Errorf("%s: socket %s isn't tried", goos, s.At)
			}
		}
		for _, p := range sandbox.LocalPorts() {
			for _, addr := range []string{"127.0.0.1", "::1"} {
				place := dbPlace{databasePortKinds[p.Port], "tcp", net.JoinHostPort(addr, fmt.Sprint(p.Port))}
				if slices.Contains(places, place) != p.OpenOnOptIn {
					t.Errorf("%s: port %d at %s tried %v, opened by the opt-in %v", goos, p.Port, addr, !p.OpenOnOptIn, p.OpenOnOptIn)
				}
			}
		}
		for _, place := range places {
			if place.Kind != "Postgres" && place.Kind != "MySQL" {
				t.Errorf("%s: %v has no login", goos, place)
			}
		}
	}
	if remoteLoginPortReal := 22; !strings.Contains(sandbox.LocalPorts()[0].Why, "ssh") || sandbox.LocalPorts()[0].Port != remoteLoginPortReal {
		t.Errorf("the lists' first port isn't Remote Login's: %+v", sandbox.LocalPorts()[0])
	}
}

// Port 22's check answers while something listens (one that closes at once, or resets the connection, too) and
// not otherwise, and a dial that neither connects nor says nothing is there counts as an answer (fail closed): a
// timeout, an error it doesn't know, and a TCP dial refused permission (a filter may stop the core and not a desk).
func TestRemoteLoginAnswersOnlyWhileSomethingListens(t *testing.T) {
	saved := remoteLoginPort
	t.Cleanup(func() { remoteLoginPort = saved })
	remoteLoginPort = checkStandIn(t, func(net.Conn) {})
	if !remoteLoginAnswers(ownAddressesReached()) {
		t.Error("a listener didn't answer")
	}
	remoteLoginPort = checkStandIn(t, func(conn net.Conn) { _ = conn.(*net.TCPConn).SetLinger(0) }) // closed with a reset
	if !remoteLoginAnswers(ownAddressesReached()) {
		t.Error("a listener that resets the connection didn't answer")
	}
	remoteLoginPort = closedPort(t)
	if remoteLoginAnswers(ownAddressesReached()) {
		t.Error("a closed port answered")
	}
	try := checkTryWait
	t.Cleanup(func() { checkTryWait = try })
	checkTryWait = time.Nanosecond
	if !remoteLoginAnswers(ownAddressesReached()) {
		t.Error("a dial that timed out counted as nothing there")
	}
	dialErr := func(network string, errno syscall.Errno) error {
		return &net.OpError{Op: "dial", Net: network, Err: os.NewSyscallError("connect", errno)}
	}
	for _, row := range []struct {
		network string
		err     error
		nothing bool
	}{
		{"tcp", dialErr("tcp", syscall.ECONNREFUSED), true},
		{"unix", dialErr("unix", syscall.ENOENT), true},
		{"unix", dialErr("unix", syscall.ECONNREFUSED), true},
		{"unix", dialErr("unix", syscall.EACCES), true},
		{"tcp", dialErr("tcp", syscall.EACCES), false},
		{"tcp", dialErr("tcp", syscall.EADDRNOTAVAIL), true},
		{"tcp", dialErr("tcp", syscall.ENETUNREACH), true},
		{"tcp", dialErr("tcp", syscall.EHOSTUNREACH), true},
		{"tcp", dialErr("tcp", syscall.EPERM), false},
		{"tcp", dialErr("tcp", syscall.ETIMEDOUT), false},
		{"tcp", dialErr("tcp", syscall.ECONNRESET), false},
		{"tcp", dialErr("tcp", syscall.EHOSTDOWN), false},
		{"tcp", dialErr("tcp", syscall.ENETDOWN), false},
		{"tcp", dialErr("tcp", syscall.EINVAL), false},
		{"tcp", context.DeadlineExceeded, false},
		{"tcp", fmt.Errorf("dial: %w", context.DeadlineExceeded), false},
		{"tcp", errors.New("i/o timeout"), false},
		{"unix", errors.New("weird"), false},
	} {
		if nothingThere(row.network, row.err) != row.nothing {
			t.Errorf("nothingThere(%s, %v) = %v", row.network, row.err, !row.nothing)
		}
	}
}

// An auth switch is answered with no password: an empty response for a plugin that hashes it, the one byte an empty
// clear password is for one that sends it as it is, and never anything more.
func TestAnAuthSwitchIsAnsweredWithAnEmptyPassword(t *testing.T) {
	quickChecks(t)
	for plugin, want := range map[string][]byte{
		"mysql_native_password": {}, "caching_sha2_password": {}, "mysql_clear_password": {0}, "sha256_password": {0},
	} {
		s := &mysqlStandIn{caps: mysqlCaps, plugin: "caching_sha2_password", answer: func(_ string, _ bool, round int) []byte {
			return [][]byte{append(append([]byte{0xFE}, plugin...), "\x00abcdefghijklmnopqrst\x00"...), mysqlErr(1045)}[min(round, 1)]
		}}
		onlyPlaces(t, dbPlace{"MySQL", "tcp", fmt.Sprintf("127.0.0.1:%d", checkStandIn(t, s.serve))})
		if found := checkDatabases(ownAddressesReached()); !found.passed() {
			t.Errorf("%s: %+v", plugin, found)
		}
		s.mu.Lock()
		if len(s.switched) == 0 || slices.ContainsFunc(s.switched, func(said []byte) bool { return !bytes.Equal(said, want) }) {
			t.Errorf("%s: answered %q, want %q each time", plugin, s.switched, want)
		}
		s.mu.Unlock()
	}
}

// RT126SAM12's probes: a place that answers, but so its answer can't be read, can't be told, whichever way: TLS from
// the first byte, garbage, a close before or after the login, a message too big to be one, TLS offered and then not
// spoken, a greeting of another protocol version, or nothing in time.
func TestADatabaseWhoseAnswerCantBeReadCantBeTold(t *testing.T) {
	quickChecks(t)
	cert := standInTLS(t)
	tlsFirst := func(conn net.Conn) { _ = tls.Server(conn, cert).Handshake() }
	garbage := func(conn net.Conn) { conn.Write([]byte("garbage, not a database\r\n")); io.Copy(io.Discard, conn) }
	closes := func(net.Conn) {}
	slow := func(net.Conn) { time.Sleep(3 * time.Second) }
	pg := func(answer []byte) func(net.Conn) {
		return (&pgStandIn{answer: func(string, string, bool) []byte { return answer }}).serve
	}
	// md5 in the clear, so the TLS try follows, where S is said and then no TLS spoken.
	sThenGarbage := func(conn net.Conn) {
		head := make([]byte, 8)
		if _, err := io.ReadFull(conn, head); err != nil {
			return
		}
		if binary.BigEndian.Uint32(head[4:]) == 80877103 {
			conn.Write([]byte("Sgarbage, not TLS\r\n"))
		} else if _, err := io.ReadFull(conn, make([]byte, binary.BigEndian.Uint32(head)-8)); err == nil {
			conn.Write(pgAuth(5))
		}
		io.Copy(io.Discard, conn)
	}
	mysql := func(s *mysqlStandIn) func(net.Conn) {
		s.caps, s.plugin = mysqlCaps, "caching_sha2_password"
		return s.serve
	}
	for _, row := range []struct {
		name, kind string
		serve      func(net.Conn)
	}{
		{"TLS from the first byte", "Postgres", tlsFirst},
		{"garbage", "Postgres", garbage},
		{"closes at once", "Postgres", closes},
		{"closes after the startup", "Postgres", func(conn net.Conn) { io.ReadFull(conn, make([]byte, 8)) }},
		// The same, but TLS answered N: what the clear startup got must count alone.
		{"closes after the startup, no TLS", "Postgres", func(conn net.Conn) {
			head := make([]byte, 8)
			if _, err := io.ReadFull(conn, head); err != nil {
				return
			}
			if binary.BigEndian.Uint32(head[4:]) == 80877103 {
				conn.Write([]byte("N"))
				io.Copy(io.Discard, conn)
				return
			}
			io.ReadFull(conn, make([]byte, binary.BigEndian.Uint32(head)-8))
		}},
		{"a message too big", "Postgres", pg([]byte{'R', 0, 0x10, 0, 0})},
		{"S, then no TLS", "Postgres", sThenGarbage},
		{"slow", "Postgres", slow},
		{"TLS from the first byte", "MySQL", tlsFirst},
		{"garbage", "MySQL", garbage},
		{"closes before the greeting", "MySQL", closes},
		{"closes after the login", "MySQL", mysql(&mysqlStandIn{answer: func(string, bool, int) []byte { return nil }})},
		{"a protocol 9 greeting", "MySQL", mysql(&mysqlStandIn{greeting: append([]byte{9}, "4.0-stand-in\x00abcdefgh\x00"...)})},
		{"slow", "MySQL", slow},
	} {
		t.Run(row.kind+", "+row.name, func(t *testing.T) {
			place := dbPlace{row.kind, "tcp", fmt.Sprintf("127.0.0.1:%d", checkStandIn(t, row.serve))}
			onlyPlaces(t, place)
			if found := checkDatabases(ownAddressesReached()); !slices.Equal(found.CantTell, []string{place.String()}) || len(found.LetIn) != 0 || found.passed() {
				t.Errorf("found %+v; want only %s can't be told", found, place)
			}
		})
	}
}

// The logins tried are the database's superuser and the owner's own login name, once each.
func TestTheLoginsTriedAreTheSuperusersAndTheOwners(t *testing.T) {
	if got := databaseLogins("Postgres"); !slices.Equal(got, []string{"postgres", testOwnerLogin}) {
		t.Errorf("Postgres: %v", got)
	}
	if got := databaseLogins("MySQL"); !slices.Equal(got, []string{"root", testOwnerLogin}) {
		t.Errorf("MySQL: %v", got)
	}
	t.Cleanup(func() { ownerLogin = func() string { return testOwnerLogin } })
	for _, own := range []string{"root", ""} {
		ownerLogin = func() string { return own }
		if got := databaseLogins("MySQL"); !slices.Equal(got, []string{"root"}) {
			t.Errorf("MySQL, the owner's login %q: %v", own, got)
		}
	}
}

// A desk's start checks: Remote Login always; the databases only when the owner's opt-in is on (none is dialed
// otherwise), open only while every login fails.
func TestADesksStartChecksTheDatabasesOnlyWithTheOptIn(t *testing.T) {
	quickChecks(t)
	freshChecks(t)
	saved := remoteLoginPort
	t.Cleanup(func() { remoteLoginPort = saved })
	remoteLoginPort = checkStandIn(t, func(net.Conn) {})
	trust := true
	s := &pgStandIn{answer: func(string, string, bool) []byte { return map[bool][]byte{true: pgAuth(0), false: pgAuth(5)}[trust] }}
	onlyPlaces(t, dbPlace{"Postgres", "tcp", fmt.Sprintf("127.0.0.1:%d", checkStandIn(t, s.serve))})
	if c := deskStartChecks("h1", false); c != (sandboxChecked{RemoteLogin: true}) || s.count() != 0 {
		t.Errorf("no opt-in: %+v, %d logins", c, s.count())
	}
	if c := deskStartChecks("h2", true); c.Database {
		t.Errorf("a database that lets in: %+v", c)
	}
	trust = false
	if c := deskStartChecks("h3", true); !c.Database || !c.RemoteLogin {
		t.Errorf("a database that asks: %+v", c)
	}
}

// Running desks are tagged "restart to apply" when Remote Login answers now and didn't at their start, and when
// they started with the databases open that the opt-in, or the last check, no longer allows. A stopped desk isn't.
func TestRunningDesksAreTaggedWhenTheirStartAllowedMoreThanNow(t *testing.T) {
	quickChecks(t)
	freshChecks(t)
	saved := remoteLoginPort
	t.Cleanup(func() { remoteLoginPort = saved })
	remoteLoginPort = closedPort(t)
	trust := false
	s := &pgStandIn{answer: func(string, string, bool) []byte { return map[bool][]byte{true: pgAuth(0), false: pgAuth(5)}[trust] }}
	onlyPlaces(t, dbPlace{"Postgres", "tcp", fmt.Sprintf("127.0.0.1:%d", checkStandIn(t, s.serve))})
	deskStartChecks("before", true)
	if tags := restartToApply(true); len(tags) != 0 {
		t.Fatalf("nothing changed: %v", tags)
	}
	remoteLoginPort = checkStandIn(t, func(net.Conn) {})
	deskStartChecks("after", true)
	tags := restartToApply(true)
	if !strings.Contains(tags["before"], "SSH keys") || len(tags) != 1 {
		t.Errorf("Remote Login on since: %v", tags)
	}
	if tags := restartToApply(false); !strings.Contains(tags["after"], "local databases") {
		t.Errorf("the opt-in off since: %v", tags)
	}
	trust = true
	deskStartChecks("third", true)
	if tags := restartToApply(true); !strings.Contains(tags["after"], "local databases") || tags["third"] != "" {
		t.Errorf("a database letting in since: %v", tags)
	}
	deskChecksForgotten("before")
	deskChecksForgotten("after")
	if tags := restartToApply(false); len(tags) != 0 {
		t.Errorf("stopped desks tagged: %v", tags)
	}
}

// The owner's opt-in counts only once the password-less login failed: refused with the place and the login
// otherwise, the setting left off. On, it's saved sealed, so it's still on after a restart, but a file changed by
// another program to say so doesn't count; off is always taken.
func TestTheDatabaseOptInIsSavedOnlyAfterTheLoginFails(t *testing.T) {
	quickChecks(t)
	freshChecks(t)
	d, _, _, _ := macDeck(t)
	trust := true
	s := &pgStandIn{answer: func(string, string, bool) []byte { return map[bool][]byte{true: pgAuth(0), false: pgAuth(5)}[trust] }}
	port := checkStandIn(t, s.serve)
	onlyPlaces(t, dbPlace{"Postgres", "tcp", fmt.Sprintf("127.0.0.1:%d", port)})
	optIn := func(d *Deck) bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.config.LocalDatabaseAsksPassword
	}
	if code, words := d.setDatabaseOptIn("1"); code != http.StatusConflict || !strings.Contains(words, fmt.Sprintf("Postgres at 127.0.0.1:%d as postgres", port)) {
		t.Errorf("a database that lets in: %d %q", code, words)
	}
	if optIn(d) {
		t.Fatal("the opt-in was kept though the login got in")
	}
	if code, _ := d.setDatabaseOptIn("yes"); code != http.StatusBadRequest {
		t.Errorf("a bad value: %d", code)
	}
	trust = false
	if code, words := d.setDatabaseOptIn("1"); code != http.StatusNoContent || !optIn(d) {
		t.Fatalf("a database that asks: %d %q", code, words)
	}
	if d = restart(t); !optIn(d) {
		t.Fatal("the opt-in didn't last a restart: its seal doesn't fit")
	}
	if code, _ := d.setDatabaseOptIn("0"); code != http.StatusNoContent || optIn(d) {
		t.Errorf("off: %d", code)
	}
	// Another program writes the opt-in into the file: the seal doesn't fit, so it's off after the restart.
	rewrite(t, "config.json", func(f map[string]any) { f["localDatabaseAsksPassword"] = true })
	if d = restart(t); optIn(d) {
		t.Error("an opt-in written by another program counted")
	}
}

// A database that answers so it can't be told never opens the opt-in: it's refused with the words that say so, and
// left off, and a desk's start with it on keeps the databases shut.
func TestADatabaseThatCantBeToldNeverOpensTheOptIn(t *testing.T) {
	quickChecks(t)
	freshChecks(t)
	d, _, _, _ := macDeck(t)
	port := checkStandIn(t, func(net.Conn) { time.Sleep(3 * time.Second) })
	onlyPlaces(t, dbPlace{"Postgres", "tcp", fmt.Sprintf("127.0.0.1:%d", port)})
	code, words := d.setDatabaseOptIn("1")
	if code != http.StatusConflict || !strings.Contains(words, fmt.Sprintf("couldn't tell whether Postgres at 127.0.0.1:%d asks for a password", port)) {
		t.Errorf("a silent database: %d %q", code, words)
	}
	d.mu.Lock()
	on := d.config.LocalDatabaseAsksPassword
	d.mu.Unlock()
	if on {
		t.Error("the opt-in was kept though the database couldn't be told")
	}
	if c := deskStartChecks("silent", true); c.Database {
		t.Errorf("a desk's start: %+v", c)
	}
}

// While the records' key can't be read, the opt-in is a switch like the others: on is taken for the run (it's off
// again at the restart, as saved), off again too while the file has it off, and off refused while the file has it on.
func TestOnAMacWhileTheKeyIsUnreadableTheDatabaseOptInIsHeldAsASwitch(t *testing.T) {
	quickChecks(t)
	freshChecks(t)
	s := &pgStandIn{answer: func(string, string, bool) []byte { return pgAuth(5) }}
	onlyPlaces(t, dbPlace{"Postgres", "tcp", fmt.Sprintf("127.0.0.1:%d", checkStandIn(t, s.serve))})
	optIn := func(d *Deck) bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.config.LocalDatabaseAsksPassword
	}
	_, mac, _, _ := macDeck(t)
	mac.recordLook = "unreadable"
	held := restart(t)
	if !recordsHeldOnly() {
		t.Fatalf("the records' state: %q", recordsState())
	}
	if code, words := held.setDatabaseOptIn("1"); code != http.StatusNoContent || !optIn(held) {
		t.Errorf("on while the key can't be read: %d %q", code, words)
	}
	if code, words := held.setDatabaseOptIn("0"); code != http.StatusNoContent || optIn(held) {
		t.Errorf("off again, off in the file: %d %q", code, words)
	}
	mac.recordLook = ""
	d := restart(t)
	if code, words := d.setDatabaseOptIn("1"); code != http.StatusNoContent {
		t.Fatalf("on, saved: %d %q", code, words)
	}
	mac.recordLook = "unreadable"
	held = restart(t)
	if code, words := held.setDatabaseOptIn("0"); code != http.StatusConflict || !strings.Contains(words, recordsHeldAnswer) {
		t.Errorf("off while the key can't be read, on in the file: %d %q", code, words)
	}
	mac.recordLook = ""
	if d = restart(t); !optIn(d) {
		t.Error("the opt-in saved on didn't last the held run")
	}
}

// The opt-in joins config.json's sealed part only while it's on: a file sealed before it was a setting still fits.
func TestTheOptInIsInTheSealedPartOnlyWhileOn(t *testing.T) {
	var c Config
	before := configSecurityPart(&c)
	if bytes.Contains(before, []byte("localDatabaseAsksPassword")) {
		t.Errorf("off, in the part: %s", before)
	}
	c.LocalDatabaseAsksPassword = true
	if on := configSecurityPart(&c); !bytes.Contains(on, []byte(`"localDatabaseAsksPassword":true`)) {
		t.Errorf("on, not in the part: %s", on)
	}
}

// standInNetwork has the checks dial a made-up set of this computer's addresses, all of them on 127.0.0.1
// underneath: a TCP dial to address:port reaches 127.0.0.1:port, except at an address in silent (every port: nothing
// comes back, as Tailscale's IPv6 address on a Mac) or an address:port in silentAt (that port only), where it waits
// for the dial's deadline, and at an address in dialErr, which fails so. Nothing leaves 127.0.0.1; a unix socket is
// dialed as it is. Every TCP address dialed is kept, so a test can tell which were probed.
type standInNetwork struct {
	silent, silentAt map[string]bool
	dialErr          map[string]error
	mu               sync.Mutex
	dialed           []string
}

func (n *standInNetwork) install(t *testing.T) {
	dial := checkDial
	t.Cleanup(func() { checkDial = dial })
	checkDial = func(ctx context.Context, network, address string) (net.Conn, error) {
		var dialer net.Dialer
		if network != "tcp" {
			return dialer.DialContext(ctx, network, address)
		}
		n.mu.Lock()
		n.dialed = append(n.dialed, address)
		n.mu.Unlock()
		host, port, _ := net.SplitHostPort(address)
		if err := n.dialErr[host]; err != nil {
			return nil, &net.OpError{Op: "dial", Net: network, Err: err}
		}
		if n.silent[host] || n.silentAt[address] {
			<-ctx.Done()
			return nil, &net.OpError{Op: "dial", Net: network, Err: ctx.Err()}
		}
		return dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
	}
}

// probed is each address the reach probe dialed: at the reach port.
func (n *standInNetwork) probed() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []string
	for _, address := range n.dialed {
		if host, port, _ := net.SplitHostPort(address); port == fmt.Sprint(reachPort()) {
			out = append(out, host)
		}
	}
	return out
}

// Stand-ins for this computer's own addresses (never dialed: the stand-in network takes them to 127.0.0.1): one like
// Tailscale's IPv6 address, which a Mac can't reach itself on, and a LAN-like one it can.
const (
	tailscaleV6StandIn = "fd7a:115c:a1e0::5"
	lanStandIn         = "192.0.2.10"
)

// FXSAM12TS B1: an address of this computer that this computer can't reach itself on (Tailscale's IPv6 address on a
// Mac times out on every port, even the open 22) decides nothing. The reach probe's dial there isn't answered either,
// so it's left out before anything else is dialed: Remote Login isn't "on" for it, the databases on it aren't "can't
// tell", the opt-in opens, and a start takes milliseconds, not the 2 s a dial there waits. An address the probe
// reaches (connected or refused) and that is silent on port 22, or on a database's port, still counts (fail closed).
func TestAnAddressThisComputerCantReachItselfOnDecidesNothing(t *testing.T) {
	quickChecks(t)
	freshChecks(t)
	checkTryWait = 1500 * time.Millisecond
	savedProbe, savedAddrs, savedPort, savedReach := probeWait, ownAddresses, remoteLoginPort, reachPort
	t.Cleanup(func() {
		probeWait, ownAddresses, remoteLoginPort, reachPort = savedProbe, savedAddrs, savedPort, savedReach
	})
	probeWait = 200 * time.Millisecond
	remoteLoginPort = closedPort(t)
	own := func(addrs ...string) { ownAddresses = func() []string { return addrs } }
	s := &pgStandIn{answer: func(string, string, bool) []byte { return pgAuth(5) }} // asks for a password
	dbPort := checkStandIn(t, s.serve)
	saved := databasePlaces
	t.Cleanup(func() { databasePlaces = saved })
	databasePlaces = func(_ string, own []string) []dbPlace {
		var out []dbPlace
		for _, addr := range own {
			out = append(out, dbPlace{"Postgres", "tcp", net.JoinHostPort(addr, fmt.Sprint(dbPort))})
		}
		return out
	}
	d, _, _, _ := macDeck(t)

	// Nothing silent: every address is reached (the reach port is init's closed 7799: refused), and a start is
	// milliseconds.
	n := &standInNetwork{}
	n.install(t)
	own("127.0.0.1", "::1", lanStandIn)
	start := time.Now()
	if c := deskStartChecks("all-reached", true); c != (sandboxChecked{RemoteLogin: false, Database: true}) {
		t.Errorf("nothing silent: %+v", c)
	}
	t.Logf("a start with nothing silent took %v", time.Since(start))
	if probed := n.probed(); !slices.Equal(probed, []string{lanStandIn}) {
		t.Errorf("probed %v; want only %s (a loopback is always reached)", probed, lanStandIn)
	}

	// Tailscale's IPv6 address, silent for the probe too: skipped, and it decides nothing.
	n = &standInNetwork{silent: map[string]bool{tailscaleV6StandIn: true}}
	n.install(t)
	own("127.0.0.1", lanStandIn, tailscaleV6StandIn)
	if got := ownAddressesReached(); !slices.Equal(got, []string{"127.0.0.1", lanStandIn}) {
		t.Errorf("reached %v; want it without %s", got, tailscaleV6StandIn)
	}
	start = time.Now()
	c := deskStartChecks("tailscale", true)
	took := time.Since(start)
	t.Logf("a start with Tailscale's IPv6 address silent took %v (probe wait %v, a dial's %v)", took, probeWait, checkTryWait)
	if c != (sandboxChecked{RemoteLogin: false, Database: true}) {
		t.Errorf("Tailscale's IPv6 address silent: %+v; want Remote Login off and the databases open", c)
	}
	if took >= checkTryWait {
		t.Errorf("the start took %v: the silent address was dialed (a dial waits %v)", took, checkTryWait)
	}
	if code, words := d.setDatabaseOptIn("1"); code != http.StatusNoContent {
		t.Errorf("the opt-in for a Tailscale user: %d %q", code, words)
	}

	// Reached by the probe, silent on port 22 or on the database's port: it counts (fail closed).
	n = &standInNetwork{silentAt: map[string]bool{
		net.JoinHostPort(lanStandIn, fmt.Sprint(remoteLoginPort)): true,
		net.JoinHostPort(lanStandIn, fmt.Sprint(dbPort)):          true,
	}}
	n.install(t)
	own("127.0.0.1", lanStandIn)
	if got := ownAddressesReached(); !slices.Equal(got, []string{"127.0.0.1", lanStandIn}) {
		t.Errorf("reached %v; want %s kept", got, lanStandIn)
	}
	if c := deskStartChecks("silent-on-22", true); c != (sandboxChecked{RemoteLogin: true, Database: false}) {
		t.Errorf("reached, silent on 22 and the database: %+v; want Remote Login on and the databases shut", c)
	}
	code, words := d.setDatabaseOptIn("1")
	if want := fmt.Sprintf("couldn't tell whether Postgres at %s asks", net.JoinHostPort(lanStandIn, fmt.Sprint(dbPort))); code != http.StatusConflict || !strings.Contains(words, want) {
		t.Errorf("the opt-in with a reached address silent: %d %q", code, words)
	}

	// What the probe's dial gets: only no answer in time, or no route, leaves an address out. A connection (the
	// core's own port), a refusal (the closed port) and every other error keep it, and its silence on port 22 then
	// counts. An address gone from this computer by now refuses too (EADDRNOTAVAIL): kept (RT126SAM12R3 F1, closed by
	// there being no listener to fail).
	answering := checkStandIn(t, func(net.Conn) {})
	silent22 := map[string]bool{net.JoinHostPort(lanStandIn, fmt.Sprint(remoteLoginPort)): true}
	for _, row := range []struct {
		name    string
		port    int // the reach port; 0 for init's closed 7799
		silent  bool
		dialErr error
		skipped bool
	}{
		{"no answer in time", 0, true, nil, true},
		{"no route to host", 0, false, os.NewSyscallError("connect", syscall.EHOSTUNREACH), true},
		{"network unreachable", 0, false, os.NewSyscallError("connect", syscall.ENETUNREACH), true},
		{"timed out", 0, false, os.NewSyscallError("connect", syscall.ETIMEDOUT), true},
		{"connected: the core's own port", answering, false, nil, false},
		{"refused: the closed port", 0, false, nil, false},
		{"refused, as the system says it", 0, false, os.NewSyscallError("connect", syscall.ECONNREFUSED), false},
		{"address not available: gone by now", 0, false, os.NewSyscallError("connect", syscall.EADDRNOTAVAIL), false},
		{"a filter's permission denied", 0, false, os.NewSyscallError("connect", syscall.EPERM), false},
		{"a filter's access denied", 0, false, os.NewSyscallError("connect", syscall.EACCES), false},
		{"reset", 0, false, os.NewSyscallError("connect", syscall.ECONNRESET), false},
		{"an unknown error", 0, false, errors.New("weird"), false},
	} {
		reachPort = savedReach
		if row.port != 0 {
			port := row.port
			reachPort = func() int { return port }
		}
		n := &standInNetwork{silentAt: silent22}
		if row.silent {
			n.silent = map[string]bool{lanStandIn: true}
		}
		if row.dialErr != nil {
			n.dialErr = map[string]error{lanStandIn: row.dialErr}
		}
		n.install(t)
		if got := unreachedHere("darwin", lanStandIn, reachPort()); got != row.skipped {
			t.Errorf("the probe's dial: %s: left out %v, want %v", row.name, got, row.skipped)
		}
		if row.skipped || row.dialErr != nil {
			continue
		}
		// Kept with its port 22 silent: it answers.
		own("127.0.0.1", lanStandIn)
		if !remoteLoginAnswers(ownAddressesReached()) {
			t.Errorf("the probe's dial: %s: kept, silent on 22, didn't answer", row.name)
		}
	}
	reachPort = savedReach

	// Windows wasn't measured: nothing is probed there, and every address is kept (fail closed). A loopback is never
	// probed, and an address that isn't one never left out.
	n = &standInNetwork{silent: map[string]bool{tailscaleV6StandIn: true}}
	n.install(t)
	for _, addr := range []string{tailscaleV6StandIn, "127.0.0.1", "::1", "not-an-address"} {
		goos := map[bool]string{true: "windows", false: "darwin"}[addr == tailscaleV6StandIn]
		if unreachedHere(goos, addr, reachPort()) {
			t.Errorf("%s on %s was left out", addr, goos)
		}
	}
	if probed := n.probed(); len(probed) != 0 {
		t.Errorf("probed %v: Windows, a loopback and a non-address are never dialed", probed)
	}

	// RT126SAM12R3 F2: restartToApply's own dial of port 22 leaves the silent address out too. Without the probe there,
	// a Tailscale Mac would tag every running desk "Remote Login was turned on after this desk started".
	n = &standInNetwork{silent: map[string]bool{tailscaleV6StandIn: true}}
	n.install(t)
	own("127.0.0.1", lanStandIn, tailscaleV6StandIn)
	freshChecks(t)
	deskStartChecks("running", false)
	if tags := restartToApply(false); len(tags) != 0 {
		t.Errorf("Remote Login off, Tailscale's IPv6 address silent: %v", tags)
	}
}

// RT126SAM12R3 (VICTOR's ruling, option B): the reach probe never listens; it dials the core's own port where the core
// listens on every address, and a port under 1024 nothing runs on everywhere else (a test copy's this computer only,
// and Tailscale's scope).
func TestTheReachProbeDialsTheCoresOwnPortOrAClosedOne(t *testing.T) {
	for _, row := range []struct {
		scope      string
		port, want int
	}{
		{listenAll, 7710, 7710},
		{listenAll, 7741, 7741},
		{listenTailscale, 7710, reachClosedPort},
		{listenThisComputer, 7741, reachClosedPort},
		{"", 7710, reachClosedPort},
		{listenAll, 0, reachClosedPort},
	} {
		if got := reachPortFor(row.scope, row.port); got != row.want {
			t.Errorf("reachPortFor(%q, %d) = %d, want %d", row.scope, row.port, got, row.want)
		}
	}
	if reachClosedPort < 1 || reachClosedPort >= 1024 {
		t.Errorf("reachClosedPort %d: a desk may listen on it on Linux", reachClosedPort)
	}
}

// What leaves an address out is a dial that timed out (as Go's own dialer says it, too: the row that pins its shape)
// or found no route; anything else, as a filter's refusal or a reset, isn't.
func TestOnlyATimeoutOrNoRouteLeavesAnAddressOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	var dialer net.Dialer
	_, realTimeout := dialer.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", closedPort(t)))
	if realTimeout == nil {
		t.Fatal("an expired dial connected")
	}
	dialErr := func(errno syscall.Errno) error {
		return &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", errno)}
	}
	for _, row := range []struct {
		err       error
		unreached bool
	}{
		{realTimeout, true},
		{context.DeadlineExceeded, true},
		{fmt.Errorf("dial: %w", context.DeadlineExceeded), true},
		{dialErr(syscall.ETIMEDOUT), true},
		{dialErr(syscall.EHOSTUNREACH), true},
		{dialErr(syscall.ENETUNREACH), true},
		{dialErr(syscall.ECONNREFUSED), false},
		{dialErr(syscall.EPERM), false},
		{dialErr(syscall.EACCES), false},
		{dialErr(syscall.ECONNRESET), false},
		{dialErr(syscall.EADDRNOTAVAIL), false},
		{context.Canceled, false},
		{errors.New("i/o timeout"), false},
	} {
		if unreachedErr(row.err) != row.unreached {
			t.Errorf("unreachedErr(%v) = %v", row.err, !row.unreached)
		}
	}
}

// RT126SAM12R3, VICTOR's guard row: nothing the sandbox checks run opens a listener, so no firewall can ask the owner
// about them (a test copy's above all, which listens on this computer only). Every declaration of sandboxchecks.go is
// read, with every function, variable and type of this package and of internal/sandbox that they name, and so on, and
// each method of a type read that is called by its name in what's read, on whatever (or is one the standard library
// calls itself, as String or ServeHTTP): more is read than runs. Short of the office's settings (the Deck and its
// Config: the opt-in's save and broadcast of the new setting, as for any setting), none may name a way to listen.
// Past them, which reach most of the office (a broadcast may start a hire), a listener is allowed only on 127.0.0.1 or
// a unix socket, said so in its own call: the office's own, as portIsFree and the desk-start socket, which the
// firewall doesn't ask about.
// What this can't see: a method of a type never named in what's read, reached through an interface value made
// elsewhere, and a function kept in a struct field by code that isn't read.
func TestNothingTheSandboxChecksRunOpensAListener(t *testing.T) {
	listens := map[string][]string{
		"net": {"Listen", "ListenTCP", "ListenUDP", "ListenUnix", "ListenUnixgram", "ListenIP", "ListenPacket",
			"ListenMulticastUDP", "ListenConfig", "FileListener", "FilePacketConn"},
		"crypto/tls":        {"Listen", "NewListener"},
		"net/http":          {"ListenAndServe", "ListenAndServeTLS"},
		"net/http/httptest": {"NewServer", "NewTLSServer", "NewUnstartedServer"},
		"syscall":           {"Listen", "Bind"},
	}
	const sandboxPath = "github.com/Sauzze/smolcorp/internal/sandbox"
	dirs := map[string]string{"desktop": ".", sandboxPath: "../internal/sandbox"}
	settings := []string{"desktop.Deck", "desktop.Config"} // with their methods
	short, found := reachedFrom(t, "sandboxchecks.go", dirs, listens, settings...)
	for _, name := range []string{"desktop.unreachedHere", "desktop.checkDial", "desktop.remoteLoginAnswers",
		"desktop.restartToApply", "desktop.checkDatabases", "desktop.Deck.setDatabaseOptIn", sandboxPath + ".Sockets"} {
		if !short[name] {
			t.Errorf("%s wasn't read", name)
		}
	}
	if short["desktop.Deck.changed"] || short["desktop.Config.save"] {
		t.Error("the walk short of the settings went past them")
	}
	t.Logf("short of the settings: %d declarations read", len(short))
	for _, f := range found {
		t.Errorf("a way to listen, run by the sandbox checks: %s: %s", strings.Join(f.via, " → "), f.call)
	}
	all, found := reachedFrom(t, "sandboxchecks.go", dirs, listens)
	for _, name := range []string{"desktop.Deck.changed", "desktop.Config.save"} {
		if !all[name] {
			t.Errorf("%s wasn't read", name)
		}
	}
	t.Logf("past them: %d declarations read", len(all))
	for _, f := range found {
		if strings.Contains(f.call, `"127.0.0.1"`) || strings.HasPrefix(f.call, `net.Listen("unix"`) {
			t.Logf("on 127.0.0.1 or a unix socket: %s, reached as %s", f.call, strings.Join(f.via, " → "))
			continue
		}
		t.Errorf("a way to listen past the settings, not on 127.0.0.1 or a unix socket: %s: %s",
			strings.Join(f.via, " → "), f.call)
	}
}

// listenFound is a use of a banned name: the declarations it was reached through, and the call, as written.
type listenFound struct {
	via  []string
	call string
}

// reachedFrom reads the packages in dirs (import path to folder; "desktop" for this one) and walks from every
// declaration of file in this package to everything they name: "<package>.<name>" for a function, variable, constant
// or type, and "<package>.<type>.<method>" for a method of a type reached that is called by its name in what's read
// (or by the standard library's own interfaces). It gives each declaration read, and each use of a name in banned
// (import path to names) with the way it was reached. The types in stop aren't read, nor their methods, unless declared
// in file.
func reachedFrom(t *testing.T, file string, dirs map[string]string, banned map[string][]string, stop ...string) (map[string]bool, []listenFound) {
	t.Helper()
	type decl struct {
		node ast.Node
		file *ast.File
		pkg  string
	}
	decls := map[string][]decl{}
	methods := map[string][]string{} // a type's methods' names, by "<package>.<type>"
	ofName := map[string][]string{}  // the types with a method, by its name
	var start []string
	fset := token.NewFileSet()
	for pkg, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			add := func(key string, node ast.Node) {
				decls[key] = append(decls[key], decl{node, f, pkg})
				if pkg == "desktop" && name == file {
					start = append(start, key)
				}
			}
			for _, d := range f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil {
						add(pkg+"."+d.Name.Name, d)
						continue
					}
					recv := d.Recv.List[0].Type
					for {
						switch r := recv.(type) {
						case *ast.StarExpr:
							recv = r.X
							continue
						case *ast.IndexExpr:
							recv = r.X
							continue
						case *ast.IndexListExpr:
							recv = r.X
							continue
						}
						break
					}
					typ := pkg + "." + recv.(*ast.Ident).Name
					methods[typ] = append(methods[typ], d.Name.Name)
					ofName[d.Name.Name] = append(ofName[d.Name.Name], typ)
					add(typ+"."+d.Name.Name, d)
				case *ast.GenDecl:
					for _, s := range d.Specs {
						switch s := s.(type) {
						case *ast.ValueSpec:
							for _, n := range s.Names {
								add(pkg+"."+n.Name, s)
							}
						case *ast.TypeSpec:
							add(pkg+"."+s.Name.Name, s)
						}
					}
				}
			}
		}
	}
	if len(start) == 0 {
		t.Fatalf("nothing declared in %s", file)
	}
	via := map[string]string{}
	reached := map[string]bool{}
	// The methods the standard library calls on a value it's given, as fmt a String, or http a ServeHTTP.
	called := map[string]bool{}
	for _, m := range []string{"String", "GoString", "Format", "Error", "Unwrap", "Is", "As", "MarshalJSON",
		"UnmarshalJSON", "MarshalText", "UnmarshalText", "Read", "Write", "Close", "ServeHTTP", "Len", "Less", "Swap"} {
		called[m] = true
	}
	var queue []string
	var walk func(from, name string)
	walk = func(from, name string) {
		if reached[name] || len(decls[name]) == 0 || slices.ContainsFunc(stop, func(s string) bool {
			return (name == s || strings.HasPrefix(name, s+".")) && !slices.Contains(start, name)
		}) {
			return
		}
		reached[name], via[name] = true, from
		queue = append(queue, name)
		for _, m := range methods[name] {
			if called[m] {
				walk(name, name+"."+m)
			}
		}
	}
	// call is a method called by its name, m, on whatever: each type reached that has one has it reached too, and
	// each type reached later.
	call := func(from, m string) {
		if called[m] {
			return
		}
		called[m] = true
		for _, typ := range ofName[m] {
			if reached[typ] {
				walk(from, typ+"."+m)
			}
		}
	}
	for _, s := range start {
		walk("", s)
	}
	chain := func(name string) []string {
		out := []string{name}
		for via[name] != "" {
			name = via[name]
			out = append([]string{name}, out...)
		}
		return out
	}
	var found []listenFound
	calls := map[token.Pos]string{} // each call, as written, by where its function starts
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, d := range decls[name] {
			imports := map[string]string{}
			for _, spec := range d.file.Imports {
				path := strings.Trim(spec.Path.Value, `"`)
				local := path[strings.LastIndex(path, "/")+1:]
				if spec.Name != nil {
					local = spec.Name.Name
				}
				imports[local] = path
			}
			var visit func(ast.Node) bool
			visit = func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.CallExpr:
					calls[n.Fun.Pos()] = types.ExprString(n)
				case *ast.SelectorExpr:
					if x, ok := n.X.(*ast.Ident); ok {
						if path, ok := imports[x.Name]; ok {
							if slices.Contains(banned[path], n.Sel.Name) {
								call := calls[n.Pos()]
								if call == "" {
									call = path + "." + n.Sel.Name + " (not called: kept to call later)"
								}
								found = append(found, listenFound{chain(name), call})
							}
							if _, ok := dirs[path]; ok {
								walk(name, path+"."+n.Sel.Name)
							}
							return false
						}
					}
					// x.field or x.method: x's type is reached already (it was named for x to be had).
					call(name, n.Sel.Name)
					ast.Inspect(n.X, visit)
					return false
				case *ast.Ident:
					walk(name, d.pkg+"."+n.Name)
				}
				return true
			}
			ast.Inspect(d.node, visit)
		}
	}
	return reached, found
}
