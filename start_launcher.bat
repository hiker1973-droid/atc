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
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"
rem Launch the exe directly. Wrapping it in `cmd /c "..."` nested the quotes
rem around SKYEYE_MIZ inside an already-quoted command string; cmd strips the
rem outer pair and mangles the inner ones, so a mission path containing a space
rem (every DCS path does -- "Saved Games") reached the launcher truncated.
start "vSFG-7 Launcher" "%~dp0launcher.exe" --listen :7000 --srs-addr %SKYEYE_SRS% --tacview-addr %SKYEYE_TACVIEW% %MIZ_FLAG%
%SystemRoot%\System32\timeout.exe /t 2 /nobreak >nul
start http://localhost:7000/
