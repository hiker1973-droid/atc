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
:: NOT part of start_region_caucasus.bat: skyeye.exe answers on 264.000 too, and
:: two GCIs on one frequency talk over each other. Stop skyeye.exe on this rig
:: first, then start this from the dashboard. Once it has proven itself, the
:: SkyEye bot comes out of the DCS server and this goes into the region bat.
::
:: --gci-picture-interval 5m: SkyEye's own default is 2m; every PICTURE is
:: spoken through ElevenLabs. --airfield only tells this atc.exe apart in the
:: launcher's process match.
start "vSFG-7 GCI Darkstar (Caucasus)" cmd /c "%~dp0atc.exe --gci-only --airfield UGSB --gci-callsign Darkstar --gci-freq 264.0 --gci-picture-interval 5m --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --log-level %LOG%"
