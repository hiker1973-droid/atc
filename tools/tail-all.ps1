param()
$ErrorActionPreference = 'Stop'
$files = @(
  @{Site='OMDM';    Path='logs/atc-omdm.log'},
  @{Site='OMAM';    Path='logs/atc-omam.log'},
  @{Site='OMAL';    Path='logs/atc-omal.log'},
  @{Site='MARSHAL'; Path='logs/atc-marshal.log'},
  @{Site='COMMAND'; Path='logs/atc-command.log'},
  @{Site='ATIS';    Path='logs/atc-atis.log'}
)
foreach ($f in $files) {
  if (Test-Path $f.Path) { $f.Position = (Get-Item $f.Path).Length } else { $f.Position = 0 }
}
$keep = 'recognized|"ATC request"|TX via|intent miss|Whisper hallu|registered on SRS|stack online|"level":"warn"|"level":"error"|ATC online'
$drop = 'Tacview telemetry offline|Tacview nominal|Tacview connected but no position|TX done|flushing transmission|converting and sending|SRS TCP failed - retrying|SRS connect failed, retrying'
while ($true) {
  foreach ($f in $files) {
    if (-not (Test-Path $f.Path)) { continue }
    $len = (Get-Item $f.Path).Length
    if ($len -le $f.Position) { if ($len -lt $f.Position) { $f.Position = 0 }; continue }
    $fs = [System.IO.File]::Open($f.Path, 'Open', 'Read', 'ReadWrite')
    [void]$fs.Seek($f.Position, 'Begin')
    $sr = New-Object System.IO.StreamReader($fs)
    while (-not $sr.EndOfStream) {
      $line = $sr.ReadLine()
      if ($line -match $keep -and $line -notmatch $drop) {
        Write-Output ("[{0}] {1}" -f $f.Site, $line)
      }
    }
    $f.Position = $fs.Position
    $sr.Close(); $fs.Close()
  }
  Start-Sleep -Milliseconds 500
}
