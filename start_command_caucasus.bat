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
:: Command freq 282.0 is shared across theatres; --map caucasus makes the
:: proactive handoff scan the Black Sea towers instead of the PG ones.
:: --handoff-marshal-freq 306.3: our Marshal (start_marshal_caucasus.bat), Hornet
:: COMM1 CH3 "CVN-72 AI MARSHALL". Operator ruling 2026-09-20 -- 306.200 is kept
:: clear for a live human Marshal, so never hand off there.
:: --jtac-freq 61.0: Hitman 1-1, the 9-line JTAC (jtac.go), 61.000 FM = Hornet
:: COMM2 CH12, Viper COMM2 CH8 (operator ruling 2026-10-14). Runs in this process
:: so Command's tasking and the JTAC share one board. 60.000 is the LIVE JTAC.
:: --jtac2-freq 62.0: Hitman 2-1 (operator 2026-10-16). With both up, Hitman 1-1
:: runs the Kuban Valley sector (targets 1-3) and Hitman 2-1 Cherkessk (4-5);
:: Command sends each flight to the one with a free target.
:: --tanker-freq 252.1: Texaco 1-1 KC-130 (Hornet COMM2 CH8, Eric). --tanker2-freq
:: 252.3: Texaco 3-1 KC-135 MPRS, angels 24 (COMM2 CH10, Brian; operator 2026-10-19).
start "vSFG-7 Command (Caucasus)" cmd /c "%~dp0atc.exe --command-only --map caucasus --command-freq 282.0 --command-name vSFG-7-Command --command-voice onyx --jtac-freq 61.0 --jtac-voice ash --jtac2-freq 62.0 --tanker-freq 252.1 --tanker2-freq 252.3 --handoff-marshal-freq 306.3 --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7770 --log-level %LOG%"
