package desktop

import (
	"slices"
	"strings"
)

// The keys that show and hide the office's window from anywhere, as the Mac app holds them (OfficeKey.swift):
// ⌥⌘O unless chosen otherwise in setup, or none. Kept in config.json as the setup page records them,
// "alt+cmd+O": the modifiers in the Mac's order (⌃⌥⇧⌘), then the key, named by where it sits on a US
// keyboard; "off" for none, and no entry for the default. Under the Mac app, the app holds them, told through
// the bridge (office-key); Windows and Linux have no key from anywhere to open the office, so the setting is
// only kept there, for the Mac. Both engines' tests read Tests/fixtures/officekey.json.

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

// officeKeyReserved are the Mac's own keys in every app, never taken from it.
var officeKeyReserved = []string{"cmd+Tab", "cmd+Space", "cmd+Q", "cmd+W", "cmd+H", "cmd+M", "cmd+,"}

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

// officeKeySymbols is keys as the Mac's menus show them: ⌥⌘O.
func officeKeySymbols(keys string) string {
	marks := map[string]string{"ctrl": "⌃", "alt": "⌥", "shift": "⇧", "cmd": "⌘"}
	parts := strings.Split(keys, "+")
	var out strings.Builder
	for _, m := range parts[:len(parts)-1] {
		out.WriteString(marks[m])
	}
	out.WriteString(parts[len(parts)-1])
	return out.String()
}

// officeKeyChoice is what the setup page asks for: "default" for ⌥⌘O, "off" for none, or keys, checked,
// as the Mac app checks them (OfficeKey.choice); refusal says why not.
func officeKeyChoice(text string) (keys, refusal string) {
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
	if slices.Contains(officeKeyReserved, keys) {
		return "", officeKeySymbols(keys) + " is the Mac's own, in every app: choose another."
	}
	return keys, ""
}

// officeKeyChosen is the keys a config holds: the default when it has none, "off" when they're off. A
// hand-edited entry that isn't fit to hold counts as none set.
func officeKeyChosen(saved string) string {
	if saved == "" {
		return officeKeyDefault
	}
	if keys, refusal := officeKeyChoice(saved); refusal == "" {
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

// officeKeyState is setup's officeKey, as the Mac app's: the keys held ("off" for none) and the default.
func officeKeyState(saved string) map[string]any {
	return map[string]any{"keys": officeKeyChosen(saved), "default": officeKeyDefault}
}
