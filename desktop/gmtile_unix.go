//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
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
// of panes (their commands), tiled.
func teamGridCommands(socket string, panes []string) [][]string {
	if len(panes) == 0 {
		return nil
	}
	target := "=" + teamGridSession + ":"
	calls := [][]string{{"new-session", "-d", "-s", teamGridSession, "-x", "240", "-y", "60", panes[0]},
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

// teamGridPane is what a pane runs: a read-only attach to one hire's session, so typing in the grid can't reach it.
func teamGridPane(tmux, hireSocket string) string {
	return "exec " + shellQuoted(tmux) + " -S " + shellQuoted(hireSocket) + " attach -r -t " + shellQuoted("="+hireSession)
}

// showTeamGrid makes the grid afresh on its own server (an old one is ended first: only that socket's) and shows it
// in a window of terminal, opened by the Mac app on a script in the hires folder. False if any step failed.
var showTeamGrid = func(h hostBridge, sockets []string, terminal string, testPort int) bool {
	tmux := tmuxPath()
	if tmux == "" || !ownSocketFolder(hireSocketFolder()) {
		return false
	}
	run := realTmux(tmux)
	socket := teamGridSocket(testPort)
	var panes []string
	for _, hire := range sockets {
		if hireTmuxAt(hire, testPort, run) != nil {
			panes = append(panes, teamGridPane(tmuxDriving(hire), hire))
		}
	}
	if len(panes) == 0 {
		return false
	}
	_, _, _ = run([]string{"-S", socket, "kill-server"})
	for _, call := range teamGridCommands(socket, panes) {
		if _, status, ran := run(call); !ran || status != 0 {
			return false
		}
	}
	line := "exec " + shellQuoted(tmux) + " -S " + shellQuoted(socket) + " attach-session -r -t " + shellQuoted("="+teamGridSession)
	script, err := writeHireStartScript(filepath.Join(supportDir, "hires", teamGridSession), "#!/bin/sh\n# Smolcorp's team grid: runs once.\nrm -f \"$0\"\n"+line+"\n")
	if err != nil {
		return false
	}
	if _, _, err := hostOpenTerminal(h, terminal, homeDir, script); err != nil {
		_ = os.Remove(script)
		return false
	}
	return true
}
