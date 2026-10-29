package desktop

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// Windows Firewall, as the self-test reads it: the rules for Smolcorp's
// port, and whether they let in only what the listening scope says (see
// listen.go). What reads and changes them is firewall_windows.go; what they
// should be is here, so it is tested everywhere.

// firewallRule is an inbound rule for a smolcorp*.exe, as PowerShell's
// Get-NetFirewallRule and its filters give it.
type firewallRule struct {
	Name          string
	DisplayName   string
	Program       string
	Profile       string // "Any", or some of "Domain, Private, Public"
	RemoteAddress string // "Any", "LocalSubnet", or addresses and ranges, a comma between
	Protocol      string // "TCP", "UDP", "Any"
	Action        string // "Allow", "Block"
	Enabled       bool
}

// firewallReady is whether the rules are set, so Windows has nothing to ask
// when the server opens Tailscale's address: until then it listens on this
// computer alone. Always, but on Windows (see firewall_windows.go).
var firewallReady = func() bool { return true }

// The rules Smolcorp sets, by their names in Windows Firewall.
const (
	firewallTailscaleRule = "Smolcorp (Tailscale)"
	firewallHomeRule      = "Smolcorp (home network)"
)

// tailnetRange is Tailscale's range as a rule names it: as set, or as
// Windows reads it back, with a mask.
var tailnetRange = []string{"100.64.0.0/10", "100.64.0.0/255.192.0.0"}

// firewallVerdict is the check for rules, for program and the scope it
// listens in: no rule for another copy, none that lets anyone at all in on a
// public network, and the ones the scope needs.
func firewallVerdict(rules []firewallRule, program, scope string) check {
	c := check{ID: "firewall", Title: "Windows Firewall lets in only your devices"}
	var problems []string
	tailscale, home := false, false
	for _, rule := range rules {
		allows := rule.Enabled && strings.EqualFold(rule.Action, "Allow")
		switch {
		case !samePath(rule.Program, program):
			problems = append(problems, fmt.Sprintf("a rule is for another copy of Smolcorp (%s)", rule.Program))
		case allows && opensToAnyone(rule):
			problems = append(problems, fmt.Sprintf("“%s” lets anyone on a public network in", rule.DisplayName))
		case allows && takesTCP(rule) && slices.ContainsFunc(tailnetRange, func(r string) bool { return strings.EqualFold(rule.RemoteAddress, r) }):
			tailscale = true
		case allows && takesTCP(rule) && strings.EqualFold(rule.RemoteAddress, "LocalSubnet") && hasProfile(rule, "Private"):
			home = true
		}
	}
	if !tailscale {
		problems = append(problems, "there's no rule for Tailscale yet")
	}
	if scope == listenAll && !home {
		problems = append(problems, "there's no rule for your home network yet")
	}
	if len(problems) > 0 {
		c.Status, c.Fix, c.Label = checkFix, fixFirewall, "Set the rules"
		c.Detail = upperFirst(strings.Join(problems, "; ")) + ". Windows asks once for your OK, then nothing else can reach Smolcorp's port."
		return c
	}
	c.Status = checkOK
	c.Detail = "Only your Tailscale devices can open Smolcorp's port."
	if scope == listenAll {
		c.Detail = "Only your Tailscale devices, and your home network's, can open Smolcorp's port."
	}
	return c
}

// opensToAnyone: from any address, on a public network (or every network).
func opensToAnyone(rule firewallRule) bool {
	return (rule.RemoteAddress == "" || strings.EqualFold(rule.RemoteAddress, "Any")) && (hasProfile(rule, "Public") || hasProfile(rule, "Any"))
}

func takesTCP(rule firewallRule) bool {
	return strings.EqualFold(rule.Protocol, "TCP") || strings.EqualFold(rule.Protocol, "Any")
}

func hasProfile(rule firewallRule, profile string) bool {
	for _, part := range strings.Split(rule.Profile, ",") {
		if part = strings.TrimSpace(part); strings.EqualFold(part, profile) || strings.EqualFold(part, "Any") {
			return true
		}
	}
	return false
}

// percentVariable is %NAME%, as a rule's program may be written.
var percentVariable = regexp.MustCompile(`%([A-Za-z0-9_()]+)%`)

// samePath is whether two Windows paths name one file: case aside, and a
// rule's %LOCALAPPDATA% and the like read as this user's.
func samePath(rule, program string) bool {
	expanded := percentVariable.ReplaceAllStringFunc(rule, func(name string) string {
		if value := os.Getenv(strings.Trim(name, "%")); value != "" {
			return value
		}
		return name
	})
	return strings.EqualFold(strings.ReplaceAll(expanded, "/", `\`), strings.ReplaceAll(program, "/", `\`))
}

func upperFirst(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
