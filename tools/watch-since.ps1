# Offset-based sweep: prints every kept event appended since the last run.
# Built for periodic check-ins ("tail the training for the next 30 min") where a
# blocking tail would not survive between turns — state lives in the offsets file.
#
#   .\tools\watch-since.ps1 -Init             # set the baseline, print nothing
#   .\tools\watch-since.ps1                   # everything since that baseline
#   .\tools\watch-since.ps1 -Theatre caucasus # named set (nothing running)
#   .\tools\watch-since.ps1 -Sites UGSB -Raw  # one field, raw JSONL
#
# Theatre-agnostic: the default site list comes from the running atc.exe
# processes, so it follows whichever map the rig is on.
param(
  [switch]$Init,
  [string[]]$Sites,
  [string]$Theatre,
  [switch]$All,
  [switch]$Raw
)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/log-sites.ps1"

$stateFile = "$PSScriptRoot/.watch-offsets.json"
$files = Resolve-LogSites -Sites $Sites -Theatre $Theatre -All:$All

# Load unconditionally, including under -Init: the offsets of sites this run does
# not touch have to be carried through either way. -Init still ignores them for
# reading (each resolved site jumps straight to the current end of its log).
$offsets = @{}
if (Test-Path $stateFile) {
  $json = Get-Content $stateFile -Raw | ConvertFrom-Json
  foreach ($p in $json.PSObject.Properties) { $offsets[$p.Name] = [int64]$p.Value }
}

# Start from the stored offsets rather than an empty map: a narrowed run
# (-Sites / -Theatre) must not drop the baselines of the sites it skipped, or the
# next full sweep restarts them at 0 and replays the entire log.
$new = @{}
foreach ($k in $offsets.Keys) { $new[$k] = $offsets[$k] }
$shown = 0
foreach ($f in $files) {
  if (-not (Test-Path $f.Path)) { continue }
  $len = (Get-Item -LiteralPath $f.Path).Length

  if ($Init) { $new[$f.Site] = $len; continue }

  $start = 0
  if ($offsets.ContainsKey($f.Site)) { $start = $offsets[$f.Site] }

  $r = Read-NewLines -Path $f.Path -Start $start
  foreach ($line in $r.Lines) {
    if (Test-KeepLine $line) {
      Format-Event -Site $f.Site -Line $line -Raw:$Raw
      $shown++
    }
  }
  # Advance only past complete lines, so a half-written record is re-read whole
  # on the next sweep instead of being reported twice in two broken halves.
  $new[$f.Site] = $r.Next
}

($new | ConvertTo-Json) | Set-Content -Encoding utf8 $stateFile

if ($Init) {
  Write-Host "baseline set for $($files.Count) log(s) — nothing printed by design." -ForegroundColor DarkGray
} elseif ($shown -eq 0) {
  Write-Host 'no new events.' -ForegroundColor DarkGray
}
