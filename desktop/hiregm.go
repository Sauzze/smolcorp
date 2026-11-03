package desktop

import (
	"net/http"
	"slices"
)

// One General Manager per office (T602 part C3-gm). Setup's hire of the GM
// refuses while a GM is working, as the Mac's FrontDesk.gmRefusal: nothing is
// ever fired for it, the owner fires the old one first. That holds for every
// route that hires a GM.

// gmWorking is Setup's refusal while a GM is working (FrontDesk.gmRefusal).
// T722's two texts for the GM's token are secondGMRefusal and
// onlyOwnerHiresGMRefusal, chosen by gmHireRefusal (frontdeskmail.go, T726).
const gmWorking = "A General Manager is already working. Fire it first, then hire another."

// gmHired is a GM that is neither fired nor failed.
func gmHired(h Hire) bool { return h.Role == gmRole && isOpen(h) }

// gmWorkingRefusal is why Setup may not hire a GM now, or nil, as the Mac's
// gmRefusal: a GM hired and not fired or failed, that is still starting or
// whose session is alive (sessionAlive). One whose session is gone doesn't block.
func gmWorkingRefusal(hires []Hire, sessionAlive func(Hire) bool) error {
	for _, h := range hires {
		if gmHired(h) && (h.State == "starting" || sessionAlive(h)) {
			return &hireRefusal{Status: http.StatusConflict, Message: gmWorking}
		}
	}
	return nil
}

// officeHires are the registry's hires.
func officeHires() []Hire {
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	return readRegistry(registryPath(), false).hires
}

// gmRefusal is the answer to a hire of the gm role by whichever route, or nil
// to go on, as the Mac's claimGMHire: one General Manager at a time. Called
// with d.mu held; it is let go while tmux is asked, so what is read of the
// sessions is copied first.
func (d *Deck) gmRefusal() error {
	hires := officeHires()
	// A hire the launcher engine made counts as working while its launcher is connected.
	launched := map[string]bool{}
	for _, tab := range d.launchers {
		if tab.hireID != "" {
			launched[tab.hireID] = true
		} else if tab.office {
			// One that connected again lost its hire id: it counts for the hire whose worktree it works in.
			// (hiresWorkingIn reads only the registry given; officeHires took frontDeskMu before, never d.mu.)
			for _, h := range hiresWorkingIn(hires, tab.folder) {
				launched[h.ID] = true
			}
		}
	}
	var live []liveSession
	for _, s := range d.sessions.All() {
		live = append(live, liveSession{ID: s.ID, Folder: s.Folder, Project: s.Project})
	}
	options, testPort := d.engine, testCopyPort()
	d.mu.Unlock() // tmux is asked without the lock
	defer d.mu.Lock()
	return gmWorkingRefusal(hires, func(h Hire) bool {
		return slices.ContainsFunc(live, func(s liveSession) bool { return works(s, h) }) || launched[h.ID] || hireSessionUp(h, testPort, options)
	})
}
