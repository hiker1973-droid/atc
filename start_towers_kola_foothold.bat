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
:: Boot weather follows the loaded mission. The DCS hook writes current_mission.txt on every
:: load (its setx re-pin of SKYEYE_MIZ can't run: os.execute is a stub in DCS hooks, 2026-11-21).
:: Falls back to SKYEYE_MIZ when the file or the .miz it names is missing.
set "CUR_MIZ="
if exist "%~dp0current_mission.txt" set /p CUR_MIZ=<"%~dp0current_mission.txt"
if defined CUR_MIZ if exist "%CUR_MIZ%" set "SKYEYE_MIZ=%CUR_MIZ%"
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

echo [vSFG-7] Starting Foothold Kola ATC system...

:: FOOTHOLD KOLA (operator 2026-11-14): FIVE fields, unlike start_towers_kola.bat (White Night: only the field in play).
:: Andoya + Bodo are blue's home fields; Bardufoss, Banak and Rovaniemi are the next captures. Tower = the DCS map's own UHF
:: tower, the same value the Foothold Kola preset cards print. Andoya (ENAN) is derived, not surveyed -- see pkg/airfield/enan.go.
:: Andoya Tower -- ENAN, 250.600, dashboard 6055
start "Andoya Tower" cmd /c "%~dp0atc.exe --airfield ENAN --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice coral --dashboard-port 6055 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul
:: Bodo Tower -- ENBO, 251.150, dashboard 6051
start "Bodo Tower" cmd /c "%~dp0atc.exe --airfield ENBO --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6051 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul
:: Bardufoss Tower -- ENDU, 250.650, dashboard 6052
start "Bardufoss Tower" cmd /c "%~dp0atc.exe --airfield ENDU --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6052 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul
:: Banak Tower -- ENNA, 250.550, dashboard 6053
start "Banak Tower" cmd /c "%~dp0atc.exe --airfield ENNA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6053 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul
:: Rovaniemi Tower -- EFRO, 250.950, dashboard 6054
start "Rovaniemi Tower" cmd /c "%~dp0atc.exe --airfield EFRO --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice echo --dashboard-port 6054 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul
echo [vSFG-7] Foothold Kola towers launched.
