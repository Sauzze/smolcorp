package sandbox

import (
	"os"

	"golang.org/x/sys/unix"
)

// OwnersMounts are the file systems mounted by this user, not root, where and of which type, as the kernel names them:
// what a desk running as the owner could have mounted (FXSWEEP3P1). The system's own volumes, the disk and /dev are
// root's.
func OwnersMounts() ([]Mount, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	buf := make([]unix.Statfs_t, n+8) // room for a mount made in between
	if n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT); err != nil {
		return nil, err
	}
	uid := uint32(os.Getuid())
	var mounts []Mount
	for _, fs := range buf[:n] {
		if fs.Owner == uid {
			mounts = append(mounts, Mount{At: unix.ByteSliceToString(fs.Mntonname[:]), Type: unix.ByteSliceToString(fs.Fstypename[:])})
		}
	}
	return mounts, nil
}
