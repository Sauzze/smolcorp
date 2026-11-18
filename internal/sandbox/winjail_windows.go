package sandbox

// Windows' jail for a hire (SA-W1, SANDBOX-PLAN §A3.3). A hire runs as you, so it has your rights; the jail takes back
// the ones that matter, with three things Windows lets a standard user do for the programs it starts:
//
//   - A restricted token. Windows checks every access a restricted token asks for twice, once against its usual SIDs
//     and once against its restricting SIDs, and grants only what both allow. A hire's restricting SIDs are yours
//     and your groups', so the second check passes wherever the first does, plus the "Smolcorp hire" SID that every
//     hire carries and the SID of that one hire. Neither names an account: they count only where an access list
//     names them, and a deny entry for the hires' SID refuses every hire there. The token holds no privilege but
//     bypassing folder traversal, so SeDebugPrivilege and backup are gone even where you hold them.
//   - Low integrity. Your other programs, the core among them, run at medium: a low process can't read or write
//     their memory, start a thread in them, copy a handle out of them or send their windows input. Without it,
//     a hire could open any program of yours, since it is you. Low has a price: a hire writes only where its
//     folder is labelled low (LetIn), not in your home, your caches or the registry.
//   - Its own access list on every process and thread a hire starts: that hire gets in, no other hire does (the
//     GM is one too), and you, so the core, still do.
//
// And a job with no breakaway: every process a hire starts stays in it, and closing the jail ends them all.
//
// Files: DenyToHires refuses hires a folder or file altogether (the support folder), ReadOnlyToHires lets them read
// one and nothing more (the guard file, a worktree's .claude settings), and LetIn lets one hire into a place inside
// those or makes a place writable for it (its brief and status, its worktree).

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hiresSID is every hire's. S-1-9 is the authority Windows leaves to programs for their own SIDs; the three words
// spell "Smol", "corp" and "hire".
const hiresSID = "S-1-9-1399680876-1668248176-1751741029"

// HiresSID is the SID every hire carries.
func HiresSID() (*windows.SID, error) { return windows.StringToSid(hiresSID) }

// HireSID is one hire's: the hires' SID and four words of its id's SHA-256, the same at every start.
func HireSID(id string) (*windows.SID, error) {
	sum := sha256.Sum256([]byte(id))
	return windows.StringToSid(fmt.Sprintf("%s-%d-%d-%d-%d", hiresSID, binary.BigEndian.Uint32(sum[0:]),
		binary.BigEndian.Uint32(sum[4:]), binary.BigEndian.Uint32(sum[8:]), binary.BigEndian.Uint32(sum[12:])))
}

const (
	fileDeleteChild = 0x40
	fileAll         = 0x1f01ff
	// fileWrites is every way to change a file or folder: its data, a folder's names, attributes, removing or
	// renaming it, its access list, its owner.
	fileWrites = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_EA | fileDeleteChild |
		windows.FILE_WRITE_ATTRIBUTES | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER
	fileReads = windows.FILE_GENERIC_READ | windows.FILE_GENERIC_EXECUTE
	// folderGuard keeps a folder where it is: not removed or renamed, nothing removed or renamed out of it on its
	// say (each file can still allow its own removal), its access list and owner unchanged.
	folderGuard = fileDeleteChild | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER

	disableMaxPrivilege = 0x1
	jobUILimits         = windows.JOB_OBJECT_UILIMIT_HANDLES | windows.JOB_OBJECT_UILIMIT_DESKTOP |
		windows.JOB_OBJECT_UILIMIT_EXITWINDOWS | windows.JOB_OBJECT_UILIMIT_SYSTEMPARAMETERS |
		windows.JOB_OBJECT_UILIMIT_DISPLAYSETTINGS | windows.JOB_OBJECT_UILIMIT_GLOBALATOMS
)

