package sandbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/Sauzze/smolcorp/internal/procmem"
	"golang.org/x/sys/windows"
)

// SA-W1, Windows' jail spike (PL126A §2.3, SANDBOX-PLAN §A3.3). The stand-ins are this test binary run again:
// partVar says which part it plays. Every row is a status (allowed, denied, or the error); nothing is read but the
// stand-ins below, and nothing changed but the test's own temp folder. The tables are logged, so CI's run is the
// spike's record.
//
// The rows mean something only for a standard user (PL126A §4): an administrator holding SeDebugPrivilege opens any
// process whatever its access list. Such a run skips, and fails when SMOLCORP_SANDBOX_REQUIRED=1, as in CI's
// windows-jail job, which runs these as a standard user it makes and wants each test's PASS by name.
const (
	partVar    = "SMOLCORP_WINJAIL_PART"
	argsVar    = "SMOLCORP_WINJAIL_ARGS"
	standInKey = "FAKEKEY"
)

func TestMain(m *testing.M) {
	switch os.Getenv(partVar) {
	case "":
		os.Exit(m.Run())
	case "core": // the core: `smolcorp serve`'s own access list (SK3)
		if err := procmem.DenyMemoryAccess(); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		standBy()
	case "program": // a program of yours, or a desk's, with nothing set
		standBy()
	case "probe":
		var args probeArgs
		if err := json.Unmarshal([]byte(os.Getenv(argsVar)), &args); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		out, _ := json.Marshal(args.run())
		os.Stdout.Write(out)
	}
	os.Exit(0)
}

// standBy says it's ready, then waits to be ended: two minutes at most.
func standBy() {
	fmt.Println("ready")
	time.Sleep(2 * time.Minute)
}

func TestAJailedHireCantOpenTheCoreTheGMAnotherHireOrYourOtherPrograms(t *testing.T) {
	requireStandardUser(t)
	targets := map[string]uint32{
		"the core":                 standIn(t, "core", nil),
		"another program of yours": standIn(t, "program", nil),
		"the GM":                   standIn(t, "program", newJail(t, "gm")),
		"another hire":             standIn(t, "program", newJail(t, "builder-2")),
	}
	threads := map[string]uint32{}
	for name, pid := range targets {
		threads[name] = firstThread(t, pid)
	}
	// The control: you (the core) still get into the GM, another hire and your other program.
	for _, name := range []string{"the GM", "another hire", "another program of yours"} {
		handle, err := windows.OpenProcess(windows.PROCESS_VM_READ|windows.PROCESS_VM_WRITE|windows.PROCESS_VM_OPERATION, false, targets[name])
		if err != nil {
			t.Fatalf("the core should still open %s's memory (the rows mean nothing otherwise): %v", name, err)
		}
		windows.CloseHandle(handle)
	}

	rows := probe(t, newJail(t, "builder-1"), probeArgs{Groups: []string{"processes"}, Processes: targets, Threads: threads})
	// Another desk: nothing at all, not even whether it runs.
	for _, name := range []string{"the GM", "another hire"} {
		for _, right := range processRights {
			want(t, rows, name+": "+right.name, "denied")
		}
		for _, right := range threadRights {
			want(t, rows, name+": "+right.name, "denied")
		}
	}
	// The core and your other programs: nothing that reads or changes them or runs as them. Stopping one, waiting
	// for it and reading a thread's registers are logged only.
	for _, name := range []string{"the core", "another program of yours"} {
		for _, right := range []string{"read its memory", "write its memory", "start a thread in it", "copy a handle out of it",
			"start a program as its child", "suspend it", "set a thread's registers", "suspend a thread"} {
			want(t, rows, name+": "+right, "denied")
		}
	}
	want(t, rows, "the core: rewrite its access list", "denied")
	want(t, rows, "the core: take it over", "denied")
	for name := range targets {
		if got := rows[name+": act with its token"]; got == "works" {
			t.Errorf("a hire acts with %s's token", name)
		}
	}
}

