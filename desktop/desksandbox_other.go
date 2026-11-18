//go:build !darwin && !windows

package desktop

import "errors"

// No desk starts in a sandbox here until SA-L2 and SA-L4: deskSandboxed is false, so the engine never makes a desk's
// sandbox nor asks for its starter, and these words are never reached (a start that asked would be refused).

// sandboxNotMade is what a desk's sandbox that can't be made would say here.
const sandboxNotMade = "Smolcorp couldn't put this hire in its sandbox on this computer, so it didn't start it. Update Smolcorp."

// starter refuses: see above.
func (s *deskSandbox) starter(program, smolcorp string) tmuxStarter {
	return func([]string) (string, int, error) {
		return "", 0, errors.New("The sandbox didn't hold on this computer, so the hire wasn't started.")
	}
}
