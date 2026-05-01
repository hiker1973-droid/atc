@echo off
cd /d C:\SkyeyeATC
set SRS=localhost:5008
set TACVIEW=localhost:42676
set EAM=blue42
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
start "vSFG-7 Scudwatch" cmd /k "%~dp0atc.exe --scudwatch-only --scudwatch-freq 264.00 --scudwatch-callsign Darkstar-1-1 --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --tts-voice onyx --log-level %LOG%"
