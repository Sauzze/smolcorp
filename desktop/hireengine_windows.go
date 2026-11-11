//go:build windows

package desktop

// The tmux engine is Linux's. On Windows a role's Claude hire is made by the
// launcher's engine (hirelauncher.go), behind the beta switch: hireAs asks for
// it by name, so managedHire here only says the tmux way isn't made.
func (d *Deck) managedHire(roleID, folder string, terminal hireOpener, hasTerminal bool, held, floor int) (int, string, bool) {
	return 0, "", false
}

func (d *Deck) managedHireWith(roleID, folder string, terminal hireOpener, hasTerminal bool, held, floor int, extra *setupHire) (int, string, bool) {
	return 0, "", false
}

type engineOptions struct{}

// worktrees is the folder hires' worktrees are made in: beside the repo here, as there's no engine to set another.
func (engineOptions) worktrees() string { return "" }

// hireSessionUp: no tmux session of an engine hire's here. A launcher hire's window
// counts through gmRefusal, which asks the launchers.
func hireSessionUp(h Hire, testPort int, o engineOptions) bool { return false }

// fireTab: a launcher tab of an engine hire is fired through its launcher
// (fireLauncherTab); any other tab is closed the old way.
func (d *Deck) fireTab(tab string, force bool) (int, string, bool) {
	return d.fireLauncherTab(tab, force)
}

// detachHireViewers: no hire runs on a tmux server of its own here.
func detachHireViewers(hire Hire, run func([]string) (string, int, bool)) detached { return detached{} }

// openHireViewer: no hire runs in tmux here.
func openHireViewer(hire Hire, terminal hireOpener) error { return errUnsupported }

// bridgeCloser and hiresToBackgroundClosing are the Mac app's (hirewindows_close.go): there is no host here.
type bridgeCloser struct{ h hostBridge }

func hiresToBackgroundClosing(hires []Hire, closer bridgeCloser, run func([]string) (string, int, bool)) []string {
	return nil
}

// For the GM's fire route (gmfire.go): a hire here is a launcher's window, never a tmux session.
func (d *Deck) fireTmuxHire(h Hire, force, keep bool) (kept, note string, err error) {
	return "", "", &hireRefusal{Status: 409, Message: "Couldn't reach " + calledHire(h) + "'s window, so nothing was changed."}
}

func ownTmuxTab(Hire, string) bool { return false }

// ownsHireSocket: no hire runs on a tmux server of its own here, so no socket is one of ours.
func ownsHireSocket(socket string, testPort int) bool { return false }
