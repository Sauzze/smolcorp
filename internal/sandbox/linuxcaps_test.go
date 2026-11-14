package sandbox

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// SA-L0 (SANDBOX-PLAN §A3.2, PL126A §4): before any Linux sandbox code is written, CI records what each runner's
// kernel, AppArmor and packaged bwrap actually allow: user, pid, mount and network namespaces, a seccomp filter
// handed over with --seccomp, and the Landlock ABI. linuxcaps_linux_test.go runs the probes; this file holds what
// they share with every system: the facts a probe reports, how a row's answer is read from them, the table, and the
// probe's seccomp filter, written by hand. All of it is checked here on stand-in facts, so a reading that would
// call a namespace made when it wasn't fails on any computer, not only on a Linux runner.

// linuxFacts are what a probe sees from where it runs: outside any sandbox, inside bwrap, or inside a user
// namespace a plain program made for itself.
type linuxFacts struct {
	Namespaces map[string]string `json:"namespaces"` // user, pid, mnt, net: the /proc/self/ns link, e.g. "net:[4026531840]"
	Pid        int               `json:"pid"`
	UIDMap     string            `json:"uidMap"`     // /proc/self/uid_map on one line; empty when unmapped
	Interfaces []string          `json:"interfaces"` // the network interfaces it can see
	ProbeDev   uint64            `json:"probeDev"`   // st_dev of the probe folder, which a tmpfs covers inside
	Mount      string            `json:"mount"`      // asked to mount a tmpfs itself: "mounted" or the error; else empty
	TIOCSTI    string            `json:"tiocsti"`    // ioctl(/dev/null, TIOCSTI): "ok" or the errno's name
	TIOCLINUX  string            `json:"tioclinux"`  // the same for TIOCLINUX
	Seccomp    string            `json:"seccomp"`    // /proc/self/status's Seccomp field: 0 none, 2 a filter
	NoNewPrivs string            `json:"noNewPrivs"` // /proc/self/status's NoNewPrivs field
	Landlock   string            `json:"landlock"`   // "ABI <n>", or "no: <errno>"
}

// probeRun is one probe started: the facts it reported, or why there are none.
type probeRun struct {
	Facts  *linuxFacts
	Err    string // didn't start, or exited with a failure
	Stderr string // the end of what it wrote to its error output
}

// capRow is one row of the table.
type capRow struct{ What, Answer, Detail string }

// failed says why a probe reported nothing, or "" when it did.
func (r probeRun) failed() string {
	if r.Facts != nil {
		return ""
	}
	why := r.Err
	if why == "" {
		why = "it reported no facts"
	}
	if r.Stderr != "" {
		why += ": " + r.Stderr
	}
	return why
}

// newNamespace says whether the probe ran in a namespace of that kind other than the one outside, and why not.
func newNamespace(kind string, outside linuxFacts, r probeRun) (bool, string) {
	if why := r.failed(); why != "" {
		return false, why
	}
	in, out := r.Facts.Namespaces[kind], outside.Namespaces[kind]
	switch {
	case !strings.HasPrefix(in, kind+":["):
		return false, fmt.Sprintf("its %s namespace can't be read inside (%q)", kind, in)
	case in == out:
		return false, fmt.Sprintf("the same %s namespace as outside (%s)", kind, in)
	}
	return true, fmt.Sprintf("%s inside, %s outside", in, out)
}

// namespaceRow reads one namespace row: the namespace is new, and what it should change inside is seen to.
func namespaceRow(what, kind string, outside linuxFacts, r probeRun) capRow {
	ok, detail := newNamespace(kind, outside, r)
	if !ok {
		return capRow{what, "no", detail}
	}
	f := r.Facts
	switch kind {
	case "user":
		if f.UIDMap == "" {
			return capRow{what, "no", detail + ", but no uid is mapped inside"}
		}
		detail += ", uid map " + f.UIDMap
	case "pid":
		detail += fmt.Sprintf(", pid %d inside", f.Pid)
	case "mnt":
		if f.ProbeDev == outside.ProbeDev {
			return capRow{what, "no", detail + ", but the tmpfs over the probe folder isn't there (same st_dev)"}
		}
		detail += fmt.Sprintf(", the probe folder's st_dev %d inside, %d outside", f.ProbeDev, outside.ProbeDev)
	case "net":
		if !slices.Equal(f.Interfaces, []string{"lo"}) {
			return capRow{what, "no", detail + fmt.Sprintf(", but it sees the interfaces %v", f.Interfaces)}
		}
		detail += ", only lo inside"
	}
	return capRow{what, "yes", detail}
}

