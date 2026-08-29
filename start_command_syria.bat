@echo off
cd /d C:\SkyeyeATC
if not defined SKYEYE_SRS (
    echo ERROR: SKYEYE_SRS env var not set. Run: setx SKYEYE_SRS ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set SRS=%SKYEYE_SRS%
if not defined SRS_EAM (
    echo ERROR: SRS_EAM env var not set. Run: setx SRS_EAM ^<password^>  then open a new cmd.
    pause
    exit /b 1
)
set EAM=%SRS_EAM%
set LOG=info
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"
set TACVIEW_FLAG=
if defined SKYEYE_TACVIEW set TACVIEW_FLAG=--tacview-addr "%SKYEYE_TACVIEW%"

echo [vSFG-7] Starting Syria Command (282.0)...
start "vSFG-7 Command (Syria)" cmd /c "%~dp0atc.exe --command-only --map syria --command-freq 282.0 --command-name vSFG-7-Command --command-voice sage --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7775 --log-level %LOG%"
echo [vSFG-7] Syria Command launched.
