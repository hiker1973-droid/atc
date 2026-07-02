param([switch]$Init)
$ErrorActionPreference = 'Stop'
$base = 'C:/SkyeyeATC/logs'
$stateFile = 'C:/SkyeyeATC/tools/.watch-offsets.json'
$files = @(
  @{Site='OMDM';    Path="$base/atc-omdm.log"},
  @{Site='OMAM';    Path="$base/atc-omam.log"},
  @{Site='OMAL';    Path="$base/atc-omal.log"},
  @{Site='MARSHAL'; Path="$base/atc-marshal.log"},
  @{Site='COMMAND'; Path="$base/atc-command.log"},
  @{Site='ATIS';    Path="$base/atc-atis.log"}
)
# Keep operational events; surface anomalies. Drop routine noise.
$keep = 'heard"|transcribed"|TX via|TX"|ATC request|intent miss|auto-release|LUAW issued|Whisper hallu|registered on SRS|stack online|"level":"warn"|"level":"error"|ATC online|already in progress|empty transcription|SRS disconnected|ExternalAudio file error|prewarm failed'
$drop = 'Tacview telemetry offline|Tacview nominal|Tacview connected but no position|TX done|flushing transmission|converting and sending|SRS TCP failed - retrying|SRS connect failed, retrying'

$offsets = @{}
if ((Test-Path $stateFile) -and -not $Init) {
  $offsets = Get-Content $stateFile -Raw | ConvertFrom-Json
}
$new = @{}
foreach ($f in $files) {
  if (-not (Test-Path $f.Path)) { continue }
  $len = (Get-Item $f.Path).Length
  $start = 0
  if ($offsets.$($f.Site)) { $start = [int64]$offsets.$($f.Site) }
  if ($len -lt $start) { $start = 0 } # rotated/truncated
  if (-not $Init -and $len -gt $start) {
    $fs = [System.IO.File]::Open($f.Path, 'Open', 'Read', 'ReadWrite')
    [void]$fs.Seek($start, 'Begin')
    $sr = New-Object System.IO.StreamReader($fs)
    while (-not $sr.EndOfStream) {
      $line = $sr.ReadLine()
      if ($line -match $keep -and $line -notmatch $drop) {
        Write-Output ("[{0}] {1}" -f $f.Site, $line)
      }
    }
    $sr.Close(); $fs.Close()
  }
  $new[$f.Site] = $len
}
($new | ConvertTo-Json) | Set-Content -Encoding utf8 $stateFile
