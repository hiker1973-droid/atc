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

echo [vSFG-7] Starting Iraq ATC system (9 recovery bases)...

:: Al Asad Tower — ORAA, UHF 363.70, dashboard 6031
start "Al Asad Tower" cmd /c "%~dp0atc.exe --airfield ORAA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6031 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Sahra Tower — ORSH, UHF 250.15, dashboard 6032
start "Al Sahra Tower" cmd /c "%~dp0atc.exe --airfield ORSH --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6032 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Salam Tower — ORBR, UHF 250.25, dashboard 6033
start "Al Salam Tower" cmd /c "%~dp0atc.exe --airfield ORBR --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6033 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Baghdad Tower — ORBI, UHF 250.30, dashboard 6034
start "Baghdad Tower" cmd /c "%~dp0atc.exe --airfield ORBI --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice echo --dashboard-port 6034 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Balad Tower — ORBD, UHF 250.55, dashboard 6035
start "Balad Tower" cmd /c "%~dp0atc.exe --airfield ORBD --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice fable --dashboard-port 6035 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Bashur Tower — ORBB, UHF 250.40, dashboard 6036
start "Bashur Tower" cmd /c "%~dp0atc.exe --airfield ORBB --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice coral --dashboard-port 6036 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Erbil Tower — ORER, UHF 250.35, dashboard 6037
start "Erbil Tower" cmd /c "%~dp0atc.exe --airfield ORER --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice sage --dashboard-port 6037 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Kirkuk Tower — ORKK, UHF 250.05, dashboard 6038
start "Kirkuk Tower" cmd /c "%~dp0atc.exe --airfield ORKK --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice ash --dashboard-port 6038 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Sulaymaniyah Tower — ORSU, UHF 250.50, dashboard 6039
start "Sulaymaniyah Tower" cmd /c "%~dp0atc.exe --airfield ORSU --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice onyx --dashboard-port 6039 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] Iraq towers launched (dashboards 6031-6039).
