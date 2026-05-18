@echo off
cd /d C:\SkyeyeATC
set SRS=localhost:5008
set TACVIEW=localhost:42676
set EAM=blue42
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
start "Marshal" cmd /c "%~dp0atc.exe --marshal-only --airfield OMDM --marshal-freq 306.3 --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 --log-level %LOG%"