func TestAJailedHireCantReadTheSupportFolderOrChangeTheProtectedFiles(t *testing.T) {
	requireStandardUser(t)
	root := t.TempDir()
	support := filepath.Join(root, "support")
	paths := map[string]string{
		"support": support, "key": filepath.Join(support, "setup-key"), "guard": filepath.Join(support, "hire-guard.json"),
		"brief": filepath.Join(support, "hires", "builder-1", "BRIEF.md"), "status": filepath.Join(support, "hires", "builder-1", "STATUS.md"),
		"other": filepath.Join(support, "hires", "builder-2", "BRIEF.md"), "worktree": filepath.Join(root, "worktree"),
		"home": filepath.Join(root, "home"),
	}
	paths["readme"] = filepath.Join(paths["worktree"], "README.md")
	paths["claude"] = filepath.Join(paths["worktree"], ".claude")
	paths["settings"] = filepath.Join(paths["claude"], "settings.json")
	paths["notes"] = filepath.Join(paths["claude"], "notes.md")
	paths["profile"] = filepath.Join(paths["home"], "profile.txt")
	hire, other := newJail(t, "builder-1"), newJail(t, "builder-2")
	for _, folder := range []string{filepath.Dir(paths["brief"]), filepath.Dir(paths["other"]), paths["worktree"], paths["home"]} {
		must(t, os.MkdirAll(folder, 0o700))
	}
	must(t, hire.LetIn(paths["worktree"], true)) // before its files, which inherit the label
	must(t, os.MkdirAll(paths["claude"], 0o700))
	for name, text := range map[string]string{"key": standInKey, "guard": `{"fence":1}`, "brief": "brief", "status": "status",
		"other": "brief", "readme": "readme", "settings": `{"fence":1}`, "notes": "notes", "profile": "profile"} {
		must(t, os.WriteFile(paths[name], []byte(text), 0o600))
	}
	must(t, DenyToHires(support))
	must(t, ReadOnlyToHires(paths["guard"], ""))
	must(t, hire.LetIn(paths["brief"], false))
	must(t, hire.LetIn(paths["status"], true))
	must(t, other.LetIn(paths["other"], false))
	must(t, ReadOnlyToHires(paths["settings"], paths["worktree"]))
	// The control: you (the core) still read the key and the other hire's brief.
	for _, name := range []string{"key", "other"} {
		if _, err := os.ReadFile(paths[name]); err != nil {
			t.Fatalf("the core should still read %s (the rows mean nothing otherwise): %v", name, err)
		}
	}

	rows := probe(t, hire, probeArgs{Groups: []string{"files"}, Paths: paths})
	for row, value := range map[string]string{
		"read the setup key": "denied", "list the support folder": "denied", "add a file to the support folder": "denied",
		"remove the setup key": "denied", "rename the support folder": "denied", "move the support folder into its worktree": "denied",
		"read the guard file": "allowed", "change the guard file": "denied", "remove the guard file": "denied",
		"rewrite the guard file's access list": "denied",
		"read its own brief":                   "allowed", "change its own brief": "denied", "write its own status": "allowed",
		"read another hire's brief": "denied",
		"write in its worktree":     "allowed", "remove a file at its worktree's top": "allowed",
		"read its settings": "allowed", "change its settings": "denied", "remove its settings": "denied",
		"rewrite its settings' access list": "denied", "rename its .claude": "denied", "rewrite its .claude's access list": "denied",
		"remove another file in its .claude": "allowed",
		"read a file in your home":           "allowed", "write in your home": "denied",
	} {
		want(t, rows, row, value)
	}
	for name, text := range map[string]string{"key": standInKey, "guard": `{"fence":1}`, "settings": `{"fence":1}`} {
		if got, err := os.ReadFile(paths[name]); err != nil || string(got) != text {
			t.Errorf("%s changed under the hire: %q, %v", name, got, err)
		}
	}
}

