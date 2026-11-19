//go:build windows

package desktop

import (
	"errors"
	"net"
	"net/http"
	"unsafe"

	"golang.org/x/sys/windows"
)

// askingProgramHere is which program sent a terminal's ask, as Windows says it (askprogram.go).
func askingProgramHere(r *http.Request) askAsker { return askerRead(r, askerOnWindows) }

// askerOnWindows is the program holding the client's end of the connection from one address to another on this
// computer, by its full path, and what started it, by name: "" if Windows can't say.
//   - The client's row in the TCP table is established, and names its process.
//   - That process is opened (for a limited query, which any program of the owner's allows), so its number can't pass
//     to another while it's read; the row is read again with the handle held, and must be the same.
//   - Its full path is read with that handle (QueryFullProcessImageName), and what started it from the process list.
func askerOnWindows(remote, local string) (string, []string) {
	from, fromErr := net.ResolveTCPAddr("tcp", remote)
	to, toErr := net.ResolveTCPAddr("tcp", local)
	if fromErr != nil || toErr != nil || !from.IP.IsLoopback() || !to.IP.IsLoopback() {
		return "", nil
	}
	row := clientRow(from, to)
	if !row.found || row.state != tcpEstablished || row.pid == 0 {
		return "", nil
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, row.pid)
	if err != nil {
		return "", nil
	}
	defer windows.CloseHandle(process)
	if clientRow(from, to) != row {
		return "", nil
	}
	program := processPath(process)
	if program == "" {
		return "", nil
	}
	return program, startedFrom(row.pid, processesListed())
}

// processPath is a process's program, by its full path, as Windows says it: "" if it can't be read.
func processPath(process windows.Handle) string {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if windows.QueryFullProcessImageName(process, 0, &buffer[0], &size) != nil || size == 0 || size > uint32(len(buffer)) {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}

// processesListed is Windows' list of processes (NtQuerySystemInformation, SystemProcessInformation, as
// peerowner_windows.go's listedProcess reads it): each one's program by name, when it started and its parent. Empty if
// it can't be read.
func processesListed() map[uint32]listedProc {
	list := map[uint32]listedProc{}
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
			return list
		}
		entrySize := uint32(unsafe.Sizeof(windows.SYSTEM_PROCESS_INFORMATION{}))
		for offset := uint32(0); offset+entrySize <= min(needed, size); {
			entry := (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(uintptr(unsafe.Pointer(&buffer[0])) + uintptr(offset)))
			created := windows.Filetime{LowDateTime: uint32(entry.CreateTime), HighDateTime: uint32(entry.CreateTime >> 32)}
			list[uint32(entry.UniqueProcessID)] = listedProc{name: entry.ImageName.String(), created: created.Nanoseconds(),
				parent: uint32(entry.InheritedFromUniqueProcessID)}
			if entry.NextEntryOffset == 0 {
				break
			}
			offset += entry.NextEntryOffset
		}
		return list
	}
	return list
}
