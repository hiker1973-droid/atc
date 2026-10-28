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

:: DCS-gRPC for DCS's own LSO grade and wire. No PG rig has been fixed yet, so
:: this defaults to :50051 (Day's port); set SKYEYE_GRPC for the rig PG runs on.
set GRPC=localhost:50051
if defined SKYEYE_GRPC set GRPC=%SKYEYE_GRPC%

:: Persian Gulf Paddles (LSO). Same role, frequency and voice as
:: start_lso_caucasus.bat (128.300, OpenAI ash, speed 1.66): one Paddles for every
:: theatre.
:: --airfield (not used by Paddles) tells this role's atc.exe apart from the
:: other theatres' Paddles: the launcher matches a role to its process by
:: command line. OMDM is the PG carrier field Deckboss names; Deckboss's own
:: command line differs, so the two do not collide.
start "Paddles (Persian Gulf)" cmd /c "%~dp0atc.exe --lso-only --airfield OMDM --lso-freq 128.3 --lso-voice ash --tts-speed 1.66 --srs-addr %SRS% --tacview-addr %TACVIEW% --lso-grpc-addr %GRPC% --eam-password %EAM% --log-level %LOG%"
