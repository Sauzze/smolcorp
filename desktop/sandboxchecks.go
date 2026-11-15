package desktop

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// Port 22 and the local databases (SA-M12, SANDBOX-PLAN §A0.1 C4 and C5, AQ12 and AQ21): the two things a desk's
// sandbox depends on that the core finds out on this computer at each start, not from its own files.
//
//   - Remote Login. While port 22 answers, a key file in ~/.ssh is a way out (ssh to this computer as the owner, from
//     outside the sandbox), so the desk's Policy read-denies the keys and its profile the SSH agents (C5). Each start
//     dials the port again, on every address of this computer; a desk started while nothing answered and still
//     running once something does is tagged "restart to apply" (restartToApply).
//   - The local databases. Homebrew's and Postgres.app's Postgres, and MySQL's root, let you in without a password by
//     default, and a superuser there reads files and runs programs as the owner (C4). So their sockets and ports are
//     denied unless the owner said theirs asks for a password (AQ21's opt-in), and the opt-in counts only while the
//     core's own password-less login fails: tried when it's switched on, and again at each start. Every socket and
//     port the opt-in opens (the lists' DatabaseOptIn sockets and OpenOnOptIn ports) is tried, as the database's
//     superuser and as the owner's own login name, over TLS too where a refusal may only be the server asking for it.
//     The logins are written here by hand, the start of each protocol only, with no new module: nothing is sent but
//     a user name and an empty password, and nothing is read past the server's answer.
//
// Each check fails closed: something on port 22 that can't be told apart from an SSH server counts as one, and a
// database that answers but whose answer can't be read (a timeout, a protocol it isn't, an error that isn't about the
// login) counts as one that lets you in. Only an address of this computer that this computer can't reach itself on
// (Tailscale's IPv6 address on a Mac) decides nothing: ownAddressesReached leaves it out before anything is dialed.
// That is also what a firewall or content filter that silently drops the core's connections to its own addresses
// (rather than refusing them) gets: those addresses are left out, while loopback, which servers on every address
// listen on too, is always dialed.

// remoteLoginPort is the port Remote Login listens on (a test's stand-in port otherwise).
var remoteLoginPort = 22

// checkWait is the longest a check waits, all its tries together; checkTryWait the longest for one connection.
var (
	checkWait    = 5 * time.Second
	checkTryWait = 2 * time.Second
)

// ownAddresses are this computer's addresses: both loopbacks, and every address of its own but a link-local IPv6 one
// (which needs a zone, and isn't one a server listens on alone). A check dials those of them it reaches
// (ownAddressesReached).
var ownAddresses = func() []string {
	out := []string{"127.0.0.1", "::1"}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || slices.Contains(out, ip.String()) {
			continue
		}
		out = append(out, ip.String())
	}
	return out
}

// checkDial is how the checks connect (a test's stand-in network otherwise). Nothing the checks run opens a listener
// (TestNothingTheSandboxChecksRunOpensAListener): a listener on an address of the computer's own may have its
// firewall ask the owner whether to accept incoming connections, and a test copy, which listens on this computer
// only, must never have it ask (RT126SAM12R3).
var checkDial = func(ctx context.Context, network, address string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, address)
}

// probeWait is the longest the core waits for an answer from one of this computer's own addresses. That connection
// never leaves this computer, so wherever the address can be reached here, it is answered within a millisecond or
// two (the Mac Studio's lancheck, 15 Nov: LAN and Tailscale v4 refused or connected in 0–2 ms on every port).
var probeWait = 500 * time.Millisecond

// reachClosedPort is the port the reach probe dials where the core doesn't listen on every address: 1, tcpmux,
// which neither macOS nor Linux runs, so the kernel refuses it. It is under 1024: on Linux only root may listen on
// it, so nothing a desk runs can hold its answer back.
const reachClosedPort = 1

