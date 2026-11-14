package procmem

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The rows start this test program again as a stand-in for Smolcorp: protected or not, holding a fake secret in its
// environment, until its input ends. Then another process of the same user (this one) tries to get at it.
const (
	childVariable = "PROCMEM_STANDIN"
	fakeSecret    = "FAKE_PROCMEM_SECRET=STANDIN-KEY"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(childVariable); mode != "" {
		os.Exit(standIn(mode))
	}
	os.Exit(m.Run())
}

// standIn protects itself if asked, says so on its output, and waits for its input to end.
func standIn(mode string) int {
	if mode == "protected" {
		if err := NotDumpable(); err != nil {
			println("NotDumpable:", err.Error())
			return 1
		}
		if err := DenyMemoryAccess(); err != nil {
			println("DenyMemoryAccess:", err.Error())
			return 1
		}
	}
	os.Stdout.WriteString("ready\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	return 0
}

// startStandIn starts the stand-in, protected or not, and waits until it has done so. It ends with the test.
func startStandIn(t *testing.T, protected bool) *exec.Cmd {
	t.Helper()
	mode := "open"
	if protected {
		mode = "protected"
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), childVariable+"="+mode, fakeSecret)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		_ = cmd.Wait()
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if strings.TrimSpace(line) != "ready" {
		t.Fatalf("the stand-in didn't start: %q, %v", line, err)
	}
	return cmd
}
