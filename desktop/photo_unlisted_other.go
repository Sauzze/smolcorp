//go:build !windows

package desktop

import "os"

// unlistPhotos has the photos folder unlisted (0300: you can make a folder in it and open one by its name, but not
// list what's there), or listed again for the app's one look as it starts (0700). An agent that runs as you can
// still chmod it back: that is the "anything your user can do" that B leaves open (SANDBOX-PLAN §5.2), not a tool's
// ordinary Glob or ls.
func unlistPhotos(photos string, unlisted bool) error {
	if unlisted {
		return os.Chmod(photos, 0o300)
	}
	return os.Chmod(photos, 0o700)
}
