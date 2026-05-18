@echo off
cd /d C:\SkyeyeATC
set SRS=localhost:5008
set EAM=blue42
set TACVIEW=localhost:42676
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB

start "Deckboss" cmd /k "%~dp0atc.exe --airfield OMDM --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --deckboss-freq 128.6 --no-atis --dashboard-port 6005 --log-level %LOG%"
