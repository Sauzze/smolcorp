package desktop

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FXWININSTALL's rows on Windows itself (windows-latest CI). Their stand-ins are this test program again, copied in
// as Smolcorp.exe and smolcorp-cli.exe, told what to be by installStandInVariable.
const installStandInVariable = "SMOLCORP_TEST_INSTALL_STANDIN"

func init() {
	switch os.Getenv(installStandInVariable) {
	case "":
		return
	case "running":
		// A Smolcorp running: it holds the instance lock the row names, listens for an update's ask to make way,
		// and leaves when asked, as the app's tray quits.
		instanceName = os.Getenv("SMOLCORP_TEST_INSTANCE")
		if running, err := acquireInstance(); running || err != nil {
			os.Exit(3)
		}
		leave = func() { os.Exit(0) }
		listenForMakeWay()
		fmt.Println("ready")
		time.Sleep(2 * time.Minute)
		os.Exit(4)
	case "hook":
		// smolcorp-cli.exe in an agent's hook that runs on, till its input ends.
		fmt.Println("ready")
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	case "name":
		// smolcorp-cli.exe, or Smolcorp.exe, as a shell ran it: it says which, by its own long name.
		program, _ := os.Executable()
		if long, err := getLongPathName(program); err == nil {
			program = long
		}
		fmt.Println(filepath.Base(program))
		os.Exit(0)
	}
}

// copyOfThisProgram puts this test program at path, with extra bytes after it to tell the copies apart.
func copyOfThisProgram(t *testing.T, path, extra string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, extra...), 0o755); err != nil {
		t.Fatal(err)
	}
}

// startStandIn starts program as the stand-in it's told to be, and waits for it to say it's ready.
func startStandIn(t *testing.T, program, be string, env ...string) (*exec.Cmd, io.WriteCloser, <-chan error) {
	t.Helper()
	cmd := exec.Command(program)
	cmd.Env = append(append(os.Environ(), installStandInVariable+"="+be), env...)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ended := make(chan error, 1)
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(output).ReadString('\n')
		ready <- strings.TrimSpace(line)
		_, _ = io.Copy(io.Discard, output)
		ended <- cmd.Wait()
	}()
	t.Cleanup(func() {
		_ = input.Close()
		_ = cmd.Process.Kill()
		select {
		case <-ended:
		case <-time.After(10 * time.Second):
		}
	})
	select {
	case line := <-ready:
		if line != "ready" {
			t.Fatalf("%s as %s said %q", program, be, line)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("%s as %s never said it was ready", program, be)
	}
	return cmd, input, ended
}

// soonEnough retries check for up to 10 s: a program's file is let go of just after it ends.
func soonEnough(check func() bool) bool {
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if check() {
			return true
		}
	}
	return check()
}

