//go:build !darwin

package desktop

// makeCoreWorktrees: off a Mac, hires' worktrees are made beside the repo as before (coreWorktreesOn), so the core's own
// folder for them isn't made.
func makeCoreWorktrees() error { return nil }
