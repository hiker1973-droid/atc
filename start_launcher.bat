@echo off
cd /d %~dp0
start "vSFG-7 Launcher" cmd /c "%~dp0launcher.exe --listen :7000 --srs-addr localhost:5008 --tacview-addr localhost:42676"
%SystemRoot%\System32\timeout.exe /t 2 /nobreak >nul
start http://localhost:7000/
