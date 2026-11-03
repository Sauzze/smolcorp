//go:build windows

package desktop

// The tmux engine is Linux's. On Windows a role's Claude hire is made by the
// launcher's engine (hirelauncher.go), behind the beta switch: hireAs asks for
// it by name, so managedHire here only says the tmux way isn't made.
func (d *Deck) managedHire(roleID, folder string, terminal hireOpener, hasTerminal bool, held, floor int) (int, string, bool) {
	return 0, "", false
}

type engineOptions struct{}

// hireSessionUp: no tmux session of an engine hire's here. A launcher hire's window
// counts through gmRefusal, which asks the launchers.
func hireSessionUp(h Hire, testPort int, o engineOptions) bool { return false }

// fireTab: a launcher tab of an engine hire is fired through its launcher
// (fireLauncherTab); any other tab is closed the old way.
func (d *Deck) fireTab(tab string, force bool) (int, string, bool) {
	return d.fireLauncherTab(tab, force)
}
