//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
)

// What may be cleaned up for a hire (Phase 6 T602, part C1a): only what is the
// hire's own. The hire's tmux server is the one socket its id and port name, in
// a hire socket folder (tmux's own, or the support folder for a hire made before
// 1.25), and it must be the exact path recorded when the hire was made, a real
// socket and not a link. ownsHireSocket is broader (any hire's name, any id that looks like a
// port and an id), so a real install would count a test copy's socket as its
// own: cleanup never relies on it alone.

// hireOwnSocket is where a new hire's socket is made, from its id and testPort (0 outside a test copy), or
// "" for an id that can't name one.
func hireOwnSocket(hireID string, testPort int) string {
	name := hireSocketName(hireID, testPort)
	if name == "" {
		return ""
	}
	return filepath.Join(hireSocketFolder(), name)
}

// recordedOwnSocket is the socket hire was made on, spelt as its socket folder is, or "" when its recorded
// tmux socket isn't exactly its own: the same file as its own name (from its id and testPort, 0 outside a
// test copy) in a hire socket folder, with the folders on the way resolved (tmux may report /private/tmp
// for /tmp), and that file a socket, not a link (and there). Anything else is not the hire's.
func recordedOwnSocket(hire Hire, testPort int) string {
	name := hireSocketName(hire.ID, testPort)
	recorded := hire.TmuxSocket
	if name == "" || !filepath.IsAbs(recorded) || strings.Contains(recorded, "..") {
		return ""
	}
	recorded = filepath.Clean(recorded)
	if filepath.Base(recorded) != name {
		return ""
	}
	for _, folder := range hireSocketFolders() {
		if physical(filepath.Dir(recorded)) != physical(folder) {
			continue
		}
		info, err := os.Lstat(filepath.Join(physical(folder), name))
		if err == nil && info.Mode()&os.ModeSocket != 0 && info.Mode()&os.ModeSymlink == 0 {
			return filepath.Join(folder, name)
		}
	}
	return ""
}

// isHireSocket says whether hire's recorded tmux socket is exactly its own (recordedOwnSocket).
func isHireSocket(hire Hire, testPort int) bool { return recordedOwnSocket(hire, testPort) != "" }

// hireOwnTmux is the hire's tmux server, or nil when its recorded socket isn't
// exactly its own (so nothing is ended there).
func hireOwnTmux(hire Hire, testPort int, run tmuxRunner) *hireTmux {
	socket := recordedOwnSocket(hire, testPort)
	if socket == "" {
		return nil
	}
	// The same file, spelt as its socket folder is: what hireTmuxAt knows.
	return hireTmuxAt(socket, testPort, run)
}

// removeHireWorktree is cleanUpHire's callback: removeOwnWorktree's checks,
// and the hire's tmux socket must be exactly its own.
func removeHireWorktree(git gitRunner, testPort int, worktrees string) func(Hire) bool {
	return func(h Hire) bool {
		if !isHireSocket(h, testPort) {
			return false
		}
		return removeOwnWorktree(git, worktrees, h)
	}
}

// detachHireViewers is hiresToBackground's detach for a hire on its own tmux
// server, through run (nil: the real tmux): how many windows were attached,
// and whether they're detached now. It only lists and detaches: the session
// and the server are never ended. A hire whose socket isn't exactly its own is
// left as it is.
func detachHireViewers(hire Hire, run tmuxRunner) detached {
	server := hireOwnTmux(hire, testCopyPort(), run)
	if server == nil {
		return detached{}
	}
	viewers := server.viewers()
	if viewers < 0 {
		return detached{own: true}
	}
	if viewers == 0 {
		return detached{own: true, asked: true, ok: true}
	}
	return detached{own: true, asked: true, viewers: viewers, ok: server.detachViewers()}
}

// openHireViewer is "Open a window" for a hire on its own tmux server: a
// window of terminal attached to its session, in its worktree or folder.
func openHireViewer(hire Hire, terminal hireOpener) error {
	server := hireOwnTmux(hire, testCopyPort(), nil)
	if server == nil {
		return errUnsupported
	}
	if alive, _ := server.alive(); !alive {
		return errUnsupported
	}
	folder := hire.Worktree
	if folder == "" {
		folder = hire.Folder
	}
	window, err := showWindow(terminal, folder, []string{cmpOr(server.program, tmuxPath()), "-S", server.socket, "attach", "-t", "=" + hireSession})
	recordHireWindow(hire.ID, window)
	return err
}
