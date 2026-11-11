package desktop

import (
	"slices"
	"strings"
)

// The office's desks under the Mac app (engine=go): what each desk's terminal
// lets the office do, as the Mac app's driver for it says (Terminals.abilities,
// from the drivers' own tables: no terminal is asked), and dictation as the
// office's mic has it. The app is asked once for the sessions' tabs, on its
// own, and again only when they change: never per desk, never per tick. The
// office is told again when it answers. Elsewhere none of this is in the
// snapshot: every terminal there is this program's to drive.

// askAbilities asks the Mac app, once, for the abilities of the sessions'
// tabs when they aren't the tabs it was last asked about. With the lock held.
func (d *Deck) askAbilities() {
	h := host
	if h == nil || d.mac.learning {
		return
	}
	var tabs []string
	for _, s := range d.sessions.All() {
		// A launcher's tab is this program's own, and a headless run has nothing to drive.
		if kind, _, _ := splitTab(s.Tab); s.Tab != "" && kind != "launcher" && kind != "headless" && !slices.Contains(tabs, s.Tab) {
			tabs = append(tabs, s.Tab)
		}
	}
	slices.Sort(tabs)
	tabs = tabs[:min(len(tabs), bridgeTabsMost)]
	key := strings.Join(tabs, "\n")
	if key == d.mac.abilitiesFor {
		return
	}
	d.mac.learning = true
	go d.learnAbilities(h, tabs, key)
}

// learnAbilities asks h for tabs' abilities, keeps them, and tells the office.
// Not answered, those it had are kept, and these tabs aren't asked about again
// until they change.
func (d *Deck) learnAbilities(h hostBridge, tabs []string, key string) {
	found := map[string]tabAbilities{}
	var err error
	if len(tabs) > 0 {
		var said []tabAbilities
		if said, err = hostAbilities(h, tabs); err == nil {
			for i, tab := range tabs {
				found[tab] = said[i]
			}
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mac.learning = false
	d.mac.abilitiesFor = key
	if err == nil {
		d.mac.abilities = found
	}
	d.changed()
}

// addMacControls puts a desk's controls in its entry under the Mac app, as the
// Swift engine's sessionEntry does: keys and fire as its terminal allows them,
// nothing until the app has said. With the lock held.
func (d *Deck) addMacControls(entry map[string]any, s *Session) {
	if host == nil {
		return
	}
	can := d.mac.abilities[s.Tab]
	// A launcher's window takes keys, lines and the stop from this program itself.
	if kind, _, _ := splitTab(s.Tab); kind == "launcher" {
		can = tabAbilities{Keys: true, Text: true, Fire: true}
	}
	entry["keys"], entry["fire"] = can.Keys, can.Fire
	// The office's mic dictates only in the Swift engine for now: nothing asks the app to listen yet.
	entry["dictates"] = false
	// What a menu asks you to choose is told to the iPhone app (personal build) alone: nothing here.
	entry["choice"] = nil
}

// addMacOffice puts dictation in the snapshot under the Mac app, as the Swift
// engine has it for the mic's quick switches: off, as above, in its language,
// with the clean-up as set. With the lock held.
func (d *Deck) addMacOffice(state map[string]any) {
	if host == nil {
		return
	}
	state["dictation"] = map[string]any{"on": false, "language": d.dictationLanguage(), "cleanup": d.config.DictationCleanup}
}