// seccompRow reads the --seccomp row. The probe's filter refuses TIOCSTI and TIOCLINUX with EPERM; on /dev/null
// both fail with ENOTTY without it. So the filter is shown only when both are EPERM inside, the kernel says a
// filter is on, and neither is EPERM outside.
func seccompRow(what string, outside linuxFacts, r probeRun) capRow {
	if why := r.failed(); why != "" {
		return capRow{what, "no", why}
	}
	f := r.Facts
	detail := fmt.Sprintf("TIOCSTI %s, TIOCLINUX %s inside (%s, %s outside), Seccomp %s, NoNewPrivs %s",
		f.TIOCSTI, f.TIOCLINUX, outside.TIOCSTI, outside.TIOCLINUX, f.Seccomp, f.NoNewPrivs)
	switch {
	case outside.TIOCSTI == "EPERM" || outside.TIOCLINUX == "EPERM":
		return capRow{what, "can't tell", detail + ": refused outside already, so the filter isn't shown"}
	case f.TIOCSTI != "EPERM" || f.TIOCLINUX != "EPERM":
		return capRow{what, "no", detail}
	case f.Seccomp != "2":
		return capRow{what, "no", detail + ": refused, but the kernel shows no filter"}
	}
	return capRow{what, "yes", detail}
}

// landlockRow gives the Landlock ABI where the probe ran.
func landlockRow(what string, r probeRun) capRow {
	if why := r.failed(); why != "" {
		return capRow{what, "can't tell", why}
	}
	answer, detail, _ := strings.Cut(r.Facts.Landlock, ": ")
	return capRow{what, answer, detail}
}

// mountRow reads whether a plain program, in a user namespace of its own, could mount a tmpfs there: what
// Ubuntu 24.04's AppArmor rule takes away from every program but those it names.
func mountRow(what string, outside linuxFacts, r probeRun) capRow {
	if ok, detail := newNamespace("user", outside, r); !ok {
		return capRow{what, "no", "no user namespace: " + detail}
	}
	if r.Facts.Mount != "mounted" {
		return capRow{what, "no", "mount: " + r.Facts.Mount}
	}
	return capRow{what, "yes", "a tmpfs mounted inside its own user and mount namespaces"}
}

// capTable writes the rows as a Markdown table, one row per line whatever a detail holds.
func capTable(rows []capRow) string {
	cell := func(s string) string {
		s = strings.ReplaceAll(s, "|", `\|`)
		return strings.Join(strings.Fields(s), " ")
	}
	var b strings.Builder
	b.WriteString("| What | Answer | Detail |\n| --- | --- | --- |\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", cell(row.What), cell(row.Answer), cell(row.Detail))
	}
	return b.String()
}

// The probe's seccomp filter, as classic BPF over struct seccomp_data, for the runner's processor. The numbers are
// the kernel's ABI, the same on every Linux; linuxcaps_linux_test.go checks them against golang.org/x/sys/unix.
const (
	bpfLoadWord = 0x20 // BPF_LD | BPF_W | BPF_ABS
	bpfJumpIfEq = 0x15 // BPF_JMP | BPF_JEQ | BPF_K
	bpfReturn   = 0x06 // BPF_RET | BPF_K

	seccompAllow = 0x7fff0000 // SECCOMP_RET_ALLOW
	seccompErrno = 0x00050000 // SECCOMP_RET_ERRNO, with the errno in the low bits
	errnoEPERM   = 1

	ioctlTIOCSTI   = 0x5412
	ioctlTIOCLINUX = 0x541c

	// Offsets into struct seccomp_data: nr, arch, then the arguments as 64-bit words from 16.
	dataNr       = 0
	dataArch     = 4
	dataArg1LoLE = 24 // the low half of the second argument, on a little-endian processor
)

