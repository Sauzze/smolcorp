//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/Sauzze/smolcorp/internal/sandbox"
)

// deskWorktrees are the folders hires' worktrees are made in that a desk's Policy holds (Places.Worktrees): on a Mac,
// the core's own, and the configured one when it's honoured (worktrees). Every desk writes only its own place there;
// a GM, none of it.
func deskWorktrees(worktrees string) []string {
	if !coreWorktreesOn() {
		return nil
	}
	folders := []string{coreWorktreesFolder()}
	if filepath.IsAbs(worktrees) && configuredWorktreesWords(worktrees) == "" {
		folders = append(folders, physicalBelow(worktrees))
	}
	return folders
}

// configuredWorktreesWords say why the configured folder for hires' worktrees (config.json's hireWorktrees) isn't
// honoured on a Mac: it must be outside every General Manager's write reach. Every GM's Policy denies writing in it and
// moving the folders above it (deskWorktrees), so what's left is where that can't hold: a link in the home folder on
// its way (a GM may change it), or a folder that holds the home folder, a support folder, the folder for hires or a GM's
// own place, or is inside a support folder or a GM's own place. "" when it's honoured, and off a Mac, where no GM is held
// from any folder yet.
func configuredWorktreesWords(worktrees string) string {
	if !coreWorktreesOn() || worktrees == "" {
		return ""
	}
	refused := "The folder set for hires' worktrees (hireWorktrees in config.json), " + shownPath(worktrees) + ", "
	fix := " Take hireWorktrees out of config.json, so hires' worktrees go in ~/.smolcorp/worktrees, or set a folder no General Manager can change."
	if !filepath.IsAbs(worktrees) {
		return refused + "isn't a whole path." + fix
	}
	home := physical(homeDir)
	for at := filepath.Clean(worktrees); filepath.Dir(at) != at; at = filepath.Dir(at) {
		if info, err := os.Lstat(at); err == nil && info.Mode()&os.ModeSymlink != 0 && sandbox.Inside(runtime.GOOS, physicalBelow(filepath.Dir(at)), home) {
			return refused + "is reached through a link in your home folder, " + shownPath(at) + ", which a General Manager could change." + fix
		}
	}
	real := physicalBelow(worktrees)
	holds := func(place string) bool { return place != "" && sandbox.Inside(runtime.GOOS, place, real) }
	in := func(place string) bool { return place != "" && sandbox.Inside(runtime.GOOS, real, place) }
	switch {
	case holds(home):
		return refused + "holds your home folder." + fix
	case holds(physical(supportDir)) || in(physical(supportDir)):
		return refused + "is in Smolcorp's own support folder, or holds it." + fix
	case holds(hireProjectsFolder()):
		return refused + "holds the folder for hires, where your projects are." + fix
	}
	for _, h := range readRegistry(registryPath(), false).hires {
		if h.Role != gmRole || !isOpen(h) {
			continue
		}
		own, _ := gmOwnPlaces(h)
		for _, place := range own {
			if holds(place) || in(place) {
				return refused + "is in a General Manager's own place, or holds one, so it could write there." + fix
			}
		}
	}
	return ""
}

// gmWorktreesWords say why GM d doesn't start: on a Mac, one of its own places is in a folder hires' worktrees are made
// in (deskWorktrees), which every GM's Policy denies it whole (FXCOREWORKTREES126). "" when none is.
func gmWorktreesWords(d deskAt, pl sandbox.Places) string {
	if !d.GM {
		return ""
	}
	for _, folder := range pl.Worktrees {
		for _, place := range []string{pl.Root, pl.Checkout, pl.GMProjects, pl.GMPlans} {
			if place != "" && sandbox.Inside(runtime.GOOS, place, folder) {
				return "This General Manager's own place, " + shownPath(place) + ", is in " + shownPath(folder) +
					", the folder for hires' worktrees, which no General Manager may write, so it didn't start. Fire it, then hire it again into another folder."
			}
		}
	}
	return ""
}

// resolvesToItself is whether path, a whole path a row keeps resolved, still resolves to that very spelling: no link at
// it or on its way. A part of it that isn't there (a tree since removed) is judged by the folders above it, which must
// resolve to themselves too (physicalBelow), so a gone tree behind a turned folder is still turned. Only on a Mac,
// where rows keep paths resolved and desks run in a sandbox: elsewhere a path always does (as hireTreeMoved was).
func resolvesToItself(path string) bool {
	if runtime.GOOS != "darwin" {
		return true
	}
	return filepath.IsAbs(path) && path == filepath.Clean(path) && physicalBelow(path) == path
}
