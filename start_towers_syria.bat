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

echo [vSFG-7] Starting Syria ATC system (5 fields)...
:: NOTE: the Foothold VM runs SRS on :5002, NOT the :5008 the Training rig uses.
:: SKYEYE_SRS must say localhost:5002 on that host.
::
:: ROSTER 2026-09-19 (operator ruling): three primaries and two
:: alternate/divert fields. The other five towers are PARKED at the
:: bottom of this file, not deleted -- their airfield definitions and
:: ATIS stations are still in the code, so restoring one is uncommenting
:: its line here, adding its ICAO to --atis-stations in
:: start_atis_syria.bat, AND adding it to the Syria list in
:: pkg/airfield/registry.go (Command only hands off to fields in that list).
::
:: Bassel Al-Assad 250.600 / ATIS 249.600 and Beirut 250.650 / ATIS 249.700
:: are on the Foothold v1.10 Hornet presets (COMM1 CH14-17). Beirut is NOT the
:: DCS terrain's 253.200 -- that is the SHELL 2 tanker (operator ruling
:: 2026-09-18). Tower frequencies live in pkg/airfield/{oslk,olba}.go.

echo   Incirlik Tower (LTAG) -^> dashboard 6041   [PRIMARY]
start "Incirlik Tower" cmd /c "%~dp0atc.exe --airfield LTAG --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6041 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 3 /nobreak >nul

echo   Ramat David Tower (LLRD) -^> dashboard 6042   [PRIMARY]
start "Ramat David Tower" cmd /c "%~dp0atc.exe --airfield LLRD --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6042 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 3 /nobreak >nul

:: Akrotiri: British female RAF controller (nova + raf-british), operator 2026-09-19.
echo   Akrotiri Tower (LCRA) -^> dashboard 6046   [PRIMARY]
start "Akrotiri Tower" cmd /c "%~dp0atc.exe --airfield LCRA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --voice-style-tower raf-british --dashboard-port 6046 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 3 /nobreak >nul

echo   Bassel Al-Assad Tower (OSLK) -^> dashboard 6049   [DIVERT]
start "Bassel Al-Assad Tower" cmd /c "%~dp0atc.exe --airfield OSLK --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6049 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 3 /nobreak >nul

echo   Beirut Tower (OLBA) -^> dashboard 6050   [DIVERT]
start "Beirut Tower" cmd /c "%~dp0atc.exe --airfield OLBA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice echo --dashboard-port 6050 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
timeout /t 3 /nobreak >nul

:: ---------------------------------------------------------------------
:: PARKED 2026-09-19 -- these five were on the card and ran until today.
:: Uncomment to bring one back, and add its ICAO to --atis-stations in
:: start_atis_syria.bat or it will have a tower and no ATIS.
:: start "King Hussein Tower" cmd /c "%~dp0atc.exe --airfield OJMF --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice alloy --dashboard-port 6043 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: start "Hatay Tower" cmd /c "%~dp0atc.exe --airfield LTDA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice echo --dashboard-port 6044 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: start "Gaziantep Tower" cmd /c "%~dp0atc.exe --airfield LTAJ --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice fable --dashboard-port 6045 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: start "Paphos Tower" cmd /c "%~dp0atc.exe --airfield LCPH --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6047 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: start "H4 Tower" cmd /c "%~dp0atc.exe --airfield OJHR --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6048 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
:: ---------------------------------------------------------------------
echo [vSFG-7] Syria towers launched.
