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

:: Command Channel
start "vSFG-7 Command" cmd /k "%~dp0atc.exe --command-only --command-freq 282.0 --command-name vSFG-7-Command --command-voice onyx --srs-addr %SRS% --eam-password %EAM% --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

:: Deckboss (carrier ops)
start "Deckboss" cmd /k "%~dp0atc.exe --airfield OMDM --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --deckboss-freq 306.2 --no-atis --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

echo [vSFG-7] All systems launched.
