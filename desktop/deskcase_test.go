//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// FXDESKCASE: a sandboxed desk's place spelt any way but the one deskSocketID reads gets nothing run on it, never a
// tmux of your own (RT126M9AB12, "Out of scope" 1). On a Mac, /tmp is case-insensitive: …/AB12/s is desk ab12's own
// socket, so taking it for a tmux of your own would drive a desk's server unchecked, perhaps with Homebrew's tmux. The
// Mac app's HireTmux.isDeskSocket judges the same way, by place, not by name.

// tabSitesRun drives a tab on socket through the core's tmux tab sites (keys, text, paste, looks, focus, scroll, the
// preview's look; never a close, which fires) and gives what the stand-in ran meanwhile.
func tabSitesRun(t *testing.T, log, socket string) []tmuxGateRun {
	t.Helper()
	from := len(tmuxGateRuns(t, log, 0))
	pane := commandTab{kind: "tmux", id: "%1", socket: socket}
	pane.alive()
	pane.tty()
	_ = pane.Press(press(Enter))
	_ = pane.Type("hello")
	_ = pane.Focus()
	_ = pane.Scroll(-3)
	_, _ = pane.screen()
	_, _ = pane.output("display-message", "-p", "-t", "%1", "#{pane_id}")
	_, _ = pane.phoneText("one line", false)
	_, _ = pane.phoneText("two\nlines", true)
	previewRootOf("tmux:%1@"+socket, nil)
	return tmuxGateRuns(t, log, from)
}

// noDeskGetsNothing checks a desk-shaped socket that isn't a desk's: tmuxFor gives the bundled tmux and guard "", the
// Mac app is sent no desk for it, and none of the tab's sites runs anything.
func noDeskGetsNothing(t *testing.T, program, log, socket string) {
	t.Helper()
	if got, guard := tmuxFor(socket); got != program || guard != "" {
		t.Errorf("tmuxFor(%q) = %q, %q; want the bundled tmux, guard \"\" (nothing run)", socket, got, guard)
	}
	if desk := deskOfPane(socket); desk != nil {
		t.Errorf("deskOfPane(%q) = %+v, want none", socket, desk)
	}
	if runs := tabSitesRun(t, log, socket); len(runs) != 0 {
		t.Errorf("a tab on %q ran %+v", socket, runs)
	}
}

