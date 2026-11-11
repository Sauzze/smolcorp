package desktop

import (
	"os/exec"
	"testing"
)

// On a Mac this engine can't read a process's folder: even a process given the folder with --add-dir isn't
// looked for, and the answer is "can't look", so Let go keeps the fence.
func TestT3OnAMacTheGoCoreCantLook(t *testing.T) {
	wt := t.TempDir()
	adder := exec.Command("/bin/sh", "-c", "/bin/sleep 30; :", "sh", "--add-dir", wt)
	if err := adder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adder.Process.Kill(); _ = adder.Wait() })
	if found, looked := processesWorkingIn(wt); looked || found != nil {
		t.Errorf("found %v, looked %v", found, looked)
	}
}
