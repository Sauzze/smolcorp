package sandbox

import "golang.org/x/sys/unix"

// fsType is the file system path is on: "tmpfs" for the private view's support folder (SA-L1), else "".
func (realSystem) fsType(path string) string {
	var st unix.Statfs_t
	if unix.Statfs(path, &st) == nil && st.Type == unix.TMPFS_MAGIC {
		return "tmpfs"
	}
	return ""
}
