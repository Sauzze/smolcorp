package desktop

import "testing"

// On Windows no desk runs in tmux, in a sandbox or not (FXDESKCASE, RT126DESKCASE): no socket is a desk's, by name or
// by place, and a tmux tab's guard is always tmuxUnchecked, never a desk's server.
func TestOnWindowsNoSocketIsADesksPlace(t *testing.T) {
	for _, socket := range []string{`C:\Users\x\AppData\Local\Temp\smolcorp-1\ab12\s`, "/tmp/smolcorp-1/ab12/s", `ab12\s`, "ab12/s", ""} {
		if deskShapedSocket(socket) {
			t.Errorf("%q is shaped as a desk's", socket)
		}
		if id := deskSocketID(socket, 0); id != "" {
			t.Errorf("deskSocketID(%q) = %q, want none", socket, id)
		}
		if _, guard := tmuxFor(socket); guard != tmuxUnchecked {
			t.Errorf("tmuxFor(%q) told %q, want %q", socket, guard, tmuxUnchecked)
		}
	}
}
