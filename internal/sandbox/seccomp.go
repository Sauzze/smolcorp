package sandbox

import (
	"encoding/binary"
	"fmt"
)

// A Linux desk's seccomp filter (SA-L2, SANDBOX-PLAN §A3.2 and §A4.2's `TIOCSTI` / `TIOCLINUX` row): classic BPF over
// the kernel's struct seccomp_data, written by hand, which the desk's bwrap loads (LinuxCommand's --seccomp) before
// anything of the desk's runs, so it holds for every process in the view and can't be taken off. It refuses, with
// EPERM, the two ioctl requests that type into a terminal: TIOCSTI (into the one the desk shares with a shell of the
// owner's, which runs it after the desk ends) and TIOCLINUX (the console's paste). Only the request's low 32 bits are
// compared, as the kernel reads only those (ioctl's request is an unsigned int): high bits set don't get past it.
//
// The processor's other ways in are checked too. A 32-bit call (i386's on amd64, ARM's on arm64) is checked by its own
// ioctl number; x32's calls on amd64 are refused whole; a call of any other kind of processor is refused. Everything
// else is allowed: the filter is this one row of the Policy, not a list of calls. `enter` shows it holds (its seccomp
// row) before the desk's program runs.

// The ioctl requests the filter refuses, and one it lets through (enter's check that it refuses no more): Linux's
// numbers, the same on amd64 and arm64.
const (
	requestTIOCSTI   = 0x5412
	requestTIOCLINUX = 0x541c
	requestTCGETS    = 0x5401
)

// Classic BPF, as linux/bpf_common.h and linux/seccomp.h number it.
const (
	filterLoad  = 0x20 // BPF_LD | BPF_W | BPF_ABS: the word at k in seccomp_data
	filterEqual = 0x15 // BPF_JMP | BPF_JEQ | BPF_K
	filterAbove = 0x35 // BPF_JMP | BPF_JGE | BPF_K
	filterRet   = 0x06 // BPF_RET | BPF_K

	filterAllow   = 0x7fff0000     // SECCOMP_RET_ALLOW
	filterRefused = 0x00050000 | 1 // SECCOMP_RET_ERRNO with EPERM

	// Offsets in seccomp_data: the call's number, the processor, and the low half of its second argument (ioctl's
	// request) on a little-endian processor, which both of filterArches are.
	atNr, atArch, atRequest = 0, 4, 24

	x32Bit = 0x40000000 // __X32_SYSCALL_BIT: an x32 call's number has it
)

// filterArch is what the filter knows of one processor: its AUDIT_ARCH_* and ioctl's number, those of the 32-bit calls
// it also runs, and whether x32's calls are refused whole.
type filterArch struct {
	audit, ioctl         uint32
	compat, compatIoctl  uint32
	refuseX32CallsWholly bool
}

// filterArches are the processors Smolcorp is built for on Linux.
var filterArches = map[string]filterArch{
	"amd64": {audit: 0xc000003e, ioctl: 16, compat: 0x40000003, compatIoctl: 54, refuseX32CallsWholly: true},
	"arm64": {audit: 0xc00000b7, ioctl: 29, compat: 0x40000028, compatIoctl: 54},
}

// DeskFilter is the filter a Linux desk's bwrap loads on goarch, as the bytes of its struct sock_filter array. A
// processor it doesn't know gets none, and its desk doesn't start.
func DeskFilter(goarch string) ([]byte, error) {
	a, ok := filterArches[goarch]
	if !ok {
		return nil, fmt.Errorf("sandbox: no seccomp filter for %s, so no desk starts there", goarch)
	}
	// Each step's jumps are by label; "" goes on to the next step.
	type step struct {
		label  string
		code   uint16
		k      uint32
		jt, jf string
	}
	steps := []step{
		{code: filterLoad, k: atArch},
		{code: filterEqual, k: a.audit, jf: "compat"},
		{code: filterLoad, k: atNr},
	}
	if a.refuseX32CallsWholly {
		steps = append(steps, step{code: filterAbove, k: x32Bit, jt: "refuse"})
	}
	steps = append(steps,
		step{code: filterEqual, k: a.ioctl, jt: "request", jf: "allow"},
		step{label: "compat", code: filterEqual, k: a.compat, jf: "refuse"},
		step{code: filterLoad, k: atNr},
		step{code: filterEqual, k: a.compatIoctl, jt: "request", jf: "allow"},
		step{label: "request", code: filterLoad, k: atRequest},
		step{code: filterEqual, k: requestTIOCSTI, jt: "refuse"},
		step{code: filterEqual, k: requestTIOCLINUX, jt: "refuse", jf: "allow"},
		step{label: "allow", code: filterRet, k: filterAllow},
		step{label: "refuse", code: filterRet, k: filterRefused},
	)
	at := map[string]int{}
	for i, s := range steps {
		if s.label != "" {
			at[s.label] = i
		}
	}
	jump := func(from int, to string) (uint8, error) {
		if to == "" {
			return 0, nil
		}
		target, ok := at[to]
		if !ok || target <= from || target-from-1 > 255 {
			return 0, fmt.Errorf("sandbox: the seccomp filter's jump to %q from step %d", to, from)
		}
		return uint8(target - from - 1), nil
	}
	out := make([]byte, 0, 8*len(steps))
	for i, s := range steps {
		jt, err := jump(i, s.jt)
		if err != nil {
			return nil, err
		}
		jf, err := jump(i, s.jf)
		if err != nil {
			return nil, err
		}
		// struct sock_filter, in the processor's own order: the kernel reads it as it is.
		out = binary.NativeEndian.AppendUint16(out, s.code)
		out = append(out, jt, jf)
		out = binary.NativeEndian.AppendUint32(out, s.k)
	}
	return out, nil
}
