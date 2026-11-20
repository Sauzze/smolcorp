//go:build darwin && !cgo

package desktop

import (
	"errors"
	"syscall"
)

// Without cgo nothing can ask macOS which processes are in a desk's sandbox (the app's own build has it:
// scripts/build-app.sh), so the reap never knows a desk empty, and its stop refuses. No desk starts in a sandbox there
// anyway (internal/sandbox's run_nocheck_darwin.go).

func reapList() ([]int, error) {
	return nil, errors.New("this build can't ask macOS: it was built without cgo")
}

func reapLook(pid int, _ reapAt) (reapRun, reapVerdict) { return reapRun{pid: pid}, reapUnknown }

func reapSignal(reapRun, syscall.Signal) error { return syscall.ENOTSUP }

func reapName(int) string { return "" }
