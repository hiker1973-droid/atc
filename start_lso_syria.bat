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

:: DCS-gRPC for DCS's own LSO grade and wire: Foothold is :50052 (operator's
:: server list 2026-10-17). SKYEYE_GRPC overrides.
set GRPC=localhost:50052
if defined SKYEYE_GRPC set GRPC=%SKYEYE_GRPC%

:: Syria / Eastern Med (Foothold) Paddles (LSO) -- CVN-72 Abe. Operator
:: 2026-10-18: "Foothold can use deckboss, Marshall and paddles".
::
:: Same role, frequency and voice as start_lso_caucasus.bat (128.300, OpenAI ash,
:: speed 1.66): one Paddles for every theatre. The Foothold card's LSO 128.100 is
:: not used. Groove calls need the rig's Tacview feed (offline on Foothold
:: since 2026-10-15); without it Paddles still answers abeam, ball and radio
:: check calls.
start "Paddles (Syria)" cmd /c "%~dp0atc.exe --lso-only --lso-freq 128.3 --lso-voice ash --tts-speed 1.66 --srs-addr %SRS% --tacview-addr %TACVIEW% --lso-grpc-addr %GRPC% --eam-password %EAM% --log-level %LOG%"