func TestAJailedHireStaysInItsJob(t *testing.T) {
	requireStandardUser(t)
	jail := newJail(t, "builder-1")
	rows := probe(t, jail, probeArgs{Groups: []string{"job"}})
	low, err := windows.CreateWellKnownSid(windows.WinLowLabelSid)
	must(t, err)
	for row, value := range map[string]string{
		"leave its job": "denied", "start a program": "allowed", "read and write its own program's memory": "allowed",
		"its integrity level": low.String(), "holds a privilege beyond traversing folders": "no",
	} {
		want(t, rows, row, value)
	}
	pid, err := strconv.ParseUint(rows["its program's pid"], 10, 32)
	if err != nil {
		t.Fatalf("the hire's own program: %v", err)
	}
	if in, err := jail.Holds(uint32(pid)); err != nil || !in {
		t.Fatalf("the hire's program should be in its job: %v, %v", in, err)
	}
	// The hire has exited; its program runs on in the job until the jail closes.
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	must(t, err)
	defer windows.CloseHandle(process)
	if event, _ := windows.WaitForSingleObject(process, 0); event != uint32(windows.WAIT_TIMEOUT) {
		t.Fatal("the hire's program ended before the jail closed")
	}
	must(t, jail.Close())
	if event, err := windows.WaitForSingleObject(process, 10_000); event != windows.WAIT_OBJECT_0 {
		t.Errorf("the hire's program outlived its jail: %v", err)
	}
}

func TestAJailRefusesAProgramThatBringsItsOwnTokenParentOrBreakaway(t *testing.T) {
	jail := newJail(t, "builder-1")
	for name, attr := range map[string]*syscall.SysProcAttr{
		"a token": {Token: syscall.Token(windows.GetCurrentProcessToken())}, "a parent": {ParentProcess: syscall.Handle(windows.CurrentProcess())},
		"an access list": {ProcessAttributes: &syscall.SecurityAttributes{}}, "a breakaway": {CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB},
		"a handle": {AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(windows.CurrentProcess())}},
	} {
		cmd := exec.Command(os.Args[0])
		cmd.SysProcAttr = attr
		if err := jail.Start(cmd); err == nil {
			cmd.Process.Kill()
			cmd.Wait()
			t.Errorf("a program with %s of its own started in the jail", name)
		}
	}
	must(t, jail.Close())
	if err := jail.Start(exec.Command(os.Args[0])); err == nil {
		t.Error("a closed jail started a program")
	}
}

// requireStandardUser skips an administrator's run, or fails it where the rows are required.
func requireStandardUser(t *testing.T) {
	t.Helper()
	token := windows.GetCurrentProcessToken()
	why := ""
	if token.IsElevated() {
		why = "this test runs elevated"
	} else if holdsPrivilege(t, token, "SeDebugPrivilege") {
		why = "this account holds SeDebugPrivilege"
	}
	if why == "" {
		return
	}
	why += ": an administrator opens any process, so the jail's rows prove nothing here. Run them as a standard user (PL126A §4)"
	if os.Getenv("SMOLCORP_SANDBOX_REQUIRED") == "1" {
		t.Fatal(why)
	}
	t.Skip(why)
}

// holdsPrivilege says whether token holds the privilege at all, on or off.
func holdsPrivilege(t *testing.T, token windows.Token, name string) bool {
	var luid windows.LUID
	must(t, windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid))
	for _, privilege := range privileges(token) {
		if privilege.Luid == luid {
			return true
		}
	}
	return false
}

func privileges(token windows.Token) []windows.LUIDAndAttributes {
	buffer := make([]byte, 4096)
	var size uint32
	if windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], uint32(len(buffer)), &size) != nil {
		return nil
	}
	held := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0]))
	return append([]windows.LUIDAndAttributes(nil), held.AllPrivileges()...)
}

func newJail(t *testing.T, id string) *Jail {
	t.Helper()
	jail, err := NewJail(id)
	must(t, err)
	t.Cleanup(func() { jail.Close() })
	return jail
}

// standIn starts this binary as part, in jail or not, and waits for it to say it's ready.
func standIn(t *testing.T, part string, jail *Jail) uint32 {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), partVar+"="+part)
	out, err := cmd.StdoutPipe()
	must(t, err)
	if jail != nil {
		err = jail.Start(cmd)
	} else {
		err = cmd.Start()
	}
	must(t, err)
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	if line, err := bufio.NewReader(out).ReadString('\n'); strings.TrimSpace(line) != "ready" {
		t.Fatalf("the %s stand-in didn't start: %q, %v", part, line, err)
	}
	return uint32(cmd.Process.Pid)
}

// firstThread is one of pid's threads, found by you (the core).
func firstThread(t *testing.T, pid uint32) uint32 {
	t.Helper()
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	must(t, err)
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID == pid {
			return entry.ThreadID
		}
	}
	t.Fatalf("no thread of %d", pid)
	return 0
}

