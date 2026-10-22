@echo off
cd /d C:\SkyeyeATC
if not defined SKYEYE_SRS (
    echo ERROR: SKYEYE_SRS env var not set. Run: setx SKYEYE_SRS ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set SRS=%SKYEYE_SRS%
if not defined SRS_EAM (
    echo ERROR: SRS_EAM env var not set. Run: setx SRS_EAM ^<password^>  then open a new cmd.
    pause
    exit /b 1
)
set EAM=%SRS_EAM%
if not defined SKYEYE_TACVIEW (
    echo ERROR: SKYEYE_TACVIEW env var not set. Run: setx SKYEYE_TACVIEW ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set TACVIEW=%SKYEYE_TACVIEW%
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"
:: RED side, ON DEMAND (operator 2026-10-22, Day, BFM Venom vs Raider: "a red Command
:: 280.00 ... Red will be using mozdok airfield", "Mozdok tower is 266", "tonight only
:: and maybe used in the future but not all the time"). NOT called by
:: start_region_caucasus.bat: start it from the dashboard when red flies. An Update
:: stops it with the rest of the Caucasus region and does NOT bring it back.
::
:: --coalition red: both register and transmit on SRS as red (coalition 1).
:: Red Command 280.0 (BFM mission: Hornet COMM2 CH4, Viper COMM1 CH3), Gideon (the old
:: Command voice, so red does not sound like blue's Jodi): check-in, radio check,
:: fence in/out, RTB and recovery handoff to Mozdok Tower only (--command-fields XRMF);
:: no tasking, ranges, tankers, JTAC, GCI or Marshal - those are blue's.
:: Mozdok Tower 266.0 (Hornet COMM1 CH7, Viper COMM1 CH6), Adam; no ATIS at Mozdok.
:: Departures are handed to Red Command 280.0.
start "vSFG-7 Red Command (Caucasus)" cmd /c "%~dp0atc.exe --command-only --coalition red --map caucasus --command-fields XRMF --command-freq 280.0 --command-name Red-Command --command-voice onyx --elevenlabs-command-voice q1h5HGdnfVxp4TXTJRNN --handoff-marshal-freq 0 --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% %MIZ_FLAG% --pprof-port 7776 --log-level %LOG%"
%SystemRoot%\System32\timeout.exe /t 5 /nobreak >nul

start "Mozdok Tower" cmd /c "%~dp0atc.exe --airfield XRMF --coalition red --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice onyx --elevenlabs-tower-voice pNInz6obpgDQGcFmaJgB --dashboard-port 6016 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%"
