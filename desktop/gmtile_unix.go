//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// hireClientTTY is the tty of the first client attached to a hire's tmux session, which its window shows: "" for
// none, or a socket that isn't one of ours.
func hireClientTTY(socket string, testPort int) string {
	server := hireTmuxAt(socket, testPort, nil)
	if server == nil {
		return ""
	}
	if ttys, ok := server.clientTTYs(); ok && len(ttys) > 0 {
		return ttys[0]
	}
	return ""
}

// teamGridSession is the one session on the team grid's server.
const teamGridSession = "team"

// teamGridSocket is the team grid's own server, beside the hires' (a test copy's: smolcorp-<port>-team), as the
// Mac's TeamGrid names it for tmux -L.
func teamGridSocket(testPort int) string {
	return filepath.Join(hireSocketFolder(), hireSocketPrefix+testPortPart(testPort)+teamGridSession)
}

// teamGridCommands are tmux's arguments, one list per call, making the session on socket with one pane for each
// of panes (their commands), tiled. The first starts the grid's server (tmuxServerStart: the guard variable taken out of
// its environment) and prints its pid, which every later call's guard is told.
func teamGridCommands(socket string, panes []string) [][]string {
	if len(panes) == 0 {
		return nil
	}
	target := "=" + teamGridSession + ":"
	calls := [][]string{tmuxServerStart("new-session", "-d", "-P", "-F", "#{pid}", "-s", teamGridSession, "-x", "240", "-y", "60", panes[0]),
		{"set-option", "-t", target, "status", "off"}}
	for _, pane := range panes[1:] {
		calls = append(calls, []string{"split-window", "-t", target, pane}, []string{"select-layout", "-t", target, "tiled"})
	}
	calls = append(calls, []string{"select-layout", "-t", target, "tiled"})
	for i, call := range calls {
		calls[i] = append([]string{"-S", socket}, call...)
	}
	return calls
}

// showTeamGrid makes the grid of desks ids afresh on its own server (an old one is ended first: only that socket's) and
// shows it in a window of terminal, opened by the Mac app on a window's script, as a hire's window is
// (writeWindowScript): no start script (SA-M7). The server reads no start file (-f /dev/null) and is started with its
// guard off for that call alone (tmuxStartCall); every other call is a one-shot client's (tmuxClient), told the pid the
// start printed, so the guard checks each call reaches that server. Each pane, and the window, run `smolcorp attach`
// with exec (SA-M8): a pane, its desk read-only, in that desk's sandbox where it has one; the window, the grid, told
// its server's pid. False if any step failed.
var showTeamGrid = func(h hostBridge, ids []string, terminal string, testPort int) bool {
	tmux := tmuxPath()
	if tmux == "" || !ownSocketFolder(hireSocketFolder()) {
		return false
	}
	socket, smolcorp := teamGridSocket(testPort), commandLineProgram()
	var panes []string
	for _, id := range ids {
		if hire, ok := registryHire(id); ok && hireOwnTmux(hire, testPort, nil) != nil {
			panes = append(panes, gridPaneLine(smolcorp, id, testPort, supportDir))
		}
	}
	if len(panes) == 0 {
		return false
	}
	// An old grid's server: its pid was never kept, so it is ended unchecked, on its own socket alone.
	_, _, _ = oneShotTmux(tmux, func() string { return tmuxUnchecked })([]string{"-S", socket, "kill-server"})
	calls := teamGridCommands(socket, panes)
	out, status, ran := withoutStartFile(tmuxStartCall(tmux))(calls[0])
	server, err := strconv.Atoi(strings.TrimSpace(out))
	if !ran || status != 0 || err != nil || server <= 1 {
		return false
	}
	run := oneShotTmux(tmux, func() string { return strconv.Itoa(server) })
	for _, call := range calls[1:] {
		if _, status, ran := run(call); !ran || status != 0 {
			return false
		}
	}
	script, err := writeWindowScript(filepath.Join(supportDir, "hires"), gridAttachLine(smolcorp, server), testPort, supportDir)
	if err != nil {
		return false
	}
	if _, _, err := hostOpenTerminal(h, terminal, homeDir, script); err != nil {
		_ = os.Remove(script)
		return false
	}
	return true
}
