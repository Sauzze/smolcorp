//go:build darwin && cgo

package desktop

/*
#include <arpa/inet.h>
#include <errno.h>
#include <libproc.h>
#include <mach/mach.h>
#include <stdlib.h>
#include <string.h>
#include <sys/proc.h>
#include <sys/proc_info.h>
#include <sys/sysctl.h>

// libsystem_sandbox's check, as deskreap_darwin.go declares it (no public header has it). Asked with no operation, it
// says whether the process is in a sandbox at all: 1 in one, 0 in none (and 1 for a run that has ended).
enum sandbox_filter_type { SANDBOX_FILTER_NONE };
int sandbox_check_by_audit_token(audit_token_t, const char *operation, enum sandbox_filter_type, ...);

// Whether process pid, which proc_pidinfo just couldn't read, has ended or is ending: 1 when it said ESRCH and the
// kernel's process table, by sysctl, has it waiting for its parent to read its end (a zombie, which proc_listpids lists
// and proc_pidinfo doesn't read, as deskreap_darwin.go's smolcorpReapZombie tells one), has it inside exit() (P_WEXIT:
// its threads are stopped and never run its own code again, though closing its files can hold it there for minutes,
// RT126SWEEP3FIXP4R2 B2), or no longer has it (gone); 0 for any other process, running or being made (SIDL), or one
// that wasn't read. None of these can read a ticket through what it holds, so it holds nothing.
static int smolcorpAskEnded(int pid) {
	if (errno != ESRCH) {
		return 0;
	}
	struct kinfo_proc k;
	size_t size = sizeof(k);
	int name[4] = {CTL_KERN, KERN_PROC, KERN_PROC_PID, pid};
	if (sysctl(name, 4, &k, &size, NULL, 0) != 0) {
		return 0;
	}
	return size == 0 || (size == sizeof(k) && k.kp_proc.p_pid == pid &&
		(k.kp_proc.p_stat == SZOMB || (k.kp_proc.p_flag & P_WEXIT) != 0));
}

// Whether process pid holds a TCP socket whose own end is laddr:lport and whose other end is faddr:fport (an address of
// 4 bytes, or 16 for IPv6): 1 it does, 0 it doesn't (an ended or ending one holds none, nor one with no files open),
// -1 its files can't be read. proc_pidinfo answers 0 both when it fails, with errno set, and when it read a process
// with no files open, with errno left 0: that one is read, and holds nothing (RT126SWEEP3FIXP4R1 B1).
static int smolcorpAskHolds(int pid, const unsigned char *laddr, int lport, const unsigned char *faddr, int fport, int v6) {
	errno = 0;
	int bytes = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, NULL, 0);
	if (bytes <= 0) {
		return smolcorpAskEnded(pid) ? 0 : -1;
	}
	int room = bytes / (int)PROC_PIDLISTFD_SIZE + 64; // files opened meanwhile
	struct proc_fdinfo *fds = malloc((size_t)room * PROC_PIDLISTFD_SIZE);
	if (fds == NULL) {
		return -1;
	}
	errno = 0;
	bytes = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, fds, room * (int)PROC_PIDLISTFD_SIZE);
	if (bytes == 0 && errno == 0) {
		free(fds);
		return 0;
	}
	if (bytes <= 0) {
		free(fds);
		return smolcorpAskEnded(pid) ? 0 : -1;
	}
	int found = 0;
	for (int i = 0; i < bytes / (int)PROC_PIDLISTFD_SIZE && !found; i++) {
		if (fds[i].proc_fdtype != PROX_FDTYPE_SOCKET) {
			continue;
		}
		struct socket_fdinfo info;
		if (proc_pidfdinfo(pid, fds[i].proc_fd, PROC_PIDFDSOCKETINFO, &info, PROC_PIDFDSOCKETINFO_SIZE) != PROC_PIDFDSOCKETINFO_SIZE ||
			info.psi.soi_kind != SOCKINFO_TCP) {
			continue;
		}
		struct in_sockinfo *in = &info.psi.soi_proto.pri_tcp.tcpsi_ini;
		if (ntohs((unsigned short)in->insi_lport) != lport || ntohs((unsigned short)in->insi_fport) != fport) {
			continue;
		}
		if (v6) {
			found = (in->insi_vflag & INI_IPV6) && memcmp(&in->insi_laddr.ina_6, laddr, 16) == 0 &&
				memcmp(&in->insi_faddr.ina_6, faddr, 16) == 0;
		} else {
			found = (in->insi_vflag & INI_IPV4) && memcmp(&in->insi_laddr.ina_46.i46a_addr4, laddr, 4) == 0 &&
				memcmp(&in->insi_faddr.ina_46.i46a_addr4, faddr, 4) == 0;
		}
	}
	free(fds);
	return found;
}

// The audit token of the program process pid runs now, through its task name port, as deskreap_darwin.go reads it: 0,
// or -1 when there is none.
static int smolcorpAskToken(int pid, audit_token_t *token) {
	mach_port_t name = MACH_PORT_NULL;
	if (task_name_for_pid(mach_task_self(), pid, &name) != KERN_SUCCESS) {
		return -1;
	}
	mach_msg_type_number_t count = TASK_AUDIT_TOKEN_COUNT;
	kern_return_t kr = task_info(name, TASK_AUDIT_TOKEN, (task_info_t)token, &count);
	mach_port_deallocate(mach_task_self(), name);
	return kr == KERN_SUCCESS && (int)token->val[5] == pid ? 0 : -1;
}

// Whether the run token names is in a sandbox: 1 in one (or ended), 0 in none, -1 not answered.
static int smolcorpAskSandboxed(const audit_token_t *token) {
	int r = sandbox_check_by_audit_token(*token, NULL, SANDBOX_FILTER_NONE);
	return r == 0 || r == 1 ? r : -1;
}
*/
import "C"

import (
	"net"
	"net/http"
	"unsafe"
)

// askingProgramHere is what macOS says of the program that sent a terminal's ask (askprogram.go): whether it's in a
// sandbox, as a hire is, or outside any, as the owner's terminal is.
func askingProgramHere(r *http.Request) askAsker {
	local, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if local == nil {
		return askAsker{sandbox: sandboxUnknown}
	}
	return askAsker{sandbox: askSandboxOf(r.RemoteAddr, local.String(), askHolderLook{
		list: reapList,
		holds: func(pid int, from, to *net.TCPAddr) int {
			return int(C.smolcorpAskHolds(C.int(pid), askAddr(from.IP), C.int(from.Port), askAddr(to.IP), C.int(to.Port), askV6(from.IP)))
		},
		in: func(pid int) int {
			var token C.audit_token_t
			if C.smolcorpAskToken(C.int(pid), &token) != 0 {
				return -1
			}
			return int(C.smolcorpAskSandboxed(&token))
		},
	})}
}

// askAddr is ip's bytes as the kernel keeps them, for C: 4 for IPv4, else 16. Go keeps them alive past the call.
func askAddr(ip net.IP) *C.uchar {
	if four := ip.To4(); four != nil {
		return (*C.uchar)(unsafe.Pointer(&four[0]))
	}
	sixteen := ip.To16()
	return (*C.uchar)(unsafe.Pointer(&sixteen[0]))
}

// askV6 is 1 for an IPv6 address (not one of IPv4's), for C.
func askV6(ip net.IP) C.int {
	if ip.To4() != nil {
		return 0
	}
	return 1
}