// On Windows itself: an update while Smolcorp and its hooks run, with no quit. The programs in use are renamed aside,
// the new pair goes in, and the running copy hands over through the instance lock.
func TestOnWindowsAnUpdateWhileSmolcorpAndItsHooksRunHandsOver(t *testing.T) {
	saved := instanceName
	instanceName = `Local\Smolcorp-test-update-` + strconv.Itoa(os.Getpid())
	t.Cleanup(func() { instanceName = saved })
	dir, download := t.TempDir(), t.TempDir()
	var programs []newProgram
	for _, name := range installedPrograms {
		copyOfThisProgram(t, filepath.Join(dir, name), "old")
		copyOfThisProgram(t, filepath.Join(download, name), "new")
		programs = append(programs, newProgram{from: filepath.Join(download, name), to: filepath.Join(dir, name)})
	}
	_, _, appEnded := startStandIn(t, filepath.Join(dir, "Smolcorp.exe"), "running", "SMOLCORP_TEST_INSTANCE="+instanceName)
	_, hookInput, hookEnded := startStandIn(t, filepath.Join(dir, "smolcorp-cli.exe"), "hook")

	// The old way fails here, as it did on a tester's computer: a program that runs can't be replaced.
	next := filepath.Join(dir, "smolcorp-cli.exe.new")
	copyOfThisProgram(t, next, "new")
	if err := os.Rename(next, filepath.Join(dir, "smolcorp-cli.exe")); err == nil {
		t.Fatal("a running program was replaced: this row's programs aren't in use")
	}
	_ = os.Remove(next)

	if err := placePrograms(programs); err != nil {
		t.Fatalf("the update while both run: %v", err)
	}
	for _, name := range installedPrograms {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !strings.HasSuffix(string(data), "new") {
			t.Errorf("%s isn't the new one (%v)", name, err)
		}
		if _, err := os.Stat(filepath.Join(dir, name+".old-1")); err != nil {
			t.Errorf("the old %s, still running, isn't set aside: %v", name, err)
		}
	}

	ask, done := runningToMakeWay()
	asked := ask()
	done()
	if !asked {
		t.Fatal("the running copy wasn't asked to make way")
	}
	select {
	case err := <-appEnded:
		if err != nil {
			t.Errorf("the running copy ended with %v, want it to leave", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the running copy didn't make way")
	}
	if running, err := waitForInstance(); running || err != nil {
		t.Errorf("the new copy didn't get the instance lock: running %v, %v", running, err)
	}

	// The next start takes away the old Smolcorp.exe, which nothing runs now; the hook's program stays till it ends.
	gone := func(name string) bool {
		removeLeftovers(dir, installedPrograms)
		_, err := os.Stat(filepath.Join(dir, name))
		return errors.Is(err, os.ErrNotExist)
	}
	if !soonEnough(func() bool { return gone("Smolcorp.exe.old-1") }) {
		t.Error("the old Smolcorp.exe stayed once it had ended")
	}
	if gone("smolcorp-cli.exe.old-1") {
		t.Error("the old smolcorp-cli.exe went while a hook still ran it")
	}
	_ = hookInput.Close()
	<-hookEnded
	if !soonEnough(func() bool { return gone("smolcorp-cli.exe.old-1") }) {
		t.Error("the old smolcorp-cli.exe stayed once its hook had ended")
	}
}

// Round 2, item 2, per shell: the hook's command, as written, runs smolcorp-cli.exe through cmd (Codex), PowerShell
// (Gemini CLI, Grok) and Git Bash (Claude Code), from the usual folder and from one under a name with a space.
func TestOnWindowsEachShellRunsTheHooksProgram(t *testing.T) {
	gitBash := filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
	for _, folder := range []string{"ada", "Ada Lovelace"} {
		base := filepath.Join(t.TempDir(), folder, "Smolcorp")
		copyOfThisProgram(t, filepath.Join(base, "Smolcorp.exe"), "app")
		copyOfThisProgram(t, filepath.Join(base, "smolcorp-cli.exe"), "cli")
		program := shellPath(filepath.Join(base, "smolcorp-cli.exe"))
		command := program + " hook --port 7710"
		if folder == "ada" && !strings.HasSuffix(program, "/smolcorp-cli.exe") {
			t.Errorf("the usual folder: %s, want the long path", program)
		}
		shells := map[string]*exec.Cmd{"powershell": exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)}
		cmd := exec.Command("cmd.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /s /c "` + command + `"`}
		shells["cmd"] = cmd
		if _, err := os.Stat(gitBash); err == nil {
			shells["git bash"] = exec.Command(gitBash, "-c", command)
		} else {
			t.Logf("no Git Bash at %s", gitBash)
		}
		for shell, run := range shells {
			if shell == "powershell" && strings.HasPrefix(program, `"`) {
				t.Logf("%s: no short name here, and PowerShell runs no quoted path (windowsPathNeedsQuotes)", folder)
				continue
			}
			run.Env = append(os.Environ(), installStandInVariable+"=name")
			out, err := run.CombinedOutput()
			if got := strings.TrimSpace(string(out)); err != nil || got != "smolcorp-cli.exe" {
				t.Errorf("%s in %s ran %q (%v), want smolcorp-cli.exe: %s", shell, folder, got, err, command)
			}
		}
	}
}

// Round 2, item 1: Smolcorp's own shortcut in the Startup folder becomes the Run entry for the installed copy, at
// install and when Open at login is set; another app's stays, and so does one to smolcorp-cli.exe; one stays if the
// entry can't be written.
func TestOnWindowsSmolcorpsStartupShortcutBecomesTheRunEntry(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	startup := startupFolder()
	ours, other := filepath.Join(startup, "Smolcorp.lnk"), filepath.Join(startup, "Other.lnk")
	serve := filepath.Join(startup, "Smolcorp server.lnk")
	put := func() {
		if err := os.MkdirAll(startup, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, ours, string(aShortcut(`C:\Users\ada\Downloads\Smolcorp\Smolcorp.exe`, true)))
		writeFile(t, other, string(aShortcut(`C:\Program Files\Other\other.exe`, true)))
		writeFile(t, serve, string(aShortcut(`C:\Users\ada\AppData\Local\Smolcorp\smolcorp-cli.exe`, true)))
	}
	saved := writeRunValue
	t.Cleanup(func() { writeRunValue = saved })
	var written string
	writeRunValue = func(program string) error { written = program; return nil }
	there := func(path string) bool { _, err := os.Stat(path); return err == nil }

	put()
	moveStartupShortcuts(installed)
	if written != installed || there(ours) || !there(other) || !there(serve) {
		t.Errorf("at install: Run entry %q, Smolcorp's shortcut there %v, another's %v, smolcorp-cli.exe's %v", written,
			there(ours), there(other), there(serve))
	}
	put()
	written = ""
	setOpenAtLogin(true)
	if written == "" || there(ours) || !there(other) || !there(serve) {
		t.Errorf("Open at login set: Run entry %q, Smolcorp's shortcut there %v, another's %v, smolcorp-cli.exe's %v", written,
			there(ours), there(other), there(serve))
	}
	put()
	writeRunValue = func(string) error { return errors.New("refused") }
	moveStartupShortcuts(installed)
	if !there(ours) {
		t.Error("the shortcut went though the Run entry couldn't be written: nothing would open Smolcorp at sign-in")
	}
}