// objectsList is the access list of a hire's processes and threads, in this order on purpose: the hire itself, then
// no other hire, then you and the system. Windows reads it from the top, so the hire's own entry wins for it alone.
// OWNER RIGHTS replaces the owner's implied right to rewrite the list, which every hire would otherwise have: it
// runs as you, and you own them all.
const objectsList = "D:(A;;GA;;;%s)(D;;GA;;;%s)(A;;RC;;;OW)(A;;GA;;;%s)(A;;GA;;;SY)"

var (
	advapi32                  = windows.NewLazySystemDLL("advapi32.dll")
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procCreateRestrictedToken = advapi32.NewProc("CreateRestrictedToken")
	procIsProcessInJob        = kernel32.NewProc("IsProcessInJob")
)

// A Jail runs one hire's programs. Make it with NewJail, start programs with Start, end them all with Close.
type Jail struct {
	ID      string
	sid     *windows.SID
	token   windows.Token
	job     windows.Handle
	objects *windows.SECURITY_DESCRIPTOR
}

// NewJail makes the jail of the hire with this id: its token and its job, with nothing running yet.
func NewJail(id string) (*Jail, error) {
	sid, err := HireSID(id)
	if err != nil {
		return nil, err
	}
	hires, err := HiresSID()
	if err != nil {
		return nil, err
	}
	var own windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_ADJUST_DEFAULT, &own); err != nil {
		return nil, err
	}
	defer own.Close()
	user, err := own.GetTokenUser()
	if err != nil {
		return nil, err
	}
	groups, err := own.GetTokenGroups()
	if err != nil {
		return nil, err
	}
	// Yours and every group you use (your logon session's among them), not one that only denies (an
	// administrator's, unelevated) nor the integrity level; then the hires' and this hire's.
	restricting := []windows.SIDAndAttributes{{Sid: user.User.Sid}}
	for _, group := range groups.AllGroups() {
		if group.Attributes&windows.SE_GROUP_ENABLED != 0 &&
			group.Attributes&(windows.SE_GROUP_USE_FOR_DENY_ONLY|windows.SE_GROUP_INTEGRITY) == 0 {
			restricting = append(restricting, windows.SIDAndAttributes{Sid: group.Sid})
		}
	}
	restricting = append(restricting, windows.SIDAndAttributes{Sid: hires}, windows.SIDAndAttributes{Sid: sid})
	var token windows.Token
	if ok, _, err := procCreateRestrictedToken.Call(uintptr(own), disableMaxPrivilege, 0, 0, 0, 0,
		uintptr(len(restricting)), uintptr(unsafe.Pointer(&restricting[0])), uintptr(unsafe.Pointer(&token))); ok == 0 {
		return nil, fmt.Errorf("sandbox: a hire's token: %w", err)
	}
	runtime.KeepAlive(groups)
	j := &Jail{ID: id, sid: sid, token: token}
	if err := j.setUp(user.User.Sid, hires); err != nil {
		j.Close()
		return nil, err
	}
	return j, nil
}

// setUp lowers the token to low integrity, gives it the hire's access list as the one its processes and threads
// get, and makes the job.
func (j *Jail) setUp(user, hires *windows.SID) error {
	low, err := windows.CreateWellKnownSid(windows.WinLowLabelSid)
	if err != nil {
		return err
	}
	label := windows.Tokenmandatorylabel{Label: windows.SIDAndAttributes{Sid: low, Attributes: windows.SE_GROUP_INTEGRITY}}
	if err := windows.SetTokenInformation(j.token, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&label)), label.Size()); err != nil {
		return fmt.Errorf("sandbox: a hire's integrity level: %w", err)
	}
	j.objects, err = windows.SecurityDescriptorFromString(fmt.Sprintf(objectsList, j.sid, hires, user))
	if err != nil {
		return err
	}
	dacl, _, err := j.objects.DACL()
	if err != nil {
		return err
	}
	byDefault := struct{ dacl *windows.ACL }{dacl}
	if err := windows.SetTokenInformation(j.token, windows.TokenDefaultDacl, (*byte)(unsafe.Pointer(&byDefault)),
		uint32(unsafe.Sizeof(byDefault))); err != nil {
		return fmt.Errorf("sandbox: a hire's processes' access list: %w", err)
	}
	if j.job, err = windows.CreateJobObject(nil, nil); err != nil {
		return err
	}
	// No breakaway allowed, silent or asked for. A crash ends the process instead of waiting on a dialog.
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION
	if _, err := windows.SetInformationJobObject(j.job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	// No window, hook or atom of a program outside the job; no desktop switch, logoff or system setting.
	ui := windows.JOBOBJECT_BASIC_UI_RESTRICTIONS{UIRestrictionsClass: jobUILimits}
	_, err = windows.SetInformationJobObject(j.job, windows.JobObjectBasicUIRestrictions, uintptr(unsafe.Pointer(&ui)), uint32(unsafe.Sizeof(ui)))
	return err
}

