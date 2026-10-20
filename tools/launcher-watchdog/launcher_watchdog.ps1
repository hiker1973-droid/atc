# vSFG-7 SkyEye: keep the launcher (dashboard :7000) running.
#
# launcher.exe has died silently on the rigs (2026-07-02, 2026-07-12, the host
# 2026-09-14) while the ATC roles kept talking: the dashboard, /fleet, the
# Update button and the nightly reports all lose that rig until someone
# restarts it by hand. This checks every 30 s and starts it again.
#
# Why a logon task and not a Windows service: a service runs in session 0, so
# the role console windows the launcher starts (one per tower / role) would
# be invisible on the desktop, and Bitdefender has flagged hidden process
# spawning on these boxes before. Install with install_launcher_watchdog.bat.
#
# It only STARTS a missing launcher. It never kills one: a launcher that is
# running but not answering is logged, not touched. An Update restarts the
# launcher itself within ~20 s, so a launcher must be missing on two checks in
# a row (60 s) before the watchdog steps in.

$Root = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent   # C:\SkyeyeATC
$Exe  = Join-Path $Root 'launcher.exe'
$Log  = Join-Path $Root 'logs\launcher-watchdog.log'
$Port = 7000

function Write-Log($msg) {
    $line = '{0}  {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $msg
    try { Add-Content -Path $Log -Value $line -Encoding utf8 } catch {}
}

function Test-Dashboard {
    try {
        $r = Invoke-WebRequest -UseBasicParsing -TimeoutSec 5 "http://127.0.0.1:$Port/api/version"
        return $r.StatusCode -eq 200
    } catch { return $false }
}

function Get-Launcher {
    Get-CimInstance Win32_Process -Filter "Name='launcher.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.ExecutablePath -and ($_.ExecutablePath -ieq $Exe) }
}

function Get-EnvVar($name) {
    foreach ($scope in 'Process', 'User', 'Machine') {
        $v = [Environment]::GetEnvironmentVariable($name, $scope)
        if ($v) { return $v }
    }
    return $null
}

function Start-Launcher {
    $srs = Get-EnvVar 'SKYEYE_SRS'
    $tv  = Get-EnvVar 'SKYEYE_TACVIEW'
    if (-not $srs -or -not $tv) {
        Write-Log 'SKYEYE_SRS / SKYEYE_TACVIEW not set - cannot start the launcher'
        return
    }
    $argList = @('--listen', ":$Port", '--srs-addr', $srs, '--tacview-addr', $tv)
    $miz = Get-EnvVar 'SKYEYE_MIZ'
    if ($miz) { $argList += @('--miz-path', ('"{0}"' -f $miz)) }
    Start-Process -FilePath $Exe -ArgumentList $argList -WorkingDirectory $Root -WindowStyle Minimized
    Write-Log ('launcher started: ' + ($argList -join ' '))
}

Write-Log "watchdog up (pid $PID), watching $Exe on :$Port"
$missing = 0
while ($true) {
    $proc = Get-Launcher
    $up = Test-Dashboard
    if ($proc -or $up) {
        if ($proc -and -not $up) {
            Write-Log "launcher running (pid $($proc.ProcessId -join ',')) but :$Port not answering - left alone"
        }
        $missing = 0
    } else {
        $missing++
        if ($missing -ge 2) {
            Write-Log "no launcher and :$Port down for $($missing * 30) s - starting it"
            Start-Launcher
            $missing = 0
            Start-Sleep -Seconds 30   # give it time to bind before the next check
        }
    }
    Start-Sleep -Seconds 30
}
