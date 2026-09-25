# Shared helpers for tail-all.ps1 and watch-since.ps1.
# Dot-source it:  . "$PSScriptRoot/log-sites.ps1"
#
# Why this file exists: both watchers hardcoded the three PG tower logs
# (atc-omdm / atc-omam / atc-omal). On a rig running any other theatre they
# showed three dead logs and silently missed every live tower — on DCS3 that
# meant all four Caucasus fields. Site resolution now happens in one place and
# defaults to whatever is actually running.

$LogDir = 'C:/SkyeyeATC/logs'

# Roles that always log to a fixed slug regardless of theatre (cmd/atc/main.go:231).
$FixedRoleLogs = [ordered]@{
  MARSHAL = 'atc-marshal.log'
  COMMAND = 'atc-command.log'
  ATIS    = 'atc-atis.log'
}

# Convenience mirror of the per-theatre tower sets in pkg/airfield/registry.go.
# The registry is the source of truth — this is only for -Theatre on a rig where
# nothing is running (post-mortem). Auto-detect is the default and needs no table,
# so if these drift the damage is limited to the explicit -Theatre shortcut.
$TheatreSites = @{
  pg       = @('OMDM', 'OMAM', 'OMAL')
  caucasus = @('UGSB', 'UG5X', 'UGKS', 'UGKO')
  syria    = @('LCRA', 'LTAG', 'LLRD', 'OSLK', 'OLBA')
  germany  = @('ETAR', 'ETAD', 'EDFH', 'EDDF', 'EDDK', 'EDDL', 'EDDV', 'EDDH')
  iraq     = @('ORAA', 'ORSH', 'ORBR', 'ORBI', 'ORBD', 'ORBB', 'ORER', 'ORKK', 'ORSU')
}

# Keep operational events and every fault signature; drop routine heartbeats.
#
# ⚠ NEVER put a dash in these patterns. The log writes an EM-DASH ("SRS TCP
# failed — retrying in 10s"), and the old drop list spelled it with an ASCII
# hyphen, so it never matched — a reconnect storm flooded the tail with the one
# line the filter was written to suppress (425 of them in atc-ugsb.log alone).
# Match on the stable prefix before the dash instead.
#
# The pilot-call events are "ATC request" (towers) and "<Role> heard" (Marshal /
# Command / Deckboss), which watch-since had and tail-all did not — so tail-all
# was blind to every non-tower pilot call.
#
# `{"message":"recognized","text":"..."}` carries the raw Whisper transcription
# and must stay in this list. It is the only line that shows what the STT
# actually heard, which is what tells "tower alive but silent" (a garbled field
# name dropped before intent matching, logging `recognized` with no `intent
# miss`) apart from a radio fault. It is quiet on a field with no traffic — 808
# in atc-omdm.log, 0 in atc-ugsb.log — so an empty grep means no pilot called
# that field, NOT that the term is dead. It was briefly cut from this list on
# exactly that mistaken reading.
$KeepPattern = 'ATC request|heard"|"message":"recognized"|TX via|intent miss|auto-release|LUAW|Whisper hallu|' +
               'registered on SRS|stack online|"level":"warn"|"level":"error"|ATC online|' +
               'already in progress|empty transcription|SRS disconnected|' +
               'ExternalAudio file error|ExternalAudio file TX timed out|prewarm failed|' +
               'Weather reloaded|runway rotation'

$DropPattern = 'Tacview telemetry offline|Tacview nominal|Tacview connected but no position|' +
               'Tacview contact first-seen|TX done|flushing transmission|' +
               'converting and sending|SRS TCP failed|SRS connect failed|connecting to SRS'

function Test-KeepLine {
  param([string]$Line)
  return ($Line -match $KeepPattern) -and ($Line -notmatch $DropPattern)
}

