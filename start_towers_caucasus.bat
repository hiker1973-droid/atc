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

echo [vSFG-7] Starting Caucasus (Black Sea) ATC system...

:: Batumi Tower — UGSB, 260.0, dashboard 6011
start "Batumi Tower" cmd /c "%~dp0atc.exe --airfield UGSB --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6011 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Kobuleti Tower — UG5X, 262.0, dashboard 6012
start "Kobuleti Tower" cmd /c "%~dp0atc.exe --airfield UG5X --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6012 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Senaki Tower — UGKS, 261.0, dashboard 6013
start "Senaki Tower" cmd /c "%~dp0atc.exe --airfield UGKS --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6013 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Kutaisi Tower — UGKO, 263.0, dashboard 6014
start "Kutaisi Tower" cmd /c "%~dp0atc.exe --airfield UGKO --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice fable --dashboard-port 6014 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] Caucasus towers launched.
