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

:: Caucasus / Black Sea Marshal -- CVN-72.
::
:: 306.100 = "CVN-72 LIVE MARSHALL", COMM 1 CH 2 on the Hornet Black Sea preset
:: card (operator ruling 2026-09-15). NOT the PG 306.300: on this card 306.300 is
:: "CVN-72 AI AIRBOSS". Launching the PG start_marshal.bat on a Caucasus mission
:: puts Marshal there, which is how Training 1 ran it until now.
::
:: --airfield UGSB (Batumi): Marshal is carrier-only and registers no tower, so
:: this sets the weather/divert context and the magnetic variation BRC is
:: spoken with (+6.5 E). Log goes to atc-marshal.log -- --marshal-only wins the
:: log-slug switch in cmd/atc/main.go.
::
:: Voice coral and dashboard 6004 are shared with the PG and Syria Marshals by
:: design: only one map runs at a time.
start "Marshal (Caucasus)" cmd /c "%~dp0atc.exe --marshal-only --airfield UGSB --marshal-freq 306.1 --marshal-voice coral --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 %MIZ_FLAG% --log-level %LOG%"
