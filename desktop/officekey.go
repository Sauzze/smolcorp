package desktop

import (
	"slices"
	"strings"
)

// The keys that show and hide the office's window from anywhere, as the Mac app holds them (OfficeKey.swift):
// ⌥⌘O unless chosen otherwise in setup, or none. Kept in config.json as the setup page records them,
// "alt+cmd+O": the modifiers in the Mac's order (⌃⌥⇧⌘), then the key, named by where it sits on a US
// keyboard; "off" for none, and no entry for the default. Under the Mac app, the app holds them, told through
// the bridge (office-key), and its facts say the keys it holds now and each key's name on the layout in use
// (labels: "Q" is "A" on a French AZERTY Mac), so setup shows the keys as printed and refuses the Mac's own by
// what they type there. Windows and Linux have no key from anywhere to open the office, so the setting is only
// kept there, for the Mac, and named by its US names. Both engines' tests read Tests/fixtures/officekey.json.

const (
	officeKeyDefault = "alt+cmd+O"
	officeKeyOff     = "off"
)

var officeKeyModifiers = []string{"ctrl", "alt", "shift", "cmd"}

// officeKeyNames are the keys the Mac app can hold (OfficeKey.codes).
var officeKeyNames = func() map[string]bool {
	names := map[string]bool{"Space": true, "Tab": true}
	for _, c := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789,./;'[]-=\\`" {
		names[string(c)] = true
	}
	for _, f := range []string{"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12"} {
		names[f] = true
	}
	return names
}()

// officeKeyReserved are the Mac's own keys in every app, never taken from it, by what they type: the system's,
// then the editing and file keys every app has, then Log Out and Lock Screen.
var officeKeyReserved = []string{"cmd+Tab", "cmd+Space", "cmd+Q", "cmd+W", "cmd+H", "cmd+M", "cmd+,",
	"cmd+C", "cmd+V", "cmd+X", "cmd+Z", "shift+cmd+Z", "cmd+A", "cmd+S", "cmd+F", "cmd+T", "cmd+N",
	"cmd+P", "cmd+O", "shift+cmd+Q", "ctrl+cmd+Q"}

const (
	officeKeyShapeRefusal    = "That isn't a shortcut Smolcorp can use: a letter, a digit, F1 to F12, Space, Tab or a punctuation key, with ⌘, ⌥, ⌃ or ⇧."
	officeKeyModifierRefusal = "Hold ⌘, ⌥ or ⌃ too: a key alone, or with ⇧ only, would stop it typing anywhere."
)

// parseOfficeKey reads keys as written, the modifiers in any order, and gives them in the Mac's order; ok is
// false for anything else.
func parseOfficeKey(text string) (keys string, ok bool) {
	parts := strings.Split(text, "+")
	key := parts[len(parts)-1]
	mods := parts[:len(parts)-1]
	if !officeKeyNames[key] {
		return "", false
	}
	held := map[string]bool{}
	for _, m := range mods {
		if !slices.Contains(officeKeyModifiers, m) || held[m] {
			return "", false
		}
		held[m] = true
	}
	var ordered []string
	for _, m := range officeKeyModifiers {
		if held[m] {
			ordered = append(ordered, m)
		}
	}
	return strings.Join(append(ordered, key), "+"), true
}

// officeKeySymbols is keys as the Mac's menus show them on the layout labels name: ⌥⌘O.
func officeKeySymbols(keys string, labels map[string]string) string {
	marks := map[string]string{"ctrl": "⌃", "alt": "⌥", "shift": "⇧", "cmd": "⌘"}
	parts := strings.Split(keys, "+")
	var out strings.Builder
	for _, m := range parts[:len(parts)-1] {
		out.WriteString(marks[m])
	}
	key := parts[len(parts)-1]
	if label, ok := labels[key]; ok {
		key = label
	}
	out.WriteString(key)
	return out.String()
}

// officeKeyTyped is keys as apps read them on the layout labels name, as macOS matches its menus' keys: the key
// by what it types there when that's a key the Mac app knows by name, else by where it sits (OfficeKey.typed).
func officeKeyTyped(keys string, labels map[string]string) string {
	parts := strings.Split(keys, "+")
	if label, ok := labels[parts[len(parts)-1]]; ok && officeKeyNames[label] {
		parts[len(parts)-1] = label
	}
	return strings.Join(parts, "+")
}

// officeKeyChoice is what the setup page asks for: "default" for ⌥⌘O, "off" for none, or keys, checked on
// the layout labels name (none: the US names), as the Mac app checks them (OfficeKey.choice); refusal says
// why not.
func officeKeyChoice(text string, labels map[string]string) (keys, refusal string) {
	switch text {
	case "default":
		return officeKeyDefault, ""
	case officeKeyOff:
		return officeKeyOff, ""
	}
	keys, ok := parseOfficeKey(text)
	if !ok {
		return "", officeKeyShapeRefusal
	}
	mods := strings.Split(keys, "+")
	mods = mods[:len(mods)-1]
	if !slices.ContainsFunc(mods, func(m string) bool { return m != "shift" }) {
		return "", officeKeyModifierRefusal
	}
	if slices.Contains(officeKeyReserved, officeKeyTyped(keys, labels)) {
		return "", officeKeySymbols(keys, labels) + " is the Mac's own, in every app: choose another."
	}
	return keys, ""
}

// officeKeyChosen is the keys a config holds: the default when it has none, "off" when they're off. A
// hand-edited entry that isn't fit to hold counts as none set. Checked on the layout labels name, as the page's
// ask was (OfficeKey.chosen).
func officeKeyChosen(saved string, labels map[string]string) string {
	if saved == "" {
		return officeKeyDefault
	}
	if keys, refusal := officeKeyChoice(saved, labels); refusal == "" {
		return keys
	}
	return officeKeyDefault
}

// officeKeySaved is what config.json keeps for keys: no entry for the default.
func officeKeySaved(keys string) string {
	if keys == officeKeyDefault {
		return ""
	}
	return keys
}

// macOfficeKey is the Mac app's word on the office's keys, in setup's facts: the keys it holds now ("off" for
// none) and each key's name on the layout in use, where it differs from the US one.
type macOfficeKey struct {
	Held   string            `json:"held"`
	Labels map[string]string `json:"labels"`
}

// labels is the layout's names for the keys: none (the US names) without the Mac app's word.
func (m *macOfficeKey) labels() map[string]string {
	if m == nil || m.Labels == nil {
		return map[string]string{}
	}
	return m.Labels
}

// officeKeyState is setup's officeKey, as the Mac app's: the keys kept ("off" for none), the default, whether
// the keys kept are the ones held now, and the layout's names for the keys. Without the Mac app's word nothing
// holds them (Windows, Linux) and they're named by their US names. A test copy of the Mac app holds none
// (testCopy, from the app's facts), whatever it says it holds: the page says so in a line of its own.
func officeKeyState(saved string, mac *macOfficeKey, testCopy bool) map[string]any {
	labels := mac.labels()
	keys := officeKeyChosen(saved, labels)
	return map[string]any{"keys": keys, "default": officeKeyDefault, "held": mac != nil && !testCopy && mac.Held == keys, "labels": labels}
}
