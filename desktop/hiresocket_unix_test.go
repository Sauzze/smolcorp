//go:build !windows

package desktop

// A hire's tmux socket goes in the test's own support folder (whichever it is at the time), never in tmux's
// real one under /tmp: set before TestMain runs. A test of the folder itself points it elsewhere.
func init() { hireSocketFolder = func() string { return supportDir } }
