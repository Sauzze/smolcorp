//go:build windows

package desktop

import (
	"context"
	"errors"
	"log"
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// On Windows, `tailscale serve` takes no unix: target, so it hands a phone's request on to the serve port
// (servePort): a port of its own, on this computer only (127.0.0.1 and ::1), never the deck's (OLD-SERVE2). Windows'
// own tables say who holds the other end of each connection to it, as it's accepted:
//   - the service manager names the Tailscale service's process (by the service's name), and the program its
//     configuration runs, tailscaled.exe;
//   - the list of processes says that process runs tailscaled.exe, and when it started: before the accept;
//   - the TCP table (GetExtendedTcpTable, IPv4 or IPv6 as the connection is) has the client's end of this
//     connection, established, held by that process;
//   - the service is read again after the table, and is the same process.
//
// None of these needs a handle on Tailscale's process, which runs as SYSTEM, so the owner's Smolcorp can read them
// all. A client that sent its request and ended, or half-closed, before the accept has no established row, and is
// refused. A program the owner runs can't pass for the service: only an administrator can register or change one. A
// program running as administrator could, as it can do anything on this computer: setup and the release notes say so.
//
// The serve port is held exclusively (SO_EXCLUSIVEADDRUSE), so no other program, the owner's included, can bind it
// too while Smolcorp listens and be handed what Tailscale sends.

// soExclusiveAddrUse is SO_EXCLUSIVEADDRUSE: Winsock defines it as ~SO_REUSEADDR.
const soExclusiveAddrUse = ^windows.SO_REUSEADDR

// listenServeSocket opens the serve port as the app starts, for `tailscale serve`'s hand-on alone: each connection
// stamped with when it was accepted, and judged as it's taken (serveListener). A second Smolcorp can't take the port
// (held exclusively), and no instance lock is needed for it.
func listenServeSocket() (net.Listener, error) {
	if !serveHandOnHere() {
		return nil, nil
	}
	return openServePort(servePort())
}

// openServePort listens on port on 127.0.0.1, and on ::1 where there is IPv6 on loopback, each held exclusively.
func openServePort(port int) (net.Listener, error) {
	set := newListenerSet(port)
	set.listen, set.stamp = listenExclusive, true
	if err := set.add("127.0.0.1"); err != nil {
		set.Close()
		return nil, err
	}
	if err := set.add("::1"); err != nil {
		log.Printf("smolcorp: the serve port isn't on ::1: %v", err)
	}
	return &serveListener{Listener: set}, nil
}

// listenExclusive listens on a TCP address that no other socket can bind while it's open, whoever asks and with
// whatever option (SO_EXCLUSIVEADDRUSE): without it, Windows lets a program of the same user bind it again with
// SO_REUSEADDR, and share what is sent there.
func listenExclusive(address string) (net.Listener, error) {
	config := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var err error
		if controlErr := raw.Control(func(fd uintptr) {
			err = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, soExclusiveAddrUse, 1)
		}); controlErr != nil {
			return controlErr
		}
		return err
	}}
	return config.Listen(context.Background(), "tcp", address)
}

// identifyServePeerHere reads the other end of a connection to the serve port and judges it (judgeWindowsPeer): the
// service, then the client's row in the TCP table, then the service again.
func identifyServePeerHere(conn net.Conn) bool {
	accepted, ok := conn.(acceptedConn)
	if !ok {
		return false
	}
	from, fromTCP := conn.RemoteAddr().(*net.TCPAddr)
	to, toTCP := conn.LocalAddr().(*net.TCPAddr)
	if !fromTCP || !toTCP || !from.IP.IsLoopback() || !to.IP.IsLoopback() {
		return false
	}
	peer := windowsPeer{accepted: accepted.at, before: tailscaleServiceNow()}
	if peer.before.pid == 0 {
		return false
	}
	peer.row = clientRow(from, to)
	peer.after = tailscaleServiceNow()
	return judgeWindowsPeer(peer)
}

// tailscaleServiceNow is the Tailscale service as Windows says it is now: its process and program by the service
// manager, and that process in the process list. Zero when there is no such service running.
func tailscaleServiceNow() serviceProcess {
	pid, binary := serviceManagerSays(tailscaleService)
	if pid == 0 {
		return serviceProcess{}
	}
	image, created := listedProcess(pid)
	return serviceProcess{pid: pid, binary: binary, image: image, created: created}
}