// probeArches are the processors the CI runners have: AUDIT_ARCH_* and ioctl's number there.
var probeArches = map[string]struct{ audit, ioctl uint32 }{
	"amd64": {0xc000003e, 16},
	"arm64": {0xc00000b7, 29},
}

// probeFilter is the program bwrap's --seccomp reads: ioctl with TIOCSTI or TIOCLINUX gets EPERM, everything else
// is allowed. Only the request's low 32 bits are compared, as the kernel reads only those (ioctl's cmd is an
// unsigned int). It is a probe, not SA-L2's filter: that one also refuses another processor's calls and x32's.
func probeFilter(goarch string) ([]byte, error) {
	arch, ok := probeArches[goarch]
	if !ok {
		return nil, fmt.Errorf("no probe filter for %s", goarch)
	}
	type insn struct {
		code   uint16
		jt, jf uint8
		k      uint32
	}
	program := []insn{
		{bpfLoadWord, 0, 0, dataArch},
		{bpfJumpIfEq, 0, 5, arch.audit}, // another processor: allow
		{bpfLoadWord, 0, 0, dataNr},
		{bpfJumpIfEq, 0, 3, arch.ioctl}, // not ioctl: allow
		{bpfLoadWord, 0, 0, dataArg1LoLE},
		{bpfJumpIfEq, 2, 0, ioctlTIOCSTI},
		{bpfJumpIfEq, 1, 0, ioctlTIOCLINUX},
		{bpfReturn, 0, 0, seccompAllow},
		{bpfReturn, 0, 0, seccompErrno | errnoEPERM},
	}
	out := make([]byte, 0, 8*len(program))
	for _, in := range program {
		out = binary.LittleEndian.AppendUint16(out, in.code)
		out = append(out, in.jt, in.jf)
		out = binary.LittleEndian.AppendUint32(out, in.k)
	}
	return out, nil
}

// runFilter runs a filter the way the kernel would on one call, for the three kinds of instruction the probe uses.
func runFilter(t *testing.T, program []byte, arch, nr uint32, arg1 uint64) uint32 {
	t.Helper()
	data := make([]byte, 64)
	binary.LittleEndian.PutUint32(data[dataNr:], nr)
	binary.LittleEndian.PutUint32(data[dataArch:], arch)
	binary.LittleEndian.PutUint64(data[16+8:], arg1)
	var acc uint32
	for pc := 0; pc*8 < len(program); pc++ {
		in := program[pc*8 : pc*8+8]
		code, jt, jf, k := binary.LittleEndian.Uint16(in), in[2], in[3], binary.LittleEndian.Uint32(in[4:])
		switch code {
		case bpfLoadWord:
			acc = binary.LittleEndian.Uint32(data[k:])
		case bpfJumpIfEq:
			if acc == k {
				pc += int(jt)
			} else {
				pc += int(jf)
			}
		case bpfReturn:
			return k
		default:
			t.Fatalf("instruction %d: unknown code %#x", pc, code)
		}
	}
	t.Fatal("the filter ran off its end")
	return 0
}

// The probe's filter refuses exactly TIOCSTI and TIOCLINUX, on each runner's processor, high bits or not, and
// lets every other call through: without the second half a "yes" could come from a filter refusing everything.
func TestTheProbeFilterRefusesOnlyTheTwoIoctls(t *testing.T) {
	const TCGETS, write = 0x5401, 1
	for goarch, arch := range probeArches {
		program, err := probeFilter(goarch)
		if err != nil {
			t.Fatal(err)
		}
		refused := uint32(seccompErrno | errnoEPERM)
		for _, c := range []struct {
			name     string
			arch, nr uint32
			arg1     uint64
			want     uint32
		}{
			{"TIOCSTI", arch.audit, arch.ioctl, ioctlTIOCSTI, refused},
			{"TIOCLINUX", arch.audit, arch.ioctl, ioctlTIOCLINUX, refused},
			{"TIOCSTI with high bits the kernel ignores", arch.audit, arch.ioctl, 1<<32 | ioctlTIOCSTI, refused},
			{"another ioctl", arch.audit, arch.ioctl, TCGETS, seccompAllow},
			{"another call with TIOCSTI's number", arch.audit, write, ioctlTIOCSTI, seccompAllow},
			{"another processor's call", 0x40000003, arch.ioctl, ioctlTIOCSTI, seccompAllow},
		} {
			if got := runFilter(t, program, c.arch, c.nr, c.arg1); got != c.want {
				t.Errorf("%s, %s: the filter returned %#x, want %#x", goarch, c.name, got, c.want)
			}
		}
	}
	if _, err := probeFilter("riscv64"); err == nil {
		t.Error("a processor without known numbers got a filter")
	}
}

