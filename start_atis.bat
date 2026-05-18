@echo off
cd /d C:\SkyeyeATC
set SRS=localhost:5008
set EAM=blue42
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
start "vSFG-7 ATIS" cmd /c "%~dp0atc.exe --atis-only --srs-addr %SRS% --eam-password %EAM% --log-level %LOG%"

