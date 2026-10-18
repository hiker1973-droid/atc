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

:: DCS-gRPC for DCS's own LSO grade and wire (what Tactical Paddles reads):
:: Day (SRS :5008) is :50051, Night (SRS :5004) is :50053 (operator's server
:: list 2026-10-17). SKYEYE_GRPC overrides.
set GRPC=localhost:50051
if "%SRS%"=="localhost:5004" set GRPC=localhost:50053
if defined SKYEYE_GRPC set GRPC=%SKYEYE_GRPC%

:: Caucasus / Black Sea Paddles (LSO) -- CVN-72 Abe.
::
:: 128.300 = Hornet COMM2 CH4, Paddles only (operator 2026-10-17: "Put it on
:: Channel 4 (Strike) Comm 2 at 128.3"; Strike leaves that channel). OpenAI
:: voice for now. Paddles reads Tacview itself and talks in the groove: roger ball,
:: high/low, power, lineup, fast/slow (Hornet), wave off, bolter. Every pass is
:: logged to atc-lso.log with where it touched down, for calibration.
:: Speed 1.66 = 1.38 +20% (operator 2026-10-17; was 1.20 +15%); "Power!" has its own
:: sterner delivery (lso.go styleLSOPower).
:: --airfield (not used by Paddles) tells this role's atc.exe apart from the
:: other theatre's Paddles: the launcher matches a role to its process by
:: command line, and the two were identical. On 2026-10-18 Foothold's Update
:: took its Syria Paddles for the Caucasus one and started the Caucasus region.
start "Paddles (Caucasus)" cmd /c "%~dp0atc.exe --lso-only --airfield UGSB --lso-freq 128.3 --lso-voice ash --tts-speed 1.66 --srs-addr %SRS% --tacview-addr %TACVIEW% --lso-grpc-addr %GRPC% --eam-password %EAM% --log-level %LOG%"
