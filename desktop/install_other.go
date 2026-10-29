//go:build !windows

package desktop

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

// On Linux, install.sh puts Smolcorp in ~/.local/bin, and a package manager
// in /usr; the checklist says when this copy is neither, as your agents'
// hooks then name a folder that may go. Elsewhere (a Go build on a Mac, for
// the tests) there's nothing to say.

// installedPlaces are the folders an installed copy is in.
func installedPlaces() []string {
	return []string{filepath.Join(homeDir, ".local", "bin"), "/usr/local/bin", "/usr/bin", "/opt"}
}

// installCheck is the install's line of the checklist, on Linux.
func installCheck() *check {
	if runtime.GOOS != "linux" {
		return nil
	}
	program, err := executable()
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(program); err == nil {
		program = resolved
	}
	c := check{ID: "install", Title: "Smolcorp is installed"}
	for _, place := range installedPlaces() {
		if strings.HasPrefix(program, place+string(filepath.Separator)) {
			c.Status, c.Detail = checkOK, "As "+program+", in your app menu."
			return &c
		}
	}
	c.Status = checkFix
	c.Detail = "Running from " + filepath.Dir(program) + ". Run ./install.sh from the download, then open Smolcorp " +
		"from your app menu: your agents then report to the installed copy, which stays put."
	return &c
}

// installAndRelaunch: install.sh installs here.
func installAndRelaunch() bool { return false }

var errInstallByScript = errors.New("Run ./install.sh from the folder you downloaded, then open Smolcorp from your app menu.")

// installNow: install.sh installs here.
func installNow() error { return errInstallByScript }
