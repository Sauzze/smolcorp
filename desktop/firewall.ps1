<#
.SYNOPSIS
  Sets Windows Firewall's rules for Smolcorp. Smolcorp runs it itself, with
  your OK, from setup's "Set the rules": you don't need to run it by hand.

.DESCRIPTION
  1. Removes every inbound rule for a smolcorp*.exe: copies in Downloads, an
     older install, and the ones Windows' "Allow access" box made, which let
     anyone in on a public network.
  2. Lets TCP on -Port in to -Program from Tailscale's range only
     (100.64.0.0/10), on every network profile.
  3. With -Mode lan, also from the local subnet, on private networks only.
  Running it again gives the same rules. It needs an administrator's rights.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\firewall.ps1 -Program "$env:LOCALAPPDATA\Smolcorp\Smolcorp.exe"
#>
param(
    [Parameter(Mandatory = $true)][string]$Program,
    [int]$Port = 7710,
    [ValidateSet('tailscale', 'lan')][string]$Mode = 'tailscale',
    [string]$LogPath
)

$ErrorActionPreference = 'Stop'

function Say([string]$text) {
    Write-Host $text
    if ($LogPath) { Add-Content -Path $LogPath -Value $text -Encoding UTF8 }
}

try {
    $rules = @(Get-NetFirewallApplicationFilter -All |
        Where-Object { $_.Program -like '*smolcorp*.exe' } |
        Get-NetFirewallRule |
        Where-Object { $_.Direction -eq 'Inbound' })
    foreach ($rule in $rules) {
        $old = ($rule | Get-NetFirewallApplicationFilter).Program
        Remove-NetFirewallRule -InputObject $rule
        Say "Removed: $($rule.DisplayName), for $old"
    }

    New-NetFirewallRule -DisplayName 'Smolcorp (Tailscale)' -Group 'Smolcorp' -Direction Inbound -Action Allow `
        -Program $Program -Protocol TCP -LocalPort $Port -RemoteAddress '100.64.0.0/10' -Profile Any | Out-Null
    Say "Added: Smolcorp (Tailscale), TCP $Port from 100.64.0.0/10"

    if ($Mode -eq 'lan') {
        New-NetFirewallRule -DisplayName 'Smolcorp (home network)' -Group 'Smolcorp' -Direction Inbound -Action Allow `
            -Program $Program -Protocol TCP -LocalPort $Port -RemoteAddress LocalSubnet -Profile Private | Out-Null
        Say "Added: Smolcorp (home network), TCP $Port from the local subnet, private networks only"
    }
    Say 'DONE'
}
catch {
    Say "ERROR: $($_.Exception.Message)"
    exit 1
}