// A row says "yes" only when the inside differs from the outside the way that namespace should make it: a probe
// that failed, ran in the same namespace, or didn't show the namespace's effect reads "no", with why.
func TestANamespaceRowIsYesOnlyWhenTheInsideIsSeenToDiffer(t *testing.T) {
	outside := linuxFacts{
		Namespaces: map[string]string{"user": "user:[1]", "pid": "pid:[2]", "mnt": "mnt:[3]", "net": "net:[4]"},
		Pid:        4242, UIDMap: "0 0 4294967295", Interfaces: []string{"lo", "eth0"}, ProbeDev: 2049,
	}
	inside := func(change func(*linuxFacts)) probeRun {
		f := linuxFacts{
			Namespaces: map[string]string{"user": "user:[11]", "pid": "pid:[12]", "mnt": "mnt:[13]", "net": "net:[14]"},
			Pid:        2, UIDMap: "1001 1001 1", Interfaces: []string{"lo"}, ProbeDev: 77,
		}
		change(&f)
		return probeRun{Facts: &f}
	}
	same := func(f *linuxFacts) {}
	for _, c := range []struct {
		name, kind string
		run        probeRun
		want       string
	}{
		{"a new user namespace", "user", inside(same), "yes"},
		{"a new pid namespace", "pid", inside(same), "yes"},
		{"a new mount namespace with the tmpfs", "mnt", inside(same), "yes"},
		{"a new network namespace with only lo", "net", inside(same), "yes"},
		{"bwrap refused", "user", probeRun{Err: "exit status 1", Stderr: "bwrap: No permissions to create a new namespace"}, "no"},
		{"started, reported nothing", "net", probeRun{}, "no"},
		{"the outside's user namespace", "user", inside(func(f *linuxFacts) { f.Namespaces["user"] = "user:[1]" }), "no"},
		{"no uid mapped", "user", inside(func(f *linuxFacts) { f.UIDMap = "" }), "no"},
		{"the outside's pid namespace", "pid", inside(func(f *linuxFacts) { f.Namespaces["pid"] = "pid:[2]" }), "no"},
		{"a namespace link that can't be read", "pid", inside(func(f *linuxFacts) { f.Namespaces["pid"] = "error: EACCES" }), "no"},
		{"a new mount namespace without the tmpfs", "mnt", inside(func(f *linuxFacts) { f.ProbeDev = 2049 }), "no"},
		{"a new network namespace that still sees eth0", "net", inside(func(f *linuxFacts) { f.Interfaces = []string{"lo", "eth0"} }), "no"},
		{"the outside's network namespace", "net", inside(func(f *linuxFacts) { f.Namespaces["net"] = "net:[4]" }), "no"},
	} {
		row := namespaceRow("row", c.kind, outside, c.run)
		if row.Answer != c.want || row.Detail == "" {
			t.Errorf("%s: %q (%s), want %q with a reason", c.name, row.Answer, row.Detail, c.want)
		}
	}
}

