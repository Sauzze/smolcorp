//go:build windows

package desktop

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// processesWorkingIn is the processes working in folder (or a folder inside it), or given it with
// --add-dir, as T3 Code's agent is (T797a): each process's current folder and command line, read from its
// own parameters (its PEB) with read access only, nothing run. A process that can't be opened (another
// user's, a protected one) isn't T3 Code's agent, which runs as you. False when the processes can't be listed.
func processesWorkingIn(folder string) ([]int, bool) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, false
	}
	defer windows.CloseHandle(snapshot)
	folders := t3PlacesOf(folder)
	var found []int
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID == 0 {
			continue
		}
		if cwd, line, ok := processPlace(entry.ProcessID); ok {
			args, _ := windows.DecomposeCommandLine(line)
			if t3WorksIn(folders, cwd, args, `\`, true) {
				found = append(found, int(entry.ProcessID))
			}
		}
	}
	return found, true
}

// processPlace is a process's current folder and command line, as its parameters hold them.
func processPlace(pid uint32) (cwd, line string, ok bool) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return "", "", false
	}
	defer windows.CloseHandle(process)
	var basic windows.PROCESS_BASIC_INFORMATION
	if windows.NtQueryInformationProcess(process, windows.ProcessBasicInformation, unsafe.Pointer(&basic), uint32(unsafe.Sizeof(basic)), nil) != nil ||
		basic.PebBaseAddress == nil {
		return "", "", false
	}
	// Only as far as each field that's read: older systems' structures are shorter.
	var peb windows.PEB
	if !readProcess(process, uintptr(unsafe.Pointer(basic.PebBaseAddress)), unsafe.Pointer(&peb),
		unsafe.Offsetof(peb.ProcessParameters)+unsafe.Sizeof(peb.ProcessParameters)) || peb.ProcessParameters == nil {
		return "", "", false
	}
	var params windows.RTL_USER_PROCESS_PARAMETERS
	if !readProcess(process, uintptr(unsafe.Pointer(peb.ProcessParameters)), unsafe.Pointer(&params),
		unsafe.Offsetof(params.CommandLine)+unsafe.Sizeof(params.CommandLine)) {
		return "", "", false
	}
	cwd, okCwd := readProcessText(process, params.CurrentDirectory.DosPath)
	line, okLine := readProcessText(process, params.CommandLine)
	return cwd, line, okCwd && okLine
}

// readProcess reads size bytes at address in process into into; false unless all were read.
func readProcess(process windows.Handle, address uintptr, into unsafe.Pointer, size uintptr) bool {
	var read uintptr
	return windows.ReadProcessMemory(process, address, (*byte)(into), size, &read) == nil && read == size
}

// readProcessText is a string of process's, as its parameters point at it.
func readProcessText(process windows.Handle, text windows.NTUnicodeString) (string, bool) {
	if text.Length == 0 || text.Buffer == nil {
		return "", true
	}
	buffer := make([]uint16, text.Length/2)
	if !readProcess(process, uintptr(unsafe.Pointer(text.Buffer)), unsafe.Pointer(&buffer[0]), uintptr(text.Length/2*2)) {
		return "", false
	}
	return windows.UTF16ToString(buffer), true
}

// processName is a process's program, as the system lists it (node.exe…), or "".
func processName(pid int) string {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if int(entry.ProcessID) == pid {
			return windows.UTF16ToString(entry.ExeFile[:])
		}
	}
	return ""
}
