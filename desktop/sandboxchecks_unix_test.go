//go:build darwin || linux

package desktop

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

// A database's unix socket is tried as its port is: a Postgres that lets a login in there is found, with no TLS
// try (a socket has none), and a socket file nothing listens on any more, or none at all, is nothing there.
func TestADatabasesSocketIsTriedAsItsPortIs(t *testing.T) {
	quickChecks(t)
	dir := shortTempDir(t)
	sock := filepath.Join(dir, ".s.PGSQL.5432")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	s := &pgStandIn{answer: func(user, _ string, _ bool) []byte { return pgAuth(0) }}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				s.serve(conn)
			}()
		}
	}()
	onlyPlaces(t, dbPlace{"Postgres", "unix", sock}, dbPlace{"MySQL", "unix", filepath.Join(dir, "mysql.sock")})
	found := checkDatabases(ownAddressesReached())
	if len(found.LetIn) != 1 || found.LetIn[0] != "Postgres at "+sock+" as postgres" || len(found.CantTell) != 0 {
		t.Errorf("trust on the socket: %+v", found)
	}
	// Left as a file with nothing behind it: as on a Mac after Postgres stopped.
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()
	if found := checkDatabases(ownAddressesReached()); !found.passed() {
		t.Errorf("a socket file nothing listens on: %+v", found)
	}
}
