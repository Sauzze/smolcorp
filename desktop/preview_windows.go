//go:build windows

package desktop

import (
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getExtendedTCPTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

const (
	tcpTableOwnerPIDListener    = 3       // TCP_TABLE_OWNER_PID_LISTENER: listening sockets, with their process
	tcpTableOwnerPIDConnections = 4       // TCP_TABLE_OWNER_PID_CONNECTIONS: connections, with their process
	tcpTableMost                = 4 << 20 // bytes of one table; more is nothing found
)

// deskListeners is the TCP sockets listened on by a desk's process and every process under it but cut's, as Windows'
// own tables say: the processes from a Toolhelp snapshot, the listening sockets and their processes from the IP
// helper (GetExtendedTcpTable). A desk has no tty here: only its process starts it. Only read; nothing is connected
// to. false when it couldn't look.
//
// There is no per-user filter here, and a process whose parent has ended keeps that parent's pid, which Windows can
// give again: rarely, a process the desk's process didn't start could be taken as under it.
func deskListeners(root previewRoot, cut []int) ([]tcpListener, bool) {
	if root.pid <= 1 {
		return nil, true
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, false
	}
	defer windows.CloseHandle(snapshot)
	parents := map[int]int{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		parents[int(entry.ProcessID)] = int(entry.ParentProcessID)
	}
	if _, known := parents[root.pid]; !known {
		return nil, true
	}
	pids := setOf(descendants([]int{root.pid}, parents, setOf(cut)))
	var found []tcpListener
	for _, family := range []struct {
		af uint32
		v6 bool
	}{{windows.AF_INET, false}, {windows.AF_INET6, true}} {
		table, ok := tcpTable(family.af, tcpTableOwnerPIDListener)
		if !ok {
			return nil, false
		}
		found = append(found, parseWindowsTCPTable(table, family.v6, pids)...)
	}
	return found, true
}

// connectionOwner is the process holding the TCP connection from one address to another, as the IP helper says, or 0.
func connectionOwner(from, to *net.TCPAddr) int {
	af, v6 := uint32(windows.AF_INET), from.IP.To4() == nil
	if v6 {
		af = windows.AF_INET6
	}
	table, ok := tcpTable(af, tcpTableOwnerPIDConnections)
	if !ok {
		return 0
	}
	return parseWindowsTCPConnection(table, v6, from, to)
}

// tcpTable is GetExtendedTcpTable's table of one kind (class) and address family, as bytes.
func tcpTable(af uint32, class uintptr) ([]byte, bool) {
	size := uint32(16 << 10)
	for range 4 {
		table := make([]byte, size)
		status, _, _ := getExtendedTCPTable.Call(uintptr(unsafe.Pointer(&table[0])), uintptr(unsafe.Pointer(&size)), 0,
			uintptr(af), class, 0)
		switch {
		case status == 0:
			return table[:min(int(size), len(table))], true
		case windows.Errno(status) != windows.ERROR_INSUFFICIENT_BUFFER || size > tcpTableMost:
			return nil, false
		}
	}
	return nil, false
}
