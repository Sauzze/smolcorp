package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// A desk's own Claude Code settings file (SETTINGSFILE1). The core writes each desk's settings to a file in its own
// folder, <support>/hires/<id>/settings.json, which a desk may read and never write (the sandbox's Policy: "its brief
// and its fence"; the settings' own Edit rules), and starts its Claude Code with --settings <that file>: never the
// settings themselves in the command line, which Windows holds to 32,767 characters, and never a fallback to them.
//
// The file is sealed by its own hash. The core writes it with SettingsSumBlank where its hash goes (in the guard's
// own words, --sum, before and after a call), then puts SettingsSum of that text there. So the file names the hash it
// was written with, and the guard, started by Claude Code with those words from the settings it read at its start,
// checks the file on both sides of every tool call (CheckSettings): a file changed since, replaced, linked or given a
// second name stops the desk. The core checks it too, just before each start (the desk's ticket, the launcher's route). A changed file
// can't match: its text would have to hash to the sum the running desk already holds.

// SettingsSumBlank stands where a settings file's hash goes while the hash is made.
var SettingsSumBlank = strings.Repeat("0", sha256.Size*2)

// settingsMost is the most a desk's settings file may hold: far more than every rule a desk gets.
const settingsMost = 4 << 20

var errSettingsTooLarge = errors.New("it is larger than 4 MiB")

// SettingsSum is the hash of text, a settings file written with SettingsSumBlank wherever its hash goes, and whether
// it holds the blank at all: the file is that text with each blank replaced by its hash.
func SettingsSum(text []byte) (string, bool) {
	if !strings.Contains(string(text), SettingsSumBlank) {
		return "", false
	}
	sum := sha256.Sum256(text)
	return hex.EncodeToString(sum[:]), true
}

// CheckSettings is why the file at path isn't the settings file sealed with sum, nil when it is. It is read as the
// guard file is (ReadFile): the name itself, never a link, a plain file with one name, and at most settingsMost.
func CheckSettings(path, sum string) error {
	if len(sum) != len(SettingsSumBlank) || sum == SettingsSumBlank || strings.Trim(sum, "0123456789abcdef") != "" {
		return errors.New("its seal isn't one Smolcorp makes")
	}
	data, err := readSmall(path, settingsMost, errSettingsTooLarge)
	if err != nil {
		return err
	}
	if again, ok := SettingsSum([]byte(strings.ReplaceAll(string(data), sum, SettingsSumBlank))); !ok || again != sum {
		return errors.New("it was changed after Smolcorp wrote it")
	}
	return nil
}

// settingsStop is what the guard says when a desk's settings file isn't the one it was started with.
func settingsStop(err error) string {
	return "Stopped by Smolcorp's hire guard: this desk's settings file isn't the one Smolcorp started it with (" +
		err.Error() + "), so it can't go on. Ask the owner to hire it again."
}
