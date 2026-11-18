package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"
)

// remoteFilesTree is a shared .git (no config, so no git runs) with old remote files from before the sandbox: the
// repo's, a nested name's, a link's and a submodule's; and one in a worktree's registration, which git never reads.
func remoteFilesTree(t *testing.T) (gitDir, outside string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitDir, outside = filepath.Join(base, ".git"), filepath.Join(base, "elsewhere")
	for rel, text := range map[string]string{
		"branches/up1": "standin::x\n", "remotes/up2": "URL: standin::y\n", "remotes/team/up3": "URL: standin::z\n",
		"modules/sub/HEAD": "1111\n", "modules/sub/remotes/origin": "URL: standin::s\n",
		"worktrees/h1/remotes/up4": "URL: standin::w\n", "HEAD": "ref: refs/heads/main\n", "hooks/pre-commit.sample": "#!/bin/sh\n",
	} {
		at := filepath.Join(gitDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(outside, []byte("URL: standin::l\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(gitDir, "remotes", "linked")); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		t.Logf("no symbolic links here (%v): the linked row isn't made", err)
	}
	return gitDir, outside
}

func remoteFilesListed(plants []oldGitPlant, gitDir string) []string {
	var files []string
	for _, p := range plants {
		if p.Why == whyRemote {
			rel, _ := filepath.Rel(gitDir, p.File)
			files = append(files, filepath.ToSlash(rel))
		}
	}
	slices.Sort(files)
	return files
}

// TestTheOldRemoteFilesAreListed: every file or link in the shared .git's branches/ and remotes/, and a submodule's,
// at any depth, is listed with "Remove"; a worktree's registration's, which git doesn't read, isn't, nor anything
// that isn't one of them. Listing removes nothing.
func TestTheOldRemoteFilesAreListed(t *testing.T) {
	gitDir, _ := remoteFilesTree(t)
	sums := treeSums(t, gitDir)
	want := []string{"branches/up1", "modules/sub/remotes/origin", "remotes/team/up3", "remotes/up2"}
	if _, err := os.Lstat(filepath.Join(gitDir, "remotes", "linked")); err == nil {
		want = append(want, "remotes/linked")
		slices.Sort(want)
	}
	plants := findOldGitPlants(context.Background(), gitDir)
	if got := remoteFilesListed(plants, gitDir); !slices.Equal(got, want) {
		t.Fatalf("listed %v, want %v (all: %+v)", got, want, plants)
	}
	if len(plants) != len(want) {
		t.Errorf("listed more than the remote files: %+v", plants)
	}
	if now := treeSums(t, gitDir); !slices.Equal(now, sums) {
		t.Error("listing changed a file")
	}
	// None there, nothing listed.
	empty := filepath.Join(t.TempDir(), ".git")
	if err := os.MkdirAll(filepath.Join(empty, "remotes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := findOldGitPlants(context.Background(), empty); len(got) != 0 {
		t.Errorf("an empty remotes folder listed %+v", got)
	}
}

func TestAnOldRemoteFolderThatCantBeListedIsListedAsSuch(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("no folder modes to shut here")
	}
	gitDir, _ := remoteFilesTree(t)
	shut := filepath.Join(gitDir, "remotes", "team")
	if err := os.Chmod(shut, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(shut, 0o700)
	plants := findOldGitPlants(context.Background(), gitDir)
	if !slices.ContainsFunc(plants, func(p oldGitPlant) bool { return p.File == shut && p.Why == whyUnread }) {
		t.Fatalf("listed %+v, want %s as unread", plants, shut)
	}
}

// TestRemoveTakesOutAnOldRemoteFileOnlyAsItWasListed: Remove takes out a listed file, and a link (not what it leads
// to); a file changed since it was listed, or an ID that wasn't listed, removes nothing.
func TestRemoveTakesOutAnOldRemoteFileOnlyAsItWasListed(t *testing.T) {
	ctx := context.Background()
	gitDir, outside := remoteFilesTree(t)
	listed := findOldGitPlants(ctx, gitDir)
	id := func(rel string) string {
		at := filepath.Join(gitDir, filepath.FromSlash(rel))
		i := slices.IndexFunc(listed, func(p oldGitPlant) bool { return p.File == at })
		if i < 0 {
			t.Fatalf("%s isn't listed", rel)
		}
		return listed[i].ID
	}
	gone := func(rel string) bool {
		_, err := os.Lstat(filepath.Join(gitDir, filepath.FromSlash(rel)))
		return errors.Is(err, os.ErrNotExist)
	}
	if err := removeOldGitPlant(ctx, listed, id("branches/up1")); err != nil || !gone("branches/up1") {
		t.Errorf("Remove branches/up1: %v, gone %v", err, gone("branches/up1"))
	}
	// Changed since it was listed: the owner looks again.
	changed := filepath.Join(gitDir, "remotes", "up2")
	if err := os.WriteFile(changed, []byte("URL: standin::changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(changed, later, later); err != nil {
		t.Fatal(err)
	}
	if err := removeOldGitPlant(ctx, listed, id("remotes/up2")); !errors.Is(err, errGitPlantGone) || gone("remotes/up2") {
		t.Errorf("Remove of a changed file: %v, gone %v", err, gone("remotes/up2"))
	}
	if err := removeOldGitPlant(ctx, listed, "0123456789abcdef"); !errors.Is(err, errGitPlantGone) {
		t.Errorf("Remove of an ID not listed: %v", err)
	}
	if !gone("remotes/linked") {
		if err := removeOldGitPlant(ctx, listed, id("remotes/linked")); err != nil || !gone("remotes/linked") {
			t.Errorf("Remove of a link: %v, gone %v", err, gone("remotes/linked"))
		}
		if _, err := os.Stat(outside); err != nil {
			t.Errorf("Remove took what the link leads to: %v", err)
		}
	}
	if gone("modules/sub/remotes/origin") || gone("remotes/team/up3") {
		t.Error("Remove took a file it wasn't asked to")
	}
}
