//go:build windows

package desktop

import (
	"context"
	"path/filepath"
	"testing"
)

// An npm .cmd shim goes through cmd.exe with launcher.go's command line, and
// keeps its own process group and hidden window.
func TestASummaryRunOfACmdShimUsesTheLaunchersCommandLine(t *testing.T) {
	shim := filepath.Join(t.TempDir(), "opencode.cmd")
	cmd := summaryCommand(context.Background(), shim, []string{"run", "--format", "json", `a "quoted" & piped|arg`})
	ownGroup(cmd)
	shell := cmd.Path
	if want := cmdLine(shell, shim, []string{"run", "--format", "json", `a "quoted" & piped|arg`}); cmd.SysProcAttr.CmdLine != want {
		t.Errorf("command line:\n%s\nwant\n%s", cmd.SysProcAttr.CmdLine, want)
	}
	if cmd.SysProcAttr.CreationFlags&createNewProcessGroup == 0 || cmd.SysProcAttr.CreationFlags&createSuspended == 0 || !cmd.SysProcAttr.HideWindow {
		t.Error("its own group and no window were lost")
	}
}
