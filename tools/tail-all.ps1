# Live tail across every running role, site-prefixed and filtered.
# Theatre-agnostic: by default it watches whatever atc.exe processes are up, so
# the same command works on PG, Caucasus, Syria, Germany and Iraq.
#
#   .\tools\tail-all.ps1                      # whatever is running now
#   .\tools\tail-all.ps1 -Theatre caucasus    # a named set (nothing running)
#   .\tools\tail-all.ps1 -Sites UGSB,UGKO     # just these fields
#   .\tools\tail-all.ps1 -All -Raw            # every log, raw JSONL
#
# Deckboss has no log of its own — it interleaves into its airfield's log
# (UGSB on Caucasus, OMDM on PG), so its events appear under that site.
param(
  [string[]]$Sites,
  [string]$Theatre,
  [switch]$All,
  [switch]$Raw
)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/log-sites.ps1"

$files = Resolve-LogSites -Sites $Sites -Theatre $Theatre -All:$All

# Start at the current end of each file: this is a live tail, not a replay.
foreach ($f in $files) { $f.Position = (Get-Item -LiteralPath $f.Path).Length }

Write-Host 'Ctrl+C to stop.' -ForegroundColor DarkGray
while ($true) {
  foreach ($f in $files) {
    if (-not (Test-Path $f.Path)) { continue }
    $r = Read-NewLines -Path $f.Path -Start $f.Position
    foreach ($line in $r.Lines) {
      if (Test-KeepLine $line) { Format-Event -Site $f.Site -Line $line -Raw:$Raw }
    }
    $f.Position = $r.Next
  }
  Start-Sleep -Milliseconds 500
}
