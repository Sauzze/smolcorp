package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// The Mac app's report.sh (SA-X11) runs from the app, where it is shipped (Contents/Resources/report.sh): signed with
// it, current with it, and written by no desk (SA-X2 denies every desk the app's install). Nothing copies it out any
// more. The Swift engine did, into the support folder, with the port filled in, and connected every Claude account's
// settings to that copy; the core's refresh puts `smolcorp hook` in place of those entries in ~/.claude only, so an
// account of the owner's connected before can still run the copy, frozen as that version left it.
//
// At each start under the Mac app, such a copy, when there is one, becomes a pointer (a shebang, two comment lines and
// an exec) that runs the app's own script with this core's port and support folder: the code those sessions run is
// the app's, and it sends a desk's own key in a desk (SA-X10) as `smolcorp hook` does, where the frozen copy sent the
// owner's sessions' key, which reaches no desk. With no copy there, nothing runs one, and none is made. A test copy
// leaves it alone, as it leaves the agents' settings (leavesAgentsAlone). A temp file a start that stopped halfway
// left beside it (.report.sh-<digits>) is removed at the next.

// reportScriptName is the script's name in the app's Resources, and the copy's in the support folder.
const reportScriptName = "report.sh"

// reportScriptTemp is the pattern of the temp file the pointer is written to before it is renamed into place, and
// leftTemp the names os.CreateTemp makes of it.
const reportScriptTemp = ".report.sh-*"

var leftTemp = regexp.MustCompile(`^\.report\.sh-[0-9]+$`)

// appReportScript is the report.sh shipped in the app this core runs from (Contents/Resources, beside bin/), or "".
func appReportScript() string {
	program := commandLineProgram()
	if program == "" {
		return ""
	}
	script := filepath.Join(filepath.Dir(filepath.Dir(program)), reportScriptName)
	if info, err := os.Stat(script); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return script
}

// reportScriptPointer is what the support folder's copy becomes: it runs script, the app's, with port and support.
func reportScriptPointer(script string, port int, support string) string {
	return fmt.Sprintf("#!/bin/bash\n"+
		"# Smolcorp's hook, for the agents an earlier version connected to this file: it runs the app's own,\n"+
		"# rewritten at each start to wherever the app is (desktop/reportscript.go).\n"+
		"exec /bin/bash %s --port %d --support %s \"$@\"\n", shellQuoted(script), port, shellQuoted(support))
}

// pointReportScriptAtTheApp turns the support folder's copy of report.sh, if there is one, into the pointer to the
// app's (see the top). It is replaced whole, never written through: a link put there is replaced, not followed.
func pointReportScriptAtTheApp() {
	if leavesAgentsAlone() {
		return
	}
	// The temp files of a start that stopped between writing one and renaming it into place: nothing else uses them.
	if left, err := filepath.Glob(filepath.Join(supportDir, reportScriptTemp)); err == nil {
		for _, path := range left {
			if leftTemp.MatchString(filepath.Base(path)) {
				_ = os.Remove(path)
			}
		}
	}
	copied := filepath.Join(supportDir, reportScriptName)
	if info, err := os.Lstat(copied); err != nil || info.IsDir() {
		return
	}
	script := appReportScript()
	if script == "" {
		return
	}
	pointer := reportScriptPointer(script, Port, supportDir)
	if info, err := os.Lstat(copied); err == nil && info.Mode().IsRegular() && info.Mode().Perm() == 0o700 {
		if saved, err := os.ReadFile(copied); err == nil && string(saved) == pointer {
			return
		}
	}
	temp, err := os.CreateTemp(supportDir, reportScriptTemp)
	if err != nil {
		return
	}
	_, err = temp.WriteString(pointer)
	if err == nil {
		err = temp.Chmod(0o700)
	}
	if closed := temp.Close(); err == nil {
		err = closed
	}
	if err == nil {
		err = os.Rename(temp.Name(), copied)
	}
	if err != nil {
		_ = os.Remove(temp.Name())
	}
}
