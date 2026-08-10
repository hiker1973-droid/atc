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

echo [vSFG-7] Starting Iraq ATC system (4 ATIS recovery bases)...

:: Limited to the 4 fields that carry a dedicated ATIS (Al Asad / Al Sahra /
:: Al Salam / Balad) to cap RAM. The other 5 tower-only bases (Baghdad ORBI,
:: Bashur ORBB, Erbil ORER, Kirkuk ORKK, Sulaymaniyah ORSU) are intentionally
:: not launched here — re-add their start lines if you need all 9.

:: Al Asad Tower — ORAA, UHF 363.70, dashboard 6031
start "Al Asad Tower" cmd /c "%~dp0atc.exe --airfield ORAA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6031 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Sahra Tower — ORSH, UHF 250.15, dashboard 6032
start "Al Sahra Tower" cmd /c "%~dp0atc.exe --airfield ORSH --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6032 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Salam Tower — ORBR, UHF 250.25, dashboard 6033
start "Al Salam Tower" cmd /c "%~dp0atc.exe --airfield ORBR --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6033 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Balad Tower — ORBD, UHF 250.55, dashboard 6035
start "Balad Tower" cmd /c "%~dp0atc.exe --airfield ORBD --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice fable --dashboard-port 6035 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] Iraq towers launched (dashboards 6031, 6032, 6033, 6035).