// reachPortFor is the port the reach probe dials on each address, with the core listening on scope at port. Where it
// listens on every address (listenAll, the Mac's default), its own port: the core's own listener answers there, a
// desk can't take that port on every address, and one that took it on one address would take the office off that
// address for the owner's phone too. Else reachClosedPort. A connection and a refusal both say the address was
// reached, so all a desk could do there is keep an answer back, the one thing that leaves an address out.
func reachPortFor(scope string, port int) int {
	if scope == listenAll && port > 0 {
		return port
	}
	return reachClosedPort
}

// reachPort is the port the reach probe dials now (a test's stand-in port otherwise).
var reachPort = func() int { return reachPortFor(listenScope, Port) }

// ownAddressesReached is ownAddresses less those this computer can't reach itself on (unreachedHere), each tried at
// once: the addresses a check dials, worked out once per check and shared by its parts.
func ownAddressesReached() []string {
	addrs := ownAddresses()
	port := reachPort()
	unreached := make([]bool, len(addrs))
	var wg sync.WaitGroup
	for i, addr := range addrs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unreached[i] = unreachedHere(runtime.GOOS, addr, port)
		}()
	}
	wg.Wait()
	var out []string
	for i, addr := range addrs {
		if !unreached[i] {
			out = append(out, addr)
		}
	}
	return out
}

// unreachedHere is whether addr, one of this computer's addresses on goos, can't be reached from this computer: a
// dial to its port (reachPort) gets no answer within probeWait, or finds no route (Tailscale's IPv6 address on a Mac,
// measured on macOS 27: every port times out there, the open 22 too, where the LAN and Tailscale's IPv4 address
// refuse or connect in milliseconds). Whether an address of one's own is reached is up to the route and the
// interface, not to what listens there or who dials, and a desk can't change the routes: a server listening only
// there can't be reached by a desk on this computer either, so the address decides nothing and isn't dialed.
// Anything else keeps it, fail closed: a loopback (always reached), a connection, a refusal and every other answer
// (unreachedErr), and every address on Windows, which wasn't measured and whose system errors this doesn't read.
func unreachedHere(goos, addr string, port int) bool {
	ip := net.ParseIP(addr)
	if ip == nil || ip.IsLoopback() || goos == "windows" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeWait)
	defer cancel()
	conn, err := checkDial(ctx, "tcp", net.JoinHostPort(addr, fmt.Sprint(port)))
	if err == nil {
		conn.Close()
		return false
	}
	return unreachedErr(err)
}

// unreachedErr is whether a dial's error says the address wasn't reached: it timed out (Go's dialer says so as
// context.DeadlineExceeded, the system as ETIMEDOUT), or no route leads there.
func unreachedErr(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EHOSTUNREACH)
}

// nothingThere is whether a dial's error on network says nothing listens there (no socket file, a refusal, an
// address that isn't this computer's any more), as opposed to something there that didn't answer in time. A socket
// file the core may not open is nothing there for a desk either; a TCP dial refused permission isn't: a filter that
// stops only the core (Little Snitch, LuLu) may let a desk's ssh through, so it counts as an answer.
func nothingThere(network string, err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case syscall.ECONNREFUSED, syscall.ENOENT, syscall.EADDRNOTAVAIL, syscall.ENETUNREACH, syscall.EHOSTUNREACH:
		return true
	case syscall.EACCES:
		return network == "unix"
	}
	// Windows' own numbers for the same: WSAECONNREFUSED, WSAEADDRNOTAVAIL, WSAENETUNREACH, WSAEHOSTUNREACH.
	return runtime.GOOS == "windows" && slices.Contains([]syscall.Errno{10061, 10049, 10051, 10065}, errno)
}

// remoteLoginAnswers is whether port 22 answers on any of own (ownAddressesReached): true for a connection, and for
// any dial that neither connects nor says nothing is there (fail closed).
func remoteLoginAnswers(own []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), checkTryWait)
	defer cancel()
	answers := make(chan bool, len(own))
	for _, addr := range own {
		go func() {
			conn, err := checkDial(ctx, "tcp", net.JoinHostPort(addr, fmt.Sprint(remoteLoginPort)))
			if err == nil {
				conn.Close()
			}
			answers <- err == nil || !nothingThere("tcp", err)
		}()
	}
	answered := false
	for range own {
		answered = <-answers || answered
	}
	return answered
}

