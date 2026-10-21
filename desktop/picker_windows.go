package desktop

import (
	"os"
	"os/exec"
	"strings"
)

// folderPicker is Windows' own folder picker, through PowerShell. The start
// folder and the title reach it in its environment, never in its script.
func folderPicker(start, title string) *exec.Cmd {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; Add-Type -AssemblyName System.Windows.Forms; "+
			"$d = New-Object System.Windows.Forms.FolderBrowserDialog; $d.Description = $env:SMOLCORP_PICKER_TITLE; "+
			"$d.SelectedPath = $env:SMOLCORP_PICKER_START; if ($d.ShowDialog() -eq 'OK') { [Console]::Out.Write($d.SelectedPath) }")
	cmd.Env = append(os.Environ(), "SMOLCORP_PICKER_TITLE="+title, "SMOLCORP_PICKER_START="+start)
	return cmd
}

// pickedFolder is what the picker printed: nothing when it was closed without a choice.
func pickedFolder(output []byte, err error) (string, bool, error) {
	if err != nil {
		return "", false, err
	}
	folder := strings.TrimRight(string(output), "\r\n")
	return folder, folder != "", nil
}