// Start starts cmd in the jail: suspended, put in the job, then let run. If it can't be put in the job it is ended
// before it runs a single instruction. A cmd that brings its own token, parent, access lists, handles or breakaway is
// refused.
func (j *Jail) Start(cmd *exec.Cmd) error {
	if j.job == 0 {
		return errors.New("sandbox: the jail is closed")
	}
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	if attr.Token != 0 || attr.ParentProcess != 0 || attr.ProcessAttributes != nil || attr.ThreadAttributes != nil ||
		len(attr.AdditionalInheritedHandles) != 0 || attr.CreationFlags&windows.CREATE_BREAKAWAY_FROM_JOB != 0 {
		return errors.New("sandbox: a hire's program takes its token, parent, access lists, handles and job from its jail")
	}
	objects := &syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})),
		SecurityDescriptor: uintptr(unsafe.Pointer(j.objects))}
	attr.Token = syscall.Token(j.token)
	attr.ProcessAttributes, attr.ThreadAttributes = objects, objects
	attr.CreationFlags |= windows.CREATE_SUSPENDED
	cmd.SysProcAttr = attr
	err := cmd.Start()
	runtime.KeepAlive(j.objects)
	if err != nil {
		return err
	}
	if err := j.adopt(uint32(cmd.Process.Pid)); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("sandbox: %s was ended before it ran: %w", cmd.Path, err)
	}
	return nil
}

// adopt puts the suspended process in the job, then resumes its one thread.
func (j *Jail) adopt(pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(j.job, process); err != nil {
		return err
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	resumed := false
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		if err != nil {
			return err
		}
		resumed = true
	}
	if !resumed {
		return errors.New("its thread wasn't found")
	}
	return nil
}

// Holds says whether the process with this pid runs in the jail's job.
func (j *Jail) Holds(pid uint32) (bool, error) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(process)
	var in int32
	if ok, _, err := procIsProcessInJob.Call(uintptr(process), uintptr(j.job), uintptr(unsafe.Pointer(&in))); ok == 0 {
		return false, err
	}
	return in != 0, nil
}

// Close ends every process in the jail and lets go of its token and job.
func (j *Jail) Close() error {
	var err error
	if j.job != 0 {
		err = windows.TerminateJobObject(j.job, 1)
		windows.CloseHandle(j.job)
		j.job = 0
	}
	if j.token != 0 {
		j.token.Close()
		j.token = 0
	}
	return err
}

// DenyToHires refuses every hire path, and everything in it now or later: no reading, listing, writing, removing,
// renaming, or changing its access list. Its parent is kept from removing or renaming it for a hire. A hire is let
// back into a place inside with LetIn or ReadOnlyToHires: their entries are the place's own, and Windows reads a
// place's own entries before the ones it inherits. The path must not be in a place a hire may write.
func DenyToHires(path string) error {
	hires, err := HiresSID()
	if err != nil {
		return err
	}
	folder, err := plainPlace(path)
	if err != nil {
		return err
	}
	if err := addEntries(path, entry(windows.DENY_ACCESS, fileAll, hires, folder), ownerRights(folder)); err != nil {
		return err
	}
	parent := filepath.Dir(filepath.Clean(path))
	if _, err := plainPlace(parent); err != nil {
		return err
	}
	return addEntries(parent, entry(windows.DENY_ACCESS, fileDeleteChild, hires, false))
}

