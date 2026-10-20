@echo off
rem vSFG-7 SkyEye: install the launcher watchdog on this rig (run once, as the
rem user the launcher runs as). It registers a Task Scheduler task that starts
rem at logon and restarts launcher.exe if it dies (tools\launcher-watchdog).
rem Remove with:  schtasks /delete /tn "SkyEye Launcher Watchdog" /f
cd /d %~dp0
set PS1=%~dp0tools\launcher-watchdog\launcher_watchdog.ps1
if not exist "%PS1%" (
    echo ERROR: %PS1% not found - git pull first.
    pause
    exit /b 1
)
schtasks /create /tn "SkyEye Launcher Watchdog" /sc onlogon /ru "%USERNAME%" /it /rl limited /f ^
  /tr "powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File \"%PS1%\""
if errorlevel 1 (
    echo.
    echo Task creation failed. Run this from an elevated Command Prompt and try again.
    pause
    exit /b 1
)
schtasks /run /tn "SkyEye Launcher Watchdog"
echo.
echo Installed and started. It logs to %~dp0logs\launcher-watchdog.log
pause
