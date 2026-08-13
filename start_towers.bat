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

echo [vSFG-7] Starting ATC system...

:: Al Minhad Tower (Marshal now has its own start_marshal.bat)
::
:: Voice: shimmer (2026-08-13, operator preference — was nova). Same voice as
:: Deckboss, and it now runs at the Deckboss rate too (--tts-speed default is
:: speedDeckboss in main.go). Both slots are set to shimmer on purpose: the
:: tower voice rotates female/male every --voice-rotate-hours (default 4), so
:: leaving --tts-voice-male at onyx would flip Minhad off shimmer half the day.
::
:: This deliberately doubles shimmer up with Deckboss (128.6) and Al Dhafra
:: Tower (OMAM, 251.1). Minhad Tower and Deckboss are the same field, so a
:: carrier pilot working the beach hears one voice on both — accepted.
start "Al Minhad Tower" cmd /c "%~dp0atc.exe --airfield OMDM --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --tts-voice-male shimmer --dashboard-port 6001 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Dhafra Tower
start "Al Dhafra Tower" cmd /c "%~dp0atc.exe --airfield OMAM --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6002 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Ain Tower
start "Al Ain Tower" cmd /c "%~dp0atc.exe --airfield OMAL --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6003 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] All systems launched.