// probe runs the prober in jail and returns its rows, logged as a table.
func probe(t *testing.T, jail *Jail, args probeArgs) map[string]string {
	t.Helper()
	encoded, err := json.Marshal(args)
	must(t, err)
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), partVar+"=probe", argsVar+"="+string(encoded))
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	must(t, jail.Start(cmd))
	if err := cmd.Wait(); err != nil {
		t.Fatalf("the prober: %v\n%s%s", err, out.String(), errs.String())
	}
	rows := map[string]string{}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("the prober's rows: %v\n%s%s", err, out.String(), errs.String())
	}
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("%-62s %s", name, rows[name])
	}
	return rows
}

func want(t *testing.T, rows map[string]string, row, value string) {
	t.Helper()
	if got, ok := rows[row]; !ok {
		t.Errorf("%s: no such row", row)
	} else if got != value {
		t.Errorf("%s: %s, want %s", row, got, value)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The prober: what a hire tries, from inside its jail.

type probeArgs struct {
	Groups    []string
	Processes map[string]uint32 // a target's name: its pid
	Threads   map[string]uint32 // a target's name: one of its threads
	Paths     map[string]string // a stand-in's name: its path
}

type right struct {
	name   string
	access uint32
}

var processRights = []right{
	{"read its memory", windows.PROCESS_VM_READ},
	{"write its memory", windows.PROCESS_VM_WRITE | windows.PROCESS_VM_OPERATION},
	{"start a thread in it", windows.PROCESS_CREATE_THREAD},
	{"copy a handle out of it", windows.PROCESS_DUP_HANDLE},
	{"start a program as its child", windows.PROCESS_CREATE_PROCESS},
	{"suspend it", windows.PROCESS_SUSPEND_RESUME},
	{"rewrite its access list", windows.WRITE_DAC},
	{"take it over", windows.WRITE_OWNER},
	{"stop it", windows.PROCESS_TERMINATE},
	{"ask whether it runs", windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.SYNCHRONIZE},
}

var threadRights = []right{
	{"read a thread's registers", windows.THREAD_GET_CONTEXT},
	{"set a thread's registers", windows.THREAD_SET_CONTEXT},
	{"suspend a thread", windows.THREAD_SUSPEND_RESUME},
	{"ask about a thread", windows.THREAD_QUERY_LIMITED_INFORMATION},
}

func (args probeArgs) run() map[string]string {
	rows := map[string]string{}
	for _, group := range args.Groups {
		switch group {
		case "processes":
			args.processes(rows)
		case "files":
			args.files(rows)
		case "job":
			job(rows)
		}
	}
	return rows
}

func (args probeArgs) processes(rows map[string]string) {
	for name, pid := range args.Processes {
		for _, right := range processRights {
			handle, err := windows.OpenProcess(right.access, false, pid)
			if err == nil {
				windows.CloseHandle(handle)
			}
			rows[name+": "+right.name] = status(err)
		}
		for _, right := range threadRights {
			handle, err := windows.OpenThread(right.access, false, args.Threads[name])
			if err == nil {
				windows.CloseHandle(handle)
			}
			rows[name+": "+right.name] = status(err)
		}
		rows[name+": act with its token"] = actAs(pid)
	}
}

// actAs takes pid's token, if it can, and looks for a file that isn't there with it: "works" if the answer is that
// it isn't there, so the token acted for the hire. It reads nothing.
func actAs(pid uint32) string {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "no token: " + status(err)
	}
	defer windows.CloseHandle(process)
	var token, as windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &token); err != nil {
		return "no token: " + status(err)
	}
	defer token.Close()
	if err := windows.DuplicateTokenEx(token, windows.TOKEN_IMPERSONATE|windows.TOKEN_QUERY, nil,
		windows.SecurityImpersonation, windows.TokenImpersonation, &as); err != nil {
		return "no copy: " + status(err)
	}
	defer as.Close()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.SetThreadToken(nil, as); err != nil {
		return "not used: " + status(err)
	}
	defer windows.RevertToSelf()
	_, err = os.Stat(filepath.Join(os.TempDir(), "smolcorp-winjail-none"))
	if errors.Is(err, fs.ErrNotExist) {
		return "works"
	}
	return "not used: " + status(err)
}

