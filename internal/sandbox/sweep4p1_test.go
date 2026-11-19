package sandbox

import "testing"

// FXRC2FIXB (RT126SWEEP4-P1 F1): no desk makes a .git, or a HEAD that makes a bare repo, above the core's folder for
// hires' worktrees (~/.smolcorp) or above a GM's hires' folders (a configured folder, <top>-hires), which git run
// there, or in a repo's folder between it and a hire's tree, takes for its repo. The GM gets the rows a hire made there
// already had; an older hire gets them too. As data, on the made-up Mac of policy_test.go; sweep4p1_darwin_test.go runs
// it under the real profile.

// dev is above a configured folder for hires' worktrees, dev/wt.
const dev = home + "/dev"

func TestNoDeskMakesAGitFolderAboveTheFoldersForHiresWorktrees(t *testing.T) {
	gm := projectsGMPlaces()
	gm.GMHires = []string{dev + "/wt"}
	cases := append(gitAbove(home+"/.smolcorp"), gitAbove(dev)...)
	cases = append(cases, projectsCase{home + "/.smolcorp/other", Write, Allow, ""}, projectsCase{dev + "/notes.md", Write, Allow, ""})
	decideEach(t, "the GM in the projects folder", mustNew(t, gm), cases)

	hire := projectsHirePlaces() // its tree in the repo's folder for hires, made before the core's folder
	hire.Worktrees = append(hire.Worktrees, dev+"/wt")
	decideEach(t, "an older hire", mustNew(t, hire), append(gitAbove(home+"/.smolcorp"), gitAbove(dev)...))
	decideEach(t, "a hire in the core's folder", mustNew(t, coreHirePlaces()), gitAbove(home+"/.smolcorp"))
}
