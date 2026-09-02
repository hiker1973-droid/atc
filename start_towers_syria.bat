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
if not defined SKYEYE_TACVIEW (
    echo ERROR: SKYEYE_TACVIEW env var not set. Run: setx SKYEYE_TACVIEW ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set TACVIEW=%SKYEYE_TACVIEW%
set LOG=info
set GOMAXPROCS=4
set GOGC=50
set GOMEMLIMIT=512MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

:: Mafraq (OJMF) and H4 (OJHR) removed 2026-09-02 at operator request while
:: chasing SRS client-GUID collisions -- see cmd/atc/main.go:1113.
echo [vSFG-7] Starting Syria ATC system (6 recovery bases)...
:: NOTE: the Foothold VM runs SRS on :5002, NOT the :5008 the Training rig uses.
:: SKYEYE_SRS must say localhost:5002 on that host.

echo   Incirlik Tower (LTAG) -^> dashboard 6041
start "Incirlik Tower" cmd /c "%~dp0atc.exe --airfield LTAG --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6041 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 2 /nobreak >nul

echo   Ramat David Tower (LLRD) -^> dashboard 6042
start "Ramat David Tower" cmd /c "%~dp0atc.exe --airfield LLRD --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6042 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 2 /nobreak >nul

echo   Hatay Tower (LTDA) -^> dashboard 6044
start "Hatay Tower" cmd /c "%~dp0atc.exe --airfield LTDA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice echo --dashboard-port 6044 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 2 /nobreak >nul

echo   Gaziantep Tower (LTAJ) -^> dashboard 6045
start "Gaziantep Tower" cmd /c "%~dp0atc.exe --airfield LTAJ --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice fable --dashboard-port 6045 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 2 /nobreak >nul

echo   Akrotiri Tower (LCRA) -^> dashboard 6046
start "Akrotiri Tower" cmd /c "%~dp0atc.exe --airfield LCRA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice onyx --dashboard-port 6046 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 2 /nobreak >nul

echo   Paphos Tower (LCPH) -^> dashboard 6047
start "Paphos Tower" cmd /c "%~dp0atc.exe --airfield LCPH --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6047 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 2 /nobreak >nul

echo [vSFG-7] Syria towers launched.