func (args probeArgs) files(rows map[string]string) {
	p := args.Paths
	read := func(name string) error { _, err := os.ReadFile(p[name]); return err }
	list := func(name string) error { _, err := os.ReadDir(p[name]); return err }
	change := func(name string) error {
		file, err := os.OpenFile(p[name], os.O_WRONLY|os.O_APPEND, 0)
		if err == nil {
			file.Close()
		}
		return err
	}
	write := func(name, file string) error { return os.WriteFile(filepath.Join(p[name], file), []byte("x"), 0o600) }
	// Reads, then writes, then removals and renames: a row wrongly allowed changes as little as it can.
	for row, err := range map[string]error{
		"read the setup key": read("key"), "list the support folder": list("support"), "read the guard file": read("guard"),
		"read its own brief": read("brief"), "read another hire's brief": read("other"), "read its settings": read("settings"),
		"read a file in your home": read("profile"),
	} {
		rows[row] = status(err)
	}
	for row, err := range map[string]error{
		"add a file to the support folder": write("support", "planted"), "change the guard file": change("guard"),
		"rewrite the guard file's access list": openFor(p["guard"], windows.WRITE_DAC),
		"change its own brief":                 change("brief"), "write its own status": os.WriteFile(p["status"], []byte("done"), 0o600),
		"write in its worktree": write("worktree", "work.txt"), "change its settings": change("settings"),
		"rewrite its settings' access list": openFor(p["settings"], windows.WRITE_DAC),
		"rewrite its .claude's access list": openFor(p["claude"], windows.WRITE_DAC), "write in your home": write("home", "planted"),
	} {
		rows[row] = status(err)
	}
	rows["remove the setup key"] = status(os.Remove(p["key"]))
	rows["remove the guard file"] = status(os.Remove(p["guard"]))
	rows["remove its settings"] = status(os.Remove(p["settings"]))
	rows["remove another file in its .claude"] = status(os.Remove(p["notes"]))
	rows["remove a file at its worktree's top"] = status(os.Remove(p["readme"]))
	rows["rename its .claude"] = status(os.Rename(p["claude"], p["claude"]+"-moved"))
	rows["rename the support folder"] = status(os.Rename(p["support"], p["support"]+"-moved"))
	rows["move the support folder into its worktree"] = status(os.Rename(p["support"], filepath.Join(p["worktree"], "support")))
}

// openFor opens path for access alone, and closes it: whether the access would be granted.
func openFor(path string, access uint32) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err == nil {
		windows.CloseHandle(handle)
	}
	return err
}

func job(rows map[string]string) {
	away := exec.Command(os.Args[0])
	away.Env = append(os.Environ(), partVar+"=program")
	away.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB}
	err := away.Start()
	if err == nil {
		away.Process.Kill()
		away.Wait()
	}
	rows["leave its job"] = status(err)
	// Left running: closing the jail must end it.
	stay := exec.Command(os.Args[0])
	stay.Env = append(os.Environ(), partVar+"=program")
	err = stay.Start()
	rows["start a program"] = status(err)
	if err == nil {
		rows["its program's pid"] = strconv.Itoa(stay.Process.Pid)
		handle, err := windows.OpenProcess(windows.PROCESS_VM_READ|windows.PROCESS_VM_WRITE|windows.PROCESS_VM_OPERATION, false, uint32(stay.Process.Pid))
		if err == nil {
			windows.CloseHandle(handle)
		}
		rows["read and write its own program's memory"] = status(err)
	}
	token := windows.GetCurrentProcessToken()
	buffer := make([]byte, 256)
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenIntegrityLevel, &buffer[0], uint32(len(buffer)), &size); err != nil {
		rows["its integrity level"] = status(err)
	} else {
		rows["its integrity level"] = (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buffer[0])).Label.Sid.String()
	}
	var traverse windows.LUID
	windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeChangeNotifyPrivilege"), &traverse)
	rows["holds a privilege beyond traversing folders"] = "no"
	for _, privilege := range privileges(token) {
		if privilege.Luid != traverse {
			rows["holds a privilege beyond traversing folders"] = "yes"
		}
	}
}

func status(err error) string {
	switch {
	case err == nil:
		return "allowed"
	case errors.Is(err, windows.ERROR_ACCESS_DENIED), errors.Is(err, fs.ErrPermission):
		return "denied"
	}
	return err.Error()
}
