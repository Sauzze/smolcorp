package desktop

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"
)

// A shortcut in your Startup folder that opens Smolcorp, as an older build or
// you may have made, opens it at sign-in beside the Run entry "Open at login"
// writes (autostart_windows.go): Smolcorp twice. Only a shortcut whose target
// is Smolcorp's app counts, read from the shortcut itself (the .lnk format,
// [MS-SHLLINK]), never by the shortcut's own name: another app's shortcut, or
// one to smolcorp-cli.exe, is left alone, whatever it is called. Here, with no
// build tag, so the tests read shortcuts everywhere.

// smolcorpShortcuts are the shortcuts in dir whose target is Smolcorp's app.
func smolcorpShortcuts(dir string) []string {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".lnk") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if info, err := entry.Info(); err != nil || info.Size() > 1<<20 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if target, err := shortcutTarget(data); err == nil && isSmolcorpTarget(target) {
			found = append(found, path)
		}
	}
	return found
}

// isSmolcorpTarget is whether a shortcut's target is Smolcorp's app, by its
// file's name: Smolcorp.exe, or the app's name before. A shortcut to
// smolcorp-cli.exe isn't: it's yours, smolcorp serve at sign-in, say, and
// stays, at install and when "Open at login" is set alike. Nor is a short
// name (SMOLCO~1.EXE), which may be either program.
func isSmolcorpTarget(target string) bool {
	name := strings.ToLower(target[strings.LastIndexAny(target, `/\`)+1:])
	return slices.Contains([]string{"smolcorp.exe", "sauzzedeck.exe", "sauzzedeck pro.exe"}, name)
}

var errNoShortcutTarget = errors.New("not a shortcut with a local target")

// shortcutTarget is the local path a .lnk opens: its LinkInfo's base path, in
// Unicode where the shortcut has it, and the path's suffix after it.
func shortcutTarget(data []byte) (string, error) {
	const headerSize, hasIDList, hasLinkInfo, volumeAndLocalPath = 0x4C, 0x1, 0x2, 0x1
	if len(data) < headerSize || binary.LittleEndian.Uint32(data) != headerSize {
		return "", errNoShortcutTarget
	}
	flags := binary.LittleEndian.Uint32(data[20:])
	at := headerSize
	if flags&hasIDList != 0 {
		if len(data) < at+2 {
			return "", errNoShortcutTarget
		}
		at += 2 + int(binary.LittleEndian.Uint16(data[at:]))
	}
	if flags&hasLinkInfo == 0 || len(data) < at+28 {
		return "", errNoShortcutTarget
	}
	info := data[at:]
	size := int(binary.LittleEndian.Uint32(info))
	if size < 28 || size > len(info) {
		return "", errNoShortcutTarget
	}
	info = info[:size]
	field := func(offset int) int { return int(binary.LittleEndian.Uint32(info[offset:])) }
	if field(8)&volumeAndLocalPath == 0 {
		return "", errNoShortcutTarget
	}
	if field(4) >= 0x24 && size >= 0x24 {
		base, okBase := unicodeAt(info, field(28))
		suffix, okSuffix := unicodeAt(info, field(32))
		if okBase && okSuffix && base != "" {
			return base + suffix, nil
		}
	}
	base, okBase := ansiAt(info, field(16))
	suffix, okSuffix := ansiAt(info, field(24))
	if !okBase || !okSuffix || base == "" {
		return "", errNoShortcutTarget
	}
	return base + suffix, nil
}

// ansiAt is the NUL-ended string at offset in data, read as bytes: enough to
// tell Smolcorp's programs, whose names are ASCII.
func ansiAt(data []byte, offset int) (string, bool) {
	if offset <= 0 || offset >= len(data) {
		return "", offset == 0
	}
	end := offset
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[offset:end]), end < len(data)
}

// unicodeAt is the NUL-ended UTF-16 string at offset in data.
func unicodeAt(data []byte, offset int) (string, bool) {
	if offset <= 0 || offset >= len(data) {
		return "", offset == 0
	}
	var units []uint16
	for at := offset; at+1 < len(data); at += 2 {
		unit := binary.LittleEndian.Uint16(data[at:])
		if unit == 0 {
			return string(utf16.Decode(units)), true
		}
		units = append(units, unit)
	}
	return "", false
}

// removeShortcuts takes the shortcuts away, quietly: one that stays only
// opens Smolcorp a second time, which a second copy hands over from.
func removeShortcuts(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