// ReadOnlyToHires lets every hire read path, and everything in it if it's a folder, and nothing more. within is the
// place a hire may write that holds it (its worktree), or "" if none does: every folder from path's own up to and
// including within is kept from being removed, renamed or having its access list changed by a hire, so the file
// can't be swapped by moving a folder above it. Other files in those folders can still be removed.
func ReadOnlyToHires(path, within string) error {
	hires, err := HiresSID()
	if err != nil {
		return err
	}
	folder, err := plainPlace(path)
	if err != nil {
		return err
	}
	if err := addEntries(path, entry(windows.DENY_ACCESS, fileWrites, hires, folder),
		entry(windows.GRANT_ACCESS, fileReads, hires, folder), ownerRights(false)); err != nil {
		return err
	}
	if within == "" {
		return nil
	}
	within = filepath.Clean(within)
	for above := filepath.Dir(filepath.Clean(path)); ; above = filepath.Dir(above) {
		rel, err := filepath.Rel(within, above)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("sandbox: %s isn't inside %s", path, within)
		}
		if _, err := plainPlace(above); err != nil {
			return err
		}
		if err := addEntries(above, entry(windows.DENY_ACCESS, folderGuard, hires, false), ownerRights(false)); err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
	}
}

// LetIn lets this jail's hire read path (and everything in it, for a folder), or also write it: then the place is
// labelled low, which a hire needs to write anywhere. Only inside a DenyToHires place does this keep other hires out:
// every hire carries your SID in its restricting set, so wherever your own entry lets you in, it lets every hire in.
func (j *Jail) LetIn(path string, write bool) error {
	folder, err := plainPlace(path)
	if err != nil {
		return err
	}
	rights := uint32(fileReads)
	if write {
		rights = fileAll
	}
	if err := addEntries(path, entry(windows.GRANT_ACCESS, rights, j.sid, folder)); err != nil || !write {
		return err
	}
	inherit := ""
	if folder {
		inherit = "OICI"
	}
	label, err := windows.SecurityDescriptorFromString("S:(ML;" + inherit + ";NW;;;LW)")
	if err != nil {
		return err
	}
	sacl, _, err := label.SACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.LABEL_SECURITY_INFORMATION, nil, nil, nil, sacl)
}

// plainPlace says whether path is a folder, and refuses a link or junction: an access list set through one would
// land on whatever it points at.
func plainPlace(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return false, fmt.Errorf("sandbox: %s is a link, not a file or folder", path)
	}
	return info.IsDir(), nil
}

// entry is one entry for sid: inherited by everything in a folder, or the place's alone.
func entry(mode windows.ACCESS_MODE, rights uint32, sid *windows.SID, inherited bool) windows.EXPLICIT_ACCESS {
	inheritance := uint32(windows.NO_INHERITANCE)
	if inherited {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	return windows.EXPLICIT_ACCESS{AccessPermissions: windows.ACCESS_MASK(rights), AccessMode: mode, Inheritance: inheritance,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(sid)}}
}

// ownerRights takes away the owner's implied right to rewrite the access list. A hire runs as you, so it owns what
// you own, and a deny entry doesn't hold against that right; you keep your own entry's full control.
func ownerRights(inherited bool) windows.EXPLICIT_ACCESS {
	owner, _ := windows.CreateWellKnownSid(windows.WinCreatorOwnerRightsSid)
	return entry(windows.GRANT_ACCESS, windows.READ_CONTROL, owner, inherited)
}

// addEntries adds entries to path's access list. SetEntriesInAcl puts denials first, and Windows passes the new
// inherited ones down to what the folder already holds.
func addEntries(path string, entries ...windows.EXPLICIT_ACCESS) error {
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	current, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries(entries, current)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
