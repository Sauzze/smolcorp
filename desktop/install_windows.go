//go:build windows

package desktop

import (
	"errors"
	"fmt"
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
		// Said plainly, as Smolcorp.exe has no console: the folder is as it
		// was, and this copy goes on from where it is (or hands over to the
		// installed one, running).
		fmt.Fprintf(os.Stderr, "smolcorp: %v\n", err)
		notifyUser(err.Error())
		return false
	}
	// An update over a Smolcorp that runs: the new copy, started as one taking
	// its place, waits for the instance lock (waitForInstance), and only then
	// is the old one asked to make way (startThenMakeWay). If the new one
	// can't start, the old one isn't asked and runs on.
	err = startThenMakeWay(runningToMakeWay, func() error {
		return startProgram(installed, installedFromVariable+"="+filepath.Dir(program), restartedVariable+"=1")
	})
	if err != nil {
		said := fmt.Sprintf("Smolcorp is installed in %s, but couldn't start from there: %s.", filepath.Dir(installed), plainReason(err))
		fmt.Fprintf(os.Stderr, "smolcorp: %s\n", said)
		notifyUser(said)
		return false
	}
	return true
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

// install puts program, and smolcorp-cli.exe beside it, in installDir, both
// or neither, even while the old ones run (placePrograms), and puts that
// folder on your PATH. The installed program's path.
func install(program string) (string, error) {
	dir := installDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("Smolcorp couldn't make its folder %s: %s. Nothing was changed.", dir, plainReason(err))
	}
	installed := filepath.Join(dir, "Smolcorp.exe")
	programs := []newProgram{{from: program, to: installed}}
	// A build by hand has no console program beside it.
	if console := filepath.Join(filepath.Dir(program), "smolcorp-cli.exe"); fileExists(console) {
		programs = append(programs, newProgram{from: console, to: filepath.Join(dir, "smolcorp-cli.exe")})
	}
	if err := placePrograms(programs); err != nil {
		return "", err
	}
	moveStartupShortcuts(installed)
	if err := addToPath(dir); err != nil {
		fmt.Fprintf(os.Stderr, "smolcorp: not on PATH: %v\n", err)
	}
	return installed, nil
}

// removeOldPrograms takes away, quietly, what the last update left in the
// installed copy's folder: the programs it set aside, once nothing runs them.
func removeOldPrograms() {
	if program, err := executable(); err == nil && installedHere(program) && testCopyPort() == 0 {
		removeLeftovers(installDir(), installedPrograms)
	}
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
