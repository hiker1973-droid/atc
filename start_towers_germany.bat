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

echo [vSFG-7] Starting Cold War Germany ATC system (8 recovery bases)...

:: Ramstein Tower — ETAR, VHF 133.20, dashboard 6021
start "Ramstein Tower" cmd /c "%~dp0atc.exe --airfield ETAR --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6021 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Spangdahlem Tower — ETAD, VHF 122.20, dashboard 6022
start "Spangdahlem Tower" cmd /c "%~dp0atc.exe --airfield ETAD --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6022 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Hahn Tower — EDFH, VHF 119.50, dashboard 6023
start "Hahn Tower" cmd /c "%~dp0atc.exe --airfield EDFH --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6023 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Frankfurt Tower — EDDF, VHF 127.30, dashboard 6024
start "Frankfurt Tower" cmd /c "%~dp0atc.exe --airfield EDDF --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice echo --dashboard-port 6024 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Cologne Tower — EDDK, VHF 119.70, dashboard 6025
start "Cologne Tower" cmd /c "%~dp0atc.exe --airfield EDDK --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice fable --dashboard-port 6025 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Dusseldorf Tower — EDDL, VHF 118.30, dashboard 6026
start "Dusseldorf Tower" cmd /c "%~dp0atc.exe --airfield EDDL --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice coral --dashboard-port 6026 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Hannover Tower — EDDV, VHF 120.20, dashboard 6027
start "Hannover Tower" cmd /c "%~dp0atc.exe --airfield EDDV --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice sage --dashboard-port 6027 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Hamburg Tower — EDDH, VHF 126.85, dashboard 6028
start "Hamburg Tower" cmd /c "%~dp0atc.exe --airfield EDDH --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice ash --dashboard-port 6028 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] Germany towers launched (dashboards 6021-6028).