// dbPlace is where a local database may listen: its kind (the protocol tried), and a unix socket or a TCP address.
type dbPlace struct{ Kind, Network, Address string }

func (p dbPlace) String() string { return fmt.Sprintf("%s at %s", p.Kind, p.Address) }

// databasePortKinds is the database each port the opt-in opens is (sandbox.LocalPorts' OpenOnOptIn).
var databasePortKinds = map[int]string{5432: "Postgres", 3306: "MySQL"}

// databasePlaces is every place on goos the opt-in opens to a desk: each DatabaseOptIn socket of the lists, and each
// OpenOnOptIn port on each of own (ownAddressesReached). The tests give their stand-ins instead.
var databasePlaces = func(goos string, own []string) []dbPlace {
	var out []dbPlace
	for _, s := range sandbox.Sockets(goos) {
		if s.Need == sandbox.DatabaseOptIn {
			out = append(out, dbPlace{s.Name, "unix", s.At})
		}
	}
	for _, p := range sandbox.LocalPorts() {
		if p.OpenOnOptIn {
			for _, addr := range own {
				out = append(out, dbPlace{databasePortKinds[p.Port], "tcp", net.JoinHostPort(addr, fmt.Sprint(p.Port))})
			}
		}
	}
	return out
}

// ownerLogin is the owner's login name: Homebrew's Postgres makes it the superuser, and MariaDB a login by socket.
// The tests give a made-up one, so the real name never ends up in their output.
var ownerLogin = func() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

// databaseLogins are the logins tried on a database of kind: its superuser, and the owner's own name.
func databaseLogins(kind string) []string {
	super := map[string]string{"Postgres": "postgres", "MySQL": "root"}[kind]
	out := []string{super}
	if own := ownerLogin(); own != "" && own != super {
		out = append(out, own)
	}
	return out
}

// dbSays is what one place said to a password-less login, the worst last.
type dbSays int8

const (
	dbNone     dbSays = iota // nothing listens there
	dbShut                   // it asked for a password, or refused the login
	dbCantTell               // something answered, but not so it could be read
	dbLetsIn                 // it let the login in
)

// databaseCheck is what a check of every place found: the places that let a login in, and those that couldn't be
// read, each in words.
type databaseCheck struct {
	LetIn, CantTell []string
}

// passed is whether the opt-in may count: no place let a password-less login in, and every place could be read.
func (c databaseCheck) passed() bool { return len(c.LetIn) == 0 && len(c.CantTell) == 0 }

// refusal is what the owner is told when it didn't pass, "" when it did.
func (c databaseCheck) refusal() string {
	where := map[bool]string{true: "this Mac", false: "this computer"}[runtime.GOOS == "darwin"]
	switch {
	case len(c.LetIn) > 0:
		return fmt.Sprintf("Smolcorp logged in to %s without a password, so desks still can't reach the databases on %s. "+
			"Give that login a password, then try again.", c.LetIn[0], where)
	case len(c.CantTell) > 0:
		return fmt.Sprintf("Smolcorp couldn't tell whether %s asks for a password, so desks still can't reach the databases on %s. "+
			"Try again once it's running normally.", c.CantTell[0], where)
	}
	return ""
}

