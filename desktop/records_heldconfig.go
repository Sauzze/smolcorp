package desktop

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
)

// While the records' key can't be read (recordsHeldOnly) the config is held in memory alone: config.save() writes
// nothing, so the restart after the owner unlocks the keychain brings back what the file holds. A setting changed
// meanwhile would seem to apply, then come back (RT126SK1F B1). So every change to a setting is refused then, with
// Setup's reason, unless it's one that can't take anything away:
//   - a plain setting (heldPlainSettings): how the office looks and sounds, where hires start, and the like;
//   - a switch turned on (heldSwitches) or a list added to (heldLists): it lasts only until the restart, which takes it
//     back to what's saved, the safe way;
//   - or one that leaves the setting as the file holds it, so nothing comes back (a switch turned on in this run, then
//     off again).
//
// Anything else is refused: a setting this list doesn't name, a new one included, so none can be missed the way
// clean-up and the viewer's options were.

// heldPlainSettings are the settings, by their name in config.json, that bear on nothing that leaves the Mac or on
// who may reach it: any change to them is taken while the key can't be read, and lasts until the restart.
var heldPlainSettings = map[string]bool{
	"pins": true, "colours": true, "layout": true, "profiles": true, "activeProfile": true, "logo": true,
	"musicEnabled": true, "maxConfirm": true, "floors": true, "floorRules": true, "floorPins": true, "botFloor": true,
	"window": true, "alwaysOnTop": true, "officeKey": true, "teamScreen": true, "showInDock": true,
	"guideDone": true, "hasCompletedSetup": true, "choicesAsked": true, "rolesOffered": true, "dictationLanguage": true,
	"hireFolder": true, "hireAgent": true, "recentHireFolders": true, "hireTerminal": true, "hireInTmux": true,
	"hireManagedWindows": true, "hiresInBackground": true,
}

// heldSwitches are the settings that are a switch whose "on" lets more happen: switched on while the key can't be
// read, it's off again at the restart, as saved; switched off, it's refused unless the file has it off too.
var heldSwitches = map[string]bool{
	"scrollEnabled": true, "hiringEnabled": true, "managerEnabled": true, "summaryOptIn": true,
	"keystrokesAllowed": true, "dictationEnabled": true, "dictationForClaude": true, "dictationCleanup": true,
	"phoneMicEnabled": true, "phoneMicSkipsConfirm": true, "phoneTextOverWiFi": true, "floorsFromOtherMacs": true,
	"mailInboxSocket": true, "builtinWatches": true, "gmByHand": true, "acpRegistry": true, "t3CodeEnabled": true,
}

// heldLists are the settings that are a list whose every item lets more happen (an app keys may go to, a bot and its
// mailbox, an account, a role, a name dictation sends): added to while the key can't be read, it's as saved again at
// the restart; an item the file holds taken off or changed is refused.
var heldLists = map[string]bool{
	"keystrokeApps": true, "cloudBots": true, "claudeProfiles": true, "roles": true, "dictationGlossary": true,
}

// heldListParts are, for a list whose items are only partly about what leaves the Mac, the parts of an item that
// count: a bot's id, where it's found and its mailbox; its name, outfit, floor and pin are the office's look.
var heldListParts = map[string][]string{"cloudBots": {"id", "source", "mailbox"}}

// heldRefusal is what change, made to the config c, is refused with while the records' key can't be read: "" when it
// may go ahead (any time the key can be read). change is made to copies, never to c: one of c, and one of what the
// file holds (savedConfig), since the core holds the safe defaults for the security settings then, so a switch
// turned off may be off in memory already, and on in the file. Each setting it moves in either is checked: what c
// would hold then against what the file holds. The caller holds whatever guards c (d.mu).
func (c *Config) heldRefusal(change func(*Config)) string {
	if !recordsHeldOnly() {
		return ""
	}
	file := savedConfig()
	before, after := c.heldCopy(), c.heldCopy()
	fileBefore, fileAfter := file.heldCopy(), file.heldCopy()
	change(after)
	change(fileAfter)
	saved := configFields(fileBefore)
	for _, pair := range [][2]map[string]json.RawMessage{{configFields(before), configFields(after)}, {saved, configFields(fileAfter)}} {
		was, now := pair[0], pair[1]
		for _, fields := range pair {
			for name := range maps.Keys(fields) {
				if !bytes.Equal(was[name], now[name]) && !heldChangeTakes(name, saved[name], was[name], now[name]) {
					return recordsHeldAnswer
				}
			}
		}
	}
	return ""
}

// refusedConfigWhileHeld answers change, a change to the deck's config a route is about to make, when it's refused
// while the records' key can't be read (heldRefusal), and says whether it did. The caller doesn't hold d.mu.
func (d *Deck) refusedConfigWhileHeld(w http.ResponseWriter, change func(*Config)) bool {
	d.mu.Lock()
	refused := d.config.heldRefusal(change)
	d.mu.Unlock()
	if refused == "" {
		return false
	}
	writeStatus(w)(http.StatusConflict, refused)
	return true
}

// heldChangeTakes is whether the setting called name may go from was to now (its JSON, nil when left out) while the
// records' key can't be read, the file holding saved. A list may lose an item the file doesn't hold (one added in this
// run), never one it holds: the core may hold less than the file then (a security setting's safe default).
func heldChangeTakes(name string, saved, was, now json.RawMessage) bool {
	switch {
	case heldPlainSettings[name], bytes.Equal(saved, now):
		return true
	case heldSwitches[name]:
		return string(now) == "true"
	case heldLists[name]:
		var had, held, has []json.RawMessage
		if json.Unmarshal(now, &has) != nil && now != nil {
			return false
		}
		_ = json.Unmarshal(saved, &had)
		_ = json.Unmarshal(was, &held)
		in := func(list []json.RawMessage, item json.RawMessage) bool {
			for _, other := range list {
				if jsonSame(heldListItem(name, item), heldListItem(name, other)) {
					return true
				}
			}
			return false
		}
		for _, item := range had {
			if in(held, item) && !in(has, item) {
				return false
			}
		}
		return true
	}
	return false
}

// heldCopy is a copy of c that shares nothing with it, through its JSON: what heldRefusal changes and compares.
func (c *Config) heldCopy() *Config {
	data, _ := json.Marshal(c)
	copied := &Config{}
	_ = json.Unmarshal(data, copied)
	return copied
}

// configFields are c's settings by their name in config.json, each as compact JSON.
func configFields(c *Config) map[string]json.RawMessage {
	data, _ := json.Marshal(c)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	for name, value := range fields {
		var compact bytes.Buffer
		if json.Compact(&compact, value) == nil {
			fields[name] = compact.Bytes()
		}
	}
	return fields
}

// heldListItem is the part of item, in the list called name, that heldChangeTakes compares (heldListParts).
func heldListItem(name string, item json.RawMessage) json.RawMessage {
	parts, ok := heldListParts[name]
	var fields map[string]json.RawMessage
	if !ok || json.Unmarshal(item, &fields) != nil {
		return item
	}
	counted := map[string]json.RawMessage{}
	for _, part := range parts {
		counted[part] = fields[part]
	}
	data, _ := json.Marshal(counted)
	return data
}

func jsonSame(a, b json.RawMessage) bool {
	var x, y bytes.Buffer
	return json.Compact(&x, a) == nil && json.Compact(&y, b) == nil && bytes.Equal(x.Bytes(), y.Bytes())
}
