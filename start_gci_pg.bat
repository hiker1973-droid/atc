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

:: GCI -- Overlord 266.000 AM for the Persian Gulf (operator 2026-10-28, Epic Fury:
:: "we need overlord gci setup for PG"). Same role as start_gci_syria.bat and
:: start_gci_caucasus.bat - read the notes there. 266.000 is the Hornets' COMM1 CH7
:: in Epic Fury v1.14 and Foothold's Overlord; the mission's AI AWACS keep their own
:: nets (Darkstar 1-1 E-3 264, Wizard 1-1 E-2 265), so the two never talk over
:: each other. No PG height grid: radar horizon only, no terrain masking.
start "vSFG-7 GCI Overlord (Persian Gulf)" cmd /c "%~dp0atc.exe --gci-only --map pg --airfield OMDM --gci-callsign Overlord --gci-freq 266.0 --gci-picture-interval 5m --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --log-level %LOG%"
