package desktop

import (
	"errors"
	"path/filepath"
	"slices"
)

// Each desk's own hook key (design A's X10, SANDBOX-PLAN AQ3; PL126F's "per-hire keys"). The hook key in the support
// folder (hook-key, hook-header) used to be every session's: any desk that read it could report as any other, end
// another's session, or move it to a tab of its own. Now each desk Smolcorp starts is given a key of its own at each
// start, through its start ticket (deskstart.go) or, for a launcher's window, its start environment (hirestartenv.go):
// never on disk, never on a command line. The registry keeps only its hash, sealed with the send keys (§3.2), so the
// desk's events still count after the core restarts. The hook sends it in place of the file's key (hook.go).
//
// The core then checks each event against the desk it comes from. A desk is known by its tab, its own tmux socket
// (or its launcher), which no other desk shares. An event with a desk's key may only land on that desk's tab, or a
// headless run it started, and only touch a session already there. An event with the support folder's key, which
// stays for the owner's own sessions, may touch no desk that has a key. And a desk with a key claims only the
// sessions on its own tab, never another's by folder (buildTeam). Desks started before the update have no key, and
// go on as before until their next start.

// hookKeyEnvironment is where a desk's hook finds its own key: with it set, the hook never reads the file.
const hookKeyEnvironment = "SMOLCORP_HOOK_KEY"

// hookNotYourDesk is the answer to an event that names a desk other than the one whose key it carries.
const hookNotYourDesk = "This event names a desk its key isn't for, so it was dropped."

// newHookKey makes hire hireID's hook key for this start (which must be in the registry already), keeps its hash and
// returns the key. A desk starting again gets a new key; the old one stops working.
func newHookKey(path, hireID string) (string, error) {
	frontDeskMu.Lock()
	defer frontDeskMu.Unlock()
	file := readRegistry(path, true)
	if !slices.ContainsFunc(file.hires, func(h Hire) bool { return h.ID == hireID }) {
		return "", errors.New("no such hire")
	}
	key := randomKey()
	if file.hookKeys == nil {
		file.hookKeys = map[string]string{}
	}
	file.hookKeys[hireID] = hashKey(key)
	if err := file.save(path); err != nil {
		return "", errors.New("couldn't save the key")
	}
	return key, nil
}

// hookDesks is what the core checks hook events by: the open desks that report with keys of their own, as the
// registry holds them, and where each one's tab is.
type hookDesks struct {
	hashes  map[string]string // desk id → its hook key's hash
	sockets map[string]string // a desk's tmux socket, as socketPlace spells it → desk id
	// launchers is each launcher's hire, by launcher id (Deck.launchers), set by whoever holds the deck.
	launchers map[string]string
}

// currentHookDesks is hookDesks as the registry holds it now.
func currentHookDesks() hookDesks {
	file := readRegistry(registryPath(), false)
	desks := hookDesks{hashes: map[string]string{}, sockets: map[string]string{}}
	for _, h := range file.hires {
		hash, ok := file.hookKeys[h.ID]
		if !ok || !isOpen(h) {
			continue
		}
		desks.hashes[h.ID] = hash
		if place := socketPlace(h.TmuxSocket); place != "" {
			desks.sockets[place] = h.ID
		}
	}
	return desks
}

// socketPlace is a socket's path spelled the one way, whatever links its folder's path goes through (engineHireAt's
// spelling), or "" for one that isn't a full path. Only the folder's links are resolved, as engineHireAt does: a tab
// naming a desk's socket through a link of its own, or in another case, is no desk's here, and pairs with no desk
// there either (deskOfTab and frontDeskLive both spell it this way), so the check and the pairing never disagree.
func socketPlace(socket string) string {
	if socket == "" || !filepath.IsAbs(socket) {
		return ""
	}
	clean := filepath.Clean(socket)
	return filepath.Join(physical(filepath.Dir(clean)), filepath.Base(clean))
}

