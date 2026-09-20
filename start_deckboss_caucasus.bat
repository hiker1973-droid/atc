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

:: Caucasus / Black Sea Deckboss -- 128.600, same as every other theatre
:: (operator ruling 2026-09-15), which DEVIATES from the Black Sea preset card.
::
:: The card puts CVN-72 DECKBOSS on COMM 2 CH 1 = 306.200, and 128.600 on
:: COMM 1 CH 1 = CVN-72 AI, the mission's own carrier controller. So on 128.600
:: we transmit on top of the DCS AI ATC, and a pilot using COMM 2 CH 1 as
:: printed will not hear us. The Comms Packet tells pilots 128.600.
::
:: --handoff-marshal-freq 306.3: the airborne ack pushes departures to our Marshal
:: (start_marshal_caucasus.bat), Hornet COMM1 CH3 "CVN-72 AI MARSHALL". Operator
:: ruling 2026-09-20 -- 306.200 stays clear for a live human Marshal.
::
:: --airfield UGSB (Batumi): --deckboss-freq makes this a deckboss-only instance
:: and the tower srsLoop is skipped (cmd/atc/main.go), so no duplicate Batumi
:: Tower registers on 260.000. It does set the log slug and the magnetic
:: variation BRC is spoken with, so Deckboss events land in atc-ugsb.log
:: alongside Batumi Tower -- filter on Deckboss/128.6 when monitoring.
::
:: Voice shimmer and dashboard 6005, same as PG and Syria Deckboss: only one map
:: runs at a time.
start "Deckboss (Caucasus)" cmd /c "%~dp0atc.exe --airfield UGSB --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --deckboss-freq 128.6 --deckboss-voice shimmer --handoff-marshal-freq 306.3 --no-atis --dashboard-port 6005 %MIZ_FLAG% --log-level %LOG%"