# Resolve which log files to watch.
#   -Sites UGSB,UGKO   explicit ICAOs (or MARSHAL/COMMAND/ATIS)
#   -Theatre caucasus  a named set from the table above
#   -All               every atc-*.log on disk
#   (default)          the roles actually running, else logs touched recently
# Returns a list of @{Site=...; Path=...}.
function Resolve-LogSites {
  param(
    [string[]]$Sites,
    [string]$Theatre,
    [switch]$All,
    [int]$RecentMinutes = 60
  )

  $slugs = @()
  $how = ''

  if ($Sites) {
    $slugs = $Sites | ForEach-Object { $_.ToUpper() }
    $how = 'explicit -Sites'
  }
  elseif ($Theatre) {
    $key = $Theatre.ToLower()
    if (-not $TheatreSites.ContainsKey($key)) {
      throw "Unknown -Theatre '$Theatre'. Known: $($TheatreSites.Keys -join ', ')"
    }
    $slugs = $TheatreSites[$key] + $FixedRoleLogs.Keys
    $how = "-Theatre $key"
  }
  elseif ($All) {
    $slugs = Get-ChildItem "$LogDir/atc-*.log" -ErrorAction SilentlyContinue |
      ForEach-Object { ($_.BaseName -replace '^atc-', '').ToUpper() }
    $how = '-All (every log on disk)'
  }
  else {
    $slugs = Get-RunningSites
    if ($slugs.Count -gt 0) {
      $how = 'running atc.exe processes'
    } else {
      # Nothing running: fall back to logs written recently, so a post-mortem
      # right after a crash still finds the right set.
      $cut = (Get-Date).AddMinutes(-$RecentMinutes)
      $slugs = Get-ChildItem "$LogDir/atc-*.log" -ErrorAction SilentlyContinue |
        Where-Object { $_.LastWriteTime -gt $cut } |
        ForEach-Object { ($_.BaseName -replace '^atc-', '').ToUpper() }
      $how = "logs written in the last $RecentMinutes min (nothing running)"
    }
  }

  $out = @()
  foreach ($s in ($slugs | Select-Object -Unique)) {
    $path = if ($FixedRoleLogs.Contains($s)) { "$LogDir/$($FixedRoleLogs[$s])" }
            else { "$LogDir/atc-$($s.ToLower()).log" }
    if (Test-Path $path) { $out += @{ Site = $s; Path = $path } }
  }
  if ($out.Count -eq 0) { throw "No log files resolved ($how). Try -All or -Sites." }

  Write-Host ("watching {0} log(s) via {1}: {2}" -f $out.Count, $how, (($out | ForEach-Object { $_.Site }) -join ' ')) -ForegroundColor DarkGray
  return $out
}

# Read the live set straight off the running processes — authoritative, and it
# does not depend on how recently a quiet tower happened to log.
# Deckboss has no log of its own: it runs as --airfield <ICAO> --deckboss-freq
# and interleaves into that field's log, so it needs no separate entry.
function Get-RunningSites {
  $procs = Get-CimInstance Win32_Process -Filter "Name='atc.exe'" -ErrorAction SilentlyContinue
  $found = @()
  foreach ($p in $procs) {
    $cl = $p.CommandLine
    if (-not $cl) { continue }
    if ($cl -match '--marshal-only')      { $found += 'MARSHAL'; continue }
    if ($cl -match '--command-only')      { $found += 'COMMAND'; continue }
    if ($cl -match '--atis-only')         { $found += 'ATIS';    continue }
    if ($cl -match '--scudwatch-only')    { continue }
    if ($cl -match '--airfield\s+(\w{4})') { $found += $Matches[1].ToUpper() }
  }
  return ($found | Select-Object -Unique)
}

# Read complete lines appended since $Start. Returns @{Lines=...; Next=<int64>}.
#
# Stops at the last newline on purpose. The previous readers consumed to EOF,
# so a half-written JSONL line was emitted as if complete and its remainder
# emitted again on the next pass — two corrupt events per interrupted write,
# which is exactly when the log is busiest.
function Read-NewLines {
  param([string]$Path, [int64]$Start)

  $len = (Get-Item -LiteralPath $Path).Length
  if ($len -lt $Start) { $Start = 0 }          # rotated or truncated
  if ($len -le $Start) { return @{ Lines = @(); Next = $Start } }

  $count = [int]($len - $Start)
  $buf = [byte[]]::new($count)
  $fs = [System.IO.File]::Open($Path, 'Open', 'Read', 'ReadWrite')
  try {
    [void]$fs.Seek($Start, 'Begin')
    $read = $fs.Read($buf, 0, $count)
  } finally { $fs.Close() }

  $text = [System.Text.Encoding]::UTF8.GetString($buf, 0, $read)
  $cut = $text.LastIndexOf("`n")
  if ($cut -lt 0) { return @{ Lines = @(); Next = $Start } }   # only a partial line so far

  $complete = $text.Substring(0, $cut + 1)
  $next = $Start + [System.Text.Encoding]::UTF8.GetByteCount($complete)
  return @{
    Lines = @($complete -split "`r?`n" | Where-Object { $_ -ne '' })
    Next  = $next
  }
}

# Condense a JSONL event to one readable line. The operator reads these live,
# so the default is the short form; pass -Raw to either script for the JSON.
function Format-Event {
  param([string]$Site, [string]$Line, [switch]$Raw)

  if ($Raw) { return ("[{0}] {1}" -f $Site, $Line) }

  $o = $null
  try { $o = $Line | ConvertFrom-Json -ErrorAction Stop } catch { }
  if (-not $o) { return ("[{0}] {1}" -f $Site, $Line) }

  $t = ''
  if ($o.time) {
    try { $t = ([datetime]$o.time).ToString('HH:mm:ss') } catch { $t = [string]$o.time }
  }
  $lvl = if ($o.level) { $o.level.ToUpper() } else { '-' }

  $extra = @()
  foreach ($p in $o.PSObject.Properties) {
    if ($p.Name -in @('level', 'time', 'message')) { continue }
    $extra += ('{0}={1}' -f $p.Name, $p.Value)
  }

  $line = '{0} [{1,-7}] {2,-5} {3}' -f $t, $Site, $lvl, $o.message
  if ($extra.Count -gt 0) { $line += '  ' + ($extra -join ' ') }
  return $line
}