// checkDatabases tries a password-less login at every place on own (ownAddressesReached), all at once, within
// checkWait.
func checkDatabases(own []string) databaseCheck {
	ctx, cancel := context.WithTimeout(context.Background(), checkWait)
	defer cancel()
	places := databasePlaces(runtime.GOOS, own)
	type found struct {
		place dbPlace
		login string
		says  dbSays
	}
	results := make([]found, len(places))
	var wg sync.WaitGroup
	for i, place := range places {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = found{place, "", dbNone}
			for _, login := range databaseLogins(place.Kind) {
				says := tryDatabase(ctx, place, login)
				if says > results[i].says {
					results[i] = found{place, login, says}
				}
				if says == dbNone || says >= dbCantTell {
					return // nothing there, or nothing more to find out
				}
			}
		}()
	}
	wg.Wait()
	var c databaseCheck
	for _, r := range results {
		switch r.says {
		case dbLetsIn:
			c.LetIn = append(c.LetIn, fmt.Sprintf("%s as %s", r.place, r.login))
		case dbCantTell:
			c.CantTell = append(c.CantTell, r.place.String())
		}
	}
	return c
}

// tryDatabase is what place says to login with no password: in the clear, then over TLS when the clear one was shut
// on a TCP port (a server may refuse a login only for not asking for TLS: Postgres' hostssl, MySQL's
// require_secure_transport). A kind with no login written here can't be told.
func tryDatabase(ctx context.Context, place dbPlace, login string) dbSays {
	try := map[string]func(context.Context, dbPlace, string, bool) dbSays{"Postgres": postgresLogin, "MySQL": mysqlLogin}[place.Kind]
	if try == nil {
		return dbCantTell
	}
	says := try(ctx, place, login, false)
	if says == dbShut && place.Network == "tcp" {
		says = max(says, try(ctx, place, login, true))
	}
	return says
}

// dialDatabase connects to place, with a deadline: nil and dbNone when nothing is there, dbCantTell for a dial that
// neither connected nor said so.
func dialDatabase(ctx context.Context, place dbPlace) (net.Conn, dbSays) {
	conn, err := checkDial(ctx, place.Network, place.Address)
	if err != nil {
		if nothingThere(place.Network, err) {
			return nil, dbNone
		}
		return nil, dbCantTell
	}
	deadline := time.Now().Add(checkTryWait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	return conn, dbShut
}

// overTLS starts TLS on conn. Nothing secret goes over it and nothing it says is trusted but whether a login got in,
// so the server's certificate isn't checked (a local database's is most often its own).
func overTLS(conn net.Conn) (net.Conn, bool) {
	tc := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) //nolint:gosec // see above
	return tc, tc.Handshake() == nil
}

// postgresLogin is Postgres' answer to a startup as login, to its own database and to postgres (pg_hba.conf may
// let a login in to one only: sameuser), each through SSLRequest first when withTLS. It reads up to the first
// authentication message: AuthenticationOk lets in, any other asks for something (a password, SCRAM, Kerberos), and an
// error is shut when its SQLSTATE is class 28 (the login refused), lets in when it's 3D000 (no such database: Postgres
// looks for the database only once the login is in), and can't be told otherwise (starting up, too many connections).
func postgresLogin(ctx context.Context, place dbPlace, login string, withTLS bool) dbSays {
	worst := dbNone
	for _, database := range slices.Compact([]string{login, "postgres"}) {
		says := postgresStartup(ctx, place, login, database, withTLS)
		worst = max(worst, says)
		if says == dbNone || says >= dbCantTell {
			break
		}
	}
	return worst
}

