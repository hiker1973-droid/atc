@echo off
cd /d C:\SkyeyeATC
if not defined SKYEYE_SRS (
    echo ERROR: SKYEYE_SRS env var not set. Run: setx SKYEYE_SRS ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set SRS=%SKYEYE_SRS%
if not defined SKYEYE_TACVIEW (
    echo ERROR: SKYEYE_TACVIEW env var not set. Run: setx SKYEYE_TACVIEW ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set TACVIEW=%SKYEYE_TACVIEW%
if not defined SRS_EAM (
    echo ERROR: SRS_EAM env var not set. Run: setx SRS_EAM ^<password^>  then open a new cmd.
    pause
    exit /b 1
)
set EAM=%SRS_EAM%
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

:: Syria / Eastern Med Marshal -- CVN-72 "ABE", TACAN 72X / ICLS 12 / ACLS on.
::
:: 306.300 since 2026-09-19 (operator ruling; was 306.100 from 2026-08-29).
:: Still use THIS bat on this map, not the PG one: --airfield LCRA sets the
:: Levant magnetic variation BRC is spoken with.
::
:: --airfield LCRA (Akrotiri): Marshal is carrier-only and does not register a
:: tower, so this only sets the weather/divert context and the log slug. Akrotiri
:: is the nearest major divert to an Eastern Med CVN. Log goes to atc-marshal.log
:: regardless -- --marshal-only wins the log-slug switch in cmd/atc/main.go.
::
:: Voice coral, same as PG Marshal: only one map runs at a time, so there is no
:: collision. Dashboard 6004 is likewise shared with PG Marshal by design.
:: 306.3 + stern female voice (operator ruling 2026-09-19): Hornet COMM1 CH3
:: "CVN-72 AI MARSHALL". Was 306.1 (card CH2 "LIVE MARSHALL", now unused).
start "Marshal (Syria)" cmd /c "%~dp0atc.exe --marshal-only --airfield LCRA --marshal-freq 306.3 --marshal-voice sage --voice-style-marshal marshal-stern --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 %MIZ_FLAG% --log-level %LOG%"
