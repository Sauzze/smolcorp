package desktop

import "testing"

// RT126SWEEP3 part 2, B2(a) and F1: the release notes' Known limits and the keys page's "What this doesn't stop" each
// say, plainly, that a hire reaches the owner's local servers that run commands (and what Smolcorp holds of them), and
// that a hire's window reaches the owner's terminal app, its clipboard included. Rancher Desktop's key folder and a
// moved LIMA_HOME or COLIMA_HOME, after the P1 review (RT126SWEEP3FIXP2). And a network or FUSE-T volume over git's
// hooks, which stops every desk's start and stop (RT126SWEEP3FIXP1R2 O1).
func TestTheKnownLimitsSayLocalServersAndATerminalWindow(t *testing.T) {
	for _, doc := range [][]string{{"RELEASE-NOTES.md"}, {"docs", "keys-and-guard.md"}} {
		text := repoText(t, doc...)
		mustSay(t, doc[len(doc)-1], text,
			// B2(a): the class, the examples, why the sandbox can't tell, and the VM tools' key folders it holds.
			"can still connect to most servers on your Mac's own addresses",
			"since the sandbox can't tell yours from the ones a hire starts for its work",
			"One that runs commands or reads files for whoever connects does that for a hire too",
			"a virtual machine's SSH port, Node's inspector, Jupyter, a browser's debugging port",
			"Smolcorp holds the SSH keys of Lima, Colima, Rancher Desktop and OrbStack machines, and of Podman's default machine",
			"`~/.lima/_config`, `~/.colima/_lima/_config`, `~/Library/Application Support/rancher-desktop/lima/_config`, `~/.orbstack/ssh` and Podman's default machine keys",
			"A `LIMA_HOME` or `COLIMA_HOME` moved elsewhere isn't followed, so keys kept there stay readable.",
			// RT126SWEEP3FIXP2 O7: the keys page's bullet says "a hire", as the release notes do.
			"A hire in its sandbox can still connect",
			// F1: the terminal app, the sequences it acts on, the clipboard, and what to do.
			"**A hire's window in your terminal app.**",
			"including sequences some terminals act on, such as one that sets your clipboard (OSC 52)",
			"Don't paste what a hire's window put on your clipboard without reading it first.",
			// RT126SWEEP3FIXP1R2 O1: an owner's own share or FUSE-T volume over git's hooks stops every desk's start
			// and stop while it's mounted.
			"If a network share or a FUSE-T volume (a vault, an rclone mount) is mounted in your home folder and holds git's hooks, or a file git's config names, at its top, no desk starts or stops while it's mounted.",
			"unmount it to go on.")
	}
}
