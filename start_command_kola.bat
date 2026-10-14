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
:: Command 282.0 is shared across theatres; --map kola makes the proactive handoff
:: scan the Kola field(s) in play instead of the PG ones.
:: --handoff-marshal-freq 306.3: our Marshal (start_marshal_kola.bat). Without the
:: flag the default happens to be 306.3 too, but Syria shipped relying on a default
:: and handed pilots to the wrong Marshal -- so it is stated here.
start "vSFG-7 Command (Kola)" cmd /c "%~dp0atc.exe --command-only --map kola --command-freq 282.0 --command-name vSFG-7-Command --command-voice onyx --handoff-marshal-freq 306.3 --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7770 --log-level %LOG%"
