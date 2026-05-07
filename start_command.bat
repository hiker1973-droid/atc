@echo off
cd /d C:\SkyeyeATC
set SRS=localhost:5008
set EAM=blue42
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB

start "vSFG-7 Command" cmd /k "%~dp0atc.exe --command-only --command-freq 282.0 --command-name vSFG-7-Command --command-voice onyx --srs-addr %SRS% --eam-password %EAM% --pprof-port 7770 --log-level %LOG%"
