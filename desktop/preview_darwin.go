//go:build darwin

package desktop

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Sauzze/smolcorp/internal/disclaim"
)

const (
	lsofProgram = "/usr/sbin/lsof" // the Mac's own, never one found on a PATH
	lsofTimeout = 3 * time.Second  // a run that takes longer is stopped
	lsofCap     = 64 << 10         // bytes of its answer; more is nothing found
)

// deskListeners is the TCP sockets listened on by a desk's processes that are this user's, as the Mac's own tables
// say: the processes from the kernel (sysctl kern.proc.all), from root (its process, or every process on its tty) and
// everything under them but cut's, then their sockets from lsof, asked about those processes alone, listening
// sockets alone, without names looked up. Nothing is connected to. false when it couldn't look.
func deskListeners(root previewRoot, cut []int) ([]tcpListener, bool) {
	all, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, false
	}
	var device unix.Stat_t
	if root.tty != "" && unix.Stat(root.tty, &device) != nil {
		return nil, true
	}
	uid := uint32(os.Getuid())
	parents, mine := map[int]int{}, map[int]bool{}
	var roots []int
	for i := range all {
		p := &all[i]
		pid := int(p.Proc.P_pid)
		parents[pid] = int(p.Eproc.Ppid)
		mine[pid] = p.Eproc.Pcred.P_ruid == uid && p.Eproc.Ucred.Uid == uid
		if mine[pid] && (pid == root.pid || root.tty != "" && p.Eproc.Tdev == device.Rdev) {
			roots = append(roots, pid)
		}
	}
	if len(roots) == 0 {
		return nil, true
	}
	var pids []string
	for _, one := range descendants(roots, parents, setOf(cut)) {
		if mine[one] {
			pids = append(pids, strconv.Itoa(one))
		}
	}
	if len(pids) == 0 {
		return nil, true
	}
	out, ok := runLsof("-nP", "-w", "-a", "-p", strings.Join(pids, ","), "-iTCP", "-sTCP:LISTEN", "-Fn")
	if !ok {
		return nil, false
	}
	return parseLsofListeners(out), true
}

// connectionOwner is this user's process holding the TCP connection from one address to another, as lsof says, or 0.
func connectionOwner(from, to *net.TCPAddr) int {
	out, ok := runLsof("-nP", "-w", "-a", "-u", strconv.Itoa(os.Getuid()), "-iTCP@"+from.String(), "-sTCP:ESTABLISHED", "-Fpn")
	if !ok {
		return 0
	}
	return parseLsofConnection(out, from.String(), to.String())
}

// runLsof runs the Mac's lsof with args, with a clean environment, a deadline and a cap on its answer. lsof says 1
// when nothing matched: that is an answer, empty.
func runLsof(args ...string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), lsofTimeout)
	defer cancel()
	cmd := disclaim.Cmd(exec.CommandContext(ctx, lsofProgram, args...))
	cmd.Dir = os.TempDir()
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C"}
	out := &capped{cap: lsofCap}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		if exit, isExit := err.(*exec.ExitError); isExit && exit.ExitCode() == 1 && ctx.Err() == nil && len(out.Bytes()) == 0 {
			return nil, true
		}
		return nil, false
	}
	return out.Bytes(), true
}