// deskForKey is the open desk key was made for, or "". Every desk is compared, so how long it takes doesn't say
// where a key was close.
func (k hookDesks) deskForKey(key string) string {
	if key == "" {
		return ""
	}
	given, desk := hashKey(key), ""
	for id, hash := range k.hashes {
		if constantTimeEqual(hash, given) {
			desk = id
		}
	}
	return desk
}

// deskOfTab is the desk with a key whose tab tab is, or "".
func (k hookDesks) deskOfTab(tab string) string {
	kind, id, socket := splitTab(tab)
	switch kind {
	case "tmux":
		if place := socketPlace(socket); place != "" {
			return k.sockets[place]
		}
	case "launcher":
		if hire := k.launchers[id]; hire != "" && k.hashes[hire] != "" {
			return hire
		}
	}
	return ""
}

// allows is whether an event with desk's key ("" for the support folder's) may land on tab, its session being
// session (nil for a new one; one dropped a while ago and about to come back counts as it was). A desk reports from its own tab, or a headless run it started, and an event that
// names no tab only goes on with a session already its own. The owner's key reaches no desk with a key.
func (k hookDesks) allows(desk, tab string, session *Session) bool {
	mine := func(t string) bool {
		return !tabThroughLink(desk, t) && (k.deskOfTab(t) == desk || desk != "" && isHeadless(t))
	}
	if session != nil && !mine(session.Tab) {
		return false
	}
	if tab == "" {
		return desk == "" || session != nil
	}
	return mine(tab)
}

// tabThroughLink is whether tab is refused for going through a link (FXTABLINK, tablink.go), for an event with desk's
// key ("" for the owner's sessions'): a desk's tmux tab with a link anywhere on its socket's path, but for the core's
// own spelling of a hire's socket (coreSpelt); anyone's whose link leads to a desk's socket. The owner's other tabs
// are taken as before: a socket tmux was given through a link of the system's (/tmp on a Mac) is still the owner's.
func tabThroughLink(desk, tab string) bool {
	kind, _, socket := splitTab(tab)
	if kind != "tmux" || !filepath.IsAbs(socket) || !socketThroughLink(socket) {
		return false
	}
	if desk != "" {
		return socketThroughOthersLink(socket, readRegistry(registryPath(), false).hires)
	}
	return linkedDeskSocket(socket)
}

// resolvedTab is tab as the core checks, keeps and drives it for an event with desk's key: a desk's tmux tab with its
// socket's path resolved, once (FXTABLINK), or, when it is a hire's socket as the core spelt it (coreSpelt), that
// spelling, which the core's own lists match by (hiredRoleAt). What is kept is never a spelling a desk chose, nor one
// a link could later turn elsewhere. The owner's key's tabs are kept as they come, as tmux spells them.
func resolvedTab(desk, tab string) string {
	kind, id, socket := splitTab(tab)
	if desk == "" || kind != "tmux" || !filepath.IsAbs(socket) {
		return tab
	}
	if coreSpelt(socket, readRegistry(registryPath(), false).hires) {
		return kind + ":" + id + "@" + filepath.Clean(socket)
	}
	return kind + ":" + id + "@" + socketPlace(socket)
}

// deskWorks is works for pairing a live session with hire h, keyed being the registry's hook keys: a desk with a key
// works only on its own tab, whatever folder another session names, and only that desk works there.
func deskWorks(l liveSession, h Hire, keyed map[string]string) bool {
	if _, own := keyed[h.ID]; own || l.KeyedDesk != "" {
		return l.KeyedDesk == h.ID
	}
	return works(l, h)
}

// launcherHires is each launcher's hire by launcher id, for hookDesks. With the lock held.
func (d *Deck) launcherHires() map[string]string {
	hires := map[string]string{}
	for id, tab := range d.launchers {
		if tab.hireID != "" {
			hires[id] = tab.hireID
		}
	}
	return hires
}
