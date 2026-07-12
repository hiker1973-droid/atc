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
set GOGC=50
set GOMEMLIMIT=256MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"
:: --map iraq selects the Iraq ATIS set (4 fields with a dedicated ATIS freq:
:: Al Asad 230.10, Al Sahra 230.20, Al Salam 230.30, Balad 230.70) and
:: broadcasts each in English then Arabic (atisSecondLangForMap).
start "vSFG-7 ATIS (Iraq)" cmd /c "%~dp0atc.exe --atis-only --map iraq --srs-addr %SRS% --eam-password %EAM% %MIZ_FLAG% --log-level %LOG%"