// serviceManagerSays is a running service's process and the command line its configuration runs, by the service's
// name, as the service manager says (0 and "" for none, or one not running). Asked for status and configuration
// only, which every signed-in user may read. A variable so a test can stand a process of its own in.
var serviceManagerSays = func(name string) (uint32, string) {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return 0, ""
	}
	defer windows.CloseServiceHandle(manager)
	named, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, ""
	}
	service, err := windows.OpenService(manager, named, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return 0, ""
	}
	defer windows.CloseServiceHandle(service)
	var status windows.SERVICE_STATUS_PROCESS
	var needed uint32
	if err := windows.QueryServiceStatusEx(service, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafe.Pointer(&status)),
		uint32(unsafe.Sizeof(status)), &needed); err != nil || status.CurrentState != windows.SERVICE_RUNNING {
		return 0, ""
	}
	size := uint32(1 << 10)
	for range 4 {
		buffer := make([]uint64, (size+7)/8)
		config := (*windows.QUERY_SERVICE_CONFIG)(unsafe.Pointer(&buffer[0]))
		err := windows.QueryServiceConfig(service, config, uint32(len(buffer)*8), &needed)
		if err == nil {
			return status.ProcessId, windows.UTF16PtrToString(config.BinaryPathName)
		}
		if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) || needed > 64<<10 {
			return 0, ""
		}
		size = needed
	}
	return 0, ""
}

// processListMost is the most the process list is read into: more is nothing found.
const processListMost = 64 << 20

// listedProcess is a process's program by name and when it started (Unix nanoseconds), as Windows' list of
// processes says (NtQuerySystemInformation, SystemProcessInformation): "" and 0 if it isn't there. Read without a
// handle on the process, which the owner's Smolcorp can't open for one running as SYSTEM.
func listedProcess(pid uint32) (string, int64) {
	size := uint32(512 << 10)
	for size <= processListMost {
		buffer := make([]uint64, size/8)
		var needed uint32
		err := windows.NtQuerySystemInformation(windows.SystemProcessInformation, unsafe.Pointer(&buffer[0]), size, &needed)
		if errors.Is(err, windows.STATUS_INFO_LENGTH_MISMATCH) {
			size = max(size*2, (needed+(64<<10))&^7)
			continue
		}
		if err != nil {
			return "", 0
		}
		entrySize := uint32(unsafe.Sizeof(windows.SYSTEM_PROCESS_INFORMATION{}))
		for offset := uint32(0); offset+entrySize <= min(needed, size); {
			entry := (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(uintptr(unsafe.Pointer(&buffer[0])) + uintptr(offset)))
			if entry.UniqueProcessID == uintptr(pid) {
				created := windows.Filetime{LowDateTime: uint32(entry.CreateTime), HighDateTime: uint32(entry.CreateTime >> 32)}
				return entry.ImageName.String(), created.Nanoseconds()
			}
			if entry.NextEntryOffset == 0 {
				break
			}
			offset += entry.NextEntryOffset
		}
		return "", 0
	}
	return "", 0
}

// clientRow is the client's end of a connection from one address to another in Windows' TCP table: an established
// row if there is one, else any row for it (one half-closed, say). A variable so a test can change what was read.
var clientRow = func(from, to *net.TCPAddr) tcpRow {
	af, v6 := uint32(windows.AF_INET), from.IP.To4() == nil
	if v6 {
		af = windows.AF_INET6
	}
	table, ok := tcpTable(af, tcpTableOwnerPIDConnections)
	if !ok {
		return tcpRow{}
	}
	var found tcpRow
	for _, row := range windowsTCPRows(table, v6) {
		if row.localPort != from.Port || row.remotePort != to.Port || !row.localIP.Equal(from.IP) || !row.remoteIP.Equal(to.IP) {
			continue
		}
		if row.state == tcpEstablished {
			return tcpRow{found: true, state: row.state, pid: uint32(row.pid)}
		}
		if !found.found {
			found = tcpRow{found: true, state: row.state, pid: uint32(row.pid)}
		}
	}
	return found
}