func postgresStartup(ctx context.Context, place dbPlace, login, database string, withTLS bool) dbSays {
	conn, says := dialDatabase(ctx, place)
	if conn == nil {
		return says
	}
	defer conn.Close()
	if withTLS {
		if _, err := conn.Write([]byte{0, 0, 0, 8, 4, 210, 22, 47}); err != nil { // SSLRequest: 80877103
			return dbCantTell
		}
		answer := make([]byte, 1)
		if _, err := io.ReadFull(conn, answer); err != nil {
			return dbCantTell
		}
		if answer[0] == 'N' {
			return dbNone // no TLS here: the clear login said it all
		}
		if answer[0] != 'S' {
			return dbCantTell
		}
		ok := false
		if conn, ok = overTLS(conn); !ok {
			return dbCantTell
		}
	}
	body := []byte{0, 3, 0, 0} // protocol 3.0
	for _, kv := range []string{"user", login, "database", database, "application_name", "smolcorp-check"} {
		body = append(append(body, kv...), 0)
	}
	body = append(body, 0)
	if _, err := conn.Write(append(binary.BigEndian.AppendUint32(nil, uint32(len(body)+4)), body...)); err != nil {
		return dbCantTell
	}
	for range 4 {
		head := make([]byte, 5)
		if _, err := io.ReadFull(conn, head); err != nil {
			return dbCantTell
		}
		size := binary.BigEndian.Uint32(head[1:])
		if size < 4 || size > 1<<16 {
			return dbCantTell
		}
		msg := make([]byte, size-4)
		if _, err := io.ReadFull(conn, msg); err != nil {
			return dbCantTell
		}
		switch head[0] {
		case 'R':
			if len(msg) < 4 {
				return dbCantTell
			}
			if binary.BigEndian.Uint32(msg) == 0 {
				_, _ = conn.Write([]byte{'X', 0, 0, 0, 4}) // Terminate
				return dbLetsIn
			}
			return dbShut
		case 'E':
			switch state := postgresSQLState(msg); {
			case strings.HasPrefix(state, "28"):
				return dbShut
			case state == "3D000":
				return dbLetsIn
			}
			return dbCantTell
		case 'v', 'N': // NegotiateProtocolVersion, a notice: the answer is still to come
		default:
			return dbCantTell
		}
	}
	return dbCantTell
}

// postgresSQLState is an ErrorResponse's SQLSTATE (its 'C' field), "" for none.
func postgresSQLState(msg []byte) string {
	for len(msg) > 1 && msg[0] != 0 {
		field, rest, _ := strings.Cut(string(msg[1:]), "\x00")
		if msg[0] == 'C' {
			return field
		}
		msg = []byte(rest)
	}
	return ""
}

// MySQL's capability flags the login uses.
const (
	mysqlLongPassword     = 0x1
	mysqlProtocol41       = 0x200
	mysqlSSL              = 0x800
	mysqlSecureConnection = 0x8000
	mysqlPluginAuth       = 0x80000
)

// mysqlShutCodes are the errors that refuse the login itself: access denied (1045, 1698 for a login by socket that
// isn't the owner's), a host not let in (1130), a connection that must use TLS (3159). A plugin this login can't
// speak (1251) isn't one: the server couldn't hear the login out, so it can't be told.
var mysqlShutCodes = []uint16{1045, 1698, 1130, 3159}

