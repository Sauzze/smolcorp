//go:build unix

package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// standInGitOnPath puts a stand-in git first on PATH: it writes the environment it was given to a file, one line each,
// and answers as git does in a folder in no repo. It gives that file's path (not there until it runs).
func standInGitOnPath(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	said := filepath.Join(bin, "env.txt")
	script := "#!/bin/sh\nenv > '" + said + "'\necho 'fatal: not a git repository (or any of the parent directories): .git' >&2\nexit 128\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return said
}

// RT126STACKFIX5R1 O1 and O3: the launcher's refusal's own read (gitFindsNoRepo) gives git
// GIT_DISCOVERY_ACROSS_FILESYSTEM=1, whatever the terminal says, so a folder on another file system's mount inside a
// checkout is found in it; and none of the terminal's GIT_TRACE settings, whose lines before git's own words would
// turn a folder in no repo into a refusal; nor its ceiling, and git's words in English (LC_ALL=C). A stand-in git
// records the environment it was given: a mount can't be made here.
func TestTheLaunchersRefusalReadsGitAcrossMountsWithNoTraceOrCeiling(t *testing.T) {
	isolate(t)
	said := standInGitOnPath(t)
	for _, kv := range [][2]string{{"GIT_DISCOVERY_ACROSS_FILESYSTEM", "0"}, {"GIT_CEILING_DIRECTORIES", "/"}, {"GIT_TRACE", "1"},
		{"GIT_TRACE2", "1"}, {"GIT_TRACE_SETUP", "true"}, {"GIT_TRACE2_EVENT", "2"}, {"LC_ALL", "fr_FR.UTF-8"}} {
		t.Setenv(kv[0], kv[1])
	}
	folder := t.TempDir()
	if !gitFindsNoRepo(folder) {
		t.Fatal("the stand-in git's no-repo answer: taken as a repo")
	}
	data, err := os.ReadFile(said)
	if err != nil {
		t.Fatalf("the stand-in git didn't run: %v", err)
	}
	env := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, want := range []string{"GIT_DISCOVERY_ACROSS_FILESYSTEM=1", "LC_ALL=C"} {
		name, _, _ := strings.Cut(want, "=")
		var got []string
		for _, kv := range env {
			if strings.HasPrefix(kv, name+"=") {
				got = append(got, kv)
			}
		}
		if !slices.Equal(got, []string{want}) {
			t.Errorf("git was given %q, want only %s", got, want)
		}
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_TRACE") || strings.HasPrefix(kv, "GIT_CEILING_DIRECTORIES=") {
			t.Errorf("git was given the terminal's %s", kv)
		}
	}
}

// RT126STACKFIX5R1 O2: in a hire's own worktree, the refusal's read answers "a repo" without running git outside its
// sandbox there, so a plain hire made with the switch off (no desk of its own) is refused there when the switch is on
// again. The same folder, held by no hire, is what git says it is: the stand-in git's "no repo", not refused.
func TestTheLaunchersRefusalTakesAHiresOwnWorktreeForARepoWithoutRunningGit(t *testing.T) {
	isolate(t)
	said := standInGitOnPath(t)
	repo, worktree := physical(t.TempDir()), physical(t.TempDir())
	sub := filepath.Join(worktree, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	run := launcherRun{hire: strings.Repeat("AB", 16), sandboxOff: true}
	if why := run.sandboxOnAgainWords(true, sub); why != "" {
		t.Fatalf("held by no hire, in the stand-in git's no repo: %q, want it not refused", why)
	}
	if err := os.Remove(said); err != nil {
		t.Fatal(err)
	}
	hire := Hire{ID: "plain-0123456789ab", Name: "Ada", Folder: keptPath(worktree), Worktree: keptPath(worktree), Repo: repo, Branch: "hire/plain-0123456789ab/ada-012345",
		State: "working", Started: 1761000000000}
	data, _ := json.Marshal(map[string]any{"version": 1, "hires": []Hire{hire}})
	if err := writeRegistryFile(registryPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{worktree, sub} {
		if gitFindsNoRepo(folder) {
			t.Errorf("%s, in a hire's own worktree: taken for no repo", folder)
		}
		if why := run.sandboxOnAgainWords(true, folder); why != plainHireSandboxOnAgain {
			t.Errorf("%s, in a hire's own worktree: %q, want it refused", folder, why)
		}
	}
	if _, err := os.Stat(said); err == nil {
		t.Error("git was run in a hire's own worktree, outside its sandbox")
	}
}
