//go:build windows

package desktop

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// The firewall script setup runs, with your OK, to set the rules.
//
//go:embed firewall.ps1
var firewallScript string

// firewallSet is whether the rules were found set, or set: see firewallReady.
var firewallSet atomic.Bool

func init() {
	firewallReady = firewallSet.Load
}

// The rules as last read, for a minute: PowerShell takes a few seconds.
var firewallCache struct {
	sync.Mutex
	rules []firewallRule
	at    time.Time
}

// readFirewallRules are the inbound rules for any smolcorp*.exe. Reading
// them needs no administrator.
var readFirewallRules = func() ([]firewallRule, error) {
	script := `$ErrorActionPreference = 'Stop'
$found = foreach ($app in @(Get-NetFirewallApplicationFilter -All | Where-Object { $_.Program -like '*smolcorp*.exe' })) {
  $rule = $app | Get-NetFirewallRule
  if ($rule.Direction -ne 'Inbound') { continue }
  $port = $rule | Get-NetFirewallPortFilter
  $address = $rule | Get-NetFirewallAddressFilter
  [pscustomobject]@{ Name = $rule.Name; DisplayName = $rule.DisplayName; Program = $app.Program; Profile = [string]$rule.Profile
    RemoteAddress = (@($address.RemoteAddress) -join ','); Protocol = [string]$port.Protocol; Action = [string]$rule.Action
    Enabled = ([string]$rule.Enabled -eq 'True') }
}
ConvertTo-Json -InputObject @($found) -Compress`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var rules []firewallRule
	if err := json.Unmarshal(out, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// firewallRulesNow are the rules, read again where the last read is over a
// minute old, or fresh is asked for.
func firewallRulesNow(fresh bool) ([]firewallRule, error) {
	firewallCache.Lock()
	defer firewallCache.Unlock()
	if !fresh && !firewallCache.at.IsZero() && time.Since(firewallCache.at) < time.Minute {
		return firewallCache.rules, nil
	}
	rules, err := readFirewallRules()
	if err != nil {
		return nil, err
	}
	firewallCache.rules, firewallCache.at = rules, time.Now()
	return rules, nil
}

// firewallCheck is the firewall's line of the checklist, for this copy.
func firewallCheck(scope string) *check {
	program, err := executable()
	if err != nil {
		return nil
	}
	rules, err := firewallRulesNow(false)
	if err != nil {
		c := check{ID: "firewall", Title: "Windows Firewall lets in only your devices", Status: checkFix, Fix: fixFirewall,
			Label: "Set the rules", Detail: "Its rules couldn't be read. Setting them asks Windows once for your OK."}
		return &c
	}
	c := firewallVerdict(rules, program, scope)
	if c.Status == checkOK {
		firewallSet.Store(true)
	}
	return &c
}

// lookAtFirewall reads the rules once, at start, so a computer set up by an
// earlier version listens on Tailscale without asking again.
func lookAtFirewall(scope string) {
	go firewallCheck(scope)
}

var errFirewallDeclined = errors.New("Windows didn't get your OK, so the rules are as they were. Try again, and choose Yes.")

// fixFirewallFor sets the rules for scope with the embedded script, run as
// administrator: Windows asks once for your OK. The rules are read again
// after.
func fixFirewallFor(scope string) error {
	program, err := executable()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "smolcorp-firewall-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	script, logFile := filepath.Join(dir, "firewall.ps1"), filepath.Join(dir, "firewall.log")
	if err := os.WriteFile(script, []byte(firewallScript), 0o600); err != nil {
		return err
	}
	mode := "tailscale"
	if scope == listenAll {
		mode = "lan"
	}
	quoted := func(text string) string { return "'" + strings.ReplaceAll(text, "'", "''") + "'" }
	arguments := strings.Join([]string{quoted("-NoProfile"), quoted("-ExecutionPolicy"), quoted("Bypass"), quoted("-File"), quoted(script),
		quoted("-Program"), quoted(program), quoted("-Port"), quoted(fmt.Sprint(Port)), quoted("-Mode"), quoted(mode),
		quoted("-LogPath"), quoted(logFile)}, ",")
	elevate := "Start-Process powershell -Verb RunAs -Wait -WindowStyle Hidden -ArgumentList " + arguments
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", elevate)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return errFirewallDeclined
	}
	said, _ := os.ReadFile(logFile)
	text := strings.TrimSpace(string(said))
	if !strings.HasSuffix(text, "DONE") {
		log.Printf("smolcorp: the firewall script said: %s", text)
		if i := strings.LastIndex(text, "ERROR: "); i >= 0 {
			return errors.New("Windows Firewall refused: " + strings.TrimPrefix(text[i:], "ERROR: "))
		}
		return errFirewallDeclined
	}
	rules, err := firewallRulesNow(true)
	if err == nil && firewallVerdict(rules, program, scope).Status == checkOK {
		firewallSet.Store(true)
	}
	return nil
}
