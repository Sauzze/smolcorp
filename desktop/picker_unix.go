//go:build !windows

package desktop

import (
	"errors"
	"os/exec"
	"strings"
)

// folderPicker is the desktop's own folder picker, where there is one: GNOME's
// zenity, or KDE's kdialog. The start folder and the title are arguments of
// their own, never read as a command.
func folderPicker(start, title string) *exec.Cmd {
	if path, err := exec.LookPath("zenity"); err == nil {
		return exec.Command(path, "--file-selection", "--directory", "--title="+title, "--filename="+strings.TrimSuffix(start, "/")+"/")
	}
	if path, err := exec.LookPath("kdialog"); err == nil {
		return exec.Command(path, "--title", title, "--getexistingdirectory", start)
	}
	return nil
}

// pickedFolder is what a picker printed, or false when it was closed without
// a choice: both pickers say so by their exit status.
func pickedFolder(output []byte, err error) (string, bool, error) {
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	folder := strings.TrimRight(string(output), "\r\n")
	return folder, folder != "", nil
}