// mysqlLogin is MySQL's (or MariaDB's) answer to a login as login with an empty password, through TLS when withTLS.
// It reads the server's greeting, answers it with the server's own auth plugin and an empty auth response, and then
// follows an auth switch (an empty response again, or the one byte an empty clear password is) until OK, which lets
// in, or an error. caching_sha2_password asking for full authentication wants the password: shut.
func mysqlLogin(ctx context.Context, place dbPlace, login string, withTLS bool) dbSays {
	conn, says := dialDatabase(ctx, place)
	if conn == nil {
		return says
	}
	defer func() { conn.Close() }()
	seq, greeting, ok := mysqlRead(conn)
	if !ok || len(greeting) == 0 {
		return dbCantTell
	}
	if greeting[0] == 0xFF {
		return mysqlError(greeting)
	}
	caps, plugin, ok := mysqlGreeting(greeting)
	if !ok {
		return dbCantTell
	}
	want := uint32(mysqlLongPassword|mysqlProtocol41|mysqlSecureConnection|mysqlPluginAuth) & (caps | mysqlProtocol41)
	fixed := func(flags uint32) []byte {
		out := binary.LittleEndian.AppendUint32(nil, flags)
		out = binary.LittleEndian.AppendUint32(out, 1<<24)
		return append(append(out, 33), make([]byte, 23)...) // utf8, then the filler
	}
	if withTLS {
		if caps&mysqlSSL == 0 {
			return dbNone // no TLS here: the clear login said it all
		}
		want |= mysqlSSL
		seq++
		if !mysqlWrite(conn, seq, fixed(want)) {
			return dbCantTell
		}
		if conn, ok = overTLS(conn); !ok {
			return dbCantTell
		}
	}
	answer := append(append(fixed(want), login...), 0, 0) // the login, then an empty auth response
	if want&mysqlPluginAuth != 0 {
		answer = append(append(answer, plugin...), 0)
	}
	seq++
	if !mysqlWrite(conn, seq, answer) {
		return dbCantTell
	}
	for range 4 {
		var reply []byte
		if seq, reply, ok = mysqlRead(conn); !ok || len(reply) == 0 {
			return dbCantTell
		}
		switch reply[0] {
		case 0x00:
			return dbLetsIn
		case 0xFF:
			return mysqlError(reply)
		case 0x01: // more auth data: caching_sha2_password's 3 is a fast auth that passed, OK follows
			if len(reply) < 2 || reply[1] != 3 {
				return dbShut
			}
		case 0xFE:
			name, _, found := strings.Cut(string(reply[1:]), "\x00")
			if !found {
				return dbCantTell // the old protocol's switch to its pre-4.1 password
			}
			empty := []byte{}
			if name == "mysql_clear_password" || name == "sha256_password" {
				empty = []byte{0}
			}
			seq++
			if !mysqlWrite(conn, seq, empty) {
				return dbCantTell
			}
		default:
			return dbCantTell
		}
	}
	return dbCantTell
}

// mysqlGreeting is a protocol 10 greeting's capability flags and auth plugin.
func mysqlGreeting(g []byte) (caps uint32, plugin string, ok bool) {
	if g[0] != 10 {
		return 0, "", false
	}
	_, rest, found := strings.Cut(string(g[1:]), "\x00") // the server's version
	if !found || len(rest) < 4+8+1+2 {
		return 0, "", false
	}
	caps = uint32(binary.LittleEndian.Uint16([]byte(rest[13:15])))
	rest = rest[15:]
	plugin = "mysql_native_password"
	if len(rest) >= 1+2+2+1+10 {
		caps |= uint32(binary.LittleEndian.Uint16([]byte(rest[3:5]))) << 16
		dataLen := int(rest[5])
		rest = rest[16:]
		if caps&mysqlSecureConnection != 0 {
			skip := max(13, dataLen-8)
			if len(rest) < skip {
				return 0, "", false
			}
			rest = rest[skip:]
		}
		if name, _, _ := strings.Cut(rest, "\x00"); caps&mysqlPluginAuth != 0 && name != "" {
			plugin = name
		}
	}
	return caps, plugin, true
}

// mysqlError is what an ERR packet says: shut for the login's own refusals, can't tell for anything else.
func mysqlError(p []byte) dbSays {
	if len(p) >= 3 && slices.Contains(mysqlShutCodes, binary.LittleEndian.Uint16(p[1:3])) {
		return dbShut
	}
	return dbCantTell
}

// mysqlRead reads one packet: its sequence number and its payload.
func mysqlRead(conn net.Conn) (byte, []byte, bool) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return 0, nil, false
	}
	size := int(head[0]) | int(head[1])<<8 | int(head[2])<<16
	if size > 1<<16 {
		return 0, nil, false
	}
	payload := make([]byte, size)
	_, err := io.ReadFull(conn, payload)
	return head[3], payload, err == nil
}

func mysqlWrite(conn net.Conn, seq byte, payload []byte) bool {
	n := len(payload)
	_, err := conn.Write(append([]byte{byte(n), byte(n >> 8), byte(n >> 16), seq}, payload...))
	return err == nil
}

// sandboxChecked is what one desk's start found, as its sandbox takes it (SA-M6 fills them in): RemoteLogin is
// Places.RemoteLogin; Database is the `database` of sandbox.DeniedPorts and sandbox.SocketPaths.
type sandboxChecked struct {
	RemoteLogin bool
	Database    bool
}

