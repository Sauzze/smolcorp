package desktop

import "golang.org/x/sys/windows"

// unlistPhotos has the photos folder unlisted, or listed again for the app's one look as it starts. On Windows its
// own access list says so, apart from the folders it holds: you (and the system) have full control, inherited by
// every desk's folder and photo, and you are denied "list folder" on the photos folder alone. A folder in it is still
// made, and a photo opened, by its name. Like chmod on a Mac or Linux, an agent that runs as you can still change the
// list back: that is the "anything your user can do" that B leaves open (SANDBOX-PLAN §5.2).
func unlistPhotos(photos string, unlisted bool) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	allow := func(sid *windows.SID, kind windows.TRUSTEE_TYPE) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{AccessPermissions: windows.GENERIC_ALL, AccessMode: windows.GRANT_ACCESS,
			Inheritance: windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT, Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
				TrusteeType: kind, TrusteeValue: windows.TrusteeValueFromSID(sid)}}
	}
	entries := []windows.EXPLICIT_ACCESS{allow(user.User.Sid, windows.TRUSTEE_IS_USER), allow(system, windows.TRUSTEE_IS_WELL_KNOWN_GROUP)}
	if unlisted {
		// Denials come first in the list SetEntriesInAcl makes; this one isn't inherited.
		entries = append(entries, windows.EXPLICIT_ACCESS{AccessPermissions: windows.FILE_LIST_DIRECTORY,
			AccessMode: windows.DENY_ACCESS, Inheritance: windows.NO_INHERITANCE, Trustee: windows.TRUSTEE{
				TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)}})
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(photos, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
