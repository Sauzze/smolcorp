package desktop

import (
	"errors"
	"net"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// peerPid is the pid of the process at conn's other end, as the system has it (SO_PEERCRED: the one that
// connected, or the one that listened, as each side sees it; no process can fake it).
func peerPid(conn net.Conn) (int, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("not a Unix socket")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	pid, failed := 0, error(nil)
	if err := raw.Control(func(fd uintptr) {
		var cred *unix.Ucred
		if cred, failed = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED); failed == nil {
			pid = int(cred.Pid)
		}
	}); err != nil {
		return 0, err
	}
	return pid, failed
}

// peerRunsProgram is whether process pid runs program: the file the system says the process was started from
// (/proc/<pid>/exe, which no process can point elsewhere) is program's own. A program replaced since gives another.
func peerRunsProgram(pid int, program string) bool {
	running, err := os.Stat("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return false
	}
	own, err := os.Stat(program)
	return err == nil && os.SameFile(running, own)
}
