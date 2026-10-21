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

:: GCI -- Darkstar 264.000 AM (Day and Night; operator 2026-10-21). SkyEye's
:: GCI brain inside atc.exe (pkg/gci, cmd/atc/gci.go) with our voice: ElevenLabs
:: "Lily" (British woman) on Flash, OpenAI shimmer as the fallback.
::
:: Started by start_region_caucasus.bat / start_all_caucasus.bat since 2026-10-21,
:: when skyeye.exe came off the servers (two GCIs on 264 talk over each other -
:: never run skyeye.exe beside this; the dashboard chip shows GCI x2 if it is).
::
:: --gci-picture-interval 5m: SkyEye's own default is 2m; every PICTURE is
:: spoken through ElevenLabs. --airfield only tells this atc.exe apart in the
:: launcher's process match.
start "vSFG-7 GCI Darkstar (Caucasus)" cmd /c "%~dp0atc.exe --gci-only --airfield UGSB --gci-callsign Darkstar --gci-freq 264.0 --gci-picture-interval 5m --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --log-level %LOG%"
