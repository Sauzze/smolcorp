//go:build !darwin && !windows

package desktop

import "errors"

// attachInSandbox: no desk starts in a sandbox on this computer yet (desksandbox_other.go), so none is shown in one.
func attachInSandbox(Hire, string, int, []string) (attaching, error) {
	return attaching{}, errors.New("this desk was started in a sandbox, which this copy of Smolcorp can't show on this computer")
}
