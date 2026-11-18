package desktop

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sauzze/smolcorp/internal/guard"
)

// A desk's settings file (SETTINGSFILE1, SIERRA's ruling). A desk's Claude Code settings used to ride in its command
// line, which on Windows holds 32,767 characters, and every rule, account and past hire made them longer. Now the core
// writes them to <support>/hires/<id>/settings.json, sealed by their own hash (internal/guard/settings.go), and every
// start (the tmux engine's ticket, Take over's, the Windows launcher and its route, the GM's) names only that file:
// --settings <file>. The file is the desk's to read and never to write: the sandbox's Policy says so ("its brief and
// its fence"), and so do the settings' own Edit rules (hires/*/settings*, hires/*/*.json). The seal is checked as the
// file is written, again just before the desk starts, and by the guard before and after each of the desk's tool calls.
//
// There is no other form: if the file can't be written or checked, the desk doesn't start, and the owner is told why.

// sealedSettings is a desk's settings file and the seal it was written with.
type sealedSettings struct {
	Path string `json:"path,omitempty"`
	Sum  string `json:"sum,omitempty"`
}

// settingsFileMode is the file's mode: the owner's to read alone where the system has modes. Windows' read-only
// attribute would keep the core from replacing it.
func settingsFileMode() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0o600
	}
	return 0o400
}

// writeHireSettings writes hire o.HireID's settings to their file in its own folder, sealed, and checks the file as
// the guard will: refused, in words, if the settings can't be made, written or checked.
func writeHireSettings(o hireSettingsOptions) (sealedSettings, error) {
	o.File = filepath.Join(hireFolder(o.Support, o.HireID), hireSettingsFile)
	text, err := hireSettingsText(o)
	if err != nil {
		return sealedSettings{}, err
	}
	sum, ok := guard.SettingsSum(text)
	if !ok {
		return sealedSettings{}, settingsUnwritten("its seal had nowhere to go")
	}
	text = []byte(strings.ReplaceAll(string(text), guard.SettingsSumBlank, sum))
	if err := writeSettingsFile(o.File, text); err != nil {
		return sealedSettings{}, settingsUnwritten(err.Error())
	}
	settings := sealedSettings{Path: o.File, Sum: sum}
	if refusal := settings.refusal(); refusal != nil {
		return sealedSettings{}, refusal
	}
	return settings, nil
}

// writeSettingsFile puts text at path through a new file of its own beside it, renamed over whatever was there: a
// link or a second name at path is replaced, never written through.
func writeSettingsFile(path string, text []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+newID())
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, settingsFileMode())
	if err != nil {
		return err
	}
	_, err = file.Write(text)
	if closed := file.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temporary, path)
	}
	if err != nil {
		_ = os.Remove(temporary)
	}
	return err
}

// settingsUnwritten is the refusal when a desk's settings file can't be written.
func settingsUnwritten(why string) *hireRefusal {
	return &hireRefusal{Status: http.StatusInternalServerError, Message: "Smolcorp couldn't write this desk's settings file, where the desk " +
		"can't change it (" + why + "), so the desk didn't start, and nothing runs unguarded. Check the disk has room, then hire again."}
}

// refusal is why s can't start a desk, nil when its file is the one sealed: a start refuses a changed, replaced or
// linked file, and one with no seal, in words.
func (s sealedSettings) refusal() *hireRefusal {
	if s.Path == "" || s.Sum == "" {
		return &hireRefusal{Status: http.StatusInternalServerError, Message: "This desk's start has no sealed settings file, so it didn't start: " +
			"Smolcorp never starts a desk unguarded. Hire again."}
	}
	if err := guard.CheckSettings(s.Path, s.Sum); err != nil {
		return &hireRefusal{Status: http.StatusConflict, Message: "This desk's settings file isn't the one Smolcorp wrote (" + err.Error() +
			"), so the desk didn't start: something on this computer changed or replaced it. Hire again."}
	}
	return nil
}
