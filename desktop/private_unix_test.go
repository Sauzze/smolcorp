//go:build !windows

package desktop

import (
	"fmt"
	"os"
)

// yoursAlone says why path isn't a file only you can read, nil when it is: mode 0600.
func yoursAlone(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o600 {
		return fmt.Errorf("%s is %v, want 0600", info.Name(), info.Mode().Perm())
	}
	return nil
}
