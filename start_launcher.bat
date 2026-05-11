@echo off
cd /d %~dp0
if not defined SKYEYE_SRS (
    echo ERROR: SKYEYE_SRS env var not set. Run: setx SKYEYE_SRS ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
if not defined SKYEYE_TACVIEW (
    echo ERROR: SKYEYE_TACVIEW env var not set. Run: setx SKYEYE_TACVIEW ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
start "vSFG-7 Launcher" cmd /k "%~dp0launcher.exe --listen :7000 --srs-addr %SKYEYE_SRS% --tacview-addr %SKYEYE_TACVIEW%"
%SystemRoot%\System32\timeout.exe /t 2 /nobreak >nul
start http://localhost:7000/
