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
if defined SKYEYE_TACVIEW (
    set TACVIEW_FLAG=--tacview-addr %SKYEYE_TACVIEW%
) else (
    echo NOTE: SKYEYE_TACVIEW not set. Command will run without proactive tower handoff.
    set TACVIEW_FLAG=
)
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

start "vSFG-7 Command" cmd /c "%~dp0atc.exe --command-only --command-freq 282.0 --command-name vSFG-7-Command --command-voice sage --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7770 --log-level %LOG%"
