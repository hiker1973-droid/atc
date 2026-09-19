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
set LOG=info
set GOMAXPROCS=2
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

echo [vSFG-7] Starting Syria ATIS (5 stations, English + Arabic)...
:: ROSTER 2026-09-19 (operator): Akrotiri / Incirlik / Ramat David as the
:: primaries, Bassel Al-Assad / Beirut as alternate-divert. --atis-stations
:: is the selector -- the theatre set in atisStationsForMap still holds all
:: ten, so restoring King Hussein / Hatay / Gaziantep / Paphos / H4 is an
:: edit to THIS LINE, not a code change.
:: Bassel Al-Assad 249.600 and Beirut 249.700 are ASSIGNED BY US -- neither
:: field is on the presets card, so pilots have no preset for either.
:: Beirut broadcasts NDB 351 only; it has no ILS, TACAN or VOR.
start "vSFG-7 ATIS (Syria)" cmd /c "%~dp0atc.exe --atis-only --map syria --atis-stations LTAG,LLRD,LCRA,OSLK,OLBA --srs-addr %SRS% --eam-password %EAM% %MIZ_FLAG% --log-level %LOG%"
echo [vSFG-7] Syria ATIS launched.
