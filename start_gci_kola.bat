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

:: GCI -- Darkstar 264.000 AM for Kola / Operation WHITE NIGHT (operator 2026-10-21:
:: "we will need Kola"). Same role as start_gci_caucasus.bat - read the notes there.
:: 264.000 is ASSUMED to match the Caucasus comm plan (Hornet COMM1 CH5); confirm
:: against the Kola preset card before the campaign.
start "vSFG-7 GCI Darkstar (Kola)" cmd /c "%~dp0atc.exe --gci-only --map kola --airfield ENBO --gci-callsign Darkstar --gci-freq 264.0 --gci-picture-interval 5m --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --log-level %LOG%"
