//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Smolcorp lives in %LOCALAPPDATA%\Smolcorp, for you alone, with no
// administrator: the two programs, the console one on your PATH. A copy
// opened from anywhere else (Downloads, an unzipped folder) installs itself
// there and starts from there instead, so your agents' hooks, the firewall's
// rules and the login item all name the installed copy, which stays put.

// installDir is where Smolcorp is installed.
func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserCacheDir()
	}
	return filepath.Join(base, "Smolcorp")
}

// installedHere is whether program is the installed copy.
func installedHere(program string) bool {
	return strings.EqualFold(filepath.Clean(filepath.Dir(program)), filepath.Clean(installDir()))
}

// installCheck is the install's line of the checklist.
func installCheck() *check {
	program, err := executable()
	if err != nil {
		return nil
	}
	c := check{ID: "install", Title: "Smolcorp is installed"}
	if installedHere(program) {
		c.Status, c.Detail = checkOK, "In "+installDir()+", with smolcorp-cli on your PATH."
		if from := installedFrom; from != "" && !strings.EqualFold(from, installDir()) {
			c.Detail += " The copy you opened in " + from + " can go."
		}
		return &c
	}
	c.Status, c.Fix, c.Label = checkFix, fixInstall, "Install"
	c.Detail = "Running from " + filepath.Dir(program) + ". Installed in its own folder, your agents keep reporting to it " +
		"wherever this folder goes."
	return &c
}

// installAndRelaunch installs this copy and starts the installed one, where
// this isn't it: true when it did, and this one should go. A test build on
// a port of its own (SMOLCORP_PORT), or SMOLCORP_NO_INSTALL=1, stays where
// it is.
func installAndRelaunch() bool {
	if os.Getenv("SMOLCORP_PORT") != "" || os.Getenv("SMOLCORP_NO_INSTALL") == "1" {
		return false
	}
	program, err := executable()
	if err != nil || installedHere(program) {
		return false
	}
	installed, err := install(program)
	if err != nil {
		// The installed copy may be running, its files in use: the instance
		// lock hands over to it next, and the checklist says how to update.
		fmt.Fprintf(os.Stderr, "smolcorp: not installed: %v\n", err)
		return false
	}
	return startProgram(installed, installedFromVariable+"="+filepath.Dir(program)) == nil
}

// installNow is the checklist's Install: this copy installed, and the
// installed one started in its place.
func installNow() error {
	program, err := executable()
	if err != nil {
		return err
	}
	installed, err := install(program)
	if err != nil {
		return err
	}
	return relaunch(installed, installedFromVariable+"="+filepath.Dir(program))
}

// install copies program, and smolcorp-cli.exe beside it, into installDir,
// and puts that folder on your PATH. The installed program's path.
func install(program string) (string, error) {
	dir := installDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	installed := filepath.Join(dir, "Smolcorp.exe")
	for _, name := range []string{filepath.Base(program), "smolcorp-cli.exe"} {
		from := filepath.Join(filepath.Dir(program), name)
		if _, err := os.Stat(from); err != nil {
			continue // a build by hand has no console program beside it
		}
		to := filepath.Join(dir, name)
		if strings.EqualFold(name, filepath.Base(program)) {
			to = installed
		}
		if err := copyOver(from, to); err != nil {
			return "", fmt.Errorf("Smolcorp is open from %s already: quit it from its tray icon, then open this copy again to update it (%w)", dir, err)
		}
	}
	if err := addToPath(dir); err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: not on PATH: %v\n", err)
	}
	return installed, nil
}

// copyOver writes from's bytes to to through a file beside it, renamed
// over it: never half a program, and an error while the old one runs.
func copyOver(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	next := to + ".new"
	target, err := os.OpenFile(next, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		os.Remove(next)
		return err
	}
	if err := target.Close(); err != nil {
		os.Remove(next)
		return err
	}
	if err := os.Rename(next, to); err != nil {
		os.Remove(next)
		return err
	}
	return nil
}

// addToPath puts dir at the end of your own PATH, once, and tells the
// programs that listen, as Explorer, so a terminal opened next has it.
func addToPath(dir string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	path, _, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	for _, part := range strings.Split(path, ";") {
		if strings.EqualFold(filepath.Clean(strings.TrimSpace(part)), filepath.Clean(dir)) {
			return nil
		}
	}
	if path != "" && !strings.HasSuffix(path, ";") {
		path += ";"
	}
	if err := key.SetExpandStringValue("Path", path+dir); err != nil {
		return err
	}
	environment, _ := windows.UTF16PtrFromString("Environment")
	const hwndBroadcast, wmSettingChange, smtoAbortIfHung = 0xffff, 0x001A, 0x0002
	var result uintptr
	_, _, _ = user32.NewProc("SendMessageTimeoutW").Call(hwndBroadcast, wmSettingChange, 0,
		uintptr(unsafe.Pointer(environment)), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&result)))
	return nil
}
