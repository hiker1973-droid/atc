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
:: Boot weather follows the loaded mission. The DCS hook writes current_mission.txt on every
:: load (its setx re-pin of SKYEYE_MIZ can't run: os.execute is a stub in DCS hooks, 2026-11-21).
:: Falls back to SKYEYE_MIZ when the file or the .miz it names is missing.
set "CUR_MIZ="
if exist "%~dp0current_mission.txt" set /p CUR_MIZ=<"%~dp0current_mission.txt"
if defined CUR_MIZ if exist "%CUR_MIZ%" set "SKYEYE_MIZ=%CUR_MIZ%"
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"
:: Command freq 282.0 is shared across theatres (COMM1 P1 on the Germany card);
:: --map germany makes the proactive handoff scan the 8 German recovery bases.
start "vSFG-7 Command (Germany)" cmd /c "%~dp0atc.exe --command-only --map germany --command-freq 282.0 --command-name vSFG-7-Command --command-voice onyx --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7770 --log-level %LOG%"
