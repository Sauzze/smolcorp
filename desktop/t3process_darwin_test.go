package desktop

import (
	"os/exec"
	"reflect"
	"slices"
	"testing"
)

// On a Mac this engine reads a process's arguments from the kernel (kern.procargs2), as the Mac app does, but
// not its folder: a process given the folder with --add-dir is found, and the answer is still "can't look".
func TestT3AProcesssArgumentsAreReadFromTheKernel(t *testing.T) {
	sleeper := exec.Command("/bin/sleep", "30")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sleeper.Process.Kill(); _ = sleeper.Wait() })
	if got := processArguments(sleeper.Process.Pid); !reflect.DeepEqual(got, []string{"/bin/sleep", "30"}) {
		t.Errorf("%q", got)
	}
	if processArguments(-1) != nil {
		t.Error("no such process has arguments")
	}
	wt := t.TempDir()
	// The shell stays (it has more to run after sleep), its arguments holding --add-dir and the folder.
	adder := exec.Command("/bin/sh", "-c", "/bin/sleep 30; :", "sh", "--add-dir", wt)
	if err := adder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adder.Process.Kill(); _ = adder.Wait() })
	found, looked := processesWorkingIn(wt)
	if looked || !slices.Contains(found, adder.Process.Pid) {
		t.Errorf("found %v, looked %v", found, looked)
	}
}
