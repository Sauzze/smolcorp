//go:build windows

package desktop

// configuredWorktreesWords: no desk starts in a sandbox on Windows yet, so no General Manager is held from any folder,
// and the configured folder for hires' worktrees is honoured as before (FXCOREWORKTREES126).
func configuredWorktreesWords(string) string { return "" }

// resolvesToItself: off a Mac a hire's row keeps its paths as they were given, and no desk runs in a sandbox, so a path
// always does (as hireTreeMoved was).
func resolvesToItself(string) bool { return true }
