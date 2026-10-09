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

echo [vSFG-7] Starting Kola ATC system...

:: Operation WHITE NIGHT. ONLY THE FIELD IN PLAY IS STARTED (operator 2026-10-09).
:: Mission 1 = Bodo. Bardufoss (missions 2-5), Rovaniemi (6-7) and Banak (8-10) are
:: parked: to bring one up, uncomment its two lines here, add its ICAO to
:: --atis-stations in start_atis_kola.bat, add it to the Kola slice in
:: pkg/airfield/registry.go and to the kola towers in cmd/launcher/ui.html.

:: Bodo Tower -- ENBO, 251.150, dashboard 6051
start "Bodo Tower" cmd /c "%~dp0atc.exe --airfield ENBO --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6051 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Bardufoss Tower -- ENDU, 250.650, dashboard 6052   [PARKED]
:: start "Bardufoss Tower" cmd /c "%~dp0atc.exe --airfield ENDU --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6052 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: %SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Banak Tower -- ENNA, 250.550, dashboard 6053   [PARKED]
:: start "Banak Tower" cmd /c "%~dp0atc.exe --airfield ENNA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6053 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: %SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Rovaniemi Tower -- EFRO, 250.950, dashboard 6054   [PARKED]
:: start "Rovaniemi Tower" cmd /c "%~dp0atc.exe --airfield EFRO --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice echo --dashboard-port 6054 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: %SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] Kola towers launched.
