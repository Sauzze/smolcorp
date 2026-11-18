package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// SA-L0's probes. Each starts this test program again as a stand-in, which reports what it sees on one line of
// its output; the outside is the same report from this process. Nothing here needs root or changes a setting: the
// probes run as the runner's own user, the way a desk would. With SMOLCORP_LINUXCAPS_REQUIRED=1 (the linuxcaps CI
// job) a missing bwrap fails the run instead of skipping it; SMOLCORP_LINUXCAPS_TABLE names a file for the table.

const (
	standInProbe = "SMOLCORP_LINUXCAPS_PROBE" // the probe folder: set only in a stand-in
	standInMount = "SMOLCORP_LINUXCAPS_MOUNT" // "1": the stand-in mounts a tmpfs over the probe folder itself
	factsPrefix  = "linuxcaps-facts: "
)

// The stand-in: only runs as one of the probes below.
func TestLinuxCapsStandIn(t *testing.T) {
	probe := os.Getenv(standInProbe)
	if probe == "" {
		t.Skip("runs only as a probe's stand-in")
	}
	line, err := json.Marshal(gatherFacts(probe, os.Getenv(standInMount) == "1"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(factsPrefix + string(line))
}

// The numbers the any-system filter and its reading are written with are the kernel's, as x/sys has them.
func TestTheProbeFilterUsesTheKernelsNumbers(t *testing.T) {
	arch, ok := probeArches[runtime.GOARCH]
	if !ok {
		t.Fatalf("no probe filter for this runner's processor, %s", runtime.GOARCH)
	}
	wantAudit := map[string]uint32{"amd64": unix.AUDIT_ARCH_X86_64, "arm64": unix.AUDIT_ARCH_AARCH64}[runtime.GOARCH]
	for _, c := range []struct {
		name      string
		got, want uint64
	}{
		{"AUDIT_ARCH", uint64(arch.audit), uint64(wantAudit)},
		{"SYS_IOCTL", uint64(arch.ioctl), unix.SYS_IOCTL},
		{"TIOCSTI", ioctlTIOCSTI, unix.TIOCSTI},
		{"TIOCLINUX", ioctlTIOCLINUX, unix.TIOCLINUX},
		{"SECCOMP_RET_ALLOW", seccompAllow, unix.SECCOMP_RET_ALLOW},
		{"SECCOMP_RET_ERRNO", seccompErrno, unix.SECCOMP_RET_ERRNO},
		{"EPERM", errnoEPERM, uint64(unix.EPERM)},
		{"BPF_LD|BPF_W|BPF_ABS", bpfLoadWord, unix.BPF_LD | unix.BPF_W | unix.BPF_ABS},
		{"BPF_JMP|BPF_JEQ|BPF_K", bpfJumpIfEq, unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K},
		{"BPF_RET|BPF_K", bpfReturn, unix.BPF_RET | unix.BPF_K},
	} {
		if c.got != c.want {
			t.Errorf("%s: the probe uses %#x, the kernel's is %#x", c.name, c.got, c.want)
		}
	}
}

// The table: the system, its settings, and each probe's answer, recorded whatever they are. It fails only when the
// rows can't mean anything (the outside's namespaces unreadable) or, in CI, when bwrap is missing.
func TestLinuxSandboxCapabilitiesAreRecorded(t *testing.T) {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		if os.Getenv("SMOLCORP_LINUXCAPS_REQUIRED") == "1" {
			t.Fatalf("no bwrap on this runner: %v", err)
		}
		t.Skip("no bwrap here (the linuxcaps CI job installs it): ", err)
	}
	outside := gatherFacts(t.TempDir(), false)
	for kind, link := range outside.Namespaces {
		if !strings.HasPrefix(link, kind+":[") {
			t.Fatalf("this process's %s namespace can't be read (%q): no row could be compared", kind, link)
		}
	}
	filter, err := probeFilter(runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	rows := systemRows(t, bwrap)
	probe := func(extra ...string) probeRun { return runBwrap(t, bwrap, nil, extra...) }

	full := runBwrap(t, bwrap, filter, "--unshare-user", "--unshare-pid", "--unshare-net", "--proc", "/proc", "--tmpfs")
	rows = append(rows,
		namespaceRow("bwrap: user namespace", "user", outside, probe("--unshare-user")),
		namespaceRow("bwrap: pid namespace", "pid", outside, probe("--unshare-pid", "--proc", "/proc")),
		namespaceRow("bwrap: mount namespace, a tmpfs over a folder", "mnt", outside, probe("--tmpfs")),
		namespaceRow("bwrap: network namespace", "net", outside, probe("--unshare-net")),
		seccompRow("bwrap: --seccomp refuses TIOCSTI and TIOCLINUX", outside, runBwrap(t, bwrap, filter)),
		capRow{"bwrap: all of them in one start", allYes(
			namespaceRow("", "user", outside, full), namespaceRow("", "pid", outside, full),
			namespaceRow("", "mnt", outside, full), namespaceRow("", "net", outside, full),
			seccompRow("", outside, full)), "user, pid, mount and network namespaces and the filter together"},
		landlockRow("Landlock ABI, outside", probeRun{Facts: &outside}),
		landlockRow("Landlock ABI, inside bwrap with the filter", full),
		namespaceRow("control: a copy of bwrap outside /usr/bin, user namespace", "user", outside,
			runBwrap(t, copyOf(t, bwrap), nil, "--unshare-user")),
		mountRow("control: a plain program's own user namespace, a tmpfs mounted in it", outside, runPlain(t)),
	)
	table := capTable(rows)
	t.Logf("SA-L0, what this runner allows (%s/%s):\n%s", runtime.GOOS, runtime.GOARCH, table)
	if path := os.Getenv("SMOLCORP_LINUXCAPS_TABLE"); path != "" {
		if err := os.WriteFile(path, []byte(table), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// allYes is "yes" when every row is, else the first other answer with its row's reason.
func allYes(rows ...capRow) string {
	for _, row := range rows {
		if row.Answer != "yes" {
			return row.Answer + " (" + row.Detail + ")"
		}
	}
	return "yes"
}

// systemRows are the facts that explain the answers: the system, the kernel, bwrap as packaged, the settings that
// limit user namespaces, the security modules, and AppArmor's profiles that name bwrap.
func systemRows(t *testing.T, bwrap string) []capRow {
	var uname unix.Utsname
	_ = unix.Uname(&uname)
	rows := []capRow{
		{"system", osRelease(), ""},
		{"kernel", unix.ByteSliceToString(uname.Release[:]), unix.ByteSliceToString(uname.Machine[:])},
	}
	info, err := os.Stat(bwrap)
	if err != nil {
		t.Fatalf("bwrap was found but can't be read: %v", err)
	}
	owner := "?"
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		owner = fmt.Sprintf("uid %d", st.Uid)
	}
	version, _ := exec.Command(bwrap, "--version").CombinedOutput()
	pkg, _ := exec.Command("dpkg-query", "-W", "-f=${Version}", "bubblewrap").Output()
	rows = append(rows, capRow{"bwrap", bwrap, fmt.Sprintf("%s, owned by %s, mode %v, package %s",
		strings.TrimSpace(string(version)), owner, info.Mode(), orNone(string(pkg)))})
	for _, setting := range []string{
		"/proc/sys/kernel/apparmor_restrict_unprivileged_userns",
		"/proc/sys/kernel/apparmor_restrict_unprivileged_unconfined",
		"/proc/sys/kernel/unprivileged_userns_clone",
		"/proc/sys/user/max_user_namespaces",
	} {
		rows = append(rows, capRow{strings.TrimPrefix(setting, "/proc/sys/"), readValue(setting), ""})
	}
	rows = append(rows, capRow{"security modules", readValue("/sys/kernel/security/lsm"), ""})
	var profiles []string
	entries, _ := os.ReadDir("/etc/apparmor.d")
	for _, entry := range entries {
		text, err := os.ReadFile(filepath.Join("/etc/apparmor.d", entry.Name()))
		if err == nil && (strings.Contains(entry.Name(), "bwrap") || strings.Contains(string(text), "/bwrap")) {
			profiles = append(profiles, entry.Name())
		}
	}
	return append(rows, capRow{"AppArmor profiles naming bwrap", orNone(strings.Join(profiles, ", ")), ""})
}

// runBwrap starts the stand-in under bwrap with the root read-only, a fresh /dev, and the extra words. A trailing
// "--tmpfs" gets the probe folder; a filter is handed over on descriptor 3 with --seccomp.
func runBwrap(t *testing.T, bwrap string, filter []byte, extra ...string) probeRun {
	probe := t.TempDir()
	args := []string{"--ro-bind", "/", "/", "--dev", "/dev", "--die-with-parent"}
	args = append(args, extra...)
	if len(extra) > 0 && extra[len(extra)-1] == "--tmpfs" {
		args = append(args, probe)
	}
	var files []*os.File
	if filter != nil {
		path := filepath.Join(t.TempDir(), "filter.bpf")
		if err := os.WriteFile(path, filter, 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		files = append(files, file)
		args = append(args, "--seccomp", "3")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args = append(args, "--", self, "-test.run=^TestLinuxCapsStandIn$")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bwrap, args...)
	cmd.Env = append(os.Environ(), standInProbe+"="+probe)
	cmd.ExtraFiles = files
	return collect(cmd)
}

// runPlain is the control without bwrap: this program, unconfined, making its own user and mount namespaces and
// mounting a tmpfs inside. Ubuntu 24.04's rule lets the namespace be made but takes the capabilities in it away.
func runPlain(t *testing.T) probeRun {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, "-test.run=^TestLinuxCapsStandIn$")
	cmd.Env = append(os.Environ(), standInProbe+"="+t.TempDir(), standInMount+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	return collect(cmd)
}

// collect runs a probe and reads its facts line, keeping the end of its error output for the table.
func collect(cmd *exec.Cmd) probeRun {
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	run := probeRun{Stderr: strings.TrimSpace(stderr.String())}
	if len(run.Stderr) > 300 {
		run.Stderr = "…" + run.Stderr[len(run.Stderr)-300:]
	}
	if err != nil {
		run.Err = err.Error()
		return run
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line, ok := strings.CutPrefix(scanner.Text(), factsPrefix); ok {
			var facts linuxFacts
			if err := json.Unmarshal([]byte(line), &facts); err != nil {
				run.Err = "its facts don't parse: " + err.Error()
				return run
			}
			run.Facts = &facts
		}
	}
	return run
}

// copyOf copies bwrap outside /usr/bin, where no AppArmor profile names it.
func copyOf(t *testing.T, bwrap string) string {
	data, err := os.ReadFile(bwrap)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bwrap")
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// gatherFacts is what a probe sees from where it runs. Asked to, it first mounts a tmpfs over the probe folder.
func gatherFacts(probe string, mount bool) linuxFacts {
	f := linuxFacts{Namespaces: map[string]string{}, Pid: os.Getpid()}
	for _, kind := range []string{"user", "pid", "mnt", "net"} {
		link, err := os.Readlink("/proc/self/ns/" + kind)
		if err != nil {
			link = "error: " + errnoName(err)
		}
		f.Namespaces[kind] = link
	}
	uidMap, _ := os.ReadFile("/proc/self/uid_map")
	f.UIDMap = strings.Join(strings.Fields(string(uidMap)), " ")
	if interfaces, err := net.Interfaces(); err == nil {
		for _, i := range interfaces {
			f.Interfaces = append(f.Interfaces, i.Name)
		}
	}
	if mount {
		f.Mount = "mounted"
		if err := unix.Mount("tmpfs", probe, "tmpfs", 0, "size=1m"); err != nil {
			f.Mount = errnoName(err)
		}
	}
	var st unix.Stat_t
	if unix.Stat(probe, &st) == nil {
		f.ProbeDev = uint64(st.Dev)
	}
	// On /dev/null, never a terminal, and with no buffer: nothing can be typed anywhere, whatever the answer.
	if null, err := os.Open("/dev/null"); err == nil {
		f.TIOCSTI = ioctlResult(int(null.Fd()), unix.TIOCSTI)
		f.TIOCLINUX = ioctlResult(int(null.Fd()), unix.TIOCLINUX)
		null.Close()
	}
	status, _ := os.ReadFile("/proc/self/status")
	for _, line := range strings.Split(string(status), "\n") {
		if value, ok := strings.CutPrefix(line, "Seccomp:"); ok {
			f.Seccomp = strings.TrimSpace(value)
		} else if value, ok := strings.CutPrefix(line, "NoNewPrivs:"); ok {
			f.NoNewPrivs = strings.TrimSpace(value)
		}
	}
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	f.Landlock = fmt.Sprintf("ABI %d", abi)
	if errno != 0 {
		f.Landlock = "no: " + unix.ErrnoName(errno)
	}
	return f
}

func ioctlResult(fd int, request uint) string {
	if err := unix.IoctlSetInt(fd, request, 0); err != nil {
		return errnoName(err)
	}
	return "ok"
}

// errnoName gives an error's errno by its name (EPERM), or its text when it has none.
func errnoName(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if name := unix.ErrnoName(errno); name != "" {
			return name
		}
	}
	return err.Error()
}

func readValue(path string) string {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "absent"
	} else if err != nil {
		return "unreadable: " + errnoName(err)
	}
	return strings.TrimSpace(string(data))
}

func osRelease() string {
	data, _ := os.ReadFile("/etc/os-release")
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(value, `"`)
		}
	}
	return "unknown"
}

func orNone(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return "none"
	}
	return s
}
