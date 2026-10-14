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

:: Kola Deckboss -- 128.600, same as every other theatre (operator ruling 2026-09-15).
:: On the Kola Hornet card 128.600 is COMM1 CH1 "CVN-72 AI" and DECKBOSS is printed on
:: COMM2 CH1 = 306.200, so as elsewhere we transmit on top of the DCS AI carrier ATC.
::
:: --handoff-marshal-freq 306.3: the airborne ack pushes departures to our Marshal
:: (start_marshal_kola.bat).
::
:: --airfield ENBO (Bodo): --deckboss-freq makes this a deckboss-only instance and the
:: tower srsLoop is skipped, so no duplicate Bodo Tower registers on 251.150. It does
:: set the log slug, so Deckboss events land in atc-enbo.log alongside Bodo Tower --
:: filter on Deckboss/128.6 when monitoring.
::
:: Voice verse (OpenAI fallback; ElevenLabs Brian -- operator 2026-10-14; was shimmer) and dashboard 6005, same as the other theatres: only one map runs at a time.
start "Deckboss (Kola)" cmd /c "%~dp0atc.exe --airfield ENBO --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --deckboss-freq 128.6 --deckboss-voice verse --handoff-marshal-freq 306.3 --no-atis --dashboard-port 6005 %MIZ_FLAG% --log-level %LOG%"