// Any folder in the socket parent that isn't exactly a desk's id, and a link to a desk's folder or socket: nothing.
func TestADesksPlaceSpeltAnyOtherWayGetsNothing(t *testing.T) {
	program, log, sandboxed, _ := tmuxGateRig(t)
	parent := deskSocketParent()
	// The rig works: the desk's own spelling is driven, told its checked server.
	if runs := tabSitesRun(t, log, sandboxed.TmuxSocket); len(runs) == 0 || runs[0].guard != "5151" {
		t.Fatalf("the desk's own tab: %+v", runs)
	}
	elsewhere, err := os.MkdirTemp("", "dc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(elsewhere) })
	elsewhere = physical(elsewhere)
	folderLink, socketLink := filepath.Join(elsewhere, "lnk"), filepath.Join(elsewhere, "x")
	if os.Symlink(filepath.Dir(sandboxed.TmuxSocket), folderLink) != nil || os.Symlink(sandboxed.TmuxSocket, socketLink) != nil {
		t.Fatal("the links")
	}
	// Spellings the kernel takes to desk ab12's socket that cleaning the text doesn't (RT126DESKCASE B1, B2): relative,
	// from the core's current folder, where tmux is run; and .. after a link to another folder in the parent, which the
	// kernel walks back from the link's target, not from the link.
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(physical(here), sandboxed.TmuxSocket)
	if err != nil || filepath.IsAbs(relative) {
		t.Fatalf("relative: %q, %v", relative, err)
	}
	inParent := filepath.Join(elsewhere, "lnk2")
	if os.MkdirAll(filepath.Join(parent, "cd34"), 0o700) != nil || os.Symlink(filepath.Join(parent, "cd34"), inParent) != nil {
		t.Fatal("the link into the parent")
	}
	afterLink := inParent + "/../ab12/" + deskSocketFile
	// Both at once, from the current folder; and a link to the desk's socket reached after the .. (…/p/cd34/y).
	toLink, err := filepath.Rel(physical(here), inParent)
	if err != nil {
		t.Fatal(err)
	}
	relativeAfterLink := toLink + "/../ab12/" + deskSocketFile
	if os.Symlink(sandboxed.TmuxSocket, filepath.Join(parent, "cd34", "y")) != nil {
		t.Fatal("the socket link in the parent")
	}
	linkAfterLink := inParent + "/../cd34/y"
	ownInfo, err := os.Stat(sandboxed.TmuxSocket)
	if err != nil {
		t.Fatal(err)
	}
	for _, socket := range []string{relative, afterLink, relativeAfterLink, linkAfterLink} {
		if info, err := os.Stat(socket); err != nil || !os.SameFile(info, ownInfo) {
			t.Fatalf("%q isn't desk ab12's socket to the kernel (%v): the row checks nothing", socket, err)
		}
	}
	for _, socket := range []string{
		filepath.Join(parent, "AB12", deskSocketFile), // the reviewer's row: desk ab12's own socket on a Mac
		filepath.Join(parent, "x_y", deskSocketFile),  // a folder no desk's id can be
		filepath.Join(parent, "ef56", deskSocketFile), // an id no row has
		filepath.Join(folderLink, deskSocketFile),     // a link to a desk's folder
		socketLink, // a link to a desk's socket
		relative,   // B1: desk ab12's socket from the current folder
		afterLink,  // B2: …/lnk2/../ab12/s, lnk2 a link to …/p/cd34
		relativeAfterLink,
		linkAfterLink, // …/lnk2/../cd34/y, y a link to desk ab12's socket
	} {
		// Each a server's socket as anyone might make there (on a Mac, …/AB12/s is ab12's own already).
		if _, err := os.Stat(socket); err != nil {
			listenIn(t, socket)
		}
		if !deskShapedSocket(socket) {
			t.Errorf("%q isn't shaped as a desk's", socket)
		}
		noDeskGetsNothing(t, program, log, socket)
	}
	// A socket not there yet, after a link into the parent: tmux would start a server in a desk's place. Its folder is
	// walked as the kernel walks it.
	if os.MkdirAll(filepath.Join(parent, "gh78"), 0o700) != nil {
		t.Fatal("gh78")
	}
	if socket := inParent + "/../gh78/" + deskSocketFile; !deskShapedSocket(socket) {
		t.Errorf("%q isn't shaped as a desk's", socket)
	} else {
		noDeskGetsNothing(t, program, log, socket)
	}
	// With no current folder to read, a relative socket is taken for a desk's: nothing is run on it (fail closed).
	savedFolder := workingFolder
	t.Cleanup(func() { workingFolder = savedFolder })
	workingFolder = func() (string, error) { return "", os.ErrNotExist }
	for _, socket := range []string{relative, "default"} {
		if !deskShapedSocket(socket) {
			t.Errorf("with no current folder, %q isn't shaped as a desk's", socket)
		}
		noDeskGetsNothing(t, program, log, socket)
	}
	workingFolder = savedFolder
	// Still a tmux of your own: a socket beside the parent, the owner's default one, and that one spelt from the current
	// folder.
	ownDefault, err := filepath.Rel(physical(here), filepath.Join(elsewhere, "default"))
	if err != nil {
		t.Fatal(err)
	}
	for _, socket := range []string{filepath.Join(filepath.Dir(parent), "q", "ab12", deskSocketFile), filepath.Join(elsewhere, "default"), ownDefault} {
		if deskShapedSocket(socket) {
			t.Errorf("%q is shaped as a desk's", socket)
		}
		if _, guard := tmuxFor(socket); guard != tmuxUnchecked {
			t.Errorf("tmuxFor(%q) told %q, want %q", socket, guard, tmuxUnchecked)
		}
	}
}

// On a Mac, the volume folds names as Unicode does: the parent's folder or s in another case, s as ſ, and the
// firmlink's spelling of the parent are all desk ab12's own socket. Each gets nothing.
func TestOnAMacADesksSocketInAnyCaseGetsNothing(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("a Mac's volume: Linux's /tmp compares names as they are (TestOnLinuxADesksSocketIsMatchedExactly)")
	}
	program, log, sandboxed, _ := tmuxGateRig(t)
	parent, own := deskSocketParent(), sandboxed.TmuxSocket
	variants := []string{
		strings.ToUpper(parent) + "/ab12/s",
		filepath.Join(filepath.Dir(parent), strings.ToUpper(filepath.Base(parent)), "ab12", "s"),
		filepath.Join(parent, "aB12", "s"),
		filepath.Join(parent, "ab12", "S"),
		filepath.Join(parent, "ab12", "ſ"), // ſ, long s: s, folded
	}
	ownInfo, err := os.Stat(own)
	if err != nil {
		t.Fatal(err)
	}
	// The firmlink's spelling, when the kernel takes it to ab12's socket: judged by the disk itself, never by samePlace,
	// the function under test (RT126DESKCASE B3).
	if firm := "/System/Volumes/Data" + own; func() bool { info, err := os.Stat(firm); return err == nil && os.SameFile(info, ownInfo) }() {
		variants = append(variants, firm)
	} else {
		t.Logf("%q isn't ab12's socket here: no firmlink row", firm)
	}
	for _, socket := range variants {
		if info, err := os.Stat(socket); err != nil || !os.SameFile(info, ownInfo) {
			t.Logf("%q isn't ab12's socket on this volume (a case-sensitive one?): still shaped as a desk's", socket)
		}
		if deskSocketID(socket, 0) != "" || !deskShapedSocket(socket) {
			t.Errorf("%q: id %q, shaped %v; want no id, shaped", socket, deskSocketID(socket, 0), deskShapedSocket(socket))
		}
		noDeskGetsNothing(t, program, log, socket)
	}
}

// On Linux, /tmp compares names as they are: the parent's folder in another case is another folder, not a desk's
// place, and a server there is a tmux of your own, as any other. The Mac's case rows don't apply.
func TestOnLinuxADesksSocketIsMatchedExactly(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux's /tmp: a Mac's folds names (TestOnAMacADesksSocketInAnyCaseGetsNothing)")
	}
	_, _, sandboxed, _ := tmuxGateRig(t)
	parent := deskSocketParent()
	other := filepath.Join(filepath.Dir(parent), strings.ToUpper(filepath.Base(parent)), "ab12", deskSocketFile)
	listenIn(t, other)
	if samePlace(filepath.Dir(filepath.Dir(other)), parent) || deskShapedSocket(other) {
		t.Fatalf("%q is taken for the desk's place %q", other, parent)
	}
	if _, guard := tmuxFor(other); guard != tmuxUnchecked {
		t.Errorf("tmuxFor(%q) told %q, want %q", other, guard, tmuxUnchecked)
	}
	// The desk's own spelling is still its own.
	if deskSocketID(sandboxed.TmuxSocket, 0) != "ab12" || !deskShapedSocket(sandboxed.TmuxSocket) {
		t.Error("the desk's own socket")
	}
}
