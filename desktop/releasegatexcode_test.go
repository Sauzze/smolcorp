//go:build unix

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The caller's choice of Xcode never reaches a git the release gate runs (FXGOPINR2). On a Mac, /usr/bin/git is
// xcrun's stand-in, and with DEVELOPER_DIR naming a folder with no Xcode in it, it fails: git said nothing, and the
// gate stopped at "git here looks at another folder (none)" before any check of the release (the Mac Studio's
// release-rows, 17 Nov). Here a stand-in git first on PATH fails as xcrun does whenever DEVELOPER_DIR names a folder
// without usr/bin/xcrun, on any system, and otherwise runs the real git, noting which of the caller's Xcode settings
// it was given. check-public.sh runs git too, as the real one does. Only the gate runs, with the stand-in go, so
// nothing is built: the release commit, with each of those settings the caller's, goes past every git check and
// check-public.sh to the forged-event table's go, and no git it ran saw any of them.
func TestReleaseGateRunsGitWithoutTheCallersXcode(t *testing.T) {
	heavyReleaseRows(t)
	t.Run("DEVELOPER_DIR, SDKROOT and TOOLCHAINS the caller's", func(t *testing.T) {
		realGit, err := exec.LookPath("git")
		if err != nil {
			t.Skip("no git")
		}
		r := newReleaseRepo(t)
		r.script = "scripts/release-gate.sh"
		r.write("scripts/check-public.sh", "#!/bin/sh\ngit ls-files >/dev/null || exit 1\necho '"+checkPublicRan+"'\n")
		r.git("commit", "-q", "-am", "check-public.sh runs git")
		r.squashed()

		log := filepath.Join(r.bin, "git.log")
		shim := filepath.Join(r.bin, "git")
		writeReleaseFile(t, shim, "#!/bin/sh\n"+
			"echo \"DEVELOPER_DIR=${DEVELOPER_DIR-unset} SDKROOT=${SDKROOT-unset} TOOLCHAINS=${TOOLCHAINS-unset} $1\" >> '"+log+"'\n"+
			"if [ -n \"${DEVELOPER_DIR:-}\" ] && [ ! -x \"$DEVELOPER_DIR/usr/bin/xcrun\" ]; then\n"+
			"  echo \"xcrun: error: invalid DEVELOPER_DIR path ($DEVELOPER_DIR), missing xcrun at: $DEVELOPER_DIR/usr/bin/xcrun\" >&2\n"+
			"  exit 1\n"+
			"fi\n"+
			"exec '"+realGit+"' \"$@\"\n", 0o755)
		xcode := []string{"DEVELOPER_DIR=" + t.TempDir(), "SDKROOT=" + t.TempDir(), "TOOLCHAINS=example.invalid.none"}

		// The stand-in fails as xcrun's does with those settings, so a git given them can't pass.
		control := exec.Command(shim, "rev-parse", "--show-toplevel")
		control.Dir, control.Env = r.dir, append(append([]string{}, r.env...), xcode...)
		said, err := control.CombinedOutput()
		if err == nil || !strings.Contains(string(said), "xcrun: error: invalid DEVELOPER_DIR path") {
			t.Fatalf("the stand-in git with the caller's DEVELOPER_DIR: want xcrun's error, got %v:\n%s", err, said)
		}
		if err := os.Remove(log); err != nil {
			t.Fatal(err)
		}

		status, out := r.release(xcode...)
		if status != 97 || !strings.Contains(out, checkPublicRan) || !strings.Contains(out, goReached) ||
			strings.Contains(out, "Stopped: ") || strings.Contains(out, "xcrun: error") {
			t.Errorf("want the gate past git's checks and check-public.sh to the forged-event table's go, got status %d:\n%s",
				status, out)
		}
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatalf("the gate ran no git through the stand-in: %v", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if !strings.HasPrefix(line, "DEVELOPER_DIR=unset SDKROOT=unset TOOLCHAINS=unset ") {
				t.Errorf("a git the gate ran had the caller's Xcode settings: %s", line)
			}
		}
	})
}
