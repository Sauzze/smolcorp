//go:build darwin && cgo

package desktop

/*
#include <dlfcn.h>
#include <errno.h>
#include <libproc.h>
#include <mach/mach.h>
#include <stdlib.h>
#include <sys/proc.h>
#include <sys/proc_info.h>
#include <sys/sysctl.h>

// libsystem_sandbox's checks, as internal/sandbox's run_check_darwin.go declares them (no public header has them). The
// filter type takes SANDBOX_CHECK_NO_REPORT, so a denied answer isn't logged as a violation.
enum sandbox_filter_type { SANDBOX_FILTER_NONE, SANDBOX_FILTER_PATH };
extern const enum sandbox_filter_type SANDBOX_CHECK_NO_REPORT;
int sandbox_check_by_audit_token(audit_token_t, const char *operation, enum sandbox_filter_type, ...);
int sandbox_check(pid_t, const char *operation, enum sandbox_filter_type, ...);

// The pids of the processes whose effective uid is uid, into pids (room for n): how many, -1 when they can't be listed,
// n+1 when there may be more than n.
static int smolcorpReapList(int uid, int *pids, int n) {
	int bytes = proc_listpids(PROC_UID_ONLY, (uint32_t)uid, pids, n * (int)sizeof(int));
	if (bytes <= 0) {
		return -1; // never none: the core is one
	}
	int got = bytes / (int)sizeof(int);
	return got >= n ? n + 1 : got;
}

// The audit token of the program process pid runs now, through its task name port: 0, or -1 when there is none (it
// has ended, or doesn't run as the caller's user on both its ids).
static int smolcorpReapToken(int pid, audit_token_t *token) {
	mach_port_t name = MACH_PORT_NULL;
	if (task_name_for_pid(mach_task_self(), pid, &name) != KERN_SUCCESS) {
		return -1;
	}
	mach_msg_type_number_t count = TASK_AUDIT_TOKEN_COUNT;
	kern_return_t kr = task_info(name, TASK_AUDIT_TOKEN, (task_info_t)token, &count);
	mach_port_deallocate(mach_task_self(), name);
	return kr == KERN_SUCCESS && (int)token->val[5] == pid ? 0 : -1; // val[5]: the pid, as audit_token_to_pid reads it
}

// macOS's answer to whether a process may do operation on path, asked by its token, or by its pid when token is NULL:
// 1 denied, 0 allowed, -1 not answered. A process that has ended is answered as denied.
static int smolcorpReapDenied(const audit_token_t *token, int pid, const char *operation, const char *path) {
	int r = token != NULL
		? sandbox_check_by_audit_token(*token, operation, SANDBOX_FILTER_PATH | SANDBOX_CHECK_NO_REPORT, path)
		: sandbox_check((pid_t)pid, operation, SANDBOX_FILTER_PATH | SANDBOX_CHECK_NO_REPORT, path);
	return r == 0 || r == 1 ? r : -1;
}

// Whether a process is in the sandbox of the desk at profile, hires and support, mark being its mark and folder the
// mark's folder: 1 in it, 0 not (or ended), -1 not answered. It's in when no desk's may write the profile, or read the
// hires folder, the support folder or the mark's folder (an unsandboxed process may do all four; a sandbox that reads
// everything, or all but one of the folders, may do one), and only that desk's may read the mark. Each answer is the
// process's at its own moment, and a process may sandbox itself between two (a browser's renderer, an App Sandbox app,
// each at its start). So the denies are asked first, and the allowed read last (FXFIREREAPR4). A sandbox is added once
// and never taken off, and an ended process is answered as denied: a process allowed the read last still had a sandbox
// then (unsandboxed, it would have been allowed the write at the first), and so had that same one at the first. All
// the answers are one sandbox's: the desk's. Asked the other way round, a read allowed before a process sandboxed
// itself and denies after it did made it "in". A process being made answers denied too, before it has its parent's
// sandbox or none: reapLookWith takes "in" only from a process it saw running whole before the first check. A desk
// made before marks (folder NULL) is asked by its profile instead of a mark, and not about the mark's folder.
static int smolcorpReapIn(const audit_token_t *token, int pid, const char *profile, const char *hires,
	const char *support, const char *folder, const char *mark) {
	const char *operation[] = {"file-write-data", "file-read-data", "file-read-data", "file-read-data"};
	const char *path[] = {profile, hires, support, folder};
	for (int i = 0; i < 4 && path[i] != NULL; i++) {
		int denied = smolcorpReapDenied(token, pid, operation[i], path[i]);
		if (denied != 1) {
			return denied == 0 ? 0 : -1;
		}
	}
	int read = smolcorpReapDenied(token, pid, "file-read-data", mark);
	if (read != 0) {
		return read == 1 ? 0 : -1; // only that desk's sandbox reads its mark
	}
	return 1;
}

// Whether process pid has ended and waits for its parent to read its end (a zombie): 1, its start time in start; 0
// not, or not read. proc_pidinfo doesn't read a zombie (ESRCH), though proc_listpids lists it; the kernel's process
// table, by sysctl, does (FXFIREREAPR2).
static int smolcorpReapZombie(int pid, uint64_t *start) {
	struct kinfo_proc k;
	size_t size = sizeof(k);
	int name[4] = {CTL_KERN, KERN_PROC, KERN_PROC_PID, pid};
	if (sysctl(name, 4, &k, &size, NULL, 0) != 0 || size != sizeof(k) || k.kp_proc.p_pid != pid ||
		k.kp_proc.p_stat != SZOMB) {
		return 0;
	}
	*start = (uint64_t)k.kp_proc.p_starttime.tv_sec * 1000000 + (uint64_t)k.kp_proc.p_starttime.tv_usec;
	return 1;
}

// Whether process pid is running, whole: 1 made (not still being forked), and not on its way out of exit(); 2 ended,
// waiting for its parent to read its end (a zombie, which the owner's list holds too); 0 neither; -1 macOS won't say
// (a process this user may not read, which no desk's is: each runs as the owner on both its ids). Which process it is
// goes in start, its start time to the microsecond: with its pid, no other process has had it.
static int smolcorpReapRunning(int pid, uint64_t *start) {
	struct proc_bsdinfo info;
	errno = 0;
	int got = proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, (int)sizeof(info));
	if (got != (int)sizeof(info)) {
		if (got <= 0 && errno == EPERM) {
			return -1;
		}
		return smolcorpReapZombie(pid, start) ? 2 : 0;
	}
	*start = info.pbi_start_tvsec * 1000000 + info.pbi_start_tvusec;
	return (info.pbi_status == SRUN || info.pbi_status == SSLEEP || info.pbi_status == SSTOP) &&
		(info.pbi_flags & PROC_FLAG_INEXIT) == 0;
}

// Signal sig to the one run token names: 0, or the error (ESRCH once that run has ended or run another program).
// Looked up when it's called, not linked: a macOS without it can't end a desk's process, so the stop refuses, and the
// core still starts.
static int smolcorpReapSignal(audit_token_t *token, int sig) {
	int (*signal_run)(audit_token_t *, int) = (int (*)(audit_token_t *, int))dlsym(RTLD_DEFAULT, "proc_signal_with_audittoken");
	return signal_run != NULL ? signal_run(token, sig) : ENOTSUP;
}
*/
import "C"

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// reapList is every process running as the owner (its effective uid): the core's own user.
func reapList() ([]int, error) {
	for n := 4096; n <= 1<<20; n *= 2 {
		pids := make([]C.int, n)
		got := int(C.smolcorpReapList(C.int(os.Getuid()), &pids[0], C.int(n)))
		if got < 0 {
			return nil, errors.New("macOS didn't list the processes")
		}
		if got <= n {
			list := make([]int, got)
			for i := range list {
				list[i] = int(pids[i])
			}
			return list, nil
		}
	}
	return nil, errors.New("too many processes to list")
}