// The seccomp row needs the filter shown both ways: EPERM inside, a filter on, and not EPERM outside.
func TestTheSeccompRowNeedsTheFilterShownInsideAndNotOutside(t *testing.T) {
	outside := linuxFacts{TIOCSTI: "ENOTTY", TIOCLINUX: "ENOTTY", Seccomp: "0"}
	filtered := linuxFacts{TIOCSTI: "EPERM", TIOCLINUX: "EPERM", Seccomp: "2", NoNewPrivs: "1"}
	for _, c := range []struct {
		name    string
		outside linuxFacts
		inside  linuxFacts
		want    string
	}{
		{"refused inside only, with a filter", outside, filtered, "yes"},
		{"not refused inside", outside, linuxFacts{TIOCSTI: "ENOTTY", TIOCLINUX: "ENOTTY", Seccomp: "0"}, "no"},
		{"only TIOCSTI refused", outside, linuxFacts{TIOCSTI: "EPERM", TIOCLINUX: "ENOTTY", Seccomp: "2"}, "no"},
		{"refused, but no filter shown", outside, linuxFacts{TIOCSTI: "EPERM", TIOCLINUX: "EPERM", Seccomp: "0"}, "no"},
		{"refused outside too", linuxFacts{TIOCSTI: "EPERM", TIOCLINUX: "ENOTTY"}, filtered, "can't tell"},
	} {
		inside := c.inside
		if row := seccompRow("row", c.outside, probeRun{Facts: &inside}); row.Answer != c.want {
			t.Errorf("%s: %q (%s), want %q", c.name, row.Answer, row.Detail, c.want)
		}
	}
	if row := seccompRow("row", outside, probeRun{Err: "exit status 1"}); row.Answer != "no" {
		t.Errorf("a probe that failed: %q, want no", row.Answer)
	}
}

// The controls: a plain program's tmpfs is "yes" only when it mounted it inside a user namespace of its own, and
// the Landlock row says what the kernel said.
func TestTheControlRowsReadWhatTheProbeSaid(t *testing.T) {
	outside := linuxFacts{Namespaces: map[string]string{"user": "user:[1]"}}
	own := func(mount string) probeRun {
		return probeRun{Facts: &linuxFacts{Namespaces: map[string]string{"user": "user:[9]"}, UIDMap: "0 1001 1", Mount: mount}}
	}
	for _, c := range []struct {
		name string
		run  probeRun
		want string
	}{
		{"mounted in its own namespace", own("mounted"), "yes"},
		{"refused the mount", own("operation not permitted"), "no"},
		{"no namespace of its own", probeRun{Facts: &linuxFacts{Namespaces: map[string]string{"user": "user:[1]"}, Mount: "mounted"}}, "no"},
		{"couldn't start", probeRun{Err: "fork/exec: operation not permitted"}, "no"},
	} {
		if row := mountRow("row", outside, c.run); row.Answer != c.want {
			t.Errorf("%s: %q (%s), want %q", c.name, row.Answer, row.Detail, c.want)
		}
	}
	if row := landlockRow("row", probeRun{Facts: &linuxFacts{Landlock: "ABI 4"}}); row.Answer != "ABI 4" {
		t.Errorf("Landlock: %q, want ABI 4", row.Answer)
	}
	if row := landlockRow("row", probeRun{Facts: &linuxFacts{Landlock: "no: EOPNOTSUPP"}}); row.Answer != "no" || row.Detail != "EOPNOTSUPP" {
		t.Errorf("Landlock off: %q %q, want no and EOPNOTSUPP", row.Answer, row.Detail)
	}
	if row := landlockRow("row", probeRun{Err: "exit status 1"}); row.Answer != "can't tell" {
		t.Errorf("Landlock unread: %q, want can't tell", row.Answer)
	}
}

// Every row lands on one line of the table, whatever bwrap wrote into its detail.
func TestTheTableKeepsEachRowOnOneLine(t *testing.T) {
	table := capTable([]capRow{{"bwrap: user namespace", "no", "bwrap: setting up uid map:\nPermission denied | x"}})
	lines := strings.Split(strings.TrimSuffix(table, "\n"), "\n")
	want := `| bwrap: user namespace | no | bwrap: setting up uid map: Permission denied \| x |`
	if len(lines) != 3 || lines[2] != want {
		t.Errorf("the table:\n%s\nwant its last line %s", table, want)
	}
}
