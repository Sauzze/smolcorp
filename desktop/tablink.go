package desktop

import (
	"errors"
	"os"
	"path/filepath"
)

// A tab through a link (FXTABLINK, 1.26.0). A tmux tab names its server by a socket's path, and the hook says it: a
// desk chooses it. Had the core taken a tab whose path goes through a link of the desk's own (checked where the link
// led, kept as spelt), the desk could point the link elsewhere afterwards, at another desk's folder or its own, and
// the core would then drive that server with what it types into the desk's card, unchecked. So:
//   - a desk's tab is taken only with no link anywhere on its socket's path, the socket itself included; it is
//     resolved once, checked as resolved, and kept as resolved (hookkeys.go): the core drives that path, never a
//     spelling the desk gave;
//   - no tab whose link leads to a desk's socket is taken, whoever's key the event has;
//   - and the core drives no tab that goes through a link to a desk's socket, nor asks the Mac app to: commandTab and
//     macTab refuse it. That covers a session kept from before, and a link put later where a desk can write under a
//     tab kept with none (on a Mac, a sandboxed desk can write in the owner's tmux folder, /private/tmp/tmux-<uid>).
//     The check comes before tmux connects, so a link swapped in between still gets through: this narrows that race,
//     and S4 (FXTMUXDIR: a Policy row and a guard check) closes it.
//
// The one spelling with a link that counts is the core's own: a hire's socket exactly as its row records it, with no
// link at the socket itself (coreSpelt). A desk outside its sandbox on a Mac is given /tmp/tmux-<uid>/smolcorp-<id>,
// /tmp being the system's link, which no desk can point elsewhere; a sandboxed desk's socket is spelt with none.

// errTabThroughLink is the core's no to a tab whose socket is reached through a link to a desk's own.
var errTabThroughLink = errors.New("this tab's tmux is reached through a link to a desk's own, so nothing was sent to it")

// socketThroughLink is whether any part of socket's path, as spelt, is a link now: a folder on the way, or the socket
// itself. A part that isn't there is no link.
func socketThroughLink(socket string) bool {
	for part := filepath.Clean(socket); ; part = filepath.Dir(part) {
		if info, err := os.Lstat(part); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return true
		}
		if filepath.Dir(part) == part {
			return false
		}
	}
}

// coreSpelt is whether socket is a hire's socket spelt exactly as the core recorded it in hires, and is no link
// itself: the links on its folder's way are the core's choice, never a desk's.
func coreSpelt(socket string, hires []Hire) bool {
	clean := filepath.Clean(socket)
	if info, err := os.Lstat(clean); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	for _, h := range hires {
		if h.TmuxSocket != "" && filepath.Clean(h.TmuxSocket) == clean {
			return true
		}
	}
	return false
}

// socketThroughOthersLink is whether socket, a full path, goes through a link the core didn't choose (coreSpelt), as
// hires record the core's sockets.
func socketThroughOthersLink(socket string, hires []Hire) bool {
	return filepath.IsAbs(socket) && socketThroughLink(socket) && !coreSpelt(socket, hires)
}

// linkedDeskSocket is whether socket, a full path, goes through a link the core didn't choose to a desk's socket:
// where its links lead now is in the socket folder (deskSocketParent, any desk's folder, at any depth), or a hire's
// socket as its row records it.
func linkedDeskSocket(socket string) bool {
	if !filepath.IsAbs(socket) || !socketThroughLink(socket) {
		return false
	}
	hires := readRegistry(registryPath(), false).hires
	if coreSpelt(socket, hires) {
		return false
	}
	place, err := filepath.EvalSymlinks(socket)
	if err != nil {
		place = socketPlace(socket) // no socket there now: where its folder leads
	}
	if inDeskSocketFolder(place) {
		return true
	}
	for _, h := range hires {
		if h.TmuxSocket != "" && sameSocket(h.TmuxSocket, place) {
			return true
		}
	}
	return false
}

// sameFolderOnDisk says whether folder is parent, as the system finds them (a case or a firmlink apart, say).
func sameFolderOnDisk(folder string, parent os.FileInfo) bool {
	info, err := os.Stat(folder)
	return err == nil && os.SameFile(info, parent)
}

// insideOnDisk is whether place, a path with no links, is somewhere inside parent, as the system finds them.
func insideOnDisk(place, parent string) bool {
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return false
	}
	for folder := filepath.Dir(place); ; folder = filepath.Dir(folder) {
		if sameFolderOnDisk(folder, info) {
			return true
		}
		if filepath.Dir(folder) == folder {
			return false
		}
	}
}
