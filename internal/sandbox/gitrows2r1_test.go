package sandbox

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

// FXGITROWS2R1 in the Policy, on a made-up Mac (policy_test.go's hirePlaces): the ref folders git removes when empty
// (RT126FIXREVIEW-GITROWS2's B1) and the default branch origin/HEAD names now (N1). Their real-profile rows are in
// gitrows2r1_darwin_test.go and desktop/deskgitrows_test.go.

// B1, in the Policy: on a Mac, every folder below refs/<kind>/ or logs/refs/<kind>/ held above a denied place, and
// otherwise the desk's to write or above a place that is (its own branch), is denied writing itself and kept by a
// keeper no desk writes: refs/heads/hire and its reflogs' folder, and refs/remotes/origin. Not the folders git never
// removes (refs/heads, refs/tags, refs/team, logs/refs/heads), nor one denied whole: since FXGITROWS2R4 the folders of
// the owner checkout's topic/x and of a default branch team/main are, as every branch but the desk's own is, and no
// desk makes them. None of them is in Hold; Linux's view keeps none.
func TestOnAMacTheRefFoldersGitRemovesAreKeptAndNoDesksToMake(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.CheckoutBranch, pl.DefaultBranch = "topic/x", "team/main"
	p := mustNew(t, pl)
	var want []string
	for _, folder := range []string{"logs/refs/heads/hire", "refs/heads/hire", "refs/remotes/origin"} {
		want = append(want, gitDir+"/"+folder+"/.smolcorp-keep")
	}
	if got := p.Keep(); !slices.Equal(got, want) {
		t.Errorf("Keep: %q, want %q", got, want)
	}
	hold := p.Hold()
	for _, keeper := range want {
		folder := filepath.Dir(keeper)
		for at, ops := range map[string][]Op{folder: {Write, Move, Link}, keeper: {Write, Move, Link}, keeper + "/x": {Write}} {
			for _, op := range ops {
				if v, by := p.Decide(at, op); v != Deny {
					t.Errorf("%s: %v %v (by %q)", at, op, v, by.Row)
				}
			}
		}
		if slices.Contains(hold, folder) || slices.Contains(hold, keeper) {
			t.Errorf("%s is in Hold", folder)
		}
	}
	for _, at := range []string{gitDir + "/refs/heads/hire/h1/work", gitDir + "/refs/heads/hire/h1", gitDir + "/logs/refs/heads/hire/h1/work",
		gitDir + "/refs/remotes/origin/main"} {
		if v, by := p.Decide(at, Write); v != Allow {
			t.Errorf("%s: Write %v (by %q), want the desk's", at, v, by.Row)
		}
	}
	for _, not := range []string{"refs/heads", "refs/tags", "refs/remotes", "refs/team", "logs/refs/heads", "refs/heads/release",
		"refs/heads/topic", "refs/heads/team", "refs/tags/team", "logs/refs/heads/topic"} {
		if slices.Contains(p.Keep(), gitDir+"/"+not+"/.smolcorp-keep") {
			t.Errorf("%s is kept", not)
		}
	}
	pl.GOOS = "linux"
	if got := mustBuild(t, pl).Keep(); len(got) != 0 {
		t.Errorf("Linux's view keeps %q", got)
	}
}

// N1, in the Policy: the branch origin/HEAD names at the start (DefaultBranchNow) is a base branch as the pinned one
// is, its reflog and its names elsewhere too, and a name a path or a pattern reads is refused.
func TestTheDefaultBranchOriginHEADNamesNowIsABaseBranchBesideThePinnedOne(t *testing.T) {
	pl := hirePlaces(realSupport)
	pl.DefaultBranchNow = "trunk"
	p := mustNew(t, pl)
	for _, at := range []string{"refs/heads/trunk", "refs/heads/trunk.lock", "refs/heads/develop", "logs/refs/heads/trunk", "refs/tags/trunk", "trunk"} {
		if v, by := p.Decide(gitDir+"/"+at, Write); v != Deny {
			t.Errorf("%s: Write %v (by %q)", at, v, by.Row)
		}
	}
	for _, bad := range []string{"../x", "t*", ".hidden", "a//b"} {
		pl.DefaultBranchNow = bad
		if _, err := New(pl); !errors.Is(err, ErrBadName) {
			t.Errorf("DefaultBranchNow %q: %v, want ErrBadName", bad, err)
		}
	}
}