// The checks as found: each running desk's at its start, by id; and the last database check's, at a start or the
// opt-in, and Remote Login's last answer with when, for the tags.
var checked = struct {
	sync.Mutex
	desks          map[string]sandboxChecked
	databaseShut   bool
	remoteLogin    bool
	remoteLoginAt  time.Time
	remoteLoginNew time.Duration
}{desks: map[string]sandboxChecked{}, remoteLoginNew: 30 * time.Second}

// deskStartChecks is what desk id's start finds, at each start before its sandbox is made: whether port 22 answers,
// and, when the owner's opt-in is on, whether every local database refuses a password-less login. Kept for the
// desk's tags while it runs (deskChecksForgotten at its stop).
func deskStartChecks(id string, optIn bool) sandboxChecked {
	own := ownAddressesReached()
	c := sandboxChecked{RemoteLogin: remoteLoginAnswers(own)}
	if optIn {
		c.Database = checkDatabases(own).passed()
	}
	checked.Lock()
	defer checked.Unlock()
	checked.remoteLogin, checked.remoteLoginAt = c.RemoteLogin, time.Now()
	if optIn {
		checked.databaseShut = !c.Database
	}
	checked.desks[id] = c
	return c
}

// deskChecksForgotten is said at desk id's stop.
func deskChecksForgotten(id string) {
	checked.Lock()
	delete(checked.desks, id)
	checked.Unlock()
}

// restartToApply is the running desks whose sandbox lets them do more than one started now would, each with why:
// started while port 22 didn't answer, which it does now (dialed again when the last answer is older than
// remoteLoginNew), or started with the local databases open, which the opt-in (optIn, the owner's setting now) or the
// last check no longer allows. The desk keeps running as it is: the owner restarts it (Smolcorp never stops a desk
// itself).
func restartToApply(optIn bool) map[string]string {
	checked.Lock()
	stale := time.Since(checked.remoteLoginAt) >= checked.remoteLoginNew
	checked.Unlock()
	if stale {
		answers := remoteLoginAnswers(ownAddressesReached())
		checked.Lock()
		checked.remoteLogin, checked.remoteLoginAt = answers, time.Now()
		checked.Unlock()
	}
	checked.Lock()
	defer checked.Unlock()
	sshWords := "Restart to apply: an SSH server answers on this computer now, and this desk started before it did, so it can still use your SSH keys."
	if runtime.GOOS == "darwin" {
		sshWords = "Restart to apply: Remote Login was turned on after this desk started, so it can still use your SSH keys."
	}
	tags := map[string]string{}
	for id, c := range checked.desks {
		switch {
		case checked.remoteLogin && !c.RemoteLogin:
			tags[id] = sshWords
		case c.Database && (!optIn || checked.databaseShut):
			tags[id] = "Restart to apply: this desk started while your local databases were allowed, so it can still reach them."
		}
	}
	return tags
}

// setDatabaseOptIn is AQ21's opt-in, "my local database asks for a password", switched by the owner (Setup's row,
// SA-X14): on counts only once a password-less login has failed at every place (tried before the lock is taken, as
// it waits on the databases), and is refused with what was found otherwise. Off takes effect at each desk's next
// start, and tags the running ones that started with it on.
func (d *Deck) setDatabaseOptIn(on string) (int, string) {
	if on != "0" && on != "1" {
		return http.StatusBadRequest, "Say on=0 or on=1"
	}
	if on == "1" {
		found := checkDatabases(ownAddressesReached())
		checked.Lock()
		checked.databaseShut = !found.passed()
		checked.Unlock()
		if words := found.refusal(); words != "" {
			return http.StatusConflict, words
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if refused := d.config.heldRefusal(func(c *Config) { c.LocalDatabaseAsksPassword = on == "1" }); refused != "" {
		return http.StatusConflict, refused
	}
	d.config.LocalDatabaseAsksPassword = on == "1"
	d.config.save()
	d.changed()
	return http.StatusNoContent, ""
}
