//go:build linux

package desktop

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// deskListeners is the TCP sockets listened on by a desk's processes that are this user's, as /proc says: the
// processes' parents and ttys from their stat, from root (its process, or every process on its tty) and everything
// under them but cut's, their sockets from their fd links, and which of those listen from /proc/net/tcp and tcp6.
// Only read; nothing is connected to. false when it couldn't look.
func deskListeners(root previewRoot, cut []int) ([]tcpListener, bool) {
	var device unix.Stat_t
	if root.tty != "" && unix.Stat(root.tty, &device) != nil {
		return nil, true
	}
	parents, mine, ttys, ok := procTable()
	if !ok {
		return nil, false
	}
	var roots []int
	for pid := range mine {
		if pid == root.pid || root.tty != "" && ttys[pid] != 0 && sameDevice(ttys[pid], uint64(device.Rdev)) {
			roots = append(roots, pid)
		}
	}
	if len(roots) == 0 {
		return nil, true
	}
	inodes := map[string]bool{}
	for _, one := range descendants(roots, parents, setOf(cut)) {
		if !mine[one] {
			continue
		}
		for _, inode := range socketsOf(one) {
			inodes[inode] = true
		}
	}
	if len(inodes) == 0 {
		return nil, true
	}
	var found []tcpListener
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		if data, err := os.ReadFile(table); err == nil {
			found = append(found, parseProcNetTCP(data, inodes)...)
		}
	}
	return found, true
}

// connectionOwner is this user's process holding the TCP connection from one address to another, as /proc says: the
// connection's socket from /proc/net/tcp or tcp6, then the process with that socket among its fds. 0 for none.
func connectionOwner(from, to *net.TCPAddr) int {
	inode := ""
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		if data, err := os.ReadFile(table); err == nil && inode == "" {
			inode = parseProcNetConnection(data, from, to)
		}
	}
	if inode == "" {
		return 0
	}
	_, mine, _, ok := procTable()
	if !ok {
		return 0
	}
	for pid := range mine {
		for _, one := range socketsOf(pid) {
			if one == inode {
				return pid
			}
		}
	}
	return 0
}

// procTable is every process's parent, which are this user's, and each one's tty (tty_nr, 0 for none), from /proc.
func procTable() (parents map[int]int, mine map[int]bool, ttys map[int]uint64, ok bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, nil, nil, false
	}
	uid := uint32(os.Getuid())
	parents, mine, ttys = map[int]int{}, map[int]bool{}, map[int]uint64{}
	for _, entry := range entries {
		one, err := strconv.Atoi(entry.Name())
		if err != nil || one <= 0 {
			continue
		}
		info, err := os.Stat(filepath.Join("/proc", entry.Name()))
		if err != nil {
			continue
		}
		if stat, isStat := info.Sys().(*syscall.Stat_t); isStat && stat.Uid == uid {
			mine[one] = true
		}
		if parent, tty, read := procStat(one); read {
			parents[one], ttys[one] = parent, tty
		}
	}
	return parents, mine, ttys, true
}

// procStat is a process's parent and tty (tty_nr) from its /proc stat: pid (comm) state ppid pgrp session tty_nr …,
// read after comm's last parenthesis, as comm may hold spaces.
func procStat(pid int) (parent int, tty uint64, ok bool) {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	end := bytes.LastIndexByte(stat, ')')
	if err != nil || end < 0 {
		return 0, 0, false
	}
	rest := strings.Fields(string(stat[end+1:]))
	if len(rest) < 5 {
		return 0, 0, false
	}
	parent, perr := strconv.Atoi(rest[1])
	tty, terr := strconv.ParseUint(rest[4], 10, 64)
	return parent, tty, perr == nil && terr == nil
}

// sameDevice is whether a tty_nr (major in bits 8-19, minor in 0-7 and 20-31) is the device rdev.
func sameDevice(ttyNR, rdev uint64) bool {
	major, minor := uint32((ttyNR>>8)&0xfff), uint32((ttyNR&0xff)|((ttyNR>>12)&0xfff00))
	return major == unix.Major(rdev) && minor == unix.Minor(rdev)
}

// socketsOf is the sockets (their inodes) among a process's fds.
func socketsOf(pid int) []string {
	folder := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	var inodes []string
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(folder, fd.Name()))
		if inode, isSocket := strings.CutPrefix(target, "socket:["); err == nil && isSocket {
			inodes = append(inodes, strings.TrimSuffix(inode, "]"))
		}
	}
	return inodes
}
