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

:: Guard Watch -- 243.000 AM, Kola first (operator 2026-11-15). Answers mayday / "I'm hit" /
:: lost / "request vectors" with the nearest friendly field (bearing, range, tower freq)
:: and a radio check addressed to Guard; silent otherwise. Blue side only. See cmd\atc\guard.go.
:: --airfield ENBO only names the log/instance key; --map kola picks the fields offered.
start "vSFG-7 Guard (Kola)" cmd /c "%~dp0atc.exe --guard-only --map kola --airfield ENBO --guard-freq 243.0 --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --log-level %LOG%"
