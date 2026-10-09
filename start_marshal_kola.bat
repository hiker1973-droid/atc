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
if not defined SKYEYE_TACVIEW (
    echo ERROR: SKYEYE_TACVIEW env var not set. Run: setx SKYEYE_TACVIEW ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set TACVIEW=%SKYEYE_TACVIEW%
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

:: Kola Marshal -- CVN-72 in the Vestfjord (Operation WHITE NIGHT).
::
:: 306.300 follows the standing operator ruling of 2026-09-20 ("306.3 is our SkyeyeATC
:: Marshall; 306.2 is when we have a live human Marshall"). On the Kola Hornet card
:: 306.300 is COMM1 CH3, printed "CVN-72 AI AIRBOSS"; COMM1 CH2 306.100 is printed
:: "CVN-72 LIVE MARSHALL" and is left clear for a human. NOT YET RATIFIED FOR KOLA --
:: this applies the Caucasus/PG ruling by analogy.
::
:: --airfield ENBO (Bodo): Marshal is carrier-only and registers no tower, so this
:: sets the weather/divert context and the magnetic variation BRC is spoken with
:: (+8.0 E at Bodo; the Vestfjord is 7.7 E).
::
:: Voice coral and dashboard 6004 are shared with the other theatres' Marshals by
:: design: only one map runs at a time.
start "Marshal (Kola)" cmd /c "%~dp0atc.exe --marshal-only --airfield ENBO --marshal-freq 306.3 --marshal-voice coral --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 %MIZ_FLAG% --log-level %LOG%"
