//go:build darwin

package desktop

import (
	"bytes"
	"encoding/binary"

	"golang.org/x/sys/unix"
)

// processesWorkingIn is the processes working in folder, or given it with --add-dir, as T3 Code's agent is
// (T797a). Here only a process's arguments can be read without cgo (kern.procargs2), not its working
// folder, which macOS keeps behind libproc: so the answer is "can't look" (false), and Let go keeps the
// fence and says why. The Mac app reads both (T3Desk.swift's Processes.working); the Go core on a Mac
// would ask it through the bridge (M1). Nothing is run.
func processesWorkingIn(folder string) ([]int, bool) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, false
	}
	folders := t3PlacesOf(folder)
	var found []int
	for _, p := range procs {
		pid := int(p.Proc.P_pid)
		if pid > 0 && t3WorksIn(folders, "", processArguments(pid), "/", false) {
			found = append(found, pid)
		}
	}
	return found, false
}

// processArguments is a process's arguments, from the kernel (kern.procargs2); nil when they can't be read.
func processArguments(pid int) []string {
	buffer, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(buffer) < 4 {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(buffer))
	i := 4
	// The program's path, then the NULs that pad it, then the arguments.
	for i < len(buffer) && buffer[i] != 0 {
		i++
	}
	for i < len(buffer) && buffer[i] == 0 {
		i++
	}
	var words []string
	for len(words) < count && i < len(buffer) {
		end := bytes.IndexByte(buffer[i:], 0)
		if end < 0 {
			end = len(buffer) - i
		}
		words = append(words, string(buffer[i:i+end]))
		i += end + 1
	}
	return words
}

// processName is a process's name as the kernel keeps it (zsh, node…), or "".
func processName(pid int) string {
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return ""
	}
	name := p.Proc.P_comm[:]
	if end := bytes.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}
	return string(name)
}
