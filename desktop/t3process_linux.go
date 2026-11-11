//go:build linux

package desktop

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// processesWorkingIn is the processes working in folder (or a folder inside it), or given it with --add-dir,
// as T3 Code's agent is (T797a): read from /proc, nothing run. A process that isn't yours can't be read,
// and isn't T3 Code's agent, which runs as you. False when /proc can't be listed.
func processesWorkingIn(folder string) ([]int, bool) {
	folders := t3PlacesOf(folder)
	return processesWhere(func(cwd string, args []string) bool { return t3WorksIn(folders, cwd, args, "/", false) })
}

// processesHolding is the processes that work in folder as processesWorkingIn finds them, or whose command line
// names Claude Code session session (Hand back's check, t3takeover.go).
func processesHolding(folder, session string) ([]int, bool) {
	folders := t3PlacesOf(folder)
	return processesWhere(func(cwd string, args []string) bool {
		return t3WorksIn(folders, cwd, args, "/", false) || t3NamesSession(args, session)
	})
}

// processesWhere is the processes whose current folder and arguments match, read from /proc.
func processesWhere(match func(cwd string, args []string) bool) ([]int, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	var found []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		place := filepath.Join("/proc", entry.Name())
		cwd, _ := os.Readlink(filepath.Join(place, "cwd"))
		line, _ := os.ReadFile(filepath.Join(place, "cmdline"))
		args := strings.Split(strings.TrimRight(string(line), "\x00"), "\x00")
		if match(cwd, args) {
			found = append(found, pid)
		}
	}
	return found, true
}

// processName is a process's name as the kernel keeps it (zsh, node…), or "".
func processName(pid int) string {
	comm, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(comm))
}
