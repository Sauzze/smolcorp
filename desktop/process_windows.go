//go:build windows

package desktop

import (
	"syscall"
	"unsafe"
)

// On Windows, what's in front of a pane can't be looked at, so its agent
// isn't stopped: firing leaves the pane open.
func stopGroup(group int) bool { return false }

// processGone is whether no process has the pid, or the one that had it has
// exited.
func processGone(pid int) bool {
	if pid <= 0 {
		return false
	}
	const queryLimitedInformation, stillActive, invalidParameter = 0x1000, 259, syscall.Errno(87)
	handle, err := syscall.OpenProcess(queryLimitedInformation, false, uint32(pid))
	if err != nil {
		return err == invalidParameter // no such process; denied means it's there
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	return syscall.GetExitCodeProcess(handle, &code) == nil && code != stillActive
}

// processParent is a process's parent and the name of its program, without
// the .exe, from a snapshot of the processes.
func processParent(pid int) (parent int, name string, ok bool) {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, "", false
	}
	defer syscall.CloseHandle(snapshot)
	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = syscall.Process32First(snapshot, &entry); err == nil; err = syscall.Process32Next(snapshot, &entry) {
		if int(entry.ProcessID) == pid {
			name := syscall.UTF16ToString(entry.ExeFile[:])
			if len(name) > 4 && (name[len(name)-4:] == ".exe" || name[len(name)-4:] == ".EXE") {
				name = name[:len(name)-4]
			}
			return int(entry.ParentProcessID), name, true
		}
	}
	return 0, "", false
}
