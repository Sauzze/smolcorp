package sandbox

import (
	"encoding/binary"
	"testing"
)

// SA-L2: a Linux desk's seccomp filter, run here instruction by instruction as the kernel runs it on one call. The
// kernel's own answer is seccomp_linux_test.go's, on Linux.

// runDeskFilter runs filter on one call of processor arch: its number nr and ioctl's request (the second argument),
// and gives what the filter returns. It knows the instructions DeskFilter writes, and fails on any other.
func runDeskFilter(t *testing.T, filter []byte, arch, nr uint32, request uint64) uint32 {
	t.Helper()
	data := make([]byte, 64) // struct seccomp_data: nr, arch, instruction pointer, six arguments
	binary.NativeEndian.PutUint32(data[atNr:], nr)
	binary.NativeEndian.PutUint32(data[atArch:], arch)
	binary.NativeEndian.PutUint64(data[16+8:], request)
	var acc uint32
	if len(filter)%8 != 0 || len(filter) == 0 || len(filter)/8 > 4096 {
		t.Fatalf("a filter of %d bytes", len(filter))
	}
	for pc := 0; pc*8 < len(filter); pc++ {
		in := filter[pc*8 : pc*8+8]
		code, jt, jf, k := binary.NativeEndian.Uint16(in), int(in[2]), int(in[3]), binary.NativeEndian.Uint32(in[4:])
		switch code {
		case filterLoad:
			if k%4 != 0 || k+4 > 64 {
				t.Fatalf("instruction %d loads at %d", pc, k)
			}
			acc = binary.NativeEndian.Uint32(data[k:])
		case filterEqual, filterAbove:
			if acc == k || code == filterAbove && acc >= k {
				pc += jt
			} else {
				pc += jf
			}
		case filterRet:
			return k
		default:
			t.Fatalf("instruction %d: code %#x", pc, code)
		}
	}
	t.Fatal("the filter ran off its end")
	return 0
}

// filterCase is one call and what the filter must answer it.
type filterCase struct {
	name     string
	arch, nr uint32
	request  uint64
	want     uint32
}

// The filter refuses TIOCSTI and TIOCLINUX, with EPERM, however they are asked (high bits the kernel ignores, a 32-bit
// call, x32), refuses a processor it doesn't know, and lets everything else through: a filter that refused all would
// fail the second half.
func TestTheDeskFilterRefusesTheTwoIoctlsEveryWayAndNothingElse(t *testing.T) {
	const write, read32, x32Ioctl, x32Read = 1, 3, x32Bit | 514, x32Bit | 0
	for _, goarch := range []string{"amd64", "arm64"} {
		a := filterArches[goarch]
		filter, err := DeskFilter(goarch)
		if err != nil {
			t.Fatal(err)
		}
		other := filterArches[map[string]string{"amd64": "arm64", "arm64": "amd64"}[goarch]].audit
		cases := []filterCase{
			{"TIOCSTI", a.audit, a.ioctl, requestTIOCSTI, filterRefused},
			{"TIOCLINUX", a.audit, a.ioctl, requestTIOCLINUX, filterRefused},
			{"TIOCSTI with high bits the kernel ignores", a.audit, a.ioctl, 1<<32 | requestTIOCSTI, filterRefused},
			{"TIOCLINUX with every high bit", a.audit, a.ioctl, 0xffffffff<<32 | requestTIOCLINUX, filterRefused},
			{"a 32-bit TIOCSTI", a.compat, a.compatIoctl, requestTIOCSTI, filterRefused},
			{"a 32-bit TIOCLINUX", a.compat, a.compatIoctl, requestTIOCLINUX, filterRefused},
			{"another processor's call", other, 29, requestTIOCSTI, filterRefused},
			{"a processor nobody named", 0x12345678, 0, 0, filterRefused},
			{"another ioctl", a.audit, a.ioctl, requestTCGETS, filterAllow},
			{"another call given TIOCSTI's number", a.audit, write, requestTIOCSTI, filterAllow},
			{"a 32-bit other ioctl", a.compat, a.compatIoctl, requestTCGETS, filterAllow},
			{"a 32-bit read", a.compat, read32, requestTIOCSTI, filterAllow},
			{"a 32-bit call with the 64-bit ioctl's number", a.compat, a.ioctl, requestTIOCSTI, filterAllow},
		}
		if goarch == "amd64" {
			cases = append(cases, filterCase{"x32's ioctl", a.audit, x32Ioctl, requestTIOCSTI, filterRefused},
				filterCase{"any x32 call", a.audit, x32Read, 0, filterRefused})
		}
		for _, c := range cases {
			if got := runDeskFilter(t, filter, c.arch, c.nr, c.request); got != c.want {
				t.Errorf("%s, %s: the filter returned %#x, want %#x", goarch, c.name, got, c.want)
			}
		}
	}
	// EPERM, as SECCOMP_RET_ERRNO carries it: what `enter`'s row and a desk's program see.
	if filterRefused != 0x00050001 || filterAllow != 0x7fff0000 {
		t.Errorf("the filter's answers: %#x, %#x", filterRefused, filterAllow)
	}
	for _, goarch := range []string{"riscv64", "386", "arm", ""} {
		if _, err := DeskFilter(goarch); err == nil {
			t.Errorf("%q got a filter: a processor whose numbers the filter doesn't know gets none", goarch)
		}
	}
}
