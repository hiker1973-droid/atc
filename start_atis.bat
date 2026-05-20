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
start "vSFG-7 ATIS" cmd /c "%~dp0atc.exe --atis-only --srs-addr %SRS% --eam-password %EAM% --log-level %LOG%"

