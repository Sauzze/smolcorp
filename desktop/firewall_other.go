//go:build !windows

package desktop

// firewallCheck: only Windows has a firewall Smolcorp sets up.
func firewallCheck(string) *check { return nil }

// fixFirewallFor: nothing to set here.
func fixFirewallFor(string) error { return nil }

// lookAtFirewall: nothing to look at here.
func lookAtFirewall(string) {}
