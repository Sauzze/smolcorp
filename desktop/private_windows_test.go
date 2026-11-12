package desktop

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// yoursAlone says why path isn't a file only you can read, nil when it is. Windows has no 0600: a file is yours
// alone when its access list lets in nobody but you, the system and the computer's administrators, as a file in
// your own profile (the support folder in AppData, a temp folder) has it from its folder.
func yoursAlone(path string) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if dacl == nil {
		return errors.New(filepath.Base(path) + " has no access list: anyone can read it")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	allowed := []*windows.SID{user.User.Sid}
	for _, known := range []windows.WELL_KNOWN_SID_TYPE{windows.WinLocalSystemSid, windows.WinBuiltinAdministratorsSid} {
		sid, err := windows.CreateWellKnownSid(known)
		if err != nil {
			return err
		}
		allowed = append(allowed, sid)
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Mask == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		yours := false
		for _, a := range allowed {
			yours = yours || windows.EqualSid(sid, a)
		}
		if !yours {
			return fmt.Errorf("%s lets %s in", filepath.Base(path), sid.String())
		}
	}
	return nil
}
