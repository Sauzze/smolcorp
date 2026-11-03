//go:build windows

package desktop

import (
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createSuspended, createNewProcessGroup, createNoWindow = 0x00000004, 0x00000200, 0x08000000

// ownGroup starts a summary's process in a process group of its own, with no
// window, keeping a command line already given (see useCommandLine), and
// suspended: joinGroup puts it in its job before it runs at all, so nothing
// it starts is ever outside it.
func ownGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createSuspended | createNewProcessGroup | createNoWindow
	cmd.SysProcAttr.HideWindow = true
}

// useCommandLine gives cmd.exe the command line cmdLine made, as it is.
func useCommandLine(cmd *exec.Cmd, line string) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CmdLine = line
}

// processGroup is a summary's process and all it started: a job object that
// kills them all when it's closed. Never a pid, which may be another
// program's once the process has gone.
type processGroup struct {
	job windows.Handle
	cmd *exec.Cmd
}

// joinGroup puts the suspended process in a job of its own, then lets it run:
// always, even when there's no job to put it in.
func joinGroup(cmd *exec.Cmd) processGroup {
	g := makeJob(cmd)
	resume(uint32(cmd.Process.Pid))
	return g
}

func makeJob(cmd *exec.Cmd) processGroup {
	g := processGroup{cmd: cmd}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return g
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return g
	}
	// The pid is still this process's: exec holds its handle until Wait.
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return g
	}
	defer windows.CloseHandle(process)
	if windows.AssignProcessToJobObject(job, process) != nil {
		windows.CloseHandle(job)
		return g
	}
	g.job = job
	return g
}

// resume lets the suspended process's threads run: it has its first only.
func resume(pid uint32) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		if thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID); err == nil {
			windows.ResumeThread(thread)
			windows.CloseHandle(thread)
		}
	}
}

// kill ends it all while it runs: called only before Wait has reaped it, so
// with no job its pid is still its own, and its tree goes by it.
func (g processGroup) kill() error {
	if g.job != 0 {
		return windows.TerminateJobObject(g.job, 1)
	}
	if g.cmd == nil || g.cmd.Process == nil {
		return nil
	}
	tree := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(g.cmd.Process.Pid))
	tree.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if tree.Run() != nil {
		return g.cmd.Process.Kill()
	}
	return nil
}

// close ends what it left running once it has exited, as the job closes.
func (g processGroup) close() {
	if g.job != 0 {
		windows.CloseHandle(g.job)
	}
}