// reapLook is a look at process pid: whether it's in the sandbox of the desk at at (the support folder being the hires
// folder's), and its run, with its start time when the look read it (reapLookWith).
func reapLook(pid int, at reapAt) (reapRun, reapVerdict) {
	allowed, folder := at.profile, (*C.char)(nil) // a desk made before marks: asked by its profile
	if at.mark != "" {
		allowed, folder = at.mark, C.CString(filepath.Dir(at.mark))
		defer C.free(unsafe.Pointer(folder))
	}
	mark := C.CString(allowed)
	defer C.free(unsafe.Pointer(mark))
	p, h, s := C.CString(at.profile), C.CString(at.hires), C.CString(filepath.Dir(at.hires))
	defer C.free(unsafe.Pointer(p))
	defer C.free(unsafe.Pointer(h))
	defer C.free(unsafe.Pointer(s))
	return reapLookWith(reapSteps{
		running: func(pid int) (int, uint64) {
			var start C.uint64_t
			state := int(C.smolcorpReapRunning(C.int(pid), &start))
			return state, uint64(start)
		},
		token: func(pid int) (token [8]uint32, ok bool) {
			var t C.audit_token_t
			if C.smolcorpReapToken(C.int(pid), &t) != 0 {
				return token, false
			}
			for i := range token {
				token[i] = uint32(t.val[i])
			}
			return token, true
		},
		in: func(token *[8]uint32, pid int) int {
			if token == nil {
				return int(C.smolcorpReapIn(nil, C.int(pid), p, h, s, folder, mark))
			}
			var t C.audit_token_t
			for i := range token {
				t.val[i] = C.uint(token[i])
			}
			return int(C.smolcorpReapIn(&t, C.int(pid), p, h, s, folder, mark))
		},
	}, pid)
}

// reapName is the name of the program process pid runs, as Activity Monitor shows it: "" when macOS won't say.
func reapName(pid int) string {
	var name [256]C.char
	if C.proc_name(C.int(pid), unsafe.Pointer(&name[0]), C.uint32_t(len(name))) <= 0 {
		return ""
	}
	return C.GoString(&name[0])
}

// reapSignal sends sig to that one run of the process.
func reapSignal(run reapRun, sig syscall.Signal) error {
	var token C.audit_token_t
	for i := range run.token {
		token.val[i] = C.uint(run.token[i])
	}
	if r := C.smolcorpReapSignal(&token, C.int(sig)); r != 0 {
		return syscall.Errno(r)
	}
	return nil
}
