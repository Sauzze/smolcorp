//go:build windows

package desktop

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Another process's structures, as far as what's read, its addresses kept as numbers: x/sys's types hold Go
// pointers, meant for this process's own, and the garbage collector mustn't take another's addresses for its
// own (RM5a N2). Laid out as x/sys's, which the checks below hold at build time.

// processText is a UNICODE_STRING of another process's: Length in bytes.
type processText struct {
	Length        uint16
	MaximumLength uint16
	Buffer        uintptr
}

type processBasics struct {
	ExitStatus                   windows.NTStatus
	PebBaseAddress               uintptr
	AffinityMask                 uintptr
	BasePriority                 int32
	UniqueProcessId              uintptr
	InheritedFromUniqueProcessId uintptr
}

// processEnvironment is a PEB as far as its parameters.
type processEnvironment struct {
	_                 [4]byte
	_                 uintptr
	ImageBaseAddress  uintptr
	Ldr               uintptr
	ProcessParameters uintptr
}

// processParameters are a process's parameters as far as its command line.
type processParameters struct {
	MaximumLength, Length                        uint32
	Flags, DebugFlags                            uint32
	ConsoleHandle                                windows.Handle
	ConsoleFlags                                 uint32
	StandardInput, StandardOutput, StandardError windows.Handle
	CurrentDirectory                             processText
	CurrentDirectoryHandle                       windows.Handle
	DllPath                                      processText
	ImagePathName                                processText
	CommandLine                                  processText
}

// Each is x/sys's layout: a difference makes an array of the wrong length (or a negative one), so it won't build.
var (
	_ [unsafe.Sizeof(windows.PROCESS_BASIC_INFORMATION{})]byte                      = [unsafe.Sizeof(processBasics{})]byte{}
	_ [unsafe.Offsetof(windows.PROCESS_BASIC_INFORMATION{}.PebBaseAddress)]byte     = [unsafe.Offsetof(processBasics{}.PebBaseAddress)]byte{}
	_ [unsafe.Offsetof(windows.PEB{}.ProcessParameters)]byte                        = [unsafe.Offsetof(processEnvironment{}.ProcessParameters)]byte{}
	_ [unsafe.Offsetof(windows.RTL_USER_PROCESS_PARAMETERS{}.CurrentDirectory)]byte = [unsafe.Offsetof(processParameters{}.CurrentDirectory)]byte{}
	_ [unsafe.Offsetof(windows.RTL_USER_PROCESS_PARAMETERS{}.CommandLine)]byte      = [unsafe.Offsetof(processParameters{}.CommandLine)]byte{}
	_ [unsafe.Sizeof(windows.NTUnicodeString{})]byte                                = [unsafe.Sizeof(processText{})]byte{}
	_ [unsafe.Offsetof(windows.NTUnicodeString{}.Buffer)]byte                       = [unsafe.Offsetof(processText{}.Buffer)]byte{}
)

// processesWorkingIn is the processes working in folder (or a folder inside it), or given it with
// --add-dir, as T3 Code's agent is (T797a): each process's current folder and command line, read from its
// own parameters (its PEB) with read access only, nothing run. A process that can't be opened (another
// user's, a protected one) isn't T3 Code's agent, which runs as you. False when the processes can't be listed,
// or the listing stops part way (RM5a N3).
func processesWorkingIn(folder string) ([]int, bool) {
	folders := t3PlacesOf(folder)
	return processesWhere(func(cwd string, args []string) bool { return t3WorksIn(folders, cwd, args, `\`, true) })
}

// processesHolding is the processes that work in folder as processesWorkingIn finds them, or whose command line
// names Claude Code session session (Hand back's check, t3takeover.go).
func processesHolding(folder, session string) ([]int, bool) {
	folders := t3PlacesOf(folder)
	return processesWhere(func(cwd string, args []string) bool {
		return t3WorksIn(folders, cwd, args, `\`, true) || t3NamesSession(args, session)
	})
}

// processesWhere is the processes whose current folder and command line match, read as processPlace reads them.
func processesWhere(match func(cwd string, args []string) bool) ([]int, bool) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, false
	}
	defer windows.CloseHandle(snapshot)
	var found []int
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID == 0 {
			continue
		}
		if cwd, line, ok := processPlace(entry.ProcessID); ok {
			args, _ := windows.DecomposeCommandLine(line)
			if match(cwd, args) {
				found = append(found, int(entry.ProcessID))
			}
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, false
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
	var basic processBasics
	if windows.NtQueryInformationProcess(process, windows.ProcessBasicInformation, unsafe.Pointer(&basic), uint32(unsafe.Sizeof(basic)), nil) != nil ||
		basic.PebBaseAddress == 0 {
		return "", "", false
	}
	// Only as far as each field that's read: older systems' structures are shorter.
	var peb processEnvironment
	if !readProcess(process, basic.PebBaseAddress, unsafe.Pointer(&peb), unsafe.Sizeof(peb)) || peb.ProcessParameters == 0 {
		return "", "", false
	}
	var params processParameters
	if !readProcess(process, peb.ProcessParameters, unsafe.Pointer(&params), unsafe.Sizeof(params)) {
		return "", "", false
	}
	cwd, okCwd := readProcessText(process, params.CurrentDirectory)
	line, okLine := readProcessText(process, params.CommandLine)
	return cwd, line, okCwd && okLine
}

// readProcess reads size bytes at address in process into into; false unless all were read.
func readProcess(process windows.Handle, address uintptr, into unsafe.Pointer, size uintptr) bool {
	var read uintptr
	return windows.ReadProcessMemory(process, address, (*byte)(into), size, &read) == nil && read == size
}

// readProcessText is a string of process's, as its parameters point at it. Less than one character (a
// length of 1, which only a process rewriting its own parameters gives) is none (RM5a N1).
func readProcessText(process windows.Handle, text processText) (string, bool) {
	if text.Length < 2 || text.Buffer == 0 {
		return "", true
	}
	buffer := make([]uint16, text.Length/2)
	if !readProcess(process, text.Buffer, unsafe.Pointer(&buffer[0]), uintptr(len(buffer)*2)) {
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
