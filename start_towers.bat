@echo off
cd /d C:\SkyeyeATC
set SRS=localhost:5008
set EAM=blue42
set TACVIEW=localhost:42676
set LOG=info
set GOMAXPROCS=4
set GOGC=50
set GOMEMLIMIT=512MiB

echo [vSFG-7] Starting ATC system...

:: Al Minhad Tower (Marshal now has its own start_marshal.bat)
start "Al Minhad Tower" cmd /c "%~dp0atc.exe --airfield OMDM --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --dashboard-port 6001 --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Dhafra Tower
start "Al Dhafra Tower" cmd /c "%~dp0atc.exe --airfield OMAM --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --dashboard-port 6002 --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Al Ain Tower
start "Al Ain Tower" cmd /c "%~dp0atc.exe --airfield OMAL --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --dashboard-port 6003 --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] All systems launched.
