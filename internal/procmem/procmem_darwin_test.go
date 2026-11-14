package procmem

import "testing"

// On a Mac neither call changes anything: the hardened runtime in the signature keeps a debugger and an injected
// library out (desktop/hardening_darwin_test.go). The stand-in still starts and answers, protected or not.
func TestOnAMacTheCallsAreNothing(t *testing.T) {
	if NotDumpable() != nil || DenyMemoryAccess() != nil {
		t.Error("nothing to fail on a Mac")
	}
	startStandIn(t, true)
	startStandIn(t, false)
}
