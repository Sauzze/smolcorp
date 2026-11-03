//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
)

// What may be cleaned up for a hire (Phase 6 T602, part C1a): only what is the
// hire's own. The hire's tmux server is the one socket its id and port name, in
// the support folder, and it must be the exact path recorded when the hire was
// made, a real socket and not a link. ownsHireSocket is broader (any hire's name, any id that looks like a
// port and an id), so a real install would count a test copy's socket as its
// own: cleanup never relies on it alone.

// hireOwnSocket is the socket the hire was made on, from its id and testPort
// (0 outside a test copy), or "" for an id that can't name one.
func hireOwnSocket(hireID string, testPort int) string {
	name := hireSocketName(hireID, testPort)
	if name == "" {
		return ""
	}
	return filepath.Join(supportDir, name)
}

// isHireSocket says whether hire's recorded tmux socket is exactly its own: the
// same file as its own name in the support folder, with the folders on the way
// resolved (tmux may report /private/var for /var), and that file a socket, not
// a link (and there). Anything else is not the hire's.
func isHireSocket(hire Hire, testPort int) bool {
	name := hireSocketName(hire.ID, testPort)
	recorded := hire.TmuxSocket
	if name == "" || !filepath.IsAbs(recorded) || strings.Contains(recorded, "..") {
		return false
	}
	recorded = filepath.Clean(recorded)
	if filepath.Base(recorded) != name || physical(filepath.Dir(recorded)) != physical(supportDir) {
		return false
	}
	info, err := os.Lstat(filepath.Join(physical(supportDir), name))
	return err == nil && info.Mode()&os.ModeSocket != 0 && info.Mode()&os.ModeSymlink == 0
}

// hireOwnTmux is the hire's tmux server, or nil when its recorded socket isn't
// exactly its own (so nothing is ended there).
func hireOwnTmux(hire Hire, testPort int, run tmuxRunner) *hireTmux {
	if !isHireSocket(hire, testPort) {
		return nil
	}
	// The same file, spelt as the support folder is: what hireTmuxAt knows.
	return hireTmuxAt(hireOwnSocket(hire.ID, testPort), testPort, run)
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
