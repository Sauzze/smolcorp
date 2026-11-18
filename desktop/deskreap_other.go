//go:build !darwin && !windows

package desktop

import (
	"errors"
	"syscall"
)

// No desk starts in a sandbox here yet (deskSandboxed), so deskReap never looks: these are never called.

func reapList() ([]int, error) { return nil, errors.New("no desk's sandbox to look in on this system") }

func reapLook(pid int, _ reapAt) (reapRun, reapVerdict) { return reapRun{pid: pid}, reapUnknown }

func reapSignal(reapRun, syscall.Signal) error { return syscall.ENOTSUP }

func reapName(int) string { return "" }
