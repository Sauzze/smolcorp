package desktop

import "strconv"

// The lines Smolcorp writes for a window or a pane to run, each the whole command, its program the app's own smolcorp.

// attachLine shows desk id in a window.
func attachLine(smolcorp, id string) []string { return []string{smolcorp, "attach", id} }

// plainAttachLine shows a plain hire's session, on the socket tmux made it on, in a window.
func plainAttachLine(smolcorp, session, socket string) []string {
	return []string{smolcorp, "attach", "--session", session, "--socket", socket}
}

// gridAttachLine shows the team grid, whose server has pid server, in a window.
func gridAttachLine(smolcorp string, server int) []string {
	return []string{smolcorp, "attach", "--team-grid", strconv.Itoa(server)}
}

// gridPaneLine is what a pane of the team grid runs, as a shell line: desk id, read-only, so typing in the grid can't
// reach it. A test copy's pane names the copy's port and folder, as a window's script does (windowScriptLines).
func gridPaneLine(smolcorp, id string, testPort int, testSupport string) string {
	line := "exec " + shellQuoted(smolcorp) + " attach --read-only " + shellQuoted(id)
	if testPort != 0 {
		line = "SMOLCORP_PORT=" + shellQuoted(strconv.Itoa(testPort)) + " SMOLCORP_TEST_SUPPORT_DIR=" + shellQuoted(testSupport) + " " + line
	}
	return line
}
